package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/videostudio"
)

const storymakerAPIBase = "/api/desktop/storymaker"

// storymakerJSONLimit bounds small JSON request bodies (create project).
const storymakerJSONLimit = 16 << 10

var storymakerIdempotencyKeyPattern = regexp.MustCompile(`^[\x21-\x7e]{1,128}$`)

// storymakerAccess is what a handler gets after the gates passed.
type storymakerAccess struct {
	cfg *config.Config
	rt  *storymakerRuntime
	svc *desktop.Service
}

func writeStorymakerJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// issuesOrEmpty caps the issues for an HTTP answer and never returns nil (the JSON stays an array).
func issuesOrEmpty(issues []storymaker.Issue) []storymaker.Issue {
	out := storymaker.CapIssues(issues, storymaker.MaxHTTPIssues)
	if out == nil {
		return []storymaker.Issue{}
	}
	return out
}

// writeStorymakerError writes the flat error document of the desktop APIs:
// {"error": code, "code": code, "message": ..., "issues"?: [...]}.
func writeStorymakerError(w http.ResponseWriter, status int, code, message string, issues []storymaker.Issue) {
	body := map[string]any{"error": code, "code": code, "message": message}
	if len(issues) > 0 {
		body["issues"] = issuesOrEmpty(issues)
	}
	writeStorymakerJSON(w, status, body)
}

func writeStorymakerMethodError(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeStorymakerError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed for this endpoint.", nil)
}

// storymakerAuthorize runs the gates every endpoint shares: desktop scope and operation (read-only
// refuses mutations with 403 desktop_readonly), the Storymaker feature flag (403
// storymaker_disabled), the desktop being enabled and the runtime. withService also resolves the
// desktop workspace.
func storymakerAuthorize(s *Server, w http.ResponseWriter, r *http.Request, scope string, op desktopOperation, withService bool) (*storymakerAccess, bool) {
	if !requireDesktopOperation(s, w, r, scope, op) {
		return nil, false
	}
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "configuration_invalid", "The Storymaker configuration is invalid.", nil)
		return nil, false
	}
	if !cfg.Storymaker.Enabled {
		writeStorymakerError(w, http.StatusForbidden, "storymaker_disabled", "Storymaker is disabled in the configuration.", nil)
		return nil, false
	}
	if !cfg.VirtualDesktop.Enabled {
		writeStorymakerError(w, http.StatusServiceUnavailable, "desktop_disabled", "The virtual desktop is disabled in the configuration.", nil)
		return nil, false
	}
	rt, err := s.storymakerRuntime()
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "storage_unavailable", "Storymaker storage is unavailable: "+err.Error(), nil)
		return nil, false
	}
	acc := &storymakerAccess{cfg: cfg, rt: rt}
	if withService {
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			writeStorymakerError(w, http.StatusServiceUnavailable, "desktop_unavailable", "The virtual desktop storage is unavailable.", nil)
			return nil, false
		}
		rt.store.use(svc)
		acc.svc = svc
	}
	return acc, true
}

func handleStorymaker(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, storymakerAPIBase+"/") {
			writeStorymakerError(w, http.StatusNotFound, "not_found", "Storymaker endpoint not found.", nil)
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, storymakerAPIBase), "/"), "/")
		switch {
		case len(parts) == 1 && parts[0] == "status":
			handleStorymakerStatus(s, w, r)
		case len(parts) == 1 && parts[0] == "player":
			handleStorymakerPlayer(s, w, r)
		case len(parts) == 1 && parts[0] == "projects":
			handleStorymakerProjects(s, w, r)
		case len(parts) >= 2 && parts[0] == "projects":
			if !storymaker.ValidProjectID(parts[1]) {
				writeStorymakerError(w, http.StatusNotFound, "project_not_found", "Project not found.", nil)
				return
			}
			id := parts[1]
			switch {
			case len(parts) == 2:
				handleStorymakerProject(s, w, r, id)
			case len(parts) == 3 && parts[2] == "story":
				handleStorymakerStory(s, w, r, id)
			case len(parts) == 3 && parts[2] == "render":
				handleStorymakerRender(s, w, r, id)
			case len(parts) == 3 && parts[2] == "events":
				handleStorymakerEvents(s, w, r, id)
			case len(parts) == 3 && parts[2] == "jobs":
				handleStorymakerProjectJobs(s, w, r, id)
			case len(parts) == 5 && parts[2] == "scenes" && parts[4] == "code":
				handleStorymakerCode(s, w, r, id, parts[3])
			case len(parts) == 5 && parts[2] == "scenes" && parts[4] == "bundle":
				handleStorymakerBundle(s, w, r, id, parts[3])
			default:
				writeStorymakerError(w, http.StatusNotFound, "not_found", "Storymaker endpoint not found.", nil)
			}
		case len(parts) == 2 && parts[0] == "jobs":
			handleStorymakerJob(s, w, r, parts[1])
		case len(parts) == 3 && parts[0] == "jobs" && parts[2] == "cancel":
			handleStorymakerJobCancel(s, w, r, parts[1])
		default:
			writeStorymakerError(w, http.StatusNotFound, "not_found", "Storymaker endpoint not found.", nil)
		}
	}
}

