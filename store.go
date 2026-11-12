package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Query is one DNS request as logged by the router, with its answer when the
// router logged one.
type Query struct {
	TS        time.Time
	Router    string
	Client    string
	Domain    string
	Type      string
	Status    string
	Answer    string
	LatencyMS int64 // -1 when unknown
}

// Host maps a client address to a device identity.
type Host struct {
	IP     string
	MAC    string
	Name   string
	Source string
}

type Store struct {
	path    string
	rw      *sql.DB // single connection; all writes go through it
	ro      *sql.DB
	queries chan Query
	hosts   chan Host
	dropped atomic.Int64
	written atomic.Int64
	log     *slog.Logger
}

const (
	batchSize    = 1000
	flushEvery   = time.Second
	purgeEvery   = 10 * time.Minute
	purgeChunk   = 20000
	queueSize    = 16384
	schemaLatest = 2
)

var migrations = [...][]string{
	// 0: the original schema, kept so existing databases upgrade in place.
	{`CREATE TABLE IF NOT EXISTS queries (
		id INTEGER PRIMARY KEY,
		timestamp INTEGER NOT NULL,
		client TEXT NOT NULL,
		domain TEXT NOT NULL,
		type TEXT NOT NULL)`},
	// 1: response data and the device table.
	{
		`ALTER TABLE queries ADD COLUMN router TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE queries ADD COLUMN status TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE queries ADD COLUMN answer TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE queries ADD COLUMN latency_ms INTEGER`,
		`CREATE TABLE IF NOT EXISTS hosts (
			ip TEXT PRIMARY KEY,
			mac TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			updated INTEGER NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS hosts_mac ON hosts(mac)`,
	},
	// 2: covering indexes so every dashboard aggregate is answered from an
	// index without touching table rows.
	{
		`CREATE INDEX IF NOT EXISTS queries_ts ON queries(timestamp, status, type, latency_ms)`,
		`CREATE INDEX IF NOT EXISTS queries_client ON queries(client, timestamp, domain, status)`,
		`CREATE INDEX IF NOT EXISTS queries_domain ON queries(domain, timestamp, client, status)`,
		`ANALYZE`,
	},
}

