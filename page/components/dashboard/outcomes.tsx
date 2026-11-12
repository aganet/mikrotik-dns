import { Ban, CheckCircle2, HelpCircle, Minus, XCircle, type LucideIcon } from "lucide-react";
import type { Bucket, Status } from "@/lib/api";

export type Outcome = "resolved" | "nxdomain" | "blocked" | "failed" | "unknown";

export interface OutcomeSpec {
  key: Outcome;
  label: string;
  color: string;
  icon: LucideIcon;
  filter: string; // value for the API status filter
}

// Query outcomes use the reserved status palette and always ship with an
// icon and a label, so colour never carries the meaning alone.
export const outcomes: OutcomeSpec[] = [
  { key: "resolved", label: "Resolved", color: "var(--status-good)", icon: CheckCircle2, filter: "resolved" },
  { key: "nxdomain", label: "NXDOMAIN", color: "var(--status-warning)", icon: HelpCircle, filter: "nxdomain" },
  { key: "blocked", label: "Blocked", color: "var(--status-serious)", icon: Ban, filter: "blocked" },
  { key: "failed", label: "Failed", color: "var(--status-critical)", icon: XCircle, filter: "failed" },
  { key: "unknown", label: "No reply", color: "var(--status-neutral)", icon: Minus, filter: "unknown" },
];

export function outcomeOf(status: Status): Outcome {
  switch (status) {
    case "ok":
    case "nodata":
      return "resolved";
    case "nxdomain":
      return "nxdomain";
    case "blocked":
      return "blocked";
    case "servfail":
    case "error":
      return "failed";
    default:
      return "unknown";
  }
}

export function outcomeSpec(o: Outcome): OutcomeSpec {
  return outcomes.find((s) => s.key === o) ?? outcomes[outcomes.length - 1];
}

export function outcomeTotals(status: Partial<Record<Status, number>>): Record<Outcome, number> {
  const t: Record<Outcome, number> = { resolved: 0, nxdomain: 0, blocked: 0, failed: 0, unknown: 0 };
  for (const [s, n] of Object.entries(status)) {
    t[outcomeOf(s as Status)] += n ?? 0;
  }
  return t;
}

export function presentOutcomes(buckets: Bucket[]): OutcomeSpec[] {
  return outcomes.filter((o) => buckets.some((b) => b[o.key] > 0));
}
