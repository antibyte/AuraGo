package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/videostudio"
)

// pictureTestProject mirrors internal/videostudio's effectTestProject: "v" is a 300-frame video on the video track
// (offset 10), "o" an image on the overlay track and "a" a song on the audio track, 60 frames each at 30 fps, plus the
// lut asset "grade".
func pictureTestProject() videostudio.Project {
	box := func(id, asset string) videostudio.Clip {
		return videostudio.Clip{ID: id, AssetID: asset, Duration: 60, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: videostudio.FitContain}
	}
	video, overlay := box("v", "video"), box("o", "logo")
	video.Offset = 10
	overlay.X, overlay.Y, overlay.Width, overlay.Height = 0.1, 0.1, 0.2, 0.2
	return videostudio.Project{
		Version: videostudio.CurrentVersion, Name: "Picture", Width: 1280, Height: 720, FPS: 30,
		Assets: []videostudio.Asset{
			{ID: "video", Name: "video.mp4", Kind: videostudio.AssetVideo, DurationFrames: 300, Width: 640, Height: 360},
			{ID: "logo", Name: "logo.png", Kind: videostudio.AssetImage, Width: 64, Height: 64},
			{ID: "song", Name: "song.mp3", Kind: videostudio.AssetAudio, DurationFrames: 600, HasAudio: true},
			{ID: "grade", Name: "grade.cube", Kind: videostudio.AssetLUT, LUTSize: 17},
		},
		Tracks: []videostudio.Track{
			{ID: "video", Name: "Video", Kind: videostudio.TrackVideo, Clips: []videostudio.Clip{video}},
			{ID: "overlay", Name: "Overlay", Kind: videostudio.TrackOverlay, Clips: []videostudio.Clip{overlay}},
			{ID: "audio", Name: "Audio", Kind: videostudio.TrackAudio, Clips: []videostudio.Clip{{ID: "a", AssetID: "song", Duration: 60, Volume: 1}}},
		},
	}
}

// pictureClipEdit edits the clip with the given id.
func pictureClipEdit(id string, edit func(*videostudio.Clip)) func(*videostudio.Project) {
	return func(project *videostudio.Project) {
		for ti := range project.Tracks {
			for ci := range project.Tracks[ti].Clips {
				if project.Tracks[ti].Clips[ci].ID == id {
					edit(&project.Tracks[ti].Clips[ci])
				}
			}
		}
	}
}

