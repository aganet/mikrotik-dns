"use client";

import { useEffect, useState } from "react";
import { Download, Search, X } from "lucide-react";
import { api } from "@/lib/api";
import { fmtInt } from "@/lib/format";
import { usePoll } from "@/hooks/use-poll";
import { useDebounced } from "@/hooks/use-debounced";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { QueriesTable } from "./queries-table";
import { emptyFilter, statusOptions, type QueryFilterState, type Selection } from "./types";

interface Props {
  range: string;
  intervalMs: number;
  active: boolean;
  filter: QueryFilterState;
  onFilter: (f: QueryFilterState) => void;
  onSelect: (s: Selection) => void;
}

const pageSizes = [25, 50, 100, 250];

export function QueriesTab({ range, intervalMs, active, filter, onFilter, onSelect }: Props) {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const search = useDebounced(filter.search, 300);
  const client = useDebounced(filter.client, 300);
  const type = useDebounced(filter.type, 300);
  const apiFilter = { range, search, client, type: type.toUpperCase(), status: filter.status };

  useEffect(() => setPage(1), [range, search, client, type, filter.status, pageSize]);

  const { data, loading, error } = usePoll(
    (signal) => api.queries(apiFilter, page, pageSize, signal),
    [range, search, client, type, filter.status, page, pageSize],
    page === 1 ? intervalMs : 0,
    active,
  );
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / pageSize));
  const filtered = filter.search || filter.client || filter.type || filter.status;

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <CardTitle className="text-base">Query log</CardTitle>
            <CardDescription>
              {data ? `${fmtInt(total)} queries match` : "Loading"}
              {error ? `. ${error}` : ""}
            </CardDescription>
          </div>
          <Button asChild variant="outline" size="sm">
            <a href={api.exportUrl(apiFilter)} download>
              <Download className="mr-2 h-4 w-4" />
              Export CSV
            </a>
          </Button>
        </div>
        <div className="flex flex-wrap gap-2 pt-2">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              value={filter.search}
              onChange={(e) => onFilter({ ...filter, search: e.target.value })}
              placeholder="Domain contains"
              className="w-56 pl-8"
            />
          </div>
          <Input
            value={filter.client}
            onChange={(e) => onFilter({ ...filter, client: e.target.value })}
            placeholder="Client address"
            className="w-44 font-mono"
          />
          <Input
            value={filter.type}
            onChange={(e) => onFilter({ ...filter, type: e.target.value })}
            placeholder="Type"
            className="w-24 font-mono uppercase"
          />
          <Select value={filter.status || "any"} onValueChange={(v) => onFilter({ ...filter, status: v === "any" ? "" : v })}>
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
          {filtered && (
            <Button variant="ghost" size="sm" onClick={() => onFilter(emptyFilter)}>
              <X className="mr-1 h-4 w-4" />
              Clear
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        <QueriesTable rows={data?.items ?? []} loading={loading} onSelect={onSelect} />
        <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
          <div className="flex items-center gap-2 text-muted-foreground">
            Rows per page
            <Select value={String(pageSize)} onValueChange={(v) => setPageSize(Number(v))}>
              <SelectTrigger className="h-8 w-20">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {pageSizes.map((n) => (
                  <SelectItem key={n} value={String(n)}>
                    {n}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
              Previous
            </Button>
            <span className="tabular-nums text-muted-foreground">
              Page {page} of {fmtInt(pages)}
            </span>
            <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => setPage((p) => p + 1)}>
              Next
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
