// Minimal GLB writer for tests: one collider node per mesh, positions and 32-bit indices in one BIN chunk.
// meshes: [{ surface, vertices, indices, ...nodeProperties }]; returns an ArrayBuffer for readColliders().
export function buildColliderGlb(meshes) {
    const parts = [], views = [], accessors = [], nodes = [], gltfMeshes = [];
    let length = 0;
    const push = (typed, componentType, type) => {
        views.push({ buffer: 0, byteOffset: length, byteLength: typed.byteLength });
        accessors.push({ bufferView: views.length - 1, componentType, count: typed.length / (type === 'VEC3' ? 3 : 1), type });
        parts.push(new Uint8Array(typed.buffer, typed.byteOffset, typed.byteLength));
        length += typed.byteLength;
        return accessors.length - 1;
    };
    meshes.forEach(({ surface, vertices, indices, ...node }, i) => {
        gltfMeshes.push({ primitives: [{ attributes: { POSITION: push(Float32Array.from(vertices), 5126, 'VEC3') }, indices: push(Uint32Array.from(indices), 5125, 'SCALAR') }] });
        nodes.push({ name: 'floor-' + i, mesh: i, extras: { aurago_collider: surface }, ...node });
    });
    const json = new TextEncoder().encode(JSON.stringify({ asset: { version: '2.0' }, scenes: [{ nodes: nodes.map((_, i) => i) }], scene: 0, nodes, meshes: gltfMeshes, accessors, bufferViews: views, buffers: [{ byteLength: length }] }));
    const jsonPadded = (json.length + 3 & ~3), bin = new Uint8Array(length);
    let at = 0;
    for (const part of parts) { bin.set(part, at); at += part.length; }
    const out = new Uint8Array(12 + 8 + jsonPadded + 8 + length), view = new DataView(out.buffer);
    view.setUint32(0, 0x46546C67, true); view.setUint32(4, 2, true); view.setUint32(8, out.length, true);
    view.setUint32(12, jsonPadded, true); view.setUint32(16, 0x4E4F534A, true);
    out.fill(0x20, 20, 20 + jsonPadded); out.set(json, 20);
    view.setUint32(20 + jsonPadded, length, true); view.setUint32(24 + jsonPadded, 0x004E4942, true);
    out.set(bin, 28 + jsonPadded);
    return out.buffer;
}
