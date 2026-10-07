package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"aurago/internal/config"
)

// Hardening tests for task 1c-06 (send_telegram.file_path, send_email.attachments). The plan
// tests live in flow_tool_extensions_test.go. Helpers carry the prefix c06.

// c06Result decodes the JSON behind the "Tool Output: " prefix of a dispatch result and
// fails when it is not one JSON object.
func c06Result(t *testing.T, out string) map[string]any {
	t.Helper()
	body, ok := strings.CutPrefix(out, "Tool Output: ")
	if !ok {
		t.Fatalf("result has no Tool Output prefix: %q", out)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("result is not valid JSON (%v): %q", err, body)
	}
	return result
}

func c06Message(t *testing.T, out string) string {
	t.Helper()
	msg, _ := c06Result(t, out)["message"].(string)
	return msg
}

func c06WriteFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// ── Telegram: a fake Bot API behind http.DefaultTransport ───────────────────

type c06BotRequest struct {
	Path     string
	JSON     string            // body of a sendMessage
	Fields   map[string]string // text fields of a sendDocument
	Filename string
	Content  string
}

// c06Bot answers every Telegram request with {"ok":true} and records it. The tools package
// sends through clients without a transport of their own, so swapping http.DefaultTransport
// catches both the message and the document path. Tests that use it must not run in parallel.
type c06Bot struct {
	mu       sync.Mutex
	requests []c06BotRequest
}

func (b *c06Bot) RoundTrip(r *http.Request) (*http.Response, error) {
	req := c06BotRequest{Path: r.URL.Path, Fields: map[string]string{}}
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return nil, err
	}
	if mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil && strings.HasPrefix(mediaType, "multipart/") {
		reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			data, _ := io.ReadAll(part)
			if part.FormName() == "document" {
				req.Filename, req.Content = part.FileName(), string(data)
			} else {
				req.Fields[part.FormName()] = string(data)
			}
		}
	} else {
		req.JSON = string(body)
	}
	b.mu.Lock()
	b.requests = append(b.requests, req)
	b.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		Request:    r,
	}, nil
}

func (b *c06Bot) all() []c06BotRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]c06BotRequest(nil), b.requests...)
}

func c06FakeBot(t *testing.T) *c06Bot {
	t.Helper()
	bot := &c06Bot{}
	previous := http.DefaultTransport
	http.DefaultTransport = bot
	t.Cleanup(func() { http.DefaultTransport = previous })
	return bot
}

func c06TelegramConfig(t *testing.T) (*config.Config, string) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Telegram.BotToken = "123:abc"
	cfg.Telegram.UserID = 4711
	return cfg, cfg.Directories.WorkspaceDir
}

func c06SendTelegram(cfg *config.Config, params map[string]interface{}) string {
	out, _ := dispatchMessagingCases(context.Background(), ToolCall{Action: "send_telegram", Params: params},
		&DispatchContext{Cfg: cfg, Logger: slog.Default()})
	return out
}

func TestC06SendTelegramWithoutAFileSendsOnlyAMessage(t *testing.T) {
	for name, extra := range map[string]map[string]interface{}{
		"no file_path":      {},
		"empty file_path":   {"file_path": ""},
		"null file_path":    {"file_path": nil},
		"blank file_path":   {"file_path": "   "},
		"foreign channel":   {"channel": "discord"},
		"foreign parameter": {"url": "https://example.com/a.pdf", "caption": "x"},
	} {
		t.Run(name, func(t *testing.T) {
			bot := c06FakeBot(t)
			cfg, _ := c06TelegramConfig(t)
			params := map[string]interface{}{"message": "Hallo Welt", "title": "Titel", "priority": "high"}
			for k, v := range extra {
				params[k] = v
			}
			out := c06SendTelegram(cfg, params)
			if got := c06Result(t, out)["status"]; got != "success" {
				t.Fatalf("status = %v, output %s", got, out)
			}
			requests := bot.all()
			if len(requests) != 1 || requests[0].Path != "/bot123:abc/sendMessage" || !strings.Contains(requests[0].JSON, "Hallo Welt") {
				t.Fatalf("requests = %+v, want one sendMessage with the text", requests)
			}
		})
	}
}

