package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newMiniRedis starts a miniredis server and returns it with a go-redis client.
// The caller should defer mr.Close().
func newMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return mr, client
}

// mockCacheClearer implements SessionCacheClearer for tests.
type mockCacheClearer struct {
	calls []string
}

func (m *mockCacheClearer) ClearStatusCache(sessionName string) {
	m.calls = append(m.calls, sessionName)
}

// =============================================================================
// Tests: NewSSERedisBridge
// =============================================================================

func TestNewSSERedisBridge_SetsDefaults(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())

	if bridge.redisClient != client {
		t.Error("expected redisClient to be set")
	}
	if bridge.hub != nil {
		t.Error("expected hub to be nil on new bridge")
	}
	if bridge.cacheClear != nil {
		t.Error("expected cacheClear to be nil on new bridge")
	}
	if bridge.channel != sseRedisChannel {
		t.Errorf("expected channel=%q, got %q", sseRedisChannel, bridge.channel)
	}
	if bridge.logger == nil {
		t.Error("expected logger to be set")
	}
}

// =============================================================================
// Tests: SetHub
// =============================================================================

func TestSetHub_AssignsHub(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	bridge.SetHub(hub)

	if bridge.hub != hub {
		t.Error("expected hub to be set after SetHub")
	}
}

// =============================================================================
// Tests: SetCacheClearer
// =============================================================================

func TestSetCacheClearer_AssignsCacheClearer(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	cc := &mockCacheClearer{}

	bridge.SetCacheClearer(cc)

	if bridge.cacheClear != cc {
		t.Error("expected cacheClear to be set after SetCacheClearer")
	}
}

// =============================================================================
// Tests: Publish
// =============================================================================

func TestPublish_SuccessfulPublish(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	defer client.Close()
	_ = mr

	bridge := NewSSERedisBridge(client, testLogger())

	event := SSEEvent{
		Type:    SSEEventSessionStatus,
		Session: "session-1",
		UserID:  "user-1",
		Data:    map[string]any{"status": "CONNECTED"},
	}

	// Subscribe to verify message arrives
	sub := client.Subscribe(context.Background(), sseRedisChannel)
	defer sub.Close()
	// Wait for subscription to be active
	_, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	if err := bridge.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	// Read the published message
	msg, err := sub.ReceiveMessage(context.Background())
	if err != nil {
		t.Fatalf("failed to receive message: %v", err)
	}

	var received SSEEvent
	if err := json.Unmarshal([]byte(msg.Payload), &received); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if received.Type != SSEEventSessionStatus {
		t.Errorf("expected type=%s, got %s", SSEEventSessionStatus, received.Type)
	}
	if received.Session != "session-1" {
		t.Errorf("expected session=session-1, got %s", received.Session)
	}
	if received.UserID != "user-1" {
		t.Errorf("expected userID=user-1, got %s", received.UserID)
	}
}

func TestPublish_RedisError(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	bridge := NewSSERedisBridge(client, testLogger())

	// Close miniredis to trigger Redis error
	mr.Close()
	client.Close()

	event := SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-1",
		Data:   map[string]any{"status": "CONNECTED"},
	}

	err := bridge.Publish(context.Background(), event)
	if err == nil {
		t.Fatal("expected error when Redis is down, got nil")
	}
}

func TestPublish_ContextCancelled(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	event := SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-1",
		Data:   map[string]any{"status": "CONNECTED"},
	}

	err := bridge.Publish(ctx, event)
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
}

// =============================================================================
// Tests: Subscribe
// =============================================================================

