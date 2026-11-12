// Sea life of the System World: habitats clear of the island, tier counts, the surface remap (JS twin and shader text), leap
// parabola, event schedules, loops, model manifests, and the module itself (stub models) incl. frozen frames and the street view.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import * as P from '../ui/js/desktop/apps/sysworld-sealife-paths.js';
import {REMAP, WATER, createSeaMaterials} from '../ui/js/desktop/apps/sysworld-sealife-shaders.js';
import {FOLDERS, MODELS, createSealife, place} from '../ui/js/desktop/apps/sysworld-sealife.js';

// ---- habitats: outside the island footprint plus 2 m, depth range, counts ----
for (const h of P.HABITATS) {
  assert.ok(P.clearOfIsland(h.x, h.z), 'habitat inside the island plus 2 m: ' + JSON.stringify(h));
  assert.ok(h.depth[0] >= .25 && h.depth[1] <= 2.2, 'habitat depth ' + JSON.stringify(h.depth));
  assert.ok(h.x[1] > h.x[0] && h.z[1] > h.z[0]);
}
assert.ok(!P.clearOfIsland([0, 10], [0, 10]) && !P.clearOfIsland([80, 90], [0, 10]) && P.clearOfIsland([88, 100], [0, 10]) && P.clearOfIsland([0, 10], [82.5, 90]), 'clearOfIsland edges');
const bySpecies = (list, id) => list.filter(h => h.species === id);
const table = {high: {fish_sardine: 8, fish_tang: 5, fish_clown: 4, fish_butterfly: 4}, medium: {fish_sardine: 4, fish_tang: 3, fish_clown: 2, fish_butterfly: 2}};
for (const [tier, want] of Object.entries(table)) for (const [id, n] of Object.entries(want)) assert.equal(bySpecies(P.habitatsFor(tier), id).length, n, tier + ' ' + id);
assert.equal(P.habitatsFor('ultra').length, 21); assert.equal(P.habitatsFor('low').length, 0);
for (const h of P.HABITATS) assert.equal(h.count, {fish_sardine: 48, fish_tang: 14, fish_clown: 10, fish_butterfly: 10}[h.species]);
const quayWall = P.HABITATS.filter(h => h.species !== 'fish_sardine' && h.z[1] <= 92 && h.z[0] >= 83);
assert.equal(quayWall.length, 13, 'tangs, clowns and butterflies keep to the quay wall');
assert.equal(P.HABITATS.filter(h => h.z[0] >= 83 && h.x[0] >= -75 && h.x[1] <= 75 && h.z[1] <= 112).length, 18, 'most habitats lie along the quay');
assert.deepEqual(P.TIER_COUNTS.low, {sharks: 0, hammers: 0, turtles: 0, whale: 0, mantas: 0, jellies: 0});
assert.deepEqual(P.TIER_COUNTS.medium, {sharks: 2, hammers: 1, turtles: 2, whale: 1, mantas: 1, jellies: 12});
assert.deepEqual(P.TIER_COUNTS.high, {sharks: 3, hammers: 2, turtles: 4, whale: 1, mantas: 2, jellies: 36});
assert.deepEqual(P.SCALE, {fish: 3, shark: 1.3, hammer: 1.3, turtle: 1.6, whale: 1.3, manta: 1.2, jelly: 2});

