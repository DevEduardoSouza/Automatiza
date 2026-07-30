package monitoroffer

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"automatiza-go/internal/models"
)

// fakeResolver decide pelo host+path, sem rede. Conta chamadas para provar que
// duplicata NÃO gera chamada externa nova.
type fakeResolver struct {
	calls int32
	fn    func(ResolveRequest) ResolveResult
}

func (f *fakeResolver) Resolve(_ context.Context, in ResolveRequest) ResolveResult {
	atomic.AddInt32(&f.calls, 1)
	return f.fn(in)
}

// produto devolve um veredito de produto convertido para o link do tenant.
func produto(key, converted string) ResolveResult {
	return ResolveResult{
		Class: models.URLClassProduct, ErrorClass: models.ErrorClassNone,
		Reason: models.ReasonNone, CanonicalKey: key, ConvertedURL: converted,
		Outcome: models.MonitorOutcomeConverted,
		Offer:   &OfferData{Title: "Produto", Price: 10},
	}
}

func campanha(converted string) ResolveResult {
	return ResolveResult{
		Class: models.URLClassCampaignOrCoupon, ErrorClass: models.ErrorClassNone,
		Reason: models.ReasonNone, ConvertedURL: converted,
		Outcome: models.MonitorOutcomeConverted,
	}
}

func transitorio(reason string) ResolveResult {
	return ResolveResult{
		Class: models.URLClassUnknown, ErrorClass: models.ErrorClassTransient, Reason: reason,
	}
}

func plan(t *testing.T, body string, r LinkResolver, opts ...func(*PlanInput)) Plan {
	t.Helper()
	in := PlanInput{Body: body, UserID: "user-1", MirrorCommon: true}
	for _, o := range opts {
		o(&in)
	}
	slots, blocked := ClassifyBody(body)
	if blocked != "" {
		return Plan{BlockedReason: blocked}
	}
	return PlanMessage(context.Background(), in, slots, r)
}

// ---------------------------------------------------------------------------
// Cardinalidade — o que o cliente vê
// ---------------------------------------------------------------------------

// TestCupomShopeeNaoPreemptaProdutoML é o bug que a melhoria existe para
// corrigir: o código antigo escolhia shopeeLinks[0] antes de olhar ML, então um
// cupom Shopee citado antes roubava a oferta do produto ML.
func TestCupomShopeeNaoPreemptaProdutoML(t *testing.T) {
	r := &fakeResolver{fn: func(in ResolveRequest) ResolveResult {
		if in.URL.Store == models.MonitorStoreShopee {
			return campanha("https://shope.ee/CUPOM-DO-DONO")
		}
		return produto("ml:MLB123", "https://mercadolivre.com/sec/DONO")
	}}

	p := plan(t, "Cupom aqui https://shopee.com.br/m/cupom-do-dia\n\nProduto https://www.mercadolivre.com.br/p/MLB123", r)

	if p.Offer == nil {
		t.Fatal("a oferta ML deveria existir")
	}
	if p.Offer.Key != "product:ml:MLB123" {
		t.Errorf("oferta = %q, queria o produto ML", p.Offer.Key)
	}
	if p.Campaign == nil {
		t.Fatal("o aviso de campanha Shopee deveria existir")
	}
	outs := p.Outputs()
	if outs[0].Kind != models.OutputKindProductOffer {
		t.Errorf("primeira saída = %q, a oferta tem de vir primeiro", outs[0].Kind)
	}
}

// TestUnknownAnteriorNaoBloqueiaProdutoPosterior: um link que só o retry
// resolve não pode impedir que o produto conhecido saia agora.
func TestUnknownAnteriorNaoBloqueiaProdutoPosterior(t *testing.T) {
	r := &fakeResolver{fn: func(in ResolveRequest) ResolveResult {
		if strings.Contains(in.URL.Raw, "s.shopee") {
			return transitorio(models.ReasonTimeout)
		}
		return produto("ml:MLB9", "https://mercadolivre.com/sec/DONO")
	}}

	p := plan(t, "https://s.shopee.com.br/abc\n\nhttps://www.mercadolivre.com.br/p/MLB9", r)

	if p.Offer == nil {
		t.Fatal("o produto conhecido tinha de sair mesmo com unknown antes")
	}
	var pendentes int
	for _, it := range p.Items {
		if it.Status == models.MonitorItemPending {
			pendentes++
		}
	}
	if pendentes != 1 {
		t.Errorf("%d itens pending, queria 1 (o link transitório)", pendentes)
	}
}

