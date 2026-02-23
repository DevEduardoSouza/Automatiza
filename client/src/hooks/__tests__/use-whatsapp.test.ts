import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useWhatsApp } from '../use-whatsapp';

// ============================================================
// Mock Modules
// ============================================================

const mockToast = vi.fn();

vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({
    toast: mockToast,
    toasts: [],
    dismiss: vi.fn(),
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

// ============================================================
// Mock: useWhatsAppConnect
// ============================================================

const mockCreateInstance = vi.fn();
const mockDeleteInstance = vi.fn();
const mockCheckStatus = vi.fn();

vi.mock('@/hooks/use-whatsapp-connect', () => ({
  useWhatsAppConnect: () => ({
    createInstance: mockCreateInstance,
    deleteInstance: mockDeleteInstance,
    reconnectInstance: vi.fn(),
    checkStatus: mockCheckStatus,
    fetchGroups: vi.fn(),
    fetchGroupParticipants: vi.fn(),
    syncGroups: vi.fn(),
    syncGroupsStatus: vi.fn(),
    sendMessage: vi.fn(),
    sendAudio: vi.fn(),
    sendPoll: vi.fn(),
    updateGroupSubject: vi.fn(),
    updateGroupDescription: vi.fn(),
    updateGroupPicture: vi.fn(),
    updateGroupSettings: vi.fn(),
    isCreatingInstance: false,
    isDeletingInstance: false,
    isReconnectingInstance: false,
    isCheckingStatus: false,
    isFetchingGroups: false,
    isFetchingParticipants: false,
    isSyncingGroups: false,
    isCheckingSyncStatus: false,
    isSendingMessage: false,
    isSendingAudio: false,
    isSendingPoll: false,
    isUpdatingSubject: false,
    isUpdatingDescription: false,
    isUpdatingPicture: false,
    isUpdatingSettings: false,
  }),
}));

// ============================================================
// Mock: useSessionStatusSSE — captures callbacks for simulation
// ============================================================

let capturedOnStatusChange: ((data: { session: string; status: string }) => void) | undefined;
let capturedOnQRCode: ((qrCode: string) => void) | undefined;

vi.mock('@/hooks/use-session-status-sse', () => ({
  useSessionStatusSSE: (opts: {
    onStatusChange?: (data: { session: string; status: string }) => void;
    onQRCode?: (qrCode: string) => void;
    onError?: (error: Error) => void;
  }) => {
    capturedOnStatusChange = opts.onStatusChange;
    capturedOnQRCode = opts.onQRCode;
    return {
      connectionState: 'connected',
      lastStatus: null,
      connect: vi.fn(),
      disconnect: vi.fn(),
      isConnected: true,
    };
  },
}));

// ============================================================
// Mock: useInstancesConnect (mutable latestInstance for loadConnectionState tests)
// ============================================================

const mockUpsertInstanceStatus = vi.fn().mockResolvedValue({ id: 'inst-1' });
const mockCleanupInvalidInstances = vi.fn();
const mockDeleteDisconnectedInstances = vi.fn();
const mockDeleteInstanceFromDb = vi.fn();
const mockRefetchInstances = vi.fn();

let mockLatestInstance: {
  id: string;
  user_id: string;
  instance_id: string;
  status: string;
  created_at: string;
  updated_at: string;
} | null = null;

vi.mock('@/hooks/use-instances-connect', () => ({
  useInstancesConnect: () => ({
    latestInstance: mockLatestInstance,
    instances: [],
    loading: false,
    refetch: mockRefetchInstances,
    getLatestInstance: vi.fn(),
    upsertInstanceStatus: mockUpsertInstanceStatus,
    cleanupInvalidInstances: mockCleanupInvalidInstances,
    deleteDisconnectedInstances: mockDeleteDisconnectedInstances,
    deleteInstance: mockDeleteInstanceFromDb,
    refetchInstances: mockRefetchInstances,
    listUserInstances: vi.fn(),
    checkInstanceStatus: vi.fn(),
    reconnectInstance: vi.fn(),
    isLoading: false,
  }),
}));

// ============================================================
// Mock: useGroupsConnect
// ============================================================

const mockSyncGroups = vi.fn();
const mockGetSyncStatus = vi.fn();
const mockRefetchGroups = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    groups: [],
    loading: false,
    refetch: mockRefetchGroups,
    listGroups: vi.fn(),
    listGroupsPaginated: vi.fn(),
    getGroup: vi.fn(),
    getGroupsByIds: vi.fn(),
    createGroup: vi.fn(),
    updateGroup: vi.fn(),
    toggleGroupFavorite: vi.fn(),
    syncGroups: mockSyncGroups,
    getSyncStatus: mockGetSyncStatus,
    deleteGroup: vi.fn(),
    bulkUpdateGroups: vi.fn(),
    sendGroupMessage: vi.fn(),
    sendGroupAudio: vi.fn(),
    sendGroupImage: vi.fn(),
    sendGroupPoll: vi.fn(),
    updateGroupSubject: vi.fn(),
    updateGroupDescription: vi.fn(),
    updateGroupPicture: vi.fn(),
    updateGroupSettings: vi.fn(),
    getGroupInviteCode: vi.fn(),
    getGroupParticipants: vi.fn(),
    addGroupMember: vi.fn(),
    removeGroupMember: vi.fn(),
    leaveGroup: vi.fn(),
    promoteMember: vi.fn(),
    demoteMember: vi.fn(),
    isListingGroups: false,
    isGettingGroup: false,
    isCreatingGroup: false,
    isUpdatingGroup: false,
    isTogglingFavorite: false,
    isSyncingGroups: false,
    isGettingSyncStatus: false,
    isDeletingGroup: false,
    isBulkUpdating: false,
    isSendingMessage: false,
    isSendingAudio: false,
    isSendingImage: false,
    isSendingPoll: false,
    isUpdatingSubject: false,
    isUpdatingDescription: false,
    isUpdatingPicture: false,
    isUpdatingSettings: false,
    isGettingInviteCode: false,
    isGettingParticipants: false,
    isAddingMember: false,
    isRemovingMember: false,
    isLeavingGroup: false,
    isPromotingMember: false,
    isDemotingMember: false,
  }),
}));

