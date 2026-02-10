import { ErrorBoundary } from '@/components/error-boundary';
import { useAuthPage } from '@/hooks/use-auth-page';
import {
  ArrowLeft,
  CheckCircle,
  Eye,
  EyeOff,
  Lock,
  LogIn,
  Mail,
  Moon,
  Send,
  Sun,
} from 'lucide-react';
import { useTheme } from 'next-themes';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';

/**
 * Authentication page component with glassmorphism login card
 * Supports both password and magic link authentication
 */
export default function Auth() {
  const { t } = useTranslation();
  const { theme, setTheme } = useTheme();

  const toggleTheme = () => {
    setTheme(theme === 'dark' ? 'light' : 'dark');
  };
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

  const isDark = theme === 'dark';

  return (
    <ErrorBoundary>
      <div
        className={`relative flex min-h-screen items-center justify-center overflow-hidden px-4 ${
          isDark ? 'bg-gradient-to-br from-background via-background to-muted/20' : 'bg-white'
        }`}
      >
        {/* Theme Toggle - Top Left */}
        <button
          type="button"
          onClick={toggleTheme}
          className={`absolute left-4 top-4 z-20 rounded-full p-2.5 backdrop-blur-sm transition-all ${
            isDark
              ? 'border border-white/10 bg-black/30 text-gray-300 hover:border-white/20 hover:bg-black/50 hover:text-white'
              : 'border border-[#520B27]/20 bg-white text-[#520B27] shadow-sm hover:border-[#520B27]/40 hover:bg-gray-50'
          }`}
          aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
        >
          {isDark ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
        </button>

        <section className="relative z-10 w-full max-w-md lg:max-w-lg">
          {/* Card */}
          <div
            className={`rounded-2xl ${
              isDark
                ? 'border border-white/10 bg-black/50 backdrop-blur-xl'
                : 'border border-[#520B27]/10 bg-white shadow-xl'
            }`}
          >
            <div className="p-6 sm:p-8 lg:p-10">
              {/* Header */}
              <div className="mb-8 text-center">
                <div className="mb-6 flex justify-center">
                  <img
                    src={isDark ? '/novalogo_dark.png' : '/novalogo.png'}
                    alt="Automatisa Grupos"
                    className="h-10 w-auto object-contain"
                  />
                </div>
                <h2
                  className={`text-2xl font-semibold tracking-tight sm:text-3xl ${
                    isDark ? 'text-white' : 'text-[#520B27]'
                  }`}
                >
                  {authMode === 'magic' ? t('pages.auth.magicLink.title') : t('pages.auth.title')}
                </h2>
                <p className={`mt-1.5 text-sm ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
                  {authMode === 'magic'
                    ? t('pages.auth.magicLink.subtitle')
                    : t('pages.auth.subtitle')}
                </p>
              </div>

              {/* Magic Link Success State */}
              {magicLinkSent ? (
                <div className="space-y-6">
                  <div className="flex flex-col items-center justify-center space-y-4 rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-6 text-center">
                    <CheckCircle
                      className={`h-12 w-12 ${isDark ? 'text-emerald-400' : 'text-emerald-500'}`}
                    />
                    <div>
                      <h3
                        className={`text-lg font-medium ${isDark ? 'text-white' : 'text-[#520B27]'}`}
                      >
                        {t('pages.auth.magicLink.success.title')}
                      </h3>
                      <p className={`mt-2 text-sm ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
                        {t('pages.auth.magicLink.success.description', { email: magicLinkEmail })}
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={resetMagicLinkState}
                    className={`flex w-full items-center justify-center gap-2 text-sm transition ${
                      isDark
                        ? 'text-gray-400 hover:text-gray-200'
                        : 'text-gray-500 hover:text-[#520B27]'
                    }`}
                  >
                    <ArrowLeft className="h-4 w-4" />
                    {t('pages.auth.magicLink.backToPassword')}
                  </button>
                </div>
              ) : authMode === 'magic' ? (
                /* Magic Link Form */
                <div className="space-y-5">
                  <form onSubmit={handleSendMagicLink} className="space-y-5">
                    {/* Email */}
                    <div>
                      <label
                        htmlFor="magic-email"
                        className={`mb-1.5 block pl-2.5 text-xs font-medium ${isDark ? 'text-gray-300' : 'text-gray-700'}`}
                      >
                        {t('pages.auth.form.email')}
                      </label>
                      <div className="relative">
                        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-gray-400">
                          <Mail className="h-4 w-4" />
                        </div>
                        <input
                          id="magic-email"
                          type="email"
                          placeholder={t('pages.auth.form.emailPlaceholder')}
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                          required
                          className={`w-full rounded-2xl py-2.5 pl-11 pr-4 text-sm transition focus:outline-none ${
                            isDark
                              ? 'border border-white/10 bg-black/30 text-gray-100 placeholder-gray-500/60 focus:border-white/30'
                              : 'border border-gray-200 bg-gray-50 text-gray-900 placeholder-gray-400 focus:border-[#520B27]/50 focus:ring-1 focus:ring-[#520B27]/20'
                          }`}
                        />
                      </div>
                    </div>

                    {/* Submit Button */}
                    <button
                      type="submit"
                      disabled={loading}
                      aria-busy={loading}
                      className={`group relative mt-6 inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-2xl py-2.5 text-sm font-medium transition-all duration-300 disabled:cursor-not-allowed disabled:opacity-50 ${
                        isDark
                          ? 'border border-white/20 bg-transparent text-gray-300 hover:border-transparent hover:text-white hover:shadow-lg hover:shadow-rose-500/25'
                          : 'border border-[#520B27] bg-[#520B27] text-white hover:bg-[#6b1033] hover:shadow-lg hover:shadow-[#520B27]/25'
                      }`}
                    >
                      {/* Gradient background - visible on hover (dark only) */}
                      {isDark && (
                        <span className="absolute inset-0 bg-gradient-to-r from-rose-700 via-pink-600 to-rose-500 opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
                      )}
                      {/* Button content */}
                      <span className="relative z-10 flex items-center gap-2">
                        <Send className="h-4 w-4" />
                        {loading
                          ? t('pages.auth.magicLink.form.submitting')
                          : t('pages.auth.magicLink.form.submit')}
                      </span>
                    </button>
                  </form>

                  {/* Back to password login */}
                  <button
                    type="button"
                    onClick={() => setAuthMode('password')}
                    className={`flex w-full items-center justify-center gap-2 text-sm transition ${
                      isDark
                        ? 'text-gray-400 hover:text-gray-200'
                        : 'text-gray-500 hover:text-[#520B27]'
                    }`}
                  >
                    <ArrowLeft className="h-4 w-4" />
                    {t('pages.auth.magicLink.backToPassword')}
                  </button>
                </div>
              ) : (
                /* Password Form */
                <>
                  <form onSubmit={handleSignIn} className="space-y-5">
                    {/* Email */}
                    <div>
                      <label
                        htmlFor="login-email"
                        className={`mb-1.5 block pl-2.5 text-xs font-medium ${isDark ? 'text-gray-300' : 'text-gray-700'}`}
                      >
                        {t('pages.auth.form.email')}
                      </label>
                      <div className="relative">
                        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-gray-400">
                          <Mail className="h-4 w-4" />
                        </div>
                        <input
                          id="login-email"
                          type="email"
                          placeholder={t('pages.auth.form.emailPlaceholder')}
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                          required
                          className={`w-full rounded-2xl py-2.5 pl-11 pr-4 text-sm transition focus:outline-none ${
                            isDark
                              ? 'border border-white/10 bg-black/30 text-gray-100 placeholder-gray-500/60 focus:border-white/30'
                              : 'border border-gray-200 bg-gray-50 text-gray-900 placeholder-gray-400 focus:border-[#520B27]/50 focus:ring-1 focus:ring-[#520B27]/20'
                          }`}
                        />
                      </div>
                    </div>

                    {/* Password */}
                    <div>
                      <div className="mb-1.5 flex items-center justify-between">
                        <label
                          htmlFor="login-password"
                          className={`block pl-2.5 text-xs font-medium ${isDark ? 'text-gray-300' : 'text-gray-700'}`}
                        >
                          {t('pages.auth.form.password')}
                        </label>
                        <Link
                          to="/forgot-password"
                          className={`pr-2.5 text-xs underline-offset-4 hover:underline ${
                            isDark ? 'text-gray-400 hover:text-gray-200' : 'text-[#520B27]'
                          }`}
                        >
                          {t('pages.auth.forgotPassword')}
                        </Link>
                      </div>
                      <div className="relative">
                        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4 text-gray-400">
                          <Lock className="h-4 w-4" />
                        </div>
                        <input
                          id="login-password"
                          type={showPassword ? 'text' : 'password'}
                          placeholder={t('pages.auth.form.passwordPlaceholder')}
                          value={password}
                          onChange={(e) => setPassword(e.target.value)}
                          required
                          className={`w-full rounded-2xl py-2.5 pl-11 pr-11 text-sm transition focus:outline-none ${
                            isDark
                              ? 'border border-white/10 bg-black/30 text-gray-100 placeholder-gray-500/60 focus:border-white/30'
                              : 'border border-gray-200 bg-gray-50 text-gray-900 placeholder-gray-400 focus:border-[#520B27]/50 focus:ring-1 focus:ring-[#520B27]/20'
                          }`}
                        />
                        <button
                          type="button"
                          onClick={togglePasswordVisibility}
                          className={`absolute inset-y-0 right-0 flex cursor-pointer items-center pr-4 transition ${
                            isDark
                              ? 'text-gray-400 hover:text-gray-200'
                              : 'text-gray-400 hover:text-[#520B27]'
                          }`}
                          aria-label={
                            showPassword
                              ? t('pages.auth.form.hidePassword')
                              : t('pages.auth.form.showPassword')
                          }
                        >
                          {showPassword ? (
                            <EyeOff className="h-4 w-4" />
                          ) : (
                            <Eye className="h-4 w-4" />
                          )}
                        </button>
                      </div>
                    </div>

                    {/* Submit Button */}
                    <button
                      type="submit"
                      disabled={loading}
                      aria-busy={loading}
                      className={`group relative mt-6 inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-2xl py-2.5 text-sm font-medium transition-all duration-300 disabled:cursor-not-allowed disabled:opacity-50 ${
                        isDark
                          ? 'border border-white/20 bg-transparent text-gray-300 hover:border-transparent hover:text-white hover:shadow-lg hover:shadow-rose-500/25'
                          : 'border border-[#520B27] bg-[#520B27] text-white hover:bg-[#6b1033] hover:shadow-lg hover:shadow-[#520B27]/25'
                      }`}
                    >
                      {/* Gradient background - visible on hover (dark only) */}
                      {isDark && (
                        <span className="absolute inset-0 bg-gradient-to-r from-rose-700 via-pink-600 to-rose-500 opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
                      )}
                      {/* Button content */}
                      <span className="relative z-10 flex items-center gap-2">
                        <LogIn className="h-4 w-4" />
                        {loading ? t('pages.auth.form.submitting') : t('pages.auth.form.submit')}
                      </span>
                    </button>
                  </form>

                  {/* Divider */}
                  <div className="my-6 flex items-center gap-3">
                    <div className={`h-px flex-1 ${isDark ? 'bg-white/10' : 'bg-gray-200'}`} />
                    <span className={`text-xs ${isDark ? 'text-gray-500' : 'text-gray-400'}`}>
                      {t('pages.auth.orContinueWith')}
                    </span>
                    <div className={`h-px flex-1 ${isDark ? 'bg-white/10' : 'bg-gray-200'}`} />
                  </div>

                  {/* Magic Link Button */}
                  <button
                    type="button"
                    onClick={() => setAuthMode('magic')}
                    className={`group relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-2xl py-2.5 text-sm font-medium transition-all duration-300 ${
                      isDark
                        ? 'border border-white/20 bg-transparent text-gray-300 hover:border-transparent hover:text-white hover:shadow-lg hover:shadow-rose-500/25'
                        : 'border border-[#520B27]/30 bg-transparent text-[#520B27] hover:border-[#520B27] hover:bg-[#520B27]/5'
                    }`}
                  >
                    {/* Gradient background - visible on hover (dark only) */}
                    {isDark && (
                      <span className="absolute inset-0 bg-gradient-to-r from-rose-700 via-pink-600 to-rose-500 opacity-0 transition-opacity duration-300 group-hover:opacity-100" />
                    )}
                    {/* Button content */}
                    <span className="relative z-10 flex items-center gap-2">
                      <Mail className="h-4 w-4" />
                      {t('pages.auth.magicLink.button')}
                    </span>
                  </button>

                  {/* Footer */}
                  <div
                    className={`mt-8 text-center text-sm ${isDark ? 'text-gray-500' : 'text-gray-400'}`}
                  >
                    <p>{t('pages.auth.noPlan')}</p>
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
