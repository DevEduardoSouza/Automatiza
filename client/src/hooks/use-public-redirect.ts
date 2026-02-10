import { redirectService } from '@/services/api.service';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useParams } from 'react-router';

interface RedirectResponse {
  success: boolean;
  group_name?: string;
  redirect_url?: string;
  error?: string;
}

interface UsePublicRedirectReturn {
  loading: boolean;
  error: string | null;
  groupName: string;
  redirectUrl: string;
  actionHint: string;
  isInAppBrowser: boolean;
  inviteCode: string;
  isIOS: boolean;
  isAndroid: boolean;
  tryOpenWhatsApp: () => void;
  copyInviteLink: () => Promise<void>;
  openInviteLink: () => void;
}

/**
 * Hook for public redirect page logic
 * Manages WhatsApp group redirect with in-app browser detection
 */
export function usePublicRedirect(): UsePublicRedirectReturn {
  const { t } = useTranslation();
  const { slug } = useParams<{ slug: string }>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [groupName, setGroupName] = useState<string>('');
  const [redirectUrl, setRedirectUrl] = useState<string>('');
  const [actionHint, setActionHint] = useState<string>('');

  const ua = useMemo(() => (typeof navigator !== 'undefined' ? navigator.userAgent || '' : ''), []);

  const referrer = useMemo(
    () => (typeof document !== 'undefined' ? document.referrer || '' : ''),
    []
  );

  const isInAppBrowser = useMemo(() => {
    const byUa =
      /TikTok|Musical\.ly|TTWebView|ByteDance|Aweme|trill|Instagram|FBAN|FBAV|FB_IAB|Line|Twitter|Snapchat/i.test(
        ua
      );
    const byReferrer = /tiktok\.com|tiktoklinksafety\.com/i.test(referrer);
    return byUa || byReferrer;
  }, [ua, referrer]);

  const inviteCode = useMemo(() => {
    if (!redirectUrl) return '';
    const match = redirectUrl.match(/chat\.whatsapp\.com\/([A-Za-z0-9]+)/i);
    return match?.[1] || '';
  }, [redirectUrl]);

  const isIOS = useMemo(() => /iPhone|iPad|iPod/i.test(ua), [ua]);
  const isAndroid = useMemo(() => /Android/i.test(ua), [ua]);

  // Translate backend error messages to user-friendly messages
  const translateError = useCallback(
    (backendError: string): string => {
      const errorMap: Record<string, string> = {
        'link not found': t('pages.publicRedirect.error.invalidLink'),
        'link is inactive': t('pages.publicRedirect.error.linkInactive'),
        'no target groups configured': t('pages.publicRedirect.error.noGroups'),
        'no available groups (all groups are full or inactive)': t(
          'pages.publicRedirect.error.groupsFull'
        ),
        'group not found': t('pages.publicRedirect.error.groupNotFound'),
        'instance not found': t('pages.publicRedirect.error.instanceNotFound'),
        'bot is not admin in group': t('pages.publicRedirect.error.notAdmin'),
        'failed to get invite code': t('pages.publicRedirect.error.inviteCodeFailed'),
        'failed to get invite code - check if WhatsApp session is connected': t(
          'pages.publicRedirect.error.sessionDisconnected'
        ),
        'invite code not available': t('pages.publicRedirect.error.inviteCodeNotAvailable'),
        'WhatsApp provider not configured': t('pages.publicRedirect.error.providerNotConfigured'),
      };
      return errorMap[backendError] || backendError;
    },
    [t]
  );

  const handleRedirect = useCallback(async () => {
    try {
      const data: RedirectResponse = await redirectService.processRedirectBySlug(slug as string);

      if (!data?.success) {
        throw new Error(translateError(data?.error || '') || t('pages.publicRedirect.error.genericError'));
      }

      setGroupName(data.group_name || '');
      setRedirectUrl(data.redirect_url || '');

      if (!isInAppBrowser && data.redirect_url) {
        window.location.href = data.redirect_url;
        setTimeout(() => setLoading(false), 2500);
        return;
      }

      setLoading(false);
    } catch (err: unknown) {
      const errorMessage =
        err instanceof Error ? err.message : t('pages.publicRedirect.error.genericError');
      setError(errorMessage);
      setLoading(false);
    }
  }, [slug, t, isInAppBrowser, translateError]);

  useEffect(() => {
    if (!slug) {
      setError(t('pages.publicRedirect.error.invalidLink'));
      setLoading(false);
      return;
    }

    handleRedirect();
  }, [slug, t, handleRedirect]);

  const tryOpenWhatsApp = useCallback(() => {
    if (!redirectUrl) return;

    setActionHint('');

    const trySchemeWithoutNavigating = (url: string) => {
      try {
        const iframe = document.createElement('iframe');
        iframe.style.display = 'none';
        iframe.src = url;
        document.body.appendChild(iframe);
        setTimeout(() => {
          try {
            document.body.removeChild(iframe);
          } catch {
            // ignore
          }
        }, 1500);
      } catch {
        // ignore
      }
    };

    if (inviteCode) {
      trySchemeWithoutNavigating(`whatsapp://chat?code=${inviteCode}`);
      trySchemeWithoutNavigating(`whatsapp://invite?code=${inviteCode}`);
      trySchemeWithoutNavigating(`whatsapp://invite/${inviteCode}`);

      if (isAndroid) {
        try {
          window.location.href = `intent://chat.whatsapp.com/${inviteCode}#Intent;scheme=https;package=com.whatsapp;end`;
          return;
        } catch {
          // ignore
        }
      }
    }

    setTimeout(() => {
      if (document.visibilityState === 'visible') {
        setActionHint(
          isIOS
            ? t('pages.publicRedirect.manual.iosHint')
            : t('pages.publicRedirect.manual.androidHint')
        );
      }
    }, 1200);
  }, [redirectUrl, inviteCode, isAndroid, isIOS, t]);

  const copyInviteLink = useCallback(async () => {
    if (!redirectUrl) return;
    try {
      await navigator.clipboard.writeText(redirectUrl);
    } catch {
      const ta = document.createElement('textarea');
      ta.value = redirectUrl;
      ta.style.position = 'fixed';
      ta.style.left = '-9999px';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    }
  }, [redirectUrl]);

  const openInviteLink = useCallback(() => {
    if (!redirectUrl) return;
    try {
      const win = window.open(redirectUrl, '_blank', 'noopener,noreferrer');
      if (!win) window.location.href = redirectUrl;
    } catch {
      window.location.href = redirectUrl;
    }
  }, [redirectUrl]);

  return {
    loading,
    error,
    groupName,
    redirectUrl,
    actionHint,
    isInAppBrowser,
    inviteCode,
    isIOS,
    isAndroid,
    tryOpenWhatsApp,
    copyInviteLink,
    openInviteLink,
  };
}
