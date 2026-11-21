// Minimal GLB reader for the Pinball asset checks: container parsing, node world matrices, mesh bounding boxes, triangle counts.
// Matrices are column-major 4x4 (glTF). No dependencies.

const GLB_MAGIC = 0x46546C67, CHUNK_JSON = 0x4E4F534A, CHUNK_BIN = 0x004E4942;

// bytes: Buffer or Uint8Array -> { version, json, bin }
export function readGlb(bytes) {
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    if (bytes.byteLength < 20 || view.getUint32(0, true) !== GLB_MAGIC) throw new Error('Not a GLB file (magic glTF missing)');
    const version = view.getUint32(4, true), length = view.getUint32(8, true);
    if (version !== 2) throw new Error('GLB version ' + version + ', expected 2');
    if (length > bytes.byteLength) throw new Error('GLB length field exceeds the file');
    let json = null, bin = null;
    for (let at = 12; at + 8 <= length;) {
        const size = view.getUint32(at, true), type = view.getUint32(at + 4, true), start = at + 8;
        if (start + size > length) throw new Error('GLB chunk overruns the file');
        if (type === CHUNK_JSON) json = JSON.parse(new TextDecoder().decode(bytes.subarray(start, start + size)));
        else if (type === CHUNK_BIN && !bin) bin = bytes.subarray(start, start + size);
        at = (start + size + 3) & ~3;
    }
    if (!json) throw new Error('GLB has no JSON chunk');
    return { version, json, bin };
}

export const IDENTITY = Object.freeze([1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]);

export function mulMat(a, b) { // a * b, column-major
    const out = new Array(16).fill(0);
    for (let c = 0; c < 4; c++) for (let r = 0; r < 4; r++) for (let k = 0; k < 4; k++) out[c * 4 + r] += a[k * 4 + r] * b[c * 4 + k];
    return out;
}

function nodeMatrix(node) {
    if (node.matrix) return node.matrix.slice();
    const [qx, qy, qz, qw] = node.rotation || [0, 0, 0, 1], [sx, sy, sz] = node.scale || [1, 1, 1], [tx, ty, tz] = node.translation || [0, 0, 0];
    const xx = qx * qx, yy = qy * qy, zz = qz * qz, xy = qx * qy, xz = qx * qz, yz = qy * qz, wx = qw * qx, wy = qw * qy, wz = qw * qz;
    return [
        (1 - 2 * (yy + zz)) * sx, (2 * (xy + wz)) * sx, (2 * (xz - wy)) * sx, 0,
        (2 * (xy - wz)) * sy, (1 - 2 * (xx + zz)) * sy, (2 * (yz + wx)) * sy, 0,
        (2 * (xz + wy)) * sz, (2 * (yz - wx)) * sz, (1 - 2 * (xx + yy)) * sz, 0,
        tx, ty, tz, 1
    ];
}

// json -> Array of 16-number world matrices, indexed like json.nodes.
export function nodeWorldMatrices(json) {
    const nodes = json.nodes || [], world = new Array(nodes.length), parent = new Array(nodes.length).fill(-1);
    nodes.forEach((n, i) => (n.children || []).forEach(c => { parent[c] = i; }));
    const resolve = (i, depth = 0) => {
        if (world[i]) return world[i];
        if (depth > nodes.length) throw new Error('GLB node hierarchy has a cycle');
        const local = nodeMatrix(nodes[i]);
        world[i] = parent[i] < 0 ? local : mulMat(resolve(parent[i], depth + 1), local);
        return world[i];
    };
    for (let i = 0; i < nodes.length; i++) resolve(i);
    return world;
}

const transformPoint = (m, x, y, z) => [m[0] * x + m[4] * y + m[8] * z + m[12], m[1] * x + m[5] * y + m[9] * z + m[13], m[2] * x + m[6] * y + m[10] * z + m[14]];

// json -> [{ node, name, mesh, min: [x, y, z], max: [x, y, z] }] world-space boxes of every mesh node
// (POSITION accessor min/max, all eight corners through the node's world matrix).
export function meshBoxes(json) {
    const world = nodeWorldMatrices(json), boxes = [];
    (json.nodes || []).forEach((node, i) => {
        if (node.mesh === undefined) return;
        const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
        for (const prim of json.meshes[node.mesh].primitives || []) {
            const acc = json.accessors[prim.attributes.POSITION];
            if (!acc || !acc.min || !acc.max) throw new Error('POSITION accessor without min/max in mesh ' + node.mesh);
            for (let c = 0; c < 8; c++) {
                const p = transformPoint(world[i], c & 1 ? acc.max[0] : acc.min[0], c & 2 ? acc.max[1] : acc.min[1], c & 4 ? acc.max[2] : acc.min[2]);
                for (let k = 0; k < 3; k++) { min[k] = Math.min(min[k], p[k]); max[k] = Math.max(max[k], p[k]); }
            }
        }
        if (min[0] !== Infinity) boxes.push({ node: i, name: node.name || '', mesh: node.mesh, min, max });
    });
    return boxes;
}

// Triangles drawn per node instance (a mesh used by two nodes counts twice).
export function triangleCount(json) {
    let total = 0;
    for (const node of json.nodes || []) {
        if (node.mesh === undefined) continue;
        for (const prim of json.meshes[node.mesh].primitives || []) {
            const mode = prim.mode ?? 4;
            const count = prim.indices !== undefined ? json.accessors[prim.indices].count : json.accessors[prim.attributes.POSITION].count;
            total += mode === 4 ? Math.floor(count / 3) : mode === 5 || mode === 6 ? Math.max(0, count - 2) : 0;
        }
    }
    return total;
}

// Names of the scene's root nodes (the toys), falling back to nodes without a parent.
export function rootNames(json) {
    const nodes = json.nodes || [], scene = json.scenes?.[json.scene ?? 0];
    const ids = scene?.nodes ?? nodes.map((_, i) => i).filter(i => !nodes.some(n => (n.children || []).includes(i)));
    return ids.map(i => nodes[i].name || '');
}
