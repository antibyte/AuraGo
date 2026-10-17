package videostudio

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// renderClipFramesRGBA runs one clip chain (clipChain.build) on its own, from one still input, as renderClipRGBA does.
// It returns the first `frames` layer frames as RGBA before any composition (canvasWidth × canvasHeight × 4 bytes each).
// The chain's window must hold at least that many frames. Over the transparent canvas, place copies the clip's straight
// RGBA wherever the alpha is above 0, so the layer's alpha is the clip's alpha.
// extra options go in front of the graph (FFmpeg thread limits, for instance).
func renderClipFramesRGBA(t *testing.T, ctx context.Context, ffmpeg string, chain clipChain, path string, frames int, extra ...string) [][]byte {
	t.Helper()
	built := chain.build("vs_clip_out")
	graph := built.filter + ";\n[vs_clip_out]format=rgba[vs_video_out]"
	root := t.TempDir()
	workDir, graphPath, err := stageVideoGraph(root, graph, chain.rate, graphFiles{commands: built.commands, luts: built.luts})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)
	inputs := []segmentInput{{path: path, image: true}}
	output := filepath.Join(root, "layers.rgba")
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error"}
	args = append(args, segmentInputArgs(inputs, chain.rate)...)
	args = append(args, extra...)
	args = append(args, "-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-frames:v", strconv.Itoa(frames),
		"-fps_mode", "passthrough", "-f", "rawvideo", "-pix_fmt", "rgba", output)
	if err := runFFmpegIn(ctx, workDir, ffmpeg, args, chain.rate.Seconds(frames), nil, inputPaths(inputs), output); err != nil {
		t.Fatalf("render clip layers: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	size := chain.canvasWidth * chain.canvasHeight * 4
	if len(data) != size*frames {
		t.Fatalf("layers have %d bytes, want %d frames of %d", len(data), frames, size)
	}
	out := make([][]byte, frames)
	for k := range out {
		out[k] = data[k*size : (k+1)*size]
	}
	return out
}

// keyChart is a picture around a key colour:
//   - red runs across x and green down y, 128 levels to either side of the key;
//   - blue swings ±48 levels in a diagonal pattern.
//
// It covers colorkey's whole response (inside the similarity, the smoothness ramp, outside) and despill's clamp.
// alpha gives each pixel's alpha (nil: opaque, which Go writes as an RGB PNG).
func keyChart(width, height int, key [3]uint8, alpha func(x, y int) uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	clamp := func(v int) uint8 { return uint8(min(max(v, 0), 255)) }
	for y := range height {
		for x := range width {
			c := color.NRGBA{
				R: clamp(int(key[0]) + (x-width/2)*256/width),
				G: clamp(int(key[1]) + (y-height/2)*256/height),
				B: clamp(int(key[2]) + (x*13+y*7)%97 - 48),
				A: 255,
			}
			if alpha != nil {
				c.A = alpha(x, y)
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// keyChain shows a width × height still 1:1 on a canvas of its size with an effect stack. still selects the builders'
// still path (the alpha wrapper); otherwise they take the video path.
func keyChain(width, height, frames int, still bool, effects ...EffectInstance) clipChain {
	clip := Clip{ID: "key", AssetID: "key", Duration: frames, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Effects: effects}
	return clipChain{item: layerClip{clip: clip, whole: clip, image: still, pictureWidth: width, pictureHeight: height}, canvasWidth: width, canvasHeight: height, rate: Rate{Num: 25, Den: 1}}
}

// chromaKeyHex is the key colour of chroma_key params: the color param, else the registry default.
func chromaKeyHex(params map[string]any) string {
	if hex, ok := params["color"].(string); ok {
		return hex
	}
	spec, _ := effectParamSpec("chroma_key", "color")
	hex, _ := spec.Default.(string)
	return hex
}

// keyAlphaTolerance is the alpha tolerance of a key layer:
//   - 0 on the video path, which has no gbrap step;
//   - 0 on the still path with the wrapper as well: alphaextract, the gray multiply and alphamerge work on rgba and
//     gray, so no rgba→gbrap step touches the alpha on any FFmpeg version (index §4.22);
//   - 1 without the wrapper, where colorkey multiplies with its own rounding.
func keyAlphaTolerance(still bool) int {
	if still && !colorkeyReplacesAlpha {
		return 1
	}
	return 0
}

// checkKeyLayer compares a rendered key layer with the Go 8-bit model of index §4.22 (the chart sits 1:1 in the layer):
//   - alpha = keyAlpha8 times the input alpha (blend's A·B//255);
//   - colour = despill8, or the input colour at spill 0, wherever the alpha stays above 0, exactly.
//
// The key reads the clip's chroma_key entry 0 at timeline frame frame.
func checkKeyLayer(t *testing.T, name string, layer []byte, chart *image.NRGBA, clip Clip, frame, alphaTolerance int) {
	t.Helper()
	p := keyParamsAt(clip, 0, frame)
	spill := effectValueAt(clip, 0, "spill", frame)
	width := chart.Rect.Dx()
	worstAlpha, worstColour := 0, 0
	var alphaAt, colourAt image.Point
	for y := range chart.Rect.Dy() {
		for x := range width {
			in := chart.NRGBAAt(x, y)
			rgb := [3]uint8{in.R, in.G, in.B}
			want := int(keyAlpha8(p, rgb)) * int(in.A) / 255
			i := (y*width + x) * 4
			if d := absInt(int(layer[i+3]) - want); d > worstAlpha {
				worstAlpha, alphaAt = d, image.Pt(x, y)
			}
			if layer[i+3] == 0 || want == 0 {
				continue // the composition keeps no colour where the alpha is 0
			}
			colour := rgb
			if spill > 0 {
				colour = despill8(p, rgb)
			}
			for ch := range 3 {
				if d := absInt(int(layer[i+ch]) - int(colour[ch])); d > worstColour {
					worstColour, colourAt = d, image.Pt(x, y)
				}
			}
		}
	}
	if worstAlpha > alphaTolerance || worstColour > 0 {
		t.Fatalf("%s: alpha off by %d at %v (tolerance %d), colour off by %d at %v (tolerance 0)", name, worstAlpha, alphaAt, alphaTolerance, worstColour, colourAt)
	}
	t.Logf("%s: largest alpha difference %d, colour difference %d", name, worstAlpha, worstColour)
}

// This test measures how colorkey treats an input alpha below 255 (index risk 20); the result decides the alpha
// wrapper. The raw filter keys a patch chart (the key colour, a colour in the smoothness ramp, white) whose rows have
// alpha 255, 200 and 128. Two outcomes are possible:
//   - replace: the output alpha is the key (keyAlpha8);
//   - multiply: the output alpha is the key times the input alpha, within 1 level.
//
// colorkeyReplacesAlpha must name the measured behaviour, and the still export must multiply either way:
// alpha = key·a/255 (checkKeyLayer).
func TestRenderColorkeyAlphaSemantics(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	colours := []color.NRGBA{{R: 0x00, G: 0xb1, B: 0x40}, {R: 140, G: 0xb1, B: 0x40}, {R: 255, G: 255, B: 255}}
	alphas := []uint8{255, 200, 128}
	const cell = 8
	patch := image.NewNRGBA(image.Rect(0, 0, cell*len(colours), cell*len(alphas)))
	for y := range patch.Rect.Dy() {
		for x := range patch.Rect.Dx() {
			c := colours[x/cell]
			c.A = alphas[y/cell]
			patch.SetNRGBA(x, y, c)
		}
	}
	path := filepath.Join(root, "patch.png")
	writePicturePNG(t, path, patch)
	entry := EffectInstance{ID: "chroma_key", On: true, Params: map[string]any{"color": "#00b140", "similarity": 0.25, "smoothness": 0.1, "spill": 0.0}}
	chain := keyChain(patch.Rect.Dx(), patch.Rect.Dy(), 1, true, entry)
	p := keyParamsAt(chain.item.whole, 0, 0)
	raw := filepath.Join(root, "raw.rgba")
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error", "-i", path,
		"-vf", "format=rgba,colorkey=color=0x00B140:similarity=0.25:blend=0.1", "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgba", raw)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("raw colorkey: %v: %s", err, out)
	}
	data, err := os.ReadFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(patch.Pix) {
		t.Fatalf("raw colorkey output has %d bytes, want %d", len(data), len(patch.Pix))
	}
	replace, multiply := true, true
	for i := 0; i < len(data); i += 4 {
		in := patch.Pix[i : i+4]
		k := float64(keyAlpha8(p, [3]uint8{in[0], in[1], in[2]}))
		got := float64(data[i+3])
		replace = replace && got == k
		multiply = multiply && math.Abs(got-k*float64(in[3])/255) <= 1
	}
	major := ffmpegMajor(t, ffmpeg)
	switch {
	case replace:
		t.Logf("colorkey replaces the input alpha on FFmpeg major %d", major)
	case multiply:
		t.Logf("colorkey multiplies the input alpha on FFmpeg major %d", major)
	default:
		t.Fatalf("colorkey's alpha on FFmpeg major %d is neither the key nor the key times the input alpha; report it (index risk 20)", major)
	}
	if replace != colorkeyReplacesAlpha {
		t.Fatalf("colorkey replaces the input alpha: %v on this FFmpeg, but colorkeyReplacesAlpha is %v; apply plan 2d Task 4 Step 7a", replace, colorkeyReplacesAlpha)
	}
	layer := renderClipFramesRGBA(t, ctx, ffmpeg, chain, path, 1)[0]
	checkKeyLayer(t, "still export", layer, patch, chain.item.whole, 0, keyAlphaTolerance(true))
}

// The chroma_key export matches the Go 8-bit model of index §4.22 (keyAlpha8, despill8):
//   - the video path (colorkey and despill on rgba, no gbrap step), at 0 levels: the registry defaults, a blue key with
//     a strong spill, a hard key (smoothness 0), spill 0 (despill left out) and full spill, each on a chart around its
//     key colour;
//   - the still path (the alpha wrapper when colorkeyReplacesAlpha): alpha = key·a/255 on rows of alpha 255, 200, 128
//     and 60, at the tolerance of keyAlphaTolerance; the colour stays exact.
//
// The test reads the clip layer itself, so colour and alpha are both checked without a backdrop.
func TestRenderChromaKeyMatchesFormula(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	const width, height = 320, 180
	bands := func(_, y int) uint8 { return []uint8{255, 200, 128, 60}[y*4/height] }
	for i, c := range []struct {
		name   string
		params map[string]any
		still  bool
	}{
		{"registry defaults", map[string]any{}, false},
		{"blue key with a strong spill", map[string]any{"color": "#0047bb", "similarity": 0.3, "smoothness": 0.15, "spill": 0.9}, false},
		{"hard key", map[string]any{"similarity": 0.2, "smoothness": 0.0, "spill": 0.3}, false},
		{"spill 0 leaves despill out", map[string]any{"similarity": 0.35, "smoothness": 0.05, "spill": 0.0}, false},
		{"full spill", map[string]any{"spill": 1.0}, false},
		{"still with graded alpha", map[string]any{"spill": 0.6}, true},
	} {
		chain := keyChain(width, height, 1, c.still, EffectInstance{ID: "chroma_key", On: true, Params: c.params})
		var alpha func(x, y int) uint8
		if c.still {
			alpha = bands
		}
		chart := keyChart(width, height, keyColor(chromaKeyHex(c.params)), alpha)
		path := filepath.Join(root, fmt.Sprintf("key-%d.png", i))
		writePicturePNG(t, path, chart)
		layer := renderClipFramesRGBA(t, ctx, ffmpeg, chain, path, 1)[0]
		checkKeyLayer(t, c.name, layer, chart, chain.item.whole, 0, keyAlphaTolerance(c.still))
	}
}

// Keyed similarity and smoothness follow ValueAt on every frame of one FFmpeg process. Spill is keyed too once the
// despill command gate has made it keyframable. colorkey@vs_key_L_C_I takes both options from sendcmd on every window
// frame:
//   - on the video path, group 0 is read in front of colorkey;
//   - on the still path, group 0 is read in front of the alpha wrapper, whose keyed colorkey sits in the split branch
//     before the merge.
func TestRenderChromaKeyKeyframes(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	const width, height, frames = 160, 90, 11
	keys := map[string][]Keyframe{
		"similarity": linearKeys(0.12, 0.4, 10),
		"smoothness": {{F: 0, V: 0, E: EaseLinear}, {F: 4, V: 0.05, E: EaseIn}, {F: 10, V: 0.25, E: EaseLinear}},
	}
	if spec, ok := effectParamSpec("chroma_key", "spill"); ok && spec.Keyframable {
		keys["spill"] = linearKeys(0.9, 0.1, 10)
	}
	entry := EffectInstance{ID: "chroma_key", On: true, Params: map[string]any{"color": "#00b140", "spill": 0.5}, Keyframes: keys}
	opaque := keyChart(width, height, keyColor("#00b140"), nil)
	graded := keyChart(width, height, keyColor("#00b140"), func(x, _ int) uint8 { return []uint8{255, 200, 128}[x*3/width] })
	for _, c := range []struct {
		name  string
		still bool
		chart *image.NRGBA
	}{{"video", false, opaque}, {"still", true, graded}} {
		path := filepath.Join(root, c.name+".png")
		writePicturePNG(t, path, c.chart)
		chain := keyChain(width, height, frames, c.still, entry)
		if files := chain.commandFiles(); len(files) != 1 || files[0].name != "vs_cmd_0_0.txt" || len(files[0].tracks) < 2 || files[0].tracks[0].Target != "colorkey@vs_key_0_0_0" {
			t.Fatalf("%s: command files %+v", c.name, files)
		}
		tolerance := keyAlphaTolerance(c.still)
		for k, layer := range renderClipFramesRGBA(t, ctx, ffmpeg, chain, path, frames) {
			checkKeyLayer(t, fmt.Sprintf("%s frame %d", c.name, k), layer, c.chart, chain.item.whole, k, tolerance)
		}
	}
}

// The despill command gate (index risk 20). despill@vs_spill_L_C_I takes the scale of its screen channel from sendcmd
// on every frame: green for a green key, blue for a blue key. Each commanded frame must equal a static render with that
// scale, byte for byte.
//   - The test logs the result on every FFmpeg it runs on.
//   - While spill is not keyframable it only reports.
//   - Once Task 4 has made spill keyframable (exact on 8.1.1 and in the 5.1.9 container), a difference fails it.
func TestRenderDespillCommands(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	const width, height, frames = 64, 64, 8
	chart := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			chart.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(255 - y*2), B: uint8((x*5 + y*9) % 256), A: 255})
		}
	}
	path := filepath.Join(root, "spill.png")
	writePicturePNG(t, path, chart)
	spill := func(k int) float64 { return 0.1 + 0.12*float64(k) }
	runs := 0
	render := func(dir, graph string, count int) []byte {
		t.Helper()
		runs++
		output := filepath.Join(dir, "out-"+strconv.Itoa(runs)+".rgba")
		cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error", "-loop", "1", "-framerate", "25", "-i", path,
			"-filter_complex", graph, "-map", "[o]", "-frames:v", strconv.Itoa(count), "-fps_mode", "passthrough", "-f", "rawvideo", "-pix_fmt", "rgba", output)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("despill run %q: %v: %s", graph, err, out)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != width*height*4*count {
			t.Fatalf("despill run %q has %d bytes, want %d frames", graph, len(data), count)
		}
		return data
	}
	exact := true
	for _, hex := range []string{"#00b140", "#0047bb"} {
		rgb := keyColor(hex)
		option := despillScale(rgb)
		dir := t.TempDir()
		var script strings.Builder
		for k := range frames {
			seconds := 0.0
			if k > 0 {
				seconds = (float64(k) - 0.5) / 25
			}
			fmt.Fprintf(&script, "%.6f despill@vs_spill_0_0_0 %s %s;\n", seconds, option, effectNum(0-spill(k)))
		}
		if err := os.WriteFile(filepath.Join(dir, "cmd.txt"), []byte(script.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		commanded := render(dir, "[0:v]format=rgba,sendcmd=f=cmd.txt,"+despillFilter("despill@vs_spill_0_0_0", rgb, spill(0))+"[o]", frames)
		size := width * height * 4
		for k := range frames {
			static := render(dir, "[0:v]format=rgba,"+despillFilter("despill", rgb, spill(k))+"[o]", 1)
			if !bytes.Equal(commanded[k*size:(k+1)*size], static) {
				exact = false
				t.Logf("despill %s: commanded frame %d differs from the static scale %s", option, k, effectNum(0-spill(k)))
			}
		}
	}
	major := ffmpegMajor(t, ffmpeg)
	if exact {
		t.Logf("despill green/blue commands are exact on FFmpeg major %d", major)
	} else {
		t.Logf("despill green/blue commands are NOT exact on FFmpeg major %d (spill stays static)", major)
	}
	if spec, _ := effectParamSpec("chroma_key", "spill"); spec.Keyframable && !exact {
		t.Fatal("spill is keyframable, but despill does not follow its scale commands on this FFmpeg; undo plan 2d Task 4 Step 7b")
	}
}
