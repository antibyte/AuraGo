import fs from 'node:fs';
import { createHash } from 'node:crypto';
import assert from 'node:assert/strict';
import { companionPose, companionGait, LOOP } from '../ui/js/desktop/apps/sysworld-companion.js';
import { groundHeight } from '../ui/js/desktop/apps/sysworld-exploration.js';
import { streetProfile } from '../ui/js/desktop/apps/sysworld-ground.js';

// --- Deterministic, bounded patrol ------------------------------------------
const first = companionPose(0), loop = companionPose(LOOP.seconds);
assert.ok(Math.abs(first.x - loop.x) < 1e-9 && Math.abs(first.z - loop.z) < 1e-9, 'The patrol is a closed loop');
assert.deepEqual(companionPose(7), companionPose(7), 'The pose is pure');
let minX = Infinity, maxX = -Infinity, minZ = Infinity, maxZ = -Infinity, previous = -Infinity;
for (let t = 0; t <= 2 * LOOP.seconds; t += .05) {
  const p = companionPose(t);
  assert.ok(Number.isFinite(p.x) && Number.isFinite(p.z) && Number.isFinite(p.heading));
  minX = Math.min(minX, p.x); maxX = Math.max(maxX, p.x); minZ = Math.min(minZ, p.z); maxZ = Math.max(maxZ, p.z);
  assert.ok(p.phase >= previous, 'Gait phase never runs backwards'); previous = p.phase;
}
// One level: the plot east of the x = 18 street, clear of its kerb, the southern boulevard (z <= 65),
// the quay step (z >= 74), the missions pavilion (x >= 37) and the harbour hover dock.
assert.ok(minX > 24 && maxX < 35.5, 'Patrol stays on the plot: ' + minX + '..' + maxX);
assert.ok(minZ > 65 && maxZ < 74, 'Patrol stays off the boulevard and the quay: ' + minZ + '..' + maxZ);
const level = groundHeight(LOOP.cx, LOOP.cz);
for (let t = 0; t <= LOOP.seconds; t += .05) {
  const p = companionPose(t), s = streetProfile(p.x, p.z);
  assert.ok(!s || s.kind === 'pavement', 'The patrol never touches a kerb or a carriageway');
  for (const [dx, dz] of [[.6, 0], [-.6, 0], [0, .35], [0, -.35]]) assert.equal(groundHeight(p.x + dx, p.z + dz), level, 'The patrol stays on one level');
  assert.ok(Math.hypot(p.x - 23.3, p.z - 75) > 2.5, 'Patrol clears the hover dock');
}

// --- A diagonal trot, not four legs in lockstep -----------------------------
for (const phase of [0, .8, 2.1, 5.7]) {
  const g = companionGait(phase);
  assert.ok(Math.abs(g.fl.hip - g.hr.hip) < 1e-9 && Math.abs(g.fr.hip - g.hl.hip) < 1e-9, 'Diagonal pairs move together');
  assert.ok(Math.abs(g.fl.hip + g.fr.hip) < 1e-9, 'Opposite pairs are half a cycle apart');
  assert.ok(Math.abs(g.fl.hip) <= .26 && Math.abs(g.fl.uleg) <= .13, 'Joint travel stays bounded');
}

// --- Shipped asset, provenance and budget ------------------------------------
const sha = data => createHash('sha256').update(data).digest('hex');
const meta = JSON.parse(fs.readFileSync('ui/3d/system-world/spot.json', 'utf8'));
const source = JSON.parse(fs.readFileSync('assets/system-world/spot-source.json', 'utf8'));
const glb = fs.readFileSync('ui/3d/system-world/' + meta.file);
assert.equal(glb.length, meta.bytes);
assert.equal(sha(glb), meta.sha256, 'Shipped GLB matches its pinned hash');
assert.equal(meta.sha256.length, 64);
assert.equal(meta.license, 'BSD-3-Clause');
assert.equal(meta.provenance.commit, source.commit);
assert.equal(meta.provenance.source_sha256, source.sha256);
assert.equal(meta.provenance.source_bytes, source.bytes);
assert.ok(meta.runtime_triangles > 10000 && meta.runtime_triangles < meta.source_triangles, 'A real triangle reduction');
assert.ok(meta.bytes < 400 * 1024, 'Compact derivative stays inside the model budget');
assert.deepEqual(meta.compression, ['KHR_mesh_quantization', 'EXT_meshopt_compression']);
const jlen = glb.readUInt32LE(12), doc = JSON.parse(glb.subarray(20, 20 + jlen));
const names = doc.nodes.map(n => n.name || '');
assert.ok(meta.joints.every(j => names.includes(j)), 'All named joints survive the export');
assert.ok(!doc.images?.length && !doc.textures?.length, 'No textures or remote images');
assert.ok(doc.buffers.every(b => !b.uri), 'Geometry is embedded');
assert.ok(doc.extensionsRequired.includes('EXT_meshopt_compression'), 'Runtime needs the meshopt decoder');
for (const file of ['ui/3d/system-world/spot-LICENSE.txt', 'licences/texts/system-world-spot-BSD-3-Clause.txt']) {
  const text = fs.readFileSync(file, 'utf8');
  assert.ok(text.includes('BSD 3-Clause License') && text.includes(source.copyright.replace('2021, ', '')), 'Licence text: ' + file);
}
const licence = fs.readFileSync('ui/3d/system-world/spot-LICENSE.txt', 'utf8');
assert.equal(licence, fs.readFileSync('licences/texts/system-world-spot-BSD-3-Clause.txt', 'utf8'), 'Licence copies stay identical');
const size = dir => fs.readdirSync(dir, { withFileTypes: true }).reduce((total, f) => {
  const p = dir + '/' + f.name;
  return total + (f.isDirectory() ? size(p) : fs.statSync(p).size);
}, 0);
assert.ok(size('ui/3d/system-world') < 40 * 1024 * 1024, 'Complete model directory stays below 40 MiB');
console.log('System World companion: bounded quay patrol, diagonal trot, BSD-3-Clause provenance, meshopt GLB and model budget verified');