func OpenStore(path string, log *slog.Logger) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_cache_size=-32000"
	rw, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	rw.SetMaxOpenConns(1)
	if err := migrate(rw); err != nil {
		rw.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	ro, err := sql.Open("sqlite3", dsn+"&_query_only=true")
	if err != nil {
		rw.Close()
		return nil, err
	}
	ro.SetMaxOpenConns(runtime.NumCPU())
	return &Store{
		path:    path,
		rw:      rw,
		ro:      ro,
		queries: make(chan Query, queueSize),
		hosts:   make(chan Host, 1024),
		log:     log,
	}, nil
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > schemaLatest {
		return fmt.Errorf("database schema %d is newer than this binary (%d)", version, schemaLatest)
	}
	// A pre-versioning database already has the v0 table but reports 0.
	var hasTable bool
	if err := db.QueryRow(`SELECT COUNT(*) > 0 FROM sqlite_master WHERE type='table' AND name='queries'`).Scan(&hasTable); err != nil {
		return err
	}
	start := version
	if version == 0 && !hasTable {
		start = -1
	}
	for v := start + 1; v <= schemaLatest; v++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		for _, stmt := range migrations[v] {
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("schema v%d: %w", v, err)
			}
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error {
	s.ro.Close()
	return s.rw.Close()
}

// Add queues a query for the writer. It never blocks the ingest path; when the
// queue is full the record is dropped and counted.
func (s *Store) Add(q Query) {
	select {
	case s.queries <- q:
	default:
		s.dropped.Add(1)
	}
}

func (s *Store) AddHost(h Host) {
	select {
	case s.hosts <- h:
	default:
		s.dropped.Add(1)
	}
}

// Run owns all writes: batched inserts, host upserts and retention purges.
// It returns after ctx is cancelled and the queues are flushed.
func (s *Store) Run(ctx context.Context, retention time.Duration) {
	flush := time.NewTicker(flushEvery)
	purge := time.NewTicker(purgeEvery)
	defer flush.Stop()
	defer purge.Stop()

	s.purge(retention)
	batch := make([]Query, 0, batchSize)
	hosts := make([]Host, 0, 64)
	for {
		select {
		case q := <-s.queries:
			batch = append(batch, q)
			if len(batch) >= batchSize {
				s.insert(batch)
				batch = batch[:0]
			}
		case h := <-s.hosts:
			hosts = append(hosts, h)
		case <-flush.C:
			s.insert(batch)
			batch = batch[:0]
			s.upsertHosts(hosts)
			hosts = hosts[:0]
		case <-purge.C:
			s.purge(retention)
		case <-ctx.Done():
			for len(s.queries) > 0 {
				batch = append(batch, <-s.queries)
			}
			for len(s.hosts) > 0 {
				hosts = append(hosts, <-s.hosts)
			}
			s.insert(batch)
			s.upsertHosts(hosts)
			return
		}
	}
}

func (s *Store) insert(batch []Query) {
	if len(batch) == 0 {
		return
	}
	tx, err := s.rw.Begin()
	if err != nil {
		s.log.Error("begin insert", "err", err)
		return
	}
	stmt, err := tx.Prepare(`INSERT INTO queries(timestamp, router, client, domain, type, status, answer, latency_ms) VALUES(?,?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		s.log.Error("prepare insert", "err", err)
		return
	}
	for _, q := range batch {
		var latency any
		if q.LatencyMS >= 0 {
			latency = q.LatencyMS
		}
		if _, err := stmt.Exec(q.TS.Unix(), q.Router, q.Client, q.Domain, q.Type, q.Status, q.Answer, latency); err != nil {
			s.log.Error("insert", "err", err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		s.log.Error("commit insert", "err", err)
		return
	}
	s.written.Add(int64(len(batch)))
}

func (s *Store) upsertHosts(hosts []Host) {
	if len(hosts) == 0 {
		return
	}
	tx, err := s.rw.Begin()
	if err != nil {
		s.log.Error("begin hosts", "err", err)
		return
	}
	now := time.Now().Unix()
	for _, h := range hosts {
		_, err := tx.Exec(`INSERT INTO hosts(ip, mac, name, source, updated) VALUES(?,?,?,?,?)
			ON CONFLICT(ip) DO UPDATE SET
				mac = CASE WHEN excluded.mac <> '' THEN excluded.mac ELSE hosts.mac END,
				name = CASE WHEN excluded.name <> '' THEN excluded.name ELSE hosts.name END,
				source = excluded.source,
				updated = excluded.updated`,
			h.IP, h.MAC, h.Name, h.Source, now)
		if err != nil {
			s.log.Error("upsert host", "ip", h.IP, "err", err)
		}
	}
	// Addresses known only by MAC (IPv6 neighbours, ARP) inherit the name of
	// the DHCP lease on the same hardware.
	_, err = tx.Exec(`UPDATE hosts SET name = (
			SELECT h2.name FROM hosts h2 WHERE h2.mac = hosts.mac AND h2.name <> '' ORDER BY h2.updated DESC LIMIT 1)
		WHERE name = '' AND mac <> ''
		  AND EXISTS (SELECT 1 FROM hosts h3 WHERE h3.mac = hosts.mac AND h3.name <> '')`)
	if err != nil {
		s.log.Error("propagate host names", "err", err)
	}
	if err := tx.Commit(); err != nil {
		s.log.Error("commit hosts", "err", err)
	}
}

func (s *Store) purge(retention time.Duration) {
	cutoff := time.Now().Add(-retention).Unix()
	var total int64
	for {
		res, err := s.rw.Exec(`DELETE FROM queries WHERE id IN (SELECT id FROM queries WHERE timestamp < ? LIMIT ?)`, cutoff, purgeChunk)
		if err != nil {
			s.log.Error("purge", "err", err)
			return
		}
		n, _ := res.RowsAffected()
		total += n
		if n < purgeChunk {
			break
		}
	}
	if total > 0 {
		s.log.Info("purged old queries", "rows", total)
	}
	if _, err := s.rw.Exec(`PRAGMA optimize`); err != nil {
		s.log.Warn("optimize", "err", err)
	}
}

// Size reports the on-disk footprint including the WAL.
func (s *Store) Size() int64 {
	var total int64
	for _, suffix := range []string{"", "-wal"} {
		if fi, err := os.Stat(s.path + suffix); err == nil {
			total += fi.Size()
		}
	}
	return total
}

func (s *Store) Stats() (written, dropped int64) {
	return s.written.Load(), s.dropped.Load()
}

// escapeLike makes a user string safe for a LIKE pattern with ESCAPE '\'.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
