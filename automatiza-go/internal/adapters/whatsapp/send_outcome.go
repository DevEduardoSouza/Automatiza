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
//
// IMPORTANTE: este conjunto tem que ser um SUPERSET dos erros que o adapter
// classifica como transporte-transitório em endpoints de send (doRequest, R2):
// connection reset, EOF, broken pipe, i/o timeout, falha de leitura da
// resposta, 502/504 do gateway. O adapter embrulha todos em ProviderError
// ErrCodeTimeout — reconhecido tipadamente abaixo — mas os padrões de string
// cobrem erros que cheguem por caminhos que não passaram pelo doRequest.
// Divergência entre os dois conjuntos foi a causa de "imagem entregue + texto
// de fallback" duplicando promoção no grupo.
func IsAmbiguousOutcome(err error) bool {
	if err == nil {
		return false
	}

	// Caminho tipado: o adapter sinaliza desfecho-desconhecido como
	// ErrCodeTimeout ("request timed out", "ambiguous send outcome ...").
	var pe *ProviderError
	if errors.As(err, &pe) && pe.Code == ErrCodeTimeout {
		return true
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
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "unexpected eof") ||
		// io.EOF embrulhado vira "...: EOF" — sufixo, para não casar palavras
		// que contenham "eof" no meio de outra mensagem.
		strings.HasSuffix(msg, "eof") ||
		strings.Contains(msg, "connection closed") ||
		strings.Contains(msg, "ambiguous send outcome")
}
