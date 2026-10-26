import assert from 'node:assert/strict';
import { COLLIDER_SURFACES, colliderDigest, parseGlb, readColliders } from '../ui/js/desktop/apps/minigolf-collision.js';

const pad4 = length => (4 - length % 4) % 4;
const f32 = values => { const out = new Uint8Array(values.length * 4), view = new DataView(out.buffer); values.forEach((value, i) => view.setFloat32(i * 4, value, true)); return out; };
const u16 = values => { const out = new Uint8Array(values.length * 2), view = new DataView(out.buffer); values.forEach((value, i) => view.setUint16(i * 2, value, true)); return out; };
const concat = parts => { const out = new Uint8Array(parts.reduce((sum, part) => sum + part.length, 0)); let at = 0; for (const part of parts) { out.set(part, at); at += part.length; } return out; };

// Hand-built GLB: parent chain (TRS), matrix node, ignored node, rail with stride and byte indices.
function buildGlb({ magic = 0x46546C67, withBin = true, surface = 'wood' } = {}) {
    const q = Math.SQRT1_2; // 90 degrees about +Y
    const views = [], accessors = [];
    let blob = new Uint8Array(0);
    const add = (bytes, extra = {}) => {
        blob = concat([blob, new Uint8Array(pad4(blob.length))]);
        views.push({ buffer: 0, byteOffset: blob.length, byteLength: bytes.length, ...extra });
        blob = concat([blob, bytes]);
        return views.length - 1;
    };
    const vec3 = (bytes, count, extra) => { accessors.push({ bufferView: add(bytes, extra), componentType: 5126, count, type: 'VEC3' }); return accessors.length - 1; };
    const scalar = (bytes, count, componentType) => { accessors.push({ bufferView: add(bytes), componentType, count, type: 'SCALAR' }); return accessors.length - 1; };
    // Mesh 0: square plus a vertex duplicating corner 0 for a degenerate triangle (local to node 1).
    const square = [1, 0.25, 1, 2, 0.25, 1, 2, 0.25, 2, 1, 0.25, 2, 1, 0.25, 1];
    const m0 = { attributes: { POSITION: vec3(f32(square), 5) }, indices: scalar(u16([0, 1, 2, 0, 2, 3, 0, 4, 1]), 9, 5123) };
    // Mesh 1: one triangle without indices, local to node 2 (scale 2, translation (1, 0, -1)).
    const m1 = { attributes: { POSITION: vec3(f32([0.5, 0.125, 0, 0.5, 0.125, -0.5, 0, 0.125, -0.25]), 3) } };
    const m2 = { attributes: { POSITION: vec3(f32([0, 0, 0, 5, 0, 0, 5, 5, 0]), 3) } };
    // Mesh 3: interleaved positions (stride 16) and byte indices.
    const rail = [0, 0, 0, 9, 0, 1, 0, 9, 0.5, 0, 0, 9];
    const m3 = { attributes: { POSITION: vec3(f32(rail), 3, { byteStride: 16 }) }, indices: scalar(Uint8Array.from([0, 1, 2]), 3, 5121) };
    const nodes = [
        { name: 'parent', translation: [1, 0, 0], rotation: [0, q, 0, q], children: [1] },
        { name: 'floor-felt', mesh: 0, extras: { aurago_collider: 'grass' } },
        { name: 'floor-extra', mesh: 1, matrix: [2, 0, 0, 0, 0, 2, 0, 0, 0, 0, 2, 0, 1, 0, -1, 1], extras: { aurago_collider: 'grass' } },
        { name: 'decor', mesh: 2 },
        { name: 'rail-wood', mesh: 3, translation: [0, 1, 5], extras: { aurago_collider: surface, aurago_hidden: true } }
    ];
    const json = { asset: { version: '2.0' }, scene: 0, scenes: [{ nodes: [0, 2, 3, 4] }], nodes, meshes: [m0, m1, m2, m3].map(primitive => ({ primitives: [primitive] })), accessors, bufferViews: views, buffers: [{ byteLength: blob.length }] };
    const jsonBytes = new TextEncoder().encode(JSON.stringify(json));
    const jsonChunk = concat([jsonBytes, new Uint8Array(pad4(jsonBytes.length)).fill(0x20)]);
    const binChunk = concat([blob, new Uint8Array(pad4(blob.length))]);
    const chunk = (kind, bytes) => { const head = new DataView(new ArrayBuffer(8)); head.setUint32(0, bytes.length, true); head.setUint32(4, kind, true); return concat([new Uint8Array(head.buffer), bytes]); };
    const body = concat([chunk(0x4E4F534A, jsonChunk), ...(withBin ? [chunk(0x004E4942, binChunk)] : [])]);
    const header = new DataView(new ArrayBuffer(12));
    header.setUint32(0, magic, true); header.setUint32(4, 2, true); header.setUint32(8, 12 + body.length, true);
    const bytes = concat([new Uint8Array(header.buffer), body]);
    return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
}

