package connectrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	supportv1 "automatiza-go/internal/gen/support/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/repository/postgres"
	"automatiza-go/internal/services/loadbalancer"

	"connectrpc.com/connect"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Limites do painel. Existem para que um atendente curioso (ou um bug de UI em
// loop) nao consiga transformar uma tela de leitura em carga sobre o WAHA/DB.
const (
	supportSearchDefaultLimit = 20
	supportSearchMaxLimit     = 50

	supportMessagesDefaultLimit = 30
	supportMessagesMaxLimit     = 100
	// Janela de historico do chat. Sem recorte, o endpoint do WAHA pagina o
	// historico inteiro do grupo (ver waha_adapter.go:1736-1737).
	supportMessagesLookback = 7 * 24 * time.Hour

	supportActivityDefaultWindow = 5
	supportActivityMaxWindow     = 60

	// Quanto tempo sem um envio ate considerarmos que uma automacao
	// event-driven (monitoring/offer_generate) parou de disparar. Essas nao
	// passam pelo scheduler, entao nao ha run aberto para consultar — o unico
	// sinal e a atividade recente.
	supportEventDrivenIdleAfter = 3 * time.Minute
)

// Filas asynq inspecionadas. Sao GLOBAIS do sistema: o Inspector nao filtra por
// payload, entao nao existe "a fila do usuario X" (ver GetUserActivity).
var supportInspectedQueues = []string{"critical", "webhooks", "messages", "default", "reconciliation"}

// SupportHandler implementa o painel de suporte (somente leitura).
//
// Separado do AdminHandler de proposito: aquele concentra acoes destrutivas
// (reset de senha, troca de role, bloqueio) e continua admin-only. Aqui a role
// 'support' entra, entao nada que escreva pode morar neste arquivo.
type SupportHandler struct {
	supportRepo    *postgres.SupportRepository
	userRoleRepo   repository.UserRoleRepository
	instanceRepo   repository.InstanceRepository
	automationRepo repository.AutomationRepository
	groupRepo      repository.GroupRepository
	linkListRepo   repository.MLLinkListRepository
	waProvider     whatsapp.WhatsAppProvider
	asynqInspector *asynq.Inspector
	// redisClient e lbService alimentam a aba Dashboard (saude do sistema).
	// Ambos podem ser nil (o handler degrada sem quebrar) — o Dashboard e
	// leitura auxiliar, nao pode derrubar o resto do painel.
	redisClient *redis.Client
	lbService   loadbalancer.Service
	// sendLimitPerMinute e a velocidade CONFIGURADA por chip
	// (WHATSAPP_SEND_LIMIT_PER_MINUTE) — referencia para comparar com a real.
	sendLimitPerMinute int
	logger             *zap.Logger
}

// NewSupportHandler constroi o handler do painel de suporte.
func NewSupportHandler(
	supportRepo *postgres.SupportRepository,
	userRoleRepo repository.UserRoleRepository,
	instanceRepo repository.InstanceRepository,
	automationRepo repository.AutomationRepository,
	groupRepo repository.GroupRepository,
	linkListRepo repository.MLLinkListRepository,
	waProvider whatsapp.WhatsAppProvider,
	asynqInspector *asynq.Inspector,
	redisClient *redis.Client,
	lbService loadbalancer.Service,
	sendLimitPerMinute int,
	logger *zap.Logger,
) *SupportHandler {
	return &SupportHandler{
		supportRepo:        supportRepo,
		userRoleRepo:       userRoleRepo,
		instanceRepo:       instanceRepo,
		automationRepo:     automationRepo,
		groupRepo:          groupRepo,
		linkListRepo:       linkListRepo,
		waProvider:         waProvider,
		asynqInspector:     asynqInspector,
		redisClient:        redisClient,
		lbService:          lbService,
		sendLimitPerMinute: sendLimitPerMinute,
		logger:             logger,
	}
}

// requireSupport exige role 'support' ou 'admin'.
//
// Nao reusa AdminHandler.requireAdmin/IsAdmin: aquele compara com a string
// 'admin' literal e mudar seu comportamento afrouxaria, de uma vez, todos os
// RPCs destrutivos do AdminService.
func (h *SupportHandler) requireSupport(ctx context.Context) error {
	user := GetUserFromContext(ctx)
	if user == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	roles, err := h.userRoleRepo.GetAllUserRoles(ctx, user.ID)
	if err != nil {
		// Fail-closed: erro ao ler papel nunca vira acesso concedido.
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check roles: %w", err))
	}

	for _, r := range roles {
		if r == models.RoleSupport || r == models.RoleAdmin {
			return nil
		}
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("support access required"))
}

func clampInt(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// ============================================================
// SearchUsers
// ============================================================

func (h *SupportHandler) SearchUsers(
	ctx context.Context,
	req *connect.Request[supportv1.SearchUsersRequest],
) (*connect.Response[supportv1.SearchUsersResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}

	query := req.Msg.GetQuery()
	if query == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("query is required"))
	}
	limit := clampInt(int(req.Msg.GetLimit()), supportSearchDefaultLimit, supportSearchMaxLimit)

	// Pede limit+1 para saber se truncou sem uma segunda contagem.
	rows, err := h.supportRepo.SearchUsers(ctx, query, limit+1)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to search users: %w", err))
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	users := make([]*supportv1.SupportUserSummary, 0, len(rows))
	for i := range rows {
		r := rows[i]
		u := &supportv1.SupportUserSummary{
			Id:                r.ID,
			Email:             r.Email,
			Name:              r.Name,
			PhoneNumber:       r.PhoneNumber,
			Plan:              r.Plan,
			BillingStatus:     r.BillingStatus,
			IsActive:          r.IsActive,
			ChipsTotal:        int32(r.ChipsTotal),
			ChipsConnected:    int32(r.ChipsConnected),
			AutomationsTotal:  int32(r.AutomationsTotal),
			AutomationsActive: int32(r.AutomationsActive),
			CreatedAt:         timePtrToString(r.CreatedAt),
			MatchedOn:         matchedFields(r),
		}
		users = append(users, u)
	}

	return connect.NewResponse(&supportv1.SearchUsersResponse{
		Users:   users,
		HasMore: hasMore,
	}), nil
}

