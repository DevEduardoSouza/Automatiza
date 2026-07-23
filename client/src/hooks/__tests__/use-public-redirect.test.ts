import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { usePublicRedirect } from '../use-public-redirect';

// ============================================================
// Mock Modules
// ============================================================

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

// ============================================================
// Mock: react-router
// ============================================================

let mockSlug: string | undefined = 'test-slug';
// Route the hook thinks it is on. `/r/...` is the legacy SPA path (auto
// redirect); `/join/...` is the backend fallback (always manual).
let mockPathname = '/r/test-slug';
let mockSearch = '';

vi.mock('react-router', () => ({
  useParams: () => ({ slug: mockSlug }),
  useLocation: () => ({
    pathname: mockPathname,
    search: mockSearch,
    hash: '',
    state: null,
    key: 'test',
  }),
}));

// ============================================================
// Mock: API service
// ============================================================

const mockProcessRedirectBySlug = vi.fn();

vi.mock('@/services/api.service', () => ({
  redirectService: {
    processRedirectBySlug: (...args: unknown[]) => mockProcessRedirectBySlug(...args),
  },
}));

// ============================================================
// Mock: Sentry
// ============================================================

const mockCaptureHookError = vi.fn();

vi.mock('@/lib/sentry', () => ({
  captureHookError: (...args: unknown[]) => mockCaptureHookError(...args),
}));

// ============================================================
// Helpers
// ============================================================

function renderPublicRedirect() {
  return renderHook(() => usePublicRedirect());
}

// Store original objects to restore
const originalNavigator = { ...navigator };
const originalLocation = window.location;

// ============================================================
// Tests
// ============================================================

