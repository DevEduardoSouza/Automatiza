import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  type DryRunReport,
  FailoverDirection,
  FailoverMode,
  type FailoverPolicy,
  FailoverState,
} from '@/gen/failover/v1/failover_pb';
import { useFailover } from '@/hooks/use-failover';
import type { LocalInstance } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { AlertTriangle, Loader2, ShieldCheck } from 'lucide-react';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { FailoverConfirmDialog } from './failover-confirm-dialog';
import { FailoverDryRunDialog } from './failover-dry-run-dialog';
import { FailoverEventList } from './failover-event-list';
import { FailoverPolicyDialog } from './failover-policy-dialog';

interface FailoverPolicyCardProps {
  instances: LocalInstance[];
}

/** Rótulos de estado. Não existe "automático" em lugar nenhum da UI. */
function stateLabel(t: (k: string) => string, state: FailoverState): string {
  switch (state) {
    case FailoverState.ACTIVE:
      return t('pages.whatsapp.failover.state.active');
    case FailoverState.PROPOSED:
      return t('pages.whatsapp.failover.state.proposed');
    case FailoverState.SWITCHING:
      return t('pages.whatsapp.failover.state.switching');
    case FailoverState.ACTIVE_ON_BACKUP:
      return t('pages.whatsapp.failover.state.activeOnBackup');
    case FailoverState.REVERTING:
      return t('pages.whatsapp.failover.state.reverting');
    case FailoverState.FAILED:
      return t('pages.whatsapp.failover.state.failed');
    default:
      return t('pages.whatsapp.failover.state.unknown');
  }
}

function stateVariant(state: FailoverState): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (state) {
    case FailoverState.ACTIVE:
      return 'default';
    case FailoverState.ACTIVE_ON_BACKUP:
    case FailoverState.PROPOSED:
      return 'secondary';
    case FailoverState.FAILED:
      return 'destructive';
    default:
      return 'outline';
  }
}

/** Dados que a confirmação precisa, resolvidos pelo ensaio. */
interface PendingSwitch {
  policy: FailoverPolicy;
  report: DryRunReport;
  direction: FailoverDirection;
}

/**
 * Seção de chip reserva da tela /whatsapp.
 *
 * Regra de visibilidade: a seção some APENAS quando o backend devolve o reason
 * code de feature desligada. Erro de rede ou 500 aparecem como erro — nunca
 * como "o recurso não existe".
 */
