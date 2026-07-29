package offer_generate

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/lib/ctapicker"
	"automatiza-go/internal/lib/humanize"
	"automatiza-go/internal/lib/pacing"
	"automatiza-go/internal/lib/selfsend"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/lib/videostream"
	"automatiza-go/internal/lib/wamentions"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/failover"
	"automatiza-go/internal/services/mercadolivre"
	"automatiza-go/internal/services/shopee"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// Service defines the offer generate service interface
type Service interface {
	ProcessIncomingMessage(ctx context.Context, msg *models.IncomingMessage) error
	// ProcessReconciledMessage processa uma mensagem RECUPERADA pelo backfill
	// (webhook perdido / crash no meio do fan-out). Pula os claims de nível
	// mensagem (og_content 2min / og_msg_dedup 10min) e confia no claim
	// fail-closed POR DESTINO (claimProductSlot, janela ≥30min) + guards de
	// loop (og_self_send/selfsend), que seguem ativos. O chamador garante
	// frescor ≤ MaxBackfillWindow (10min).
	ProcessReconciledMessage(ctx context.Context, msg *models.IncomingMessage) error
}

// CouponResult contains the result of applying a coupon to an offer.
type CouponResult struct {
	Applied         bool
	CouponText      string
	CouponCode      string
	SkipReason      string
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
	ApplyCoupon(ctx context.Context, automation *models.Automation, offerPrice float64, offerStore string, productIsOfficial bool, productCategoryIDs []int64) (*CouponResult, error)
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

	// dispatchGate é a barreira contra troca de chip em andamento (Melhoria 5).
	// nil = sem gate (comportamento histórico).
	dispatchGate DispatchGate
}

// DispatchGate é a fatia do failover que este produtor consome. Definida aqui,
// no consumidor, para o offer-generate não depender do pacote failover.
type DispatchGate interface {
	Acquire(ctx context.Context, userID, automationID string) (*failover.Lease, error)
}

// SetDispatchGate liga o gate de failover (Melhoria 5) neste serviço.
func (s *offerGenerateService) SetDispatchGate(gate DispatchGate) {
	s.dispatchGate = gate
}

// acquireLease pega UMA lease para a automação e devolve a versão relida dela.
//
// Uma só: o caminho de oferta pode delegar para o relay no meio do percurso, e
// duas leases para a mesma automação no mesmo fluxo não protegem nada a mais —
// só dobram o custo e criam a chance de a segunda falhar com a primeira já
// segurando trabalho em voo.
func (s *offerGenerateService) acquireLease(ctx context.Context, auto *models.Automation) (*failover.Lease, *models.Automation, error) {
	if s.dispatchGate == nil {
		return &failover.Lease{Ctx: ctx, SendCtx: context.Background(), Release: func() {}}, auto, nil
	}
	lease, err := s.dispatchGate.Acquire(ctx, auto.UserID, auto.ID)
	if err != nil {
		s.logger.Warn("offer_generate: automation blocked by failover gate",
			zap.String("automationID", auto.ID), zap.Error(err))
		return nil, nil, err
	}
	// Reler SOB a lease: entre a listagem e agora, uma troca pode ter reescrito
	// o roteamento desta automação.
	fresh, err := s.automationRepo.GetAutomationByIDAndUser(lease.Ctx, auto.ID, auto.UserID)
	if err != nil {
		lease.Release()
		return nil, nil, err
	}
	if fresh == nil {
		lease.Release()
		return nil, nil, nil
	}
	return lease, fresh, nil
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
	return s.processIncoming(ctx, msg, false)
}

// ProcessReconciledMessage — ver contrato na interface. fromReconcile=true
// pula apenas os claims de nível mensagem; selfsend/og_self_send e o claim
// por destino seguem idênticos ao caminho vivo.
func (s *offerGenerateService) ProcessReconciledMessage(ctx context.Context, msg *models.IncomingMessage) error {
	return s.processIncoming(ctx, msg, true)
}

