package server

import (
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"aurago/internal/chibisensei"
)

func chibiLoadRun(w http.ResponseWriter, r *http.Request, acc *chibiSenseiAccess, runID string) (chibisensei.Run, bool) {
	run, err := acc.rt.store.GetRun(r.Context(), runID)
	if errors.Is(err, chibisensei.ErrNotFound) {
		chibiRunNotFound(w)
		return chibisensei.Run{}, false
	}
	if err != nil {
		chibiDomainError(w, err)
		return chibisensei.Run{}, false
	}
	return run, true
}

// chibiRunRead is the shared start of the read routes: GET only, read authorization, the run loaded.
func chibiRunRead(s *Server, w http.ResponseWriter, r *http.Request, runID string) (*chibiSenseiAccess, chibisensei.Run, bool) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return nil, chibisensei.Run{}, false
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return nil, chibisensei.Run{}, false
	}
	run, ok := chibiLoadRun(w, r, acc, runID)
	if !ok {
		return nil, chibisensei.Run{}, false
	}
	return acc, run, true
}

// handleChibiRunList answers GET /projects/{id}/runs?limit=N with the run summaries, newest first.
func handleChibiRunList(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	limit, ok := chibiQueryInt(w, r, "limit", 20)
	if !ok {
		return
	}
	if _, err := acc.rt.store.GetProject(r.Context(), id); err != nil {
		chibiDomainError(w, err)
		return
	}
	runs, err := acc.rt.store.ListRuns(r.Context(), id, limit)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	out := []chibisensei.RunSummary{}
	for _, run := range runs {
		out = append(out, run.Summary())
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"runs": out})
}

// handleChibiRun answers GET /runs/{id} with the full run view.
func handleChibiRun(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	_, run, ok := chibiRunRead(s, w, r, runID)
	if !ok {
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"run": run})
}

// handleChibiRunMetrics answers GET /runs/{id}/metrics?since_step=N (inclusive, A25).
func handleChibiRunMetrics(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	since, ok := chibiQueryInt(w, r, "since_step", 0)
	if !ok {
		return
	}
	run, ok := chibiLoadRun(w, r, acc, runID)
	if !ok {
		return
	}
	points, err := acc.rt.store.ListMetrics(r.Context(), runID, max(since, 0))
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	maxSteps := 0
	if st := run.Remote.Stage; st != nil {
		maxSteps = st.MaxSteps
	}
	if maxSteps == 0 && run.Remote.Result != nil {
		maxSteps = run.Remote.Result.Steps
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"metrics": points, "max_steps": maxSteps, "live": run.Remote.Live})
}

// handleChibiRunEvents answers GET /runs/{id}/events?after=N&limit=N.
func handleChibiRunEvents(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	after, ok := chibiQueryInt(w, r, "after", 0)
	if !ok {
		return
	}
	limit, ok := chibiQueryInt(w, r, "limit", 200)
	if !ok {
		return
	}
	if _, ok := chibiLoadRun(w, r, acc, runID); !ok {
		return
	}
	events, err := acc.rt.store.ListRunEvents(r.Context(), runID, int64(after), limit)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	if events == nil {
		events = []chibisensei.RunEvent{}
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"events": events})
}

// handleChibiRunArtifacts answers GET /runs/{id}/artifacts. The JSON hides run, project and path (A29).
func handleChibiRunArtifacts(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	acc, _, ok := chibiRunRead(s, w, r, runID)
	if !ok {
		return
	}
	list, err := acc.rt.store.ListArtifacts(r.Context(), runID)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	if list == nil {
		list = []chibisensei.Artifact{}
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"artifacts": list})
}

// handleChibiRunReport answers GET /runs/{id}/report: the automatic report or null.
func handleChibiRunReport(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	acc, _, ok := chibiRunRead(s, w, r, runID)
	if !ok {
		return
	}
	body, found, err := acc.rt.store.GetReport(r.Context(), runID, "auto")
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	if !found {
		writeChibiJSON(w, http.StatusOK, map[string]any{"report": nil})
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"report": body})
}

// handleChibiRunLog answers GET /runs/{id}/log with the scrubbed log tail (at most 64 KiB).
func handleChibiRunLog(s *Server, w http.ResponseWriter, r *http.Request, runID string) {
	acc, _, ok := chibiRunRead(s, w, r, runID)
	if !ok {
		return
	}
	b, err := chibisensei.ReadLogTail(acc.rt.dataDir, runID)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"tail": string(b), "bytes": len(b)})
}

const (
	chibiArtifactMinRate    = 256 << 10 // bytes per second a download is given time for
	chibiArtifactBaseWindow = 10 * time.Minute
	chibiArtifactMaxWindow  = 4 * time.Hour
)

// chibiArtifactWriteWindow is the write deadline of one artifact download: 10 min plus the size at 256 KiB/s, at most
// 4 h. It replaces the listener's absolute write timeout (40 min) for this response only, as the Video Studio upload
// widens its window (video_studio_service.go); httpstream renews deadlines only for SSE and media.
func chibiArtifactWriteWindow(size int64) time.Duration {
	window := chibiArtifactBaseWindow + time.Duration(size/chibiArtifactMinRate)*time.Second
	if window > chibiArtifactMaxWindow {
		return chibiArtifactMaxWindow
	}
	return window
}

// handleChibiArtifactFile serves GET /runs/{id}/artifacts/{aid}/file as an attachment (Range, HEAD) through os.Root.
func handleChibiArtifactFile(s *Server, w http.ResponseWriter, r *http.Request, runID, artifactID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeChibiMethodError(w, "GET, HEAD")
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	a, err := acc.rt.store.GetArtifact(r.Context(), artifactID)
	if errors.Is(err, chibisensei.ErrNotFound) || (err == nil && a.RunID != runID) {
		chibiArtifactNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	f, err := chibisensei.OpenArtifact(acc.rt.dataDir, a)
	if errors.Is(err, fs.ErrNotExist) {
		chibiArtifactNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		chibiArtifactNotFound(w)
		return
	}
	name := "chibi-" + chibisensei.Run12(runID) + "-" + a.Name
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(chibiArtifactWriteWindow(info.Size()))); err != nil &&
		!errors.Is(err, http.ErrNotSupported) {
		slog.Warn("[ChibiSensei] The download deadline could not be extended", "error", err)
	}
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, name, info.ModTime(), f)
}
