package push

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestSubscribeLogsOnlyNewEndpointOnce(t *testing.T) {
	var logs bytes.Buffer
	vault, err := security.NewVault(strings.Repeat("1", 64), t.TempDir()+"/vault.bin")
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	mgr, err := NewManager(t.TempDir()+"/push.db", vault, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	defer mgr.Close()

	sub := PushSubscription{
		Endpoint: "https://fcm.googleapis.com/fcm/send/same-endpoint",
		Keys: map[string]string{
			"auth":   "auth-key",
			"p256dh": "p256dh-key",
		},
	}
	if err := mgr.Subscribe(sub); err != nil {
		t.Fatalf("Subscribe first: %v", err)
	}
	if err := mgr.Subscribe(sub); err != nil {
		t.Fatalf("Subscribe duplicate: %v", err)
	}

	if got := strings.Count(logs.String(), "New Web Push subscription added"); got != 1 {
		t.Fatalf("new subscription log count = %d, want 1; logs:\n%s", got, logs.String())
	}
}
