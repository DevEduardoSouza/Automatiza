/**
 * ConnectRPC utilities and error handling.
 * Public API for connect-related functionality.
 */

export {
  extractRemainingMinutes,
  handleConnectError,
  isAutomationLockError,
  isConnectError,
  isLockConflictError,
  type ErrorHandlerResult,
} from './error-handler';
