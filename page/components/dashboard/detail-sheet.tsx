"use client";

import { useState } from "react";
import { Copy, Globe, RefreshCw } from "lucide-react";
import { api, type Resolution } from "@/lib/api";
import { fmtInt, fmtMs, fmtPct, statusLabel } from "@/lib/format";
import { usePoll } from "@/hooks/use-poll";
import { useCopy } from "@/hooks/use-copy";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { QueriesTable } from "./queries-table";
import { RankedList } from "./ranked-list";
import { TimelineChart } from "./timeline-chart";
import type { Selection } from "./types";

interface Head {
  count: number;
  related: number;
  problems: number;
  name?: string;
  mac?: string;
}

interface Related {
  key: string;
  name?: string;
  value: number;
}

interface Props {
  selection: Selection | null;
  range: string;
  onClose: () => void;
  onSelect: (s: Selection) => void;
}

export function DetailSheet({ selection, range, onClose, onSelect }: Props) {
  return (
    <Sheet open={selection !== null} onOpenChange={(open) => !open && onClose()}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-3xl">
        {selection && <DetailBody key={`${selection.kind}:${selection.key}`} selection={selection} range={range} onSelect={onSelect} />}
      </SheetContent>
    </Sheet>
  );
}

function DetailBody({ selection, range, onSelect }: { selection: Selection; range: string; onSelect: (s: Selection) => void }) {
  const copy = useCopy();
  const isClient = selection.kind === "client";
  const filter = isClient ? { range, client: selection.key } : { range, domain: selection.key };

  const timeline = usePoll((signal) => api.timeline(filter, signal), [range, selection.key]);
  const related = usePoll(
    async (signal): Promise<Related[]> =>
      isClient
        ? (await api.domains(filter, 15, signal)).map((d) => ({ key: d.domain, value: d.count }))
        : (await api.clients(filter, 15, signal)).map((c) => ({ key: c.client, name: c.name, value: c.count })),
    [range, selection.key],
  );
  const problems = usePoll((signal) => api.domains({ ...filter, status: "problem" }, 10, signal), [range, selection.key], 0, isClient);
  const recent = usePoll((signal) => api.queries(filter, 1, 25, signal), [range, selection.key]);
  const head = usePoll(
    async (signal): Promise<Head | null> => {
      if (isClient) {
        const [c] = await api.clients(filter, 1, signal);
        return c ? { count: c.count, related: c.domains, problems: c.problems, name: c.name, mac: c.mac } : null;
      }
      const [d] = await api.domains(filter, 1, signal);
      return d ? { count: d.count, related: d.clients, problems: d.problems } : null;
    },
    [range, selection.key],
  );

  const buckets = timeline.data?.buckets ?? [];
  const totals = buckets.reduce(
    (acc, b) => {
      acc.total += b.total;
      acc.resolved += b.resolved;
      return acc;
    },
    { total: 0, resolved: 0 },
  );
  const info = head.data;

  return (
    <div className="space-y-6">
      <SheetHeader className="text-left">
        <SheetTitle className="flex items-center gap-2 break-all font-mono text-lg">
          {selection.key}
          <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={() => copy(selection.key)} title="Copy">
            <Copy className="h-4 w-4" />
          </Button>
        </SheetTitle>
        <SheetDescription>
          {isClient ? (
            <>
              {info?.name || "Unnamed device"}
              {info?.mac && <span className="ml-2 font-mono text-xs">{info.mac}</span>}
            </>
          ) : (
            "Domain activity in the selected range"
          )}
        </SheetDescription>
      </SheetHeader>

      <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Stat label="Queries" value={info ? fmtInt(info.count) : "-"} />
        <Stat label={isClient ? "Domains" : "Clients"} value={info ? fmtInt(info.related) : "-"} />
        <Stat label="Problems" value={info ? `${fmtInt(info.problems)} (${fmtPct(info.problems, info.count, 0)})` : "-"} />
        <Stat label="Resolved" value={fmtPct(totals.resolved, totals.total, 0)} />
      </dl>

      <section className="space-y-2">
        <h3 className="text-sm font-medium">Queries over time</h3>
        {timeline.data ? (
          <TimelineChart buckets={buckets} bucketSecs={timeline.data.bucket_secs} height={180} />
        ) : (
          <p className="text-sm text-muted-foreground">{timeline.error ?? "Loading"}</p>
        )}
      </section>

      {!isClient && <ResolveNow domain={selection.key} />}

      <div className="grid gap-6 md:grid-cols-2">
        <section className="space-y-2">
          <h3 className="text-sm font-medium">{isClient ? "Top domains" : "Clients"}</h3>
          <RankedList
            items={(related.data ?? []).map((r) => ({
              key: r.key,
              title: r.key,
              value: r.value,
              label: isClient ? (
                r.key
              ) : r.name ? (
                <>
                  {r.name} <span className="font-mono text-xs text-muted-foreground">{r.key}</span>
                </>
              ) : (
                <span className="font-mono">{r.key}</span>
              ),
            }))}
            onSelect={(key) => onSelect({ kind: isClient ? "domain" : "client", key })}
          />
        </section>
        {isClient && (
          <section className="space-y-2">
            <h3 className="text-sm font-medium">Problem domains</h3>
            <RankedList
              items={(problems.data ?? []).map((d) => ({ key: d.domain, label: d.domain, title: d.domain, value: d.problems }))}
              onSelect={(key) => onSelect({ kind: "domain", key })}
              empty="No problems in this range"
            />
          </section>
        )}
      </div>

      <section className="space-y-2">
        <h3 className="text-sm font-medium">Recent queries</h3>
        <QueriesTable rows={recent.data?.items ?? []} loading={recent.loading} hideClient={isClient} hideDomain={!isClient} onSelect={onSelect} />
      </section>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border p-3">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="mt-1 text-lg font-semibold tabular-nums">{value}</dd>
    </div>
  );
}

// Looks the domain up right now through the collector's resolver, so the
// logged answers can be compared with the current state.
function ResolveNow({ domain }: { domain: string }) {
  const [type, setType] = useState("A");
  const [result, setResult] = useState<Resolution | null>(null);
  const [busy, setBusy] = useState(false);
  const run = async (t: string) => {
    setType(t);
    setBusy(true);
    try {
      setResult(await api.resolve(domain, t));
    } catch (err) {
      setResult({ status: "error", records: [], error: err instanceof Error ? err.message : String(err), duration_ms: 0 });
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className="space-y-2 rounded-md border p-3">
      <div className="flex flex-wrap items-center gap-2">
        <Globe className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm font-medium">Resolve now</span>
        {["A", "AAAA", "CNAME", "MX", "TXT"].map((t) => (
          <Button key={t} size="sm" variant={type === t && result ? "default" : "outline"} className="h-7 font-mono text-xs" disabled={busy} onClick={() => run(t)}>
            {t}
          </Button>
        ))}
        {busy && <RefreshCw className="h-4 w-4 animate-spin text-muted-foreground" />}
      </div>
      {result && (
        <div className="text-sm">
          <span className="font-medium">{statusLabel[result.status]}</span>
          <span className="ml-2 text-xs text-muted-foreground">{fmtMs(result.duration_ms)}</span>
          {result.error && <div className="mt-1 text-xs text-muted-foreground">{result.error}</div>}
          {result.records.length > 0 && (
            <ul className="mt-1 space-y-0.5 font-mono text-xs">
              {result.records.map((r) => (
                <li key={r} className="break-all">
                  {r}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}
