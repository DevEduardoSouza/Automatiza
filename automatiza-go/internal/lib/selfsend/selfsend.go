// Package selfsend marca envios originados pela PRÓPRIA plataforma por
// (grupo, hash do corpo) para que os fluxos de espelho (monitoring /
// offer_generate) distingam post manual de disparo automatizado. O guard
// og_self_send cobre só o eco FromMe do MESMO chip; esta chave é global —
// pega disparo de lista/busca, relay de espelho e oferta gerada chegando
// num grupo monitorado por OUTRO chip (inclusive de outro usuário), o que
// evita cadeias lista → espelho → espelho amplificando tráfego automatizado.
package selfsend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"automatiza-go/internal/lib/cache"
)

// ttl cobre o atraso envio → webhook nos chips membros com folga (normalmente
// segundos; minutos sob WAHA saturado), mas curto o bastante para não engolir
// um humano repostando o mesmo texto bem mais tarde.
const ttl = 5 * time.Minute

func key(group, body string) string {
	h := sha256.Sum256([]byte(body))
	return fmt.Sprintf("platform_send:%s:%s", group, hex.EncodeToString(h[:8]))
}

// Mark registra que a plataforma está prestes a enviar `body` (o texto FINAL,
// pós-variação — é o byte exato que o webhook devolve) para `group`. Chamar
// ANTES do envio real; se o envio falhar a chave órfã expira sem efeito.
// Nil-safe e fail-open: erro de Redis não bloqueia o envio.
func Mark(ctx context.Context, c *cache.Cache, group, body string) {
	if c == nil || group == "" || body == "" {
		return
	}
	_ = c.Set(ctx, key(group, body), "1", ttl)
}

// IsPlatformSend informa se `body` chegando em `group` corresponde a um envio
// recente da própria plataforma. Fail-open: sem cache ou erro de Redis =>
// false (a mensagem é espelhada normalmente, comportamento antigo).
func IsPlatformSend(ctx context.Context, c *cache.Cache, group, body string) bool {
	if c == nil || group == "" || body == "" {
		return false
	}
	ok, err := c.Exists(ctx, key(group, body))
	return err == nil && ok
}
