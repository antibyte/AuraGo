package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/videostudio"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
)

// The 2f preview benchmark (index §4.26.6, spec §6.5, spike S3). It reports and never asserts.

const (
	// videoStudioBenchSourceFPS is the frame rate of the benchmark sources and of the project.
	videoStudioBenchSourceFPS = 30
	// videoStudioBenchEffectCap is VideoStudioPicture.EFFECT_CAP_LONG (plan 2f Task 11): the effectCap option of every
	// quality except Full, which passes 0. The preview ignores the option before Task 11.
	videoStudioBenchEffectCap = 1280
)

// videoStudioBenchEffect is one effect of a benchmark layer: a registry id and the values that differ from its defaults.
type videoStudioBenchEffect struct {
	id     string
	values map[string]any
}

// videoStudioBenchLayerTable is the spike's M1 project (p2fspike js2/lib.js and gen_media2.sh), with the same values as
// the export benchmark's effectsBenchLayers (index §4.26.6). Per layer: the lavfi source and its filter, the blend mode
// of the mixed scenario (layer 1 stays normal), and two effects in stack order. Numbers are float64, as Validate wants.
var videoStudioBenchLayerTable = []struct {
	source, filter, blend string
	effects               []videoStudioBenchEffect
}{
	{source: "testsrc2=size=1920x1080:rate=30", filter: "noise=alls=3:allf=t,format=yuv420p", effects: []videoStudioBenchEffect{
		{"color", map[string]any{"exposure": 0.15, "contrast": 1.1, "saturation": 1.2, "temperature": 6000.0}},
		{"blur", map[string]any{"radius": 0.01}},
	}},
	{source: "mandelbrot=size=1920x1080:rate=30:maxiter=400", filter: "noise=alls=2:allf=t,format=yuv420p", blend: "overlay", effects: []videoStudioBenchEffect{
		{"lut", map[string]any{"lut": "builtin:teal_orange", "intensity": 0.8}},
		{"vignette", map[string]any{"amount": 0.5, "size": 1.0}},
	}},
	{source: "gradients=size=1920x1080:rate=30:speed=0.04:nb_colors=5", filter: "noise=alls=4:allf=t,format=yuv420p", blend: "softlight", effects: []videoStudioBenchEffect{
		{"sharpen", map[string]any{"strength": 0.6}},
		{"color", map[string]any{"saturation": 1.15, "contrast": 1.05}},
	}},
	{source: "smptehdbars=size=1920x1080:rate=30", filter: "hue=h=t*18:s=1.2,noise=alls=3:allf=t,format=yuv420p", blend: "screen", effects: []videoStudioBenchEffect{
		{"chroma_key", map[string]any{"color": "#00b140", "similarity": 0.25, "smoothness": 0.1, "spill": 0.5}},
		{"vignette", map[string]any{"amount": 0.3, "size": 1.2}},
	}},
}

// videoStudioBenchLauncher is one browser setup of the benchmark (index §4.26.6).
type videoStudioBenchLauncher struct {
	name     string   // gpu or swiftshader
	bin      string   // the browser binary
	explicit bool     // bin came from AURAGO_VS_BENCH_BROWSER: a launch failure fails instead of skipping
	flags    []string // Chrome switches, "name" or "name=value"
}

// videoStudioBenchLaunchers returns the launchers that AURAGO_VS_BENCH_LAUNCHERS names (default "gpu,swiftshader").
//   - Both add the autoplay policy and an unthrottled requestAnimationFrame (disable-gpu-vsync and
//     disable-frame-rate-limit, the spike's -unthrottled). Headless Chrome has no vsync and otherwise fires frames at
//     about 32 Hz on Windows; the page paces them to 60 Hz itself (videoStudioBenchJS).
//   - gpu: ignore-gpu-blocklist plus AURAGO_VS_BENCH_GPU_FLAGS (space separated; force_low_power_gpu selects the local
//     iGPU, and the Renoir container passes use-gl=angle use-angle=gl-egl disable-gpu-sandbox ozone-platform=headless).
//   - swiftshader: newWebGLSmokeBrowser's switches.
//
// AURAGO_VS_BENCH_BROWSER names the binary. Otherwise browserExecutable (Windows paths only) finds Chrome or Edge, and
// the test skips without one.
func videoStudioBenchLaunchers(t *testing.T) []videoStudioBenchLauncher {
	t.Helper()
	bin, explicit := strings.TrimSpace(os.Getenv("AURAGO_VS_BENCH_BROWSER")), true
	if bin == "" {
		found, ok := browserExecutable()
		if !ok {
			t.Skip("the preview benchmark needs Chrome or Edge, or AURAGO_VS_BENCH_BROWSER")
		}
		bin, explicit = found, false
	} else if _, err := os.Stat(bin); err != nil {
		t.Fatalf("AURAGO_VS_BENCH_BROWSER=%s: %v", bin, err)
	}
	common := []string{"autoplay-policy=no-user-gesture-required", "disable-gpu-vsync", "disable-frame-rate-limit"}
	names := strings.TrimSpace(os.Getenv("AURAGO_VS_BENCH_LAUNCHERS"))
	if names == "" {
		names = "gpu,swiftshader"
	}
	var out []videoStudioBenchLauncher
	for _, name := range strings.Split(names, ",") {
		switch name = strings.TrimSpace(name); name {
		case "":
		case "gpu":
			switches := slices.Concat(common, []string{"ignore-gpu-blocklist"}, strings.Fields(os.Getenv("AURAGO_VS_BENCH_GPU_FLAGS")))
			out = append(out, videoStudioBenchLauncher{name: name, bin: bin, explicit: explicit, flags: switches})
		case "swiftshader":
			switches := slices.Concat(common, []string{"use-angle=swiftshader", "enable-unsafe-swiftshader"})
			out = append(out, videoStudioBenchLauncher{name: name, bin: bin, explicit: explicit, flags: switches})
		default:
			t.Fatalf("AURAGO_VS_BENCH_LAUNCHERS: unknown launcher %q (gpu, swiftshader)", name)
		}
	}
	if len(out) == 0 {
		t.Fatalf("AURAGO_VS_BENCH_LAUNCHERS=%q names no launcher", names)
	}
	return out
}

