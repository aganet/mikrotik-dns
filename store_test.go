package main

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMigrateFromOriginalSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE queries(id INTEGER PRIMARY KEY, timestamp INTEGER, client TEXT, domain TEXT, type TEXT);
		INSERT INTO queries(timestamp, client, domain, type) VALUES(?, '10.0.0.9', 'legacy.example', 'A')`, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := OpenStore(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var version int
	if err := s.rw.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != schemaLatest {
		t.Fatalf("user_version = %d, err %v", version, err)
	}
	rows, _, err := s.Queries(context.Background(), Filter{Since: time.Now().Add(-time.Hour)}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Domain != "legacy.example" || rows[0].Status != "" || rows[0].LatencyMS != nil {
		t.Fatalf("legacy row: %+v", rows)
	}
	// Reopening an already migrated database is a no-op.
	s.Close()
	if s, err = OpenStore(path, slog.Default()); err != nil {
		t.Fatal(err)
	}
}

func TestStoreReads(t *testing.T) {
	s := testStore(t)
	now := time.Now().Truncate(time.Second)
	batch := []Query{
		{TS: now.Add(-10 * time.Minute), Router: "r", Client: "10.0.0.1", Domain: "example.com", Type: "A", Status: statusOK, Answer: "1.1.1.1", LatencyMS: 10},
		{TS: now.Add(-9 * time.Minute), Router: "r", Client: "10.0.0.1", Domain: "example.com", Type: "AAAA", Status: statusNoData, LatencyMS: 30},
		{TS: now.Add(-8 * time.Minute), Router: "r", Client: "10.0.0.2", Domain: "ads.example", Type: "A", Status: statusBlocked, Answer: "0.0.0.0", LatencyMS: 1},
		{TS: now.Add(-7 * time.Minute), Router: "r", Client: "fd00::2", Domain: "nope.example", Type: "A", Status: statusNXDomain, LatencyMS: 200},
		{TS: now.Add(-6 * time.Minute), Router: "r", Client: "10.0.0.2", Domain: "under_score.example", Type: "A", LatencyMS: -1},
		{TS: now.Add(-3 * time.Hour), Router: "r", Client: "10.0.0.3", Domain: "old.example", Type: "A", Status: statusOK, LatencyMS: 5},
	}
	s.insert(batch)
	s.upsertHosts([]Host{
		{IP: "10.0.0.1", MAC: "AA:AA:AA:AA:AA:AA", Name: "laptop", Source: "dhcp"},
		{IP: "fd00::2", MAC: "BB:BB:BB:BB:BB:BB", Source: "nd"},
		{IP: "10.0.0.2", MAC: "BB:BB:BB:BB:BB:BB", Name: "phone", Source: "dhcp"},
		{IP: "10.0.0.1", MAC: "AA:AA:AA:AA:AA:AA", Source: "arp"},
	})
	ctx := context.Background()

	sum, err := s.Summary(ctx, time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	tot := sum.Totals
	if tot.Queries != 5 || tot.Clients != 3 || tot.Domains != 4 || tot.IPv6 != 1 {
		t.Errorf("totals: %+v", tot)
	}
	if tot.Status[statusOK] != 1 || tot.Status[statusBlocked] != 1 || tot.Status[""] != 1 {
		t.Errorf("status counts: %+v", tot.Status)
	}
	if tot.LatencyMS == nil || *tot.LatencyMS != 60.25 {
		t.Errorf("latency: %v", tot.LatencyMS)
	}
	if len(sum.Types) != 2 || sum.Types[0].Key != "A" || sum.Types[0].Count != 4 {
		t.Errorf("types: %+v", sum.Types)
	}
	if len(sum.TopDomains) != 4 || sum.TopDomains[0].Domain != "example.com" || sum.TopDomains[0].Count != 2 {
		t.Errorf("top domains: %+v", sum.TopDomains)
	}
	if len(sum.Problems) != 2 {
		t.Errorf("problems: %+v", sum.Problems)
	}
	if len(sum.TopClients) != 3 || sum.TopClients[0].Client != "10.0.0.1" || sum.TopClients[0].Name != "laptop" {
		t.Errorf("top clients: %+v", sum.TopClients)
	}
	for _, c := range sum.TopClients {
		if c.Client == "fd00::2" && c.Name != "phone" {
			t.Errorf("ipv6 client should inherit the lease name via MAC: %+v", c)
		}
	}
	if sum.BucketSecs != 60 || len(sum.Timeline) != 61 {
		t.Errorf("timeline: bucket %d, %d points", sum.BucketSecs, len(sum.Timeline))
	}
	var total int64
	for _, b := range sum.Timeline {
		total += b.Total
	}
	if total != 5 {
		t.Errorf("timeline total %d", total)
	}

	rows, n, err := s.Queries(ctx, Filter{Since: now.Add(-time.Hour), Search: "under_"}, 1, 10)
	if err != nil || n != 1 || len(rows) != 1 || rows[0].Domain != "under_score.example" {
		t.Errorf("search with wildcard char: n=%d rows=%+v err=%v", n, rows, err)
	}
	rows, n, err = s.Queries(ctx, Filter{Since: now.Add(-time.Hour), Status: "problem"}, 1, 10)
	if err != nil || n != 2 || len(rows) != 2 {
		t.Errorf("problem filter: n=%d rows=%+v err=%v", n, rows, err)
	}
	rows, n, err = s.Queries(ctx, Filter{Since: now.Add(-time.Hour), Client: "10.0.0.1"}, 2, 1)
	if err != nil || n != 2 || len(rows) != 1 || rows[0].Type != "A" || rows[0].Name != "laptop" {
		t.Errorf("paged client filter: n=%d rows=%+v err=%v", n, rows, err)
	}

	var buf bytes.Buffer
	if err := s.ExportCSV(ctx, Filter{Since: now.Add(-time.Hour), Domain: "example.com"}, &buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[1], ",laptop,example.com,AAAA,nodata,,30") {
		t.Errorf("csv:\n%s", buf.String())
	}

	s.purge(2 * time.Hour)
	if _, n, err := s.Queries(ctx, Filter{Since: now.Add(-24 * time.Hour)}, 1, 10); err != nil || n != 5 {
		t.Errorf("after purge: n=%d err=%v", n, err)
	}
}