func matchedFields(r postgres.SupportUserRow) []supportv1.MatchField {
	var out []supportv1.MatchField
	if r.MatchedEmail {
		out = append(out, supportv1.MatchField_MATCH_FIELD_EMAIL)
	}
	if r.MatchedName {
		out = append(out, supportv1.MatchField_MATCH_FIELD_NAME)
	}
	if r.MatchedPhone {
		out = append(out, supportv1.MatchField_MATCH_FIELD_PHONE)
	}
	if r.MatchedInstance {
		out = append(out, supportv1.MatchField_MATCH_FIELD_INSTANCE)
	}
	return out
}

func timePtrToString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// ============================================================
// GetUserOverview
// ============================================================

func (h *SupportHandler) GetUserOverview(
	ctx context.Context,
	req *connect.Request[supportv1.GetUserOverviewRequest],
) (*connect.Response[supportv1.GetUserOverviewResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	profile, err := h.supportRepo.GetUserProfile(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found: %w", err))
	}

	instances, err := h.instanceRepo.GetUserInstances(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instances: %w", err))
	}

	automations, err := h.automationRepo.GetUserAutomations(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get automations: %w", err))
	}

	groups, err := h.groupRepo.GetUserGroups(ctx, userID, false)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get groups: %w", err))
	}

	lists, err := h.linkListRepo.GetListsByUserID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get link lists: %w", err))
	}

	groupsByInstance, err := h.supportRepo.GetGroupCountByInstance(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count groups per chip: %w", err))
	}

	listStats, err := h.supportRepo.GetLinkListItemStats(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get list stats: %w", err))
	}

	// instances.id (numerico) -> instance_id (nome da sessao WAHA).
	// groups.participating_instance_ids guarda o id numerico, mas quem atende
	// raciocina pelo nome da sessao / telefone do chip.
	instNameByNumericID := make(map[string]string, len(instances))
	for i := range instances {
		instNameByNumericID[strconv.Itoa(instances[i].ID)] = instances[i].InstanceID
	}

	// wa_group_id -> nome, para exibir "monitorando/enviando para <nome do grupo>"
	// em vez do id cru (numero@g.us).
	groupNameByWAID := make(map[string]string, len(groups))
	for i := range groups {
		groupNameByWAID[groups[i].WAGroupID] = groups[i].Name
	}

	chips := make([]*supportv1.SupportChip, 0, len(instances))
	for i := range instances {
		inst := instances[i]
		chips = append(chips, &supportv1.SupportChip{
			InstanceId:   inst.InstanceID,
			FriendlyName: inst.FriendlyName,
			Status:       inst.Status,
			PhoneNumber:  inst.PhoneNumber,
			UpdatedAt:    strPtrOrNil(inst.UpdatedAt),
			GroupsCount:  int32(groupsByInstance[inst.InstanceID]),
		})
	}

	protoAutos := make([]*supportv1.SupportAutomation, 0, len(automations))
	for i := range automations {
		a := automations[i]
		protoAutos = append(protoAutos, &supportv1.SupportAutomation{
			Id:              a.ID,
			Name:            a.Name,
			Type:            automationType(&a),
			Status:          a.Status,
			IntervalMinutes: intPtrToInt32Ptr(a.IntervalMinutes),
			StartTime:       a.StartTime,
			EndTime:         a.EndTime,
			LastRunAt:       a.LastRunAt,
			NextRunAt:       a.NextRunAt,
			LastError:       a.LastError,
			LastErrorAt:     a.LastErrorAt,
			// automations.instance_ids e int4[]: guarda instances.id, nao o
			// nome da sessao (ver models/automation.go:108-114). Traduzimos
			// para o instance_id textual, que e o que a UI mostra.
			InstanceIds:     resolveInstanceNames(a.InstanceIDs, instNameByNumericID),
			SendGroupsCount: int32(len(a.SendGroups)),
			MlLinkListId:    a.MlLinkListID,
			CreatedAt:       strPtrOrNil(a.CreatedAt),
			MonitorGroups:   resolveGroupRefs(a.MonitorGroupIDs, groupNameByWAID),
			SendGroups:      resolveGroupRefs(a.SendGroups, groupNameByWAID),
		})
	}

	protoLists := make([]*supportv1.SupportLinkList, 0, len(lists))
	for i := range lists {
		l := lists[i]
		stats := listStats[l.ID]
		protoLists = append(protoLists, &supportv1.SupportLinkList{
			Id:            l.ID,
			Name:          l.Name,
			Description:   l.Description,
			IsActive:      l.IsActive,
			ItemsTotal:    int32(stats[0]),
			ItemsConsumed: int32(stats[1]),
			ItemsPending:  int32(stats[2]),
			CreatedAt:     strPtrOrNil(l.CreatedAt),
		})
	}

	protoGroups := make([]*supportv1.SupportGroup, 0, len(groups))
	for i := range groups {
		g := groups[i]
		names := make([]string, 0, len(g.ParticipatingInstanceIDs))
		for _, numericID := range g.ParticipatingInstanceIDs {
			if name, ok := instNameByNumericID[numericID]; ok {
				names = append(names, name)
			}
		}
		name := g.Name
		protoGroups = append(protoGroups, &supportv1.SupportGroup{
			WaGroupId:                g.WAGroupID,
			Name:                     &name,
			IsAdmin:                  g.IsAdmin,
			ParticipatingInstanceIds: names,
		})
	}

	return connect.NewResponse(&supportv1.GetUserOverviewResponse{
		Profile: &supportv1.SupportUserProfile{
			Id:                      profile.ID,
			Email:                   profile.Email,
			Name:                    profile.Name,
			PhoneNumber:             profile.PhoneNumber,
			Plan:                    profile.Plan,
			BillingStatus:           profile.BillingStatus,
			BillingSuspensionReason: profile.BillingSuspensionReason,
			IsActive:                profile.IsActive,
			MaxInstances:            int32(profile.MaxInstances),
			MaxAutomations:          int32(profile.MaxAutomations),
			MaxGroups:               int32(profile.MaxGroups),
			LastLoginAt:             timePtrToString(profile.LastLoginAt),
			CreatedAt:               timePtrToString(profile.CreatedAt),
		},
		Chips:       chips,
		Automations: protoAutos,
		LinkLists:   protoLists,
		Groups:      protoGroups,
	}), nil
}