// videoStudioBenchBrowser starts the launcher's browser with the smoke launcher's base switches (headless, no-sandbox,
// disable-dev-shm-usage, no disable-gpu) under a context of up to 55 minutes. newSmokeBrowserWith's 180 s context would
// kill a benchmark run. The cleanup closes the browser and removes its rod user-data directory.
func videoStudioBenchBrowser(t *testing.T, l videoStudioBenchLauncher) *rod.Browser {
	t.Helper()
	deadline := time.Now().Add(55 * time.Minute)
	if testDeadline, ok := t.Deadline(); ok && testDeadline.Before(deadline) {
		deadline = testDeadline
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	launch := launcher.New().Context(ctx).Bin(l.bin).Headless(true).NoSandbox(true).Set("disable-dev-shm-usage")
	launch.Delete("disable-gpu")
	if os.Getenv("AURAGO_BROWSER_MUTE_AUDIO") == "1" {
		launch.Set("mute-audio")
	}
	for _, raw := range l.flags {
		name, value, hasValue := strings.Cut(strings.TrimLeft(raw, "-"), "=")
		if hasValue {
			launch.Set(flags.Flag(name), value)
		} else {
			launch.Set(flags.Flag(name))
		}
	}
	controlURL, err := launch.Launch()
	if err != nil {
		cancel()
		if l.explicit {
			t.Fatalf("the %s launcher could not start AURAGO_VS_BENCH_BROWSER=%s: %v", l.name, l.bin, err)
		}
		t.Skipf("the %s launcher could not start %s: %v", l.name, l.bin, err)
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		launch.Kill()
		launch.Cleanup()
		cancel()
		t.Fatalf("connect the %s browser: %v", l.name, err)
	}
	t.Cleanup(func() {
		_ = browser.Close()
		launch.Kill()
		launch.Cleanup()
		cancel()
	})
	return browser
}

// videoStudioBenchScenario is one row of the §4.26.6 scenario table.
type videoStudioBenchScenario struct {
	name    string
	painter string  // gl or 2d
	quality string  // auto, full or half; effectCap is 0 at full, else videoStudioBenchEffectCap
	scale   float64 // the backing scale that previewScale gives a 1920 x 1080 project at that quality
	proxy   bool    // the 540p proxies instead of the originals
	effects bool    // the M1 effects (false: the same layers without effects)
	blends  bool    // layers 2-4 in overlay, softlight and screen
	scopes  bool    // the scopes panel's read and analysis in onPaint (§4.26.2)
}

// videoStudioBenchScenarios returns the eight scenarios of §4.26.6 in table order, or the comma list in
// AURAGO_VS_BENCH_SCENARIOS, in its order. fx-gl-exact is fx-gl-full at Full quality: the interleaved partner for plan
// Task 11's keep rule. The two are identical before Task 11.
func videoStudioBenchScenarios(t *testing.T) []videoStudioBenchScenario {
	t.Helper()
	all := []videoStudioBenchScenario{
		{name: "fx-gl-full", painter: "gl", quality: "auto", scale: 1, effects: true},
		{name: "fx-gl-exact", painter: "gl", quality: "full", scale: 1, effects: true},
		{name: "plain-gl-full", painter: "gl", quality: "auto", scale: 1},
		{name: "fx-gl-half", painter: "gl", quality: "half", scale: 0.5, effects: true},
		{name: "fx-gl-proxy", painter: "gl", quality: "auto", scale: 1, proxy: true, effects: true},
		{name: "fx-gl-mixed", painter: "gl", quality: "auto", scale: 1, effects: true, blends: true},
		{name: "fx-gl-scopes", painter: "gl", quality: "auto", scale: 1, effects: true, scopes: true},
		{name: "fx-2d-full", painter: "2d", quality: "auto", scale: 1, effects: true},
	}
	raw := strings.TrimSpace(os.Getenv("AURAGO_VS_BENCH_SCENARIOS"))
	if raw == "" {
		return all
	}
	var picked []videoStudioBenchScenario
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		index := slices.IndexFunc(all, func(s videoStudioBenchScenario) bool { return s.name == name })
		if index < 0 {
			t.Fatalf("AURAGO_VS_BENCH_SCENARIOS: unknown scenario %q", name)
		}
		picked = append(picked, all[index])
	}
	if len(picked) == 0 {
		t.Fatalf("AURAGO_VS_BENCH_SCENARIOS=%q names no scenario", raw)
	}
	return picked
}

