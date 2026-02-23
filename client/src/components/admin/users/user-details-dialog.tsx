import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { ChevronDown, ChevronUp, Key } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { UserInvestigation } from './types';

interface UserDetailsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  loading: boolean;
  details: UserInvestigation | null;
  bulkActionLoading: boolean;
  spreadMinutes: number;
  onSpreadMinutesChange: (minutes: number) => void;
  blockReason: string;
  onBlockReasonChange: (reason: string) => void;
  blockKind: 'payment' | 'manual';
  onBlockKindChange: (kind: 'payment' | 'manual') => void;
  onBlockUser: () => void;
  onUnblockUser: () => void;
  onResetPassword: () => void;
  onPauseAllAutomations: () => void;
  onActivatePausedAutomations: (alsoSpread: boolean) => void;
  onSpreadNextRunAt: () => void;
  onRefreshDetails: () => void;
}

/**
 * Dialog de detalhes do usuário com perfil, ações rápidas e automações.
 */
export function UserDetailsDialog({
  open,
  onOpenChange,
  loading,
  details,
  bulkActionLoading,
  spreadMinutes,
  onSpreadMinutesChange,
  blockReason,
  onBlockReasonChange,
  blockKind,
  onBlockKindChange,
  onBlockUser,
  onUnblockUser,
  onResetPassword,
  onPauseAllAutomations,
  onActivatePausedAutomations,
  onSpreadNextRunAt,
  onRefreshDetails,
}: UserDetailsDialogProps) {
  const { t } = useTranslation();
  const [actionsOpen, setActionsOpen] = useState(true);

  const isSuspended = String(details?.profile.billing_status || '').toLowerCase() === 'suspended';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[90vh] flex flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>{t('admin.users.details.title')}</DialogTitle>
          <DialogDescription>{details?.profile.email || t('common.loading')}</DialogDescription>
        </DialogHeader>

        {loading ? (
          <div className="py-8 text-center text-sm text-muted-foreground">
            {t('admin.users.details.loading')}
          </div>
        ) : !details ? (
          <div className="py-8 text-center text-sm text-muted-foreground">
            {t('admin.users.details.noData')}
          </div>
        ) : (
          <div className="flex flex-col flex-1 min-h-0 space-y-4">
            {/* Ações Rápidas - Collapsible */}
            <Collapsible open={actionsOpen} onOpenChange={setActionsOpen}>
              <CollapsibleTrigger asChild>
                <Button variant="outline" className="w-full justify-between">
                  <span>{t('admin.users.details.quickActions')}</span>
                  {actionsOpen ? (
                    <ChevronUp className="h-4 w-4" />
                  ) : (
                    <ChevronDown className="h-4 w-4" />
                  )}
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent className="mt-2">
                <div className="flex flex-col gap-3 p-4 border rounded-lg bg-muted/50">
                  <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-2">
                    {isSuspended ? (
                      <Button
                        variant="outline"
                        onClick={onUnblockUser}
                        disabled={bulkActionLoading}
                        title={t('admin.users.details.actions.unblockTitle')}
                        className="text-xs"
                      >
                        {bulkActionLoading
                          ? t('common.processing')
                          : t('admin.users.details.actions.unblock')}
                      </Button>
                    ) : (
                      <Button
                        variant="destructive"
                        onClick={onBlockUser}
                        disabled={bulkActionLoading}
                        title={t('admin.users.details.actions.blockTitle')}
                        className="text-xs"
                      >
                        {bulkActionLoading
                          ? t('common.processing')
                          : t('admin.users.details.actions.block')}
                      </Button>
                    )}
                    <Button
                      variant="outline"
                      onClick={() => onActivatePausedAutomations(false)}
                      disabled={bulkActionLoading}
                      title={t('admin.users.details.actions.activatePausedTitle')}
                      className="text-xs"
                    >
                      {bulkActionLoading
                        ? t('common.processing')
                        : t('admin.users.details.actions.activatePaused')}
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => onActivatePausedAutomations(true)}
                      disabled={bulkActionLoading}
                      title={t('admin.users.details.actions.activateAndSpreadTitle')}
                      className="text-xs"
                    >
                      {bulkActionLoading
                        ? t('common.processing')
                        : t('admin.users.details.actions.activateAndSpread')}
                    </Button>
                    <Button
                      variant="outline"
                      onClick={onPauseAllAutomations}
                      disabled={bulkActionLoading}
                      className="text-xs"
                    >
                      {bulkActionLoading
                        ? t('common.processing')
                        : t('admin.users.details.actions.pause')}
                    </Button>
                    <Button
                      variant="outline"
                      onClick={onResetPassword}
                      disabled={bulkActionLoading}
                      title={t('admin.users.details.actions.resetPasswordTitle')}
                      className="text-xs"
                    >
                      <Key className="h-3 w-3 mr-1" />
                      {bulkActionLoading
                        ? t('common.processing')
                        : t('admin.users.details.actions.resetPassword')}
                    </Button>
                  </div>
                  {!isSuspended && (
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2 border-t">
                      <div className="flex gap-2 items-center">
                        <Label className="text-xs text-muted-foreground whitespace-nowrap w-16">
                          {t('admin.users.details.actions.blockType.label')}
                        </Label>
                        <Select
                          value={blockKind}
                          onValueChange={(v) =>
                            onBlockKindChange(v === 'manual' ? 'manual' : 'payment')
                          }
                        >
                          <SelectTrigger className="flex-1">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="payment">
                              {t('admin.users.details.actions.blockType.payment')}
                            </SelectItem>
                            <SelectItem value="manual">
                              {t('admin.users.details.actions.blockType.manual')}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="flex gap-2 items-center">
                        <Label className="text-xs text-muted-foreground whitespace-nowrap w-16">
                          {t('admin.users.details.actions.blockReason.label')}
                        </Label>
                        <Input
                          className="flex-1"
                          placeholder={t('admin.users.details.actions.blockReason.placeholder')}
                          value={blockReason}
                          onChange={(e) => onBlockReasonChange(e.target.value)}
                        />
                      </div>
                    </div>
                  )}
                  <div className="flex gap-2 items-center pt-2 border-t">
                    <Label className="text-xs text-muted-foreground whitespace-nowrap">
                      {t('admin.users.details.actions.spreadMinutes')}
                    </Label>
                    <Input
                      type="number"
                      className="w-20"
                      value={spreadMinutes}
                      min={1}
                      onChange={(e) => onSpreadMinutesChange(Math.max(1, Number(e.target.value)))}
                    />
                    <Button
                      variant="outline"
                      onClick={onSpreadNextRunAt}
                      disabled={bulkActionLoading}
                      className="text-xs"
                    >
                      {bulkActionLoading
                        ? t('common.processing')
                        : t('admin.users.details.actions.rescheduleNextRun')}
                    </Button>
                  </div>
                </div>
              </CollapsibleContent>
            </Collapsible>

            <div className="flex-1 overflow-y-auto space-y-4">
              <ProfileTab details={details} onRefreshDetails={onRefreshDetails} />
              <AutomationsTab automations={details.automations} stats={details.automationStats} />
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

interface ProfileTabProps {
  details: UserInvestigation;
  onRefreshDetails: () => void;
}

function ProfileTab({ details, onRefreshDetails }: ProfileTabProps) {
  const { t } = useTranslation();
  const isSuspended = String(details.profile.billing_status || '').toLowerCase() === 'suspended';

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('admin.users.details.tabs.profile')}</CardTitle>
        <CardDescription className="text-xs">
          ID: <span className="font-mono">{details.profile.id}</span>
        </CardDescription>
      </CardHeader>
      <CardContent className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
        <div>
          <span className="text-muted-foreground">{t('admin.users.details.profile.email')}:</span>{' '}
          <span className="font-mono">{details.profile.email}</span>
        </div>
        <div>
          <span className="text-muted-foreground">{t('admin.users.details.profile.plan')}:</span>{' '}
          <Badge variant="secondary">
            {details.profile.plan?.toUpperCase?.() || details.profile.plan}
          </Badge>
        </div>
        <div>
          <span className="text-muted-foreground">
            {t('admin.users.details.profile.billingStatus')}:
          </span>{' '}
          {isSuspended ? (
            <Badge variant="destructive">{t('admin.users.details.profile.suspended')}</Badge>
          ) : (
            <Badge variant="secondary">{t('common.ok')}</Badge>
          )}
        </div>
        <div className="break-words">
          <span className="text-muted-foreground">{t('admin.users.details.profile.reason')}:</span>{' '}
          {details.profile.billing_suspension_reason ? (
            <span className="font-mono text-xs">{details.profile.billing_suspension_reason}</span>
          ) : (
            <span className="text-muted-foreground">-</span>
          )}
        </div>
        <div>
          <span className="text-muted-foreground">{t('admin.users.details.profile.type')}:</span>{' '}
          {details.profile.billing_suspension_kind ? (
            <Badge variant="secondary">
              {String(details.profile.billing_suspension_kind).toUpperCase()}
            </Badge>
          ) : (
            <span className="text-muted-foreground">-</span>
          )}
        </div>
        <div>
          <span className="text-muted-foreground">WhatsApp:</span>{' '}
          {details.profile.phone_number ? (
            <span>{details.profile.phone_number}</span>
          ) : (
            <span className="text-muted-foreground">-</span>
          )}
        </div>
        <div>
          <span className="text-muted-foreground">{t('admin.users.details.profile.admin')}:</span>{' '}
          {details.isAdmin ? (
            <Badge>ADMIN</Badge>
          ) : (
            <Badge variant="secondary">{t('common.no')}</Badge>
          )}
        </div>
        <div>
          <span className="text-muted-foreground">
            {t('admin.users.details.profile.automationLimit')}:
          </span>{' '}
          {details.profile.max_automations ?? (
            <span className="text-muted-foreground">
              ({t('admin.users.details.profile.default')})
            </span>
          )}
        </div>
        <div>
          <span className="text-muted-foreground">{t('admin.users.registration')}:</span>{' '}
          {format(new Date(details.profile.created_at), 'dd/MM/yyyy', {
            locale: ptBR,
          })}
        </div>
      </CardContent>
    </Card>
  );
}

