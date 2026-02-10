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
import { Table, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useVirtualizer } from '@tanstack/react-virtual';
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { ChevronDown, ChevronUp, Key, Loader2 } from 'lucide-react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { SystemEvent, UserInvestigation } from './types';

interface LoadBalancerServer {
  id: number;
  name: string;
  url: string;
}

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
  lbUserLoading: boolean;
  lbUserServer: LoadBalancerServer | null;
  lbTargetServerId: string;
  onLbTargetServerIdChange: (id: string) => void;
  lbMigrating: boolean;
  systemEvents: SystemEvent[];
  systemEventsLoading: boolean;
  systemEventsSinceMinutes: number;
  onSystemEventsSinceMinutesChange: (minutes: number) => void;
  onBlockUser: () => void;
  onUnblockUser: () => void;
  onResetPassword: () => void;
  onPauseAllAutomations: () => void;
  onActivatePausedAutomations: (alsoSpread: boolean) => void;
  onSpreadNextRunAt: () => void;
  onMigrateServer: () => void;
  onFetchSystemEvents: (userId: string) => void;
  onRefreshDetails: () => void;
}

/**
 * Dialog de detalhes do usuário com tabs para perfil, instâncias, automações e logs.
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
  lbUserLoading,
  lbUserServer,
  lbTargetServerId,
  onLbTargetServerIdChange,
  lbMigrating,
  systemEvents,
  systemEventsLoading,
  systemEventsSinceMinutes,
  onSystemEventsSinceMinutesChange,
  onBlockUser,
  onUnblockUser,
  onResetPassword,
  onPauseAllAutomations,
  onActivatePausedAutomations,
  onSpreadNextRunAt,
  onMigrateServer,
  onFetchSystemEvents,
  onRefreshDetails,
}: UserDetailsDialogProps) {
  const { t } = useTranslation();
  const [actionsOpen, setActionsOpen] = useState(false);

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

            {/* Tabs para organizar conteúdo */}
            <Tabs defaultValue="profile" className="flex-1 flex flex-col min-h-0">
              <TabsList className="grid w-full grid-cols-4">
                <TabsTrigger value="profile">{t('admin.users.details.tabs.profile')}</TabsTrigger>
                <TabsTrigger value="instances">WhatsApp ({details.instances.length})</TabsTrigger>
                <TabsTrigger value="automations">
                  {t('admin.users.details.tabs.automations')} ({details.automationStats.total})
                </TabsTrigger>
                <TabsTrigger value="logs">{t('admin.users.details.tabs.logs')}</TabsTrigger>
              </TabsList>

              <TabsContent value="profile" className="flex-1 overflow-y-auto">
                <ProfileTab
                  details={details}
                  lbUserLoading={lbUserLoading}
                  lbUserServer={lbUserServer}
                  lbTargetServerId={lbTargetServerId}
                  onLbTargetServerIdChange={onLbTargetServerIdChange}
                  lbMigrating={lbMigrating}
                  onMigrateServer={onMigrateServer}
                  onRefreshDetails={onRefreshDetails}
                />
              </TabsContent>

              <TabsContent value="instances" className="flex-1 overflow-y-auto">
                <InstancesTab instances={details.instances} />
              </TabsContent>

              <TabsContent value="automations" className="flex-1 overflow-y-auto">
                <AutomationsTab automations={details.automations} stats={details.automationStats} />
              </TabsContent>

              <TabsContent value="logs" className="flex-1 overflow-y-auto">
                <LogsTab
                  userId={details.profile.id}
                  systemEvents={systemEvents}
                  systemEventsLoading={systemEventsLoading}
                  systemEventsSinceMinutes={systemEventsSinceMinutes}
                  onSystemEventsSinceMinutesChange={onSystemEventsSinceMinutesChange}
                  onFetchSystemEvents={onFetchSystemEvents}
                />
              </TabsContent>
            </Tabs>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

interface ProfileTabProps {
  details: UserInvestigation;
  lbUserLoading: boolean;
  lbUserServer: LoadBalancerServer | null;
  lbTargetServerId: string;
  onLbTargetServerIdChange: (id: string) => void;
  lbMigrating: boolean;
  onMigrateServer: () => void;
  onRefreshDetails: () => void;
}

