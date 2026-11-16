package tsnetnode

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"aurago/internal/config"
	"tailscale.com/tsnet"
)

func waitDisabled(t *testing.T, m *Manager) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		m.operationMu.Lock()
		pending := m.disablePending
		m.operationMu.Unlock()
		if !pending {
			return
		}
		select {
		case <-deadline:
			t.Fatal("disable did not finish")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestDisableCancelsOperationsAndClosesAllListeners(t *testing.T) {
	for _, kind := range []string{"start", "reauth_login", "reconfigure"} {
		t.Run(kind, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Tailscale.TsNet.Enabled = true
			m := NewManager(cfg, nil)
			listeners := make([]*blockingListener, 6)
			for i := range listeners {
				listeners[i] = newBlockingListener("fixture:443")
			}
			m.listener, m.homepageLn, m.speechLabLn = listeners[0], listeners[1], listeners[2]
			m.manifest.Listener, m.spaceAgent.Listener = listeners[3], listeners[4]
			m.storeProxyLns = map[string]net.Listener{"test": listeners[5]}
			m.running, m.servingHTTP, m.funnelActive = true, true, true
			ctx, id, err := m.beginOperation(kind, NodeMain)
			if err != nil {
				t.Fatal(err)
			}
			cfg.Tailscale.TsNet.Enabled = false
			m.UpdateConfig(cfg)
			m.UpdateConfig(cfg)
			if ctx.Err() != context.Canceled {
				t.Fatal("active operation not cancelled")
			}
			if _, err := m.BeginStart(http.NotFoundHandler()); err == nil {
				t.Fatal("disabled start accepted")
			}
			cfg.Tailscale.TsNet.Enabled = true
			m.UpdateConfig(cfg)
			if _, err := m.BeginStart(http.NotFoundHandler()); err == nil {
				t.Fatal("start accepted before disable drained")
			}
			m.finishOperation(id, ctx.Err())
			waitDisabled(t, m)
			for _, ln := range listeners {
				select {
				case <-ln.closed:
				default:
					t.Fatal("listener survived disable")
				}
			}
			if m.running || m.servingHTTP || m.funnelActive {
				t.Fatal("resources still published")
			}
			op := m.operationSnapshot()
			if op == nil || op.Type != "stop" || op.State != "succeeded" {
				t.Fatalf("stop status: %+v", op)
			}
			// Re-enabling only permits a future explicit start.
			ctx, id, err = m.beginOperation("start", NodeMain)
			if err != nil {
				t.Fatal(err)
			}
			m.finishOperation(id, nil)
		})
	}
}

func TestCancelledMainListenerCannotPublishAfterReenable(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Enabled = true
	m := NewManager(cfg, nil)
	srv := &tsnet.Server{}
	m.server = srv
	ctx, cancel := context.WithCancel(context.Background())
	ln := newBlockingListener("late:443")
	m.listenTLSForNode = func(context.Context, *tsnet.Server, string, time.Duration, bool) (net.Listener, error) {
		cancel()
		return ln, nil
	}
	if err := m.startMainListener(ctx, srv, http.NotFoundHandler()); err == nil {
		t.Fatal("cancelled listener accepted")
	}
	select {
	case <-ln.closed:
	default:
		t.Fatal("late listener not closed")
	}
	if m.listener != nil || m.httpSrv != nil || m.servingHTTP {
		t.Fatal("late listener published")
	}
}

func TestDisableRepeatedSavesAreIdempotent(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Enabled = true
	m := NewManager(cfg, nil)
	cfg.Tailscale.TsNet.Enabled = false
	m.UpdateConfig(cfg)
	waitDisabled(t, m)
	id := m.operationSnapshot().ID
	for range 3 {
		m.UpdateConfig(cfg)
	}
	if m.operationSnapshot().ID != id {
		t.Fatal("repeated disable scheduled another stop")
	}
	cfg.Tailscale.TsNet.Enabled = true
	m.UpdateConfig(cfg)
	if m.operationSnapshot().ID != id || m.running {
		t.Fatal("enable started the node")
	}
}
