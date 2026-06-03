package offer_generate

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/ctapicker"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/mercadolivre"
	"automatiza-go/internal/services/shopee"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// Service defines the offer generate service interface
type Service interface {
	ProcessIncomingMessage(ctx context.Context, msg *models.IncomingMessage) error
}

// CouponResult contains the result of applying a coupon to an offer.
type CouponResult struct {
	Applied         bool
	CouponText      string
	DiscountedPrice float64
}

// CouponApplier handles coupon loading and application.
// This interface avoids an import cycle with the automation package.
//
// LoadCouponOnce / ApplyCouponWithPreloaded are kept for legacy code
// (e.g. discount-tag override flow); ApplyCoupon is the rules-engine
// entry point used per offer with the actual price + platform.
type CouponApplier interface {
	LoadCouponOnce(ctx context.Context, automation *models.Automation) (*models.Coupon, string)
	ApplyCouponWithPreloaded(coupon *models.Coupon, offerPrice float64, offerStore string) *CouponResult
	ApplyCoupon(ctx context.Context, automation *models.Automation, offerPrice float64, offerStore string, productIsOfficial bool) (*CouponResult, error)
}

type offerGenerateService struct {
	automationRepo  repository.AutomationRepository
	instanceRepo    repository.InstanceRepository
	groupRepo       repository.GroupRepository
	dispatchLogRepo repository.DispatchLogRepository
	settingsRepo    repository.SettingsRepository
	couponRepo      repository.CouponRepository
	ctaRepo         repository.CTARepository
	affiliateRepo   repository.AffiliateCredentialsRepository

	shopeeService shopee.Service
	mlService     mercadolivre.Service
	waProvider    whatsapp.WhatsAppProvider

	couponHandler CouponApplier

	cache  *cache.Cache
	logger *zap.Logger
}

// NewOfferGenerateService creates a new offer generate service.
func NewOfferGenerateService(
	automationRepo repository.AutomationRepository,
	instanceRepo repository.InstanceRepository,
	groupRepo repository.GroupRepository,
	dispatchLogRepo repository.DispatchLogRepository,
	settingsRepo repository.SettingsRepository,
	couponRepo repository.CouponRepository,
	ctaRepo repository.CTARepository,
	affiliateRepo repository.AffiliateCredentialsRepository,
	shopeeService shopee.Service,
	mlService mercadolivre.Service,
	waProvider whatsapp.WhatsAppProvider,
	couponHandler CouponApplier,
	cache *cache.Cache,
	logger *zap.Logger,
) Service {
	return &offerGenerateService{
		automationRepo:  automationRepo,
		instanceRepo:    instanceRepo,
		groupRepo:       groupRepo,
		dispatchLogRepo: dispatchLogRepo,
		settingsRepo:    settingsRepo,
		couponRepo:      couponRepo,
		ctaRepo:         ctaRepo,
		affiliateRepo:   affiliateRepo,
		shopeeService:   shopeeService,
		mlService:       mlService,
		waProvider:      waProvider,
		couponHandler:   couponHandler,
		cache:           cache,
		logger:          logger.Named("offer-generate"),
	}
}

func (s *offerGenerateService) ProcessIncomingMessage(ctx context.Context, msg *models.IncomingMessage) error {
	// 1. Validate: must be a group message
	if !strings.HasSuffix(msg.RemoteJID, "@g.us") {
		return nil
	}

	// FromMe handling: a blanket "skip every fromMe" guard would also kill
	// the legitimate case of the chip's owner posting links into a monitor
	// group from their own phone. Instead, mark each outgoing message
	// before we send it (see processAutomation, og_self_send key) and
	// skip only when this incoming hash matches one we just sent. That
	// blocks real loops (group is both monitor and send) without blocking
	// manual posts.
	if msg.FromMe {
		if s.isOwnOutgoing(ctx, msg.InstanceName, msg.RemoteJID, msg.Body) {
			s.logger.Debug("Offer generate: own outgoing returned via webhook, skipping",
				zap.String("instance", msg.InstanceName),
				zap.String("group", msg.RemoteJID))
			return nil
		}
	}

	// 2. Extract links from message body. info[...] must be stripped first
	// so URLs embedded inside an info tag (typically a coupon link) are not
	// mistaken for a separate product link.
	bodyForLinks := text.StripInfoTag(msg.Body)
	links := text.ExtractLinks(bodyForLinks)
	if len(links) == 0 {
		// Nenhum link de produto detectado: cai no modo "relay" — repassa
		// a mensagem original (texto, foto, áudio, vídeo, documento) pros
		// grupos de destino exatamente como veio, sem processamento de
		// afiliado/cupom. O grupo de origem agora aceita qualquer tipo
		// de conteúdo informativo, não só links.
		return s.processRelay(ctx, msg)
	}

	s.logger.Info("Processing incoming message for offer generation",
		zap.String("instance", msg.InstanceName),
		zap.String("group", msg.RemoteJID),
		zap.Int("links", len(links)),
		zap.String("bodyPreview", truncateStr(msg.Body, 100)))

	// 3. Content fingerprint dedup (2 min window).
	//
	// Keyed by (group, body-hash) — sem o instance — pra que mensagens
	// vistas por múltiplos chips do mesmo grupo monitor disparem só 1
	// processamento. Se 4 chips do user estão no grupo de origem, WAHA
	// fira 4 webhooks; o primeiro marca a key, os outros 3 batem aqui
	// e fazem bailout antes da chamada à API da Shopee/ML — evita 4×
	// rate limit + spam de "skipped:dedupe" no dispatch_logs.
	//
	// claimProductSlot() no loop de dispatch segue como cinto de
	// segurança por (user, destinationGroup, canonicalKey), então
	// mesmo que esse pré-dedup falhe (Redis offline), nunca há
	// duplicação real no destino.
	contentKey := s.buildContentKey(msg.RemoteJID, msg.Body)
	if s.isDuplicate(ctx, contentKey) {
		s.logger.Debug("Offer generate: duplicate content, skipping")
		return nil
	}
	s.markProcessed(ctx, contentKey, 2*time.Minute)

	// 4. Message ID dedup (10 min window). Mesma motivação do (3) —
	// keyed por (group, msgID), não por instance.
	msgKey := fmt.Sprintf("og_msg_dedup:%s:%s", msg.RemoteJID, msg.MessageID)
	if s.isDuplicate(ctx, msgKey) {
		s.logger.Debug("Offer generate: duplicate message ID, skipping")
		return nil
	}
	s.markProcessed(ctx, msgKey, 10*time.Minute)

	// 5. Find matching offer_generate automations for this group
	automations, err := s.automationRepo.GetOfferGenerateAutomationsForGroup(ctx, msg.RemoteJID)
	if err != nil {
		return err
	}

	// Filter to active only
	var active []models.Automation
	for _, a := range automations {
		if a.IsActive() {
			active = append(active, a)
		}
	}

	if len(active) == 0 {
		return nil
	}

	s.logger.Info("Found matching offer_generate automations",
		zap.Int("count", len(active)),
		zap.String("group", msg.RemoteJID))

	// 6. Process each automation
	for i := range active {
		if err := s.processAutomation(ctx, &active[i], msg, links); err != nil {
			s.logger.Warn("Failed to process offer_generate automation",
				zap.String("automationID", active[i].ID),
				zap.Error(err))
		}
	}

	return nil
}

