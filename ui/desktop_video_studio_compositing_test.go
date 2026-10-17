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
