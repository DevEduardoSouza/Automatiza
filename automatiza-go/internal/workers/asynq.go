package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Task types
const (
	TypeSendMessage           = "whatsapp:send_message"
	TypeSyncGroups            = "whatsapp:sync_groups"
	TypeWAHAWebhook           = "whatsapp:waha_webhook"
	TypeReconcileChat         = "whatsapp:reconcile:chat"
	TypeReconcileSession      = "whatsapp:reconcile:session"
	TypeInstagramPublishStory = "instagram:publish_story"
	// TypeMonitoringDeferredDispatch executa UM item da fila de cadência do
	// espelho (Melhoria 7). A task é só um GATILHO: a verdade está no
	// PostgreSQL, então perder a fila do Redis não perde nenhum envio — o
	// poller reemite com geração nova.
	TypeMonitoringDeferredDispatch = "monitoring:deferred_dispatch"
)

// InstagramStoryJobData is the payload for publishing one Instagram story on
// behalf of a user. Exactly one of MediaKey / Text is set: MediaKey points to a
// JPEG already stored in the ephemeral media store (mirrored image); Text is
// the source message body to be rendered into an image by the worker. LedgerID
// is the instagram_story_posts row to update with the result.
type InstagramStoryJobData struct {
	UserID      string    `json:"userId"`
	MediaKey    string    `json:"mediaKey,omitempty"`
	Text        string    `json:"text,omitempty"`
	SourceGroup string    `json:"sourceGroup"`
	MessageID   string    `json:"messageId"`
	LedgerID    string    `json:"ledgerId"`
	TraceID     string    `json:"trace_id,omitempty"`
	EnqueuedAt  time.Time `json:"enqueuedAt"`
}

// DeferredDispatchJobData aponta para um item da fila de cadência. Repare no
// que NÃO está aqui: nada do conteúdo da mensagem, nenhum grupo, nenhuma
// mídia. O payload é uma referência + a prova de posse (geração + token); todo
// o resto é lido do banco no momento da execução, já revalidado.
//
// Generation/Token são o antídoto do ABA: se a lease expirou e o poller
// reemitiu, a task antiga chega com a geração velha, o CAS falha e ela vira
// no-op — em vez de despachar duas vezes.
type DeferredDispatchJobData struct {
	PendingID  int64     `json:"pendingId"`
	Generation int       `json:"generation"`
	Token      string    `json:"token"`
	TraceID    string    `json:"trace_id,omitempty"`
	EnqueuedAt time.Time `json:"enqueuedAt"`
}

// MessageJobData represents data for a send message job
type MessageJobData struct {
	InstanceName  string           `json:"instanceName"`
	GroupID       string           `json:"groupId"`
	Message       string           `json:"message"`
	Mentions      []string         `json:"mentions,omitempty"`
	HistoryID     *string          `json:"historyId,omitempty"`
	ImageURL      *string          `json:"imageUrl,omitempty"`
	ImageBase64   *string          `json:"imageBase64,omitempty"`
	DispatchLogID *int             `json:"dispatchLogId,omitempty"`
	AudioBase64   *string          `json:"audioBase64,omitempty"`
	Mimetype      *string          `json:"mimetype,omitempty"`
	PTT           bool             `json:"ptt,omitempty"`
	UserID        string           `json:"userId"`
	AutomationID  string           `json:"automationId,omitempty"`
	TraceID       string           `json:"trace_id,omitempty"`
	LinkPreview   *LinkPreviewData `json:"linkPreview,omitempty"`
	// HumanizeMode is the per-automation anti-ban preset (off|low|medium|high).
	// nil/empty => the global default ("high"). Consumed by the worker to scale
	// the presence choreography and per-recipient content variation.
	HumanizeMode *string `json:"humanizeMode,omitempty"`
	// ScheduledFor é o instante em que o envio DEVERIA acontecer (enqueue +
	// delay de pacing). O worker descarta a mensagem se pegá-la atrasada demais
	// (staleMessageTTL): depois de um apagão/backlog, promoções enfileiradas
	// horas antes saíam de madrugada, fora de hora e percebidas como duplicadas.
	// Zero (tasks antigas) => sem checagem.
	ScheduledFor time.Time `json:"scheduledFor,omitempty"`
}