func TestC06SendTelegramFileReachesTheBotAsADocument(t *testing.T) {
	cfg, workspace := c06TelegramConfig(t)
	absolute := c06WriteFile(t, workspace, "c06-bericht.pdf", "%PDF-1.4 c06")
	for name, params := range map[string]map[string]interface{}{
		"file_path relative": {"file_path": "c06-bericht.pdf"},
		"file_path absolute": {"file_path": absolute},
	} {
		t.Run(name, func(t *testing.T) {
			bot := c06FakeBot(t)
			all := map[string]interface{}{"message": "Hallo", "title": "Titel"}
			for k, v := range params {
				all[k] = v
			}
			out := c06SendTelegram(cfg, all)
			result := c06Result(t, out)
			if result["status"] != "success" || result["document"] != "c06-bericht.pdf" {
				t.Fatalf("result = %s", out)
			}
			requests := bot.all()
			if len(requests) != 1 {
				t.Fatalf("requests = %+v, want exactly one sendDocument", requests)
			}
			got := requests[0]
			if got.Path != "/bot123:abc/sendDocument" || got.Filename != "c06-bericht.pdf" || got.Content != "%PDF-1.4 c06" ||
				got.Fields["chat_id"] != "4711" || got.Fields["caption"] != "Titel\nHallo" {
				t.Fatalf("document request = %+v", got)
			}
		})
	}
}

func TestC06SendTelegramFileNeedsTelegramJustLikeAMessageDoes(t *testing.T) {
	bot := c06FakeBot(t)
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = t.TempDir()
	const want = "telegram bot_token and telegram_user_id must be configured"

	plain := c06Result(t, c06SendTelegram(cfg, map[string]interface{}{"message": "hi"}))
	fileOut := c06SendTelegram(cfg, map[string]interface{}{"message": "hi", "file_path": "c06-not-there.pdf"})
	file := c06Result(t, fileOut)

	// The plain path reports a per-channel failure under status "success"; the file path
	// reports the same channel failure and also fails the call as a whole.
	if !reflect.DeepEqual(plain["results"], file["results"]) {
		t.Fatalf("results differ: plain %v, file %v", plain["results"], file["results"])
	}
	if plainJSON, _ := json.Marshal(plain); !strings.Contains(string(plainJSON), want) || !strings.Contains(fileOut, want) || file["status"] != "error" {
		t.Fatalf("plain %v, file %s", plain, fileOut)
	}
	// The gate is checked before the file: the file itself is never looked at.
	if strings.Contains(fileOut, "missing or outside") {
		t.Fatalf("the file was resolved before the Telegram gate: %s", fileOut)
	}
	if got := bot.all(); len(got) != 0 {
		t.Fatalf("requests = %+v, want none", got)
	}
}

func TestC06SendTelegramRefusesAFileThatIsNotAString(t *testing.T) {
	cfg, _ := c06TelegramConfig(t)
	for _, key := range []string{"file_path"} { // FF1: path is ignored (TestFF1SendTelegramIgnoresAStrayPath)
		for name, value := range map[string]interface{}{
			"list":   []interface{}{"a.pdf"},
			"object": map[string]interface{}{"path": "a.pdf"},
			"number": float64(5),
			"bool":   true,
		} {
			t.Run(key+"/"+name, func(t *testing.T) {
				bot := c06FakeBot(t)
				out := c06SendTelegram(cfg, map[string]interface{}{"message": "x", key: value})
				result := c06Result(t, out)
				if result["status"] != "error" || !strings.Contains(c06Message(t, out), key+" must be one file path string") {
					t.Fatalf("output = %s", out)
				}
				if got := bot.all(); len(got) != 0 {
					t.Fatalf("a refused call sent %+v", got)
				}
			})
		}
	}
}

// ── Email: a fake SMTP server that counts connections and refuses with odd text ──

const c06SMTPRefusal = `say "no" \ now`

