// Reads the physics meshes straight from a course GLB (nodes carrying the glTF
// extra `aurago_collider`) so the picture and the collision share one source.
// No DOM, three.js or Rapier: it runs in Node tests. The vertex arithmetic
// mirrors `glb_fixed_colliders` in assets/minigolf/kit/export.py term by term,
// so the Python digest and `colliderDigest()` agree bit for bit.
export const COLLIDER_SURFACES = Object.freeze(['grass', 'sand', 'wood', 'stone', 'metal']);

const MAGIC = 0x46546C67, JSON_CHUNK = 0x4E4F534A, BIN_CHUNK = 0x004E4942, TRIANGLES = 4;
const COMPONENTS = { 5121: ['getUint8', 1], 5123: ['getUint16', 2], 5125: ['getUint32', 4], 5126: ['getFloat32', 4] };
const bad = message => new TypeError('Invalid GLB: ' + message);

export function parseGlb(buffer) {
    const bytes = ArrayBuffer.isView(buffer) ? new Uint8Array(buffer.buffer, buffer.byteOffset, buffer.byteLength) : new Uint8Array(buffer);
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    if (view.byteLength < 20 || view.getUint32(0, true) !== MAGIC) throw bad('bad header');
    let offset = 12, json = null, bin = null;
    while (offset + 8 <= view.byteLength) {
        const length = view.getUint32(offset, true), kind = view.getUint32(offset + 4, true), start = offset + 8;
        if (start + length > view.byteLength) throw bad('truncated chunk');
        if (kind === JSON_CHUNK && !json) {
            try { json = JSON.parse(new TextDecoder().decode(bytes.subarray(start, start + length)).replace(/[\0 ]+$/, '')); } catch { throw bad('unreadable JSON chunk'); }
        } else if (kind === BIN_CHUNK && !bin) bin = bytes.subarray(start, start + length);
        offset = start + length;
    }
    if (!json || !bin) throw bad('missing chunks');
    return { json, bin };
}

function localTransform(node) {
    const list = (value, length, fallback) => {
        if (value === undefined) return fallback;
        if (!Array.isArray(value) || value.length !== length || !value.every(Number.isFinite)) throw bad('node transform');
        return value;
    };
    if (node.matrix !== undefined) return { matrix: list(node.matrix, 16) };
    return { scale: list(node.scale, 3, [1, 1, 1]), rotation: list(node.rotation, 4, [0, 0, 0, 1]), translation: list(node.translation, 3, [0, 0, 0]) };
}

// Same term order as the Python reader (left-to-right sums), in Float64; `p` is updated in place.
function applyLocal(local, p) {
    let x = p[0], y = p[1], z = p[2];
    if (local.matrix) {
        const m = local.matrix;
        p[0] = m[0] * x + m[4] * y + m[8] * z + m[12];
        p[1] = m[1] * x + m[5] * y + m[9] * z + m[13];
        p[2] = m[2] * x + m[6] * y + m[10] * z + m[14];
        return;
    }
    const { scale: s, rotation: [qx, qy, qz, qw], translation: t } = local;
    x *= s[0]; y *= s[1]; z *= s[2];
    const tx = 2 * (qy * z - qz * y), ty = 2 * (qz * x - qx * z), tz = 2 * (qx * y - qy * x);
    p[0] = x + qw * tx + qy * tz - qz * ty + t[0];
    p[1] = y + qw * ty + qz * tx - qx * tz + t[1];
    p[2] = z + qw * tz + qx * ty - qy * tx + t[2];
}

function reader(json, bin) {
    const data = new DataView(bin.buffer, bin.byteOffset, bin.byteLength);
    return (index, type, components, allowed) => {
        const accessor = json.accessors?.[index], view = json.bufferViews?.[accessor?.bufferView], kind = COMPONENTS[accessor?.componentType];
        if (!accessor || accessor.sparse || accessor.type !== type || !kind || !allowed.includes(accessor.componentType) || !view || (view.buffer ?? 0) !== 0) throw bad('accessor');
        const [method, size] = kind, count = accessor.count, stride = view.byteStride || components * size;
        const start = (view.byteOffset || 0) + (accessor.byteOffset || 0);
        if (!Number.isInteger(count) || count < 0 || start < 0 || (count && start + (count - 1) * stride + components * size > bin.byteLength)) throw bad('accessor range');
        const out = accessor.componentType === 5126 ? new Float32Array(count * components) : new Uint32Array(count * components);
        for (let i = 0; i < count; i++) for (let c = 0; c < components; c++) out[i * components + c] = data[method](start + i * stride + c * size, true);
        return out;
    };
}

