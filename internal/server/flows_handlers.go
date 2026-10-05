package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"aurago/internal/flows"
	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"
)

const (
	flowsDocBodyLimit   = 4 << 20
	flowsSmallBodyLimit = 256 << 10
	// flowsLogPathRunes bounds the request path a flow API log line carries.
	flowsLogPathRunes = 200
	// flowsInternalMessage is the whole answer of a FLOW_INTERNAL error; the cause (a
	// path, SQL, driver or vault text) only goes to the log.
	flowsInternalMessage = "the flow service failed; see the server log"
	// flowPublishIncompleteMessage answers a publish that made the revision live but could
	// not update Mission Control or the timers (see flowPublishIncomplete).
	flowPublishIncompleteMessage = "Published, but Mission Control could not be updated: publish again to finish."
)

// flowsCollectionRoutes are the first path segments under /api/desktop/flows/ that name
// a collection route of the API contract, not a flow id.
var flowsCollectionRoutes = map[string]bool{"secrets": true, "runs": true, "node-types": true, "templates": true, "validate": true}

// flowIDPattern accepts what can be a flow id (flows.NewFlowID gives "flow_" and ten
// characters); anything else is FLOW_NOT_FOUND before the store is asked.
var flowIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// flowRouteSegments is the number of path segments after the flow id that each
// /api/desktop/flows/{id}/… route takes ("" is the flow itself). Any other path is
// FLOW_NOT_FOUND (404); a new route adds its entry here.
var flowRouteSegments = map[string]int{"": 0, "publish-preview": 1, "publish": 1, "enabled": 1, "export": 1}

// flowSecretValueMaxBytes bounds the value of a flow secret. Flow secrets are API tokens
// and passwords; the vault keeps all its secrets in one encrypted file.
const flowSecretValueMaxBytes = 16 << 10

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
//
// It scrubs the values, not the JSON text: value is encoded and decoded once into plain
// JSON values, which scrubFlowValue copies with every string, map key and number
// scrubbed. A secret holding a quote, a backslash or a control character is escaped in
// JSON text, where a text scrub misses it, and a redaction inside the text could break
// the JSON; the walk avoids both, so the answer is always valid JSON. The walk is not
// budgeted (the payloads are bounded by the stored outputs, at most
// flows.MaxStoredOutputBytes per step); numbers come back as float64.
func flowsJSONScrubbed(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", "the response cannot be encoded")
		return
	}
	var plain any
	if err := json.Unmarshal(data, &plain); err != nil {
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", "the response cannot be encoded")
		return
	}
	flowsJSON(w, status, scrubFlowValue(plain))
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

