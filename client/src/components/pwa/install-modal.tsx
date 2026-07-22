import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import type { InstallState } from '@/pwa/use-pwa-install';
import { useTranslation } from 'react-i18next';

interface InstallModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  state: InstallState;
}

// Instruções por plataforma quando não há prompt nativo de instalação.
// Nunca é um "botão morto": cada estado mostra o caminho manual correto.
export function InstallModal({ open, onOpenChange, state }: InstallModalProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('pwa.install.title')}</DialogTitle>
          <DialogDescription>{t('pwa.install.subtitle')}</DialogDescription>
        </DialogHeader>

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
      </DialogContent>
    </Dialog>
  );
}