// Reads every collider node, welds bit-identical world vertices per surface and drops
// triangles that collapse. Returns `[{ type: 'mesh', surface, vertices, indices, nodes }]`
// in COLLIDER_SURFACES order; surfaces without triangles are left out.
export function readColliders(buffer) {
    const { json, bin } = parseGlb(buffer), nodes = json.nodes || [], read = reader(json, bin);
    const parents = new Array(nodes.length).fill(-1);
    nodes.forEach((node, index) => { for (const child of node.children || []) if (child >= 0 && child < nodes.length) parents[child] = index; });
    const groups = new Map(COLLIDER_SURFACES.map(surface => [surface, { surface, nodes: 0, keys: new Map(), vertices: [], indices: [] }]));
    const bits = new Uint32Array(3), floats = new Float32Array(bits.buffer), point = [0, 0, 0];
    nodes.forEach((node, nodeIndex) => {
        const surface = node.extras?.aurago_collider;
        if (surface === undefined) return;
        const group = groups.get(surface);
        if (!group) throw new TypeError('Unknown collider surface: ' + String(surface));
        if (node.mesh === undefined) return;
        const chain = [];
        for (let current = nodeIndex; current >= 0; current = parents[current]) {
            if (chain.length > nodes.length) throw bad('node cycle');
            chain.push(localTransform(nodes[current]));
        }
        const mesh = json.meshes?.[node.mesh];
        if (!mesh) throw bad('mesh index');
        group.nodes++;
        for (const primitive of mesh.primitives || []) {
            if ((primitive.mode ?? TRIANGLES) !== TRIANGLES) continue;
            const positions = read(primitive.attributes?.POSITION, 'VEC3', 3, [5126]), count = positions.length / 3;
            const indices = primitive.indices === undefined ? Uint32Array.from({ length: count }, (_, i) => i) : read(primitive.indices, 'SCALAR', 1, [5121, 5123, 5125]);
            if (indices.length % 3 || indices.some(index => index >= count)) throw bad('triangle indices');
            // World position of each vertex, rounded to Float32 once; the welding key is its bit pattern.
            const world = new Uint32Array(count * 3), worldBits = new Float32Array(world.buffer);
            for (let i = 0; i < count; i++) {
                point[0] = positions[i * 3]; point[1] = positions[i * 3 + 1]; point[2] = positions[i * 3 + 2];
                for (const local of chain) applyLocal(local, point);
                floats[0] = point[0]; floats[1] = point[1]; floats[2] = point[2];
                if (!floats.every(Number.isFinite)) throw bad('non-finite vertex');
                world.set(bits, i * 3);
            }
            const ids = new Int32Array(count).fill(-1);
            const weld = i => {
                if (ids[i] >= 0) return ids[i];
                const key = world[i * 3] + ',' + world[i * 3 + 1] + ',' + world[i * 3 + 2];
                let id = group.keys.get(key);
                if (id === undefined) { id = group.keys.size; group.keys.set(key, id); group.vertices.push(worldBits[i * 3], worldBits[i * 3 + 1], worldBits[i * 3 + 2]); }
                return ids[i] = id;
            };
            for (let i = 0; i < indices.length; i += 3) {
                const a = weld(indices[i]), b = weld(indices[i + 1]), c = weld(indices[i + 2]);
                if (a !== b && b !== c && a !== c) group.indices.push(a, b, c);
            }
        }
    });
    return [...groups.values()].filter(group => group.indices.length)
        .map(group => ({ type: 'mesh', surface: group.surface, vertices: Float32Array.from(group.vertices), indices: Uint32Array.from(group.indices), nodes: group.nodes }));
}

// SHA-256 over the groups in surface order: name, 0x00, vertex count, index count
// (Uint32 LE), Float32 LE vertices, Uint32 LE indices. Empty groups are skipped.
export async function colliderDigest(groups) {
    const order = group => COLLIDER_SURFACES.indexOf(group.surface);
    const used = [...groups].filter(group => group.indices.length).sort((a, b) => order(a) - order(b));
    const names = used.map(group => { if (order(group) < 0) throw new TypeError('Unknown collider surface: ' + String(group.surface)); return new TextEncoder().encode(group.surface); });
    const total = used.reduce((sum, group, i) => sum + names[i].length + 9 + group.vertices.length * 4 + group.indices.length * 4, 0);
    const bytes = new Uint8Array(total), view = new DataView(bytes.buffer);
    let at = 0;
    used.forEach((group, i) => {
        bytes.set(names[i], at); at += names[i].length + 1;
        view.setUint32(at, group.vertices.length / 3, true); view.setUint32(at + 4, group.indices.length, true); at += 8;
        for (const value of group.vertices) { view.setFloat32(at, value, true); at += 4; }
        for (const index of group.indices) { view.setUint32(at, index, true); at += 4; }
    });
    const hash = new Uint8Array(await globalThis.crypto.subtle.digest('SHA-256', bytes));
    return Array.from(hash, byte => byte.toString(16).padStart(2, '0')).join('');
}