func TestSubscribe_ErrorWhenHubNil(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	// hub is nil by default

	err := bridge.Subscribe(context.Background())
	if err == nil {
		t.Fatal("expected error when hub is nil, got nil")
	}
	if err.Error() != "SSEHub not set; call SetHub before Subscribe" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestSubscribe_SuccessfulSubscription(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := bridge.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	// Give goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop the subscription loop
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestSubscribe_ForwardsEventsToHub(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	// Register a client on the hub to receive events
	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := bridge.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	// Give subscription goroutine time to start
	time.Sleep(100 * time.Millisecond)

	// Publish an event via Redis (simulating worker process)
	event := SSEEvent{
		Type:    SSEEventSessionStatus,
		Session: "session-1",
		UserID:  "user-1",
		Data:    map[string]any{"status": "CONNECTED"},
	}
	data, _ := json.Marshal(event)
	client.Publish(ctx, sseRedisChannel, data)

	// Verify event arrives at the SSE client
	evt := readEvent(t, sseClient.channel, 2*time.Second)
	if evt.Type != SSEEventSessionStatus {
		t.Errorf("expected type=%s, got %s", SSEEventSessionStatus, evt.Type)
	}
	if evt.UserID != "user-1" {
		t.Errorf("expected userID=user-1, got %s", evt.UserID)
	}
	if evt.Data["status"] != "CONNECTED" {
		t.Errorf("expected data.status=CONNECTED, got %v", evt.Data["status"])
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestSubscribe_RedisSubscribeError(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	// Close Redis to cause subscribe error
	mr.Close()
	client.Close()

	err := bridge.Subscribe(context.Background())
	if err == nil {
		t.Fatal("expected error when Redis is unavailable, got nil")
	}
}

// =============================================================================
// Tests: handleMessage
// =============================================================================

func TestHandleMessage_ValidJSON_ForwardsToHub(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	event := SSEEvent{
		Type:    SSEEventJobStatus,
		UserID:  "user-1",
		Data:    map[string]any{"jobId": "job-1", "status": "running"},
	}
	data, _ := json.Marshal(event)

	msg := &redis.Message{
		Channel: sseRedisChannel,
		Payload: string(data),
	}

	bridge.handleMessage(msg)
	time.Sleep(100 * time.Millisecond)

	evt := readEvent(t, sseClient.channel, time.Second)
	if evt.Type != SSEEventJobStatus {
		t.Errorf("expected type=%s, got %s", SSEEventJobStatus, evt.Type)
	}
	if evt.UserID != "user-1" {
		t.Errorf("expected userID=user-1, got %s", evt.UserID)
	}
}

func TestHandleMessage_InvalidJSON_DoesNotForward(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	msg := &redis.Message{
		Channel: sseRedisChannel,
		Payload: "this is not valid json{{{",
	}

	bridge.handleMessage(msg)
	time.Sleep(50 * time.Millisecond)

	// No event should arrive
	assertNoEvent(t, sseClient.channel, 200*time.Millisecond)
}

func TestHandleMessage_SessionStatusEvent_ClearsCache(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	cc := &mockCacheClearer{}
	bridge.SetCacheClearer(cc)

	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	event := SSEEvent{
		Type:    SSEEventSessionStatus,
		Session: "session-abc",
		UserID:  "user-1",
		Data:    map[string]any{"status": "CONNECTED"},
	}
	data, _ := json.Marshal(event)

	msg := &redis.Message{
		Channel: sseRedisChannel,
		Payload: string(data),
	}

	bridge.handleMessage(msg)
	time.Sleep(50 * time.Millisecond)

	if len(cc.calls) != 1 {
		t.Fatalf("expected 1 ClearStatusCache call, got %d", len(cc.calls))
	}
	if cc.calls[0] != "session-abc" {
		t.Errorf("expected ClearStatusCache called with session-abc, got %s", cc.calls[0])
	}
}

func TestHandleMessage_SessionStatusEvent_NoCacheClearer(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)
	// cacheClear is nil — should not panic

	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	event := SSEEvent{
		Type:    SSEEventSessionStatus,
		Session: "session-abc",
		UserID:  "user-1",
		Data:    map[string]any{"status": "CONNECTED"},
	}
	data, _ := json.Marshal(event)

	msg := &redis.Message{
		Channel: sseRedisChannel,
		Payload: string(data),
	}

	// Should not panic even without cache clearer
	bridge.handleMessage(msg)
	time.Sleep(50 * time.Millisecond)

	// Event should still be forwarded
	evt := readEvent(t, sseClient.channel, time.Second)
	if evt.Type != SSEEventSessionStatus {
		t.Errorf("expected type=%s, got %s", SSEEventSessionStatus, evt.Type)
	}
}

func TestHandleMessage_SessionStatusEvent_EmptySession_NoCacheClear(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	cc := &mockCacheClearer{}
	bridge.SetCacheClearer(cc)

	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	// Session is empty — should not clear cache
	event := SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-1",
		Data:   map[string]any{"status": "CONNECTED"},
	}
	data, _ := json.Marshal(event)

	msg := &redis.Message{
		Channel: sseRedisChannel,
		Payload: string(data),
	}

	bridge.handleMessage(msg)
	time.Sleep(50 * time.Millisecond)

	if len(cc.calls) != 0 {
		t.Errorf("expected 0 ClearStatusCache calls for empty session, got %d", len(cc.calls))
	}
}

func TestHandleMessage_NonStatusEvent_NoCacheClear(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	cc := &mockCacheClearer{}
	bridge.SetCacheClearer(cc)

	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	// Non-status event should NOT clear cache
	event := SSEEvent{
		Type:    SSEEventJobStatus,
		Session: "session-abc",
		UserID:  "user-1",
		Data:    map[string]any{"jobId": "job-1"},
	}
	data, _ := json.Marshal(event)

	msg := &redis.Message{
		Channel: sseRedisChannel,
		Payload: string(data),
	}

	bridge.handleMessage(msg)
	time.Sleep(50 * time.Millisecond)

	if len(cc.calls) != 0 {
		t.Errorf("expected 0 ClearStatusCache calls for non-status event, got %d", len(cc.calls))
	}
}

// =============================================================================
// Tests: drainPubSub
// =============================================================================

func TestDrainPubSub_StopsOnContextCancel(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	ctx, cancel := context.WithCancel(context.Background())
	pubsub := client.Subscribe(ctx, sseRedisChannel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		bridge.drainPubSub(ctx, pubsub)
		close(done)
	}()

	// Give drainPubSub time to start
	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case <-done:
		// drainPubSub exited as expected
	case <-time.After(2 * time.Second):
		t.Fatal("drainPubSub did not exit after context cancellation")
	}

	pubsub.Close()
}

func TestDrainPubSub_ProcessesMessages(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pubsub := client.Subscribe(ctx, sseRedisChannel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	go bridge.drainPubSub(ctx, pubsub)

	// Give drainPubSub time to start reading
	time.Sleep(100 * time.Millisecond)

	// Publish a message
	event := SSEEvent{
		Type:   SSEEventGroupSync,
		UserID: "user-1",
		Data:   map[string]any{"synced": true},
	}
	data, _ := json.Marshal(event)
	client.Publish(ctx, sseRedisChannel, data)

	evt := readEvent(t, sseClient.channel, 2*time.Second)
	if evt.Type != SSEEventGroupSync {
		t.Errorf("expected type=%s, got %s", SSEEventGroupSync, evt.Type)
	}

	cancel()
	pubsub.Close()
	time.Sleep(100 * time.Millisecond)
}

// =============================================================================
// Tests: subscribeLoop — reconnect behavior
// =============================================================================

func TestSubscribeLoop_ExitsOnContextCancel(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	ctx, cancel := context.WithCancel(context.Background())

	pubsub := client.Subscribe(ctx, sseRedisChannel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		bridge.subscribeLoop(ctx, pubsub)
		close(done)
	}()

	// Cancel context to trigger shutdown
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// subscribeLoop exited as expected
	case <-time.After(5 * time.Second):
		t.Fatal("subscribeLoop did not exit after context cancellation")
	}
}

// =============================================================================
// Tests: NewRedisSSEStatusNotifier
// =============================================================================

func TestNewRedisSSEStatusNotifier_SetsFields(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	resolver := func(s string) (string, error) { return "user-1", nil }

	notifier := NewRedisSSEStatusNotifier(bridge, testLogger(), resolver)

	if notifier.bridge != bridge {
		t.Error("expected bridge to be set")
	}
	if notifier.logger == nil {
		t.Error("expected logger to be set")
	}
	if notifier.userResolver == nil {
		t.Error("expected userResolver to be set")
	}
}

// =============================================================================
// Tests: RedisSSEStatusNotifier.ProcessStatus
// =============================================================================

func TestRedisStatusNotifier_ProcessStatus_Success(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())

	resolver := func(sessionName string) (string, error) {
		if sessionName == "my-session" {
			return "resolved-user", nil
		}
		return "", errors.New("not found")
	}

	notifier := NewRedisSSEStatusNotifier(bridge, testLogger(), resolver)

	// Subscribe to verify the event is published
	sub := client.Subscribe(context.Background(), sseRedisChannel)
	defer sub.Close()
	_, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	err = notifier.ProcessStatus(context.Background(), "my-session", "CONNECTED")
	if err != nil {
		t.Fatalf("ProcessStatus returned error: %v", err)
	}

	// Read the published message
	msg, err := sub.ReceiveMessage(context.Background())
	if err != nil {
		t.Fatalf("failed to receive message: %v", err)
	}

	var received SSEEvent
	if err := json.Unmarshal([]byte(msg.Payload), &received); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if received.Type != SSEEventSessionStatus {
		t.Errorf("expected type=%s, got %s", SSEEventSessionStatus, received.Type)
	}
	if received.UserID != "resolved-user" {
		t.Errorf("expected userID=resolved-user, got %s", received.UserID)
	}
	if received.Session != "my-session" {
		t.Errorf("expected session=my-session, got %s", received.Session)
	}
	if received.Data["status"] != "CONNECTED" {
		t.Errorf("expected data.status=CONNECTED, got %v", received.Data["status"])
	}
	if received.Data["session"] != "my-session" {
		t.Errorf("expected data.session=my-session, got %v", received.Data["session"])
	}
}

