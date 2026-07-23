package monitoroffer

import (
	"context"
	"errors"
	"strings"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/mercadolivre"
	"automatiza-go/internal/services/shopee"

	"go.uber.org/zap"
)

// StoreResolver é o LinkResolver real: traduz o resultado dos serviços
// Shopee/ML para a taxonomia fechada do plano.
//
// A tradução é o valor deste arquivo. Os serviços devolvem erros de formatos
// variados (sentinelas, strings, RemoteError); o planner precisa de exatamente
// três coisas — classe, error_class e reason_code — para decidir entre
// "retenta", "bloqueia" e "some do corpo". Sem essa camada, cada caminho
// reinterpretaria os erros à sua maneira, que é como um timeout virava campanha.
type StoreResolver struct {
	shopee shopee.Service
	ml     mercadolivre.Service
	logger *zap.Logger
}

// NewStoreResolver monta o resolvedor. Um serviço nil faz sua loja resolver
// como transient — nunca como campanha, e nunca deixando a URL de origem sair.
func NewStoreResolver(sh shopee.Service, ml mercadolivre.Service, logger *zap.Logger) *StoreResolver {
	return &StoreResolver{shopee: sh, ml: ml, logger: logger}
}

// Resolve implementa LinkResolver.
func (r *StoreResolver) Resolve(ctx context.Context, in ResolveRequest) ResolveResult {
	switch in.URL.Store {
	case models.MonitorStoreShopee:
		return r.resolveShopee(ctx, in)
	case models.MonitorStoreMercadoLivr:
		return r.resolveML(ctx, in)
	default:
		return ResolveResult{
			Class: models.URLClassUnsupportedCommerce, ErrorClass: models.ErrorClassNone,
			Reason: models.ReasonUnsupportedStore, Outcome: models.MonitorOutcomeRemoved,
		}
	}
}

func (r *StoreResolver) resolveShopee(ctx context.Context, in ResolveRequest) ResolveResult {
	if r.shopee == nil {
		return transientResult(models.ReasonCredentialLoadFail)
	}

	aff, err := r.shopee.GenerateAffiliateLink(ctx, in.URL.Raw, in.UserID)
	if err != nil {
		return classifyShopeeError(err)
	}
	if aff == nil || aff.AffiliateLink == "" {
		// Sucesso sem link é resposta incompleta, não campanha.
		return transientResult(models.ReasonInvalidResponse)
	}

	// Produto quando há IDs; senão a própria API confirmou campanha ao devolver
	// um short link para uma URL sem ItemID/ShopID (§5.3.7b).
	if aff.ItemID > 0 && aff.ShopID > 0 {
		return ResolveResult{
			Class: models.URLClassProduct, ErrorClass: models.ErrorClassNone,
			Reason:       models.ReasonNone,
			CanonicalKey: commerceurl.ShopeeKey(aff.ShopID, aff.ItemID),
			ConvertedURL: firstNonEmpty(aff.ShortLink, aff.AffiliateLink),
			Outcome:      models.MonitorOutcomeConverted,
			Offer: &OfferData{
				Store:    models.MonitorStoreShopee,
				ImageURL: aff.ImageURL,
			},
		}
	}
	return ResolveResult{
		Class: models.URLClassCampaignOrCoupon, ErrorClass: models.ErrorClassNone,
		Reason:       models.ReasonNone,
		ConvertedURL: firstNonEmpty(aff.ShortLink, aff.AffiliateLink),
		Outcome:      models.MonitorOutcomeConverted,
	}
}

// classifyShopeeError mapeia os erros do serviço Shopee para a taxonomia.
//
// A distinção que mais importa: credencial ausente/inválida é
// permanent_config (pausa a automação e o cliente conserta), enquanto 10030 e
// circuit breaker são transient (voltam sozinhos). Colapsar os dois faria uma
// rajada de rate limit pausar automações que não têm nada de errado.
func classifyShopeeError(err error) ResolveResult {
	switch {
	case errors.Is(err, shopee.ErrShopeeNoCredentials):
		return blockedResult(models.ErrorClassPermanentConfig, models.ReasonMissingCredentials)
	case errors.Is(err, shopee.ErrInvalidCredentials):
		return blockedResult(models.ErrorClassPermanentConfig, models.ReasonInvalidSignature)
	case errors.Is(err, shopee.ErrShopeeNoAPIAccess):
		// 10035: a conta não tem Open API. É permanente E por usuário — só ele
		// resolve, reativando na Shopee.
		return blockedResult(models.ErrorClassPermanentConfig, models.ReasonShopee10035)
	case errors.Is(err, shopee.ErrShopeeUnconvertibleURL):
		// Carrinho / URL que a própria Shopee recusa: o item não sobrevive à
		// expansão. A URL não volta ao corpo, mas o entorno pode ser relayado.
		return blockedResult(models.ErrorClassPermanentConte, models.ReasonCart)
	case errors.Is(err, shopee.ErrShopeeUnavailable):
		return transientResult(models.ReasonHTTP5xx)
	}

	// Bloqueio de segurança do commerceurl atravessa intacto: ele já é terminal
	// e não pode ser rebaixado a transient.
	if e, ok := commerceurl.AsError(err); ok {
		return blockedResult(e.Class, e.Reason)
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "10030"):
		return transientResult(models.ReasonShopee10030)
	case strings.Contains(msg, "10020"):
		return blockedResult(models.ErrorClassPermanentConfig, models.ReasonShopee10020)
	case strings.Contains(msg, "11001"):
		return blockedResult(models.ErrorClassPermanentConte, models.ReasonShopee11001)
	case strings.Contains(msg, "Invalid Signature"),
		strings.Contains(msg, "credentials incomplete"),
		strings.Contains(msg, "credentials not configured"):
		return blockedResult(models.ErrorClassPermanentConfig, models.ReasonInvalidSignature)
	case strings.Contains(msg, "failed to load shopee credentials"):
		// Falha de LEITURA no banco não é credencial ausente: um soluço do
		// Postgres não pode pausar a automação do cliente.
		return transientResult(models.ReasonCredentialLoadFail)
	}
	return transientResult(models.ReasonRemoteRejected)
}

