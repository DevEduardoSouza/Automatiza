package monitoring

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"automatiza-go/internal/models"
)

// Cálculo de janela da cadência (Melhoria 7).
//
// O espelho vivo só pergunta "estou dentro da janela AGORA?" (TimeWindow.
// IsWithinWindow). A cadência precisa da pergunta oposta e mais difícil:
// "qual é o PRÓXIMO instante permitido?". Sem ela, um item admitido às 23h50
// numa janela 08:00–18:00 ficaria devido imediatamente e sairia fora do horário
// que o cliente configurou.

// ErrInvalidMonitorWindow: janela impossível de satisfazer (horário mal
// formado, ou nenhum dia válido nos próximos 8 dias). Fail-closed de propósito:
// agendar "para qualquer hora" seria pior que recusar.
var ErrInvalidMonitorWindow = errors.New(models.MonitorCodeInvalidWindow)

// monitorWindowSearchDays: uma semana + 1. Uma janela válida sempre cai dentro
// desse horizonte; não cair significa configuração impossível, não "espera
// mais".
const monitorWindowSearchDays = 8

// monitorLocation é o fuso das janelas. Mesma constante que o caminho vivo já
// usa (processAutomation), resolvida uma vez.
var monitorLocation = sync.OnceValue(func() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.UTC
	}
	return loc
})

// MonitorLocation devolve o fuso usado no cálculo de janela.
func MonitorLocation() *time.Location { return monitorLocation() }

// AdvanceToAllowedWindow devolve o primeiro instante >= candidate que cai
// dentro da janela. Sem janela configurada, devolve candidate intocado.
//
// Regras (§10 do plano):
//   - start/end ausentes  → sem restrição de horário (dias ainda valem);
//   - start == end        → dia inteiro aberto;
//   - start < end         → [start, end) no mesmo dia;
//   - start > end         → atravessa a meia-noite: a janela PERTENCE ao dia em
//     que começa, então 01:00 de terça está na janela de segunda 22:00→06:00;
//   - days vazio          → todos os dias.
//
// O resultado é sempre um instante REAL: horário local inexistente (virada de
// horário de verão) é normalizado para frente pelo time.Date, e horário
// ambíguo usa a primeira ocorrência. Brasil não usa mais DST desde 2019, então
// isto é defensivo — mas é o tipo de defesa que só se descobre que faltava
// quando a fila inteira dispara na hora errada.
func AdvanceToAllowedWindow(candidate time.Time, w models.MonitorWindowSnapshot, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = monitorLocation()
	}
	startMin, endMin, hasHours, err := parseWindowBounds(w)
	if err != nil {
		return time.Time{}, err
	}
	days, err := parseWindowDays(w.Days)
	if err != nil {
		return time.Time{}, err
	}
	if !hasHours && len(days) == 0 {
		return candidate, nil
	}
	// start == end é dia inteiro: o horário deixa de restringir, mas os dias
	// continuam valendo.
	if hasHours && startMin == endMin {
		hasHours = false
	}
	if !hasHours && len(days) == 0 {
		return candidate, nil
	}

	local := candidate.In(loc)
	// Começa em -1 dia: uma janela que atravessa a meia-noite pertence ao dia
	// em que COMEÇOU, então o candidato das 01:00 pode estar dentro da janela
	// aberta ontem às 22:00.
	for offset := -1; offset <= monitorWindowSearchDays; offset++ {
		day := time.Date(local.Year(), local.Month(), local.Day()+offset, 0, 0, 0, 0, loc)
		if len(days) > 0 {
			if _, ok := days[day.Weekday()]; !ok {
				continue
			}
		}
		if !hasHours {
			// Dia inteiro liberado: vale o próprio candidato se já estamos
			// dentro do dia, senão a meia-noite dele.
			if !candidate.Before(day) && candidate.Before(day.AddDate(0, 0, 1)) {
				return candidate, nil
			}
			if candidate.Before(day) {
				return day, nil
			}
			continue
		}

		windowStart := addMinutes(day, startMin, loc)
		windowEnd := addMinutes(day, endMin, loc)
		if endMin <= startMin {
			// Atravessa a meia-noite: termina no dia seguinte.
			windowEnd = addMinutes(day.AddDate(0, 0, 1), endMin, loc)
		}
		if !candidate.Before(windowStart) && candidate.Before(windowEnd) {
			return candidate, nil
		}
		if candidate.Before(windowStart) {
			return windowStart, nil
		}
	}
	return time.Time{}, ErrInvalidMonitorWindow
}