func TestRedisStatusNotifier_ProcessStatus_ResolverError_ReturnsNil(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())

	resolver := func(sessionName string) (string, error) {
		return "", errors.New("session not found")
	}

	notifier := NewRedisSSEStatusNotifier(bridge, testLogger(), resolver)

	// Subscribe to verify no event is published
	sub := client.Subscribe(context.Background(), sseRedisChannel)
	defer sub.Close()
	_, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	err = notifier.ProcessStatus(context.Background(), "deleted-session", "DISCONNECTED")
	if err != nil {
		t.Fatalf("expected nil error on resolver failure, got: %v", err)
	}

	// No message should be published
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, receiveErr := sub.ReceiveMessage(ctx)
	if receiveErr == nil {
		t.Error("expected no message to be published when resolver fails")
	}
}

func TestRedisStatusNotifier_ProcessStatus_RedisPublishError(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)

	bridge := NewSSERedisBridge(client, testLogger())

	resolver := func(sessionName string) (string, error) {
		return "user-1", nil
	}

	notifier := NewRedisSSEStatusNotifier(bridge, testLogger(), resolver)

	// Close Redis to cause publish error
	mr.Close()
	client.Close()

	err := notifier.ProcessStatus(context.Background(), "session-1", "CONNECTED")
	if err == nil {
		t.Fatal("expected error when Redis is down, got nil")
	}
}

