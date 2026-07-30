package connectrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	affiliatesv1 "automatiza-go/internal/gen/affiliates/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/awin"
	"automatiza-go/internal/services/magalu"
	"automatiza-go/internal/services/shopee"

	"connectrpc.com/connect"
)

// AffiliateCredentialsHandler implements the affiliatesv1connect.AffiliateCredentialsServiceHandler interface.
type AffiliateCredentialsHandler struct {
	credentialsRepo repository.AffiliateCredentialsRepository

	// validateShopee testa App ID + Senha contra a API da Shopee antes de
	// salvar (injetável para testes). Default: shopee.ValidateAffiliateCredentials.
	validateShopee func(ctx context.Context, appID, appSecret string) error

	// fetchAwinProgrammes lista os anunciantes aprovados do publisher
	// (injetável para testes). Default: awin.FetchProgrammes.
	fetchAwinProgrammes func(ctx context.Context, publisherID, token string) ([]awin.Programme, error)
}

// NewAffiliateCredentialsHandler creates a new affiliate credentials handler.
func NewAffiliateCredentialsHandler(credentialsRepo repository.AffiliateCredentialsRepository) *AffiliateCredentialsHandler {
	return &AffiliateCredentialsHandler{
		credentialsRepo: credentialsRepo,
		validateShopee: func(ctx context.Context, appID, appSecret string) error {
			return shopee.ValidateAffiliateCredentials(ctx, "", appID, appSecret)
		},
		fetchAwinProgrammes: func(ctx context.Context, publisherID, token string) ([]awin.Programme, error) {
			return awin.FetchProgrammes(ctx, "", publisherID, token)
		},
	}
}

// programmeNames reduz os programas aos nomes exibidos no seletor de marcas.
// Vai para o JSONB como chave NÃO-secreta e NÃO-interna, então chega ao
// frontend junto com o resto da credencial — sem precisar de campo novo no
// proto nem de bundle novo no navegador.
func programmeNames(programmes []awin.Programme) []string {
	out := make([]string, 0, len(programmes))
	for _, p := range programmes {
		if p.Name != "" {
			out = append(out, p.Name)
		}
	}
	return out
}

// awinCredentials extrai publisher ID e token de uma credencial Awin, aceitando
// as duas grafias (formulário atual e legado do JSONB).
func awinCredentials(c *models.AffiliateCredentials) (publisherID, token string) {
	publisherID = strings.TrimSpace(c.GetCredential("awinId"))
	if publisherID == "" {
		publisherID = strings.TrimSpace(c.GetCredential("publisher_id"))
	}
	token = strings.TrimSpace(c.GetCredential("oauth2Token"))
	if token == "" {
		token = strings.TrimSpace(c.GetCredential("api_key"))
	}
	return publisherID, token
}

// ListCredentials returns all affiliate credentials for the authenticated user.
func (h *AffiliateCredentialsHandler) ListCredentials(
	ctx context.Context,
	req *connect.Request[affiliatesv1.ListCredentialsRequest],
) (*connect.Response[affiliatesv1.ListCredentialsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	credentials, err := h.credentialsRepo.GetAllByUser(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list credentials: %w", err))
	}

	protoCredentials := make([]*affiliatesv1.AffiliateCredential, len(credentials))
	for i, c := range credentials {
		protoCredentials[i] = credentialToProto(&c)
	}

	return connect.NewResponse(&affiliatesv1.ListCredentialsResponse{
		Credentials: protoCredentials,
	}), nil
}

// GetCredential returns credentials for a specific store.
func (h *AffiliateCredentialsHandler) GetCredential(
	ctx context.Context,
	req *connect.Request[affiliatesv1.GetCredentialRequest],
) (*connect.Response[affiliatesv1.GetCredentialResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	store := protoStoreToModelAffiliate(req.Msg.GetStore())
	if store == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid store"))
	}

	credential, err := h.credentialsRepo.GetByUserAndStore(ctx, user.ID, store)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get credential: %w", err))
	}

	// Return empty response if not found (optional field in proto)
	if credential == nil {
		return connect.NewResponse(&affiliatesv1.GetCredentialResponse{}), nil
	}

	return connect.NewResponse(&affiliatesv1.GetCredentialResponse{
		Credential: credentialToProto(credential),
	}), nil
}