func (s *offerGenerateService) processAutomation(ctx context.Context, auto *models.Automation, msg *models.IncomingMessage, links []string) error {
	// 1. Check time window
	if auto.StartTime != nil && auto.EndTime != nil {
		tw := &models.TimeWindow{
			StartTime: *auto.StartTime,
			EndTime:   *auto.EndTime,
			Timezone:  "America/Sao_Paulo",
		}
		if auto.ScheduleDays != nil {
			tw.Days = auto.ScheduleDays
		}
		if !tw.IsWithinWindow(time.Now()) {
			s.logger.Info("Outside time window, skipping offer_generate",
				zap.String("automationID", auto.ID))
			return nil
		}
	}

	// 2. Categorize links by store
	var shopeeLinks, mlLinks []string
	for _, link := range links {
		if text.IsShopeeLink(link) {
			shopeeLinks = append(shopeeLinks, link)
		} else if text.IsMLLink(link) {
			mlLinks = append(mlLinks, link)
		}
	}

	// 3. Filter by configured stores (fallback to user's active affiliate stores)
	stores := auto.Stores
	if len(stores) == 0 || (len(stores) == 1 && stores[0] == "shopee") {
		// If only default "shopee" or empty, check if user has more active stores
		if s.affiliateRepo != nil {
			activeStores, err := s.affiliateRepo.GetActiveStores(ctx, auto.UserID)
			if err == nil && len(activeStores) > 0 {
				stores = activeStores
			}
		}
	}
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
	if !storeSet["mercadolivre"] && !storeSet["mercado_livre"] && !storeSet["ml"] {
		mlLinks = nil
	}

	if len(shopeeLinks) == 0 && len(mlLinks) == 0 {
		s.logger.Info("No links matching configured stores for offer_generate",
			zap.String("automationID", auto.ID),
			zap.Strings("stores", stores))
		return nil
	}

	// 4. Parse discount and info tags from message body
	discountTag := text.ParseDiscountTag(msg.Body)
	infoText := text.ParseInfoTag(msg.Body)

	// 5. Load dispatch template (required for offer_generate)
	if auto.DispatchTextID == nil || *auto.DispatchTextID == "" {
		s.logger.Warn("No dispatch text configured for offer_generate",
			zap.String("automationID", auto.ID))
		return nil
	}

	dispatchText, err := s.settingsRepo.GetActiveDispatchTextByID(ctx, *auto.DispatchTextID, auto.UserID)
	if err != nil || dispatchText == nil {
		s.logger.Warn("Failed to load dispatch text",
			zap.String("automationID", auto.ID),
			zap.Error(err))
		return nil
	}

	// 6. Load CTA pool. Política espelha automation_service:
	//   - auto.CTAIDs preenchido → override explícito, busca só esses
	//   - auto.CTAIDs vazio → pool inteiro de CTAs ativos do user
	//
	// O texto do CTA é selecionado mais tarde via ctapicker.PickByPlatform,
	// já que aqui ainda não sabemos se o link vai resolver para Shopee ou ML
	// (definido nos branches mais abaixo). Manter a busca uma vez fora do
	// branch evita N+1 quando processa muitos links da mesma message.
	var ctaPool []models.CTA
	if s.ctaRepo != nil {
		if len(auto.CTAIDs) > 0 {
			fetched, ctaErr := s.ctaRepo.GetCTAsByIDs(ctx, auto.CTAIDs, auto.UserID)
			if ctaErr != nil {
				s.logger.Warn("Failed to fetch CTA entities by IDs",
					zap.String("automationID", auto.ID),
					zap.Error(ctaErr))
			} else {
				ctaPool = fetched
			}
		} else {
			fetched, ctaErr := s.ctaRepo.GetUserCTAs(ctx, auto.UserID, map[string]interface{}{"is_active": true})
			if ctaErr != nil {
				s.logger.Warn("Failed to fetch user CTA pool",
					zap.String("automationID", auto.ID),
					zap.Error(ctaErr))
			} else {
				ctaPool = fetched
			}
		}
	}

	// pickCTA escolhe pela plataforma resolvida (Shopee/ML) e cai pros
	// inline auto.CTAs como fallback se o pool não tiver match.
	pickCTA := func(isML, isShopee bool) string {
		if text := ctapicker.PickByPlatform(ctaPool, isML, isShopee); text != "" {
			return text
		}
		if len(auto.CTAs) > 0 {
			return auto.CTAs[rand.IntN(len(auto.CTAs))] //nolint:gosec // G404: non-security random for CTA selection
		}
		return ""
	}

	// 7. Coupon resolution (rules engine + discount-tag fallback).
	//
	// Per-automation coupon was retired with migration 046 — coupons are
	// now picked by the rule engine, evaluated below per offer once we
	// know the price and platform. The only preload that still makes
	// sense here is the discount-tag override: messages can carry a
	// coupon code in the tag itself, and that takes priority over rules.
	var preloadedCoupon *models.Coupon
	if discountTag != nil && discountTag.CouponCode != "" && s.couponRepo != nil {
		tagCoupon, err := s.couponRepo.GetCouponByCode(ctx, auto.UserID, discountTag.CouponCode)
		if err == nil && tagCoupon != nil && tagCoupon.IsActive {
			preloadedCoupon = tagCoupon
			s.logger.Debug("Coupon loaded from discount tag",
				zap.String("couponCode", discountTag.CouponCode),
				zap.String("automationID", auto.ID))
		}
	}

	// 8. Resolve sender chips for the multi-chip fan-out (049).
	//
	// Build (a) the connected-by-id map for picking session names, and
	// (b) the ordered preference list. Falls back to "first connected"
	// when the automation has no chip preference set, preserving the
	// pre-049 behavior for older rows.
	instances, err := s.instanceRepo.GetUserInstances(ctx, auto.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user instances: %w", err)
	}
	connectedByID := make(map[string]string, len(instances)) // db id (string) -> session name
	var firstConnected string
	for _, inst := range instances {
		if inst.Status != "connected" {
			continue
		}
		idStr := fmt.Sprintf("%d", inst.ID)
		connectedByID[idStr] = inst.InstanceID
		if firstConnected == "" {
			firstConnected = inst.InstanceID
		}
	}
	if firstConnected == "" {
		s.logger.Warn("No connected WhatsApp instance for offer_generate",
			zap.String("automationID", auto.ID),
			zap.String("userID", auto.UserID))
		return nil
	}

	// Preference order: explicit list when set; legacy single InstanceID
	// otherwise; finally, fall through to "use firstConnected for every
	// group" (no participation filter).
	preferenceIDs := make([]string, 0, len(auto.InstanceIDs)+1)
	for _, id := range auto.InstanceIDs {
		if _, ok := connectedByID[id]; ok {
			preferenceIDs = append(preferenceIDs, id)
		}
	}
	if len(preferenceIDs) == 0 && auto.InstanceID != nil && *auto.InstanceID != "" {
		if _, ok := connectedByID[*auto.InstanceID]; ok {
			preferenceIDs = append(preferenceIDs, *auto.InstanceID)
		}
	}
	// senderSession is the *default* chip for the dispatch run — used
	// for markOwnOutgoing keys and as a fallback when no chip in the
	// preference participates in a given group. Picks the first chip in
	// the preference list, or the first connected if there's no list.
	senderSession := firstConnected
	if len(preferenceIDs) > 0 {
		senderSession = connectedByID[preferenceIDs[0]]
	}

	// 9. Process the FIRST valid link
	var renderedMessage string
	var imageURL string
	var canonicalKey string
	var store string

	if len(shopeeLinks) > 0 && s.shopeeService != nil {
		link := shopeeLinks[0]
		store = "shopee"

		// Expand short URL if needed
		expandedURL, expandErr := s.shopeeService.ExpandURL(ctx, link)
		if expandErr == nil && expandedURL != "" {
			link = expandedURL
		}

		// Extract product IDs and get product details
		shopID, itemID, idErr := s.shopeeService.ExtractProductIDs(link)
		if idErr != nil {
			s.logger.Warn("Failed to extract Shopee product IDs",
				zap.String("link", link), zap.Error(idErr))
			return nil
		}

		product, prodErr := s.shopeeService.GetProduct(ctx, itemID, shopID, auto.UserID)
		if prodErr != nil || product == nil {
			s.logger.Warn("Failed to get Shopee product",
				zap.Int64("itemID", itemID), zap.Int64("shopID", shopID), zap.Error(prodErr))
			return nil
		}

		// Generate affiliate link
		affLink, affErr := s.shopeeService.GenerateAffiliateLink(ctx, link, auto.UserID)
		if affErr != nil {
			if isCredentialError(affErr) {
				s.pauseWithError(ctx, auto, "Credenciais Shopee inválidas - automação pausada")
				return nil
			}
			s.logger.Warn("Failed to generate Shopee affiliate link",
				zap.String("link", link), zap.Error(affErr))
			return nil
		}

		affiliateURL := link
		if affLink != nil && affLink.AffiliateLink != "" {
			affiliateURL = affLink.AffiliateLink
		}
		if affLink != nil && affLink.ImageURL != "" {
			imageURL = affLink.ImageURL
		}

		// Apply discount tag to price
		offerPrice := product.Price
		if discountTag != nil {
			offerPrice = text.ApplyDiscountTag(offerPrice, discountTag)
		}

		// Apply coupon: discount-tag override first, then rule engine.
		// product.IsMall flags Shopee Mall items so official_only rules
		// can filter correctly.
		couponBlock := ""
		if s.couponHandler != nil {
			var couponResult *CouponResult
			if preloadedCoupon != nil {
				couponResult = s.couponHandler.ApplyCouponWithPreloaded(preloadedCoupon, offerPrice, store)
			} else if cr, err := s.couponHandler.ApplyCoupon(ctx, auto, offerPrice, store, product.IsMall); err == nil {
				couponResult = cr
			}
			if couponResult != nil && couponResult.Applied {
				couponBlock = couponResult.CouponText
				if couponResult.DiscountedPrice > 0 {
					offerPrice = couponResult.DiscountedPrice
				}
			}
		}

		canonicalKey = fmt.Sprintf("shopee:%d:%d", shopID, itemID)

		// Render template
		renderedMessage = text.RenderShopeeOfferTemplate(dispatchText.Body, text.ShopeeOfferVars{
			Title:         product.Name,
			Price:         offerPrice,
			OriginalPrice: product.OriginalPrice,
			Discount:      product.Discount,
			Link:          link,
			AffiliateLink: affiliateURL,
			Image:         imageURL,
			Category:      product.Category,
			Store:         "Shopee",
			CTA:           pickCTA(false, true),
			CouponBlock:   couponBlock,
			Info:          infoText,
		})

	} else if len(mlLinks) > 0 && s.mlService != nil {
		link := mlLinks[0]
		store = "mercadolivre"

		// Get product preview
		preview, prevErr := s.mlService.GetProductPreview(ctx, link, auto.UserID)
		if prevErr != nil || preview == nil {
			s.logger.Warn("Failed to get ML product preview",
				zap.String("link", link), zap.Error(prevErr))
			return nil
		}

		// Generate affiliate link
		affLink, affErr := s.mlService.CreateAffiliateLink(ctx, link, auto.UserID)
		if affErr != nil {
			if isCredentialError(affErr) {
				s.pauseWithError(ctx, auto, "Credenciais ML inválidas - automação pausada")
				return nil
			}
			s.logger.Warn("Failed to generate ML affiliate link",
				zap.String("link", link), zap.Error(affErr))
			return nil
		}

		affiliateURL := link
		if affLink != nil && affLink.AffiliateLink != "" {
			affiliateURL = affLink.AffiliateLink
		}
		imageURL = preview.Image

		// Apply discount tag to price
		offerPrice := preview.Price
		if discountTag != nil {
			offerPrice = text.ApplyDiscountTag(offerPrice, discountTag)
		}

		// Apply coupon: discount-tag override first, then rule engine.
		// ML doesn't expose a Mall-style flag — productIsOfficial=false.
		couponBlock := ""
		if s.couponHandler != nil {
			var couponResult *CouponResult
			if preloadedCoupon != nil {
				couponResult = s.couponHandler.ApplyCouponWithPreloaded(preloadedCoupon, offerPrice, store)
			} else if cr, err := s.couponHandler.ApplyCoupon(ctx, auto, offerPrice, store, false); err == nil {
				couponResult = cr
			}
			if couponResult != nil && couponResult.Applied {
				couponBlock = couponResult.CouponText
				if couponResult.DiscountedPrice > 0 {
					offerPrice = couponResult.DiscountedPrice
				}
			}
		}

		// Canonical key from ML product ID
		if preview.ItemID != nil {
			canonicalKey = fmt.Sprintf("ml:%s", *preview.ItemID)
		} else {
			canonicalKey = fmt.Sprintf("ml:%s", hashStr(link))
		}

		// Build ML vars
		originalPrice := offerPrice
		if preview.OriginalPrice != nil {
			originalPrice = *preview.OriginalPrice
		}
		var soldCount int
		if preview.SoldCount != nil {
			soldCount = *preview.SoldCount
		}
		var rating float64
		if preview.Rating != nil {
			rating = *preview.Rating
		}
		var reviewCount int
		if preview.ReviewCount != nil {
			reviewCount = *preview.ReviewCount
		}
		condition := ""
		if preview.Condition != nil {
			condition = *preview.Condition
		}

		renderedMessage = text.RenderMLOfferTemplate(dispatchText.Body, text.MLOfferVars{
			Title:         preview.Title,
			Price:         offerPrice,
			OriginalPrice: originalPrice,
			Discount:      preview.Discount,
			Link:          link,
			AffiliateLink: affiliateURL,
			Image:         imageURL,
			Condition:     condition,
			SoldCount:     soldCount,
			Rating:        rating,
			ReviewCount:   reviewCount,
			CTA:           pickCTA(true, false),
			CouponBlock:   couponBlock,
			Info:          infoText,
		})
	} else {
		return nil
	}

	if renderedMessage == "" {
		s.logger.Warn("Empty rendered message, skipping offer_generate",
			zap.String("automationID", auto.ID))
		return nil
	}

	// Normalize outgoing text
	renderedMessage = text.NormalizeOutgoingWhatsAppText(renderedMessage)

	// 10. Send to destination groups
	destinationGroups := auto.SendGroups
	if len(destinationGroups) == 0 {
		destinationGroups = auto.GetTargetGroups()
	}

	// Build wa_group_id → []participating chip IDs map for the multi-chip
	// fan-out. With the map, each destination group routes through the
	// FIRST chip in the preference list that actually participates in it
	// — falling back to senderSession when no preference applies. The
	// lookup is O(1) per group and the load is one DB round-trip total.
	groupParticipants := map[string][]string{}
	if s.groupRepo != nil && len(preferenceIDs) > 0 && len(destinationGroups) > 0 {
		groups, err := s.groupRepo.GetGroupsByWAGroupIDs(ctx, auto.UserID, destinationGroups)
		if err != nil {
			s.logger.Warn("Failed to load destination group participants — falling back to default chip",
				zap.String("automationID", auto.ID), zap.Error(err))
		} else {
			for _, g := range groups {
				groupParticipants[g.WAGroupID] = g.ParticipatingInstanceIDs
			}
		}
	}

	chipForGroup := func(waGroupID string) string {
		return PickChipForGroup(ChipRoutingInput{
			WaGroupID:        waGroupID,
			PreferenceIDs:    preferenceIDs,
			GroupParticipants: groupParticipants[waGroupID],
			ConnectedByID:    connectedByID,
			ChipOverrides:    auto.ChipOverrides,
			SenderSession:    senderSession,
		})
	}

	// Dedup window é fixo em 30min pra /offer-generate. Diferente do
	// monitoring/search (que usa dedupe_hours da automation, default 7d),
	// aqui o disparo é manual: o operador cola o link no grupo de origem
	// quando quer enviar a oferta. Janela longa só atrapalha — se ele
	// reenvia o mesmo produto 1h depois é proposital. 30min ainda corta
	// o caso comum (mesma mensagem chegando por múltiplos chips no grupo
	// monitor) e os webhooks Redis SetNX duplicados em segundos.
	const dedupeWindow = 30 * time.Minute

	for _, groupID := range destinationGroups {
		// Skip source group
		if groupID == msg.RemoteJID {
			continue
		}

		dispatchSession := chipForGroup(groupID)
		if dispatchSession == "" {
			s.logger.Info("No preferred chip participates in destination group, skipping",
				zap.String("automationID", auto.ID),
				zap.String("toGroup", groupID))
			s.logDispatch(ctx, auto, groupID, links[0], "skipped", "no_chip_in_group", canonicalKey, store)
			continue
		}

		// Dedup by (userID, groupID, canonicalKey)
		if !s.claimProductSlot(ctx, auto.UserID, groupID, canonicalKey, dedupeWindow) {
			s.logger.Info("Product already sent (dedup), skipping",
				zap.String("automationID", auto.ID),
				zap.String("toGroup", groupID),
				zap.String("canonicalKey", canonicalKey))
			s.logDispatch(ctx, auto, groupID, links[0], "skipped", "dedupe", canonicalKey, store)
			continue
		}

		// Apply delay
		delay := 1 + rand.IntN(3)
		if auto.DelaySeconds != nil && *auto.DelaySeconds > 0 {
			delay = *auto.DelaySeconds
		}
		time.Sleep(time.Duration(delay) * time.Second)

		// Mark this body as our own outgoing BEFORE sending. The webhook
		// for our send arrives with FromMe=true; ProcessIncomingMessage
		// looks for this key to distinguish "real loop" from "user posted
		// manually." 60s TTL is far longer than any reasonable webhook
		// delay; if the send fails, the orphan key is harmless. Indexed
		// by the actual sending session so multi-chip fan-out doesn't
		// share keys across chips.
		s.markOwnOutgoing(ctx, dispatchSession, groupID, renderedMessage)

		// Send via WhatsApp
		sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
		var sendErr error
		if imageURL != "" {
			_, sendErr = s.waProvider.SendImage(sendCtx, whatsapp.SendImageInput{
				SessionName: dispatchSession,
				ChatID:      groupID,
				Caption:     renderedMessage,
				ImageURL:    imageURL,
			})
		} else {
			_, sendErr = s.waProvider.SendText(sendCtx, whatsapp.SendTextInput{
				SessionName: dispatchSession,
				ChatID:      groupID,
				Text:        renderedMessage,
			})
		}
		sendCancel()

		if sendErr != nil {
			s.releaseProductSlot(ctx, auto.UserID, groupID, canonicalKey)
			s.logDispatch(ctx, auto, groupID, links[0], "failed", sendErr.Error(), canonicalKey, store)
			s.logger.Error("Failed to send offer",
				zap.String("groupID", groupID),
				zap.String("session", dispatchSession),
				zap.Error(sendErr))
			continue
		}

		s.logDispatch(ctx, auto, groupID, links[0], "sent", "", canonicalKey, store)

		s.logger.Info("Offer generated and sent successfully",
			zap.String("fromGroup", msg.RemoteJID),
			zap.String("toGroup", groupID),
			zap.String("session", dispatchSession),
			zap.String("automationID", auto.ID),
			zap.String("canonicalKey", canonicalKey))
	}

	return nil
}

