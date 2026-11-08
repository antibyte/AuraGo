// Cyber Neon Metropolis 3D toys (plan 6.1 / T16): skyline along the top arc, holographic Data Core spire, subway cap
// and flying traffic. buildToys(THREE, kit, layout) -> {group, update(dt, t, snap, toyState, lamps), dispose}.
// Local frame is the table root: X = x, Y = h (above the playfield), Z = -y. Draw calls: high 14, low 11.
import { rngNext } from '../../config.js';

const ARC = { cx: 0, cy: 0.8125, r: 0.225 };
const SIGN_WORDS = ['BAR', 'NEON', 'CYBER', 'HOTEL', 'AURA', 'NOODLE'];
const SIGN_COLORS = ['#ff48c4', '#37d9ff', '#ff48c4', '#37d9ff', '#ff48c4', '#37d9ff'];

function canvas(w, h) { const c = document.createElement('canvas'); c.width = w; c.height = h; return c; }

// Facade atlas: 8 columns (one per building style), 12 window rows covering 0.06 m of height.
function facadeAtlas(THREE) {
    const c = canvas(512, 512), g = c.getContext('2d'), rng = { seed: 0xFACADE };
    g.fillStyle = '#07061a'; g.fillRect(0, 0, 512, 512);
    const tints = ['#37d9ff', '#ff48c4', '#ffb347', '#d8f0ff', '#a98bff', '#37d9ff', '#ff48c4', '#ffd24a'];
    for (let s = 0; s < 8; s++) {
        const x0 = s * 64;
        g.fillStyle = s % 2 ? '#0d0a26' : '#0a0c24'; g.fillRect(x0, 0, 64, 512);
        for (let r = 0; r < 12; r++) for (let col = 0; col < 4; col++) {
            const lit = rngNext(rng) < 0.55;
            if (!lit) { g.fillStyle = 'rgba(40,60,110,.35)'; g.fillRect(x0 + 6 + col * 14, 6 + r * 42, 8, 22); continue; }
            g.fillStyle = rngNext(rng) < 0.7 ? tints[s] : tints[(s + 3) % 8];
            g.globalAlpha = 0.65 + rngNext(rng) * 0.35; g.fillRect(x0 + 6 + col * 14, 6 + r * 42, 8, 22); g.globalAlpha = 1;
        }
        g.fillStyle = tints[s]; g.globalAlpha = 0.8; g.fillRect(x0, 0, 64, 4); g.globalAlpha = 1;
    }
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    t.wrapS = THREE.ClampToEdgeWrapping; t.wrapT = THREE.RepeatWrapping;
    return t;
}

function signAtlas(THREE) {
    const c = canvas(768, 128), g = c.getContext('2d');
    g.textAlign = 'center'; g.textBaseline = 'middle';
    SIGN_WORDS.forEach((w, i) => {
        const x = i * 128 + 64, col = SIGN_COLORS[i];
        g.fillStyle = 'rgba(0,0,0,.0)';
        g.strokeStyle = col; g.lineWidth = 5; g.shadowColor = col; g.shadowBlur = 14;
        g.strokeRect(x - 58, 28, 116, 72);
        g.font = '900 38px "Segoe UI",Arial,sans-serif'; g.fillStyle = '#fff'; g.shadowBlur = 16;
        g.fillText(w, x, 66, 100);
    });
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace;
    return t;
}

