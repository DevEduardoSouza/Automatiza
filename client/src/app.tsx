import { AccountSuspendedBanner } from '@/components/billing';
import { ErrorFallback } from '@/components/error-boundary';
import { AppSidebar, MobileHeader } from '@/components/layout';
import { AdminRoute, ProtectedRoute } from '@/components/routing';
import { PageLoader } from '@/components/ui/page-loader';
import { SidebarProvider } from '@/components/ui/sidebar';
import { Toaster as Sonner } from '@/components/ui/sonner';
import { Toaster } from '@/components/ui/toaster';
import { TooltipProvider } from '@/components/ui/tooltip';
import { AuthProvider } from '@/contexts';
import { transport } from '@/lib/connect-transport';
import { TransportProvider } from '@connectrpc/connect-query';
import * as Sentry from '@sentry/react';
import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from 'next-themes';
import { Suspense, lazy } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router';

/**
 * Wraps React.lazy with ChunkLoadError retry.
 * Captures error in Sentry and reloads once on chunk load failure (FR7).
 */
function lazyWithRetry<T extends React.ComponentType>(
  loader: () => Promise<{ default: T }>,
) {
  return lazy(() =>
    loader().catch((error: unknown) => {
      const isChunkError =
        error instanceof Error &&
        (error.name === 'ChunkLoadError' ||
          error.message.includes('Loading chunk') ||
          error.message.includes('dynamically imported module'));

      if (isChunkError) {
        Sentry.captureException(error, { tags: { type: 'chunk_load_error' } });
        window.location.reload();
        // Return empty component while reloading
        return { default: (() => null) as unknown as T };
      }

      throw error;
    }),
  );
}

// Lazy-loaded page components with ChunkLoadError retry
const Admin = lazyWithRetry(() => import('./pages/admin'));
const AffiliatePrograms = lazyWithRetry(() => import('./pages/affiliate-programs'));
const Auth = lazyWithRetry(() => import('./pages/auth'));
const AuthMagic = lazyWithRetry(() => import('./pages/auth-magic'));
const AutomationLogs = lazyWithRetry(() => import('./pages/automation-logs'));
const Coupons = lazyWithRetry(() => import('./pages/coupons'));
const CTAs = lazyWithRetry(() => import('./pages/ctas'));
const Dashboard = lazyWithRetry(() => import('./pages/dashboard'));
const DispatchTexts = lazyWithRetry(() => import('./pages/dispatch-texts'));
const ForgotPassword = lazyWithRetry(() => import('./pages/forgot-password'));
const Groups = lazyWithRetry(() => import('./pages/groups'));
const History = lazyWithRetry(() => import('./pages/history'));
const Jobs = lazyWithRetry(() => import('./pages/jobs'));
const MlLinkLists = lazyWithRetry(() => import('./pages/ml-link-lists'));
const NotFound = lazyWithRetry(() => import('./pages/not-found'));
const OfferAutomations = lazyWithRetry(() => import('./pages/offer-automations'));
const OfferGenerate = lazyWithRetry(() => import('./pages/offer-generate'));
const Profile = lazyWithRetry(() => import('./pages/profile'));
const PublicRedirect = lazyWithRetry(() => import('./pages/public-redirect'));
const RedirectLink = lazyWithRetry(() => import('./pages/redirect-link/index'));
const ResetPassword = lazyWithRetry(() => import('./pages/reset-password'));
const WhatsApp = lazyWithRetry(() => import('./pages/whatsapp'));

const queryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error) => {
      Sentry.captureException(error, {
        tags: { source: 'react-query', type: 'query' },
      });
    },
  }),
  mutationCache: new MutationCache({
    onError: (error) => {
      Sentry.captureException(error, {
        tags: { source: 'react-query', type: 'mutation' },
      });
    },
  }),
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

const AppLayout = ({ children }: { children: React.ReactNode }) => (
  <SidebarProvider>
    <div className="flex min-h-screen w-full">
      <AppSidebar />
      <main className="flex-1 flex flex-col">
        <MobileHeader />
        <AccountSuspendedBanner />
        <div className="flex-1 p-4 nav:p-8 lg:p-10 xl:p-12">{children}</div>
      </main>
    </div>
  </SidebarProvider>
);

const App = () => (
  <QueryClientProvider client={queryClient}>
    <TransportProvider transport={transport}>
      <ThemeProvider attribute="class" defaultTheme="light" forcedTheme="light" enableSystem={false}>
        <BrowserRouter>
          <AuthProvider>
            <TooltipProvider>
              <Toaster />
              <Sonner />
              <Sentry.ErrorBoundary
                fallback={({ error, resetError }) => (
                  <ErrorFallback error={error} onRetry={resetError} />
                )}
              >
                <Suspense fallback={<PageLoader />}>
                  <Routes>
                    <Route path="/" element={<Navigate to="/dashboard" replace />} />
                    <Route path="/auth" element={<Auth />} />
                    <Route path="/auth/magic" element={<AuthMagic />} />
                    <Route path="/forgot-password" element={<ForgotPassword />} />
                    <Route path="/reset-password" element={<ResetPassword />} />

                    {/* Rota publica de redirecionamento */}
                    <Route path="/r/:slug" element={<PublicRedirect />} />

                    <Route
                      path="/dashboard"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <Dashboard />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/whatsapp"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <WhatsApp />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/groups"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <Groups />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/redirect"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <RedirectLink />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/jobs"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <Jobs />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/offer-automations"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <OfferAutomations />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/offer-generate"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <OfferGenerate />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/ml-link-lists"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <MlLinkLists />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/dispatch-texts"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <DispatchTexts />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/affiliate-programs"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <AffiliatePrograms />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/ctas"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <CTAs />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/coupons"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <Coupons />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/automation-logs"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <AutomationLogs />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/history"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <History />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/profile"
                      element={
                        <ProtectedRoute>
                          <AppLayout>
                            <Profile />
                          </AppLayout>
                        </ProtectedRoute>
                      }
                    />

                    <Route
                      path="/admin"
                      element={
                        <AdminRoute>
                          <AppLayout>
                            <Admin />
                          </AppLayout>
                        </AdminRoute>
                      }
                    />

                    <Route path="*" element={<NotFound />} />
                  </Routes>
                </Suspense>
              </Sentry.ErrorBoundary>
            </TooltipProvider>
          </AuthProvider>
        </BrowserRouter>
      </ThemeProvider>
    </TransportProvider>
  </QueryClientProvider>
);

export default App;
