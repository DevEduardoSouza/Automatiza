package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/deadgroup"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/mercadolivre"
	"automatiza-go/internal/services/shopee"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// Service defines the monitoring service interface
type Service interface {
	ProcessIncomingMessage(ctx context.Context, msg *models.IncomingMessage) error
}

type monitoringService struct {
	automationRepo  repository.AutomationRepository
	instanceRepo    repository.InstanceRepository
	groupRepo       repository.GroupRepository
	dispatchLogRepo repository.DispatchLogRepository
	settingsRepo    repository.SettingsRepository

	shopeeService shopee.Service
	mlService     mercadolivre.Service
	waProvider    whatsapp.WhatsAppProvider

	cache  *cache.Cache
	logger *zap.Logger
}

// NewMonitoringService creates a new monitoring service.
// IMPORTANT: This now requires a WhatsAppProvider instead of waha.Service.
// groupRepo is optional: passing nil disables per-group dispatch routing
// (the service falls back to the user's first connected instance).
func NewMonitoringService(
	automationRepo repository.AutomationRepository,
	instanceRepo repository.InstanceRepository,
	groupRepo repository.GroupRepository,
	dispatchLogRepo repository.DispatchLogRepository,
	settingsRepo repository.SettingsRepository,
	shopeeService shopee.Service,
	mlService mercadolivre.Service,
	waProvider whatsapp.WhatsAppProvider,
	cache *cache.Cache,
	logger *zap.Logger,
) Service {
	return &monitoringService{
		automationRepo:  automationRepo,
		instanceRepo:    instanceRepo,
		groupRepo:       groupRepo,
		dispatchLogRepo: dispatchLogRepo,
		settingsRepo:    settingsRepo,
		shopeeService:   shopeeService,
		mlService:       mlService,
		waProvider:      waProvider,
		cache:           cache,
		logger:          logger,
	}
}

// deadGroupTTL: por quanto tempo um par (sessao, grupo) fica marcado como morto
// apos um erro "not participating". Curto: se o bot voltar ao grupo, o monitor
// volta a tentar apos o TTL, sem intervencao.
const deadGroupTTL = 30 * time.Minute

func deadGroupKey(session, groupID string) string {
	return "deadgroup:" + session + ":" + groupID
}

// isDeadGroup informa se a sessao foi marcada como nao e membro do grupo
// dentro do TTL. Fail-open: cache ausente/erro => false (tenta enviar normal).
func (s *monitoringService) isDeadGroup(ctx context.Context, session, groupID string) bool {
	if s.cache == nil || session == "" || groupID == "" {
		return false
	}
	ok, err := s.cache.Exists(ctx, deadGroupKey(session, groupID))
	if err != nil {
		return false
	}
	return ok
}

// markDeadGroup marca a sessao como nao e membro do grupo por deadGroupTTL.
func (s *monitoringService) markDeadGroup(ctx context.Context, session, groupID string) {
	if s.cache == nil || session == "" || groupID == "" {
		return
	}
	if err := s.cache.Set(ctx, deadGroupKey(session, groupID), "1", deadGroupTTL); err != nil {
		s.logger.Warn("Failed to set dead-group cache key",
			zap.String("session", session), zap.String("groupID", groupID), zap.Error(err))
	}
}

func (s *monitoringService) ProcessIncomingMessage(ctx context.Context, msg *models.IncomingMessage) error {
	// 1. Validate message is from group
	if !strings.HasSuffix(msg.RemoteJID, "@g.us") {
		return nil
	}

	// 2. Ignore own messages
	if msg.FromMe {
		return nil
	}

	// 3. Extract links
	links := s.extractLinks(msg.Body)
	if len(links) == 0 {
		return nil
	}

	s.logger.Info("Processing incoming message with links",
		zap.String("instance", msg.InstanceName),
		zap.String("group", msg.RemoteJID),
		zap.Int("links", len(links)),
		zap.String("bodyPreview", truncateStr(msg.Body, 100)))

	// 4. Content fingerprint dedup (2 min window).
	//
	// Keyed by (group, body-hash) — SEM o instance — pra que a mesma
	// mensagem vista por vários chips do MESMO dono no grupo de origem
	// dispare só 1 processamento. Se N chips do user são membros do grupo
	// monitorado, a WAHA fira N webhooks (mesmo corpo); o primeiro reivindica
	// a key e os outros N-1 caem aqui ANTES do fan-out (link afiliado,
	// GetProduct, download de mídia) — evita envio duplicado/triplicado e N×
	// o custo. claimProductSlot() no loop de dispatch segue como cinto de
	// segurança por (user, destino, canonicalKey).
	contentKey := s.buildContentKey(msg.RemoteJID, msg.Body)
	if !s.claimOnce(ctx, contentKey, 2*time.Minute) {
		s.logger.Debug("Duplicate content detected, skipping")
		return nil
	}

	// 5. Message ID dedup (10 min window). Mesma motivação — keyed por
	// (group, msgID), não por instance: o key.id do WhatsApp é idêntico nos
	// N webhooks (é a mesma mensagem), então colapsa os chips. Se o MessageID
	// vier vazio, pula esse dedup pra não colapsar mensagens distintas numa
	// chave vazia (o dedup de conteúdo acima já cobre esse caso).
	if msg.MessageID != "" {
		msgKey := fmt.Sprintf("msg_dedup:%s:%s", msg.RemoteJID, msg.MessageID)
		if !s.claimOnce(ctx, msgKey, 10*time.Minute) {
			s.logger.Debug("Duplicate message ID detected, skipping")
			return nil
		}
	}

	// 6. Find matching monitoring automations for this group
	automations, err := s.findMatchingAutomations(ctx, msg.InstanceName, msg.RemoteJID)
	if err != nil {
		return err
	}
	if len(automations) == 0 {
		s.logger.Info("No matching monitoring automations for group",
			zap.String("group", msg.RemoteJID))
		return nil
	}

	s.logger.Info("Found matching automations",
		zap.Int("count", len(automations)),
		zap.String("group", msg.RemoteJID))

	// 7. Process each automation
	for i := range automations {
		if err := s.processAutomation(ctx, &automations[i], msg, links); err != nil {
			s.logger.Warn("Failed to process monitoring automation",
				zap.String("automationID", automations[i].ID),
				zap.Error(err))
		}
	}

	return nil
}

