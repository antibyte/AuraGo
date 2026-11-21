// Glass clearance audit (R2): which built meshes reach the glass. Three coordinates in the space of the roots' parent
// (tableRoot): X = x, Y = h, Z = -y. Plain meshes are checked vertex by vertex (a ramp that rises towards the back must
// not be judged by the glass height at its front edge); instanced meshes use the box of every instance.
import * as THREE from 'three';

const SKIP = new Set(['balls', 'ballShadows', 'simpleBalls']);

const label = o => { for (let n = o; n; n = n.parent) if (n.name) return n.name; return 'mesh'; };

// -> [{ name, top, glass }], one entry per offending mesh: `top` is its highest point above the glass limit, `glass` the
// glass underside height there. A violation is top > glassHeight(y) - margin.
export function glassViolations(roots, { glassHeight, margin = 0.006 }) {
    const out = [], rel = new THREE.Matrix4(), inv = new THREE.Matrix4(), m = new THREE.Matrix4(), v = new THREE.Vector3(), box = new THREE.Box3();
    const worst = { over: -Infinity, top: 0, glass: 0 };
    const test = (y, z) => {
        const g = glassHeight(-z), over = y - (g - margin);
        if (over > worst.over) { worst.over = over; worst.top = y; worst.glass = g; }
    };
    for (const root of roots) {
        if (!root) continue;
        root.updateMatrixWorld(true);
        if (root.parent) inv.copy(root.parent.matrixWorld).invert(); else inv.identity();
        const walk = o => {
            if (!o.visible || SKIP.has(o.name)) return;
            if ((o.isMesh || o.isInstancedMesh) && o.geometry) {
                const geo = o.geometry;
                worst.over = -Infinity;
                rel.multiplyMatrices(inv, o.matrixWorld);
                if (o.isInstancedMesh) {
                    if (!geo.boundingBox) geo.computeBoundingBox();
                    for (let i = 0; i < o.count; i++) {
                        o.getMatrixAt(i, m); m.premultiply(rel);
                        box.copy(geo.boundingBox).applyMatrix4(m);
                        test(box.max.y, box.max.z);
                    }
                } else {
                    const pos = geo.attributes.position;
                    if (pos) for (let i = 0; i < pos.count; i++) { v.fromBufferAttribute(pos, i).applyMatrix4(rel); test(v.y, v.z); }
                }
                if (worst.over > 0) out.push({ name: label(o), top: worst.top, glass: worst.glass });
            }
            for (const c of o.children) walk(c);
        };
        walk(root);
    }
    return out;
}
