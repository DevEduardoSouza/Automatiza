package offer_generate //nolint:revive // nome do pacote já existe com underscore; renomear está fora de escopo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/shopee"

	"go.uber.org/zap"
)

// Melhoria 6 — CTA estritamente opt-in por {cta} no offer_generate.
//
// Dois oráculos por caso: o TEXTO enviado (a CTA aparece/não aparece, e quantas
// vezes) e os CONTADORES do repositório (a consulta aconteceu/não aconteceu).
// O segundo é o que prova o ganho: um texto sem {cta} não pode custar query.

// ctaCountingRepo embute a interface e conta as duas leituras do dispatch.
type ctaCountingRepo struct {
	repository.CTARepository
	pool      []models.CTA
	err       error
	userCalls int
	idCalls   int
}

func (c *ctaCountingRepo) GetUserCTAs(_ context.Context, _ string, _ map[string]interface{}) ([]models.CTA, error) {
	c.userCalls++
	if c.err != nil {
		return nil, c.err
	}
	return c.pool, nil
}

func (c *ctaCountingRepo) GetCTAsByIDs(_ context.Context, _ []string, _ string) ([]models.CTA, error) {
	c.idCalls++
	if c.err != nil {
		return nil, c.err
	}
	return c.pool, nil
}

func (c *ctaCountingRepo) assertCalls(t *testing.T, wantUser, wantID int) {
	t.Helper()
	if c.userCalls != wantUser {
		t.Errorf("GetUserCTAs chamado %d vez(es), want %d", c.userCalls, wantUser)
	}
	if c.idCalls != wantID {
		t.Errorf("GetCTAsByIDs chamado %d vez(es), want %d", c.idCalls, wantID)
	}
}

func ctaOptInPool() []models.CTA {
	return []models.CTA{
		{Message: "CTA_POOL_SHOPEE", Platform: models.CTAPlatformShopee, IsActive: true},
		{Message: "CTA_POOL_ML", Platform: models.CTAPlatformMercadoLivre, IsActive: true},
	}
}

// --- fake Shopee que resolve PRODUTO (o caminho que renderiza oferta) -------

type ctaFakeShopeeProduct struct {
	shopee.Service
}

func (ctaFakeShopeeProduct) ExpandURL(_ context.Context, url string) (string, error) { return url, nil }

func (ctaFakeShopeeProduct) ExtractProductIDs(_ string) (int64, int64, error) { return 456, 123, nil }

func (ctaFakeShopeeProduct) GetProduct(_ context.Context, _, _ int64, _ string) (*models.ShopeeOffer, error) {
	return &models.ShopeeOffer{Name: "Produto OG", Price: 50.0, OriginalPrice: 100.0, Discount: 50}, nil
}

func (ctaFakeShopeeProduct) GenerateAffiliateLink(_ context.Context, _, _ string) (*models.ShopeeAffiliateLink, error) {
	return &models.ShopeeAffiliateLink{AffiliateLink: ctaAffLink, ShortLink: ctaAffLink}, nil
}

// fake Shopee de link NÃO-produto (cupom/campanha): exercita o relay.
type ctaFakeShopeeCoupon struct {
	shopee.Service
}

func (ctaFakeShopeeCoupon) ExpandURL(_ context.Context, _ string) (string, error) {
	return couponExpandedNP, nil
}

func (ctaFakeShopeeCoupon) ExtractProductIDs(_ string) (int64, int64, error) {
	return 0, 0, errors.New("could not extract product IDs from URL")
}

func (ctaFakeShopeeCoupon) GenerateAffiliateLink(_ context.Context, _, _ string) (*models.ShopeeAffiliateLink, error) {
	return &models.ShopeeAffiliateLink{AffiliateLink: ctaAffLink, ShortLink: ctaAffLink}, nil
}

const (
	ctaAffLink     = "https://s.shopee.com.br/AFFdoDono"
	ctaProductLink = "https://shopee.com.br/produto-i.123.456"
)

