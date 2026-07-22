package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"strings"
	"time"

	// Decoders for re-encoding non-JPEG WhatsApp media to the JPEG that the
	// Instagram Story API requires (stickers arrive as webp, some images png).
	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"automatiza-go/internal/lib/imagerender"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/instagram"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// storyDailyCap is a local guard below Instagram's ~100 posts/24h limit. The
// authoritative source is GetPublishingLimit, but a cheap local count avoids
// wasting a container create when we already published a lot for this user.
const storyDailyCap = 90

// InstagramStoryPublisher is the slice of the Instagram service the worker needs.
type InstagramStoryPublisher interface {
	PublishStoryImage(ctx context.Context, userID, publicImageURL string) (string, error)
}

// InstagramStoryWorker consumes instagram:publish_story tasks: it materializes
// a public JPEG URL (mirrored image or text-rendered), publishes the story, and
// records the result in the ledger.
type InstagramStoryWorker struct {
	svc           InstagramStoryPublisher
	store         *instagram.MediaStore
	storyRepo     repository.InstagramStoryPostRepository
	publicBaseURL string
	logger        *zap.Logger
}

// NewInstagramStoryWorker builds the worker.
func NewInstagramStoryWorker(
	svc InstagramStoryPublisher,
	store *instagram.MediaStore,
	storyRepo repository.InstagramStoryPostRepository,
	publicBaseURL string,
	logger *zap.Logger,
) *InstagramStoryWorker {
	return &InstagramStoryWorker{
		svc:           svc,
		store:         store,
		storyRepo:     storyRepo,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		logger:        logger,
	}
}

// ProcessTask implements the asynq handler.
func (w *InstagramStoryWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var data InstagramStoryJobData
	if err := json.Unmarshal(t.Payload(), &data); err != nil {
		// Malformed payload never becomes valid — don't retry.
		return fmt.Errorf("instagram story: bad payload: %v: %w", err, asynq.SkipRetry)
	}

	// Local daily-cap guard (permanent for this run).
	if n, err := w.storyRepo.CountPublishedSince(ctx, data.UserID, time.Now().Add(-24*time.Hour)); err == nil && n >= storyDailyCap {
		w.finishFailed(ctx, data.LedgerID, "limite diário de stories atingido")
		return fmt.Errorf("instagram story: daily cap reached (%d): %w", n, asynq.SkipRetry)
	}

	reuseKey, jpegBytes, err := w.prepareImage(ctx, &data)
	if err != nil {
		if errors.Is(err, instagram.ErrMediaNotFound) {
			w.finishFailed(ctx, data.LedgerID, "mídia expirou antes da publicação")
			return fmt.Errorf("instagram story: media expired: %w", asynq.SkipRetry)
		}
		// Render/decode failures are permanent for this payload.
		w.finishFailed(ctx, data.LedgerID, "falha ao preparar imagem: "+err.Error())
		return fmt.Errorf("instagram story: prepare image: %v: %w", err, asynq.SkipRetry)
	}

	// Resolve the public serving key. When the mirrored image is already JPEG
	// we serve the shared MediaKey DIRECTLY and never delete it — it is shared
	// across every owner of the source group and expires by TTL. Deleting it
	// here made the second owner's job fail with "media expired". Only a key we
	// created ourselves (rendered text / re-encoded non-JPEG) is deleted.
	servingKey := reuseKey
	if servingKey == "" {
		servingKey, err = w.store.Put(ctx, jpegBytes, "image/jpeg", storyMediaTTL)
		if err != nil {
			return fmt.Errorf("instagram story: store serving image: %w", err) // transient
		}
		defer w.store.Delete(context.WithoutCancel(ctx), servingKey)
	}

	publicURL := w.publicBaseURL + "/api/v1/public/story-media/" + servingKey
	mediaID, err := w.svc.PublishStoryImage(ctx, data.UserID, publicURL)
	if err != nil {
		if errors.Is(err, instagram.ErrInstagramAuth) {
			w.finishFailed(ctx, data.LedgerID, "conta Instagram precisa reconectar: "+err.Error())
			return fmt.Errorf("instagram story: auth error: %v: %w", err, asynq.SkipRetry)
		}
		return fmt.Errorf("instagram story: publish: %w", err) // transient → asynq retries
	}

	// Persist success with a context that survives task cancellation.
	if err := w.storyRepo.MarkResult(context.WithoutCancel(ctx), data.LedgerID,
		models.InstagramStoryStatusPublished, mediaID, ""); err != nil {
		w.logger.Warn("instagram story: mark published failed", zap.String("ledgerID", data.LedgerID), zap.Error(err))
	}
	w.logger.Info("instagram story published",
		zap.String("userID", data.UserID),
		zap.String("igMediaId", mediaID),
		zap.String("sourceGroup", data.SourceGroup),
		zap.String("mediaSource", sourceOf(&data)))
	return nil
}

// prepareImage resolves the story image. It returns EITHER a non-empty reuseKey
// (the shared MediaKey is already a servable JPEG — serve it directly, never
// delete it: it is shared across the group's owners and expires by TTL) OR
// jpegBytes to be stored under a fresh, worker-owned key. Errors here are
// permanent for the payload (expired media, undecodable image, empty text).
func (w *InstagramStoryWorker) prepareImage(ctx context.Context, data *InstagramStoryJobData) (reuseKey string, jpegBytes []byte, err error) {
	if data.MediaKey != "" {
		raw, mime, err := w.store.Get(ctx, data.MediaKey)
		if err != nil {
			return "", nil, err
		}
		if strings.Contains(strings.ToLower(mime), "jpeg") || strings.Contains(strings.ToLower(mime), "jpg") {
			return data.MediaKey, nil, nil
		}
		b, err := reencodeJPEG(raw)
		return "", b, err
	}
	if strings.TrimSpace(data.Text) == "" {
		return "", nil, errors.New("empty story text")
	}
	b, err := imagerender.RenderStoryImage(data.Text, imagerender.Options{})
	return "", b, err
}

func reencodeJPEG(raw []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode media: %w", err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

func (w *InstagramStoryWorker) finishFailed(ctx context.Context, ledgerID, msg string) {
	if ledgerID == "" {
		return
	}
	if err := w.storyRepo.MarkResult(context.WithoutCancel(ctx), ledgerID,
		models.InstagramStoryStatusFailed, "", msg); err != nil {
		w.logger.Warn("instagram story: mark failed error", zap.String("ledgerID", ledgerID), zap.Error(err))
	}
}

func sourceOf(data *InstagramStoryJobData) string {
	if data.MediaKey != "" {
		return models.InstagramMediaSourceImage
	}
	return models.InstagramMediaSourceRenderedText
}
