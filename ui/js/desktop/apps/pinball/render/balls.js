// AuraPinball balls (plan 7 "Ball"): chrome instanced balls reflecting a one-time CubeCamera capture of the built table,
// micro-scratch map so rolling is visible, spin derived from displacement, blob shadows on the playfield and on ramps.
import * as THREE from 'three';
import { rngNext } from '../config.js';
import { BALL_RADIUS } from './lighting.js';

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
    gr.addColorStop(0, 'rgba(0,0,0,.92)'); gr.addColorStop(0.55, 'rgba(0,0,0,.5)'); gr.addColorStop(1, 'rgba(0,0,0,0)');
    g.fillStyle = gr; g.fillRect(0, 0, 64, 64);
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace;
    return t;
}

export function createBalls({ root, renderer, scene, tableRoot, env, tier }) {
    let low = !!tier && tier.shadow === 0 && tier.lights <= 2;
    const group = new THREE.Group(); group.name = 'balls'; root.add(group);
    const geo = new THREE.SphereGeometry(BALL_RADIUS, low ? 20 : 36, low ? 12 : 22);
    const tex = scratchTexture(), blob = blobTexture();
    const mat = new THREE.MeshStandardMaterial({ color: '#c3cad4', map: tex, metalness: 1, roughness: 0.035, envMap: env, envMapIntensity: 1.5 });
    const mesh = new THREE.InstancedMesh(geo, mat, MAX_BALLS); mesh.count = 0; mesh.frustumCulled = false; mesh.castShadow = true; mesh.name = 'balls'; group.add(mesh);
    const shGeo = new THREE.PlaneGeometry(0.058, 0.058); shGeo.rotateX(-Math.PI / 2);
    const shMat = new THREE.MeshBasicMaterial({ map: blob, transparent: true, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2 });
    const shadows = new THREE.InstancedMesh(shGeo, shMat, MAX_BALLS * 2); shadows.count = 0; shadows.frustumCulled = false; shadows.renderOrder = 1; shadows.name = 'ballShadows'; group.add(shadows);

    let cubeRT = null, cubeCam = null, frames = 0;
    const state = new Map();
    const m4 = new THREE.Matrix4(), q = new THREE.Quaternion(), dq = new THREE.Quaternion(), axis = new THREE.Vector3(), pos = new THREE.Vector3(), one = new THREE.Vector3(1, 1, 1), sc = new THREE.Vector3();

    function capture() {
        if (!renderer || !scene) return;
        if (!cubeRT) {
            cubeRT = new THREE.WebGLCubeRenderTarget(256, { type: THREE.HalfFloatType, generateMipmaps: true, minFilter: THREE.LinearMipmapLinearFilter });
            cubeCam = new THREE.CubeCamera(0.01, 20, cubeRT);
        }
        tableRoot.updateWorldMatrix(true, true);
        cubeCam.position.copy(tableRoot.localToWorld(new THREE.Vector3(0, 0.06, -0.5)));
        const prev = group.visible;
        group.visible = false;
        const bg = scene.background;
        if (env) scene.background = env;                    // softbox room behind the table, so the upper hemisphere reflects light
        try { cubeCam.update(renderer, scene); } catch (_) { /* keep the room environment */ }
        group.visible = prev; scene.background = bg;
        mat.envMap = cubeRT.texture; mat.envMapIntensity = 1.5; mat.needsUpdate = true;
    }

    function update(snap, dt) {
        frames++;
        if (frames === 3 || (!low && frames === 45)) capture();
        let n = 0, s = 0;
        const seen = new Set();
        if (snap && snap.balls) for (const b of snap.balls) {
            if (b.hidden || b.mode === 'held' || n >= MAX_BALLS) continue;
            seen.add(b.id);
            const x = b.x, z = -b.y, h = b.h || 0;
            let st = state.get(b.id);
            if (!st) { st = { x, z, h, q: new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0.3, 0.7, 0.2).normalize(), b.id * 1.7) }; state.set(b.id, st); }
            const dx = x - st.x, dz = z - st.z, dh = h - st.h, d = Math.hypot(dx, dh, dz);
            if (d > TELEPORT) { /* teleported: do not spin */ } else if (d > 1e-7) {
                axis.set(dz, 0, -dx);                          // up x displacement: rolling without slip on a horizontal floor
                if (axis.lengthSq() > 1e-18) { axis.normalize(); dq.setFromAxisAngle(axis, Math.hypot(dx, dz) / BALL_RADIUS); st.q.premultiply(dq); }
            }
            st.x = x; st.z = z; st.h = h;
            pos.set(x, BALL_RADIUS + h, z);
            m4.compose(pos, st.q, one); mesh.setMatrixAt(n++, m4);
            // Blob shadow on the playfield (softer and wider with height) and, on a ramp, on its surface.
            const k = 1 + Math.min(1.5, h * 9);
            pos.set(x + 0.003 + h * 0.5, 0.0005, z - 0.002);
            m4.compose(pos, q.identity(), sc.set(k, 1, k)); shadows.setMatrixAt(s++, m4);
            if (b.mode === 'path' && h > 0.004) { pos.set(x + 0.001, h + 0.0007, z - 0.0008); m4.compose(pos, q.identity(), sc.set(1, 1, 1)); shadows.setMatrixAt(s++, m4); }
        }
        for (const id of state.keys()) if (!seen.has(id)) state.delete(id);
        mesh.count = n; shadows.count = s;
        if (n) mesh.instanceMatrix.needsUpdate = true;
        if (s) shadows.instanceMatrix.needsUpdate = true;
        void dt;
    }

    return {
        group, update, capture,
        setQuality(t) { low = t.shadow === 0 && t.lights <= 2; if (!low && frames > 3) frames = 44; },
        dispose() {
            root.remove(group);
            geo.dispose(); mat.dispose(); tex.dispose(); blob.dispose(); shGeo.dispose(); shMat.dispose(); mesh.dispose(); shadows.dispose();
            if (cubeRT) cubeRT.dispose();
        }
    };
}
