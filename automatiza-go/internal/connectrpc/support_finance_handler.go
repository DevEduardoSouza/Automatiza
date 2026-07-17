package connectrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	supportv1 "automatiza-go/internal/gen/support/v1"
	"automatiza-go/internal/repository/postgres"
	"automatiza-go/internal/services/kiwify"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// ============================================================
// Financeiro (aba Financeiro do painel de suporte)
// ============================================================
//
// A receita de ASSINATURA e' ESTIMATIVA: profiles.plan_id -> plans.price para
// os ativos mapeados. A Kiwify nao expoe transacao no banco (webhook so guarda
// idempotencia, sem valor; nao ha client da API). O que e' fiel: os lancamentos
// manuais (finance_entries) e as contagens de assinantes por billing_status.
// Ver 069_finance_entries.up.sql e a memoria da feature.

// 36 meses => a UI agrega em ate' 12 trimestres ou 3 anos.
const financeProjectionMonths = 36

// mesesPT sao as abreviacoes usadas nos rotulos da projecao.
var mesesPT = [...]string{
	"jan", "fev", "mar", "abr", "mai", "jun",
	"jul", "ago", "set", "out", "nov", "dez",
}

// monthIndex reduz uma data a um numero absoluto de meses (ano*12+mes), para
// comparar competencia sem lidar com fuso/dia.
func monthIndex(t time.Time) int {
	return t.Year()*12 + int(t.Month()) - 1
}

// entryActiveInMonth diz se um lancamento incide na competencia `idx`:
//   - unico (recurring=false): so no mes do entry_date;
//   - recorrente com N meses: de entry_date por N meses (inclusive);
//   - recorrente infinito (RecurrenceMonths nil): de entry_date em diante.
func entryActiveInMonth(e postgres.FinanceEntryRow, idx int) bool {
	start := monthIndex(e.EntryDate)
	if idx < start {
		return false
	}
	if !e.Recurring {
		return idx == start
	}
	if e.RecurrenceMonths == nil {
		return true // infinito
	}
	return idx <= start+int(*e.RecurrenceMonths)-1
}

