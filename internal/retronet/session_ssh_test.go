package retronet

import (
	"context"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerSSHPinnedSessionPumpsData(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	done := sessStart(context.Background(), m, f.entry(false, f.fingerprint), Size{Cols: 100, Rows: 30}, c)

	connected := c.waitControl(t, controlConnected)
	if connected.Protocol != "ssh" || connected.Charset != "utf8" || connected.Kind != "" {
		t.Fatalf("connected = %+v, want protocol ssh, charset utf8, no kind", connected)
	}
	c.waitOutput(t, "welcome to the fixture")
	c.waitOutput(t, "stderr-line")
	if term, size := f.pty(); term != "xterm-256color" || size != (sshWindow{Cols: 100, Rows: 30}) {
		t.Fatalf("pty = %q %+v, want xterm-256color 100x30", term, size)
	}
	c.typeText("ping")
	c.waitOutput(t, "ping")
	c.typeText("exit")

	res := sessAwait(t, done)
	sessExpectResult(t, c, res, CodeNoCarrier, ReasonRemoteClosed)
	if !strings.Contains(c.output(), "bye") {
		t.Fatalf("output %q misses the farewell", c.output())
	}
	if res.Target != f.address() || res.BytesOut != int64(len("pingexit")) || res.BytesIn == 0 {
		t.Fatalf("target %q, bytes in %d, bytes out %d", res.Target, res.BytesIn, res.BytesOut)
	}
	if len(c.controlsOfType(controlHostKeyPrompt)) != 0 || len(c.controlsOfType(controlEcho)) != 0 {
		t.Fatalf("unexpected controls for a pinned ssh entry: %s", sessFormatControls(c.controlLog()))
	}
}

func TestManagerSSHResizeReachesServer(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := sessStart(ctx, m, f.entry(false, f.fingerprint), Size{Cols: 80, Rows: 25}, c)
	c.waitControl(t, controlConnected)
	c.resize(120, 40)
	f.waitResize(t, sshWindow{Cols: 120, Rows: 40})
	cancel()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
}

func TestManagerSSHHostKeyMismatchIsNoCarrier(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	cases := map[string]Entry{
		"catalog pinned to another key": f.entry(false, "SHA256:"+strings.Repeat("B", 43)),
		"catalog without pin":           f.entry(false, ""),
		"own entry with another key":    f.entry(true, "SHA256:"+strings.Repeat("C", 43)),
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			m := &Manager{Dialer: Dialer{AllowRestricted: true}}
			c := newSessionTestClient()
			res := m.Run(context.Background(), entry, Size{Cols: 80, Rows: 25}, c)
			sessExpectResult(t, c, res, CodeNoCarrier, ReasonHostKeyMismatch)
			if len(c.controlLog()) != 1 || res.Target != f.address() {
				t.Fatalf("controls %s, target %q", sessFormatControls(c.controlLog()), res.Target)
			}
		})
	}
}

func TestManagerSSHFirstContactAcceptStoresKey(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	stored := make(chan [2]string, 1)
	m := &Manager{Dialer: Dialer{AllowRestricted: true}, OnHostKeyAccepted: func(_ context.Context, entryID, fingerprint string) error {
		stored <- [2]string{entryID, fingerprint}
		return nil
	}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entry := f.entry(true, "")
	done := sessStart(ctx, m, entry, Size{Cols: 100, Rows: 30}, c)

	prompt := c.waitControl(t, controlHostKeyPrompt)
	if prompt.KeyType != "ssh-ed25519" || prompt.Fingerprint != f.fingerprint {
		t.Fatalf("prompt = %+v, want ssh-ed25519 %s", prompt, f.fingerprint)
	}
	c.typeText("ignored while the prompt is open")
	c.resize(110, 35)
	c.decide(true)
	c.waitControl(t, controlConnected)
	select {
	case got := <-stored:
		if got != [2]string{entry.ID, f.fingerprint} {
			t.Fatalf("OnHostKeyAccepted(%q, %q), want (%q, %q)", got[0], got[1], entry.ID, f.fingerprint)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnHostKeyAccepted was not called")
	}
	f.waitResize(t, sshWindow{Cols: 110, Rows: 35})
	types := make([]string, 0, 2)
	for _, ctl := range c.controlLog() {
		types = append(types, ctl.Type)
	}
	if i, j := slices.Index(types, controlHostKeyPrompt), slices.Index(types, controlConnected); i < 0 || j < i {
		t.Fatalf("controls %v: hostkey_prompt must precede connected", types)
	}
	cancel()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
	if strings.Contains(c.output(), "ignored while the prompt is open") {
		t.Fatal("keystrokes typed during the prompt reached the service")
	}
}

func TestManagerSSHFirstContactRejectEndsSession(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	var calls atomic.Int32
	m := &Manager{Dialer: Dialer{AllowRestricted: true}, OnHostKeyAccepted: func(context.Context, string, string) error {
		calls.Add(1)
		return nil
	}}
	c := newSessionTestClient()
	done := sessStart(context.Background(), m, f.entry(true, ""), Size{Cols: 80, Rows: 25}, c)
	c.waitControl(t, controlHostKeyPrompt)
	c.decide(false)
	res := sessAwait(t, done)
	sessExpectResult(t, c, res, CodeNoCarrier, ReasonHostKeyRejected)
	if calls.Load() != 0 || len(c.controlsOfType(controlConnected)) != 0 {
		t.Fatalf("rejected key: OnHostKeyAccepted calls %d, controls %s", calls.Load(), sessFormatControls(c.controlLog()))
	}
}

func TestManagerSSHHostKeyDecisionTimesOut(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	m := &Manager{Dialer: Dialer{AllowRestricted: true}, decisionTimeout: 100 * time.Millisecond}
	c := newSessionTestClient()
	done := sessStart(context.Background(), m, f.entry(true, ""), Size{Cols: 80, Rows: 25}, c)
	c.waitControl(t, controlHostKeyPrompt)
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonHostKeyRejected)
}

func TestManagerSSHBrowserGoneDuringPrompt(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	done := sessStart(context.Background(), m, f.entry(true, ""), Size{Cols: 80, Rows: 25}, c)
	c.waitControl(t, controlHostKeyPrompt)
	c.disconnect()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
}

func TestManagerSSHCancelDuringPromptUsesCause(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	done := sessStart(ctx, m, f.entry(true, ""), Size{Cols: 80, Rows: 25}, c)
	c.waitControl(t, controlHostKeyPrompt)
	cancel(ErrDisabled)
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonDisabled)
}

