export interface Selection {
  kind: "client" | "domain";
  key: string;
}

export interface QueryFilterState {
  search: string;
  client: string;
  type: string;
  status: string;
}

export const emptyFilter: QueryFilterState = { search: "", client: "", type: "", status: "" };

export const rangeOptions = [
  { value: "1h", label: "Last hour" },
  { value: "6h", label: "Last 6 hours" },
  { value: "24h", label: "Last 24 hours" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
];

export const intervalOptions = [
  { value: 5, label: "5s" },
  { value: 10, label: "10s" },
  { value: 30, label: "30s" },
  { value: 60, label: "1m" },
  { value: 300, label: "5m" },
];

export const statusOptions = [
  { value: "", label: "Any outcome" },
  { value: "resolved", label: "Resolved" },
  { value: "problem", label: "Any problem" },
  { value: "nxdomain", label: "NXDOMAIN" },
  { value: "blocked", label: "Blocked" },
  { value: "failed", label: "Failed" },
  { value: "nodata", label: "No data" },
  { value: "unknown", label: "No reply" },
];
