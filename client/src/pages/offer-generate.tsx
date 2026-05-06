import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Switch } from '@/components/ui/switch';
import {
  DispatchTextSelector,
  MonitorGroupsSelector,
  SendGroupsSelector,
} from '@/components/automations/form-sections';
import { ErrorBoundary } from '@/components/error-boundary';
import { Label } from '@/components/ui/label';
import { ChipSelector } from '@/components/whatsapp/chip-selector';
import { useOfferGenerate } from '@/hooks/use-offer-generate';
import type { AutomationGroup } from '@/types/automation';
import {
  ChevronDown,
  ChevronUp,
  Package,
  Save,
  Settings,
  Trash2,
  WifiOff,
  Zap,
} from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

type StatusTab = '' | 'sent' | 'failed' | 'skipped';

export default function OfferGenerate() {
  const { t } = useTranslation();
  const [configOpen, setConfigOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<StatusTab>('');

  const {
    config,
    formData,
    setFormData,
    groups,
    allGroups,
    instances,
    whatsappConnected,
    logs,
    logsLoading,
    logsTotalCount,
    logFilterStatus,
    setLogFilterStatus,
    loading,
    saving,
    handleSave,
    handleToggleStatus,
    handleDelete,
    toggleFavorite,
    dispatchTexts,
    deleteDialogOpen,
    setDeleteDialogOpen,
    confirmDelete,
  } = useOfferGenerate();

  // Chip selection: which WhatsApp number this offer-generate config runs
  // on. Default to the existing config's chip when editing; otherwise the
  // user's default. The dispatch routing reads each saved group's chip,
  // so this is purely a UI affordance to keep the group list focused.
  const [selectedChipId, setSelectedChipId] = useState<string | null>(null);
  useEffect(() => {
    if (instances.length === 0) {
      setSelectedChipId(null);
      return;
    }
    const firstSendGroup = formData.send_groups?.[0];
    const fromExisting = firstSendGroup
      ? groups.find((g) => g.wa_group_id === firstSendGroup)?.instance_id
      : undefined;
    if (fromExisting) {
      setSelectedChipId(fromExisting);
      return;
    }
    const defaultInstance = instances.find((i) => i.is_default) ?? instances[0];
    setSelectedChipId((prev) => prev ?? defaultInstance?.id ?? null);
    // Re-resolve only when the config row changes, not on every keystroke.
  }, [config?.id, instances]);

  // Match against participating_instance_ids (every chip whose sync has
  // seen the group) instead of just instance_id (the single owner). Without
  // this, a group both chips are admin in only shows under the owning chip
  // and the user can't reach it from the other chip's view. instance_id
  // stays as the fallback for pre-migration legacy rows.
  const matchesChip = (g: AutomationGroup, chipId: string): boolean => {
    const participants = g.participating_instance_ids;
    if (participants && participants.length > 0) {
      return participants.includes(chipId);
    }
    return g.instance_id === chipId;
  };

  const filteredGroups = useMemo(() => {
    if (!selectedChipId) return groups;
    return groups.filter((g) => matchesChip(g, selectedChipId));
  }, [groups, selectedChipId]);

  const filteredAllGroups = useMemo(() => {
    if (!selectedChipId) return allGroups;
    return allGroups.filter((g) => matchesChip(g, selectedChipId));
  }, [allGroups, selectedChipId]);

  const handleChipChange = (id: string | null) => {
    if (id === selectedChipId) return;
    setSelectedChipId(id);
    // Clear selections so the user does not save group ids that belong to
    // a chip they are no longer targeting.
    setFormData((prev) => ({
      ...prev,
      send_groups: [],
      monitor_groups: [],
    }));
  };

  const handleTabChange = (tab: StatusTab) => {
    setActiveTab(tab);
    setLogFilterStatus(tab);
  };

  if (loading) {
    return (
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8 lg:p-10 xl:p-12"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="flex items-center justify-center min-h-[400px]">
            <div className="text-center space-y-4">
              <div
                className="h-12 w-12 animate-spin rounded-full border-4 border-t-transparent mx-auto"
                style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
              />
              <p style={{ color: colors.wineTextMuted }}>{t('common.loading')}</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Auto-open config if no config exists yet
  const showConfig = configOpen || !config;

  const tabs: { key: StatusTab; label: string }[] = [
    { key: '', label: t('pages.offerGenerate.tabs.all') },
    { key: 'sent', label: t('pages.offerGenerate.tabs.sent') },
    { key: 'failed', label: t('pages.offerGenerate.tabs.failed') },
    { key: 'skipped', label: t('pages.offerGenerate.tabs.skipped') },
  ];

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8 lg:p-10 xl:p-12"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6">
            {/* Header */}
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <Zap className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
                </div>
                <div>
                  <div className="flex items-center gap-2 flex-wrap">
                    <h2
                      className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                      style={{ color: '#18181b' }}
                    >
                      {t('pages.offerGenerate.title')}
                    </h2>
                    {config && (
                      <span
                        className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
                        style={{
                          backgroundColor:
                            config.status === 'active' ? 'rgba(34, 197, 94, 0.1)' : 'rgba(161, 161, 170, 0.15)',
                          color: config.status === 'active' ? 'rgb(22, 163, 74)' : 'rgb(113, 113, 122)',
                        }}
                      >
                        {config.status === 'active'
                          ? t('pages.offerGenerate.status.active')
                          : t('pages.offerGenerate.status.paused')}
                      </span>
                    )}
                  </div>
                  <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
                    {t('pages.offerGenerate.subtitle')}
                  </p>
                </div>
              </div>

              <div className="flex items-center gap-3">
                {config && (
                  <div className="flex items-center gap-2">
                    <span className="text-sm" style={{ color: colors.wineTextMuted }}>
                      {config.status === 'active'
                        ? t('pages.offerGenerate.actions.pause')
                        : t('pages.offerGenerate.actions.activate')}
                    </span>
                    <Switch
                      checked={config.status === 'active'}
                      onCheckedChange={handleToggleStatus}
                      className="cursor-pointer"
                    />
                  </div>
                )}
              </div>
            </div>

            {/* WhatsApp Disconnected Alert */}
            {!whatsappConnected && (
              <div
                className="rounded-xl xs:rounded-2xl border p-4 xs:p-5"
                role="alert"
                style={{
                  backgroundColor: 'rgba(239, 68, 68, 0.06)',
                  borderColor: 'rgba(239, 68, 68, 0.25)',
                }}
              >
                <div className="flex items-center gap-2 xs:gap-3">
                  <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: 'rgba(239, 68, 68, 0.1)' }}>
                    <WifiOff className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: 'rgb(220, 50, 50)' }} />
                  </div>
                  <p className="text-xs xs:text-sm leading-relaxed" style={{ color: 'rgba(185, 40, 40, 0.8)' }}>
                    {t('pages.offerGenerate.whatsappDisconnected')}
                  </p>
                </div>
              </div>
            )}

            {/* Config - collapsible when config exists */}
            <div
              className="rounded-xl xs:rounded-2xl border"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              {/* Config header - clickable to toggle */}
              <button
                type="button"
                onClick={() => config && setConfigOpen(!configOpen)}
                className={`w-full flex items-center justify-between p-4 xs:p-5 ${config ? 'cursor-pointer' : 'cursor-default'}`}
              >
                <div className="flex items-center gap-2 xs:gap-3">
                  <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                    <Settings className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
                  </div>
                  <span
                    className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
                    style={{ color: colors.wineTextLight }}
                  >
                    {t('pages.offerGenerate.config.title')}
                  </span>
                </div>
                {config && (
                  showConfig
                    ? <ChevronUp className="w-4 h-4" style={{ color: colors.wineTextMuted }} />
                    : <ChevronDown className="w-4 h-4" style={{ color: colors.wineTextMuted }} />
                )}
              </button>

              {/* Config body */}
              {showConfig && (
                <div className="px-4 xs:px-5 pb-4 xs:pb-5 space-y-3 xs:space-y-4">
                  {/* Chip selector — picks which WhatsApp number this
                      offer-generate config runs on. Filters the group lists
                      below to that chip's groups. */}
                  {instances.length > 0 && (
                    <div className="space-y-1.5 xs:space-y-2">
                      <Label>
                        {t('pages.offerGenerate.config.chip', 'Número')} *{' '}
                        <span className="text-xs text-muted-foreground">
                          {t(
                            'pages.offerGenerate.config.chipHint',
                            'Os grupos abaixo são deste número. Trocar limpa a seleção.'
                          )}
                        </span>
                      </Label>
                      <ChipSelector
                        value={selectedChipId}
                        onChange={handleChipChange}
                        instances={instances}
                      />
                    </div>
                  )}

                  {/* Source Group (Monitor / Router group) */}
                  <div className="space-y-1.5">
                    <p className="text-xs font-medium" style={{ color: colors.wineTextLight }}>
                      {t('pages.offerGenerate.config.sourceGroup')}
                    </p>
                    <p className="text-[11px]" style={{ color: colors.wineTextMuted }}>
                      {t('pages.offerGenerate.config.sourceGroupDescription')}
                    </p>
                    <MonitorGroupsSelector
                      formData={formData}
                      setFormData={setFormData}
                      allGroups={filteredAllGroups}
                      onToggleFavorite={toggleFavorite}
                      maxGroups={1}
                      hideLabel
                    />
                  </div>

                  {/* Destination Groups */}
                  <SendGroupsSelector
                    formData={formData}
                    setFormData={setFormData}
                    groups={filteredGroups}
                    onToggleFavorite={toggleFavorite}
                  />

                  {/* Dispatch Text */}
                  <DispatchTextSelector
                    formData={formData}
                    setFormData={setFormData}
                    dispatchTexts={dispatchTexts}
                  />

                  {/* Action buttons */}
                  <div className="flex flex-col xs:flex-row justify-between gap-2 pt-1">
                    {config && (
                      <button
                        type="button"
                        onClick={handleDelete}
                        className="inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-red-300 bg-white text-red-600 shadow-sm hover:-translate-y-0.5 hover:border-red-400 hover:shadow-md"
                      >
                        <Trash2 className="h-3.5 w-3.5 xs:h-4 xs:w-4" />
                        {t('pages.offerGenerate.actions.delete')}
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={handleSave}
                      disabled={saving}
                      className="auth-btn-glow relative inline-flex w-full xs:w-auto cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50 ml-auto"
                    >
                      <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                        <div className="auth-btn-glow__glow absolute inset-0" />
                      </div>
                      <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                        <Save className="h-3.5 w-3.5 xs:h-4 xs:w-4" />
                        {saving ? t('pages.offerGenerate.actions.saving') : t('pages.offerGenerate.actions.save')}
                      </span>
                    </button>
                  </div>
                </div>
              )}
            </div>

            {/* Messages / Logs - main content area */}
            {config && (
              <div>
                {/* Status tabs */}
                <div className="flex border-b" style={{ borderColor: 'rgba(228, 228, 231, 0.8)' }}>
                  {tabs.map((tab) => (
                    <button
                      key={tab.key}
                      type="button"
                      onClick={() => handleTabChange(tab.key)}
                      className="px-4 py-2.5 text-sm font-medium transition-colors cursor-pointer relative"
                      style={{
                        color: activeTab === tab.key ? colors.wine : 'rgb(113, 113, 122)',
                      }}
                    >
                      {tab.label}
                      {activeTab === tab.key && (
                        <div
                          className="absolute bottom-0 left-0 right-0 h-0.5 rounded-full"
                          style={{ backgroundColor: colors.wine }}
                        />
                      )}
                    </button>
                  ))}
                  <div className="ml-auto flex items-center pr-2">
                    <span className="text-xs" style={{ color: colors.wineTextMuted }}>
                      {logsTotalCount} {logsTotalCount === 1 ? 'mensagem' : 'mensagens'}
                    </span>
                  </div>
                </div>

                {/* Table */}
                {logsLoading ? (
                  <div className="flex items-center justify-center py-16">
                    <div
                      className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
                      style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
                    />
                  </div>
                ) : logs.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-16">
                    <div className="p-3 rounded-xl mb-4" style={{ backgroundColor: colors.iconBg }}>
                      <Package className="h-10 w-10" style={{ color: colors.wine }} />
                    </div>
                    <p className="text-sm" style={{ color: colors.wineTextMuted }}>
                      {t('pages.offerGenerate.logs.noLogs')}
                    </p>
                    <p className="text-xs mt-1" style={{ color: colors.wineTextMuted }}>
                      {t('pages.offerGenerate.logs.noLogsDescription')}
                    </p>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b" style={{ borderColor: 'rgba(228, 228, 231, 0.6)' }}>
                          <th className="px-4 py-3 text-left text-xs font-medium text-zinc-500">
                            {t('pages.offerGenerate.logs.table.product')}
                          </th>
                          <th className="px-4 py-3 text-left text-xs font-medium text-zinc-500 hidden sm:table-cell">
                            {t('pages.offerGenerate.logs.table.store')}
                          </th>
                          <th className="px-4 py-3 text-left text-xs font-medium text-zinc-500">
                            {t('pages.offerGenerate.logs.table.status')}
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {logs.map((log) => (
                          <tr
                            key={log.id}
                            className="border-b hover:bg-zinc-50/50 transition-colors"
                            style={{ borderColor: 'rgba(228, 228, 231, 0.4)' }}
                          >
                            <td className="px-4 py-3">
                              <div className="flex items-center gap-3">
                                <div className="w-10 h-10 rounded-lg bg-zinc-100 flex items-center justify-center shrink-0">
                                  <Package className="w-5 h-5 text-zinc-400" />
                                </div>
                                <div className="min-w-0 space-y-0.5">
                                  <p className="text-sm font-medium text-zinc-900 truncate max-w-[220px] sm:max-w-[350px]">
                                    Produto: {log.offerTitle || log.canonicalKey || '-'}
                                  </p>
                                  {log.groupName && (
                                    <p className="text-xs text-zinc-500 truncate max-w-[220px] sm:max-w-[350px]">
                                      {log.groupName}
                                    </p>
                                  )}
                                  <p className="text-[11px] text-zinc-400">
                                    Criado em:{' '}
                                    {log.createdAt
                                      ? new Date(log.createdAt).toLocaleString('pt-BR', {
                                          day: '2-digit',
                                          month: '2-digit',
                                          year: 'numeric',
                                          hour: '2-digit',
                                          minute: '2-digit',
                                        })
                                      : '-'}
                                  </p>
                                </div>
                              </div>
                            </td>
                            <td className="px-4 py-3 hidden sm:table-cell">
                              <span className="text-sm text-zinc-600 capitalize">{log.store || '-'}</span>
                            </td>
                            <td className="px-4 py-3">
                              <LogStatusBadge status={log.status} />
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Delete Confirmation */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('pages.offerGenerate.confirm.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('pages.offerGenerate.confirm.deleteDescription')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>{t('common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ErrorBoundary>
  );
}

function LogStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();

  const styles: Record<string, { bg: string; color: string }> = {
    sent: { bg: 'rgba(34, 197, 94, 0.1)', color: 'rgb(22, 163, 74)' },
    failed: { bg: 'rgba(239, 68, 68, 0.1)', color: 'rgb(185, 40, 40)' },
    error: { bg: 'rgba(239, 68, 68, 0.1)', color: 'rgb(185, 40, 40)' },
    skipped: { bg: 'rgba(180, 120, 60, 0.1)', color: 'rgb(160, 100, 40)' },
    queued: { bg: 'rgba(59, 130, 246, 0.1)', color: 'rgb(37, 99, 235)' },
  };

  const s = styles[status] ?? styles.queued;
  const label = t(`pages.offerGenerate.logs.status.${status === 'error' ? 'failed' : status}`);

  return (
    <span
      className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
      style={{ backgroundColor: s.bg, color: s.color }}
    >
      {label}
    </span>
  );
}
