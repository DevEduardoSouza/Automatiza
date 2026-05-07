import { cn } from '@/lib/utils';
import { NavLink } from 'react-router';

/**
 * Tab strip shared between /coupons (Simples) and /coupons/rules
 * (Avançados). Each tab is a NavLink so clicking updates the URL —
 * the active styling follows what react-router considers active for
 * the given `to`. With the sidebar collapsed back to a single "Cupons"
 * entry, this header is the only way users switch between the two
 * coupon flows, so it lives at the top of both pages.
 */
const tabs = [
  { to: '/coupons', label: 'Cupons Simples', end: true },
  { to: '/coupons/rules', label: 'Cupons Avançados', end: false },
];

export function CouponsTabsHeader() {
  return (
    <div className="flex items-center gap-1 border-b">
      {tabs.map((tab) => (
        <NavLink
          key={tab.to}
          to={tab.to}
          end={tab.end}
          className={({ isActive }) =>
            cn(
              'relative inline-flex items-center px-4 py-2.5 text-sm font-medium transition-colors',
              'hover:text-foreground',
              isActive
                ? 'text-foreground after:absolute after:left-0 after:right-0 after:-bottom-px after:h-0.5 after:bg-[#8D2D53]'
                : 'text-muted-foreground',
            )
          }
        >
          {tab.label}
        </NavLink>
      ))}
    </div>
  );
}
