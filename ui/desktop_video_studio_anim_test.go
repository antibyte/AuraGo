package ui

import (
	"strings"
	"testing"
)

// clipState and placement in 2e (index §4.4, §4.4b):
//   - anim is the in/out animation at the frame, or null at rest; every other field keeps the keyed value.
//   - placement's optional fifth argument applies VideoStudioAnim.box before the fit and adds reveal (box fractions).
//     Without it the 2a–2d result is unchanged; with it the box equals the Go-generated box rows of anim.json exactly.
//
// Also checked: the registry helpers animPresets, animAvailable and transitionGroups, and the direct cases of the
// validation mirror VideoStudioAnim.valid.
func TestVideoStudioAnimState(t *testing.T) {
	runVideoStudioEffectsScript(t, `
const A = window.VideoStudioAnim;
const project = {width: 1280, height: 720, fps: 25};
const clip = {start: 10, duration: 40, offset: 0, x: 0.25, y: 0.25, width: 0.5, height: 0.5, rotation: 0, opacity: 0.8, fit: 'contain',
  anim_in: {preset: 'pop', duration: 10}, anim_out: {preset: 'wipe', duration: 8}};
// At rest, and outside the clip, the state carries no animation.
assert.equal(E.clipState(clip, 25, project).anim, null);
assert.equal(E.clipState(clip, 9, project).anim, null, 'before the clip');
// The clip's first frame: pop at p = 0. The keyed fields keep their values; only anim holds the animation.
const first = E.clipState(clip, 10, project);
assert.deepEqual(first.anim, {opacity: 0, dx: 0, dy: 0, scale: 0.6, blur: null, wipe: null});
assert.deepEqual([first.x, first.y, first.width, first.height, first.rotation, first.opacity], [0.25, 0.25, 0.5, 0.5, 0, 0.8]);
// Without the fifth argument placement is the keyed placement of 2a–2d, without a reveal.
assert.deepEqual(E.placement(first, project, project, null), {box: {x: 320, y: 180, w: 640, h: 360}, draw: {x: 320, y: 180, w: 640, h: 360}, clip: false, angle: 0});
// j = 7, p = 0.7: the 1.08 peak about the box centre (640, 360), 692 x 388; contain fits a 16:9 source into it.
const peak = E.clipState(clip, 17, project);
assert.equal(peak.anim.scale, 1.08);
const placed = E.placement(peak, project, project, {width: 1920, height: 1080}, peak.anim);
assert.deepEqual(placed.box, {x: 294, y: 166, w: 692, h: 388});
assert.deepEqual(placed.draw, {x: 295, y: 166, w: 690, h: 388});
assert.deepEqual(placed.reveal, [0, 0, 1, 1], 'no wipe: the whole box');
assert.deepEqual(E.placement(peak, {width: 640, height: 360}, project, null, peak.anim).box, {x: 147, y: 83, w: 346, h: 194}, 'the canvas scale applies after the exact box');
// The wipe out: j = 36, p = (39 - 36) / 8 = 0.375, R = round(0.375 * 640) = 240, so columns [400, 640) show.
const wiped = E.clipState(clip, 46, project);
assert.deepEqual(wiped.anim, {opacity: 1, dx: 0, dy: 0, scale: 1, blur: null, wipe: {side: 'out', p: 0.375}});
const wipedPlace = E.placement(wiped, project, project, null, wiped.anim);
assert.deepEqual(wipedPlace.box, {x: 320, y: 180, w: 640, h: 360});
assert.deepEqual(wipedPlace.reveal, [0.625, 0, 1, 1]);
// placement with an animation equals Go's animBox on every box row of the parity fixture (the rows' rest boxes are even
// clip boxes inside their canvas, so a clip state rebuilds them exactly).
const fs = require('node:fs'), path = require('node:path');
const fixture = JSON.parse(fs.readFileSync(path.join('..', 'internal', 'videostudio', 'testdata', 'parity', 'anim.json'), 'utf8'));
const boxes = fixture.cases.filter(c => c.input.kind === 'box');
assert.equal(boxes.length, 10);
for (const c of boxes) {
  const canvas = c.input.canvas, b = c.input.box;
  const state = E.clipState({x: b.x / canvas.width, y: b.y / canvas.height, width: b.w / canvas.width, height: b.h / canvas.height, fit: 'cover'}, 0, canvas);
  const anim = Object.assign({opacity: 1, blur: null, wipe: null}, c.input.anim);
  assert.deepStrictEqual(E.placement(state, canvas, canvas, null, anim).box, c.want, c.name);
}
// Registry helpers.
E.adopt(undefined, undefined);
assert.deepEqual(E.animPresets().map(p => [p.id, p.label_key, p.available]), [['fade', 'anim_fade', true], ['slide_up', 'anim_slide_up', true],
  ['slide_down', 'anim_slide_down', true], ['slide_left', 'anim_slide_left', true], ['slide_right', 'anim_slide_right', true],
  ['pop', 'anim_pop', true], ['wipe', 'anim_wipe', true], ['blur_in', 'anim_blur_in', true]]);
assert.deepEqual(E.animPresets()[0].filters, ['sendcmd', 'colorchannelmixer']);
E.adopt(E.BUILTIN, {effects: [], transitions: ['fadegrays'], blend_modes: [], anim_presets: ['blur_in']});
assert.equal(E.animAvailable('blur_in'), false);
assert.equal(E.animAvailable('pop'), true);
assert.equal(E.animAvailable('spin'), false, 'an unknown preset is not available');
assert.equal(E.animPresets().find(p => p.id === 'blur_in').available, false);
const groups = E.transitionGroups();
assert.deepEqual(groups.map(g => [g.category, g.label_key, g.items.length]), [['basic', 'transitionCategory_basic', 5], ['wipe', 'transitionCategory_wipe', 9],
  ['motion', 'transitionCategory_motion', 6], ['shape', 'transitionCategory_shape', 4]]);
assert.deepEqual(groups[0].items.map(i => i.id), ['dissolve', 'black', 'fadewhite', 'fadegrays', 'pixelize']);
assert.equal(groups[0].items.find(i => i.id === 'fadegrays').available, false, 'items are transitions() entries');
// The validation mirror (the shared table of TestVideoStudioPictureValidationMatchesGo covers the Go agreement).
const overlay = {id: 'o', kind: 'overlay'}, audio = {id: 'a', kind: 'audio'};
assert.equal(A.valid({anim_in: null}, audio, project), true, 'null is no animation');
assert.equal(A.valid({anim_in: {preset: 'fade', duration: 13}}, audio, project), false, 'audio clips have none');
assert.equal(A.valid({anim_in: {preset: 'fade', duration: 13}}, null, project), false, 'without a track the clip counts as audio');
assert.equal(A.valid({anim_out: {preset: 'blur_in', duration: 75}}, overlay, project), true, '75 frames is the 25 fps maximum');
assert.equal(A.valid({anim_out: {preset: 'blur_in', duration: 76}}, overlay, project), false);
assert.equal(A.valid({anim_out: {preset: 'blur_in', duration: 12.5}}, overlay, project), false, 'whole frames only');
assert.equal(A.valid({anim_out: {preset: 'blur_in'}}, overlay, project), false, 'a missing duration');
assert.equal(A.valid({anim_in: 'pop'}, overlay, project), false, 'not an object');
assert.equal(A.valid({anim_in: {preset: 'blur_in', duration: 13}}, overlay, project), true, 'an unavailable preset validates; only the export refuses it');
E.adopt(undefined, undefined);
console.log('ok');
`)
}

