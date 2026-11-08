// AuraPinball playfield: art canvas, lamp mask / state / GI textures, the patched playfield material (insert lamps glow
// from the 256x1 lamp texture), CPU emissive fallback and the wall geometry (wood rails, metal guides).
import * as THREE from 'three';
import { buildLampMask } from '../lights/mask.js';
import { FIELD_W, FIELD_H, lampPolygon } from '../lights/shapes.js';
import { createArtKit, hexRgb } from './art-kit.js';
import { canvasTexture, ribbonGeometry, merge } from './materials.js';
import { VISUALS } from '../tables/visuals.js';

export const FIELD_CENTER = [0, FIELD_H / 2];

function makeCanvas(w, h) { const c = document.createElement('canvas'); c.width = w; c.height = h; return c; }

// Paints the whole art stack for the table into a fresh canvas texture.
export function paintPlayfieldArt({ layout, tr, tableId, size, underlay, aniso }) {
    const [W, H] = size, canvas = makeCanvas(W, H), ctx = canvas.getContext('2d');
    const palette = { accent: layout.meta && layout.meta.accent, secondary: layout.meta && layout.meta.secondary };
    const kit = createArtKit(ctx, { W, H, layout, tr, palette, underlay });
    const visual = VISUALS[tableId] || null;
    kit.autoArt({ paint: visual && visual.paintArt ? visual.paintArt : null, brightness: 0.52 });
    const tex = canvasTexture(canvas, { aniso });
    return { texture: tex, canvas, kit };
}

// Lamp mask (nearest), lamp state (256x1 RGBA, nearest) and GI weight map (64x128: R lower, G upper).
export function createLampTextures(layout) {
    const mask = buildLampMask(layout, { width: 1024, height: 2048 });
    const maskTex = new THREE.DataTexture(mask.data, mask.width, mask.height, THREE.RGBAFormat, THREE.UnsignedByteType);
    maskTex.magFilter = maskTex.minFilter = THREE.NearestFilter; maskTex.generateMipmaps = false; maskTex.flipY = false; maskTex.needsUpdate = true;
    const stateData = new Uint8Array(1024);
    const stateTex = new THREE.DataTexture(stateData, 256, 1, THREE.RGBAFormat, THREE.UnsignedByteType);
    stateTex.magFilter = stateTex.minFilter = THREE.NearestFilter; stateTex.generateMipmaps = false; stateTex.flipY = false; stateTex.needsUpdate = true;
    const gw = 64, gh = 128, gi = new Uint8Array(gw * gh * 4), groups = (layout.gi || []);
    const sm = (a, b, x) => { const t = Math.max(0, Math.min(1, (x - a) / (b - a))); return t * t * (3 - 2 * t); };
    for (let j = 0; j < gh; j++) {
        const y = ((j + 0.5) / gh) * FIELD_H, w = [0, 0];
        for (const g of groups) {
            const k = g.id === 'upper' ? 1 : g.id === 'lower' ? 0 : -1;
            if (k < 0) continue;
            const lo = g.yMin <= 0.001 ? 1 : sm(g.yMin - 0.07, g.yMin + 0.07, y), hi = g.yMax >= FIELD_H - 0.001 ? 1 : 1 - sm(g.yMax - 0.07, g.yMax + 0.07, y);
            w[k] = Math.max(w[k], lo * hi);
        }
        const sum = w[0] + w[1];
        const r = sum > 0 ? w[0] / sum : 1, g2 = sum > 0 ? w[1] / sum : 0;
        for (let i = 0; i < gw; i++) { const o = (j * gw + i) * 4; gi[o] = Math.round(r * 255); gi[o + 1] = Math.round(g2 * 255); gi[o + 3] = 255; }
    }
    const giTex = new THREE.DataTexture(gi, gw, gh, THREE.RGBAFormat, THREE.UnsignedByteType);
    giTex.magFilter = giTex.minFilter = THREE.LinearFilter; giTex.generateMipmaps = false; giTex.flipY = false; giTex.needsUpdate = true;
    return { maskTex, stateTex, stateData, giTex, mask };
}

const FRAG_HEAD = `
uniform sampler2D lampMask; uniform sampler2D lampState; uniform sampler2D giMap;
uniform vec2 giLevel; uniform float lampGain; uniform float giFloor;
vec3 lampRgb(float idx){ return texture2D(lampState, vec2((idx + 0.5) / 256.0, 0.5)).rgb; }
`;
const AFTER_MAP = `
  vec4 pfMask = texture2D(lampMask, vMapUv);
  vec2 pfGw = texture2D(giMap, vMapUv).rg;
  float pfGi = clamp(pfGw.x * giLevel.x + pfGw.y * giLevel.y, 0.0, 1.0);
  diffuseColor.rgb *= mix(giFloor, 1.0, pfGi);
`;
const AFTER_EMISSIVE = `
  float pfIns = floor(pfMask.r * 255.0 + 0.5);
  if (pfIns > 0.5) {
    vec3 pfLit = pow(lampRgb(pfIns), vec3(2.2));
    vec2 pfT = vec2(0.0029, 0.00146);
    float pfHot = 0.0;
    pfHot += floor(texture2D(lampMask, vMapUv + vec2( pfT.x,  pfT.y)).r * 255.0 + 0.5) == pfIns ? 0.25 : 0.0;
    pfHot += floor(texture2D(lampMask, vMapUv + vec2(-pfT.x,  pfT.y)).r * 255.0 + 0.5) == pfIns ? 0.25 : 0.0;
    pfHot += floor(texture2D(lampMask, vMapUv + vec2( pfT.x, -pfT.y)).r * 255.0 + 0.5) == pfIns ? 0.25 : 0.0;
    pfHot += floor(texture2D(lampMask, vMapUv + vec2(-pfT.x, -pfT.y)).r * 255.0 + 0.5) == pfIns ? 0.25 : 0.0;
    float pfInk = smoothstep(0.015, 0.14, max(diffuseColor.r, max(diffuseColor.g, diffuseColor.b)));
    float pfPeak = max(pfLit.r, max(pfLit.g, pfLit.b));
    vec3 pfCore = pfLit * (0.5 + 1.15 * pfHot * pfHot) + vec3(pfPeak * pfHot * pfHot * pfHot * 0.55);
    totalEmissiveRadiance += pfCore * lampGain * mix(0.1, 1.0, pfInk);
  } else {
    float pfHalo = floor(pfMask.g * 255.0 + 0.5);
    if (pfHalo > 0.5) totalEmissiveRadiance += pow(lampRgb(pfHalo), vec3(2.2)) * pfMask.b * 0.55 * lampGain;
  }
`;

