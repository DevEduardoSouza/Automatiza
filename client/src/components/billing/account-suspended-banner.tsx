import { Alert, AlertDescription } from '@/components/ui/alert';
import { useAuth } from '@/contexts';
import {
  GetProfileRequestSchema,
  type GetProfileResponse,
  type Profile,
  ProfilesService,
} from '@/gen/profiles/v1/profiles_pb';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

// Polling interval for checking account status (30 seconds)
const POLLING_INTERVAL = 30 * 1000;

interface LocalProfile {
  id: string;
  is_blocked: boolean;
}

function fromProtoProfile(profile: Profile): LocalProfile {
  return {
    id: profile.id,
    is_blocked: profile.isBlocked,
  };
}

/**
 * Banner de conta suspensa por cobranca.
 * Exibe alerta quando billing_status e "suspended".
 * Polls every 30 seconds to detect when account is unblocked.
 */
export function AccountSuspendedBanner() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const transport = useTransport();

  // Query with polling to detect account status changes
  const { data: profile } = useQuery({
    queryKey: ['profile', 'suspended-banner', user?.id],
    queryFn: async (): Promise<LocalProfile | null> => {
      if (!user?.id) return null;
      const client = createClient(ProfilesService, transport);
      const response: GetProfileResponse = await client.getProfile(
        create(GetProfileRequestSchema, {})
      );
      return response.profile ? fromProtoProfile(response.profile) : null;
    },
    enabled: !!user?.id,
    staleTime: POLLING_INTERVAL,
    refetchInterval: POLLING_INTERVAL,
  });

  // If no user or no profile, don't show banner
  if (!user || !profile) {
    return null;
  }

  // TODO: billing_status field needs to be added to the profile proto
  // For now, we check if the user is blocked as a fallback
  const isSuspended = profile.is_blocked;

  if (!isSuspended) {
    return null;
  }

  return (
    <div className="px-4 md:px-6 pt-4">
      <Alert variant="destructive">
        <AlertDescription>
          <div className="space-y-1">
            <div className="font-semibold">{t('billing.suspended.title')}</div>
            <div>{t('billing.suspended.description')}</div>
          </div>
        </AlertDescription>
      </Alert>
    </div>
  );
}
