"use client";

import { useState } from "react";
import { Globe, Search } from "lucide-react";
import { api } from "@/lib/api";
import { fmtAgo, fmtInt } from "@/lib/format";
import { usePoll } from "@/hooks/use-poll";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { SectionTitle } from "./section-title";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { statusOptions, type Selection } from "./types";
import { useDebounced } from "@/hooks/use-debounced";

interface Props {
  range: string;
  intervalMs: number;
  active: boolean;
  onSelect: (s: Selection) => void;
}

export function DomainsTab({ range, intervalMs, active, onSelect }: Props) {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const q = useDebounced(search, 300);
  const { data, loading, error } = usePoll(
    (signal) => api.domains({ range, search: q, status }, 200, signal),
    [range, q, status],
    intervalMs,
    active,
  );
  const rows = data ?? [];

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <SectionTitle icon={Globe} tone="blue" title="Domains" description={`Top 200 by query count${error ? `. ${error}` : ""}`} />
          <div className="flex flex-wrap gap-2">
            <div className="relative">
              <Search className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Filter by name" className="w-56 pl-8" />
            </div>
            <Select value={status || "any"} onValueChange={(v) => setStatus(v === "any" ? "" : v)}>
              <SelectTrigger className="w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {statusOptions.map((o) => (
                  <SelectItem key={o.value || "any"} value={o.value || "any"}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto" style={{ opacity: loading ? 0.6 : 1, transition: "opacity 150ms" }}>
          <Table className="text-sm">
            <TableHeader>
              <TableRow>
                <TableHead className="w-10">#</TableHead>
                <TableHead>Domain</TableHead>
                <TableHead className="text-right">Queries</TableHead>
                <TableHead className="text-right">Clients</TableHead>
                <TableHead className="text-right">Problems</TableHead>
                <TableHead className="text-right">Last seen</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                    {loading ? "Loading" : "No domains match"}
                  </TableCell>
                </TableRow>
              )}
              {rows.map((d, i) => (
                <TableRow key={d.domain} className="cursor-pointer" onClick={() => onSelect({ kind: "domain", key: d.domain })}>
                  <TableCell className="tabular-nums text-muted-foreground">{i + 1}</TableCell>
                  <TableCell className="max-w-[32rem] truncate font-medium" title={d.domain}>
                    {d.domain}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{fmtInt(d.count)}</TableCell>
                  <TableCell className="text-right tabular-nums">{fmtInt(d.clients)}</TableCell>
                  <TableCell className="text-right tabular-nums">{d.problems > 0 ? fmtInt(d.problems) : "-"}</TableCell>
                  <TableCell className="whitespace-nowrap text-right text-xs text-muted-foreground">{fmtAgo(d.last_seen)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}
