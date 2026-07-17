import { SystemDashboard } from '@/components/support/system-dashboard';
import { SupportShell } from './support';

/**
 * Aba Dashboard do painel de suporte (/support/dashboard): saúde da VPS e da
 * aplicação em tempo nascente (polling). Reusa o shell do painel.
 */
export default function SupportDashboard() {
  return (
    <SupportShell>
      <SystemDashboard />
    </SupportShell>
  );
}
