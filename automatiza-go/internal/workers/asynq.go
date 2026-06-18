package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand/v2"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Task types
const (
	TypeSendMessage      = "whatsapp:send_message"
	TypeSyncGroups       = "whatsapp:sync_groups"
	TypeWAHAWebhook      = "whatsapp:waha_webhook"
	TypeReconcileChat    = "whatsapp:reconcile:chat"
	TypeReconcileSession = "whatsapp:reconcile:session"
)

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
}

// QueueClient wraps asynq client for enqueueing jobs
type QueueClient struct {
	client *asynq.Client
	logger *zap.Logger
	pacer  *sessionPacer
	rdb    *redis.Client // só quando o pacer está ligado (para Close)
}

// NewQueueClient creates a new queue client.
//
// Quando SESSION_PACE_MS > 0, liga o pacer por sessão: abre uma conexão Redis
// própria (mesmo endereço do asynq) e passa a espaçar os envios da mesma sessão
// do WhatsApp. Default desligado (0) — comportamento inalterado.
func NewQueueClient(redisAddr string, logger *zap.Logger) *QueueClient {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})

	q := &QueueClient{
		client: client,
		logger: logger,
	}

	if ms := sessionPaceMsFromEnv(); ms > 0 {
		q.rdb = redis.NewClient(&redis.Options{Addr: redisAddr})
		q.pacer = newSessionPacer(q.rdb, time.Duration(ms)*time.Millisecond, logger)
		logger.Info("session pacer enabled", zap.Int("spacingMs", ms))
	}

	return q
}

// Close closes the queue client
func (q *QueueClient) Close() error {
	err := q.client.Close()
	if q.rdb != nil {
		if rerr := q.rdb.Close(); rerr != nil && err == nil {
			err = rerr
		}
	}
	return err
}

// EnqueueMessage enqueues a message sending job
func (q *QueueClient) EnqueueMessage(ctx context.Context, data *MessageJobData, delay time.Duration) (*asynq.TaskInfo, error) {
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

	// Pacer por sessão: empurra o agendamento para suavizar rajadas à mesma
	// sessão. Usa max() para não desfazer o espaçamento por grupo já calculado
	// pelo chamador — só atrasa além disso quando a sessão está saturada.
	// Fail-open: reserve() devolve 0 em qualquer erro.
	if q.pacer != nil {
		if paceDelay := q.pacer.reserve(ctx, data.InstanceName); paceDelay > delay {
			delay = paceDelay
		}
	}

	opts := []asynq.Option{
		asynq.MaxRetry(3),
		asynq.Queue("default"),
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

	task := asynq.NewTask(TypeWAHAWebhook, payload,
		asynq.MaxRetry(5),
		asynq.Queue("webhooks"),
		asynq.Timeout(120*time.Second),
		asynq.TaskID("waha:"+data.EventID), // Use event ID for deduplication
		asynq.Deadline(time.Now().Add(5*time.Minute)),
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
