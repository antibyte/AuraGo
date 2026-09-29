// System World surface detail: texture set integrity, material coverage, pane seeds and the
// shader injection against the pinned Three.js standard material.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import {createHash} from 'node:crypto';
import * as THREE from 'three';
import {KINDS, WINDOWS, windowSeeds, createSurfaces} from '../ui/js/desktop/apps/sysworld-surfaces.js';

const dir = 'ui/3d/system-world/textures/v1/';
const manifest = JSON.parse(await fs.readFile(dir + 'manifest.json', 'utf8'));
assert.equal(manifest.license, 'MIT');
let total = 0;
for (const id of new Set(Object.values(KINDS).flatMap(kind => [kind.top, kind.side]))) {
  const entry = manifest.textures.find(texture => texture.id === id);
  assert.ok(entry, 'Surface map missing from manifest: ' + id);
  const data = await fs.readFile(dir + entry.file);
  assert.equal(data.length, entry.bytes, entry.file + ' size');
  assert.equal(createHash('sha256').update(data).digest('hex'), entry.sha256, entry.file + ' hash');
  assert.ok(entry.tile_metres > 0 && [512, 1024].includes(entry.px), entry.file + ' metadata');
  total += data.length;
}
assert.ok(total < 1.5 * 1024 * 1024, 'Surface textures exceed 1.5 MiB: ' + total);

// Every kit material is textured, a window, or deliberately left untextured (glass and signals).
const untextured = new Set(['city.glass', 'city.garden-glass', 'city.cyan', 'city.jade']);
const names = new Set();
for (const kit of ['v1', 'v2']) {
  for (const file of await fs.readdir(`ui/3d/system-world/${kit}`)) {
    if (!file.endsWith('.glb')) continue;
    const bytes = await fs.readFile(`ui/3d/system-world/${kit}/${file}`);
    const json = JSON.parse(bytes.subarray(20, 20 + bytes.readUInt32LE(12)).toString('utf8'));
    for (const material of json.materials || []) names.add(material.name);
  }
}
for (const name of names) assert.ok(KINDS[name] || WINDOWS.has(name) || untextured.has(name), 'Unmapped kit material: ' + name);

// Separate four-vertex panes get their own constant seed; larger lit parts are fixtures (2).
const quad = (x, y) => [[x, y, 0], [x + 1, y, 0], [x + 1, y + 1, 0], [x, y + 1, 0]];
const vertices = [...quad(0, 0), ...quad(3, 0), ...quad(6, 0), ...quad(7, 0)];
const indices = [0, 1, 2, 0, 2, 3, 4, 5, 6, 4, 6, 7, 8, 9, 10, 8, 10, 11, 12, 13, 14, 12, 14, 15, 9, 12, 14];
const geometry = new THREE.BufferGeometry();
geometry.setAttribute('position', new THREE.Float32BufferAttribute(vertices.flat(), 3));
geometry.setIndex(indices);
windowSeeds(geometry);
const seeds = geometry.getAttribute('swSeed').array;
assert.ok(seeds.slice(0, 4).every(s => s === seeds[0]) && seeds.slice(4, 8).every(s => s === seeds[4]), 'Pane seed varies inside one pane');
assert.ok(seeds[0] !== seeds[4] && seeds[0] >= 0 && seeds[0] < 1 && seeds[4] >= 0 && seeds[4] < 1, 'Panes share a seed');
assert.ok(seeds.slice(8).every(s => s === 2), 'Joined lit part is not a fixture');

// The injection must hook every chunk of the pinned standard shader, for all tiers and spaces.
const surfaces = createSurfaces({capabilities: {getMaxAnisotropy: () => 8}}, {});
const compile = (name, space, tier) => {
  surfaces.setTier(tier);
  const material = new THREE.MeshStandardMaterial({name});
  surfaces.apply(material, space);
  const shader = {uniforms: {}, vertexShader: THREE.ShaderLib.standard.vertexShader, fragmentShader: THREE.ShaderLib.standard.fragmentShader};
  material.onBeforeCompile(shader);
  return {material, shader, text: shader.vertexShader + shader.fragmentShader};
};
for (const tier of ['low', 'medium', 'high']) {
  for (const space of ['world', 'object']) {
    const {material, shader, text} = compile('city.road', space, tier);
    for (const marker of ['vSwPos=(swObject', 'diffuseColor.rgb*=swAlbedoF', 'roughnessFactor=clamp(roughnessFactor*swRoughF',
      'metalnessFactor*=mix', 'normal=swPerturb(-vViewPosition', 'totalEmissiveRadiance+=swSky', 'float swWetness=']) {
      assert.ok(text.includes(marker), `${tier}/${space}: injection missing ${marker}`);
    }
    assert.ok(text.includes('#define SW_LEVEL ' + {low: 0, medium: 1, high: 2}[tier]), tier + ' level define');
    assert.equal(text.includes('#define SW_WORLD'), space === 'world', space + ' space define');
    assert.ok(shader.uniforms.swTop && shader.uniforms.swSide && shader.uniforms.swWet && shader.uniforms.swShelter, 'uniforms missing');
    assert.ok(material.customProgramCacheKey().includes(space), 'cache key ignores space');
  }
}
const window = compile('city.ivory', 'world', 'high');
assert.ok(window.text.includes('attribute float swSeed') && window.text.includes('swPane') && !window.text.includes('#define SW_SURFACE'), 'Window material injection');
assert.ok(window.text.includes('#define SW_LEVEL 0'), 'Window material must define SW_LEVEL for #if');
const glass = new THREE.MeshStandardMaterial({name: 'city.glass'});
surfaces.apply(glass);
assert.equal(glass.onBeforeCompile, THREE.Material.prototype.onBeforeCompile, 'Glass must stay untouched');

// Pane lighting: few panes by day, most at night; wetness follows weather, instantly without motion.
surfaces.setLighting(1, 0);
const day = surfaces.stats().windows;
surfaces.setLighting(0, 0);
assert.ok(day < .2 && surfaces.stats().windows > .8, 'Pane lighting does not follow the time of day');
surfaces.setWeather('rain');
surfaces.update(1, true);
const easing = surfaces.stats().wet;
surfaces.update(0, false);
assert.ok(easing > 0 && easing < 1 && surfaces.stats().wet === 1, 'Wetness easing or reduced-motion snap broken');
surfaces.dispose();
console.log(`System World surfaces: ${Object.keys(KINDS).length} kinds, ${names.size} kit materials mapped, ${total} texture bytes, pane seeds and shader hooks verified.`);
