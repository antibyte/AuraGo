// AuraPinball ramps and habitrails (plan 8 / T13): translucent plastic U-channels with lips and edge glow, chrome wire
// ramps with instanced ribs, glass tubes, entry flaps, support posts and a ramp sign. Static apart from the entry flap.
// pathFrames() is pure (plain arrays) so Node tests can import this module.
import * as THREE from 'three';
import { merge } from './materials.js';

const BALL_R = 0.0135;

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
function at(F, i, a, b, out) {
    out.set(F.p[3 * i] + F.r[3 * i] * a + F.u[3 * i] * b, F.p[3 * i + 1] + F.r[3 * i + 1] * a + F.u[3 * i + 1] * b, F.p[3 * i + 2] + F.r[3 * i + 2] * a + F.u[3 * i + 2] * b);
    return out;
}

function indicesEvery(count, step) {
    const out = [];
    for (let i = 0; i < count; i += step) out.push(i);
    if (out[out.length - 1] !== count - 1) out.push(count - 1);
    return out;
}

// Sweep a (r, u) profile along the frames; flat-shaded non-indexed triangles, plus an end cap fan when `caps`.
function sweep(F, idx, profile, closed = true) {
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

function signTexture(color) {
    const c = document.createElement('canvas'); c.width = 128; c.height = 40;
    const g = c.getContext('2d');
    g.fillStyle = '#05070c'; g.fillRect(0, 0, 128, 40);
    g.strokeStyle = color; g.lineWidth = 4; g.lineJoin = 'round';
    for (let k = 0; k < 3; k++) { const x = 30 + k * 34; g.globalAlpha = 0.45 + k * 0.27; g.beginPath(); g.moveTo(x - 12, 28); g.lineTo(x, 12); g.lineTo(x + 12, 28); g.stroke(); }
    g.globalAlpha = 1; g.strokeStyle = color; g.lineWidth = 3; g.strokeRect(2, 2, 124, 36);
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    return t;
}

// Support posts: thin rods from the ramp underside down to the playfield every ~0.1 m of arc length (merged into the ramp's metal mesh).
function supportGeos(F, lateral, low) {
    const out = [];
    let acc = 0;
    for (let i = 1; i < F.count; i++) {
        acc += Math.hypot(F.p[3 * i] - F.p[3 * i - 3], F.p[3 * i + 1] - F.p[3 * i - 2], F.p[3 * i + 2] - F.p[3 * i - 1]);
        if (acc < (low ? 0.2 : 0.14) || F.p[3 * i + 1] < 0.014) continue;
        acc = 0;
        for (const a of lateral) {
            const s = at(F, i, a, -0.004, new THREE.Vector3()), h = Math.max(0.004, s.y);
            out.push(new THREE.CylinderGeometry(0.0012, 0.0012, h, low ? 5 : 7).translate(s.x, h / 2, s.z));
        }
    }
    return out;
}

// One merged metal mesh per ramp (wires, ribs, supports, struts, collars).
function metalMesh(def, parts, mat, group, D) {
    const geo = merge(parts);
    if (!geo) return;
    D.g(geo);
    const m = new THREE.Mesh(geo, mat); m.name = def.id + '_metal'; m.castShadow = true; m.frustumCulled = false; group.add(m);
}

function plasticRamp(def, F, ctx, group, D) {
    const { mats, low, env } = ctx, w = def.width || 0.05, half = w / 2, lt = 0.0026, th = 0.0025, lh = 0.011;
    const color = new THREE.Color(def.color || '#37d9ff');
    const idx = indicesEvery(F.count, low ? 6 : 2);
    const body = sweep(F, idx, [[-half - lt, -th], [-half - lt, lh], [-half, lh], [-half, 0], [half, 0], [half, lh], [half + lt, lh], [half + lt, -th]]);
    D.g(body);
    const bodyMat = new THREE.MeshPhysicalMaterial({ color: color.clone().multiplyScalar(0.4), roughness: 0.14, metalness: 0, transparent: true, opacity: 0.36, side: THREE.DoubleSide, depthWrite: false,
        envMap: env, envMapIntensity: 1.0, clearcoat: low ? 0 : 0.6, clearcoatRoughness: 0.3, emissive: color.clone().multiplyScalar(0.07), emissiveIntensity: 1 });
    D.m(bodyMat);
    const mesh = new THREE.Mesh(body, bodyMat); mesh.name = def.id + '_channel'; mesh.renderOrder = 2; mesh.frustumCulled = false; group.add(mesh);
    // Edge highlight along both lip tops.
    const e = 0.0008, glowParts = [];
    for (const side of [-1, 1]) {
        const a = side * (half + lt / 2);
        glowParts.push(sweep(F, idx, [[a - e, lh + 0.0004], [a + e, lh + 0.0004], [a + e, lh + 0.0016], [a - e, lh + 0.0016]]));
    }
    const glow = merge(glowParts); D.g(glow);
    const glowMat = new THREE.MeshBasicMaterial({ color: color.clone().multiplyScalar(1.9) }); D.m(glowMat);
    const gm = new THREE.Mesh(glow, glowMat); gm.name = def.id + '_edge'; gm.frustumCulled = false; group.add(gm);
    const metal = supportGeos(F, [-(half + lt + 0.0015), half + lt + 0.0015], low);

    // Sign plate on two struts above the floor, at ~40 % of the ramp (or the first point that is high enough).
    let si = Math.floor(F.count * 0.4);
    for (let i = si; i < F.count - 4; i++) { if (F.p[3 * i + 1] >= 0.03) { si = i; break; } }
    const tex = signTexture(def.color || '#37d9ff'); D.t(tex);
    const signMat = new THREE.MeshStandardMaterial({ map: tex, emissive: new THREE.Color('#ffffff'), emissiveMap: tex, emissiveIntensity: 0.5, roughness: 0.4, metalness: 0, side: THREE.DoubleSide }); D.m(signMat);
    const signGeo = new THREE.PlaneGeometry(w + 2 * lt, 0.016); D.g(signGeo);
    const sign = new THREE.Mesh(signGeo, signMat); sign.name = def.id + '_sign';
    const basis = new THREE.Matrix4().makeBasis(new THREE.Vector3(F.r[3 * si], F.r[3 * si + 1], F.r[3 * si + 2]), new THREE.Vector3(F.u[3 * si], F.u[3 * si + 1], F.u[3 * si + 2]),
        new THREE.Vector3(-F.t[3 * si], -F.t[3 * si + 1], -F.t[3 * si + 2]));
    sign.quaternion.setFromRotationMatrix(basis);
    sign.position.copy(at(F, si, 0, 0.040, new THREE.Vector3()));
    group.add(sign);
    for (const side of [-1, 1]) {
        const sg = new THREE.CylinderGeometry(0.0009, 0.0009, 0.032 - lh, 5).translate(0, (0.032 - lh) / 2 + lh, 0);
        sg.applyQuaternion(new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 1, 0), new THREE.Vector3(F.u[3 * si], F.u[3 * si + 1], F.u[3 * si + 2])));
        const o = at(F, si, side * (half + lt / 2), 0, new THREE.Vector3()); sg.translate(o.x, o.y, o.z); metal.push(sg);
    }
    metalMesh(def, metal, mats.wire, group, D);

    // Entry flap: a hinged lip that lowers when a ball arrives.
    const fi = Math.min(3, F.count - 2);
    const flapGeo = new THREE.BoxGeometry(w - 0.002, 0.0012, 0.02); flapGeo.translate(0, 0.0006, -0.01); D.g(flapGeo);
    const flapMat = new THREE.MeshPhysicalMaterial({ color: color.clone().multiplyScalar(0.8), transparent: true, opacity: 0.6, roughness: 0.15, envMap: env, emissive: color.clone().multiplyScalar(0.2) }); D.m(flapMat);
    const flap = new THREE.Mesh(flapGeo, flapMat); flap.name = def.id + '_flap';
    const fb = new THREE.Matrix4().makeBasis(new THREE.Vector3(F.r[3 * fi], F.r[3 * fi + 1], F.r[3 * fi + 2]), new THREE.Vector3(F.u[3 * fi], F.u[3 * fi + 1], F.u[3 * fi + 2]),
        new THREE.Vector3(-F.t[3 * fi], -F.t[3 * fi + 1], -F.t[3 * fi + 2]));
    const pivot = new THREE.Group(); pivot.quaternion.setFromRotationMatrix(fb); pivot.position.copy(at(F, fi, 0, 0.0004, new THREE.Vector3()));
    pivot.add(flap); group.add(pivot);
    return { flap, pivot, state: 1, hinge: pivot.position };
}

