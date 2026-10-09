package retronet

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"

	"aurago/internal/security"
)

// DefaultDialTimeout bounds name resolution plus TCP connect of one dial.
const DefaultDialTimeout = 10 * time.Second

// Reason codes used across the package and on the wire.
const (
	ReasonRefused         = "refused"
	ReasonLimit           = "limit"
	ReasonTimeout         = "timeout"
	ReasonDNS             = "dns"
	ReasonBlocked         = "blocked"
	ReasonRemoteClosed    = "remote_closed"
	ReasonIdle            = "idle"
	ReasonMaxDuration     = "max_duration"
	ReasonDisabled        = "disabled"
	ReasonHostKeyMismatch = "hostkey_mismatch"
	ReasonHostKeyRejected = "hostkey_rejected"
	ReasonServerShutdown  = "server_shutdown"
)

// Hayes result codes.
const (
	CodeBusy       = "BUSY"
	CodeNoAnswer   = "NO ANSWER"
	CodeNoDialtone = "NO DIALTONE"
	CodeNoCarrier  = "NO CARRIER"
)

// errnoWSAConnRefused is WSAECONNREFUSED, the errno Windows reports for a
// refused TCP connect. No other platform uses this errno value.
const errnoWSAConnRefused = syscall.Errno(10061)

// Resolver resolves a host name; *net.Resolver satisfies it.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// DialFunc opens a raw connection; (*net.Dialer).DialContext satisfies it.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Dialer is the guarded dialer every Retro-Net connection goes through.
type Dialer struct {
	Resolver        Resolver      // nil -> net.DefaultResolver
	Dial            DialFunc      // nil -> (&net.Dialer{}).DialContext
	Timeout         time.Duration // 0 -> 10 * time.Second (covers resolve + connect)
	AllowRestricted bool          // tests only: skip the public-IP check (never set in production code)
}

// DialError carries a reason code.
type DialError struct {
	Reason string
	Err    error
}

// Error implements error.
func (e *DialError) Error() string {
	if e.Err == nil {
		return "retronet: " + e.Reason
	}
	return "retronet: " + e.Reason + ": " + e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *DialError) Unwrap() error { return e.Err }

// dialAttemptTimeout caps one connect attempt while further addresses remain.
// It is a variable only so tests can shorten it.
var dialAttemptTimeout = 4 * time.Second

// DialEntry resolves once and rejects blocked ports and restricted IPs (every resolved address
// must be public, otherwise "blocked"; no partial use). It then tries the validated addresses
// one by one, IPv4 first and IPv6 second (resolver order within each family, duplicates
// removed), each dial pinned to that exact "ip:port". Timeout covers the resolve and all
// attempts; an attempt gets at most 4 s while further addresses remain, the last one gets
// whatever is left. It returns the first connection and its pinned target for auditing. When
// every attempt fails, the reason comes from the last attempt's error and the returned target
// is the last attempted address; failures before any attempt return an empty target. Every
// error is a *DialError.
func (d Dialer) DialEntry(ctx context.Context, e Entry) (net.Conn, string, error) {
	if blockedPort(e.Port) {
		return nil, "", &DialError{Reason: ReasonBlocked, Err: errors.New("port " + strconv.Itoa(e.Port) + " is blocked")}
	}
	if e.Host == "" {
		return nil, "", &DialError{Reason: ReasonBlocked, Err: errors.New("entry has no host")}
	}
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = DefaultDialTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ips, err := d.resolve(ctx, e.Host)
	if err != nil {
		return nil, "", err
	}
	if !d.AllowRestricted {
		for _, ip := range ips {
			if security.IsRestrictedNetworkIP(ip) {
				return nil, "", &DialError{Reason: ReasonBlocked, Err: errors.New(e.Host + " resolves to a non-public address")}
			}
		}
	}
	dial := d.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	port := strconv.Itoa(e.Port)
	candidates := orderAddresses(ips)
	deadline, _ := ctx.Deadline()
	var target string
	var lastErr error
	for i, ip := range candidates {
		if i > 0 && (ctx.Err() != nil || !time.Now().Before(deadline)) {
			break // overall budget used up or caller gone
		}
		target = net.JoinHostPort(ip.String(), port)
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, attemptTimeout(deadline, i == len(candidates)-1))
		conn, err := dial(attemptCtx, "tcp", target)
		cancelAttempt()
		if err == nil {
			return conn, target, nil
		}
		lastErr = err
	}
	return nil, target, &DialError{Reason: ReasonOf(lastErr), Err: lastErr}
}

// attemptTimeout returns the budget of one connect attempt: everything left
// until deadline for the last address, otherwise at most dialAttemptTimeout.
func attemptTimeout(deadline time.Time, last bool) time.Duration {
	remaining := time.Until(deadline)
	if last || remaining < dialAttemptTimeout {
		return remaining
	}
	return dialAttemptTimeout
}

// orderAddresses returns ips with IPv4 addresses first and IPv6 second,
// keeping the resolver order within each family and dropping duplicates.
func orderAddresses(ips []net.IP) []net.IP {
	seen := make(map[string]bool, len(ips))
	v4 := make([]net.IP, 0, len(ips))
	var v6 []net.IP
	for _, ip := range ips {
		key := ip.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		if ip.To4() != nil {
			v4 = append(v4, ip)
		} else {
			v6 = append(v6, ip)
		}
	}
	return append(v4, v6...)
}

// resolve returns the addresses for host exactly once. IP literals are used
// as they are; IPv4-mapped IPv6 addresses are unmapped.
func (d Dialer) resolve(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{unmapIP(ip)}, nil
	}
	resolver := d.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, &DialError{Reason: ReasonDNS, Err: err}
	}
	if len(addrs) == 0 {
		return nil, &DialError{Reason: ReasonDNS, Err: errors.New("no addresses for " + host)}
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		ips = append(ips, unmapIP(addr.IP))
	}
	return ips, nil
}

// blockedPort reports ports that are never dialed: 0, the mail submission and
// relay ports 25/465/587, and values outside 1-65535.
func blockedPort(port int) bool {
	switch port {
	case 0, 25, 465, 587:
		return true
	}
	return port < 0 || port > 65535
}

// unmapIP turns an IPv4-mapped IPv6 address into its 4-byte IPv4 form.
func unmapIP(ip net.IP) net.IP {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}

// ReasonOf maps any error to a reason: *DialError -> its Reason; ECONNREFUSED -> refused;
// timeouts/deadline -> timeout; *net.DNSError -> dns; everything else (including nil) -> remote_closed.
func ReasonOf(err error) string {
	if err == nil {
		return ReasonRemoteClosed
	}
	var dialErr *DialError
	if errors.As(err, &dialErr) && dialErr.Reason != "" {
		return dialErr.Reason
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, errnoWSAConnRefused) {
		return ReasonRefused
	}
	if isTimeout(err) {
		return ReasonTimeout
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ReasonDNS
	}
	return ReasonRemoteClosed
}

// isTimeout reports deadline and timeout errors.
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// CodeFor maps a reason to its Hayes code: refused, limit -> BUSY; timeout -> NO ANSWER;
// dns, blocked -> NO DIALTONE; all others -> NO CARRIER.
func CodeFor(reason string) string {
	switch reason {
	case ReasonRefused, ReasonLimit:
		return CodeBusy
	case ReasonTimeout:
		return CodeNoAnswer
	case ReasonDNS, ReasonBlocked:
		return CodeNoDialtone
	default:
		return CodeNoCarrier
	}
}
