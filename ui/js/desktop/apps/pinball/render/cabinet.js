// AuraPinball cabinet: side rails with metal trim, lockdown bar, apron with instruction cards, side art, backbox with
// backglass + DMD screen, legs, floor and room backdrop. Geometry lives in tableRoot space (X = x, Y = h, Z = -y).
import * as THREE from 'three';
import { FONT_STACK } from '../config.js';
import { box, merge, quad, canvasTexture } from './materials.js';
import { hexRgb, rgbCss, trText } from './art-kit.js';

const TILT = 6.5 * Math.PI / 180;
export const DIM = Object.freeze({ frontY: -0.125, backY: 1.09, sideIn: 0.2675, sideOut: 0.2975, top: 0.075, bottom: -0.24 });
const { frontY, backY, sideIn, sideOut, top, bottom } = DIM;
const canvasOf = (w, h) => { const c = document.createElement('canvas'); c.width = w; c.height = h; return c; };
const LEVELS = [0, 89, 166, 255];

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
function cardCanvas(text, accent, big) {
    const c = canvasOf(1024, 400), g = c.getContext('2d');
    const bg = g.createLinearGradient(0, 0, 0, 400);
    bg.addColorStop(0, '#141a26'); bg.addColorStop(1, '#070a10'); g.fillStyle = bg; g.fillRect(0, 0, 1024, 400);
    g.shadowColor = rgbCss(accent, 1, 1); g.shadowBlur = 14; g.strokeStyle = rgbCss(accent, 1.1); g.lineWidth = 8;
    g.strokeRect(14, 14, 996, 372); g.shadowBlur = 0; g.lineWidth = 2; g.strokeStyle = 'rgba(255,255,255,.35)'; g.strokeRect(30, 30, 964, 340);
    const lines = String(text).split('\n').filter(Boolean);
    let fs = big ? 96 : 64;
    g.textAlign = 'center'; g.textBaseline = 'middle';
    const fit = () => { g.font = `800 ${fs}px ${FONT_STACK}`; return Math.max(...lines.map(l => g.measureText(l).width)); };
    while (fit() > 900 && fs > 20) fs -= 2;
    while (lines.length * fs * 1.28 > 310 && fs > 20) { fs -= 2; fit(); }
    g.fillStyle = '#f2f6ff'; g.shadowColor = rgbCss(accent, 1, 0.9); g.shadowBlur = 10;
    lines.forEach((l, i) => g.fillText(l, 512, 200 + (i - (lines.length - 1) / 2) * fs * 1.28));
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
    const data = new Uint8Array(192 * 48);
    const tex = new THREE.DataTexture(data, 192, 48, THREE.RedFormat, THREE.UnsignedByteType);
    tex.magFilter = tex.minFilter = THREE.NearestFilter; tex.generateMipmaps = false; tex.flipY = false; tex.unpackAlignment = 1; tex.needsUpdate = true;
    const material = new THREE.ShaderMaterial({
        uniforms: { frame: { value: tex }, dims: { value: new THREE.Vector2(192, 48) }, amber: { value: new THREE.Vector3(1.0, 0.27, 0.012) }, unlit: { value: new THREE.Vector3(0.026, 0.0045, 0.0006) } },
        vertexShader: 'varying vec2 vUv; void main(){ vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
        fragmentShader: `uniform sampler2D frame; uniform vec2 dims; uniform vec3 amber; uniform vec3 unlit; varying vec2 vUv;
        float lv(vec2 id){ return texture2D(frame, (id + 0.5) / dims).r; }
        void main(){
            vec2 cell = vec2(vUv.x, 1.0 - vUv.y) * dims; vec2 id = floor(cell); vec2 f = fract(cell) - 0.5;
            float l = lv(id), d = length(f), dotm = smoothstep(0.5, 0.3, d);
            float nb = (lv(id + vec2(1.0, 0.0)) + lv(id - vec2(1.0, 0.0)) + lv(id + vec2(0.0, 1.0)) + lv(id - vec2(0.0, 1.0))) * 0.25;
            vec3 col = mix(unlit, amber * 1.7, l) * dotm + amber * (l * 0.16 + nb * 0.07) * (1.0 - dotm);
            gl_FragColor = vec4(col, 1.0);
        }`
    });
    return {
        texture: tex, material,
        setFrame(frame) { for (let i = 0; i < 9216; i++) data[i] = LEVELS[frame[i] & 3]; tex.needsUpdate = true; },
        dispose() { tex.dispose(); material.dispose(); }
    };
}

