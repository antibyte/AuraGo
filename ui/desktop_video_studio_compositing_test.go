package ui

import (
	"strings"
	"testing"
)

// clipState in 2d (index §4.4): blend is the clip's mode or 'normal'; mask is null, or the mask at the frame with its
// numbers through valueAt (keys at the source position, else the field) and the static shape and invert. The registry
// helpers of 2d (blendModes, blendAvailable, maskAnimated, MASK_PATHS) and the mask rules of animatablePaths and
// paramSpec are checked alongside.
func TestVideoStudioCompositingState(t *testing.T) {
	runVideoStudioEffectsScript(t, `
const near = (got, want, label) => assert(Math.abs(got - want) < 1e-9, label + ': ' + got + ' != ' + want);
const project = {fps: 25};
const clip = {start: 100, offset: 20, duration: 60, blend: 'screen',
  mask: {shape: 'ellipse', x: 0.1, y: 0.2, w: 0.5, h: 0.4, rotation: 30, feather: 0.05, invert: true},
  keyframes: {'mask.x': [{f: 20, v: 0, e: 'linear'}, {f: 40, v: 0.4, e: 'linear'}], 'mask.feather': [{f: 30, v: 0.1, e: 'hold'}, {f: 50, v: 0.3, e: 'linear'}]},
  effects: [{id: 'chroma_key', on: true, params: E.defaults('chroma_key')}]};
const at = frame => E.clipState(clip, frame, project).mask;
near(at(100).x, 0, 'source 20 sits on the first mask.x key');
near(at(110).x, 0.2, 'source 30, halfway between the keys');
near(at(130).x, 0.4, 'source 50, after the last key');
near(at(100).feather, 0.1, 'before the first feather key its value holds');
near(at(125).feather, 0.1, 'hold keeps the value until the next key');
near(at(130).feather, 0.3, 'and switches on it');
assert.deepEqual(Object.keys(at(110)), ['shape', 'invert', 'x', 'y', 'w', 'h', 'rotation', 'feather']);
assert.deepEqual([at(110).shape, at(110).invert, at(110).y, at(110).w, at(110).h, at(110).rotation], ['ellipse', true, 0.2, 0.5, 0.4, 30], 'paths without keys come from the field');
const st = E.clipState(clip, 110, project);
assert.equal(st.blend, 'screen');
assert.deepEqual(st.effects.map(e => [e.id, e.values.color, e.values.similarity]), [['chroma_key', '#00b140', E.defaults('chroma_key').similarity]], 'chroma_key reaches the painters like any entry (Task 12 may tune the similarity default)');
assert.equal(E.clipState({}, 0, project).blend, 'normal', 'no blend field is normal');
assert.equal(E.clipState({}, 0, project).mask, null);
assert.equal(E.clipState({mask: 'square'}, 0, project).mask, null, 'a mask that is not an object is no mask');
// animatablePaths: mask.* only while the clip has a mask, between crop.* and effects.*.
const masked = E.animatablePaths(clip);
assert.deepEqual(masked.slice(6, 16), ['crop.l', 'crop.t', 'crop.r', 'crop.b', 'mask.x', 'mask.y', 'mask.w', 'mask.h', 'mask.rotation', 'mask.feather']);
assert(masked.slice(16).every(path => path.startsWith('effects.0.')) && masked.includes('effects.0.similarity') && masked.includes('effects.0.smoothness') && masked.includes('effects.0.spill'), 'spill is keyframable (Task 4 despill gate)');
assert.equal(masked.includes('effects.0.color'), false, 'the key colour carries no keys');
assert(!E.animatablePaths(Object.assign({}, clip, {mask: undefined})).some(path => path.startsWith('mask.')), 'no mask, no mask paths');
assert.deepEqual(E.MASK_PATHS, ['mask.x', 'mask.y', 'mask.w', 'mask.h', 'mask.rotation', 'mask.feather']);
assert.deepEqual(E.keyframable().filter(path => path.startsWith('mask.')), E.MASK_PATHS, 'keyframable lists the mask rows');
const w = E.paramSpec(clip, 'mask.w');
assert.deepEqual([w.min, w.max, w.default, w.unit, w.keyframable], [0.01, 2, 0.5, 'fraction', true]);
assert.equal(E.paramSpec(clip, 'mask.feather').unit, 'height');
assert.equal(E.paramSpec(clip, 'mask.shape').type, 'choice');
assert.equal(E.maskAnimated(clip), true);
assert.equal(E.maskAnimated({keyframes: {x: [{f: 0, v: 0, e: 'linear'}]}}), false);
assert.equal(E.maskAnimated({}), false);
// blendModes: registry order, availability from the server's unavailable list.
const ids = ['normal', 'multiply', 'screen', 'overlay', 'softlight', 'hardlight', 'darken', 'lighten', 'difference', 'exclusion', 'addition', 'subtract', 'dodge', 'burn'];
assert.deepEqual(E.blendModes().map(mode => mode.id), ids);
assert.deepEqual(E.blendModes().map(mode => mode.blend), [''].concat(ids.slice(1)));
assert(E.blendModes().every(mode => mode.available && mode.label_key === 'blend_' + mode.id));
E.adopt(E.BUILTIN, {effects: [], transitions: [], blend_modes: ['overlay', 'dodge'], anim_presets: []});
assert.deepEqual(E.blendModes().filter(mode => !mode.available).map(mode => mode.id), ['overlay', 'dodge']);
assert.equal(E.blendAvailable('overlay'), false);
assert.equal(E.blendAvailable('screen'), true);
assert.equal(E.blendAvailable('normal'), true);
assert.equal(E.blendAvailable('glow'), false, 'unknown modes are not available');
assert.equal(E.clipState(Object.assign({}, clip, {blend: 'overlay'}), 110, project).blend, 'overlay', 'an unavailable mode still previews');
E.adopt(undefined, undefined);
console.log('ok');
`)
}

