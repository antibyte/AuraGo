package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/webhooks"
)

func webhookMaskSecrets(wh webhooks.Webhook, vault *security.Vault) webhooks.Webhook {
	if strings.TrimSpace(wh.Format.SignatureSecret) != "" {
		wh.Format.SignatureSecret = maskedKey
		return wh
	}
	if vault == nil {
		return wh
	}
	if secret, err := vault.ReadSecret(webhooks.SignatureSecretVaultKey(wh.ID)); err == nil && strings.TrimSpace(secret) != "" {
		wh.Format.SignatureSecret = maskedKey
	}
	return wh
}

// --- Token Admin API Handlers ---

func handleListTokens(tm *security.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tm.List())
	}
}

func handleCreateToken(tm *security.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name      string   `json:"name"`
			Scopes    []string `json:"scopes"`
			ExpiresAt *string  `json:"expires_at,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			jsonError(w, `{"error":"name is required"}`, http.StatusBadRequest)
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
					jsonError(w, `{"error":"invalid expires_at format"}`, http.StatusBadRequest)
					return
				}
			}
			expiresAt = &t
		}

		raw, meta, err := tm.Create(req.Name, req.Scopes, expiresAt)
		if err != nil {
			jsonError(w, "Failed to create token", http.StatusInternalServerError)
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/tokens/")
		if id == "" {
			jsonError(w, `{"error":"missing token id"}`, http.StatusBadRequest)
			return
		}
		var req struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := tm.Update(id, req.Name, req.Enabled); err != nil {
			jsonError(w, "Token not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	}
}

func handleDeleteToken(tm *security.TokenManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/tokens/")
		if id == "" {
			jsonError(w, `{"error":"missing token id"}`, http.StatusBadRequest)
			return
		}
		if err := tm.Delete(id); err != nil {
			jsonError(w, "Token not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Webhook Admin API Handlers ---

func handleListWebhooks(s *Server, mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		list := mgr.List()
		for i := range list {
			list[i] = webhookMaskSecrets(list[i], s.Vault)
		}
		json.NewEncoder(w).Encode(list)
	}
}

func handleCreateWebhook(s *Server, mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if rejectWebhookMutationIfReadOnly(w, s) {
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			jsonError(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		var wh webhooks.Webhook
		if err := json.Unmarshal(body, &wh); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		signatureSecret := strings.TrimSpace(wh.Format.SignatureSecret)
		if signatureSecret == maskedKey {
			jsonError(w, "A new webhook requires the actual signature secret", http.StatusBadRequest)
			return
		}
		if err := webhooks.ValidateSignatureConfiguration(wh.Format); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if signatureSecret != "" && s.Vault == nil {
			jsonError(w, "Vault unavailable for webhook signature secret", http.StatusServiceUnavailable)
			return
		}
		if signatureSecret != "" {
			wh.Format.SignatureSecret = ""
		}
		created, err := mgr.Create(wh)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "prompt template") {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			jsonError(w, "Failed to create webhook", http.StatusBadRequest)
			return
		}
		if signatureSecret != "" {
			if err := s.Vault.WriteSecret(webhooks.SignatureSecretVaultKey(created.ID), signatureSecret); err != nil {
				_ = mgr.Delete(created.ID)
				jsonError(w, "Failed to store webhook secret", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(webhookMaskSecrets(created, s.Vault))
	}
}

func handleUpdateWebhook(s *Server, mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if rejectWebhookMutationIfReadOnly(w, s) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/webhooks/")
		// Strip trailing sub-paths like "/log"
		if idx := strings.Index(id, "/"); idx >= 0 {
			id = id[:idx]
		}
		if id == "" {
			jsonError(w, `{"error":"missing webhook id"}`, http.StatusBadRequest)
			return
		}
		existing, err := mgr.Get(id)
		if err != nil {
			jsonError(w, "Webhook not found", http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			jsonError(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		var patch webhooks.Webhook
		if err := json.Unmarshal(body, &patch); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		updateOpts := webhookUpdateOptions(body)
		signatureSecret := strings.TrimSpace(patch.Format.SignatureSecret)
		keepExistingSecret := signatureSecret == maskedKey
		effectiveFormat, err := effectiveWebhookSignatureFormat(existing.Format, patch.Format, updateOpts, s.Vault, id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := webhooks.ValidateSignatureConfiguration(effectiveFormat); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if updateOpts.SignatureSecretSet && signatureSecret != "" && !keepExistingSecret && s.Vault == nil {
			jsonError(w, "Vault unavailable for webhook signature secret", http.StatusServiceUnavailable)
			return
		}

		var previousVaultSecret string
		previousVaultSecretExists := false
		vaultChanged := false
		vaultKey := webhooks.SignatureSecretVaultKey(id)
		if s.Vault != nil && updateOpts.SignatureSecretSet && !keepExistingSecret {
			previousVaultSecret, err = s.Vault.ReadSecret(vaultKey)
			previousVaultSecretExists = err == nil
			if signatureSecret != "" {
				err = s.Vault.WriteSecret(vaultKey, signatureSecret)
			} else {
				err = s.Vault.DeleteSecret(vaultKey)
			}
			if err != nil {
				jsonError(w, "Failed to update webhook secret", http.StatusInternalServerError)
				return
			}
			vaultChanged = true
		}
		if s.Vault != nil {
			patch.Format.SignatureSecret = ""
		} else if keepExistingSecret {
			patch.Format.SignatureSecret = existing.Format.SignatureSecret
		}
		updated, err := mgr.UpdateWithOptions(id, patch, updateOpts)
		if err != nil {
			if vaultChanged {
				restoreWebhookVaultSecret(s.Vault, vaultKey, previousVaultSecret, previousVaultSecretExists)
			}
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				jsonError(w, "Webhook not found", http.StatusNotFound)
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "prompt template") {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			jsonError(w, "Failed to update webhook", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(webhookMaskSecrets(updated, s.Vault))
	}
}

func effectiveWebhookSignatureFormat(existing, patch webhooks.WebhookFormat, opts webhooks.UpdateOptions, vault *security.Vault, webhookID string) (webhooks.WebhookFormat, error) {
	effective := existing
	if opts.SignatureHeaderSet {
		effective.SignatureHeader = patch.SignatureHeader
	}
	if opts.SignatureAlgoSet {
		effective.SignatureAlgo = patch.SignatureAlgo
	}

	secret := strings.TrimSpace(existing.SignatureSecret)
	if secret == "" && vault != nil {
		if value, err := vault.ReadSecret(webhooks.SignatureSecretVaultKey(webhookID)); err == nil {
			secret = strings.TrimSpace(value)
		}
	}
	if opts.SignatureSecretSet {
		switch strings.TrimSpace(patch.SignatureSecret) {
		case maskedKey:
			if secret == "" {
				return webhooks.WebhookFormat{}, fmt.Errorf("masked signature secret cannot be resolved")
			}
		case "":
			secret = ""
		default:
			secret = strings.TrimSpace(patch.SignatureSecret)
		}
	}
	effective.SignatureSecret = secret
	return effective, nil
}

func restoreWebhookVaultSecret(vault *security.Vault, key, value string, existed bool) {
	if vault == nil {
		return
	}
	if existed {
		_ = vault.WriteSecret(key, value)
		return
	}
	_ = vault.DeleteSecret(key)
}

func handleDeleteWebhook(s *Server, mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if rejectWebhookMutationIfReadOnly(w, s) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/webhooks/")
		if id == "" {
			jsonError(w, `{"error":"missing webhook id"}`, http.StatusBadRequest)
			return
		}
		if err := mgr.Delete(id); err != nil {
			jsonError(w, "Webhook not found", http.StatusNotFound)
			return
		}
		if s.Vault != nil {
			_ = s.Vault.DeleteSecret(webhooks.SignatureSecretVaultKey(id))
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleWebhookLog(mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Path: /api/webhooks/{id}/log
		path := strings.TrimPrefix(r.URL.Path, "/api/webhooks/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[1] != "log" {
			jsonError(w, "Not found", http.StatusNotFound)
			return
		}
		id := parts[0]
		entries := mgr.GetLog().ForWebhook(id, 50)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}
}

func handleTestWebhook(mgr *webhooks.Manager, _ *webhooks.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/webhooks/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[1] != "test" {
			jsonError(w, "Not found", http.StatusNotFound)
			return
		}
		id := parts[0]
		wh, err := mgr.Get(id)
		if err != nil {
			jsonError(w, "Webhook not found", http.StatusNotFound)
			return
		}
		// Return what the rendered prompt would look like with test data
		testPayload := `{"test":true,"message":"This is a test webhook event","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}`
		prepared, err := webhooks.PreparePayload([]byte(testPayload), "application/json", wh.Format.Fields, 4000)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		prompt, err := webhooks.RenderPromptPublic(wh, prepared.PromptPayload, prepared.Fields, map[string]string{"Content-Type": "application/json"})
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
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
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhooks.Presets())
}

