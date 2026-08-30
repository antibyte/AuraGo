package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/security"
)

const telegramBotAPIBaseURL = "https://api.telegram.org"

var telegramConnectionHTTPClient = &http.Client{Timeout: 15 * time.Second}

// CheckBotToken performs Telegram's read-only getMe authentication check.
// It deliberately does not start polling, delete webhooks, or send messages.
func CheckBotToken(ctx context.Context, token string) error {
	return checkBotTokenAt(ctx, token, telegramBotAPIBaseURL)
}

func checkBotTokenAt(ctx context.Context, token, baseURL string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("Telegram bot token is not configured")
	}
	security.RegisterSensitive(token)

	endpoint := strings.TrimRight(baseURL, "/") + "/bot" + url.PathEscape(token) + "/getMe"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return errors.New("could not create the Telegram API request")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := telegramConnectionHTTPClient.Do(req)
	if err != nil {
		return errors.New("Telegram API request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return errors.New("Telegram API returned an invalid response")
	}
	if !payload.OK {
		return errors.New("Telegram API rejected the bot token")
	}
	return nil
}
