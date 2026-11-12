import type { LucideIcon } from "lucide-react";
import { CardDescription, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { toneClass, type Tone } from "./stat-tile";

interface Props {
  icon: LucideIcon;
  tone: Tone;
  title: string;
  description?: string;
}

export function SectionTitle({ icon: Icon, tone, title, description }: Props) {
  return (
    <div className="flex items-center gap-3">
      <span className={cn("flex h-8 w-8 shrink-0 items-center justify-center rounded-lg", toneClass[tone])}>
        <Icon className="h-4 w-4" aria-hidden />
      </span>
      <div className="min-w-0">
        <CardTitle className="text-base">{title}</CardTitle>
        {description && <CardDescription className="mt-0.5">{description}</CardDescription>}
      </div>
    </div>
  );
}
