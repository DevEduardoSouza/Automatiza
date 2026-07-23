package models

import "time"

// InstagramTokens is the plaintext payload cifrado dentro de
// instagram_credentials.encrypted_data (EncryptedBlobV1). Só o token de longa
// duração é sensível; ig_user_id/username também ficam em colunas claras para
// o cron de refresh não precisar decriptar para listar.
type InstagramTokens struct {
	AccessToken string `json:"accessToken"`
}

// InstagramCredentialsDB espelha a linha da tabela instagram_credentials.
type InstagramCredentialsDB struct {
	ID            string `json:"id" db:"id"`
	UserID        string `json:"user_id" db:"user_id"`
	EncryptedData string `json:"encrypted_data" db:"encrypted_data"`
	// AppScopedUserID é o identificador APP-SCOPED devolvido pela troca do
	// authorization code. NÃO é o id usado para publicar (esse é IGUserID, vindo
	// do /me) — serve para casar os callbacks de desautorização/exclusão da Meta,
	// cujo signed_request só carrega este id. Nunca serializado: é um
	// identificador da Meta e não tem uso no frontend.
	AppScopedUserID string     `json:"-" db:"app_scoped_user_id"`
	IGUserID        string     `json:"ig_user_id" db:"ig_user_id"`
	Username        string     `json:"username" db:"username"`
	TokenExpiresAt  *time.Time `json:"token_expires_at" db:"token_expires_at"`
	CreatedAt       string     `json:"created_at" db:"created_at"`
	UpdatedAt       string     `json:"updated_at" db:"updated_at"`
}

// Instagram story media sources.
const (
	InstagramMediaSourceImage        = "image"
	InstagramMediaSourceRenderedText = "rendered_text"
)

// Instagram story post statuses.
const (
	InstagramStoryStatusPending   = "pending"
	InstagramStoryStatusPublished = "published"
	InstagramStoryStatusFailed    = "failed"
)

// InstagramStoryPost é uma linha do ledger instagram_story_posts.
//
// Os estados públicos continuam sendo três (pending/published/failed). "Em
// processamento" NÃO é um quarto estado: é o lease ProcessingStartedAt sobre uma
// linha ainda pending — assim um worker morto não deixa a linha num estado
// intermediário que ninguém mais pode reivindicar.
type InstagramStoryPost struct {
	ID          string `json:"id" db:"id"`
	UserID      string `json:"user_id" db:"user_id"`
	SourceGroup string `json:"source_group" db:"source_group"`
	MessageID   string `json:"message_id" db:"message_id"`
	MediaSource string `json:"media_source" db:"media_source"`
	Status      string `json:"status" db:"status"`
	IGMediaID   string `json:"ig_media_id" db:"ig_media_id"`
	Error       string `json:"error" db:"error"`
	CreatedAt   string `json:"created_at" db:"created_at"`
	UpdatedAt   string `json:"updated_at" db:"updated_at"`

	// Campos de controle interno — nenhum vai para o frontend.
	ContainerID         string     `json:"-" db:"container_id"`
	ProcessingStartedAt *time.Time `json:"-" db:"processing_started_at"`
	AttemptCount        int        `json:"-" db:"attempt_count"`
	ErrorCode           string     `json:"-" db:"error_code"`
	PublishedReconciled bool       `json:"-" db:"published_reconciled"`
}

// Códigos de erro SEGUROS do ledger. O banco só recebe um destes valores
// enumerados; err.Error() nunca é concatenado ali, porque o erro cru do
// provedor carrega URL, ids e às vezes trechos do conteúdo. O detalhe técnico
// vive apenas no log estruturado.
const (
	InstagramErrFeatureDisabled    = "feature_disabled"
	InstagramErrInvalidPayload     = "invalid_payload"
	InstagramErrMediaExpired       = "media_expired"
	InstagramErrMediaInvalid       = "media_invalid"
	InstagramErrMediaTooLarge      = "media_too_large"
	InstagramErrQuotaLocal         = "quota_local"
	InstagramErrQuotaMeta          = "quota_meta"
	InstagramErrAuthRequired       = "auth_required"
	InstagramErrTokenExpired       = "token_expired"
	InstagramErrContainerError     = "container_error"
	InstagramErrContainerExpired   = "container_expired"
	InstagramErrPublishRejected    = "publish_rejected"
	InstagramErrEnqueueFailed      = "enqueue_failed"
	InstagramErrTransientExhausted = "transient_exhausted"
)

