package models

import (
	"errors"
	"time"
)

// Cadência durável do modo monitoramento (Melhoria 7, migração 078).
//
// Contrato em uma frase: uma mensagem aceita vira LINHA no PostgreSQL antes de
// qualquer efeito colateral, e nenhuma etapa posterior (fila, Redis, deploy,
// crash) pode fazê-la sumir nem sair duas vezes no mesmo grupo.

// Estados de um item da fila (automation_monitor_pending.status).
const (
	MonitorPendingPending    = "pending"
	MonitorPendingEnqueuing  = "enqueuing"
	MonitorPendingEnqueued   = "enqueued"
	MonitorPendingProcessing = "processing"
	MonitorPendingCompleted  = "completed"
	MonitorPendingFailed     = "failed"
	MonitorPendingCanceled   = "canceled"
)

// Estados de um destino (automation_monitor_pending_destinations.status).
const (
	MonitorDestinationPending = "pending"
	MonitorDestinationClaimed = "claimed"
	// Sending é a fronteira do resultado incerto: marcada ANTES do POST. Um
	// destino que morre aqui vira assumed_sent e nunca é reenviado.
	MonitorDestinationSending     = "sending"
	MonitorDestinationSent        = "sent"
	MonitorDestinationAssumedSent = "assumed_sent"
	MonitorDestinationSkipped     = "skipped"
	MonitorDestinationFailed      = "failed"
	MonitorDestinationCanceled    = "canceled"
)

// Códigos fechados persistidos em last_error_code / outcome_code. Erro bruto do
// provedor NUNCA é gravado: pode carregar URL, nome de grupo ou corpo.
const (
	MonitorCodeBacklogFull        = "monitor_cadence_backlog_full"
	MonitorCodeStorageUnavailable = "monitor_cadence_storage_unavailable"
	MonitorCodeMediaUnavailable   = "monitor_cadence_media_unavailable"
	MonitorCodeMediaTooLarge      = "monitor_cadence_media_too_large"
	MonitorCodeInvalidWindow      = "invalid_monitor_window"
	MonitorCodeWindowClosed       = "window_closed"
	MonitorCodePrepareFailed      = "prepare_failed"
	MonitorCodeAutomationGone     = "automation_gone"
	MonitorCodeSendFailed         = "send_failed"
	MonitorCodeSendAmbiguous      = "send_ambiguous"
	MonitorCodeDedupe             = "dedupe"
	MonitorCodeNotParticipating   = "not_participating"
	MonitorCodeReputation         = "reputation"
	MonitorCodeLeaseExpired       = "lease_expired"
)

// Limites fechados da feature (§6 do plano). Constantes, não config: cada um
// tem um motivo de segurança e mudar por env transformaria o contrato de
// entrega em algo que ninguém consegue auditar depois.
const (
	// MonitorMaxIntervalMinutes é o teto do intervalo configurável (24h).
	MonitorMaxIntervalMinutes = 1440
	// MonitorMaxBacklogPerAutomation: acima disso a admissão falha inteira em
	// vez de acumular fila infinita de oferta velha.
	MonitorMaxBacklogPerAutomation = 200
	// MonitorMaxMediaBytes: teto dos bytes persistidos por mensagem-fonte.
	MonitorMaxMediaBytes = 15 * 1024 * 1024
	// MonitorMaxDestinationAttempts: tentativas definitivas por destino.
	MonitorMaxDestinationAttempts = 5
	// MonitorContentDedupWindow espelha a janela de dedup de conteúdo do
	// caminho vivo (claimOnce de 2 min), agora resolvida no banco.
	MonitorContentDedupWindow = 2 * time.Minute
	// MonitorRetentionDays: retenção de itens terminais.
	MonitorRetentionDays = 30
)

// ErrMonitorBacklogFull aborta a admissão inteira (nenhum pending é criado)
// quando uma automação já acumulou MonitorMaxBacklogPerAutomation itens.
var ErrMonitorBacklogFull = errors.New(MonitorCodeBacklogFull)

// MonitorWindowSnapshot congela a janela de disparo no momento da admissão.
// Editar a automação depois NÃO reescreve reservas já feitas — só o
// not_before_at do head pode ser adiado por uma janela que fechou.
type MonitorWindowSnapshot struct {
	StartTime string   `json:"start_time,omitempty"`
	EndTime   string   `json:"end_time,omitempty"`
	Days      []string `json:"days,omitempty"`
}

// MonitorMessageSnapshot é a cópia sanitizada da mensagem-fonte. Contém apenas
// o necessário para reconstruir o disparo; nunca media.url, media.s3, base64
// ou o payload WAHA bruto (URL de mídia expira e não é referência durável).
type MonitorMessageSnapshot struct {
	InstanceName           string `json:"instance_name"`
	MessageID              string `json:"message_id"`
	RemoteJID              string `json:"remote_jid"`
	SenderJID              string `json:"sender_jid,omitempty"`
	PushName               string `json:"push_name,omitempty"`
	Body                   string `json:"body"`
	Timestamp              int64  `json:"timestamp"`
	HasMedia               bool   `json:"has_media,omitempty"`
	MediaType              string `json:"media_type,omitempty"`
	MimeType               string `json:"mime_type,omitempty"`
	LinkPreviewTitle       string `json:"link_preview_title,omitempty"`
	LinkPreviewDescription string `json:"link_preview_description,omitempty"`
}

