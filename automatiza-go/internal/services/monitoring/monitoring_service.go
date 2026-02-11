package monitoring

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
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
	_ = s.cache.Set(ctx, msgKey, "1", 10*time.Minute)

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
			s.logger.Debug("Outside time window, skipping automation",
				zap.String("automationID", automation.ID))
			return nil
		}
	}

	// 2. Categorize and resolve links
	shopeeLinks, mlLinks := s.categorizeLinks(links)

	// 3. Generate affiliate links
	affiliateLinks := make(map[string]string)

	for _, link := range shopeeLinks {
		if s.shopeeService != nil {
			aff, err := s.shopeeService.GenerateAffiliateLink(ctx, link, automation.UserID)
			if err == nil && aff != nil {
				affiliateLinks[link] = aff.AffiliateLink
			}
		}
	}

	for _, link := range mlLinks {
		if s.mlService != nil {
			aff, err := s.mlService.CreateAffiliateLink(ctx, link, automation.UserID)
			if err == nil && aff != nil {
				affiliateLinks[link] = aff.AffiliateLink
			}
		}
	}

	// 4. Apply text transformation rules
	processedText := msg.Body

	// Get monitoring text rules if configured
	if automation.MonitoringTextRules != nil {
		processedText = text.ProcessarMensagem(processedText, automation.MonitoringTextRules)
	}

	// 5. Replace original links with affiliate links
	for orig, aff := range affiliateLinks {
		processedText = strings.ReplaceAll(processedText, orig, aff)
	}

	// 6. Normalize text
	processedText = text.NormalizeOutgoingWhatsAppText(processedText)
	if processedText == "" {
		return nil
	}

	// 7. Download image media if present
	// WAHA media.url is internal (localhost) and requires auth, so we download
	// the media bytes via DownloadMedia (which adds the API key) and send as base64.
	var imageBase64 string
	var imageMimetype string
	if msg.HasMedia && msg.MediaURL != "" && (msg.MediaType == "image" || strings.HasPrefix(msg.MimeType, "image/")) {
		s.logger.Info("Downloading image media for forwarding",
			zap.String("automationID", automation.ID),
			zap.String("mediaURL", msg.MediaURL),
			zap.String("mimeType", msg.MimeType))
		if s.waProvider != nil {
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

	// 8. Get destination groups
	destinationGroups := automation.SendGroups
	if len(destinationGroups) == 0 {
		destinationGroups = automation.GetTargetGroups()
	}

	if len(destinationGroups) == 0 {
		s.logger.Warn("No destination groups configured",
			zap.String("automationID", automation.ID))
		return nil
	}

	// 9. Get deduplication settings
	dedupeHours := automation.GetDedupeHours()

	// 10. Send to destination groups
	for _, groupID := range destinationGroups {
		// Skip if same group
		if groupID == msg.RemoteJID {
			continue
		}

		// Product dedup check
		canonicalKey := s.extractCanonicalKey(links)
		if s.isProductDuplicate(ctx, automation.UserID, groupID, canonicalKey, dedupeHours) {
			s.logDispatch(ctx, automation, groupID, links[0], "skipped", "dedupe")
			continue
		}

		// Apply delay
		delay := 1 + rand.Intn(3) // 1-3 seconds random
		if automation.DelaySeconds != nil && *automation.DelaySeconds > 0 {
			delay = *automation.DelaySeconds
		}
		time.Sleep(time.Duration(delay) * time.Second)

		// Send message via WhatsAppProvider
		var err error
		if imageBase64 != "" {
			_, err = s.waProvider.SendImage(ctx, whatsapp.SendImageInput{
				SessionName: msg.InstanceName,
				ChatID:      groupID,
				Caption:     processedText,
				ImageBase64: imageBase64,
				Mimetype:    imageMimetype,
			})
		} else {
			_, err = s.waProvider.SendText(ctx, whatsapp.SendTextInput{
				SessionName: msg.InstanceName,
				ChatID:      groupID,
				Text:        processedText,
			})
		}

		if err != nil {
			s.logDispatch(ctx, automation, groupID, links[0], "failed", err.Error())
			s.logger.Error("Failed to send message",
				zap.String("groupID", groupID),
				zap.Error(err))
			continue
		}

		// Mark as sent for dedup
		s.markProductSent(ctx, automation.UserID, groupID, canonicalKey, dedupeHours)
		s.logDispatch(ctx, automation, groupID, links[0], "sent", "")

		s.logger.Info("Message forwarded successfully",
			zap.String("fromGroup", msg.RemoteJID),
			zap.String("toGroup", groupID),
			zap.String("automationID", automation.ID))
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
	shopeeURLRegex = regexp.MustCompile(`(?i)(shopee\.com\.br|shope\.ee)`)
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
	hash := md5.Sum([]byte(instanceName + ":" + groupID + ":" + body))
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
		_ = s.cache.Set(ctx, key, "1", 2*time.Minute)
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
		hash := md5.Sum([]byte(links[0]))
		return "link:" + hex.EncodeToString(hash[:8])
	}

	return ""
}

func (s *monitoringService) isProductDuplicate(ctx context.Context, userID, groupID, canonicalKey string, hours int) bool {
	if s.cache == nil || canonicalKey == "" {
		return false
	}

	cacheKey := fmt.Sprintf("prod_dedup:%s:%s:%s", userID, groupID, canonicalKey)
	val, err := s.cache.Get(ctx, cacheKey)
	return err == nil && val != ""
}

func (s *monitoringService) markProductSent(ctx context.Context, userID, groupID, canonicalKey string, hours int) {
	if s.cache == nil || canonicalKey == "" {
		return
	}

	cacheKey := fmt.Sprintf("prod_dedup:%s:%s:%s", userID, groupID, canonicalKey)
	_ = s.cache.Set(ctx, cacheKey, "1", time.Duration(hours)*time.Hour)
}

func (s *monitoringService) logDispatch(ctx context.Context, automation *models.Automation, groupID, productURL, status, errorMsg string) {
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
	log := &models.DispatchLog{
		UserID:         automation.UserID,
		AutomationID:   &automation.ID,
		AutomationName: &automation.Name,
		GroupID:        &groupID,
		ProductURL:     &productURL,
		Status:         status,
		Type:           &automationType,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	if errorMsg != "" {
		log.Error = &errorMsg
	}

	// W34: Capture dispatch log creation errors with Sentry tags
	if _, err := s.dispatchLogRepo.Create(ctx, log); err != nil {
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
