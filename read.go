package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Filter narrows every read to a time window and optional dimensions.
type Filter struct {
	Since  time.Time
	Client string
	Domain string // exact match
	Search string // substring match on domain
	Type   string
	Status string // stored status or one of the groups: resolved, failed, unknown
}

const problemStatuses = `('nxdomain','blocked','servfail','error')`

func (f Filter) where() (string, []any) {
	clauses := []string{"timestamp >= ?"}
	args := []any{f.Since.Unix()}
	if f.Client != "" {
		clauses = append(clauses, "client = ?")
		args = append(args, f.Client)
	}
	if f.Domain != "" {
		clauses = append(clauses, "domain = ?")
		args = append(args, strings.ToLower(f.Domain))
	}
	if f.Search != "" {
		clauses = append(clauses, `domain LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(strings.ToLower(f.Search))+"%")
	}
	if f.Type != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, strings.ToUpper(f.Type))
	}
	switch f.Status {
	case "":
	case "resolved":
		clauses = append(clauses, "status IN ('ok','nodata')")
	case "failed":
		clauses = append(clauses, "status IN ('servfail','error')")
	case "problem":
		clauses = append(clauses, "status IN "+problemStatuses)
	case "unknown":
		clauses = append(clauses, "status = ''")
	default:
		clauses = append(clauses, "status = ?")
		args = append(args, f.Status)
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

type Count struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type Bucket struct {
	TS        int64    `json:"ts"`
	Total     int64    `json:"total"`
	Resolved  int64    `json:"resolved"`
	NXDomain  int64    `json:"nxdomain"`
	Blocked   int64    `json:"blocked"`
	Failed    int64    `json:"failed"`
	Unknown   int64    `json:"unknown"`
	LatencyMS *float64 `json:"latency_ms"`
}

type DomainStat struct {
	Domain   string `json:"domain"`
	Count    int64  `json:"count"`
	Clients  int64  `json:"clients"`
	Problems int64  `json:"problems"`
	LastSeen int64  `json:"last_seen"`
}

type ClientStat struct {
	Client   string `json:"client"`
	Name     string `json:"name"`
	MAC      string `json:"mac"`
	Count    int64  `json:"count"`
	Domains  int64  `json:"domains"`
	Problems int64  `json:"problems"`
	LastSeen int64  `json:"last_seen"`
}

type QueryRow struct {
	ID        int64  `json:"id"`
	TS        int64  `json:"ts"`
	Router    string `json:"router"`
	Client    string `json:"client"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Answer    string `json:"answer"`
	LatencyMS *int64 `json:"latency_ms"`
}

type Totals struct {
	Queries   int64            `json:"queries"`
	Clients   int64            `json:"clients"`
	Domains   int64            `json:"domains"`
	IPv6      int64            `json:"ipv6"`
	Status    map[string]int64 `json:"status"`
	LatencyMS *float64         `json:"latency_ms"`
}

type Summary struct {
	Since      int64        `json:"since"`
	Until      int64        `json:"until"`
	BucketSecs int64        `json:"bucket_secs"`
	Totals     Totals       `json:"totals"`
	Types      []Count      `json:"types"`
	Timeline   []Bucket     `json:"timeline"`
	TopDomains []DomainStat `json:"top_domains"`
	TopClients []ClientStat `json:"top_clients"`
	Problems   []DomainStat `json:"problems"`
}

// bucketFor picks a timeline resolution that keeps a range under ~300 points.
func bucketFor(window time.Duration) int64 {
	switch {
	case window <= 2*time.Hour:
		return 60
	case window <= 12*time.Hour:
		return 300
	case window <= 48*time.Hour:
		return 900
	case window <= 8*24*time.Hour:
		return 3600
	default:
		return 6 * 3600
	}
}

func (s *Store) Summary(ctx context.Context, window time.Duration, top int) (*Summary, error) {
	now := time.Now()
	f := Filter{Since: now.Add(-window)}
	out := &Summary{Since: f.Since.Unix(), Until: now.Unix(), BucketSecs: bucketFor(window)}
	err := parallel(
		func() (err error) { out.Totals, out.Types, err = s.totals(ctx, f); return },
		func() (err error) { out.Timeline, err = s.Timeline(ctx, f, out.BucketSecs, now); return },
		func() (err error) { out.TopDomains, err = s.Domains(ctx, f, top); return },
		func() (err error) { out.TopClients, err = s.Clients(ctx, f, top); return },
		func() (err error) {
			pf := f
			pf.Status = "problem"
			out.Problems, err = s.Domains(ctx, pf, top)
			return
		},
	)
	return out, err
}

// totals keeps each figure on the index that answers it cheapest: the time
// index for counts and latency, the client and domain indexes for distinct
// counts (grouping walks the index in order, so no temporary sort is needed).
func (s *Store) totals(ctx context.Context, f Filter) (Totals, []Count, error) {
	where, args := f.where()
	t := Totals{Status: map[string]int64{}}
	var latency sql.NullFloat64
	err := s.ro.QueryRowContext(ctx, `SELECT COUNT(*), AVG(latency_ms) FROM queries`+where, args...).Scan(&t.Queries, &latency)
	if err != nil {
		return t, nil, err
	}
	if latency.Valid {
		t.LatencyMS = &latency.Float64
	}
	err = s.ro.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(INSTR(client, ':') > 0), 0)
		FROM (SELECT client FROM queries`+where+` GROUP BY client)`, args...).Scan(&t.Clients, &t.IPv6)
	if err != nil {
		return t, nil, err
	}
	err = s.ro.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT 1 FROM queries`+where+` GROUP BY domain)`, args...).Scan(&t.Domains)
	if err != nil {
		return t, nil, err
	}

	// One pass over the time index yields both breakdowns.
	rows, err := s.ro.QueryContext(ctx, `SELECT type, status, COUNT(*) FROM queries`+where+` GROUP BY 1, 2`, args...)
	if err != nil {
		return t, nil, err
	}
	defer rows.Close()
	byType := map[string]int64{}
	for rows.Next() {
		var typ, status string
		var n int64
		if err := rows.Scan(&typ, &status, &n); err != nil {
			return t, nil, err
		}
		byType[typ] += n
		t.Status[status] += n
	}
	types := make([]Count, 0, len(byType))
	for k, n := range byType {
		types = append(types, Count{Key: k, Count: n})
	}
	sort.Slice(types, func(i, j int) bool {
		if types[i].Count != types[j].Count {
			return types[i].Count > types[j].Count
		}
		return types[i].Key < types[j].Key
	})
	return t, types, rows.Err()
}

