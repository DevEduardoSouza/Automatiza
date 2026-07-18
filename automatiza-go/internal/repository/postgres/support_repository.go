package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// SupportRepository concentra as consultas exclusivas do painel de suporte.
//
// Vive separado dos repositorios de produto de proposito: as consultas daqui
// atravessam a fronteira de dono (recebem user_id arbitrario) e agregam dados
// que nenhum fluxo de cliente precisa. Mantendo-as isoladas, o dia em que
// alguem mexer nos repositorios de produto nao muda o painel sem querer — e
// vice-versa.
type SupportRepository struct {
	db     *DB
	logger *zap.Logger
}

// SupportUserRow e uma linha do resultado de busca de usuario.
type SupportUserRow struct {
	ID                string
	Email             string
	Name              *string
	PhoneNumber       *string
	Plan              *string
	BillingStatus     *string
	IsActive          bool
	ChipsTotal        int
	ChipsConnected    int
	AutomationsTotal  int
	AutomationsActive int
	CreatedAt         *time.Time
	MatchedEmail      bool
	MatchedName       bool
	MatchedPhone      bool
	MatchedInstance   bool
}

// SupportProfileRow e o perfil como o painel precisa ver.
//
// Existe porque models.Profile nao carrega name, is_active, max_groups nem
// last_login_at — colunas que a tabela tem e que o atendimento usa (achar o
// cliente pelo nome, ver se a conta esta ativa, quando ele logou por ultimo).
type SupportProfileRow struct {
	ID                      string
	Email                   string
	Name                    *string
	PhoneNumber             *string
	Plan                    *string
	BillingStatus           *string
	BillingSuspensionReason *string
	IsActive                bool
	MaxInstances            int
	MaxAutomations          int
	MaxGroups               int
	LastLoginAt             *time.Time
	CreatedAt               *time.Time
}

// SupportUserActivity agrega o que saiu (ou nao) para um usuario numa janela.
type SupportUserActivity struct {
	SentInWindow   int
	FailedInWindow int
	QueuedNow      int
}

// SupportAutomationActivityRow e a atividade por automacao na janela.
type SupportAutomationActivityRow struct {
	AutomationID   string
	SentInWindow   int
	FailedInWindow int
	QueuedNow      int
	LastDispatchAt *time.Time
}

// NewSupportRepository cria o repositorio do painel de suporte.
func NewSupportRepository(db *DB, logger *zap.Logger) *SupportRepository {
	return &SupportRepository{db: db, logger: logger}
}

// GetUserProfile busca o perfil completo de um usuario para o painel.
func (r *SupportRepository) GetUserProfile(ctx context.Context, userID string) (*SupportProfileRow, error) {
	sql := `
		SELECT id, email, name, phone_number, plan, billing_status,
		       billing_suspension_reason, COALESCE(is_active, true),
		       COALESCE(max_instances, 0), COALESCE(max_automations, 0),
		       COALESCE(max_groups, 0), last_login_at, created_at
		FROM profiles
		WHERE id = $1
	`
	var p SupportProfileRow
	err := r.db.pool.QueryRow(ctx, sql, userID).Scan(
		&p.ID, &p.Email, &p.Name, &p.PhoneNumber, &p.Plan, &p.BillingStatus,
		&p.BillingSuspensionReason, &p.IsActive,
		&p.MaxInstances, &p.MaxAutomations, &p.MaxGroups, &p.LastLoginAt, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get support profile: %w", err)
	}
	return &p, nil
}

