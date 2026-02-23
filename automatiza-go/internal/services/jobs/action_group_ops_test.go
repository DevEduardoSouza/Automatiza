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
// ExecuteUpdateDescription Tests
// =============================================================================

func TestExecuteUpdateDescription_Success(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionUpdateDescription,
		Payload: models.JobPayload{
			Groups:      []string{"group1@g.us"},
			Description: "Nova descricao do grupo",
		},
	}

	results := ExecuteUpdateDescription(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("UpdateGroupDescription"))

	call := provider.LastCall("UpdateGroupDescription")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.UpdateGroupDescriptionInput)
	assert.Equal(t, "group1@g.us", input.GroupID)
	assert.Equal(t, "Nova descricao do grupo", input.Description)
}

func TestExecuteUpdateDescription_MultipleGroups(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionUpdateDescription,
		Payload: models.JobPayload{
			Groups:      []string{"group1@g.us", "group2@g.us", "group3@g.us"},
			Description: "Descricao padrao",
		},
	}

	results := ExecuteUpdateDescription(ctx, provider, "session1", job, logger)

	require.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Success)
	}
	assert.Equal(t, 3, provider.CallCount("UpdateGroupDescription"))
}

func TestExecuteUpdateDescription_Error(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.UpdateGroupDescriptionFunc = func(_ context.Context, _ whatsapp.UpdateGroupDescriptionInput) error {
		return errors.New("not admin")
	}
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionUpdateDescription,
		Payload: models.JobPayload{
			Groups:      []string{"group1@g.us"},
			Description: "Desc",
		},
	}

	results := ExecuteUpdateDescription(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, "not admin", results[0].Error)
}

// =============================================================================
// ExecuteCloseGroups Tests
// =============================================================================

func TestExecuteCloseGroups_Success(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionCloseGroups,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us", "group2@g.us"},
		},
	}

	results := ExecuteCloseGroups(ctx, provider, "session1", job, logger)

	require.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success)
	}
	assert.Equal(t, 2, provider.CallCount("SetGroupAnnouncement"))

	// Verify Announce=true was set
	calls := provider.GetCalls("SetGroupAnnouncement")
	for _, call := range calls {
		input := call.Args[0].(whatsapp.SetGroupAnnouncementInput)
		assert.True(t, input.Announce)
	}
}

func TestExecuteCloseGroups_Error(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SetGroupAnnouncementFunc = func(_ context.Context, _ whatsapp.SetGroupAnnouncementInput) error {
		return errors.New("permission denied")
	}
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionCloseGroups,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us"},
		},
	}

	results := ExecuteCloseGroups(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, "permission denied", results[0].Error)
}

// =============================================================================
// ExecuteOpenGroups Tests
// =============================================================================

func TestExecuteOpenGroups_Success(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionOpenGroups,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us", "group2@g.us"},
		},
	}

	results := ExecuteOpenGroups(ctx, provider, "session1", job, logger)

	require.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success)
	}
	assert.Equal(t, 2, provider.CallCount("SetGroupAnnouncement"))

	// Verify Announce=false was set (open)
	calls := provider.GetCalls("SetGroupAnnouncement")
	for _, call := range calls {
		input := call.Args[0].(whatsapp.SetGroupAnnouncementInput)
		assert.False(t, input.Announce)
	}
}

func TestExecuteOpenGroups_Error(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SetGroupAnnouncementFunc = func(_ context.Context, _ whatsapp.SetGroupAnnouncementInput) error {
		return errors.New("failed")
	}
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionOpenGroups,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us"},
		},
	}

	results := ExecuteOpenGroups(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, "failed", results[0].Error)
}

// =============================================================================
// ExecuteChangeGroupName Tests
// =============================================================================

func TestExecuteChangeGroupName_Success_NoAutoNumber(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupName,
		Payload: models.JobPayload{
			Groups:     []string{"group1@g.us", "group2@g.us"},
			Name:       "Novo Nome",
			AutoNumber: false,
		},
	}

	results := ExecuteChangeGroupName(ctx, provider, "session1", job, logger)

	require.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success)
	}
	assert.Equal(t, 2, provider.CallCount("UpdateGroupSubject"))

	// Both should have the same name (no numbering)
	calls := provider.GetCalls("UpdateGroupSubject")
	for _, call := range calls {
		input := call.Args[0].(whatsapp.UpdateGroupSubjectInput)
		assert.Equal(t, "Novo Nome", input.Subject)
	}
}

