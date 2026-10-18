package ui

import "testing"

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
