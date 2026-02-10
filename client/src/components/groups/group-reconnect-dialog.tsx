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
        <div className="flex flex-col items-center space-y-4">
          {qrCode ? (
            <img
              src={qrCode}
              alt={t('pages.groups.whatsapp.qrCodeAlt')}
              className="w-64 h-64 border-2 border-border rounded-lg"
            />
          ) : (
            <div className="w-64 h-64 flex items-center justify-center border-2 border-dashed border-border rounded-lg">
              <p className="text-muted-foreground">{t('pages.groups.whatsapp.generatingQr')}</p>
            </div>
          )}
          <p className="text-sm text-muted-foreground text-center">
            {t('pages.groups.whatsapp.waitingScan')}
          </p>
        </div>
      </DialogContent>
    </Dialog>
  );
}