// compositingShadersPrelude loads the GL shader sources after runVideoStudioEffectsScript's prelude (time, effects and
// picture modules), for the uniform checks of plan 2d Tasks 9-11.
const compositingShadersPrelude = `
require('./js/desktop/apps/video-studio-shaders.js');
const P = window.VideoStudioPicture, S = window.VideoStudioShaders;
const near = (got, want, message) => assert(Math.abs(got - want) < 1e-9, message + ': got ' + got + ', want ' + want);
// Every uniform a program is given is declared in its source.
const declares = (source, uniforms, message) => Object.keys(uniforms).forEach(name => assert(new RegExp('uniform [a-zA-Z0-9]+ ' + name + ';').test(source), message + ' declares ' + name));
`

// TestVideoStudioCompositingUniforms checks the GL side of plan 2d without a browser (index §4.4a, §4.7), one subtest
// per program family:
//   - key (Task 9): passUniforms('chroma_key') and the KEY pass;
//   - mask (Task 10): maskUniforms, boxRect, PLACE_VERTEX's v_box and PLACE_MASKED;
//   - blend (Task 11): one blend(id) source per non-normal registry mode.
func TestVideoStudioCompositingUniforms(t *testing.T) {
	t.Run("key", func(t *testing.T) {
		runVideoStudioEffectsScript(t, compositingShadersPrelude+`
const size = {width: 640, height: 360};
const values = Object.assign(E.defaults('chroma_key'), {color: '#00b140', similarity: 0.3, smoothness: 0.2, spill: 0.7});
const passes = P.passUniforms('chroma_key', values, size);
assert.equal(passes.length, 1, 'one KEY pass');
const u = passes[0];
// #00b140 = (0, 177, 64), as keyColor's 0..1 triple; g >= b despills green.
[0, 177 / 255, 64 / 255].forEach((want, i) => near(u.u_key[i], want, 'u_key[' + i + ']'));
assert.deepEqual(Array.from(u.u_key), Array.from(P.keyColor('#00b140')), 'u_key is keyColor');
assert.deepEqual([u.u_similarity, u.u_smoothness, u.u_spill, u.u_spillBlue], [0.3, 0.2, 0.7, 0]);
// A blue screen (#0047bb = (0, 71, 187)) despills blue.
assert.equal(P.passUniforms('chroma_key', Object.assign({}, values, {color: '#0047bb'}), size)[0].u_spillBlue, 1, 'b > g despills blue');
// Never the identity: the hard step (smoothness 0) with spill 0 still draws, and so do the registry defaults.
assert.equal(P.passUniforms('chroma_key', Object.assign({}, values, {smoothness: 0, spill: 0}), size).length, 1, 'no identity skip');
assert.equal(P.passUniforms('chroma_key', E.defaults('chroma_key'), size).length, 1, 'the registry defaults draw');
// One pass that follows the 2c pass contract and declares every uniform it gets.
const sources = S.effect('chroma_key').passes;
assert.equal(sources.length, 1, 'chroma_key is one pass');
assert(sources[0].startsWith('#version 300 es') && sources[0].includes('uniform sampler2D u_source;') && sources[0].includes('uniform vec2 u_size;'), 'KEY follows the pass contract');
declares(sources[0], u, 'KEY');
`)
	})
	t.Run("mask", func(t *testing.T) {
		runVideoStudioEffectsScript(t, compositingShadersPrelude+`
const box = {width: 640, height: 360};
const mask = {shape: 'ellipse', x: 0.25, y: 0.2, w: 0.5, h: 0.6, rotation: 90, feather: 0.04, invert: true};
const f = P.maskFrame(mask, box), u = P.maskUniforms(mask, box);
assert.deepEqual(u, {u_maskCentre: [f.cx, f.cy], u_maskRadii: [f.rx, f.ry], u_maskRot: [f.cos, f.sin], u_maskRamp: f.ramp, u_maskShape: 1, u_maskInvert: 1}, 'maskFrame as uniforms');
// Law L's numbers: c = ((x + w/2) W, (y + h/2) H), r = (w W / 2, h H / 2), θ = 90° clockwise, F = feather x H.
[[u.u_maskCentre[0], 320], [u.u_maskCentre[1], 180], [u.u_maskRadii[0], 160], [u.u_maskRadii[1], 108], [u.u_maskRot[0], 0], [u.u_maskRot[1], 1], [u.u_maskRamp, 14.4]].forEach(([got, want], i) => near(got, want, 'mask number ' + i));
near(P.maskUniforms(Object.assign({}, mask, {feather: 0}), box).u_maskRamp, 1, 'F is at least 1 box px');
const rect = P.maskUniforms(Object.assign({}, mask, {shape: 'rect', invert: false}), box);
assert.deepEqual([rect.u_maskShape, rect.u_maskInvert], [0, 0], 'rect, not inverted');
// u_boxRect: the drawn quad in box fractions. A square picture in a 640x360 box: contain draws 360x360 at x 140; cover
// draws the box.
const clip = {id: 'c', asset_id: 'a', start: 0, offset: 0, duration: 10, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain'};
const state = E.clipState(clip, 0, box);
assert.deepEqual(P.boxRect(E.placement(state, box, box, {width: 360, height: 360})), [140 / 640, 0, 360 / 640, 1], 'contain: the fitted picture, centred');
assert.deepEqual(P.boxRect(E.placement(Object.assign({}, state, {fit: 'cover'}), box, box, {width: 360, height: 360})), [0, 0, 1, 1], 'cover: the box');
assert.deepEqual(P.boxRect(null), [0, 0, 1, 1], 'no placement: the box');
// The sources: PLACE_VERTEX gives v_box, PLACE reads it for the wipe reveal (2e), PLACE_MASKED inlines MASK and declares
// every uniform.
assert(S.PLACE_VERTEX.includes('uniform vec4 u_boxRect;') && S.PLACE_VERTEX.includes('out vec2 v_box;'), 'PLACE_VERTEX gives v_box');
assert(S.PLACE.includes('in vec2 v_box;') && S.PLACE.includes('uniform vec4 u_reveal;'), 'PLACE reads the box for the wipe reveal (2e)');
assert(S.MASK.includes('float vsMask(vec2 p)'), 'MASK defines vsMask');
assert(S.PLACE_MASKED.startsWith('#version 300 es') && S.PLACE_MASKED.includes(S.MASK) && S.PLACE_MASKED.includes('in vec2 v_box;'), 'PLACE_MASKED inlines MASK');
declares(S.PLACE_MASKED, Object.assign({u_boxSize: [640, 360], u_opacity: 1}, u), 'PLACE_MASKED');
`)
	})
	t.Run("blend", func(t *testing.T) {
		runVideoStudioEffectsScript(t, compositingShadersPrelude+`
const modes = E.blendModes().map(mode => mode.id);
assert.equal(modes[0], 'normal', 'normal leads the menu');
assert.equal(modes.length, 14, 'normal plus 13 modes');
assert.equal(S.blend('normal'), null, 'normal composites inside placeClip');
assert.equal(S.blend('unknown'), null, 'unknown ids have no program');
modes.slice(1).forEach(id => {
  const source = S.blend(id);
  assert(typeof source === 'string' && source.startsWith('#version 300 es'), id + ' has a GLSL ES 3.00 source');
  assert(source.includes('uniform sampler2D u_dst;') && source.includes('uniform sampler2D u_src;'), id + ' reads the composite and the layer');
  assert(source.includes('vec3 vsBlend(vec3 a, vec3 b)') && source.includes('outColor = vec4(mix(below,'), id + ' mixes F with the layer alpha');
});
`)
	})
}

