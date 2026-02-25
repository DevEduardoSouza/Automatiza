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
	dispatchLogRepo repository.DispatchLogRepository

	shopeeService shopee.Service
	mlService     mercadolivre.Service
	waProvider    whatsapp.WhatsAppProvider

	cache  *cache.Cache
	logger *zap.Logger
}

// NewMonitoringService creates a new monitoring service.
// IMPORTANT: This now requires a WhatsAppProvider instead of waha.Service.
func NewMonitoringService(
	automationRepo repository.AutomationRepository,
	instanceRepo repository.InstanceRepository,
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
		dispatchLogRepo: dispatchLogRepo,
		shopeeService:   shopeeService,
		mlService:       mlService,
		waProvider:      waProvider,
		cache:           cache,
		logger:          logger,
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

	// 4. Generate affiliate links
	affiliateLinks := make(map[string]string)

	for _, link := range shopeeLinks {
		if s.shopeeService != nil {
			aff, err := s.shopeeService.GenerateAffiliateLink(ctx, link, automation.UserID)
			if err != nil {
				s.logger.Warn("Failed to generate Shopee affiliate link, keeping original",
					zap.String("automationID", automation.ID),
					zap.String("userID", automation.UserID),
					zap.String("originalLink", link),
					zap.Error(err))
			} else if aff != nil && aff.AffiliateLink != "" {
				affiliateLinks[link] = aff.AffiliateLink
			}
		}
	}

	for _, link := range mlLinks {
		if s.mlService != nil {
			aff, err := s.mlService.CreateAffiliateLink(ctx, link, automation.UserID)
			if err != nil {
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

	// 8. Resolve image media if present
	// Supports two sources:
	//   a) Inline base64 from link preview thumbnail (JPEGThumbnail extracted by processor)
	//   b) Download via WAHA media.url (internal localhost, requires API key auth)
	var imageBase64 string
	var imageMimetype string
	if msg.HasMedia && (msg.MediaType == "image" || strings.HasPrefix(msg.MimeType, "image/")) {
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
		} else if msg.MediaURL != "" && s.waProvider != nil {
			// Download via WAHA media URL
			s.logger.Info("Downloading image media for forwarding",
				zap.String("automationID", automation.ID),
				zap.String("mediaURL", msg.MediaURL),
				zap.String("mimeType", msg.MimeType))
			result, err := s.waProvider.DownloadMediaByURL(ctx, msg.MediaURL)
			if err == nil && result != nil && len(result.Data) > 0 {
				imageBase64 = base64.StdEncoding.EncodeToString(result.Data)
				imageMimetype = result.Mimetype
				if imageMimetype == "" {
					imageMimetype = msg.MimeType
				}
				s.logger.Info("Image media downloaded successfully",
					zap.String("automationID", automation.ID),
					zap.Int("dataLen", len(result.Data)),
					zap.String("mimetype", imageMimetype))
			} else if err != nil {
				s.logger.Warn("Failed to download image media, will send text only",
					zap.String("automationID", automation.ID),
					zap.Error(err))
			}
		}
	}

	// 9. Get destination groups
	destinationGroups := automation.SendGroups
	if len(destinationGroups) == 0 {
		destinationGroups = automation.GetTargetGroups()
	}

	if len(destinationGroups) == 0 {
		s.logger.Warn("No destination groups configured",
			zap.String("automationID", automation.ID))
		return nil
	}

	// 10. Get deduplication settings
	dedupeHours := automation.GetDedupeHours()
	canonicalKey := s.extractCanonicalKey(links)
	store := s.detectStore(links)

	// 11. Send to destination groups
	for _, groupID := range destinationGroups {
		// Skip if same group
		if groupID == msg.RemoteJID {
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

		// Send message via WhatsAppProvider with dedicated timeout
		sendCtx, sendCancel := context.WithTimeout(ctx, 30*time.Second)
		var err error
		if imageBase64 != "" {
			_, err = s.waProvider.SendImage(sendCtx, whatsapp.SendImageInput{
				SessionName: msg.InstanceName,
				ChatID:      groupID,
				Caption:     processedText,
				ImageBase64: imageBase64,
				Mimetype:    imageMimetype,
			})
		} else {
			_, err = s.waProvider.SendText(sendCtx, whatsapp.SendTextInput{
				SessionName: msg.InstanceName,
				ChatID:      groupID,
				Text:        processedText,
			})
		}
		sendCancel()

		if err != nil {
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
	// Get all monitoring automations for this group
	automations, err := s.automationRepo.GetMonitoringAutomationsForGroup(ctx, groupJID)
	if err != nil {
		return nil, err
	}

	// Filter to active ones with matching instance
	var matching []models.Automation
	for _, auto := range automations {
		if !auto.IsActive() {
			continue
		}

		// Check if instance matches (if we can determine)
		// For now, accept all active monitoring automations for this group
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
	// Prefer product links over coupon links
	for _, link := range links {
		if shopeeURLRegex.MatchString(link) {
			// Try to extract shop/item IDs
			if s.shopeeService != nil {
				shopID, itemID, err := s.shopeeService.ExtractProductIDs(link)
				if err == nil {
					return fmt.Sprintf("shopee:%d:%d", shopID, itemID)
				}
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

	// Fallback to first link hash
	if len(links) > 0 {
		hash := sha256.Sum256([]byte(links[0]))
		return "link:" + hex.EncodeToString(hash[:8])
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
		log, err := s.dispatchLogRepo.GetRecentByCanonicalKey(ctx, userID, groupID, canonicalKey, hours)
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
