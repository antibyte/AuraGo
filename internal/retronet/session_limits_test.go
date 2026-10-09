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
