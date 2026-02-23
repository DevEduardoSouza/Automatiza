package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhatsAppMessageWebhook_IsGroupMessage(t *testing.T) {
	t.Parallel()

	// NOTE: IsGroupMessage has a known bug — it compares the last 4 characters
	// against "@g.us" (5 characters), so the comparison can never succeed.
	// The function always returns false for valid inputs (len >= 4).
	// Additionally, inputs with 1-3 character From field will cause a panic
	// because From[len(From)-4:] produces a negative index.

	t.Run("From with group JID returns false (bug: -4 vs -5)", func(t *testing.T) {
		t.Parallel()
		w := &WhatsAppMessageWebhook{From: "120363022145917299@g.us"}
		// Due to bug: From[len-4:] = "g.us" != "@g.us"
		assert.False(t, w.IsGroupMessage())
	})

	t.Run("To with group JID returns false (bug: -4 vs -5)", func(t *testing.T) {
		t.Parallel()
		w := &WhatsAppMessageWebhook{From: "5511999999999@c.us", To: "120363022145917299@g.us"}
		assert.False(t, w.IsGroupMessage())
	})

	t.Run("both with group JID returns false (bug)", func(t *testing.T) {
		t.Parallel()
		w := &WhatsAppMessageWebhook{From: "120363022@g.us", To: "120363023@g.us"}
		assert.False(t, w.IsGroupMessage())
	})

	t.Run("neither is group", func(t *testing.T) {
		t.Parallel()
		w := &WhatsAppMessageWebhook{From: "5511999@c.us", To: "5511888@c.us"}
		assert.False(t, w.IsGroupMessage())
	})

	t.Run("From empty short-circuits to false", func(t *testing.T) {
		t.Parallel()
		w := &WhatsAppMessageWebhook{From: ""}
		assert.False(t, w.IsGroupMessage())
	})

	t.Run("From exactly @g.us (len=5) returns false (bug)", func(t *testing.T) {
		t.Parallel()
		w := &WhatsAppMessageWebhook{From: "@g.us"}
		// From[5-4:] = From[1:] = "g.us" != "@g.us"
		assert.False(t, w.IsGroupMessage())
	})

	t.Run("From with 4 chars no panic", func(t *testing.T) {
		t.Parallel()
		w := &WhatsAppMessageWebhook{From: "abcd"}
		assert.False(t, w.IsGroupMessage())
	})

	t.Run("From with 1-3 chars panics (bug: no len guard)", func(t *testing.T) {
		t.Parallel()
		for _, from := range []string{"a", "ab", "abc"} {
			from := from
			t.Run("len="+from, func(t *testing.T) {
				t.Parallel()
				w := &WhatsAppMessageWebhook{From: from}
				assert.Panics(t, func() { w.IsGroupMessage() })
			})
		}
	})

	t.Run("To empty with non-group From", func(t *testing.T) {
		t.Parallel()
		w := &WhatsAppMessageWebhook{From: "5511999@c.us", To: ""}
		assert.False(t, w.IsGroupMessage())
	})
}
