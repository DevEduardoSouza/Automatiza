package models

import (
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Melhoria 9 — monitor com conteúdo diverso e ofertas.
//
// Este arquivo é a parte PURA do domínio: taxonomia fechada (url_class,
// error_class, reason_code), estados do ledger e as linhas de
// monitor_offer_jobs/items/deliveries. Nada aqui faz I/O.
//
// A taxonomia é fechada de propósito: os mesmos valores são CHECK constraint na
// migração e enum no código. Um reason code novo exige migração — é o preço de
// não deixar erro livre virar string arbitrária no banco (e de nunca persistir
// URL crua num campo de erro).
// =============================================================================

// url_class — o que a URL é, depois de classificada com host exato.
const (
	// URLClassProduct: produto canônico identificado (Shopee shop/item ou MLB).
	URLClassProduct = "product"
	// URLClassCampaignOrCoupon: campanha/cupom confirmado por path positivo.
	URLClassCampaignOrCoupon = "campaign_or_coupon"
	// URLClassNonCommerce: link comum; não gera I/O nem conta para o teto.
	URLClassNonCommerce = "non_commerce"
	// URLClassUnsupportedCommerce: loja sem conversor (Amazon/Magalu/...).
	URLClassUnsupportedCommerce = "unsupported_commerce"
	// URLClassUnknown: ainda indeterminado. NUNCA equivale a campanha.
	URLClassUnknown = "unknown"
)

// error_class — por que o item não chegou a um desfecho positivo.
const (
	ErrorClassNone            = "none"
	ErrorClassTransient       = "transient"
	ErrorClassPermanentConfig = "permanent_config"
	ErrorClassPermanentConte  = "permanent_content"
	ErrorClassSecurityBlock   = "security_block"
)

// reason_code — enum fechado. A ordem espelha a §5.1 do plano e o CHECK da
// migração; acrescentar valor aqui sem acrescentar na migração quebra o insert.
const (
	ReasonNone                = "none"
	ReasonTimeout             = "timeout"
	ReasonDNS                 = "dns"
	ReasonHTTP3xx             = "http_3xx"
	ReasonHTTP408             = "http_408"
	ReasonHTTP425             = "http_425"
	ReasonHTTP429             = "http_429"
	ReasonHTTP4xx             = "http_4xx"
	ReasonHTTP5xx             = "http_5xx"
	ReasonInvalidResponse     = "invalid_response"
	ReasonCredentialLoadFail  = "credential_load_failed"
	ReasonShopee10020         = "shopee_10020"
	ReasonShopee10030         = "shopee_10030"
	ReasonRemoteRejected      = "remote_rejected"
	ReasonPreviewIncomplete   = "preview_incomplete"
	ReasonParserInconclusive  = "parser_inconclusive"
	ReasonResponseTooLarge    = "response_too_large"
	ReasonInvalidSignature    = "invalid_signature"
	ReasonMissingCredentials  = "missing_credentials"
	ReasonInvalidToken        = "invalid_token"
	ReasonShopee10035         = "shopee_10035"
	ReasonShopee11001         = "shopee_11001"
	ReasonCart                = "cart"
	ReasonNotFound            = "not_found"
	ReasonInvalidURL          = "invalid_url"
	ReasonUnsafeScheme        = "unsafe_scheme"
	ReasonUnsafeHost          = "unsafe_host"
	ReasonPrivateAddress      = "private_address"
	ReasonRedirectEscape      = "redirect_escape"
	ReasonURLTooLong          = "url_too_long"
	ReasonTooManyLinks        = "too_many_links"
	ReasonUnsupportedStore    = "unsupported_store"
	ReasonDuplicateLink       = "duplicate_link"
	ReasonExcessProduct       = "excess_product"
	ReasonAutomationInactive  = "automation_inactive"
	ReasonAutomationOwnerMism = "automation_owner_mismatch"
	ReasonSourceGroupMismatch = "source_group_mismatch"
	ReasonModeMismatch        = "mode_mismatch"
	ReasonOutsideWindow       = "outside_window"
	ReasonNoDestination       = "no_destination"
	ReasonNoChip              = "no_chip"
	ReasonAlreadySent         = "already_sent"
	ReasonMissingMessageID    = "missing_message_id"
	ReasonPayloadTooLarge     = "payload_too_large"
	ReasonPayloadEncryptFail  = "payload_encrypt_failed"
	ReasonPayloadDecryptFail  = "payload_decrypt_failed"
	ReasonPayloadExpired      = "payload_expired"
	ReasonUnsupportedMedia    = "unsupported_media"
	ReasonMediaUnavailable    = "media_unavailable"
	ReasonMediaFPUnavailable  = "media_fingerprint_unavailable"
	ReasonSessionRestricted   = "session_restricted"
	ReasonNotParticipating    = "not_participating"
	ReasonDeliveryRejected    = "delivery_rejected"
	ReasonDeliveryAmbiguous   = "delivery_ambiguous"
)

// allReasonCodes é a lista canônica usada pela validação de boot e pelo teste
// que compara código × CHECK da migração 079.
var allReasonCodes = []string{
	ReasonNone, ReasonTimeout, ReasonDNS, ReasonHTTP3xx, ReasonHTTP408,
	ReasonHTTP425, ReasonHTTP429, ReasonHTTP4xx, ReasonHTTP5xx,
	ReasonInvalidResponse, ReasonCredentialLoadFail, ReasonShopee10020,
	ReasonShopee10030, ReasonRemoteRejected, ReasonPreviewIncomplete,
	ReasonParserInconclusive, ReasonResponseTooLarge, ReasonInvalidSignature,
	ReasonMissingCredentials, ReasonInvalidToken, ReasonShopee10035,
	ReasonShopee11001, ReasonCart, ReasonNotFound, ReasonInvalidURL,
	ReasonUnsafeScheme, ReasonUnsafeHost, ReasonPrivateAddress,
	ReasonRedirectEscape, ReasonURLTooLong, ReasonTooManyLinks,
	ReasonUnsupportedStore, ReasonDuplicateLink, ReasonExcessProduct,
	ReasonAutomationInactive, ReasonAutomationOwnerMism,
	ReasonSourceGroupMismatch, ReasonModeMismatch, ReasonOutsideWindow,
	ReasonNoDestination, ReasonNoChip, ReasonAlreadySent,
	ReasonMissingMessageID, ReasonPayloadTooLarge, ReasonPayloadEncryptFail,
	ReasonPayloadDecryptFail, ReasonPayloadExpired, ReasonUnsupportedMedia,
	ReasonMediaUnavailable, ReasonMediaFPUnavailable, ReasonSessionRestricted,
	ReasonNotParticipating, ReasonDeliveryRejected, ReasonDeliveryAmbiguous,
}

// AllReasonCodes devolve uma cópia da lista canônica de reason codes.
func AllReasonCodes() []string {
	out := make([]string, len(allReasonCodes))
	copy(out, allReasonCodes)
	return out
}

var reasonCodeSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(allReasonCodes))
	for _, r := range allReasonCodes {
		m[r] = struct{}{}
	}
	return m
}()

