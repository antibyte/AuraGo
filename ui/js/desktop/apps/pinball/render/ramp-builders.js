// AuraPinball ramp builders (R2): stainless wire habitrails, clear polycarbonate ramps, acrylic tubes, entry flaps,
// standoffs and entrance signs. buildRamps (ramps.js) owns the lifecycle; every builder gets a ramp context `rc`
// ({ low, tier, env, layout, images, D, steel, obstacles, sheetTexture() }, see rampContext) and returns update state or null.
// Frames are (r, u, -t): local x = lateral right of travel, local y = up from the path line, local -z = travel direction.
import * as THREE from 'three';
import { merge, canvasTexture } from './materials.js';
import { at, sweep, indicesEvery } from './ramps.js';
import { distPointSeg, pointInPolygon } from '../tables/kit/geom.js';
import { cellUV } from '../tables/kit/plastics.js';

const BALL_R = 0.0135, WR = 0.0021;
const V = (a, b, c) => new THREE.Vector3(a, b, c);
const FU = BALL_R - Math.sqrt((BALL_R + WR) ** 2 - 0.0092 ** 2);   // bottom rail height so the ball rests on both rails
const SIGN = { w: 0.060, h: 0.018, t: 0.0012, lift: 0.036, tilt: -15 * Math.PI / 180 };

// Frame i as a matrix (local x = r, y = u, z = -t, origin on the path line).
function frame(F, i) {
    return new THREE.Matrix4().makeBasis(V(F.r[3 * i], F.r[3 * i + 1], F.r[3 * i + 2]), V(F.u[3 * i], F.u[3 * i + 1], F.u[3 * i + 2]), V(-F.t[3 * i], -F.t[3 * i + 1], -F.t[3 * i + 2]))
        .setPosition(F.p[3 * i], F.p[3 * i + 1], F.p[3 * i + 2]);
}
// Box in the frame of sample i: centre (lat, up, along) and size (lateral, up, along).
function fbox(F, i, lat, up, along, sl, su, sa) {
    return new THREE.BoxGeometry(sl, su, sa).translate(lat, up, along).applyMatrix4(frame(F, i));
}
function rod(a, b, r, seg) {
    const d = new THREE.Vector3().subVectors(b, a), len = d.length() || 1e-6, m = a.clone().add(b).multiplyScalar(0.5);
    return new THREE.CylinderGeometry(r, r, len, seg).applyQuaternion(new THREE.Quaternion().setFromUnitVectors(V(0, 1, 0), d.divideScalar(len))).translate(m.x, m.y, m.z);
}
// Sample indices at every `step` metres of arc length, the first one at `first`.
function stationsEvery(F, step, first = step) {
    const out = [];
    let acc = 0, next = first;
    for (let i = 1; i < F.count; i++) {
        acc += Math.hypot(F.p[3 * i] - F.p[3 * i - 3], F.p[3 * i + 1] - F.p[3 * i - 2], F.p[3 * i + 2] - F.p[3 * i - 1]);
        if (acc >= next) { out.push(i); next += step; }
    }
    return out;
}
const tierName = rc => (rc.low ? 'low' : rc.tier && rc.tier.shadow > 0 && rc.tier.shadow < 2048 ? 'medium' : 'high');

// Shared state for one buildRamps call: stainless material, obstacle lists for the standoff test, the sheet texture.
export function rampContext(ctx, D, layout, images) {
    const base = ctx.mats && (ctx.mats.chrome || ctx.mats.steel);
    const steel = D.m(base ? base.clone() : new THREE.MeshStandardMaterial({ color: '#c9d3df', metalness: 0.78 }));
    steel.roughness = 0.4; steel.envMapIntensity = 0.5;   // plan: 0.32 / 0.55; the long rails bloom out with the table lights, so a little satin if (ctx.env) steel.envMap = ctx.env;
    const walls = (layout && layout.walls) || [], segs = [];
    for (const w of walls) {
        const pts = w.points || [], n = w.closed ? pts.length : pts.length - 1;
        for (let i = 0; i < n; i++) segs.push([pts[i], pts[(i + 1) % pts.length]]);
    }
    const outer = walls.find(w => w.id === 'outer');
    let sheet = null;
    return {
        low: !!ctx.low, tier: ctx.tier || null, env: ctx.env, layout, images: images || {}, D, steel,
        obstacles: layout ? { segs, posts: (layout.posts || []), outline: outer ? outer.points : null } : null,
        sheetTexture() {
            const img = this.images.plastics;
            if (!img) return null;
            if (!sheet) {
                const c = document.createElement('canvas'); c.width = img.width; c.height = img.height;
                c.getContext('2d').drawImage(img, 0, 0);
                sheet = D.t(canvasTexture(c, { aniso: 4 }));
            }
            return sheet;
        }
    };
}

