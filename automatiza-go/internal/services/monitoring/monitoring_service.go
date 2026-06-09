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

	// 4. Content fingerprint dedup (2 min window)
	contentKey := s.buildContentKey(msg.InstanceName, msg.RemoteJID, msg.Body)
	if s.isDuplicateContent(ctx, contentKey) {
		s.logger.Debug("Duplicate content detected, skipping")
		return nil
	}
	s.markContentProcessed(ctx, contentKey)

	// 5. Message ID dedup (10 min window)
	msgKey := fmt.Sprintf("msg_dedup:%s:%s:%s", msg.InstanceName, msg.RemoteJID, msg.MessageID)
	if s.isDuplicateContent(ctx, msgKey) {
		s.logger.Debug("Duplicate message ID detected, skipping")
		return nil
	}
	if cacheErr := s.cache.Set(ctx, msgKey, "1", 10*time.Minute); cacheErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to set message ID dedup cache key", cacheErr, map[string]string{
			"module": "monitoring", "operation": "msg_dedup_set", "cache_policy": "fail-open",
		})
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
	if !storeSet["mercadolivre"] {
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
			}
		}
	}

	// 5. Apply text transformation rules
	processedText := msg.Body

	// Get monitoring text rules if configured
	if automation.MonitoringTextRules != nil {
		processedText = text.ProcessarMensagem(processedText, automation.MonitoringTextRules)
	}

	// 6. Replace original links with affiliate links
	for orig, aff := range affiliateLinks {
		processedText = strings.ReplaceAll(processedText, orig, aff)
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

		// Priority 1: Link custom preview (source had WhatsApp link preview)
		if firstAffiliateURL != "" && msg.LinkPreviewTitle != "" {
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
	mlURLRegex     = regexp.MustCompile(`(?i)(mercadolivre\.com\.br|mercadolibre\.com|meli\.co)`)
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

func (s *monitoringService) buildContentKey(instanceName, groupID, body string) string {
	// Create a hash of the content for deduplication
	hash := sha256.Sum256([]byte(instanceName + ":" + groupID + ":" + body))
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
