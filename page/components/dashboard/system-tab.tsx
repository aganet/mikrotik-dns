"use client";

import type { ReactNode } from "react";
import { Radio, Router, Server } from "lucide-react";
import { api } from "@/lib/api";
import { fmtAgo, fmtBytes, fmtDuration, fmtInt, fmtPct } from "@/lib/format";
import { usePoll } from "@/hooks/use-poll";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { SectionTitle } from "./section-title";

interface Props {
  active: boolean;
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-1 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right tabular-nums">{children}</span>
    </div>
  );
}

export function SystemTab({ active }: Props) {
  const { data, error } = usePoll((signal) => api.status(signal), [], 5000, active);
  if (!data) {
    return <p className="py-8 text-center text-sm text-muted-foreground">{error ?? "Loading"}</p>;
  }
  const { ingest, store, router } = data;
  const routers = Object.entries(ingest.routers).sort((a, b) => b[1] - a[1]);

  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <Card>
        <CardHeader className="pb-2">
          <SectionTitle icon={Server} tone="blue" title="Collector" description={`mikrotik-dns ${data.version}`} />
        </CardHeader>
        <CardContent>
          <Row label="Uptime">{fmtDuration(data.uptime_secs)}</Row>
          <Row label="Retention">{fmtDuration(data.retention_secs)}</Row>
          <Row label="Database size">{fmtBytes(store.size_bytes)}</Row>
          <Row label="Rows written">{fmtInt(store.written)}</Row>
          <Row label="Dropped (queue full)">{fmtInt(store.dropped)}</Row>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <SectionTitle icon={Radio} tone="emerald" title="Log stream" description="Syslog lines since start" />
        </CardHeader>
        <CardContent>
          <Row label="Lines received">{fmtInt(ingest.received)}</Row>
          <Row label="Queries">{fmtInt(ingest.queries)}</Row>
          <Row label="Answered">
            {fmtInt(ingest.answered)} ({fmtPct(ingest.answered, ingest.queries, 0)})
          </Row>
          <Row label="Answers without a query">{fmtInt(ingest.orphan_answers)}</Row>
          <Row label="DHCP leases seen">{fmtInt(ingest.leases)}</Row>
          <Row label="Unparsed lines">{fmtInt(ingest.unparsed)}</Row>
          <Row label="Waiting for answer">{fmtInt(ingest.pending)}</Row>
          <div className="mt-3 border-t pt-3">
            <div className="mb-1 text-sm font-medium">Sources</div>
            {routers.length === 0 && <p className="text-sm text-muted-foreground">No syslog messages received yet.</p>}
            {routers.map(([addr, ts]) => (
              <Row key={addr} label={addr}>
                {fmtAgo(ts)}
              </Row>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <SectionTitle
            icon={Router}
            tone="violet"
            title="Router"
            description={router ? (router.error ? `Poll failed: ${router.error}` : `${router.identity} via REST API, ${fmtAgo(router.updated_at)}`) : "REST API not configured"}
          />
        </CardHeader>
        <CardContent>
          {!router && (
            <p className="text-sm text-muted-foreground">
              Set MIKROTIK_URL, MIKROTIK_USER and MIKROTIK_PASSWORD to pull DHCP leases, ARP and IPv6 neighbours for device names,
              plus resource and DNS cache statistics.
            </p>
          )}
          {router && !router.error && (
            <>
              <Row label="Board">{router.board}</Row>
              <Row label="RouterOS">{router.version}</Row>
              <Row label="Uptime">{router.uptime}</Row>
              <Row label="CPU load">{router.cpu_load}%</Row>
              <Row label="Memory used">
                {fmtBytes(router.total_memory - router.free_memory)} of {fmtBytes(router.total_memory)}
              </Row>
              <Row label="DNS servers">{[router.dns_servers, router.dns_dynamic_servers].filter(Boolean).join(", ") || "-"}</Row>
              <Row label="DNS cache">
                {fmtInt(router.cache_used_kib)} of {fmtInt(router.cache_size_kib)} KiB
              </Row>
              <Row label="Known hosts">
                {fmtInt(router.hosts)} ({fmtInt(router.leases)} leases)
              </Row>
              {router.adlists && router.adlists.length > 0 && (
                <div className="mt-3 border-t pt-3">
                  <div className="mb-1 text-sm font-medium">Adlists</div>
                  {router.adlists.map((a) => (
                    <Row key={a.url} label={a.url.replace(/^https?:\/\//, "").slice(0, 40)}>
                      {fmtInt(a.matches)} hits, {fmtInt(a.names)} names
                    </Row>
                  ))}
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
