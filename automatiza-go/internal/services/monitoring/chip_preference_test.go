package monitoring

import "testing"

// Reproduz o caso do cliente (j.abreu.castro): 2 chips, o grupo de destino tem
// os dois como membros mas o "dono" é o chip errado. Com o chip certo pinado na
// automação e sendo membro do grupo, o envio deve sair por ele.
func TestPreferredParticipantSession(t *testing.T) {
	connected := map[string]string{
		"3595": "session_num1", // número 1 (o que a cliente quer)
		"3597": "session_num2", // número 2 (dono do grupo)
	}

	tests := []struct {
		name         string
		preferred    []string
		participants []string
		want         string
	}{
		{
			name:         "pinned chip is a connected member -> usa o pinado",
			preferred:    []string{"3595"},
			participants: []string{"3597", "3595"}, // #01 tem os dois
			want:         "session_num1",
		},
		{
			name:         "sem chip pinado -> mantem roteamento default",
			preferred:    nil,
			participants: []string{"3597", "3595"},
			want:         "",
		},
		{
			name:         "pinado nao participa do grupo -> mantem default (evita not-participating)",
			preferred:    []string{"3595"},
			participants: []string{"3597"},
			want:         "",
		},
		{
			name:         "pinado desconectado -> mantem default",
			preferred:    []string{"9999"}, // ausente do connected map
			participants: []string{"9999", "3597"},
			want:         "",
		},
		{
			name:         "grupo sem dados de participacao -> mantem default",
			preferred:    []string{"3595"},
			participants: nil,
			want:         "",
		},
		{
			name:         "multiplos pinados -> pega o primeiro que e membro conectado",
			preferred:    []string{"9999", "3597", "3595"},
			participants: []string{"3595", "3597"},
			want:         "session_num2", // 3597 e o primeiro pinado valido
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preferredParticipantSession(tt.preferred, tt.participants, connected)
			if got != tt.want {
				t.Fatalf("preferredParticipantSession() = %q, want %q", got, tt.want)
			}
		})
	}
}