// Distance from a playfield point to the nearest wall segment or post edge; -1 when the point lies outside the outline.
function obstacleDistance(ob, x, y) {
    if (ob.outline && !pointInPolygon([x, y], ob.outline)) return -1;
    let d = Infinity;
    for (const [a, b] of ob.segs) d = Math.min(d, distPointSeg(x, y, a[0], a[1], b[0], b[1]));
    for (const p of ob.posts) d = Math.min(d, Math.hypot(x - p.x, y - p.y) - p.r);
    return d;
}

// Hex standoffs with a base flange and a flat bracket to the ramp. Candidate stations carry a foot on each side at
// lateral `foot`; a foot is kept when it lands 6.5..12 mm from a wall or post edge, or outside the outline. At least one
// foot per ramp survives (the valid candidate nearest a wall). `uTop` is the bracket underside in the frame, `from` the
// lateral where the bracket starts under the ramp. Returns merged-ready geometries.
function standoffs(F, rc, { step, from, foot, uTop }) {
    const cands = [];
    for (const i of stationsEvery(F, step)) {
        if (F.p[3 * i + 1] <= 0.015) continue;
        for (const side of [-1, 1]) {
            const P = at(F, i, side * foot, uTop, new THREE.Vector3());
            if (P.y < 0.004) continue;
            const d = rc.obstacles ? obstacleDistance(rc.obstacles, P.x, -P.z) : 0.009;
            cands.push({ i, side, P, d: d < 0 ? 0.012 : d, out: d < 0 });
        }
    }
    let keep = cands.filter(c => c.d >= 0.0065 && c.d <= 0.012);
    if (!keep.length) {
        const valid = cands.filter(c => c.d >= 0.0065).sort((a, b) => a.d - b.d);
        keep = valid.length ? [valid[0]] : cands.slice(0, 1);
    }
    const out = [], seg = rc.low ? 8 : 14;
    for (const c of keep) {
        const { P, i, side } = c, hh = P.y - 0.001;
        out.push(new THREE.CylinderGeometry(0.0028, 0.0028, hh, 6).translate(P.x, 0.001 + hh / 2, P.z));
        out.push(new THREE.CylinderGeometry(0.005, 0.005, 0.001, seg).translate(P.x, 0.0005, P.z));
        const len = foot + 0.003 - from;
        out.push(fbox(F, i, side * (from + len / 2), uTop + 0.0007, 0, len, 0.0014, 0.008));
    }
    return out;
}

function metalMesh(def, parts, rc, group) {
    const geo = merge(parts);
    if (!geo) return;
    rc.D.g(geo);
    const m = new THREE.Mesh(geo, rc.steel); m.name = def.id + '_metal'; m.castShadow = !rc.low; m.frustumCulled = false; group.add(m);
}

function signTexture(color) {
    const c = document.createElement('canvas'); c.width = 128; c.height = 40;
    const g = c.getContext('2d');
    const bg = g.createLinearGradient(0, 0, 0, 40);
    bg.addColorStop(0, '#101622'); bg.addColorStop(1, '#04060b');
    g.fillStyle = bg; g.fillRect(0, 0, 128, 40);
    g.shadowColor = color; g.shadowBlur = 8;
    g.strokeStyle = color; g.lineWidth = 4; g.lineJoin = 'round'; g.lineCap = 'round';
    for (let k = 0; k < 3; k++) {
        const x = 30 + k * 34;
        g.globalAlpha = 0.48 + k * 0.26;
        g.beginPath(); g.moveTo(x - 12, 28); g.lineTo(x, 11); g.lineTo(x + 12, 28); g.stroke();
    }
    g.shadowBlur = 0; g.globalAlpha = 1; g.lineWidth = 2.6; g.strokeRect(2.5, 2.5, 123, 35);
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    return t;
}

