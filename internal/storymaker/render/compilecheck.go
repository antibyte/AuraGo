package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"time"

	"aurago/internal/security"
	"aurago/internal/storymaker"
)

// compileCheckLimit bounds one compile check, browser probe included. A variable, so a test can shorten it.
var compileCheckLimit = 60 * time.Second

const (
	// compileOnlyAnswer is the error of the report answer. host-render.js ends on any answer without start.
	compileOnlyAnswer = "compile_only"
	// Reasons a check could not run. None of them says anything about the story.
	compileReasonTimeout = "compile_timeout"
	compileReasonCrashed = "browser_crashed"
	compileReasonSetup   = "compile_setup"

	compileProxyConnections = 400
	compileProxyBytes       = int64(256) << 20
	compileShutdownWait     = 2 * time.Second
	compileMaxFrames        = math.MaxInt32
	compileSceneFix         = "Rewrite this scene with patch_scene or put_scene, then check again."
)

// compileCheckReport is the part of the report a compile check keeps. parseReport validates the same body.
type compileCheckReport struct {
	FrameCount int                 `json:"frame_count"`
	Scenes     []compileCheckScene `json:"scenes"`
	Issues     []storymaker.Issue  `json:"issues"`
}

type compileCheckScene struct {
	ID         string            `json:"id"`
	StartFrame int               `json:"start_frame"`
	EndFrame   int               `json:"end_frame"`
	Cues       []compileCheckCue `json:"cues"`
}

type compileCheckCue struct {
	Marker int     `json:"marker"`
	AtMS   float64 `json:"at_ms"`
}

// CompileCheck compiles the saved story in the real page and stops after the report: no FFmpeg, no frames, no download.
// A check that cannot run answers Available false with a Reason. A cancelled ctx returns its error. A page or compile
// failure is an issue with code compile_failed. Cue times are scene-local, as the report gives them.
func (r *Runner) CompileCheck(ctx context.Context, projectID string, st Settings) (storymaker.CompileSummary, error) {
	checkCtx, cancel := context.WithTimeout(ctx, compileCheckLimit)
	defer cancel()
	if r.deps.Browsers == nil {
		return compileUnavailable(ctx, CodeBrowserUnavailable)
	}
	status, bin := r.deps.Browsers.resolve(checkCtx, st.BrowserPath, false, nil)
	if !status.Ready {
		return compileUnavailable(ctx, CodeBrowserUnavailable)
	}
	plan, err := r.prepareCore(checkCtx, projectID, "", st, RenderOptions{}, nil)
	if err != nil {
		var refused *PrepareError
		if errors.As(err, &refused) {
			return compileUnavailable(ctx, refused.Code)
		}
		return storymaker.CompileSummary{}, err
	}
	return r.compileInBrowser(ctx, checkCtx, plan, bin, st)
}

// compileUnavailable is the answer of a check that could not run. A cancelled job is no such answer.
func compileUnavailable(ctx context.Context, reason string) (storymaker.CompileSummary, error) {
	if err := ctx.Err(); err != nil {
		return storymaker.CompileSummary{}, err
	}
	return storymaker.CompileSummary{Reason: reason}, nil
}