// --- Helper methods ---

// buildContentKey returns a Redis key that uniquely identifies a content
// fingerprint inside a WhatsApp group, regardless of which chip received
// the webhook. This lets the pre-dispatch dedup (ProcessIncomingMessage)
// short-circuit when the same message reaches us via multiple chips —
// e.g. when more than one of the user's chips is a member of the source
// group, WAHA fires one webhook per chip with the same body.
//
// The matching dispatch-side dedup (claimProductSlot) keeps using
// (user, destinationGroup, canonicalKey) as a stronger guarantee, so a
// hash collision here at most slips through to the per-destination
// claim, which still prevents real duplication.
func (s *offerGenerateService) buildContentKey(group, body string) string {
	h := sha256.Sum256([]byte(body))
	return fmt.Sprintf("og_content:%s:%s", group, hex.EncodeToString(h[:8]))
}

// buildSelfSendKey identifies an outgoing message hash we just sent so the
// FromMe webhook that returns can be distinguished from a chip user's
// manual post in a monitor group. Indexed by (session, group, body-hash)
// because the webhook fires on the same session that sent the message.
func (s *offerGenerateService) buildSelfSendKey(instance, group, body string) string {
	h := sha256.Sum256([]byte(body))
	return fmt.Sprintf("og_self_send:%s:%s:%s", instance, group, hex.EncodeToString(h[:8]))
}

