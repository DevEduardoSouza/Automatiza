import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { AlertCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * Full-content notice shown in place of a page's body when the account is
 * suspended. Replacing the page content (instead of rendering it under a
 * banner) stops the page's data hooks/pollers from mounting — those endpoints
 * return PermissionDenied while suspended and were retried in a tight loop.
 */
export function AccountSuspendedNotice() {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-[50vh] items-center justify-center">
      <Alert variant="destructive" className="max-w-md">
        <AlertCircle className="h-5 w-5" />
        <AlertTitle>{t('billing.suspended.title')}</AlertTitle>
        <AlertDescription>{t('billing.suspended.description')}</AlertDescription>
      </Alert>
    </div>
  );
}
