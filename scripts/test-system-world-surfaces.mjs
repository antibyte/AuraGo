// System World surface detail: texture set integrity, material coverage, pane seeds and the
// shader injection against the pinned Three.js standard material.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import {createHash} from 'node:crypto';
import * as THREE from 'three';
import {KINDS, WINDOWS, windowSeeds, createSurfaces} from '../ui/js/desktop/apps/sysworld-surfaces.js';
import {WINDOW_MATERIAL} from '../ui/js/desktop/apps/sysworld-windows.js';
import {sharedMaterialKey, kitPlacements} from '../ui/js/desktop/apps/sysworld-scene.js';

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
// Emissive lamp glass stays untextured: a triplanar albedo/roughness pass would
// dim the glow instead of adding detail.
const untextured = new Set(['city.garden-glass', 'city.cyan', 'city.jade', 'city.lantern-glow']);
// Kit v3 window panes are interior-mapped by sysworld-windows.js, never by the surface maps.
const windowPanes = new Set([WINDOW_MATERIAL]);
// Lobby glazing is the shared transparent material of sysworld-lobby-render.js, never the surface maps.
const lobbyGlass = new Set(['city.pane']);
const names = new Set();
for (const kit of ['v1', 'v2', 'v3']) {
  const files = await fs.readdir(`ui/3d/system-world/${kit}`).catch(() => []);
  for (const file of files) {
    if (!file.endsWith('.glb')) continue;
    const bytes = await fs.readFile(`ui/3d/system-world/${kit}/${file}`);
    const json = JSON.parse(bytes.subarray(20, 20 + bytes.readUInt32LE(12)).toString('utf8'));
    for (const material of json.materials || []) names.add(material.name);
  }
}
for (const name of names) assert.ok(KINDS[name] || WINDOWS.has(name) || untextured.has(name) || windowPanes.has(name) || lobbyGlass.has(name), 'Unmapped kit material: ' + name);

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
// Kit glass (doors, small panes) takes the surface maps; window panes never do.
const glass = compile('city.glass', 'world', 'high');
assert.ok(glass.text.includes('diffuseColor.rgb*=swAlbedoF'), 'Kit glass takes the surface maps');
const pane = new THREE.MeshStandardMaterial({name: 'city.window', vertexColors: true});
surfaces.apply(pane);
assert.equal(pane.onBeforeCompile, THREE.Material.prototype.onBeforeCompile, 'city.window belongs to sysworld-windows.js');
const lobbyPane = new THREE.MeshStandardMaterial({name: 'city.pane', vertexColors: true});
surfaces.apply(lobbyPane);
assert.equal(lobbyPane.onBeforeCompile, THREE.Material.prototype.onBeforeCompile, 'city.pane belongs to sysworld-lobby-render.js');
// Vertex-coloured (kit v3) warm/ivory parts are light fixtures: one seed 2, never per-face switching.
{
  const fixture = new THREE.BufferGeometry();
  fixture.setAttribute('position', new THREE.Float32BufferAttribute(vertices.flat(), 3));
  fixture.setIndex(indices);
  surfaces.prepare(new THREE.Mesh(fixture, new THREE.MeshStandardMaterial({name: 'city.ivory', vertexColors: true})));
  assert.ok(fixture.getAttribute('swSeed').array.every(s => s === 2), 'Kit v3 light parts must be fixtures');
}

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
// Kit v3 street materials: every new key is a projected surface, the marking wets like the road.
for (const name of ['city.kerb', 'city.setts', 'city.iron', 'city.marking', 'city.tactile']) assert.ok(KINDS[name], 'Kit v3 kind missing: ' + name);
assert.deepEqual(KINDS['city.marking'].puddle, KINDS['city.road'].puddle);
assert.equal(KINDS['city.marking'].top, 'asphalt');
// Vertex-coloured (baked AO) materials keep the surface injection and never share with plain ones.
{
  const ao = new THREE.MeshStandardMaterial({ name: 'city.kerb', vertexColors: true });
  surfaces.apply(ao);
  const shader = { uniforms: {}, vertexShader: THREE.ShaderLib.standard.vertexShader, fragmentShader: THREE.ShaderLib.standard.fragmentShader };
  ao.onBeforeCompile(shader);
  assert.ok(shader.fragmentShader.includes('diffuseColor.rgb*=swAlbedoF'), 'AO material keeps the surface maps');
  assert.notEqual(sharedMaterialKey(ao), sharedMaterialKey(new THREE.MeshStandardMaterial({ name: 'city.kerb' })), 'AO and plain materials never share');
}
assert.ok(Array.isArray(kitPlacements));
// Leaf sway (phase 4): only city.leaf, only on instanced draws and tiers above low; the lawn (leaf kind) never sways.
{
  const leafHigh = compile('city.leaf', 'world', 'high'), leafLow = compile('city.leaf', 'world', 'low');
  assert.ok(leafHigh.text.includes('#define SW_SWAY ') && leafHigh.text.includes('defined(SW_SWAY)&&defined(USE_INSTANCING)') && leafHigh.text.includes('uniform vec4 swWind'), 'leaf sway injection');
  assert.ok(!leafLow.text.includes('#define SW_SWAY '), 'no sway on low');
  surfaces.setTier('high');const lawn = new THREE.MeshStandardMaterial({ name: 'lawn' });surfaces.apply(lawn, 'world', 'city.leaf');
  const shader = { uniforms: {}, vertexShader: THREE.ShaderLib.standard.vertexShader, fragmentShader: THREE.ShaderLib.standard.fragmentShader };lawn.onBeforeCompile(shader);
  assert.ok(!shader.vertexShader.includes('#define SW_SWAY '), 'the lawn never sways');assert.notEqual(lawn.customProgramCacheKey(), leafHigh.material.customProgramCacheKey(), 'sway changes the program key');
  surfaces.setWind({ x: 0, z: 1, strength: .4, time: 12 });assert.deepEqual(shader.uniforms.swWind.value.toArray(), [0, 1, .4, 12]);
}
surfaces.dispose();
console.log(`System World surfaces: ${Object.keys(KINDS).length} kinds, ${names.size} kit materials mapped, ${total} texture bytes, pane seeds and shader hooks verified.`);
