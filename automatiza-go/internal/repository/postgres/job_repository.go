package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type jobRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewJobRepository creates a new PostgreSQL job repository for standalone jobs
func NewJobRepository(db *DB, logger *zap.Logger) repository.JobRepository {
	return &jobRepository{
		db:     db,
		logger: logger,
	}
}

const getJobByIDQuery = `
	SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
	FROM jobs
	WHERE id = $1
`

func (r *jobRepository) GetByID(ctx context.Context, id string) (*models.StandaloneJob, error) {
	var job models.StandaloneJob

	err := r.db.QueryRow(ctx, getJobByIDQuery, id).Scan(
		&job.ID,
		&job.UserID,
		&job.ActionType,
		&job.ScheduledFor,
		&job.Status,
		&job.Payload,
		&job.ErrorMessage,
		&job.RetryCount,
		&job.MaxRetries,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get job by ID",
			zap.String("jobID", id),
			zap.Error(err))
		return nil, err
	}

	return &job, nil
}

func (r *jobRepository) GetByUserID(ctx context.Context, userID string, status *models.StandaloneJobStatus, limit int) ([]models.StandaloneJob, error) {
	if limit <= 0 {
		limit = 50
	}

	var query string
	var args []interface{}

	if status != nil {
		query = `
			SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
			FROM jobs
			WHERE user_id = $1 AND status = $2
			ORDER BY scheduled_for DESC
			LIMIT $3
		`
		args = []interface{}{userID, *status, limit}
	} else {
		query = `
			SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
			FROM jobs
			WHERE user_id = $1
			ORDER BY scheduled_for DESC
			LIMIT $2
		`
		args = []interface{}{userID, limit}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to get jobs by user ID",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var jobs []models.StandaloneJob
	for rows.Next() {
		var job models.StandaloneJob
		err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.ActionType,
			&job.ScheduledFor,
			&job.Status,
			&job.Payload,
			&job.ErrorMessage,
			&job.RetryCount,
			&job.MaxRetries,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan job row",
				zap.Error(err))
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

// encodeJobCursor encodes a (scheduled_for, id) pair into an opaque cursor token.
func encodeJobCursor(scheduledFor time.Time, id string) string {
	raw := fmt.Sprintf("%d|%s", scheduledFor.UnixMicro(), id)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// decodeJobCursor decodes a cursor token into (scheduled_for, id).
func decodeJobCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor: %w", err)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid cursor format")
	}
	var micros int64
	_, err = fmt.Sscanf(parts[0], "%d", &micros)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor timestamp: %w", err)
	}
	return time.UnixMicro(micros), parts[1], nil
}

