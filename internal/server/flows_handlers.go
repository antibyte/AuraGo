package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"aurago/internal/flows"
	"aurago/internal/i18n"
	"aurago/internal/security"
)

const (
	flowsDocBodyLimit   = 4 << 20
	flowsSmallBodyLimit = 256 << 10
)

func registerFlowsRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/desktop/flows", s.handleFlows)
	mux.HandleFunc("/api/desktop/flows/", s.handleFlows)
}

func flowsJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// flowsJSONScrubbed writes run data with registered secret values redacted.
func flowsJSONScrubbed(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", "the response cannot be encoded")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(security.Scrub(string(data))))
}

func flowsError(w http.ResponseWriter, status int, code, msg string) {
	flowsJSON(w, status, map[string]string{"error": msg, "code": code})
}

func flowsMethodNotAllowed(w http.ResponseWriter) {
	flowsError(w, http.StatusMethodNotAllowed, "FLOW_BAD_REQUEST", "method not allowed")
}

func nonNilIssues(issues []flows.Issue) []flows.Issue {
	if issues == nil {
		return []flows.Issue{}
	}
	return issues
}

// flowsErrorFrom maps service errors to the API error codes of the contract.
func flowsErrorFrom(w http.ResponseWriter, err error) {
	var ve *flows.ValidationError
	var ne *flows.NodeError
	switch {
	case errors.As(err, &ve):
		flowsJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "code": "FLOW_INVALID", "issues": nonNilIssues(ve.Issues)})
	case errors.Is(err, flows.ErrNotFound):
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", err.Error())
	case errors.Is(err, flows.ErrRunNotFound):
		flowsError(w, http.StatusNotFound, "FLOW_RUN_NOT_FOUND", err.Error())
	case errors.Is(err, flows.ErrRevisionConflict):
		flowsError(w, http.StatusConflict, "FLOW_REVISION_CONFLICT", err.Error())
	case errors.Is(err, flows.ErrNotPublished):
		flowsError(w, http.StatusConflict, "FLOW_NOT_PUBLISHED", err.Error())
	case errors.Is(err, flows.ErrNoTrigger):
		flowsError(w, http.StatusConflict, "FLOW_NO_TRIGGER", err.Error())
	case errors.Is(err, flows.ErrQueueFull):
		flowsError(w, http.StatusTooManyRequests, "FLOW_RUN_LIMIT", err.Error())
	case errors.Is(err, flows.ErrRunnerClosed):
		flowsError(w, http.StatusServiceUnavailable, "FLOWS_DISABLED", err.Error())
	case errors.Is(err, flows.ErrDocumentTooLarge), errors.Is(err, flows.ErrUnsupportedSchema):
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", err.Error())
	case errors.As(err, &ne):
		flowsError(w, http.StatusBadRequest, ne.Code, ne.Message)
	case strings.Contains(err.Error(), "mission is locked"):
		flowsError(w, http.StatusConflict, "FLOW_LOCKED", err.Error())
	default:
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", err.Error())
	}
}

func flowsDecode(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the request body is not valid JSON")
		return false
	}
	return true
}

// flowsOriginOK requires a same-origin request for session-authenticated writes.
func flowsOriginOK(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	if _, isBearer := bearerCredential(r.Header.Get("Authorization")); isBearer {
		return true
	}
	return checkCSRFOriginWithPolicy(r, true)
}

func (s *Server) flowsLang(r *http.Request) string {
	if lang := strings.TrimSpace(r.URL.Query().Get("lang")); lang != "" {
		return i18n.NormalizeLang(lang)
	}
	if cfg := s.ConfigSnapshot(); cfg != nil {
		return i18n.NormalizeLang(cfg.Server.UILanguage)
	}
	return "en"
}

func flowsTranslator(lang string) func(string) string {
	return func(key string) string { return i18n.T(lang, key) }
}

func flowsPathParts(path string) []string {
	rest := strings.Trim(strings.TrimPrefix(path, "/api/desktop/flows"), "/")
	if rest == "" {
		return nil
	}
	return strings.Split(rest, "/")
}

