package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"github.com/google/uuid"
)

// storymakerFixture answers /api/desktop/storymaker like the server does (contract section 5), on top of the real
// storymaker package: the projects live in a MemStore, saves go through DecodeStory, ApplyDefaults, Validate and the
// project store, so a story that the fixture accepts is one the real server accepts, and code scenes are bundled by esbuild.
// What is faked is only the world around it: the capability flags, and the render job, which a test steps by hand
// (advance) so it can look at every state. The real handlers get their own tour in internal/server.
type storymakerFixture struct {
	mu       sync.Mutex
	t        *testing.T
	store    *storymaker.MemStore
	projects *storymaker.ProjectStore
	catalog  *storymaker.Catalog
	limits   storymaker.Limits

	ffmpegReady  bool
	browserReady bool
	readOnly     bool
	disabled     bool
	unauthorized bool

	requests []string
	bodies   map[string]string // last body per "METHOD route"
	jobs     map[string]*storymakerFixtureJob
	order    []string
	watchers map[string][]chan []byte
	steps    int // advance() calls so far
}

type storymakerFixtureJob struct {
	ID, ProjectID, Status, Phase, Slug string
	Progress                           float64
	FramesDone, FramesTotal            int
	OutputPath                         string
	ErrorCode                          string
	Created                            time.Time
}

func newStorymakerFixture(t *testing.T) *storymakerFixture {
	t.Helper()
	catalog, err := storymaker.LoadCatalog(os.DirFS("."), "img/storymaker/catalog.json")
	if err != nil {
		t.Fatalf("load the catalog: %v", err)
	}
	store := storymaker.NewMemStore()
	return &storymakerFixture{
		t: t, store: store, projects: storymaker.NewProjectStore(store, "Documents/Storymaker"), catalog: catalog, limits: storymaker.DefaultLimits(),
		ffmpegReady: true, browserReady: true, bodies: map[string]string{}, jobs: map[string]*storymakerFixtureJob{}, watchers: map[string][]chan []byte{},
	}
}

func (f *storymakerFixture) set(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

func (f *storymakerFixture) joined() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.requests, "\n")
}

func (f *storymakerFixture) body(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[key]
}

func (f *storymakerFixture) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (f *storymakerFixture) fail(w http.ResponseWriter, status int, code, message string, issues []storymaker.Issue) {
	body := map[string]any{"error": code, "code": code, "message": message}
	if len(issues) > 0 {
		body["issues"] = issues
	}
	f.writeJSON(w, status, body)
}

func fixtureProjectID(route string) (id, rest string) {
	trimmed := strings.TrimPrefix(route, "/projects/")
	id, rest, _ = strings.Cut(trimmed, "/")
	return id, rest
}

func (f *storymakerFixture) jobBody(job *storymakerFixtureJob) map[string]any {
	body := map[string]any{
		"id": job.ID, "project_id": job.ProjectID, "kind": "render", "status": job.Status, "progress": job.Progress, "phase": job.Phase,
		"frames_done": job.FramesDone, "frames_total": job.FramesTotal, "created_at": job.Created.UTC().Format(time.RFC3339), "updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if job.OutputPath != "" {
		body["output"] = map[string]any{"path": job.OutputPath, "size": 1234567}
	}
	if job.ErrorCode != "" {
		body["error"] = map[string]any{"code": job.ErrorCode, "message": "fixture failure"}
	}
	return body
}

// publish sends the job to every open event stream of its project. The caller holds f.mu.
func (f *storymakerFixture) publish(job *storymakerFixtureJob) {
	payload, _ := json.Marshal(map[string]any{"job": f.jobBody(job)})
	for _, ch := range f.watchers[job.ProjectID] {
		select {
		case ch <- payload:
		default:
		}
	}
}