// ctaSettingsRepo devolve o corpo de dispatch text pedido pelo teste.
type ctaSettingsRepo struct {
	repository.SettingsRepository
	body   string
	copies []string
}

func (r ctaSettingsRepo) GetActiveDispatchTextByID(_ context.Context, _, _ string) (*models.DispatchText, error) {
	return &models.DispatchText{ID: "dt-1", Body: r.body, Copies: r.copies, IsActive: true}, nil
}

// captureSent instala um adapter que registra o texto de TODOS os caminhos de
// envio (texto puro, imagem com legenda e card de preview) — assim o oráculo
// não depende de qual ramo da cascata o produto escolheu.
func captureSent(sent *[]string) *whatsapp.FakeAdapter {
	fake := whatsapp.NewFakeAdapter()
	fake.SendTextFunc = func(_ context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		*sent = append(*sent, in.Text)
		return &whatsapp.MessageResult{ID: "msg-1"}, nil
	}
	fake.SendImageFunc = func(_ context.Context, in whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
		*sent = append(*sent, in.Caption)
		return &whatsapp.MessageResult{ID: "msg-2"}, nil
	}
	fake.SendLinkCustomPreviewFunc = func(_ context.Context, in whatsapp.SendLinkPreviewInput) (*whatsapp.MessageResult, error) {
		*sent = append(*sent, in.Text)
		return &whatsapp.MessageResult{ID: "msg-3"}, nil
	}
	return fake
}

func ctaOptInAutomation(ctaIDs, inline []string, stores []string) *models.Automation {
	dtID := "dt-1"
	zero := 0
	sendAsImageOff := false
	return &models.Automation{
		ID:                       "auto-cta",
		UserID:                   couponOwnerID,
		Name:                     "Gerar Oferta CTA",
		Status:                   models.AutomationStatusActive,
		DispatchTextID:           &dtID,
		SendGroups:               []string{couponDestGroup},
		DelaySeconds:             &zero,
		Stores:                   stores,
		CTAIDs:                   ctaIDs,
		CTAs:                     inline,
		OfferGenerateSendAsImage: &sendAsImageOff,
	}
}