// Entrance sign (AM7): a 0.060 x 0.018 plate on two thin posts a little way into the ramp, facing the player, tilted
// back 15 degrees, lower edge 0.036 above the ramp floor. Plastic ramps stand the posts on the wall tops, wire ramps
// on the playfield. Returns the front material (the pulse lights it) and pushes the posts into `metal`.
function entranceSign(def, F, rc, group, metal, { cell, postLat, postBase, flange }) {
    let si = 1;
    const first = stationsEvery(F, 0.032)[0];
    if (first !== undefined) si = first;
    si = Math.min(si, F.count - 2);
    let ax = F.r[3 * si], az = F.r[3 * si + 2];
    if (ax < 0) { ax = -ax; az = -az; }
    const yaw = Math.max(-0.6, Math.min(0.6, Math.atan2(-az, ax || 1e-6)));
    const q = new THREE.Quaternion().setFromEuler(new THREE.Euler(SIGN.tilt, yaw, 0, 'YXZ'));
    const lower = V(F.p[3 * si], F.p[3 * si + 1] + SIGN.lift, F.p[3 * si + 2]);
    const centre = lower.clone().add(V(0, SIGN.h / 2, 0).applyQuaternion(q));
    const shared = rc.sheetTexture(), sheet = !!shared, tex = shared || rc.D.t(signTexture(def.color || '#37d9ff'));
    const geo = new THREE.BoxGeometry(SIGN.w, SIGN.h, SIGN.t); rc.D.g(geo);
    if (sheet) {   // front face (+z) = vertices 16..19: map them onto the sheet cell
        const [u0, v0, u1, v1] = cellUV(cell), uv = geo.attributes.uv;
        for (let k = 16; k < 20; k++) uv.setXY(k, u0 + uv.getX(k) * (u1 - u0), v0 + uv.getY(k) * (v1 - v0));
    }
    const edge = rc.D.m(new THREE.MeshStandardMaterial({ color: '#2a2f38', roughness: 0.5, metalness: 0.6 }));
    const front = rc.D.m(new THREE.MeshStandardMaterial({ map: tex, emissive: new THREE.Color('#ffffff'), emissiveMap: tex, emissiveIntensity: sheet ? 0.2 : 0.5, roughness: 0.35, metalness: 0, alphaTest: sheet ? 0.5 : 0 }));
    const mesh = new THREE.Mesh(geo, [edge, edge, edge, edge, front, edge]); mesh.name = def.id + '_sign';
    mesh.position.copy(centre); mesh.quaternion.copy(q); mesh.castShadow = !rc.low; group.add(mesh);
    for (const side of [-1, 1]) {
        const top = centre.clone().add(V(side * (postLat), -SIGN.h / 2 + 0.006, -SIGN.t / 2).applyQuaternion(q));
        const base = postBase(side, si, top);
        metal.push(rod(base, top, 0.0009, 5));
        if (flange) metal.push(new THREE.CylinderGeometry(0.004, 0.004, 0.001, rc.low ? 8 : 12).translate(base.x, base.y + 0.0005, base.z));
    }
    return { signMat: front, signBase: sheet ? 0.2 : 0.5 };
}

// ---- plastic ramp: clear tinted polycarbonate U-channel, 22 mm walls ------------------------------------------------
// Clear polycarbonate: no transmission pass (it milked the playfield art on bright accents); a low alpha so the art shows through,
// a light tint in the accent colour and a fresnel term that lifts the alpha towards the grazing edges.
function plasticMaterial(rc, color) {
    const tier = tierName(rc), tint = color.clone().lerp(new THREE.Color('#ffffff'), 0.5);
    const o = { color: tint, roughness: 0.1, metalness: 0, side: THREE.DoubleSide, depthWrite: false, transparent: true, opacity: tier === 'low' ? 0.22 : 0.18,
        envMap: rc.env, envMapIntensity: 0.45, emissive: color.clone().multiplyScalar(0.04) };
    if (tier === 'high') Object.assign(o, { clearcoat: 0.5, clearcoatRoughness: 0.12 });
    else if (tier === 'medium') Object.assign(o, { clearcoat: 0.3, clearcoatRoughness: 0.2 });
    const m = new THREE.MeshPhysicalMaterial(o);
    m.onBeforeCompile = sh => {
        sh.fragmentShader = sh.fragmentShader.replace('#include <opaque_fragment>',
            'float rampRim = pow(1.0 - clamp(abs(dot(normalize(vNormal), normalize(vViewPosition))), 0.0, 1.0), 2.5);\n  diffuseColor.a = clamp(diffuseColor.a + rampRim * 0.45, 0.0, 0.8);\n#include <opaque_fragment>');
    };
    m.customProgramCacheKey = () => 'ramp-clear-v1';
    return rc.D.m(m);
}