// Timeline returns one bucket per interval from f.Since to now, zero-filled.
func (s *Store) Timeline(ctx context.Context, f Filter, bucket int64, now time.Time) ([]Bucket, error) {
	where, args := f.where()
	rows, err := s.ro.QueryContext(ctx, `SELECT (timestamp / ?) * ?, COUNT(*),
		SUM(status IN ('ok','nodata')), SUM(status = 'nxdomain'), SUM(status = 'blocked'),
		SUM(status IN ('servfail','error')), SUM(status = ''), AVG(latency_ms)
		FROM queries`+where+` GROUP BY 1 ORDER BY 1`, append([]any{bucket, bucket}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byTS := map[int64]Bucket{}
	for rows.Next() {
		var b Bucket
		var latency sql.NullFloat64
		if err := rows.Scan(&b.TS, &b.Total, &b.Resolved, &b.NXDomain, &b.Blocked, &b.Failed, &b.Unknown, &latency); err != nil {
			return nil, err
		}
		if latency.Valid {
			b.LatencyMS = &latency.Float64
		}
		byTS[b.TS] = b
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	first := (f.Since.Unix() / bucket) * bucket
	last := (now.Unix() / bucket) * bucket
	out := make([]Bucket, 0, (last-first)/bucket+1)
	for ts := first; ts <= last; ts += bucket {
		b, ok := byTS[ts]
		if !ok {
			b = Bucket{TS: ts}
		}
		out = append(out, b)
	}
	return out, nil
}

// Domains ranks domains by query count. Distinct clients and problem counts
// are added in a second pass for the returned rows only, since counting
// distinct values per group over the whole window is what makes large
// windows slow.
func (s *Store) Domains(ctx context.Context, f Filter, limit int) ([]DomainStat, error) {
	where, args := f.where()
	rows, err := s.ro.QueryContext(ctx, `SELECT domain, COUNT(*) AS n, MAX(timestamp)
		FROM queries`+where+` GROUP BY domain ORDER BY n DESC, domain LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	out := []DomainStat{}
	for rows.Next() {
		var d DomainStat
		if err := rows.Scan(&d.Domain, &d.Count, &d.LastSeen); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		err := s.ro.QueryRowContext(ctx, `SELECT COUNT(DISTINCT client), SUM(status IN `+problemStatuses+`)
			FROM queries`+where+` AND domain = ?`, append(args, out[i].Domain)...).Scan(&out[i].Clients, &out[i].Problems)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) Clients(ctx context.Context, f Filter, limit int) ([]ClientStat, error) {
	where, args := f.where()
	rows, err := s.ro.QueryContext(ctx, `SELECT client, COUNT(*) AS n, MAX(timestamp)
		FROM queries`+where+` GROUP BY client ORDER BY n DESC, client LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	out := []ClientStat{}
	for rows.Next() {
		var c ClientStat
		if err := rows.Scan(&c.Client, &c.Count, &c.LastSeen); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		c := &out[i]
		err := s.ro.QueryRowContext(ctx, `SELECT COUNT(DISTINCT domain), SUM(status IN `+problemStatuses+`)
			FROM queries`+where+` AND client = ?`, append(args, c.Client)...).Scan(&c.Domains, &c.Problems)
		if err != nil {
			return nil, err
		}
		err = s.ro.QueryRowContext(ctx, `SELECT name, mac FROM hosts WHERE ip = ?`, c.Client).Scan(&c.Name, &c.MAC)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	return out, nil
}

const queryColumns = `q.id, q.timestamp, q.router, q.client, COALESCE(h.name, ''), q.domain, q.type, q.status, q.answer, q.latency_ms`

func scanQuery(rows *sql.Rows) (QueryRow, error) {
	var r QueryRow
	var latency sql.NullInt64
	err := rows.Scan(&r.ID, &r.TS, &r.Router, &r.Client, &r.Name, &r.Domain, &r.Type, &r.Status, &r.Answer, &latency)
	if latency.Valid {
		r.LatencyMS = &latency.Int64
	}
	return r, err
}

// Queries returns one page of matching rows, newest first, and the total match count.
func (s *Store) Queries(ctx context.Context, f Filter, page, size int) ([]QueryRow, int64, error) {
	where, args := f.where()
	var total int64
	items := []QueryRow{}
	err := parallel(
		func() error {
			return s.ro.QueryRowContext(ctx, `SELECT COUNT(*) FROM queries`+where, args...).Scan(&total)
		},
		func() error {
			rows, err := s.ro.QueryContext(ctx, `SELECT `+queryColumns+` FROM (SELECT * FROM queries`+where+
				` ORDER BY timestamp DESC, id DESC LIMIT ? OFFSET ?) q LEFT JOIN hosts h ON h.ip = q.client ORDER BY q.timestamp DESC, q.id DESC`,
				append(args, size, (page-1)*size)...)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				r, err := scanQuery(rows)
				if err != nil {
					return err
				}
				items = append(items, r)
			}
			return rows.Err()
		},
	)
	return items, total, err
}

// ExportCSV streams every matching row, newest first.
func (s *Store) ExportCSV(ctx context.Context, f Filter, w io.Writer) error {
	where, args := f.where()
	rows, err := s.ro.QueryContext(ctx, `SELECT `+queryColumns+` FROM queries q LEFT JOIN hosts h ON h.ip = q.client`+
		where+` ORDER BY q.timestamp DESC, q.id DESC`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"time", "router", "client", "device", "domain", "type", "status", "answer", "latency_ms"}); err != nil {
		return err
	}
	for rows.Next() {
		r, err := scanQuery(rows)
		if err != nil {
			return err
		}
		latency := ""
		if r.LatencyMS != nil {
			latency = strconv.FormatInt(*r.LatencyMS, 10)
		}
		rec := []string{time.Unix(r.TS, 0).UTC().Format(time.RFC3339), r.Router, r.Client, r.Name, r.Domain, r.Type, r.Status, r.Answer, latency}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return errors.Join(rows.Err(), cw.Error())
}

func parallel(fns ...func() error) error {
	var wg sync.WaitGroup
	errs := make([]error, len(fns))
	for i, fn := range fns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = fn()
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}