// UpsertCredential creates or updates credentials for a store.
func (h *AffiliateCredentialsHandler) UpsertCredential(
	ctx context.Context,
	req *connect.Request[affiliatesv1.UpsertCredentialRequest],
) (*connect.Response[affiliatesv1.UpsertCredentialResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	store := protoStoreToModelAffiliate(req.Msg.GetStore())
	if store == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid store"))
	}

	// Parse credentials JSON string into map
	credentialsJSON := req.Msg.GetCredentials()
	var credentialsMap map[string]string
	if err := json.Unmarshal([]byte(credentialsJSON), &credentialsMap); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid credentials format: must be a JSON object"))
	}

	// Build the credentials model
	data := &models.AffiliateCredentials{
		Store:       store,
		Credentials: credentialsMap,
	}

	// Campo secreto vazio ou MASCARADO preserva o valor salvo. O GET devolve
	// secrets mascarados (nunca mais o valor real para o navegador), então o
	// round-trip do formulário chega aqui com a máscara — e autofill do
	// navegador/estado stale não consegue mais sobrescrever um secret válido
	// com lixo (incidente 11/07 21:58: secret válido da Isa virou inválido
	// sem ninguém tocar nas credenciais — o formulário persistiu um valor
	// que não veio do usuário).
	if existing, exErr := h.credentialsRepo.GetByUserAndStore(ctx, user.ID, store); exErr == nil && existing != nil {
		for _, key := range secretCredentialKeys {
			incoming, has := credentialsMap[key]
			if !has {
				continue
			}
			if isMaskedOrEmptySecret(incoming) {
				if old := existing.GetCredential(key); old != "" {
					credentialsMap[key] = old
				}
			}
		}
		// Chaves internas nunca saem para o navegador, então nunca voltam no
		// payload: sem esta cópia, o SEGUNDO save do formulário apagaria o mapa
		// de programas do Awin e a conversão pararia sem erro visível.
		for _, key := range internalCredentialKeys {
			if _, has := credentialsMap[key]; has {
				continue
			}
			if old := existing.GetCredential(key); old != "" {
				credentialsMap[key] = old
			}
		}
	}

	// Shopee: valida App ID + Senha AO VIVO antes de salvar. Um secret errado
	// aceito aqui só aparece depois, como rajada de [10020] Invalid Signature
	// queimando itens de lista e pausando automações (incidente 11/07 — a
	// cliente salvou secret regenerado e 90 itens foram queimados em 2h).
	// Fail-open para instabilidade da Shopee; só rejeita 10020 determinístico.
	if store == "shopee" && h.validateShopee != nil {
		appID, appSecret := data.GetCredential("appId"), data.GetCredential("password")
		if appID == "" {
			appID = data.GetCredential("app_id")
		}
		if appSecret == "" {
			appSecret = data.GetCredential("app_secret")
		}
		if err := h.validateShopee(ctx, appID, appSecret); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(
				"credenciais Shopee invalidas: a Senha nao confere com o painel de afiliados da Shopee. Copie o App ID e a Senha novamente (sem espacos) e tente de novo"))
		}
	}

	// Awin: a MESMA chamada faz dois trabalhos — valida o token e traz o mapa
	// domínio→awinmid, sem o qual a conversão de link não sabe qual anunciante
	// creditar. Mesmo contrato fail-open da Shopee: só 401/403 (determinístico)
	// bloqueia o cadastro; timeout e 5xx deixam salvar.
	if store == "awin" && h.fetchAwinProgrammes != nil {
		publisherID, token := awinCredentials(data)
		if publisherID != "" && token != "" {
			programmes, err := h.fetchAwinProgrammes(ctx, publisherID, token)
			switch {
			case errors.Is(err, awin.ErrInvalidCredentials):
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(
					"credenciais Awin invalidas: a Awin recusou o token OAuth2. Gere um token novo em ui.awin.com/awin-api e confira se o Awin ID e o do seu perfil de publisher"))
			case err != nil:
				// Instabilidade da Awin não pode travar o cadastro; a lista de
				// marcas é sincronizada de novo no botão Testar.
			default:
				if encoded, encErr := awin.EncodeDomainMap(awin.DomainMap(programmes)); encErr == nil {
					data.SetCredential(awin.ProgrammesCredentialKey, encoded)
				}
				if brands, brErr := awin.EncodeBrandMap(awin.BrandMap(programmes)); brErr == nil {
					data.SetCredential(awin.BrandMapCredentialKey, brands)
				}
				if names, nameErr := json.Marshal(programmeNames(programmes)); nameErr == nil {
					data.SetCredential(awin.ProgrammeNamesCredentialKey, string(names))
				}
			}
		}
	}

	// Magalu: normaliza a loja da Vitrine para o slug canônico ANTES de gravar.
	// O cliente cola qualquer coisa (o slug puro, o domínio, a URL completa, um
	// link de produto da própria vitrine) e o conversor precisa de um valor só.
	// Guardar já normalizado evita re-derivar a cada envio e faz o painel
	// mostrar o que de fato será usado. Sem rede: o Magalu devolve 403 para
	// HTTP puro, então validar online seria falso-negativo.
	if store == "magalu" {
		raw := data.GetCredential("partnerStore")
		if raw == "" {
			raw = data.GetCredential("partner_store")
		}
		if raw != "" {
			slug := magalu.NormalizeStoreSlug(raw)
			if slug == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(
					"loja parceira Magalu invalida: informe o identificador da sua Vitrine (ex.: minhaloja) ou cole o link dela (https://www.magazinevoce.com.br/minhaloja). Link do magazineluiza.com.br nao serve: ele nao contem a sua loja"))
			}
			data.SetCredential("partnerStore", slug)
		}
	}

	// Handle auto_generate - default to true if not specified
	if req.Msg.AutoGenerate != nil {
		data.AutoGenerate = req.Msg.GetAutoGenerate()
	} else {
		data.AutoGenerate = true
	}

	// Determine is_active based on credential validation
	// This matches the Node.js behavior where is_active is calculated from credential validity
	if req.Msg.IsActive != nil {
		// If explicitly provided, use that value (but only if credentials are valid)
		data.IsActive = req.Msg.GetIsActive() && data.HasValidCredentials()
	} else {
		// Auto-calculate is_active based on credential validation
		data.IsActive = data.HasValidCredentials()
	}

	// Upsert the credentials
	if err := h.credentialsRepo.Upsert(ctx, user.ID, store, data); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save credentials: %w", err))
	}

	// Fetch the updated credential to return
	credential, err := h.credentialsRepo.GetByUserAndStore(ctx, user.ID, store)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated credential: %w", err))
	}

	return connect.NewResponse(&affiliatesv1.UpsertCredentialResponse{
		Credential: credentialToProto(credential),
	}), nil
}

