package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

// FF1: send_telegram reads its file only from file_path. A stray path argument (as a native
// call delivers it: the Path field and the parameter) neither sends a file nor fails the
// call: the message goes out as a plain message.
func TestFF1SendTelegramIgnoresAStrayPath(t *testing.T) {
	cfg, _ := c06TelegramConfig(t)
	for name, path := range map[string]interface{}{
		"outside the workspace": "../../config.yaml",
		"a list":                []interface{}{"a.pdf"},
		"a number":              float64(5),
	} {
		t.Run(name, func(t *testing.T) {
			bot := c06FakeBot(t)
			tc := ToolCall{Action: "send_telegram", Params: map[string]interface{}{"message": "ff1 Nachricht", "path": path}}
			if s, ok := path.(string); ok {
				tc.Path = s
			}
			if got := decodeSendTelegramArgs(tc).FilePath; got != "" {
				t.Fatalf("FilePath from a stray path = %q", got)
			}
			out, handled := dispatchMessagingCases(context.Background(), tc, &DispatchContext{Cfg: cfg, Logger: slog.Default()})
			if !handled || c06Result(t, out)["status"] == "error" {
				t.Fatalf("output = %s", out)
			}
			requests := bot.all()
			if len(requests) != 1 || !strings.HasSuffix(requests[0].Path, "/sendMessage") || !strings.Contains(requests[0].JSON, "ff1 Nachricht") {
				t.Fatalf("requests = %+v, want one plain message", requests)
			}
		})
	}
}
