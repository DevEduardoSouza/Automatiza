import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import type { InstallState } from '@/pwa/use-pwa-install';
import { useWebPush } from '@/pwa/use-web-push';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';

interface InstallModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  state: InstallState;
}

// Hub de instalação + notificações. Nunca é "botão morto": cada estado mostra o
// caminho manual correto (instalar) e o opt-in de notificações quando cabível.
export function InstallModal({ open, onOpenChange, state }: InstallModalProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('pwa.install.title')}</DialogTitle>
          <DialogDescription>{t('pwa.install.subtitle')}</DialogDescription>
        </DialogHeader>

        {state !== 'installed' && (
          <section className="space-y-2">
            {state === 'ios-manual' && (
              <ol className="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
                <li>{t('pwa.install.iosStep1')}</li>
                <li>{t('pwa.install.iosStep2')}</li>
                <li>{t('pwa.install.iosStep3')}</li>
              </ol>
            )}
            {state === 'chromium-manual' && (
              <p className="text-sm text-muted-foreground">{t('pwa.install.chromiumHint')}</p>
            )}
            {state === 'unsupported' && (
              <p className="text-sm text-muted-foreground">{t('pwa.install.unsupported')}</p>
            )}
          </section>
        )}

        <NotificationsSection />
      </DialogContent>
    </Dialog>
  );
}

function NotificationsSection() {
  const { t } = useTranslation();
  const { state, subscribe, unsubscribe, sendTest, busy } = useWebPush();

  // Sem suporte ou feature desligada no servidor: não mostra nada (degradação).
  if (state === 'unsupported' || state === 'disabled-server') return null;

  return (
    <section className="mt-4 space-y-2 border-t pt-4">
      <h3 className="text-sm font-semibold">{t('pwa.push.title')}</h3>

      {state === 'needs-install' && (
        <p className="text-sm text-muted-foreground">{t('pwa.push.needsInstall')}</p>
      )}

      {state === 'denied' && (
        <p className="text-sm text-muted-foreground">{t('pwa.push.denied')}</p>
      )}

      {state === 'available' && (
        <>
          <p className="text-sm text-muted-foreground">{t('pwa.push.pitch')}</p>
          <Button
            size="sm"
            disabled={busy}
            onClick={async () => {
              const ok = await subscribe();
              if (ok) {
                toast.success(t('pwa.push.enabled'));
                await sendTest().catch(() => {});
              } else {
                toast.error(t('pwa.push.failed'));
              }
            }}
          >
            {t('pwa.push.enable')}
          </Button>
        </>
      )}

      {state === 'subscribed' && (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm text-muted-foreground">{t('pwa.push.active')}</span>
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={async () => {
              const n = await sendTest().catch(() => 0);
              toast[n > 0 ? 'success' : 'error'](
                n > 0 ? t('pwa.push.testSent') : t('pwa.push.failed')
              );
            }}
          >
            {t('pwa.push.test')}
          </Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={() => unsubscribe()}>
            {t('pwa.push.disable')}
          </Button>
        </div>
      )}
    </section>
  );
}