function entryFlap(F, rc, group, w) {
    const fi = Math.min(3, F.count - 2), W = w - 0.004;
    // Sheet 0.8 mm thick lying on the floor ahead of the hinge rod, two knuckle tabs wrap the rod. Pivot = rod axis (u 0.0012).
    const geo = merge([new THREE.BoxGeometry(W, 0.0008, 0.022).translate(0, -0.0008, -0.011),
        ...[-1, 1].map(s => new THREE.BoxGeometry(0.004, 0.0024, 0.004).translate(s * (W / 2 - 0.004), 0, 0))]);
    rc.D.g(geo);
    const flap = new THREE.Mesh(geo, rc.D.m(new THREE.MeshStandardMaterial({ color: '#4a525c', metalness: 0.8, roughness: 0.55, envMap: rc.env, envMapIntensity: 0.22 }))); flap.name = 'flap'; flap.castShadow = !rc.low;
    const pivot = new THREE.Group(); pivot.quaternion.setFromRotationMatrix(frame(F, fi).setPosition(0, 0, 0));
    pivot.position.copy(at(F, fi, 0, 0.0012, new THREE.Vector3()));
    const hinge = new THREE.CylinderGeometry(0.0009, 0.0009, w - 0.001, rc.low ? 6 : 8).rotateZ(Math.PI / 2); rc.D.g(hinge);
    pivot.add(flap, new THREE.Mesh(hinge, rc.steel));
    group.add(pivot);
    return { flap, pivot };
}

export function plasticRamp(def, F, rc, group, withSign) {
    const { low, D } = rc, w = def.width || 0.05, half = w / 2, lt = 0.0026, th = 0.0025, lh = 0.022;
    const color = new THREE.Color(def.color || '#37d9ff'), idx = indicesEvery(F.count, low ? 6 : 2);
    const body = sweep(F, idx, [[-half - lt, -th], [-half - lt, lh], [-half, lh], [-half, 0], [half, 0], [half, lh], [half + lt, lh], [half + lt, -th]]); D.g(body);
    const mesh = new THREE.Mesh(body, plasticMaterial(rc, color)); mesh.name = def.id + '_channel'; mesh.renderOrder = 2; mesh.frustumCulled = false; group.add(mesh);
    // Thin edge glow along both lip tops.
    const e = 0.0008, glowParts = [];
    for (const side of [-1, 1]) {
        const a = side * (half + lt / 2);
        glowParts.push(sweep(F, idx, [[a - e, lh + 0.0004], [a + e, lh + 0.0004], [a + e, lh + 0.0016], [a - e, lh + 0.0016]]));
    }
    const glow = merge(glowParts); D.g(glow);
    const glowMat = D.m(new THREE.MeshBasicMaterial({ color: color.clone().multiplyScalar(1.25) }));
    const gm = new THREE.Mesh(glow, glowMat); gm.name = def.id + '_edge'; gm.frustumCulled = false; group.add(gm);
    const metal = standoffs(F, rc, { step: low ? 0.2 : 0.1, from: half - 0.004, foot: half + lt + 0.0058, uTop: -th - 0.0014 });
    let sign = null;
    if (withSign) {
        const wall = half + lt / 2;
        sign = entranceSign(def, F, rc, group, metal, { cell: 'sign1', postLat: wall, flange: false,
            postBase: (side, si) => at(F, si, side * wall, lh, new THREE.Vector3()) });   // posts on the wall tops, no flange
    }
    metalMesh(def, metal, rc, group);
    const fl = entryFlap(F, rc, group, w);
    return { ...fl, state: 1, hinge: fl.pivot.position, glowMat, ...(sign || {}), baseColor: color.clone(), pulse: 0, F };
}

