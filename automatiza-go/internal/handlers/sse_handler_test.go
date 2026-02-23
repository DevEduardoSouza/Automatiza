package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Helper: read an SSEEvent from a client channel with timeout
// ---------------------------------------------------------------------------

func readEvent(t *testing.T, ch <-chan SSEEvent, timeout time.Duration) SSEEvent {
	t.Helper()
	select {
	case evt := <-ch:
		return evt
	case <-time.After(timeout):
		t.Fatal("timeout waiting for SSE event")
		return SSEEvent{} // unreachable
	}
}

// assertNoEvent verifies that no event is received within the given duration.
func assertNoEvent(t *testing.T, ch <-chan SSEEvent, wait time.Duration) {
	t.Helper()
	select {
	case evt := <-ch:
		t.Fatalf("expected no event, got %+v", evt)
	case <-time.After(wait):
		// ok — no event received
	}
}

// newTestClient creates an sseClient suitable for direct registration in the hub.
func newTestClient(userID string) *sseClient {
	return &sseClient{
		userID:   userID,
		channel:  make(chan SSEEvent, 32),
		doneChan: make(chan struct{}),
	}
}

// registerClient sends a client to the hub's register channel and waits for processing.
func registerClient(t *testing.T, hub *SSEHub, client *sseClient) {
	t.Helper()
	hub.register <- client
	time.Sleep(50 * time.Millisecond)
}

// =============================================================================
// Tests: NewSSEHub
// =============================================================================

func TestNewSSEHub_CreatesHubWithZeroClients(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	if count := hub.ClientCount(); count != 0 {
		t.Fatalf("expected 0 clients on new hub, got %d", count)
	}
}

// =============================================================================
// Tests: Publish — targeted vs broadcast
// =============================================================================

func TestPublish_TargetedEvent_OnlyReachesMatchingUser(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	clientA := newTestClient("user-a")
	clientB := newTestClient("user-b")
	registerClient(t, hub, clientA)
	registerClient(t, hub, clientB)

	// Publish event targeted to user-a
	hub.Publish(SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-a",
		Data:   map[string]any{"msg": "only-a"},
	})
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, clientA.channel, time.Second)
	if evt.Data["msg"] != "only-a" {
		t.Errorf("expected msg=only-a, got %v", evt.Data["msg"])
	}

	// user-b should NOT receive the event
	assertNoEvent(t, clientB.channel, 100*time.Millisecond)
}

func TestPublish_BroadcastEvent_ReachesAllClients(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	clientA := newTestClient("user-a")
	clientB := newTestClient("user-b")
	registerClient(t, hub, clientA)
	registerClient(t, hub, clientB)

	// Publish event without UserID → broadcast
	hub.Publish(SSEEvent{
		Type: SSEEventGroupSync,
		Data: map[string]any{"broadcast": true},
	})
	time.Sleep(50 * time.Millisecond)

	evtA := readEvent(t, clientA.channel, time.Second)
	if evtA.Data["broadcast"] != true {
		t.Errorf("clientA: expected broadcast=true, got %v", evtA.Data["broadcast"])
	}

	evtB := readEvent(t, clientB.channel, time.Second)
	if evtB.Data["broadcast"] != true {
		t.Errorf("clientB: expected broadcast=true, got %v", evtB.Data["broadcast"])
	}
}

func TestPublish_SetsTimestampWhenZero(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	before := time.Now().UnixMilli()
	hub.Publish(SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-1",
		Data:   map[string]any{"ts": "check"},
	})
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client.channel, time.Second)
	if evt.Timestamp < before {
		t.Errorf("expected timestamp >= %d, got %d", before, evt.Timestamp)
	}
}

// =============================================================================
// Tests: PublishSessionStatus
// =============================================================================

func TestPublishSessionStatus_WithoutQRCode(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	hub.PublishSessionStatus("user-1", "session-abc", "CONNECTED", "")
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client.channel, time.Second)

	if evt.Type != SSEEventSessionStatus {
		t.Errorf("expected type %s, got %s", SSEEventSessionStatus, evt.Type)
	}
	if evt.Session != "session-abc" {
		t.Errorf("expected session=session-abc, got %s", evt.Session)
	}
	if evt.UserID != "user-1" {
		t.Errorf("expected userID=user-1, got %s", evt.UserID)
	}
	if evt.Data["session"] != "session-abc" {
		t.Errorf("expected data.session=session-abc, got %v", evt.Data["session"])
	}
	if evt.Data["status"] != "CONNECTED" {
		t.Errorf("expected data.status=CONNECTED, got %v", evt.Data["status"])
	}
	if _, hasQR := evt.Data["qrCode"]; hasQR {
		t.Error("expected no qrCode in data when QR is empty")
	}
}

