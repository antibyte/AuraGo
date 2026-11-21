// Moving physics devices of the Clockwork Observatory (plan R5 step 3): the flush orrery disc and the pendulum.
// Both draw the physics state (snapshot anim slots), so they are truthful with reduced motion too.
// buildOrrery / buildPendulum(ctx, def) -> { object, update(anim, snap, ex) } as the other builders in builders.js.
import * as THREE from 'three';
import { rngNext } from '../config.js';
import { cloneRoot } from './models.js';

const A = (anim, i) => (i >= 0 && i < anim.length ? anim[i] : 0);
const BRASS = '#d9a650';

function canvasOf(size) { const c = document.createElement('canvas'); c.width = c.height = size; return c; }

// Engraved star chart for the disc. Deliberately lopsided (a pointer wedge, a moon half and uneven constellations) so the
// turning direction and speed can be read at a glance. Canvas top is the table's +y at angle 0.
function discTexture(size, id) {
    const c = canvasOf(size), g = c.getContext('2d'), R = size / 2, rng = { seed: 0x0DE5 ^ (id.length * 977) };
    const field = g.createRadialGradient(R, R, R * 0.1, R, R, R);
    field.addColorStop(0, '#16284f'); field.addColorStop(0.8, '#09142b'); field.addColorStop(1, '#050b1c');
    g.fillStyle = field; g.beginPath(); g.arc(R, R, R, 0, Math.PI * 2); g.fill();
    // brass rim with hour marks
    const rim = g.createLinearGradient(0, 0, size, size);
    rim.addColorStop(0, '#f1cf86'); rim.addColorStop(0.5, '#a97a2c'); rim.addColorStop(1, '#e2b865');
    g.fillStyle = rim; g.beginPath(); g.arc(R, R, R, 0, Math.PI * 2); g.arc(R, R, R * 0.86, 0, Math.PI * 2, true); g.fill('evenodd');
    g.strokeStyle = 'rgba(40,24,6,.8)'; g.lineWidth = size / 170;
    g.beginPath(); g.arc(R, R, R * 0.86, 0, Math.PI * 2); g.stroke(); g.beginPath(); g.arc(R, R, R * 0.995, 0, Math.PI * 2); g.stroke();
    for (let i = 0; i < 12; i++) {
        const a = i * Math.PI / 6, major = i % 3 === 0, r0 = R * (major ? 0.875 : 0.9), r1 = R * 0.975;
        g.strokeStyle = '#2b1a06'; g.lineWidth = size / (major ? 44 : 78);
        g.beginPath(); g.moveTo(R + Math.sin(a) * r0, R - Math.cos(a) * r0); g.lineTo(R + Math.sin(a) * r1, R - Math.cos(a) * r1); g.stroke();
        if (major) { g.fillStyle = '#2b1a06'; g.beginPath(); g.arc(R + Math.sin(a) * R * 0.925, R - Math.cos(a) * R * 0.925, size / 60, 0, 7); g.fill(); }
    }
    // faint engraved rings
    g.strokeStyle = 'rgba(217,166,80,.28)'; g.lineWidth = size / 400;
    for (const f of [0.74, 0.56, 0.38]) { g.beginPath(); g.arc(R, R, R * f, 0, Math.PI * 2); g.stroke(); }
    // stars
    const pts = [];
    for (let i = 0; i < 110; i++) {
        const a = rngNext(rng) * Math.PI * 2, d = Math.sqrt(rngNext(rng)) * R * 0.8, x = R + Math.cos(a) * d, y = R + Math.sin(a) * d, s = 0.5 + rngNext(rng) * rngNext(rng) * 2.6;
        pts.push([x, y]);
        g.fillStyle = `rgba(255,${230 + (rngNext(rng) * 25 | 0)},${190 + (rngNext(rng) * 60 | 0)},${0.55 + rngNext(rng) * 0.45})`;
        g.beginPath(); g.arc(x, y, s * size / 512, 0, 7); g.fill();
    }
    g.strokeStyle = 'rgba(232,198,120,.5)'; g.lineWidth = size / 300;
    for (let k = 0; k < 5; k++) {
        g.beginPath(); let p = pts[(rngNext(rng) * pts.length) | 0]; g.moveTo(p[0], p[1]);
        for (let j = 0; j < 4; j++) { p = pts[(rngNext(rng) * pts.length) | 0]; g.lineTo(p[0], p[1]); }
        g.stroke();
    }
    // moon half and a bright pointer wedge towards "twelve o'clock": the turning cue
    g.fillStyle = 'rgba(232,238,252,.85)'; g.beginPath(); g.arc(R * 0.62, R * 1.42, R * 0.13, Math.PI / 2, Math.PI * 1.5); g.fill();
    g.fillStyle = '#e8c36f'; g.beginPath(); g.moveTo(R, R * 0.14); g.lineTo(R + R * 0.075, R * 0.46); g.lineTo(R, R * 0.4); g.lineTo(R - R * 0.075, R * 0.46); g.closePath(); g.fill();
    g.strokeStyle = '#4a3010'; g.lineWidth = size / 300; g.stroke();
    // hub collar
    const hub = g.createRadialGradient(R * 0.97, R * 0.97, 0, R, R, R * 0.14);
    hub.addColorStop(0, '#fff0c4'); hub.addColorStop(1, '#a97a2c');
    g.fillStyle = hub; g.beginPath(); g.arc(R, R, R * 0.13, 0, Math.PI * 2); g.fill();
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    return t;
}

