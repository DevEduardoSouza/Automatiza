// Package relaymedia envia a mídia de uma mensagem-fonte para um destino,
// escolhendo o método WAHA pelo tipo, e prepara os bytes UMA vez por webhook.
//
// Extraído de offer_generate.sendRelayMessage/prepareStreamableVideo, que já
// eram o caminho maduro. O monitoring só sabia enviar imagem — vídeo, áudio e
// documento degradavam para texto lá.
//
// Três coisas mudam em relação ao original, todas por segurança ou por perda
// de dado, nunca por gosto:
//
//  1. MediaURL NUNCA é devolvida à WAHA como fallback. A URL do store interno
//     (/api/files) é protegida por API key; passá-la em SendImage faz a WAHA
//     buscá-la sozinha SEM a key e levar 401→500. Sem bytes, mandamos só a
//     legenda e registramos media_unavailable.
//  2. Tipo desconhecido não é mais "tenta como imagem". Mandar um documento ou
//     sticker por SendImage produz um envio corrompido no grupo do cliente;
//     agora é unsupported_media, com texto permitido se houver corpo.
//  3. Log não carrega MediaURL — só hash8 e mimetype.
package relaymedia

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/videostream"
	"automatiza-go/internal/lib/wamentions"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// Prepared é a mídia já resolvida, pronta para o fan-out.
//
// Existe para que o download aconteça UMA vez por webhook, e não uma vez por
// destino: um espelho com 30 grupos baixaria o mesmo arquivo 30 vezes.
type Prepared struct {
	// Base64 são os bytes prontos. Vazio = não conseguimos a mídia.
	Base64 string
	Mime   string
	// SHA256 dos bytes, em hex. É o que distingue duas imagens sem legenda —
	// sem ele, ambas colapsam no hash do corpo vazio e o dedup confunde as duas.
	SHA256 string
	// VideoStreamable indica que o vídeo já foi remuxado para faststart, então
	// a WAHA não precisa transcodificar por grupo.
	VideoStreamable bool
	// Reason != models.ReasonNone quando a mídia não pôde ser obtida.
	Reason string
}

// HasBytes reporta se há mídia para enviar.
func (p Prepared) HasBytes() bool { return p.Base64 != "" }

// Fingerprint devolve a impressão digital da mensagem para o guard de
// self-send e o dedup de conteúdo.
//
// Mensagem SEM mídia devolve exatamente o corpo — assim a chave textual de
// hoje é preservada byte a byte e o dedup em voo não quebra no deploy.
// Com mídia, entra body ‖ NUL ‖ mime ‖ NUL ‖ sha256(bytes).
func (p Prepared) Fingerprint(body string) string {
	if p.SHA256 == "" {
		return body
	}
	sum := sha256.Sum256([]byte(body + "\x00" + strings.ToLower(p.Mime) + "\x00" + p.SHA256))
	return "fp:" + hex.EncodeToString(sum[:])
}

// Downloader é a parte do provider que o Prepare usa. Interface estreita de
// propósito: o teste não precisa de um WhatsAppProvider inteiro.
type Downloader interface {
	DownloadMediaByURL(ctx context.Context, mediaURL string) (*whatsapp.DownloadMediaResult, error)
}

// Prepare resolve os bytes da mídia uma única vez.
//
// Ordem: base64 inline do webhook primeiro (custo zero), senão download
// autenticado do store da WAHA. Nunca devolve MediaURL para uso posterior.
func Prepare(ctx context.Context, dl Downloader, msg *models.IncomingMessage, logger *zap.Logger) Prepared {
	out := Prepared{Mime: msg.MimeType, Reason: models.ReasonNone}
	if !msg.HasMedia {
		return out
	}

	var raw []byte
	if msg.MediaBase64 != "" {
		if b, err := base64.StdEncoding.DecodeString(msg.MediaBase64); err == nil {
			raw = b
		}
	}
	if len(raw) == 0 && msg.MediaURL != "" && dl != nil {
		res, err := dl.DownloadMediaByURL(ctx, msg.MediaURL)
		switch {
		case err != nil:
			logger.Warn("relaymedia: download falhou, seguindo sem bytes",
				zap.String("media_hash", hash8(msg.MediaURL)),
				zap.String("mime", msg.MimeType),
				zap.String("media_type", msg.MediaType),
				zap.Error(err))
		case res != nil && len(res.Data) > 0:
			raw = res.Data
			if out.Mime == "" {
				out.Mime = res.Mimetype
			}
		}
	}

	if len(raw) == 0 {
		// Sem bytes: NÃO devolvemos MediaURL. O envio segue só com a legenda.
		out.Reason = models.ReasonMediaUnavailable
		return out
	}

	sum := sha256.Sum256(raw)
	out.SHA256 = hex.EncodeToString(sum[:])

	// Vídeo: remuxa para faststart uma vez, aqui, em vez de deixar a WAHA
	// transcodificar por grupo.
	if strings.EqualFold(msg.MediaType, "video") && videostream.Available() {
		fctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if remuxed, err := videostream.EnsureFaststart(fctx, raw); err == nil {
			raw = remuxed
			out.VideoStreamable = true
		} else {
			logger.Warn("relaymedia: faststart falhou, a WAHA converte",
				zap.String("group", msg.RemoteJID), zap.Error(err))
		}
	}

	out.Base64 = base64.StdEncoding.EncodeToString(raw)
	return out
}

