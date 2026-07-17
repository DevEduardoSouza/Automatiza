package connectrpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	supportv1 "automatiza-go/internal/gen/support/v1"
	"automatiza-go/internal/repository/postgres"

	"connectrpc.com/connect"
)

// ============================================================
// Kanban (aba Kanban do painel de suporte)
// ============================================================
//
// Quadro de bugs estilo Trello, interno da equipe. Assim como o Financeiro,
// e' um desvio consciente da invariante read-only do SupportService: escreve,
// mas so nas tabelas kanban_* e sempre atras de requireSupport. Ver support.proto.

// Colunas FIXAS do fluxo, na ordem do processo. O mesmo conjunto vale no CHECK
// da migracao 070 e na UI — mudar exige tocar os tres lugares de proposito.
var kanbanColumns = map[string]bool{
	"corrigir":     true,
	"aguardando":   true,
	"em_andamento": true,
	"em_revisao":   true,
	"corrigido":    true,
}

var kanbanPriorities = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
}

// maxKanbanAttachmentBytes limita o tamanho de um anexo. Os bytes trafegam no
// corpo do RPC e vivem em BYTEA no Postgres — sem teto, um upload gigante
// pressiona memoria do server e o banco. 15 MB cobre print/PDF/log com folga.
const maxKanbanAttachmentBytes = 15 * 1024 * 1024

// ListKanbanCards devolve o quadro inteiro (cards + metadados de anexos).
func (h *SupportHandler) ListKanbanCards(
	ctx context.Context,
	req *connect.Request[supportv1.ListKanbanCardsRequest],
) (*connect.Response[supportv1.ListKanbanCardsResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}

	cards, err := h.supportRepo.ListKanbanCards(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	attByCard, err := h.supportRepo.ListKanbanAttachments(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*supportv1.KanbanCard, 0, len(cards))
	for _, c := range cards {
		out = append(out, kanbanCardToProto(c, attByCard[c.ID]))
	}
	return connect.NewResponse(&supportv1.ListKanbanCardsResponse{Cards: out}), nil
}

// CreateKanbanCard cria um card no fim da coluna informada.
func (h *SupportHandler) CreateKanbanCard(
	ctx context.Context,
	req *connect.Request[supportv1.CreateKanbanCardRequest],
) (*connect.Response[supportv1.CreateKanbanCardResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}
	m := req.Msg

	title := strings.TrimSpace(m.GetTitle())
	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("título é obrigatório"))
	}

	column := m.GetColumnKey()
	if column == "" {
		column = "corrigir"
	}
	if !kanbanColumns[column] {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("coluna inválida: %q", column))
	}

	priority := m.GetPriority()
	if priority == "" {
		priority = "medium"
	}
	if !kanbanPriorities[priority] {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("prioridade inválida: %q", priority))
	}

	params := postgres.CreateKanbanCardParams{
		Title:       title,
		Description: m.GetDescription(),
		ColumnKey:   column,
		Priority:    priority,
	}
	if u := GetUserFromContext(ctx); u != nil {
		id := u.ID
		params.CreatedBy = &id
	}

	row, err := h.supportRepo.CreateKanbanCard(ctx, params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&supportv1.CreateKanbanCardResponse{
		Card: kanbanCardToProto(*row, nil),
	}), nil
}

// UpdateKanbanCard altera titulo/descricao/prioridade de um card.
func (h *SupportHandler) UpdateKanbanCard(
	ctx context.Context,
	req *connect.Request[supportv1.UpdateKanbanCardRequest],
) (*connect.Response[supportv1.UpdateKanbanCardResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}
	m := req.Msg

	if m.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id é obrigatório"))
	}
	title := strings.TrimSpace(m.GetTitle())
	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("título é obrigatório"))
	}
	priority := m.GetPriority()
	if priority == "" {
		priority = "medium"
	}
	if !kanbanPriorities[priority] {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("prioridade inválida: %q", priority))
	}

	row, err := h.supportRepo.UpdateKanbanCard(ctx, m.GetId(), title, m.GetDescription(), priority)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if row == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("card não encontrado"))
	}
	// Anexos podem existir; recarrega os metadados para a UI nao perde-los.
	att, err := h.supportRepo.ListKanbanAttachments(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&supportv1.UpdateKanbanCardResponse{
		Card: kanbanCardToProto(*row, att[row.ID]),
	}), nil
}

