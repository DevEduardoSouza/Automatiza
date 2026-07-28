package postgres

import (
	"automatiza-go/internal/repository"
	"context"
	"fmt"

	"go.uber.org/zap"
)

type dashboardRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewDashboardRepository creates a new PostgreSQL dashboard repository
func NewDashboardRepository(db *DB, logger *zap.Logger) repository.DashboardRepository {
	return &dashboardRepository{
		db:     db,
		logger: logger,
	}
}

// GetGroupsStats retrieves aggregated group statistics for a user's dashboard
func (r *dashboardRepository) GetGroupsStats(ctx context.Context, userID string) (*repository.DashboardStats, error) {
	// Counts only groups that are connected (a participating chip is online)
	// AND in use (referenced by an active automation as send target or mirror source).
	//
	// total_members é a EXCEÇÃO: vem do tracker (group_memberships), que conta
	// pessoa a pessoa em TODOS os grupos rastreados. SUM(groups.members_count)
	// é um snapshot sobrescrito a cada sync e restrito a esse recorte, então
	// divergia do gráfico de crescimento. As duas fontes do tracker batem entre
	// si (count(is_active) == soma do último members_count por grupo).
	// Fallback para o valor antigo enquanto o usuário não tem grupo rastreado —
	// sem ele, quem está fora do tracking veria o card cair para zero.
	query := `
		SELECT
			COUNT(*)::int as total_groups,
			COUNT(*) FILTER (WHERE g.status = 'full')::int as full_groups,
			COALESCE(SUM(g.members_count), 0)::int as legacy_members,
			(SELECT COUNT(*) FROM groups tg
			  WHERE tg.user_id = $1 AND tg.member_tracking_started_at IS NOT NULL)::int as tracked_groups,
			(SELECT COUNT(*) FROM group_memberships gm
			  WHERE gm.user_id = $1 AND gm.is_active)::int as tracked_members
		FROM groups g
		WHERE g.user_id = $1
		  AND EXISTS (
		      SELECT 1 FROM instances i
		      WHERE (i.id = g.instance_id OR i.id = ANY(g.participating_instance_ids))
		        AND i.status IN ('connected', 'open')
		  )
		  AND EXISTS (
		      SELECT 1 FROM automations a
		      WHERE a.user_id = g.user_id
		        AND a.status = 'active'
		        AND (
		             a.send_groups        @> to_jsonb(g.wa_group_id::text)
		          OR a.group_ids          @> to_jsonb(g.wa_group_id::text)
		          OR a.monitor_group_ids  @> to_jsonb(g.wa_group_id::text)
		          OR a.group_ids          @> to_jsonb(g.id::text)
		        )
		  )
	`

	r.logger.Info("Executing GetGroupsStats query",
		zap.String("userID", userID))

	var stats repository.DashboardStats
	var legacyMembers, trackedGroups, trackedMembers int
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&stats.TotalGroups,
		&stats.FullGroups,
		&legacyMembers,
		&trackedGroups,
		&trackedMembers,
	)
	if err != nil {
		r.logger.Error("Failed to get groups stats",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}

	stats.TotalMembers = legacyMembers
	if trackedGroups > 0 {
		stats.TotalMembers = trackedMembers
	}

	r.logger.Info("GetGroupsStats result",
		zap.String("userID", userID),
		zap.Int("totalGroups", stats.TotalGroups),
		zap.Int("fullGroups", stats.FullGroups),
		zap.Int("totalMembers", stats.TotalMembers))

	return &stats, nil
}

// GetActiveLinksCount retrieves the count of active redirect links for a user
func (r *dashboardRepository) GetActiveLinksCount(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*)::int
		FROM saved_redirect_links
		WHERE user_id = $1 AND is_active = true
	`

	r.logger.Info("Executing GetActiveLinksCount query",
		zap.String("userID", userID))

	var count int
	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to get active links count",
			zap.String("userID", userID),
			zap.Error(err))
		return 0, err
	}

	r.logger.Info("GetActiveLinksCount result",
		zap.String("userID", userID),
		zap.Int("count", count))

	return count, nil
}

// GetAnnouncementSettings retrieves the dashboard announcement settings
func (r *dashboardRepository) GetAnnouncementSettings(ctx context.Context) (*repository.AnnouncementSettings, error) {
	query := `
		SELECT key, value
		FROM settings
		WHERE key IN (
			'dashboard_announcement_enabled',
			'dashboard_announcement_title',
			'dashboard_announcement_body'
		)
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to get announcement settings", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	settings := &repository.AnnouncementSettings{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}

		switch key {
		case "dashboard_announcement_enabled":
			settings.Enabled = value == "1" || value == "true"
		case "dashboard_announcement_title":
			settings.Title = value
		case "dashboard_announcement_body":
			settings.Body = value
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return settings, nil
}

