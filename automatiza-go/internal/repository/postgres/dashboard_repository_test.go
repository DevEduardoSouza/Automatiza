package postgres

import (
	"context"
	"testing"

	"automatiza-go/internal/repository"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// A série de membros do dashboard vem de observações PONTUAIS e ESPARSAS: um
// grupo só ganha linha no dia em que houve webhook ou reconciliação dele. Os
// dois modos de errar isso são somar cru o bucket (subconta os grupos não
// observados naquele dia) e não retropropagar por grupo (o dia em que o
// baseline de um grupo entra vira um degrau fantasma de crescimento). Os testes
// abaixo falham em qualquer um dos dois.

// seedTrackedSnapshot grava uma observação no dia local (America/Sao_Paulo)
// de hoje menos daysAgo. membersCount < 0 grava NULL (dia só com fluxo).
func seedTrackedSnapshot(t *testing.T, db *DB, f tenantFixture, daysAgo, membersCount, joins, leaves int) {
	t.Helper()

	var members any
	if membersCount >= 0 {
		members = membersCount
	}

	err := db.Exec(context.Background(), `
		INSERT INTO group_member_daily_snapshots
			(user_id, group_id, wa_group_id, snapshot_date, members_count,
			 members_observed_at, joins_count, leaves_count)
		VALUES ($1, $2::uuid, $3,
			(now() AT TIME ZONE 'America/Sao_Paulo')::date - $4::int,
			$5, now(), $6, $7)`,
		f.tenant.UserID, f.groupID, f.waGroupID, daysAgo, members, joins, leaves)
	require.NoError(t, err)
}

// markTracked marca o grupo como rastreado (é a guarda que o writer usa para
// gravar members_count, e a mesma que a série do dashboard usa para varrer).
func markTracked(t *testing.T, db *DB, f tenantFixture) {
	t.Helper()
	err := db.Exec(context.Background(),
		`UPDATE groups SET member_tracking_started_at = now() WHERE id = $1::uuid`, f.groupID)
	require.NoError(t, err)
}

// addGroup adiciona um segundo grupo ao MESMO tenant.
func addGroup(t *testing.T, db *DB, f tenantFixture, waGroupID string, membersCount int) tenantFixture {
	t.Helper()

	var groupID string
	require.NoError(t, db.QueryRow(context.Background(),
		`INSERT INTO groups (user_id, instance_id, wa_group_id, participating_instance_ids, members_count)
		 VALUES ($1, $2, $3, ARRAY[$2::int], $4) RETURNING id::text`,
		f.tenant.UserID, f.tenant.InstancePK, waGroupID, membersCount).Scan(&groupID))

	return tenantFixture{tenant: f.tenant, groupID: groupID, waGroupID: waGroupID}
}

func TestDashboardHistory_CarryForwardAndBackward(t *testing.T) {
	db, _ := setupGroupMemberTestDB(t)
	repo := NewDashboardRepository(db, zap.NewNop())

	a := seedTenant(t, db, "sess-dash", "111@g.us")
	b := addGroup(t, db, a, "222@g.us", 0)
	markTracked(t, db, a)
	markTracked(t, db, b)

	// Só o grupo A é observado 4 dias atrás. O grupo B entra no tracking só 3
	// dias atrás — sem carry-BACKWARD, o dia 4 somaria 100 e o dia 3 somaria
	// 300, um salto de +200 que ninguém deu.
	seedTrackedSnapshot(t, db, a, 4, 100, 0, 0)
	seedTrackedSnapshot(t, db, a, 3, 110, 10, 0)
	seedTrackedSnapshot(t, db, b, 3, 200, 0, 0)
	// Dia 1: nenhum grupo observado, mas houve fluxo em A. Sem carry-FORWARD a
	// série cairia para 200 (só B) ou 0.
	seedTrackedSnapshot(t, db, a, 1, -1, 5, 2)

	points, err := repo.GetHistoryStats(context.Background(), a.tenant.UserID, repository.HistoryPeriodWeekly)
	require.NoError(t, err)
	require.Len(t, points, 7, "a janela semanal tem sempre 7 pontos")

	// points[0] = hoje-6 ... points[6] = hoje.
	require.Equal(t, 300, points[2].Members,
		"hoje-4: B ainda não tinha baseline, mas o carry-backward evita o degrau")
	require.Equal(t, 300, points[0].Members, "antes de qualquer observação a linha é reta")
	require.Equal(t, 310, points[3].Members, "hoje-3: as duas observações do dia")
	require.Equal(t, 310, points[5].Members, "hoje-1: sem observação, mantém o último conhecido")
	require.Equal(t, 310, points[6].Members, "hoje: idem")

	require.Equal(t, 10, points[3].Joins, "joins do bucket é soma crua")
	require.Equal(t, 5, points[5].Joins)
	require.Equal(t, 2, points[5].Leaves)
	require.Equal(t, points[5].Joins, points[5].NewMembers,
		"new_members é o campo que o client já renderiza como 'novos membros'")

	total := 0
	for _, p := range points {
		total += p.Joins
	}
	require.Equal(t, 15, total, "nenhuma entrada pode ser contada duas vezes")
}

func TestDashboardHistory_SemTrackingCaiNoCalculoAntigo(t *testing.T) {
	db, _ := setupGroupMemberTestDB(t)
	repo := NewDashboardRepository(db, zap.NewNop())

	// Usuário sem nenhuma observação: não pode perder o gráfico nem ver zero.
	f := seedTenant(t, db, "sess-legacy", "333@g.us")
	err := db.Exec(context.Background(),
		`UPDATE groups SET members_count = 42 WHERE id = $1::uuid`, f.groupID)
	require.NoError(t, err)

	points, err := repo.GetHistoryStats(context.Background(), f.tenant.UserID, repository.HistoryPeriodWeekly)
	require.NoError(t, err)
	require.Len(t, points, 7)

	// O fallback é o cálculo ANTIGO, byte a byte: SUM(members_count) dos grupos
	// já criados até o bucket. Grupo criado hoje contribui 0 no passado — não é
	// bug, é o comportamento que esses usuários já viam.
	require.Equal(t, 42, points[6].Members, "fallback para SUM(groups.members_count)")
	require.Equal(t, 0, points[0].Members, "grupo ainda não existia nesse bucket")
	for _, p := range points {
		require.Equal(t, 0, p.Joins)
		require.Equal(t, 0, p.Leaves)
	}
}

func TestDashboardHistory_TodosOsPeriodosRespondem(t *testing.T) {
	db, _ := setupGroupMemberTestDB(t)
	repo := NewDashboardRepository(db, zap.NewNop())

	f := seedTenant(t, db, "sess-periods", "444@g.us")
	markTracked(t, db, f)
	seedTrackedSnapshot(t, db, f, 0, 77, 3, 1)

	for _, tc := range []struct {
		period repository.HistoryPeriod
		points int
	}{
		{repository.HistoryPeriodWeekly, 7},
		{repository.HistoryPeriodMonthly, 4},
		{repository.HistoryPeriodYearly, 12},
	} {
		points, err := repo.GetHistoryStats(context.Background(), f.tenant.UserID, tc.period)
		require.NoError(t, err, "período %s", tc.period)
		require.Len(t, points, tc.points, "período %s", tc.period)

		last := points[len(points)-1]
		require.Equal(t, 77, last.Members, "período %s: último bucket tem a observação", tc.period)
		require.Equal(t, 3, last.Joins, "período %s", tc.period)
		require.Equal(t, 1, last.Leaves, "período %s", tc.period)
		require.NotEmpty(t, last.Label, "período %s: rótulo do eixo X", tc.period)

		// Carry-backward: todo bucket anterior fica na reta do primeiro valor
		// conhecido, nunca em zero.
		require.Equal(t, 77, points[0].Members, "período %s: linha reta antes do tracking", tc.period)
	}
}