func (r *jobRepository) ListByUserID(ctx context.Context, params repository.JobListParams) (*repository.JobListResult, error) {
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Count total
	var totalCount int
	countErr := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE user_id = $1`, params.UserID).Scan(&totalCount)
	if countErr != nil {
		r.logger.Error("Failed to count jobs", zap.String("userID", params.UserID), zap.Error(countErr))
		return nil, countErr
	}

	// Build cursor-based query (ORDER BY scheduled_for DESC, id DESC)
	var query string
	var args []interface{}

	// Fetch one extra to know if there's a next page
	fetchLimit := pageSize + 1

	if params.Cursor != "" {
		cursorTime, cursorID, err := decodeJobCursor(params.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid page_token: %w", err)
		}
		query = `
			SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
			FROM jobs
			WHERE user_id = $1 AND (scheduled_for, id) < ($2, $3)
			ORDER BY scheduled_for DESC, id DESC
			LIMIT $4
		`
		args = []interface{}{params.UserID, cursorTime, cursorID, fetchLimit}
	} else {
		query = `
			SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
			FROM jobs
			WHERE user_id = $1
			ORDER BY scheduled_for DESC, id DESC
			LIMIT $2
		`
		args = []interface{}{params.UserID, fetchLimit}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to list jobs by user ID", zap.String("userID", params.UserID), zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var jobs []models.StandaloneJob
	for rows.Next() {
		var job models.StandaloneJob
		if err := rows.Scan(
			&job.ID, &job.UserID, &job.ActionType, &job.ScheduledFor,
			&job.Status, &job.Payload, &job.ErrorMessage, &job.RetryCount, &job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
		); err != nil {
			r.logger.Error("Failed to scan job row", zap.Error(err))
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := &repository.JobListResult{
		TotalCount: totalCount,
	}

	if len(jobs) > pageSize {
		// There's a next page
		result.Jobs = jobs[:pageSize]
		lastJob := result.Jobs[pageSize-1]
		result.NextCursor = encodeJobCursor(lastJob.ScheduledFor, lastJob.ID)
	} else {
		result.Jobs = jobs
	}

	return result, nil
}

// ClaimPendingDueJobs reivindica jobs vencidos marcando-os 'running' e
// devolvendo-os no MESMO statement.
//
// A versão anterior fazia `SELECT ... FOR UPDATE SKIP LOCKED` por `pool.Query`,
// que roda fora de transação: o lock morria no `rows.Close()`, ANTES de o
// serviço chamar `UpdateStatus('running')` — e esse UPDATE ia por outra
// conexão. Entre uma coisa e outra o job seguia 'pending' e sem lock, então um
// tick sobreposto (ou uma segunda réplica do server) pegava o mesmo job e
// enviava a mensagem duas vezes. Com o UPDATE ... WHERE id IN (SELECT ... FOR
// UPDATE SKIP LOCKED) RETURNING, lock e mudança de status são atômicos.
func (r *jobRepository) ClaimPendingDueJobs(ctx context.Context, limit int) ([]models.StandaloneJob, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		UPDATE jobs
		SET status = $1, updated_at = NOW()
		WHERE id IN (
			SELECT id
			FROM jobs
			WHERE status = $2 AND scheduled_for <= NOW()
			ORDER BY scheduled_for ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
	`

	rows, err := r.db.Query(ctx, query,
		models.StandaloneJobStatusRunning,
		models.StandaloneJobStatusPending,
		limit,
	)
	if err != nil {
		r.logger.Error("Failed to claim pending due jobs",
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var jobs []models.StandaloneJob
	for rows.Next() {
		var job models.StandaloneJob
		err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.ActionType,
			&job.ScheduledFor,
			&job.Status,
			&job.Payload,
			&job.ErrorMessage,
			&job.RetryCount,
			&job.MaxRetries,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan pending job row",
				zap.Error(err))
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// RETURNING não garante ordem (o ORDER BY vale para o subselect que escolhe
	// as linhas, não para a saída). O consumidor processa em sequência e pode
	// ser cortado pelo cap de 5min do cron, então o mais atrasado tem que sair
	// primeiro — quem sobrar é devolvido para 'pending' e pega o próximo tick.
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].ScheduledFor.Before(jobs[j].ScheduledFor)
	})

	return jobs, nil
}

// ReleaseClaimedJobs devolve para 'pending' jobs reivindicados que não chegaram
// a rodar.
//
// Existe por causa do cap de 5min do ctx do cron: com o claim em lote, todos os
// 50 jobs já saem 'running', então um lote que estoura o tempo deixaria os não
// processados presos até o reaper marcá-los 'failed' 30min depois. Antes do
// claim atômico eles simplesmente continuavam 'pending' e voltavam no tick
// seguinte — este método preserva esse comportamento.
//
// O `AND status = 'running'` garante que um job que JÁ teve desfecho terminal
// (done/failed) nunca seja ressuscitado por um release atrasado.
func (r *jobRepository) ReleaseClaimedJobs(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	query := `
		UPDATE jobs
		SET status = $1, updated_at = NOW()
		WHERE id = ANY($2) AND status = $3
	`

	result, err := r.db.pool.Exec(ctx, query,
		models.StandaloneJobStatusPending,
		ids,
		models.StandaloneJobStatusRunning,
	)
	if err != nil {
		r.logger.Error("Failed to release claimed jobs",
			zap.Int("count", len(ids)),
			zap.Error(err))
		return 0, fmt.Errorf("release claimed jobs: %w", err)
	}

	return result.RowsAffected(), nil
}

// isDuplicatePendingJobErr reconhece SOMENTE a violação do índice de dedupe de
// agendamentos. Qualquer outro 23505 continua subindo cru: mascarar violação de
// schema como "já existe agendamento igual" esconderia bug real.
func isDuplicatePendingJobErr(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return pgErr.ConstraintName == "uq_jobs_pending_logical"
}

// jobExecutor é o mínimo comum entre o pool e uma transação, para que o INSERT
// idempotente seja literalmente o mesmo código dentro e fora de transação.
type jobExecutor interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

// `type` e `action_type` recebem o mesmo valor ($3) porque é assim que a tabela
// sempre foi escrita; o índice único usa `type`, que é NOT NULL.
//
// O ON CONFLICT mira o índice parcial `uq_jobs_pending_logical` (migração 079).
// DO NOTHING em vez de DO UPDATE de propósito: um agendamento idêntico já
// pending não tem nada a atualizar — a resposta certa é devolver o original.
const insertJobQuery = `
	INSERT INTO jobs (id, user_id, type, action_type, scheduled_for, status, payload, created_at, updated_at)
	VALUES ($1, $2, $3, $3, $4, $5, $6::jsonb, $7, $7)
	ON CONFLICT (user_id, type, scheduled_for, md5(payload::text)) WHERE status = 'pending'
	DO NOTHING
	RETURNING id, created_at, updated_at
`

