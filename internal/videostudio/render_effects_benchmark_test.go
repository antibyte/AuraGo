package videostudio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// effectsBenchEffect is one effect of a benchmark layer: a registry id and the values that differ from its defaults.
type effectsBenchEffect struct {
	id     string
	values map[string]any
}

// effectsBenchLayers is the 2f spike's M1 project (index §4.26.6; p2fspike js2/lib.js and gen_media2.sh), with the same
// values as the ui preview benchmark's videoStudioBenchLayerTable. Per layer: the lavfi source and its filter, the blend
// mode of the mixed variant (layer 1 stays normal), and two effects in stack order. Numbers are float64, as Validate
// wants.
var effectsBenchLayers = []struct {
	source, filter, blend string
	effects               []effectsBenchEffect
}{
	{source: "testsrc2=size=1920x1080:rate=30", filter: "noise=alls=3:allf=t,format=yuv420p", effects: []effectsBenchEffect{
		{"color", map[string]any{"exposure": 0.15, "contrast": 1.1, "saturation": 1.2, "temperature": 6000.0}},
		{"blur", map[string]any{"radius": 0.01}},
	}},
	{source: "mandelbrot=size=1920x1080:rate=30:maxiter=400", filter: "noise=alls=2:allf=t,format=yuv420p", blend: "overlay", effects: []effectsBenchEffect{
		{"lut", map[string]any{"lut": "builtin:teal_orange", "intensity": 0.8}},
		{"vignette", map[string]any{"amount": 0.5, "size": 1.0}},
	}},
	{source: "gradients=size=1920x1080:rate=30:speed=0.04:nb_colors=5", filter: "noise=alls=4:allf=t,format=yuv420p", blend: "softlight", effects: []effectsBenchEffect{
		{"sharpen", map[string]any{"strength": 0.6}},
		{"color", map[string]any{"saturation": 1.15, "contrast": 1.05}},
	}},
	{source: "smptehdbars=size=1920x1080:rate=30", filter: "hue=h=t*18:s=1.2,noise=alls=3:allf=t,format=yuv420p", blend: "screen", effects: []effectsBenchEffect{
		{"chroma_key", map[string]any{"color": "#00b140", "similarity": 0.25, "smoothness": 0.1, "spill": 0.5}},
		{"vignette", map[string]any{"amount": 0.3, "size": 1.2}},
	}},
}

// effectsBenchEntry is an enabled effect with complete params: the registry defaults (EffectRegistry) with values on top.
func effectsBenchEntry(t *testing.T, id string, values map[string]any) EffectInstance {
	t.Helper()
	for _, effect := range EffectRegistry().Effects {
		if effect.ID != id {
			continue
		}
		params := map[string]any{}
		for _, param := range effect.Params {
			params[param.ID] = param.Default
		}
		for key, value := range values {
			params[key] = value
		}
		return EffectInstance{ID: id, On: true, Params: params}
	}
	t.Fatalf("effect %q is not in the registry", id)
	return EffectInstance{}
}

// effectsBenchMedia writes the four 60 s sources as1..as4.mp4 into dir with the §4.26.6 recipe (libx264, High profile,
// level 4.1, 12 Mbps with a 16 Mbps / 24 Mbit VBV, GOP 30, yuv420p, no audio, faststart) at -preset veryfast to keep
// generation short. The renders decode the sources, so the preset does not change what is measured. It returns the
// asset id -> path map.
func effectsBenchMedia(t *testing.T, ctx context.Context, ffmpeg, dir string) map[string]string {
	t.Helper()
	encode := []string{"-c:v", "libx264", "-preset", "veryfast", "-b:v", "12M", "-maxrate", "16M", "-bufsize", "24M",
		"-pix_fmt", "yuv420p", "-profile:v", "high", "-level", "4.1", "-g", "30", "-an", "-movflags", "+faststart"}
	paths := map[string]string{}
	for i, layer := range effectsBenchLayers {
		id := fmt.Sprintf("as%d", i+1)
		path := filepath.Join(dir, id+".mp4")
		started := time.Now()
		args := slices.Concat([]string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", layer.source,
			"-vf", layer.filter, "-t", "60"}, encode, []string{path})
		if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
			t.Fatalf("source %s (%s): %v: %s", id, layer.source, err, out)
		}
		t.Logf("vs-export-bench v2 source %s: 60 s of %s in %.1f s", id, layer.source, time.Since(started).Seconds())
		paths[id] = path
	}
	return paths
}

