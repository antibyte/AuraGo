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

// FF1 review: text-format calls. The parser promotes path, filepath, filename and file
// into the FilePath field; send_telegram takes the file_path parameter first and the field
// only when the parameters hold none of those names, so a stray alias neither attaches a
// file nor fails the call.
func TestFF1SendTelegramTextFormatAliasesAreNoFile(t *testing.T) {
	for _, alias := range []string{"path", "filepath", "filename", "file"} {
		tc := ToolCall{Action: "send_telegram", FilePath: "a.pdf", Params: map[string]interface{}{"message": "m", alias: "a.pdf"}}
		if got := decodeSendTelegramArgs(tc).FilePath; got != "" {
			t.Errorf("%s promoted into FilePath = %q", alias, got)
		}
		tc.Params["file_path"] = "b.pdf"
		if got := decodeSendTelegramArgs(tc).FilePath; got != "b.pdf" {
			t.Errorf("file_path next to %s = %q", alias, got)
		}
	}
	if got := decodeSendTelegramArgs(ToolCall{Action: "send_telegram", FilePath: "a.pdf"}).FilePath; got != "a.pdf" {
		t.Errorf("a top-level file_path without parameters = %q", got)
	}
	for name, raw := range map[string]string{
		"json params": `{"action":"send_telegram","params":{"message":"m","path":"../../config.yaml"}}`,
		"xml path":    `<tool_call><function=send_telegram><parameter=message>m</parameter><parameter=path>../../config.yaml</parameter></function></tool_call>`,
	} {
		tc := ParseToolCall(raw)
		if tc.Action != "send_telegram" {
			t.Fatalf("%s: parsed action %q", name, tc.Action)
		}
		if got := decodeSendTelegramArgs(tc).FilePath; got != "" {
			t.Errorf("%s: FilePath = %q", name, got)
		}
		if msg := telegramFilePathArgError(tc.Params); msg != "" {
			t.Errorf("%s: refused with %q", name, msg)
		}
	}
	xml := ParseToolCall(`<tool_call><function=send_telegram><parameter=message>m</parameter><parameter=file_path>bericht.pdf</parameter></function></tool_call>`)
	if got := decodeSendTelegramArgs(xml).FilePath; got != "bericht.pdf" {
		t.Errorf("xml file_path = %q", got)
	}
}