// c06HasRefusal reports whether msg carries the refusal of the fake SMTP server. Go quotes a
// server's text in the error it returns, so the quote and the backslash are escaped once more
// there; either way both characters are still in the message that must stay valid JSON.
func c06HasRefusal(msg string) bool {
	return strings.Contains(msg, "say") && strings.Contains(msg, "now") && strings.Contains(msg, `"`) && strings.Contains(msg, `\`)
}

type c06SMTP struct {
	port  int
	conns atomic.Int32
}

// c06StartSMTP listens on a free local port and greets every connection with a 554 whose text
// holds a quote and a backslash, then hangs up. A send that reaches it fails with that text,
// and conns shows whether a send reached it at all.
func c06StartSMTP(t *testing.T) *c06SMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &c06SMTP{port: ln.Addr().(*net.TCPAddr).Port}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			srv.conns.Add(1)
			_, _ = conn.Write([]byte("554 " + c06SMTPRefusal + "\r\n"))
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return srv
}

func c06EmailConfig(t *testing.T, srv *c06SMTP, mutate func(*config.EmailAccount)) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Email.Enabled = true
	acct := config.EmailAccount{ID: "main", SMTPHost: "127.0.0.1", SMTPPort: srv.port, Username: "user", Password: "pw", FromAddress: "from@example.com"}
	if mutate != nil {
		mutate(&acct)
	}
	cfg.EmailAccounts = []config.EmailAccount{acct}
	return cfg
}

func c06SendEmail(t *testing.T, cfg *config.Config, logger *slog.Logger, params map[string]interface{}) string {
	t.Helper()
	if logger == nil {
		logger = slog.Default()
	}
	out, handled := dispatchEmailCases(context.Background(), ToolCall{Action: "send_email", Params: params}, &DispatchContext{Cfg: cfg, Logger: logger})
	if !handled {
		t.Fatal("send_email was not handled")
	}
	return out
}

func c06Eleven() []interface{} {
	paths := make([]interface{}, 11)
	for i := range paths {
		paths[i] = "c06-file.txt"
	}
	return paths
}

func TestC06SendEmailGatesRunBeforeAnyAttachmentIsTouched(t *testing.T) {
	srv := c06StartSMTP(t)
	gates := []struct {
		name   string
		cfg    func() *config.Config
		params map[string]interface{}
		want   string
	}{
		{"email not enabled", func() *config.Config {
			cfg := c06EmailConfig(t, srv, nil)
			cfg.Email.Enabled, cfg.EmailAccounts = false, nil
			return cfg
		}, map[string]interface{}{"to": "a@example.com"}, "Email is not enabled"},
		{"unknown account", func() *config.Config { return c06EmailConfig(t, srv, nil) },
			map[string]interface{}{"to": "a@example.com", "account": "nobody"}, "not found"},
		{"disabled account", func() *config.Config {
			return c06EmailConfig(t, srv, func(a *config.EmailAccount) { a.Disabled = true })
		}, map[string]interface{}{"to": "a@example.com", "account": "main"}, "is disabled"},
		{"read-only account", func() *config.Config {
			return c06EmailConfig(t, srv, func(a *config.EmailAccount) { a.ReadOnly = true })
		}, map[string]interface{}{"to": "a@example.com"}, "is read-only"},
		{"no recipient", func() *config.Config { return c06EmailConfig(t, srv, nil) },
			map[string]interface{}{"subject": "x"}, "'to' (recipient address) is required"},
	}
	attachments := map[string]interface{}{
		"missing file as list":   []interface{}{"c06-missing.pdf"},
		"missing file as string": "c06-missing.pdf",
		"eleven files":           c06Eleven(),
		"wrong type":             float64(5),
		"item of wrong type":     []interface{}{"c06-a.pdf", float64(7)},
	}
	for _, gate := range gates {
		for kind, attached := range attachments {
			t.Run(gate.name+"/"+kind, func(t *testing.T) {
				params := map[string]interface{}{"attachments": attached}
				for k, v := range gate.params {
					params[k] = v
				}
				out := c06SendEmail(t, gate.cfg(), nil, params)
				result := c06Result(t, out)
				msg, _ := result["message"].(string)
				if result["status"] != "error" || !strings.Contains(msg, gate.want) {
					t.Fatalf("output = %s, want the %q refusal", out, gate.want)
				}
				// The refusal is the gate's alone: no attachment argument or file was judged.
				if strings.Contains(strings.ToLower(msg), "attachment") {
					t.Fatalf("a gate answered with an attachment error: %s", out)
				}
			})
		}
	}
	if n := srv.conns.Load(); n != 0 {
		t.Fatalf("%d SMTP connections for refused calls", n)
	}
}

func TestC06SendEmailRefusedAttachmentsSendNothing(t *testing.T) {
	srv := c06StartSMTP(t)
	cfg := c06EmailConfig(t, srv, nil)
	workspace := cfg.Directories.WorkspaceDir
	c06WriteFile(t, workspace, "vault.bin", "secret")
	outside := c06WriteFile(t, filepath.Dir(workspace), "c06-outside.txt", "outside")
	for name, tc := range map[string]struct {
		attached interface{}
		want     string
	}{
		"missing file":      {[]interface{}{"c06-missing.pdf"}, "is missing or outside"},
		"file outside":      {[]interface{}{outside}, "is missing or outside"},
		"protected file":    {[]interface{}{"vault.bin"}, "vault.bin"},
		"eleven files":      {c06Eleven(), "at most 10 attachments"},
		"item of bad type":  {[]interface{}{"c06-a.pdf", float64(7)}, "attachments item 2 must be a file path string, not a number"},
		"null item":         {[]interface{}{nil}, "attachments item 1 must be a file path string, not null"},
		"object":            {map[string]interface{}{"path": "a.pdf"}, "attachments must be a list of file paths, not an object"},
		"number":            {float64(3), "attachments must be a list of file paths, not a number"},
		"json string items": {`[1,2]`, "attachments item 1 must be a file path string, not a number"},
	} {
		t.Run(name, func(t *testing.T) {
			out := c06SendEmail(t, cfg, nil, map[string]interface{}{"to": "a@example.com", "body": "x", "attachments": tc.attached})
			msg := c06Message(t, out)
			if c06Result(t, out)["status"] != "error" || !strings.Contains(msg, tc.want) {
				t.Fatalf("output = %s, want an error containing %q", out, tc.want)
			}
		})
	}
	if n := srv.conns.Load(); n != 0 {
		t.Fatalf("%d SMTP connections for calls whose attachments were refused", n)
	}
}

func TestC06SendEmailTakesAttachmentsInEveryFormModelsSend(t *testing.T) {
	forms := map[string]func(path string) interface{}{
		"array":             func(p string) interface{} { return []interface{}{p} },
		"typed array":       func(p string) interface{} { return []string{p} },
		"single string":     func(p string) interface{} { return p },
		"json array string": func(p string) interface{} { return `["` + p + `"]` },
		"padded and empty":  func(p string) interface{} { return []interface{}{"", " " + p + " ", "  "} },
	}
	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			srv := c06StartSMTP(t)
			cfg := c06EmailConfig(t, srv, nil)
			c06WriteFile(t, cfg.Directories.WorkspaceDir, "c06-report.pdf", "%PDF-1.4 c06")

			// A path that does not exist shows that the form was decoded and the file looked up.
			out := c06SendEmail(t, cfg, nil, map[string]interface{}{"to": "a@example.com", "attachments": form("c06-missing.pdf")})
			if msg := c06Message(t, out); !strings.Contains(msg, `attachment "c06-missing.pdf" is missing or outside`) {
				t.Fatalf("missing file: output = %s", out)
			}
			if n := srv.conns.Load(); n != 0 {
				t.Fatalf("%d SMTP connections although the attachment was refused", n)
			}

			// An existing file loads, and the send goes on to the server.
			out = c06SendEmail(t, cfg, nil, map[string]interface{}{"to": "a@example.com", "attachments": form("c06-report.pdf")})
			if msg := c06Message(t, out); !strings.Contains(msg, "SMTP send failed (main)") || !c06HasRefusal(msg) {
				t.Fatalf("existing file: output = %s", out)
			}
			if n := srv.conns.Load(); n != 1 {
				t.Fatalf("%d SMTP connections, want 1", n)
			}
		})
	}
}

func TestC06SendEmailWithoutAttachmentsStillSends(t *testing.T) {
	for name, attached := range map[string]interface{}{
		"absent":       nil,
		"empty list":   []interface{}{},
		"empty string": "",
		"empty items":  []interface{}{"", "  "},
	} {
		t.Run(name, func(t *testing.T) {
			srv := c06StartSMTP(t)
			cfg := c06EmailConfig(t, srv, nil)
			params := map[string]interface{}{"to": "a@example.com", "body": "x"}
			if name != "absent" {
				params["attachments"] = attached
			}
			out := c06SendEmail(t, cfg, nil, params)
			if msg := c06Message(t, out); !strings.Contains(msg, "SMTP send failed (main)") {
				t.Fatalf("output = %s", out)
			}
			if n := srv.conns.Load(); n != 1 {
				t.Fatalf("%d SMTP connections, want 1", n)
			}
		})
	}
}

func TestC06SendEmailErrorsAreValidJSON(t *testing.T) {
	srv := c06StartSMTP(t)
	cfg := c06EmailConfig(t, srv, nil)

	// The server's refusal holds a quote and a backslash: the result must still be one JSON
	// object that carries them (c06Result fails the test on invalid JSON).
	out := c06SendEmail(t, cfg, nil, map[string]interface{}{"to": "a@example.com"})
	if msg := c06Message(t, out); c06Result(t, out)["status"] != "error" || !c06HasRefusal(msg) {
		t.Fatalf("send error: output = %s", out)
	}

	// A model-supplied account name with a quote and a backslash is echoed bounded and encoded.
	account := `ab"c\d` + strings.Repeat("ü", 5000)
	out = c06SendEmail(t, cfg, nil, map[string]interface{}{"to": "a@example.com", "account": account})
	msg := c06Message(t, out)
	if !strings.Contains(msg, `ab"c\d`) || utf8.RuneCountInString(msg) > 400 || !strings.Contains(msg, "not found") {
		t.Fatalf("account error: %d runes, output starts %.120s", utf8.RuneCountInString(msg), out)
	}
}

func TestC06SendEmailLogBoundsRecipientAndSubject(t *testing.T) {
	srv := c06StartSMTP(t)
	cfg := c06EmailConfig(t, srv, nil)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	huge := strings.Repeat("ä", 5000)

	c06SendEmail(t, cfg, logger, map[string]interface{}{"to": huge + "@example.com", "subject": huge, "attachments": []interface{}{}})

	found := false
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not JSON: %q", line)
		}
		if record["msg"] != "LLM requested email send" {
			continue
		}
		found = true
		for _, key := range []string{"to", "subject"} {
			value, _ := record[key].(string)
			if n := utf8.RuneCountInString(value); n == 0 || n > logTextRunes+1 {
				t.Fatalf("log %s has %d runes, want at most %d", key, n, logTextRunes+1)
			}
		}
		if record["attachments"] != float64(0) {
			t.Fatalf("log attachments = %v", record["attachments"])
		}
	}
	if !found {
		t.Fatalf("no send log line in %s", logs.String())
	}
}