// LinkPreviewData contains custom link preview metadata for WhatsApp messages.
type LinkPreviewData struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	ImageURL    string `json:"imageUrl,omitempty"`
}

// GroupsSyncJobData represents data for a groups sync job
type GroupsSyncJobData struct {
	UserID       string `json:"userId"`
	InstanceName string `json:"instanceName"`
	InstanceID   int    `json:"instanceId"`
	TraceID      string `json:"trace_id,omitempty"`
}

// WebhookJobData represents data for a WAHA webhook processing job
type WebhookJobData struct {
	EventID    string                 `json:"event_id"`
	Event      string                 `json:"event"`
	Session    string                 `json:"session"`
	Engine     string                 `json:"engine"`
	Payload    map[string]interface{} `json:"payload"`
	ReceivedAt int64                  `json:"received_at"`
	TraceID    string                 `json:"trace_id,omitempty"` // For distributed tracing
	// ProviderEventID/ProviderTimestamp são o ID (ULID) e o timestamp (ms) do
	// envelope WAHA, preservados como vieram. `omitempty` mantém a
	// compatibilidade com tasks enfileiradas por produtores anteriores: elas
	// desserializam com zero-value e o consumidor as trata como "sem ID".
	ProviderEventID   string `json:"provider_event_id,omitempty"`
	ProviderTimestamp int64  `json:"provider_timestamp,omitempty"`
}

// QueueClient wraps asynq client for enqueueing jobs
type QueueClient struct {
	client    *asynq.Client
	inspector *asynq.Inspector
	logger    *zap.Logger
	pacer     *sessionPacer
	rdb       *redis.Client // só quando o pacer está ligado (para Close)

	// Cache da profundidade da fila webhooks (ver WebhooksBacklogExceeds):
	// amostrada no máximo a cada 10s para não custar nada no caminho quente.
	depthMu        sync.Mutex
	depthSampledAt time.Time
	depthValue     int
}

// NewQueueClient creates a new queue client.
//
// Quando SESSION_PACE_MS > 0, liga o pacer por sessão: abre uma conexão Redis
// própria (mesmo endereço do asynq) e passa a espaçar os envios da mesma sessão
// do WhatsApp. Default desligado (0) — comportamento inalterado.
func NewQueueClient(redisAddr string, logger *zap.Logger) *QueueClient {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})

	q := &QueueClient{
		client:    client,
		inspector: asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr}),
		logger:    logger,
	}

	// Per-session pacer spacing: SESSION_PACE_MS takes precedence; otherwise
	// derive it from the (previously dead) WHATSAPP_SEND_LIMIT_PER_MINUTE so that
	// limit becomes a REAL smoothing per-chip cap — spacing = 60000/limit — that
	// pushes the schedule instead of dropping sends.
	ms := sessionPaceMsFromEnv()
	spacingSource := "SESSION_PACE_MS"
	if ms == 0 {
		if lpm := sendLimitPerMinuteFromEnv(); lpm > 0 {
			ms = 60000 / lpm
			spacingSource = "WHATSAPP_SEND_LIMIT_PER_MINUTE"
		}
	}
	if ms > 0 {
		q.rdb = redis.NewClient(&redis.Options{Addr: redisAddr})
		q.pacer = newSessionPacer(q.rdb, time.Duration(ms)*time.Millisecond, logger)
		logger.Info("session pacer enabled", zap.Int("spacingMs", ms), zap.String("source", spacingSource))
	}

	return q
}

// Close closes the queue client
func (q *QueueClient) Close() error {
	err := q.client.Close()
	if q.inspector != nil {
		if ierr := q.inspector.Close(); ierr != nil && err == nil {
			err = ierr
		}
	}
	if q.rdb != nil {
		if rerr := q.rdb.Close(); rerr != nil && err == nil {
			err = rerr
		}
	}
	return err
}

