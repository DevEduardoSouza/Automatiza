import { Code, ConnectError, type Interceptor } from '@connectrpc/connect';
import * as Sentry from '@sentry/react';

/**
 * gRPC error codes that should be IGNORED (expected errors, not real failures).
 * InvalidArgument = validation errors (handled by form UI)
 * Unauthenticated = session expired (handled by redirect)
 * NotFound = resource not found (handled by UI)
 */
const IGNORED_CODES = new Set([
  Code.InvalidArgument,
  Code.Unauthenticated,
  Code.NotFound,
  Code.Aborted,
]);

/**
 * gRPC error codes that should be reported as WARNING level.
 * ResourceExhausted = rate limiting
 * PermissionDenied = access denied
 */
const WARNING_CODES = new Set([Code.ResourceExhausted, Code.PermissionDenied]);

/**
 * Sentry interceptor for ConnectRPC.
 *
 * Classifies errors by gRPC code and reports to Sentry with rich context:
 * - Ignore: InvalidArgument, Unauthenticated, NotFound, AbortError
 * - Warning: ResourceExhausted, PermissionDenied
 * - Error: Internal, Unknown, Unavailable, DataLoss, DeadlineExceeded, network errors
 *
 * Must be registered as the LAST interceptor in the chain
 * (after csrf -> tracing -> retry -> errorNormalization).
 */
export const sentryInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (error) {
    // AbortError: user cancelled — always ignore
    if (error instanceof Error && error.name === 'AbortError') {
      throw error;
    }

    const tags: Record<string, string> = {
      'rpc.method': req.method.name,
      'rpc.service': req.service.typeName,
    };

    // Extract trace_id from request headers
    const traceId = req.header.get('x-trace-id');
    if (traceId) {
      tags.trace_id = traceId;
    }

    if (error instanceof ConnectError) {
      tags['rpc.code'] = Code[error.code];

      // Ignored codes: don't send to Sentry
      if (IGNORED_CODES.has(error.code)) {
        throw error;
      }

      // Warning codes: send as message with warning level
      if (WARNING_CODES.has(error.code)) {
        Sentry.captureMessage(
          `ConnectRPC ${Code[error.code]}: ${req.service.typeName}/${req.method.name}`,
          {
            level: 'warning',
            tags,
            extra: { message: error.message },
          }
        );
        throw error;
      }

      // Error codes (Internal, Unknown, Unavailable, DataLoss, DeadlineExceeded, etc.)
      Sentry.captureException(error, { tags });
      throw error;
    }

    // Non-ConnectError (network errors, fetch failures, etc.)
    tags['rpc.code'] = 'NETWORK_ERROR';
    Sentry.captureException(error, { tags });
    throw error;
  }
};
