import type { LocalInstance } from '@/hooks/use-instances-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { Check, Loader2, Pencil, Plus, RefreshCw, Star, Trash2, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AddChipModal } from './add-chip-modal';
import { ReconnectChipModal } from './reconnect-chip-modal';

const MAX_INSTANCES_PER_USER = 10;

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  defaultBadgeBg: 'rgba(212, 175, 55, 0.15)',
  defaultBadgeBorder: 'rgba(212, 175, 55, 0.4)',
  defaultBadgeText: '#a78920',
  connectedDot: '#22c55e',
  disconnectedDot: '#ef4444',
  pendingDot: '#f59e0b',
};

function statusDot(status: LocalInstance['status']): string {
  if (status === 'connected') return colors.connectedDot;
  if (status === 'disconnected') return colors.disconnectedDot;
  return colors.pendingDot;
}

/**
 * Formats a WhatsApp phone number for display. Brazilian numbers are split
 * as +55 DD 9xxxx-xxxx; everything else just gets a leading "+" so the user
 * sees a recognisable international number. Returns null when input is empty.
 */
function formatPhone(raw: string | undefined): string | null {
  if (!raw) return null;
  const digits = raw.replace(/\D/g, '');
  if (digits.length === 0) return null;
  // Brazilian: 55 + DDD (2) + 9-digit mobile or 8-digit landline.
  if (digits.startsWith('55') && (digits.length === 12 || digits.length === 13)) {
    const cc = digits.slice(0, 2);
    const ddd = digits.slice(2, 4);
    const rest = digits.slice(4);
    if (rest.length === 9) {
      return `+${cc} ${ddd} ${rest.slice(0, 5)}-${rest.slice(5)}`;
    }
    if (rest.length === 8) {
      return `+${cc} ${ddd} ${rest.slice(0, 4)}-${rest.slice(4)}`;
    }
  }
  return `+${digits}`;
}

interface InstanceRowProps {
  instance: LocalInstance;
  onSetDefault: (id: string) => void;
  onDelete: (id: string, name: string) => void;
  onRename: (id: string, friendlyName: string) => Promise<void>;
  onReconnect: (instance: LocalInstance) => void;
  busy: boolean;
}

