package retronet

import (
	"context"
	"sync"
	"testing"
)

// sessShutdownRaceClient cancels the session with ErrShutdown and closes its events
// while the pump handles the first output, so the pump's next select sees both at once.
type sessShutdownRaceClient struct {
	*sessionTestClient
	once   sync.Once
	cancel context.CancelCauseFunc
}

func (c *sessShutdownRaceClient) SendData(p []byte) error {
	c.once.Do(func() {
		c.cancel(ErrShutdown)
		c.disconnect()
	})
	return c.sessionTestClient.SendData(p)
}

// TestManagerRunPrefersTheCancelCauseOverClosedEvents pins that a browser socket closed by
// the same shutdown that cancelled the session reports server_shutdown, whichever select
// case the pump picks. The select is random, so the scenario repeats.
func TestManagerRunPrefersTheCancelCauseOverClosedEvents(t *testing.T) {
	fixture := startTelnetFixture(t, telnetFixtureScript{greeting: []byte("HELLO\r\n")})
	m := &Manager{Dialer: Dialer{AllowRestricted: true}}
	for i := 0; i < 40; i++ {
		ctx, cancel := context.WithCancelCause(context.Background())
		client := &sessShutdownRaceClient{sessionTestClient: newSessionTestClient(), cancel: cancel}
		res := m.Run(ctx, fixture.entry(KindWorld, CharsetUTF8), Size{Cols: 80, Rows: 25}, client)
		cancel(nil)
		if res.Code != CodeNoCarrier || res.Reason != ReasonServerShutdown {
			t.Fatalf("run %d: result = %s/%s, want %s/%s", i, res.Code, res.Reason, CodeNoCarrier, ReasonServerShutdown)
		}
	}
}
