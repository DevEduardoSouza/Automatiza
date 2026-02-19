import { useAuth } from '@/contexts';
import {
  clearHistory as clearHistoryRpc,
  listHistory,
} from '@/gen/history/v1/history-HistoryService_connectquery';
import {
  EventType,
  type HistoryEvent as ProtoHistoryEvent,
  type HistoryStats as ProtoHistoryStats,
} from '@/gen/history/v1/history_pb';
import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo } from 'react';

/**
 * Local event type for UI (snake_case).
 */
type LocalEventType =
  | 'unspecified'
  | 'message_sent'
  | 'send_message'
  | 'automation_run'
  | 'coupon_used'
  | 'link_clicked'
  | 'group_sync'
  | 'error'
  | 'close_groups'
  | 'open_groups'
  | 'change_name'
  | 'change_photo'
  | 'change_description'
  | 'import_groups'
  | 'send_campaign'
  | 'generate_image'
  | 'add_member'
  | 'remove_member'
  | 'promote_member'
  | 'demote_member'
  | 'leave_group';

/**
 * Local history stats interface.
 */
interface LocalHistoryStats {
  total: number;
  completed: number;
  running: number;
  failed: number;
}

/**
 * Local history event interface matching UI expectations (snake_case).
 */
interface LocalHistoryEvent {
  id: string;
  user_id: string;
  event_type: LocalEventType;
  description: string;
  metadata: Record<string, unknown>;
  created_at: string;
  status?: string;
  error_message?: string;
  completed_at?: string;
  target_count?: number;
  success_count?: number;
  error_count?: number;
}

/**
 * Maps proto EventType string (from JSON) to local string.
 * Used when the enum is serialized as a string in the JSON response.
 */
function mapStringEventTypeToLocal(str: string): LocalEventType {
  const map: Record<string, LocalEventType> = {
    EVENT_TYPE_UNSPECIFIED: 'unspecified',
    EVENT_TYPE_MESSAGE_SENT: 'message_sent',
    EVENT_TYPE_AUTOMATION_RUN: 'automation_run',
    EVENT_TYPE_COUPON_USED: 'coupon_used',
    EVENT_TYPE_LINK_CLICKED: 'link_clicked',
    EVENT_TYPE_GROUP_SYNC: 'group_sync',
    EVENT_TYPE_ERROR: 'error',
    EVENT_TYPE_CLOSE_GROUPS: 'close_groups',
    EVENT_TYPE_OPEN_GROUPS: 'open_groups',
    EVENT_TYPE_CHANGE_NAME: 'change_name',
    EVENT_TYPE_CHANGE_PHOTO: 'change_photo',
    EVENT_TYPE_CHANGE_DESCRIPTION: 'change_description',
    EVENT_TYPE_IMPORT_GROUPS: 'import_groups',
    EVENT_TYPE_SEND_CAMPAIGN: 'send_campaign',
    EVENT_TYPE_GENERATE_IMAGE: 'generate_image',
    EVENT_TYPE_ADD_MEMBER: 'add_member',
    EVENT_TYPE_REMOVE_MEMBER: 'remove_member',
    EVENT_TYPE_PROMOTE_MEMBER: 'promote_member',
    EVENT_TYPE_DEMOTE_MEMBER: 'demote_member',
    EVENT_TYPE_LEAVE_GROUP: 'leave_group',
  };
  return map[str] ?? 'unspecified';
}

/**
 * Maps proto EventType to local string.
 */
function fromProtoEventType(type: EventType): LocalEventType {
  switch (type) {
    case EventType.MESSAGE_SENT:
      return 'message_sent';
    case EventType.AUTOMATION_RUN:
      return 'automation_run';
    case EventType.COUPON_USED:
      return 'coupon_used';
    case EventType.LINK_CLICKED:
      return 'link_clicked';
    case EventType.GROUP_SYNC:
      return 'group_sync';
    case EventType.ERROR:
      return 'error';
    case EventType.CLOSE_GROUPS:
      return 'close_groups';
    case EventType.OPEN_GROUPS:
      return 'open_groups';
    case EventType.CHANGE_NAME:
      return 'change_name';
    case EventType.CHANGE_PHOTO:
      return 'change_photo';
    case EventType.CHANGE_DESCRIPTION:
      return 'change_description';
    case EventType.IMPORT_GROUPS:
      return 'import_groups';
    case EventType.SEND_CAMPAIGN:
      return 'send_campaign';
    case EventType.GENERATE_IMAGE:
      return 'generate_image';
    case EventType.ADD_MEMBER:
      return 'add_member';
    case EventType.REMOVE_MEMBER:
      return 'remove_member';
    case EventType.PROMOTE_MEMBER:
      return 'promote_member';
    case EventType.DEMOTE_MEMBER:
      return 'demote_member';
    case EventType.LEAVE_GROUP:
      return 'leave_group';
    default:
      return 'unspecified';
  }
}

/**
 * Maps local event type to proto EventType.
 */
