package rocketchat

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

// Usernames can be renamed by their owner (or an admin), so an allowlist entry
// matched by username is weaker than one matched by the immutable user ID.
func TestRocketChatWarnsOnUsernameAllowlistEntries(t *testing.T) {
	var sink bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&sink, nil))
	cfg := &config.Config{}
	cfg.RocketChat.AllowedUsers = []string{"id:123", "username:alice", "legacy"}

	warnRenameableAllowlistEntries(logger, cfg)

	out := sink.String()
	if strings.Count(out, "level=WARN") != 1 || !strings.Contains(out, "entry=username:alice") || !strings.Contains(out, "prefer id:<user-id>") {
		t.Fatalf("want exactly one warning for username:alice, got:\n%s", out)
	}
	for _, quiet := range []string{"id:123", "legacy"} {
		if strings.Contains(out, "entry="+quiet) {
			t.Fatalf("%s must not warn:\n%s", quiet, out)
		}
	}

	sink.Reset()
	cfg.RocketChat.AllowedUsers = []string{"id:123"}
	warnRenameableAllowlistEntries(logger, cfg)
	if sink.Len() != 0 {
		t.Fatalf("ID-only allowlist warned:\n%s", sink.String())
	}

	// StartBot runs the check for an enabled bot (here one that then stops at
	// its missing connection settings, so nothing is contacted); a disabled
	// bot stays silent.
	cfg.RocketChat.AllowedUsers = []string{"username:alice"}
	start := func() {
		t.Helper()
		if bot := StartBot(context.Background(), cfg, logger, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); bot != nil {
			bot.Stop()
			t.Fatal("bot without connection settings started")
		}
	}
	start()
	if sink.Len() != 0 {
		t.Fatalf("disabled bot warned:\n%s", sink.String())
	}
	cfg.RocketChat.Enabled = true
	start()
	if !strings.Contains(sink.String(), "entry=username:alice") {
		t.Fatalf("enabled bot did not warn about username:alice:\n%s", sink.String())
	}
}

func TestRocketChatAllowlistSeparatesIDsAndUsernames(t *testing.T) {
	cfg := &config.Config{}
	cfg.RocketChat.AllowedUsers = []string{"id:trusted", "username:alice", "legacy"}
	for _, tc := range []struct {
		id, username string
		want         bool
	}{
		{"trusted", "other", true},
		{"other", "alice", true},
		{"legacy", "other", true},
		{"other", "trusted", false},
		{"alice", "other", false},
		{"other", "legacy", false},
	} {
		msg := message{}
		msg.User.ID, msg.User.Username = tc.id, tc.username
		got := isAllowedRocketChatUser(cfg, msg)
		if got != tc.want {
			t.Errorf("id=%q username=%q: got %v, want %v", tc.id, tc.username, got, tc.want)
		}
	}
}
