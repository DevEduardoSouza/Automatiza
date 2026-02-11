import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AlertCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface Group {
  id: string;
  name: string;
  invite_code: string | null;
}

interface GroupInviteCodeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedGroup: Group | null;
  inviteCode: string;
  onInviteCodeChange: (value: string) => void;
  onSubmit: () => void;
}

/**
 * Dialog para editar código de convite do grupo.
 */
export function GroupInviteCodeDialog({
  open,
  onOpenChange,
  selectedGroup,
  inviteCode,
  onInviteCodeChange,
  onSubmit,
}: GroupInviteCodeDialogProps) {
  const { t } = useTranslation();

  const detectedCode = inviteCode.match(/chat\.whatsapp\.com\/([A-Za-z0-9]+)/)?.[1];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('pages.groups.inviteCode.dialogTitle')}</DialogTitle>
          <DialogDescription>
            {t('pages.groups.inviteCode.dialogDescription', {
              name: selectedGroup?.name,
            })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <Alert>
            <AlertCircle className="h-4 w-4" aria-hidden="true" />
            <AlertDescription>{t('pages.groups.inviteCode.hint')}</AlertDescription>
          </Alert>
          <div className="space-y-2">
            <Label htmlFor="inviteCode">{t('pages.groups.inviteCode.label')}</Label>
            <Input
              id="inviteCode"
              placeholder={t('pages.groups.inviteCode.placeholder')}
              value={inviteCode}
              onChange={(e) => onInviteCodeChange(e.target.value)}
            />
            {inviteCode?.includes('chat.whatsapp.com/') && (
              <p className="text-xs text-muted-foreground">
                {t('pages.groups.inviteCode.detectedCode', {
                  code: detectedCode,
                })}
              </p>
            )}
          </div>
          <button
            type="button"
            onClick={onSubmit}
            disabled={!inviteCode.trim()}
            className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
              <div className="auth-btn-glow__glow absolute inset-0" />
            </div>
            <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
              {t('pages.groups.inviteCode.save')}
            </span>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
