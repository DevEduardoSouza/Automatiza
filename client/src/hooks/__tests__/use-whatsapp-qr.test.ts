import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useWhatsApp } from '../use-whatsapp';

// ============================================================
// Mock Modules (identical to use-whatsapp.test.ts)
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
// Mock: useInstancesConnect
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
    return lower !== 'sera gerado automaticamente' && lower !== 'gerado automaticamente';
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
// Tests: QR Code Scenarios
// ============================================================

describe('useWhatsApp — QR code scenarios', () => {
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

  // ── QR1: Countdown reaches 0 triggers auto-refresh ──────────
  it('QR1: countdown reaching 0 should trigger refreshQrCode automatically', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    expect(result.current.connecting).toBe(true);
    expect(result.current.qrCountdown).toBe(30);

    // Clear mocks to isolate the auto-refresh call
    mockDeleteInstance.mockClear();
    mockCreateInstance.mockClear();
    mockCreateInstance.mockResolvedValue({
      success: true,
      instanceName: 'session-abc',
      qrcodeBase64: 'refreshed-qr',
    });

    // Advance 30 seconds so countdown reaches 0 and triggers refresh
    for (let i = 0; i < 30; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000);
      });
    }

    // The countdown effect calls countdownRefreshRef.current (= refreshQrCode)
    // which does deleteInstance + createInstance
    // Allow the 1s delay inside refreshQrCode to complete
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1200);
    });

    expect(mockDeleteInstance).toHaveBeenCalled();
    expect(mockCreateInstance).toHaveBeenCalled();
  }, 15000);

  // ── QR2: Countdown stops when SSE CONNECTED fires ──────────
  it('QR2: countdown should stop when SSE CONNECTED fires', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    expect(result.current.connecting).toBe(true);
    expect(result.current.qrCountdown).toBe(30);

    // Advance 5 seconds (countdown should be ~25)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });

    expect(result.current.qrCountdown).toBe(25);

    // Fire SSE CONNECTED
    act(() => {
      simulateSSE('session-abc', 'CONNECTED');
    });

    expect(result.current.connected).toBe(true);
    expect(result.current.connecting).toBe(false);

    // Capture the countdown value at the moment of connection
    const countdownAtConnection = result.current.qrCountdown;

    // Advance more time — countdown should NOT decrement further
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });

    // Countdown should remain the same (interval was cleared)
    expect(result.current.qrCountdown).toBe(countdownAtConnection);
  });

  // ── QR3: SSE onQRCode resets countdown to 30 ───────────────
  it('QR3: SSE onQRCode should reset countdown to 30', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    // Advance 5 seconds so countdown decrements
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });

    expect(result.current.qrCountdown).toBe(25);

    // Fire onQRCode via captured callback
    act(() => {
      if (capturedOnQRCode) {
        capturedOnQRCode('new-sse-qr');
      }
    });

    expect(result.current.qrCode).toBe('new-sse-qr');
    expect(result.current.qrCountdown).toBe(30);
  });

  // ── QR4: refreshQrCode with createInstance returning no QR ──
  it('QR4: refreshQrCode should stay connecting with null qrCode when no qrcodeBase64 returned', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    // Setup: createInstance returns success but no QR
    mockCreateInstance.mockResolvedValue({
      success: true,
      instanceName: 'session-abc',
      qrcodeBase64: undefined,
    });

    await act(async () => {
      const refreshPromise = result.current.refreshQrCode();
      await vi.advanceTimersByTimeAsync(1200);
      await refreshPromise;
    });

    // Should stay connecting with null QR (refreshQrCode sets connecting=true in finally)
    expect(result.current.connecting).toBe(true);
    // QR was set to null at the start of refreshQrCode, and no new QR was returned
    expect(result.current.qrCode).toBeNull();
  });

  // ── QR5: refreshQrCode with createInstance error ────────────
  it('QR5: refreshQrCode should show destructive toast when createInstance rejects', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    mockToast.mockClear();
    mockCreateInstance.mockRejectedValue(new Error('WAHA unavailable'));

    await act(async () => {
      const refreshPromise = result.current.refreshQrCode();
      await vi.advanceTimersByTimeAsync(1200);
      await refreshPromise;
    });

    // Should show destructive toast
    expect(mockToast).toHaveBeenCalledWith(
      expect.objectContaining({
        variant: 'destructive',
        title: 'pages.whatsapp.errors.qrRefreshFailed',
      })
    );

    // refreshQrCode sets connecting=true in the finally block
    expect(result.current.connecting).toBe(true);
  });

  // ── QR6: refreshQrCode guard prevents concurrent calls ─────
  it('QR6: refreshQrCode should prevent concurrent calls via refreshingQrRef guard', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    mockDeleteInstance.mockClear();

    // Make deleteInstance hang so refreshingQrRef stays true
    let resolveDelete!: () => void;
    mockDeleteInstance.mockImplementation(
      () =>
        new Promise<{ success: boolean }>((resolve) => {
          resolveDelete = () => resolve({ success: true });
        })
    );

    // Start first refresh (sets refreshingQrRef = true)
    let refreshPromise1: Promise<void>;
    await act(async () => {
      refreshPromise1 = result.current.refreshQrCode();
    });

    // Attempt second refresh while first is in progress
    await act(async () => {
      await result.current.refreshQrCode();
    });

    // Only one deleteInstance call (second was blocked by guard)
    expect(mockDeleteInstance).toHaveBeenCalledTimes(1);

    // Cleanup
    await act(async () => {
      resolveDelete();
      await vi.advanceTimersByTimeAsync(1200);
      await refreshPromise1!;
    });
  });

  // ── QR7: QR code updated via polling checkStatus ────────────
  it('QR7: checkStatus should update qrCode when polling returns qrcodeBase64', async () => {
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

  // ── QR8: QR cleared when SSE CONNECTED fires ───────────────
  it('QR8: qrCode should be cleared when SSE CONNECTED fires', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    // Verify QR is set from handleConnect
    expect(result.current.qrCode).toBe('base64-qr-data');

    // Fire SSE CONNECTED
    act(() => {
      simulateSSE('session-abc', 'CONNECTED');
    });

    expect(result.current.connected).toBe(true);
    expect(result.current.qrCode).toBeNull();
  });

  // ── QR9: QR cleared when SSE STOPPED fires (no refresh guard) ──
  it('QR9: qrCode should be cleared and state reset when SSE STOPPED fires for current session', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    // Verify QR is set
    expect(result.current.qrCode).toBe('base64-qr-data');
    expect(result.current.connecting).toBe(true);

    // Fire SSE STOPPED for the matching session (refreshingQrRef is false after handleConnect's finally)
    act(() => {
      simulateSSE('session-abc', 'STOPPED');
    });

    // SSE STOPPED should clear QR and reset state
    expect(result.current.qrCode).toBeNull();
    expect(result.current.connected).toBe(false);
    expect(result.current.connecting).toBe(false);
  });

  // ── QR10: checkStatus returns qrcodeBase64 updates qrCode ──
  it('QR10: checkStatus detecting open state should set connected=true and connecting=false', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    mockCheckStatus.mockClear();
    mockToast.mockClear();

    mockCheckStatus.mockResolvedValue({
      success: true,
      state: 'open',
    });

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

  // ── QR11: Max refresh retries captures to Sentry ────────────
  it('QR11: should capture to Sentry when max QR refresh retries exceeded', async () => {
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

  // ── QR12: fetchQrCode (SSE SCAN_QR_CODE) detects open state ──
  it('QR12: fetchQrCode via SSE SCAN_QR_CODE should detect open state and transition to connected', async () => {
    const { result } = renderWhatsApp();

    await act(async () => {
      await result.current.handleConnect();
    });

    expect(result.current.connecting).toBe(true);

    // Mock checkStatus to return open (fetchQrCode uses checkStatusConnect internally)
    mockCheckStatus.mockResolvedValue({
      success: true,
      state: 'open',
      qrcodeBase64: undefined,
    });

    // Fire SSE SCAN_QR_CODE which triggers refreshQrCodeRef.current (= fetchQrCode)
    await act(async () => {
      simulateSSE('session-abc', 'SCAN_QR_CODE');
      // Allow the async fetchQrCode to complete
      await vi.advanceTimersByTimeAsync(100);
    });

    expect(result.current.connected).toBe(true);
    expect(result.current.connecting).toBe(false);
    expect(result.current.qrCode).toBeNull();
  });
});
