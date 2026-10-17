package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/videostudio"
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
cropNear({l: 0.25, t: 0.5, r: 0.35, b: 0.45}, 'the typed right side is clamped to the room the animated left side leaves at the frame (0.6)');
assert.deepEqual(c().keyframes['crop.l'], [kf(110, 0.25), kf(120, 0.6)], 'and the keys of the animated side stay');
valid(p);
// Settling never writes the field of an animated side: crop.l keyed at 0.30, the user types r = 0.80.
p = fresh();
assert.equal(K.setCrop(p, 'c', {l: 0.3}, 10), true);
assert.equal(K.setAnimated(p, 'c', 'crop.l', true, 10), true);
assert.deepEqual(c().keyframes, {'crop.l': [kf(110, 0.3)]});
assert.equal(K.setCrop(p, 'c', {r: 0.8}, 10), true);
cropNear({l: 0.3, t: 0, r: 0.65, b: 0}, 'r follows the keyed 0.30, and the field of l is not written');
const shown = FX.clipState(c(), 10, p).crop;
assert(shown.l + shown.r <= 0.95 + 1e-9 && Math.abs(shown.r - 0.65) < 1e-6, 'the picture shows what the fields say: ' + JSON.stringify(shown));
// The key moves; the field of l keeps its old 0.30 and still counts, because the server validates the fields.
assert.equal(K.setCrop(p, 'c', {l: 0.1}, 10), true);
assert.deepEqual(c().keyframes['crop.l'], [kf(110, 0.1)]);
assert.equal(K.setCrop(p, 'c', {r: 0.9}, 10), true);
cropNear({l: 0.3, t: 0, r: 0.65, b: 0}, 'the stale field of l still bounds r');
valid(p);
// Both sides animated: the typed key is clamped to l's value at the frame, and no field is written.
assert.equal(K.setAnimated(p, 'c', 'crop.r', true, 10), true);
assert.equal(K.setCrop(p, 'c', {r: 0.9}, 10), true);
assert.deepEqual(c().keyframes['crop.r'], [kf(110, 0.85)], 'the typed key is clamped to 0.95 - 0.10');
cropNear({l: 0.3, t: 0, r: 0.65, b: 0}, 'both fields stay as they were');
// Ending an animation clamps the side that becomes a field against the animated partner at the frame (keys that disagree
// at the frame, as a project from before this rule may hold).
c().keyframes = {'crop.l': [kf(110, 0.6)], 'crop.r': [kf(110, 0.5)]};
Object.assign(c().crop, {l: 0.1, r: 0.1});
assert.equal(K.setAnimated(p, 'c', 'crop.l', false, 10), true);
cropNear({l: 0.45, t: 0, r: 0.1, b: 0}, 'l off: the shown 0.60 is clamped to the room the animated r (0.50) leaves; r is not written');
assert.deepEqual(c().keyframes, {'crop.r': [kf(110, 0.5)]});
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

// videoStudioEffectStackPartsPrelude loads the keyframe app parts, the shared picture math and the Effects section on
// top of the keyframe prelude; none of them touches the DOM at load.
const videoStudioEffectStackPartsPrelude = videoStudioKeyframePartsPrelude + `
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-inspector-effects.js');
const FXI = window.VideoStudioInspectorEffects;
`

