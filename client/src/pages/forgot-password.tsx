import { ErrorBoundary } from '@/components/error-boundary';
import { Form, FormControl, FormField, FormItem, FormMessage } from '@/components/ui/form';
import { useForgotPassword } from '@/hooks';
import { ArrowLeft, Mail, Send } from 'lucide-react';
import { useTheme } from 'next-themes';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router';

/**
 * Forgot password page component with glassmorphism card.
 * UI-only: all logic is handled by useForgotPassword hook.
 */
export default function ForgotPassword() {
  const { t } = useTranslation();
  const { theme } = useTheme();
  const navigate = useNavigate();
  const { form, emailSent, sentEmail, onSubmit, handleResend } = useForgotPassword();

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
                    src={theme === 'dark' ? '/novalogo_dark.png' : '/novalogo.png'}
                    alt="Automatisa Grupos"
                    className="h-10 w-auto object-contain"
                  />
                </div>
                <h2 className="text-2xl font-semibold tracking-tight text-white sm:text-3xl">
                  {t('pages.forgotPassword.title')}
                </h2>
                <p className="mt-1.5 text-sm text-gray-400">
                  {emailSent
                    ? t('pages.forgotPassword.emailSentSubtitle')
                    : t('pages.forgotPassword.subtitle')}
                </p>
              </div>

              {/* Content */}
              {emailSent ? (
                <div className="space-y-5">
                  {/* Success message */}
                  <div className="rounded-2xl border border-white/10 bg-black/30 p-4 text-center">
                    <p className="text-sm text-gray-300">
                      {t('pages.forgotPassword.emailSent.message', { email: sentEmail })}
                    </p>
                    <p className="mt-2 text-sm text-gray-500">
                      {t('pages.forgotPassword.emailSent.spamHint')}
                    </p>
                  </div>

                  {/* Resend button */}
                  <button
                    type="button"
                    onClick={handleResend}
                    className="group relative mt-6 inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-2xl border border-white/20 bg-transparent py-2.5 text-sm font-medium text-gray-300 transition-all duration-300 hover:border-transparent hover:text-white hover:shadow-lg hover:shadow-rose-500/25"
                  >
                    <span className="absolute inset-0 bg-gradient-to-r from-rose-700 via-pink-600 to-rose-500 opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
                    <span className="relative z-10 flex items-center gap-2">
                      <Send className="h-4 w-4" />
                      {t('pages.forgotPassword.emailSent.resend')}
                    </span>
                  </button>

                  {/* Back to login */}
                  <button
                    type="button"
                    onClick={() => navigate('/auth')}
                    className="inline-flex w-full cursor-pointer items-center justify-center gap-2 rounded-2xl py-2.5 text-sm font-medium text-gray-400 transition-colors hover:text-white"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    {t('pages.forgotPassword.backToLogin')}
                  </button>
                </div>
              ) : (
                <Form {...form}>
                  <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-5">
                    {/* Email */}
                    <FormField
                      control={form.control}
                      name="email"
                      render={({ field }) => (
                        <FormItem>
                          <label
                            htmlFor="forgot-email"
                            className="mb-1.5 block pl-2.5 text-xs font-medium text-gray-300"
                          >
                            {t('pages.forgotPassword.form.email')}
                          </label>
                          <FormControl>
                            <div className="relative">
                              <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-gray-400">
                                <Mail className="h-4 w-4" />
                              </div>
                              <input
                                id="forgot-email"
                                type="email"
                                placeholder={t('pages.forgotPassword.form.emailPlaceholder')}
                                {...field}
                                className="w-full rounded-2xl border border-white/10 bg-black/30 py-2.5 pl-11 pr-4 text-sm text-gray-100 placeholder-gray-500/60 transition focus:border-white/30 focus:outline-none"
                              />
                            </div>
                          </FormControl>
                          <FormMessage className="pl-2.5 text-xs text-red-400" />
                        </FormItem>
                      )}
                    />

                    {/* Submit */}
                    <button
                      type="submit"
                      disabled={form.formState.isSubmitting}
                      className="group relative mt-6 inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-2xl border border-white/20 bg-transparent py-2.5 text-sm font-medium text-gray-300 transition-all duration-300 hover:border-transparent hover:text-white hover:shadow-lg hover:shadow-rose-500/25 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <span className="absolute inset-0 bg-gradient-to-r from-rose-700 via-pink-600 to-rose-500 opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
                      <span className="relative z-10 flex items-center gap-2">
                        <Send className="h-4 w-4" />
                        {form.formState.isSubmitting
                          ? t('pages.forgotPassword.form.submitting')
                          : t('pages.forgotPassword.form.submit')}
                      </span>
                    </button>

                    {/* Back to login */}
                    <div className="text-center">
                      <Link
                        to="/auth"
                        className="inline-flex items-center gap-1 text-sm text-gray-400 transition-colors hover:text-white"
                      >
                        <ArrowLeft className="h-3 w-3" />
                        {t('pages.forgotPassword.backToLogin')}
                      </Link>
                    </div>
                  </form>
                </Form>
              )}
            </div>
          </div>
        </section>
      </div>
    </ErrorBoundary>
  );
}
