package videostudio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFadeBlackMatchesFFmpeg measures the export's fade through black: a red and a blue still in a centred box on the
// upper track fade through black over a full-frame green still on the lower track. Inside the box the upper layer is
// opaque, so the centre pixel reads out·red + inc·blue directly; outside the box only the black share is left, and it
// darkens the green by exactly that weight when FFmpeg's black is opaque. fadeBlackWeights must agree within 2/255
// at every transition frame of two durations.
func TestFadeBlackMatchesFFmpeg(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"red": filepath.Join(root, "red.png"), "blue": filepath.Join(root, "blue.png"), "green": filepath.Join(root, "green.png")}
	writeFramePNG(t, paths["red"], 640, 360, color.NRGBA{R: 255, A: 255})
	writeFramePNG(t, paths["blue"], 640, 360, color.NRGBA{B: 255, A: 255})
	writeFramePNG(t, paths["green"], 1280, 720, color.NRGBA{G: 255, A: 255})
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "red", "blue", "green")
	centred := func(id string, start int, transition *Transition) Clip {
		return Clip{ID: id, AssetID: id, Start: start, Duration: 40, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain, Transition: transition}
	}
	const tolerance = 2.0 / 255
	var table strings.Builder
	failed := false
	for _, duration := range []int{20, 8} {
		start := 40 - duration
		project := Project{Version: CurrentVersion, Name: "Fade through black", Width: 1280, Height: 720, FPS: 25, Assets: assets,
			Tracks: []Track{
				{ID: "lower", Name: "Lower", Kind: TrackVideo, Clips: []Clip{
					{ID: "green", AssetID: "green", Duration: 80, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}}},
				{ID: "upper", Name: "Upper", Kind: TrackVideo, Clips: []Clip{
					centred("red", 0, &Transition{Type: TransitionBlack, Duration: duration}), centred("blue", start, nil)}},
			}}
		fmt.Fprintf(&table, "D=%d at 25 fps (p = i/D)\n", duration)
		for i := 0; i < duration; i++ {
			p := float64(i) / float64(duration)
			img := renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: start + i,
				Output: filepath.Join(root, fmt.Sprintf("black-%02d-%02d.png", duration, i))})
			centre, lower := framePixel(img, 640, 360), framePixel(img, 100, 100)
			out, inc := float64(centre.R)/255, float64(centre.B)/255
			dark := 1 - float64(lower.G)/255
			wantOut, wantInc, wantBlack := fadeBlackWeights(p)
			mark := ""
			if math.Abs(out-wantOut) > tolerance || math.Abs(inc-wantInc) > tolerance || math.Abs(dark-wantBlack) > tolerance ||
				centre.G > 2 || lower.R > 2 || lower.B > 2 {
				mark, failed = "  <-- differs", true
			}
			fmt.Fprintf(&table, "  i=%2d p=%.4f measured out=%.4f inc=%.4f black=%.4f lower-dark=%.4f centre-green=%3d | model out=%.4f inc=%.4f black=%.4f%s\n",
				i, p, out, inc, 1-out-inc, dark, centre.G, wantOut, wantInc, wantBlack, mark)
		}
	}
	if failed {
		t.Fatalf("fade through black differs from FFmpeg; fit fadeBlackWeights (effects.go) to the measured columns:\n%s", table.String())
	}
	t.Logf("fade through black matches FFmpeg:\n%s", table.String())
}

// TestTransitionParityWipesMatchFFmpeg checks the wipe mapping against the export: at p = 0.25 the incoming layer
// shows inside wipeEdge's band and the outgoing layer outside it.
func TestTransitionParityWipesMatchFFmpeg(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	paths := map[string]string{"red": filepath.Join(root, "red.png"), "blue": filepath.Join(root, "blue.png")}
	writeFramePNG(t, paths["red"], 640, 360, color.NRGBA{R: 255, A: 255})
	writeFramePNG(t, paths["blue"], 640, 360, color.NRGBA{B: 255, A: 255})
	assets := probeFrameAssets(t, ctx, ffmpeg, paths, "red", "blue")
	full := func(id string, start int, transition *Transition) Clip {
		return Clip{ID: id, AssetID: id, Start: start, Duration: 40, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: transition}
	}
	for _, kind := range []TransitionType{TransitionWipeLeft, TransitionWipeRight} {
		project := Project{Version: CurrentVersion, Name: "Wipe", Width: 1280, Height: 720, FPS: 25, Assets: assets,
			Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{
				full("red", 0, &Transition{Type: kind, Duration: 20}), full("blue", 20, nil)}}}}
		// Frame 25 is transition-local frame 5 of 20: p = 0.25.
		img := renderFrameImage(t, ctx, FrameOptions{FFmpegPath: ffmpeg, Project: project, AssetPaths: paths, Frame: 25, Output: filepath.Join(root, string(kind)+".png")})
		left, reveal, ok := wipeEdge(kind, 0.25, 1280)
		if !ok {
			t.Fatalf("wipeEdge(%s) reported no edge", kind)
		}
		inside, outside := int(left+reveal/2), int(left/2)
		if left == 0 {
			outside = int((reveal + 1280) / 2)
		}
		in, out := framePixel(img, inside, 360), framePixel(img, outside, 360)
		if in.B < 200 || in.R > 50 || out.R < 200 || out.B > 50 {
			t.Fatalf("%s at p=0.25: x=%d shows %v (want the incoming blue), x=%d shows %v (want the outgoing red)", kind, inside, in, outside, out)
		}
	}
}

