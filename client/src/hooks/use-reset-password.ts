import { useAuth } from '@/contexts';
import { useToast } from '@/hooks/use-toast';
import { getAuthErrorKey } from '@/lib/auth-error-mapper';
import { type ResetPasswordFormData, resetPasswordSchema } from '@/schemas';
import { zodResolver } from '@hookform/resolvers/zod';
import { useCallback, useEffect, useState } from 'react';
import { type UseFormReturn, useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';

interface UseResetPasswordReturn {
  form: UseFormReturn<ResetPasswordFormData>;
  validSession: boolean;
  checkingSession: boolean;
  onSubmit: (data: ResetPasswordFormData) => Promise<void>;
}

/**
 * Hook for reset password page logic.
 * Manages form state, validation, session checking, and password update.
 * Uses ConnectRPC auth context for password reset.
 */
export function useResetPassword(): UseResetPasswordReturn {
  const { t } = useTranslation();
  const [validSession, setValidSession] = useState(false);
  const [checkingSession, setCheckingSession] = useState(true);
  const { user, loading, updatePassword } = useAuth();
  const { toast } = useToast();
  const navigate = useNavigate();

  const form = useForm<ResetPasswordFormData>({
    resolver: zodResolver(resetPasswordSchema(t)),
    defaultValues: {
      password: '',
      confirmPassword: '',
    },
  });

  useEffect(() => {
    // Wait for auth context to finish loading
    if (loading) {
      return;
    }

    // Check if user has a valid session (authenticated via password reset link)
    if (user) {
      setValidSession(true);
    } else {
      toast({
        variant: 'destructive',
        title: t('pages.resetPassword.errors.invalidLink'),
        description: t('pages.resetPassword.errors.invalidLinkMessage'),
      });
      navigate('/forgot-password');
    }
    setCheckingSession(false);
  }, [loading, user, navigate, toast, t]);

  const onSubmit = useCallback(
    async (data: ResetPasswordFormData) => {
      // updatePassword now requires current password, but for reset flow
      // we pass empty string since the user authenticated via reset link
      const { error } = await updatePassword('', data.password);

      if (error) {
        toast({
          variant: 'destructive',
          title: t('pages.resetPassword.errors.update'),
          description: t(getAuthErrorKey(error)),
        });
      } else {
        toast({
          title: t('pages.resetPassword.success.title'),
          description: t('pages.resetPassword.success.message'),
        });
        navigate('/auth');
      }
    },
    [updatePassword, toast, t, navigate]
  );

  return {
    form,
    validSession,
    checkingSession,
    onSubmit,
  };
}
