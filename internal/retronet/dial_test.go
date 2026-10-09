package retronet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

// fakeResolver returns answers[i] on call i (the last answer repeats) or err.
type fakeResolver struct {
	mu      sync.Mutex
	answers [][]string
	err     error
	calls   int
}

func (r *fakeResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if len(r.answers) == 0 {
		return nil, nil
	}
	answer := r.answers[min(r.calls-1, len(r.answers)-1)]
	out := make([]net.IPAddr, 0, len(answer))
	for _, s := range answer {
		out = append(out, net.IPAddr{IP: net.ParseIP(s)})
	}
	return out, nil
}

func (r *fakeResolver) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// fakeDial records every dial and answers with an in-memory pipe, with
// failures[address] for that address, or with err for every address.
type fakeDial struct {
	mu        sync.Mutex
	networks  []string
	addresses []string
	err       error
	failures  map[string]error
	peers     []net.Conn
}

func (f *fakeDial) dial(_ context.Context, network, address string) (net.Conn, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.networks = append(f.networks, network)
	f.addresses = append(f.addresses, address)
	if err, ok := f.failures[address]; ok {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	client, server := net.Pipe()
	f.peers = append(f.peers, server)
	return client, nil
}

func (f *fakeDial) dialed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.addresses...)
}

func (f *fakeDial) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.peers {
		p.Close()
	}
}

func telnetTestEntry(host string, port int) Entry {
	return Entry{ID: "test", Protocol: ProtocolTelnet, Host: host, Port: port, Kind: KindWorld, Charset: CharsetUTF8}
}

func requireReason(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("DialEntry() error = nil, want reason %q", want)
	}
	var dialErr *DialError
	if !errors.As(err, &dialErr) {
		t.Fatalf("DialEntry() error %T is not a *DialError", err)
	}
	if got := ReasonOf(err); got != want {
		t.Fatalf("ReasonOf(%v) = %q, want %q", err, got, want)
	}
}

var restrictedAddresses = []struct {
	name string
	ip   string
}{
	{"loopback v4", "127.0.0.1"},
	{"loopback v6", "::1"},
	{"private 10/8", "10.1.2.3"},
	{"private 172.16/12", "172.20.0.5"},
	{"private 192.168/16", "192.168.178.1"},
	{"cgnat", "100.64.0.1"},
	{"link-local v4 metadata", "169.254.169.254"},
	{"link-local v6", "fe80::1"},
	{"unique local v6", "fd00::1"},
	{"v4-mapped loopback", "::ffff:127.0.0.1"},
	{"v4-mapped private", "::ffff:10.0.0.1"},
	{"unspecified v4", "0.0.0.0"},
	{"unspecified v6", "::"},
	{"multicast", "224.0.0.1"},
	// Regression guard for internal/security: embedded and reserved forms.
	{"nat64 well-known prefix", "64:ff9b::7f00:1"},
	{"6to4 of loopback", "2002:7f00:1::"},
	{"teredo", "2001:0:4136:e378:8000:63bf:3fff:fdd2"},
	{"v4-compatible loopback", "::127.0.0.1"},
	{"documentation 192.0.2/24", "192.0.2.1"},
	{"reserved 240/4", "240.0.0.1"},
	{"link-local multicast v6", "ff02::1"},
}

func TestDialEntryBlocksRestrictedResolvedAddresses(t *testing.T) {
	for _, tc := range restrictedAddresses {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &fakeResolver{answers: [][]string{{tc.ip}}}
			dial := &fakeDial{}
			_, target, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry("bbs.example.org", 23))
			requireReason(t, err, ReasonBlocked)
			if target != "" {
				t.Fatalf("target = %q, want empty", target)
			}
			if got := dial.dialed(); len(got) != 0 {
				t.Fatalf("dialed %v, want no dial", got)
			}
		})
	}
}

func TestDialEntryBlocksRestrictedLiteralHosts(t *testing.T) {
	for _, tc := range restrictedAddresses {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &fakeResolver{answers: [][]string{{"1.1.1.1"}}}
			dial := &fakeDial{}
			_, _, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry(tc.ip, 23))
			requireReason(t, err, ReasonBlocked)
			if n := resolver.callCount(); n != 0 {
				t.Fatalf("resolver called %d times for a literal IP", n)
			}
			if got := dial.dialed(); len(got) != 0 {
				t.Fatalf("dialed %v, want no dial", got)
			}
		})
	}
}

