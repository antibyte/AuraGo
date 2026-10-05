package tools

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
)

// tg04Request is one request the fake Bot API received.
type tg04Request struct {
	path       string // decoded URL path
	method     string // last path element: sendDocument, sendMessage
	chunked    bool   // no Content-Length: the body was streamed
	fields     map[string]string
	fileName   string
	fileHeader textproto.MIMEHeader
	fileSize   int64
	fileSum    [32]byte
	text       string // sendMessage text
	parseErr   string
}

func tg04Logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// tg04Server points telegramAPIBase at a local server for the test.
func tg04Server(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	old := telegramAPIBase
	telegramAPIBase = srv.URL
	t.Cleanup(func() {
		telegramAPIBase = old
		srv.Close()
	})
}

// tg04API is a fake Bot API that records every request and answers with reply, or with
// ok:true when reply is nil.
func tg04API(t *testing.T, reply http.HandlerFunc) <-chan tg04Request {
	t.Helper()
	requests := make(chan tg04Request, 32)
	tg04Server(t, func(w http.ResponseWriter, r *http.Request) {
		requests <- tg04Read(r)
		if reply != nil {
			reply(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	})
	return requests
}

func tg04Read(r *http.Request) tg04Request {
	req := tg04Request{path: r.URL.Path, method: path.Base(r.URL.Path), chunked: r.ContentLength == -1, fields: map[string]string{}}
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch mediaType {
	case "application/json":
		var body struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			req.parseErr = err.Error()
		}
		req.text = body.Text
		req.fields["chat_id"] = strconv.FormatInt(body.ChatID, 10)
	case "multipart/form-data":
		mr, err := r.MultipartReader()
		if err != nil {
			req.parseErr = err.Error()
			return req
		}
		for {
			part, err := mr.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				req.parseErr = err.Error()
				break
			}
			if part.FormName() == "document" {
				h := sha256.New()
				n, err := io.Copy(h, part)
				if err != nil {
					req.parseErr = err.Error()
				}
				req.fileName, req.fileHeader, req.fileSize = part.FileName(), part.Header, n
				copy(req.fileSum[:], h.Sum(nil))
				continue
			}
			data, _ := io.ReadAll(part)
			req.fields[part.FormName()] = string(data)
		}
	default:
		req.parseErr = "unexpected content type " + mediaType
	}
	return req
}

func tg04Config(t *testing.T, token string) (*config.Config, string) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Telegram.BotToken = token
	cfg.Telegram.UserID = 4711
	return cfg, cfg.Directories.WorkspaceDir
}

func tg04File(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func tg04RandomBytes(t *testing.T, n int) []byte {
	t.Helper()
	data := make([]byte, n)
	if _, err := cryptorand.Read(data); err != nil {
		t.Fatal(err)
	}
	return data
}

// tg04Next returns the next request, or fails when none reaches the bot in time.
func tg04Next(t *testing.T, requests <-chan tg04Request, what string) tg04Request {
	t.Helper()
	select {
	case req := <-requests:
		return req
	case <-time.After(10 * time.Second):
		t.Fatalf("%s: no request reached the bot", what)
		return tg04Request{}
	}
}

func tg04NoRequest(t *testing.T, requests <-chan tg04Request, what string) {
	t.Helper()
	select {
	case req := <-requests:
		t.Fatalf("%s: a request reached the bot: %s", what, req.path)
	default:
	}
}

// tg04UploadWriters counts the goroutines sendTelegramDocumentFile started that still run.
func tg04UploadWriters() int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	return strings.Count(string(buf), "created by aurago/internal/tools.sendTelegramDocumentFile")
}

func tg04WaitNoUploadWriters(t *testing.T, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for tg04UploadWriters() > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%s: the upload writer goroutine still runs", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// tg04Within runs send and fails the test when it does not return in time, which is what a
// writer stuck on the pipe would cause.
func tg04Within(t *testing.T, what string, send func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- send() }()
	select {
	case err := <-done:
		return err
	case <-time.After(20 * time.Second):
		t.Fatalf("%s: the upload did not return", what)
		return nil
	}
}

// The body is streamed (no Content-Length, no 50 MB buffer) and arrives intact.
func TestTelegramDocumentStreamsALargeFileIntact(t *testing.T) {
	requests := tg04API(t, nil)
	cfg, dir := tg04Config(t, "123:secret")
	data := tg04RandomBytes(t, 3<<20+17)
	tg04File(t, dir, "gross.bin", data)

	out := SendTelegramFile(context.Background(), cfg, tg04Logger(), "gross.bin", "Titel", "Text")
	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("result = %s", out)
	}
	req := <-requests
	if req.parseErr != "" || req.method != "sendDocument" || req.fileName != "gross.bin" {
		t.Fatalf("request = %+v", req)
	}
	if req.fileSize != int64(len(data)) || req.fileSum != sha256.Sum256(data) {
		t.Fatalf("the document arrived with %d bytes and another hash than the %d bytes sent", req.fileSize, len(data))
	}
	if !req.chunked {
		t.Error("the request has a Content-Length, so the body was buffered instead of streamed")
	}
	if req.fields["chat_id"] != "4711" || req.fields["caption"] != "Titel\nText" {
		t.Fatalf("fields = %q", req.fields)
	}
}

