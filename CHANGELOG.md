# Changelog

## 2.0.0 - 2026-11-12

Rewrite of the collector and the dashboard. Ports (3000, 5354/udp) and the
`/data` volume are unchanged; existing databases are migrated in place.

### Added

- Answers are logged, not only questions: each query stores its outcome
  (`ok`, `nodata`, `nxdomain`, `servfail`, `blocked`, `error`), the answer
  records and the response time, taken from the RouterOS 7
  `dns done query` lines.
- Device names: `dhcp,info ... assigned` lines map client addresses to
  hostnames and MACs. Optional RouterOS REST polling adds DHCP leases, ARP and
  IPv6 neighbours (IPv6 clients inherit the lease name through the MAC), plus
  identity, resources, DNS cache usage and adlist hit counts.
- Any syslog envelope is accepted: PRI, BSD or ISO timestamps, hostname,
  user prefix.
- Time ranges from 1 hour to 30 days; retention is configurable (`RETENTION`,
  default 7 days).
- Dashboard: timeline stacked by outcome, response-time chart, outcome and
  IPv4/IPv6 split, record types, top domains, top clients with device names,
  problem domains, filterable query log with CSV export, client and domain
  drill-downs, System tab with collector and router health, deep links
  (`#queries`, `?client=`, `?domain=`), dark mode.
- API: `/api/summary`, `/api/timeline`, `/api/domains`, `/api/clients`,
  `/api/queries`, `/api/export.csv`, `/api/resolve`, `/api/status`,
  `/healthz`, all taking `range` and filters.
- Tests for the parser, the query/answer correlator and the store, run in CI
  together with the TypeScript type-check.

### Changed

- The dashboard is a static export served by the Go binary. The container no
  longer runs Node.js; the image is about 20 MB and starts one process.
- Inserts are batched in one transaction per second instead of one autocommit
  per line, and the per-query log output is gone.
- Covering indexes make every dashboard aggregate index-only. On a million
  rows the 7-day summary went from about 5 s to under 1 s.
- Purges run in chunks on a timer; `PRAGMA optimize` keeps planner statistics
  fresh.
- Live DNS resolution is on demand per domain ("Resolve now") instead of once
  per search result.
- Timestamps are the time a line arrives at the collector, so the router clock
  does not matter.
- CI runs tests and builds the image; images are published only for `v*`
  tags.

### Fixed

- The README said MIT; the project has always shipped the GPL-3.0 text in
  `LICENSE`, so the documentation now states GPL-3.0.

### Removed

- The old endpoints (`/api/top-domains`, `/api/query-types`, `/api/clients`
  with the old shape, `/api/unique-*-count`, `/api/queries-per-*`,
  `/api/ipv4-vs-ipv6`, `/api/all-queries`, `/api/domain-queries`,
  `/api/domain-clients`, `/api/client-queries`), replaced by the API above.
- Demo pages (`/switches`, `/dashboard-with-darkmode`), unused UI components
  and about thirty unused frontend dependencies.
- `BACKEND_URL`; `PORT` is still honoured as an alias for `HTTP_ADDR`.

## 1.x

Upstream project by publi0: query logging over UDP, SQLite with 24 hour
retention, Next.js dashboard with a Go API behind a proxy.