// GetFinanceOverview: KPIs de SaaS, receita estimada por plano, projecao de 12
// meses e os lancamentos manuais. Filtro por data (from/to) afeta os KPIs de
// fluxo do periodo; a projecao e' sempre 12 meses a partir do mes atual.
func (h *SupportHandler) GetFinanceOverview(
	ctx context.Context,
	req *connect.Request[supportv1.GetFinanceOverviewRequest],
) (*connect.Response[supportv1.GetFinanceOverviewResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Janela do filtro: default = mes corrente.
	from := monthStart
	to := monthStart.AddDate(0, 1, -1) // ultimo dia do mes
	if s := req.Msg.GetFrom(); s != "" {
		if d, err := time.Parse("2006-01-02", s); err == nil {
			from = d
		}
	}
	if s := req.Msg.GetTo(); s != "" {
		if d, err := time.Parse("2006-01-02", s); err == nil {
			to = d
		}
	}

	planRows, err := h.supportRepo.GetFinancePlanRevenue(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	counts, err := h.supportRepo.GetFinanceSubscriberCounts(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	entries, err := h.supportRepo.ListFinanceEntries(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// --- Base ESTIMADA (fallback): MRR = plan_id x preco de catalogo. ---
	var mrr float64
	planRevenue := make([]*supportv1.PlanRevenue, 0, len(planRows))
	for _, p := range planRows {
		lineMRR := p.Price * float64(p.Subscribers)
		mrr += lineMRR
		planRevenue = append(planRevenue, &supportv1.PlanRevenue{
			Plan:        p.Plan,
			Price:       p.Price,
			Subscribers: int32(p.Subscribers),
			Mrr:         lineMRR,
		})
	}
	var coverage float64
	if counts.Active > 0 {
		coverage = float64(counts.Mapped) / float64(counts.Active) * 100
	}
	activeSubs := counts.Active
	note := fmt.Sprintf(
		"Receita de assinatura é ESTIMATIVA: %d de %d ativos têm plano mapeado (%.0f%%). "+
			"A Kiwify não expõe transações no banco — os demais ativos não entram no MRR.",
		counts.Mapped, counts.Active, coverage,
	)

	// --- Reais da Kiwify (produto AUTOMATISA), quando habilitado. Sobrepoe a
	// estimativa; em erro/timeout mantem o fallback (nunca quebra o painel). ---
	var (
		kiwifyLive            bool
		revenueRealizedPeriod float64
		paidSales             int
		refundedSales         int
		refundRate            float64
		avgTicket             float64
	)
	if h.kiwifyAPI != nil && h.kiwifyAPI.Enabled() {
		periodSum, perr := h.cachedKiwifySummary(ctx, from, to)
		mrrSum, merr := h.cachedKiwifySummary(ctx, now.AddDate(0, 0, -30), now)
		if perr == nil && merr == nil {
			kiwifyLive = true
			revenueRealizedPeriod = float64(periodSum.GrossPaidCents) / 100
			paidSales = periodSum.PaidCount
			refundedSales = periodSum.RefundedCount
			if periodSum.PaidCount+periodSum.RefundedCount > 0 {
				refundRate = float64(periodSum.RefundedCount) /
					float64(periodSum.PaidCount+periodSum.RefundedCount) * 100
			}
			if periodSum.PaidCount > 0 {
				avgTicket = revenueRealizedPeriod / float64(periodSum.PaidCount)
			}
			// MRR real = total pago nos ultimos 30 dias; assinantes = nº de pagas.
			mrr = float64(mrrSum.GrossPaidCents) / 100
			activeSubs = mrrSum.PaidCount
			planRevenue = planRevenue[:0]
			for _, p := range mrrSum.ByPlan {
				price := 0.0
				if p.Count > 0 {
					price = float64(p.GrossCents) / 100 / float64(p.Count)
				}
				planRevenue = append(planRevenue, &supportv1.PlanRevenue{
					Plan:        p.Plan,
					Price:       price,
					Subscribers: int32(p.Count),
					Mrr:         float64(p.GrossCents) / 100,
				})
			}
			note = "Receita do produto AUTOMATISA via API da Kiwify (vendas pagas). " +
				"MRR = total pago nos últimos 30 dias; faturamento = pago na janela filtrada."
		} else {
			h.logger.Warn("finance: Kiwify indisponível, usando estimativa por plano",
				zap.Errors("errs", []error{perr, merr}))
		}
	}

	// Fluxo do periodo filtrado (lancamentos manuais).
	fromIdx, toIdx := monthIndex(from), monthIndex(to)
	if toIdx < fromIdx {
		fromIdx, toIdx = toIdx, fromIdx
	}
	var expensesPeriod, revenuePeriod float64
	for _, e := range entries {
		if !e.Recurring {
			// Unico: conta se o dia cai na janela exata.
			if !e.EntryDate.Before(from) && !e.EntryDate.After(to) {
				if e.Kind == "expense" {
					expensesPeriod += e.Amount
				} else {
					revenuePeriod += e.Amount
				}
			}
			continue
		}
		for idx := fromIdx; idx <= toIdx; idx++ {
			if entryActiveInMonth(e, idx) {
				if e.Kind == "expense" {
					expensesPeriod += e.Amount
				} else {
					revenuePeriod += e.Amount
				}
			}
		}
	}

	months := float64(toIdx-fromIdx) + 1
	// Contribuicao de assinatura no periodo: real (Kiwify) ou run-rate estimado.
	subscriptionPeriod := mrr * months
	if kiwifyLive {
		subscriptionPeriod = revenueRealizedPeriod
	}
	grossPeriod := subscriptionPeriod + revenuePeriod
	netPeriod := grossPeriod - expensesPeriod
	var margin float64
	if grossPeriod > 0 {
		margin = netPeriod / grossPeriod * 100
	}

	var arpu float64
	if kiwifyLive {
		if activeSubs > 0 {
			arpu = mrr / float64(activeSubs)
		}
	} else if counts.Mapped > 0 {
		arpu = mrr / float64(counts.Mapped)
	}

	kpis := &supportv1.FinanceKpis{
		MrrEstimated:          mrr,
		ArrEstimated:          mrr * 12,
		ArpuEstimated:         arpu,
		ActiveSubscribers:     int32(activeSubs),
		MappedSubscribers:     int32(counts.Mapped),
		CoveragePercent:       coverage,
		Suspended:             int32(counts.Suspended),
		Canceled:              int32(counts.Canceled),
		Refunded:              int32(counts.Refunded),
		Late:                  int32(counts.Late),
		ExpensesPeriod:        expensesPeriod,
		ManualRevenuePeriod:   revenuePeriod,
		NetResultPeriod:       netPeriod,
		MarginPercent:         margin,
		RevenueRealizedPeriod: revenueRealizedPeriod,
		PaidSales:             int32(paidSales),
		RefundedSales:         int32(refundedSales),
		RefundRate:            refundRate,
		AvgTicket:             avgTicket,
		KiwifyLive:            kiwifyLive,
	}

	// Projecao: 12 meses a partir do mes atual. Receita = MRR + receitas manuais
	// incidentes; despesa = despesas manuais incidentes.
	projection := make([]*supportv1.FinanceProjectionPoint, 0, financeProjectionMonths)
	for k := 0; k < financeProjectionMonths; k++ {
		m := monthStart.AddDate(0, k, 0)
		idx := monthIndex(m)
		revenue := mrr
		var expense float64
		for _, e := range entries {
			if !entryActiveInMonth(e, idx) {
				continue
			}
			if e.Kind == "expense" {
				expense += e.Amount
			} else {
				revenue += e.Amount
			}
		}
		projection = append(projection, &supportv1.FinanceProjectionPoint{
			PeriodStart: m.Format("2006-01-02"),
			Label:       fmt.Sprintf("%s/%d", mesesPT[int(m.Month())-1], m.Year()),
			Revenue:     revenue,
			Expenses:    expense,
			Net:         revenue - expense,
		})
	}

	protoEntries := make([]*supportv1.FinanceEntry, 0, len(entries))
	for _, e := range entries {
		protoEntries = append(protoEntries, financeEntryToProto(e))
	}

	return connect.NewResponse(&supportv1.GetFinanceOverviewResponse{
		Kpis:         kpis,
		PlanRevenue:  planRevenue,
		Projection:   projection,
		Entries:      protoEntries,
		CoverageNote: note,
	}), nil
}

// cachedKiwifySummary busca o resumo de vendas da Kiwify com cache no Redis
// (TTL 15min) — a API tem rate limit 100/min e uma janela grande pagina varias
// vezes, entao nao pode rodar a cada load do painel. Timeout proprio para nunca
// travar o dashboard; erro/timeout sobe para o chamador cair na estimativa.
func (h *SupportHandler) cachedKiwifySummary(ctx context.Context, from, to time.Time) (*kiwify.SalesSummary, error) {
	key := fmt.Sprintf("support:kiwify_sales:%s:%s",
		from.Format("2006-01-02"), to.Format("2006-01-02"))

	if h.redisClient != nil {
		if raw, err := h.redisClient.Get(ctx, key).Result(); err == nil {
			var s kiwify.SalesSummary
			if json.Unmarshal([]byte(raw), &s) == nil {
				return &s, nil
			}
		}
	}

	cctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	sum, err := h.kiwifyAPI.GetSalesSummary(cctx, from, to)
	if err != nil {
		return nil, err
	}

	if h.redisClient != nil {
		if b, e := json.Marshal(sum); e == nil {
			h.redisClient.Set(ctx, key, b, 15*time.Minute)
		}
	}
	return sum, nil
}

// CreateFinanceEntry cria um lancamento manual (despesa/lucro).
func (h *SupportHandler) CreateFinanceEntry(
	ctx context.Context,
	req *connect.Request[supportv1.CreateFinanceEntryRequest],
) (*connect.Response[supportv1.CreateFinanceEntryResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}
	m := req.Msg

	kind := m.GetKind()
	if kind != "expense" && kind != "revenue" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("kind deve ser 'expense' ou 'revenue'"))
	}
	if m.GetDescription() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("descrição é obrigatória"))
	}
	if m.GetAmount() < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("valor não pode ser negativo"))
	}
	entryDate, err := time.Parse("2006-01-02", m.GetEntryDate())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("entry_date inválida (use YYYY-MM-DD)"))
	}

	params := postgres.CreateFinanceEntryParams{
		Kind:        kind,
		Description: m.GetDescription(),
		Amount:      m.GetAmount(),
		EntryDate:   entryDate,
		Recurring:   m.GetRecurring(),
	}
	if m.Category != nil && m.GetCategory() != "" {
		c := m.GetCategory()
		params.Category = &c
	}
	// recurrence_months so vale para recorrente; ausente + recorrente => infinito.
	if m.GetRecurring() && m.RecurrenceMonths != nil {
		n := m.GetRecurrenceMonths()
		if n <= 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("recurrence_months deve ser > 0 (ou ausente para infinito)"))
		}
		params.RecurrenceMonths = &n
	}
	if u := GetUserFromContext(ctx); u != nil {
		id := u.ID
		params.CreatedBy = &id
	}

	row, err := h.supportRepo.CreateFinanceEntry(ctx, params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&supportv1.CreateFinanceEntryResponse{
		Entry: financeEntryToProto(*row),
	}), nil
}