function buildSkyline(THREE, low) {
    const rng = { seed: 0xB11D5 }, parts = [], N = 24, maxH = 0.06;
    for (let i = 0; i < N; i++) {
        const th = (35 + (110 * i) / (N - 1)) * Math.PI / 180, mid = 1 - Math.abs(i - (N - 1) / 2) / ((N - 1) / 2);
        const w = 0.0165, d = 0.013 + rngNext(rng) * 0.006, hgt = 0.020 + rngNext(rng) * 0.018 + mid * 0.022;
        const x = ARC.cx + ARC.r * Math.cos(th), y = ARC.cy + ARC.r * Math.sin(th);
        const g = new THREE.BoxGeometry(w, hgt, d, 1, 1, 1);
        const cell = Math.floor(rngNext(rng) * 8), uv = g.attributes.uv;
        for (let k = 0; k < uv.count; k++) {
            uv.setXY(k, (cell + uv.getX(k) * 0.98 + 0.01) / 8, uv.getY(k) * hgt / maxH);
        }
        g.rotateY(th + Math.PI / 2); g.translate(x, hgt / 2, -y);
        parts.push(g);
    }
    // Rooftop antenna spikes are cheap triangles merged in the same mesh (use the dark top of cell 0).
    const geos = parts;
    const pos = [], uvs = [], idx = [];
    let off = 0;
    for (const g of geos) {
        const p = g.attributes.position, u = g.attributes.uv, n = g.attributes.normal, ix = g.index;
        for (let k = 0; k < p.count; k++) { pos.push(p.getX(k), p.getY(k), p.getZ(k)); uvs.push(u.getX(k), u.getY(k)); }
        for (let k = 0; k < ix.count; k++) idx.push(ix.getX(k) + off);
        off += p.count; void n; g.dispose();
    }
    const merged = new THREE.BufferGeometry();
    merged.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
    merged.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
    merged.setIndex(idx);
    void low;
    return merged;
}

function buildSigns(THREE) {
    const pos = [], uvs = [], col = [], idx = [], sx = [-0.150, -0.090, -0.030, 0.030, 0.090, 0.150];
    sx.forEach((x, i) => {
        const th = Math.acos(x / ARC.r * 0.99), y = ARC.cy + ARC.r * Math.sin(th) - 0.012, w = 0.0175, hgt = 0.0125, h0 = 0.012 + (i % 3) * 0.008;
        const k = pos.length / 3, tilt = 0.22;
        const corners = [[-w, 0], [w, 0], [w, 1], [-w, 1]];
        for (const [cx, up] of corners) { pos.push(x + cx, h0 + up * hgt * Math.cos(tilt), -(y - up * hgt * Math.sin(tilt))); col.push(0, 0, 0); }
        uvs.push(i / 6, 0, (i + 1) / 6, 0, (i + 1) / 6, 1, i / 6, 1);
        idx.push(k, k + 1, k + 2, k, k + 2, k + 3);
    });
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
    g.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
    g.setAttribute('color', new THREE.Float32BufferAttribute(col, 3));
    g.setIndex(idx);
    return g;
}

// Twisted hexagonal prism: radius tapers to the top, rotation grows with height.
function prismGeometry(THREE, rBot, rTop, height, twist) {
    const g = new THREE.CylinderGeometry(rTop, rBot, height, 6, 28, true), p = g.attributes.position;
    for (let i = 0; i < p.count; i++) {
        const y = p.getY(i), t = y / height + 0.5, a = twist * t, x = p.getX(i), z = p.getZ(i);
        p.setXYZ(i, x * Math.cos(a) - z * Math.sin(a), y, x * Math.sin(a) + z * Math.cos(a));
    }
    g.computeVertexNormals();
    return g;
}

const HOLO_VERT = `
varying vec3 vN; varying vec3 vP; varying vec3 vV;
void main(){ vec4 mv = modelViewMatrix * vec4(position,1.0); vN = normalize(normalMatrix * normal); vP = position; vV = normalize(-mv.xyz); gl_Position = projectionMatrix * mv; }`;
const HOLO_FRAG = `
uniform vec3 uColor; uniform float uTime; uniform float uPulse; uniform float uHeight;
varying vec3 vN; varying vec3 vP; varying vec3 vV;
void main(){
  float t = vP.y / uHeight + 0.5;
  float fres = pow(1.0 - abs(dot(normalize(vN), normalize(vV))), 1.6);
  float scan = 0.55 + 0.45 * sin(vP.y * 900.0 - uTime * 6.0);
  float rise = smoothstep(0.0, 0.18, t) * (1.0 - smoothstep(0.85, 1.0, t));
  float wave = smoothstep(0.92, 1.0, sin(t * 9.0 - uTime * 2.2));
  float a = (0.45 + 0.9 * fres + 0.5 * wave + 0.6 * uPulse) * scan * rise;
  vec3 c = uColor * (1.1 + 1.4 * uPulse) + vec3(0.25) * wave;
  gl_FragColor = vec4(c * a, a);
}`;