// mapStorymakerError answers a project store error.
func mapStorymakerError(w http.ResponseWriter, err error, notFoundCode string) {
	var decode *storymaker.DecodeError
	switch {
	case errors.Is(err, storymaker.ErrInvalidID):
		writeStorymakerError(w, http.StatusBadRequest, "invalid_id", err.Error(), nil)
	case errors.Is(err, storymaker.ErrInvalidArgument):
		writeStorymakerError(w, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
	case errors.Is(err, storymaker.ErrTooLarge):
		writeStorymakerError(w, http.StatusRequestEntityTooLarge, "too_large", err.Error(), nil)
	case errors.Is(err, storymaker.ErrConflict):
		writeStorymakerError(w, http.StatusPreconditionFailed, "precondition_failed", "The document changed since it was loaded.", nil)
	case errors.Is(err, fs.ErrNotExist):
		writeStorymakerError(w, http.StatusNotFound, notFoundCode, "Not found.", nil)
	case errors.As(err, &decode):
		writeStorymakerError(w, http.StatusUnprocessableEntity, "invalid_story", decode.Message, decode.Issues)
	default:
		writeStorymakerError(w, http.StatusInternalServerError, "storage_failed", "The project storage failed.", nil)
	}
}

// storymakerETagPattern is the strong etag form: a quoted SHA-256.
var storymakerETagPattern = regexp.MustCompile(`^"[0-9a-f]{64}"$`)

// requireStorymakerMatch reads If-Match; it answers 428 without it and 400 for a malformed value.
func requireStorymakerMatch(w http.ResponseWriter, r *http.Request) (string, bool) {
	match := strings.TrimSpace(r.Header.Get("If-Match"))
	if match == "" {
		writeStorymakerError(w, http.StatusPreconditionRequired, "precondition_required", "Send the current version in If-Match.", nil)
		return "", false
	}
	if !storymakerETagPattern.MatchString(match) {
		writeStorymakerError(w, http.StatusBadRequest, "precondition_invalid", "If-Match must be a version taken from an earlier answer.", nil)
		return "", false
	}
	return match, true
}

func handleStorymakerStatus(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	if !requireDesktopOperation(s, w, r, desktopScopeRead, desktopRead) {
		return
	}
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "configuration_invalid", "The Storymaker configuration is invalid.", nil)
		return
	}
	status := map[string]any{
		"enabled":         cfg.Storymaker.Enabled,
		"read_only":       cfg.VirtualDesktop.ReadOnly,
		"desktop_enabled": cfg.VirtualDesktop.Enabled,
	}
	if !cfg.Storymaker.Enabled || !cfg.VirtualDesktop.Enabled {
		// Disabled: nothing is probed, the app is hidden anyway.
		writeStorymakerJSON(w, http.StatusOK, status)
		return
	}
	rt, err := s.storymakerRuntime()
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "storage_unavailable", "Storymaker storage is unavailable: "+err.Error(), nil)
		return
	}
	lim := storymakerLimits(cfg)
	ff := videostudio.CheckRuntime(r.Context(), cfg.VideoStudio.FFmpegPath)
	ffmpeg := map[string]any{"ready": ff.Ready}
	if ff.Version != "" {
		ffmpeg["version"] = storymakerFFmpegVersion(ff.Version)
	}
	if !ff.Ready && ff.Error != "" {
		ffmpeg["issue"] = ff.Error
	}
	browser := rt.browsers.Status(r.Context(), cfg.Storymaker.Render.BrowserPath)
	status["ffmpeg"] = ffmpeg
	status["browser"] = browser
	status["limits"] = map[string]any{
		"max_duration_seconds": cfg.Storymaker.Render.MaxDurationSeconds,
		"max_scenes":           lim.MaxScenes,
		"max_items_per_scene":  lim.MaxItemsPerScene,
		"max_beats_per_scene":  lim.MaxBeatsPerScene,
		"max_narration_chars":  lim.MaxNarrationChars,
		"max_story_bytes":      lim.MaxStoryBytes,
		"max_code_bytes":       lim.MaxCodeBytes,
		"max_queued_renders":   render.DefaultMaxQueued,
	}
	status["transport"] = render.ChooseTransport(cfg.Storymaker.Render.Transport)
	status["catalog_version"] = rt.catalog.Version()
	w.Header().Set("Cache-Control", "no-store")
	writeStorymakerJSON(w, http.StatusOK, status)
}

