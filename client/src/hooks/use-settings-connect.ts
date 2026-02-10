import { useAuth } from '@/contexts';
import { getSettings } from '@/gen/settings/v1/settings-SettingsService_connectquery';
import { SettingsService, UpdateSettingRequestSchema } from '@/gen/settings/v1/settings_pb';
import { useToast } from '@/hooks/use-toast';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { type Value, ValueSchema } from '@bufbuild/protobuf/wkt';
import { createClient } from '@connectrpc/connect';
import { createConnectQueryKey, useQuery, useTransport } from '@connectrpc/connect-query';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Converts a JavaScript value to proto Value.
 */
function toProtoValue(value: unknown): Value {
  if (value === null || value === undefined) {
    return create(ValueSchema, { kind: { case: 'nullValue', value: 0 } });
  }

  if (typeof value === 'boolean') {
    return create(ValueSchema, { kind: { case: 'boolValue', value } });
  }

  if (typeof value === 'number') {
    return create(ValueSchema, { kind: { case: 'numberValue', value } });
  }

  if (typeof value === 'string') {
    return create(ValueSchema, { kind: { case: 'stringValue', value } });
  }

  // For objects and arrays, serialize as string
  return create(ValueSchema, {
    kind: { case: 'stringValue', value: JSON.stringify(value) },
  });
}

/**
 * Converts proto Value to JavaScript value.
 */
function fromProtoValue(value: Value | undefined): unknown {
  if (!value || !value.kind) {
    return null;
  }

  switch (value.kind.case) {
    case 'nullValue':
      return null;
    case 'boolValue':
      return value.kind.value;
    case 'numberValue':
      return value.kind.value;
    case 'stringValue': {
      // Try to parse as JSON if it looks like an object/array
      const str = value.kind.value;
      if (str.startsWith('{') || str.startsWith('[')) {
        try {
          return JSON.parse(str);
        } catch {
          return str;
        }
      }
      return str;
    }
    default:
      return null;
  }
}

interface UseSettingsConnectReturn {
  settings: Record<string, unknown>;
  loading: boolean;
  getSetting: <T = unknown>(key: string, defaultValue?: T) => T;
  updateSetting: (key: string, value: unknown) => Promise<void>;
  isUpdating: boolean;
}

/**
 * Hook for managing user settings via ConnectRPC.
 *
 * @param keys Optional list of specific setting keys to fetch
 */
export function useSettingsConnect(keys?: string[]): UseSettingsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const transport = useTransport();

  // Query for fetching settings using ConnectRPC Query
  const { data: settingsResponse, isLoading: loading } = useQuery(
    getSettings,
    { keys: keys ?? [] },
    { enabled: !!user }
  );

  // Transform proto settings map to JavaScript record
  const settings: Record<string, unknown> = useMemo(() => {
    if (!settingsResponse?.settings) {
      return {};
    }

    const result: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(settingsResponse.settings)) {
      result[key] = fromProtoValue(value);
    }
    return result;
  }, [settingsResponse]);

  // Mutation for updating a setting
  const updateMutation = useMutation({
    mutationFn: async ({ key, value }: { key: string; value: unknown }) => {
      const client = createClient(SettingsService, transport);
      return client.updateSetting(
        create(UpdateSettingRequestSchema, {
          key,
          value: toProtoValue(value),
        })
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: getSettings, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({ queryKey: ['settings'] });
      toast({
        title: t('common.success'),
        description: t('pages.settings.success.updated'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  /**
   * Gets a specific setting value with optional default.
   */
  const getSetting = useCallback(
    <T = unknown>(key: string, defaultValue?: T): T => {
      const value = settings[key];
      if (value === undefined || value === null) {
        return defaultValue as T;
      }
      return value as T;
    },
    [settings]
  );

  /**
   * Updates a setting value.
   */
  const updateSettingFn = useCallback(
    async (key: string, value: unknown): Promise<void> => {
      await updateMutation.mutateAsync({ key, value });
    },
    [updateMutation]
  );

  return {
    settings,
    loading,
    getSetting,
    updateSetting: updateSettingFn,
    isUpdating: updateMutation.isPending,
  };
}
