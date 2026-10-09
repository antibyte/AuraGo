package retronet

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// sessionTestClient is a scripted browser: buffered events in, recorded output and controls out.
type sessionTestClient struct {
	events    chan ClientEvent
	closeOnce sync.Once

	mu       sync.Mutex
	data     []byte
	controls []Control
}

func newSessionTestClient() *sessionTestClient {
	return &sessionTestClient{events: make(chan ClientEvent, 32)}
}

func (c *sessionTestClient) Events() <-chan ClientEvent { return c.events }

func (c *sessionTestClient) SendData(p []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = append(c.data, p...)
	return nil
}

func (c *sessionTestClient) SendControl(ctl Control) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.controls = append(c.controls, ctl)
	return nil
}

func (c *sessionTestClient) typeText(s string) { c.events <- ClientEvent{Data: []byte(s)} }

func (c *sessionTestClient) resize(cols, rows int) {
	c.events <- ClientEvent{Resize: &Size{Cols: cols, Rows: rows}}
}

func (c *sessionTestClient) decide(accept bool) { c.events <- ClientEvent{HostKeyAccept: &accept} }

// disconnect closes the event channel like a browser that went away.
func (c *sessionTestClient) disconnect() { c.closeOnce.Do(func() { close(c.events) }) }

func (c *sessionTestClient) output() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return string(c.data)
}

func (c *sessionTestClient) controlLog() []Control {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.controls)
}

// controlsOfType returns the recorded controls of one type in order.
func (c *sessionTestClient) controlsOfType(typ string) []Control {
	var out []Control
	for _, ctl := range c.controlLog() {
		if ctl.Type == typ {
			out = append(out, ctl)
		}
	}
	return out
}

// waitControl waits for the first control of type typ and returns it.
func (c *sessionTestClient) waitControl(t *testing.T, typ string) Control {
	t.Helper()
	sessWaitFor(t, func() bool { return len(c.controlsOfType(typ)) > 0 }, func() string {
		return "control " + typ + "; controls so far: " + sessFormatControls(c.controlLog())
	})
	return c.controlsOfType(typ)[0]
}

// waitOutput waits until the terminal output contains want.
func (c *sessionTestClient) waitOutput(t *testing.T, want string) {
	t.Helper()
	sessWaitFor(t, func() bool { return strings.Contains(c.output(), want) }, func() string {
		return "output " + strconv.Quote(want) + "; output so far: " + strconv.Quote(c.output())
	})
}

// sessWaitFor polls cond every 5 ms for up to 5 s.
func sessWaitFor(t *testing.T, cond func() bool, describe func() string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", describe())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// sessStart runs m.Run in the background; the channel receives its result.
func sessStart(ctx context.Context, m *Manager, e Entry, size Size, c Client) <-chan Result {
	done := make(chan Result, 1)
	go func() { done <- m.Run(ctx, e, size, c) }()
	return done
}

// sessAwait waits up to 5 s for a result from sessStart.
func sessAwait(t *testing.T, done <-chan Result) Result {
	t.Helper()
	select {
	case res := <-done:
		return res
	case <-time.After(5 * time.Second):
		t.Fatal("session did not end within 5s")
		return Result{}
	}
}

// sessExpectResult checks the returned result and that exactly one matching "result"
// control was sent, as the last control.
func sessExpectResult(t *testing.T, c *sessionTestClient, res Result, code, reason string) {
	t.Helper()
	if res.Code != code || res.Reason != reason {
		t.Fatalf("result = %s/%s, want %s/%s", res.Code, res.Reason, code, reason)
	}
	results := c.controlsOfType(controlResult)
	if len(results) != 1 {
		t.Fatalf("result controls = %d, want exactly 1; controls: %s", len(results), sessFormatControls(c.controlLog()))
	}
	if results[0].Code != code || results[0].Reason != reason {
		t.Fatalf("result control = %s/%s, want %s/%s", results[0].Code, results[0].Reason, code, reason)
	}
	if log := c.controlLog(); log[len(log)-1].Type != controlResult {
		t.Fatalf("result is not the last control: %s", sessFormatControls(log))
	}
}

// echoStates returns the echo controls as "remote/hidden" pairs such as "false/true"; a
// missing field shows as "nil".
func (c *sessionTestClient) echoStates() []string {
	var out []string
	for _, ctl := range c.controlsOfType(controlEcho) {
		out = append(out, sessBoolText(ctl.Remote)+"/"+sessBoolText(ctl.Hidden))
	}
	return out
}

func sessBoolText(b *bool) string {
	if b == nil {
		return "nil"
	}
	return strconv.FormatBool(*b)
}

func sessFormatControls(controls []Control) string {
	parts := make([]string, 0, len(controls))
	for _, ctl := range controls {
		part := ctl.Type
		if ctl.Type == controlEcho {
			part += "(" + sessBoolText(ctl.Remote) + "/" + sessBoolText(ctl.Hidden) + ")"
		}
		if ctl.Code != "" {
			part += "(" + ctl.Code + "/" + ctl.Reason + ")"
		}
		parts = append(parts, part)
	}
	return "[" + strings.Join(parts, " ") + "]"
}
