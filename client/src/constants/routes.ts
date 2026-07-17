/**
 * Constantes de rotas da aplicação.
 * Centraliza todas as rotas para evitar strings hardcoded.
 */
export const ROUTES = {
  /** Rotas públicas */
  AUTH: '/auth',
  FORGOT_PASSWORD: '/forgot-password',
  RESET_PASSWORD: '/reset-password',
  PUBLIC_REDIRECT: '/r',

  /** Rotas protegidas */
  DASHBOARD: '/dashboard',
  WHATSAPP: '/whatsapp',
  GROUPS: '/groups',
  REDIRECT: '/redirect',
  JOBS: '/jobs',
  OFFER_AUTOMATIONS: '/offer-automations',
  ML_LINK_LISTS: '/ml-link-lists',
  DISPATCH_TEXTS: '/dispatch-texts',
  AFFILIATE_PROGRAMS: '/affiliate-programs',
  CTAS: '/ctas',
  COUPONS: '/coupons',
  AUTOMATION_LOGS: '/automation-logs',
  HISTORY: '/history',
  PROFILE: '/profile',
  ADMIN: '/admin',
  SUPPORT: '/support',
} as const;

export type RouteKey = keyof typeof ROUTES;
export type RoutePath = (typeof ROUTES)[RouteKey];
