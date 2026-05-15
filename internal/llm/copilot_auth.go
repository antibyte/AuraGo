package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// CopilotAuth implements the GitHub OAuth device-code flow and the subsequent
// GitHub-token → Copilot-token exchange used by GitHub Copilot.
//
// Flow overview:
//  1. Request device code from GitHub   (POST /login/device/code)
//  2. User visits verification URI and enters user_code
//  3. Poll for access token             (POST /login/oauth/access_token)
//  4. Exchange GitHub token for Copilot token (GET /copilot_internal/v2/token)
//  5. Cache Copilot token with expiry safety margin
//
// The Copilot token is short-lived (typically ~25 min). It is refreshed
// automatically when GetToken is called and the cached token is near expiry.
type CopilotAuth struct {
	mu          sync.RWMutex
	githubToken string        // the OAuth access token from GitHub
	copilotToken string       // the short-lived token for api.githubcopilot.com
	expiresAt   time.Time     // cached token expiry (already reduced by safety margin)
	client      *http.Client
}

// Global Copilot auth manager. Initialized at server startup when a Copilot
// provider is configured. The server calls InitCopilotAuth with the GitHub
// token loaded from the vault.
var copilotAuthInstance *CopilotAuth

// InitCopilotAuth creates and configures the global CopilotAuth instance.
func InitCopilotAuth(githubToken string) {
	copilotAuthInstance = NewCopilotAuth()
	if githubToken != "" {
		copilotAuthInstance.SetGitHubToken(githubToken)
	}
}

// GetCopilotAuth returns the global CopilotAuth instance (may be nil).
func GetCopilotAuth() *CopilotAuth {
	return copilotAuthInstance
}

// NewCopilotAuth creates a new CopilotAuth manager.
func NewCopilotAuth() *CopilotAuth {
	return &CopilotAuth{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// DeviceCodeResponse holds the initial response from GitHub's device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// PollTokenResponse holds the result of polling for an OAuth access token.
type PollTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
}

// CopilotTokenResponse holds the result of exchanging a GitHub token for a
// Copilot token.
type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"` // unix timestamp
}

const (
	githubDeviceCodeURL   = "https://github.com/login/device/code"
	githubAccessTokenURL  = "https://github.com/login/oauth/access_token"
	copilotTokenURL       = "https://api.github.com/copilot_internal/v2/token"
	githubOAuthClientID   = "Iv1.b507a08c87ecfe98" // GitHub Copilot's public client ID
	copilotExpiryBuffer   = 2 * time.Minute          // refresh 2 min before actual expiry
)

// RequestDeviceCode initiates the GitHub OAuth device-code flow.
// It returns the device_code, user_code, and verification URI that must be
// shown to the user.
func (ca *CopilotAuth) RequestDeviceCode() (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", githubOAuthClientID)
	data.Set("scope", "read:user")

	req, err := http.NewRequest(http.MethodPost, githubDeviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("copilot device code: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ca.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("copilot device code: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("copilot device code: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("copilot device code: decode response: %w", err)
	}
	return &result, nil
}

// PollForToken polls GitHub for an access token using the device_code.
// If the user has not yet authorized the device, it returns error with the
// "authorization_pending" text so the caller can continue polling.
func (ca *CopilotAuth) PollForToken(deviceCode string) (*PollTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", githubOAuthClientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequest(http.MethodPost, githubAccessTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("copilot poll token: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ca.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("copilot poll token: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("copilot poll token: read body: %w", err)
	}

	var result PollTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("copilot poll token: decode response: %w", err)
	}

	if result.Error != "" {
		return &result, fmt.Errorf("copilot poll token: %s", result.Error)
	}
	if result.AccessToken == "" {
		return &result, fmt.Errorf("copilot poll token: no access_token received")
	}

	ca.mu.Lock()
	ca.githubToken = result.AccessToken
	ca.copilotToken = "" // invalidate cached copilot token
	ca.expiresAt = time.Time{}
	ca.mu.Unlock()

	slog.Info("[Copilot] GitHub OAuth token obtained successfully")
	return &result, nil
}

// ExchangeGitHubTokenForCopilotToken exchanges a GitHub access token for a
// short-lived Copilot API token.
func (ca *CopilotAuth) ExchangeGitHubTokenForCopilotToken(githubToken string) (*CopilotTokenResponse, error) {
	req, err := http.NewRequest(http.MethodGet, copilotTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("copilot token exchange: build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")

	resp, err := ca.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("copilot token exchange: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("copilot token exchange: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result CopilotTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("copilot token exchange: decode response: %w", err)
	}
	return &result, nil
}

// GetToken returns a valid Copilot API token, refreshing from the cached
// GitHub token if necessary.
func (ca *CopilotAuth) GetToken() (string, error) {
	ca.mu.RLock()
	token := ca.copilotToken
	expires := ca.expiresAt
	githubTok := ca.githubToken
	ca.mu.RUnlock()

	// Return cached token if still valid (with safety buffer)
	if token != "" && time.Now().Before(expires) {
		return token, nil
	}

	if githubTok == "" {
		return "", fmt.Errorf("copilot: no GitHub token available; initiate device code flow first")
	}

	// Refresh Copilot token from GitHub token
	ca.mu.Lock()
	defer ca.mu.Unlock()

	// Double-check after acquiring write lock
	if ca.copilotToken != "" && time.Now().Before(ca.expiresAt) {
		return ca.copilotToken, nil
	}

	resp, err := ca.ExchangeGitHubTokenForCopilotToken(ca.githubToken)
	if err != nil {
		return "", err
	}

	ca.copilotToken = resp.Token
	if resp.ExpiresAt > 0 {
		ca.expiresAt = time.Unix(resp.ExpiresAt, 0).Add(-copilotExpiryBuffer)
	} else {
		// Fallback: assume 25 min expiry if header missing
		ca.expiresAt = time.Now().Add(23 * time.Minute)
	}

	slog.Debug("[Copilot] Token refreshed", "expires_at", ca.expiresAt)
	return ca.copilotToken, nil
}

// SetGitHubToken manually sets the GitHub access token (e.g. loaded from vault
// on startup). This allows GetToken to work without re-doing the device flow.
func (ca *CopilotAuth) SetGitHubToken(token string) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.githubToken = token
	ca.copilotToken = ""
	ca.expiresAt = time.Time{}
}

// HasGitHubToken reports whether a GitHub token has been configured.
func (ca *CopilotAuth) HasGitHubToken() bool {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ca.githubToken != ""
}

// GitHubToken returns the raw GitHub access token (for vault persistence).
func (ca *CopilotAuth) GitHubToken() string {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ca.githubToken
}
