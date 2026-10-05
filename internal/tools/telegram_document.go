package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
)

// telegramAPIBase is the Telegram Bot API endpoint; tests point it to a local server.
var telegramAPIBase = "https://api.telegram.org"

var telegramUploadClient = &http.Client{Timeout: 2 * time.Minute}

const (
	telegramMaxDocumentBytes = 50 << 20
	telegramMaxCaptionRunes  = 1024
)

// SendTelegramDocument uploads a local file as a document to the configured Telegram chat.
// The caller resolves and checks the path (see ResolveOutgoingAttachmentPath).
func SendTelegramDocument(ctx context.Context, cfg *config.Config, path, caption string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	token, chatID := cfg.Telegram.BotToken, cfg.Telegram.UserID
	if token == "" || chatID == 0 {
		return fmt.Errorf("telegram bot_token and telegram_user_id must be configured")
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open the file: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("read the file: %w", err)
	}
	if info.Size() > telegramMaxDocumentBytes {
		return fmt.Errorf("the file is larger than 50 MB, the Telegram upload limit")
	}
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	_ = form.WriteField("chat_id", strconv.FormatInt(chatID, 10))
	if caption = truncateRunes(strings.TrimSpace(caption), telegramMaxCaptionRunes); caption != "" {
		_ = form.WriteField("caption", caption)
	}
	part, err := form.CreateFormFile("document", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("read the file: %w", err)
	}
	if err := form.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramAPIBase+"/bot"+token+"/sendDocument", body)
	if err != nil {
		return fmt.Errorf("telegram request failed: %s", strings.ReplaceAll(err.Error(), token, "***"))
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := telegramUploadClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram request failed: %s", strings.ReplaceAll(err.Error(), token, "***"))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := readHTTPResponseBody(resp.Body, 64<<10)
		return fmt.Errorf("telegram returned HTTP %d: %s", resp.StatusCode, strings.ReplaceAll(string(data), token, "***"))
	}
	return nil
}

// SendTelegramFile sends a workspace or document file with an optional title and text.
// A text longer than a caption is sent as a message first. The result mirrors
// SendNotification: {"status", "results":[{"channel","status","detail"}], "document"}.
func SendTelegramFile(ctx context.Context, cfg *config.Config, logger *slog.Logger, filePath, title, message string) string {
	encode := func(v map[string]any) string {
		data, _ := json.Marshal(v)
		return string(data)
	}
	fail := func(err error) string {
		return encode(map[string]any{"status": "error", "message": err.Error(),
			"results": []map[string]string{{"channel": "telegram", "status": "error", "detail": err.Error()}}})
	}
	path, err := ResolveOutgoingAttachmentPath(filePath, cfg)
	if err != nil {
		return fail(err)
	}
	caption := strings.TrimSpace(strings.TrimSpace(title) + "\n" + strings.TrimSpace(message))
	if len([]rune(caption)) > telegramMaxCaptionRunes {
		if res := SendNotification(cfg, logger, "telegram", title, message, "normal", nil); strings.Contains(res, `"status":"error"`) {
			return res
		}
		caption = ""
	}
	if err := SendTelegramDocument(ctx, cfg, path, caption); err != nil {
		if logger != nil {
			logger.Warn("Telegram document could not be sent", "file", filepath.Base(path), "error", err)
		}
		return fail(err)
	}
	return encode(map[string]any{"status": "success", "document": filepath.Base(path),
		"results": []map[string]string{{"channel": "telegram", "status": "sent"}}})
}

func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit-1]) + "…"
}