// GetHistoryStats retrieves historical stats for the dashboard chart
func (r *dashboardRepository) GetHistoryStats(ctx context.Context, userID string, period repository.HistoryPeriod) ([]repository.HistoryDataPoint, error) {
	r.logger.Info("Executing GetHistoryStats query",
		zap.String("userID", userID),
		zap.String("period", string(period)))

	switch period {
	case repository.HistoryPeriodWeekly:
		return r.getWeeklyStats(ctx, userID)
	case repository.HistoryPeriodMonthly:
		return r.getMonthlyStats(ctx, userID)
	case repository.HistoryPeriodYearly:
		return r.getYearlyStats(ctx, userID)
	default:
		return r.getWeeklyStats(ctx, userID)
	}
}

// historyStatsTemplate monta a série do gráfico do dashboard.
//
// members: LOCF (last observation carried forward) por GRUPO sobre
// group_member_daily_snapshots. members_count é observação PONTUAL e ESPARSA
// (só é gravada no dia em que houve webhook ou reconciliação daquele grupo),
// então somar cru o bucket subconta — 13 de 17 grupos observados no dia dariam
// 1372 em vez de 1878. Antes da primeira observação do grupo vale o
// carry-BACKWARD: a linha fica reta no primeiro valor conhecido, em vez de um
// degrau fantasma quando os baselines entram em dias diferentes.
//
// joins/leaves: soma CRUA do bucket. São fatos datados, não estado — LOCF aqui
// seria errado.
//
// O cálculo antigo (SUM(groups.members_count) fatiado por data de CRIAÇÃO do
// grupo) sobrevive só como fallback por bucket: quem ainda não tem nenhuma
// observação do tracker continua vendo exatamente o que via antes, em vez de
// perder o gráfico.
//
// ponytail: a sondagem de baseline caminha para trás na PK até achar um
// members_count não-nulo, e esse predicado não está em índice. Medido em
// 62ms no anual do maior usuário (680 grupos rastreados) com ~6 dias de
// histórico. Se com a tabela cheia (retenção 400d) passar de ~1s, o upgrade é
// um índice parcial (user_id, wa_group_id, snapshot_date DESC)
// WHERE members_count IS NOT NULL.
//
// %[1]s = CTE buckets (idx, bucket_start, bucket_end)
// %[2]s = expressão do rótulo, interpretada pelo Go
const historyStatsTemplate = `
	WITH buckets AS (
%[1]s
	),
	-- idx 0 é o pseudo-bucket do baseline pré-janela: é dele que o
	-- carry-forward entra na janela.
	bucket_axis AS (
		SELECT 0 AS idx
		UNION ALL
		SELECT idx FROM buckets
	),
	-- TODOS os grupos rastreados: sem is_admin, sem filtro de automação.
	-- member_tracking_started_at IS NOT NULL é a definição de "rastreado" — a
	-- mesma guarda que o writer usa para gravar members_count.
	tracked AS (
		SELECT g.wa_group_id
		FROM groups g
		WHERE g.user_id = $1
		  AND g.member_tracking_started_at IS NOT NULL
	),
	-- Uma sondagem por GRUPO (não por grupo × bucket): última observação
	-- anterior à janela. A PK (user_id, wa_group_id, snapshot_date) deixa o
	-- histórico do grupo contíguo, então é descend + leitura para trás.
	baseline AS (
		SELECT t.wa_group_id, b.snapshot_date, b.members_count
		FROM tracked t
		CROSS JOIN LATERAL (
			SELECT s.snapshot_date, s.members_count
			FROM group_member_daily_snapshots s
			WHERE s.user_id = $1
			  AND s.wa_group_id = t.wa_group_id
			  AND s.snapshot_date < (SELECT min(bucket_start) FROM buckets)
			  AND s.members_count IS NOT NULL
			ORDER BY s.snapshot_date DESC
			LIMIT 1
		) b
	),
	-- Varredura de faixa única, em idx_group_member_daily_snapshots_user_date.
	-- Lida duas vezes: dela saem joins/leaves E as observações de members, ou
	-- seja, o LOCF pega carona na varredura que já era obrigatória.
	window_rows AS MATERIALIZED (
		SELECT b.idx, s.wa_group_id, s.snapshot_date,
		       s.members_count, s.joins_count, s.leaves_count
		FROM buckets b
		JOIN group_member_daily_snapshots s
		  ON s.user_id = $1
		 AND s.snapshot_date BETWEEN b.bucket_start AND b.bucket_end
	),
	flows AS (
		SELECT idx,
		       coalesce(sum(joins_count), 0)::int  AS joins,
		       coalesce(sum(leaves_count), 0)::int AS leaves
		FROM window_rows
		GROUP BY idx
	),
	-- Colapso da janela: por grupo e por bucket vale a observação do ÚLTIMO
	-- dia observado do bucket (não a soma, não o max).
	obs_bucketed AS (
		SELECT DISTINCT ON (wa_group_id, idx) wa_group_id, idx, members_count
		FROM (
			SELECT idx, wa_group_id, snapshot_date, members_count
			FROM window_rows
			WHERE members_count IS NOT NULL
			UNION ALL
			SELECT 0, wa_group_id, snapshot_date, members_count
			FROM baseline
		) x
		ORDER BY wa_group_id, idx, snapshot_date DESC
	),
	-- Grupo sem observação nenhuma não entra na grade: contribui nada em TODO
	-- bucket, então não cria degrau.
	observed_groups AS (
		SELECT DISTINCT wa_group_id FROM obs_bucketed
	),
	grid AS (
		SELECT a.idx, og.wa_group_id, ob.members_count
		FROM bucket_axis a
		CROSS JOIN observed_groups og
		LEFT JOIN obs_bucketed ob
		       ON ob.wa_group_id = og.wa_group_id
		      AND ob.idx = a.idx
	),
	-- Emulação de IGNORE NULLS: run = nº de observações até aqui; dentro de um
	-- run existe exatamente um não-NULL, então max() sobre (grupo, run) é o
	-- último valor conhecido.
	runs AS (
		SELECT idx, wa_group_id, members_count,
		       count(members_count) OVER (
		           PARTITION BY wa_group_id ORDER BY idx
		           ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		       ) AS run
		FROM grid
	),
	locf AS (
		SELECT idx, wa_group_id,
		       coalesce(
		           max(members_count) OVER (PARTITION BY wa_group_id, run),
		           first_value(members_count) OVER (
		               PARTITION BY wa_group_id
		               ORDER BY (members_count IS NULL), idx
		           )
		       ) AS members
		FROM runs
	),
	members_by_bucket AS (
		SELECT idx, coalesce(sum(members), 0)::int AS members
		FROM locf
		WHERE idx > 0
		GROUP BY idx
	)
	SELECT
		b.idx,
		%[2]s AS label_key,
		(SELECT count(*) FROM groups g
		  WHERE g.user_id = $1
		    AND (g.created_at AT TIME ZONE 'America/Sao_Paulo')::date <= b.bucket_end)::int
		    AS total_groups,
		(SELECT count(*) FROM groups g
		  WHERE g.user_id = $1
		    AND (g.created_at AT TIME ZONE 'America/Sao_Paulo')::date
		        BETWEEN b.bucket_start AND b.bucket_end)::int
		    AS new_groups,
		coalesce(
			mb.members,
			(SELECT coalesce(sum(g.members_count), 0) FROM groups g
			  WHERE g.user_id = $1
			    AND (g.created_at AT TIME ZONE 'America/Sao_Paulo')::date <= b.bucket_end)
		)::int AS members,
		coalesce(f.joins, 0)::int  AS joins,
		coalesce(f.leaves, 0)::int AS leaves
	FROM buckets b
	LEFT JOIN members_by_bucket mb ON mb.idx = b.idx
	LEFT JOIN flows f              ON f.idx  = b.idx
	ORDER BY b.idx`