// resolveInstanceNames traduz instances.id (numerico, como string) para o
// instance_id textual — a sessao WAHA que o atendente reconhece.
//
// Um id que nao resolve significa chip apagado com a automacao ainda
// apontando para ele: devolvemos o numero cru em vez de omitir, porque some
// justamente o sintoma que explica "essa automacao nao dispara mais".
func resolveInstanceNames(numericIDs []string, nameByID map[string]string) []string {
	if len(numericIDs) == 0 {
		return nil
	}
	out := make([]string, 0, len(numericIDs))
	for _, id := range numericIDs {
		if name, ok := nameByID[id]; ok {
			out = append(out, name)
			continue
		}
		out = append(out, id)
	}
	return out
}

// resolveGroupRefs transforma wa_group_ids em refs com nome. Um grupo sem nome
// no mapa (nao sincronizado) mantem o id cru — some justamente o grupo que o
// cliente pode nem ter sincronizado, e o atendente precisa ver que ele existe.
func resolveGroupRefs(waGroupIDs []string, nameByWAID map[string]string) []*supportv1.SupportGroupRef {
	if len(waGroupIDs) == 0 {
		return nil
	}
	out := make([]*supportv1.SupportGroupRef, 0, len(waGroupIDs))
	for _, id := range waGroupIDs {
		ref := &supportv1.SupportGroupRef{WaGroupId: id}
		if name, ok := nameByWAID[id]; ok && name != "" {
			ref.Name = &name
		}
		out = append(out, ref)
	}
	return out
}

