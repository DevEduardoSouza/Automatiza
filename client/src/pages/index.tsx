import { ErrorBoundary } from '@/components/error-boundary';
import { useIndex } from '@/hooks/use-index';

/**
 * Landing page component.
 * Displays the welcome screen with title and subtitle for visitors.
 * Uses useIndex hook for translation logic following project patterns.
 */
export default function Index() {
  const { title, subtitle } = useIndex();

  return (
    <ErrorBoundary>
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="text-center">
          <h1 className="mb-4 text-4xl font-bold">{title}</h1>
          <p className="text-xl text-muted-foreground">{subtitle}</p>
        </div>
      </div>
    </ErrorBoundary>
  );
}