func TestDoisProdutosSoOPrimeiroSai(t *testing.T) {
	n := 0
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult {
		n++
		if n == 1 {
			return produto("shopee:1:1", "https://shope.ee/A")
		}
		return produto("shopee:2:2", "https://shope.ee/B")
	}}

	p := plan(t, "https://shopee.com.br/product/1/1 e https://shopee.com.br/product/2/2", r)

	if p.Offer == nil {
		t.Fatal("faltou oferta")
	}
	var excedentes int
	for _, it := range p.Items {
		if it.Status == models.MonitorItemSuperseded && it.Reason == models.ReasonExcessProduct {
			excedentes++
		}
	}
	if excedentes != 1 {
		t.Errorf("%d superseded/excess_product, queria 1", excedentes)
	}
}

// TestMesmoProdutoDuasVezesNaoChamaDuasConversoes prova que a dedup por
// canonical key evita chamada externa repetida — é cota da Shopee em jogo.
func TestMesmoProdutoDuasVezesNaoDuplicaSaida(t *testing.T) {
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult {
		return produto("shopee:1:1", "https://shope.ee/MESMO")
	}}

	p := plan(t, "https://shopee.com.br/product/1/1\n\nde novo https://shopee.com.br/slug/1/1", r)

	if p.Offer == nil {
		t.Fatal("faltou oferta")
	}
	var dups int
	for _, it := range p.Items {
		if it.Status == models.MonitorItemSuperseded && it.Reason == models.ReasonDuplicateLink {
			dups++
		}
	}
	if dups != 1 {
		t.Errorf("%d superseded/duplicate_link, queria 1", dups)
	}
	if n := len(p.Outputs()); n != 1 {
		t.Errorf("%d saídas, queria 1 (uma oferta só)", n)
	}
}

// ---------------------------------------------------------------------------
// Ausência de dado nunca vira campanha
// ---------------------------------------------------------------------------

func TestTransitorioNuncaViraCampanha(t *testing.T) {
	for _, reason := range []string{
		models.ReasonTimeout, models.ReasonHTTP429, models.ReasonHTTP5xx,
		models.ReasonPreviewIncomplete, models.ReasonResponseTooLarge,
	} {
		r := &fakeResolver{fn: func(ResolveRequest) ResolveResult { return transitorio(reason) }}
		p := plan(t, "https://www.mercadolivre.com.br/social/canal", r)

		if p.Campaign != nil {
			t.Errorf("%s gerou aviso de campanha — é exatamente o cupom falso", reason)
		}
		if p.Offer != nil {
			t.Errorf("%s gerou oferta", reason)
		}
		if len(p.Items) != 1 || p.Items[0].Status != models.MonitorItemPending {
			t.Errorf("%s: item não ficou pending para retry", reason)
		}
	}
}