// Mesma expressão do índice: `$4::jsonb::text` passa pela normalização canônica
// do Postgres, então ordem de chaves e espaços no JSON do cliente não mudam a
// chave (verificado: {"a":1,"b":2} e {"b":2,"a":1} colidem).
const findPendingJobByLogicalKeyQuery = `
	SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
	FROM jobs
	WHERE user_id = $1 AND type = $2 AND scheduled_for = $3
	  AND md5(payload::text) = md5($4::jsonb::text)
	  AND status = 'pending'
	LIMIT 1
`

// createJob insere o job de forma idempotente e informa se ele já existia.
//
// Contrato deliberado: duplicata NÃO é erro. Um segundo clique no botão de
// agendar (ou o retry de uma aba com o bundle antigo em cache) devolve o MESMO
// job, não um 409 — quem clica duas vezes quis uma coisa só.
func (r *jobRepository) createJob(ctx context.Context, q jobExecutor, job *models.StandaloneJob) (*models.StandaloneJob, bool, error) {
	now := time.Now().UTC()

	// Generate UUID if not provided
	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	// Set default status to pending
	if job.Status == "" {
		job.Status = models.StandaloneJobStatusPending
	}

	// Use ScheduledFor if provided, otherwise default to now
	scheduledFor := job.ScheduledFor
	if scheduledFor.IsZero() {
		scheduledFor = now
	}

	// Serializar aqui (em vez de deixar o driver.Valuer do JobPayload agir)
	// para que o MESMO texto vá para o INSERT e para a busca da chave lógica.
	payloadJSON, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, false, fmt.Errorf("serializar payload do job: %w", err)
	}

	var createdAt, updatedAt time.Time

	err = q.QueryRow(ctx, insertJobQuery,
		job.ID,
		job.UserID,
		job.ActionType,
		scheduledFor,
		job.Status,
		string(payloadJSON),
		now,
	).Scan(&job.ID, &createdAt, &updatedAt)

	if err == nil {
		job.CreatedAt = createdAt
		job.UpdatedAt = updatedAt
		job.ScheduledFor = scheduledFor
		return job, false, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		r.logger.Error("Failed to create job",
			zap.String("userID", job.UserID),
			zap.String("actionType", string(job.ActionType)),
			zap.Error(err))
		return nil, false, err
	}

	// Zero linhas = o DO NOTHING agiu: existe agendamento idêntico pending.
	// Nesse ponto a linha conflitante já está commitada (o ON CONFLICT DO
	// NOTHING espera a transação concorrente resolver), então ela é visível.
	var existing models.StandaloneJob
	err = q.QueryRow(ctx, findPendingJobByLogicalKeyQuery,
		job.UserID,
		job.ActionType,
		scheduledFor,
		string(payloadJSON),
	).Scan(
		&existing.ID,
		&existing.UserID,
		&existing.ActionType,
		&existing.ScheduledFor,
		&existing.Status,
		&existing.Payload,
		&existing.ErrorMessage,
		&existing.RetryCount,
		&existing.MaxRetries,
		&existing.CreatedAt,
		&existing.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Corrida rara: o job conflitante foi apagado entre o INSERT e esta
			// leitura. Erro explícito é melhor que devolver nil silenciosamente.
			return nil, false, fmt.Errorf("agendamento duplicado desapareceu durante a criacao")
		}
		r.logger.Error("Failed to fetch existing duplicate job",
			zap.String("userID", job.UserID),
			zap.Error(err))
		return nil, false, err
	}

	return &existing, true, nil
}

func (r *jobRepository) Create(ctx context.Context, job *models.StandaloneJob) (*models.StandaloneJob, bool, error) {
	return r.createJob(ctx, r.db, job)
}

