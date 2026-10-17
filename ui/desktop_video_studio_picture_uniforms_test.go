package ui

import "testing"

// TestVideoStudioPictureUniforms checks the GL side of the 2c effect passes without a browser (plan 2c, Task 10;
// index §4.7, §4.8). VideoStudioPicture.passUniforms per effect id: identity entries draw nothing, blur has two
// directions, a sigma and a kernel radius, and sharpen gets the CAS peak constants. It also checks capSize and
// decodedSize, and that every uniform passUniforms returns is declared by the matching VideoStudioShaders.effect pass.
func TestVideoStudioPictureUniforms(t *testing.T) {
	runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-shaders.js');
const P = window.VideoStudioPicture, S = window.VideoStudioShaders;
const near = (got, want, message) => assert(Math.abs(got - want) < 1e-9, message + ': got ' + got + ', want ' + want);
const size = {width: 640, height: 360};
// Colour: one pass with colorCoefficients' numbers; the registry defaults are the identity and draw nothing.
const values = Object.assign(E.defaults('color'), {exposure: 0.5, contrast: 1.2, saturation: 0.8, temperature: 5000, tint: 0.2, lift: 0.05, lift_x: 0.3, lift_y: -0.2, gamma: 1.2, gain: 0.9, gain_y: 0.4});
const c = P.colorCoefficients(values);
assert.deepEqual(P.passUniforms('color', values, size), [{u_mul: Array.from(c.mul), u_contrast: c.contrast, u_lift: Array.from(c.lift), u_gain: Array.from(c.gain), u_invGamma: Array.from(c.invGamma), u_saturation: c.saturation}]);
assert.deepEqual(P.passUniforms('color', E.defaults('color'), size), [], 'colour at its defaults is the identity');
// LUT: the loaded table's grid and domain (the compositor passes it as values.table); nothing without one or at 0.
const table = {size: 17, domainMin: [0, 0, 0], domainMax: [1, 1, 1], data: new Float32Array(17 * 17 * 17 * 3)};
assert.deepEqual(P.passUniforms('lut', {lut: 'builtin:warm', intensity: 0.6, table}, size), [{u_lutSize: 17, u_domainMin: [0, 0, 0], u_domainMax: [1, 1, 1], u_intensity: 0.6}]);
assert.deepEqual(P.passUniforms('lut', {lut: 'builtin:warm', intensity: 0, table}, size), [], 'intensity 0 is the identity');
assert.deepEqual(P.passUniforms('lut', {lut: '', intensity: 1}, size), [], 'no table (none chosen, loading or failed) passes through');
// Blur: sigma = radius x target height, horizontal then vertical. The kernel half-width is min(128, ceil(3 sigma)),
// read in bilinear pairs, so one pass takes 1 + 2 ceil(radius / 2) samples.
const taps = radius => 1 + 2 * Math.ceil(radius / 2);
const blur = P.passUniforms('blur', {radius: 0.01}, size);
assert.equal(blur.length, 2, 'two passes');
near(blur[0].u_sigma, 3.6, 'sigma'); near(blur[1].u_sigma, 3.6, 'the same sigma both ways');
assert.deepEqual([blur[0].u_direction, blur[1].u_direction], [[1 / 640, 0], [0, 1 / 360]], 'texel steps');
assert.deepEqual([blur[0].u_radius, blur[1].u_radius, taps(blur[0].u_radius)], [11, 11, 13], 'ceil(3 x 3.6) = 11 texels in 13 samples');
const wide = P.passUniforms('blur', {radius: 0.05}, {width: 3840, height: 2160});
near(wide[0].u_sigma, 108, 'sigma on a 4K target');
assert.deepEqual([wide[0].u_radius, taps(wide[0].u_radius)], [128, 129], 'the kernel stops at 128 texels');
assert.deepEqual(P.passUniforms('blur', {radius: 0}, size), [], 'radius 0 is the identity');
// Sharpen: CAS draws even at strength 0, and the peak constants come from one place.
assert(Number.isFinite(P.CAS_P0) && Number.isFinite(P.CAS_P1) && P.CAS_P1 > 4, 'CAS constants (CAS_P1 > 4 keeps 1 + 4w positive)');
assert.deepEqual(P.passUniforms('sharpen', {strength: 0}, size), [{u_strength: 0, u_peak: [P.CAS_P0, P.CAS_P1]}], 'strength 0 still sharpens');
assert.deepEqual(P.passUniforms('sharpen', {strength: 0.7}, size), [{u_strength: 0.7, u_peak: [P.CAS_P0, P.CAS_P1]}]);
// Vignette: vignetteAngle at the target size, about 0.801 rad for amount 0.5, size 1 on 16:9.
const vignette = P.passUniforms('vignette', {amount: 0.5, size: 1}, size);
near(vignette[0].u_angle, P.vignetteAngle(0.5, 1, 640, 360), 'angle');
assert(Math.abs(vignette[0].u_angle - 0.801) < 1e-3, '0.5 x pi/2 x hypot(320, 180) / 360');
assert.deepEqual(P.passUniforms('vignette', {amount: 0, size: 1}, size), [], 'amount 0 is the identity');
assert.deepEqual(P.passUniforms('chroma_key', {}, size), [], 'ids without a 2c pass draw nothing');
// Every pass gets its uniform set, and each shader declares the uniforms it is given.
const samples = {color: values, lut: {lut: 'builtin:warm', intensity: 0.5, table}, blur: {radius: 0.01}, sharpen: {strength: 0.5}, vignette: {amount: 0.5, size: 1}};
Object.keys(samples).forEach(id => {
  const passes = S.effect(id).passes, uniforms = P.passUniforms(id, samples[id], size);
  assert.equal(passes.length, uniforms.length, id + ': one uniform set per pass');
  passes.forEach((source, index) => {
    assert(source.startsWith('#version 300 es'), id + ' is GLSL ES 3.00');
    assert(source.includes('uniform sampler2D u_source;') && source.includes('uniform vec2 u_size;'), id + ' follows the pass contract');
    Object.keys(uniforms[index]).forEach(name => assert(new RegExp('uniform [a-zA-Z0-9]+ ' + name + ';').test(source), id + ' declares ' + name));
  });
});
assert(S.effect('lut').passes[0].includes('precision highp sampler3D;'), 'the LUT pass declares its sampler3D precision');
assert.deepEqual(Object.values(P.LUMA).map(Number), [0.2126, 0.7152, 0.0722], 'BT.709 luma');
assert(S.effect('color').passes[0].includes('vec3(0.2126, 0.7152, 0.0722)'), 'saturation uses the shared luma');
assert.equal(S.effect('unknown'), null);
assert(S.PICTURE.includes('uniform vec4 u_uvRect;') && S.PICTURE.includes('uniform vec2 u_flip;'), 'PICTURE takes the crop rect and the flips');
// capSize keeps the cropped texture's aspect. It is never more than a full-canvas cover needs and never upscaled.
const canvas = {width: 1280, height: 720};
assert.deepEqual(P.capSize(3840, 2160, canvas), {width: 1280, height: 720}, 'a 4K source drops to the canvas');
assert.deepEqual(P.capSize(720, 1280, canvas), {width: 720, height: 1280}, 'a portrait keeps what a full-canvas cover shows');
assert.deepEqual(P.capSize(1080, 1920, {width: 640, height: 360}), {width: 640, height: 1138}, 'k = max(640 / 1080, 360 / 1920)');
assert.deepEqual(P.capSize(320, 180, canvas), {width: 320, height: 180}, 'never upscaled');
assert.deepEqual(P.capSize(0.2, 0, canvas), {width: 1, height: 1}, 'at least one pixel');
// decodedSize: the asset size that crop fractions refer to. A proxy keeps the original's size; a rotated element swaps it.
assert.deepEqual(P.decodedSize({width: 1920, height: 1080}, 960, 540), {width: 1920, height: 1080}, 'a proxy keeps the original pixels');
assert.deepEqual(P.decodedSize({width: 1920, height: 1080}, 1080, 1920), {width: 1080, height: 1920}, 'a rotated phone video swaps');
assert.deepEqual(P.decodedSize({}, 64, 36), {width: 64, height: 36}, 'without an asset size the element size');
`)
}
