package retronet

import (
	"context"
	"testing"
	"time"
)

func TestManagerIdleTimeoutIgnoresServiceOutput(t *testing.T) {
	f := startTelnetFixture(t, telnetFixtureScript{greeting: []byte("tick")})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}, IdleTimeout: 150 * time.Millisecond}
	c := newSessionTestClient()
	started := time.Now()
	done := sessStart(context.Background(), m, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, c)
	c.waitOutput(t, "tick")
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	resizes := time.NewTicker(25 * time.Millisecond)
	defer resizes.Stop()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case res := <-done:
			sessExpectResult(t, c, res, CodeNoCarrier, ReasonIdle)
			if elapsed := time.Since(started); elapsed < 150*time.Millisecond {
				t.Fatalf("idle timeout after %v, want >= 150ms", elapsed)
			}
			return
		case <-ticker.C:
			f.write([]byte(".")) // service output must not keep the session alive
		case <-resizes.C:
			c.resize(100, 30) // neither does a window resize: only keystrokes count as activity
		case <-deadline.C:
			t.Fatal("idle session did not end")
		}
	}
}

func TestManagerUserInputResetsIdleTimer(t *testing.T) {
	f := startTelnetFixture(t, telnetFixtureScript{})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}, IdleTimeout: 300 * time.Millisecond}
	c := newSessionTestClient()
	done := sessStart(context.Background(), m, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, c)
	c.waitControl(t, controlConnected)
	typing := time.NewTicker(50 * time.Millisecond)
	stopTyping := time.NewTimer(900 * time.Millisecond)
	for typingDone := false; !typingDone; {
		select {
		case res := <-done:
			t.Fatalf("session ended while the user was typing: %s/%s", res.Code, res.Reason)
		case <-typing.C:
			c.typeText("a")
		case <-stopTyping.C:
			typingDone = true
		}
	}
	typing.Stop()
	sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonIdle)
}

// The browser's automatic terminal reports reach the service but do not keep an otherwise
// idle session alive; a mouse report is user input and does.
func TestManagerIdleTimeoutIgnoresTerminalReports(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		alive bool
	}{
		{"terminal reports", "\x1b[12;40R\x1b[I", false},
		{"mouse reports", "\x1b[<0;10;5M", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := startTelnetFixture(t, telnetFixtureScript{})
			m := &Manager{Dialer: Dialer{AllowRestricted: true}, IdleTimeout: 200 * time.Millisecond}
			c := newSessionTestClient()
			started := time.Now()
			done := sessStart(context.Background(), m, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, c)
			c.waitControl(t, controlConnected)
			sending := time.NewTicker(40 * time.Millisecond)
			stopSending := time.NewTimer(700 * time.Millisecond)
			for sendingDone := false; !sendingDone; {
				select {
				case res := <-done:
					if tc.alive {
						t.Fatalf("session ended while the user sent %s: %s/%s", tc.name, res.Code, res.Reason)
					}
					sessExpectResult(t, c, res, CodeNoCarrier, ReasonIdle)
					if elapsed := time.Since(started); elapsed > 600*time.Millisecond {
						t.Fatalf("idle timeout after %v despite only terminal reports, want about 200ms", elapsed)
					}
					f.waitReceived(t, []byte(tc.input))
					return
				case <-sending.C:
					c.typeText(tc.input)
				case <-stopSending.C:
					sendingDone = true
				}
			}
			sending.Stop()
			if !tc.alive {
				t.Fatal("a session that received only terminal reports did not go idle")
			}
			sessExpectResult(t, c, sessAwait(t, done), CodeNoCarrier, ReasonIdle)
			f.waitReceived(t, []byte(tc.input))
		})
	}
}

func TestManagerMaxDurationEndsSession(t *testing.T) {
	f := startTelnetFixture(t, telnetFixtureScript{})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}, IdleTimeout: time.Hour, MaxDuration: 200 * time.Millisecond}
	c := newSessionTestClient()
	done := sessStart(context.Background(), m, f.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, c)
	c.waitControl(t, controlConnected)
	res := sessAwait(t, done)
	sessExpectResult(t, c, res, CodeNoCarrier, ReasonMaxDuration)
	if res.Duration < 200*time.Millisecond {
		t.Fatalf("duration = %v, want >= 200ms", res.Duration)
	}
}

// The spec limits are the defaults of a zero Manager; a positive setting overrides them and a
// negative one falls back to the default.
func TestManagerLimitDefaultsMatchTheSpec(t *testing.T) {
	zero := &Manager{}
	defaults := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"idle timeout", zero.idleTimeout(), 30 * time.Minute},
		{"maximum duration", zero.maxDuration(), 4 * time.Hour},
		{"host key decision", zero.hostKeyTimeout(), 60 * time.Second},
		{"write timeout", sessionWriteTimeout, 10 * time.Second},
		{"ssh handshake", sshHandshakeTimeout, 10 * time.Second},
	}
	for _, tc := range defaults {
		if tc.got != tc.want {
			t.Errorf("default %s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
	if zero.maxSessions() != 4 {
		t.Errorf("default session limit = %d, want 4", zero.maxSessions())
	}

	set := &Manager{IdleTimeout: 5 * time.Second, MaxDuration: 6 * time.Second, MaxSessions: 2, decisionTimeout: 7 * time.Second}
	if set.idleTimeout() != 5*time.Second || set.maxDuration() != 6*time.Second || set.hostKeyTimeout() != 7*time.Second || set.maxSessions() != 2 {
		t.Errorf("configured limits ignored: idle %v, max %v, decision %v, sessions %d",
			set.idleTimeout(), set.maxDuration(), set.hostKeyTimeout(), set.maxSessions())
	}
	negative := &Manager{IdleTimeout: -time.Second, MaxDuration: -time.Second, MaxSessions: -1}
	if negative.idleTimeout() != 30*time.Minute || negative.maxDuration() != 4*time.Hour || negative.maxSessions() != 4 {
		t.Errorf("negative limits must fall back to the defaults: idle %v, max %v, sessions %d",
			negative.idleTimeout(), negative.maxDuration(), negative.maxSessions())
	}
}
