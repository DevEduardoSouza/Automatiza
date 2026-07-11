package whatsapp

import (
	"context"
	"errors"
	"net"
	"strings"
)

// IsAmbiguousOutcome reports whether a send error leaves the delivery outcome
// UNKNOWN: timeouts and cancellations happen no cliente HTTP, mas o WAHA pode
// já ter recebido e entregue a mensagem no grupo. Reenviar (fallback de outro
// formato) ou liberar o slot de dedup depois de um erro desses é exatamente o
// que duplica promoção em produção. Erros definitivos (4xx de validação, "not
// participating", sessão inexistente) retornam false — nesses o envio
// comprovadamente não aconteceu.
func IsAmbiguousOutcome(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection reset")
}
