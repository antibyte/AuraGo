// AuraPinball balls (plan 7 "Ball"): hard polished chrome instanced balls reflecting a contrasty studio environment (softboxes in a dark room),
// micro-scratch map so rolling is visible, orientation integrated from the physics spin (displacement fallback), contact + soft blob shadows on the playfield and on ramps.
import * as THREE from 'three';
import { rngNext } from '../config.js';
import { BALL_RADIUS } from './lighting.js';
import { REFLECT_LAYER } from './reflect.js';

const MAX_BALLS = 16;
const TELEPORT = 0.05;

function scratchTexture() {
    const W = 512, H = 256, c = document.createElement('canvas'); c.width = W; c.height = H;
    const g = c.getContext('2d'), holder = { seed: 0xBA11 };
    g.fillStyle = '#f2f4f7'; g.fillRect(0, 0, W, H);
    g.lineCap = 'round';
    for (let i = 0; i < 160; i++) {
        const x = rngNext(holder) * W, y = rngNext(holder) * H, len = 6 + rngNext(holder) * 40, a = rngNext(holder) * Math.PI;
        g.strokeStyle = rngNext(holder) > 0.5 ? `rgba(120,128,140,${0.12 + rngNext(holder) * 0.3})` : `rgba(255,255,255,${0.2 + rngNext(holder) * 0.5})`;
        g.lineWidth = 0.5 + rngNext(holder) * 0.9;
        g.beginPath(); g.moveTo(x, y); g.lineTo(x + Math.cos(a) * len, y + Math.sin(a) * len * 0.5); g.stroke();
    }
    // A faint stamped ring so even a clean roll shows rotation.
    g.strokeStyle = 'rgba(95,104,118,.55)'; g.lineWidth = 2.2;
    g.beginPath(); g.ellipse(W * 0.25, H * 0.5, 38, 38, 0, 0, Math.PI * 2); g.stroke();
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    return t;
}

function blobTexture() {
    const c = document.createElement('canvas'); c.width = c.height = 64;
    const g = c.getContext('2d'), gr = g.createRadialGradient(32, 32, 2, 32, 32, 31);
    gr.addColorStop(0, 'rgba(0,0,0,.95)'); gr.addColorStop(0.4, 'rgba(0,0,0,.82)'); gr.addColorStop(0.72, 'rgba(0,0,0,.32)'); gr.addColorStop(1, 'rgba(0,0,0,0)');
    g.fillStyle = gr; g.fillRect(0, 0, 64, 64);
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace;
    return t;
}

// Brushed chrome, not a mirror: a near-black room with soft, dim softboxes overhead, a modest window highlight,
// a dim playfield below and a few coloured insert-light spots near the horizon. Peak panel values stay below the
// bloom threshold (see TIERS in materials.js) so reflections never blow out into white glare. Returns a PMREM
// render target with a little blur (sigma 0.02) to keep the highlight edges soft.
function chromeEnvironment(renderer, accent, secondary) {
    const s = new THREE.Scene();
    s.add(new THREE.Mesh(new THREE.SphereGeometry(20, 32, 16), new THREE.MeshBasicMaterial({ color: '#020307', side: THREE.BackSide })));
    const floor = new THREE.Mesh(new THREE.CircleGeometry(19, 32), new THREE.MeshBasicMaterial({ color: new THREE.Color(accent).multiplyScalar(0.03).add(new THREE.Color('#05030a')) }));
    floor.rotation.x = -Math.PI / 2; floor.position.y = -1.2; s.add(floor);
    const box = (w, h, pos, color, k) => {
        const m = new THREE.Mesh(new THREE.PlaneGeometry(w, h), new THREE.MeshBasicMaterial({ color: new THREE.Color(color).multiplyScalar(k), side: THREE.DoubleSide }));
        m.position.set(...pos); m.lookAt(0, 0, 0); s.add(m);
    };
    box(7, 0.55, [0, 6, -2.7], '#f4f8ff', 1.5);                 // two long overhead tubes with a dark gap between
    box(7, 0.55, [0, 6.2, 0.2], '#f4f8ff', 1.5);
    box(5, 1.2, [-3.6, 4.6, 3.2], '#fff3e2', 1.25);             // warm key from the player's left
    box(1.4, 3.8, [5.6, 2.6, 1.0], '#dbe7ff', 1.0);             // cool strip on the right
    box(0.55, 0.55, [-2.6, 3.4, 1.8], '#ffffff', 2.2);          // window: a soft specular highlight, not a white flare
    box(1.6, 0.5, [-5.2, 0.9, -2.4], accent, 1.1);               // neon room strips near the horizon
    box(1.6, 0.5, [5.0, 0.8, -3.0], secondary, 1.1);
    box(2.4, 0.5, [0, 0.6, 5.5], '#ffffff', 0.6);                // faint rim at the player's end
    const pm = new THREE.PMREMGenerator(renderer);
    const target = pm.fromScene(s, 0.02);
    pm.dispose();
    s.traverse(o => { if (o.geometry) o.geometry.dispose(); if (o.material) o.material.dispose(); });
    return target;
}

