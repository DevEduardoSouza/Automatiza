import { ErrorBoundary } from '@/components/error-boundary';
import { useAuth } from '@/contexts';
import { Loader2, XCircle } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';

/**
 * Magic link verification page.
 * Verifies the token from URL and redirects to dashboard on success.
 */
export default function AuthMagic() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const { verifyMagicLink } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [verifying, setVerifying] = useState(true);

  const token = searchParams.get('token');

  useEffect(() => {
    const verify = async () => {
      if (!token) {
        setError(t('pages.auth.magicLink.errors.invalid'));
        setVerifying(false);
        return;
      }

      const { error: verifyError } = await verifyMagicLink(token);

      if (verifyError) {
        setError(t('pages.auth.magicLink.errors.invalid'));
        setVerifying(false);
      }
      // On success, verifyMagicLink already redirects to dashboard
    };

    verify();
  }, [token, verifyMagicLink, t]);

  return (
    <ErrorBoundary>
      <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-gradient-to-br from-background via-background to-muted/20 px-4">
        <section className="relative z-10 w-full max-w-md lg:max-w-lg">
          {/* Card - Glass Dark Effect */}
          <div className="rounded-2xl border border-white/10 bg-black/50 backdrop-blur-xl">
            <div className="p-6 sm:p-8 lg:p-10">
              {/* Header */}
              <div className="mb-8 text-center">
                <div className="mb-6 flex justify-center">
                  <img
                    src="/novalogo.png"
                    alt="Automatisa Grupos"
                    className="h-10 w-auto object-contain"
                  />
                </div>
              </div>

              {/* Content */}
              {verifying ? (
                <div className="flex flex-col items-center justify-center space-y-4 py-8">
                  <Loader2 className="h-12 w-12 animate-spin text-gray-400" />
                  <p className="text-sm text-gray-400">{t('pages.auth.magicLink.verifying')}</p>
                </div>
              ) : error ? (
                <div className="space-y-6">
                  <div className="flex flex-col items-center justify-center space-y-4 rounded-xl border border-red-500/20 bg-red-500/10 p-6 text-center">
                    <XCircle className="h-12 w-12 text-red-400" />
                    <div>
                      <h3 className="text-lg font-medium text-white">
                        {t('pages.auth.magicLink.errors.title')}
                      </h3>
                      <p className="mt-2 text-sm text-gray-400">{error}</p>
                    </div>
                  </div>
                  <Link
                    to="/auth"
                    className="flex w-full items-center justify-center gap-2 rounded-2xl border border-white/10 bg-black/30 py-2.5 text-sm text-gray-300 transition hover:border-white/20 hover:bg-black/40 hover:text-white"
                  >
                    {t('pages.auth.magicLink.backToLogin')}
                  </Link>
                </div>
              ) : null}
            </div>
          </div>
        </section>
      </div>
    </ErrorBoundary>
  );
}