// IsValidReasonCode reporta se r pertence ao enum fechado. Chamado antes de
// qualquer escrita para que um reason code fora do enum falhe no código (com
// contexto) e não como violação de CHECK no Postgres.
func IsValidReasonCode(r string) bool {
	_, ok := reasonCodeSet[r]
	return ok
}

// Modos aceitos pelo ledger. Espelham o CHECK de monitor_offer_jobs.mode.
const (
	MonitorOfferModeMonitoring    = "monitoring"
	MonitorOfferModeOfferGenerate = "offer_generate"
)

// Lojas reconhecidas pelo ledger.
const (
	MonitorStoreShopee      = "shopee"
	MonitorStoreMercadoLivr = "mercadolivre"
	MonitorStoreUnsupported = "unsupported"
)

// Status do job.
const (
	MonitorJobActive     = "active"
	MonitorJobSucceeded  = "succeeded"
	MonitorJobExhausted  = "exhausted"
	MonitorJobBlocked    = "blocked"
	MonitorJobSuperseded = "superseded"
	MonitorJobAmbiguous  = "ambiguous"
)

// Status do item.
const (
	MonitorItemPending    = "pending"
	MonitorItemProcessing = "processing"
	MonitorItemSucceeded  = "succeeded"
	MonitorItemExhausted  = "exhausted"
	MonitorItemBlocked    = "blocked"
	MonitorItemSuperseded = "superseded"
	MonitorItemAmbiguous  = "ambiguous"
)

