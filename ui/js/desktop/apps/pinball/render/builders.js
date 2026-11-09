// AuraPinball mechanics builder (plan 8 / T13): turns the layout into 3D flippers, rubbers, posts, slings, pop bumpers,
// targets, spinner, rollovers, gates, holes, plunger, ramps and balls. buildMechanics(host) is the T12 addMechanics builder.
// Per-type builders return {object, update(anim, snap, extra)}; static pieces are pushed into ctx.statics and merged per material.
import * as THREE from 'three';
import { FLIPPER } from '../physics/constants.js';
import { buildAnimIndex } from '../physics/snapshot.js';
import { createPaths } from '../physics/paths.js';
import { merge } from './materials.js';
import * as M from './mechs.js';
import { buildRamps } from './ramps.js';
import { createBalls } from './balls.js';

const EMPTY = new Float32Array(0);
const isLow = tier => !!tier && tier.shadow === 0 && tier.lights <= 2;
const place = (g, x, y, ang = 0, h = 0) => { if (ang) g.rotateY(ang); g.translate(x, h, -y); return g; };
const dirAngle = (a, b) => Math.atan2(b[1] - a[1], b[0] - a[0]);

function makeCtx(host, D) {
    const { layout, mats, env, tier } = host, meta = layout.meta || {};
    const accent = meta.accent || '#37d9ff', secondary = meta.secondary || '#ff48c4', low = isLow(tier);
    const std = o => D.m(new THREE.MeshStandardMaterial({ envMap: env, ...o }));
    const rubber = std({ color: '#262932', roughness: 0.46, metalness: 0, envMapIntensity: 0.8 });
    const flipperTop = low
        ? std({ color: new THREE.Color(accent).multiplyScalar(0.8), roughness: 0.3, emissive: new THREE.Color(accent).multiplyScalar(0.14), envMapIntensity: 1 })
        : D.m(new THREE.MeshPhysicalMaterial({ color: new THREE.Color(accent).multiplyScalar(0.8), roughness: 0.28, clearcoat: 0.8, clearcoatRoughness: 0.12, emissive: new THREE.Color(accent).multiplyScalar(0.14), envMap: env, envMapIntensity: 1.1 }));
    const chrome = D.m(mats.chrome.clone()); chrome.envMapIntensity = 0.7;
    const wire = D.m(mats.chrome.clone()); wire.color.set('#8d98a8'); wire.envMapIntensity = 0.32; wire.roughness = 0.52;
    const lampList = layout.lamps || [];
    const ctx = {
        host, layout, mats: { ...mats, chrome, wire, rubber, flipperTop }, env, tier, low, accent, secondary, D, idx: buildAnimIndex(layout),
        statics: { chrome: [], steel: [], dark: [], rubber: [], glow: [], glow2: [] }, updaters: [], root: new THREE.Group(),
        lampIndex: (pred) => lampList.findIndex(pred),
        slot: id => (ctx.idx.has(id) ? ctx.idx.get(id) : -1),
        nearestLamp(x, y, maxD = 0.045) {
            let best = null, bd = maxD;
            for (const l of lampList) { if (l.mount !== 'insert') continue; const d = Math.hypot(l.x - x, l.y - y); if (d < bd) { bd = d; best = l; } }
            return best;
        }
    };
    ctx.root.name = 'mechanics_built';
    return ctx;
}

const mesh = (ctx, geo, mat, name) => { ctx.D.g(geo); const m = new THREE.Mesh(geo, mat); if (name) m.name = name; m.castShadow = !ctx.low; return m; };
const at = (grp, x, y, h = 0, ang = 0) => { grp.position.set(x, h, -y); grp.rotation.y = ang; return grp; };
const A = (anim, i) => (i >= 0 && i < anim.length ? anim[i] : 0);

