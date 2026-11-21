// AuraPinball cabinet: wedge body (glass 6.5 cm above the playfield at the front, 14 cm at the back), wooden inner side
// boards, rail caps, lockdown bar, tilted glass sheet, trapezoid side art, instruction cards, backbox with backglass + DMD.
// The apron lives in apron.js, the room (floor, legs, wall) in room.js. Geometry is in tableRoot space (X = x, Y = h, Z = -y).
import * as THREE from 'three';
import { FONT_STACK } from '../config.js';
import { CABINET, glassHeight, topAt } from '../tables/kit/cabinet-dims.js';
import { box, merge, quad, canvasTexture } from './materials.js';
import { hexRgb, rgbCss, trText } from './art-kit.js';
import { buildApron } from './apron.js';

const TILT = 6.5 * Math.PI / 180;
const { frontY, backY, sideIn, sideOut, bottom } = CABINET;
const canvasOf = (w, h) => { const c = document.createElement('canvas'); c.width = w; c.height = h; return c; };
const LEVELS = [0, 89, 166, 255];

// Flat-shaded faces: each [a, b, c, d] is wound counter-clockwise seen from outside. Own vertices per face (indexed) so the
// result merges with box() geometry; uv is planar (or `uvOf(point) -> [u, v]`).
function faces(list, uvOf) {
    const pos = [], nor = [], uv = [], idx = [];
    for (const f of list) {
        const [a, b, c] = f, u = [b[0] - a[0], b[1] - a[1], b[2] - a[2]], w = [c[0] - b[0], c[1] - b[1], c[2] - b[2]];
        let n = [u[1] * w[2] - u[2] * w[1], u[2] * w[0] - u[0] * w[2], u[0] * w[1] - u[1] * w[0]];
        const l = Math.hypot(...n) || 1; n = n.map(v => v / l);
        const [ax, ay, az] = n.map(Math.abs), base = pos.length / 3;
        for (const p of f) {
            pos.push(...p); nor.push(...n);
            uv.push(...(uvOf ? uvOf(p) : ax >= ay && ax >= az ? [-p[2], p[1]] : ay >= az ? [p[0], -p[2]] : [p[0], p[1]]));
        }
        idx.push(base, base + 1, base + 2, base, base + 2, base + 3);
    }
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
    g.setAttribute('normal', new THREE.Float32BufferAttribute(nor, 3));
    g.setAttribute('uv', new THREE.Float32BufferAttribute(uv, 2));
    g.setIndex(idx);
    return g;
}

// Prism with a sloped top (and optionally bottom): x0..x1, y0 (front) .. y1 (back), heights per y (number or function).
export function wedgeBox(x0, x1, y0, y1, hBottom, hTopAt) {
    const hb = typeof hBottom === 'function' ? hBottom : () => hBottom, ht = typeof hTopAt === 'function' ? hTopAt : () => hTopAt;
    const zf = -y0, zb = -y1, b0 = hb(y0), b1 = hb(y1), t0 = ht(y0), t1 = ht(y1);
    return faces([
        [[x0, t0, zf], [x1, t0, zf], [x1, t1, zb], [x0, t1, zb]],
        [[x0, b1, zb], [x1, b1, zb], [x1, b0, zf], [x0, b0, zf]],
        [[x0, b0, zf], [x1, b0, zf], [x1, t0, zf], [x0, t0, zf]],
        [[x1, b1, zb], [x0, b1, zb], [x0, t1, zb], [x1, t1, zb]],
        [[x1, b0, zf], [x1, b1, zb], [x1, t1, zb], [x1, t0, zf]],
        [[x0, b1, zb], [x0, b0, zf], [x0, t0, zf], [x0, t1, zb]]
    ]);
}

// Quad with explicit per-corner uv (a, b, c, d counter-clockwise seen from the front); same attributes as materials.quad().
function quadUV(a, b, c, d, ua, ub, uc, ud) {
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute([...a, ...b, ...c, ...a, ...c, ...d], 3));
    g.setAttribute('uv', new THREE.Float32BufferAttribute([...ua, ...ub, ...uc, ...ua, ...uc, ...ud], 2));
    g.computeVertexNormals();
    return g;
}

