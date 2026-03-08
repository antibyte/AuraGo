package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/security"
	"aurago/internal/webhooks"
)

// --- Token Admin API Handlers ---

func handleListTokens(tm *security.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tm.List())
	}
}

func handleCreateToken(tm *security.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name      string   `json:"name"`
			Scopes    []string `json:"scopes"`
			ExpiresAt *string  `json:"expires_at,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}
		if len(req.Scopes) == 0 {
			req.Scopes = []string{"webhook"}
		}

		var expiresAt *time.Time
		if req.ExpiresAt != nil && *req.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				t, err = time.Parse("2006-01-02", *req.ExpiresAt)
				if err != nil {
					http.Error(w, `{"error":"invalid expires_at format"}`, http.StatusBadRequest)
					return
				}
			}
			expiresAt = &t
		}

		raw, meta, err := tm.Create(req.Name, req.Scopes, expiresAt)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token": raw,
			"meta":  meta,
		})
	}
}

func handleUpdateToken(tm *security.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/tokens/")
		if id == "" {
			http.Error(w, `{"error":"missing token id"}`, http.StatusBadRequest)
			return
		}
		var req struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := tm.Update(id, req.Name, req.Enabled); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	}
}

func handleDeleteToken(tm *security.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/tokens/")
		if id == "" {
			http.Error(w, `{"error":"missing token id"}`, http.StatusBadRequest)
			return
		}
		if err := tm.Delete(id); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Webhook Admin API Handlers ---

func handleListWebhooks(mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mgr.List())
	}
}

func handleCreateWebhook(mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		var wh webhooks.Webhook
		if err := json.Unmarshal(body, &wh); err != nil {
			http.Error(w, `{"error":"invalid JSON: `+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		created, err := mgr.Create(wh)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	}
}

func handleUpdateWebhook(mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/webhooks/")
		// Strip trailing sub-paths like "/log"
		if idx := strings.Index(id, "/"); idx >= 0 {
			id = id[:idx]
		}
		if id == "" {
			http.Error(w, `{"error":"missing webhook id"}`, http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		var patch webhooks.Webhook
		if err := json.Unmarshal(body, &patch); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		updated, err := mgr.Update(id, patch)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
	}
}

func handleDeleteWebhook(mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/webhooks/")
		if id == "" {
			http.Error(w, `{"error":"missing webhook id"}`, http.StatusBadRequest)
			return
		}
		if err := mgr.Delete(id); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleWebhookLog(mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Path: /api/webhooks/{id}/log
		path := strings.TrimPrefix(r.URL.Path, "/api/webhooks/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[1] != "log" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		id := parts[0]
		entries := mgr.GetLog().ForWebhook(id, 50)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}
}

func handleTestWebhook(mgr *webhooks.Manager, handler *webhooks.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/webhooks/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[1] != "test" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		id := parts[0]
		wh, err := mgr.Get(id)
		if err != nil {
			http.Error(w, `{"error":"webhook not found"}`, http.StatusNotFound)
			return
		}
		// Return what the rendered prompt would look like with test data
		testPayload := `{"test":true,"message":"This is a test webhook event","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}`
		fields := webhooks.ExtractFieldsPublic([]byte(testPayload), wh.Format.Fields)
		prompt, err := webhooks.RenderPromptPublic(wh, testPayload, fields, map[string]string{"Content-Type": "application/json"})
		if err != nil {
			http.Error(w, `{"error":"template render failed: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":          "test",
			"rendered_prompt": prompt,
		})
	}
}

func handleWebhookPresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhooks.Presets())
}

// handleWebhookLogGlobal returns the most recent webhook log entries across all webhooks.
func handleWebhookLogGlobal(mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		entries := mgr.GetLog().Recent(100)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}
}
