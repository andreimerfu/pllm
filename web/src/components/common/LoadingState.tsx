import { Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Card, CardContent, CardHeader } from '@/components/ui/card';

/**
 * LoadingState component for consistent loading indicators
 *
 * @example
 * ```tsx
 * // Spinner variant
 * <LoadingState variant="spinner" text="Loading data..." />
 *
 * // Skeleton variant for cards
 * <LoadingState variant="cards" rows={3} />
 *
 * // Skeleton variant for table
 * <LoadingState variant="table" rows={5} />
 * ```
 */

interface LoadingStateProps {
  /**
   * Loading indicator variant
   * @default "spinner"
   */
  variant?: 'spinner' | 'skeleton' | 'table' | 'cards';

  /**
   * Number of skeleton rows/items to display
   * @default 3
   */
  rows?: number;

  /**
   * Loading text to display with spinner
   * @default "Loading..."
   */
  text?: string;

  /**
   * Additional CSS classes
   */
  className?: string;
}

export function LoadingState({
  variant = 'spinner',
  rows = 3,
  text = 'Loading...',
  className,
}: LoadingStateProps) {
  if (variant === 'spinner') {
    return (
      <div className={cn(
        "flex items-center justify-center py-12",
        className
      )}>
        <div className="flex items-center gap-2">
          <Loader2 className="h-4 w-4 animate-spin" />
          <span className="text-muted-foreground">{text}</span>
        </div>
      </div>
    );
  }

  if (variant === 'table') {
    return (
      <div className={cn("space-y-3", className)}>
        {Array.from({ length: rows }).map((_, i) => (
          <div key={i} className="flex items-center space-x-4">
            <div className="h-4 w-4 bg-muted animate-pulse rounded" />
            <div className="h-4 w-24 bg-muted animate-pulse rounded" />
            <div className="h-4 w-16 bg-muted animate-pulse rounded" />
            <div className="h-4 w-12 bg-muted animate-pulse rounded" />
            <div className="h-4 w-16 bg-muted animate-pulse rounded" />
            <div className="h-4 w-12 bg-muted animate-pulse rounded ml-auto" />
          </div>
        ))}
      </div>
    );
  }

  if (variant === 'cards') {
    return (
      <div className={cn("grid gap-4 md:grid-cols-2 lg:grid-cols-4", className)}>
        {Array.from({ length: rows }).map((_, i) => (
          <Card key={i}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <div className="h-4 w-24 bg-muted animate-pulse rounded" />
              <div className="h-4 w-4 bg-muted animate-pulse rounded" />
            </CardHeader>
            <CardContent>
              <div className="h-8 w-20 bg-muted animate-pulse rounded mb-2" />
              <div className="h-3 w-32 bg-muted animate-pulse rounded" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  // skeleton variant (default content skeleton)
  return (
    <div className={cn("space-y-4", className)}>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="space-y-2">
          <div className="h-4 w-full bg-muted animate-pulse rounded" />
          <div className="h-4 w-5/6 bg-muted animate-pulse rounded" />
        </div>
      ))}
    </div>
  );
}

export type { LoadingStateProps };
