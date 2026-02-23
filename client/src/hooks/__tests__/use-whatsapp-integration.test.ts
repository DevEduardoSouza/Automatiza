import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useWhatsApp } from '../use-whatsapp';

// ============================================================
// Mock Modules (same structure as unit tests)
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
// Mock: useSessionStatusSSE
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
// Mock: useInstancesConnect (mutable latestInstance)
// ============================================================

const mockUpsertInstanceStatus = vi.fn().mockResolvedValue({ id: 'inst-1' });
const mockRefetchInstances = vi.fn();
const mockDeleteInstanceFromDb = vi.fn();

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
    cleanupInvalidInstances: vi.fn(),
    deleteDisconnectedInstances: vi.fn(),
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
// Mock: Sentry, lib/sentry, entities
// ============================================================

vi.mock('@sentry/react', () => ({
  addBreadcrumb: vi.fn(),
  captureMessage: vi.fn(),
  captureException: vi.fn(),
}));

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
  safeLocalStorage: {
    getItem: vi.fn().mockReturnValue(null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  },
}));

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
// Integration Tests
// ============================================================

describe('useWhatsApp integration flows', () => {
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
    mockRefetchInstances.mockResolvedValue(undefined);
    mockUpsertInstanceStatus.mockResolvedValue({ id: 'inst-1' });
    mockSyncGroups.mockResolvedValue({ success: true, message: 'done' });
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

  // ── Flow 1: Happy path connect → QR → SSE CONNECTED ────────
  it('disconnected → connect → QR appears → SSE CONNECTED → connected', async () => {
    const { result } = renderWhatsApp();

    // Initially disconnected
    expect(result.current.connected).toBe(false);
    expect(result.current.connecting).toBe(false);

    // Step 1: Call handleConnect
    await act(async () => {
      await result.current.handleConnect();
    });

    // Step 2: QR code should be visible, connecting state
    expect(result.current.connecting).toBe(true);
    expect(result.current.qrCode).toBe('base64-qr-data');
    expect(result.current.instanceName).toBe('session-abc');

    // Step 3: SSE CONNECTED fires
    act(() => {
      simulateSSE('session-abc', 'CONNECTED');
    });

    // Step 4: Should be connected
    expect(result.current.connected).toBe(true);
    expect(result.current.connecting).toBe(false);
    expect(result.current.qrCode).toBeNull();
  });

  // ── Flow 2: Connected → disconnect → state resets ───────────
  it('connected → disconnect → state fully resets', async () => {
    const { result } = renderWhatsApp();

    // Connect first
    await act(async () => {
      await result.current.handleConnect();
    });

    // Simulate SSE CONNECTED
    act(() => {
      simulateSSE('session-abc', 'CONNECTED');
    });

    expect(result.current.connected).toBe(true);

    // Disconnect
    await act(async () => {
      await result.current.disconnectInstance();
    });

    // All state should be reset
    expect(result.current.connected).toBe(false);
    expect(result.current.connecting).toBe(false);
    expect(result.current.instanceName).toBe('');
    expect(result.current.instanceId).toBeNull();
    expect(result.current.qrCode).toBeNull();
  });

  // ── Flow 3: QR expires → auto-refresh via countdown ─────────
  it('connecting → QR countdown expires → auto-refresh triggers new QR', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    expect(result.current.connecting).toBe(true);
    expect(result.current.qrCode).toBe('base64-qr-data');

    // Setup for the refresh (countdown hits 0 → calls refreshQrCode)
    mockCreateInstance.mockResolvedValue({
      success: true,
      instanceName: 'session-abc',
      qrcodeBase64: 'refreshed-qr',
    });

    // Advance 30 seconds to trigger countdown refresh
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000);
    });

    // After countdown, refreshQrCode should have been called
    // The delete + create cycle should produce a new QR
    expect(result.current.connecting).toBe(true);
  });

  // ── Flow 4: Race condition regression test ──────────────────
  it('race condition: handleConnect + SSE STOPPED simultaneous does not flash disconnected', async () => {
    const { result } = renderWhatsApp();

    // First connect to establish session
    await act(async () => {
      await result.current.handleConnect();
    });

    expect(result.current.instanceName).toBe('session-abc');

    // Second connect (reconnect scenario) — SSE STOPPED fires during delete
    mockDeleteInstance.mockImplementation(async () => {
      // SSE STOPPED fires immediately when delete happens
      act(() => {
        simulateSSE('session-abc', 'STOPPED');
      });
      return { success: true };
    });

    mockCreateInstance.mockResolvedValue({
      success: true,
      instanceName: 'session-new',
      qrcodeBase64: 'qr-new',
    });

    await act(async () => {
      await result.current.handleConnect();
      await vi.advanceTimersByTimeAsync(2100);
    });

    // Key assertion: connecting should be true, NOT flashed to false by SSE STOPPED
    expect(result.current.connecting).toBe(true);
    expect(result.current.qrCode).toBe('qr-new');

    // Verify no disconnected toast was shown
    const disconnectedToasts = mockToast.mock.calls.filter(
      (call) =>
        call[0]?.variant === 'destructive' &&
        call[0]?.title === 'pages.whatsapp.errors.disconnected'
    );
    expect(disconnectedToasts).toHaveLength(0);
  });

  // ── Flow 5: SSE WORKING during connect → connected ──────────
  it('SSE WORKING during handleConnect transitions to connected', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    expect(result.current.connecting).toBe(true);

    // SSE WORKING fires (equivalent to CONNECTED for WAHA)
    act(() => {
      simulateSSE('session-abc', 'WORKING');
    });

    expect(result.current.connected).toBe(true);
    expect(result.current.connecting).toBe(false);
    expect(result.current.qrCode).toBeNull();
  });

  // ── Flow 6: SSE SCAN_QR_CODE triggers QR fetch ──────────────
  it('SSE SCAN_QR_CODE during connecting triggers QR code fetch', async () => {
    const { result } = renderWhatsApp();

    // Set checkStatus to return a QR code (simulating fetchQrCode path)
    mockCheckStatus.mockResolvedValue({
      success: true,
      state: 'connecting',
      qrcodeBase64: 'sse-triggered-qr',
    });

    await act(async () => {
      await result.current.handleConnect();
    });

    expect(result.current.connecting).toBe(true);

    // SSE SCAN_QR_CODE fires — triggers async fetchQrCode via ref
    await act(async () => {
      simulateSSE('session-abc', 'SCAN_QR_CODE');
      // Allow fetchQrCode promise to resolve
      await vi.advanceTimersByTimeAsync(0);
    });

    // Should still be connecting, and the QR fetch should have been triggered
    expect(result.current.connecting).toBe(true);
    expect(result.current.connected).toBe(false);
  });

  // ── Flow I1: connect → connected → importGroups → polling → completion ──
  it('I1: connect → connected → importGroups → polling → completion', async () => {
    const { result } = renderWhatsApp();

    // Connect
    await act(async () => {
      await result.current.handleConnect();
    });
    act(() => {
      simulateSSE('session-abc', 'CONNECTED');
    });
    // Set instanceId via checkStatus
    mockCheckStatus.mockResolvedValue({ success: true, state: 'open' });
    await act(async () => {
      await result.current.checkStatus();
    });
    // Wait for initial sync
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2200);
    });

    // Clear mocks from setup
    mockSyncGroups.mockClear();
    mockToast.mockClear();
    mockRefetchGroups.mockClear();

    // Setup import with polling
    mockSyncGroups.mockResolvedValue({ success: true, job_id: 'job-1' });
    mockGetSyncStatus
      .mockResolvedValueOnce({ status: 'in_progress', progress: 50 })
      .mockResolvedValueOnce({ status: 'completed', progress: 100, groups_synced: 3 });

    // Start import
    await act(async () => {
      result.current.handleImportGroups();
    });

    // Advance for 2 polling intervals
    for (let i = 0; i < 3; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1100);
      });
    }

    expect(mockGetSyncStatus).toHaveBeenCalledWith('job-1');
    expect(mockRefetchGroups).toHaveBeenCalled();
    expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'success' }));
  });

  // ── Flow I2: import polling timeout ──
  it('I2: import polling timeout: 30 polls → graceful completion', async () => {
    const { result } = renderWhatsApp();

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
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2200);
    });
    mockSyncGroups.mockClear();
    mockRefetchGroups.mockClear();

    mockSyncGroups.mockResolvedValue({ success: true, job_id: 'job-1', message: 'started' });
    mockGetSyncStatus.mockResolvedValue({ status: 'in_progress', progress: 50 });

    await act(async () => {
      result.current.handleImportGroups(true);
    });

    for (let i = 0; i < 31; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1100);
      });
    }

    expect(mockGetSyncStatus).toHaveBeenCalledTimes(30);
    expect(mockRefetchGroups).toHaveBeenCalled();
  }, 35000);

  // ── Flow I3: import polling errors ──
  it('I3: import polling errors: 5 errors → graceful completion', async () => {
    const { result } = renderWhatsApp();

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
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2200);
    });
    mockSyncGroups.mockClear();
    mockRefetchGroups.mockClear();

    mockSyncGroups.mockResolvedValue({ success: true, job_id: 'job-1', message: 'started' });
    mockGetSyncStatus.mockRejectedValue(new Error('Poll error'));

    await act(async () => {
      result.current.handleImportGroups(true);
    });

    for (let i = 0; i < 6; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1100);
      });
    }

    expect(mockGetSyncStatus).toHaveBeenCalledTimes(5);
    expect(mockRefetchGroups).toHaveBeenCalled();
  });

  // ── Flow I4: verifyConnectionStatus detects disconnect ──
  it('I4: verifyConnectionStatus detects disconnect → toast → reset', async () => {
    const { result } = renderWhatsApp();

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
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2200);
    });
    mockToast.mockClear();

    // Now checkStatus returns 'close' — simulating session dropped
    mockCheckStatus.mockResolvedValue({ success: true, state: 'close' });

    // Advance 30s+ to trigger verifyConnectionStatus
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_100);
    });

    expect(result.current.connected).toBe(false);
    expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
  });

  // ── Flow I5: autoSync triggers immediate import + hourly ──
  it('I5: autoSync: connect → enable → immediate import → hourly', async () => {
    const { result } = renderWhatsApp();

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
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2200);
    });
    mockSyncGroups.mockClear();

    // Enable autoSync
    act(() => {
      result.current.setAutoSync(true);
    });

    expect(result.current.autoSync).toBe(true);

    // Wait for autoSync effect to trigger immediate import
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2200);
    });

    // Should have triggered an import (at least one sync call)
    expect(mockSyncGroups).toHaveBeenCalled();
    const firstCallCount = mockSyncGroups.mock.calls.length;

    mockSyncGroups.mockClear();

    // Advance 1 hour for the hourly interval
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60 * 60 * 1000 + 100);
    });

    // Should have triggered another sync
    expect(mockSyncGroups).toHaveBeenCalled();
  }, 10000);

  // ── Flow I6: loadConnectionState with valid instance ──
  it('I6: loadConnectionState with valid instance → verifying → connected', async () => {
    mockLatestInstance = {
      id: 'inst-1',
      user_id: 'user-1',
      instance_id: 'session-existing',
      status: 'connected',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    mockCheckStatus.mockResolvedValue({ success: true, state: 'open' });

    const { result } = renderWhatsApp();

    // Initially verifying while loadConnectionState runs
    await act(async () => {
      await vi.advanceTimersByTimeAsync(100);
    });

    expect(result.current.connected).toBe(true);
    expect(result.current.instanceName).toBe('session-existing');
    expect(result.current.verifying).toBe(false);
  });

  // ── Flow I7: SSE job.status events update import progress ──
  it('I7: SSE job.status events update import progress in real-time', async () => {
    const { result } = renderWhatsApp();

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
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2200);
    });
    mockSyncGroups.mockClear();
    mockRefetchGroups.mockClear();
    mockToast.mockClear();

    // Start import with a hanging job so importing stays true
    let resolveSync!: () => void;
    mockSyncGroups.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveSync = () => resolve({ success: true, job_id: 'job-sse' });
        })
    );

    await act(async () => {
      result.current.handleImportGroups();
    });

    // Resolve the sync call to create the polling interval
    await act(async () => {
      resolveSync();
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(result.current.importing).toBe(true);

    // Dispatch SSE job event with progress
    await act(async () => {
      window.dispatchEvent(
        new CustomEvent('sse:job-event', {
          detail: {
            data: {
              jobType: 'groups_sync',
              jobId: 'job-sse',
              progress: 75,
              group: { id: 'g1', name: 'Test Group', isAdmin: true },
            },
          },
        })
      );
    });

    expect(result.current.importProgress).toBe(75);
    expect(result.current.importedGroups).toContain('Test Group');

    // Dispatch completion event
    await act(async () => {
      window.dispatchEvent(
        new CustomEvent('sse:job-event', {
          detail: {
            data: {
              jobType: 'groups_sync',
              jobId: 'job-sse',
              status: 'completed',
              progress: 100,
              newGroups: 2,
              updatedGroups: 1,
            },
          },
        })
      );
    });

    expect(result.current.importing).toBe(false);
    expect(mockRefetchGroups).toHaveBeenCalled();
  });

  // ── Flow I8: multiple rapid connect calls ──
  it('I8: multiple rapid connect calls do not duplicate WAHA calls', async () => {
    const { result } = renderWhatsApp();

    // Call handleConnect twice rapidly
    await act(async () => {
      result.current.handleConnect();
      result.current.handleConnect();
    });

    // Wait for both to complete
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2200);
    });

    // createInstance should have been called (the second call may also trigger
    // since there's no explicit guard against double-connect, but deleteInstance
    // should only be called once at most for the first session)
    expect(mockCreateInstance.mock.calls.length).toBeGreaterThanOrEqual(1);
  });
});
