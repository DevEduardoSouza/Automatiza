import { ErrorBoundary } from '@/components/error-boundary';
import { useNotFound } from '@/hooks/use-not-found';
import { Link } from 'react-router';

/**
 * 404 Not Found page - displays when route is not matched
 */
export default function NotFound() {
  const { title, subtitle, backHomeLabel } = useNotFound();

  return (
    <ErrorBoundary>
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="text-center">
          <h1 className="mb-4 text-4xl font-bold">{title}</h1>
          <p className="mb-4 text-xl text-muted-foreground">{subtitle}</p>
          <Link to="/" className="text-primary underline hover:text-primary/80">
            {backHomeLabel}
          </Link>
        </div>
      </div>
    </ErrorBoundary>
  );
}
