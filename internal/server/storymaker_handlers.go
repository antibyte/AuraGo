package server

import (
	"context"
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
	"aurago/internal/phonon"
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
		case len(parts) == 1 && parts[0] == "voices":
			handleStorymakerVoices(s, w, r)
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
			case len(parts) == 3 && parts[2] == "music":
				handleStorymakerMusic(s, w, r, id)
			case len(parts) == 3 && parts[2] == "render":
				handleStorymakerRender(s, w, r, id)
			case len(parts) == 3 && parts[2] == "events":
				handleStorymakerEvents(s, w, r, id)
			case len(parts) == 3 && parts[2] == "jobs":
				handleStorymakerProjectJobs(s, w, r, id)
			case len(parts) == 3 && parts[2] == "narration":
				handleStorymakerNarration(s, w, r, id)
			case len(parts) == 3 && parts[2] == "make-silent":
				handleStorymakerMakeSilent(s, w, r, id)
			case len(parts) == 3 && parts[2] == "restore-speech":
				handleStorymakerRestoreSpeech(s, w, r, id)
			case len(parts) == 4 && parts[2] == "agent" && parts[3] == "revise":
				handleStorymakerAgentRevise(s, w, r, id)
			case len(parts) == 3 && parts[2] == "narrate":
				handleStorymakerNarrate(s, w, r, id)
			case len(parts) == 5 && parts[2] == "audio":
				handleStorymakerAudio(s, w, r, id, parts[3], parts[4])
			case len(parts) == 5 && parts[2] == "scenes" && parts[4] == "code":
				handleStorymakerCode(s, w, r, id, parts[3])
			case len(parts) == 5 && parts[2] == "scenes" && parts[4] == "bundle":
				handleStorymakerBundle(s, w, r, id, parts[3])
			default:
				writeStorymakerError(w, http.StatusNotFound, "not_found", "Storymaker endpoint not found.", nil)
			}
		case len(parts) == 2 && parts[0] == "agent" && parts[1] == "jobs":
			handleStorymakerAgentJobs(s, w, r)
		case len(parts) == 2 && parts[0] == "jobs":
			handleStorymakerJob(s, w, r, parts[1])
		case len(parts) == 3 && parts[0] == "jobs" && parts[2] == "cancel":
			handleStorymakerJobCancel(s, w, r, parts[1])
		default:
			writeStorymakerError(w, http.StatusNotFound, "not_found", "Storymaker endpoint not found.", nil)
		}
	}
}

// storymakerRefusal is a refused request without a ResponseWriter, so the routes, the agent job and
// the chat tool share one refusal order. Extra fields go next to the fixed three. ETag is the current
// version after a stale If-Match.
type storymakerRefusal struct {
	Status  int
	Code    string
	Message string
	Issues  []storymaker.Issue
	Extra   map[string]any
	ETag    string
}

// writeStorymakerRefusal answers a refusal in the flat error document of the desktop APIs.
func writeStorymakerRefusal(w http.ResponseWriter, refusal *storymakerRefusal) {
	if refusal.ETag != "" {
		w.Header().Set("ETag", refusal.ETag)
	}
	if refusal.Extra != nil {
		writeStorymakerErrorWith(w, refusal.Status, refusal.Code, refusal.Message, refusal.Extra)
		return
	}
	writeStorymakerError(w, refusal.Status, refusal.Code, refusal.Message, refusal.Issues)
}

// storymakerStoreRefusal maps a project store error to its refusal.
func storymakerStoreRefusal(err error, notFoundCode string) *storymakerRefusal {
	var decode *storymaker.DecodeError
	switch {
	case errors.Is(err, storymaker.ErrInvalidID):
		return &storymakerRefusal{Status: http.StatusBadRequest, Code: "invalid_id", Message: err.Error()}
	case errors.Is(err, storymaker.ErrInvalidArgument):
		return &storymakerRefusal{Status: http.StatusBadRequest, Code: "invalid_argument", Message: err.Error()}
	case errors.Is(err, storymaker.ErrTooLarge):
		return &storymakerRefusal{Status: http.StatusRequestEntityTooLarge, Code: "too_large", Message: err.Error()}
	case errors.Is(err, storymaker.ErrConflict):
		return &storymakerRefusal{Status: http.StatusPreconditionFailed, Code: "precondition_failed", Message: "The document changed since it was loaded."}
	case errors.Is(err, fs.ErrNotExist):
		return &storymakerRefusal{Status: http.StatusNotFound, Code: notFoundCode, Message: "Not found."}
	case errors.As(err, &decode):
		return &storymakerRefusal{Status: http.StatusUnprocessableEntity, Code: "invalid_story", Message: decode.Message, Issues: decode.Issues}
	}
	return &storymakerRefusal{Status: http.StatusInternalServerError, Code: "storage_failed", Message: "The project storage failed."}
}

