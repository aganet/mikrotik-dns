"use client";

import type { Summary } from "@/lib/api";
import { fmtPct } from "@/lib/format";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { RankedList } from "./ranked-list";
import { SegmentedBar } from "./segmented-bar";
import { LatencyChart, TimelineChart } from "./timeline-chart";
import { outcomes, outcomeTotals } from "./outcomes";
import type { Selection } from "./types";

interface Props {
  summary: Summary;
  onSelect: (s: Selection) => void;
  onOutcome: (filter: string) => void;
}

export function Overview({ summary, onSelect, onOutcome }: Props) {
  const { totals, timeline, bucket_secs } = summary;
  const byOutcome = outcomeTotals(totals.status);
  const hasLatency = timeline.some((b) => b.latency_ms !== null);
  const ipv4 = totals.queries - totals.ipv6;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Queries over time</CardTitle>
          <CardDescription>Stacked by outcome, {bucket_secs >= 3600 ? `${bucket_secs / 3600}h` : `${bucket_secs / 60}min`} buckets</CardDescription>
        </CardHeader>
        <CardContent>
          <TimelineChart buckets={timeline} bucketSecs={bucket_secs} />
        </CardContent>
      </Card>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Outcomes</CardTitle>
            <CardDescription>As reported by the router</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <SegmentedBar
              segments={outcomes.map((o) => ({ key: o.filter, label: o.label, value: byOutcome[o.key], color: o.color, icon: o.icon }))}
              onSelect={onOutcome}
            />
            <div>
              <div className="mb-2 text-sm font-medium">Client IP version</div>
              <SegmentedBar
                segments={[
                  { key: "ipv4", label: "IPv4", value: ipv4, color: "var(--viz-1)" },
                  { key: "ipv6", label: "IPv6", value: totals.ipv6, color: "var(--viz-2)" },
                ]}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Query types</CardTitle>
            <CardDescription>{summary.types.length} record types seen</CardDescription>
          </CardHeader>
          <CardContent>
            <RankedList
              items={summary.types.slice(0, 8).map((t) => ({
                key: t.key,
                label: <span className="font-mono">{t.key}</span>,
                value: t.count,
                sub: fmtPct(t.count, totals.queries),
              }))}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Response time</CardTitle>
            <CardDescription>Average delay between query and answer</CardDescription>
          </CardHeader>
          <CardContent>
            {hasLatency ? (
              <LatencyChart buckets={timeline} bucketSecs={bucket_secs} />
            ) : (
              <p className="py-8 text-center text-sm text-muted-foreground">
                No answer lines received yet. RouterOS 7 logs them with the dns topic.
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Top domains</CardTitle>
            <CardDescription>Most requested names</CardDescription>
          </CardHeader>
          <CardContent>
            <RankedList
              items={summary.top_domains.map((d) => ({
                key: d.domain,
                label: d.domain,
                title: d.domain,
                value: d.count,
                sub: `${d.clients} client${d.clients === 1 ? "" : "s"}`,
              }))}
              onSelect={(key) => onSelect({ kind: "domain", key })}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Top clients</CardTitle>
            <CardDescription>Most active devices</CardDescription>
          </CardHeader>
          <CardContent>
            <RankedList
              items={summary.top_clients.map((c) => ({
                key: c.client,
                label: c.name ? (
                  <>
                    {c.name} <span className="font-mono text-xs text-muted-foreground">{c.client}</span>
                  </>
                ) : (
                  <span className="font-mono">{c.client}</span>
                ),
                title: c.client,
                value: c.count,
                sub: `${c.domains} domain${c.domains === 1 ? "" : "s"}`,
              }))}
              onSelect={(key) => onSelect({ kind: "client", key })}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Problem domains</CardTitle>
            <CardDescription>Blocked, NXDOMAIN or failed answers</CardDescription>
          </CardHeader>
          <CardContent>
            <RankedList
              items={summary.problems.map((d) => ({
                key: d.domain,
                label: d.domain,
                title: d.domain,
                value: d.problems,
                sub: `${d.clients} client${d.clients === 1 ? "" : "s"}`,
              }))}
              onSelect={(key) => onSelect({ kind: "domain", key })}
              empty="No problems in this range"
            />
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