// InstagramSafeErrorMessage traduz um código seguro para a frase estática
// mostrada ao usuário. Retorna "" para código desconhecido — o chamador nunca
// deve inventar texto a partir do erro real.
func InstagramSafeErrorMessage(code string) string {
	switch code {
	case InstagramErrFeatureDisabled:
		return "publicação de Stories indisponível no momento"
	case InstagramErrInvalidPayload:
		return "solicitação inválida"
	case InstagramErrMediaExpired:
		return "mídia expirou antes da publicação"
	case InstagramErrMediaInvalid:
		return "imagem não pôde ser preparada"
	case InstagramErrMediaTooLarge:
		return "imagem excede o tamanho permitido"
	case InstagramErrQuotaLocal, InstagramErrQuotaMeta:
		return "limite diário de stories atingido"
	case InstagramErrAuthRequired, InstagramErrTokenExpired:
		return "conta Instagram precisa reconectar"
	case InstagramErrContainerError, InstagramErrPublishRejected:
		return "Instagram recusou a publicação"
	case InstagramErrContainerExpired:
		return "a publicação expirou antes de concluir"
	case InstagramErrEnqueueFailed:
		return "não foi possível agendar a publicação"
	case InstagramErrTransientExhausted:
		return "falha temporária persistiu após novas tentativas"
	default:
		return ""
	}
}

// InstagramDataDeletionRequest é o comprovante de exclusão exigido pela Meta.
// Nenhum campo é serializado: a resposta pública é montada explicitamente com
// url + confirmation_code, para não vazar subject_hash ou timestamps internos.
type InstagramDataDeletionRequest struct {
	ID               string    `json:"-" db:"id"`
	SubjectHash      string    `json:"-" db:"subject_hash"`
	ConfirmationCode string    `json:"-" db:"confirmation_code"`
	Status           string    `json:"-" db:"status"`
	RequestedAt      time.Time `json:"-" db:"requested_at"`
	CompletedAt      time.Time `json:"-" db:"completed_at"`
	ExpiresAt        time.Time `json:"-" db:"expires_at"`
}

// InstagramAppSecret é o payload cifrado dentro de
// instagram_apps.encrypted_app_secret (EncryptedBlobV1, mesmo cofre AES usado
// para o token do usuário).
type InstagramAppSecret struct {
	AppSecret string `json:"appSecret"`
}

// InstagramApp é o app da Meta que o PRÓPRIO cliente registrou.
//
// Existe porque a Meta isenta de App Review o app criado para o negócio do
// próprio dono (Standard Access). Com um app nosso atendendo conta de terceiro
// seria Advanced Access + análise da Meta — semanas, e sujeito a recusa.
//
// EncryptedAppSecret nunca é serializado e nunca volta ao frontend: depois de
// salvo, a UI só informa que existe.
type InstagramApp struct {
	ID     string `json:"id" db:"id"`
	UserID string `json:"user_id" db:"user_id"`
	AppID  string `json:"app_id" db:"app_id"`
	// EncryptedAppSecret guarda o blob AES; o valor em claro só existe em
	// memória, no momento da chamada à Meta.
	EncryptedAppSecret string `json:"-" db:"encrypted_app_secret"`
	// WebhookToken identifica o cliente pela URL, ANTES de ler o corpo do
	// webhook — é o que resolve "qual app secret valida esta assinatura".
	WebhookToken string `json:"-" db:"webhook_token"`
	// VerifyToken é o segredo do handshake GET (hub.verify_token) da Meta.
	VerifyToken string     `json:"-" db:"verify_token"`
	IsVerified  bool       `json:"is_verified" db:"is_verified"`
	VerifiedAt  *time.Time `json:"verified_at,omitempty" db:"verified_at"`
	CreatedAt   string     `json:"created_at" db:"created_at"`
	UpdatedAt   string     `json:"updated_at" db:"updated_at"`
}
