package render

import (
	"context"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/storymaker"
)

// compileCheckRunner needs neither FFmpeg nor a publisher: a check that touched them would panic on the nil publisher.
func compileCheckRunner(t *testing.T, page *fakePage) (*Runner, string) {
	t.Helper()
	ui := os.DirFS(uiDirForTests)
	cat, err := storymaker.LoadCatalog(ui, "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	projects := storymaker.NewProjectStore(storymaker.NewMemStore(), ProjectsRoot)
	proj, _, _, err := projects.Create(context.Background(), "Compile check", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	browsers := NewBrowsersWithProbe(dir, func() (string, bool) { return "/fake/chrome", true }, func(context.Context, string) (string, error) { return "Fake/1", nil })
	return NewRunner(Deps{Dir: dir, UI: ui, Catalog: cat, Projects: projects, Browsers: browsers,
		OpenPage: func(context.Context, sessionOptions) (browserPage, error) { return page, nil }}), proj.ID
}

func TestCompileCheckStopsAfterReport(t *testing.T) {
	limits := storymaker.DefaultLimits()
	limits.MaxDurationSeconds = 1 // 30 frames, so the 60 frames below are over the export limit

	t.Run("summary", func(t *testing.T) {
		var sent atomic.Int64
		told := make(chan ReportAnswer, 1)
		page := &fakePage{crashed: make(chan struct{})}
		page.script = play(t, playOptions{frames: 60, framesSent: &sent,
			onRefused: func(a ReportAnswer) { told <- a },
			mutateRep: func(rep map[string]any) {
				rep["scenes"].([]map[string]any)[0]["cues"] = []any{map[string]any{"marker": 1, "at_ms": 700, "placed": "estimate"}}
				rep["issues"] = []any{map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "dead_stretch", "message": "nothing moves"}}
			}})
		runner, id := compileCheckRunner(t, page)
		got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: limits})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case answer := <-told:
			if answer.Error != compileOnlyAnswer {
				t.Errorf("the page was told %+v", answer)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("the page never got an answer to its report")
		}
		if sent.Load() != 0 || !page.closed.Load() {
			t.Errorf("frames sent %d, page closed %v", sent.Load(), page.closed.Load())
		}
		wantScenes := []storymaker.CompileScene{{ID: "s1", StartMS: 0, EndMS: 2000, CuesMS: []storymaker.CompileCue{{Marker: 1, AtMS: 700}}}}
		if !got.Available || got.DurationMS != 2000 || !reflect.DeepEqual(got.Scenes, wantScenes) {
			t.Errorf("summary = %+v", got)
		}
		if len(got.Issues) != 2 || got.Issues[0].Code != storymaker.CodeDeadStretch || got.Issues[1].Code != storymaker.CodeOverTarget {
			t.Errorf("issues = %+v", got.Issues)
		}
		entries, err := os.ReadDir(StagingRoot(runner.deps.Dir))
		if err != nil || len(entries) != 0 {
			t.Errorf("staging left %v, error %v", entries, err)
		}
	})

	t.Run("error issue keeps its scene", func(t *testing.T) {
		page := &fakePage{crashed: make(chan struct{})}
		page.script = play(t, playOptions{frames: 24, mutateRep: func(rep map[string]any) {
			rep["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s1", "path": "/scenes/0", "code": "unknown_asset", "message": "no such asset"}}
		}})
		runner, id := compileCheckRunner(t, page)
		got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits()})
		if err != nil {
			t.Fatal(err)
		}
		if !got.Available || len(got.Issues) != 1 {
			t.Fatalf("summary = %+v", got)
		}
		is := got.Issues[0]
		if is.Code != storymaker.CodeCompileFailed || is.SceneID != "s1" || !strings.Contains(is.Message, "no such asset") || !storymaker.IssueModelFixable(is) {
			t.Errorf("issue = %+v", is)
		}
	})
}

func TestCompileCheckBrowser(t *testing.T) {
	chrome := browserSmoke(t)
	fx := newRealFixture(t, chrome)
	fx.rebuild(t)
	ctx := context.Background()
	doc := `{"schema":"storymaker/1","title":"Compile check","scenes":[
	 {"id":"s1","environment":"env.office","narration":"A firewall {1} checks every packet before it reaches your network.",
	  "beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1}]},
	 {"id":"s2","narration":"After that everything runs safely."}]}`
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(doc), ""); err != nil {
		t.Fatal(err)
	}
	got, err := fx.runner.CompileCheck(ctx, fx.projectID, fx.settings)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.DurationMS <= 0 || len(got.Scenes) != 2 {
		t.Fatalf("summary = %+v", got)
	}
	if storymaker.HasErrors(got.Issues) {
		t.Fatalf("issues = %+v", got.Issues)
	}
	first := got.Scenes[0]
	if first.ID != "s1" || first.EndMS <= first.StartMS || len(first.CuesMS) != 1 || first.CuesMS[0].Marker != 1 || first.CuesMS[0].AtMS <= 0 {
		t.Errorf("first scene = %+v", first)
	}
	if got.Scenes[1].EndMS != got.DurationMS {
		t.Errorf("the last scene ends at %d, the film at %d", got.Scenes[1].EndMS, got.DurationMS)
	}
	// The page measured the layout of both storyboard scenes, in fractions of the frame.
	if got.LayoutError != "" || got.StoryboardScenes != 2 || got.CodeScenes != 0 || len(got.Layout) != 2 {
		t.Fatalf("layout: %+v (error %q)", got.Layout, got.LayoutError)
	}
	firstLayout := got.Layout[0]
	if firstLayout.ID != "s1" || len(firstLayout.Props) != 1 || !firstLayout.Props[0].Hero || firstLayout.Props[0].Box[3] <= 0 || firstLayout.Props[0].Box[3] > 1 {
		t.Errorf("layout of s1 = %+v", firstLayout)
	}
	if firstLayout.Caption == nil || firstLayout.Caption[1] < 0.5 {
		t.Errorf("the caption of s1 must be near the bottom: %+v", firstLayout.Caption)
	}
	if len(fx.pub.names()) != 0 {
		t.Errorf("a compile check published %v", fx.pub.names())
	}
}