// handleWebhookLogGlobal returns the most recent webhook log entries across all webhooks.
func handleWebhookLogGlobal(mgr *webhooks.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		entries := mgr.GetLog().Recent(100)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}
}

// --- Outgoing Webhooks Handlers ---

func handleOutgoingWebhooks(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetOutgoingWebhooks(s, w, r)
		case http.MethodPut:
			handlePutOutgoingWebhooks(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleGetOutgoingWebhooks(s *Server, w http.ResponseWriter, r *http.Request) {
	s.CfgMu.RLock()
	outgoing := append([]config.OutgoingWebhook(nil), s.Cfg.Webhooks.Outgoing...)
	s.CfgMu.RUnlock()

	if outgoing == nil {
		outgoing = []config.OutgoingWebhook{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config.MaskOutgoingWebhooks(outgoing))
}

func handlePutOutgoingWebhooks(s *Server, w http.ResponseWriter, r *http.Request) {
	if rejectWebhookMutationIfReadOnly(w, s) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var incoming []config.OutgoingWebhook
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.CfgMu.RLock()
	current := *s.Cfg
	current.Webhooks.Outgoing = append([]config.OutgoingWebhook(nil), s.Cfg.Webhooks.Outgoing...)
	configPath := current.ConfigPath
	s.CfgMu.RUnlock()

	if configPath == "" {
		jsonError(w, "Config path not set", http.StatusInternalServerError)
		return
	}

	if s.Vault == nil {
		jsonError(w, "Vault unavailable for outgoing webhook secrets", http.StatusServiceUnavailable)
		return
	}
	prepared, err := config.PrepareOutgoingWebhooks(incoming, current.Webhooks.Outgoing)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	newCfg, err := config.PersistOutgoingWebhooks(configPath, &current, prepared, s.Vault)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Error("Failed to persist outgoing webhooks", "error", err)
		}
		jsonError(w, "Failed to save outgoing webhooks", http.StatusInternalServerError)
		return
	}

	s.CfgMu.Lock()
	s.replaceConfigSnapshot(newCfg)
	s.CfgMu.Unlock()

	if s.Logger != nil {
		s.Logger.Info("[OutgoingWebhooks] Updated list", "count", len(prepared))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(prepared),
	})
}