func TestPublishSessionStatus_WithQRCode(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	hub.PublishSessionStatus("user-1", "session-abc", "SCAN_QR_CODE", "base64data==")
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client.channel, time.Second)

	if evt.Type != SSEEventSessionStatus {
		t.Errorf("expected type %s, got %s", SSEEventSessionStatus, evt.Type)
	}
	if evt.Data["qrCode"] != "base64data==" {
		t.Errorf("expected data.qrCode=base64data==, got %v", evt.Data["qrCode"])
	}
	if evt.Data["session"] != "session-abc" {
		t.Errorf("expected data.session=session-abc, got %v", evt.Data["session"])
	}
	if evt.Data["status"] != "SCAN_QR_CODE" {
		t.Errorf("expected data.status=SCAN_QR_CODE, got %v", evt.Data["status"])
	}
}

// =============================================================================
// Tests: PublishGroupChange
// =============================================================================

func TestPublishGroupChange_SetsEventTypeAndGroupID(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	hub.PublishGroupChange("user-1", string(SSEEventGroupCreated), "group-42")
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client.channel, time.Second)

	if evt.Type != SSEEventGroupCreated {
		t.Errorf("expected type %s, got %s", SSEEventGroupCreated, evt.Type)
	}
	if evt.UserID != "user-1" {
		t.Errorf("expected userID=user-1, got %s", evt.UserID)
	}
	if evt.Data["groupId"] != "group-42" {
		t.Errorf("expected data.groupId=group-42, got %v", evt.Data["groupId"])
	}
}

// =============================================================================
// Tests: PublishJobStatus
// =============================================================================

func TestPublishJobStatus_WithoutErrorMessage(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	hub.PublishJobStatus("user-1", "job-99", "completed", nil)
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client.channel, time.Second)

	if evt.Type != SSEEventJobStatus {
		t.Errorf("expected type %s, got %s", SSEEventJobStatus, evt.Type)
	}
	if evt.Data["jobId"] != "job-99" {
		t.Errorf("expected data.jobId=job-99, got %v", evt.Data["jobId"])
	}
	if evt.Data["status"] != "completed" {
		t.Errorf("expected data.status=completed, got %v", evt.Data["status"])
	}
	if _, hasErr := evt.Data["errorMessage"]; hasErr {
		t.Error("expected no errorMessage in data when error is nil")
	}
}

func TestPublishJobStatus_WithErrorMessage(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	errMsg := "timeout exceeded"
	hub.PublishJobStatus("user-1", "job-99", "failed", &errMsg)
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client.channel, time.Second)

	if evt.Type != SSEEventJobStatus {
		t.Errorf("expected type %s, got %s", SSEEventJobStatus, evt.Type)
	}
	if evt.Data["jobId"] != "job-99" {
		t.Errorf("expected data.jobId=job-99, got %v", evt.Data["jobId"])
	}
	if evt.Data["status"] != "failed" {
		t.Errorf("expected data.status=failed, got %v", evt.Data["status"])
	}
	if evt.Data["errorMessage"] != "timeout exceeded" {
		t.Errorf("expected data.errorMessage=timeout exceeded, got %v", evt.Data["errorMessage"])
	}
}

// =============================================================================
// Tests: PublishHistoryChange
// =============================================================================

func TestPublishHistoryChange_SetsHistoryIDAndStatus(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	hub.PublishHistoryChange("user-1", "hist-77", "running")
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client.channel, time.Second)

	if evt.Type != SSEEventHistoryChange {
		t.Errorf("expected type %s, got %s", SSEEventHistoryChange, evt.Type)
	}
	if evt.UserID != "user-1" {
		t.Errorf("expected userID=user-1, got %s", evt.UserID)
	}
	if evt.Data["historyId"] != "hist-77" {
		t.Errorf("expected data.historyId=hist-77, got %v", evt.Data["historyId"])
	}
	if evt.Data["status"] != "running" {
		t.Errorf("expected data.status=running, got %v", evt.Data["status"])
	}
}

