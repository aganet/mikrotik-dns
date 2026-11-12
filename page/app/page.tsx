"use client";

import { useEffect, useState } from "react";
import { Activity, AlertTriangle, Globe, Pause, Play, Radar, RefreshCw, Timer, Users } from "lucide-react";
import { api } from "@/lib/api";
import { fmtMs, fmtNum, fmtPct } from "@/lib/format";
import { usePoll } from "@/hooks/use-poll";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ThemeToggle } from "@/components/theme-toggle";
import { StatTile } from "@/components/dashboard/stat-tile";
import { Overview } from "@/components/dashboard/overview";
import { DomainsTab } from "@/components/dashboard/domains-tab";
import { ClientsTab } from "@/components/dashboard/clients-tab";
import { QueriesTab } from "@/components/dashboard/queries-tab";
import { SystemTab } from "@/components/dashboard/system-tab";
import { DetailSheet } from "@/components/dashboard/detail-sheet";
import { outcomeTotals } from "@/components/dashboard/outcomes";
import { emptyFilter, intervalOptions, rangeOptions, type QueryFilterState, type Selection } from "@/components/dashboard/types";

const tabs = ["overview", "domains", "clients", "queries", "system"];

// The tab lives in the URL hash and the open client or domain in the query
// string, so a view can be bookmarked or shared.
function readLocation(): { tab: string; selection: Selection | null } {
  if (typeof window === "undefined") return { tab: "overview", selection: null };
  const hash = window.location.hash.slice(1);
  const params = new URLSearchParams(window.location.search);
  const client = params.get("client");
  const domain = params.get("domain");
  return {
    tab: tabs.includes(hash) ? hash : "overview",
    selection: client ? { kind: "client", key: client } : domain ? { kind: "domain", key: domain } : null,
  };
}

function writeLocation(tab: string, selection: Selection | null) {
  const params = new URLSearchParams();
  if (selection) params.set(selection.kind, selection.key);
  const search = params.toString();
  const url = `${window.location.pathname}${search ? `?${search}` : ""}${tab === "overview" ? "" : `#${tab}`}`;
  window.history.replaceState(null, "", url);
}