func (s *monitoringService) processAutomation(ctx context.Context, automation *models.Automation, msg *models.IncomingMessage, links []string) error {
	// 1. Check time window
	if automation.StartTime != nil && automation.EndTime != nil {
		tw := &models.TimeWindow{
			StartTime: *automation.StartTime,
			EndTime:   *automation.EndTime,
			Timezone:  "America/Sao_Paulo",
		}
		if automation.ScheduleDays != nil {
			tw.Days = automation.ScheduleDays
		}

		if !tw.IsWithinWindow(time.Now()) {
			s.logger.Info("Outside time window, skipping automation",
				zap.String("automationID", automation.ID),
				zap.String("startTime", *automation.StartTime),
				zap.String("endTime", *automation.EndTime))
			return nil
		}
	}

	// 2. Categorize and resolve links
	shopeeLinks, mlLinks := s.categorizeLinks(links)

	// 3. Filter by configured stores
	stores := automation.Stores
	if len(stores) == 0 {
		stores = []string{"shopee"}
	}

	storeSet := make(map[string]bool, len(stores))
	for _, st := range stores {
		storeSet[st] = true
	}

	if !storeSet["shopee"] {
		shopeeLinks = nil
	}
	// When mercadolivre isn't enabled, ML is blocked for this automation:
	// drop the ML links from affiliate generation and remember them so we can
	// strip them from the forwarded body below. The relay forwards msg.Body
	// as-is (only swapping links we affiliated), so an ML link the source
	// included would otherwise be repassed even with ML disabled.
	var blockedMLLinks []string
	if !storeSet["mercadolivre"] {
		blockedMLLinks = mlLinks
		mlLinks = nil
	}

	if len(shopeeLinks) == 0 && len(mlLinks) == 0 {
		s.logger.Info("No links matching configured stores, skipping",
			zap.String("automationID", automation.ID),
			zap.Strings("stores", stores),
			zap.Int("totalLinks", len(links)))
		return nil
	}

	// 4. Generate affiliate links + capture CDN image URL
	affiliateLinks := make(map[string]string)
	var productImageURL string

	// Captured only to enrich + render a dispatch-text template below (opt-in).
	// firstShopeeAff carries ItemID/ShopID so we can call GetProduct for the price;
	// the ML pair carries the affiliate link for the ML template.
	var firstShopeeAff *models.ShopeeAffiliateLink
	var firstShopeeOriginalLink, firstMLOriginalLink, firstMLAffiliateLink string

	for _, link := range shopeeLinks {
		if s.shopeeService != nil {
			aff, err := s.shopeeService.GenerateAffiliateLink(ctx, link, automation.UserID)
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "Invalid Signature") ||
					strings.Contains(errStr, "credentials incomplete") {
					s.logger.Warn("Shopee credential error, pausing automation",
						zap.String("automationID", automation.ID),
						zap.String("userID", automation.UserID),
						zap.Error(err))

					ck := s.extractCanonicalKey(links)
					st := s.detectStore(links)
					s.logDispatch(ctx, automation, "", link,
						"failed", "Credenciais Shopee inválidas - reconfigure App ID e Senha",
						ck, st)

					now := time.Now().UTC().Format(time.RFC3339)
					_ = s.automationRepo.UpdateAutomation(ctx, automation.ID, map[string]interface{}{
						"status":        models.AutomationStatusPaused,
						"last_error":    "Credenciais Shopee inválidas (Invalid Signature) - automação pausada automaticamente",
						"last_error_at": now,
					})

					return nil
				}

				s.logger.Warn("Failed to generate Shopee affiliate link, keeping original",
					zap.String("automationID", automation.ID),
					zap.String("userID", automation.UserID),
					zap.String("originalLink", link),
					zap.Error(err))
			} else if aff != nil && aff.AffiliateLink != "" {
				affiliateLinks[link] = aff.AffiliateLink
				if aff.ImageURL != "" && productImageURL == "" {
					productImageURL = aff.ImageURL
				}
				if firstShopeeAff == nil {
					firstShopeeAff = aff
					firstShopeeOriginalLink = link
				}
			}
		}
	}

	for _, link := range mlLinks {
		if s.mlService != nil {
			aff, err := s.mlService.CreateAffiliateLink(ctx, link, automation.UserID)
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "Invalid Signature") ||
					strings.Contains(errStr, "credentials incomplete") ||
					strings.Contains(errStr, "invalid_token") {
					s.logger.Warn("ML credential error, pausing automation",
						zap.String("automationID", automation.ID),
						zap.String("userID", automation.UserID),
						zap.Error(err))

					ck := s.extractCanonicalKey(links)
					st := s.detectStore(links)
					s.logDispatch(ctx, automation, "", link,
						"failed", "Credenciais Mercado Livre inválidas - reconfigure suas credenciais",
						ck, st)

					now := time.Now().UTC().Format(time.RFC3339)
					_ = s.automationRepo.UpdateAutomation(ctx, automation.ID, map[string]interface{}{
						"status":        models.AutomationStatusPaused,
						"last_error":    "Credenciais ML inválidas - automação pausada automaticamente",
						"last_error_at": now,
					})

					return nil
				}

				s.logger.Warn("Failed to generate ML affiliate link, keeping original",
					zap.String("automationID", automation.ID),
					zap.String("userID", automation.UserID),
					zap.String("originalLink", link),
					zap.Error(err))
			} else if aff != nil && aff.AffiliateLink != "" {
				affiliateLinks[link] = aff.AffiliateLink
				if firstMLAffiliateLink == "" {
					firstMLOriginalLink = link
					firstMLAffiliateLink = aff.AffiliateLink
				}
			}
		}
	}

	// 5. Build the outgoing body.
	//
	// Opt-in dispatch text: when the automation has a dispatch_text_id, render
	// that template as the body (enriching the product for price/title) INSTEAD
	// of relaying the source text. Sem dispatch text → comportamento histórico:
	// repassa msg.Body com as regras de rodapé + troca de links por afiliado.
	processedText := msg.Body
	dispatchTextRendered := ""
	if automation.DispatchTextID != nil && *automation.DispatchTextID != "" && s.settingsRepo != nil {
		dispatchTextRendered = s.renderMonitorDispatchText(
			ctx, automation, firstShopeeAff, firstShopeeOriginalLink, firstMLOriginalLink, firstMLAffiliateLink, productImageURL)
	}

	if dispatchTextRendered != "" {
		// Template is self-contained (already carries the affiliate link), so we
		// skip the source-text rules and link substitution entirely.
		processedText = dispatchTextRendered
	} else {
		// Get monitoring text rules if configured
		if automation.MonitoringTextRules != nil {
			processedText = text.ProcessarMensagem(processedText, automation.MonitoringTextRules)
		}

		// 6. Replace original links with affiliate links
		for orig, aff := range affiliateLinks {
			processedText = strings.ReplaceAll(processedText, orig, aff)
		}

		// 6b. Block ML relay: strip any ML links the source posted when this
		// automation has mercadolivre disabled. Pure-ML messages were already
		// dropped by the store filter above; this covers mixed messages where a
		// Shopee link keeps the relay alive but an ML link rode along in the body.
		for _, mlLink := range blockedMLLinks {
			processedText = strings.ReplaceAll(processedText, mlLink, "")
		}
	}

	// 7. Normalize text
	processedText = text.NormalizeOutgoingWhatsAppText(processedText)
	if processedText == "" {
		s.logger.Info("Text empty after normalization, skipping",
			zap.String("automationID", automation.ID))
		return nil
	}

	// 8. Resolve image via multi-layer strategy:
	//   Layer 1: CDN Shopee URL (high quality ~800px, from productOfferV2)
	//   Layer 2: WAHA download (local cache → re-download from WhatsApp servers)
	//   Layer 3: Text-only fallback
	var imageBase64 string
	var imageMimetype string
	var imageURL string

	// Layer 1: Use CDN image URL from Shopee affiliate link result
	if productImageURL != "" {
		imageURL = productImageURL
		s.logger.Info("Using Shopee CDN image URL",
			zap.String("automationID", automation.ID),
			zap.String("imageURL", imageURL))
	}

	// Layer 2: WAHA media download (only if no CDN image)
	if imageURL == "" && msg.HasMedia && (msg.MediaType == "image" || strings.HasPrefix(msg.MimeType, "image/")) {
		if msg.MediaBase64 != "" {
			// Use inline base64 directly (link preview thumbnail)
			imageBase64 = msg.MediaBase64
			imageMimetype = msg.MimeType
			if imageMimetype == "" {
				imageMimetype = "image/jpeg"
			}
			s.logger.Info("Using inline base64 image (link preview)",
				zap.String("automationID", automation.ID),
				zap.Int("base64Len", len(imageBase64)),
				zap.String("mimetype", imageMimetype))
		} else if s.waProvider != nil {
			// Try downloading via WAHA media URL (fast, uses local cache)
			if msg.MediaURL != "" {
				result, dlErr := s.waProvider.DownloadMediaByURL(ctx, msg.MediaURL)
				if dlErr == nil && result != nil && len(result.Data) > 0 {
					imageBase64 = base64.StdEncoding.EncodeToString(result.Data)
					imageMimetype = result.Mimetype
					if imageMimetype == "" {
						imageMimetype = msg.MimeType
					}
					s.logger.Info("Image media downloaded successfully",
						zap.String("automationID", automation.ID),
						zap.Int("dataLen", len(result.Data)),
						zap.String("mimetype", imageMimetype))
				} else if dlErr != nil {
					s.logger.Warn("WAHA local download failed, trying re-download from WhatsApp servers",
						zap.String("automationID", automation.ID),
						zap.Error(dlErr))
				}
			}

			// Fallback: re-download from WhatsApp servers (bypasses expired local cache)
			if imageBase64 == "" && msg.MessageID != "" && msg.RemoteJID != "" {
				result, reErr := s.waProvider.RedownloadMedia(ctx, msg.InstanceName, msg.RemoteJID, msg.MessageID)
				if reErr == nil && result != nil && len(result.Data) > 0 {
					imageBase64 = base64.StdEncoding.EncodeToString(result.Data)
					imageMimetype = result.Mimetype
					if imageMimetype == "" {
						imageMimetype = msg.MimeType
					}
					s.logger.Info("Image re-downloaded from WhatsApp servers",
						zap.String("automationID", automation.ID),
						zap.Int("dataLen", len(result.Data)),
						zap.String("mimetype", imageMimetype))
				} else if reErr != nil {
					s.logger.Warn("All image download attempts failed, sending text only",
						zap.String("automationID", automation.ID),
						zap.Error(reErr))
				}
			}
		}
	}

	// Layer 3: No image available → imageURL and imageBase64 stay empty → text-only send

	// 9. Resolve the automation owner's connected WAHA session for sending.
	// We must NOT use msg.InstanceName (the session that received the webhook),
	// because that belongs to whichever user's session captured the incoming message.
	instances, err := s.instanceRepo.GetUserInstances(ctx, automation.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user instances: %w", err)
	}
	var fallbackInstance *models.Instance
	for i := range instances {
		if instances[i].Status == "connected" {
			fallbackInstance = &instances[i]
			break
		}
	}
	if fallbackInstance == nil {
		s.logger.Warn("No connected WhatsApp instance for automation owner, skipping",
			zap.String("automationID", automation.ID),
			zap.String("userID", automation.UserID))
		return nil
	}

	// Map connected chips (db id → session) and the automation's pinned chip
	// preference, so a destination group can be dispatched from the chip the
	// user actually selected instead of the arbitrary group owner. Mirrors the
	// offer_generate chip preference (migrations 049/050).
	connectedSessionByID := make(map[string]string, len(instances))
	for i := range instances {
		if instances[i].Status == "connected" {
			connectedSessionByID[fmt.Sprintf("%d", instances[i].ID)] = instances[i].InstanceID
		}
	}
	preferredChipIDs := make([]string, 0, len(automation.InstanceIDs)+1)
	for _, id := range automation.InstanceIDs {
		if _, ok := connectedSessionByID[id]; ok {
			preferredChipIDs = append(preferredChipIDs, id)
		}
	}
	if len(preferredChipIDs) == 0 && automation.InstanceID != nil && *automation.InstanceID != "" {
		if _, ok := connectedSessionByID[*automation.InstanceID]; ok {
			preferredChipIDs = append(preferredChipIDs, *automation.InstanceID)
		}
	}

	// 10. Get destination groups
	destinationGroups := automation.SendGroups
	if len(destinationGroups) == 0 {
		destinationGroups = automation.GetTargetGroups()
	}

	if len(destinationGroups) == 0 {
		s.logger.Warn("No destination groups configured",
			zap.String("automationID", automation.ID))
		return nil
	}

	// Per-group instance lookup. Returns nil when the flag is off or the
	// repo is unavailable; in that case dispatchSenderForGroup collapses to
	// the legacy fallback path (single chip handles every group).
	var groupInstances map[string]*models.Instance
	if config.GetFeatureFlags().UseMultiInstanceDispatch && s.groupRepo != nil {
		gi, gErr := s.groupRepo.GetInstancesForGroups(ctx, automation.UserID, destinationGroups)
		if gErr != nil {
			s.logger.Warn("Failed to load per-group instances; falling back to singleton routing",
				zap.String("userID", automation.UserID),
				zap.Error(gErr))
		} else {
			groupInstances = gi
		}
	}

	// Per-group participation (participating_instance_ids), loaded only when the
	// automation pins a chip. Used to honor the pinned chip only when it's an
	// actual member of the destination group — otherwise WAHA rejects the send
	// with "not participating", so we keep the owner/fallback routing.
	var groupParticipants map[string][]string
	if len(preferredChipIDs) > 0 && s.groupRepo != nil {
		if grps, gErr := s.groupRepo.GetGroupsByWAGroupIDs(ctx, automation.UserID, destinationGroups); gErr != nil {
			s.logger.Warn("Failed to load destination group participants; keeping owner routing",
				zap.String("automationID", automation.ID), zap.Error(gErr))
		} else {
			groupParticipants = make(map[string][]string, len(grps))
			for _, g := range grps {
				groupParticipants[g.WAGroupID] = g.ParticipatingInstanceIDs
			}
		}
	}

	// Extract first affiliate URL for link custom preview
	var firstAffiliateURL string
	for _, v := range affiliateLinks {
		firstAffiliateURL = v
		break
	}

	// 12. Get deduplication settings
	dedupeHours := automation.GetDedupeHours()
	canonicalKey := s.extractCanonicalKey(links)
	store := s.detectStore(links)

	// 13. Send to destination groups
	for _, groupID := range destinationGroups {
		// Skip if same group
		if groupID == msg.RemoteJID {
			continue
		}

		// Resolve sender per group: prefer the group's owning chip when
		// connected, otherwise fall back to the user's first connected chip.
		senderSession := fallbackInstance.InstanceID
		if config.GetFeatureFlags().UseMultiInstanceDispatch && groupInstances != nil {
			if owner, ok := groupInstances[groupID]; ok && owner != nil && owner.IsConnected() {
				senderSession = owner.InstanceID
			}
		}
		// Prefer the chip the user pinned on the automation when it's a
		// connected member of this destination group. Makes the "Número"
		// selection control the sender instead of the arbitrary group owner;
		// stays on owner/fallback when the pinned chip isn't in the group.
		if pinned := preferredParticipantSession(preferredChipIDs, groupParticipants[groupID], connectedSessionByID); pinned != "" {
			senderSession = pinned
		}

		// Pula grupos onde a sessao sabidamente nao e mais membro. Sem isso, o
		// monitor re-mira o grupo morto a cada mensagem do canal de origem, o
		// WAHA responde 500 "not participating" e o produto e marcado failed
		// (com release do slot -> retry eterno). Marcado no envio que falhar
		// abaixo, com TTL curto: se o bot voltar ao grupo, volta a tentar.
		if s.isDeadGroup(ctx, senderSession, groupID) {
			s.logger.Warn("Skipping group — session not participating (cached)",
				zap.String("automationID", automation.ID),
				zap.String("toGroup", groupID))
			s.logDispatch(ctx, automation, groupID, links[0], "skipped", "bot nao participa do grupo", canonicalKey, store)
			continue
		}

		// Atomic dedup claim: SetNX + DB fallback (prevents race conditions)
		if !s.claimProductSlot(ctx, automation.UserID, groupID, canonicalKey, dedupeHours) {
			s.logger.Info("Product already sent (dedup), skipping",
				zap.String("automationID", automation.ID),
				zap.String("toGroup", groupID),
				zap.String("canonicalKey", canonicalKey),
				zap.Int("dedupeHours", dedupeHours))
			s.logDispatch(ctx, automation, groupID, links[0], "skipped", "dedupe", canonicalKey, store)
			continue
		}

		// Apply delay
		delay := 1 + rand.IntN(3) // 1-3 seconds random
		if automation.DelaySeconds != nil && *automation.DelaySeconds > 0 {
			delay = *automation.DelaySeconds
		}
		time.Sleep(time.Duration(delay) * time.Second)

		// Send message via WhatsAppProvider using the automation owner's session
		// Priority 1: Link custom preview (source had WhatsApp link preview)
		// Priority 2: Image + caption (current behavior)
		// Priority 3: Text only
		sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
		var err error
		var sent bool

		// "Enviar como imagem" (MONITORING, default FALSE): ligado pula o
		// preview nativo e força imagem baixada + legenda; desligado mantém o
		// preview-first (comportamento atual). nil = false (default do schema).
		monitoringSendAsImage := automation.MonitoringSendAsImage != nil && *automation.MonitoringSendAsImage

		// Priority 1: Link custom preview (source had WhatsApp link preview)
		if !monitoringSendAsImage && firstAffiliateURL != "" && msg.LinkPreviewTitle != "" {
			_, err = s.waProvider.SendLinkCustomPreview(sendCtx, whatsapp.SendLinkPreviewInput{
				SessionName: senderSession,
				ChatID:      groupID,
				Text:        processedText,
				URL:         firstAffiliateURL,
				Title:       msg.LinkPreviewTitle,
				Description: msg.LinkPreviewDescription,
				ImageURL:    imageURL,
				ImageBase64: imageBase64,
				Mimetype:    imageMimetype,
			})
			if err != nil {
				s.logger.Warn("Link preview failed, falling back",
					zap.String("groupID", groupID),
					zap.Error(err))
				err = nil
			} else {
				sent = true
			}
		}

		// Priority 2: Image + caption
		if !sent && (imageURL != "" || imageBase64 != "") {
			_, err = s.waProvider.SendImage(sendCtx, whatsapp.SendImageInput{
				SessionName: senderSession,
				ChatID:      groupID,
				Caption:     processedText,
				ImageURL:    imageURL,
				ImageBase64: imageBase64,
				Mimetype:    imageMimetype,
			})
			if err == nil {
				sent = true
			}
		}

		// Priority 3: Text only
		if !sent && err == nil {
			_, err = s.waProvider.SendText(sendCtx, whatsapp.SendTextInput{
				SessionName: senderSession,
				ChatID:      groupID,
				Text:        processedText,
			})
		}
		sendCancel()

		if err != nil {
			// Bot removido do grupo: nao adianta retentar e nem manter o slot
			// para retry. Marca o grupo como morto (TTL curto) para os proximos
			// produtos saltarem cedo no early-skip acima, e registra skipped.
			if deadgroup.IsNotParticipatingError(err) {
				s.markDeadGroup(ctx, senderSession, groupID)
				s.logDispatch(ctx, automation, groupID, links[0], "skipped", "bot nao participa do grupo", canonicalKey, store)
				s.logger.Warn("Session not participating in group, marking dead (no retry)",
					zap.String("automationID", automation.ID),
					zap.String("groupID", groupID))
				continue
			}
			// Release dedup slot so the product can be retried on the next cycle
			s.releaseProductSlot(ctx, automation.UserID, groupID, canonicalKey)
			s.logDispatch(ctx, automation, groupID, links[0], "failed", err.Error(), canonicalKey, store)
			s.logger.Error("Failed to send message",
				zap.String("groupID", groupID),
				zap.Error(err))
			continue
		}

		s.logDispatch(ctx, automation, groupID, links[0], "sent", "", canonicalKey, store)

		s.logger.Info("Message forwarded successfully",
			zap.String("fromGroup", msg.RemoteJID),
			zap.String("toGroup", groupID),
			zap.String("automationID", automation.ID),
			zap.String("canonicalKey", canonicalKey))
	}

	return nil
}

