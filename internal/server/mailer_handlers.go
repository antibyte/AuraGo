package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/contacts"
	"aurago/internal/mailer"
)

func registerMailerRoutes(mux *http.ServeMux, s *Server) {
	registerMailerAccountRoutes(mux, s)
	mux.HandleFunc("/api/desktop/mailer/", s.handleMailer)
}

func mailerJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeMailerJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return mailer.ErrInvalid
	}
	return nil
}

func mailerHTTPError(w http.ResponseWriter, err error) {
	code, status := "mailer_failed", http.StatusInternalServerError
	switch {
	case errors.Is(err, mailer.ErrNotFound):
		code, status = "not_found", http.StatusNotFound
	case errors.Is(err, mailer.ErrConflict):
		code, status = "revision_conflict", http.StatusConflict
	case errors.Is(err, mailer.ErrReadOnly):
		code, status = "read_only", http.StatusForbidden
	case errors.Is(err, mailer.ErrDisabled):
		code, status = "disabled", http.StatusForbidden
	case errors.Is(err, mailer.ErrInvalid):
		code, status = "invalid_request", http.StatusBadRequest
	case errors.Is(err, mailer.ErrCapability):
		code, status = "capability_unavailable", http.StatusForbidden
	case errors.Is(err, mailer.ErrStorageFull):
		code, status = "storage_full", http.StatusInsufficientStorage
	}
	mailerJSON(w, status, map[string]string{"error": code, "code": code})
}

func mailerOriginOK(r *http.Request) bool {
	if isSafeMethod(r.Method) {
		return true
	}
	if _, ok := bearerCredential(r.Header.Get("Authorization")); ok {
		return true
	}
	return checkCSRFOriginWithPolicy(r, true)
}

func (s *Server) handleMailer(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
		return
	}
	if !mailerOriginOK(r) {
		mailerJSON(w, http.StatusForbidden, map[string]string{"error": "same_origin_required"})
		return
	}
	if !isSafeMethod(r.Method) {
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.VirtualDesktop.Enabled {
			mailerHTTPError(w, mailer.ErrDisabled)
			return
		}
		if cfg.VirtualDesktop.ReadOnly {
			mailerHTTPError(w, mailer.ErrReadOnly)
			return
		}
	}
	if s.Mailer == nil {
		mailerJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mailer_unavailable"})
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/desktop/mailer/"), "/"), "/")
	if len(parts) == 1 {
		switch parts[0] {
		case "state":
			if r.Method == http.MethodGet {
				s.mailerState(w, r)
				return
			}
		case "settings":
			if r.Method == http.MethodPost {
				var settings mailer.Settings
				if err := decodeMailerJSON(w, r, &settings, 4096); err != nil {
					mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
					return
				}
				if err := s.Mailer.UpdateSettings(r.Context(), settings); err != nil {
					mailerHTTPError(w, err)
					return
				}
				mailerJSON(w, 200, settings)
				return
			}
		case "assist":
			s.handleMailerAssist(w, r)
			return
		case "sync":
			if r.Method == http.MethodPost {
				var body struct {
					AccountID string `json:"account_id"`
				}
				if err := decodeMailerJSON(w, r, &body, 4096); err != nil {
					mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
					return
				}
				if err := s.Mailer.SyncAccount(r.Context(), body.AccountID); err != nil {
					mailerHTTPError(w, err)
					return
				}
				mailerJSON(w, 200, map[string]bool{"ok": true})
				return
			}
		}
	}
	switch parts[0] {
	case "accounts":
		s.mailerAccountsHTTP(w, r, parts[1:])
	case "threads", "messages", "attachments":
		s.mailerMessagesHTTP(w, r, parts)
	case "drafts", "outbox":
		s.mailerDraftsHTTP(w, r, parts)
	case "templates", "signatures", "rules":
		s.mailerFeaturesHTTP(w, r, parts)
	default:
		mailerJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	}
}

