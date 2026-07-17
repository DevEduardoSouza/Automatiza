import { ROUTES } from '@/constants/routes';
import { useAuth } from '@/contexts';
import { useToast } from '@/hooks/use-toast';
import { getAuthErrorKey } from '@/lib/auth-error-mapper';
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';

type AuthMode = 'password' | 'magic';

interface UseAuthPageReturn {
  email: string;
  password: string;
  loading: boolean;
  showPassword: boolean;
  authMode: AuthMode;
  magicLinkSent: boolean;
  magicLinkEmail: string;
  setEmail: (value: string) => void;
  setPassword: (value: string) => void;
  togglePasswordVisibility: () => void;
  handleSignIn: (e: React.FormEvent) => Promise<void>;
  handleSendMagicLink: (e: React.FormEvent) => Promise<void>;
  setAuthMode: (mode: AuthMode) => void;
  resetMagicLinkState: () => void;
}

/**
 * Hook for auth page logic
 * Manages login form state and authentication (password and magic link)
 */
export function useAuthPage(): UseAuthPageReturn {
  const { t } = useTranslation();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [authMode, setAuthMode] = useState<AuthMode>('password');
  const [magicLinkSent, setMagicLinkSent] = useState(false);
  const [magicLinkEmail, setMagicLinkEmail] = useState('');
  const { signIn, sendMagicLink, user } = useAuth();
  const { toast } = useToast();
  const navigate = useNavigate();

  const togglePasswordVisibility = useCallback(() => {
    setShowPassword((prev) => !prev);
  }, []);

  const resetMagicLinkState = useCallback(() => {
    setMagicLinkSent(false);
    setMagicLinkEmail('');
    setAuthMode('password');
  }, []);

  useEffect(() => {
    if (user) {
      navigate(user.role === 'support' ? ROUTES.SUPPORT : ROUTES.DASHBOARD);
    }
  }, [user, navigate]);

  const handleSignIn = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setLoading(true);

      const { error } = await signIn(email, password);

      if (error) {
        toast({
          variant: 'destructive',
          title: t('pages.auth.errors.login'),
          description: t(getAuthErrorKey(error)),
        });
      }

      setLoading(false);
    },
    [email, password, signIn, toast, t]
  );

  const handleSendMagicLink = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setLoading(true);

      const { error } = await sendMagicLink(email);

      if (error) {
        toast({
          variant: 'destructive',
          title: t('pages.auth.magicLink.errors.sendFailed'),
          description: t(getAuthErrorKey(error)),
        });
      } else {
        setMagicLinkSent(true);
        setMagicLinkEmail(email);
        toast({
          title: t('pages.auth.magicLink.success.title'),
          description: t('pages.auth.magicLink.success.description', { email }),
        });
      }

      setLoading(false);
    },
    [email, sendMagicLink, toast, t]
  );

  return {
    email,
    password,
    loading,
    showPassword,
    authMode,
    magicLinkSent,
    magicLinkEmail,
    setEmail,
    setPassword,
    togglePasswordVisibility,
    handleSignIn,
    handleSendMagicLink,
    setAuthMode,
    resetMagicLinkState,
  };
}
