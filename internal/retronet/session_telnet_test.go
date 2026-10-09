package retronet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestManagerTelnetWorldSessionEndToEnd(t *testing.T) {
	greeting := []byte{
		tfIAC, tfDO, tfTTYPE, tfIAC, tfDO, tfNAWS,
		tfIAC, tfWILL, tfSGA, tfIAC, tfDO, tfSGA, tfIAC, tfWILL, tfECHO,
		tfIAC, tfSB, tfTTYPE, 1, tfIAC, tfSE,
	}
	greeting = append(greeting, "\x1b[1m"...)
	greeting = append(greeting, 0xC9, 0xCD, 0xBB) // CP437 box drawing
	greeting = append(greeting, " Willkommen\r\n"...)
	f := startTelnetFixture(t, telnetFixtureScript{greeting: greeting})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := sessStart(ctx, m, f.entry(KindWorld, CharsetCP437), Size{Cols: 100, Rows: 30}, c)

	f.waitReceived(t, NewTelnet("XTERM-256COLOR", 100, 30).Initial())
	f.waitReceived(t, tfTTypeIs("XTERM-256COLOR"))
	f.waitReceived(t, tfNAWSFor(100, 30))
	f.waitReceived(t, []byte{tfIAC, tfDO, tfECHO})
	c.waitOutput(t, "\x1b[1m╔═╗ Willkommen\r\n")
	sessWaitFor(t, func() bool { return len(c.controlsOfType(controlEcho)) == 2 }, func() string {
		return "second echo control; controls: " + sessFormatControls(c.controlLog())
	})

	controls := c.controlLog()
	connected := controls[0]
	if connected.Type != controlConnected || connected.Protocol != "telnet" || connected.Kind != "world" || connected.Charset != "cp437" {
		t.Fatalf("first control = %+v, want connected telnet/world/cp437", connected)
	}
	if controls[1].Type != controlEcho || !slices.Equal(c.echoStates(), []string{"false/false", "true/false"}) {
		t.Fatalf("controls = %s, want echo(false/false) right after connected, then echo(true/false)", sessFormatControls(controls))
	}

	c.resize(120, 40)
	f.waitReceived(t, tfNAWSFor(120, 40))
	c.typeText("cafés\r")
	f.waitReceived(t, []byte("caf\x82s\r\n"))
	c.typeText("x y") // NBSP is CP437 0xFF and must be sent as IAC IAC
	f.waitReceived(t, []byte{'x', tfIAC, tfIAC, 'y'})

	cancel()
	res := sessAwait(t, done)
	sessExpectResult(t, c, res, CodeNoCarrier, ReasonRemoteClosed)
	if res.Target != f.target() {
		t.Fatalf("target = %q, want %q", res.Target, f.target())
	}
	if res.BytesIn != int64(len(greeting)) {
		t.Fatalf("bytes in = %d, want %d", res.BytesIn, len(greeting))
	}
	if res.BytesOut != int64(len(f.receivedBytes())) {
		t.Fatalf("bytes out = %d, service received %d", res.BytesOut, len(f.receivedBytes()))
	}
	if res.Duration <= 0 || m.Active() != 0 {
		t.Fatalf("duration = %v, active = %d", res.Duration, m.Active())
	}
}

func TestManagerTelnetBBSPinsGeometryAndMapsEnter(t *testing.T) {
	greeting := []byte{tfIAC, tfDO, tfTTYPE, tfIAC, tfDO, tfNAWS, tfIAC, tfSB, tfTTYPE, 1, tfIAC, tfSE, 0xB0, 0xB1, 0xB2, '\r', '\n'}
	f := startTelnetFixture(t, telnetFixtureScript{greeting: greeting})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := sessStart(ctx, m, f.entry(KindBBS, CharsetCP437), Size{Cols: 132, Rows: 50}, c)

	f.waitReceived(t, tfTTypeIs("ANSI"))
	f.waitReceived(t, tfNAWSFor(80, 25))
	c.waitOutput(t, "░▒▓\r\n")
	if got := c.waitControl(t, controlConnected); got.Kind != "bbs" || got.Charset != "cp437" {
		t.Fatalf("connected = %+v, want kind bbs, charset cp437", got)
	}
	c.typeText("\r")
	c.resize(100, 40)
	c.typeText("q")
	f.waitReceived(t, []byte("\r\x00q"))
	if bytes.Contains(f.receivedBytes(), tfNAWSFor(100, 40)) {
		t.Fatal("BBS entry sent NAWS for a browser resize")
	}
	if !slices.Equal(c.echoStates(), []string{"false/false"}) {
		t.Fatalf("controls = %s, want a single echo(false/false)", sessFormatControls(c.controlLog()))
	}
	cancel()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
}

