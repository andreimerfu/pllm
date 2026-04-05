import * as React from 'react';
import { Icon } from '@iconify/react';
import { cn } from '@/lib/utils';

/**
 * EmptyState component for displaying empty data states with icon, title,
 * description, and optional action button.
 *
 * @example
 * ```tsx
 * <EmptyState
 *   icon={icons.users}
 *   title="No team members"
 *   description="Start building your team by adding members"
 *   action={<Button>Add Member</Button>}
 * />
 * ```
 */

interface EmptyStateProps {
  /**
   * Iconify icon string to display
   * @default undefined
   */
  icon?: string;

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
  icon,
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
      {icon && (
        <div className="rounded-xl bg-primary/10 p-4 mb-4 flex items-center justify-center">
          <Icon icon={icon} className="h-8 w-8 text-primary" />
        </div>
      )}
      <h3 className="text-[15px] font-semibold mb-1">{title}</h3>
      {description && (
        <p className="text-[13px] text-muted-foreground text-center mb-4 max-w-sm">
          {description}
        </p>
      )}
      {action}
    </div>
  );
}

export type { EmptyStateProps };
