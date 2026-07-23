package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	failoverv1 "automatiza-go/internal/gen/failover/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/failover"

	"connectrpc.com/connect"
)

// FailoverHandler expõe a Melhoria 5 (chip reserva) via ConnectRPC.
//
// O handler é registrado SEMPRE, mesmo com a feature desligada. Registrar só
// com a flag ligada faria clientes com o bundle novo receberem `Unimplemented`
// — indistinguível de um deploy quebrado. Desligada, toda RPC responde
// FailedPrecondition com o reason estável INSTANCE_FAILOVER_DISABLED, que é o
// ÚNICO sinal que a UI usa para esconder a seção.
type FailoverHandler struct {
	service *failover.Service
}

// NewFailoverHandler cria o handler.
func NewFailoverHandler(service *failover.Service) *FailoverHandler {
	return &FailoverHandler{service: service}
}

// requireUser resolve o usuário SÓ do contexto autenticado. Nenhuma request do
// serviço carrega user_id ou ator — se carregasse, seria só uma sugestão que o
// servidor teria de ignorar, e um dia alguém deixaria de ignorar.
func requireUser(ctx context.Context) (string, error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return "", connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	return user.ID, nil
}

// failoverError mapeia o domínio para códigos Connect.
func failoverError(err error) error {
	if err == nil {
		return nil
	}
	var alreadyConnect *connect.Error
	if errors.As(err, &alreadyConnect) {
		return err
	}

	switch {
	case errors.Is(err, failover.ErrDisabled):
		ce := connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("%s: instance failover is disabled", failover.ReasonDisabled))
		ce.Meta().Set("failover-reason", failover.ReasonDisabled)
		return ce

	case errors.Is(err, failover.ErrNotFound):
		// Recurso de outro tenant responde igual a inexistente: revelar
		// "existe, mas não é seu" já é vazamento horizontal.
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("not found"))

	case errors.Is(err, failover.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)

	case errors.Is(err, failover.ErrStateConflict),
		errors.Is(err, failover.ErrCoverageBlocked),
		errors.Is(err, failover.ErrDryRunExpired),
		errors.Is(err, failover.ErrDryRunMismatch):
		return connect.NewError(connect.CodeFailedPrecondition, err)

	case errors.Is(err, repository.ErrFailoverIdempotencyMismatch):
		return connect.NewError(connect.CodeAlreadyExists, err)

	case errors.Is(err, repository.ErrFailoverVersionConflict),
		errors.Is(err, repository.ErrFailoverConcurrentOperation),
		errors.Is(err, repository.ErrFailoverStaleConfiguration),
		errors.Is(err, repository.ErrFailoverInProgress):
		return connect.NewError(connect.CodeAborted, err)

	case errors.Is(err, failover.ErrDrainTimeout):
		return connect.NewError(connect.CodeDeadlineExceeded, err)

	case errors.Is(err, failover.ErrIndeterminate),
		errors.Is(err, repository.ErrFailoverIndeterminate):
		// Sem detalhe sensível: o operador vai ao ledger, não ao toast.
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failover state requires human review"))
	}

	return connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))
}

// =============================================================================
// Conversões
// =============================================================================

func modeToProto(mode string) failoverv1.FailoverMode {
	switch mode {
	case models.FailoverModeManual:
		return failoverv1.FailoverMode_FAILOVER_MODE_MANUAL
	case models.FailoverModeAssisted:
		return failoverv1.FailoverMode_FAILOVER_MODE_ASSISTED
	}
	return failoverv1.FailoverMode_FAILOVER_MODE_UNSPECIFIED
}

// modeFromProto rejeita qualquer valor fora de MANUAL/ASSISTED — inclusive um
// número desconhecido que um cliente futuro (ou um curioso) mande no wire.
// `fallback` cobre UNSPECIFIED: MANUAL no create, o modo atual no update.
func modeFromProto(m failoverv1.FailoverMode, fallback string) (string, error) {
	switch m {
	case failoverv1.FailoverMode_FAILOVER_MODE_UNSPECIFIED:
		return fallback, nil
	case failoverv1.FailoverMode_FAILOVER_MODE_MANUAL:
		return models.FailoverModeManual, nil
	case failoverv1.FailoverMode_FAILOVER_MODE_ASSISTED:
		return models.FailoverModeAssisted, nil
	}
	return "", fmt.Errorf("%w: unknown failover mode %d", failover.ErrInvalidArgument, int32(m))
}