func TestManagerTelnetRemoteCloseIsNoCarrier(t *testing.T) {
	greeting := []byte("Goodbye!\r\n")
	f := startTelnetFixture(t, telnetFixtureScript{greeting: greeting, hangUp: true})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	res := m.Run(context.Background(), f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 24}, c)
	sessExpectResult(t, c, res, CodeNoCarrier, ReasonRemoteClosed)
	if out := c.output(); out != "Goodbye!\r\n" {
		t.Fatalf("output = %q", out)
	}
	if res.Target != f.target() || res.BytesIn != int64(len(greeting)) {
		t.Fatalf("target = %q, bytes in = %d", res.Target, res.BytesIn)
	}
	if want := int64(len(NewTelnet("XTERM-256COLOR", 80, 24).Initial())); res.BytesOut != want {
		t.Fatalf("bytes out = %d, want the initial negotiation (%d)", res.BytesOut, want)
	}
}

func TestManagerRunMapsCancellationCauses(t *testing.T) {
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
			f := startTelnetFixture(t, telnetFixtureScript{})
			m := &Manager{Dialer: Dialer{AllowRestricted: true}}
			c := newSessionTestClient()
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			done := sessStart(ctx, m, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, c)
			c.waitControl(t, controlConnected)
			cancel(tc.cause)
			sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, tc.reason)
		})
	}
}

func TestManagerRunEndsWhenBrowserDisconnects(t *testing.T) {
	f := startTelnetFixture(t, telnetFixtureScript{})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	done := sessStart(context.Background(), m, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, c)
	c.waitControl(t, controlConnected)
	c.disconnect()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
}

func TestManagerRunRejectsSessionsOverLimit(t *testing.T) {
	f := startTelnetFixture(t, telnetFixtureScript{})
	var dials atomic.Int32
	m := &Manager{MaxSessions: 1, Dialer: Dialer{AllowRestricted: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		dials.Add(1)
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}}
	first := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := sessStart(ctx, m, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, first)
	first.waitControl(t, controlConnected)
	if m.Active() != 1 {
		t.Fatalf("active = %d, want 1", m.Active())
	}

	second := newSessionTestClient()
	res := m.Run(ctx, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, second)
	sessExpectResult(t, second, res, CodeBusy, ReasonLimit)
	if len(second.controlLog()) != 1 || dials.Load() != 1 {
		t.Fatalf("second session: controls %s, dials %d", sessFormatControls(second.controlLog()), dials.Load())
	}

	cancel()
	sessExpectResult(t, first, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
	if m.Active() != 0 {
		t.Fatalf("active = %d after both sessions ended", m.Active())
	}
}

// sessTrackingConn records Close and the number of Read calls in progress.
type sessTrackingConn struct {
	net.Conn
	reading atomic.Int32
	closed  atomic.Bool
}

func (c *sessTrackingConn) Read(p []byte) (int, error) {
	c.reading.Add(1)
	defer c.reading.Add(-1)
	return c.Conn.Read(p)
}

func (c *sessTrackingConn) Close() error {
	c.closed.Store(true)
	return c.Conn.Close()
}

func TestManagerRunClosesConnectionAndStopsReader(t *testing.T) {
	f := startTelnetFixture(t, telnetFixtureScript{greeting: []byte("hello\r\n")})
	var tracked atomic.Pointer[sessTrackingConn]
	m := &Manager{Dialer: Dialer{AllowRestricted: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		wrapped := &sessTrackingConn{Conn: conn}
		tracked.Store(wrapped)
		return wrapped, nil
	}}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	done := sessStart(ctx, m, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, c)
	c.waitOutput(t, "hello")
	cancel()
	sessAwait(t, done)
	conn := tracked.Load()
	if conn == nil {
		t.Fatal("dial was not observed")
	}
	if !conn.closed.Load() {
		t.Fatal("connection still open after Run returned")
	}
	if n := conn.reading.Load(); n != 0 {
		t.Fatalf("%d reads still running after Run returned", n)
	}
	if _, err := conn.Conn.Write([]byte("x")); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("write after Run = %v, want net.ErrClosed", err)
	}
}

