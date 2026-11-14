package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
)

func receiptPage(t *testing.T, dir string) MailReceiptPage {
	t.Helper()
	page, err := ListMailReceipts(context.Background(), dir, "", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func TestMailNotificationTelegramFileSharesTextReceipt(t *testing.T) {
	for _, documentStatus := range []int{http.StatusOK, http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(documentStatus), func(t *testing.T) {
			cfg, path := telegramTestConfig(t)
			dir := t.TempDir()
			ctx := WithMailNotificationPolicy(context.Background(), dir, "file-and-caption", true)
			var mu sync.Mutex
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				mu.Lock()
				paths = append(paths, r.URL.Path)
				mu.Unlock()
				if strings.HasSuffix(r.URL.Path, "/sendDocument") && documentStatus != http.StatusOK {
					w.WriteHeader(documentStatus)
					_, _ = w.Write([]byte(`{"ok":false,"description":"synthetic rejection"}`))
					return
				}
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			oldBase := telegramAPIBase
			telegramAPIBase = server.URL
			t.Cleanup(func() { telegramAPIBase = oldBase; server.Close() })
			out := SendTelegramFile(ctx, cfg, slog.Default(), path, "fixture", strings.Repeat("caption ", 180))
			if !strings.Contains(out, `"text_sent":true`) {
				t.Fatal("long text did not share file attempt", out)
			}
			mu.Lock()
			count := len(paths)
			mu.Unlock()
			if count != 2 {
				t.Fatalf("message/document requests = %v", paths)
			}
			receipts := receiptPage(t, dir).Receipts
			want := "sent"
			if documentStatus != http.StatusOK {
				want = "unknown"
			}
			if len(receipts) != 1 || receipts[0].Status != want {
				t.Fatalf("partial / complete receipt = %+v", receipts)
			}
			out = SendTelegramFile(ctx, cfg, slog.Default(), "missing-private-file", "fixture", "fixture")
			if documentStatus == http.StatusOK && (!strings.Contains(out, "already delivered") || strings.Contains(out, "require explicit relay")) {
				t.Fatal("wrong duplicate reason", out)
			}
			if documentStatus != http.StatusOK && !strings.Contains(out, "outcome unknown") {
				t.Fatal("partial send replay reached file open", out)
			}
			mu.Lock()
			count = len(paths)
			mu.Unlock()
			if count != 2 {
				t.Fatal("duplicate text/file request")
			}
		})
	}
}

type mailReceiptRoundTripFunc func(*http.Request) (*http.Response, error)

func (f mailReceiptRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMailNotificationAttemptKeepsTrackingWithoutStorageLookup(t *testing.T) {
	dir := t.TempDir()
	var parentConnections atomic.Int32
	ctx := httptrace.WithClientTrace(WithMailNotificationPolicy(context.Background(), dir, "bound-attempt", true), &httptrace.ClientTrace{GotConn: func(httptrace.GotConnInfo) { parentConnections.Add(1) }})
	finish, sendCtx, skipped, err := beginMailNotification(ctx, "ntfy", "high")
	if err != nil || skipped {
		t.Fatal("could not reserve", err)
	}
	defer finish(false)
	// A later storage location cannot be reopened. The already claimed sender
	// must retain its own attempt identity, not rediscover it through that path.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	sendCtx = WithMailNotificationPolicy(sendCtx, blocked, "bound-attempt", true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		connection, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = connection.Close()
		}
	}))
	defer server.Close()
	cfg := &config.Config{}
	cfg.Notifications.Ntfy.Enabled = true
	cfg.Notifications.Ntfy.URL = server.URL
	cfg.Notifications.Ntfy.Topic = "fixture"
	if err := sendNtfy(cfg, "fixture", "fixture", "high", sendCtx); err == nil {
		t.Fatal("lost acknowledgement unexpectedly succeeded")
	}
	if err := finish(false); err != nil {
		t.Fatal(err)
	}
	page := receiptPage(t, dir)
	if len(page.Receipts) != 1 || page.Receipts[0].Status != "unknown" || parentConnections.Load() == 0 {
		t.Fatalf("lost attempt or parent trace after storage became unavailable: %+v", page.Receipts)
	}
}

