import { SupportMobileHeader } from '@/components/layout/support-mobile-header';
import { SupportSidebar } from '@/components/layout/support-sidebar';
import { SidebarProvider } from '@/components/ui/sidebar';
import { useAuth } from '@/contexts';
import { Headset } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * Painel de Suporte — area exclusiva da equipe de suporte (role 'support').
 * Possui shell proprio (nao usa o AppLayout de cliente): sem navegacao de
 * cliente, sem pollers de automacoes/instancias, sem gate de suspensao de conta.
 *
 * O shell espelha o do painel de cliente: sidebar no desktop e header com
 * dropdown abaixo de 900px, onde a sidebar nao renderiza.
 *
 * Scaffold inicial: o roteamento, a guarda e a role funcionam ponta a ponta.
 * As funcionalidades de atendimento entram nas proximas entregas.
 */
export default function Support() {
  const { t } = useTranslation();
  const { user } = useAuth();

  return (
    <SidebarProvider>
      <div className="flex min-h-screen w-full bg-muted/30">
        <SupportSidebar />
        <main className="flex-1 flex flex-col">
          <SupportMobileHeader />
          <div className="flex flex-1 items-center justify-center p-6">
            <div className="w-full max-w-md rounded-2xl border bg-background p-8 text-center shadow-sm">
              <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
                <Headset className="h-6 w-6" />
              </div>
              <h2 className="mb-2 text-lg font-semibold">{t('pages.support.welcome')}</h2>
              <p className="text-sm text-muted-foreground">{t('pages.support.placeholder')}</p>
              {user?.email ? (
                <p className="mt-4 text-xs text-muted-foreground">{user.email}</p>
              ) : null}
            </div>
          </div>
        </main>
      </div>
    </SidebarProvider>
  );
}
