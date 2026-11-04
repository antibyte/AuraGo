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
	var lastErr error
	// Leave time for later addresses if a preferred route stalls.
	dialer := &net.Dialer{Timeout: max(2*time.Second, timeout/time.Duration(len(ips)))}
	for _, addr := range ips {
		if network == "tcp4" && addr.IP.To4() == nil || network == "tcp6" && addr.IP.To4() != nil {
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = ErrMailTarget
	}
	return nil, fmt.Errorf("connect to mail server: %w", lastErr)
}
