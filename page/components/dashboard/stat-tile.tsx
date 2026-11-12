import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export type Tone = "blue" | "emerald" | "violet" | "amber" | "sky" | "rose";

// Icon chip tints; data marks keep the chart palette, this is chrome only.
export const toneClass: Record<Tone, string> = {
  blue: "bg-blue-500/12 text-blue-600 dark:bg-blue-400/15 dark:text-blue-300",
  emerald: "bg-emerald-500/12 text-emerald-600 dark:bg-emerald-400/15 dark:text-emerald-300",
  violet: "bg-violet-500/12 text-violet-600 dark:bg-violet-400/15 dark:text-violet-300",
  amber: "bg-amber-500/15 text-amber-600 dark:bg-amber-400/15 dark:text-amber-300",
  sky: "bg-sky-500/12 text-sky-600 dark:bg-sky-400/15 dark:text-sky-300",
  rose: "bg-rose-500/12 text-rose-600 dark:bg-rose-400/15 dark:text-rose-300",
};

interface Props {
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  icon: LucideIcon;
  tone: Tone;
  onClick?: () => void;
}

export function StatTile({ label, value, detail, icon: Icon, tone, onClick }: Props) {
  const content = (
    <CardContent className="flex items-start gap-4 p-5">
      <span className={cn("flex h-11 w-11 shrink-0 items-center justify-center rounded-xl", toneClass[tone])}>
        <Icon className="h-5 w-5" aria-hidden />
      </span>
      <span className="min-w-0">
        <span className="block text-sm text-muted-foreground">{label}</span>
        <span className="mt-0.5 block text-2xl font-semibold leading-tight tracking-tight">{value}</span>
        {detail && <span className="mt-1 block truncate text-xs text-muted-foreground">{detail}</span>}
      </span>
    </CardContent>
  );
  if (!onClick) return <Card>{content}</Card>;
  return (
    <Card
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick();
        }
      }}
      className="cursor-pointer text-left transition-all hover:-translate-y-0.5 hover:border-primary/40 focus-visible:ring-2 focus-visible:ring-ring"
    >
      {content}
    </Card>
  );
}
