import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { LocalInstance } from '@/hooks/use-instances-connect';
import { effectiveFriendlyName } from '@/lib/instance-label';
import { ROUTES } from '@/constants/routes';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';

const ALL_VALUE = '__all__';

const dotColors = {
  connected: '#22c55e',
  disconnected: '#ef4444',
  pending: '#f59e0b',
  superseded: '#a1a1aa',
};

function formatPhoneShort(raw: string | undefined): string | null {
  if (!raw) return null;
  const digits = raw.replace(/\D/g, '');
  if (digits.length === 0) return null;
  if (digits.startsWith('55') && (digits.length === 12 || digits.length === 13)) {
    const ddd = digits.slice(2, 4);
    const rest = digits.slice(4);
    if (rest.length === 9) return `(${ddd}) ${rest.slice(0, 5)}-${rest.slice(5)}`;
    if (rest.length === 8) return `(${ddd}) ${rest.slice(0, 4)}-${rest.slice(4)}`;
  }
  return digits;
}

/** Truncated session id used as a last-resort label when no friendly_name and no phone. */
function shortInstanceId(id: string): string {
  if (id.length <= 16) return id;
  return `${id.slice(0, 8)}…${id.slice(-4)}`;
}

interface ChipSelectorProps {
  /** Selected instance.id or null when allowAll is on and "Todos" is picked. */
  value: string | null;
  /** Called with the new instance.id, or null when "Todos" is selected. */
  onChange: (id: string | null) => void;
  instances: LocalInstance[];
  /** When true, adds a "Todos os números" option that resolves to null. */
  allowAll?: boolean;
  /** Disables the trigger (e.g. while a parent action is mid-flight). */
  disabled?: boolean;
  /** Visual placeholder when nothing is selected. */
  placeholder?: string;
  /**
   * When set, restricts the dropdown to instances matching this status.
   * Only use for *live* actions where picking a non-matching chip would
   * fail immediately (e.g. "send message now"). Configuration UIs should
   * leave it unset so the user can pick a disconnected chip and reconnect
   * it later — the option is rendered with a "desconectado" hint so the
   * state is still obvious.
   */
  statusFilter?: LocalInstance['status'];
}

/**
 * Picker for WhatsApp chips. Used by /groups (filter list) and the
 * automation form (pick the chip the automation will run on). Renders
 * each chip's friendly_name + abbreviated phone, with a status dot so
 * the user can tell at a glance which numbers are live.
 *
 * Optional "Todos os números" option (`allowAll`) is for filter UIs
 * where the user wants to see everything; the automation form omits it
 * because automations need a concrete chip.
 *
 * Empty state: when no instances are visible (either the user has zero
 * chips connected anywhere, or `statusFilter` filters everything out)
 * we render a disabled trigger with a hint and a link to /whatsapp so
 * the user can immediately add or reconnect a chip without hunting for
 * the page in the sidebar.
 */
export function ChipSelector({
  value,
  onChange,
  instances,
  allowAll = false,
  disabled = false,
  placeholder,
  statusFilter,
}: ChipSelectorProps) {
  const { t } = useTranslation();

  const visible = statusFilter
    ? instances.filter((i) => i.status === statusFilter)
    : instances;

  const handleChange = (next: string) => {
    if (next === ALL_VALUE) {
      onChange(null);
      return;
    }
    onChange(next);
  };

  // Empty state. Two flavors:
  //   - statusFilter set AND zero matches → "no connected chip right now"
  //     (filter-induced empty; other chips exist but are offline)
  //   - no statusFilter AND zero instances → "no chip at all"
  // Both render the same CTA: a link to /whatsapp where the user can
  // either reconnect or add a new chip.
  if (visible.length === 0) {
    const filtered = statusFilter !== undefined && instances.length > 0;
    return (
      <div className="flex flex-col gap-1.5 w-full sm:w-72">
        <div className="rounded-md border border-dashed border-amber-300 bg-amber-50/70 px-3 py-2 text-xs text-amber-800">
          {filtered
            ? t(
                'components.chipSelector.emptyConnected',
                'Nenhum número conectado no momento.'
              )
            : t(
                'components.chipSelector.empty',
                'Nenhum número WhatsApp configurado.'
              )}
        </div>
        <Link
          to={ROUTES.WHATSAPP}
          className="text-xs font-medium text-[#8D2D53] hover:underline"
        >
          {t('components.chipSelector.connectLink', 'Conectar um número →')}
        </Link>
      </div>
    );
  }

  // Select expects a non-empty value; map null to ALL_VALUE for the UI.
  const internalValue = value ?? (allowAll ? ALL_VALUE : '');

  return (
    <Select value={internalValue} onValueChange={handleChange} disabled={disabled}>
      <SelectTrigger className="w-full sm:w-72">
        <SelectValue
          placeholder={
            placeholder ?? t('components.chipSelector.placeholder', 'Selecione um número')
          }
        />
      </SelectTrigger>
      <SelectContent>
        {allowAll && (
          <SelectItem value={ALL_VALUE}>
            {t('components.chipSelector.all', 'Todos os números')}
          </SelectItem>
        )}
        {visible.map((inst) => {
          const friendly = effectiveFriendlyName(inst);
          const phone = formatPhoneShort(inst.phone_number);
          // Prefer the user-set friendly name; otherwise the phone reads
          // far better than the raw session id, which is unreadable.
          const primary = friendly || phone || shortInstanceId(inst.instance_id);
          // Show phone as a secondary line only when it isn't already the
          // primary (avoids duplication for un-named chips).
          const secondary = friendly && phone ? phone : null;
          // Visual cue for non-connected chips: muted text + hint. The
          // option stays SELECTABLE on purpose — config flows should let
          // the user pick a chip that's offline now and bring it online
          // later, otherwise editing an automation while its chip blinks
          // would silently lose the selection.
          const isOffline = inst.status !== 'connected';
          return (
            <SelectItem key={inst.id} value={inst.id}>
              <div
                className={`flex items-center gap-2 min-w-0 whitespace-nowrap ${
                  isOffline ? 'opacity-70' : ''
                }`}
              >
                <span
                  className="inline-block h-2 w-2 rounded-full shrink-0"
                  style={{ backgroundColor: dotColors[inst.status] }}
                  aria-hidden="true"
                />
                <span className="truncate">{primary}</span>
                {secondary && (
                  <span className="text-xs text-muted-foreground font-mono shrink-0">
                    {secondary}
                  </span>
                )}
                {isOffline && (
                  <span className="text-[10px] uppercase tracking-wide text-red-600 shrink-0">
                    {t('components.chipSelector.disconnected', 'desconectado')}
                  </span>
                )}
              </div>
            </SelectItem>
          );
        })}
      </SelectContent>
    </Select>
  );
}