func (r *Runner) compileInBrowser(ctx, checkCtx context.Context, plan *Plan, bin string, st Settings) (storymaker.CompileSummary, error) {
	setupFailed := func(err error) (storymaker.CompileSummary, error) {
		r.deps.Logger.Warn("[Storymaker] The compile check could not start", "error", err)
		return compileUnavailable(ctx, compileReasonSetup)
	}
	root := StagingRoot(r.deps.Dir)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return setupFailed(err)
	}
	staging, err := os.MkdirTemp(root, "compile-")
	if err != nil {
		return setupFailed(err)
	}
	defer os.RemoveAll(staging)
	snap, err := (&renderRun{r: r, plan: plan}).snapshot()
	if err != nil {
		return setupFailed(err)
	}
	prefix, err := NewPrefix()
	if err != nil {
		return setupFailed(err)
	}
	token, err := NewPrefix()
	if err != nil {
		return setupFailed(err)
	}
	width, height := CanvasSize(plan.Story.Format.Aspect)
	fps := plan.Story.Format.FPS
	if fps <= 0 {
		fps = storymaker.DefaultFPS
	}
	frameLimit := int(st.Limits.MaxDurationSeconds) * fps
	if frameLimit <= 0 {
		frameLimit = st.MaxDurationSeconds * fps
	}
	reports := make(chan []byte, 1)
	pageErrors := make(chan PageError, 1)
	// No workers: helper workers would only start up for frames that never come.
	listener, err := StartListener(prefix, snap, PageParams{Width: width, Height: height, FPS: fps, Transport: DefaultTransport, FrameLimit: frameLimit, FrameDeadlineMS: frameDeadlineMS, Token: token, AudioInfo: plan.AudioJSON},
		Hooks{
			Report: func(body []byte) ReportAnswer {
				select {
				case reports <- body:
				default:
				}
				return ReportAnswer{Error: compileOnlyAnswer, Message: "This page only compiles the story."}
			},
			Error: func(e PageError) {
				select {
				case pageErrors <- e:
				default:
				}
			},
		})
	if err != nil {
		return setupFailed(err)
	}
	defer listener.Close()
	origin, err := url.Parse(listener.Origin())
	if err != nil {
		return setupFailed(err)
	}
	proxy, err := security.StartStrictCaptureProxy(origin, security.CaptureProxyLimits{
		MaxConcurrent:         16,
		MaxConnections:        compileProxyConnections,
		MaxBytes:              compileProxyBytes,
		ResponseHeaderTimeout: proxyHeaderTimeout,
	})
	if err != nil {
		return setupFailed(err)
	}
	defer proxy.Close()
	opts := sessionOptions{Bin: bin, Profile: staging + string(os.PathSeparator) + "profile", ProxyAddr: proxy.Addr(), Base: listener.Base(), Width: width, Height: height}
	if r.deps.SessionTweak != nil {
		r.deps.SessionTweak(&opts)
	}
	page, err := r.deps.OpenPage(checkCtx, opts)
	if err != nil {
		r.deps.Logger.Warn("[Storymaker] The compile check could not open a browser", "issue", launchIssue(err))
		return compileUnavailable(ctx, CodeBrowserUnavailable)
	}
	defer page.Close()
	if err := page.Navigate(checkCtx, listener.Base()+"/"); err != nil {
		return setupFailed(err)
	}

	ids := make([]string, len(plan.Story.Scenes))
	for i := range plan.Story.Scenes {
		ids[i] = plan.Story.Scenes[i].ID
	}
	blocks, captionText, maxEvents := planReportBlocks(plan)
	want := reportExpect{FPS: fps, Width: width, Height: height, MaxFrames: compileMaxFrames, CatalogVersion: r.deps.Catalog.Version(),
		Catalog: r.deps.Catalog, SFXEvents: sfxEventBound(&plan.Story, r.deps.Catalog), SceneIDs: ids,
		Blocks: blocks, CaptionText: captionText, MaxEvents: maxEvents, Prefix: prefix, Token: token}
	select {
	case <-checkCtx.Done():
		return compileUnavailable(ctx, compileReasonTimeout)
	case <-page.Crashed():
		return compileUnavailable(ctx, compileReasonCrashed)
	case e := <-pageErrors:
		sceneID := ""
		if knownCompileScene(want, e.SceneID) {
			sceneID = e.SceneID
		}
		return compileFailure(sceneID, e.Code+": "+e.Message), nil
	case body := <-reports:
		// Shutdown lets the handler finish, so the page reads its answer. Close would cut it.
		shutdownCtx, stop := context.WithTimeout(context.Background(), compileShutdownWait)
		_ = listener.srv.Shutdown(shutdownCtx)
		stop()
		return summarizeCompileReport(body, want, frameLimit), nil
	}
}