type transitionParityInput struct {
	Kind  string  `json:"kind"` // "fade_black" or "wipe"
	P     float64 `json:"p"`
	Type  string  `json:"type,omitempty"`
	Width float64 `json:"width,omitempty"`
}

type transitionParityCase struct {
	Name  string                `json:"name"`
	Input transitionParityInput `json:"input"`
	Want  json.RawMessage       `json:"want"`
}

type fadeBlackWant struct {
	Out   float64 `json:"out"`
	Inc   float64 `json:"inc"`
	Black float64 `json:"black"`
}

type wipeEdgeWant struct {
	Left   float64 `json:"left"`
	Reveal float64 `json:"reveal"`
}

// The samples include both ease edges of the fitted curve (0.2 and 0.8) and a point inside the outgoing fade (0.1).
var transitionParityFadeSamples = []float64{0, 0.1, 0.125, 0.2, 0.25, 0.375, 0.5, 0.625, 0.75, 0.8, 0.875, 1, 1.5, -0.5}

var transitionParityWipeSamples = []transitionParityInput{
	{Kind: "wipe", P: 0.25, Type: "wipeleft", Width: 1280},
	{Kind: "wipe", P: 0.25, Type: "wiperight", Width: 1280},
	{Kind: "wipe", P: 0.5, Type: "wipeleft", Width: 640},
	{Kind: "wipe", P: 1, Type: "wiperight", Width: 640},
	{Kind: "wipe", P: 0.5, Type: "dissolve", Width: 1280},
	{Kind: "wipe", P: 0.5, Type: "black", Width: 1280},
}

// TestParityTransitions runs the shared fixture (phase 2 index §4.13) through Task 6's readParityCases; the JS side
// runs the same file in TestVideoStudioEffectsParity. AURAGO_UPDATE_PARITY=1 regenerates it from Go first (after a
// curve fit).
func TestParityTransitions(t *testing.T) {
	if os.Getenv("AURAGO_UPDATE_PARITY") == "1" {
		writeTransitionParity(t, filepath.Join("testdata", "parity", "transitions.json"))
	}
	for _, c := range readParityCases(t, "transitions") {
		var input transitionParityInput
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Errorf("%s: input %s: %v", c.Name, c.Input, err)
			continue
		}
		got, known := transitionParityResult(input)
		if !known {
			t.Errorf("%s: unknown input kind %q", c.Name, input.Kind)
			continue
		}
		var want map[string]float64
		if err := json.Unmarshal(c.Want, &want); err != nil {
			t.Errorf("%s: want %s: %v", c.Name, c.Want, err)
			continue
		}
		if (want == nil) != (got == nil) || len(want) != len(got) {
			t.Errorf("%s: got %v, want %s", c.Name, got, c.Want)
			continue
		}
		for key, value := range want {
			if actual, ok := got[key]; !ok || math.Abs(actual-value) > 1e-6 {
				t.Errorf("%s: %s = %v, want %v", c.Name, key, got[key], value)
			}
		}
	}
}

// transitionParityResult computes one fixture input with the Go implementation; nil stands for JSON null (no edge).
func transitionParityResult(input transitionParityInput) (map[string]float64, bool) {
	switch input.Kind {
	case "fade_black":
		out, inc, black := fadeBlackWeights(input.P)
		return map[string]float64{"out": out, "inc": inc, "black": black}, true
	case "wipe":
		left, reveal, ok := wipeEdge(TransitionType(input.Type), input.P, input.Width)
		if !ok {
			return nil, true
		}
		return map[string]float64{"left": left, "reveal": reveal}, true
	}
	return nil, false
}

// writeTransitionParity regenerates the fixture from the Go implementation, one case per line.
func writeTransitionParity(t *testing.T, path string) {
	t.Helper()
	var cases []transitionParityCase
	add := func(name string, input transitionParityInput, want any) {
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, transitionParityCase{Name: name, Input: input, Want: raw})
	}
	for _, p := range transitionParityFadeSamples {
		out, inc, black := fadeBlackWeights(p)
		add(fmt.Sprintf("fade_black p=%g", p), transitionParityInput{Kind: "fade_black", P: p}, fadeBlackWant{out, inc, black})
	}
	for _, input := range transitionParityWipeSamples {
		var want any
		if left, reveal, ok := wipeEdge(TransitionType(input.Type), input.P, input.Width); ok {
			want = wipeEdgeWant{left, reveal}
		}
		add(fmt.Sprintf("wipe %s p=%g w=%g", input.Type, input.P, input.Width), input, want)
	}
	var buf bytes.Buffer
	buf.WriteString("{\n  \"cases\": [\n")
	for i, c := range cases {
		line, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		buf.WriteString("    ")
		buf.Write(line)
		if i < len(cases)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]\n}\n")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
