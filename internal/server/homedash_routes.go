package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
)

const (
	homeDashAPIPrefix      = "/api/desktop/homedash/"
	homeDashAPIVersion     = 1
	homeDashStreamProtocol = 1
	homeDashTemplateSystem = "system"

	homeDashSmallBody        int64 = 64 << 10
	homeDashSubscriptionBody int64 = 256 << 10
	homeDashDocumentBody     int64 = 512 << 10

	// homeDashCatalogWait bounds a request that may fetch catalogs (8 s per source plus slack).
	homeDashCatalogWait = 9 * time.Second
)

// HTTP-layer codes the core does not define (design section 12.3).
const (
	homeDashCodeNotFound           = "not_found"
	homeDashCodeMethodNotAllowed   = "method_not_allowed"
	homeDashCodeBodyTooLarge       = "body_too_large"
	homeDashCodeStorageUnavailable = "storage_unavailable"
	homeDashCodeTimeout            = "timeout"
	homeDashCodeInternal           = "internal_error"
)

// hdRequest is one routed HomeDash request with the configuration read once.
type hdRequest struct {
	s   *Server
	svc *homeDashService
	cfg *config.Config
	w   http.ResponseWriter
	r   *http.Request
}

// hdEndpoint is one method of a route. op desktopRead works on a read-only desktop; any other op
// passes checkDesktopOperation first (403 desktop_readonly on a read-only desktop).
type hdEndpoint struct {
	op desktopOperation
	fn func(*hdRequest)
}

// homeDashRoutes maps the path below homeDashAPIPrefix to its methods. Later tasks add rows.
var homeDashRoutes = map[string]map[string]hdEndpoint{
	"bootstrap":      {http.MethodGet: {desktopRead, (*hdRequest).bootstrap}},
	"widget-catalog": {http.MethodGet: {desktopRead, (*hdRequest).widgetCatalog}},
	"entities":       {http.MethodGet: {desktopRead, (*hdRequest).listEntities}},
	"entity":         {http.MethodGet: {desktopRead, (*hdRequest).getEntity}},
	"history":        {http.MethodGet: {desktopRead, (*hdRequest).history}},
	"snapshot":       {http.MethodGet: {desktopRead, (*hdRequest).snapshot}},
	"commands":       {http.MethodPost: {desktopWrite, (*hdRequest).command}},
	"dashboards":     {http.MethodGet: {desktopRead, (*hdRequest).listDashboards}, http.MethodPost: {desktopWrite, (*hdRequest).createDashboard}},
	"dashboard": {http.MethodGet: {desktopRead, (*hdRequest).getDashboard}, http.MethodPut: {desktopWrite, (*hdRequest).updateDashboard},
		http.MethodDelete: {desktopWrite, (*hdRequest).deleteDashboard}},
	"dashboards/validate":  {http.MethodPost: {desktopRead, (*hdRequest).validateDashboard}},
	"dashboard/duplicate":  {http.MethodPost: {desktopWrite, (*hdRequest).duplicateDashboard}},
	"dashboard/revisions":  {http.MethodGet: {desktopRead, (*hdRequest).listRevisions}},
	"dashboard/restore":    {http.MethodPost: {desktopWrite, (*hdRequest).restoreDashboard}},
	"dashboard/approve":    {http.MethodPost: {desktopWrite, (*hdRequest).approveDashboard}},
	"stream":               {http.MethodGet: {desktopRead, (*hdRequest).stream}},
	"stream/subscriptions": {http.MethodPut: {desktopRead, (*hdRequest).subscriptions}},
}

// handleHomeDash serves /api/desktop/homedash/. Order: desktop:admin (session or token) → route
// (404) → method (405) → availability (503) → desktop operation for writes (403) → handler.
func handleHomeDash(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !authenticateDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		methods, ok := homeDashRoutes[strings.TrimPrefix(r.URL.Path, homeDashAPIPrefix)]
		if !ok || !strings.HasPrefix(r.URL.Path, homeDashAPIPrefix) {
			homeDashWriteError(w, http.StatusNotFound, homeDashCodeNotFound, "unknown HomeDash route", nil)
			return
		}
		ep, ok := methods[r.Method]
		if !ok {
			w.Header().Set("Allow", homeDashAllow(methods))
			homeDashWriteError(w, http.StatusMethodNotAllowed, homeDashCodeMethodNotAllowed, "method not allowed", nil)
			return
		}
		cfg := s.ConfigSnapshot()
		if !homeDashEnabled(cfg) {
			homeDashWriteError(w, http.StatusServiceUnavailable, homedash.CodeHomeDashDisabled, "HomeDash is disabled", nil)
			return
		}
		svc := s.homeDash.Load()
		if svc == nil {
			homeDashWriteError(w, http.StatusServiceUnavailable, homeDashCodeStorageUnavailable, "HomeDash storage is unavailable", nil)
			return
		}
		if ep.op != desktopRead && !checkDesktopOperation(s, w, r, ep.op) {
			return
		}
		ep.fn(&hdRequest{s: s, svc: svc, cfg: cfg, w: w, r: r})
	}
}

