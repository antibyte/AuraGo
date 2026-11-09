// Lost Temple of the Sun 3D toys (plan 6.3 / T25): the idol head with glowing eyes and a feather headdress, stone gate doors
// that sink with the door state, a stepped pyramid with a glowing capstone, a rotating sun disc with a crown of lamp-lit rays,
// swaying jungle foliage (instanced), torch flames and fireflies. buildToys(THREE, kit, layout, {tier, low}) ->
// {group, update(dt, t, snap, toyState, lamps, events?), dispose}. Local frame is the table root: X = x, Y = h, Z = -y.
// Draw calls: high 15, low 14 (fireflies only on high). The physical door device (render/builders.js buildDoor) is a
// translucent plate; the stone leaves below enclose it on both sides so it is never seen separately (no builder change).
import { rngNext } from '../../config.js';

const ARC = { cx: 0, cy: 0.8125, r: 0.2575 };
const STONE = '#9b8a6e', STONE_D = '#6f624e', STONE_L = '#b8a583', GOLD = '#ffcb65', GOLD_D = '#c8902f', JADE = '#2f9a72', DARK = '#14100b';

function canvas(w, h) { const c = document.createElement('canvas'); c.width = w; c.height = h; return c; }
function colorOf(THREE, v, fallback) { try { return new THREE.Color(v || fallback); } catch { return new THREE.Color(fallback); } }
const isLowTier = (kit, opts) => {
    if (opts && typeof opts.low === 'boolean') return opts.low;
    if (typeof opts === 'string') return opts === 'low';
    return !!(kit && kit.W && kit.W <= 1024);
};

// ---- procedural textures ----
function stoneTexture(THREE, size = 256) {
    const c = canvas(size, size), g = c.getContext('2d'), rng = { seed: 0x57017E };
    g.fillStyle = '#b3a283'; g.fillRect(0, 0, size, size);
    const img = g.getImageData(0, 0, size, size);
    for (let i = 0; i < img.data.length; i += 4) { const v = (rngNext(rng) - 0.5) * 38; img.data[i] += v; img.data[i + 1] += v; img.data[i + 2] += v * 0.9; }
    g.putImageData(img, 0, 0);
    for (let i = 0; i < 26; i++) { const x = rngNext(rng) * size, y = rngNext(rng) * size, r = 8 + rngNext(rng) * 26, gr = g.createRadialGradient(x, y, 0, x, y, r); gr.addColorStop(0, `rgba(${rngNext(rng) < 0.3 ? '70,110,60' : '60,48,34'},.22)`); gr.addColorStop(1, 'rgba(0,0,0,0)'); g.fillStyle = gr; g.fillRect(x - r, y - r, r * 2, r * 2); }
    g.strokeStyle = 'rgba(30,22,14,.55)'; g.lineWidth = 2;
    for (let i = 0; i < 5; i++) { g.beginPath(); let x = rngNext(rng) * size, y = rngNext(rng) * size; g.moveTo(x, y); for (let k = 0; k < 6; k++) { x += (rngNext(rng) - 0.5) * 30; y += (rngNext(rng) - 0.3) * 24; g.lineTo(x, y); } g.stroke(); }
    g.strokeStyle = 'rgba(25,18,10,.6)'; g.lineWidth = 3; g.strokeRect(1, 1, size - 2, size - 2);
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.wrapS = t.wrapT = THREE.RepeatWrapping; t.anisotropy = 4;
    return t;
}

function doorTexture(THREE) {
    const c = canvas(256, 256), g = c.getContext('2d');
    const base = stoneTexture(THREE, 128); void base;
    g.fillStyle = '#8d7c61'; g.fillRect(0, 0, 256, 256);
    const rng = { seed: 0xD00D };
    for (let i = 0; i < 1400; i++) { g.fillStyle = `rgba(${rngNext(rng) < 0.5 ? '0,0,0' : '255,240,200'},${0.03 + rngNext(rng) * 0.05})`; g.fillRect(rngNext(rng) * 256, rngNext(rng) * 256, 2 + rngNext(rng) * 3, 2 + rngNext(rng) * 3); }
    g.strokeStyle = '#1b130a'; g.lineWidth = 7; g.strokeRect(5, 5, 246, 246);
    g.strokeStyle = GOLD_D; g.lineWidth = 3; g.strokeRect(14, 14, 228, 228);
    g.translate(128, 128);
    g.shadowColor = '#ffb347'; g.shadowBlur = 10; g.strokeStyle = GOLD; g.fillStyle = 'rgba(255,203,101,.9)';
    for (let i = 0; i < 16; i++) { const a = i / 16 * Math.PI * 2, r0 = 50, r1 = i % 2 ? 78 : 96; g.beginPath(); g.moveTo(Math.cos(a - 0.12) * r0, Math.sin(a - 0.12) * r0); g.lineTo(Math.cos(a) * r1, Math.sin(a) * r1); g.lineTo(Math.cos(a + 0.12) * r0, Math.sin(a + 0.12) * r0); g.closePath(); g.fill(); }
    g.lineWidth = 5; g.beginPath(); g.arc(0, 0, 46, 0, Math.PI * 2); g.stroke(); g.lineWidth = 3; g.beginPath(); g.arc(0, 0, 30, 0, Math.PI * 2); g.stroke();
    g.fillStyle = '#3a2208'; g.shadowBlur = 0; g.beginPath(); g.arc(-10, -6, 4, 0, 7); g.arc(10, -6, 4, 0, 7); g.fill();
    g.strokeStyle = '#3a2208'; g.lineWidth = 3; g.beginPath(); g.moveTo(-10, 12); g.quadraticCurveTo(0, 20, 10, 12); g.stroke();
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    return t;
}

