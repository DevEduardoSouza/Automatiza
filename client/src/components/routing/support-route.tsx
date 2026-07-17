import { useAuth } from '@/contexts';
import { Navigate } from 'react-router';

interface SupportRouteProps {
  children: React.ReactNode;
}

/**
 * Rota protegida para o painel de suporte.
 * Redireciona para /auth se nao autenticado, ou para /dashboard se o usuario
 * nao for suporte nem admin. Admins tambem podem acessar o painel de suporte.
 * Usa o role do auth context (ConnectRPC) para verificacao de permissao.
 */
export function SupportRoute({ children }: SupportRouteProps) {
  const { user, loading } = useAuth();

  const canAccess = user?.role === 'support' || user?.role === 'admin';

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/auth" replace />;
  }

  if (!canAccess) {
    return <Navigate to="/dashboard" replace />;
  }

  return <>{children}</>;
}