// renderMonitorDispatchText renders the automation's configured dispatch-text
// template for a monitored offer, so the relay can send the operator's own copy
// instead of the source group's text. Shopee is enriched via GetProduct (the
// item/shop id comes from the affiliate step) so {preco}/{nome_produto} fill in;
// ML uses the affiliate preview (best-effort). Returns "" when there's no
// template or no affiliated product — the caller then relays the source text.
func (s *monitoringService) renderMonitorDispatchText(
	ctx context.Context,
	automation *models.Automation,
	shopeeAff *models.ShopeeAffiliateLink,
	shopeeOriginalLink, mlOriginalLink, mlAffiliateLink, fallbackImageURL string,
) string {
	dispatchText, err := s.settingsRepo.GetActiveDispatchTextByID(ctx, *automation.DispatchTextID, automation.UserID)
	if err != nil || dispatchText == nil {
		s.logger.Warn("Failed to load dispatch text for monitor, relaying source text",
			zap.String("automationID", automation.ID),
			zap.Error(err))
		return ""
	}
	body := dispatchText.Body
	if len(dispatchText.Copies) > 0 {
		body = dispatchText.Copies[rand.IntN(len(dispatchText.Copies))] //nolint:gosec // G404: non-security random for copy rotation
	}
	if body == "" {
		return ""
	}

	cta := ""
	if len(automation.CTAs) > 0 {
		cta = automation.CTAs[rand.IntN(len(automation.CTAs))] //nolint:gosec // G404: non-security random for CTA rotation
	}

	if shopeeAff != nil && shopeeAff.AffiliateLink != "" {
		return s.renderShopeeDispatchTemplate(ctx, automation, body, cta, shopeeAff, shopeeOriginalLink)
	}
	if mlAffiliateLink != "" && s.mlService != nil {
		return s.renderMLDispatchTemplate(ctx, automation, body, cta, mlOriginalLink, mlAffiliateLink, fallbackImageURL)
	}
	return ""
}