// A frame-rate change keeps each animation's length in seconds (index §4.24 edit semantics): round(duration·ratio),
// clamped into the new rate's range (VideoStudioTime.rescaleProject through VideoStudioAnim.rescale). A clip without
// an animation gets none.
func TestVideoStudioRescaleAnim(t *testing.T) {
	runVideoStudioEffectsScript(t, `
const T = window.VideoStudioTime, A = window.VideoStudioAnim;
const project = (fps, ntsc, inAnim, outAnim) => ({fps, ntsc, width: 1280, height: 720, assets: [{id: 'logo', kind: 'image', width: 64, height: 64}],
  tracks: [{id: 'o', kind: 'overlay', clips: [
    {id: 'a', asset_id: 'logo', start: 0, offset: 0, duration: 100, anim_in: inAnim, anim_out: outAnim},
    {id: 'b', asset_id: 'logo', start: 100, offset: 0, duration: 50}]}]});
let p = T.rescaleProject(project(25, false, {preset: 'pop', duration: 13}, {preset: 'fade', duration: 75}), 30, false);
assert.deepEqual(p.tracks[0].clips[0].anim_in, {preset: 'pop', duration: 16}, '13 x 1.2 = 15.6 rounds to 16');
assert.deepEqual(p.tracks[0].clips[0].anim_out, {preset: 'fade', duration: 90}, '75 x 1.2 = 90, the 30 fps maximum');
assert.equal('anim_in' in p.tracks[0].clips[1] || 'anim_out' in p.tracks[0].clips[1], false, 'a clip without animation gets none');
p = T.rescaleProject(project(25, false, {preset: 'pop', duration: 13}, {preset: 'fade', duration: 75}), 24, false);
assert.deepEqual([p.tracks[0].clips[0].anim_in.duration, p.tracks[0].clips[0].anim_out.duration], [12, 72], '12.48 and 72');
p = T.rescaleProject(project(60, false, {preset: 'wipe', duration: 12}, {preset: 'blur_in', duration: 180}), 24, false);
assert.deepEqual([p.tracks[0].clips[0].anim_in.duration, p.tracks[0].clips[0].anim_out.duration], [5, 72], '4.8 clamps up to the 24 fps minimum 5');
p = T.rescaleProject(project(24, false, {preset: 'wipe', duration: 5}, {preset: 'blur_in', duration: 72}), 60, false);
assert.deepEqual([p.tracks[0].clips[0].anim_in.duration, p.tracks[0].clips[0].anim_out.duration], [13, 180], '12.5 rounds away from zero');
p = T.rescaleProject(project(30, true, {preset: 'slide_up', duration: 15}, null), 30, false);
assert.deepEqual(p.tracks[0].clips[0].anim_in, {preset: 'slide_up', duration: 15}, '29.97 to 30 fps keeps 15 frames');
assert.equal(p.tracks[0].clips[0].anim_out, null, 'a null side stays null');
assert.deepEqual(A.rescale({preset: 'fade', duration: 75}, 3, {fps: 25}), {preset: 'fade', duration: 75}, 'clamped to the maximum');
console.log('ok');
`)
}

