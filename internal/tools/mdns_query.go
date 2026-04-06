// Package tools – mdns_query: low-level mDNS service discovery using miekg/dns.
package tools

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
)

const (
	mdnsIPv4Group = "224.0.0.251"
	mdnsPortNum   = 5353
)

// mdnsEntry holds information about a discovered mDNS service instance.
type mdnsEntry struct {
	Name string
	Host string
	IPs  []string
	Port int
	TXTs []string
}

// mdnsQueryServices sends a PTR query for serviceType over IPv4 mDNS multicast
// and collects responses for the given timeout duration.
// It joins the multicast group on all multicast-capable interfaces and uses
// SO_REUSEADDR/SO_REUSEPORT so it can co-exist with system mDNS daemons.
func mdnsQueryServices(serviceType string, timeout time.Duration) ([]*mdnsEntry, error) {
	lc := net.ListenConfig{Control: mdnsSocketControl}

	pc, err := lc.ListenPacket(context.Background(), "udp4", "0.0.0.0:5353")
	if err != nil {
		return nil, fmt.Errorf("mdns: bind :5353: %w", err)
	}
	defer pc.Close()

	// Join the multicast group on every multicast-capable interface.
	p4 := ipv4.NewPacketConn(pc)
	group := &net.UDPAddr{IP: net.ParseIP(mdnsIPv4Group)}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagMulticast != 0 {
			ifc := iface // shadow for loop variable safety
			_ = p4.JoinGroup(&ifc, group)
		}
	}

	// Build the PTR query.
	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn(serviceType), dns.TypePTR)
	q.RecursionDesired = false

	buf, err := q.Pack()
	if err != nil {
		return nil, fmt.Errorf("mdns: pack query: %w", err)
	}

	dst := &net.UDPAddr{IP: net.ParseIP(mdnsIPv4Group), Port: mdnsPortNum}
	if _, err := pc.WriteTo(buf, dst); err != nil {
		return nil, fmt.Errorf("mdns: send query: %w", err)
	}

	// Collect responses until timeout.
	if err := pc.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("mdns: set deadline: %w", err)
	}

	entries := make(map[string]*mdnsEntry) // keyed by full service instance name
	rbuf := make([]byte, 65535)

	for {
		n, _, err := pc.ReadFrom(rbuf)
		if err != nil {
			break // timeout or closed
		}

		var msg dns.Msg
		if err := msg.Unpack(rbuf[:n]); err != nil {
			continue
		}

		fqSvc := dns.Fqdn(serviceType)
		allRRs := append(msg.Answer, msg.Extra...)

		// Collect PTR records first to create or update entries.
		for _, rr := range msg.Answer {
			if ptr, ok := rr.(*dns.PTR); ok {
				if strings.EqualFold(ptr.Hdr.Name, fqSvc) {
					name := ptr.Ptr
					if _, exists := entries[name]; !exists {
						entries[name] = &mdnsEntry{Name: name}
					}
				}
			}
		}

		// Fill in SRV, TXT, A, AAAA from all sections.
		for _, rr := range allRRs {
			switch r := rr.(type) {
			case *dns.SRV:
				if e, ok := entries[r.Hdr.Name]; ok {
					e.Host = r.Target
					e.Port = int(r.Port)
				}
			case *dns.TXT:
				if e, ok := entries[r.Hdr.Name]; ok {
					e.TXTs = r.Txt
				}
			case *dns.A:
				for _, e := range entries {
					if strings.EqualFold(e.Host, r.Hdr.Name) {
						e.IPs = mdnsAppendUniq(e.IPs, r.A.String())
					}
				}
			case *dns.AAAA:
				for _, e := range entries {
					if strings.EqualFold(e.Host, r.Hdr.Name) {
						e.IPs = mdnsAppendUniq(e.IPs, r.AAAA.String())
					}
				}
			}
		}
	}

	result := make([]*mdnsEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, e)
	}
	return result, nil
}

func mdnsAppendUniq(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
