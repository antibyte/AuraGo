// AuraPinball 3D visual effects: pooled ball speed trails, under-ball playfield light halos, impact spark bursts
// and expanding shockwave rings. Zero per-frame allocations; respects reduced-motion and quality tiers.
import * as THREE from 'three';
import { rngNext } from '../config.js';
import { BALL_RADIUS } from './lighting.js';

const MAX_BALLS = 12;
const TRAIL_STEPS = 7;
const MAX_TRAIL = MAX_BALLS * (TRAIL_STEPS - 1);
const MAX_RINGS = 18;
const MAX_SPARKS = 96;

function glowDiscTexture() {
    const c = document.createElement('canvas'); c.width = c.height = 64;
    const g = c.getContext('2d'), gr = g.createRadialGradient(32, 32, 0, 32, 32, 32);
    gr.addColorStop(0, 'rgba(255,255,255,1)');
    gr.addColorStop(0.28, 'rgba(255,255,255,.55)');
    gr.addColorStop(0.65, 'rgba(255,255,255,.14)');
    gr.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = gr; g.fillRect(0, 0, 64, 64);
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace;
    return t;
}

function ringWaveTexture() {
    const c = document.createElement('canvas'); c.width = c.height = 128;
    const g = c.getContext('2d'), gr = g.createRadialGradient(64, 64, 18, 64, 64, 62);
    gr.addColorStop(0, 'rgba(255,255,255,0)');
    gr.addColorStop(0.58, 'rgba(255,255,255,.12)');
    gr.addColorStop(0.82, 'rgba(255,255,255,.95)');
    gr.addColorStop(0.93, 'rgba(255,255,255,.45)');
    gr.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = gr; g.fillRect(0, 0, 128, 128);
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace;
    return t;
}

function deviceCenters(layout) {
    const map = new Map();
    for (const d of (layout && layout.devices) || []) {
        if (Number.isFinite(d.x) && Number.isFinite(d.y)) map.set(d.id, [d.x, d.y, 0.012, d.color]);
        else if (Array.isArray(d.a) && Array.isArray(d.b)) map.set(d.id, [(d.a[0] + d.b[0]) / 2, (d.a[1] + d.b[1]) / 2, 0.016, d.color]);
        if (d.type === 'dropBank' && Array.isArray(d.targets)) {
            for (const t of d.targets) map.set(t.id, [t.x, t.y, 0.014, d.color]);
        }
    }
    for (const p of (layout && layout.paths) || []) {
        const pts = p.points || [];
        if (pts.length) {
            const first = pts[0], last = pts[pts.length - 1], mid = pts[Math.floor(pts.length * 0.45)];
            map.set(p.id + ':enter', [first[0], first[1], (first[2] || 0) + 0.012, p.color]);
            map.set(p.id, [mid[0], mid[1], (mid[2] || 0) + 0.018, p.color]);
            map.set(p.id + ':exit', [last[0], last[1], (last[2] || 0) + 0.012, p.color]);
        }
    }
    return map;
}

