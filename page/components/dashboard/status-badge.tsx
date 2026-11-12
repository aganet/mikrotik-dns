import type { Status } from "@/lib/api";
import { statusLabel } from "@/lib/format";
import { cn } from "@/lib/utils";
import { outcomeOf, outcomeSpec } from "./outcomes";

export function StatusBadge({ status, className }: { status: Status; className?: string }) {
  const spec = outcomeSpec(outcomeOf(status));
  const Icon = spec.icon;
  return (
    <span
      className={cn("inline-flex items-center gap-1 whitespace-nowrap text-xs font-medium", className)}
      title={statusLabel[status]}
    >
      <Icon className="h-3.5 w-3.5 shrink-0" style={{ color: spec.color }} aria-hidden />
      {statusLabel[status]}
    </span>
  );
}
