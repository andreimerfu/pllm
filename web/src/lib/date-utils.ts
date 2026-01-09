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
