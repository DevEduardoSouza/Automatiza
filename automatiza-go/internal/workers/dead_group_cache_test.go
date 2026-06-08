package workers

import (
	"testing"
	"time"
)

func TestDeadGroupCache_MarkAndExpire(t *testing.T) {
	c := newDeadGroupCache()
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	if c.isDead("s1", "g1@g.us", now) {
		t.Fatal("grupo nao deveria estar morto antes de marcar")
	}

	c.markDead("s1", "g1@g.us", now)
	if !c.isDead("s1", "g1@g.us", now) {
		t.Fatal("grupo deveria estar morto logo apos marcar")
	}

	// Outro grupo/sessao nao deve ser afetado.
	if c.isDead("s1", "g2@g.us", now) {
		t.Fatal("grupo diferente nao deveria estar morto")
	}
	if c.isDead("s2", "g1@g.us", now) {
		t.Fatal("sessao diferente nao deveria estar morta")
	}

	// Ainda morto pouco antes do TTL.
	if !c.isDead("s1", "g1@g.us", now.Add(deadGroupTTL-time.Second)) {
		t.Fatal("grupo deveria continuar morto dentro do TTL")
	}

	// Expira depois do TTL.
	if c.isDead("s1", "g1@g.us", now.Add(deadGroupTTL+time.Second)) {
		t.Fatal("grupo deveria expirar apos o TTL")
	}
}

func TestDeadGroupCache_NilSafeAndEmptyKeys(t *testing.T) {
	var c *deadGroupCache // nil
	now := time.Now()
	c.markDead("s1", "g1", now) // nao deve panicar
	if c.isDead("s1", "g1", now) {
		t.Fatal("cache nil nunca reporta morto")
	}

	c = newDeadGroupCache()
	c.markDead("", "g1", now)
	c.markDead("s1", "", now)
	if c.isDead("", "g1", now) || c.isDead("s1", "", now) {
		t.Fatal("chaves vazias devem ser ignoradas")
	}
}

func TestIsNotParticipatingError(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{"waha not participating", "failed to send image message: WAHA error 500: you're not participating in that group", true},
		{"group members", "2 UNKNOWN: failed to get group members: you're not participating in that group", true},
		{"unrelated timeout", "context deadline exceeded", false},
		{"unrelated 404", "instance not found", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.msg != "" {
				err = errText(tc.msg)
			}
			if got := isNotParticipatingError(err); got != tc.want {
				t.Fatalf("isNotParticipatingError(%q) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}
}

type errText string

func (e errText) Error() string { return string(e) }