// advance moves the newest active job one step: queued, running (compiling), rendering, encoding, succeeded.
func (f *storymakerFixture) advance() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.steps++
	for i := len(f.order) - 1; i >= 0; i-- {
		job := f.jobs[f.order[i]]
		if job.Status != "queued" && job.Status != "running" {
			continue
		}
		switch {
		case job.Status == "queued":
			job.Status, job.Phase, job.Progress = "running", "compiling", 0.1
		case job.Phase == "compiling":
			job.Phase, job.Progress, job.FramesTotal, job.FramesDone = "rendering", 0.5, 354, 177
		case job.Phase == "rendering":
			job.Phase, job.Progress, job.FramesDone = "encoding", 0.9, 354
		default:
			job.Status, job.Phase, job.Progress = "succeeded", "writing", 1
			job.OutputPath = fmt.Sprintf("Documents/Storymaker/%s/exports/%s-%s.mp4", job.ProjectID, job.Slug, job.ID[:8])
		}
		f.publish(job)
		return
	}
}

func (f *storymakerFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := strings.TrimPrefix(r.URL.Path, "/api/desktop/storymaker")
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+route)
	f.bodies[r.Method+" "+route] = string(raw)
	disabled, readOnly, unauthorized := f.disabled, f.readOnly, f.unauthorized
	f.mu.Unlock()

	if route == "/player" {
		f.player(w, r)
		return
	}
	if unauthorized {
		f.fail(w, http.StatusUnauthorized, "unauthorized", "Sign in.", nil)
		return
	}
	if disabled {
		f.fail(w, http.StatusForbidden, "storymaker_disabled", "Storymaker is switched off.", nil)
		return
	}
	if r.Method != http.MethodGet && readOnly && !strings.HasSuffix(route, "/cancel") {
		f.fail(w, http.StatusForbidden, "desktop_readonly", "The desktop is read-only.", nil)
		return
	}
	ctx := r.Context()
	switch {
	case route == "/status" && r.Method == http.MethodGet:
		f.status(w)
	case route == "/projects" && r.Method == http.MethodGet:
		list, err := f.projects.List(ctx)
		if err != nil {
			f.fail(w, http.StatusInternalServerError, "internal", err.Error(), nil)
			return
		}
		if list == nil {
			list = []storymaker.Project{}
		}
		f.writeJSON(w, http.StatusOK, map[string]any{"projects": list})
	case route == "/projects" && r.Method == http.MethodPost:
		f.create(w, ctx, raw)
	case strings.HasPrefix(route, "/projects/"):
		f.project(w, r, route, raw)
	case strings.HasPrefix(route, "/jobs/"):
		f.job(w, r, route)
	default:
		f.fail(w, http.StatusNotFound, "not_found", "No such route.", nil)
	}
}

func (f *storymakerFixture) status(w http.ResponseWriter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ffmpeg := map[string]any{"ready": f.ffmpegReady}
	if f.ffmpegReady {
		ffmpeg["version"] = "7.1"
	} else {
		ffmpeg["issue"] = "ffmpeg: executable file not found in $PATH"
	}
	browser := map[string]any{"ready": f.browserReady, "source": "system"}
	if !f.browserReady {
		browser["source"] = "missing"
		browser["issue"] = "no Chrome or Chromium found"
	}
	f.writeJSON(w, http.StatusOK, map[string]any{
		"enabled": true, "read_only": f.readOnly, "ffmpeg": ffmpeg, "browser": browser, "transport": "png", "catalog_version": f.catalog.Version(), "music_configured": false,
		"limits": map[string]any{"max_duration_seconds": f.limits.MaxDurationSeconds, "max_scenes": f.limits.MaxScenes, "max_narration_chars": f.limits.MaxNarrationChars},
	})
}