// flowsErrorFrom maps a flow service error to the API error codes of the contract (plan
// 1c, "HTTP API contract"), extended by FLOW_TOO_LARGE (413: a document, test data or a
// request body over its limit), FLOW_EXISTS (409) and FLOW_MISSION_AMBIGUOUS (409).
//
//   - A request whose client went away (r's context ended and err is that context's
//     error) gets no answer; it is logged at Debug.
//   - A mapped error echoes its text scrubbed and cut to flowErrorRunes runes (the flow
//     package already bounds the user data it quotes).
//   - Anything else is FLOW_INTERNAL with flowsInternalMessage only: the error can hold
//     file paths, SQL, driver or vault text, so it goes to the log at Warn, scrubbed and
//     bounded, with the route and the flow id.
func (s *Server) flowsErrorFrom(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		s.Logger.Debug("Flow API request ended by the client", "method", r.Method, "path", flowBoundRunes(r.URL.Path, flowsLogPathRunes))
		return
	}
	var ve *flows.ValidationError
	var ne *flows.NodeError
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &ve):
		flowsJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": flowsErrorText(err), "code": "FLOW_INVALID", "issues": nonNilIssues(ve.Issues)})
	case errors.Is(err, flows.ErrNotFound):
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", flowsErrorText(err))
	case errors.Is(err, flows.ErrRunNotFound):
		flowsError(w, http.StatusNotFound, "FLOW_RUN_NOT_FOUND", flowsErrorText(err))
	case errors.Is(err, flows.ErrRevisionConflict):
		flowsError(w, http.StatusConflict, "FLOW_REVISION_CONFLICT", flowsErrorText(err))
	case errors.Is(err, flows.ErrNotPublished):
		flowsError(w, http.StatusConflict, "FLOW_NOT_PUBLISHED", flowsErrorText(err))
	case errors.Is(err, flows.ErrNoTrigger):
		flowsError(w, http.StatusConflict, "FLOW_NO_TRIGGER", flowsErrorText(err))
	case errors.Is(err, flows.ErrFlowExists):
		flowsError(w, http.StatusConflict, "FLOW_EXISTS", flowsErrorText(err))
	case errors.Is(err, flows.ErrMissionAmbiguous):
		flowsError(w, http.StatusConflict, "FLOW_MISSION_AMBIGUOUS",
			"more than one flow is linked to the same Mission Control mission, so it is not clear which one is meant; delete the extra flow")
	case errors.Is(err, tools.ErrMissionLocked):
		flowsError(w, http.StatusConflict, "FLOW_LOCKED", "the flow's mission is locked in Mission Control; unlock it there first")
	case errors.Is(err, flows.ErrQueueFull):
		flowsError(w, http.StatusTooManyRequests, "FLOW_RUN_LIMIT", flowsErrorText(err))
	case errors.Is(err, flows.ErrRunnerClosed):
		flowsError(w, http.StatusServiceUnavailable, "FLOWS_DISABLED", flowsErrorText(err))
	case errors.Is(err, flows.ErrDocumentTooLarge):
		flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE", flowsDocumentTooLargeMessage())
	case errors.Is(err, flows.ErrTestDataTooLarge):
		flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE", flowsErrorText(err))
	case errors.As(err, &tooLarge):
		flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE", fmt.Sprintf("the request body is larger than %d KiB", tooLarge.Limit>>10))
	case errors.Is(err, flows.ErrUnsupportedSchema), errors.Is(err, flows.ErrUnknownTemplate):
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", flowsErrorText(err))
	case errors.As(err, &ne):
		code := ne.Code
		if code == "" {
			code = "FLOW_NODE_FAILED"
		}
		flowsError(w, http.StatusBadRequest, code, flowBoundRunes(security.Scrub(ne.Message), flowErrorRunes))
	default:
		s.Logger.Warn("Flow API request failed", "method", r.Method, "path", flowBoundRunes(r.URL.Path, flowsLogPathRunes),
			"flow_id", flowsLogFlowID(r), "error", flowsErrorText(err))
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
	}
}

// flowsErrorText is err's text for an answer or a log line: registered secrets and
// credential-looking pairs redacted (as flowScrubbedError does), cut to flowErrorRunes.
func flowsErrorText(err error) string {
	return flowBoundRunes(security.RedactSensitiveInfo(security.Scrub(err.Error())), flowErrorRunes)
}

func flowsDocumentTooLargeMessage() string {
	return fmt.Sprintf("the flow document is larger than %d MiB", flows.MaxDocumentBytes>>20)
}

// flowsDocumentError answers a document flows.ParseFlow refused: FLOW_TOO_LARGE (413)
// above flows.MaxDocumentBytes, else FLOW_BAD_REQUEST (400) with the bounded reason (a
// JSON decode error or an unsupported schema version).
func flowsDocumentError(w http.ResponseWriter, err error) {
	if errors.Is(err, flows.ErrDocumentTooLarge) {
		flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE", flowsDocumentTooLargeMessage())
		return
	}
	flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", flowsErrorText(err))
}