// WebhooksBacklogExceeds informa se pending+active da fila webhooks passou de
// max. Guarda anti-OOM da ingestão: no incidente de Redis-OOM a fila webhooks
// inchou sem limite (payload integral por task) até estourar o maxmemory e
// derrubar TODAS as escritas. Derramar explícito num teto alto é perda contida
// e visível; o OOM era apagão global e silencioso. Amostra a profundidade no
// máximo a cada 10s (cache) e é fail-open: qualquer erro => false (não dropa
// evento por falha de introspecção).
func (q *QueueClient) WebhooksBacklogExceeds(max int) bool {
	if q == nil || q.inspector == nil || max <= 0 {
		return false
	}

	q.depthMu.Lock()
	defer q.depthMu.Unlock()

	if time.Since(q.depthSampledAt) > 10*time.Second {
		info, err := q.inspector.GetQueueInfo("webhooks")
		if err != nil {
			q.depthSampledAt = time.Now()
			q.depthValue = 0
			return false
		}
		q.depthSampledAt = time.Now()
		q.depthValue = info.Pending + info.Active
	}

	return q.depthValue > max
}

// EnqueueMessage enqueues a message sending job
func (q *QueueClient) EnqueueMessage(ctx context.Context, data *MessageJobData, delay time.Duration) (*asynq.TaskInfo, error) {
	// OTel: propagate traceId from caller context for enqueue→worker correlation
	if data.TraceID == "" {
		if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
			data.TraceID = span.SpanContext().TraceID().String()
		}
	}

	// Pacer por sessão: empurra o agendamento para suavizar rajadas à mesma
	// sessão. Usa max() para não desfazer o espaçamento por grupo já calculado
	// pelo chamador — só atrasa além disso quando a sessão está saturada.
	// Fail-open: reserve() devolve 0 em qualquer erro.
	if q.pacer != nil {
		if paceDelay := q.pacer.reserve(ctx, data.InstanceName); paceDelay > delay {
			delay = paceDelay
		}
	}

	// Marca o horário-alvo do envio DEPOIS do pacer (delay final). O worker usa
	// isso pra descartar mensagem que ficou velha demais na fila.
	data.ScheduledFor = time.Now().Add(delay)

	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	opts := []asynq.Option{
		asynq.MaxRetry(3),
		// Fila "messages" (peso 3), NÃO "default": jobs de envio em massa
		// seguram worker por segundos (digitação simulada) e não podem
		// disputar de igual com o processamento de webhooks (espelho ≤1min).
		asynq.Queue("messages"),
		// 120s (was 60s) so a SendImage that hits the 30s httpClient timeout
		// still has budget for the adapter's single timeout-retry before the
		// task deadline cuts it — otherwise the retry that recovers a "sem
		// foto" upload would be truncated.
		asynq.Timeout(120 * time.Second),
	}

	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}

	// Use dispatch log ID as unique ID if available
	if data.DispatchLogID != nil {
		opts = append(opts, asynq.TaskID(formatTaskID(*data.DispatchLogID)))
	}

	task := asynq.NewTask(TypeSendMessage, payload, opts...)
	info, err := q.client.EnqueueContext(ctx, task)
	if err != nil {
		return nil, err
	}

	q.logger.Info("Message enqueued",
		zap.String("taskID", info.ID),
		zap.String("queue", info.Queue),
		zap.String("groupID", data.GroupID),
		zap.String("automationID", data.AutomationID),
		zap.Bool("hasImage", data.ImageURL != nil && *data.ImageURL != ""))

	return info, nil
}

// InstagramStoryTaskID deriva o id da task do EVENTO de origem (dono + grupo +
// mensagem), não de um id aleatório. Duas entregas do mesmo evento produzem o
// mesmo id e o asynq recusa a segunda — segunda barreira de idempotência, atrás
// do índice único do ledger.
//
// O hash não é enfeite: o id da task fica visível no Redis e no dashboard do
// asynq, e o material cru é UUID de cliente + JID de grupo + id de mensagem.
func InstagramStoryTaskID(userID, sourceGroup, messageID string) string {
	sum := sha256.Sum256([]byte(userID + "\x00" + sourceGroup + "\x00" + messageID))
	return "ig-story:" + hex.EncodeToString(sum[:])
}

