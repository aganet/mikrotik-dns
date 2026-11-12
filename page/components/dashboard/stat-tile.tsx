import type { ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface Props {
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  onClick?: () => void;
}

export function StatTile({ label, value, detail, onClick }: Props) {
  const content = (
    <CardContent className="p-4">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="mt-1 text-2xl font-semibold leading-tight">{value}</div>
      {detail && <div className="mt-1 text-xs text-muted-foreground">{detail}</div>}
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
      className={cn("cursor-pointer text-left transition-colors hover:bg-accent/40 focus-visible:ring-2 focus-visible:ring-ring")}
    >
      {content}
    </Card>
  );
}
