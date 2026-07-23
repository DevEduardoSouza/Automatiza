import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  type DryRunReport,
  FailoverDirection,
  type FailoverPolicy,
} from '@/gen/failover/v1/failover_pb';
import { useFailover } from '@/hooks/use-failover';
import { useToast } from '@/hooks/use-toast';
import { AlertTriangle, Loader2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface FailoverConfirmDialogProps {
  open: boolean;
  report: DryRunReport | null;
  policy: FailoverPolicy | null;
  direction: FailoverDirection;
  onClose: () => void;
}

/** Palavra que o usuário digita para liberar a ação. */
const CONFIRM_WORD_SWITCH = 'TROCAR';
const CONFIRM_WORD_REVERT = 'REVERTER';

/**
 * Confirmação forte da troca (ou da reversão).
 *
 * Digitar a palavra existe porque esta ação muda por qual número os grupos do
 * cliente passam a receber — não é um botão que se aperta por engano ao
 * navegar com o teclado.
 *
 * Escape fica bloqueado enquanto a mutação está em voo: fechar a janela não
 * cancela nada no servidor, e sumir com a tela no meio só faria o usuário achar
 * que nada aconteceu.
 */
export function FailoverConfirmDialog({
  open,
  report,
  policy,
  direction,
  onClose,
}: FailoverConfirmDialogProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const failover = useFailover();
  const [typed, setTyped] = useState('');

  useEffect(() => {
    if (open) setTyped('');
  }, [open]);

  if (!report || !policy) return null;

  const isRevert = direction === FailoverDirection.REVERT;
  const word = isRevert ? CONFIRM_WORD_REVERT : CONFIRM_WORD_SWITCH;
  const armed = typed.trim().toUpperCase() === word;

  const handleConfirm = async () => {
    const params = {
      policyId: policy.id,
      expectedVersion: policy.version,
      dryRunId: report.dryRunId,
      dryRunRef: report.dryRunRef,
    };
    try {
      const op = isRevert
        ? await failover.revertFailover(params)
        : await failover.confirmFailover(params);
      toast({
        title: isRevert
          ? t('pages.whatsapp.failover.confirm.revertDone')
          : t('pages.whatsapp.failover.confirm.switchDone'),
        description: op?.operationId,
      });
      onClose();
    } catch (err) {
      toast({
        title: t('pages.whatsapp.failover.confirm.failed'),
        description: (err as Error).message,
        variant: 'destructive',
      });
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        // Enquanto envia, a janela não fecha: a operação continua no servidor.
        if (!o && !failover.isMutating) onClose();
      }}
    >
      <DialogContent
        onEscapeKeyDown={(e) => {
          if (failover.isMutating) e.preventDefault();
        }}
        onInteractOutside={(e) => {
          if (failover.isMutating) e.preventDefault();
        }}
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-amber-600" aria-hidden="true" />
            {isRevert
              ? t('pages.whatsapp.failover.confirm.titleRevert')
              : t('pages.whatsapp.failover.confirm.titleSwitch')}
          </DialogTitle>
          <DialogDescription>
            {isRevert
              ? t('pages.whatsapp.failover.confirm.descriptionRevert')
              : t('pages.whatsapp.failover.confirm.descriptionSwitch', {
                  count: report.items.length,
                })}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label htmlFor="failover-confirm-word">
            {t('pages.whatsapp.failover.confirm.typeToConfirm', { word })}
          </Label>
          <Input
            id="failover-confirm-word"
            value={typed}
            autoFocus
            autoComplete="off"
            disabled={failover.isMutating}
            onChange={(e) => setTyped(e.target.value)}
            aria-describedby="failover-confirm-help"
          />
          <p id="failover-confirm-help" className="text-xs text-muted-foreground">
            {t('pages.whatsapp.failover.confirm.help')}
          </p>
        </div>

        <DialogFooter>
          <Button variant="ghost" disabled={failover.isMutating} onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button disabled={!armed || failover.isMutating} onClick={handleConfirm}>
            {failover.isMutating && (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
            )}
            {isRevert
              ? t('pages.whatsapp.failover.confirm.revertAction')
              : t('pages.whatsapp.failover.confirm.switchAction')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
