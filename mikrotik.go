package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// RouterInfo is the latest snapshot taken from the RouterOS REST API.
type RouterInfo struct {
	UpdatedAt   int64    `json:"updated_at"`
	Error       string   `json:"error,omitempty"`
	Identity    string   `json:"identity"`
	Version     string   `json:"version"`
	Board       string   `json:"board"`
	Uptime      string   `json:"uptime"`
	CPULoad     int64    `json:"cpu_load"`
	FreeMemory  int64    `json:"free_memory"`
	TotalMemory int64    `json:"total_memory"`
	DNSServers  string   `json:"dns_servers"`
	DNSDynamic  string   `json:"dns_dynamic_servers"`
	CacheUsed   int64    `json:"cache_used_kib"`
	CacheSize   int64    `json:"cache_size_kib"`
	Adlists     []Adlist `json:"adlists"`
	Leases      int      `json:"leases"`
	Hosts       int      `json:"hosts"`
}

type Adlist struct {
	URL     string `json:"url"`
	Matches int64  `json:"matches"`
	Names   int64  `json:"names"`
}

// Poller enriches client addresses with DHCP, ARP and IPv6 neighbour data and
// keeps a router status snapshot. It is optional; without credentials the
// dashboard still works from the log stream alone.
type Poller struct {
	base     string
	user     string
	password string
	interval time.Duration
	client   *http.Client
	store    *Store
	log      *slog.Logger
	info     atomic.Pointer[RouterInfo]
}

func NewPoller(cfg Config, store *Store, log *slog.Logger) *Poller {
	return &Poller{
		base:     strings.TrimRight(cfg.MikrotikURL, "/"),
		user:     cfg.MikrotikUser,
		password: cfg.MikrotikPassword,
		interval: cfg.MikrotikPoll,
		client: &http.Client{
			Timeout:   15 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.MikrotikInsecure}},
		},
		store: store,
		log:   log,
	}
}

func (p *Poller) Info() *RouterInfo { return p.info.Load() }

func (p *Poller) Run(ctx context.Context) {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		p.poll(ctx)
		select {
		case <-t.C:
		case <-ctx.Done():
			return
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	info := RouterInfo{UpdatedAt: time.Now().Unix()}
	if err := p.pollSystem(ctx, &info); err != nil {
		info.Error = err.Error()
		p.log.Warn("router poll", "err", err)
	}
	if err := p.pollHosts(ctx, &info); err != nil {
		info.Error = err.Error()
		p.log.Warn("host poll", "err", err)
	}
	p.info.Store(&info)
}

func (p *Poller) pollSystem(ctx context.Context, info *RouterInfo) error {
	var identity struct {
		Name string `json:"name"`
	}
	if err := p.get(ctx, "/system/identity", &identity); err != nil {
		return err
	}
	info.Identity = identity.Name

	var res struct {
		Version     string `json:"version"`
		Board       string `json:"board-name"`
		Uptime      string `json:"uptime"`
		CPULoad     string `json:"cpu-load"`
		FreeMemory  string `json:"free-memory"`
		TotalMemory string `json:"total-memory"`
	}
	if err := p.get(ctx, "/system/resource", &res); err != nil {
		return err
	}
	info.Version, info.Board, info.Uptime = res.Version, res.Board, res.Uptime
	info.CPULoad = atoi(res.CPULoad)
	info.FreeMemory = atoi(res.FreeMemory)
	info.TotalMemory = atoi(res.TotalMemory)

	var dns struct {
		Servers   string `json:"servers"`
		Dynamic   string `json:"dynamic-servers"`
		CacheUsed string `json:"cache-used"`
		CacheSize string `json:"cache-size"`
	}
	if err := p.get(ctx, "/ip/dns", &dns); err != nil {
		return err
	}
	info.DNSServers, info.DNSDynamic = dns.Servers, dns.Dynamic
	info.CacheUsed, info.CacheSize = atoi(dns.CacheUsed), atoi(dns.CacheSize)

	// Adlists exist from RouterOS 7.15; older versions answer 404.
	var adlists []struct {
		URL     string `json:"url"`
		Matches string `json:"match-count"`
		Names   string `json:"name-count"`
	}
	if err := p.get(ctx, "/ip/dns/adlist", &adlists); err == nil {
		info.Adlists = make([]Adlist, 0, len(adlists))
		for _, a := range adlists {
			info.Adlists = append(info.Adlists, Adlist{URL: a.URL, Matches: atoi(a.Matches), Names: atoi(a.Names)})
		}
	}
	return nil
}

func (p *Poller) pollHosts(ctx context.Context, info *RouterInfo) error {
	var leases []struct {
		Address string `json:"address"`
		MAC     string `json:"mac-address"`
		Host    string `json:"host-name"`
		Comment string `json:"comment"`
	}
	if err := p.get(ctx, "/ip/dhcp-server/lease", &leases); err != nil {
		return err
	}
	var hosts []Host
	for _, l := range leases {
		if l.Address == "" || l.MAC == "" {
			continue
		}
		name := l.Comment
		if name == "" {
			name = l.Host
		}
		hosts = append(hosts, Host{IP: l.Address, MAC: strings.ToUpper(l.MAC), Name: name, Source: "dhcp"})
	}
	info.Leases = len(hosts)

	var neighbours []struct {
		Address string `json:"address"`
		MAC     string `json:"mac-address"`
	}
	if err := p.get(ctx, "/ip/arp", &neighbours); err != nil {
		return err
	}
	for _, n := range neighbours {
		if n.Address != "" && n.MAC != "" {
			hosts = append(hosts, Host{IP: n.Address, MAC: strings.ToUpper(n.MAC), Source: "arp"})
		}
	}
	neighbours = neighbours[:0]
	if err := p.get(ctx, "/ipv6/neighbor", &neighbours); err != nil {
		return err
	}
	for _, n := range neighbours {
		if n.Address != "" && n.MAC != "" {
			hosts = append(hosts, Host{IP: n.Address, MAC: strings.ToUpper(n.MAC), Source: "nd"})
		}
	}
	info.Hosts = len(hosts)
	for _, h := range hosts {
		p.store.AddHost(h)
	}
	return nil
}

func (p *Poller) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base+"/rest"+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.user, p.password)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// atoi reads the numeric strings RouterOS returns, tolerating units like "KiB".
func atoi(s string) int64 {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && (s[end] >= '0' && s[end] <= '9') {
		end++
	}
	n, _ := strconv.ParseInt(s[:end], 10, 64)
	return n
}