// ---- flipper ----
export function buildFlipper(ctx, def, i) {
    const p = { ...FLIPPER, ...def }, side = def.side < 0 ? -1 : 1;
    const { rubber, top } = M.flipperGeometry(p, ctx.low);
    const root = at(new THREE.Group(), def.x, def.y), bat = new THREE.Group(); root.add(bat);
    bat.add(mesh(ctx, rubber, ctx.mats.rubber, def.id + '_rubber'), mesh(ctx, top, ctx.mats.flipperTop, def.id + '_top'));
    const { cap, dot } = M.pivotCap(def.x, def.y, ctx.low);
    ctx.statics.chrome.push(cap); ctx.statics.glow.push(dot);
    const set = a => { bat.rotation.y = side > 0 ? a : Math.PI - a; };
    set(p.rest);
    return { object: root, update(anim, snap) { if (snap && snap.flippers && 2 * i < snap.flippers.length) set(snap.flippers[2 * i]); } };
}

// ---- rubber walls, sling assemblies ----
function slingTriangle(layout, face) {
    const pre = face.id.replace(/_face$/, ''), pts = [];
    for (const w of layout.walls) if (w.id === face.id || w.id === pre + '_side' || w.id === pre + '_base') for (const q of w.points) if (!pts.some(p => Math.hypot(p[0] - q[0], p[1] - q[1]) < 1e-4)) pts.push(q);
    return pts.length >= 3 ? pts.slice(0, 3) : null;
}

export function buildSling(ctx, def) {
    const face = ctx.layout.walls.find(w => w.id === def.face);
    if (!face) return null;
    const tri = slingTriangle(ctx.layout, face);
    if (!tri) return null;
    const cx = tri.reduce((a, p) => a + p[0], 0) / 3, cy = tri.reduce((a, p) => a + p[1], 0) / 3;
    const [a, b] = face.points, dx = b[0] - a[0], dy = b[1] - a[1], l = Math.hypot(dx, dy) || 1;
    let nx = -dy / l, ny = dx / l;
    if (nx * ((a[0] + b[0]) / 2 - cx) + ny * ((a[1] + b[1]) / 2 - cy) < 0) { nx = -nx; ny = -ny; }
    const root = new THREE.Group(), band = mesh(ctx, M.bandGeometry(a, b, ctx.low, [-nx * 0.0022, -ny * 0.0022]), ctx.mats.rubber, def.id + '_kick');
    root.add(band);
    const col = new THREE.Color(ctx.secondary), inner = M.insetPolygon(tri, 0.0058), bounds = M.slingBounds(inner);
    const plate = M.planarUV(M.polygonPlate(inner, 0.0018, 0.0006, ctx.low).translate(0, 0.0188, 0), bounds), art = ctx.D.t(M.slingArtTexture(inner, bounds, ctx.secondary, ctx.accent));
    const plasticMat = ctx.D.m(new THREE.MeshPhysicalMaterial({ color: '#ffffff', map: art, transparent: true, opacity: 0.88, roughness: 0.16, metalness: 0, envMap: ctx.env, envMapIntensity: 0.9,
        clearcoat: ctx.low ? 0 : 0.8, clearcoatRoughness: 0.1, emissive: new THREE.Color(0.32, 0.32, 0.32), emissiveMap: art, emissiveIntensity: 1, depthWrite: false }));
    const glowMat = ctx.D.m(new THREE.MeshBasicMaterial({ color: col.clone().multiplyScalar(1.8), transparent: true, opacity: 0.1, blending: THREE.AdditiveBlending, depthWrite: false }));
    const plastic = mesh(ctx, plate, plasticMat, def.id + '_plastic'); plastic.renderOrder = 2;
    root.add(plastic);
    let glow = null;
    if (!ctx.low) { glow = mesh(ctx, M.polygonPlate(M.insetPolygon(tri, 0.0075), 0.0004, 0, true).translate(0, 0.0182, 0), glowMat, def.id + '_glow'); glow.castShadow = false; root.add(glow); }
    const slot = ctx.slot(def.id);
    return { object: root, update(anim) {
        const k = A(anim, slot);
        band.position.set(nx * 0.0055 * k, 0, -ny * 0.0055 * k);
        glowMat.opacity = 0.08 + 0.8 * k; plasticMat.emissiveIntensity = 1 + (glow ? 2.6 : 5) * k;
    } };
}

