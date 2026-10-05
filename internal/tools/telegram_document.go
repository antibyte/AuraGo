package tools

import (
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

// telegramUploadClient sends documents. It does not follow redirects: a document counts
// as sent only when the Bot API endpoint itself answers ok:true.
var telegramUploadClient = &http.Client{
	Timeout: 2 * time.Minute,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

const (
	telegramMaxDocumentBytes = 50 << 20
	// telegramMaxCaptionUnits is Telegram's caption limit. Telegram counts text length in
	// UTF-16 code units, so an emoji outside the BMP counts twice.
	telegramMaxCaptionUnits = 1024
	// telegramMaxReplyBytes bounds how much of Telegram's answer is read.
	telegramMaxReplyBytes = 64 << 10
	// telegramMaxEchoRunes bounds how much of an answer or a transport error an error repeats.
	telegramMaxEchoRunes = 300
)

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
func SendTelegramFile(ctx context.Context, cfg *config.Config, logger *slog.Logger, filePath, title, message string) string {
	if logger == nil {
		logger = slog.Default()
	}
	encode := func(v map[string]any) string {
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
	textSent := false
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
// caller has checked that f may leave AuraGo (OpenOutgoingAttachment) and closes it. The
// multipart body is streamed through a pipe, so the file is never held in memory; the
// writer goroutine always ends before this function returns.
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

	pr, pw := io.Pipe()
	form := multipart.NewWriter(pw)
	contentType := form.FormDataContentType()
	written := make(chan struct{})
	go func() {
		defer close(written)
		// LimitReader keeps the upload at the size that was checked, even if the file grows.
		pw.CloseWithError(writeTelegramDocumentForm(form, chatID, caption, name, telegramFileReader{io.LimitReader(f, size)}))
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
		return fmt.Errorf("telegram request failed: %s", telegramErrorText(err.Error(), token))
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := telegramUploadClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram request failed: %s", telegramErrorText(err.Error(), token))
	}
	defer resp.Body.Close()
	return checkTelegramReply(resp, token)
}

// writeTelegramDocumentForm writes the sendDocument form. Its error closes the pipe, so the
// request fails with it instead of sending a cut document.
func writeTelegramDocumentForm(form *multipart.Writer, chatID int64, caption, name string, content io.Reader) error {
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
	if _, err := io.Copy(part, content); err != nil {
		return err
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

// checkTelegramReply accepts only an answer that is 2xx and says ok:true. Telegram, or a
// proxy in between, can answer 200 with ok:false.
func checkTelegramReply(resp *http.Response, token string) error {
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
		return telegramReplyError("telegram did not accept the document", detail, token)
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
func sendTelegramFileText(cfg *config.Config, logger *slog.Logger, title, message string) error {
	if strings.TrimSpace(message) == "" {
		title, message = "", title
	}
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
// the bot token is replaced in every form a URL carries it, control characters become
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

// redactTelegramToken replaces the bot token, raw and URL-escaped (a *url.Error prints the
// request URL), with "***". Longer forms go first, so no form leaves a piece of another.
func redactTelegramToken(text, token string) string {
	if token == "" {
		return text
	}
	forms := []string{token, url.PathEscape(token), url.QueryEscape(token)}
	sort.Slice(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })
	for _, form := range forms {
		text = strings.ReplaceAll(text, form, "***")
	}
	return text
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
