package mailer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"

	"aurago/internal/security"
)

var ErrMailTarget = errors.New("mail server target is not permitted")

// Configured LAN and loopback mail servers are explicit operator targets.
func allowedMailIP(ip net.IP) bool {
	if (!ip.IsGlobalUnicast() && !ip.IsLoopback()) || ip.IsLinkLocalUnicast() ||
		ip.Equal(net.ParseIP("fd00:ec2::254")) || ip.Equal(net.ParseIP("100.100.100.200")) {
		return false
	}
	addr, _ := netip.AddrFromSlice(ip)
	return ip.IsPrivate() || ip.IsLoopback() || netip.MustParsePrefix("100.64.0.0/10").Contains(addr.Unmap()) || !security.IsRestrictedNetworkIP(ip)
}

func dialMailTransport(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, ErrMailTarget
	}
	host, port, err := net.SplitHostPort(address)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || host == "" || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return nil, ErrMailTarget
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve mail server: %w", err)
	}
	if len(ips) == 0 {
		return nil, ErrMailTarget
	}
	for _, addr := range ips {
		if addr.Zone != "" || !allowedMailIP(addr.IP) {
			return nil, ErrMailTarget
		}
	}
	dialer := &net.Dialer{Timeout: max(2*time.Second, timeout/time.Duration(len(ips)))}
	conn, err := dialMailIPs(ctx, network, port, ips, dialer.DialContext)
	if err != nil {
		return nil, fmt.Errorf("connect to mail server: %w", err)
	}
	return conn, nil
}

// Race address families without resolving the validated targets again.
func dialMailIPs(ctx context.Context, network, port string, ips []net.IPAddr, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	var primary, fallback []net.IPAddr
	for _, addr := range ips {
		if network == "tcp4" && addr.IP.To4() == nil || network == "tcp6" && addr.IP.To4() != nil {
			continue
		}
		if len(primary) == 0 || (addr.IP.To4() != nil) == (primary[0].IP.To4() != nil) {
			primary = append(primary, addr)
		} else {
			fallback = append(fallback, addr)
		}
	}
	if len(primary) == 0 {
		return nil, ErrMailTarget
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		conn net.Conn
		err  error
	}
	results := make(chan result)
	dialGroup := func(group []net.IPAddr) {
		var r result
		for _, addr := range group {
			r.conn, r.err = dial(ctx, network, net.JoinHostPort(addr.IP.String(), port))
			if r.err == nil || ctx.Err() != nil {
				break
			}
		}
		select {
		case results <- r:
		case <-ctx.Done():
			if r.conn != nil {
				_ = r.conn.Close()
			}
		}
	}
	timer := time.NewTimer(300 * time.Millisecond)
	defer timer.Stop()
	pending, fallbackStarted := 1, len(fallback) == 0
	startFallback := func() {
		if !fallbackStarted {
			fallbackStarted = true
			pending++
			go dialGroup(fallback)
		}
	}
	go dialGroup(primary)
	var lastErr error
	for pending > 0 {
		select {
		case r := <-results:
			pending--
			if r.err == nil {
				return r.conn, nil
			}
			lastErr = r.err
			startFallback()
		case <-timer.C:
			startFallback()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}
