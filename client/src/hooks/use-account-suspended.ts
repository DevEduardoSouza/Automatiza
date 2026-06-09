import { useAuth } from '@/contexts';
import {
  GetProfileRequestSchema,
  type GetProfileResponse,
  ProfilesService,
} from '@/gen/profiles/v1/profiles_pb';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useQuery } from '@tanstack/react-query';

// Re-check account status every 30s so the UI recovers automatically once the
// user regularizes the payment.
const POLLING_INTERVAL = 30 * 1000;

/**
 * Whether the current account is suspended for billing. `getProfile` is an
 * always-allowed endpoint (see SuspensionInterceptor.isAlwaysAllowed), so it
 * keeps working while suspended — unlike the data endpoints that return
 * PermissionDenied. Used to gate page content so suspended accounts don't mount
 * pages whose hooks/pollers would hammer those blocked endpoints in a loop.
 */
export function useAccountSuspended(): { suspended: boolean; loading: boolean } {
  const { user } = useAuth();
  const transport = useTransport();

  const { data, isLoading } = useQuery({
    queryKey: ['profile', 'suspension-status', user?.id],
    queryFn: async (): Promise<boolean> => {
      if (!user?.id) return false;
      const client = createClient(ProfilesService, transport);
      const response: GetProfileResponse = await client.getProfile(
        create(GetProfileRequestSchema, {})
      );
      return Boolean(response.profile?.isBlocked);
    },
    enabled: !!user?.id,
    staleTime: POLLING_INTERVAL,
    refetchInterval: POLLING_INTERVAL,
  });

  return { suspended: data ?? false, loading: isLoading };
}
