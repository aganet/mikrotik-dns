import type { ReactNode } from "react";
import { fmtInt } from "@/lib/format";
import { cn } from "@/lib/utils";

export interface RankedItem {
  key: string;
  label: ReactNode;
  sub?: ReactNode;
  value: number;
  title?: string;
}

interface Props {
  items: RankedItem[];
  onSelect?: (key: string) => void;
  empty?: string;
  className?: string;
}

// A meter list: one row per item, bars on a shared baseline in one hue.
export function RankedList({ items, onSelect, empty = "No data in this range", className }: Props) {
  if (items.length === 0) {
    return <p className="py-6 text-center text-sm text-muted-foreground">{empty}</p>;
  }
  const max = Math.max(...items.map((i) => i.value), 1);
  return (
    <ol className={cn("space-y-1", className)}>
      {items.map((item, i) => {
        const row = (
          <>
            <span className="w-5 shrink-0 text-right text-xs tabular-nums text-muted-foreground">{i + 1}</span>
            <span className="min-w-0 flex-1">
              <span className="flex items-baseline justify-between gap-3">
                <span className="truncate text-sm font-medium" title={item.title}>
                  {item.label}
                </span>
                <span className="shrink-0 text-sm tabular-nums">{fmtInt(item.value)}</span>
              </span>
              <span className="mt-1 flex items-center gap-3">
                <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
                  <span
                    className="block h-full rounded-full bg-viz-1"
                    style={{ width: `${(item.value / max) * 100}%` }}
                  />
                </span>
                {item.sub && <span className="shrink-0 text-xs text-muted-foreground">{item.sub}</span>}
              </span>
            </span>
          </>
        );
        return (
          <li key={item.key}>
            {onSelect ? (
              <button
                type="button"
                onClick={() => onSelect(item.key)}
                className="flex w-full items-start gap-2 rounded-md px-1 py-1.5 text-left transition-colors hover:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {row}
              </button>
            ) : (
              <div className="flex items-start gap-2 px-1 py-1.5">{row}</div>
            )}
          </li>
        );
      })}
    </ol>
  );
}
