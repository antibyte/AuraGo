package rocketchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
)

// CheckConnection performs Rocket.Chat's authenticated read-only /me request.
// It does not resolve channels or send a message.
func CheckConnection(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return errors.New("Rocket.Chat configuration is unavailable")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.RocketChat.URL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("Rocket.Chat URL is invalid")
	}
	if strings.TrimSpace(cfg.RocketChat.UserID) == "" || strings.TrimSpace(cfg.RocketChat.AuthToken) == "" {
		return errors.New("Rocket.Chat credentials are not configured")
	}
	security.RegisterSensitive(cfg.RocketChat.AuthToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/me", nil)
	if err != nil {
		return errors.New("could not create the Rocket.Chat API request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", cfg.RocketChat.AuthToken)
	req.Header.Set("X-User-Id", cfg.RocketChat.UserID)

	resp, err := rcHTTPClient.Do(req)
	if err != nil {
		return errors.New("Rocket.Chat API request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Rocket.Chat API returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Success *bool `json:"success"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return errors.New("Rocket.Chat API returned an invalid response")
	}
	if payload.Success != nil && !*payload.Success {
		return errors.New("Rocket.Chat API rejected the credentials")
	}
	return nil
}