// validTimeline refuses what Go's Validate refuses for crop, flips, effects (plan 2c) and for blend modes, masks and
// the chroma_key entry (plan 2d, the rows of TestValidateCompositing without its non-finite numbers, which JSON
// cannot carry; index §4.4). Go validates
// every case first; then the same JSON goes through VideoStudioTimeline.validTimeline, and so through
// VideoStudioEffects.validPicture and validAnimation. The projects reach Node through a file: together they exceed
// the Windows command-line limit of node -e.
func TestVideoStudioPictureValidationMatchesGo(t *testing.T) {
	key := func(f int, v float64) videostudio.Keyframe {
		return videostudio.Keyframe{F: f, V: v, E: videostudio.EaseLinear}
	}
	keys := func(n int, v float64) []videostudio.Keyframe {
		list := make([]videostudio.Keyframe, n)
		for i := range list {
			list[i] = key(i, v)
		}
		return list
	}
	// baseKeys spreads total keys over eight base paths, at most 64 per path.
	baseKeys := func(total int) map[string][]videostudio.Keyframe {
		out := map[string][]videostudio.Keyframe{}
		for _, path := range []string{"opacity", "x", "y", "rotation", "crop.l", "crop.t", "crop.r", "crop.b"} {
			n := min(64, total)
			total -= n
			value := 0.0
			if path == "opacity" {
				value = 1
			}
			if n > 0 {
				out[path] = keys(n, value)
			}
		}
		return out
	}
	defaults := func(id string) map[string]any {
		spec, _ := videostudio.EffectByID(id)
		params := map[string]any{}
		for _, param := range spec.Params {
			params[param.ID] = param.Default
		}
		return params
	}
	with := func(params map[string]any, name string, value any) map[string]any {
		out := map[string]any{}
		for k, v := range params {
			out[k] = v
		}
		out[name] = value
		return out
	}
	entry := func(id string, params map[string]any) videostudio.EffectInstance {
		return videostudio.EffectInstance{ID: id, On: true, Params: params}
	}
	stack := func(entries ...videostudio.EffectInstance) func(*videostudio.Project) {
		return pictureClipEdit("v", func(c *videostudio.Clip) { c.Effects = entries })
	}
	crop := func(value videostudio.Crop) func(*videostudio.Project) {
		return pictureClipEdit("v", func(c *videostudio.Clip) { c.Crop = &value })
	}
	sharpens := func(n int) []videostudio.EffectInstance {
		list := make([]videostudio.EffectInstance, n)
		for i := range list {
			list[i] = videostudio.EffectInstance{ID: "sharpen", On: i%2 == 0, Params: map[string]any{}}
		}
		return list
	}
	strength := func(list []videostudio.Keyframe) videostudio.EffectInstance {
		return videostudio.EffectInstance{ID: "sharpen", On: true, Params: map[string]any{}, Keyframes: map[string][]videostudio.Keyframe{"strength": list}}
	}
	color := defaults("color")
	blendOn := func(id, mode string) func(*videostudio.Project) {
		return pictureClipEdit(id, func(c *videostudio.Clip) { c.Blend = mode })
	}
	validMask := func() *videostudio.Mask {
		return &videostudio.Mask{Shape: videostudio.MaskEllipse, X: 0.25, Y: 0.2, W: 0.5, H: 0.6, Rotation: 15, Feather: 0.04}
	}
	same := func(*videostudio.Mask) {}
	maskV := func(edit func(*videostudio.Mask), keys map[string][]videostudio.Keyframe) func(*videostudio.Project) {
		return pictureClipEdit("v", func(c *videostudio.Clip) {
			mask := validMask()
			edit(mask)
			c.Mask, c.Keyframes = mask, keys
		})
	}
	keyEntry := func(params map[string]any, keys map[string][]videostudio.Keyframe) func(*videostudio.Project) {
		return stack(videostudio.EffectInstance{ID: "chroma_key", On: true, Params: params, Keyframes: keys})
	}
	keyDefaults := defaults("chroma_key")
	cases := []struct {
		name  string
		edit  func(*videostudio.Project)
		valid bool
	}{
		{"no picture fields", func(*videostudio.Project) {}, true},
		{"crop and a flip on the video clip", pictureClipEdit("v", func(c *videostudio.Clip) { c.Crop, c.FlipH = &videostudio.Crop{L: 0.1, R: 0.1, B: 0.05}, true }), true},
		{"l + r at 0.95 within float rounding", crop(videostudio.Crop{L: 0.9, R: 0.05}), true},
		{"both flips on the overlay image", pictureClipEdit("o", func(c *videostudio.Clip) { c.FlipH, c.FlipV = true, true }), true},
		{"crop keyframes", pictureClipEdit("v", func(c *videostudio.Clip) {
			c.Keyframes = map[string][]videostudio.Keyframe{"crop.l": {key(0, 0), key(20, 0.95)}}
		}), true},
		{"a complete colour entry with keyframes", stack(videostudio.EffectInstance{ID: "color", On: true, Params: color, Keyframes: map[string][]videostudio.Keyframe{"exposure": {key(-5, -3), key(10, 3)}}}), true},
		{"a built-in look, the project LUT and a pass-through LUT that is off", stack(
			entry("lut", map[string]any{"lut": "builtin:teal_orange", "intensity": 0.6}),
			entry("lut", map[string]any{"lut": "grade"}),
			videostudio.EffectInstance{ID: "lut", Params: map[string]any{"lut": ""}}), true},
		{"eight effects with empty params", stack(sharpens(8)...), true},
		{"512 keys with one effect key", pictureClipEdit("v", func(c *videostudio.Clip) {
			c.Keyframes, c.Effects = baseKeys(511), []videostudio.EffectInstance{strength([]videostudio.Keyframe{key(0, 0.5)})}
		}), true},
		{"crop on the audio clip", pictureClipEdit("a", func(c *videostudio.Clip) { c.Crop = &videostudio.Crop{L: 0.1} }), false},
		{"a flip on the audio clip", pictureClipEdit("a", func(c *videostudio.Clip) { c.FlipV = true }), false},
		{"effects on the audio clip", pictureClipEdit("a", func(c *videostudio.Clip) { c.Effects = sharpens(1) }), false},
		{"an all-zero crop", crop(videostudio.Crop{}), false},
		{"a side above 0.95", crop(videostudio.Crop{T: 0.96}), false},
		{"a negative side", crop(videostudio.Crop{L: -0.1, R: 0.2}), false},
		{"t + b above 0.95", crop(videostudio.Crop{T: 0.5, B: 0.46}), false},
		{"a crop keyframe above 0.95", pictureClipEdit("v", func(c *videostudio.Clip) {
			c.Keyframes = map[string][]videostudio.Keyframe{"crop.r": {key(0, 0.96)}}
		}), false},
		{"nine effects", stack(sharpens(9)...), false},
		{"an unknown effect", stack(entry("glow", map[string]any{})), false},
		{"params missing", stack(videostudio.EffectInstance{ID: "blur", On: true}), false},
		{"an unknown parameter", stack(entry("blur", map[string]any{"sigma": 2.0})), false},
		{"a number given as text", stack(entry("color", with(color, "exposure", "0.5"))), false},
		{"exposure above 3", stack(entry("color", with(color, "exposure", 3.5))), false},
		{"a LUT ref to the video asset", stack(entry("lut", map[string]any{"lut": "video"})), false},
		{"an unknown built-in look", stack(entry("lut", map[string]any{"lut": "builtin:sepia"})), false},
		{"keyframes on the blur radius", stack(videostudio.EffectInstance{ID: "blur", On: true, Params: map[string]any{}, Keyframes: map[string][]videostudio.Keyframe{"radius": {key(0, 0.01)}}}), false},
		{"an empty effect keyframe list", stack(strength([]videostudio.Keyframe{})), false},
		{"an effect keyframe above its range", stack(strength([]videostudio.Keyframe{key(0, 1.5)})), false},
		{"unsorted effect keyframes", stack(strength([]videostudio.Keyframe{key(5, 0.5), key(3, 0.5)})), false},
		{"513 keys with one effect key", pictureClipEdit("v", func(c *videostudio.Clip) {
			c.Keyframes, c.Effects = baseKeys(512), []videostudio.EffectInstance{strength([]videostudio.Keyframe{key(0, 0.5)})}
		}), false},
		{"multiply on the video clip", blendOn("v", "multiply"), true},
		{"dodge on the overlay image", blendOn("o", "dodge"), true},
		{"an ellipse mask with keys on every keyframable path", maskV(same, map[string][]videostudio.Keyframe{
			"mask.x": {key(0, -1), key(20, 1)}, "mask.y": {key(0, 0.1)}, "mask.w": {key(0, 0.01), key(30, 2)},
			"mask.h": {key(5, 0.5)}, "mask.rotation": {key(0, -360), key(10, 360)}, "mask.feather": {key(0, 0), key(10, 0.5)},
		}), true},
		{"a rect mask at its bounds on the overlay image", pictureClipEdit("o", func(c *videostudio.Clip) {
			c.Mask = &videostudio.Mask{Shape: videostudio.MaskRect, X: -1, Y: 1, W: 2, H: 0.01, Rotation: -360, Feather: 0.5, Invert: true}
		}), true},
		{"chroma_key at its defaults with keyed similarity and smoothness", keyEntry(keyDefaults, map[string][]videostudio.Keyframe{
			"similarity": {key(0, 0.01), key(30, 1)}, "smoothness": {key(0, 0)},
		}), true},
		{"an upper-case key colour", keyEntry(with(keyDefaults, "color", "#00B140"), nil), true},
		{"blend normal", blendOn("v", "normal"), false},
		{"an unknown blend mode", blendOn("v", "glow"), false},
		{"a blend mode on the audio clip", blendOn("a", "multiply"), false},
		{"a mask on the audio clip", pictureClipEdit("a", func(c *videostudio.Clip) { c.Mask = validMask() }), false},
		{"mask keys without a mask", pictureClipEdit("v", func(c *videostudio.Clip) {
			c.Keyframes = map[string][]videostudio.Keyframe{"mask.x": {key(0, 0.1)}}
		}), false},
		{"a mask shape star", maskV(func(m *videostudio.Mask) { m.Shape = "star" }, nil), false},
		{"a mask without a shape", maskV(func(m *videostudio.Mask) { m.Shape = "" }, nil), false},
		{"mask x above 1", maskV(func(m *videostudio.Mask) { m.X = 1.5 }, nil), false},
		{"mask y below -1", maskV(func(m *videostudio.Mask) { m.Y = -1.5 }, nil), false},
		{"mask w below 0.01", maskV(func(m *videostudio.Mask) { m.W = 0.005 }, nil), false},
		{"mask h above 2", maskV(func(m *videostudio.Mask) { m.H = 2.5 }, nil), false},
		{"mask rotation above 360", maskV(func(m *videostudio.Mask) { m.Rotation = 400 }, nil), false},
		{"mask feather above 0.5", maskV(func(m *videostudio.Mask) { m.Feather = 0.6 }, nil), false},
		{"a mask key out of range", maskV(same, map[string][]videostudio.Keyframe{"mask.w": {key(0, 3)}}), false},
		{"a mask key below the 1 % size", maskV(same, map[string][]videostudio.Keyframe{"mask.h": {key(0, 0.005)}}), false},
		{"a key colour that is not #rrggbb", keyEntry(with(keyDefaults, "color", "green"), nil), false},
		{"similarity below 0.01", keyEntry(with(keyDefaults, "similarity", 0.0), nil), false},
		{"keyframes on the key colour", keyEntry(keyDefaults, map[string][]videostudio.Keyframe{"color": {key(0, 0)}}), false},
	}
	projects := make([]videostudio.Project, len(cases))
	for i, c := range cases {
		projects[i] = pictureTestProject()
		c.edit(&projects[i])
		if err := videostudio.Validate(projects[i]); (err == nil) != c.valid {
			t.Fatalf("%s: Go Validate = %v, want valid %v", c.name, err, c.valid)
		}
	}
	data, err := json.Marshal(projects)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "projects.json")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	quoted, _ := json.Marshal(file)
	output := runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-timeline.js');
