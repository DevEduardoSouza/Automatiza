package jobs

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// ExecuteSendMessage Tests
// =============================================================================

func TestExecuteSendMessage_TextOnly_Success(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:  []string{"group1@g.us"},
			Message: "Hello group!",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, "group1@g.us", results[0].GroupID)
	assert.Equal(t, 1, provider.CallCount("SendText"))
	assert.Equal(t, 0, provider.CallCount("SendImage"))
}

func TestExecuteSendMessage_TextOnly_MultipleGroups(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:  []string{"group1@g.us", "group2@g.us", "group3@g.us"},
			Message: "Broadcast message",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Success)
	}
	assert.Equal(t, 3, provider.CallCount("SendText"))
}

func TestExecuteSendMessage_TextOnly_ProviderError(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SendTextFunc = func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		return nil, errors.New("WAHA error")
	}
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:  []string{"group1@g.us"},
			Message: "Test",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, "WAHA error", results[0].Error)
}

func TestExecuteSendMessage_WithImageBase64(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			Message:   "Check this out",
			Media:     "base64ImageData",
			MediaType: "image",
			FileName:  "photo.jpg",
			MimeType:  "image/jpeg",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 0, provider.CallCount("SendText"))
	assert.Equal(t, 1, provider.CallCount("SendImage"))

	// Verify input was sent with base64
	call := provider.LastCall("SendImage")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendImageInput)
	assert.Equal(t, "base64ImageData", input.ImageBase64)
	assert.Empty(t, input.ImageURL)
	assert.Equal(t, "Check this out", input.Caption)
}

func TestExecuteSendMessage_WithImageURL(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			Message:   "From URL",
			Media:     "https://example.com/image.jpg",
			MediaType: "image",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("SendImage"))

	call := provider.LastCall("SendImage")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendImageInput)
	assert.Equal(t, "https://example.com/image.jpg", input.ImageURL)
	assert.Empty(t, input.ImageBase64)
}

func TestExecuteSendMessage_WithVideo(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			Media:     "base64VideoData",
			MediaType: "video",
			MimeType:  "video/mp4",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("SendVoice"))
}

func TestExecuteSendMessage_WithUnknownMediaType(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			Media:     "data",
			MediaType: "audio",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	// Falls back to image
	assert.Equal(t, 1, provider.CallCount("SendImage"))
}

func TestExecuteSendMessage_WithEmptyMediaType_DefaultsToImage(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			Media:     "base64data",
			MediaType: "",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("SendImage"))
}

func TestExecuteSendMessage_WithMentions(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SetParticipants("session1", "group1@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511999999999@s.whatsapp.net"},
		{ID: "5511888888888@s.whatsapp.net"},
	})
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:  []string{"group1@g.us"},
			Message: "Hello @todos!",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("GetGroupParticipants"))

	call := provider.LastCall("SendText")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendTextInput)
	assert.Contains(t, input.Text, "@5511999999999")
	assert.Contains(t, input.Text, "@5511888888888")
	assert.NotContains(t, input.Text, "@todos")
	assert.Len(t, input.Mentions, 2)
}

func TestExecuteSendMessage_MediaError(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SendImageFunc = func(_ context.Context, _ whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
		return nil, errors.New("upload failed")
	}
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			Media:     "data",
			MediaType: "image",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, "upload failed", results[0].Error)
}

func TestExecuteSendMessage_VideoURL_FallsBack(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			Media:     "https://example.com/video.mp4",
			MediaType: "Video", // uppercase test
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("SendVoice"))
}

func TestExecuteSendMessage_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:  []string{"group1@g.us", "group2@g.us"},
			Message: "Test",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	assert.Len(t, results, 2)
	hasCancelled := false
	for _, r := range results {
		if !r.Success && r.Error == "canceled" {
			hasCancelled = true
		}
	}
	assert.True(t, hasCancelled)
}

func TestExecuteSendMessage_UnknownMediaType_WithURL(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMessage,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			Media:     "https://example.com/doc.pdf",
			MediaType: "document",
		},
	}

	results := ExecuteSendMessage(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	// Falls back to image with URL
	call := provider.LastCall("SendImage")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendImageInput)
	assert.Equal(t, "https://example.com/doc.pdf", input.ImageURL)
}