// The Effects section (spec §8.1).
//   - The "+ Effect" menu groups the registry by category. Unavailable effects and a full stack disable their items
//     with a reason.
//   - Entries carry the header toggle, the on/off switch, reset, remove and the grip; sliders show the values at the
//     playhead, with keyframe tools on keyframable params (none on the blur radius).
//   - The LUT picker offers none, the looks and the project LUTs, plus "Upload .cube".
//   - Badges mark unavailable effects and failed preview programs; effectsCropAnimatedHint shows with an animated crop.
//   - The Picture section's crop fields and flips sit in the same view.
func TestVideoStudioEffectStackMarkup(t *testing.T) {
	runVideoStudioKeyframesScript(t, videoStudioEffectStackPartsPrelude+`
let groups = FXI.menuModel({});
assert.deepEqual(groups.map(group => [group.label_key, group.items.map(item => item.id)]), [['effectCategory_color', ['color', 'lut']], ['effectCategory_detail', ['blur', 'sharpen']], ['effectCategory_stylize', ['vignette']]]);
assert(groups.every(group => group.items.every(item => item.available && !item.disabled && item.reason === '')));
FX.adopt(FX.BUILTIN, {effects: ['lut'], transitions: [], blend_modes: [], anim_presets: []});
assert.deepEqual(FXI.menuModel({}).flatMap(group => group.items).find(item => item.id === 'lut'), {id: 'lut', label_key: 'effect.lut.name', available: false, disabled: true, reason: 'effectUnavailable'});
const full = {effects: Array.from({length: 8}, () => ({id: 'blur', on: true, params: FX.defaults('blur')}))};
assert(FXI.menuModel(full).flatMap(group => group.items).every(item => item.disabled), 'a full stack disables every item');
assert.equal(FXI.menuModel(full)[0].items[0].reason, 'effectsFull');
assert.deepEqual([FXI.dropIndex(0, 2, true), FXI.dropIndex(2, 0, false), FXI.dropIndex(0, 1, false), FXI.dropIndex(1, 1, true)], [2, 0, 0, 1]);
assert.deepEqual([FXI.formatValue({unit: 'ev', step: 0.01}, 0.5, 'en'), FXI.formatValue({unit: 'ev', step: 0.01}, -1, 'en'), FXI.formatValue({unit: 'K', step: 10}, 6500, 'en'),
  FXI.formatValue({unit: 'height', step: 0.0005}, 0.01, 'en'), FXI.formatValue({step: 0.01}, 1.2, 'en')], ['+0.50 EV', '-1.00 EV', '6,500 K', '1.00 %', '1.20']);
const twin = {effects: [{id: 'color'}, {id: 'blur'}, {id: 'color'}]};
assert.deepEqual([0, 1, 2].map(index => FXI.entryName(twin, index, key => '[' + key + ']')), ['[effect.color.name] 1', '[effect.blur.name]', '[effect.color.name] 2']);

// A clip with an animated crop and three entries, at frame 20 (source 120).
const p = project([
  track('v1','video',[clip('c',V,0,100,{offset:100, keyframes: {'crop.l': [kf(100, 0), kf(150, 0.2)]}})]),
  track('v2','video',[clip('e',V,0,50)]),
  track('a1','audio',[clip('m',A,0,100)])
]);
K.addEffect(p, 'c', 'color'); K.addEffect(p, 'c', 'lut', {lut: 'builtin:warm'}); K.addEffect(p, 'c', 'blur');
at(p,'c').effects[0].keyframes = {exposure: [kf(100, 0), kf(160, 1.5)]};
p.assets.push({id: 'lut1', name: 'Film.cube', kind: 'lut', duration_frames: 0, lut_size: 17});
valid(p);
const s = {project: p, selectedClipId: 'c', frame: 20, readonly: false};
const view = P.keyframeView(s);
assert(view.controls['effects.0.exposure'] && view.controls['crop.l'], 'keyframe controls cover crop sides and effect params');
assert.equal('effects.2.radius' in view.controls, false, 'the blur radius has no keyframe controls');
const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => '&#' + ch.charCodeAt(0) + ';');
const render = (clipId, asset, fx) => {
  const host = {innerHTML: ''};
  IN.render(host, {selected: T.clipFor(p, clipId), asset, project: p, locale: 'en', caps: {}, transitionInfo: {next: null}, esc,
    icon: name => '<i data-icon="' + name + '"></i>', tr: key => '[' + key + ']', keyframes: P.keyframeView(Object.assign({}, s, {selectedClipId: clipId})), fineOpen: true, effects: fx});
  return host.innerHTML;
};
const html = render('c', V, {failed: new Set(['blur']), luts: [{id: 'lut1', name: 'Film.cube'}], collapsed: entry => entry.id === 'blur'});
const entry = index => { const start = html.indexOf('data-fx-entry="' + index + '"'), next = html.indexOf('data-fx-entry="' + (index + 1) + '"'); return html.slice(start, next < 0 ? html.indexOf('data-fx-action="add"') : next); };
assert(html.indexOf('data-fx-section') > html.indexOf('[picture]') && html.indexOf('data-fx-section') < html.indexOf('[animation]'), 'Effects sit between Picture and Animation');
assert(html.includes('<span class="vs-fx-count">3/8</span>'));
assert.deepEqual(Array.from(html.matchAll(/data-fx-entry="(\d)" data-fx-id="([a-z_]+)"/g), m => m[1] + m[2]), ['0color', '1lut', '2blur']);
assert(/<button[^>]*role="switch"[^>]*aria-checked="true"/.test(entry(0)), 'each entry has an on/off switch');
assert(/<button[^>]*data-fx-head aria-expanded="true"/.test(entry(0)) && entry(0).includes('[effect.color.name]'));
assert(/<button[^>]*data-fx-head aria-expanded="false"/.test(entry(2)) && /class="vs-fx-body"[^>]*hidden/.test(entry(2)), 'a collapsed entry hides its params');
assert(entry(0).includes('data-fx-action="reset"') && entry(0).includes('data-fx-action="remove"') && entry(0).includes('data-fx-grip'));
assert(/<input type="range" data-fx-param="exposure"[^>]*value="0.5"/.test(entry(0)), 'sliders show the value at the playhead');
assert(entry(0).includes('<output>+0.50 EV</output>'));
assert(entry(0).includes('data-kf-path="effects.0.exposure"') && entry(0).includes('data-kf-path="effects.0.lift_x"'), 'keyframable params carry keyframe tools');
assert(/<input type="range" data-fx-param="radius"/.test(entry(2)) && !entry(2).includes('data-kf-path='), 'the blur radius is a slider without keyframe tools');
assert(entry(1).includes('<option value="builtin:warm" selected>[look_warm]</option>'), 'the LUT picker shows the chosen look');
assert(entry(1).includes('<option value="lut1">Film.cube</option>') && entry(1).includes('<option value="">[lutNone]</option>'));
assert(entry(1).includes('data-fx-action="lut-upload"') && html.includes('data-fx-lut-file'));
assert(entry(1).includes('data-fx-badge="unavailable"') && !entry(0).includes('data-fx-badge'), 'an effect the server lacks is badged');
assert(entry(2).includes('data-fx-badge="failed"'), 'an effect whose preview program failed is badged');
assert(html.includes('data-fx-hint') && html.includes('[effectsCropAnimatedHint]'), 'an animated crop explains the processing order');
assert(/<input type="number" data-crop="l"[^>]*value="8"/.test(html), 'crop fields show the value at the playhead in percent');
assert(html.includes('data-kf-path="crop.l"') && /<button[^>]*data-flip="h" aria-pressed="false"/.test(html), 'crop keyframe tools and the flip toggles');
assert(!/<button[^>]*data-fx-action="add"[^>]*disabled/.test(html), 'a stack below 8 can grow');
// No effects: a hint and an enabled button; a full stack disables it; audio clips have no Effects section.
let other = render('e', V, {failed: new Set(), luts: [], collapsed: () => false});
assert(other.includes('data-fx-empty') && !other.includes('data-fx-hint') && other.includes('<span class="vs-fx-count">0/8</span>'));
at(p, 'e').effects = full.effects.map(item => JSON.parse(JSON.stringify(item)));
other = render('e', V, {failed: new Set(), luts: [], collapsed: () => false});
assert(/<button[^>]*data-fx-action="add"[^>]*disabled title="\[effectsFull\]"/.test(other), 'a full stack disables Add effect');
assert(!render('m', A, null).includes('data-fx-section'), 'audio clips have no Effects section');
FX.adopt(null, null);
`)
}

