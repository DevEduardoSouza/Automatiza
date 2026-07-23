package models

import "time"

// Tipos da fundação histórica de membros de grupo (Melhoria 2, Fase A).
//
// Contrato de privacidade: NENHUM tipo aqui carrega identificador bruto
// (participant.id, pn, telefone). A conversão bruto -> hash acontece uma única
// vez em internal/services/groupmembers/identity.go e só o hash atravessa a
// fronteira do repositório.

// Tipos de evento aceitos no payload group.v2.participants
// (waha-plus-openapi.json, GroupV2ParticipantsEvent.type).
const (
	GroupMemberEventJoin    = "join"
	GroupMemberEventLeave   = "leave"
	GroupMemberEventPromote = "promote"
	GroupMemberEventDemote  = "demote"
)

// Origem do primeiro avistamento de uma projeção.
const (
	GroupMemberSourceWebhook        = "webhook"
	GroupMemberSourceReconciliation = "reconciliation"
)

// Sufixos de identidade do participante (GroupParticipant.id).
const (
	GroupMemberIDTypeCUs = "c_us"
	GroupMemberIDTypeLID = "lid"
)

// TrackingTenant é a fronteira de tenant resolvida a partir do nome da sessão
// WAHA. Toda escrita de tracking exige um destes: o user_id entra no predicado
// de todas as queries para impedir atribuição cruzada entre contas.
type TrackingTenant struct {
	UserID string
	// InstancePK é instances.id (INTEGER, FK do ledger).
	InstancePK int
	// SessionName é instances.instance_id (o nome da sessão no WAHA).
	SessionName string
}

// GroupMemberIdentity é a identidade pseudonimizada de um participante.
//
// ParticipantHash deriva SEMPRE de participant.id (campo obrigatório do
// contrato), nunca de pn — assim um @lid visto primeiro sem telefone continua
// na MESMA projeção quando a reconciliação depois trouxer o pn.
type GroupMemberIdentity struct {
	ParticipantHash string
	PhoneHash       *string
	IDType          string
	DDD             *string
}

// GroupMemberEventParticipant é um participante já validado e pseudonimizado
// dentro de um envelope.
type GroupMemberEventParticipant struct {
	GroupMemberIdentity
	Role string
}

// GroupMemberWebhookEvent é um envelope group.v2.participants já validado.
type GroupMemberWebhookEvent struct {
	// ProviderEventID é o ID original do envelope (ULID minúsculo, prefixo
	// evt_ preservado como veio). Nunca o EventID interno de fallback.
	ProviderEventID string
	// ProviderCreatedAt vem do timestamp (ms) do envelope.
	ProviderCreatedAt time.Time
	WAGroupID         string
	EventType         string
	// OccurredAt vem do timestamp (s) do payload.
	OccurredAt time.Time
	// AppliedAt é o instante em que NÓS aplicamos o fato — comparado com o
	// observation_started_at da reconciliação para decidir quem vence.
	AppliedAt    time.Time
	Participants []GroupMemberEventParticipant
}

// RecordBatchResult resume o efeito de um envelope no ledger.
type RecordBatchResult struct {
	Inserted   int
	Duplicates int
}

// GroupReconciliationCandidate é um grupo reivindicado para reconciliação,
// com o chip conectado escolhido para fazer a chamada WAHA.
type GroupReconciliationCandidate struct {
	GroupID     string
	UserID      string
	WAGroupID   string
	InstancePK  int
	SessionName string
}

// GroupReconciliationMember é um participante presente na fotografia atual do
// grupo devolvida pelo WAHA, já pseudonimizado.
type GroupReconciliationMember struct {
	GroupMemberIdentity
	Role string
}

// GroupMemberDailySnapshot é o agregado diário de um grupo.
// MembersCount é observação PONTUAL em MembersObservedAt, nunca "fim do dia";
// permanece nil até a primeira reconciliação bem-sucedida do grupo.
type GroupMemberDailySnapshot struct {
	UserID            string
	GroupID           *string
	WAGroupID         string
	SnapshotDate      time.Time
	MembersCount      *int
	MembersObservedAt *time.Time
	JoinsCount        int
	LeavesCount       int
}

// =============================================================================
// Métricas (Fase B) — leitura do agregado diário e da projeção
// =============================================================================

// GroupMemberDailyPoint é um dia da série de membros.
type GroupMemberDailyPoint struct {
	Date time.Time
	// MembersCount é nil quando NENHUM grupo do recorte teve observação
	// naquele dia. Nil nunca deve ser renderizado como zero.
	MembersCount *int
	Joins        int
	Leaves       int
	// GroupsObserved evita a leitura errada mais provável desta série: um
	// salto de COBERTURA (mais grupos reconciliados) parecendo crescimento
	// real de membros.
	GroupsObserved int
}

// GroupMemberRankingEntry é uma linha do ranking de crescimento/perda.
type GroupMemberRankingEntry struct {
	GroupID      string
	GroupName    string
	Joins        int
	Leaves       int
	Net          int
	MembersCount *int
}

// DDDDistributionEntry conta membros ativos por DDD.
type DDDDistributionEntry struct {
	DDD     string
	Members int
}

// GroupMemberMetrics é o pacote de leitura que alimenta a tela de métricas.
type GroupMemberMetrics struct {
	GroupsTotal    int
	GroupsWithData int
	// ActiveMembers é nil enquanto não houver nenhum baseline: sem cobertura,
	// "0 membros" seria uma afirmação falsa.
	ActiveMembers   *int
	Series          []GroupMemberDailyPoint
	TopGrowth       []GroupMemberRankingEntry
	TopLoss         []GroupMemberRankingEntry
	DDDDistribution []DDDDistributionEntry
}
