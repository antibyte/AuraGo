// Package tools – mdns_query: low-level mDNS service discovery using miekg/dns.
package tools

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
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

// mdnsQueryServices sends PTR queries for serviceType over IPv4 mDNS multicast
// and collects responses for the given timeout duration.
// It joins the multicast group on the default-route LAN interface and uses
// SO_REUSEADDR/SO_REUSEPORT so it can co-exist with system mDNS daemons.
//
// Queries are sent from BOTH the multicast socket (port 5353) and a unicast
// socket (ephemeral port). Per RFC 6762 §5.5, queries from port 5353 force
// multicast responses; queries from other ports get unicast responses.
// We read from both sockets to catch all replies.
func mdnsQueryServices(serviceType string, timeout time.Duration, logger *slog.Logger) ([]*mdnsEntry, error) {
	// Select the correct LAN interface to avoid Docker/veth confusion.
	iface, ifErr := defaultRouteInterface()
	if ifErr != nil {
		logger.Warn("mdns: could not determine default route interface, falling back to nil", "error", ifErr)
	} else {
		logger.Info("mdns: using interface for multicast", "interface", iface.Name)
	}

	ipv4Group := net.UDPAddr{IP: net.ParseIP(mdnsIPv4Group), Port: mdnsPortNum}
	mcastConn, err := net.ListenMulticastUDP("udp4", iface, &ipv4Group)
	if err != nil {
		// Fallback: try with nil interface if specific one failed
		if iface != nil {
			logger.Warn("mdns: ListenMulticastUDP with specific interface failed, retrying with nil", "error", err)
			mcastConn, err = net.ListenMulticastUDP("udp4", nil, &ipv4Group)
		}
		if err != nil {
			return nil, fmt.Errorf("mdns: ListenMulticastUDP: %w", err)
		}
	}
	defer mcastConn.Close()

	// Wrap with ipv4.PacketConn to explicitly control outgoing multicast interface,
	// TTL, and loopback. Without this, the kernel picks the outgoing interface via
	// routing table — which can be Docker/VPN/wrong interface, and packets never
	// reach chromecasts on the LAN.
	mcastP := ipv4.NewPacketConn(mcastConn)
	if iface != nil {
		if mifErr := mcastP.SetMulticastInterface(iface); mifErr != nil {
			logger.Warn("mdns: SetMulticastInterface failed on mcast socket", "error", mifErr)
		}
	}
	_ = mcastP.SetMulticastTTL(255) // RFC 6762: mDNS packets SHOULD have TTL 255
	_ = mcastP.SetMulticastLoopback(true)

	logger.Info("mdns: listening on multicast group 224.0.0.251:5353")

	// Build the query packet once.
	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn(serviceType), dns.TypePTR)
	q.RecursionDesired = false

	buf, err := q.Pack()
	if err != nil {
		return nil, fmt.Errorf("mdns: pack query: %w", err)
	}

	// Create a unicast socket for sending and receiving unicast replies.
	// Devices may unicast responses back to the ephemeral source port.
	// Bind to the selected interface's IP if possible, so the source address
	// in outgoing packets is on the LAN (not a Docker/VPN address).
	var bindAddr string
	if iface != nil {
		if addrs, aErr := iface.Addrs(); aErr == nil {
			for _, a := range addrs {
				if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() {
					bindAddr = ipn.IP.String() + ":0"
					break
				}
			}
		}
	}
	if bindAddr == "" {
		bindAddr = ":0"
	}
	ucastConn, err := net.ListenPacket("udp4", bindAddr)
	if err != nil {
		logger.Warn("mdns: could not create unicast listener, unicast replies will be missed", "error", err, "bindAddr", bindAddr)
		// Retry with wildcard
		ucastConn, err = net.ListenPacket("udp4", ":0")
		if err != nil {
			logger.Warn("mdns: could not create wildcard unicast listener either", "error", err)
		}
	}
	var ucastP *ipv4.PacketConn
	if ucastConn != nil {
		defer ucastConn.Close()
		ucastP = ipv4.NewPacketConn(ucastConn)
		if iface != nil {
			_ = ucastP.SetMulticastInterface(iface)
		}
		_ = ucastP.SetMulticastTTL(255)
	}

	dst := &net.UDPAddr{IP: net.ParseIP(mdnsIPv4Group), Port: mdnsPortNum}
	deadline := time.Now().Add(timeout)

	// Send multiple queries from both sockets to maximize discovery reliability.
	// Schedule: 0ms, 250ms, 750ms, 2000ms (exponential backoff)
	queryIntervals := []time.Duration{0, 250 * time.Millisecond, 500 * time.Millisecond, 1250 * time.Millisecond}
	go func() {
		for i, delay := range queryIntervals {
			if delay > 0 {
				time.Sleep(delay)
			}
			// Send from multicast socket (source port 5353).
			// Per RFC 6762 §5.5, responses to queries from port 5353 MUST be multicast.
			if _, wErr := mcastConn.WriteTo(buf, dst); wErr != nil {
				logger.Warn("mdns: multicast send failed", "attempt", i+1, "error", wErr)
			} else {
				logger.Info("mdns: sent query via multicast socket", "attempt", i+1, "service", serviceType)
			}
			// Also send from unicast socket (ephemeral port).
			// Some devices respond faster via unicast, and this catches devices
			// that ignore multicast queries on busy networks.
			if ucastConn != nil {
				if _, wErr := ucastConn.WriteTo(buf, dst); wErr != nil {
					logger.Warn("mdns: unicast send failed", "attempt", i+1, "error", wErr)
				}
			}
		}
	}()

	// Set read deadline for both sockets.
	if err := mcastConn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("mdns: set deadline: %w", err)
	}
	if ucastConn != nil {
		_ = ucastConn.SetDeadline(deadline)
	}

	var mu sync.Mutex
	entries := make(map[string]*mdnsEntry)
	packetCount := 0

	// Normalize service type to FQDN for filtering, e.g. "_googlecast._tcp.local."
	serviceTypeFQDN := dns.Fqdn(serviceType)

	// processDNS extracts mDNS entries from a raw DNS packet.
	// Only PTR records whose Hdr.Name matches the queried service type are accepted.
	// This prevents unsolicited Avahi/mDNS announcements for other services
	// (Spotify Connect, AirPlay, etc.) from polluting the results.
	processDNS := func(data []byte, n int, source string) {
		var msg dns.Msg
		if err := msg.Unpack(data[:n]); err != nil {
			return
		}

		mu.Lock()
		defer mu.Unlock()

		allRRs := append(msg.Answer, msg.Extra...)

		// Collect PTR records — only for the service type we queried.
		for _, rr := range allRRs {
			if ptr, ok := rr.(*dns.PTR); ok {
				// Only accept PTR records whose owner name is the queried service type.
				// In mDNS, PTR answers look like:
				//   _googlecast._tcp.local. PTR Google-Home._googlecast._tcp.local.
				// So Hdr.Name must be our service type FQDN.
				if !strings.EqualFold(dns.Fqdn(ptr.Hdr.Name), serviceTypeFQDN) {
					continue
				}
				if _, exists := entries[ptr.Ptr]; !exists {
					entries[ptr.Ptr] = &mdnsEntry{Name: ptr.Ptr}
					logger.Info("mdns: discovered service", "name", ptr.Ptr, "via", source)
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

	// Read from unicast socket in a separate goroutine.
	var wg sync.WaitGroup
	if ucastConn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ubuf := make([]byte, 65535)
			for {
				n, _, readErr := ucastConn.ReadFrom(ubuf)
				if readErr != nil {
					return
				}
				mu.Lock()
				packetCount++
				mu.Unlock()
				processDNS(ubuf, n, "unicast")
			}
		}()
	}

	// Read from multicast socket in the main goroutine.
	rbuf := make([]byte, 65535)
	for {
		n, _, readErr := mcastConn.ReadFrom(rbuf)
		if readErr != nil {
			break
		}
		mu.Lock()
		packetCount++
		mu.Unlock()
		processDNS(rbuf, n, "multicast")
	}

	// Wait for unicast reader to finish (deadline will end it).
	wg.Wait()

	mu.Lock()
	result := make([]*mdnsEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, e)
	}
	logger.Info("mdns: discovery complete", "entries_found", len(result), "packets_received", packetCount)
	mu.Unlock()
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
