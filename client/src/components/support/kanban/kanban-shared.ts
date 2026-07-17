/**
 * Definições compartilhadas do Kanban do painel de suporte.
 *
 * As 5 colunas são FIXAS e sua ordem espelha o fluxo de correção de bug. O
 * mesmo conjunto de chaves vale no backend (CHECK da migração 070 e o mapa
 * kanbanColumns do handler) — alterar aqui sem alterar lá quebra a validação.
 */
export type ColumnKey = 'corrigir' | 'aguardando' | 'em_andamento' | 'em_revisao' | 'corrigido';

export type Priority = 'low' | 'medium' | 'high';

export const PRIORITIES: Priority[] = ['low', 'medium', 'high'];

export interface ColumnMeta {
  key: ColumnKey;
  labelKey: string;
  /** Cor do "trilho" do topo da coluna (Tailwind). */
  accent: string;
}

export const COLUMN_META: ColumnMeta[] = [
  { key: 'corrigir', labelKey: 'pages.support.kanban.columns.corrigir', accent: 'bg-slate-400' },
  {
    key: 'aguardando',
    labelKey: 'pages.support.kanban.columns.aguardando',
    accent: 'bg-amber-400',
  },
  {
    key: 'em_andamento',
    labelKey: 'pages.support.kanban.columns.em_andamento',
    accent: 'bg-blue-500',
  },
  {
    key: 'em_revisao',
    labelKey: 'pages.support.kanban.columns.em_revisao',
    accent: 'bg-purple-500',
  },
  {
    key: 'corrigido',
    labelKey: 'pages.support.kanban.columns.corrigido',
    accent: 'bg-emerald-500',
  },
];

/** Cores da tarja de prioridade no card. */
export const PRIORITY_DOT: Record<Priority, string> = {
  low: 'bg-slate-400',
  medium: 'bg-amber-500',
  high: 'bg-red-500',
};

/** Espaçamento entre posições ao inserir no fim de uma coluna. */
export const POSITION_STEP = 1;

/** Formata bytes em unidade legível (KB/MB). */
export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
