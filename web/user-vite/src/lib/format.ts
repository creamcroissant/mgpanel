/**
 * Format bytes to human readable string
 */
export function formatBytes(bytes: number, decimals = 2): string {
  if (bytes === 0) return "0 B";

  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];

  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + " " + sizes[i];
}

/**
 * Format Unix timestamp to date string
 */
export function formatDate(timestamp: number): string {
  if (!timestamp) return "-";
  const date = new Date(timestamp * 1000);
  return date.toLocaleDateString();
}

/**
 * Format Unix timestamp to datetime string
 */
export function formatDateTime(timestamp: number): string {
  if (!timestamp) return "-";
  const date = new Date(timestamp * 1000);
  return date.toLocaleString();
}

/**
 * Format currency (cents to yuan/dollars)
 */
export function formatCurrency(cents: number, symbol = "¥"): string {
  return `${symbol}${(cents / 100).toFixed(2)}`;
}

/**
 * Calculate days until expiration
 */
export function daysUntil(timestamp: number): number {
  if (!timestamp) return Infinity;
  const now = Math.floor(Date.now() / 1000);
  const diff = timestamp - now;
  return Math.ceil(diff / 86400);
}

/**
 * Check if timestamp is expired
 */
export function isExpired(timestamp: number): boolean {
  if (!timestamp) return false;
  return timestamp * 1000 < Date.now();
}

/**
 * Format percentage
 */
export function formatPercent(value: number, total: number): string {
  if (total === 0) return "0%";
  return ((value / total) * 100).toFixed(1) + "%";
}
