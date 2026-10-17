package videostudio

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// chartBox is where chartProject shows its 640x360 chart, 1:1, on the 1280x720 canvas.
var chartBox = image.Rect(320, 180, 960, 540)

// pictureChart is the opaque chart of the 2c render tests. It has six 60-row bands of smooth ramps: grey, warm, cool,
// skin, a red–cyan cross ramp and a magenta–blue ramp. Together they reach every lutrgb entry and every matrix row.
func pictureChart(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	band := height / 6
	for y := range height {
		for x := range width {
			v := x * 255 / (width - 1)
			var c color.NRGBA
			switch min(y/band, 5) {
			case 0:
				c = color.NRGBA{R: uint8(v), G: uint8(v), B: uint8(v), A: 255}
			case 1:
				c = color.NRGBA{R: uint8(v), G: uint8(64 + v/4), B: 32, A: 255}
			case 2:
				c = color.NRGBA{R: 48, G: uint8(v), B: uint8(160 + v/4), A: 255}
			case 3:
				c = color.NRGBA{R: uint8(150 + v*90/255), G: uint8(100 + v*70/255), B: uint8(80 + v*60/255), A: 255}
			case 4:
				c = color.NRGBA{R: uint8(v), G: uint8(255 - v), B: 128, A: 255}
			default:
				c = color.NRGBA{R: uint8(255 - v), G: 40, B: uint8(v), A: 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// chartProject shows the asset "chart" (640x360) 1:1 in chartBox of a 1280x720 canvas with an effect stack.
func chartProject(assets []Asset, duration int, effects ...EffectInstance) Project {
	clip := Clip{ID: "pic", AssetID: "chart", Duration: duration, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain, Effects: effects}
	return Project{Version: CurrentVersion, Name: "Effects", Width: 1280, Height: 720, FPS: 25, Assets: assets,
		Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{clip}}}}
}

// imageRGB returns the rgb24 bytes of a decoded frame PNG.
func imageRGB(img image.Image) []byte {
	bounds := img.Bounds()
	out := make([]byte, 0, bounds.Dx()*bounds.Dy()*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := framePixel(img, x, y)
			out = append(out, c.R, c.G, c.B)
		}
	}
	return out
}

// checkBox compares the box of an rgb24 canvas frame with model applied to the chart pixel that each box pixel shows
// (the chart sits 1:1 in the box). It fails when a channel differs by more than tolerance code values.
func checkBox(t *testing.T, name string, frame []byte, stride int, box image.Rectangle, chart *image.NRGBA, tolerance float64, model func([3]uint8) [3]float64) {
	t.Helper()
	worst, at := 0.0, image.Point{}
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			source := chart.NRGBAAt(x-box.Min.X, y-box.Min.Y)
			want := model([3]uint8{source.R, source.G, source.B})
			i := (y*stride + x) * 3
			for ch := range 3 {
				if d := math.Abs(float64(frame[i+ch]) - want[ch]); d > worst {
					worst, at = d, image.Pt(x, y)
				}
			}
		}
	}
	if worst > tolerance {
		t.Fatalf("%s: largest difference %.2f/255 at %v, tolerance %.0f/255", name, worst, at, tolerance)
	}
	t.Logf("%s: largest difference %.2f/255", name, worst)
}

// colorModel8 is the export's 8-bit colour path for one pixel:
//   - lutrgb's round(255·curve(v/255)) per channel (channelCurveExpr equals it, TestChannelCurveExprMatchesApply);
//   - then the saturation matrix on those bytes, rounded and clamped.
//
// At saturation 1 the matrix is the identity, so the model applies it always.
func colorModel8(curves [3]ChannelCurve, matrix [9]float64, in [3]uint8) [3]float64 {
	var c [3]float64
	for ch := range c {
		c[ch] = math.Round(255 * curves[ch].apply(float64(in[ch])/255))
	}
	var out [3]float64
	for row := range out {
		v := matrix[row*3]*c[0] + matrix[row*3+1]*c[1] + matrix[row*3+2]*c[2]
		out[row] = math.Max(0, math.Min(255, math.Round(v)))
	}
	return out
}

