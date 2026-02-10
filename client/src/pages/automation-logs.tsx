import { LogsVirtualTable } from '@/components/automation-logs';
import { ErrorBoundary } from '@/components/error-boundary';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useAutomationLogs } from '@/hooks';
import type { DispatchStatus } from '@/types/dispatch-log';
import { STORES } from '@/types/dispatch-log';
import { format } from 'date-fns';
import { Download, ExternalLink, FileText, Search } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};

const MOBILE_CARDS_LIMIT = 50;

/**
 * Automation logs page component
 */
export default function AutomationLogs() {
  const { t } = useTranslation();
  const {
    logs,
    automations,
    groups,
    loading,
    queueStatus,
    filterAutomation,
    filterStore,
    filterStatus,
    filterGroup,
    searchTerm,
    filteredLogs,
    setFilterAutomation,
    setFilterStore,
    setFilterStatus,
    setFilterGroup,
    setSearchTerm,
    exportCSV,
    getProductHref,
    getGroupName,
  } = useAutomationLogs();

  const [showAllMobileCards, setShowAllMobileCards] = useState(false);

  const getStatusBadge = (status: DispatchStatus) => {
    const labels: Record<DispatchStatus, string> = {
      queued: t('pages.automationLogs.status.queued'),
      sent: t('pages.automationLogs.status.sent'),
      skipped: t('pages.automationLogs.status.skipped'),
      error: t('pages.automationLogs.status.error'),
    };
    const styles: Record<DispatchStatus, { bg: string; color: string }> = {
      queued: { bg: 'rgba(141, 45, 83, 0.06)', color: colors.wineTextLight },
      sent: { bg: colors.iconBg, color: colors.wine },
      skipped: { bg: 'rgba(234, 179, 8, 0.1)', color: '#a16207' },
      error: { bg: 'rgba(239, 68, 68, 0.1)', color: '#dc2626' },
    };
    const s = styles[status] || styles.queued;
    return (
      <span
        className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
        style={{ backgroundColor: s.bg, color: s.color }}
      >
        {labels[status] || status}
      </span>
    );
  };

  const mobileLogsToShow = showAllMobileCards
    ? filteredLogs
    : filteredLogs.slice(0, MOBILE_CARDS_LIMIT);
  const hasMoreMobileLogs = filteredLogs.length > MOBILE_CARDS_LIMIT;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div
          className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
          style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
        />
      </div>
    );
  }

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8">
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <FileText
                    className="w-5 h-5 xs:w-6 xs:h-6"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <h1
                    className="text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.automationLogs.title')}
                  </h1>
                  <p
                    className="text-xs xs:text-sm md:text-base tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.automationLogs.subtitle')}
                  </p>
                </div>
              </div>
              <Button
                onClick={exportCSV}
                variant="outline"
                disabled={filteredLogs.length === 0}
                className="w-full md:w-auto"
              >
                <Download className="h-4 w-4 mr-2" />
                {t('pages.automationLogs.export.csv')}
              </Button>
            </div>

            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-4 xs:mb-5">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <Search
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
                  style={{ color: colors.wineText }}
                >
                  {t('pages.automationLogs.filters.title')}
                </span>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
                <div className="space-y-2">
                  <Label>{t('pages.automationLogs.filters.automation')}</Label>
                  <Select value={filterAutomation} onValueChange={setFilterAutomation}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t('pages.automationLogs.filters.all')}</SelectItem>
                      {automations.map((auto) => (
                        <SelectItem key={auto.id} value={auto.id}>
                          {auto.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <Label>{t('pages.automationLogs.filters.store')}</Label>
                  <Select value={filterStore} onValueChange={setFilterStore}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t('pages.automationLogs.filters.all')}</SelectItem>
                      {STORES.map((store) => (
                        <SelectItem key={store.value} value={store.value}>
                          {store.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <Label>{t('pages.automationLogs.filters.group')}</Label>
                  <Select value={filterGroup} onValueChange={setFilterGroup}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">
                        {t('pages.automationLogs.filters.allGroups')}
                      </SelectItem>
                      {groups.map((group) => (
                        <SelectItem key={group.id} value={group.wa_group_id}>
                          {group.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <Label>{t('pages.automationLogs.filters.status')}</Label>
                  <Select value={filterStatus} onValueChange={setFilterStatus}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">
                        {t('pages.automationLogs.filters.allStatus')}
                      </SelectItem>
                      <SelectItem value="queued">
                        {t('pages.automationLogs.status.queued')}
                      </SelectItem>
                      <SelectItem value="sent">{t('pages.automationLogs.status.sent')}</SelectItem>
                      <SelectItem value="skipped">
                        {t('pages.automationLogs.status.skipped')}
                      </SelectItem>
                      <SelectItem value="error">
                        {t('pages.automationLogs.status.error')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <Label>{t('pages.automationLogs.filters.searchUrl')}</Label>
                  <Input
                    placeholder={t('pages.automationLogs.filters.searchPlaceholder')}
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                  />
                </div>
              </div>
            </div>

            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 space-y-2 text-sm"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                <div style={{ color: colors.wineTextMuted }}>
                  <strong className="text-foreground">
                    {t('pages.automationLogs.status.queued')}
                  </strong>{' '}
                  {t('pages.automationLogs.queue.explanation')}
                  {queueStatus?.estimatedMinutes ? (
                    <span>
                      {' '}
                      {t('pages.automationLogs.queue.estimate')}{' '}
                      <strong className="text-foreground">
                        {queueStatus.estimatedMinutes} {t('pages.automationLogs.queue.minutes')}
                      </strong>
                      .
                    </span>
                  ) : null}
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" asChild>
                    <Link to="/whatsapp">{t('pages.automationLogs.queue.reconnect')}</Link>
                  </Button>
                </div>
              </div>
              {queueStatus ? (
                <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('pages.automationLogs.queue.waiting')}: {queueStatus.queue.queued} (
                  {t('pages.automationLogs.queue.waitingLabel')}={queueStatus.queue.waiting},{' '}
                  {t('pages.automationLogs.queue.delayedLabel')}={queueStatus.queue.delayed},{' '}
                  {t('pages.automationLogs.queue.activeLabel')}={queueStatus.queue.active})
                  {queueStatus.settings.limitPerMinute ? (
                    <span>
                      {' '}
                      {t('pages.automationLogs.queue.limit')} {queueStatus.settings.limitPerMinute}
                      {t('pages.automationLogs.queue.perMinute')}
                    </span>
                  ) : null}
                  {queueStatus.settings.workerConcurrency ? (
                    <span>
                      {' '}
                      {t('pages.automationLogs.queue.concurrency')}{' '}
                      {queueStatus.settings.workerConcurrency}
                    </span>
                  ) : null}
                </div>
              ) : null}
            </div>

            {filteredLogs.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12">
                <FileText
                  className="h-12 w-12 mb-4"
                  style={{ color: colors.wineTextMuted }}
                  aria-hidden="true"
                />
                <p style={{ color: colors.wineTextMuted }}>
                  {logs.length === 0
                    ? t('pages.automationLogs.empty')
                    : t('pages.automationLogs.emptyFiltered')}
                </p>
              </div>
            ) : (
              <div
                className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="mb-4 xs:mb-5">
                  <span className="text-base font-semibold" style={{ color: '#18181b' }}>
                    {t('pages.automationLogs.table.title')}
                  </span>
                  <p className="text-sm mt-1" style={{ color: colors.wineTextMuted }}>
                    {t('pages.automationLogs.table.description', {
                      filtered: filteredLogs.length,
                      total: logs.length,
                    })}
                  </p>
                </div>

                {/* Desktop - Tabela Virtualizada */}
                <div className="hidden md:block">
                  <LogsVirtualTable logs={filteredLogs} getGroupName={getGroupName} />
                </div>

                {/* Mobile - Cards */}
                <div className="md:hidden space-y-3">
                  {mobileLogsToShow.map((log) => (
                    <div
                      key={log.id}
                      className="rounded-xl border p-4 space-y-3 overflow-hidden"
                      style={{ backgroundColor: '#ffffff', borderColor: colors.wineBorder }}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex-1 min-w-0">
                          <h3 className="font-semibold text-sm mb-1 break-words">
                            {log.automation_name}
                          </h3>
                          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                            {format(new Date(log.created_at), 'dd/MM/yyyy HH:mm')}
                          </p>
                        </div>
                        {getStatusBadge(log.status)}
                      </div>

                      <div className="space-y-1 text-xs">
                        <div className="flex items-center gap-2">
                          <span style={{ color: colors.wineTextMuted }}>
                            {t('pages.automationLogs.table.store')}:
                          </span>
                          <span className="font-medium">
                            {STORES.find((s) => s.value === log.store)?.label || log.store}
                          </span>
                        </div>
                        <div className="flex items-center gap-2">
                          <span style={{ color: colors.wineTextMuted }}>
                            {t('pages.automationLogs.table.category')}:
                          </span>
                          <span className="font-medium truncate">
                            {log.product_category || '-'}
                          </span>
                        </div>
                        <div className="flex items-center gap-2">
                          <span style={{ color: colors.wineTextMuted }}>
                            {t('pages.automationLogs.table.group')}:
                          </span>
                          <span className="font-medium truncate">
                            {getGroupName(log.group_id, log.group_name)}
                          </span>
                        </div>
                        {log.coupon_id && (
                          <div className="flex items-start gap-2">
                            <span className="shrink-0" style={{ color: colors.wineTextMuted }}>
                              {t('pages.automationLogs.table.coupon')}:
                            </span>
                            <div className="min-w-0">
                              <span className="font-medium">
                                {log.coupon_code || t('pages.automationLogs.table.coupon')}
                              </span>{' '}
                              <span style={{ color: colors.wineTextMuted }}>
                                {log.coupon_applied
                                  ? `(${t('pages.automationLogs.couponStatus.applied')})`
                                  : `(${t('pages.automationLogs.couponStatus.notApplied')})`}
                              </span>
                              {!log.coupon_applied && log.coupon_skip_reason && (
                                <div
                                  className="break-words"
                                  style={{ color: colors.wineTextMuted }}
                                >
                                  {log.coupon_skip_reason}
                                </div>
                              )}
                            </div>
                          </div>
                        )}
                        {log.error && (
                          <div className="flex items-start gap-2 pt-1">
                            <span className="shrink-0" style={{ color: colors.wineTextMuted }}>
                              {t('pages.automationLogs.table.error')}:
                            </span>
                            <span className="text-destructive text-xs break-words">
                              {log.error}
                            </span>
                          </div>
                        )}
                      </div>

                      <div className="pt-2 border-t">
                        <a
                          href={getProductHref(log)}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center justify-center gap-2 text-xs text-primary hover:underline"
                        >
                          <ExternalLink className="h-3 w-3" />
                          {t('pages.automationLogs.table.viewProduct')}
                        </a>
                      </div>
                    </div>
                  ))}
                  {hasMoreMobileLogs && !showAllMobileCards && (
                    <Button
                      variant="outline"
                      className="w-full"
                      onClick={() => setShowAllMobileCards(true)}
                    >
                      {t('common.showMore')} ({filteredLogs.length - MOBILE_CARDS_LIMIT}{' '}
                      {t('common.more')})
                    </Button>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