export function createBalls({ root, renderer, scene, tableRoot, env, tier, accent = '#37d9ff', secondary = '#ff48c4' }) {
    let low = !!tier && tier.shadow === 0 && tier.lights <= 2;
    const group = new THREE.Group(); group.name = 'balls'; root.add(group);
    const geo = new THREE.SphereGeometry(BALL_RADIUS, low ? 20 : 36, low ? 12 : 22);
    const tex = scratchTexture(), blob = blobTexture();
    const mat = new THREE.MeshStandardMaterial({ color: '#eef2f8', map: tex, metalness: 1, roughness: 0.10, envMap: env, envMapIntensity: 0.65 });
    const mesh = new THREE.InstancedMesh(geo, mat, MAX_BALLS); mesh.count = 0; mesh.frustumCulled = false; mesh.castShadow = true; mesh.name = 'balls'; mesh.layers.enable(REFLECT_LAYER); group.add(mesh);
    // Two blob layers: a tight dark contact shadow under the ball and a soft key-light shadow that drifts away with height.
    const shGeoC = new THREE.PlaneGeometry(0.030, 0.030); shGeoC.rotateX(-Math.PI / 2);
    const shGeoS = new THREE.PlaneGeometry(0.056, 0.056); shGeoS.rotateX(-Math.PI / 2);
    const shOpts = { map: blob, transparent: true, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2 };
    const shMatC = new THREE.MeshBasicMaterial({ ...shOpts, opacity: 0.85 }), shMatS = new THREE.MeshBasicMaterial({ ...shOpts, opacity: 0.42 });
    const contact = new THREE.InstancedMesh(shGeoC, shMatC, MAX_BALLS * 2); contact.count = 0; contact.frustumCulled = false; contact.renderOrder = 1; contact.name = 'ballContactShadows'; group.add(contact);
    const soft = new THREE.InstancedMesh(shGeoS, shMatS, MAX_BALLS * 2); soft.count = 0; soft.frustumCulled = false; soft.renderOrder = 1; soft.name = 'ballShadows'; group.add(soft);

    let chromeRT = null, frames = 0;
    const state = new Map();
    const m4 = new THREE.Matrix4(), q = new THREE.Quaternion(), dq = new THREE.Quaternion(), axis = new THREE.Vector3(), pos = new THREE.Vector3(), one = new THREE.Vector3(1, 1, 1), sc = new THREE.Vector3();

    // (Re)builds the studio chrome environment; also what a lost-context recovery calls.
    function capture() {
        if (!renderer) return;
        if (chromeRT) chromeRT.dispose();
        try { chromeRT = chromeEnvironment(renderer, accent, secondary); } catch (_) { chromeRT = null; return; }
        mat.envMap = chromeRT.texture; mat.envMapIntensity = 0.6; mat.needsUpdate = true;
    }

    function update(snap, dt) {
        frames++;
        if (frames === 3) capture();
        let n = 0, s = 0, c = 0;
        const seen = new Set();
        if (snap && snap.balls) for (const b of snap.balls) {
            if (b.hidden || b.mode === 'held' || n >= MAX_BALLS) continue;
            seen.add(b.id);
            const x = b.x, z = -b.y, h = b.h || 0;
            let st = state.get(b.id);
            if (!st) { st = { x, z, h, q: new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0.3, 0.7, 0.2).normalize(), b.id * 1.7) }; state.set(b.id, st); }
            const dx = x - st.x, dz = z - st.z, dh = h - st.h, d = Math.hypot(dx, dh, dz);
            if (d > TELEPORT) { /* teleported: do not spin */ }
            else if (b.wx !== undefined) {
                // True rotation from the physics spin (physics x, y, z = three X, -Z, Y): omega3 = (wx, wz, -wy) rad/s.
                axis.set(b.wx, b.wz, -b.wy);
                const w = axis.length();
                if (w > 1e-6 && dt > 0) { dq.setFromAxisAngle(axis.multiplyScalar(1 / w), w * dt); st.q.premultiply(dq); }
            } else if (d > 1e-7) {
                axis.set(dz, 0, -dx);                          // fallback without spin data: rolling without slip on a horizontal floor
                if (axis.lengthSq() > 1e-18) { axis.normalize(); dq.setFromAxisAngle(axis, Math.hypot(dx, dz) / BALL_RADIUS); st.q.premultiply(dq); }
            }
            st.x = x; st.z = z; st.h = h;
            pos.set(x, BALL_RADIUS + h, z);
            m4.compose(pos, st.q, one); mesh.setMatrixAt(n++, m4);
            // Soft key-light shadow on the playfield (offset away from the key light, wider with height); tight contact shadow
            // under a ball that is on the field; on a ramp the contact shadow sits on its surface.
            const onRamp = b.mode === 'path' && h > 0.004, k = 1 + Math.min(1.5, h * 9);
            pos.set(x + 0.0025 + 0.5 * h, 0.0005, z - (0.002 + 0.4 * h));
            m4.compose(pos, q.identity(), sc.set(k, 1, k)); soft.setMatrixAt(s++, m4);
            if (onRamp) { pos.set(x + 0.001, h + 0.0007, z - 0.0008); m4.compose(pos, q.identity(), sc.set(1, 1, 1)); contact.setMatrixAt(c++, m4); }
            else if (h < 0.02) { pos.set(x, 0.0007, z); m4.compose(pos, q.identity(), sc.set(1, 1, 1)); contact.setMatrixAt(c++, m4); }
        }
        for (const id of state.keys()) if (!seen.has(id)) state.delete(id);
        mesh.count = n; soft.count = s; contact.count = c;
        if (n) mesh.instanceMatrix.needsUpdate = true;
        if (s) soft.instanceMatrix.needsUpdate = true;
        if (c) contact.instanceMatrix.needsUpdate = true;
    }

    return {
        group, update, capture,
        setQuality(t) { low = t.shadow === 0 && t.lights <= 2; },
        dispose() {
            root.remove(group);
            geo.dispose(); mat.dispose(); tex.dispose(); blob.dispose(); shGeoC.dispose(); shGeoS.dispose(); shMatC.dispose(); shMatS.dispose(); mesh.dispose(); contact.dispose(); soft.dispose();
            if (chromeRT) chromeRT.dispose();
        }
    };
}