func hash8(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// SendInput descreve um envio de relay.
type SendInput struct {
	Session string
	ChatID  string
	// Body é a legenda/texto JÁ processado pelo chamador (separação de emoji,
	// substituição de links, remoção de tags).
	Body      string
	MediaType string
	Media     Prepared
	Mentions  []string
}

// Sender é a parte do provider usada no envio.
type Sender interface {
	SendText(ctx context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error)
	SendImage(ctx context.Context, in whatsapp.SendImageInput) (*whatsapp.MessageResult, error)
	SendVideo(ctx context.Context, in whatsapp.SendVideoInput) (*whatsapp.MessageResult, error)
	SendVoice(ctx context.Context, in whatsapp.SendVoiceInput) (*whatsapp.MessageResult, error)
	SendDocument(ctx context.Context, in whatsapp.SendDocumentInput) (*whatsapp.MessageResult, error)
}

// Result reporta o desfecho de um envio.
type Result struct {
	Reason string
	// TextOnly indica que a mídia não saiu e apenas o texto foi enviado.
	TextOnly bool
}

// Send dispara o método WAHA correto para o tipo de mídia.
//
// Sem mídia (ou sem bytes) manda texto — e é aqui que a diferença com o código
// antigo aparece: em vez de entregar MediaURL à WAHA e receber 401→500, o
// cliente recebe pelo menos a legenda, e o motivo fica registrado.
func Send(ctx context.Context, snd Sender, in SendInput, logger *zap.Logger) (Result, error) {
	body := in.Body

	if !in.Media.HasBytes() {
		if strings.TrimSpace(body) == "" {
			// Nem mídia nem texto: não há mensagem. Não é erro — é o evento de
			// sistema que a WAHA encaminhou (reaction, edit).
			return Result{Reason: in.Media.Reason, TextOnly: true}, nil
		}
		_, err := snd.SendText(ctx, whatsapp.SendTextInput{
			SessionName: in.Session, ChatID: in.ChatID, Text: body, Mentions: in.Mentions,
		})
		return Result{Reason: in.Media.Reason, TextOnly: true}, err
	}

	switch strings.ToLower(in.MediaType) {
	case "image":
		_, err := snd.SendImage(ctx, whatsapp.SendImageInput{
			SessionName: in.Session, ChatID: in.ChatID, Caption: body,
			Mimetype: in.Media.Mime, Mentions: in.Mentions, ImageBase64: in.Media.Base64,
		})
		return Result{Reason: models.ReasonNone}, err

	case "video":
		_, err := snd.SendVideo(ctx, whatsapp.SendVideoInput{
			SessionName: in.Session, ChatID: in.ChatID, Caption: body,
			Mentions: in.Mentions, MediaBase64: in.Media.Base64,
			AlreadyStreamable: in.Media.VideoStreamable,
		})
		return Result{Reason: models.ReasonNone}, err

	case "audio", "voice", "ptt":
		mime := in.Media.Mime
		if mime == "" {
			mime = "audio/ogg; codecs=opus"
		}
		_, err := snd.SendVoice(ctx, whatsapp.SendVoiceInput{
			SessionName: in.Session, ChatID: in.ChatID,
			AudioBase64: in.Media.Base64, Mimetype: mime,
		})
		return Result{Reason: models.ReasonNone}, err

	case "document", "file":
		_, err := snd.SendDocument(ctx, whatsapp.SendDocumentInput{
			SessionName: in.Session, ChatID: in.ChatID, Caption: body,
			Mentions: in.Mentions, MediaBase64: in.Media.Base64,
		})
		return Result{Reason: models.ReasonNone}, err

	default:
		// Tipo desconhecido NÃO vira imagem. Um documento ou sticker enviado
		// por SendImage chega corrompido no grupo; o cliente vê um arquivo
		// quebrado e não sabe por quê. Preferimos entregar o texto e registrar
		// unsupported_media.
		logger.Warn("relaymedia: tipo de mídia sem método dedicado",
			zap.String("media_type", in.MediaType), zap.String("mime", in.Media.Mime))
		if strings.TrimSpace(body) != "" {
			_, err := snd.SendText(ctx, whatsapp.SendTextInput{
				SessionName: in.Session, ChatID: in.ChatID, Text: body, Mentions: in.Mentions,
			})
			return Result{Reason: models.ReasonUnsupportedMedia, TextOnly: true}, err
		}
		return Result{Reason: models.ReasonUnsupportedMedia, TextOnly: true}, nil
	}
}

// ExpandMentions resolve "@todos" para os membros do grupo de DESTINO.
// Fino de propósito: só embrulha o helper existente para os dois modos
// chamarem a mesma coisa.
func ExpandMentions(ctx context.Context, p whatsapp.WhatsAppProvider, session, chatID, body string) []string {
	if !wamentions.Contains(body) {
		return nil
	}
	return wamentions.Expand(ctx, p, session, chatID, body)
}