// A file over the limit is refused from its Stat, before any text or document is sent.
func TestTelegramDocumentRefusesOversizedFilesBeforeSendingAnything(t *testing.T) {
	requests := tg04API(t, nil)
	cfg, dir := tg04Config(t, "123:secret")
	huge := filepath.Join(dir, "huge.bin")
	f, err := os.Create(huge)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(telegramMaxDocumentBytes + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	out := SendTelegramFile(context.Background(), cfg, tg04Logger(), "huge.bin", "", strings.Repeat("😀", 600))
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "50 MB") {
		t.Fatalf("result = %s", out)
	}
	if err := SendTelegramDocument(context.Background(), cfg, huge, ""); err == nil || !strings.Contains(err.Error(), "50 MB") {
		t.Fatalf("error = %v", err)
	}
	tg04NoRequest(t, requests, "an oversized file")
}

// SendTelegramDocument opens through the attachment jail like SendTelegramFile.
func TestSendTelegramDocumentOnlySendsAttachmentFiles(t *testing.T) {
	requests := tg04API(t, nil)
	cfg, _ := tg04Config(t, "123:secret")
	outside := tg04File(t, t.TempDir(), "geheim.txt", []byte("geheim"))
	if err := SendTelegramDocument(context.Background(), cfg, outside, ""); err == nil {
		t.Fatal("a file outside the workspace and the documents folder was sent")
	}
	tg04NoRequest(t, requests, "a file outside the attachment folders")
}

// Only a 2xx answer with ok:true is a sent document; error texts echo a bounded, cleaned
// part of the answer.
func TestTelegramDocumentNeedsOKTrue(t *testing.T) {
	type reply struct {
		status   int
		location string
		body     string
	}
	var current reply
	requests := tg04API(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		if current.location != "" {
			w.Header().Set("Location", current.location)
		}
		w.WriteHeader(current.status)
		_, _ = w.Write([]byte(current.body))
	})
	cfg, dir := tg04Config(t, "123:secret")
	doc := tg04File(t, dir, "a.pdf", []byte("%PDF"))
	long := strings.Repeat("y", 5000)

	for _, tc := range []struct {
		name  string
		reply reply
		want  string // "" means success
	}{
		{"ok", reply{status: 200, body: `{"ok":true,"result":{"message_id":1}}`}, ""},
		{"200 with ok false", reply{status: 200, body: `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`}, "telegram did not accept the document: Bad Request: chat not found"},
		{"200 without ok", reply{status: 200, body: `{"result":{}}`}, "telegram did not accept the document"},
		{"200 that is not JSON", reply{status: 200, body: `<html>proxy login</html>`}, "not a Bot API reply: <html>proxy login</html>"},
		{"200 over 64 KiB", reply{status: 200, body: `{"ok":true,"pad":"` + strings.Repeat("x", 70<<10) + `"}`}, "could not be read"},
		{"502 with a description", reply{status: 502, body: `{"ok":false,"description":"Bad Gateway"}`}, "telegram returned HTTP 502: Bad Gateway"},
		{"400 with control characters", reply{status: 400, body: `{"ok":false,"description":"line1\nline2\u001b[31m"}`}, "telegram returned HTTP 400: line1 line2 [31m"},
		{"400 with a huge description", reply{status: 400, body: `{"ok":false,"description":"` + long + `"}`}, "telegram returned HTTP 400: yyy"},
		{"a redirect is not followed", reply{status: 302, location: "/elsewhere"}, "telegram returned HTTP 302"},
	} {
		current = tc.reply
		err := SendTelegramDocument(context.Background(), cfg, doc, "")
		if req := <-requests; req.method != "sendDocument" {
			t.Fatalf("%s: request = %+v", tc.name, req)
		}
		tg04NoRequest(t, requests, tc.name)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want %q", tc.name, err, tc.want)
			continue
		}
		if n := strings.Count(err.Error(), "y"); tc.reply.body != "" && strings.Contains(tc.reply.body, long) && n > telegramMaxEchoRunes {
			t.Errorf("%s: the error echoes %d runes of the answer", tc.name, n)
		}
		if strings.ContainsAny(err.Error(), "\n\x1b") {
			t.Errorf("%s: the error has control characters: %q", tc.name, err)
		}
	}
}

// tg04JSONInner is s as it appears inside a JSON string, written independently of the code
// under test.
func tg04JSONInner(t *testing.T, s string, escapeHTML bool) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(escapeHTML)
	if err := enc.Encode(s); err != nil {
		t.Fatal(err)
	}
	out := strings.TrimSpace(buf.String())
	return out[1 : len(out)-1]
}