// summarizeCompileReport turns a report into the summary. An error issue keeps its scene, so the model can fix it. The
// frame limit is no refusal here: a film over it is reported as over_target.
func summarizeCompileReport(body []byte, want reportExpect, frameLimit int) storymaker.CompileSummary {
	var rep compileCheckReport
	if err := json.Unmarshal(body, &rep); err != nil {
		return compileFailure("", "The compile report is not valid JSON.")
	}
	issues := cleanCompileIssues(rep.Issues, want)
	if storymaker.HasErrors(issues) {
		return storymaker.CompileSummary{Available: true, Issues: issues}
	}
	if _, refusal := parseReport(body, want); refusal != nil {
		return compileFailure("", refusal.Message)
	}
	if frameLimit > 0 && rep.FrameCount > frameLimit {
		issues = append(issues, storymaker.Issue{
			Severity: storymaker.SeverityWarning, Path: "/scenes", Code: storymaker.CodeOverTarget,
			Message: fmt.Sprintf("The film runs %d seconds, and an export allows at most %d seconds.", rep.FrameCount/want.FPS, frameLimit/want.FPS),
			Fix:     "Shorten the narration or remove a scene.",
		})
	}
	scenes := make([]storymaker.CompileScene, 0, len(rep.Scenes))
	for _, sc := range rep.Scenes {
		scene := storymaker.CompileScene{ID: sc.ID, StartMS: compileFrameMS(sc.StartFrame, want.FPS), EndMS: compileFrameMS(sc.EndFrame, want.FPS)}
		for _, cue := range sc.Cues {
			scene.CuesMS = append(scene.CuesMS, storymaker.CompileCue{Marker: cue.Marker, AtMS: int64(math.Round(cue.AtMS))})
		}
		scenes = append(scenes, scene)
	}
	return storymaker.CompileSummary{Available: true, DurationMS: compileFrameMS(rep.FrameCount, want.FPS), Scenes: scenes, Issues: issues}
}

// cleanCompileIssues removes the render secrets from the page's text and bounds it. An error issue becomes compile_failed
// and keeps its own code in the message.
func cleanCompileIssues(list []storymaker.Issue, want reportExpect) []storymaker.Issue {
	out := make([]storymaker.Issue, 0, len(list))
	for _, is := range list {
		is.SceneID = want.clean(is.SceneID, 60)
		if !knownCompileScene(want, is.SceneID) {
			is.SceneID = ""
		}
		is.Path = want.clean(is.Path, 200)
		is.Message = want.clean(is.Message, 400)
		is.Fix = want.clean(is.Fix, 400)
		is.Code = want.clean(is.Code, 60)
		switch is.Severity {
		case storymaker.SeverityError:
			is.Message = is.Code + ": " + is.Message
			is.Code = storymaker.CodeCompileFailed
			if is.Fix == "" && is.SceneID != "" {
				is.Fix = compileSceneFix
			}
		case storymaker.SeverityWarning, storymaker.SeverityMinor:
		default:
			is.Severity = storymaker.SeverityMinor
		}
		out = append(out, is)
	}
	return storymaker.CapIssues(out, storymaker.MaxHTTPIssues)
}

func compileFailure(sceneID, message string) storymaker.CompileSummary {
	issue := storymaker.Issue{Severity: storymaker.SeverityError, SceneID: sceneID, Code: storymaker.CodeCompileFailed, Message: message}
	if sceneID != "" {
		issue.Fix = compileSceneFix
	}
	return storymaker.CompileSummary{Available: true, Issues: []storymaker.Issue{issue}}
}

func knownCompileScene(want reportExpect, id string) bool {
	for _, known := range want.SceneIDs {
		if known == id {
			return true
		}
	}
	return false
}

func compileFrameMS(frames, fps int) int64 {
	return int64(math.Round(float64(frames) * 1000 / float64(fps)))
}
