package tools

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

var oneDriveTokenRefreshMu sync.Mutex
var googleWorkspaceTokenRefreshMu sync.Mutex

// The Vault is authoritative even when the calling configuration predates a
// rotation. Missing/revoked/malformed stored tokens never reuse stale config.
func readIntegrationOAuth(v *security.Vault, key string) (config.OAuthToken, string, error) {
	raw, err := v.ReadSecret(key)
	if err != nil {
		return config.OAuthToken{}, "", fmt.Errorf("read OAuth state: %w", err)
	}
	var token config.OAuthToken
	if json.Unmarshal([]byte(raw), &token) != nil || token.AccessToken == "" {
		return token, "", fmt.Errorf("stored OAuth state is invalid; reconnect the integration")
	}
	if token.Expiry != "" {
		if _, err := time.Parse(time.RFC3339, token.Expiry); err != nil {
			return token, "", fmt.Errorf("stored OAuth expiry is invalid")
		}
	}
	security.RegisterSensitive(token.AccessToken)
	security.RegisterSensitive(token.RefreshToken)
	return token, raw, nil
}

func persistIntegrationOAuth(v *security.Vault, key, previous string, token config.OAuthToken) error {
	security.RegisterSensitive(token.AccessToken)
	security.RegisterSensitive(token.RefreshToken)
	if v == nil {
		return nil
	}
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("encode OAuth state: %w", err)
	}
	if err := v.CompareAndSwapSecret(key, previous, string(data)); err != nil {
		return fmt.Errorf("persist rotated OAuth state: %w; reconnect if the provider invalidated the old refresh token", err)
	}
	return nil
}
