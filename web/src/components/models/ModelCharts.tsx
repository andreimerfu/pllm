import { LineChart, Line, AreaChart, Area, ResponsiveContainer } from "recharts";
import { TrendingUp, TrendingDown, Minus } from "lucide-react";

interface SparklineChartProps {
  data: number[];
  color?: string;
  type?: "line" | "area";
  className?: string;
}

export function SparklineChart({ 
  data, 
  color = "#3b82f6", 
  type = "line",
  className = "h-12 w-24" 
}: SparklineChartProps) {
  if (!data || data.length === 0) {
    return <div className={`${className} flex items-center justify-center text-muted-foreground`}>
      <Minus className="h-4 w-4" />
    </div>;
  }

  const chartData = data.map((value, index) => ({ value, index }));

  return (
    <div className={className}>
      <ResponsiveContainer width="100%" height="100%">
        {type === "area" ? (
          <AreaChart data={chartData}>
            <Area
              type="monotone"
              dataKey="value"
              stroke={color}
              fill={color}
              fillOpacity={0.2}
              strokeWidth={2}
            />
          </AreaChart>
        ) : (
          <LineChart data={chartData}>
            <Line
              type="monotone"
              dataKey="value"
              stroke={color}
              strokeWidth={2}
              dot={false}
            />
          </LineChart>
        )}
      </ResponsiveContainer>
    </div>
  );
}

interface TrendIndicatorProps {
  current: number;
  previous: number;
  className?: string;
}

export function TrendIndicator({ current, previous, className = "" }: TrendIndicatorProps) {
  if (previous === 0) {
    return <Minus className={`h-4 w-4 text-muted-foreground ${className}`} />;
  }

  const change = ((current - previous) / previous) * 100;
  const isPositive = change > 0;
  const isNeutral = Math.abs(change) < 1;

  if (isNeutral) {
    return <Minus className={`h-4 w-4 text-muted-foreground ${className}`} />;
  }

  return (
    <div className={`flex items-center gap-1 ${className}`}>
      {isPositive ? (
        <TrendingUp className="h-4 w-4 text-green-600" />
      ) : (
        <TrendingDown className="h-4 w-4 text-red-600" />
      )}
      <span className={`text-sm font-medium ${isPositive ? 'text-green-600' : 'text-red-600'}`}>
        {Math.abs(change).toFixed(1)}%
      </span>
    </div>
  );
}

interface MetricCardProps {
  label: string;
  value: string | number;
  trend?: number[];
  previousValue?: number;
  icon?: React.ReactNode;
  color?: string;
}

export function MetricCard({ 
  label, 
  value, 
  trend, 
  previousValue, 
  icon, 
  color = "#3b82f6" 
}: MetricCardProps) {
  return (
    <div className="p-3 bg-muted/20 rounded-lg border">
      <div className="flex items-center gap-2 mb-2">
        {icon && <div className="flex-shrink-0">{icon}</div>}
        <div className="min-w-0 flex-1">
          <p className="text-xs text-muted-foreground truncate">{label}</p>
          <p className="font-semibold text-sm">{value}</p>
        </div>
      </div>
      {(previousValue !== undefined || trend) && (
        <div className="flex items-center justify-between gap-2">
          {previousValue !== undefined && (
            <TrendIndicator 
              current={typeof value === 'string' ? 0 : value} 
              previous={previousValue}
              className="flex-shrink-0"
            />
          )}
          {trend && (
            <div className="flex-1 min-w-0">
              <SparklineChart data={trend} color={color} className="h-8 w-full" />
            </div>
          )}
        </div>
      )}
    </div>
  );
}