// Edit semantics of the 2d compositing fields (spec §5.1, §5.3, §8.1; index §4.16; plan 2d decision 9):
//   - setBlend, setMask, removeMask and the key colour of setEffectParams, with their refusals;
//   - split, duplicate and copy carry blend, mask and keys;
//   - reverse mirrors the mask keys like every base key;
//   - freeze bakes the mask at the frame and copies the blend;
//   - detached audio drops both;
//   - a rate change rescales the mask keys.
//
// Paste attributes, basePicture (crop, flips, blend, mask):
//   - animated mask numbers are keyed at the playhead;
//   - a source without a mask removes the target's mask and keys;
//   - the keyframes group never pastes mask keys onto a clip without a mask.
//
// The group's label names the blend mode and the mask in every locale.
func TestVideoStudioCompositingEdits(t *testing.T) {
	previous := map[string]string{
		"cs": "Oříznutí a překlopení", "da": "Beskæring og spejling", "de": "Zuschnitt und Spiegeln", "el": "Περικοπή και αναστροφή",
		"en": "Crop and flip", "es": "Recorte y volteo", "fr": "Recadrage et miroir", "hi": "क्रॉप और फ़्लिप",
		"it": "Ritaglio e capovolgimento", "ja": "クロップと反転", "nl": "Bijsnijden en spiegelen", "no": "Beskjæring og speiling",
		"pl": "Kadrowanie i odbicie", "pt": "Corte e espelhamento", "sv": "Beskärning och spegling", "zh": "裁剪与翻转",
	}
	for _, lang := range videoStudioLocales {
		if text := readDesktopLocale(t, lang)["videoStudio.attrBasePicture"]; text == "" || text == previous[lang] {
			t.Errorf("%s: videoStudio.attrBasePicture is %q, want the 2d text that names crop, flip, blend mode and mask", lang, text)
		}
	}
	for lang, want := range map[string]string{"en": "Crop, flip, blend mode and mask", "de": "Zuschnitt, Spiegeln, Mischmodus und Maske"} {
		if got := readDesktopLocale(t, lang)["videoStudio.attrBasePicture"]; got != want {
			t.Errorf("%s: videoStudio.attrBasePicture = %q, want %q", lang, got, want)
		}
	}
	runVideoStudioKeyframesScript(t, `
const editing = require('fs').readFileSync('./js/desktop/apps/video-studio-editing.js', 'utf8');
assert(editing.includes("label('attrBasePicture', 'Crop, flip, blend mode and mask')"), 'the paste attributes dialog names the new basePicture group');
const DEFAULT_MASK = {shape: 'rect', x: 0.25, y: 0.25, w: 0.5, h: 0.5, rotation: 0, feather: 0, invert: false};
const mask = extra => Object.assign({shape: 'ellipse', x: 0.2, y: 0.1, w: 0.5, h: 0.6, rotation: 15, feather: 0.04, invert: false}, extra || {});
const fresh = () => project([
  track('v1','video',[clip('c',V,0,100,{offset:100})]),
  track('o1','overlay',[clip('t',I,0,60)]),
  track('a1','audio',[clip('m',A,0,100)])
]);
let p = fresh();
const c = () => at(p, 'c');

// setMask: a clip without a mask first gets the registry defaults; any subset; numbers clamped to the registry ranges.
assert.equal(K.setMask(p, 'c', {shape: 'ellipse'}, 10), true);
assert.deepEqual(c().mask, Object.assign({}, DEFAULT_MASK, {shape: 'ellipse'}));
assert.equal(K.setMask(p, 'c', {x: -3, w: 5, feather: 0.9, rotation: 400, invert: true}, 10), true);
assert.deepEqual(c().mask, {shape: 'ellipse', x: -1, y: 0.25, w: 2, h: 0.5, rotation: 360, feather: 0.5, invert: true});
valid(p);
const before = JSON.stringify(p);
for (const bad of [{shape: 'star'}, {invert: 'yes'}, {bogus: 1}, {x: 'a'}, {w: null}, {h: true}, {}]) assert.equal(K.setMask(p, 'c', bad, 10), 'nothing', 'refuses ' + JSON.stringify(bad));
assert.equal(JSON.stringify(p), before, 'a refused value changes nothing');
assert.equal(K.setMask(p, 'm', {shape: 'rect'}, 10), 'kind', 'audio clips have no picture');
p.tracks[1].locked = true;
for (const result of [K.setMask(p, 't', {shape: 'rect'}, 10), K.setBlend(p, 't', 'multiply'), K.removeMask(p, 't')]) assert.equal(result, 'locked');
p.tracks[1].locked = false;
assert.equal(K.setMask(p, 't', {w: 0.3}, 10), true, 'stills take masks');
assert.deepEqual(at(p, 't').mask, Object.assign({}, DEFAULT_MASK, {w: 0.3}));
valid(p);

// Animated mask numbers take a key at the playhead and never write their field; shape and invert stay static.
p = fresh();
assert.equal(K.setMask(p, 'c', {x: 0.1}, 10), true);
assert(FX.animatablePaths(c()).includes('mask.x'));
assert.equal(K.setAnimated(p, 'c', 'mask.x', true, 10), true);
assert.deepEqual(c().keyframes, {'mask.x': [kf(110, 0.1)]});
assert.equal(K.setMask(p, 'c', {x: 0.4, y: 0.3, shape: 'ellipse'}, 30), true);
assert.deepEqual(c().keyframes, {'mask.x': [kf(110, 0.1), kf(130, 0.4)]});
assert.deepEqual([c().mask.x, c().mask.y, c().mask.shape], [0.1, 0.3, 'ellipse'], 'the animated x keeps its field; y and shape change theirs');
near(FX.valueAt(c(), 'mask.x', 20, p), 0.25, 'the keys drive the value');
assert.equal(K.setMask(p, 'c', {x: 0.5}, 150), 'nothing', 'no key outside the clip');
for (const path of ['mask.shape', 'mask.invert']) assert.equal(K.setAnimated(p, 'c', path, true, 10), 'nothing', path + ' is not keyframable');
valid(p);
assert.equal(K.setAnimated(p, 'c', 'mask.x', false, 20), true);
assert.equal('keyframes' in c(), false);
near(c().mask.x, 0.25, 'stopping the animation writes the value shown into the field');
valid(p);

// removeMask: the mask and its keys go in one step; other keys stay.
p = fresh();
assert.equal(K.setMask(p, 'c', {shape: 'ellipse'}, 10), true);
for (const path of ['mask.w', 'mask.feather', 'x']) assert.equal(K.setAnimated(p, 'c', path, true, 10), true);
valid(p);
assert.equal(K.removeMask(p, 'c'), true);
assert.equal('mask' in c(), false);
assert.deepEqual(c().keyframes, {x: [kf(110, 0)]});
assert.equal(FX.animatablePaths(c()).some(path => path.startsWith('mask.')), false);
assert.equal(K.removeMask(p, 'c'), 'nothing');
valid(p);

// setBlend: registry modes only; normal deletes the field (canonical form).
p = fresh();
assert.equal(K.setBlend(p, 'c', 'multiply'), true);
assert.equal(c().blend, 'multiply');
assert.equal(K.setBlend(p, 'c', 'multiply'), 'nothing');
assert.equal(K.setBlend(p, 'c', 'sparkle'), 'nothing');
valid(p);
assert.equal(K.setBlend(p, 'c', 'normal'), true);
assert.equal('blend' in c(), false, 'normal is stored as no field');
assert.equal(K.setBlend(p, 'c', 'normal'), 'nothing');
assert.equal(K.setBlend(p, 'm', 'screen'), 'kind');
valid(p);

// The key colour: setEffectParams accepts #rrggbb and stores it lowercased; other values refuse.
p = fresh();
assert.equal(K.addEffect(p, 'c', 'chroma_key'), true);
assert.equal(c().effects[0].params.color, FX.defaults('chroma_key').color);
assert.equal(K.setEffectParams(p, 'c', 0, {color: '#0047BB'}, 10), true);
assert.equal(c().effects[0].params.color, '#0047bb');
for (const bad of ['green', '#00b14', '#00b1400', '00b140', 4567, null]) assert.equal(K.setEffectParams(p, 'c', 0, {color: bad}, 10), 'nothing', 'refuses ' + bad);
assert.equal(K.setEffectParams(p, 'c', 0, {similarity: 0.4, color: '#00FF00'}, 10), true, 'a colour and a number in one call');
assert.deepEqual([c().effects[0].params.similarity, c().effects[0].params.color], [0.4, '#00ff00']);
valid(p);

// Split, duplicate and copy carry the blend mode, the mask and its keys; the values stay continuous across the cut.
const masked = () => ({blend: 'screen', mask: mask(), keyframes: {'mask.x': [kf(110, 0.1, 'ease_in'), kf(150, 0.5)]}});
const original = () => project([track('v1','video',[clip('c',V,0,100,Object.assign({offset:100}, masked()))]), track('a1','audio',[])]);
const maskX = (q, id, frame) => FX.valueAt(at(q, id), 'mask.x', frame, q);
p = original();
valid(p);
assert.equal(E.blade(p, 40, {clipIds: ['c'], linked: false}, {}), true);
const right = p.tracks[0].clips.find(item => item.id !== 'c');
for (const part of [at(p,'c'), right]) assert.deepEqual([part.blend, part.mask, part.keyframes], ['screen', mask(), masked().keyframes]);
for (const frame of [39, 40, 41, 70]) near(maskX(p, frame < 40 ? 'c' : right.id, frame), maskX(original(), 'c', frame), 'split @' + frame);
const out = {};
assert.equal(E.duplicateClips(p, [right.id], out), true);
assert.deepEqual([at(p, out.created[0]).blend, at(p, out.created[0]).mask], ['screen', mask()]);
const board = E.copyClips(p, ['c']);
assert.deepEqual([board.entries[0].clip.blend, board.entries[0].clip.mask, board.entries[0].clip.keyframes], ['screen', mask(), masked().keyframes]);
valid(p);

// Reverse: mask keys mirror around the reversed asset's last frame (pivot 199) like every base key; the mask is not
// mirrored in space.
p = original();
p.assets.push({id:'r', kind:'video', has_audio:true, duration_frames:100});
assert.equal(E.reverseClips(p, 'c', {assetId:'r', rangeStart:100, frames:100, sourceAssetId:'v'}), true);
assert.deepEqual(at(p,'c').keyframes, {'mask.x': [kf(49, 0.5, 'ease_out'), kf(89, 0.1)]});
assert.deepEqual([at(p,'c').blend, at(p,'c').mask], ['screen', mask()]);
for (let frame = 0; frame < 100; frame++) near(maskX(p, 'c', 99 - frame), maskX(original(), 'c', frame), 'mirror @' + frame);
valid(p);

// Freeze frame: the still takes the blend mode and the mask with its numbers at the frozen frame, without keys.
p = project([track('v1','video',[clip('c',V,0,100,{offset:100, blend: 'overlay', mask: mask({invert: true}), keyframes: {'mask.w': [kf(100, 0.2), kf(200, 0.8)], 'mask.rotation': [kf(100, 0), kf(200, 90)]}})])]);
valid(p);
assert.equal(E.freezeFrame(p, 'c', 50, {asset_id: 'i', clip_id: 'fz', duration: 30}), true);
const still = at(p, 'fz');
assert.equal('keyframes' in still, false);
assert.equal(still.blend, 'overlay');
near(still.mask.w, 0.5, 'the width shown at source 150');
near(still.mask.rotation, 45, 'the rotation shown at source 150');
assert.deepEqual(Object.assign({}, still.mask, {w: 0, rotation: 0}), mask({invert: true, w: 0, rotation: 0}), 'shape, invert and the static numbers are copied');
valid(p);

// Detach audio: the audio copy has no blend mode and no mask; the video clip keeps both.
p = original();
assert.equal(E.detachAudio(p, 'c', {clip: 'au', link: 'l1', track: 'a2', trackName: 'Audio 2'}), true);
for (const key of ['blend', 'mask', 'keyframes']) assert.equal(key in at(p,'au'), false, 'the audio copy has no ' + key);
assert.deepEqual([at(p,'c').blend, at(p,'c').mask], ['screen', mask()]);
valid(p);

// Rate change: mask key frames scale like the offset (rounded; a key landing on the one before it is dropped).
p = project([track('v1','video',[clip('c',V,0,100,{offset:10, mask: mask(), keyframes: {'mask.x': [kf(10, 0.1), kf(11, 0.2), kf(12, 0.3), kf(40, 0.4)]}})])]);
p.assets = p.assets.map(asset => Object.assign({}, asset));
p.fps = 60;
window.VideoStudioTime.rescaleProject(p, 24, false);
assert.deepEqual(at(p,'c').keyframes, {'mask.x': [kf(4, 0.1), kf(5, 0.3), kf(16, 0.4)]});
assert.deepEqual(at(p,'c').mask, mask(), 'mask numbers are fractions of the box, not time');
valid(p);

// Paste attributes, basePicture: crop, flips, blend mode and mask fields; keys belong to the keyframes group.
assert.deepEqual(E.ATTRIBUTE_GROUPS.basePicture, ['crop', 'flip_h', 'flip_v', 'blend', 'mask']);
const source = () => at(original(), 'c');
p = project([track('v1','video',[clip('d',V,200,100,{offset:300})]), track('a1','audio',[clip('m',A,0,100)])]);
assert.equal(E.pasteAttributes(p, source(), ['d', 'm'], ['basePicture'], 250), true);
assert.deepEqual([at(p,'d').blend, at(p,'d').mask, 'keyframes' in at(p,'d')], ['screen', mask(), false]);
for (const key of ['blend', 'mask']) assert.equal(key in at(p,'m'), false, 'audio clips take no picture attributes');
valid(p);
// Mask numbers the target animates take the source's fields as keys at the playhead (source 350); the others take the
// fields, and shape and invert are copied.
p = project([track('v1','video',[clip('d',V,200,100,{offset:300, mask: mask({shape: 'rect', x: 0.6, y: 0.5, w: 0.3, h: 0.3, rotation: -20, feather: 0.2, invert: true}),
  keyframes: {'mask.x': [kf(300, 0), kf(400, 0.4)], 'mask.feather': [kf(300, 0.1)]}})])]);
valid(p);
assert.equal(E.pasteAttributes(p, source(), ['d'], ['basePicture'], 250), true);
assert.deepEqual(at(p,'d').keyframes, {'mask.feather': [kf(300, 0.1), kf(350, 0.04)], 'mask.x': [kf(300, 0), kf(350, 0.2), kf(400, 0.4)]});
assert.deepEqual(at(p,'d').mask, mask({x: 0.6, feather: 0.2}), 'the animated x and feather keep their fields');
assert.equal(at(p,'d').blend, 'screen');
valid(p);
assert.equal(E.pasteAttributes(p, source(), ['d'], ['basePicture'], 50), true);
assert.deepEqual(at(p,'d').keyframes['mask.x'], [kf(300, 0), kf(350, 0.2), kf(400, 0.4)], 'outside the clip an animated number is left as it is');
// A source without a mask removes the target's mask, its keys and the blend mode.
const plain = at(project([track('v1','video',[clip('s',V,0,100)])]), 's');
assert.equal(E.pasteAttributes(p, plain, ['d'], ['basePicture'], 250), true);
for (const key of ['blend', 'mask', 'keyframes']) assert.equal(key in at(p,'d'), false, 'pasting a plain clip clears ' + key);
valid(p);
// With the keyframes group the source's mask is copied as it is and its keys re-anchored (d: offset 300, speed 2).
p = project([track('v1','video',[clip('d',V,200,100,{offset:300, speed:2})])]);
assert.equal(E.pasteAttributes(p, source(), ['d'], ['basePicture', 'keyframes'], 250), true);
assert.deepEqual([at(p,'d').mask, at(p,'d').keyframes], [mask(), {'mask.x': [kf(320, 0.1, 'ease_in'), kf(400, 0.5)]}]);
valid(p);
// The keyframes group alone, onto a clip without a mask, leaves the mask keys out (mask keys need a mask).
p = project([track('v1','video',[clip('d',V,200,100,{offset:300})])]);
const keyedSource = Object.assign(source(), {keyframes: {'mask.x': [kf(110, 0.1)], x: [kf(110, 0.2)]}});
assert.equal(E.pasteAttributes(p, keyedSource, ['d'], ['keyframes'], 250), true);
assert.deepEqual(at(p,'d').keyframes, {x: [kf(310, 0.2)]});
assert.equal('mask' in at(p,'d'), false);
valid(p);
`)
}

