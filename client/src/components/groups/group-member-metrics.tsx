import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import { useGroupMemberMetrics } from '@/hooks/use-group-member-metrics';
import type { MemberRankingRow } from '@/hooks/use-group-member-metrics';
import { Bar, BarChart, CartesianGrid, Legend, Line, LineChart, XAxis, YAxis } from 'recharts';

// Mesma paleta wine da página de grupos, para a seção não parecer enxertada.
const wine = {
  bg: 'rgba(141, 45, 83, 0.08)',
  border: 'rgba(141, 45, 83, 0.2)',
  text: '#8D2D53',
  textMuted: 'rgba(141, 45, 83, 0.6)',
};

// Hues da rampa categórica do design system (as mesmas do dashboard admin),
// atribuídas em ordem FIXA por entidade — nunca por posição no ranking, senão
// filtrar repintaria os sobreviventes.
const HUE_MEMBERS = 'hsl(var(--chart-1))';
const HUE_JOINS = 'hsl(var(--chart-2))';
const HUE_LEAVES = 'hsl(var(--chart-4))';

/** Caixa padrão da seção. */
function Panel({
  title,
  hint,
  children,
}: {
  title: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: wine.bg, borderColor: wine.border }}
    >
      <div className="mb-3 xs:mb-4">
        <span
          className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
          style={{ color: wine.text }}
        >
          {title}
        </span>
        {hint && (
          <p className="mt-1 text-[11px] xs:text-xs" style={{ color: wine.textMuted }}>
            {hint}
          </p>
        )}
      </div>
      {children}
    </div>
  );
}

/** Estado honesto de cobertura parcial: número, não spinner eterno. */
function CollectingNotice({ withData, total }: { withData: number; total: number }) {
  const pct = total > 0 ? Math.round((withData / total) * 1000) / 10 : 0;
  return (
    <div
      className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: wine.bg, borderColor: wine.border }}
    >
      <span
        className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
        style={{ color: wine.text }}
      >
        Coletando dados de membros
      </span>
      <p className="mt-2 text-xs xs:text-sm" style={{ color: wine.textMuted }}>
        O histórico começa a existir quando cada grupo é lido pela primeira vez. Já mapeamos{' '}
        <strong>{withData.toLocaleString('pt-BR')}</strong> de{' '}
        <strong>{total.toLocaleString('pt-BR')}</strong> grupos ({pct}%). Os gráficos aparecem
        conforme os dias passam — ainda não há histórico suficiente para uma comparação honesta.
      </p>
    </div>
  );
}

/** Lista de ranking. Barra proporcional ao maior valor absoluto da lista. */
function RankingList({
  rows,
  hue,
  emptyLabel,
}: {
  rows: MemberRankingRow[];
  hue: string;
  emptyLabel: string;
}) {
  if (rows.length === 0) {
    return (
      <p className="text-xs xs:text-sm" style={{ color: wine.textMuted }}>
        {emptyLabel}
      </p>
    );
  }
  const max = Math.max(...rows.map((r) => Math.abs(r.net)), 1);

  return (
    <ul className="space-y-2.5">
      {rows.map((r) => (
        <li key={r.groupId}>
          <div className="flex items-baseline justify-between gap-3">
            <span className="truncate text-xs xs:text-sm" title={r.groupName}>
              {r.groupName}
            </span>
            {/* O número fica em tinta de texto; a barra colorida ao lado é
                que carrega a identidade — cor nunca é o único canal. */}
            <span className="shrink-0 text-xs xs:text-sm font-semibold tabular-nums">
              {r.net > 0 ? '+' : ''}
              {r.net.toLocaleString('pt-BR')}
            </span>
          </div>
          <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-black/5">
            <div
              className="h-full rounded-full"
              style={{ width: `${(Math.abs(r.net) / max) * 100}%`, backgroundColor: hue }}
            />
          </div>
          <p className="mt-0.5 text-[11px]" style={{ color: wine.textMuted }}>
            {r.joins.toLocaleString('pt-BR')} entraram · {r.leaves.toLocaleString('pt-BR')} saíram
            {r.members !== null && ` · ${r.members.toLocaleString('pt-BR')} membros`}
          </p>
        </li>
      ))}
    </ul>
  );
}