// =============================================================================
// Tests: PublishJobProgress
// =============================================================================

func TestPublishJobProgress_PassesThroughData(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	progressData := map[string]any{
		"jobId":    "job-55",
		"progress": 75,
		"total":    100,
		"status":   "running",
	}
	hub.PublishJobProgress("user-1", progressData)
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client.channel, time.Second)

	if evt.Type != SSEEventJobStatus {
		t.Errorf("expected type %s, got %s", SSEEventJobStatus, evt.Type)
	}
	if evt.UserID != "user-1" {
		t.Errorf("expected userID=user-1, got %s", evt.UserID)
	}
	if evt.Data["jobId"] != "job-55" {
		t.Errorf("expected data.jobId=job-55, got %v", evt.Data["jobId"])
	}
	// progress is int in the map, comes back as int
	if evt.Data["progress"] != 75 {
		t.Errorf("expected data.progress=75, got %v", evt.Data["progress"])
	}
	if evt.Data["total"] != 100 {
		t.Errorf("expected data.total=100, got %v", evt.Data["total"])
	}
}

// =============================================================================
// Tests: ClientCount
// =============================================================================

func TestClientCount_TracksRegistrations(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients initially, got %d", hub.ClientCount())
	}

	c1 := newTestClient("user-1")
	registerClient(t, hub, c1)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client after first register, got %d", hub.ClientCount())
	}

	c2 := newTestClient("user-2")
	registerClient(t, hub, c2)

	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients after second register, got %d", hub.ClientCount())
	}
}

func TestClientCount_ReplacesExistingClientForSameUser(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	c1 := newTestClient("user-1")
	registerClient(t, hub, c1)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}

	// Register a second client with the same userID — should replace, not add
	c2 := newTestClient("user-1")
	registerClient(t, hub, c2)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client after replacement, got %d", hub.ClientCount())
	}
}

func TestClientCount_DecreasesOnUnregister(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.unregister <- client
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients after unregister, got %d", hub.ClientCount())
	}
}

// =============================================================================
// Tests: Close — graceful shutdown
// =============================================================================

func TestClose_GracefullyStopsHub(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client before close, got %d", hub.ClientCount())
	}

	hub.Close()
	// Give the run loop time to process the done signal
	time.Sleep(100 * time.Millisecond)

	// After Close, the clients map is emptied
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after close, got %d", hub.ClientCount())
	}
}