// DeleteCredential deletes credentials for a store.
func (h *AffiliateCredentialsHandler) DeleteCredential(
	ctx context.Context,
	req *connect.Request[affiliatesv1.DeleteCredentialRequest],
) (*connect.Response[affiliatesv1.DeleteCredentialResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	store := protoStoreToModelAffiliate(req.Msg.GetStore())
	if store == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid store"))
	}

	// Hard delete the credentials
	if err := h.credentialsRepo.Delete(ctx, user.ID, store); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete credentials: %w", err))
	}

	return connect.NewResponse(&affiliatesv1.DeleteCredentialResponse{}), nil
}

// ListActiveStores returns all stores with active credentials for the authenticated user.
func (h *AffiliateCredentialsHandler) ListActiveStores(
	ctx context.Context,
	req *connect.Request[affiliatesv1.ListActiveStoresRequest],
) (*connect.Response[affiliatesv1.ListActiveStoresResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Get all credentials to build the active stores response
	credentials, err := h.credentialsRepo.GetAllByUser(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list active stores: %w", err))
	}

	// Build a map of configured stores
	configuredStores := make(map[string]*models.AffiliateCredentials)
	for i := range credentials {
		c := &credentials[i]
		configuredStores[c.Store] = c
	}

	// Build the response with all known stores
	allStores := []affiliatesv1.AffiliateStore{
		affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
		affiliatesv1.AffiliateStore_AFFILIATE_STORE_AMAZON,
		affiliatesv1.AffiliateStore_AFFILIATE_STORE_MAGALU,
		affiliatesv1.AffiliateStore_AFFILIATE_STORE_MERCADO_LIVRE,
		affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHEIN,
		affiliatesv1.AffiliateStore_AFFILIATE_STORE_ALIEXPRESS,
		affiliatesv1.AffiliateStore_AFFILIATE_STORE_AWIN,
	}

	protoStores := make([]*affiliatesv1.ActiveStore, 0, len(allStores))
	for _, store := range allStores {
		storeModel := protoStoreToModelAffiliate(store)
		if storeModel == "" {
			continue
		}

		activeStore := &affiliatesv1.ActiveStore{
			Store:        store,
			IsConfigured: false,
			AutoGenerate: false,
		}

		if cred, ok := configuredStores[storeModel]; ok {
			activeStore.IsConfigured = cred.HasValidCredentials()
			activeStore.AutoGenerate = cred.AutoGenerate
		}

		protoStores = append(protoStores, activeStore)
	}

	return connect.NewResponse(&affiliatesv1.ListActiveStoresResponse{
		Stores: protoStores,
	}), nil
}

// Helper functions

// credentialToProto converts a model credential to proto format.
// secretCredentialKeys são os campos de credencial que NUNCA saem em claro
// para o cliente e que, quando chegam vazios/mascarados num save, preservam
// o valor armazenado.
var secretCredentialKeys = []string{
	"password", "app_secret", "secret", "apiToken", "api_token",
	// Segredos das lojas que ganharam formulário depois: sem eles nesta lista,
	// o GET devolvia o valor REAL ao navegador (e o autofill podia sobrescrevê-lo
	// com lixo — o incidente 11/07 que criou a máscara).
	"secretKey", "secret_key", // Amazon (Creators API)
	"clientSecret",                // Amazon (Creators API)
	"oauth2Token", "oauth2_token", // Awin
	"appSecret", // AliExpress
}

// internalCredentialKeys são chaves que o servidor mantém no JSONB mas que
// NUNCA vão para o navegador — não por sigilo, por volume e por não terem uso
// na UI. Diferente dos segredos, elas são REMOVIDAS da resposta em vez de
// mascaradas, e preservadas no save quando não vêm no payload.
var internalCredentialKeys = []string{awin.ProgrammesCredentialKey, awin.BrandMapCredentialKey}

// secretMask é o placeholder devolvido no lugar de secrets. O formulário
// exibe os pontinhos; se voltarem intactos num save, o Upsert mantém o valor
// salvo (ver isMaskedOrEmptySecret).
const secretMask = "••••••••"

// isMaskedOrEmptySecret reconhece valores que NÃO são um secret digitado pelo
// usuário: vazio, o placeholder do GET, ou uma sequência de caracteres de
// máscara (•, *, ●, ·).
func isMaskedOrEmptySecret(v string) bool {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return true
	}
	for _, r := range trimmed {
		switch r {
		case '•', '*', '●', '·':
		default:
			return false
		}
	}
	return true
}