type storymakerCreateRequest struct {
	Title  string `json:"title"`
	Aspect string `json:"aspect"`
	FPS    int    `json:"fps"`
}

func handleStorymakerProjects(s *Server, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
		if !ok {
			return
		}
		projects, err := acc.rt.projects.List(r.Context())
		if err != nil {
			mapStorymakerError(w, err, "not_found")
			return
		}
		if projects == nil {
			projects = []storymaker.Project{}
		}
		acc.rt.warmBrowser(acc.cfg)
		writeStorymakerJSON(w, http.StatusOK, map[string]any{"projects": projects})
	case http.MethodPost:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
		if !ok {
			return
		}
		var req storymakerCreateRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, storymakerJSONLimit))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The request body must be {\"title\": ..., \"aspect\"?: ..., \"fps\"?: ...}.", nil)
			return
		}
		project, story, etag, err := acc.rt.projects.Create(r.Context(), req.Title, req.Aspect, req.FPS)
		if err != nil {
			mapStorymakerError(w, err, "not_found")
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Location", storymakerAPIBase+"/projects/"+project.ID)
		writeStorymakerJSON(w, http.StatusCreated, map[string]any{"project": project, "story": story, "etag": etag})
	default:
		writeStorymakerMethodError(w, "GET, POST")
	}
}

func handleStorymakerProject(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
		if !ok {
			return
		}
		ctx := r.Context()
		project, err := acc.rt.projects.Get(ctx, id)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		data, etag, err := acc.rt.projects.LoadStory(ctx, id)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		lim := storymakerLimits(acc.cfg)
		status, err := acc.rt.projects.CodeSceneStatus(ctx, id)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		if status == nil {
			status = []storymaker.CodeSceneStatus{}
		}
		body := map[string]any{"project": project, "etag": etag, "code_scenes": status}
		story, decodeIssues, derr := storymaker.DecodeStory(data, lim)
		if derr != nil {
			// A story.json edited by hand can be unreadable: the app shows the issues and the text.
			body["story"] = nil
			body["story_text"] = string(data)
			body["issues"] = issuesOrEmpty(decodeIssues)
		} else {
			checked := story
			storymaker.ApplyDefaults(&checked)
			issues := storymaker.Validate(&checked, acc.rt.catalog, lim)
			issues = append(issues, storymaker.CodeSceneIssues(&checked, status)...)
			body["story"] = story
			body["issues"] = issuesOrEmpty(issues)
		}
		w.Header().Set("ETag", etag)
		writeStorymakerJSON(w, http.StatusOK, body)
	case http.MethodDelete:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
		if !ok {
			return
		}
		if _, err := acc.rt.projects.Get(r.Context(), id); err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		acc.rt.jobs.CancelProject(id)
		if err := acc.rt.projects.Delete(r.Context(), id); err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeStorymakerMethodError(w, "GET, DELETE")
	}
}