// IsWithinWindowAt informa se `at` está dentro da janela. Derivado de
// AdvanceToAllowedWindow para não existirem duas noções de "dentro da janela"
// que possam divergir.
func IsWithinWindowAt(at time.Time, w models.MonitorWindowSnapshot, loc *time.Location) bool {
	next, err := AdvanceToAllowedWindow(at, w, loc)
	if err != nil {
		return false
	}
	return next.Equal(at)
}

// addMinutes soma minutos ao dia LOCAL (não ao instante): passa pelo time.Date
// para que a virada de offset seja normalizada em vez de gerar um horário que
// não existe no calendário.
func addMinutes(day time.Time, minutes int, loc *time.Location) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), minutes/60, minutes%60, 0, 0, loc)
}

// parseWindowBounds lê start/end estritos no formato HH:MM.
func parseWindowBounds(w models.MonitorWindowSnapshot) (startMin, endMin int, hasHours bool, err error) {
	start := strings.TrimSpace(w.StartTime)
	end := strings.TrimSpace(w.EndTime)
	if start == "" || end == "" {
		return 0, 0, false, nil
	}
	startMin, err = parseHHMM(start)
	if err != nil {
		return 0, 0, false, err
	}
	endMin, err = parseHHMM(end)
	if err != nil {
		return 0, 0, false, err
	}
	return startMin, endMin, true, nil
}

// parseHHMM aceita "HH:MM" e "HH:MM:SS" (o TIME do Postgres pode chegar nos
// dois formatos dependendo do caminho de leitura). Qualquer outra coisa é
// configuração inválida — não há fallback silencioso.
func parseHHMM(v string) (int, error) {
	parts := strings.Split(v, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, ErrInvalidMonitorWindow
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, ErrInvalidMonitorWindow
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, ErrInvalidMonitorWindow
	}
	return h*60 + m, nil
}

// weekdayAliases cobre os dois formatos que o frontend já gravou historicamente
// (abreviado e completo), sempre em minúsculas.
var weekdayAliases = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday, "dom": time.Sunday,
	"mon": time.Monday, "monday": time.Monday, "seg": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday, "ter": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday, "qua": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday, "qui": time.Thursday,
	"fri": time.Friday, "friday": time.Friday, "sex": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday, "sab": time.Saturday,
}

func parseWindowDays(days []string) (map[time.Weekday]struct{}, error) {
	if len(days) == 0 {
		return nil, nil
	}
	out := make(map[time.Weekday]struct{}, len(days))
	for _, d := range days {
		key := strings.ToLower(strings.TrimSpace(d))
		if key == "" {
			continue
		}
		wd, ok := weekdayAliases[key]
		if !ok {
			return nil, ErrInvalidMonitorWindow
		}
		out[wd] = struct{}{}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// WindowSnapshotFor extrai a janela vigente de uma automação.
func WindowSnapshotFor(automation *models.Automation) models.MonitorWindowSnapshot {
	var w models.MonitorWindowSnapshot
	if automation.StartTime != nil {
		w.StartTime = *automation.StartTime
	}
	if automation.EndTime != nil {
		w.EndTime = *automation.EndTime
	}
	if len(automation.ScheduleDays) > 0 {
		w.Days = append([]string(nil), automation.ScheduleDays...)
	}
	return w
}