// DeleteFinanceEntry remove um lancamento manual.
func (h *SupportHandler) DeleteFinanceEntry(
	ctx context.Context,
	req *connect.Request[supportv1.DeleteFinanceEntryRequest],
) (*connect.Response[supportv1.DeleteFinanceEntryResponse], error) {
	if err := h.requireSupport(ctx); err != nil {
		return nil, err
	}
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id é obrigatório"))
	}
	deleted, err := h.supportRepo.DeleteFinanceEntry(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&supportv1.DeleteFinanceEntryResponse{Deleted: deleted}), nil
}

// financeEntryToProto converte a linha do banco no message do proto.
func financeEntryToProto(e postgres.FinanceEntryRow) *supportv1.FinanceEntry {
	pe := &supportv1.FinanceEntry{
		Id:          e.ID,
		Kind:        e.Kind,
		Description: e.Description,
		Amount:      e.Amount,
		EntryDate:   e.EntryDate.Format("2006-01-02"),
		Recurring:   e.Recurring,
		CreatedAt:   e.CreatedAt.Format(time.RFC3339),
	}
	if e.Category != nil {
		pe.Category = e.Category
	}
	if e.RecurrenceMonths != nil {
		pe.RecurrenceMonths = e.RecurrenceMonths
	}
	return pe
}