// Alt+ArrowUp / Alt+ArrowDown on a focused entry header move the entry, stop the event and keep the playhead.
// Elsewhere the keys still jump between keyframes (P.handleKeyframeKeys, matched by event.code), and handleKeys tries the
// stack first.
func TestVideoStudioEffectStackKeys(t *testing.T) {
	app, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	stack, keys := strings.Index(text, "P.handleEffectStackKeys(s, event)"), strings.Index(text, "P.handleKeyframeKeys(s, event)")
	if stack < 0 || keys < stack {
		t.Fatal("handleKeys must try P.handleEffectStackKeys before P.handleKeyframeKeys")
	}
	for _, want := range []string{"P.effectCommand(s, event)", "P.wireEffectStack(s)", "effects: P.effectView ? P.effectView(s) : null"} {
		if !strings.Contains(text, want) {
			t.Errorf("video-studio.js lacks %s", want)
		}
	}
	runVideoStudioKeyframesScript(t, videoStudioEffectStackPartsPrelude+`
P.applyEdit = (state, label, op) => op(state.project, {}) === true;
P.seekTo = (state, frame) => { state.frame = frame; };
const p = project([track('v1','video',[clip('c',V,0,100,{offset:100, keyframes: {opacity: [kf(100, 1), kf(130, 0.5)]}})])]);
for (const id of ['color', 'lut', 'vignette']) assert.equal(K.addEffect(p, 'c', id), true);
valid(p);
const s = {project: p, selectedClipId: 'c', frame: 20, stageMode: 'timeline', readonly: false, q: () => null};
const ids = () => at(p, 'c').effects.map(entry => entry.id);
const header = index => { const entry = {dataset: {fxEntry: String(index)}}, el = {closest: selector => (selector === '[data-fx-head]' ? el : selector === '[data-fx-entry]' ? entry : null)}; return el; };
const elsewhere = {closest: () => null};
const ev = (code, target, mods) => Object.assign({code, key: code, target, altKey: true, ctrlKey: false, metaKey: false, shiftKey: false,
  prevented: false, stopped: false, preventDefault() { this.prevented = true; }, stopPropagation() { this.stopped = true; }}, mods || {});
let event = ev('ArrowUp', header(1));
assert.equal(P.handleEffectStackKeys(s, event), true);
assert(event.prevented && event.stopped, 'the stack stops the event');
assert.deepEqual(ids(), ['lut', 'color', 'vignette']);
assert.equal(s.frame, 20, 'the playhead stays');
assert.equal(P.handleEffectStackKeys(s, ev('ArrowDown', header(0))), true);
assert.deepEqual(ids(), ['color', 'lut', 'vignette']);
event = ev('ArrowUp', header(0));
assert.equal(P.handleEffectStackKeys(s, event), true, 'the top entry keeps the key without moving');
assert(event.stopped);
assert.deepEqual(ids(), ['color', 'lut', 'vignette']);
assert.equal(P.handleEffectStackKeys(s, ev('ArrowDown', header(2))), true);
assert.deepEqual(ids(), ['color', 'lut', 'vignette'], 'nor the bottom one');
for (const mods of [{altKey: false}, {shiftKey: true}, {ctrlKey: true}, {metaKey: true}]) assert.equal(P.handleEffectStackKeys(s, ev('ArrowUp', header(1), mods)), false, JSON.stringify(mods));
assert.equal(P.handleEffectStackKeys(s, ev('ArrowLeft', header(1))), false);
assert.equal(P.handleEffectStackKeys(Object.assign({}, s, {stageMode: 'source'}), ev('ArrowUp', header(1))), false, 'the source monitor keeps its keys');
event = ev('ArrowUp', elsewhere);
assert.equal(P.handleEffectStackKeys(s, event), false);
assert.equal(event.stopped, false);
assert.equal(P.handleKeyframeKeys(s, event), true);
assert.equal(s.frame, 0, 'Alt+ArrowUp elsewhere jumps to the previous keyframe');
assert.equal(P.handleKeyframeKeys(s, ev('ArrowDown', elsewhere)), true);
assert.equal(s.frame, 30);
p.tracks[0].locked = true;
assert.equal(P.handleEffectStackKeys(s, ev('ArrowUp', header(1))), true);
assert.deepEqual(ids(), ['color', 'lut', 'vignette'], 'locked tracks never change');
`)
}

