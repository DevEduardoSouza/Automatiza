import { useToast } from '@/hooks/use-toast';
import { getAuthErrorKey } from '@/lib/auth-error-mapper';
import { transport } from '@/lib/connect-transport';
import { captureHookError } from '@/lib/sentry';
import { type ForgotPasswordFormData, forgotPasswordSchema } from '@/schemas';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { zodResolver } from '@hookform/resolvers/zod';
import { useCallback, useState } from 'react';
import { type UseFormReturn, useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';

interface UseForgotPasswordReturn {
  form: UseFormReturn<ForgotPasswordFormData>;
  emailSent: boolean;
  sentEmail: string;
  onSubmit: (data: ForgotPasswordFormData) => Promise<void>;
  handleResend: () => void;
}

/**
 * Hook for forgot password page logic.
 * Manages form state, validation, and password reset email request.
 * Uses ProfilesService.RequestPasswordReset (consistent token hashing).
 */
export function useForgotPassword(): UseForgotPasswordReturn {
  const { t } = useTranslation();
  const [emailSent, setEmailSent] = useState(false);
  const [sentEmail, setSentEmail] = useState('');
  const { toast } = useToast();

  const form = useForm<ForgotPasswordFormData>({
    resolver: zodResolver(forgotPasswordSchema(t)),
    defaultValues: {
      email: '',
    },
  });

  const onSubmit = useCallback(
    async (data: ForgotPasswordFormData) => {
      try {
        const { ProfilesService, RequestPasswordResetRequestSchema } = await import(
          '@/gen/profiles/v1/profiles_pb'
        );
        const client = createClient(ProfilesService, transport);
        await client.requestPasswordReset(
          create(RequestPasswordResetRequestSchema, { email: data.email })
        );

        setSentEmail(data.email);
        setEmailSent(true);
        toast({
          title: t('pages.forgotPassword.success.title'),
          description: t('pages.forgotPassword.success.message'),
        });
      } catch (error: unknown) {
        captureHookError(error, 'useForgotPassword', 'onSubmit');
        toast({
          variant: 'destructive',
          title: t('pages.forgotPassword.errors.send'),
          description: error instanceof Error ? error.message : t(getAuthErrorKey(error)),
        });
      }
    },
    [toast, t]
  );

  const handleResend = useCallback(() => {
    setEmailSent(false);
    form.reset();
  }, [form]);

  return {
    form,
    emailSent,
    sentEmail,
    onSubmit,
    handleResend,
  };
}