// markOwnOutgoing records that we are about to send `body` from `instance`
// into `group`. 60s TTL is large enough for any normal webhook delay; on
// send failure the orphan key is harmless (no webhook fires).
func (s *offerGenerateService) markOwnOutgoing(ctx context.Context, instance, group, body string) {
	if s.cache == nil {
		return
	}
	key := s.buildSelfSendKey(instance, group, body)
	if err := s.cache.Set(ctx, key, "1", 60*time.Second); err != nil {
		// Logged at warn — without the mark we fall back to the existing
		// content/message-id dedup, which catches most loops anyway.
		observability.LogAndCapture(s.logger, sentry.LevelWarning,
			"Failed to mark own outgoing for offer_generate", err,
			map[string]string{"module": "offer_generate", "operation": "mark_self_send"})
	}
}

// isOwnOutgoing returns true if this incoming FromMe message matches a
// self-send we marked recently — i.e. it's our own dispatch returning via
// the WhatsApp webhook, not the chip user posting manually.
func (s *offerGenerateService) isOwnOutgoing(ctx context.Context, instance, group, body string) bool {
	if s.cache == nil {
		return false
	}
	val, err := s.cache.Get(ctx, s.buildSelfSendKey(instance, group, body))
	if err != nil {
		return false
	}
	return val != ""
}

