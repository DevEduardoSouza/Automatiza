import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useTranslation } from 'react-i18next';

interface GroupReconnectDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  instanceName: string;
  qrCode: string | null;
}

/**
 * Dialog para reconexão do WhatsApp via QR Code.
 */
export function GroupReconnectDialog({
  open,
  onOpenChange,
  instanceName,
  qrCode,
}: GroupReconnectDialogProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('pages.groups.whatsapp.reconnectTitle')}</DialogTitle>
          <DialogDescription>
            {t('pages.groups.whatsapp.reconnectDescription', { instanceName })}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col items-center space-y-4 xs:space-y-5 sm:space-y-6">
          {qrCode ? (
            <img
              src={qrCode}
              alt={t('pages.groups.whatsapp.qrCodeAlt')}
              className="w-48 h-48 xs:w-56 xs:h-56 sm:w-64 sm:h-64 md:w-72 md:h-72 border-2 border-border rounded-lg sm:rounded-xl"
            />
          ) : (
            <div className="w-48 h-48 xs:w-56 xs:h-56 sm:w-64 sm:h-64 md:w-72 md:h-72 flex items-center justify-center border-2 border-dashed border-border rounded-lg sm:rounded-xl">
              <p className="text-muted-foreground">{t('pages.groups.whatsapp.generatingQr')}</p>
            </div>
          )}
          <p className="text-sm xs:text-base text-muted-foreground text-center">
            {t('pages.groups.whatsapp.waitingScan')}
          </p>
        </div>
      </DialogContent>
    </Dialog>
  );
}