function InstanceRow({
  instance,
  onSetDefault,
  onDelete,
  onRename,
  onReconnect,
  busy,
}: InstanceRowProps) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(instance.friendly_name?.trim() || '');

  const statusLabel =
    instance.status === 'connected'
      ? t('pages.whatsapp.instancesList.statusConnected', 'Conectado')
      : instance.status === 'disconnected'
        ? t('pages.whatsapp.instancesList.statusDisconnected', 'Desconectado')
        : t('pages.whatsapp.instancesList.statusPending', 'Pendente');

  const startEdit = () => {
    setDraft(instance.friendly_name?.trim() || '');
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setDraft(instance.friendly_name?.trim() || '');
  };

  const commitEdit = async () => {
    const trimmed = draft.trim();
    // Treat unchanged value as a no-op so we don't fire useless requests.
    if (trimmed === (instance.friendly_name?.trim() || '')) {
      setEditing(false);
      return;
    }
    await onRename(instance.id, trimmed);
    setEditing(false);
  };

  return (
    <div
      className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 rounded-xl border p-3 sm:p-4"
      style={{ backgroundColor: '#ffffff', borderColor: colors.wineBorder }}
    >
      <div className="flex items-start gap-3 min-w-0 flex-1">
        <div
          className="mt-1 h-2.5 w-2.5 shrink-0 rounded-full"
          style={{ backgroundColor: statusDot(instance.status) }}
          aria-label={statusLabel}
        />
        <div className="min-w-0 flex-1">
          {editing ? (
            <div className="flex items-center gap-1.5 flex-wrap">
              <input
                type="text"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    commitEdit();
                  } else if (e.key === 'Escape') {
                    e.preventDefault();
                    cancelEdit();
                  }
                }}
                disabled={busy}
                maxLength={255}
                placeholder={instance.instance_id}
                className="flex-1 min-w-0 rounded border px-2 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-[#8D2D53]"
                style={{ borderColor: colors.wineBorder }}
                autoFocus
              />
              <button
                type="button"
                onClick={commitEdit}
                disabled={busy}
                className="inline-flex items-center justify-center rounded-full p-1 text-green-600 hover:bg-green-50 disabled:opacity-50"
                title={t('common.save', 'Salvar')}
              >
                <Check className="h-4 w-4" />
              </button>
              <button
                type="button"
                onClick={cancelEdit}
                disabled={busy}
                className="inline-flex items-center justify-center rounded-full p-1 text-muted-foreground hover:bg-muted disabled:opacity-50"
                title={t('common.cancel', 'Cancelar')}
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          ) : (
            <div className="flex items-center gap-2 flex-wrap">
              <p className="font-medium text-sm truncate" style={{ color: '#18181b' }}>
                {instance.friendly_name?.trim() || instance.instance_id}
              </p>
              {(() => {
                const phone = formatPhone(instance.phone_number);
                return phone ? (
                  <span
                    className="text-xs font-mono tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                    title={t('pages.whatsapp.instancesList.phoneTooltip', 'Número de telefone')}
                  >
                    {phone}
                  </span>
                ) : null;
              })()}
              <button
                type="button"
                onClick={startEdit}
                disabled={busy}
                className="inline-flex items-center justify-center rounded p-0.5 text-muted-foreground transition-opacity opacity-50 hover:opacity-100 disabled:opacity-30"
                title={t('pages.whatsapp.instancesList.renameTooltip', 'Renomear número')}
              >
                <Pencil className="h-3 w-3" />
              </button>
              {instance.is_default && (
                <span
                  className="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium"
                  style={{
                    backgroundColor: colors.defaultBadgeBg,
                    borderColor: colors.defaultBadgeBorder,
                    color: colors.defaultBadgeText,
                  }}
                >
                  <Star className="h-3 w-3 fill-current" />
                  {t('pages.whatsapp.instancesList.defaultBadge', 'Padrão')}
                </span>
              )}
            </div>
          )}
          <p className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
            {statusLabel}
          </p>
        </div>
      </div>

      <div className="flex gap-2 self-stretch sm:self-auto">
        {instance.status === 'disconnected' && (
          <button
            type="button"
            onClick={() => onReconnect(instance)}
            disabled={busy}
            className="inline-flex items-center justify-center gap-1.5 rounded-full border border-green-200 bg-white px-3 py-1.5 text-xs font-medium text-green-700 shadow-sm transition-all hover:-translate-y-0.5 hover:border-green-300 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
          >
            <RefreshCw className="h-3.5 w-3.5" />
            {t('pages.whatsapp.instancesList.reconnect', 'Reconectar')}
          </button>
        )}
        {!instance.is_default && (
          <button
            type="button"
            onClick={() => onSetDefault(instance.id)}
            disabled={busy}
            className="inline-flex items-center justify-center gap-1.5 rounded-full border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Star className="h-3.5 w-3.5" />
            {t('pages.whatsapp.instancesList.setDefault', 'Tornar padrão')}
          </button>
        )}
        <button
          type="button"
          onClick={() => onDelete(instance.id, instance.instance_id)}
          disabled={busy}
          className="inline-flex items-center justify-center gap-1.5 rounded-full border border-red-200 bg-white px-3 py-1.5 text-xs font-medium text-red-600 shadow-sm transition-all hover:-translate-y-0.5 hover:border-red-300 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Trash2 className="h-3.5 w-3.5" />
          {t('pages.whatsapp.instancesList.remove', 'Remover')}
        </button>
      </div>
    </div>
  );
}

/**
 * Lists all WhatsApp instances belonging to the current user with per-instance
 * actions (set as default, remove). Renders nothing when the user has zero
 * instances — the existing connection card handles the "first connect" flow.
 *
 * Multi-instance support is the goal of Fase 3; this component is the
 * surfaced UI for the backend already deployed in Fases 0-2.
 */