// MonitorDeferredTaskID inclui a GERAÇÃO: reemissões geram IDs diferentes, então
// uma task nova nunca é engolida como duplicata da antiga — e a antiga, se
// ressuscitar, não consegue mexer no item (o CAS de geração a barra).
func MonitorDeferredTaskID(pendingID int64, generation int) string {
	return fmt.Sprintf("monpending:%d:%d", pendingID, generation)
}

// EnqueueMonitorDeferredDispatch enfileira o gatilho de um item da cadência.
//
// MaxRetry(0) é deliberado: retry de task não é o mecanismo de recuperação
// desta feature — o ledger é. Se a task morrer, a lease expira e o poller a
// reemite com geração nova, com o estado do item revalidado no banco. Retry
// cego do asynq reexecutaria com uma geração que já pode ter sido superada.
//
// Conflito de TaskID devolve (nil, nil): o gatilho já existe, o item está
// atendido.
func (q *QueueClient) EnqueueMonitorDeferredDispatch(ctx context.Context, pendingID int64, generation int, token string) error {
	data := &DeferredDispatchJobData{
		PendingID:  pendingID,
		Generation: generation,
		Token:      token,
		EnqueuedAt: time.Now(),
	}
	if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
		data.TraceID = span.SpanContext().TraceID().String()
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	task := asynq.NewTask(TypeMonitoringDeferredDispatch, payload,
		asynq.Queue("messages"),
		asynq.MaxRetry(0),
		asynq.Timeout(5*time.Minute),
		asynq.Retention(24*time.Hour),
		asynq.TaskID(MonitorDeferredTaskID(pendingID, generation)),
	)
	if _, err := q.client.EnqueueContext(ctx, task); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
			return nil
		}
		return err
	}
	return nil
}

// EnqueueInstagramStory enqueues a story-publish job on the "default" queue.
// Story publishing has no ≤1min SLA, so it must not compete with webhooks(4)
// or the mirror's own message sends(3).
//
// Conflito de TaskID devolve (nil, nil): o job JÁ está na fila, então o evento
// está atendido. Tratar isso como erro faria o chamador marcar o ledger como
// falho enquanto a publicação seguia normalmente.
func (q *QueueClient) EnqueueInstagramStory(ctx context.Context, data *InstagramStoryJobData) (*asynq.TaskInfo, error) {
	if data.TraceID == "" {
		if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
			data.TraceID = span.SpanContext().TraceID().String()
		}
	}
	data.EnqueuedAt = time.Now()

	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	task := asynq.NewTask(TypeInstagramPublishStory, payload,
		asynq.Queue("default"),
		asynq.MaxRetry(4),
		asynq.Timeout(90*time.Second),
		asynq.TaskID(InstagramStoryTaskID(data.UserID, data.SourceGroup, data.MessageID)),
	)
	info, err := q.client.EnqueueContext(ctx, task)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
			q.logger.Info("Instagram story already enqueued (idempotent)",
				zap.String("userID", data.UserID))
			return nil, nil
		}
		return nil, err
	}
	q.logger.Info("Instagram story enqueued",
		zap.String("taskID", info.ID),
		zap.String("userID", data.UserID),
		zap.String("sourceGroup", data.SourceGroup),
		zap.Bool("rendered", data.MediaKey == ""))
	return info, nil
}

// EnqueueGroupsSync enqueues a groups sync job
func (q *QueueClient) EnqueueGroupsSync(ctx context.Context, data *GroupsSyncJobData) (*asynq.TaskInfo, error) {
	// OTel: propagate traceId from caller context for enqueue→worker correlation
	if data.TraceID == "" {
		if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
			data.TraceID = span.SpanContext().TraceID().String()
		}
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	task := asynq.NewTask(TypeSyncGroups, payload,
		asynq.MaxRetry(2),
		asynq.Queue("default"),
		asynq.Timeout(5*time.Minute),
	)

	return q.client.EnqueueContext(ctx, task)
}