function ProfileTab({
  details,
  lbUserLoading,
  lbUserServer,
  lbTargetServerId,
  onLbTargetServerIdChange,
  lbMigrating,
  onMigrateServer,
  onRefreshDetails,
}: ProfileTabProps) {
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

        <div className="md:col-span-2 border-t pt-3 mt-2">
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between gap-2">
              <div>
                <div className="text-sm font-medium">
                  {t('admin.users.details.profile.loadBalancer')}
                </div>
                <div className="text-xs text-muted-foreground">
                  {t('admin.users.details.profile.stickyMapping')} <code>user_servers</code>
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={onRefreshDetails}
                disabled={lbUserLoading || lbMigrating}
              >
                {lbUserLoading ? (
                  <span className="flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    {t('admin.users.details.profile.updating')}
                  </span>
                ) : (
                  t('admin.users.details.profile.update')
                )}
              </Button>
            </div>

            <div className="text-xs text-muted-foreground">
              {t('admin.users.details.profile.current')}:{' '}
              {lbUserServer ? (
                <span className="font-mono">
                  {lbUserServer.name} ({lbUserServer.url})
                </span>
              ) : (
                <span className="text-muted-foreground">—</span>
              )}
            </div>

            <div className="flex flex-col sm:flex-row gap-2 sm:items-center">
              <Select value={lbTargetServerId} onValueChange={onLbTargetServerIdChange}>
                <SelectTrigger className="w-full sm:w-48">
                  <SelectValue placeholder={t('admin.users.details.profile.targetServer')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">{t('admin.users.details.profile.server')} 1</SelectItem>
                  <SelectItem value="2">{t('admin.users.details.profile.server')} 2</SelectItem>
                  <SelectItem value="3">{t('admin.users.details.profile.server')} 3</SelectItem>
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                onClick={onMigrateServer}
                disabled={lbMigrating}
                className="w-full sm:w-auto"
              >
                {lbMigrating
                  ? t('admin.users.details.profile.migrating')
                  : t('admin.users.details.profile.migrateUser')}
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

interface InstancesTabProps {
  instances: Array<{
    instance_id: string;
    status: string;
    updated_at: string | null;
  }>;
}

function InstancesTab({ instances }: InstancesTabProps) {
  const { t } = useTranslation();

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('admin.users.details.instances.title')}</CardTitle>
        <CardDescription className="text-xs">
          {t('admin.users.details.instances.found', {
            count: instances.length,
          })}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-2 text-sm">
        {instances.length === 0 ? (
          <div className="text-muted-foreground">
            {t('admin.users.details.instances.noInstances')}
          </div>
        ) : (
          instances.map((i) => (
            <div
              key={i.instance_id}
              className="flex flex-col md:flex-row md:items-center md:justify-between gap-1 border rounded p-2"
            >
              <div className="font-mono text-xs">{i.instance_id}</div>
              <div className="flex items-center gap-2 text-xs">
                <Badge variant={i.status === 'connected' ? 'default' : 'secondary'}>
                  {String(i.status || '-').toUpperCase()}
                </Badge>
                <span className="text-muted-foreground">
                  {i.updated_at
                    ? format(new Date(i.updated_at), 'dd/MM HH:mm', {
                        locale: ptBR,
                      })
                    : '-'}
                </span>
              </div>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  );
}

interface AutomationsTabProps {
  automations: Array<{
    id: string;
    name: string;
    status: string;
    interval_minutes?: number | null;
    start_time?: string | null;
    end_time?: string | null;
    next_run_at?: string | null;
  }>;
  stats: { total: number; active: number; paused: number };
}

const ROW_HEIGHT = 60;

/**
 * Tabela virtualizada de automações do usuário.
 */
function AutomationsTab({ automations, stats }: AutomationsTabProps) {
  const { t } = useTranslation();
  const parentRef = useRef<HTMLDivElement>(null);

  const virtualizer = useVirtualizer({
    count: automations.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 5,
  });

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
          <div className="border rounded-lg overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('admin.users.details.automations.columns.name')}</TableHead>
                  <TableHead>{t('admin.users.details.automations.columns.status')}</TableHead>
                  <TableHead>{t('admin.users.details.automations.columns.interval')}</TableHead>
                  <TableHead>{t('admin.users.details.automations.columns.window')}</TableHead>
                  <TableHead>{t('admin.users.details.automations.columns.next')}</TableHead>
                </TableRow>
              </TableHeader>
            </Table>
            <div ref={parentRef} className="max-h-[340px] overflow-y-auto">
              <div
                style={{
                  height: `${virtualizer.getTotalSize()}px`,
                  width: '100%',
                  position: 'relative',
                }}
              >
                {virtualizer.getVirtualItems().map((virtualRow) => {
                  const a = automations[virtualRow.index];
                  return (
                    <div
                      key={a.id}
                      style={{
                        position: 'absolute',
                        top: 0,
                        left: 0,
                        width: '100%',
                        height: `${virtualRow.size}px`,
                        transform: `translateY(${virtualRow.start}px)`,
                      }}
                      className="flex items-center border-b text-sm"
                    >
                      <div className="flex-1 px-4 max-w-[260px]">
                        <div className="font-medium truncate">
                          {a.name || t('admin.users.details.automations.noName')}
                        </div>
                        <div className="text-xs text-muted-foreground font-mono">{a.id}</div>
                      </div>
                      <div className="w-24 px-2">
                        <Badge variant={a.status === 'active' ? 'default' : 'secondary'}>
                          {String(a.status || '-').toUpperCase()}
                        </Badge>
                      </div>
                      <div className="w-24 px-2 text-muted-foreground">
                        {a.interval_minutes ? `${a.interval_minutes} min` : '-'}
                      </div>
                      <div className="w-32 px-2 text-muted-foreground">
                        {a.start_time && a.end_time ? `${a.start_time} - ${a.end_time}` : '-'}
                      </div>
                      <div className="w-28 px-2 text-muted-foreground">
                        {a.next_run_at
                          ? format(new Date(a.next_run_at), 'dd/MM HH:mm', {
                              locale: ptBR,
                            })
                          : '-'}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

interface LogsTabProps {
  userId: string;
  systemEvents: SystemEvent[];
  systemEventsLoading: boolean;
  systemEventsSinceMinutes: number;
  onSystemEventsSinceMinutesChange: (minutes: number) => void;
  onFetchSystemEvents: (userId: string) => void;
}

const LOG_ROW_HEIGHT = 48;

/**
 * Tabela virtualizada de logs do sistema.
 */
function LogsTab({
  userId,
  systemEvents,
  systemEventsLoading,
  systemEventsSinceMinutes,
  onSystemEventsSinceMinutesChange,
  onFetchSystemEvents,
}: LogsTabProps) {
  const { t } = useTranslation();
  const logsParentRef = useRef<HTMLDivElement>(null);

  const logsVirtualizer = useVirtualizer({
    count: systemEvents.length,
    getScrollElement: () => logsParentRef.current,
    estimateSize: () => LOG_ROW_HEIGHT,
    overscan: 5,
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('admin.users.details.logs.title')}</CardTitle>
        <CardDescription className="text-xs">
          {t('admin.users.details.logs.description')}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-col sm:flex-row gap-2 sm:items-end sm:justify-between">
          <div className="flex items-end gap-2">
            <div className="space-y-1">
              <Label className="text-xs">{t('admin.users.details.logs.window')}</Label>
              <Input
                type="number"
                className="w-28"
                min={1}
                value={systemEventsSinceMinutes}
                onChange={(e) =>
                  onSystemEventsSinceMinutesChange(Math.max(1, Number(e.target.value || 1)))
                }
              />
            </div>
            <Button
              variant="outline"
              onClick={() => onFetchSystemEvents(userId)}
              disabled={systemEventsLoading}
            >
              {systemEventsLoading ? (
                <span className="flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t('common.loading')}
                </span>
              ) : (
                t('admin.users.details.logs.load')
              )}
            </Button>
          </div>
          <div className="text-xs text-muted-foreground">{t('admin.users.details.logs.hint')}</div>
        </div>

        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-32">{t('admin.users.details.logs.columns.when')}</TableHead>
                <TableHead className="w-20">
                  {t('admin.users.details.logs.columns.level')}
                </TableHead>
                <TableHead className="w-28">
                  {t('admin.users.details.logs.columns.scope')}
                </TableHead>
                <TableHead>{t('admin.users.details.logs.columns.message')}</TableHead>
                <TableHead className="w-36">
                  {t('admin.users.details.logs.columns.instance')}
                </TableHead>
              </TableRow>
            </TableHeader>
          </Table>
          {systemEvents.length === 0 ? (
            <div className="text-sm text-muted-foreground p-4">
              {t('admin.users.details.logs.noLogs')}
            </div>
          ) : (
            <div ref={logsParentRef} className="max-h-[300px] overflow-y-auto">
              <div
                style={{
                  height: `${logsVirtualizer.getTotalSize()}px`,
                  width: '100%',
                  position: 'relative',
                }}
              >
                {logsVirtualizer.getVirtualItems().map((virtualRow) => {
                  const ev = systemEvents[virtualRow.index];
                  return (
                    <div
                      key={ev.id}
                      style={{
                        position: 'absolute',
                        top: 0,
                        left: 0,
                        width: '100%',
                        height: `${virtualRow.size}px`,
                        transform: `translateY(${virtualRow.start}px)`,
                      }}
                      className="flex items-center border-b text-xs"
                    >
                      <div className="w-32 px-4 whitespace-nowrap">
                        {ev.created_at
                          ? format(new Date(ev.created_at), 'dd/MM HH:mm:ss', {
                              locale: ptBR,
                            })
                          : '-'}
                      </div>
                      <div className="w-20 px-2">
                        <Badge
                          variant={
                            ev.level === 'error'
                              ? 'destructive'
                              : ev.level === 'warn'
                                ? 'secondary'
                                : 'secondary'
                          }
                        >
                          {String(ev.level || '').toUpperCase()}
                        </Badge>
                      </div>
                      <div className="w-28 px-2 font-mono">{ev.scope}</div>
                      <div className="flex-1 px-2 truncate">{ev.message}</div>
                      <div className="w-36 px-4 font-mono truncate">{ev.instance_id || '-'}</div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
