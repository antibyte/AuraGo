package rocketchat

import (
	"testing"

	"aurago/internal/config"
)

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