// ---- wire habitrail: 4.2 mm stainless wire -----------------------------------------------------------------------
export function wireRamp(def, F, rc, group, withSign) {
    const { low } = rc, BR = 0.0092, GR = 0.0158, GU = 0.0135, SR = GR + WR + 0.0007;   // strap outside the guide rails
    const idx = indicesEvery(F.count, low ? 8 : 4), metal = [];
    for (const [a, b] of [[-BR, FU], [BR, FU], [-GR, GU], [GR, GU]]) {
        const pts = idx.map(i => at(F, i, a, b, new THREE.Vector3()));
        metal.push(new THREE.TubeGeometry(new THREE.CatmullRomCurve3(pts, false, 'centripetal'), idx.length * (low ? 1 : 2), WR, low ? 4 : 6, false));
    }
    // Formed U straps: bottom bar under the rails, a bar up each outer side and a short hook over each guide rail.
    const straps = stationsEvery(F, low ? 0.1 : 0.05, 0.025), bu = FU - WR - 0.0007, tw = 0.0026;
    for (const i of straps) {
        metal.push(fbox(F, i, 0, bu, 0, 2 * SR, 0.0014, tw));
        for (const s of [-1, 1]) {
            metal.push(fbox(F, i, s * SR, (bu + GU + WR) / 2, 0, 0.0014, GU + WR - bu + 0.0007, tw));
            metal.push(fbox(F, i, s * (SR - 0.0025), GU + WR + 0.0007, 0, 0.0050, 0.0014, tw));
        }
    }
    metal.push(...standoffs(F, rc, { step: low ? 0.2 : 0.1, from: 0.012, foot: SR + 0.0058, uTop: bu - 0.0014 }));
    let sign = null;
    if (withSign) sign = entranceSign(def, F, rc, group, metal, { cell: 'sign2', postLat: 0.0262, flange: true, postBase: (side, si, top) => V(top.x, 0, top.z) });
    metalMesh(def, metal, rc, group);
    return { state: 1, hinge: new THREE.Vector3(F.p[0], F.p[1], F.p[2]), ...(sign || {}), pulse: 0, F };
}

// ---- tube: clear acrylic with metal collars ----------------------------------------------------------------------
export function tubeRamp(def, F, rc, group) {
    const { low, D } = rc, rad = 0.0185, idx = indicesEvery(F.count, low ? 8 : 4), color = new THREE.Color(def.color || '#a98bff');
    const curve = new THREE.CatmullRomCurve3(idx.map(i => at(F, i, 0, BALL_R, new THREE.Vector3())), false, 'centripetal');
    const geo = D.g(new THREE.TubeGeometry(curve, idx.length * (low ? 1 : 2), rad, low ? 8 : 14, false));
    const mat = D.m(new THREE.MeshPhysicalMaterial({ color: color.clone().lerp(new THREE.Color('#ffffff'), 0.6), transparent: true, opacity: 0.12, roughness: 0.1, metalness: 0, side: THREE.DoubleSide, depthWrite: false,
        envMap: rc.env, envMapIntensity: 0.6, emissive: color.clone().multiplyScalar(0.06) }));
    const mesh = new THREE.Mesh(geo, mat); mesh.name = def.id + '_tube'; mesh.renderOrder = 2; mesh.frustumCulled = false; group.add(mesh);
    const ring = new THREE.TorusGeometry(rad + 0.0006, 0.0012, 5, low ? 12 : 20), parts = [];
    for (const i of [0, F.count - 1]) parts.push(ring.clone().applyMatrix4(frame(F, i).setPosition(...at(F, i, 0, BALL_R, new THREE.Vector3()).toArray())));
    ring.dispose();
    parts.push(...standoffs(F, rc, { step: low ? 0.2 : 0.1, from: 0.008, foot: rad + 0.0058, uTop: -0.0034 }));
    metalMesh(def, parts, rc, group);
    return null;
}