interface AutomationsTabProps {
  automations: Array<{
    id: string;
    name: string;
    status: string;
    mode?: string | null;
    interval_minutes?: number | null;
    delay_seconds?: number | null;
    start_time?: string | null;
    end_time?: string | null;
    next_run_at?: string | null;
  }>;
  stats: { total: number; active: number; paused: number };
}

/**
 * Tabela de automações do usuário.
 */
function AutomationsTab({ automations, stats }: AutomationsTabProps) {
  const { t } = useTranslation();

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('admin.users.details.tabs.automations')}</CardTitle>
        <CardDescription className="text-xs">
          {t('admin.users.details.automations.total')}: {stats.total} •{' '}
          {t('admin.users.details.automations.active')}: {stats.active} •{' '}
          {t('admin.users.details.automations.paused')}: {stats.paused}
        </CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        {automations.length === 0 ? (
          <div className="text-muted-foreground text-sm p-6">
            {t('admin.users.details.automations.noAutomations')}
          </div>
        ) : (
          <div className="max-h-[340px] overflow-y-auto">
            <Table>
              <TableHeader className="sticky top-0 bg-background z-10">
                <TableRow>
                  <TableHead>{t('admin.users.details.automations.columns.name')}</TableHead>
                  <TableHead>{t('admin.users.details.automations.columns.status')}</TableHead>
                  <TableHead>{t('admin.users.details.automations.columns.interval')}</TableHead>
                  <TableHead>{t('admin.users.details.automations.columns.window')}</TableHead>
                  <TableHead>{t('admin.users.details.automations.columns.next')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {automations.map((a) => (
                  <TableRow key={a.id}>
                    <TableCell>
                      <div className="font-medium truncate max-w-[260px]">
                        {a.name || t('admin.users.details.automations.noName')}
                      </div>
                      <div className="text-xs text-muted-foreground font-mono">{a.id}</div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={a.status === 'active' ? 'default' : 'secondary'}>
                        {String(a.status || '-').toUpperCase()}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {a.mode === 'monitor'
                        ? (() => {
                            const s = Number(a.delay_seconds || 10);
                            return s >= 60 && s % 60 === 0
                              ? `${s / 60} min (${t('admin.users.details.automations.delay')})`
                              : `${s}s (${t('admin.users.details.automations.delay')})`;
                          })()
                        : a.interval_minutes
                          ? `${a.interval_minutes} min`
                          : '-'}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {a.start_time && a.end_time ? `${a.start_time} - ${a.end_time}` : '-'}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {a.next_run_at
                        ? format(new Date(a.next_run_at), 'dd/MM HH:mm', {
                            locale: ptBR,
                          })
                        : '-'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