// ---- loops: clear of the island, depth ranges, speeds, shore distances ----
const out = {}, shore = ([x, z]) => Math.hypot(Math.max(0, P.ISLAND.x0 - x, x - P.ISLAND.x1), Math.max(0, P.ISLAND.z0 - z, z - P.ISLAND.z1));
const speedRange = {shark: [1.2, 1.8], hammer: [1.2, 1.8], turtle: [.25, .5], whale: [1, 2], manta: [1, 1.6]};
for (const [kind, loops] of Object.entries(P.LOOPS)) loops.forEach((L, i) => {
  let lo = Infinity, hi = -Infinity, near = Infinity, far = 0, fin = 0, samples = 0;
  for (let t = 0; t < 2400; t += .5) {
    P.seaPosition(kind, i, t, loops.length, out); const d = P.SEA - out.y, s = shore([out.x, out.z]);
    lo = Math.min(lo, d); hi = Math.max(hi, d); near = Math.min(near, s); far = Math.max(far, s); samples++; if (d < .48) fin++;
    assert.ok(Number.isFinite(out.x + out.y + out.z), kind + ' NaN');
  }
  assert.ok(near >= P.KEEP_FROM_ISLAND + 2, kind + i + ' too close to the island: ' + near);
  const [a, b] = speedRange[kind]; assert.ok(P.loopSpeed(L) >= a && P.loopSpeed(L) <= b, kind + i + ' speed ' + P.loopSpeed(L));
  if (kind === 'shark') { assert.ok(lo >= .2 - 1e-9 && hi <= 1.4 + 1e-9, 'shark depth ' + lo + ' ' + hi); assert.ok(fin / samples > .15 && fin / samples < .6, 'shark fin out for stretches: ' + fin / samples); }
  if (kind === 'hammer') { assert.ok(lo >= 1.3 - 1e-9 && hi <= 3.3 + 1e-9 && near >= 44 && far <= 90, 'hammer ' + [lo, hi, near, far]); }
  if (kind === 'manta') { assert.ok(lo >= .6 - 1e-9 && hi <= 1.6 + 1e-9 && near >= 60 && far <= 150, 'manta ' + [lo, hi, near, far]); }
  if (kind === 'whale') { assert.ok(lo < 1 && hi <= 5.5 + 1e-9, 'whale depth ' + [lo, hi]); }
});
{ const w = P.LOOPS.whale[0], rs = []; for (let a = 0; a < TAU(); a += .1) rs.push(Math.hypot(w.rx * Math.cos(a) + w.cx, w.rz * Math.sin(a) + w.cz)); assert.ok(Math.min(...rs) > 150 && Math.max(...rs) < 270, 'whale loop about 210 m out'); }
function TAU() { return Math.PI * 2; }

// ---- surface remap: JS twin ----
const cam = [30, 40, 150], S = {};
for (const p of [[10, -4, 90], [31, -3.3, 140], [-50, -9, 20], [30, -3.2001, 149]]) {
  P.seaRemap(...cam, ...p, S);
  const toP = p.map((v, i) => v - cam[i]), toS = [S.x, S.y, S.z].map((v, i) => v - cam[i]), cross = [toP[1] * toS[2] - toP[2] * toS[1], toP[2] * toS[0] - toP[0] * toS[2], toP[0] * toS[1] - toP[1] * toS[0]];
  assert.ok(Math.hypot(...cross) < 1e-6 * Math.hypot(...toP) * Math.hypot(...toS), 'remap stays on the view ray');
  const bias = P.remapBias(P.SEA - p[1], Math.hypot(...toS)), reach = Math.hypot(...toS), onPlane = (S.y - P.SEA) / (toS[1] / reach);   // distance back along the ray at which the plane is crossed
  assert.ok(Math.abs(Math.abs(onPlane) - bias) < 2e-3 && S.y >= P.SEA - 1e-9, 'remap sits on the sea plane, moved ' + bias + ' m towards the camera: ' + onPlane);
  assert.ok(Math.hypot(...toS) < Math.hypot(...toP), 'remap is nearer the camera');
}
const above = P.seaRemap(...cam, 12, 3, 80, {}); assert.deepEqual([above.x, above.y, above.z], [12, 3, 80]);
const atSurface = P.seaRemap(...cam, 12, P.SEA, 80, {}); assert.deepEqual([atSurface.x, atSurface.y, atSurface.z], [12, P.SEA, 80]);
{ // a point just below the surface projects to (nearly) the same screen place as the point itself: the two agree at the surface
  const q = P.seaRemap(...cam, 12, P.SEA - 1e-4, 80, {}), p = new THREE.Vector3(12, P.SEA - 1e-4, 80), c = new THREE.PerspectiveCamera(43, 1.7, .3, 1800);
  c.position.set(...cam); c.lookAt(12, -3, 80); c.updateMatrixWorld(); c.updateProjectionMatrix();
  const a = p.clone().project(c), b = new THREE.Vector3(q.x, q.y, q.z).project(c); assert.ok(Math.hypot(a.x - b.x, a.y - b.y) < 1e-6, 'same screen position');
  assert.ok(P.seaRemap(cam[0], P.SEA + 1e-3, cam[2], 5, -9, 5, {}).y <= P.SEA + 1.1e-3, 'camera at the surface does not blow up'); }