// renderShopeeDispatchTemplate enriches the Shopee product via GetProduct
// (best-effort — renders with at least the affiliate link if it fails) and
// fills the dispatch-text template.
func (s *monitoringService) renderShopeeDispatchTemplate(
	ctx context.Context,
	automation *models.Automation,
	body, cta string,
	shopeeAff *models.ShopeeAffiliateLink,
	shopeeOriginalLink string,
) string {
	vars := text.ShopeeOfferVars{
		AffiliateLink: shopeeAff.AffiliateLink,
		Link:          shopeeOriginalLink,
		Image:         shopeeAff.ImageURL,
		Store:         "Shopee",
		CTA:           cta,
	}
	if s.shopeeService != nil && shopeeAff.ItemID > 0 && shopeeAff.ShopID > 0 {
		product, prodErr := s.shopeeService.GetProduct(ctx, shopeeAff.ItemID, shopeeAff.ShopID, automation.UserID)
		if prodErr != nil {
			s.logger.Warn("Failed to enrich Shopee product for monitor dispatch text, rendering without price",
				zap.String("automationID", automation.ID), zap.Error(prodErr))
		} else if product != nil {
			vars.Title = product.Name
			vars.Price = product.Price
			vars.Discount = product.Discount
			if product.OriginalPrice > product.Price {
				vars.OriginalPrice = product.OriginalPrice
			}
			if product.Image != "" {
				vars.Image = product.Image
			}
		}
	}
	return text.RenderShopeeOfferTemplate(body, vars)
}

