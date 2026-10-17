package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Effect stack operations (index §4.16, spec §5.4, §8.1).
//   - New entries carry complete params at their registry defaults, are on, and go to the end of the stack, up to
//     LIMITS.effectsPerClip; the ninth refuses with limit.
//   - Entries move, switch and reset (numbers and keys; the on state and the LUT stay), and are removed.
//   - Params, crop and flips change through the same operations.
//   - Audio clips refuse with kind, locked tracks with locked, unknown LUTs with media.
func TestVideoStudioEffectStackOps(t *testing.T) {
	runVideoStudioKeyframesScript(t, `
const fresh = () => {
  const p = project([
    track('v1','video',[clip('c',V,0,100,{offset:100})]),
    track('o1','overlay',[clip('t',I,0,60)]),
    track('a1','audio',[clip('m',A,0,100)])
  ]);
  p.assets.push({id:'lut1', name:'Film.cube', kind:'lut', duration_frames:0, lut_size:17});
  return p;
};
let p = fresh();
const c = () => at(p, 'c');
const cropNear = (want, message) => ['l', 't', 'r', 'b'].forEach(side => assert(Math.abs(c().crop[side] - want[side]) < 1e-6, message + ' (' + side + '): ' + JSON.stringify(c().crop)));

// Add: on, complete params at the registry defaults, at the end of the stack.
assert.equal(K.addEffect(p, 'c', 'color'), true);
assert.deepEqual(c().effects, [{id: 'color', on: true, params: FX.defaults('color')}]);
assert.deepEqual(Object.keys(c().effects[0].params).sort(), ['contrast', 'exposure', 'gain', 'gain_x', 'gain_y', 'gamma', 'gamma_x', 'gamma_y', 'lift', 'lift_x', 'lift_y', 'saturation', 'temperature', 'tint']);
assert.equal(c().effects[0].params.temperature, 6500);
assert.equal(K.addEffect(p, 'c', 'lut', {lut: 'builtin:teal_orange', intensity: 0.6}), true, 'the library sets a look directly');
assert.deepEqual(c().effects[1], {id: 'lut', on: true, params: {intensity: 0.6, lut: 'builtin:teal_orange'}});
assert.equal(K.addEffect(p, 'c', 'lut', {lut: 'lut1'}), true, 'a LUT asset of the project');
assert.equal(K.addEffect(p, 'c', 'lut', {lut: 'missing'}), 'media', 'a LUT the project does not hold');
assert.equal(K.addEffect(p, 'c', 'lut', {lut: 'builtin:nope'}), 'media', 'registry looks only');
assert.equal(K.addEffect(p, 'c', 'color', {bogus: 1}), 'nothing', 'unknown params refuse');
assert.equal(K.addEffect(p, 'c', 'sparkle'), 'nothing', 'unknown effects refuse');
assert.equal(K.addEffect(p, 'c', 'blur', {radius: 1}), true);
assert.equal(c().effects[3].params.radius, 0.05, 'numbers are clamped to the registry range');
valid(p);
for (const id of ['sharpen', 'vignette', 'color', 'blur']) assert.equal(K.addEffect(p, 'c', id), true);
assert.equal(c().effects.length, 8);
assert.equal(K.addEffect(p, 'c', 'color'), 'limit', 'at most 8 effects per clip');
valid(p);
assert.equal(K.addEffect(p, 'm', 'color'), 'kind', 'audio clips have no picture');
p.tracks[1].locked = true;
assert.equal(K.addEffect(p, 't', 'color'), 'locked');
p.tracks[1].locked = false;
assert.equal(K.addEffect(p, 't', 'vignette'), true, 'stills on overlay tracks take effects');
valid(p);

// Move: the processing order changes, and the entry keeps its params and keys.
p = fresh();
for (const id of ['color', 'lut', 'vignette']) K.addEffect(p, 'c', id);
c().effects[0].keyframes = {exposure: [kf(100, 0), kf(150, 1)]};
c().effects[0].params.contrast = 1.2;
valid(p);
assert.equal(K.moveEffect(p, 'c', 0, 2), true);
assert.deepEqual(c().effects.map(entry => entry.id), ['lut', 'vignette', 'color']);
assert.deepEqual(c().effects[2].keyframes, {exposure: [kf(100, 0), kf(150, 1)]});
assert.equal(c().effects[2].params.contrast, 1.2);
near(FX.valueAt(c(), 'effects.2.exposure', 25, p), 0.5, 'the moved entry keeps its keys on their content');
assert.equal(K.moveEffect(p, 'c', 2, 3), 'nothing', 'past the end');
assert.equal(K.moveEffect(p, 'c', 0, -1), 'nothing', 'before the start');
assert.equal(K.moveEffect(p, 'c', 1, 1), 'nothing', 'the same place');
assert.equal(K.moveEffect(p, 'c', 5, 0), 'nothing', 'no such entry');
// On and off.
assert.equal(K.setEffectOn(p, 'c', 2, false), true);
assert.equal(c().effects[2].on, false);
assert.equal(K.setEffectOn(p, 'c', 2, false), 'nothing');
assert.equal(FX.clipState(c(), 10, p).effects.some(item => item.id === 'color'), false, 'an entry turned off is not drawn');
assert.equal(K.setEffectOn(p, 'c', 2, true), true);
// Reset: numbers back to their defaults and the keys gone; the on state and the LUT choice stay.
assert.equal(K.setEffectParams(p, 'c', 0, {lut: 'builtin:warm', intensity: 0.3}, 10), true);
assert.equal(K.resetEffect(p, 'c', 0), true);
assert.deepEqual(c().effects[0], {id: 'lut', on: true, params: {intensity: 1, lut: 'builtin:warm'}});
assert.equal(K.resetEffect(p, 'c', 0), 'nothing', 'already at its defaults');
assert.equal(K.resetEffect(p, 'c', 2), true);
assert.deepEqual(c().effects[2], {id: 'color', on: true, params: FX.defaults('color')}, 'the color entry loses its keys and its contrast');
// Remove; the last entry leaves no empty list.
assert.equal(K.removeEffect(p, 'c', 1), true);
assert.deepEqual(c().effects.map(entry => entry.id), ['lut', 'color']);
assert.equal(K.removeEffect(p, 'c', 7), 'nothing');
assert.equal(K.removeEffect(p, 'c', 0), true);
assert.equal(K.removeEffect(p, 'c', 0), true);
assert.equal('effects' in c(), false);
valid(p);

// Params, flips and the crop.
p = fresh();
for (const id of ['color', 'lut', 'blur']) K.addEffect(p, 'c', id);
assert.equal(K.setEffectParams(p, 'c', 0, {exposure: 9, saturation: 0.5}, 10), true);
assert.deepEqual([c().effects[0].params.exposure, c().effects[0].params.saturation], [3, 0.5], 'numbers are clamped');
assert.equal(K.setEffectParams(p, 'c', 1, {lut: 'nope'}, 10), 'media');
assert.equal(K.setEffectParams(p, 'c', 1, {lut: 'lut1'}, 10), true);
assert.equal(K.setEffectParams(p, 'c', 2, {radius: 0.02}, 10), true, 'the blur radius is a static value');
assert.equal(K.setEffectParams(p, 'c', 2, {sigma: 1}, 10), 'nothing', 'unknown params refuse');
assert.equal(K.setEffectParams(p, 'c', 9, {radius: 0.02}, 10), 'nothing', 'no such entry');
valid(p);
assert.equal(K.setFlip(p, 'c', 'h', true), true);
assert.equal(c().flip_h, true);
assert.equal(K.setFlip(p, 'c', 'h', true), 'nothing');
assert.equal(K.setFlip(p, 'c', 'v', false), 'nothing');
assert.equal(K.setFlip(p, 'c', 'h', false), true);
assert.equal('flip_h' in c(), false, 'false flips are not stored');
assert.equal(K.setFlip(p, 'c', 'x', true), 'nothing');
assert.equal(K.setFlip(p, 'm', 'h', true), 'kind');
assert.equal(K.setCrop(p, 'c', {l: 0.25, r: 0.9}, 10), true);
cropNear({l: 0.25, t: 0, r: 0.7, b: 0}, 'both sides written: the second gives way');
assert.equal(K.setCrop(p, 'c', {t: 0.5}, 10), true);
assert.equal(K.setCrop(p, 'c', {b: 0.6}, 10), true);
cropNear({l: 0.25, t: 0.5, r: 0.7, b: 0.45}, 'a side written alone is clamped against the other');
valid(p);
assert.equal(K.setAnimated(p, 'c', 'crop.l', true, 10), true);
assert.deepEqual(c().keyframes, {'crop.l': [kf(110, 0.25)]});
assert.equal(K.setCrop(p, 'c', {l: 0.6}, 20), true);
assert.deepEqual(c().keyframes['crop.l'], [kf(110, 0.25), kf(120, 0.6)], 'an animated side takes a key');
assert.equal(c().crop.l, 0.25, 'and leaves its field alone');
assert.equal(K.setCrop(p, 'c', {r: 0.8}, 20), true);
cropNear({l: 0.15, t: 0.5, r: 0.8, b: 0.45}, 'the field of the animated left side gives way');
valid(p);
p = fresh();
assert.equal(K.setCrop(p, 'c', {l: 0.1}, 10), true);
assert.equal(K.setCrop(p, 'c', {l: 0}, 10), true);
assert.equal('crop' in c(), false, 'a crop of four zeros is removed');
assert.equal(K.setCrop(p, 'c', {x: 0.1}, 10), 'nothing', 'crop sides only');
assert.equal(K.setCrop(p, 'm', {l: 0.1}, 10), 'kind');
`)
}

