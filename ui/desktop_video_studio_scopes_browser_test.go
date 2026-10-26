package ui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/videostudio/goldenframe"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

// TestDesktopVideoStudioScopesBrowser checks the 2f scopes path in a real browser (index §4.26). Subtests:
//   - readScaled (Task 1): both painters' readScaled on the engine page under the SwiftShader launcher;
//   - panel (Task 3): the scopes panel in the app.
func TestDesktopVideoStudioScopesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	t.Run("readScaled", videoStudioReadScaledPart)
	t.Run("panel", videoStudioScopesPanelPart)
}

// videoStudioReadScaledPart (Task 1) draws a 1280x720 frame with each painter: the moving gradient under the colour
// chart and the detail gratings, each 1:1 in a bottom quarter. It compares readScaled with a point sample of capture(),
// using the formula of index §4.26.1:
//   - GL exactly. 256x144 has no near-integer sample point, which is asserted. 7x5 has some, and there the pixel below
//     is allowed too.
//   - 2D within the 2x2 neighbourhood around the sample point (the browser's nearest-neighbour rule).
//
// It also checks:
//   - readScaled returns a Promise, and null for sizes outside 1..1024;
//   - an onPaint counter: readScaled never paints, and capture() paints once;
//   - on GL: null for a second read while one is pending, and for a read whose context is lost before it ends.
func videoStudioReadScaledPart(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("the readScaled check needs ffmpeg on the PATH for its media")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	media, err := goldenframe.MakeMedia(ctx, ffmpeg, t.TempDir())
	if err != nil {
		t.Fatalf("MakeMedia: %v", err)
	}
	chart := goldenframe.NewClip("c", goldenframe.MediaChart, 0, 75)
	chart.X, chart.Y, chart.Width, chart.Height = 0, 0.5, 0.5, 0.5
	detail := goldenframe.NewClip("d", goldenframe.MediaDetail, 0, 75)
	detail.X, detail.Y, detail.Width, detail.Height = 0.5, 0.5, 0.5, 0.5
	project := media.Project("Scopes readback",
		goldenframe.VideoTrack("v1", goldenframe.NewClip("g", goldenframe.MediaGradient, 0, 75)),
		goldenframe.OverlayTrack("o1", chart), goldenframe.OverlayTrack("o2", detail))
	page := goldenEnginePage(t, newVideoStudioEngineServer(t, media.Dir), media.Files)
	requireWebGL2(t, page)

	type scaled struct {
		Width  int    `json:"width"`
		Height int    `json:"height"`
		RGBA   string `json:"rgba"`
	}
	type run struct {
		Painter       string  `json:"painter"`
		Settled       bool    `json:"settled"`
		IsPromise     bool    `json:"isPromise"`
		Shot          scaled  `json:"shot"`
		Small         *scaled `json:"small"`
		Tiny          *scaled `json:"tiny"`
		Invalid       []bool  `json:"invalid"`
		ReadPaints    int     `json:"readPaints"`
		CapturePaints int     `json:"capturePaints"`
		FirstOfTwo    bool    `json:"firstOfTwo"`
		SecondOfTwo   bool    `json:"secondOfTwo"`
		AfterLoss     bool    `json:"afterLoss"`
	}
	// sources lists the frame pixels that output pixel i of size may show, along an axis of n frame pixels (index
	// §4.26.1): s = min(n − 1, floor((i + 0.5)·n/size)).
	//   - GL shows s. Where (i + 0.5)·n/size lies within 1e-4 of an integer r, it may show r − 1 or r.
	//   - The 2D painter may show either pixel of the 2x2 neighbourhood around the sample point: s and s + 1 when the
	//     point lies right of s's centre, else s − 1 and s.
	sources := func(i, size, n int, footprint bool) []int {
		v := (float64(i) + 0.5) * float64(n) / float64(size)
		s := min(n-1, int(math.Floor(v)))
		candidates := []int{s}
		switch r := math.Round(v); {
		case footprint && v-float64(s) >= 0.5:
			candidates = append(candidates, s+1)
		case footprint:
			candidates = append(candidates, s-1)
		case math.Abs(v-r) < 1e-4:
			candidates = []int{int(r) - 1, int(r)}
		}
		kept := candidates[:0]
		for _, c := range candidates {
			if c >= 0 && c < n {
				kept = append(kept, c)
			}
		}
		return kept
	}
	decode := func(t *testing.T, label string, img *scaled) []byte {
		t.Helper()
		if img == nil {
			t.Fatalf("%s: readScaled resolved null", label)
		}
		raw, err := base64.StdEncoding.DecodeString(img.RGBA)
		if err != nil || len(raw) != img.Width*img.Height*4 {
			t.Fatalf("%s: %d bytes for %dx%d (%v)", label, len(raw), img.Width, img.Height, err)
		}
		return raw
	}
	// compare checks every pixel of got (width x height, alpha 255) against its allowed frame pixels and returns how many
	// output pixels had more than one allowed source.
	compare := func(t *testing.T, label string, frame []byte, fw, fh int, got []byte, width, height int, footprint bool) int {
		t.Helper()
		bad, several := 0, 0
		for j := 0; j < height; j++ {
			ys := sources(j, height, fh, footprint)
			for i := 0; i < width; i++ {
				xs := sources(i, width, fw, footprint)
				if len(xs)*len(ys) > 1 {
					several++
				}
				o := (j*width + i) * 4
				match := false
				for _, y := range ys {
					for _, x := range xs {
						p := (y*fw + x) * 4
						match = match || bytes.Equal(got[o:o+4], frame[p:p+4])
					}
				}
				if got[o+3] != 255 || !match {
					if bad++; bad <= 5 {
						t.Errorf("%s: pixel (%d, %d) = %v; allowed frame pixels x %v, y %v (alpha must be 255)", label, i, j, got[o:o+4], xs, ys)
					}
				}
			}
		}
		if bad > 0 {
			t.Errorf("%s: %d of %d pixels are not the point sample", label, bad, width*height)
		}
		t.Logf("%s: %d pixels checked, %d with more than one allowed source", label, width*height, several)
		return several
	}
	const js = `async (project, painter) => {
  const stage = document.getElementById('stage'), canvas = document.createElement('canvas');
  canvas.setAttribute('data-preview', '');
  canvas.width = project.width; canvas.height = project.height;
  stage.replaceChildren(canvas);
  let shown = 12, paints = 0;
  const preview = VideoStudioPreview.mount(canvas, () => project, () => shown, value => { shown = value; }, () => {}, () => 1,
    { painter, mediaURL: vsGolden.mediaURL, onPaint: () => { paints++; } });
  const b64 = bytes => { let text = ''; for (let i = 0; i < bytes.length; i += 0x8000) text += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000)); return btoa(text); };
  const pack = value => (value ? { width: value.width, height: value.height, rgba: b64(value.data) } : null);
  try {
    preview.seek(shown);
    const out = { painter: preview.painterKind(), settled: await preview.settle(15000) };
    out.shot = pack(preview.capture());
    const before = paints;
    const first = preview.readScaled(256, 144);
    out.isPromise = first instanceof Promise;
    out.small = pack(await first);
    out.tiny = pack(await preview.readScaled(7, 5));
    out.invalid = [];
    for (const [w, h] of [[0, 144], [256, 2000], [1025, 1], [2.5, 3]]) out.invalid.push((await preview.readScaled(w, h)) === null);
    out.readPaints = paints - before;
    preview.capture();
    out.capturePaints = paints - before;
    if (out.painter === 'gl') {
      const [a, b] = await Promise.all([preview.readScaled(256, 144), preview.readScaled(256, 144)]);
      out.firstOfTwo = !!a; out.secondOfTwo = b === null;
      const lose = canvas.getContext('webgl2').getExtension('WEBGL_lose_context');
      const pending = preview.readScaled(256, 144);
      lose.loseContext();
      out.afterLoss = (await pending) === null;
    }
    return JSON.stringify(out);
  } finally {
    preview.dispose();
    stage.replaceChildren();
  }
}`
	for _, painter := range []string{"gl", "2d"} {
		t.Run(painter, func(t *testing.T) {
			result, err := page.Timeout(2*time.Minute).Eval(js, project, painter)
			if err != nil {
				t.Fatalf("readScaled run: %v", err)
			}
			var out run
			if err := json.Unmarshal([]byte(result.Value.Str()), &out); err != nil {
				t.Fatalf("readScaled run result: %v", err)
			}
			if out.Painter != painter || !out.Settled {
				t.Fatalf("asked for the %s painter: drew with %q, settled %v", painter, out.Painter, out.Settled)
			}
			if !out.IsPromise {
				t.Error("readScaled does not return a Promise")
			}
			frame := decode(t, "capture", &out.Shot)
			if out.Shot.Width != 1280 || out.Shot.Height != 720 {
				t.Fatalf("capture is %dx%d, want 1280x720", out.Shot.Width, out.Shot.Height)
			}
			footprint := painter == "2d"
			if out.Small == nil || out.Small.Width != 256 || out.Small.Height != 144 {
				t.Fatalf("readScaled(256, 144) = %+v", out.Small)
			}
			if several := compare(t, "256x144", frame, 1280, 720, decode(t, "256x144", out.Small), 256, 144, footprint); !footprint && several != 0 {
				t.Errorf("256x144 from 1280x720 met %d near-integer sample points, want 0", several)
			}
			if out.Tiny == nil || out.Tiny.Width != 7 || out.Tiny.Height != 5 {
				t.Fatalf("readScaled(7, 5) = %+v", out.Tiny)
			}
			compare(t, "7x5", frame, 1280, 720, decode(t, "7x5", out.Tiny), 7, 5, footprint)
			for i, isNull := range out.Invalid {
				if !isNull {
					t.Errorf("invalid size %d did not resolve null", i)
				}
			}
			if len(out.Invalid) != 4 {
				t.Errorf("checked %d invalid sizes, want 4", len(out.Invalid))
			}
			if out.ReadPaints != 0 || out.CapturePaints != 1 {
				t.Errorf("onPaint counted %d paints across the reads and %d with capture(), want 0 and 1", out.ReadPaints, out.CapturePaints)
			}
			if painter == "gl" && !(out.FirstOfTwo && out.SecondOfTwo && out.AfterLoss) {
				t.Errorf("GL: first of two %v, second of two null %v, null after a context loss %v; want all true", out.FirstOfTwo, out.SecondOfTwo, out.AfterLoss)
			}
			if errs := page.MustEval(`() => JSON.stringify(window.vsEngineErrors)`).Str(); errs != "[]" {
				t.Fatalf("engine page errors: %s", errs)
			}
		})
	}
}