func TestDialEntryBlocksWhenAnyResolvedAddressIsRestricted(t *testing.T) {
	answers := map[string][]string{
		"public v4 then private v4": {"1.1.1.1", "192.168.1.10"},
		"private v4 then public v4": {"10.0.0.7", "8.8.8.8"},
		"public v4 then ula v6":     {"1.1.1.1", "fd00::1"},
		"public v6 then loopback":   {"2606:4700:4700::1111", "127.0.0.1"},
	}
	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			resolver := &fakeResolver{answers: [][]string{answer}}
			dial := &fakeDial{}
			_, target, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry("mixed.example.org", 23))
			requireReason(t, err, ReasonBlocked)
			if target != "" {
				t.Fatalf("target = %q, want empty", target)
			}
			if got := dial.dialed(); len(got) != 0 {
				t.Fatalf("dialed %v, want zero DialFunc calls", got)
			}
		})
	}
}

func TestDialEntryBlocksPortsBeforeResolving(t *testing.T) {
	for _, port := range []int{25, 465, 587, 0, -1, 65536} {
		t.Run(strconv.Itoa(port), func(t *testing.T) {
			resolver := &fakeResolver{answers: [][]string{{"1.1.1.1"}}}
			dial := &fakeDial{}
			_, _, err := Dialer{Resolver: resolver, Dial: dial.dial, AllowRestricted: true}.DialEntry(context.Background(), telnetTestEntry("mail.example.org", port))
			requireReason(t, err, ReasonBlocked)
			if n := resolver.callCount(); n != 0 {
				t.Fatalf("resolver called %d times for a blocked port", n)
			}
			if got := dial.dialed(); len(got) != 0 {
				t.Fatalf("dialed %v, want no dial", got)
			}
		})
	}
}

func TestDialEntryRejectsEmptyHost(t *testing.T) {
	_, _, err := Dialer{Resolver: &fakeResolver{}, Dial: (&fakeDial{}).dial}.DialEntry(context.Background(), telnetTestEntry("", 23))
	requireReason(t, err, ReasonBlocked)
}

func TestDialEntryDialsPinnedAddressesInOrder(t *testing.T) {
	tests := []struct {
		name    string
		answers [][]string
		port    int
		want    string
	}{
		{"first of two public addresses", [][]string{{"1.1.1.1", "8.8.8.8"}}, 23, "1.1.1.1:23"},
		{"ipv4 before ipv6", [][]string{{"2606:4700:4700::1111", "1.1.1.1"}}, 3023, "1.1.1.1:3023"},
		{"ipv4 before ipv6 keeps v4 order", [][]string{{"2606:4700:4700::1111", "9.9.9.9", "8.8.8.8"}}, 23, "9.9.9.9:23"},
		{"rebinding resolver resolved once", [][]string{{"9.9.9.9"}, {"127.0.0.1"}}, 2323, "9.9.9.9:2323"},
		{"ipv6 only", [][]string{{"2606:4700:4700::1111"}}, 4000, "[2606:4700:4700::1111]:4000"},
		{"v4-mapped public is unmapped", [][]string{{"::ffff:8.8.4.4"}}, 23, "8.8.4.4:23"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &fakeResolver{answers: tc.answers}
			dial := &fakeDial{}
			defer dial.close()
			conn, target, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry("bbs.example.org", tc.port))
			if err != nil {
				t.Fatalf("DialEntry() error = %v", err)
			}
			defer conn.Close()
			if target != tc.want {
				t.Fatalf("target = %q, want %q", target, tc.want)
			}
			if got := dial.dialed(); len(got) != 1 || got[0] != tc.want {
				t.Fatalf("dialed %v, want exactly [%s]", got, tc.want)
			}
			if dial.networks[0] != "tcp" {
				t.Fatalf("network = %q, want tcp", dial.networks[0])
			}
			if n := resolver.callCount(); n != 1 {
				t.Fatalf("resolver called %d times, want 1", n)
			}
		})
	}
}