// flowsLogFlowID returns the flow id of a /api/desktop/flows/{id}/… request for a log line
// (bounded), or "" for the collection routes, which name no flow.
func flowsLogFlowID(r *http.Request) string {
	parts := flowsPathParts(r.URL.Path)
	if len(parts) == 0 || flowsCollectionRoutes[parts[0]] {
		return ""
	}
	return flowBoundRunes(parts[0], flowNameEchoRunes)
}

// flowsDecode reads a JSON body of at most limit bytes into dst. A larger body is
// FLOW_TOO_LARGE (413), anything that does not decode FLOW_BAD_REQUEST (400).
func flowsDecode(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE", fmt.Sprintf("the request body is larger than %d KiB", limit>>10))
			return false
		}
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
			s.flowsErrorFrom(w, r, err)
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
				flowsDocumentError(w, err)
				return
			}
			req.Import = doc
		}
		rec, err := s.Flows.CreateFlow(r.Context(), req)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
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
	if !flowIDPattern.MatchString(id) {
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "flow not found")
		return
	}
	action := ""
	if len(rest) > 0 {
		action = rest[0]
	}
	if n, ok := flowRouteSegments[action]; !ok || len(rest) != n {
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown flow route")
		return
	}
	switch action {
	case "":
		switch r.Method {
		case http.MethodGet:
			rec, err := s.Flows.GetFlow(ctx, id)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
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
				flowsDocumentError(w, err)
				return
			}
			rev, issues, err := s.Flows.SaveDraft(ctx, id, doc, body.BaseRevision)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return
			}
			s.broadcastFlowsChanged(id, "saved")
			flowsJSON(w, http.StatusOK, map[string]any{"draft_revision": rev, "issues": nonNilIssues(issues)})
		case http.MethodDelete:
			rec, err := s.Flows.GetFlow(ctx, id)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return
			}
			if err := s.Flows.DeleteFlow(ctx, id); err != nil {
				s.flowsErrorFrom(w, r, err)
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
			s.flowsErrorFrom(w, r, err)
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
		if err != nil && rec != nil {
			s.flowPublishIncomplete(w, rec, issues, err)
			return
		}
		if err != nil {
			s.flowsErrorFrom(w, r, err)
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
			s.flowsErrorFrom(w, r, err)
			return
		}
		event, state := "flow_disable", "disabled"
		if body.Enabled {
			event, state = "flow_enable", "enabled"
		}
		// The name for the audit entry comes from the lock-free read; the switch is done,
		// so a client that went away does not leave the entry without it.
		name, label := "", id
		if rec, err := s.Flows.GetFlow(context.WithoutCancel(ctx), id); err == nil {
			name, label = rec.Name, rec.Name
		}
		s.recordFlowAudit(event, id, name, "Flow "+label+" "+state)
		s.broadcastFlowsChanged(id, "enabled")
		flowsJSON(w, http.StatusOK, map[string]bool{"enabled": body.Enabled})
	case "export":
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		rec, err := s.Flows.GetFlow(ctx, id)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+flowExportName(rec.Name)+`"`)
		flowsJSON(w, http.StatusOK, rec.Draft)
	default:
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown flow route")
	}
}

