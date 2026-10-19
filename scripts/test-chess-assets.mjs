import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import validator from 'gltf-validator';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const assetRoot = path.join(root, 'ui/3d/chess/v1');
const manifest = JSON.parse((await readFile(path.join(assetRoot, 'manifest.json'))).toString('utf8'));

assert.equal(manifest.units, 'metres', 'chess pieces must be authored in metres');
assert.equal(manifest.up_axis, 'Y', 'chess pieces must be exported Y-up');
assert.equal(manifest.license, 'MIT');
assert.ok(manifest.square_size > 0, 'manifest must declare the board square size');

const names = ['pawn', 'knight', 'bishop', 'rook', 'queen', 'king'];
assert.deepEqual(Object.keys(manifest.pieces).sort(), names.slice().sort());

function parseGLB(bytes) {
  assert.equal(bytes.toString('ascii', 0, 4), 'glTF');
  assert.equal(bytes.readUInt32LE(4), 2);
  assert.equal(bytes.readUInt32LE(8), bytes.length);
  let offset = 12;
  let json = null;
  let binary = null;
  while (offset < bytes.length) {
    const length = bytes.readUInt32LE(offset);
    const type = bytes.readUInt32LE(offset + 4);
    offset += 8;
    const chunk = bytes.subarray(offset, offset + length);
    if (type === 0x4e4f534a) json = JSON.parse(chunk.toString('utf8').replace(/[\u0000 ]+$/, ''));
    if (type === 0x004e4942) binary = chunk;
    offset += length;
  }
  assert.ok(json && binary, 'GLB must contain embedded JSON and binary chunks');
  assert.ok(json.buffers.every(buffer => !buffer.uri), 'GLB buffers must stay embedded');
  assert.ok((json.images ?? []).every(image => !image.uri), 'GLB images must stay embedded');
  return { json, binary };
}

function positionBounds(json, binary) {
  const points = [];
  for (const mesh of json.meshes ?? []) {
    for (const primitive of mesh.primitives) {
      const accessor = json.accessors[primitive.attributes.POSITION];
      assert.ok(accessor, 'every primitive needs a POSITION accessor');
      assert.ok(accessor.min && accessor.max, 'POSITION accessors must carry min/max bounds');
      const view = json.bufferViews[accessor.bufferView];
      const stride = view.byteStride ?? 12;
      const start = (view.byteOffset ?? 0) + (accessor.byteOffset ?? 0);
      for (let index = 0; index < accessor.count; index += 1) {
        const at = start + index * stride;
        points.push([binary.readFloatLE(at), binary.readFloatLE(at + 4), binary.readFloatLE(at + 8)]);
      }
    }
  }
  assert.ok(points.length, 'chess pieces must carry vertex positions');
  const min = [Infinity, Infinity, Infinity];
  const max = [-Infinity, -Infinity, -Infinity];
  let radius = 0;
  for (const point of points) {
    for (let axis = 0; axis < 3; axis += 1) {
      min[axis] = Math.min(min[axis], point[axis]);
      max[axis] = Math.max(max[axis], point[axis]);
    }
    radius = Math.max(radius, Math.hypot(point[0], point[2]));
  }
  return { min, max, radius };
}

let total = 0;
for (const name of names) {
  const entry = manifest.pieces[name];
  assert.equal(entry.file, `${name}.glb`);
  const bytes = await readFile(path.join(assetRoot, entry.file));
  assert.equal(bytes.length, entry.bytes, `${name}: manifest byte count mismatch`);
  assert.equal(createHash('sha256').update(bytes).digest('hex'), entry.sha256, `${name}: manifest hash mismatch`);

  const report = await validator.validateBytes(new Uint8Array(bytes), { uri: entry.file });
  assert.equal(report.issues.numErrors, 0, `${name}: ${JSON.stringify(report.issues.messages)}`);

  const { json, binary } = parseGLB(bytes);
  const { min, max, radius } = positionBounds(json, binary);
  assert.ok(Math.abs(min[1]) < 0.0005, `${name}: piece base must sit on the ground plane`);
  const height = max[1] - min[1];
  assert.ok(Math.abs(height - entry.height) < 0.001, `${name}: manifest height ${entry.height} != ${height}`);
  assert.ok(Math.abs(radius - entry.radius) < 0.001, `${name}: manifest radius ${entry.radius} != ${radius}`);
  assert.ok(radius * 2 < manifest.square_size, `${name}: base must stay inside one board square`);
  assert.ok(height < manifest.square_size * 2.2, `${name}: piece must stay in readable proportion to the board`);
  total += bytes.length;
}

assert.ok(total <= manifest.budget_bytes, `chess 3D pack is ${total} bytes, over the ${manifest.budget_bytes} byte budget`);
await readFile(path.join(assetRoot, 'LICENSE.txt'));

console.log(`chess 3D assets ok: ${names.length} pieces, ${total} bytes, budget ${manifest.budget_bytes}`);