func credentialToProto(c *models.AffiliateCredentials) *affiliatesv1.AffiliateCredential {
	if c == nil {
		return nil
	}

	// Convert credentials map to JSON string — com secrets MASCARADOS. O
	// valor real nunca mais viaja ao navegador: além de vazamento, era o
	// vetor da corrupção silenciosa (autofill/estado stale re-salvando por
	// cima do secret válido).
	var credentialsJSON string
	if c.Credentials != nil {
		masked := make(map[string]string, len(c.Credentials))
		for k, v := range c.Credentials {
			masked[k] = v
		}
		for _, key := range secretCredentialKeys {
			if v, ok := masked[key]; ok && strings.TrimSpace(v) != "" {
				masked[key] = secretMask
			}
		}
		// Chave interna sai FORA (não mascarada): mascarar faria o mapa de
		// programas voltar como "••••••••" e o save seguinte gravaria isso
		// por cima do mapa real.
		for _, key := range internalCredentialKeys {
			delete(masked, key)
		}
		if data, err := json.Marshal(masked); err == nil {
			credentialsJSON = string(data)
		}
	}

	return &affiliatesv1.AffiliateCredential{
		Id:           c.ID,
		UserId:       c.UserID,
		Store:        modelStoreToProtoAffiliate(c.Store),
		Credentials:  credentialsJSON,
		AutoGenerate: c.AutoGenerate,
		IsActive:     c.IsActive,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

// modelStoreToProtoAffiliate converts a model store string to proto enum.
func modelStoreToProtoAffiliate(store string) affiliatesv1.AffiliateStore {
	switch store {
	case "shopee":
		return affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE
	case "amazon":
		return affiliatesv1.AffiliateStore_AFFILIATE_STORE_AMAZON
	case "magalu":
		return affiliatesv1.AffiliateStore_AFFILIATE_STORE_MAGALU
	case "mercado_livre", "mercadolivre", "ml":
		return affiliatesv1.AffiliateStore_AFFILIATE_STORE_MERCADO_LIVRE
	case "shein":
		return affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHEIN
	case "aliexpress":
		return affiliatesv1.AffiliateStore_AFFILIATE_STORE_ALIEXPRESS
	case "awin":
		return affiliatesv1.AffiliateStore_AFFILIATE_STORE_AWIN
	default:
		return affiliatesv1.AffiliateStore_AFFILIATE_STORE_UNSPECIFIED
	}
}

// protoStoreToModelAffiliate converts a proto store enum to model string.
func protoStoreToModelAffiliate(store affiliatesv1.AffiliateStore) string {
	switch store {
	case affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE:
		return "shopee"
	case affiliatesv1.AffiliateStore_AFFILIATE_STORE_AMAZON:
		return "amazon"
	case affiliatesv1.AffiliateStore_AFFILIATE_STORE_MAGALU:
		return "magalu"
	case affiliatesv1.AffiliateStore_AFFILIATE_STORE_MERCADO_LIVRE:
		return "mercado_livre"
	case affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHEIN:
		return "shein"
	case affiliatesv1.AffiliateStore_AFFILIATE_STORE_ALIEXPRESS:
		return "aliexpress"
	case affiliatesv1.AffiliateStore_AFFILIATE_STORE_AWIN:
		return "awin"
	default:
		return ""
	}
}

// ListAwinProgrammes lista os anunciantes Awin aprovados para o usuário.
//
// Serve a dois propósitos de propósito: alimenta o seletor de marcas do
// formulário E é o teste real da credencial Awin (é a mesma chamada). O token
// não viaja no request — o servidor lê a credencial já salva —, então apertar
// "Testar" nunca persiste o conteúdo do formulário por cima de um segredo
// válido (o vetor do incidente 11/07).
func (h *AffiliateCredentialsHandler) ListAwinProgrammes(
	ctx context.Context,
	req *connect.Request[affiliatesv1.ListAwinProgrammesRequest],
) (*connect.Response[affiliatesv1.ListAwinProgrammesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if h.fetchAwinProgrammes == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("integracao Awin indisponivel"))
	}

	credential, err := h.credentialsRepo.GetByUserAndStore(ctx, user.ID, "awin")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get credential: %w", err))
	}
	if credential == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf(
			"salve o Awin ID e o token OAuth2 antes de listar as marcas"))
	}

	publisherID, token := awinCredentials(credential)
	if publisherID == "" || token == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf(
			"credencial Awin incompleta: preencha o Awin ID e o token OAuth2"))
	}

	programmes, err := h.fetchAwinProgrammes(ctx, publisherID, token)
	switch {
	case errors.Is(err, awin.ErrInvalidCredentials):
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(
			"a Awin recusou o token OAuth2. Gere um token novo em ui.awin.com/awin-api e confira se o Awin ID e o do seu perfil de publisher"))
	case err != nil:
		// Instabilidade da Awin: o cliente mantém a lista que já tem em cache.
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf(
			"a Awin nao respondeu agora. Tente de novo em alguns minutos"))
	}

	// Persiste o mapa domínio→awinmid: é ele que a conversão de link usa em
	// runtime, e este é o único momento em que a lista chega fresca.
	h.persistAwinProgrammes(ctx, user.ID, credential, programmes)

	out := make([]*affiliatesv1.AwinProgramme, 0, len(programmes))
	for _, p := range programmes {
		out = append(out, &affiliatesv1.AwinProgramme{
			Id:         p.ID,
			Name:       p.Name,
			DisplayUrl: p.DisplayURL,
		})
	}
	return connect.NewResponse(&affiliatesv1.ListAwinProgrammesResponse{Programmes: out}), nil
}

// persistAwinProgrammes grava o mapa de programas na credencial. Best-effort:
// falhar aqui não pode derrubar a listagem que o usuário pediu.
func (h *AffiliateCredentialsHandler) persistAwinProgrammes(
	ctx context.Context,
	userID string,
	credential *models.AffiliateCredentials,
	programmes []awin.Programme,
) {
	if len(programmes) == 0 {
		return
	}
	encoded, err := awin.EncodeDomainMap(awin.DomainMap(programmes))
	if err != nil {
		return
	}
	credential.SetCredential(awin.ProgrammesCredentialKey, encoded)
	if brands, brErr := awin.EncodeBrandMap(awin.BrandMap(programmes)); brErr == nil {
		credential.SetCredential(awin.BrandMapCredentialKey, brands)
	}
	if names, nameErr := json.Marshal(programmeNames(programmes)); nameErr == nil {
		credential.SetCredential(awin.ProgrammeNamesCredentialKey, string(names))
	}
	_ = h.credentialsRepo.Upsert(ctx, userID, "awin", credential)
}