func (s *offerGenerateService) processIncoming(ctx context.Context, msg *models.IncomingMessage, fromReconcile bool) error {
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

	// Disparo da PRÓPRIA plataforma chegando no grupo-fonte (lista/busca,
	// relay/oferta de outro espelho, inclusive de outro chip ou usuário):
	// não é post manual, então não processa — re-espelhar encadearia
	// tráfego automatizado no ritmo do disparador de origem. Antes da
	// extração de links de propósito, para cobrir também o modo relay.
	if selfsend.IsPlatformSend(ctx, s.cache, msg.RemoteJID, msg.Body) {
		s.logger.Info("Offer generate: ignoring platform-originated message in monitor group",
			zap.String("instance", msg.InstanceName),
			zap.String("group", msg.RemoteJID))
		return nil
	}

	// 2. Extract links from message body. info[...] and cta[...] must be
	// stripped first so URLs embedded inside those tags (typically a coupon
	// link) are not mistaken for a separate product link.
	bodyForLinks := text.StripCTATag(text.StripInfoTag(msg.Body))
	links := text.ExtractLinks(bodyForLinks)
	if len(links) == 0 {
		// Nenhum link de produto detectado: cai no modo "relay" — repassa
		// a mensagem original (texto, foto, áudio, vídeo, documento) pros
		// grupos de destino exatamente como veio, sem processamento de
		// afiliado/cupom. O grupo de origem agora aceita qualquer tipo
		// de conteúdo informativo, não só links.
		return s.processRelay(ctx, msg, fromReconcile)
	}

	s.logger.Info("Processing incoming message for offer generation",
		zap.String("instance", msg.InstanceName),
		zap.String("group", msg.RemoteJID),
		zap.Int("links", len(links)),
		zap.Int("bodyLen", len(msg.Body)),
		zap.String("bodyHash", commerceurl.Hash8(msg.Body)))

	// Resolve o DONO do chip que entregou a mensagem ANTES do dedup — as chaves
	// abaixo TÊM que ser escopadas por ele (mesmo fix do monitoring, #245): num
	// grupo-fonte popular, o webhook chega pelo chip de TODO membro; keyar só
	// por (grupo, hash/msgID) fazia o PRIMEIRO chip a processar (de QUALQUER
	// usuário, inclusive um sem automação aqui) consumir o claim e ESFOMEAR os
	// demais usuários que monitoram o mesmo grupo. Escopar por dono ainda
	// colapsa os N chips do MESMO dono; fallback pro InstanceName nunca colide
	// global.
	ownerUserID := ""
	dedupScope := msg.InstanceName
	if inst, ierr := s.instanceRepo.GetInstanceByName(ctx, msg.InstanceName); ierr == nil && inst != nil {
		ownerUserID = inst.UserID
		if inst.UserID != "" {
			dedupScope = inst.UserID
		}
	}

	// 3. Content fingerprint dedup (2 min window).
	//
	// Keyed by (dono, group, body-hash) — sem o instance — pra que mensagens
	// vistas por múltiplos chips do MESMO dono no grupo monitor disparem só 1
	// processamento. Se 4 chips do user estão no grupo de origem, WAHA
	// fira 4 webhooks; o primeiro marca a key, os outros 3 batem aqui
	// e fazem bailout antes da chamada à API da Shopee/ML — evita 4×
	// rate limit + spam de "skipped:dedupe" no dispatch_logs.
	//
	// claimProductSlot() no loop de dispatch segue como cinto de
	// segurança por (user, destinationGroup, canonicalKey), então
	// mesmo que esse pré-dedup falhe (Redis offline), nunca há
	// duplicação real no destino.
	// Claims de nível mensagem — pulados no replay do backfill (a passagem
	// original pode tê-los reivindicado e morrido no meio do fan-out); a
	// idempotência do replay é o claimProductSlot por destino.
	if !fromReconcile {
		contentKey := s.buildContentKey(dedupScope, msg.RemoteJID, msg.Body)
		if !s.claimOnce(ctx, contentKey, 2*time.Minute) {
			s.logger.Debug("Offer generate: duplicate content, skipping")
			return nil
		}
	}

	// 4. Message ID dedup (10 min window). Mesma motivação do (3) —
	// keyed por (dono, group, msgID), não por instance.
	msgKey := fmt.Sprintf("og_msg_dedup:%s:%s:%s", dedupScope, msg.RemoteJID, msg.MessageID)
	if !fromReconcile && !s.claimOnce(ctx, msgKey, 10*time.Minute) {
		s.logger.Debug("Offer generate: duplicate message ID, skipping")
		return nil
	}

	// 5. Find matching offer_generate automations for this group (all users).
	automations, err := s.automationRepo.GetOfferGenerateAutomationsForGroup(ctx, msg.RemoteJID)
	if err != nil {
		return err
	}

	// Scope to the user whose session actually delivered this message — i.e.
	// whose chip is in the source group. A popular source group is delivered by
	// EVERY member's session, so without this a user who LEFT the group keeps
	// relaying it as long as anyone else's chip remains (cross-user leak).
	// (ownerUserID já resolvido acima, junto com o dedupScope.)

	// Filter to active, owned-by-delivering-session automations.
	var active []models.Automation
	for _, a := range automations {
		if !a.IsActive() {
			continue
		}
		if ownerUserID != "" && a.UserID != ownerUserID {
			continue
		}
		active = append(active, a)
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

// processAutomation adquire a lease de dispatch e delega. É o ÚNICO ponto de
// aquisição deste caminho: se ele acabar delegando para o relay, o relay roda
// com a MESMA lease (processAutomationRelayLeased), nunca com uma segunda.
func (s *offerGenerateService) processAutomation(ctx context.Context, auto *models.Automation, msg *models.IncomingMessage, links []string) error {
	lease, fresh, err := s.acquireLease(ctx, auto)
	if err != nil {
		return err
	}
	if lease == nil {
		return nil
	}
	defer lease.Release()
	return s.processAutomationLeased(lease.Ctx, lease.SendCtx, fresh, msg, links)
}

func (s *offerGenerateService) processAutomationLeased(ctx, sendRoot context.Context, auto *models.Automation, msg *models.IncomingMessage, links []string) error {
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

	// 2b. Cardinalidade (Melhoria 9, §12.7): a oferta é do PRIMEIRO PRODUTO na
	// ordem do texto, não "Shopee sempre primeiro".
	//
	// O ramo abaixo é `if shopee {...} else if ml {...}`, então um cupom Shopee
	// citado antes de um produto ML roubava a oferta: entrava no ramo Shopee,
	// virava aviso de cupom, e o produto que o cliente queria anunciar nunca
	// era montado. A reordenação usa só classificação LOCAL (host + path, zero
	// I/O) — o mesmo `commerceurl.ClassifyLocal` do planner.
	//
	// Quando nenhum link é produto localmente (ex.: dois short links, que só a
	// expansão resolve), a ordem histórica é preservada: sem prova de produto
	// não há motivo para mudar o desempate.
	// shopeeCarryOver leva os links Shopee para o ramo ML quando a oferta é
	// dele. Sem isso, o aviso de cupom do ML (que PRESERVA o corpo original)
	// deixaria os links Shopee crus na mensagem — vazando a comissão para quem
	// postou no grupo-fonte.
	var shopeeCarryOver []string
	if firstProductStore(links) == models.MonitorStoreMercadoLivr && len(shopeeLinks) > 0 {
		shopeeCarryOver = shopeeLinks
		shopeeLinks = nil
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
		// A mensagem tem link, mas nenhum é de LOJA (Shopee/ML) — ex: convite
		// de grupo, Instagram, site. Em vez de dropar, cai no relay: repassa a
		// original como veio pros destinos DESTA automação, igual ao caminho
		// sem-link (ProcessIncomingMessage:139). Consistência do "gerar oferta
		// repassa qualquer conteúdo": só link de loja é enriquecido, o resto é
		// repassado. processAutomationRelay já deduplica por destino
		// (claimProductSlot, chave relay:).
		s.logger.Info("No store links matched — relaying original message as-is",
			zap.String("automationID", auto.ID),
			zap.Strings("stores", stores))
		return s.processAutomationRelayLeased(ctx, sendRoot, auto, msg)
	}

	// 4. Parse discount and info tags from message body. A tag cta[...] fica
	// para depois do passo 5: só faz sentido lê-la quando a cópia sorteada
	// autorizar CTA.
	discountTag := text.ParseDiscountTag(msg.Body)
	infoText := text.ParseInfoTag(msg.Body)

	// 5. Load dispatch template. When the automation has no dispatch text
	// configured (or it fails to load), fall back to the system default
	// template so the offer still relays instead of silently dropping. This
	// mirrors the product expectation: offer_generate without a custom text
	// should still work — in prod, 19 active automations had no dispatch text
	// and were silently not relaying because this path returned nil.
	var dispatchText *models.DispatchText
	if auto.DispatchTextID != nil && *auto.DispatchTextID != "" {
		loaded, loadErr := s.settingsRepo.GetActiveDispatchTextByID(ctx, *auto.DispatchTextID, auto.UserID)
		if loadErr != nil || loaded == nil {
			s.logger.Warn("Failed to load dispatch text, falling back to system default",
				zap.String("automationID", auto.ID),
				zap.Error(loadErr))
		} else {
			dispatchText = loaded
		}
	}
	if dispatchText == nil {
		dispatchText = &models.DispatchText{Body: text.DefaultShopeeTemplate()}
		s.logger.Info("Using system default dispatch template for offer_generate",
			zap.String("automationID", auto.ID))
	}
	// Sorteia entre as variações (copies) a cada disparo. Sem isso só a 1ª
	// variação (Body) era usada, mesmo com N cadastradas. Os renders abaixo
	// (Shopee/ML) usam dispatchText.Body.
	if len(dispatchText.Copies) > 0 {
		dispatchText.Body = dispatchText.Copies[rand.IntN(len(dispatchText.Copies))] //nolint:gosec // G404: non-security random for copy rotation
	}

	// 6. CTA: estritamente opt-in pela variável {cta} na cópia JÁ sorteada.
	// Sem ela o texto não tem onde receber a CTA (o append pós-render saiu), e
	// nada disso roda — nem o parse do override, nem consulta, nem sorteio.
	useCTA := text.TemplateUsesVariable(dispatchText.Body, text.VarCTA)

	// A tag cta[...] no corpo da mensagem é um override per-send: quando
	// presente, substitui o CTA automático naquele disparo, ignorando pool e
	// inline (espelha o comportamento da tag info[...]).
	ctaOverride := ""
	if useCTA {
		ctaOverride = text.ParseCTATag(msg.Body)
	}

	// Pool carregado LAZY, dentro de pickCTA: os ramos de relay (cupom Shopee,
	// aviso de cupom ML, mensagem sem link de loja) devolvem antes de renderizar
	// oferta e nunca chamam pickCTA — não podem pagar consulta. Política:
	//   - auto.CTAIDs preenchido → override explícito, busca só esses
	//   - auto.CTAIDs vazio → pool inteiro de CTAs ativos do user
	// No máximo uma consulta por mensagem (ctaLoadAttempted), inclusive em erro,
	// o que também mantém o antigo cuidado de não fazer N+1 por link.
	var ctaPool []models.CTA
	ctaLoadAttempted := false
	loadCTAPool := func() []models.CTA {
		if ctaLoadAttempted {
			return ctaPool
		}
		ctaLoadAttempted = true
		if s.ctaRepo == nil {
			return nil
		}
		if len(auto.CTAIDs) > 0 {
			fetched, ctaErr := s.ctaRepo.GetCTAsByIDs(ctx, auto.CTAIDs, auto.UserID)
			if ctaErr != nil {
				s.logger.Warn("Failed to fetch CTA entities by IDs",
					zap.String("automationID", auto.ID),
					zap.Error(ctaErr))
			} else {
				ctaPool = fetched
			}
			return ctaPool
		}
		fetched, ctaErr := s.ctaRepo.GetUserCTAs(ctx, auto.UserID, map[string]interface{}{"is_active": true})
		if ctaErr != nil {
			s.logger.Warn("Failed to fetch user CTA pool",
				zap.String("automationID", auto.ID),
				zap.Error(ctaErr))
		} else {
			ctaPool = fetched
		}
		return ctaPool
	}

	// pickCTA escolhe pela plataforma resolvida (Shopee/ML) e cai pros
	// inline auto.CTAs como fallback se o pool não tiver match. Com override
	// o resultado já está decidido — não vale gastar consulta.
	pickCTA := func(isML, isShopee bool) string {
		if !useCTA {
			return ""
		}
		if ctaOverride != "" {
			return ctaOverride
		}
		if picked := ctapicker.PickByPlatform(loadCTAPool(), isML, isShopee); picked != "" {
			return picked
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
	// couponResult carries the rule-engine outcome for this offer so the
	// dispatch log can audit which coupon (or why none) was applied. It's
	// hoisted here and set in the Shopee/ML branches below.
	var couponResult *CouponResult
	// Dados pro card de preview nativo (usado quando offer_generate_send_as_image
	// está OFF). previewURL é o link de afiliado (também presente no texto), e
	// o título alimenta o card. Preenchidos em cada branch (Shopee/ML).
	// O card NÃO leva descrição: o preço já sai no corpo da mensagem (28/07).
	var previewURL string
	var previewTitle string

	if len(shopeeLinks) > 0 && s.shopeeService != nil {
		link := shopeeLinks[0]
		store = "shopee"

		// Expand short URL if needed
		expandedURL, expandErr := s.shopeeService.ExpandURL(ctx, link)
		if expandErr == nil && expandedURL != "" {
			link = expandedURL
		}

		// Link de cupom/campanha/voucher (ex. shopee.com.br/m/super-ofertas):
		// não é página de produto, então não há o que enriquecer/gerar como
		// oferta. A mensagem ainda é informativa (anúncio de cupom) e o cliente
		// espera que chegue nos grupos destino — repassa (relay) em vez de
		// descartar silenciosamente, mas com os links reatribuídos ao dono da
		// automação. Gateamos pela URL *expandida* já conhecida como não-produto
		// pra não relayar falhas transitórias de ExpandURL/parse de links de
		// produto legítimos.
		if isNonProductShopeeURL(link) {
			s.logger.Info("Shopee link is coupon/campaign (non-product), converting links before relay",
				zap.String("automationID", auto.ID),
				zap.String("link", link))
			return s.relayWithConvertedShopeeLinks(ctx, sendRoot, auto, msg, shopeeLinks)
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
		// Conversão sem imagem (curto-circuito de link próprio): usa a imagem
		// do produto já enriquecido — oferta nunca sai sem imagem tendo dado.
		if imageURL == "" && product.Image != "" {
			imageURL = product.Image
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
			if preloadedCoupon != nil {
				couponResult = s.couponHandler.ApplyCouponWithPreloaded(preloadedCoupon, offerPrice, store)
			} else if cr, err := s.couponHandler.ApplyCoupon(ctx, auto, offerPrice, store, product.IsMall, product.ProductCategoryIDs); err == nil {
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

		// Só passa preço original quando há desconto real (> preço de oferta);
		// senão o template renderiza "De R$ X por R$ X" (mesmo valor).
		shopeeOriginalPrice := 0.0
		if product.OriginalPrice > offerPrice {
			shopeeOriginalPrice = product.OriginalPrice
		}

		// Render template
		renderedMessage = text.RenderShopeeOfferTemplate(dispatchText.Body, text.ShopeeOfferVars{
			Title:         product.Name,
			Price:         offerPrice,
			OriginalPrice: shopeeOriginalPrice,
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

		// Card de preview nativo (caminho send_as_image OFF)
		previewURL = affiliateURL
		previewTitle = product.Name

	} else if len(mlLinks) > 0 && s.mlService != nil {
		link := mlLinks[0]
		store = "mercadolivre"

		// Get product preview
		preview, prevErr := s.mlService.GetProductPreview(ctx, link, auto.UserID)
		if prevErr != nil || !preview.HasProductData() {
			// Não é um produto: link de cupom/campanha ML (meli.la/sec sem
			// item real) ou preview indisponível. Não há oferta a montar —
			// desvia para o relay de aviso de cupom, que PRESERVA o texto
			// original e só troca os links pelo afiliado do dono. Antes, este
			// caminho ou renderizava um card com título/preço errados (preview
			// vazio não-nil) ou fazia `return nil` e a mensagem SUMIA.
			s.logger.Info("ML link is coupon/campaign or preview unusable — relaying as coupon notice",
				zap.String("automationID", auto.ID),
				zap.String("link", link),
				zap.Error(prevErr))
			return s.relayMLCouponNotice(ctx, sendRoot, auto, msg, mlLinks, shopeeCarryOver)
		}

		// ML is passthrough: the user posts their OWN affiliate link (meli.la/sec
		// or an already-affiliated mercadolivre URL), so we relay it as-is — the
		// same way list mode does (see process_list.go: only Shopee is converted
		// on the user's behalf). No affiliate-link generation needed; trying to
		// (re)generate one for a short link used to fail and DROP the offer.
		affiliateURL := link
		imageURL = preview.Image

		// Apply discount tag to price
		offerPrice := preview.Price
		if discountTag != nil {
			offerPrice = text.ApplyDiscountTag(offerPrice, discountTag)
		}

		// Apply coupon: discount-tag override first, then rule engine.
		// ML doesn't expose a Mall-style flag — productIsOfficial=false —
		// nor official categories: nil deixa só as regras genéricas.
		couponBlock := ""
		if s.couponHandler != nil {
			if preloadedCoupon != nil {
				couponResult = s.couponHandler.ApplyCouponWithPreloaded(preloadedCoupon, offerPrice, store)
			} else if cr, err := s.couponHandler.ApplyCoupon(ctx, auto, offerPrice, store, false, nil); err == nil {
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

		// Só passa preço original quando há desconto real (> preço de oferta);
		// senão o template renderiza "De R$ X por R$ X". O preview às vezes não
		// traz original, e o default acima iguala ao offerPrice.
		mlOriginalPrice := 0.0
		if originalPrice > offerPrice {
			mlOriginalPrice = originalPrice
		}

		renderedMessage = text.RenderMLOfferTemplate(dispatchText.Body, text.MLOfferVars{
			Title:         preview.Title,
			Price:         offerPrice,
			OriginalPrice: mlOriginalPrice,
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

		// Card de preview nativo (caminho send_as_image OFF)
		previewURL = affiliateURL
		previewTitle = preview.Title
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
			WaGroupID:         waGroupID,
			PreferenceIDs:     preferenceIDs,
			GroupParticipants: groupParticipants[waGroupID],
			ConnectedByID:     connectedByID,
			ChipOverrides:     auto.ChipOverrides,
			SenderSession:     senderSession,
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

	// Fan out in PARALLEL BY CHIP. Each destination group is hosted by exactly
	// one of the user's chips; the old single serial loop sent to one group at
	// a time, so with 131 groups × delay the LAST group only got the offer
	// after minutes — while the other 12 chips sat idle. Grouping by chip lets
	// every chip fan out at once. The per-group delay still paces EACH chip
	// independently, so per-chip anti-ban behavior is unchanged; only the
	// cross-chip waiting is removed (≈ Nchips× faster for the last group).
	chipGroups := make(map[string][]string)
	for _, groupID := range destinationGroups {
		if groupID == msg.RemoteJID {
			continue // skip source group
		}
		dispatchSession := chipForGroup(groupID)
		if dispatchSession == "" {
			s.logger.Info("No preferred chip participates in destination group, skipping",
				zap.String("automationID", auto.ID),
				zap.String("toGroup", groupID))
			s.logDispatch(ctx, auto, groupID, links[0], "skipped", "no_chip_in_group", canonicalKey, store)
			continue
		}
		chipGroups[dispatchSession] = append(chipGroups[dispatchSession], groupID)
	}

	// Anti-ban: resolve pacing/humanize preset once; shared read-only across the
	// per-chip goroutines below (resolved/rt/baseDelay are never mutated).
	resolved := config.GetHumanization().ForMode(auto.HumanizeMode)
	rt := config.GetAntiBanRuntime()
	baseDelay := 0
	if auto.DelaySeconds != nil && *auto.DelaySeconds > 0 {
		baseDelay = *auto.DelaySeconds
	}
	// Espelho tem prazo: a oferta precisa chegar em TODOS os grupos em <1min.
	// Deadline compartilhado entre os goroutines; cada chip aplica o orçamento
	// à sua fatia (fan-out paralelo → a fatia por chip é pequena).
	fanoutDeadline := resolved.Pacing.FanoutDeadline()
	// Digitação capada no espelho: até 12s/grupo do preset, serial por chip,
	// estourava sozinha o teto de 1 min com poucos chips conectados.
	mirrorHumanize := resolved.Humanize
	if mirrorHumanize.MaxTyping > 2500*time.Millisecond {
		mirrorHumanize.MaxTyping = 2500 * time.Millisecond
	}
	var wg sync.WaitGroup
	for dispatchSession, groupIDs := range chipGroups {
		wg.Add(1)
		go func(dispatchSession string, groupIDs []string) {
			defer wg.Done()
			// recover: panic numa goroutine NÃO é capturado pelo middleware do
			// asynq (que só cobre a goroutine do handler) — sem isto, um panic
			// aqui derrubava o worker inteiro (40 slots, todas as filas).
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("Panic in offer_generate chip fan-out",
						zap.String("session", dispatchSession),
						zap.Any("panic", r))
				}
			}()
			// Shuffle this chip's destination order (anti-ban).
			resolved.Pacing.Shuffle(groupIDs)
			for gi, groupID := range groupIDs {
				// Anti-ban: skip groups with a bad reputation (fail-open).
				if rt.Reputation.IsSuppressed(ctx, dispatchSession, groupID) {
					s.logDispatch(ctx, auto, groupID, links[0], "skipped", "reputacao (grupo problematico)", canonicalKey, store)
					continue
				}
				// Dedup by (userID, groupID, canonicalKey). claimProductSlot,
				// logDispatch, markOwnOutgoing and the provider sends are all
				// concurrency-safe (Redis SetNX / DB / per-session HTTP), and
				// every shared value read here is read-only.
				if !s.claimProductSlot(ctx, auto.UserID, groupID, canonicalKey, dedupeWindow) {
					s.logger.Info("Product already sent (dedup), skipping",
						zap.String("automationID", auto.ID),
						zap.String("toGroup", groupID),
						zap.String("canonicalKey", canonicalKey))
					s.logDispatch(ctx, auto, groupID, links[0], "skipped", "dedupe", canonicalKey, store)
					continue
				}

				// Anti-ban pacing: jittered per-chip delay (honors delay_seconds)
				// plus an occasional micro-break, keyed on the sending chip —
				// clamped so this chip's slice still fits the fan-out budget.
				// Timer cancelável, não time.Sleep: um sleep surdo acordaria para
				// enviar mesmo depois de a lease ter sido perdida no meio do fan-out.
				if !pacing.Sleep(sendRoot, pacing.BudgetedGap(
					resolved.Pacing.Step(baseDelay)+rt.Pacing.MicroBreak(ctx, dispatchSession, resolved.Pacing),
					fanoutDeadline, len(groupIDs)-gi)) {
					s.logger.Warn("offer_generate: dispatch lease lost mid fan-out, stopping",
						zap.String("automationID", auto.ID))
					return
				}

				// Anti-ban: per-recipient content variation (URLs never mutated).
				// markOwnOutgoing MUST key on the VARIED body — that's the exact text
				// WhatsApp echoes back — or the FromMe loop-guard misses it.
				perGroupMessage := text.Vary(renderedMessage, resolved.Vary, groupID)
				s.markOwnOutgoing(ctx, dispatchSession, groupID, perGroupMessage)
				// Marca global (grupo + hash): destino que também é grupo-fonte
				// de outro espelhamento não re-espelha este envio automatizado.
				selfsend.Mark(ctx, s.cache, groupID, perGroupMessage)

				// Send via WhatsApp
				// Raiz do envio: Background quando não há gate (o cancelamento do
				// consumidor de webhook nunca abortou um envio em andamento) e o
				// contexto da lease quando há — perder a lease PRECISA parar o fan-out.
				sendCtx, sendCancel := context.WithTimeout(sendRoot, 30*time.Second)
				var sendErr error
				// send_as_image default TRUE (preserva o comportamento histórico
				// do offer_generate: sempre imagem quando há). OFF usa preview nativo.
				sendAsImage := true
				if auto.OfferGenerateSendAsImage != nil {
					sendAsImage = *auto.OfferGenerateSendAsImage
				}

				// @todos no repasse: se o POST de ORIGEM traz "@todos", pinga todos
				// os membros do grupo de DESTINO via mentionedJid. Opt-in por
				// mensagem (anti-ban). Calculado uma vez por destino via a sessão
				// que envia.
				var mentions []string
				if wamentions.Contains(msg.Body) {
					mentions = wamentions.Expand(sendCtx, s.waProvider, dispatchSession, groupID, msg.Body)
				}
				hasMentions := len(mentions) > 0

				// Anti-ban: human presence choreography (typing/online/seen) before
				// the real send, using the varied per-group text.
				stopTyping := rt.Humanizer.BeforeSend(sendCtx, humanize.Plan{
					Session:  dispatchSession,
					ChatID:   groupID,
					Text:     perGroupMessage,
					HasMedia: imageURL != "",
					Cfg:      mirrorHumanize,
				})

				usedPreviewCard := false
				switch {
				case hasMentions && imageURL != "":
					// @todos precisa de menção e o card do WAHA a dropa: força
					// imagem+legenda (honra mentionedJid) seja qual for o toggle.
					_, sendErr = s.waProvider.SendImage(sendCtx, whatsapp.SendImageInput{
						SessionName: dispatchSession,
						ChatID:      groupID,
						Caption:     perGroupMessage,
						ImageURL:    imageURL,
						Mentions:    mentions,
					})
				case hasMentions:
					// @todos sem imagem: texto com menção (o card também não serve).
					_, sendErr = s.waProvider.SendText(sendCtx, whatsapp.SendTextInput{
						SessionName: dispatchSession,
						ChatID:      groupID,
						Text:        perGroupMessage,
						Mentions:    mentions,
					})
				case imageURL != "" && sendAsImage:
					_, sendErr = s.waProvider.SendImage(sendCtx, whatsapp.SendImageInput{
						SessionName: dispatchSession,
						ChatID:      groupID,
						Caption:     perGroupMessage,
						ImageURL:    imageURL,
					})
				case previewURL != "" && (!sendAsImage || imageURL == ""):
					usedPreviewCard = true
					// Preview nativo: card com título/preço/thumb em vez de anexo.
					// Também é o caminho quando sendAsImage está LIGADO mas nenhuma
					// imagem foi resolvida (link próprio em curto-circuito, 10030 no
					// enriquecimento): o card mostra a imagem do produto via og: do
					// WhatsApp — oferta nunca sai texto puro tendo link.
					_, sendErr = s.waProvider.SendLinkCustomPreview(sendCtx, whatsapp.SendLinkPreviewInput{
						SessionName: dispatchSession,
						ChatID:      groupID,
						Text:        perGroupMessage,
						URL:         previewURL,
						Title:       previewTitle,
						Description: "", // card sem preço: o De/Por já sai no corpo
						ImageURL:    imageURL,
					})
				default:
					_, sendErr = s.waProvider.SendText(sendCtx, whatsapp.SendTextInput{
						SessionName: dispatchSession,
						ChatID:      groupID,
						Text:        perGroupMessage,
					})
				}

				// O endpoint de card do WAHA (GOWS) devolve 500 intermitente. Falha
				// DEFINITIVA do card = o envio comprovadamente não saiu, então cair
				// para imagem/texto é seguro (não duplica) e salva o repasse.
				// Timeout NÃO cai aqui — entrega incerta, tratada abaixo.
				if sendErr != nil && usedPreviewCard && !whatsapp.IsAmbiguousOutcome(sendErr) {
					s.logger.Warn("Preview card failed definitively, falling back to image/text",
						zap.String("groupID", groupID),
						zap.Error(sendErr))
					if imageURL != "" {
						_, sendErr = s.waProvider.SendImage(sendCtx, whatsapp.SendImageInput{
							SessionName: dispatchSession,
							ChatID:      groupID,
							Caption:     perGroupMessage,
							ImageURL:    imageURL,
						})
					} else {
						_, sendErr = s.waProvider.SendText(sendCtx, whatsapp.SendTextInput{
							SessionName: dispatchSession,
							ChatID:      groupID,
							Text:        perGroupMessage,
						})
					}
				}
				stopTyping()
				sendCancel()

				if sendErr != nil {
					// Timeout/cancelamento = entrega incerta: liberar o slot deixa
					// um repost do mesmo produto reenviar no grupo. Slot mantido;
					// só falha DEFINITIVA (envio comprovadamente não saiu) libera.
					if whatsapp.IsAmbiguousOutcome(sendErr) {
						s.logDispatch(ctx, auto, groupID, links[0], "failed",
							"timeout no envio (entrega incerta) - slot de dedup mantido", canonicalKey, store, couponDispatchExtra(couponResult))
						s.logger.Warn("Offer send timed out — delivery uncertain, keeping dedup slot",
							zap.String("groupID", groupID),
							zap.String("session", dispatchSession),
							zap.Error(sendErr))
						continue
					}
					s.releaseProductSlot(ctx, auto.UserID, groupID, canonicalKey)
					s.logDispatch(ctx, auto, groupID, links[0], "failed", sendErr.Error(), canonicalKey, store, couponDispatchExtra(couponResult))
					s.logger.Error("Failed to send offer",
						zap.String("groupID", groupID),
						zap.String("session", dispatchSession),
						zap.Error(sendErr))
					continue
				}

				s.logDispatch(ctx, auto, groupID, links[0], "sent", "", canonicalKey, store, couponDispatchExtra(couponResult))

				if hasMentions {
					// Observabilidade anti-ban: quantos @todos-relays saem por hora.
					s.logger.Info("Relay @todos mention dispatched",
						zap.String("automationID", auto.ID),
						zap.String("toGroup", groupID),
						zap.Int("mentionCount", len(mentions)))
				}

				s.logger.Info("Offer generated and sent successfully",
					zap.String("fromGroup", msg.RemoteJID),
					zap.String("toGroup", groupID),
					zap.String("session", dispatchSession),
					zap.String("automationID", auto.ID),
					zap.String("canonicalKey", canonicalKey))
			}
		}(dispatchSession, groupIDs)
	}
	wg.Wait()

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
func (s *offerGenerateService) buildContentKey(scope, group, body string) string {
	h := sha256.Sum256([]byte(body))
	return fmt.Sprintf("og_content:%s:%s:%s", scope, group, hex.EncodeToString(h[:8]))
}

// isNonProductShopeeURL detecta links Shopee que NÃO são página de produto
// (cupom, campanha/super-ofertas, mall, flash sale, voucher, perfil de loja).
// Esses links não têm shopID/itemID extraíveis, então o fluxo de oferta não
// consegue enriquecê-los — devem cair no relay (repasse como veio).
//
// Espelha shopee.isNonProductShopeeURL (shopee_service.go). Mantido em
// paralelo de propósito: expor isso como método na interface shopee.Service
// obrigaria atualizar os vários fakes que a implementam nos testes. Se a
// lista canônica mudar lá, atualizar aqui também.
func isNonProductShopeeURL(expandedURL string) bool {
	lower := strings.ToLower(expandedURL)
	return strings.Contains(lower, "shopee.com.br/m/") ||
		strings.Contains(lower, "shopee.com.br/mall/") ||
		strings.Contains(lower, "shopee.com.br/flash_sale") ||
		strings.Contains(lower, "shopee.com.br/daily_discover") ||
		strings.Contains(lower, "shopee.com.br/voucher") ||
		strings.Contains(lower, "shopee.com.br/user/") ||
		strings.Contains(lower, "shopee.com.br/oficial") ||
		// Ressincronizado com a canônica (15/07): sem estes dois, coleção e
		// shopeevip caíam no ExtractProductIDs, voltavam nil e a mensagem
		// sumia sem log de descarte.
		strings.Contains(lower, "shopee.com.br/collections/") ||
		strings.Contains(lower, "shopee.com.br/shopeevip")
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

// claimOnce atomically marks `key` as processed for `ttl` and returns true only
// for the FIRST caller within that window. It replaces the old isDuplicate()
// (GET) + markProcessed() (SET) pair, whose check-then-set GAP let concurrent
// duplicate deliveries all slip through: the same source message seen by N
// chips in a monitor group (WAHA fires one webhook per chip) — or a double
// webhook — would each pass the GET before any of them ran the SET, so all N
// processings fanned the same offer out to the destination groups, producing
// the same-second duplicate sends. SetNX closes that gap (cache.go does a real
// Redis SETNX).
//
// Fail-open: with no cache, or on a Redis error, returns true (process) so a
// real message is never silently dropped; claimProductSlot still backstops
// duplication at dispatch time in that degraded case.
func (s *offerGenerateService) claimOnce(ctx context.Context, key string, ttl time.Duration) bool {
	if s.cache == nil {
		return true
	}
	claimed, err := s.cache.SetNX(ctx, key, "1", ttl)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "offer_generate dedup SetNX failed, processing fail-open", err, map[string]string{
			"module": "offer_generate", "operation": "dedup_claim",
		})
		return true
	}
	return claimed
}

func (s *offerGenerateService) claimProductSlot(ctx context.Context, userID, groupID, canonicalKey string, window time.Duration) bool {
	if canonicalKey == "" {
		return true
	}

	// Contexto PRÓPRIO (não o da task): em fan-outs longos o ctx da task
	// expira e o SetNX falhava com "context deadline exceeded" — o claim
	// caía no fallback de DB (janela de corrida) e a mesma promoção saía
	// mais de uma vez no grupo. Dedup não pode morrer junto com a task.
	opCtx, opCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer opCancel()

	cacheKey := fmt.Sprintf("og_prod_dedup:%s:%s:%s", userID, groupID, canonicalKey)

	if s.cache != nil {
		wasSet, err := s.cache.SetNX(opCtx, cacheKey, "1", window)
		if err != nil {
			// FAIL-CLOSED: sem SetNX atômico não há proteção de concorrência —
			// suprimir este envio é melhor que duplicar promoção no grupo.
			s.logger.Warn("offer_generate dedup SetNX failed — send suppressed (fail-closed)",
				zap.String("groupID", groupID),
				zap.String("canonicalKey", canonicalKey),
				zap.Error(err))
			return false
		}
		if !wasSet {
			return false
		}
	}

	// Check DB for historical dispatches (Redis pode ter perdido a chave em
	// restart/eviction; o claim no Redis acima já protege a concorrência).
	if s.dispatchLogRepo != nil {
		log, err := s.dispatchLogRepo.GetRecentByCanonicalKey(opCtx, userID, groupID, canonicalKey, window)
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
	// Contexto próprio: o release precisa funcionar mesmo com o ctx da task
	// já estourado, senão o slot vaza até o TTL.
	opCtx, opCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer opCancel()
	cacheKey := fmt.Sprintf("og_prod_dedup:%s:%s:%s", userID, groupID, canonicalKey)
	_ = s.cache.Delete(opCtx, cacheKey)
}

func (s *offerGenerateService) logDispatch(ctx context.Context, auto *models.Automation, groupID, productURL, status, errorMsg, canonicalKey, store string, extras ...func(*models.DispatchLog)) {
	if s.dispatchLogRepo == nil {
		return
	}

	// Skips de dedupe eram 96% dos inserts em dispatch_logs (~63/s) e ninguém
	// os consome (dedup lê só sent/queued/pending; suporte agrega sem skips).
	// Kill-switch: LOG_DEDUPE_SKIPS=true restaura o comportamento antigo.
	if status == "skipped" && strings.HasPrefix(errorMsg, "dedupe") && !config.LogDedupeSkips() {
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

	for _, extra := range extras {
		if extra != nil {
			extra(dispatchLog)
		}
	}

	if _, err := s.dispatchLogRepo.Create(ctx, dispatchLog); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to create offer_generate dispatch log", err, map[string]string{
			"module": "offer_generate", "operation": "dispatch_log_create",
		})
	}
}

// couponDispatchExtra devolve um mutator que audita no dispatch log qual cupom
// (ou por que nenhum) saiu neste disparo. nil-safe: cr nil (cupom não avaliado)
// deixa as colunas NULL. A semântica das colunas mora em SetCouponAudit.
func couponDispatchExtra(cr *CouponResult) func(*models.DispatchLog) {
	return func(log *models.DispatchLog) {
		if cr == nil {
			return
		}
		log.SetCouponAudit(cr.Applied, cr.CouponCode, cr.SkipReason)
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
func (s *offerGenerateService) processRelay(ctx context.Context, msg *models.IncomingMessage, fromReconcile bool) error {
	s.logger.Info("Processing incoming message for relay (no product link)",
		zap.String("instance", msg.InstanceName),
		zap.String("group", msg.RemoteJID),
		zap.Bool("hasMedia", msg.HasMedia),
		zap.String("mediaType", msg.MediaType),
		zap.Int("bodyLen", len(msg.Body)),
		zap.String("bodyHash", commerceurl.Hash8(msg.Body)))

	// Escopo por dono ANTES dos claims — mesmo racional do fluxo principal
	// (chip de outro usuário sem automação não pode consumir o claim).
	ownerUserID := ""
	dedupScope := msg.InstanceName
	if inst, ierr := s.instanceRepo.GetInstanceByName(ctx, msg.InstanceName); ierr == nil && inst != nil {
		ownerUserID = inst.UserID
		if inst.UserID != "" {
			dedupScope = inst.UserID
		}
	}

	// Claims de nível mensagem — pulados no replay do backfill (mesmo racional
	// do fluxo principal); o claimProductSlot por destino (canonical
	// relay:hash(msgID:grupo), janela 30min) segue garantindo a idempotência.
	if !fromReconcile {
		// Content fingerprint dedup (2 min) — mesma chave do fluxo principal
		// (buildContentKey hashea o Body). Pra mídia sem caption (Body=""),
		// inclui a MediaURL no hash pra evitar que duas mídias sem legenda
		// colidam na mesma janela.
		contentKey := s.buildRelayContentKey(dedupScope, msg)
		if !s.claimOnce(ctx, contentKey, 2*time.Minute) {
			s.logger.Debug("Offer relay: duplicate content, skipping")
			return nil
		}

		// Message ID dedup (10 min) — idêntico ao fluxo principal.
		msgKey := fmt.Sprintf("og_relay_msg_dedup:%s:%s:%s", dedupScope, msg.RemoteJID, msg.MessageID)
		if !s.claimOnce(ctx, msgKey, 10*time.Minute) {
			s.logger.Debug("Offer relay: duplicate message ID, skipping")
			return nil
		}
	}

	automations, err := s.automationRepo.GetOfferGenerateAutomationsForGroup(ctx, msg.RemoteJID)
	if err != nil {
		return err
	}

	// (ownerUserID já resolvido acima, junto com o dedupScope.)

	var active []models.Automation
	for _, a := range automations {
		if !a.IsActive() {
			continue
		}
		if ownerUserID != "" && a.UserID != ownerUserID {
			continue
		}
		active = append(active, a)
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

// relayWithConvertedShopeeLinks repassa uma mensagem de cupom/campanha trocando
// os links Shopee da FONTE pelos do dono da automação. Sem a troca, o link
// repassado carrega o mmp_pid de quem postou no grupo monitorado e a Shopee
// credita a comissão pra ELE — o cliente espalha a oferta nos grupos dele de
// graça (medido 15/07: 22 cupons/dia × 164 grupos ≈ 3.6k envios numa conta só,
// todos creditando um afiliado de fora da plataforma).
//
// Regra de falha espelha o monitoring (monitoring_service.go:342-359, :431-439),
// que já resolveu esse mesmo problema no espelho: credencial inválida pausa;
// 10035 (conta sem Open API — conversão impossível para sempre) repassa o
// original, senão o cupom nunca mais sairia; qualquer outra falha (10030,
// timeout) retém o repasse, porque enviar sem converter é trabalhar de graça.
func (s *offerGenerateService) relayWithConvertedShopeeLinks(ctx, sendRoot context.Context, auto *models.Automation, msg *models.IncomingMessage, shopeeLinks []string) error {
	canonicalKey := fmt.Sprintf("relay:%s", hashStr(msg.MessageID+":"+msg.RemoteJID))
	body := msg.Body

	// Substituir do link MAIS LONGO para o mais curto. ExtractLinks dedupa por
	// string exata, então a mesma página pode entrar duas vezes — nua e com
	// querystring ("/m/cupom" e "/m/cupom?mmp_pid=an_X"). Trocando a curta
	// primeiro, o ReplaceAll casa DENTRO da longa e solda o mmp_pid da fonte no
	// shortlink do dono ("s.shopee.com.br/AFF?mmp_pid=an_X"), enquanto a longa
	// vira um no-op silencioso. Do maior para o menor, nenhum candidato é
	// prefixo de um ainda não substituído. (Hoje todo link de cupom que chega é
	// shortlink opaco sem query — 0/182 em 7 dias —, mas isso é propriedade da
	// fonte, não uma garantia nossa.)
	ordered := append([]string(nil), shopeeLinks...)
	sort.SliceStable(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })

	for _, orig := range ordered {
		aff, err := s.shopeeService.GenerateAffiliateLink(ctx, orig, auto.UserID)
		if err != nil {
			if isCredentialError(err) {
				s.pauseWithError(ctx, auto, "Credenciais Shopee inválidas - automação pausada")
				return nil
			}
			// Impossível converter até o usuário agir (conta sem Open API =
			// 10035, ou Shopee nunca configurada — 1 conta com offer_generate
			// ativo em 15/07). Reter seria reter PARA SEMPRE e o grupo nunca
			// mais veria um cupom, então o original passa: sem comissão, mas
			// sem tirar a funcionalidade de quem já vivia assim.
			//
			// errors.Is, NUNCA substring do texto: "credentials not configured"
			// também era o que saía de um timeout de Postgres, e aí um soluço de
			// banco derrubava o gate e mandava o cupom cru pra todo mundo.
			if errors.Is(err, shopee.ErrShopeeNoAPIAccess) ||
				errors.Is(err, shopee.ErrShopeeNoCredentials) {
				s.logger.Warn("Cannot convert coupon link (no Open API access or no credentials) — relaying as-is, commission goes to the source",
					zap.String("automationID", auto.ID),
					zap.String("userID", auto.UserID),
					zap.String("link", orig),
					zap.Error(err))
				continue
			}
			// Sem o que converter (perfil de loja shopee.com.br/<loja>, carrinho):
			// erro ESTRUTURAL, não falha do momento. Reter aqui não protege
			// comissão nenhuma — só apaga a mensagem para sempre. Um aviso de
			// cupom costuma trazer o cupom E o link da loja parceira; reter o
			// conjunto por causa do segundo tirou o cupom de 164 grupos da
			// isaa_deoliveira em 15/07. O cupom converte, este segue como veio.
			if errors.Is(err, shopee.ErrShopeeUnconvertibleURL) {
				s.logger.Warn("Shopee link has nothing to convert (store page/cart) — relaying it as-is, other links still converted",
					zap.String("automationID", auto.ID),
					zap.String("link", orig),
					zap.Error(err))
				continue
			}
			s.logDispatch(ctx, auto, "", orig, "skipped",
				fmt.Sprintf("link de afiliado indisponivel (%v) - repasse retido para nao perder comissao", err),
				canonicalKey, "shopee")
			s.logger.Warn("Coupon relay withheld — affiliate conversion failed",
				zap.String("automationID", auto.ID),
				zap.String("link", orig),
				zap.Error(err))
			return nil
		}
		if aff != nil && aff.AffiliateLink != "" {
			body = strings.ReplaceAll(body, orig, aff.AffiliateLink)
		}
	}

	// Cópia rasa: a MESMA msg é processada para automações de donos diferentes
	// (ProcessIncomingMessage itera as automações que assistem o grupo-fonte),
	// então o body convertido não pode vazar do dono A pro B por mutação
	// compartilhada — cada um repassa com o próprio link.
	relayMsg := *msg
	relayMsg.Body = body
	return s.processAutomationRelayLeased(ctx, sendRoot, auto, &relayMsg)
}

// relayMLCouponNotice repassa um aviso de cupom/campanha do Mercado Livre.
// Diferente do ramo de oferta, NÃO monta card de produto (o preview de uma
// página de cupom não tem produto): converte cada link ML para o afiliado do
// dono e reenvia o texto ORIGINAL sob um cabeçalho de cupom, via o mesmo relay
// batido do Shopee. Espelha a mecânica de relayWithConvertedShopeeLinks (ordena
// do link mais longo para o mais curto p/ o ReplaceAll não solar prefixo, e usa
// cópia rasa para o body convertido não vazar entre donos).
func (s *offerGenerateService) relayMLCouponNotice(ctx, sendRoot context.Context, auto *models.Automation, msg *models.IncomingMessage, mlLinks, shopeeLinks []string) error {
	body := msg.Body

	// Links Shopee que vieram junto (a oferta era do ML): converter para o
	// afiliado do DONO ou REMOVER do corpo. O corpo deste aviso é o texto
	// original preservado, então um link Shopee não convertido sairia cru e
	// creditaria a venda a quem postou no grupo-fonte. Remover é a única
	// alternativa segura a converter — nunca deixar passar.
	if len(shopeeLinks) > 0 && s.shopeeService != nil {
		ordered := append([]string(nil), shopeeLinks...)
		sort.SliceStable(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
		for _, orig := range ordered {
			aff, err := s.shopeeService.GenerateAffiliateLink(ctx, orig, auto.UserID)
			if err == nil && aff != nil && aff.AffiliateLink != "" {
				body = strings.ReplaceAll(body, orig, aff.AffiliateLink)
				continue
			}
			s.logger.Warn("offer_generate: link Shopee não convertido no aviso ML — removido do corpo",
				zap.String("automationID", auto.ID),
				zap.String("linkHash", commerceurl.Hash8(orig)),
				zap.Error(err))
			body = strings.ReplaceAll(body, orig, "")
		}
	}

	ordered := append([]string(nil), mlLinks...)
	sort.SliceStable(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })

	for _, orig := range ordered {
		aff, err := s.mlService.CreateAffiliateLink(ctx, orig, auto.UserID)
		if err != nil || aff == nil || aff.AffiliateLink == "" {
			// Sem conversão possível (URL sem item ID, falha transitória):
			// nunca reter — o link segue original. Reter só apagaria o aviso;
			// um shortlink de terceiro já passaria igual hoje no ramo de oferta.
			s.logger.Warn("ML coupon link not convertible — relaying original link",
				zap.String("automationID", auto.ID),
				zap.String("link", orig),
				zap.Error(err))
			continue
		}
		body = strings.ReplaceAll(body, orig, aff.AffiliateLink)
	}

	relayMsg := *msg
	relayMsg.Body = text.RenderMLCouponNotice(body)
	return s.processAutomationRelayLeased(ctx, sendRoot, auto, &relayMsg)
}

// processAutomationRelay envia a mensagem original pra cada grupo destino
// da automação. Resolve chips e dedup do mesmo jeito que processAutomation,
// só que não monta texto formatado — usa msg.Body + msg.MediaURL/MediaBase64
// e dispara o método WAHA apropriado por tipo.
// processAutomationRelay é a ENTRADA direta do relay (mensagem sem link de
// loja). Adquire a lease e delega — as chamadas internas usam a variante
// Leased para não pegar uma segunda.
func (s *offerGenerateService) processAutomationRelay(ctx context.Context, auto *models.Automation, msg *models.IncomingMessage) error {
	lease, fresh, err := s.acquireLease(ctx, auto)
	if err != nil {
		return err
	}
	if lease == nil {
		return nil
	}
	defer lease.Release()
	return s.processAutomationRelayLeased(lease.Ctx, lease.SendCtx, fresh, msg)
}

func (s *offerGenerateService) processAutomationRelayLeased(ctx, sendRoot context.Context, auto *models.Automation, msg *models.IncomingMessage) error {
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

	// Fase 2 — vídeo: normaliza (faststart) UMA vez antes do fan-out e reusa o
	// resultado, mandando convert:false em cada envio. Sem isso, o WAHA rodaria
	// ffmpeg por grupo (o mesmo vídeo N vezes) — pesado em automações com muitos
	// destinos, e o WAHA é o gargalo. Feito antes de qualquer goroutine de envio
	// (happens-before), então a mutação de msg.MediaBase64 é segura. Se o preparo
	// falhar, videoStreamable fica false e cada envio cai no convert do WAHA
	// (Fase 1) — a entrega nunca fica pior.
	videoStreamable := false
	if b64, ok := s.prepareStreamableVideo(ctx, msg); ok {
		msg.MediaBase64 = b64
		videoStreamable = true
		s.logger.Info("Relay video normalized once (faststart) — fan-out skips WAHA transcode",
			zap.String("automationID", auto.ID),
			zap.Int("destinations", len(destinationGroups)))
	}

	// Fan out in PARALLEL BY CHIP — same rationale as the offer flow: group the
	// destinations by their hosting chip so every chip relays at once instead
	// of one group at a time, with the per-group delay pacing each chip on its
	// own (anti-ban unchanged).
	chipGroups := make(map[string][]string)
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
		chipGroups[dispatchSession] = append(chipGroups[dispatchSession], groupID)
	}

	// Anti-ban: resolve pacing/humanize preset once; shared read-only across the
	// per-chip relay goroutines.
	resolved := config.GetHumanization().ForMode(auto.HumanizeMode)
	rt := config.GetAntiBanRuntime()
	baseDelay := 0
	if auto.DelaySeconds != nil && *auto.DelaySeconds > 0 {
		baseDelay = *auto.DelaySeconds
	}
	// Mesmo prazo do fan-out de oferta: relay de espelho em <1min em todos
	// os grupos, com o orçamento aplicado à fatia de cada chip.
	fanoutDeadline := resolved.Pacing.FanoutDeadline()
	// Digitação capada no espelho: até 12s/grupo do preset, serial por chip,
	// estourava sozinha o teto de 1 min com poucos chips conectados.
	mirrorHumanize := resolved.Humanize
	if mirrorHumanize.MaxTyping > 2500*time.Millisecond {
		mirrorHumanize.MaxTyping = 2500 * time.Millisecond
	}
	var wg sync.WaitGroup
	for dispatchSession, groupIDs := range chipGroups {
		wg.Add(1)
		go func(dispatchSession string, groupIDs []string) {
			defer wg.Done()
			// recover: panic numa goroutine NÃO é capturado pelo middleware do
			// asynq (que só cobre a goroutine do handler) — sem isto, um panic
			// aqui derrubava o worker inteiro (40 slots, todas as filas).
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("Panic in offer_generate chip fan-out",
						zap.String("session", dispatchSession),
						zap.Any("panic", r))
				}
			}()
			// Shuffle this chip's destination order (anti-ban).
			resolved.Pacing.Shuffle(groupIDs)
			for gi, groupID := range groupIDs {
				// Anti-ban: skip groups with a bad reputation (fail-open).
				if rt.Reputation.IsSuppressed(ctx, dispatchSession, groupID) {
					s.logDispatch(ctx, auto, groupID, "", "skipped", "reputacao (grupo problematico)", canonicalKey, "")
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

				// Anti-ban pacing: jittered per-chip delay (honors delay_seconds)
				// plus an occasional micro-break, keyed on the sending chip —
				// clamped so this chip's slice still fits the fan-out budget.
				// Timer cancelável, não time.Sleep: um sleep surdo acordaria para
				// enviar mesmo depois de a lease ter sido perdida no meio do fan-out.
				if !pacing.Sleep(sendRoot, pacing.BudgetedGap(
					resolved.Pacing.Step(baseDelay)+rt.Pacing.MicroBreak(ctx, dispatchSession, resolved.Pacing),
					fanoutDeadline, len(groupIDs)-gi)) {
					s.logger.Warn("offer_generate: dispatch lease lost mid fan-out, stopping",
						zap.String("automationID", auto.ID))
					return
				}

				// Marca o body+mídia como nosso próprio outgoing pra que o
				// webhook FromMe que retorna seja distinguido de um chip user
				// postando manualmente. Pula quando Body+MediaURL ambos vazios.
				ownKey := msg.Body
				if msg.MediaURL != "" {
					ownKey += "\x00" + msg.MediaURL
				}
				if ownKey != "" {
					s.markOwnOutgoing(ctx, dispatchSession, groupID, ownKey)
				}
				// Marca global pelo corpo repassado (o webhook do destino traz o
				// mesmo body): corta cadeia espelho → espelho no modo relay.
				selfsend.Mark(ctx, s.cache, groupID, msg.Body)

				// Raiz do envio: Background quando não há gate (o cancelamento do
				// consumidor de webhook nunca abortou um envio em andamento) e o
				// contexto da lease quando há — perder a lease PRECISA parar o fan-out.
				sendCtx, sendCancel := context.WithTimeout(sendRoot, 30*time.Second)
				// Anti-ban: typing/online/seen choreography before the relay send.
				// Text is the relayed body (used only to size the typing time — the
				// relay content itself is forwarded unchanged).
				stopTyping := rt.Humanizer.BeforeSend(sendCtx, humanize.Plan{
					Session:  dispatchSession,
					ChatID:   groupID,
					Text:     msg.Body,
					HasMedia: msg.MediaURL != "",
					Cfg:      mirrorHumanize,
				})
				sendErr := s.sendRelayMessage(sendCtx, dispatchSession, groupID, msg, videoStreamable)
				stopTyping()
				sendCancel()

				if sendErr != nil {
					// Mesma regra do fan-out de oferta: timeout = entrega incerta,
					// slot mantido para o relay não sair duplicado no grupo.
					if whatsapp.IsAmbiguousOutcome(sendErr) {
						s.logDispatch(ctx, auto, groupID, "", "failed",
							"timeout no envio (entrega incerta) - slot de dedup mantido", canonicalKey, "")
						s.logger.Warn("Relay send timed out — delivery uncertain, keeping dedup slot",
							zap.String("groupID", groupID),
							zap.String("session", dispatchSession),
							zap.Error(sendErr))
						continue
					}
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
		}(dispatchSession, groupIDs)
	}
	wg.Wait()

	return nil
}

// prepareStreamableVideo baixa/decodifica o vídeo do relay UMA vez e o remuxa
// para faststart, devolvendo o base64 pronto para reuso no fan-out. Retorna
// ok=false quando não é vídeo, o ffmpeg não está disponível, não há bytes ou a
// normalização falha — nesses casos o envio cai no convert do WAHA (Fase 1),
// então a entrega nunca fica pior. Chamado antes do fan-out (1x por mensagem).
func (s *offerGenerateService) prepareStreamableVideo(ctx context.Context, msg *models.IncomingMessage) (string, bool) {
	if strings.ToLower(msg.MediaType) != "video" || !videostream.Available() {
		return "", false
	}

	// Obtém os bytes originais uma única vez: base64 inline se veio no webhook,
	// senão baixa do store do WAHA (mesma origem que o sendRelayMessage usaria).
	var raw []byte
	if msg.MediaBase64 != "" {
		if b, err := base64.StdEncoding.DecodeString(msg.MediaBase64); err == nil {
			raw = b
		}
	}
	if len(raw) == 0 && msg.MediaURL != "" {
		if res, err := s.waProvider.DownloadMediaByURL(ctx, msg.MediaURL); err == nil && res != nil {
			raw = res.Data
		}
	}
	if len(raw) == 0 {
		return "", false
	}

	// Remux é rápido, mas limita o tempo pra não prender o worker num vídeo
	// patológico.
	fctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	out, err := videostream.EnsureFaststart(fctx, raw)
	if err != nil {
		s.logger.Warn("Relay video: faststart remux failed, falling back to WAHA convert",
			zap.String("group", msg.RemoteJID),
			zap.Error(err))
		return "", false
	}
	return base64.StdEncoding.EncodeToString(out), true
}

// sendRelayMessage dispara o método WAHA correto baseado no tipo da
// mensagem original. Pra mídia, prefere MediaURL (mais leve, sem reupload);
// cai pra MediaBase64 quando URL não está disponível (caso comum no
// link preview JPEG do WhatsApp). Áudio só funciona via Base64 — o input
// SendVoiceInput não tem campo URL.
func (s *offerGenerateService) sendRelayMessage(ctx context.Context, session, chatID string, msg *models.IncomingMessage, videoStreamable bool) error {
	// @todos no relay: se a mensagem de origem traz "@todos", pinga todos os
	// membros do grupo de DESTINO (chatID) via mentionedJid. Opt-in por
	// mensagem (anti-ban). O relay nunca usa card, então texto/imagem/vídeo/
	// documento honram a menção diretamente. Calculado uma vez.
	var mentions []string
	if wamentions.Contains(msg.Body) {
		mentions = wamentions.Expand(ctx, s.waProvider, session, chatID, msg.Body)
	}

	// Emoji colado no fim da URL faz o WhatsApp engolir o emoji no trecho
	// clicável e o link não abre. O relay repassa o corpo da fonte verbatim,
	// então não passa pelo NormalizeOutgoingWhatsAppText (que reescreveria a
	// formatação do canal); aplicamos só a separação da fronteira URL→emoji.
	body := text.SeparateURLEmojiGlue(msg.Body)

	// Sem mídia → texto puro
	if !msg.HasMedia {
		if strings.TrimSpace(body) == "" {
			// Mensagem sem texto E sem mídia — provavelmente um evento
			// sistema (reaction, edit, etc) que o WAHA encaminhou. Skipa.
			return nil
		}
		_, err := s.waProvider.SendText(ctx, whatsapp.SendTextInput{
			SessionName: session,
			ChatID:      chatID,
			Text:        body,
			Mentions:    mentions,
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

	// Sem os bytes NÃO adianta passar msg.MediaURL adiante: é URL interna do
	// WAHA (localhost:3000, autenticada), então o fileToMedia do lado dele leva
	// 401 → 500 e a mensagem some INTEIRA — nem imagem nem texto. Degradar para
	// texto entrega ao menos a oferta com os links já convertidos.
	sendWithoutMedia := func() error {
		s.logger.Warn("Relay: sem bytes de mídia, degradando para texto",
			zap.String("mediaType", msg.MediaType),
			zap.String("mimeType", msg.MimeType))
		if strings.TrimSpace(body) == "" {
			return fmt.Errorf("relay %s sem bytes de mídia e sem texto", msg.MediaType)
		}
		_, err := s.waProvider.SendText(ctx, whatsapp.SendTextInput{
			SessionName: session,
			ChatID:      chatID,
			Text:        body,
			Mentions:    mentions,
		})
		return err
	}

	switch strings.ToLower(msg.MediaType) {
	case "image":
		if mediaB64 == "" {
			return sendWithoutMedia()
		}
		_, err := s.waProvider.SendImage(ctx, whatsapp.SendImageInput{
			SessionName: session,
			ChatID:      chatID,
			Caption:     body,
			Mimetype:    mediaMime,
			Mentions:    mentions,
			ImageBase64: mediaB64,
		})
		return err

	case "video":
		if mediaB64 == "" {
			return sendWithoutMedia()
		}
		_, err := s.waProvider.SendVideo(ctx, whatsapp.SendVideoInput{
			SessionName: session,
			ChatID:      chatID,
			Caption:     body,
			Mentions:    mentions,
			// videoStreamable=true quando o relay já normalizou o vídeo
			// (faststart) 1x antes do fan-out — evita o WAHA transcodificar
			// por grupo. Quando false, o WAHA converte (Fase 1).
			AlreadyStreamable: videoStreamable,
			MediaBase64:       mediaB64,
		})
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
		if mediaB64 == "" {
			return sendWithoutMedia()
		}
		_, err := s.waProvider.SendDocument(ctx, whatsapp.SendDocumentInput{
			SessionName: session,
			ChatID:      chatID,
			Caption:     body,
			Mentions:    mentions,
			MediaBase64: mediaB64,
		})
		return err

	default:
		// Mídia com tipo não resolvido (mediaType e mime ausentes/desconhecidos).
		// Defesa em profundidade: se temos os BYTES, tenta enviar como imagem
		// — de longe o caso mais comum no relay — em vez de degradar pra texto
		// puro e perder a mídia silenciosamente. Se falhar, o erro propaga e o
		// dispatch fica logado como "failed".
		if msg.HasMedia && mediaB64 != "" {
			s.logger.Warn("Relay media with unresolved type — sending as image (fallback)",
				zap.String("mediaType", msg.MediaType),
				zap.String("mimeType", msg.MimeType))
			_, err := s.waProvider.SendImage(ctx, whatsapp.SendImageInput{
				SessionName: session,
				ChatID:      chatID,
				Caption:     body,
				Mimetype:    mediaMime,
				Mentions:    mentions,
				ImageBase64: mediaB64,
			})
			return err
		}
		// Sticker/location/contact sem método dedicado: fallback pra texto se
		// tiver Body, senão skipa com warning.
		if strings.TrimSpace(body) != "" {
			_, err := s.waProvider.SendText(ctx, whatsapp.SendTextInput{
				SessionName: session,
				ChatID:      chatID,
				Text:        body,
				Mentions:    mentions,
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
func (s *offerGenerateService) buildRelayContentKey(scope string, msg *models.IncomingMessage) string {
	h := sha256.Sum256([]byte(msg.Body + "\x00" + msg.MediaURL + "\x00" + msg.MimeType))
	return fmt.Sprintf("og_relay_content:%s:%s:%s", scope, msg.RemoteJID, hex.EncodeToString(h[:8]))
}

// firstProductStore devolve a loja do primeiro link que, SÓ pela classificação
// local (host + path, sem nenhuma chamada de rede), já é uma página de produto.
//
// Devolve "" quando nenhum link prova ser produto localmente — caso em que o
// chamador mantém a ordem histórica, porque não há evidência para desempatar.
func firstProductStore(links []string) string {
	for _, raw := range links {
		u, err := commerceurl.Parse(raw)
		if err != nil || u.Store == "" {
			continue
		}
		if commerceurl.ClassifyLocal(u).Class == models.URLClassProduct {
			return u.Store
		}
	}
	return ""
}
