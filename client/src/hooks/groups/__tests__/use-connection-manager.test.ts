import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useConnectionManager } from '../use-connection-manager';

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
    t: (key: string, params?: Record<string, unknown>) => {
      if (params) {
        return `${key} ${JSON.stringify(params)}`;
      }
      return key;
    },
    i18n: { language: 'pt-BR' },
  }),
}));

// Mock useInstancesConnect with mutable state
let mockLatestInstance: {
  id: string;
  user_id: string;
  instance_id: string;
  status: 'pending' | 'connected' | 'disconnected';
  created_at: string;
  updated_at: string;
} | null = {
  id: 'instance-1',
  user_id: 'user-1',
  instance_id: 'test-instance',
  status: 'connected',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

const mockUpsertInstanceStatus = vi.fn().mockResolvedValue(undefined);
const mockRefetchInstances = vi.fn().mockResolvedValue(undefined);
const mockCheckInstanceStatus = vi.fn();
const mockReconnectInstance = vi.fn();

vi.mock('@/hooks/use-instances-connect', () => ({
  useInstancesConnect: () => ({
    latestInstance: mockLatestInstance,
    upsertInstanceStatus: mockUpsertInstanceStatus,
    refetch: mockRefetchInstances,
    checkInstanceStatus: mockCheckInstanceStatus,
    reconnectInstance: mockReconnectInstance,
  }),
}));

// ============================================================
// Test Setup
// ============================================================

describe('useConnectionManager', () => {
  const mockLoadGroups = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    vi.clearAllMocks();
    mockLoadGroups.mockResolvedValue(undefined);
    mockUpsertInstanceStatus.mockResolvedValue(undefined);
    mockRefetchInstances.mockResolvedValue(undefined);
    mockCheckInstanceStatus.mockReset();
    mockReconnectInstance.mockReset();

    // Reset to connected instance by default
    mockLatestInstance = {
      id: 'instance-1',
      user_id: 'user-1',
      instance_id: 'test-instance',
      status: 'connected',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    };
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.resetAllMocks();
  });

  // ============================================================
  // Initial State Tests
  // ============================================================

  describe('initial state', () => {
    it('should have reconnecting as false', () => {
      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.reconnecting).toBe(false);
    });

    it('should have reconnectQrCode as null', () => {
      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.reconnectQrCode).toBeNull();
    });

    it('should have showReconnectDialog as false', () => {
      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.showReconnectDialog).toBe(false);
    });

    it('should provide all required return properties', () => {
      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current).toHaveProperty('connectionStatus');
      expect(result.current).toHaveProperty('instanceName');
      expect(result.current).toHaveProperty('reconnecting');
      expect(result.current).toHaveProperty('reconnectQrCode');
      expect(result.current).toHaveProperty('showReconnectDialog');
      expect(typeof result.current.setShowReconnectDialog).toBe('function');
      expect(typeof result.current.handleReconnect).toBe('function');
      expect(typeof result.current.checkConnectionStatus).toBe('function');
    });
  });

  // ============================================================
  // Connection Status Tests (based on latestInstance)
  // ============================================================

  describe('connected state', () => {
    it('should set connectionStatus to connected when instance status is connected', () => {
      mockLatestInstance = {
        id: 'instance-1',
        user_id: 'user-1',
        instance_id: 'test-instance',
        status: 'connected',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.connectionStatus).toBe('connected');
    });

    it('should set instanceName from latestInstance.instance_id when connected', () => {
      mockLatestInstance = {
        id: 'instance-1',
        user_id: 'user-1',
        instance_id: 'my-whatsapp-instance',
        status: 'connected',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.instanceName).toBe('my-whatsapp-instance');
    });
  });

  describe('disconnected state', () => {
    it('should set connectionStatus to disconnected when instance status is disconnected', () => {
      mockLatestInstance = {
        id: 'instance-1',
        user_id: 'user-1',
        instance_id: 'test-instance',
        status: 'disconnected',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.connectionStatus).toBe('disconnected');
    });

    it('should set connectionStatus to disconnected when instance status is pending', () => {
      mockLatestInstance = {
        id: 'instance-1',
        user_id: 'user-1',
        instance_id: 'test-instance',
        status: 'pending',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.connectionStatus).toBe('disconnected');
    });

    it('should set connectionStatus to disconnected and empty instanceName when no latestInstance', () => {
      mockLatestInstance = null;

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.connectionStatus).toBe('disconnected');
      expect(result.current.instanceName).toBe('');
    });

    it('should set instanceName even when disconnected', () => {
      mockLatestInstance = {
        id: 'instance-1',
        user_id: 'user-1',
        instance_id: 'test-instance',
        status: 'disconnected',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.instanceName).toBe('test-instance');
    });
  });

  // ============================================================
  // Reconnection Flow Tests
  // ============================================================

  describe('handleReconnect', () => {
    it('should show error toast when instanceName is empty', async () => {
      mockLatestInstance = null;

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'common.error',
        description: 'pages.groups.errors.instanceNotFound',
      });

      expect(mockReconnectInstance).not.toHaveBeenCalled();
    });

    it('should call reconnectInstance with the instanceName', async () => {
      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,iVBORw0KGgo=',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(mockReconnectInstance).toHaveBeenCalledWith('test-instance');
    });

    it('should set reconnectQrCode when QR code is returned', async () => {
      vi.useFakeTimers();

      const expectedQrCode = 'data:image/png;base64,iVBORw0KGgo=';
      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: expectedQrCode,
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(result.current.reconnectQrCode).toBe(expectedQrCode);
      expect(result.current.showReconnectDialog).toBe(true);

      vi.useRealTimers();
    });

    it('should show QR generated toast on successful reconnect', async () => {
      vi.useFakeTimers();

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.whatsapp.qrGenerated',
        description: 'pages.groups.whatsapp.scanQRCode',
      });

      vi.useRealTimers();
    });

    it('should handle reconnect failure when success is false', async () => {
      mockReconnectInstance.mockResolvedValue({
        success: false,
        error: 'Instance not available',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.reconnect',
        description: 'Instance not available',
      });

      expect(result.current.reconnecting).toBe(false);
    });

    it('should handle reconnect failure when qrcodeBase64 is empty', async () => {
      // Empty string is falsy, so `data.qrcodeBase64` check in
      // `if (data.success && data.qrcodeBase64)` fails, hitting the outer else branch
      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: '',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.reconnect',
        description: 'pages.groups.errors.qrGeneration',
      });
    });

    it('should handle reconnectInstance throwing an error', async () => {
      mockReconnectInstance.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.reconnect',
        description: 'Network error',
      });

      expect(result.current.reconnecting).toBe(false);
    });

    it('should set reconnecting to false in finally block even on error', async () => {
      mockReconnectInstance.mockRejectedValue(new Error('Something went wrong'));

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(result.current.reconnecting).toBe(false);
    });

    it('should handle non-Error thrown values', async () => {
      mockReconnectInstance.mockRejectedValue('string error');

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.reconnect',
        description: 'pages.groups.errors.reconnect',
      });
    });
  });

  // ============================================================
  // Status Polling During Reconnection Tests
  // ============================================================

  describe('status polling during reconnection', () => {
    it('should start polling every 3 seconds after QR code is generated', async () => {
      vi.useFakeTimers();

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockResolvedValue({
        success: false,
        state: 'closed',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      // No polling call yet at time 0
      expect(mockCheckInstanceStatus).not.toHaveBeenCalled();

      // Advance 3 seconds - first poll
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockCheckInstanceStatus).toHaveBeenCalledTimes(1);
      expect(mockCheckInstanceStatus).toHaveBeenCalledWith('test-instance');

      // Advance another 3 seconds - second poll
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockCheckInstanceStatus).toHaveBeenCalledTimes(2);

      vi.useRealTimers();
    });

    it('should stop polling and set connected when status returns open', async () => {
      vi.useFakeTimers();

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockResolvedValue({
        success: true,
        state: 'open',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      // Advance to trigger first poll
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(result.current.connectionStatus).toBe('connected');
      expect(result.current.showReconnectDialog).toBe(false);
      expect(result.current.reconnectQrCode).toBeNull();
      expect(result.current.reconnecting).toBe(false);

      // Verify upsertInstanceStatus was called with connected
      expect(mockUpsertInstanceStatus).toHaveBeenCalledWith('test-instance', 'connected');

      // Verify refetch was called
      expect(mockRefetchInstances).toHaveBeenCalled();

      vi.useRealTimers();
    });

    it('should show success toast when reconnection completes', async () => {
      vi.useFakeTimers();

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockResolvedValue({
        success: true,
        state: 'open',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.reconnected',
        description: 'pages.groups.success.reconnectedMessage',
      });

      vi.useRealTimers();
    });

    it('should call loadGroups after 1 second delay when reconnected', async () => {
      vi.useFakeTimers();

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockResolvedValue({
        success: true,
        state: 'open',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      // Trigger poll that detects connected status
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockLoadGroups).not.toHaveBeenCalled();

      // Advance 1 second for the delayed loadGroups call
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000);
      });

      expect(mockLoadGroups).toHaveBeenCalledTimes(1);

      vi.useRealTimers();
    });

    it('should continue polling when status is not open', async () => {
      vi.useFakeTimers();

      // Use disconnected instance so connectionStatus starts as disconnected
      mockLatestInstance = {
        id: 'instance-1',
        user_id: 'user-1',
        instance_id: 'test-instance',
        status: 'disconnected',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockResolvedValue({
        success: true,
        state: 'closed',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.connectionStatus).toBe('disconnected');

      await act(async () => {
        await result.current.handleReconnect();
      });

      // First poll - not open, should remain disconnected
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockCheckInstanceStatus).toHaveBeenCalledTimes(1);
      expect(result.current.connectionStatus).toBe('disconnected');

      // Second poll - still not open
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockCheckInstanceStatus).toHaveBeenCalledTimes(2);

      // Third poll - still not open
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockCheckInstanceStatus).toHaveBeenCalledTimes(3);

      vi.useRealTimers();
    });

    it('should ignore polling errors silently', async () => {
      vi.useFakeTimers();

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockRejectedValue(new Error('Network failure'));

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      // Advance to trigger poll - should not throw
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      // Should continue to poll on next interval despite error
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockCheckInstanceStatus).toHaveBeenCalledTimes(2);

      vi.useRealTimers();
    });
  });

  // ============================================================
  // Timeout Handling (120s deadline)
  // ============================================================

  describe('timeout handling (2 minute deadline)', () => {
    it('should stop polling after 120 seconds', async () => {
      vi.useFakeTimers();

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      // Always return not-open status
      mockCheckInstanceStatus.mockResolvedValue({
        success: true,
        state: 'closed',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      // Advance slightly past 120 seconds to trigger the timeout
      await act(async () => {
        await vi.advanceTimersByTimeAsync(121000);
      });

      const callCountAtTimeout = mockCheckInstanceStatus.mock.calls.length;

      // Advance more time - polling should have stopped
      await act(async () => {
        await vi.advanceTimersByTimeAsync(10000);
      });

      // No new calls should have been made after the timeout
      expect(mockCheckInstanceStatus.mock.calls.length).toBe(callCountAtTimeout);

      vi.useRealTimers();
    });

    it('should clear poll interval when timeout fires', async () => {
      vi.useFakeTimers();

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockResolvedValue({
        success: true,
        state: 'closed',
      });

      const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval');

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      // Advance to trigger timeout at 120s
      await act(async () => {
        await vi.advanceTimersByTimeAsync(120000);
      });

      // clearInterval should have been called for the poll
      expect(clearIntervalSpy).toHaveBeenCalled();

      clearIntervalSpy.mockRestore();
      vi.useRealTimers();
    });
  });

  // ============================================================
  // Status Transition Tests
  // ============================================================

  describe('status transitions', () => {
    it('should transition from connected to disconnected when instance changes', () => {
      mockLatestInstance = {
        id: 'instance-1',
        user_id: 'user-1',
        instance_id: 'test-instance',
        status: 'connected',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };

      const { result, rerender } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.connectionStatus).toBe('connected');

      // Change mock to disconnected and rerender
      mockLatestInstance = {
        ...mockLatestInstance,
        status: 'disconnected',
      };

      rerender();

      // The effect re-runs because latestInstance changes via the mock
      // Due to how vi.mock works, the new value is picked up on rerender
      expect(result.current.connectionStatus).toBe('disconnected');
    });

    it('should transition from disconnected to connected via reconnection polling', async () => {
      vi.useFakeTimers();

      mockLatestInstance = {
        id: 'instance-1',
        user_id: 'user-1',
        instance_id: 'test-instance',
        status: 'disconnected',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      // First poll returns closed, second returns open
      mockCheckInstanceStatus
        .mockResolvedValueOnce({ success: true, state: 'closed' })
        .mockResolvedValueOnce({ success: true, state: 'open' });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.connectionStatus).toBe('disconnected');

      // Start reconnection
      await act(async () => {
        await result.current.handleReconnect();
      });

      // First poll - still closed
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      // connectionStatus may not have changed yet since state was 'closed'

      // Second poll - open
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(result.current.connectionStatus).toBe('connected');

      vi.useRealTimers();
    });
  });

  // ============================================================
  // Cleanup on Unmount Tests
  // ============================================================

  describe('cleanup on unmount', () => {
    it('should clear intervals on unmount', async () => {
      vi.useFakeTimers();

      const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval');
      const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout');

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockResolvedValue({
        success: true,
        state: 'closed',
      });

      const { result, unmount } = renderHook(() => useConnectionManager(mockLoadGroups));

      // Start reconnection to create intervals
      await act(async () => {
        await result.current.handleReconnect();
      });

      // Unmount - should clear both interval and timeout
      unmount();

      expect(clearIntervalSpy).toHaveBeenCalled();
      expect(clearTimeoutSpy).toHaveBeenCalled();

      clearIntervalSpy.mockRestore();
      clearTimeoutSpy.mockRestore();
      vi.useRealTimers();
    });

    it('should not leak intervals after unmount', async () => {
      vi.useFakeTimers();

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockResolvedValue({
        success: true,
        state: 'closed',
      });

      const { result, unmount } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      // Trigger one poll to confirm it is active
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      const callCountBeforeUnmount = mockCheckInstanceStatus.mock.calls.length;

      unmount();

      // Advance time after unmount
      await act(async () => {
        await vi.advanceTimersByTimeAsync(10000);
      });

      // No new calls should have been made
      expect(mockCheckInstanceStatus.mock.calls.length).toBe(callCountBeforeUnmount);

      vi.useRealTimers();
    });

    it('should handle unmount when no intervals are active', () => {
      const { unmount } = renderHook(() => useConnectionManager(mockLoadGroups));

      // Should not throw when unmounting without active intervals
      expect(() => unmount()).not.toThrow();
    });
  });

  // ============================================================
  // setShowReconnectDialog Tests
  // ============================================================

  describe('setShowReconnectDialog', () => {
    it('should update showReconnectDialog state', () => {
      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      expect(result.current.showReconnectDialog).toBe(false);

      act(() => {
        result.current.setShowReconnectDialog(true);
      });

      expect(result.current.showReconnectDialog).toBe(true);

      act(() => {
        result.current.setShowReconnectDialog(false);
      });

      expect(result.current.showReconnectDialog).toBe(false);
    });
  });

  // ============================================================
  // checkConnectionStatus manual call Tests
  // ============================================================

  describe('checkConnectionStatus', () => {
    it('should be callable to manually recheck connection', async () => {
      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.checkConnectionStatus();
      });

      // After manual check with connected instance, status should reflect it
      expect(result.current.connectionStatus).toBe('connected');
    });
  });

  // ============================================================
  // Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should clear existing intervals before starting new ones on repeated handleReconnect', async () => {
      vi.useFakeTimers();

      const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval');

      mockReconnectInstance.mockResolvedValue({
        success: true,
        qrcodeBase64: 'data:image/png;base64,abc123',
      });

      mockCheckInstanceStatus.mockResolvedValue({
        success: true,
        state: 'closed',
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      // First reconnect attempt
      await act(async () => {
        await result.current.handleReconnect();
      });

      // Advance a bit
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      const clearCallsAfterFirst = clearIntervalSpy.mock.calls.length;

      // Second reconnect attempt - should clear previous intervals
      await act(async () => {
        await result.current.handleReconnect();
      });

      // clearInterval should have been called for the previous poll
      expect(clearIntervalSpy.mock.calls.length).toBeGreaterThan(clearCallsAfterFirst);

      clearIntervalSpy.mockRestore();
      vi.useRealTimers();
    });

    it('should handle reconnect when success is false with no error message', async () => {
      mockReconnectInstance.mockResolvedValue({
        success: false,
      });

      const { result } = renderHook(() => useConnectionManager(mockLoadGroups));

      await act(async () => {
        await result.current.handleReconnect();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.reconnect',
        description: 'pages.groups.errors.qrGeneration',
      });
    });

    it('should maintain stability across rerenders', () => {
      const { result, rerender } = renderHook(() => useConnectionManager(mockLoadGroups));

      const initialStatus = result.current.connectionStatus;
      const initialInstanceName = result.current.instanceName;

      rerender();

      expect(result.current.connectionStatus).toBe(initialStatus);
      expect(result.current.instanceName).toBe(initialInstanceName);
    });
  });
});
