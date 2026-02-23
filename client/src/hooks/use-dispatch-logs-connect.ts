import {
  type DispatchLog as ProtoDispatchLog,
  listDispatchLogs,
} from '@/gen/automations/v1/automations-AutomationsService_connectquery';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import { useQuery } from '@connectrpc/connect-query';
import { useCallback, useMemo, useState } from 'react';

// ============================================================
// Local Types
// ============================================================

/**
 * Dispatch log status values.
 * Matches the proto string status field values.
 */
export type DispatchLogStatus = 'sent' | 'failed' | 'skipped' | 'queued' | 'error';

/**
 * Local dispatch log type mapped from proto.
 * Includes all fields from the proto DispatchLog message.
 */
export interface LocalDispatchLog {
  id: string;
  automationId: string;
  groupId: string;
  groupName: string;
  canonicalKey: string;
  offerTitle: string;
  offerUrl?: string;
  offerPrice?: number;
  status: DispatchLogStatus;
  errorMessage?: string;
  createdAt: Date;
  /** Store name (e.g., 'shopee', 'amazon') */
  store?: string;
  /** Product category */
  productCategory?: string;
  /** Product price */
  productPrice?: number;
  /** Associated coupon ID */
  couponId?: string;
  /** Coupon code */
  couponCode?: string;
  /** Whether the coupon was applied */
  couponApplied?: boolean;
  /** Reason why the coupon was skipped */
  couponSkipReason?: string;
  /** Product URL */
  productUrl?: string;
  /** Product page URL */
  productPageUrl?: string;
  /** Affiliate link */
  affiliateLink?: string;
}

/**
 * Options for the useDispatchLogsConnect hook.
 */
export interface UseDispatchLogsConnectOptions {
  /** The automation ID to fetch logs for (optional - if empty, fetches all automations) */
  automationId?: string;
  /** Number of logs per page (default: 50, max: 500) */
  limit?: number;
  /** Offset for pagination (default: 0) */
  offset?: number;
  /** Whether the query is enabled (default: true) */
  enabled?: boolean;
  /** Filter by WhatsApp group ID */
  groupId?: string;
  /** Filter by dispatch status (sent, failed, skipped, queued, error) */
  status?: string;
  /** Filter by store name (e.g., 'shopee', 'amazon') */
  store?: string;
  /** Search filter for product URL (ILIKE match) */
  searchUrl?: string;
}

/**
 * Return type for useDispatchLogsConnect hook.
 */
export interface UseDispatchLogsConnectReturn {
  /** List of dispatch logs */
  logs: LocalDispatchLog[];
  /** Total count of logs for the automation */
  totalCount: number;
  /** Whether there are more logs to fetch */
  hasMore: boolean;
  /** Whether the query is loading */
  isLoading: boolean;
  /** Whether the query has an error */
  isError: boolean;
  /** The error object if any */
  error: Error | null;
  /** Refetch the logs */
  refetch: () => void;
}

/**
 * Return type for useDispatchLogsPaginated hook.
 */
export interface UseDispatchLogsPaginatedReturn
  extends Omit<UseDispatchLogsConnectReturn, 'refetch'> {
  /** Current page number (0-indexed) */
  page: number;
  /** Number of items per page */
  pageSize: number;
  /** Total number of pages */
  totalPages: number;
  /** Go to next page (no-op if on last page) */
  nextPage: () => void;
  /** Go to previous page (no-op if on first page) */
  prevPage: () => void;
  /** Go to a specific page */
  goToPage: (page: number) => void;
  /** Refetch the current page */
  refetch: () => void;
}

// ============================================================
// Converters
// ============================================================

/**
 * Converts a proto Timestamp to a JavaScript Date.
 */
export function fromProtoTimestamp(timestamp: Timestamp | undefined): Date {
  if (!timestamp) {
    return new Date();
  }
  // Timestamp has seconds (bigint) and nanos (number)
  const millis = Number(timestamp.seconds) * 1000 + Math.floor(timestamp.nanos / 1_000_000);
  return new Date(millis);
}

/**
 * Converts a proto dispatch log status to local status.
 * Handles various status strings from the backend.
 */
export function fromProtoStatus(status: string): DispatchLogStatus {
  const normalized = status.toLowerCase();
  switch (normalized) {
    case 'sent':
    case 'reconciled':
      return 'sent';
    case 'failed':
    case 'error':
      return 'error';
    case 'skipped':
    case 'blocked':
      return 'skipped';
    case 'queued':
    case 'pending':
      return 'queued';
    default:
      return 'error';
  }
}

