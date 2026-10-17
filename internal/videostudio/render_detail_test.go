package videostudio

import (
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

// detailChart is a grey detail picture. The top half holds vertical sine gratings with periods of 4, 8, 16 and 32 px,
// whose contrast grows downwards. The bottom half holds a checkerboard of 8 px squares on the left and a hard vertical
// edge on the right. Grey keeps R, G and B equal, so one reference serves all three channels.
func detailChart(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			var v float64
			switch {
			case y < height/2:
				period := 4 * math.Pow(2, float64(x*4/width))
				v = 128 + 80*math.Sin(2*math.Pi*float64(x)/period)*(0.5+0.5*float64(y)/float64(height/2))
			case x < width/2:
				v = float64(70 + 120*((x/8+y/8)%2))
			case x < width*3/4:
				v = 40
			default:
				v = 210
			}
			g := uint8(math.Round(v))
			img.SetNRGBA(x, y, color.NRGBA{R: g, G: g, B: g, A: 255})
		}
	}
	return img
}

func unitClamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// casPlane holds, for each pixel of one channel, what AMD's contrast adaptive sharpening needs before the peak is
// chosen. It follows the published FidelityFX CAS, non-upscaling path:
//   - the centre value e and the sum of its four cross neighbours b, d, f, h;
//   - the shaped amount sqrt(clamp(min(mn, 2 − mx) / mx)). mn is the soft minimum: the cross minimum plus the minimum of
//     the 3x3 window. mx is the soft maximum, built the same way.
//
// Values are 0..1; the border pixel repeats outward. For a peak p (negative) the output is
// clamp((cross·w + e) / (1 + 4w)) with w = amount·p, and AMD's mapping is p = −1/mix(8, 5, sharpness).
type casPlane struct {
	centre, cross, amount []float64
}

func casPrepare(plane []float64, width, height int) casPlane {
	at := func(x, y int) float64 { return plane[min(max(y, 0), height-1)*width+min(max(x, 0), width-1)] }
	n := width * height
	p := casPlane{centre: make([]float64, n), cross: make([]float64, n), amount: make([]float64, n)}
	for y := range height {
		for x := range width {
			a, b, c := at(x-1, y-1), at(x, y-1), at(x+1, y-1)
			d, e, f := at(x-1, y), at(x, y), at(x+1, y)
			g, h, i := at(x-1, y+1), at(x, y+1), at(x+1, y+1)
			crossMin, crossMax := min(b, d, e, f, h), max(b, d, e, f, h)
			mn := crossMin + min(crossMin, a, c, g, i)
			mx := crossMax + max(crossMax, a, c, g, i)
			amount := 0.0
			if mx > 0 {
				amount = math.Sqrt(unitClamp(math.Min(mn, 2-mx) / mx))
			}
			k := y*width + x
			p.centre[k], p.cross[k], p.amount[k] = e, b+d+f+h, amount
		}
	}
	return p
}

// sharpen applies the filter with peak p (negative), 0..1.
func (p casPlane) sharpen(peak float64) []float64 {
	out := make([]float64, len(p.centre))
	for k := range out {
		w := p.amount[k] * peak
		out[k] = unitClamp((p.cross[k]*w + p.centre[k]) / (1 + 4*w))
	}
	return out
}

// casReference is AMD's CAS with the recorded peak mapping −1/mix(casPeakP0, casPeakP1, strength) (index §4.20): the
// sharpening the preview shader models.
func casReference(prepared casPlane, strength float64) []float64 {
	return prepared.sharpen(-1 / (casPeakP0 + (casPeakP1-casPeakP0)*strength))
}

