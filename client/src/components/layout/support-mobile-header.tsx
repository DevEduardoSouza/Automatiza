import { useAuth } from '@/contexts';
import { useIsMobile } from '@/hooks/use-mobile';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { NavLink, useLocation } from 'react-router';
import { LogoutIcon } from './app-sidebar';
import { MenuClosedIcon, MenuOpenIcon } from './mobile-header';
import { type SupportMenuItem, supportMenuItems } from './support-sidebar';

/**
 * Header mobile do painel de suporte.
 * Necessario porque a Sidebar nao renderiza abaixo de 900px (ver ui/sidebar):
 * nessa faixa a navegacao e o logout vivem aqui.
 * Espelha o MobileHeader do painel de cliente, com a navegacao do suporte.
 */
export function SupportMobileHeader() {
  const isMobile = useIsMobile();
  const [isOpen, setIsOpen] = useState(false);
  const { t } = useTranslation();
  const location = useLocation();
  const { signOut } = useAuth();
  const prevPathnameRef = useRef(location.pathname);

  const closeMenu = useCallback(() => setIsOpen(false), []);

  // Close on route change (e.g., browser back/forward)
  useEffect(() => {
    if (prevPathnameRef.current !== location.pathname) {
      prevPathnameRef.current = location.pathname;
      setIsOpen(false);
    }
  }, [location.pathname]);

  // Lock body scroll when menu is open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [isOpen]);

  if (!isMobile) return null;

  const getNavCls = ({ isActive }: { isActive: boolean }) =>
    `mobile-header-item ${isActive ? 'mobile-header-item-active' : ''}`;

  return (
    <>
      <header className="mobile-header">
        <button type="button" onClick={closeMenu} className="shrink-0">
          <img src="/novalogo.png" alt="Automatisa" className="h-9 w-auto object-contain" />
        </button>

        <button
          type="button"
          className="mobile-header-hamburger"
          onClick={() => setIsOpen((prev) => !prev)}
          aria-label={t('sidebar.menu', 'Menu')}
          aria-expanded={isOpen}
        >
          <MenuClosedIcon
            className={`mobile-header-menu-icon ${isOpen ? 'mobile-header-menu-icon-hidden' : ''}`}
          />
          <MenuOpenIcon
            className={`mobile-header-menu-icon ${isOpen ? '' : 'mobile-header-menu-icon-hidden'}`}
          />
        </button>
      </header>

      {isOpen && (
        <div
          className="mobile-header-backdrop"
          onClick={closeMenu}
          onKeyDown={(e) => e.key === 'Escape' && closeMenu()}
          role="button"
          tabIndex={-1}
          aria-label={t('common.close', 'Fechar')}
        />
      )}

      <div className={`mobile-header-dropdown ${isOpen ? 'mobile-header-dropdown-open' : ''}`}>
        <nav className="mobile-header-nav">
          {supportMenuItems.map((item: SupportMenuItem) => (
            <NavLink
              key={item.titleKey}
              to={item.url}
              end
              className={getNavCls}
              onClick={closeMenu}
            >
              <item.icon className="mobile-header-icon" aria-hidden="true" />
              <span className="mobile-header-text">{t(item.titleKey)}</span>
            </NavLink>
          ))}

          <div className="mobile-header-separator" />

          <button
            type="button"
            className="mobile-header-item"
            onClick={() => {
              signOut();
              closeMenu();
            }}
          >
            <LogoutIcon className="mobile-header-icon" aria-hidden="true" />
            <span className="mobile-header-text">{t('sidebar.logout')}</span>
          </button>
        </nav>
      </div>
    </>
  );
}