func TestDialEntryDialsPublicLiteralWithoutResolver(t *testing.T) {
	resolver := &fakeResolver{}
	dial := &fakeDial{}
	defer dial.close()
	conn, target, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry("87.106.7.15", 23))
	if err != nil {
		t.Fatalf("DialEntry() error = %v", err)
	}
	conn.Close()
	if target != "87.106.7.15:23" {
		t.Fatalf("target = %q, want 87.106.7.15:23", target)
	}
	if n := resolver.callCount(); n != 0 {
		t.Fatalf("resolver called %d times for a literal IP", n)
	}
}

func TestDialEntryDNSFailures(t *testing.T) {
	tests := []struct {
		name     string
		resolver *fakeResolver
	}{
		{"not found", &fakeResolver{err: &net.DNSError{Err: "no such host", Name: "nowhere.example", IsNotFound: true}}},
		{"resolver timeout", &fakeResolver{err: &net.DNSError{Err: "i/o timeout", Name: "slow.example", IsTimeout: true}}},
		{"empty answer", &fakeResolver{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dial := &fakeDial{}
			_, target, err := Dialer{Resolver: tc.resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry("nowhere.example", 23))
			requireReason(t, err, ReasonDNS)
			if CodeFor(ReasonOf(err)) != CodeNoDialtone {
				t.Fatalf("CodeFor = %q, want %q", CodeFor(ReasonOf(err)), CodeNoDialtone)
			}
			if target != "" || len(dial.dialed()) != 0 {
				t.Fatalf("target %q / dials %v after DNS failure", target, dial.dialed())
			}
		})
	}
}

func TestDialEntryConnectFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"econnrefused", &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}, ReasonRefused},
		{"wsaeconnrefused", &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connectex", syscall.Errno(10061))}, ReasonRefused},
		{"i/o timeout", &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}, ReasonTimeout},
		{"reset", &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNRESET)}, ReasonRemoteClosed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &fakeResolver{answers: [][]string{{"1.1.1.1"}}}
			dial := &fakeDial{err: tc.err}
			_, target, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry("bbs.example.org", 23))
			requireReason(t, err, tc.want)
			if target != "1.1.1.1:23" {
				t.Fatalf("target = %q, want the pinned 1.1.1.1:23 for auditing", target)
			}
		})
	}
}

func TestDialEntryTimeoutCoversConnect(t *testing.T) {
	resolver := &fakeResolver{answers: [][]string{{"1.1.1.1"}}}
	blocking := func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	start := time.Now()
	_, _, err := Dialer{Resolver: resolver, Dial: blocking, Timeout: 50 * time.Millisecond}.DialEntry(context.Background(), telnetTestEntry("slow.example.org", 23))
	requireReason(t, err, ReasonTimeout)
	if CodeFor(ReasonOf(err)) != CodeNoAnswer {
		t.Fatalf("CodeFor = %q, want %q", CodeFor(ReasonOf(err)), CodeNoAnswer)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("dial took %v, want about 50ms", elapsed)
	}
}

func TestDialEntryDefaultTimeoutIsTenSeconds(t *testing.T) {
	resolver := &fakeResolver{answers: [][]string{{"1.1.1.1"}}}
	var remaining time.Duration
	probe := func(ctx context.Context, _, _ string) (net.Conn, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return nil, errors.New("no deadline")
		}
		remaining = time.Until(deadline)
		return nil, io.EOF
	}
	_, _, _ = Dialer{Resolver: resolver, Dial: probe}.DialEntry(context.Background(), telnetTestEntry("bbs.example.org", 23))
	if remaining <= 9*time.Second || remaining > DefaultDialTimeout {
		t.Fatalf("remaining deadline = %v, want just under %v", remaining, DefaultDialTimeout)
	}
}

func refusedErr() error {
	return &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
}

func timeoutErr() error {
	return &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}
}

