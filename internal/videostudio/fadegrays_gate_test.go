package videostudio

import (
	"bytes"
	"context"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// spikePair is the spike's structured pair SA/SB (p2espike head.sh) as gbrp frames, plane order g, b, r:
//   - A: a horizontal gradient with a 16 px grid in g, and two hash planes;
//   - B: a vertical gradient with a 10×24 grid in g, and two other hashes.
//
// geq rounds like C round(), half away from zero: math.Round.
func spikePair(width, height int) (a, b []byte) {
	n := width * height
	a, b = make([]byte, 3*n), make([]byte, 3*n)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			ga := math.Round(255 * float64(x) / float64(width-1))
			if x%16 < 1 || y%16 < 1 {
				ga = 0
			}
			a[i], a[n+i], a[2*n+i] = byte(ga), byte((x*7+y*13)%256), byte((x*3+y*5)%256)
			gb := math.Round(255 * float64(y) / float64(height-1))
			if (x+3)%10 < 1 || (y+4)%24 < 1 {
				gb = 255
			}
			b[i], b[n+i], b[2*n+i] = byte(gb), byte((x*11+y*5)%256), byte(255-(x*5+y*3)%256)
		}
	}
	return a, b
}

// fadegraysModel against the spike of 2026-10-18 (models.js EXACT, analysis-8.txt, index §4.24). FFmpeg 8.1.1 renders
// it bit-exact.
//   - The flat white → black pair at 320×180, D = 25: every plane of frame i holds the measured fade value
//     trunc(f(255·P)). The white mixes with its own grey (255) and the black with its own (0).
//   - The spike's structured pair at 320×180, D = 25: every byte equals the normative rule written out per pixel, with
//     the grey of the outgoing pixel in the first inner mix and the grey of the incoming pixel in the second. Frame 0
//     is the outgoing picture.
//   - The self-test case: the pinned lavfi sources and D frames of 3·W·H bytes. A malformed input gives nothing.
func TestFadegraysModelMatchesSpike(t *testing.T) {
	const W, H, D = 320, 180, 25
	n := W * H
	measured := []int{255, 244, 234, 224, 214, 204, 193, 183, 173, 163, 153, 142, 132, 122, 112, 101, 91, 81, 71, 61, 50, 40, 30, 20, 10}
	frames := fadegraysModel(bytes.Repeat([]byte{255}, 3*n), make([]byte, 3*n), W, H, D)
	if len(frames) != D {
		t.Fatalf("fadegraysModel returned %d frames, want %d", len(frames), D)
	}
	for i, frame := range frames {
		for j, value := range frame {
			if int(value) != measured[i] {
				t.Fatalf("flat white → black, frame %d, byte %d = %d, want the measured %d", i, j, value, measured[i])
			}
		}
	}
	a, b := spikePair(W, H)
	frames = fadegraysModel(a, b, W, H, D)
	if !bytes.Equal(frames[0], a) {
		t.Fatal("frame 0 must be the outgoing picture (P = 1)")
	}
	for _, i := range []int{1, 6, 12, 13, 18, 24} {
		f := transitionFrameAt("fadegrays", i, D, W, H)
		for k := 0; k < n; k++ {
			gA := float32((int(a[k]) + int(a[n+k]) + int(a[2*n+k])) / 3)
			gB := float32((int(b[k]) + int(b[n+k]) + int(b[2*n+k])) / 3)
			for p := 0; p < 3; p++ {
				want := uint8(mix32(mix32(float32(a[p*n+k]), gA, f.U), mix32(gB, float32(b[p*n+k]), f.V), f.P))
				if got := frames[i][p*n+k]; got != want {
					t.Fatalf("structured pair, frame %d, plane %d, pixel %d = %d, want %d", i, p, k, got, want)
				}
			}
		}
	}
	sourceA, sourceB, want := fadegraysSelfTestCase()
	if sourceA != "nullsrc=s=64x36:r=25:d=0.2,format=gbrp,geq=g='mod(3*X+5*Y+0,256)':b='mod(7*X+13*Y+40,256)':r='mod(11*X+2*Y+90,256)'" ||
		sourceB != "nullsrc=s=64x36:r=25:d=0.2,format=gbrp,geq=g='mod(5*X+3*Y+128,256)':b='mod(13*X+7*Y+7,256)':r='mod(2*X+11*Y+200,256)'" {
		t.Fatalf("self-test sources:\n%s\n%s", sourceA, sourceB)
	}
	if len(want) != 5*3*64*36 || fadegraysSelfTestFilter != "[0:v][1:v]xfade=transition=fadegrays:duration=0.2:offset=0" {
		t.Fatalf("self-test output %d bytes with %q; want 34560 bytes and duration 0.2 (D = 5 at 25 fps)", len(want), fadegraysSelfTestFilter)
	}
	if fadegraysModel(a[:10], b, W, H, D) != nil || fadegraysModel(a, b, W, H, 0) != nil || fadegraysModel(a, b, 0, H, D) != nil {
		t.Fatal("fadegraysModel must return nil for a malformed input")
	}
}

