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
	ID             string     `json:"id" db:"id"`
	UserID         string     `json:"user_id" db:"user_id"`
	EncryptedData  string     `json:"encrypted_data" db:"encrypted_data"`
	IGUserID       string     `json:"ig_user_id" db:"ig_user_id"`
	Username       string     `json:"username" db:"username"`
	TokenExpiresAt *time.Time `json:"token_expires_at" db:"token_expires_at"`
	CreatedAt      string     `json:"created_at" db:"created_at"`
	UpdatedAt      string     `json:"updated_at" db:"updated_at"`
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
}
