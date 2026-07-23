import { getGroupMemberMetrics } from '@/gen/groups/v1/groups-GroupsService_connectquery';
import { MemberDataStatus } from '@/gen/groups/v1/groups_pb';
import { useQuery } from '@connectrpc/connect-query';
import { useMemo } from 'react';

/**
 * Um dia da série, já normalizado para o gráfico.
 *
 * `members` é `null` — nunca 0 — quando nenhum grupo do recorte teve
 * observação naquele dia. Recharts pula pontos nulos em vez de desenhar uma
 * queda a zero, que seria uma mentira visual.
 */
export interface MemberSeriesPoint {
  date: string;
  /** Rótulo curto (DD/MM) para o eixo. */
  label: string;
  members: number | null;
  joins: number;
  leaves: number;
  /** Quantos grupos contribuíram com observação — expõe salto de cobertura. */
  groupsObserved: number;
}

export interface MemberRankingRow {
  groupId: string;
  groupName: string;
  joins: number;
  leaves: number;
  net: number;
  members: number | null;
}

export interface MemberDddRow {
  ddd: string;
  members: number;
}

export interface CapacityRow {
  groupId: string;
  groupName: string;
  members: number;
  net: number;
}

export interface XRayRow {
  groupId: string;
  groupName: string;
  active: number;
  joins: number;
  leaves: number;
  net: number;
  topDdd: string;
  topDddMembers: number;
  clicks: number;
}

export interface GroupMemberMetrics {
  loading: boolean;
  error: Error | null;
  /** true enquanto a reconciliação não cobriu nenhum grupo. */
  collecting: boolean;
  groupsTotal: number;
  groupsWithData: number;
  /** null enquanto não há baseline: "0 membros" seria falso. */
  activeMembers: number | null;
  series: MemberSeriesPoint[];
  topGrowth: MemberRankingRow[];
  topLoss: MemberRankingRow[];
  ddd: MemberDddRow[];
  /** Série curta demais para uma linha ter significado (< 2 dias). */
  seriesTooShort: boolean;
  /** Conta PESSOAS, não participações. null enquanto não há cobertura. */
  uniqueLeads: number | null;
  /** Exclui quem entrou e saiu no mesmo dia. Substitui o CPL (sem custo no sistema). */
  qualifiedLeads: number | null;
  /** null — nunca 0 — enquanto não existir coorte com idade suficiente. */
  retentionPct: number | null;
  recurringPct: number | null;
  capacity: CapacityRow[];
  xray: XRayRow[];
}

/**
 * Lê o histórico de membros dos grupos do usuário.
 *
 * A coleta começa quando a reconciliação cobre cada grupo, então nos primeiros
 * dias `collecting` é true e a interface precisa dizer isso — em vez de
 * desenhar um gráfico vazio, que o usuário lê como bug.
 */
export function useGroupMemberMetrics(days = 30, groupId?: string): GroupMemberMetrics {
  const { data, isLoading, error } = useQuery(
    getGroupMemberMetrics,
    { days, groupId },
    {
      // O agregado só muda quando a reconciliação roda (1x/h) ou chega um
      // evento; refazer a query a cada foco de janela seria desperdício.
      staleTime: 60_000,
      refetchOnWindowFocus: false,
    }
  );

  return useMemo(() => {
    const series: MemberSeriesPoint[] = (data?.series ?? []).map((p) => {
      const [, month, day] = p.date.split('-');
      return {
        date: p.date,
        label: `${day}/${month}`,
        // members_count ausente no proto => não observado, não zero.
        members: p.membersCount ?? null,
        joins: p.joins,
        leaves: p.leaves,
        groupsObserved: p.groupsObserved,
      };
    });

    const toRows = (
      list: readonly {
        groupId: string;
        groupName: string;
        joins: number;
        leaves: number;
        net: number;
        membersCount?: number;
      }[]
    ): MemberRankingRow[] =>
      list.map((e) => ({
        groupId: e.groupId,
        groupName: e.groupName,
        joins: e.joins,
        leaves: e.leaves,
        net: e.net,
        members: e.membersCount ?? null,
      }));

    const observedDays = series.filter((p) => p.members !== null).length;

    return {
      loading: isLoading,
      error: (error as Error) ?? null,
      collecting: !data || data.status !== MemberDataStatus.OK || (data.groupsWithData ?? 0) === 0,
      groupsTotal: data?.groupsTotal ?? 0,
      groupsWithData: data?.groupsWithData ?? 0,
      activeMembers: data?.activeMembers ?? null,
      series,
      topGrowth: toRows(data?.topGrowth ?? []),
      topLoss: toRows(data?.topLoss ?? []),
      ddd: (data?.dddDistribution ?? []).map((d) => ({ ddd: d.ddd, members: d.members })),
      seriesTooShort: observedDays < 2,
      uniqueLeads: data?.uniqueLeads ?? null,
      qualifiedLeads: data?.qualifiedLeads ?? null,
      retentionPct: data?.retentionPct ?? null,
      recurringPct: data?.recurringPct ?? null,
      capacity: (data?.capacity ?? []).map((c) => ({
        groupId: c.groupId,
        groupName: c.groupName,
        members: c.members,
        net: c.net,
      })),
      xray: (data?.xray ?? []).map((x) => ({
        groupId: x.groupId,
        groupName: x.groupName,
        active: x.active,
        joins: x.joins,
        leaves: x.leaves,
        net: x.net,
        topDdd: x.topDdd,
        topDddMembers: x.topDddMembers,
        clicks: x.clicks,
      })),
    };
  }, [data, isLoading, error]);
}