export function FailoverPolicyCard({ instances }: FailoverPolicyCardProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const failover = useFailover();

  const [policyDialogOpen, setPolicyDialogOpen] = useState(false);
  const [editing, setEditing] = useState<FailoverPolicy | null>(null);

  // Fluxo linear: ensaio → revisão → confirmação forte. O diálogo de
  // confirmação só abre depois que o usuário leu o relatório.
  const [pending, setPending] = useState<PendingSwitch | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const instanceLabel = useCallback(
    (id: bigint) => {
      const found = instances.find((i) => String(i.id) === String(id));
      return found?.friendly_name || found?.instance_id || `#${id}`;
    },
    [instances]
  );

  const handleDryRun = useCallback(
    async (policy: FailoverPolicy, direction: FailoverDirection) => {
      try {
        const report = await failover.runDryRun(policy.id, direction);
        if (report) {
          setPending({ policy, report, direction });
          setConfirmOpen(false);
        }
      } catch (err) {
        toast({
          title: t('pages.whatsapp.failover.dryRunFailed'),
          description: (err as Error).message,
          variant: 'destructive',
        });
      }
    },
    [failover, t, toast]
  );

  // Feature desligada: a seção inteira some, sem deixar rastro na UI.
  if (failover.disabled) return null;

  if (failover.error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <AlertTriangle className="h-4 w-4 text-destructive" aria-hidden="true" />
            {t('pages.whatsapp.failover.title')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-destructive" role="alert">
            {t('pages.whatsapp.failover.loadError')}: {failover.error.message}
          </p>
          <Button variant="outline" size="sm" className="mt-3" onClick={() => failover.refetch()}>
            {t('common.retry')}
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-2 space-y-0">
          <CardTitle className="flex items-center gap-2 text-base">
            <ShieldCheck className="h-4 w-4" aria-hidden="true" />
            {t('pages.whatsapp.failover.title')}
          </CardTitle>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setEditing(null);
              setPolicyDialogOpen(true);
            }}
            disabled={instances.length < 2}
          >
            {t('pages.whatsapp.failover.newPolicy')}
          </Button>
        </CardHeader>

        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            {t('pages.whatsapp.failover.description')}
          </p>

          {failover.loading && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              {t('common.loading')}
            </div>
          )}

          {!failover.loading && failover.policies.length === 0 && (
            <p className="text-sm text-muted-foreground">{t('pages.whatsapp.failover.empty')}</p>
          )}

          {failover.policies.map((policy) => {
            const busy =
              policy.state === FailoverState.SWITCHING || policy.state === FailoverState.REVERTING;
            const onBackup = policy.state === FailoverState.ACTIVE_ON_BACKUP;
            const canSwitch =
              policy.state === FailoverState.ACTIVE || policy.state === FailoverState.PROPOSED;

            return (
              <div key={String(policy.id)} className="rounded-lg border p-4 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="space-y-1">
                    <p className="text-sm font-medium">
                      {instanceLabel(policy.primaryInstanceId)} →{' '}
                      {instanceLabel(policy.backupInstanceId)}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {t('pages.whatsapp.failover.version')}: {String(policy.version)}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant={stateVariant(policy.state)}>
                      {stateLabel(t, policy.state)}
                    </Badge>
                    <Badge variant="outline">
                      {policy.mode === FailoverMode.ASSISTED
                        ? t('pages.whatsapp.failover.mode.assisted')
                        : t('pages.whatsapp.failover.mode.manual')}
                    </Badge>
                  </div>
                </div>

                {policy.state === FailoverState.PROPOSED && (
                  <p className="text-xs text-amber-600 dark:text-amber-500" role="status">
                    {t('pages.whatsapp.failover.proposalNotice')}
                  </p>
                )}

                {policy.state === FailoverState.FAILED && (
                  <p className="text-xs text-destructive" role="alert">
                    {t('pages.whatsapp.failover.failedNotice')}
                    {policy.currentOperationId ? ` (${policy.currentOperationId})` : ''}
                  </p>
                )}

                {busy && (
                  <p
                    className="flex items-center gap-2 text-xs text-muted-foreground"
                    role="status"
                  >
                    <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
                    {t('pages.whatsapp.failover.inProgress')}
                  </p>
                )}

                <div className="flex flex-wrap gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!canSwitch || failover.isMutating}
                    onClick={() => handleDryRun(policy, FailoverDirection.FAILOVER)}
                  >
                    {failover.isMutating && (
                      <Loader2 className="mr-2 h-3 w-3 animate-spin" aria-hidden="true" />
                    )}
                    {t('pages.whatsapp.failover.testSwitch')}
                  </Button>

                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!onBackup || failover.isMutating}
                    onClick={() => handleDryRun(policy, FailoverDirection.REVERT)}
                  >
                    {t('pages.whatsapp.failover.testRevert')}
                  </Button>

                  <Button
                    size="sm"
                    variant="ghost"
                    disabled={busy}
                    onClick={() => {
                      setEditing(policy);
                      setPolicyDialogOpen(true);
                    }}
                  >
                    {t('common.edit')}
                  </Button>

                  <Button
                    size="sm"
                    variant="ghost"
                    // Em ACTIVE_ON_BACKUP a política guarda o único registro do
                    // roteamento original: reverter primeiro é obrigatório.
                    disabled={busy || onBackup || failover.isMutating}
                    onClick={async () => {
                      try {
                        await failover.disablePolicy(policy.id, policy.version);
                      } catch (err) {
                        toast({
                          title: t('pages.whatsapp.failover.disableFailed'),
                          description: (err as Error).message,
                          variant: 'destructive',
                        });
                      }
                    }}
                  >
                    {t('pages.whatsapp.failover.disable')}
                  </Button>
                </div>

                <FailoverEventList policyId={policy.id} />
              </div>
            );
          })}
        </CardContent>
      </Card>

      <FailoverPolicyDialog
        open={policyDialogOpen}
        policy={editing}
        instances={instances}
        onClose={() => setPolicyDialogOpen(false)}
      />

      <FailoverDryRunDialog
        open={pending !== null && !confirmOpen}
        report={pending?.report ?? null}
        direction={pending?.direction ?? FailoverDirection.FAILOVER}
        onClose={() => setPending(null)}
        onContinue={() => setConfirmOpen(true)}
      />

      <FailoverConfirmDialog
        open={confirmOpen && pending !== null}
        report={pending?.report ?? null}
        policy={pending?.policy ?? null}
        direction={pending?.direction ?? FailoverDirection.FAILOVER}
        onClose={() => {
          setConfirmOpen(false);
          setPending(null);
        }}
      />
    </>
  );
}
