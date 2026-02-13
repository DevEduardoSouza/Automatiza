import { ErrorBoundary } from '@/components/error-boundary';
import { useAuthPage } from '@/hooks/use-auth-page';
import { CheckCircle } from 'lucide-react';

function ArrowLeftIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className={className}>
      <path d="M5.5 12.002H19" />
      <path d="M10.9999 18.002C10.9999 18.002 4.99998 13.583 4.99997 12.0019C4.99996 10.4208 11 6.00195 11 6.00195" />
    </svg>
  );
}

function EyeIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="currentColor" strokeWidth="1.5" className={className}>
      <path d="M21.544 11.045C21.848 11.4713 22 11.6845 22 12C22 12.3155 21.848 12.5287 21.544 12.955C20.1779 14.8706 16.6892 19 12 19C7.31078 19 3.8221 14.8706 2.45604 12.955C2.15201 12.5287 2 12.3155 2 12C2 11.6845 2.15201 11.4713 2.45604 11.045C3.8221 9.12944 7.31078 5 12 5C16.6892 5 20.1779 9.12944 21.544 11.045Z" />
      <path d="M15 12C15 10.3431 13.6569 9 12 9C10.3431 9 9 10.3431 9 12C9 13.6569 10.3431 15 12 15C13.6569 15 15 13.6569 15 12Z" />
    </svg>
  );
}

function EyeOffIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" className={className}>
      <path d="M22 8C22 8 18 14 12 14C6 14 2 8 2 8" />
      <path d="M15 13.5L16.5 16" />
      <path d="M20 11L22 13" />
      <path d="M2 13L4 11" />
      <path d="M9 13.5L7.5 16" />
    </svg>
  );
}

function LockIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" className={className}>
      <path d="M12.9994 2H9.99936C6.22812 2 4.34251 2 3.17093 3.17157C1.99936 4.34315 1.99936 6.22876 1.99936 10C1.99936 13.7712 1.99936 15.6569 3.17093 16.8284C4.34251 18 6.22812 18 9.99936 18H13.9994C17.7706 18 19.6563 18 20.8278 16.8284C21.481 16.1752 21.77 15.3001 21.8979 14" />
      <path d="M11.9994 18V22" />
      <path d="M7.99936 22H15.9994" />
      <path d="M10.9994 15H12.9994" />
      <path d="M16.7501 5.37603C16.7501 5.31603 16.7487 4.55715 16.7501 4.12257C16.7514 3.72548 16.7165 3.34257 16.9061 2.99217C17.6165 1.57857 19.6565 1.72257 20.1605 3.16257C20.2478 3.39964 20.2531 3.77549 20.2505 4.12257C20.2472 4.56606 20.2565 5.37603 20.2565 5.37603M16.7501 5.37603C15.6701 5.37603 15.2165 6.15603 15.0965 6.63603C14.9765 7.11603 14.9765 8.85603 15.0485 9.57603C15.2885 10.476 15.8885 10.848 16.4765 10.968C17.0165 11.016 19.2965 10.998 19.9565 10.998C20.9165 11.016 21.6365 10.656 21.9365 9.57603C21.9965 9.21603 22.0565 7.23603 21.9065 6.63603C21.5885 5.67603 20.8565 5.37603 20.2565 5.37603M16.7501 5.37603H20.2565" />
    </svg>
  );
}

function LoginIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className={className}>
      <path d="M8.00002 8C8.00002 7.42459 8.00002 7.17765 8.04465 6.92457C8.21993 5.93047 8.89355 5.09255 9.83302 4.70001C10.0723 4.60003 10.3559 4.53526 10.9232 4.40573L13.6508 3.78286C17.0405 3.00882 18.7353 2.6218 19.8677 3.51317C21 4.40454 21 6.1257 21 9.56803L21 14.432C21 17.8743 21 19.5955 19.8676 20.4868C18.7353 21.3782 17.0405 20.9912 13.6508 20.2171L10.9232 19.5943C10.3559 19.4647 10.0723 19.4 9.833 19.3C8.89353 18.9074 8.21991 18.0695 8.04462 17.0754C8 16.8224 8 16.5754 8 16" />
      <path d="M13 9C13 9 16 11.2095 16 12C16 12.7906 13 15 13 15M15.5 12H3" />
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
import { Link } from 'react-router';

