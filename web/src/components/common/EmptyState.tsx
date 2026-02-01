import * as React from 'react';
import { LucideIcon } from 'lucide-react';
import { cn } from '@/lib/utils';

/**
 * EmptyState component for displaying empty data states with icon, title,
 * description, and optional action button.
 *
 * @example
 * ```tsx
 * <EmptyState
 *   icon={Users}
 *   title="No team members"
 *   description="Start building your team by adding members"
 *   action={<Button>Add Member</Button>}
 * />
 * ```
 */

interface EmptyStateProps {
  /**
   * Lucide icon component to display
   * @default undefined
   */
  icon?: LucideIcon;

  /**
   * Primary heading text
   * @required
   */
  title: string;

  /**
   * Optional description text below title
   * @default undefined
   */
  description?: string;

  /**
   * Optional action button or element
   * @default undefined
   */
  action?: React.ReactNode;

  /**
   * Additional CSS classes
   */
  className?: string;
}

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div className={cn(
      "flex flex-col items-center justify-center py-16 px-4",
      "rounded-lg bg-muted/40",
      className
    )}>
      {Icon && (
        <div className="rounded-full bg-muted p-4 mb-4">
          <Icon className="h-8 w-8 text-muted-foreground" />
        </div>
      )}
      <h3 className="text-base font-semibold mb-1">{title}</h3>
      {description && (
        <p className="text-muted-foreground text-center mb-4 max-w-sm text-sm">
          {description}
        </p>
      )}
      {action}
    </div>
  );
}

export type { EmptyStateProps };