// MoveKanbanCard move um card de coluna e/ou reordena.
func (h *SupportHandler) MoveKanbanCard(
	ctx context.Context,
	req *connect.Request[supportv1.MoveKanbanCardRequest],
) (*connect.Response[supportv1.MoveKanbanCardResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}
	m := req.Msg

	if m.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id é obrigatório"))
	}
	if !kanbanColumns[m.GetColumnKey()] {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("coluna inválida: %q", m.GetColumnKey()))
	}

	row, err := h.supportRepo.MoveKanbanCard(ctx, m.GetId(), m.GetColumnKey(), m.GetPosition())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if row == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("card não encontrado"))
	}
	att, err := h.supportRepo.ListKanbanAttachments(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&supportv1.MoveKanbanCardResponse{
		Card: kanbanCardToProto(*row, att[row.ID]),
	}), nil
}

// DeleteKanbanCard remove um card e seus anexos (cascata).
func (h *SupportHandler) DeleteKanbanCard(
	ctx context.Context,
	req *connect.Request[supportv1.DeleteKanbanCardRequest],
) (*connect.Response[supportv1.DeleteKanbanCardResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id é obrigatório"))
	}
	deleted, err := h.supportRepo.DeleteKanbanCard(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&supportv1.DeleteKanbanCardResponse{Deleted: deleted}), nil
}

// UploadKanbanAttachment anexa um documento a um card.
func (h *SupportHandler) UploadKanbanAttachment(
	ctx context.Context,
	req *connect.Request[supportv1.UploadKanbanAttachmentRequest],
) (*connect.Response[supportv1.UploadKanbanAttachmentResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}
	m := req.Msg

	if m.GetCardId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("card_id é obrigatório"))
	}
	filename := strings.TrimSpace(m.GetFilename())
	if filename == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("filename é obrigatório"))
	}
	data := m.GetData()
	if len(data) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("arquivo vazio"))
	}
	if len(data) > maxKanbanAttachmentBytes {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("arquivo excede o limite de %d MB", maxKanbanAttachmentBytes/(1024*1024)))
	}

	exists, err := h.supportRepo.CardExists(ctx, m.GetCardId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("card não encontrado"))
	}

	contentType := m.GetContentType()
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var createdBy *string
	if u := GetUserFromContext(ctx); u != nil {
		id := u.ID
		createdBy = &id
	}

	row, err := h.supportRepo.CreateKanbanAttachment(ctx, m.GetCardId(), filename, contentType, data, createdBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&supportv1.UploadKanbanAttachmentResponse{
		Attachment: kanbanAttachmentToProto(*row),
	}), nil
}

// DownloadKanbanAttachment devolve os bytes de um anexo.
func (h *SupportHandler) DownloadKanbanAttachment(
	ctx context.Context,
	req *connect.Request[supportv1.DownloadKanbanAttachmentRequest],
) (*connect.Response[supportv1.DownloadKanbanAttachmentResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id é obrigatório"))
	}
	content, err := h.supportRepo.GetKanbanAttachmentContent(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if content == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("anexo não encontrado"))
	}
	return connect.NewResponse(&supportv1.DownloadKanbanAttachmentResponse{
		Filename:    content.Filename,
		ContentType: content.ContentType,
		Data:        content.Data,
	}), nil
}

// DeleteKanbanAttachment remove um anexo.
func (h *SupportHandler) DeleteKanbanAttachment(
	ctx context.Context,
	req *connect.Request[supportv1.DeleteKanbanAttachmentRequest],
) (*connect.Response[supportv1.DeleteKanbanAttachmentResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id é obrigatório"))
	}
	deleted, err := h.supportRepo.DeleteKanbanAttachment(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&supportv1.DeleteKanbanAttachmentResponse{Deleted: deleted}), nil
}

// kanbanCardToProto converte a linha do banco no message do proto.
func kanbanCardToProto(c postgres.KanbanCardRow, atts []postgres.KanbanAttachmentRow) *supportv1.KanbanCard {
	protoAtts := make([]*supportv1.KanbanAttachment, 0, len(atts))
	for _, a := range atts {
		protoAtts = append(protoAtts, kanbanAttachmentToProto(a))
	}
	return &supportv1.KanbanCard{
		Id:          c.ID,
		ColumnKey:   c.ColumnKey,
		Position:    c.Position,
		Title:       c.Title,
		Description: c.Description,
		Priority:    c.Priority,
		CreatedAt:   c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   c.UpdatedAt.Format(time.RFC3339),
		Attachments: protoAtts,
	}
}

func kanbanAttachmentToProto(a postgres.KanbanAttachmentRow) *supportv1.KanbanAttachment {
	return &supportv1.KanbanAttachment{
		Id:          a.ID,
		CardId:      a.CardID,
		Filename:    a.Filename,
		ContentType: a.ContentType,
		SizeBytes:   a.SizeBytes,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
	}
}