function wireRamp(def, F, ctx, group, D) {
    const { mats, low } = ctx, FR = 0.0095, GR = 0.0165, FU = 0.0019, GU = 0.0165, WR = 0.0015;
    const idx = indicesEvery(F.count, low ? 10 : 5);
    const tubes = [];
    for (const [a, b] of [[-FR, FU], [FR, FU], [-GR, GU], [GR, GU]]) {
        const pts = idx.map(i => at(F, i, a, b, new THREE.Vector3()));
        tubes.push(new THREE.TubeGeometry(new THREE.CatmullRomCurve3(pts, false, 'centripetal'), idx.length * (low ? 1 : 2), WR, low ? 4 : 6, false));
    }
    const metal = tubes;
    // Ribs every 0.06 m of arc length.
    const m = new THREE.Matrix4(), ribGeo = merge([
        new THREE.BoxGeometry(2 * GR, 0.0011, 0.0011).translate(0, FU - 0.0006, 0),
        new THREE.BoxGeometry(0.0011, GU - FU + 0.001, 0.0011).translate(-GR, (GU + FU) / 2, 0),
        new THREE.BoxGeometry(0.0011, GU - FU + 0.001, 0.0011).translate(GR, (GU + FU) / 2, 0)
    ]); D.g(ribGeo);
    const stations = [];
    let acc = 0.06;
    for (let i = 1; i < F.count; i++) {
        acc += Math.hypot(F.p[3 * i] - F.p[3 * i - 3], F.p[3 * i + 1] - F.p[3 * i - 2], F.p[3 * i + 2] - F.p[3 * i - 1]);
        if (acc >= 0.06) { acc = 0; stations.push(i); }
    }
    stations.forEach(i => {
        const g = ribGeo.clone();
        g.applyMatrix4(m.makeBasis(new THREE.Vector3(F.r[3 * i], F.r[3 * i + 1], F.r[3 * i + 2]), new THREE.Vector3(F.u[3 * i], F.u[3 * i + 1], F.u[3 * i + 2]), new THREE.Vector3(-F.t[3 * i], -F.t[3 * i + 1], -F.t[3 * i + 2])));
        g.translate(F.p[3 * i], F.p[3 * i + 1], F.p[3 * i + 2]); metal.push(g);
    });
    metal.push(...supportGeos(F, [0], low));
    metalMesh(def, metal, mats.wire, group, D);
    return null;
}