// handleStorymakerStory saves story.json: decode strictly (an unreadable story is a 422 and is not
// saved), fill the static defaults, validate and write the canonical form. Validation findings do
// not block the save (a draft may be incomplete); they are returned, and a render needs none of
// severity error.
func handleStorymakerStory(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPut {
		writeStorymakerMethodError(w, "PUT")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	match, ok := requireStorymakerMatch(w, r)
	if !ok {
		return
	}
	lim := storymakerLimits(acc.cfg)
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(lim.MaxStoryBytes)+1))
	if err != nil {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The request body could not be read.", nil)
		return
	}
	story, issues, err := storymaker.DecodeStory(body, lim)
	if err != nil {
		var de *storymaker.DecodeError
		if errors.As(err, &de) {
			status, code := http.StatusUnprocessableEntity, "invalid_story"
			if de.Code == storymaker.CodeTooLarge {
				status, code = http.StatusRequestEntityTooLarge, "too_large"
			}
			writeStorymakerError(w, status, code, de.Message, de.Issues)
			return
		}
		writeStorymakerError(w, http.StatusUnprocessableEntity, "invalid_story", err.Error(), issues)
		return
	}
	defaults := storymaker.ApplyDefaults(&story)
	issues = storymaker.Validate(&story, acc.rt.catalog, lim)
	if status, serr := acc.rt.projects.CodeSceneStatus(r.Context(), id); serr == nil {
		issues = append(issues, storymaker.CodeSceneIssues(&story, status)...)
	}
	canonical, err := storymaker.CanonicalJSON(story)
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "encode_failed", "The story could not be encoded.", nil)
		return
	}
	etag, err := acc.rt.projects.SaveStory(r.Context(), id, canonical, match)
	if err != nil {
		if errors.Is(err, storymaker.ErrConflict) {
			// Tell the editor which version is current, so it can offer a reload.
			if _, current, readErr := acc.rt.projects.LoadStory(r.Context(), id); readErr == nil {
				w.Header().Set("ETag", current)
			}
		}
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	if defaults == nil {
		defaults = []storymaker.DefaultApplied{}
	}
	w.Header().Set("ETag", etag)
	writeStorymakerJSON(w, http.StatusOK, map[string]any{
		"etag": etag, "issues": issuesOrEmpty(issues), "defaults_applied": defaults, "story": story,
	})
}

func handleStorymakerCode(s *Server, w http.ResponseWriter, r *http.Request, id, sceneID string) {
	switch r.Method {
	case http.MethodGet:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
		if !ok {
			return
		}
		src, etag, err := acc.rt.projects.LoadCodeSource(r.Context(), id, sceneID)
		if err != nil {
			mapStorymakerError(w, err, "code_not_found")
			return
		}
		writeStorymakerScript(w, r, src, etag)
	case http.MethodPut:
		acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
		if !ok {
			return
		}
		ifMatch := ""
		switch {
		case strings.TrimSpace(r.Header.Get("If-None-Match")) == "*":
			ifMatch = storymaker.MustNotExist
		case strings.TrimSpace(r.Header.Get("If-Match")) != "":
			var good bool
			if ifMatch, good = requireStorymakerMatch(w, r); !good {
				return
			}
		default:
			writeStorymakerError(w, http.StatusPreconditionRequired, "precondition_required", "Send the current version in If-Match, or If-None-Match: * for a new code scene.", nil)
			return
		}
		lim := storymakerLimits(acc.cfg)
		src, err := io.ReadAll(io.LimitReader(r.Body, int64(lim.MaxCodeBytes)+1))
		if err != nil {
			writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The request body could not be read.", nil)
			return
		}
		if len(src) > lim.MaxCodeBytes {
			writeStorymakerError(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("A code scene is limited to %d bytes.", lim.MaxCodeBytes), nil)
			return
		}
		etag, res, err := acc.rt.projects.SaveCodeScene(r.Context(), id, sceneID, src, ifMatch)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		errs := res.Errors
		if errs == nil {
			errs = []storymaker.BundleError{}
		}
		w.Header().Set("ETag", etag)
		writeStorymakerJSON(w, http.StatusOK, map[string]any{"etag": etag, "ok": len(res.Errors) == 0, "errors": errs})
	default:
		writeStorymakerMethodError(w, "GET, PUT")
	}
}

func handleStorymakerBundle(s *Server, w http.ResponseWriter, r *http.Request, id, sceneID string) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
	if !ok {
		return
	}
	bundle, err := acc.rt.projects.LoadCodeBundle(r.Context(), id, sceneID)
	if err != nil {
		mapStorymakerError(w, err, "bundle_not_found")
		return
	}
	writeStorymakerScript(w, r, bundle, storymaker.ETag(bundle))
}

