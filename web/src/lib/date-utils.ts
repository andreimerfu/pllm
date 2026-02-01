/**
 * Date utility functions for consistent date formatting across the application
 *
 * @module date-utils
 */

/**
 * Formats a date string to a localized date format (e.g., "Jan 15, 2024")
 *
 * @param date - ISO date string, timestamp, or null/undefined
 * @returns Formatted date string or 'N/A' if invalid
 *
 * @example
 * ```tsx
 * formatDate("2024-01-15T10:30:00Z") // "Jan 15, 2024"
 * formatDate(null) // "N/A"
 * formatDate("invalid") // "N/A"
 * ```
 */
export function formatDate(date: string | number | null | undefined): string {
  if (!date) return 'N/A';

  try {
    const parsed = new Date(date);
    if (isNaN(parsed.getTime())) return 'N/A';

    return parsed.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  } catch {
    return 'N/A';
  }
}

/**
 * Formats a date string to include both date and time (e.g., "Jan 15, 2024, 10:30 AM")
 *
 * @param date - ISO date string, timestamp, or null/undefined
 * @returns Formatted date-time string or 'N/A' if invalid
 *
 * @example
 * ```tsx
 * formatDateTime("2024-01-15T10:30:00Z") // "Jan 15, 2024, 10:30 AM"
 * ```
 */
export function formatDateTime(date: string | number | null | undefined): string {
  if (!date) return 'N/A';

  try {
    const parsed = new Date(date);
    if (isNaN(parsed.getTime())) return 'N/A';

    return parsed.toLocaleString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return 'N/A';
  }
}

/**
 * Formats a date as relative time (e.g., "2 days ago", "Today")
 *
 * @param date - ISO date string, timestamp, or null/undefined
 * @returns Relative time string or 'N/A' if invalid
 *
 * @example
 * ```tsx
 * formatRelativeTime(Date.now() - 86400000) // "Yesterday"
 * formatRelativeTime(Date.now()) // "Today"
 * ```
 */
export function formatRelativeTime(date: string | number | null | undefined): string {
  if (!date) return 'N/A';

  try {
    const parsed = new Date(date);
    if (isNaN(parsed.getTime())) return 'N/A';

    const now = new Date();
    const diffMs = now.getTime() - parsed.getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffDays === 0) return 'Today';
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7) return `${diffDays} days ago`;
    if (diffDays < 30) {
      const weeks = Math.floor(diffDays / 7);
      return `${weeks} ${weeks === 1 ? 'week' : 'weeks'} ago`;
    }
    if (diffDays < 365) {
      const months = Math.floor(diffDays / 30);
      return `${months} ${months === 1 ? 'month' : 'months'} ago`;
    }

    return formatDate(date);
  } catch {
    return 'N/A';
  }
}

/**
 * Validates if a date string is valid
 *
 * @param date - Date string or value to validate
 * @returns True if the date is valid, false otherwise
 *
 * @example
 * ```tsx
 * isValidDate("2024-01-15") // true
 * isValidDate("invalid") // false
 * isValidDate(null) // false
 * ```
 */
export function isValidDate(date: string | number | null | undefined): boolean {
  if (!date) return false;

  try {
    const parsed = new Date(date);
    return !isNaN(parsed.getTime());
  } catch {
    return false;
  }
}

/**
 * Formats a date to a short format (e.g., "Jan 15")
 * Useful for charts and compact displays
 *
 * @param date - ISO date string, timestamp, or null/undefined
 * @returns Short formatted date or 'N/A' if invalid
 *
 * @example
 * ```tsx
 * formatDateShort("2024-01-15") // "Jan 15"
 * ```
 */
export function formatDateShort(date: string | number | null | undefined): string {
  if (!date) return 'N/A';

  try {
    const parsed = new Date(date);
    if (isNaN(parsed.getTime())) return 'N/A';

    return parsed.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
    });
  } catch {
    return 'N/A';
  }
}

/**
 * Formats a date for chart axis labels, adapting to the granularity.
 * For hourly interval: "Jan 31, 2 PM"
 * For daily interval: "Jan 31"
 *
 * @param date - ISO date string, timestamp, or null/undefined
 * @param interval - "hourly" or "daily" (default: "daily")
 * @returns Formatted label string or 'N/A' if invalid
 *
 * @example
 * ```tsx
 * formatChartLabel("2026-01-31T14:00:00Z", "hourly") // "Jan 31, 2 PM"
 * formatChartLabel("2026-01-31", "daily") // "Jan 31"
 * ```
 */
export function formatChartLabel(date: string | number | null | undefined, interval: "hourly" | "daily" = "daily"): string {
  if (!date) return 'N/A';

  try {
    const parsed = new Date(date);
    if (isNaN(parsed.getTime())) return 'N/A';

    if (interval === "hourly") {
      return parsed.toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
      });
    }

    return parsed.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
    });
  } catch {
    return 'N/A';
  }
}

/**
 * Fills in missing time buckets so charts show a continuous series with zeros
 * for periods without data. Generates all buckets for the given range and
 * overlays actual data on matching buckets.
 *
 * @param data - Array of trend data points with a `date` field
 * @param interval - "hourly" or "daily"
 * @param range - Number of hours (for hourly) or days (for daily) to cover
 * @returns Complete array with zero-filled gaps
 */
export function fillTimeGaps<T extends Record<string, any>>(
  data: (T & { date: string })[],
  interval: "hourly" | "daily",
  range: number,
): (T & { date: string })[] {
  const now = new Date();
  const bucketKey = (d: Date) =>
    interval === "hourly"
      ? d.toISOString().slice(0, 13) // "2026-01-31T14"
      : d.toISOString().slice(0, 10); // "2026-01-31"

  // Build a map of existing data keyed by bucket
  const dataMap = new Map<string, T & { date: string }>();
  for (const item of data) {
    const parsed = new Date(item.date);
    if (!isNaN(parsed.getTime())) {
      dataMap.set(bucketKey(parsed), item);
    }
  }

  // Generate all buckets
  const result: (T & { date: string })[] = [];
  const count = interval === "hourly" ? range : range;

  for (let i = count - 1; i >= 0; i--) {
    const d = new Date(now);
    if (interval === "hourly") {
      d.setMinutes(0, 0, 0);
      d.setHours(d.getHours() - i);
    } else {
      d.setHours(0, 0, 0, 0);
      d.setDate(d.getDate() - i);
    }

    const key = bucketKey(d);
    const existing = dataMap.get(key);
    if (existing) {
      result.push(existing);
    } else {
      // Zero-fill — keep the date and set everything else to 0
      const dateStr = interval === "hourly" ? d.toISOString() : d.toISOString().slice(0, 10);
      result.push({ date: dateStr, requests: 0, tokens: 0, cost: 0 } as any);
    }
  }

  return result;
}

/**
 * Formats a Unix timestamp (seconds since epoch) to a date string
 *
 * @param timestamp - Unix timestamp in seconds
 * @returns Formatted date string or 'N/A' if invalid
 *
 * @example
 * ```tsx
 * formatUnixTimestamp(1705334400) // "Jan 15, 2024"
 * ```
 */
export function formatUnixTimestamp(timestamp: number | null | undefined): string {
  if (!timestamp) return 'N/A';

  try {
    // Convert seconds to milliseconds
    const parsed = new Date(timestamp * 1000);
    if (isNaN(parsed.getTime())) return 'N/A';

    return parsed.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  } catch {
    return 'N/A';
  }
}