// SearchUsers busca usuarios por email, nome, telefone do perfil ou por chip
// (instance_id / telefone da instancia).
//
// A busca por chip existe porque o atendimento quase sempre comeca por "o
// numero X parou": o cliente reclama pelo chip, nao pelo email da conta.
//
// Nota de custo: sao ILIKE sem indice trigram, sobre ~874 profiles e ~989
// instances. Nessa escala o seq scan e barato; se profiles crescer uma ordem
// de grandeza, isto precisa de pg_trgm.
func (r *SupportRepository) SearchUsers(ctx context.Context, query string, limit int) ([]SupportUserRow, error) {
	term := strings.TrimSpace(query)
	if term == "" {
		return nil, fmt.Errorf("search query is required")
	}
	pattern := "%" + term + "%"

	// digitsPattern permite achar telefone digitado com mascara ("(11) 99999")
	// contra o valor cru do banco, e vice-versa.
	//
	// hasDigits desliga o ramo de telefone quando o termo nao tem numero: sem
	// ele, o padrao viraria "%%" e casaria com todo mundo que tem telefone —
	// buscar "maria" devolveria a base inteira.
	digits := digitsOnly(term)
	hasDigits := digits != ""
	digitsPattern := "%" + digits + "%"

	sql := `
		WITH matched AS (
			SELECT p.id,
			       (p.email ILIKE $1) AS m_email,
			       (p.name IS NOT NULL AND p.name ILIKE $1) AS m_name,
			       ($4::bool AND p.phone_number IS NOT NULL
			        AND regexp_replace(p.phone_number, '\D', '', 'g') ILIKE $2) AS m_phone,
			       EXISTS (
			           SELECT 1 FROM instances i
			           WHERE i.user_id = p.id
			             AND (i.instance_id ILIKE $1
			                  OR ($4::bool AND i.phone_number IS NOT NULL
			                      AND regexp_replace(i.phone_number, '\D', '', 'g') ILIKE $2))
			       ) AS m_instance
			FROM profiles p
		)
		SELECT p.id, p.email, p.name, p.phone_number, p.plan, p.billing_status,
		       COALESCE(p.is_active, true),
		       COALESCE(c.chips_total, 0), COALESCE(c.chips_connected, 0),
		       COALESCE(a.autos_total, 0), COALESCE(a.autos_active, 0),
		       p.created_at,
		       m.m_email, m.m_name, m.m_phone, m.m_instance
		FROM profiles p
		JOIN matched m ON m.id = p.id
		LEFT JOIN (
			SELECT user_id,
			       COUNT(*) AS chips_total,
			       COUNT(*) FILTER (WHERE status = 'connected') AS chips_connected
			FROM instances GROUP BY user_id
		) c ON c.user_id = p.id
		LEFT JOIN (
			SELECT user_id,
			       COUNT(*) AS autos_total,
			       COUNT(*) FILTER (WHERE status = 'active') AS autos_active
			FROM automations GROUP BY user_id
		) a ON a.user_id = p.id
		WHERE m.m_email OR m.m_name OR m.m_phone OR m.m_instance
		ORDER BY
			-- Casar por email vem primeiro: e a busca mais comum e a mais exata.
			(m.m_email)::int DESC,
			p.created_at DESC
		LIMIT $3
	`

	rows, err := r.db.pool.Query(ctx, sql, pattern, digitsPattern, limit, hasDigits)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer rows.Close()

	var out []SupportUserRow
	for rows.Next() {
		var u SupportUserRow
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.Plan, &u.BillingStatus,
			&u.IsActive, &u.ChipsTotal, &u.ChipsConnected,
			&u.AutomationsTotal, &u.AutomationsActive, &u.CreatedAt,
			&u.MatchedEmail, &u.MatchedName, &u.MatchedPhone, &u.MatchedInstance,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate user rows: %w", err)
	}
	return out, nil
}

