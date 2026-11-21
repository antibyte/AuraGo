// AuraPinball ramps and habitrails (plan 8 / T13, R2): frames along the paths, the sweep helper and the buildRamps
// lifecycle. The meshes (wire habitrail, clear plastic ramp, tube, flap, standoffs, signs) live in ramp-builders.js.
// pathFrames() is pure (plain arrays) so Node tests can import this module.
import * as THREE from 'three';
import { rampContext, plasticRamp, wireRamp, tubeRamp } from './ramp-builders.js';

// Orthonormal, continuous frames along a path (three coordinates: X = x, Y = h, Z = -y).
// Returns {count, p, t, r, u}: Float64Arrays of 3 * count. t = tangent, r = lateral (right of travel), u = up.
export function pathFrames(path) {
    const s = path.samples || path.points || [], n = s.length;
    const p = new Float64Array(3 * n), t = new Float64Array(3 * n), r = new Float64Array(3 * n), u = new Float64Array(3 * n);
    for (let i = 0; i < n; i++) { p[3 * i] = s[i][0]; p[3 * i + 1] = s[i][2] || 0; p[3 * i + 2] = -s[i][1]; }
    let pr = [1, 0, 0];
    for (let i = 0; i < n; i++) {
        const a = Math.max(0, i - 1), b = Math.min(n - 1, i + 1);
        let tx = p[3 * b] - p[3 * a], ty = p[3 * b + 1] - p[3 * a + 1], tz = p[3 * b + 2] - p[3 * a + 2];
        let l = Math.hypot(tx, ty, tz);
        if (l < 1e-9) { tx = i ? t[3 * i - 3] : 0; ty = i ? t[3 * i - 2] : 0; tz = i ? t[3 * i - 1] : -1; l = 1; }
        tx /= l; ty /= l; tz /= l;
        // r = t x worldUp = (-tz, 0, tx); near-vertical tangents carry the previous lateral axis (parallel transport).
        let rx = -tz, ry = 0, rz = tx, rl = Math.hypot(rx, rz);
        if (rl < 0.05) {
            const d = pr[0] * tx + pr[1] * ty + pr[2] * tz;
            rx = pr[0] - d * tx; ry = pr[1] - d * ty; rz = pr[2] - d * tz; rl = Math.hypot(rx, ry, rz) || 1;
        } else rl = Math.hypot(rx, ry, rz);
        rx /= rl; ry /= rl; rz /= rl; pr = [rx, ry, rz];
        const ux = ry * tz - rz * ty, uy = rz * tx - rx * tz, uz = rx * ty - ry * tx;
        t[3 * i] = tx; t[3 * i + 1] = ty; t[3 * i + 2] = tz;
        r[3 * i] = rx; r[3 * i + 1] = ry; r[3 * i + 2] = rz;
        u[3 * i] = ux; u[3 * i + 1] = uy; u[3 * i + 2] = uz;
    }
    return { count: n, p, t, r, u };
}

// Point on the frame i: origin + r*a + u*b.
export function at(F, i, a, b, out) {
    out.set(F.p[3 * i] + F.r[3 * i] * a + F.u[3 * i] * b, F.p[3 * i + 1] + F.r[3 * i + 1] * a + F.u[3 * i + 1] * b, F.p[3 * i + 2] + F.r[3 * i + 2] * a + F.u[3 * i + 2] * b);
    return out;
}

export function indicesEvery(count, step) {
    const out = [];
    for (let i = 0; i < count; i += step) out.push(i);
    if (out[out.length - 1] !== count - 1) out.push(count - 1);
    return out;
}

// Sweep a (r, u) profile along the frames; flat-shaded non-indexed triangles.
export function sweep(F, idx, profile, closed = true) {
    const pos = [], rings = idx.map(i => profile.map(([a, b]) => at(F, i, a, b, new THREE.Vector3())));
    const m = profile.length, edges = closed ? m : m - 1;
    for (let k = 0; k < idx.length - 1; k++) {
        for (let e = 0; e < edges; e++) {
            const e2 = (e + 1) % m, a = rings[k][e], b = rings[k][e2], c = rings[k + 1][e2], d = rings[k + 1][e];
            for (const q of [a, b, c, a, c, d]) pos.push(q.x, q.y, q.z);
        }
    }
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
    g.computeVertexNormals();
    return g;
}

// Builds every layout path. `paths` are physics debug entries ({id, kind, color, width, samples, entry}); `ctx` is
// {mats, low, env, tier?, layout?, images?}. The layout supplies what debug() does not carry (minSpeed, skin) and the
// walls and posts for the standoff test; the plastics sheet image supplies the sign art. Without `layout` no entrance
// signs are built. A path with skin 'telescope' gets no mesh at all (a model covers it).
// Returns {group, update(snap, dt), dispose()}.
export function buildRamps(paths, ctx) {
    const group = new THREE.Group(); group.name = 'ramps';
    const own = { g: [], m: [], t: [] };
    const D = { g: x => { if (x) own.g.push(x); return x; }, m: x => { own.m.push(x); return x; }, t: x => { own.t.push(x); return x; } };
    const layout = ctx.layout || (ctx.host && ctx.host.layout) || null, images = ctx.images || (ctx.host && ctx.host.images) || {};
    const defs = new Map(((layout && layout.paths) || []).map(d => [d.id, d]));
    const rc = rampContext(ctx, D, layout, images), entries = [];
    for (const def of paths) {
        const src = defs.get(def.id) || def;
        if (src.skin === 'telescope') continue;
        const F = pathFrames(def);
        if (F.count < 2) continue;
        const sign = !!layout && !(+src.minSpeed >= 99);   // only ramps a ball enters from the field carry a sign
        let entry = null;
        if (def.kind === 'plastic') entry = plasticRamp(def, F, rc, group, sign);
        else if (def.kind === 'tube') tubeRamp(def, F, rc, group);
        else entry = wireRamp(def, F, rc, group, sign);
        if (entry) entries.push(entry);
    }
    return {
        group,
        update(snap, dt) {
            for (const f of entries) {
                let near = 0, riding = 0;
                if (snap && snap.balls) for (const b of snap.balls) {
                    if (b.hidden) continue;
                    const dx = b.x - f.hinge.x, dy = -b.y - f.hinge.z;
                    if (dx * dx + dy * dy < 0.045 * 0.045) near = 1;
                    if (b.mode === 'path' && (b.h || 0) > 0.008) {
                        for (let i = 0; i < f.F.count; i += 4) {
                            const rx = b.x - f.F.p[3 * i], rz = -b.y - f.F.p[3 * i + 2];
                            if (rx * rx + rz * rz < 0.042 * 0.042) { riding = 1; break; }
                        }
                    }
                }
                f.state += ((near ? 0 : 1) - f.state) * Math.min(1, dt * 14);
                if (f.flap) f.flap.rotation.x = 0.38 * f.state;
                f.pulse = riding ? Math.min(1, f.pulse + dt * 10) : Math.max(0, f.pulse - dt * 3.5);
                if (f.glowMat) f.glowMat.color.copy(f.baseColor).multiplyScalar(1.9 + 1.5 * f.pulse);
                if (f.signMat) f.signMat.emissiveIntensity = f.signBase + 1.4 * f.pulse;
            }
        },
        dispose() {
            own.g.forEach(g => g.dispose()); own.m.forEach(m => m.dispose()); own.t.forEach(t => t.dispose());
            group.traverse(o => { if (o.isInstancedMesh) o.dispose(); });
        }
    };
}