// runShopeeOffer roda o caminho de OFERTA Shopee (produto) e devolve os textos
// enviados.
func runShopeeOffer(t *testing.T, repo repository.CTARepository, body, msgBody string, ctaIDs, inline []string) []string {
	t.Helper()
	var sent []string
	svc := &offerGenerateService{
		logger:        zap.NewNop(),
		waProvider:    captureSent(&sent),
		instanceRepo:  couponFakeInstanceRepo{},
		settingsRepo:  ctaSettingsRepo{body: body},
		shopeeService: ctaFakeShopeeProduct{},
		ctaRepo:       repo,
	}
	msg := &models.IncomingMessage{
		InstanceName: "session_test",
		MessageID:    "msg-cta-1",
		RemoteJID:    couponMonitorGroup,
		Body:         msgBody,
	}
	auto := ctaOptInAutomation(ctaIDs, inline, []string{"shopee"})
	if err := svc.processAutomation(context.Background(), auto, msg, []string{ctaProductLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) == 0 {
		t.Fatal("nenhum envio capturado — a oferta não chegou ao destino")
	}
	return sent
}

// zwStripper remove os zero-width que o text.Vary injeta no envio (anti-ban)
// — sem isso "R$ 40,00" nunca casaria com o texto real. É saneamento do
// ORÁCULO, não da produção: a humanização é comportamento pré-existente e
// deliberado, alheio a esta melhoria.
var zwStripper = strings.NewReplacer("\u200b", "", "\u200c", "", "\ufeff", "")

func countIn(texts []string, marker string) int {
	total := 0
	for _, s := range texts {
		total += strings.Count(zwStripper.Replace(s), marker)
	}
	return total
}

// Sem {cta} no texto de disparo: nenhuma consulta e nenhum texto de CTA —
// mesmo com pool cadastrado, CTAIDs e inline configurados.
func TestOfferGenerate_CTAOptIn_NoVariableNoQuery(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	sent := runShopeeOffer(t, repo, "{nome_produto} por {preco_formatado}\n{link_afiliado}",
		"Olha essa oferta "+ctaProductLink, []string{"cta-1"}, []string{"CTA_INLINE"})

	if n := countIn(sent, "CTA_"); n != 0 {
		t.Errorf("ocorrências de CTA = %d, want 0.\n got: %q", n, sent)
	}
	repo.assertCalls(t, 0, 0)
}

// Com {cta}: uma consulta ao pool e a CTA da plataforma certa, exatamente 1×.
func TestOfferGenerate_CTAOptIn_PoolQueriedOnce(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	sent := runShopeeOffer(t, repo, "{nome_produto}\n{link_afiliado}\n{cta}",
		"Olha essa oferta "+ctaProductLink, nil, nil)

	if n := countIn(sent, "CTA_POOL_SHOPEE"); n != 1 {
		t.Errorf("ocorrências do CTA Shopee = %d, want 1.\n got: %q", n, sent)
	}
	if countIn(sent, "CTA_POOL_ML") != 0 {
		t.Errorf("CTA de ML não pode sair num link Shopee.\n got: %q", sent)
	}
	repo.assertCalls(t, 1, 0)
}

// CTAIDs preenchido + {cta}: só o ramo por IDs.
func TestOfferGenerate_CTAOptIn_IDsQueriedOnce(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	sent := runShopeeOffer(t, repo, "{nome_produto}\n{link_afiliado}\n{cta}",
		"Olha essa oferta "+ctaProductLink, []string{"cta-1"}, nil)

	if n := countIn(sent, "CTA_POOL_SHOPEE"); n != 1 {
		t.Errorf("ocorrências do CTA = %d, want 1.\n got: %q", n, sent)
	}
	repo.assertCalls(t, 0, 1)
}

// Duas ocorrências autoradas: o MESMO valor duas vezes, nunca três.
func TestOfferGenerate_CTAOptIn_TwoPlaceholders(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	sent := runShopeeOffer(t, repo, "{cta}\n{nome_produto}\n{link_afiliado}\n{{cta}}",
		"Olha essa oferta "+ctaProductLink, nil, nil)

	if n := countIn(sent, "CTA_POOL_SHOPEE"); n != 2 {
		t.Errorf("ocorrências do CTA = %d, want 2.\n got: %q", n, sent)
	}
	repo.assertCalls(t, 1, 0)
}

// Override cta[...] com {cta} no template: vence o pool E dispensa a consulta,
// porque o resultado já está decidido. A tag não pode vazar no texto enviado.
func TestOfferGenerate_CTAOptIn_OverrideWinsWithoutQuery(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	sent := runShopeeOffer(t, repo, "{nome_produto}\n{link_afiliado}\n{cta}",
		"cta[CTA_OVERRIDE] Olha essa oferta "+ctaProductLink, nil, []string{"CTA_INLINE"})

	if n := countIn(sent, "CTA_OVERRIDE"); n != 1 {
		t.Errorf("ocorrências do override = %d, want 1.\n got: %q", n, sent)
	}
	if countIn(sent, "CTA_POOL_SHOPEE") != 0 || countIn(sent, "CTA_INLINE") != 0 {
		t.Errorf("override deveria vencer pool e inline.\n got: %q", sent)
	}
	if countIn(sent, "cta[") != 0 {
		t.Errorf("a tag cta[...] não pode aparecer no disparo.\n got: %q", sent)
	}
	repo.assertCalls(t, 0, 0)
}

// Override cta[...] SEM {cta} no template: o valor não é usado nem parseado
// para uso, e o pool continua intocado.
func TestOfferGenerate_CTAOptIn_OverrideIgnoredWithoutVariable(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	sent := runShopeeOffer(t, repo, "{nome_produto}\n{link_afiliado}",
		"cta[CTA_OVERRIDE] Olha essa oferta "+ctaProductLink, nil, []string{"CTA_INLINE"})

	if countIn(sent, "CTA_OVERRIDE") != 0 {
		t.Errorf("sem {cta} o override não entra.\n got: %q", sent)
	}
	if countIn(sent, "cta[") != 0 {
		t.Errorf("a tag cta[...] não pode aparecer no disparo.\n got: %q", sent)
	}
	repo.assertCalls(t, 0, 0)
}

// Sem override e com pool sem CTA elegível: cai no inline, uma consulta só.
func TestOfferGenerate_CTAOptIn_FallsBackToInline(t *testing.T) {
	repo := &ctaCountingRepo{pool: []models.CTA{
		{Message: "CTA_POOL_ML", Platform: models.CTAPlatformMercadoLivre, IsActive: true},
	}}
	sent := runShopeeOffer(t, repo, "{nome_produto}\n{link_afiliado}\n{cta}",
		"Olha essa oferta "+ctaProductLink, nil, []string{"CTA_INLINE"})

	if n := countIn(sent, "CTA_INLINE"); n != 1 {
		t.Errorf("ocorrências do inline = %d, want 1.\n got: %q", n, sent)
	}
	repo.assertCalls(t, 1, 0)
}

// Repositório em erro: fail-open (o envio acontece com o inline) e uma
// tentativa só.
func TestOfferGenerate_CTAOptIn_RepoErrorIsFailOpen(t *testing.T) {
	repo := &ctaCountingRepo{err: errors.New("db down")}
	sent := runShopeeOffer(t, repo, "{nome_produto}\n{link_afiliado}\n{cta}",
		"Olha essa oferta "+ctaProductLink, nil, []string{"CTA_INLINE"})

	if n := countIn(sent, "CTA_INLINE"); n != 1 {
		t.Errorf("erro de CTA deveria cair no inline; ocorrências = %d.\n got: %q", n, sent)
	}
	repo.assertCalls(t, 1, 0)
}

// Cópia sorteada decide o gate: com uma única cópia SEM {cta}, zero consulta,
// mesmo que o Body base tenha a variável.
func TestOfferGenerate_CTAOptIn_SelectedCopyDecides(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	var sent []string
	svc := &offerGenerateService{
		logger:       zap.NewNop(),
		waProvider:   captureSent(&sent),
		instanceRepo: couponFakeInstanceRepo{},
		settingsRepo: ctaSettingsRepo{
			body:   "{nome_produto}\n{link_afiliado}\n{cta}",
			copies: []string{"{nome_produto}\n{link_afiliado}"},
		},
		shopeeService: ctaFakeShopeeProduct{},
		ctaRepo:       repo,
	}
	msg := &models.IncomingMessage{
		InstanceName: "session_test", MessageID: "msg-cta-copy",
		RemoteJID: couponMonitorGroup, Body: "Olha essa oferta " + ctaProductLink,
	}
	auto := ctaOptInAutomation(nil, []string{"CTA_INLINE"}, []string{"shopee"})
	if err := svc.processAutomation(context.Background(), auto, msg, []string{ctaProductLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if countIn(sent, "CTA_") != 0 {
		t.Errorf("a cópia sorteada não tem {cta} — nenhuma CTA pode sair.\n got: %q", sent)
	}
	repo.assertCalls(t, 0, 0)
}

// Relay de cupom/campanha Shopee (link não-produto): não existe renderer de
// oferta, então nem com {cta} no texto de disparo pode haver consulta.
func TestOfferGenerate_CTAOptIn_ShopeeCouponRelayNeverQueries(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	var sent []string
	svc := &offerGenerateService{
		logger:        zap.NewNop(),
		waProvider:    captureSent(&sent),
		instanceRepo:  couponFakeInstanceRepo{},
		settingsRepo:  ctaSettingsRepo{body: "{nome_produto}\n{link_afiliado}\n{cta}"},
		shopeeService: ctaFakeShopeeCoupon{},
		ctaRepo:       repo,
	}
	msg := &models.IncomingMessage{
		InstanceName: "session_test", MessageID: "msg-cta-coupon",
		RemoteJID: couponMonitorGroup, Body: couponBody,
	}
	auto := ctaOptInAutomation(nil, []string{"CTA_INLINE"}, []string{"shopee"})
	if err := svc.processAutomation(context.Background(), auto, msg, []string{couponSourceLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) == 0 {
		t.Fatal("o cupom deveria ter sido repassado")
	}
	if countIn(sent, "CTA_") != 0 {
		t.Errorf("relay de cupom não usa CTA.\n got: %q", sent)
	}
	repo.assertCalls(t, 0, 0)
}

// Mensagem sem link de loja: relay puro, sem renderer de oferta e sem CTA.
func TestOfferGenerate_CTAOptIn_PlainRelayNeverQueries(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	var sent []string
	svc := &offerGenerateService{
		logger:        zap.NewNop(),
		waProvider:    captureSent(&sent),
		instanceRepo:  couponFakeInstanceRepo{},
		settingsRepo:  ctaSettingsRepo{body: "{nome_produto}\n{link_afiliado}\n{cta}"},
		shopeeService: ctaFakeShopeeProduct{},
		ctaRepo:       repo,
	}
	msg := &models.IncomingMessage{
		InstanceName: "session_test", MessageID: "msg-cta-plain",
		RemoteJID: couponMonitorGroup, Body: "Aviso do grupo https://instagram.com/perfil",
	}
	auto := ctaOptInAutomation(nil, []string{"CTA_INLINE"}, []string{"shopee"})
	if err := svc.processAutomation(context.Background(), auto, msg,
		[]string{"https://instagram.com/perfil"}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if countIn(sent, "CTA_") != 0 {
		t.Errorf("relay puro não usa CTA.\n got: %q", sent)
	}
	repo.assertCalls(t, 0, 0)
}

// Tags mistas: desc[...], info[...] e cta[...] na mesma mensagem. O gate do CTA
// não pode alterar preço nem {info}.
func TestOfferGenerate_CTAOptIn_MixedTags(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	sent := runShopeeOffer(t, repo,
		"{nome_produto} por {preco_formatado}\n{info}\n{link_afiliado}\n{cta}",
		"desc[10] info[FRETE GRATIS] cta[CTA_OVERRIDE] "+ctaProductLink, nil, nil)

	if countIn(sent, "FRETE GRATIS") != 1 {
		t.Errorf("{info} deveria continuar sendo preenchido.\n got: %q", sent)
	}
	if n := countIn(sent, "CTA_OVERRIDE"); n != 1 {
		t.Errorf("ocorrências do override = %d, want 1.\n got: %q", n, sent)
	}
	// desc[10] sem '%' é valor FIXO: 50,00 - 10 = 40,00 (ver ApplyDiscountTag).
	if countIn(sent, "R$ 40,00") != 1 {
		t.Errorf("a tag desc[10] deveria continuar valendo.\n got: %q", sent)
	}
	repo.assertCalls(t, 0, 0)
}

// Sem DispatchTextID a automação cai no template default do sistema, que NÃO
// tem {cta} — logo, zero consulta.
func TestOfferGenerate_CTAOptIn_SystemDefaultTemplateNeverQueries(t *testing.T) {
	repo := &ctaCountingRepo{pool: ctaOptInPool()}
	var sent []string
	svc := &offerGenerateService{
		logger:        zap.NewNop(),
		waProvider:    captureSent(&sent),
		instanceRepo:  couponFakeInstanceRepo{},
		shopeeService: ctaFakeShopeeProduct{},
		ctaRepo:       repo,
	}
	auto := ctaOptInAutomation(nil, []string{"CTA_INLINE"}, []string{"shopee"})
	auto.DispatchTextID = nil
	msg := &models.IncomingMessage{
		InstanceName: "session_test", MessageID: "msg-cta-default",
		RemoteJID: couponMonitorGroup, Body: "Olha essa oferta " + ctaProductLink,
	}
	if err := svc.processAutomation(context.Background(), auto, msg, []string{ctaProductLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if countIn(sent, "CTA_") != 0 {
		t.Errorf("o template default não tem {cta}.\n got: %q", sent)
	}
	repo.assertCalls(t, 0, 0)
}
