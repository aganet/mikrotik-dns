"use client";

import { useMemo } from "react";
import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { TooltipProps } from "recharts";
import type { Bucket } from "@/lib/api";
import { fmtInt, fmtMs, fmtNum, fmtTime, tickFormatter } from "@/lib/format";
import { outcomes, presentOutcomes, type OutcomeSpec } from "./outcomes";

const tick = { fontSize: 11, fill: "hsl(var(--muted-foreground))" };
const axisLine = { stroke: "var(--viz-grid)" };

function bucketLabel(ts: number, bucketSecs: number): string {
  const from = new Date(ts * 1000);
  const to = ts + bucketSecs;
  const day = bucketSecs >= 3600 ? from.toLocaleDateString([], { weekday: "short", day: "2-digit", month: "short" }) + " " : "";
  return `${day}${fmtTime(ts)} - ${fmtTime(to)}`;
}

function TimelineTooltip({ active, payload, label, bucketSecs, series }: TooltipProps<number, string> & { bucketSecs: number; series: OutcomeSpec[] }) {
  if (!active || !payload?.length) return null;
  const row = payload[0].payload as Bucket;
  return (
    <div className="rounded-md border bg-popover px-3 py-2 text-xs shadow-md">
      <div className="mb-1 font-medium">{bucketLabel(Number(label), bucketSecs)}</div>
      <div className="mb-1 flex justify-between gap-6">
        <span className="text-muted-foreground">Total</span>
        <span className="font-semibold tabular-nums">{fmtInt(row.total)}</span>
      </div>
      {series.map((o) => (
        <div key={o.key} className="flex items-center justify-between gap-6">
          <span className="flex items-center gap-1.5 text-muted-foreground">
            <span className="h-0.5 w-3 rounded" style={{ background: o.color }} />
            {o.label}
          </span>
          <span className="font-semibold tabular-nums">{fmtInt(row[o.key])}</span>
        </div>
      ))}
    </div>
  );
}

export function Legend({ series }: { series: OutcomeSpec[] }) {
  return (
    <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
      {series.map((o) => {
        const Icon = o.icon;
        return (
          <li key={o.key} className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-sm" style={{ background: o.color }} />
            <Icon className="h-3 w-3" aria-hidden />
            {o.label}
          </li>
        );
      })}
    </ul>
  );
}

interface Props {
  buckets: Bucket[];
  bucketSecs: number;
  height?: number;
}

// Queries per bucket, stacked by outcome.
export function TimelineChart({ buckets, bucketSecs, height = 220 }: Props) {
  const series = useMemo(() => {
    const present = presentOutcomes(buckets);
    return present.length ? present : [outcomes[0]];
  }, [buckets]);
  const tf = tickFormatter(bucketSecs);
  return (
    <div className="space-y-2">
      <div style={{ height }}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={buckets} margin={{ top: 4, right: 8, left: 0, bottom: 0 }} barCategoryGap={2}>
            <CartesianGrid vertical={false} stroke="var(--viz-grid)" />
            <XAxis dataKey="ts" tickFormatter={tf} minTickGap={56} tickLine={false} axisLine={axisLine} tick={tick} />
            <YAxis tickFormatter={fmtNum} width={44} tickLine={false} axisLine={false} tick={tick} allowDecimals={false} />
            <Tooltip
              content={<TimelineTooltip bucketSecs={bucketSecs} series={series} />}
              cursor={{ fill: "hsl(var(--muted))", opacity: 0.6 }}
              isAnimationActive={false}
            />
            {series.map((o) => (
              <Bar key={o.key} dataKey={o.key} stackId="q" fill={o.color} isAnimationActive={false} maxBarSize={24} />
            ))}
          </BarChart>
        </ResponsiveContainer>
      </div>
      {series.length > 1 && <Legend series={series} />}
    </div>
  );
}

function LatencyTooltip({ active, payload, label, bucketSecs }: TooltipProps<number, string> & { bucketSecs: number }) {
  if (!active || !payload?.length) return null;
  const row = payload[0].payload as Bucket;
  return (
    <div className="rounded-md border bg-popover px-3 py-2 text-xs shadow-md">
      <div className="mb-1 font-medium">{bucketLabel(Number(label), bucketSecs)}</div>
      <div className="flex justify-between gap-6">
        <span className="text-muted-foreground">Avg response</span>
        <span className="font-semibold tabular-nums">{fmtMs(row.latency_ms)}</span>
      </div>
      <div className="flex justify-between gap-6">
        <span className="text-muted-foreground">Queries</span>
        <span className="font-semibold tabular-nums">{fmtInt(row.total)}</span>
      </div>
    </div>
  );
}

// Average time between a query and its answer line, per bucket.
export function LatencyChart({ buckets, bucketSecs, height = 160 }: Props) {
  const tf = tickFormatter(bucketSecs);
  return (
    <div style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={buckets} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
          <CartesianGrid vertical={false} stroke="var(--viz-grid)" />
          <XAxis dataKey="ts" tickFormatter={tf} minTickGap={56} tickLine={false} axisLine={axisLine} tick={tick} />
          <YAxis tickFormatter={(v: number) => `${fmtNum(v)} ms`} width={64} tickLine={false} axisLine={false} tick={tick} />
          <Tooltip content={<LatencyTooltip bucketSecs={bucketSecs} />} cursor={{ stroke: "var(--viz-grid)" }} isAnimationActive={false} />
          <Line
            type="monotone"
            dataKey="latency_ms"
            stroke="var(--viz-1)"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4, stroke: "hsl(var(--background))", strokeWidth: 2, fill: "var(--viz-1)" }}
            connectNulls
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