describe('usePublicRedirect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockSlug = 'test-slug';
    mockPathname = '/r/test-slug';
    mockSearch = '';
    mockProcessRedirectBySlug.mockResolvedValue({
      success: true,
      group_name: 'Promocoes Tech',
      redirect_url: 'https://chat.whatsapp.com/ABC123',
    });

    // Mock window.location
    Object.defineProperty(window, 'location', {
      writable: true,
      value: {
        ...originalLocation,
        href: 'http://localhost:3003/r/test-slug',
        origin: 'http://localhost:3003',
      },
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    // Restore window.location
    Object.defineProperty(window, 'location', {
      writable: true,
      value: originalLocation,
    });
  });

  // ── Initial State ──────────────────────────────────────────────
  describe('initial state', () => {
    it('should start with loading=true', () => {
      // Prevent the effect from running
      mockProcessRedirectBySlug.mockImplementation(() => new Promise(() => {}));
      const { result } = renderPublicRedirect();
      expect(result.current.loading).toBe(true);
    });

    it('should start with error=null', () => {
      mockProcessRedirectBySlug.mockImplementation(() => new Promise(() => {}));
      const { result } = renderPublicRedirect();
      expect(result.current.error).toBeNull();
    });

    it('should start with empty groupName and redirectUrl', () => {
      mockProcessRedirectBySlug.mockImplementation(() => new Promise(() => {}));
      const { result } = renderPublicRedirect();
      expect(result.current.groupName).toBe('');
      expect(result.current.redirectUrl).toBe('');
    });
  });

  // ── No Slug ────────────────────────────────────────────────────
  describe('no slug', () => {
    it('should set error with invalidLink when slug is undefined', async () => {
      mockSlug = undefined;
      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.error).toBe('pages.publicRedirect.error.invalidLink');
      expect(result.current.loading).toBe(false);
    });
  });

  // ── Successful Redirect (standard browser) ─────────────────────
  describe('successful redirect - standard browser', () => {
    it('should call processRedirectBySlug with slug', async () => {
      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(mockProcessRedirectBySlug).toHaveBeenCalledWith('test-slug');
      expect(result.current.groupName).toBe('Promocoes Tech');
      expect(result.current.redirectUrl).toBe('https://chat.whatsapp.com/ABC123');
    });

    it('should redirect via window.location.href in standard browser', async () => {
      renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(window.location.href).toBe('https://chat.whatsapp.com/ABC123');
    });

    it('should set loading=false after 2500ms timeout', async () => {
      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      // Still loading (waiting for redirect timeout)
      expect(result.current.loading).toBe(true);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2500);
      });

      expect(result.current.loading).toBe(false);
    });
  });

  // ── In-App Browser Detection ───────────────────────────────────
  describe('in-app browser detection', () => {
    it('should detect TikTok UA as in-app browser', async () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) TikTok/14.0.0',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.isInAppBrowser).toBe(true);

      // Restore
      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });

    it('should detect Instagram UA as in-app browser', async () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) Instagram 200.0',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();

      expect(result.current.isInAppBrowser).toBe(true);

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });

    it('should detect FBAN/FBAV as in-app browser', async () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 FBAN/FBAV/200.0',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();

      expect(result.current.isInAppBrowser).toBe(true);

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });

    it('should NOT auto-redirect in in-app browser', async () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 TikTok/14.0.0',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      // loading should be false immediately in in-app browser
      expect(result.current.loading).toBe(false);
      // Should not redirect
      expect(window.location.href).not.toBe('https://chat.whatsapp.com/ABC123');

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });
  });

  // ── Invite Code Extraction ─────────────────────────────────────
  describe('inviteCode extraction', () => {
    it('should extract code from chat.whatsapp.com URL', async () => {
      mockProcessRedirectBySlug.mockResolvedValue({
        success: true,
        group_name: 'Test',
        redirect_url: 'https://chat.whatsapp.com/ABC123XYZ',
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.inviteCode).toBe('ABC123XYZ');
    });

    it('should return empty string when URL has no invite code', async () => {
      mockProcessRedirectBySlug.mockResolvedValue({
        success: true,
        group_name: 'Test',
        redirect_url: 'https://example.com/other',
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.inviteCode).toBe('');
    });

    it('should return empty string when redirectUrl is empty', () => {
      mockProcessRedirectBySlug.mockImplementation(() => new Promise(() => {}));
      const { result } = renderPublicRedirect();
      expect(result.current.inviteCode).toBe('');
    });
  });

  // ── Platform Detection ─────────────────────────────────────────
  describe('platform detection', () => {
    it('should detect iPhone as iOS', () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();
      expect(result.current.isIOS).toBe(true);
      expect(result.current.isAndroid).toBe(false);

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });

    it('should detect iPad as iOS', () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 (iPad; CPU OS 14_0 like Mac OS X)',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();
      expect(result.current.isIOS).toBe(true);

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });

    it('should detect Android UA', () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 (Linux; Android 12; SM-G998B)',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();
      expect(result.current.isAndroid).toBe(true);
      expect(result.current.isIOS).toBe(false);

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });

    it('should detect desktop (neither iOS nor Android)', () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();
      expect(result.current.isIOS).toBe(false);
      expect(result.current.isAndroid).toBe(false);

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });
  });

  // ── Error Handling ─────────────────────────────────────────────
  describe('error handling', () => {
    it('should translate "link not found" error', async () => {
      mockProcessRedirectBySlug.mockResolvedValue({
        success: false,
        error: 'link not found',
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.error).toBe('pages.publicRedirect.error.invalidLink');
      expect(result.current.errorType).toBe('not_found');
    });

    it('should translate "link is inactive" error', async () => {
      mockProcessRedirectBySlug.mockResolvedValue({
        success: false,
        error: 'link is inactive',
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.error).toBe('pages.publicRedirect.error.linkInactive');
    });

    it('should translate "no target groups configured" error', async () => {
      mockProcessRedirectBySlug.mockResolvedValue({
        success: false,
        error: 'no target groups configured',
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.error).toBe('pages.publicRedirect.error.noGroups');
    });

    it('should classify "groups_full" error type', async () => {
      mockProcessRedirectBySlug.mockResolvedValue({
        success: false,
        error: 'no available groups (all groups are full or inactive)',
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.errorType).toBe('groups_full');
    });

    it('should classify "temporary" error type', async () => {
      mockProcessRedirectBySlug.mockResolvedValue({
        success: false,
        error: 'service temporarily unavailable',
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.errorType).toBe('temporary');
    });

    it('should classify generic error type by default', async () => {
      mockProcessRedirectBySlug.mockResolvedValue({
        success: false,
        error: 'unknown error',
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.errorType).toBe('generic');
    });

    it('should handle API exception with captureHookError', async () => {
      mockProcessRedirectBySlug.mockRejectedValue(new Error('Network error'));

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.error).toBe('Network error');
      expect(result.current.loading).toBe(false);
      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'usePublicRedirect',
        'handleRedirect'
      );
    });

    it('should handle non-Error exceptions with generic message', async () => {
      mockProcessRedirectBySlug.mockRejectedValue('string error');

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.error).toBe('pages.publicRedirect.error.genericError');
    });

    it('should translate known backend errors', async () => {
      const errorCases = [
        { error: 'group not found', expected: 'pages.publicRedirect.error.groupNotFound' },
        { error: 'instance not found', expected: 'pages.publicRedirect.error.instanceNotFound' },
        { error: 'bot is not admin in group', expected: 'pages.publicRedirect.error.notAdmin' },
        {
          error: 'failed to get invite code',
          expected: 'pages.publicRedirect.error.inviteCodeFailed',
        },
        {
          error: 'invite code not available',
          expected: 'pages.publicRedirect.error.inviteCodeNotAvailable',
        },
        {
          error: 'WhatsApp provider not configured',
          expected: 'pages.publicRedirect.error.providerNotConfigured',
        },
      ];

      for (const { error, expected } of errorCases) {
        mockProcessRedirectBySlug.mockResolvedValue({
          success: false,
          error,
        });

        const { result } = renderPublicRedirect();

        await act(async () => {
          await vi.advanceTimersByTimeAsync(100);
        });

        expect(result.current.error).toBe(expected);
      }
    });
  });

  // ── tryOpenWhatsApp ────────────────────────────────────────────
  describe('tryOpenWhatsApp', () => {
    it('should noop when redirectUrl is empty', async () => {
      mockProcessRedirectBySlug.mockImplementation(() => new Promise(() => {}));
      const { result } = renderPublicRedirect();

      act(() => result.current.tryOpenWhatsApp());

      // No error, just noop
      expect(result.current.actionHint).toBe('');
    });

    it('should create iframes with whatsapp:// schemes', async () => {
      // Set up in-app browser context
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 TikTok/14.0.0',
        writable: true,
        configurable: true,
      });

      const createElementSpy = vi.spyOn(document, 'createElement');

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      act(() => result.current.tryOpenWhatsApp());

      // Should have created iframes for whatsapp:// schemes
      const iframeCalls = createElementSpy.mock.calls.filter((call) => call[0] === 'iframe');
      expect(iframeCalls.length).toBeGreaterThanOrEqual(3);

      createElementSpy.mockRestore();
      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });

    it('should set actionHint after timeout if document is visible', async () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 TikTok/14.0.0 (iPhone; CPU iPhone OS 14_0)',
        writable: true,
        configurable: true,
      });

      Object.defineProperty(document, 'visibilityState', {
        value: 'visible',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      act(() => result.current.tryOpenWhatsApp());

      await act(async () => {
        await vi.advanceTimersByTimeAsync(1300);
      });

      // Should have set actionHint
      expect(result.current.actionHint).toBe('pages.publicRedirect.manual.iosHint');

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });
  });

  // ── copyInviteLink ─────────────────────────────────────────────
  describe('copyInviteLink', () => {
    it('should noop when redirectUrl is empty', async () => {
      mockProcessRedirectBySlug.mockImplementation(() => new Promise(() => {}));
      const { result } = renderPublicRedirect();

      await act(async () => {
        await result.current.copyInviteLink();
      });

      // No error, just noop
    });

    it('should use navigator.clipboard.writeText when available', async () => {
      // Set up in-app browser context so we don't redirect
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 TikTok/14.0.0',
        writable: true,
        configurable: true,
      });

      const writeTextMock = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText: writeTextMock },
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      await act(async () => {
        await result.current.copyInviteLink();
      });

      expect(writeTextMock).toHaveBeenCalledWith('https://chat.whatsapp.com/ABC123');

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });

    it('should fallback to execCommand when clipboard fails', async () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 TikTok/14.0.0',
        writable: true,
        configurable: true,
      });

      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockRejectedValue(new Error('Clipboard blocked')),
        },
        writable: true,
        configurable: true,
      });

      // Define execCommand if not present in jsdom
      if (!document.execCommand) {
        Object.defineProperty(document, 'execCommand', {
          value: vi.fn().mockReturnValue(true),
          writable: true,
          configurable: true,
        });
      }
      const execCommandSpy = vi.spyOn(document, 'execCommand').mockReturnValue(true);

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      await act(async () => {
        await result.current.copyInviteLink();
      });

      expect(execCommandSpy).toHaveBeenCalledWith('copy');

      execCommandSpy.mockRestore();
      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });
  });

  // ── openInviteLink ─────────────────────────────────────────────
  describe('openInviteLink', () => {
    it('should noop when redirectUrl is empty', () => {
      mockProcessRedirectBySlug.mockImplementation(() => new Promise(() => {}));
      const { result } = renderPublicRedirect();

      act(() => result.current.openInviteLink());
      // No error, just noop
    });

    it('should use window.open with _blank', async () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 TikTok/14.0.0',
        writable: true,
        configurable: true,
      });

      const openSpy = vi.spyOn(window, 'open').mockReturnValue({} as Window);

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      act(() => result.current.openInviteLink());

      expect(openSpy).toHaveBeenCalledWith(
        'https://chat.whatsapp.com/ABC123',
        '_blank',
        'noopener,noreferrer'
      );

      openSpy.mockRestore();
      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });

    it('should fallback to location.href when window.open returns null', async () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 TikTok/14.0.0',
        writable: true,
        configurable: true,
      });

      const openSpy = vi.spyOn(window, 'open').mockReturnValue(null);

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      act(() => result.current.openInviteLink());

      expect(window.location.href).toBe('https://chat.whatsapp.com/ABC123');

      openSpy.mockRestore();
      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });
  });

  // ── /join fallback route ───────────────────────────────────────
  // The backend only 302s a client to /join when a direct redirect would not
  // work for it, so this route must NEVER auto-redirect — not even on a
  // desktop UA.
  describe('/join fallback route', () => {
    beforeEach(() => {
      mockPathname = '/join/test-slug';
    });

    it('should make exactly one POST and stay manual on a desktop UA', async () => {
      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).toHaveBeenCalledTimes(1);
      expect(window.location.href).toBe('http://localhost:3003/r/test-slug');
      expect(result.current.loading).toBe(false);
      expect(result.current.redirectUrl).toBe('https://chat.whatsapp.com/ABC123');
    });

    it('should stay manual on an in-app UA too', async () => {
      Object.defineProperty(navigator, 'userAgent', {
        value: 'Mozilla/5.0 (iPhone) Instagram 334.0.0.32.98',
        writable: true,
        configurable: true,
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).toHaveBeenCalledTimes(1);
      expect(window.location.href).toBe('http://localhost:3003/r/test-slug');
      expect(result.current.isInAppBrowser).toBe(true);

      Object.defineProperty(navigator, 'userAgent', {
        value: originalNavigator.userAgent,
        writable: true,
        configurable: true,
      });
    });
  });

  // ── ?error=<code> from the direct GET ──────────────────────────
  describe('error code from the backend', () => {
    beforeEach(() => {
      mockPathname = '/join/test-slug';
    });

    it('should render the mapped error WITHOUT calling the API again', async () => {
      mockSearch = '?error=not_found';

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(mockProcessRedirectBySlug).not.toHaveBeenCalled();
      expect(result.current.error).toBe('pages.publicRedirect.error.invalidLink');
      expect(result.current.errorType).toBe('not_found');
      expect(result.current.loading).toBe(false);
    });

    it('should map groups_full to the retryable error type', async () => {
      mockSearch = '?error=groups_full';

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.error).toBe('pages.publicRedirect.error.groupsFull');
      expect(result.current.errorType).toBe('groups_full');
      expect(mockProcessRedirectBySlug).not.toHaveBeenCalled();
    });

    it('should fall back to generic for a prototype key (tela branca)', async () => {
      // `?error=constructor` resolvia no protótipo do objeto de mapeamento e
      // devolvia uma função, então o fallback nunca disparava e a página
      // renderizava vazia.
      for (const evil of ['constructor', '__proto__', 'toString', 'hasOwnProperty']) {
        mockSearch = `?error=${evil}`;
        const { result, unmount } = renderPublicRedirect();

        await act(async () => {
          await vi.advanceTimersByTimeAsync(100);
        });

        expect(result.current.error).toBe('pages.publicRedirect.error.genericError');
        expect(result.current.errorType).toBe('generic');
        expect(result.current.loading).toBe(false);
        expect(mockProcessRedirectBySlug).not.toHaveBeenCalled();
        unmount();
      }
    });

    it('should fall back to generic for an unknown code', async () => {
      mockSearch = '?error=whatever-someone-typed';

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(result.current.error).toBe('pages.publicRedirect.error.genericError');
      expect(result.current.errorType).toBe('generic');
      expect(mockProcessRedirectBySlug).not.toHaveBeenCalled();
    });

    it('should strip the query so a reload becomes a clean attempt', async () => {
      mockSearch = '?error=temporary';
      const replaceSpy = vi.spyOn(window.history, 'replaceState');

      renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(replaceSpy).toHaveBeenCalledWith(null, '', '/join/test-slug');

      // The clean URL: a reload now performs one real attempt.
      mockSearch = '';
      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).toHaveBeenCalledTimes(1);
      expect(result.current.redirectUrl).toBe('https://chat.whatsapp.com/ABC123');

      replaceSpy.mockRestore();
    });
  });

  // ── ?mode=preview ──────────────────────────────────────────────
  describe('preview mode', () => {
    beforeEach(() => {
      mockPathname = '/join/test-slug';
      mockSearch = '?mode=preview';
    });

    afterEach(() => {
      Reflect.deleteProperty(document, 'prerendering');
    });

    it('should never call the API when the document is not prerendering', async () => {
      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).not.toHaveBeenCalled();
      expect(result.current.loading).toBe(false);
      // A real user caught by a UA false positive still gets a retry button.
      expect(result.current.errorType).toBe('temporary');
    });

    it('should not call the API before the prerender is activated', async () => {
      Object.defineProperty(document, 'prerendering', {
        value: true,
        writable: true,
        configurable: true,
      });

      renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).not.toHaveBeenCalled();
    });

    it('should call the API exactly once when the user activates the prerender', async () => {
      Object.defineProperty(document, 'prerendering', {
        value: true,
        writable: true,
        configurable: true,
      });

      renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(mockProcessRedirectBySlug).not.toHaveBeenCalled();

      await act(async () => {
        document.dispatchEvent(new Event('prerenderingchange'));
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).toHaveBeenCalledTimes(1);
    });

    it('should not call the API when unmounted before activation', async () => {
      Object.defineProperty(document, 'prerendering', {
        value: true,
        writable: true,
        configurable: true,
      });

      const { unmount } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      unmount();

      await act(async () => {
        document.dispatchEvent(new Event('prerenderingchange'));
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).not.toHaveBeenCalled();
    });
  });

  // ── Attempt guard ──────────────────────────────────────────────
  describe('attempt guard', () => {
    it('should not repeat the POST on a rerender', async () => {
      const { rerender } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).toHaveBeenCalledTimes(1);

      await act(async () => {
        rerender();
        await vi.advanceTimersByTimeAsync(100);
      });

      expect(mockProcessRedirectBySlug).toHaveBeenCalledTimes(1);
    });

    it('should start a new attempt when the slug actually changes', async () => {
      const { rerender } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).toHaveBeenCalledTimes(1);

      mockSlug = 'another-slug';
      await act(async () => {
        rerender();
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(mockProcessRedirectBySlug).toHaveBeenCalledTimes(2);
      expect(mockProcessRedirectBySlug).toHaveBeenLastCalledWith('another-slug');
    });
  });

  // ── Invite code extraction ─────────────────────────────────────
  describe('invite code extraction', () => {
    it('should keep _ and - in the invite code', async () => {
      mockPathname = '/join/test-slug';
      mockProcessRedirectBySlug.mockResolvedValue({
        success: true,
        group_name: 'Promocoes Tech',
        redirect_url: 'https://chat.whatsapp.com/Abc_1-Xyz9',
      });

      const { result } = renderPublicRedirect();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      expect(result.current.inviteCode).toBe('Abc_1-Xyz9');
    });
  });
});