// functionLUT samples f on a size³ grid over [0,1]³, red fastest.
func functionLUT(size int, f func(r, g, b float64) [3]float64) *LUT3D {
	table := &LUT3D{Size: size, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, size*size*size*3)}
	i := 0
	for b := range size {
		for g := range size {
			for r := range size {
				out := f(float64(r)/float64(size-1), float64(g)/float64(size-1), float64(b)/float64(size-1))
				table.Data[i], table.Data[i+1], table.Data[i+2] = float32(out[0]), float32(out[1]), float32(out[2])
				i += 3
			}
		}
	}
	return table
}

// writeLUTFile writes a table as a new .cube file (WriteCube).
func writeLUTFile(t *testing.T, path string, table *LUT3D) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCube(file, table); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

// lutModel8 is lut3d's trilinear output for one pixel, rounded to 8 bit, mixed with the input at intensity k.
func lutModel8(table *LUT3D, k float64, in [3]uint8) [3]float64 {
	mapped := table.Sample([3]float64{float64(in[0]) / 255, float64(in[1]) / 255, float64(in[2]) / 255})
	var out [3]float64
	for ch := range out {
		out[ch] = (1-k)*float64(in[ch]) + k*math.Round(255*math.Max(0, math.Min(1, mapped[ch])))
	}
	return out
}

// The colour effect through the export graph matches the Go 8-bit model of index §4.20 within 2/255:
//   - six static parameter sets, wheels included (RenderFrame);
//   - keyframed exposure and saturation in one process, so every checked frame was set by curve and matrix commands.
func TestRenderColorMatchesFormula(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	root := t.TempDir()
	chart := pictureChart(640, 360)
	paths := map[string]string{"chart": filepath.Join(root, "chart.png")}
	writePicturePNG(t, paths["chart"], chart)
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "chart")
	model := func(clip Clip, frame int) func([3]uint8) [3]float64 {
		curves := colorCurves(colorParamsAt(clip, 0, frame))
		matrix := saturationMatrix(effectValueAt(clip, 0, "saturation", frame))
		return func(in [3]uint8) [3]float64 { return colorModel8(curves, matrix, in) }
	}
	for i, params := range []map[string]any{
		{"exposure": 0.7, "contrast": 1.25},
		{"temperature": 3800.0, "tint": 0.35},
		{"saturation": 1.7},
		{"saturation": 0.35, "exposure": -0.6, "gamma": 1.4},
		{"lift_x": 0.5, "lift_y": -0.3, "lift": 0.04, "gamma_x": -0.4, "gamma_y": 0.2, "gamma": 0.9, "gain_x": 0.25, "gain_y": 0.6, "gain": 1.1},
		{"exposure": 0.3, "contrast": 0.85, "saturation": 1.25, "temperature": 9000.0, "tint": -0.2, "lift": -0.05, "lift_x": -0.2, "gamma": 0.75, "gain": 1.15, "gain_y": -0.3},
	} {
		project := chartProject(assets, 30, EffectInstance{ID: "color", On: true, Params: params})
		if err := Validate(project); err != nil {
			t.Fatalf("set %d invalid: %v", i, err)
		}
		img := renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: 0, Output: filepath.Join(root, fmt.Sprintf("color-%d.png", i))})
		checkBox(t, fmt.Sprintf("set %d %v", i, params), imageRGB(img), 1280, chartBox, chart, 2, model(project.Tracks[0].Clips[0], 0))
	}
	keyed := EffectInstance{ID: "color", On: true, Params: map[string]any{"contrast": 1.1},
		Keyframes: map[string][]Keyframe{"exposure": linearKeys(-0.8, 0.8, 20), "saturation": linearKeys(0.4, 1.6, 20)}}
	project := chartProject(assets, 30, keyed)
	if err := Validate(project); err != nil {
		t.Fatalf("keyframed colour invalid: %v", err)
	}
	frames := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 0, End: 21})
	for _, k := range []int{0, 5, 10, 15, 20} {
		checkBox(t, fmt.Sprintf("keyframed frame %d", k), frames[k], 1280, chartBox, chart, 2, model(project.Tracks[0].Clips[0], k))
	}
}