// writeStorymakerScript serves author-written JavaScript as inert text: a browser that navigates to
// the URL cannot run it (sandbox without allow-scripts, nosniff).
func writeStorymakerScript(w http.ResponseWriter, r *http.Request, body []byte, etag string) {
	h := w.Header()
	h.Set("Content-Type", "text/javascript; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "no-store")
	h.Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func handleStorymakerRender(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	match, ok := requireStorymakerMatch(w, r)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key != "" && !storymakerIdempotencyKeyPattern.MatchString(key) {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 1 to 128 printable characters.", nil)
		return
	}
	ctx := r.Context()
	if _, err := acc.rt.projects.Get(ctx, id); err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	settings, err := s.storymakerSettings(ctx)
	if err != nil {
		writeStorymakerError(w, http.StatusConflict, render.CodeFFmpegUnavailable, err.Error(), nil)
		return
	}
	if ff := videostudio.CheckRuntime(ctx, acc.cfg.VideoStudio.FFmpegPath); !ff.Ready {
		writeStorymakerError(w, http.StatusConflict, render.CodeFFmpegUnavailable, "FFmpeg is not ready: "+ff.Error, nil)
		return
	}
	if b := acc.rt.browsers.Status(ctx, settings.BrowserPath); !b.Ready && !b.CanDownload {
		writeStorymakerError(w, http.StatusConflict, render.CodeBrowserUnavailable, b.Issue, nil)
		return
	}
	plan, err := acc.rt.runner.Prepare(ctx, id, match, settings)
	if err != nil {
		var pe *render.PrepareError
		if errors.As(err, &pe) {
			switch {
			case pe.Stale:
				if _, current, readErr := acc.rt.projects.LoadStory(ctx, id); readErr == nil {
					w.Header().Set("ETag", current)
				}
				writeStorymakerError(w, http.StatusPreconditionFailed, "precondition_failed", pe.Message, nil)
			case pe.Code == render.CodeCodeError:
				writeStorymakerError(w, http.StatusUnprocessableEntity, render.CodeCodeError, pe.Message, pe.Issues)
			default:
				writeStorymakerError(w, http.StatusUnprocessableEntity, "invalid_story", pe.Message, pe.Issues)
			}
			return
		}
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	job, _, err := acc.rt.jobs.Enqueue(render.EnqueueRequest{ProjectID: id, Key: key, Fingerprint: id + "\x00" + match, Payload: plan})
	switch {
	case errors.Is(err, render.ErrBusy):
		writeStorymakerError(w, http.StatusTooManyRequests, render.CodeBusy, "Sixteen exports are already waiting. Try again when one has finished.", nil)
		return
	case errors.Is(err, render.ErrIdempotencyConflict):
		writeStorymakerError(w, http.StatusConflict, "idempotency_conflict", "This Idempotency-Key was used for a different request.", nil)
		return
	case err != nil:
		writeStorymakerError(w, http.StatusInternalServerError, "job_failed", "The export could not be queued.", nil)
		return
	}
	writeStorymakerJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

func handleStorymakerJob(s *Server, w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	job, found := acc.rt.jobs.Get(jobID)
	if !found {
		writeStorymakerError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
		return
	}
	writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": job})
}

// handleStorymakerProjectJobs lists the jobs of a project, newest first (an addition to the contract:
// the app needs it to find a running export after a reload).
func handleStorymakerProjectJobs(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	jobs := acc.rt.jobs.List(id)
	if jobs == nil {
		jobs = []render.Job{}
	}
	writeStorymakerJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

// handleStorymakerJobCancel maps to desktopStop: it works while the desktop is read-only, like the
// Video Studio cancel.
func handleStorymakerJobCancel(s *Server, w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopStop, false)
	if !ok {
		return
	}
	job, found := acc.rt.jobs.Cancel(jobID)
	if !found {
		writeStorymakerError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
		return
	}
	writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": job})
}

// handleStorymakerEvents streams job changes of a project as server-sent events: an event "job"
// with {"job": ...} for every state or progress change. On connect it sends the project's latest jobs
// so a reloaded app finds a running export.
func handleStorymakerEvents(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeStorymakerError(w, http.StatusInternalServerError, "streaming_unsupported", "Streaming is not supported.", nil)
		return
	}
	// A stream outlives the server's usual write window.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	_ = rc.SetReadDeadline(time.Time{})
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-store")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	jobs := acc.rt.jobs
	events, unsubscribe := jobs.Subscribe(id)
	defer unsubscribe()
	rev := jobs.Revision()
	send := func(job render.Job) bool {
		data, err := json.Marshal(map[string]any{"job": job})
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "event: job\ndata: %s\n\n", data); err != nil {
			return false
		}
		return true
	}
	if _, err := io.WriteString(w, ": storymaker events\n\n"); err != nil {
		return
	}
	initial := jobs.List(id)
	if len(initial) > 5 {
		initial = initial[:5]
	}
	for i := len(initial) - 1; i >= 0; i-- { // oldest first
		if !send(initial[i]) {
			return
		}
	}
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-events:
			var changed []render.Job
			changed, rev = jobs.ChangedSince(id, rev)
			for _, job := range changed {
				if !send(job) {
					return
				}
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
