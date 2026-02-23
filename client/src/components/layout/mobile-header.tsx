import { useAuth } from '@/contexts';
import { useIsMobile } from '@/hooks/use-mobile';
import { useSettingsConnect } from '@/hooks/use-settings-connect';
import { BookOpen } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { NavLink, useLocation } from 'react-router';
import { type MenuItem, menuItems } from './app-sidebar';

/**
 * Header fixo mobile com dropdown de navegacao.
 * Visivel apenas em viewports < md (768px).
 * Substitui o Sheet lateral da sidebar desktop.
 */
export function MobileHeader() {
  const isMobile = useIsMobile();
  const [isOpen, setIsOpen] = useState(false);
  const { t } = useTranslation();
  const location = useLocation();
  const { signOut, user } = useAuth();
  const prevPathnameRef = useRef(location.pathname);

  const isAdmin = user?.role === 'admin';

  const { getSetting } = useSettingsConnect(['support_whatsapp', 'tutorial_link']);
  const supportWhatsapp = getSetting<string>('support_whatsapp', '5511999999999');
  const tutorialLink = getSetting<string>('tutorial_link', '');

  const visibleMenuItems = menuItems.filter((item) => {
    if (item.url === '/admin') return isAdmin;
    return true;
  });

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
      {/* Fixed header bar */}
      <header className="mobile-header">
        <button type="button" onClick={() => setIsOpen(false)} className="shrink-0">
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

      {/* Backdrop overlay */}
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

      {/* Dropdown panel */}
      <div className={`mobile-header-dropdown ${isOpen ? 'mobile-header-dropdown-open' : ''}`}>
        <nav className="mobile-header-nav">
          {visibleMenuItems.map((item: MenuItem) => (
            <NavLink
              key={item.titleKey}
              to={item.url}
              end
              className={getNavCls}
              onClick={closeMenu}
            >
              <item.icon className="mobile-header-icon" aria-hidden="true" />
              <span className="mobile-header-text">
                {t(item.titleKey)}
                {item.comingSoon && (
                  <span className="ml-2 text-[9px] font-semibold uppercase tracking-wider bg-primary/10 text-primary px-1.5 py-0.5 rounded">
                    {t('common.comingSoon')}
                  </span>
                )}
              </span>
            </NavLink>
          ))}

          {/* Separator */}
          <div className="mobile-header-separator" />

          {/* Extra items: Support, Tutorials, Logout */}
          <button
            type="button"
            className="mobile-header-item"
            onClick={() => {
              window.open(`https://wa.me/${supportWhatsapp}`, '_blank');
              closeMenu();
            }}
          >
            <SupportIcon />
            <span className="mobile-header-text">{t('sidebar.support')}</span>
          </button>

          {tutorialLink && (
            <button
              type="button"
              className="mobile-header-item"
              onClick={() => {
                window.open(tutorialLink, '_blank');
                closeMenu();
              }}
            >
              <BookOpen className="mobile-header-icon" aria-hidden="true" />
              <span className="mobile-header-text">{t('sidebar.tutorials')}</span>
            </button>
          )}

          <button
            type="button"
            className="mobile-header-item"
            onClick={() => {
              signOut();
              closeMenu();
            }}
          >
            <LogoutIcon />
            <span className="mobile-header-text">{t('sidebar.logout')}</span>
          </button>
        </nav>
      </div>
    </>
  );
}

/** Menu icon: lines at bottom (closed state) */
function MenuClosedIcon({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      width="24"
      height="24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      className={className}
      aria-hidden="true"
    >
      <path d="M2 12C2 8.31087 2 6.4663 2.81382 5.15877C3.1149 4.67502 3.48891 4.25427 3.91891 3.91555C5.08116 3 6.72077 3 10 3H14C17.2792 3 18.9188 3 20.0811 3.91555C20.5111 4.25427 20.8851 4.67502 21.1862 5.15877C22 6.4663 22 8.31087 22 12C22 15.6891 22 17.5337 21.1862 18.8412C20.8851 19.325 20.5111 19.7457 20.0811 20.0845C18.9188 21 17.2792 21 14 21H10C6.72077 21 5.08116 21 3.91891 20.0845C3.48891 19.7457 3.1149 19.325 2.81382 18.8412C2 17.5337 2 15.6891 2 12Z" />
      <path d="M2 14.5L22 14.5" />
      <path d="M6 18H7M10 18H11" />
    </svg>
  );
}