func stateToProto(state string) failoverv1.FailoverState {
	switch state {
	case models.FailoverStateActive:
		return failoverv1.FailoverState_FAILOVER_STATE_ACTIVE
	case models.FailoverStateProposed:
		return failoverv1.FailoverState_FAILOVER_STATE_PROPOSED
	case models.FailoverStateSwitching:
		return failoverv1.FailoverState_FAILOVER_STATE_SWITCHING
	case models.FailoverStateActiveOnBackup:
		return failoverv1.FailoverState_FAILOVER_STATE_ACTIVE_ON_BACKUP
	case models.FailoverStateReverting:
		return failoverv1.FailoverState_FAILOVER_STATE_REVERTING
	case models.FailoverStateFailed:
		return failoverv1.FailoverState_FAILOVER_STATE_FAILED
	}
	return failoverv1.FailoverState_FAILOVER_STATE_UNSPECIFIED
}

func directionFromProto(d failoverv1.FailoverDirection) (string, error) {
	switch d {
	case failoverv1.FailoverDirection_FAILOVER_DIRECTION_FAILOVER:
		return models.FailoverDirectionFailover, nil
	case failoverv1.FailoverDirection_FAILOVER_DIRECTION_REVERT:
		return models.FailoverDirectionRevert, nil
	}
	return "", fmt.Errorf("%w: direction must be FAILOVER or REVERT", failover.ErrInvalidArgument)
}

func directionToProto(d string) failoverv1.FailoverDirection {
	if d == models.FailoverDirectionRevert {
		return failoverv1.FailoverDirection_FAILOVER_DIRECTION_REVERT
	}
	return failoverv1.FailoverDirection_FAILOVER_DIRECTION_FAILOVER
}

func opStatusToProto(status string) failoverv1.OperationStatus {
	switch status {
	case models.FailoverOpPreparing:
		return failoverv1.OperationStatus_OPERATION_STATUS_PREPARING
	case models.FailoverOpDraining:
		return failoverv1.OperationStatus_OPERATION_STATUS_DRAINING
	case models.FailoverOpVerifying:
		return failoverv1.OperationStatus_OPERATION_STATUS_VERIFYING
	case models.FailoverOpCompleted:
		return failoverv1.OperationStatus_OPERATION_STATUS_COMPLETED
	case models.FailoverOpAborted:
		return failoverv1.OperationStatus_OPERATION_STATUS_ABORTED
	case models.FailoverOpFailed:
		return failoverv1.OperationStatus_OPERATION_STATUS_FAILED
	}
	return failoverv1.OperationStatus_OPERATION_STATUS_UNSPECIFIED
}

func coverageToProto(result string) failoverv1.CoverageStatus {
	switch result {
	case models.FailoverCoverageCovered:
		return failoverv1.CoverageStatus_COVERAGE_STATUS_COMPLETE
	case models.FailoverCoverageBlocked:
		return failoverv1.CoverageStatus_COVERAGE_STATUS_BLOCKED
	case models.FailoverCoverageDeletedNoop:
		return failoverv1.CoverageStatus_COVERAGE_STATUS_UNSPECIFIED
	}
	return failoverv1.CoverageStatus_COVERAGE_STATUS_UNSPECIFIED
}