function buildStaticRubbers(ctx) {
    const faces = new Set(ctx.layout.devices.filter(d => d.type === 'sling').map(d => d.face));
    for (const w of ctx.layout.walls) {
        if (w.render !== 'rubber' || faces.has(w.id) || !w.points) continue;
        const m = /^(.*)_(side|base)$/.exec(w.id), face = m && ctx.layout.walls.find(q => q.id === m[1] + '_face'), tri = face && slingTriangle(ctx.layout, face);
        let sh = null;
        if (tri) {
            const cx = tri.reduce((a, p) => a + p[0], 0) / 3, cy = tri.reduce((a, p) => a + p[1], 0) / 3, [p, q] = w.points, dx = q[0] - p[0], dy = q[1] - p[1], l = Math.hypot(dx, dy) || 1;
            let nx = -dy / l, ny = dx / l;
            if (nx * ((p[0] + q[0]) / 2 - cx) + ny * ((p[1] + q[1]) / 2 - cy) < 0) { nx = -nx; ny = -ny; }
            sh = [-nx * 0.0022, -ny * 0.0022];
        }
        for (let i = 0; i < w.points.length - 1; i++) ctx.statics.rubber.push(M.bandGeometry(w.points[i], w.points[i + 1], ctx.low, sh));
    }
}

function buildPosts(ctx) {
    for (const p of ctx.layout.posts || []) {
        if (p.render !== 'post') continue;
        const g = M.postGeometry(p, ctx.low);
        if (g.rubber) ctx.statics.rubber.push(g.rubber);
        ctx.statics.chrome.push(g.chrome);
    }
}

// ---- pop bumper ----
export function buildBumper(ctx, def) {
    const r = def.r ?? 0.0215, geo = M.bumperGeometry(r, ctx.low), li = ctx.lampIndex(l => l.mount === 'bumper' && l.device === def.id);
    const lamp = li >= 0 ? ctx.layout.lamps[li] : null, lc = new THREE.Color(lamp ? lamp.color : ctx.secondary), sc = new THREE.Color(ctx.secondary);
    const root = at(new THREE.Group(), def.x, def.y);
    const gp = g => g.translate(def.x, 0, -def.y);
    ctx.statics.dark.push(gp(geo.base)); ctx.statics.chrome.push(gp(geo.stud));
    const ring = mesh(ctx, geo.ring, ctx.mats.chrome, def.id + '_ring');
    const skirtMat = ctx.D.m(new THREE.MeshStandardMaterial({ color: sc.clone().multiplyScalar(0.8), roughness: 0.32, metalness: 0.1, emissive: sc, emissiveIntensity: 0.15, envMap: ctx.env, envMapIntensity: 0.9 }));
    const skirt = mesh(ctx, geo.skirt, skirtMat, def.id + '_skirt');
    const capMat = ctx.D.m(new THREE.MeshPhysicalMaterial({ color: lc.clone().lerp(new THREE.Color('#ffffff'), 0.4), transparent: true, opacity: 0.78, roughness: 0.12, metalness: 0, emissive: lc, emissiveIntensity: 0.3,
        envMap: ctx.env, envMapIntensity: 1.3, clearcoat: ctx.low ? 0 : 1, depthWrite: false }));
    const cap = mesh(ctx, geo.cap, capMat, def.id + '_cap'); cap.renderOrder = 2;
    root.add(ring, skirt, cap);
    const slot = ctx.slot(def.id);
    return { object: root, update(anim, snap, ex) {
        const k = A(anim, slot), lit = lamp && ex.lamps ? ex.lamps.intensity[li] : 0.45;
        ring.position.y = -k * 0.0026; skirt.position.y = -k * 0.0024; cap.position.y = -k * 0.0012;
        capMat.emissiveIntensity = 0.12 + 0.8 * lit + 1.1 * k; skirtMat.emissiveIntensity = 0.1 + 0.3 * lit + 1.4 * k;
    } };
}

// ---- targets ----
function faceAtlasItems(ctx) {
    const items = [];
    const add = (id, ordinal, x, y, fallback) => {
        const l = ctx.nearestLamp(x, y);
        items.push({ key: id, label: l && l.text ? l.text : ordinal, color: (l && l.color) || fallback });
    };
    for (const d of ctx.layout.devices) {
        if (d.type === 'dropBank') d.targets.forEach((t, k) => add(t.id, d.targets.length > 1 ? k + 1 : '', t.x, t.y, ctx.secondary));
        else if (d.type === 'standup') add(d.id, '', d.x, d.y, ctx.accent);
    }
    return items;
}

