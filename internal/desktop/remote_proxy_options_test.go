package desktop

import (
	"testing"
	"time"
)

func TestRemoteProxyOptionsFromConfigUsesConfiguredDurations(t *testing.T) {
	t.Parallel()

	got := RemoteProxyOptionsFromConfig(Config{
		RemoteMaxSessionMinutes:  12,
		RemoteIdleTimeoutMinutes: 3,
	})

	if got.MaxSessionDuration != 12*time.Minute {
		t.Fatalf("max session duration = %s, want 12m", got.MaxSessionDuration)
	}
	if got.IdleTimeout != 3*time.Minute {
		t.Fatalf("idle timeout = %s, want 3m", got.IdleTimeout)
	}
}

func TestRemoteProxyOptionsFromConfigDefaultsInvalidDurations(t *testing.T) {
	t.Parallel()

	got := RemoteProxyOptionsFromConfig(Config{})
	if got.MaxSessionDuration != remoteProxyMaxSessionDuration {
		t.Fatalf("max session duration = %s, want %s", got.MaxSessionDuration, remoteProxyMaxSessionDuration)
	}
	if got.IdleTimeout != remoteProxyIdleTimeout {
		t.Fatalf("idle timeout = %s, want %s", got.IdleTimeout, remoteProxyIdleTimeout)
	}
}

func TestRemoteProxyHandlersAcceptCustomOptions(t *testing.T) {
	t.Parallel()

	options := RemoteProxyOptions{
		MaxSessionDuration: 2 * time.Minute,
		IdleTimeout:        30 * time.Second,
	}
	if handler := HandleSSHProxy(nil, nil, nil, options); handler == nil {
		t.Fatal("SSH proxy handler is nil")
	}
	if handler := HandleVNCProxy(nil, nil, nil, options); handler == nil {
		t.Fatal("VNC proxy handler is nil")
	}
}