function sunFaceTexture(THREE) {
    const c = canvas(256, 256), g = c.getContext('2d');
    g.translate(128, 128);
    const bg = g.createRadialGradient(-20, -24, 0, 0, 0, 120); bg.addColorStop(0, '#ffe7a6'); bg.addColorStop(0.6, '#f0b24a'); bg.addColorStop(1, '#a8681a');
    g.fillStyle = bg; g.beginPath(); g.arc(0, 0, 120, 0, Math.PI * 2); g.fill();
    g.strokeStyle = '#4a2a06'; g.lineWidth = 7; g.beginPath(); g.arc(0, 0, 116, 0, 7); g.stroke();
    g.lineWidth = 3; g.beginPath(); g.arc(0, 0, 100, 0, 7); g.stroke();
    for (let i = 0; i < 24; i++) { const a = i / 24 * Math.PI * 2; g.save(); g.rotate(a); g.fillStyle = '#5a3408'; g.fillRect(-4, -112, 8, 9); g.restore(); }
    g.strokeStyle = '#4a2a06'; g.lineWidth = 6; g.lineCap = 'round'; g.lineJoin = 'round';
    g.beginPath(); g.moveTo(-58, -20); g.quadraticCurveTo(-36, -44, -14, -20); g.moveTo(58, -20); g.quadraticCurveTo(36, -44, 14, -20);
    g.moveTo(0, -14); g.lineTo(-10, 28); g.lineTo(8, 28); g.moveTo(-34, 58); g.quadraticCurveTo(0, 82, 34, 58); g.stroke();
    g.fillStyle = '#4a2a06'; g.beginPath(); g.arc(-36, -18, 6, 0, 7); g.arc(36, -18, 6, 0, 7); g.fill();
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    return t;
}

function leafTexture(THREE) {
    const c = canvas(128, 256), g = c.getContext('2d');
    g.translate(64, 252);
    g.fillStyle = '#ffffff';
    // fern frond: a spine with paired leaflets, then a few broader jungle-leaf lobes at the tip
    g.strokeStyle = '#cfe8c4'; g.lineWidth = 4; g.beginPath(); g.moveTo(0, 0); g.quadraticCurveTo(6, -120, 0, -240); g.stroke();
    for (let i = 1; i < 18; i++) {
        const y = -i * 13, len = 54 * Math.sin(Math.PI * Math.min(1, i / 18) * 0.92 + 0.25) + 6, ox = i * 0.35;
        for (const s of [-1, 1]) {
            g.beginPath(); g.moveTo(ox, y); g.quadraticCurveTo(ox + s * len * 0.55, y - 14, ox + s * len, y - 20 - i * 0.6); g.quadraticCurveTo(ox + s * len * 0.55, y - 2, ox, y); g.fill();
        }
    }
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 4;
    return t;
}

function glowTexture(THREE, flame = false) {
    const c = canvas(64, 128), g = c.getContext('2d');
    g.translate(32, flame ? 96 : 64);
    const gr = g.createRadialGradient(0, 0, 0, 0, 0, flame ? 60 : 32);
    gr.addColorStop(0, 'rgba(255,255,255,1)'); gr.addColorStop(0.3, 'rgba(255,200,110,.7)'); gr.addColorStop(1, 'rgba(255,120,20,0)');
    g.fillStyle = gr; g.save(); if (flame) g.scale(0.55, 1); g.beginPath(); g.arc(0, 0, flame ? 60 : 32, 0, Math.PI * 2); g.fill(); g.restore();
    const t = new THREE.CanvasTexture(c); t.colorSpace = THREE.SRGBColorSpace;
    return t;
}