assert.deepEqual([...COLLIDER_SURFACES], ['grass', 'sand', 'wood', 'stone', 'metal']);

const groups = readColliders(buildGlb());
assert.deepEqual(groups.map(group => group.surface), ['grass', 'wood'], 'groups follow the surface order');
assert.ok(groups.every(group => group.type === 'mesh' && group.vertices instanceof Float32Array && group.indices instanceof Uint32Array), 'typed arrays');
const [grass, wood] = groups;
assert.deepEqual([grass.nodes, wood.nodes], [2, 1], 'collider nodes per surface (the unmarked node is ignored)');

// Parent TRS: rotate 90 degrees about +Y, then translate by (1, 0, 0): local (x, y, z) -> (z + 1, y, -x).
assert.deepEqual([...grass.vertices.slice(0, 3)], [2, 0.25, -1], 'first vertex in world space');
assert.deepEqual([...grass.vertices.slice(3, 6)], [2, 0.25, -2]);
assert.deepEqual([...grass.vertices.slice(6, 9)], [3, 0.25, -2]);
assert.deepEqual([...grass.vertices.slice(9, 12)], [3, 0.25, -1]);
// Matrix node: two vertices are bit-identical with the first mesh, so only the third is new (4 + 1, not 4 + 3).
assert.equal(grass.vertices.length / 3, 5, 'welded vertices');
assert.deepEqual([...grass.vertices.slice(12, 15)], [1, 0.25, -1.5]);
// The degenerate triangle (two vertices welded together) is gone: 2 + 1 triangles remain.
assert.deepEqual([...grass.indices], [0, 1, 2, 0, 2, 3, 0, 1, 4]);

assert.deepEqual([...wood.vertices], [0, 1, 5, 0, 2, 5, 0.5, 1, 5], 'stride and byte indices');
assert.deepEqual([...wood.indices], [0, 1, 2]);

const digest = await colliderDigest(groups);
assert.match(digest, /^[0-9a-f]{64}$/);
assert.equal(digest, await colliderDigest(readColliders(buildGlb())), 'digest is stable');
// The same bytes hashed with Python's hashlib/struct per contract V1 (hand-derived groups).
assert.equal(digest, '2cf0cc79a6e255fe3e33bffec976abb4b057a96b56a08e5f8af9fa2db2327111', 'digest value');
assert.equal(await colliderDigest([wood, grass]), digest, 'the digest orders the groups itself');
assert.equal(await colliderDigest(groups.map(group => ({ ...group, vertices: Array.from(group.vertices), indices: Array.from(group.indices) }))), digest, 'plain arrays digest the same');

// The digest hashes Float32 bit patterns, so the sign of zero matters.
const signed = [{ type: 'mesh', surface: 'grass', vertices: Float32Array.of(0, 0, 0, 1, 0, 0, 0, 1, 0), indices: Uint32Array.of(0, 1, 2) }];
assert.notEqual(await colliderDigest(signed), await colliderDigest([{ ...signed[0], vertices: Float32Array.of(-0, 0, 0, 1, 0, 0, 0, 1, 0) }]), 'the digest sees the sign of zero');

assert.equal(parseGlb(buildGlb()).json.nodes.length, 5);
assert.throws(() => readColliders(buildGlb({ magic: 0x12345678 })), TypeError, 'wrong magic');
assert.throws(() => readColliders(buildGlb({ withBin: false })), TypeError, 'missing BIN chunk');
assert.throws(() => readColliders(buildGlb({ surface: 'lava' })), TypeError, 'unknown surface');
assert.throws(() => readColliders(new ArrayBuffer(8)), TypeError, 'truncated file');

console.log('minigolf collision: all checks passed');