// mapStorymakerError answers a project store error.
func mapStorymakerError(w http.ResponseWriter, err error, notFoundCode string) {
	writeStorymakerRefusal(w, storymakerStoreRefusal(err, notFoundCode))
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
	ff := rt.ffmpeg.status(r.Context(), cfg.VideoStudio.FFmpegPath)
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
	status["music_configured"] = cfg.MusicConfigured()
	status["limits"] = map[string]any{
		"max_duration_seconds":  cfg.Storymaker.Render.MaxDurationSeconds,
		"max_scenes":            lim.MaxScenes,
		"max_items_per_scene":   lim.MaxItemsPerScene,
		"max_beats_per_scene":   lim.MaxBeatsPerScene,
		"max_narration_chars":   lim.MaxNarrationChars,
		"max_story_bytes":       lim.MaxStoryBytes,
		"max_code_bytes":        lim.MaxCodeBytes,
		"max_queued_renders":    render.DefaultMaxQueued,
		"max_queued_narrations": storymakerMaxQueuedNarrations,
		"max_daily_chars":       cfg.Storymaker.Narration.MaxDailyChars,
		"max_job_chars":         cfg.Storymaker.Narration.MaxJobChars,
	}
	status["transport"] = render.ChooseTransport(cfg.Storymaker.Render.Transport)
	status["catalog_version"] = rt.catalog.Version()
	status["agent"] = s.storymakerAgentStatus(cfg)
	w.Header().Set("Cache-Control", "no-store")
	writeStorymakerJSON(w, http.StatusOK, status)
}

// storymakerDefaultVoiceFor is the default voice of a provider for a language, for
// ResolveProjectVoice. A list voice of that language wins, the provider's own default first, so
// piper does not start a German story with a voice of another language.
func storymakerDefaultVoiceFor(cfg *config.Config, providers []storymaker.VoiceProvider) func(provider, language string) string {
	return func(provider, language string) string {
		id := storymaker.NormalizeProvider(provider)
		if id == "phonon" {
			return phonon.DefaultVoice(language, cfg.Personality.CorePersonality)
		}
		base := storymaker.LanguageBase(language)
		for _, row := range providers {
			if row.ID != id {
				continue
			}
			first := ""
			for _, option := range row.Voices {
				if storymaker.LanguageBase(option.Language) != base {
					continue
				}
				if option.ID == row.DefaultVoice {
					return option.ID
				}
				if first == "" {
					first = option.ID
				}
			}
			if first != "" {
				return first
			}
			return row.DefaultVoice
		}
		return ""
	}
}