// homeDashEnabled: the API runs with homedash and the virtual desktop on and outside egg mode.
func homeDashEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.HomeDash.Enabled && !cfg.EggMode.Enabled && cfg.VirtualDesktop.Enabled
}

func homeDashAllow(methods map[string]hdEndpoint) string {
	list := make([]string, 0, len(methods))
	for m := range methods {
		list = append(list, m)
	}
	sort.Strings(list)
	return strings.Join(list, ", ")
}

func homeDashWriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func homeDashWriteOK(w http.ResponseWriter, status int, fields map[string]any) {
	body := map[string]any{"status": "ok"}
	for k, v := range fields {
		body[k] = v
	}
	homeDashWriteJSON(w, status, body)
}

func homeDashWriteError(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	body := map[string]any{"status": "error", "code": code, "message": message}
	for k, v := range extra {
		if k != "status" && k != "code" && k != "message" {
			body[k] = v
		}
	}
	homeDashWriteJSON(w, status, body)
}

// homeDashStatus maps a code to its HTTP status (contract table "Error codes").
func homeDashStatus(code string) int {
	switch code {
	case homedash.CodeBadRequest, homedash.CodeInvalidID, homedash.CodeInvalidArgs, homedash.CodeUnsupportedCommand, homedash.CodePairingInvalid:
		return http.StatusBadRequest
	case homedash.CodeKioskUnpaired, homedash.CodeKioskRevoked:
		return http.StatusUnauthorized
	case homedash.CodeReadOnly, homedash.CodeForbiddenEntity, homedash.CodeViewOnly:
		return http.StatusForbidden
	case homedash.CodeDashboardNotFound, homedash.CodeEntityNotFound, homedash.CodeStreamNotFound,
		homedash.CodeRevisionNotFound, homedash.CodeKioskDisabled, homeDashCodeNotFound:
		return http.StatusNotFound
	case homeDashCodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case homedash.CodeRevisionConflict, homedash.CodeConfirmationRequired, homedash.CodeSchemaTooNew, homedash.CodeSlugTaken:
		return http.StatusConflict
	case homeDashCodeBodyTooLarge:
		return http.StatusRequestEntityTooLarge
	case homedash.CodeInvalidDocument, homedash.CodeInvalidPatch, homedash.CodeLimitExceeded:
		return http.StatusUnprocessableEntity
	case homedash.CodeRateLimited, homedash.CodeTooManyStreams:
		return http.StatusTooManyRequests
	case homedash.CodeCommandFailed, homedash.CodeSourceError:
		return http.StatusBadGateway
	case homedash.CodeHomeDashDisabled, homedash.CodeSourceUnavailable, homedash.CodeHubClosed, homeDashCodeStorageUnavailable:
		return http.StatusServiceUnavailable
	case homeDashCodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func (h *hdRequest) bad(message string) {
	homeDashWriteError(h.w, http.StatusBadRequest, homedash.CodeBadRequest, message, nil)
}

// fail answers an error: *homedash.Error by its code (with blocked_by, retry_after_ms + Retry-After,
// current_revision, issues), a finished request context as 504 timeout, anything else as a logged
// 500 internal_error that never carries the error text.
func (h *hdRequest) fail(err error) {
	var he *homedash.Error
	switch {
	case errors.As(err, &he):
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		he = &homedash.Error{Code: homeDashCodeTimeout, Message: "the request did not finish in time"}
	default:
		h.s.logHomeDash("Request failed: "+h.r.URL.Path, err)
		he = &homedash.Error{Code: homeDashCodeInternal, Message: "HomeDash could not complete the request"}
	}
	extra := map[string]any{}
	if he.BlockedBy != "" {
		extra["blocked_by"] = he.BlockedBy
	}
	if he.RetryAfterMS > 0 {
		extra["retry_after_ms"] = he.RetryAfterMS
		h.w.Header().Set("Retry-After", strconv.FormatInt((he.RetryAfterMS+999)/1000, 10))
	}
	if he.CurrentRevision > 0 {
		extra["current_revision"] = he.CurrentRevision
	}
	if len(he.Issues) > 0 {
		extra["issues"] = he.Issues
	}
	homeDashWriteError(h.w, homeDashStatus(he.Code), he.Code, he.Message, extra)
}

func (h *hdRequest) decode(dst any, limit int64) bool         { return h.decodeBody(dst, limit, false) }
func (h *hdRequest) decodeOptional(dst any, limit int64) bool { return h.decodeBody(dst, limit, true) }

// decodeBody reads one JSON body through the server's decodeJSONRequest; the route limit wins
// because the smaller MaxBytesReader wraps the body first (internal/server/AGENTS.md).
func (h *hdRequest) decodeBody(dst any, limit int64, optional bool) bool {
	if h.r.Body != nil {
		h.r.Body = http.MaxBytesReader(h.w, h.r.Body, limit)
	}
	err := decodeJSONRequest(h.w, h.r, dst, h.s)
	switch {
	case err == nil:
		return true
	case errors.Is(err, io.EOF) && optional:
		return true
	case errors.Is(err, io.EOF):
		h.bad("a JSON body is required")
	case jsonDecodeStatus(err) == http.StatusRequestEntityTooLarge:
		homeDashWriteError(h.w, http.StatusRequestEntityTooLarge, homeDashCodeBodyTooLarge, "the request body is too large", nil)
	default:
		h.bad("the body is not valid JSON for this endpoint")
	}
	return false
}

func (h *hdRequest) dashboardID() (string, bool) {
	id := h.r.URL.Query().Get("id")
	if !homedash.ValidDashboardID(id) {
		h.bad("id must be a dashboard id")
		return "", false
	}
	return id, true
}

func (h *hdRequest) revisionParam() (int64, bool) {
	rev, err := strconv.ParseInt(h.r.URL.Query().Get("revision"), 10, 64)
	if err != nil || rev < 1 {
		h.bad("revision must be a positive integer")
		return 0, false
	}
	return rev, true
}

func (h *hdRequest) entityID() (homedash.EntityID, bool) {
	raw := h.r.URL.Query().Get("id")
	if _, _, err := homedash.ParseEntityID(raw); err != nil {
		h.bad("id must be an entity id")
		return "", false
	}
	return homedash.EntityID(raw), true
}

// intParam reads an optional integer query parameter within lo..hi.
func (h *hdRequest) intParam(name string, def, lo, hi int) (int, bool) {
	raw := h.r.URL.Query().Get(name)
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < lo || n > hi {
		h.bad(name + " is out of range")
		return 0, false
	}
	return n, true
}

// homeDashPrincipal identifies the caller for rate limits, dedupe and stream ownership: a token
// digest, the session nonce digest (stable across session renewal), or the local desktop when login
// is off. The GET stream and its PUT subscriptions resolve to the same key in one browser.
func homeDashPrincipal(r *http.Request) homedash.Principal {
	key := "local-desktop"
	if token, bearer := bearerCredential(r.Header.Get("Authorization")); bearer && token != "" {
		key = "t:" + homeDashDigest(token)
	} else if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		key = "s:" + homeDashDigest(sessionNonce(c.Value))
	}
	return homedash.Principal{Kind: homedash.PrincipalSession, Key: key}
}

func homeDashDigest(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:16])
}

