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

// TestDeleteInstance_UnpairsSessionBeforeDeletingRow locks the orphan leak: a
// deliberate single-chip delete must also unlink the number in WAHA. Deleting
// only the DB row leaves the session running and still logged into WhatsApp,
// with no record tying it to anyone — the app shows the number as disconnected
// while it never actually left WhatsApp.
func TestDeleteInstance_UnpairsSessionBeforeDeletingRow(t *testing.T) {
	repo := new(mockInstanceRepo)
	fake := whatsapp.NewFakeAdapter()
	h := &InstancesHandler{instanceRepo: repo, waProvider: fake}

	phone := "554898348964"
	repo.On("GetInstanceByID", mock.Anything, 42).Return(
		&models.Instance{ID: 42, UserID: "u1", InstanceID: "sess-live", Status: "connected", PhoneNumber: &phone}, nil)
	repo.On("DeleteInstance", mock.Anything, 42).Return(nil)

	ctx := contextWithUser("u1", "u1@example.com")
	_, err := h.DeleteInstance(ctx, connect.NewRequest(&instancesv1.DeleteInstanceRequest{Id: "42", Force: true}))

	require.NoError(t, err)
	require.Equal(t, 1, fake.CallCount("UnpairSession"),
		"an explicit delete must unpair the number in WAHA, not just drop the row")
	assert.Equal(t, 0, fake.CallCount("DeleteSession"),
		"must go through UnpairSession (carries X-Confirm-Unpair); a plain DeleteSession is blocked by the router for a paired chip")
	repo.AssertCalled(t, "DeleteInstance", mock.Anything, 42)
}

// TestDeleteInstance_KeepsRowWhenUnpairFails is the anti-orphan guarantee: if
// WAHA cannot be reached, we must NOT delete the row. Dropping it anyway is
// exactly what strands a live, paired chip with no way back.
func TestDeleteInstance_KeepsRowWhenUnpairFails(t *testing.T) {
	repo := new(mockInstanceRepo)
	fake := whatsapp.NewFakeAdapter()
	h := &InstancesHandler{instanceRepo: repo, waProvider: fake}

	phone := "554898348964"
	repo.On("GetInstanceByID", mock.Anything, 42).Return(
		&models.Instance{ID: 42, UserID: "u1", InstanceID: "sess-live", Status: "connected", PhoneNumber: &phone}, nil)
	fake.UnpairSessionFunc = func(_ context.Context, _ string) error {
		return whatsapp.NewProviderError(whatsapp.ErrCodeTimeout, "waha unreachable", nil)
	}

	ctx := contextWithUser("u1", "u1@example.com")
	_, err := h.DeleteInstance(ctx, connect.NewRequest(&instancesv1.DeleteInstanceRequest{Id: "42", Force: true}))

	require.Error(t, err, "a failed unpair must surface, not silently orphan the session")
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	repo.AssertNotCalled(t, "DeleteInstance", mock.Anything, 42)
}

// TestDeleteInstance_DeletesRowWhenSessionAlreadyGone: nothing to unpair is not
// an error — the row must still go, or the user can never clear a dead chip.
func TestDeleteInstance_DeletesRowWhenSessionAlreadyGone(t *testing.T) {
	repo := new(mockInstanceRepo)
	fake := whatsapp.NewFakeAdapter()
	h := &InstancesHandler{instanceRepo: repo, waProvider: fake}

	repo.On("GetInstanceByID", mock.Anything, 9).Return(
		&models.Instance{ID: 9, UserID: "u1", InstanceID: "sess-gone", Status: "disconnected"}, nil)
	repo.On("DeleteInstance", mock.Anything, 9).Return(nil)
	fake.UnpairSessionFunc = func(_ context.Context, _ string) error {
		return whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
	}

	ctx := contextWithUser("u1", "u1@example.com")
	_, err := h.DeleteInstance(ctx, connect.NewRequest(&instancesv1.DeleteInstanceRequest{Id: "9"}))

	require.NoError(t, err)
	repo.AssertCalled(t, "DeleteInstance", mock.Anything, 9)
}


// TestDeleteInstance_RefusesConnectedWithoutForce trava a proteção anti-
// destruição: número CONECTADO e operando não pode ser removido sem
// force=true. Foi exatamente uma remoção em massa de chips saudáveis que
// destruiu a frota de 15 chips recém-escaneada (12/07, 8 gerações em uma
// noite). Sem force: FailedPrecondition, sem unpair, linha intacta.
func TestDeleteInstance_RefusesConnectedWithoutForce(t *testing.T) {
	repo := new(mockInstanceRepo)
	fake := whatsapp.NewFakeAdapter()
	h := &InstancesHandler{instanceRepo: repo, waProvider: fake}

	phone := "554898231627"
	repo.On("GetInstanceByID", mock.Anything, 42).Return(
		&models.Instance{ID: 42, UserID: "u1", InstanceID: "sess-live", Status: "connected", PhoneNumber: &phone}, nil)

	ctx := contextWithUser("u1", "u1@example.com")
	_, err := h.DeleteInstance(ctx, connect.NewRequest(&instancesv1.DeleteInstanceRequest{Id: "42"}))

	require.Error(t, err, "connected chip without force must be refused")
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, 0, fake.CallCount("UnpairSession"), "must not unpair a protected chip")
	repo.AssertNotCalled(t, "DeleteInstance", mock.Anything, 42)
}