func TestClose_ClosesClientChannels(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	hub.Close()
	time.Sleep(100 * time.Millisecond)

	// Reading from a closed channel returns zero value + ok=false
	select {
	case _, ok := <-client.channel:
		if ok {
			t.Error("expected client channel to be closed after hub.Close()")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout: client channel should be closed and readable immediately")
	}
}

// =============================================================================
// Tests: SSEStatusNotifier.ProcessStatus
// =============================================================================

func TestSSEStatusNotifier_ProcessStatus_Success(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("resolved-user")
	registerClient(t, hub, client)

	resolver := func(sessionName string) (string, error) {
		if sessionName == "my-session" {
			return "resolved-user", nil
		}
		return "", errors.New("not found")
	}

	notifier := NewSSEStatusNotifier(hub, zap.NewNop(), resolver)

	err := notifier.ProcessStatus(context.Background(), "my-session", "CONNECTED")
	if err != nil {
		t.Fatalf("ProcessStatus returned error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client.channel, time.Second)

	if evt.Type != SSEEventSessionStatus {
		t.Errorf("expected type %s, got %s", SSEEventSessionStatus, evt.Type)
	}
	if evt.UserID != "resolved-user" {
		t.Errorf("expected userID=resolved-user, got %s", evt.UserID)
	}
	if evt.Data["session"] != "my-session" {
		t.Errorf("expected data.session=my-session, got %v", evt.Data["session"])
	}
	if evt.Data["status"] != "CONNECTED" {
		t.Errorf("expected data.status=CONNECTED, got %v", evt.Data["status"])
	}
}

func TestSSEStatusNotifier_ProcessStatus_ResolverError_ReturnsNil(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	client := newTestClient("user-1")
	registerClient(t, hub, client)

	resolver := func(sessionName string) (string, error) {
		return "", errors.New("session not found")
	}

	notifier := NewSSEStatusNotifier(hub, zap.NewNop(), resolver)

	err := notifier.ProcessStatus(context.Background(), "deleted-session", "DISCONNECTED")
	if err != nil {
		t.Fatalf("expected nil error on resolver failure, got: %v", err)
	}

	// No event should be published
	assertNoEvent(t, client.channel, 200*time.Millisecond)
}

func TestSSEStatusNotifier_ProcessStatus_ResolverReturnsCorrectUser(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	clientA := newTestClient("user-a")
	clientB := newTestClient("user-b")
	registerClient(t, hub, clientA)
	registerClient(t, hub, clientB)

	resolver := func(sessionName string) (string, error) {
		// Map session to user-b
		return "user-b", nil
	}

	notifier := NewSSEStatusNotifier(hub, zap.NewNop(), resolver)

	err := notifier.ProcessStatus(context.Background(), "session-x", "WORKING")
	if err != nil {
		t.Fatalf("ProcessStatus returned error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// user-b should receive the event
	evt := readEvent(t, clientB.channel, time.Second)
	if evt.UserID != "user-b" {
		t.Errorf("expected event for user-b, got userID=%s", evt.UserID)
	}

	// user-a should NOT receive the event
	assertNoEvent(t, clientA.channel, 100*time.Millisecond)
}

// =============================================================================
// Tests: NewSSEHandler (constructor)
// =============================================================================

func TestNewSSEHandler_CreatesHandlerWithHubAndLogger(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	logger := zap.NewNop()
	handler := NewSSEHandler(hub, logger)

	if handler == nil {
		t.Fatal("expected non-nil SSEHandler")
	}
	if handler.hub != hub {
		t.Error("expected handler.hub to reference the provided hub")
	}
	if handler.logger == nil {
		t.Error("expected handler.logger to be non-nil")
	}
}

func TestNewSSEHandler_LoggerIsNamed(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	// Use a development logger so Named() actually shows in the logger name
	logger, _ := zap.NewDevelopment()
	handler := NewSSEHandler(hub, logger)

	// The handler should have a named logger (sse-handler).
	// We verify the handler was created successfully with a non-nil logger.
	if handler.logger == nil {
		t.Error("expected handler.logger to be non-nil after Named()")
	}
}

// =============================================================================
// Tests: HandleSSE — authentication and error paths
// =============================================================================

func TestHandleSSE_NoUserID_Returns401(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	handler := NewSSEHandler(hub, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil)

	// Do NOT set userID in context
	handler.HandleSSE(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("expected error=unauthorized, got %q", body["error"])
	}
}

func TestHandleSSE_NonStringUserID_Returns500(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	handler := NewSSEHandler(hub, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil)

	// Set userID as a non-string type
	c.Set("userID", 12345)

	handler.HandleSSE(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body["error"] != "invalid user ID" {
		t.Errorf("expected error='invalid user ID', got %q", body["error"])
	}
}

// =============================================================================
// Tests: HandleSSE — streaming (headers, initial event, disconnect)
// =============================================================================

func TestHandleSSE_SetsCorrectSSEHeaders(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	handler := NewSSEHandler(hub, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil).WithContext(ctx)
	c.Set("userID", "user-123")

	done := make(chan struct{})
	go func() {
		handler.HandleSSE(c)
		close(done)
	}()

	// Give the handler time to set headers and write the initial event
	time.Sleep(150 * time.Millisecond)

	// Cancel context to disconnect the client
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleSSE did not return after context cancel")
	}

	// Verify SSE headers
	headers := w.Header()
	if ct := headers.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type=text/event-stream, got %q", ct)
	}
	if cc := headers.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control=no-cache, got %q", cc)
	}
	if conn := headers.Get("Connection"); conn != "keep-alive" {
		t.Errorf("expected Connection=keep-alive, got %q", conn)
	}
	if xab := headers.Get("X-Accel-Buffering"); xab != "no" {
		t.Errorf("expected X-Accel-Buffering=no, got %q", xab)
	}
	if acao := headers.Get("Access-Control-Allow-Origin"); acao != "*" {
		t.Errorf("expected Access-Control-Allow-Origin=*, got %q", acao)
	}
}

func TestHandleSSE_SendsInitialConnectedEvent(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	handler := NewSSEHandler(hub, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil).WithContext(ctx)
	c.Set("userID", "user-42")

	done := make(chan struct{})
	go func() {
		handler.HandleSSE(c)
		close(done)
	}()

	// Wait for initial event to be written
	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleSSE did not return after context cancel")
	}

	body := w.Body.String()

	// Verify SSE format: "event: connected\ndata: {...}\n\n"
	if !strings.Contains(body, "event: connected\n") {
		t.Errorf("expected 'event: connected' in body, got:\n%s", body)
	}
	if !strings.Contains(body, "data: ") {
		t.Errorf("expected 'data: ' in body, got:\n%s", body)
	}

	// Extract the JSON data line
	lines := strings.Split(body, "\n")
	var dataJSON string
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataJSON = strings.TrimPrefix(line, "data: ")
			break
		}
	}
	if dataJSON == "" {
		t.Fatalf("no data line found in SSE output:\n%s", body)
	}

	var event SSEEvent
	if err := json.Unmarshal([]byte(dataJSON), &event); err != nil {
		t.Fatalf("failed to parse SSE data JSON: %v", err)
	}
	if event.Type != "connected" {
		t.Errorf("expected type=connected, got %s", event.Type)
	}
	if event.UserID != "user-42" {
		t.Errorf("expected userID=user-42, got %s", event.UserID)
	}
	if event.Data["connected"] != true {
		t.Errorf("expected data.connected=true, got %v", event.Data["connected"])
	}
	if event.Timestamp == 0 {
		t.Error("expected non-zero timestamp in connected event")
	}
}