// Keyframe operations on crop sides and effect params (index §4.18, spec §5.8).
//   - Every 2b operation works on effects.<i>.<param> and crop.l: stopwatch, diamond, auto-key, ◀▶, moves, removal,
//     easing, and copy and paste across speeds.
//   - Entry keys live in the entry; the 512 keys per clip count them.
//   - The blur radius has no stopwatch.
func TestVideoStudioEffectKeyframeOps(t *testing.T) {
	runVideoStudioKeyframesScript(t, `
const fresh = () => {
  const p = project([track('v1','video',[clip('c',V,0,100,{offset:100})]), track('v2','video',[clip('d',V,200,100,{speed:2})])]);
  for (const id of ['blur', 'color']) { K.addEffect(p, 'c', id); K.addEffect(p, 'd', id); }
  return p;
};
let p = fresh();
valid(p);
const c = () => at(p, 'c'), fx = () => c().effects[1];
assert(FX.animatablePaths(c()).includes('effects.1.exposure') && FX.animatablePaths(c()).includes('crop.l'));
assert.equal(FX.animatablePaths(c()).includes('effects.0.radius'), false, 'the blur radius is not keyframable');

// Stopwatch on an effect param: the key lives in the entry and holds the value shown.
assert.equal(K.setAnimated(p, 'c', 'effects.1.exposure', true, 10), true);
assert.deepEqual(fx().keyframes, {exposure: [kf(110, 0)]});
assert.equal('keyframes' in c(), false, 'nothing lands in the clip map');
assert.deepEqual(K.atPlayhead(c(), 'effects.1.exposure', 10), kf(110, 0));
assert.deepEqual(K.keysOf(c(), 'effects.1.exposure'), [kf(110, 0)]);
assert.deepEqual(K.keyedPaths(c()), ['effects.1.exposure']);
assert.equal(K.setAnimated(p, 'c', 'effects.0.radius', true, 10), 'nothing', 'no stopwatch for the blur radius');
assert.equal(K.setAnimated(p, 'c', 'effects.5.exposure', true, 10), 'nothing', 'no such entry');
// Auto-key through setEffectParams and setValues; static params keep their fields.
assert.equal(K.setEffectParams(p, 'c', 1, {exposure: 1.5, contrast: 1.2}, 40), true);
assert.deepEqual(fx().keyframes.exposure, [kf(110, 0), kf(140, 1.5)]);
assert.equal(fx().params.exposure, 0, 'the field of the animated exposure is untouched');
assert.equal(fx().params.contrast, 1.2);
assert.equal(K.setValues(p, 'c', {'effects.1.exposure': 9}, 50), true);
assert.deepEqual(fx().keyframes.exposure[2], kf(150, 3), 'values are clamped to the registry range');
assert.equal(K.setEffectParams(p, 'c', 1, {exposure: 1}, 150), 'nothing', 'no key outside the clip');
near(FX.valueAt(c(), 'effects.1.exposure', 25, p), 0.75, 'the entry keys drive the value');
// Crop sides keep their keys in the clip map.
assert.equal(K.setAnimated(p, 'c', 'crop.l', true, 0), true);
assert.deepEqual(c().keyframes, {'crop.l': [kf(100, 0)]});
assert.equal(K.setCrop(p, 'c', {l: 0.4}, 30), true);
assert.deepEqual(c().keyframes['crop.l'], [kf(100, 0), kf(130, 0.4)]);
assert.equal('crop' in c(), false, 'the animated side leaves the field alone (four zeros: no crop object)');
near(FX.clipState(c(), 15, p).crop.l, 0.2, 'clipState evaluates the crop keys');
valid(p);

// The diamond.
assert.equal(K.toggleKeyframe(p, 'c', 'effects.1.exposure', 40), true, 'removes the key at the playhead');
assert.deepEqual(fx().keyframes.exposure.map(key => key.f), [110, 150]);
assert.equal(K.toggleKeyframe(p, 'c', 'effects.1.contrast', 20), true, 'a static param starts its animation');
assert.deepEqual(fx().keyframes.contrast, [kf(120, 1.2)]);
assert.equal(K.toggleKeyframe(p, 'c', 'effects.1.contrast', 20), true, 'removing the last key ends it');
assert.equal('contrast' in fx().keyframes, false);
assert.equal(fx().params.contrast, 1.2, 'and leaves its value in the field');
// ◀▶ over entry and crop keys.
assert.equal(K.neighbor(c(), ['effects.1.exposure', 'crop.l'], 20, 1), 30);
assert.equal(K.neighbor(c(), ['effects.1.exposure', 'crop.l'], 30, 1), 50);
assert.equal(K.neighbor(c(), ['effects.1.exposure'], 50, -1), 10);
// Moves, collisions and easing.
assert.equal(K.moveKeys(p, 'c', [{path: 'effects.1.exposure', f: 110}, {path: 'crop.l', f: 130}], 5), true);
assert.deepEqual(fx().keyframes.exposure.map(key => key.f), [115, 150]);
assert.deepEqual(c().keyframes['crop.l'].map(key => key.f), [100, 135]);
assert.equal(K.moveKeys(p, 'c', [{path: 'effects.1.exposure', f: 115}], 35), 'bounds', 'onto the key at 150');
assert.equal(K.setEasing(p, 'c', [{path: 'effects.1.exposure', f: 115}, {path: 'crop.l', f: 100}], 'ease_out'), true);
assert.deepEqual([fx().keyframes.exposure[0].e, c().keyframes['crop.l'][0].e], ['ease_out', 'ease_out']);
valid(p);
// Copy and paste onto a clip at speed 2 whose entry 1 is a color entry too.
const board = K.copyKeys(c(), [{path: 'effects.1.exposure', f: 115}, {path: 'effects.1.exposure', f: 150}, {path: 'crop.l', f: 135}], 15);
assert.deepEqual(board, {keys: [{path: 'effects.1.exposure', dt: 0, v: 0, e: 'ease_out'}, {path: 'crop.l', dt: 20, v: 0.4, e: 'linear'}, {path: 'effects.1.exposure', dt: 35, v: 3, e: 'linear'}]});
assert.equal(K.pasteKeys(p, 'd', board, 220), true);
assert.deepEqual(at(p,'d').effects[1].keyframes, {exposure: [kf(40, 0, 'ease_out'), kf(110, 3)]});
assert.deepEqual(at(p,'d').keyframes, {'crop.l': [kf(80, 0.4)]});
valid(p);
// Removing a param's keys writes the value at the clip start into its field; no empty keyframes map is left.
assert.equal(K.removeKeys(p, 'c', [{path: 'effects.1.exposure', f: 115}, {path: 'effects.1.exposure', f: 150}]), true);
assert.equal('keyframes' in fx(), false);
near(fx().params.exposure, 0, 'the value at the clip start');
valid(p);

// 512 keys per clip, effect keys included.
p = fresh();
const full = () => Array.from({length: 64}, (_, i) => kf(100 + i, 0));
c().keyframes = {x: full(), y: full(), rotation: full(), opacity: full()};
fx().keyframes = {exposure: full(), contrast: full(), saturation: full(), tint: full()};
valid(p);
assert.equal(K.toggleKeyframe(p, 'c', 'effects.1.gamma', 10), 'limit', 'base and effect keys share the 512');
assert.equal(K.setAnimated(p, 'c', 'crop.t', true, 10), 'limit');
assert.equal(K.setValues(p, 'c', {'effects.1.exposure': 1}, 10), true, 'an existing key still takes a value');
assert.equal(K.pasteKeys(p, 'c', {keys: [{path: 'effects.1.lift', dt: 0, v: 0.1, e: 'linear'}]}, 70), 'limit');
`)
}