export function createEffects({ root, layout, palette = {}, tier }) {
    let low = !!tier && tier.shadow === 0 && tier.lights <= 2;
    const group = new THREE.Group(); group.name = 'playfieldFx'; root.add(group);
    const rng = { seed: 0xF1A54E };
    const accent = new THREE.Color(palette.accent || '#37d9ff');
    const secondary = new THREE.Color(palette.secondary || '#ff48c4');
    const gold = new THREE.Color('#ffd86b');
    const white = new THREE.Color('#ffffff');
    const centers = deviceCenters(layout);
    const lampColors = new Map();
    for (const l of (layout && layout.lamps) || []) {
        if (l.device && l.color) lampColors.set(l.device, new THREE.Color(l.color));
        if (l.id && l.color) lampColors.set(l.id, new THREE.Color(l.color));
    }

    const discTex = glowDiscTexture(), ringTex = ringWaveTexture();

    // 1. Under-ball dynamic bounce halos on the playfield / ramps.
    const haloGeo = new THREE.PlaneGeometry(0.082, 0.082); haloGeo.rotateX(-Math.PI / 2);
    const haloMat = new THREE.MeshBasicMaterial({ map: discTex, color: '#ffffff', transparent: true, opacity: 0.55, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false });
    const halos = new THREE.InstancedMesh(haloGeo, haloMat, MAX_BALLS);
    halos.count = 0; halos.frustumCulled = false; halos.renderOrder = 2; halos.name = 'ballHalos';
    for (let i = 0; i < MAX_BALLS; i++) halos.setColorAt(i, accent);
    group.add(halos);

    // 2. Tapered speed trails behind fast balls.
    const trailGeo = new THREE.PlaneGeometry(1, 1); trailGeo.rotateX(-Math.PI / 2);
    const trailMat = new THREE.MeshBasicMaterial({ map: discTex, color: '#ffffff', transparent: true, opacity: 0.72, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false });
    const trails = new THREE.InstancedMesh(trailGeo, trailMat, MAX_TRAIL);
    trails.count = 0; trails.frustumCulled = false; trails.renderOrder = 3; trails.name = 'ballTrails';
    for (let i = 0; i < MAX_TRAIL; i++) trails.setColorAt(i, accent);
    group.add(trails);

    // 3. Expanding shockwave rings on bumpers, scoops, magnet & jackpots.
    const ringGeo = new THREE.PlaneGeometry(1, 1); ringGeo.rotateX(-Math.PI / 2);
    const ringMat = new THREE.MeshBasicMaterial({ map: ringTex, color: '#ffffff', transparent: true, opacity: 0.9, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false });
    const rings = new THREE.InstancedMesh(ringGeo, ringMat, MAX_RINGS);
    rings.count = 0; rings.frustumCulled = false; rings.renderOrder = 4; rings.name = 'impactRings';
    for (let i = 0; i < MAX_RINGS; i++) rings.setColorAt(i, accent);
    group.add(rings);

    // 4. Pooled additive spark particles (world-space physics with playfield bounce).
    const sparkPos = new Float32Array(MAX_SPARKS * 3);
    const sparkCol = new Float32Array(MAX_SPARKS * 3);
    const sparkGeo = new THREE.BufferGeometry();
    sparkGeo.setAttribute('position', new THREE.BufferAttribute(sparkPos, 3));
    sparkGeo.setAttribute('color', new THREE.BufferAttribute(sparkCol, 3));
    const sparkMat = new THREE.PointsMaterial({ map: discTex, size: 0.013, vertexColors: true, transparent: true, opacity: 0.95, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false, sizeAttenuation: true });
    const sparks = new THREE.Points(sparkGeo, sparkMat);
    sparks.frustumCulled = false; sparks.renderOrder = 7; sparks.name = 'impactSparks';
    group.add(sparks);

    const ringPool = Array.from({ length: MAX_RINGS }, () => ({ life: 0, dur: 0.3, x: 0, h: 0, z: 0, r0: 0.015, r1: 0.08, c: new THREE.Color() }));
    const sparkPool = Array.from({ length: MAX_SPARKS }, () => ({ life: 0, dur: 0.4, x: 0, y: -2, z: 0, vx: 0, vy: 0, vz: 0, r: 1, g: 1, b: 1 }));
    let ringHead = 0, sparkHead = 0;
    const history = new Map();

    const m4 = new THREE.Matrix4(), q = new THREE.Quaternion(), pos = new THREE.Vector3(), scl = new THREE.Vector3(), tmpC = new THREE.Color();

    function spawnRing(x, y, h, color, r0 = 0.018, r1 = 0.088, dur = 0.30) {
        const slot = ringPool[ringHead]; ringHead = (ringHead + 1) % MAX_RINGS;
        slot.life = dur; slot.dur = dur; slot.x = x; slot.h = Math.max(0.0022, h); slot.z = -y;
        slot.r0 = r0; slot.r1 = r1; slot.c.copy(color);
    }

    function spawnSparks(x, y, h, color, count, speed = 0.65, up = 0.45, dur = 0.38) {
        const n = low ? Math.max(2, count >> 1) : count;
        for (let i = 0; i < n; i++) {
            const s = sparkPool[sparkHead]; sparkHead = (sparkHead + 1) % MAX_SPARKS;
            const ang = rngNext(rng) * Math.PI * 2, sp = speed * (0.35 + rngNext(rng) * 0.85);
            s.life = dur * (0.65 + rngNext(rng) * 0.55); s.dur = s.life;
            s.x = x + (rngNext(rng) - 0.5) * 0.008;
            s.y = Math.max(0.006, h + 0.006);
            s.z = -y + (rngNext(rng) - 0.5) * 0.008;
            s.vx = Math.cos(ang) * sp;
            s.vy = up * (0.45 + rngNext(rng) * 0.85);
            s.vz = Math.sin(ang) * sp;
            tmpC.copy(color).lerp(white, 0.25 + rngNext(rng) * 0.4);
            s.r = tmpC.r; s.g = tmpC.g; s.b = tmpC.b;
        }
    }

    function locate(ev, keySuffix = '') {
        const key = String(ev.id || '') + keySuffix;
        const hit = centers.get(key) || centers.get(String(ev.id || ''));
        const x = Number.isFinite(ev.x) ? ev.x : (hit ? hit[0] : 0);
        const y = Number.isFinite(ev.y) ? ev.y : (hit ? hit[1] : 0.55);
        const h = hit ? hit[2] : 0.012;
        const c = lampColors.get(ev.id) || (hit && hit[3] ? tmpC.set(hit[3]).clone() : null);
        return { x, y, h, color: c };
    }

    function hit(ev) {
        if (!ev || !ev.type) return;
        const { x, y, h, color } = locate(ev, ev.type === 'pathEnter' ? ':enter' : '');
        switch (ev.type) {
        case 'bumper':
            spawnRing(x, y, 0.0025, color || secondary, 0.022, 0.105, 0.28);
            spawnSparks(x, y, 0.018, color || secondary, 14, 0.85, 0.55, 0.36);
            break;
        case 'sling':
        case 'kickback':
            spawnRing(x, y, 0.0025, ev.type === 'kickback' ? gold : secondary, 0.016, 0.078, 0.24);
            spawnSparks(x, y, 0.014, ev.type === 'kickback' ? gold : secondary, 11, 0.78, 0.45, 0.32);
            break;
        case 'drop':
        case 'standup':
        case 'captive':
            spawnSparks(x, y, h, color || accent, 9, 0.62, 0.42, 0.30);
            spawnRing(x, y, 0.0024, color || accent, 0.012, 0.056, 0.22);
            break;
        case 'hole':
            spawnRing(x, y, 0.0026, color || gold, 0.058, 0.012, 0.26);
            spawnSparks(x, y, 0.012, color || gold, 10, 0.45, 0.50, 0.34);
            break;
        case 'eject':
            spawnRing(x, y, 0.0026, accent, 0.016, 0.092, 0.30);
            spawnSparks(x, y, 0.020, accent, 14, 0.72, 0.75, 0.42);
            break;
        case 'pathEnter':
            spawnSparks(x, y, h, color || accent, 8, 0.50, 0.42, 0.28);
            break;
        case 'pathMade':
            spawnRing(x, y, h, color || gold, 0.018, 0.095, 0.34);
            spawnSparks(x, y, h, color || gold, 16, 0.82, 0.68, 0.46);
            break;
        case 'spinner':
            if ((ev.rate ?? 15) > 12) spawnSparks(x, y, 0.022, white, 4, 0.46, 0.35, 0.20);
            break;
        case 'magnet':
            if (ev.state === 'grab' || ev.state === 'throw') {
                spawnRing(x, y, 0.0028, ev.state === 'throw' ? gold : accent, 0.015, 0.11, 0.36);
                spawnSparks(x, y, 0.012, ev.state === 'throw' ? gold : accent, 12, 0.7, 0.5, 0.38);
            }
            break;
        case 'clack':
            spawnSparks(x, y, 0.014, gold, 8, 0.68, 0.44, 0.26);
            break;
        case 'impact':
            if ((ev.speed ?? ev.vn ?? 0) >= 1.25) spawnSparks(x, y, 0.012, ev.material === 'rubber' ? secondary : white, 5, 0.48, 0.30, 0.20);
            break;
        default:
            break;
        }
    }

    function celebrate(spots, colorA, colorB, ringsToo = true) {
        for (let i = 0; i < spots.length; i++) {
            const [x, y, h] = spots[i], c = i % 2 ? colorB : colorA;
            if (ringsToo) spawnRing(x, y, h || 0.003, c, 0.020, 0.135, 0.44);
            spawnSparks(x, y, (h || 0.015) + 0.01, c, 18, 0.95, 0.88, 0.58);
        }
    }

    function command(cmd) {
        if (!cmd) return;
        if (cmd.type === 'cue') {
            if (cmd.name === 'jackpot') celebrate([[-0.09, 0.72, 0.02], [0.09, 0.72, 0.02], [0, 0.82, 0.03]], gold, accent);
            else if (cmd.name === 'superJackpot' || cmd.name === 'wizard') celebrate([[-0.12, 0.68, 0.02], [0.12, 0.68, 0.02], [0, 0.78, 0.04], [0, 0.58, 0.02]], gold, secondary);
            else if (cmd.name === 'multiball') celebrate([[-0.08, 0.64, 0.02], [0.08, 0.64, 0.02], [0, 0.84, 0.03]], secondary, accent);
            else if (cmd.name === 'surprise' || cmd.name === 'skillShot' || cmd.name === 'extraBall') celebrate([[0, 0.68, 0.025], [-0.10, 0.76, 0.02], [0.10, 0.76, 0.02]], gold, white);
        } else if (cmd.type === 'dmd' && cmd.scene === 'surprise') {
            celebrate([[-0.09, 0.66, 0.02], [0.09, 0.66, 0.02], [0, 0.80, 0.03]], gold, secondary);
        }
    }

    function clearTransient() {
        halos.count = 0; trails.count = 0; rings.count = 0;
        for (const r of ringPool) r.life = 0;
        for (let i = 0; i < MAX_SPARKS; i++) { sparkPool[i].life = 0; sparkPos[3 * i + 1] = -2; sparkCol[3 * i] = sparkCol[3 * i + 1] = sparkCol[3 * i + 2] = 0; }
        sparkGeo.attributes.position.needsUpdate = true;
        sparkGeo.attributes.color.needsUpdate = true;
        history.clear();
    }

    function update({ snap, dt = 0, reduced = false } = {}) {
        if (reduced) { if (halos.count || trails.count || rings.count) clearTransient(); group.visible = false; return; }
        group.visible = true;

        // Balls: bounce halos + motion trails.
        const balls = (snap && snap.balls) || [];
        const multi = balls.filter(b => !b.hidden && b.mode !== 'held').length >= 2;
        const baseTint = multi ? gold : accent;
        const seen = new Set();
        let hc = 0, tc = 0;
        for (const b of balls) {
            if (b.hidden || b.mode === 'held' || hc >= MAX_BALLS) continue;
            seen.add(b.id);
            const x = b.x, y = b.y, h = b.h || 0, z = -y;
            const speed = Math.hypot(b.vx || 0, b.vy || 0);
            const boost = 0.75 + Math.min(0.85, speed * 0.22);
            pos.set(x, (b.mode === 'path' && h > 0.004 ? h : 0) + 0.0014, z);
            m4.compose(pos, q.identity(), scl.set(boost, 1, boost));
            halos.setMatrixAt(hc, m4);
            tmpC.copy(baseTint).lerp(secondary, Math.min(0.6, speed * 0.12)).multiplyScalar(0.35 + Math.min(0.55, speed * 0.16));
            halos.setColorAt(hc, tmpC);
            hc++;

            if (!low) {
                let pts = history.get(b.id);
                if (!pts) { pts = []; history.set(b.id, pts); }
                if (dt > 0) {
                    if (pts.length && Math.hypot(x - pts[0][0], z - pts[0][2]) > 0.09) pts.length = 0;
                    if (!pts.length || Math.hypot(x - pts[0][0], z - pts[0][2]) > 0.0035) {
                        pts.unshift([x, BALL_RADIUS * 0.72 + h, z, speed]);
                        if (pts.length > TRAIL_STEPS) pts.pop();
                    }
                }
                if (speed > 0.95 && pts.length >= 2) {
                    for (let k = 0; k < pts.length - 1 && tc < MAX_TRAIL; k++) {
                        const a = pts[k], p2 = pts[k + 1];
                        const dx = a[0] - p2[0], dz = a[2] - p2[2], segLen = Math.hypot(dx, dz);
                        if (segLen < 0.001 || segLen > 0.08) continue;
                        const fade = (1 - (k + 0.5) / pts.length) * Math.min(1, (speed - 0.85) / 2.2);
                        const w = BALL_RADIUS * (1.28 - k * 0.14);
                        pos.set((a[0] + p2[0]) * 0.5, (a[1] + p2[1]) * 0.5, (a[2] + p2[2]) * 0.5);
                        q.setFromAxisAngle(new THREE.Vector3(0, 1, 0), Math.atan2(-dz, dx));
                        m4.compose(pos, q, scl.set(segLen * 1.45, 1, w));
                        trails.setMatrixAt(tc, m4);
                        tmpC.copy(baseTint).lerp(white, 0.35 * fade).multiplyScalar(0.52 * fade);
                        trails.setColorAt(tc, tmpC);
                        tc++;
                    }
                }
            }
        }
        for (const id of history.keys()) if (!seen.has(id)) history.delete(id);
        halos.count = hc;
        if (hc) { halos.instanceMatrix.needsUpdate = true; if (halos.instanceColor) halos.instanceColor.needsUpdate = true; }
        trails.count = low ? 0 : tc;
        if (trails.count) { trails.instanceMatrix.needsUpdate = true; if (trails.instanceColor) trails.instanceColor.needsUpdate = true; }

        // Shockwave rings.
        let rc = 0;
        for (let i = 0; i < MAX_RINGS; i++) {
            const r = ringPool[i];
            if (r.life <= 0) continue;
            r.life -= dt;
            if (r.life <= 0) continue;
            const u = 1 - r.life / r.dur, ease = 1 - (1 - u) * (1 - u);
            const size = (r.r0 + (r.r1 - r.r0) * ease) * 2;
            pos.set(r.x, r.h, r.z);
            m4.compose(pos, q.identity(), scl.set(size, 1, size));
            rings.setMatrixAt(rc, m4);
            tmpC.copy(r.c).multiplyScalar((1 - u) * 1.35);
            rings.setColorAt(rc, tmpC);
            rc++;
        }
        rings.count = rc;
        if (rc) { rings.instanceMatrix.needsUpdate = true; if (rings.instanceColor) rings.instanceColor.needsUpdate = true; }

        // Sparks.
        let activeSparks = false;
        for (let i = 0; i < MAX_SPARKS; i++) {
            const s = sparkPool[i];
            if (s.life <= 0) { sparkPos[3 * i + 1] = -2; sparkCol[3 * i] = sparkCol[3 * i + 1] = sparkCol[3 * i + 2] = 0; continue; }
            s.life -= dt;
            if (s.life <= 0) { sparkPos[3 * i + 1] = -2; sparkCol[3 * i] = sparkCol[3 * i + 1] = sparkCol[3 * i + 2] = 0; continue; }
            activeSparks = true;
            if (dt > 0) {
                s.vy -= 2.6 * dt;
                s.x += s.vx * dt; s.y += s.vy * dt; s.z += s.vz * dt;
                if (s.y < 0.003) { s.y = 0.003; s.vy = Math.abs(s.vy) * 0.42; s.vx *= 0.78; s.vz *= 0.78; }
            }
            const f = Math.max(0, s.life / s.dur);
            sparkPos[3 * i] = s.x; sparkPos[3 * i + 1] = s.y; sparkPos[3 * i + 2] = s.z;
            sparkCol[3 * i] = s.r * f * 1.4; sparkCol[3 * i + 1] = s.g * f * 1.4; sparkCol[3 * i + 2] = s.b * f * 1.4;
        }
        sparks.visible = activeSparks;
        sparkGeo.attributes.position.needsUpdate = true;
        sparkGeo.attributes.color.needsUpdate = true;
    }

    return {
        group, hit, command, update,
        setQuality(t) { low = !!t && t.shadow === 0 && t.lights <= 2; if (low) trails.count = 0; },
        dispose() {
            root.remove(group);
            for (const g of [haloGeo, trailGeo, ringGeo, sparkGeo]) g.dispose();
            for (const m of [haloMat, trailMat, ringMat, sparkMat]) m.dispose();
            for (const t of [discTex, ringTex]) t.dispose();
            for (const im of [halos, trails, rings]) im.dispose();
        }
    };
}

export default createEffects;
