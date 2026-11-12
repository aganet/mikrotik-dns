package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// How long a query waits for its "done query" line before being stored
	// without a response. RouterOS v6 never logs one, so this is also the
	// visibility delay on those routers.
	answerTimeout = 5 * time.Second
	flushInterval = 500 * time.Millisecond
	udpReadBuffer = 4 << 20
)

type pendingQuery struct {
	q        Query
	received time.Time
	done     bool
	answers  []string
}

// Ingester turns syslog datagrams into stored queries, pairing each query
// with the answer line that follows it.
type Ingester struct {
	store *Store
	log   *slog.Logger

	mu      sync.Mutex
	pending map[string]*pendingQuery
	routers map[string]time.Time

	received atomic.Int64
	queries  atomic.Int64
	answered atomic.Int64
	orphans  atomic.Int64
	leases   atomic.Int64
	unparsed atomic.Int64
}

type IngestStats struct {
	Received int64            `json:"received"`
	Queries  int64            `json:"queries"`
	Answered int64            `json:"answered"`
	Orphans  int64            `json:"orphan_answers"`
	Leases   int64            `json:"leases"`
	Unparsed int64            `json:"unparsed"`
	Pending  int              `json:"pending"`
	Routers  map[string]int64 `json:"routers"` // last message time per source address
}

func NewIngester(store *Store, log *slog.Logger) *Ingester {
	return &Ingester{
		store:   store,
		log:     log,
		pending: map[string]*pendingQuery{},
		routers: map[string]time.Time{},
	}
}

// Listen receives datagrams until ctx is cancelled.
func (in *Ingester) Listen(ctx context.Context, addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	if err := conn.SetReadBuffer(udpReadBuffer); err != nil {
		in.log.Warn("set udp read buffer", "err", err)
	}
	in.log.Info("listening for syslog", "addr", conn.LocalAddr())

	go in.flushLoop(ctx)
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buf := make([]byte, 65535)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, net.ErrClosed) {
				return err
			}
			in.log.Warn("udp read", "err", err)
			continue
		}
		now := time.Now()
		router := from.IP.String()
		for line := range strings.SplitSeq(string(buf[:n]), "\n") {
			if strings.TrimSpace(line) != "" {
				in.Handle(router, line, now)
			}
		}
	}
}

// Handle processes one log line from the given router.
func (in *Ingester) Handle(router, line string, now time.Time) {
	in.received.Add(1)
	in.mu.Lock()
	in.routers[router] = now
	in.mu.Unlock()

	ev := parseLine(line)
	switch ev.kind {
	case evQuery:
		in.queries.Add(1)
		client := ev.client
		if client == "" {
			client = router
		}
		p := &pendingQuery{
			q:        Query{TS: now, Router: router, Client: client, Domain: ev.domain, Type: ev.qtype, LatencyMS: -1},
			received: now,
		}
		key := router + "#" + ev.id
		in.mu.Lock()
		if old := in.pending[key]; old != nil {
			in.emit(old)
		}
		in.pending[key] = p
		in.mu.Unlock()
	case evDone:
		key := router + "#" + ev.id
		in.mu.Lock()
		p := in.pending[key]
		if p == nil {
			in.mu.Unlock()
			in.orphans.Add(1)
			return
		}
		if !p.done {
			p.done = true
			p.q.LatencyMS = now.Sub(p.received).Milliseconds()
			in.answered.Add(1)
		}
		// Later lines for the same id (one per record) only add answers, but
		// a blocked or error status always wins over a plain "ok".
		if p.q.Status == "" || p.q.Status == statusOK {
			p.q.Status = ev.status
		}
		if ev.domain != "" && ev.domain != p.q.Domain {
			p.answers = append(p.answers, ev.domain)
		}
		p.answers = append(p.answers, ev.answers...)
		in.mu.Unlock()
	case evLease:
		in.leases.Add(1)
		in.store.AddHost(Host{IP: ev.ip, MAC: ev.mac, Name: ev.name, Source: "dhcp"})
	default:
		in.unparsed.Add(1)
		in.log.Debug("unparsed line", "router", router, "line", line)
	}
}

// emit must be called with in.mu held.
func (in *Ingester) emit(p *pendingQuery) {
	p.q.Answer = strings.Join(p.answers, " ")
	in.store.Add(p.q)
}

func (in *Ingester) flushLoop(ctx context.Context) {
	t := time.NewTicker(flushInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			in.flush(time.Now(), false)
		case <-ctx.Done():
			return
		}
	}
}

// Close hands every pending query to the store, answered or not.
func (in *Ingester) Close() {
	in.flush(time.Now(), true)
}

func (in *Ingester) flush(now time.Time, all bool) {
	in.mu.Lock()
	defer in.mu.Unlock()
	for key, p := range in.pending {
		if all || p.done || now.Sub(p.received) > answerTimeout {
			in.emit(p)
			delete(in.pending, key)
		}
	}
}

func (in *Ingester) Stats() IngestStats {
	in.mu.Lock()
	routers := make(map[string]int64, len(in.routers))
	for r, t := range in.routers {
		routers[r] = t.Unix()
	}
	pending := len(in.pending)
	in.mu.Unlock()
	return IngestStats{
		Received: in.received.Load(),
		Queries:  in.queries.Load(),
		Answered: in.answered.Load(),
		Orphans:  in.orphans.Load(),
		Leases:   in.leases.Load(),
		Unparsed: in.unparsed.Load(),
		Pending:  pending,
		Routers:  routers,
	}
}
