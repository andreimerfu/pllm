import { Icon } from '@iconify/react';
import { cn } from '@/lib/utils';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

/**
 * StatCard component for displaying metric cards with consistent styling
 *
 * @example
 * ```tsx
 * <StatCard
 *   title="Total Requests"
 *   value="45,231"
 *   description="Live data from backend"
 *   icon={icons.trendingUp}
 *   trend={{ value: 12.5, label: "+12.5% from last month" }}
 * />
 * ```
 */

interface StatCardProps {
  /**
   * Card title/metric name
   * @required
   */
  title: string;

  /**
   * Main metric value to display
   * @required
   */
  value: string | number;

  /**
   * Optional description text below value
   * @default undefined
   */
  description?: string;

  /**
   * Optional Iconify icon string to display in header
   * @default undefined
   */
  icon?: string;

  /**
   * Optional trend information
   * @default undefined
   */
  trend?: {
    value: number;
    label: string;
  };

  /**
   * Additional CSS classes
   */
  className?: string;

  /**
   * Custom color for positive trends
   * @default "text-emerald-500"
   */
  trendColorPositive?: string;

  /**
   * Custom color for negative trends
   * @default "text-red-500"
   */
  trendColorNegative?: string;
}

export function StatCard({
  title,
  value,
  description,
  icon,
  trend,
  className,
  trendColorPositive = "text-emerald-500",
  trendColorNegative = "text-red-500",
}: StatCardProps) {
  const isPositiveTrend = trend && trend.value >= 0;
  const trendColor = isPositiveTrend ? trendColorPositive : trendColorNegative;

  return (
    <Card className={cn("bg-card border rounded-lg p-4", className)}>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 p-0">
        <CardTitle className="text-[11px] font-medium text-muted-foreground">{title}</CardTitle>
        {icon && <Icon icon={icon} className="h-4 w-4 text-muted-foreground" />}
      </CardHeader>
      <CardContent className="p-0 pt-2">
        <div className="text-2xl font-bold font-mono">{value}</div>
        {description && (
          <p className="text-[11px] text-muted-foreground mt-1">
            {description}
          </p>
        )}
        {trend && (
          <p className={cn("text-[11px] mt-1", trendColor)}>
            {trend.label}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

export type { StatCardProps };