func (s *offerGenerateService) isDuplicate(ctx context.Context, key string) bool {
	if s.cache == nil {
		return false
	}
	val, err := s.cache.Get(ctx, key)
	if err != nil {
		return false
	}
	return val != ""
}

func (s *offerGenerateService) markProcessed(ctx context.Context, key string, ttl time.Duration) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Set(ctx, key, "1", ttl); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to set offer_generate dedup key", err, map[string]string{
			"module": "offer_generate", "operation": "dedup_set",
		})
	}
}

func (s *offerGenerateService) claimProductSlot(ctx context.Context, userID, groupID, canonicalKey string, window time.Duration) bool {
	if canonicalKey == "" {
		return true
	}

	cacheKey := fmt.Sprintf("og_prod_dedup:%s:%s:%s", userID, groupID, canonicalKey)

	if s.cache != nil {
		wasSet, err := s.cache.SetNX(ctx, cacheKey, "1", window)
		if err == nil && !wasSet {
			return false
		}
	}

	// Check DB for historical dispatches
	if s.dispatchLogRepo != nil {
		log, err := s.dispatchLogRepo.GetRecentByCanonicalKey(ctx, userID, groupID, canonicalKey, window)
		if err == nil && log != nil {
			return false
		}
	}

	return true
}

func (s *offerGenerateService) releaseProductSlot(ctx context.Context, userID, groupID, canonicalKey string) {
	if s.cache == nil || canonicalKey == "" {
		return
	}
	cacheKey := fmt.Sprintf("og_prod_dedup:%s:%s:%s", userID, groupID, canonicalKey)
	_ = s.cache.Delete(ctx, cacheKey)
}

