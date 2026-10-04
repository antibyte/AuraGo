package push

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestPushTransientFailuresRetainSubscriptionAndDoNotCountSuccess(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("2", 64), t.TempDir()+"/vault.bin")
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(t.TempDir()+"/push.db", vault, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
		w.WriteHeader(status)
	}))
	defer server.Close()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{"auth": base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")), "p256dh": base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())}
	for _, status := range []int{201, 404, 410, 429, 503} {
		if err := mgr.Subscribe(PushSubscription{Endpoint: fmt.Sprintf("%s/%d", server.URL, status), Keys: keys}); err != nil {
			t.Fatal(err)
		}
	}
	if err := mgr.Subscribe(PushSubscription{Endpoint: "x", Keys: keys}); err != nil {
		t.Fatal(err)
	}
	count, err := mgr.SendPush([]byte("fixture"))
	if err != nil || count != 1 {
		t.Fatalf("success=%d error=%v", count, err)
	}
	if got := mgr.CountSubscriptions(); got != 4 {
		t.Fatalf("subscriptions=%d, want success + transient + malformed", got)
	}
}

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
