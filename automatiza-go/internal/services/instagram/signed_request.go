package instagram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// signedRequestMaxFuture tolera relógios levemente adiantados sem aceitar um
// payload forjado com data distante.
const signedRequestMaxFuture = 5 * time.Minute

// ErrInvalidSignedRequest é a ÚNICA saída de erro do parser. Um erro por causa
// (assinatura errada, base64 quebrado, algoritmo estranho) viraria um oráculo
// para quem estivesse sondando o endpoint.
var ErrInvalidSignedRequest = errors.New("instagram: invalid signed request")

// SignedRequestPayload é o conteúdo verificado de um signed_request da Meta.
// UserID aqui é o id APP-SCOPED — o mesmo gravado em
// instagram_credentials.app_scoped_user_id.
type SignedRequestPayload struct {
	Algorithm string `json:"algorithm"`
	UserID    string `json:"user_id"`
	IssuedAt  int64  `json:"issued_at"`
	Expires   int64  `json:"expires"`
}

// ParseSignedRequest valida e decodifica o signed_request que a Meta envia nos
// callbacks de desautorização e de exclusão de dados.
//
// Formato: "<assinatura base64url>.<payload base64url>", com HMAC-SHA256 do
// app secret sobre o SEGMENTO CODIFICADO do payload (não sobre o JSON decodificado).
//
// Sem esta verificação, qualquer um podia POSTar no endpoint e apagar a conta
// de um cliente — os callbacks são públicos por contrato da Meta e a assinatura
// é a única autenticação que existe neles.
func ParseSignedRequest(raw, appSecret string, now time.Time) (*SignedRequestPayload, error) {
	if raw == "" || appSecret == "" {
		return nil, ErrInvalidSignedRequest
	}

	sigPart, payloadPart, found := strings.Cut(raw, ".")
	if !found || sigPart == "" || payloadPart == "" {
		return nil, ErrInvalidSignedRequest
	}
	// Um segundo ponto significa formato desconhecido — não é um signed_request.
	if strings.Contains(payloadPart, ".") {
		return nil, ErrInvalidSignedRequest
	}

	// A Meta envia base64url SEM padding.
	sig, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil {
		return nil, ErrInvalidSignedRequest
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return nil, ErrInvalidSignedRequest
	}

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(payloadPart))
	// hmac.Equal: comparação em tempo constante. Um == comum vazaria, pelo
	// tempo de resposta, quantos bytes iniciais da assinatura foram acertados.
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, ErrInvalidSignedRequest
	}

	var p SignedRequestPayload
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		return nil, ErrInvalidSignedRequest
	}
	// Só HMAC-SHA256. Aceitar o algoritmo declarado no próprio payload sem
	// travá-lo é como aceitar "alg: none" em JWT.
	if !strings.EqualFold(p.Algorithm, "HMAC-SHA256") || p.UserID == "" {
		return nil, ErrInvalidSignedRequest
	}
	if p.IssuedAt > 0 && time.Unix(p.IssuedAt, 0).After(now.Add(signedRequestMaxFuture)) {
		return nil, ErrInvalidSignedRequest
	}
	if p.Expires > 0 && !time.Unix(p.Expires, 0).After(now) {
		return nil, ErrInvalidSignedRequest
	}
	return &p, nil
}