// renderMLDispatchTemplate fills the dispatch-text template for an ML offer.
// The preview is best-effort (often blocked by ML anti-bot — price may be empty).
func (s *monitoringService) renderMLDispatchTemplate(
	ctx context.Context,
	automation *models.Automation,
	body, cta, mlOriginalLink, mlAffiliateLink, fallbackImageURL string,
) string {
	vars := text.MLOfferVars{
		AffiliateLink: mlAffiliateLink,
		Link:          mlOriginalLink,
		Image:         fallbackImageURL,
		CTA:           cta,
	}
	if preview, prevErr := s.mlService.GetProductPreview(ctx, mlOriginalLink, automation.UserID); prevErr == nil && preview != nil {
		vars.Title = preview.Title
		vars.Price = preview.Price
		if preview.Image != "" {
			vars.Image = preview.Image
		}
		if preview.OriginalPrice != nil && *preview.OriginalPrice > preview.Price {
			vars.OriginalPrice = *preview.OriginalPrice
		}
	}
	return text.RenderMLOfferTemplate(body, vars)
}

// preferredParticipantSession returns the session of the first pinned chip
// that is both connected and a participant of the group. Returns "" when no
// pinned chip qualifies, signaling the caller to keep the default routing
// (group owner / first-connected fallback). Guarding on participation avoids
// forcing a send from a chip that isn't in the group (WAHA "not participating").
func preferredParticipantSession(preferredChipIDs, participants []string, connectedSessionByID map[string]string) string {
	if len(preferredChipIDs) == 0 || len(participants) == 0 {
		return ""
	}
	participantSet := make(map[string]struct{}, len(participants))
	for _, p := range participants {
		participantSet[p] = struct{}{}
	}
	for _, id := range preferredChipIDs {
		session, connected := connectedSessionByID[id]
		if !connected {
			continue
		}
		if _, ok := participantSet[id]; ok {
			return session
		}
	}
	return ""
}

