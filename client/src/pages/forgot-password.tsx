import { ErrorBoundary } from '@/components/error-boundary';
import { Form, FormControl, FormField, FormItem, FormMessage } from '@/components/ui/form';
import { useForgotPassword } from '@/hooks';
import { CheckCircle, Send } from 'lucide-react';

function ArrowLeftIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className={className}>
      <path d="M5.5 12.002H19" />
      <path d="M10.9999 18.002C10.9999 18.002 4.99998 13.583 4.99997 12.0019C4.99996 10.4208 11 6.00195 11 6.00195" />
    </svg>
  );
}

function MailIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" className={className}>
      <path d="M2 6L8.91302 9.91697C11.4616 11.361 12.5384 11.361 15.087 9.91697L22 6" />
      <path d="M2.01577 13.4756C2.08114 16.5412 2.11383 18.0739 3.24496 19.2094C4.37608 20.3448 5.95033 20.3843 9.09883 20.4634C11.0393 20.5122 12.9607 20.5122 14.9012 20.4634C18.0497 20.3843 19.6239 20.3448 20.7551 19.2094C21.8862 18.0739 21.9189 16.5412 21.9842 13.4756C22.0053 12.4899 22.0053 11.5101 21.9842 10.5244C21.9189 7.45886 21.8862 5.92609 20.7551 4.79066C19.6239 3.65523 18.0497 3.61568 14.9012 3.53657C12.9607 3.48781 11.0393 3.48781 9.09882 3.53656C5.95033 3.61566 4.37608 3.65521 3.24495 4.79065C2.11382 5.92608 2.08114 7.45885 2.01576 10.5244C1.99474 11.5101 1.99475 12.4899 2.01577 13.4756Z" />
    </svg>
  );
}
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';

/**
 * Forgot password page component matching auth page style.
 * UI-only: all logic is handled by useForgotPassword hook.
 */
export default function ForgotPassword() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { form, emailSent, sentEmail, onSubmit, handleResend } = useForgotPassword();

  return (
    <ErrorBoundary>
      <div className="relative flex min-h-screen items-center justify-center overflow-hidden px-3 xs:px-4 sm:px-6 xl:px-8 bg-white">
        <section className="group relative z-10 w-full max-w-[calc(100%-1rem)] xs:max-w-md lg:max-w-lg xl:max-w-xl">
          {/* Card */}
          <div className="relative rounded-2xl border border-[#520B27]/10 bg-white shadow-xl">
            <div className="p-4 xs:p-6 sm:p-8 lg:p-10 xl:p-12">
              {/* Header */}
              <div className="mb-5 xs:mb-6 sm:mb-8 lg:mb-10 text-center">
                <div className="mb-4 xs:mb-5 sm:mb-6 flex justify-center">
                  <img
                    src="/novalogo.png"
                    alt="Automatisa Grupos"
                    className="h-8 xs:h-9 sm:h-10 lg:h-12 w-auto object-contain"
                  />
                </div>
                <h2 className="text-xl xs:text-2xl font-semibold tracking-tight sm:text-3xl lg:text-4xl text-[#520B27]">
                  {t('pages.forgotPassword.title')}
                </h2>
                <p className="mt-1.5 text-xs xs:text-sm lg:text-base text-gray-500">
                  {emailSent
                    ? t('pages.forgotPassword.emailSentSubtitle')
                    : t('pages.forgotPassword.subtitle')}
                </p>
              </div>

              {/* Content */}
              {emailSent ? (
                <>
                  {/* Success message */}
                  <div className="flex flex-col items-center justify-center space-y-4 rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-4 xs:p-5 sm:p-6 lg:p-8 text-center">
                    <CheckCircle className="h-8 w-8 xs:h-10 xs:w-10 sm:h-12 sm:w-12 text-emerald-500" />
                    <div>
                      <h3 className="text-base xs:text-lg font-medium text-[#520B27]">
                        {t('pages.forgotPassword.emailSent.message', { email: sentEmail })}
                      </h3>
                      <p className="mt-2 text-xs xs:text-sm text-gray-500">
                        {t('pages.forgotPassword.emailSent.spamHint')}
                      </p>
                    </div>
                  </div>

                  {/* Resend button */}
                  <button
                    type="button"
                    onClick={handleResend}
                    className="auth-btn-glow relative mt-6 inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                      <div className="auth-btn-glow__glow absolute inset-0" />
                    </div>
                    <span className="relative z-[2] flex gap-2 font-semibold select-none">
                      <Send className="h-4 w-4" />
                      {t('pages.forgotPassword.emailSent.resend')}
                    </span>
                  </button>

                  {/* Divider */}
                  <div className="relative my-4 py-1 xs:my-5 sm:my-6">
                    <div className="absolute inset-0 flex items-center" aria-hidden="true">
                      <div className="w-full border-t border-gray-200" />
                    </div>
                  </div>

                  {/* Back to login */}
                  <button
                    type="button"
                    onClick={() => navigate('/auth')}
                    className="inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold transition-all !border !border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:!border-gray-400 hover:shadow-md"
                  >
                    <ArrowLeftIcon className="h-4 w-4" />
                    {t('pages.forgotPassword.backToLogin')}
                  </button>
                </>
              ) : (
                <>
                  <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3 xs:space-y-4 sm:space-y-5">
                      {/* Email */}
                      <FormField
                        control={form.control}
                        name="email"
                        render={({ field }) => (
                          <FormItem>
                            <label
                              htmlFor="forgot-email"
                              className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
                            >
                              {t('pages.forgotPassword.form.email')}
                            </label>
                            <FormControl>
                              <div className="relative">
                                <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
                                  <MailIcon className="h-4 w-4" />
                                </div>
                                <input
                                  id="forgot-email"
                                  type="email"
                                  placeholder={t('pages.forgotPassword.form.emailPlaceholder')}
                                  {...field}
                                  className="w-full py-2 pl-9 pr-3 text-xs xs:py-2.5 xs:pl-10 xs:pr-4 xs:text-sm lg:py-3 lg:text-base transition focus:outline-none rounded-2xl !border-2 !border-gray-300 bg-gray-50 text-gray-900 placeholder-gray-400 hover:!border-[#520B27] focus:!border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30"
                                />
                              </div>
                            </FormControl>
                            <FormMessage className="pl-2.5 text-xs text-red-400" />
                          </FormItem>
                        )}
                      />

                      {/* Submit Button */}
                      <button
                        type="submit"
                        disabled={form.formState.isSubmitting}
                        className="auth-btn-glow relative mt-6 inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                          <div className="auth-btn-glow__glow absolute inset-0" />
                        </div>
                        <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                          <MailIcon className="h-4 w-4" />
                          {form.formState.isSubmitting
                            ? t('pages.forgotPassword.form.submitting')
                            : t('pages.forgotPassword.form.submit')}
                        </span>
                      </button>
                    </form>
                  </Form>

                  {/* Divider */}
                  <div className="relative my-4 py-1 xs:my-5 sm:my-6">
                    <div className="absolute inset-0 flex items-center" aria-hidden="true">
                      <div className="w-full border-t border-gray-200" />
                    </div>
                  </div>

                  {/* Back to login */}
                  <button
                    type="button"
                    onClick={() => navigate('/auth')}
                    className="inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold transition-all !border !border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:!border-gray-400 hover:shadow-md"
                  >
                    <ArrowLeftIcon className="h-4 w-4" />
                    {t('pages.forgotPassword.backToLogin')}
                  </button>
                </>
              )}
            </div>
          </div>
        </section>
      </div>
    </ErrorBoundary>
  );
}