// tableRoot children: cabinet body, trims, apron + cards, head. Returns pieces for update/dispose.
export function buildCabinet({ tableRoot, layout, mats, tr, tableId, backglass, aniso = 1 }) {
    const accent = hexRgb(layout.meta && layout.meta.accent, [55, 217, 255]), secondary = hexRgb(layout.meta && layout.meta.secondary, [255, 72, 196]);
    const group = new THREE.Group(); group.name = 'cabinet'; tableRoot.add(group);
    const meshes = [], textures = [], own = [];
    const add = (geo, mat, name, parent = group) => { if (!geo) return null; const m = new THREE.Mesh(geo, mat); m.name = name; m.castShadow = false; m.receiveShadow = false; parent.add(m); meshes.push(m); return m; };
    const len = backY - frontY, cy = -(frontY + backY) / 2, hh = top - bottom, ch = (top + bottom) / 2;
    const sideX = (sideIn + sideOut) / 2, sideW = sideOut - sideIn;

    // Laminate body: sides, front, back, bottom, apron slab, header boards.
    add(merge([
        box(sideX, ch, cy, sideW, hh, len), box(-sideX, ch, cy, sideW, hh, len),
        box(0, bottom + 0.012, cy, sideIn * 2, 0.024, len),
        box(0, -0.115, -frontY + 0.01, sideOut * 2, 0.25, 0.02),
        box(0, ch, -backY + 0.01, sideOut * 2, hh, 0.02)
    ]), mats.laminate, 'body');
    add(box(0, -0.008, -(frontY / 2) + 0.0, sideIn * 2, 0.024, -frontY), mats.apron, 'apron');
    // Side art planes (outer faces), art reads left-to-right for a viewer standing at each side.
    const sa = canvasTexture(sideArt(layout, accent, secondary), { aniso: Math.min(4, aniso) }); textures.push(sa);
    const mx = sideOut + 0.0006;
    const left = quad([-mx, bottom, -backY], [-mx, bottom, -frontY], [-mx, top, -frontY], [-mx, top, -backY], [0, 0, 1, 1]);
    const right = quad([mx, bottom, -frontY], [mx, bottom, -backY], [mx, top, -backY], [mx, top, -frontY], [0, 0, 1, 1]);
    const sideMat = new THREE.MeshStandardMaterial({ map: sa, roughness: 0.55, metalness: 0.05, envMap: mats.chrome.envMap, envMapIntensity: 0.22 }); own.push(sideMat);
    add(merge([left, right]), sideMat, 'sideArt');

    // Chrome: side-rail caps, lockdown bar, front trim, bezel lines.
    add(merge([
        box(sideIn - 0.003, top - 0.012, cy - 0.01, 0.004, 0.03, len - 0.04), box(-sideIn + 0.003, top - 0.012, cy - 0.01, 0.004, 0.03, len - 0.04),
        box(0, 0.032, -(frontY + 0.009), sideIn * 2, 0.014, 0.014), box(0, 0.002, -(frontY - 0.0005), sideOut * 2 + 0.004, 0.012, 0.004),
        box(0, bottom + 0.05, -(frontY - 0.0005), 0.22, 0.12, 0.003)
    ]), mats.chrome, 'chrome');
    add(merge([box(sideX, top + 0.003, cy, sideW + 0.008, 0.007, len + 0.004), box(-sideX, top + 0.003, cy, sideW + 0.008, 0.007, len + 0.004)]), mats.darkMetal, 'siderailCap');
    add(merge([box(sideIn - 0.026, 0.02, -(frontY + 0.016), 0.044, 0.034, 0.03), box(-sideIn + 0.026, 0.02, -(frontY + 0.016), 0.044, 0.034, 0.03)]), mats.steel, 'lockdownBlocks');
    // Accent underglow strips.
    add(merge([
        box(sideOut + 0.001, bottom + 0.014, cy, 0.004, 0.007, len - 0.02), box(-sideOut - 0.001, bottom + 0.014, cy, 0.004, 0.007, len - 0.02),
        box(0, bottom + 0.014, -(frontY - 0.0012), sideOut * 2 - 0.02, 0.007, 0.004)
    ]), mats.glow, 'underglow');

    // Apron instruction cards.
    const scoreText = new Intl.NumberFormat('en-US').format((layout.meta && layout.meta.replayScore) || 2000000);
    const cardMat = (canvasEl, hex) => {
        const t = canvasTexture(canvasEl, { aniso: Math.min(4, aniso) }); textures.push(t);
        const m = new THREE.MeshStandardMaterial({ map: t, roughness: 0.32, metalness: 0, emissive: '#ffffff', emissiveMap: t, emissiveIntensity: 0.42, envMap: mats.chrome.envMap, envMapIntensity: 0.5 });
        own.push(m); return m;
    };
    const cardQuad = (x, y, w, h) => quad([x - w / 2, 0.0052, -(y - h / 2)], [x + w / 2, 0.0052, -(y - h / 2)], [x + w / 2, 0.0052, -(y + h / 2)], [x - w / 2, 0.0052, -(y + h / 2)]);
    const rules = trText(tr, `card_rules_${tableId}`, undefined, '');
    const replay = trText(tr, 'card_replay', { score: scoreText }, 'REPLAY AT ' + scoreText);
    add(cardQuad(-0.112, -0.064, 0.205, 0.08), cardMat(cardCanvas(rules, accent, false)), 'cardRules');
    add(cardQuad(0.108, -0.064, 0.18, 0.08), cardMat(cardCanvas(replay, secondary, true)), 'cardReplay');

    // ---- head (backbox), un-tilted so it stands vertical in the room ----
    const head = new THREE.Group(); head.name = 'head'; head.position.set(0, 0, -backY); head.rotation.x = -TILT; tableRoot.add(head);
    const W = sideOut * 2, bodyTop = 0.585;
    add(merge([box(0, (bodyTop - 0.12) / 2, -0.07, W, bodyTop + 0.12, 0.14)]), mats.laminate, 'headBody', head);
    const glassCanvas = backglass.canvas, glassH = (W - 0.05) * (glassCanvas.height / glassCanvas.width);
    const gy0 = 0.232, gTex = canvasTexture(glassCanvas, { aniso: Math.min(4, aniso) }); textures.push(gTex);
    const glassMat = new THREE.MeshBasicMaterial({ map: gTex, color: new THREE.Color(1.12, 1.12, 1.12) }); own.push(glassMat);
    const glass = new THREE.Mesh(new THREE.PlaneGeometry(W - 0.05, glassH), glassMat);
    glass.position.set(0, gy0 + glassH / 2, 0.002); head.add(glass); meshes.push(glass);
    const sp = canvasTexture(speakerPanel(accent), { aniso: Math.min(4, aniso) }); textures.push(sp);
    const panelMat = new THREE.MeshStandardMaterial({ map: sp, roughness: 0.5, metalness: 0.2, envMap: mats.chrome.envMap, envMapIntensity: 0.4 }); own.push(panelMat);
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
        group, head, dmd, meshes, textures,
        dispose() {
            dmd.dispose();
            textures.forEach(t => t.dispose()); own.forEach(m => m.dispose());
            for (const m of meshes) m.geometry.dispose();
            tableRoot.remove(group, head);
        }
    };
}