// outcome do item — o que aconteceu com a URL no corpo publicado.
const (
	MonitorOutcomeNone          = "none"
	MonitorOutcomeConverted     = "converted"
	MonitorOutcomeOwnerPassthru = "owner_passthrough"
	MonitorOutcomeRemoved       = "removed"
	MonitorOutcomeIncluded      = "included"
)

// Tipos de saída.
const (
	OutputKindProductOffer   = "product_offer"
	OutputKindCampaignNotice = "campaign_notice"
	OutputKindCommonRelay    = "common_relay"
)

// Status da delivery.
const (
	DeliveryPrepared  = "prepared"
	DeliverySending   = "sending"
	DeliverySent      = "sent"
	DeliveryFailed    = "failed"
	DeliveryAmbiguous = "ambiguous"
	DeliverySkipped   = "skipped"
)

// MaxItemAttempts é o teto de tentativas duráveis por item (§8.1).
const MaxItemAttempts = 5

// MaxCommercialLinks é o teto de URLs comerciais por mensagem (§5.2).
// A 21ª bloqueia a mensagem inteira com too_many_links.
const MaxCommercialLinks = 20

// MonitorOfferJob é a linha de monitor_offer_jobs.
type MonitorOfferJob struct {
	ID                 uuid.UUID `json:"id" db:"id"`
	UserID             uuid.UUID `json:"user_id" db:"user_id"`
	AutomationID       uuid.UUID `json:"automation_id" db:"automation_id"`
	Mode               string    `json:"mode" db:"mode"`
	SourceGroupID      string    `json:"source_group_id" db:"source_group_id"`
	MessageID          string    `json:"message_id" db:"message_id"`
	MessageTimestamp   time.Time `json:"message_timestamp" db:"message_timestamp"`
	SelectedProductKey *string   `json:"selected_product_key" db:"selected_product_key"`
	PayloadCiphertext  *string   `json:"-" db:"payload_ciphertext"`
	PayloadExpiresAt   time.Time `json:"payload_expires_at" db:"payload_expires_at"`
	ReasonCode         string    `json:"reason_code" db:"reason_code"`
	Status             string    `json:"status" db:"status"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

// MonitorOfferItem é a linha de monitor_offer_items: um link comercial.
type MonitorOfferItem struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	JobID             uuid.UUID  `json:"job_id" db:"job_id"`
	Ordinal           int16      `json:"ordinal" db:"ordinal"`
	ParagraphOrdinal  int16      `json:"paragraph_ordinal" db:"paragraph_ordinal"`
	SourceLinkHash    string     `json:"source_link_hash" db:"source_link_hash"`
	Host              string     `json:"host" db:"host"`
	CanonicalKey      *string    `json:"canonical_key" db:"canonical_key"`
	DuplicateOfItemID *uuid.UUID `json:"duplicate_of_item_id" db:"duplicate_of_item_id"`
	Store             string     `json:"store" db:"store"`
	URLClass          string     `json:"url_class" db:"url_class"`
	ErrorClass        string     `json:"error_class" db:"error_class"`
	ReasonCode        string     `json:"reason_code" db:"reason_code"`
	OutputKey         *string    `json:"output_key" db:"output_key"`
	OutputKind        *string    `json:"output_kind" db:"output_kind"`
	Outcome           string     `json:"outcome" db:"outcome"`
	Status            string     `json:"status" db:"status"`
	Attempts          int16      `json:"attempts" db:"attempts"`
	TotalAttempts     int        `json:"total_attempts" db:"total_attempts"`
	ReplayCount       int        `json:"replay_count" db:"replay_count"`
	EnqueueGeneration int        `json:"enqueue_generation" db:"enqueue_generation"`
	NextAttemptAt     *time.Time `json:"next_attempt_at" db:"next_attempt_at"`
	LeaseUntil        *time.Time `json:"lease_until" db:"lease_until"`
	AttemptStartedAt  *time.Time `json:"attempt_started_at" db:"attempt_started_at"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

// MonitorOfferDelivery é a linha de monitor_offer_deliveries: a autoridade de
// idempotência por destino. Nunca chamar a WAHA apoiado só em claim Redis.
type MonitorOfferDelivery struct {
	JobID              uuid.UUID  `json:"job_id" db:"job_id"`
	OutputKey          string     `json:"output_key" db:"output_key"`
	OutputKind         string     `json:"output_kind" db:"output_kind"`
	DestinationGroupID string     `json:"destination_group_id" db:"destination_group_id"`
	Status             string     `json:"status" db:"status"`
	AttemptToken       *uuid.UUID `json:"attempt_token" db:"attempt_token"`
	ProviderMessageID  *string    `json:"provider_message_id" db:"provider_message_id"`
	ReasonCode         string     `json:"reason_code" db:"reason_code"`
	SendingAt          *time.Time `json:"sending_at" db:"sending_at"`
	SentAt             *time.Time `json:"sent_at" db:"sent_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
}

// MonitorOfferPayloadVersion é a versão do envelope cifrado. Bump obrigatório
// se o formato mudar: o decrypt recusa versão desconhecida em vez de adivinhar.
const MonitorOfferPayloadVersion = 1

// MonitorOfferPayload é o conteúdo mínimo cifrado (AES-GCM) necessário para
// refazer classificação e render num retry.
//
// O que NÃO entra aqui é tão importante quanto o que entra: sender, push name,
// telefone, MediaURL, base64, título/descrição de preview e token ficam de
// fora por contrato (§7). BodyAfterStory conserva info/cta/desc porque o
// renderer do retry precisa delas; elas só somem quando o corpo público é
// construído.
type MonitorOfferPayload struct {
	Version          int       `json:"version"`
	BodyAfterStory   string    `json:"body_after_story"`
	FromMe           bool      `json:"from_me"`
	MessageTimestamp time.Time `json:"message_timestamp"`
	OrderedURLs      []string  `json:"ordered_urls"`
}

// MaxPayloadPlaintextBytes é o teto do payload ANTES de cifrar (§7).
// Estourar não trunca: o job vira blocked/payload_too_large, porque um payload
// truncado reclassificaria links diferentes dos que a mensagem realmente tinha.
const MaxPayloadPlaintextBytes = 16 * 1024

// PayloadTTL é a janela de retenção do ciphertext (§7).
const PayloadTTL = 6 * time.Hour

// TerminalMetadataTTL é a retenção da metadata terminal (§7).
const TerminalMetadataTTL = 30 * 24 * time.Hour

// IsTerminalItemStatus reporta se o item não recebe mais tentativa automática.
func IsTerminalItemStatus(s string) bool {
	switch s {
	case MonitorItemSucceeded, MonitorItemExhausted, MonitorItemBlocked,
		MonitorItemSuperseded, MonitorItemAmbiguous:
		return true
	}
	return false
}

// IsTerminalJobStatus reporta se o job não recebe mais trabalho automático.
func IsTerminalJobStatus(s string) bool {
	switch s {
	case MonitorJobSucceeded, MonitorJobExhausted, MonitorJobBlocked,
		MonitorJobSuperseded, MonitorJobAmbiguous:
		return true
	}
	return false
}

// IsReplayableBlock reporta se um item bloqueado pode ser reprocessado pelo
// comando de replay manual (§8.4). permanent_content, security_block e
// unsupported_commerce são não replayáveis por definição: reprocessar não muda
// o desfecho e, no caso de security_block, reabriria a superfície fechada.
func IsReplayableBlock(errorClass string) bool {
	return errorClass == ErrorClassPermanentConfig
}
