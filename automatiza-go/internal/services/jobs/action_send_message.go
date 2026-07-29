package jobs

import (
	"context"
	"errors"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

const (
	// defaultVoiceMimetype é o default do WAHA para VoiceBinaryFile.
	defaultVoiceMimetype = "audio/ogg; codecs=opus"

	// voiceFollowUpDelay dá tempo do áudio assentar antes do texto complementar,
	// mesmo intervalo usado no envio de automação (message_worker.go:247).
	voiceFollowUpDelay = 1500 * time.Millisecond
)

// errAudioURLUnsupported: sendVoice só aceita o áudio em base64.
var errAudioURLUnsupported = errors.New("audio por URL nao suportado no agendamento")

// ExecuteSendMessage executes the send_message action for all target groups.
// Supports text messages with optional media (image/video) and @todos expansion.
func ExecuteSendMessage(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	sessionName string,
	job *models.StandaloneJob,
	logger *zap.Logger,
) []ActionResult {
	groups := job.Payload.Groups

	return ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		message := job.Payload.Message

		// Expand @todos to mention all participants
		var mentions []string
		if ContainsMentions(message) {
			mentions, message = ExpandMentions(ctx, provider, sessionName, groupID, message)
		}

		// Determine message type based on payload
		if job.Payload.Media != "" {
			// Has media attachment. Desfecho ambíguo = assumed-sent (ver
			// ambiguousAsSent): re-agendar o job re-enviaria a mídia ao grupo.
			return ambiguousAsSent(
				sendMessageWithMedia(ctx, provider, sessionName, groupID, message, mentions, &job.Payload, logger),
				logger, groupID)
		}

		// Text-only message
		input := whatsapp.SendTextInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Text:        message,
			Mentions:    mentions,
		}

		_, err := provider.SendText(ctx, input)
		if err != nil {
			logger.Warn("Failed to send text message",
				zap.String("groupID", groupID),
				zap.Error(err))
			return ambiguousAsSent(err, logger, groupID)
		}

		return nil
	})
}

func sendMessageWithMedia(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	sessionName, groupID, caption string,
	mentions []string,
	payload *models.JobPayload,
	logger *zap.Logger,
) error {
	mediaType := strings.ToLower(payload.MediaType)

	switch mediaType {
	case "image", "":
		// Default to image if not specified
		input := whatsapp.SendImageInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Caption:     caption,
			Filename:    payload.FileName,
			Mimetype:    payload.MimeType,
			Mentions:    mentions,
		}

		// Determine if media is URL or base64
		if strings.HasPrefix(payload.Media, "http") {
			input.ImageURL = payload.Media
		} else {
			input.ImageBase64 = payload.Media
		}

		_, err := provider.SendImage(ctx, input)
		return err

	case "video":
		// POST /api/sendVideo — aceita caption e convert (waha-plus-openapi.json:1602).
		input := whatsapp.SendVideoInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Caption:     caption,
			MimeType:    payload.MimeType,
			Filename:    payload.FileName,
			Mentions:    mentions,
		}

		// Exatamente uma fonte: Validate() rejeita as duas juntas ou nenhuma.
		if strings.HasPrefix(payload.Media, "http") {
			input.MediaURL = payload.Media
		} else {
			input.MediaBase64 = payload.Media
		}

		_, err := provider.SendVideo(ctx, input)
		return err

	case "audio":
		// POST /api/sendVoice — SendVoiceInput só aceita base64 (port.go:636),
		// então URL não tem caminho aqui: falhar alto é melhor que mandar a URL
		// como se fosse o áudio (era exatamente o defeito antigo do vídeo).
		if strings.HasPrefix(payload.Media, "http") {
			logger.Warn("Audio via URL is not supported by sendVoice",
				zap.String("groupID", groupID))
			return errAudioURLUnsupported
		}

		mimetype := payload.MimeType
		if mimetype == "" {
			mimetype = defaultVoiceMimetype
		}

		if _, err := provider.SendVoice(ctx, whatsapp.SendVoiceInput{
			SessionName: sessionName,
			ChatID:      groupID,
			AudioBase64: payload.Media,
			Mimetype:    mimetype,
		}); err != nil {
			return err
		}

		// Voz não tem caption (MessageVoiceRequest, waha-plus-openapi.json:1564):
		// o texto vai como mensagem separada, igual ao envio de automação
		// (message_worker.go:244).
		if caption == "" {
			return nil
		}

		select {
		case <-time.After(voiceFollowUpDelay):
		case <-ctx.Done():
			// O áudio JÁ foi entregue: retornar erro faria o job re-executar e o
			// grupo receberia o áudio 2x. Perder o texto é o mal menor.
			logger.Warn("Context done after voice sent; skipping follow-up text",
				zap.String("groupID", groupID))
			return nil
		}

		if _, err := provider.SendText(ctx, whatsapp.SendTextInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Text:        caption,
			Mentions:    mentions,
		}); err != nil {
			// Mesma razão do bloco acima: falha do texto é terminal.
			logger.Warn("Failed to send follow-up text after voice",
				zap.String("groupID", groupID),
				zap.Error(err))
		}
		return nil

	default:
		logger.Warn("Unknown media type, defaulting to image",
			zap.String("mediaType", mediaType))
		// Fall back to image
		input := whatsapp.SendImageInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Caption:     caption,
			Filename:    payload.FileName,
			Mimetype:    payload.MimeType,
			Mentions:    mentions,
		}
		if strings.HasPrefix(payload.Media, "http") {
			input.ImageURL = payload.Media
		} else {
			input.ImageBase64 = payload.Media
		}
		_, err := provider.SendImage(ctx, input)
		return err
	}
}
