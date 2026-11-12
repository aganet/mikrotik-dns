package main

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

type Resolution struct {
	Status     string   `json:"status"` // ok, nxdomain, blocked, error
	Records    []string `json:"records"`
	Error      string   `json:"error,omitempty"`
	DurationMS int64    `json:"duration_ms"`
}

func newResolver(server string) *net.Resolver {
	if server == "" {
		return net.DefaultResolver
	}
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "53")
	}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 2 * time.Second}
			return d.DialContext(ctx, network, server)
		},
	}
}

// resolve performs a live lookup so a logged domain can be checked against
// the resolver's current answer.
func resolve(ctx context.Context, r *net.Resolver, name, qtype string) Resolution {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var records []string
	var err error
	switch strings.ToUpper(qtype) {
	case "AAAA":
		var ips []net.IPAddr
		ips, err = r.LookupIPAddr(ctx, name)
		for _, ip := range ips {
			if ip.IP.To4() == nil {
				records = append(records, ip.IP.String())
			}
		}
	case "CNAME":
		var cname string
		cname, err = r.LookupCNAME(ctx, name)
		if cname != "" && strings.TrimSuffix(cname, ".") != name {
			records = append(records, strings.TrimSuffix(cname, "."))
		}
	case "TXT":
		records, err = r.LookupTXT(ctx, name)
	case "MX":
		var mxs []*net.MX
		mxs, err = r.LookupMX(ctx, name)
		for _, mx := range mxs {
			records = append(records, strconv.Itoa(int(mx.Pref))+" "+strings.TrimSuffix(mx.Host, "."))
		}
	case "NS":
		var nss []*net.NS
		nss, err = r.LookupNS(ctx, name)
		for _, ns := range nss {
			records = append(records, strings.TrimSuffix(ns.Host, "."))
		}
	case "PTR":
		records, err = r.LookupAddr(ctx, name)
	default:
		var ips []net.IPAddr
		ips, err = r.LookupIPAddr(ctx, name)
		for _, ip := range ips {
			if ip.IP.To4() != nil || qtype == "" {
				records = append(records, ip.IP.String())
			}
		}
	}

	res := Resolution{Records: records, DurationMS: time.Since(start).Milliseconds()}
	if res.Records == nil {
		res.Records = []string{}
	}
	switch {
	case err != nil:
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			res.Status = statusNXDomain
		} else {
			res.Status = statusError
		}
		res.Error = err.Error()
	default:
		res.Status = statusOK
		for _, rec := range records {
			if rec == "0.0.0.0" || rec == "::" {
				res.Status = statusBlocked
			}
		}
	}
	return res
}