function colorOf(THREE, v, fallback) { try { return new THREE.Color(v || fallback); } catch { return new THREE.Color(fallback); } }

export function buildToys(THREE, kit, layout) {
    const low = !!(kit && kit.W && kit.W <= 1024);
    const group = new THREE.Group(); group.name = 'neonToys';
    const disposables = [];
    const own = o => { disposables.push(o); return o; };
    const find = id => ((layout && layout.toys) || []).find(t => t.id === id);

    // ---- skyline: one merged mesh of 24 buildings + one sign mesh (per-vertex brightness from the skyline lamps) ----
    const facade = own(facadeAtlas(THREE));
    const cityMat = own(new THREE.MeshBasicMaterial({ map: facade, color: new THREE.Color(1.25, 1.25, 1.35) }));
    const city = new THREE.Mesh(own(buildSkyline(THREE, low)), cityMat); city.name = 'skylineBuildings'; city.frustumCulled = false;
    group.add(city);
    const signGeo = own(buildSigns(THREE)), signTex = own(signAtlas(THREE));
    const signMat = own(new THREE.MeshBasicMaterial({ map: signTex, vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, side: THREE.DoubleSide }));
    const signs = new THREE.Mesh(signGeo, signMat); signs.name = 'skylineSigns'; signs.frustumCulled = false; signs.renderOrder = 4;
    group.add(signs);
    const signCol = signGeo.attributes.color, signLamp = [0, 1, 2, 3, 4, 5].map(i => 'skyline' + (i + 1));
    const signLevel = new Float32Array(6);

    // ---- spire ----
    const sp = find('spire') || { x: 0, y: 0.7 };
    const spire = new THREE.Group(); spire.position.set(sp.x, 0, -sp.y); group.add(spire);
    const baseMat = own(new THREE.MeshStandardMaterial({ color: '#10121c', metalness: 0.9, roughness: 0.3, emissive: '#000000' }));
    const base = new THREE.Mesh(own(new THREE.CylinderGeometry(0.023, 0.026, 0.016, low ? 20 : 32)), baseMat); base.position.y = 0.008; spire.add(base);
    const emitterMat = own(new THREE.MeshBasicMaterial({ color: '#37d9ff', transparent: true, opacity: 0.95, blending: THREE.AdditiveBlending, depthWrite: false }));
    const emitter = new THREE.Mesh(own(new THREE.TorusGeometry(0.0195, 0.0015, 6, low ? 28 : 48)), emitterMat);
    emitter.rotation.x = Math.PI / 2; emitter.position.y = 0.0165; spire.add(emitter);
    const HH = 0.115, holoU = { uColor: { value: new THREE.Color('#37d9ff') }, uTime: { value: 0 }, uPulse: { value: 0 }, uHeight: { value: HH } };
    const holoMat = own(new THREE.ShaderMaterial({ uniforms: holoU, vertexShader: HOLO_VERT, fragmentShader: HOLO_FRAG, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, side: THREE.DoubleSide }));
    const prism = new THREE.Mesh(own(prismGeometry(THREE, 0.021, 0.006, HH, 2.6)), holoMat); prism.position.y = 0.017 + HH / 2; prism.renderOrder = 5; spire.add(prism);
    const crystalMat = own(new THREE.MeshBasicMaterial({ color: '#ffffff', transparent: true, opacity: 0.9, blending: THREE.AdditiveBlending, depthWrite: false, wireframe: true }));
    const crystal = new THREE.Mesh(own(new THREE.OctahedronGeometry(0.0085, 0)), crystalMat); crystal.position.y = 0.017 + HH * 0.5; crystal.renderOrder = 6; spire.add(crystal);
    const rings = [];
    for (let i = 0; i < (low ? 2 : 3); i++) {
        const mat = own(new THREE.MeshBasicMaterial({ color: '#37d9ff', transparent: true, opacity: 0.8, blending: THREE.AdditiveBlending, depthWrite: false }));
        const ring = new THREE.Mesh(own(new THREE.TorusGeometry(0.0125 + (1 - i) * 0.004 * (i === 1 ? 0 : 1) + i * 0.002, 0.0005, 5, low ? 28 : 56)), mat);
        ring.rotation.x = Math.PI / 2; ring.position.y = 0.04 + i * 0.03; ring.renderOrder = 6; spire.add(ring); rings.push(ring);
    }
    let motes = null, moteBase = null;
    if (!low) {
        const n = 40, p = new Float32Array(n * 3), rng = { seed: 0x30735 };
        moteBase = new Float32Array(n * 3);
        for (let i = 0; i < n; i++) { moteBase[i * 3] = rngNext(rng) * Math.PI * 2; moteBase[i * 3 + 1] = rngNext(rng); moteBase[i * 3 + 2] = 0.006 + rngNext(rng) * 0.016; }
        const g = own(new THREE.BufferGeometry()); g.setAttribute('position', new THREE.BufferAttribute(p, 3));
        const m = own(new THREE.PointsMaterial({ color: '#9ff0ff', size: 0.0022, transparent: true, opacity: 0.9, blending: THREE.AdditiveBlending, depthWrite: false }));
        motes = new THREE.Points(g, m); motes.frustumCulled = false; motes.renderOrder = 6; spire.add(motes);
    }

    // ---- subway cap over the vault feed ----
    const sw = find('subway') || { x: -0.172, y: 0.9 };
    const sub = new THREE.Group(); sub.position.set(-0.166, 0.043, -0.897); sub.rotation.y = -Math.atan2(0.015, -0.022) + Math.PI;
    group.add(sub);
    const capMat = own(new THREE.MeshStandardMaterial({ color: '#2a2f42', metalness: 0.85, roughness: 0.32, side: THREE.DoubleSide }));
    const arch = new THREE.Mesh(own(new THREE.CylinderGeometry(0.021, 0.021, 0.05, 20, 1, true, 0, Math.PI)), capMat);
    arch.rotation.z = Math.PI / 2; sub.add(arch);
    const stripMat = own(new THREE.MeshBasicMaterial({ color: '#a98bff' }));
    const strip = new THREE.Mesh(own(new THREE.BoxGeometry(0.046, 0.0016, 0.004)), stripMat); strip.position.set(0, 0.0212, 0); sub.add(strip);
    void sw;

    // ---- traffic: light trails drifting above the right ramp ----
    const rr = ((layout && layout.paths) || []).find(p => p.id === 'rightRamp');
    let traffic = null, curve = null;
    const TN = low ? 6 : 12, tmp = new THREE.Object3D();
    if (rr && rr.points) {
        curve = new THREE.CatmullRomCurve3(rr.points.map(q => new THREE.Vector3(q[0], q[2] + 0.028, -q[1])));
        const tg = own(new THREE.PlaneGeometry(0.014, 0.0022).rotateX(-Math.PI / 2));
        const tm = own(new THREE.MeshBasicMaterial({ color: '#ffffff', transparent: true, opacity: 0.85, blending: THREE.AdditiveBlending, depthWrite: false }));
        traffic = new THREE.InstancedMesh(tg, tm, TN); traffic.frustumCulled = false; traffic.renderOrder = 4;
        const pink = new THREE.Color('#ff48c4'), cy = new THREE.Color('#37d9ff');
        for (let i = 0; i < TN; i++) traffic.setColorAt(i, i % 2 ? pink : cy);
        group.add(traffic);
    }

    const tmpV = new THREE.Vector3(), tmpT = new THREE.Vector3(), target = new THREE.Color(), hot = new THREE.Color('#ffffff');
    let pulse = 0, hue = new THREE.Color('#37d9ff');
    return {
        group,
        update(dt, t, snap, toyState, lamps) {
            const st = toyState && toyState.spire ? toyState.spire : { speed: 0.4, color: '#37d9ff' };
            const speed = Number.isFinite(st.speed) ? st.speed : 0.4;
            hue.lerp(target.set(colorOf(THREE, st.color, '#37d9ff')), Math.min(1, dt * 4));
            const flash = lamps && lamps.flashLevel && lamps.flashLevel.length ? lamps.flashLevel[0] : 0;
            pulse = Math.max(flash || 0, pulse - dt * 2.2);
            holoU.uTime.value = t; holoU.uPulse.value = Math.min(1, pulse); holoU.uColor.value.copy(hue);
            prism.rotation.y = t * speed * 0.9;
            crystal.rotation.y = -t * speed * 1.3; crystal.rotation.x = t * speed * 0.7;
            crystalMat.color.copy(hue).lerp(hot, 0.4 + 0.5 * pulse);
            emitterMat.color.copy(hue).multiplyScalar(1.1 + pulse);
            rings.forEach((r, i) => { r.rotation.z = t * speed * (i % 2 ? -1.2 : 1.2) * (1 + i * 0.4); r.material.color.copy(hue); r.position.y = 0.04 + i * 0.03 + Math.sin(t * speed * 0.6 + i * 2) * 0.004; });
            if (motes) {
                const a = motes.geometry.attributes.position;
                for (let i = 0; i < a.count; i++) {
                    const ph = (moteBase[i * 3 + 1] + t * (0.05 + 0.08 * speed)) % 1, an = moteBase[i * 3] + t * speed * 0.8;
                    a.setXYZ(i, Math.cos(an) * moteBase[i * 3 + 2], 0.02 + ph * 0.09, Math.sin(an) * moteBase[i * 3 + 2]);
                }
                a.needsUpdate = true; motes.material.color.copy(hue).lerp(hot, 0.4);
            }
            // signs follow the skyline lamps (fallback: gentle flicker)
            let dirty = false;
            for (let i = 0; i < 6; i++) {
                let lv = 0.5 + 0.12 * Math.sin(t * 1.7 + i * 2.1);
                const idx = lamps && lamps.index && lamps.index.get ? lamps.index.get(signLamp[i]) : undefined;
                if (idx !== undefined && lamps.intensity) lv = 0.25 + 0.95 * lamps.intensity[idx];
                if (i === 2 && Math.sin(t * 9.0) > 0.96) lv *= 0.55;     // a faulty tube
                signLevel[i] += (lv - signLevel[i]) * Math.min(1, dt * 12);
                for (let k = 0; k < 4; k++) signCol.setXYZ(i * 4 + k, signLevel[i], signLevel[i], signLevel[i]);
                dirty = true;
            }
            if (dirty) signCol.needsUpdate = true;
            const vault = toyState && Number.isFinite(toyState.vault) ? toyState.vault : 0;
            stripMat.color.set('#a98bff').multiplyScalar(0.55 + 0.25 * vault + 0.2 * Math.sin(t * 3));
            if (traffic && curve) {
                for (let i = 0; i < TN; i++) {
                    const u = ((i / TN) + t * (0.035 + (i % 3) * 0.008)) % 1;
                    curve.getPointAt(u, tmpV); curve.getTangentAt(u, tmpT);
                    tmp.position.copy(tmpV).y += Math.sin(t * 2 + i) * 0.002;
                    tmp.rotation.set(0, Math.atan2(-tmpT.z, tmpT.x), 0); tmp.updateMatrix();
                    traffic.setMatrixAt(i, tmp.matrix);
                }
                traffic.instanceMatrix.needsUpdate = true;
            }
        },
        dispose() {
            for (const d of disposables) if (d && d.dispose) d.dispose();
            if (traffic) traffic.dispose();
            if (group.parent) group.parent.remove(group);
        }
    };
}

export default buildToys;