type storymakerCreateRequest struct {
	Title    string `json:"title"`
	Aspect   string `json:"aspect"`
	FPS      int    `json:"fps"`
	Language string `json:"language"`
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
			writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The request body must be {\"title\": ..., \"aspect\"?: ..., \"fps\"?: ..., \"language\"?: ...}.", nil)
			return
		}
		var voice *storymaker.Voice
		if language := strings.TrimSpace(req.Language); language != "" {
			providers := s.storymakerVoiceProviders(r.Context())
			resolved, languages, err := storymaker.ResolveProjectVoice(providers, language, storymakerDefaultVoiceFor(acc.cfg, providers))
			if err != nil {
				if languages == nil {
					languages = []string{}
				}
				writeStorymakerErrorWith(w, http.StatusUnprocessableEntity, render.CodeNarrationUnavailable, "No ready voice speaks this language.", map[string]any{"languages": languages})
				return
			}
			voice = &resolved
		}
		project, story, etag, err := acc.rt.projects.CreateWithVoice(r.Context(), req.Title, req.Aspect, req.FPS, voice)
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
		if has, err := acc.rt.projects.HasSilentBackup(ctx, id); err == nil && has {
			body["silent_backup"] = true
		}
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
		acc.rt.cancelProject(id)
		acc.rt.waitProject(r.Context(), id)
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
	if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
		return
	}
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
		if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
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
	if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key != "" && !storymakerIdempotencyKeyPattern.MatchString(key) {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 1 to 128 printable characters.", nil)
		return
	}
	const maxRenderOptionsBytes = 16 << 10
	optionsBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRenderOptionsBytes))
	if err != nil {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The export options are larger than 16 KiB.", nil)
		return
	}
	opts, err := render.ParseRenderOptions(optionsBody)
	if err != nil {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The export options are invalid: "+err.Error(), nil)
		return
	}
	job, _, refusal := s.storymakerEnqueueRender(r.Context(), acc, id, match, key, opts)
	if refusal != nil {
		writeStorymakerRefusal(w, refusal)
		return
	}
	writeStorymakerJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

// storymakerEnqueueRender runs the render gates in their fixed order and queues the export. ifMatch
// is the story etag, or "" to skip that check. The bool says the key replayed an earlier job.
func (s *Server) storymakerEnqueueRender(ctx context.Context, acc *storymakerAccess, id, ifMatch, key string, opts render.RenderOptions) (render.Job, bool, *storymakerRefusal) {
	if _, err := acc.rt.projects.Get(ctx, id); err != nil {
		return render.Job{}, false, storymakerStoreRefusal(err, "project_not_found")
	}
	settings, err := s.storymakerSettings(ctx)
	if err != nil {
		return render.Job{}, false, &storymakerRefusal{Status: http.StatusConflict, Code: render.CodeFFmpegUnavailable, Message: err.Error()}
	}
	if ff := videostudio.CheckRuntime(ctx, acc.cfg.VideoStudio.FFmpegPath); !ff.Ready {
		return render.Job{}, false, &storymakerRefusal{Status: http.StatusConflict, Code: render.CodeFFmpegUnavailable, Message: "FFmpeg is not ready: " + ff.Error}
	}
	if b := acc.rt.browsers.Status(ctx, settings.BrowserPath); !b.Ready && !b.CanDownload {
		return render.Job{}, false, &storymakerRefusal{Status: http.StatusConflict, Code: render.CodeBrowserUnavailable, Message: b.Issue}
	}
	plan, err := acc.rt.runner.Prepare(ctx, id, ifMatch, settings, opts)
	if err != nil {
		var pe *render.PrepareError
		if errors.As(err, &pe) {
			return render.Job{}, false, storymakerPrepareRefusal(ctx, acc, id, pe)
		}
		return render.Job{}, false, storymakerStoreRefusal(err, "project_not_found")
	}
	job, replayed, err := acc.rt.jobs.Enqueue(render.EnqueueRequest{ProjectID: id, Key: key, Fingerprint: id + "\x00" + ifMatch + "\x00" + opts.Fingerprint(), Payload: plan})
	switch {
	case errors.Is(err, render.ErrBusy):
		return render.Job{}, false, &storymakerRefusal{Status: http.StatusTooManyRequests, Code: render.CodeBusy, Message: "Sixteen exports are already waiting. Try again when one has finished."}
	case errors.Is(err, render.ErrIdempotencyConflict):
		return render.Job{}, false, &storymakerRefusal{Status: http.StatusConflict, Code: "idempotency_conflict", Message: "This Idempotency-Key was used for a different request."}
	case err != nil:
		return render.Job{}, false, &storymakerRefusal{Status: http.StatusInternalServerError, Code: "job_failed", Message: "The export could not be queued."}
	}
	return job, replayed, nil
}

