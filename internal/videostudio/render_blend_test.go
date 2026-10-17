package videostudio

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"
)

// blendFormulaAlphas are the layer alphas of TestRenderBlendModesMatchFormula's three bands.
var blendFormulaAlphas = [3]uint8{255, 128, 51}

// blendLayerPicture is the layer of TestRenderBlendModesMatchFormula: three bands of rows with the alphas of
// blendFormulaAlphas.
//   - Down each band, red rises from 0 to 255 and green falls from 255 to 0, which reaches both ends of dodge and burn.
//   - Blue runs from 67 to 186 and crosses hardlight's threshold at 127/128.
//   - The chart's columns carry the values below, so each band meets 640 below values with 120 layer values per
//     channel.
func blendLayerPicture(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	band := height / 3
	for y := range height {
		row := y % band
		red := uint8(row * 255 / (band - 1))
		alpha := blendFormulaAlphas[min(y/band, 2)]
		for x := range width {
			img.SetNRGBA(x, y, color.NRGBA{R: red, G: 255 - red, B: uint8(67 + row*119/(band-1)), A: alpha})
		}
	}
	return img
}

// TestRenderBlendModesMatchFormula composites a layer over the 2c chart with each of the 13 modes through the export graph
// (RenderFrame). Every box pixel must equal the Go 8-bit model of index §4.22 at 0 levels:
// mix8(below, blend8(mode, below, layer), alpha) per channel, at the layer alphas 255, 128 and 51.
//   - blend works on gbrp, and the layer's alpha leaves through alphaextract. No rgba → gbrap step touches the alpha,
//     so FFmpeg 5.1 is exact too.
//   - Outside the box the layer is transparent and the canvas stays black.
func TestRenderBlendModesMatchFormula(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	root := t.TempDir()
	chart, layer := pictureChart(640, 360), blendLayerPicture(640, 360)
	paths := map[string]string{"chart": filepath.Join(root, "chart.png"), "layer": filepath.Join(root, "layer.png")}
	writePicturePNG(t, paths["chart"], chart)
	writePicturePNG(t, paths["layer"], layer)
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "chart", "layer")
	modes := blendModeIDs()
	if len(modes) != 13 {
		t.Fatalf("blend modes with an FFmpeg name = %v, want 13", modes)
	}
	for _, mode := range modes {
		project := chartProject(assets, 10)
		project.Tracks = append(project.Tracks, Track{ID: "top", Name: "Top", Kind: TrackOverlay, Clips: []Clip{{
			ID: "layer", AssetID: "layer", Duration: 10, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain, Blend: mode,
		}}})
		if err := Validate(project); err != nil {
			t.Fatalf("%s: project invalid: %v", mode, err)
		}
		img := renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: 0, Output: filepath.Join(root, fmt.Sprintf("blend-%s.png", mode))})
		wrong, worst, at := 0, 0, image.Point{}
		for y := chartBox.Min.Y; y < chartBox.Max.Y; y++ {
			for x := chartBox.Min.X; x < chartBox.Max.X; x++ {
				below, top := chart.NRGBAAt(x-chartBox.Min.X, y-chartBox.Min.Y), layer.NRGBAAt(x-chartBox.Min.X, y-chartBox.Min.Y)
				got := framePixel(img, x, y)
				for _, c := range [][3]uint8{{got.R, below.R, top.R}, {got.G, below.G, top.G}, {got.B, below.B, top.B}} {
					want := mix8(c[1], blend8(mode, c[1], c[2]), top.A)
					if d := absInt(int(c[0]) - int(want)); d > 0 {
						wrong++
						if d > worst {
							worst, at = d, image.Pt(x, y)
						}
					}
				}
			}
		}
		if wrong > 0 {
			t.Errorf("%s: %d channel values differ from mix8(blend8), largest %d at %v", mode, wrong, worst, at)
		} else {
			t.Logf("%s: every box channel equals mix8(blend8) at alpha 255, 128 and 51", mode)
		}
		if got := framePixel(img, 100, 100); got != (color.NRGBA{A: 255}) {
			t.Errorf("%s: pixel outside the box = %v, want black", mode, got)
		}
	}
}

