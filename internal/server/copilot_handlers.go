package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/llm"
)

// Copilot Device Code Flow handlers
// These endpoints implement the GitHub OAuth device-code flow for Copilot.

const copilotGitHubTokenVaultKey = "copilot_github_token"

// handleCopilotDeviceCode initiates the GitHub OAuth device-code flow.
// POST /api/copilot/device-code
// Returns JSON: {"device_code": "...", "user_code": "...", "verification_uri": "...", "expires_in": 900, "interval": 5}
func handleCopilotDeviceCode(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		auth := llm.NewCopilotAuth()
		resp, err := auth.RequestDeviceCode()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// handleCopilotPollToken polls GitHub for an access token using the device_code.
// POST /api/copilot/poll-token
// Body JSON: {"device_code": "..."}
// On success, stores the GitHub token in the vault and initializes the global CopilotAuth.
func handleCopilotPollToken(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			DeviceCode string `json:"device_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
		if body.DeviceCode == "" {
			jsonError(w, "Missing device_code", http.StatusBadRequest)
			return
		}

		auth := llm.NewCopilotAuth()
		resp, err := auth.PollForToken(body.DeviceCode)
		if err != nil {
			// If it's still pending, return the error text so the UI can keep polling
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "pending",
				"error":  err.Error(),
			})
			return
		}

		// Store GitHub token in vault
		if err := s.Vault.WriteSecret(copilotGitHubTokenVaultKey, resp.AccessToken); err != nil {
			jsonError(w, "Failed to store token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Initialize global CopilotAuth with the new token
		llm.InitCopilotAuth(resp.AccessToken)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "authorized",
		})
	}
}

// handleCopilotModels returns the list of available Copilot models.
// GET /api/copilot/models
// Uses the cached Copilot token to call https://api.githubcopilot.com/models
func handleCopilotModels(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		auth := llm.GetCopilotAuth()
		if auth == nil || !auth.HasGitHubToken() {
			// Try to load from vault
			if tok, err := s.Vault.ReadSecret(copilotGitHubTokenVaultKey); err == nil && tok != "" {
				llm.InitCopilotAuth(tok)
				auth = llm.GetCopilotAuth()
			}
		}
		if auth == nil || !auth.HasGitHubToken() {
			jsonError(w, "Copilot not authorized. Complete the device code flow first.", http.StatusUnauthorized)
			return
		}

		token, err := auth.GetToken()
		if err != nil {
			jsonError(w, "Failed to get Copilot token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		models, err := fetchCopilotModels(token)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": models,
		})
	}
}

// copilotModel represents a model from the Copilot /models endpoint.
type copilotModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func fetchCopilotModels(token string) ([]copilotModel, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://api.githubcopilot.com/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Version", llm.CopilotEditorVersion)
	req.Header.Set("Editor-Plugin-Version", llm.CopilotPluginVersion)
	req.Header.Set("Copilot-Integration-Id", llm.CopilotIntegrationID)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("copilot models request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot models returned HTTP %d", resp.StatusCode)
	}

	var apiResp struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("copilot models decode failed: %w", err)
	}

	models := make([]copilotModel, 0, len(apiResp.Data))
	for _, m := range apiResp.Data {
		// Filter out non-chat models (router endpoints)
		if strings.Contains(m.ID, "/routers/") {
			continue
		}
		models = append(models, copilotModel{
			ID:          "copilot/" + m.ID,
			Name:        m.Name,
			Description: m.Description,
		})
	}
	return models, nil
}
