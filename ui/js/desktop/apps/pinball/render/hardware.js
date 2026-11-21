// AuraPinball visible hardware (plan R3b): translucent star posts with black rubber rings, chrome posts with screw heads,
// stamped metal lane guides (rolled top bead and mounting tabs) and one InstancedMesh for every screw head.
// Static pieces go into ctx.statics (merged per material by builders.flushStatics); the star prisms get their own mesh because
// they are translucent. Screws from other builders (plastics standoffs) are collected through addScrew before buildHardware runs.
import * as THREE from 'three';
import { merge } from './materials.js';
import { postGeometry } from './mechs.js';

export const STAR_MAX_R = 0.006;
const STAR_R = 0.0068, STAR_INNER = 0.0043, STAR_H = 0.024, RING_TUBE = 0.0024, RING_H = 0.0135;
const BEAD_R = 0.0009, TAB_STEP = 0.08, TAB = [0.006, 0.0015, 0.004], TAB_SIDE = 0.0043;
const SCREW_R = 0.0022;

// Posts this module draws (builders.buildPosts skips them): rubber posts up to STAR_MAX_R and every metal post.
export const isStarPost = p => p.render === 'post' && p.material === 'rubber' && p.r <= STAR_MAX_R;
export const isHardwarePost = p => isStarPost(p) || (p.render === 'post' && p.material === 'metal');

// Queue a screw head: playfield x, y, height of the surface it sits on, and a slot angle.
export function addScrew(ctx, x, y, h, ang = 0) {
    (ctx.screws || (ctx.screws = [])).push([x, y, h, ang]);
}

const tint = (g, c) => {
    const n = g.attributes.position.count, col = new Float32Array(n * 3);
    for (let i = 0; i < n; i++) { col[3 * i] = c.r; col[3 * i + 1] = c.g; col[3 * i + 2] = c.b; }
    g.setAttribute('color', new THREE.BufferAttribute(col, 3));
    return g;
};

function starGeometry(low) {
    const s = new THREE.Shape();
    for (let i = 0; i < 10; i++) {
        const a = Math.PI / 2 + i * Math.PI / 5, r = i % 2 ? STAR_INNER : STAR_R;
        if (i) s.lineTo(Math.cos(a) * r, Math.sin(a) * r); else s.moveTo(Math.cos(a) * r, Math.sin(a) * r);
    }
    s.closePath();
    return new THREE.ExtrudeGeometry(s, { depth: STAR_H, bevelEnabled: !low, bevelSize: 0.0004, bevelThickness: 0.0004, bevelSegments: 1 }).rotateX(-Math.PI / 2);
}

// Dome with a raised dark slot, built around its own origin (sits on y = 0); vertex colours carry the two tones.
function screwGeometry(low) {
    const dome = new THREE.SphereGeometry(SCREW_R, low ? 6 : 10, low ? 3 : 4, 0, Math.PI * 2, 0, Math.PI / 2).scale(1, 0.55, 1);
    const slot = new THREE.BoxGeometry(SCREW_R * 1.5, 0.0005, 0.0004).translate(0, SCREW_R * 0.55, 0);
    return merge([tint(dome.toNonIndexed(), new THREE.Color('#d3dae3')), tint(slot.toNonIndexed(), new THREE.Color('#1b1f26'))]);
}

function starPosts(ctx, posts, parts) {
    const seg = ctx.low ? 8 : 16, accent = new THREE.Color(ctx.accent).lerp(new THREE.Color('#ffffff'), 0.2), secondary = new THREE.Color(ctx.secondary).lerp(new THREE.Color('#ffffff'), 0.2);
    for (const p of posts) {
        // Divider posts (small) take the accent colour, the sling posts the secondary colour.
        parts.stars.push(tint(starGeometry(ctx.low).translate(p.x, 0, -p.y), /^divider/.test(p.id) ? accent : secondary));
        ctx.statics.rubber.push(new THREE.TorusGeometry(p.r + 0.0026, RING_TUBE, 6, seg).rotateX(Math.PI / 2).translate(p.x, RING_H, -p.y));
        ctx.statics.chrome.push(postGeometry(p, ctx.low).chrome);
        if (!ctx.plastics) addScrew(ctx, p.x, p.y, STAR_H, p.x * 37);
    }
}