func TestHandleSSE_ClientDisconnect_ReturnsAndUnregisters(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	handler := NewSSEHandler(hub, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil).WithContext(ctx)
	c.Set("userID", "user-disconnect")

	done := make(chan struct{})
	go func() {
		handler.HandleSSE(c)
		close(done)
	}()

	// Wait for client to register
	time.Sleep(150 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client after connection, got %d", hub.ClientCount())
	}

	// Simulate client disconnect
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleSSE did not return after context cancel")
	}

	// Wait for unregister to be processed
	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", hub.ClientCount())
	}
}

func TestHandleSSE_ReceivesPublishedEvent(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	handler := NewSSEHandler(hub, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil).WithContext(ctx)
	c.Set("userID", "user-stream")

	done := make(chan struct{})
	go func() {
		handler.HandleSSE(c)
		close(done)
	}()

	// Wait for client to register and initial event to be written
	time.Sleep(150 * time.Millisecond)

	// Publish an event to this user
	hub.Publish(SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-stream",
		Data:   map[string]any{"session": "test-session", "status": "CONNECTED"},
	})

	// Wait for event to be written
	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleSSE did not return after context cancel")
	}

	body := w.Body.String()

	// Should contain both "connected" and "session.status" events
	if !strings.Contains(body, "event: connected\n") {
		t.Errorf("expected initial 'event: connected' in body")
	}
	if !strings.Contains(body, "event: session.status\n") {
		t.Errorf("expected 'event: session.status' in body, got:\n%s", body)
	}
}

func TestHandleSSE_ChannelClosed_ReturnsCleanly(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	handler := NewSSEHandler(hub, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Use a context that does NOT cancel (we rely on channel closure)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil)
	c.Set("userID", "user-replace")

	done := make(chan struct{})
	go func() {
		handler.HandleSSE(c)
		close(done)
	}()

	// Wait for client to register
	time.Sleep(150 * time.Millisecond)

	// Register a second client with the same userID -> closes the first client's channel
	client2 := newTestClient("user-replace")
	registerClient(t, hub, client2)

	select {
	case <-done:
		// HandleSSE returned because channel was closed — correct behavior
	case <-time.After(2 * time.Second):
		t.Fatal("HandleSSE did not return after channel was closed by replacement")
	}
}

// =============================================================================
// Tests: writeEvent — tested indirectly via HandleSSE
// =============================================================================