export function buildDropBank(ctx, def, atlas, faceMat) {
    const root = new THREE.Group(), slots = [], parts = [], w = (def.width ?? 0.024) - 0.002;
    for (const t of def.targets) {
        const g = at(new THREE.Group(), t.x, t.y, 0, def.angle || 0), cell = atlas.cells.get(t.id);
        g.add(mesh(ctx, M.faceBox(w, 0.022, 0.0032, cell), faceMat, t.id));
        root.add(g); slots.push(ctx.slot(t.id)); parts.push(g);
        ctx.statics.dark.push(place(new THREE.BoxGeometry(0.0026, 0.016, w + 0.004).translate(-0.0048, 0.008, 0), t.x, t.y, def.angle || 0));
        ctx.statics.dark.push(place(new THREE.BoxGeometry(0.0075, 0.0012, w + 0.004).translate(-0.0012, 0.0006, 0), t.x, t.y, def.angle || 0));
    }
    return { object: root, update(anim) { for (let i = 0; i < parts.length; i++) parts[i].position.y = -A(anim, slots[i]) * 0.024; } };
}

export function buildStandup(ctx, def, atlas, faceMat) {
    const w = (def.width ?? 0.02) - 0.002, g = at(new THREE.Group(), def.x, def.y, 0, def.angle || 0), tilt = new THREE.Group();
    tilt.add(mesh(ctx, M.faceBox(w, 0.019, 0.003, atlas.cells.get(def.id)), faceMat, def.id)); g.add(tilt);
    ctx.statics.dark.push(place(new THREE.BoxGeometry(0.0034, 0.0016, w + 0.003).translate(-0.0014, 0.0008, 0), def.x, def.y, def.angle || 0));
    const slot = ctx.slot(def.id);
    return { object: g, update(anim, snap, ex) { const k = A(anim, slot); tilt.rotation.z = k * 0.2 * Math.cos(ex.time * 75); } };
}

// ---- spinner, rollovers, gate, door ----
export function buildSpinner(ctx, def) {
    const len = Math.hypot(def.b[0] - def.a[0], def.b[1] - def.a[1]), ang = dirAngle(def.a, def.b), parts = M.spinnerParts(len, ctx.low);
    const mx = (def.a[0] + def.b[0]) / 2, my = (def.a[1] + def.b[1]) / 2;
    ctx.statics.chrome.push(place(parts.frame, mx, my, ang));
    const g = at(new THREE.Group(), mx, my, parts.ry, ang), plate = mesh(ctx, parts.plate, ctx.mats.chrome, def.id + '_plate');
    g.add(plate);
    const slot = ctx.slot(def.id);
    return { object: g, update(anim) { plate.rotation.x = A(anim, slot); } };
}

export function buildRollovers(ctx, defs) {
    const geo = M.rolloverGeometry(defs[0].r ?? 0.008, ctx.low); ctx.D.g(geo);
    const im = new THREE.InstancedMesh(geo, ctx.mats.chrome, defs.length), m = new THREE.Matrix4(), slots = defs.map(d => ctx.slot(d.id));
    im.name = 'rollovers'; im.frustumCulled = false;
    const write = anim => { defs.forEach((d, i) => { m.makeTranslation(d.x, 0.0017 - A(anim, slots[i]) * 0.0016, -d.y); im.setMatrixAt(i, m); }); im.instanceMatrix.needsUpdate = true; };
    write(EMPTY);
    ctx.D.o(im);
    return { object: im, update: write };
}

export function buildGate(ctx, def) {
    const len = Math.hypot(def.b[0] - def.a[0], def.b[1] - def.a[1]), ang = dirAngle(def.a, def.b), mx = (def.a[0] + def.b[0]) / 2, my = (def.a[1] + def.b[1]) / 2;
    const pass = def.pass || [0, 1], s = (pass[0] * -(def.b[1] - def.a[1]) + pass[1] * (def.b[0] - def.a[0])) >= 0 ? 1 : -1;
    ctx.statics.chrome.push(place(new THREE.CylinderGeometry(0.0013, 0.0013, len, 6).rotateZ(Math.PI / 2).translate(0, 0.0205, 0), mx, my, ang));
    const g = at(new THREE.Group(), mx, my, 0.0205, ang), flapGeo = new THREE.BoxGeometry(len * 0.92, 0.0165, 0.0009).translate(0, -0.00825, 0);
    const flap = mesh(ctx, flapGeo, ctx.mats.chrome, def.id + '_flap'); g.add(flap);
    const slot = ctx.slot(def.id);
    return { object: g, update(anim) { flap.rotation.x = s * A(anim, slot); } };
}