assert.ok(P.remapBias(.3, 10) > P.remapBias(2, 10), 'shallower animals win over deeper ones');
for (const v of [[0, 0, 0], [1e4, -1e3, 1e4]]) { const r = P.seaRemap(cam[0], P.SEA + .01, cam[2], ...v, {}); assert.ok(Number.isFinite(r.x + r.y + r.z)); }
// the shader text states the same numbers and guards every division
assert.ok(REMAP.includes('max(C.y-uSea,0.02)') && REMAP.includes('h/max(C.y-P.y,h)') && REMAP.includes('max(length(toC),0.001)'), 'seaClip guards');
assert.ok(REMAP.includes('0.03+0.02*clamp(6.0-below,0.0,6.0)') && REMAP.includes('1.0+0.01*len'), 'seaClip bias matches remapBias');
for (const m of REMAP.matchAll(/\/([^;,)]{0,12})/g)) assert.ok(/^(max\(|len)/.test(m[1]) || m[1].startsWith('*'), 'unguarded division in seaClip: /' + m[1]);
assert.ok(WATER.includes('insideIsle') && (await fs.readFile('ui/js/desktop/apps/sysworld-sealife-shaders.js', 'utf8')).split('insideIsle(vWorldPos)').length === 4, 'every fragment shader discards what lies inside the island'); assert.ok(WATER.includes('max(cosV,0.12)') && WATER.includes('0.45,0.16,0.11') && WATER.includes('pow(T.g,0.7)') && WATER.includes('0.02+0.98*pow(clamp(1.0-cosV,0.0,1.0),5.0)'), 'water look');
for (const m of WATER.replace(/\x2f\x2f.*$/gm, '').matchAll(/\/([^;,)]{0,14})/g)) assert.ok(/^max\(/.test(m[1]), 'unguarded division in the water chunk: /' + m[1]);

// ---- leaps ----
const lp = {};
for (let k = 0; k < 300; k++) {
  const q = P.leapParams(k, {}); assert.ok(q.apex >= .6 && q.apex <= 1.1, 'apex'); assert.ok(q.speed >= 2.2 && q.speed <= 3.2);
  let top = 0, topAt = 0; for (let s = 0; s <= 1.0001; s += .01) { P.leapPose(q.apex, q.speed, 1, 0, s, lp); if (lp.h > top) { top = lp.h; topAt = s; }
    assert.ok(Math.abs(Math.hypot(lp.fx, lp.fy, lp.fz) - 1) < 1e-9); assert.equal(lp.fy > 0, s < .5 || (s === .5 && lp.fy >= 0) ? lp.fy > 0 : false, 'nose follows the arc'); }
  assert.ok(Math.abs(top - q.apex) < 1e-3 && Math.abs(topAt - .5) < .011, 'apex reached half way');
  P.leapPose(q.apex, q.speed, 1, 0, 0, lp); assert.equal(lp.h, 0); P.leapPose(q.apex, q.speed, 1, 0, 1, lp); assert.ok(Math.abs(lp.h) < 1e-12 && Math.abs(lp.x - q.speed * P.LEAP.duration) < 1e-9, 'enters the water where it should');
}
assert.equal(P.LEAP.duration, .9);
for (let k = 1; k < 600; k++) { const g = P.LEAP_STARTS[k] - P.LEAP_STARTS[k - 1]; assert.ok(g >= 4 && g <= 9, 'leap gap ' + g); }
for (let k = 1; k < 100; k++) { assert.ok(P.WHALE_EVENTS[k] - P.WHALE_EVENTS[k - 1] >= 40 && P.WHALE_EVENTS[k] - P.WHALE_EVENTS[k - 1] <= 60, 'whale gap'); assert.ok(P.TURTLE_EVENTS[k] - P.TURTLE_EVENTS[k - 1] >= 30 && P.TURTLE_EVENTS[k] - P.TURTLE_EVENTS[k - 1] <= 50, 'turtle gap'); }

// ---- whale, turtle, blow ----
const pose = {}, E = P.WHALE_EVENTS[3], top = 1.326 * P.SCALE.whale;
assert.equal(P.depthAt('whale', 0, E - 30), 5.5, 'cruising');
assert.ok(P.depthAt('whale', 0, E + 1) < top, 'back above the surface while up'); P.seaPose('whale', 0, E + 1, 1, pose);
P.seaPose('whale', 0, E + 4.6, 1, pose); assert.ok(pose.fy < -.3, 'dives nose-down: ' + pose.fy);
P.seaPose('whale', 0, E - 30, 1, pose); assert.ok(Math.abs(pose.fy) < .01, 'level while cruising');
const nT = 4, T = P.TURTLE_EVENTS[6], who = 6 % nT;
for (let i = 0; i < nT; i++) { const d = P.depthAt('turtle', i, T + 1, nT); if (i === who) assert.ok(d <= .13, 'turtle comes up: ' + d); else assert.ok(d >= .9, 'the others stay down'); }
assert.ok(P.depthAt('turtle', who, T + 3.5, nT) <= .13 && P.depthAt('turtle', who, T + 12, nT) >= .9, 'about 4 s at the top');
let maxRise = 0, alive = 0; const pf = {};
for (let t = E; t < E + 8; t += .05) for (let i = 0; i < P.BLOW.count; i++) { const a = P.blowPuff(t, i, pf); if (a >= 0) { alive++; maxRise = Math.max(maxRise, pf.y - P.SEA); assert.ok(a <= 1 && Number.isFinite(pf.x + pf.z)); } }
assert.ok(alive > 0 && maxRise > 3.5 && maxRise < 4.6, 'puffs rise about 4 m: ' + maxRise);
assert.equal(P.blowPuff(E - 40, 0, pf), -1); assert.equal(P.blowPuff(E + 20, 0, pf), -1);
assert.equal(P.waterInView(12, 1.9, 19), false, 'street-agent view sees no water'); assert.equal(P.waterInView(95, 70, 120), true); assert.equal(P.waterInView(30, 1.9, 79.2), true); assert.equal(P.waterInView(120, 2, 0), true);
for (let i = 0; i < P.JELLY_HOMES.length; i++) { P.jellyPosition(i, 777, out); const d = P.SEA - out.y; assert.ok(d >= .29 && d <= 1.51, 'jelly depth ' + d); assert.ok(out.z >= P.ISLAND.z1 + 1.5 - 1e-9 && out.z < 160 && Math.abs(out.x) < 80, 'jellies stay near the quay and the buoys'); }

// ---- cost rules: distance levels, fixed boids step, drawn-only uploads ----
assert.deepEqual([P.DRAW.school, P.DRAW.big, P.DRAW.jelly, P.BOIDS_STEP], [90, 320, 60, .05]);
for (const [d, near, want] of [[10, true, 1], [59, true, 1], [61, true, 2], [89, true, 2], [91, true, 0], [10, false, 1], [26, false, 2], [44, false, 2], [46, false, 0], [300, false, 0]]) assert.equal(P.schoolStride(d, near), want, 'stride at ' + d + (near ? ' (water edge)' : ' (inland or high)'));
assert.equal(P.nearWater(-31, 1.9, 79.2), true, 'quay camera is at the water'); assert.equal(P.nearWater(-30, 7, -74), false, 'park camera is inland'); assert.equal(P.nearWater(95, 70, 120), false, 'overview is high'); assert.equal(P.nearWater(120, 2, 0), true, 'a boat deck is at the water');

{ const a = new Float32Array(32), m = new THREE.Matrix4();
  place(a, 0, 1, 2, 3, 0, 0, 1, 2, 0); m.fromArray(a, 0); const e = m.elements; assert.deepEqual([e[0], e[5], e[10]], [2, 2, 2]); assert.deepEqual([e[12], e[13], e[14]], [1, 2, 3]);
  for (const f of [[1, 0, 0], [0, 0, -1], [.3, .5, .8], [0, 1, 0], [0, -1, 0], [-.4, -.9, .1]]) for (const roll of [0, .4]) {
    place(a, 16, 0, 0, 0, ...f, 1, roll); m.fromArray(a, 16); assert.ok(Math.abs(m.determinant() - 1) < 1e-6, 'rotation ' + f); const z = new THREE.Vector3().setFromMatrixColumn(m, 2), n = new THREE.Vector3(...f).normalize(); assert.ok(z.distanceTo(n) < 1e-6, 'nose along heading'); }
  place(a, 0, 0, 0, 0, 0, .3, 1, 1, 0); m.fromArray(a, 0); assert.ok(new THREE.Vector3().setFromMatrixColumn(m, 1).y > .9, 'level fish keep their back up'); }

// ---- model folders: the manifests match the files the module names ----
const manifests = {fish: 'ui/3d/system-world/sea-life/v1/', chat: 'ui/3d/chat-ocean-life/v1/', abyss: 'ui/3d/screensaver/abyss/v1/'};
assert.deepEqual(Object.keys(FOLDERS).sort(), Object.keys(manifests).sort());
for (const [key, dir] of Object.entries(manifests)) assert.equal('ui/3d/' + FOLDERS[key], dir);
const loaded = {};
for (const [key, dir] of Object.entries(manifests)) loaded[key] = JSON.parse(await fs.readFile(dir + 'manifest.json', 'utf8'));
for (const m of MODELS) {
  const entry = loaded[m.folder].assets.find(a => a.id === m.id); assert.ok(entry, m.id + ' is in the ' + m.folder + ' manifest'); assert.equal(entry.file, m.file);
  const bytes = await fs.readFile(manifests[m.folder] + m.file); assert.equal(bytes.length, entry.bytes, m.file + ' bytes'); assert.equal(crypto.createHash('sha256').update(bytes).digest('hex'), entry.sha256, m.file + ' sha256');
  assert.equal(bytes.readUInt32LE(0), 0x46546c67, m.file + ' is binary glTF');
  const json = JSON.parse(bytes.subarray(20, 20 + bytes.readUInt32LE(12)).toString()); assert.ok(!json.extensionsRequired?.length, m.file + ' needs no decoder (plain GLTFLoader)');
  for (const prim of json.meshes.flatMap(x => x.primitives)) for (const a of ['POSITION', 'NORMAL', 'TEXCOORD_0', 'TEXCOORD_1']) assert.ok(prim.attributes[a] !== undefined, m.file + ' ' + a);
}
for (const m of MODELS.filter(x => x.folder === 'fish')) { const a = loaded.fish.assets.find(a => a.id === m.id); assert.ok(a.triangles <= (m.id === 'jelly_moon' ? 1500 : 320), 'light model ' + m.id); assert.ok(['chat-ocean-life', 'abyss'].includes(a.kit), m.id + ' kit field'); }

// ---- the module with stub models ----
const water = {day: {value: 1}, dusk: {value: 0}, sunDir: new THREE.Vector3(-70, 145, 85).normalize(), sunColor: new THREE.Color(0xdff0ff), shallow: new THREE.Color(0x1d6f98), sky: new THREE.Color(0x7090a0)};
const partNames = {fish_sardine: ['body', 'fin', 'eye'], fish_tang: ['body', 'fin', 'eye'], fish_clown: ['body', 'fin', 'eye'], fish_butterfly: ['body', 'fin', 'eye'], shark_reef: ['body', 'fin', 'eye'], shark_hammer: ['body', 'fin', 'eye'],
  turtle: ['shell', 'body', 'flipper', 'eye'], whale: ['body', 'fin', 'flipper', 'fluke', 'eye'], manta: ['manta_top', 'manta_belly'], jelly_moon: ['bell', 'gonad', 'arm', 'tentacle']};
const files = new Map(MODELS.map(m => [m.file, m.id]));
const requested = [];
const stubLoad = (folder, file) => {
  requested.push(folder + file); const id = files.get(file); if (!id) return Promise.reject(Error('unknown ' + file));
  const root = new THREE.Group();
  for (const name of partNames[id]) { const g = new THREE.BoxGeometry(.4, .3, 1.6); g.setAttribute('uv1', g.getAttribute('uv').clone()); root.add(new THREE.Mesh(g, new THREE.MeshBasicMaterial({name}))); }
  return Promise.resolve(root);
};
const flush = () => new Promise(r => setTimeout(r, 20));
const overview = new THREE.PerspectiveCamera(43, 1.7, .3, 1800); overview.position.set(95, 70, 120); overview.lookAt(0, 10, -10);
const quay = new THREE.PerspectiveCamera(43, 1.7, .3, 1800); quay.position.set(-20, 1.9, 79.2); quay.lookAt(-14, -3.2, 90);
const street = new THREE.PerspectiveCamera(43, 1.7, .3, 1800); street.position.set(12, 1.9, 19); street.lookAt(0, 6, -12);
const finite = list => list.every(Number.isFinite);
async function run(tier) {
  const group = new THREE.Group(), sea = createSealife({group, load: stubLoad, water});
  sea.setTier(tier); sea.start(); await flush(); await flush();
  return {group, sea};
}
{
  const {group, sea} = await run('low'); assert.equal(sea.stats().loaded, false); assert.deepEqual(requested, [], 'low tier loads nothing'); sea.update(1, quay, true); sea.dispose();
}
{
  const {group, sea} = await run('high'), s = sea.stats();
  assert.equal(s.failures, 0); assert.ok(s.loaded); assert.equal(s.schools, 21); assert.equal(s.fish, 8 * 48 + 5 * 14 + 4 * 10 + 4 * 10);
  assert.deepEqual([s.sharks, s.hammerheads, s.turtles, s.whale, s.mantas, s.jellies], [3, 2, 4, 1, 2, 36]);
  assert.equal(requested.length, 10);
  const holder = group.getObjectByName('sealife');
  // quay camera: fish are seen, big animals only within reach and view
  let t = 100; for (let i = 0; i < 60; i++) { t += .1; sea.update(t, quay, true); }
  let q = sea.stats(); assert.ok(q.visibleFish > 0 && q.visibleSchools > 0 && q.drawCalls > 0, 'fish near the quay are drawn: ' + JSON.stringify(q));
  assert.ok(q.visibleSchools < 21, 'schools out of view or beyond 90 m are not drawn');
  for (const m of holder.children) { assert.ok(m.frustumCulled === false); const a = m.instanceMatrix.array; for (let i = 0; i < m.count * 16; i++) assert.ok(Number.isFinite(a[i]), 'finite matrix'); }
  // fish stay inside their habitat and under water
  for (let i = 0; i < 600; i++) { t += .1; sea.update(t, quay, true); }
  P.HABITATS.forEach((h, i) => { const l = sea.locate('school', i); assert.ok(l.x >= h.x[0] && l.x <= h.x[1] && l.z >= h.z[0] && l.z <= h.z[1] && l.y < P.SEA && l.y >= P.SEA - h.depth[1], 'school ' + i + ' in its habitat: ' + JSON.stringify(l)); });
  // a frozen frame: the same time again changes nothing and stays finite
  const snapshot = holder.children.map(m => Array.from(m.instanceMatrix.array.subarray(0, Math.max(1, m.count) * 16)));
  sea.update(t, quay, false); sea.update(t, quay, false);
  holder.children.forEach((m, i) => { const a = Array.from(m.instanceMatrix.array.subarray(0, Math.max(1, m.count) * 16)); assert.ok(a.every(Number.isFinite)); if (m.visible) assert.deepEqual(a.slice(0, snapshot[i].length), snapshot[i], 'frozen sea life does not move'); });
  // forced leap: a fish leaves and re-enters the water, rings appear
  assert.ok(sea.stats().leaps >= 5, 'natural leaps every 4-9 s: ' + sea.stats().leaps); sea.update(t + .01, quay, true); while (sea.locate('leap')) { t += .2; sea.update(t, quay, true); } t += .05; sea.update(t, quay, true);
  const leapsBefore = sea.stats().leaps; sea.debug({leapNow: true}); sea.update(t + .05, quay, true); assert.equal(sea.stats().leaps, leapsBefore + 1, 'forced leap'); assert.ok(sea.locate('leap'), 'the leaper has a position');
  sea.update(t + .5, quay, true); assert.ok(sea.locate('leap').y > P.SEA - 1e-9, 'above the surface mid-leap'); assert.ok(sea.stats().splashes >= 1, 'splash ring'); sea.update(t + 1.1, quay, true); assert.equal(sea.locate('leap'), null);
  sea.update(t + 1.2, quay, true); sea.update(t + 3, quay, true); assert.equal(sea.stats().splashes, 0, 'rings fade away');
  // events by force
  for (const [opts, kind] of [[{whaleSurfaceAt: 1}, 'whale'], [{turtleSurfaceAt: 1, turtle: 1}, 'turtle'], [{sharkFinOut: 0}, 'shark']]) {
    sea.debug({reset: true}); sea.debug(opts); sea.update(t + 3.1, overview, true); const at = sea.locate(kind, kind === 'turtle' ? 1 : 0); assert.ok(at && finite([at.x, at.y, at.z]), kind);
    if (kind === 'whale') assert.ok(at.y > P.SEA - 1.5, 'whale up: ' + at.y); if (kind === 'turtle') assert.ok(at.y > P.SEA - .2, 'turtle up: ' + at.y); if (kind === 'shark') assert.ok(at.y > P.SEA - .3, 'shark fin: ' + at.y);
  }
  sea.debug({reset: true});
  // overview: everything within 320 m and in view is submitted, big ones included
  sea.update(t + 4, overview, true); const o = sea.stats(); assert.ok(o.visibleBig > 0 && o.visibleFish >= 0 && o.drawCalls > 0, 'overview draws: ' + JSON.stringify(o));
  // the street view sees no water: nothing simulated, nothing submitted (the street views must not gain triangles)
  sea.update(t + 5, street, true); const st = sea.stats(); assert.equal(st.visibleFish + st.visibleBig + st.drawCalls + st.splashes + st.puffs, 0, 'street view submits nothing: ' + JSON.stringify(st));
  for (const m of holder.children) assert.ok(!m.visible, 'every mesh hidden in the street view');
  // fixed 20 Hz boids: a long frame steps at most twice, a short one not at all; the same clock gives the same school
  { const g2 = new THREE.Group(), s2 = createSealife({group: g2, load: stubLoad, water}); s2.setTier('high'); s2.start(); await flush(); await flush();
    const g3 = new THREE.Group(), s3 = createSealife({group: g3, load: stubLoad, water}); s3.setTier('high'); s3.start(); await flush(); await flush();
    let u = 50; s2.update(u, quay, true); s3.update(u, quay, true);
    for (let i = 0; i < 90; i++) { u += 1 / 60; s2.update(u, quay, true); }
    for (let i = 0; i < 45; i++) { u += 1 / 30; }   // s3 sees the same 1.5 s as 45 frames of 1/30
    let v = 50; for (let i = 0; i < 45; i++) { v += 1 / 30; s3.update(v, quay, true); }
    const a = s2.locate('school', 9), b = s3.locate('school', 9); assert.ok(Math.hypot(a.x - b.x, a.z - b.z) < 3, 'frame rate hardly matters: ' + Math.hypot(a.x - b.x, a.z - b.z));
    const still = s2.locate('school', 9); s2.update(u, quay, false); const same = s2.locate('school', 9); assert.deepEqual(still, same, 'a frozen clock does not step'); s2.dispose(); s3.dispose(); }
  // the hide-and-pause switch (cost A/B): nothing drawn, nothing simulated, back on afterwards
  sea.update(t + 5.5, overview, true); assert.ok(sea.stats().drawCalls > 0); sea.debug({hidden: true}); sea.update(t + 5.6, overview, true); { const h = sea.stats(); assert.equal(h.drawCalls + h.visibleFish + h.visibleBig, 0, 'hidden switch'); } sea.debug({hidden: false}); sea.update(t + 5.7, overview, true); assert.ok(sea.stats().drawCalls > 0, 'visible again');
  // pins put an animal at a place (the quay occlusion check)
  sea.debug({pin: {kind: 'jelly', index: 0, x: -25, z: 81.3, depth: .7}}); sea.update(t + 5.8, quay, true); assert.ok(Math.abs(sea.locate('jelly', 0).z - 81.3) < 1e-9); sea.debug({reset: true});
  // no per-frame allocation worth the name: a thousand frames
  const before = process.memoryUsage().heapUsed; for (let i = 0; i < 1000; i++) sea.update(t + 6 + i * .02, overview, true); assert.ok(process.memoryUsage().heapUsed - before < 60e6, 'allocation across 1000 frames');
  sea.dispose(); assert.equal(group.children.length, 0, 'dispose empties the group');
}
{
  const {sea} = await run('medium'), s = sea.stats(); assert.equal(s.schools, 11); assert.equal(s.fish, 4 * 48 + 3 * 14 + 2 * 10 + 2 * 10); assert.deepEqual([s.sharks, s.hammerheads, s.turtles, s.whale, s.mantas, s.jellies], [2, 1, 2, 1, 1, 12]);
  sea.setTier('low'); assert.equal(sea.stats().loaded, false); sea.setTier('ultra'); await flush(); await flush(); assert.equal(sea.stats().schools, 21); sea.dispose();
}
{ // a failed load leaves the sea empty and is counted
  const group = new THREE.Group(), sea = createSealife({group, load: () => Promise.reject(Error('offline')), water}); sea.start(); await flush(); await flush();
  const s = sea.stats(); assert.equal(s.failures, 10); assert.equal(s.fish, 0); sea.update(5, quay, true); sea.dispose();
}
{ // materials compile-free: the factory builds every kind and counts them
  const mats = createSeaMaterials(THREE, water, {value: 0}), species = {spec: {shader: 'fish', variant: 0, swim: {k: 2}, length: .3}, front: .1, scale: 1};
  for (const part of ['body', 'fin', 'eye']) assert.ok(mats.creature(species, part).fog === true);
  mats.jelly('bell'); mats.manta(species); mats.ring(); mats.puff(); assert.equal(mats.count(), 7); mats.dispose(); assert.equal(mats.count(), 0);
}
// ---- source guards ----
for (const f of ['sysworld-sealife.js', 'sysworld-sealife-paths.js', 'sysworld-sealife-shaders.js']) { const text = await fs.readFile('ui/js/desktop/apps/' + f, 'utf8'); assert.ok(!/Math\.random/.test(text), f + ' uses Math.random'); assert.ok(!/https?:\/\//.test(text), f + ' has a URL'); assert.ok(!/ocean-life-(scene|actors)/.test(text), f + ' imports the chat scene or actors'); }
console.log('System World sea life: habitats clear, tiers, remap twin and shader guards, leaps, events, loops, manifests, stub run (frozen frame, street view, forced events) verified.');