// =============================================================================
// Tests: NewRedisJobProgressPublisher
// =============================================================================

func TestNewRedisJobProgressPublisher_SetsFields(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())

	publisher := NewRedisJobProgressPublisher(bridge, testLogger())

	if publisher.bridge != bridge {
		t.Error("expected bridge to be set")
	}
	if publisher.logger == nil {
		t.Error("expected logger to be set")
	}
}

// =============================================================================
// Tests: RedisJobProgressPublisher.PublishJobProgress
// =============================================================================

func TestRedisJobProgressPublisher_PublishJobProgress_Success(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	publisher := NewRedisJobProgressPublisher(bridge, testLogger())

	// Subscribe to verify the event is published
	sub := client.Subscribe(context.Background(), sseRedisChannel)
	defer sub.Close()
	_, err := sub.Receive(context.Background())
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	progressData := map[string]any{
		"jobId":    "job-42",
		"progress": 50,
		"total":    100,
		"status":   "running",
	}

	publisher.PublishJobProgress("user-1", progressData)

	// Read the published message
	msg, msgErr := sub.ReceiveMessage(context.Background())
	if msgErr != nil {
		t.Fatalf("failed to receive message: %v", msgErr)
	}

	var received SSEEvent
	if err := json.Unmarshal([]byte(msg.Payload), &received); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if received.Type != SSEEventJobStatus {
		t.Errorf("expected type=%s, got %s", SSEEventJobStatus, received.Type)
	}
	if received.UserID != "user-1" {
		t.Errorf("expected userID=user-1, got %s", received.UserID)
	}
}

func TestRedisJobProgressPublisher_PersistsToRedisKey(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	publisher := NewRedisJobProgressPublisher(bridge, testLogger())

	progressData := map[string]any{
		"jobId":    "job-42",
		"progress": 75,
		"total":    100,
		"status":   "running",
	}

	publisher.PublishJobProgress("user-1", progressData)

	// Give time for the async Redis SET to complete
	time.Sleep(100 * time.Millisecond)

	// Verify the key was set in Redis
	key := "sync_progress:job-42"
	val, err := mr.Get(key)
	if err != nil {
		t.Fatalf("expected key %s to exist in Redis, got error: %v", key, err)
	}

	var stored map[string]any
	if err := json.Unmarshal([]byte(val), &stored); err != nil {
		t.Fatalf("failed to unmarshal stored value: %v", err)
	}

	if stored["jobId"] != "job-42" {
		t.Errorf("expected stored jobId=job-42, got %v", stored["jobId"])
	}
	// JSON numbers unmarshal as float64
	if stored["progress"] != float64(75) {
		t.Errorf("expected stored progress=75, got %v", stored["progress"])
	}

	// Verify TTL is set (miniredis uses integer seconds)
	ttl := mr.TTL(key)
	if ttl <= 0 {
		t.Errorf("expected positive TTL on key %s, got %v", key, ttl)
	}
	if ttl > 5*time.Minute {
		t.Errorf("expected TTL <= 5 minutes, got %v", ttl)
	}
}

