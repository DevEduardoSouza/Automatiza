import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { LocalInstance } from '@/hooks/use-instances-connect';
import { useTranslation } from 'react-i18next';

const ALL_VALUE = '__all__';

const dotColors = {
  connected: '#22c55e',
  disconnected: '#ef4444',
  pending: '#f59e0b',
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
  /** Restrict the dropdown to instances matching this status (e.g. only connected). */
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
          const friendly = inst.friendly_name?.trim();
          const phone = formatPhoneShort(inst.phone_number);
          // Prefer the user-set friendly name; otherwise the phone reads
          // far better than the raw session id, which is unreadable.
          const primary = friendly || phone || shortInstanceId(inst.instance_id);
          // Show phone as a secondary line only when it isn't already the
          // primary (avoids duplication for un-named chips).
          const secondary = friendly && phone ? phone : null;
          return (
            <SelectItem key={inst.id} value={inst.id}>
              <div className="flex items-center gap-2 min-w-0 whitespace-nowrap">
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
              </div>
            </SelectItem>
          );
        })}
      </SelectContent>
    </Select>
  );
}
