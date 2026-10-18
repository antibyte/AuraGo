package ui

import "testing"

// TestVideoStudioShadersCoverTransitions checks the GL transition sources without a browser (plan 2e Task 11; index
// §4.7, §4.24):
//   - every registry transition has a GLSL ES 3.00 source that mixes u_from and u_to; ids outside the registry give null;
//   - every uniform a source declares and reads comes from VideoStudioPicture.transitionUniforms with the declared type
//     (u_from, u_to and u_size come from drawTransition), at several frames and sizes;
//   - the 20 new sources follow the D1 rules: the pixel from vsPixel (rows from the top), texelFetch reads, and u_P
//     instead of u_progress.
func TestVideoStudioShadersCoverTransitions(t *testing.T) {
	runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-shaders.js');
const P = window.VideoStudioPicture, S = window.VideoStudioShaders;
const ids = E.transitions().map(spec => spec.id);
assert.equal(ids.length, 24, 'the registry lists 24 transitions');
const OLD = ['dissolve', 'black', 'wipeleft', 'wiperight'];
const drawn = ['u_from', 'u_to', 'u_size'];
const word = name => new RegExp('\\b' + name + '\\b', 'g');
const list = value => (Array.isArray(value) || ArrayBuffer.isView(value) ? Array.from(value) : null);
const fits = (type, value) => {
  if (type === 'float') return typeof value === 'number' && Number.isFinite(value);
  if (type === 'int') return Number.isInteger(value);
  const items = /^vec[234]$/.test(type) ? list(value) : null;
  return !!items && items.length === Number(type.slice(3)) && items.every(Number.isFinite);
};
const fresh = new Set();
ids.forEach(id => {
  const source = S.transition(id);
  assert(typeof source === 'string' && source.startsWith('#version 300 es'), id + ' has a GLSL ES 3.00 source');
  assert(source.includes('uniform sampler2D u_from;') && source.includes('uniform sampler2D u_to;'), id + ' mixes the two track layers');
  const declared = Array.from(source.matchAll(/uniform (\w+) (u_\w+);/g), m => ({type: m[1], name: m[2]}));
  const read = declared.filter(u => !drawn.includes(u.name) && source.match(word(u.name)).length > 1);
  [[0, 20], [5, 20], [19, 20], [3, 7], [6, 7]].forEach(([i, D]) => [{width: 96, height: 54}, {width: 75, height: 43}].forEach(size => {
    const uniforms = P.transitionUniforms(id, i, D, size), label = id + ' at i ' + i + ' of ' + D + ', ' + size.width + 'x' + size.height;
    read.forEach(u => assert(fits(u.type, uniforms[u.name]), label + ': transitionUniforms gives ' + JSON.stringify(uniforms[u.name]) + ' for ' + u.type + ' ' + u.name));
  }));
  if (OLD.includes(id)) return;
  fresh.add(source);
  assert(source.includes('ivec2 p = vsPixel();') && source.includes('texelFetch(u_from') && source.includes('texelFetch(u_to'), id + ' reads texels by index, rows from the top');
  assert.equal(source.match(word('u_progress')).length, 1, id + ' declares u_progress (TRANSITION_HEAD) but never reads it (D1: u_P)');
});
assert.equal(fresh.size, 20, 'the 20 new transitions have 20 distinct sources');
['not-a-transition', 'toString', '__proto__', '', undefined].forEach(id => assert.equal(S.transition(id), null, String(id) + ' has no source'));
console.log('ok');
`)
}