func TestRedisJobProgressPublisher_NoJobID_NoRedisSet(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	publisher := NewRedisJobProgressPublisher(bridge, testLogger())

	// No jobId in data
	progressData := map[string]any{
		"progress": 50,
		"total":    100,
		"status":   "running",
	}

	publisher.PublishJobProgress("user-1", progressData)

	// Give time for potential SET
	time.Sleep(100 * time.Millisecond)

	// No sync_progress key should exist
	keys := mr.Keys()
	for _, k := range keys {
		if k == "sync_progress:" || len(k) > 14 && k[:14] == "sync_progress:" {
			t.Errorf("expected no sync_progress keys, found %s", k)
		}
	}
}

func TestRedisJobProgressPublisher_NonStringJobID_NoRedisSet(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	publisher := NewRedisJobProgressPublisher(bridge, testLogger())

	// jobId is an integer, not a string — type assertion should fail
	progressData := map[string]any{
		"jobId":    12345,
		"progress": 50,
		"status":   "running",
	}

	publisher.PublishJobProgress("user-1", progressData)

	time.Sleep(100 * time.Millisecond)

	// No sync_progress key should exist
	keys := mr.Keys()
	for _, k := range keys {
		if len(k) >= 14 && k[:14] == "sync_progress:" {
			t.Errorf("expected no sync_progress keys for non-string jobId, found %s", k)
		}
	}
}

func TestRedisJobProgressPublisher_EmptyJobID_NoRedisSet(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	publisher := NewRedisJobProgressPublisher(bridge, testLogger())

	// jobId is empty string — should skip SET
	progressData := map[string]any{
		"jobId":    "",
		"progress": 50,
		"status":   "running",
	}

	publisher.PublishJobProgress("user-1", progressData)

	time.Sleep(100 * time.Millisecond)

	keys := mr.Keys()
	for _, k := range keys {
		if len(k) >= 14 && k[:14] == "sync_progress:" {
			t.Errorf("expected no sync_progress keys for empty jobId, found %s", k)
		}
	}
}

func TestRedisJobProgressPublisher_RedisDown_NoErrorPanics(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	bridge := NewSSERedisBridge(client, testLogger())
	publisher := NewRedisJobProgressPublisher(bridge, testLogger())

	// Close Redis to force errors
	mr.Close()
	client.Close()

	progressData := map[string]any{
		"jobId":    "job-42",
		"progress": 50,
		"status":   "running",
	}

	// Should not panic — errors are logged and captured
	publisher.PublishJobProgress("user-1", progressData)
}

// =============================================================================
// Tests: NewRedisSyncProgressReader
// =============================================================================

func TestNewRedisSyncProgressReader_SetsClient(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	reader := NewRedisSyncProgressReader(client)

	if reader.redisClient != client {
		t.Error("expected redisClient to be set")
	}
}

// =============================================================================
// Tests: RedisSyncProgressReader.GetSyncProgress
// =============================================================================

func TestGetSyncProgress_Success(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	defer client.Close()

	reader := NewRedisSyncProgressReader(client)

	// Set up test data in Redis
	progressData := map[string]any{
		"jobId":    "job-42",
		"progress": float64(75),
		"total":    float64(100),
		"status":   "running",
	}
	data, _ := json.Marshal(progressData)
	mr.Set("sync_progress:job-42", string(data))

	result, err := reader.GetSyncProgress(context.Background(), "job-42")
	if err != nil {
		t.Fatalf("GetSyncProgress returned error: %v", err)
	}

	if result["jobId"] != "job-42" {
		t.Errorf("expected jobId=job-42, got %v", result["jobId"])
	}
	if result["progress"] != float64(75) {
		t.Errorf("expected progress=75, got %v", result["progress"])
	}
	if result["total"] != float64(100) {
		t.Errorf("expected total=100, got %v", result["total"])
	}
	if result["status"] != "running" {
		t.Errorf("expected status=running, got %v", result["status"])
	}
}

func TestGetSyncProgress_KeyNotFound(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	reader := NewRedisSyncProgressReader(client)

	_, err := reader.GetSyncProgress(context.Background(), "nonexistent-job")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
	if !errors.Is(err, redis.Nil) {
		t.Errorf("expected redis.Nil error, got: %v", err)
	}
}