export function InstancesList() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const {
    instances,
    loading,
    setDefaultInstance,
    deleteInstance,
    updateInstanceLabel,
    isSettingDefault,
    isDeletingInstance,
    isUpdatingLabel,
  } = useInstancesConnect();

  const [addModalOpen, setAddModalOpen] = useState(false);
  const [reconnectTarget, setReconnectTarget] = useState<LocalInstance | null>(null);

  if (loading && instances.length === 0) {
    return null;
  }

  if (instances.length === 0) {
    return null;
  }

  const busy = isSettingDefault || isDeletingInstance || isUpdatingLabel;
  const canAddMore = instances.length < MAX_INSTANCES_PER_USER;

  const handleSetDefault = async (id: string) => {
    try {
      await setDefaultInstance(id);
      toast({
        title: t('pages.whatsapp.instancesList.defaultSet', 'Número padrão atualizado'),
      });
    } catch {
      // Hook already shows error toast via getErrorInfo.
    }
  };

  const handleRename = async (id: string, friendlyName: string) => {
    try {
      await updateInstanceLabel(id, friendlyName);
      toast({
        title: t('pages.whatsapp.instancesList.renamed', 'Número renomeado'),
      });
    } catch {
      // Hook already shows the error toast.
    }
  };

  const handleDelete = async (id: string, name: string) => {
    const confirmed = window.confirm(
      t('pages.whatsapp.instancesList.confirmDelete', {
        name,
        defaultValue: 'Remover o número "{{name}}"? Grupos vinculados a ele perderão o vínculo.',
      })
    );
    if (!confirmed) return;
    try {
      await deleteInstance(id);
      toast({
        title: t('pages.whatsapp.instancesList.removed', 'Número removido'),
      });
    } catch {
      // Hook already shows error toast.
    }
  };

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 mb-5 xs:mb-6 sm:mb-8 lg:mb-10"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center justify-between mb-4 gap-3 flex-wrap">
        <div>
          <h3
            className="text-base xs:text-lg font-semibold tracking-tight"
            style={{ color: '#18181b' }}
          >
            {t('pages.whatsapp.instancesList.title', 'Meus números')}
          </h3>
          <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
            {t('pages.whatsapp.instancesList.subtitle', {
              count: instances.length,
              max: MAX_INSTANCES_PER_USER,
              defaultValue: '{{count}} de {{max}} conectados',
            })}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {busy && <Loader2 className="h-4 w-4 animate-spin" style={{ color: colors.wine }} />}
          <button
            type="button"
            onClick={() => setAddModalOpen(true)}
            disabled={!canAddMore}
            title={
              !canAddMore
                ? t('pages.whatsapp.instancesList.atCapTooltip', {
                    max: MAX_INSTANCES_PER_USER,
                    defaultValue: 'Limite de {{max}} números atingido',
                  })
                : undefined
            }
            className="inline-flex items-center justify-center gap-1.5 rounded-full bg-[#8D2D53] px-3 py-1.5 text-xs font-medium text-white shadow-sm transition-all hover:-translate-y-0.5 hover:bg-[#732242] hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Plus className="h-3.5 w-3.5" />
            {t('pages.whatsapp.instancesList.addChip', 'Adicionar número')}
          </button>
        </div>
      </div>

      <div className="space-y-2 xs:space-y-3">
        {instances.map((inst) => (
          <InstanceRow
            key={inst.id}
            instance={inst}
            onSetDefault={handleSetDefault}
            onDelete={handleDelete}
            onRename={handleRename}
            onReconnect={setReconnectTarget}
            busy={busy}
          />
        ))}
      </div>

      <AddChipModal open={addModalOpen} onClose={() => setAddModalOpen(false)} />
      <ReconnectChipModal
        open={reconnectTarget !== null}
        onClose={() => setReconnectTarget(null)}
        instance={reconnectTarget}
      />
    </div>
  );
}
