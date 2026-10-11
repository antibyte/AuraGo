package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"aurago/internal/cydcreator"
	"aurago/internal/desktop"
	"aurago/internal/security"
)

const (
	cydCreatorAPIPrefix    = "/api/desktop/cyd-creator/"
	cydCreatorDevicePrefix = "/api/cyd-creator/device/"
)

// cydRoute is one CYD Creator API route. Pattern is relative to the prefix;
// "{name}" matches one segment, "{name...}" the rest of the path.
//   - Write: bearer tokens need desktop:admin; refused while the desktop is
//     read-only (CYD_READONLY) and admitted through checkDesktopOperation.
//   - Stop: a cancel action; desktop:admin for bearer tokens but allowed while
//     the desktop is read-only (desktop invocation ownership).
//   - neither: a read (desktop:read).
type cydRoute struct {
	Method  string
	Pattern string
	Write   bool
	Stop    bool
	Handler func(w http.ResponseWriter, r *http.Request, params map[string]string)
}

func registerCYDCreatorRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc(cydCreatorAPIPrefix, s.handleCYDCreator)
	mux.HandleFunc(cydCreatorDevicePrefix, s.handleCYDCreatorDevice)
}

// cydCreatorRoutes is the desktop route table. Plan 3 appends its job,
// message and revision routes, plan 5 its device and transfer routes.
func cydCreatorRoutes(s *Server) []cydRoute {
	return slices.Concat(cydCreatorCoreRoutes(s))
}

// cydCreatorDeviceRoutes is the device table (/api/cyd-creator/device/,
// device-token auth, plan 5). Plan 2 has none.
func cydCreatorDeviceRoutes(s *Server) []cydRoute {
	return nil
}

var (
	cydIDParam   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	cydNameParam = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

// matchCYDRoute matches rel against pattern and returns the parameters.
func matchCYDRoute(pattern, rel string) (map[string]string, bool) {
	pp := strings.Split(pattern, "/")
	rp := strings.Split(rel, "/")
	params := map[string]string{}
	for i, seg := range pp {
		if i >= len(rp) {
			return nil, false
		}
		switch {
		case strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "...}"):
			rest := strings.Join(rp[i:], "/")
			if rest == "" {
				return nil, false
			}
			params[seg[1:len(seg)-4]] = rest
			return params, true
		case strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}"):
			if rp[i] == "" {
				return nil, false
			}
			params[seg[1:len(seg)-1]] = rp[i]
		case seg != rp[i]:
			return nil, false
		}
	}
	return params, len(pp) == len(rp)
}

// findCYDRoute returns the route for method and rel; pathFound reports a
// route with that path but another method.
func findCYDRoute(routes []cydRoute, method, rel string) (route cydRoute, params map[string]string, ok, pathFound bool) {
	for _, candidate := range routes {
		p, matched := matchCYDRoute(candidate.Pattern, rel)
		if !matched {
			continue
		}
		pathFound = true
		if candidate.Method == method || (method == http.MethodHead && candidate.Method == http.MethodGet) {
			return candidate, p, true, true
		}
	}
	return cydRoute{}, nil, false, pathFound
}

// cydAllowedMethods is the Allow header of a 405 for rel: the methods of the
// routes with that path (HEAD wherever GET is), sorted.
func cydAllowedMethods(routes []cydRoute, rel string) string {
	var methods []string
	for _, candidate := range routes {
		if _, matched := matchCYDRoute(candidate.Pattern, rel); !matched {
			continue
		}
		methods = append(methods, candidate.Method)
		if candidate.Method == http.MethodGet {
			methods = append(methods, http.MethodHead)
		}
	}
	slices.Sort(methods)
	return strings.Join(slices.Compact(methods), ", ")
}

// cydMethodNotAllowed writes the 405 answer with its Allow header.
func cydMethodNotAllowed(w http.ResponseWriter, routes []cydRoute, rel string) {
	w.Header().Set("Allow", cydAllowedMethods(routes, rel))
	cydWriteError(w, http.StatusMethodNotAllowed, cydcreator.CodeBadRequest, "method not allowed", nil)
}

func validCYDParams(params map[string]string) bool {
	for name, value := range params {
		switch name {
		case "path":
			// validated by the service (IsEditablePath / IsGeneratedPath)
		case "name":
			if !cydNameParam.MatchString(value) {
				return false
			}
		default:
			if !cydIDParam.MatchString(value) {
				return false
			}
		}
	}
	return true
}

// isCYDCreatorAPIPath reports the desktop CYD Creator API.
func isCYDCreatorAPIPath(path string) bool { return strings.HasPrefix(path, cydCreatorAPIPrefix) }