export function createPlayfieldMaterial({ artTex, lampTex, envMap, lampGain = 2.6, forceFallback = false }) {
    const uniforms = {
        lampMask: { value: lampTex.maskTex }, lampState: { value: lampTex.stateTex }, giMap: { value: lampTex.giTex },
        giLevel: { value: new THREE.Vector2(1, 1) }, lampGain: { value: lampGain }, giFloor: { value: 0.35 }
    };
    const material = new THREE.MeshPhysicalMaterial({
        map: artTex, color: '#ffffff', roughness: 0.55, metalness: 0.04, clearcoat: 0.55, clearcoatRoughness: 0.12,
        envMap, envMapIntensity: 0.4, emissive: '#000000'
    });
    let patched = !forceFallback;
    const patch = shader => {
        Object.assign(shader.uniforms, uniforms);
        shader.fragmentShader = FRAG_HEAD + shader.fragmentShader
            .replace('#include <map_fragment>', '#include <map_fragment>\n' + AFTER_MAP)
            .replace('#include <emissivemap_fragment>', '#include <emissivemap_fragment>\n' + AFTER_EMISSIVE);
    };
    if (patched) material.onBeforeCompile = patch;
    material.customProgramCacheKey = () => (patched ? 'pf-lamp-v1' : 'pf-plain');
    // CPU fallback: an emissive canvas redrawn at 30 Hz from the lamp state (insets + soft glow).
    let fb = null;
    const fallback = {
        get active() { return !patched; },
        enable() {
            if (fb) return;
            patched = false; material.onBeforeCompile = () => {};
            const c = makeCanvas(512, 1024);
            fb = { canvas: c, ctx: c.getContext('2d'), tex: new THREE.CanvasTexture(c), t: 0, polys: new Map() };
            fb.tex.colorSpace = THREE.SRGBColorSpace;
            material.emissiveMap = fb.tex; material.emissive.set('#ffffff'); material.emissiveIntensity = 1.2; material.needsUpdate = true;
        },
        update(layout, rgba, time) {
            if (!fb || time - fb.t < 1 / 30) return;
            fb.t = time;
            const g = fb.ctx, W = 512, H = 1024;
            g.globalCompositeOperation = 'source-over'; g.fillStyle = '#000'; g.fillRect(0, 0, W, H);
            g.globalCompositeOperation = 'lighter';
            (layout.lamps || []).forEach((lamp, i) => {
                if (lamp.mount && lamp.mount !== 'insert') return;
                const o = (i + 1) * 4, r = rgba[o], gg = rgba[o + 1], b = rgba[o + 2];
                if (r + gg + b < 12) return;
                let poly = fb.polys.get(i);
                if (!poly) { poly = lampPolygon(lamp, 20).map(p => [((p[0] + FIELD_W / 2) / FIELD_W) * W, (1 - p[1] / FIELD_H) * H]); fb.polys.set(i, poly); }
                g.shadowColor = `rgb(${r},${gg},${b})`; g.shadowBlur = 14; g.fillStyle = `rgb(${r},${gg},${b})`;
                g.beginPath(); poly.forEach((p, k) => (k ? g.lineTo(p[0], p[1]) : g.moveTo(p[0], p[1]))); g.closePath(); g.fill();
            });
            fb.tex.needsUpdate = true;
        },
        dispose() { if (fb) fb.tex.dispose(); fb = null; }
    };
    return { material, uniforms, fallback };
}

const WALL_T = { wood: 0.010, metal: 0.0036 };

// Merged wood rails (offset outward from the field centre, h 0..height) and metal guides (centred).
export function buildWallGeometry(layout) {
    const wood = [], metal = [];
    for (const w of layout.walls || []) {
        if ((w.render !== 'wood' && w.render !== 'metal') || !w.points || w.points.length < 2) continue;
        const wood1 = w.render === 'wood', h1 = Math.max(0.012, w.height || 0.03) * (wood1 ? 1.25 : 1);
        const g = ribbonGeometry(w.points, { thickness: WALL_T[w.render], h0: -0.002, h1, away: wood1 ? FIELD_CENTER : null, uvLen: 0.18 });
        if (g) (wood1 ? wood : metal).push(g);
    }
    return { wood: merge(wood), metal: merge(metal) };
}

export function hexToLinear(hex, k = 1) {
    const c = hexRgb(hex);
    return new THREE.Color(c[0] / 255, c[1] / 255, c[2] / 255).convertSRGBToLinear().multiplyScalar(k);
}
