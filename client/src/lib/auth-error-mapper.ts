/**
 * Generic authentication error interface.
 * Compatible with both ConnectRPC errors and legacy error formats.
 */
interface AuthError {
  code?: string;
  message: string;
}

/**
 * Mapeamento de codigos de erro de autenticacao para chaves i18n.
 * Adaptado para funcionar com erros genericos (ConnectRPC, etc.)
 */
const AUTH_ERROR_MAP: Record<string, string> = {
  // Erros de credenciais
  invalid_credentials: 'common.errors.auth.invalidCredentials',
  invalid_grant: 'common.errors.auth.invalidCredentials',
  unauthenticated: 'common.errors.auth.invalidCredentials',

  // Erros de email
  email_not_confirmed: 'common.errors.auth.emailNotConfirmed',
  email_exists: 'common.errors.auth.emailExists',
  user_already_exists: 'common.errors.auth.emailExists',
  already_exists: 'common.errors.auth.emailExists',

  // Erros de usuario
  user_not_found: 'common.errors.auth.userNotFound',
  user_banned: 'common.errors.auth.userBanned',
  user_disabled: 'common.errors.auth.userDisabled',

  // Erros de senha
  weak_password: 'common.errors.auth.weakPassword',
  same_password: 'common.errors.auth.samePassword',

  // Erros de sessao/token
  session_expired: 'common.errors.auth.sessionExpired',
  refresh_token_not_found: 'common.errors.auth.sessionExpired',
  invalid_refresh_token: 'common.errors.auth.sessionExpired',

  // Erros de rate limit
  over_request_rate_limit: 'common.errors.auth.rateLimitExceeded',
  over_email_send_rate_limit: 'common.errors.auth.emailRateLimitExceeded',
  resource_exhausted: 'common.errors.auth.rateLimitExceeded',

  // Erros de OTP
  otp_expired: 'common.errors.auth.otpExpired',
  otp_disabled: 'common.errors.auth.otpDisabled',

  // Erros de OAuth/SSO
  provider_disabled: 'common.errors.auth.providerDisabled',
  sso_domain_not_found: 'common.errors.auth.ssoDomainNotFound',

  // Erros genericos
  validation_failed: 'common.errors.auth.validationFailed',
  invalid_argument: 'common.errors.auth.validationFailed',
  bad_request: 'common.errors.auth.badRequest',
  unauthorized: 'common.errors.auth.unauthorized',
  permission_denied: 'common.errors.auth.forbidden',
  forbidden: 'common.errors.auth.forbidden',
  not_found: 'common.errors.auth.notFound',
  conflict: 'common.errors.auth.conflict',
  internal_server_error: 'common.errors.auth.serverError',
  internal: 'common.errors.auth.serverError',
  service_unavailable: 'common.errors.auth.serviceUnavailable',
  unavailable: 'common.errors.auth.serviceUnavailable',
  unknown: 'common.errors.auth.unknown',
};

/**
 * Retorna a chave i18n para um erro de autenticacao.
 * Funciona com erros genericos (ConnectRPC, etc.)
 * @param error - Erro retornado pela operacao de auth
 * @returns Chave i18n para traducao ou fallback generico
 */
export function getAuthErrorKey(error: AuthError | null): string {
  if (!error) {
    return 'common.errors.auth.unknown';
  }

  // Tenta encontrar pelo codigo do erro (ex: "invalid_credentials", "Unauthenticated")
  const errorCode = error.code?.toLowerCase().replace(/-/g, '_');
  if (errorCode && AUTH_ERROR_MAP[errorCode]) {
    return AUTH_ERROR_MAP[errorCode];
  }

  // Fallback: tenta identificar pelo conteudo da mensagem
  const message = error.message.toLowerCase();

  if (message.includes('invalid login credentials') || message.includes('invalid credentials')) {
    return 'common.errors.auth.invalidCredentials';
  }

  if (message.includes('email not confirmed')) {
    return 'common.errors.auth.emailNotConfirmed';
  }

  if (message.includes('user not found')) {
    return 'common.errors.auth.userNotFound';
  }

  if (message.includes('rate limit') || message.includes('too many requests')) {
    return 'common.errors.auth.rateLimitExceeded';
  }

  if (message.includes('network') || message.includes('fetch')) {
    return 'common.errors.auth.networkError';
  }

  // Fallback generico
  return 'common.errors.auth.unknown';
}