func TestMailNotificationSafeFailureAndExplicitResolution(t *testing.T) {
	dir := t.TempDir()
	ctx := WithMailNotificationPolicy(context.Background(), dir, "independent-event", true)
	finish, deliveryCtx, skipped, err := beginMailNotification(ctx, "discord", "high")
	if err != nil || skipped {
		t.Fatalf("reserve: %v, %v", skipped, err)
	}
	active := receiptPage(t, dir).Receipts[0]
	if active.Status != "active" || !errors.Is(ResolveMailReceipt(ctx, dir, active.ID, active.Revision, "not_sent"), ErrMailReceiptConflict) {
		t.Fatal("an active attempt must not be resolved")
	}
	if err := finish(false); err != nil || len(receiptPage(t, dir).Receipts) != 0 {
		t.Fatalf("safe non-send must release capacity: %v", err)
	}
	finish, deliveryCtx, _, err = beginMailNotification(ctx, "discord", "high")
	if err != nil {
		t.Fatal(err)
	}
	markMailNotificationPossible(deliveryCtx)
	if err := finish(false); err != nil {
		t.Fatal(err)
	}
	unknown := receiptPage(t, dir).Receipts[0]
	if unknown.Status != "unknown" || unknown.Revision == active.Revision {
		t.Fatalf("ambiguous attempt = %+v", unknown)
	}
	if _, _, _, err := beginMailNotification(ctx, "discord", "high"); err == nil {
		t.Fatal("ambiguous attempt was replayed")
	}
	if !errors.Is(ResolveMailReceipt(ctx, dir, unknown.ID, active.Revision, "sent"), ErrMailReceiptConflict) {
		t.Fatal("stale revision accepted")
	}
	if err := ResolveMailReceipt(ctx, dir, unknown.ID, unknown.Revision, "sent"); err != nil {
		t.Fatal(err)
	}
	sent := receiptPage(t, dir).Receipts[0] // Re-reading recovers a lost resolve response.
	if sent.Status != "sent" || sent.ExpiresAt == nil || sent.ExpiresAt.Before(time.Now().Add(29*24*time.Hour)) {
		t.Fatalf("confirmed metadata = %+v", sent)
	}
	if _, _, skipped, err := beginMailNotification(ctx, "discord", "high"); err != nil || !skipped {
		t.Fatal("confirmation lost its duplicate fence", err)
	}
	if err := ResolveMailReceipt(ctx, dir, sent.ID, sent.Revision, "not_sent"); err != nil {
		t.Fatal(err)
	}
	finish, deliveryCtx, _, err = beginMailNotification(ctx, "discord", "high")
	if err != nil {
		t.Fatal("explicit non-send must permit a later attempt", err)
	}
	if !errors.Is(ResolveMailReceipt(ctx, dir, sent.ID, sent.Revision, "sent"), ErrMailReceiptConflict) {
		t.Fatal("an old answer resolved a new attempt")
	}
	_ = finish(false)
}

