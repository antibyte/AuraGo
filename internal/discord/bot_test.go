package discord

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestStatusReportsDisabledAndMissingToken(t *testing.T) {
	resetStatusForTest()

	disabled := Status(&config.Config{})
	if disabled.Status != "disabled" || disabled.Connected {
		t.Fatalf("disabled status = %+v, want disabled and disconnected", disabled)
	}

	cfg := &config.Config{}
	cfg.Discord.Enabled = true
	missing := Status(cfg)
	if missing.Status != "missing_token" || missing.Connected || missing.TokenPresent {
		t.Fatalf("missing-token status = %+v, want missing_token and disconnected", missing)
	}
}

func TestStatusAnnotatesDisallowedIntentErrors(t *testing.T) {
	resetStatusForTest()
	setErrorForTest("websocket closed: 4014: Disallowed intent(s)")

	cfg := &config.Config{}
	cfg.Discord.Enabled = true
	cfg.Discord.BotToken = "token"

	st := Status(cfg)
	if st.Status != "error" {
		t.Fatalf("status = %q, want error", st.Status)
	}
	if !strings.Contains(strings.ToLower(st.Message), "message content intent") {
		t.Fatalf("message = %q, want message content intent hint", st.Message)
	}
}
