import { Financeiro } from '@/components/support/financeiro';
import { SupportShell } from './support';

/**
 * Aba Financeiro do painel de suporte (/support/financeiro): KPIs de SaaS,
 * receita estimada por plano, projeção de lucro e lançamentos manuais de
 * despesas/lucros. Reusa o shell do painel.
 */
export default function SupportFinanceiro() {
  return (
    <SupportShell>
      <Financeiro />
    </SupportShell>
  );
}
