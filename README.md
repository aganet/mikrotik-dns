# MikroTik DNS Analytics

A DNS logger and dashboard for MikroTik routers. The router sends its `dns`
(and optionally `dhcp`) log topics to this service over syslog; the service
stores every query with its answer in SQLite and serves a dashboard and a JSON
API. One container, one Go binary, no external database.

## What is logged

For every DNS request the router logs, the collector records:

| Field | Source |
| --- | --- |
| time, router, client address, domain, record type | `dns query from <client>: #<id> <name>. <type>` |
| outcome (`ok`, `nodata`, `nxdomain`, `servfail`, `blocked`, `error`) and the answer records | `dns done query: #<id> ...` (RouterOS 7) |
| response time | delay between the query and its done line, measured at the collector |
| device name and MAC for the client | `dhcp,info ... assigned <ip> for <mac> <hostname>` lines, and the REST API when configured |

Queries answered with `0.0.0.0` or `::` (adlists, static block entries) are
recorded as `blocked`. Routers that do not log done lines (RouterOS 6) still
get the query fields; outcome and response time stay empty.

The dashboard shows queries over time stacked by outcome, response time,
record types, top domains, top clients with device names, problem domains, a
filterable query log with CSV export, per-client and per-domain drill downs,
and collector and router health. Time ranges from 1 hour to 30 days, retention
is configurable (default 7 days).

## Quick start

```bash
git clone https://github.com/aganet/mikrotik-dns.git
cd mikrotik-dns
docker compose up -d --build
```

The dashboard is on <http://localhost:3000>, syslog is received on UDP 5354.

## MikroTik configuration

Send the `dns` topic, and `dhcp` for device names, to the collector:

```text
/system logging action add name=dnslog target=remote remote=<collector-ip> remote-port=5354
/system logging add topics=dns action=dnslog
/system logging add topics=dhcp,info action=dnslog
```

The default remote log format, BSD syslog format and ISO 8601 timestamps are
all accepted. Do not enable the `packet` sub-topic; it is not needed and only
adds noise.

Logged lines look like this on RouterOS 7:

```text
dns query from 192.168.88.10: #4711 example.com. A
dns done query: #4711 example.com 93.184.216.34
dns done query: #4712 dns name does not exist
dhcp,info defconf assigned 192.168.88.37 for B0:E4:5C:27:EF:F2 Samsung
```

### Optional: REST API enrichment

With a read-only RouterOS user the collector also pulls DHCP leases, the ARP
table and IPv6 neighbours (so IPv6 clients get the name of the device's DHCP
lease), plus identity, resources, DNS cache usage and adlist hit counts for
the System tab.

```text
/user group add name=readonly policy=read,rest-api,!local,!telnet,!ssh,!ftp,!reboot,!write,!policy,!test,!winbox,!password,!web,!sniff,!sensitive,!api,!romon
/user add name=dnslog group=readonly password=<password>
```

Then set `MIKROTIK_URL=https://<router-ip>`, `MIKROTIK_USER` and
`MIKROTIK_PASSWORD`. Routers with the default self-signed certificate need
`MIKROTIK_INSECURE=true`.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `HTTP_ADDR` | `:3000` | Dashboard and API listen address (`PORT` is honoured for compatibility) |
| `SYSLOG_ADDR` | `:5354` | UDP syslog listen address |
| `DATABASE_PATH` | `./data/queries.db` (`/data/queries.db` in Docker) | SQLite file |
| `RETENTION` | `7d` | How long queries are kept; accepts `24h`, `7d`, `30d` |
| `WEB_DIR` | unset (set in Docker) | Directory with the exported dashboard; API only when unset |
| `DNS_SERVER` | system resolver | Resolver used by "Resolve now" in the dashboard |
| `MIKROTIK_URL` | unset | RouterOS REST API base URL, e.g. `https://192.168.88.1` |
| `MIKROTIK_USER`, `MIKROTIK_PASSWORD` | unset | REST API credentials |
| `MIKROTIK_INSECURE` | `false` | Skip TLS certificate verification |
| `MIKROTIK_POLL_INTERVAL` | `1m` | REST API poll interval |
| `LOG_LEVEL` | `info` | `debug` also logs unparsed syslog lines |
| `TZ` | `UTC` | Time zone used in CSV exports and logs |

Timestamps are taken when a line arrives at the collector, so the router's
clock does not need to be correct.

Existing databases from earlier versions are migrated in place on start.

## API

All endpoints accept `range` (`1h`, `6h`, `24h`, `7d`, ...; default `24h`).
List endpoints also accept `client`, `domain` (exact), `search` (substring of
the domain), `type` and `status`. `status` is a stored outcome or one of the
groups `resolved`, `problem`, `failed`, `unknown`.

| Endpoint | Returns |
| --- | --- |
| `GET /api/summary?range=24h&limit=10` | totals, outcome counts, record types, zero-filled timeline, top domains, top clients, problem domains |
| `GET /api/timeline?range=&client=&domain=` | per-bucket counts by outcome and average response time |
| `GET /api/domains?range=&limit=&search=&status=&client=` | domains with query count, distinct clients, problems, last seen |
| `GET /api/clients?range=&limit=&domain=` | clients with device name, MAC, query count, distinct domains, problems |
| `GET /api/queries?range=&page=&page_size=&...` | paged query rows with answers and response time, plus the total |
| `GET /api/export.csv?range=&...` | the same rows as CSV |
| `GET /api/resolve?name=&type=` | live lookup through the collector's resolver |
| `GET /api/status` | collector counters, syslog sources, router snapshot |
| `GET /healthz` | liveness |

## Development

```bash
make dev-api     # collector on :8080 with debug logging
make dev-web     # Next.js dev server on :3000, proxies /api to :8080
make test        # go vet, go test -race, tsc
make build       # static dashboard export + binary; make run serves both on :3000
```

Feed test data without a router:

```bash
printf 'dns query from 192.168.88.10: #1 example.com. A\ndns done query: #1 example.com 93.184.216.34\n' | nc -u -w1 127.0.0.1 5354
```

## Architecture

```text
MikroTik --UDP syslog--> collector (Go) --> SQLite (WAL, batched writes)
                             |
                             +--> /api/*  and the exported Next.js dashboard
                             +--> RouterOS REST API (optional, polled)
```

The collector pairs each `query` line with its `done` line by request id,
batches inserts in one transaction per second, purges in chunks on a timer and
answers dashboard requests from a small read pool with a short-lived summary
cache. The dashboard is a static export served by the same binary, so there is
no Node.js process at runtime.

## Changes

See [CHANGELOG.md](CHANGELOG.md).

## License

GNU General Public License v3.0, see [LICENSE](LICENSE).

This is a derivative of [publi0/mikrotik-dns](https://github.com/publi0/mikrotik-dns)
by Felipe Publio, Copyright (C) 2025, released under the GPL-3.0. The
collector, dashboard and build were rewritten by Georgios Anestopoulos in
November 2026 (Copyright (C) 2026); see [CHANGELOG.md](CHANGELOG.md) for what
changed. The whole work remains licensed under the GPL-3.0.