// CTEs de bucket. Ancoradas em America/Sao_Paulo porque snapshot_date já é
// gravado no dia LOCAL — com CURRENT_DATE (UTC) a borda erraria por 3h.
const (
	bucketsWeekly = `
		SELECT
			(row_number() OVER (ORDER BY d.day))::int AS idx,
			d.day::date AS bucket_start,
			d.day::date AS bucket_end
		FROM generate_series(
			(now() AT TIME ZONE 'America/Sao_Paulo')::date - INTERVAL '6 days',
			(now() AT TIME ZONE 'America/Sao_Paulo')::date,
			INTERVAL '1 day'
		) AS d(day)`

	bucketsMonthly = `
		SELECT
			(4 - w.week_offset)::int AS idx,
			(DATE_TRUNC('week', (now() AT TIME ZONE 'America/Sao_Paulo')::date)
			  - (w.week_offset * INTERVAL '1 week'))::date AS bucket_start,
			(DATE_TRUNC('week', (now() AT TIME ZONE 'America/Sao_Paulo')::date)
			  - (w.week_offset * INTERVAL '1 week') + INTERVAL '6 days')::date AS bucket_end
		FROM generate_series(3, 0, -1) AS w(week_offset)`

	bucketsYearly = `
		SELECT
			(12 - m.month_offset)::int AS idx,
			(DATE_TRUNC('month', (now() AT TIME ZONE 'America/Sao_Paulo')::date)
			  - (m.month_offset * INTERVAL '1 month'))::date AS bucket_start,
			(DATE_TRUNC('month', (now() AT TIME ZONE 'America/Sao_Paulo')::date)
			  - (m.month_offset * INTERVAL '1 month')
			  + INTERVAL '1 month' - INTERVAL '1 day')::date AS bucket_end
		FROM generate_series(11, 0, -1) AS m(month_offset)`
)

