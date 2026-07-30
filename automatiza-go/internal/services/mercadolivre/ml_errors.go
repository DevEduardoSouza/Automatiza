package mercadolivre

import (
	"errors"
	"time"
)

// Erros tipados do Mercado Livre.
//
// Antes disso o serviço devolvia só `fmt.Errorf` com texto livre, e os callers
// decidiam o que fazer casando SUBSTRING da mensagem (monitoring_service.go
// fazia strings.Contains(err, "credentials not configured")). Isso produziu um
// bug real e latente: CreateAffiliateLink embrulhava QUALQUER falha de
// LoadCredentials — inclusive timeout de Postgres ou ctx cancelado no shutdown
// do worker — em "ML credentials not configured", e o espelho PAUSAVA a
// automação inteira do cliente (inclusive a parte Shopee dela) por causa de um
// soluço de banco. É a mesma lição já aprendida na Shopee (shopee_service.go:66-78).
//
// A regra agora é: caller decide por errors.Is, nunca por texto.

var (
	// ErrMLNoCredentials — o usuário não conectou a conta ML. Estado PERMANENTE
	// até ele agir, e é o único caso em que faz sentido pedir ação ao cliente.
	ErrMLNoCredentials = errors.New("ml credentials not configured")

	// ErrMLCredentialsUnreadable — a credencial existe mas não deu para LER
	// agora (timeout de Postgres, ctx cancelado, falha ao decifrar). É
	// TRANSITÓRIO: repetir daqui a pouco tende a funcionar. Separar isto de
	// ErrMLNoCredentials é a correção central deste arquivo — colapsados na
	// mesma string, um soluço de banco virava "cliente sem credencial".
	ErrMLCredentialsUnreadable = errors.New("ml credentials could not be read")

	// ErrMLSessionExpired — o ML respondeu 401/403 e o refresh de CSRF não
	// recuperou. A sessão do navegador morreu; só o cliente reconectando
	// resolve. Distinto de ErrMLNoCredentials porque a ação do cliente é outra
	// ("reconecte" vs "conecte") e porque dispara o cooldown por usuário.
	ErrMLSessionExpired = errors.New("ml session expired — reconnect required")

	// ErrMLBlocked — o ML recusou de forma não-autenticável: 429, desafio de
	// WAF, ou corpo que não é JSON (página HTML de bloqueio). NUNCA retentar:
	// sondar um bloqueio é como se ganha um IP banido de vez.
	ErrMLBlocked = errors.New("ml blocked the request (rate limit or WAF)")

	// ErrMLUnavailable — breaker aberto. Retorna instantâneo para que um ML
	// fora do ar não queime o orçamento de contexto do scheduler serial e
	// deixe as outras automações (Shopee, listas) fluindo. Mesmo raciocínio de
	// ErrShopeeUnavailable (shopee/circuit_breaker.go:9-16), invertido.
	ErrMLUnavailable = errors.New("ml API temporarily unavailable (circuit breaker open)")

	// ErrMLUnconvertibleURL — URL do domínio ML que não é página de produto
	// (cupom, campanha, /l/ sem MLB). ESTRUTURAL: repetir amanhã dá o mesmo
	// erro. Existe para o caller distinguir "não converteu agora" (vale reter e
	// retentar) de "não há o que converter" — reter este engoliria a mensagem
	// para sempre. Precedente literal: monitoring_service.go:706-717 (Shopee).
	ErrMLUnconvertibleURL = errors.New("ml: URL is not a product page")
)

// sessionDeadCooldown é por quanto tempo as chamadas ao ML são puladas para um
// usuário depois de ErrMLSessionExpired. Sem isso, uma sessão morta faz o
// worker bater no linkbuilder a cada item de cada lista — milhares de requests
// autenticadas/dia contra o endpoint mais sensível do ML, todas fadadas a 401.
const sessionDeadCooldown = 30 * time.Minute

func sessionDeadKey(userID string) string { return "ml:session_dead:" + userID }