// Edit semantics of the 2c picture fields (spec §5.1, §5.3, §5.4; index §4.18).
//   - Split and duplicate keep entries and keys; reverse mirrors entry keys (with the limit).
//   - Freeze bakes crop, flips and the drawn effects at the frame, without keys.
//   - Detached audio drops the picture fields, and a rate change rescales entry keys.
//   - Paste attributes copies the groups basePicture and effects: entry keys are re-anchored, an animated crop side is
//     keyed at the playhead, the limits hold, and LUTs the project lost become pass-through.
func TestVideoStudioEffectEditSemantics(t *testing.T) {
	editing, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-editing.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`value="basePicture"`, `value="effects"`, "label('attrBasePicture'", "label('attrEffects'"} {
		if !strings.Contains(string(editing), want) {
			t.Errorf("the paste attributes dialog lacks %s", want)
		}
	}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		values := readDesktopLocale(t, lang)
		for _, key := range []string{"attrBasePicture", "attrEffects"} {
			if strings.TrimSpace(values["videoStudio."+key]) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			}
			if lang != "en" && values["videoStudio."+key] == english["videoStudio."+key] {
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
	}
	runVideoStudioKeyframesScript(t, `
assert.deepEqual([E.ATTRIBUTE_GROUPS.basePicture, E.ATTRIBUTE_GROUPS.effects], [['crop', 'flip_h', 'flip_v'], ['effects']]);
const stack = () => [
  {id: 'color', on: true, params: Object.assign(FX.defaults('color'), {contrast: 1.2}), keyframes: {exposure: [kf(110, 0, 'ease_in'), kf(150, 1)]}},
  {id: 'vignette', on: false, params: FX.defaults('vignette')}
];
const picture = () => ({crop: {l: 0.1, t: 0, r: 0.2, b: 0.05}, flip_h: true, effects: stack()});
const original = () => project([track('v1','video',[clip('c',V,0,100,Object.assign({offset:100}, picture()))]), track('a1','audio',[])]);
const exposure = (p, id, frame) => FX.valueAt(at(p, id), 'effects.0.exposure', frame, p);
let p = original();
valid(p);

// Split and duplicate: both parts and the copy keep every entry and key; the values stay continuous across the cut.
assert.equal(E.blade(p, 40, {clipIds: ['c'], linked: false}, {}), true);
const right = p.tracks[0].clips.find(item => item.id !== 'c');
for (const part of [at(p,'c'), right]) {
  assert.deepEqual(part.effects, stack());
  assert.deepEqual([part.crop, part.flip_h], [picture().crop, true]);
}
for (const frame of [39, 40, 41, 70]) near(FX.valueAt(frame < 40 ? at(p,'c') : right, 'effects.0.exposure', frame, p), exposure(original(), 'c', frame), 'split @' + frame);
const out = {};
assert.equal(E.duplicateClips(p, [right.id], out), true);
assert.deepEqual(at(p, out.created[0]).effects, stack());
valid(p);

// Reverse: entry keys mirror around the reversed asset's last frame (pivot 199) with ease_in <-> ease_out.
p = original();
p.assets.push({id:'r', kind:'video', has_audio:true, duration_frames:100});
assert.equal(E.reverseClips(p, 'c', {assetId:'r', rangeStart:100, frames:100, sourceAssetId:'v'}), true);
assert.deepEqual(at(p,'c').effects[0].keyframes, {exposure: [kf(49, 1, 'ease_out'), kf(89, 0)]});
assert.equal('keyframes' in at(p,'c').effects[1], false, 'an entry without keys stays without');
assert.deepEqual([at(p,'c').effects[0].params.contrast, at(p,'c').effects[1].on, at(p,'c').crop, at(p,'c').flip_h], [1.2, false, picture().crop, true], 'params, the on state, crop and flip are not time');
for (let frame = 0; frame < 100; frame++) near(exposure(p, 'c', 99 - frame), exposure(original(), 'c', frame), 'mirror @' + frame);
valid(p);
// Extra hold keys counted with the entry keys: past 64 per value the reverse refuses and leaves the project as it was.
p = project([track('v1','video',[clip('c',V,0,100,{offset:100, effects: [{id: 'color', on: true, params: FX.defaults('color'), keyframes: {exposure: Array.from({length: 40}, (_, i) => kf(100 + 2 * i, i % 2, 'hold'))}}]})])]);
valid(p);
p.assets.push({id:'r', kind:'video', has_audio:true, duration_frames:100});
const untouched = JSON.stringify(p);
assert.equal(E.reverseClips(p, 'c', {assetId:'r', rangeStart:100, frames:100, sourceAssetId:'v'}), 'limit');
assert.equal(JSON.stringify(p), untouched);

// Freeze frame: the still takes the crop (a valid field), flips and the drawn effects at the frozen frame, no keys.
p = project([track('v1','video',[clip('c',V,0,100,{offset:100, crop: {l: 0.3, t: 0, r: 0, b: 0}, flip_v: true, keyframes: {'crop.r': [kf(100, 0.5), kf(200, 0.95)]},
  effects: [{id: 'color', on: true, params: FX.defaults('color'), keyframes: {exposure: [kf(100, 0), kf(200, 2)]}}, {id: 'blur', on: false, params: FX.defaults('blur')}]})])]);
valid(p);
assert.equal(E.freezeFrame(p, 'c', 50, {asset_id: 'i', clip_id: 'fz', duration: 30}), true);
const still = at(p, 'fz');
assert.equal('keyframes' in still, false);
near(still.crop.l, 0.3, 'the static left side');
near(still.crop.r, 0.65, 'the right side shown at source 150 (0.725) is pulled in so l + r <= 0.95');
assert(still.crop.l + still.crop.r <= 0.95);
assert.deepEqual([still.flip_v, 'flip_h' in still], [true, false]);
assert.deepEqual(still.effects.map(entry => [entry.id, entry.on, 'keyframes' in entry]), [['color', true, false]], 'only the entries the clip shows, without keys');
near(still.effects[0].params.exposure, 1, 'the exposure shown at the frozen frame');
assert.deepEqual(Object.keys(still.effects[0].params).sort(), Object.keys(FX.defaults('color')).sort(), 'complete params');
valid(p);

// Detach audio: the audio copy has no picture fields; the video clip keeps them.
p = original();
assert.equal(E.detachAudio(p, 'c', {clip: 'au', link: 'l1', track: 'a2', trackName: 'Audio 2'}), true);
for (const key of ['crop', 'flip_h', 'flip_v', 'effects', 'keyframes']) assert.equal(key in at(p,'au'), false, 'the audio copy has no ' + key);
assert.deepEqual(at(p,'c').effects, stack());
valid(p);

// Rate change: entry key frames scale like the offset (rounded; a key landing on the one before it is dropped).
p = project([track('v1','video',[clip('c',V,0,100,{offset:10, effects: [{id: 'color', on: true, params: FX.defaults('color'), keyframes: {exposure: [kf(10, 0.1), kf(11, 0.2), kf(12, 0.3), kf(40, 0.4)]}}]})])]);
p.assets = p.assets.map(asset => Object.assign({}, asset));
p.assets.push({id: 'lut1', name: 'Film.cube', kind: 'lut', duration_frames: 0, lut_size: 17});
p.fps = 60;
window.VideoStudioTime.rescaleProject(p, 24, false);
assert.deepEqual(at(p,'c').effects[0].keyframes, {exposure: [kf(4, 0.1), kf(5, 0.3), kf(16, 0.4)]});
assert.equal(p.assets.find(asset => asset.id === 'lut1').duration_frames, 0, 'a LUT asset has no duration at any rate');
assert.deepEqual(at(p,'c').effects[0].params, FX.defaults('color'));
valid(p);

// Paste attributes: basePicture and effects replace the target's, entry keys re-anchored through both time maps
// (d: offset 300, speed 2; t: a still, source time = clip time); audio clips are skipped.
const source = () => at(project([track('v1','video',[clip('s',V,0,100,Object.assign({offset:100}, picture()))])]), 's');
p = project([
  track('v1','video',[clip('d',V,200,100,{offset:300, speed:2, effects: [{id: 'sharpen', on: true, params: FX.defaults('sharpen')}]})]),
  track('o1','overlay',[clip('t',I,0,60,{crop: {l: 0.2, t: 0, r: 0, b: 0}})]),
  track('a1','audio',[clip('m',A,0,100)])
]);
valid(p);
assert.equal(E.pasteAttributes(p, source(), ['d', 't', 'm'], ['basePicture', 'effects'], 250), true);
assert.deepEqual([at(p,'d').crop, at(p,'d').flip_h], [picture().crop, true]);
assert.deepEqual(at(p,'d').effects.map(entry => entry.id), ['color', 'vignette'], 'the source stack replaces the target stack');
assert.deepEqual(at(p,'d').effects[0].keyframes, {exposure: [kf(320, 0, 'ease_in'), kf(400, 1)]});
assert.deepEqual(at(p,'t').effects[0].keyframes, {exposure: [kf(10, 0, 'ease_in'), kf(50, 1)]});
assert.equal(at(p,'t').crop.l, 0.1, 'the source crop replaces the target crop');
for (const key of ['crop', 'effects', 'flip_h']) assert.equal(key in at(p,'m'), false, 'audio clips take no picture attributes');
for (let k = 0; k <= 50; k += 5) near(exposure(p, 'd', 200 + k), FX.valueAt(source(), 'effects.0.exposure', k, p), 're-anchored @' + k);
valid(p);
const plain = at(project([track('v1','video',[clip('s',V,0,100)])]), 's');
assert.equal(E.pasteAttributes(p, plain, ['d'], ['basePicture', 'effects'], 250), true);
for (const key of ['crop', 'flip_h', 'flip_v', 'effects']) assert.equal(key in at(p,'d'), false, 'pasting a plain clip clears ' + key);
valid(p);
// A crop side the target animates takes the source's value as a key at the playhead; static sides take the fields.
p = project([track('v1','video',[clip('d',V,200,100,{offset:300, keyframes: {'crop.l': [kf(300, 0), kf(400, 0.4)]}})])]);
valid(p);
assert.equal(E.pasteAttributes(p, source(), ['d'], ['basePicture'], 250), true);
assert.deepEqual(at(p,'d').keyframes['crop.l'], [kf(300, 0), kf(350, 0.1), kf(400, 0.4)]);
assert.deepEqual(at(p,'d').crop, {l: 0, t: 0, r: 0.2, b: 0.05});
valid(p);
// Pasted entry keys count toward the 512 per clip.
const many = v => Array.from({length: 64}, (_, i) => kf(i, v));
p = project([track('v1','video',[clip('d',V,0,100,{keyframes: {x: many(0), y: many(0), width: many(0.5), height: many(0.5), rotation: many(0), opacity: many(1), 'crop.l': many(0), 'crop.t': many(0)}})])]);
valid(p);
assert.equal(E.pasteAttributes(p, source(), ['d'], ['effects'], 10), 'limit');
// A LUT the project no longer holds becomes pass-through (the clipboard never crosses projects: P.clipboardReady).
const lutSource = Object.assign(source(), {effects: [{id: 'lut', on: true, params: {intensity: 1, lut: 'gone'}}]});
p = project([track('v1','video',[clip('d',V,0,100)])]);
assert.equal(E.pasteAttributes(p, lutSource, ['d'], ['effects'], 10), true);
assert.deepEqual(at(p,'d').effects[0].params, {intensity: 1, lut: ''});
valid(p);
const board = {span: 50, entries: [{trackId: 'v1', kind: 'video', kindIndex: 0, clip: clip('s', V, 0, 50, {effects: [{id: 'lut', on: true, params: {intensity: 1, lut: 'gone'}}]})}]};
p = project([track('v1','video',[])]);
assert.equal(E.paste(p, board, 0, 'overwrite', {}), true);
assert.equal(p.tracks[0].clips[0].effects[0].params.lut, '');
valid(p);
`)
}