export function buildDoor(ctx, def) {
    const len = Math.hypot(def.b[0] - def.a[0], def.b[1] - def.a[1]), g = at(new THREE.Group(), (def.a[0] + def.b[0]) / 2, (def.a[1] + def.b[1]) / 2, 0, dirAngle(def.a, def.b));
    const mat = ctx.D.m(new THREE.MeshPhysicalMaterial({ color: new THREE.Color(ctx.accent).multiplyScalar(0.6), transparent: true, opacity: 0.7, roughness: 0.2, envMap: ctx.env, emissive: new THREE.Color(ctx.accent).multiplyScalar(0.2) }));
    g.add(mesh(ctx, new THREE.BoxGeometry(len, 0.03, 0.0035).translate(0, 0.015, 0), mat, def.id));
    const slot = ctx.slot(def.id);
    return { object: g, update(anim) { g.position.y = -A(anim, slot) * 0.034; } };
}

// ---- kickback, holes, magnet, captive, diverter ----
export function buildKickback(ctx, def) {
    const r = def.r ?? 0.012;
    ctx.statics.dark.push(place(new THREE.CylinderGeometry(r * 1.05, r * 1.15, 0.0022, ctx.low ? 10 : 20).translate(0, 0.0011, 0), def.x, def.y));
    const g = at(new THREE.Group(), def.x, def.y), head = mesh(ctx, merge([new THREE.CylinderGeometry(r * 0.62, r * 0.7, 0.0075, ctx.low ? 10 : 18).translate(0, 0.0058, 0),
        new THREE.CylinderGeometry(r * 0.5, r * 0.5, 0.0016, ctx.low ? 8 : 14).translate(0, 0.0104, 0)]), ctx.mats.chrome, def.id + '_pin');
    g.add(head);
    const slot = ctx.slot(def.id);
    return { object: g, update(anim) { const k = A(anim, slot); head.position.set(0, k * 0.003, -k * 0.012); } };
}

export function buildHole(ctx, def) {
    const parts = M.holeParts(def, ctx.low);
    ctx.statics.dark.push(place(parts.dark, def.x, def.y));
    if (parts.shell) ctx.statics.chrome.push(place(parts.shell, def.x, def.y));
    const rimMat = ctx.D.m(ctx.mats.chrome.clone());
    rimMat.emissive = new THREE.Color(ctx.accent); rimMat.emissiveIntensity = 0;
    const g = at(new THREE.Group(), def.x, def.y); g.add(mesh(ctx, parts.rim, rimMat, def.id + '_rim'));
    const slot = ctx.slot(def.id);
    return { object: g, update(anim) { rimMat.emissiveIntensity = 2.5 * A(anim, slot); } };
}

export function buildMagnet(ctx, def) {
    ctx.statics.dark.push(place(new THREE.CircleGeometry(0.0125, 20).rotateX(-Math.PI / 2).translate(0, 0.0014, 0), def.x, def.y));
    const mat = ctx.D.m(new THREE.MeshBasicMaterial({ color: new THREE.Color(ctx.accent).multiplyScalar(1.6), transparent: true, opacity: 0, blending: THREE.AdditiveBlending, depthWrite: false }));
    const g = at(new THREE.Group(), def.x, def.y); g.add(mesh(ctx, new THREE.RingGeometry(0.0128, 0.0165, 28).rotateX(-Math.PI / 2).translate(0, 0.0018, 0), mat, def.id + '_field'));
    const slot = ctx.slot(def.id);
    return { object: g, update(anim) { mat.opacity = A(anim, slot) * 0.9; } };
}