// handleFlows serves /api/desktop/flows/… (see the API contract in plan 1c).
func (s *Server) handleFlows(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopPermission(s, w, r, desktopMethodScope(r.Method)) {
		return
	}
	if !flowsOriginOK(r) {
		flowsError(w, http.StatusForbidden, "FLOW_PERMISSION_DENIED", "same-origin request required")
		return
	}
	if !s.flowsAvailable() {
		flowsError(w, http.StatusServiceUnavailable, "FLOWS_DISABLED", "EasyDrag flows are disabled")
		return
	}
	cfg := s.ConfigSnapshot()
	if r.Method != http.MethodGet && r.Method != http.MethodHead && (cfg.Tools.Missions.ReadOnly || cfg.VirtualDesktop.ReadOnly) {
		flowsError(w, http.StatusForbidden, "FLOW_PERMISSION_DENIED", "flows are read-only")
		return
	}
	s.flowsCatalog.refreshRegistry(s.Flows.Registry(), cfg)
	parts := flowsPathParts(r.URL.Path)
	switch {
	case len(parts) == 0:
		s.flowsCollection(w, r)
	case parts[0] == "secrets":
		s.flowsSecrets(w, r, parts[1:])
	default:
		s.flowRoute(w, r, parts[0], parts[1:])
	}
}

type flowCreateBody struct {
	Name     string          `json:"name"`
	Template string          `json:"template"`
	Import   json.RawMessage `json:"import"`
}

func (s *Server) flowsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Flows.ListFlows(r.Context())
		if err != nil {
			flowsErrorFrom(w, err)
			return
		}
		flowsJSON(w, http.StatusOK, map[string]any{"flows": list})
	case http.MethodPost:
		var body flowCreateBody
		if !flowsDecode(w, r, &body, flowsDocBodyLimit) {
			return
		}
		req := flows.CreateRequest{Name: body.Name, Template: body.Template, Translate: flowsTranslator(s.flowsLang(r))}
		if len(body.Import) > 0 && string(body.Import) != "null" {
			doc, err := flows.ParseFlow(body.Import)
			if err != nil {
				flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", err.Error())
				return
			}
			req.Import = doc
		}
		rec, err := s.Flows.CreateFlow(r.Context(), req)
		if err != nil {
			flowsErrorFrom(w, err)
			return
		}
		event := "flow_create"
		if req.Import != nil {
			event = "flow_import"
		}
		s.recordFlowAudit(event, rec.ID, rec.Name, "Flow "+rec.Name+" created")
		s.broadcastFlowsChanged(rec.ID, "created")
		flowsJSON(w, http.StatusCreated, map[string]any{"flow": rec})
	default:
		flowsMethodNotAllowed(w)
	}
}

func (s *Server) flowEnabled(rec *flows.FlowRecord) bool {
	return rec.MissionID != "" && flowMissionBridge{s: s}.FlowMissionEnabled(rec.MissionID)
}