// EnqueueWebhook enqueues a WAHA webhook processing job
func (q *QueueClient) EnqueueWebhook(ctx context.Context, data *WebhookJobData) (*asynq.TaskInfo, error) {
	// OTel: propagate traceId from caller context for enqueue→worker correlation
	if data.TraceID == "" {
		if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
			data.TraceID = span.SpanContext().TraceID().String()
		}
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// SEM asynq.Deadline: o deadline absoluto de 5min matava TODO o backlog em
	// massa (worker parado num deploy travado, fila cheia pós-incidente) — as
	// tasks acordavam com ctx já expirado, falhavam sem executar e o evento
	// morria (session.status/group perdidos, webhook_events preso em 'received';
	// ~195k eventos perdidos na madrugada de 14/07). O frescor de MENSAGEM de
	// espelho é tratado no consumidor via ReceivedAt (staleWebhookMessageTTL);
	// eventos de status/grupo não perdem valor com o tempo e sempre processam.
	task := asynq.NewTask(TypeWAHAWebhook, payload,
		// MaxRetry cobre o budget dedicado de backpressure do espelho
		// (maxBackpressureRetries no webhook_consumer_worker): sem entrega
		// suficiente do asynq, o retry da classe "mirror fan-out busy" pararia
		// antes da janela de frescor de 10min e a oferta cairia na DLQ. Só a
		// classe busy usa essas tentativas extras — as demais param no gate do
		// app (MaxWebhookRetries=3), então elevar o teto não muda o resto.
		asynq.MaxRetry(15),
		asynq.Queue("webhooks"),
		asynq.Timeout(120*time.Second),
		asynq.TaskID("waha:"+data.EventID), // Use event ID for deduplication
	)

	return q.client.EnqueueContext(ctx, task)
}

// IsDuplicateTaskError checks if the error is a duplicate task error
func IsDuplicateTaskError(err error) bool {
	return errors.Is(err, asynq.ErrDuplicateTask) || errors.Is(err, asynq.ErrTaskIDConflict)
}