func rfc3339(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func policyToProto(p *models.InstanceFailoverPolicy) *failoverv1.FailoverPolicy {
	if p == nil {
		return nil
	}
	out := &failoverv1.FailoverPolicy{
		Id:                         p.ID,
		PrimaryInstanceId:          int64(p.PrimaryInstanceID),
		BackupInstanceId:           int64(p.BackupInstanceID),
		Mode:                       modeToProto(p.Mode),
		State:                      stateToProto(p.State),
		Version:                    p.Version,
		UnhealthyAfterSeconds:      int32(p.UnhealthyAfterSeconds),
		RequiredConsecutiveProbes:  int32(p.RequiredConsecutiveProbes),
		DrainDeadlineSeconds:       int32(p.DrainDeadlineSeconds),
		ProposalTtlSeconds:         int32(p.ProposalTTLSeconds),
		ConsecutiveUnhealthyProbes: int32(p.ConsecutiveUnhealthyProbes),
		PrimaryUnhealthySince:      rfc3339(p.PrimaryUnhealthySince),
		LastProbeAt:                rfc3339(p.LastProbeAt),
		ProposalExpiresAt:          rfc3339(p.ProposalExpiresAt),
		CreatedAt:                  p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                  p.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if p.CurrentOperationID != nil {
		out.CurrentOperationId = *p.CurrentOperationID
	}
	if p.LastProbeStatus != nil {
		out.LastProbeStatus = *p.LastProbeStatus
	}
	return out
}

func eventToProto(e *models.InstanceFailoverEvent) *failoverv1.FailoverEvent {
	if e == nil {
		return nil
	}
	out := &failoverv1.FailoverEvent{
		Id:               e.ID,
		EventType:        e.EventType,
		ActorType:        e.ActorType,
		PolicyIdSnapshot: e.PolicyIDSnapshot,
		ReportJson:       string(e.Report),
		CreatedAt:        e.CreatedAt.UTC().Format(time.RFC3339),
	}
	if e.OperationID != nil {
		out.OperationId = *e.OperationID
	}
	if e.DryRunID != nil {
		out.DryRunId = *e.DryRunID
	}
	if e.PolicyVersion != nil {
		out.PolicyVersion = *e.PolicyVersion
	}
	return out
}

func operationToProto(op *models.InstanceFailoverOperation, items []models.InstanceFailoverOperationItem, reasons []string) *failoverv1.OperationReport {
	if op == nil {
		return nil
	}
	out := &failoverv1.OperationReport{
		OperationId:      op.ID,
		Kind:             directionToProto(op.Kind),
		Status:           opStatusToProto(op.Status),
		PolicyId:         op.PolicyIDSnapshot,
		DryRunId:         op.DryRunID,
		GateClosedAt:     rfc3339(op.GateClosedAt),
		AppliedAt:        rfc3339(op.AppliedAt),
		CompletedAt:      rfc3339(op.CompletedAt),
		ReasonCodes:      reasons,
		AutomationsTotal: int32(len(items)),
	}
	for i := range items {
		switch items[i].Result {
		case models.FailoverItemApplied:
			out.AutomationsApplied++
		case models.FailoverItemRestored:
			out.AutomationsRestored++
		}
	}
	return out
}

func dryRunToProto(dr *models.InstanceFailoverDryRun, items []models.InstanceFailoverDryRunItem) *failoverv1.DryRunReport {
	if dr == nil {
		return nil
	}
	out := &failoverv1.DryRunReport{
		DryRunId:      dr.ID,
		DryRunRef:     dr.DryRunRef,
		Direction:     directionToProto(dr.Direction),
		PolicyId:      dr.PolicyIDSnapshot,
		PolicyVersion: dr.PolicyVersion,
		Passed:        dr.Status == models.FailoverDryRunPass,
		ExpiresAt:     dr.ExpiresAt.UTC().Format(time.RFC3339),
		CreatedAt:     dr.CreatedAt.UTC().Format(time.RFC3339),
	}
	blocking := map[string]struct{}{}
	for i := range items {
		it := &items[i]
		out.Items = append(out.Items, &failoverv1.AutomationCoverage{
			AutomationId:         it.AutomationIDSnapshot,
			AutomationName:       it.AutomationName,
			Status:               coverageToProto(it.Result),
			MissingGroups:        it.MissingGroups,
			MissingAdminGroups:   it.MissingAdminGroups,
			ReasonCodes:          it.ReasonCodes,
			RequiredTargetGroups: int32(len(it.RequiredTargetGroups)),
			RequiredSourceGroups: int32(len(it.RequiredSourceGroups)),
		})
		if it.Result != models.FailoverCoverageCovered {
			for _, r := range it.ReasonCodes {
				blocking[r] = struct{}{}
			}
		}
	}
	for r := range blocking {
		out.BlockingReasonCodes = append(out.BlockingReasonCodes, r)
	}
	return out
}

// =============================================================================
// RPCs
// =============================================================================

// SetPolicy cria ou atualiza a política primária/reserva.
func (h *FailoverHandler) SetPolicy(
	ctx context.Context,
	req *connect.Request[failoverv1.SetPolicyRequest],
) (*connect.Response[failoverv1.SetPolicyResponse], error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg

	primaryID, err := int64ToInstanceID(msg.GetPrimaryInstanceId())
	if err != nil {
		return nil, failoverError(err)
	}
	backupID, err := int64ToInstanceID(msg.GetBackupInstanceId())
	if err != nil {
		return nil, failoverError(err)
	}

	// UNSPECIFIED resolve para MANUAL no create e preserva o modo atual no
	// update.
	fallbackMode := models.FailoverModeManual
	if msg.GetPolicyId() != 0 {
		current, gerr := h.service.GetPolicyRaw(ctx, userID, msg.GetPolicyId())
		if gerr != nil {
			return nil, failoverError(gerr)
		}
		if current != nil {
			fallbackMode = current.Mode
		}
	}
	mode, err := modeFromProto(msg.GetMode(), fallbackMode)
	if err != nil {
		return nil, failoverError(err)
	}

	out, err := h.service.SetPolicy(ctx, failover.SetPolicyInput{
		UserID:                    userID,
		PolicyID:                  msg.GetPolicyId(),
		PrimaryInstanceID:         primaryID,
		BackupInstanceID:          backupID,
		Mode:                      mode,
		ExpectedVersion:           msg.GetExpectedVersion(),
		IdempotencyKey:            msg.GetIdempotencyKey(),
		UnhealthyAfterSeconds:     int32PtrToInt(msg.UnhealthyAfterSeconds),
		RequiredConsecutiveProbes: int32PtrToInt(msg.RequiredConsecutiveProbes),
		DrainDeadlineSeconds:      int32PtrToInt(msg.DrainDeadlineSeconds),
		ProposalTTLSeconds:        int32PtrToInt(msg.ProposalTtlSeconds),
	})
	if err != nil {
		return nil, failoverError(err)
	}

	return connect.NewResponse(&failoverv1.SetPolicyResponse{
		Policy:    policyToProto(out.Policy),
		LastEvent: eventToProto(out.Event),
	}), nil
}

// GetPolicy devolve uma política do próprio usuário.
func (h *FailoverHandler) GetPolicy(
	ctx context.Context,
	req *connect.Request[failoverv1.GetPolicyRequest],
) (*connect.Response[failoverv1.GetPolicyResponse], error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	policy, last, op, err := h.service.GetPolicy(ctx, userID, req.Msg.GetPolicyId())
	if err != nil {
		return nil, failoverError(err)
	}

	resp := &failoverv1.GetPolicyResponse{
		Policy:    policyToProto(policy),
		LastEvent: eventToProto(last),
	}
	if op != nil {
		items, ierr := h.service.OperationItems(ctx, op.ID)
		if ierr != nil {
			return nil, failoverError(ierr)
		}
		resp.CurrentOperation = operationToProto(op, items, nil)
	}
	return connect.NewResponse(resp), nil
}

// ListPolicies lista as políticas do próprio usuário.
func (h *FailoverHandler) ListPolicies(
	ctx context.Context,
	_ *connect.Request[failoverv1.ListPoliciesRequest],
) (*connect.Response[failoverv1.ListPoliciesResponse], error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	policies, err := h.service.ListPolicies(ctx, userID)
	if err != nil {
		return nil, failoverError(err)
	}
	out := &failoverv1.ListPoliciesResponse{}
	for i := range policies {
		out.Policies = append(out.Policies, policyToProto(&policies[i]))
	}
	return connect.NewResponse(out), nil
}

// RunDryRun ensaia a troca (ou a reversão) usando SOMENTE leituras do WAHA.
func (h *FailoverHandler) RunDryRun(
	ctx context.Context,
	req *connect.Request[failoverv1.RunDryRunRequest],
) (*connect.Response[failoverv1.RunDryRunResponse], error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	direction, err := directionFromProto(req.Msg.GetDirection())
	if err != nil {
		return nil, failoverError(err)
	}

	result, err := h.service.RunDryRun(ctx, userID, req.Msg.GetPolicyId(), direction, req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, failoverError(err)
	}
	return connect.NewResponse(&failoverv1.RunDryRunResponse{
		Report: dryRunToProto(result.DryRun, result.Items),
	}), nil
}

// ConfirmFailover executa a troca primária→reserva.
func (h *FailoverHandler) ConfirmFailover(
	ctx context.Context,
	req *connect.Request[failoverv1.ConfirmFailoverRequest],
) (*connect.Response[failoverv1.ConfirmFailoverResponse], error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	result, err := h.service.ConfirmFailover(ctx, failover.ConfirmInput{
		UserID:          userID,
		PolicyID:        req.Msg.GetPolicyId(),
		ExpectedVersion: req.Msg.GetExpectedVersion(),
		DryRunID:        req.Msg.GetDryRunId(),
		DryRunRef:       req.Msg.GetDryRunRef(),
		IdempotencyKey:  req.Msg.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, failoverError(err)
	}
	return connect.NewResponse(&failoverv1.ConfirmFailoverResponse{
		Operation: h.operationReport(ctx, result),
		Policy:    policyToProto(result.Policy),
	}), nil
}

// RevertFailover restaura o roteamento original a partir do snapshot.
func (h *FailoverHandler) RevertFailover(
	ctx context.Context,
	req *connect.Request[failoverv1.RevertFailoverRequest],
) (*connect.Response[failoverv1.RevertFailoverResponse], error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	result, err := h.service.RevertFailover(ctx, failover.ConfirmInput{
		UserID:          userID,
		PolicyID:        req.Msg.GetPolicyId(),
		ExpectedVersion: req.Msg.GetExpectedVersion(),
		DryRunID:        req.Msg.GetDryRunId(),
		DryRunRef:       req.Msg.GetDryRunRef(),
		IdempotencyKey:  req.Msg.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, failoverError(err)
	}
	return connect.NewResponse(&failoverv1.RevertFailoverResponse{
		Operation: h.operationReport(ctx, result),
		Policy:    policyToProto(result.Policy),
	}), nil
}

// DisablePolicy remove a política.
func (h *FailoverHandler) DisablePolicy(
	ctx context.Context,
	req *connect.Request[failoverv1.DisablePolicyRequest],
) (*connect.Response[failoverv1.DisablePolicyResponse], error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.service.DisablePolicy(ctx, userID,
		req.Msg.GetPolicyId(), req.Msg.GetExpectedVersion(), req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, failoverError(err)
	}
	return connect.NewResponse(&failoverv1.DisablePolicyResponse{
		LastEvent: eventToProto(out.Event),
	}), nil
}

// ListEvents pagina o ledger de auditoria.
func (h *FailoverHandler) ListEvents(
	ctx context.Context,
	req *connect.Request[failoverv1.ListEventsRequest],
) (*connect.Response[failoverv1.ListEventsResponse], error) {
	userID, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	events, next, err := h.service.ListEvents(ctx, userID,
		req.Msg.GetPolicyId(), req.Msg.GetCursor(), int(req.Msg.GetLimit()))
	if err != nil {
		return nil, failoverError(err)
	}
	out := &failoverv1.ListEventsResponse{NextCursor: next}
	for i := range events {
		out.Events = append(out.Events, eventToProto(&events[i]))
	}
	return connect.NewResponse(out), nil
}

// operationReport monta o relatório de uma operação já concluída.
func (h *FailoverHandler) operationReport(ctx context.Context, result *failover.ConfirmResult) *failoverv1.OperationReport {
	if result == nil || result.Operation == nil {
		return nil
	}
	items, err := h.service.OperationItems(ctx, result.Operation.ID)
	if err != nil {
		items = nil
	}
	return operationToProto(result.Operation, items, result.Reasons)
}

// int64ToInstanceID valida a faixa do `int` da plataforma antes de estreitar.
func int64ToInstanceID(v int64) (int, error) {
	if v <= 0 || v > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("%w: instance id %s out of range", failover.ErrInvalidArgument, strconv.FormatInt(v, 10))
	}
	return int(v), nil
}

func int32PtrToInt(v *int32) *int {
	if v == nil {
		return nil
	}
	out := int(*v)
	return &out
}