// sessShortenSSHHandshake shortens the 10 s SSH handshake deadline for one test.
func sessShortenSSHHandshake(t *testing.T, d time.Duration) {
	t.Helper()
	previous := sshHandshakeTimeout
	sshHandshakeTimeout = d
	t.Cleanup(func() { sshHandshakeTimeout = previous })
}

// A pinned catalog entry has no host-key prompt, so the handshake deadline runs from the
// start: a server that accepts the connection and never answers ends as NO ANSWER/timeout.
func TestManagerSSHStalledPinnedHandshakeTimesOut(t *testing.T) {
	sessShortenSSHHandshake(t, 200*time.Millisecond)
	stalled := startTelnetFixture(t, telnetFixtureScript{}) // accepts, reads, never sends an SSH banner
	entry := Entry{
		ID: "sshstalled", Name: "SSH Stalled", Category: CategoryGames, Protocol: ProtocolSSH,
		Host: "127.0.0.1", Port: stalled.listener.Addr().(*net.TCPAddr).Port, User: "guest",
		HostKey: "SHA256:" + strings.Repeat("A", 43),
	}
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	c := newSessionTestClient()
	started := time.Now()
	res := m.Run(context.Background(), entry, Size{Cols: 80, Rows: 25}, c)
	sessExpectResult(t, c, res, CodeNoAnswer, ReasonTimeout)
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("session gave up after %v, want about the 200ms handshake timeout", elapsed)
	}
	if res.Target != stalled.target() || len(c.controlsOfType(controlConnected)) != 0 {
		t.Fatalf("target %q, controls %s", res.Target, sessFormatControls(c.controlLog()))
	}
}

// sessDeadlineConn records every SetDeadline call of the connection.
type sessDeadlineConn struct {
	net.Conn
	mu    sync.Mutex
	calls []sessDeadlineCall
}

type sessDeadlineCall struct{ at, deadline time.Time }

func (c *sessDeadlineConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.calls = append(c.calls, sessDeadlineCall{at: time.Now(), deadline: t})
	c.mu.Unlock()
	return c.Conn.SetDeadline(t)
}

func (c *sessDeadlineConn) recorded() []sessDeadlineCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

// The time the user needs for the host-key decision is not part of the handshake budget:
// the deadline is lifted while the prompt is open and re-armed with a full timeout once the
// user decided, so a decision that took longer than the whole timeout still connects.
func TestManagerSSHHandshakeDeadlineIsRestoredAfterHostKeyDecision(t *testing.T) {
	const handshake = 400 * time.Millisecond
	sessShortenSSHHandshake(t, handshake)
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	var recorder *sessDeadlineConn
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		recorder = &sessDeadlineConn{Conn: conn}
		return recorder, nil
	}
	m := &Manager{Dialer: Dialer{AllowRestricted: true, Dial: dial}}
	c := newSessionTestClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := sessStart(ctx, m, f.entry(true, ""), Size{Cols: 80, Rows: 25}, c)

	c.waitControl(t, controlHostKeyPrompt)
	time.Sleep(2 * handshake) // the decision alone outlasts the whole handshake budget
	decided := time.Now()
	c.decide(true)
	c.waitControl(t, controlConnected)
	c.waitOutput(t, "welcome to the fixture")

	lifted, rearmed := false, false
	for _, call := range recorder.recorded() {
		if call.at.Before(decided) && call.deadline.IsZero() {
			lifted = true
		}
		if !call.at.Before(decided) && call.deadline.Sub(call.at) >= handshake/2 {
			rearmed = true
		}
	}
	if !lifted || !rearmed {
		t.Fatalf("deadline lifted while prompting: %v, re-armed after the decision: %v; SetDeadline calls: %+v", lifted, rearmed, recorder.recorded())
	}
	cancel()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonRemoteClosed)
}