// Selected effect keys follow the stack (fix 2c-review).
//   - The strip names an effect key by its entry's stack index, so every edit that moves, removes or replaces entries,
//     and every undo and redo, clears the selected effects.<i>.* ids. Base keys stay selected.
//   - Edits that leave the entries in place (param changes, an entry added at the end) and refused edits keep the selection.
func TestVideoStudioEffectKeySelectionFollowsTheStack(t *testing.T) {
	runVideoStudioKeyframesScript(t, videoStudioEffectStackPartsPrelude+`
P.renderUI = () => {}; P.renderToolbar = () => {}; P.resyncArtwork = () => {};
const p = project([track('v1','video',[clip('c',V,0,100,{offset:100, keyframes: {opacity: [kf(100, 1), kf(130, 0.5)]}})]), track('v2','video',[clip('d',V,0,100,{offset:100})])]);
for (const id of ['color', 'lut', 'blur']) assert.equal(K.addEffect(p, 'c', id), true);
at(p,'c').effects[0].keyframes = {exposure: [kf(100, 0), kf(160, 1)]};
at(p,'c').effects[1].keyframes = {intensity: [kf(100, 1), kf(130, 0.5)]};
valid(p);
// conflict keeps scheduleSave from arming its autosave timer; the stubs above stand in for the rendered app.
const s = {project: p, projectId: '', readonly: false, conflict: true, history: [], redo: [], frame: 20, dirty: false, revision: 0, selectedClipId: 'c', selectionRevision: 0,
  selectionSet: new Set(['c']), selectionPrimary: 'c', ctx: {}, q: () => null, preview: {seek() {}}, kfStrip: {clipId: 'c', expanded: true, selected: []}};
const ALL = ['opacity:100', 'effects.0.exposure:100', 'effects.1.intensity:130', 'effects.0.exposure:160'];
const select = () => { s.kfStrip.selected = ALL.slice(); };
const effectIds = () => s.kfStrip.selected.filter(id => id.startsWith('effects.'));
const ids = () => at(s.project, 'c').effects.map(entry => entry.id).join();

// Edits that keep the entries where they are keep the selection: a param change, an entry added at the end, a refusal.
select();
assert.equal(P.mutate(s, 'param', project => K.setEffectParams(project, 'c', 0, {contrast: 1.4}, 20) === true ? undefined : false), true);
assert.deepEqual(s.kfStrip.selected, ALL, 'a param change keeps the selection');
assert.equal(P.mutate(s, 'add', project => K.addEffect(project, 'c', 'vignette') === true ? undefined : false), true);
assert.equal(ids(), 'color,lut,blur,vignette');
assert.deepEqual(s.kfStrip.selected, ALL, 'an entry added at the end keeps it');
assert.equal(P.mutate(s, 'refused', () => false), false);
assert.deepEqual(s.kfStrip.selected, ALL, 'a refused edit keeps it');
assert.equal(P.mutate(s, 'other clip', project => { at(project, 'd').opacity = 0.5; }), true);
assert.deepEqual(s.kfStrip.selected, ALL, 'an edit of another clip keeps it');

// A move shifts the indexes: the effect ids go, the base key stays.
assert.equal(P.mutate(s, 'move', project => K.moveEffect(project, 'c', 0, 2) === true ? undefined : false), true);
assert.equal(ids(), 'lut,blur,color,vignette');
assert.deepEqual(s.kfStrip.selected, ['opacity:100'], 'a move drops the effect keys');

// Undo and redo rebuild the project: the effect ids go whatever the step was.
select();
P.undo(s, false);
assert.equal(ids(), 'color,lut,blur,vignette', 'the move is undone');
assert.deepEqual(s.kfStrip.selected, ['opacity:100'], 'undo drops the effect keys');
select();
P.undo(s, true);
assert.equal(ids(), 'lut,blur,color,vignette', 'the move is redone');
assert.deepEqual(s.kfStrip.selected, ['opacity:100'], 'redo drops the effect keys');
// Back through the move and the edit of the other clip to before the added entry: that step left the stack alone.
P.undo(s, false);
P.undo(s, false);
select();
P.undo(s, false);
assert.equal(ids(), 'color,lut,blur', 'the added entry is undone');
assert.deepEqual(s.kfStrip.selected, ['opacity:100'], 'undoing an edit that left the stack alone drops them too');

// A removal and an edit that replaces the entries (paste attributes) drop them as well.
select();
assert.equal(P.mutate(s, 'remove', project => K.removeEffect(project, 'c', 0) === true ? undefined : false), true);
assert.deepEqual(effectIds(), [], 'a removal drops the effect keys');
assert.equal(ids(), 'lut,blur');
select();
assert.equal(P.mutate(s, 'paste', project => E.pasteAttributes(project, at(project, 'c'), ['c'], ['effects'], 20) === true ? undefined : false), true);
assert.equal(ids(), 'lut,blur', 'the stack is the same');
assert.deepEqual(s.kfStrip.selected, ['opacity:100'], 'paste attributes replaced the entries: the effect keys go');

// Without effect ids selected nothing is touched, and a strip on no clip is no error.
s.kfStrip.selected = ['opacity:130'];
assert.equal(P.mutate(s, 'move', project => K.moveEffect(project, 'c', 0, 1) === true ? undefined : false), true);
assert.deepEqual(s.kfStrip.selected, ['opacity:130']);
delete s.kfStrip;
P.undo(s, false);
assert.equal(s.kfStrip, undefined);
`)
}