const projects = JSON.parse(require('node:fs').readFileSync(`+string(quoted)+`, 'utf8'));
process.stdout.write(JSON.stringify(projects.map(project => window.VideoStudioTimeline.validTimeline(project))));
`)
	var got []bool
	if err := json.Unmarshal(output, &got); err != nil || len(got) != len(cases) {
		t.Fatalf("validTimeline results %s: %v", output, err)
	}
	for i, c := range cases {
		if got[i] != c.valid {
			t.Errorf("%s: validTimeline = %v, Go Validate valid = %v", c.name, got[i], c.valid)
		}
	}
}

// clipState in 2c (index §4.4) evaluates the picture parts at the frame:
//   - crop from keyframes, else the field, else 0;
//   - the flips as booleans;
//   - one entry per effect that is on and known, available or not, with every parameter. Numbers go through valueAt;
//     the lut ref comes as a string; a value of the wrong type takes its default.
//
// The picture helpers of the registry are checked alongside.
func TestVideoStudioClipStatePicture(t *testing.T) {
	runVideoStudioEffectsScript(t, `
const near = (got, want, label) => assert(Math.abs(got - want) < 1e-9, label + ': ' + got + ' != ' + want);
const project = {fps: 25};
const clip = {start: 100, offset: 20, duration: 60, crop: {l: 0.1, t: 0.05, r: 0.2, b: 0}, flip_h: true,
  keyframes: {'crop.l': [{f: 20, v: 0, e: 'linear'}, {f: 40, v: 0.2, e: 'linear'}]},
  effects: [
    {id: 'color', on: true, params: Object.assign(E.defaults('color'), {exposure: 0.5}), keyframes: {contrast: [{f: 20, v: 1, e: 'linear'}, {f: 30, v: 2, e: 'linear'}]}},
    {id: 'blur', on: false, params: {radius: 0.02}},
    {id: 'retired', on: true, params: {}},
    {id: 'lut', on: true, params: {lut: 'builtin:mono'}},
    {id: 'vignette', on: true, params: {amount: 'strong'}}
  ]};