func TestExecuteChangeGroupName_Success_WithAutoNumber(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupName,
		Payload: models.JobPayload{
			Groups:     []string{"group1@g.us", "group2@g.us", "group3@g.us"},
			Name:       "Grupo",
			AutoNumber: true,
		},
	}

	results := ExecuteChangeGroupName(ctx, provider, "session1", job, logger)

	require.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Success)
	}

	calls := provider.GetCalls("UpdateGroupSubject")
	require.Len(t, calls, 3)
	assert.Equal(t, "#1 Grupo", calls[0].Args[0].(whatsapp.UpdateGroupSubjectInput).Subject)
	assert.Equal(t, "#2 Grupo", calls[1].Args[0].(whatsapp.UpdateGroupSubjectInput).Subject)
	assert.Equal(t, "#3 Grupo", calls[2].Args[0].(whatsapp.UpdateGroupSubjectInput).Subject)
}

func TestExecuteChangeGroupName_PartialError(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	callCount := 0
	provider.UpdateGroupSubjectFunc = func(_ context.Context, input whatsapp.UpdateGroupSubjectInput) error {
		callCount++
		if callCount == 2 {
			return errors.New("not admin in group2")
		}
		return nil
	}
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupName,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us", "group2@g.us", "group3@g.us"},
			Name:   "Test",
		},
	}

	results := ExecuteChangeGroupName(ctx, provider, "session1", job, logger)

	require.Len(t, results, 3)
	assert.True(t, results[0].Success)
	assert.False(t, results[1].Success)
	assert.Equal(t, "not admin in group2", results[1].Error)
	assert.True(t, results[2].Success)
}

func TestExecuteChangeGroupName_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	provider := whatsapp.NewFakeAdapter()
	provider.UpdateGroupSubjectFunc = func(_ context.Context, _ whatsapp.UpdateGroupSubjectInput) error {
		cancel() // Cancel after first execution
		return nil
	}
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupName,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us", "group2@g.us", "group3@g.us", "group4@g.us"},
			Name:   "Test",
		},
	}

	results := ExecuteChangeGroupName(ctx, provider, "session1", job, logger)

	require.Len(t, results, 4)
	// First should succeed, rest should be canceled
	assert.True(t, results[0].Success)
	hasCancelled := false
	for _, r := range results[1:] {
		if !r.Success && r.Error == "canceled" {
			hasCancelled = true
		}
	}
	assert.True(t, hasCancelled)
}

func TestExecuteChangeGroupName_SingleGroup_AutoNumber(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupName,
		Payload: models.JobPayload{
			Groups:     []string{"group1@g.us"},
			Name:       "Solo",
			AutoNumber: true,
		},
	}

	results := ExecuteChangeGroupName(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)

	call := provider.LastCall("UpdateGroupSubject")
	require.NotNil(t, call)
	assert.Equal(t, "#1 Solo", call.Args[0].(whatsapp.UpdateGroupSubjectInput).Subject)
}

// =============================================================================
// ExecuteChangeGroupPhoto Tests
// =============================================================================

func TestExecuteChangeGroupPhoto_Success_Base64(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupPhoto,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us"},
			Image:  "base64ImageData",
		},
	}

	results := ExecuteChangeGroupPhoto(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("UpdateGroupPicture"))

	call := provider.LastCall("UpdateGroupPicture")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.UpdateGroupPictureInput)
	assert.Equal(t, "base64ImageData", input.ImageBase64)
}

func TestExecuteChangeGroupPhoto_EmptyImage(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupPhoto,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us", "group2@g.us"},
			Image:  "",
		},
	}

	results := ExecuteChangeGroupPhoto(ctx, provider, "session1", job, logger)

	require.Len(t, results, 2)
	for _, r := range results {
		assert.False(t, r.Success)
		assert.Contains(t, r.Error, "imagem")
	}
	assert.Equal(t, 0, provider.CallCount("UpdateGroupPicture"))
}

func TestExecuteChangeGroupPhoto_MultipleGroups(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupPhoto,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us", "group2@g.us"},
			Image:  "imageData",
		},
	}

	results := ExecuteChangeGroupPhoto(ctx, provider, "session1", job, logger)

	require.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success)
	}
	assert.Equal(t, 2, provider.CallCount("UpdateGroupPicture"))
}

func TestExecuteChangeGroupPhoto_Error(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.UpdateGroupPictureFunc = func(_ context.Context, _ whatsapp.UpdateGroupPictureInput) error {
		return errors.New("invalid image format")
	}
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupPhoto,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us"},
			Image:  "badData",
		},
	}

	results := ExecuteChangeGroupPhoto(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, "invalid image format", results[0].Error)
}

func TestExecuteChangeGroupPhoto_WithURL(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionChangeGroupPhoto,
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us"},
			Image:  "https://example.com/photo.jpg",
		},
	}

	results := ExecuteChangeGroupPhoto(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	// Should still use ImageBase64 field
	call := provider.LastCall("UpdateGroupPicture")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.UpdateGroupPictureInput)
	assert.Equal(t, "https://example.com/photo.jpg", input.ImageBase64)
}