function sideArt(layout, accent, secondary) {
    const c = canvasOf(2048, 512), g = c.getContext('2d');
    const bg = g.createLinearGradient(0, 0, 0, 512);
    bg.addColorStop(0, '#05070b'); bg.addColorStop(0.55, '#0b1019'); bg.addColorStop(1, rgbCss(accent, 0.55));
    g.fillStyle = bg; g.fillRect(0, 0, 2048, 512);
    g.save(); g.globalCompositeOperation = 'lighter';
    for (let i = 0; i < 7; i++) {
        const col = i % 2 ? secondary : accent, y = 70 + i * 52;
        g.strokeStyle = rgbCss(col, 1, 0.16 + (i % 3) * 0.05); g.lineWidth = 6 + (i % 3) * 3;
        g.beginPath(); g.moveTo(-40, y + 200); g.lineTo(2100, y - 160); g.stroke();
    }
    g.restore();
    g.shadowColor = rgbCss(accent, 1, 1); g.shadowBlur = 26; g.fillStyle = rgbCss(accent, 1.1);
    g.font = `800 150px ${FONT_STACK}`; g.textAlign = 'center'; g.textBaseline = 'middle';
    g.fillText(String((layout.meta && layout.meta.title) || '').toUpperCase(), 1024, 235, 1700);
    g.shadowBlur = 0; g.fillStyle = 'rgba(210,220,235,.55)'; g.fillRect(0, 12, 2048, 6); g.fillRect(0, 494, 2048, 6);
    return c;
}

// Instruction card: dark printed card with an accent frame; lines are the translated copy.
function cardCanvas(text, accent, big, aspect = 2.56) {
    const W = Math.round(400 * aspect), c = canvasOf(W, 400), g = c.getContext('2d');
    const bg = g.createLinearGradient(0, 0, 0, 400);
    bg.addColorStop(0, '#141a26'); bg.addColorStop(1, '#070a10'); g.fillStyle = bg; g.fillRect(0, 0, W, 400);
    g.shadowColor = rgbCss(accent, 1, 1); g.shadowBlur = 14; g.strokeStyle = rgbCss(accent, 1.1); g.lineWidth = 8;
    g.strokeRect(14, 14, W - 28, 372); g.shadowBlur = 0; g.lineWidth = 2; g.strokeStyle = 'rgba(255,255,255,.35)'; g.strokeRect(30, 30, W - 60, 340);
    const lines = String(text).split('\n').filter(Boolean);
    let fs = big ? 96 : 64;
    g.textAlign = 'center'; g.textBaseline = 'middle';
    const fit = () => { g.font = `800 ${fs}px ${FONT_STACK}`; return Math.max(...lines.map(l => g.measureText(l).width)); };
    while (fit() > W - 124 && fs > 20) fs -= 2;
    while (lines.length * fs * 1.28 > 310 && fs > 20) { fs -= 2; fit(); }
    g.fillStyle = '#f2f6ff'; g.shadowColor = rgbCss(accent, 1, 0.9); g.shadowBlur = 10;
    lines.forEach((l, i) => g.fillText(l, W / 2, 200 + (i - (lines.length - 1) / 2) * fs * 1.28));
    return c;
}

function speakerPanel(accent) {
    const c = canvasOf(2048, 380), g = c.getContext('2d');
    const bg = g.createLinearGradient(0, 0, 0, 380);
    bg.addColorStop(0, '#171c26'); bg.addColorStop(1, '#07090e'); g.fillStyle = bg; g.fillRect(0, 0, 2048, 380);
    for (const cx of [310, 1738]) {
        g.fillStyle = '#05070a'; g.beginPath(); g.arc(cx, 190, 150, 0, 7); g.fill();
        g.strokeStyle = 'rgba(200,210,225,.55)'; g.lineWidth = 6; g.stroke();
        for (let y = -6; y <= 6; y++) for (let x = -6; x <= 6; x++) {
            const px = cx + (x + (y & 1) * 0.5) * 21, py = 190 + y * 18.5;
            if (Math.hypot(px - cx, py - 190) > 132) continue;
            g.fillStyle = '#2b3342'; g.beginPath(); g.arc(px, py, 6.2, 0, 7); g.fill();
        }
    }
    g.fillStyle = rgbCss(accent, 1.2, 0.9); g.fillRect(0, 0, 2048, 5);
    return c;
}