// flowPublishIncomplete answers a publish that Service.Publish returned with a record AND
// an error: the store published the draft (the new revision is live and
// HasUnpublishedChanges is false), but the Mission Control sync or the timer update
// failed. The flow is broadcast as published and audited with status warning, and the
// answer is 200 with
//
//	{"flow": FlowRecord, "issues": [Issue], "partial": true,
//	 "code": "FLOW_PUBLISH_INCOMPLETE", "error": flowPublishIncompleteMessage}
//
// so the editor can tell it from a failure (an error status) and from a full publish (no
// "partial"). Publishing the same draft revision again repeats the update (the heal path
// of Service.Publish), so the editor keeps its Publish button. The cause stays in the log.
func (s *Server) flowPublishIncomplete(w http.ResponseWriter, rec *flows.FlowRecord, issues []flows.Issue, err error) {
	s.Logger.Warn("Flow published, but Mission Control could not be updated", "flow_id", rec.ID, "error", flowsErrorText(err))
	s.recordFlowAuditStatus("flow_publish", rec.ID, rec.Name, memory.AuditStatusWarning,
		"Flow "+rec.Name+" published; Mission Control update failed")
	s.broadcastFlowsChanged(rec.ID, "published")
	flowsJSON(w, http.StatusOK, map[string]any{"flow": rec, "issues": nonNilIssues(issues), "partial": true,
		"code": "FLOW_PUBLISH_INCOMPLETE", "error": flowPublishIncompleteMessage})
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
			s.flowsErrorFrom(w, r, err)
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
		trimmed := strings.TrimSpace(body.Value)
		if trimmed == "" {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the secret value is empty")
			return
		}
		if len(body.Value) > flowSecretValueMaxBytes {
			flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE",
				fmt.Sprintf("the secret value is larger than %d KiB", flowSecretValueMaxBytes>>10))
			return
		}
		// WriteUserSecretContext stores the value as not agent-readable and gives up when
		// the request ends while it waits for the vault lock.
		if err := s.Vault.WriteUserSecretContext(r.Context(), flowSecretPrefix+name, body.Value, true); err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		// Scrub the value (as stored and as nodes send it) from outputs from the first run
		// on, as flowSecrets.ReadSecret does when a run reads it.
		security.RegisterSensitive(body.Value)
		security.RegisterSensitive(trimmed)
		s.recordFlowAudit("flow_secret_set", "", name, "Flow secret "+name+" saved")
		flowsJSON(w, http.StatusOK, map[string]string{"status": "saved"})
	case http.MethodDelete:
		if err := s.Vault.DeleteSecret(flowSecretPrefix + name); err != nil && !errors.Is(err, security.ErrSecretNotFound) {
			s.flowsErrorFrom(w, r, err)
			return
		}
		s.recordFlowAudit("flow_secret_delete", "", name, "Flow secret "+name+" deleted")
		answer := map[string]any{"status": "deleted"}
		if users := s.flowSecretUsers(r.Context(), name); users != nil {
			answer["used_by"] = users
		}
		flowsJSON(w, http.StatusOK, answer)
	default:
		flowsMethodNotAllowed(w)
	}
}

// flowSecretUsers returns the names of the published flows (sorted) whose live revision
// passes the flow secret name to a secret_ref parameter of an enabled node: their live
// runs now fail with FLOW_SECRET_UNAVAILABLE, so the editor can warn after a delete. A
// template in the parameter is not resolved, so a name chosen at run time is not found.
// It reads the store without a flow lock. nil means the flows could not be read.
func (s *Server) flowSecretUsers(ctx context.Context, name string) []string {
	records, err := s.Flows.Store().ListFlows(context.WithoutCancel(ctx), flows.KindFlow)
	if err != nil {
		s.Logger.Warn("The flows using a deleted flow secret could not be listed", "error", flowsErrorText(err))
		return nil
	}
	reg := s.Flows.Registry()
	users := []string{}
	for _, rec := range records {
		if rec.Live != nil && flowUsesSecret(rec.Live, reg, name) {
			users = append(users, rec.Name)
		}
	}
	sort.Strings(users)
	return users
}

// flowUsesSecret reports whether an enabled node of doc names the flow secret in a
// secret_ref parameter of its type.
func flowUsesSecret(doc *flows.Flow, reg *flows.Registry, name string) bool {
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		if n.Settings.Disabled {
			continue
		}
		def, ok := reg.Lookup(n.Type)
		if !ok {
			continue
		}
		for _, p := range def.Params {
			if v, isText := n.Params[p.Name].(string); p.Kind == flows.ParamSecretRef && isText && strings.TrimSpace(v) == name {
				return true
			}
		}
	}
	return false
}