// ---- geometry helpers (all indexed, attributes position/normal/uv/color) ----
function tint(THREE, g, hex, uvScale = 1) {
    const col = new THREE.Color(hex), n = g.attributes.position.count, a = new Float32Array(n * 3);
    for (let i = 0; i < n; i++) { a[i * 3] = col.r; a[i * 3 + 1] = col.g; a[i * 3 + 2] = col.b; }
    g.setAttribute('color', new THREE.BufferAttribute(a, 3));
    if (uvScale !== 1 && g.attributes.uv) { const uv = g.attributes.uv; for (let i = 0; i < uv.count; i++) uv.setXY(i, uv.getX(i) * uvScale, uv.getY(i) * uvScale); }
    return g;
}
function mergeGeos(THREE, list) {
    const pos = [], nor = [], uvs = [], col = [], idx = [];
    let off = 0;
    for (const g of list) {
        const p = g.attributes.position, n = g.attributes.normal, u = g.attributes.uv, c = g.attributes.color, ix = g.index;
        for (let k = 0; k < p.count; k++) {
            pos.push(p.getX(k), p.getY(k), p.getZ(k)); nor.push(n.getX(k), n.getY(k), n.getZ(k)); uvs.push(u ? u.getX(k) : 0, u ? u.getY(k) : 0);
            col.push(c ? c.getX(k) : 1, c ? c.getY(k) : 1, c ? c.getZ(k) : 1);
        }
        if (ix) for (let k = 0; k < ix.count; k++) idx.push(ix.getX(k) + off); else for (let k = 0; k < p.count; k++) idx.push(k + off);
        off += p.count; g.dispose();
    }
    const m = new THREE.BufferGeometry();
    m.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3)); m.setAttribute('normal', new THREE.Float32BufferAttribute(nor, 3));
    m.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2)); m.setAttribute('color', new THREE.Float32BufferAttribute(col, 3));
    m.setIndex(idx);
    return m;
}