// TestVideoStudioAnimUniforms checks the GL side of the 2e animation without a browser (plan 2e Task 12; index §4.4b,
// §4.7, §4.8):
//   - blurUniforms gives nothing without blur_in. With it: the 2c BLUR pairs at σ = S and S/4 (S = 0.02 · target
//     height), then two ANIM_MIX passes with u_mix = m1/255 and m2/255 from the state's blurMasks;
//   - placement's reveal is VideoStudioAnim.revealPx in box fractions, for an in and an out wipe;
//   - PLACE and PLACE_MASKED read u_reveal and v_box, the reveal after the mask; ANIM_MIX follows the pass contract;
//   - a failed blur_in mix ('effect', 'anim:blur_in') shows previewEffectFallback.
func TestVideoStudioAnimUniforms(t *testing.T) {
	runVideoStudioEffectsScript(t, `
// video-studio-anim.js comes with videoStudioEffectsPrelude (Task 4).
require('./js/desktop/apps/video-studio-shaders.js');
const A = window.VideoStudioAnim, P = window.VideoStudioPicture, S = window.VideoStudioShaders;
const near = (got, want, message) => assert(Math.abs(got - want) < 1e-9, message + ': got ' + got + ', want ' + want);
const project = {version: 3, name: 'Anim', width: 640, height: 360, fps: 25, assets: [], tracks: []};
const clipOf = extra => Object.assign({id: 'c', asset_id: 'a', start: 100, offset: 0, duration: 40, x: 0.25, y: 0.25, width: 0.5, height: 0.5, rotation: 0, opacity: 1, fit: 'cover'}, extra);
const size = {width: 320, height: 180};
assert.deepEqual(A.blurUniforms(null, size), [], 'at rest: no passes');
const fade = E.clipState(clipOf({anim_in: {preset: 'fade', duration: 10}}), 104, project);
assert(fade.anim && fade.anim.blur === null, 'a fade has no blur');
assert.deepEqual(A.blurUniforms(fade.anim, size), [], 'a fade draws no blur pass');
// blur_in at p = 0.4 (frame 4 of a 10-frame span).
const blurred = E.clipState(clipOf({anim_in: {preset: 'blur_in', duration: 10}}), 104, project);
assert.deepEqual(blurred.anim.blur, A.blurMasks(0.4), 'the state carries blurMasks(p)');
const passes = A.blurUniforms(blurred.anim, size);
assert.deepEqual(passes.map(pass => pass.kind), ['blur', 'blur', 'blur', 'blur', 'mix', 'mix']);
const wide = P.passUniforms('blur', {radius: A.BLUR_RADIUS}, size), narrow = P.passUniforms('blur', {radius: A.BLUR_RADIUS / 4}, size);
assert.deepEqual(passes.slice(0, 4).map(pass => pass.uniforms), wide.concat(narrow), 'the 2c BLUR pairs, S first');
near(passes[0].uniforms.u_sigma, 0.02 * 180, 'S = 0.02 x the target height');
near(passes[2].uniforms.u_sigma, 0.02 * 180 / 4, 'then S/4');
assert.deepEqual([passes[0].uniforms.u_direction, passes[1].uniforms.u_direction], [[1 / 320, 0], [0, 1 / 180]], 'horizontal, then vertical');
near(passes[4].uniforms.u_mix, blurred.anim.blur.m1 / 255, 'the first mix weighs g(S/4) over g(S) by m1/255');
near(passes[5].uniforms.u_mix, blurred.anim.blur.m2 / 255, 'the second weighs the sharp picture by m2/255');
const late = E.clipState(clipOf({anim_in: {preset: 'blur_in', duration: 10}}), 109, project).anim;
assert.equal(late.blur.m1, 255, 'p = 0.9: the narrow node only');
assert(late.blur.m2 > 0 && late.blur.m2 < 255, 'p = 0.9: the sharp share grows');
near(A.blurUniforms(late, size)[5].uniforms.u_mix, late.blur.m2 / 255, 'u_mix = m2/255');
// Wipes: placement's reveal is revealPx in box fractions; 40 % of the box shows at p = 0.4.
[['in', clipOf({anim_in: {preset: 'wipe', duration: 10}}), 104], ['out', clipOf({anim_out: {preset: 'wipe', duration: 10}}), 135]].forEach(([side, clip, frame]) => {
  const st = E.clipState(clip, frame, project), place = E.placement(st, project, project, {width: 320, height: 180}, st.anim);
  assert.equal(st.anim.wipe.side, side);
  const columns = A.revealPx(st.anim, place.box.w);
  assert.deepEqual(Array.from(place.reveal), [columns[0] / place.box.w, 0, columns[1] / place.box.w, 1], side + ' reveal');
  assert.equal(columns[1] - columns[0], Math.round(0.4 * place.box.w), side + ': 40 % of the box');
  assert.equal(side === 'in' ? columns[0] : columns[1], side === 'in' ? 0 : place.box.w, side + ': reads left to right');
});
// The sources.
assert(S.PLACE.includes('in vec2 v_box;') && S.PLACE.includes('uniform vec4 u_reveal;') && S.PLACE.includes('vsReveal(v_box)'), 'PLACE hides alpha outside the reveal');
assert(S.PLACE_MASKED.includes('uniform vec4 u_reveal;') && S.PLACE_MASKED.includes('vsMask(v_box * u_boxSize) * vsReveal(v_box)'), 'the reveal comes after the mask');
assert(S.ANIM_MIX.startsWith('#version 300 es') && ['uniform sampler2D u_source;', 'uniform vec2 u_size;', 'uniform sampler2D u_base;', 'uniform float u_mix;'].every(text => S.ANIM_MIX.includes(text)), 'ANIM_MIX follows the pass contract');
// A failed blur_in mix is an effect failure: the stage shows previewEffectFallback.
window.VideoStudioParts = {PREF_TIMELINE: 'vs.timeline', tr: (s, key) => key};
require('./js/desktop/apps/video-studio-stage.js');
const notice = {textContent: '', hidden: true};
window.VideoStudioParts.syncPreviewNotice({project: {}, q: selector => (selector === '[data-preview-notice]' ? notice : null), preview: {painterKind: () => 'gl', failedPrograms: () => [{kind: 'effect', id: 'anim:blur_in'}]}});
assert.deepEqual([notice.textContent, notice.hidden], ['previewEffectFallback', false]);
console.log('ok');
`)
}

