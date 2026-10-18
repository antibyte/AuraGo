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
// returned path is a regular file, which is all resolveBinary and the memo key need; it is never run. The fadegrays
// self-test (plan 2e D2) renders exactly what fadegraysModel predicts, so a listed fadegrays stays available and a probe
// that lists it costs two more commands.
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
		if slices.Contains(args, fadegraysSelfTestFilter) {
			_, _, want := fadegraysSelfTestCase()
			return want, nil
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
		if slices.Contains(effect.Filters, "lut3d") || slices.Contains(effect.Filters, "blend") {
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
	// A failed fadegrays self-test (plan 2e D2) greys out exactly that type; the export refuses it by the transition rule.
	if got := registry.Unavailable([]string{"xfade:fadegrays"}).Transitions; !slices.Equal(got, []string{"fadegrays"}) {
		t.Fatalf("xfade:fadegrays greys out %v, want [fadegrays]", got)
	}
	err = CheckProjectCapabilities(capabilityTestProject("fadegrays"), []string{"xfade:fadegrays"})
	var capability *CapabilityError
	if !errors.As(err, &capability) || capability.Feature != "transition:fadegrays" || capability.Missing != "xfade:fadegrays" {
		t.Fatalf("a fadegrays transition without xfade:fadegrays = %v", err)
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
	// -filters, -h filter=xfade and the two fadegrays self-test renders (plan 2e D2).
	missing, err := ProbeCapabilities(ctx, path)
	if err != nil || !slices.Equal(missing, []string{"xfade:fadeblack"}) || *calls != 4 {
		t.Fatalf("first probe = %v, %v after %d commands", missing, err, *calls)
	}
	missing[0] = "changed by the caller"
	again, err := ProbeCapabilities(ctx, path)
	if err != nil || !slices.Equal(again, []string{"xfade:fadeblack"}) || *calls != 4 {
		t.Fatalf("second probe = %v, %v after %d commands; want the memoized result", again, err, *calls)
	}
	// A replaced binary (another size) is probed again, the self-test included.
	if err := os.WriteFile(path, []byte("a newer fake ffmpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ProbeCapabilities(ctx, path); err != nil || *calls != 8 {
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

// The installed FFmpeg lacks nothing phase 2 needs. Exception: where the fadegrays self-test fails (FFmpeg 5.1, plan 2e
// D2), exactly xfade:fadegrays is missing.
func TestProbeCapabilitiesLocalFFmpegMissesNothing(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	resetCapabilityMemo()
	ctx := context.Background()
	missing, err := ProbeCapabilities(ctx, ffmpeg)
	if err != nil {
		t.Fatalf("ProbeCapabilities(%s): %v", ffmpeg, err)
	}
	fadegrays, err := fadegraysSelfTest(ctx, ffmpeg)
	if err != nil {
		t.Fatalf("fadegraysSelfTest(%s): %v", ffmpeg, err)
	}
	want := []string{}
	if !fadegrays {
		want = []string{"xfade:fadegrays"}
	}
	if !slices.Equal(missing, want) {
		t.Fatalf("ProbeCapabilities(%s) = %v; want %v (the fadegrays self-test passed: %t)", ffmpeg, missing, want, fadegrays)
	}
	if status := CheckRuntime(ctx, ffmpeg); !status.Ready || !slices.Equal(status.Missing, want) {
		t.Fatalf("CheckRuntime() = %+v, want Missing %v", status, want)
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

func TestCheckProjectCapabilitiesEffects(t *testing.T) {
	color := EffectInstance{ID: "color", On: true, Params: map[string]any{"exposure": 0.5}}
	lut := EffectInstance{ID: "lut", On: true, Params: map[string]any{"lut": "builtin:warm", "intensity": 0.6}}
	keyedColor := EffectInstance{ID: "color", On: true, Params: map[string]any{}, Keyframes: map[string][]Keyframe{"exposure": {{F: 0, V: 0, E: EaseLinear}, {F: 10, V: 1, E: EaseLinear}}}}
	with := func(edit func(*Project), effects ...EffectInstance) Project {
		project := validProject()
		project.Tracks[0].Clips[0].Effects = effects
		if edit != nil {
			edit(&project)
		}
		return project
	}
	cropKeys := func(p *Project) {
		p.Tracks[0].Clips[0].Keyframes = map[string][]Keyframe{"crop.l": {{F: 0, V: 0, E: EaseLinear}, {F: 10, V: 0.2, E: EaseLinear}}}
	}
	var capability *CapabilityError
	for name, c := range map[string]struct {
		project       Project
		missing       []string
		feature, need string
	}{
		"colour without lutrgb": {with(nil, color), []string{"filter:lutrgb"}, "effect:color", "filter:lutrgb"},
		"lut without blend":     {with(nil, color, lut), []string{"filter:gblur", "filter:blend"}, "effect:lut", "filter:blend"},
		"effect keyframes":      {with(nil, keyedColor), []string{"filter:sendcmd"}, "keyframes", "filter:sendcmd"},
		"crop keyframes":        {with(cropKeys), []string{"filter:sendcmd"}, "keyframes", "filter:sendcmd"},
	} {
		err := CheckProjectCapabilities(c.project, c.missing)
		if !errors.Is(err, ErrFeatureUnavailable) || !errors.As(err, &capability) || capability.Feature != c.feature || capability.Missing != c.need {
			t.Errorf("%s = %v, want %s needs %s", name, err, c.feature, c.need)
		}
	}
	offKeyed := keyedColor
	offKeyed.On = false
	for name, c := range map[string]struct {
		project Project
		missing []string
	}{
		"filters present":         {with(nil, color, lut), []string{"filter:gblur", "filter:cas"}},
		"effect off":              {with(nil, EffectInstance{ID: "color", On: false, Params: map[string]any{"exposure": 0.5}}), []string{"filter:lutrgb"}},
		"keys of an entry is off": {with(nil, offKeyed), []string{"filter:sendcmd"}},
		"disabled clip":           {with(func(p *Project) { p.Tracks[0].Clips[0].Disabled = true }, color), []string{"filter:lutrgb"}},
		"hidden track":            {with(func(p *Project) { p.Tracks[0].Hidden = true }, color), []string{"filter:lutrgb"}},
	} {
		if err := CheckProjectCapabilities(c.project, c.missing); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A mask needs geq, blend and loop (index §4.3); masks get no unavailable entry, so the export refuses. Disabled clips
// and hidden tracks do not count. loop is probed with the phase 2 filters.
func TestCheckProjectCapabilitiesMask(t *testing.T) {
	masked := func(edit func(*Project)) Project {
		project := validProject()
		project.Tracks[0].Clips[0].Mask = &Mask{Shape: MaskEllipse, X: 0.25, Y: 0.25, W: 0.5, H: 0.5}
		if edit != nil {
			edit(&project)
		}
		return project
	}
	var capability *CapabilityError
	for _, need := range []string{"filter:geq", "filter:blend", "filter:loop"} {
		err := CheckProjectCapabilities(masked(nil), []string{need})
		if !errors.Is(err, ErrFeatureUnavailable) || !errors.As(err, &capability) || capability.Feature != "mask" || capability.Missing != need {
			t.Errorf("mask without %s = %v", need, err)
		}
	}
	for name, c := range map[string]struct {
		project Project
		missing []string
	}{
		"filters present": {masked(nil), []string{"filter:lut3d", "filter:maskedmerge"}},
		"no mask":         {validProject(), []string{"filter:geq", "filter:loop"}},
		"disabled clip":   {masked(func(p *Project) { p.Tracks[0].Clips[0].Disabled = true }), []string{"filter:geq"}},
		"hidden track":    {masked(func(p *Project) { p.Tracks[0].Hidden = true }), []string{"filter:loop"}},
	} {
		if err := CheckProjectCapabilities(c.project, c.missing); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if !slices.Contains(capabilityFilters(), "loop") {
		t.Fatal("capabilityFilters() lacks loop")
	}
}

// blendModeIDs lists the registry blend modes with an FFmpeg name (all but normal), in registry order.
func blendModeIDs() []string {
	var ids []string
	for _, mode := range EffectRegistry().BlendModes {
		if mode.Blend != "" {
			ids = append(ids, mode.ID)
		}
	}
	return ids
}

// A blend mode composites through blend, maskedmerge and alphaextract (index §4.2, §4.22). Without any of them the
// editor greys out all 13 modes, never normal, and the probe covers all three.
func TestRegistryUnavailableBlendModes(t *testing.T) {
	registry := EffectRegistry()
	modes := blendModeIDs()
	if len(modes) != 13 || len(registry.BlendModes) != 14 || registry.BlendModes[0].ID != BlendNormal || registry.BlendModes[0].Blend != "" {
		t.Fatalf("blend modes = %+v", registry.BlendModes)
	}
	for _, filter := range []string{"blend", "maskedmerge", "alphaextract"} {
		if got := registry.Unavailable([]string{"filter:" + filter}).BlendModes; !slices.Equal(got, modes) {
			t.Errorf("without %s: unavailable blend modes %v, want %v", filter, got, modes)
		}
		if !slices.Contains(capabilityFilters(), filter) {
			t.Errorf("capabilityFilters() lacks %q", filter)
		}
	}
	for _, missing := range [][]string{nil, {"filter:lut3d", "filter:geq", "filter:sendcmd", "xfade:fadeblack"}} {
		if got := registry.Unavailable(missing).BlendModes; len(got) != 0 {
			t.Errorf("Unavailable(%v).BlendModes = %v, want none", missing, got)
		}
	}
}

// The export refuses a blend mode it cannot composite (index §4.3). Only clips that render count.
func TestCheckProjectCapabilitiesBlend(t *testing.T) {
	blended := func(edit func(*Project)) Project {
		project := validProject()
		project.Tracks[0].Clips[0].Blend = "screen"
		if edit != nil {
			edit(&project)
		}
		return project
	}
	if err := Validate(blended(nil)); err != nil {
		t.Fatalf("blended project invalid: %v", err)
	}
	var capability *CapabilityError
	for _, filter := range []string{"blend", "maskedmerge", "alphaextract"} {
		err := CheckProjectCapabilities(blended(nil), []string{"filter:" + filter})
		if !errors.Is(err, ErrFeatureUnavailable) || !errors.As(err, &capability) || capability.Feature != "blend:screen" || capability.Missing != "filter:"+filter {
			t.Errorf("screen without %s = %v, want blend:screen needs filter:%s", filter, err, filter)
		}
	}
	for name, c := range map[string]struct {
		project Project
		missing []string
	}{
		"filters present": {blended(nil), []string{"filter:lut3d", "filter:geq"}},
		"normal clip":     {validProject(), []string{"filter:blend", "filter:maskedmerge", "filter:alphaextract"}},
		"disabled clip":   {blended(func(p *Project) { p.Tracks[0].Clips[0].Disabled = true }), []string{"filter:blend"}},
		"hidden track":    {blended(func(p *Project) { p.Tracks[0].Hidden = true }), []string{"filter:maskedmerge"}},
	} {
		if err := CheckProjectCapabilities(c.project, c.missing); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// An animation preset is unavailable when one of its Filters is missing (index §4.2), and the probe lists every one of them
// (capabilityFilters). A missing filter greys exactly the presets that list it, in menu order; other features grey none.
func TestRegistryUnavailableAnimPresets(t *testing.T) {
	registry := EffectRegistry()
	if len(registry.AnimPresets) != 8 {
		t.Fatalf("registry has %d animation presets, want 8", len(registry.AnimPresets))
	}
	for _, preset := range registry.AnimPresets {
		if len(preset.Filters) == 0 {
			t.Fatalf("preset %s has no filters", preset.ID)
		}
		for _, filter := range preset.Filters {
			if !slices.Contains(capabilityFilters(), filter) {
				t.Errorf("capabilityFilters() lacks %q of preset %s", filter, preset.ID)
			}
			want := []string{}
			for _, other := range registry.AnimPresets {
				if slices.Contains(other.Filters, filter) {
					want = append(want, other.ID)
				}
			}
			if got := registry.Unavailable([]string{"filter:" + filter}).AnimPresets; !slices.Equal(got, want) {
				t.Errorf("without %s: unavailable presets %v, want %v", filter, got, want)
			}
		}
	}
	for name, c := range map[string]struct {
		missing, want []string
	}{
		"blend":          {[]string{"filter:blend"}, []string{"wipe"}},
		"gblur":          {[]string{"filter:gblur"}, []string{"blur_in"}},
		"sendcmd":        {[]string{"filter:sendcmd"}, []string{"fade", "slide_up", "slide_down", "slide_left", "slide_right", "pop", "blur_in"}},
		"nothing":        {nil, []string{}},
		"other features": {[]string{"filter:lut3d", "filter:cas", "xfade:fadegrays", "bogus"}, []string{}},
	} {
		if got := registry.Unavailable(c.missing).AnimPresets; !slices.Equal(got, c.want) {
			t.Errorf("%s: unavailable presets %v, want %v", name, got, c.want)
		}
	}
}

// The export refuses an in or out animation that plays when a filter of its preset is missing (index §4.3). Only an
// effective span of at least one frame counts, on an enabled clip of a visible visual track. An unavailable preset still
// validates.
func TestCheckProjectCapabilitiesAnim(t *testing.T) {
	animated := func(edit func(*Clip)) Project {
		project := validProject()
		clip := &project.Tracks[0].Clips[0]
		clip.AnimIn = &ClipAnim{Preset: "blur_in", Duration: 10}
		clip.AnimOut = &ClipAnim{Preset: "wipe", Duration: 8}
		if edit != nil {
			edit(clip)
		}
		return project
	}
	if err := Validate(animated(nil)); err != nil {
		t.Fatalf("animated project invalid: %v", err)
	}
	var capability *CapabilityError
	for name, c := range map[string]struct {
		project       Project
		missing       []string
		feature, need string
	}{
		"blur_in without gblur":       {animated(nil), []string{"filter:gblur"}, "anim:blur_in", "filter:gblur"},
		"blur_in without premultiply": {animated(nil), []string{"filter:premultiply"}, "anim:blur_in", "filter:premultiply"},
		"wipe without blend":          {animated(nil), []string{"filter:blend"}, "anim:wipe", "filter:blend"},
		"fade without sendcmd": {animated(func(c *Clip) { c.AnimIn, c.AnimOut = &ClipAnim{Preset: "fade", Duration: 10}, nil }),
			[]string{"filter:sendcmd"}, "anim:fade", "filter:sendcmd"},
		"pop out without colorchannelmixer": {animated(func(c *Clip) { c.AnimIn, c.AnimOut = nil, &ClipAnim{Preset: "pop", Duration: 10} }),
			[]string{"filter:colorchannelmixer"}, "anim:pop", "filter:colorchannelmixer"},
	} {
		err := CheckProjectCapabilities(c.project, c.missing)
		if !errors.Is(err, ErrFeatureUnavailable) || !errors.As(err, &capability) || capability.Feature != c.feature || capability.Missing != c.need {
			t.Errorf("%s = %v, want %s needs %s", name, err, c.feature, c.need)
		}
	}
	hidden := animated(nil)
	hidden.Tracks[0].Hidden = true
	for name, c := range map[string]struct {
		project Project
		missing []string
	}{
		"filters present":           {animated(nil), []string{"filter:lut3d", "filter:cas", "xfade:fadegrays"}},
		"no animation":              {validProject(), []string{"filter:gblur", "filter:blend", "filter:sendcmd"}},
		"an effective span of zero": {animated(func(c *Clip) { c.Duration = 1 }), []string{"filter:gblur", "filter:blend"}},
		"disabled clip":             {animated(func(c *Clip) { c.Disabled = true }), []string{"filter:gblur"}},
		"hidden track":              {hidden, []string{"filter:blend"}},
	} {
		if err := CheckProjectCapabilities(c.project, c.missing); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