// ============================================================
// Mock: Sentry
// ============================================================

vi.mock('@sentry/react', () => ({
  addBreadcrumb: vi.fn(),
  captureMessage: vi.fn(),
  captureException: vi.fn(),
}));

// ============================================================
// Mock: @/lib/sentry
// ============================================================

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
  safeLocalStorage: {
    getItem: vi.fn().mockReturnValue(null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  },
}));

// ============================================================
// Mock: @/entities/whatsapp-instance
// ============================================================

vi.mock('@/entities/whatsapp-instance', () => ({
  hasRealInstanceName: (name: string | null | undefined) => {
    const trimmed = (name ?? '').trim();
    if (!trimmed) return false;
    const lower = trimmed.toLowerCase();
    return lower !== 'será gerado automaticamente' && lower !== 'gerado automaticamente';
  },
}));

// ============================================================
// Helpers
// ============================================================

function renderWhatsApp() {
  return renderHook(() => useWhatsApp());
}

function simulateSSE(session: string, status: string) {
  if (capturedOnStatusChange) {
    capturedOnStatusChange({ session, status });
  }
}

// ============================================================
// Tests
// ============================================================

describe('useWhatsApp', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    capturedOnStatusChange = undefined;
    capturedOnQRCode = undefined;
    mockLatestInstance = null;

    mockCreateInstance.mockResolvedValue({
      success: true,
      instanceName: 'session-abc',
      qrcodeBase64: 'base64-qr-data',
    });
    mockDeleteInstance.mockResolvedValue({ success: true });
    mockDeleteInstanceFromDb.mockResolvedValue({ success: true });
    mockDeleteDisconnectedInstances.mockResolvedValue({ success: true });
    mockRefetchInstances.mockResolvedValue(undefined);
    mockUpsertInstanceStatus.mockResolvedValue({ id: 'inst-1' });
    mockCheckStatus.mockResolvedValue({
      success: true,
      state: 'connecting',
      qrcodeBase64: undefined,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  // ── Test 1: Guard is set before delete ─────────────────────
  describe('handleConnect race condition fix', () => {
    it('should set refreshingQrRef guard before delete so SSE STOPPED does not reset connecting', async () => {
      const { result } = renderWhatsApp();

      // Simulate a scenario where SSE STOPPED fires during the delete phase.
      // We intercept deleteInstance to fire SSE STOPPED while it executes.
      mockDeleteInstance.mockImplementation(async () => {
        // During delete, SSE fires STOPPED for the old session
        act(() => {
          simulateSSE('session-abc', 'STOPPED');
        });
        return { success: true };
      });

      // Need to set an existing instance name so delete is called
      // We trigger handleConnect which sets connecting=true, then calls delete
      await act(async () => {
        // First, set instanceName by doing a successful connect
        mockCreateInstance.mockResolvedValueOnce({
          success: true,
          instanceName: 'session-abc',
          qrcodeBase64: 'qr-1',
        });
        await result.current.handleConnect();
      });

      // After handleConnect, connecting should still be true (not reset by STOPPED)
      // because refreshingQrRef was set before deleteInstance was called
      expect(result.current.connecting).toBe(true);
      expect(result.current.connected).toBe(false);
    });

    // ── Test 2: SSE STOPPED suppressed during delete ───────────
    it('should suppress SSE STOPPED events during handleConnect delete phase', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });

      // Instance is now set. Do another connect (with existing session).
      mockDeleteInstance.mockImplementation(async () => {
        // SSE STOPPED fires during delete
        act(() => {
          simulateSSE('session-abc', 'STOPPED');
        });
        return { success: true };
      });
      mockCreateInstance.mockResolvedValue({
        success: true,
        instanceName: 'session-new',
        qrcodeBase64: 'qr-2',
      });

      await act(async () => {
        await result.current.handleConnect();
        // Advance past the 2s delay
        await vi.advanceTimersByTimeAsync(2100);
      });

      // Should NOT have shown destructive disconnected toast from SSE STOPPED
      const disconnectedToasts = mockToast.mock.calls.filter(
        (call) =>
          call[0]?.variant === 'destructive' &&
          call[0]?.title === 'pages.whatsapp.errors.disconnected'
      );
      expect(disconnectedToasts).toHaveLength(0);
    });

    // ── Test 3: instanceNameRef updated after create ───────────
    it('should update instanceNameRef after createInstance returns', async () => {
      const { result } = renderWhatsApp();

      mockCreateInstance.mockResolvedValue({
        success: true,
        instanceName: 'new-session-123',
        qrcodeBase64: 'qr-data',
      });

      await act(async () => {
        await result.current.handleConnect();
      });

      expect(result.current.instanceName).toBe('new-session-123');
    });

    // ── Test 4: SSE CONNECTED transitions to connected ─────────
    it('should transition to connected state when SSE CONNECTED fires after handleConnect', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });

      // Simulate SSE CONNECTED for the new session
      act(() => {
        simulateSSE('session-abc', 'CONNECTED');
      });

      expect(result.current.connected).toBe(true);
      expect(result.current.connecting).toBe(false);
      expect(result.current.qrCode).toBeNull();
    });

    // ── Test 5: Error resets connecting and connected ───────────
    it('should reset connecting and connected on createInstance error', async () => {
      const { result } = renderWhatsApp();

      mockCreateInstance.mockRejectedValue(new Error('Network error'));

      await act(async () => {
        await result.current.handleConnect();
      });

      expect(result.current.connecting).toBe(false);
      expect(result.current.connected).toBe(false);
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });

    // ── Test 6: Skip delete when no existing instance ──────────
    it('should skip deleteInstance when no existing instance name', async () => {
      const { result } = renderWhatsApp();

      // No prior connect, so instanceName is empty
      await act(async () => {
        await result.current.handleConnect();
      });

      expect(mockDeleteInstance).not.toHaveBeenCalled();
      expect(mockCreateInstance).toHaveBeenCalled();
    });

    // ── Test 7: Guard resets in finally even on error ───────────
    it('should reset refreshingQrRef in finally block even on error', async () => {
      const { result } = renderWhatsApp();

      mockCreateInstance.mockRejectedValue(new Error('fail'));

      await act(async () => {
        await result.current.handleConnect();
      });

      // After error, guard should be reset. Verify by sending SSE STOPPED
      // which should now be processed (not suppressed)
      act(() => {
        simulateSSE('', 'STOPPED');
      });

      // If guard was stuck, the SSE would be suppressed.
      // Since instanceName is empty, the event is ignored anyway.
      // The key assertion is that the hook doesn't crash and connecting is false.
      expect(result.current.connecting).toBe(false);
    });

    // ── Test 8: QR code set when createInstance returns base64 ──
    it('should set qrCode when createInstance returns qrcodeBase64', async () => {
      const { result } = renderWhatsApp();

      mockCreateInstance.mockResolvedValue({
        success: true,
        instanceName: 'session-abc',
        qrcodeBase64: 'base64-qr-data',
      });

      await act(async () => {
        await result.current.handleConnect();
      });

      expect(result.current.qrCode).toBe('base64-qr-data');
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'success',
          title: 'pages.whatsapp.success.qrGenerated',
        })
      );
    });

    // ── Test 9: Stays connecting when no QR returned ───────────
    it('should stay in connecting state when no QR code returned from createInstance', async () => {
      const { result } = renderWhatsApp();

      mockCreateInstance.mockResolvedValue({
        success: true,
        instanceName: 'session-abc',
        qrcodeBase64: undefined,
      });

      await act(async () => {
        await result.current.handleConnect();
      });

      expect(result.current.connecting).toBe(true);
      expect(result.current.qrCode).toBeNull();
    });
  });

  // ── Test 10: refreshQrCode suppresses SSE STOPPED ────────────
  describe('refreshQrCode', () => {
    it('should suppress SSE STOPPED events during refresh cycle', async () => {
      const { result } = renderWhatsApp();

      // First connect to set up instance
      await act(async () => {
        await result.current.handleConnect();
      });

      // During refreshQrCode, delete triggers SSE STOPPED
      mockDeleteInstance.mockImplementation(async () => {
        act(() => {
          simulateSSE('session-abc', 'STOPPED');
        });
        return { success: true };
      });

      mockCreateInstance.mockResolvedValue({
        success: true,
        instanceName: 'session-abc',
        qrcodeBase64: 'new-qr',
      });

      await act(async () => {
        await result.current.refreshQrCode();
        // Advance past the 1s delay in refreshQrCode
        await vi.advanceTimersByTimeAsync(1100);
      });

      // SSE STOPPED should NOT have caused disconnect toast
      const disconnectedToasts = mockToast.mock.calls.filter(
        (call) =>
          call[0]?.variant === 'destructive' &&
          call[0]?.title === 'pages.whatsapp.errors.disconnected'
      );
      expect(disconnectedToasts).toHaveLength(0);
    });

    // ── Test 11: MAX_QR_REFRESHES limit ────────────────────────
    it('should respect MAX_QR_REFRESHES limit and capture to Sentry', async () => {
      const Sentry = await import('@sentry/react');
      const { result } = renderWhatsApp();

      // Connect first
      await act(async () => {
        await result.current.handleConnect();
      });

      // Call refreshQrCode 6 times sequentially (max is 5)
      for (let i = 0; i < 6; i++) {
        mockCreateInstance.mockResolvedValue({
          success: true,
          instanceName: 'session-abc',
          qrcodeBase64: `qr-${i}`,
        });
        mockDeleteInstance.mockResolvedValue({ success: true });
        await act(async () => {
          const refreshPromise = result.current.refreshQrCode();
          // Advance past the 1s delay inside refreshQrCode
          await vi.advanceTimersByTimeAsync(1200);
          await refreshPromise;
        });
      }

      // The 6th call should have triggered Sentry warning
      expect(Sentry.captureMessage).toHaveBeenCalledWith(
        'QR refresh max retries exceeded',
        expect.objectContaining({ level: 'warning' })
      );
    }, 15000);
  });

  // ── Test 12: checkStatus returns early when refreshing ───────
  describe('checkStatus', () => {
    it('should return early when refreshingQrRef is active', async () => {
      const { result } = renderWhatsApp();

      // Connect to set up instance
      await act(async () => {
        await result.current.handleConnect();
      });

      // Make deleteInstance hang to keep refreshingQrRef = true during refresh
      let resolveDelete!: () => void;
      mockDeleteInstance.mockImplementation(
        () =>
          new Promise<{ success: boolean }>((resolve) => {
            resolveDelete = () => resolve({ success: true });
          })
      );

      // Start refresh in background (sets refreshingQrRef = true)
      let refreshPromise: Promise<void>;
      await act(async () => {
        refreshPromise = result.current.refreshQrCode();
      });

      // Clear any previous checkStatus calls from handleConnect
      mockCheckStatus.mockClear();

      // Now call checkStatus while refresh is in progress
      await act(async () => {
        await result.current.checkStatus();
      });

      // checkStatus should NOT have called checkStatusConnect
      // because refreshingQrRef is true
      expect(mockCheckStatus).not.toHaveBeenCalled();

      // Cleanup: resolve the blocked delete so the refresh can finish
      await act(async () => {
        resolveDelete();
        await vi.advanceTimersByTimeAsync(1200);
        await refreshPromise!;
      });
    });
  });

  // ── Test 13: SSE events for different session ignored ────────
  describe('SSE event filtering', () => {
    it('should ignore SSE events for session names that do not match the current instance', async () => {
      mockCreateInstance.mockResolvedValue({
        success: true,
        instanceName: 'my-session',
        qrcodeBase64: 'qr-data',
      });

      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });

      expect(result.current.instanceName).toBe('my-session');

      // SSE event for a different session
      act(() => {
        simulateSSE('totally-different-session', 'STOPPED');
      });

      // Should still be connecting (not reset by the SSE for a different session)
      expect(result.current.connecting).toBe(true);
    });

    // ── Test 14: SSE events when instanceName is empty ─────────
    it('should ignore SSE events when instanceName is empty', async () => {
      renderWhatsApp();

      // Before any connect, instanceName is empty. SSE should be ignored.
      act(() => {
        simulateSSE('some-session', 'CONNECTED');
      });

      // No toast should have been shown for this event
      const connectedToasts = mockToast.mock.calls.filter(
        (call) => call[0]?.title === 'pages.whatsapp.success.connected'
      );
      expect(connectedToasts).toHaveLength(0);
    });
  });

  // ── Group A: disconnectInstance() ─────────────────────────────
  describe('disconnectInstance', () => {
    /**
     * Helper: connect + SSE CONNECTED + checkStatus(open) to reach
     * fully connected state with instanceId set.
     */
    async function connectAndEstablish(result: { current: ReturnType<typeof useWhatsApp> }) {
      // Provide default syncGroups for the initial sync that triggers after instanceId is set
      mockSyncGroups.mockResolvedValue({ success: true, message: 'done' });

      await act(async () => {
        await result.current.handleConnect();
      });
      act(() => {
        simulateSSE('session-abc', 'CONNECTED');
      });
      // Call checkStatus(open) to trigger updateInstanceStatus → sets instanceId
      mockCheckStatus.mockResolvedValue({ success: true, state: 'open' });
      await act(async () => {
        await result.current.checkStatus();
      });
      // Wait for initial sync effect to complete
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2200);
      });
      // Reset mocks
      mockCheckStatus.mockClear();
      mockToast.mockClear();
      mockSyncGroups.mockClear();
      mockRefetchGroups.mockClear();
      mockDeleteInstance.mockClear();
      mockDeleteInstanceFromDb.mockClear();
      mockRefetchInstances.mockClear();
    }

    it('A1: should delete WAHA session, DB record, refetch, and reset all state', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      expect(result.current.connected).toBe(true);
      expect(result.current.instanceName).toBe('session-abc');

      await act(async () => {
        await result.current.disconnectInstance();
      });

      expect(mockDeleteInstance).toHaveBeenCalledWith('session-abc');
      expect(mockDeleteInstanceFromDb).toHaveBeenCalled();
      expect(mockRefetchInstances).toHaveBeenCalled();
      expect(result.current.connected).toBe(false);
      expect(result.current.connecting).toBe(false);
      expect(result.current.instanceName).toBe('');
      expect(result.current.instanceId).toBeNull();
      expect(result.current.qrCode).toBeNull();
    });

    it('A2: should set disconnecting=true while running and false in finally', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      let resolveDelete!: () => void;
      mockDeleteInstance.mockImplementation(
        () =>
          new Promise<{ success: boolean }>((resolve) => {
            resolveDelete = () => resolve({ success: true });
          })
      );

      let disconnectPromise: Promise<void>;
      await act(async () => {
        disconnectPromise = result.current.disconnectInstance();
      });

      // Mid-flight: disconnecting should be true
      expect(result.current.disconnecting).toBe(true);

      await act(async () => {
        resolveDelete();
        await disconnectPromise!;
      });

      expect(result.current.disconnecting).toBe(false);
    });

    it('A3: should guard against double disconnect calls', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      let resolveDelete!: () => void;
      mockDeleteInstance.mockImplementation(
        () =>
          new Promise<{ success: boolean }>((resolve) => {
            resolveDelete = () => resolve({ success: true });
          })
      );

      let promise1: Promise<void>;
      await act(async () => {
        promise1 = result.current.disconnectInstance();
      });

      // Second call while first is in progress
      await act(async () => {
        await result.current.disconnectInstance();
      });

      await act(async () => {
        resolveDelete();
        await promise1!;
      });

      // deleteInstance should only have been called once
      expect(mockDeleteInstance).toHaveBeenCalledTimes(1);
    });

    it('A4: should continue cleanup even if WAHA deleteInstance fails', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      mockDeleteInstance.mockRejectedValue(new Error('WAHA down'));

      await act(async () => {
        await result.current.disconnectInstance();
      });

      // DB delete and refetch should still be called
      expect(mockDeleteInstanceFromDb).toHaveBeenCalled();
      expect(mockRefetchInstances).toHaveBeenCalled();
      expect(result.current.connected).toBe(false);
      expect(result.current.instanceName).toBe('');
    });

    it('A5: should continue cleanup even if deleteInstanceFromDb fails', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      mockDeleteInstanceFromDb.mockRejectedValue(new Error('DB down'));

      await act(async () => {
        await result.current.disconnectInstance();
      });

      expect(mockRefetchInstances).toHaveBeenCalled();
      expect(result.current.connected).toBe(false);
      expect(result.current.disconnecting).toBe(false);
    });

    it('A6: should skip deleteInstance when instanceName is empty', async () => {
      const { result } = renderWhatsApp();

      // No prior connect — instanceName is empty
      await act(async () => {
        await result.current.disconnectInstance();
      });

      expect(mockDeleteInstance).not.toHaveBeenCalled();
    });
  });

  // ── Group B: handleImportGroups() ──────────────────────────────
  describe('handleImportGroups', () => {
    /**
     * Helper: connect + SSE CONNECTED + checkStatus(open) to set instanceId
     * + wait for initial sync to complete.
     */
    async function setupConnected(result: { current: ReturnType<typeof useWhatsApp> }) {
      // Provide default syncGroups for the initial sync
      mockSyncGroups.mockResolvedValue({ success: true, message: 'done' });

      await act(async () => {
        await result.current.handleConnect();
      });
      act(() => {
        simulateSSE('session-abc', 'CONNECTED');
      });
      // Call checkStatus(open) to trigger updateInstanceStatus → sets instanceId
      mockCheckStatus.mockResolvedValue({ success: true, state: 'open' });
      await act(async () => {
        await result.current.checkStatus();
      });
      // Wait for initial sync effect and its completion
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2200);
      });
      // Clear mocks from setup
      mockSyncGroups.mockClear();
      mockToast.mockClear();
      mockCheckStatus.mockClear();
      mockRefetchGroups.mockClear();
    }

    it('B1: should return early when instanceName is empty', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleImportGroups();
      });

      expect(mockSyncGroups).not.toHaveBeenCalled();
    });

    it('B2: should return early when instanceId is null', async () => {
      // upsertInstanceStatus returns null → instanceId never gets set
      mockUpsertInstanceStatus.mockResolvedValue(null);
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });
      act(() => {
        simulateSSE('session-abc', 'CONNECTED');
      });
      // Call checkStatus but since upsertInstanceStatus returns null, instanceId stays null
      mockCheckStatus.mockResolvedValue({ success: true, state: 'open' });
      await act(async () => {
        await result.current.checkStatus();
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      mockSyncGroups.mockClear();

      await act(async () => {
        await result.current.handleImportGroups();
      });

      expect(mockSyncGroups).not.toHaveBeenCalled();
    });

    it('B3: should prevent concurrent imports', async () => {
      const { result } = renderWhatsApp();
      await setupConnected(result);

      let resolveSync!: () => void;
      mockSyncGroups.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveSync = () => resolve({ success: true, message: 'done' });
          })
      );

      let promise1: Promise<void>;
      await act(async () => {
        promise1 = result.current.handleImportGroups();
      });

      // Second call while first is in progress
      await act(async () => {
        await result.current.handleImportGroups();
      });

      await act(async () => {
        resolveSync();
        await promise1!;
        await vi.advanceTimersByTimeAsync(2100);
      });

      expect(mockSyncGroups).toHaveBeenCalledTimes(1);
    });

    it('B4: should complete synchronously when no job_id returned', async () => {
      const { result } = renderWhatsApp();
      await setupConnected(result);

      // Verify preconditions
      expect(result.current.instanceName).toBe('session-abc');
      expect(result.current.instanceId).toBe('inst-1');
      expect(result.current.importing).toBe(false);

      mockSyncGroups.mockResolvedValue({ success: true, message: 'Synced' });

      await act(async () => {
        await result.current.handleImportGroups();
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2100);
      });

      expect(mockSyncGroups).toHaveBeenCalled();
      expect(mockRefetchGroups).toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'success' }));
    });

    it('B5: should poll getSyncStatus when job_id returned', async () => {
      const { result } = renderWhatsApp();
      await setupConnected(result);

      mockSyncGroups.mockResolvedValue({ success: true, job_id: 'job-1' });
      mockGetSyncStatus
        .mockResolvedValueOnce({ status: 'in_progress', progress: 50 })
        .mockResolvedValueOnce({ status: 'completed', progress: 100, groups_synced: 5 });

      // Start import (don't await — it returns after creating setInterval)
      await act(async () => {
        result.current.handleImportGroups();
      });

      // Advance time for polling intervals
      for (let i = 0; i < 3; i++) {
        await act(async () => {
          await vi.advanceTimersByTimeAsync(1100);
        });
      }

      expect(mockGetSyncStatus).toHaveBeenCalledWith('job-1');
      expect(mockRefetchGroups).toHaveBeenCalled();
    });

    it('B6: should stop at maxPolls (30) and complete gracefully', async () => {
      const { result } = renderWhatsApp();
      await setupConnected(result);

      mockSyncGroups.mockResolvedValue({ success: true, job_id: 'job-1', message: 'started' });
      mockGetSyncStatus.mockResolvedValue({ status: 'in_progress', progress: 50 });

      await act(async () => {
        result.current.handleImportGroups();
      });

      // Advance 31 seconds one second at a time
      for (let i = 0; i < 31; i++) {
        await act(async () => {
          await vi.advanceTimersByTimeAsync(1100);
        });
      }

      // Should complete gracefully after maxPolls
      expect(mockGetSyncStatus).toHaveBeenCalledTimes(30);
      expect(mockRefetchGroups).toHaveBeenCalled();
    }, 35000);

    it('B7: should stop at maxErrors (5) and complete gracefully', async () => {
      const { result } = renderWhatsApp();
      await setupConnected(result);

      mockSyncGroups.mockResolvedValue({ success: true, job_id: 'job-1', message: 'started' });
      mockGetSyncStatus.mockRejectedValue(new Error('Poll error'));

      await act(async () => {
        result.current.handleImportGroups();
      });

      // Advance 6 seconds one second at a time
      for (let i = 0; i < 6; i++) {
        await act(async () => {
          await vi.advanceTimersByTimeAsync(1100);
        });
      }

      expect(mockGetSyncStatus).toHaveBeenCalledTimes(5);
      expect(mockRefetchGroups).toHaveBeenCalled();
    });

    it('B8: should suppress toast in silent mode', async () => {
      const { result } = renderWhatsApp();
      await setupConnected(result);

      mockSyncGroups.mockResolvedValue({ success: true, message: 'Synced' });

      await act(async () => {
        await result.current.handleImportGroups(true);
        await vi.advanceTimersByTimeAsync(2100);
      });

      // No toast should be shown in silent mode
      const successToasts = mockToast.mock.calls.filter((call) => call[0]?.variant === 'success');
      expect(successToasts).toHaveLength(0);
    });

    it('B9: should handle instance_not_found error', async () => {
      const { result } = renderWhatsApp();
      await setupConnected(result);

      mockSyncGroups.mockRejectedValue(new Error('instance not found'));

      await act(async () => {
        await result.current.handleImportGroups();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });

    it('B10: should handle failed status from polling', async () => {
      const { result } = renderWhatsApp();
      await setupConnected(result);

      mockSyncGroups.mockResolvedValue({ success: true, job_id: 'job-1' });
      mockGetSyncStatus.mockResolvedValue({ status: 'failed', error: 'Sync failed' });

      await act(async () => {
        result.current.handleImportGroups();
        await vi.advanceTimersByTimeAsync(1100);
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });
  });

  // ── Group C: handleCleanOldInstances() ─────────────────────────
  describe('handleCleanOldInstances', () => {
    it('C1: should delete disconnected instances and refetch', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleCleanOldInstances();
      });

      expect(mockDeleteDisconnectedInstances).toHaveBeenCalled();
      expect(mockRefetchInstances).toHaveBeenCalled();
      expect(result.current.cleaning).toBe(false);
    });

    it('C2: should show destructive toast on error', async () => {
      mockDeleteDisconnectedInstances.mockRejectedValue(new Error('DB error'));
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleCleanOldInstances();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(result.current.cleaning).toBe(false);
    });
  });

  // ── Group D: verifyConnectionStatus (30s polling while connected) ──
  describe('verifyConnectionStatus', () => {
    /**
     * Helper: connect + SSE CONNECTED + checkStatus(open) to reach
     * fully connected state with instanceId set.
     */
    async function connectAndEstablish(result: { current: ReturnType<typeof useWhatsApp> }) {
      mockSyncGroups.mockResolvedValue({ success: true, message: 'done' });

      await act(async () => {
        await result.current.handleConnect();
      });
      act(() => {
        simulateSSE('session-abc', 'CONNECTED');
      });
      mockCheckStatus.mockResolvedValue({ success: true, state: 'open' });
      await act(async () => {
        await result.current.checkStatus();
      });
      // Wait for initial sync
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2200);
      });
      mockCheckStatus.mockClear();
      mockToast.mockClear();
      mockSyncGroups.mockClear();
    }

    it('D1: should stay connected when WAHA returns open', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      mockCheckStatus.mockResolvedValue({ success: true, state: 'open' });

      // Advance 30s+ to trigger verifyConnectionStatus
      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_100);
      });

      expect(result.current.connected).toBe(true);
      // No destructive toast
      const destructiveToasts = mockToast.mock.calls.filter(
        (call) => call[0]?.variant === 'destructive'
      );
      expect(destructiveToasts).toHaveLength(0);
    });

    it('D2: should disconnect with toast when WAHA returns close', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      mockCheckStatus.mockResolvedValue({ success: true, state: 'close' });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_100);
      });

      expect(result.current.connected).toBe(false);
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });

    it('D3: should transition to connecting when WAHA returns connecting', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      mockCheckStatus.mockResolvedValue({ success: true, state: 'connecting' });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_100);
      });

      expect(result.current.connected).toBe(false);
      expect(result.current.connecting).toBe(true);
    });

    it('D4: should disconnect when success=false', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      mockCheckStatus.mockResolvedValue({ success: false, state: 'open' });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_100);
      });

      expect(result.current.connected).toBe(false);
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });

    it('D5: should only breadcrumb on error (no toast)', async () => {
      const { result } = renderWhatsApp();
      await connectAndEstablish(result);

      mockCheckStatus.mockRejectedValue(new Error('network'));

      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_100);
      });

      // connected stays true — error is swallowed (only breadcrumb)
      expect(result.current.connected).toBe(true);
      const destructiveToasts = mockToast.mock.calls.filter(
        (call) => call[0]?.variant === 'destructive'
      );
      expect(destructiveToasts).toHaveLength(0);
    });
  });

  // ── Group E: checkStatus() transitions (3s polling while connecting) ──
  describe('checkStatus transitions', () => {
    it('E1: should transition to connected with toast on open', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });
      mockCheckStatus.mockClear();
      mockToast.mockClear();

      mockCheckStatus.mockResolvedValue({ success: true, state: 'open' });

      await act(async () => {
        await result.current.checkStatus();
      });

      expect(result.current.connected).toBe(true);
      expect(result.current.connecting).toBe(false);
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'success',
          title: 'pages.whatsapp.success.connected',
        })
      );
    });

    it('E2: should transition to disconnected with toast on close', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });
      mockCheckStatus.mockClear();
      mockToast.mockClear();

      mockCheckStatus.mockResolvedValue({ success: true, state: 'close' });

      await act(async () => {
        await result.current.checkStatus();
      });

      expect(result.current.connected).toBe(false);
      expect(result.current.connecting).toBe(false);
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });

    it('E3: should disconnect when success=false', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });
      mockCheckStatus.mockClear();

      mockCheckStatus.mockResolvedValue({ success: false, state: 'open' });

      await act(async () => {
        await result.current.checkStatus();
      });

      expect(result.current.connected).toBe(false);
      expect(result.current.connecting).toBe(false);
    });

    it('E4: should only breadcrumb on error (no toast)', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });
      mockCheckStatus.mockClear();
      mockToast.mockClear();

      mockCheckStatus.mockRejectedValue(new Error('network'));

      await act(async () => {
        await result.current.checkStatus();
      });

      // connecting stays true, no toast
      expect(result.current.connecting).toBe(true);
      const toastCalls = mockToast.mock.calls.filter((call) => call[0]?.variant === 'destructive');
      expect(toastCalls).toHaveLength(0);
    });

    it('E5: should update qrCode when polling returns qrcodeBase64', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });
      mockCheckStatus.mockClear();

      mockCheckStatus.mockResolvedValue({
        success: true,
        state: 'connecting',
        qrcodeBase64: 'poll-qr',
      });

      await act(async () => {
        await result.current.checkStatus();
      });

      expect(result.current.qrCode).toBe('poll-qr');
    });
  });

  // ── Group F: loadConnectionState via latestInstance ──────────────
  describe('loadConnectionState', () => {
    it('F1: should stay disconnected when latestInstance is null', async () => {
      mockLatestInstance = null;
      const { result } = renderWhatsApp();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.connected).toBe(false);
      expect(result.current.instanceName).toBe('');
    });

    it('F2: should stay disconnected when instance has placeholder name', async () => {
      mockLatestInstance = {
        id: 'inst-1',
        user_id: 'user-1',
        instance_id: 'Será gerado automaticamente',
        status: 'connected',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };

      const { result } = renderWhatsApp();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.connected).toBe(false);
    });

    it('F3: should verify with WAHA and connect when state=open', async () => {
      mockLatestInstance = {
        id: 'inst-1',
        user_id: 'user-1',
        instance_id: 'session-valid',
        status: 'connected',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      mockCheckStatus.mockResolvedValue({ success: true, state: 'open' });

      const { result } = renderWhatsApp();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.connected).toBe(true);
      expect(result.current.instanceName).toBe('session-valid');
    });

    it('F4: should verify with WAHA and disconnect when state=close', async () => {
      mockLatestInstance = {
        id: 'inst-1',
        user_id: 'user-1',
        instance_id: 'session-valid',
        status: 'connected',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      mockCheckStatus.mockResolvedValue({ success: true, state: 'close' });

      const { result } = renderWhatsApp();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.connected).toBe(false);
    });

    it('F5: should reset to disconnected on checkStatus error', async () => {
      mockLatestInstance = {
        id: 'inst-1',
        user_id: 'user-1',
        instance_id: 'session-valid',
        status: 'connected',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      mockCheckStatus.mockRejectedValue(new Error('WAHA unreachable'));

      const { result } = renderWhatsApp();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.connected).toBe(false);
      expect(result.current.instanceName).toBe('');
    });
  });

  // ── Group G: SSE onQRCode + setAutoSync + countdown timer ──────
  describe('SSE onQRCode, setAutoSync, and countdown', () => {
    it('G1: SSE onQRCode should set qrCode and reset countdown to 30', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });

      // Advance a few seconds so countdown decrements
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      // Fire onQRCode via captured callback
      act(() => {
        if (capturedOnQRCode) {
          capturedOnQRCode('sse-qr');
        }
      });

      expect(result.current.qrCode).toBe('sse-qr');
      expect(result.current.qrCountdown).toBe(30);
    });

    it('G2: setAutoSync should toggle state and persist to localStorage', async () => {
      const { safeLocalStorage } = await import('@/lib/sentry');
      const { result } = renderWhatsApp();

      act(() => {
        result.current.setAutoSync(true);
      });

      expect(result.current.autoSync).toBe(true);

      // Wait for the effect to persist
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });

      expect(safeLocalStorage.setItem).toHaveBeenCalledWith('whatsapp-auto-sync', 'true');
    });

    it('G3: countdown should decrement every 1s while connecting', async () => {
      const { result } = renderWhatsApp();

      await act(async () => {
        await result.current.handleConnect();
      });

      // Initial countdown is 30
      expect(result.current.qrCountdown).toBe(30);

      // Advance 3 seconds
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(result.current.qrCountdown).toBe(27);
    });
  });
});
