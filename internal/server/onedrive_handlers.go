package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
)

// ── OneDrive Device Code Flow ───────────────────────────────────────────────

var onedriveHTTP = &http.Client{Timeout: 30 * time.Second}

// handleOneDriveAuthStart initiates the Microsoft Device Code flow.
// POST /api/onedrive/auth/start
// Returns JSON: {"user_code":"ABCD-EFGH","verification_uri":"https://microsoft.com/devicelogin","device_code":"...","expires_in":900,"interval":5}
func handleOneDriveAuthStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		clientID := s.Cfg.OneDrive.ClientID
		tenantID := s.Cfg.OneDrive.TenantID
		readOnly := s.Cfg.OneDrive.ReadOnly
		s.CfgMu.RUnlock()

		if clientID == "" {
			odWriteJSON(w, http.StatusBadRequest, map[string]string{"error": "OneDrive client_id is not configured"})
			return
		}
		if tenantID == "" {
			tenantID = "common"
		}

		scope := "Files.ReadWrite.All offline_access"
		if readOnly {
			scope = "Files.Read.All offline_access"
		}

		// Request device code from Microsoft
		deviceCodeURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", url.PathEscape(tenantID))
		form := url.Values{
			"client_id": {clientID},
			"scope":     {scope},
		}

		resp, err := onedriveHTTP.Post(deviceCodeURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			s.Logger.Error("[OneDrive] Device code request failed", "error", err)
			odWriteJSON(w, http.StatusBadGateway, map[string]string{"error": "Failed to contact Microsoft"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			s.Logger.Error("[OneDrive] Device code request error", "status", resp.StatusCode, "body", string(body))
			odWriteJSON(w, http.StatusBadGateway, map[string]string{"error": "Failed to start device authorization"})
			return
		}

		var dcResp struct {
			DeviceCode      string `json:"device_code"`
			UserCode        string `json:"user_code"`
			VerificationURI string `json:"verification_uri"`
			ExpiresIn       int    `json:"expires_in"`
			Interval        int    `json:"interval"`
			Message         string `json:"message"`
		}
		if err := json.Unmarshal(body, &dcResp); err != nil {
			odWriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse device code response"})
			return
		}

		// Store device_code in vault for the poll step
		if s.Vault != nil {
			stateData, _ := json.Marshal(map[string]string{
				"device_code": dcResp.DeviceCode,
				"created_at":  time.Now().UTC().Format(time.RFC3339),
			})
			_ = s.Vault.WriteSecret("onedrive_device_code", string(stateData))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user_code":        dcResp.UserCode,
			"verification_uri": dcResp.VerificationURI,
			"expires_in":       dcResp.ExpiresIn,
			"interval":         dcResp.Interval,
			"message":          dcResp.Message,
		})
	}
}

// handleOneDriveAuthPoll polls Microsoft's token endpoint using the device code.
// POST /api/onedrive/auth/poll
// Returns JSON: {"status":"pending"} or {"status":"authorized"} or {"status":"error","error":"..."}
func handleOneDriveAuthPoll(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.Vault == nil {
			odWriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Vault not available"})
			return
		}

		// Read stored device_code
		stateJSON, err := s.Vault.ReadSecret("onedrive_device_code")
		if err != nil || stateJSON == "" {
			odWriteJSON(w, http.StatusBadRequest, map[string]string{"error": "No pending device code — start auth first"})
			return
		}

		var state struct {
			DeviceCode string `json:"device_code"`
		}
		if err := json.Unmarshal([]byte(stateJSON), &state); err != nil || state.DeviceCode == "" {
			odWriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid device code state"})
			return
		}

		s.CfgMu.RLock()
		clientID := s.Cfg.OneDrive.ClientID
		tenantID := s.Cfg.OneDrive.TenantID
		s.CfgMu.RUnlock()

		if tenantID == "" {
			tenantID = "common"
		}

		// Poll token endpoint
		tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", url.PathEscape(tenantID))
		form := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"client_id":   {clientID},
			"device_code": {state.DeviceCode},
		}

		resp, err := onedriveHTTP.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
		if err != nil {
			s.Logger.Error("[OneDrive] Token poll request failed", "error", err)
			odWriteJSON(w, http.StatusBadGateway, map[string]string{"error": "Failed to contact Microsoft"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == 200 {
			// Success — extract tokens
			var tokenResp struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
				ExpiresIn    int    `json:"expires_in"`
				TokenType    string `json:"token_type"`
			}
			if err := json.Unmarshal(body, &tokenResp); err != nil {
				odWriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse token response"})
				return
			}

			expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

			// Store OAuthToken in vault (same pattern as Google Workspace)
			oauthToken := config.OAuthToken{
				AccessToken:  tokenResp.AccessToken,
				RefreshToken: tokenResp.RefreshToken,
				TokenType:    tokenResp.TokenType,
				Expiry:       expiry.Format(time.RFC3339),
			}
			tokenJSON, _ := json.Marshal(oauthToken)
			if err := s.Vault.WriteSecret("oauth_onedrive", string(tokenJSON)); err != nil {
				s.Logger.Error("[OneDrive] Failed to save token to vault", "error", err)
				odWriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save token"})
				return
			}

			// Update live config
			s.CfgMu.Lock()
			s.Cfg.OneDrive.AccessToken = tokenResp.AccessToken
			s.Cfg.OneDrive.RefreshToken = tokenResp.RefreshToken
			s.Cfg.OneDrive.TokenExpiry = expiry.Format(time.RFC3339)
			s.CfgMu.Unlock()

			// Clean up device code
			_ = s.Vault.DeleteSecret("onedrive_device_code")

			s.Logger.Info("[OneDrive] Device Code auth successful, token stored")
			odWriteJSON(w, http.StatusOK, map[string]string{"status": "authorized"})
			return
		}

		// Check for pending / error states
		var errResp struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &errResp)

		switch errResp.Error {
		case "authorization_pending":
			odWriteJSON(w, http.StatusOK, map[string]string{"status": "pending"})
		case "slow_down":
			odWriteJSON(w, http.StatusOK, map[string]string{"status": "slow_down"})
		case "authorization_declined":
			_ = s.Vault.DeleteSecret("onedrive_device_code")
			odWriteJSON(w, http.StatusOK, map[string]string{"status": "declined"})
		case "expired_token":
			_ = s.Vault.DeleteSecret("onedrive_device_code")
			odWriteJSON(w, http.StatusOK, map[string]string{"status": "expired"})
		default:
			s.Logger.Error("[OneDrive] Device authorization failed", "error", errResp.Error, "description", errResp.Description)
			odWriteJSON(w, http.StatusOK, map[string]string{"status": "error", "error": "Authorization failed"})
		}
	}
}