export default function Dashboard() {
  const [range, setRange] = useState("24h");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [interval, setInterval] = useState(10);
  const [tab, setTabState] = useState("overview");
  const [selection, setSelectionState] = useState<Selection | null>(null);
  const [filter, setFilter] = useState<QueryFilterState>(emptyFilter);

  useEffect(() => {
    const loc = readLocation();
    setTabState(loc.tab);
    setSelectionState(loc.selection);
  }, []);
  const setTab = (t: string) => {
    setTabState(t);
    writeLocation(t, selection);
  };
  const setSelection = (s: Selection | null) => {
    setSelectionState(s);
    writeLocation(tab, s);
  };

  const intervalMs = autoRefresh ? interval * 1000 : 0;
  const summary = usePoll((signal) => api.summary(range, 10, signal), [range], intervalMs);
  const s = summary.data;
  const totals = s?.totals;
  const byOutcome = totals ? outcomeTotals(totals.status) : null;
  const problems = byOutcome ? byOutcome.nxdomain + byOutcome.blocked + byOutcome.failed : 0;
  const windowMinutes = s ? Math.max(1, (s.until - s.since) / 60) : 1;

  const showQueries = (f: Partial<QueryFilterState>) => {
    setFilter({ ...emptyFilter, ...f });
    setTab("queries");
  };

  return (
    <div className="mx-auto max-w-7xl space-y-5 px-4 py-6 sm:px-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-gradient-to-br from-primary to-sky-500 text-primary-foreground shadow-lg shadow-primary/30">
            <Radar className="h-6 w-6" aria-hidden />
          </span>
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">DNS Analytics</h1>
            <p className="text-sm text-muted-foreground">
            {summary.updatedAt ? `Updated ${new Date(summary.updatedAt).toLocaleTimeString()}` : "Connecting"}
            {summary.error && <span className="ml-2 text-destructive">{summary.error}</span>}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Select value={range} onValueChange={setRange}>
            <SelectTrigger className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {rangeOptions.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className="flex h-10 items-center gap-1 rounded-md border px-2">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => setAutoRefresh((v) => !v)}
              title={autoRefresh ? "Pause auto refresh" : "Resume auto refresh"}
              aria-label={autoRefresh ? "Pause auto refresh" : "Resume auto refresh"}
            >
              {autoRefresh ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
            </Button>
            <Select value={String(interval)} onValueChange={(v) => setInterval(Number(v))} disabled={!autoRefresh}>
              <SelectTrigger className="h-7 w-20 border-0 bg-transparent shadow-none">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {intervalOptions.map((o) => (
                  <SelectItem key={o.value} value={String(o.value)}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button variant="outline" size="icon" onClick={summary.refresh} disabled={summary.loading} title="Refresh" aria-label="Refresh">
            <RefreshCw className={`h-4 w-4 ${summary.loading ? "animate-spin" : ""}`} />
          </Button>
          <ThemeToggle />
        </div>
      </header>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5" style={{ opacity: summary.loading && s ? 0.7 : 1, transition: "opacity 150ms" }}>
        <StatTile
          icon={Activity}
          tone="blue"
          label="Queries"
          value={totals ? fmtNum(totals.queries) : "-"}
          detail={totals ? `${(totals.queries / windowMinutes).toFixed(1)} per minute` : undefined}
          onClick={() => showQueries({})}
        />
        <StatTile icon={Users} tone="emerald" label="Clients" value={totals ? fmtNum(totals.clients) : "-"} detail="Unique addresses" onClick={() => setTab("clients")} />
        <StatTile icon={Globe} tone="violet" label="Domains" value={totals ? fmtNum(totals.domains) : "-"} detail="Unique names" onClick={() => setTab("domains")} />
        <StatTile
          icon={AlertTriangle}
          tone="amber"
          label="Problems"
          value={totals ? fmtNum(problems) : "-"}
          detail={totals ? `${fmtPct(problems, totals.queries)} of queries` : undefined}
          onClick={() => showQueries({ status: "problem" })}
        />
        <StatTile
          icon={Timer}
          tone="sky"
          label="Response time"
          value={totals ? fmtMs(totals.latency_ms) : "-"}
          detail={totals?.latency_ms === null ? "Needs RouterOS 7 answers" : "Average, query to answer"}
        />
      </div>

      <Tabs value={tab} onValueChange={setTab} className="space-y-4">
        <TabsList className="grid w-full grid-cols-5 sm:inline-grid sm:w-auto">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="domains">Domains</TabsTrigger>
          <TabsTrigger value="clients">Clients</TabsTrigger>
          <TabsTrigger value="queries">Queries</TabsTrigger>
          <TabsTrigger value="system">System</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          {s ? (
            <div style={{ opacity: summary.loading ? 0.7 : 1, transition: "opacity 150ms" }}>
              <Overview summary={s} onSelect={setSelection} onOutcome={(status) => showQueries({ status })} />
            </div>
          ) : (
            <p className="py-12 text-center text-sm text-muted-foreground">{summary.error ?? "Loading"}</p>
          )}
        </TabsContent>
        <TabsContent value="domains">
          <DomainsTab range={range} intervalMs={intervalMs} active={tab === "domains"} onSelect={setSelection} />
        </TabsContent>
        <TabsContent value="clients">
          <ClientsTab range={range} intervalMs={intervalMs} active={tab === "clients"} onSelect={setSelection} />
        </TabsContent>
        <TabsContent value="queries">
          <QueriesTab range={range} intervalMs={intervalMs} active={tab === "queries"} filter={filter} onFilter={setFilter} onSelect={setSelection} />
        </TabsContent>
        <TabsContent value="system">
          <SystemTab active={tab === "system"} />
        </TabsContent>
      </Tabs>

      <DetailSheet
        selection={selection}
        range={range}
        onClose={() => setSelection(null)}
        onSelect={setSelection}
        onShowAll={(f) => {
          setSelectionState(null);
          setTabState("queries");
          setFilter({ ...emptyFilter, ...f });
          writeLocation("queries", null);
        }}
      />

      {totals && totals.queries === 0 && (
        <p className="text-center text-xs text-muted-foreground">
          No queries in this range. Check the System tab for received lines, or configure the router: /system logging add topics=dns action=remote
        </p>
      )}
    </div>
  );
}
