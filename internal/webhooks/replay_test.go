package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const replayTestSecret = "synthetic-replay-fixture"

func newReplayHandler(t *testing.T) (*Handler, *Manager, Webhook) {
	t.Helper()
	h, _, m := newSilentWebhookHandler(t, WebhookFormat{SignatureHeader: "X-Signature", SignatureAlgo: "sha256", ReplayProtection: true})
	w := m.List()[0]
	if err := h.vault.WriteSecret(SignatureSecretVaultKey(w.ID), replayTestSecret); err != nil {
		t.Fatal(err)
	}
	return h, m, w
}

func replayRequest(timestamp, nonce, body string) *http.Request {
	r := httptest.NewRequest("POST", "/webhook/test-hook", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-AuraGo-Webhook-Timestamp", timestamp)
	r.Header.Set("X-AuraGo-Webhook-Nonce", nonce)
	mac := hmac.New(sha256.New, []byte(replayTestSecret))
	mac.Write([]byte(timestamp + "." + nonce + "." + body))
	r.Header.Set("X-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return r
}

func TestReplayProtectionConcurrentDeliveryAndRestart(t *testing.T) {
	h, m, wh := newReplayHandler(t)
	var callbacks, accepted, duplicates atomic.Int32
	m.RegisterMissionTrigger(wh.ID, func([]byte) { callbacks.Add(1) })
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			h.ServeHTTP(w, replayRequest(stamp, "same_nonce_12345678", `{"ok":true}`))
			if w.Code != 200 {
				t.Errorf("response %d: %s", w.Code, w.Body)
				return
			}
			if strings.Contains(w.Body.String(), "duplicate") {
				duplicates.Add(1)
			} else {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	saved, _ := m.Get(wh.ID)
	if accepted.Load() != 1 || duplicates.Load() != 23 || callbacks.Load() != 1 || saved.FireCount != 1 {
		t.Fatalf("accepted=%d duplicate=%d callbacks=%d fires=%d", accepted.Load(), duplicates.Load(), callbacks.Load(), saved.FireCount)
	}
	restarted, err := NewManager(m.filePath, m.log.filePath)
	if err != nil {
		t.Fatal(err)
	}
	h2 := NewHandler(restarted, nil, h.vault, nil, nil, h.cfg, nil, 8080, 4096, 0)
	for i := 0; i < cap(h2.deliverySlots); i++ {
		h2.deliverySlots <- struct{}{}
	}
	w := httptest.NewRecorder()
	h2.ServeHTTP(w, replayRequest(stamp, "same_nonce_12345678", `{"ok":true}`))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "duplicate") {
		t.Fatalf("restart/full queue: %d %s", w.Code, w.Body)
	}
	data, err := os.ReadFile(m.replayPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "same_nonce") || strings.Contains(string(data), replayTestSecret) || strings.Contains(string(data), "ok") {
		t.Fatal("replay store leaked source material")
	}
}

func TestReplayProtectionRejectsInvalidFreshnessOrSignature(t *testing.T) {
	for _, kind := range []string{"old", "future", "missing", "short", "long", "characters", "wrong_signature", "tampered_body", "raw_body_signature"} {
		t.Run(kind, func(t *testing.T) {
			h, m, wh := newReplayHandler(t)
			now := time.Now().Unix()
			stamp, nonce := strconv.FormatInt(now, 10), "valid_nonce_12345678"
			switch kind {
			case "old":
				stamp = strconv.FormatInt(now-301, 10)
			case "future":
				stamp = strconv.FormatInt(now+600, 10)
			case "missing":
				stamp = ""
			case "short":
				nonce = "short"
			case "long":
				nonce = strings.Repeat("x", 129)
			case "characters":
				nonce = "invalid_nonce_@123456"
			}
			r := replayRequest(stamp, nonce, `{"ok":true}`)
			if kind == "wrong_signature" {
				r.Header.Set("X-Signature", "sha256=0000")
			}
			if kind == "tampered_body" {
				r.Body = io.NopCloser(strings.NewReader(`{"ok":false}`))
			}
			if kind == "raw_body_signature" {
				mac := hmac.New(sha256.New, []byte(replayTestSecret))
				mac.Write([]byte(`{"ok":true}`))
				r.Header.Set("X-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("response %d %s", w.Code, w.Body)
			}
			got, _ := m.Get(wh.ID)
			if got.FireCount != 0 {
				t.Fatal("invalid event fired")
			}
		})
	}
}

func TestReplayProtectionClaimsBeforeGuardianAndFailsClosed(t *testing.T) {
	h, m, wh := newReplayHandler(t)
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	h.cfg.LLMGuardian.ScanDocuments = true // unavailable scanner quarantines only the first admission
	w := httptest.NewRecorder()
	h.ServeHTTP(w, replayRequest(stamp, "quarantine_nonce_123", `{"ok":true}`))
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, replayRequest(stamp, "quarantine_nonce_123", `{"ok":true}`))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "duplicate") {
		t.Fatalf("repeated scan: %d %s", w.Code, w.Body)
	}
	h.cfg.LLMGuardian.ScanDocuments = false
	if err := os.WriteFile(m.replayPath(), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, replayRequest(stamp, "storage_nonce_123456", `{"ok":true}`))
	if w.Code != 503 {
		t.Fatal("corrupt replay store admitted event")
	}
	got, _ := m.Get(wh.ID)
	if got.FireCount != 0 {
		t.Fatal("failed storage fired event")
	}
	// Legacy signatures do not read the replay store.
	if _, err := m.UpdateWithOptions(wh.ID, Webhook{Format: WebhookFormat{ReplayProtection: false}}, UpdateOptions{ReplayProtectionSet: true}); err != nil {
		t.Fatal(err)
	}
	r := replayRequest(stamp, "storage_nonce_123456", `{"ok":true}`)
	mac := hmac.New(sha256.New, []byte(replayTestSecret))
	mac.Write([]byte(`{"ok":true}`))
	r.Header.Set("X-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("legacy response %d %s", w.Code, w.Body)
	}
}

func TestReplayStoreCapacityExpiryAndAtomicFailure(t *testing.T) {
	_, m, _ := newReplayHandler(t)
	now := time.Now()
	state := replayState{Version: 1, Claims: map[string]int64{}}
	for i := 0; i < maxReplayClaims; i++ {
		state.Claims[fmt.Sprintf("%064x", i)] = now.Add(10 * time.Minute).Unix()
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(m.replayPath(), data, 0600); err != nil {
		t.Fatal(err)
	}
	admitted := false
	if _, err := m.claimNonce(t.Context(), "new-hook", "new_nonce_12345678", now, func() bool { admitted = true; return true }); err == nil || admitted {
		t.Fatal("full cache evicted active receipt")
	}
	state.Claims[fmt.Sprintf("%064x", 0)] = now.Add(-time.Second).Unix()
	data, _ = json.Marshal(state)
	if err := os.WriteFile(m.replayPath(), data, 0600); err != nil {
		t.Fatal(err)
	}
	if dup, err := m.claimNonce(t.Context(), "new-hook", "new_nonce_12345678", now, nil); err != nil || dup {
		t.Fatalf("expiry admission %t %v", dup, err)
	}
	if dup, err := m.claimNonce(t.Context(), "new-hook", "new_nonce_12345678", now.Add(9*time.Minute), nil); err != nil || !dup {
		t.Fatalf("early eviction %t %v", dup, err)
	}
	if err := os.Remove(m.replayPath()); err != nil {
		t.Fatal(err)
	}
	_, err := m.claimNonce(context.Background(), "id", "failed_nonce_123456", now, func() bool { return os.Mkdir(m.replayPath(), 0700) == nil })
	if err == nil {
		t.Fatal("failed receipt publication accepted")
	}
}

func TestReplayProtectionRejectsIncompleteStoredState(t *testing.T) {
	for _, data := range []string{`null`, `{}`, `{"version":1}`, `{"claims":{}}`, `{"version":1,"claims":null}`, `{"version":2,"claims":{}}`} {
		t.Run(data, func(t *testing.T) {
			h, m, wh := newReplayHandler(t)
			if err := os.WriteFile(m.replayPath(), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, replayRequest(strconv.FormatInt(time.Now().Unix(), 10), "stored_state_12345678", `{"ok":true}`))
			if w.Code != 503 {
				t.Fatalf("incomplete receipt store admitted event: %d %s", w.Code, w.Body)
			}
			got, _ := m.Get(wh.ID)
			if got.FireCount != 0 || len(h.deliverySlots) != 0 {
				t.Fatal("failed receipt validation caused delivery")
			}
			if after, err := os.ReadFile(m.replayPath()); err != nil || string(after) != data {
				t.Fatalf("invalid evidence was replaced: %v", err)
			}
		})
	}
}