func TestCompileCheckKeepsTheProbeFindings(t *testing.T) {
	page := &fakePage{crashed: make(chan struct{})}
	page.script = play(t, playOptions{frames: 60, mutateRep: func(rep map[string]any) {
		rep["issues"] = []any{map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "dead_stretch", "message": "nothing moves"}}
		rep["checks"] = []any{
			map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "flash", "message": "flashes", "at_seconds": 0.3, "fix": "slow it down"},
			map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "flash_unverified", "message": "not checked"},
			// the page is trusted for these four codes only, and only as warnings
			map[string]any{"severity": "error", "scene_id": "s1", "path": "/scenes/0", "code": "flash", "message": "an error"},
			map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "compile_failed", "message": "not a probe code"},
			map[string]any{"severity": "warning", "scene_id": "elsewhere", "path": "/scenes/7", "code": "first_frame_empty", "message": "unknown scene"},
		}
	}})
	runner, id := compileCheckRunner(t, page)
	got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, is := range got.Issues {
		codes = append(codes, is.Code)
		if is.Code == storymaker.CodeFlash && is.Severity != storymaker.SeverityWarning {
			t.Errorf("a probe finding is a warning: %+v", is)
		}
		if is.Message == "flashes" && (is.Severity != storymaker.SeverityWarning || is.SceneID != "s1" || is.Fix != "slow it down" || is.AtSeconds == nil) {
			t.Errorf("flash = %+v", is)
		}
		if is.Code == storymaker.CodeFirstFrameEmpty && is.SceneID != "" {
			t.Errorf("a scene the story does not have must be dropped: %+v", is)
		}
	}
	want := []string{storymaker.CodeDeadStretch, storymaker.CodeFlash, storymaker.CodeFlashUnverified, storymaker.CodeFlash, storymaker.CodeFirstFrameEmpty}
	if !reflect.DeepEqual(codes, want) {
		t.Errorf("issue codes = %v, want %v", codes, want)
	}
	if storymaker.HasErrors(got.Issues) {
		t.Errorf("a probe finding must never be an error: %+v", got.Issues)
	}
}

// TestCompileCheckFlashBrowser runs the frame probe in the real page: a code scene that alternates a black and a white frame
// every two frames raises flash for its scene, and a calm story raises none.
func TestCompileCheckFlashBrowser(t *testing.T) {
	chrome := browserSmoke(t)
	fx := newRealFixture(t, chrome)
	fx.rebuild(t)
	ctx := context.Background()
	check := func(doc string) []storymaker.Issue {
		t.Helper()
		if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(doc), ""); err != nil {
			t.Fatal(err)
		}
		got, err := fx.runner.CompileCheck(ctx, fx.projectID, fx.settings)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Available || storymaker.HasErrors(got.Issues) {
			t.Fatalf("summary = %+v", got)
		}
		return got.Issues
	}
	calm := `{"schema":"storymaker/1","title":"Calm","scenes":[
	 {"id":"s1","environment":"env.office","beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"}]}]}`
	for _, is := range check(calm) {
		if is.Code == storymaker.CodeFlash || is.Code == storymaker.CodeFlashUnverified {
			t.Errorf("a calm story raised %+v", is)
		}
	}
	strobing := `{"schema":"storymaker/1","title":"Strobe","scenes":[
	 {"id":"s1","environment":"env.office","beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"}]},
	 {"id":"c1","kind":"code"}]}`
	// 15 flashes per second: the picture flips between black and white every second frame
	src := "export default function scene(api) {\n  api.duration(3);\n  api.onFrame(function (t, g) {\n    g.fill(Math.floor(t * 15) % 2 ? '#ffffff' : '#000000');\n    g.rect(0, 0, 1920, 1080);\n  });\n}\n"
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "c1", []byte(src), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
	var flashes []storymaker.Issue
	for _, is := range check(strobing) {
		if is.Code == storymaker.CodeFlash {
			flashes = append(flashes, is)
		}
		if is.Code == storymaker.CodeFlashUnverified {
			t.Errorf("the probe must reach every frame of a 4 s film: %+v", is)
		}
	}
	if len(flashes) != 1 {
		t.Fatalf("flash issues = %+v", flashes)
	}
	f := flashes[0]
	if f.SceneID != "c1" || f.Path != "/scenes/1" || f.Severity != storymaker.SeverityWarning || f.AtSeconds == nil || !storymaker.IssueModelFixable(f) {
		t.Errorf("flash = %+v", f)
	}
}
