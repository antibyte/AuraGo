package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDecodeSendTelegramAndEmailFileArgs(t *testing.T) {
	tg := decodeSendTelegramArgs(ToolCall{Action: "send_telegram", Params: map[string]interface{}{
		"message": "Hallo", "file_path": "documents/bericht.pdf",
	}})
	if tg.FilePath != "documents/bericht.pdf" || tg.Message != "Hallo" || tg.Channel != "telegram" {
		t.Fatalf("telegram args = %+v", tg)
	}
	mail := decodeEmailSendArgs(ToolCall{Action: "send_email", Params: map[string]interface{}{
		"to": "a@example.com", "body": "x", "attachments": []interface{}{"a.pdf", "b.txt"},
	}})
	if len(mail.Attachments) != 2 || mail.Attachments[1] != "b.txt" {
		t.Fatalf("email args = %+v", mail)
	}
}

func TestSendTelegramWithFileRejectsPathsOutsideTheWorkspace(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Telegram.BotToken = "123:abc"
	cfg.Telegram.UserID = 1
	out, handled := dispatchMessagingCases(context.Background(), ToolCall{Action: "send_telegram", Params: map[string]interface{}{
		"message": "x", "file_path": "../../config.yaml",
	}}, &DispatchContext{Cfg: cfg, Logger: slog.Default()})
	if !handled || !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "outside") {
		t.Fatalf("output = %q handled=%v", out, handled)
	}
}

func TestNativeSchemasDescribeFileParameters(t *testing.T) {
	schemas := BuildNativeToolSchemas("", nil, ToolFeatureFlags{TelegramEnabled: true, EmailEnabled: true}, slog.Default())
	found := map[string]string{}
	for _, s := range schemas {
		if s.Function == nil {
			continue
		}
		if s.Function.Name == "send_telegram" || s.Function.Name == "send_email" {
			found[s.Function.Name] = mustJSON(t, s.Function.Parameters)
		}
	}
	if !strings.Contains(found["send_telegram"], `"file_path"`) || !strings.Contains(found["send_email"], `"attachments"`) {
		t.Fatalf("schemas = %+v", found)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