// handleOneDriveAuthStatus returns the current OneDrive authentication status.
// GET /api/onedrive/auth/status
func handleOneDriveAuthStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		accessToken := s.Cfg.OneDrive.AccessToken
		refreshToken := s.Cfg.OneDrive.RefreshToken
		tokenExpiry := s.Cfg.OneDrive.TokenExpiry
		s.CfgMu.RUnlock()

		if accessToken == "" {
			odWriteJSON(w, http.StatusOK, map[string]interface{}{
				"connected": false,
				"status":    "not_connected",
			})
			return
		}

		expired := false
		if tokenExpiry != "" {
			if exp, err := time.Parse(time.RFC3339, tokenExpiry); err == nil {
				expired = time.Now().After(exp)
			}
		}

		status := "connected"
		if expired && refreshToken != "" {
			status = "token_expired_refreshable"
		} else if expired {
			status = "token_expired"
		}

		odWriteJSON(w, http.StatusOK, map[string]interface{}{
			"connected":     true,
			"status":        status,
			"has_refresh":   refreshToken != "",
			"token_expired": expired,
		})
	}
}

// handleOneDriveAuthRevoke removes OneDrive tokens from the vault.
// DELETE /api/onedrive/auth/revoke
func handleOneDriveAuthRevoke(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.Vault != nil {
			_ = s.Vault.DeleteSecret("oauth_onedrive")
			_ = s.Vault.DeleteSecret("onedrive_device_code")
		}

		s.CfgMu.Lock()
		s.Cfg.OneDrive.AccessToken = ""
		s.Cfg.OneDrive.RefreshToken = ""
		s.Cfg.OneDrive.TokenExpiry = ""
		s.CfgMu.Unlock()

		s.Logger.Info("[OneDrive] Auth revoked, tokens deleted")
		odWriteJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
	}
}

// handleOneDriveTest tests the OneDrive connection by calling Microsoft Graph /me/drive.
// GET /api/onedrive/test
func handleOneDriveTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		accessToken := s.Cfg.OneDrive.AccessToken
		s.CfgMu.RUnlock()

		if accessToken == "" {
			odWriteJSON(w, http.StatusOK, map[string]interface{}{
				"ok":    false,
				"error": "Not connected — complete Device Code authorization first",
			})
			return
		}

		req, _ := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me/drive", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := onedriveHTTP.Do(req)
		if err != nil {
			s.Logger.Error("[OneDrive] Drive test request failed", "error", err)
			odWriteJSON(w, http.StatusOK, map[string]interface{}{
				"ok":    false,
				"error": "Connection failed",
			})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			s.Logger.Error("[OneDrive] Drive test returned non-200", "status", resp.StatusCode, "body", string(body))
			odWriteJSON(w, http.StatusOK, map[string]interface{}{
				"ok":    false,
				"error": "Microsoft Graph request failed",
			})
			return
		}

		var driveInfo struct {
			DriveType string `json:"driveType"`
			Owner     struct {
				User struct {
					DisplayName string `json:"displayName"`
				} `json:"user"`
			} `json:"owner"`
			Quota struct {
				Total     int64 `json:"total"`
				Used      int64 `json:"used"`
				Remaining int64 `json:"remaining"`
			} `json:"quota"`
		}
		_ = json.Unmarshal(body, &driveInfo)

		odWriteJSON(w, http.StatusOK, map[string]interface{}{
			"ok":             true,
			"drive_type":     driveInfo.DriveType,
			"owner":          driveInfo.Owner.User.DisplayName,
			"quota_total_mb": driveInfo.Quota.Total / (1024 * 1024),
			"quota_used_mb":  driveInfo.Quota.Used / (1024 * 1024),
		})
	}
}

// odWriteJSON is a small helper to write JSON responses for OneDrive handlers.
func odWriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