func (s *Server) mailerState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	settings, err := s.Mailer.GetSettings(ctx)
	if err != nil {
		mailerHTTPError(w, err)
		return
	}
	accounts, err := s.Mailer.Accounts(ctx)
	if err != nil {
		mailerHTTPError(w, err)
		return
	}
	sources, err := s.Mailer.Sources(ctx)
	if err != nil {
		mailerHTTPError(w, err)
		return
	}
	folders, err := s.Mailer.Folders(ctx, "")
	if err != nil {
		mailerHTTPError(w, err)
		return
	}
	templates, err := s.Mailer.ListTemplates(ctx)
	if err != nil {
		mailerHTTPError(w, err)
		return
	}
	signatures := []mailer.Template{}
	regularTemplates := []mailer.Template{}
	for _, template := range templates {
		if template.Kind == "signature" {
			signatures = append(signatures, template)
		} else {
			regularTemplates = append(regularTemplates, template)
		}
	}
	rules, err := s.Mailer.ListRules(ctx)
	if err != nil {
		mailerHTTPError(w, err)
		return
	}
	accountSettings := map[string]mailer.AccountSettings{}
	for _, a := range accounts {
		v, e := s.Mailer.AccountSettings(ctx, a.ID)
		if e != nil {
			mailerHTTPError(w, e)
			return
		}
		accountSettings[a.ID] = v
	}
	contactChoices := []map[string]string{}
	if s.ContactsDB != nil {
		list, e := contacts.List(s.ContactsDB, "")
		if e == nil {
			for _, c := range list {
				if c.Email != "" {
					contactChoices = append(contactChoices, map[string]string{"name": c.Name, "email": c.Email})
				}
			}
		}
	}
	providers := []map[string]string{}
	readonly := true
	defaultProvider := ""
	assistPolicy := ""
	var assistGuardian map[string]string
	if cfg := s.ConfigSnapshot(); cfg != nil {
		readonly = cfg.VirtualDesktop.ReadOnly
		defaultProvider = cfg.LLM.Provider
		assistPolicy, assistGuardian = mailerAssistPolicy(cfg)
		for _, p := range cfg.Providers {
			if config.ProviderUsageAllowsGeneralChat(&p) {
				providers = append(providers, map[string]string{"id": p.ID, "name": p.Name, "model": p.Model})
			}
		}
	}
	mailerJSON(w, 200, map[string]any{"enabled": settings.Enabled, "readonly": readonly, "timezone": time.Local.String(), "accounts": accounts, "sources": sources, "account_settings": accountSettings, "folders": folders, "templates": regularTemplates, "signatures": signatures, "rules": rules, "contacts": contactChoices, "providers": providers, "default_provider_id": defaultProvider, "assist_guardian": assistGuardian, "assist_policy": assistPolicy, "attachment_limits": s.Mailer.AttachmentLimits()})
}

func (s *Server) mailerAccountsHTTP(w http.ResponseWriter, r *http.Request, parts []string) {
	ctx := r.Context()
	if len(parts) == 0 && r.Method == http.MethodPost {
		var body struct {
			SourceID string `json:"source_id"`
		}
		if err := decodeMailerJSON(w, r, &body, 4096); err != nil || body.SourceID == "" {
			mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		if err := s.Mailer.Connect(ctx, body.SourceID); err != nil {
			mailerHTTPError(w, err)
			return
		}
		mailerJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if len(parts) == 1 && parts[0] == "test" && r.Method == http.MethodPost {
		var body struct {
			AccountID string `json:"account_id"`
			Receive   bool   `json:"receive"`
			Send      bool   `json:"send"`
		}
		if err := decodeMailerJSON(w, r, &body, 4096); err != nil || body.AccountID == "" || (!body.Receive && !body.Send) {
			mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		s.mailerTestConnection(w, r, body.AccountID, body.Receive, body.Send)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := s.Mailer.Disconnect(ctx, parts[0]); err != nil {
			mailerHTTPError(w, err)
			return
		}
		mailerJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if len(parts) == 2 && parts[1] == "settings" && r.Method == http.MethodPut {
		var body mailer.AccountSettings
		if err := decodeMailerJSON(w, r, &body, 8192); err != nil {
			mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		if err := s.Mailer.SetAccountSettings(ctx, parts[0], body); err != nil {
			mailerHTTPError(w, err)
			return
		}
		mailerJSON(w, 200, body)
		return
	}
	if len(parts) == 2 && parts[1] == "folders" && r.Method == http.MethodPost {
		var body struct {
			Name     string `json:"name"`
			ParentID string `json:"parent_id"`
		}
		if err := decodeMailerJSON(w, r, &body, 4096); err != nil {
			mailerJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		folder, err := s.Mailer.CreateFolder(ctx, parts[0], body.Name, body.ParentID)
		if err != nil {
			mailerHTTPError(w, err)
			return
		}
		mailerJSON(w, 201, folder)
		return
	}
	mailerJSON(w, 404, map[string]string{"error": "not_found"})
}

func mailerQueryInt(r *http.Request, key string, def int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	return n
}
