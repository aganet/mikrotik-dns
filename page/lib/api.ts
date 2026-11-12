export type Status =
  | "ok"
  | "nodata"
  | "nxdomain"
  | "servfail"
  | "blocked"
  | "error"
  | "";

export interface Count {
  key: string;
  count: number;
}

export interface Bucket {
  ts: number;
  total: number;
  resolved: number;
  nxdomain: number;
  blocked: number;
  failed: number;
  unknown: number;
  latency_ms: number | null;
}

export interface DomainStat {
  domain: string;
  count: number;
  clients: number;
  problems: number;
  last_seen: number;
}

export interface ClientStat {
  client: string;
  name: string;
  mac: string;
  count: number;
  domains: number;
  problems: number;
  last_seen: number;
}

export interface QueryRow {
  id: number;
  ts: number;
  router: string;
  client: string;
  name: string;
  domain: string;
  type: string;
  status: Status;
  answer: string;
  latency_ms: number | null;
}

export interface Totals {
  queries: number;
  clients: number;
  domains: number;
  ipv6: number;
  status: Partial<Record<Status, number>>;
  latency_ms: number | null;
}

export interface Summary {
  since: number;
  until: number;
  bucket_secs: number;
  totals: Totals;
  types: Count[];
  timeline: Bucket[];
  top_domains: DomainStat[];
  top_clients: ClientStat[];
  problems: DomainStat[];
}

export interface Timeline {
  bucket_secs: number;
  since: number;
  buckets: Bucket[];
}

export interface QueryPage {
  items: QueryRow[];
  total: number;
  page: number;
  page_size: number;
}

export interface Resolution {
  status: "ok" | "nxdomain" | "blocked" | "error";
  records: string[];
  error?: string;
  duration_ms: number;
}

export interface RouterInfo {
  updated_at: number;
  error?: string;
  identity: string;
  version: string;
  board: string;
  uptime: string;
  cpu_load: number;
  free_memory: number;
  total_memory: number;
  dns_servers: string;
  dns_dynamic_servers: string;
  cache_used_kib: number;
  cache_size_kib: number;
  adlists: { url: string; matches: number; names: number }[] | null;
  leases: number;
  hosts: number;
}

export interface SystemStatus {
  version: string;
  uptime_secs: number;
  retention_secs: number;
  ingest: {
    received: number;
    queries: number;
    answered: number;
    orphan_answers: number;
    leases: number;
    unparsed: number;
    pending: number;
    routers: Record<string, number>;
  };
  store: { written: number; dropped: number; size_bytes: number };
  router: RouterInfo | null;
}

export type QueryFilter = {
  range: string;
  client?: string;
  domain?: string;
  search?: string;
  type?: string;
  status?: string;
};

type Params = Record<string, string | number | undefined>;

export function apiUrl(path: string, params: Params): string {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") qs.set(k, String(v));
  }
  const s = qs.toString();
  return s ? `${path}?${s}` : path;
}

async function get<T>(path: string, params: Params, signal?: AbortSignal): Promise<T> {
  const res = await fetch(apiUrl(path, params), { signal });
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body?.error) msg = body.error;
    } catch {
      // keep the HTTP status text
    }
    throw new Error(msg);
  }
  return res.json();
}

export const api = {
  summary: (range: string, limit: number, signal?: AbortSignal) =>
    get<Summary>("/api/summary", { range, limit }, signal),
  timeline: (f: QueryFilter, signal?: AbortSignal) =>
    get<Timeline>("/api/timeline", f, signal),
  domains: (f: QueryFilter, limit: number, signal?: AbortSignal) =>
    get<DomainStat[]>("/api/domains", { ...f, limit }, signal),
  clients: (f: QueryFilter, limit: number, signal?: AbortSignal) =>
    get<ClientStat[]>("/api/clients", { ...f, limit }, signal),
  queries: (f: QueryFilter, page: number, pageSize: number, signal?: AbortSignal) =>
    get<QueryPage>("/api/queries", { ...f, page, page_size: pageSize }, signal),
  resolve: (name: string, type: string, signal?: AbortSignal) =>
    get<Resolution>("/api/resolve", { name, type }, signal),
  status: (signal?: AbortSignal) => get<SystemStatus>("/api/status", {}, signal),
  exportUrl: (f: QueryFilter) => apiUrl("/api/export.csv", f),
};