// automationType normaliza o tipo. `type` e nullable no banco; o dispatcher
// trata ausencia como 'search', entao o painel mostra o mesmo.
func automationType(a *models.Automation) string {
	if a.Type == nil || *a.Type == "" {
		return "search"
	}
	return *a.Type
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtrToInt32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	n := int32(*v)
	return &n
}

// ============================================================
// GetGroupMessages
// ============================================================

func (h *SupportHandler) GetGroupMessages(
	ctx context.Context,
	req *connect.Request[supportv1.GetGroupMessagesRequest],
) (*connect.Response[supportv1.GetGroupMessagesResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	instanceID := req.Msg.GetInstanceId()
	groupID := req.Msg.GetGroupId()
	if userID == "" || instanceID == "" || groupID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("user_id, instance_id and group_id are required"))
	}

	// Confirma que o chip e o grupo sao mesmo daquele usuario antes de ler o
	// chat. Sem isso o RPC viraria um leitor de qualquer grupo de qualquer
	// conta para quem soubesse um par (sessao, grupo) — inclusive de contas
	// que o atendente nao esta atendendo.
	inst, err := h.instanceRepo.GetInstanceByUserAndName(ctx, userID, instanceID)
	if err != nil || inst == nil {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("instance does not belong to this user"))
	}
	group, err := h.groupRepo.GetGroupByWAGroupID(ctx, userID, groupID)
	if err != nil || group == nil {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("group does not belong to this user"))
	}

	limit := clampInt(int(req.Msg.GetLimit()), supportMessagesDefaultLimit, supportMessagesMaxLimit)
	since := time.Now().Add(-supportMessagesLookback)

	result, err := h.waProvider.GetMessages(ctx, whatsapp.GetMessagesInput{
		SessionName: instanceID,
		ChatID:      groupID,
		Limit:       limit,
		// Explicito e sempre false: o default da API do WAHA e true, e deixar
		// implicito faria o painel baixar a midia de cada mensagem (o mesmo
		// tropeco ja documentado em waha_adapter.go:1722-1725). O modal mostra
		// so o texto e um marcador de midia.
		DownloadMedia: false,
		Filter: &whatsapp.MessageFilter{
			FromTimestamp: &since,
		},
	})
	if err != nil {
		return nil, wahaReadError(err)
	}

	msgs := make([]*supportv1.SupportMessage, 0, len(result.Messages))
	for i := range result.Messages {
		m := result.Messages[i]
		msgs = append(msgs, &supportv1.SupportMessage{
			Id:        m.ID,
			Body:      strPtrOrNil(m.Body),
			FromMe:    m.FromMe,
			Author:    strPtrOrNil(m.Sender),
			Timestamp: m.Timestamp.Unix(),
			HasMedia:  m.HasMedia,
			MediaType: strPtrOrNil(m.MediaType),
			AckName:   strPtrOrNil(ackName(m.Ack)),
		})
	}

	return connect.NewResponse(&supportv1.GetGroupMessagesResponse{Messages: msgs}), nil
}