// Edit semantics of the in and out animations (2e, Task 14; spec §5.6, §8.1; index §4.16, §4.24):
//   - setAnim: set, change, None, clamping into frameRange, and the refusals (unknown side, preset or value; audio
//     clips; locked tracks); an unavailable preset is kept.
//   - Pieces follow the audio-fade rule (split, blade, insert and overwrite): anim_in stays on the piece that keeps the
//     start, anim_out on the one that keeps the end.
//   - Duplicate, copy and paste keep both sides; reverse keeps both (no swap); trims and speed changes keep both.
//   - Freeze: the still has none, and the parts around it keep theirs. Detach: the audio copy has none.
//   - A rate change rescales both lengths (rescaleProject; TestVideoStudioRescaleAnim pins the clamping itself).
//   - Paste attributes, group animation: it copies both sides (a source without animation removes the target's), skips
//     audio targets, and the other groups leave the animations alone.
//
// The group's label is binding in en and de; TestVideoStudioEffectEditSemantics covers the other locales.
func TestVideoStudioAnimEdits(t *testing.T) {
	for lang, want := range map[string]string{"en": "In and out animations", "de": "Animationen am Anfang und Ende"} {
		if got := readDesktopLocale(t, lang)["videoStudio.attrAnimation"]; got != want {
			t.Errorf("%s: videoStudio.attrAnimation = %q, want %q", lang, got, want)
		}
	}
	runVideoStudioKeyframesScript(t, `
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-anim.js');
const AN = window.VideoStudioAnim;
const editing = require('fs').readFileSync('./js/desktop/apps/video-studio-editing.js', 'utf8');
assert(editing.includes('value="animation"') && editing.includes("label('attrAnimation', 'In and out animations')"), 'the paste attributes dialog offers the animation group');
assert.deepEqual(AN.frameRange(project([])), {min: 6, max: 90, default: 15}, '30 fps: 0.2 s to 3 s, 0.5 s by default');
const fresh = () => project([
  track('v1','video',[clip('c',V,0,100,{offset:100})]),
  track('o1','overlay',[clip('t',I,0,60)]),
  track('a1','audio',[clip('m',A,0,100)])
]);
let p = fresh();
const c = () => at(p, 'c');

// setAnim: a new animation takes the default length; a preset change keeps the length; a length is rounded and clamped
// into the range. A length longer than half the clip is valid (the render clamps).
assert.equal(K.setAnim(p, 'c', 'in', {preset: 'pop'}), true);
assert.deepEqual(c().anim_in, {preset: 'pop', duration: 15});
assert.deepEqual(Object.keys(c().anim_in), ['preset', 'duration'], 'the canonical key order');
assert.equal(K.setAnim(p, 'c', 'in', {preset: 'fade'}), true);
assert.deepEqual(c().anim_in, {preset: 'fade', duration: 15});
assert.equal(K.setAnim(p, 'c', 'in', {duration: 40.4}), true);
assert.deepEqual(c().anim_in, {preset: 'fade', duration: 40});
assert.equal(K.setAnim(p, 'c', 'in', {duration: 2}), true);
assert.equal(c().anim_in.duration, 6, 'below 0.2 s the length is clamped up');
assert.equal(K.setAnim(p, 'c', 'in', {duration: 500}), true);
assert.equal(c().anim_in.duration, 90, 'above 3 s the length is clamped down');
assert.equal(K.setAnim(p, 'c', 'in', {duration: 90}), 'nothing', 'no change');
assert.equal(K.setAnim(p, 'c', 'out', {preset: 'slide_left', duration: 20}), true);
assert.deepEqual(c().anim_out, {preset: 'slide_left', duration: 20});
valid(p);
const before = JSON.stringify(p);
for (const [side, values] of [['up', {preset: 'fade'}], ['in', {}], ['in', null], ['in', {preset: 'spin'}], ['in', {preset: ''}], ['in', {duration: 'x'}], ['in', {duration: null}], ['in', {duration: true}]]) {
  assert.equal(K.setAnim(p, 'c', side, values), 'nothing', 'refuses ' + side + ' ' + JSON.stringify(values));
}
assert.equal(JSON.stringify(p), before, 'a refused value changes nothing');
// A length alone needs an animation; None removes the field, and None again changes nothing; stills take animations.
assert.equal(K.setAnim(p, 't', 'in', {duration: 20}), 'nothing', 'a length alone needs a preset');
assert.equal(K.setAnim(p, 't', 'out', {preset: 'wipe', duration: 30}), true, 'stills take animations');
assert.deepEqual(at(p,'t').anim_out, {preset: 'wipe', duration: 30});
assert.equal(K.setAnim(p, 't', 'out', {preset: 'none'}), true);
assert.equal('anim_out' in at(p,'t'), false, 'None removes the field');
assert.equal(K.setAnim(p, 't', 'out', {preset: 'none'}), 'nothing');
// Audio clips and locked tracks refuse; an unavailable preset is kept (only the export refuses it).
assert.equal(K.setAnim(p, 'm', 'in', {preset: 'fade'}), 'kind');
p.tracks[1].locked = true;
assert.equal(K.setAnim(p, 't', 'in', {preset: 'fade'}), 'locked');
p.tracks[1].locked = false;
FX.adopt(FX.BUILTIN, {effects: [], transitions: [], blend_modes: [], anim_presets: ['blur_in']});
assert.equal(FX.animAvailable('blur_in'), false);
assert.equal(K.setAnim(p, 't', 'in', {preset: 'blur_in'}), true, 'an unavailable preset is kept, like an unavailable blend mode');
FX.adopt(null, null);
valid(p);

// Pieces (the audio-fade rule): anim_in stays on the piece that keeps the clip's start, anim_out on the one that keeps
// its end, like the fades.
const both = () => [{preset: 'pop', duration: 15}, {preset: 'fade', duration: 20}];
const animated = () => project([track('v1','video',[clip('c',V,0,100,{offset:100, fade_in: 10, fade_out: 12, anim_in: both()[0], anim_out: both()[1]})]), track('a1','audio',[])]);
const edges = clips => clips.slice().sort((a, b) => a.start - b.start).map(item => [item.start, item.duration, item.anim_in ? item.anim_in.preset : '', item.anim_out ? item.anim_out.preset : '']);
p = animated();
valid(p);
assert.equal(E.blade(p, 40, {clipIds: ['c'], linked: false}, {}), true);
const right = p.tracks[0].clips.find(item => item.id !== 'c');
assert.deepEqual(edges(p.tracks[0].clips), [[0, 40, 'pop', ''], [40, 60, '', 'fade']], 'split: the left part keeps anim_in, the right part anim_out');
assert.deepEqual([at(p,'c').fade_in, at(p,'c').fade_out, right.fade_in, right.fade_out], [10, 0, 0, 12], 'the same rule as the fades');
valid(p);
p = animated();
assert.equal(E.insertGap(p, 40, 10, ['v1']), true);
assert.deepEqual(edges(p.tracks[0].clips), [[0, 40, 'pop', ''], [50, 60, '', 'fade']], 'an insert splits by the same rule');
valid(p);
p = animated();
assert.equal(E.overwrite(p, [{trackId: 'v1', clips: [clip('n',V,30,20)]}]), true);
assert.deepEqual(edges(p.tracks[0].clips), [[0, 30, 'pop', ''], [30, 20, '', ''], [50, 50, '', 'fade']], 'an overwrite in the middle');
valid(p);
p = animated();
assert.equal(E.overwrite(p, [{trackId: 'v1', clips: [clip('h',V,0,10), clip('e',V,90,10)]}]), true);
assert.deepEqual([at(p,'c').start, at(p,'c').duration, 'anim_in' in at(p,'c'), 'anim_out' in at(p,'c')], [10, 80, false, false], 'a piece that keeps neither edge has neither');
valid(p);

// Duplicate, copy and paste keep both sides; reverse keeps both (no swap); trims and speed changes keep both.
p = animated();
const out = {};
assert.equal(E.duplicateClips(p, ['c'], out), true);
assert.deepEqual([at(p, out.created[0]).anim_in, at(p, out.created[0]).anim_out], both(), 'a duplicate keeps both');
const board = E.copyClips(p, ['c']);
assert.deepEqual([board.entries[0].clip.anim_in, board.entries[0].clip.anim_out], both(), 'the clipboard holds both');
const pasted = {};
assert.equal(E.paste(p, board, 300, 'overwrite', {out: pasted}), true);
assert.deepEqual([at(p, pasted.created[0]).anim_in, at(p, pasted.created[0]).anim_out], both(), 'a pasted copy keeps both');
valid(p);
p = animated();
p.assets.push({id:'r', kind:'video', has_audio:true, duration_frames:100});
assert.equal(E.reverseClips(p, 'c', {assetId:'r', rangeStart:100, frames:100, sourceAssetId:'v'}), true);
assert.deepEqual([at(p,'c').anim_in, at(p,'c').anim_out], both(), 'reverse keeps the in and the out animation where they are');
valid(p);
p = animated();
assert.equal(E.rippleTrim(p, ['c'], 'end', -40), true);
assert.equal(E.setSpeed(p, ['c'], 2, true), true);
assert.deepEqual([at(p,'c').anim_in, at(p,'c').anim_out], both(), 'trims and speed changes keep both (the render clamps a span to half the clip)');
valid(p);

// Freeze frame: the still has no animation; the parts around it keep their edges. Detach: the audio copy has none.
p = animated();
assert.equal(E.freezeFrame(p, 'c', 50, {asset_id: 'i', clip_id: 'fz', duration: 30}), true);
const still = at(p, 'fz');
assert.deepEqual(['anim_in' in still, 'anim_out' in still], [false, false], 'the still has no animation');
assert.deepEqual(edges(p.tracks[0].clips.filter(item => item.id !== 'fz')), [[0, 50, 'pop', ''], [80, 50, '', 'fade']], 'the parts around the still keep their edges');
valid(p);
p = animated();
assert.equal(E.detachAudio(p, 'c', {clip: 'au', link: 'l1', track: 'a2', trackName: 'Audio 2'}), true);
assert.deepEqual(['anim_in' in at(p,'au'), 'anim_out' in at(p,'au')], [false, false], 'the audio copy has no animation');
assert.deepEqual([at(p,'c').anim_in, at(p,'c').anim_out], both(), 'the video clip keeps both');
valid(p);

// Rate change: round(duration × ratio) inside the new range (Task 5's rescale).
p = animated();
window.VideoStudioTime.rescaleProject(p, 24, false);
assert.deepEqual([at(p,'c').anim_in, at(p,'c').anim_out], [{preset: 'pop', duration: 12}, {preset: 'fade', duration: 16}], '30 to 24 fps');
valid(p);
p = project([track('v1','video',[clip('c',V,0,100,{offset:100, anim_in: {preset: 'fade', duration: 90}, anim_out: {preset: 'pop', duration: 6}})])]);
window.VideoStudioTime.rescaleProject(p, 60, false);
assert.deepEqual([at(p,'c').anim_in, at(p,'c').anim_out], [{preset: 'fade', duration: 180}, {preset: 'pop', duration: 12}], '30 to 60 fps: the range ends move with the rate');
valid(p);

// Paste attributes, group animation.
assert.deepEqual(E.ATTRIBUTE_GROUPS.animation, ['anim_in', 'anim_out']);
const source = () => at(animated(), 'c');
const targets = () => project([track('v1','video',[clip('d',V,200,100,{offset:300, anim_out: {preset: 'wipe', duration: 30}})]), track('o1','overlay',[clip('t',I,0,60)]), track('a1','audio',[clip('m',A,0,100)])]);
p = targets();
valid(p);
assert.equal(E.pasteAttributes(p, source(), ['d', 't', 'm'], ['animation'], 250), true);
for (const id of ['d', 't']) assert.deepEqual([at(p,id).anim_in, at(p,id).anim_out], both(), id + ' takes both sides');
assert.deepEqual(['anim_in' in at(p,'m'), 'anim_out' in at(p,'m')], [false, false], 'audio clips take no animation');
valid(p);
assert.equal(E.pasteAttributes(p, source(), ['m'], ['animation'], 250), 'nothing', 'audio targets alone change nothing');
const outOnly = source();
delete outOnly.anim_in;
assert.equal(E.pasteAttributes(p, outOnly, ['d'], ['animation'], 250), true);
assert.deepEqual(['anim_in' in at(p,'d'), at(p,'d').anim_out], [false, {preset: 'fade', duration: 20}], 'a side the source lacks is removed');
const odd = Object.assign(source(), {anim_in: {preset: 'spin', duration: 20}, anim_out: {preset: 'fade', duration: 500}});
assert.equal(E.pasteAttributes(p, odd, ['d'], ['animation'], 250), true);
assert.deepEqual(['anim_in' in at(p,'d'), at(p,'d').anim_out], [false, {preset: 'fade', duration: 90}], 'an unknown preset is removed, a length is clamped');
const plain = at(project([track('v1','video',[clip('s',V,0,100)])]), 's');
assert.equal(E.pasteAttributes(p, plain, ['d', 't'], ['animation'], 250), true);
for (const id of ['d', 't']) assert.deepEqual(['anim_in' in at(p,id), 'anim_out' in at(p,id)], [false, false], 'a source without animation clears ' + id);
valid(p);
p = project([track('v1','video',[clip('d',V,200,100,{offset:300, anim_in: {preset: 'blur_in', duration: 25}})])]);
assert.equal(E.pasteAttributes(p, plain, ['d'], ['picture', 'basePicture', 'effects'], 250), true);
assert.deepEqual(at(p,'d').anim_in, {preset: 'blur_in', duration: 25}, 'the other groups leave the animations alone');
valid(p);
`)
}