// The lut effect matches (1−k)·in + k·L(in) within 2/255 for:
//   - an uploaded 17³ table and the built-in teal_orange look, each at intensity 1 (lut3d alone) and 0.6 (blend opacity);
//   - a keyframed intensity, rendered in one process through blend all_expr in T.
func TestRenderLUTMatchesSample(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	root := t.TempDir()
	chart := pictureChart(640, 360)
	paths := map[string]string{"chart": filepath.Join(root, "chart.png"), "cube": filepath.Join(root, "fx.cube")}
	writePicturePNG(t, paths["chart"], chart)
	fixture := functionLUT(17, func(r, g, b float64) [3]float64 {
		return [3]float64{0.05 + 0.9*g*g, 1 - 0.7*r, 0.2 + 0.6*b*(0.5+0.5*r)}
	})
	writeLUTFile(t, paths["cube"], fixture)
	assets := append(probeFrameAssets(t, ctx, ffmpeg, paths, "chart"), Asset{ID: "cube", Name: "fx.cube", Kind: AssetLUT, LUTSize: 17})
	teal, ok := BuiltinLook("teal_orange")
	if !ok {
		t.Fatal("the built-in look teal_orange is missing")
	}
	model := func(table *LUT3D, k float64) func([3]uint8) [3]float64 {
		return func(in [3]uint8) [3]float64 { return lutModel8(table, k, in) }
	}
	for i, c := range []struct {
		ref       string
		table     *LUT3D
		intensity float64
	}{
		{"cube", fixture, 1}, {"cube", fixture, 0.6}, {"builtin:teal_orange", teal, 1}, {"builtin:teal_orange", teal, 0.6},
	} {
		project := chartProject(assets, 30, EffectInstance{ID: "lut", On: true, Params: map[string]any{"lut": c.ref, "intensity": c.intensity}})
		if err := Validate(project); err != nil {
			t.Fatalf("%s invalid: %v", c.ref, err)
		}
		img := renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: 0, Output: filepath.Join(root, fmt.Sprintf("lut-%d.png", i))})
		checkBox(t, fmt.Sprintf("%s at %.1f", c.ref, c.intensity), imageRGB(img), 1280, chartBox, chart, 2, model(c.table, c.intensity))
	}
	keyed := EffectInstance{ID: "lut", On: true, Params: map[string]any{"lut": "cube"}, Keyframes: map[string][]Keyframe{"intensity": linearKeys(0, 1, 20)}}
	project := chartProject(assets, 30, keyed)
	if err := Validate(project); err != nil {
		t.Fatalf("keyframed lut invalid: %v", err)
	}
	frames := renderRawRGB(t, ctx, ffmpeg, project, paths, Segment{Start: 0, End: 21})
	for _, k := range []int{0, 5, 10, 15, 20} {
		intensity := effectValueAt(project.Tracks[0].Clips[0], 0, "intensity", k)
		checkBox(t, fmt.Sprintf("keyframed frame %d (k %.2f)", k, intensity), frames[k], 1280, chartBox, chart, 2, model(fixture, intensity))
	}
}

// The largest grid the parser accepts (MaxLUTGrid, 65) must load in FFmpeg's lut3d as well, on 8.1.1 and in the 5.1
// container (decision A4).
func TestRenderLUTMaxGrid(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	chart := pictureChart(640, 360)
	paths := map[string]string{"chart": filepath.Join(root, "chart.png"), "cube": filepath.Join(root, "max.cube")}
	writePicturePNG(t, paths["chart"], chart)
	big := functionLUT(MaxLUTGrid, func(r, g, b float64) [3]float64 { return [3]float64{math.Sqrt(r), 1 - g, 0.25 + 0.5*b} })
	writeLUTFile(t, paths["cube"], big)
	assets := append(probeFrameAssets(t, ctx, ffmpeg, paths, "chart"), Asset{ID: "cube", Name: "max.cube", Kind: AssetLUT, LUTSize: MaxLUTGrid})
	project := chartProject(assets, 10, EffectInstance{ID: "lut", On: true, Params: map[string]any{"lut": "cube", "intensity": 1.0}})
	if err := Validate(project); err != nil {
		t.Fatalf("max-grid project invalid: %v", err)
	}
	img := renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: 0, Output: filepath.Join(root, "max.png")})
	checkBox(t, "65-point LUT", imageRGB(img), 1280, chartBox, chart, 2, func(in [3]uint8) [3]float64 { return lutModel8(big, 1, in) })
}