// videoStudioBenchSources are the four 20 s sources of the M1 project and their 540p proxies, in one directory.
type videoStudioBenchSources struct {
	dir     string
	assets  []videostudio.Asset // as1..as4, probed, at 30 fps
	files   map[string]string   // asset id -> original file name under /media/
	proxies map[string]string   // asset id -> proxy file name under /media/
}

// videoStudioBenchMedia writes the sources with the spike's recipe (gen_media2.sh), then probes them:
//   - originals: libx264 medium, High profile, level 4.1, 12 Mbps with a 16 Mbps / 24 Mbit VBV, GOP 30, yuv420p, no
//     audio, faststart, 20 s;
//   - proxies: scale=960:540 bicubic, crf 24, High, GOP 30.
func videoStudioBenchMedia(t *testing.T, ctx context.Context, ffmpeg, dir string) videoStudioBenchSources {
	t.Helper()
	original := []string{"-c:v", "libx264", "-preset", "medium", "-b:v", "12M", "-maxrate", "16M", "-bufsize", "24M",
		"-pix_fmt", "yuv420p", "-profile:v", "high", "-level", "4.1", "-g", "30", "-an", "-movflags", "+faststart"}
	proxy := []string{"-c:v", "libx264", "-preset", "medium", "-crf", "24", "-pix_fmt", "yuv420p", "-profile:v", "high",
		"-g", "30", "-an", "-movflags", "+faststart"}
	encode := func(args ...string) {
		t.Helper()
		full := slices.Concat([]string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y"}, args)
		if out, err := exec.CommandContext(ctx, ffmpeg, full...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	started := time.Now()
	sources := videoStudioBenchSources{dir: dir, files: map[string]string{}, proxies: map[string]string{}}
	rate := videostudio.Rate{Num: videoStudioBenchSourceFPS, Den: 1}
	for i, layer := range videoStudioBenchLayerTable {
		id, stem := fmt.Sprintf("as%d", i+1), string(rune('a'+i))
		file, small := stem+".mp4", stem+"-540.mp4"
		encode(slices.Concat([]string{"-f", "lavfi", "-i", layer.source, "-vf", layer.filter, "-t", "20"}, original, []string{filepath.Join(dir, file)})...)
		encode(slices.Concat([]string{"-i", filepath.Join(dir, file), "-vf", "scale=960:540:flags=bicubic"}, proxy, []string{filepath.Join(dir, small)})...)
		asset, err := videostudio.Probe(ctx, ffmpeg, filepath.Join(dir, file))
		if err != nil {
			t.Fatalf("Probe(%s): %v", file, err)
		}
		asset.ID, asset.Name = id, file
		sources.assets = append(sources.assets, videostudio.AssetAtRate(asset, rate))
		sources.files[id], sources.proxies[id] = file, small
	}
	t.Logf("vs-preview-bench v2 media: %d sources (1920x1080, 30 fps, 20 s, H.264 High 12 Mbps) and 540p proxies in %s",
		len(sources.assets), time.Since(started).Round(time.Second))
	return sources
}

// videoStudioBenchProject is the M1 project of a scenario: 1920 x 1080 at 30 fps, four full-frame opaque video layers,
// each on its own video track (overlay tracks take images only), covering frames 0-599. Each effect gets complete
// params: the registry defaults plus the table's values. plain-gl-full drops the effects, and fx-gl-mixed gives layers
// 2-4 their blend modes.
func videoStudioBenchProject(t *testing.T, sources videoStudioBenchSources, scenario videoStudioBenchScenario) videostudio.Project {
	t.Helper()
	frames := 20 * videoStudioBenchSourceFPS
	for _, asset := range sources.assets {
		frames = min(frames, asset.DurationFrames)
	}
	project := videostudio.Project{Version: videostudio.CurrentVersion, Name: "Preview benchmark " + scenario.name,
		Width: 1920, Height: 1080, FPS: videoStudioBenchSourceFPS, Assets: sources.assets}
	for i, layer := range videoStudioBenchLayerTable {
		clip := videostudio.Clip{ID: fmt.Sprintf("c%d", i+1), AssetID: sources.assets[i].ID, Duration: frames,
			Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: videostudio.FitContain}
		if scenario.effects {
			for _, effect := range layer.effects {
				spec, ok := videostudio.EffectByID(effect.id)
				if !ok {
					t.Fatalf("effect %q is not in the registry", effect.id)
				}
				params := map[string]any{}
				for _, param := range spec.Params {
					params[param.ID] = param.Default
				}
				for key, value := range effect.values {
					params[key] = value
				}
				clip.Effects = append(clip.Effects, videostudio.EffectInstance{ID: effect.id, On: true, Params: params})
			}
		}
		if scenario.blends {
			clip.Blend = layer.blend
		}
		project.Tracks = append(project.Tracks, videostudio.Track{ID: fmt.Sprintf("v%d", i+1), Kind: videostudio.TrackVideo,
			Name: fmt.Sprintf("Layer %d", i+1), Clips: []videostudio.Clip{clip}})
	}
	if err := videostudio.Validate(project); err != nil {
		t.Fatalf("benchmark project %s: %v", scenario.name, err)
	}
	return project
}

// videoStudioBenchRun is what window.vsBench.run returns for one run.
type videoStudioBenchRun struct {
	Painter     string         `json:"painter"`
	Settled     bool           `json:"settled"`
	Seconds     float64        `json:"seconds"`
	Paints      []float64      `json:"paints"` // onPaint times in ms from the start of the measured window
	Shown       map[string]int `json:"shown"`  // asset id -> frames the layer showed in the window
	Dropped     int            `json:"dropped"`
	Videos      int            `json:"videos"`
	ScopeReads  int            `json:"scopeReads"`
	ScopeFrames int            `json:"scopeFrames"`
	Errors      []string       `json:"errors"`
	PageErrors  []string       `json:"pageErrors"`
}

// videoStudioBenchFigures are one run's figures (§4.26.6).
type videoStudioBenchFigures struct {
	videoFPS    []float64 // per layer, in asset order: frames shown / measured seconds
	videoFPSMin float64   // the slowest layer; 30 when smooth
	gaps50      int       // paint intervals over 50 ms
	p95, p99    float64   // paint interval percentiles in ms
}

// figures computes the run's figures over the project's assets. A layer that showed nothing counts as 0 fps.
func (r videoStudioBenchRun) figures(assets []videostudio.Asset) videoStudioBenchFigures {
	f := videoStudioBenchFigures{videoFPSMin: math.Inf(1)}
	for _, asset := range assets {
		fps := 0.0
		if r.Seconds > 0 {
			fps = float64(r.Shown[asset.ID]) / r.Seconds
		}
		f.videoFPS = append(f.videoFPS, fps)
		f.videoFPSMin = math.Min(f.videoFPSMin, fps)
	}
	if len(assets) == 0 {
		f.videoFPSMin = 0
	}
	var intervals []float64
	for i := 1; i < len(r.Paints); i++ {
		gap := r.Paints[i] - r.Paints[i-1]
		intervals = append(intervals, gap)
		if gap > 50 {
			f.gaps50++
		}
	}
	f.p95, f.p99 = videoStudioBenchPercentile(intervals, 95), videoStudioBenchPercentile(intervals, 99)
	return f
}

// videoStudioBenchPercentile is the spike's percentile (p2fspike js2/lib.js pct): the sorted value at index
// floor(p/100 * n), clamped to the last, or 0 for no values. p 50 is the median of the spike's tables.
func videoStudioBenchPercentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return sorted[min(len(sorted)-1, int(math.Floor(p/100*float64(len(sorted)))))]
}

// TestDesktopVideoStudioPreviewBenchmarkOptIn measures how smoothly the shipped preview plays the 2f benchmark project
// (index §4.26.6, spec §6.5, spike S3). It reports and asserts nothing. It fails only when a run cannot happen: a
// script error, no paint, a scenario drawn by another painter, or a scopes scenario that analysed no frame.
//   - Gates: AURAGO_RUN_BROWSER_SMOKE=1 and AURAGO_VIDEO_STUDIO_BENCHMARK=1, ffmpeg on the PATH, and a browser.
//   - Project: the spike's M1, four full-frame 1080p30 video layers with two effects each, one per video track.
//   - Method: the page paces requestAnimationFrame to a 60 Hz grid with two frames in flight (videoStudioBenchJS).
//     Each run seeks 0, settles, plays, waits a 2 s warm-up and measures 10 s. The scenarios run round-robin in each
//     of AURAGO_VS_BENCH_REPS (default and minimum 3) repetitions, because a busy Renoir drifts.
//   - Report: one line per launcher and scenario with the medians over the repetitions, then the target line.
//
// Local: AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./ui -run '^TestDesktopVideoStudioPreviewBenchmarkOptIn$' -count=1 -v -timeout 60m
// (AURAGO_VS_BENCH_LAUNCHERS=gpu AURAGO_VS_BENCH_GPU_FLAGS=force_low_power_gpu for the local iGPU). Renoir: index §6.
func TestDesktopVideoStudioPreviewBenchmarkOptIn(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" || os.Getenv("AURAGO_VIDEO_STUDIO_BENCHMARK") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 and AURAGO_VIDEO_STUDIO_BENCHMARK=1 to run the preview benchmark (reported, not asserted)")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("the preview benchmark needs ffmpeg on the PATH")
	}
	launchers := videoStudioBenchLaunchers(t)
	scenarios := videoStudioBenchScenarios(t)
	reps := 3
	if raw := strings.TrimSpace(os.Getenv("AURAGO_VS_BENCH_REPS")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("AURAGO_VS_BENCH_REPS=%q: %v", raw, err)
		}
		reps = max(3, n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	sources := videoStudioBenchMedia(t, ctx, ffmpeg, t.TempDir())
	engine := newVideoStudioEngineServer(t, sources.dir)
	projects := map[string]videostudio.Project{}
	for _, scenario := range scenarios {
		projects[scenario.name] = videoStudioBenchProject(t, sources, scenario)
	}
	yes := func(ok bool) string {
		if ok {
			return "yes"
		}
		return "no"
	}
	for _, l := range launchers {
		t.Run(l.name, func(t *testing.T) {
			page := openVideoStudioEngine(t, videoStudioBenchBrowser(t, l), engine)
			page.MustSetViewport(1280, 720, 1, false)
			// The preview routes its media through Web Audio; a click is the user gesture next to the autoplay switch.
			page.Mouse.MustMoveTo(20, 20)
			page.Mouse.MustClick(proto.InputMouseButtonLeft)
			page.MustEval(videoStudioBenchJS)
			renderer := page.MustEval(`() => window.vsBench.renderer()`).Str()
			capBuild := page.MustEval(`() => !!(window.VideoStudioPicture && typeof window.VideoStudioPicture.effectSize === 'function')`).Bool()
			t.Logf("vs-preview-bench v2 env launcher=%s browser=%q renderer=%q effect_cap_build=%s reps=%d flags=%q",
				l.name, page.Browser().MustVersion().Product, renderer, yes(capBuild), reps, strings.Join(l.flags, " "))
			software := renderer == "n/a"
			for _, marker := range []string{"SwiftShader", "llvmpipe", "softpipe", "lavapipe", "Basic Render"} {
				software = software || strings.Contains(renderer, marker)
			}
			if l.name == "gpu" && software {
				t.Logf("vs-preview-bench v2 warning: launcher=gpu renderer=%q is not a hardware GPU; its lines do not count (index §4.26.6)", renderer)
			}
			runs := map[string][]videoStudioBenchRun{}
			for rep := 1; rep <= reps; rep++ {
				for _, scenario := range scenarios {
					files := sources.files
					if scenario.proxy {
						files = sources.proxies
					}
					effectCap := videoStudioBenchEffectCap
					if scenario.quality == "full" {
						effectCap = 0
					}
					spec := map[string]any{"project": projects[scenario.name], "files": files, "painter": scenario.painter,
						"scale": scenario.scale, "effectCap": effectCap, "scopes": scenario.scopes, "warmMs": 2000, "measureMs": 10000}
					timed := page.Timeout(4 * time.Minute)
					result, err := timed.Eval(`spec => window.vsBench.run(spec)`, spec)
					timed.CancelTimeout()
					if err != nil {
						t.Fatalf("%s, repetition %d: %v", scenario.name, rep, err)
					}
					var run videoStudioBenchRun
					if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &run); err != nil {
						t.Fatalf("%s, repetition %d: decode %s: %v", scenario.name, rep, result.Value.JSON("", ""), err)
					}
					fig := run.figures(sources.assets)
					perLayer := make([]string, len(fig.videoFPS))
					for i, value := range fig.videoFPS {
						perLayer[i] = strconv.FormatFloat(value, 'f', 1, 64)
					}
					t.Logf("vs-preview-bench v2 run launcher=%s scenario=%s rep=%d painter=%s settled=%t seconds=%.2f paints=%d video_fps=%s video_fps_min=%.1f gaps50=%d interval_p95_ms=%.1f interval_p99_ms=%.1f dropped=%d videos=%d scope_reads=%d scope_frames=%d",
						l.name, scenario.name, rep, run.Painter, run.Settled, run.Seconds, len(run.Paints), strings.Join(perLayer, "/"),
						fig.videoFPSMin, fig.gaps50, fig.p95, fig.p99, run.Dropped, run.Videos, run.ScopeReads, run.ScopeFrames)
					if len(run.Errors) > 0 || len(run.PageErrors) > 0 {
						t.Errorf("%s, repetition %d: script errors %q, page errors %q", scenario.name, rep, run.Errors, run.PageErrors)
					}
					if len(run.Paints) < 2 {
						t.Errorf("%s, repetition %d: %d paints in the %.1f s window", scenario.name, rep, len(run.Paints), run.Seconds)
					}
					if scenario.scopes && run.ScopeFrames == 0 {
						t.Errorf("%s, repetition %d: the scopes rule read and analysed no frame (%d reads)", scenario.name, rep, run.ScopeReads)
					}
					if run.Painter != scenario.painter {
						t.Errorf("%s, repetition %d: the preview drew with the %s painter, want %s; the run does not count", scenario.name, rep, run.Painter, scenario.painter)
					} else {
						runs[scenario.name] = append(runs[scenario.name], run)
					}
					time.Sleep(300 * time.Millisecond)
				}
			}
			median := func(values []float64) float64 { return videoStudioBenchPercentile(values, 50) }
			for _, scenario := range scenarios {
				var fps, gaps, p95, p99, dropped []float64
				for _, run := range runs[scenario.name] {
					fig := run.figures(sources.assets)
					fps = append(fps, fig.videoFPSMin)
					gaps = append(gaps, float64(fig.gaps50))
					p95 = append(p95, fig.p95)
					p99 = append(p99, fig.p99)
					dropped = append(dropped, float64(run.Dropped))
				}
				if len(fps) == 0 {
					continue
				}
				media := "original"
				if scenario.proxy {
					media = "proxy"
				}
				fpsMin, gaps50 := median(fps), median(gaps)
				met := fpsMin >= 0.98*videoStudioBenchSourceFPS && gaps50 <= 3
				t.Logf("vs-preview-bench v2 launcher=%s renderer=%q scenario=%s painter=%s quality=%s backing=%dx%d media=%s source_fps=%d reps=%d video_fps_min=%.1f gaps50=%.0f interval_p95_ms=%.1f interval_p99_ms=%.1f dropped=%.0f target_met=%s",
					l.name, renderer, scenario.name, scenario.painter, scenario.quality, int(math.Round(1920*scenario.scale)), int(math.Round(1080*scenario.scale)),
					media, videoStudioBenchSourceFPS, len(fps), fpsMin, gaps50, median(p95), median(p99), median(dropped), yes(met))
			}
		})
	}
	t.Logf("vs-preview-bench v2 target: video_fps_min >= %.1f (0.98 x source fps) and gaps50 <= 3 (spec §6.5; reported, not asserted)", 0.98*videoStudioBenchSourceFPS)
}

