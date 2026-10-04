package logger

import (
	"aurago/internal/security"
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestLoggerScrubsMessagesErrorsAndGroups(t *testing.T) {
	const token = "audit-fixture-telegram-token-20261004"
	security.RegisterSensitive(token)
	var out bytes.Buffer
	l := buildLogger(&out, true).With("context", token)
	l.Error(token, "error", fmt.Errorf("GET https://api.telegram.org/bot%s/sendMessage failed", token), slog.Group("nested", "value", token))
	if strings.Contains(out.String(), token) {
		t.Fatal("registered secret leaked through logger")
	}
	if !strings.Contains(out.String(), "sendMessage") {
		t.Fatal("useful error context lost")
	}
}
