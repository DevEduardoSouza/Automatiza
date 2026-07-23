package automation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/ctapicker"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/workers"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// consumeListItem marca o item como consumido com contexto DESACOPLADO e um
// pequeno retry. O consumo é o registro de progresso da lista: falhar em
// silêncio (ctx do run já estourado, blip do banco) fazia GetNextUnconsumed
// devolver o MESMO item no próximo tick e reenviá-lo quando a janela de dedup
// não cobria (dedupe_hours < intervalo, ou chaves do Redis perdidas).
func (s *automationService) consumeListItem(ctx context.Context, itemID, automationID string, groupID *string) {
	base := context.WithoutCancel(ctx)
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		opCtx, cancel := context.WithTimeout(base, 5*time.Second)
		err = s.mlLinkListItemRepo.MarkAsConsumed(opCtx, []string{itemID}, automationID, groupID)
		cancel()
		if err == nil {
			return
		}
		time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
	}
	observability.LogAndCapture(s.logger, sentry.LevelError,
		"Failed to mark list item as consumed after retries (risco de reenvio no proximo ciclo)", err,
		map[string]string{"module": "automation", "operation": "consume_list_item", "item_id": itemID})
}

const (
	// maxDeadLinksPerRun is how many unusable items a single run may walk past
	// while still trying to deliver. Dead links (product gone from the affiliate
	// catalogue, unparseable URL, no affiliate link) are common in long-lived
	// lists, and without this budget one of them silently costs a whole cycle.
	maxDeadLinksPerRun = 9

	// maxListPoolSize bounds the candidate fetch so a list full of dead links
	// can't turn one run into a long march of failing enrichment calls.
	maxListPoolSize = 30
)

// firstURL cuts a raw list-item URL down to its first embedded URL. Returns
// the input unchanged when it holds a single URL. Matches only full scheme
// markers ("https://", "http://") past position 0, so a bare "http" inside a
// query string doesn't trigger a cut.
func firstURL(raw string) string {
	cut := len(raw)
	for _, marker := range []string{"https://", "http://"} {
		if i := strings.Index(raw[1:], marker); i >= 0 && i+1 < cut {
			cut = i + 1
		}
	}
	return raw[:cut]
}

