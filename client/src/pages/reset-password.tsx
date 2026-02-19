import { ErrorBoundary } from '@/components/error-boundary';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { useResetPassword } from '@/hooks';
import { useTranslation } from 'react-i18next';

/**
 * Reset password page component.
 * UI-only: all logic is delegated to useResetPassword hook.
 */
export default function ResetPassword() {
  const { t } = useTranslation();
  const { form, validSession, checkingSession, onSubmit } = useResetPassword();

  if (checkingSession) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-background via-background to-muted/20 p-4">
        <Card className="w-full max-w-md border-border/50 shadow-xl">
          <CardHeader className="space-y-1 text-center pb-8">
            <Skeleton className="h-10 w-32 mx-auto mb-6" />
            <Skeleton className="h-6 w-48 mx-auto" />
            <Skeleton className="h-4 w-64 mx-auto" />
          </CardHeader>
          <CardContent className="space-y-4">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </CardContent>
        </Card>
      </div>
    );
  }

  if (!validSession) {
    return null;
  }

  return (
    <ErrorBoundary>
      <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-background via-background to-muted/20 p-4">
        <Card className="w-full max-w-md border-border/50 shadow-xl">
          <CardHeader className="space-y-1 text-center pb-8">
            <div className="mb-6 flex justify-center">
              <img
                src="/novalogo.png"
                alt="Automatisa Grupos"
                className="h-10 w-auto object-contain"
              />
            </div>
            <CardTitle className="text-xl">{t('pages.resetPassword.title')}</CardTitle>
            <CardDescription>{t('pages.resetPassword.subtitle')}</CardDescription>
          </CardHeader>
          <CardContent>
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                <FormField
                  control={form.control}
                  name="password"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('pages.resetPassword.form.newPassword')}</FormLabel>
                      <FormControl>
                        <Input
                          type="password"
                          placeholder={t('pages.resetPassword.form.passwordPlaceholder')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="confirmPassword"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('pages.resetPassword.form.confirmPassword')}</FormLabel>
                      <FormControl>
                        <Input
                          type="password"
                          placeholder={t('pages.resetPassword.form.passwordPlaceholder')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <button
                  type="submit"
                  disabled={form.formState.isSubmitting}
                  aria-busy={form.formState.isSubmitting}
                  className="animated-save-btn group/btn mt-6 w-full inline-flex items-center gap-2 !border !border-gray-300 shadow-sm transition relative overflow-hidden font-semibold rounded-full py-2 text-xs xs:py-2.5 xs:text-sm lg:py-3 lg:text-base justify-center"
                >
                  {form.formState.isSubmitting
                    ? t('pages.resetPassword.form.submitting')
                    : t('pages.resetPassword.form.submit')}

                  <span className="animated-save-btn-circle absolute right-[0.3em] flex items-center justify-center h-[calc(100%-0.4em)] w-[3em] rounded-full">
                    {form.formState.isSubmitting ? (
                      <div className="w-[1.1em] h-[1.1em] border-2 border-white/20 border-t-white rounded-full animate-spin" />
                    ) : (
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        viewBox="0 0 24 24"
                        className="animated-save-btn-icon w-[1.1em] h-[1.1em] text-white"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="1.5"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      >
                        <path d="M8 7.86859V7.34663C8 5.90823 9.02094 4.67225 10.4335 4.40061L17.4335 3.05446C19.2836 2.69866 21 4.1164 21 6.00048V17.9995C21 19.8836 19.2836 21.3013 17.4335 20.9455L10.4335 19.5994C9.02094 19.3278 8 18.0918 8 16.6534V16.1314" />
                        <path d="M13 14.9995L16 11.9995L13 8.99953M15.5 11.9995H3" />
                      </svg>
                    )}
                  </span>
                </button>
              </form>
            </Form>
          </CardContent>
        </Card>
      </div>
    </ErrorBoundary>
  );
}