// gaussianBlur is a separable Gaussian with sigma s over ±ceil(4s), normalized, the border pixel repeated outward.
func gaussianBlur(plane []float64, width, height int, sigma float64) []float64 {
	radius := int(math.Ceil(4 * sigma))
	kernel := make([]float64, 2*radius+1)
	sum := 0.0
	for i := range kernel {
		d := float64(i - radius)
		kernel[i] = math.Exp(-d * d / (2 * sigma * sigma))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	rows := make([]float64, len(plane))
	for y := range height {
		for x := range width {
			s := 0.0
			for i, w := range kernel {
				s += w * plane[y*width+min(max(x+i-radius, 0), width-1)]
			}
			rows[y*width+x] = s
		}
	}
	out := make([]float64, len(plane))
	for y := range height {
		for x := range width {
			s := 0.0
			for i, w := range kernel {
				s += w * rows[min(max(y+i-radius, 0), height-1)*width+x]
			}
			out[y*width+x] = s
		}
	}
	return out
}

// planePSNR is the PSNR in dB of got against want (both 0..255), leaving out a border of border pixels.
func planePSNR(got, want []float64, width, height, border int) float64 {
	sum, n := 0.0, 0
	for y := border; y < height-border; y++ {
		for x := border; x < width-border; x++ {
			d := got[y*width+x] - want[y*width+x]
			sum += d * d
			n++
		}
	}
	if sum == 0 {
		return math.Inf(1)
	}
	return 10 * math.Log10(255*255/(sum/float64(n)))
}

// boxPlane is the red channel (0..255) of chartBox in an rgb24 1280x720 frame.
func boxPlane(frame []byte) []float64 {
	plane := make([]float64, 0, chartBox.Dx()*chartBox.Dy())
	for y := chartBox.Min.Y; y < chartBox.Max.Y; y++ {
		for x := chartBox.Min.X; x < chartBox.Max.X; x++ {
			plane = append(plane, float64(frame[(y*1280+x)*3]))
		}
	}
	return plane
}

// The vignette's falloff is cos⁴(α·r/R) with R = hypot(W/2, H/2), where α is the angle that vignetteAngle folds amount
// and size into, and rgb is scaled by it (index §4.20). On flat grey 200 it holds within 2/255 for:
//   - static values, one of them clamped at π/2;
//   - a keyframed amount rendered in one process (eval=frame).
//
// If the law does not hold, the test searches the angle scale, the power and the pixel-centre offset. Record the best
// fit it reports in index §4.20; do not loosen the tolerance.
func TestVignetteMatchesFormula(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"chart": filepath.Join(root, "grey.png")}
	writeFramePNG(t, paths["chart"], 640, 360, color.NRGBA{R: 200, G: 200, B: 200, A: 255})
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "chart")
	radius := math.Hypot(320, 180)
	law := func(angle, scale, power, offset float64) func(x, y int) float64 {
		return func(x, y int) float64 {
			c := math.Cos(math.Min(math.Pi/2, scale*angle) * math.Hypot(float64(x)+offset-320, float64(y)+offset-180) / radius)
			return 200 * math.Pow(math.Max(c, 0), power)
		}
	}
	worst := func(frame []byte, f func(x, y int) float64) (float64, image.Point) {
		largest, at := 0.0, image.Point{}
		for y := chartBox.Min.Y; y < chartBox.Max.Y; y++ {
			for x := chartBox.Min.X; x < chartBox.Max.X; x++ {
				want := f(x-chartBox.Min.X, y-chartBox.Min.Y)
				for ch := range 3 {
					if d := math.Abs(float64(frame[(y*1280+x)*3+ch]) - want); d > largest {
						largest, at = d, image.Pt(x-chartBox.Min.X, y-chartBox.Min.Y)
					}
				}
			}
		}
		return largest, at
	}
	check := func(name string, frame []byte, angle float64) {
		t.Helper()
		e, at := worst(frame, law(angle, 1, 4, 0))
		if e <= 2 {
			t.Logf("%s: α %.4f, largest difference %.2f/255", name, angle, e)
			return
		}
		best, fit := math.Inf(1), ""
		for _, offset := range []float64{0, 0.5} {
			for power := 2.0; power <= 6; power++ {
				for scale := 0.5; scale <= 2.0001; scale += 0.01 {
					if e2, _ := worst(frame, law(angle, scale, power, offset)); e2 < best {
						best, fit = e2, fmt.Sprintf("cos^%.0f(%.2f·α·r/R), pixel offset %.1f", power, scale, offset)
					}
				}
			}
		}
		t.Fatalf("%s: cos⁴(α·r/R) is off by %.2f/255 at %v; the best measured law is %s (largest difference %.2f/255), record it in index §4.20", name, e, at, fit, best)
	}
	for i, c := range []struct{ amount, size float64 }{{0.6, 1}, {0.9, 1.6}, {1, 0.25}} {
		project := chartProject(assets, 10, EffectInstance{ID: "vignette", On: true, Params: map[string]any{"amount": c.amount, "size": c.size}})
		if err := Validate(project); err != nil {
			t.Fatalf("vignette %d invalid: %v", i, err)
		}
		img := renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: 0, Output: filepath.Join(root, fmt.Sprintf("vignette-%d.png", i))})
		check(fmt.Sprintf("amount %.2f size %.2f", c.amount, c.size), imageRGB(img), vignetteAngle(c.amount, c.size, 640, 360))
	}
	keyed := EffectInstance{ID: "vignette", On: true, Params: map[string]any{"size": 1.0}, Keyframes: map[string][]Keyframe{"amount": linearKeys(0.2, 1, 10)}}
	project := chartProject(assets, 12, keyed)
	if err := Validate(project); err != nil {
		t.Fatalf("keyframed vignette invalid: %v", err)
	}
	frames := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 0, End: 11})
	for _, k := range []int{0, 5, 10} {
		check(fmt.Sprintf("keyframed frame %d", k), frames[k], vignetteAngle(effectValueAt(project.Tracks[0].Clips[0], 0, "amount", k), 1, 640, 360))
	}
}

