import { Card, CardContent } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { MAX_SEND_GROUPS } from '@/config/automation-categories';
import { type LocalInstance, useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { effectiveFriendlyName } from '@/lib/instance-label';
import type { Automation, AutomationGroup } from '@/types/automation';
import { Search, Star } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

// Sentinel value for the "Auto" option in the per-group chip Select.
// Radix Select rejects empty-string values, so we use a non-empty token
// and translate it to null/absence when persisting the override map.
const CHIP_AUTO = '__auto__';

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

// Short, human-readable label for a chip. Prefers the user-set friendly
// name (when it isn't the raw session id placeholder); falls back to a
// formatted phone, then the truncated session id.
function chipLabel(inst: LocalInstance): string {
  const friendly = effectiveFriendlyName(inst);
  if (friendly) return friendly;
  const phone = formatPhoneShort(inst.phone_number);
  if (phone) return phone;
  return inst.instance_id.length > 16
    ? `${inst.instance_id.slice(0, 8)}…${inst.instance_id.slice(-4)}`
    : inst.instance_id;
}

// Mirrors the backend cap in internal/lib/quota/quota.go (MaxGroupsPerInstance).
// Keeping it as a local const so the UI can short-circuit before the API
// returns ResourceExhausted; the server still enforces it for callers that
// bypass the form.
const MAX_GROUPS_PER_INSTANCE = 20;

// Counts how many of the currently selected groups belong to each chip.
// Returns a map keyed by instance_id -> count; selected groups whose owning
// chip is unknown (no instance_id on the AutomationGroup) are ignored — they
// can't trigger the cap because no chip can be identified for them.
function countSelectedByInstance(
  selectedIds: string[],
  groups: AutomationGroup[]
): Map<string, number> {
  const byWaId = new Map<string, AutomationGroup>();
  for (const g of groups) byWaId.set(g.wa_group_id, g);

  const counts = new Map<string, number>();
  for (const waId of selectedIds) {
    const group = byWaId.get(waId);
    if (!group?.instance_id) continue;
    counts.set(group.instance_id, (counts.get(group.instance_id) ?? 0) + 1);
  }
  return counts;
}

interface GroupsListCheckboxProps {
  groups: AutomationGroup[];
  selectedIds: string[];
  onToggle: (waGroupId: string, checked: boolean) => void;
  onToggleFavorite: (groupId: string) => void;
  maxGroups?: number;
  disabled?: boolean;
  showSearch?: boolean;
  // When set, groups whose owning chip already has this many selections
  // become non-toggleable (unless they are the one being unchecked).
  perChipCap?: number;
  selectedCountByInstance?: Map<string, number>;
  // Chips currently enabled on the automation. The per-group chip
  // dropdown only renders when this has 2+ entries — single-chip
  // automations have nothing to override, matching backend semantics.
  selectedChipIds?: string[];
  // Current overrides: wa_group_id → instance_id. Groups absent here
  // follow the automatic sticky-hash distribution at dispatch time.
  chipOverrides?: Record<string, string>;
  // Called when the user pins / un-pins a chip for a group. instanceId
  // is null when the user selects "Auto" (clear the override).
  onOverrideChange?: (waGroupId: string, instanceId: string | null) => void;
}

export function GroupsListCheckbox({
  groups,
  selectedIds,
  onToggle,
  onToggleFavorite,
  maxGroups,
  disabled = false,
  showSearch = false,
  perChipCap,
  selectedCountByInstance,
  selectedChipIds,
  chipOverrides,
  onOverrideChange,
}: GroupsListCheckboxProps) {
  const { instances } = useInstancesConnect();
  // Multi-chip routing UI only appears when the user actually has 2+
  // chips on the automation. Single-chip automations skip the dropdown
  // entirely — zero visual change from before.
  const showChipRouting = (selectedChipIds?.length ?? 0) >= 2 && !!onOverrideChange;
  const eligibleInstances = useMemo(() => {
    if (!showChipRouting) return [] as LocalInstance[];
    const set = new Set(selectedChipIds);
    return instances.filter((i) => set.has(i.id));
  }, [showChipRouting, selectedChipIds, instances]);
  const { t } = useTranslation();
  const [searchTerm, setSearchTerm] = useState('');

  const filteredGroups = useMemo(() => {
    if (!searchTerm.trim()) return groups;
    const lowerSearch = searchTerm.toLowerCase();
    return groups.filter((g) => g.name.toLowerCase().includes(lowerSearch));
  }, [groups, searchTerm]);

  return (
    <Card>
      <CardContent className="pt-4">
        {showSearch && (
          <div className="relative mb-3">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder={t('pages.offerAutomations.form.searchGroups')}
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="pl-8"
            />
          </div>
        )}
        <div className="max-h-40 overflow-y-auto">
          {filteredGroups.map((group) => {
            const isSelected = selectedIds.includes(group.wa_group_id);
            const chipCount =
              group.instance_id !== undefined && selectedCountByInstance
                ? (selectedCountByInstance.get(group.instance_id) ?? 0)
                : 0;
            const chipFull =
              !isSelected && perChipCap !== undefined && chipCount >= perChipCap;
            const isDisabled =
              disabled ||
              (!isSelected && maxGroups !== undefined && selectedIds.length >= maxGroups) ||
              chipFull;

            // Per-group chip pin. Only renders for selected groups in
            // multi-chip mode; absent chips are disabled with a hint.
            const participants = group.participating_instance_ids ?? [];
            const overrideValue =
              showChipRouting && isSelected ? (chipOverrides?.[group.wa_group_id] ?? '') : '';

            // Chips selecionados na automação que efetivamente são membros
            // deste grupo. Quando só 1 chip é elegível, não há override real
            // pra escolher — o backend usaria esse chip de qualquer jeito —
            // então renderizamos um label estático em vez do <Select>. O
            // dropdown só aparece quando 2+ chips podem mesmo enviar pro
            // grupo (caso onde o "Auto" tem semântica de espalhar via hash
            // ou o operador pode pinar manualmente um dos membros).
            const inGroup = (inst: LocalInstance): boolean =>
              participants.length === 0
                ? group.instance_id === inst.id
                : participants.includes(inst.id);
            const eligibleInGroup = showChipRouting && isSelected
              ? eligibleInstances.filter(inGroup)
              : [];

            return (
              <div key={group.id} className="flex items-center space-x-2 mb-2 group/item">
                <Checkbox
                  checked={isSelected}
                  onCheckedChange={(checked) => onToggle(group.wa_group_id, Boolean(checked))}
                  disabled={isDisabled}
                />
                <Label
                  className="cursor-pointer flex-1"
                  title={
                    chipFull
                      ? t('pages.offerAutomations.validation.chipFullTooltip', {
                          max: perChipCap,
                          defaultValue:
                            'Este número já tem {{max}} grupos selecionados nesta automação',
                        })
                      : undefined
                  }
                >
                  {group.name}
                </Label>
                {showChipRouting && isSelected && eligibleInGroup.length === 1 && (
                  <span
                    className="inline-flex h-7 items-center rounded-md border bg-muted/30 px-2 text-xs text-muted-foreground shrink-0 w-[140px] truncate"
                    title={t('pages.offerAutomations.form.chipOverride.onlyMember', {
                      defaultValue: 'Só este número é membro deste grupo',
                    })}
                  >
                    📱 <span className="ml-1 truncate">{chipLabel(eligibleInGroup[0])}</span>
                  </span>
                )}
                {showChipRouting && isSelected && eligibleInGroup.length >= 2 && (
                  <Select
                    value={overrideValue || CHIP_AUTO}
                    onValueChange={(v) =>
                      onOverrideChange?.(group.wa_group_id, v === CHIP_AUTO ? null : v)
                    }
                  >
                    <SelectTrigger className="h-7 text-xs w-[140px] shrink-0">
                      <SelectValue placeholder="🤖 Auto" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={CHIP_AUTO}>
                        🤖{' '}
                        {t('pages.offerAutomations.form.chipOverride.auto', {
                          defaultValue: 'Auto',
                        })}
                      </SelectItem>
                      {eligibleInGroup.map((inst) => (
                        <SelectItem key={inst.id} value={inst.id}>
                          📱 {chipLabel(inst)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
                <button
                  type="button"
                  onClick={() => onToggleFavorite(group.id)}
                  className="opacity-50 hover:opacity-100 transition-opacity"
                  title={
                    group.is_favorite ? t('common.removeFromFavorites') : t('common.addToFavorites')
                  }
                >
                  <Star
                    className={`h-4 w-4 ${
                      group.is_favorite
                        ? 'fill-yellow-400 text-yellow-400'
                        : 'text-muted-foreground'
                    }`}
                  />
                </button>
              </div>
            );
          })}
          {filteredGroups.length === 0 && searchTerm && (
            <p className="text-sm text-muted-foreground text-center py-2">
              {t('common.noResults')}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

interface PerChipBreakdownProps {
  countByInstance: Map<string, number>;
  cap: number;
}

function PerChipBreakdown({ countByInstance, cap }: PerChipBreakdownProps) {
  const { t } = useTranslation();
  const { instances } = useInstancesConnect();

  // Map db-id -> friendly label so badges show a recognizable string
  // instead of the database id integer. Prefer (in order) user-set
  // friendly_name, phone number, raw session id, fallback "número #id".
  const labelByInstance = useMemo(() => {
    const map = new Map<string, string>();
    for (const inst of instances) {
      const friendly = effectiveFriendlyName(inst);
      const label =
        friendly ||
        (inst.phone_number ? `+${inst.phone_number}` : inst.instance_id) ||
        `número #${inst.id}`;
      map.set(inst.id, label);
    }
    return map;
  }, [instances]);

  if (countByInstance.size === 0) return null;

  const entries = Array.from(countByInstance.entries()).sort((a, b) => b[1] - a[1]);

  return (
    <div className="flex flex-wrap gap-2 text-[11px] xs:text-xs mt-1">
      {entries.map(([instanceID, count]) => {
        const atCap = count >= cap;
        const label = labelByInstance.get(instanceID) ?? `número #${instanceID}`;
        return (
          <span
            key={instanceID}
            className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 ${
              atCap
                ? 'border-amber-300 bg-amber-50 text-amber-700'
                : 'border-muted bg-muted/30 text-muted-foreground'
            }`}
            title={t('pages.offerAutomations.form.perChipBreakdownTitle', {
              defaultValue: 'Grupos por número nesta automação',
            })}
          >
            {label}: {count}/{cap}
          </span>
        );
      })}
    </div>
  );
}

interface SendGroupsSelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  groups: AutomationGroup[];
  onToggleFavorite: (groupId: string) => Promise<void>;
  // When true, no global or per-chip cap is enforced and the counter/hint UI
  // is hidden. Used by /offer-generate, which has no server-side quota check
  // (the per-instance cap in internal/lib/quota only runs from the
  // /offer-automations handler).
  unlimited?: boolean;
}

export function SendGroupsSelector({
  formData,
  setFormData,
  groups,
  onToggleFavorite,
  unlimited = false,
}: SendGroupsSelectorProps) {
  const { t } = useTranslation();
  const { toast } = useToast();

  const selectedIds = formData.send_groups || [];
  const countByInstance = useMemo(
    () => countSelectedByInstance(selectedIds, groups),
    [selectedIds, groups]
  );

  const handleToggle = (waGroupId: string, checked: boolean) => {
    setFormData((prev) => {
      const current = prev.send_groups || [];
      const isSelected = current.includes(waGroupId);
      if (!unlimited && checked && !isSelected && current.length >= MAX_SEND_GROUPS) {
        toast({
          title: t('pages.offerAutomations.validation.groupsLimit'),
          description: t('pages.offerAutomations.validation.groupsLimitMessage', {
            max: MAX_SEND_GROUPS,
          }),
          variant: 'destructive',
        });
        return prev;
      }
      // Per-chip cap guard: block the 21st group on any single chip even if
      // we are still under MAX_SEND_GROUPS overall. Unchecking is always allowed.
      if (!unlimited && checked && !isSelected) {
        const target = groups.find((g) => g.wa_group_id === waGroupId);
        if (target?.instance_id) {
          const current = countByInstance.get(target.instance_id) ?? 0;
          if (current >= MAX_GROUPS_PER_INSTANCE) {
            toast({
              title: t('pages.offerAutomations.validation.chipFull', {
                defaultValue: 'Limite de grupos por número atingido',
              }),
              description: t('pages.offerAutomations.validation.chipFullMessage', {
                max: MAX_GROUPS_PER_INSTANCE,
                defaultValue:
                  'Cada número aceita no máximo {{max}} grupos por automação. Distribua os grupos entre mais números.',
              }),
              variant: 'destructive',
            });
            return prev;
          }
        }
      }
      // When the user unchecks a group, also drop any chip override
      // that was pinned to it so we don't leave orphan keys in the map.
      let nextOverrides = prev.chip_overrides;
      if (!checked && nextOverrides && waGroupId in nextOverrides) {
        const { [waGroupId]: _dropped, ...rest } = nextOverrides;
        nextOverrides = rest;
      }
      return {
        ...prev,
        send_groups: checked ? [...current, waGroupId] : current.filter((g) => g !== waGroupId),
        chip_overrides: nextOverrides,
      };
    });
  };

  const handleOverrideChange = (waGroupId: string, instanceId: string | null) => {
    setFormData((prev) => {
      const current = prev.chip_overrides ?? {};
      if (instanceId === null) {
        // "Auto" — drop the override so the dispatch falls through to hash.
        if (!(waGroupId in current)) return prev;
        const { [waGroupId]: _dropped, ...rest } = current;
        return { ...prev, chip_overrides: rest };
      }
      if (current[waGroupId] === instanceId) return prev;
      return { ...prev, chip_overrides: { ...current, [waGroupId]: instanceId } };
    });
  };

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <Label className="cursor-pointer">
        {t('pages.offerAutomations.form.sendGroups')} *{' '}
        {!unlimited && (
          <span className="text-xs text-muted-foreground">
            ({selectedIds.length}/{MAX_SEND_GROUPS})
          </span>
        )}
      </Label>
      {!unlimited && (
        <p className="text-xs text-muted-foreground">
          {t('pages.offerAutomations.form.sendGroupsHint', { max: MAX_SEND_GROUPS })}
        </p>
      )}
      {!unlimited && (
        <PerChipBreakdown countByInstance={countByInstance} cap={MAX_GROUPS_PER_INSTANCE} />
      )}
      <GroupsListCheckbox
        groups={groups}
        selectedIds={selectedIds}
        onToggle={handleToggle}
        onToggleFavorite={onToggleFavorite}
        maxGroups={unlimited ? undefined : MAX_SEND_GROUPS}
        perChipCap={unlimited ? undefined : MAX_GROUPS_PER_INSTANCE}
        selectedCountByInstance={countByInstance}
        selectedChipIds={formData.instance_ids}
        chipOverrides={formData.chip_overrides}
        onOverrideChange={handleOverrideChange}
        showSearch
      />
    </div>
  );
}

const MAX_MONITOR_GROUPS = 3;

interface MonitorGroupsSelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  allGroups: AutomationGroup[];
  onToggleFavorite: (groupId: string) => Promise<void>;
  maxGroups?: number;
  hideLabel?: boolean;
}

export function MonitorGroupsSelector({
  formData,
  setFormData,
  allGroups,
  onToggleFavorite,
  maxGroups = MAX_MONITOR_GROUPS,
  hideLabel = false,
}: MonitorGroupsSelectorProps) {
  const { t } = useTranslation();
  const { toast } = useToast();

  const selectedIds = formData.monitor_groups || [];
  const countByInstance = useMemo(
    () => countSelectedByInstance(selectedIds, allGroups),
    [selectedIds, allGroups]
  );

  const handleToggle = (waGroupId: string, checked: boolean) => {
    setFormData((prev) => {
      const current = prev.monitor_groups || [];
      const isSelected = current.includes(waGroupId);
      if (checked && !isSelected && current.length >= maxGroups) {
        toast({
          title: t('pages.offerAutomations.validation.maxMonitorGroups'),
          description: t('pages.offerAutomations.validation.maxMonitorGroupsMessage'),
          variant: 'destructive',
        });
        return prev;
      }
      // Same per-chip cap as send_groups so monitor lists do not concentrate
      // a single chip into ban risk either.
      if (checked && !isSelected) {
        const target = allGroups.find((g) => g.wa_group_id === waGroupId);
        if (target?.instance_id) {
          const currentChipCount = countByInstance.get(target.instance_id) ?? 0;
          if (currentChipCount >= MAX_GROUPS_PER_INSTANCE) {
            toast({
              title: t('pages.offerAutomations.validation.chipFull', {
                defaultValue: 'Limite de grupos por número atingido',
              }),
              description: t('pages.offerAutomations.validation.chipFullMessage', {
                max: MAX_GROUPS_PER_INSTANCE,
                defaultValue:
                  'Cada número aceita no máximo {{max}} grupos por automação. Distribua os grupos entre mais números.',
              }),
              variant: 'destructive',
            });
            return prev;
          }
        }
      }
      const next = checked ? [...current, waGroupId] : current.filter((g) => g !== waGroupId);
      return { ...prev, monitor_groups: next };
    });
  };

  const selectedCount = selectedIds.length;

  return (
    <div className="space-y-1.5 xs:space-y-2">
      {!hideLabel && (
        <Label className="cursor-pointer">
          {t('pages.offerAutomations.form.monitorGroups')}{' '}
          <span className="text-xs text-muted-foreground">
            ({selectedCount}/{maxGroups} {t('pages.offerAutomations.form.groupsSelected')})
          </span>
        </Label>
      )}
      <PerChipBreakdown countByInstance={countByInstance} cap={MAX_GROUPS_PER_INSTANCE} />
      <GroupsListCheckbox
        groups={allGroups}
        selectedIds={selectedIds}
        onToggle={handleToggle}
        onToggleFavorite={onToggleFavorite}
        maxGroups={maxGroups}
        perChipCap={MAX_GROUPS_PER_INSTANCE}
        selectedCountByInstance={countByInstance}
        showSearch
      />
    </div>
  );
}