func (s *offerGenerateService) logDispatch(ctx context.Context, auto *models.Automation, groupID, productURL, status, errorMsg, canonicalKey, store string) {
	if s.dispatchLogRepo == nil {
		return
	}

	featureFlags := config.GetFeatureFlags()
	if !featureFlags.ShouldCreateDispatchLog() {
		return
	}

	automationType := "offer_generate"
	dispatchLog := &models.DispatchLog{
		UserID:         auto.UserID,
		AutomationID:   &auto.ID,
		AutomationName: &auto.Name,
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

	if _, err := s.dispatchLogRepo.Create(ctx, dispatchLog); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to create offer_generate dispatch log", err, map[string]string{
			"module": "offer_generate", "operation": "dispatch_log_create",
		})
	}
}

func (s *offerGenerateService) pauseWithError(ctx context.Context, auto *models.Automation, errMsg string) {
	now := time.Now().UTC().Format(time.RFC3339)
	_ = s.automationRepo.UpdateAutomation(ctx, auto.ID, map[string]interface{}{
		"status":        models.AutomationStatusPaused,
		"last_error":    errMsg,
		"last_error_at": now,
	})
	s.logger.Warn("Automation paused due to credential error",
		zap.String("automationID", auto.ID),
		zap.String("error", errMsg))
}

func isCredentialError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "Invalid Signature") ||
		strings.Contains(errStr, "credentials incomplete") ||
		strings.Contains(errStr, "invalid_token")
}

func hashStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// =============================================================================
// Relay mode — repassa mensagens sem link de produto pros grupos de destino
// =============================================================================

// processRelay é o caminho alternativo ao fluxo de oferta quando a mensagem
// que cai no grupo de origem NÃO contém link de produto Shopee/ML. Em vez
// de ignorar (comportamento antigo), repassa o conteúdo bruto (texto solto,
// foto, áudio, vídeo, documento) pros grupos de destino configurados.
//
// Compartilha as proteções de dedup com o fluxo principal (content
// fingerprint + message ID) pra que múltiplos chips no grupo de origem não
// disparem replays. Não há resolução de cupom/preço — relay é "pipe puro".
func (s *offerGenerateService) processRelay(ctx context.Context, msg *models.IncomingMessage) error {
	s.logger.Info("Processing incoming message for relay (no product link)",
		zap.String("instance", msg.InstanceName),
		zap.String("group", msg.RemoteJID),
		zap.Bool("hasMedia", msg.HasMedia),
		zap.String("mediaType", msg.MediaType),
		zap.String("bodyPreview", truncateStr(msg.Body, 100)))

	// Content fingerprint dedup (2 min) — mesma chave do fluxo principal
	// (buildContentKey hashea o Body). Pra mídia sem caption (Body=""),
	// inclui a MediaURL no hash pra evitar que duas mídias sem legenda
	// colidam na mesma janela.
	contentKey := s.buildRelayContentKey(msg)
	if s.isDuplicate(ctx, contentKey) {
		s.logger.Debug("Offer relay: duplicate content, skipping")
		return nil
	}
	s.markProcessed(ctx, contentKey, 2*time.Minute)

	// Message ID dedup (10 min) — idêntico ao fluxo principal.
	msgKey := fmt.Sprintf("og_relay_msg_dedup:%s:%s", msg.RemoteJID, msg.MessageID)
	if s.isDuplicate(ctx, msgKey) {
		s.logger.Debug("Offer relay: duplicate message ID, skipping")
		return nil
	}
	s.markProcessed(ctx, msgKey, 10*time.Minute)

	automations, err := s.automationRepo.GetOfferGenerateAutomationsForGroup(ctx, msg.RemoteJID)
	if err != nil {
		return err
	}

	var active []models.Automation
	for _, a := range automations {
		if a.IsActive() {
			active = append(active, a)
		}
	}
	if len(active) == 0 {
		return nil
	}

	s.logger.Info("Found matching offer_generate automations for relay",
		zap.Int("count", len(active)),
		zap.String("group", msg.RemoteJID))

	for i := range active {
		if err := s.processAutomationRelay(ctx, &active[i], msg); err != nil {
			s.logger.Warn("Failed to process offer_generate relay",
				zap.String("automationID", active[i].ID),
				zap.Error(err))
		}
	}

	return nil
}