// FFmpeg's cas follows AMD's published contrast adaptive sharpening. The test fits the peak mapping −1/mix(P0, P1, s)
// to it, starting from AMD's mapping (8, 5), and checks the recorded constants casPeakP0 and casPeakP1 (picture_math.go;
// JS VideoStudioPicture.CAS_P0/CAS_P1, the GL shader's u_peak):
//   - each lies within 3 % of the fit;
//   - with them (casReference) the mean difference stays within 2/255 at every tested strength.
//
// AURAGO_PRINT_CAS_FIT=1 logs the fit as the two Go lines to copy into picture_math.go and skips both checks.
// Strength 0 still sharpens, which is why the builder never leaves sharpen out.
func TestSharpenMatchesReference(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	chart := detailChart(640, 360)
	paths := map[string]string{"chart": filepath.Join(root, "detail.png")}
	writePicturePNG(t, paths["chart"], chart)
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "chart")
	input := make([]float64, 640*360)
	for i := range input {
		input[i] = float64(chart.Pix[i*4]) / 255
	}
	prepared := casPrepare(input, 640, 360)
	render := func(i int, strength float64) []byte {
		project := chartProject(assets, 10, EffectInstance{ID: "sharpen", On: true, Params: map[string]any{"strength": strength}})
		if err := Validate(project); err != nil {
			t.Fatalf("sharpen %.2f invalid: %v", strength, err)
		}
		img := renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: 0, Output: filepath.Join(root, fmt.Sprintf("sharpen-%d.png", i))})
		return cropRGB(imageRGB(img), 1280, chartBox)
	}
	// meanAgainst is the mean absolute difference (0..255) of a rendered box from a reference plane, without the border
	// pixel.
	meanAgainst := func(got []byte, reference []float64) float64 {
		sum, n := 0.0, 0
		for y := 1; y < 359; y++ {
			for x := 1; x < 639; x++ {
				k := y*640 + x
				want := math.Round(255 * reference[k])
				for ch := range 3 {
					sum += math.Abs(float64(got[k*3+ch]) - want)
					n++
				}
			}
		}
		return sum / float64(n)
	}
	meanError := func(got []byte, divisor float64) float64 { return meanAgainst(got, prepared.sharpen(-1/divisor)) }
	fit := func(got []byte, start float64) (float64, float64) {
		divisor, best := start, meanError(got, start)
		for d := 2.0; d <= 40; d += 0.5 {
			if e := meanError(got, d); e < best {
				divisor, best = d, e
			}
		}
		lo, hi := math.Max(2, divisor-0.5), divisor+0.5
		for range 40 {
			m1, m2 := lo+(hi-lo)*0.382, lo+(hi-lo)*0.618
			if meanError(got, m1) < meanError(got, m2) {
				hi = m2
			} else {
				lo = m1
			}
		}
		if d := (lo + hi) / 2; meanError(got, d) < best {
			divisor, best = d, meanError(got, d)
		}
		return divisor, best
	}
	strengths := []float64{0, 0.25, 0.5, 0.75, 1}
	outputs := make([][]byte, len(strengths))
	divisors := make([]float64, len(strengths))
	for i, s := range strengths {
		outputs[i] = render(i, s)
		var e float64
		divisors[i], e = fit(outputs[i], 8+(5-8)*s)
		t.Logf("strength %.2f: fitted peak divisor %.3f, mean difference %.3f/255", s, divisors[i], e)
	}
	p0, p1 := divisors[0], divisors[len(divisors)-1]
	t.Logf("CAS peak mapping −1/mix(P0, P1, strength): P0 = %.3f, P1 = %.3f (AMD's published mapping: 8, 5)", p0, p1)
	for i, s := range strengths {
		if e := meanError(outputs[i], p0+(p1-p0)*s); e > 2 {
			t.Fatalf("strength %.2f: mean difference %.3f/255 with the fitted mapping, want at most 2/255", s, e)
		}
	}
	if os.Getenv("AURAGO_PRINT_CAS_FIT") == "1" {
		// The pinning run of Step 5: copy both lines over the constants in picture_math.go.
		t.Logf("casPeakP0 = %.3f", p0)
		t.Logf("casPeakP1 = %.3f", p1)
	} else {
		if math.Abs(p0-casPeakP0) > 0.03*casPeakP0 || math.Abs(p1-casPeakP1) > 0.03*casPeakP1 {
			t.Fatalf("fitted P0 %.3f and P1 %.3f differ by more than 3 %% from casPeakP0 %v and casPeakP1 %v; pin them with AURAGO_PRINT_CAS_FIT=1, or report the FFmpeg difference", p0, p1, casPeakP0, casPeakP1)
		}
		for i, s := range strengths {
			e := meanAgainst(outputs[i], casReference(prepared, s))
			t.Logf("strength %.2f: mean difference %.3f/255 with casPeakP0 and casPeakP1", s, e)
			if e > 2 {
				t.Fatalf("strength %.2f: mean difference %.3f/255 with casPeakP0 and casPeakP1, want at most 2/255", s, e)
			}
		}
	}
	unchanged := 0.0
	for k, v := range input {
		unchanged += math.Abs(float64(outputs[0][k*3]) - math.Round(255*v))
	}
	if unchanged/float64(len(input)) < 0.1 {
		t.Fatalf("cas at strength 0 left the picture unchanged (mean %.3f/255)", unchanged/float64(len(input)))
	}
}