function floorCanvas(accent, secondary) {
    const c = canvasOf(1024, 1024), g = c.getContext('2d');
    g.fillStyle = '#020307'; g.fillRect(0, 0, 1024, 1024);
    let r = g.createRadialGradient(512, 560, 20, 512, 560, 480);
    r.addColorStop(0, rgbCss(accent, 0.55, 0.55)); r.addColorStop(0.35, rgbCss(accent, 0.22, 0.28)); r.addColorStop(1, 'rgba(0,0,0,0)');
    g.fillStyle = r; g.fillRect(0, 0, 1024, 1024);
    r = g.createRadialGradient(512, 300, 10, 512, 300, 360);
    r.addColorStop(0, rgbCss(secondary, 0.4, 0.3)); r.addColorStop(1, 'rgba(0,0,0,0)'); g.fillStyle = r; g.fillRect(0, 0, 1024, 1024);
    g.strokeStyle = 'rgba(120,150,190,.07)'; g.lineWidth = 2;
    for (let i = 0; i <= 16; i++) { g.beginPath(); g.moveTo(i * 64, 0); g.lineTo(i * 64, 1024); g.moveTo(0, i * 64); g.lineTo(1024, i * 64); g.stroke(); }
    const vg = g.createRadialGradient(512, 512, 250, 512, 512, 512); vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, 'rgba(0,0,0,1)');
    g.fillStyle = vg; g.fillRect(0, 0, 1024, 1024);
    return c;
}

