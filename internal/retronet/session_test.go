package retronet

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
)

// sessResolver answers every lookup with ips, or fails with err.
type sessResolver struct {
	ips []string
	err error
}

func (r sessResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make([]net.IPAddr, 0, len(r.ips))
	for _, ip := range r.ips {
		out = append(out, net.IPAddr{IP: net.ParseIP(ip)})
	}
	return out, nil
}

// sessBlockingResolver blocks every lookup until ctx is done and signals entered first.
type sessBlockingResolver struct{ entered chan struct{} }

func (r sessBlockingResolver) LookupIPAddr(ctx context.Context, _ string) ([]net.IPAddr, error) {
	r.entered <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}

// sessTelnetEntry is a public-looking Telnet entry for dial tests.
func sessTelnetEntry(host string) Entry {
	return Entry{ID: "own-dialtest01", Name: "Dial Test", Category: CategoryOwn, Protocol: ProtocolTelnet, Host: host, Port: 23, Kind: KindWorld, Charset: CharsetUTF8, Own: true}
}

func TestControlWireShape(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		ctl  Control
		want string
	}{
		{Control{Type: "connected", Protocol: "telnet", Kind: "bbs", Charset: "cp437"}, `{"type":"connected","protocol":"telnet","kind":"bbs","charset":"cp437"}`},
		{Control{Type: "echo", Remote: &no, Hidden: &no}, `{"type":"echo","remote":false,"hidden":false}`},
		{Control{Type: "echo", Remote: &no, Hidden: &yes}, `{"type":"echo","remote":false,"hidden":true}`},
		{Control{Type: "echo", Remote: &yes, Hidden: &no}, `{"type":"echo","remote":true,"hidden":false}`},
		{Control{Type: "hostkey_prompt", KeyType: "ssh-rsa", Fingerprint: "SHA256:abc"}, `{"type":"hostkey_prompt","key_type":"ssh-rsa","fingerprint":"SHA256:abc"}`},
		{Control{Type: "result", Code: "BUSY", Reason: "refused"}, `{"type":"result","code":"BUSY","reason":"refused"}`},
	}
	for _, tc := range cases {
		got, err := json.Marshal(tc.ctl)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want {
			t.Fatalf("json = %s, want %s", got, tc.want)
		}
	}
}

func TestManagerRunRejectsWhenAllSlotsAreTaken(t *testing.T) {
	var dials atomic.Int32
	m := &Manager{Dialer: Dialer{AllowRestricted: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		dials.Add(1)
		return nil, errors.New("must not dial")
	}}}
	m.active.Store(defaultMaxSessions)
	c := newSessionTestClient()
	res := m.Run(context.Background(), sessTelnetEntry("203.0.113.7"), Size{Cols: 80, Rows: 25}, c)
	sessExpectResult(t, c, res, CodeBusy, ReasonLimit)
	if len(c.controlLog()) != 1 {
		t.Fatalf("controls = %s, want only the result", sessFormatControls(c.controlLog()))
	}
	if dials.Load() != 0 {
		t.Fatalf("dialed %d times over the limit", dials.Load())
	}
	if res.Target != "" || m.Active() != defaultMaxSessions {
		t.Fatalf("target = %q, active = %d", res.Target, m.Active())
	}
}

func TestManagerRunReportsDialFailures(t *testing.T) {
	refused := func(context.Context, string, string) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	}
	timedOut := func(context.Context, string, string) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}
	}
	cases := []struct {
		name       string
		dialer     Dialer
		host       string
		code       string
		reason     string
		wantTarget string
	}{
		{name: "refused", dialer: Dialer{AllowRestricted: true, Resolver: sessResolver{ips: []string{"203.0.113.7"}}, Dial: refused}, host: "refused.example", code: CodeBusy, reason: ReasonRefused, wantTarget: "203.0.113.7:23"},
		{name: "timeout", dialer: Dialer{AllowRestricted: true, Resolver: sessResolver{ips: []string{"203.0.113.8"}}, Dial: timedOut}, host: "slow.example", code: CodeNoAnswer, reason: ReasonTimeout, wantTarget: "203.0.113.8:23"},
		{name: "blocked loopback", dialer: Dialer{}, host: "127.0.0.1", code: CodeNoDialtone, reason: ReasonBlocked},
		{name: "dns", dialer: Dialer{Resolver: sessResolver{err: &net.DNSError{Err: "no such host", Name: "nowhere.invalid", IsNotFound: true}}}, host: "nowhere.invalid", code: CodeNoDialtone, reason: ReasonDNS},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manager{Dialer: tc.dialer}
			c := newSessionTestClient()
			res := m.Run(context.Background(), sessTelnetEntry(tc.host), Size{Cols: 80, Rows: 25}, c)
			sessExpectResult(t, c, res, tc.code, tc.reason)
			if res.Target != tc.wantTarget {
				t.Fatalf("target = %q, want %q", res.Target, tc.wantTarget)
			}
			if len(c.controlsOfType(controlConnected)) != 0 {
				t.Fatal("connected sent for a failed dial")
			}
			if m.Active() != 0 {
				t.Fatalf("active = %d after the session ended", m.Active())
			}
		})
	}
}

func TestManagerRunMapsCancellationDuringDial(t *testing.T) {
	cases := []struct {
		name   string
		cause  error
		reason string
	}{
		{"disabled", ErrDisabled, ReasonDisabled},
		{"shutdown", ErrShutdown, ReasonServerShutdown},
		{"plain cancel", nil, ReasonRemoteClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := sessBlockingResolver{entered: make(chan struct{}, 1)}
			m := &Manager{Dialer: Dialer{Resolver: resolver}}
			c := newSessionTestClient()
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			done := sessStart(ctx, m, sessTelnetEntry("slow-dns.example"), Size{Cols: 80, Rows: 25}, c)
			<-resolver.entered
			cancel(tc.cause)
			res := sessAwait(t, done)
			sessExpectResult(t, c, res, CodeNoCarrier, tc.reason)
		})
	}
}
