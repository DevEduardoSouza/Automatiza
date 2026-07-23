package connectrpc

import (
	"context"
	"testing"
	"time"

	"automatiza-go/internal/auth"
	groupsv1 "automatiza-go/internal/gen/groups/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMetricsRepo embute a interface: só GetMemberMetrics é exercitado, e
// qualquer outro método chamado por engano estoura em vez de passar batido.
type fakeMetricsRepo struct {
	repository.GroupMemberRepository
	metrics *models.GroupMemberMetrics
	gotDays int
	gotUser string
}

func (f *fakeMetricsRepo) GetMemberMetrics(_ context.Context, userID string, days int, _ string) (*models.GroupMemberMetrics, error) {
	f.gotUser = userID
	f.gotDays = days
	return f.metrics, nil
}

func metricsCtx() context.Context {
	return context.WithValue(context.Background(), UserContextKey, &auth.User{ID: "user-1"})
}

func callMetrics(t *testing.T, h *GroupsHandler, days int32) *groupsv1.GetGroupMemberMetricsResponse {
	t.Helper()
	resp, err := h.GetGroupMemberMetrics(metricsCtx(),
		connect.NewRequest(&groupsv1.GetGroupMemberMetricsRequest{Days: days}))
	require.NoError(t, err)
	return resp.Msg
}

func TestGetGroupMemberMetrics_RequiresAuth(t *testing.T) {
	h := &GroupsHandler{}
	_, err := h.GetGroupMemberMetrics(context.Background(),
		connect.NewRequest(&groupsv1.GetGroupMemberMetricsRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// Sem repositório configurado a tela de grupos NÃO pode quebrar: o RPC degrada
// para "coletando" em vez de estourar.
func TestGetGroupMemberMetrics_NoRepoDegrades(t *testing.T) {
	h := &GroupsHandler{}
	msg := callMetrics(t, h, 0)
	assert.Equal(t, groupsv1.MemberDataStatus_MEMBER_DATA_STATUS_COLLECTING, msg.GetStatus())
	assert.Nil(t, msg.ActiveMembers)
}

// O caso que mais importa: sem NENHUM grupo coberto, a resposta é COLLECTING e
// active_members vem AUSENTE. Devolver 0 seria afirmar que a conta não tem
// membros, o que é falso — ela só não foi lida ainda.
func TestGetGroupMemberMetrics_NoCoverageIsCollectingNotZero(t *testing.T) {
	repo := &fakeMetricsRepo{metrics: &models.GroupMemberMetrics{
		GroupsTotal:    28022,
		GroupsWithData: 0,
		ActiveMembers:  nil,
	}}
	h := &GroupsHandler{groupMemberRepo: repo}

	msg := callMetrics(t, h, 0)
	assert.Equal(t, groupsv1.MemberDataStatus_MEMBER_DATA_STATUS_COLLECTING, msg.GetStatus())
	assert.Nil(t, msg.ActiveMembers, "ausência de dado NUNCA vira zero")
	assert.Equal(t, int32(28022), msg.GetGroupsTotal())
	assert.Equal(t, int32(0), msg.GetGroupsWithData())
}

func TestGetGroupMemberMetrics_MapsSeriesPreservingNulls(t *testing.T) {
	active := 49508
	observed := 1200
	repo := &fakeMetricsRepo{metrics: &models.GroupMemberMetrics{
		GroupsTotal:    100,
		GroupsWithData: 40,
		ActiveMembers:  &active,
		Series: []models.GroupMemberDailyPoint{
			// Dia SEM observação: members_count nil precisa continuar ausente.
			{Date: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC), Joins: 3, Leaves: 1},
			{
				Date: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC), MembersCount: &observed,
				Joins: 5, Leaves: 2, GroupsObserved: 40,
			},
		},
	}}
	h := &GroupsHandler{groupMemberRepo: repo}

	msg := callMetrics(t, h, 7)

	assert.Equal(t, groupsv1.MemberDataStatus_MEMBER_DATA_STATUS_OK, msg.GetStatus())
	require.NotNil(t, msg.ActiveMembers)
	assert.Equal(t, int32(49508), msg.GetActiveMembers())
	assert.Equal(t, "user-1", repo.gotUser, "o escopo de tenant precisa chegar ao repositório")

	require.Len(t, msg.GetSeries(), 2)
	assert.Equal(t, "2026-07-22", msg.GetSeries()[0].GetDate())
	assert.Nil(t, msg.GetSeries()[0].MembersCount, "dia sem observação segue ausente, não 0")
	assert.Equal(t, int32(3), msg.GetSeries()[0].GetJoins())

	require.NotNil(t, msg.GetSeries()[1].MembersCount)
	assert.Equal(t, int32(1200), msg.GetSeries()[1].GetMembersCount())
	// groups_observed é o que impede ler um salto de COBERTURA como crescimento.
	assert.Equal(t, int32(40), msg.GetSeries()[1].GetGroupsObserved())
}

func TestGetGroupMemberMetrics_ClampsWindow(t *testing.T) {
	repo := &fakeMetricsRepo{metrics: &models.GroupMemberMetrics{GroupsTotal: 1}}
	h := &GroupsHandler{groupMemberRepo: repo}

	callMetrics(t, h, 0)
	assert.Equal(t, metricsDefaultDays, repo.gotDays, "0 cai no default")

	callMetrics(t, h, -5)
	assert.Equal(t, metricsDefaultDays, repo.gotDays, "negativo cai no default")

	callMetrics(t, h, 9999)
	assert.Equal(t, metricsMaxDays, repo.gotDays, "janela larga é limitada pelo teto")
}

func TestGetGroupMemberMetrics_MapsRankingAndDdd(t *testing.T) {
	members := 300
	repo := &fakeMetricsRepo{metrics: &models.GroupMemberMetrics{
		GroupsTotal:    10,
		GroupsWithData: 10,
		TopGrowth: []models.GroupMemberRankingEntry{
			{GroupID: "g1", GroupName: "Ofertas SP", Joins: 40, Leaves: 5, Net: 35, MembersCount: &members},
		},
		TopLoss: []models.GroupMemberRankingEntry{
			{GroupID: "g2", GroupName: "Achados RJ", Joins: 2, Leaves: 20, Net: -18},
		},
		DDDDistribution: []models.DDDDistributionEntry{{DDD: "11", Members: 900}},
	}}
	h := &GroupsHandler{groupMemberRepo: repo}

	msg := callMetrics(t, h, 30)

	require.Len(t, msg.GetTopGrowth(), 1)
	assert.Equal(t, int32(35), msg.GetTopGrowth()[0].GetNet())
	assert.Equal(t, int32(300), msg.GetTopGrowth()[0].GetMembersCount())

	require.Len(t, msg.GetTopLoss(), 1)
	assert.Equal(t, int32(-18), msg.GetTopLoss()[0].GetNet())
	assert.Nil(t, msg.GetTopLoss()[0].MembersCount, "grupo sem observação não inventa contagem")

	require.Len(t, msg.GetDddDistribution(), 1)
	assert.Equal(t, "11", msg.GetDddDistribution()[0].GetDdd())
	assert.Equal(t, int32(900), msg.GetDddDistribution()[0].GetMembers())
}