// The compositing controls of the inspector (Task 14):
//   - Picture: the blend list in registry order with the clip's mode selected; unavailable modes are disabled with a
//     title, but the current one stays selectable; the transition hint appears when a transition joins two modes. The
//     mask block per state: no mask; a mask with values at the playhead in percent, keyframe tools, animated fields
//     frozen outside the clip, and the mask-mode button. Titles take blend and mask but no crop.
//   - A chroma_key entry: the colour input and the presets; addEntry puts chroma_key at stack index 0.
//   - Every commit is one undo step: blend, mask shape, field, invert, removal, key colour, preset and "+ Effect". An
//     animated mask path gets a key at the playhead; audio clips and locked tracks change nothing.
func TestVideoStudioCompositingInspector(t *testing.T) {
	runVideoStudioKeyframesScript(t, videoStudioEffectStackPartsPrelude+`
P.renderUI = () => {}; P.renderToolbar = () => {}; P.resyncArtwork = () => {}; P.renderInspector = () => {};
const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => '&#' + ch.charCodeAt(0) + ';');
const between = (text, start, end) => { const from = text.indexOf(start); assert(from >= 0, 'missing ' + start); return text.slice(from, text.indexOf(end, from)); };
const MASK = {shape: 'ellipse', x: 0.1, y: 0.2, w: 0.5, h: 0.4, rotation: 15, feather: 0.04, invert: false};
const p = project([
  track('v1','video',[clip('c',V,0,100,{offset:100, blend: 'multiply', transition: {type: 'dissolve', duration: 10}}), clip('d',V,90,100,{offset:100})]),
  track('o1','overlay',[clip('t',I,0,60,{mask: JSON.parse(JSON.stringify(MASK)), keyframes: {'mask.x': [kf(0, 0.1), kf(40, 0.3)]}})]),
  track('a1','audio',[clip('m',A,0,100)])
]);
valid(p);
const render = (clipId, frame, extra) => {
  const host = {innerHTML: ''}, selected = T.clipFor(p, clipId);
  IN.render(host, Object.assign({selected, asset: selected.track.kind === 'overlay' ? I : V, project: p, locale: 'en', caps: {}, transitionInfo: {next: null}, esc,
    icon: name => '<i data-icon="' + name + '"></i>', tr: key => '[' + key + ']', keyframes: P.keyframeView({project: p, selectedClipId: clipId, frame, readonly: false}), fineOpen: false, effects: null}, extra || {}));
  return host.innerHTML;
};

// The blend list follows the registry with the clip's mode selected. A transition into a clip of another mode shows
// the hint on both clips of the pair.
let html = render('c', 20);
const list = between(html, '<select data-blend>', '</select>');
assert.deepEqual(Array.from(list.matchAll(/<option value="([a-z]+)"/g), m => m[1]), FX.blendModes().map(mode => mode.id));
assert.equal(FX.blendModes().length, 14);
assert(list.includes('<option value="multiply" selected>[blend_multiply]</option>'), 'the clip mode is selected');
assert(html.includes('data-blend-hint') && html.includes('[blendTransitionHint]'), 'multiply into normal explains the overlap');
assert(render('d', 95).includes('data-blend-hint'), 'the incoming clip shows the hint too');
at(p, 'd').blend = 'multiply';
assert(!render('c', 20).includes('data-blend-hint'), 'a shared mode blends the overlap: no hint');
delete at(p, 'd').blend;
assert(!render('t', 20).includes('data-blend-hint'), 'no transition, no hint');
// Unavailable modes are disabled with a title; the clip's own mode stays selectable.
FX.adopt(FX.BUILTIN, {effects: [], transitions: [], blend_modes: ['multiply', 'screen'], anim_presets: []});
html = render('c', 20);
assert(html.includes('<option value="multiply" selected title="[blendUnavailable]">'), 'the current unavailable mode stays enabled');
assert(html.includes('<option value="screen" disabled title="[blendUnavailable]">'), 'another unavailable mode is disabled');
assert(html.includes('<option value="overlay">'), 'available modes carry no title');
FX.adopt(null, null);

// No mask: None is pressed, there are no fields, and Edit mask in the preview is disabled.
html = render('c', 20);
assert(html.includes('<button type="button" data-mask-set="none" aria-pressed="true">[mask_none]</button>'));
assert(!html.includes('data-mask-field') && !html.includes('data-mask-invert'));
assert(/<button[^>]*data-action="mask-mode"[^>]*disabled/.test(html), 'no mask: no mask mode');
// A mask shows its values at the playhead (mask.x is keyed 0.1 at 0 and 0.3 at 40, so 0.2 at frame 20), with keyframe
// tools, Invert and mask mode.
html = render('t', 20);
assert(html.includes('data-mask-set="ellipse" aria-pressed="true"'));
for (const [field, value] of [['x', 20], ['y', 20], ['w', 50], ['h', 40], ['rotation', 15], ['feather', 4]]) {
  assert(new RegExp('<input type="number" data-mask-field="' + field + '"[^>]*value="' + value + '">').test(html), field + ' shows ' + value + ': ' + html.match(new RegExp('<input type="number" data-mask-field="' + field + '"[^>]*>')));
}
assert(html.includes('data-kf-path="mask.x"') && html.includes('data-kf-path="mask.feather"'), 'mask values carry keyframe tools');
assert(html.includes('data-mask-invert aria-pressed="false"'));
assert(/<button type="button" class="vs-chip-button vs-mask-mode" data-action="mask-mode" aria-pressed="false" title="\[maskMode\]">/.test(html), 'mask mode is available inside the clip');
assert(render('t', 20, {maskMode: true}).includes('data-action="mask-mode" aria-pressed="true"'));
// Outside the clip (t ends at frame 60) the animated mask.x freezes at its last key, and static values stay editable.
// Mask mode needs the playhead inside.
html = render('t', 80);
assert(/<input type="number" data-mask-field="x"[^>]*value="30" disabled title="\[keyframeOutside\]">/.test(html), 'an animated mask value freezes outside the clip');
assert(/<input type="number" data-mask-field="y"[^>]*value="20">/.test(html), 'a static mask value stays editable');
assert(/<button[^>]*data-action="mask-mode"[^>]*title="\[maskModeOutside\]" disabled>/.test(html));
// A title takes blend and mask, but no crop.
const title = Object.assign(JSON.parse(JSON.stringify(at(p, 't'))), {text: 'Hello'});
html = render('t', 20, {selected: {clip: title, track: p.tracks[1]}, keyframes: null});
assert(html.includes('<select data-blend>') && html.includes('data-mask-set="ellipse" aria-pressed="true"') && !html.includes('data-crop='), 'titles take blend and mask, not crop');
assert(/<input type="number" data-mask-field="x"[^>]*value="10">/.test(html), 'without a keyframe view the stored values show');

// addEntry puts chroma_key at stack index 0 (a second key too) and every other effect last.
assert.equal(FXI.addEntry(p, 'c', 'color'), true);
assert.equal(FXI.addEntry(p, 'c', 'chroma_key'), true);
assert.equal(FXI.addEntry(p, 'c', 'blur'), true);
const firstKey = at(p, 'c').effects[0];
assert.equal(FXI.addEntry(p, 'c', 'chroma_key'), true);
assert.deepEqual(at(p, 'c').effects.map(entry => entry.id), ['chroma_key', 'chroma_key', 'color', 'blur']);
assert.equal(at(p, 'c').effects[1], firstKey, 'the older key moves down by one');
assert.equal(at(p, 'c').effects[0].params.color, '#00b140');
valid(p);
html = render('c', 20);
const key = between(html, 'data-fx-entry="0"', 'data-fx-entry="1"');
assert(key.includes('<input type="color" data-fx-param="color" value="#00b140">'), 'the key colour is a colour input');
assert(key.includes('data-fx-key-preset="green" aria-pressed="true"') && key.includes('data-fx-key-preset="blue" aria-pressed="false"'));
assert(key.includes('[keyPresetGreen]') && key.includes('[keyPresetBlue]'));
assert(key.includes('data-kf-path="effects.0.similarity"') && key.includes('data-kf-path="effects.0.smoothness"'), 'similarity and smoothness have keyframe tools');
assert(!key.includes('data-kf-path="effects.0.color"'), 'the key colour has none');

// Commits: one undo step each, through VideoStudioKeyframeEdit.
const q = project([track('v1','video',[clip('c',V,0,100,{offset:100})]), track('a1','audio',[clip('m',A,0,100)])]);
const s = {project: q, projectId: '', readonly: false, conflict: true, history: [], redo: [], frame: 20, dirty: false, revision: 0, selectedClipId: 'c', selectionRevision: 0,
  selectionSet: new Set(['c']), selectionPrimary: 'c', ctx: {}, q: () => null, preview: {seek() {}}};
const c = () => at(s.project, 'c'), steps = () => s.history.length;
// A stand-in element. matches holds the selectors it matches. closest finds the element itself for a selector that
// names one of its attributes, and the entry it sits in for [data-fx-entry].
const fake = (attrs, entry) => { const el = {dataset: attrs.dataset || {}, value: attrs.value, disabled: false,
  getAttribute: name => (attrs.attributes && name in attrs.attributes ? String(attrs.attributes[name]) : null),
  matches: selector => (attrs.matches || []).includes(selector),
  closest: selector => (selector === '[data-fx-entry]' ? (entry == null ? null : {dataset: {fxEntry: String(entry)}}) : (attrs.closest || []).some(name => selector.includes(name)) ? el : null)}; return el; };
const click = (name, dataset, attributes, entry) => P.effectCommand(s, {target: fake({closest: [name], dataset, attributes}, entry)});
const change = (selector, dataset, value, entry) => P.compositingChange(s, fake({matches: [selector], dataset, value}, entry));
const field = (name, value) => change('input[data-mask-field]', {maskField: name}, value);

assert.equal(change('select[data-blend]', {}, 'screen'), true);
assert.equal(c().blend, 'screen'); assert.equal(steps(), 1);
change('select[data-blend]', {}, 'normal');
assert.equal('blend' in c(), false, 'Normal deletes the field'); assert.equal(steps(), 2);
assert.equal(click('data-mask-set', {maskSet: 'rect'}, {'aria-pressed': 'false'}), true);
assert.deepEqual(c().mask, {shape: 'rect', x: 0.25, y: 0.25, w: 0.5, h: 0.5, rotation: 0, feather: 0, invert: false}, 'Rectangle adds the registry default mask');
assert.equal(steps(), 3);
click('data-mask-set', {maskSet: 'rect'}, {'aria-pressed': 'true'});
assert.equal(steps(), 3, 'the pressed shape changes nothing');
click('data-mask-set', {maskSet: 'ellipse'}, {'aria-pressed': 'false'});
assert.equal(c().mask.shape, 'ellipse'); assert.equal(c().mask.x, 0.25, 'a new shape keeps the values'); assert.equal(steps(), 4);
field('x', '10'); near(c().mask.x, 0.1, 'x in percent of the box'); assert.equal(steps(), 5);
field('rotation', '500'); field('feather', '80'); field('w', '0');
assert.deepEqual([c().mask.rotation, c().mask.feather, c().mask.w], [360, 0.5, 0.01], 'typed values clamp to the registry ranges');
assert.equal(steps(), 8);
assert.equal(K.setAnimated(s.project, 'c', 'mask.y', true, 20), true);
s.frame = 50;
field('y', '40');
assert.deepEqual(c().keyframes['mask.y'].map(item => [item.f, item.v]), [[120, 0.25], [150, 0.4]], 'an animated mask value keys the playhead');
assert.equal(steps(), 9);
click('data-mask-invert', {}, {'aria-pressed': 'false'});
assert.equal(c().mask.invert, true); assert.equal(steps(), 10);
click('data-mask-set', {maskSet: 'none'}, {'aria-pressed': 'false'});
assert.equal('mask' in c(), false, 'None removes the mask');
assert.equal((c().keyframes || {})['mask.y'], undefined, 'with its keys');
assert.equal(steps(), 11);
assert.equal(FXI.addEntry(s.project, 'c', 'color'), true);
assert.equal(P.addEffect(s, 'c', 'chroma_key'), true, '"+ Effect" adds through addEntry');
assert.deepEqual(c().effects.map(entry => entry.id), ['chroma_key', 'color']); assert.equal(steps(), 12);
change('input[type="color"][data-fx-param]', {fxParam: 'color'}, '#0047BB', 0);
assert.equal(c().effects[0].params.color, '#0047bb', 'the colour input lowercases'); assert.equal(steps(), 13);
click('data-fx-key-preset', {fxKeyPreset: 'green'}, {}, 0);
assert.equal(c().effects[0].params.color, '#00b140'); assert.equal(steps(), 14);
// Refusals change nothing: an audio clip has no blend mode, a locked track takes no edit.
s.selectedClipId = 'm';
change('select[data-blend]', {}, 'screen');
assert.equal(steps(), 14, 'an audio clip takes no blend mode');
s.selectedClipId = 'c'; s.project.tracks[0].locked = true;
change('select[data-blend]', {}, 'screen'); field('x', '30');
assert.equal(steps(), 14, 'a locked track changes nothing'); assert.equal('blend' in c(), false);
`)
}

// Every desktop locale carries the compositing inspector strings (Task 14), and the sentences are translated.
// videoStudio.attrBasePicture got its 2d text in Task 13 (TestVideoStudioCompositingEdits pins it).
func TestVideoStudioCompositingInspectorLocales(t *testing.T) {
	keys := []string{"mask", "mask_none", "mask_rect", "mask_ellipse", "maskFieldsHint", "maskMode", "maskModeOutside", "blendUnavailable", "blendTransitionHint", "keyPresetGreen", "keyPresetBlue"}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		values := readDesktopLocale(t, lang)
		for _, key := range keys {
			if strings.TrimSpace(values["videoStudio."+key]) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			}
		}
		if lang == "en" {
			continue
		}
		for _, key := range []string{"maskFieldsHint", "maskModeOutside", "blendUnavailable", "blendTransitionHint"} {
			if values["videoStudio."+key] == english["videoStudio."+key] {
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
	}
}