func TestMailNotificationChannelQuotaPagingAndLegacy(t *testing.T) {
	dir := t.TempDir()
	store, err := openMailReceiptStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.release()
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < mailReceiptLimit; i++ {
		r := mailReceiptRecord{Version: 1, Channel: "ntfy", Status: "unknown", AttemptedAt: now.Add(-31 * 24 * time.Hour), Token: newMailReceiptToken()}
		data, _ := json.Marshal(r)
		if _, err := tx.Exec("INSERT INTO system_tasks VALUES(?,?,?)", fmt.Sprintf("mail_notice:%x", sha256.Sum256([]byte(fmt.Sprint(i)))), string(data), now.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct {
		name, state string
		at          time.Time
	}{
		{"old-confirmed", `"sent"`, now.Add(-31 * 24 * time.Hour)},
		{"old-uncertain", `"pending"`, now.Add(-90 * 24 * time.Hour)},
	} {
		if _, err := tx.Exec("INSERT INTO system_tasks VALUES(?,?,?)", fmt.Sprintf("mail_notice:%x", sha256.Sum256([]byte(item.name))), item.state, item.at.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	ctx := WithMailNotificationPolicy(context.Background(), dir, "new-event", true)
	if _, _, _, err := beginMailNotification(ctx, "ntfy", "high"); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("full channel admitted: %v", err)
	}
	finish, _, _, err := beginMailNotification(ctx, "telegram", "high")
	if err != nil {
		t.Fatal("one full channel must not block another", err)
	}
	_ = finish(true)
	page := receiptPage(t, dir)
	if len(page.Receipts) != 50 || page.NextCursor == "" {
		t.Fatalf("page bounds = %d, %q", len(page.Receipts), page.NextCursor)
	}
	counts := map[string]int{}
	for _, c := range page.Channels {
		counts[c.Channel] = c.Count
	}
	if counts["ntfy"] != 2000 || counts["telegram"] != 1 || counts["legacy"] != 1 {
		t.Fatalf("independent occupancy / legacy expiry = %v", counts)
	}
	seen := map[string]bool{}
	cursor := ""
	for {
		page, err := ListMailReceipts(context.Background(), dir, "ntfy", cursor, 50)
		if err != nil {
			t.Fatal(err)
		}
		for _, receipt := range page.Receipts {
			if seen[receipt.ID] || receipt.Channel != "ntfy" {
				t.Fatal("duplicate or foreign cursor item")
			}
			seen[receipt.ID] = true
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 2000 {
		t.Fatalf("paging lost records: %d", len(seen))
	}
	// Race for the last free slot with distinct event identities. Keep every
	// accepted claim active until all competitors have completed admission.
	if _, err := store.db.Exec("DELETE FROM system_tasks WHERE namespace = ?", fmt.Sprintf("mail_notice:%x", sha256.Sum256([]byte("0")))); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var claimsMu sync.Mutex
	var finishes []func(bool) error
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			claimCtx := WithMailNotificationPolicy(context.Background(), dir, fmt.Sprintf("quota-race-%d", i), true)
			finish, _, _, err := beginMailNotification(claimCtx, "ntfy", "high")
			if err != nil {
				if !strings.Contains(err.Error(), "capacity") {
					t.Error(err)
				}
				return
			}
			claimsMu.Lock()
			finishes = append(finishes, finish)
			claimsMu.Unlock()
		}(i)
	}
	wg.Wait()
	if len(finishes) != 1 {
		t.Fatalf("last-slot admissions = %d", len(finishes))
	}
	for _, finish := range finishes {
		_ = finish(false)
	}
	legacy, err := ListMailReceipts(context.Background(), dir, "legacy", "", 50)
	if err != nil || len(legacy.Receipts) != 1 || legacy.Receipts[0].Status != "unknown" {
		t.Fatalf("legacy = %+v, %v", legacy, err)
	}
	for _, limit := range []int{0, 51} {
		if _, err := ListMailReceipts(context.Background(), dir, "", "", limit); !errors.Is(err, ErrMailReceiptRequest) {
			t.Fatal("unbounded page accepted")
		}
	}
}

func TestMailNotificationConfirmedJSONExpiry(t *testing.T) {
	dir := t.TempDir()
	store, err := openMailReceiptStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.release()
	now := time.Now().UTC()
	for i, status := range []string{"sent", "unknown"} {
		record := mailReceiptRecord{Version: 1, Channel: "cyd", Status: status, AttemptedAt: now.Add(-60 * 24 * time.Hour), ExpiresAt: now.Add(-time.Hour), Token: newMailReceiptToken()}
		data, _ := json.Marshal(record)
		if _, err := store.db.Exec("INSERT INTO system_tasks VALUES(?,?,?)", fmt.Sprintf("mail_notice:%x", sha256.Sum256([]byte(fmt.Sprint(i)))), string(data), now.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	page := receiptPage(t, dir)
	if len(page.Receipts) != 1 || page.Receipts[0].Status != "unknown" {
		t.Fatalf("confirmed expiry / indefinite uncertainty = %+v", page.Receipts)
	}
}

func TestMailNotificationRestartAndConcurrentResolution(t *testing.T) {
	dir := t.TempDir()
	store, err := openMailReceiptStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := mailReceiptRecord{Version: 1, Channel: "discord", Status: "active", AttemptedAt: time.Now().Add(-90 * 24 * time.Hour), Token: newMailReceiptToken()}
	data, _ := json.Marshal(r)
	id := fmt.Sprintf("%x", sha256.Sum256([]byte("interrupted-process")))
	_, err = store.db.Exec("INSERT INTO system_tasks VALUES(?,?,?)", mailReceiptPrefix+id, string(data), time.Now().UTC().Format(time.RFC3339Nano))
	_ = store.release()
	if err != nil {
		t.Fatal(err)
	}
	receipt := receiptPage(t, dir).Receipts[0]
	if receipt.Status != "unknown" {
		t.Fatal("restart must retain an uncertain receipt")
	}
	var wg sync.WaitGroup
	errorsCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errorsCh <- ResolveMailReceipt(context.Background(), dir, id, receipt.Revision, "sent")
		}()
	}
	wg.Wait()
	close(errorsCh)
	succeeded := 0
	for err := range errorsCh {
		if err == nil {
			succeeded++
		} else if !errors.Is(err, ErrMailReceiptConflict) {
			t.Fatal(err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent resolutions = %d", succeeded)
	}
}

func TestMailNotificationHTTPAckSurvivesCancellationAndLocalFailure(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(WithMailNotificationPolicy(context.Background(), dir, "http-event", true))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	cfg := &config.Config{}
	cfg.Notifications.Ntfy.Enabled = true
	cfg.Notifications.Ntfy.URL, cfg.Notifications.Ntfy.Topic = server.URL, "private-fixture-topic"
	// A disconnected transport before GotConn must free the reservation.
	cfg.Notifications.Ntfy.URL = "http://127.0.0.1:0"
	result := SendNotificationContext(context.WithoutCancel(ctx), cfg, slog.Default(), "ntfy", "private title", "private mail body", "high", nil)
	if !strings.Contains(result, `"error"`) || len(receiptPage(t, dir).Receipts) != 0 {
		t.Fatal("preconnection failure retained a receipt", result)
	}
	// Cancel precisely after the transport receives the acknowledgement. The
	// notification can succeed while bookkeeping's original context is cancelled.
	oldClient := notifyHTTPClient
	transport := http.DefaultTransport.(*http.Transport).Clone()
	notifyHTTPClient = &http.Client{Transport: mailReceiptRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		response, err := transport.RoundTrip(r)
		if err == nil {
			cancel()
		}
		return response, err
	}), Timeout: 5 * time.Second}
	t.Cleanup(func() { notifyHTTPClient = oldClient; transport.CloseIdleConnections() })
	cfg.Notifications.Ntfy.URL = server.URL
	result = SendNotificationContext(ctx, cfg, slog.Default(), "ntfy", "private title", "private mail body", "high", nil)
	if !strings.Contains(result, `"status":"sent"`) || ctx.Err() == nil {
		t.Fatal("acknowledged send did not survive cancellation", result)
	}
	page := receiptPage(t, dir)
	if len(page.Receipts) != 1 || page.Receipts[0].Status != "sent" {
		t.Fatal("cancelled bookkeeping lost acknowledgement")
	}
	encoded, _ := json.Marshal(page)
	for _, secret := range []string{"private title", "private mail body", "private-fixture-topic", "http-event"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("content escaped metadata boundary")
		}
	}
}