func (s *Server) flowRoute(w http.ResponseWriter, r *http.Request, id string, rest []string) {
	ctx := r.Context()
	action := ""
	if len(rest) > 0 {
		action = rest[0]
	}
	switch action {
	case "":
		switch r.Method {
		case http.MethodGet:
			rec, err := s.Flows.GetFlow(ctx, id)
			if err != nil {
				flowsErrorFrom(w, err)
				return
			}
			issues := s.Flows.Validate(rec.Draft, flows.ModeDraft)
			flowsJSON(w, http.StatusOK, map[string]any{"flow": rec, "enabled": s.flowEnabled(rec), "issues": nonNilIssues(issues)})
		case http.MethodPut:
			var body struct {
				Doc          json.RawMessage `json:"doc"`
				BaseRevision int             `json:"base_revision"`
			}
			if !flowsDecode(w, r, &body, flowsDocBodyLimit) {
				return
			}
			doc, err := flows.ParseFlow(body.Doc)
			if err != nil {
				flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", err.Error())
				return
			}
			rev, issues, err := s.Flows.SaveDraft(ctx, id, doc, body.BaseRevision)
			if err != nil {
				flowsErrorFrom(w, err)
				return
			}
			s.broadcastFlowsChanged(id, "saved")
			flowsJSON(w, http.StatusOK, map[string]any{"draft_revision": rev, "issues": nonNilIssues(issues)})
		case http.MethodDelete:
			rec, err := s.Flows.GetFlow(ctx, id)
			if err != nil {
				flowsErrorFrom(w, err)
				return
			}
			if err := s.Flows.DeleteFlow(ctx, id); err != nil {
				flowsErrorFrom(w, err)
				return
			}
			s.recordFlowAudit("flow_delete", id, rec.Name, "Flow "+rec.Name+" deleted")
			s.broadcastFlowsChanged(id, "deleted")
			flowsJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			flowsMethodNotAllowed(w)
		}
	case "publish-preview":
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		preview, err := s.Flows.PublishPreview(ctx, id)
		if err != nil {
			flowsErrorFrom(w, err)
			return
		}
		flowsJSON(w, http.StatusOK, preview)
	case "publish":
		if r.Method != http.MethodPost {
			flowsMethodNotAllowed(w)
			return
		}
		var body struct {
			BaseRevision int `json:"base_revision"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit) {
			return
		}
		rec, issues, err := s.Flows.Publish(ctx, id, body.BaseRevision)
		if err != nil {
			flowsErrorFrom(w, err)
			return
		}
		s.recordFlowAudit("flow_publish", id, rec.Name, "Flow "+rec.Name+" published")
		s.broadcastFlowsChanged(id, "published")
		flowsJSON(w, http.StatusOK, map[string]any{"flow": rec, "issues": nonNilIssues(issues)})
	case "enabled":
		if r.Method != http.MethodPost {
			flowsMethodNotAllowed(w)
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit) {
			return
		}
		if err := s.Flows.SetEnabled(ctx, id, body.Enabled); err != nil {
			flowsErrorFrom(w, err)
			return
		}
		event := "flow_disable"
		if body.Enabled {
			event = "flow_enable"
		}
		s.recordFlowAudit(event, id, "", "Flow "+id+" switched")
		s.broadcastFlowsChanged(id, "enabled")
		flowsJSON(w, http.StatusOK, map[string]bool{"enabled": body.Enabled})
	case "export":
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		rec, err := s.Flows.GetFlow(ctx, id)
		if err != nil {
			flowsErrorFrom(w, err)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+flowExportName(rec.Name)+`"`)
		flowsJSON(w, http.StatusOK, rec.Draft)
	default:
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown flow route")
	}
}

// flowExportName turns a flow name into "<slug>.easydrag.json".
func flowExportName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "-")
	}
	if slug == "" {
		slug = "flow"
	}
	return slug + ".easydrag.json"
}

// flowsSecrets manages the flow secrets (vault entries "easydrag_<name>"). Values are
// write-only: the API lists names and never returns values.
func (s *Server) flowsSecrets(w http.ResponseWriter, r *http.Request, rest []string) {
	if s.Vault == nil {
		flowsError(w, http.StatusServiceUnavailable, "FLOWS_DISABLED", "the vault is not available")
		return
	}
	if len(rest) == 0 {
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		keys, err := s.Vault.ListKeys()
		if err != nil {
			flowsErrorFrom(w, err)
			return
		}
		names := []string{}
		for _, key := range keys {
			if name, ok := strings.CutPrefix(key, flowSecretPrefix); ok && flowSecretNamePattern.MatchString(name) {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		flowsJSON(w, http.StatusOK, map[string]any{"secrets": names})
		return
	}
	name := rest[0]
	if len(rest) != 1 || !flowSecretNamePattern.MatchString(name) {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "secret names use a-z, 0-9 and _ (at most 40 characters)")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var body struct {
			Value string `json:"value"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit) {
			return
		}
		if strings.TrimSpace(body.Value) == "" {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the secret value is empty")
			return
		}
		if err := s.Vault.WriteUserSecret(flowSecretPrefix+name, body.Value, true); err != nil {
			flowsErrorFrom(w, err)
			return
		}
		s.recordFlowAudit("flow_secret_set", "", name, "Flow secret "+name+" saved")
		flowsJSON(w, http.StatusOK, map[string]string{"status": "saved"})
	case http.MethodDelete:
		if err := s.Vault.DeleteSecret(flowSecretPrefix + name); err != nil && !errors.Is(err, security.ErrSecretNotFound) {
			flowsErrorFrom(w, err)
			return
		}
		s.recordFlowAudit("flow_secret_delete", "", name, "Flow secret "+name+" deleted")
		flowsJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		flowsMethodNotAllowed(w)
	}
}
