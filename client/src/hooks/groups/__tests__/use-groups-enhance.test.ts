import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsEnhance } from '../use-groups-enhance';

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

const mockEnhanceMessage = vi.fn();
let mockIsEnhancingApi = false;

vi.mock('@/hooks/use-ai-tools-connect', () => ({
  useAiToolsConnect: () => ({
    enhanceMessage: mockEnhanceMessage,
    isEnhancing: mockIsEnhancingApi,
    analyzeEngagement: vi.fn(),
    analyzeTrends: vi.fn(),
    suggestOfferText: vi.fn(),
    isAnalyzingEngagement: false,
    isAnalyzingTrends: false,
    isSuggesting: false,
  }),
}));

const mockCaptureHookError = vi.fn();

vi.mock('@/lib/sentry', () => ({
  captureHookError: (...args: unknown[]) => mockCaptureHookError(...args),
}));

// ============================================================
// Test Setup
// ============================================================

describe('useGroupsEnhance', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsEnhancingApi = false;
    mockEnhanceMessage.mockResolvedValue(['Enhanced message content']);
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  // ============================================================
  // Initial State Tests
  // ============================================================

  describe('initial state', () => {
    it('should have isEnhancing === false when neither local nor API is enhancing', () => {
      const { result } = renderHook(() => useGroupsEnhance());

      expect(result.current.isEnhancing).toBe(false);
    });

    it('should expose handleEnhanceMessage function', () => {
      const { result } = renderHook(() => useGroupsEnhance());

      expect(typeof result.current.handleEnhanceMessage).toBe('function');
    });
  });

  // ============================================================
  // handleEnhanceMessage Tests
  // ============================================================

  describe('handleEnhanceMessage', () => {
    it('should call enhanceMessage with productLinks and set result via setMessage on success', async () => {
      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage('Check out this product link', mockSetMessage);
      });

      expect(mockEnhanceMessage).toHaveBeenCalledWith({
        productLinks: 'Check out this product link',
      });
      expect(mockSetMessage).toHaveBeenCalledWith('Enhanced message content');
      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.messageEnhanced',
        description: 'pages.groups.success.messageEnhancedMessage',
      });
    });

    it('should show destructive toast when message is empty and NOT call API', async () => {
      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage('', mockSetMessage);
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.emptyMessage',
        description: 'pages.groups.errors.typeMessageToEnhance',
      });
      expect(mockEnhanceMessage).not.toHaveBeenCalled();
      expect(mockSetMessage).not.toHaveBeenCalled();
    });

    it('should show destructive toast when message is only whitespace and NOT call API', async () => {
      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage('   ', mockSetMessage);
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.emptyMessage',
        description: 'pages.groups.errors.typeMessageToEnhance',
      });
      expect(mockEnhanceMessage).not.toHaveBeenCalled();
    });

    it('should show destructive toast when API returns empty array', async () => {
      mockEnhanceMessage.mockResolvedValue([]);

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage('Some product link', mockSetMessage);
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.unexpectedResponse',
        description: 'pages.groups.errors.unexpectedResponseMessage',
      });
      expect(mockSetMessage).not.toHaveBeenCalled();
    });

    it('should use only the first message when API returns multiple', async () => {
      mockEnhanceMessage.mockResolvedValue(['First message', 'Second message']);

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage('Some link', mockSetMessage);
      });

      expect(mockSetMessage).toHaveBeenCalledWith('First message');
    });

    it('should call captureHookError and show destructive toast on API error', async () => {
      const apiError = new Error('Network failure');
      mockEnhanceMessage.mockRejectedValue(apiError);

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage('Some product link', mockSetMessage);
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        apiError,
        'useGroupsEnhance',
        'handleEnhanceMessage'
      );
      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.enhanceMessage',
        description: 'pages.groups.errors.tryAgain',
      });
      expect(mockSetMessage).not.toHaveBeenCalled();
    });

    it('should handle non-Error exceptions', async () => {
      mockEnhanceMessage.mockRejectedValue('string error');

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage('Some link', mockSetMessage);
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        'string error',
        'useGroupsEnhance',
        'handleEnhanceMessage'
      );
      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.enhanceMessage',
        description: 'pages.groups.errors.tryAgain',
      });
    });

    it('should transition isEnhancing false -> true -> false on success', async () => {
      let resolveEnhance: (value: string[]) => void;
      mockEnhanceMessage.mockImplementation(
        () =>
          new Promise<string[]>((resolve) => {
            resolveEnhance = resolve;
          })
      );

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      expect(result.current.isEnhancing).toBe(false);

      let enhancePromise: Promise<void>;
      act(() => {
        enhancePromise = result.current.handleEnhanceMessage('Some link', mockSetMessage);
      });

      expect(result.current.isEnhancing).toBe(true);

      await act(async () => {
        resolveEnhance!(['Enhanced']);
        await enhancePromise!;
      });

      expect(result.current.isEnhancing).toBe(false);
    });

    it('should transition isEnhancing false -> true -> false on error', async () => {
      let rejectEnhance: (error: Error) => void;
      mockEnhanceMessage.mockImplementation(
        () =>
          new Promise<string[]>((_resolve, reject) => {
            rejectEnhance = reject;
          })
      );

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      expect(result.current.isEnhancing).toBe(false);

      let enhancePromise: Promise<void>;
      act(() => {
        enhancePromise = result.current.handleEnhanceMessage('Some link', mockSetMessage);
      });

      expect(result.current.isEnhancing).toBe(true);

      await act(async () => {
        rejectEnhance!(new Error('fail'));
        await enhancePromise!;
      });

      expect(result.current.isEnhancing).toBe(false);
    });

    it('should not set localEnhancing when message is empty (guard return)', async () => {
      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage('', mockSetMessage);
      });

      // isEnhancing should remain false since the guard triggers before setLocalEnhancing(true)
      expect(result.current.isEnhancing).toBe(false);
    });
  });

  // ============================================================
  // isEnhancing Computed Tests
  // ============================================================

  describe('isEnhancing computed', () => {
    it('should be false when nothing is enhancing', () => {
      mockIsEnhancingApi = false;

      const { result } = renderHook(() => useGroupsEnhance());

      expect(result.current.isEnhancing).toBe(false);
    });

    it('should be true when localEnhancing is true', async () => {
      let resolveEnhance: (value: string[]) => void;
      mockEnhanceMessage.mockImplementation(
        () =>
          new Promise<string[]>((resolve) => {
            resolveEnhance = resolve;
          })
      );

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      let enhancePromise: Promise<void>;
      act(() => {
        enhancePromise = result.current.handleEnhanceMessage('link', mockSetMessage);
      });

      // localEnhancing is true
      expect(result.current.isEnhancing).toBe(true);

      await act(async () => {
        resolveEnhance!(['done']);
        await enhancePromise!;
      });
    });

    it('should be true when API isEnhancing is true', () => {
      mockIsEnhancingApi = true;

      const { result } = renderHook(() => useGroupsEnhance());

      expect(result.current.isEnhancing).toBe(true);
    });

    it('should be true when both localEnhancing and API isEnhancing are true', async () => {
      mockIsEnhancingApi = true;

      let resolveEnhance: (value: string[]) => void;
      mockEnhanceMessage.mockImplementation(
        () =>
          new Promise<string[]>((resolve) => {
            resolveEnhance = resolve;
          })
      );

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      let enhancePromise: Promise<void>;
      act(() => {
        enhancePromise = result.current.handleEnhanceMessage('link', mockSetMessage);
      });

      expect(result.current.isEnhancing).toBe(true);

      await act(async () => {
        resolveEnhance!(['done']);
        await enhancePromise!;
      });

      // Still true because mockIsEnhancingApi is true
      expect(result.current.isEnhancing).toBe(true);
    });
  });

  // ============================================================
  // Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should handle very long messages', async () => {
      const longMessage = 'A'.repeat(10000);
      mockEnhanceMessage.mockResolvedValue(['Enhanced long message']);

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage(longMessage, mockSetMessage);
      });

      expect(mockEnhanceMessage).toHaveBeenCalledWith({
        productLinks: longMessage,
      });
      expect(mockSetMessage).toHaveBeenCalledWith('Enhanced long message');
    });

    it('should handle message with only special characters', async () => {
      const specialMessage = '!@#$%^&*()';
      mockEnhanceMessage.mockResolvedValue(['Enhanced special']);

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage = vi.fn();

      await act(async () => {
        await result.current.handleEnhanceMessage(specialMessage, mockSetMessage);
      });

      expect(mockEnhanceMessage).toHaveBeenCalledWith({
        productLinks: specialMessage,
      });
    });

    it('should handle concurrent enhance calls', async () => {
      let resolveFirst: (value: string[]) => void;
      let resolveSecond: (value: string[]) => void;

      mockEnhanceMessage
        .mockImplementationOnce(
          () =>
            new Promise<string[]>((resolve) => {
              resolveFirst = resolve;
            })
        )
        .mockImplementationOnce(
          () =>
            new Promise<string[]>((resolve) => {
              resolveSecond = resolve;
            })
        );

      const { result } = renderHook(() => useGroupsEnhance());
      const mockSetMessage1 = vi.fn();
      const mockSetMessage2 = vi.fn();

      let promise1: Promise<void>;
      let promise2: Promise<void>;

      act(() => {
        promise1 = result.current.handleEnhanceMessage('link1', mockSetMessage1);
      });

      act(() => {
        promise2 = result.current.handleEnhanceMessage('link2', mockSetMessage2);
      });

      expect(result.current.isEnhancing).toBe(true);

      await act(async () => {
        resolveFirst!(['Result 1']);
        await promise1!;
      });

      await act(async () => {
        resolveSecond!(['Result 2']);
        await promise2!;
      });

      expect(mockSetMessage1).toHaveBeenCalledWith('Result 1');
      expect(mockSetMessage2).toHaveBeenCalledWith('Result 2');
    });
  });
});
