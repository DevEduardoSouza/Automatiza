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
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background to-secondary/20 p-3 xs:p-4 sm:p-6 md:p-8">
          <Card className="w-full max-w-[calc(100%-0.5rem)] xs:max-w-md lg:max-w-lg shadow-md xs:shadow-lg lg:shadow-xl rounded-xl xs:rounded-2xl lg:rounded-3xl">
            <CardHeader className="text-center p-4 xs:p-5 sm:p-6 lg:p-8">
              <div className="mx-auto mb-3 xs:mb-4 sm:mb-5">
                <Logo className="h-8 xs:h-10 sm:h-12 w-auto object-contain" />
              </div>
              <div className="mx-auto mb-3 xs:mb-4 sm:mb-5">
                <Loader2 className="h-8 w-8 xs:h-10 xs:w-10 sm:h-12 sm:w-12 animate-spin text-primary" aria-hidden="true" />
              </div>
              <CardTitle className="text-lg xs:text-xl sm:text-2xl lg:text-3xl">{t('pages.publicRedirect.loading.title')}</CardTitle>
              <CardDescription className="text-xs xs:text-sm sm:text-base">
                {t('pages.publicRedirect.loading.subtitle')}
                {groupName && ` "${groupName}"`}
              </CardDescription>
            </CardHeader>
            <CardContent className="text-center text-xs xs:text-sm text-muted-foreground p-4 xs:p-5 sm:p-6 pt-0">
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
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background to-secondary/20 p-3 xs:p-4 sm:p-6 md:p-8">
          <Card className="w-full max-w-[calc(100%-0.5rem)] xs:max-w-md lg:max-w-lg shadow-md xs:shadow-lg lg:shadow-xl rounded-xl xs:rounded-2xl lg:rounded-3xl">
            <CardHeader className="text-center p-4 xs:p-5 sm:p-6 lg:p-8">
              <div className="mx-auto mb-3 xs:mb-4 sm:mb-5">
                <Logo className="h-8 xs:h-10 sm:h-12 w-auto object-contain" />
              </div>
              <div className="mx-auto mb-3 xs:mb-4 sm:mb-5">
                <AlertCircle className="h-8 w-8 xs:h-10 xs:w-10 sm:h-12 sm:w-12 text-destructive" aria-hidden="true" />
              </div>
              <CardTitle className="text-lg xs:text-xl sm:text-2xl lg:text-3xl">{t('pages.publicRedirect.error.title')}</CardTitle>
              <CardDescription className="text-xs xs:text-sm sm:text-base text-destructive">{error}</CardDescription>
            </CardHeader>
            <CardContent className="text-center space-y-3 xs:space-y-4 p-4 xs:p-5 sm:p-6 pt-0">
              <p className="text-xs xs:text-sm text-muted-foreground">
                {isGroupsFull
                  ? t('pages.publicRedirect.error.groupsFull')
                  : t('pages.publicRedirect.error.genericError')}
              </p>
              <Button
                onClick={
                  isGroupsFull ? () => window.location.reload() : () => window.history.back()
                }
                variant={isGroupsFull ? 'default' : 'outline'}
                className="w-full text-sm xs:text-base"
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
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background to-secondary/20 p-3 xs:p-4 sm:p-6 md:p-8">
          <Card className="w-full max-w-[calc(100%-0.5rem)] xs:max-w-md lg:max-w-lg shadow-md xs:shadow-lg lg:shadow-xl rounded-xl xs:rounded-2xl lg:rounded-3xl">
            <CardHeader className="text-center p-4 xs:p-5 sm:p-6 lg:p-8">
              <div className="mx-auto mb-3 xs:mb-4 sm:mb-5">
                <Logo className="h-8 xs:h-10 sm:h-12 w-auto object-contain" />
              </div>
              <CardTitle className="text-lg xs:text-xl sm:text-2xl lg:text-3xl">{t('pages.publicRedirect.manual.title')}</CardTitle>
              <CardDescription className="text-xs xs:text-sm sm:text-base">
                {groupName
                  ? `${t('pages.publicRedirect.manual.groupLabel')} "${groupName}"`
                  : t('pages.publicRedirect.manual.tapToJoin')}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-2.5 xs:space-y-3 sm:space-y-4 p-4 xs:p-5 sm:p-6 pt-0">
              <Button onClick={tryOpenWhatsApp} className="w-full text-sm xs:text-base">
                {t('pages.publicRedirect.manual.openWhatsApp')}
              </Button>
              <Button onClick={copyInviteLink} variant="outline" className="w-full text-sm xs:text-base">
                {t('pages.publicRedirect.manual.copyLink')}
              </Button>
              <Button onClick={openInviteLink} variant="ghost" className="w-full text-sm xs:text-base">
                {t('pages.publicRedirect.manual.openInviteLink')}
              </Button>
              {actionHint && (
                <p className="text-[11px] xs:text-xs sm:text-sm text-muted-foreground text-center">{actionHint}</p>
              )}
              {isInAppBrowser && (
                <p className="text-[11px] xs:text-xs sm:text-sm text-muted-foreground text-center">
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