function metalPosts(ctx, posts) {
    for (const p of posts) {
        ctx.statics.chrome.push(postGeometry(p, ctx.low).chrome);
        addScrew(ctx, p.x, p.y, 0.0216, p.y * 41);
    }
}

// Rolled bead along the top of a metal guide plus (high tiers) mounting tabs every TAB_STEP metres, alternating sides.
function laneGuide(ctx, w, beads, tabs) {
    const pts = w.points, h1 = Math.max(0.012, w.height || 0.03), seg = ctx.low ? 4 : 6;
    let acc = TAB_STEP * 0.25, side = 1;
    for (let i = 0; i < pts.length - 1; i++) {
        const a = pts[i], b = pts[i + 1], dx = b[0] - a[0], dy = b[1] - a[1], L = Math.hypot(dx, dy);
        if (L < 1e-5) continue;
        const ang = Math.atan2(dy, dx);
        beads.push(new THREE.CylinderGeometry(BEAD_R, BEAD_R, L, seg).rotateZ(Math.PI / 2).rotateY(ang).translate((a[0] + b[0]) / 2, h1 - 0.0003, -(a[1] + b[1]) / 2));
        if (i > 0) beads.push(new THREE.SphereGeometry(BEAD_R, seg, 3).translate(a[0], h1 - 0.0003, -a[1]));
        if (ctx.low) continue;
        for (; acc < L; acc += TAB_STEP, side = -side) {
            const t = acc / L, x = a[0] + dx * t, y = a[1] + dy * t, nx = -dy / L * side * TAB_SIDE, ny = dx / L * side * TAB_SIDE;
            tabs.push(new THREE.BoxGeometry(TAB[0], TAB[1], TAB[2]).rotateY(ang).translate(x + nx, TAB[1] / 2, -(y + ny)));
            addScrew(ctx, x + nx, y + ny, TAB[1], ang + Math.PI / 2);
        }
        acc -= L;
    }
}

export function buildHardware(ctx) {
    const posts = (ctx.layout.posts || []).filter(p => p.render === 'post'), parts = { stars: [] }, beads = [], tabs = [];
    starPosts(ctx, posts.filter(isStarPost), parts);
    metalPosts(ctx, posts.filter(p => p.material === 'metal'));
    for (const w of ctx.layout.walls || []) if (w.render === 'metal' && w.points && w.points.length >= 2) laneGuide(ctx, w, beads, tabs);
    for (const g of [merge(beads), merge(tabs)]) if (g) ctx.statics.chrome.push(g);
    const stars = merge(parts.stars);
    if (stars) {
        ctx.D.g(stars);
        const m = new THREE.Mesh(stars, ctx.mats.starPost); m.name = 'starPosts'; m.castShadow = !ctx.low; m.renderOrder = 1;
        ctx.root.add(m);
    }
    const list = ctx.screws || [];
    if (!list.length) return null;
    const geo = ctx.D.g(screwGeometry(ctx.low)), im = new THREE.InstancedMesh(geo, ctx.mats.screw, list.length), mat = new THREE.Matrix4(), rot = new THREE.Matrix4();
    list.forEach(([x, y, h, ang], i) => { mat.makeTranslation(x, h, -y); mat.multiply(rot.makeRotationY(ang)); im.setMatrixAt(i, mat); });
    im.name = 'screws'; im.frustumCulled = false; im.instanceMatrix.needsUpdate = true;
    ctx.D.o(im); ctx.root.add(im);
    return im;
}