// wahaReadError traduz a falha do WAHA para um codigo Connect com a semantica
// de retry certa.
//
// Isto importa mais do que parece: o transport do frontend re-tenta
// automaticamente em CodeUnavailable (client/src/lib/connect-transport.ts:111).
// Devolver Unavailable para "sessao nao existe" — que e permanente — faz o
// painel martelar o WAHA 3x e ficar preso carregando, em vez de dizer ao
// atendente o que houve. Chip caido e o caso mais comum aqui, entao esse
// caminho precisa ser terminal e legivel.
func wahaReadError(err error) *connect.Error {
	var provErr *whatsapp.ProviderError
	if errors.As(err, &provErr) {
		switch provErr.Code {
		case whatsapp.ErrCodeSessionNotFound, whatsapp.ErrCodeGroupNotFound:
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("chip nao pareado no WAHA ou grupo inacessivel: %w", err))
		case whatsapp.ErrCodeTimeout, whatsapp.ErrCodeNetworkError, whatsapp.ErrCodeRateLimited:
			// Transitorio: aqui re-tentar de fato ajuda.
			return connect.NewError(connect.CodeUnavailable,
				fmt.Errorf("WAHA indisponivel no momento: %w", err))
		}
		// Sessao inexistente chega como 422, nao 404 (verificado contra o WAHA:
		// GET /api/{session}/chats/{chatId}/messages devolve
		// 422 {"error":"Session \"x\" does not exist"}). O mapper do adapter so
		// traduz 404 -> SESSION_NOT_FOUND (mapper.go:797), entao 422 cai em
		// ErrCodeProviderError e chegaria aqui como erro generico.
		if code, ok := provErr.Details["statusCode"].(int); ok && code == http.StatusUnprocessableEntity {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("chip nao pareado no WAHA: %w", err))
		}
	}
	// Desconhecido: Internal nao dispara o retry do cliente.
	return connect.NewError(connect.CodeInternal,
		fmt.Errorf("failed to read group messages: %w", err))
}

func ackName(ack int) string {
	switch whatsapp.AckStatus(ack) {
	case whatsapp.AckPending:
		return "pending"
	case whatsapp.AckSent:
		return "sent"
	case whatsapp.AckDelivered:
		return "delivered"
	case whatsapp.AckRead:
		return "read"
	default:
		return ""
	}
}

// ============================================================
// GetUserActivity
// ============================================================

func (h *SupportHandler) GetUserActivity(
	ctx context.Context,
	req *connect.Request[supportv1.GetUserActivityRequest],
) (*connect.Response[supportv1.GetUserActivityResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}
	windowMin := clampInt(int(req.Msg.GetWindowMinutes()), supportActivityDefaultWindow, supportActivityMaxWindow)
	window := time.Duration(windowMin) * time.Minute

	activity, err := h.supportRepo.GetUserActivity(ctx, userID, window)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get activity: %w", err))
	}

	perAuto, err := h.supportRepo.GetAutomationActivity(ctx, userID, window)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get automation activity: %w", err))
	}

	running, err := h.supportRepo.GetRunningAutomationIDs(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get running automations: %w", err))
	}

	automations, err := h.automationRepo.GetUserAutomations(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get automations: %w", err))
	}
	typeByID := make(map[string]string, len(automations))
	for i := range automations {
		typeByID[automations[i].ID] = automationType(&automations[i])
	}

	now := time.Now()
	acts := make([]*supportv1.SupportAutomationActivity, 0, len(perAuto))
	for i := range perAuto {
		a := perAuto[i]
		isDispatching, signal := dispatchState(typeByID[a.AutomationID], running[a.AutomationID], a.LastDispatchAt, now)
		acts = append(acts, &supportv1.SupportAutomationActivity{
			AutomationId:   a.AutomationID,
			IsDispatching:  isDispatching,
			Signal:         signal,
			LastDispatchAt: timePtrToString(a.LastDispatchAt),
			SentInWindow:   int32(a.SentInWindow),
			FailedInWindow: int32(a.FailedInWindow),
			QueuedNow:      int32(a.QueuedNow),
		})
	}

	var sentPerMinute float64
	if windowMin > 0 {
		sentPerMinute = float64(activity.SentInWindow) / float64(windowMin)
	}

	return connect.NewResponse(&supportv1.GetUserActivityResponse{
		SentPerMinute:       sentPerMinute,
		ConfiguredPerMinute: float64(h.sendLimitPerMinute),
		SentInWindow:        int32(activity.SentInWindow),
		FailedInWindow:      int32(activity.FailedInWindow),
		QueuedNow:           int32(activity.QueuedNow),
		WindowMinutes:       int32(windowMin),
		Automations:         acts,
		Queues:              h.queueDepths(),
		MeasuredAt:          now.Format(time.RFC3339),
	}), nil
}

// dispatchState decide se a automacao esta disparando, e diz por qual sinal.
//
// Os dois tipos de automacao respondem a perguntas diferentes:
//   - search/list rodam por cron (tick de 1 min) e abrem automation_run, entao
//     "rodando agora" e um fato consultavel;
//   - monitoring/offer_generate sao event-driven por webhook — o scheduler as
//     ignora (scheduler.go:112-116) e elas nunca abrem run. Para essas, o
//     unico sinal honesto e "houve envio ha pouco". Um espelho parado nao esta
//     necessariamente quebrado: pode ser que o grupo-fonte esteja quieto.
func dispatchState(autoType string, hasRunningRun bool, lastDispatch *time.Time, now time.Time) (bool, string) {
	switch autoType {
	case "monitoring", "offer_generate":
		if lastDispatch != nil && now.Sub(*lastDispatch) < supportEventDrivenIdleAfter {
			return true, "recent_dispatch"
		}
		return false, "event_driven_idle"
	default:
		if hasRunningRun {
			return true, "run_active"
		}
		return false, "idle"
	}
}