// gblur with four steps approximates a Gaussian with sigma = radius·H (blurSigma; H is the processed height, 360 here).
// The test logs and bounds the PSNR against a Go Gaussian, and checks that the documented sigma beats 0.6 and 1.6 times
// it.
func TestBlurApproximatesGaussian(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	chart := detailChart(640, 360)
	paths := map[string]string{"chart": filepath.Join(root, "detail.png")}
	writePicturePNG(t, paths["chart"], chart)
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "chart")
	project := chartProject(assets, 10, EffectInstance{ID: "blur", On: true, Params: map[string]any{"radius": 0.01}})
	if err := Validate(project); err != nil {
		t.Fatalf("blur project invalid: %v", err)
	}
	got := boxPlane(imageRGB(renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: 0, Output: filepath.Join(root, "blur.png")})))
	input := make([]float64, 640*360)
	for i := range input {
		input[i] = float64(chart.Pix[i*4])
	}
	sigma := blurSigma(0.01, 360)
	border := int(math.Ceil(4*sigma)) + 2
	score := func(s float64) float64 { return planePSNR(got, gaussianBlur(input, 640, 360, s), 640, 360, border) }
	exact, narrow, wide := score(sigma), score(0.6*sigma), score(1.6*sigma)
	t.Logf("gblur sigma %.3f px: PSNR %.2f dB against the Gaussian (%.2f dB at 0.6σ, %.2f dB at 1.6σ)", sigma, exact, narrow, wide)
	if exact < 30 || exact <= narrow || exact <= wide {
		t.Fatalf("gblur is not the documented Gaussian: %.2f dB (0.6σ %.2f dB, 1.6σ %.2f dB), want at least 30 dB and the best fit at σ", exact, narrow, wide)
	}
}