/**
 * Seção de métricas históricas de membros dos grupos.
 *
 * Regra que atravessa o componente: ausência de dado NUNCA vira zero. Um dia
 * sem observação some da linha (null) em vez de despencar para o eixo, e a
 * seção inteira mostra o estado de coleta enquanto a cobertura for nula.
 */
export function GroupMemberMetrics() {
  const m = useGroupMemberMetrics(30);

  if (m.loading) {
    return (
      <div
        className="h-32 animate-pulse rounded-xl xs:rounded-2xl lg:rounded-3xl border"
        style={{ backgroundColor: wine.bg, borderColor: wine.border }}
      />
    );
  }

  // Erro não pode derrubar a página de grupos: a seção some, o resto vive.
  if (m.error) return null;

  if (m.collecting) {
    return <CollectingNotice withData={m.groupsWithData} total={m.groupsTotal} />;
  }

  const hasFlow = m.series.some((p) => p.joins > 0 || p.leaves > 0);

  return (
    <div className="space-y-3 xs:space-y-4">
      {/* Hero: o número mais importante, sem gráfico em volta. */}
      <div className="grid gap-3 xs:gap-4 grid-cols-1 sm:grid-cols-3">
        <Panel title="Membros ativos">
          <p className="text-2xl xs:text-3xl font-semibold tabular-nums">
            {m.activeMembers?.toLocaleString('pt-BR') ?? '—'}
          </p>
          <p className="mt-1 text-[11px] xs:text-xs" style={{ color: wine.textMuted }}>
            em {m.groupsWithData.toLocaleString('pt-BR')} de {m.groupsTotal.toLocaleString('pt-BR')}{' '}
            grupos já mapeados
          </p>
        </Panel>

        <div className="sm:col-span-2">
          <Panel
            title="Membros ao longo do tempo"
            hint={
              m.seriesTooShort
                ? 'A linha precisa de pelo menos dois dias de leitura. Volte amanhã.'
                : 'Total de membros observado por dia nos grupos já mapeados.'
            }
          >
            {m.seriesTooShort ? (
              <p className="py-6 text-center text-xs xs:text-sm" style={{ color: wine.textMuted }}>
                Ainda só há uma leitura.
              </p>
            ) : (
              <ChartContainer
                config={{ members: { label: 'Membros', color: HUE_MEMBERS } }}
                className="h-[180px] w-full"
              >
                <LineChart data={m.series} margin={{ top: 8, right: 8, bottom: 0, left: -12 }}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} opacity={0.35} />
                  <XAxis dataKey="label" tickLine={false} axisLine={false} fontSize={11} />
                  <YAxis tickLine={false} axisLine={false} fontSize={11} width={44} />
                  <ChartTooltip
                    content={
                      <ChartTooltipContent
                        // Sem isto, um salto de COBERTURA (mais grupos lidos)
                        // seria lido como crescimento real de membros.
                        formatter={(value, _name, item) => (
                          <span>
                            {Number(value).toLocaleString('pt-BR')} membros
                            <span className="ml-1 opacity-60">
                              ({item?.payload?.groupsObserved ?? 0} grupos lidos)
                            </span>
                          </span>
                        )}
                      />
                    }
                  />
                  <Line
                    type="monotone"
                    dataKey="members"
                    stroke={HUE_MEMBERS}
                    strokeWidth={2}
                    dot={{ r: 4 }}
                    activeDot={{ r: 6 }}
                    // Dia sem observação não vira zero: a linha só pula o ponto.
                    connectNulls={false}
                  />
                </LineChart>
              </ChartContainer>
            )}
          </Panel>
        </div>
      </div>

      <Panel
        title="Entradas e saídas por dia"
        hint="Mostra a rotatividade mesmo quando o total de membros fica parado."
      >
        {hasFlow ? (
          <ChartContainer
            config={{
              joins: { label: 'Entraram', color: HUE_JOINS },
              leaves: { label: 'Saíram', color: HUE_LEAVES },
            }}
            className="h-[200px] w-full"
          >
            <BarChart data={m.series} margin={{ top: 8, right: 8, bottom: 0, left: -12 }}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} opacity={0.35} />
              <XAxis dataKey="label" tickLine={false} axisLine={false} fontSize={11} />
              <YAxis
                tickLine={false}
                axisLine={false}
                fontSize={11}
                width={44}
                allowDecimals={false}
              />
              <ChartTooltip content={<ChartTooltipContent />} />
              {/* Duas séries => legenda obrigatória: identidade nunca só por cor. */}
              <Legend iconType="circle" iconSize={8} wrapperStyle={{ fontSize: 12 }} />
              <Bar
                dataKey="joins"
                name="Entraram"
                fill={HUE_JOINS}
                radius={[4, 4, 0, 0]}
                maxBarSize={22}
              />
              <Bar
                dataKey="leaves"
                name="Saíram"
                fill={HUE_LEAVES}
                radius={[4, 4, 0, 0]}
                maxBarSize={22}
              />
            </BarChart>
          </ChartContainer>
        ) : (
          <p className="py-6 text-center text-xs xs:text-sm" style={{ color: wine.textMuted }}>
            Nenhuma entrada ou saída registrada no período.
          </p>
        )}
      </Panel>

      <div className="grid gap-3 xs:gap-4 grid-cols-1 md:grid-cols-2">
        <Panel title="Grupos que mais cresceram" hint="Saldo de entradas menos saídas no período.">
          <RankingList
            rows={m.topGrowth.filter((r) => r.net > 0)}
            hue={HUE_JOINS}
            emptyLabel="Nenhum grupo cresceu no período."
          />
        </Panel>
        <Panel title="Grupos que mais perderam" hint="Saldo negativo no período.">
          <RankingList
            rows={m.topLoss.filter((r) => r.net < 0)}
            hue={HUE_LEAVES}
            emptyLabel="Nenhum grupo perdeu membros no período."
          />
        </Panel>
      </div>

      <Panel
        title="De onde são seus membros (DDD)"
        hint="Só entram membros com telefone identificado — o WhatsApp esconde o número de parte deles."
      >
        {m.ddd.length > 0 ? (
          <ChartContainer
            config={{ members: { label: 'Membros', color: HUE_MEMBERS } }}
            className="h-[240px] w-full"
          >
            <BarChart
              data={m.ddd}
              layout="vertical"
              margin={{ top: 4, right: 12, bottom: 0, left: 4 }}
            >
              <CartesianGrid strokeDasharray="3 3" horizontal={false} opacity={0.35} />
              <XAxis
                type="number"
                tickLine={false}
                axisLine={false}
                fontSize={11}
                allowDecimals={false}
              />
              <YAxis
                type="category"
                dataKey="ddd"
                tickLine={false}
                axisLine={false}
                fontSize={11}
                width={34}
              />
              <ChartTooltip content={<ChartTooltipContent />} />
              {/* Uma série só: sem legenda — o título já a nomeia. */}
              <Bar
                dataKey="members"
                name="Membros"
                fill={HUE_MEMBERS}
                radius={[0, 4, 4, 0]}
                maxBarSize={16}
              />
            </BarChart>
          </ChartContainer>
        ) : (
          <p className="py-6 text-center text-xs xs:text-sm" style={{ color: wine.textMuted }}>
            Nenhum membro com telefone identificado ainda.
          </p>
        )}
      </Panel>
    </div>
  );
}
