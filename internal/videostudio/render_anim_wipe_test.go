package videostudio

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// renderAnimGraph runs a hand-built filter graph the way the export runs a segment:
//   - stageVideoGraph writes the graph and its command files into a fresh staging directory;
//   - runFFmpegIn runs FFmpeg there;
//   - stills are looped image inputs 0, 1, …; nil runs a graph of sources only;
//   - extra options go in front of the graph (thread limits).
//
// It returns the first frames of the label vs_video_out as raw pixFmt frames of frameSize bytes each. Task 8 reuses it.
func renderAnimGraph(t *testing.T, ctx context.Context, ffmpeg, graph string, commands []sendCmdFile, rate Rate, stills []string, frames, frameSize int, pixFmt string, extra ...string) [][]byte {
	t.Helper()
	root := t.TempDir()
	workDir, graphPath, err := stageVideoGraph(root, graph, rate, graphFiles{commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)
	inputs := make([]segmentInput, len(stills))
	for i, path := range stills {
		inputs[i] = segmentInput{path: path, image: true}
	}
	output := filepath.Join(root, "frames.raw")
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error"}
	args = append(args, segmentInputArgs(inputs, rate)...)
	args = append(args, extra...)
	args = append(args, "-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-frames:v", strconv.Itoa(frames),
		"-fps_mode", "passthrough", "-f", "rawvideo", "-pix_fmt", pixFmt, output)
	if err := runFFmpegIn(ctx, workDir, ffmpeg, args, rate.Seconds(frames), nil, inputPaths(inputs), output); err != nil {
		t.Fatalf("render graph: %v\n%s", err, graph)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != frameSize*frames {
		t.Fatalf("graph output has %d bytes, want %d frames of %d", len(data), frames, frameSize)
	}
	out := make([][]byte, frames)
	for k := range out {
		out[k] = data[k*frameSize : (k+1)*frameSize]
	}
	return out
}

// wipeColumns is the reveal model on the project canvas at an absolute frame (index §4.24). It returns the columns
// [from, to) the wipe shows, and whether a wipe span is active. The box's first column is its animated centre less half
// its width (animBox), which is where place puts the box of the fit canvas.
func wipeColumns(clip Clip, frame, canvasWidth, canvasHeight int) (from, to int, active bool) {
	a := animAt(clip, frame)
	if a.Wipe == 0 {
		return 0, canvasWidth, false
	}
	f := clipFrameAt(clip, frame)
	x, y, w, h := clipBox(Clip{X: f.X, Y: f.Y, Width: f.Width, Height: f.Height}, canvasWidth, canvasHeight)
	cx, _, boxWidth, _ := animBox(x, y, w, h, a, canvasWidth, canvasHeight)
	x0, x1 := animRevealPx(a, boxWidth)
	left := cx - boxWidth/2
	return left + x0, left + x1, true
}

// checkWipeLayer compares one layer frame with the same frame of a reference chain. Inside a span (active), the
// columns [from, to) must equal the reference, and every other column must have alpha 0. Outside the spans the whole
// frame must equal it.
//   - Colour is compared exactly where both alphas are above 0; alpha within alphaTolerance.
//   - It returns the hidden pixels that the reference shows.
//
// Task 8 uses it with active false for its frames at rest.
func checkWipeLayer(t *testing.T, name string, got, want []byte, width, height, from, to int, active bool, alphaTolerance int) int {
	t.Helper()
	hid := 0
	for y := range height {
		for x := range width {
			i := (y*width + x) * 4
			if active && (x < from || x >= to) {
				if got[i+3] != 0 {
					t.Fatalf("%s: pixel (%d, %d) lies outside the shown columns [%d, %d) but has alpha %d", name, x, y, from, to, got[i+3])
				}
				if want[i+3] > 0 {
					hid++
				}
				continue
			}
			if d := absInt(int(got[i+3]) - int(want[i+3])); d > alphaTolerance {
				t.Fatalf("%s: pixel (%d, %d) has alpha %d, the reference %d (tolerance %d)", name, x, y, got[i+3], want[i+3], alphaTolerance)
			}
			if got[i+3] > 0 && want[i+3] > 0 && !bytes.Equal(got[i:i+3], want[i:i+3]) {
				t.Fatalf("%s: pixel (%d, %d) has colour %v, the reference %v", name, x, y, got[i:i+3], want[i:i+3])
			}
		}
	}
	return hid
}

// The gate for blend's timeline enable (Task 7, controller decision 1). The reveal stage's own structure merges an
// alpha-0 row into a white still: geq on one row, a neighbour upscale, and blend c3_mode=multiply with animSpanExpr's
// enable. So the merged alpha shows exactly when the blend runs: 0 on the frames of the wipe spans, and the still's 255
// on every other frame.
//   - Rates: 25 and 30000/1001 fps.
//   - Windows: starting at the clip, inside the in span, and inside the out span.
//   - Threads: FFmpeg's default, 1 and 16.
//
// The test fails when a single frame switches early or late. The fallback is animRevealGated = false (Step 4).
func TestRenderRevealBlendEnableGate(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	if !animRevealGated {
		t.Skip("animRevealGated is false: the reveal merge runs on every frame (Task 7 gate fallback)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	white := filepath.Join(t.TempDir(), "white.png")
	writeFramePNG(t, white, 64, 8, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	clip := Clip{ID: "g", AssetID: "g", Duration: 24, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
		AnimIn: &ClipAnim{Preset: "wipe", Duration: 7}, AnimOut: &ClipAnim{Preset: "wipe", Duration: 6}}
	// D_in = 7: frames [0, 7); D_out = 6: frames [18, 24).
	for _, rate := range []Rate{{Num: 25, Den: 1}, {Num: 30000, Den: 1001}} {
		for _, at := range []int{0, 5, 19} {
			local := clip
			local.Duration = clip.Duration - at
			chain := clipChain{item: layerClip{clip: local, whole: clip, at: at, image: true, pictureWidth: 64, pictureHeight: 8}, canvasWidth: 64, canvasHeight: 8, rate: rate}
			enable := animSpanExpr(clip, at, rate, "wipe").String()
			graph := fmt.Sprintf("[0:V:0]%s,%s,format=gbrap[vs_g_in];nullsrc=s=64x1:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=gbrap,geq=r='0':g='0':b='0':a='0',scale=w=64:h=8:flags=neighbor[vs_g_m];[vs_g_in][vs_g_m]blend=c0_mode=normal:c1_mode=normal:c2_mode=normal:c3_mode=multiply:enable='%s',format=gbrap[vs_video_out]",
				chain.retime(), chain.window(), rate, rate.SecondsArg(local.Duration), local.Duration, enable)
			for _, threads := range [][]string{nil, {"-filter_threads", "1", "-filter_complex_threads", "1"}, {"-filter_threads", "16", "-filter_complex_threads", "16"}} {
				out := renderAnimGraph(t, ctx, ffmpeg, graph, nil, rate, []string{white}, local.Duration, 64*8*4, "gbrap", threads...)
				for k, frame := range out {
					want := byte(255)
					if animAt(clip, at+k).Wipe != 0 {
						want = 0
					}
					for _, alpha := range frame[3*64*8:] {
						if alpha != want {
							t.Fatalf("rate %s, window at %d, threads %v: frame %d has merged alpha %d, want %d (enable %s)", rate, at, threads, at+k, alpha, want, enable)
						}
					}
				}
			}
		}
	}
}

// TestRenderAnimWipeEdges renders wiped clips on FFmpeg and compares every frame with the same chain without the wipe
// (index §4.24, Task 7 gate):
//   - on the canvas columns the Go model shows (wipeColumns: animAt, animBox, animRevealPx), the layer equals the
//     reference;
//   - on the columns it hides, the alpha is 0;
//   - outside the spans, the whole layer equals the reference.
//
// Colour is exact. Alpha is exact, except for the still with alpha below FFmpeg 6: there 5.1.9's rgba → gbrap step in
// the reveal stage adds 1 to alpha 128..254 (index §4.23).
//
// The cases:
//   - a static box with an opaque chart at 1:1, so the reference and the reveal see the same bytes;
//   - the same box with a still whose alpha is 0, 200 and 255;
//   - an ellipse mask under the reveal;
//   - keyed geometry with an out pop on the pad canvas, whose reference keeps the pop; a window that starts at frame 3
//     renders the same frames;
//   - a rotated box. Its reference renders a still with the hidden box columns made transparent through the same chain
//     without the wipe, so rotate sees the same bytes.
func TestRenderAnimWipeEdges(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	root := t.TempDir()
	rate := Rate{Num: 25, Den: 1}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	stillTolerance := 0
	if major := ffmpegMajor(t, ffmpeg); major > 0 && major < 6 {
		stillTolerance = 1
	}
	paths := map[string]string{"chart": filepath.Join(root, "chart.png"), "alpha": filepath.Join(root, "alpha.png"),
		"white160": filepath.Join(root, "white-160.png"), "white320": filepath.Join(root, "white-320.png")}
	writePicturePNG(t, paths["chart"], keyChart(160, 90, [3]uint8{120, 140, 90}, nil))
	writePicturePNG(t, paths["alpha"], compositingStillPicture(160, 90))
	writeFramePNG(t, paths["white160"], 160, 90, white)
	writeFramePNG(t, paths["white320"], 320, 180, white)
	const frames = 20
	// boxClip is a 160x90 box at (80, 46) of a 320x180 canvas, wiped in over 6 frames ([0, 6)) and out over 5 ([15, 20)).
	boxClip := func(edit func(*Clip)) Clip {
		c := Clip{ID: "w", AssetID: "w", Duration: frames, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain,
			AnimIn: &ClipAnim{Preset: "wipe", Duration: 6}, AnimOut: &ClipAnim{Preset: "wipe", Duration: 5}}
		edit(&c)
		return c
	}
	noWipe := func(c *Clip) { c.AnimIn, c.AnimOut = nil, nil }
	chainOf := func(clip Clip, at, canvasWidth, canvasHeight, pictureWidth, pictureHeight int) clipChain {
		local := clip
		local.Start, local.Duration = 0, clip.Start+clip.Duration-at
		return clipChain{item: layerClip{clip: local, whole: clip, at: at, image: true, pictureWidth: pictureWidth, pictureHeight: pictureHeight,
			sourceAspect: float64(pictureWidth) / float64(pictureHeight)}, canvasWidth: canvasWidth, canvasHeight: canvasHeight, rate: rate}
	}
	mask := &Mask{Shape: MaskEllipse, X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Feather: 0.05}
	keyed := func(c *Clip) {
		c.X, c.Y, c.Width, c.Height = 0.1, 0.2, 0.3, 0.3
		c.Keyframes = map[string][]Keyframe{"x": linearKeys(0.1, 0.3, 19), "width": linearKeys(0.3, 0.5, 19), "height": linearKeys(0.3, 0.5, 19)}
		c.AnimIn, c.AnimOut = &ClipAnim{Preset: "wipe", Duration: 7}, &ClipAnim{Preset: "pop", Duration: 6}
	}
	cases := []struct {
		name                        string
		clip, reference             Clip
		still                       string
		canvasWidth, canvasHeight   int
		pictureWidth, pictureHeight int
		tolerance                   int
	}{
		{"static box, opaque chart", boxClip(func(*Clip) {}), boxClip(noWipe), paths["chart"], 320, 180, 160, 90, 0},
		{"static box, still with alpha", boxClip(func(*Clip) {}), boxClip(noWipe), paths["alpha"], 320, 180, 160, 90, stillTolerance},
		{"ellipse mask, then the reveal", boxClip(func(c *Clip) { c.Mask = mask }), boxClip(func(c *Clip) { c.Mask = mask; noWipe(c) }), paths["white160"], 320, 180, 160, 90, 0},
		{"keyed box with an out pop (pad canvas)", boxClip(keyed), boxClip(func(c *Clip) { keyed(c); c.AnimIn = nil }), paths["white320"], 640, 360, 320, 180, 0},
	}
	for _, c := range cases {
		layers := renderClipFramesRGBA(t, ctx, ffmpeg, chainOf(c.clip, 0, c.canvasWidth, c.canvasHeight, c.pictureWidth, c.pictureHeight), c.still, frames)
		references := renderClipFramesRGBA(t, ctx, ffmpeg, chainOf(c.reference, 0, c.canvasWidth, c.canvasHeight, c.pictureWidth, c.pictureHeight), c.still, frames)
		hid, active := 0, 0
		for k := range frames {
			from, to, wiping := wipeColumns(c.clip, k, c.canvasWidth, c.canvasHeight)
			if wiping {
				active++
			}
			hid += checkWipeLayer(t, fmt.Sprintf("%s, frame %d", c.name, k), layers[k], references[k], c.canvasWidth, c.canvasHeight, from, to, wiping, c.tolerance)
		}
		if active == 0 || hid == 0 {
			t.Fatalf("%s: %d frames inside the spans hid %d visible pixels; the case does not test the wipe", c.name, active, hid)
		}
		t.Logf("%s: %d frames inside the spans, %d visible pixels hidden, alpha tolerance %d", c.name, active, hid, c.tolerance)
		if c.canvasWidth == 640 {
			late := renderClipFramesRGBA(t, ctx, ffmpeg, chainOf(c.clip, 3, c.canvasWidth, c.canvasHeight, c.pictureWidth, c.pictureHeight), c.still, frames-3)
			for i, layer := range late {
				if !bytes.Equal(layer, layers[3+i]) {
					t.Fatalf("%s: frame %d differs when its window starts at frame 3", c.name, 3+i)
				}
			}
		}
	}

	// A rotated box: the reveal turns with the box.
	rotated := boxClip(func(c *Clip) { c.Rotation = 20 })
	still := boxClip(func(c *Clip) { c.Rotation = 20; noWipe(c); c.Duration = 1 })
	layers := renderClipFramesRGBA(t, ctx, ffmpeg, chainOf(rotated, 0, 320, 180, 160, 90), paths["white160"], frames)
	for _, k := range []int{0, 2, 4, 10, 16, 18} {
		x0, x1 := 0, 160
		if a := animAt(rotated, k); a.Wipe != 0 {
			x0, x1 = animRevealPx(a, 160)
		}
		picture := image.NewNRGBA(image.Rect(0, 0, 160, 90))
		for y := range 90 {
			for x := range 160 {
				alpha := uint8(0)
				if x >= x0 && x < x1 {
					alpha = 255
				}
				picture.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: alpha})
			}
		}
		path := filepath.Join(root, fmt.Sprintf("rotated-%d.png", k))
		writePicturePNG(t, path, picture)
		reference := renderClipFramesRGBA(t, ctx, ffmpeg, chainOf(still, 0, 320, 180, 160, 90), path, 1)[0]
		for i := range reference {
			if layers[k][i] != reference[i] {
				t.Fatalf("rotated box, frame %d (box columns [%d, %d)): byte %d (pixel %d) is %d, the reference %d", k, x0, x1, i, i/4, layers[k][i], reference[i])
			}
		}
	}
}