// processListMode processes LIST mode automations.
// It fetches unconsumed items from the linked ML link list, processes each item
// (generates affiliate links, builds messages), and sends them to configured groups.
//
// LIST mode behavior (following Node.js parity):
// - Always processes 1 item per run per group (fixed for product consistency)
// - Always includes image when available
// - Items are consumed in order (order_index) and marked as consumed after sending
// - Deduplication is handled at the list level (consumed items are not reprocessed)
func (s *automationService) processListMode(ctx context.Context, automation *models.Automation) (*models.RunAutomationResult, error) {
	s.logger.Info("Processing LIST mode automation",
		zap.String("automationID", automation.ID),
		zap.String("automationName", automation.Name))

	// Validate ml_link_list_id is configured
	if automation.MlLinkListID == nil || strings.TrimSpace(*automation.MlLinkListID) == "" {
		return s.pauseForConfigError(ctx, automation,
			"Selecione uma Lista de Links para usar no modo Lista."), nil
	}

	listID := strings.TrimSpace(*automation.MlLinkListID)

	// Validate target groups are configured
	targetGroups := automation.GetTargetGroups()
	if len(targetGroups) == 0 {
		return s.pauseForConfigError(ctx, automation,
			"Nenhum grupo de envio configurado. Configure pelo menos um grupo na automacao."), nil
	}

	// Filter to only valid WhatsApp group JIDs
	validGroups := make([]string, 0, len(targetGroups))
	for _, gid := range targetGroups {
		if strings.Contains(gid, "@g.us") {
			validGroups = append(validGroups, gid)
		}
	}

	if len(validGroups) == 0 {
		return s.pauseForConfigError(ctx, automation,
			"Nenhum grupo valido configurado. Os grupos devem terminar com @g.us."), nil
	}

	// Get connected WhatsApp instance
	instances, err := s.instanceRepo.GetUserInstances(ctx, automation.UserID)
	if err != nil {
		s.logger.Error("Failed to get user instances",
			zap.String("userID", automation.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get WhatsApp instances: %w", err)
	}

	var connectedInstance *models.Instance
	for i := range instances {
		if instances[i].Status == "connected" {
			connectedInstance = &instances[i]
			break
		}
	}

	if connectedInstance == nil {
		// Chip desconectado NÃO é erro de disparo: nada foi tentado e a automação
		// fica em stand-by (o cron re-roda a cada minuto e retoma sozinho quando o
		// cliente reconecta — não pausa, não carimba last_error). Registrar como
		// 'skipped' (mesmo status do dedupe) tira do card de erros do painel; a
		// contagem de números desconectados vira métrica própria (não erro).
		errMsg := "WhatsApp nao esta conectado. Conecte seu WhatsApp para enviar ofertas."
		s.logDispatch(ctx, automation, "system", "https://error.log", "skipped", errMsg)
		return &models.RunAutomationResult{
			Success: false,
			Message: errMsg,
		}, nil
	}

	// Resolve per-group instances when multi-instance dispatch is enabled.
	// Returns nil when flag is off, so the inner loop transparently uses
	// connectedInstance as the sender.
	groupInstances := s.fetchGroupInstancesIfEnabled(ctx, automation.UserID, validGroups)

	// Get items per run (defaults to 1 for LIST mode per Node.js parity)
	// Clamped to 1-20 range to match frontend validation
	itemsPerRun := 1
	if automation.ListItemsPerRun != nil && *automation.ListItemsPerRun > 0 {
		itemsPerRun = *automation.ListItemsPerRun
	}
	if itemsPerRun > 20 {
		itemsPerRun = 20
	}

	// Include image (defaults to true for LIST mode per Node.js parity)
	includeImage := true
	if automation.ListIncludeImage != nil {
		includeImage = *automation.ListIncludeImage
	}

	// Send link as downloaded image + caption instead of native link preview card.
	// Defaults to true (rollout global 18/07: sem card por padrão; nil = imagem).
	// Valor explícito no banco (TRUE/FALSE) continua sobrepondo o default.
	sendAsImage := true
	if automation.ListSendAsImage != nil {
		sendAsImage = *automation.ListSendAsImage
	}

	// Candidate pool, deliberately larger than what we intend to deliver.
	//
	// itemsPerRun is a DELIVERY target, not a fetch size. Fetching exactly
	// itemsPerRun meant a single dead link wasted the whole cycle: the item was
	// dropped (no price/image → we refuse to send a broken offer) and the run
	// ended with zero sends until the next interval. With a list where a good
	// share of the Shopee links are no longer in the affiliate catalogue, that
	// turned a 10-minute automation into roughly one delivery per hour.
	//
	// Pulling spare candidates lets the run walk past dead links and still
	// deliver. The extra ones cost nothing when the first items are healthy —
	// the loop stops as soon as it has delivered itemsPerRun items.
	poolSize := itemsPerRun + maxDeadLinksPerRun
	if poolSize > maxListPoolSize {
		poolSize = maxListPoolSize
	}

	items, err := s.mlLinkListItemRepo.GetNextUnconsumed(ctx, listID, poolSize)
	if err != nil {
		s.logger.Error("Failed to get unconsumed items",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get unconsumed items: %w", err)
	}

	// Auto-repeat: quando a lista esgota (todos consumidos) E a flag está
	// ligada, reseta consumed_at em massa e refetch. Permite que automation
	// LIST rode em loop sem intervenção manual. Default TRUE (no schema).
	//
	// Sem auto-repeat, a automation para silenciosamente e o usuário precisa
	// clicar "Resetar enviados" na UI — fricção desnecessária para o caso
	// comum em que a lista deve ciclar.
	if len(items) == 0 && automation.RepeatWhenFinished {
		resetCount, resetErr := s.mlLinkListRepo.ResetConsumedItems(ctx, listID)
		if resetErr != nil {
			s.logger.Warn("Failed to auto-reset consumed items, list remains empty",
				zap.String("listID", listID),
				zap.Error(resetErr))
		} else if resetCount > 0 {
			s.logger.Info("List exhausted — auto-reset triggered (repeat_when_finished=true)",
				zap.String("listID", listID),
				zap.Int64("resetCount", resetCount))
			items, err = s.mlLinkListItemRepo.GetNextUnconsumed(ctx, listID, poolSize)
			if err != nil {
				s.logger.Error("Failed to refetch unconsumed items after auto-reset",
					zap.String("listID", listID),
					zap.Error(err))
				return nil, fmt.Errorf("failed to refetch unconsumed items: %w", err)
			}
		}
	}

	if len(items) == 0 {
		infoMsg := "Lista sem itens disponiveis (todos consumidos ou inativos)."
		s.logger.Info("No unconsumed items in list",
			zap.String("listID", listID))
		s.logDispatch(ctx, automation, "system", "about:blank", "skipped", infoMsg)
		return &models.RunAutomationResult{
			Success:     true,
			Message:     infoMsg,
			OffersFound: 0,
		}, nil
	}

	// Get dispatch text template if configured (only if active)
	var dispatchTemplate string
	if automation.DispatchTextID != nil {
		dt, _ := s.settingsRepo.GetActiveDispatchTextByID(ctx, *automation.DispatchTextID, automation.UserID)
		if dt != nil {
			// Sorteia entre as variações a cada envio. Antes usava só dt.Body
			// (= copies[0]), então mesmo com N variações cadastradas sempre saía
			// a primeira. dt.Copies é a lista de variações (Body é a 1ª).
			dispatchTemplate = dt.Body
			if len(dt.Copies) > 0 {
				dispatchTemplate = dt.Copies[rand.IntN(len(dt.Copies))] //nolint:gosec // G404: non-security random for copy rotation
			}
		}
	}

	// Coupon resolution moved to per-item via the rules engine — see the
	// ApplyCoupon call inside the per-item loop below. The legacy
	// per-automation preload is gone because the matched coupon depends
	// on the item's price (different rules apply at different price tiers).

	result := &models.RunAutomationResult{
		Success:      true,
		OffersFound:  0,
		MessagesSent: 0,
	}

	// Items successfully turned into a dispatch. Drives the itemsPerRun stop
	// condition; dead links walked past do not count.
	delivered := 0

	// Pre-fetch CTA entities. Política:
	//   - automation.CTAIDs preenchido → override explícito, busca só esses
	//   - automation.CTAIDs vazio → pool inteiro de CTAs ativos do user
	//
	// Pool inteiro é o caminho default (paridade com affilipro): o user
	// cadastra CTAs em /ctas e o sistema escolhe automaticamente o que casa
	// com a plataforma do produto. O override existe pra automations que
	// querem restringir a um subconjunto curado.
	var ctaEntities []models.CTA
	if s.ctaRepo != nil {
		if len(automation.CTAIDs) > 0 {
			fetched, ctaErr := s.ctaRepo.GetCTAsByIDs(ctx, automation.CTAIDs, automation.UserID)
			if ctaErr != nil {
				s.logger.Warn("Failed to fetch CTA entities by IDs, falling back to inline CTAs",
					zap.String("automationID", automation.ID),
					zap.Error(ctaErr))
			} else {
				ctaEntities = fetched
			}
		} else {
			fetched, ctaErr := s.ctaRepo.GetUserCTAs(ctx, automation.UserID, map[string]interface{}{"is_active": true})
			if ctaErr != nil {
				s.logger.Warn("Failed to fetch user CTA pool, falling back to inline CTAs",
					zap.String("automationID", automation.ID),
					zap.Error(ctaErr))
			} else {
				ctaEntities = fetched
			}
		}
	}

	// Process each item
	for _, item := range items {
		originURL := strings.TrimSpace(item.OriginURL)
		if originURL == "" {
			s.logger.Warn("Empty origin URL in list item, skipping",
				zap.String("itemID", item.ID))
			continue
		}
		// Itens importados chegam com emoji colado na URL ("🔥https://s.shopee.
		// com.br/xyz🔥"). URL válida é ASCII: sem o corte, a conversão falha e
		// o fallback (affiliateURL = originURL) mandava o link COM emoji pro
		// grupo — link que não abre no destinatário. Primeiro descarta lixo
		// ANTES do esquema, depois corta no primeiro rune não-ASCII.
		if idx := strings.Index(originURL, "http"); idx > 0 {
			originURL = originURL[idx:]
		}
		if cut := text.CutURLAtNonASCII(originURL); cut != originURL {
			s.logger.Warn("List item URL had non-ASCII glued to it — truncated",
				zap.String("itemID", item.ID),
				zap.String("kept", cut))
			originURL = cut
		}
		if originURL == "" {
			s.logger.Warn("List item URL empty after non-ASCII cut, skipping",
				zap.String("itemID", item.ID))
			continue
		}
		// Users paste several share links into one item often enough that prod
		// carries entries like "https://s.shopee.com.br/Ahttps://s.shopee...".
		// Expanding that mangled string fails enrichment ("invalid shopee URL
		// after expansion") and the item gets burned as a dead link. One item
		// yields one offer, so anything past the first URL is unusable from
		// here anyway — keep the first URL and deliver it instead of losing
		// the item.
		if trimmed := firstURL(originURL); trimmed != originURL {
			s.logger.Warn("List item holds multiple concatenated URLs — using the first",
				zap.String("itemID", item.ID),
				zap.String("kept", trimmed))
			originURL = trimmed
		}

		// Detect platform. Shopee is the only store the backend converts to
		// an affiliate URL on the user's behalf — ML and every other store
		// run as passthrough: the user pasted their already-affiliated URL
		// and (optionally) the product metadata, and we just dispatch it.
		isML := text.IsMLLink(originURL)
		isShopee := text.IsShopeeLink(originURL)

		// Fetch product preview and generate affiliate link based on platform
		var preview *models.MLProductPreview
		var shopeeProduct *models.ShopeeOffer
		var shopeeImageURL string
		// Last Shopee enrichment error (affiliate link or product fetch). Used
		// below to decide whether a degraded item may be retried (transient
		// failure) or must be consumed (permanent failure).
		var shopeeEnrichErr error
		affiliateURL := originURL

		if isML {
			// ML: preferimos o preview FRESCO (mlService), pois ele traz preço
			// atual + ORIGINAL + desconto + imagem — o metadata gravado no item
			// só tem o preço atual (sem original), então o "De X por Y" sairia
			// quebrado. Resolve o link (meli.la/sec) pela página /social. Tem
			// cache (10min), então não refaz a cada disparo. Ver issue #158.
			if s.mlService != nil {
				fetched, perr := s.mlService.GetProductPreview(ctx, originURL, automation.UserID)
				if perr != nil {
					s.logger.Warn("Failed to enrich ML list item via affiliate preview",
						zap.String("originURL", originURL),
						zap.Error(perr))
				} else if fetched != nil && (fetched.Title != "" || fetched.Price > 0) {
					preview = fetched
				}
			}
			// Fallback pro metadata gravado no item quando a busca não trouxe
			// dados (ex.: link ML direto bloqueado por anti-bot, ou preço
			// preenchido manualmente).
			if preview == nil && (item.ProductTitle != nil || item.ProductPrice != nil || item.ProductImageURL != nil) {
				p := &models.MLProductPreview{}
				if item.ProductTitle != nil {
					p.Title = *item.ProductTitle
				}
				if item.ProductPrice != nil {
					p.Price = *item.ProductPrice
				}
				if item.ProductImageURL != nil {
					p.Image = *item.ProductImageURL
				}
				preview = p
			}
		} else if isShopee && s.shopeeService != nil {
			// Shopee: generate affiliate link + fetch product details.
			//
			// Retry 1x: o ExpandURL/afiliado falha por timeout transitório do
			// Shopee de vez em quando (context deadline exceeded). Sem retry, a
			// oferta saía degradada (link cru, sem imagem/preço). A 2ª tentativa
			// quase sempre passa — basta uma pausa curta.
			var link *models.ShopeeAffiliateLink
			var err error
			for attempt := 0; attempt < 2; attempt++ {
				link, err = s.shopeeService.GenerateAffiliateLink(ctx, originURL, automation.UserID)
				if err == nil {
					break
				}
				if attempt == 0 {
					s.logger.Warn("Shopee affiliate link failed, retrying once",
						zap.String("originURL", originURL),
						zap.Error(err))
					time.Sleep(800 * time.Millisecond)
				}
			}
			if err != nil {
				shopeeEnrichErr = err
				s.logger.Warn("Failed to create Shopee affiliate link after retry, using origin URL",
					zap.String("originURL", originURL),
					zap.Error(err))
			} else if link != nil && link.AffiliateLink != "" {
				affiliateURL = link.AffiliateLink
				if link.ImageURL != "" {
					shopeeImageURL = link.ImageURL
				}
				// Fetch product details (title, price) for the message template.
				// Retry espaçado: o rate limit da Shopee (10030) é janela rolante
				// com brechas — a sondagem em produção mostrou chamadas passando
				// entre bursts. Sem retry, uma única brecha perdida custa o ciclo
				// inteiro da lista (o transient dá break lá embaixo).
				if link.ItemID > 0 && link.ShopID > 0 {
					var product *models.ShopeeOffer
					var prodErr error
					for attempt := 0; attempt < 3; attempt++ {
						product, prodErr = s.shopeeService.GetProduct(ctx, link.ItemID, link.ShopID, automation.UserID)
						if prodErr == nil || !isTransientShopeeEnrichError(prodErr) {
							break
						}
						if attempt < 2 {
							time.Sleep(2 * time.Second)
						}
					}
					if prodErr != nil {
						shopeeEnrichErr = prodErr
						s.logger.Warn("Failed to get Shopee product details for list item",
							zap.String("originURL", originURL),
							zap.Int64("itemID", link.ItemID),
							zap.Int64("shopID", link.ShopID),
							zap.Error(prodErr))
					} else {
						shopeeProduct = product
						// Use product image if affiliate link didn't return one
						if shopeeImageURL == "" && product.Image != "" {
							shopeeImageURL = product.Image
						}
					}
				}
			}
		}

		// Skip de oferta degradada (Shopee): se o item é de produto Shopee mas
		// o enriquecimento não trouxe o produto (shopeeProduct nil, mesmo após
		// o retry acima), não temos preço nem imagem — enviar geraria uma
		// oferta quebrada (texto cru, sem preço/imagem) no grupo de destino.
		// Melhor pular esse envio.
		//
		// Consumimos o item mesmo assim, de propósito: GetNextUnconsumed
		// devolve sempre o próximo NÃO-consumido por ordem, então deixá-lo
		// pendente travaria a lista nesse item a cada ciclo (item ruim/404
		// nunca avançaria). Como listas com repeat_when_finished re-incluem
		// tudo no próximo ciclo completo, o item é tentado de novo depois —
		// nada se perde de vez, e nenhuma oferta quebrada sai no grupo.
		if isShopee && shopeeProduct == nil {
			// Erro de CREDENCIAL (secret errado/regenerado no painel, credencial
			// ausente/incompleta): TODO item vai falhar identicamente até o
			// usuário recadastrar — deixar o ciclo rodando queima a lista
			// inteira, ~poolSize itens por vez (caso real 11/07: secret errado
			// salvo às 21:58 queimou 90 itens em 2h). Pausa a automação com
			// erro acionável, preserva o item e encerra o run — o MESMO
			// contrato do monitoring e do offer_generate para credenciais.
			if isShopeeCredentialEnrichError(shopeeEnrichErr) {
				s.logger.Warn("Shopee credential error in LIST enrichment — pausing automation",
					zap.String("automationID", automation.ID),
					zap.String("originURL", originURL),
					zap.Error(shopeeEnrichErr))
				s.logDispatch(ctx, automation, "system", originURL, "failed",
					"Credenciais Shopee ausentes ou invalidas - automacao pausada; recadastre App ID e Senha")
				nowStr := time.Now().UTC().Format(time.RFC3339)
				if upErr := s.automationRepo.UpdateAutomation(ctx, automation.ID, map[string]interface{}{
					"status":        models.AutomationStatusPaused,
					"last_error":    "Credenciais Shopee ausentes ou invalidas - automacao pausada automaticamente; recadastre App ID e Senha",
					"last_error_at": nowStr,
				}); upErr != nil {
					s.logger.Error("Failed to pause automation on credential error",
						zap.String("automationID", automation.ID),
						zap.Error(upErr))
				}
				result.Message = "Automacao pausada: credenciais Shopee invalidas"
				return result, nil
			}
			// A TRANSIENT enrichment failure (Shopee rate limit 10030, timeout,
			// network blip, 5xx) must NOT consume the item. Consuming advances
			// GetNextUnconsumed past a link we never delivered, so a rate-limit
			// window silently drains the whole list to zero sends — the exact
			// failure mode where a "list" automation keeps running every cycle
			// but delivers nothing, and the burned items only come back after a
			// full repeat_when_finished reset. Leaving the item pending makes the
			// next run retry it once the limit clears.
			//
			// A PERMANENT failure (product not found, missing/invalid credentials,
			// no Open-API access) still consumes: retrying forever would stall the
			// list on a dead item — the original reason this skip consumed at all.
			transient := isTransientShopeeEnrichError(shopeeEnrichErr)
			reason := "enriquecimento Shopee falhou (sem preco/imagem) - item pulado"
			if transient {
				reason = "Shopee indisponivel (rate limit/timeout) - item mantido para retry"
			}
			s.logger.Warn("Skipping degraded Shopee list item — no product after retry (avoiding priceless/imageless offer)",
				zap.String("automationID", automation.ID),
				zap.String("originURL", originURL),
				zap.Bool("transient", transient),
				zap.Error(shopeeEnrichErr))
			s.logDispatch(ctx, automation, "system", originURL, "skipped", reason)
			if transient {
				// Shopee is saturated or unreachable right now. Stop the run
				// instead of walking to the next candidate: every one of them
				// would hit the same rate limit, and each extra call pushes the
				// window further out. The item stays unconsumed, so the next
				// cycle picks it up once Shopee recovers.
				break
			}
			// Permanently dead link — burn it and try the next candidate in THIS
			// run, so one bad item doesn't cost the whole cycle.
			s.consumeListItem(ctx, item.ID, automation.ID, nil)
			continue
		}

		// Resolve a coupon for this specific item via the rules engine.
		// The matched coupon depends on offerPrice, so this lookup runs
		// once per item — see CouponHandler.ApplyCoupon for details.
		var couponResult *CouponResult
		if s.couponHandler != nil {
			var offerPrice float64
			var offerStore string
			var productIsOfficial bool
			var productCategoryIDs []int64
			if isML && preview != nil {
				offerPrice = preview.Price
				offerStore = "ml"
				// ML doesn't expose a Mall-style flag — leave false.
				// Nem categoria oficial: productCategoryIDs fica nil, então
				// só regras genéricas ficam elegíveis.
			} else if isShopee && shopeeProduct != nil {
				offerPrice = shopeeProduct.Price
				offerStore = "shopee"
				productIsOfficial = shopeeProduct.IsMall
				productCategoryIDs = shopeeProduct.ProductCategoryIDs
			}
			if offerPrice > 0 {
				if cr, err := s.couponHandler.ApplyCoupon(ctx, automation, offerPrice, offerStore, productIsOfficial, productCategoryIDs); err == nil {
					couponResult = cr
				}
			}
		}

		// Build message for this item
		message := s.buildListModeMessage(automation, item, preview, shopeeProduct, affiliateURL, dispatchTemplate, isML, isShopee, ctaEntities, couponResult)

		// Get image URL if available and enabled.
		// ML delivers .webp which WhatsApp SendImage chokes on; the LinkPreview
		// branch below already does this swap, apply it here too so the
		// sendAsImage=true path gets a usable URL.
		var imageURL *string
		if includeImage {
			if preview != nil && preview.Image != "" {
				normalized := strings.Replace(preview.Image, ".webp", ".jpg", 1)
				imageURL = &normalized
			} else if shopeeImageURL != "" {
				imageURL = &shopeeImageURL
			} else if item.ProductImageURL != nil && *item.ProductImageURL != "" {
				imageURL = item.ProductImageURL
			}
		}

		// Build custom link preview for ML/Shopee items with preview data.
		// Skipped when sendAsImage is true: worker falls back to SendImage using imageURL.
		var linkPreview *workers.LinkPreviewData
		if !sendAsImage {
			if isML && preview != nil && preview.Title != "" {
				desc := fmt.Sprintf("R$ %.2f", preview.Price)
				if preview.OriginalPrice != nil {
					desc = fmt.Sprintf("De R$ %.2f por R$ %.2f", *preview.OriginalPrice, preview.Price)
				}
				if couponResult != nil && couponResult.Applied && couponResult.DiscountedPrice < preview.Price {
					desc = fmt.Sprintf("De R$ %.2f por R$ %.2f com cupom", preview.Price, couponResult.DiscountedPrice)
				}
				imgURL := ""
				if preview.Image != "" {
					imgURL = strings.Replace(preview.Image, ".webp", ".jpg", 1)
				}
				linkPreview = &workers.LinkPreviewData{
					URL:         affiliateURL,
					Title:       preview.Title,
					Description: desc,
					ImageURL:    imgURL,
				}
				// Use link preview instead of image attachment
				imageURL = nil
			} else if isShopee && shopeeProduct != nil && shopeeProduct.Name != "" {
				desc := fmt.Sprintf("R$ %.2f", shopeeProduct.Price)
				if shopeeProduct.OriginalPrice > 0 && shopeeProduct.OriginalPrice > shopeeProduct.Price {
					desc = fmt.Sprintf("De R$ %.2f por R$ %.2f", shopeeProduct.OriginalPrice, shopeeProduct.Price)
				}
				if couponResult != nil && couponResult.Applied && couponResult.DiscountedPrice < shopeeProduct.Price {
					desc = fmt.Sprintf("De R$ %.2f por R$ %.2f com cupom", shopeeProduct.Price, couponResult.DiscountedPrice)
				}
				imgURL := shopeeImageURL
				if imgURL == "" && shopeeProduct.Image != "" {
					imgURL = shopeeProduct.Image
				}
				linkPreview = &workers.LinkPreviewData{
					URL:         affiliateURL,
					Title:       shopeeProduct.Name,
					Description: desc,
					ImageURL:    imgURL,
				}
				imageURL = nil
			}
		}

		// Send to each target group
		groupIDsSent := make([]string, 0, len(validGroups))
		// Espaçamento de envio acumulado POR CHIP (não global). Com o delay
		// global antigo (len(groupIDsSent)*3s), uma lista de 132 grupos
		// empurrava o último envio pra ~6,5 min — estourando o intervalo.
		// Por chip, os grupos se distribuem entre as sessões e entregam em
		// ~(grupos/chips)*3s. O session pacer (atômico por sessão) continua
		// garantindo o anti-ban entre automações que dividem o mesmo chip.
		perChipSent := make(map[string]int)
		// Anti-ban pacing: jittered per-chip delay accumulator + micro-breaks,
		// destination shuffle, and reputation-based suppression. Resolved for this
		// automation's humanize preset (default "high").
		perChipDelay := make(map[string]time.Duration)
		pacingCfg := config.GetHumanization().PacingConfigForMode(automation.HumanizeMode)
		rt := config.GetAntiBanRuntime()
		pacingCfg.Shuffle(validGroups)
		// Base delay: the user's delay_seconds when set, else 0 so pacing.Config's
		// BaseDelaySeconds (default 3s — the LIST mode's historical spacing) applies.
		baseDelay := 0
		if automation.DelaySeconds != nil && *automation.DelaySeconds > 0 {
			baseDelay = *automation.DelaySeconds
		}
		// Dedup por (grupo, produto): lista com o MESMO produto importado em
		// itens repetidos mandava a mesma promoção 2-3x no mesmo grupo em
		// ciclos consecutivos (com o pool, minutos de distância). Chave por
		// produto Shopee quando conhecido (pega o mesmo produto até via
		// shortlinks diferentes), senão hash da URL de origem. Janela:
		// dedupe_hours EXPLÍCITO da automação quando setado; default 2h —
		// curto de propósito para não brigar com repeat_when_finished
		// (recycles completos levam horas).
		listDedupHours := 2
		if automation.DedupeHours != nil && *automation.DedupeHours > 0 {
			listDedupHours = *automation.DedupeHours
		}
		itemCanonical := ""
		if isShopee && shopeeProduct != nil && shopeeProduct.ItemID > 0 && shopeeProduct.ShopID > 0 {
			itemCanonical = fmt.Sprintf("shopee:%d:%d", shopeeProduct.ShopID, shopeeProduct.ItemID)
		} else {
			sum := sha256.Sum256([]byte(strings.TrimSpace(originURL)))
			itemCanonical = "listurl:" + hex.EncodeToString(sum[:8])
		}
		// Store do item (para o dispatch log e o fallback de dedup em banco).
		itemStore := "other"
		if isShopee {
			itemStore = "shopee"
		} else if isML {
			itemStore = "ml"
		}
		dedupSkips := 0
		for _, groupID := range validGroups {
			// Apply URL safety check
			if s.urlSafetyService != nil {
				decision, err := s.urlSafetyService.Evaluate(ctx, message)
				if err != nil {
					s.logger.Warn("URL safety check failed",
						zap.String("groupID", groupID),
						zap.Error(err))
				} else if decision != nil && decision.Action == "block" {
					reason := fmt.Sprintf("Link bloqueado por seguranca: %s", decision.Reason)
					s.logDispatch(ctx, automation, groupID, originURL, "skipped", reason)
					continue
				}
			}

			// Check backpressure
			if s.backpressure != nil {
				can, _, _ := s.backpressure.CanEnqueue(ctx, automation.UserID)
				if !can {
					s.logDispatch(ctx, automation, groupID, originURL, "skipped", "backpressure")
					continue
				}
			}

			// Resolve the sending chip up-front so we can reputation-gate the group
			// before creating a dispatch log.
			senderSession := s.resolveSenderForGroup(automation, instances, connectedInstance, groupInstances, groupID)
			if senderSession == "" {
				s.logger.Warn("No connected chip for group, skipping LIST dispatch",
					zap.String("automationID", automation.ID),
					zap.String("groupID", groupID))
				continue
			}

			// Anti-ban: skip groups with a bad reputation (repeated failures / bot
			// removed). Quality-based suppression, not volume — fail-open (Redis
			// down => not suppressed).
			if rt.Reputation.IsSuppressed(ctx, senderSession, groupID) {
				s.logDispatch(ctx, automation, groupID, originURL, "skipped", "reputacao (grupo problematico)")
				continue
			}

			// Produto repetido: já foi enviado a ESTE grupo dentro da janela.
			if s.isDuplicate(ctx, automation.UserID, groupID, itemCanonical, listDedupHours) {
				dedupSkips++
				s.logDispatch(ctx, automation, groupID, originURL, "skipped", "dedupe (produto repetido na lista)")
				continue
			}

			// Create dispatch log BEFORE enqueueing so the worker can update its status.
			// Type é setado a partir de automation.Type ("list" pra esse caminho) pra
			// que queries de analytics consigam distinguir disparos do LIST mode dos
			// de monitoring/search/offer_generate. Sem isso o campo fica NULL e o
			// agrupamento por tipo na UI deixa de funcionar pra esse modo.
			var dispatchLogID *int
			if s.dispatchLogRepo != nil {
				featureFlags := config.GetFeatureFlags()
				if featureFlags.ShouldCreateDispatchLog() {
					dl := &models.DispatchLog{
						UserID:         automation.UserID,
						AutomationID:   &automation.ID,
						AutomationName: &automation.Name,
						GroupID:        &groupID,
						ProductURL:     &originURL,
						Status:         models.DispatchLogStatusQueued,
						Type:           automation.Type,
						// CanonicalKey persiste o dedup no BANCO: sem ele o slow
						// path (GetRecentByCanonicalKey) nunca casava para o LIST
						// e um restart do Redis zerava toda a proteção contra
						// produto repetido. Store distingue shopee/ml nas queries.
						CanonicalKey: &itemCanonical,
						Store:        &itemStore,
						// ListItemID liga o log ao item para o reconciliador
						// devolver itens "consumidos mas nunca entregues".
						ListItemID: &item.ID,
						CreatedAt:  time.Now().UTC().Format(time.RFC3339),
					}
					// Audita qual cupom (ou por que nenhum) saiu neste envio.
					withCoupon(couponResult)(dl)
					created, dlErr := s.dispatchLogRepo.Create(ctx, dl)
					if dlErr == nil && created != nil {
						dispatchLogID = &created.ID
					}
				}
			}

			// Enqueue message for sending
			jobData := &workers.MessageJobData{
				InstanceName:  senderSession,
				GroupID:       groupID,
				Message:       message,
				ImageURL:      imageURL,
				UserID:        automation.UserID,
				AutomationID:  automation.ID,
				DispatchLogID: dispatchLogID,
				LinkPreview:   linkPreview,
				HumanizeMode:  automation.HumanizeMode,
			}

			if s.queueClient != nil {
				// Delay acumulado por chip com jitter anti-ban (ver perChipDelay
				// acima): o N-ésimo envio DESTE chip espera o acumulado jittered,
				// em vez de N*3s fixo — e agora honra o delay_seconds da automação.
				delayMs := perChipDelay[senderSession]

				_, err := s.queueClient.EnqueueMessage(ctx, jobData, delayMs)
				if err != nil {
					s.logger.Error("Failed to enqueue message",
						zap.String("groupID", groupID),
						zap.Error(err))
					s.logDispatch(ctx, automation, groupID, originURL, "failed", err.Error())
					continue
				}
			}

			// Record backpressure
			if s.backpressure != nil {
				_ = s.backpressure.RecordEnqueue(ctx, automation.UserID)
			}
			// Reivindica o slot de dedup NO ENFILEIRAMENTO (não na entrega): o
			// próximo ciclo roda em minutos e precisa ver o produto como já
			// disparado para este grupo, mesmo com a fila ainda drenando.
			s.markAsSent(ctx, automation.UserID, groupID, itemCanonical, listDedupHours)
			groupIDsSent = append(groupIDsSent, groupID)
			perChipSent[senderSession]++
			// Advance this chip's jittered delay accumulator: base (delay_seconds)
			// × jitter, plus an occasional micro-break, clamped so a big list can't
			// overrun the interval.
			perChipDelay[senderSession] = pacingCfg.ClampSpread(
				perChipDelay[senderSession] +
					pacingCfg.Step(baseDelay) +
					rt.Pacing.MicroBreak(ctx, senderSession, pacingCfg))
			result.MessagesSent++
		}

		// Item repetido de ponta a ponta: TODOS os grupos dedupam (o mesmo
		// produto foi importado mais de uma vez na lista). Consome o item —
		// deixá-lo pendente travaria a fila neste item a cada ciclo — e segue
		// para o próximo candidato do pool AINDA neste ciclo.
		if len(groupIDsSent) == 0 && dedupSkips > 0 {
			s.logger.Info("List item fully deduped (duplicate product in list) — consuming",
				zap.String("automationID", automation.ID),
				zap.String("originURL", originURL),
				zap.Int("dedupSkips", dedupSkips))
			s.consumeListItem(ctx, item.ID, automation.ID, nil)
			continue
		}

		// Mark item as consumed (only if we sent to at least one group).
		// O consumo aqui é OTIMISTA (no enfileiramento): se TODOS os envios do
		// item falharem terminalmente no worker, o reconciliador
		// (ReconcileListDeliveries, cron de 10min) detecta pelo dispatch_logs
		// (0 sent, ≥1 failed, 0 queued) e devolve o item para retry.
		if len(groupIDsSent) > 0 {
			// Use the first group ID for tracking (the item is consumed for all groups)
			firstGroupID := groupIDsSent[0]
			s.consumeListItem(ctx, item.ID, automation.ID, &firstGroupID)

			// itemsPerRun counts items actually DELIVERED, not items looked at.
			// The pool above is only a budget for walking past dead links, so
			// stop as soon as the target is met — a healthy list still sends
			// exactly itemsPerRun items per cycle, as configured.
			delivered++
			if delivered >= itemsPerRun {
				break
			}
		}
	}

	// Report what actually became an offer, not how many candidates we pulled:
	// the pool is an implementation detail and would otherwise inflate this.
	result.OffersFound = delivered

	if result.MessagesSent > 0 {
		result.Message = fmt.Sprintf("Automacao executada: %d mensagem(s) enfileirada(s)", result.MessagesSent)
	} else {
		result.Message = "Nenhuma mensagem foi enviada"
	}

	return result, nil
}

// isTransientShopeeEnrichError reports whether a Shopee enrichment failure is
// temporary — worth retrying the same list item on the next run — rather than
// permanent.
//
// Transient (retry, don't consume): rate limit (error 10030), request timeouts /
// context deadline, network failures, and 5xx responses. These clear on their
// own, so burning the item would lose a deliverable offer for no reason.
//
// Permanent (consume + skip, as before): product not found, missing/invalid
// credentials, no Open-API access (10035), bad signature, unparseable URL.
// Retrying these forever would stall the list on a dead item.
//
// Unknown/unclassified errors are treated as permanent: keeping an item that
// fails for an unexplained reason pending would risk an endless no-progress
// loop, which is worse than dropping one item.
// isShopeeCredentialEnrichError identifica falhas de CREDENCIAL no
// enriquecimento (secret errado → Invalid Signature, credencial ausente ou
// incompleta). Diferente de link morto, esse erro repete em TODO item até o
// usuário recadastrar — o chamador pausa a automação em vez de queimar itens.
func isShopeeCredentialEnrichError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid signature") ||
		strings.Contains(msg, "credentials incomplete") ||
		strings.Contains(msg, "credentials not configured")
}

func isTransientShopeeEnrichError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// Permanent signals take precedence over transient ones.
	permanent := []string{
		"not found",
		"credentials not configured",
		"credentials incomplete",
		"invalid signature",
		"invalid value",
		"could not extract product ids",
		"no open api",
		"open api platform access",
		"10035",
	}
	for _, p := range permanent {
		if strings.Contains(msg, p) {
			return false
		}
	}

	transient := []string{
		"rate limit",
		"10030",
		"too many",  // "too many requests"
		"error 429", // HTTP 429: "shopee API error 429: ..."
		"deadline exceeded",
		"timeout",
		"timed out",
		"request failed",
		"connection reset",
		"connection refused",
		"temporarily",
		"try again",
		"eof",
		"shopee api error 5", // 5xx: "shopee API error 500/502/503: ..."
	}
	for _, t := range transient {
		if strings.Contains(msg, t) {
			return true
		}
	}

	return false
}

// buildListModeMessage builds the message for a LIST mode item
func (s *automationService) buildListModeMessage(
	automation *models.Automation,
	item *models.MlLinkListItem,
	preview *models.MLProductPreview,
	shopeeProduct *models.ShopeeOffer,
	affiliateURL string,
	dispatchTemplate string,
	isML bool,
	isShopee bool,
	ctaEntities []models.CTA,
	couponResult *CouponResult,
) string {
	// Select CTA: filtra ctaEntities pela plataforma do produto antes do
	// sorteio (ver pickCTAForPlatform). Se não houver CTA com platform
	// compatível, cai pros CTAs inline da automation.
	cta := ctapicker.PickByPlatform(ctaEntities, isML, isShopee)
	if cta == "" && len(automation.CTAs) > 0 {
		cta = automation.CTAs[rand.IntN(len(automation.CTAs))]
	}

	if isShopee {
		// Shopee: use ShopeeOfferVars.
		//
		// Info propositalmente NÃO é populado aqui. {info} é preenchido a
		// partir da tag info[...] do body da mensagem de gatilho — fluxo
		// que só existe no offer_generate. No LIST mode os itens vêm de
		// uma lista pré-cadastrada sem mensagem de gatilho, então {info}
		// no template renderiza vazio e o RenderTemplate colapsa a linha
		// em branco resultante.
		//
		// Operadores que querem texto fixo no LIST mode devem incluí-lo
		// diretamente no body do dispatch_text (ex.: trocar `{info}` por
		// um literal). Compartilhar o mesmo dispatch_text entre
		// /offer-generate e LIST faz a linha do {info} aparecer preenchida
		// só nos disparos do offer_generate — comportamento esperado.
		vars := text.ShopeeOfferVars{
			Link:          item.OriginURL,
			AffiliateLink: affiliateURL,
			CTA:           cta,
		}
		// Use product data fetched from Shopee API
		if shopeeProduct != nil {
			vars.Title = shopeeProduct.Name
			vars.Price = shopeeProduct.Price
			// Só preenche o preço original quando há desconto real
			// (original > atual). Sem essa guarda, produtos sem desconto saem
			// como "De R$ 99,90 por R$ 99,90" (mesmo valor) — o renderer só
			// remove a linha quando OriginalPrice <= 0. Mesma regra do texto
			// "desc" acima.
			if shopeeProduct.OriginalPrice > shopeeProduct.Price {
				vars.OriginalPrice = shopeeProduct.OriginalPrice
			}
			vars.Discount = shopeeProduct.Discount
			vars.Image = shopeeProduct.Image
			vars.Store = shopeeProduct.ShopName
		}
		// Fallback to item metadata if product fetch failed
		if vars.Title == "" && item.ProductTitle != nil {
			vars.Title = *item.ProductTitle
		}
		if vars.Price == 0 && item.ProductPrice != nil {
			vars.Price = *item.ProductPrice
		}
		if vars.Image == "" && item.ProductImageURL != nil {
			vars.Image = *item.ProductImageURL
		}

		// {info} via campo info do item (coluna "info" da planilha) — mesmo
		// motivo do ramo ML: no LIST mode não há tag info[...] de gatilho.
		if item.Info != nil {
			vars.Info = *item.Info
		}

		// Populate coupon fields and override price with discounted value
		if couponResult != nil && couponResult.Applied {
			vars.CouponCode = couponResult.CouponCode
			vars.CouponText = couponResult.CouponText
			vars.CouponBlock = couponResult.CouponText
			if couponResult.DiscountedPrice > 0 {
				vars.Price = couponResult.DiscountedPrice
			}
		}

		var message string
		if dispatchTemplate != "" {
			message = text.RenderShopeeOfferTemplate(dispatchTemplate, vars)
		} else {
			message = text.RenderShopeeOfferTemplate(text.DefaultShopeeTemplate(), vars)
		}

		// Auto-append CTA if template doesn't contain {cta} variable
		if cta != "" && !strings.Contains(dispatchTemplate, "{cta}") {
			message = message + "\n\n" + cta
		}

		// Auto-append coupon text só quando o template não posiciona o cupom por
		// nenhum placeholder ({cupom_bloco} ou {couponText}) — evita render duplo.
		if couponResult != nil && couponResult.Applied &&
			!strings.Contains(dispatchTemplate, "{cupom_bloco}") && !strings.Contains(dispatchTemplate, "{couponText}") {
			message = message + couponResult.CouponText
		}

		return text.NormalizeOutgoingWhatsAppText(message)
	}

	// ML: use MLOfferVars (original logic)
	vars := text.MLOfferVars{
		Link:          item.OriginURL,
		AffiliateLink: affiliateURL,
		CTA:           cta,
	}

	// Use preview data if available, otherwise fall back to item metadata
	if preview != nil {
		vars.Title = preview.Title
		vars.Price = preview.Price
		// Só com desconto real (original > atual) — evita "De X por X".
		if preview.OriginalPrice != nil && *preview.OriginalPrice > preview.Price {
			vars.OriginalPrice = *preview.OriginalPrice
		}
		vars.Discount = preview.Discount
		vars.Image = preview.Image
		if preview.Condition != nil {
			vars.Condition = *preview.Condition
		}
		if preview.SoldCount != nil {
			vars.SoldCount = *preview.SoldCount
		}
		if preview.Rating != nil {
			vars.Rating = *preview.Rating
		}
		if preview.ReviewCount != nil {
			vars.ReviewCount = *preview.ReviewCount
		}
		if preview.Installments != nil {
			vars.Installments = *preview.Installments
		}
	} else {
		if item.ProductTitle != nil {
			vars.Title = *item.ProductTitle
		}
		if item.ProductPrice != nil {
			vars.Price = *item.ProductPrice
		}
		if item.ProductImageURL != nil {
			vars.Image = *item.ProductImageURL
		}
	}

	// {info} no LIST mode: a lista não tem mensagem de gatilho com a tag
	// info[...] (fluxo do offer_generate), então preenchemos a variável a
	// partir do campo info do item — vindo da coluna "info" da planilha de
	// importação. Vazio mantém a linha do {info} colapsada como antes.
	if item.Info != nil {
		vars.Info = *item.Info
	}

	// Populate coupon fields and override price with discounted value
	if couponResult != nil && couponResult.Applied {
		vars.CouponCode = couponResult.CouponCode
		vars.CouponText = couponResult.CouponText
		vars.CouponBlock = couponResult.CouponText
		if couponResult.DiscountedPrice > 0 {
			vars.Price = couponResult.DiscountedPrice
		}
	}

	var message string
	if dispatchTemplate != "" {
		message = text.RenderMLOfferTemplate(dispatchTemplate, vars)
	} else {
		message = text.RenderMLOfferTemplate(text.DefaultMLTemplate(), vars)
	}

	// Auto-append CTA if template doesn't contain {cta} variable
	if cta != "" && !strings.Contains(dispatchTemplate, "{cta}") {
		message = message + "\n\n" + cta
	}

	// Auto-append coupon text if template doesn't contain {cupom_bloco}
	if couponResult != nil && couponResult.Applied && !strings.Contains(dispatchTemplate, "{cupom_bloco}") {
		message = message + couponResult.CouponText
	}

	return text.NormalizeOutgoingWhatsAppText(message)
}

// pauseForConfigError pausa a automação por erro de CONFIGURAÇÃO permanente
// (sem lista selecionada, sem grupo válido): o erro re-dispara idêntico a cada
// ciclo do cron para sempre — deixar ativa só acumula dispatch_logs de erro
// (21/h medidos em 20/07). MESMO contrato da pausa por credencial (acima,
// isShopeeCredentialEnrichError): logDispatch failed + status paused +
// last_error acionável. A reativação limpa last_error automaticamente quando
// o usuário corrige e religa o toggle (automations_handler, paused→active).
func (s *automationService) pauseForConfigError(ctx context.Context, automation *models.Automation, errMsg string) *models.RunAutomationResult {
	pausedMsg := errMsg + " Automacao pausada; corrija a configuracao e reative."
	s.logger.Warn("Config error in LIST automation — pausing",
		zap.String("automationID", automation.ID),
		zap.String("reason", errMsg))
	s.logDispatch(ctx, automation, "system", "https://error.log", "failed", pausedMsg)
	nowStr := time.Now().UTC().Format(time.RFC3339)
	if upErr := s.automationRepo.UpdateAutomation(ctx, automation.ID, map[string]interface{}{
		"status":        models.AutomationStatusPaused,
		"last_error":    pausedMsg,
		"last_error_at": nowStr,
	}); upErr != nil {
		// Fail-open: a pausa falhar não pode derrubar o run — no pior caso a
		// automação tenta de novo no próximo ciclo, comportamento antigo.
		s.logger.Error("Failed to pause automation on config error",
			zap.String("automationID", automation.ID),
			zap.Error(upErr))
	}
	return &models.RunAutomationResult{
		Success: false,
		Message: pausedMsg,
	}
}