// effectsBenchProject is the benchmark project of a variant (index §4.26.7): 1920 x 1080 at 30 fps, and one clip of
// frames per layer, each on its own video track (overlay tracks take images only). fx carries the M1 effects, plain
// none, and mixed the M1 effects with layers 2-4 in overlay, softlight and screen.
func effectsBenchProject(t *testing.T, variant string, assets []Asset, frames int) Project {
	t.Helper()
	project := Project{Version: CurrentVersion, Name: "Effects benchmark " + variant, Width: 1920, Height: 1080, FPS: 30, Assets: assets}
	for i, layer := range effectsBenchLayers {
		clip := Clip{ID: fmt.Sprintf("c%d", i+1), AssetID: assets[i].ID, Duration: frames, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
		if variant != "plain" {
			for _, effect := range layer.effects {
				clip.Effects = append(clip.Effects, effectsBenchEntry(t, effect.id, effect.values))
			}
		}
		if variant == "mixed" {
			clip.Blend = layer.blend
		}
		project.Tracks = append(project.Tracks, Track{ID: fmt.Sprintf("v%d", i+1), Kind: TrackVideo, Name: fmt.Sprintf("Layer %d", i+1), Clips: []Clip{clip}})
	}
	if err := Validate(project); err != nil {
		t.Fatalf("%s project: %v", variant, err)
	}
	return project
}

// Run with AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio -run '^TestEffectsExportBenchmarkOptIn$' -count=1 -v -timeout 150m.
//
// TestEffectsExportBenchmarkOptIn measures the export of the 2f benchmark project (index §4.26.7, spike S4/S5): 60 s of
// 1920 x 1080 at 30 fps with four full-frame video layers, each on its own video track. It asserts nothing.
//   - Variants: fx (the M1 effects, two per layer), plain (the same layers without effects, for the effect share) and
//     mixed (fx with layers 2-4 in overlay, softlight and screen).
//   - Export: RenderWithOptions with the zero Encoder (libx264 at the export's defaults) over the whole timeline. Each
//     variant takes the best of AURAGO_VS_BENCH_EXPORT_RUNS runs (default 3), interleaved across the variants, because
//     the desktop varies up to 2x. The 5.1.9 container passes 1.
//   - Graph: segmentCost over frames 0-250, best of 3, at FFmpeg's default filter threads and at the export's threadLimit.
//
// There is no export effect resolution cap (spike S4). Effects run on the source frame before scale and transform
// (spec §7.1), so a smaller canvas renders as slowly, and a cap would change output pixels.
func TestEffectsExportBenchmarkOptIn(t *testing.T) {
	if os.Getenv("AURAGO_VIDEO_STUDIO_BENCHMARK") != "1" {
		t.Skip("set AURAGO_VIDEO_STUDIO_BENCHMARK=1 to run the export benchmark (reported, not asserted)")
	}
	ffmpeg := requireFFmpeg(t)
	runs := 3
	if raw := strings.TrimSpace(os.Getenv("AURAGO_VS_BENCH_EXPORT_RUNS")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			t.Fatalf("AURAGO_VS_BENCH_EXPORT_RUNS=%q: want a whole number of at least 1", raw)
		}
		runs = n
	}
	deadline := time.Now().Add(150 * time.Minute)
	if testDeadline, ok := t.Deadline(); ok {
		deadline = testDeadline.Add(-time.Minute)
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	version := ffmpegVersionLine(t, ffmpeg)
	t.Logf("vs-export-bench v2 env ffmpeg=%q cpus=%d export_runs=%d", version, runtime.NumCPU(), runs)
	root := t.TempDir()
	paths := effectsBenchMedia(t, ctx, ffmpeg, root)
	rate := Rate{Num: 30, Den: 1}
	frames := 1800
	var assets []Asset
	for i := range effectsBenchLayers {
		id := fmt.Sprintf("as%d", i+1)
		asset, err := Probe(ctx, ffmpeg, paths[id])
		if err != nil {
			t.Fatalf("Probe(%s): %v", id, err)
		}
		asset.ID, asset.Name = id, id+".mp4"
		asset = AssetAtRate(asset, rate)
		frames = min(frames, asset.DurationFrames)
		assets = append(assets, asset)
	}
	variants := []string{"fx", "plain", "mixed"}
	projects := map[string]Project{}
	for _, variant := range variants {
		projects[variant] = effectsBenchProject(t, variant, assets, frames)
	}
	best := map[string]time.Duration{}
	for attempt := 1; attempt <= runs; attempt++ {
		for _, variant := range variants {
			output := filepath.Join(root, variant+".mp4")
			started := time.Now()
			if err := RenderWithOptions(ctx, RenderOptions{FFmpegPath: ffmpeg, Project: projects[variant], AssetPaths: paths, Output: output}); err != nil {
				t.Fatalf("export %s, run %d: %v", variant, attempt, err)
			}
			took := time.Since(started)
			_ = os.Remove(output)
			t.Logf("vs-export-bench v2 run variant=%s run=%d export_s=%.1f", variant, attempt, took.Seconds())
			if previous, ok := best[variant]; !ok || took < previous {
				best[variant] = took
			}
		}
	}
	graphFrames := min(250, frames)
	timeline := float64(frames) / 30
	for _, variant := range variants {
		perFrame := func(threads int) float64 {
			cost := segmentCost(t, ctx, ffmpeg, projects[variant], paths, Segment{Start: 0, End: graphFrames}, threads, 3)
			return float64(cost) / float64(time.Millisecond) / float64(graphFrames)
		}
		defaultThreads, exportThreads := perFrame(0), perFrame(threadLimit)
		exportSeconds := best[variant].Seconds()
		t.Logf("vs-export-bench v2 ffmpeg=%q variant=%s frames=%d runs=%d encoder=libx264 export_s=%.1f export_fps=%.2f realtime=%.2f graph_ms_per_frame=%.1f/%.1f",
			version, variant, frames, runs, exportSeconds, float64(frames)/exportSeconds, exportSeconds/timeline, defaultThreads, exportThreads)
	}
	fx, plain := best["fx"].Seconds(), best["plain"].Seconds()
	t.Logf("vs-export-bench v2 effect share: fx %.0f %% of the export time over plain", 100*(fx-plain)/fx)
	t.Log("vs-export-bench v2 note: no export effect resolution cap (spike S4); effects run on the source frame before scale and transform (spec §7.1)")
	t.Log("vs-export-bench v2 spike (S4, for comparison): fx 291.7 s on 8.1.1 (best of two) and 299.5 s on 5.1.9; plain 63.7 s and 99.9 s; mixed 328.6 s and 362.5 s")
}
