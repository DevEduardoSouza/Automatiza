package selfsend

import (
	"context"
	"testing"

	"automatiza-go/internal/lib/cache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newMiniredisCache(t *testing.T) *cache.Cache {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return cache.NewCache(rdb, "test", false)
}

func TestMarkAndIsPlatformSend(t *testing.T) {
	c := newMiniredisCache(t)
	ctx := context.Background()
	group := "120363000000000000@g.us"
	body := "*PROMO* Produto X https://s.shopee.com.br/abc"

	if IsPlatformSend(ctx, c, group, body) {
		t.Fatal("nao deveria constar como envio da plataforma antes do Mark")
	}
	Mark(ctx, c, group, body)
	if !IsPlatformSend(ctx, c, group, body) {
		t.Fatal("deveria constar como envio da plataforma apos o Mark")
	}
	// Mesmo corpo em OUTRO grupo não pode dar match (chave inclui o grupo).
	if IsPlatformSend(ctx, c, "120363999999999999@g.us", body) {
		t.Fatal("match indevido em grupo diferente")
	}
	// Corpo diferente no mesmo grupo não pode dar match.
	if IsPlatformSend(ctx, c, group, body+" editado") {
		t.Fatal("match indevido para corpo diferente")
	}
}

func TestNilSafeAndEmptyArgs(t *testing.T) {
	ctx := context.Background()
	// Cache nil: fail-open, nunca bloqueia o espelho.
	Mark(ctx, nil, "g@g.us", "body")
	if IsPlatformSend(ctx, nil, "g@g.us", "body") {
		t.Fatal("cache nil deve retornar false (fail-open)")
	}
	// Corpo/grupo vazios nunca marcam nem dão match (evita colisao universal).
	c := newMiniredisCache(t)
	Mark(ctx, c, "g@g.us", "")
	if IsPlatformSend(ctx, c, "g@g.us", "") {
		t.Fatal("corpo vazio nao pode marcar/dar match")
	}
}