function toProtoEventType(type: LocalEventType | undefined): EventType | undefined {
  if (!type || type === 'unspecified') {
    return undefined;
  }

  switch (type) {
    case 'message_sent':
    case 'send_message':
      return EventType.MESSAGE_SENT;
    case 'automation_run':
      return EventType.AUTOMATION_RUN;
    case 'coupon_used':
      return EventType.COUPON_USED;
    case 'link_clicked':
      return EventType.LINK_CLICKED;
    case 'group_sync':
      return EventType.GROUP_SYNC;
    case 'error':
      return EventType.ERROR;
    case 'close_groups':
      return EventType.CLOSE_GROUPS;
    case 'open_groups':
      return EventType.OPEN_GROUPS;
    case 'change_name':
      return EventType.CHANGE_NAME;
    case 'change_photo':
      return EventType.CHANGE_PHOTO;
    case 'change_description':
      return EventType.CHANGE_DESCRIPTION;
    case 'import_groups':
      return EventType.IMPORT_GROUPS;
    case 'send_campaign':
      return EventType.SEND_CAMPAIGN;
    case 'generate_image':
      return EventType.GENERATE_IMAGE;
    case 'add_member':
      return EventType.ADD_MEMBER;
    case 'remove_member':
      return EventType.REMOVE_MEMBER;
    case 'promote_member':
      return EventType.PROMOTE_MEMBER;
    case 'demote_member':
      return EventType.DEMOTE_MEMBER;
    case 'leave_group':
      return EventType.LEAVE_GROUP;
    default:
      return undefined;
  }
}

/**
 * Converts proto HistoryEvent to local format.
 * Handles both numeric enum values and string representations from JSON.
 */
function fromProtoHistoryEvent(event: ProtoHistoryEvent): LocalHistoryEvent {
  // Handle eventType as either string (from JSON) or numeric enum
  let eventType: LocalEventType;
  const rawEventType = event.eventType as unknown;

  if (typeof rawEventType === 'string') {
    // JSON response returns enum as string like "EVENT_TYPE_CHANGE_NAME"
    eventType = mapStringEventTypeToLocal(rawEventType);
  } else {
    // Numeric enum value from protobuf-es
    eventType = fromProtoEventType(event.eventType);
  }

  return {
    id: event.id,
    user_id: event.userId,
    event_type: eventType,
    description: event.description,
    metadata: (event.metadata as Record<string, unknown>) ?? {},
    created_at: event.createdAt,
    status: event.status,
    error_message: event.errorMessage,
    completed_at: event.completedAt,
    target_count: event.targetCount,
    success_count: event.successCount,
    error_count: event.errorCount,
  };
}

/**
 * Converts proto HistoryStats to local format.
 */
function fromProtoHistoryStats(stats?: ProtoHistoryStats): LocalHistoryStats {
  if (!stats) {
    return { total: 0, completed: 0, running: 0, failed: 0 };
  }
  return {
    total: stats.total,
    completed: stats.completed,
    running: stats.running,
    failed: stats.failed,
  };
}

interface UseHistoryConnectOptions {
  limit?: number;
  offset?: number;
  eventType?: LocalEventType;
  status?: string;
  startTime?: string;
  endTime?: string;
  page?: number;
  pageSize?: number;
}

interface ClearHistoryOptions {
  beforeDate?: string;
  eventTypes?: LocalEventType[];
}

interface UseHistoryConnectReturn {
  events: LocalHistoryEvent[];
  hasMore: boolean;
  loading: boolean;
  stats: LocalHistoryStats;
  clearHistory: (options?: ClearHistoryOptions) => Promise<number>;
  clearingHistory: boolean;
  refetch: () => void;
  // Pagination fields
  totalCount: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

/**
 * Hook for fetching event history via ConnectRPC.
 *
 * @param options Query options for filtering history
 */
export function useHistoryConnect(options?: UseHistoryConnectOptions): UseHistoryConnectReturn {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const transport = useTransport();

  // Build query input
  const queryInput = useMemo(
    () => ({
      limit: options?.limit ?? 50,
      offset: options?.offset ?? 0,
      eventType: toProtoEventType(options?.eventType),
      status: options?.status,
      startTime: options?.startTime,
      endTime: options?.endTime,
      page: options?.page,
      pageSize: options?.pageSize,
    }),
    [
      options?.limit,
      options?.offset,
      options?.eventType,
      options?.status,
      options?.startTime,
      options?.endTime,
      options?.page,
      options?.pageSize,
    ]
  );

  // Query for fetching history using ConnectRPC Query
  const {
    data: historyResponse,
    isLoading: loading,
    refetch,
  } = useQuery(listHistory, queryInput, {
    enabled: !!user,
  });

  // Transform proto events to local format
  const events: LocalHistoryEvent[] = useMemo(
    () => (historyResponse?.events ?? []).map(fromProtoHistoryEvent),
    [historyResponse]
  );

  // Transform proto stats to local format
  const stats: LocalHistoryStats = useMemo(
    () => fromProtoHistoryStats(historyResponse?.stats),
    [historyResponse]
  );

  // Clear history mutation
  const clearHistoryMutation = useMutation(clearHistoryRpc, {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['history'] });
      refetch();
    },
  });

  const clearHistory = useCallback(
    async (clearOptions?: ClearHistoryOptions): Promise<number> => {
      const result = await clearHistoryMutation.mutateAsync({
        beforeDate: clearOptions?.beforeDate,
        eventTypes:
          clearOptions?.eventTypes
            ?.map(toProtoEventType)
            .filter((t): t is EventType => t !== undefined) ?? [],
      });
      return result.deletedCount;
    },
    [clearHistoryMutation]
  );

  return {
    events,
    hasMore: historyResponse?.hasMore ?? false,
    loading,
    stats,
    clearHistory,
    clearingHistory: clearHistoryMutation.isPending,
    refetch,
    // Pagination fields
    totalCount: historyResponse?.totalCount ?? 0,
    page: historyResponse?.page ?? 1,
    pageSize: historyResponse?.pageSize ?? 20,
    totalPages: historyResponse?.totalPages ?? 1,
  };
}