/**
 * Authentication page component with glassmorphism login card
 * Supports both password and magic link authentication
 */
export default function Auth() {
  const { t } = useTranslation();

  const {
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
  } = useAuthPage();

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
                  {authMode === 'magic' ? t('pages.auth.magicLink.title') : t('pages.auth.title')}
                </h2>
                <p className="mt-1.5 text-xs xs:text-sm lg:text-base text-gray-500">
                  {authMode === 'magic'
                    ? t('pages.auth.magicLink.subtitle')
                    : t('pages.auth.subtitle')}
                </p>
              </div>

              {/* Magic Link Success State */}
              {magicLinkSent ? (
                <div className="space-y-6">
                  <div className="flex flex-col items-center justify-center space-y-4 rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-4 xs:p-5 sm:p-6 lg:p-8 text-center">
                    <CheckCircle className="h-8 w-8 xs:h-10 xs:w-10 sm:h-12 sm:w-12 text-emerald-500" />
                    <div>
                      <h3 className="text-base xs:text-lg font-medium text-[#520B27]">
                        {t('pages.auth.magicLink.success.title')}
                      </h3>
                      <p className="mt-2 text-xs xs:text-sm text-gray-500">
                        {t('pages.auth.magicLink.success.description', { email: magicLinkEmail })}
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={resetMagicLinkState}
                    className="flex w-full items-center justify-center gap-2 text-sm transition text-gray-500 hover:text-[#520B27]"
                  >
                    <ArrowLeftIcon className="h-4 w-4" />
                    {t('pages.auth.magicLink.backToPassword')}
                  </button>
                </div>
              ) : authMode === 'magic' ? (
                /* Magic Link Form */
                <>
                  <form onSubmit={handleSendMagicLink} className="space-y-3 xs:space-y-4 sm:space-y-5">
                    {/* Email */}
                    <div>
                      <label
                        htmlFor="magic-email"
                        className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
                      >
                        {t('pages.auth.form.email')}
                      </label>
                      <div className="relative">
                        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
                          <MailIcon className="h-4 w-4" />
                        </div>
                        <input
                          id="magic-email"
                          type="email"
                          placeholder={t('pages.auth.form.emailPlaceholder')}
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                          required
                          className="w-full py-2 pl-9 pr-3 text-xs xs:py-2.5 xs:pl-10 xs:pr-4 xs:text-sm lg:py-3 lg:text-base transition focus:outline-none rounded-2xl !border-2 !border-gray-300 bg-gray-50 text-gray-900 placeholder-gray-400 hover:!border-[#520B27] focus:!border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30"
                        />
                      </div>
                    </div>

                    {/* Submit Button */}
                    <button
                      type="submit"
                      disabled={loading}
                      aria-busy={loading}
                      className="auth-btn-glow relative mt-6 inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                        <div className="auth-btn-glow__glow absolute inset-0" />
                      </div>
                      <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                        <MailIcon className="h-4 w-4" />
                        {loading
                          ? t('pages.auth.magicLink.form.submitting')
                          : t('pages.auth.magicLink.form.submit')}
                      </span>
                    </button>
                  </form>

                  {/* Divider */}
                  <div className="relative my-4 py-1 xs:my-5 sm:my-6">
                    <div className="absolute inset-0 flex items-center" aria-hidden="true">
                      <div className="w-full border-t border-gray-200" />
                    </div>
                    <div className="relative flex justify-center">
                      <span className="px-2 text-[10px] uppercase tracking-wide bg-white text-gray-400">
                        {t('pages.auth.orContinueWith')}
                      </span>
                    </div>
                  </div>

                  {/* Back to password login */}
                  <button
                    type="button"
                    onClick={() => setAuthMode('password')}
                    className="inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold transition-all !border !border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:!border-gray-400 hover:shadow-md"
                  >
                    <ArrowLeftIcon className="h-4 w-4" />
                    {t('pages.auth.magicLink.backToPassword')}
                  </button>

                  {/* Footer */}
                  <div className="mt-5 xs:mt-6 sm:mt-8 text-xs">
                    <div className="text-center text-sm text-gray-400">
                      <p>{t('pages.auth.noPlan')}</p>
                    </div>
                  </div>
                </>
              ) : (
                /* Password Form */
                <>
                  <form onSubmit={handleSignIn} className="space-y-3 xs:space-y-4 sm:space-y-5">
                    {/* Email */}
                    <div>
                      <label
                        htmlFor="login-email"
                        className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
                      >
                        {t('pages.auth.form.email')}
                      </label>
                      <div className="relative">
                        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
                          <MailIcon className="h-4 w-4" />
                        </div>
                        <input
                          id="login-email"
                          type="email"
                          placeholder={t('pages.auth.form.emailPlaceholder')}
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                          required
                          className="w-full py-2 pl-9 pr-3 text-xs xs:py-2.5 xs:pl-10 xs:pr-4 xs:text-sm lg:py-3 lg:text-base transition focus:outline-none rounded-2xl !border-2 !border-gray-300 bg-gray-50 text-gray-900 placeholder-gray-400 hover:!border-[#520B27] focus:!border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30"
                        />
                      </div>
                    </div>

                    {/* Password */}
                    <div>
                      <div className="mb-1.5 flex items-center justify-between">
                        <label
                          htmlFor="login-password"
                          className="block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
                        >
                          {t('pages.auth.form.password')}
                        </label>
                        <Link
                          to="/forgot-password"
                          className="text-xs underline-offset-4 hover:underline pr-2.5 text-[#520B27]"
                        >
                          {t('pages.auth.forgotPassword')}
                        </Link>
                      </div>
                      <div className="relative">
                        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
                          <LockIcon className="h-4 w-4" />
                        </div>
                        <input
                          id="login-password"
                          type={showPassword ? 'text' : 'password'}
                          placeholder={t('pages.auth.form.passwordPlaceholder')}
                          value={password}
                          onChange={(e) => setPassword(e.target.value)}
                          required
                          className="w-full py-2 pl-9 pr-10 text-xs xs:py-2.5 xs:pl-10 xs:pr-11 xs:text-sm lg:py-3 lg:text-base transition focus:outline-none rounded-2xl !border-2 !border-gray-300 bg-gray-50 text-gray-900 placeholder-gray-400 hover:!border-[#520B27] focus:!border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30"
                        />
                        <button
                          type="button"
                          onClick={togglePasswordVisibility}
                          className="absolute inset-y-0 right-0 flex cursor-pointer items-center pr-3 transition text-gray-400 hover:text-[#520B27]"
                          aria-label={
                            showPassword
                              ? t('pages.auth.form.hidePassword')
                              : t('pages.auth.form.showPassword')
                          }
                        >
                          {showPassword ? (
                            <EyeOffIcon className="h-4 w-4" />
                          ) : (
                            <EyeIcon className="h-4 w-4" />
                          )}
                        </button>
                      </div>
                    </div>

                    {/* Submit Button */}
                    <button
                      type="submit"
                      disabled={loading}
                      aria-busy={loading}
                      className="auth-btn-glow relative mt-6 inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                        <div className="auth-btn-glow__glow absolute inset-0" />
                      </div>
                      <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                        <LoginIcon className="h-4 w-4" />
                        {loading
                          ? t('pages.auth.form.submitting')
                          : t('pages.auth.form.submit')}
                      </span>
                    </button>
                  </form>

                  {/* Divider */}
                  <div className="relative my-4 py-1 xs:my-5 sm:my-6">
                    <div className="absolute inset-0 flex items-center" aria-hidden="true">
                      <div className="w-full border-t border-gray-200" />
                    </div>
                    <div className="relative flex justify-center">
                      <span className="px-2 text-[10px] uppercase tracking-wide bg-white text-gray-400">
                        {t('pages.auth.orContinueWith')}
                      </span>
                    </div>
                  </div>

                  {/* Magic Link Button */}
                  <button
                    type="button"
                    onClick={() => setAuthMode('magic')}
                    className="inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold transition-all !border !border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:!border-gray-400 hover:shadow-md"
                  >
                    <MailIcon className="h-4 w-4" />
                    {t('pages.auth.magicLink.button')}
                  </button>

                  {/* Footer */}
                  <div className="mt-5 xs:mt-6 sm:mt-8 text-xs">
                    <div className="text-center text-sm text-gray-400">
                      <p>{t('pages.auth.noPlan')}</p>
                    </div>
                  </div>

                </>
              )}
            </div>
          </div>
        </section>
      </div>
    </ErrorBoundary>
  );
}