// ToIncomingMessage reconstrói a mensagem para o fan-out diferido. Os campos de
// mídia voláteis ficam vazios de propósito: os bytes vêm do payload persistido.
func (s *MonitorMessageSnapshot) ToIncomingMessage() *IncomingMessage {
	return &IncomingMessage{
		InstanceName:           s.InstanceName,
		MessageID:              s.MessageID,
		RemoteJID:              s.RemoteJID,
		SenderJID:              s.SenderJID,
		PushName:               s.PushName,
		Body:                   s.Body,
		Timestamp:              s.Timestamp,
		HasMedia:               s.HasMedia,
		MediaType:              s.MediaType,
		MimeType:               s.MimeType,
		LinkPreviewTitle:       s.LinkPreviewTitle,
		LinkPreviewDescription: s.LinkPreviewDescription,
	}
}

// NewMonitorMessageSnapshot extrai o snapshot sanitizado de uma mensagem viva.
func NewMonitorMessageSnapshot(msg *IncomingMessage) MonitorMessageSnapshot {
	return MonitorMessageSnapshot{
		InstanceName:           msg.InstanceName,
		MessageID:              msg.MessageID,
		RemoteJID:              msg.RemoteJID,
		SenderJID:              msg.SenderJID,
		PushName:               msg.PushName,
		Body:                   msg.Body,
		Timestamp:              msg.Timestamp,
		HasMedia:               msg.HasMedia,
		MediaType:              msg.MediaType,
		MimeType:               msg.MimeType,
		LinkPreviewTitle:       msg.LinkPreviewTitle,
		LinkPreviewDescription: msg.LinkPreviewDescription,
	}
}

// MonitorPayloadMedia são os bytes duráveis da mídia da mensagem-fonte.
type MonitorPayloadMedia struct {
	Mimetype  string
	Filename  string
	SHA256    string
	SizeBytes int
	Bytes     []byte
}

// MonitorAdmissionAutomation descreve uma automação candidata à admissão.
type MonitorAdmissionAutomation struct {
	ID              string
	IntervalMinutes int
	DelaySeconds    int
	Window          MonitorWindowSnapshot
}

// MonitorAdmissionInput é a entrada de uma admissão transacional.
type MonitorAdmissionInput struct {
	UserID            string
	SourceSessionName string
	SourceChatID      string
	SourceMessageID   string
	ContentSHA256     string
	ReceivedAt        time.Time
	MessageSnapshot   MonitorMessageSnapshot
	Media             *MonitorPayloadMedia
	Automations       []MonitorAdmissionAutomation
	// Schedule resolve o primeiro instante permitido a partir de `base` para a
	// janela dada. Função PURA (sem I/O), avaliada dentro da transação com o
	// relógio do PostgreSQL — o relógio da aplicação nunca decide horário aqui.
	Schedule func(base time.Time, window MonitorWindowSnapshot) (time.Time, error)
}

// MonitorAdmissionResult diz o que fazer com cada automação candidata.
type MonitorAdmissionResult struct {
	// Deferred: automação que ficou com um item na fila (nova ou redelivery).
	// O caminho imediato NÃO deve processá-la para este evento.
	Deferred map[string]int64
	// Suppressed: conteúdo idêntico já aceito na janela de dedup. Nem fila nem
	// envio imediato.
	Suppressed map[string]struct{}
	// Redelivery indica que o payload já existia (webhook repetido): nenhum
	// slot foi consumido.
	Redelivery bool
}

// MonitorPendingLease é o resultado do claim do poller sobre o head da fila.
type MonitorPendingLease struct {
	PendingID       int64
	AutomationID    string
	EnqueueGen      int
	EnqueueToken    string
	SequenceNo      int64
	IntervalMinutes int
}

// MonitorPendingWork é o item reivindicado pelo worker, já com o payload.
type MonitorPendingWork struct {
	PendingID          int64
	AutomationID       string
	AutomationSnapshot string
	UserID             string
	PayloadID          int64
	SequenceNo         int64
	IntervalMinutes    int
	DelaySeconds       int
	Window             MonitorWindowSnapshot
	ProcessingToken    string
	ProcessingAttempts int
	FirstStarted       bool
	Message            MonitorMessageSnapshot
	Media              *MonitorPayloadMedia
}

// MonitorPendingDestination é uma linha do ledger por grupo de destino.
type MonitorPendingDestination struct {
	ID           int64
	PendingID    int64
	Ordinal      int
	GroupID      string
	CanonicalKey string
	Snapshot     []byte
	Status       string
	Attempts     int
	NotBeforeAt  time.Time
	OutcomeCode  string
}

// IsTerminal informa se o destino já tem desfecho definitivo.
func (d *MonitorPendingDestination) IsTerminal() bool {
	switch d.Status {
	case MonitorDestinationSent, MonitorDestinationAssumedSent,
		MonitorDestinationSkipped, MonitorDestinationFailed, MonitorDestinationCanceled:
		return true
	}
	return false
}

// MonitorPendingResult são os contadores gravados no fechamento do item.
// Só números e códigos — nenhum ID externo, grupo ou texto.
type MonitorPendingResult struct {
	SentCount        int `json:"sent_count"`
	AssumedSentCount int `json:"assumed_sent_count"`
	SkippedCount     int `json:"skipped_count"`
	FailedCount      int `json:"failed_count"`
	CanceledCount    int `json:"canceled_count"`
}

// MonitorRecoveryStats conta o que o tick de manutenção recuperou.
type MonitorRecoveryStats struct {
	EnqueueLeasesReleased int
	DestinationsReleased  int
	DestinationsAssumed   int
	PendingsRecovered     int
	PendingsCanceled      int
	MediaPurged           int
}

// MonitorQueueSummary alimenta a UI (contagem + próximo horário do head).
type MonitorQueueSummary struct {
	PendingCount    int
	NextScheduledAt *time.Time
}