func TestGetSyncProgress_InvalidJSON(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	defer client.Close()

	reader := NewRedisSyncProgressReader(client)

	// Set invalid JSON in Redis
	mr.Set("sync_progress:bad-job", "not-valid-json{{{")

	_, err := reader.GetSyncProgress(context.Background(), "bad-job")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestGetSyncProgress_RedisError(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	reader := NewRedisSyncProgressReader(client)

	// Close Redis to cause error
	mr.Close()
	client.Close()

	_, err := reader.GetSyncProgress(context.Background(), "job-42")
	if err == nil {
		t.Fatal("expected error when Redis is down, got nil")
	}
}

// =============================================================================
// Tests: End-to-end: Publisher → Redis → Subscribe → Hub → SSE Client
// =============================================================================

func TestEndToEnd_WorkerPublish_ServerReceives(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	// --- Server side ---
	serverBridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	serverBridge.SetHub(hub)

	cc := &mockCacheClearer{}
	serverBridge.SetCacheClearer(cc)

	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := serverBridge.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	// Give subscription goroutine time to start
	time.Sleep(150 * time.Millisecond)

	// --- Worker side ---
	workerBridge := NewSSERedisBridge(client, testLogger())

	event := SSEEvent{
		Type:    SSEEventSessionStatus,
		Session: "session-1",
		UserID:  "user-1",
		Data:    map[string]any{"status": "CONNECTED"},
	}

	if err := workerBridge.Publish(ctx, event); err != nil {
		t.Fatalf("worker Publish returned error: %v", err)
	}

	// Verify the SSE client receives the event
	evt := readEvent(t, sseClient.channel, 3*time.Second)
	if evt.Type != SSEEventSessionStatus {
		t.Errorf("expected type=%s, got %s", SSEEventSessionStatus, evt.Type)
	}
	if evt.UserID != "user-1" {
		t.Errorf("expected userID=user-1, got %s", evt.UserID)
	}
	if evt.Data["status"] != "CONNECTED" {
		t.Errorf("expected data.status=CONNECTED, got %v", evt.Data["status"])
	}

	// Verify cache was cleared for session.status event
	if len(cc.calls) != 1 {
		t.Fatalf("expected 1 ClearStatusCache call, got %d", len(cc.calls))
	}
	if cc.calls[0] != "session-1" {
		t.Errorf("expected ClearStatusCache called with session-1, got %s", cc.calls[0])
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestEndToEnd_JobProgressPublisher_ReaderReads(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	publisher := NewRedisJobProgressPublisher(bridge, testLogger())
	reader := NewRedisSyncProgressReader(client)

	progressData := map[string]any{
		"jobId":    "job-99",
		"progress": 42,
		"total":    100,
		"status":   "running",
	}

	publisher.PublishJobProgress("user-1", progressData)

	// Give time for Redis SET
	time.Sleep(100 * time.Millisecond)

	result, err := reader.GetSyncProgress(context.Background(), "job-99")
	if err != nil {
		t.Fatalf("GetSyncProgress returned error: %v", err)
	}

	if result["jobId"] != "job-99" {
		t.Errorf("expected jobId=job-99, got %v", result["jobId"])
	}
	if result["progress"] != float64(42) {
		t.Errorf("expected progress=42, got %v", result["progress"])
	}
	if result["status"] != "running" {
		t.Errorf("expected status=running, got %v", result["status"])
	}
}

func TestEndToEnd_StatusNotifier_FullFlow(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	// --- Server side ---
	serverBridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	serverBridge.SetHub(hub)

	sseClient := newTestClient("user-abc")
	registerClient(t, hub, sseClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := serverBridge.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	// --- Worker side ---
	workerBridge := NewSSERedisBridge(client, testLogger())
	resolver := func(sessionName string) (string, error) {
		if sessionName == "wa-session-1" {
			return "user-abc", nil
		}
		return "", errors.New("unknown session")
	}
	notifier := NewRedisSSEStatusNotifier(workerBridge, testLogger(), resolver)

	err = notifier.ProcessStatus(ctx, "wa-session-1", "WORKING")
	if err != nil {
		t.Fatalf("ProcessStatus returned error: %v", err)
	}

	// Verify the SSE client receives the event
	evt := readEvent(t, sseClient.channel, 3*time.Second)
	if evt.Type != SSEEventSessionStatus {
		t.Errorf("expected type=%s, got %s", SSEEventSessionStatus, evt.Type)
	}
	if evt.UserID != "user-abc" {
		t.Errorf("expected userID=user-abc, got %s", evt.UserID)
	}
	if evt.Data["session"] != "wa-session-1" {
		t.Errorf("expected data.session=wa-session-1, got %v", evt.Data["session"])
	}
	if evt.Data["status"] != "WORKING" {
		t.Errorf("expected data.status=WORKING, got %v", evt.Data["status"])
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// =============================================================================
// Tests: Publish marshal error (lines 69-71)
// =============================================================================

func TestPublish_MarshalError_ReturnsError(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())

	// json.Marshal cannot serialize a channel — triggers marshal error
	event := SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-1",
		Data:   map[string]any{"bad": make(chan int)},
	}

	err := bridge.Publish(context.Background(), event)
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if !strings.Contains(err.Error(), "marshal SSE event") {
		t.Errorf("expected error to contain 'marshal SSE event', got: %s", err.Error())
	}
}

// =============================================================================
// Tests: subscribeLoop reconnect — successful reconnect (lines 140-195)
// =============================================================================

func TestSubscribeLoop_ReconnectsSuccessfully(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	defer mr.Close()
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	sseClient := newTestClient("user-1")
	registerClient(t, hub, sseClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create initial pubsub
	pubsub := client.Subscribe(ctx, sseRedisChannel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		bridge.subscribeLoop(ctx, pubsub)
		close(done)
	}()

	// Close the initial pubsub externally so drainPubSub returns (channel closed)
	time.Sleep(50 * time.Millisecond)
	pubsub.Close()

	// subscribeLoop should reconnect (miniredis is still running).
	// Wait for reconnect to complete and publish a message through the new subscription.
	time.Sleep(500 * time.Millisecond)

	// Publish an event — if reconnect succeeded, the bridge should forward it to the hub
	event := SSEEvent{
		Type:   SSEEventGroupSync,
		UserID: "user-1",
		Data:   map[string]any{"synced": true},
	}
	data, _ := json.Marshal(event)
	client.Publish(ctx, sseRedisChannel, data)

	evt := readEvent(t, sseClient.channel, 3*time.Second)
	if evt.Type != SSEEventGroupSync {
		t.Errorf("expected type=%s after reconnect, got %s", SSEEventGroupSync, evt.Type)
	}

	cancel()

	select {
	case <-done:
		// subscribeLoop exited
	case <-time.After(5 * time.Second):
		t.Fatal("subscribeLoop did not exit after context cancellation")
	}
}

// =============================================================================
// Tests: subscribeLoop reconnect failure with backoff (lines 168-182)
// =============================================================================

func TestSubscribeLoop_ReconnectFailure_ExitsOnCancel(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create initial pubsub
	pubsub := client.Subscribe(ctx, sseRedisChannel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		bridge.subscribeLoop(ctx, pubsub)
		close(done)
	}()

	// Close the initial pubsub so drainPubSub returns
	time.Sleep(50 * time.Millisecond)
	pubsub.Close()

	// Close miniredis so reconnect fails (exercises backoff path, lines 168-182)
	time.Sleep(50 * time.Millisecond)
	mr.Close()
	client.Close()

	// Let the reconnect loop attempt and fail at least once (backoff starts at 100ms)
	time.Sleep(300 * time.Millisecond)

	// Cancel context to exit the loop
	cancel()

	select {
	case <-done:
		// subscribeLoop exited after failed reconnect
	case <-time.After(5 * time.Second):
		t.Fatal("subscribeLoop did not exit after context cancellation during reconnect failure")
	}
}

// =============================================================================
// Tests: subscribeLoop — backoff capped at maxBackoff (lines 179-181)
// =============================================================================

func TestSubscribeLoop_BackoffCappedAtMax(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)

	bridge := NewSSERedisBridge(client, testLogger())
	// Use tiny backoff values so we reach the cap quickly without long waits.
	// With base=1ms and max=5ms, the sequence is: 1ms → 2ms → 4ms → 8ms → capped to 5ms
	bridge.baseBackoff = 1 * time.Millisecond
	bridge.maxBackoff = 5 * time.Millisecond

	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create initial pubsub
	pubsub := client.Subscribe(ctx, sseRedisChannel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		bridge.subscribeLoop(ctx, pubsub)
		close(done)
	}()

	// Close initial pubsub to start reconnect loop
	time.Sleep(20 * time.Millisecond)
	pubsub.Close()

	// Close miniredis so all reconnect attempts fail
	time.Sleep(20 * time.Millisecond)
	mr.Close()
	client.Close()

	// Wait for enough iterations to exceed maxBackoff cap:
	// 1ms + 2ms + 4ms + 5ms + 5ms + 5ms = ~22ms — give generous margin
	time.Sleep(200 * time.Millisecond)

	// Cancel context to exit
	cancel()

	select {
	case <-done:
		// subscribeLoop exited after reaching backoff cap
	case <-time.After(5 * time.Second):
		t.Fatal("subscribeLoop did not exit after backoff cap test")
	}
}

// =============================================================================
// Tests: subscribeLoop — close error after reconnect drain (lines 193-195)
// =============================================================================

func TestSubscribeLoop_CloseErrorAfterReconnectDrain(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	bridge.baseBackoff = 1 * time.Millisecond
	bridge.maxBackoff = 5 * time.Millisecond

	// Inject closePubSub that returns an error on the second call (after reconnect drain)
	var closeCallCount int32
	bridge.closePubSub = func(ps *redis.PubSub) error {
		closeCallCount++
		closeErr := ps.Close()
		if closeCallCount >= 2 {
			return errors.New("forced close error after reconnect")
		}
		return closeErr
	}

	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create initial pubsub
	pubsub := client.Subscribe(ctx, sseRedisChannel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		bridge.subscribeLoop(ctx, pubsub)
		close(done)
	}()

	// Close initial pubsub to end initial drain → enters reconnect loop
	time.Sleep(30 * time.Millisecond)
	pubsub.Close()

	// miniredis is still running — reconnect succeeds → new drain starts.
	time.Sleep(100 * time.Millisecond)

	// Cancel context so drain exits → closePubSub call #2 returns error
	cancel()

	select {
	case <-done:
		// subscribeLoop exited — close error after drain was exercised
	case <-time.After(5 * time.Second):
		t.Fatal("subscribeLoop did not exit after close-error-after-drain test")
	}

	if closeCallCount < 2 {
		t.Errorf("expected at least 2 closePubSub calls, got %d", closeCallCount)
	}
}

// =============================================================================
// Tests: subscribeLoop — context cancelled during backoff wait (lines 157-163)
// =============================================================================

func TestSubscribeLoop_ContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pubsub := client.Subscribe(ctx, sseRedisChannel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		bridge.subscribeLoop(ctx, pubsub)
		close(done)
	}()

	// Close pubsub so drainPubSub returns, triggering the reconnect loop
	time.Sleep(50 * time.Millisecond)
	pubsub.Close()

	// Close miniredis so it enters the backoff wait
	time.Sleep(50 * time.Millisecond)
	mr.Close()
	client.Close()

	// Cancel context during the backoff timer wait (lines 159-162)
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// subscribeLoop exited during backoff
	case <-time.After(5 * time.Second):
		t.Fatal("subscribeLoop did not exit when context cancelled during backoff")
	}
}

// =============================================================================
// Tests: drainPubSub channel-closed (!ok path, lines 209-212)
// =============================================================================

func TestDrainPubSub_StopsOnChannelClose(t *testing.T) {
	t.Parallel()
	_, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()
	bridge.SetHub(hub)

	ctx := context.Background() // intentionally not cancelled — we test channel close path

	pubsub := client.Subscribe(ctx, sseRedisChannel)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		bridge.drainPubSub(ctx, pubsub)
		close(done)
	}()

	// Give drainPubSub time to start reading from the channel
	time.Sleep(50 * time.Millisecond)

	// Close pubsub — this closes the internal channel, triggering the !ok branch (lines 209-212)
	pubsub.Close()

	select {
	case <-done:
		// drainPubSub exited because channel was closed (not context cancellation)
	case <-time.After(3 * time.Second):
		t.Fatal("drainPubSub did not exit after pubsub channel was closed")
	}
}

// =============================================================================
// Tests: PublishJobProgress marshal error (lines 353-361)
// =============================================================================

func TestRedisJobProgressPublisher_MarshalError_LogsAndContinues(t *testing.T) {
	t.Parallel()
	mr, client := newMiniRedis(t)
	defer client.Close()

	bridge := NewSSERedisBridge(client, testLogger())
	publisher := NewRedisJobProgressPublisher(bridge, testLogger())

	// math.NaN() causes json.Marshal to fail with "unsupported value: NaN".
	// This triggers the marshal error at line 352-360.
	// The Publish at line 340 also fails (NaN in SSEEvent.Data) — so both
	// error paths are exercised. The function should not panic.
	progressData := map[string]any{
		"jobId":    "job-nan",
		"progress": math.NaN(),
	}

	// Should not panic — errors are logged via observability
	publisher.PublishJobProgress("user-1", progressData)

	// Verify no sync_progress key was set (marshal failed)
	keys := mr.Keys()
	for _, k := range keys {
		if strings.HasPrefix(k, "sync_progress:") {
			t.Errorf("expected no sync_progress key due to marshal error, found %s", k)
		}
	}
}