// queueDepths le as filas asynq. Sao globais do sistema — nao existe recorte
// por usuario (o Inspector nao filtra por payload), entao a UI apresenta isso
// como saude da plataforma, nao como backlog do cliente.
//
// Fail-soft: se o Inspector falhar, o painel perde o bloco de filas mas
// continua mostrando velocidade e automacoes, que e o que responde "meu
// cliente esta disparando?".
func (h *SupportHandler) queueDepths() []*supportv1.SupportQueueDepth {
	if h.asynqInspector == nil {
		return nil
	}
	out := make([]*supportv1.SupportQueueDepth, 0, len(supportInspectedQueues))
	for _, q := range supportInspectedQueues {
		info, err := h.asynqInspector.GetQueueInfo(q)
		if err != nil {
			// Fila inexistente/vazia devolve erro no asynq; nao e falha do painel.
			continue
		}
		out = append(out, &supportv1.SupportQueueDepth{
			Queue:     q,
			Pending:   int32(info.Pending),
			Active:    int32(info.Active),
			Scheduled: int32(info.Scheduled),
			Retry:     int32(info.Retry),
		})
	}
	return out
}

// ============================================================
// GetSystemHealth (aba Dashboard)
// ============================================================

// Limiares do semaforo geral. Vindos da realidade dos incidentes do runbook:
// Redis estourando maxmemory, WAHA perto do teto de sessoes, e filas
// empilhando (pending > 5k = backlog). Sao poucos e claros de proposito — um
// semaforo para nao tecnico nao pode viver amarelo por ruido cronico.
const (
	healthRedisWarnPct = 75.0
	healthRedisCritPct = 90.0
	healthWahaWarnPct  = 85.0
	healthWahaCritPct  = 95.0
	healthQueueWarn    = 1000
	healthQueueCrit    = 5000
	healthSessionsWarn = 0.7 // conectadas/total abaixo disto = atencao
	healthSessionsCrit = 0.4 // ...abaixo disto = critico
)

const (
	statusOK   = "ok"
	statusWarn = "atencao"
	statusCrit = "critico"
)

func statusRank(s string) int {
	switch s {
	case statusCrit:
		return 2
	case statusWarn:
		return 1
	default:
		return 0
	}
}

// worst devolve a pior sinaleira entre as informadas.
func worst(statuses ...string) string {
	out := statusOK
	for _, s := range statuses {
		if statusRank(s) > statusRank(out) {
			out = s
		}
	}
	return out
}

func pctStatus(pct, warn, crit float64) string {
	switch {
	case pct >= crit:
		return statusCrit
	case pct >= warn:
		return statusWarn
	default:
		return statusOK
	}
}

// GetSystemHealth agrega saude da VPS e da aplicacao para a aba Dashboard.
//
// Filosofia: cada fonte degrada sozinha. Se o Redis nao responder, o card do
// Redis vem indisponivel mas o resto do dashboard continua — um painel de
// saude que cai junto com o que ele monitora nao serve para nada.
func (h *SupportHandler) GetSystemHealth(
	ctx context.Context,
	_ *connect.Request[supportv1.GetSystemHealthRequest],
) (*connect.Response[supportv1.GetSystemHealthResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}

	now := time.Now()
	resp := &supportv1.GetSystemHealthResponse{
		MeasuredAt: now.Format(time.RFC3339),
		Queues:     h.queueDepths(),
	}

	// --- Sessoes WAHA + capacidade dos servidores ---
	waha := &supportv1.WahaHealth{}
	if c, err := h.supportRepo.GetSessionStatusCounts(ctx); err == nil {
		waha.SessionsTotal = int32(c.Total)
		waha.SessionsConnected = int32(c.Connected)
		waha.SessionsDisconnected = int32(c.Disconnected)
		waha.SessionsPending = int32(c.Pending)
	}
	if h.lbService != nil {
		if stats, err := h.lbService.GetServerStats(ctx); err == nil && stats != nil {
			var cap, mapped int32
			for _, s := range stats.Servers {
				cap += int32(s.MaxUsers)
				mapped += int32(s.UserCount)
			}
			waha.Servers = int32(stats.TotalServers)
			waha.Capacity = cap
			waha.Mapped = mapped
			if cap > 0 {
				waha.CapacityPercent = float64(mapped) / float64(cap) * 100
			}
		}
	}
	resp.Waha = waha

	// --- Redis ---
	resp.Redis = h.redisHealth(ctx)

	// --- Aplicacao ---
	resp.App = h.appHealth(ctx)

	// --- Host (CPU/disco/load): coletado pela sentinela, nao pelo backend ---
	resp.Host = h.hostHealth(ctx)

	// --- Semaforo geral ---
	resp.Overall = h.overallStatus(waha, resp.Redis, resp.Queues)

	return connect.NewResponse(resp), nil
}