func TestDialEntryFallsBackToNextAddress(t *testing.T) {
	tests := []struct {
		name       string
		answer     []string
		failures   map[string]error
		wantDialed []string
		want       string
	}{
		{
			name:       "ipv4 refused, ipv6 answers",
			answer:     []string{"2606:4700:4700::1111", "1.1.1.1"},
			failures:   map[string]error{"1.1.1.1:3023": refusedErr()},
			wantDialed: []string{"1.1.1.1:3023", "[2606:4700:4700::1111]:3023"},
			want:       "[2606:4700:4700::1111]:3023",
		},
		{
			name:       "first ipv4 times out, second ipv4 answers",
			answer:     []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"},
			failures:   map[string]error{"1.1.1.1:3023": timeoutErr()},
			wantDialed: []string{"1.1.1.1:3023", "8.8.8.8:3023"},
			want:       "8.8.8.8:3023",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &fakeResolver{answers: [][]string{tc.answer}}
			dial := &fakeDial{failures: tc.failures}
			defer dial.close()
			conn, target, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry("funtopia.example.org", 3023))
			if err != nil {
				t.Fatalf("DialEntry() error = %v", err)
			}
			defer conn.Close()
			if target != tc.want {
				t.Fatalf("target = %q, want %q", target, tc.want)
			}
			if got := dial.dialed(); !slices.Equal(got, tc.wantDialed) {
				t.Fatalf("dialed %v, want %v", got, tc.wantDialed)
			}
			if n := resolver.callCount(); n != 1 {
				t.Fatalf("resolver called %d times, want 1", n)
			}
		})
	}
}

func TestDialEntryAllAddressesFail(t *testing.T) {
	answer := []string{"2606:4700:4700::1111", "1.1.1.1", "8.8.8.8"}
	order := []string{"1.1.1.1:23", "8.8.8.8:23", "[2606:4700:4700::1111]:23"}
	tests := []struct {
		name     string
		failures map[string]error
		want     string
	}{
		{"all refused", map[string]error{order[0]: refusedErr(), order[1]: refusedErr(), order[2]: refusedErr()}, ReasonRefused},
		{"last attempt decides: timeout", map[string]error{order[0]: refusedErr(), order[1]: refusedErr(), order[2]: timeoutErr()}, ReasonTimeout},
		{"last attempt decides: refused", map[string]error{order[0]: timeoutErr(), order[1]: timeoutErr(), order[2]: refusedErr()}, ReasonRefused},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &fakeResolver{answers: [][]string{answer}}
			dial := &fakeDial{failures: tc.failures}
			_, target, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry("down.example.org", 23))
			requireReason(t, err, tc.want)
			if target != order[2] {
				t.Fatalf("target = %q, want the last attempted %q", target, order[2])
			}
			if got := dial.dialed(); !slices.Equal(got, order) {
				t.Fatalf("dialed %v, want %v", got, order)
			}
		})
	}
}

func TestDialEntryDeduplicatesAddresses(t *testing.T) {
	resolver := &fakeResolver{answers: [][]string{{"1.1.1.1", "::ffff:1.1.1.1", "1.1.1.1"}}}
	dial := &fakeDial{err: refusedErr()}
	_, target, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(context.Background(), telnetTestEntry("dup.example.org", 23))
	requireReason(t, err, ReasonRefused)
	if got := dial.dialed(); !slices.Equal(got, []string{"1.1.1.1:23"}) || target != "1.1.1.1:23" {
		t.Fatalf("dialed %v (target %q), want exactly one attempt at 1.1.1.1:23", got, target)
	}
}

// blockingDial waits until the attempt context ends and records the budget
// each attempt received.
type blockingDial struct {
	mu      sync.Mutex
	budgets []time.Duration
}