const st = E.clipState(clip, 105, project);
near(st.crop.l, 0.05, 'crop.l follows its keyframes at source 25');
assert.deepEqual([st.crop.t, st.crop.r, st.crop.b], [0.05, 0.2, 0], 'the other sides come from the field');
assert.equal(st.flipH, true);
assert.equal(st.flipV, false);
assert.deepEqual(st.effects.map(e => [e.index, e.id]), [[0, 'color'], [3, 'lut'], [4, 'vignette']], 'entries that are on and known, with their stack index');
near(st.effects[0].values.contrast, 1.5, 'entry keyframes at the source position');
assert.equal(st.effects[0].values.exposure, 0.5);
assert.equal(st.effects[0].values.temperature, 6500);
assert.deepEqual(Object.keys(st.effects[0].values), E.effect('color').params.map(p => p.id), 'every parameter, in registry order');
assert.deepEqual(st.effects[1].values, {lut: 'builtin:mono', intensity: 1}, 'the lut ref as a string, a missing number at its default');
assert.deepEqual(st.effects[2].values, {amount: 0.5, size: 1}, 'a value of the wrong type takes the default');
// Unavailable effects are still drawn: the state does not depend on the FFmpeg probe.
E.adopt(E.BUILTIN, {effects: ['lut'], transitions: [], blend_modes: [], anim_presets: []});
assert.deepEqual(E.clipState(clip, 105, project).effects.map(e => e.id), ['color', 'lut', 'vignette']);
assert.equal(E.effectAvailable('lut'), false);
assert.equal(E.effectAvailable('color'), true);
assert.equal(E.effectAvailable('nope'), false);
assert.deepEqual(E.effects().map(e => [e.id, e.available]), [['color', true], ['lut', false], ['blur', true], ['sharpen', true], ['vignette', true], ['chroma_key', true]]);
E.adopt(undefined, undefined);
assert.deepEqual(E.looks(), ['warm', 'cool', 'teal_orange', 'mono', 'faded_film', 'high_contrast', 'vintage', 'bleach_bypass'].map(id => ({id, label_key: 'look_' + id})));
assert.equal(E.LIMITS.effectsPerClip, 8);
// Crop animation, the animatable paths of a clip and the spec of any path.
assert.equal(E.cropAnimated(clip), true);
assert.equal(E.cropAnimated({keyframes: {x: [{f: 0, v: 0, e: 'linear'}]}}), false);
assert.equal(E.cropAnimated({}), false);
const paths = E.animatablePaths(clip);
assert.deepEqual(paths.slice(0, 10), ['x', 'y', 'width', 'height', 'rotation', 'opacity', 'crop.l', 'crop.t', 'crop.r', 'crop.b']);
assert(paths.includes('effects.0.exposure') && paths.includes('effects.0.gain') && paths.includes('effects.3.intensity') && paths.includes('effects.4.size'));
assert(!paths.includes('effects.1.radius') && !paths.includes('effects.3.lut') && !paths.some(p => p.startsWith('effects.2.')), 'no blur radius, no LUT ref, no unknown entry');
assert.equal(E.paramSpec(clip, 'effects.0.exposure').unit, 'ev');
assert.equal(E.paramSpec(clip, 'crop.l').max, 0.95);
assert.equal(E.paramSpec(clip, 'effects.2.amount'), null);
assert.equal(E.paramSpec(clip, 'effects.9.exposure'), null);
assert.equal(E.paramSpec(clip, 'effects.01.exposure'), null);
console.log('ok');
`)
}
