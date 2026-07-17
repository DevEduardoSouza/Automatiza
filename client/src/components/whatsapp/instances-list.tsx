import type { LocalInstance } from '@/hooks/use-instances-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useProfilesConnect } from '@/hooks/use-profiles-connect';
import { useToast } from '@/hooks/use-toast';
import { Code, ConnectError } from '@connectrpc/connect';
import { Check, Loader2, Pencil, QrCode, RefreshCw, Trash2, Wifi, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AddChipModal } from './add-chip-modal';
import { ReconnectChipModal } from './reconnect-chip-modal';

/**
 * Fallback chip cap, used only until the server's limits land (or if the call
 * fails). The real cap is per-account and comes from the server — an account can
 * be granted more chips without touching this file. Keep it equal to the server
 * default (quota.MaxInstancesPerUser) so the two never disagree again: the UI
 * saying 15 while the server said 30 is what made "limite atingido" unreadable.
 */
const DEFAULT_MAX_INSTANCES = 15;

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
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
  onDelete: (id: string, name: string, status: LocalInstance['status']) => void;
  onRename: (id: string, friendlyName: string) => Promise<void>;
  onReconnect: (instance: LocalInstance) => void;
  busy: boolean;
}

function InstanceRow({
  instance,
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
                // biome-ignore lint/a11y/noAutofocus: inline rename — input só monta após clique explícito no lápis, focus imediato é a UX esperada
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
        <button
          type="button"
          onClick={() => onDelete(instance.id, instance.instance_id, instance.status)}
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
 * actions (rename, reconnect, remove). Renders nothing when the user has zero
 * instances — the existing connection card handles the "first connect" flow.
 *
 * The is_default flag is now managed entirely by the backend (auto-promote
 * on first chip / on default-delete) so this component does not expose any
 * "Tornar padrão" affordance.
 */
export function InstancesList() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const {
    instances,
    loading,
    deleteInstance,
    updateInstanceLabel,
    isDeletingInstance,
    isUpdatingLabel,
  } = useInstancesConnect();
  const { limits } = useProfilesConnect();

  // Per-account cap, straight from the server (an account can be granted more
  // chips server-side). Falls back to the default only while limits load.
  const maxInstances = limits?.max_instances || DEFAULT_MAX_INSTANCES;

  const [addModalOpen, setAddModalOpen] = useState(false);
  const [reconnectTarget, setReconnectTarget] = useState<LocalInstance | null>(null);
  const [showHistory, setShowHistory] = useState(false);

  // Hide noise from the default view: superseded rows (already replaced
  // by a newer session of the same chip) and zombies (disconnected for
  // >7 days without ever capturing a phone — abandoned QR scans). The
  // operator can still see them via the "Mostrar histórico" toggle.
  const ZOMBIE_THRESHOLD_MS = 7 * 24 * 60 * 60 * 1000;
  const now = Date.now();
  const isHidden = (inst: LocalInstance) => {
    if (inst.status === 'superseded') return true;
    if (inst.status === 'disconnected' && !inst.phone_number) {
      const created = new Date(inst.created_at).getTime();
      if (!Number.isNaN(created) && now - created > ZOMBIE_THRESHOLD_MS) {
        return true;
      }
    }
    return false;
  };
  const visibleInstances = showHistory ? instances : instances.filter((i) => !isHidden(i));
  const hiddenCount = instances.length - instances.filter((i) => !isHidden(i)).length;

  if (loading && instances.length === 0) {
    return null;
  }

  if (instances.length === 0) {
    return null;
  }

  const busy = isDeletingInstance || isUpdatingLabel;
  const canAddMore = visibleInstances.length < maxInstances;

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

  const handleDelete = async (
    id: string,
    name: string,
    status: LocalInstance['status']
  ) => {
    const connectedWarning = t(
      'pages.whatsapp.instancesList.confirmDeleteConnected',
      {
        name,
        defaultValue:
          'O número "{{name}}" está CONECTADO. Removê-lo vai DESCONECTAR o WhatsApp e você precisará escanear um novo QR Code para reconectar. Deseja continuar?',
      }
    );
    const isConnected = status === 'connected';
    const confirmed = window.confirm(
      isConnected
        ? connectedWarning
        : t('pages.whatsapp.instancesList.confirmDelete', {
            name,
            defaultValue:
              'Remover o número "{{name}}"? Grupos vinculados a ele perderão o vínculo.',
          })
    );
    if (!confirmed) return;
    try {
      // Chip conectado exige force=true: sem isso o backend devolve
      // FailedPrecondition e a remoção nunca acontece (era o bug do "não
      // consigo deslogar o WhatsApp").
      await deleteInstance(id, isConnected);
      toast({
        title: t('pages.whatsapp.instancesList.removed', 'Número removido'),
      });
    } catch (error) {
      // O backend também barra um chip VIVO na WhatsApp cuja linha ainda
      // aparece como "desconectado" (status defasado). Nesse caso oferecemos
      // a desconexão forçada em vez de deixar o usuário travado.
      if (
        !isConnected &&
        error instanceof ConnectError &&
        error.code === Code.FailedPrecondition &&
        window.confirm(connectedWarning)
      ) {
        try {
          await deleteInstance(id, true);
          toast({
            title: t('pages.whatsapp.instancesList.removed', 'Número removido'),
          });
        } catch {
          // Hook already shows error toast.
        }
      }
      // Caso contrário, o hook já mostrou o toast de erro.
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
              count: visibleInstances.length,
              max: maxInstances,
              defaultValue: '{{count}} de {{max}} conectados',
            })}
          </p>
        </div>
        {busy && <Loader2 className="h-4 w-4 animate-spin" style={{ color: colors.wine }} />}
      </div>

      <div className="space-y-2 xs:space-y-3">
        {visibleInstances.map((inst) => (
          <InstanceRow
            key={inst.id}
            instance={inst}
            onDelete={handleDelete}
            onRename={handleRename}
            onReconnect={setReconnectTarget}
            busy={busy}
          />
        ))}
      </div>

      {hiddenCount > 0 && (
        <div className="mt-3 flex justify-center">
          <button
            type="button"
            onClick={() => setShowHistory((v) => !v)}
            className="text-xs xs:text-sm font-medium underline-offset-4 hover:underline"
            style={{ color: colors.wineTextMuted }}
          >
            {showHistory
              ? t('pages.whatsapp.instancesList.hideHistory', {
                  defaultValue: 'Esconder histórico',
                })
              : t('pages.whatsapp.instancesList.showHistory', {
                  count: hiddenCount,
                  defaultValue: 'Mostrar histórico ({{count}} sessões antigas)',
                })}
          </button>
        </div>
      )}

      {/* QR-pairing card below the list — mirrors the legacy ConnectionCard
          look (wine bg, header with QR icon, divider, primary glow button)
          so the "add another number" CTA reads as the same first-class
          action it was before multi-instance shipped. The small header
          button stays as a quick-access shortcut. */}
      <div
        className="mt-4 xs:mt-5 sm:mt-6 rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
        style={{ backgroundColor: '#ffffff', borderColor: colors.wineBorder }}
      >
        <div className="flex items-center gap-3 xs:gap-4 mb-3 xs:mb-4">
          <div
            className="p-2 xs:p-2.5 rounded-lg xs:rounded-xl border"
            style={{
              backgroundColor: 'rgba(141, 45, 83, 0.08)',
              borderColor: colors.wineBorder,
            }}
          >
            <QrCode
              className="h-6 w-6 xs:h-7 xs:w-7 sm:h-8 sm:w-8"
              style={{ color: colors.wine }}
            />
          </div>
          <div>
            <div
              className="text-base xs:text-lg font-semibold tracking-tight"
              style={{ color: '#18181b' }}
            >
              {canAddMore
                ? t('pages.whatsapp.instancesList.addAnother', 'Adicionar outro número')
                : t('pages.whatsapp.instancesList.atCapTitle', 'Limite atingido')}
            </div>
            <div
              className="text-xs xs:text-sm tracking-tight"
              style={{ color: colors.wineTextMuted }}
            >
              {canAddMore
                ? t(
                    'pages.whatsapp.instancesList.addAnotherDescription',
                    'Conecte mais um número escaneando o QR code com o WhatsApp.'
                  )
                : t('pages.whatsapp.instancesList.atCapHint', {
                    max: maxInstances,
                    defaultValue: 'Você atingiu o limite de {{max}} números',
                  })}
            </div>
          </div>
        </div>

        <div className="h-px mb-4 xs:mb-5" style={{ backgroundColor: colors.wineBorder }} />

        <button
          type="button"
          onClick={() => canAddMore && setAddModalOpen(true)}
          disabled={!canAddMore || busy}
          title={
            !canAddMore
              ? t('pages.whatsapp.instancesList.atCapTooltip', {
                  max: maxInstances,
                  defaultValue: 'Limite de {{max}} números atingido',
                })
              : undefined
          }
          className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
        >
          <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
            <div className="auth-btn-glow__glow absolute inset-0" />
          </div>
          <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
            <Wifi className="h-4 w-4 xs:h-5 xs:w-5" />
            {canAddMore
              ? t('pages.whatsapp.instancesList.addAnotherCta', 'Conectar novo número')
              : t(
                  'pages.whatsapp.instancesList.atCapCta',
                  `Limite de ${maxInstances} atingido`
                )}
          </span>
        </button>

        {canAddMore && (
          <p
            className="text-[11px] xs:text-xs text-center mt-2 xs:mt-3"
            style={{ color: colors.wineTextMuted }}
          >
            {t('pages.whatsapp.instancesList.addAnotherHint', {
              remaining: maxInstances - instances.length,
              max: maxInstances,
              defaultValue: 'Você pode adicionar mais {{remaining}} (limite: {{max}})',
            })}
          </p>
        )}
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