export function buildCaptive(ctx, def) {
    const c = def.channel, ang = dirAngle(c.a, c.b), len = Math.hypot(c.b[0] - c.a[0], c.b[1] - c.a[1]), mx = (c.a[0] + c.b[0]) / 2, my = (c.a[1] + c.b[1]) / 2;
    for (const s of [-1, 1]) ctx.statics.chrome.push(place(new THREE.BoxGeometry(len, 0.012, 0.0018).translate(0, 0.006, s * ((c.width ?? 0.034) / 2 + 0.0009)), mx, my, ang));
    ctx.statics.dark.push(place(new THREE.BoxGeometry(len, 0.0012, (c.width ?? 0.034) + 0.004).translate(0, 0.0006, 0), mx, my, ang));
    return null;
}

export function buildDiverter(ctx, def) {
    const [x, y, h] = def.at;
    ctx.statics.chrome.push(new THREE.CylinderGeometry(0.0012, 0.0012, 0.016, 6).translate(x, h - 0.004, -y));
    const g = at(new THREE.Group(), x, y, h), flap = mesh(ctx, new THREE.BoxGeometry(0.024, 0.0022, 0.0026).translate(0.012, 0, 0), ctx.mats.steel, def.id + '_flap');
    g.add(flap);
    const slot = ctx.slot(def.id);
    return { object: g, update(anim) { flap.rotation.y = -0.15 - A(anim, slot) * 0.9; } };
}

// ---- plunger + shooter lane cover ----
export function buildPlunger(ctx) {
    const pl = { x: 0.2405, yRest: 0.0535, travel: 0.06, laneX: [0.2235, 0.2575], ...(ctx.layout.plunger || {}) }, parts = M.plungerParts(ctx.low);
    const g = new THREE.Group(), rod = mesh(ctx, merge([parts.rod, parts.tip]), ctx.mats.steel, 'plunger_rod');
    const spring = mesh(ctx, parts.spring, ctx.mats.chrome, 'plunger_spring');
    g.add(rod, spring);
    const anchor = pl.yRest - 0.0135 - pl.travel - 0.02;
    const update = (anim, snap) => {
        const pull = snap ? Math.max(0, Math.min(1, snap.plunger || 0)) : 0, tipY = pl.yRest - 0.0135 - pl.travel * pull;
        g.position.set(pl.x, 0.0135, -tipY); spring.scale.z = Math.max(0.005, tipY - anchor);
        spring.position.z = 0.0072;
    };
    update(EMPTY, null);
    // Translucent lane cover with chrome edges, above ball height.
    const x0 = pl.laneX[0] + 0.0015, x1 = pl.laneX[1] - 0.0015, y0 = 0.16, y1 = 0.7;
    const cover = mesh(ctx, new THREE.BoxGeometry(x1 - x0, 0.0014, y1 - y0).translate((x0 + x1) / 2, 0.0315, -(y0 + y1) / 2), ctx.D.m(new THREE.MeshPhysicalMaterial({
        color: '#9fb4c8', transparent: true, opacity: 0.16, roughness: 0.05, envMap: ctx.env, envMapIntensity: 1.4, depthWrite: false })), 'laneCover');
    cover.castShadow = false; cover.renderOrder = 2;
    for (const x of [x0, x1]) ctx.statics.chrome.push(new THREE.BoxGeometry(0.0013, 0.0058, y1 - y0).translate(x, 0.0293, -(y0 + y1) / 2));
    const root = new THREE.Group(); root.add(g, cover);
    return { object: root, update };
}

// ---- assembly ----
function flushStatics(ctx, root) {
    const table = { chrome: ctx.mats.chrome, steel: ctx.mats.steel, dark: ctx.mats.darkMetal, rubber: ctx.mats.rubber, glow: ctx.mats.glow, glow2: ctx.mats.glowSecondary };
    for (const [k, list] of Object.entries(ctx.statics)) {
        const geo = merge(list);
        if (!geo) continue;
        ctx.D.g(geo);
        const m = new THREE.Mesh(geo, table[k]); m.name = 'static_' + k; m.castShadow = !ctx.low && k !== 'glow'; m.receiveShadow = k !== 'glow'; m.frustumCulled = false;
        root.add(m);
    }
}

