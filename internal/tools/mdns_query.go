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

// mdnsQueryServices sends PTR queries for serviceType over IPv4 mDNS multicast
// and collects responses for the given timeout duration.
// It joins the multicast group on the default-route LAN interface and uses
// SO_REUSEADDR/SO_REUSEPORT so it can co-exist with system mDNS daemons.
// Multiple queries are sent to compensate for UDP packet loss.
func mdnsQueryServices(serviceType string, timeout time.Duration, logger *slog.Logger) ([]*mdnsEntry, error) {
	// Select the correct LAN interface to avoid Docker/veth confusion.
	iface, ifErr := defaultRouteInterface()
	if ifErr != nil {
		logger.Warn("mdns: could not determine default route interface, falling back to nil", "error", ifErr)
	} else {
		logger.Info("mdns: using interface for multicast", "interface", iface.Name)
	}

	ipv4Group := net.UDPAddr{IP: net.ParseIP(mdnsIPv4Group), Port: mdnsPortNum}
	pc, err := net.ListenMulticastUDP("udp4", iface, &ipv4Group)
	if err != nil {
		// Fallback: try with nil interface if specific one failed
		if iface != nil {
			logger.Warn("mdns: ListenMulticastUDP with specific interface failed, retrying with nil", "error", err)
			pc, err = net.ListenMulticastUDP("udp4", nil, &ipv4Group)
		}
		if err != nil {
			return nil, fmt.Errorf("mdns: ListenMulticastUDP: %w", err)
		}
	}
	defer pc.Close()

	logger.Info("mdns: listening on multicast group 224.0.0.251:5353")

	// Build the query packet once.
	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn(serviceType), dns.TypePTR)
	q.RecursionDesired = false

	buf, err := q.Pack()
	if err != nil {
		return nil, fmt.Errorf("mdns: pack query: %w", err)
	}

	// Send query via a separate dialed socket so kernel picks source IP via routing.
	// Keep the socket open so unicast replies are received too.
	sendConn, err := net.Dial("udp4", "224.0.0.251:5353")
	if err != nil {
		return nil, fmt.Errorf("mdns: dial send socket: %w", err)
	}
	defer sendConn.Close()

	// Send multiple queries to compensate for UDP packet loss.
	// Schedule: 0ms, 500ms, 1500ms, 3500ms (exponential backoff)
	queryIntervals := []time.Duration{0, 500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
	go func() {
		for i, delay := range queryIntervals {
			if delay > 0 {
				time.Sleep(delay)
			}
			if _, wErr := sendConn.Write(buf); wErr != nil {
				logger.Warn("mdns: send query failed", "attempt", i+1, "error", wErr)
			} else {
				logger.Info("mdns: sent query", "attempt", i+1, "service", serviceType)
			}
		}
	}()

	// Set read deadline for the collection phase.
	deadline := time.Now().Add(timeout)
	if err := pc.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("mdns: set deadline: %w", err)
	}

	entries := make(map[string]*mdnsEntry)
	rbuf := make([]byte, 65535)
	packetCount := 0

	for {
		n, _, readErr := pc.ReadFrom(rbuf)
		if readErr != nil {
			logger.Info("mdns: read loop ended", "packet_count", packetCount, "error", readErr)
			break
		}
		packetCount++
		logger.Debug("mdns: received packet", "size", n, "packet_num", packetCount)

		var msg dns.Msg
		if err := msg.Unpack(rbuf[:n]); err != nil {
			logger.Debug("mdns: unpack failed", "error", err)
			continue
		}

		logger.Debug("mdns: dns message", "answers", len(msg.Answer), "extra", len(msg.Extra))

		allRRs := append(msg.Answer, msg.Extra...)

		// Collect PTR records first to create or update entries.
		for _, rr := range allRRs {
			if ptr, ok := rr.(*dns.PTR); ok {
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