// processAutomationRelay envia a mensagem original pra cada grupo destino
// da automação. Resolve chips e dedup do mesmo jeito que processAutomation,
// só que não monta texto formatado — usa msg.Body + msg.MediaURL/MediaBase64
// e dispara o método WAHA apropriado por tipo.
func (s *offerGenerateService) processAutomationRelay(ctx context.Context, auto *models.Automation, msg *models.IncomingMessage) error {
	// Time window
	if auto.StartTime != nil && auto.EndTime != nil {
		tw := &models.TimeWindow{
			StartTime: *auto.StartTime,
			EndTime:   *auto.EndTime,
			Timezone:  "America/Sao_Paulo",
		}
		if auto.ScheduleDays != nil {
			tw.Days = auto.ScheduleDays
		}
		if !tw.IsWithinWindow(time.Now()) {
			s.logger.Info("Outside time window, skipping offer_generate relay",
				zap.String("automationID", auto.ID))
			return nil
		}
	}

	// Chip resolution (same logic as processAutomation step 8).
	instances, err := s.instanceRepo.GetUserInstances(ctx, auto.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user instances: %w", err)
	}
	connectedByID := make(map[string]string, len(instances))
	var firstConnected string
	for _, inst := range instances {
		if inst.Status != "connected" {
			continue
		}
		idStr := fmt.Sprintf("%d", inst.ID)
		connectedByID[idStr] = inst.InstanceID
		if firstConnected == "" {
			firstConnected = inst.InstanceID
		}
	}
	if firstConnected == "" {
		s.logger.Warn("No connected WhatsApp instance for offer_generate relay",
			zap.String("automationID", auto.ID),
			zap.String("userID", auto.UserID))
		return nil
	}

	preferenceIDs := make([]string, 0, len(auto.InstanceIDs)+1)
	for _, id := range auto.InstanceIDs {
		if _, ok := connectedByID[id]; ok {
			preferenceIDs = append(preferenceIDs, id)
		}
	}
	if len(preferenceIDs) == 0 && auto.InstanceID != nil && *auto.InstanceID != "" {
		if _, ok := connectedByID[*auto.InstanceID]; ok {
			preferenceIDs = append(preferenceIDs, *auto.InstanceID)
		}
	}
	senderSession := firstConnected
	if len(preferenceIDs) > 0 {
		senderSession = connectedByID[preferenceIDs[0]]
	}

	destinationGroups := auto.SendGroups
	if len(destinationGroups) == 0 {
		destinationGroups = auto.GetTargetGroups()
	}
	if len(destinationGroups) == 0 {
		return nil
	}

	groupParticipants := map[string][]string{}
	if s.groupRepo != nil && len(preferenceIDs) > 0 {
		groups, gerr := s.groupRepo.GetGroupsByWAGroupIDs(ctx, auto.UserID, destinationGroups)
		if gerr != nil {
			s.logger.Warn("Failed to load destination group participants — falling back to default chip",
				zap.String("automationID", auto.ID), zap.Error(gerr))
		} else {
			for _, g := range groups {
				groupParticipants[g.WAGroupID] = g.ParticipatingInstanceIDs
			}
		}
	}

	chipForGroup := func(waGroupID string) string {
		return PickChipForGroup(ChipRoutingInput{
			WaGroupID:         waGroupID,
			PreferenceIDs:     preferenceIDs,
			GroupParticipants: groupParticipants[waGroupID],
			ConnectedByID:     connectedByID,
			ChipOverrides:     auto.ChipOverrides,
			SenderSession:     senderSession,
		})
	}

	// Canonical key pro relay: prefixo "relay:" + hash do MessageID. O
	// hash do MessageID é único o suficiente pra dedupar reenvios em
	// janela curta sem fundir mensagens distintas. O prefixo distingue
	// das chaves do fluxo de oferta em queries de analytics.
	canonicalKey := fmt.Sprintf("relay:%s", hashStr(msg.MessageID+":"+msg.RemoteJID))
	const dedupeWindow = 30 * time.Minute

	for _, groupID := range destinationGroups {
		if groupID == msg.RemoteJID {
			continue
		}

		dispatchSession := chipForGroup(groupID)
		if dispatchSession == "" {
			s.logger.Info("No preferred chip participates in destination group, skipping relay",
				zap.String("automationID", auto.ID),
				zap.String("toGroup", groupID))
			s.logDispatch(ctx, auto, groupID, "", "skipped", "no_chip_in_group", canonicalKey, "")
			continue
		}

		if !s.claimProductSlot(ctx, auto.UserID, groupID, canonicalKey, dedupeWindow) {
			s.logger.Info("Relay message already sent (dedup), skipping",
				zap.String("automationID", auto.ID),
				zap.String("toGroup", groupID),
				zap.String("canonicalKey", canonicalKey))
			s.logDispatch(ctx, auto, groupID, "", "skipped", "dedupe", canonicalKey, "")
			continue
		}

		// Random 1-3s delay pra espalhar envios em fan-out — mantém
		// paridade com o fluxo de oferta.
		delay := 1 + rand.IntN(3)
		if auto.DelaySeconds != nil && *auto.DelaySeconds > 0 {
			delay = *auto.DelaySeconds
		}
		time.Sleep(time.Duration(delay) * time.Second)

		// Marca o body+mídia como nosso próprio outgoing pra que o webhook
		// FromMe que retorna seja distinguido de um chip user postando
		// manualmente no grupo. Pula quando Body+MediaURL ambos vazios
		// (caso degenerado — não deveria existir webhook desse formato).
		ownKey := msg.Body
		if msg.MediaURL != "" {
			ownKey += "\x00" + msg.MediaURL
		}
		if ownKey != "" {
			s.markOwnOutgoing(ctx, dispatchSession, groupID, ownKey)
		}

		sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
		sendErr := s.sendRelayMessage(sendCtx, dispatchSession, groupID, msg)
		sendCancel()

		if sendErr != nil {
			s.releaseProductSlot(ctx, auto.UserID, groupID, canonicalKey)
			s.logDispatch(ctx, auto, groupID, "", "failed", sendErr.Error(), canonicalKey, "")
			s.logger.Error("Failed to send relay message",
				zap.String("groupID", groupID),
				zap.String("session", dispatchSession),
				zap.String("mediaType", msg.MediaType),
				zap.Error(sendErr))
			continue
		}

		s.logDispatch(ctx, auto, groupID, "", "sent", "", canonicalKey, "")
		s.logger.Info("Relay message sent successfully",
			zap.String("fromGroup", msg.RemoteJID),
			zap.String("toGroup", groupID),
			zap.String("session", dispatchSession),
			zap.String("automationID", auto.ID),
			zap.String("mediaType", msg.MediaType))
	}

	return nil
}

