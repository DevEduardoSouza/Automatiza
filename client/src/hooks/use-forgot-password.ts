import { useAuth } from '@/contexts';
import { useToast } from '@/hooks/use-toast';
import { getAuthErrorKey } from '@/lib/auth-error-mapper';
import { type ForgotPasswordFormData, forgotPasswordSchema } from '@/schemas';
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
 */
export function useForgotPassword(): UseForgotPasswordReturn {
  const { t } = useTranslation();
  const [emailSent, setEmailSent] = useState(false);
  const [sentEmail, setSentEmail] = useState('');
  const { resetPasswordForEmail } = useAuth();
  const { toast } = useToast();

  const form = useForm<ForgotPasswordFormData>({
    resolver: zodResolver(forgotPasswordSchema(t)),
    defaultValues: {
      email: '',
    },
  });

  const onSubmit = useCallback(
    async (data: ForgotPasswordFormData) => {
      const { error } = await resetPasswordForEmail(data.email);

      if (error) {
        toast({
          variant: 'destructive',
          title: t('pages.forgotPassword.errors.send'),
          description: t(getAuthErrorKey(error)),
        });
      } else {
        setSentEmail(data.email);
        setEmailSent(true);
        toast({
          title: t('pages.forgotPassword.success.title'),
          description: t('pages.forgotPassword.success.message'),
        });
      }
    },
    [resetPasswordForEmail, toast, t]
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