export function buildToys(THREE, kit, layout, opts) {
    const low = isLowTier(kit, opts);
    const group = new THREE.Group(); group.name = 'templeToys';
    const disposables = [];
    const own = o => { disposables.push(o); return o; };
    const find = id => ((layout && layout.toys) || []).find(t => t.id === id);
    const frontZ = y => -y;                                         // table y (metres) -> local z
    const box = (w, h, d, x, y, z, c, ry = 0, rz = 0, uvs = 1 / 0.05) => { const g = new THREE.BoxGeometry(w, h, d); if (rz) g.rotateZ(rz); if (ry) g.rotateY(ry); g.translate(x, y, z); return tint(THREE, g, c, Math.max(w, h, d) * uvs); };
    const frustum = (rb, rt, h, x, y, z, c) => { const g = new THREE.CylinderGeometry(rt * Math.SQRT2, rb * Math.SQRT2, h, 4, 1).rotateY(Math.PI / 4).translate(x, y, z); return tint(THREE, g, c, 2.2); };
    const cyl = (rt, rb, h, seg, x, y, z, c, rx = 0, rz = 0) => { const g = new THREE.CylinderGeometry(rt, rb, h, seg); if (rx) g.rotateX(rx); if (rz) g.rotateZ(rz); g.translate(x, y, z); return tint(THREE, g, c, 1.5); };

    const stoneTex = own(stoneTexture(THREE));
    const stoneMat = own(new THREE.MeshStandardMaterial({ map: stoneTex, vertexColors: true, roughness: 0.86, metalness: 0.04, emissive: '#2a1d0e', emissiveIntensity: 0.55 }));
    const goldMat = own(new THREE.MeshStandardMaterial({ vertexColors: true, roughness: 0.34, metalness: 0.7, emissive: '#6a4410', emissiveIntensity: 0.65 }));
    const stoneParts = [], goldParts = [];
    const S = (g) => stoneParts.push(g), G = (g) => goldParts.push(g);

    // ---- idol (face plane at table y 0.795, body extends toward larger y) ----
    const idol = find('idolHead') || { x: 0, y: 0.790 }, ix = idol.x || 0, fy = 0.795, fz = frontZ(fy);
    const B = (w, h, d, x, y, dz, c) => S(box(w, h, d, ix + x, y, fz - dz, c));        // dz: depth of the box centre behind the face plane
    S(box(0.056, 0.006, 0.040, ix, 0.003, fz - 0.014, STONE_D)); S(box(0.046, 0.008, 0.032, ix, 0.010, fz - 0.013, STONE));      // plinth
    for (let i = 0; i < 3; i++) S(box(0.026 - i * 0.002, 0.004, 0.0075, ix, 0.002 + i * 0.004, frontZ(0.752 + i * 0.0105), i % 2 ? STONE_L : STONE));   // altar steps
    B(0.030, 0.012, 0.020, 0, 0.020, 0.012, STONE_D);                                                     // neck / chest
    B(0.034, 0.030, 0.022, 0, 0.037, 0.011, STONE);                                                       // head block
    B(0.037, 0.005, 0.0065, 0, 0.0505, -0.0005, STONE_D);                                                  // brow ridge, proud of the face
    B(0.007, 0.016, 0.006, 0, 0.034, -0.0008, STONE_L);                                                    // nose
    B(0.0095, 0.0075, 0.004, -0.0095, 0.0415, 0.0, DARK); B(0.0095, 0.0075, 0.004, 0.0095, 0.0415, 0.0, DARK);       // eye sockets
    B(0.008, 0.005, 0.005, -0.0125, 0.0285, -0.0012, STONE_L); B(0.008, 0.005, 0.005, 0.0125, 0.0285, -0.0012, STONE_L); // cheekbones
    B(0.018, 0.0075, 0.004, 0, 0.0230, 0.0, DARK);                                                         // open mouth
    for (let i = 0; i < 4; i++) G(box(0.0028, 0.0025, 0.0016, ix - 0.0066 + i * 0.0044, i % 2 ? 0.0213 : 0.0247, fz + 0.0004 - 0.0015, '#e9d7a4'));  // teeth
    B(0.014, 0.005, 0.006, 0, 0.0155, -0.0005, STONE);                                                     // chin
    for (const s of [-1, 1]) {                                                                              // gold ear spools
        G(cyl(0.0052, 0.0052, 0.0045, 14, ix + s * 0.0197, 0.036, fz - 0.011, GOLD, 0, Math.PI / 2));
        G(cyl(0.0027, 0.0027, 0.0052, 10, ix + s * 0.0197, 0.036, fz - 0.011, '#fff0b8', 0, Math.PI / 2));
    }
    G(box(0.040, 0.0042, 0.026, ix, 0.0545, fz - 0.011, GOLD_D));                                           // headband
    G(box(0.040, 0.0016, 0.0265, ix, 0.0568, fz - 0.011, GOLD));
    G(cyl(0.0052, 0.0052, 0.0016, 18, ix, 0.0555, fz - 0.0003, '#fff0b8', Math.PI / 2));                    // sun glyph on the band
    for (const [n, len, wd, y0, col, dz] of [[9, 0.024, 0.0050, 0.0575, GOLD, 0.012], [8, 0.016, 0.0042, 0.0575, JADE, 0.0105]]) {   // feather fan
        for (let i = 0; i < n; i++) {
            const f = n === 9 ? (i / 8 - 0.5) : ((i + 0.5) / 8 - 0.5), ang = f * 2.5, l = len * (1 - Math.abs(f) * 0.38) * (n === 9 ? 1 : 0.92);
            const g = new THREE.BoxGeometry(wd, l, 0.0024); g.translate(0, l / 2, 0); g.rotateZ(-ang); g.translate(ix + Math.sin(ang) * 0.0, y0, fz - dz);
            G(tint(THREE, g, col === GOLD ? (i % 2 ? GOLD : GOLD_D) : col));
        }
    }
    for (const s of [-1, 1]) { B(0.003, 0.034, 0.016, s * 0.0185, 0.037, 0.012, STONE_D); }                  // temples

    // ---- stone gate (visible doors) frame: posts + lintel; the leaves are separate meshes ----
    const door = find('doorsToy') || { x: 0, y: 0.660 }, dz = frontZ(door.y), dx = door.x || 0;
    for (const s of [-1, 1]) { S(box(0.0075, 0.046, 0.011, dx + s * 0.0300, 0.023, dz, STONE_D)); G(box(0.0085, 0.0025, 0.0125, dx + s * 0.0300, 0.0470, dz, GOLD)); }
    S(box(0.070, 0.0085, 0.0125, dx, 0.0385, dz, STONE)); G(box(0.072, 0.0022, 0.0140, dx, 0.0437, dz, GOLD_D));
    G(cyl(0.0058, 0.0058, 0.0018, 20, dx, 0.0385, dz + 0.0068, GOLD, Math.PI / 2));
    G(cyl(0.0027, 0.0027, 0.0022, 12, dx, 0.0385, dz + 0.0070, '#fff0b8', Math.PI / 2));

    // ---- torch posts inside the idol channel ----
    const torchPos = [[-0.0205, 0.752], [0.0205, 0.752]];
    for (const [x, y] of torchPos) { S(box(0.0066, 0.022, 0.0066, x, 0.011, frontZ(y), STONE_D)); G(cyl(0.0055, 0.0032, 0.0045, 10, x, 0.0243, frontZ(y), GOLD)); }

    // ---- stepped pyramid with a capstone slot ----
    const py = find('pyramidToy') || { x: 0.125, y: 0.930 }, px0 = py.x, pz0 = frontZ(0.922);
    const tiers = 4, step = 0.0112, baseHalf = 0.0300;
    for (let i = 0; i < tiers; i++) {
        const half = baseHalf - i * 0.0062, y = step * i + step / 2;
        S(frustum(half, half - 0.0016, step, px0, y, pz0, i % 2 ? STONE : STONE_D));
        G(frustum(half + 0.0004, half - 0.0004, 0.0016, px0, step * (i + 1) - 0.0008, pz0, GOLD_D));
    }
    for (let i = 0; i < tiers; i++) S(box(0.0105, step * 0.9, 0.0030, px0, step * i + step * 0.45, pz0 + (baseHalf - i * 0.0062) + 0.0006, STONE_L, 0, 0, 0));   // stair on the front face
    const capY = step * tiers + 0.0050;

    // ---- merged static meshes ----
    const stoneMesh = new THREE.Mesh(own(mergeGeos(THREE, stoneParts)), stoneMat); stoneMesh.name = 'templeStone'; stoneMesh.frustumCulled = false;
    const goldMesh = new THREE.Mesh(own(mergeGeos(THREE, goldParts)), goldMat); goldMesh.name = 'templeGold'; goldMesh.frustumCulled = false;
    group.add(stoneMesh, goldMesh);

    // ---- door leaves (sink with the door state) ----
    const doorTex = own(doorTexture(THREE));
    const doorMat = own(new THREE.MeshStandardMaterial({ map: doorTex, roughness: 0.8, metalness: 0.05, emissive: '#3a2610', emissiveIntensity: 0.7, emissiveMap: doorTex }));
    const leaves = [];
    for (const s of [-1, 1]) {
        const g = own(new THREE.BoxGeometry(0.0262, 0.034, 0.0075)), uv = g.attributes.uv;
        for (let i = 0; i < uv.count; i++) uv.setX(i, uv.getX(i) * 0.5 + (s > 0 ? 0.5 : 0));
        const m = new THREE.Mesh(g, doorMat); m.position.set(dx + s * 0.01315, 0.017, dz); m.frustumCulled = false; m.name = 'templeDoor'; group.add(m); leaves.push(m);
    }

    // ---- glowing parts: eyes + halos, mouth/torch flames, capstone + beam ----
    const eyeGeo = own(mergeGeos(THREE, [-1, 1].map(s => tint(THREE, new THREE.SphereGeometry(0.0034, low ? 8 : 12, low ? 6 : 8).scale(1, 0.8, 0.6).translate(ix + s * 0.0095, 0.0415, fz + 0.0006), '#ffffff'))));
    const eyeMat = own(new THREE.MeshBasicMaterial({ vertexColors: true, toneMapped: false }));
    const eyes = new THREE.Mesh(eyeGeo, eyeMat); eyes.name = 'idolEyes'; eyes.frustumCulled = false; group.add(eyes);
    const glowTex = own(glowTexture(THREE)), flameTex = own(glowTexture(THREE, true));
    const haloGeo = own(mergeGeos(THREE, [-1, 1].map(s => tint(THREE, new THREE.PlaneGeometry(0.030, 0.030).translate(ix + s * 0.0095, 0.0415, fz + 0.0030), '#ffffff'))));
    const haloMat = own(new THREE.MeshBasicMaterial({ map: glowTex, vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }));
    const halo = new THREE.Mesh(haloGeo, haloMat); halo.renderOrder = 6; halo.frustumCulled = false; group.add(halo);
    const eyeCol = eyeGeo.attributes.color, haloCol = haloGeo.attributes.color;
    const eyeVerts = eyeCol.count / 2, haloVerts = haloCol.count / 2;

    // flames: 2 torches + the idol's mouth fire, instanced additive quads (crossed pairs via two instances per flame)
    const flameGeo = own(new THREE.PlaneGeometry(0.014, 0.026).translate(0, 0.013, 0));
    const flameMat = own(new THREE.MeshBasicMaterial({ map: flameTex, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false, side: THREE.DoubleSide, color: new THREE.Color(1.5, 1.0, 0.6) }));
    const flameSpots = [...torchPos.map(([x, y]) => [x, 0.0265, frontZ(y), 1]), [ix, 0.0205, fz + 0.0012, 0.6]];
    const flames = new THREE.InstancedMesh(flameGeo, flameMat, flameSpots.length * 2); flames.frustumCulled = false; flames.renderOrder = 7; group.add(flames);

    const capMat = own(new THREE.MeshBasicMaterial({ color: GOLD, toneMapped: false }));
    const capstone = new THREE.Mesh(own(new THREE.ConeGeometry(0.0072 * Math.SQRT2, 0.0105, 4, 1).rotateY(Math.PI / 4)), capMat);
    capstone.position.set(px0, capY, pz0); capstone.frustumCulled = false; group.add(capstone);
    const beamMat = own(new THREE.MeshBasicMaterial({ color: '#ffd27a', transparent: true, opacity: 0, blending: THREE.AdditiveBlending, depthWrite: false, side: THREE.DoubleSide, toneMapped: false }));
    const beam = new THREE.Mesh(own(new THREE.CylinderGeometry(0.0016, 0.0085, 0.062, 14, 1, true).translate(0, 0.031, 0)), beamMat);
    beam.position.set(px0, capY + 0.004, pz0); beam.renderOrder = 6; beam.frustumCulled = false; group.add(beam);

    // ---- sun disc + crown of rays ----
    const sd = find('sunDisc') || { x: 0, y: 1.0 }, sy = Math.min(1.045, sd.y + 0.045), sh = 0.058, sz = frontZ(sy);
    const rotor = new THREE.Group(); rotor.position.set(sd.x, sh, sz);
    const rotorGeos = [tint(THREE, new THREE.TorusGeometry(0.0335, 0.0030, 8, low ? 28 : 44), GOLD)];
    for (let i = 0; i < 16; i++) {
        const a = i / 16 * Math.PI * 2, g = new THREE.ConeGeometry(0.0050, i % 2 ? 0.0095 : 0.0140, 4, 1); g.translate(0, 0.0335 + (i % 2 ? 0.0048 : 0.0070), 0); g.rotateZ(a - Math.PI * 0 );
        rotorGeos.push(tint(THREE, g, i % 2 ? GOLD_D : GOLD));
    }
    const rotorMesh = new THREE.Mesh(own(mergeGeos(THREE, rotorGeos)), goldMat); rotorMesh.frustumCulled = false; rotor.add(rotorMesh); group.add(rotor);
    const faceTex = own(sunFaceTexture(THREE));
    const faceMat = own(new THREE.MeshBasicMaterial({ map: faceTex, color: new THREE.Color(1.1, 1.0, 0.9), toneMapped: false }));
    const face = new THREE.Mesh(own(new THREE.CircleGeometry(0.0315, low ? 28 : 44)), faceMat); face.position.set(sd.x, sh, sz + 0.0006); face.frustumCulled = false; group.add(face);
    const sunHaloMat = own(new THREE.MeshBasicMaterial({ map: glowTex, color: '#ffb347', transparent: true, opacity: 0.4, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }));
    const sunHalo = new THREE.Mesh(own(new THREE.PlaneGeometry(0.17, 0.17)), sunHaloMat); sunHalo.position.set(sd.x, sh, sz - 0.004); sunHalo.renderOrder = 5; sunHalo.frustumCulled = false; group.add(sunHalo);

    // crown: six rays standing on the top arc at the ray lamp positions, tilted outward (static gold) + lamp-lit glow quads
    const rayX = [-0.150, -0.090, -0.030, 0.030, 0.090, 0.150], crownGold = [], rayGlowGeos = [];
    rayX.forEach((x, i) => {
        const y = ARC.cy + Math.sqrt(Math.max(0, ARC.r * ARC.r - x * x)) - 0.013, tilt = Math.atan2(x, 0.34) * 1.6;
        const g = new THREE.CylinderGeometry(0.0011, 0.0072, 0.060, 4, 1); g.translate(0, 0.030, 0); g.rotateZ(-tilt); g.translate(x, 0.0, frontZ(y)); crownGold.push(tint(THREE, g, i % 2 ? GOLD_D : GOLD));
        const gl = new THREE.PlaneGeometry(0.016, 0.052); gl.translate(0, 0.026, 0.002); gl.rotateZ(-tilt); gl.translate(x, 0.004, frontZ(y)); rayGlowGeos.push(tint(THREE, gl, '#ffffff'));
    });
    const crownMesh = new THREE.Mesh(own(mergeGeos(THREE, crownGold)), goldMat); crownMesh.frustumCulled = false; group.add(crownMesh);
    const rayGeo = own(mergeGeos(THREE, rayGlowGeos)), rayCol = rayGeo.attributes.color, rayVerts = rayCol.count / 6;
    const rayMat = own(new THREE.MeshBasicMaterial({ map: glowTex, vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }));
    const rayMesh = new THREE.Mesh(rayGeo, rayMat); rayMesh.renderOrder = 6; rayMesh.frustumCulled = false; group.add(rayMesh);
    const rayLevel = new Float32Array(6);

    // ---- jungle foliage (instanced, swaying) ----
    const leafTex = own(leafTexture(THREE));
    const frond = new THREE.PlaneGeometry(0.038, 0.078, 1, 4).translate(0, 0.039, 0);
    { const p = frond.attributes.position; for (let i = 0; i < p.count; i++) { const y = p.getY(i); p.setZ(i, -y * y * 2.4); } frond.computeVertexNormals(); }
    own(frond);
    const leafMat = own(new THREE.MeshStandardMaterial({ map: leafTex, alphaTest: 0.5, side: THREE.DoubleSide, roughness: 0.7, metalness: 0, emissive: '#10300c', emissiveIntensity: 0.6 }));
    const clusters = [[-0.2515, 0.45, 1], [-0.2515, 0.56, 1], [-0.2515, 0.68, 1], [-0.2490, 0.80, 1], [-0.2380, 0.90, 1], [-0.2100, 0.975, 1],
        [0.2515, 0.56, -1], [0.2515, 0.68, -1], [0.2490, 0.80, -1], [0.2380, 0.90, -1], [0.2100, 0.975, -1]];
    const perCluster = low ? 3 : 5, FN = clusters.length * perCluster, frng = { seed: 0xF011A6E };
    const foliage = new THREE.InstancedMesh(frond, leafMat, FN); foliage.frustumCulled = false; foliage.name = 'templeFoliage';
    const leafData = [], palette = ['#2c7a3a', '#3f9a47', '#1f6a30', '#58b04f', '#2f8a58'].map(c => new THREE.Color(c));
    clusters.forEach(([x, y, side]) => {
        for (let k = 0; k < perCluster; k++) {
            const sc = 0.6 + rngNext(frng) * 0.55;
            leafData.push({ x: x + (rngNext(frng) - 0.5) * 0.012, y: y + (rngNext(frng) - 0.5) * 0.05, h: 0.034 + rngNext(frng) * 0.02, side, yaw: -side * Math.PI / 2 + (rngNext(frng) - 0.5) * 1.0, pitch: 0.6 + rngNext(frng) * 0.4, sc, ph: rngNext(frng) * 6.28, sp: 0.5 + rngNext(frng) * 0.5 });
            foliage.setColorAt(leafData.length - 1, palette[Math.floor(rngNext(frng) * palette.length)]);
        }
    });
    group.add(foliage);
    const tmp = new THREE.Object3D(), euler = new THREE.Euler(0, 0, 0, 'YXZ');
    const writeFoliage = (t, amp = 0) => {
        for (let i = 0; i < FN; i++) {
            const d = leafData[i], sw = Math.sin(t * d.sp + d.ph) * 0.07 + Math.sin(t * 14 + d.ph) * 0.1 * amp;
            tmp.position.set(d.x, d.h, frontZ(d.y)); euler.set(-d.pitch + sw, d.yaw + Math.cos(t * d.sp * 0.7 + d.ph) * 0.05, 0); tmp.quaternion.setFromEuler(euler);
            tmp.scale.setScalar(d.sc); tmp.updateMatrix(); foliage.setMatrixAt(i, tmp.matrix);
        }
        foliage.instanceMatrix.needsUpdate = true;
    };
    writeFoliage(0);

    // ---- fireflies (high tier) ----
    let flies = null, fliesBase = null;
    if (!low) {
        const n = 26, frng2 = { seed: 0xF1E5 }; fliesBase = new Float32Array(n * 4);
        for (let i = 0; i < n; i++) {
            const left = i % 2 === 0; fliesBase[i * 4] = (left ? -0.17 : 0.17) + (rngNext(frng2) - 0.5) * 0.12; fliesBase[i * 4 + 1] = 0.62 + rngNext(frng2) * 0.34; fliesBase[i * 4 + 2] = rngNext(frng2) * 6.28; fliesBase[i * 4 + 3] = 0.3 + rngNext(frng2) * 0.5;
        }
        const g = own(new THREE.BufferGeometry()); g.setAttribute('position', new THREE.BufferAttribute(new Float32Array(n * 3), 3));
        const m = own(new THREE.PointsMaterial({ color: '#ffe08a', size: 0.0032, transparent: true, opacity: 0.85, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }));
        flies = new THREE.Points(g, m); flies.frustumCulled = false; flies.renderOrder = 7; group.add(flies);
    }

    // ---- per-frame ----
    const flameObj = new THREE.Object3D(), eye = new THREE.Color(), eyeTarget = new THREE.Color(), white = new THREE.Color('#ffffff');
    let eyePulse = 0, rayPulse = 0, shake = 0, doorK = 0, glow = 0.15, pyr = 0, rays = 0.1, eyeLevel = [0.3, 0.3];
    const lampLevel = (lamps, id, fallback) => {
        const i = lamps && lamps.index && lamps.index.get ? lamps.index.get(id) : undefined;
        return i !== undefined && lamps.intensity ? lamps.intensity[i] : fallback;
    };
    return {
        group,
        update(dt, t, snap, toyState, lamps, events) {
            dt = Number.isFinite(dt) ? Math.min(dt, 0.1) : 0.016; t = Number.isFinite(t) ? t : 0;
            const ts = toyState || {}, ie = ts.idolEyes || {};
            const k = a => Math.min(1, dt * a);
            // events [{source, type, id}]: the idol flares on its scoop, rays on SUN drops and ramps, the jungle shakes on pops
            if (events && events.length) for (const e of events) {
                const id = String((e && (e.id || e.device)) || '');
                if (/idolScoop|idolDoors/.test(id)) eyePulse = 1; else if (/^sun|^ray|Ramp/.test(id)) rayPulse = 1; else if (/jungle/.test(id)) shake = Math.min(1.5, shake + 0.6);
            }
            eyePulse = Math.max(0, eyePulse - dt * 1.6); rayPulse = Math.max(0, rayPulse - dt * 1.4); shake = Math.max(0, shake - dt * 1.2);
            doorK += ((Number.isFinite(ts.doors) ? ts.doors : 0) - doorK) * k(5);
            glow += ((Number.isFinite(ie.glow) ? ie.glow : 0.15) - glow) * k(4);
            pyr += ((Number.isFinite(ts.pyramidGlow) ? ts.pyramidGlow : 0) - pyr) * k(3);
            rays += ((Number.isFinite(ts.sunRays) ? ts.sunRays : 0.1) - rays) * k(3);
            leaves.forEach((m, i) => { m.position.y = 0.017 - doorK * 0.0365; m.position.x = dx + (i ? 1 : -1) * (0.01315 + doorK * 0.0006); });

            // eyes: colour from the rules, brightness from the eye lamps and the glow level
            eyeTarget.copy(colorOf(THREE, ie.color, GOLD)); eye.lerp(eyeTarget, k(6));
            ['idolEyeL', 'idolEyeR'].forEach((id, e) => {
                const lv = Math.min(1.4, Math.max(lampLevel(lamps, id, 0), glow * 0.85) + eyePulse * 0.6), flick = 0.92 + 0.08 * Math.sin(t * 7 + e * 1.7);
                eyeLevel[e] = 0.12 + lv * 2.4 * flick;
                for (let v = 0; v < eyeVerts; v++) eyeCol.setXYZ(e * eyeVerts + v, eye.r * eyeLevel[e], eye.g * eyeLevel[e], eye.b * eyeLevel[e]);
                for (let v = 0; v < haloVerts; v++) haloCol.setXYZ(e * haloVerts + v, eye.r * eyeLevel[e] * 0.28, eye.g * eyeLevel[e] * 0.28, eye.b * eyeLevel[e] * 0.28);
            });
            eyeCol.needsUpdate = true; haloCol.needsUpdate = true;

            // flames: torches burn brighter with the glow level; the mouth fire follows the eye glow
            for (let i = 0; i < flameSpots.length; i++) {
                const [x, y, z, sc] = flameSpots[i], fl = 0.85 + 0.25 * Math.sin(t * 9 + i * 2.3) + 0.1 * Math.sin(t * 17 + i), s = sc * (0.8 + glow * 0.45) * fl;
                for (let c = 0; c < 2; c++) {
                    flameObj.position.set(x, y, z); flameObj.rotation.set(0, c * Math.PI / 2 + Math.sin(t * 2 + i) * 0.2, 0); flameObj.scale.set(s, s * (0.9 + 0.2 * Math.sin(t * 11 + i)), s);
                    flameObj.updateMatrix(); flames.setMatrixAt(i * 2 + c, flameObj.matrix);
                }
            }
            flames.instanceMatrix.needsUpdate = true;

            // capstone and beam
            capMat.color.set(GOLD).lerp(white, 0.2 + 0.5 * pyr).multiplyScalar(0.7 + 1.9 * pyr + 0.12 * Math.sin(t * 3));
            beamMat.opacity = pyr * (0.28 + 0.08 * Math.sin(t * 2.4)); beam.visible = pyr > 0.02;
            beam.scale.set(1, 0.7 + 0.5 * pyr, 1);

            // sun disc: slow turn, faster with the ray level; crown glow follows the ray lamps
            rotor.rotation.z = -t * (0.12 + 0.5 * rays);
            faceMat.color.setScalar(0.85 + 0.55 * rays);
            sunHaloMat.opacity = 0.18 + 0.5 * rays; const hs = 1 + 0.06 * Math.sin(t * 1.3); sunHalo.scale.set(hs, hs, 1);
            for (let i = 0; i < 6; i++) {
                const lv = Math.max(lampLevel(lamps, 'ray' + (i + 1), 0), rays * 0.6) + rayPulse * 0.5 * (0.9 + 0.1 * Math.sin(t * 5 + i * 1.1));
                rayLevel[i] += (lv - rayLevel[i]) * k(10);
                const v = 0.05 + 0.95 * rayLevel[i];
                for (let q = 0; q < rayVerts; q++) rayCol.setXYZ(i * rayVerts + q, v, v * 0.78, v * 0.36);
            }
            rayCol.needsUpdate = true;

            writeFoliage(t, shake);
            if (flies) {
                const a = flies.geometry.attributes.position;
                for (let i = 0; i < a.count; i++) {
                    const b = i * 4, ph = fliesBase[b + 2] + t * fliesBase[b + 3];
                    a.setXYZ(i, fliesBase[b] + Math.sin(ph * 1.3) * 0.02, 0.02 + 0.035 * (1 + Math.sin(ph * 0.9)), -(fliesBase[b + 1] + Math.cos(ph) * 0.02));
                }
                a.needsUpdate = true; flies.material.opacity = 0.45 + 0.4 * Math.sin(t * 1.7) ** 2;
            }
        },
        dispose() {
            for (const d of disposables) if (d && d.dispose) d.dispose();
            foliage.dispose(); flames.dispose();
            if (group.parent) group.parent.remove(group);
        }
    };
}

export default buildToys;