func (b *blockingDial) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	deadline, _ := ctx.Deadline()
	b.mu.Lock()
	b.budgets = append(b.budgets, time.Until(deadline))
	b.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestDialEntryBudgetCoversAllAttempts(t *testing.T) {
	resolver := &fakeResolver{answers: [][]string{{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"}}}
	dial := &blockingDial{}
	const timeout = 300 * time.Millisecond
	start := time.Now()
	_, target, err := Dialer{Resolver: resolver, Dial: dial.dial, Timeout: timeout}.DialEntry(context.Background(), telnetTestEntry("slow.example.org", 23))
	elapsed := time.Since(start)
	requireReason(t, err, ReasonTimeout)
	if elapsed < timeout-20*time.Millisecond || elapsed > 2*timeout {
		t.Fatalf("three blocking addresses took %v, want about %v (not 3 x %v)", elapsed, timeout, timeout)
	}
	if len(dial.budgets) != 1 || target != "1.1.1.1:23" {
		t.Fatalf("attempts %v (target %q), want one attempt that used the whole budget", dial.budgets, target)
	}
}

func TestDialEntryAttemptTimeoutsShareTheBudget(t *testing.T) {
	previous := dialAttemptTimeout
	dialAttemptTimeout = 100 * time.Millisecond
	t.Cleanup(func() { dialAttemptTimeout = previous })

	resolver := &fakeResolver{answers: [][]string{{"2606:4700:4700::1111", "1.1.1.1", "8.8.8.8"}}}
	dial := &blockingDial{}
	const timeout = 600 * time.Millisecond
	start := time.Now()
	_, target, err := Dialer{Resolver: resolver, Dial: dial.dial, Timeout: timeout}.DialEntry(context.Background(), telnetTestEntry("slow.example.org", 23))
	elapsed := time.Since(start)
	requireReason(t, err, ReasonTimeout)
	if target != "[2606:4700:4700::1111]:23" {
		t.Fatalf("target = %q, want the last attempted IPv6 address", target)
	}
	if len(dial.budgets) != 3 {
		t.Fatalf("attempt budgets = %v, want three attempts", dial.budgets)
	}
	for i, budget := range dial.budgets[:2] {
		if budget > dialAttemptTimeout || budget < dialAttemptTimeout/2 {
			t.Fatalf("attempt %d budget = %v, want the %v cap", i, budget, dialAttemptTimeout)
		}
	}
	if last := dial.budgets[2]; last < 300*time.Millisecond {
		t.Fatalf("last attempt budget = %v, want everything that remains (about 400ms)", last)
	}
	if elapsed < timeout-50*time.Millisecond || elapsed > timeout+300*time.Millisecond {
		t.Fatalf("dial took %v, want about %v", elapsed, timeout)
	}
}

func TestDialEntryRealLoopbackWithAllowRestricted(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	entry := telnetTestEntry("127.0.0.1", port)

	if _, _, err := (Dialer{}).DialEntry(context.Background(), entry); ReasonOf(err) != ReasonBlocked {
		t.Fatalf("production dialer reached loopback: %v", err)
	}
	conn, target, err := Dialer{AllowRestricted: true}.DialEntry(context.Background(), entry)
	if err != nil {
		t.Fatalf("DialEntry(AllowRestricted) error = %v", err)
	}
	defer conn.Close()
	if want := fmt.Sprintf("127.0.0.1:%d", port); target != want {
		t.Fatalf("target = %q, want %q", target, want)
	}
	select {
	case c := <-accepted:
		c.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("listener never accepted the connection")
	}
}

func TestDialEntryRealRefusedLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	_, _, err = Dialer{AllowRestricted: true}.DialEntry(context.Background(), telnetTestEntry("127.0.0.1", port))
	requireReason(t, err, ReasonRefused)
}

func TestReasonOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ReasonRemoteClosed},
		{"dial error", &DialError{Reason: ReasonLimit}, ReasonLimit},
		{"wrapped dial error", fmt.Errorf("ssh: %w", &DialError{Reason: ReasonHostKeyMismatch, Err: errors.New("key changed")}), ReasonHostKeyMismatch},
		{"econnrefused", &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}, ReasonRefused},
		{"wsaeconnrefused", &net.OpError{Op: "dial", Err: os.NewSyscallError("connectex", syscall.Errno(10061))}, ReasonRefused},
		{"context deadline", context.DeadlineExceeded, ReasonTimeout},
		{"wrapped deadline", fmt.Errorf("dial: %w", context.DeadlineExceeded), ReasonTimeout},
		{"os deadline", &net.OpError{Op: "read", Err: os.ErrDeadlineExceeded}, ReasonTimeout},
		{"dns timeout", &net.DNSError{Err: "i/o timeout", IsTimeout: true}, ReasonTimeout},
		{"dns not found", &net.DNSError{Err: "no such host", IsNotFound: true}, ReasonDNS},
		{"eof", io.EOF, ReasonRemoteClosed},
		{"canceled", context.Canceled, ReasonRemoteClosed},
		{"other", errors.New("boom"), ReasonRemoteClosed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReasonOf(tc.err); got != tc.want {
				t.Fatalf("ReasonOf(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestCodeFor(t *testing.T) {
	tests := map[string]string{
		ReasonRefused:         CodeBusy,
		ReasonLimit:           CodeBusy,
		ReasonTimeout:         CodeNoAnswer,
		ReasonDNS:             CodeNoDialtone,
		ReasonBlocked:         CodeNoDialtone,
		ReasonRemoteClosed:    CodeNoCarrier,
		ReasonIdle:            CodeNoCarrier,
		ReasonMaxDuration:     CodeNoCarrier,
		ReasonDisabled:        CodeNoCarrier,
		ReasonHostKeyMismatch: CodeNoCarrier,
		ReasonHostKeyRejected: CodeNoCarrier,
		ReasonServerShutdown:  CodeNoCarrier,
		"":                    CodeNoCarrier,
		"unknown":             CodeNoCarrier,
	}
	for reason, want := range tests {
		if got := CodeFor(reason); got != want {
			t.Errorf("CodeFor(%q) = %q, want %q", reason, got, want)
		}
	}
}

func TestDialErrorFormatting(t *testing.T) {
	inner := errors.New("port 25 is blocked")
	err := &DialError{Reason: ReasonBlocked, Err: inner}
	if got := err.Error(); got != "retronet: blocked: port 25 is blocked" {
		t.Fatalf("Error() = %q", got)
	}
	if !errors.Is(err, inner) {
		t.Fatal("errors.Is does not reach the wrapped error")
	}
	if got := (&DialError{Reason: ReasonLimit}).Error(); got != "retronet: limit" {
		t.Fatalf("Error() without cause = %q", got)
	}
}

func TestDialEntryNeverDialsWithAnEndedContext(t *testing.T) {
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		ctx  context.Context
		host string
		want string
	}{
		{"canceled, resolved host", canceled, "bbs.example.org", ReasonOf(context.Canceled)},
		{"canceled, literal host", canceled, "1.1.1.1", ReasonOf(context.Canceled)},
		{"deadline passed, resolved host", expired, "bbs.example.org", ReasonTimeout},
		{"deadline passed, literal host", expired, "1.1.1.1", ReasonTimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &fakeResolver{answers: [][]string{{"1.1.1.1", "8.8.8.8"}}}
			dial := &fakeDial{}
			defer dial.close()
			conn, target, err := Dialer{Resolver: resolver, Dial: dial.dial}.DialEntry(tc.ctx, telnetTestEntry(tc.host, 23))
			if conn != nil {
				conn.Close()
			}
			requireReason(t, err, tc.want)
			if got := dial.dialed(); len(got) != 0 {
				t.Fatalf("dialed %v with an ended context, want no dial", got)
			}
			if target != "" {
				t.Fatalf("target = %q, want empty (no attempt was made)", target)
			}
		})
	}
}

// ignoringResolver sleeps past the dial budget and then answers, like a
// resolver that does not honour its context.
type ignoringResolver struct{ delay time.Duration }

func (r ignoringResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	time.Sleep(r.delay)
	return []net.IPAddr{{IP: net.ParseIP("1.1.1.1")}}, nil
}

func TestDialEntryResolveUsingUpTheBudgetStartsNoAttempt(t *testing.T) {
	dial := &fakeDial{}
	defer dial.close()
	_, target, err := Dialer{Resolver: ignoringResolver{delay: 120 * time.Millisecond}, Dial: dial.dial, Timeout: 30 * time.Millisecond}.DialEntry(context.Background(), telnetTestEntry("slow-dns.example.org", 23))
	requireReason(t, err, ReasonTimeout)
	if got := dial.dialed(); len(got) != 0 {
		t.Fatalf("dialed %v after the budget was used up, want no dial", got)
	}
	if target != "" {
		t.Fatalf("target = %q, want empty", target)
	}
}
