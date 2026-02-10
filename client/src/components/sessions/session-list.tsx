import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { useAuth } from '@/contexts';
import { useToast } from '@/hooks/use-toast';
import { Laptop, Loader2, LogOut, Monitor, Smartphone, Tablet } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface SessionInfo {
  sessionId: string;
  expiresAt: Date | null;
  deviceFingerprint: string;
  ipAddress: string;
  userAgent: string;
  createdAt: Date | null;
  lastActivityAt: Date | null;
  isCurrent: boolean;
}

/**
 * SessionList component displays all active sessions for the current user.
 * Allows users to view device info and revoke sessions.
 */
export function SessionList() {
  const { t } = useTranslation();
  const { listSessions, revokeSession, signOut } = useAuth();
  const { toast } = useToast();

  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [revoking, setRevoking] = useState<string | null>(null);
  const [revokingAll, setRevokingAll] = useState(false);

  // Fetch sessions on mount
  useEffect(() => {
    async function fetchSessions() {
      setLoading(true);
      const result = await listSessions();
      if (!result.error && result.sessions) {
        setSessions(result.sessions);
      }
      setLoading(false);
    }
    fetchSessions();
  }, [listSessions]);

  // Parse user agent to get device type
  const getDeviceInfo = useCallback(
    (userAgent: string) => {
      const ua = userAgent.toLowerCase();

      if (ua.includes('mobile') || ua.includes('android') || ua.includes('iphone')) {
        return {
          type: 'mobile' as const,
          icon: Smartphone,
          label: t('pages.profile.sessions.mobile'),
        };
      }
      if (ua.includes('tablet') || ua.includes('ipad')) {
        return {
          type: 'tablet' as const,
          icon: Tablet,
          label: t('pages.profile.sessions.tablet'),
        };
      }
      if (ua.includes('electron') || ua.includes('desktop')) {
        return {
          type: 'desktop' as const,
          icon: Laptop,
          label: t('pages.profile.sessions.desktop'),
        };
      }
      return {
        type: 'browser' as const,
        icon: Monitor,
        label: t('pages.profile.sessions.browser'),
      };
    },
    [t]
  );

  // Parse browser from user agent
  const getBrowserName = useCallback(
    (userAgent: string): string => {
      const ua = userAgent.toLowerCase();
      if (ua.includes('firefox')) return 'Firefox';
      if (ua.includes('edg/')) return 'Edge';
      if (ua.includes('chrome')) return 'Chrome';
      if (ua.includes('safari')) return 'Safari';
      if (ua.includes('opera') || ua.includes('opr')) return 'Opera';
      return t('pages.profile.sessions.unknownBrowser');
    },
    [t]
  );

  // Format relative time
  const formatRelativeTime = useCallback(
    (date: Date | null): string => {
      if (!date) return '-';
      const now = new Date();
      const diff = now.getTime() - date.getTime();
      const minutes = Math.floor(diff / 60000);
      const hours = Math.floor(minutes / 60);
      const days = Math.floor(hours / 24);

      if (minutes < 1) return t('pages.profile.sessions.justNow');
      if (minutes < 60) return t('pages.profile.sessions.minutesAgo', { count: minutes });
      if (hours < 24) return t('pages.profile.sessions.hoursAgo', { count: hours });
      return t('pages.profile.sessions.daysAgo', { count: days });
    },
    [t]
  );

  // Revoke a specific session
  const handleRevokeSession = useCallback(
    async (sessionId: string) => {
      setRevoking(sessionId);
      const result = await revokeSession(sessionId);
      if (result.error) {
        toast({
          variant: 'destructive',
          title: t('pages.profile.sessions.revokeError'),
          description: result.error.message,
        });
      } else {
        toast({
          title: t('pages.profile.sessions.revokeSuccess'),
          description: t('pages.profile.sessions.revokeSuccessMessage'),
        });
        // Remove from local state
        setSessions((prev) => prev.filter((s) => s.sessionId !== sessionId));
      }
      setRevoking(null);
    },
    [revokeSession, toast, t]
  );

  // Logout all other devices
  const handleLogoutAllDevices = useCallback(async () => {
    setRevokingAll(true);
    await signOut(true); // logoutAllDevices = true
    toast({
      title: t('pages.profile.sessions.logoutAllSuccess'),
      description: t('pages.profile.sessions.logoutAllSuccessMessage'),
    });
    // Refresh sessions
    const result = await listSessions();
    if (!result.error && result.sessions) {
      setSessions(result.sessions);
    }
    setRevokingAll(false);
  }, [signOut, listSessions, toast, t]);

  if (loading) {
    return (
      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <span
          className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
          style={{ color: colors.wineText }}
        >
          {t('pages.profile.sessions.title')}
        </span>
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin" style={{ color: colors.wineTextMuted }} />
        </div>
      </div>
    );
  }

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center justify-between mb-3 xs:mb-4">
        <div>
          <span
            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
            style={{ color: colors.wineText }}
          >
            {t('pages.profile.sessions.title')}
          </span>
          <p className="text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
            {t('pages.profile.sessions.description', { count: sessions.length })}
          </p>
        </div>
        {sessions.length > 1 && (
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="outline" size="sm" disabled={revokingAll}>
                {revokingAll ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <LogOut className="mr-2 h-4 w-4" />
                )}
                {t('pages.profile.sessions.logoutAll')}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  {t('pages.profile.sessions.logoutAllConfirmTitle')}
                </AlertDialogTitle>
                <AlertDialogDescription>
                  {t('pages.profile.sessions.logoutAllConfirmDescription')}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                <AlertDialogAction onClick={handleLogoutAllDevices}>
                  {t('pages.profile.sessions.logoutAllConfirm')}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        )}
      </div>
      <div className="space-y-3 xs:space-y-4">
        {sessions.map((session) => {
          const device = getDeviceInfo(session.userAgent);
          const DeviceIcon = device.icon;
          const browser = getBrowserName(session.userAgent);

          return (
            <div
              key={session.sessionId}
              className="flex items-center justify-between rounded-xl xs:rounded-2xl border p-3 xs:p-4 transition-all"
              style={{
                backgroundColor: colors.wineBg,
                borderColor: colors.wineBorder,
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = colors.wineBgHover;
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = colors.wineBg;
              }}
            >
              <div className="flex items-center gap-3 xs:gap-4">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <DeviceIcon className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm xs:text-base">
                      {browser} - {device.label}
                    </span>
                    {session.isCurrent && (
                      <span
                        className="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] xs:text-xs font-medium text-white"
                        style={{ backgroundColor: colors.wine }}
                      >
                        {t('pages.profile.sessions.current')}
                      </span>
                    )}
                  </div>
                  <div
                    className="flex flex-wrap gap-2 text-[10px] xs:text-xs"
                    style={{ color: colors.wineTextMuted }}
                  >
                    <span>{session.ipAddress || t('pages.profile.sessions.unknownIP')}</span>
                    <span>•</span>
                    <span>
                      {t('pages.profile.sessions.lastActivity')}:{' '}
                      {formatRelativeTime(session.lastActivityAt)}
                    </span>
                  </div>
                </div>
              </div>
              {!session.isCurrent && (
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button variant="ghost" size="sm" disabled={revoking === session.sessionId}>
                      {revoking === session.sessionId ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <LogOut className="h-4 w-4" />
                      )}
                      <span className="sr-only">{t('pages.profile.sessions.revoke')}</span>
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>
                        {t('pages.profile.sessions.revokeConfirmTitle')}
                      </AlertDialogTitle>
                      <AlertDialogDescription>
                        {t('pages.profile.sessions.revokeConfirmDescription')}
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                      <AlertDialogAction onClick={() => handleRevokeSession(session.sessionId)}>
                        {t('pages.profile.sessions.revokeConfirm')}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              )}
            </div>
          );
        })}
        {sessions.length === 0 && (
          <p className="text-center text-sm py-4" style={{ color: colors.wineTextMuted }}>
            {t('pages.profile.sessions.noSessions')}
          </p>
        )}
      </div>
    </div>
  );
}
