import { useAuth } from '@/contexts';
import { Navigate } from 'react-router';

interface AdminRouteProps {
  children: React.ReactNode;
}

/**
 * Rota protegida para administradores.
 * Redireciona para /auth se nao autenticado ou /dashboard se nao for admin.
 * Usa o role do auth context (ConnectRPC) para verificacao de permissao.
 */
export function AdminRoute({ children }: AdminRouteProps) {
  const { user, loading } = useAuth();

  // Check admin role directly from auth context
  const isAdmin = user?.role === 'admin';

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

  if (!isAdmin) {
    return <Navigate to="/dashboard" replace />;
  }

  return <>{children}</>;
}