// sendRelayMessage dispara o método WAHA correto baseado no tipo da
// mensagem original. Pra mídia, prefere MediaURL (mais leve, sem reupload);
// cai pra MediaBase64 quando URL não está disponível (caso comum no
// link preview JPEG do WhatsApp). Áudio só funciona via Base64 — o input
// SendVoiceInput não tem campo URL.
func (s *offerGenerateService) sendRelayMessage(ctx context.Context, session, chatID string, msg *models.IncomingMessage) error {
	// Sem mídia → texto puro
	if !msg.HasMedia {
		if strings.TrimSpace(msg.Body) == "" {
			// Mensagem sem texto E sem mídia — provavelmente um evento
			// sistema (reaction, edit, etc) que o WAHA encaminhou. Skipa.
			return nil
		}
		_, err := s.waProvider.SendText(ctx, whatsapp.SendTextInput{
			SessionName: session,
			ChatID:      chatID,
			Text:        msg.Body,
		})
		return err
	}

	// A mídia do relay vem do store interno do WAHA (/api/files), protegido
	// por API key. Se passarmos essa URL no sendImage/sendVideo, o WAHA tenta
	// buscá-la sozinho (fileToMedia) SEM a key e leva 401 → 500. Então baixamos
	// a mídia aqui (DownloadMediaByURL reescreve o host e autentica com
	// X-Api-Key) e mandamos como base64. URL fica só como último recurso.
	mediaB64 := msg.MediaBase64
	mediaMime := msg.MimeType
	if mediaB64 == "" && msg.MediaURL != "" {
		if res, derr := s.waProvider.DownloadMediaByURL(ctx, msg.MediaURL); derr == nil && res != nil && len(res.Data) > 0 {
			mediaB64 = base64.StdEncoding.EncodeToString(res.Data)
			if mediaMime == "" {
				mediaMime = res.Mimetype
			}
		} else if derr != nil {
			s.logger.Warn("Relay: failed to download media for base64, falling back to URL",
				zap.String("mediaURL", msg.MediaURL),
				zap.String("mediaType", msg.MediaType),
				zap.Error(derr))
		}
	}

	switch strings.ToLower(msg.MediaType) {
	case "image":
		input := whatsapp.SendImageInput{
			SessionName: session,
			ChatID:      chatID,
			Caption:     msg.Body,
			Mimetype:    mediaMime,
		}
		if mediaB64 != "" {
			input.ImageBase64 = mediaB64
		} else if msg.MediaURL != "" {
			input.ImageURL = msg.MediaURL
		} else {
			return fmt.Errorf("relay image without MediaURL or MediaBase64")
		}
		_, err := s.waProvider.SendImage(ctx, input)
		return err

	case "video":
		input := whatsapp.SendVideoInput{
			SessionName: session,
			ChatID:      chatID,
			Caption:     msg.Body,
		}
		if mediaB64 != "" {
			input.MediaBase64 = mediaB64
		} else if msg.MediaURL != "" {
			input.MediaURL = msg.MediaURL
		} else {
			return fmt.Errorf("relay video without MediaURL or MediaBase64")
		}
		_, err := s.waProvider.SendVideo(ctx, input)
		return err

	case "audio", "voice", "ptt":
		// Voice/PTT só aceita base64. Como agora baixamos a mídia do WAHA
		// quando só veio URL, o caso URL-only também passa a funcionar.
		if mediaB64 == "" {
			s.logger.Warn("Relay audio skipped: no base64 available (download failed?)",
				zap.String("mediaType", msg.MediaType))
			return fmt.Errorf("relay audio without base64")
		}
		mime := mediaMime
		if mime == "" {
			mime = "audio/ogg; codecs=opus"
		}
		_, err := s.waProvider.SendVoice(ctx, whatsapp.SendVoiceInput{
			SessionName: session,
			ChatID:      chatID,
			AudioBase64: mediaB64,
			Mimetype:    mime,
		})
		return err

	case "document", "file":
		input := whatsapp.SendDocumentInput{
			SessionName: session,
			ChatID:      chatID,
			Caption:     msg.Body,
		}
		if mediaB64 != "" {
			input.MediaBase64 = mediaB64
		} else if msg.MediaURL != "" {
			input.MediaURL = msg.MediaURL
		} else {
			return fmt.Errorf("relay document without MediaURL or MediaBase64")
		}
		_, err := s.waProvider.SendDocument(ctx, input)
		return err

	default:
		// Mídia com tipo não resolvido (mediaType e mime ausentes/desconhecidos).
		// Defesa em profundidade: se temos os bytes/URL, tenta enviar como imagem
		// — de longe o caso mais comum no relay — em vez de degradar pra texto
		// puro e perder a mídia silenciosamente. Se falhar, o erro propaga e o
		// dispatch fica logado como "failed".
		if msg.HasMedia && (mediaB64 != "" || msg.MediaURL != "") {
			input := whatsapp.SendImageInput{
				SessionName: session,
				ChatID:      chatID,
				Caption:     msg.Body,
				Mimetype:    mediaMime,
			}
			if mediaB64 != "" {
				input.ImageBase64 = mediaB64
			} else {
				input.ImageURL = msg.MediaURL
			}
			s.logger.Warn("Relay media with unresolved type — sending as image (fallback)",
				zap.String("mediaType", msg.MediaType),
				zap.String("mimeType", msg.MimeType))
			_, err := s.waProvider.SendImage(ctx, input)
			return err
		}
		// Sticker/location/contact sem método dedicado: fallback pra texto se
		// tiver Body, senão skipa com warning.
		if strings.TrimSpace(msg.Body) != "" {
			_, err := s.waProvider.SendText(ctx, whatsapp.SendTextInput{
				SessionName: session,
				ChatID:      chatID,
				Text:        msg.Body,
			})
			return err
		}
		s.logger.Info("Relay skipped: unsupported media type without text fallback",
			zap.String("mediaType", msg.MediaType))
		return nil
	}
}

// buildRelayContentKey é uma variante do buildContentKey que inclui a
// MediaURL no hash. Em relay, mídias sem caption (Body="") são comuns —
// se 3 fotos diferentes caem no grupo de origem sem texto, queremos
// processar as 3 (não dedupar pela colisão de hash do body vazio).
func (s *offerGenerateService) buildRelayContentKey(msg *models.IncomingMessage) string {
	h := sha256.Sum256([]byte(msg.Body + "\x00" + msg.MediaURL + "\x00" + msg.MimeType))
	return fmt.Sprintf("og_relay_content:%s:%s", msg.RemoteJID, hex.EncodeToString(h[:8]))
}