export function createDmdScreen(accent) {
    void accent;
    const data = new Uint8Array(192 * 48);
    const tex = new THREE.DataTexture(data, 192, 48, THREE.RedFormat, THREE.UnsignedByteType);
    tex.magFilter = tex.minFilter = THREE.NearestFilter; tex.generateMipmaps = false; tex.flipY = false; tex.unpackAlignment = 1; tex.needsUpdate = true;
    const material = new THREE.ShaderMaterial({
        uniforms: { frame: { value: tex }, dims: { value: new THREE.Vector2(192, 48) }, amber: { value: new THREE.Vector3(1.0, 0.29, 0.018) }, hot: { value: new THREE.Vector3(1.0, 0.82, 0.42) }, unlit: { value: new THREE.Vector3(0.024, 0.0045, 0.0006) } },
        vertexShader: 'varying vec2 vUv; void main(){ vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
        fragmentShader: `uniform sampler2D frame; uniform vec2 dims; uniform vec3 amber; uniform vec3 hot; uniform vec3 unlit; varying vec2 vUv;
        float lv(vec2 id){ return texture2D(frame, (id + 0.5) / dims).r; }
        void main(){
            vec2 cell = vec2(vUv.x, 1.0 - vUv.y) * dims; vec2 id = floor(cell); vec2 f = fract(cell) - 0.5;
            float l = lv(id), d = length(f), dotm = smoothstep(0.49, 0.28, d);
            float core = smoothstep(0.22, 0.0, d) * smoothstep(0.72, 1.0, l);
            float nb = (lv(id + vec2(1.0, 0.0)) + lv(id - vec2(1.0, 0.0)) + lv(id + vec2(0.0, 1.0)) + lv(id - vec2(0.0, 1.0))) * 0.25;
            vec3 col = mix(unlit, amber * 1.72, l) * dotm + hot * (core * 0.55) + amber * (l * 0.17 + nb * 0.08) * (1.0 - dotm);
            col += vec3(0.012, 0.014, 0.018) * smoothstep(0.62, 0.0, abs(vUv.x + vUv.y * 0.45 - 0.55));
            gl_FragColor = vec4(col, 1.0);
        }`
    });
    return {
        texture: tex, material,
        setFrame(frame) { for (let i = 0; i < 9216; i++) data[i] = LEVELS[frame[i] & 3]; tex.needsUpdate = true; },
        dispose() { tex.dispose(); material.dispose(); }
    };
}

function playfieldGlassCanvas(accent) {
    const c = canvasOf(512, 1024), g = c.getContext('2d');
    g.clearRect(0, 0, 512, 1024);
    const sh = g.createLinearGradient(0, 0, 512, 1024);
    sh.addColorStop(0.14, 'rgba(255,255,255,0)');
    sh.addColorStop(0.24, 'rgba(225,240,255,.028)');
    sh.addColorStop(0.30, 'rgba(255,255,255,0)');
    sh.addColorStop(0.64, 'rgba(255,255,255,0)');
    sh.addColorStop(0.71, rgbCss(accent, 1, 0.022));
    sh.addColorStop(0.78, 'rgba(255,255,255,0)');
    g.fillStyle = sh; g.fillRect(0, 0, 512, 1024);
    g.strokeStyle = 'rgba(220,238,255,.055)'; g.lineWidth = 4;
    g.strokeRect(3, 3, 506, 1018);
    return c;
}

const bitmapCanvas = (img, w) => { const c = canvasOf(w, Math.round(w * img.height / img.width)); c.getContext('2d').drawImage(img, 0, 0, c.width, c.height); return c; };

