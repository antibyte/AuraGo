package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"aurago/internal/config"
)

// telegramAPIBase is the Telegram Bot API endpoint; tests point it to a local server.
// Messages (sendTelegramNotification) and documents both use it. AuraGo has no Telegram
// proxy or API base setting: every Telegram client uses http.DefaultTransport, so both
// follow the HTTP(S)_PROXY environment the same way.
var telegramAPIBase = "https://api.telegram.org"

// telegramNoRedirect keeps a Telegram call on the Bot API endpoint: a redirect is returned
// as the answer and fails, instead of being followed to a page that may say ok:true.
func telegramNoRedirect(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// telegramUploadClient sends documents. Its timeout is only a backstop: each upload gets a
// deadline that grows with the file size (telegramUploadTimeout).
var telegramUploadClient = &http.Client{Timeout: 15 * time.Minute, CheckRedirect: telegramNoRedirect}

// telegramMessageClient sends the text messages of sendTelegramNotification. It is not
// notifyHTTPClient, which ntfy and Pushover share and which follows redirects.
var telegramMessageClient = &http.Client{Timeout: 15 * time.Second, CheckRedirect: telegramNoRedirect}

const (
	telegramMaxDocumentBytes = 50 << 20
	// telegramMaxCaptionUnits is Telegram's caption limit. Telegram counts text length in
	// UTF-16 code units, so an emoji outside the BMP counts twice.
	telegramMaxCaptionUnits = 1024
	// telegramMaxMessageUnits is Telegram's limit for the text of a message, in UTF-16 code
	// units after entity parsing.
	telegramMaxMessageUnits = 4096
	// telegramMaxReplyBytes bounds how much of Telegram's answer is read. A delivered message
	// comes back whole, with its entities, which can pass 64 KiB for a 4096-unit text; an
	// error repeats at most telegramMaxEchoRunes of it anyway.
	telegramMaxReplyBytes = 1 << 20
	// telegramMaxEchoRunes bounds how much of an answer or a transport error an error repeats.
	telegramMaxEchoRunes = 300
)

// telegramUploadTimeout is the deadline for uploading a document of size bytes: a minute,
// plus a second for every 128 KiB (about 1 Mbit/s), at most ten minutes. A 50 MB document
// gets about 7.7 minutes.
func telegramUploadTimeout(size int64) time.Duration {
	return min(time.Minute+time.Duration(size/(128<<10))*time.Second, 10*time.Minute)
}

// SendTelegramDocument uploads a file as a document to the configured Telegram chat. The
// file must qualify as outgoing attachment: it is opened with OpenOutgoingAttachment, so
// only files in the workspace or the documents folder can be sent. The caption is cut to
// Telegram's limit.
func SendTelegramDocument(ctx context.Context, cfg *config.Config, path, caption string) error {
	if _, _, err := telegramDocumentTarget(cfg); err != nil {
		return err
	}
	f, resolved, err := OpenOutgoingAttachment(path, cfg)
	if err != nil {
		return err
	}
	defer f.Close()
	return sendTelegramDocumentFile(ctx, cfg, f, filepath.Base(resolved), caption)
}

// SendTelegramFile sends a workspace or document file with an optional title and text.
// A text longer than a caption is sent as a message first. The file is opened and its size
// checked before anything is sent, so a refused file sends no text either. The result
// mirrors SendNotification: {"status", "results":[{"channel","status","detail"}], "document"}.
// When the title and text went out as a message of their own, the result also holds
// "text_sent": true. That is true on success and on error: an error with "text_sent" means
// the text was delivered and only the document failed, so a retry sends the text again.
func SendTelegramFile(ctx context.Context, cfg *config.Config, logger *slog.Logger, filePath, title, message string) string {
	if logger == nil {
		logger = slog.Default()
	}
	textSent := false
	encode := func(v map[string]any) string {
		if textSent {
			v["text_sent"] = true
		}
		data, _ := json.Marshal(v)
		return string(data)
	}
	fail := func(err error) string {
		return encode(map[string]any{"status": "error", "message": err.Error(),
			"results": []map[string]string{{"channel": "telegram", "status": "error", "detail": err.Error()}}})
	}
	if _, _, err := telegramDocumentTarget(cfg); err != nil {
		return fail(err)
	}
	f, path, err := OpenOutgoingAttachment(filePath, cfg)
	if err != nil {
		return fail(err)
	}
	defer f.Close()
	if _, err := telegramDocumentSize(f); err != nil {
		return fail(err)
	}
	name := telegramDocumentName(filepath.Base(path))
	caption := strings.TrimSpace(strings.TrimSpace(title) + "\n" + strings.TrimSpace(message))
	if utf16Units(caption) > telegramMaxCaptionUnits {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if err := sendTelegramFileText(cfg, logger, title, message); err != nil {
			return fail(err)
		}
		caption, textSent = "", true
	}
	if err := sendTelegramDocumentFile(ctx, cfg, f, name, caption); err != nil {
		logger.Warn("Telegram document could not be sent", "file", name, "error", err)
		if textSent {
			err = fmt.Errorf("the text was sent as a message, but the document was not: %w", err)
		}
		return fail(err)
	}
	return encode(map[string]any{"status": "success", "document": name,
		"results": []map[string]string{{"channel": "telegram", "status": "sent"}}})
}

// telegramDocumentTarget returns the bot token and the chat a document goes to.
func telegramDocumentTarget(cfg *config.Config) (string, int64, error) {
	if cfg == nil {
		return "", 0, fmt.Errorf("config is required")
	}
	token, chatID := cfg.Telegram.BotToken, cfg.Telegram.UserID
	if token == "" || chatID == 0 {
		return "", 0, fmt.Errorf("telegram bot_token and telegram_user_id must be configured")
	}
	return token, chatID, nil
}

// telegramDocumentSize checks the open file against Telegram's upload limit.
func telegramDocumentSize(f *os.File) (int64, error) {
	if f == nil {
		return 0, errors.New("no file to send")
	}
	info, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("read the file: %w", withoutPathError(err))
	}
	if !info.Mode().IsRegular() {
		return 0, errors.New("the file is not a regular file")
	}
	if info.Size() > telegramMaxDocumentBytes {
		return 0, errors.New("the file is larger than 50 MB, the Telegram upload limit")
	}
	return info.Size(), nil
}

