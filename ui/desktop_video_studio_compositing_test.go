package ui

import "testing"

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
//   - mask (Task 10): maskUniforms, boxRect, PLACE_VERTEX's v_box and PLACE_MASKED.
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
// The sources: PLACE_VERTEX gives v_box, PLACE ignores it, PLACE_MASKED inlines MASK and declares every uniform.
assert(S.PLACE_VERTEX.includes('uniform vec4 u_boxRect;') && S.PLACE_VERTEX.includes('out vec2 v_box;'), 'PLACE_VERTEX gives v_box');
assert(!S.PLACE.includes('v_box'), 'PLACE ignores the box');
assert(S.MASK.includes('float vsMask(vec2 p)'), 'MASK defines vsMask');
assert(S.PLACE_MASKED.startsWith('#version 300 es') && S.PLACE_MASKED.includes(S.MASK) && S.PLACE_MASKED.includes('in vec2 v_box;'), 'PLACE_MASKED inlines MASK');
declares(S.PLACE_MASKED, Object.assign({u_boxSize: [640, 360], u_opacity: 1}, u), 'PLACE_MASKED');
`)
	})
}