// EnqueueReconcileChat enqueues a single chat reconciliation task.
// The task will be processed by the ReconciliationWorker.ProcessChat handler.
//
// Parameters:
//   - sessionName: The WAHA session identifier
//   - chatID: The WhatsApp chat JID to reconcile
func (q *QueueClient) EnqueueReconcileChat(ctx context.Context, sessionName, chatID string) (*asynq.TaskInfo, error) {
	payload, err := json.Marshal(ReconcileChatPayload{
		SessionName: sessionName,
		ChatID:      chatID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reconcile chat payload: %w", err)
	}

	task := asynq.NewTask(TypeReconcileChat, payload,
		asynq.MaxRetry(3),
		asynq.Queue("reconciliation"),
		asynq.Timeout(5*time.Minute),
		// Use session:chat as unique ID to prevent duplicate tasks
		asynq.TaskID("reconcile:chat:"+sessionName+":"+chatID),
	)

	info, err := q.client.EnqueueContext(ctx, task)
	if err != nil {
		if IsDuplicateTaskError(err) {
			q.logger.Debug("Reconcile chat task already queued",
				zap.String("session", sessionName),
				zap.String("chat", chatID))
			return nil, nil
		}
		return nil, fmt.Errorf("failed to enqueue reconcile chat task: %w", err)
	}

	q.logger.Info("Enqueued reconcile chat task",
		zap.String("session", sessionName),
		zap.String("chat", chatID),
		zap.String("taskID", info.ID))

	return info, nil
}

// EnqueueReconcileSession enqueues a full session reconciliation task.
// The task will reconcile all chats for the given session.
//
// Parameters:
//   - sessionName: The WAHA session identifier to reconcile
func (q *QueueClient) EnqueueReconcileSession(ctx context.Context, sessionName string) (*asynq.TaskInfo, error) {
	payload, err := json.Marshal(ReconcileSessionPayload{
		SessionName: sessionName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reconcile session payload: %w", err)
	}

	task := asynq.NewTask(TypeReconcileSession, payload,
		asynq.MaxRetry(3),
		asynq.Queue("reconciliation"),
		asynq.Timeout(10*time.Minute),
		// Use session as unique ID to prevent duplicate tasks
		asynq.TaskID("reconcile:session:"+sessionName),
	)

	info, err := q.client.EnqueueContext(ctx, task)
	if err != nil {
		if IsDuplicateTaskError(err) {
			q.logger.Debug("Reconcile session task already queued",
				zap.String("session", sessionName))
			return nil, nil
		}
		return nil, fmt.Errorf("failed to enqueue reconcile session task: %w", err)
	}

	q.logger.Info("Enqueued reconcile session task",
		zap.String("session", sessionName),
		zap.String("taskID", info.ID))

	return info, nil
}

// ReconcileChatPayload is the payload for single chat reconciliation tasks.
type ReconcileChatPayload struct {
	SessionName string `json:"session_name"`
	ChatID      string `json:"chat_id"`
}

// ReconcileSessionPayload is the payload for full session reconciliation tasks.
type ReconcileSessionPayload struct {
	SessionName string `json:"session_name"`
}

// QueueServer wraps asynq server for processing jobs
type QueueServer struct {
	server *asynq.Server
	mux    *asynq.ServeMux
	logger *zap.Logger
}

// NewQueueServer creates a new queue server
func NewQueueServer(redisAddr string, concurrency int, logger *zap.Logger) *QueueServer {
	if concurrency <= 0 {
		concurrency = 5
	}

	server := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: concurrency,
			// Default de 8s requeuava envios em voo a cada deploy: o task que já
			// tinha POSTado ao WAHA (mensagem entregue) mas não retornado em 8s
			// era re-executado pelo worker novo = promoção duplicada no grupo.
			// 130s > Timeout(120s) da task: o envio termina (ou vira
			// timeout-assumed-sent) ANTES do requeue. Exige stop_grace_period
			// compatível no container do worker.
			ShutdownTimeout: 130 * time.Second,
			Queues: map[string]int{
				"critical":       6,
				"webhooks":       4,
				"messages":       3, // Message queue for text/audio/image/poll
				"default":        3,
				"reconciliation": 2,
				"low":            1,
				"dlq":            1, // Dead letter queue - lowest priority
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// Exponential backoff with jitter: n² seconds + random jitter (0-30%)
				// This prevents thundering herd in distributed scenarios
				baseDelay := time.Duration(n*n) * time.Second
				// Add jitter: random 0-30% of base delay
				jitter := time.Duration(float64(baseDelay) * 0.3 * mathrand.Float64())
				return baseDelay + jitter
			},
			Logger: &asynqLogger{logger: logger},
		},
	)

	return &QueueServer{
		server: server,
		mux:    asynq.NewServeMux(),
		logger: logger,
	}
}

// RegisterHandler registers a handler for a task type
func (s *QueueServer) RegisterHandler(taskType string, handler asynq.Handler) {
	s.mux.Handle(taskType, handler)
}

// Start starts the queue server
func (s *QueueServer) Start() error {
	return s.server.Start(s.mux)
}

// Shutdown gracefully shuts down the queue server
func (s *QueueServer) Shutdown() {
	s.server.Shutdown()
}

// asynqLogger adapts zap logger for asynq
type asynqLogger struct {
	logger *zap.Logger
}

func (l *asynqLogger) Debug(args ...interface{}) {
	l.logger.Debug("asynq", zap.Any("args", args))
}

func (l *asynqLogger) Info(args ...interface{}) {
	l.logger.Info("asynq", zap.Any("args", args))
}

func (l *asynqLogger) Warn(args ...interface{}) {
	l.logger.Warn("asynq", zap.Any("args", args))
}

func (l *asynqLogger) Error(args ...interface{}) {
	l.logger.Error("asynq", zap.Any("args", args))
}

func (l *asynqLogger) Fatal(args ...interface{}) {
	l.logger.Fatal("asynq", zap.Any("args", args))
}

func formatTaskID(dispatchLogID int) string {
	return "dispatch:" + strconv.Itoa(dispatchLogID)
}