func TestWriteEvent_ProducesCorrectSSEFormat(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	handler := NewSSEHandler(hub, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil).WithContext(ctx)
	c.Set("userID", "user-format")

	done := make(chan struct{})
	go func() {
		handler.HandleSSE(c)
		close(done)
	}()

	time.Sleep(150 * time.Millisecond)

	// Publish an event with specific data
	hub.Publish(SSEEvent{
		Type:   SSEEventQRCode,
		UserID: "user-format",
		Data:   map[string]any{"qr": "base64data"},
	})

	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleSSE did not return")
	}

	body := w.Body.String()

	// Verify SSE format: each event is "event: <type>\ndata: <json>\n\n"
	// We expect two events: connected + qr.code
	events := strings.Split(body, "\n\n")
	// Filter out empty trailing entries
	var nonEmpty []string
	for _, e := range events {
		if strings.TrimSpace(e) != "" {
			nonEmpty = append(nonEmpty, e)
		}
	}

	if len(nonEmpty) < 2 {
		t.Fatalf("expected at least 2 SSE events, got %d in body:\n%s", len(nonEmpty), body)
	}

	// Verify second event is qr.code
	qrEventBlock := nonEmpty[1]
	if !strings.HasPrefix(qrEventBlock, "event: qr.code\n") {
		t.Errorf("expected second event to start with 'event: qr.code', got:\n%s", qrEventBlock)
	}

	qrLines := strings.Split(qrEventBlock, "\n")
	var qrDataJSON string
	for _, line := range qrLines {
		if strings.HasPrefix(line, "data: ") {
			qrDataJSON = strings.TrimPrefix(line, "data: ")
			break
		}
	}
	if qrDataJSON == "" {
		t.Fatalf("no data line found in qr.code event:\n%s", qrEventBlock)
	}

	var evt SSEEvent
	if err := json.Unmarshal([]byte(qrDataJSON), &evt); err != nil {
		t.Fatalf("failed to parse qr.code data: %v", err)
	}
	if evt.Type != SSEEventQRCode {
		t.Errorf("expected type=qr.code, got %s", evt.Type)
	}
	if evt.Data["qr"] != "base64data" {
		t.Errorf("expected data.qr=base64data, got %v", evt.Data["qr"])
	}
}

// =============================================================================
// Tests: Hub run loop edge cases
// =============================================================================

func TestHub_UnregisterNonMatchingClient_DoesNotRemoveCurrent(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	// Register client1
	client1 := newTestClient("user-1")
	registerClient(t, hub, client1)

	// Replace client1 with client2 for the same userID
	client2 := newTestClient("user-1")
	registerClient(t, hub, client2)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client after replacement, got %d", hub.ClientCount())
	}

	// Now try to unregister client1 (the old one, which has already been replaced).
	// The run loop checks `existing == client` — since the current client is client2,
	// client1's unregister should be a no-op.
	hub.unregister <- client1
	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("expected client count to remain 1 after unregistering stale client, got %d", hub.ClientCount())
	}

	// Verify client2 is still functional by publishing an event
	hub.Publish(SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-1",
		Data:   map[string]any{"test": "still-alive"},
	})
	time.Sleep(50 * time.Millisecond)

	evt := readEvent(t, client2.channel, time.Second)
	if evt.Data["test"] != "still-alive" {
		t.Errorf("expected data.test=still-alive, got %v", evt.Data["test"])
	}
}

func TestHub_TargetedEvent_ClientChannelFull_DropsEvent(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	// Create a client with a tiny channel buffer (size 1) to easily fill it
	client := &sseClient{
		userID:   "user-full",
		channel:  make(chan SSEEvent, 1),
		doneChan: make(chan struct{}),
	}
	registerClient(t, hub, client)

	// Fill the client's channel
	client.channel <- SSEEvent{Type: "filler", Data: map[string]any{}}

	// Now publish a targeted event — the hub should drop it (default branch)
	hub.Publish(SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-full",
		Data:   map[string]any{"dropped": true},
	})
	time.Sleep(100 * time.Millisecond)

	// Read the filler event
	filler := readEvent(t, client.channel, time.Second)
	if filler.Type != "filler" {
		t.Errorf("expected filler event, got type=%s", filler.Type)
	}

	// The dropped event should NOT be in the channel
	assertNoEvent(t, client.channel, 200*time.Millisecond)
}

