package main

import (
	"reflect"
	"testing"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want event
	}{
		{
			name: "bare query",
			line: "dns query from 192.168.88.10: #123 Example.COM. A",
			want: event{kind: evQuery, client: "192.168.88.10", id: "123", domain: "example.com", qtype: "A"},
		},
		{
			name: "iso timestamp prefix",
			line: "2025-01-15 14:23:45 dns query from 192.168.1.100: #12345 google.com. AAAA",
			want: event{kind: evQuery, client: "192.168.1.100", id: "12345", domain: "google.com", qtype: "AAAA"},
		},
		{
			name: "bsd syslog envelope with prefix",
			line: "<30>Nov 12 13:34:00 router MT-DNS: query from 10.0.0.5: #8 cdn.example.net. HTTPS",
			want: event{kind: evQuery, client: "10.0.0.5", id: "8", domain: "cdn.example.net", qtype: "HTTPS"},
		},
		{
			name: "unknown numeric type",
			line: "dns query from 10.0.0.5: #9 example.org. UNKNOWN (65)",
			want: event{kind: evQuery, client: "10.0.0.5", id: "9", domain: "example.org", qtype: "HTTPS"},
		},
		{
			name: "unmapped numeric type",
			line: "dns query from 10.0.0.5: #9 example.org. UNKNOWN (12345)",
			want: event{kind: evQuery, client: "10.0.0.5", id: "9", domain: "example.org", qtype: "TYPE12345"},
		},
		{
			name: "ipv6 client",
			line: "dns query from fd00::1:2: #77 example.org. A",
			want: event{kind: evQuery, client: "fd00::1:2", id: "77", domain: "example.org", qtype: "A"},
		},
		{
			name: "local query",
			line: "dns local query: #33347 cloud.mikrotik.com. A",
			want: event{kind: evQuery, id: "33347", domain: "cloud.mikrotik.com", qtype: "A"},
		},
		{
			name: "done with answer",
			line: "dns done query: #33347 cloud.mikrotik.com 91.188.51.139",
			want: event{kind: evDone, id: "33347", domain: "cloud.mikrotik.com", status: statusOK, answers: []string{"91.188.51.139"}},
		},
		{
			name: "done with trailing dot and several answers",
			line: "dns done query: #239527 www.example.gov. 192.0.66.168 192.0.66.169",
			want: event{kind: evDone, id: "239527", domain: "www.example.gov", status: statusOK, answers: []string{"192.0.66.168", "192.0.66.169"}},
		},
		{
			name: "done blocked",
			line: "dns done query: #5 ads.example.com 0.0.0.0",
			want: event{kind: evDone, id: "5", domain: "ads.example.com", status: statusBlocked, answers: []string{"0.0.0.0"}},
		},
		{
			name: "done cname only",
			line: "dns done query: #3129141 time.g.aaplimg.com.",
			want: event{kind: evDone, id: "3129141", domain: "time.g.aaplimg.com", status: statusOK},
		},
		{
			name: "done nxdomain",
			line: "dns done query: #25 dns name does not exist",
			want: event{kind: evDone, id: "25", status: statusNXDomain, answers: []string{"name does not exist"}},
		},
		{
			name: "done nodata",
			line: "dns done query: #26 dns name exists, but no appropriate record",
			want: event{kind: evDone, id: "26", status: statusNoData, answers: []string{"name exists, but no appropriate record"}},
		},
		{
			name: "done servfail",
			line: "dns done query: #27 dns server failure",
			want: event{kind: evDone, id: "27", status: statusServFail, answers: []string{"server failure"}},
		},
		{
			name: "done unknown error",
			line: "dns done query: #28 dns query refused",
			want: event{kind: evDone, id: "28", status: statusError, answers: []string{"query refused"}},
		},
		{
			name: "done for a domain literally named dns",
			line: "dns done query: #29 dns 10.0.0.1",
			want: event{kind: evDone, id: "29", domain: "dns", status: statusOK, answers: []string{"10.0.0.1"}},
		},
		{
			name: "dhcp assigned with hostname",
			line: "dhcp,info defconf assigned 192.168.88.37 for B0:E4:5C:27:EF:F2 Samsung Galaxy",
			want: event{kind: evLease, ip: "192.168.88.37", mac: "B0:E4:5C:27:EF:F2", name: "Samsung Galaxy"},
		},
		{
			name: "dhcp assigned legacy wording",
			line: "dhcp,info dhcp1 assigned 192.168.88.10 to aa:bb:cc:dd:ee:01",
			want: event{kind: evLease, ip: "192.168.88.10", mac: "AA:BB:CC:DD:EE:01"},
		},
		{
			name: "dhcp deassigned is ignored",
			line: "dhcp,info defconf deassigned 192.168.88.37 for B0:E4:5C:27:EF:F2 Samsung",
			want: event{},
		},
		{
			name: "packet topic is ignored",
			line: "dns,packet <example.com:A:93.184.216.34:ttl=3600>",
			want: event{},
		},
		{
			name: "crlf is tolerated",
			line: "dns query from 10.0.0.5: #1 example.com. A\r\n",
			want: event{kind: evQuery, client: "10.0.0.5", id: "1", domain: "example.com", qtype: "A"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLine(tt.line); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLine(%q)\n got %+v\nwant %+v", tt.line, got, tt.want)
			}
		})
	}
}
