// Package affiliatelink roteia uma URL de loja para o conversor da loja.
//
// Existe para que os caminhos quentes (espelho, lista, relays de cupom) ganhem
// lojas novas sem um switch por call site. O contrato é deliberadamente magro —
// uma string de volta — porque os callers já têm política de falha própria e
// madura; ver o comentário de Convert.
package affiliatelink

import (
	"context"
	"errors"

	"automatiza-go/internal/lib/commerceurl"
)

// Erros sentinela dos conversores.
//
// ATENÇÃO ao texto destas mensagens: process_list.go e offer_generate_service.go
// classificam erro de credencial por SUBSTRING em inglês ("credentials not
// configured", "credentials incomplete", "Invalid Signature") e PAUSAM a
// automação do cliente quando casam. Uma loja nova nunca pode pausar a
// automação Shopee de ninguém, então as mensagens aqui são em português e não
// contêm nenhuma dessas expressões.
var (
	// ErrNoCredentials: a loja não está configurada para este usuário.
	ErrNoCredentials = errors.New("affiliatelink: loja sem credencial cadastrada")
	// ErrUnconvertible: a URL não tem identificador que permita reatribuir a
	// comissão (encurtador não expandido, busca, storefront, marca fora do
	// programa). Permanente para esta URL.
	ErrUnconvertible = errors.New("affiliatelink: URL sem identificador convertivel")
	// ErrUnavailable: o provedor falhou de forma transitória (timeout, 5xx,
	// rate limit, breaker aberto). Vale tentar de novo depois.
	ErrUnavailable = errors.New("affiliatelink: provedor indisponivel no momento")
	// ErrRejectedCredential: o provedor recusou a credencial de forma
	// determinística. Pede ação do usuário, mas NÃO pausa a automação.
	ErrRejectedCredential = errors.New("affiliatelink: provedor recusou a credencial")
)

// Converter transforma a URL de uma loja no link de afiliado do usuário.
//
// Recebe a URL já parseada e validada por commerceurl (host normalizado,
// anti-spoof aplicado) para que nenhum conversor precise reimplementar isso.
type Converter interface {
	Convert(ctx context.Context, u commerceurl.URL, userID string) (string, error)
}

// Registry mapeia loja (models.MonitorStore*) para conversor.
type Registry struct {
	byStore map[string]Converter
}

// New monta o registry. Entradas nil são ignoradas, então o caller pode passar
// um conversor que não foi construído (feature desligada) sem checar antes.
func New(converters map[string]Converter) *Registry {
	byStore := make(map[string]Converter, len(converters))
	for store, c := range converters {
		if store == "" || c == nil {
			continue
		}
		byStore[store] = c
	}
	return &Registry{byStore: byStore}
}

// Handles reporta se existe conversor para a loja. Registry nil não resolve
// nada — é o que faz "sem wiring" e "gate desligado" serem idênticos ao
// comportamento anterior à feature, sem nenhum if extra nos callers.
func (r *Registry) Handles(store string) bool {
	if r == nil || store == "" {
		return false
	}
	_, ok := r.byStore[store]
	return ok
}

// Convert resolve uma URL crua para o link de afiliado do usuário.
//
// Os três desfechos são distintos de propósito:
//
//	ok=false, err=nil  → não há conversor para esta loja. O caller deve manter
//	                     exatamente o comportamento que tinha antes da feature.
//	ok=false, err!=nil → havia conversor e ele falhou. O caller deve REMOVER o
//	                     link do corpo: repassar o original entregaria a
//	                     comissão a quem postou no grupo-fonte.
//	ok=true            → converted é o link do tenant.
//
// Nunca devolve a URL de origem em converted.
func (r *Registry) Convert(ctx context.Context, raw, userID string) (string, bool, error) {
	if r == nil || raw == "" || userID == "" {
		return "", false, nil
	}

	u, err := commerceurl.Parse(raw)
	if err != nil {
		// URL bloqueada por segurança (spoof, IP literal, scheme estranho).
		// Não é "sem conversor": é link que não pode ser emitido de jeito
		// nenhum, então o caller precisa removê-lo.
		return "", false, ErrUnconvertible
	}

	c, ok := r.byStore[u.Store]
	if !ok {
		return "", false, nil
	}

	converted, err := c.Convert(ctx, u, userID)
	if err != nil {
		return "", false, err
	}
	if converted == "" || converted == raw {
		// Conversor que devolve vazio (ou a própria origem) é bug de
		// implementação; tratar como falha impede o vazamento de comissão.
		return "", false, ErrUnconvertible
	}
	return converted, true, nil
}