// TestRenderBlendTransitionModes renders a multiply ⇒ multiply ⇒ screen track with two dissolves over a moving video
// and checks every frame (index §4.22, risk 18):
//   - frames 0–11: the multiply clips and their shared dissolve composite with multiply;
//   - frames 12–15: the dissolve into screen takes normal, because the two modes differ;
//   - frames 16–23: the screen clip composites with screen.
//
// Every clip is opaque and fills the canvas, so blend's output is the frame. The same project is also rendered with
// normal modes (the layer itself, dissolves included) and with the top track hidden (the video below). Every channel
// of the blended render must equal blend8 of those two, or the layer itself where the mode is normal, exactly.
func TestRenderBlendTransitionModes(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	root := t.TempDir()
	fixtures := map[string]string{
		"below": "testsrc=s=240x240:r=25:d=2",
		"one":   "testsrc2=s=240x240:r=25:d=2",
		"two":   "smptebars=s=240x240:r=25:d=2",
		"three": "rgbtestsrc=s=240x240:r=25:d=2",
	}
	project := Project{Version: CurrentVersion, Name: "Blend transitions", Width: 720, Height: 720, FPS: 25}
	rate := ProjectRate(project)
	paths := map[string]string{}
	for _, id := range []string{"below", "one", "two", "three"} {
		paths[id] = filepath.Join(root, id+".mp4")
		makeLavfiVideoFixture(t, ctx, ffmpeg, paths[id], fixtures[id])
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID = id
		project.Assets = append(project.Assets, AssetAtRate(asset, rate))
	}
	full := func(id, asset string, start, duration int, blend string, transition *Transition) Clip {
		return Clip{ID: id, AssetID: asset, Start: start, Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Blend: blend, Transition: transition}
	}
	dissolve := &Transition{Type: TransitionDissolve, Duration: 4}
	project.Tracks = []Track{
		{ID: "below", Name: "Below", Kind: TrackVideo, Clips: []Clip{full("bg", "below", 0, 24, "", nil)}},
		// A second video track: overlay tracks take images only, and the clips here are moving video.
		{ID: "top", Name: "Top", Kind: TrackVideo, Clips: []Clip{
			full("p1", "one", 0, 10, "multiply", dissolve),
			full("p2", "two", 6, 10, "multiply", dissolve),
			full("p3", "three", 12, 12, "screen", nil),
		}},
	}
	if err := Validate(project); err != nil {
		t.Fatalf("blend transition project invalid: %v", err)
	}
	variant := func(edit func(*Track)) Project {
		edited := project
		edited.Tracks = make([]Track, len(project.Tracks))
		for i, track := range project.Tracks {
			track.Clips = append([]Clip(nil), track.Clips...)
			edited.Tracks[i] = track
		}
		edit(&edited.Tracks[1])
		return edited
	}
	plain := variant(func(track *Track) {
		for i := range track.Clips {
			track.Clips[i].Blend = ""
		}
	})
	hidden := variant(func(track *Track) { track.Hidden = true })
	seg := Segment{Start: 0, End: 24}
	blended := renderRawRGB(t, ctx, ffmpeg, project, paths, seg)
	layer := renderRawRGB(t, ctx, ffmpeg, plain, paths, seg)
	below := renderRawRGB(t, ctx, ffmpeg, hidden, paths, seg)
	for k := range seg.End {
		mode := BlendNormal
		switch {
		case k < 12:
			mode = "multiply"
		case k >= 16:
			mode = "screen"
		}
		want := func(i int) uint8 {
			if mode == BlendNormal {
				return layer[k][i]
			}
			return blend8(mode, below[k][i], layer[k][i])
		}
		wrong, changed, first := 0, 0, -1
		for i, got := range blended[k] {
			if want(i) != layer[k][i] {
				changed++
			}
			if got != want(i) {
				if first < 0 {
					first = i
				}
				wrong++
			}
		}
		if wrong > 0 {
			t.Errorf("frame %d (%s): %d channel values differ; first at byte %d: got %d, want %d (below %d, layer %d)",
				k, mode, wrong, first, blended[k][first], want(first), below[k][first], layer[k][first])
		}
		if mode != BlendNormal && changed < len(blended[k])/10 {
			t.Errorf("frame %d (%s): the mode changes only %d of %d values, so the fixtures do not test it", k, mode, changed, len(blended[k]))
		}
	}
}