func (s *monitoringService) findMatchingAutomations(ctx context.Context, instanceName, groupJID string) ([]models.Automation, error) {
	// Get all monitoring automations for this group (across ALL users).
	automations, err := s.automationRepo.GetMonitoringAutomationsForGroup(ctx, groupJID)
	if err != nil {
		return nil, err
	}

	// A popular source group is delivered by EVERY member's session, so a
	// webhook for it can fire automations of users whose own chip isn't in the
	// group anymore — they'd keep "copying" a group they already left (real bug
	// reported by paollas2kaiber). Scope the match to the user whose session
	// actually delivered this message (= whose chip is in the source group).
	ownerUserID := ""
	if inst, ierr := s.instanceRepo.GetInstanceByName(ctx, instanceName); ierr == nil && inst != nil {
		ownerUserID = inst.UserID
	} else {
		// Couldn't resolve the session owner — fail open (group-only match)
		// rather than drop a legitimate relay. Rare; logged for visibility.
		s.logger.Warn("Could not resolve session owner for monitoring match; matching by group only",
			zap.String("instance", instanceName),
			zap.String("group", groupJID))
	}

	var matching []models.Automation
	for _, auto := range automations {
		if !auto.IsActive() {
			continue
		}
		// Only the owner of the delivering session — i.e. the user whose chip is
		// actually in the source group — should fire here.
		if ownerUserID != "" && auto.UserID != ownerUserID {
			continue
		}
		matching = append(matching, auto)
	}

	return matching, nil
}

