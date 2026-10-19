package ui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"math"
	"os/exec"
	"testing"
	"time"

	"aurago/internal/videostudio/goldenframe"
)

// TestDesktopVideoStudioScopesBrowser checks the 2f scopes path in a real browser (index §4.26). Subtests:
//   - readScaled (Task 1): both painters' readScaled on the engine page under the SwiftShader launcher;
//   - panel (Task 3): the scopes panel in the app.
func TestDesktopVideoStudioScopesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	t.Run("readScaled", videoStudioReadScaledPart)
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
