import { ErrorBoundary } from '@/components/error-boundary';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Logo } from '@/components/ui/logo';
import { usePublicRedirect } from '@/hooks/use-public-redirect';
import { AlertCircle, Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * Public redirect page for WhatsApp group links.
 * Handles in-app browser detection and provides manual fallback options.
 * All logic is delegated to usePublicRedirect hook.
 */
export default function PublicRedirect() {
  const { t } = useTranslation();
  const {
    loading,
    error,
    groupName,
    redirectUrl,
    actionHint,
    isInAppBrowser,
    tryOpenWhatsApp,
    copyInviteLink,
    openInviteLink,
  } = usePublicRedirect();

  if (loading) {
    return (
      <ErrorBoundary>
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background to-secondary/20">
          <Card className="w-full max-w-md shadow-lg">
            <CardHeader className="text-center">
              <div className="mx-auto mb-4">
                <Logo />
              </div>
              <div className="mx-auto mb-4">
                <Loader2 className="h-12 w-12 animate-spin text-primary" aria-hidden="true" />
              </div>
              <CardTitle>{t('pages.publicRedirect.loading.title')}</CardTitle>
              <CardDescription>
                {t('pages.publicRedirect.loading.subtitle')}
                {groupName && ` "${groupName}"`}
              </CardDescription>
            </CardHeader>
            <CardContent className="text-center text-sm text-muted-foreground">
              <p>{t('pages.publicRedirect.loading.hint')}</p>
            </CardContent>
          </Card>
        </div>
      </ErrorBoundary>
    );
  }

  if (error) {
    const isGroupsFull = error.includes('cheios');
    return (
      <ErrorBoundary>
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background to-secondary/20 p-4">
          <Card className="w-full max-w-md shadow-lg">
            <CardHeader className="text-center">
              <div className="mx-auto mb-4">
                <Logo />
              </div>
              <div className="mx-auto mb-4">
                <AlertCircle className="h-12 w-12 text-destructive" aria-hidden="true" />
              </div>
              <CardTitle>{t('pages.publicRedirect.error.title')}</CardTitle>
              <CardDescription className="text-destructive">{error}</CardDescription>
            </CardHeader>
            <CardContent className="text-center space-y-4">
              <p className="text-sm text-muted-foreground">
                {isGroupsFull
                  ? t('pages.publicRedirect.error.groupsFull')
                  : t('pages.publicRedirect.error.genericError')}
              </p>
              <Button
                onClick={
                  isGroupsFull ? () => window.location.reload() : () => window.history.back()
                }
                variant={isGroupsFull ? 'default' : 'outline'}
                className="w-full"
              >
                {isGroupsFull
                  ? t('pages.publicRedirect.error.tryAgain')
                  : t('pages.publicRedirect.error.back')}
              </Button>
            </CardContent>
          </Card>
        </div>
      </ErrorBoundary>
    );
  }

  if (redirectUrl) {
    return (
      <ErrorBoundary>
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background to-secondary/20 p-4">
          <Card className="w-full max-w-md shadow-lg">
            <CardHeader className="text-center">
              <div className="mx-auto mb-4">
                <Logo />
              </div>
              <CardTitle>{t('pages.publicRedirect.manual.title')}</CardTitle>
              <CardDescription>
                {groupName
                  ? `${t('pages.publicRedirect.manual.groupLabel')} "${groupName}"`
                  : t('pages.publicRedirect.manual.tapToJoin')}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <Button onClick={tryOpenWhatsApp} className="w-full">
                {t('pages.publicRedirect.manual.openWhatsApp')}
              </Button>
              <Button onClick={copyInviteLink} variant="outline" className="w-full">
                {t('pages.publicRedirect.manual.copyLink')}
              </Button>
              <Button onClick={openInviteLink} variant="ghost" className="w-full">
                {t('pages.publicRedirect.manual.openInviteLink')}
              </Button>
              {actionHint && (
                <p className="text-xs text-muted-foreground text-center">{actionHint}</p>
              )}
              {isInAppBrowser && (
                <p className="text-xs text-muted-foreground text-center">
                  {t('pages.publicRedirect.manual.tiktokHint')}
                </p>
              )}
            </CardContent>
          </Card>
        </div>
      </ErrorBoundary>
    );
  }

  return null;
}