// The keyframe strip covers crop sides and effect params.
//   - Rows follow VideoStudioEffects.animatablePaths (base, crop, then each entry's params); summary diamonds include entry
//     keys.
//   - Effect rows are named "<entry> · <param>", and their diamonds select like base ones.
func TestVideoStudioKeyframeStripEffectRows(t *testing.T) {
	runVideoStudioKeyframesScript(t, videoStudioEffectStackPartsPrelude+`
const clip0 = () => ({id: 'c', asset_id: 'none', start: 30, offset: 0, duration: 60, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain',
  keyframes: {opacity: [kf(0, 1)], 'crop.l': [kf(10, 0.1)]},
  effects: [{id: 'blur', on: true, params: FX.defaults('blur')}, {id: 'color', on: true, params: FX.defaults('color'), keyframes: {exposure: [kf(10, 0.5, 'ease_in'), kf(20, 1)]}}]});
const model = KF.stripModel({fps: 30}, clip0(), 52, {expanded: true});
assert.deepEqual(model.rows.map(row => row.id), ['summary', 'opacity', 'crop.l', 'effects.1.exposure']);
assert.deepEqual(model.rows[0].keys.map(key => [key.f, key.paths, key.e]), [[0, ['opacity'], 'linear'], [10, ['crop.l', 'effects.1.exposure'], 'mixed'], [20, ['effects.1.exposure'], 'linear']]);
near(model.rows[3].keys[1].frame, 50, 'entry keys sit at timelineAt like base keys');
const p = project([track('v1', 'video', [clip0()])]);
const s = {project: p, selectedClipId: 'c', frame: 40, readonly: false, q: () => null, kfStrip: {clipId: 'c', expanded: true, selected: ['effects.1.exposure:20']},
  ctx: {t: key => '[' + key.slice(12) + ']', esc: value => String(value)}};
const html = P.keyframeStrip(s, p.tracks[0], 52, 2000);
assert(html.includes('data-kf-row="effects.1.exposure"') && html.includes('[effect.color.name] · [effect.color.exposure]'), 'effect rows are named after their entry');
assert(html.includes('data-kf-row="crop.l"') && html.includes('<span class="vs-kf-rowname">[crop_l]</span>'));
assert(/data-kf-key="effects\.1\.exposure:20"[^>]*aria-pressed="true"/.test(html), 'the selection holds entry keys');
assert(/data-kf-key="summary:10"[^>]*aria-pressed="false"/.test(html));
`)
}