// tableRoot children: cabinet body, trims, glass, apron + cards, head. images = { plastics, side, roomWall } (ImageBitmap | null).
export function buildCabinet({ tableRoot, layout, mats, tr, tableId, backglass, images = {}, aniso = 1, low = false }) {
    const accent = hexRgb(layout.meta && layout.meta.accent, [55, 217, 255]), secondary = hexRgb(layout.meta && layout.meta.secondary, [255, 72, 196]);
    const group = new THREE.Group(); group.name = 'cabinet'; tableRoot.add(group);
    const meshes = [], textures = [], own = [];
    const add = (geo, mat, name, parent = group) => { if (!geo) return null; const m = new THREE.Mesh(geo, mat); m.name = name; m.castShadow = false; m.receiveShadow = false; parent.add(m); meshes.push(m); return m; };
    const len = backY - frontY, cy = -(frontY + backY) / 2;
    const sideX = (sideIn + sideOut) / 2, sideW = sideOut - sideIn;
    const topBack = topAt(backY), topFront = topAt(frontY), wallTop = y => topAt(y) - 0.007;

    // Laminate body: wedge side walls, front, back, bottom and the rear housing under the backbox.
    add(merge([
        wedgeBox(sideIn, sideOut, frontY, backY, bottom, wallTop), wedgeBox(-sideOut, -sideIn, frontY, backY, bottom, wallTop),
        box(0, bottom + 0.012, cy, sideIn * 2, 0.024, len),
        box(0, -0.115, -frontY + 0.01, sideOut * 2, 0.25, 0.02),
        box(0, (topBack + bottom) / 2, -backY + 0.01, sideOut * 2, topBack - bottom, 0.02),
        box(0, (topBack + bottom) / 2, -backY - 0.07, (sideOut - 0.002) * 2, topBack - bottom, 0.14)
    ]), mats.laminate, 'body');
    add(box(0, -0.008, -(frontY / 2), sideIn * 2, 0.024, -frontY), mats.apron, 'apron');
    // Wooden inner side boards: glass-high faces at x = +-sideIn, grain repeats once per 0.25 m.
    const grain = p => [p[2] * -4, p[1] * 16];
    const xi = sideIn - 0.0005;
    add(merge([
        faces([[[xi, 0, -backY], [xi, 0, -frontY], [xi, glassHeight(frontY), -frontY], [xi, glassHeight(backY), -backY]]], grain),
        faces([[[-xi, 0, -frontY], [-xi, 0, -backY], [-xi, glassHeight(backY), -backY], [-xi, glassHeight(frontY), -frontY]]], grain)
    ]), mats.wood, 'woodSideBoards');
    // Side art: trapezoids following topAt(y); the 3:1 image covers the bounding rectangle (bottom .. topAt(backY)).
    const artW = low ? 1024 : 2048;
    const sa = canvasTexture(images.side ? bitmapCanvas(images.side, artW) : sideArt(layout, accent, secondary), { aniso: Math.min(4, aniso) }); textures.push(sa);
    const mx = sideOut + 0.0006, vOf = h => (h - bottom) / (topBack - bottom);
    const vF = vOf(topFront);
    // right panel: art runs front-to-back; left panel is mirrored so it also reads left-to-right from outside.
    const right = quadUV([mx, bottom, -frontY], [mx, bottom, -backY], [mx, topBack, -backY], [mx, topFront, -frontY], [0, 0], [1, 0], [1, 1], [0, vF]);
    const left = quadUV([-mx, bottom, -backY], [-mx, bottom, -frontY], [-mx, topFront, -frontY], [-mx, topBack, -backY], [0, 0], [1, 0], [1, vF], [0, 1]);
    const sideMat = new THREE.MeshStandardMaterial({ map: sa, roughness: 0.6, metalness: 0.05, envMap: mats.chrome.envMap, envMapIntensity: 0.15 }); own.push(sideMat);
    add(merge([left, right]), sideMat, 'sideArt');

    const chromeSoft = mats.chrome.clone(); chromeSoft.color.set('#9ea8b4'); chromeSoft.roughness = 0.62; chromeSoft.envMapIntensity = 0.26; own.push(chromeSoft);   // the tall glass-channel strips caught the key light as a white streak
    // Chrome: glass channel strips, lockdown bar, front trim, bezel lines.
    add(merge([
        wedgeBox(sideIn - 0.004, sideIn, frontY + 0.01, backY - 0.01, y => glassHeight(y) - 0.004, topAt), wedgeBox(-sideIn, -sideIn + 0.004, frontY + 0.01, backY - 0.01, y => glassHeight(y) - 0.004, topAt),
        box(0, topFront - 0.007, -(frontY + 0.009), sideIn * 2, 0.014, 0.014), box(0, 0.002, -(frontY - 0.0005), sideOut * 2 + 0.004, 0.012, 0.004),
        box(0, bottom + 0.05, -(frontY - 0.0005), 0.22, 0.12, 0.003)
    ]), chromeSoft, 'chrome');
    add(wedgeBox(sideIn - 0.004, sideOut + 0.004, frontY - 0.002, backY + 0.002, wallTop, topAt), mats.darkMetal, 'siderailCap');
    add(wedgeBox(-sideOut - 0.004, -sideIn + 0.004, frontY - 0.002, backY + 0.002, wallTop, topAt), mats.darkMetal, 'siderailCapL');
    add(merge([box(sideIn - 0.005, topFront / 2, -(frontY + 0.016), 0.010, topFront, 0.03), box(-sideIn + 0.005, topFront / 2, -(frontY + 0.016), 0.010, topFront, 0.03)]), mats.steel, 'lockdownBlocks');
    // Accent underglow strips and illuminated arcade flipper/start buttons.
    add(merge([
        box(sideOut + 0.001, bottom + 0.014, cy, 0.004, 0.007, len - 0.02), box(-sideOut - 0.001, bottom + 0.014, cy, 0.004, 0.007, len - 0.02),
        box(0, bottom + 0.014, -(frontY - 0.0012), sideOut * 2 - 0.02, 0.007, 0.004),
        new THREE.CylinderGeometry(0.0115, 0.0115, 0.006, 16).rotateZ(Math.PI / 2).translate(sideOut + 0.004, -0.045, -(frontY + 0.105)),
        new THREE.CylinderGeometry(0.0115, 0.0115, 0.006, 16).rotateZ(Math.PI / 2).translate(-sideOut - 0.004, -0.045, -(frontY + 0.105)),
        new THREE.CylinderGeometry(0.0095, 0.0095, 0.005, 14).rotateX(Math.PI / 2).translate(-0.175, -0.055, -(frontY - 0.003))
    ]), mats.glow, 'underglow');
    // Plunger front housing, shooter ball knob and button bezels.
    add(merge([
        new THREE.CylinderGeometry(0.0145, 0.0155, 0.003, 16).rotateZ(Math.PI / 2).translate(sideOut + 0.0015, -0.045, -(frontY + 0.105)),
        new THREE.CylinderGeometry(0.0145, 0.0155, 0.003, 16).rotateZ(Math.PI / 2).translate(-sideOut - 0.0015, -0.045, -(frontY + 0.105)),
        box(0.2405, -0.012, -(frontY - 0.006), 0.036, 0.036, 0.014),
        new THREE.CylinderGeometry(0.005, 0.005, 0.024, 12).rotateX(Math.PI / 2).translate(0.2405, -0.012, -(frontY - 0.022)),
        new THREE.SphereGeometry(0.0135, 16, 12).translate(0.2405, -0.012, -(frontY - 0.038))
    ]), mats.chrome, 'cabinetHardware');

    // Tempered playfield glass sheet on top of the glass (subtle studio softbox sheen; hidden in overhead/low/reduced modes).
    const pgTex = canvasTexture(playfieldGlassCanvas(accent), { aniso: 2, mip: false }); textures.push(pgTex);
    const pgMat = new THREE.MeshBasicMaterial({ map: pgTex, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }); own.push(pgMat);
    const gf = frontY + 0.012, gb = backY - 0.015, gx = sideIn - 0.003, gt = CABINET.glassThickness;
    const glassSheet = add(quad([-gx, glassHeight(gf) + gt, -gf], [gx, glassHeight(gf) + gt, -gf], [gx, glassHeight(gb) + gt, -gb], [-gx, glassHeight(gb) + gt, -gb]), pgMat, 'playfieldGlass');
    if (glassSheet) glassSheet.renderOrder = 8;

    // Apron plate (apron.js) and the two narrower instruction cards; the middle stays free for the printed badge.
    const apron = buildApron({ group, mats, accent: secondary, low, aniso });
    const scoreText = new Intl.NumberFormat('en-US').format((layout.meta && layout.meta.replayScore) || 40000000);
    const cardMat = canvasEl => {
        const t = canvasTexture(canvasEl, { aniso: Math.min(4, aniso) }); textures.push(t);
        const m = new THREE.MeshStandardMaterial({ map: t, roughness: 0.4, metalness: 0, emissive: '#ffffff', emissiveMap: t, emissiveIntensity: 0.42, envMap: mats.chrome.envMap, envMapIntensity: 0.3 });
        own.push(m); return m;
    };
    const cardQuad = (x, y, w, h) => quad([x - w / 2, 0.0125, -(y - h / 2)], [x + w / 2, 0.0125, -(y - h / 2)], [x + w / 2, 0.0125, -(y + h / 2)], [x - w / 2, 0.0125, -(y + h / 2)]);
    const rules = trText(tr, `card_rules_${tableId}`, undefined, '');
    const replay = trText(tr, 'card_replay', { score: scoreText }, 'REPLAY AT ' + scoreText);
    add(cardQuad(-0.1665, -0.064, 0.177, 0.08), cardMat(cardCanvas(rules, accent, false, 0.177 / 0.08)), 'cardRules');
    add(cardQuad(0.1375, -0.064, 0.151, 0.08), cardMat(cardCanvas(replay, secondary, true, 0.151 / 0.08)), 'cardReplay');

    // ---- head (backbox): its base sits on the back wall (topAt(backY)); un-tilted so it stands vertical in the room ----
    const head = new THREE.Group(); head.name = 'head'; head.position.set(0, topBack, -backY); head.rotation.x = -TILT; tableRoot.add(head);
    const W = sideOut * 2, bodyTop = 0.585;
    add(merge([box(0, (bodyTop - 0.02) / 2, -0.07, W, bodyTop + 0.02, 0.14)]), mats.laminate, 'headBody', head);
    const glassCanvas = backglass.canvas, glassH = (W - 0.05) * (glassCanvas.height / glassCanvas.width);
    const gy0 = 0.232, gTex = canvasTexture(glassCanvas, { aniso: Math.min(4, aniso) }); textures.push(gTex);
    const glassMat = new THREE.MeshBasicMaterial({ map: gTex, color: new THREE.Color(1.12, 1.12, 1.12) }); own.push(glassMat);
    const glass = new THREE.Mesh(new THREE.PlaneGeometry(W - 0.05, glassH), glassMat);
    glass.position.set(0, gy0 + glassH / 2, 0.002); head.add(glass); meshes.push(glass);
    const sp = canvasTexture(speakerPanel(accent), { aniso: Math.min(4, aniso) }); textures.push(sp);
    const panelMat = new THREE.MeshStandardMaterial({ map: sp, roughness: 0.55, metalness: 0.2, envMap: mats.chrome.envMap, envMapIntensity: 0.28 }); own.push(panelMat);
    const panel = new THREE.Mesh(new THREE.PlaneGeometry(W - 0.05, 0.19), panelMat);
    panel.position.set(0, 0.033 + 0.095, 0.002); head.add(panel); meshes.push(panel);
    const dmd = createDmdScreen(accent);
    const screen = new THREE.Mesh(new THREE.PlaneGeometry(0.34, 0.085), dmd.material);
    screen.position.set(0, 0.128, 0.004); head.add(screen); meshes.push(screen);
    const gw = W - 0.05;
    add(merge([
        box(0, 0.128 + 0.0525, 0.006, 0.364, 0.014, 0.012), box(0, 0.128 - 0.0525, 0.006, 0.364, 0.014, 0.012),
        box(0.177, 0.128, 0.006, 0.012, 0.1, 0.012), box(-0.177, 0.128, 0.006, 0.012, 0.1, 0.012),
        box(0, gy0 - 0.003, 0.004, gw + 0.006, 0.006, 0.008), box(0, gy0 + glassH + 0.003, 0.004, gw + 0.006, 0.006, 0.008),
        box(gw / 2 + 0.003, gy0 + glassH / 2, 0.004, 0.006, glassH + 0.012, 0.008), box(-gw / 2 - 0.003, gy0 + glassH / 2, 0.004, 0.006, glassH + 0.012, 0.008)
    ]), mats.steel, 'headTrim', head);
    add(merge([box(0, bodyTop - 0.012, 0.004, W - 0.06, 0.005, 0.006), box(0, 0.016, 0.004, W - 0.06, 0.005, 0.006)]), mats.glowSecondary, 'headGlow', head);
    return {
        group, head, dmd, glass: glassSheet, meshes, textures,
        dispose() {
            dmd.dispose(); apron.dispose();
            textures.forEach(t => t.dispose()); own.forEach(m => m.dispose());
            for (const m of meshes) m.geometry.dispose();
            tableRoot.remove(group, head);
        }
    };
}