export function brassMaterial(ctx, o = {}) {
    return ctx.D.m(new THREE.MeshStandardMaterial({ color: BRASS, metalness: 0.9, roughness: 0.32, envMap: ctx.env, envMapIntensity: 0.9, ...o }));
}

// ---- orrery disc: flush, set 0.4 mm above the playfield art; the rim ring stays still, the chart turns ----
export function buildOrrery(ctx, def) {
    const r = def.r ?? 0.05, root = new THREE.Group(), turn = new THREE.Group();
    root.position.set(def.x, 0, -def.y);
    const tex = ctx.D.t(discTexture(ctx.low ? 256 : 1024, def.id || 'orrery'));
    const disc = new THREE.Mesh(ctx.D.g(new THREE.CircleGeometry(r * 0.985, ctx.low ? 36 : 72).rotateX(-Math.PI / 2)),
        ctx.D.m(new THREE.MeshStandardMaterial({ map: tex, metalness: 0.45, roughness: 0.4, envMap: ctx.env, envMapIntensity: 0.55 })));
    disc.position.y = 0.0006; disc.name = 'orreryDisc'; disc.receiveShadow = true;
    turn.add(disc);
    const ring = new THREE.Mesh(ctx.D.g(new THREE.RingGeometry(r * 0.975, r * 1.045, ctx.low ? 36 : 72).rotateX(-Math.PI / 2)), brassMaterial(ctx, { roughness: 0.25 }));
    ring.position.y = 0.0009; ring.name = 'orreryRim';
    root.add(turn, ring);
    const slot = ctx.slot(def.id);
    return { object: root, update(anim) { turn.rotation.y = A(anim, slot); } };
}

// ---- pendulum: GLB root with `pendulum_arm`, or a procedural gantry, rod and brass bob ----
export function buildPendulum(ctx, def) {
    const px = def.pivot[0], py = def.pivot[1], pivotH = def.pivotH ?? 0.0885, L = def.length ?? 0.075, bobR = def.bobR ?? 0.011;
    const axis = def.axis || [1, 0], yaw = Math.atan2(axis[1], axis[0]);
    const root = new THREE.Group(), slot = ctx.slot(def.id);
    root.position.set(px, 0, -py); root.rotation.y = yaw; root.name = 'pendulum';
    const model = cloneRoot(ctx.host.models, 'toys', def.model || 'pendulum'), arm0 = model && model.find('pendulum_arm');
    if (arm0) {
        root.add(model.root);
        return { object: root, update(anim) { arm0.rotation.z = A(anim, slot); } };
    }
    const seg = ctx.low ? 10 : 20, brass = brassMaterial(ctx), arm = new THREE.Group();
    arm.position.y = pivotH;
    const rod = new THREE.Mesh(ctx.D.g(new THREE.CylinderGeometry(0.0009, 0.0009, L, 8).translate(0, -L / 2, 0)), brass);
    const bob = new THREE.Mesh(ctx.D.g(new THREE.SphereGeometry(bobR, seg, seg / 2).translate(0, -L, 0)), brassMaterial(ctx, { roughness: 0.2, metalness: 1 }));
    rod.castShadow = bob.castShadow = !ctx.low; bob.name = 'pendulumBob';
    arm.add(rod, bob);
    // gantry: a beam on two posts behind the swing plane, a strut to the pivot barrel
    const span = 0.024, zb = -0.017, hb = pivotH + 0.0035, parts = [
        new THREE.CylinderGeometry(0.0025, 0.0025, 2 * span + 0.004, 10).rotateZ(Math.PI / 2).translate(0, hb, zb),
        new THREE.CylinderGeometry(0.0028, 0.0034, hb, 10).translate(-span, hb / 2, zb),
        new THREE.CylinderGeometry(0.0028, 0.0034, hb, 10).translate(span, hb / 2, zb),
        new THREE.CylinderGeometry(0.0016, 0.0016, -zb, 8).rotateX(Math.PI / 2).translate(0, hb, zb / 2),
        new THREE.CylinderGeometry(0.0035, 0.0035, 0.006, 12).rotateX(Math.PI / 2).translate(0, pivotH, 0)
    ];
    for (const g of parts) ctx.D.g(g);
    const gantry = new THREE.Group();
    for (const g of parts) { const m = new THREE.Mesh(g, brass); m.castShadow = !ctx.low; gantry.add(m); }
    root.add(gantry, arm);
    return { object: root, update(anim) { arm.rotation.z = A(anim, slot); } };
}
