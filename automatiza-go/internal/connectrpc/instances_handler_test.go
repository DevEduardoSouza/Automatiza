package connectrpc

import (
	"context"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	instancesv1 "automatiza-go/internal/gen/instances/v1"
	"automatiza-go/internal/models"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestDeleteDisconnectedInstances_ProtectsAliveAndPairedChips locks the fix for
// the "sent once then stopped" incident: the bulk "delete disconnected" action
// must NOT delete a chip that ever paired (phone_number set) or whose WAHA
// session is still alive — deleting those orphans a live, paired number (its
// WAHA session survives but automations lose the DB record). Only genuine
// never-paired, WAHA-dead zombies are deleted.
func TestDeleteDisconnectedInstances_ProtectsAliveAndPairedChips(t *testing.T) {
	repo := new(mockInstanceRepo)
	fake := whatsapp.NewFakeAdapter()
	h := &InstancesHandler{instanceRepo: repo, waProvider: fake}

	phone := "5548998348964"
	instances := []models.Instance{
		// Paired but momentarily disconnected in DB → protected via phone_number.
		{ID: 1, InstanceID: "sess-paired", Status: "disconnected", PhoneNumber: &phone},
		// No phone in DB, but the WAHA session is actually WORKING → protected via live check.
		{ID: 2, InstanceID: "sess-alive", Status: "disconnected", PhoneNumber: nil},
		// Never-paired QR-scan zombie, gone from WAHA → deletable.
		{ID: 3, InstanceID: "sess-zombie", Status: "pending", PhoneNumber: nil},
		// Connected: out of scope for this action, must be left untouched.
		{ID: 4, InstanceID: "sess-connected", Status: "connected", PhoneNumber: nil},
	}

	fake.GetSessionStatusFunc = func(_ context.Context, sessionName string) (*whatsapp.SessionInfo, error) {
		switch sessionName {
		case "sess-alive":
			return &whatsapp.SessionInfo{Name: sessionName, Status: whatsapp.SessionStatusConnected}, nil
		case "sess-zombie":
			return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
		default:
			t.Fatalf("unexpected GetSessionStatus call for %q (should be short-circuited)", sessionName)
			return nil, nil
		}
	}

	repo.On("GetUserInstances", mock.Anything, "u1").Return(instances, nil)
	repo.On("DeleteInstance", mock.Anything, 3).Return(nil)

	ctx := contextWithUser("u1", "u1@example.com")
	resp, err := h.DeleteDisconnectedInstances(ctx, connect.NewRequest(&instancesv1.DeleteDisconnectedInstancesRequest{}))

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(1), resp.Msg.GetDeletedCount(), "only the dead never-paired zombie should be deleted")

	// The zombie (3) is the ONLY delete.
	repo.AssertCalled(t, "DeleteInstance", mock.Anything, 3)
	// Paired (1), alive (2) and connected (4) must never be deleted.
	repo.AssertNotCalled(t, "DeleteInstance", mock.Anything, 1)
	repo.AssertNotCalled(t, "DeleteInstance", mock.Anything, 2)
	repo.AssertNotCalled(t, "DeleteInstance", mock.Anything, 4)
}

// TestDeleteDisconnectedInstances_FailsSafeOnWahaError verifies that a transient
// WAHA error (not a definitive SESSION_NOT_FOUND) protects an unpaired instance
// from deletion — we never delete a record we could not verify is dead.
func TestDeleteDisconnectedInstances_FailsSafeOnWahaError(t *testing.T) {
	repo := new(mockInstanceRepo)
	fake := whatsapp.NewFakeAdapter()
	h := &InstancesHandler{instanceRepo: repo, waProvider: fake}

	instances := []models.Instance{
		{ID: 7, InstanceID: "sess-unknown", Status: "disconnected", PhoneNumber: nil},
	}
	fake.GetSessionStatusFunc = func(_ context.Context, _ string) (*whatsapp.SessionInfo, error) {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeCircuitBreakerOpen, "service temporarily unavailable", nil)
	}

	repo.On("GetUserInstances", mock.Anything, "u1").Return(instances, nil)

	ctx := contextWithUser("u1", "u1@example.com")
	resp, err := h.DeleteDisconnectedInstances(ctx, connect.NewRequest(&instancesv1.DeleteDisconnectedInstancesRequest{}))

	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.Msg.GetDeletedCount(), "must not delete when WAHA status is unverifiable")
	repo.AssertNotCalled(t, "DeleteInstance", mock.Anything, 7)
}