// storymakerPrepareRefusal maps a refused Prepare to its refusal.
func storymakerPrepareRefusal(ctx context.Context, acc *storymakerAccess, id string, pe *render.PrepareError) *storymakerRefusal {
	switch {
	case pe.Stale:
		refusal := &storymakerRefusal{Status: http.StatusPreconditionFailed, Code: "precondition_failed", Message: pe.Message}
		if _, current, readErr := acc.rt.projects.LoadStory(ctx, id); readErr == nil {
			refusal.ETag = current
		}
		return refusal
	case pe.Code == render.CodeNarrationIncomplete:
		return &storymakerRefusal{Status: http.StatusConflict, Code: render.CodeNarrationIncomplete, Message: pe.Message, Extra: map[string]any{"missing": pe.Missing, "chars": pe.Chars}}
	case pe.Code == render.CodeMusicIncomplete:
		extra := map[string]any{"key": pe.MusicKey}
		if pe.MusicIssue != "" {
			extra["music_issue"] = pe.MusicIssue
		}
		return &storymakerRefusal{Status: http.StatusConflict, Code: render.CodeMusicIncomplete, Message: pe.Message, Extra: extra}
	case pe.Code == render.CodeCodeError:
		return &storymakerRefusal{Status: http.StatusUnprocessableEntity, Code: render.CodeCodeError, Message: pe.Message, Issues: pe.Issues}
	}
	return &storymakerRefusal{Status: http.StatusUnprocessableEntity, Code: "invalid_story", Message: pe.Message, Issues: pe.Issues}
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
	job, _, found := acc.rt.jobByID(jobID)
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
	jobs := acc.rt.listJobs(id)
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
	_, manager, found := acc.rt.jobByID(jobID)
	if !found {
		writeStorymakerError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
		return
	}
	job, found := manager.Cancel(jobID)
	if !found {
		writeStorymakerError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
		return
	}
	writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": job})
}

// handleStorymakerEvents streams job changes of a project as server-sent events: an event "job"
// with {"job": ...} for every state or progress change, from all queues. On connect it sends every
// unfinished job plus the merged latest five, so a reloaded app finds a running export. An event
// "agent" carries {job_id, seq, at, type, ...}: on connect the events of the unfinished agent job or
// else the latest one, then each new event. The client dedupes by job id and seq.
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
	renderJobs, audioJobs, agentJobs := acc.rt.jobs, acc.rt.narrate, acc.rt.agent
	renderEvents, unsubscribeRender := renderJobs.Subscribe(id)
	defer unsubscribeRender()
	audioEvents, unsubscribeAudio := audioJobs.Subscribe(id)
	defer unsubscribeAudio()
	agentEvents, unsubscribeAgent := agentJobs.Subscribe(id)
	defer unsubscribeAgent()
	agentLines, unsubscribeAgentLines := acc.rt.agentLog.subscribe(id)
	defer unsubscribeAgentLines()
	agentSeq := map[string]int{}
	sendAgentEvents := func() bool {
		job, ok := acc.rt.agentStreamJob(id)
		if !ok {
			return true
		}
		for _, line := range acc.rt.agentLog.after(job.ID, agentSeq[job.ID]) {
			if _, err := fmt.Fprintf(w, "event: agent\ndata: %s\n\n", line.Data); err != nil {
				return false
			}
			agentSeq[job.ID] = line.Seq
		}
		return true
	}
	revRender, revAudio, revAgent := renderJobs.Revision(), audioJobs.Revision(), agentJobs.Revision()
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
	initial := acc.rt.initialJobs(id)
	for i := len(initial) - 1; i >= 0; i-- { // oldest first
		if !send(initial[i]) {
			return
		}
	}
	if !sendAgentEvents() {
		return
	}
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-renderEvents:
			var changed []render.Job
			changed, revRender = renderJobs.ChangedSince(id, revRender)
			for _, job := range changed {
				if !send(job) {
					return
				}
			}
			flusher.Flush()
		case <-audioEvents:
			var changed []render.Job
			changed, revAudio = audioJobs.ChangedSince(id, revAudio)
			for _, job := range changed {
				if !send(job) {
					return
				}
			}
			flusher.Flush()
		case <-agentEvents:
			var changed []render.Job
			changed, revAgent = agentJobs.ChangedSince(id, revAgent)
			for _, job := range changed {
				if !send(job) {
					return
				}
			}
			flusher.Flush()
		case <-agentLines:
			if !sendAgentEvents() {
				return
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
