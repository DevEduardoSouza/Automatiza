import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  useSidebar,
} from '@/components/ui/sidebar';
import { ROUTES } from '@/constants/routes';
import { useAuth } from '@/contexts';
import { Activity, Headset, KanbanSquare, type LucideIcon, Wallet } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { NavLink } from 'react-router';
import { LogoutIcon, ToggleSidebarIcon } from './app-sidebar';

export interface SupportMenuItem {
  titleKey: string;
  url: string;
  icon: LucideIcon;
}

/**
 * Itens de navegacao do painel de suporte.
 * Compartilhado com o SupportMobileHeader para que as duas navegacoes
 * (desktop e mobile) nunca divirjam.
 */
export const supportMenuItems: SupportMenuItem[] = [
  { titleKey: 'pages.support.nav.home', url: ROUTES.SUPPORT, icon: Headset },
  {
    titleKey: 'pages.support.nav.dashboard',
    url: '/support/dashboard',
    icon: Activity,
  },
  {
    titleKey: 'pages.support.nav.financeiro',
    url: '/support/financeiro',
    icon: Wallet,
  },
  {
    titleKey: 'pages.support.nav.kanban',
    url: '/support/kanban',
    icon: KanbanSquare,
  },
];

/**
 * Sidebar do painel de suporte.
 * Espelha a AppSidebar do painel de cliente (mesmo shell, mesmas classes de
 * estilo e mesmo comportamento de colapso), mas com a navegacao propria do
 * suporte — sem os itens de cliente e sem os pollers do AppLayout.
 *
 * Assim como a AppSidebar, some abaixo de 900px: nessa faixa quem navega e o
 * SupportMobileHeader.
 */
export function SupportSidebar() {
  const { t } = useTranslation();
  const { state, isMobile, setOpenMobile, toggleSidebar } = useSidebar();
  const { signOut } = useAuth();
  const collapsed = state === 'collapsed';

  const handleLinkClick = () => {
    if (isMobile) {
      setOpenMobile(false);
    }
  };

  const getNavCls = ({ isActive }: { isActive: boolean }) =>
    `sidebar-nav-item ${isActive ? 'sidebar-nav-item-active' : ''}`;

  return (
    <Sidebar collapsible="icon">
      <SidebarContent>
        <div
          className={
            collapsed ? 'px-2 pt-3 mb-4 flex justify-center' : 'px-4 mb-4 flex justify-center'
          }
        >
          {collapsed ? (
            <button
              type="button"
              className="sidebar-nav-item !size-10 justify-center"
              onClick={toggleSidebar}
              aria-label={t('common.toggleSidebar')}
            >
              <ToggleSidebarIcon className="sidebar-nav-icon" />
            </button>
          ) : (
            <button type="button" onClick={toggleSidebar} className="cursor-pointer">
              <img
                src="/novalogo.png"
                alt="Automatisa Grupos"
                className="h-16 w-auto object-contain transition-all duration-300 hover:opacity-80"
              />
            </button>
          )}
        </div>

        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {supportMenuItems.map((item) => (
                <SidebarMenuItem key={item.titleKey}>
                  <NavLink to={item.url} end className={getNavCls} onClick={handleLinkClick}>
                    <item.icon className="sidebar-nav-icon" />
                    {!collapsed && <span className="sidebar-nav-text">{t(item.titleKey)}</span>}
                  </NavLink>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <div className={collapsed ? 'mt-auto p-2 space-y-2' : 'mt-auto p-4 space-y-2'}>
          <button
            type="button"
            className={`sidebar-nav-item w-full ${collapsed ? '!size-10 justify-center' : ''}`}
            onClick={() => signOut()}
          >
            <LogoutIcon className="sidebar-nav-icon" />
            {!collapsed && <span className="sidebar-nav-text">{t('sidebar.logout')}</span>}
          </button>
        </div>
      </SidebarContent>
    </Sidebar>
  );
}
