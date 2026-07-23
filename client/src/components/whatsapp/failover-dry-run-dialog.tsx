import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  CoverageStatus,
  type DryRunReport,
  FailoverDirection,
} from '@/gen/failover/v1/failover_pb';
import { CheckCircle2, XCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface FailoverDryRunDialogProps {
  open: boolean;
  report: DryRunReport | null;
  direction: FailoverDirection;
  onClose: () => void;
  onContinue: () => void;
}

/** O ensaio expira em 15 minutos; passado isso, confirmar é proibido. */
function isExpired(report: DryRunReport): boolean {
  if (!report.expiresAt) return false;
  const expires = Date.parse(report.expiresAt);
  return Number.isFinite(expires) && expires <= Date.now();
}

function coverageLabel(t: (k: string) => string, status: CoverageStatus): string {
  switch (status) {
    case CoverageStatus.COMPLETE:
      return t('pages.whatsapp.failover.coverage.complete');
    case CoverageStatus.BLOCKED:
      return t('pages.whatsapp.failover.coverage.blocked');
    case CoverageStatus.UNAVAILABLE:
      return t('pages.whatsapp.failover.coverage.unavailable');
    default:
      return t('pages.whatsapp.failover.coverage.unavailable');
  }
}

/**
 * Relatório do ensaio.
 *
 * O ensaio NÃO altera nada: ele só lê o WhatsApp e diz se o chip reserva
 * consegue cobrir tudo o que a primária cobre hoje. É a tela que decide se o
 * botão de troca fica disponível.
 */
export function FailoverDryRunDialog({
  open,
  report,
  direction,
  onClose,
  onContinue,
}: FailoverDryRunDialogProps) {
  const { t } = useTranslation();
  if (!report) return null;

  const expired = isExpired(report);
  const canContinue = report.passed && !expired;

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {report.passed ? (
              <CheckCircle2 className="h-5 w-5 text-emerald-600" aria-hidden="true" />
            ) : (
              <XCircle className="h-5 w-5 text-destructive" aria-hidden="true" />
            )}
            {direction === FailoverDirection.REVERT
              ? t('pages.whatsapp.failover.dryRun.titleRevert')
              : t('pages.whatsapp.failover.dryRun.titleFailover')}
          </DialogTitle>
          <DialogDescription>{t('pages.whatsapp.failover.dryRun.description')}</DialogDescription>
        </DialogHeader>

        {expired && (
          <p className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
            {t('pages.whatsapp.failover.dryRun.expired')}
          </p>
        )}

        {!report.passed && report.blockingReasonCodes.length > 0 && (
          <div className="rounded-md bg-destructive/10 p-3" role="alert">
            <p className="text-sm font-medium text-destructive">
              {t('pages.whatsapp.failover.dryRun.blocked')}
            </p>
            <ul className="mt-1 list-inside list-disc text-xs text-destructive">
              {report.blockingReasonCodes.map((code) => (
                <li key={code}>{t(`pages.whatsapp.failover.reason.${code}`)}</li>
              ))}
            </ul>
          </div>
        )}

        <ScrollArea className="max-h-72">
          <div className="space-y-2 pr-3">
            {report.items.map((item) => (
              <div key={item.automationId} className="rounded-md border p-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="text-sm font-medium">
                    {item.automationName || item.automationId}
                  </span>
                  <Badge
                    variant={item.status === CoverageStatus.COMPLETE ? 'default' : 'destructive'}
                  >
                    {coverageLabel(t, item.status)}
                  </Badge>
                </div>

                <p className="mt-1 text-xs text-muted-foreground">
                  {t('pages.whatsapp.failover.dryRun.groupCounts', {
                    targets: item.requiredTargetGroups,
                    sources: item.requiredSourceGroups,
                  })}
                </p>

                {item.missingGroups.length > 0 && (
                  <p className="mt-1 text-xs text-destructive">
                    {t('pages.whatsapp.failover.dryRun.missingGroups')}:{' '}
                    {item.missingGroups.join(', ')}
                  </p>
                )}
                {item.missingAdminGroups.length > 0 && (
                  <p className="mt-1 text-xs text-destructive">
                    {t('pages.whatsapp.failover.dryRun.missingAdmin')}:{' '}
                    {item.missingAdminGroups.join(', ')}
                  </p>
                )}
              </div>
            ))}

            {report.items.length === 0 && (
              <p className="text-sm text-muted-foreground">
                {t('pages.whatsapp.failover.dryRun.noAutomations')}
              </p>
            )}
          </div>
        </ScrollArea>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            {t('common.close')}
          </Button>
          {/* Desabilitar é só UX: o backend recusa ensaio bloqueado/expirado. */}
          <Button disabled={!canContinue} onClick={onContinue}>
            {t('pages.whatsapp.failover.dryRun.continue')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