// The token never shows in an error, raw, URL-escaped, Go-quoted or JSON-escaped, also
// when a transport error prints the request URL or Telegram echoes it, for documents and
// for the text messages of sendTelegramNotification.
func TestTelegramErrorsHideEveryTokenForm(t *testing.T) {
	const token = "4711:sé cr+et/?&=#%x\"q\\b<t>\x01"
	quoted := strconv.Quote(token)
	forms := []string{token, url.PathEscape(token), url.QueryEscape(token), quoted[1 : len(quoted)-1],
		tg04JSONInner(t, token, true), tg04JSONInner(t, token, false)}
	for i, form := range forms[1:] {
		if form == token {
			t.Fatalf("form %d leaves the token as it is, so it proves nothing", i+1)
		}
	}
	hidden := func(t *testing.T, what, text string) {
		t.Helper()
		if text == "" {
			t.Fatalf("%s: no error text", what)
		}
		for _, form := range forms {
			if strings.Contains(text, form) {
				t.Errorf("%s: %q contains the token as %q", what, text, form)
			}
		}
	}
	cfg, dir := tg04Config(t, token)
	doc := tg04File(t, dir, "a.pdf", []byte("%PDF"))
	ctx := context.Background()
	sends := map[string]func() error{
		"sendDocument": func() error { return SendTelegramDocument(ctx, cfg, doc, "") },
		"sendMessage":  func() error { return sendTelegramNotification(cfg, "t", "m") },
	}

	t.Run("a transport error prints the URL", func(t *testing.T) {
		tg04Server(t, func(w http.ResponseWriter, r *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
		})
		for method, send := range sends {
			err := send()
			if err == nil {
				t.Fatalf("%s: a hang-up must fail", method)
			}
			hidden(t, method+" hang-up", err.Error())
			if !strings.Contains(err.Error(), "/bot***/"+method) {
				t.Errorf("%s: the error does not carry the redacted URL, so this case proves nothing: %v", method, err)
			}
		}
	})

	echo := "Unauthorized: " + strings.Join(forms, " | ")
	t.Run("Telegram echoes the token", func(t *testing.T) {
		var status int
		var asJSON bool
		requests := tg04API(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			if asJSON {
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": echo})
				return
			}
			_, _ = w.Write([]byte(echo)) // a proxy page: the forms as they are, not decoded
		})
		for _, asJSON = range []bool{true, false} {
			for _, status = range []int{http.StatusUnauthorized, http.StatusOK} {
				for method, send := range sends {
					what := fmt.Sprintf("%s HTTP %d json=%v", method, status, asJSON)
					err := send()
					if err == nil {
						t.Fatalf("%s: must fail", what)
					}
					hidden(t, what, err.Error())
					if strings.Count(err.Error(), "***") < len(forms)-1 {
						t.Errorf("%s: the echo was dropped instead of redacted: %v", what, err)
					}
					if req := tg04Next(t, requests, what); req.path != "/bot"+token+"/"+method {
						t.Errorf("%s: the token reached the bot as %q", what, req.path)
					}
				}
			}
		}
	})

	t.Run("the text message of a long caption", func(t *testing.T) {
		requests := tg04API(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": echo})
		})
		out := SendTelegramFile(ctx, cfg, tg04Logger(), "a.pdf", "", strings.Repeat("😀", 600))
		// The text message really reached the bot, so the error is the bot's echo.
		if req := tg04Next(t, requests, "the text message"); req.method != "sendMessage" || req.path != "/bot"+token+"/sendMessage" || req.parseErr != "" {
			t.Fatalf("request = %+v", req)
		}
		tg04NoRequest(t, requests, "a failed text message")
		// json.Marshal escapes & and <, so the decoded texts are checked, not the JSON.
		var res struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Results []struct {
				Detail string `json:"detail"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(out), &res); err != nil || res.Status != "error" || len(res.Results) != 1 {
			t.Fatalf("result = %s (%v)", out, err)
		}
		hidden(t, "message", res.Message)
		hidden(t, "detail", res.Results[0].Detail)
	})
}

// Telegram counts the caption in UTF-16 code units: 600 emoji are 1200 units and go as a
// message first, while a caption of exactly 1024 units stays a caption.
func TestTelegramFileSendsLongCaptionsAsAMessageFirst(t *testing.T) {
	requests := tg04API(t, nil)
	cfg, dir := tg04Config(t, "123:secret")
	tg04File(t, dir, "bild.png", []byte("png"))
	ctx := context.Background()

	for _, tc := range []struct {
		name, title, message string
		split                bool
		heading, text        string // the message Telegram gets when split
	}{
		{name: "600 emoji are 1200 units", message: strings.Repeat("😀", 600), split: true, heading: "AuraGo", text: strings.Repeat("😀", 600)},
		{name: "512 emoji fit exactly", message: strings.Repeat("😀", 512)},
		{name: "1024 units with a title", title: "T", message: strings.Repeat("ä", 1022)},
		{name: "1025 units with a title", title: "T", message: strings.Repeat("ä", 1023), split: true, heading: "T", text: strings.Repeat("ä", 1023)},
		{name: "a long title without text", title: strings.Repeat("😀", 600), split: true, heading: "AuraGo", text: strings.Repeat("😀", 600)},
	} {
		out := SendTelegramFile(ctx, cfg, tg04Logger(), "bild.png", tc.title, tc.message)
		if !strings.Contains(out, `"status":"success"`) {
			t.Fatalf("%s: result = %s", tc.name, out)
		}
		if marked := strings.Contains(out, `"text_sent":true`); marked != tc.split {
			t.Errorf("%s: text_sent marker = %v, want %v: %s", tc.name, marked, tc.split, truncateStr(out, 120))
		}
		if tc.split {
			msg := <-requests
			want := "*" + escapeMarkdownV2(tc.heading) + "*\n" + escapeMarkdownV2(tc.text)
			if msg.method != "sendMessage" || msg.text != want {
				t.Fatalf("%s: first request = %s %q", tc.name, msg.method, truncateStr(msg.text, 40))
			}
		}
		doc := <-requests
		if doc.method != "sendDocument" || doc.parseErr != "" {
			t.Fatalf("%s: request = %+v", tc.name, doc)
		}
		caption, hasCaption := doc.fields["caption"]
		if tc.split && hasCaption {
			t.Errorf("%s: the document still has a caption of %d units", tc.name, utf16Units(caption))
		}
		if want := strings.TrimSpace(tc.title + "\n" + tc.message); !tc.split && caption != want {
			t.Errorf("%s: caption has %d units, want the %d-unit text", tc.name, utf16Units(caption), utf16Units(want))
		}
		tg04NoRequest(t, requests, tc.name)
	}

	// SendTelegramDocument cuts a long caption to the limit, at a rune boundary.
	if err := SendTelegramDocument(ctx, cfg, filepath.Join(dir, "bild.png"), strings.Repeat("😀", 600)); err != nil {
		t.Fatal(err)
	}
	caption := (<-requests).fields["caption"]
	if n := utf16Units(caption); n > telegramMaxCaptionUnits || !utf8.ValidString(caption) ||
		caption != strings.Repeat("😀", 511)+"…" {
		t.Fatalf("cut caption has %d units: %q", n, truncateStr(caption, 20))
	}
}

// tg04Result decodes a SendTelegramFile result.
type tg04Result struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	TextSent *bool  `json:"text_sent"`
	Results  []struct {
		Channel string `json:"channel"`
		Status  string `json:"status"`
		Detail  string `json:"detail"`
	} `json:"results"`
}

func tg04Decode(t *testing.T, out string) tg04Result {
	t.Helper()
	var res tg04Result
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("the result is not JSON: %v: %s", err, out)
	}
	return res
}

// When the text message fails, no document follows; when the document fails after the
// text, the error says the text went out, in words and as "text_sent": true.
func TestTelegramFileReportsWhichPartOfASplitSendFailed(t *testing.T) {
	var failMessage, failDocument atomic.Bool
	requests := tg04API(t, func(w http.ResponseWriter, r *http.Request) {
		if (strings.HasSuffix(r.URL.Path, "/sendMessage") && failMessage.Load()) ||
			(strings.HasSuffix(r.URL.Path, "/sendDocument") && failDocument.Load()) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: nope"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	cfg, dir := tg04Config(t, "123:secret")
	tg04File(t, dir, "a.pdf", []byte("%PDF"))
	long := strings.Repeat("😀", 600)

	failMessage.Store(true)
	out := SendTelegramFile(context.Background(), cfg, tg04Logger(), "a.pdf", "", long)
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "the text message could not be sent") || !strings.Contains(out, "nope") {
		t.Fatalf("result = %s", out)
	}
	if res := tg04Decode(t, out); res.TextSent != nil {
		t.Errorf("a text that failed is marked as sent: %s", out)
	}
	if req := <-requests; req.method != "sendMessage" {
		t.Fatalf("request = %s", req.method)
	}
	tg04NoRequest(t, requests, "a failed text message")

	failMessage.Store(false)
	failDocument.Store(true)
	out = SendTelegramFile(context.Background(), cfg, tg04Logger(), "a.pdf", "", long)
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "the text was sent as a message, but the document was not") {
		t.Fatalf("result = %s", out)
	}
	if res := tg04Decode(t, out); res.TextSent == nil || !*res.TextSent || len(res.Results) != 1 || res.Results[0].Status != "error" {
		t.Errorf("the result does not mark the text as sent: %s", out)
	}
	if first, second := <-requests, <-requests; first.method != "sendMessage" || second.method != "sendDocument" {
		t.Fatalf("requests = %s, %s", first.method, second.method)
	}
}

// The backslash is escaped first, so "\_" and a trailing "\" reach Telegram as text.
func TestEscapeMarkdownV2EscapesTheBackslash(t *testing.T) {
	for in, want := range map[string]string{
		`a\_b\`:          `a\\\_b\\`,
		`C:\Temp\x.txt`:  `C:\\Temp\\x\.txt`,
		`\\`:             `\\\\`,
		`*bold* [l](u)!`: `\*bold\* \[l\]\(u\)\!`,
	} {
		if got := escapeMarkdownV2(in); got != want {
			t.Errorf("escapeMarkdownV2(%q) = %q, want %q", in, got, want)
		}
	}

	requests := tg04API(t, nil)
	cfg, _ := tg04Config(t, "123:secret")
	out := SendNotification(cfg, tg04Logger(), "telegram", `T\`, `x\_y\`, "normal", nil)
	if !strings.Contains(out, `"status":"sent"`) {
		t.Fatalf("result = %s", out)
	}
	if req := <-requests; req.method != "sendMessage" || req.text != "*"+`T\\`+"*\n"+`x\\\_y\\` {
		t.Fatalf("Telegram got %q", req.text)
	}
}

// The name in the multipart header is a base name without control characters; quotes are
// escaped by CreateFormFile, so no name can add a header or break the form.
func TestTelegramDocumentNameCannotBreakTheMultipartHeader(t *testing.T) {
	requests := tg04API(t, nil)
	cfg, dir := tg04Config(t, "123:secret")
	content := []byte("inhalt")
	p := tg04File(t, dir, "inhalt.txt", content)

	for _, tc := range []struct{ name, want string }{
		{"a\"b\r\nX-Evil: 1\r\n\r\nc.pdf", "a\"b__X-Evil: 1____c.pdf"},
		{"tab\there\x00.pdf", "tab_here_.pdf"},
		{"../../etc/passwd", "passwd"},
		{"bad\xffutf8.txt", "bad\uFFFDutf8.txt"},
		{"", "document"},
	} {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		err = sendTelegramDocumentFile(context.Background(), cfg, f, tc.name, "Unterschrift")
		f.Close()
		if err != nil {
			t.Fatalf("%q: %v", tc.name, err)
		}
		req := <-requests
		if req.parseErr != "" || req.fileName != tc.want {
			t.Errorf("%q: arrived as %q (%s)", tc.name, req.fileName, req.parseErr)
		}
		if len(req.fileHeader) != 2 || req.fileHeader.Get("Content-Disposition") == "" || req.fileHeader.Get("Content-Type") == "" {
			t.Errorf("%q: part header = %q", tc.name, req.fileHeader)
		}
		if req.fields["chat_id"] != "4711" || req.fields["caption"] != "Unterschrift" || len(req.fields) != 2 || req.fileSum != sha256.Sum256(content) {
			t.Errorf("%q: fields = %q", tc.name, req.fields)
		}
	}
}

// tg04Transport answers without reading the request body and records whether the
// response body was closed.
type tg04Transport struct {
	status int
	body   string
	closed *atomic.Int32
}

type tg04Body struct {
	io.Reader
	closed *atomic.Int32
}

func (b tg04Body) Close() error {
	b.closed.Add(1)
	return nil
}

func (tr tg04Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: tr.status, Header: http.Header{}, Request: r,
		Body: tg04Body{Reader: strings.NewReader(tr.body), closed: tr.closed}}, nil
}

// The writer goroutine ends and the response body is closed on every way an upload can
// end: hang-up, early answer, cancellation, and a transport that never reads the body.
func TestTelegramDocumentUploadAlwaysEndsItsWriter(t *testing.T) {
	cfg, dir := tg04Config(t, "123:secret")
	doc := tg04File(t, dir, "gross.bin", tg04RandomBytes(t, 4<<20))
	ctx := context.Background()

	t.Run("the server hangs up", func(t *testing.T) {
		tg04Server(t, func(w http.ResponseWriter, r *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
		})
		if err := tg04Within(t, "hang-up", func() error { return SendTelegramDocument(ctx, cfg, doc, "") }); err == nil {
			t.Fatal("a hang-up must fail")
		}
		tg04WaitNoUploadWriters(t, "hang-up")
	})

	t.Run("the server answers before reading the body", func(t *testing.T) {
		tg04Server(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = w.Write([]byte(`{"ok":false,"description":"Request Entity Too Large"}`))
		})
		err := tg04Within(t, "early answer", func() error { return SendTelegramDocument(ctx, cfg, doc, "") })
		if err == nil || !strings.Contains(err.Error(), "413") {
			t.Fatalf("error = %v", err)
		}
		tg04WaitNoUploadWriters(t, "early answer")
	})

	t.Run("the caller cancels a stalled upload", func(t *testing.T) {
		release := make(chan struct{})
		tg04Server(t, func(w http.ResponseWriter, r *http.Request) { <-release })
		t.Cleanup(func() { close(release) })
		cctx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- SendTelegramDocument(cctx, cfg, doc, "") }()
		deadline := time.Now().Add(5 * time.Second)
		for tg04UploadWriters() == 0 {
			if time.Now().After(deadline) {
				t.Fatal("the writer goroutine was never seen, so the leak check proves nothing")
			}
			time.Sleep(time.Millisecond)
		}
		cancel()
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "context canceled") {
				t.Fatalf("error = %v", err)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("the cancelled upload did not return")
		}
		tg04WaitNoUploadWriters(t, "cancel")
	})

	t.Run("a transport that never reads the body", func(t *testing.T) {
		old := telegramUploadClient
		t.Cleanup(func() { telegramUploadClient = old })
		for _, tc := range []struct {
			status int
			body   string
			ok     bool
		}{
			{200, `{"ok":true}`, true},
			{200, `{"ok":false}`, false},
			{400, `{"ok":false,"description":"x"}`, false},
			{200, strings.Repeat("z", 100<<10), false},
		} {
			var closed atomic.Int32
			telegramUploadClient = &http.Client{Transport: tg04Transport{status: tc.status, body: tc.body, closed: &closed}}
			err := tg04Within(t, "transport", func() error { return SendTelegramDocument(ctx, cfg, doc, "") })
			if (err == nil) != tc.ok {
				t.Errorf("HTTP %d %.20s: error = %v", tc.status, tc.body, err)
			}
			if closed.Load() == 0 {
				t.Errorf("HTTP %d %.20s: the response body was not closed", tc.status, tc.body)
			}
			tg04WaitNoUploadWriters(t, "transport")
		}
	})
}

// tg04ReadingTransport reads the request body as a server would: the first after bytes,
// then mid(), then the rest. A body that ends cleanly and parses is answered with ok:true,
// as Telegram would accept a well-formed document, even a cut one; the document goes to got.
type tg04ReadingTransport struct {
	after int64
	mid   func()
	got   chan<- []byte
}

func (tr tg04ReadingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	defer r.Body.Close()
	var buf bytes.Buffer
	if _, err := io.CopyN(&buf, r.Body, tr.after); err != nil {
		return nil, err
	}
	tr.mid()
	if _, err := io.Copy(&buf, r.Body); err != nil {
		return nil, err
	}
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	mr := multipart.NewReader(&buf, params["boundary"])
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() == "document" {
			data, err := io.ReadAll(part)
			if err != nil {
				return nil, err
			}
			tr.got <- data
		}
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Request: r,
		Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
}

// A file that shrinks while it is sent fails instead of arriving cut; one that grows is
// sent as it was at the check.
func TestTelegramDocumentRefusesAFileThatShrinksWhileItIsSent(t *testing.T) {
	cfg, dir := tg04Config(t, "123:secret")
	orig := tg04RandomBytes(t, 4<<20)
	p := tg04File(t, dir, "doc.bin", orig)
	old := telegramUploadClient
	t.Cleanup(func() { telegramUploadClient = old })

	for _, tc := range []struct {
		name    string
		mid     func() error
		success bool
	}{
		{"shrink to 1 MiB", func() error { return os.Truncate(p, 1<<20) }, false},
		{"grow by 1 MiB", func() error {
			f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND, 0)
			if err != nil {
				return err
			}
			if _, err := f.Write(bytes.Repeat([]byte("B"), 1<<20)); err != nil {
				f.Close()
				return err
			}
			return f.Close()
		}, true},
	} {
		if err := os.WriteFile(p, orig, 0o600); err != nil {
			t.Fatal(err)
		}
		got := make(chan []byte, 1)
		telegramUploadClient = &http.Client{Transport: tg04ReadingTransport{after: 256 << 10, got: got, mid: func() {
			if err := tc.mid(); err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
		}}}
		out := SendTelegramFile(context.Background(), cfg, tg04Logger(), "doc.bin", "", "")
		tg04WaitNoUploadWriters(t, tc.name)
		if !tc.success {
			if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "the file changed while it was sent: 1048576 of 4194304 bytes") {
				t.Errorf("%s: result = %s", tc.name, out)
			}
			select {
			case data := <-got:
				t.Errorf("%s: a cut document of %d bytes reached the bot", tc.name, len(data))
			default:
			}
			continue
		}
		if !strings.Contains(out, `"status":"success"`) {
			t.Fatalf("%s: result = %s", tc.name, out)
		}
		if data := <-got; sha256.Sum256(data) != sha256.Sum256(orig) {
			t.Errorf("%s: the bot got %d bytes, not the %d bytes of the checked file", tc.name, len(data), len(orig))
		}
	}
}

// tg04FuncTransport answers with fn.
type tg04FuncTransport func(*http.Request) (*http.Response, error)

func (fn tg04FuncTransport) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

// The upload deadline grows with the size, so a 50 MB document gets minutes, not the
// client's fixed timeout, and the client's timeout is only a backstop above it.
func TestTelegramUploadTimeoutGrowsWithTheSize(t *testing.T) {
	for size, want := range map[int64]time.Duration{
		0:                        time.Minute,
		100 << 10:                time.Minute,
		1 << 20:                  time.Minute + 8*time.Second,
		telegramMaxDocumentBytes: time.Minute + 400*time.Second, // 7m40s
		1 << 30:                  10 * time.Minute,
	} {
		if got := telegramUploadTimeout(size); got != want {
			t.Errorf("telegramUploadTimeout(%d) = %v, want %v", size, got, want)
		}
	}
	if telegramUploadClient.Timeout <= telegramUploadTimeout(1<<40) {
		t.Errorf("the client timeout %v cuts uploads before their deadline", telegramUploadClient.Timeout)
	}

	// The deadline reaches the request.
	cfg, dir := tg04Config(t, "123:secret")
	doc := tg04File(t, dir, "a.bin", tg04RandomBytes(t, 2<<20))
	old := telegramUploadClient
	t.Cleanup(func() { telegramUploadClient = old })
	var deadline time.Time
	var hasDeadline bool
	telegramUploadClient = &http.Client{Transport: tg04FuncTransport(func(r *http.Request) (*http.Response, error) {
		deadline, hasDeadline = r.Context().Deadline()
		_, _ = io.Copy(io.Discard, r.Body)
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Request: r,
			Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})}
	start := time.Now()
	if err := SendTelegramDocument(context.Background(), cfg, doc, ""); err != nil {
		t.Fatal(err)
	}
	want := telegramUploadTimeout(2 << 20)
	if left := deadline.Sub(start); !hasDeadline || left > want+5*time.Second || left < want-5*time.Second {
		t.Errorf("the request deadline is %v away (set: %v), want about %v", left, hasDeadline, want)
	}
}

// A text over Telegram's message limit is cut, counted after MarkdownV2 parsing (the
// heading, a line break and the text), and the document still follows.
func TestTelegramFileCutsATextOverTheMessageLimit(t *testing.T) {
	requests := tg04API(t, nil)
	cfg, dir := tg04Config(t, "123:secret")
	tg04File(t, dir, "a.pdf", []byte("%PDF"))

	for _, tc := range []struct{ name, title, message, heading, text string }{
		{"5000 runes", "", strings.Repeat("ä", 5000), "AuraGo", strings.Repeat("ä", 4088) + "…"},
		{"3000 emoji with a title", "Titel", strings.Repeat("😀", 3000), "Titel", strings.Repeat("😀", 2044) + "…"},
		{"a long title and a long text", strings.Repeat("😀", 1000), strings.Repeat("ä", 5000), strings.Repeat("😀", 511) + "…", strings.Repeat("ä", 3071) + "…"},
		{"reserved characters do not count", "", strings.Repeat(".", 5000), "AuraGo", strings.Repeat(".", 4088) + "…"},
	} {
		out := SendTelegramFile(context.Background(), cfg, tg04Logger(), "a.pdf", tc.title, tc.message)
		if res := tg04Decode(t, out); res.Status != "success" || res.TextSent == nil || !*res.TextSent {
			t.Fatalf("%s: result = %s", tc.name, truncateStr(out, 200))
		}
		msg := <-requests
		want := "*" + escapeMarkdownV2(tc.heading) + "*\n" + escapeMarkdownV2(tc.text)
		if msg.method != "sendMessage" || msg.text != want {
			t.Fatalf("%s: message of %d units after parsing, want %d", tc.name, utf16Units(msg.text), utf16Units(tc.heading)+1+utf16Units(tc.text))
		}
		if n := utf16Units(tc.heading) + 1 + utf16Units(tc.text); n > telegramMaxMessageUnits {
			t.Fatalf("%s: the expected message has %d units", tc.name, n)
		}
		if doc := <-requests; doc.method != "sendDocument" || doc.fields["caption"] != "" {
			t.Fatalf("%s: second request = %+v", tc.name, doc)
		}
		tg04NoRequest(t, requests, tc.name)
	}
}

// sendTelegramNotification also needs a 2xx answer with ok:true, does not follow a
// redirect, and SendNotification keeps returning valid JSON whatever Telegram answers.
func TestTelegramMessageNeedsOKTrue(t *testing.T) {
	type reply struct {
		status   int
		location string
		body     string
	}
	var current reply
	requests := tg04API(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		if current.location != "" {
			w.Header().Set("Location", current.location)
		}
		w.WriteHeader(current.status)
		_, _ = w.Write([]byte(current.body))
	})
	cfg, _ := tg04Config(t, "123:secret")
	hostile := "bad\xff\xfe\"}\n\x00"

	for _, tc := range []struct {
		name  string
		reply reply
		want  string // "" means sent
	}{
		{"ok", reply{status: 200, body: `{"ok":true,"result":{"message_id":7}}`}, ""},
		{"200 with ok false", reply{status: 200, body: `{"ok":false,"description":"Forbidden: bot was blocked by the user"}`}, "telegram did not accept the message: Forbidden: bot was blocked by the user"},
		{"200 that is not JSON", reply{status: 200, body: `<html>login</html>`}, "not a Bot API reply: <html>login</html>"},
		{"400 with a description", reply{status: 400, body: `{"ok":false,"description":"Bad Request: can't parse entities"}`}, "telegram returned HTTP 400: Bad Request: can't parse entities"},
		{"a redirect is not followed", reply{status: 302, location: "/elsewhere"}, "telegram returned HTTP 302"},
		{"a hostile short body", reply{status: 400, body: hostile}, "telegram returned HTTP 400: bad"},
		{"a hostile huge body", reply{status: 400, body: hostile + strings.Repeat("q", 1<<20)}, "telegram returned HTTP 400"},
	} {
		current = tc.reply
		out := SendNotification(cfg, tg04Logger(), "telegram", "t", "m", "normal", nil)
		if req := <-requests; req.method != "sendMessage" {
			t.Fatalf("%s: request = %+v", tc.name, req)
		}
		tg04NoRequest(t, requests, tc.name)
		res := tg04Decode(t, out)
		if len(res.Results) != 1 {
			t.Fatalf("%s: result = %s", tc.name, out)
		}
		entry := res.Results[0]
		if tc.want == "" {
			if entry.Status != "sent" {
				t.Errorf("%s: result = %s", tc.name, out)
			}
			continue
		}
		if entry.Status != "error" || !strings.Contains(entry.Detail, tc.want) {
			t.Errorf("%s: detail = %q, want %q", tc.name, entry.Detail, tc.want)
		}
		if !utf8.ValidString(entry.Detail) || strings.ContainsAny(entry.Detail, "\n\x00") || utf8.RuneCountInString(entry.Detail) > 400 {
			t.Errorf("%s: the detail is not clean and bounded: %q", tc.name, truncateStr(entry.Detail, 80))
		}
	}
}

// The redacted errors keep their cause, without the *url.Error and its URL, so a caller
// can tell a cancellation or a deadline apart.
func TestTelegramErrorsKeepTheirCause(t *testing.T) {
	release := make(chan struct{})
	tg04Server(t, func(w http.ResponseWriter, r *http.Request) { <-release })
	t.Cleanup(func() { close(release) })
	cfg, dir := tg04Config(t, "123:secret")
	doc := tg04File(t, dir, "a.pdf", []byte("%PDF"))

	check := func(what string, err, cause error) {
		t.Helper()
		var urlErr *url.Error
		switch {
		case err == nil:
			t.Errorf("%s: no error", what)
		case !errors.Is(err, cause):
			t.Errorf("%s: %v is not %v", what, err, cause)
		case errors.As(err, &urlErr):
			t.Errorf("%s: the chain still holds the *url.Error with the URL", what)
		case strings.Contains(err.Error(), "secret"):
			t.Errorf("%s: the token is in %v", what, err)
		}
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	check("cancelled document", SendTelegramDocument(cancelled, cfg, doc, ""), context.Canceled)

	short, cancelShort := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelShort()
	check("document past its deadline", SendTelegramDocument(short, cfg, doc, ""), context.DeadlineExceeded)

	old := telegramMessageClient
	t.Cleanup(func() { telegramMessageClient = old })
	telegramMessageClient = &http.Client{Timeout: 100 * time.Millisecond, CheckRedirect: telegramNoRedirect}
	check("message past the client timeout", sendTelegramNotification(cfg, "t", "m"), context.DeadlineExceeded)

	err := fmt.Errorf("the text was sent as a message, but the document was not: %w", SendTelegramDocument(cancelled, cfg, doc, ""))
	check("wrapped", err, context.Canceled)
}

// The title SendNotification logs is bounded.
func TestSendNotificationBoundsTheLoggedTitle(t *testing.T) {
	tg04API(t, nil)
	cfg, _ := tg04Config(t, "123:secret")
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	out := SendNotification(cfg, logger, "telegram", strings.Repeat("x", 1000), "m", "normal", nil)
	if !strings.Contains(out, `"status":"sent"`) {
		t.Fatalf("result = %s", out)
	}
	if !strings.Contains(logs.String(), "Notification sent") || !strings.Contains(logs.String(), strings.Repeat("x", 100)) {
		t.Fatalf("the title was not logged: %s", truncateStr(logs.String(), 200))
	}
	if strings.Contains(logs.String(), strings.Repeat("x", 101)) {
		t.Errorf("the log holds more than 100 runes of the title")
	}
}
