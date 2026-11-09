import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';

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
console.log('System World v3: ' + manifest.assets.length + ' designs, ' + files + ' meshopt GLBs, ' + bytes + ' bytes, model directory ' + model + ' bytes verified');