// TestConvertidoVazioNuncaEmiteOriginal: se o resolvedor diz "produto" mas não
// entrega link do tenant, nada sai. Emitir a origem repassaria a comissão.
func TestConvertidoVazioNuncaEmiteOriginal(t *testing.T) {
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult {
		return ResolveResult{Class: models.URLClassProduct, ErrorClass: models.ErrorClassNone,
			Reason: models.ReasonNone, CanonicalKey: "shopee:1:1"}
	}}
	p := plan(t, "https://shopee.com.br/product/1/1", r)

	if p.Offer != nil {
		t.Fatal("sem link do tenant não pode haver oferta")
	}
	if p.Items[0].Status != models.MonitorItemPending {
		t.Errorf("status = %q, queria pending", p.Items[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Anti-vazamento no corpo público
// ---------------------------------------------------------------------------

// TestNenhumCorpoContemURLDeOrigem varre TODAS as saídas: nenhuma pode conter a
// URL original nem o tracking da fonte.
func TestNenhumCorpoContemURLDeOrigem(t *testing.T) {
	const origem = "https://shopee.com.br/m/cupom?utm_source=an_FONTE"
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult {
		return campanha("https://shope.ee/DO-DONO")
	}}

	p := plan(t, "Corre! "+origem+"\n\nAproveita", r)

	for _, o := range p.Outputs() {
		if strings.Contains(o.Body, "utm_source=an_FONTE") {
			t.Errorf("saída %s vazou o tracking da fonte:\n%s", o.Kind, o.Body)
		}
		if strings.Contains(o.Body, "shopee.com.br/m/cupom") {
			t.Errorf("saída %s vazou a URL de origem:\n%s", o.Kind, o.Body)
		}
	}
}

func TestLojaSemConversorSaiDoCorpoMasOEntornoFica(t *testing.T) {
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult {
		t.Error("loja sem conversor não pode provocar I/O")
		return ResolveResult{}
	}}

	// A Shein é a última loja sem conversor: o link de afiliado dela é um
	// onelink opaco, sem ID de produto para reatribuir. Amazon/AliExpress/Awin
	// e Magalu saíram desta classe quando ganharam conversor próprio — ver
	// TestLojaComConversorVaiAoResolver.
	p := plan(t, "Olha essa oferta https://br.shein.com/produto-p-123.html imperdível", r)

	if p.Relay == nil {
		t.Fatal("o entorno textual deveria ser relayado")
	}
	if strings.Contains(p.Relay.Body, "shein.com") {
		t.Errorf("URL da loja sem conversor sobreviveu:\n%s", p.Relay.Body)
	}
	if !strings.Contains(p.Relay.Body, "imperdível") {
		t.Errorf("o entorno foi perdido:\n%s", p.Relay.Body)
	}
	if strings.Contains(p.Relay.Body, "[link removido]") {
		t.Error("texto técnico não pode aparecer no grupo do cliente")
	}
	if p.Items[0].Reason != models.ReasonUnsupportedStore {
		t.Errorf("reason = %q", p.Items[0].Reason)
	}
	if atomic.LoadInt32(&r.calls) != 0 {
		t.Errorf("%d chamadas ao resolvedor — deveria ser 0", r.calls)
	}
}

// TestHostForjadoEhBloqueadoSemIO: o link de phishing vira item blocked e some
// do corpo, sem uma única chamada externa.
func TestHostForjadoEhBloqueadoSemIO(t *testing.T) {
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult {
		t.Error("host forjado não pode provocar I/O")
		return ResolveResult{}
	}}

	p := plan(t, "Promo https://evil.test/shopee.com.br/x agora", r)

	if len(p.Items) != 1 {
		t.Fatalf("%d itens, queria 1", len(p.Items))
	}
	if p.Items[0].ErrorClass != models.ErrorClassSecurityBlock {
		t.Errorf("error_class = %q, queria security_block", p.Items[0].ErrorClass)
	}
	if p.Items[0].Status != models.MonitorItemBlocked {
		t.Errorf("status = %q, queria blocked", p.Items[0].Status)
	}
	if p.Relay != nil && strings.Contains(p.Relay.Body, "evil.test") {
		t.Errorf("URL bloqueada sobreviveu no corpo:\n%s", p.Relay.Body)
	}
	if atomic.LoadInt32(&r.calls) != 0 {
		t.Errorf("%d chamadas — deveria ser 0", r.calls)
	}
}

// ---------------------------------------------------------------------------
// Parágrafos e limites
// ---------------------------------------------------------------------------

// TestParagrafoComLinkPendenteEspera: o bloco não pode sair mutilado agora para
// depois reaparecer inteiro quando o retry resolver.
func TestParagrafoComLinkPendenteEspera(t *testing.T) {
	r := &fakeResolver{fn: func(in ResolveRequest) ResolveResult {
		if strings.Contains(in.URL.Raw, "s.shopee") {
			return transitorio(models.ReasonTimeout)
		}
		return campanha("https://shope.ee/PRONTO")
	}}

	p := plan(t, "bloco A https://shopee.com.br/m/x\n\nbloco B https://s.shopee.com.br/pendente", r)

	if p.Campaign == nil {
		t.Fatal("o bloco pronto deveria sair")
	}
	if strings.Contains(p.Campaign.Body, "bloco B") {
		t.Errorf("bloco com link pendente saiu:\n%s", p.Campaign.Body)
	}
	if !strings.Contains(p.Campaign.Body, "bloco A") {
		t.Errorf("bloco pronto não saiu:\n%s", p.Campaign.Body)
	}
}