// fakeFadegraysRenders is fakeCapabilityFFmpeg with every capability filter and every registry transition listed. The
// n-th fadegrays self-test render (0-based) returns render(n). It returns the fake binary and the number of renders so
// far.
func fakeFadegraysRenders(t *testing.T, render func(n int) ([]byte, error)) (string, *int) {
	t.Helper()
	path, _ := fakeCapabilityFFmpeg(t, filterListing(append(capabilityFilters(), "xfade")...), xfadeHelp(registryXFadeNames("")...))
	renders := 0
	fake := capabilityCommand
	capabilityCommand = func(ctx context.Context, ffmpeg string, args ...string) ([]byte, error) {
		if !slices.Contains(args, fadegraysSelfTestFilter) {
			return fake(ctx, ffmpeg, args...)
		}
		renders++
		return render(renders - 1)
	}
	t.Cleanup(func() { capabilityCommand = fake })
	return path, &renders
}

// The fadegrays gate inside the probe (plan 2e D2, index §4.3), with a fake FFmpeg:
//   - matching runs keep fadegrays;
//   - a wrong byte, two runs that differ, a short output or a failed render mark xfade:fadegrays missing;
//   - the result is memoized with the probe (at most one pair of renders per binary);
//   - a probe whose context ends during the self-test is an error that is not memoized.
func TestProbeCapabilitiesFadegraysSelfTest(t *testing.T) {
	ctx := context.Background()
	_, _, want := fadegraysSelfTestCase()
	wrong := slices.Clone(want)
	wrong[len(wrong)/2] ^= 1
	for _, c := range []struct {
		name      string
		runs      [][]byte
		fail      bool
		available bool
		renders   int
	}{
		{"both runs match the model", [][]byte{want, want}, false, true, 2},
		{"a byte differs from the model", [][]byte{wrong, wrong}, false, false, 1},
		{"the second run differs from the first", [][]byte{want, wrong}, false, false, 2},
		{"a short output", [][]byte{want[:len(want)-1]}, false, false, 1},
		{"the render fails", nil, true, false, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			path, renders := fakeFadegraysRenders(t, func(n int) ([]byte, error) {
				if c.fail {
					return nil, errors.New("exit status 1: Unknown input format: 'lavfi'")
				}
				return c.runs[min(n, len(c.runs)-1)], nil
			})
			missing, err := ProbeCapabilities(ctx, path)
			if err != nil {
				t.Fatalf("ProbeCapabilities: %v", err)
			}
			wantMissing := []string{}
			if !c.available {
				wantMissing = []string{"xfade:fadegrays"}
			}
			if !slices.Equal(missing, wantMissing) || *renders != c.renders {
				t.Fatalf("missing = %v after %d renders; want %v after %d", missing, *renders, wantMissing, c.renders)
			}
			if again, err := ProbeCapabilities(ctx, path); err != nil || !slices.Equal(again, wantMissing) || *renders != c.renders {
				t.Fatalf("second probe = %v, %v after %d renders; want the memoized result", again, err, *renders)
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	defer cancel()
	path, renders := fakeFadegraysRenders(t, func(n int) ([]byte, error) {
		if n == 0 {
			cancel()
			return nil, context.Canceled
		}
		return want, nil
	})
	if _, err := ProbeCapabilities(cancelled, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("a probe cancelled during the self-test = %v, want context.Canceled", err)
	}
	if missing, err := ProbeCapabilities(ctx, path); err != nil || len(missing) != 0 || *renders != 3 {
		t.Fatalf("the probe after the cancelled one = %v, %v after %d renders; want nothing missing after 3", missing, err, *renders)
	}
}

// TestFadegraysSelfTestOnInstalledFFmpeg runs the self-test on the installed FFmpeg (FFmpeg-gated; index §6 container
// check).
//   - The spike measured 5.1.9's fadegrays as wrong and thread-dependent, and 8.1.1's as bit-exact.
//   - Major 5 must report unavailable and major 8 or later available; others are logged.
//   - ProbeCapabilities must agree.
//
// If FFmpeg 5 passes, stop and report (plan 2e Task 3, open question 8): the probe then grows to 320×180, the
// spike's failing size.
func TestFadegraysSelfTestOnInstalledFFmpeg(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), capabilityProbeTimeout)
	defer cancel()
	ok, err := fadegraysSelfTest(ctx, ffmpeg)
	if err != nil {
		t.Fatalf("fadegraysSelfTest: %v", err)
	}
	major := ffmpegMajor(t, ffmpeg)
	t.Logf("fadegrays self-test on FFmpeg major %d: available=%t", major, ok)
	switch {
	case major == 5 && ok:
		t.Fatal("FFmpeg 5 passed the fadegrays self-test, but the spike measured its fadegrays wrong and thread-dependent. Stop and report: the probe size then grows to 320x180")
	case major >= 8 && !ok:
		t.Fatalf("FFmpeg %d failed the fadegrays self-test; the spike measured 8.1.1 bit-exact against fadegraysModel", major)
	}
	resetCapabilityMemo()
	t.Cleanup(resetCapabilityMemo)
	missing, err := ProbeCapabilities(context.Background(), ffmpeg)
	if err != nil {
		t.Fatalf("ProbeCapabilities: %v", err)
	}
	if slices.Contains(missing, "xfade:fadegrays") == ok {
		t.Fatalf("ProbeCapabilities = %v disagrees with the self-test (available=%t)", missing, ok)
	}
	if !strings.Contains(strings.Join(missing, ","), "xfade:fadegrays") && !ok {
		t.Fatal("a failed self-test must report xfade:fadegrays")
	}
}

// fadegraysRenderOptions is a render of capabilityTestProject(kind) that never reaches a real source: the asset path
// does not exist.
func fadegraysRenderOptions(t *testing.T, ffmpeg string, project Project) RenderOptions {
	t.Helper()
	return RenderOptions{
		FFmpegPath: ffmpeg,
		Project:    project,
		AssetPaths: map[string]string{"video": filepath.Join(t.TempDir(), "never-read.mp4")},
		Output:     filepath.Join(t.TempDir(), "out.mp4"),
	}
}

// The fadegrays gate must not fail open (2e review I-2, index §4.3). FFmpeg 5.1 renders fadegrays wrongly without an
// error, so a probe that ends before the self-test finishes (a loaded host, a short render deadline) cannot let the
// export through:
//   - a render deadline that ends inside the self-test refuses a project with an effective fadegrays transition with an
//     error that wraps the probe error and is a *CapabilityError (ErrFeatureUnavailable);
//   - a probe that fails for another reason does the same, with a live context;
//   - every other project keeps today's behaviour: a dissolve project, and fadegrays that does not play (disabled
//     incoming clip, hidden track), go on past the gate.
func TestRenderFadegraysFailsClosedWhenTheProbeEnds(t *testing.T) {
	fadegrays := TransitionType("fadegrays")
	notPlaying := map[string]func(*Project){
		"a disabled incoming clip": func(p *Project) { p.Tracks[0].Clips[1].Disabled = true },
		"a hidden track":           func(p *Project) { p.Tracks[0].Hidden = true },
	}
	probeFailure := errors.New("ffmpeg crashed while listing filters")
	scenarios := []struct {
		name string
		// live is true when the render context is still alive after the probe failed, so a project that passes the gate
		// goes on to its sources.
		live bool
		// setup installs the fake FFmpeg and returns it with the render context and the probe error to find.
		setup func(t *testing.T) (ffmpeg string, ctx context.Context, probe error)
	}{
		{"the self-test outlasts the render deadline", false, func(t *testing.T) (string, context.Context, error) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			t.Cleanup(cancel)
			path, _ := fakeFadegraysRenders(t, func(int) ([]byte, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			})
			return path, ctx, context.DeadlineExceeded
		}},
		{"the filter listing fails", true, func(t *testing.T) (string, context.Context, error) {
			path, _ := fakeFadegraysRenders(t, func(int) ([]byte, error) {
				_, _, want := fadegraysSelfTestCase()
				return want, nil
			})
			fake := capabilityCommand
			capabilityCommand = func(ctx context.Context, ffmpeg string, args ...string) ([]byte, error) {
				if slices.Contains(args, "-filters") {
					return nil, probeFailure
				}
				return fake(ctx, ffmpeg, args...)
			}
			t.Cleanup(func() { capabilityCommand = fake })
			return path, context.Background(), probeFailure
		}},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			ffmpeg, ctx, probe := scenario.setup(t)
			err := RenderWithOptions(ctx, fadegraysRenderOptions(t, ffmpeg, capabilityTestProject(fadegrays)))
			var capability *CapabilityError
			if err == nil || !errors.Is(err, probe) || !errors.Is(err, ErrFeatureUnavailable) || !errors.As(err, &capability) ||
				capability.Feature != "transition:fadegrays" || capability.Missing != "xfade:fadegrays" {
				t.Fatalf("a fadegrays render with a failed probe = %v; want the probe error %v wrapped in a transition:fadegrays CapabilityError", err, probe)
			}

			// The same failed probe does not touch any other project.
			others := map[string]Project{"a dissolve project": capabilityTestProject(TransitionDissolve)}
			for name, edit := range notPlaying {
				project := capabilityTestProject(fadegrays)
				edit(&project)
				others["fadegrays that does not play: "+name] = project
			}
			for name, project := range others {
				err := RenderWithOptions(ctx, fadegraysRenderOptions(t, ffmpeg, project))
				if errors.Is(err, ErrFeatureUnavailable) {
					t.Fatalf("%s: refused by the capability gate: %v", name, err)
				}
				// Past the gate the fake binary and the missing source are what fail: the staging check, or the first
				// segment (a hidden track renders black without a source).
				if scenario.live && (err == nil || errors.Is(err, probe) ||
					!strings.Contains(err.Error(), "not a private staged file") && !strings.Contains(err.Error(), "start FFmpeg")) {
					t.Fatalf("%s: = %v; want it to go on past the gate and fail on the fake binary or the missing source", name, err)
				}
			}
		})
	}
}

