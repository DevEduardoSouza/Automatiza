import { useToast } from '@/hooks/use-toast';
import { getAuthErrorKey } from '@/lib/auth-error-mapper';
import { transport } from '@/lib/connect-transport';
import { type ResetPasswordFormData, resetPasswordSchema } from '@/schemas';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { zodResolver } from '@hookform/resolvers/zod';
import { useCallback, useEffect, useState } from 'react';
import { type UseFormReturn, useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router';

interface UseResetPasswordReturn {
  form: UseFormReturn<ResetPasswordFormData>;
  tokenStatus: 'loading' | 'valid' | 'invalid' | 'expired' | 'used';
  maskedEmail: string | null;
  onSubmit: (data: ResetPasswordFormData) => Promise<void>;
}

/**
 * Hook for reset password page logic.
 * Extracts token from URL, validates it via backend, then resets password.
 * Uses ProfilesService for both validation and reset (consistent token hashing).
 */
export function useResetPassword(): UseResetPasswordReturn {
  const { t } = useTranslation();
  const [tokenStatus, setTokenStatus] = useState<'loading' | 'valid' | 'invalid' | 'expired' | 'used'>('loading');
  const [maskedEmail, setMaskedEmail] = useState<string | null>(null);
  const { toast } = useToast();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  const token = searchParams.get('token');

  const form = useForm<ResetPasswordFormData>({
    resolver: zodResolver(resetPasswordSchema(t)),
    defaultValues: {
      password: '',
      confirmPassword: '',
    },
  });

  useEffect(() => {
    if (!token) {
      setTokenStatus('invalid');
      return;
    }

    const validateToken = async () => {
      try {
        const { ProfilesService, ValidateResetTokenRequestSchema } = await import(
          '@/gen/profiles/v1/profiles_pb'
        );
        const client = createClient(ProfilesService, transport);
        const response = await client.validateResetToken(
          create(ValidateResetTokenRequestSchema, { token })
        );

        if (response.valid) {
          setTokenStatus('valid');
          setMaskedEmail(response.email ?? null);
        } else {
          const reason = response.reason ?? 'invalid_token';
          if (reason === 'token_expired') {
            setTokenStatus('expired');
          } else if (reason === 'token_used') {
            setTokenStatus('used');
          } else {
            setTokenStatus('invalid');
          }
        }
      } catch {
        setTokenStatus('invalid');
      }
    };

    validateToken();
  }, [token]);

  const onSubmit = useCallback(
    async (data: ResetPasswordFormData) => {
      if (!token) return;

      try {
        const { ProfilesService, ResetPasswordRequestSchema } = await import(
          '@/gen/profiles/v1/profiles_pb'
        );
        const client = createClient(ProfilesService, transport);
        await client.resetPassword(
          create(ResetPasswordRequestSchema, {
            token,
            newPassword: data.password,
          })
        );

        toast({
          title: t('pages.resetPassword.success.title'),
          description: t('pages.resetPassword.success.message'),
        });
        navigate('/auth');
      } catch (error) {
        toast({
          variant: 'destructive',
          title: t('pages.resetPassword.errors.update'),
          description: t(getAuthErrorKey(error)),
        });
      }
    },
    [token, toast, t, navigate]
  );

  return {
    form,
    tokenStatus,
    maskedEmail,
    onSubmit,
  };
}
