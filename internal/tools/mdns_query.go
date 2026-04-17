// Package tools – mdns_query: low-level mDNS service discovery using miekg/dns.
package tools

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
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
	// Use net.ListenMulticastUDP with nil interface like the original
	// hashicorp/mdns library. The OS manages multicast routing automatically.
	ipv4Group := net.UDPAddr{IP: net.ParseIP(mdnsIPv4Group), Port: mdnsPortNum}
	pc, err := net.ListenMulticastUDP("udp4", nil, &ipv4Group)
	if err != nil {
		return nil, fmt.Errorf("mdns: ListenMulticastUDP: %w", err)
	}
	defer pc.Close()

	logger.Info("mdns: listening on multicast group 224.0.0.251:5353")

	// Send query via a separate dialed socket so kernel picks source IP via routing.
	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn(serviceType), dns.TypePTR)
	q.RecursionDesired = false

	buf, err := q.Pack()
	if err != nil {
		return nil, fmt.Errorf("mdns: pack query: %w", err)
	}

	sendConn, err := net.Dial("udp4", "224.0.0.251:5353")
	if err != nil {
		return nil, fmt.Errorf("mdns: dial send socket: %w", err)
	}
	_, err = sendConn.Write(buf)
	sendConn.Close()
	if err != nil {
		return nil, fmt.Errorf("mdns: send query: %w", err)
	}

	// Set read deadline for the collection phase.
	if err := pc.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("mdns: set deadline: %w", err)
	}

	entries := make(map[string]*mdnsEntry)
	rbuf := make([]byte, 65535)
	packetCount := 0

	for {
		n, _, err := pc.ReadFrom(rbuf)
		if err != nil {
			logger.Info("mdns: read loop ended", "packet_count", packetCount, "error", err)
			break
		}
		packetCount++
		logger.Info("mdns: received packet", "size", n, "packet_num", packetCount, "hex", fmt.Sprintf("%x", rbuf[:n]))

		var msg dns.Msg
		if err := msg.Unpack(rbuf[:n]); err != nil {
			logger.Info("mdns: unpack failed", "error", err)
			continue
		}

		logger.Info("mdns: dns message received", "questions", len(msg.Question), "answers", len(msg.Answer), "extra", len(msg.Extra))

		allRRs := append(msg.Answer, msg.Extra...)

		// Collect PTR records first to create or update entries.
		for _, rr := range allRRs {
			if ptr, ok := rr.(*dns.PTR); ok {
				// ptr.Ptr is the service instance name, e.g. "Google-Home-Mini-..._googlecast._tcp.local."
				if _, exists := entries[ptr.Ptr]; !exists {
					entries[ptr.Ptr] = &mdnsEntry{Name: ptr.Ptr}
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