function wallCanvas(accent, secondary) {
    const c = canvasOf(1024, 512), g = c.getContext('2d');
    const bg = g.createLinearGradient(0, 0, 0, 512); bg.addColorStop(0, '#030409'); bg.addColorStop(1, '#0a0d16'); g.fillStyle = bg; g.fillRect(0, 0, 1024, 512);
    g.globalCompositeOperation = 'lighter';
    [[120, 80, 360, 14, accent], [520, 150, 380, 14, secondary], [260, 300, 520, 10, accent], [700, 360, 250, 10, secondary]].forEach(([x, y, w, h, col]) => {
        g.shadowColor = rgbCss(col, 1); g.shadowBlur = 40; g.fillStyle = rgbCss(col, 1.1, 0.8); g.fillRect(x, y, w, h);
    });
    return c;
}

// World-space room: legs, floor pool of light and a dark back wall. Returns {group, floorY, dispose}.
export function buildRoom({ scene, tableRoot, mats, accent, secondary }) {
    const acc = hexRgb(accent, [55, 217, 255]), sec = hexRgb(secondary, [255, 72, 196]);
    tableRoot.updateMatrixWorld(true);
    const world = (x, h, y) => new THREE.Vector3(x, h, -y).applyMatrix4(tableRoot.matrixWorld);
    const front = world(0.25, bottom, frontY + 0.04), floorY = front.y - 0.66;
    const group = new THREE.Group(); group.name = 'room'; scene.add(group);
    const legs = [], feet = [];
    for (const [x, y] of [[0.255, frontY + 0.04], [-0.255, frontY + 0.04], [0.255, backY - 0.04], [-0.255, backY - 0.04]]) {
        const p = world(x, bottom, y), l = p.y - floorY;
        legs.push(new THREE.BoxGeometry(0.055, l, 0.055).translate(p.x, floorY + l / 2, p.z));
        feet.push(new THREE.CylinderGeometry(0.03, 0.034, 0.02, 14).translate(p.x, floorY + 0.01, p.z));
    }
    const legMesh = new THREE.Mesh(merge(legs), mats.steel), footMesh = new THREE.Mesh(merge(feet), mats.rubberBlack);
    const ft = canvasTexture(floorCanvas(acc, sec), { aniso: 4 });
    const floor = new THREE.Mesh(new THREE.PlaneGeometry(9, 9), new THREE.MeshBasicMaterial({ map: ft }));
    floor.rotation.x = -Math.PI / 2; floor.position.set(0, floorY, -0.45);
    const wt = canvasTexture(wallCanvas(acc, sec), { aniso: 1 });
    const wall = new THREE.Mesh(new THREE.PlaneGeometry(12, 6), new THREE.MeshBasicMaterial({ map: wt }));
    wall.position.set(0, floorY + 3, -3.4);
    group.add(legMesh, footMesh, floor, wall);
    return {
        group, floorY,
        dispose() {
            for (const m of [legMesh, footMesh, floor, wall]) { m.geometry.dispose(); if (m.material !== mats.steel && m.material !== mats.rubberBlack) { if (m.material.map) m.material.map.dispose(); m.material.dispose(); } }
            scene.remove(group);
        }
    };
}
