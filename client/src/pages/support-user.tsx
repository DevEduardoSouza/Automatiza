import { UserDetail } from '@/components/support/user-detail';
import { SupportShell } from './support';

/**
 * Pagina de detalhe de um usuario no painel de suporte (/support/user/:userId).
 * Reusa o shell do painel para manter a sidebar e a navegacao mobile.
 */
export default function SupportUser() {
  return (
    <SupportShell>
      <UserDetail />
    </SupportShell>
  );
}