// ── Argument decoding ───────────────────────────────────────────────────────

func TestC06ToolArgPathList(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    map[string]interface{}
		want    []string
		wantErr string
	}{
		{"absent", map[string]interface{}{}, nil, ""},
		{"null", map[string]interface{}{"a": nil}, nil, ""},
		{"empty list", map[string]interface{}{"a": []interface{}{}}, nil, ""},
		{"empty string", map[string]interface{}{"a": ""}, nil, ""},
		{"blank string", map[string]interface{}{"a": "  \t"}, nil, ""},
		{"single string", map[string]interface{}{"a": "a.pdf"}, []string{"a.pdf"}, ""},
		{"padded string", map[string]interface{}{"a": "  a.pdf "}, []string{"a.pdf"}, ""},
		{"comma stays in the name", map[string]interface{}{"a": "a,b.pdf"}, []string{"a,b.pdf"}, ""},
		{"json array string", map[string]interface{}{"a": `["a.pdf","b.txt"]`}, []string{"a.pdf", "b.txt"}, ""},
		{"padded json array string", map[string]interface{}{"a": ` ["a.pdf"] `}, []string{"a.pdf"}, ""},
		{"empty json array string", map[string]interface{}{"a": `[]`}, nil, ""},
		{"json array of blanks", map[string]interface{}{"a": `["", " "]`}, nil, ""},
		{"bracketed name is no json", map[string]interface{}{"a": `[draft]`}, []string{"[draft]"}, ""},
		{"name that starts with a bracket", map[string]interface{}{"a": `[draft].pdf`}, []string{"[draft].pdf"}, ""},
		{"list", map[string]interface{}{"a": []interface{}{"a.pdf", "b.txt"}}, []string{"a.pdf", "b.txt"}, ""},
		{"list drops empty items", map[string]interface{}{"a": []interface{}{"a.pdf", "", "  ", " b.txt"}}, []string{"a.pdf", "b.txt"}, ""},
		{"typed list", map[string]interface{}{"a": []string{"a.pdf", " ", "b.txt"}}, []string{"a.pdf", "b.txt"}, ""},
		{"number item", map[string]interface{}{"a": []interface{}{"a.pdf", float64(7)}}, nil, "a item 2 must be a file path string, not a number"},
		{"null item", map[string]interface{}{"a": []interface{}{nil}}, nil, "a item 1 must be a file path string, not null"},
		{"bool item", map[string]interface{}{"a": []interface{}{true}}, nil, "not a boolean"},
		{"object item", map[string]interface{}{"a": []interface{}{map[string]interface{}{"path": "secret-value"}}}, nil, "not an object"},
		{"nested list item", map[string]interface{}{"a": []interface{}{[]interface{}{"x"}}}, nil, "not a list"},
		{"json string with numbers", map[string]interface{}{"a": `[1,2]`}, nil, "a item 1 must be a file path string, not a number"},
		{"number", map[string]interface{}{"a": float64(3)}, nil, "a must be a list of file paths, not a number"},
		{"object", map[string]interface{}{"a": map[string]interface{}{"x": "y"}}, nil, "a must be a list of file paths, not an object"},
		{"bool", map[string]interface{}{"a": false}, nil, "not a boolean"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := toolArgPathList(tc.args, "a")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
				}
				if strings.Contains(err.Error(), "secret-value") {
					t.Fatalf("the error repeats the value: %v", err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, %v; want %#v", got, err, tc.want)
			}
		})
	}
}