// cydCreatorBearerScope is the token scope validRouteBearer requires: reads
// (GET/HEAD and the read-only POST decode-backtrace) need desktop:read,
// everything else desktop:admin (spec §16).
func cydCreatorBearerScope(method, path string) string {
	if method == http.MethodGet || method == http.MethodHead {
		return desktopScopeRead
	}
	rel := strings.TrimPrefix(path, cydCreatorAPIPrefix)
	if _, ok := matchCYDRoute("builds/{id}/decode-backtrace", rel); ok && method == http.MethodPost {
		return desktopScopeRead
	}
	return desktopScopeAdmin
}

// cydCreatorOriginOK requires a same-origin request for session requests
// other than GET and HEAD (pattern flowsOriginOK; bearer clients send no
// cookies).
func cydCreatorOriginOK(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	if _, isBearer := bearerCredential(r.Header.Get("Authorization")); isBearer {
		return true
	}
	return checkCSRFOriginWithPolicy(r, true)
}

// cydCreatorUnavailable writes the CYD_DISABLED answer. When the feature is
// switched on but has no service, details say why, mirroring
// cydCreatorRestartReason: details.problem is the generic reason (no paths,
// no secrets) of a storage refusal of the live configuration (a restart would
// be refused again) or of a failed start; otherwise details.restart_required
// is true (switched on since startup, or the refusal was fixed since).
func (s *Server) cydCreatorUnavailable(w http.ResponseWriter) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.CYDCreator.Enabled || s.CYDCreator != nil {
		cydWriteError(w, http.StatusServiceUnavailable, cydcreator.CodeDisabled, "CYD Creator is switched off", nil)
		return
	}
	if problem := cydCreatorStorageProblem(cfg, desktop.ConfigFromAuraConfig(cfg).WorkspaceDir); problem != "" {
		cydWriteError(w, http.StatusServiceUnavailable, cydcreator.CodeDisabled, "CYD Creator is not available",
			map[string]any{"problem": problem})
		return
	}
	if s.cydCreatorProblem == cydCreatorStartFailed {
		cydWriteError(w, http.StatusServiceUnavailable, cydcreator.CodeDisabled, "CYD Creator is not available; restart AuraGo to try again",
			map[string]any{"problem": cydCreatorStartFailed, "restart_required": true})
		return
	}
	cydWriteError(w, http.StatusServiceUnavailable, cydcreator.CodeDisabled, "CYD Creator was switched on; restart AuraGo to start it",
		map[string]any{"restart_required": true})
}

// handleCYDCreator authenticates, applies the feature and read-only gates
// and dispatches through the route table.
func (s *Server) handleCYDCreator(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, cydCreatorAPIPrefix)
	route, params, ok, pathFound := findCYDRoute(cydCreatorRoutes(s), r.Method, rel)
	scope := desktopScopeRead
	if ok && (route.Write || route.Stop) {
		scope = desktopScopeAdmin
	}
	if !authenticateDesktopPermission(s, w, r, scope) {
		return
	}
	switch {
	case !ok && pathFound:
		cydMethodNotAllowed(w, cydCreatorRoutes(s), rel)
		return
	case !ok || !validCYDParams(params):
		cydWriteError(w, http.StatusNotFound, cydcreator.CodeNotFound, "unknown CYD Creator route", nil)
		return
	}
	if !s.cydCreatorAvailable() {
		s.cydCreatorUnavailable(w)
		return
	}
	if !cydCreatorOriginOK(r) {
		cydWriteError(w, http.StatusForbidden, cydcreator.CodeBadRequest, "cross-origin request refused", nil)
		return
	}
	if route.Write {
		if cfg := s.ConfigSnapshot(); cfg == nil || cfg.VirtualDesktop.ReadOnly {
			cydWriteError(w, http.StatusForbidden, cydcreator.CodeReadOnly, "the desktop is read-only", nil)
			return
		}
		if !checkDesktopOperation(s, w, r, desktopWrite) {
			return
		}
	} else if route.Stop && !checkDesktopOperation(s, w, r, desktopStop) {
		return
	}
	route.Handler(w, r, params)
}