// sendTelegramDocumentFile uploads the already open file f as a document named name. The
// caller has checked that f may leave AuraGo (OpenOutgoingAttachment) and closes it.
//
// The document is the file as large as its Stat says: a file that grows meanwhile is sent
// up to that size, one that shrinks makes the upload fail. The upload gets a deadline from
// the size (telegramUploadTimeout) on top of ctx.
//
// The multipart body is streamed through a pipe, so the file is never held in memory; the
// writer goroutine always ends before this function returns. That join waits for a file
// read in progress, so a read that stalls (a hung network mount, for example) keeps this
// call from returning even after ctx is cancelled.
func sendTelegramDocumentFile(ctx context.Context, cfg *config.Config, f *os.File, name, caption string) error {
	token, chatID, err := telegramDocumentTarget(cfg)
	if err != nil {
		return err
	}
	size, err := telegramDocumentSize(f)
	if err != nil {
		return err
	}
	name = telegramDocumentName(name)
	caption = truncateUTF16Units(strings.TrimSpace(strings.ToValidUTF8(caption, "�")), telegramMaxCaptionUnits)
	ctx, cancel := context.WithTimeout(ctx, telegramUploadTimeout(size))
	defer cancel()

	pr, pw := io.Pipe()
	form := multipart.NewWriter(pw)
	contentType := form.FormDataContentType()
	written := make(chan struct{})
	go func() {
		defer close(written)
		content := telegramFileReader{io.NewSectionReader(f, 0, size)}
		pw.CloseWithError(writeTelegramDocumentForm(form, chatID, caption, name, content, size))
	}()
	// Closing the reader side makes a writer that still waits on the pipe fail and end.
	// Waiting for it keeps the goroutine from outliving this call and from reading f after
	// the caller closed it.
	defer func() {
		_ = pr.Close()
		<-written
	}()

	endpoint := telegramAPIBase + "/bot" + url.PathEscape(token) + "/sendDocument"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, pr)
	if err != nil {
		return telegramRequestError(err, token)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := telegramUploadClient.Do(req)
	if err != nil {
		return telegramRequestError(err, token)
	}
	defer resp.Body.Close()
	return checkTelegramReply(resp, token, "document")
}