func TestC06DecodeEmailSendArgsCarriesTheAttachmentError(t *testing.T) {
	ok := decodeEmailSendArgs(ToolCall{Action: "send_email", Params: map[string]interface{}{"to": "a@example.com", "attachments": "a.pdf"}})
	if !reflect.DeepEqual(ok.Attachments, []string{"a.pdf"}) || ok.AttachmentsErr != nil {
		t.Fatalf("single string: %+v", ok)
	}
	bad := decodeEmailSendArgs(ToolCall{Action: "send_email", Params: map[string]interface{}{"to": "a@example.com", "attachments": []interface{}{"a.pdf", float64(1)}}})
	if bad.Attachments != nil || bad.AttachmentsErr == nil || bad.To != "a@example.com" {
		t.Fatalf("bad item: %+v", bad)
	}
	none := decodeEmailSendArgs(ToolCall{Action: "send_email", Params: map[string]interface{}{"to": "a@example.com"}})
	if none.Attachments != nil || none.AttachmentsErr != nil {
		t.Fatalf("no attachments: %+v", none)
	}
}

// FF1: only file_path names the file; path (param or field) is ignored.
func TestC06DecodeSendTelegramArgsReadsOnlyFilePath(t *testing.T) {
	for name, tc := range map[string]ToolCall{
		"file_path param": {Action: "send_telegram", Params: map[string]interface{}{"message": "m", "file_path": "a.pdf"}},
		"FilePath field":  {Action: "send_telegram", FilePath: "a.pdf", Params: map[string]interface{}{"message": "m"}},
	} {
		if got := decodeSendTelegramArgs(tc).FilePath; got != "a.pdf" {
			t.Errorf("%s: FilePath = %q", name, got)
		}
	}
	for name, tc := range map[string]ToolCall{
		"path param": {Action: "send_telegram", Params: map[string]interface{}{"message": "m", "path": "a.pdf"}},
		"Path field": {Action: "send_telegram", Path: "a.pdf", Params: map[string]interface{}{"message": "m"}},
	} {
		if got := decodeSendTelegramArgs(tc).FilePath; got != "" {
			t.Errorf("%s: FilePath = %q, want none", name, got)
		}
	}
	if got := decodeSendTelegramArgs(ToolCall{Action: "send_telegram", Params: map[string]interface{}{"message": "m", "title": "t"}}).FilePath; got != "" {
		t.Errorf("a call without a file got FilePath %q", got)
	}
}

func TestC06BoundedRunes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"short", "abc", 5, "abc"},
		{"exact", "abcde", 5, "abcde"},
		{"cut", "abcdef", 5, "abcde…"},
		{"multibyte exact", "äöüäö", 5, "äöüäö"},
		{"multibyte cut", "äöüäöü", 5, "äöüäö…"},
		{"invalid utf8", "ab\xff\xfecd", 3, "ab\xff…"},
		{"empty", "", 3, ""},
	} {
		if got := boundedRunes(tc.in, tc.max); got != tc.want {
			t.Errorf("%s: boundedRunes(%q, %d) = %q, want %q", tc.name, tc.in, tc.max, got, tc.want)
		}
	}
}