// renderClipRGBA runs one clip chain (clipChain.build) on its own, from one still input, and returns the first frame of
// the clip layer as RGBA before any composition. The layer's alpha is the clip's alpha after the transform.
func renderClipRGBA(t *testing.T, ctx context.Context, ffmpeg string, chain clipChain, path string) []byte {
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
	output := filepath.Join(root, "layer.rgba")
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error"}
	args = append(args, segmentInputArgs(inputs, chain.rate)...)
	args = append(args, "-filter_complex_script", graphPath, "-map", "[vs_video_out]", "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgba", output)
	if err := runFFmpegIn(ctx, workDir, ffmpeg, args, chain.rate.Seconds(1), nil, inputPaths(inputs), output); err != nil {
		t.Fatalf("render clip layer: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if want := chain.canvasWidth * chain.canvasHeight * 4; len(data) != want {
		t.Fatalf("layer has %d bytes, want %d", len(data), want)
	}
	return data
}

// ffmpegMajor is the major version of the FFmpeg binary, read from the first line of `ffmpeg -version`
// ("ffmpeg version 8.1.1-full_build…", "ffmpeg version 5.1.9-0+deb12u1…"). It is 0 for a build without a release number
// (a git build reports "N-<count>-g<hash>").
func ffmpegMajor(t *testing.T, ffmpeg string) int {
	t.Helper()
	out, err := exec.Command(ffmpeg, "-hide_banner", "-version").Output()
	if err != nil {
		t.Fatalf("ffmpeg -version: %v", err)
	}
	first, _, _ := strings.Cut(string(out), "\n")
	rest, ok := strings.CutPrefix(first, "ffmpeg version ")
	if !ok {
		return 0
	}
	digits := rest
	if i := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' }); i >= 0 {
		digits = rest[:i]
		if rest[i] != '.' {
			return 0
		}
	}
	major, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return major
}

// The vignette of a still keeps its alpha (alphaextract/alphamerge, index §4.10) and still darkens the opaque corners.
//   - FFmpeg 6 and later keep every alpha byte exactly.
//   - FFmpeg 5.1 (measured on 5.1.9) is allowed to differ by 1: its swscale adds 1 to the alpha values 128..254 in the
//     rgba to gbrap conversion that clipChain.effects inserts as format=gbrap in front of the stage (255 and 0 stay
//     exact; 8.1.1 is exact). That affects every gbrap effect on a still (blur, sharpen, lut as well as the vignette) and
//     cannot be avoided, because cas and gblur take no rgba input. The alpha of videos is not involved.
func TestRenderVignetteKeepsAlpha(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	alphaTolerance := 0
	major := ffmpegMajor(t, ffmpeg)
	if major > 0 && major < 6 {
		alphaTolerance = 1
	}
	t.Logf("FFmpeg major version %d, alpha tolerance %d", major, alphaTolerance)
	path := filepath.Join(t.TempDir(), "logo.png")
	logo := image.NewNRGBA(image.Rect(0, 0, 160, 96))
	for y := range 96 {
		for x := range 160 {
			alpha := uint8(255)
			switch {
			case x < 16:
				alpha = 0
			case x < 32:
				alpha = 128
			}
			logo.SetNRGBA(x, y, color.NRGBA{R: uint8(60 + x), G: uint8(80 + y), B: 140, A: alpha})
		}
	}
	writePicturePNG(t, path, logo)
	chain := func(effects ...EffectInstance) clipChain {
		clip := Clip{ID: "logo", AssetID: "logo", Duration: 5, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Effects: effects}
		return clipChain{item: layerClip{clip: clip, whole: clip, image: true, pictureWidth: 160, pictureHeight: 96}, canvasWidth: 160, canvasHeight: 96, rate: Rate{Num: 25, Den: 1}}
	}
	shaded := renderClipRGBA(t, ctx, ffmpeg, chain(EffectInstance{ID: "vignette", On: true, Params: map[string]any{"amount": 0.8, "size": 0.8}}), path)
	plain := renderClipRGBA(t, ctx, ffmpeg, chain(), path)
	for y := range 96 {
		for x := range 160 {
			if got, want := shaded[(y*160+x)*4+3], logo.NRGBAAt(x, y).A; math.Abs(float64(got)-float64(want)) > float64(alphaTolerance) {
				t.Fatalf("alpha at (%d, %d) = %d after the vignette, want %d (tolerance %d)", x, y, got, want, alphaTolerance)
			}
		}
	}
	corner, centre := (95*160+159)*4, (48*160+80)*4
	if shaded[corner] > 20 || plain[corner] < 200 || math.Abs(float64(shaded[centre])-float64(plain[centre])) > 2 {
		t.Fatalf("vignette red: corner %d (plain %d), centre %d (plain %d)", shaded[corner], plain[corner], shaded[centre], plain[centre])
	}
}
