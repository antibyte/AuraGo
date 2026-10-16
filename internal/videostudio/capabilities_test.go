package videostudio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// specPhase2XFade are the 20 xfade transitions spec §7.7 adds in 2e; FFmpeg 5.1 lists every one.
var specPhase2XFade = []string{"wipeup", "wipedown", "slideleft", "slideright", "slideup", "slidedown", "circleopen", "circleclose", "circlecrop", "rectcrop", "radial", "smoothleft", "smoothright", "fadewhite", "fadegrays", "diagtl", "diagbr", "pixelize", "zoomin", "squeezeh"}

func resetCapabilityMemo() {
	capabilityMemo.Lock()
	clear(capabilityMemo.entries)
	capabilityMemo.Unlock()
}

// filterListing builds a minimal `ffmpeg -filters` output that lists the given filters.
func filterListing(names ...string) string {
	var b strings.Builder
	b.WriteString("Filters:\n  T.. = Timeline support\n  .S. = Slice threading\n  ..C = Command support\n")
	for _, name := range names {
		fmt.Fprintf(&b, " TSC %-17s V->V       Test filter.\n", name)
	}
	return b.String()
}

// xfadeHelp builds a minimal `ffmpeg -h filter=xfade` output that lists the given transitions.
func xfadeHelp(names ...string) string {
	var b strings.Builder
	b.WriteString("Filter xfade\n  Cross fade one video with another video.\nxfade AVOptions:\n")
	b.WriteString("  transition        <int>        ..FV....... set cross fade transition (from -1 to 45) (default fade)\n")
	b.WriteString("     custom          -1           ..FV....... custom transition\n")
	for i, name := range names {
		fmt.Fprintf(&b, "     %-15s %-12d ..FV....... %s transition\n", name, i, name)
	}
	b.WriteString("  duration          <duration>   ..FV....... set cross fade duration (default 1)\n")
	return b.String()
}

// registryXFadeNames lists the xfade names of the registry transitions, leaving out except.
func registryXFadeNames(except string) []string {
	var names []string
	for _, spec := range EffectRegistry().Transitions {
		if spec.XFade != except {
			names = append(names, spec.XFade)
		}
	}
	return names
}

