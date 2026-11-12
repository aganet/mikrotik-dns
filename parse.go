package main

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// Response status values stored per query. Empty means no answer was logged.
const (
	statusOK       = "ok"
	statusNoData   = "nodata"
	statusNXDomain = "nxdomain"
	statusServFail = "servfail"
	statusBlocked  = "blocked"
	statusError    = "error"
)

type eventKind int

const (
	evUnknown eventKind = iota
	evQuery
	evDone
	evLease
)

// event is one parsed RouterOS log message. Only the fields for its kind are set.
type event struct {
	kind eventKind

	id     string // DNS request id shared by the query and its done lines
	client string // empty for the router's own ("local") queries
	domain string
	qtype  string

	status  string
	answers []string

	ip   string
	mac  string
	name string
}

// The patterns are unanchored at the start so any syslog envelope (PRI,
// timestamp, hostname, topics, user prefix) in front of the message is ignored.
var (
	reQuery = regexp.MustCompile(`(?:^|\s)(?:query from (\S+)|local query): #(\d+) (\S+?)\.? (\S+(?: \(\d+\))?)$`)
	reDone  = regexp.MustCompile(`(?:^|\s)done query: #(\d+)(?: (.*))?$`)
	reLease = regexp.MustCompile(`(?:^|\s)\S+ assigned (\S+) (?:for|to) ([0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5})(?: (.+))?$`)
)

var doneErrors = map[string]string{
	"name does not exist":                    statusNXDomain,
	"name exists, but no appropriate record": statusNoData,
	"server failure":                         statusServFail,
}

func parseLine(line string) event {
	line = strings.TrimRight(line, "\r\n\x00 ")
	if m := reQuery.FindStringSubmatch(line); m != nil {
		return event{kind: evQuery, client: m[1], id: m[2], domain: strings.ToLower(m[3]), qtype: normalizeType(m[4])}
	}
	if m := reDone.FindStringSubmatch(line); m != nil {
		ev := parseDone(m[2])
		ev.id = m[1]
		return ev
	}
	if m := reLease.FindStringSubmatch(line); m != nil {
		return event{kind: evLease, ip: m[1], mac: strings.ToUpper(m[2]), name: strings.TrimSpace(m[3])}
	}
	return event{}
}

// parseDone interprets the text after "done query: #id", which is either
// "<name> <answer>..." or an error description starting with "dns ".
func parseDone(rest string) event {
	ev := event{kind: evDone}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		ev.status = statusOK
		return ev
	}
	if fields[0] == "dns" && len(fields) > 1 && !isAddress(fields[1]) {
		msg := strings.Join(fields[1:], " ")
		ev.status = doneErrors[msg]
		if ev.status == "" {
			ev.status = statusError
		}
		ev.answers = []string{msg}
		return ev
	}
	ev.domain = strings.ToLower(strings.TrimSuffix(fields[0], "."))
	ev.status = statusOK
	for _, a := range fields[1:] {
		a = strings.TrimSuffix(a, ".")
		if a == "0.0.0.0" || a == "::" {
			ev.status = statusBlocked
		}
		ev.answers = append(ev.answers, a)
	}
	return ev
}

func isAddress(s string) bool {
	_, err := netip.ParseAddr(s)
	return err == nil
}

// normalizeType maps "UNKNOWN (65)" style types to their RFC names.
func normalizeType(t string) string {
	if !strings.HasPrefix(t, "UNKNOWN") {
		return t
	}
	open, close := strings.IndexByte(t, '('), strings.IndexByte(t, ')')
	if open < 0 || close <= open {
		return "UNKNOWN"
	}
	n, err := strconv.Atoi(t[open+1 : close])
	if err != nil {
		return "UNKNOWN"
	}
	if name, ok := dnsTypeNames[n]; ok {
		return name
	}
	return "TYPE" + strconv.Itoa(n)
}

var dnsTypeNames = map[int]string{
	1: "A", 2: "NS", 5: "CNAME", 6: "SOA", 12: "PTR", 13: "HINFO", 15: "MX", 16: "TXT",
	17: "RP", 18: "AFSDB", 24: "SIG", 25: "KEY", 28: "AAAA", 29: "LOC", 33: "SRV",
	35: "NAPTR", 36: "KX", 37: "CERT", 39: "DNAME", 41: "OPT", 42: "APL", 43: "DS",
	44: "SSHFP", 45: "IPSECKEY", 46: "RRSIG", 47: "NSEC", 48: "DNSKEY", 49: "DHCID",
	50: "NSEC3", 51: "NSEC3PARAM", 52: "TLSA", 53: "SMIMEA", 55: "HIP", 59: "CDS",
	60: "CDNSKEY", 61: "OPENPGPKEY", 62: "CSYNC", 63: "ZONEMD", 64: "SVCB", 65: "HTTPS",
	99: "SPF", 108: "EUI48", 109: "EUI64", 249: "TKEY", 250: "TSIG", 252: "AXFR",
	255: "ANY", 256: "URI", 257: "CAA", 32768: "TA", 32769: "DLV",
}
