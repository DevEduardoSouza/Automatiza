import { Skeleton } from '@/components/ui/skeleton';

/**
 * Full-page loading fallback for lazy-loaded route components.
 * Used as Suspense fallback during code-split chunk loading.
 */
export function PageLoader() {
  return (
    <div className="flex flex-col gap-6 p-4 md:p-8 animate-in fade-in duration-300">
      {/* Header skeleton */}
      <div className="flex items-center justify-between">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-10 w-32" />
      </div>

      {/* Content skeleton */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Skeleton className="h-32 rounded-xl" />
        <Skeleton className="h-32 rounded-xl" />
        <Skeleton className="h-32 rounded-xl" />
        <Skeleton className="h-32 rounded-xl" />
      </div>

      {/* Main content area skeleton */}
      <Skeleton className="h-64 rounded-xl" />
    </div>
  );
}

/**
 * Minimal loading spinner for smaller components/modals.
 */
export function LoadingSpinner({ className }: { className?: string }) {
  return (
    <div className={`flex items-center justify-center ${className ?? ''}`}>
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
    </div>
  );
}
