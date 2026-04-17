// Package tools – mdns_query: low-level mDNS service discovery using miekg/dns.
package tools

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"log/slog"

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
// It joins the multicast group on the default-route LAN interface and uses
// SO_REUSEADDR/SO_REUSEPORT so it can co-exist with system mDNS daemons.
func mdnsQueryServices(serviceType string, timeout time.Duration, logger *slog.Logger) ([]*mdnsEntry, error) {
	lc := net.ListenConfig{Control: mdnsSocketControl}

	pc, err := lc.ListenPacket(context.Background(), "udp4", "0.0.0.0:5353")
	if err != nil {
		return nil, fmt.Errorf("mdns: bind :5353: %w", err)
	}
	defer pc.Close()

	// Join the multicast group on the interface that carries the default route
	// (i.e. the LAN interface), not on every multicast-capable interface.
	// Joining on 30+ Docker veth/bridge interfaces can confuse the kernel's
	// multicast routing, especially when Docker networks overlap the LAN subnet.
	p4 := ipv4.NewPacketConn(pc)
	group := &net.UDPAddr{IP: net.ParseIP(mdnsIPv4Group)}

	iface, err := defaultRouteInterface()
	if err != nil {
		logger.Info("mdns: no default route interface, will attempt to join anyway", "error", err)
	} else {
		logger.Info("mdns: joining multicast group on interface", "interface", iface.Name, "index", iface.Index)
		if err := p4.JoinGroup(iface, group); err != nil {
			logger.Info("mdns: JoinGroup failed", "error", err)
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
	packetCount := 0

	for {
		n, _, err := pc.ReadFrom(rbuf)
		if err != nil {
			logger.Info("mdns: read loop ended", "packet_count", packetCount, "error", err)
			break // timeout or closed
		}
		packetCount++
		logger.Info("mdns: received packet", "size", n, "packet_num", packetCount, "hex", fmt.Sprintf("%x", rbuf[:n]))

		var msg dns.Msg
		if err := msg.Unpack(rbuf[:n]); err != nil {
			logger.Info("mdns: unpack failed", "error", err)
			continue
		}

		logger.Info("mdns: dns message received", "questions", len(msg.Question), "answers", len(msg.Answer), "extra", len(msg.Extra))

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
	logger.Info("mdns: discovery complete", "entries_found", len(result), "packets_received", packetCount)
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

// defaultRouteInterface returns the network interface that carries the default
// route (i.e. the primary LAN interface). This avoids joining multicast on Docker
// veth/bridge interfaces which can cause kernel multicast routing confusion.
func defaultRouteInterface() (*net.Interface, error) {
	// Find the interface with the default route by checking routing tables.
	// The default route interface is typically the one with 0.0.0.0/0 or via a gateway.
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	// Try to find interface via routing - look for default route in system
	// Use dial to trigger routing table lookup and get the interface used
	conn, err := net.Dial("udp4", "1.1.1.1:53")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.Equal(localAddr.IP) {
				return &iface, nil
			}
		}
	}

	// Fallback: return the first non-loopback, non-docker interface with multicast
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		// Skip Docker interfaces
		if strings.HasPrefix(iface.Name, "docker") || strings.HasPrefix(iface.Name, "veth") || strings.HasPrefix(iface.Name, "br-") {
			continue
		}
		return &iface, nil
	}

	return nil, fmt.Errorf("no suitable interface found")
}
