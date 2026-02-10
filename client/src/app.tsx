import { AccountSuspendedBanner } from '@/components/billing';
import { AppSidebar } from '@/components/layout';
import { AdminRoute, ProtectedRoute } from '@/components/routing';
import { PageLoader } from '@/components/ui/page-loader';
import { SidebarProvider } from '@/components/ui/sidebar';
import { Toaster as Sonner } from '@/components/ui/sonner';
import { Toaster } from '@/components/ui/toaster';
import { TooltipProvider } from '@/components/ui/tooltip';
import { AuthProvider } from '@/contexts';
import { transport } from '@/lib/connect-transport';
import { TransportProvider } from '@connectrpc/connect-query';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from 'next-themes';
import { Suspense, lazy } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router';

// Lazy-loaded page components for code splitting
const Admin = lazy(() => import('./pages/admin'));
const AffiliatePrograms = lazy(() => import('./pages/affiliate-programs'));
const Auth = lazy(() => import('./pages/auth'));
const AuthMagic = lazy(() => import('./pages/auth-magic'));
const AutomationLogs = lazy(() => import('./pages/automation-logs'));
const Coupons = lazy(() => import('./pages/coupons'));
const Dashboard = lazy(() => import('./pages/dashboard'));
const DispatchTexts = lazy(() => import('./pages/dispatch-texts'));
const ForgotPassword = lazy(() => import('./pages/forgot-password'));
const Groups = lazy(() => import('./pages/groups'));
const History = lazy(() => import('./pages/history'));
const Jobs = lazy(() => import('./pages/jobs'));
const MlLinkLists = lazy(() => import('./pages/ml-link-lists'));
const NotFound = lazy(() => import('./pages/not-found'));
const OfferAutomations = lazy(() => import('./pages/offer-automations'));
const Profile = lazy(() => import('./pages/profile'));
const PublicRedirect = lazy(() => import('./pages/public-redirect'));
const RedirectLink = lazy(() => import('./pages/redirect-link/index'));
const ResetPassword = lazy(() => import('./pages/reset-password'));
const WhatsApp = lazy(() => import('./pages/whatsapp'));

const queryClient = new QueryClient();

const AppLayout = ({ children }: { children: React.ReactNode }) => (
  <SidebarProvider>
    <div className="flex min-h-screen w-full">
      <AppSidebar />
      <main className="flex-1 flex flex-col">
        <AccountSuspendedBanner />
        <div className="flex-1 p-4 md:p-8">{children}</div>
      </main>
    </div>
  </SidebarProvider>
);

const App = () => (
  <QueryClientProvider client={queryClient}>
    <TransportProvider transport={transport}>
      <ThemeProvider attribute="class" defaultTheme="light" enableSystem={false}>
        <BrowserRouter>
          <AuthProvider>
            <TooltipProvider>
              <Toaster />
              <Sonner />
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
            </TooltipProvider>
          </AuthProvider>
        </BrowserRouter>
      </ThemeProvider>
    </TransportProvider>
  </QueryClientProvider>
);

export default App;