/** Menu icon: lines at top (open state) */
function MenuOpenIcon({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      width="24"
      height="24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      className={className}
      aria-hidden="true"
    >
      <path d="M2 12C2 8.31088 2 6.46632 2.81382 5.15878C3.1149 4.67504 3.48891 4.25428 3.91891 3.91556C5.08116 3.00002 6.72077 3.00002 10 3.00002H14C17.2792 3.00002 18.9188 3.00002 20.0811 3.91556C20.5111 4.25428 20.8851 4.67504 21.1862 5.15878C22 6.46632 22 8.31088 22 12C22 15.6891 22 17.5337 21.1862 18.8412C20.8851 19.325 20.5111 19.7457 20.0811 20.0845C18.9188 21 17.2792 21 14 21H10C6.72077 21 5.08116 21 3.91891 20.0845C3.48891 19.7457 3.1149 19.325 2.81382 18.8412C2 17.5337 2 15.6891 2 12Z" />
      <path d="M2 9.50002L22 9.50002" />
      <path d="M6 6.00002H7M10 6.00002H11" />
    </svg>
  );
}

function SupportIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      width="24"
      height="24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="mobile-header-icon"
      aria-hidden="true"
    >
      <path d="M12.0045 11H12.0135M8.00903 11H8.018" />
      <path d="M22 13.2497C21.9163 15.6232 21.8353 16.8099 20.8699 17.7826C19.9046 18.7552 18.6843 18.8074 16.2437 18.9118C15.5098 18.9432 14.7498 18.9667 13.9693 18.9815C13.2282 18.9955 12.8576 19.0026 12.532 19.1266C12.2064 19.2506 11.9325 19.4855 11.3845 19.9553L9.20503 21.8242C9.07273 21.9376 8.90419 22 8.72991 22C8.32679 22 8 21.6732 8 21.2701V18.9219C7.91842 18.9186 7.83715 18.9153 7.75619 18.9118C5.31569 18.8074 4.09545 18.7552 3.13007 17.7825C2.16469 16.8099 2.12282 15.6232 2.03909 13.2497C2.01346 12.5232 2 11.7708 2 11C2 10.2292 2.01346 9.47679 2.03909 8.7503C2.12282 6.37683 2.16469 5.19009 3.13007 4.21745C4.09545 3.24481 5.3157 3.1926 7.7562 3.08819C9.09517 3.0309 10.5209 3 12 3C12.6684 3 13.3258 3.00631 13.9693 3.01851" />
      <path d="M19.491 11H19.5" />
      <path d="M17 4.5C17 3.11929 18.1193 2 19.5 2C20.8807 2 22 3.11929 22 4.5C22 5.53529 21.3707 6.4236 20.4737 6.80327C19.9651 7.01856 19.5 7.44772 19.5 8" />
    </svg>
  );
}

function LogoutIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      width="24"
      height="24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="mobile-header-icon"
      aria-hidden="true"
    >
      <path d="M8.00002 8C8.00002 7.42459 8.00002 7.17765 8.04465 6.92457C8.21993 5.93047 8.89355 5.09255 9.83302 4.70001C10.0723 4.60003 10.3559 4.53526 10.9232 4.40573L13.6508 3.78286C17.0405 3.00882 18.7353 2.6218 19.8677 3.51317C21 4.40454 21 6.1257 21 9.56803L21 14.432C21 17.8743 21 19.5955 19.8676 20.4868C18.7353 21.3782 17.0405 20.9912 13.6508 20.2171L10.9232 19.5943C10.3559 19.4647 10.0723 19.4 9.833 19.3C8.89353 18.9074 8.21991 18.0695 8.04462 17.0754C8 16.8224 8 16.5754 8 16" />
      <path d="M13 9C13 9 16 11.2095 16 12C16 12.7906 13 15 13 15M15.5 12H3" />
    </svg>
  );
}
