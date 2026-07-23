package instagram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrInvalidWebhookSignature é a única saída de rejeição de assinatura. Um erro
// por causa viraria oráculo para quem estivesse sondando o endpoint.
var ErrInvalidWebhookSignature = errors.New("instagram: invalid webhook signature")

// VerifyWebhookSignature confere o header X-Hub-Signature-256 contra o corpo
// CRU, usando o app secret do cliente.
//
// O corpo tem de ser exatamente o que chegou: qualquer normalização (reserializar
// o JSON, por exemplo) muda os bytes e a assinatura deixa de bater. Por isso o
// handler lê os bytes primeiro e só decodifica depois de verificar.
func VerifyWebhookSignature(rawBody []byte, header, appSecret string) error {
	if appSecret == "" || header == "" {
		return ErrInvalidWebhookSignature
	}
	// Formato "sha256=<hex>".
	value, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return ErrInvalidWebhookSignature
	}
	want, err := hex.DecodeString(value)
	if err != nil {
		return ErrInvalidWebhookSignature
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(rawBody)
	// hmac.Equal: tempo constante. Um == comum vazaria, pelo tempo de resposta,
	// quantos bytes iniciais foram acertados.
	if !hmac.Equal(want, mac.Sum(nil)) {
		return ErrInvalidWebhookSignature
	}
	return nil
}

// webhookEnvelope é o formato documentado pela Meta: um objeto com `entry[]`,
// cada entrada trazendo `changes[]` (comentários, menções) OU `messaging[]`
// (Direct). O conteúdo INTERNO de cada um não é publicado na doc, então o
// parser abaixo é tolerante de propósito.
type webhookEnvelope struct {
	Object string `json:"object"`
	Entry  []struct {
		ID   string `json:"id"`
		Time int64  `json:"time"`

		Changes []struct {
			Field string          `json:"field"`
			Value json.RawMessage `json:"value"`
		} `json:"changes"`

		Messaging []struct {
			Sender struct {
				ID string `json:"id"`
			} `json:"sender"`
			Recipient struct {
				ID string `json:"id"`
			} `json:"recipient"`
			Timestamp int64 `json:"timestamp"`
			Message   struct {
				MID    string `json:"mid"`
				Text   string `json:"text"`
				IsEcho bool   `json:"is_echo"`
				IsSelf bool   `json:"is_self"`
			} `json:"message"`
		} `json:"messaging"`
	} `json:"entry"`
}

// changeValue cobre os campos que comentários e menções costumam trazer. Cada
// um é opcional: a doc pública não fixa o shape interno, então ausência é
// tratada como "não veio", nunca como erro.
type changeValue struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	ParentID string `json:"parent_id"`
	From     struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"from"`
	Media struct {
		ID string `json:"id"`
	} `json:"media"`
}

// InboundEvent é um evento já normalizado, pronto para virar linha no ledger.
type InboundEvent struct {
	ProviderEventID string
	EventType       string
	IGAccountID     string
	SenderID        string
	ThreadID        string
	Content         string
	OccurredAt      *time.Time
}

// ParseWebhook normaliza o payload da Meta numa lista de eventos.
//
// Tolerante de propósito: a doc pública documenta o ENVELOPE mas não o conteúdo
// interno de `comments`/`messages`. Campo que não vier é simplesmente ignorado,
// e forma desconhecida devolve lista vazia em vez de erro — um webhook novo da
// Meta não pode virar 500 num endpoint que ela usa para medir nossa saúde.
//
// O que NÃO é tolerante: evento sem identificador próprio é descartado, porque
// sem ele não há idempotência possível e a Meta reenvia.
func ParseWebhook(raw []byte, ownIGAccountIDs map[string]bool) ([]InboundEvent, error) {
	var env webhookEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}

	var out []InboundEvent
	for _, entry := range env.Entry {
		// entry.id é a conta do Instagram que sofreu o evento.
		account := entry.ID

		for _, ch := range entry.Changes {
			eventType := changeFieldToEventType(ch.Field)
			if eventType == "" {
				continue // campo que não tratamos (story_insights etc.)
			}
			var v changeValue
			if err := json.Unmarshal(ch.Value, &v); err != nil {
				continue
			}
			if v.ID == "" {
				continue // sem id não há idempotência
			}
			// Anti-loop: comentário do próprio dono da conta não é atendimento.
			if v.From.ID != "" && (v.From.ID == account || ownIGAccountIDs[v.From.ID]) {
				continue
			}
			out = append(out, InboundEvent{
				ProviderEventID: v.ID,
				EventType:       eventType,
				IGAccountID:     account,
				SenderID:        v.From.ID,
				ThreadID:        firstNonEmpty(v.Media.ID, v.ParentID, v.ID),
				Content:         v.Text,
				OccurredAt:      unixOrNil(entry.Time),
			})
		}

		for _, m := range entry.Messaging {
			// message_echoes: a própria conta enviando. Descartar é o que impede
			// duas contas nossas conversando entre si para sempre.
			if m.Message.IsEcho || m.Message.IsSelf {
				continue
			}
			if m.Message.MID == "" {
				continue
			}
			if m.Sender.ID != "" && (m.Sender.ID == account || ownIGAccountIDs[m.Sender.ID]) {
				continue
			}
			out = append(out, InboundEvent{
				ProviderEventID: m.Message.MID,
				EventType:       "message",
				IGAccountID:     firstNonEmpty(m.Recipient.ID, account),
				SenderID:        m.Sender.ID,
				ThreadID:        m.Sender.ID, // a conversa é identificada pelo interlocutor
				Content:         m.Message.Text,
				OccurredAt:      unixMillisOrNil(m.Timestamp),
			})
		}
	}
	return out, nil
}

func changeFieldToEventType(field string) string {
	switch field {
	case "comments", "live_comments":
		return "comment"
	case "mentions":
		return "mention"
	default:
		return ""
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// unixOrNil trata `entry.time`, que a Meta manda em SEGUNDOS.
func unixOrNil(sec int64) *time.Time {
	if sec <= 0 {
		return nil
	}
	t := time.Unix(sec, 0).UTC()
	return &t
}

// unixMillisOrNil trata `messaging[].timestamp`, que vem em MILISSEGUNDOS.
// Confundir as duas escalas jogaria a data para 1970 ou para o ano 56000 — e o
// gate de 24h do Direct depende dela para decidir se ainda dá para responder.
func unixMillisOrNil(ms int64) *time.Time {
	if ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}