function createDisposer() {
    const own = { g: [], m: [], t: [], o: [] };
    return {
        g: x => { if (x) own.g.push(x); return x; }, m: x => { own.m.push(x); return x; }, t: x => { own.t.push(x); return x; }, o: x => { own.o.push(x); return x; },
        dispose() { own.g.forEach(g => g.dispose()); own.m.forEach(m => m.dispose()); own.t.forEach(t => t.dispose()); own.o.forEach(o => o.dispose && o.dispose()); }
    };
}

export function buildMechanics(host) {
    const { root, layout } = host;
    let low = isLow(host.tier), built = null;
    const balls = createBalls({ root, renderer: host.renderer, scene: host.scene, tableRoot: host.tableRoot, env: host.env, tier: host.tier, accent: (layout.meta || {}).accent, secondary: (layout.meta || {}).secondary });

    function build() {
        const D = createDisposer(), ctx = makeCtx({ ...host, tier: host.tier }, D);
        ctx.low = low;
        const items = [], upd = [];
        const keep = b => { if (b) { items.push(b); ctx.root.add(b.object); upd.push(b); } };
        (layout.flippers || []).forEach((f, i) => keep(buildFlipper(ctx, f, i)));
        buildStaticRubbers(ctx); buildPosts(ctx);
        const atlasItems = faceAtlasItems(ctx), atlas = atlasItems.length ? M.createAtlas(atlasItems) : null;
        let faceMat = null;
        if (atlas) {
            D.t(atlas.texture);
            faceMat = D.m(new THREE.MeshStandardMaterial({ map: atlas.texture, emissive: new THREE.Color('#ffffff'), emissiveMap: atlas.texture, emissiveIntensity: 0.32, roughness: 0.34, metalness: 0, envMap: host.env, envMapIntensity: 0.6 }));
        }
        const rollovers = [];
        for (const d of layout.devices || []) {
            if (d.type === 'sling') keep(buildSling(ctx, d));
            else if (d.type === 'bumper') keep(buildBumper(ctx, d));
            else if (d.type === 'dropBank') keep(buildDropBank(ctx, d, atlas, faceMat));
            else if (d.type === 'standup') keep(buildStandup(ctx, d, atlas, faceMat));
            else if (d.type === 'spinner') keep(buildSpinner(ctx, d));
            else if (d.type === 'rollover') rollovers.push(d);
            else if (d.type === 'gate') keep(buildGate(ctx, d));
            else if (d.type === 'door') keep(buildDoor(ctx, d));
            else if (d.type === 'kickback') keep(buildKickback(ctx, d));
            else if (d.type === 'hole') keep(buildHole(ctx, d));
            else if (d.type === 'magnet') keep(buildMagnet(ctx, d));
            else if (d.type === 'captive') keep(buildCaptive(ctx, d));
            else if (d.type === 'diverter') keep(buildDiverter(ctx, d));
        }
        if (rollovers.length) keep(buildRollovers(ctx, rollovers));
        keep(buildPlunger(ctx));
        flushStatics(ctx, ctx.root);
        let ramps = null;
        if ((layout.paths || []).length) {
            const debug = createPaths(layout.paths, { layout }).debug();
            ramps = buildRamps(debug, { mats: ctx.mats, low, env: host.env });
            ctx.root.add(ramps.group);
        }
        root.add(ctx.root);
        return { ctx, D, upd, ramps };
    }
    function teardown() {
        if (!built) return;
        root.remove(built.ctx.root);
        if (built.ramps) built.ramps.dispose();
        built.D.dispose();
        built = null;
    }
    built = build();

    return {
        update(c) {
            const snap = c.snap, anim = snap && snap.anim ? snap.anim : EMPTY, ex = { lamps: c.lamps, time: c.time || 0, dt: c.dt || 0, reduced: c.reduced };
            if (snap) for (const b of built.upd) b.update(anim, snap, ex);
            if (built.ramps) built.ramps.update(snap, ex.dt);
            balls.update(snap, ex.dt);
        },
        setQuality(t) {
            const nl = isLow(t);
            host.tier = t;
            balls.setQuality(t);
            if (nl !== low) { low = nl; teardown(); built = build(); }
        },
        dispose() { teardown(); balls.dispose(); }
    };
}

export default buildMechanics;