// queryHistory roda o template para um período. A ordem do scan é a mesma para
// os três: idx, label_key, total_groups, new_groups, members, joins, leaves.
func (r *dashboardRepository) queryHistory(
	ctx context.Context,
	userID, period, bucketsCTE, labelExpr string,
	label func(idx, labelKey int) string,
) ([]repository.HistoryDataPoint, error) {
	query := fmt.Sprintf(historyStatsTemplate, bucketsCTE, labelExpr)

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to get history stats",
			zap.String("userID", userID),
			zap.String("period", period),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var result []repository.HistoryDataPoint
	for rows.Next() {
		var idx, labelKey int
		var dp repository.HistoryDataPoint
		if err := rows.Scan(&idx, &labelKey, &dp.Groups, &dp.NewGroups, &dp.Members, &dp.Joins, &dp.Leaves); err != nil {
			return nil, err
		}
		// new_members é o campo que o client rotula como "Novos membros" —
		// agora são entradas de verdade, não o tamanho de grupos importados.
		dp.NewMembers = dp.Joins
		dp.Label = label(idx, labelKey)
		result = append(result, dp)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	r.logger.Info("GetHistoryStats result",
		zap.String("userID", userID),
		zap.String("period", period),
		zap.Int("dataPoints", len(result)))

	return result, nil
}

// getWeeklyStats retrieves stats for the last 7 days, grouped by day
func (r *dashboardRepository) getWeeklyStats(ctx context.Context, userID string) ([]repository.HistoryDataPoint, error) {
	// Portuguese day abbreviations
	dayLabels := []string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"}

	return r.queryHistory(ctx, userID, "weekly", bucketsWeekly,
		"EXTRACT(DOW FROM b.bucket_end)::int",
		func(_, dow int) string {
			if dow < 0 || dow >= len(dayLabels) {
				return ""
			}
			return dayLabels[dow]
		})
}

// getMonthlyStats retrieves stats for the last 4 weeks, grouped by week
func (r *dashboardRepository) getMonthlyStats(ctx context.Context, userID string) ([]repository.HistoryDataPoint, error) {
	return r.queryHistory(ctx, userID, "monthly", bucketsMonthly,
		"b.idx",
		func(idx, _ int) string {
			// Format: "Sem 1", "Sem 2", etc.
			return fmt.Sprintf("Sem %d", idx)
		})
}

// getYearlyStats retrieves stats for the last 12 months, grouped by month
func (r *dashboardRepository) getYearlyStats(ctx context.Context, userID string) ([]repository.HistoryDataPoint, error) {
	monthLabels := []string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}

	return r.queryHistory(ctx, userID, "yearly", bucketsYearly,
		"EXTRACT(MONTH FROM b.bucket_start)::int",
		func(_, month int) string {
			if month < 1 || month > len(monthLabels) {
				return ""
			}
			return monthLabels[month-1]
		})
}
