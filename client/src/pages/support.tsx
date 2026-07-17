import { SupportMobileHeader } from '@/components/layout/support-mobile-header';
import { SupportSidebar } from '@/components/layout/support-sidebar';
import { UserSearch } from '@/components/support/user-search';
import { SidebarProvider } from '@/components/ui/sidebar';

/**
 * Painel de Suporte — area exclusiva da equipe de suporte (role 'support').
 * Possui shell proprio (nao usa o AppLayout de cliente): sem navegacao de
 * cliente, sem pollers de automacoes/instancias, sem gate de suspensao de conta.
 *
 * A primeira tela e a busca de usuario: o atendimento sempre comeca por
 * "achar o cliente que reclamou".
 */
export default function Support() {
  return (
    <SupportShell>
      <UserSearch />
    </SupportShell>
  );
}

/**
 * Shell do painel: sidebar no desktop, header com dropdown abaixo de 900px
 * (onde a Sidebar nao renderiza). Compartilhado com a pagina de detalhe.
 */
export function SupportShell({ children }: { children: React.ReactNode }) {
  return (
    <SidebarProvider>
      <div className="flex min-h-screen w-full bg-muted/30">
        <SupportSidebar />
        <main className="flex-1 flex flex-col">
          <SupportMobileHeader />
          <div className="flex-1 p-4 sm:p-6 lg:p-8">{children}</div>
        </main>
      </div>
    </SidebarProvider>
  );
}