func (f *storymakerFixture) create(w http.ResponseWriter, ctx context.Context, raw []byte) {
	var in struct {
		Title  string `json:"title"`
		Aspect string `json:"aspect"`
		FPS    int    `json:"fps"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		f.fail(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
		return
	}
	project, story, etag, err := f.projects.Create(ctx, in.Title, in.Aspect, in.FPS)
	if err != nil {
		f.fail(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
		return
	}
	text, _ := storymaker.CanonicalJSON(story)
	f.writeJSON(w, http.StatusCreated, map[string]any{"project": project, "story": json.RawMessage(text), "etag": etag})
}

func (f *storymakerFixture) issuesFor(ctx context.Context, id string, data []byte) ([]storymaker.Issue, []storymaker.CodeSceneStatus) {
	story, _, err := storymaker.DecodeStory(data, f.limits)
	if err != nil {
		var decode *storymaker.DecodeError
		if errors.As(err, &decode) {
			return decode.Issues, nil
		}
		return nil, nil
	}
	storymaker.ApplyDefaults(&story)
	issues := storymaker.Validate(&story, f.catalog, f.limits)
	status, _ := f.projects.CodeSceneStatus(ctx, id)
	issues = append(issues, storymaker.CodeSceneIssues(&story, status)...)
	if status == nil {
		status = []storymaker.CodeSceneStatus{}
	}
	return storymaker.CapIssues(issues, storymaker.MaxHTTPIssues), status
}

func (f *storymakerFixture) project(w http.ResponseWriter, r *http.Request, route string, raw []byte) {
	ctx := r.Context()
	id, rest := fixtureProjectID(route)
	if !storymaker.ValidProjectID(id) {
		f.fail(w, http.StatusBadRequest, "bad_request", "invalid project id", nil)
		return
	}
	switch {
	case rest == "" && r.Method == http.MethodGet:
		project, err := f.projects.Get(ctx, id)
		if err != nil {
			f.notFound(w, err)
			return
		}
		data, etag, err := f.projects.LoadStory(ctx, id)
		if err != nil {
			f.notFound(w, err)
			return
		}
		issues, status := f.issuesFor(ctx, id, data)
		if issues == nil {
			issues = []storymaker.Issue{}
		}
		f.writeJSON(w, http.StatusOK, map[string]any{"project": project, "story": json.RawMessage(data), "etag": etag, "issues": issues, "code_scenes": status})
	case rest == "" && r.Method == http.MethodDelete:
		if err := f.projects.Delete(ctx, id); err != nil {
			f.notFound(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case rest == "story" && r.Method == http.MethodPut:
		f.saveStory(w, r, id, raw)
	case strings.HasPrefix(rest, "scenes/"):
		f.codeScene(w, r, id, strings.TrimPrefix(rest, "scenes/"), raw)
	case rest == "narration" && r.Method == http.MethodGet:
		f.narration(w, ctx, id)
	case rest == "render" && r.Method == http.MethodPost:
		f.render(w, r, id, raw)
	case rest == "music" && r.Method == http.MethodPost:
		f.writeJSON(w, http.StatusOK, map[string]any{"job": nil})
	case rest == "jobs" && r.Method == http.MethodGet:
		f.jobList(w, id)
	case rest == "events" && r.Method == http.MethodGet:
		f.events(w, r, id)
	default:
		f.fail(w, http.StatusNotFound, "not_found", "No such route.", nil)
	}
}

func (f *storymakerFixture) notFound(w http.ResponseWriter, err error) {
	if errors.Is(err, fs.ErrNotExist) {
		f.fail(w, http.StatusNotFound, "not_found", "No such project.", nil)
		return
	}
	f.fail(w, http.StatusInternalServerError, "internal", err.Error(), nil)
}

func (f *storymakerFixture) saveStory(w http.ResponseWriter, r *http.Request, id string, raw []byte) {
	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		f.fail(w, http.StatusPreconditionRequired, "precondition_required", "If-Match is required.", nil)
		return
	}
	story, _, err := storymaker.DecodeStory(raw, f.limits)
	if err != nil {
		var decode *storymaker.DecodeError
		var issues []storymaker.Issue
		if errors.As(err, &decode) {
			issues = decode.Issues
		}
		f.fail(w, http.StatusUnprocessableEntity, "invalid_story", err.Error(), issues)
		return
	}
	applied := storymaker.ApplyDefaults(&story)
	issues := storymaker.Validate(&story, f.catalog, f.limits)
	text, err := storymaker.CanonicalJSON(story)
	if err != nil {
		f.fail(w, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	etag, err := f.projects.SaveStory(r.Context(), id, text, ifMatch)
	if errors.Is(err, storymaker.ErrConflict) {
		f.fail(w, http.StatusPreconditionFailed, "precondition_failed", "The story changed.", nil)
		return
	}
	if err != nil {
		f.notFound(w, err)
		return
	}
	status, _ := f.projects.CodeSceneStatus(r.Context(), id)
	issues = append(issues, storymaker.CodeSceneIssues(&story, status)...)
	if issues == nil {
		issues = []storymaker.Issue{}
	}
	f.writeJSON(w, http.StatusOK, map[string]any{"etag": etag, "issues": storymaker.CapIssues(issues, storymaker.MaxHTTPIssues), "defaults_applied": applied, "story": json.RawMessage(text)})
}

func (f *storymakerFixture) codeScene(w http.ResponseWriter, r *http.Request, id, rest string, raw []byte) {
	ctx := r.Context()
	scene, kind, _ := strings.Cut(rest, "/")
	switch {
	case kind == "code" && r.Method == http.MethodGet:
		source, etag, err := f.projects.LoadCodeSource(ctx, id, scene)
		if err != nil {
			f.notFound(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("ETag", etag)
		_, _ = w.Write(source)
	case kind == "bundle" && r.Method == http.MethodGet:
		bundle, err := f.projects.LoadCodeBundle(ctx, id, scene)
		if err != nil {
			f.notFound(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(bundle)
	case kind == "code" && r.Method == http.MethodPut:
		ifMatch := r.Header.Get("If-Match")
		if ifMatch == "" && r.Header.Get("If-None-Match") == "*" {
			ifMatch = storymaker.MustNotExist
		}
		if ifMatch == "" {
			f.fail(w, http.StatusPreconditionRequired, "precondition_required", "If-Match or If-None-Match is required.", nil)
			return
		}
		etag, result, err := f.projects.SaveCodeScene(ctx, id, scene, raw, ifMatch)
		if errors.Is(err, storymaker.ErrConflict) {
			f.fail(w, http.StatusPreconditionFailed, "precondition_failed", "The source changed.", nil)
			return
		}
		if err != nil {
			f.fail(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
			return
		}
		errs := result.Errors
		if errs == nil {
			errs = []storymaker.BundleError{}
		}
		f.writeJSON(w, http.StatusOK, map[string]any{"etag": etag, "ok": len(errs) == 0, "errors": errs})
	default:
		f.fail(w, http.StatusNotFound, "not_found", "No such route.", nil)
	}
}

// narration answers GET /projects/{id}/narration through the real ProjectStore.AudioAnswer.
func (f *storymakerFixture) narration(w http.ResponseWriter, ctx context.Context, id string) {
	data, _, err := f.projects.LoadStory(ctx, id)
	if err != nil {
		f.notFound(w, err)
		return
	}
	story, _, err := storymaker.DecodeStory(data, f.limits)
	if err != nil {
		f.fail(w, http.StatusUnprocessableEntity, "invalid_story", err.Error(), nil)
		return
	}
	storymaker.ApplyDefaults(&story)
	answer, err := f.projects.AudioAnswer(ctx, id, &story, nil)
	if err != nil {
		f.fail(w, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	// This stand-in has no narrate route, so its narration can never get audio. Without this, the export's narration blocker
	// would keep export-start disabled and stop the tour.
	answer.Missing = []storymaker.MissingBlock{}
	answer.Chars.Missing = 0
	w.Header().Set("Cache-Control", "no-store")
	f.writeJSON(w, http.StatusOK, answer)
}

func (f *storymakerFixture) render(w http.ResponseWriter, r *http.Request, id string, raw []byte) {
	if _, err := render.ParseRenderOptions(raw); err != nil {
		f.fail(w, http.StatusBadRequest, "invalid_request", err.Error(), nil)
		return
	}
	f.mu.Lock()
	ffmpeg, browser := f.ffmpegReady, f.browserReady
	f.mu.Unlock()
	if r.Header.Get("If-Match") == "" {
		f.fail(w, http.StatusPreconditionRequired, "precondition_required", "If-Match is required.", nil)
		return
	}
	if !ffmpeg {
		f.fail(w, http.StatusServiceUnavailable, "ffmpeg_unavailable", "FFmpeg is not available.", nil)
		return
	}
	if !browser {
		f.fail(w, http.StatusServiceUnavailable, "browser_unavailable", "No browser.", nil)
		return
	}
	project, err := f.projects.Get(r.Context(), id)
	if err != nil {
		f.notFound(w, err)
		return
	}
	job := &storymakerFixtureJob{ID: uuid.NewString(), ProjectID: id, Status: "queued", Phase: "starting", Slug: storymaker.Slugify(project.Title, id), Created: time.Now()}
	f.mu.Lock()
	f.jobs[job.ID] = job
	f.order = append(f.order, job.ID)
	body := f.jobBody(job)
	f.publish(job)
	f.mu.Unlock()
	f.writeJSON(w, http.StatusAccepted, map[string]any{"job": body})
}

func (f *storymakerFixture) job(w http.ResponseWriter, r *http.Request, route string) {
	trimmed := strings.TrimPrefix(route, "/jobs/")
	id, rest, _ := strings.Cut(trimmed, "/")
	f.mu.Lock()
	defer f.mu.Unlock()
	job := f.jobs[id]
	if job == nil {
		f.fail(w, http.StatusNotFound, "not_found", "No such job.", nil)
		return
	}
	if rest == "cancel" && r.Method == http.MethodPost {
		if job.Status == "queued" || job.Status == "running" {
			job.Status, job.ErrorCode = "cancelled", "cancelled"
			f.publish(job)
		}
	}
	f.writeJSON(w, http.StatusOK, map[string]any{"job": f.jobBody(job)})
}

// jobList answers GET /projects/{id}/jobs like the real handler: the jobs of the project, newest first.
func (f *storymakerFixture) jobList(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	jobs := []map[string]any{}
	for i := len(f.order) - 1; i >= 0; i-- {
		if job := f.jobs[f.order[i]]; job.ProjectID == id {
			jobs = append(jobs, f.jobBody(job))
		}
	}
	f.writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (f *storymakerFixture) events(w http.ResponseWriter, r *http.Request, id string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		f.fail(w, http.StatusInternalServerError, "internal", "no streaming", nil)
		return
	}
	ch := make(chan []byte, 16)
	f.mu.Lock()
	f.watchers[id] = append(f.watchers[id], ch)
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		list := f.watchers[id]
		for i, candidate := range list {
			if candidate == ch {
				f.watchers[id] = append(list[:i], list[i+1:]...)
				break
			}
		}
		f.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, ": open\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case payload := <-ch:
			fmt.Fprintf(w, "event: job\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

// player serves the app player host exactly like the real handler: the markup of contract section 6 under the CSP of the spec.
func (f *storymakerFixture) player(w http.ResponseWriter, r *http.Request) {
	origin := "http://" + r.Host
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", fmt.Sprintf(storymakerPlayerCSP, origin))
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, `<!doctype html><html data-sm-mode="app" data-sm-base="%[1]s"><head><meta charset="utf-8"><link rel="stylesheet" href="%[1]s/css/storymaker-player.css"></head><body><canvas id="sm-stage"></canvas><script src="%[1]s/js/storymaker/host.bundle.js"></script></body></html>`, origin)
}

// storedStory returns the story of a project as the store holds it.
func (f *storymakerFixture) storedStory(id string) (map[string]any, string) {
	data, etag, err := f.projects.LoadStory(context.Background(), id)
	if err != nil {
		f.t.Fatalf("load the stored story: %v", err)
	}
	var story map[string]any
	if err := json.Unmarshal(data, &story); err != nil {
		f.t.Fatalf("decode the stored story: %v", err)
	}
	return story, etag
}

// watcherCount is the number of open event streams of a project.
func (f *storymakerFixture) watcherCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.watchers[id])
}

func (f *storymakerFixture) projectIDs() []string {
	list, _ := f.projects.List(context.Background())
	ids := make([]string, 0, len(list))
	for _, project := range list {
		ids = append(ids, project.ID)
	}
	sort.Strings(ids)
	return ids
}
