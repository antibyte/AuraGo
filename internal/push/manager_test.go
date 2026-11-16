package push

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/security"
	"github.com/SherClockHolmes/webpush-go"
)

type pushRoundTripper func(*http.Request) (*http.Response, error)

func (f pushRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func pushFixture(t *testing.T) (*Manager, map[string]string, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	vault, err := security.NewVault(strings.Repeat("2", 64), t.TempDir()+"/vault.bin")
	if err != nil {
		t.Fatal(err)
	}
	previous := GlobalManager
	mgr, err := NewManager(t.TempDir()+"/push.db", vault, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mgr.Close(); GlobalManager = previous })
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{"auth": base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")), "p256dh": base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())}
	return mgr, keys, &logs
}

func TestPushTransientFailuresRetainSubscriptionAndDoNotCountSuccess(t *testing.T) {
	mgr, keys, _ := pushFixture(t)
	attempts := 0
	mgr.sendNotification = func(payload []byte, sub *webpush.Subscription, opts *webpush.Options) (*http.Response, error) {
		client := opts.HTTPClient.(observedPushClient).client
		if client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
			t.Fatal("push redirects allowed")
		}
		if client.Transport.(*http.Transport).Proxy != nil {
			t.Fatal("push bypasses pinned DNS through proxy")
		}
		client.Transport = pushRoundTripper(func(r *http.Request) (*http.Response, error) {
			attempts++
			status, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		})
		return webpush.SendNotification(payload, sub, opts)
	}
	for _, status := range []int{201, 404, 410, 429, 503} {
		if err := mgr.Subscribe(PushSubscription{Endpoint: fmt.Sprintf("https://8.8.8.8/%d", status), Keys: keys}); err != nil {
			t.Fatal(err)
		}
	}
	// Legacy database rows are revalidated before either a send or its observer.
	if _, err := mgr.db.Exec("INSERT INTO subscriptions(endpoint,auth_key,p256dh_key) VALUES(?,?,?)", "http://127.0.0.1/private", keys["auth"], keys["p256dh"]); err != nil {
		t.Fatal(err)
	}
	observed := 0
	count, err := mgr.SendPush([]byte("fixture"), func() { observed++ })
	if err != nil || count != 1 || attempts != 5 || observed != 5 {
		t.Fatalf("success=%d attempts=%d observed=%d error=%v", count, attempts, observed, err)
	}
	if got := mgr.CountSubscriptions(); got != 4 {
		t.Fatalf("subscriptions=%d, want success + transient + invalid legacy row", got)
	}
}

func TestSubscribeRejectsInvalidUpdatesAndLogsNoEndpoint(t *testing.T) {
	mgr, keys, logs := pushFixture(t)
	sub := PushSubscription{Endpoint: "https://8.8.8.8/opaque-subscription-token", Keys: keys}
	for range 2 {
		if err := mgr.Subscribe(sub); err != nil {
			t.Fatal(err)
		}
	}
	for _, bad := range []map[string]string{
		{"auth": "bad", "p256dh": keys["p256dh"]},
		{"auth": keys["auth"], "p256dh": base64.RawURLEncoding.EncodeToString(make([]byte, 65))},
	} {
		if err := mgr.Subscribe(PushSubscription{Endpoint: sub.Endpoint, Keys: bad}); !errors.Is(err, ErrInvalidSubscription) {
			t.Fatal("bad keys accepted")
		}
	}
	var auth, public string
	if err := mgr.db.QueryRow("SELECT auth_key,p256dh_key FROM subscriptions WHERE endpoint=?", sub.Endpoint).Scan(&auth, &public); err != nil {
		t.Fatal(err)
	}
	if auth != keys["auth"] || public != keys["p256dh"] {
		t.Fatal("invalid update overwrote working keys")
	}
	if strings.Contains(logs.String(), sub.Endpoint) || strings.Contains(logs.String(), "opaque-subscription-token") || strings.Count(logs.String(), "New Web Push subscription added") != 1 {
		t.Fatal("subscription logging exposed endpoint or repeated update")
	}
	mgr.sendNotification = func(payload []byte, sub *webpush.Subscription, opts *webpush.Options) (*http.Response, error) {
		opts.HTTPClient.(observedPushClient).client.Transport = pushRoundTripper(func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("fixture failed for %s", r.URL.String())
		})
		return webpush.SendNotification(payload, sub, opts)
	}
	if count, err := mgr.SendPush([]byte("fixture")); count != 0 || err != nil {
		t.Fatalf("failed send: %d/%v", count, err)
	}
	if strings.Contains(logs.String(), "opaque-subscription-token") {
		t.Fatal("delivery error leaked endpoint")
	}
}

func TestPushRejectsUnsafeEndpointsEvenWithLoopbackOverride(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	mgr, keys, _ := pushFixture(t)
	for _, endpoint := range []string{"", "x", "http://8.8.8.8/push", "https://127.0.0.1/push", "https://10.0.0.1/push", "https://[::1]/push", "https://[64:ff9b::7f00:1]/push", "https://user:password@8.8.8.8/push", "https://8.8.8.8/push#fragment"} {
		if err := mgr.Subscribe(PushSubscription{Endpoint: endpoint, Keys: keys}); !errors.Is(err, ErrInvalidSubscription) {
			t.Errorf("unsafe endpoint accepted: %q", endpoint)
		}
	}
	if mgr.CountSubscriptions() != 0 {
		t.Fatal("unsafe endpoint stored")
	}
}

func TestPushLocalPreparationDoesNotMarkPossibleSend(t *testing.T) {
	mgr, keys, _ := pushFixture(t)
	if err := mgr.Subscribe(PushSubscription{Endpoint: "https://8.8.8.8/push", Keys: keys}); err != nil {
		t.Fatal(err)
	}
	observed := 0
	count, err := mgr.SendPush(bytes.Repeat([]byte("x"), 10000), func() { observed++ })
	if err != nil || count != 0 || observed != 0 {
		t.Fatalf("local failure marked as sent: %d/%d/%v", count, observed, err)
	}
}