// CheckRuntime reports an unverifiable fadegrays as missing (2e review I-2): a probe context that ends inside the
// self-test keeps the runtime ready, greys exactly fadegrays through Registry.Unavailable, and does not hide the probe
// error. The next status with a live context reports the real result (the failed probe is not memoized). FFmpeg-gated.
func TestCheckRuntimeReportsFadegraysMissingWhenTheProbeEnds(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	resetCapabilityMemo()
	t.Cleanup(resetCapabilityMemo)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	real := capabilityCommand
	capabilityCommand = func(ctx context.Context, binary string, args ...string) ([]byte, error) {
		if slices.Contains(args, fadegraysSelfTestFilter) {
			cancel()
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return real(ctx, binary, args...)
	}
	t.Cleanup(func() { capabilityCommand = real })

	status := CheckRuntime(ctx, ffmpeg)
	if !status.Ready || !slices.Equal(status.Missing, []string{"xfade:fadegrays"}) || !strings.Contains(status.ProbeError, "fadegrays self-test") ||
		!strings.Contains(status.ProbeError, context.Canceled.Error()) {
		t.Fatalf("CheckRuntime() with an ending probe = %+v; want a ready runtime, Missing [xfade:fadegrays] and the probe error", status)
	}
	if set := EffectRegistry().Unavailable(status.Missing); !slices.Equal(set.Transitions, []string{"fadegrays"}) ||
		len(set.Effects)+len(set.BlendModes)+len(set.AnimPresets) != 0 {
		t.Fatalf("Unavailable(%v) = %+v; want exactly the fadegrays transition", status.Missing, set)
	}

	capabilityCommand = real
	want, err := ProbeCapabilities(context.Background(), ffmpeg)
	if err != nil {
		t.Fatalf("ProbeCapabilities: %v", err)
	}
	if again := CheckRuntime(context.Background(), ffmpeg); again.ProbeError != "" || !again.Ready || !slices.Equal(again.Missing, want) {
		t.Fatalf("CheckRuntime() after the ended probe = %+v; want Missing %v and no probe error", again, want)
	}
}
