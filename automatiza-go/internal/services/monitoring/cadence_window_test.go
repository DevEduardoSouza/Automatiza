package monitoring

import (
	"testing"
	"time"

	"automatiza-go/internal/models"
)

// A janela é onde um erro custa caro em silêncio: errar aqui não quebra nada
// visível, só faz a fila inteira disparar na hora errada. Daí a bateria abaixo
// cobrir borda por borda em vez de "um caso feliz".

func spTime(t *testing.T, s string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("tzdata indisponível: %v", err)
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		t.Fatalf("data inválida %q: %v", s, err)
	}
	return parsed
}

func TestAdvanceToAllowedWindow(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skipf("tzdata indisponível no ambiente: %v", err)
	}

	tests := []struct {
		name      string
		candidate string
		window    models.MonitorWindowSnapshot
		want      string
		wantErr   bool
	}{
		{
			name:      "sem janela devolve o candidato intocado",
			candidate: "2026-07-23 03:14",
			window:    models.MonitorWindowSnapshot{},
			want:      "2026-07-23 03:14",
		},
		{
			name:      "dentro da janela normal devolve o candidato",
			candidate: "2026-07-23 10:00",
			window:    models.MonitorWindowSnapshot{StartTime: "08:00", EndTime: "18:00"},
			want:      "2026-07-23 10:00",
		},
		{
			name:      "antes da abertura espera a abertura do MESMO dia",
			candidate: "2026-07-23 06:30",
			window:    models.MonitorWindowSnapshot{StartTime: "08:00", EndTime: "18:00"},
			want:      "2026-07-23 08:00",
		},
		{
			name:      "depois do fechamento pula para a abertura do dia seguinte",
			candidate: "2026-07-23 23:30",
			window:    models.MonitorWindowSnapshot{StartTime: "08:00", EndTime: "18:00"},
			want:      "2026-07-24 08:00",
		},
		{
			name:      "instante do fechamento ja esta FORA (intervalo semiaberto)",
			candidate: "2026-07-23 18:00",
			window:    models.MonitorWindowSnapshot{StartTime: "08:00", EndTime: "18:00"},
			want:      "2026-07-24 08:00",
		},
		{
			name:      "start igual a end libera o dia inteiro",
			candidate: "2026-07-23 03:00",
			window:    models.MonitorWindowSnapshot{StartTime: "09:00", EndTime: "09:00"},
			want:      "2026-07-23 03:00",
		},
		{
			name:      "overnight: madrugada pertence a janela aberta ONTEM",
			candidate: "2026-07-23 01:00",
			window:    models.MonitorWindowSnapshot{StartTime: "22:00", EndTime: "06:00"},
			want:      "2026-07-23 01:00",
		},
		{
			name:      "overnight: meio do dia espera a abertura da noite",
			candidate: "2026-07-23 12:00",
			window:    models.MonitorWindowSnapshot{StartTime: "22:00", EndTime: "06:00"},
			want:      "2026-07-23 22:00",
		},
		{
			name:      "overnight: fim da janela pula para a proxima noite",
			candidate: "2026-07-23 06:00",
			window:    models.MonitorWindowSnapshot{StartTime: "22:00", EndTime: "06:00"},
			want:      "2026-07-23 22:00",
		},
		{
			name:      "dia nao listado adia para o proximo dia valido",
			candidate: "2026-07-23 10:00", // quinta
			window:    models.MonitorWindowSnapshot{StartTime: "08:00", EndTime: "18:00", Days: []string{"mon"}},
			want:      "2026-07-27 08:00", // segunda seguinte
		},
		{
			name:      "virada de semana: sabado a noite espera segunda",
			candidate: "2026-07-25 23:00", // sábado
			window:    models.MonitorWindowSnapshot{StartTime: "09:00", EndTime: "17:00", Days: []string{"mon", "tue"}},
			want:      "2026-07-27 09:00",
		},
		{
			name:      "dias em portugues tambem valem (dado legado do front)",
			candidate: "2026-07-23 10:00", // quinta
			window:    models.MonitorWindowSnapshot{StartTime: "08:00", EndTime: "18:00", Days: []string{"sex"}},
			want:      "2026-07-24 08:00",
		},
		{
			name:      "dias sem horario: proximo dia valido a meia-noite",
			candidate: "2026-07-23 10:00", // quinta
			window:    models.MonitorWindowSnapshot{Days: []string{"fri"}},
			want:      "2026-07-24 00:00",
		},
		{
			name:      "horario mal formado e recusado (nunca fail-open)",
			candidate: "2026-07-23 10:00",
			window:    models.MonitorWindowSnapshot{StartTime: "8h", EndTime: "18:00"},
			wantErr:   true,
		},
		{
			name:      "hora fora da faixa e recusada",
			candidate: "2026-07-23 10:00",
			window:    models.MonitorWindowSnapshot{StartTime: "25:00", EndTime: "18:00"},
			wantErr:   true,
		},
		{
			name:      "dia desconhecido e recusado",
			candidate: "2026-07-23 10:00",
			window:    models.MonitorWindowSnapshot{StartTime: "08:00", EndTime: "18:00", Days: []string{"funday"}},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AdvanceToAllowedWindow(spTime(t, tc.candidate), tc.window, loc)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("esperava erro de janela inválida, obtive %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			want := spTime(t, tc.want)
			if !got.Equal(want) {
				t.Errorf("janela errada:\n  candidato: %s\n  obtido:    %s\n  esperado:  %s",
					tc.candidate, got.In(loc).Format("2006-01-02 15:04 MST"), want.Format("2006-01-02 15:04 MST"))
			}
		})
	}
}