// redisHealth roda INFO e extrai memoria/evictions/clientes. Timeout curto: o
// dashboard nao pode travar esperando um Redis lento.
func (h *SupportHandler) redisHealth(ctx context.Context) *supportv1.RedisHealth {
	rh := &supportv1.RedisHealth{Available: false}
	if h.redisClient == nil {
		return rh
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	raw, err := h.redisClient.Info(cctx, "memory", "stats", "clients").Result()
	if err != nil {
		return rh
	}
	info := parseRedisInfo(raw)
	rh.Available = true
	rh.UsedMemoryBytes = atoiSafe(info["used_memory"])
	rh.MaxMemoryBytes = atoiSafe(info["maxmemory"])
	rh.EvictedKeys = atoiSafe(info["evicted_keys"])
	rh.ConnectedClients = int32(atoiSafe(info["connected_clients"]))
	rh.MaxmemoryPolicy = info["maxmemory_policy"]
	if rh.MaxMemoryBytes > 0 {
		rh.UsagePercent = float64(rh.UsedMemoryBytes) / float64(rh.MaxMemoryBytes) * 100
	}
	return rh
}

// hostHealth le as metricas de host (CPU/load/disco) publicadas pela sentinela
// em Redis (chave support:host_health, SETEX 300s a cada ciclo de 2min). O
// backend roda em container e NAO enxerga o disco real do host nem a carga
// agregada da VPS — por isso a fonte da verdade e a sentinela. Se a chave nao
// existir (sentinela parada/desatualizada), degrada para available=false com
// uma nota, em vez de mostrar um numero inventado.
//
// A frescura e garantida pelo proprio TTL: a chave so existe se a sentinela
// escreveu nos ultimos 300s. Chave presente => dado fresco.
func (h *SupportHandler) hostHealth(ctx context.Context) *supportv1.HostHealth {
	hh := &supportv1.HostHealth{
		Available: false,
		Note:      "CPU, disco e load da VPS sao coletados pela sentinela no host. Conecte a sentinela para exibir aqui.",
	}
	if h.redisClient == nil {
		return hh
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	raw, err := h.redisClient.Get(cctx, "support:host_health").Result()
	if err != nil {
		// redis.Nil (chave ausente/expirada) ou erro de conexao: mantem a nota.
		return hh
	}

	var payload struct {
		TS              string  `json:"ts"`
		CPUPercent      float64 `json:"cpu_percent"`
		LoadAvg1m       float64 `json:"load_avg_1m"`
		DiskUsedPercent float64 `json:"disk_used_percent"`
		DiskFreeBytes   int64   `json:"disk_free_bytes"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		h.logger.Warn("host_health: JSON invalido da sentinela", zap.Error(err))
		return hh
	}

	return &supportv1.HostHealth{
		Available:       true,
		Source:          "sentinela",
		CpuPercent:      payload.CPUPercent,
		LoadAvg_1M:      payload.LoadAvg1m,
		DiskUsedPercent: payload.DiskUsedPercent,
		DiskFreeBytes:   payload.DiskFreeBytes,
	}
}

// parseRedisInfo transforma a saida do INFO ("chave:valor\r\n") num mapa.
func parseRedisInfo(raw string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if ok {
			out[k] = v
		}
	}
	return out
}

func atoiSafe(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// appHealth monta as contagens de nivel de aplicacao. Cada agregado degrada
// para vazio se a query falhar — nunca derruba o dashboard inteiro.
func (h *SupportHandler) appHealth(ctx context.Context) *supportv1.AppHealth {
	app := &supportv1.AppHealth{}

	if types, err := h.supportRepo.GetAutomationHealthByType(ctx); err == nil {
		for _, t := range types {
			app.AutomationTypes = append(app.AutomationTypes, &supportv1.AutomationTypeHealth{
				Type:      t.Type,
				Total:     int32(t.Total),
				Active:    int32(t.Active),
				Paused:    int32(t.Paused),
				WithError: int32(t.WithError),
			})
		}
	}
	if l, err := h.supportRepo.GetListStatusCounts(ctx); err == nil {
		app.ListsTotal = int32(l.Total)
		app.ListsActive = int32(l.Active)
		app.ListsInactive = int32(l.Inactive)
	}
	if sent, failed, err := h.supportRepo.GetDispatchLastHour(ctx); err == nil {
		app.SentLastHour = int32(sent)
		app.FailedLastHour = int32(failed)
	}
	if size, err := h.supportRepo.GetDatabaseSizeBytes(ctx); err == nil {
		app.DbSizeBytes = size
	}
	return app
}

// overallStatus deriva o semaforo geral dos sinais classicos de incidente
// (Redis, capacidade WAHA, backlog de filas, sessoes derrubadas).
func (h *SupportHandler) overallStatus(
	waha *supportv1.WahaHealth,
	redis *supportv1.RedisHealth,
	queues []*supportv1.SupportQueueDepth,
) string {
	signals := []string{statusOK}

	if redis.Available && redis.MaxMemoryBytes > 0 {
		signals = append(signals, pctStatus(redis.UsagePercent, healthRedisWarnPct, healthRedisCritPct))
	}
	if waha.Servers > 0 && waha.Capacity > 0 {
		signals = append(signals, pctStatus(waha.CapacityPercent, healthWahaWarnPct, healthWahaCritPct))
	}
	// Backlog: soma pending das filas de envio.
	var pending int32
	for _, q := range queues {
		pending += q.Pending
	}
	switch {
	case pending >= healthQueueCrit:
		signals = append(signals, statusCrit)
	case pending >= healthQueueWarn:
		signals = append(signals, statusWarn)
	}
	// Sessoes derrubadas em massa e outage real.
	if waha.SessionsTotal > 0 {
		ratio := float64(waha.SessionsConnected) / float64(waha.SessionsTotal)
		switch {
		case ratio < healthSessionsCrit:
			signals = append(signals, statusCrit)
		case ratio < healthSessionsWarn:
			signals = append(signals, statusWarn)
		}
	}
	return worst(signals...)
}

// ============================================================
// ResumePausedAutomations (unica escrita do painel)
// ============================================================

// ResumePausedAutomations religa automacoes pausadas SEM motivo protetor.
//
// De proposito NAO religa:
//   - pausadas por cobranca (paused_by_billing): religar dispararia WhatsApp
//     para contas inadimplentes;
//   - com erro recente (last_error_at < 24h, ex.: credencial Shopee invalida):
//     religar so re-quebraria na hora.
//
// E a unica operacao de escrita do painel de suporte. E o espelho do incidente
// de mass-pause de 16/07 — por isso as exclusoes e a confirmacao (dry_run) na
// UI, e o log de auditoria de quem disparou.
func (h *SupportHandler) ResumePausedAutomations(
	ctx context.Context,
	req *connect.Request[supportv1.ResumePausedAutomationsRequest],
) (*connect.Response[supportv1.ResumePausedAutomationsResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}

	// dry_run: so conta, nao religa (alimenta a confirmacao da UI).
	if req.Msg.GetDryRun() {
		c, err := h.supportRepo.CountResumable(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to preview resume: %w", err))
		}
		return connect.NewResponse(&supportv1.ResumePausedAutomationsResponse{
			Resumed:        int32(c.Resumed),
			SkippedBilling: int32(c.SkippedBilling),
			SkippedError:   int32(c.SkippedError),
		}), nil
	}

	c, err := h.supportRepo.ResumePaused(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to resume automations: %w", err))
	}

	// Auditoria: escrita em massa sempre registra quem e quanto.
	user := GetUserFromContext(ctx)
	actor := "desconhecido"
	if user != nil {
		actor = user.Email
	}
	if h.logger != nil {
		h.logger.Warn("support: religou automacoes pausadas em massa",
			zap.String("event", "support_bulk_resume"),
			zap.String("actor", actor),
			zap.Int("resumed", c.Resumed),
			zap.Int("skipped_billing", c.SkippedBilling),
			zap.Int("skipped_error", c.SkippedError),
		)
	}

	return connect.NewResponse(&supportv1.ResumePausedAutomationsResponse{
		Resumed:        int32(c.Resumed),
		SkippedBilling: int32(c.SkippedBilling),
		SkippedError:   int32(c.SkippedError),
	}), nil
}