// writeTelegramDocumentForm writes the sendDocument form with exactly size bytes of content.
// Its error closes the pipe, so the request fails with it instead of sending a cut document:
// without the closing boundary the server cannot take the form as complete.
func writeTelegramDocumentForm(form *multipart.Writer, chatID int64, caption, name string, content io.Reader, size int64) error {
	if err := form.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return err
	}
	if caption != "" {
		if err := form.WriteField("caption", caption); err != nil {
			return err
		}
	}
	part, err := form.CreateFormFile("document", name)
	if err != nil {
		return err
	}
	n, err := io.Copy(part, content)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("the file changed while it was sent: %d of %d bytes", n, size)
	}
	return form.Close()
}

// telegramFileReader marks read errors of the file and drops the path they carry.
type telegramFileReader struct{ r io.Reader }

func (t telegramFileReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		err = fmt.Errorf("read the file: %w", withoutPathError(err))
	}
	return n, err
}

// telegramError is a Telegram failure whose text is safe to show: the bot token is
// redacted and the echo is bounded. It keeps its cause, without the request URL a
// *url.Error carries, so errors.Is still sees a cancellation or a deadline.
type telegramError struct {
	text  string
	cause error
}

func (e *telegramError) Error() string { return e.text }
func (e *telegramError) Unwrap() error { return e.cause }

// telegramRequestError reports a request that got no answer. The error of a failed request
// repeats its URL, and with it the bot token, so only the redacted text is shown.
func telegramRequestError(err error, token string) error {
	cause := err
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		cause = urlErr.Err
	}
	return &telegramError{text: "telegram request failed: " + telegramErrorText(err.Error(), token), cause: cause}
}

// checkTelegramReply accepts only an answer that is 2xx and says ok:true. Telegram, or a
// proxy in between, can answer 200 with ok:false. what names the sent object in the error.
func checkTelegramReply(resp *http.Response, token, what string) error {
	data, readErr := readHTTPResponseBody(resp.Body, telegramMaxReplyBytes)
	var reply struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	decoded := readErr == nil && json.Unmarshal(data, &reply) == nil
	detail := ""
	switch {
	case decoded:
		detail = reply.Description
	case readErr == nil:
		detail = string(data)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return telegramReplyError(fmt.Sprintf("telegram returned HTTP %d", resp.StatusCode), detail, token)
	}
	if readErr != nil {
		return telegramReplyError(fmt.Sprintf("telegram returned HTTP %d with an answer that could not be read", resp.StatusCode), readErr.Error(), token)
	}
	if !decoded {
		return telegramReplyError(fmt.Sprintf("telegram returned HTTP %d with an answer that is not a Bot API reply", resp.StatusCode), detail, token)
	}
	if !reply.OK {
		return telegramReplyError("telegram did not accept the "+what, detail, token)
	}
	return nil
}

func telegramReplyError(prefix, detail, token string) error {
	if strings.TrimSpace(detail) == "" {
		return errors.New(prefix)
	}
	return fmt.Errorf("%s: %s", prefix, telegramErrorText(detail, token))
}

