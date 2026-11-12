// mikrotik-dns receives RouterOS DNS and DHCP logs over syslog, stores them in
// SQLite and serves an analytics API together with the dashboard.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var version = "dev"

type Config struct {
	HTTPAddr     string
	SyslogAddr   string
	DatabasePath string
	WebDir       string
	Retention    time.Duration
	DNSServer    string
	LogLevel     slog.Level

	MikrotikURL      string
	MikrotikUser     string
	MikrotikPassword string
	MikrotikInsecure bool
	MikrotikPoll     time.Duration
}

func loadConfig() (Config, error) {
	cfg := Config{
		HTTPAddr:     getenv("HTTP_ADDR"),
		SyslogAddr:   getenv("SYSLOG_ADDR"),
		DatabasePath: getenv("DATABASE_PATH"),
		WebDir:       getenv("WEB_DIR"),
		DNSServer:    getenv("DNS_SERVER"),
		MikrotikURL:  getenv("MIKROTIK_URL"),
		MikrotikUser: getenv("MIKROTIK_USER"),
		Retention:    7 * 24 * time.Hour,
		MikrotikPoll: time.Minute,
	}
	cfg.MikrotikPassword = getenv("MIKROTIK_PASSWORD")
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":3000"
		if port := getenv("PORT"); port != "" {
			cfg.HTTPAddr = ":" + port
		}
	}
	if cfg.SyslogAddr == "" {
		cfg.SyslogAddr = ":5354"
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "./data/queries.db"
	}
	if s := getenv("RETENTION"); s != "" {
		d, err := parseDuration(s)
		if err != nil || d < time.Hour {
			return cfg, fmt.Errorf("RETENTION must be a duration of at least 1h, got %q", s)
		}
		cfg.Retention = d
	}
	if s := getenv("MIKROTIK_POLL_INTERVAL"); s != "" {
		d, err := parseDuration(s)
		if err != nil || d < 5*time.Second {
			return cfg, fmt.Errorf("MIKROTIK_POLL_INTERVAL must be a duration of at least 5s, got %q", s)
		}
		cfg.MikrotikPoll = d
	}
	if s := getenv("MIKROTIK_INSECURE"); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return cfg, fmt.Errorf("MIKROTIK_INSECURE must be a boolean, got %q", s)
		}
		cfg.MikrotikInsecure = b
	}
	if cfg.MikrotikURL != "" && cfg.MikrotikUser == "" {
		return cfg, errors.New("MIKROTIK_USER is required when MIKROTIK_URL is set")
	}
	if s := getenv("LOG_LEVEL"); s != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(s)); err != nil {
			return cfg, fmt.Errorf("LOG_LEVEL: %w", err)
		}
	}
	return cfg, nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(2)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)
	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := OpenStore(cfg.DatabasePath, log)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()
	log.Info("database ready", "path", cfg.DatabasePath, "retention", cfg.Retention, "version", version)

	// The store outlives the other components so their final flushes land.
	storeCtx, stopStore := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		store.Run(storeCtx, cfg.Retention)
	}()

	ingest := NewIngester(store, log)
	errc := make(chan error, 2)
	go func() { errc <- ingest.Listen(ctx, cfg.SyslogAddr) }()

	var poller *Poller
	if cfg.MikrotikURL != "" {
		poller = NewPoller(cfg, store, log)
		go poller.Run(ctx)
		log.Info("polling router", "url", cfg.MikrotikURL, "interval", cfg.MikrotikPoll)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           NewAPI(cfg, store, ingest, poller, log).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       time.Minute,
	}
	go func() {
		log.Info("http listening", "addr", cfg.HTTPAddr, "web", cfg.WebDir)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case runErr = <-errc:
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
	ingest.Close()
	stopStore()
	wg.Wait()
	return runErr
}