// GetUserActivity agrega dispatch_logs do usuario na janela.
//
// 'queued' NAO tem recorte de janela: um job preso em queued ha horas ainda e
// backlog vivo, e some do numero se filtrarmos pelos ultimos N minutos —
// justamente o caso que o suporte precisa enxergar.
func (r *SupportRepository) GetUserActivity(ctx context.Context, userID string, window time.Duration) (*SupportUserActivity, error) {
	sql := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'sent'   AND created_at > NOW() - $2::interval),
			COUNT(*) FILTER (WHERE status IN ('failed','error') AND created_at > NOW() - $2::interval),
			COUNT(*) FILTER (WHERE status = 'queued')
		FROM dispatch_logs
		WHERE user_id = $1
		  AND created_at > NOW() - GREATEST($2::interval, INTERVAL '24 hours')
	`
	var a SupportUserActivity
	err := r.db.pool.QueryRow(ctx, sql, userID, window.String()).
		Scan(&a.SentInWindow, &a.FailedInWindow, &a.QueuedNow)
	if err != nil {
		return nil, fmt.Errorf("failed to get user activity: %w", err)
	}
	return &a, nil
}

// GetAutomationActivity agrega dispatch_logs por automacao do usuario.
func (r *SupportRepository) GetAutomationActivity(ctx context.Context, userID string, window time.Duration) ([]SupportAutomationActivityRow, error) {
	sql := `
		SELECT automation_id,
		       COUNT(*) FILTER (WHERE status = 'sent'   AND created_at > NOW() - $2::interval),
		       COUNT(*) FILTER (WHERE status IN ('failed','error') AND created_at > NOW() - $2::interval),
		       COUNT(*) FILTER (WHERE status = 'queued'),
		       MAX(created_at) FILTER (WHERE status = 'sent')
		FROM dispatch_logs
		WHERE user_id = $1
		  AND automation_id IS NOT NULL
		  AND created_at > NOW() - GREATEST($2::interval, INTERVAL '24 hours')
		GROUP BY automation_id
	`
	rows, err := r.db.pool.Query(ctx, sql, userID, window.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get automation activity: %w", err)
	}
	defer rows.Close()

	var out []SupportAutomationActivityRow
	for rows.Next() {
		var a SupportAutomationActivityRow
		if err := rows.Scan(&a.AutomationID, &a.SentInWindow, &a.FailedInWindow, &a.QueuedNow, &a.LastDispatchAt); err != nil {
			return nil, fmt.Errorf("failed to scan automation activity: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate automation activity: %w", err)
	}
	return out, nil
}

// GetRunningAutomationIDs devolve as automacoes com run aberto e lock vivo —
// o sinal de "esta disparando agora" para os tipos search/list, que rodam por
// cron. Nao vale para monitoring/offer_generate: essas sao event-driven por
// webhook e nunca passam pelo scheduler, entao nao abrem automation_run.
func (r *SupportRepository) GetRunningAutomationIDs(ctx context.Context, userID string) (map[string]bool, error) {
	sql := `
		SELECT DISTINCT ar.automation_id
		FROM automation_runs ar
		JOIN automations a ON a.id = ar.automation_id
		WHERE a.user_id = $1
		  AND ar.status = 'running'
		  AND (ar.lock_until IS NULL OR ar.lock_until > NOW())
	`
	rows, err := r.db.pool.Query(ctx, sql, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get running automations: %w", err)
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan running automation: %w", err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate running automations: %w", err)
	}
	return out, nil
}

// GetGroupCountByInstance conta grupos do usuario por chip participante,
// chaveado pelo instance_id textual (a sessao WAHA), que e o que a UI mostra.
//
// groups.participating_instance_ids e int4[] e referencia instances.id (a PK
// serial), NAO o instance_id textual — dai o join.
func (r *SupportRepository) GetGroupCountByInstance(ctx context.Context, userID string) (map[string]int, error) {
	sql := `
		SELECT i.instance_id, COUNT(*)
		FROM groups g
		CROSS JOIN LATERAL unnest(
			COALESCE(g.participating_instance_ids, ARRAY[]::int[])
		) AS pid
		JOIN instances i ON i.id = pid
		WHERE g.user_id = $1
		GROUP BY i.instance_id
	`
	rows, err := r.db.pool.Query(ctx, sql, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count groups by instance: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var instID string
		var n int
		if err := rows.Scan(&instID, &n); err != nil {
			return nil, fmt.Errorf("failed to scan group count: %w", err)
		}
		out[instID] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate group counts: %w", err)
	}
	return out, nil
}

// GetLinkListItemStats conta itens por lista do usuario numa tacada so.
// Evita o N+1 de chamar GetItemsByListIDPaginated por lista — ml_link_list_items
// tem ~2.5M linhas em producao.
func (r *SupportRepository) GetLinkListItemStats(ctx context.Context, userID string) (map[string][3]int, error) {
	sql := `
		SELECT l.id,
		       COUNT(i.*),
		       COUNT(i.*) FILTER (WHERE i.consumed_at IS NOT NULL),
		       COUNT(i.*) FILTER (WHERE i.consumed_at IS NULL)
		FROM ml_link_lists l
		LEFT JOIN ml_link_list_items i ON i.list_id = l.id
		WHERE l.user_id = $1
		GROUP BY l.id
	`
	rows, err := r.db.pool.Query(ctx, sql, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get link list stats: %w", err)
	}
	defer rows.Close()

	out := make(map[string][3]int)
	for rows.Next() {
		var id string
		var total, consumed, pending int
		if err := rows.Scan(&id, &total, &consumed, &pending); err != nil {
			return nil, fmt.Errorf("failed to scan link list stats: %w", err)
		}
		out[id] = [3]int{total, consumed, pending}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate link list stats: %w", err)
	}
	return out, nil
}

// ============================================================================
// Saude do sistema (aba Dashboard) — agregados de TODO o sistema, sem recorte
// por usuario. Cada consulta e barata e usa indice existente.
// ============================================================================

// SessionStatusCounts conta instances (sessoes WAHA) por status.
type SessionStatusCounts struct {
	Total        int
	Connected    int
	Disconnected int
	Pending      int
}

// GetSessionStatusCounts agrega instances por status, contando APENAS sessoes
// de usuarios pagantes (billing_status = 'active'). Exclui suspensos (deixaram
// de pagar) e donos sem billing/NULL (legado nao cobrado via Kiwify). O JOIN
// (inner) tambem descarta instances orfas sem profile. Fonte da definicao de
// "pagante": models.BillingStatusActive + suspension_interceptor (so 'suspended'
// bloqueia; NULL e tratado como ativo no acesso, mas nao conta como pagante aqui).
func (r *SupportRepository) GetSessionStatusCounts(ctx context.Context) (*SessionStatusCounts, error) {
	sql := `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE i.status = 'connected'),
		       COUNT(*) FILTER (WHERE i.status = 'disconnected'),
		       COUNT(*) FILTER (WHERE i.status = 'pending')
		FROM instances i
		JOIN profiles p ON p.id = i.user_id
		WHERE p.billing_status = 'active'
	`
	var c SessionStatusCounts
	if err := r.db.pool.QueryRow(ctx, sql).Scan(&c.Total, &c.Connected, &c.Disconnected, &c.Pending); err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}
	return &c, nil
}

// DispatchErrorStat e uma assinatura de ERRO de disparo agregada.
type DispatchErrorStat struct {
	Error string
	Count int
	Users int
}

// GetTopDispatchErrors agrega as assinaturas de ERRO REAL de disparo na janela,
// para o card de erros do painel de suporte. Conta SOMENTE status 'error'/'failed'
// — skips (dedupe, chip desconectado, sem-desconto, backpressure) NAO sao erro e
// ficam de fora (chip desconectado vira a metrica sessions_disconnected). Agrupa
// pela mensagem crua (assinaturas estaticas dominam o topo); janela e limite
// parametrizados. Barato: status 'error'/'failed' e uma fracao minima da tabela.
func (r *SupportRepository) GetTopDispatchErrors(ctx context.Context, window time.Duration, limit int) ([]DispatchErrorStat, error) {
	sql := `
		SELECT error, COUNT(*), COUNT(DISTINCT user_id)
		FROM dispatch_logs
		WHERE status IN ('error', 'failed')
		  AND error IS NOT NULL AND error <> ''
		  AND created_at > NOW() - $1::interval
		GROUP BY error
		ORDER BY COUNT(*) DESC
		LIMIT $2
	`
	interval := fmt.Sprintf("%d seconds", int(window.Seconds()))
	rows, err := r.db.pool.Query(ctx, sql, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top dispatch errors: %w", err)
	}
	defer rows.Close()

	out := make([]DispatchErrorStat, 0, limit)
	for rows.Next() {
		var e DispatchErrorStat
		if err := rows.Scan(&e.Error, &e.Count, &e.Users); err != nil {
			return nil, fmt.Errorf("failed to scan dispatch error stat: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AutomationTypeCounts e a saude de um tipo de automacao.
type AutomationTypeCounts struct {
	Type      string
	Total     int
	Active    int
	Paused    int
	WithError int
}

// GetAutomationHealthByType agrega automacoes por tipo, contando pausadas e com
// erro recente. `type` NULL cai em 'search' (o dispatcher trata assim).
// "com erro" = last_error_at nas ultimas 24h — erro velho ja pode estar
// resolvido e nao deve inflar o painel (mesma regra M2 da pagina de detalhe).
func (r *SupportRepository) GetAutomationHealthByType(ctx context.Context) ([]AutomationTypeCounts, error) {
	sql := `
		SELECT COALESCE(NULLIF(type, ''), 'search') AS t,
		       COUNT(*),
		       COUNT(*) FILTER (WHERE status = 'active'),
		       COUNT(*) FILTER (WHERE status = 'paused'),
		       COUNT(*) FILTER (WHERE last_error_at IS NOT NULL AND last_error_at > NOW() - INTERVAL '24 hours')
		FROM automations
		GROUP BY t
	`
	rows, err := r.db.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate automations: %w", err)
	}
	defer rows.Close()

	var out []AutomationTypeCounts
	for rows.Next() {
		var a AutomationTypeCounts
		if err := rows.Scan(&a.Type, &a.Total, &a.Active, &a.Paused, &a.WithError); err != nil {
			return nil, fmt.Errorf("failed to scan automation health: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListStatusCounts conta ml_link_lists por is_active.
type ListStatusCounts struct {
	Total    int
	Active   int
	Inactive int
}

// GetListStatusCounts agrega ml_link_lists por ativa/inativa.
func (r *SupportRepository) GetListStatusCounts(ctx context.Context) (*ListStatusCounts, error) {
	sql := `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE is_active),
		       COUNT(*) FILTER (WHERE NOT is_active OR is_active IS NULL)
		FROM ml_link_lists
	`
	var c ListStatusCounts
	if err := r.db.pool.QueryRow(ctx, sql).Scan(&c.Total, &c.Active, &c.Inactive); err != nil {
		return nil, fmt.Errorf("failed to count lists: %w", err)
	}
	return &c, nil
}

// GetDispatchLastHour conta enviados e falhos na ultima hora (todo o sistema).
// Usa idx_dispatch_logs_created_at para o range de 1h — barato mesmo com a
// tabela grande.
func (r *SupportRepository) GetDispatchLastHour(ctx context.Context) (sent int, failed int, err error) {
	sql := `
		SELECT COUNT(*) FILTER (WHERE status = 'sent'),
		       COUNT(*) FILTER (WHERE status IN ('failed','error'))
		FROM dispatch_logs
		WHERE created_at > NOW() - INTERVAL '1 hour'
	`
	if e := r.db.pool.QueryRow(ctx, sql).Scan(&sent, &failed); e != nil {
		return 0, 0, fmt.Errorf("failed to count dispatch last hour: %w", e)
	}
	return sent, failed, nil
}

// GetDatabaseSizeBytes retorna o tamanho do banco atual.
func (r *SupportRepository) GetDatabaseSizeBytes(ctx context.Context) (int64, error) {
	var size int64
	if err := r.db.pool.QueryRow(ctx, `SELECT pg_database_size(current_database())`).Scan(&size); err != nil {
		return 0, fmt.Errorf("failed to get database size: %w", err)
	}
	return size, nil
}

// ResumeCounts descreve o resultado de um religar (ou o preview via dry-run).
type ResumeCounts struct {
	Resumed        int // religadas (ou que seriam)
	SkippedBilling int // puladas por paused_by_billing
	SkippedError   int // puladas por erro recente
}

// resumeEligibleWhere e o criterio de "seguro religar", usado igual no preview
// e na execucao: pausada, NAO por cobranca, e SEM erro recente (24h). Manter os
// dois no mesmo lugar garante que a confirmacao mostre exatamente o que sera
// religado.
const resumeEligibleWhere = `
	status = 'paused'
	AND COALESCE(paused_by_billing, false) = false
	AND (last_error_at IS NULL OR last_error_at < NOW() - INTERVAL '24 hours')
