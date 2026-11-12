"use client";

import { Users } from "lucide-react";
import { api } from "@/lib/api";
import { fmtAgo, fmtInt } from "@/lib/format";
import { usePoll } from "@/hooks/use-poll";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { SectionTitle } from "./section-title";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { Selection } from "./types";

interface Props {
  range: string;
  intervalMs: number;
  active: boolean;
  onSelect: (s: Selection) => void;
}

export function ClientsTab({ range, intervalMs, active, onSelect }: Props) {
  const { data, loading, error } = usePoll((signal) => api.clients({ range }, 200, signal), [range], intervalMs, active);
  const rows = data ?? [];
  return (
    <Card>
      <CardHeader className="pb-3">
        <SectionTitle icon={Users} tone="emerald" title="Clients" description={`Devices by query count. Names come from DHCP leases${error ? `. ${error}` : ""}`} />
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto" style={{ opacity: loading ? 0.6 : 1, transition: "opacity 150ms" }}>
          <Table className="text-sm">
            <TableHeader>
              <TableRow>
                <TableHead className="w-10">#</TableHead>
                <TableHead>Address</TableHead>
                <TableHead>Device</TableHead>
                <TableHead>MAC</TableHead>
                <TableHead className="text-right">Queries</TableHead>
                <TableHead className="text-right">Domains</TableHead>
                <TableHead className="text-right">Problems</TableHead>
                <TableHead className="text-right">Last seen</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.length === 0 && (
                <TableRow>
                  <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                    {loading ? "Loading" : "No clients in this range"}
                  </TableCell>
                </TableRow>
              )}
              {rows.map((c, i) => (
                <TableRow key={c.client} className="cursor-pointer" onClick={() => onSelect({ kind: "client", key: c.client })}>
                  <TableCell className="tabular-nums text-muted-foreground">{i + 1}</TableCell>
                  <TableCell className="font-mono text-xs">{c.client}</TableCell>
                  <TableCell className="font-medium">{c.name || <span className="text-muted-foreground">-</span>}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{c.mac || "-"}</TableCell>
                  <TableCell className="text-right tabular-nums">{fmtInt(c.count)}</TableCell>
                  <TableCell className="text-right tabular-nums">{fmtInt(c.domains)}</TableCell>
                  <TableCell className="text-right tabular-nums">{c.problems > 0 ? fmtInt(c.problems) : "-"}</TableCell>
                  <TableCell className="whitespace-nowrap text-right text-xs text-muted-foreground">{fmtAgo(c.last_seen)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}
