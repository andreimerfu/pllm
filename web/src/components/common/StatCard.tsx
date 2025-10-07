import { LucideIcon } from 'lucide-react';
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
 *   icon={TrendingUp}
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
   * Optional icon to display in header
   * @default undefined
   */
  icon?: LucideIcon;

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
  icon: Icon,
  trend,
  className,
  trendColorPositive = "text-emerald-500",
  trendColorNegative = "text-red-500",
}: StatCardProps) {
  const isPositiveTrend = trend && trend.value >= 0;
  const trendColor = isPositiveTrend ? trendColorPositive : trendColorNegative;

  return (
    <Card className={cn(className)}>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        {Icon && <Icon className="h-4 w-4 text-muted-foreground" />}
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value}</div>
        {description && (
          <p className="text-xs text-muted-foreground mt-1">
            {description}
          </p>
        )}
        {trend && (
          <p className={cn("text-xs mt-1", trendColor)}>
            {trend.label}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

export type { StatCardProps };
