import * as React from 'react';
import { cn } from '@/lib/utils';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';

/**
 * PageHeader component for consistent page headers with title, description,
 * actions, and optional breadcrumbs
 *
 * @example
 * ```tsx
 * <PageHeader
 *   title="API Keys"
 *   description="Manage your API keys and access tokens"
 *   actions={<Button>Generate Key</Button>}
 *   breadcrumbs={[
 *     { label: "Dashboard", href: "/" },
 *     { label: "API Keys", href: "/keys" }
 *   ]}
 * />
 * ```
 */

export interface BreadcrumbItemData {
  label: string;
  href?: string;
}

interface PageHeaderProps {
  /**
   * Page title
   * @required
   */
  title: string;

  /**
   * Optional page description
   * @default undefined
   */
  description?: string;

  /**
   * Optional action buttons or elements
   * @default undefined
   */
  actions?: React.ReactNode;

  /**
   * Optional breadcrumb items
   * @default undefined
   */
  breadcrumbs?: BreadcrumbItemData[];

  /**
   * Additional CSS classes for container
   */
  className?: string;

  /**
   * Additional CSS classes for title
   */
  titleClassName?: string;

  /**
   * Additional CSS classes for description
   */
  descriptionClassName?: string;
}

export function PageHeader({
  title,
  description,
  actions,
  breadcrumbs,
  className,
  titleClassName,
  descriptionClassName,
}: PageHeaderProps) {
  return (
    <div className={cn("space-y-4", className)}>
      {/* Breadcrumbs */}
      {breadcrumbs && breadcrumbs.length > 0 && (
        <Breadcrumb>
          <BreadcrumbList>
            {breadcrumbs.map((item, index) => {
              const isLast = index === breadcrumbs.length - 1;
              return (
                <React.Fragment key={index}>
                  <BreadcrumbItem>
                    {isLast || !item.href ? (
                      <BreadcrumbPage>{item.label}</BreadcrumbPage>
                    ) : (
                      <BreadcrumbLink href={item.href}>
                        {item.label}
                      </BreadcrumbLink>
                    )}
                  </BreadcrumbItem>
                  {!isLast && <BreadcrumbSeparator />}
                </React.Fragment>
              );
            })}
          </BreadcrumbList>
        </Breadcrumb>
      )}

      {/* Header content */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div className="space-y-1">
          <h1 className={cn(
            "text-2xl lg:text-3xl font-bold",
            titleClassName
          )}>
            {title}
          </h1>
          {description && (
            <p className={cn(
              "text-sm lg:text-base text-muted-foreground",
              descriptionClassName
            )}>
              {description}
            </p>
          )}
        </div>

        {/* Actions */}
        {actions && (
          <div className="flex items-center gap-2">
            {actions}
          </div>
        )}
      </div>
    </div>
  );
}

export type { PageHeaderProps };