// CreateBatch cria o lote inteiro numa única transação.
//
// Tudo-ou-nada é o ponto: o frontend criava o lote com N `CreateJob`
// sequenciais e sem try/catch, então falhar no item 300 de 500 deixava 299
// agendamentos órfãos no banco e nenhuma forma de o usuário saber quais.
func (r *jobRepository) CreateBatch(ctx context.Context, jobs []*models.StandaloneJob) ([]models.StandaloneJob, int, error) {
	if len(jobs) == 0 {
		return nil, 0, nil
	}

	created := make([]models.StandaloneJob, 0, len(jobs))
	deduped := 0

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Reset: WithTx não repete a função, mas deixar explícito evita que um
		// retry futuro acumule resultados de uma tentativa abortada.
		created = created[:0]
		deduped = 0

		for _, job := range jobs {
			result, wasDuplicate, err := r.createJob(ctx, tx, job)
			if err != nil {
				return err
			}
			if wasDuplicate {
				deduped++
			}
			created = append(created, *result)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return created, deduped, nil
}

func (r *jobRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic UPDATE query
	setClauses := make([]string, 0, len(updates)+1)
	args := make([]interface{}, 0, len(updates)+2)
	argIndex := 1

	for key, value := range updates {
		// Handle JSONB payload specially - JobPayload implements driver.Valuer
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIndex))
		args = append(args, value)
		argIndex++
	}

	// Always update updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now().UTC())
	argIndex++

	// Add id as the last argument
	args = append(args, id)

	query := fmt.Sprintf("UPDATE jobs SET %s WHERE id = $%d", strings.Join(setClauses, ", "), argIndex)

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		if isDuplicatePendingJobErr(err) {
			// Reagendar um job para cima de outro idêntico já pending. É estado
			// do usuário, não bug: a mensagem tem que ser legível na tela.
			return repository.ErrDuplicatePendingJob
		}
		r.logger.Error("Failed to update job",
			zap.String("jobID", id),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (r *jobRepository) UpdateStatus(ctx context.Context, id string, status models.StandaloneJobStatus, errorMsg *string) error {
	query := `
		UPDATE jobs
		SET status = $1, error_message = $2, updated_at = $3
		WHERE id = $4
	`

	result, err := r.db.pool.Exec(ctx, query, status, errorMsg, time.Now().UTC(), id)
	if err != nil {
		r.logger.Error("Failed to update job status",
			zap.String("jobID", id),
			zap.String("status", string(status)),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

// FailStaleRunning marca como 'failed' jobs presos em 'running' há mais de
// olderThan (crash/deploy no meio da execução, ou o ctx do cron — cap 5min —
// morreu antes do desfecho terminal). Sem isso o job sumia do pipeline em
// 'running' eterno. Vai para 'failed' (e NÃO de volta para 'pending') de
// propósito: re-executar um job de envio parcialmente concluído re-enviaria
// aos grupos já atendidos — duplicar é pior que perder o restante.
func (r *jobRepository) FailStaleRunning(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		UPDATE jobs
		SET status = $1,
		    error_message = 'job interrompido (crash/timeout) - marcado como failed para nao duplicar envios',
		    updated_at = NOW()
		WHERE status = $2 AND updated_at < $3
	`

	result, err := r.db.pool.Exec(ctx, query,
		models.StandaloneJobStatusFailed,
		models.StandaloneJobStatusRunning,
		time.Now().UTC().Add(-olderThan),
	)
	if err != nil {
		r.logger.Error("Failed to fail stale running jobs", zap.Error(err))
		return 0, fmt.Errorf("fail stale running jobs: %w", err)
	}

	n := result.RowsAffected()
	if n > 0 {
		r.logger.Warn("Marked stale 'running' jobs as failed", zap.Int64("count", n))
	}
	return n, nil
}

func (r *jobRepository) ScheduleRetry(ctx context.Context, id string, nextRunAt time.Time) error {
	query := `
		UPDATE jobs
		SET status = $1, retry_count = retry_count + 1, scheduled_for = $2, error_message = NULL, updated_at = $3
		WHERE id = $4
	`

	result, err := r.db.pool.Exec(ctx, query, models.StandaloneJobStatusPending, nextRunAt, time.Now().UTC(), id)
	if err != nil {
		r.logger.Error("Failed to schedule retry",
			zap.String("jobID", id),
			zap.Time("nextRunAt", nextRunAt),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (r *jobRepository) Delete(ctx context.Context, id, userID string) error {
	query := `
		DELETE FROM jobs
		WHERE id = $1 AND user_id = $2
	`

	result, err := r.db.pool.Exec(ctx, query, id, userID)
	if err != nil {
		r.logger.Error("Failed to delete job",
			zap.String("jobID", id),
			zap.String("userID", userID),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found or not owned by user: %s", id)
	}

	return nil
}

func (r *jobRepository) CountByStatus(ctx context.Context, userID string) (map[models.StandaloneJobStatus]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM jobs
		WHERE user_id = $1
		GROUP BY status
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to count jobs by status",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	result := make(map[models.StandaloneJobStatus]int)
	for rows.Next() {
		var status models.StandaloneJobStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			r.logger.Error("Failed to scan job count row",
				zap.Error(err))
			return nil, err
		}
		result[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