// videoStudioScopesPanelPart (Task 3) drives the scopes panel in the app on the keyframe fixture: c1, the 512x288
// picture at half size in the top left. It runs under the SwiftShader launcher, then once more after a reload with the
// 2D painter forced by the hidden previewEngine preference. It checks:
//   - the toggle sits in the transport before "Render range exactly", and a closed panel reads nothing while playing;
//   - Enter on the focused toggle opens the panel and keeps focus; every scope shows data; the panel reads 256x144;
//   - playing with the panel open reads 1..13 times in 1 s;
//   - waveform off hides its tile, the last shown kind stays pressed, and the preference follows;
//   - Escape in the bar closes the panel and focuses the toggle;
//   - source mode hides the dock;
//   - at 1280x900, 900x900 and 390x844: the centre column does not scroll sideways, the toggle stays in the transport
//     row, and the panel is 132 px high (96 px on the phone);
//   - with the panel open at those sizes the centre column fits its grid row (scrollHeight - clientHeight at most 1 px),
//     the transport ends inside the workspace and the timeline panel stays whole below it (review I-2); on the phone the
//     open panel collapses the library row (the stage keeps 150 px or more), source mode and closing bring it back;
//   - the screenshots for review (wide, narrow, phone open and closed, 2D), with no script errors.
func videoStudioScopesPanelPart(t *testing.T) {
	page := openVideoStudioKeyframes(t, newWebGLSmokeBrowser(t), newVideoStudioKeyframesServer(t, videoStudioKeyframesProject()))
	shots := videoStudioKeyframesShotDir(t)
	const app = `window.VideoStudioApp.instances.get('kf')`
	const toggle, section = `[data-action="scopes"]`, `[data-scopes]`
	kinds := []string{"histogram", "waveform", "vectorscope"}
	text := func(js string, args ...any) string { t.Helper(); return page.MustEval(js, args...).Str() }
	truth := func(js string, args ...any) bool { t.Helper(); return page.MustEval(js, args...).Bool() }
	click := func(selector string) {
		t.Helper()
		page.MustEval(`sel => document.querySelector(sel).click()`, selector)
	}
	reads := func() int { t.Helper(); return page.MustEval(`() => window.kfScopeReads`).Int() }
	play := func(d time.Duration) {
		t.Helper()
		page.MustEval(`() => { const s = ` + app + `; s.preview.pause(); s.preview.seek(0); s.preview.play(); }`)
		time.Sleep(d)
		page.MustEval(`() => ` + app + `.preview.pause()`)
	}
	pressed := func(kind string) string {
		t.Helper()
		return text(`k => document.querySelector('[data-scope="' + k + '"]').getAttribute('aria-pressed') + '|' + (document.querySelector('[data-scope-tile="' + k + '"]').hidden ? 'hidden' : 'shown')`, kind)
	}
	stored := func() string {
		t.Helper()
		return text(`() => String(localStorage.getItem('aurago.videoStudio.scopes'))`)
	}
	requireData := func(step string, want ...string) {
		t.Helper()
		stats := videoStudioScopeCanvasStats(t, page)
		for _, kind := range want {
			if stats[kind] == 0 {
				t.Errorf("%s: the %s shows no data (data pixels %v)", step, kind, stats)
			}
		}
	}

	if kind := text(`() => ` + app + `.preview.painterKind()`); kind != "gl" {
		t.Fatalf("the panel part needs the GL painter under the SwiftShader launcher, got %q", kind)
	}
	// The panel reads through s.preview.readScaled; count the calls and their sizes.
	page.MustEval(`() => { const preview = ` + app + `.preview, real = preview.readScaled; window.kfScopeReads = 0; window.kfScopeSizes = []; preview.readScaled = (width, height) => { window.kfScopeReads++; window.kfScopeSizes.push(width + 'x' + height); return real(width, height); }; }`)

	// Closed (the default): the toggle sits in the transport before "Render range exactly", and playing reads nothing.
	if got := text(`sel => { const b = document.querySelector(sel); return [b.getAttribute('aria-expanded'), b.nextElementSibling && b.nextElementSibling.dataset.action, b.closest('.vs-transport-right') ? 'transport' : 'elsewhere', b.getAttribute('aria-controls'), b.getAttribute('aria-label')].join('|'); }`, toggle); got != "false|range-render|transport|vs-scopes-kf|Scopes" {
		t.Fatalf("toggle expanded|next|place|controls|label = %s", got)
	}
	if !truth(`sel => document.querySelector(sel).hidden`, section) {
		t.Fatal("the scopes show before the toggle was used")
	}
	play(500 * time.Millisecond)
	if n := reads(); n != 0 {
		t.Errorf("a closed panel read %d frames during 0.5 s of play, want 0", n)
	}

	// Enter on the focused toggle opens the panel; focus stays on the toggle and every scope shows data.
	page.MustElement(toggle).MustFocus()
	page.Keyboard.MustType(input.Enter)
	waitForJSBool(t, page, `() => document.querySelector('`+toggle+`').getAttribute('aria-expanded') === 'true'`)
	if !truth(`sel => { const el = document.querySelector(sel); return !el.hidden && el.offsetHeight > 0; }`, section) {
		t.Fatal("the dock does not show after opening")
	}
	if !truth(`sel => document.activeElement === document.querySelector(sel)`, toggle) {
		t.Error("opening moved focus off the toggle")
	}
	requireData("open", kinds...)
	if got := text(`() => window.kfScopeSizes.length ? window.kfScopeSizes[0] : ''`); got != "256x144" {
		t.Errorf("the panel read %q, want 256x144", got)
	}
	if got := text(`() => document.querySelector('[data-scope-canvas="waveform"]').getAttribute('aria-label')`); got != "Waveform of the preview: luma (BT.709) of each column from left to right, 0 % at the bottom, 100 % at the top" {
		t.Errorf("waveform canvas label %q", got)
	}
	if got := stored(); got != `{"open":true,"show":["histogram","waveform","vectorscope"]}` {
		t.Errorf("stored preference %s", got)
	}

	// Playing with the panel open: about 10 reads per second at most.
	page.MustEval(`() => { window.kfScopeReads = 0; }`)
	play(time.Second)
	if n := reads(); n < 1 || n > 13 {
		t.Errorf("an open panel read %d frames in 1 s of play, want 1..13", n)
	}

	// Kind buttons: waveform off hides its tile; the last shown kind stays pressed; the preference follows.
	click(`[data-scope="waveform"]`)
	if got := pressed("waveform"); got != "false|hidden" {
		t.Errorf("waveform after its button: %s, want false|hidden", got)
	}
	click(`[data-scope="histogram"]`)
	click(`[data-scope="vectorscope"]`)
	if got := pressed("vectorscope"); got != "true|shown" {
		t.Errorf("the last shown kind: %s, want true|shown", got)
	}
	if got := stored(); got != `{"open":true,"show":["vectorscope"]}` {
		t.Errorf("stored preference after the kind buttons: %s", got)
	}
	click(`[data-scope="histogram"]`)
	click(`[data-scope="waveform"]`)
	requireData("kinds shown again", kinds...)

	// Escape in the bar closes the panel and focuses the toggle.
	page.MustElement(`[data-scope="histogram"]`).MustFocus()
	page.Keyboard.MustType(input.Escape)
	waitForJSBool(t, page, `() => document.querySelector('`+section+`').hidden`)
	if got := text(`sel => document.querySelector(sel).getAttribute('aria-expanded') + '|' + (document.activeElement === document.querySelector(sel))`, toggle); got != "false|true" {
		t.Errorf("after Escape in the bar: expanded|focused = %s, want false|true", got)
	}

	// Reopen; source mode hides the dock with the stage, and the timeline brings it back.
	click(toggle)
	waitForJSBool(t, page, `() => !document.querySelector('`+section+`').hidden`)
	page.MustEval(`() => window.VideoStudioParts.openSource(` + app + `, 'art1')`)
	waitForJSBool(t, page, `() => document.querySelector('.vs-scopes-dock').hidden`)
	page.MustEval(`() => window.VideoStudioParts.setStageMode(` + app + `, 'timeline')`)
	waitForJSBool(t, page, `() => !document.querySelector('.vs-scopes-dock').hidden && !document.querySelector('`+section+`').hidden`)

	// Layouts: no sideways scrolling of the centre column, the toggle in the transport row, the panel height.
	// Review I-2: with the panel open the centre column must also fit its grid row and the timeline must stay whole.
	// - The centre column may not scroll vertically (scrollHeight - clientHeight at most 1 px), and its last row, the
	//   transport, ends inside .vs-workspace (which clips what overflows it).
	// - The timeline panel is not part of the workspace: it sits below it. Its bottom stays inside the app and its top
	//   below the workspace bottom, so neither covers the other.
	type layout struct {
		Overflow  int     `json:"overflow"`
		OverflowY int     `json:"overflowY"`
		Panel     int     `json:"panel"`
		Stage     int     `json:"stage"`
		Transport float64 `json:"transport"`
		Workspace float64 `json:"workspace"`
		TimeTop   float64 `json:"timeTop"`
		TimeEnd   float64 `json:"timeEnd"`
		AppEnd    float64 `json:"appEnd"`
		Library   int     `json:"library"`
		InRow     bool    `json:"inRow"`
	}
	measure := func(name string) layout {
		t.Helper()
		var got layout
		raw := text(`() => { const center = document.querySelector('.vs-center'), transport = document.querySelector('.vs-transport').getBoundingClientRect(), button = document.querySelector('[data-action="scopes"]').getBoundingClientRect(), panel = document.querySelector('.vs-scopes'), time = document.querySelector('.vs-timeline-panel').getBoundingClientRect(); return JSON.stringify({overflow: center.scrollWidth - center.clientWidth, overflowY: center.scrollHeight - center.clientHeight, panel: panel.hidden ? 0 : panel.offsetHeight, stage: document.querySelector('.vs-preview-stage').offsetHeight, transport: transport.bottom, workspace: document.querySelector('.vs-workspace').getBoundingClientRect().bottom, timeTop: time.top, timeEnd: time.bottom, appEnd: document.querySelector('.vs-app').getBoundingClientRect().bottom, library: document.querySelector('.vs-library').offsetHeight, inRow: button.width > 0 && button.left >= transport.left - 1 && button.right <= transport.right + 1 && button.top >= transport.top - 1 && button.bottom <= transport.bottom + 1}); }`)
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("%s layout: %v (%s)", name, err, raw)
		}
		if got.OverflowY > 1 {
			t.Errorf("%s: the centre column overflows its row by %d px (scrollHeight - clientHeight)", name, got.OverflowY)
		}
		if got.Transport > got.Workspace+1 {
			t.Errorf("%s: the transport ends at y %.1f, below the workspace bottom %.1f", name, got.Transport, got.Workspace)
		}
		if got.TimeEnd > got.AppEnd+1 || got.TimeTop < got.Workspace-1 {
			t.Errorf("%s: the timeline panel spans y %.1f..%.1f, want inside the app (bottom %.1f) and below the workspace (bottom %.1f)", name, got.TimeTop, got.TimeEnd, got.AppEnd, got.Workspace)
		}
		return got
	}
	for _, size := range []struct{ width, height, panel int }{{1280, 900, 132}, {900, 900, 132}, {390, 844, 96}} {
		name := fmt.Sprintf("%dx%d", size.width, size.height)
		page.MustSetViewport(size.width, size.height, 1, false)
		time.Sleep(300 * time.Millisecond)
		got := measure(name)
		if got.Overflow > 1 {
			t.Errorf("%s: the centre column scrolls sideways by %d px", name, got.Overflow)
		}
		if !got.InRow {
			t.Errorf("%s: the scopes toggle left the transport row", name)
		}
		if got.Panel != size.panel {
			t.Errorf("%s: the panel is %d px high, want %d", name, got.Panel, size.panel)
		}
		t.Logf("%s open: panel %d px, stage %d px, library %d px, centre overflow %d px, transport bottom %.1f, workspace bottom %.1f, timeline %.1f..%.1f", name, got.Panel, got.Stage, got.Library, got.OverflowY, got.Transport, got.Workspace, got.TimeTop, got.TimeEnd)
		page.MustScreenshot(filepath.Join(shots, "scopes-"+name+".png"))
		if size.width == 390 {
			// Source mode hides the dock, so the library row comes back with it; the timeline brings the dock back.
			page.MustEval(`() => window.VideoStudioParts.openSource(` + app + `, 'art1')`)
			waitForJSBool(t, page, `() => document.querySelector('.vs-scopes-dock').hidden`)
			if library := page.MustEval(`() => document.querySelector('.vs-library').offsetHeight`).Int(); library < 100 {
				t.Errorf("%s: source mode left the library %d px high, want it back (100 px or more)", name, library)
			}
			page.MustEval(`() => window.VideoStudioParts.setStageMode(` + app + `, 'timeline')`)
			waitForJSBool(t, page, `() => !document.querySelector('.vs-scopes-dock').hidden && !document.querySelector('`+section+`').hidden`)
			// The closed phone layout is the reference the open one must not break: same checks, screenshot for review.
			click(toggle)
			waitForJSBool(t, page, `() => document.querySelector('`+section+`').hidden`)
			time.Sleep(300 * time.Millisecond)
			closed := measure(name + " closed")
			t.Logf("%s closed: stage %d px, library %d px, centre overflow %d px, transport bottom %.1f, workspace bottom %.1f, timeline %.1f..%.1f", name, closed.Stage, closed.Library, closed.OverflowY, closed.Transport, closed.Workspace, closed.TimeTop, closed.TimeEnd)
			// The open panel takes the library row (review I-2): the stage keeps 150 px or more instead of its 90 px floor,
			// and closing the panel brings the library back.
			if got.Library != 0 || got.Stage < 150 || closed.Library < 100 {
				t.Errorf("%s: library %d px and stage %d px with the panel open, library %d px closed; want the library collapsed, a stage of 150 px or more, and the library back (100 px or more) after closing", name, got.Library, got.Stage, closed.Library)
			}
			page.MustScreenshot(filepath.Join(shots, "scopes-"+name+"-closed.png"))
			click(toggle)
			waitForJSBool(t, page, `() => !document.querySelector('`+section+`').hidden`)
		}
	}
	page.MustSetViewport(1280, 900, 1, false)
	if errs := text(`() => JSON.stringify(window.kfErrors)`); errs != "[]" {
		t.Fatalf("script errors in the scopes panel part: %s", errs)
	}

	// The 2D painter: the hidden previewEngine preference, then a reload. The stored preference opens the panel.
	page.MustEval(`() => localStorage.setItem('aurago.videoStudio.previewEngine', '2d')`)
	page.MustReload()
	page.MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('.vs-clip[data-clip-id="c1"]') || window.kfErrors.length > 0`)
	if kind := text(`() => ` + app + `.preview.painterKind()`); kind != "2d" {
		t.Fatalf("the forced 2D pass draws with %q", kind)
	}
	waitForJSBool(t, page, `() => !document.querySelector('`+section+`').hidden`)
	requireData("2D painter", kinds...)
	page.MustScreenshot(filepath.Join(shots, "scopes-2d-1280x900.png"))
	if errs := text(`() => JSON.stringify(window.kfErrors)`); errs != "[]" {
		t.Fatalf("script errors in the 2D scopes pass: %s", errs)
	}
	page.MustEval(`() => { localStorage.removeItem('aurago.videoStudio.previewEngine'); localStorage.removeItem('aurago.videoStudio.scopes'); }`)
}

// videoStudioScopeCanvasStats counts the data pixels of every shown scope canvas. The read is asynchronous, so it waits
// up to 10 s until each of them has some.
//   - Histogram data pixels are the bars and the luma line: a channel of at least 120 (the grid strokes stay below 64).
//   - Waveform and vectorscope data pixels are the green traces: G − R > 30 and G − B > 15, which no graticule stroke is.
//   - Hidden tiles and a closed panel count nothing.
func videoStudioScopeCanvasStats(t *testing.T, page *rod.Page) map[string]int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		raw := page.MustEval(`() => { const out = {}, section = document.querySelector('[data-scopes]'); if (!section || section.hidden) return JSON.stringify(out); section.querySelectorAll('[data-scope-tile]').forEach(tile => { if (tile.hidden) return; const kind = tile.dataset.scopeTile, canvas = tile.querySelector('canvas'), data = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data; let n = 0; for (let i = 0; i < data.length; i += 4) { const r = data[i], g = data[i + 1], b = data[i + 2]; if (kind === 'histogram' ? Math.max(r, g, b) >= 120 : g - r > 30 && g - b > 15) n++; } out[kind] = n; }); return JSON.stringify(out); }`).Str()
		var stats map[string]int
		if err := json.Unmarshal([]byte(raw), &stats); err != nil {
			t.Fatalf("scope canvas stats: %v (%s)", err, raw)
		}
		ready := len(stats) > 0
		for _, n := range stats {
			ready = ready && n > 0
		}
		if ready || time.Now().After(deadline) {
			return stats
		}
		time.Sleep(50 * time.Millisecond)
	}
}