func TestHub_BroadcastEvent_ClientChannelFull_DropsForFullClient(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	// clientA has a full channel (buffer 1)
	clientA := &sseClient{
		userID:   "user-a",
		channel:  make(chan SSEEvent, 1),
		doneChan: make(chan struct{}),
	}
	registerClient(t, hub, clientA)

	// clientB has a normal channel
	clientB := newTestClient("user-b")
	registerClient(t, hub, clientB)

	// Fill clientA's channel
	clientA.channel <- SSEEvent{Type: "filler", Data: map[string]any{}}

	// Broadcast event (no UserID) — should reach clientB but be dropped for clientA
	hub.Publish(SSEEvent{
		Type: SSEEventGroupSync,
		Data: map[string]any{"broadcast": "test"},
	})
	time.Sleep(100 * time.Millisecond)

	// clientB should receive the broadcast
	evtB := readEvent(t, clientB.channel, time.Second)
	if evtB.Data["broadcast"] != "test" {
		t.Errorf("expected clientB to receive broadcast, got %v", evtB.Data)
	}

	// clientA should only have the filler event
	filler := readEvent(t, clientA.channel, time.Second)
	if filler.Type != "filler" {
		t.Errorf("expected filler event for clientA, got type=%s", filler.Type)
	}
	assertNoEvent(t, clientA.channel, 200*time.Millisecond)
}

func TestHub_Publish_BroadcastChannelFull_DropsEvent(t *testing.T) {
	t.Parallel()
	// Create a hub manually (without NewSSEHub) to control the broadcast channel size
	// We need to avoid starting the run loop so the broadcast channel fills up.
	hub := &SSEHub{
		clients:    make(map[string]*sseClient),
		broadcast:  make(chan SSEEvent, 1), // tiny buffer
		register:   make(chan *sseClient, 16),
		unregister: make(chan *sseClient, 16),
		logger:     zap.NewNop().Named("sse-hub"),
		done:       make(chan struct{}),
	}
	// Do NOT start the run loop — so nothing drains the broadcast channel

	// Fill the broadcast channel
	hub.broadcast <- SSEEvent{Type: "filler", Data: map[string]any{}}

	// This Publish should hit the default branch (channel full) and not block
	publishDone := make(chan struct{})
	go func() {
		hub.Publish(SSEEvent{
			Type: SSEEventSessionStatus,
			Data: map[string]any{"dropped": true},
		})
		close(publishDone)
	}()

	select {
	case <-publishDone:
		// Publish returned without blocking — correct behavior
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked when broadcast channel was full")
	}
}

func TestHub_Heartbeat_DeliveredToClients(t *testing.T) {
	t.Parallel()
	// Create a hub with a very short heartbeat interval for fast testing.
	hub := &SSEHub{
		clients:           make(map[string]*sseClient),
		broadcast:         make(chan SSEEvent, 256),
		register:          make(chan *sseClient, 16),
		unregister:        make(chan *sseClient, 16),
		logger:            zap.NewNop().Named("sse-hub"),
		done:              make(chan struct{}),
		heartbeatInterval: 20 * time.Millisecond,
	}
	go hub.run()
	defer hub.Close()

	client := newTestClient("user-hb")
	registerClient(t, hub, client)

	// Wait for the heartbeat ticker to fire (20ms interval)
	evt := readEvent(t, client.channel, 2*time.Second)
	if evt.Type != SSEEventHeartbeat {
		t.Errorf("expected heartbeat event type, got %s", evt.Type)
	}
	if evt.Data["ping"] != true {
		t.Errorf("expected data.ping=true, got %v", evt.Data["ping"])
	}
}

func TestHub_HeartbeatSkipsFullChannel(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	// Create a client with a buffer of 1 and fill it
	client := &sseClient{
		userID:   "user-hb-full",
		channel:  make(chan SSEEvent, 1),
		doneChan: make(chan struct{}),
	}
	registerClient(t, hub, client)

	// Fill the channel
	client.channel <- SSEEvent{Type: "filler", Data: map[string]any{}}

	// Send a heartbeat through broadcast — the hub should skip the full client
	heartbeat := SSEEvent{
		Type:      SSEEventHeartbeat,
		Data:      map[string]any{"ping": true},
		Timestamp: time.Now().UnixMilli(),
	}
	hub.broadcast <- heartbeat
	time.Sleep(100 * time.Millisecond)

	// Drain the filler
	filler := readEvent(t, client.channel, time.Second)
	if filler.Type != "filler" {
		t.Errorf("expected filler, got %s", filler.Type)
	}

	// Heartbeat should have been dropped
	assertNoEvent(t, client.channel, 200*time.Millisecond)
}

