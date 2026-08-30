package discord

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/security"
)

const discordAPIBaseURL = "https://discord.com/api/v10"

var discordConnectionHTTPClient = &http.Client{Timeout: 15 * time.Second}

// CheckBotToken performs a one-shot Discord REST authentication check. It does
// not open a Gateway session or request privileged intents.
func CheckBotToken(ctx context.Context, token string) error {
	return checkBotTokenAt(ctx, token, discordAPIBaseURL)
}

func checkBotTokenAt(ctx context.Context, token, baseURL string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("Discord bot token is not configured")
	}
	security.RegisterSensitive(token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/users/@me", nil)
	if err != nil {
		return errors.New("could not create the Discord API request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("User-Agent", "AuraGo")

	resp, err := discordConnectionHTTPClient.Do(req)
	if err != nil {
		return errors.New("Discord API request failed")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Discord API returned HTTP %d", resp.StatusCode)
	}
	return nil
}