func (h *hdRequest) bootstrap() {
	hd, cat := h.cfg.HomeDash, h.svc.cat
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{
		"api_version": homeDashAPIVersion, "stream_protocol": homeDashStreamProtocol,
		"schema_version": homedash.SchemaVersion, "catalog_hash": cat.Hash,
		"readonly": hd.ReadOnly, "desktop_readonly": h.cfg.VirtualDesktop.ReadOnly, "kiosk_enabled": hd.Kiosk.Enabled,
		"limits": map[string]any{
			"dashboards": hd.Limits.MaxDashboards, "subscriptions": hd.Limits.MaxSubscriptions,
			"pages": cat.Limits.Pages, "widgets_per_page": cat.Limits.WidgetsPerPage, "widgets": cat.Limits.Widgets,
			"document_bytes": cat.Limits.DocumentBytes, "history_points": homedash.MaxHistoryPoints,
			"history_range_seconds": int(homedash.MaxHistoryRange / time.Second),
		},
		"templates": []string{homeDashTemplateSystem},
		"sources":   h.svc.hub.Sources(),
	})
}

func (h *hdRequest) widgetCatalog() {
	cat := h.svc.cat
	h.w.Header().Set("ETag", `"`+cat.Hash+`"`)
	if inm := h.r.Header.Get("If-None-Match"); inm != "" && strings.Contains(inm, cat.Hash) {
		h.w.WriteHeader(http.StatusNotModified)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"catalog": cat})
}
