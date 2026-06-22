package automation

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/ctapicker"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/workers"

	"go.uber.org/zap"
)

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
		errMsg := "Selecione uma Lista de Links para usar no modo Lista."
		s.logDispatch(ctx, automation, "system", "https://error.log", "error", errMsg)
		return &models.RunAutomationResult{
			Success: false,
			Message: errMsg,
		}, nil
	}

	listID := strings.TrimSpace(*automation.MlLinkListID)

	// Validate target groups are configured
	targetGroups := automation.GetTargetGroups()
	if len(targetGroups) == 0 {
		errMsg := "Nenhum grupo de envio configurado. Configure pelo menos um grupo na automacao."
		s.logDispatch(ctx, automation, "system", "https://error.log", "error", errMsg)
		return &models.RunAutomationResult{
			Success: false,
			Message: errMsg,
		}, nil
	}

	// Filter to only valid WhatsApp group JIDs
	validGroups := make([]string, 0, len(targetGroups))
	for _, gid := range targetGroups {
		if strings.Contains(gid, "@g.us") {
			validGroups = append(validGroups, gid)
		}
	}

	if len(validGroups) == 0 {
		errMsg := "Nenhum grupo valido configurado. Os grupos devem terminar com @g.us."
		s.logDispatch(ctx, automation, "system", "https://error.log", "error", errMsg)
		return &models.RunAutomationResult{
			Success: false,
			Message: errMsg,
		}, nil
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
		errMsg := "WhatsApp nao esta conectado. Conecte seu WhatsApp para enviar ofertas."
		s.logDispatch(ctx, automation, "system", "https://error.log", "error", errMsg)
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
	// Defaults to false (keep current behavior: native preview via /api/send/link-custom-preview).
	sendAsImage := false
	if automation.ListSendAsImage != nil {
		sendAsImage = *automation.ListSendAsImage
	}

	// Get next unconsumed items from the list
	items, err := s.mlLinkListItemRepo.GetNextUnconsumed(ctx, listID, itemsPerRun)
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
			items, err = s.mlLinkListItemRepo.GetNextUnconsumed(ctx, listID, itemsPerRun)
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
			dispatchTemplate = dt.Body
		}
	}

	// Coupon resolution moved to per-item via the rules engine — see the
	// ApplyCoupon call inside the per-item loop below. The legacy
	// per-automation preload is gone because the matched coupon depends
	// on the item's price (different rules apply at different price tiers).

	result := &models.RunAutomationResult{
		Success:      true,
		OffersFound:  len(items),
		MessagesSent: 0,
	}

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
				s.logger.Warn("Failed to create Shopee affiliate link after retry, using origin URL",
					zap.String("originURL", originURL),
					zap.Error(err))
			} else if link != nil && link.AffiliateLink != "" {
				affiliateURL = link.AffiliateLink
				if link.ImageURL != "" {
					shopeeImageURL = link.ImageURL
				}
				// Fetch product details (title, price) for the message template
				if link.ItemID > 0 && link.ShopID > 0 {
					product, prodErr := s.shopeeService.GetProduct(ctx, link.ItemID, link.ShopID, automation.UserID)
					if prodErr != nil {
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
			s.logger.Warn("Skipping degraded Shopee list item — no product after retry (avoiding priceless/imageless offer)",
				zap.String("automationID", automation.ID),
				zap.String("originURL", originURL))
			s.logDispatch(ctx, automation, "system", originURL, "skipped",
				"enriquecimento Shopee falhou (sem preco/imagem) - item pulado")
			if err := s.mlLinkListItemRepo.MarkAsConsumed(ctx, []string{item.ID}, automation.ID, nil); err != nil {
				s.logger.Error("Failed to mark skipped item as consumed",
					zap.String("itemID", item.ID),
					zap.Error(err))
			}
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
			if isML && preview != nil {
				offerPrice = preview.Price
				offerStore = "ml"
				// ML doesn't expose a Mall-style flag — leave false.
			} else if isShopee && shopeeProduct != nil {
				offerPrice = shopeeProduct.Price
				offerStore = "shopee"
				productIsOfficial = shopeeProduct.IsMall
			}
			if offerPrice > 0 {
				if cr, err := s.couponHandler.ApplyCoupon(ctx, automation, offerPrice, offerStore, productIsOfficial); err == nil {
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
						CreatedAt:      time.Now().UTC().Format(time.RFC3339),
					}
					created, dlErr := s.dispatchLogRepo.Create(ctx, dl)
					if dlErr == nil && created != nil {
						dispatchLogID = &created.ID
					}
				}
			}

			senderSession := s.resolveSenderForGroup(connectedInstance, groupInstances, groupID)
			if senderSession == "" {
				s.logger.Warn("No connected chip for group, skipping LIST dispatch",
					zap.String("automationID", automation.ID),
					zap.String("groupID", groupID))
				continue
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
			}

			if s.queueClient != nil {
				// Delay acumulado por chip (ver perChipSent acima): o N-ésimo
				// envio DESTE chip espera N*3s, em vez de N*3s global.
				delayMs := time.Duration(perChipSent[senderSession]*3) * time.Second

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
			groupIDsSent = append(groupIDsSent, groupID)
			perChipSent[senderSession]++
			result.MessagesSent++
		}

		// Mark item as consumed (only if we sent to at least one group)
		if len(groupIDsSent) > 0 {
			// Use the first group ID for tracking (the item is consumed for all groups)
			firstGroupID := groupIDsSent[0]
			err := s.mlLinkListItemRepo.MarkAsConsumed(ctx, []string{item.ID}, automation.ID, &firstGroupID)
			if err != nil {
				s.logger.Error("Failed to mark item as consumed",
					zap.String("itemID", item.ID),
					zap.Error(err))
			}
		}
	}

	if result.MessagesSent > 0 {
		result.Message = fmt.Sprintf("Automacao executada: %d mensagem(s) enfileirada(s)", result.MessagesSent)
	} else {
		result.Message = "Nenhuma mensagem foi enviada"
	}

	return result, nil
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
			vars.OriginalPrice = shopeeProduct.OriginalPrice
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

		// Auto-append coupon text if template doesn't contain {cupom_bloco}
		if couponResult != nil && couponResult.Applied && !strings.Contains(dispatchTemplate, "{cupom_bloco}") {
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
		if preview.OriginalPrice != nil {
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