// Every desktop locale carries the Effects section strings (Task 14), and the sentences are translated:
// - the registry's label keys: crop and flip; effect names, params and categories; the eight looks;
// - the section's own strings, with their {{name}} placeholders.
func TestVideoStudioEffectStackLocales(t *testing.T) {
	registry := videostudio.EffectRegistry()
	keys := []string{"effects", "effectAdd", "effectsEmpty", "effectsFull", "effectUnavailable", "effectBadgeUnavailable", "effectPreviewFailed", "effectBadgeFailed", "effectOn", "effectReset", "effectRemove", "effectDrag", "effectHeadHint", "effectsCropAnimatedHint", "crop", "flip", "lutNone", "lutLooks", "lutProject", "lutUpload", "lutUploadReady", "lutInvalid", "lutTooLarge", "lutCountLimit", "lutUploadType"}
	for _, spec := range registry.Base {
		if strings.HasPrefix(spec.ID, "crop.") || strings.HasPrefix(spec.ID, "flip_") {
			keys = append(keys, spec.LabelKey)
		}
	}
	for _, effect := range registry.Effects {
		keys = append(keys, effect.LabelKey, "effectCategory_"+effect.Category)
		for _, param := range effect.Params {
			keys = append(keys, param.LabelKey)
		}
	}
	for _, look := range registry.Looks {
		keys = append(keys, look.LabelKey)
	}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		values := readDesktopLocale(t, lang)
		for _, key := range keys {
			if strings.TrimSpace(values["videoStudio."+key]) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			}
		}
		if !strings.Contains(values["videoStudio.lutInvalid"], "{{line}}") {
			t.Errorf("%s videoStudio.lutInvalid lost {{line}}: %q", lang, values["videoStudio.lutInvalid"])
		}
		for _, key := range []string{"effectPreviewFailed", "effectOn", "effectReset", "effectRemove", "effectHeadHint"} {
			if !strings.Contains(values["videoStudio."+key], "{{name}}") {
				t.Errorf("%s videoStudio.%s lost {{name}}: %q", lang, key, values["videoStudio."+key])
			}
		}
		if lang == "en" {
			continue
		}
		for _, key := range []string{"effectsEmpty", "effectsFull", "effectUnavailable", "effectPreviewFailed", "effectHeadHint", "effectsCropAnimatedHint", "lutUpload", "lutUploadType", "lutInvalid"} {
			if values["videoStudio."+key] == english["videoStudio."+key] {
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
	}
}

