package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

func writeHereNowHandlerError(w http.ResponseWriter, err error) {
	var apiErr *tools.HereNowAPIError
	if errors.As(err, &apiErr) {
		status := apiErr.StatusCode
		if status < 400 || status > 599 {
			status = http.StatusBadGateway
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error", "http_code": apiErr.StatusCode, "error": apiErr.ProviderError,
			"code": apiErr.Code, "message": apiErr.Message, "details": apiErr.Details,
			"retry_after": apiErr.RetryAfter, "docs_url": apiErr.DocsURL,
		})
		return
	}
	w.WriteHeader(http.StatusBadGateway)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error()})
}

func hereNowClientFromServer(s *Server) (*tools.HereNowClient, error) {
	if s == nil || s.Vault == nil || s.Cfg == nil {
		return nil, errors.New("here.now configuration or Vault is unavailable")
	}
	apiKey, err := s.Vault.ReadSecret("here_now_api_key")
	if err != nil || strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("here.now API key not found in Vault")
	}
	return tools.NewHereNowClient(apiKey, s.Cfg.HereNow.DefaultAccount)
}

// handleHereNowStatus reports saved local readiness without consuming a provider request.
func handleHereNowStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if s == nil || s.Cfg == nil || !s.Cfg.HereNow.Enabled {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "disabled", "key_present": false})
			return
		}
		if s.Vault == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "no_key", "key_present": false})
			return
		}
		apiKey, err := s.Vault.ReadSecret("here_now_api_key")
		if err != nil || strings.TrimSpace(apiKey) == "" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "no_key", "key_present": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ready", "key_present": true, "default_account": s.Cfg.HereNow.DefaultAccount,
		})
	}
}

func handleHereNowTestConnection(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		client, err := hereNowClientFromServer(s)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		raw, err := client.ListAccounts(r.Context())
		if err != nil {
			writeHereNowHandlerError(w, err)
			return
		}
		var accounts interface{}
		_ = json.Unmarshal(raw, &accounts)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "accounts": accounts})
	}
}

func handleHereNowAccounts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		client, err := hereNowClientFromServer(s)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		raw, err := client.ListAccounts(r.Context())
		if err != nil {
			writeHereNowHandlerError(w, err)
			return
		}
		_, _ = w.Write(raw)
	}
}