/**
 * Converts a proto DispatchLog to a local DispatchLog.
 * Maps all fields including the new store, coupon, and URL fields.
 */
export function fromProtoDispatchLog(log: ProtoDispatchLog): LocalDispatchLog {
  return {
    id: log.id,
    automationId: log.automationId,
    groupId: log.groupId,
    groupName: log.groupName,
    canonicalKey: log.canonicalKey,
    offerTitle: log.offerTitle,
    offerUrl: log.offerUrl,
    offerPrice: log.offerPrice,
    status: fromProtoStatus(log.status),
    errorMessage: log.errorMessage,
    createdAt: fromProtoTimestamp(log.createdAt),
    // New fields from proto
    store: log.store,
    productCategory: log.productCategory,
    productPrice: log.productPrice,
    couponId: log.couponId,
    couponCode: log.couponCode,
    couponApplied: log.couponApplied,
    couponSkipReason: log.couponSkipReason,
    productUrl: log.productUrl,
    productPageUrl: log.productPageUrl,
    affiliateLink: log.affiliateLink,
  };
}

// ============================================================
// Hooks
// ============================================================

/**
 * Hook to fetch dispatch logs for an automation using ConnectRPC.
 *
 * Uses the ListDispatchLogs RPC endpoint with pagination and filter support.
 * Logs are cached for 30 seconds to reduce unnecessary requests.
 *
 * @example
 * ```tsx
 * const { logs, isLoading, error, totalCount, hasMore, refetch } = useDispatchLogsConnect({
 *   automationId: 'abc-123', // optional - empty fetches all automations
 *   limit: 50,
 *   offset: 0,
 *   store: 'shopee',
 *   status: 'sent',
 * });
 * ```
 */
export function useDispatchLogsConnect({
  automationId,
  limit = 50,
  offset = 0,
  enabled = true,
  groupId,
  status,
  store,
  searchUrl,
}: UseDispatchLogsConnectOptions): UseDispatchLogsConnectReturn {
  const query = useQuery(
    listDispatchLogs,
    {
      automationId: automationId || undefined, // Pass undefined if empty string
      limit,
      offset,
      groupId: groupId || undefined,
      status: status || undefined,
      store: store || undefined,
      searchUrl: searchUrl || undefined,
    },
    {
      enabled,
      staleTime: 30_000, // 30 seconds
    }
  );

  const logs = useMemo(() => {
    return query.data?.logs.map(fromProtoDispatchLog) ?? [];
  }, [query.data?.logs]);

  return {
    logs,
    totalCount: query.data?.totalCount ?? 0,
    hasMore: query.data?.hasMore ?? false,
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
  };
}

/**
 * Hook to fetch dispatch logs with pagination controls.
 *
 * Provides page-based navigation for dispatch logs,
 * useful for tables and lists with page controls.
 *
 * @example
 * ```tsx
 * const {
 *   logs,
 *   page,
 *   totalPages,
 *   nextPage,
 *   prevPage,
 *   goToPage,
 *   isLoading,
 * } = useDispatchLogsPaginated('abc-123', 20);
 *
 * return (
 *   <div>
 *     <LogList logs={logs} />
 *     <Pagination
 *       page={page}
 *       totalPages={totalPages}
 *       onNext={nextPage}
 *       onPrev={prevPage}
 *       onPageChange={goToPage}
 *     />
 *   </div>
 * );
 * ```
 */
export function useDispatchLogsPaginated(
  automationId: string,
  pageSize = 20
): UseDispatchLogsPaginatedReturn {
  const [page, setPage] = useState(0);

  const { logs, totalCount, hasMore, isLoading, isError, error, refetch } = useDispatchLogsConnect({
    automationId,
    limit: pageSize,
    offset: page * pageSize,
    enabled: !!automationId,
  });

  const totalPages = useMemo(() => {
    return Math.ceil(totalCount / pageSize);
  }, [totalCount, pageSize]);

  const nextPage = useCallback(() => {
    if (hasMore) {
      setPage((p) => p + 1);
    }
  }, [hasMore]);

  const prevPage = useCallback(() => {
    if (page > 0) {
      setPage((p) => p - 1);
    }
  }, [page]);

  const goToPage = useCallback(
    (newPage: number) => {
      if (newPage >= 0 && newPage < totalPages) {
        setPage(newPage);
      }
    },
    [totalPages]
  );

  return {
    logs,
    totalCount,
    hasMore,
    isLoading,
    isError,
    error,
    page,
    pageSize,
    totalPages,
    nextPage,
    prevPage,
    goToPage,
    refetch,
  };
}
