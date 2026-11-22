// Pixel pure image library, 3D text part. DOM-free; loads in the main context, in pixel-worker.js and in a node vm.
// Provides font/texture/warp/bevel/preset catalogs, exact anti-aliased 2D SDF generation (Felzenszwalb-Huttenlocher),
// deterministic procedural textures (RGB albedo + A height/bump), 3D projection math, and CPU 3D SDF raymarching.
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    if (typeof Lib.registerOp !== 'function') return;

    const TAU = Math.PI * 2;
    const DEG = Math.PI / 180;
    const INF = 1e12;
    const clamp = (v, lo, hi) => (v < lo ? lo : (v > hi ? hi : v));
    const clamp01 = v => (v < 0 ? 0 : (v > 1 ? 1 : v));
    const mix = (a, b, t) => a + (b - a) * t;
    const fract = v => v - Math.floor(v);
    const fade = t => t * t * t * (t * (t * 6 - 15) + 10);
    const smoothstep = (e0, e1, x) => { const t = clamp01((x - e0) / ((e1 - e0) || 1e-9)); return t * t * (3 - 2 * t); };

    // --- deterministic hashing and seamless tileable value/fBm noise ---

    function mix32(x) {
        x ^= x >>> 16;
        x = Math.imul(x, 0x7FEB352D);
        x ^= x >>> 15;
        x = Math.imul(x, 0x846CA68B);
        return (x ^ (x >>> 16)) >>> 0;
    }

    function hash2(x, y, s) {
        return mix32((mix32((x | 0) ^ s) + Math.imul(y | 0, 0x9E3779B1)) | 0);
    }

    function wrapInt(v, p) { const m = v % p; return m < 0 ? m + p : m; }

    function tileNoise(u, v, px, py, seed) {
        const x = u * px, y = v * py;
        const xf = Math.floor(x), yf = Math.floor(y);
        const tx = fade(x - xf), ty = fade(y - yf);
        const x0 = wrapInt(xf, px), x1 = wrapInt(xf + 1, px);
        const y0 = wrapInt(yf, py), y1 = wrapInt(yf + 1, py);
        const inv = 1 / 4294967296;
        const a = hash2(x0, y0, seed) * inv, b = hash2(x1, y0, seed) * inv;
        const c = hash2(x0, y1, seed) * inv, d = hash2(x1, y1, seed) * inv;
        return mix(mix(a, b, tx), mix(c, d, tx), ty);
    }

    function tileFbm(u, v, px, py, octaves, seed) {
        let sum = 0, amp = 0.5, norm = 0, fx = px, fy = py;
        for (let i = 0; i < octaves; i++) {
            sum += tileNoise(u, v, fx, fy, seed + i * 97) * amp;
            norm += amp;
            amp *= 0.5;
            fx *= 2;
            fy *= 2;
        }
        return sum / (norm || 1);
    }

    function tileWorley(u, v, cells, seed) {
        const x = u * cells, y = v * cells;
        const ix = Math.floor(x), iy = Math.floor(y);
        const inv = 1 / 4294967296;
        let d1 = 9, d2 = 9;
        for (let oy = -1; oy <= 1; oy++) {
            for (let ox = -1; ox <= 1; ox++) {
                const cx = wrapInt(ix + ox, cells), cy = wrapInt(iy + oy, cells);
                const h1 = hash2(cx, cy, seed) * inv;
                const h2 = hash2(cx, cy, seed ^ 0x517CC1B7) * inv;
                const dx = ix + ox + h1 - x, dy = iy + oy + h2 - y;
                const dist = Math.hypot(dx, dy);
                if (dist < d1) { d2 = d1; d1 = dist; } else if (dist < d2) d2 = dist;
            }
        }
        return [d1, d2];
    }

    function parseHex(hex, fallback) {
        const s = typeof hex === 'string' ? hex.replace('#', '').trim() : '';
        if (/^[0-9a-f]{6}$/i.test(s)) {
            return [parseInt(s.slice(0, 2), 16) / 255, parseInt(s.slice(2, 4), 16) / 255, parseInt(s.slice(4, 6), 16) / 255];
        }
        return fallback || [0.95, 0.78, 0.32];
    }

    // --- catalogs ---

    const FONTS = [
        { id: 'oxanium', name: 'Oxanium', stack: "'Oxanium', sans-serif", cat: 'display', weight: 700 },
        { id: 'barlow-condensed', name: 'Barlow Condensed', stack: "'Barlow Condensed', sans-serif", cat: 'display', weight: 600 },
        { id: 'montserrat', name: 'Montserrat', stack: "'Montserrat', sans-serif", cat: 'display', weight: 700 },
        { id: 'exo-2', name: 'Exo 2', stack: "'Exo 2', sans-serif", cat: 'display', weight: 700 },
        { id: 'geist', name: 'Geist', stack: "'Geist', sans-serif", cat: 'sans', weight: 700 },
        { id: 'inter', name: 'Inter', stack: "'Inter', sans-serif", cat: 'sans', weight: 700 },
        { id: 'darker-grotesque', name: 'Darker Grotesque', stack: "'Darker Grotesque', sans-serif", cat: 'display', weight: 700 },
        { id: 'roboto-condensed', name: 'Roboto Condensed', stack: "'Roboto Condensed', sans-serif", cat: 'sans', weight: 700 },
        { id: 'impact', name: 'Impact', stack: "Impact, 'Barlow Condensed', sans-serif", cat: 'poster', weight: 700 },
        { id: 'liberation-serif', name: 'Liberation Serif', stack: "'Liberation Serif', Georgia, serif", cat: 'serif', weight: 700 },
        { id: 'georgia', name: 'Georgia', stack: "Georgia, 'Liberation Serif', serif", cat: 'serif', weight: 700 },
        { id: 'press-start-2p', name: 'Press Start 2P', stack: "'Press Start 2P', monospace", cat: 'retro', weight: 400 },
        { id: 'special-elite', name: 'Special Elite', stack: "'Special Elite', monospace", cat: 'retro', weight: 400 },
        { id: 'schoolbell', name: 'Schoolbell', stack: "'Schoolbell', cursive", cat: 'hand', weight: 400 },
        { id: 'shadows-into-light', name: 'Shadows Into Light', stack: "'Shadows Into Light Two', cursive", cat: 'hand', weight: 400 },
        { id: 'share-tech-mono', name: 'Share Tech Mono', stack: "'Share Tech Mono', monospace", cat: 'mono', weight: 400 },
        { id: 'jetbrains-mono', name: 'JetBrains Mono', stack: "'JetBrains Mono', monospace", cat: 'mono', weight: 600 },
        { id: 'c64-pro-mono', name: 'C64 Pro Mono', stack: "'C64 Pro Mono', monospace", cat: 'retro', weight: 400 }
    ];

    const TEXTURES = [
        { id: 'solid', labelKey: 'pixel.text3d_tex_solid', fallback: 'Solid', metalness: 0.12, gloss: 0.45, bump: 0.0, emissive: 0.0, reflect: 0.18, holo: 0, tint: true, side: '#2b3a55' },
        { id: 'gold', labelKey: 'pixel.text3d_tex_gold', fallback: 'Gold', metalness: 0.92, gloss: 0.86, bump: 0.22, emissive: 0.0, reflect: 0.88, holo: 0, tint: false, side: '#8a5a14' },
        { id: 'chrome', labelKey: 'pixel.text3d_tex_chrome', fallback: 'Chrome', metalness: 0.98, gloss: 0.96, bump: 0.08, emissive: 0.0, reflect: 0.98, holo: 0, tint: false, side: '#6c7788' },
        { id: 'brushed-metal', labelKey: 'pixel.text3d_tex_brushed', fallback: 'Brushed Steel', metalness: 0.85, gloss: 0.65, bump: 0.42, emissive: 0.0, reflect: 0.72, holo: 0, tint: false, side: '#4f5866' },
        { id: 'copper', labelKey: 'pixel.text3d_tex_copper', fallback: 'Copper', metalness: 0.9, gloss: 0.78, bump: 0.25, emissive: 0.0, reflect: 0.82, holo: 0, tint: false, side: '#6e3219' },
        { id: 'wood', labelKey: 'pixel.text3d_tex_wood', fallback: 'Oak Wood', metalness: 0.05, gloss: 0.3, bump: 0.62, emissive: 0.0, reflect: 0.1, holo: 0, tint: false, side: '#4a2b14' },
        { id: 'marble', labelKey: 'pixel.text3d_tex_marble', fallback: 'Marble', metalness: 0.08, gloss: 0.84, bump: 0.18, emissive: 0.0, reflect: 0.48, holo: 0, tint: false, side: '#8d929c' },
        { id: 'stone', labelKey: 'pixel.text3d_tex_stone', fallback: 'Granite', metalness: 0.04, gloss: 0.2, bump: 0.78, emissive: 0.0, reflect: 0.08, holo: 0, tint: false, side: '#3d4147' },
        { id: 'brick', labelKey: 'pixel.text3d_tex_brick', fallback: 'Brick', metalness: 0.02, gloss: 0.18, bump: 0.85, emissive: 0.0, reflect: 0.05, holo: 0, tint: false, side: '#66281c' },
        { id: 'carbon', labelKey: 'pixel.text3d_tex_carbon', fallback: 'Carbon Fiber', metalness: 0.45, gloss: 0.75, bump: 0.55, emissive: 0.0, reflect: 0.52, holo: 0, tint: false, side: '#1b1f26' },
        { id: 'lava', labelKey: 'pixel.text3d_tex_lava', fallback: 'Molten Lava', metalness: 0.08, gloss: 0.32, bump: 0.75, emissive: 0.82, reflect: 0.12, holo: 0, tint: false, side: '#2b120c' },
        { id: 'ice', labelKey: 'pixel.text3d_tex_ice', fallback: 'Frosted Ice', metalness: 0.18, gloss: 0.92, bump: 0.46, emissive: 0.12, reflect: 0.76, holo: 0, tint: false, side: '#3b7fa8' },
        { id: 'neon', labelKey: 'pixel.text3d_tex_neon', fallback: 'Neon Cyber', metalness: 0.25, gloss: 0.88, bump: 0.15, emissive: 0.65, reflect: 0.58, holo: 0, tint: false, side: '#210f4d' },
        { id: 'holo', labelKey: 'pixel.text3d_tex_holo', fallback: 'Holographic', metalness: 0.82, gloss: 0.92, bump: 0.14, emissive: 0.18, reflect: 0.9, holo: 1, tint: false, side: '#3f3d7a' },
        { id: 'camo', labelKey: 'pixel.text3d_tex_camo', fallback: 'Camouflage', metalness: 0.02, gloss: 0.16, bump: 0.3, emissive: 0.0, reflect: 0.05, holo: 0, tint: false, side: '#2d3a24' },
        { id: 'checker', labelKey: 'pixel.text3d_tex_checker', fallback: 'Checker', metalness: 0.12, gloss: 0.76, bump: 0.12, emissive: 0.0, reflect: 0.42, holo: 0, tint: false, side: '#1c1e24' },
        { id: 'gradient', labelKey: 'pixel.text3d_tex_gradient', fallback: 'Gradient', metalness: 0.22, gloss: 0.7, bump: 0.0, emissive: 0.05, reflect: 0.38, holo: 0, tint: true, side: '#25314d' },
        { id: 'nebula', labelKey: 'pixel.text3d_tex_nebula', fallback: 'Galaxy', metalness: 0.2, gloss: 0.78, bump: 0.12, emissive: 0.45, reflect: 0.45, holo: 0, tint: false, side: '#150d33' }
    ];

    const WARPS = [
        { id: 'none', code: 0, labelKey: 'pixel.text3d_warp_none', fallback: 'None', hasFreq: false, lip: 0.9 },
        { id: 'arc', code: 1, labelKey: 'pixel.text3d_warp_arc', fallback: 'Arc', hasFreq: false, lip: 0.55 },
        { id: 'arch', code: 2, labelKey: 'pixel.text3d_warp_arch', fallback: 'Arch', hasFreq: false, lip: 0.65 },
        { id: 'wave', code: 3, labelKey: 'pixel.text3d_warp_wave', fallback: 'Wave', hasFreq: true, lip: 0.52 },
        { id: 'flag', code: 4, labelKey: 'pixel.text3d_warp_flag', fallback: '3D Flag', hasFreq: true, lip: 0.45 },
        { id: 'bulge', code: 5, labelKey: 'pixel.text3d_warp_bulge', fallback: 'Bulge', hasFreq: false, lip: 0.58 },
        { id: 'cylinder', code: 6, labelKey: 'pixel.text3d_warp_cylinder', fallback: '3D Cylinder', hasFreq: false, lip: 0.48 },
        { id: 'twist', code: 7, labelKey: 'pixel.text3d_warp_twist', fallback: '3D Twist', hasFreq: false, lip: 0.45 },
        { id: 'bridge', code: 8, labelKey: 'pixel.text3d_warp_bridge', fallback: 'Bridge', hasFreq: false, lip: 0.58 },
        { id: 'wedge', code: 9, labelKey: 'pixel.text3d_warp_wedge', fallback: 'Wedge', hasFreq: false, lip: 0.6 },
        { id: 'rise', code: 10, labelKey: 'pixel.text3d_warp_rise', fallback: 'Rise', hasFreq: false, lip: 0.65 },
        { id: 'dome', code: 11, labelKey: 'pixel.text3d_warp_dome', fallback: '3D Dome', hasFreq: false, lip: 0.55 }
    ];

    const BEVELS = [
        { id: 'round', code: 0, labelKey: 'pixel.text3d_bevel_round', fallback: 'Round' },
        { id: 'chamfer', code: 1, labelKey: 'pixel.text3d_bevel_chamfer', fallback: 'Chamfer' },
        { id: 'sharp', code: 2, labelKey: 'pixel.text3d_bevel_sharp', fallback: 'Sharp' },
        { id: 'inflate', code: 3, labelKey: 'pixel.text3d_bevel_inflate', fallback: 'Inflate' }
    ];

    const ANGLE_PRESETS = [
        { id: 'front', labelKey: 'pixel.text3d_angle_front', fallback: 'Front', pitch: 0, yaw: 0, roll: 0, perspective: 35 },
        { id: 'hero-left', labelKey: 'pixel.text3d_angle_hero_left', fallback: 'Hero L', pitch: -14, yaw: 28, roll: -3, perspective: 55 },
        { id: 'hero-right', labelKey: 'pixel.text3d_angle_hero_right', fallback: 'Hero R', pitch: -14, yaw: -28, roll: 3, perspective: 55 },
        { id: 'low', labelKey: 'pixel.text3d_angle_low', fallback: 'Low', pitch: -34, yaw: 12, roll: 0, perspective: 68 },
        { id: 'top', labelKey: 'pixel.text3d_angle_top', fallback: 'Top Tilt', pitch: 34, yaw: -16, roll: 0, perspective: 50 },
        { id: 'iso', labelKey: 'pixel.text3d_angle_iso', fallback: 'Isometric', pitch: 28, yaw: -36, roll: -12, perspective: 18 },
        { id: 'dramatic', labelKey: 'pixel.text3d_angle_dramatic', fallback: 'Dramatic', pitch: -22, yaw: 44, roll: -8, perspective: 82 }
    ];

    const STYLE_PRESETS = [
        {
            id: 'cinema-gold', labelKey: 'pixel.text3d_preset_gold', fallback: 'Cinema Gold',
            values: { font: 'barlow-condensed', bold: true, texture: 'gold', warp: 'arc', warpStrength: 28, depth: 46, bevel: 36, bevelStyle: 'chamfer', pitch: -14, yaw: 24, roll: -2, perspective: 56, shadow: true, sideMatchTexture: true }
        },
        {
            id: 'liquid-chrome', labelKey: 'pixel.text3d_preset_chrome', fallback: 'Liquid Chrome',
            values: { font: 'oxanium', bold: true, texture: 'chrome', warp: 'cylinder', warpStrength: 38, depth: 52, bevel: 45, bevelStyle: 'round', pitch: -10, yaw: -24, roll: 0, perspective: 60, shadow: true, sideMatchTexture: true }
        },
        {
            id: 'neon-wave', labelKey: 'pixel.text3d_preset_neon', fallback: 'Neon Wave',
            values: { font: 'exo-2', bold: true, texture: 'neon', warp: 'flag', warpStrength: 42, warpFreq: 2, depth: 36, bevel: 30, bevelStyle: 'round', pitch: -8, yaw: 18, roll: -3, perspective: 50, shadow: true, sideMatchTexture: false, sideColor: '#241152' }
        },
        {
            id: 'magma-twist', labelKey: 'pixel.text3d_preset_lava', fallback: 'Magma Twist',
            values: { font: 'impact', bold: true, texture: 'lava', warp: 'twist', warpStrength: 36, depth: 56, bevel: 24, bevelStyle: 'chamfer', pitch: -12, yaw: -20, roll: 2, perspective: 62, shadow: true, sideMatchTexture: true }
        },
        {
            id: 'marble-arch', labelKey: 'pixel.text3d_preset_marble', fallback: 'Marble Arch',
            values: { font: 'liberation-serif', bold: true, texture: 'marble', warp: 'bridge', warpStrength: 42, depth: 48, bevel: 40, bevelStyle: 'chamfer', pitch: 18, yaw: -22, roll: 0, perspective: 45, shadow: true, sideMatchTexture: true }
        },
        {
            id: 'retro-pixel', labelKey: 'pixel.text3d_preset_retro', fallback: 'Retro 8-Bit',
            values: { font: 'press-start-2p', bold: false, texture: 'holo', warp: 'wave', warpStrength: 34, warpFreq: 2, depth: 62, bevel: 10, bevelStyle: 'sharp', pitch: -18, yaw: 28, roll: -4, perspective: 42, shadow: true, sideMatchTexture: false, sideColor: '#2d2463' }
        }
    ];

    const DEFAULTS = Object.freeze({
        text: 'AURAGO',
        font: 'oxanium',
        bold: true,
        italic: false,
        align: 'center',
        fontSize: 140,
        letterSpacing: 4,
        lineHeight: 1.15,
        posX: 0.5,
        posY: 0.5,
        pitch: -12,
        yaw: 22,
        roll: 0,
        perspective: 52,
        depth: 42,
        bevel: 34,
        bevelStyle: 'round',
        warp: 'none',
        warpStrength: 40,
        warpFreq: 2,
        texture: 'gold',
        color: '#f5b83d',
        sideColor: '#3a4b6e',
        sideMatchTexture: true,
        texScale: 100,
        texRotate: 0,
        lightAzimuth: 38,
        lightElevation: 48,
        gloss: 75,
        metalness: 75,
        bumpStrength: 40,
        shadow: true,
        shadowOpacity: 48,
        shadowBlur: 35,
        dragMode: 'auto'
    });

    const fontById = id => FONTS.find(f => f.id === id) || FONTS[0];
    const textureById = id => TEXTURES.find(t => t.id === id) || TEXTURES[0];
    const warpById = id => WARPS.find(w => w.id === id) || WARPS[0];
    const bevelById = id => BEVELS.find(b => b.id === id) || BEVELS[0];

    function defaultParams(overrides) {
        return normalizeParams(Object.assign({}, DEFAULTS, overrides || {}));
    }

    function normalizeParams(input) {
        const p = Object.assign({}, DEFAULTS, input || {});
        p.text = typeof p.text === 'string' ? p.text.slice(0, 240) : DEFAULTS.text;
        p.font = fontById(p.font).id;
        p.bold = !!p.bold;
        p.italic = !!p.italic;
        p.align = p.align === 'left' || p.align === 'right' ? p.align : 'center';
        p.fontSize = clamp(Math.round(Number(p.fontSize) || DEFAULTS.fontSize), 16, 800);
        p.letterSpacing = clamp(Math.round(Number(p.letterSpacing) || 0), -20, 80);
        p.lineHeight = clamp(Number(p.lineHeight) || DEFAULTS.lineHeight, 0.8, 2.2);
        p.posX = clamp(Number.isFinite(Number(p.posX)) ? Number(p.posX) : 0.5, -0.5, 1.5);
        p.posY = clamp(Number.isFinite(Number(p.posY)) ? Number(p.posY) : 0.5, -0.5, 1.5);
        p.pitch = clamp(Number.isFinite(Number(p.pitch)) ? Number(p.pitch) : DEFAULTS.pitch, -85, 85);
        p.yaw = clamp(Number.isFinite(Number(p.yaw)) ? Number(p.yaw) : DEFAULTS.yaw, -180, 180);
        p.roll = clamp(Number.isFinite(Number(p.roll)) ? Number(p.roll) : 0, -180, 180);
        p.perspective = clamp(Number.isFinite(Number(p.perspective)) ? Number(p.perspective) : DEFAULTS.perspective, 0, 100);
        p.depth = clamp(Number.isFinite(Number(p.depth)) ? Number(p.depth) : DEFAULTS.depth, 0, 100);
        p.bevel = clamp(Number.isFinite(Number(p.bevel)) ? Number(p.bevel) : DEFAULTS.bevel, 0, 100);
        p.bevelStyle = bevelById(p.bevelStyle).id;
        p.warp = warpById(p.warp).id;
        p.warpStrength = clamp(Number.isFinite(Number(p.warpStrength)) ? Number(p.warpStrength) : DEFAULTS.warpStrength, -100, 100);
        p.warpFreq = clamp(Number.isFinite(Number(p.warpFreq)) ? Number(p.warpFreq) : DEFAULTS.warpFreq, 0.5, 5);
        p.texture = textureById(p.texture).id;
        p.color = /^#[0-9a-f]{6}$/i.test(p.color) ? p.color : DEFAULTS.color;
        p.sideColor = /^#[0-9a-f]{6}$/i.test(p.sideColor) ? p.sideColor : DEFAULTS.sideColor;
        p.sideMatchTexture = !!p.sideMatchTexture;
        p.texScale = clamp(Number(p.texScale) || 100, 25, 300);
        p.texRotate = clamp(Number(p.texRotate) || 0, 0, 360);
        p.lightAzimuth = clamp(Number.isFinite(Number(p.lightAzimuth)) ? Number(p.lightAzimuth) : DEFAULTS.lightAzimuth, -180, 180);
        p.lightElevation = clamp(Number.isFinite(Number(p.lightElevation)) ? Number(p.lightElevation) : DEFAULTS.lightElevation, 5, 88);
        p.gloss = clamp(Number.isFinite(Number(p.gloss)) ? Number(p.gloss) : DEFAULTS.gloss, 0, 100);
        p.metalness = clamp(Number.isFinite(Number(p.metalness)) ? Number(p.metalness) : DEFAULTS.metalness, 0, 100);
        p.bumpStrength = clamp(Number.isFinite(Number(p.bumpStrength)) ? Number(p.bumpStrength) : DEFAULTS.bumpStrength, 0, 100);
        p.shadow = !!p.shadow;
        p.shadowOpacity = clamp(Number.isFinite(Number(p.shadowOpacity)) ? Number(p.shadowOpacity) : DEFAULTS.shadowOpacity, 0, 100);
        p.shadowBlur = clamp(Number.isFinite(Number(p.shadowBlur)) ? Number(p.shadowBlur) : DEFAULTS.shadowBlur, 0, 100);
        p.dragMode = p.dragMode === 'rotate' || p.dragMode === 'move' ? p.dragMode : 'auto';
        return p;
    }

    // --- procedural seamless texture generator (RGB = albedo, A = height/bump) ---

    function sampleProceduralTexel(id, u, v, primary, secondary) {
        let r = 0.8, g = 0.8, b = 0.8, h = 0.5;
        if (id === 'solid') {
            r = primary[0]; g = primary[1]; b = primary[2]; h = 0.5;
        } else if (id === 'gold') {
            const streak = tileFbm(u, v, 3, 14, 3, 101);
            const micro = tileNoise(u, v, 6, 48, 107);
            const k = 0.86 + streak * 0.2 + (micro - 0.5) * 0.05;
            r = clamp01(1.0 * k); g = clamp01(0.78 * k); b = clamp01(0.24 * k);
            h = clamp01(0.46 + streak * 0.24 + micro * 0.12);
        } else if (id === 'chrome') {
            const n = tileFbm(u, v, 4, 16, 3, 201);
            const k = 0.84 + n * 0.14;
            r = k * 0.94; g = k * 0.97; b = k;
            h = 0.5 + (n - 0.5) * 0.2;
        } else if (id === 'brushed-metal') {
            const grain = tileFbm(u, v, 4, 64, 3, 303);
            const fine = tileNoise(u, v, 8, 128, 307);
            const k = 0.62 + grain * 0.28 + fine * 0.12;
            r = k * 0.88; g = k * 0.92; b = k * 0.98;
            h = clamp01(grain * 0.65 + fine * 0.35);
        } else if (id === 'copper') {
            const grain = tileFbm(u, v, 6, 32, 3, 401);
            const patina = smoothstep(0.66, 0.84, tileFbm(u, v, 6, 6, 4, 419));
            const k = 0.78 + grain * 0.28;
            r = mix(clamp01(0.9 * k), 0.28, patina * 0.35);
            g = mix(clamp01(0.47 * k), 0.62, patina * 0.35);
            b = mix(clamp01(0.28 * k), 0.55, patina * 0.35);
            h = clamp01(0.4 + grain * 0.45 + patina * 0.15);
        } else if (id === 'wood') {
            const warp = tileFbm(u, v, 4, 8, 4, 501) * 0.24;
            const ring = Math.sin((u * 9 + warp * 12) * TAU);
            const fiber = tileNoise(u, v, 64, 8, 509);
            const t = clamp01(0.5 + 0.5 * ring) * 0.72 + fiber * 0.28;
            r = mix(0.32, 0.74, t); g = mix(0.18, 0.49, t); b = mix(0.08, 0.25, t);
            h = clamp01(0.25 + t * 0.55 + fiber * 0.2);
        } else if (id === 'marble') {
            const f1 = tileFbm(u, v, 4, 4, 5, 601);
            const f2 = tileFbm(u + f1 * 0.35, v + f1 * 0.2, 6, 6, 4, 613);
            const vein = Math.pow(clamp01(1 - Math.abs(Math.sin((u * 3 + v * 2 + f1 * 3.2) * Math.PI))), 7);
            const goldVein = Math.pow(clamp01(1 - Math.abs(Math.sin((u * 2 - v * 3 + f2 * 2.5) * Math.PI))), 14);
            const base = 0.88 + f2 * 0.1;
            r = mix(mix(base, 0.38, vein * 0.65), 0.82, goldVein * 0.55);
            g = mix(mix(base * 0.99, 0.41, vein * 0.65), 0.67, goldVein * 0.55);
            b = mix(mix(base * 1.01, 0.46, vein * 0.65), 0.36, goldVein * 0.55);
            h = clamp01(0.7 - vein * 0.35);
        } else if (id === 'stone') {
            const coarse = tileFbm(u, v, 8, 8, 4, 701);
            const speck = tileNoise(u, v, 48, 48, 709);
            const mica = smoothstep(0.84, 0.96, tileNoise(u, v, 64, 64, 719));
            const k = 0.24 + coarse * 0.36 + (speck - 0.5) * 0.24 + mica * 0.28;
            r = clamp01(k * 0.97); g = clamp01(k * 0.99); b = clamp01(k * 1.04);
            h = clamp01(coarse * 0.55 + speck * 0.45);
        } else if (id === 'brick') {
            const rows = 6, cols = 3;
            const ry = v * rows, row = Math.floor(ry), fy = ry - row;
            const rx = u * cols + (row & 1 ? 0.5 : 0), col = Math.floor(rx), fx = rx - col;
            const mortar = (fy < 0.11 || fy > 0.96 || fx < 0.055 || fx > 0.975) ? 0 : 1;
            const vary = (hash2(wrapInt(col, cols), wrapInt(row, rows), 809) / 4294967296 - 0.5) * 0.18;
            const grit = tileFbm(u, v, 16, 16, 3, 811);
            if (mortar) {
                r = clamp01(0.68 + vary + (grit - 0.5) * 0.16);
                g = clamp01(0.24 + vary * 0.5 + (grit - 0.5) * 0.1);
                b = clamp01(0.16 + vary * 0.4 + (grit - 0.5) * 0.08);
                h = clamp01(0.7 + grit * 0.3);
            } else {
                const m = 0.68 + (grit - 0.5) * 0.16;
                r = m * 0.96; g = m * 0.94; b = m * 0.89;
                h = 0.15 + grit * 0.15;
            }
        } else if (id === 'carbon') {
            const cells = 12;
            const gx = u * cells, gy = v * cells;
            const ix = Math.floor(gx), iy = Math.floor(gy);
            const fx = gx - ix, fy = gy - iy;
            const dir = ((ix + iy) & 1) === 0;
            const strand = dir ? Math.sin(fx * Math.PI) : Math.sin(fy * Math.PI);
            const cross = dir ? Math.sin(fy * Math.PI) : Math.sin(fx * Math.PI);
            const sheen = 0.12 + strand * (0.22 + 0.12 * cross) + (dir ? 0.05 : -0.02);
            r = clamp01(sheen * 0.95); g = clamp01(sheen * 1.0); b = clamp01(sheen * 1.08);
            h = clamp01(0.25 + strand * 0.7);
        } else if (id === 'lava') {
            const w = tileWorley(u, v, 5, 901);
            const turb = tileFbm(u, v, 8, 8, 4, 911);
            const crack = smoothstep(0.18, 0.02, (w[1] - w[0]) + (turb - 0.5) * 0.09);
            const heat = clamp01(crack * 1.15);
            const crust = 0.11 + turb * 0.14;
            r = mix(crust * 1.2, 1.0, smoothstep(0.05, 0.45, heat));
            g = mix(crust * 0.9, 0.82, smoothstep(0.3, 0.9, heat));
            b = mix(crust * 0.85, 0.28, smoothstep(0.7, 1.0, heat));
            h = clamp01(0.85 - heat * 0.72 + (turb - 0.5) * 0.15);
        } else if (id === 'ice') {
            const w = tileWorley(u, v, 6, 1009);
            const frost = tileFbm(u, v, 8, 8, 4, 1013);
            const edge = smoothstep(0.16, 0.01, w[1] - w[0]);
            const k = clamp01(0.55 + frost * 0.35 + edge * 0.4);
            r = clamp01(mix(0.45, 0.96, k)); g = clamp01(mix(0.76, 0.99, k)); b = clamp01(mix(0.95, 1.0, k));
            h = clamp01(0.55 + frost * 0.3 - edge * 0.35);
        } else if (id === 'neon') {
            const wave = 0.5 + 0.5 * Math.sin((u * 2 + v * 1.5 + tileFbm(u, v, 4, 4, 3, 1103) * 0.4) * TAU);
            const grid = Math.max(smoothstep(0.92, 0.99, Math.abs(Math.sin(u * 8 * Math.PI))), smoothstep(0.92, 0.99, Math.abs(Math.sin(v * 8 * Math.PI))));
            r = mix(0.08, 0.98, wave) + grid * 0.25;
            g = mix(0.95, 0.14, wave) + grid * 0.2;
            b = mix(0.98, 0.86, wave) + grid * 0.25;
            r = clamp01(r); g = clamp01(g); b = clamp01(b);
            h = clamp01(0.5 + grid * 0.35);
        } else if (id === 'holo') {
            const shift = tileFbm(u, v, 3, 3, 3, 1201) * 0.6;
            const ang = (u * 1.5 + v * 1.2 + shift) * TAU;
            r = 0.58 + 0.42 * Math.sin(ang);
            g = 0.58 + 0.42 * Math.sin(ang + 2.094);
            b = 0.58 + 0.42 * Math.sin(ang + 4.188);
            h = clamp01(0.5 + 0.25 * Math.sin(ang * 2));
        } else if (id === 'camo') {
            const n1 = tileFbm(u, v, 4, 4, 3, 1301);
            const n2 = tileFbm(u + 0.37, v + 0.19, 5, 5, 3, 1319);
            let col;
            if (n1 < 0.38) col = [0.22, 0.29, 0.16];
            else if (n1 < 0.58) col = [0.42, 0.47, 0.28];
            else if (n2 < 0.52) col = [0.32, 0.24, 0.16];
            else col = [0.56, 0.52, 0.36];
            const grain = (tileNoise(u, v, 32, 32, 1327) - 0.5) * 0.06;
            r = clamp01(col[0] + grain); g = clamp01(col[1] + grain); b = clamp01(col[2] + grain);
            h = clamp01(0.45 + n1 * 0.25 + grain);
        } else if (id === 'checker') {
            const n = 6;
            const parity = ((Math.floor(u * n) + Math.floor(v * n)) & 1) === 0;
            const k = parity ? 0.94 : 0.11;
            r = k; g = k; b = k;
            h = parity ? 0.65 : 0.35;
        } else if (id === 'gradient') {
            const t = clamp01(v * 0.85 + u * 0.15);
            r = mix(primary[0], secondary[0], t);
            g = mix(primary[1], secondary[1], t);
            b = mix(primary[2], secondary[2], t);
            h = 0.5;
        } else if (id === 'nebula') {
            const c1 = tileFbm(u, v, 4, 4, 5, 1409);
            const c2 = tileFbm(u + c1 * 0.3, v - c1 * 0.25, 6, 6, 4, 1423);
            const star = smoothstep(0.986, 0.999, tileNoise(u, v, 64, 64, 1433));
            r = clamp01(0.06 + c1 * 0.55 + c2 * 0.32 + star * 0.95);
            g = clamp01(0.04 + c2 * 0.34 + c1 * 0.14 + star * 0.95);
            b = clamp01(0.14 + c1 * 0.68 + c2 * 0.38 + star * 1.0);
            h = clamp01(0.45 + c2 * 0.25 + star * 0.3);
        }
        return [Math.round(clamp01(r) * 255), Math.round(clamp01(g) * 255), Math.round(clamp01(b) * 255), Math.round(clamp01(h) * 255)];
    }

    function generateTexture(id, size, opts) {
        const s = clamp(Math.round(size) || 128, 16, 512);
        const o = opts || {};
        const primary = parseHex(o.color, [0.95, 0.72, 0.24]);
        const secondary = parseHex(o.sideColor, [0.22, 0.45, 0.88]);
        const opaque = !!o.opaque;
        const data = new Uint8ClampedArray(s * s * 4);
        const inv = 1 / s;
        let idx = 0;
        for (let y = 0; y < s; y++) {
            const v = (y + 0.5) * inv;
            for (let x = 0; x < s; x++) {
                const u = (x + 0.5) * inv;
                const px = sampleProceduralTexel(id, u, v, primary, secondary);
                data[idx++] = px[0];
                data[idx++] = px[1];
                data[idx++] = px[2];
                data[idx++] = opaque ? 255 : px[3];
            }
        }
        return { width: s, height: s, data };
    }

    // --- exact 2D Euclidean Signed Distance Field (Felzenszwalb-Huttenlocher O(W*H) + anti-aliased boundary) ---

    function edt1d(f, n, d, v, z) {
        let k = 0;
        v[0] = 0;
        z[0] = -Infinity;
        z[1] = Infinity;
        for (let q = 1; q < n; q++) {
            let s = ((f[q] + q * q) - (f[v[k]] + v[k] * v[k])) / (2 * q - 2 * v[k]);
            while (k > 0 && s <= z[k]) {
                k--;
                s = ((f[q] + q * q) - (f[v[k]] + v[k] * v[k])) / (2 * q - 2 * v[k]);
            }
            k++;
            v[k] = q;
            z[k] = s;
            z[k + 1] = Infinity;
        }
        k = 0;
        for (let q = 0; q < n; q++) {
            while (z[k + 1] < q) k++;
            const dx = q - v[k];
            d[q] = dx * dx + f[v[k]];
        }
    }

    function edt2d(grid, w, h) {
        const maxDim = Math.max(w, h);
        const f = new Float64Array(maxDim);
        const d = new Float64Array(maxDim);
        const v = new Int32Array(maxDim);
        const z = new Float64Array(maxDim + 1);
        for (let x = 0; x < w; x++) {
            for (let y = 0; y < h; y++) f[y] = grid[y * w + x];
            edt1d(f, h, d, v, z);
            for (let y = 0; y < h; y++) grid[y * w + x] = d[y];
        }
        for (let y = 0; y < h; y++) {
            const row = y * w;
            for (let x = 0; x < w; x++) f[x] = grid[row + x];
            edt1d(f, w, d, v, z);
            for (let x = 0; x < w; x++) grid[row + x] = d[x];
        }
    }

    // Input: Uint8ClampedArray/Uint8Array alpha (length = width * height, 0..255).
    // Returns { sdf: Float32Array (signed distance in px, negative inside), rgba8: Uint8ClampedArray, width, height, spread }.
    function computeSDF(alpha, width, height, spread) {
        const w = Math.max(1, width | 0), h = Math.max(1, height | 0), n = w * h;
        const sp = Math.max(4, Number(spread) || 48);
        const outGrid = new Float64Array(n);
        const inGrid = new Float64Array(n);
        let anyInside = false;
        for (let i = 0; i < n; i++) {
            const a = (alpha[i] || 0) / 255;
            if (a >= 1) {
                anyInside = true;
                outGrid[i] = 0;
                inGrid[i] = INF;
            } else if (a <= 0) {
                outGrid[i] = INF;
                inGrid[i] = 0;
            } else {
                anyInside = true;
                const d = 0.5 - a;
                outGrid[i] = d > 0 ? d * d : 0;
                inGrid[i] = d < 0 ? d * d : 0;
            }
        }
        const sdf = new Float32Array(n);
        if (!anyInside) {
            sdf.fill(sp);
        } else {
            edt2d(outGrid, w, h);
            edt2d(inGrid, w, h);
            for (let i = 0; i < n; i++) {
                const a = (alpha[i] || 0) / 255;
                if (a > 0 && a < 1) {
                    const dOut = Math.sqrt(outGrid[i]);
                    const dIn = Math.sqrt(inGrid[i]);
                    const edge = 0.5 - a;
                    sdf[i] = edge >= 0 ? Math.min(dOut, edge + 0.25) : -Math.min(dIn, -edge + 0.25);
                } else {
                    sdf[i] = Math.sqrt(outGrid[i]) - Math.sqrt(inGrid[i]);
                }
            }
        }
        // Pack RGBA8 for WebGL1/universal fallback: R = encoded SDF in [-sp, sp], G/B = 2D gradient, A = raw alpha.
        const rgba8 = new Uint8ClampedArray(n * 4);
        for (let y = 0; y < h; y++) {
            const ym = Math.max(0, y - 1) * w, y0 = y * w, yp = Math.min(h - 1, y + 1) * w;
            for (let x = 0; x < w; x++) {
                const xm = Math.max(0, x - 1), xp = Math.min(w - 1, x + 1);
                const i = y0 + x;
                const gx = sdf[y0 + xp] - sdf[y0 + xm];
                const gy = sdf[yp + x] - sdf[ym + x];
                const gl = Math.hypot(gx, gy) || 1;
                const p4 = i * 4;
                rgba8[p4] = Math.round(clamp01(0.5 + sdf[i] / (2 * sp)) * 255);
                rgba8[p4 + 1] = Math.round(clamp01(0.5 + 0.5 * (gx / gl)) * 255);
                rgba8[p4 + 2] = Math.round(clamp01(0.5 + 0.5 * (gy / gl)) * 255);
                rgba8[p4 + 3] = alpha[i] || 0;
            }
        }
        return { sdf, rgba8, width: w, height: h, spread: sp };
    }

    // --- 3D math, rotation, deformations and perspective projection ---

    // Row-major 3x3 rotation matrix R = Rz(roll) * Rx(pitch) * Ry(yaw), mapping local text space -> world space.
    function rotationMatrix(pitchDeg, yawDeg, rollDeg) {
        const px = pitchDeg * DEG, py = yawDeg * DEG, rz = rollDeg * DEG;
        const cx = Math.cos(px), sx = Math.sin(px);
        const cy = Math.cos(py), sy = Math.sin(py);
        const cz = Math.cos(rz), sz = Math.sin(rz);
        // Rx * Ry (note: pitch>0 tilts front (0,0,1) down (-Y); yaw>0 turns front right (+X); roll>0 rotates CW on screen)
        const a00 = cy, a01 = 0, a02 = sy;
        const a10 = sx * sy, a11 = cx, a12 = -sx * cy;
        const a20 = -cx * sy, a21 = sx, a22 = cx * cy;
        // Rz * (Rx * Ry) where Rz rotates (0,1,0) -> (+sz, cz, 0)
        return [
            cz * a00 + sz * a10, cz * a01 + sz * a11, cz * a02 + sz * a12,
            -sz * a00 + cz * a10, -sz * a01 + cz * a11, -sz * a02 + cz * a12,
            a20, a21, a22
        ];
    }

    function transposeMat3(m) {
        return [m[0], m[3], m[6], m[1], m[4], m[7], m[2], m[5], m[8]];
    }

    function mulMat3Vec(m, x, y, z) {
        return [
            m[0] * x + m[1] * y + m[2] * z,
            m[3] * x + m[4] * y + m[5] * z,
            m[6] * x + m[7] * y + m[8] * z
        ];
    }

    function cameraDistance(perspective, aspect) {
        const t = clamp(perspective, 0, 100) / 100;
        const span = Math.max(1.6, aspect * 0.72 + 0.9);
        if (t <= 0.001) return 90 * span;
        return span * mix(16.0, 2.35, Math.pow(t, 0.72));
    }

    function halfDepthOf(depth) {
        return Math.max(0.015, (clamp(depth, 0, 100) / 100) * 0.58);
    }

    function bevelRadiusOf(bevel, halfDepth) {
        return Math.min(halfDepth * 0.92, (clamp(bevel, 0, 100) / 100) * 0.16);
    }

    // Conservative local 3D bounding half-extents for ray-AABB intersection & viewport culling.
    function deformedBounds(aspect, halfDepth, warpId, warpStrength, warpFreq) {
        const k = clamp(warpStrength, -100, 100) / 100;
        const ak = Math.abs(k);
        const pad = 0.24;
        let hx = aspect + pad, hy = 1.0 + pad, hz = halfDepth + pad;
        if (warpId === 'arc') {
            hx = aspect + 0.55;
            hy = 1.0 + ak * Math.max(1.1, aspect * 0.78) + 0.55;
        } else if (warpId === 'arch' || warpId === 'rise') {
            hy = 1.0 + ak * 0.95 + pad;
        } else if (warpId === 'wave') {
            hy = 1.0 + ak * 0.58 + pad;
        } else if (warpId === 'flag') {
            hy = 1.0 + ak * 0.48 + pad;
            hz = halfDepth + ak * 0.52 + pad;
        } else if (warpId === 'bulge' || warpId === 'wedge') {
            hy = 1.0 * (1 + ak * 0.75) + pad;
        } else if (warpId === 'cylinder') {
            if (ak > 0.01) {
                const R = Math.max(0.6, aspect / (ak * 2.15));
                const maxAng = Math.min(Math.PI * 0.92, (aspect + 0.2) / R);
                hx = Math.max(aspect, Math.sin(maxAng) * (R + halfDepth + 0.35)) + pad;
                hz = halfDepth + R * (1 - Math.cos(maxAng)) + 0.38;
            }
        } else if (warpId === 'twist') {
            const rad = Math.hypot(1.0 + pad, halfDepth + pad);
            hy = Math.max(hy, rad);
            hz = Math.max(hz, rad);
        } else if (warpId === 'bridge') {
            hy = 1.0 + ak * 0.5 + pad;
        } else if (warpId === 'dome') {
            hz = halfDepth + ak * 0.68 + pad;
        }
        return { hx, hy, hz, freq: clamp(Number(warpFreq) || 2, 0.5, 5) };
    }

    // Inverse warp: deformed 3D local space (px, py, pz) -> canonical flat text SDF space [qx, qy, qz].
    // Matches the GLSL shader verbatim so CPU fallback and WebGL produce identical geometry.
    function inverseWarp(px, py, pz, aspect, warpCode, warpStrength, warpFreq) {
        const k = clamp(warpStrength, -100, 100) / 100;
        const a = Math.max(0.25, aspect);
        if (warpCode === 0 || Math.abs(k) < 1e-4) return [px, py, pz];
        if (warpCode === 1) { // arc
            const sgn = k >= 0 ? 1 : -1;
            const R = Math.max(0.85, a / (Math.abs(k) * 1.45));
            const cy = py + sgn * R;
            const r = Math.hypot(px, cy);
            const ang = Math.atan2(px, sgn * cy);
            return [ang * R, sgn * (r - R), pz];
        }
        if (warpCode === 2) { // arch
            const nx = clamp(px / a, -1.35, 1.35);
            return [px, py - k * 0.72 * (1 - nx * nx), pz];
        }
        if (warpCode === 3) { // wave
            const off = k * 0.42 * Math.sin((px / a) * warpFreq * Math.PI);
            return [px, py - off, pz];
        }
        if (warpCode === 4) { // 3D flag
            const phase = (px / a) * warpFreq * Math.PI;
            return [px, py - k * 0.34 * Math.sin(phase), pz - k * 0.38 * Math.cos(phase * 1.15)];
        }
        if (warpCode === 5) { // bulge / pinch
            const nx = clamp(px / a, -1.2, 1.2);
            const sc = Math.max(0.28, 1 + k * 0.62 * Math.cos(nx * Math.PI * 0.5));
            return [px, py / sc, pz];
        }
        if (warpCode === 6) { // 3D cylinder
            const sgn = k >= 0 ? 1 : -1;
            const R = Math.max(0.6, a / (Math.abs(k) * 2.15));
            const cz = pz + sgn * R;
            const r = Math.hypot(px, cz);
            const ang = Math.atan2(px, sgn * cz);
            return [ang * R, py, sgn * (r - R)];
        }
        if (warpCode === 7) { // 3D twist
            const th = k * 1.35 * clamp(px / a, -1.4, 1.4);
            const ct = Math.cos(th), st = Math.sin(th);
            return [px, py * ct + pz * st, -py * st + pz * ct];
        }
        if (warpCode === 8) { // bridge
            const nx = clamp(px / a, -1.25, 1.25);
            const arch = Math.abs(k) * 0.62 * Math.max(0, 1 - nx * nx);
            if (k > 0) {
                const lo = -1 + arch, hi = 1;
                return [px, ((py - lo) / Math.max(0.25, hi - lo)) * 2 - 1, pz];
            }
            const lo = -1, hi = 1 - arch;
            return [px, ((py - lo) / Math.max(0.25, hi - lo)) * 2 - 1, pz];
        }
        if (warpCode === 9) { // wedge / taper
            const nx = clamp(px / a, -1.25, 1.25);
            const sc = Math.max(0.28, 1 + k * 0.52 * nx);
            return [px, py / sc, pz];
        }
        if (warpCode === 10) { // rise / stairs
            return [px, py - k * 0.58 * (px / a), pz];
        }
        if (warpCode === 11) { // 3D dome / sphere
            const nx = clamp(px / a, -1.25, 1.25), ny = clamp(py, -1.25, 1.25);
            const r2 = nx * nx + ny * ny;
            return [px, py, pz - k * 0.55 * Math.max(0, 1 - r2)];
        }
        return [px, py, pz];
    }

    // Projects the 8 corners of the 3D text bounding box into document pixels for rendering crops & the interactive 3D gizmo.
    function projectBox(params, aspect, docW, docH, lineCount) {
        const p = normalizeParams(params);
        const a = Math.max(0.35, Number(aspect) || 2.5);
        const lines = Math.max(1, Number(lineCount) || 1);
        const halfDepth = halfDepthOf(p.depth);
        const bounds = deformedBounds(a, halfDepth, p.warp, p.warpStrength, p.warpFreq);
        const R = rotationMatrix(p.pitch, p.yaw, p.roll);
        const camDist = cameraDistance(p.perspective, a);
        const cx = p.posX * docW, cy = p.posY * docH;
        // Text height spans local [-1, 1] -> baseScale = total text block pixel height / 2.
        const totalTextH = p.fontSize * (1 + (lines - 1) * p.lineHeight);
        const baseScale = Math.max(8, totalTextH * 0.56);

        const project = (lx, ly, lz) => {
            const w = mulMat3Vec(R, lx, ly, lz);
            const denom = Math.max(0.35, camDist - w[2]);
            const s = (camDist / denom) * baseScale;
            return { x: cx + w[0] * s, y: cy - w[1] * s, z: w[2] };
        };

        const front = [
            project(-bounds.hx, bounds.hy, bounds.hz),
            project(bounds.hx, bounds.hy, bounds.hz),
            project(bounds.hx, -bounds.hy, bounds.hz),
            project(-bounds.hx, -bounds.hy, bounds.hz)
        ];
        const back = [
            project(-bounds.hx, bounds.hy, -bounds.hz),
            project(bounds.hx, bounds.hy, -bounds.hz),
            project(bounds.hx, -bounds.hy, -bounds.hz),
            project(-bounds.hx, -bounds.hy, -bounds.hz)
        ];
        const corners = front.concat(back);
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        for (let i = 0; i < corners.length; i++) {
            if (corners[i].x < minX) minX = corners[i].x;
            if (corners[i].y < minY) minY = corners[i].y;
            if (corners[i].x > maxX) maxX = corners[i].x;
            if (corners[i].y > maxY) maxY = corners[i].y;
        }
        const shadowPad = p.shadow ? Math.ceil(baseScale * (0.45 + (p.shadowBlur / 100) * 0.35)) : 12;
        const pad = Math.max(16, shadowPad);
        const x0 = Math.max(0, Math.floor(minX - pad));
        const y0 = Math.max(0, Math.floor(minY - pad));
        const x1 = Math.min(docW, Math.ceil(maxX + pad));
        const y1 = Math.min(docH, Math.ceil(maxY + pad));
        const boxW = Math.max(1, maxX - minX), boxH = Math.max(1, maxY - minY);
        const orbitR = Math.max(36, Math.hypot(boxW, boxH) * 0.56);
        const topMid = project(0, bounds.hy, 0);
        const rollDirX = topMid.x - cx, rollDirY = topMid.y - cy;
        const rollLen = Math.hypot(rollDirX, rollDirY) || 1;
        const rollHandle = { x: topMid.x + (rollDirX / rollLen) * 26, y: topMid.y + (rollDirY / rollLen) * 26 };
        return {
            R, Rt: transposeMat3(R), camDist, baseScale, cx, cy, aspect: a, halfDepth, bounds,
            front, back, corners, minX, minY, maxX, maxY,
            rect: { x: x0, y: y0, w: Math.max(0, x1 - x0), h: Math.max(0, y1 - y0) },
            orbitR, rollHandle
        };
    }

    // --- CPU 3D SDF Raymarching Engine (reference + automatic fallback when WebGL is unavailable) ---

    function sampleSdf2D(sdfObj, u, v, aspect, texHalfX, texHalfY) {
        const dx = Math.max(0, Math.abs(u) - texHalfX);
        const dy = Math.max(0, Math.abs(v) - texHalfY);
        const outBox = Math.hypot(dx, dy);
        const tu = clamp01(0.5 + u / (2 * texHalfX));
        const tv = clamp01(0.5 - v / (2 * texHalfY));
        const fx = tu * (sdfObj.width - 1), fy = tv * (sdfObj.height - 1);
        const x0 = Math.floor(fx), y0 = Math.floor(fy);
        const x1 = Math.min(sdfObj.width - 1, x0 + 1), y1 = Math.min(sdfObj.height - 1, y0 + 1);
        const tx = fx - x0, ty = fy - y0;
        const s = sdfObj.sdf, w = sdfObj.width;
        const d00 = s[y0 * w + x0], d10 = s[y0 * w + x1];
        const d01 = s[y1 * w + x0], d11 = s[y1 * w + x1];
        const pxDist = mix(mix(d00, d10, tx), mix(d01, d11, tx), ty);
        const localPerPx = (2 * texHalfY) / sdfObj.height;
        return pxDist * localPerPx + outBox;
    }

    function evalExtrudedSDF(d2, qz, halfDepth, bevelR, bevelCode) {
        let dZ = Math.abs(qz) - halfDepth;
        if (bevelCode === 3) { // inflate
            const bulge = (Math.sqrt(Math.max(0, -d2) + 0.006) - 0.07746) * (bevelR * 2.1);
            dZ -= Math.max(0, bulge);
            const r = bevelR * 0.45;
            const ax = d2 + r, ay = dZ + r;
            return Math.hypot(Math.max(ax, 0), Math.max(ay, 0)) + Math.min(Math.max(ax, ay), 0) - r;
        }
        if (bevelCode === 2 || bevelR <= 1e-4) { // sharp
            const r = 0.004;
            const ax = d2 + r, ay = dZ + r;
            return Math.hypot(Math.max(ax, 0), Math.max(ay, 0)) + Math.min(Math.max(ax, ay), 0) - r;
        }
        if (bevelCode === 1) { // chamfer
            const c = (d2 + dZ + bevelR) * 0.70710678;
            return Math.max(Math.max(d2, dZ), c);
        }
        // round
        const ax = d2 + bevelR, ay = dZ + bevelR;
        return Math.hypot(Math.max(ax, 0), Math.max(ay, 0)) + Math.min(Math.max(ax, ay), 0) - bevelR;
    }

    function eval3DSDF(px, py, pz, sdfObj, ctx) {
        const q = inverseWarp(px, py, pz, ctx.aspect, ctx.warpCode, ctx.warpStrength, ctx.warpFreq);
        const d2 = sampleSdf2D(sdfObj, q[0], q[1], ctx.aspect, ctx.texHalfX, ctx.texHalfY);
        return evalExtrudedSDF(d2, q[2], ctx.halfDepth, ctx.bevelR, ctx.bevelCode);
    }

    function intersectAABB(ro, rd, hx, hy, hz) {
        let tmin = -INF, tmax = INF;
        const h = [hx, hy, hz];
        for (let i = 0; i < 3; i++) {
            const inv = Math.abs(rd[i]) > 1e-7 ? 1 / rd[i] : 1e7;
            let t1 = (-h[i] - ro[i]) * inv, t2 = (h[i] - ro[i]) * inv;
            if (t1 > t2) { const tmp = t1; t1 = t2; t2 = tmp; }
            if (t1 > tmin) tmin = t1;
            if (t2 < tmax) tmax = t2;
            if (tmin > tmax) return null;
        }
        if (tmax < 0) return null;
        return [Math.max(0.001, tmin), tmax];
    }

    function sampleEnv(rx, ry, rz) {
        const up = clamp01(ry * 0.5 + 0.5);
        const hor = Math.exp(-Math.abs(ry) * 5.0);
        const softbox = smoothstep(0.68, 0.96, rx * 0.45 + ry * 0.65 + rz * 0.62);
        const softbox2 = smoothstep(0.74, 0.98, -rx * 0.7 + ry * 0.35 + rz * 0.58);
        return [
            mix(0.12, 0.72, up) + hor * 0.38 + softbox * 0.9 + softbox2 * 0.35,
            mix(0.14, 0.79, up) + hor * 0.42 + softbox * 0.88 + softbox2 * 0.45,
            mix(0.19, 0.92, up) + hor * 0.5 + softbox * 0.82 + softbox2 * 0.62
        ];
    }

    function sampleTexBilinear(texData, texSize, u, v) {
        const fx = fract(u) * texSize - 0.5, fy = fract(v) * texSize - 0.5;
        const x0 = wrapInt(Math.floor(fx), texSize), y0 = wrapInt(Math.floor(fy), texSize);
        const x1 = (x0 + 1) % texSize, y1 = (y0 + 1) % texSize;
        const tx = fx - Math.floor(fx), ty = fy - Math.floor(fy);
        const i00 = (y0 * texSize + x0) * 4, i10 = (y0 * texSize + x1) * 4;
        const i01 = (y1 * texSize + x0) * 4, i11 = (y1 * texSize + x1) * 4;
        const out = [0, 0, 0, 0];
        for (let c = 0; c < 4; c++) {
            out[c] = mix(mix(texData[i00 + c], texData[i10 + c], tx), mix(texData[i01 + c], texData[i11 + c], tx), ty) / 255;
        }
        return out;
    }

    function renderCPU(sdfObj, texObj, params, outW, outH, options) {
        const p = normalizeParams(params);
        const o = options || {};
        const docW = o.docW || outW, docH = o.docH || outH;
        const viewX = o.viewX || 0, viewY = o.viewY || 0;
        const scaleX = (o.viewW || outW) / Math.max(1, outW);
        const scaleY = (o.viewH || outH) / Math.max(1, outH);
        const aspect = o.aspect || (sdfObj.width / Math.max(1, sdfObj.height));
        const padFactor = o.padFactor || 1.25;
        const texHalfX = o.texHalfX || (aspect * padFactor);
        const texHalfY = o.texHalfY || padFactor;
        const proj = projectBox(p, aspect, docW, docH, o.lineCount || 1);
        const warpDef = warpById(p.warp);
        const texDef = textureById(p.texture);
        const bevelDef = bevelById(p.bevelStyle);
        const bevelR = bevelRadiusOf(p.bevel, proj.halfDepth);
        const ctx = {
            aspect, halfDepth: proj.halfDepth, bevelR, bevelCode: bevelDef.code,
            warpCode: warpDef.code, warpStrength: p.warpStrength, warpFreq: p.warpFreq,
            texHalfX, texHalfY
        };
        const az = p.lightAzimuth * DEG, el = p.lightElevation * DEG;
        const lw = [Math.cos(el) * Math.sin(az), Math.sin(el), Math.cos(el) * Math.cos(az)];
        const lLen = Math.hypot(lw[0], lw[1], lw[2]) || 1;
        lw[0] /= lLen; lw[1] /= lLen; lw[2] /= lLen;
        const sideRgb = parseHex(p.sideColor, [0.22, 0.28, 0.42]);
        const primRgb = parseHex(p.color, [0.95, 0.72, 0.24]);
        const texScale = (p.texScale || 100) / 100;
        const tCos = Math.cos(p.texRotate * DEG), tSin = Math.sin(p.texRotate * DEG);
        const metal = clamp01((p.metalness / 100) * 0.7 + texDef.metalness * 0.3);
        const gloss = clamp01((p.gloss / 100) * 0.7 + texDef.gloss * 0.3);
        const bumpAmp = (p.bumpStrength / 100) * texDef.bump;
        const steps = o.draft ? 40 : 68;
        const out = new Uint8ClampedArray(outW * outH * 4);
        const ro = mulMat3Vec(proj.Rt, 0, 0, proj.camDist);
        const b = proj.bounds;
        const backPlaneZ = -Math.max(b.hx, b.hy, b.hz) - 0.15;

        for (let py = 0; py < outH; py++) {
            if (o.progress && (py & 15) === 0) o.progress(py / outH);
            const docY = viewY + (py + 0.5) * scaleY;
            const sy = (proj.cy - docY) / proj.baseScale;
            for (let px = 0; px < outW; px++) {
                const docX = viewX + (px + 0.5) * scaleX;
                const sx = (docX - proj.cx) / proj.baseScale;
                const rl = Math.hypot(sx, sy, -proj.camDist);
                const rdw = [sx / rl, sy / rl, -proj.camDist / rl];
                const rd = mulMat3Vec(proj.Rt, rdw[0], rdw[1], rdw[2]);
                const hitBox = intersectAABB(ro, rd, b.hx, b.hy, b.hz);
                let hit = false, tHit = 0;
                if (hitBox) {
                    let t = hitBox[0], prevT = t, prevD = 1;
                    for (let s = 0; s < steps && t <= hitBox[1]; s++) {
                        const d = eval3DSDF(ro[0] + rd[0] * t, ro[1] + rd[1] * t, ro[2] + rd[2] * t, sdfObj, ctx);
                        if (d < 0.0025) {
                            let lo = prevT, hi = t;
                            if (prevD > 0) {
                                for (let k = 0; k < 4; k++) {
                                    const mid = 0.5 * (lo + hi);
                                    const dm = eval3DSDF(ro[0] + rd[0] * mid, ro[1] + rd[1] * mid, ro[2] + rd[2] * mid, sdfObj, ctx);
                                    if (dm < 0) hi = mid; else lo = mid;
                                }
                            }
                            tHit = hi;
                            hit = true;
                            break;
                        }
                        prevT = t; prevD = d;
                        t += Math.max(0.004, d * warpDef.lip);
                    }
                }
                const idx = (py * outW + px) * 4;
                if (hit) {
                    const hx = ro[0] + rd[0] * tHit, hy = ro[1] + rd[1] * tHit, hz = ro[2] + rd[2] * tHit;
                    const e = 0.006;
                    const d1 = eval3DSDF(hx + e, hy - e, hz - e, sdfObj, ctx);
                    const d2 = eval3DSDF(hx - e, hy - e, hz + e, sdfObj, ctx);
                    const d3 = eval3DSDF(hx - e, hy + e, hz - e, sdfObj, ctx);
                    const d4 = eval3DSDF(hx + e, hy + e, hz + e, sdfObj, ctx);
                    let nx = d1 - d2 - d3 + d4, ny = -d1 - d2 + d3 + d4, nz = -d1 + d2 - d3 + d4;
                    const nl = Math.hypot(nx, ny, nz) || 1;
                    nx /= nl; ny /= nl; nz /= nl;
                    const q = inverseWarp(hx, hy, hz, aspect, warpDef.code, p.warpStrength, p.warpFreq);
                    const isFront = Math.abs(q[2]) >= proj.halfDepth - Math.max(bevelR * 1.15, 0.014);
                    const uvx = isFront ? q[0] * 0.45 : (q[0] + q[1]) * 0.45;
                    const uvy = isFront ? q[1] * 0.45 : q[2] * 0.9;
                    const tu = (uvx * tCos - uvy * tSin) * texScale + 0.5;
                    const tv = (uvx * tSin + uvy * tCos) * texScale + 0.5;
                    const tex = sampleTexBilinear(texObj.data, texObj.width, tu, tv);
                    let ar = tex[0], ag = tex[1], ab = tex[2];
                    if (bumpAmp > 0.01 && (isFront || p.sideMatchTexture)) {
                        const tx2 = sampleTexBilinear(texObj.data, texObj.width, tu + 0.012, tv)[3];
                        const ty2 = sampleTexBilinear(texObj.data, texObj.width, tu, tv + 0.012)[3];
                        nx += (tex[3] - tx2) * bumpAmp * 1.2;
                        ny += (tex[3] - ty2) * bumpAmp * 1.2;
                        const nl2 = Math.hypot(nx, ny, nz) || 1;
                        nx /= nl2; ny /= nl2; nz /= nl2;
                    }
                    if (!isFront && !p.sideMatchTexture) {
                        ar = sideRgb[0]; ag = sideRgb[1]; ab = sideRgb[2];
                    } else if (texDef.tint && p.texture === 'solid') {
                        ar = primRgb[0]; ag = primRgb[1]; ab = primRgb[2];
                    }
                    const nw = mulMat3Vec(proj.R, nx, ny, nz);
                    const vx = -rdw[0], vy = -rdw[1], vz = -rdw[2];
                    if (texDef.holo) {
                        const ndv = clamp01(nw[0] * vx + nw[1] * vy + nw[2] * vz);
                        const ph = (1 - ndv) * TAU + (q[0] + q[1]) * 1.4;
                        ar = mix(ar, 0.55 + 0.45 * Math.sin(ph), 0.65);
                        ag = mix(ag, 0.55 + 0.45 * Math.sin(ph + 2.094), 0.65);
                        ab = mix(ab, 0.55 + 0.45 * Math.sin(ph + 4.188), 0.65);
                    }
                    const ndl = Math.max(0, nw[0] * lw[0] + nw[1] * lw[1] + nw[2] * lw[2]);
                    const fill = Math.max(0, nw[0] * -0.55 + nw[1] * 0.28 + nw[2] * 0.78);
                    const rim = Math.pow(1 - clamp01(nw[0] * vx + nw[1] * vy + nw[2] * vz), 2.8);
                    const hx2 = lw[0] + vx, hy2 = lw[1] + vy, hz2 = lw[2] + vz;
                    const hl = Math.hypot(hx2, hy2, hz2) || 1;
                    const ndh = Math.max(0, (nw[0] * hx2 + nw[1] * hy2 + nw[2] * hz2) / hl);
                    const specPow = mix(10, 128, gloss);
                    const spec = Math.pow(ndh, specPow) * mix(0.25, 1.35, gloss);
                    const ndv2 = nw[0] * vx + nw[1] * vy + nw[2] * vz;
                    const env = sampleEnv(2 * ndv2 * nw[0] - vx, 2 * ndv2 * nw[1] - vy, 2 * ndv2 * nw[2] - vz);
                    const aoD = eval3DSDF(hx + nx * 0.08, hy + ny * 0.08, hz + nz * 0.08, sdfObj, ctx);
                    const ao = clamp(0.45 + 0.55 * clamp01(aoD / 0.08), 0.45, 1.0);
                    const diffK = (0.24 + ndl * 0.78 + fill * 0.24) * ao;
                    const refK = clamp01(texDef.reflect * (0.45 + metal * 0.65) * (0.35 + gloss * 0.65));
                    const em = texDef.emissive;
                    const fr = ar * (diffK * (1 - metal * 0.55) + em) + mix(spec, spec * ar, metal) + env[0] * mix(refK * 0.35, ar * refK, metal) + rim * 0.22;
                    const fg = ag * (diffK * (1 - metal * 0.55) + em) + mix(spec, spec * ag, metal) + env[1] * mix(refK * 0.35, ag * refK, metal) + rim * 0.25;
                    const fb = ab * (diffK * (1 - metal * 0.55) + em) + mix(spec, spec * ab, metal) + env[2] * mix(refK * 0.35, ab * refK, metal) + rim * 0.32;
                    out[idx] = Math.round(clamp01(fr / (1 + fr * 0.22)) * 255);
                    out[idx + 1] = Math.round(clamp01(fg / (1 + fg * 0.22)) * 255);
                    out[idx + 2] = Math.round(clamp01(fb / (1 + fb * 0.22)) * 255);
                    out[idx + 3] = 255;
                } else if (p.shadow && p.shadowOpacity > 0 && rdw[2] < -0.05) {
                    const tPlane = (backPlaneZ - proj.camDist) / rdw[2];
                    const wx = rdw[0] * tPlane, wy = rdw[1] * tPlane;
                    const shift = (proj.halfDepth + 0.35) / Math.max(0.25, lw[2]);
                    const pLoc = mulMat3Vec(proj.Rt, wx + lw[0] * shift * 0.45, wy + lw[1] * shift * 0.45, 0);
                    const dS = eval3DSDF(pLoc[0], pLoc[1], pLoc[2], sdfObj, ctx);
                    const blurR = 0.06 + (p.shadowBlur / 100) * 0.38;
                    const sh = clamp01(1 - smoothstep(-0.02, blurR, dS)) * (p.shadowOpacity / 100);
                    if (sh > 0.005) {
                        out[idx] = 8; out[idx + 1] = 10; out[idx + 2] = 18;
                        out[idx + 3] = Math.round(sh * 255);
                    }
                }
            }
        }
        return { width: outW, height: outH, data: out };
    }

    Lib.registerOp('text3d.render', function text3dRenderOp(args, ctx) {
        const a = args || {};
        const sdfObj = a.sdf;
        const texObj = a.texture || generateTexture((a.params && a.params.texture) || 'gold', 128, a.params);
        const img = renderCPU(sdfObj, texObj, a.params, a.width, a.height, {
            docW: a.docW, docH: a.docH, viewX: a.viewX, viewY: a.viewY, viewW: a.viewW, viewH: a.viewH,
            aspect: a.aspect, padFactor: a.padFactor, lineCount: a.lineCount, draft: !!a.draft,
            progress: ctx && ctx.progress
        });
        return { image: img, width: img.width, height: img.height, data: img.data };
    });

    Lib.text3d = {
        FONTS, TEXTURES, WARPS, BEVELS, ANGLE_PRESETS, STYLE_PRESETS, DEFAULTS,
        fontById, textureById, warpById, bevelById, defaultParams, normalizeParams,
        generateTexture, computeSDF, rotationMatrix, transposeMat3, mulMat3Vec,
        cameraDistance, halfDepthOf, bevelRadiusOf, deformedBounds, inverseWarp,
        projectBox, eval3DSDF, renderCPU
    };
})(typeof self !== 'undefined' ? self : globalThis);