func (r *StoreResolver) resolveML(ctx context.Context, in ResolveRequest) ResolveResult {
	if r.ml == nil {
		return transientResult(models.ReasonCredentialLoadFail)
	}

	// Exceção ÚNICA de passthrough (§5.4.7): URL ML validada postada pelo
	// PRÓPRIO dono. Post de terceiro sempre reatribui — é a diferença entre
	// respeitar o link do cliente e repassar a comissão de um estranho.
	if in.FromMe && in.Local.Class == models.URLClassProduct {
		return ResolveResult{
			Class: models.URLClassProduct, ErrorClass: models.ErrorClassNone,
			Reason:       models.ReasonNone,
			CanonicalKey: in.Local.CanonicalKey,
			ConvertedURL: in.Local.CanonicalURL,
			Outcome:      models.MonitorOutcomeOwnerPassthru,
			Offer:        &OfferData{Store: models.MonitorStoreMercadoLivr},
		}
	}

	aff, err := r.ml.CreateAffiliateLink(ctx, in.URL.Raw, in.UserID)
	if err != nil {
		return classifyMLError(err)
	}
	if aff == nil || aff.AffiliateLink == "" {
		return transientResult(models.ReasonInvalidResponse)
	}

	key := in.Local.CanonicalKey
	if key == "" && aff.ProductID != "" {
		key = commerceurl.MLKey(aff.ProductID)
	}
	if key != "" {
		return ResolveResult{
			Class: models.URLClassProduct, ErrorClass: models.ErrorClassNone,
			Reason: models.ReasonNone, CanonicalKey: key,
			ConvertedURL: aff.AffiliateLink, Outcome: models.MonitorOutcomeConverted,
			Offer: &OfferData{Store: models.MonitorStoreMercadoLivr},
		}
	}
	// Sem MLB: só é campanha se o path JÁ provou campanha localmente
	// (/cupons, /l/). Caso contrário permanece unknown — ausência de ID não é
	// prova de nada.
	if in.Local.Class == models.URLClassCampaignOrCoupon {
		return ResolveResult{
			Class: models.URLClassCampaignOrCoupon, ErrorClass: models.ErrorClassNone,
			Reason: models.ReasonNone, ConvertedURL: aff.AffiliateLink,
			Outcome: models.MonitorOutcomeConverted,
		}
	}
	return transientResult(models.ReasonPreviewIncomplete)
}

// classifyMLError usa o RemoteError tipado quando existe — ele já carrega
// classe e reason decididos na fronteira HTTP.
func classifyMLError(err error) ResolveResult {
	var remote *mercadolivre.RemoteError
	if errors.As(err, &remote) {
		if remote.Class == models.ErrorClassTransient {
			return transientResult(remote.Reason)
		}
		return blockedResult(remote.Class, remote.Reason)
	}
	if e, ok := commerceurl.AsError(err); ok {
		return blockedResult(e.Class, e.Reason)
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "credentials not configured"),
		strings.Contains(msg, "csrf token is required"):
		return blockedResult(models.ErrorClassPermanentConfig, models.ReasonMissingCredentials)
	case strings.Contains(msg, "invalid mercadolivre url"):
		return blockedResult(models.ErrorClassPermanentConte, models.ReasonInvalidURL)
	}
	return transientResult(models.ReasonRemoteRejected)
}

func transientResult(reason string) ResolveResult {
	return ResolveResult{
		Class: models.URLClassUnknown, ErrorClass: models.ErrorClassTransient,
		Reason: reason, Outcome: models.MonitorOutcomeNone,
	}
}

func blockedResult(class, reason string) ResolveResult {
	return ResolveResult{
		Class: models.URLClassUnknown, ErrorClass: class,
		Reason: reason, Outcome: models.MonitorOutcomeRemoved,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// PausesAutomation reporta se este desfecho deve pausar a automação.
//
// Só permanent_config: é o erro que o CLIENTE resolve (credencial errada,
// conta sem Open API). Pausar por transient transformaria uma rajada de rate
// limit numa pausa em massa — foi assim que 303 usuários pararam em 16/07.
func PausesAutomation(errorClass string) bool {
	return errorClass == models.ErrorClassPermanentConfig
}