var (
	httpURLRegex   = regexp.MustCompile(`https?://[^\s\n\r*_~` + "`" + `]+`)
	shopeeURLRegex = regexp.MustCompile(`(?i)(shopee\.com\.br|shope\.ee|shp\.ee)`)
	// Keep in sync with text.IsMLLink. meli.la is the official ML affiliate
	// short domain (meli.co was an outdated guess that almost never appears);
	// ml.com.br is the legacy short host. Missing meli.la here let ML offers
	// slip past the per-automation store filter in monitoring relays.
	mlURLRegex = regexp.MustCompile(`(?i)(mercadolivre\.com\.br|mercadolibre\.com|ml\.com\.br|meli\.la|meli\.co)`)
)

func (s *monitoringService) extractLinks(text string) []string {
	matches := httpURLRegex.FindAllString(text, -1)
	// Clean up trailing punctuation
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		m = strings.TrimRight(m, ".,;:!?)")
		result = append(result, m)
	}
	return result
}

func (s *monitoringService) categorizeLinks(links []string) (shopeeLinks, mlLinks []string) {
	for _, link := range links {
		if shopeeURLRegex.MatchString(link) {
			shopeeLinks = append(shopeeLinks, link)
		} else if mlURLRegex.MatchString(link) {
			mlLinks = append(mlLinks, link)
		}
	}
	return
}

// buildContentKey monta a chave de dedup de conteúdo por (grupo, body-hash),
// SEM o instance de propósito: a mesma mensagem entregue a vários chips do
// mesmo dono no grupo de origem colapsa numa chave só (dedup multi-chip).
func (s *monitoringService) buildContentKey(groupID, body string) string {
	hash := sha256.Sum256([]byte(groupID + ":" + body))
	return "content:" + hex.EncodeToString(hash[:])
}

func (s *monitoringService) isDuplicateContent(ctx context.Context, key string) bool {
	if s.cache == nil {
		return false
	}
	val, err := s.cache.Get(ctx, key)
	return err == nil && val != ""
}

func (s *monitoringService) markContentProcessed(ctx context.Context, key string) {
	if s.cache != nil {
		if cacheErr := s.cache.Set(ctx, key, "1", 2*time.Minute); cacheErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to set content dedup cache key", cacheErr, map[string]string{
				"module": "monitoring", "operation": "content_dedup_set", "cache_policy": "fail-open",
			})
		}
	}
}

func (s *monitoringService) extractCanonicalKey(links []string) string {
	// Pick the best link for deduplication
	// Prefer product links (with shop/item IDs) — skip coupon/campaign links
	for _, link := range links {
		if shopeeURLRegex.MatchString(link) {
			if s.shopeeService != nil {
				// Try direct extraction first
				shopID, itemID, err := s.shopeeService.ExtractProductIDs(link)
				if err == nil {
					return fmt.Sprintf("shopee:%d:%d", shopID, itemID)
				}
				// For short URLs, try expanding to get product IDs
				lower := strings.ToLower(link)
				if strings.Contains(lower, "s.shopee.com.br/") || strings.Contains(lower, "shp.ee/") || strings.Contains(lower, "shope.ee/") {
					expanded, expandErr := s.shopeeService.ExpandURL(context.Background(), link)
					if expandErr == nil {
						shopID, itemID, err = s.shopeeService.ExtractProductIDs(expanded)
						if err == nil {
							return fmt.Sprintf("shopee:%d:%d", shopID, itemID)
						}
					}
				}
				// No product IDs → this is a coupon/campaign link, skip it for dedup
			}
		} else if mlURLRegex.MatchString(link) {
			if s.mlService != nil {
				productID, err := s.mlService.ExtractProductID(link)
				if err == nil {
					return "ml:" + productID
				}
			}
		}
	}

	// Fallback: hash the last product-like link (product links appear after coupon links)
	for i := len(links) - 1; i >= 0; i-- {
		link := links[i]
		if shopeeURLRegex.MatchString(link) || mlURLRegex.MatchString(link) {
			hash := sha256.Sum256([]byte(link))
			return "link:" + hex.EncodeToString(hash[:8])
		}
	}

	return ""
}

// claimOnce marca `key` como processado por `ttl` de forma ATÔMICA e retorna
// true só pro PRIMEIRO caller na janela. Substitui o par isDuplicateContent()
// (GET) + markContentProcessed()/Set() (SET), cujo gap check-then-set deixava
// entregas duplicadas concorrentes passarem todas: a mesma mensagem de origem
// vista por N chips do mesmo dono no grupo monitor (a WAHA fira um webhook por
// chip, todos com o mesmo key.id) passava o GET antes de qualquer SET, então
// os N processamentos faziam fan-out da mesma oferta pros grupos de destino,
// gerando envios duplicados/triplicados no mesmo segundo. O SetNX fecha o gap.
//
// Fail-open: sem cache, ou em erro de Redis, retorna true (processa) pra nunca
// descartar mensagem real; claimProductSlot ainda barra duplicação no dispatch
// nesse caso degradado.
func (s *monitoringService) claimOnce(ctx context.Context, key string, ttl time.Duration) bool {
	if s.cache == nil {
		return true
	}
	claimed, err := s.cache.SetNX(ctx, key, "1", ttl)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "monitoring dedup SetNX failed, processing fail-open", err, map[string]string{
			"module": "monitoring", "operation": "dedup_claim",
		})
		return true
	}
	return claimed
}