// The inspector's Animation section (2e, Task 15; spec §8.1; index §4.24) and its locale keys:
//   - Every visual clip shows the in and out rows, titles and stickers included; audio clips show none. The speed ramp
//     stays on video-track clips of video media, after the rows.
//   - The list is None plus the presets in menu order, labelled by label_key. An unavailable preset is disabled with a
//     title, except the clip's own.
//   - The length slider runs over frameRange in frames, with its output in seconds; it is disabled without an animation.
//   - The clamp hint shows only while the stored length is longer than half the clip, and names the length the render
//     plays.
//   - P.animChange takes the preset lists and the length sliders (one applyEdit each) and nothing else.
//   - 2f F1: animClamped has U+00A0 between {{seconds}} and the unit in every locale and in the JS fallback, so the
//     hint never breaks between "2.5" and "s".
func TestVideoStudioAnimInspector(t *testing.T) {
	binding := map[string][2]string{
		"animIn":          {"In animation", "Animation am Anfang"},
		"animOut":         {"Out animation", "Animation am Ende"},
		"animLength":      {"Length", "Länge"},
		"animClamped":     {"This clip is short: the animation plays over {{seconds}}\xc2\xa0s, half the clip.", "Dieser Clip ist kurz: Die Animation dauert {{seconds}}\xc2\xa0s, die halbe Cliplänge."},
		"animUnavailable": {"This server's FFmpeg cannot export this animation.", "Das FFmpeg dieses Servers kann diese Animation nicht exportieren."},
	}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		values := readDesktopLocale(t, lang)
		for key := range binding {
			text := strings.TrimSpace(values["videoStudio."+key])
			if text == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			} else if lang != "en" && text == english["videoStudio."+key] {
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
		if !strings.Contains(values["videoStudio.animClamped"], "{{seconds}}") {
			t.Errorf("%s: videoStudio.animClamped lacks {{seconds}}", lang)
		}
		if strings.Contains(values["videoStudio.animClamped"], "{{seconds}} ") {
			t.Errorf("%s: videoStudio.animClamped has a plain space after {{seconds}}; use U+00A0 so the unit stays with the number", lang)
		}
	}
	for key, texts := range binding {
		for i, lang := range []string{"en", "de"} {
			if got := readDesktopLocale(t, lang)["videoStudio."+key]; got != texts[i] {
				t.Errorf("%s: videoStudio.%s = %q, want %q", lang, key, got, texts[i])
			}
		}
	}
	runVideoStudioKeyframesScript(t, videoStudioEffectStackPartsPrelude+`
require('./js/desktop/apps/video-studio-anim.js');
const AN = window.VideoStudioAnim;
const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => '&#' + ch.charCodeAt(0) + ';');
const title = {font_family: 'Arial', font_size: 72, color: '#ffffff', alignment: 'center'};
const p = project([
  track('v1','video',[clip('c',V,0,100,{offset:100})]),
  track('o1','overlay',[clip('t',I,0,60,{text: 'Title', text_style: title}), clip('k',I,100,50)]),
  track('a1','audio',[clip('m',A,0,100)])
]);
valid(p);
const tr = (key, fallback) => key === 'animClamped' ? 'SHORT {{seconds}}' : key === 'animUnavailable' ? 'NO EXPORT' : '[' + key + ']';
const render = (clipId, asset) => {
  const host = {innerHTML: ''};
  IN.render(host, {selected: T.clipFor(p, clipId), asset, project: p, locale: 'en', caps: {}, transitionInfo: {next: null}, esc,
    icon: name => '<i data-icon="' + name + '"></i>', tr, keyframes: P.keyframeView({project: p, selectedClipId: clipId, frame: 10, readonly: false}), fineOpen: false, effects: null});
  return host.innerHTML;
};
const side = (html, name) => { const start = html.indexOf('data-anim-side="' + name + '"'); assert(start >= 0, 'no ' + name + ' row'); return html.slice(start, html.indexOf('</div>', start)); };

// A video clip without animations: None on both sides, sliders disabled at the default 15 frames (0.5 s at 30 fps) over
// 6..90, no hint. The rows come after Effects and before the speed ramp.
let html = render('c', V);
assert(html.includes('vs-anim-section') && html.includes('data-ramp-preset="ramp_up"'), 'a video clip shows the animations and the speed ramp');
assert(html.indexOf('data-fx-section') < html.indexOf('data-anim-side="in"'), 'the Animation section follows Effects');
assert(html.indexOf('data-anim-side="in"') < html.indexOf('data-anim-side="out"') && html.indexOf('data-anim-side="out"') < html.indexOf('data-ramp-preset'), 'In, Out, then the speed ramp');
assert.deepEqual(Array.from(side(html, 'in').matchAll(/<option value="([a-z_]+)"/g), m => m[1]), ['none', 'fade', 'slide_up', 'slide_down', 'slide_left', 'slide_right', 'pop', 'wipe', 'blur_in'], 'None plus the presets in menu order');
assert(side(html, 'in').includes('<option value="none" selected>[none]</option>') && side(html, 'in').includes('<option value="pop">[anim_pop]</option>'), 'None first; labels from label_key');
for (const name of ['in', 'out']) {
  assert(new RegExp('<input type="range" data-anim-duration="' + name + '" min="6" max="90" step="1" value="15"[^>]*disabled').test(html), name + ': a disabled slider at the default length');
  assert(side(html, name).includes('<output>0.5 s</output>') && side(html, name).includes('data-anim-clamped="' + name + '" hidden'), name + ': 0.5 s and no hint');
}
assert(side(html, 'in').includes('aria-label="[animIn]: [animLength]"') && side(html, 'out').includes('<span class="vs-fx-subhead">[animOut]</span>'), 'each side names its list and its slider');

// Stored animations: the presets are selected and the sliders hold the lengths. The out side's 80 frames are more than
// half the 100-frame clip, so the hint names the 50 frames (1.7 s) the render plays.
at(p,'c').anim_in = {preset: 'pop', duration: 20};
at(p,'c').anim_out = {preset: 'fade', duration: 80};
valid(p);
html = render('c', V);
assert(side(html, 'in').includes('<option value="pop" selected>[anim_pop]</option>') && side(html, 'out').includes('<option value="fade" selected>[anim_fade]</option>'));
assert(/<input type="range" data-anim-duration="in" min="6" max="90" step="1" value="20" aria-label="[^"]*">/.test(html), 'an enabled slider at the stored length');
assert(side(html, 'in').includes('<output>0.7 s</output>') && side(html, 'in').includes('data-anim-clamped="in" hidden'), '20 frames fit in half the clip');
assert(!side(html, 'out').includes('data-anim-clamped="out" hidden') && side(html, 'out').includes('<span>SHORT 1.7</span>'), 'the out side plays over 50 frames');
assert.equal(IN.animClampText({tr, locale: 'en', project: p}, at(p,'c'), 'out'), 'SHORT 1.7');
assert.equal(IN.animClampText({tr, locale: 'en', project: p}, at(p,'c'), 'in'), '');
// Without a locale text the fallback keeps the number and its unit together (U+00A0, 2f F1).
assert.equal(IN.animClampText({tr: (key, fallback) => fallback, locale: 'en', project: p}, at(p,'c'), 'out'), 'This clip is short: the animation plays over 1.7\xa0s, half the clip.');

// A preset the server's FFmpeg cannot export is disabled with a title, except the clip's own.
FX.adopt(FX.BUILTIN, {effects: [], transitions: [], blend_modes: [], anim_presets: ['pop', 'blur_in']});
html = render('c', V);
assert(side(html, 'in').includes('<option value="pop" selected title="NO EXPORT">'), 'the current preset stays selectable');
assert(side(html, 'in').includes('<option value="blur_in" disabled title="NO EXPORT">') && side(html, 'out').includes('<option value="pop" disabled title="NO EXPORT">'), 'other unavailable presets are disabled');
assert(side(html, 'in').includes('<option value="fade">[anim_fade]</option>'), 'available presets carry no title');
FX.adopt(null, null);

// Titles and stickers (stills on overlay tracks) take the animations without the speed ramp; audio clips take neither.
for (const id of ['t', 'k']) {
  const other = render(id, I);
  assert(other.includes('data-anim-side="in"') && other.includes('data-anim-side="out"') && !other.includes('data-ramp-preset'), id + ': animations, no speed ramp');
}
assert(!render('m', A).includes('vs-anim-section'), 'audio clips have no Animation section');

// The slider range follows the project rate: 25 fps gives 5..75 frames, 13 by default.
p.fps = 25;
assert.deepEqual(AN.frameRange(p), {min: 5, max: 75, default: 13});
assert(/data-anim-duration="in" min="5" max="75" step="1" value="13"/.test(render('k', I)));
p.fps = 30;

// P.animChange takes the preset lists and the length sliders (one applyEdit each), and nothing else.
const edits = [];
P.applyEdit = (state, label, op) => { edits.push(label); return op(state.project, {}) === true; };
const s = {project: p, selectedClipId: 'k', frame: 110, readonly: false, q: () => null};
const control = (selector, data, value) => ({value, dataset: data, matches: sel => sel === selector, closest: () => null});
assert.equal(P.animChange(s, control('select[data-anim-preset]', {animPreset: 'in'}, 'wipe')), true);
assert.deepEqual(at(p,'k').anim_in, {preset: 'wipe', duration: 15});
assert.equal(P.animChange(s, control('input[type="range"][data-anim-duration]', {animDuration: 'in'}, '45')), true);
assert.equal(at(p,'k').anim_in.duration, 45);
assert.equal(P.animChange(s, control('select[data-anim-preset]', {animPreset: 'in'}, 'none')), true);
assert.equal('anim_in' in at(p,'k'), false, 'None removes the side');
assert.deepEqual(edits, ['animation', 'animation length', 'animation'], 'one edit per commit');
assert.equal(P.animChange(s, control('select[data-blend]', {}, 'multiply')), false, 'other inputs are not animation changes');
`)
}