// sendTelegramFileText sends the title and text as a Telegram message ahead of a document
// whose caption they do not fit. A title without text becomes the text.
//
// Telegram limits a message to telegramMaxMessageUnits after MarkdownV2 parsing. That counts
// the heading, the line break and the text that sendTelegramNotification joins, but not its
// escapes or bold markers. A heading over a caption's length is cut to it, and the text is
// cut to what is left, so a long text cannot keep the document from being sent.
func sendTelegramFileText(cfg *config.Config, logger *slog.Logger, title, message string) error {
	if strings.TrimSpace(message) == "" {
		title, message = "", title
	}
	if title == "" {
		title = defaultNotificationTitle
	}
	title = truncateUTF16Units(title, telegramMaxCaptionUnits)
	message = truncateUTF16Units(message, telegramMaxMessageUnits-utf16Units(title)-1)
	out := SendNotification(cfg, logger, string(ChannelTelegram), title, message, "normal", nil)
	var res struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Results []struct {
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		return errors.New("the text message could not be sent")
	}
	detail := res.Message
	for _, r := range res.Results {
		if res.Status == "success" && r.Status == "sent" {
			return nil
		}
		if detail == "" {
			detail = r.Detail
		}
	}
	if strings.TrimSpace(detail) == "" {
		return errors.New("the text message could not be sent")
	}
	return fmt.Errorf("the text message could not be sent: %s", telegramErrorText(detail, cfg.Telegram.BotToken))
}

// telegramDocumentName is the base name a document is uploaded under. Control characters
// (CR and LF included) cannot reach the multipart header; quotes are escaped there by
// CreateFormFile.
func telegramDocumentName(name string) string {
	name = filepath.Base(strings.ToValidUTF8(name, "�"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '_'
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
		return "document"
	}
	return name
}

// telegramErrorText makes text from Telegram or the transport safe to repeat in an error:
// the bot token is replaced in every form it can take there, control characters become
// spaces, and at most telegramMaxEchoRunes runes are kept. The token is replaced before the
// cut, so a cut can never leave part of it behind.
func telegramErrorText(text, token string) string {
	text = redactTelegramToken(strings.ToValidUTF8(text, "�"), token)
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	return truncateStr(strings.TrimSpace(text), telegramMaxEchoRunes)
}

// redactTelegramToken replaces the bot token with "***" in every form an error can carry
// it: raw, URL-escaped (a *url.Error prints the request URL), Go-quoted (%q) and
// JSON-escaped (an answer that is not decoded, with and without HTML escaping). One pass
// with the longer forms first, so no form leaves a piece of another.
func redactTelegramToken(text, token string) string {
	if token == "" {
		return text
	}
	quoted := strconv.Quote(token)
	forms := []string{
		token,
		url.PathEscape(token),
		url.QueryEscape(token),
		quoted[1 : len(quoted)-1],
		jsonStringInner(token, true),
		jsonStringInner(token, false),
	}
	sort.SliceStable(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })
	pairs := make([]string, 0, 2*len(forms))
	seen := make(map[string]bool, len(forms))
	for _, form := range forms {
		if form == "" || seen[form] {
			continue
		}
		seen[form] = true
		pairs = append(pairs, form, "***")
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// jsonStringInner is s as it appears inside a JSON string, without the quotes.
func jsonStringInner(s string, escapeHTML bool) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(escapeHTML)
	if err := enc.Encode(s); err != nil {
		return ""
	}
	out := strings.TrimSuffix(buf.String(), "\n")
	if len(out) < 2 {
		return ""
	}
	return out[1 : len(out)-1]
}

// utf16Units counts s the way Telegram counts text length: in UTF-16 code units.
func utf16Units(s string) int {
	n := 0
	for _, r := range s {
		n += utf16RuneUnits(r)
	}
	return n
}

func utf16RuneUnits(r rune) int {
	if r > 0xFFFF {
		return 2
	}
	return 1
}

// truncateUTF16Units cuts s to at most limit UTF-16 code units, at a rune boundary, and
// marks the cut with an ellipsis (one unit).
func truncateUTF16Units(s string, limit int) string {
	if utf16Units(s) <= limit {
		return s
	}
	if limit < 1 {
		return ""
	}
	n := 0
	for i, r := range s {
		if n+utf16RuneUnits(r) > limit-1 {
			return s[:i] + "…"
		}
		n += utf16RuneUnits(r)
	}
	return s
}