// handleCYDCreatorDevice serves the device endpoints of plan 5.
func (s *Server) handleCYDCreatorDevice(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, cydCreatorDevicePrefix)
	route, params, ok, pathFound := findCYDRoute(cydCreatorDeviceRoutes(s), r.Method, rel)
	switch {
	case !ok && pathFound:
		cydMethodNotAllowed(w, cydCreatorDeviceRoutes(s), rel)
	case !ok || !validCYDParams(params):
		cydWriteError(w, http.StatusNotFound, cydcreator.CodeNotFound, "unknown CYD Creator device route", nil)
	case !s.cydCreatorAvailable():
		cydWriteError(w, http.StatusServiceUnavailable, cydcreator.CodeDisabled, "CYD Creator is switched off", nil)
	default:
		route.Handler(w, r, params)
	}
}

// cydCreatorHTTPStatus maps every CYD_* code to its HTTP status (overview
// "Error codes"); build-only codes answer 409 if they ever reach HTTP.
var cydCreatorHTTPStatus = map[string]int{
	cydcreator.CodeDisabled:            http.StatusServiceUnavailable,
	cydcreator.CodeDockerUnavailable:   http.StatusServiceUnavailable,
	cydcreator.CodeImageMissing:        http.StatusConflict,
	cydcreator.CodeImagePullFailed:     http.StatusBadGateway,
	cydcreator.CodeBadRequest:          http.StatusBadRequest,
	cydcreator.CodeNotFound:            http.StatusNotFound,
	cydcreator.CodeReadOnly:            http.StatusForbidden,
	cydcreator.CodeBusy:                http.StatusConflict,
	cydcreator.CodeProjectLocked:       http.StatusLocked,
	cydcreator.CodeProjectConflict:     http.StatusConflict,
	cydcreator.CodeInvalidLayout:       http.StatusUnprocessableEntity,
	cydcreator.CodeFileReadOnly:        http.StatusForbidden,
	cydcreator.CodeFileConflict:        http.StatusConflict,
	cydcreator.CodeFileLimit:           http.StatusRequestEntityTooLarge,
	cydcreator.CodeAssetTooLarge:       http.StatusRequestEntityTooLarge,
	cydcreator.CodeGeneratorError:      http.StatusInternalServerError,
	cydcreator.CodeBuildFailed:         http.StatusConflict,
	cydcreator.CodeBuildTimeout:        http.StatusConflict,
	cydcreator.CodeBuildCancelled:      http.StatusConflict,
	cydcreator.CodeBuildInterrupted:    http.StatusConflict,
	cydcreator.CodeFlashBudgetExceeded: http.StatusConflict,
	cydcreator.CodeJobNotFound:         http.StatusNotFound,
	cydcreator.CodeJobState:            http.StatusConflict,
	cydcreator.CodeDeviceNotFound:      http.StatusNotFound,
	cydcreator.CodeUnauthorized:        http.StatusUnauthorized,
	cydcreator.CodeRateLimited:         http.StatusTooManyRequests,
	cydcreator.CodeOTADisabled:         http.StatusServiceUnavailable,
	cydcreator.CodeImportInvalid:       http.StatusUnprocessableEntity,
	cydcreator.CodeTemplateTooNew:      http.StatusUnprocessableEntity,
	cydcreator.CodeInternal:            http.StatusInternalServerError,
}

// cydWriteJSON encodes before writing the status (an encode failure is a
// plain 500 CYD_INTERNAL).
func cydWriteJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		status = http.StatusInternalServerError
		data = []byte(`{"error":"the response cannot be encoded","code":"CYD_INTERNAL"}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

// cydWriteError writes the flows-style envelope {"error","code","details"}.
func cydWriteError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	body := map[string]any{"error": message, "code": code}
	if len(details) > 0 {
		body["details"] = details
	}
	cydWriteJSON(w, status, body)
}

// cydError maps a service error to the envelope. Untyped errors and typed
// CYD_INTERNAL become 500 CYD_INTERNAL with a generic message and no
// details; the cause is logged (scrubbed).
func (s *Server) cydError(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		cydWriteError(w, http.StatusServiceUnavailable, cydcreator.CodeInternal, "the request was cancelled", nil)
		return
	}
	var typed *cydcreator.Error
	if errors.As(err, &typed) && typed != nil && typed.Code != cydcreator.CodeInternal {
		status, ok := cydCreatorHTTPStatus[typed.Code]
		if !ok {
			status = http.StatusInternalServerError
		}
		cydWriteError(w, status, typed.Code, typed.Message, typed.Details)
		return
	}
	if s != nil && s.Logger != nil && err != nil {
		s.Logger.Warn("CYD Creator request failed", "method", r.Method, "path", r.URL.Path,
			"error", security.Scrub(err.Error()))
	}
	cydWriteError(w, http.StatusInternalServerError, cydcreator.CodeInternal, "internal error", nil)
}
