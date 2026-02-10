/**
 * Centralized error handler for ConnectRPC errors.
 * Maps gRPC error codes to user-friendly messages and handles specific scenarios
 * like automation lock conflicts (409/AlreadyExists).
 */

import { Code, ConnectError } from '@connectrpc/connect';
import type { TFunction } from 'i18next';

/**
 * Error handler result with categorized actions.
 */
export interface ErrorHandlerResult {
  code: Code;
  message: string;
  userMessage: string;
  action: 'inline' | 'toast' | 'redirect' | 'retry' | 'none';
  redirectTo?: string;
  retryable: boolean;
  /** Extracted data from error message, if applicable */
  metadata?: {
    remainingMinutes?: number;
    lockType?: 'automation' | 'resource';
  };
}

/**
 * Extracts remaining minutes from an error message.
 * Supports formats like:
 * - "Aguarde X minuto(s)"
 * - "X minute(s) remaining"
 * - "locked for X min"
 *
 * @param message - Error message to parse
 * @returns Extracted minutes or null if not found
 */
export function extractRemainingMinutes(message: string): number | null {
  // Portuguese patterns
  const ptMatch = message.match(/aguarde\s+(\d+)\s*minuto/i);
  if (ptMatch) {
    return Number.parseInt(ptMatch[1], 10);
  }

  // English patterns
  const enMatch = message.match(/(\d+)\s*minute/i);
  if (enMatch) {
    return Number.parseInt(enMatch[1], 10);
  }

  // Generic "X min" pattern
  const minMatch = message.match(/(\d+)\s*min/i);
  if (minMatch) {
    return Number.parseInt(minMatch[1], 10);
  }

  return null;
}

/**
 * Detects if the error is related to an automation lock.
 * Checks for specific keywords in the error message.
 *
 * @param message - Error message to analyze
 * @returns True if the error is about an automation lock
 */
export function isAutomationLockError(message: string): boolean {
  const lowerMessage = message.toLowerCase();
  return (
    lowerMessage.includes('automation') ||
    lowerMessage.includes('automação') ||
    lowerMessage.includes('em execução') ||
    lowerMessage.includes('running') ||
    lowerMessage.includes('locked') ||
    lowerMessage.includes('bloqueada')
  );
}

/**
 * Handles ConnectRPC errors with specific logic for different scenarios.
 * Provides context-aware messages for automation-specific errors.
 *
 * @param error - The error to handle
 * @param t - i18n translation function
 * @param context - Optional context to determine error handling strategy
 * @returns Structured error information with action guidance
 */
export function handleConnectError(
  error: unknown,
  t: TFunction,
  context?: {
    operation?: 'run' | 'diagnose' | 'create' | 'update' | 'delete' | 'list' | 'get';
  }
): ErrorHandlerResult {
  if (!(error instanceof ConnectError)) {
    // Handle non-ConnectRPC errors
    if (error instanceof Error) {
      if (error.name === 'AbortError') {
        return {
          code: Code.Canceled,
          message: error.message,
          userMessage: t('common.errors.network.canceled'),
          action: 'none',
          retryable: false,
        };
      }

      return {
        code: Code.Unavailable,
        message: error.message,
        userMessage: t('common.errors.network.connection'),
        action: 'toast',
        retryable: true,
      };
    }

    return {
      code: Code.Unknown,
      message: String(error),
      userMessage: t('common.errors.network.unknown'),
      action: 'toast',
      retryable: false,
    };
  }

  const { code, message } = error;

  switch (code) {
    case Code.AlreadyExists: {
      // Special handling for automation lock conflicts (409)
      const isAutomationLock = isAutomationLockError(message);
      const remainingMinutes = extractRemainingMinutes(message);

      if (isAutomationLock) {
        const userMessage = remainingMinutes
          ? t('common.errors.automations.lockConflict', { minutes: remainingMinutes })
          : t('common.errors.automations.lockConflictGeneric');

        return {
          code,
          message,
          userMessage,
          action: 'toast',
          retryable: false,
          metadata: {
            remainingMinutes: remainingMinutes ?? undefined,
            lockType: 'automation',
          },
        };
      }

      // Generic already exists error
      return {
        code,
        message,
        userMessage: t('common.errors.network.alreadyExists'),
        action: 'inline',
        retryable: false,
      };
    }

    case Code.InvalidArgument:
      return {
        code,
        message,
        userMessage: t('common.errors.network.invalidArgument'),
        action: 'inline',
        retryable: false,
      };

    case Code.Unauthenticated:
      return {
        code,
        message,
        userMessage: t('common.errors.network.unauthenticated'),
        action: 'redirect',
        redirectTo: '/auth',
        retryable: false,
      };

    case Code.PermissionDenied:
      return {
        code,
        message,
        userMessage: t('common.errors.network.permissionDenied'),
        action: 'toast',
        retryable: false,
      };

    case Code.NotFound:
      return {
        code,
        message,
        userMessage: t('common.errors.network.notFound'),
        action: 'toast',
        retryable: false,
      };

    case Code.ResourceExhausted: {
      // Check if it's a plan limit error
      const isPlanLimit =
        message.toLowerCase().includes('limit') || message.toLowerCase().includes('plano');

      return {
        code,
        message,
        userMessage: isPlanLimit
          ? t('common.errors.automations.planLimitReached')
          : t('common.errors.network.resourceExhausted'),
        action: 'toast',
        retryable: !isPlanLimit,
      };
    }

    case Code.FailedPrecondition:
      return {
        code,
        message,
        userMessage: t('common.errors.network.failedPrecondition'),
        action: 'toast',
        retryable: false,
      };

    case Code.Aborted:
      return {
        code,
        message,
        userMessage: t('common.errors.network.aborted'),
        action: 'toast',
        retryable: true,
      };

    case Code.OutOfRange:
      return {
        code,
        message,
        userMessage: t('common.errors.network.outOfRange'),
        action: 'inline',
        retryable: false,
      };

    case Code.Unimplemented:
      return {
        code,
        message,
        userMessage: t('common.errors.network.unimplemented'),
        action: 'toast',
        retryable: false,
      };

    case Code.Internal:
      return {
        code,
        message,
        userMessage: t('common.errors.network.internal'),
        action: 'toast',
        retryable: true,
      };

    case Code.Unavailable:
      return {
        code,
        message,
        userMessage: t('common.errors.network.unavailable'),
        action: 'retry',
        retryable: true,
      };

    case Code.DataLoss:
      return {
        code,
        message,
        userMessage: t('common.errors.network.dataLoss'),
        action: 'toast',
        retryable: false,
      };

    case Code.DeadlineExceeded:
      return {
        code,
        message,
        userMessage: t('common.errors.network.deadlineExceeded'),
        action: 'toast',
        retryable: true,
      };

    case Code.Canceled:
      return {
        code,
        message,
        userMessage: t('common.errors.network.canceled'),
        action: 'none',
        retryable: false,
      };

    default:
      return {
        code,
        message,
        userMessage: t('common.errors.network.unknown'),
        action: 'toast',
        retryable: false,
      };
  }
}

/**
 * Type guard to check if an error is a ConnectError.
 */
export function isConnectError(error: unknown): error is ConnectError {
  return error instanceof ConnectError;
}

/**
 * Checks if an error code indicates a lock conflict.
 */
export function isLockConflictError(error: unknown): boolean {
  if (!isConnectError(error)) {
    return false;
  }
  return error.code === Code.AlreadyExists && isAutomationLockError(error.message);
}
