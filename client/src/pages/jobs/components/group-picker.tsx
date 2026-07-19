import { useDebouncedValue } from '@/hooks/use-debounced-value';
import { useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { T } from './handoff-tokens';

interface Group {
  id: string;
  name: string;
  wa_group_id: string;
  is_favorite?: boolean;
}

interface GroupPickerProps {
  groups: Group[];
  selectedGroups: string[];
  searchTerm: string;
  loading?: boolean;
  onSearchChange: (value: string) => void;
  onToggleGroup: (id: string) => void;
  onSelectAll: (ids: string[]) => void;
  onClearAll: () => void;
  onToggleGroups: (ids: string[], selected: boolean) => void;
  onToggleFavorite?: (id: string, next: boolean) => void;
}

interface PickerGroup {
  id: string;
  name: string;
  num: string | null;
  is_favorite: boolean;
}

interface Section {
  key: string;
  label: string;
  groups: PickerGroup[];
}

/**
 * Deriva "seção" (comunidade) e "número" de um nome de grupo livre. Os dados de
 * produção não têm faixa/comunidade limpa como o mock do handoff — agrupamos
 * pelo nome-base (sem o número/#NN e emojis nas pontas). ponytail: heurística
 * com teto conhecido (nomes muito livres viram singletons em "Outros"); upgrade
 * = campo de comunidade real no backend.
 */
function deriveSection(name: string): { base: string; num: string | null } {
  const numMatch = name.match(/#\s*0*(\d+)/) ?? name.match(/(?:^|\s)0*(\d+)(?:\s|$)/);
  const num = numMatch ? `#${numMatch[1]}` : null;
  let base = name;
  if (numMatch) base = base.replace(numMatch[0], ' ');
  base = base
    .replace(/[^\p{L}\p{N}]+/gu, ' ')
    .replace(/\s+/g, ' ')
    .trim();
  return { base, num };
}

/**
 * Agrupa os grupos em seções. Bases com ≥2 grupos viram uma seção própria
 * (ordenada pelo número); o resto cai em "Outros grupos".
 */
function buildSections(groups: Group[], othersLabel: string): Section[] {
  const map = new Map<string, { label: string; groups: PickerGroup[] }>();
  for (const g of groups) {
    const { base, num } = deriveSection(g.name);
    const key = base.length >= 2 ? base.toLowerCase() : `__single__${g.id}`;
    const entry = map.get(key) ?? { label: base, groups: [] };
    entry.groups.push({ id: g.id, name: g.name, num, is_favorite: !!g.is_favorite });
    map.set(key, entry);
  }

  const numOf = (n: string | null) =>
    n ? Number.parseInt(n.slice(1), 10) : Number.POSITIVE_INFINITY;
  const named: Section[] = [];
  const others: PickerGroup[] = [];
  for (const [key, entry] of map) {
    if (entry.groups.length >= 2 && entry.label) {
      entry.groups.sort((a, b) => numOf(a.num) - numOf(b.num) || a.name.localeCompare(b.name));
      named.push({ key, label: entry.label.toUpperCase(), groups: entry.groups });
    } else {
      others.push(...entry.groups);
    }
  }
  named.sort((a, b) => a.label.localeCompare(b.label));
  if (others.length > 0) {
    others.sort((a, b) => a.name.localeCompare(b.name));
    named.push({ key: '__others__', label: othersLabel.toUpperCase(), groups: others });
  }
  return named;
}

/**
 * Painel de seleção de grupos no estilo do handoff: busca, selecionar todos,
 * limpar, favoritos, atalhos por seção (com scroll suave) e seções colapsáveis
 * com checkbox tri-state e favoritar por estrela.
 */
export function GroupPicker({
  groups,
  selectedGroups,
  searchTerm,
  loading = false,
  onSearchChange,
  onToggleGroup,
  onSelectAll,
  onClearAll,
  onToggleGroups,
  onToggleFavorite,
}: GroupPickerProps) {
  const { t } = useTranslation();
  const debounced = useDebouncedValue(searchTerm, 250).trim().toLowerCase();
  const [onlyFav, setOnlyFav] = useState(false);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const listRef = useRef<HTMLDivElement>(null);
  const sectionEls = useRef<Map<string, HTMLDivElement>>(new Map());

  const selected = useMemo(() => new Set(selectedGroups), [selectedGroups]);

  const allSections = useMemo(
    () => buildSections(groups, t('pages.jobs.schedule.otherGroups')),
    [groups, t]
  );

  const sections = useMemo(() => {
    if (!debounced && !onlyFav) return allSections;
    return allSections
      .map((s) => ({
        ...s,
        groups: s.groups.filter((g) => {
          if (onlyFav && !g.is_favorite) return false;
          if (!debounced) return true;
          return `${g.name} ${g.num ?? ''}`.toLowerCase().includes(debounced);
        }),
      }))
      .filter((s) => s.groups.length > 0);
  }, [allSections, debounced, onlyFav]);

  const visibleIds = useMemo(() => sections.flatMap((s) => s.groups.map((g) => g.id)), [sections]);
  const searchActive = debounced.length > 0;
  const singleSection = sections.length === 1;

  const isOpen = (key: string) => searchActive || singleSection || expanded.has(key);

  const scrollToSection = (key: string) => {
    const el = sectionEls.current.get(key);
    const box = listRef.current;
    if (el && box) box.scrollTo({ top: el.offsetTop, behavior: 'smooth' });
  };

  const toggleCollapse = (key: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });

  const sectionState = (s: Section) => {
    const sel = s.groups.filter((g) => selected.has(g.id)).length;
    const all = sel > 0 && sel === s.groups.length;
    const some = sel > 0 && !all;
    return { sel, all, some };
  };

  const onSectionToggle = (s: Section) => {
    const { all } = sectionState(s);
    onToggleGroups(
      s.groups.map((g) => g.id),
      !all
    );
  };

  const chipBox = (all: boolean, some: boolean, size: number) => (
    <span
      style={{
        width: size,
        height: size,
        flex: 'none',
        borderRadius: size >= 19 ? 6 : 5,
        border: `1.5px solid ${all || some ? T.wine : T.checkOff}`,
        background: all ? T.wine : T.white,
        color: T.white,
        fontSize: size >= 19 ? 12 : 10,
        fontWeight: 700,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        lineHeight: 1,
      }}
    >
      {all ? '✓' : some ? '—' : ''}
    </span>
  );

  return (
    <div style={{ border: `1px solid ${T.border}`, borderRadius: 16, overflow: 'hidden' }}>
      {/* Toolbar */}
      <div
        className="flex flex-wrap items-center gap-2.5 p-3"
        style={{ borderBottom: `1px solid ${T.divider}`, background: T.toolbar }}
      >
        <div
          className="hf-search flex min-w-[180px] flex-1 items-center gap-2"
          style={{
            background: T.white,
            border: `1px solid ${T.border}`,
            borderRadius: 10,
            padding: '8px 12px',
          }}
        >
          <svg
            width="15"
            height="15"
            viewBox="0 0 15 15"
            style={{ flex: 'none', opacity: 0.45 }}
            aria-hidden="true"
          >
            <circle cx="6.5" cy="6.5" r="4.75" fill="none" stroke={T.ink} strokeWidth="1.5" />
            <line
              x1="10.2"
              y1="10.2"
              x2="13.5"
              y2="13.5"
              stroke={T.ink}
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
          <input
            value={searchTerm}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder={t('pages.jobs.schedule.searchGroups')}
            className="w-full bg-transparent outline-none"
            style={{ border: 'none', fontSize: 13.5, color: T.ink }}
          />
        </div>
        <button
          type="button"
          onClick={() => onSelectAll(visibleIds)}
          className="hf-btn-wine cursor-pointer whitespace-nowrap"
          style={{
            fontSize: 12.5,
            fontWeight: 600,
            color: T.wine,
            background: T.white,
            border: `1px solid ${T.wineBtnBorder}`,
            borderRadius: 10,
            padding: '8px 13px',
          }}
        >
          ✓ {t('pages.jobs.schedule.selectAll')}
        </button>
        <button
          type="button"
          onClick={onClearAll}
          className="hf-btn-neutral cursor-pointer whitespace-nowrap"
          style={{
            fontSize: 12.5,
            fontWeight: 600,
            color: T.ink,
            opacity: 0.75,
            background: T.white,
            border: `1px solid ${T.border}`,
            borderRadius: 10,
            padding: '8px 13px',
          }}
        >
          ✕ {t('pages.jobs.schedule.clear')}
        </button>
        <button
          type="button"
          onClick={() => setOnlyFav((v) => !v)}
          className="cursor-pointer whitespace-nowrap"
          style={{
            fontSize: 12.5,
            fontWeight: 600,
            color: onlyFav ? T.white : T.wine,
            background: onlyFav ? T.wine : T.white,
            border: `1px solid ${onlyFav ? T.wine : T.wineBtnBorder}`,
            borderRadius: 10,
            padding: '8px 13px',
          }}
        >
          ★ {t('pages.jobs.schedule.favorites')}
        </button>
      </div>

      {/* Atalhos */}
      {sections.length > 0 && (
        <div
          className="flex flex-wrap items-center gap-2 px-3.5 py-2.5"
          style={{ borderBottom: `1px solid ${T.divider}`, background: T.white }}
        >
          <span
            className="uppercase"
            style={{
              fontSize: 11,
              fontWeight: 700,
              letterSpacing: '0.06em',
              color: T.ink,
              opacity: 0.45,
              marginRight: 2,
            }}
          >
            {t('pages.jobs.schedule.shortcuts')}
          </span>
          {sections.map((s) => {
            const { sel, all, some } = sectionState(s);
            return (
              <button
                type="button"
                key={s.key}
                onClick={() => {
                  onSectionToggle(s);
                  setExpanded((prev) => new Set(prev).add(s.key));
                  setTimeout(() => scrollToSection(s.key), 60);
                }}
                className="hf-chip flex cursor-pointer select-none items-center gap-1.5"
                style={{
                  padding: '6px 11px',
                  borderRadius: 999,
                  border: `1.5px solid ${all || some ? T.borderSel : T.border}`,
                  background: all ? T.rose : T.white,
                  transition: 'background .12s,border-color .12s',
                }}
              >
                {chipBox(all, some, 15)}
                <span
                  style={{ fontSize: 12.5, fontWeight: 600, color: T.ink, whiteSpace: 'nowrap' }}
                >
                  {s.label}
                </span>
                <span
                  style={{ fontSize: 11, fontWeight: 700, color: sel > 0 ? T.wine : T.countZero }}
                >
                  {sel}/{s.groups.length}
                </span>
              </button>
            );
          })}
        </div>
      )}

      {/* Lista */}
      <div
        ref={listRef}
        className="relative overflow-y-auto"
        style={{ maxHeight: 300, scrollBehavior: 'smooth' }}
      >
        {loading ? (
          <div
            className="flex items-center justify-center py-9"
            style={{ color: T.ink, opacity: 0.5, fontSize: 13.5 }}
          >
            {t('common.loading')}
          </div>
        ) : sections.length === 0 ? (
          <div className="py-9 text-center" style={{ fontSize: 13.5, color: T.ink, opacity: 0.5 }}>
            {groups.length === 0
              ? t('pages.jobs.form.noGroups')
              : t('pages.jobs.schedule.noResults')}
          </div>
        ) : (
          sections.map((s) => {
            const { sel, all, some } = sectionState(s);
            const open = isOpen(s.key);
            return (
              <div
                key={s.key}
                ref={(el) => {
                  if (el) sectionEls.current.set(s.key, el);
                  else sectionEls.current.delete(s.key);
                }}
                style={{ borderBottom: `1px solid ${T.rowLine}` }}
              >
                <div
                  className="sticky top-0 z-[2] flex select-none items-center gap-2.5 px-4 py-2.5"
                  style={{ background: T.section }}
                >
                  <button
                    type="button"
                    onClick={() => onSectionToggle(s)}
                    className="cursor-pointer"
                    title={t('pages.jobs.schedule.selectAll')}
                    style={{ display: 'flex', flex: 'none' }}
                  >
                    {chipBox(all, some, 19)}
                  </button>
                  <button
                    type="button"
                    onClick={() => toggleCollapse(s.key)}
                    className="flex flex-1 cursor-pointer items-center gap-2.5 text-left"
                  >
                    <span
                      className="uppercase"
                      style={{
                        fontSize: 12,
                        fontWeight: 700,
                        letterSpacing: '0.06em',
                        color: T.wine,
                      }}
                    >
                      {s.label}
                    </span>
                    <span style={{ fontSize: 11.5, fontWeight: 600, color: T.ink, opacity: 0.45 }}>
                      {t('pages.jobs.schedule.selectedInSection', {
                        selected: sel,
                        total: s.groups.length,
                      })}
                    </span>
                    <span
                      style={{
                        marginLeft: 'auto',
                        fontSize: 12,
                        color: T.ink,
                        opacity: 0.4,
                        padding: '0 4px',
                      }}
                    >
                      {open ? '▾' : '▸'}
                    </span>
                  </button>
                </div>
                {open &&
                  s.groups.map((g) => {
                    const on = selected.has(g.id);
                    return (
                      <div
                        key={g.id}
                        className="hf-row flex items-center gap-3"
                        style={{
                          padding: '0 16px',
                          background: on ? T.roseSel : T.white,
                          borderLeft: `3px solid ${on ? T.wine : 'transparent'}`,
                          transition: 'background .1s',
                        }}
                      >
                        <button
                          type="button"
                          onClick={() => onToggleGroup(g.id)}
                          className="flex min-w-0 flex-1 cursor-pointer select-none items-center gap-3 text-left"
                          style={{ padding: '10px 0' }}
                        >
                          <span
                            style={{
                              width: 19,
                              height: 19,
                              flex: 'none',
                              borderRadius: 6,
                              border: `1.5px solid ${on ? T.wine : T.checkOff}`,
                              background: on ? T.wine : T.white,
                              color: T.white,
                              fontSize: 12,
                              fontWeight: 700,
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              lineHeight: 1,
                            }}
                          >
                            {on ? '✓' : ''}
                          </span>
                          {g.num && (
                            <span
                              style={{
                                fontSize: 12.5,
                                fontWeight: 700,
                                color: T.wine,
                                background: T.tagWine,
                                borderRadius: 6,
                                padding: '2px 7px',
                                flex: 'none',
                              }}
                            >
                              {g.num}
                            </span>
                          )}
                          <span
                            className="overflow-hidden text-ellipsis whitespace-nowrap"
                            style={{ fontSize: 14, fontWeight: 500, color: T.ink }}
                          >
                            {g.name}
                          </span>
                        </button>
                        <button
                          type="button"
                          onClick={() => onToggleFavorite?.(g.id, !g.is_favorite)}
                          title={t('pages.jobs.schedule.favorites')}
                          className="flex-none cursor-pointer"
                          style={{
                            fontSize: 16,
                            lineHeight: 1,
                            color: g.is_favorite ? T.wine : T.starOff,
                            padding: 4,
                          }}
                        >
                          {g.is_favorite ? '★' : '☆'}
                        </button>
                      </div>
                    );
                  })}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