function tubeRamp(def, F, ctx, group, D) {
    const { mats, low, env } = ctx, rad = 0.0185, idx = indicesEvery(F.count, low ? 8 : 4);
    const color = new THREE.Color(def.color || '#a98bff');
    const pts = idx.map(i => at(F, i, 0, BALL_R, new THREE.Vector3()));
    const curve = new THREE.CatmullRomCurve3(pts, false, 'centripetal');
    const geo = new THREE.TubeGeometry(curve, idx.length * (low ? 1 : 2), rad, low ? 8 : 14, false); D.g(geo);
    const mat = new THREE.MeshPhysicalMaterial({ color: color.clone().lerp(new THREE.Color('#ffffff'), 0.2), transparent: true, opacity: 0.1, roughness: 0.05, metalness: 0, side: THREE.DoubleSide, depthWrite: false,
        envMap: env, envMapIntensity: 0.7, emissive: color.clone().multiplyScalar(0.1) }); D.m(mat);
    const mesh = new THREE.Mesh(geo, mat); mesh.name = def.id + '_tube'; mesh.renderOrder = 2; mesh.frustumCulled = false; group.add(mesh);
    // Metal collars at both ends.
    const ring = new THREE.TorusGeometry(rad + 0.0006, 0.0012, 5, low ? 12 : 20); D.g(ring);
    const rings = [];
    for (const [i, flip] of [[0, 1], [F.count - 1, -1]]) {
        const g = ring.clone(), b = new THREE.Matrix4().makeBasis(new THREE.Vector3(F.r[3 * i], F.r[3 * i + 1], F.r[3 * i + 2]), new THREE.Vector3(F.u[3 * i], F.u[3 * i + 1], F.u[3 * i + 2]),
            new THREE.Vector3(-F.t[3 * i], -F.t[3 * i + 1], -F.t[3 * i + 2]));
        g.applyMatrix4(b); g.translate(...at(F, i, 0, BALL_R, new THREE.Vector3()).toArray()); void flip; rings.push(g);
    }
    metalMesh(def, [...rings, ...supportGeos(F, [0], low)], mats.steel, group, D);
    return null;
}

// Builds every layout path. `paths` are physics debug entries ({id, kind, color, width, samples}).
// Returns {group, update(snap), dispose()}.
export function buildRamps(paths, ctx) {
    const group = new THREE.Group(); group.name = 'ramps';
    const own = { g: [], m: [], t: [] };
    const D = { g: x => { if (x) own.g.push(x); return x; }, m: x => { own.m.push(x); return x; }, t: x => { own.t.push(x); return x; } };
    const flaps = [];
    for (const def of paths) {
        const F = pathFrames(def);
        if (F.count < 2) continue;
        let entry = null;
        if (def.kind === 'plastic') entry = plasticRamp(def, F, ctx, group, D);
        else if (def.kind === 'tube') tubeRamp(def, F, ctx, group, D);
        else wireRamp(def, F, ctx, group, D);
        if (entry) flaps.push(entry);
    }
    return {
        group,
        update(snap, dt) {
            for (const f of flaps) {
                let near = 0;
                if (snap && snap.balls) for (const b of snap.balls) {
                    if (b.hidden) continue;
                    const dx = b.x - f.hinge.x, dy = -b.y - f.hinge.z;
                    if (dx * dx + dy * dy < 0.045 * 0.045) { near = 1; break; }
                }
                f.state += ((near ? 0 : 1) - f.state) * Math.min(1, dt * 14);
                f.pivot.children[0].rotation.x = 0.38 * f.state;
            }
        },
        dispose() {
            own.g.forEach(g => g.dispose()); own.m.forEach(m => m.dispose()); own.t.forEach(t => t.dispose());
            group.traverse(o => { if (o.isInstancedMesh) o.dispose(); });
        }
    };
}
