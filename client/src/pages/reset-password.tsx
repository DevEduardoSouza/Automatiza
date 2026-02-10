import { ErrorBoundary } from '@/components/error-boundary';
import { Button } from '@/components/ui/button';
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
import { useTheme } from 'next-themes';
import { useTranslation } from 'react-i18next';

/**
 * Reset password page component.
 * UI-only: all logic is delegated to useResetPassword hook.
 */
export default function ResetPassword() {
  const { t } = useTranslation();
  const { theme } = useTheme();
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
                src={theme === 'dark' ? '/novalogo_dark.png' : '/novalogo.png'}
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
                <Button
                  type="submit"
                  className="w-full"
                  disabled={form.formState.isSubmitting}
                  aria-busy={form.formState.isSubmitting}
                >
                  {form.formState.isSubmitting
                    ? t('pages.resetPassword.form.submitting')
                    : t('pages.resetPassword.form.submit')}
                </Button>
              </form>
            </Form>
          </CardContent>
        </Card>
      </div>
    </ErrorBoundary>
  );
}