// TestMuitosLinksBloqueiaAMensagemInteira: acima do teto nada é chamado e nada
// é enviado — representar input ilimitado em linhas seria pior.
func TestMuitosLinksBloqueiaAMensagemInteira(t *testing.T) {
	var b strings.Builder
	// URLs DISTINTAS: ExtractLinks deduplica idênticas, então repetir a mesma
	// não exercitaria o teto.
	for i := 0; i <= models.MaxCommercialLinks; i++ {
		b.WriteString(fmt.Sprintf("https://shopee.com.br/product/1/%d ", 1000+i))
	}
	slots, blocked := ClassifyBody(b.String())
	if blocked != models.ReasonTooManyLinks {
		t.Fatalf("blocked = %q, queria too_many_links (%d slots)", blocked, len(slots))
	}
	if slots != nil {
		t.Error("nenhum slot pode ser devolvido quando a mensagem é bloqueada")
	}
}

func TestLinkNaoComercialNaoContaNemChamaResolver(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		b.WriteString("https://instagram.com/canal")
		b.WriteString(strings.Repeat("x", i+1))
		b.WriteString(" ")
	}
	slots, blocked := ClassifyBody(b.String())
	if blocked != "" {
		t.Fatalf("40 links não-comerciais bloquearam a mensagem: %q", blocked)
	}
	if len(slots) != 0 {
		t.Errorf("%d slots, queria 0 — non_commerce não vira item", len(slots))
	}
}

// TestTagsNaoViramLinkComercial: um cupom dentro de info[...] não pode
// concorrer com o produto real da mensagem.
func TestTagsNaoViramLinkComercialNemAparecemNoCorpo(t *testing.T) {
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult {
		return produto("shopee:1:1", "https://shope.ee/DONO")
	}}

	body := "info[cupom em https://shopee.com.br/m/cupom] Produto https://shopee.com.br/product/1/1"
	slots, _ := ClassifyBody(body)
	if len(slots) != 1 {
		t.Fatalf("%d slots, queria 1 (o link dentro de info[] não conta)", len(slots))
	}

	p := plan(t, body, r)
	for _, o := range p.Outputs() {
		if strings.Contains(o.Body, "info[") {
			t.Errorf("tag info[] apareceu no corpo público:\n%s", o.Body)
		}
	}
}

// ---------------------------------------------------------------------------
// Gate
// ---------------------------------------------------------------------------

// TestGateOffNaoGeraRelayComum é a prova de "gate OFF = byte a byte": sem o
// gate, mensagem de conteúdo comum não produz saída nenhuma.
func TestGateOffNaoGeraRelayComum(t *testing.T) {
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult { return ResolveResult{} }}

	p := plan(t, "só um texto https://instagram.com/canal", r, func(in *PlanInput) {
		in.MirrorCommon = false
	})

	if p.Relay != nil {
		t.Error("gate off não pode gerar relay de conteúdo comum")
	}
	if len(p.Outputs()) != 0 {
		t.Errorf("%d saídas com gate off, queria 0", len(p.Outputs()))
	}
}

func TestGateOnRelayaConteudoComum(t *testing.T) {
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult { return ResolveResult{} }}
	p := plan(t, "só um texto https://instagram.com/canal", r)

	if p.Relay == nil {
		t.Fatal("gate on deveria relayar conteúdo comum")
	}
	if !strings.Contains(p.Relay.Body, "instagram.com/canal") {
		t.Errorf("link não-comercial deveria passar intacto:\n%s", p.Relay.Body)
	}
}

// TestOfertaOuCampanhaSuprimeRelayComum: o corpo não pode sair duas vezes no
// mesmo grupo.
func TestOfertaSuprimeRelayComum(t *testing.T) {
	r := &fakeResolver{fn: func(ResolveRequest) ResolveResult {
		return produto("shopee:1:1", "https://shope.ee/DONO")
	}}
	p := plan(t, "Produto https://shopee.com.br/product/1/1", r)

	if p.Relay != nil {
		t.Error("com oferta não pode haver relay comum do mesmo corpo")
	}
}

// ---------------------------------------------------------------------------
// Aviso agregado
// ---------------------------------------------------------------------------

