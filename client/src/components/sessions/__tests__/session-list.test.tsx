import { act, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SessionList } from '../session-list';

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
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts?.count !== undefined) return `${key}_${opts.count}`;
      return key;
    },
    i18n: { language: 'pt-BR' },
  }),
}));

// Simplified AlertDialog mock (Radix portals break in jsdom)
vi.mock('@/components/ui/alert-dialog', () => ({
  AlertDialog: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  AlertDialogTrigger: ({ children, asChild }: { children: React.ReactNode; asChild?: boolean }) => (
    <>{children}</>
  ),
  AlertDialogContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="alert-dialog-content">{children}</div>
  ),
  AlertDialogHeader: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  AlertDialogTitle: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
  AlertDialogDescription: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
  AlertDialogFooter: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  AlertDialogCancel: ({ children }: { children: React.ReactNode }) => (
    <button type="button" data-testid="alert-cancel">
      {children}
    </button>
  ),
  AlertDialogAction: ({
    children,
    onClick,
  }: { children: React.ReactNode; onClick?: () => void }) => (
    <button type="button" data-testid="alert-action" onClick={onClick}>
      {children}
    </button>
  ),
}));

// ============================================================
// Auth Mock
// ============================================================

const mockListSessions = vi.fn();
const mockRevokeSession = vi.fn();
const mockSignOut = vi.fn();

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    listSessions: mockListSessions,
    revokeSession: mockRevokeSession,
    signOut: mockSignOut,
  }),
}));

// ============================================================
// Test Data
// ============================================================

function createSession(
  overrides: Partial<{
    sessionId: string;
    ipAddress: string;
    userAgent: string;
    isCurrent: boolean;
    lastActivityAt: Date | null;
    createdAt: Date | null;
    expiresAt: Date | null;
    deviceFingerprint: string;
  }> = {}
) {
  return {
    sessionId: 'session-1',
    ipAddress: '192.168.1.1',
    userAgent: 'Mozilla/5.0 Chrome/120',
    isCurrent: false,
    lastActivityAt: new Date(),
    createdAt: new Date(),
    expiresAt: new Date(Date.now() + 3600000),
    deviceFingerprint: 'fp-123',
    ...overrides,
  };
}

const CURRENT_SESSION = createSession({
  sessionId: 'session-current',
  ipAddress: '10.0.0.1',
  userAgent: 'Mozilla/5.0 Chrome/120',
  isCurrent: true,
});

const OTHER_SESSION_1 = createSession({
  sessionId: 'session-2',
  ipAddress: '192.168.1.100',
  userAgent: 'Mozilla/5.0 Firefox/119',
});

const OTHER_SESSION_2 = createSession({
  sessionId: 'session-3',
  ipAddress: '172.16.0.50',
  userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) Safari/604',
});

// ============================================================
// Tests
// ============================================================

