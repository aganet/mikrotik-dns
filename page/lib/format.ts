import type { Status } from "./api";

const compact = new Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 });
const plain = new Intl.NumberFormat();

export function fmtNum(n: number): string {
  return n >= 10000 ? compact.format(n) : plain.format(n);
}

export function fmtInt(n: number): string {
  return plain.format(n);
}

export function fmtPct(part: number, whole: number, digits = 1): string {
  if (whole <= 0) return "0%";
  return `${((part / whole) * 100).toFixed(digits)}%`;
}

export function fmtMs(ms: number | null | undefined): string {
  if (ms === null || ms === undefined) return "-";
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`;
  return `${Math.round(ms)} ms`;
}

export function fmtBytes(n: number): string {
  const units = ["B", "KiB", "MiB", "GiB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

export function fmtDateTime(ts: number): string {
  return new Date(ts * 1000).toLocaleString([], {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function fmtTime(ts: number): string {
  return new Date(ts * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function fmtAgo(ts: number, now = Date.now()): string {
  const s = Math.max(0, Math.round(now / 1000 - ts));
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

export function fmtDuration(secs: number): string {
  const d = Math.floor(secs / 86400);
  const h = Math.floor((secs % 86400) / 3600);
  const m = Math.floor((secs % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m ${secs % 60}s`;
}

// Axis tick formatter that adapts to the bucket size of a timeline.
export function tickFormatter(bucketSecs: number): (ts: number) => string {
  if (bucketSecs >= 3600) {
    return (ts) =>
      new Date(ts * 1000).toLocaleString([], { weekday: "short", hour: "2-digit", minute: "2-digit" });
  }
  return fmtTime;
}

export const statusLabel: Record<Status, string> = {
  ok: "Resolved",
  nodata: "No data",
  nxdomain: "NXDOMAIN",
  servfail: "SERVFAIL",
  blocked: "Blocked",
  error: "Error",
  "": "No reply",
};

export function isProblem(s: Status): boolean {
  return s === "nxdomain" || s === "blocked" || s === "servfail" || s === "error";
}
