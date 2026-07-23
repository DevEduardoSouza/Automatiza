import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { FailoverMode, type FailoverPolicy } from '@/gen/failover/v1/failover_pb';
import { useFailover } from '@/hooks/use-failover';
import type { LocalInstance } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { Loader2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface FailoverPolicyDialogProps {
  open: boolean;
  /** null = criar. */
  policy: FailoverPolicy | null;
  instances: LocalInstance[];
  onClose: () => void;
}

/**
 * Criação/edição da política de chip reserva.
 *
 * Duas restrições que a tela impõe e o backend reafirma:
 *   - a reserva só pode ser um chip do próprio usuário, diferente da primária;
 *   - a primária é imutável depois de criada (trocar exige apagar a política e
 *     criar outra), porque é ela que carrega o histórico de quem servia o quê.
 *
 * E uma que não existe: modo automático. Só Manual e Assistido.
 */
export function FailoverPolicyDialog({
  open,
  policy,
  instances,
  onClose,
}: FailoverPolicyDialogProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const failover = useFailover();

  const isEdit = policy !== null;
  const [primaryId, setPrimaryId] = useState('');
  const [backupId, setBackupId] = useState('');
  const [mode, setMode] = useState<FailoverMode>(FailoverMode.MANUAL);

  useEffect(() => {
    if (!open) return;
    setPrimaryId(policy ? String(policy.primaryInstanceId) : '');
    setBackupId(policy ? String(policy.backupInstanceId) : '');
    setMode(policy ? policy.mode : FailoverMode.MANUAL);
  }, [open, policy]);

  const valid = primaryId !== '' && backupId !== '' && primaryId !== backupId;

  const handleSave = async () => {
    try {
      await failover.setPolicy({
        policyId: policy?.id,
        primaryInstanceId: Number(primaryId),
        backupInstanceId: Number(backupId),
        mode,
        expectedVersion: policy?.version ?? 0n,
      });
      onClose();
    } catch (err) {
      toast({
        title: t('pages.whatsapp.failover.saveFailed'),
        description: (err as Error).message,
        variant: 'destructive',
      });
    }
  };

  const label = (i: LocalInstance) => i.friendly_name || i.instance_id || `#${i.id}`;

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o && !failover.isMutating) onClose();
      }}
    >
      <DialogContent
        onEscapeKeyDown={(e) => {
          if (failover.isMutating) e.preventDefault();
        }}
      >
        <DialogHeader>
          <DialogTitle>
            {isEdit
              ? t('pages.whatsapp.failover.dialog.editTitle')
              : t('pages.whatsapp.failover.dialog.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('pages.whatsapp.failover.dialog.description')}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="failover-primary">
              {t('pages.whatsapp.failover.dialog.primaryLabel')}
            </Label>
            <Select value={primaryId} onValueChange={setPrimaryId} disabled={isEdit}>
              <SelectTrigger id="failover-primary">
                <SelectValue placeholder={t('pages.whatsapp.failover.dialog.selectChip')} />
              </SelectTrigger>
              <SelectContent>
                {instances.map((i) => (
                  <SelectItem key={i.id} value={String(i.id)}>
                    {label(i)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {isEdit && (
              <p className="text-xs text-muted-foreground">
                {t('pages.whatsapp.failover.dialog.primaryImmutable')}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="failover-backup">
              {t('pages.whatsapp.failover.dialog.backupLabel')}
            </Label>
            <Select value={backupId} onValueChange={setBackupId}>
              <SelectTrigger id="failover-backup">
                <SelectValue placeholder={t('pages.whatsapp.failover.dialog.selectChip')} />
              </SelectTrigger>
              <SelectContent>
                {/* A primária nunca aparece como opção de reserva. */}
                {instances
                  .filter((i) => String(i.id) !== primaryId)
                  .map((i) => (
                    <SelectItem key={i.id} value={String(i.id)}>
                      {label(i)}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="failover-mode">{t('pages.whatsapp.failover.dialog.modeLabel')}</Label>
            <Select value={String(mode)} onValueChange={(v) => setMode(Number(v) as FailoverMode)}>
              <SelectTrigger id="failover-mode">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={String(FailoverMode.MANUAL)}>
                  {t('pages.whatsapp.failover.mode.manual')}
                </SelectItem>
                <SelectItem value={String(FailoverMode.ASSISTED)}>
                  {t('pages.whatsapp.failover.mode.assisted')}
                </SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              {t('pages.whatsapp.failover.dialog.modeHelp')}
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" disabled={failover.isMutating} onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button disabled={!valid || failover.isMutating} onClick={handleSave}>
            {failover.isMutating && (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
            )}
            {t('common.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
