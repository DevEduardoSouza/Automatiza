import { Alert, AlertDescription } from '@/components/ui/alert';
import { useAccountSuspended } from '@/hooks/use-account-suspended';
import { useTranslation } from 'react-i18next';

/**
 * Banner de conta suspensa por cobranca. Exibe alerta no topo quando a conta
 * esta suspensa. A deteccao e o polling ficam em useAccountSuspended (mesma
 * fonte usada pelo gate de conteudo no AppLayout).
 */
export function AccountSuspendedBanner() {
  const { t } = useTranslation();
  const { suspended } = useAccountSuspended();

  if (!suspended) {
    return null;
  }

  return (
    <div className="px-4 md:px-6 pt-4">
      <Alert variant="destructive">
        <AlertDescription>
          <div className="space-y-1">
            <div className="font-semibold">{t('billing.suspended.title')}</div>
            <div>{t('billing.suspended.description')}</div>
          </div>
        </AlertDescription>
      </Alert>
    </div>
  );
}