func TestHub_HeartbeatTicker_DeliversToMultipleClients(t *testing.T) {
	t.Parallel()
	// Use a hub with a short heartbeat interval to test the actual ticker path (lines 199-214)
	hub := &SSEHub{
		clients:           make(map[string]*sseClient),
		broadcast:         make(chan SSEEvent, 256),
		register:          make(chan *sseClient, 16),
		unregister:        make(chan *sseClient, 16),
		logger:            zap.NewNop().Named("sse-hub"),
		done:              make(chan struct{}),
		heartbeatInterval: 30 * time.Millisecond,
	}
	go hub.run()
	defer hub.Close()

	clientA := newTestClient("user-a")
	clientB := newTestClient("user-b")
	registerClient(t, hub, clientA)
	registerClient(t, hub, clientB)

	// Wait for the ticker to fire
	evtA := readEvent(t, clientA.channel, 2*time.Second)
	if evtA.Type != SSEEventHeartbeat {
		t.Errorf("expected heartbeat for client A, got %s", evtA.Type)
	}

	evtB := readEvent(t, clientB.channel, 2*time.Second)
	if evtB.Type != SSEEventHeartbeat {
		t.Errorf("expected heartbeat for client B, got %s", evtB.Type)
	}

	if evtA.Data["ping"] != true {
		t.Errorf("expected ping=true, got %v", evtA.Data["ping"])
	}
}

func TestHub_HeartbeatTicker_SkipsFullChannels(t *testing.T) {
	t.Parallel()
	// Test the ticker path with a full channel (lines 207-212 default branch)
	hub := &SSEHub{
		clients:           make(map[string]*sseClient),
		broadcast:         make(chan SSEEvent, 256),
		register:          make(chan *sseClient, 16),
		unregister:        make(chan *sseClient, 16),
		logger:            zap.NewNop().Named("sse-hub"),
		done:              make(chan struct{}),
		heartbeatInterval: 500 * time.Millisecond, // long enough to fill channel first
	}
	go hub.run()
	defer hub.Close()

	// Create client with buffer 1
	fullClient := &sseClient{
		userID:   "user-full",
		channel:  make(chan SSEEvent, 1),
		doneChan: make(chan struct{}),
	}
	// Also create a normal client to confirm heartbeat was sent
	normalClient := newTestClient("user-normal")

	registerClient(t, hub, fullClient)
	registerClient(t, hub, normalClient)

	// Fill fullClient's channel before ticker fires
	fullClient.channel <- SSEEvent{Type: "filler", Data: map[string]any{}}

	// Wait for ticker to fire (500ms)
	evtNormal := readEvent(t, normalClient.channel, 2*time.Second)
	if evtNormal.Type != SSEEventHeartbeat {
		t.Errorf("expected heartbeat for normal client, got %s", evtNormal.Type)
	}

	// fullClient should only have the filler (heartbeat dropped)
	filler := readEvent(t, fullClient.channel, time.Second)
	if filler.Type != "filler" {
		t.Errorf("expected filler, got %s", filler.Type)
	}
	assertNoEvent(t, fullClient.channel, 200*time.Millisecond)
}

func TestHub_TargetedEvent_UserNotConnected_NoPanic(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	// Publish a targeted event for a user who is not connected
	// This should be a no-op — no panic, no error
	hub.Publish(SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "nonexistent-user",
		Data:   map[string]any{"test": true},
	})

	// Wait for the event to be processed
	time.Sleep(100 * time.Millisecond)

	// If we got here without panic, the test passes
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

// ---------------------------------------------------------------------------
// SSEHandler.writeEvent — json.Marshal error (lines 429-433)
// ---------------------------------------------------------------------------

func TestSSEHandler_WriteEvent_MarshalError(t *testing.T) {
	t.Parallel()
	hub := NewSSEHub(zap.NewNop())
	defer hub.Close()

	handler := NewSSEHandler(hub, zap.NewNop())

	// Create a gin response writer to pass to writeEvent
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil)

	// Use a channel value in Data which json.Marshal cannot handle
	unmarshalableEvent := SSEEvent{
		Type:   SSEEventSessionStatus,
		UserID: "user-1",
		Data:   map[string]any{"bad": make(chan struct{})},
	}

	// Call writeEvent directly — should log error and return without writing
	handler.writeEvent(c.Writer, unmarshalableEvent)

	// Verify nothing was written to the response body (marshal failed before write)
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body on marshal error, got %d bytes: %s", w.Body.Len(), w.Body.String())
	}
}
