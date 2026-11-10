import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { MeshoptDecoder } from 'three/addons/libs/meshopt_decoder.module.js';

const base = 'ui/3d/system-world/v3';
const manifest = JSON.parse(await fs.readFile(base + '/manifest.json', 'utf8'));
async function size(dir) { let total = 0; for (const f of await fs.readdir(dir, { withFileTypes: true })) { const p = path.join(dir, f.name); total += f.isDirectory() ? await size(p) : (await fs.stat(p)).size; } return total; }

assert.equal(manifest.schema_version, 3);
assert.equal(manifest.compression, 'EXT_meshopt_compression');
assert.equal(manifest.validator_errors, 0);
assert.ok(manifest.assets.length >= 6, 'Kit v3 ships at least six designs');
const ids = manifest.assets.map(a => a.id);
for (const id of ['street-plain', 'street-drain', 'street-zebra', 'street-fill', 'street-crossing', 'street-end'])
  assert.ok(ids.includes(id), 'Missing street design ' + id);
assert.equal(new Set(ids).size, ids.length, 'Design ids are unique');

let files = 0, bytes = 0;
const listed = new Set();
for (const asset of manifest.assets) {
  assert.equal(asset.lods.length, 3, asset.id + ' has exactly three LODs');
  assert.deepEqual(asset.lods.map(l => l.level), [0, 1, 2]);
  let triangles = Infinity;
  for (const lod of asset.lods) {
    assert.match(lod.file, /^[a-z0-9-]+\.lod[0-2]\.glb$/);
    assert.ok(lod.triangles <= triangles, lod.file + ' triangles must not increase with the level');
    triangles = lod.triangles;
    const data = await fs.readFile(path.join(base, lod.file));
    listed.add(lod.file); files++; bytes += data.length;
    assert.equal(data.length, lod.bytes, lod.file + ' size');
    assert.equal(createHash('sha256').update(data).digest('hex'), lod.sha256, lod.file + ' sha256');
    assert.equal(data.readUInt32LE(0), 0x46546c67, lod.file + ' is a GLB');
    const doc = JSON.parse(data.subarray(20, 20 + data.readUInt32LE(12)).toString('utf8'));
    assert.ok(doc.extensionsRequired?.includes('EXT_meshopt_compression'), lod.file + ' requires meshopt');
    assert.ok(!doc.images?.length && !doc.textures?.length, lod.file + ' has no textures');
    assert.ok(doc.buffers.every(b => !b.uri), lod.file + ' has no external buffer');
    for (const mesh of doc.meshes) for (const prim of mesh.primitives)
      assert.notEqual(prim.attributes.COLOR_0, undefined, lod.file + ' primitive has COLOR_0');
    for (const material of doc.materials) assert.ok(material.name.startsWith('city.'), lod.file + ' material ' + material.name);
    const nodes = new Set(doc.nodes.map(n => n.name));
    for (const component of asset.components ?? []) assert.ok(nodes.has(component), lod.file + ' component ' + component);
  }
}
assert.equal(manifest.total_model_bytes, bytes);
for (const name of await fs.readdir(base)) if (name.endsWith('.glb')) assert.ok(listed.has(name), 'Unlisted GLB ' + name);
await fs.access(base + '/LICENSE.txt');
const model = await size('ui/3d/system-world');
assert.ok(model < 40 * 1024 * 1024, 'Complete model directory exceeds 40 MiB');
// Window panes (city.window): TEXCOORD_0 holds room coordinates that must survive packing.
// After undoing glTF's flipped V, u advances one unit per bay (0.8-6 m) left to right seen from
// outside, v one unit per storey (1.2-6 m) upwards, and every pane corner sits on the room grid.
await MeshoptDecoder.ready;
const loader = new GLTFLoader().setMeshoptDecoder(MeshoptDecoder);
const windowDesigns = new Set();
let paneTriangles = 0;
const a = new THREE.Vector3(), b = new THREE.Vector3(), c = new THREE.Vector3(), n = new THREE.Vector3();
for (const asset of manifest.assets) for (const lod of asset.lods) {
  const data = await fs.readFile(path.join(base, lod.file));
  const doc = JSON.parse(data.subarray(20, 20 + data.readUInt32LE(12)).toString('utf8'));
  if (!(doc.materials || []).some(m => m.name === 'city.window')) continue;
  windowDesigns.add(asset.id);
  const gltf = await loader.parseAsync(data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength), '');
  gltf.scene.updateMatrixWorld(true);
  gltf.scene.traverse(mesh => {
    if (!mesh.isMesh || mesh.material.name !== 'city.window') return;
    const g = mesh.geometry, pos = g.getAttribute('position'), uv = g.getAttribute('uv'), nor = g.getAttribute('normal'), index = g.getIndex();
    assert.ok(uv && nor && g.getAttribute('color'), lod.file + ': window panes need TEXCOORD_0, NORMAL and COLOR_0');
    const normalMatrix = new THREE.Matrix3().getNormalMatrix(mesh.matrixWorld);
    const room = i => [uv.getX(i), 1 - uv.getY(i)];
    for (let i = 0; i < uv.count; i++) for (const value of room(i))
      assert.ok(Math.abs(value - Math.round(value)) < .01, lod.file + ': pane corner off the room grid ' + room(i));
    let checked = 0;
    const count = index ? index.count : pos.count;
    for (let t = 0; t < count; t += 3) {
      const [i0, i1, i2] = [t, t + 1, t + 2].map(k => index ? index.getX(k) : k);
      a.fromBufferAttribute(pos, i0).applyMatrix4(mesh.matrixWorld);
      b.fromBufferAttribute(pos, i1).applyMatrix4(mesh.matrixWorld);
      c.fromBufferAttribute(pos, i2).applyMatrix4(mesh.matrixWorld);
      const [r0, r1, r2] = [room(i0), room(i1), room(i2)];
      const d1 = [r1[0] - r0[0], r1[1] - r0[1]], d2 = [r2[0] - r0[0], r2[1] - r0[1]], det = d1[0] * d2[1] - d1[1] * d2[0];
      if (Math.abs(det) < 1e-6) continue;
      const e1 = b.clone().sub(a), e2 = c.clone().sub(a);
      const pu = e1.clone().multiplyScalar(d2[1]).addScaledVector(e2, -d1[1]).divideScalar(det);
      const pv = e2.clone().multiplyScalar(d1[0]).addScaledVector(e1, -d2[0]).divideScalar(det);
      n.fromBufferAttribute(nor, i0).applyMatrix3(normalMatrix).normalize();
      const where = lod.file + ' pane triangle ' + t / 3;
      assert.ok(pu.length() > .8 && pu.length() < 6, where + ': bay width ' + pu.length().toFixed(2) + ' m');
      assert.ok(pv.length() > 1.2 && pv.length() < 6, where + ': storey height ' + pv.length().toFixed(2) + ' m');
      assert.ok(Math.abs(pu.y) < .05 * pu.length() && pv.y > .98 * pv.length(), where + ': rooms must run along the facade and upwards');
      assert.ok(pu.clone().cross(pv).dot(n) > 0, where + ': u must run left to right seen from outside');
      checked++; paneTriangles++;
    }
    assert.ok(checked > 0, lod.file + ': no pane triangle carries room coordinates');
  });
}
if (windowDesigns.size) assert.match(manifest.pack ?? '', /-vtf\b/, 'Room coordinates need float texture coordinates');
console.log('System World v3 window panes: ' + (windowDesigns.size ? windowDesigns.size + ' designs, ' + paneTriangles + ' pane triangles verified' : 'none shipped yet'));
console.log('System World v3: ' + manifest.assets.length + ' designs, ' + files + ' meshopt GLBs, ' + bytes + ' bytes, model directory ' + model + ' bytes verified');