// TestCampanhasDeDuasLojasGeramUmAvisoSo prova o §6.6: um único aviso, cada
// parágrafo uma vez, um CTA só.
func TestCampanhasDeDuasLojasGeramUmAvisoSo(t *testing.T) {
	r := &fakeResolver{fn: func(in ResolveRequest) ResolveResult {
		if in.URL.Store == models.MonitorStoreShopee {
			return campanha("https://shope.ee/S")
		}
		return campanha("https://mercadolivre.com/sec/M")
	}}

	p := plan(t, "Shopee https://shopee.com.br/m/x\n\nML https://www.mercadolivre.com.br/cupons", r,
		func(in *PlanInput) { in.CTA = "CORRE LÁ" })

	if p.Campaign == nil {
		t.Fatal("faltou aviso")
	}
	if n := strings.Count(p.Campaign.Body, "CORRE LÁ"); n != 1 {
		t.Errorf("CTA aparece %d vezes, queria 1", n)
	}
	if !strings.Contains(p.Campaign.Body, "shope.ee/S") || !strings.Contains(p.Campaign.Body, "sec/M") {
		t.Errorf("os dois links do tenant deveriam estar no aviso:\n%s", p.Campaign.Body)
	}
	// Todos os itens de campanha compartilham o MESMO output_key: a entrega é
	// criada uma vez para o agregado.
	keys := map[string]struct{}{}
	for _, it := range p.Items {
		if it.OutputKind == models.OutputKindCampaignNotice {
			keys[it.OutputKey] = struct{}{}
		}
	}
	if len(keys) != 1 {
		t.Errorf("%d output_keys distintos no aviso agregado, queria 1", len(keys))
	}
}

// TestLiderDoAvisoEhOMenorOrdinal: só o líder pode ficar pending por falha de
// entrega, senão N itens gerariam N retries do MESMO aviso.
func TestLiderDoAvisoEhOMenorOrdinal(t *testing.T) {
	r := &fakeResolver{fn: func(in ResolveRequest) ResolveResult {
		return campanha("https://shope.ee/" + in.URL.Host)
	}}
	p := plan(t, "https://shopee.com.br/m/a\n\nhttps://www.mercadolivre.com.br/cupons", r)

	if p.Campaign == nil {
		t.Fatal("faltou aviso")
	}
	if p.Campaign.LeaderOrdinal != 0 {
		t.Errorf("líder = ordinal %d, queria 0", p.Campaign.LeaderOrdinal)
	}
	for _, it := range p.Items {
		if it.OutputKind == models.OutputKindCampaignNotice && it.Slot.Ordinal != 0 {
			if it.Outcome != models.MonitorOutcomeIncluded {
				t.Errorf("não-líder ordinal %d com outcome %q, queria included", it.Slot.Ordinal, it.Outcome)
			}
		}
	}
}

// TestLojaComConversorVaiAoResolver é o contra-teste do anterior: a partir do
// momento em que Amazon/AliExpress/Awin/Magalu ganharam conversor, o link
// precisa CHEGAR ao resolvedor (é lá que a reatribuição acontece) em vez de ser
// descartado localmente como loja sem conversor.
func TestLojaComConversorVaiAoResolver(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"amazon com ASIN", "https://www.amazon.com.br/Echo-Dot/dp/B09B8V1LZ3"},
		{"aliexpress com item", "https://pt.aliexpress.com/item/1005006123456789.html"},
		{"anunciante awin", "https://www.netshoes.com.br/produto/tenis-123"},
		{"magalu no catalogo", "https://www.magazineluiza.com.br/geladeira/p/hbf8fjj15a/ed/refr/"},
		// Vitrine de OUTRO divulgador: é o caso em que NÃO chegar ao resolvedor
		// significaria repassar o link com a comissão de quem postou.
		{"magalu na vitrine de terceiro", "https://www.magazinevoce.com.br/lojadooutro/geladeira/p/hbf8fjj15a/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeResolver{fn: func(ResolveRequest) ResolveResult {
				return produto("k", "https://link.do/TENANT")
			}}

			p := plan(t, "Olha essa oferta "+tc.url+" imperdível", r)

			if atomic.LoadInt32(&r.calls) == 0 {
				t.Fatal("loja com conversor precisa chegar ao resolvedor")
			}
			for _, o := range p.Outputs() {
				if strings.Contains(o.Body, tc.url) {
					t.Errorf("saída %s vazou a URL da fonte:\n%s", o.Kind, o.Body)
				}
			}
		})
	}
}