describe('SessionList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ============================================================
  // Loading State
  // ============================================================

  describe('loading state', () => {
    it('renders loading spinner while fetching', () => {
      mockListSessions.mockReturnValue(new Promise(() => {})); // never resolves

      render(<SessionList />);

      expect(screen.getByText('pages.profile.sessions.title')).toBeInTheDocument();
      const spinner = document.querySelector('.animate-spin');
      expect(spinner).toBeInTheDocument();
    });

    it('does not render session cards while loading', () => {
      mockListSessions.mockReturnValue(new Promise(() => {}));

      render(<SessionList />);

      expect(screen.queryByText(/Chrome/)).not.toBeInTheDocument();
    });
  });

  // ============================================================
  // Empty State
  // ============================================================

  describe('empty state', () => {
    it('renders no-sessions message', async () => {
      mockListSessions.mockResolvedValue({ sessions: [], error: null });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText('pages.profile.sessions.noSessions')).toBeInTheDocument();
      });
    });

    it('hides logout-all button', async () => {
      mockListSessions.mockResolvedValue({ sessions: [], error: null });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.queryByText('pages.profile.sessions.logoutAll')).not.toBeInTheDocument();
      });
    });
  });

  // ============================================================
  // Session Rendering
  // ============================================================

  describe('session rendering', () => {
    it('renders all session cards after load', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [CURRENT_SESSION, OTHER_SESSION_1, OTHER_SESSION_2],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/Chrome/)).toBeInTheDocument();
        expect(screen.getByText(/Firefox/)).toBeInTheDocument();
        expect(screen.getByText(/Safari/)).toBeInTheDocument();
      });
    });

    it('shows IP address per session', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [CURRENT_SESSION, OTHER_SESSION_1],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
        expect(screen.getByText('192.168.1.100')).toBeInTheDocument();
      });
    });

    it('shows "current" badge for isCurrent=true', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [CURRENT_SESSION],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText('pages.profile.sessions.current')).toBeInTheDocument();
      });
    });

    it('hides revoke button for current session', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [CURRENT_SESSION],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.queryByText('pages.profile.sessions.revoke')).not.toBeInTheDocument();
      });
    });

    it('shows revoke button for non-current sessions', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [OTHER_SESSION_1],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText('pages.profile.sessions.revoke')).toBeInTheDocument();
      });
    });

    it('shows logout-all button when >1 session', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [CURRENT_SESSION, OTHER_SESSION_1],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText('pages.profile.sessions.logoutAll')).toBeInTheDocument();
      });
    });

    it('hides logout-all button when 1 session', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [CURRENT_SESSION],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/Chrome/)).toBeInTheDocument();
      });

      expect(screen.queryByText('pages.profile.sessions.logoutAll')).not.toBeInTheDocument();
    });

    it('shows session description with count', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [CURRENT_SESSION, OTHER_SESSION_1],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText('pages.profile.sessions.description_2')).toBeInTheDocument();
      });
    });
  });

  // ============================================================
  // Device Detection
  // ============================================================

  describe('device detection', () => {
    it('detects mobile for android user agent', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ userAgent: 'Mozilla/5.0 (Linux; Android 13) Chrome/120' })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.mobile/)).toBeInTheDocument();
      });
    });

    it('detects mobile for iphone user agent', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0)' })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.mobile/)).toBeInTheDocument();
      });
    });

    it('detects tablet for ipad user agent', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ userAgent: 'Mozilla/5.0 (iPad; CPU OS 17_0) Safari/604' })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.tablet/)).toBeInTheDocument();
      });
    });

    it('detects desktop for electron user agent', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ userAgent: 'Mozilla/5.0 Electron/28.0' })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.desktop/)).toBeInTheDocument();
      });
    });

    it('defaults to browser for standard user agent', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ userAgent: 'Mozilla/5.0 Chrome/120' })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.browser/)).toBeInTheDocument();
      });
    });
  });

  // ============================================================
  // Browser Detection
  // ============================================================

  describe('browser detection', () => {
    const browserCases = [
      { ua: 'Mozilla/5.0 Chrome/120', expected: 'Chrome' },
      { ua: 'Mozilla/5.0 Firefox/119', expected: 'Firefox' },
      { ua: 'Mozilla/5.0 Edg/120', expected: 'Edge' },
      { ua: 'Mozilla/5.0 Safari/604', expected: 'Safari' },
      { ua: 'Mozilla/5.0 OPR/106', expected: 'Opera' },
    ];

    for (const { ua, expected } of browserCases) {
      it(`detects ${expected}`, async () => {
        mockListSessions.mockResolvedValue({
          sessions: [createSession({ userAgent: ua })],
          error: null,
        });

        render(<SessionList />);

        await waitFor(() => {
          expect(screen.getByText(new RegExp(expected))).toBeInTheDocument();
        });
      });
    }

    it('returns unknown browser for unrecognized user agent', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ userAgent: 'CurlBot/1.0' })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.unknownBrowser/)).toBeInTheDocument();
      });
    });
  });

  // ============================================================
  // Relative Time
  // ============================================================

  describe('relative time', () => {
    it('returns dash for null date', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ lastActivityAt: null })],
        error: null,
      });

      render(<SessionList />);

      // formatRelativeTime returns '-' for null, rendered inside "lastActivity: -"
      await waitFor(() => {
        const activityText = screen.getByText(
          (content) =>
            content.includes('-') && content.includes('pages.profile.sessions.lastActivity')
        );
        expect(activityText).toBeInTheDocument();
      });
    });

    it('returns "just now" for recent activity', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ lastActivityAt: new Date() })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.justNow/)).toBeInTheDocument();
      });
    });

    it('returns minutes ago for older activity', async () => {
      const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000);
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ lastActivityAt: fiveMinutesAgo })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.minutesAgo_5/)).toBeInTheDocument();
      });
    });

    it('returns hours ago for older activity', async () => {
      const threeHoursAgo = new Date(Date.now() - 3 * 60 * 60 * 1000);
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ lastActivityAt: threeHoursAgo })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.hoursAgo_3/)).toBeInTheDocument();
      });
    });

    it('returns days ago for old activity', async () => {
      const twoDaysAgo = new Date(Date.now() - 2 * 24 * 60 * 60 * 1000);
      mockListSessions.mockResolvedValue({
        sessions: [createSession({ lastActivityAt: twoDaysAgo })],
        error: null,
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/pages\.profile\.sessions\.daysAgo_2/)).toBeInTheDocument();
      });
    });
  });

  // ============================================================
  // Revoke Session
  // ============================================================

  describe('revoke session', () => {
    it('calls revokeSession with correct ID', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [OTHER_SESSION_1],
        error: null,
      });
      mockRevokeSession.mockResolvedValue({ error: null });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/Firefox/)).toBeInTheDocument();
      });

      const confirmButton = screen.getByText('pages.profile.sessions.revokeConfirm');
      await act(async () => {
        confirmButton.click();
      });

      expect(mockRevokeSession).toHaveBeenCalledWith('session-2');
    });

    it('removes session from list on success', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [CURRENT_SESSION, OTHER_SESSION_1],
        error: null,
      });
      mockRevokeSession.mockResolvedValue({ error: null });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/Firefox/)).toBeInTheDocument();
      });

      const confirmButton = screen.getByText('pages.profile.sessions.revokeConfirm');
      await act(async () => {
        confirmButton.click();
      });

      await waitFor(() => {
        expect(screen.queryByText(/Firefox/)).not.toBeInTheDocument();
      });
    });

    it('shows success toast', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [OTHER_SESSION_1],
        error: null,
      });
      mockRevokeSession.mockResolvedValue({ error: null });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/Firefox/)).toBeInTheDocument();
      });

      const confirmButton = screen.getByText('pages.profile.sessions.revokeConfirm');
      await act(async () => {
        confirmButton.click();
      });

      await waitFor(() => {
        expect(mockToast).toHaveBeenCalledWith({
          title: 'pages.profile.sessions.revokeSuccess',
          description: 'pages.profile.sessions.revokeSuccessMessage',
        });
      });
    });

    it('shows error toast on failure', async () => {
      mockListSessions.mockResolvedValue({
        sessions: [OTHER_SESSION_1],
        error: null,
      });
      mockRevokeSession.mockResolvedValue({
        error: { message: 'Failed to revoke' },
      });

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/Firefox/)).toBeInTheDocument();
      });

      const confirmButton = screen.getByText('pages.profile.sessions.revokeConfirm');
      await act(async () => {
        confirmButton.click();
      });

      await waitFor(() => {
        expect(mockToast).toHaveBeenCalledWith({
          variant: 'destructive',
          title: 'pages.profile.sessions.revokeError',
          description: 'Failed to revoke',
        });
      });
    });
  });

  // ============================================================
  // Logout All Devices
  // ============================================================

  describe('logout all devices', () => {
    it('calls signOut(true)', async () => {
      mockListSessions
        .mockResolvedValueOnce({
          sessions: [CURRENT_SESSION, OTHER_SESSION_1],
          error: null,
        })
        .mockResolvedValueOnce({ sessions: [CURRENT_SESSION], error: null });
      mockSignOut.mockResolvedValue(undefined);

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText('pages.profile.sessions.logoutAll')).toBeInTheDocument();
      });

      // The logout-all confirm is the first alert-action; the second is for the revoke
      const confirmButtons = screen.getAllByTestId('alert-action');
      // Logout-all confirm comes before the revoke confirm in DOM order
      const logoutAllConfirm = confirmButtons.find(
        (btn) => btn.textContent === 'pages.profile.sessions.logoutAllConfirm'
      );
      await act(async () => {
        logoutAllConfirm!.click();
      });

      expect(mockSignOut).toHaveBeenCalledWith(true);
    });

    it('shows success toast', async () => {
      mockListSessions
        .mockResolvedValueOnce({
          sessions: [CURRENT_SESSION, OTHER_SESSION_1],
          error: null,
        })
        .mockResolvedValueOnce({ sessions: [CURRENT_SESSION], error: null });
      mockSignOut.mockResolvedValue(undefined);

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText('pages.profile.sessions.logoutAll')).toBeInTheDocument();
      });

      const logoutAllConfirm = screen.getByText('pages.profile.sessions.logoutAllConfirm');
      await act(async () => {
        logoutAllConfirm.click();
      });

      await waitFor(() => {
        expect(mockToast).toHaveBeenCalledWith({
          title: 'pages.profile.sessions.logoutAllSuccess',
          description: 'pages.profile.sessions.logoutAllSuccessMessage',
        });
      });
    });

    it('refreshes session list after logout', async () => {
      mockListSessions
        .mockResolvedValueOnce({
          sessions: [CURRENT_SESSION, OTHER_SESSION_1],
          error: null,
        })
        .mockResolvedValueOnce({ sessions: [CURRENT_SESSION], error: null });
      mockSignOut.mockResolvedValue(undefined);

      render(<SessionList />);

      await waitFor(() => {
        expect(screen.getByText(/Firefox/)).toBeInTheDocument();
      });

      const logoutAllConfirm = screen.getByText('pages.profile.sessions.logoutAllConfirm');
      await act(async () => {
        logoutAllConfirm.click();
      });

      // listSessions called: once on mount + once after logout
      await waitFor(() => {
        expect(mockListSessions).toHaveBeenCalledTimes(2);
      });
    });
  });
});