// claimProductSlot atomically checks and claims a dedup slot using Redis SetNX + DB fallback.
// Returns true if the slot was claimed (product not yet sent), false if duplicate.
func (s *monitoringService) claimProductSlot(ctx context.Context, userID, groupID, canonicalKey string, hours int) bool {
	if canonicalKey == "" {
		return true
	}

	cacheKey := fmt.Sprintf("prod_dedup:%s:%s:%s", userID, groupID, canonicalKey)
	ttl := time.Duration(hours) * time.Hour

	// Fast path: atomic Redis SetNX (check + set in one operation)
	if s.cache != nil {
		wasSet, err := s.cache.SetNX(ctx, cacheKey, "1", ttl)
		if err == nil {
			if !wasSet {
				return false // Already claimed by another goroutine or previous send
			}
			// We claimed the slot in Redis. Still check DB for stale records
			// (Redis may have lost the key due to restart/eviction).
		} else {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Monitoring dedup SetNX failed", err, map[string]string{
				"module": "monitoring", "operation": "product_dedup_setnx", "cache_policy": "fall-through-db",
			})
		}
	}

	// Slow path: check database for historical dispatches
	if s.dispatchLogRepo != nil {
		log, err := s.dispatchLogRepo.GetRecentByCanonicalKey(ctx, userID, groupID, canonicalKey, time.Duration(hours)*time.Hour)
		if err != nil {
			s.logger.Warn("Monitoring dedup DB check failed",
				zap.String("userID", userID),
				zap.String("groupID", groupID),
				zap.String("canonicalKey", canonicalKey),
				zap.Error(err))
		} else if log != nil {
			// DB has a recent dispatch — this is a duplicate. Ensure Redis is populated.
			if s.cache != nil {
				if cacheErr := s.cache.Set(ctx, cacheKey, "1", ttl); cacheErr != nil {
					s.logger.Debug("Monitoring dedup cache backfill failed", zap.Error(cacheErr))
				}
			}
			return false
		}
	}

	// Ensure Redis has the claim (in case SetNX failed due to error above)
	if s.cache != nil {
		if cacheErr := s.cache.Set(ctx, cacheKey, "1", ttl); cacheErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to ensure product dedup cache key", cacheErr, map[string]string{
				"module": "monitoring", "operation": "product_dedup_ensure", "cache_policy": "fail-open",
			})
		}
	}

	return true
}

// releaseProductSlot removes a dedup slot from Redis, allowing the product to be retried.
// Called when a send fails so transient errors don't permanently block products.
func (s *monitoringService) releaseProductSlot(ctx context.Context, userID, groupID, canonicalKey string) {
	if canonicalKey == "" || s.cache == nil {
		return
	}

	cacheKey := fmt.Sprintf("prod_dedup:%s:%s:%s", userID, groupID, canonicalKey)
	if err := s.cache.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to release dedup slot after send failure",
			zap.String("userID", userID),
			zap.String("groupID", groupID),
			zap.String("canonicalKey", canonicalKey),
			zap.Error(err))
	}
}

// detectStore returns the store name based on the links found in the message.
func (s *monitoringService) detectStore(links []string) string {
	for _, link := range links {
		if shopeeURLRegex.MatchString(link) {
			return "shopee"
		}
		if mlURLRegex.MatchString(link) {
			return "mercadolivre"
		}
	}
	return ""
}

func (s *monitoringService) logDispatch(ctx context.Context, automation *models.Automation, groupID, productURL, status, errorMsg, canonicalKey, store string) {
	if s.dispatchLogRepo == nil {
		return
	}

	// Check feature flag: skip if Go should not create dispatch logs
	featureFlags := config.GetFeatureFlags()
	if !featureFlags.ShouldCreateDispatchLog() {
		s.logger.Debug("Skipping dispatch log creation (feature flag disabled)",
			zap.String("automationID", automation.ID),
			zap.String("groupID", groupID),
			zap.String("status", status))
		return
	}

	automationType := "monitoring"
	dispatchLog := &models.DispatchLog{
		UserID:         automation.UserID,
		AutomationID:   &automation.ID,
		AutomationName: &automation.Name,
		GroupID:        &groupID,
		ProductURL:     &productURL,
		Status:         status,
		Type:           &automationType,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	if canonicalKey != "" {
		dispatchLog.CanonicalKey = &canonicalKey
	}
	if store != "" {
		dispatchLog.Store = &store
	}

	if errorMsg != "" {
		dispatchLog.Error = &errorMsg
	}

	// Use dedicated timeout to avoid inheriting caller's potentially-exhausted deadline
	logCtx, logCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer logCancel()

	// W34: Capture dispatch log creation errors with Sentry tags
	if _, err := s.dispatchLogRepo.Create(logCtx, dispatchLog); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to create dispatch log", err, map[string]string{
			"module":       "monitoring",
			"operation":    "dispatch_log_create",
			"automationID": automation.ID,
			"groupID":      groupID,
			"status":       status,
		})
	}
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
