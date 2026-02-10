import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { AlertCircle, RefreshCw } from 'lucide-react';
import { Component, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

interface ErrorBoundaryProps {
  children: ReactNode;
  fallback?: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

/**
 * Error Boundary component to catch and display errors gracefully.
 * Wraps children and renders fallback UI when an error occurs.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  handleRetry = (): void => {
    this.setState({ hasError: false, error: null });
  };

  render(): ReactNode {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return <ErrorFallback error={this.state.error} onRetry={this.handleRetry} />;
    }

    return this.props.children;
  }
}

interface ErrorFallbackProps {
  error: Error | null;
  onRetry: () => void;
}

/**
 * Verifica se a string é uma chave i18n (formato: namespace.key.subkey).
 */
function isI18nKey(message: string): boolean {
  return /^[a-z]+(\.[a-z]+)+$/i.test(message);
}

/**
 * Default fallback UI for ErrorBoundary.
 */
function ErrorFallback({ error, onRetry }: ErrorFallbackProps): ReactNode {
  const { t } = useTranslation();

  const errorMessage = error?.message
    ? isI18nKey(error.message)
      ? t(error.message)
      : error.message
    : null;

  return (
    <div className="min-h-[400px] flex items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4">
            <AlertCircle className="h-12 w-12 text-destructive" aria-hidden="true" />
          </div>
          <CardTitle>{t('errorBoundary.title')}</CardTitle>
          <CardDescription>{t('errorBoundary.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {errorMessage && (
            <p className="text-sm text-muted-foreground text-center bg-muted p-2 rounded">
              {errorMessage}
            </p>
          )}
          <Button onClick={onRetry} className="w-full" aria-label={t('errorBoundary.retry')}>
            <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
            {t('errorBoundary.retry')}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

export default ErrorBoundary;