func TestManagerRunReportsTheAddressThatConnected(t *testing.T) {
	f := startTelnetFixture(t, telnetFixtureScript{greeting: []byte("hi\r\n"), hangUp: true})
	port := strconv.Itoa(f.listener.Addr().(*net.TCPAddr).Port)
	v4, v6 := net.JoinHostPort("203.0.113.9", port), net.JoinHostPort("2001:db8::9", port)
	var mu sync.Mutex
	var dialed []string
	m := &Manager{Dialer: Dialer{
		AllowRestricted: true,
		Resolver:        sessResolver{ips: []string{"2001:db8::9", "203.0.113.9"}},
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			mu.Lock()
			dialed = append(dialed, address)
			mu.Unlock()
			if address != v6 {
				return nil, &net.OpError{Op: "dial", Net: network, Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
			}
			return (&net.Dialer{}).DialContext(ctx, network, f.target())
		},
	}}
	entry := f.entry(KindWorld, CharsetUTF8)
	entry.Host = "fallback.example"
	c := newSessionTestClient()
	res := m.Run(context.Background(), entry, Size{Cols: 80, Rows: 25}, c)
	sessExpectResult(t, c, res, CodeNoCarrier, ReasonRemoteClosed)
	if res.Target != v6 || c.output() != "hi\r\n" {
		t.Fatalf("target %q, output %q; want the fallback address %q and the greeting", res.Target, c.output(), v6)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(dialed) != 2 || dialed[0] != v4 || dialed[1] != v6 {
		t.Fatalf("dial order = %q, want IPv4 %q first, then IPv6 %q", dialed, v4, v6)
	}
}

func TestManagerTelnetEchoControlsFollowNegotiation(t *testing.T) {
	f := startTelnetFixture(t, telnetFixtureScript{})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := sessStart(ctx, m, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, c)
	f.waitReceived(t, NewTelnet("XTERM-256COLOR", 80, 25).Initial()) // the fixture holds the connection now
	steps := []struct {
		name string
		send []byte
		want []string // "remote/hidden" of every echo control so far
	}{
		{"initial", nil, []string{"false/false"}},
		{"WILL ECHO only: password prompt in line mode", []byte{tfIAC, tfWILL, tfECHO}, []string{"false/false", "false/true"}},
		{"WILL SGA: kludge character mode", []byte{tfIAC, tfWILL, tfSGA}, []string{"false/false", "false/true", "true/false"}},
		{"WONT ECHO: local line editing again", []byte{tfIAC, tfWONT, tfECHO}, []string{"false/false", "false/true", "true/false", "false/false"}},
	}
	for _, step := range steps {
		if step.send != nil {
			f.write(step.send)
		}
		sessWaitFor(t, func() bool { return slices.Equal(c.echoStates(), step.want) }, func() string {
			return fmt.Sprintf("%s: echo states %v, have %v", step.name, step.want, c.echoStates())
		})
	}
	f.waitReceived(t, []byte{tfIAC, tfDO, tfECHO})
	f.waitReceived(t, []byte{tfIAC, tfDONT, tfECHO})
	cancel()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
}

func TestManagerTelnetStalledServiceHitsWriteDeadline(t *testing.T) {
	previous := sessionWriteTimeout
	sessionWriteTimeout = 100 * time.Millisecond
	t.Cleanup(func() { sessionWriteTimeout = previous })
	clientEnd, serviceEnd := net.Pipe() // synchronous: a write blocks until the other side reads
	defer serviceEnd.Close()
	m := &Manager{Dialer: Dialer{AllowRestricted: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		return clientEnd, nil
	}}}
	entry := Entry{ID: "own-stalled02", Name: "Stalled", Category: CategoryOwn, Protocol: ProtocolTelnet, Host: "127.0.0.1", Port: 2323, Kind: KindWorld, Charset: CharsetUTF8, Own: true}
	initial := NewTelnet("XTERM-256COLOR", 80, 25).Initial()
	served := make(chan error, 1)
	go func() {
		if _, err := io.ReadFull(serviceEnd, make([]byte, len(initial))); err != nil {
			served <- err
			return
		}
		// Ask for the terminal type, then never read again: the TTYPE reply cannot be written.
		_, err := serviceEnd.Write([]byte{tfIAC, tfDO, tfTTYPE, tfIAC, tfSB, tfTTYPE, 1, tfIAC, tfSE})
		served <- err
	}()
	c := newSessionTestClient()
	started := time.Now()
	res := m.Run(context.Background(), entry, Size{Cols: 80, Rows: 25}, c)
	sessExpectResult(t, c, res, CodeNoCarrier, ReasonRemoteClosed)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("stalled reply write ended the session after %v, want about the 100ms write deadline", elapsed)
	}
	if err := <-served; err != nil {
		t.Fatalf("fake service: %v", err)
	}
	if res.BytesOut != int64(len(initial)) {
		t.Fatalf("bytes out = %d, want only the initial negotiation (%d)", res.BytesOut, len(initial))
	}
}