// fakeCapabilityFFmpeg replaces the FFmpeg queries of ProbeCapabilities with fixed outputs and counts them. The
// returned path is a regular file, which is all resolveBinary and the memo key need; it is never run.
func fakeCapabilityFFmpeg(t *testing.T, filters, help string) (string, *int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(path, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	previous := capabilityCommand
	capabilityCommand = func(_ context.Context, ffmpeg string, args ...string) ([]byte, error) {
		calls++
		if ffmpeg != path {
			return nil, fmt.Errorf("unexpected binary %q", ffmpeg)
		}
		switch strings.Join(args, " ") {
		case "-hide_banner -filters":
			return []byte(filters), nil
		case "-hide_banner -h filter=xfade":
			return []byte(help), nil
		}
		return nil, fmt.Errorf("unexpected arguments %q", args)
	}
	resetCapabilityMemo()
	t.Cleanup(func() {
		capabilityCommand = previous
		resetCapabilityMemo()
	})
	return path, &calls
}

func capabilityTestProject(kind TransitionType) Project {
	project := validProject()
	project.Tracks[0].Clips = []Clip{
		{ID: "out", AssetID: "video", Start: 0, Duration: 60, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: &Transition{Type: kind, Duration: 10}},
		{ID: "in", AssetID: "video", Start: 50, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain},
	}
	return project
}

func TestCapabilitiesXFadeHelpParser(t *testing.T) {
	for _, name := range []string{"xfade-help-5.1.txt", "xfade-help-8.txt"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		found := xfadeTransitionNames(string(data))
		for _, spec := range EffectRegistry().Transitions {
			if !found[spec.XFade] {
				t.Fatalf("%s: registry transition %q (xfade %q) not found in %v", name, spec.ID, spec.XFade, found)
			}
		}
		for _, xfade := range specPhase2XFade {
			if !found[xfade] {
				t.Fatalf("%s: phase 2 transition %q not found in %v", name, xfade, found)
			}
		}
		for _, option := range []string{"transition", "duration", "offset", "expr"} {
			if found[option] {
				t.Fatalf("%s: the option %q was read as a transition", name, option)
			}
		}
	}
	if found := xfadeTransitionNames("Unknown filter 'xfade'.\n"); len(found) != 0 {
		t.Fatalf("an unknown-filter message produced transitions: %v", found)
	}
}

func TestCapabilitiesFilterListParser(t *testing.T) {
	const listing = "Filters:\n" +
		"  T.. = Timeline support\n" +
		"  .S. = Slice threading\n" +
		"  ..C = Command support\n" +
		"  A = Audio input/output\n" +
		"  | = Source or sink filter\n" +
		" ... abench            A->A       Benchmark part of a filtergraph.\n" +
		" TSC blend             VV->V      Blend two video frames into each other.\n" +
		" T.C gblur             V->V       Apply Gaussian Blur filter.\n" +
		" TSC tblend            V->V       Blend successive frames.\n" +
		" ... xfade             VV->V      Cross fade one video with another video.\n"
	// FFmpeg 8 lists two flags per filter and drops the command-support legend line; the rows are CRLF-terminated on
	// Windows builds.
	const listing8 = "Filters:\r\n" +
		"  T.. = Timeline support\r\n" +
		"  .S. = Slice threading\r\n" +
		"  A = Audio input/output\r\n" +
		"  | = Source or sink filter\r\n" +
		"  ------\r\n" +
		" .. abench            A->A       Benchmark part of a filtergraph.\r\n" +
		" TS blend             VV->V      Blend two video frames into each other.\r\n" +
		" TS gblur             V->V       Apply Gaussian Blur filter.\r\n" +
		" .S xfade             VV->V      Cross fade one video with another video.\r\n"
	for name, text := range map[string]string{"FFmpeg 5.1 layout": listing, "FFmpeg 8 layout": listing8} {
		names := ffmpegFilterNames(text)
		for _, want := range []string{"abench", "blend", "gblur", "xfade"} {
			if !names[want] {
				t.Fatalf("%s: filter %q not found in %v", name, want, names)
			}
		}
		for _, wrong := range []string{"=", "Blend", "Benchmark", "Timeline", "A", "|", "------"} {
			if names[wrong] {
				t.Fatalf("%s: %q was read as a filter name: %v", name, wrong, names)
			}
		}
	}
	if names := ffmpegFilterNames(listing); !names["tblend"] {
		t.Fatalf("filter %q not found in %v", "tblend", names)
	}
}

func TestRegistryUnavailableMapsMissingFeatures(t *testing.T) {
	registry := EffectRegistry()
	data, err := json.Marshal(registry.Unavailable(nil))
	if err != nil || string(data) != `{"effects":[],"transitions":[],"blend_modes":[],"anim_presets":[]}` {
		t.Fatalf("Unavailable(nil) = %s, %v", data, err)
	}
	set := registry.Unavailable([]string{"xfade:fadeblack", "filter:lut3d", "filter:blend", "bogus"})
	if !slices.Equal(set.Transitions, []string{"black"}) {
		t.Fatalf("unavailable transitions = %v, want [black]", set.Transitions)
	}
	wantEffects := []string{}
	for _, effect := range registry.Effects {
		if slices.Contains(effect.Filters, "lut3d") {
			wantEffects = append(wantEffects, effect.ID)
		}
	}
	if !slices.Equal(set.Effects, wantEffects) {
		t.Fatalf("unavailable effects = %v, want %v", set.Effects, wantEffects)
	}
	wantModes := []string{}
	for _, mode := range registry.BlendModes {
		if mode.Blend != "" {
			wantModes = append(wantModes, mode.ID)
		}
	}
	if !slices.Equal(set.BlendModes, wantModes) || set.AnimPresets == nil {
		t.Fatalf("unavailable blend modes = %v (want %v), animation presets = %v", set.BlendModes, wantModes, set.AnimPresets)
	}
}

func TestCheckProjectCapabilitiesRefusesMissingTransition(t *testing.T) {
	missing := []string{"xfade:fadeblack", "filter:lut3d"}
	if err := CheckProjectCapabilities(capabilityTestProject(TransitionBlack), nil); err != nil {
		t.Fatalf("nothing missing: %v", err)
	}
	if err := CheckProjectCapabilities(capabilityTestProject(TransitionDissolve), missing); err != nil {
		t.Fatalf("a dissolve needs no missing feature: %v", err)
	}
	err := CheckProjectCapabilities(capabilityTestProject(TransitionBlack), missing)
	var capability *CapabilityError
	if !errors.Is(err, ErrFeatureUnavailable) || !errors.As(err, &capability) || capability.Feature != "transition:black" || capability.Missing != "xfade:fadeblack" || !strings.Contains(err.Error(), "transition:black") {
		t.Fatalf("black transition without xfade:fadeblack = %v", err)
	}
	// Transitions that never render do not block the export.
	disabled := capabilityTestProject(TransitionBlack)
	disabled.Tracks[0].Clips[1].Disabled = true
	hidden := capabilityTestProject(TransitionBlack)
	hidden.Tracks[0].Hidden = true
	for name, project := range map[string]Project{"disabled incoming clip": disabled, "hidden track": hidden} {
		if err := CheckProjectCapabilities(project, missing); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestProbeCapabilitiesMemoizedPerBinary(t *testing.T) {
	ctx := context.Background()
	path, calls := fakeCapabilityFFmpeg(t, filterListing(append(capabilityFilters(), "xfade")...), xfadeHelp(registryXFadeNames("fadeblack")...))
	missing, err := ProbeCapabilities(ctx, path)
	if err != nil || !slices.Equal(missing, []string{"xfade:fadeblack"}) || *calls != 2 {
		t.Fatalf("first probe = %v, %v after %d commands", missing, err, *calls)
	}
	missing[0] = "changed by the caller"
	again, err := ProbeCapabilities(ctx, path)
	if err != nil || !slices.Equal(again, []string{"xfade:fadeblack"}) || *calls != 2 {
		t.Fatalf("second probe = %v, %v after %d commands; want the memoized result", again, err, *calls)
	}
	// A replaced binary (another size) is probed again.
	if err := os.WriteFile(path, []byte("a newer fake ffmpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ProbeCapabilities(ctx, path); err != nil || *calls != 4 {
		t.Fatalf("probe after the binary changed: %v after %d commands", err, *calls)
	}
}

func TestProbeCapabilitiesReportsMissingFiltersAndTransitions(t *testing.T) {
	ctx := context.Background()
	var withoutLUT []string
	for _, name := range capabilityFilters() {
		if name != "lut3d" {
			withoutLUT = append(withoutLUT, name)
		}
	}
	path, _ := fakeCapabilityFFmpeg(t, filterListing(append(withoutLUT, "xfade")...), xfadeHelp(registryXFadeNames("")...))
	if missing, err := ProbeCapabilities(ctx, path); err != nil || !slices.Equal(missing, []string{"filter:lut3d"}) {
		t.Fatalf("probe without lut3d = %v, %v", missing, err)
	}
	// Without the xfade filter the help is not read and every registry transition is missing.
	path, calls := fakeCapabilityFFmpeg(t, filterListing(capabilityFilters()...), "")
	var want []string
	for _, xfade := range registryXFadeNames("") {
		want = append(want, "xfade:"+xfade)
	}
	if missing, err := ProbeCapabilities(ctx, path); err != nil || !slices.Equal(missing, want) || *calls != 1 {
		t.Fatalf("probe without xfade = %v, %v after %d commands; want %v", missing, err, *calls, want)
	}
}

func TestRenderRefusesMissingCapabilityBeforeStaging(t *testing.T) {
	path, _ := fakeCapabilityFFmpeg(t, filterListing(append(capabilityFilters(), "xfade")...), xfadeHelp(registryXFadeNames("fadeblack")...))
	err := RenderWithOptions(context.Background(), RenderOptions{
		FFmpegPath: path,
		Project:    capabilityTestProject(TransitionBlack),
		AssetPaths: map[string]string{"video": filepath.Join(t.TempDir(), "never-read.mp4")},
		Output:     filepath.Join(t.TempDir(), "out.mp4"),
	})
	var capability *CapabilityError
	if !errors.Is(err, ErrFeatureUnavailable) || !errors.As(err, &capability) || capability.Feature != "transition:black" || capability.Missing != "xfade:fadeblack" {
		t.Fatalf("render with a missing xfade transition = %v", err)
	}
}

func TestProbeCapabilitiesLocalFFmpegMissesNothing(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	resetCapabilityMemo()
	missing, err := ProbeCapabilities(context.Background(), ffmpeg)
	if err != nil || len(missing) != 0 {
		t.Fatalf("ProbeCapabilities(%s) = %v, %v; want nothing missing", ffmpeg, missing, err)
	}
	if status := CheckRuntime(context.Background(), ffmpeg); !status.Ready || len(status.Missing) != 0 {
		t.Fatalf("CheckRuntime() = %+v", status)
	}
}

func TestCheckProjectCapabilitiesKeyframes(t *testing.T) {
	keyframed := func(edit func(*Project)) Project {
		project := validProject()
		project.Tracks[0].Clips[0].Keyframes = map[string][]Keyframe{"opacity": {{F: 0, V: 0, E: EaseLinear}, {F: 10, V: 1, E: EaseLinear}}}
		if edit != nil {
			edit(&project)
		}
		return project
	}
	err := CheckProjectCapabilities(keyframed(nil), []string{"filter:sendcmd"})
	var capability *CapabilityError
	if !errors.Is(err, ErrFeatureUnavailable) || !errors.As(err, &capability) || capability.Feature != "keyframes" || capability.Missing != "filter:sendcmd" {
		t.Fatalf("keyframes without sendcmd = %v", err)
	}
	for name, c := range map[string]struct {
		project Project
		missing []string
	}{
		"sendcmd present": {keyframed(nil), []string{"filter:lut3d"}},
		"no keyframes":    {validProject(), []string{"filter:sendcmd"}},
		"disabled clip":   {keyframed(func(p *Project) { p.Tracks[0].Clips[0].Disabled = true }), []string{"filter:sendcmd"}},
		"hidden track":    {keyframed(func(p *Project) { p.Tracks[0].Hidden = true }), []string{"filter:sendcmd"}},
	} {
		if err := CheckProjectCapabilities(c.project, c.missing); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
