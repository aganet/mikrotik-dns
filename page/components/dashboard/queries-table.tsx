"use client";

import { Copy } from "lucide-react";
import type { QueryRow } from "@/lib/api";
import { fmtDateTime, fmtMs } from "@/lib/format";
import { useCopy } from "@/hooks/use-copy";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { StatusBadge } from "./status-badge";
import type { Selection } from "./types";

interface Props {
  rows: QueryRow[];
  loading?: boolean;
  hideClient?: boolean;
  hideDomain?: boolean;
  onSelect: (s: Selection) => void;
}

export function QueriesTable({ rows, loading, hideClient, hideDomain, onSelect }: Props) {
  const copy = useCopy();
  return (
    <div className="overflow-x-auto" style={{ opacity: loading ? 0.6 : 1, transition: "opacity 150ms" }}>
      <Table className="text-sm">
        <TableHeader>
          <TableRow>
            <TableHead className="w-40">Time</TableHead>
            {!hideClient && <TableHead>Client</TableHead>}
            {!hideDomain && <TableHead>Domain</TableHead>}
            <TableHead className="w-20">Type</TableHead>
            <TableHead className="w-28">Outcome</TableHead>
            <TableHead>Answer</TableHead>
            <TableHead className="w-20 text-right">Latency</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.length === 0 && (
            <TableRow>
              <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                {loading ? "Loading" : "No queries match"}
              </TableCell>
            </TableRow>
          )}
          {rows.map((q) => (
            <TableRow key={q.id}>
              <TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">{fmtDateTime(q.ts)}</TableCell>
              {!hideClient && (
                <TableCell className="whitespace-nowrap">
                  <button
                    type="button"
                    className="font-mono text-xs hover:underline"
                    onClick={() => onSelect({ kind: "client", key: q.client })}
                    title="Show this client"
                  >
                    {q.client}
                  </button>
                  {q.name && <span className="ml-2 text-xs text-muted-foreground">{q.name}</span>}
                </TableCell>
              )}
              {!hideDomain && (
                <TableCell className="max-w-[28rem]">
                  <span className="group flex items-center gap-1">
                    <button
                      type="button"
                      className="truncate font-medium hover:underline"
                      onClick={() => onSelect({ kind: "domain", key: q.domain })}
                      title={q.domain}
                    >
                      {q.domain}
                    </button>
                    <button
                      type="button"
                      onClick={() => copy(q.domain)}
                      className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100 focus-visible:opacity-100"
                      title="Copy domain"
                      aria-label="Copy domain"
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </button>
                  </span>
                </TableCell>
              )}
              <TableCell>
                <Badge variant="outline" className="font-mono text-xs">
                  {q.type}
                </Badge>
              </TableCell>
              <TableCell>
                <StatusBadge status={q.status} />
              </TableCell>
              <TableCell className="max-w-[20rem] truncate font-mono text-xs text-muted-foreground" title={q.answer}>
                {q.answer}
              </TableCell>
              <TableCell className="text-right font-mono text-xs tabular-nums text-muted-foreground">{fmtMs(q.latency_ms)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
