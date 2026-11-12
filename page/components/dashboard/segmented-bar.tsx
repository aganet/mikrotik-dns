import type { LucideIcon } from "lucide-react";
import { fmtInt, fmtPct } from "@/lib/format";

export interface Segment {
  key: string;
  label: string;
  value: number;
  color: string;
  icon?: LucideIcon;
}

interface Props {
  segments: Segment[];
  onSelect?: (key: string) => void;
}

// A thin part-to-whole bar with the figures listed beside it.
export function SegmentedBar({ segments, onSelect }: Props) {
  const total = segments.reduce((s, p) => s + p.value, 0);
  const visible = segments.filter((s) => s.value > 0);
  return (
    <div className="space-y-3">
      <div className="flex h-2 w-full gap-[2px] overflow-hidden rounded-full" role="img" aria-label={visible.map((s) => `${s.label} ${fmtPct(s.value, total)}`).join(", ")}>
        {visible.map((s) => (
          <div key={s.key} className="h-full" style={{ width: `${(s.value / total) * 100}%`, background: s.color }} />
        ))}
        {total === 0 && <div className="h-full w-full bg-muted" />}
      </div>
      <ul className="space-y-0.5">
        {segments.map((s) => {
          const Icon = s.icon;
          const inner = (
            <>
              <span className="h-2.5 w-2.5 shrink-0 rounded-sm" style={{ background: s.color }} />
              {Icon && <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />}
              <span className="flex-1 truncate">{s.label}</span>
              <span className="tabular-nums">{fmtInt(s.value)}</span>
              <span className="w-12 text-right tabular-nums text-muted-foreground">{fmtPct(s.value, total)}</span>
            </>
          );
          return (
            <li key={s.key} className="text-sm">
              {onSelect ? (
                <button
                  type="button"
                  onClick={() => onSelect(s.key)}
                  className="flex w-full items-center gap-2 rounded px-1 py-0.5 text-left hover:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {inner}
                </button>
              ) : (
                <div className="flex items-center gap-2 px-1 py-0.5">{inner}</div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