// Colour wheels (spec §8.3).
//   - Model: pointer ↔ x/y with the SVG y axis flipped and clamped to the disc; arrow steps 0.01 (Shift 0.1); Delete and
//     Backspace reset; the value text gives hue and strength; the hue ring and the point's angle follow the shared φ
//     mapping (VideoStudioPicture.wheel).
//   - Markup: role="slider" with aria-roledescription and aria-valuetext.
//   - The color entry's panel: three wheels, the master sliders and five sliders.
//   - Each wheel's one set of keyframe tools acts on its x and y together; handleKeys lets the wheel keep its keys.
func TestVideoStudioColorWheelModel(t *testing.T) {
	app, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(app), `[role="separator"],[role="slider"]`) {
		t.Fatal("handleKeys must leave the keys of a focused colour wheel (role=slider) to the wheel")
	}
	runVideoStudioKeyframesScript(t, videoStudioEffectStackPartsPrelude+`
require('./js/desktop/apps/video-studio-colorwheel.js');
global.CSS = global.CSS || {escape: value => String(value)};
const W = window.VideoStudioColorWheel, PIC = window.VideoStudioPicture;
const tr = (key, fallback) => fallback;
const rect = {left: 100, top: 50, width: 120, height: 120}; // centre (160, 110), disc radius 60 / 1.2 = 50 px
assert.deepEqual(W.fromPointer(185, 110, rect), {x: 0.5, y: 0}, 'right of the centre is +x');
assert.deepEqual(W.fromPointer(160, 85, rect), {x: 0, y: 0.5}, 'above the centre is +y (the SVG y axis points down)');
assert.deepEqual(W.fromPointer(160, 160, rect), {x: 0, y: -1});
const far = W.fromPointer(260, 10, rect);
assert(Math.abs(Math.hypot(far.x, far.y) - 1) < 2e-3 && Math.abs(far.x - far.y) < 1e-9, 'clamped to the disc: ' + JSON.stringify(far));
assert.deepEqual(W.clampDisc(3, 4), {x: 0.6, y: 0.8});
assert.deepEqual(W.clampDisc(-0.0001, 0), {x: 0, y: 0}, 'no negative zero');
assert.deepEqual(W.keyStep(0, 0, 'ArrowRight', false), {x: 0.01, y: 0});
assert.deepEqual(W.keyStep(0.02, 0, 'ArrowLeft', false), {x: 0.01, y: 0});
assert.deepEqual(W.keyStep(0, 0, 'ArrowUp', true), {x: 0, y: 0.1});
assert.deepEqual(W.keyStep(0, 0, 'ArrowDown', false), {x: 0, y: -0.01});
assert.deepEqual(W.keyStep(0, 0.95, 'ArrowUp', true), {x: 0, y: 1}, 'kept inside the disc');
assert.deepEqual(W.keyStep(0.3, 0.2, 'Delete', false), {x: 0, y: 0});
assert.deepEqual(W.keyStep(0.3, 0.2, 'Backspace', false), {x: 0, y: 0});
assert.equal(W.keyStep(0.3, 0.2, 'Enter', false), null);
let x = 0;
for (let i = 0; i < 3; i++) x = W.keyStep(x, 0, 'ArrowRight', false).x;
assert.equal(x, 0.03, 'steps stay on the 0.001 grid');
assert.equal(W.valueText(tr, 0, 0), 'Hue 0°, strength 0 %');
assert.equal(W.valueText(tr, 0.5, 0), 'Hue 0°, strength 50 %');
assert.equal(W.valueText(tr, 0, 0.25), 'Hue 90°, strength 25 %');
assert.equal(W.valueText(tr, -0.5, -0.866), 'Hue 240°, strength 100 %');
assert.equal(W.valueText(tr, 0.03, 0.1), 'Hue 73°, strength 10 %');
for (const [deg, channel] of [[0, 0], [120, 1], [240, 2]]) {
  const a = deg * Math.PI / 180, v = W.fromPointer(160 + 40 * Math.cos(a), 110 - 40 * Math.sin(a), rect), rgb = PIC.wheel(v.x, v.y);
  assert.equal(rgb.indexOf(Math.max(...rgb)), channel, deg + '° pushes channel ' + channel + ': ' + JSON.stringify(rgb));
  assert.equal(W.polar(v.x, v.y).hue, deg);
}
assert.equal(W.ringColor(0), '#ff4040', 'the ring is drawn from the φ mapping: red at 0°');
let html = W.markup({name: 'lift', label: 'Lift wheel', x: 0.5, y: 0, tr, esc: value => String(value)});
for (const want of ['role="slider"', 'aria-roledescription="color wheel"', 'aria-valuetext="Hue 0°, strength 50 %"', 'aria-valuenow="50"', 'tabindex="0"', 'aria-label="Lift wheel"', 'data-wheel="lift"', 'cx="0.5" cy="0"']) assert(html.includes(want), 'markup lacks ' + want);
assert.equal((html.match(/<path d="M[^"]*Z" fill="#[0-9a-f]{6}"/g) || []).length, 72, 'a hue ring of 72 segments');
html = W.markup({name: 'gain', label: 'Gain wheel', x: 0, y: 0, tr, esc: value => String(value), disabled: true});
assert(html.includes('tabindex="-1"') && html.includes('aria-disabled="true"'));
const dot = {attrs: {}, setAttribute(key, value) { this.attrs[key] = value; }};
const el = {dataset: {}, attrs: {}, setAttribute(key, value) { this.attrs[key] = value; }, querySelector: () => dot};
assert.deepEqual(W.update(el, 0.3, 0.4, tr), {x: 0.3, y: 0.4});
assert.deepEqual([dot.attrs.cx, dot.attrs.cy, el.attrs['aria-valuenow'], el.attrs['aria-valuetext'], el.dataset.wheelX, el.dataset.wheelY], ['0.3', '-0.4', '50', 'Hue 53°, strength 50 %', '0.3', '0.4']);

// The color entry's panel.
const p = project([track('v1','video',[clip('c',V,0,100,{offset:100})])]);
assert.equal(K.addEffect(p, 'c', 'color'), true);
at(p,'c').effects[0].params.gain_x = 0.5;
valid(p);
const s = {project: p, selectedClipId: 'c', frame: 10, readonly: false, stageMode: 'timeline', q: () => null};
const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => '&#' + ch.charCodeAt(0) + ';');
const host = {innerHTML: ''};
IN.render(host, {selected: T.clipFor(p, 'c'), asset: V, project: p, locale: 'en', caps: {}, transitionInfo: {next: null}, esc, icon: name => '<i data-icon="' + name + '"></i>',
  tr: (key, fallback) => fallback || key, keyframes: P.keyframeView(s), fineOpen: false, effects: {failed: new Set(), luts: [], collapsed: () => false}});
html = host.innerHTML;
assert.deepEqual(Array.from(html.matchAll(/data-wheel="([a-z]+)"/g), m => m[1]), ['lift', 'gamma', 'gain']);
assert(/data-wheel="gain"[^>]*aria-valuetext="Hue 0°, strength 50 %"/.test(html), 'the gain wheel shows its stored x/y');
for (const id of ['lift', 'gamma', 'gain', 'exposure', 'contrast', 'saturation', 'temperature', 'tint']) assert(html.includes('data-fx-param="' + id + '"'), id + ' slider');
assert(!html.includes('data-fx-param="lift_x"'), 'the wheels replace the x/y sliders');
assert(html.includes('class="vs-slider vs-fx-slider vs-fx-p-temperature"'), 'the temperature slider gets its blue-to-orange track');
assert(html.includes('data-kf-path="effects.0.gain_x effects.0.gain_y"'), 'one set of keyframe tools per wheel');

// The wheel's keyframe tools animate, key and stop x and y together.
P.applyEdit = (state, label, op) => op(state.project, {}) === true;
const wheelPair = 'effects.0.gain_x effects.0.gain_y';
const click = action => P.handleKeyframeClick(s, {target: {closest: selector => (selector === '[data-kf-path][data-kf-action]' ? {dataset: {kfPath: wheelPair, kfAction: action}, disabled: false, parentElement: null} : null)}});
assert.equal(click('stopwatch'), true);
assert.deepEqual(at(p,'c').effects[0].keyframes, {gain_x: [kf(110, 0.5)], gain_y: [kf(110, 0)]});
s.frame = 30;
assert.deepEqual(IN.controlFor(P.keyframeView(s), wheelPair), {animated: true, onKey: false, prev: 10, next: null, value: null});
assert.equal(click('key'), true);
assert.deepEqual([at(p,'c').effects[0].keyframes.gain_x.map(key => key.f), at(p,'c').effects[0].keyframes.gain_y.map(key => key.f)], [[110, 130], [110, 130]]);
assert.equal(IN.controlFor(P.keyframeView(s), wheelPair).onKey, true);
assert.equal(click('key'), true, 'on a key of both, the diamond removes both');
assert.deepEqual(at(p,'c').effects[0].keyframes.gain_x.map(key => key.f), [110]);
assert.equal(click('stopwatch'), true);
assert.equal('keyframes' in at(p,'c').effects[0], false, 'the stopwatch stops both');
assert.equal(at(p,'c').effects[0].params.gain_x, 0.5);
valid(p);
`)
}

// Every desktop locale carries the colour wheel strings, keeps the hue and strength placeholders, and translates them.
func TestVideoStudioColorWheelLocales(t *testing.T) {
	keys := []string{"colorWheels", "colorWheelLift", "colorWheelGamma", "colorWheelGain", "colorWheelRole", "colorWheelValue", "colorWheelHint"}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		values := readDesktopLocale(t, lang)
		for _, key := range keys {
			if strings.TrimSpace(values["videoStudio."+key]) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			}
		}
		if text := values["videoStudio.colorWheelValue"]; !strings.Contains(text, "{{hue}}") || !strings.Contains(text, "{{strength}}") {
			t.Errorf("%s videoStudio.colorWheelValue lost a placeholder: %q", lang, text)
		}
		if lang == "en" {
			continue
		}
		for _, key := range []string{"colorWheelValue", "colorWheelHint", "colorWheelLift", "colorWheelGain"} {
			if values["videoStudio."+key] == english["videoStudio."+key] {
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
	}
}
