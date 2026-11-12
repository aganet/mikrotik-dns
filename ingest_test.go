package main

import (
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "test.db"), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func drain(s *Store) []Query {
	var out []Query
	for len(s.queries) > 0 {
		out = append(out, <-s.queries)
	}
	return out
}

func TestIngesterCorrelatesAnswers(t *testing.T) {
	s := testStore(t)
	in := NewIngester(s, slog.Default())
	t0 := time.Unix(1_700_000_000, 0)

	in.Handle("10.0.0.1", "dns query from 192.168.88.10: #1 example.com. A", t0)
	in.Handle("10.0.0.1", "dns query from 192.168.88.11: #2 ads.example.com. A", t0)
	in.Handle("10.0.0.1", "dns local query: #3 cloud.mikrotik.com. A", t0)
	in.Handle("10.0.0.1", "dns query from 192.168.88.12: #4 missing.example. AAAA", t0)
	in.Handle("10.0.0.1", "dns done query: #1 example.com 93.184.216.34", t0.Add(42*time.Millisecond))
	in.Handle("10.0.0.1", "dns done query: #1 example.com 93.184.216.35", t0.Add(43*time.Millisecond))
	in.Handle("10.0.0.1", "dns done query: #2 ads.example.com 0.0.0.0", t0.Add(time.Millisecond))
	in.Handle("10.0.0.1", "dns done query: #4 dns name does not exist", t0.Add(300*time.Millisecond))
	in.Handle("10.0.0.1", "dns done query: #99 dns server failure", t0)

	in.flush(t0.Add(time.Second), false)
	got := map[string]Query{}
	for _, q := range drain(s) {
		got[q.Domain] = q
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 completed queries, got %d: %+v", len(got), got)
	}
	if q := got["example.com"]; q.Status != statusOK || q.Answer != "93.184.216.34 93.184.216.35" || q.LatencyMS != 42 || q.Client != "192.168.88.10" {
		t.Errorf("example.com: %+v", q)
	}
	if q := got["ads.example.com"]; q.Status != statusBlocked || q.Answer != "0.0.0.0" {
		t.Errorf("ads.example.com: %+v", q)
	}
	if q := got["missing.example"]; q.Status != statusNXDomain || q.Type != "AAAA" {
		t.Errorf("missing.example: %+v", q)
	}

	// The unanswered local query is released only after the timeout.
	in.flush(t0.Add(answerTimeout), false)
	if n := len(drain(s)); n != 0 {
		t.Fatalf("query released before timeout: %d", n)
	}
	in.flush(t0.Add(answerTimeout+time.Second), false)
	rest := drain(s)
	if len(rest) != 1 || rest[0].Domain != "cloud.mikrotik.com" || rest[0].Status != "" || rest[0].Client != "10.0.0.1" || rest[0].LatencyMS != -1 {
		t.Errorf("timed out query: %+v", rest)
	}

	st := in.Stats()
	if st.Queries != 4 || st.Answered != 3 || st.Orphans != 1 || st.Unparsed != 0 || st.Pending != 0 {
		t.Errorf("stats: %+v", st)
	}
}

func TestIngesterReusedIDReleasesOld(t *testing.T) {
	s := testStore(t)
	in := NewIngester(s, slog.Default())
	t0 := time.Now()
	in.Handle("r", "dns query from 10.0.0.2: #7 a.example. A", t0)
	in.Handle("r", "dns query from 10.0.0.2: #7 b.example. A", t0.Add(time.Millisecond))
	got := drain(s)
	if len(got) != 1 || got[0].Domain != "a.example" || got[0].Status != "" {
		t.Fatalf("expected the first query to be released unanswered, got %+v", got)
	}
}

func TestIngesterLeaseAndUnparsed(t *testing.T) {
	s := testStore(t)
	in := NewIngester(s, slog.Default())
	in.Handle("r", "dhcp,info defconf assigned 192.168.88.37 for b0:e4:5c:27:ef:f2 Phone", time.Now())
	in.Handle("r", "system,info router rebooted", time.Now())
	select {
	case h := <-s.hosts:
		if h.IP != "192.168.88.37" || h.MAC != "B0:E4:5C:27:EF:F2" || h.Name != "Phone" || h.Source != "dhcp" {
			t.Errorf("host: %+v", h)
		}
	default:
		t.Fatal("lease not queued")
	}
	if st := in.Stats(); st.Leases != 1 || st.Unparsed != 1 || st.Received != 2 {
		t.Errorf("stats: %+v", st)
	}
}