`

// CountResumable conta, sem religar, quantas automacoes sao elegiveis e quantas
// seriam puladas e por que. E o preview da confirmacao.
func (r *SupportRepository) CountResumable(ctx context.Context) (*ResumeCounts, error) {
	sql := `
		SELECT
			COUNT(*) FILTER (WHERE ` + resumeEligibleWhere + `),
			COUNT(*) FILTER (WHERE status = 'paused' AND COALESCE(paused_by_billing, false) = true),
			COUNT(*) FILTER (WHERE status = 'paused'
			                 AND COALESCE(paused_by_billing, false) = false
			                 AND last_error_at IS NOT NULL
			                 AND last_error_at >= NOW() - INTERVAL '24 hours')
		FROM automations
	`
	var c ResumeCounts
	if err := r.db.pool.QueryRow(ctx, sql).Scan(&c.Resumed, &c.SkippedBilling, &c.SkippedError); err != nil {
		return nil, fmt.Errorf("failed to count resumable automations: %w", err)
	}
	return &c, nil
}

// ResumePaused religa as automacoes elegiveis (mesmo criterio do preview) numa
// unica transacao, e devolve quantas foram efetivamente religadas + os pulados.
func (r *SupportRepository) ResumePaused(ctx context.Context) (*ResumeCounts, error) {
	// Conta os pulados ANTES de religar (depois eles nao sao mais 'paused').
	pre, err := r.CountResumable(ctx)
	if err != nil {
		return nil, err
	}

	tag, err := r.db.pool.Exec(ctx, `
		UPDATE automations
		SET status = 'active', updated_at = NOW()
		WHERE `+resumeEligibleWhere)
	if err != nil {
		return nil, fmt.Errorf("failed to resume paused automations: %w", err)
	}

	return &ResumeCounts{
		Resumed:        int(tag.RowsAffected()),
		SkippedBilling: pre.SkippedBilling,
		SkippedError:   pre.SkippedError,
	}, nil
}

// ============================================================
// Financeiro (aba Financeiro)
// ============================================================

// FinancePlanRevenueRow e a receita mensal ESTIMADA de um plano (preco do
// catalogo x assinantes ativos mapeados). Nao e' o valor efetivamente pago —
// a Kiwify nao expoe transacao no banco (ver 069_finance_entries).
type FinancePlanRevenueRow struct {
	Plan        string
	Price       float64
	Subscribers int
}

// FinanceSubscriberCounts agrega os perfis por situacao de cobranca.
type FinanceSubscriberCounts struct {
	Active    int // billing_status = 'active'
	Mapped    int // ativos com plan_id (base do MRR estimado)
	Suspended int
	Canceled  int
	Refunded  int // billing_last_event_type ~ refund
	Late      int // subscription_late
}

// FinanceEntryRow e um lancamento financeiro manual (despesa/lucro).
type FinanceEntryRow struct {
	ID               string
	Kind             string
	Description      string
	Amount           float64
	Category         *string
	EntryDate        time.Time
	Recurring        bool
	RecurrenceMonths *int32
	CreatedAt        time.Time
}

// CreateFinanceEntryParams sao os campos de um lancamento novo.
type CreateFinanceEntryParams struct {
	Kind             string
	Description      string
	Amount           float64
	Category         *string
	EntryDate        time.Time
	Recurring        bool
	RecurrenceMonths *int32
	CreatedBy        *string
}

// GetFinancePlanRevenue devolve, por plano, o preco mensal e quantos ativos o
// assinam. Preco = price_monthly (o ciclo real dos ativos e' 'monthly'), com
// fallback para price. So planos com dono ativo e plan_id aparecem.
func (r *SupportRepository) GetFinancePlanRevenue(ctx context.Context) ([]FinancePlanRevenueRow, error) {
	sql := `
		SELECT pl.name,
		       COALESCE(pl.price_monthly, pl.price, 0)::float8 AS price,
		       COUNT(*) AS subs
		FROM profiles p
		JOIN plans pl ON pl.id = p.plan_id
		WHERE p.billing_status = 'active'
		GROUP BY pl.name, COALESCE(pl.price_monthly, pl.price, 0)
		ORDER BY COUNT(*) * COALESCE(pl.price_monthly, pl.price, 0) DESC
	`
	rows, err := r.db.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("finance plan revenue: %w", err)
	}
	defer rows.Close()

	var out []FinancePlanRevenueRow
	for rows.Next() {
		var pr FinancePlanRevenueRow
		if err := rows.Scan(&pr.Plan, &pr.Price, &pr.Subscribers); err != nil {
			return nil, fmt.Errorf("finance plan revenue scan: %w", err)
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

// GetFinanceSubscriberCounts conta perfis por situacao de cobranca numa unica
// varredura.
func (r *SupportRepository) GetFinanceSubscriberCounts(ctx context.Context) (*FinanceSubscriberCounts, error) {
	sql := `
		SELECT
		  COUNT(*) FILTER (WHERE billing_status = 'active'),
		  COUNT(*) FILTER (WHERE billing_status = 'active' AND plan_id IS NOT NULL),
		  COUNT(*) FILTER (WHERE billing_status = 'suspended'),
		  COUNT(*) FILTER (WHERE billing_status = 'canceled'),
		  COUNT(*) FILTER (WHERE billing_last_event_type ILIKE '%refund%'),
		  COUNT(*) FILTER (WHERE billing_last_event_type = 'subscription_late')
		FROM profiles
	`
	var c FinanceSubscriberCounts
	err := r.db.pool.QueryRow(ctx, sql).Scan(
		&c.Active, &c.Mapped, &c.Suspended, &c.Canceled, &c.Refunded, &c.Late,
	)
	if err != nil {
		return nil, fmt.Errorf("finance subscriber counts: %w", err)
	}
	return &c, nil
}

// ListFinanceEntries devolve todos os lancamentos, mais recentes primeiro.
func (r *SupportRepository) ListFinanceEntries(ctx context.Context) ([]FinanceEntryRow, error) {
	sql := `
		SELECT id, kind, description, amount::float8, category, entry_date,
		       recurring, recurrence_months, created_at
		FROM finance_entries
		ORDER BY entry_date DESC, created_at DESC
	`
	rows, err := r.db.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("list finance entries: %w", err)
	}
	defer rows.Close()

	var out []FinanceEntryRow
	for rows.Next() {
		var e FinanceEntryRow
		if err := rows.Scan(&e.ID, &e.Kind, &e.Description, &e.Amount, &e.Category,
			&e.EntryDate, &e.Recurring, &e.RecurrenceMonths, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("finance entry scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CreateFinanceEntry insere um lancamento e devolve a linha criada.
func (r *SupportRepository) CreateFinanceEntry(ctx context.Context, p CreateFinanceEntryParams) (*FinanceEntryRow, error) {
	sql := `
		INSERT INTO finance_entries
		  (kind, description, amount, category, entry_date, recurring, recurrence_months, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, kind, description, amount::float8, category, entry_date,
		          recurring, recurrence_months, created_at
	`
	var e FinanceEntryRow
	err := r.db.pool.QueryRow(ctx, sql,
		p.Kind, p.Description, p.Amount, p.Category, p.EntryDate,
		p.Recurring, p.RecurrenceMonths, p.CreatedBy,
	).Scan(&e.ID, &e.Kind, &e.Description, &e.Amount, &e.Category,
		&e.EntryDate, &e.Recurring, &e.RecurrenceMonths, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create finance entry: %w", err)
	}
	return &e, nil
}

// DeleteFinanceEntry remove um lancamento; devolve false se o id nao existia.
func (r *SupportRepository) DeleteFinanceEntry(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.pool.Exec(ctx, `DELETE FROM finance_entries WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("delete finance entry: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// digitsOnly extrai apenas digitos — telefones sao gravados com formatacao
// inconsistente entre cadastro manual e o que volta do WhatsApp.
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
