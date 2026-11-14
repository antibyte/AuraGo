package tools

import (
	"aurago/internal/config"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestMailNotificationPolicySurvivesRestartAndUncertainDelivery(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.Enabled = true
	cfg.Discord.DefaultChannelID = "fixture"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	event := MailEventSessionID("fixture account folder UID 1")
	calls := 0
	send := func(_ string, _ string) error { calls++; return nil }
	run := func(event, priority string, sender DiscordSendFunc) string {
		ctx := WithMailNotificationPolicy(context.Background(), dir, event, false)
		return SendNotificationContext(ctx, cfg, logger, "discord", "fixture", "fixture", priority, sender)
	}
	if out := run(event, "normal", send); !strings.Contains(out, `"status":"skipped"`) || calls != 0 {
		t.Fatalf("routine mail sent: %s calls=%d", out, calls)
	}
	if out := run(event, "high", send); !strings.Contains(out, `"status":"sent"`) || calls != 1 {
		t.Fatalf("new urgent mail: %s calls=%d", out, calls)
	}
	// Each run reopens the SQLite store: the receipt must outlive the context/connection.
	if out := run(event, "critical", send); !strings.Contains(out, `"status":"skipped"`) || calls != 1 {
		t.Fatalf("replayed confirmed delivery: %s calls=%d", out, calls)
	}
	uncertain := MailEventSessionID("fixture account folder UID 2")
	failed := func(_ string, _ string) error { calls++; return errors.New("synthetic ambiguous send") }
	run(uncertain, "high", failed)
	if out := run(uncertain, "high", send); !strings.Contains(out, "outcome unknown") || calls != 2 {
		t.Fatalf("replayed uncertain delivery: %s calls=%d", out, calls)
	}
	if out := run(MailEventSessionID("fixture account folder UID 2 changed verdict"), "high", send); !strings.Contains(out, `"status":"sent"`) || calls != 3 {
		t.Fatalf("changed finding not admitted: %s calls=%d", out, calls)
	}
	// Human/mission contexts keep their existing normal-priority delivery.
	out := SendNotificationContext(context.Background(), cfg, logger, "discord", "fixture", "fixture", "normal", send)
	if !strings.Contains(out, `"status":"sent"`) || calls != 4 {
		t.Fatalf("ordinary context changed: %s calls=%d", out, calls)
	}
}

func TestMailNotificationClaimsAreAtomicAndChannelScoped(t *testing.T) {
	dir := t.TempDir()
	ctx := WithMailNotificationPolicy(context.Background(), dir, MailEventSessionID("fixture UID 3"), false)
	var claims atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			finish, _, skipped, err := beginMailNotification(ctx, "telegram", "high")
			if err != nil { // An in-flight, unconfirmed claim is deliberately not replayable.
				if !strings.Contains(err.Error(), "outcome unknown") {
					t.Error(err)
				}
				return
			}
			if skipped {
				return
			}
			claims.Add(1)
			if err := finish(true); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if claims.Load() != 1 {
		t.Fatalf("claims=%d, want one", claims.Load())
	}
	finish, _, skipped, err := beginMailNotification(ctx, "ntfy", "high")
	if err != nil || skipped || finish == nil {
		t.Fatalf("independent channel refused: skipped=%v err=%v", skipped, err)
	}
	if err := finish(true); err != nil {
		t.Fatal(err)
	}
}

func TestMailEventIdentityIgnoresOrderingAndSeparatesReusedUIDs(t *testing.T) {
	account := config.EmailAccount{ID: "fixture", WatchFolder: "INBOX"}
	ids := []uint32{9, 4}
	first := mailMessageIdentity(EmailMessage{UID: 9, Subject: "first fixture", Body: "fixture"})
	second := mailMessageIdentity(EmailMessage{UID: 4, Subject: "second fixture", Body: "fixture"})
	event := mailAccountEventID(account, ids, "new-mail", first, second)
	reordered := mailAccountEventID(account, []uint32{4, 9}, "new-mail", second, first)
	if event != reordered || ids[0] != 9 || !ValidMailEventSessionID(event) {
		t.Fatal("event identity depends on ordering or mutates callers")
	}
	replacement := mailMessageIdentity(EmailMessage{UID: 9, Subject: "new mail after UID reuse", Body: "fixture"})
	if event == mailAccountEventID(account, ids, "new-mail", replacement, second) {
		t.Fatal("new content under a reused UID was suppressed")
	}
	if ValidMailEventSessionID("default") || ValidMailEventSessionID("mail-"+strings.Repeat("z", 64)) {
		t.Fatal("accepted non-mail or malformed session")
	}
}

func TestMailNotificationSuppressesUnsolicitedFileBeforeOpeningIt(t *testing.T) {
	ctx := WithMailNotificationPolicy(context.Background(), t.TempDir(), MailEventSessionID("file fixture"), false)
	out := SendTelegramFile(ctx, &config.Config{}, slog.Default(), "missing-private-file", "fixture", "fixture")
	if !strings.Contains(out, `"status":"skipped"`) {
		t.Fatalf("unsolicited mail file reached target/file validation: %s", out)
	}
}

func TestMailNotificationExplicitRelayAndMissingStore(t *testing.T) {
	ctx := WithMailNotificationPolicy(context.Background(), t.TempDir(), MailEventSessionID("relay fixture"), true)
	finish, _, skipped, err := beginMailNotification(ctx, "telegram", "normal")
	if err != nil || skipped || finish == nil {
		t.Fatalf("configured relay refused: %v %v", skipped, err)
	}
	if err := finish(true); err != nil {
		t.Fatal(err)
	}
	ctx = WithMailNotificationPolicy(context.Background(), "", "fixture", false)
	if _, _, _, err := beginMailNotification(ctx, "telegram", "high"); err == nil {
		t.Fatal("missing durable store must not send")
	}
}