// O resultado é sempre um instante real e o cálculo é feito em horário LOCAL,
// mas persistido em UTC. Este teste ancora a conversão: 08:00 em São Paulo é
// 11:00 UTC (UTC-3, sem horário de verão desde 2019).
func TestAdvanceToAllowedWindowReturnsUTCInstant(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skipf("tzdata indisponível: %v", err)
	}
	got, err := AdvanceToAllowedWindow(
		spTime(t, "2026-07-23 06:00"),
		models.MonitorWindowSnapshot{StartTime: "08:00", EndTime: "18:00"},
		loc)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if utc := got.UTC().Format("2006-01-02 15:04"); utc != "2026-07-23 11:00" {
		t.Errorf("instante UTC errado: %s (esperado 2026-07-23 11:00)", utc)
	}
}

// A monotonicidade é o que garante que a fila não anda para trás: o resultado
// NUNCA pode ser anterior ao candidato, em nenhuma configuração válida.
func TestAdvanceToAllowedWindowNeverGoesBackwards(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skipf("tzdata indisponível: %v", err)
	}
	windows := []models.MonitorWindowSnapshot{
		{},
		{StartTime: "08:00", EndTime: "18:00"},
		{StartTime: "22:00", EndTime: "06:00"},
		{StartTime: "09:00", EndTime: "09:00"},
		{StartTime: "08:00", EndTime: "18:00", Days: []string{"mon", "wed", "fri"}},
		{Days: []string{"sun"}},
	}
	base := spTime(t, "2026-07-23 00:00")
	for _, w := range windows {
		for minute := 0; minute < 24*60; minute += 17 {
			candidate := base.Add(time.Duration(minute) * time.Minute)
			got, err := AdvanceToAllowedWindow(candidate, w, loc)
			if err != nil {
				t.Fatalf("janela %+v em %s: %v", w, candidate, err)
			}
			if got.Before(candidate) {
				t.Fatalf("resultado anterior ao candidato: %s < %s (janela %+v)", got, candidate, w)
			}
		}
	}
}

func TestIsWithinWindowAtMatchesAdvance(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skipf("tzdata indisponível: %v", err)
	}
	w := models.MonitorWindowSnapshot{StartTime: "08:00", EndTime: "18:00"}
	if !IsWithinWindowAt(spTime(t, "2026-07-23 09:00"), w, loc) {
		t.Error("09:00 deveria estar dentro de 08:00-18:00")
	}
	if IsWithinWindowAt(spTime(t, "2026-07-23 07:59"), w, loc) {
		t.Error("07:59 não deveria estar dentro de 08:00-18:00")
	}
}

func TestWindowSnapshotForCopiesDays(t *testing.T) {
	start, end := "08:00", "18:00"
	days := []string{"mon", "tue"}
	automation := &models.Automation{StartTime: &start, EndTime: &end, ScheduleDays: days}

	snap := WindowSnapshotFor(automation)
	days[0] = "sat" // muta a origem DEPOIS do snapshot

	if snap.Days[0] != "mon" {
		t.Error("o snapshot precisa ser uma cópia: editar a automação não pode reescrever reserva já feita")
	}
	if snap.StartTime != "08:00" || snap.EndTime != "18:00" {
		t.Errorf("snapshot de horário errado: %+v", snap)
	}
}