// videoStudioBenchJS installs the test-only window.vsBench on the engine page; it is never shipped. It ports the spike
// harness (p2fspike js2/early.js, instr.js, lib.js playRun and s2play.js):
//   - requestAnimationFrame runs on an emulated 60 Hz display. One native pump runs the queued callbacks of a frame
//     together, but only on a grid boundary at or after the end of the previous frame, and only while fewer than two
//     paints are on the GPU. During a run each paint pushes a fence and flushes. The 2D painter is paced by the grid
//     alone. With pace 0 (outside runs) native frames pass through.
//   - HTMLCanvasElement.prototype.getContext captures the compositor's webgl2 context during mount, and
//     document.createElement the preview's video elements. Each element records the media time of its last presented
//     frame (requestVideoFrameCallback).
//   - run(spec) mounts the shipped preview on a fresh 640 x 360 CSS canvas with {painter, mediaURL, onPaint, effectCap},
//     seeks 0, settles (30 s), plays, waits spec.warmMs, then measures spec.measureMs. It returns:
//   - the painter and the paint times of the window;
//   - the frames each layer showed: the paints at which its element had presented a new frame since that layer's
//     previous paint;
//   - the dropped decoded frames;
//   - for the scopes scenario, the panel's rule (§4.26.2): due(), one read in flight, and one trailing read when a
//     due paint found a read pending, then analyse and the three images put on 2D canvases.
//   - renderer() names the renderer of a fresh webgl2 context: UNMASKED_RENDERER_WEBGL, else RENDERER, else "n/a".
const videoStudioBenchJS = `() => {
  'use strict';
  if (window.vsBench) return true;
  const B = { pace: 0, inflight: 2, next: 0, gl: null, captureGL: false, fences: [], videos: [], errors: [] };
  const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
  const note = (where, error) => { B.errors.push(where + ': ' + String(error && error.stack || error)); };

  function gpuBusy() {
    const gl = B.gl;
    if (!gl || !B.inflight) return false;
    while (B.fences.length) {
      let status = 0;
      try { status = gl.clientWaitSync(B.fences[0], 0, 0); } catch (_) { status = 0; }
      if (status === gl.TIMEOUT_EXPIRED) break;
      const fence = B.fences.shift();
      try { gl.deleteSync(fence); } catch (_) { /* a lost context */ }
    }
    return B.fences.length >= B.inflight;
  }
  function dropFences() {
    const gl = B.gl;
    B.fences.splice(0).forEach(fence => { try { if (gl) gl.deleteSync(fence); } catch (_) { /* a lost context */ } });
  }

  const nativeRAF = window.requestAnimationFrame.bind(window);
  const queue = new Map();
  let nextId = 1, pumpId = 0;
  function pump(stamp) {
    pumpId = 0;
    if (!queue.size) return;
    if (B.pace > 0 && (performance.now() < B.next || gpuBusy())) { pumpId = nativeRAF(pump); return; }
    const batch = Array.from(queue.values());
    queue.clear();
    batch.forEach(callback => { try { callback(stamp); } catch (error) { note('requestAnimationFrame', error); } });
    if (B.pace > 0) { const period = 1000 / B.pace; B.next = Math.ceil(performance.now() / period) * period; }
    if (queue.size && !pumpId) pumpId = nativeRAF(pump);
  }
  window.requestAnimationFrame = callback => {
    const id = nextId++;
    queue.set(id, callback);
    if (!pumpId) pumpId = nativeRAF(pump);
    return id;
  };
  window.cancelAnimationFrame = id => { queue.delete(id); };

  const nativeGetContext = HTMLCanvasElement.prototype.getContext;
  HTMLCanvasElement.prototype.getContext = function () {
    const context = nativeGetContext.apply(this, arguments);
    if (B.captureGL && arguments[0] === 'webgl2' && context) B.gl = context;
    return context;
  };
  const nativeCreate = document.createElement;
  document.createElement = function () {
    const el = nativeCreate.apply(this, arguments);
    if (el instanceof HTMLVideoElement) B.videos.push(el);
    return el;
  };

  function chain(el) {
    if (el.__vsBenchChained || typeof el.requestVideoFrameCallback !== 'function') return;
    el.__vsBenchChained = true;
    el.__vsBenchMediaTime = -1;
    const onFrame = (now, meta) => {
      el.__vsBenchMediaTime = meta && typeof meta.mediaTime === 'number' ? meta.mediaTime : now;
      if (!el.__vsBenchStopped) el.requestVideoFrameCallback(onFrame);
    };
    el.requestVideoFrameCallback(onFrame);
  }
  const mark = el => (el.__vsBenchChained ? el.__vsBenchMediaTime : Number(el.currentTime) || 0);

  const scopeTargets = [['histogram', 256, 100], ['waveform', 256, 128], ['vectorscope', 128, 128]].map(([kind, width, height]) => {
    const canvas = nativeCreate.call(document, 'canvas');
    canvas.width = width;
    canvas.height = height;
    return { kind, width, height, context: canvas.getContext('2d') };
  });
  function drawScopes(frame) {
    const SM = window.VideoStudioScopeMath;
    const analysis = SM.analyse(frame);
    if (!analysis) return false;
    const images = { histogram: SM.histogramImage(analysis), waveform: SM.waveformImage(analysis), vectorscope: SM.vectorscopeImage(analysis) };
    scopeTargets.forEach(target => {
      const image = images[target.kind];
      const pixels = image instanceof Uint8ClampedArray ? image : new Uint8ClampedArray(image);
      target.context.putImageData(new ImageData(pixels, target.width, target.height), 0, 0);
    });
    return true;
  }
  function scopeRead(run, now) {
    run.scopePending = true;
    run.scopeDirty = false;
    run.scopeLast = now;
    if (run.measuring) run.scopeReads++;
    Promise.resolve(run.preview.readScaled(256, 144)).then(frame => {
      if (frame && drawScopes(frame) && run.measuring) run.scopeFrames++;
    }).catch(error => note('scopes', error)).then(() => {
      run.scopePending = false;
      if (run.scopeDirty && run.preview) scopeRead(run, performance.now());
    });
  }
  function scopeTick(run, now) {
    const SM = window.VideoStudioScopeMath;
    if (!run.preview || !SM.due(run.scopeLast, now, run.preview.isPlaying())) return;
    if (run.scopePending) { run.scopeDirty = true; return; }
    scopeRead(run, now);
  }

  function afterPaint(run) {
    try {
      const now = performance.now();
      B.videos.forEach(chain);
      if (run.measuring) {
        run.paints.push(now);
        B.videos.forEach(el => {
          const id = el.__vsAssetId;
          if (!id) return;
          const seen = mark(el);
          if (seen !== el.__vsBenchSeen) {
            el.__vsBenchSeen = seen;
            if (seen >= 0) run.shown[id] = (run.shown[id] || 0) + 1;
          }
        });
      }
      if (run.scopes) scopeTick(run, now);
      if (B.pace > 0 && B.gl) {
        const fence = B.gl.fenceSync(B.gl.SYNC_GPU_COMMANDS_COMPLETE, 0);
        if (fence) B.fences.push(fence);
        B.gl.flush();
      }
    } catch (error) { note('onPaint', error); }
  }

  async function run(spec) {
    const SM = window.VideoStudioScopeMath;
    if (spec.scopes && !(SM && typeof SM.due === 'function' && typeof SM.analyse === 'function')) throw new Error('VideoStudioScopeMath is not loaded (plan 2f Task 2)');
    const stage = document.getElementById('stage');
    const canvas = nativeCreate.call(document, 'canvas');
    canvas.setAttribute('data-preview', '');
    canvas.style.width = '640px';
    canvas.style.height = '360px';
    stage.replaceChildren(canvas);
    B.pace = 0; B.videos = []; B.gl = null; B.fences = []; B.errors = [];
    const state = { measuring: false, paints: [], shown: {}, scopes: !!spec.scopes, preview: null, scopePending: false, scopeDirty: false, scopeLast: -Infinity, scopeReads: 0, scopeFrames: 0 };
    let shown = 0;
    const mediaURL = asset => '/media/' + encodeURIComponent(String(spec.files[asset.id] || ''));
    let preview;
    B.captureGL = true;
    try {
      preview = window.VideoStudioPreview.mount(canvas, () => spec.project, () => shown, value => { shown = value; }, () => {}, () => spec.scale,
        { painter: spec.painter, mediaURL, onPaint: () => afterPaint(state), effectCap: () => spec.effectCap });
    } finally { B.captureGL = false; }
    if (spec.scopes && typeof preview.readScaled !== 'function') { preview.dispose(); throw new Error('the preview has no readScaled (plan 2f Task 1)'); }
    state.preview = preview;
    const out = { painter: preview.painterKind(), settled: false, seconds: 0, paints: [], shown: {}, dropped: 0, videos: 0, scopeReads: 0, scopeFrames: 0, errors: [], pageErrors: [] };
    try {
      preview.seek(0);
      out.settled = await preview.settle(30000);
      B.pace = 60; B.next = 0;
      preview.play();
      await sleep(spec.warmMs);
      const elements = B.videos.filter(el => el.__vsAssetId);
      const dropped0 = elements.map(el => el.getVideoPlaybackQuality().droppedVideoFrames);
      elements.forEach(el => { el.__vsBenchSeen = mark(el); });
      state.paints = []; state.shown = {}; state.scopeReads = 0; state.scopeFrames = 0;
      const t0 = performance.now();
      state.measuring = true;
      await sleep(spec.measureMs);
      state.measuring = false;
      const t1 = performance.now();
      out.dropped = elements.reduce((sum, el, i) => sum + Math.max(0, el.getVideoPlaybackQuality().droppedVideoFrames - dropped0[i]), 0);
      out.seconds = (t1 - t0) / 1000;
      out.paints = state.paints.map(at => Math.round((at - t0) * 100) / 100);
      out.shown = state.shown;
      out.videos = elements.length;
      out.scopeReads = state.scopeReads;
      out.scopeFrames = state.scopeFrames;
      out.painter = preview.painterKind();
    } finally {
      B.pace = 0;
      state.preview = null;
      try { preview.pause(); preview.dispose(); } catch (error) { note('dispose', error); }
      B.videos.forEach(el => { el.__vsBenchStopped = true; });
      dropFences();
      B.gl = null;
      stage.replaceChildren();
      out.errors = B.errors.splice(0);
      out.pageErrors = (window.vsEngineErrors || []).splice(0);
    }
    return out;
  }

  function renderer() {
    const canvas = nativeCreate.call(document, 'canvas');
    const gl = nativeGetContext.call(canvas, 'webgl2', { powerPreference: 'high-performance' });
    if (!gl) return 'n/a';
    const info = gl.getExtension('WEBGL_debug_renderer_info');
    const name = String((info ? gl.getParameter(info.UNMASKED_RENDERER_WEBGL) : gl.getParameter(gl.RENDERER)) || '');
    const lose = gl.getExtension('WEBGL_lose_context');
    if (lose) lose.loseContext();
    return name || 'n/a';
  }

  window.vsBench = { run, renderer };
  return true;
}`
