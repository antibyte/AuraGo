/* Aura Drift - 3D effects: soft lit particles, skid ribbon, rings, beams and the event to effect mapping.
 *
 * Space: three space is [x, up, y_sim]. Sim events carry (x, y); they become (x, height, y) here.
 * Everything runs from two instanced billboard pools (alpha and additive) fed by one ring buffer of
 * `q.particles` particles. Particles are plain typed arrays: no allocation per particle or frame.
 * Math.random is allowed here (presentation only). Hits on animals and props never produce gore. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp, rnd = Math.random, TAU = Math.PI * 2;
    const rr = (a, b) => a + (b - a) * rnd();
    /* Stochastic rounding: turns a rate times dt into a whole number of particles. */
    const N = x => { const f = Math.floor(x); return f + (rnd() < x - f ? 1 : 0); };
    /* Atlas cells (4x4). */
    const F = { PUFF: 0, SPARK: 1, FLAME: 2, FEATHER: 3, CONFETTI: 4, RING: 5, SHARD: 6, DROP: 7, STAR: 8, BOLT: 9, HEX: 10, LEAF: 11, PUFF2: 12, GLOW: 13, STREAK: 14, DOT: 15 };
    const K_BOUNCE = 1, K_FLUTTER = 2, K_DIE = 3;
    /* Car geometry for wheel and exhaust anchors: [rear axle back, tail back, half track]. */
    const GEO = { vektor: [1.35, 2.15, 0.85], kobalt: [1.35, 2.15, 0.85], lumen: [1.4, 2.25, 0.9], tundra: [1.35, 2.1, 0.9], softeis: [1.7, 2.45, 1.0], maehkoenig: [0.8, 1.15, 0.6], couch: [1.0, 1.45, 0.7], bertha: [1.8, 2.55, 1.35] };
    const GEO_DEFAULT = GEO.vektor;

    function buildAtlas(T) {
        const S = 512, C = 128;
        const cv = document.createElement('canvas');
        cv.width = cv.height = S;
        const g = cv.getContext('2d');
        const cell = (i, fn) => {
            g.save();
            g.translate((i % 4) * C + C / 2, Math.floor(i / 4) * C + C / 2);
            g.beginPath(); g.rect(-C / 2, -C / 2, C, C); g.clip();
            fn();
            g.restore();
        };
        const blob = (x, y, r, a, shade) => {
            const c = shade | 0, gr = g.createRadialGradient(x, y, 0, x, y, r);
            gr.addColorStop(0, 'rgba(' + c + ',' + c + ',' + c + ',' + a + ')');
            gr.addColorStop(1, 'rgba(' + c + ',' + c + ',' + c + ',0)');
            g.fillStyle = gr; g.fillRect(x - r, y - r, r * 2, r * 2);
        };
        /* Radial mask: the soft-particle alpha falloff that keeps every sprite edge invisible. */
        const mask = (r0, r1) => {
            g.globalCompositeOperation = 'destination-in';
            const gr = g.createRadialGradient(0, 0, r0, 0, 0, r1);
            gr.addColorStop(0, 'rgba(0,0,0,1)'); gr.addColorStop(1, 'rgba(0,0,0,0)');
            g.fillStyle = gr; g.fillRect(-C / 2, -C / 2, C, C);
            g.globalCompositeOperation = 'source-over';
        };
        const poly = (pts, open) => { g.beginPath(); pts.forEach((p, i) => (i ? g.lineTo(p[0], p[1]) : g.moveTo(p[0], p[1]))); if (!open) g.closePath(); };
        const puff = (n, spread, rmin, rmax, a) => {
            for (let k = 0; k < n; k++) { const an = rnd() * TAU, d = Math.sqrt(rnd()) * spread; blob(Math.cos(an) * d, Math.sin(an) * d * 0.9, rmin + rnd() * (rmax - rmin), a, 95 + rnd() * 160); }
        };
        cell(F.PUFF, () => { puff(22, 26, 18, 34, 0.5); mask(22, 60); });
        cell(F.PUFF2, () => { puff(26, 30, 14, 28, 0.45); g.save(); g.scale(1.25, 0.8); puff(8, 18, 16, 26, 0.4); g.restore(); mask(20, 61); });
        cell(F.SPARK, () => { g.save(); g.scale(1, 0.2); blob(0, 0, 60, 1, 255); blob(0, 0, 30, 1, 255); g.restore(); blob(0, 0, 11, 1, 255); });
        cell(F.FLAME, () => { blob(0, 0, 56, 0.5, 255); blob(0, 0, 34, 0.75, 255); blob(0, 0, 16, 1, 255); puff(7, 20, 12, 22, 0.55); mask(26, 60); });
        cell(F.FEATHER, () => {
            g.fillStyle = '#fff'; g.beginPath(); g.moveTo(-52, 0); g.quadraticCurveTo(-8, -28, 52, -4); g.quadraticCurveTo(12, 24, -52, 0); g.fill();
            g.strokeStyle = 'rgb(150,150,150)'; g.lineWidth = 3; g.beginPath(); g.moveTo(-54, 1); g.lineTo(50, -3); g.stroke();
            g.lineWidth = 2; g.strokeStyle = 'rgb(205,205,205)';
            for (let k = -3; k <= 4; k++) { g.beginPath(); g.moveTo(k * 11, -1); g.lineTo(k * 11 + 14, -13 + Math.abs(k)); g.moveTo(k * 11, 0); g.lineTo(k * 11 + 14, 11 - Math.abs(k)); g.stroke(); }
        });
        cell(F.CONFETTI, () => { g.fillStyle = '#fff'; g.fillRect(-26, -15, 52, 15); g.fillStyle = 'rgb(205,205,205)'; g.fillRect(-26, 0, 52, 15); });
        const radial = (r, stops) => {
            const gr = g.createRadialGradient(0, 0, 0, 0, 0, r);
            for (const st of stops) gr.addColorStop(st[0], 'rgba(255,255,255,' + st[1] + ')');
            g.fillStyle = gr; g.fillRect(-64, -64, 128, 128);
        };
        cell(F.RING, () => radial(62, [[0, 0.03], [0.66, 0.08], [0.8, 1], [0.9, 0.45], [1, 0]]));
        cell(F.SHARD, () => {
            const lg = g.createLinearGradient(-40, -40, 40, 40);
            lg.addColorStop(0, '#fff'); lg.addColorStop(1, 'rgb(120,120,120)'); g.fillStyle = lg; poly([[-42, 8], [-10, -40], [36, -20], [46, 16], [6, 42]]); g.fill();
            g.fillStyle = 'rgba(255,255,255,0.8)'; poly([[-10, -40], [36, -20], [4, 0]]); g.fill();
        });
        cell(F.DROP, () => { radial(44, [[0, 1], [0.7, 0.9], [1, 0]]); blob(-12, -12, 14, 1, 255); });
        cell(F.STAR, () => {
            blob(0, 0, 62, 0.5, 255);
            const pts = [];
            for (let k = 0; k < 10; k++) { const r = k % 2 ? 22 : 52, a = -Math.PI / 2 + k * Math.PI / 5; pts.push([Math.cos(a) * r, Math.sin(a) * r]); }
            g.fillStyle = '#fff'; poly(pts); g.fill();
        });
        cell(F.BOLT, () => {
            const pts = [[-56, -6], [-30, 10], [-14, -12], [6, 12], [24, -10], [56, 8]];
            g.lineJoin = 'round';
            g.strokeStyle = 'rgba(255,255,255,0.35)'; g.lineWidth = 16; poly(pts, true); g.stroke();
            g.strokeStyle = '#fff'; g.lineWidth = 6; poly(pts, true); g.stroke();
        });
        cell(F.HEX, () => {
            const pts = []; for (let k = 0; k < 6; k++) pts.push([Math.cos(k * Math.PI / 3) * 52, Math.sin(k * Math.PI / 3) * 52]);
            g.fillStyle = 'rgba(255,255,255,0.16)'; poly(pts); g.fill();
            g.strokeStyle = '#fff'; g.lineWidth = 8; poly(pts); g.stroke();
        });
        cell(F.LEAF, () => {
            g.fillStyle = '#fff'; g.beginPath(); g.moveTo(-50, 0); g.quadraticCurveTo(0, -34, 50, 0); g.quadraticCurveTo(0, 34, -50, 0); g.fill();
            g.strokeStyle = 'rgb(160,160,160)'; g.lineWidth = 3; poly([[-50, 0], [46, 0]], true); g.stroke();
        });
        cell(F.GLOW, () => radial(62, [[0, 1], [0.2, 0.6], [0.5, 0.16], [1, 0]]));
        cell(F.STREAK, () => { g.save(); g.scale(1, 0.09); blob(0, 0, 62, 1, 255); blob(0, 0, 40, 1, 255); g.restore(); });
        cell(F.DOT, () => radial(42, [[0, 1], [0.8, 1], [1, 0]]));
        const tex = new T.CanvasTexture(cv);
        tex.generateMipmaps = false;
        tex.minFilter = T.LinearFilter; tex.magFilter = T.LinearFilter;
        tex.needsUpdate = true;
        return tex;
    }

    const PARTICLE_VS = [
        'attribute vec4 aPos; attribute vec4 aColor; attribute vec4 aMisc; attribute vec3 aVel;',
        'uniform vec3 uSunDir;',
        'varying vec2 vUv; varying vec2 vQ; varying vec4 vCol; varying float vLit; varying vec3 vL; varying float vDepth; varying float vSz;',
        'void main() {',
        '  vec4 mv = viewMatrix * vec4(aPos.xyz, 1.0);',
        '  float hs = aPos.w * 0.5; vec2 q = position.xy; vec2 off;',
        '  float c = cos(aMisc.x), s = sin(aMisc.x);',
        '  if (aMisc.z > 0.0) {',
        '    vec3 vv = (viewMatrix * vec4(aVel, 0.0)).xyz; float l = length(vv.xy);',
        '    vec2 d = l > 0.001 ? vv.xy / l : vec2(1.0, 0.0);',
        '    off = (d * q.x * (1.0 + aMisc.z * min(l * 0.09, 5.0)) + vec2(-d.y, d.x) * q.y) * hs;',
        '  } else off = vec2(c * q.x - s * q.y, s * q.x + c * q.y) * hs;',
        '  mv.xy += off;',
        '  vec3 L = normalize((viewMatrix * vec4(uSunDir, 0.0)).xyz);',
        '  vL = vec3(c * L.x + s * L.y, -s * L.x + c * L.y, L.z);',
        '  vQ = q; vCol = aColor; vLit = aMisc.w; vDepth = -mv.z; vSz = aPos.w;',
        '  vUv = (vec2(mod(aMisc.y, 4.0), 3.0 - floor(aMisc.y / 4.0)) + q * 0.5 + 0.5) / 4.0; gl_Position = projectionMatrix * mv;',
        '}'
    ].join('\n');
    /* Lit puffs get a fake sphere normal against the sun: the lit side takes the sun colour, the far side the sky. */
    const PARTICLE_FS = [
        'uniform sampler2D uMap; uniform vec3 uSunCol; uniform vec3 uAmb; uniform vec3 uFog; uniform vec2 uFogR; uniform float uAdd;',
        'varying vec2 vUv; varying vec2 vQ; varying vec4 vCol; varying float vLit; varying vec3 vL; varying float vDepth; varying float vSz;',
        'void main() {',
        '  vec4 tx = texture2D(uMap, vUv);',
        '  float a = tx.a * vCol.a * smoothstep(vSz * 0.35, vSz * 1.3 + 1.0, vDepth);',
        '  if (a < 0.003) discard; vec3 col = vCol.rgb;',
        '  if (vLit > 0.5) {',
        '    vec3 n = normalize(vec3(vQ, sqrt(max(0.0, 1.0 - dot(vQ, vQ))) * 0.9 + 0.1));',
        '    float d = clamp(dot(n, normalize(vL)) * 0.5 + 0.5, 0.0, 1.0);',
        '    col *= (uAmb + uSunCol * d * d) * (0.62 + 0.45 * tx.r);',
        '  } else if (uAdd < 0.5) col *= 0.7 + 0.3 * tx.r;',
        '  float f = smoothstep(uFogR.x, uFogR.y, vDepth);',
        '  if (uAdd > 0.5) a *= 1.0 - f; else col = mix(col, uFog, f);',
        '  gl_FragColor = vec4(col, a);',
        '#include <tonemapping_fragment>',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');
    const RING_VS = 'varying vec2 vP;\nvoid main() { vP = position.xz * 2.0; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }';
    const RING_FS = [
        'uniform vec3 uColor; uniform float uAlpha; uniform float uIn; varying vec2 vP;',
        'void main() {',
        '  float d = length(vP); float w = 0.17;',
        '  float a = (smoothstep(uIn - w, uIn, d) * (1.0 - smoothstep(uIn, uIn + w * 0.3, d)) + (1.0 - smoothstep(0.0, uIn, d)) * 0.1) * uAlpha;',
        '  if (a < 0.003) discard; gl_FragColor = vec4(uColor, a);',
        '#include <tonemapping_fragment>',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');
    const BEAM_VS = [
        'uniform vec3 uA; uniform vec3 uB; uniform float uW0; uniform float uW1; uniform float uFlat; varying vec2 vUv;',
        'void main() {',
        '  float t = position.x * 0.5 + 0.5; vec3 p = mix(uA, uB, t); vec3 ax = uB - uA;',
        '  vec3 side = uFlat < 0.5 ? cross(ax, cameraPosition - p) : (uFlat < 1.5 ? cross(ax, vec3(0.0, 1.0, 0.0)) : vec3(0.0, 1.0, 0.0));',
        '  side = normalize(side + vec3(0.000001, 0.0, 0.0));',
        '  p += side * position.y * mix(uW0, uW1, t); vUv = vec2(t, position.y);',
        '  gl_Position = projectionMatrix * viewMatrix * vec4(p, 1.0);',
        '}'
    ].join('\n');
    const BEAM_FS = [
        'uniform vec3 uColor; uniform float uAlpha; uniform float uKind; uniform float uTime; uniform float uLen; varying vec2 vUv;',
        'void main() {',
        '  float ay = abs(vUv.y); float t = vUv.x; float a; vec3 col = uColor;',
        '  if (uKind < 0.5) {',
        '    float core = pow(1.0 - ay, 3.0);',
        '    a = core + pow(1.0 - ay, 1.6) * 0.4; col = mix(uColor, vec3(2.0), core * 0.7);',
        '    a *= smoothstep(0.0, 0.04, t) * (1.0 - smoothstep(0.92, 1.0, t));',
        '  } else if (uKind < 1.5) {',
        '    float line = 1.0 - smoothstep(0.35, 1.0, ay);',
        '    a = line * (0.65 + 0.35 * sin(t * uLen * 4.0 - uTime * 18.0 + vUv.y * 2.0)); col = mix(uColor, vec3(1.6), line * 0.5);',
        '  } else if (uKind < 2.5) {',
        '    float edge = pow(1.0 - ay, 1.4);',
        '    a = edge * (0.65 + 0.35 * sin(t * uLen * 0.9 + uTime * 5.0) * sin(vUv.y * 6.0 + uTime * 2.0)) * (0.25 + 0.75 * t);',
        '    col = mix(uColor, vec3(1.0), edge * 0.5);',
        '  } else {',
        '    a = pow(1.0 - ay, 2.0) * pow(1.0 - t, 1.1) * (0.75 + 0.25 * sin(t * 22.0 - uTime * 46.0 + vUv.y * 4.0));',
        '    col = mix(vec3(2.2, 1.7, 0.8), vec3(1.6, 0.35, 0.05), t);',
        '  }',
        '  gl_FragColor = vec4(col, a * uAlpha);',
        '#include <tonemapping_fragment>',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');

    AD.createFx = function (T, scene, q) {
        let gl = q || AD.QUALITY.medium.gl;
        const root = new T.Group();
        root.name = 'fx';
        scene.add(root);
        const colour = new T.Color();
        const col = (hex, k) => { colour.set(hex); k = k || 1; return [colour.r * k, colour.g * k, colour.b * k]; };
        const atlas = buildAtlas(T);
        const U = {
            uMap: { value: atlas }, uSunDir: { value: new T.Vector3(0, 1, 0) }, uSunCol: { value: new T.Color(1, 1, 1) },
            uAmb: { value: new T.Color(0.4, 0.4, 0.4) }, uFog: { value: new T.Color(0.7, 0.7, 0.7) }, uFogR: { value: new T.Vector2(100, 900) }
        };

        /* ---- palettes ------------------------------------------------------ */
        const WHITE = col('#ffffff'), HOT = col('#fff3d0', 2), FIRE = col('#ff9a2a', 1.7), EMBER = col('#d8280c', 1.1), BLACK = col('#050505');
        const FB0 = col('#ff9a3a', 1.15), FB1 = col('#2a0904', 0.7), FB_HOT = col('#ffe2a0', 1.6), FB_DARK = col('#c04a14', 0.9);
        const SMOKE0 = col('#f4f6fa'), SMOKE1 = col('#a9b2c0'), SOOT0 = col('#3a3a3e'), SOOT1 = col('#17171a'), ROCK = col('#6c6258');
        const TURBO = [col('#4fc3ff', 2.4), col('#ff9a3c', 2.4), col('#ff5ec4', 2.4)];
        const TURBO_SOFT = [col('#4fc3ff', 1.1), col('#ff9a3c', 1.1), col('#ff5ec4', 1.1)];
        const NITRO_CORE = col('#9fdcff', 3), NITRO_OUT = col('#ffa23a', 2.6), NITRO_END = col('#ff4a10', 1.4), RAIL_AIM = col('#ff3040', 2), ROPE = col('#d8e2ee', 1.4), UFO0 = col('#8dffb0', 1.3), UFO1 = col('#b8ffd0', 1.6), OVER_OUT = col('#ff4a12', 2.6), OVER_CORE = col('#ffd27a', 3);
        const PARTY = ['#ff4d6d', '#ffd166', '#06d6a0', '#4cc9f0', '#b388ff', '#ff9f1c', '#ffffff'].map(h => col(h, 1.1));
        const GOLD = col('#ffd23c', 2.2), ICE0 = col('#d8f2ff', 1.4), ICE1 = col('#8cc8f0'), PURPLE = col('#b34cff', 2.4), PINK = col('#ff5ec4', 2.4);
        const TEAL = col('#5eead4', 2.2), ELEC = col('#9ad8ff', 3), STRAW0 = col('#e8c85c'), STRAW1 = col('#b58a2c'), WOOD = col('#a8743a');
        const SURF = {
            asphalt: { c0: SMOKE0, c1: SMOKE1, s0: 1.5, s1: 5.2, life: 1.5, a: 0.7, rate: 80, up: 1.0, sp: 2.2 },
            salt: { c0: col('#ffffff'), c1: col('#e4e4dc'), s0: 1.5, s1: 5, life: 1.5, a: 0.6, rate: 75, up: 1.0, sp: 2.2 },
            oil: { c0: col('#555a60'), c1: col('#2c2f33'), s0: 1.4, s1: 4.6, life: 1.4, a: 0.6, rate: 70, up: 1.0, sp: 2.2 },
            wet: { c0: col('#dfeaf5'), c1: col('#a9c2d8'), s0: 1.5, s1: 4.6, life: 1.2, a: 0.5, rate: 65, up: 1.2, sp: 2.6, spray: true },
            sand: { c0: col('#f0cf98'), c1: col('#cfa468'), s0: 1.8, s1: 7, life: 2.0, a: 0.7, rate: 85, up: 1.6, sp: 3.4 },
            gravel: { c0: col('#d2c6b2'), c1: col('#9c917f'), s0: 1.5, s1: 5.4, life: 1.6, a: 0.6, rate: 60, up: 1.2, sp: 2.8, stones: true },
            snow: { c0: col('#ffffff'), c1: col('#dcebff'), s0: 2.0, s1: 7.5, life: 2.0, a: 0.8, rate: 85, up: 1.8, sp: 3.4, powder: true },
            mud: { c0: col('#7a6048'), c1: col('#4e3d2d'), s0: 1.2, s1: 3.8, life: 1.2, a: 0.5, rate: 40, up: 0.9, sp: 2.2, clods: true },
            ice: { c0: col('#e8f6ff'), c1: col('#bfe0f5'), s0: 1.2, s1: 3.8, life: 1.3, a: 0.5, rate: 50, up: 0.9, sp: 1.8, powder: true }
        };
        const SKID = { asphalt: [0.012, 0.012, 0.015, 0.55], salt: [0.35, 0.35, 0.33, 0.3], oil: [0.01, 0.01, 0.012, 0.5], wet: [0.015, 0.015, 0.02, 0.25], sand: [0.32, 0.22, 0.11, 0.35], gravel: [0.2, 0.17, 0.13, 0.4], snow: [0.38, 0.45, 0.58, 0.45], mud: [0.05, 0.035, 0.02, 0.55], ice: [0.5, 0.65, 0.8, 0.22] };

        /* ---- particle store ---------------------------------------------- */
        let S = null, pools = null, dens = 1;
        function makeStore(cap) {
            const f = () => new Float32Array(cap);
            return {
                cap: cap, cur: 0, live: 0, x: f(), y: f(), z: f(), vx: f(), vy: f(), vz: f(), age: f(), life: f(), s0: f(), s1: f(), rot: f(), spin: f(),
                c0: new Float32Array(cap * 3), c1: new Float32Array(cap * 3), a: f(), drag: f(), grav: f(), str: f(), lit: f(),
                fr: new Uint8Array(cap), fl: new Uint8Array(cap)
            };
        }
        function makePool(cap, additive) {
            const geo = new T.InstancedBufferGeometry();
            geo.setAttribute('position', new T.BufferAttribute(new Float32Array([-1, -1, 0, 1, -1, 0, 1, 1, 0, -1, 1, 0]), 3));
            geo.setIndex([0, 1, 2, 0, 2, 3]);
            const mk = (name, n) => {
                const at = new T.InstancedBufferAttribute(new Float32Array(cap * n), n);
                at.setUsage(T.DynamicDrawUsage); geo.setAttribute(name, at);
                return at;
            };
            const pos = mk('aPos', 4), color = mk('aColor', 4), misc = mk('aMisc', 4), vel = mk('aVel', 3);
            geo.instanceCount = 0;
            const mat = new T.ShaderMaterial({
                uniforms: Object.assign({ uAdd: { value: additive ? 1 : 0 } }, U), vertexShader: PARTICLE_VS, fragmentShader: PARTICLE_FS,
                transparent: true, depthWrite: false, fog: false, blending: additive ? T.AdditiveBlending : T.NormalBlending
            });
            const mesh = new T.Mesh(geo, mat);
            mesh.frustumCulled = false; mesh.renderOrder = additive ? 6 : 5; mesh.visible = false; mesh.name = additive ? 'fx-add' : 'fx-alpha';
            root.add(mesh);
            return { geo: geo, mat: mat, mesh: mesh, pos: pos, color: color, misc: misc, vel: vel, n: 0 };
        }
        function buildPools() {
            if (pools) for (const p of pools) { p.mesh.removeFromParent(); p.geo.dispose(); p.mat.dispose(); }
            const cap = Math.max(60, gl.particles | 0);
            S = makeStore(cap);
            pools = [makePool(cap, false), makePool(cap, true)];
            dens = clamp(cap / 1500, 0.4, 1.5);
        }
        const cnt = n => Math.max(1, Math.round(n * (0.75 + 0.25 * dens)));

        function spawn(fr, add, x, y, z, vx, vy, vz, life, s0, s1, c0, c1, a, drag, grav, str, lit, spin, kind) {
            const i = S.cur; S.cur = (i + 1) % S.cap;
            S.x[i] = x; S.y[i] = y; S.z[i] = z; S.vx[i] = vx; S.vy[i] = vy; S.vz[i] = vz;
            S.age[i] = 0; S.life[i] = life; S.s0[i] = s0; S.s1[i] = s1; S.a[i] = a; S.drag[i] = drag; S.grav[i] = grav;
            S.str[i] = str; S.lit[i] = lit; S.rot[i] = rnd() * TAU; S.spin[i] = spin; S.fr[i] = fr; S.fl[i] = add | (kind << 1);
            S.c0[i * 3] = c0[0]; S.c0[i * 3 + 1] = c0[1]; S.c0[i * 3 + 2] = c0[2];
            S.c1[i * 3] = c1[0]; S.c1[i * 3 + 1] = c1[1]; S.c1[i * 3 + 2] = c1[2];
        }
        /* Lit smoke puff (alpha pool). grav < 0 is buoyancy. */
        const puff = (x, y, z, vx, vy, vz, life, s0, s1, c0, c1, a, drag, grav) =>
            spawn(rnd() < 0.5 ? F.PUFF : F.PUFF2, 0, x, y, z, vx, vy, vz, life, s0, s1, c0, c1, a, drag === undefined ? 1.4 : drag, grav || 0, 0, 1, rr(-1, 1), 0);
        /* Soft additive glow or flame blob. */
        const glow = (x, y, z, vx, vy, vz, life, s0, s1, c0, c1, a, drag, fr) => {
            const k = fr === F.STAR || fr === F.BOLT || fr === F.HEX ? 1.6 : 1;
            spawn(fr === undefined ? F.GLOW : fr, 1, x, y, z, vx, vy, vz, life, s0 * k, s1 * k, c0, c1, a, drag || 0, 0, 0, 0, rr(-2, 2), 0);
        };
        /* Velocity-stretched additive spark that bounces on the road. */
        const spark = (x, y, z, vx, vy, vz, life, size, c0, c1) =>
            spawn(F.SPARK, 1, x, y, z, vx, vy, vz, life, size * 1.7, size * 0.9, c0, c1, 1, 0.35, 18, 1, 0, 0, K_BOUNCE);
        /* A solid bit of something (alpha pool): shard, feather, leaf, confetti, drop, star. */
        const bit = (fr, x, y, z, vx, vy, vz, life, size, c0, c1, grav, kind, drag, add, stretch) =>
            spawn(fr, add || 0, x, y, z, vx, vy, vz, life, size * 1.8, size * (add ? 0.8 : 1.8), c0, c1, 1, drag || 0.25, grav, stretch || 0, 0, rr(-9, 9), kind);

        /* ---- point lights from the render3d pool -------------------------- */
        const LT = [];
        for (let i = 0; i < 12; i++) LT.push({ on: false, x: 0, y: 0, z: 0, r: 1, g: 1, b: 1, i: 0, cur: 0, ttl: 0, age: 0, dist: 30, once: false });
        const taken = new Uint8Array(LT.length);
        function addLight(x, y, z, c, intensity, ttl, dist, once) {
            let s = null;
            for (const l of LT) if (!l.on) { s = l; break; }
            if (!s) { s = LT[0]; for (const l of LT) if (l.cur < s.cur) s = l; if (s.cur > intensity && !once) return; }
            s.on = true; s.x = x; s.y = y; s.z = z; s.r = c[0]; s.g = c[1]; s.b = c[2]; s.i = intensity; s.cur = intensity;
            s.ttl = ttl; s.age = 0; s.dist = dist || 30; s.once = !!once;
        }
        function applyLights(dt) {
            const pool = scene.userData.lightPool;
            if (!pool || !pool.length) { for (const l of LT) l.on = false; return; }
            for (const l of LT) {
                if (!l.on || l.once) continue;
                l.age += dt;
                if (l.age >= l.ttl) { l.on = false; continue; }
                const k = 1 - l.age / l.ttl; l.cur = l.i * k * k;
            }
            taken.fill(0);
            for (let p = 0; p < pool.length; p++) {
                let best = -1, bi = 0.5;
                for (let k = 0; k < LT.length; k++) if (LT[k].on && !taken[k] && LT[k].cur > bi) { bi = LT[k].cur; best = k; }
                const light = pool[p];
                if (best < 0) { light.intensity = 0; continue; }
                taken[best] = 1;
                const l = LT[best];
                light.position.set(l.x, l.y, l.z);
                light.color.setRGB(Math.min(1, l.r), Math.min(1, l.g), Math.min(1, l.b));
                light.intensity = l.cur; light.distance = l.dist;
            }
            for (const l of LT) if (l.once) l.on = false;
        }

        /* ---- shock rings (flat meshes on the road) ------------------------ */
        const ringGeo = new T.PlaneGeometry(1, 1).rotateX(-Math.PI / 2);
        const rings = [];
        for (let i = 0; i < 12; i++) {
            const mat = new T.ShaderMaterial({
                uniforms: { uColor: { value: new T.Color() }, uAlpha: { value: 0 }, uIn: { value: 0.8 } }, vertexShader: RING_VS, fragmentShader: RING_FS,
                transparent: true, depthWrite: false, blending: T.AdditiveBlending, polygonOffset: true, polygonOffsetFactor: -3, polygonOffsetUnits: -3
            });
            const mesh = new T.Mesh(ringGeo, mat);
            mesh.visible = false; mesh.renderOrder = 4; mesh.frustumCulled = false;
            root.add(mesh);
            rings.push({ mesh: mesh, mat: mat, on: false, r0: 0, r1: 0, life: 1, age: 0, a: 1 });
        }
        let ringCur = 0;
        function ring(x, h, z, r0, r1, life, c, a) {
            const r = rings[ringCur]; ringCur = (ringCur + 1) % rings.length;
            r.on = true; r.r0 = r0; r.r1 = r1; r.life = life; r.age = 0; r.a = a === undefined ? 0.9 : a;
            r.mesh.position.set(x, h + 0.12, z);
            r.mat.uniforms.uColor.value.setRGB(c[0], c[1], c[2]);
            r.mesh.visible = true;
        }
        /* A camera-facing ring from the atlas, for blasts in the air. */
        const ringBb = (x, y, z, s0, s1, life, c, a) => spawn(F.RING, 1, x, y, z, 0, 0, 0, life, s0, s1, c, c, a === undefined ? 0.9 : a, 0, 0, 0, 0, 0, 0);

        /* ---- beams ----------------------------------------------------------- */
        const beams = [];
        for (let i = 0; i < 8; i++) {
            const mat = new T.ShaderMaterial({
                uniforms: {
                    uA: { value: new T.Vector3() }, uB: { value: new T.Vector3() }, uW0: { value: 1 }, uW1: { value: 1 }, uFlat: { value: 0 }, uColor: { value: new T.Color() },
                    uAlpha: { value: 1 }, uKind: { value: 0 }, uTime: { value: 0 }, uLen: { value: 1 }
                },
                vertexShader: BEAM_VS, fragmentShader: BEAM_FS, transparent: true, depthWrite: false, blending: T.AdditiveBlending, side: T.DoubleSide
            });
            const mesh = new T.Mesh(new T.PlaneGeometry(2, 2, 1, 1), mat);
            mesh.frustumCulled = false; mesh.visible = false; mesh.renderOrder = 7;
            root.add(mesh);
            beams.push({ mesh: mesh, mat: mat });
        }
        let beamN = 0;
        function beam(kind, ax, ay, az, bx, by, bz, w0, w1, c, a, flat) {
            if (beamN >= beams.length) return;
            const b = beams[beamN++], u = b.mat.uniforms;
            u.uA.value.set(ax, ay, az); u.uB.value.set(bx, by, bz);
            u.uW0.value = w0; u.uW1.value = w1; u.uColor.value.setRGB(c[0], c[1], c[2]); u.uAlpha.value = a; u.uKind.value = kind; u.uFlat.value = flat || 0;
            u.uLen.value = Math.hypot(bx - ax, by - ay, bz - az);
            b.mesh.visible = true;
        }

        /* ---- skid ribbon ------------------------------------------------------ */
        let skid = null;
        function buildSkid() {
            if (skid) { skid.mesh.removeFromParent(); skid.geo.dispose(); skid.mat.dispose(); }
            const cap = Math.max(100, gl.skid | 0);
            const geo = new T.BufferGeometry();
            const pos = new T.BufferAttribute(new Float32Array(cap * 12), 3), clr = new T.BufferAttribute(new Float32Array(cap * 16), 4);
            pos.setUsage(T.DynamicDrawUsage); clr.setUsage(T.DynamicDrawUsage);
            const idx = new Uint32Array(cap * 6);
            for (let i = 0; i < cap; i++) idx.set([i * 4, i * 4 + 1, i * 4 + 2, i * 4 + 1, i * 4 + 3, i * 4 + 2], i * 6);
            geo.setAttribute('position', pos); geo.setAttribute('color', clr); geo.setIndex(new T.BufferAttribute(idx, 1));
            geo.setDrawRange(0, 0);
            const mat = new T.MeshBasicMaterial({ vertexColors: true, transparent: true, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -4, polygonOffsetUnits: -4 });
            const mesh = new T.Mesh(geo, mat);
            mesh.frustumCulled = false; mesh.renderOrder = 2;
            root.add(mesh);
            skid = { cap: cap, cur: 0, filled: 0, lo: 1e9, hi: -1, full: false, geo: geo, mat: mat, mesh: mesh, pos: pos, clr: clr };
        }
        /* One wheel trail: the previous end edge becomes the next start edge, so curves stay closed. */
        function skidAdd(w, x, z, width, c) {
            if (!w.has) { w.has = true; w.x = x; w.z = z; w.lx = w.rx = w.ly = w.ry = NaN; return; }
            const dx = x - w.x, dz = z - w.z, len = Math.hypot(dx, dz);
            if (len < 0.55) return;
            const nx = -dz / len * width * 0.5, nz = dx / len * width * 0.5;
            const lx = x + nx, lz = z + nz, rx = x - nx, rz = z - nz;
            if (w.lx !== w.lx) { w.lx = w.x + nx; w.lz = w.z + nz; w.rx = w.x - nx; w.rz = w.z - nz; }
            const i = skid.cur, p = skid.pos.array, k = skid.clr.array, o = i * 12, ko = i * 16, y = 0.045;
            p[o] = w.lx; p[o + 1] = y; p[o + 2] = w.lz; p[o + 3] = w.rx; p[o + 4] = y; p[o + 5] = w.rz;
            p[o + 6] = lx; p[o + 7] = y; p[o + 8] = lz; p[o + 9] = rx; p[o + 10] = y; p[o + 11] = rz;
            for (let v = 0; v < 4; v++) { k[ko + v * 4] = c[0]; k[ko + v * 4 + 1] = c[1]; k[ko + v * 4 + 2] = c[2]; k[ko + v * 4 + 3] = c[3] * (v < 2 ? 0.7 : 1); }
            skid.lo = Math.min(skid.lo, i); skid.hi = Math.max(skid.hi, i);
            skid.cur = (i + 1) % skid.cap;
            if (skid.cur === 0) skid.full = true;
            if (skid.filled < skid.cap) skid.filled++;
            w.x = x; w.z = z; w.lx = lx; w.lz = lz; w.rx = rx; w.rz = rz;
        }
        function flushSkid() {
            if (skid.hi < 0) return;
            const lo = skid.lo, hi = skid.hi;
            skid.pos.clearUpdateRanges(); skid.clr.clearUpdateRanges();
            skid.pos.addUpdateRange(lo * 12, (hi - lo + 1) * 12); skid.clr.addUpdateRange(lo * 16, (hi - lo + 1) * 16);
            skid.pos.needsUpdate = true; skid.clr.needsUpdate = true;
            skid.geo.setDrawRange(0, (skid.full ? skid.cap : skid.filled) * 6);
            skid.lo = 1e9; skid.hi = -1;
        }

        /* ---- state -------------------------------------------------------------- */
        let lastRace = null, time = 0, eyeX = 0, eyeZ = 0, eyeSet = false, night = false;
        const cst = [], timers = [], ufos = [];
        const carState = i => cst[i] || (cst[i] = { wl: { has: false }, wr: { has: false } });
        function use(race) {
            if (race === lastRace) return;
            lastRace = race; cst.length = 0; timers.length = 0; ufos.length = 0;
            const look = AD.LOOK[race.track.biomeId] || AD.LOOK.alpine;
            const d = AD.sunDirection(look, 16); U.uSunDir.value.set(d[0], d[1], d[2]);
            night = !!look.night;
            U.uSunCol.value.set(look.sun).multiplyScalar(night ? 0.7 : 1.3);
            colour.set(look.hemiSky).lerp(new T.Color(look.hemiGround), 0.4);
            U.uAmb.value.copy(colour).multiplyScalar(0.7 * look.hemi).addScalar(night ? 0.1 : 0.16);
        }
        const carOf = (race, i) => (race.cars[i] ? race.cars[i].v : null);
        const geoOf = v => GEO[v.car.id] || GEO_DEFAULT;
        const profOf = v => SURF[v.surface] || SURF.asphalt;
        const near = (x, z, r) => !eyeSet || ((x - eyeX) * (x - eyeX) + (z - eyeZ) * (z - eyeZ) < r * r);
        function later(delay, fn) { timers.push({ t: delay, fn: fn }); }

        /* ---- one-shot effects ---------------------------------------------------- */
        function explode(x, z, h, s) {
            glow(x, h, z, 0, 0, 0, 0.12, 4 * s, 8 * s, HOT, HOT, 0.8);
            ring(x, 0, z, 0.8 * s, 11 * s, 0.55, HOT, 0.45);
            ringBb(x, h + 0.8, z, 2 * s, 20 * s, 0.45, FIRE, 0.6);
            /* Fireball: blended (not additive) flame puffs, so the edges go red and sooty instead of burning out to white. */
            for (let k = cnt(34 * s); k > 0; k--) {
                const zz = rr(-0.1, 1), an = rnd() * TAU, sp = rr(2.5, 9) * s, ra = Math.sqrt(1 - zz * zz);
                spawn(k % 3 ? F.PUFF : F.PUFF2, 0, x + rr(-1, 1), h + 0.6, z + rr(-1, 1), Math.cos(an) * ra * sp, zz * sp * 0.9 + 2, Math.sin(an) * ra * sp,
                    rr(0.6, 1.3), rr(2, 3.2) * s, rr(5, 8) * s, k % 4 === 0 ? FB_HOT : k % 4 === 1 ? FB_DARK : FB0, FB1, 0.95, 2.0, -1.2, 0, 0, rr(-1.5, 1.5), 0);
            }
            for (let k = cnt(8 * s); k > 0; k--) glow(x + rr(-1, 1), h + rr(0.4, 1.6), z + rr(-1, 1), 0, rr(1, 3), 0, rr(0.3, 0.5), 3 * s, 5 * s, HOT, FB0, 0.6, 0, F.FLAME);
            for (let k = cnt(13 * s); k > 0; k--) { const an = rnd() * TAU, sp = rr(0.5, 3.5) * s; puff(x + rr(-1, 1) * s, h + rr(0.4, 2), z + rr(-1, 1) * s, Math.cos(an) * sp, rr(2.5, 5.5) * s, Math.sin(an) * sp, rr(2.8, 4.4), rr(3.5, 5) * s, rr(9, 14) * s, SOOT0, SOOT1, 0.85, 0.6, -0.5); }
            for (let k = cnt(16 * s); k > 0; k--) { const an = rnd() * TAU, sp = rr(4, 14) * s; bit(F.SHARD, x, h + 0.5, z, Math.cos(an) * sp, rr(6, 16) * s, Math.sin(an) * sp, rr(0.9, 1.6), rr(0.25, 0.55), FIRE, BLACK, 24, K_BOUNCE, 0.2, 0, 0.35); }
            for (let k = cnt(28 * s); k > 0; k--) { const an = rnd() * TAU, sp = rr(10, 28) * s; spark(x, h + 0.5, z, Math.cos(an) * sp, rr(3, 14) * s, Math.sin(an) * sp, rr(0.5, 1.1), rr(0.3, 0.5), HOT, EMBER); }
            for (let k = cnt(9); k > 0; k--) glow(x + rr(-2, 2) * s, h + 1, z + rr(-2, 2) * s, rr(-2, 2), rr(1, 4), rr(-2, 2), rr(1.4, 2.6), 0.22, 0.05, FIRE, EMBER, 0.9, 0.5, F.DOT);
            addLight(x, h + 1.8, z, FIRE, 340 * s * s, 0.5, 22 + 10 * s);
        }
        function wallSparks(race, x, z, speed, heavy, car) {
            const sm = AD.sampleTrack(race.track, x, z), side = sm.offset >= 0 ? 1 : -1;
            const inx = -side * sm.nx, inz = -side * sm.ny, v = carOf(race, car);
            const dir = v && (Math.cos(v.psi) * sm.tx + Math.sin(v.psi) * sm.ty) < 0 ? -1 : 1;
            const px = x - inx * 0.8, pz = z - inz * 0.8;
            for (let k = cnt(heavy ? 38 : 16); k > 0; k--) {
                const sp = speed * rr(0.25, 0.8) * dir, side2 = rr(3, 11);
                spark(px, rr(0.15, 0.8), pz, sm.tx * sp + inx * side2, rr(1.5, 8), sm.ty * sp + inz * side2, rr(0.35, 0.9), rr(0.28, 0.5), HOT, FIRE);
            }
            glow(px, 0.5, pz, 0, 0, 0, 0.12, 1.5, 3, HOT, FIRE, 0.9);
            for (let k = cnt(heavy ? 5 : 2); k > 0; k--) puff(px, 0.5, pz, inx * rr(1, 3), rr(0.5, 1.5), inz * rr(1, 3), 1.0, 0.7, 2.4, SMOKE1, SMOKE1, 0.35);
            if (car === 0) addLight(px, 1, pz, FIRE, heavy ? 60 : 25, 0.2, 14);
        }
        function turboBurst(v, level, power) {
            const g = geoOf(v), cp = Math.cos(v.psi), sp = Math.sin(v.psi), rx = -sp, rz = cp;
            const c = TURBO[clamp(level - 1, 0, 2)], cs = TURBO_SOFT[clamp(level - 1, 0, 2)];
            for (const side of [-1, 1]) {
                const ex = v.x - cp * g[1] + rx * g[2] * 0.5 * side, ez = v.y - sp * g[1] + rz * g[2] * 0.5 * side, h = v.z + 0.5;
                glow(ex, h, ez, -cp * 4, 0, -sp * 4, 0.16, 2.2 * power, 4 * power, WHITE, c, 1);
                for (let k = cnt(14 * power); k > 0; k--) {
                    const sd = 8 + rr(0, 12) * power;
                    glow(ex, h + rr(-0.1, 0.2), ez, -cp * sd + rx * rr(-1.6, 1.6), rr(-0.6, 1.4), -sp * sd + rz * rr(-1.6, 1.6), rr(0.22, 0.46), rr(0.9, 1.4) * power, 0.2, c, cs, 0.95, 1.2, F.FLAME);
                }
                for (let k = cnt(6); k > 0; k--) spark(ex, h, ez, -cp * rr(10, 26) + rx * rr(-4, 4), rr(0, 4), -sp * rr(10, 26) + rz * rr(-4, 4), rr(0.3, 0.6), 0.35, c, cs);
            }
            addLight(v.x - cp * (g[1] + 1.2), v.z + 0.9, v.y - sp * (g[1] + 1.2), c, 55 * power, 0.35, 16);
        }
        function dustRing(x, z, n, speed, prof, size) {
            for (let k = cnt(n); k > 0; k--) { const an = (k / n) * TAU + rr(-0.2, 0.2), sp = speed * rr(0.6, 1.2); puff(x + Math.cos(an) * 0.8, 0.5, z + Math.sin(an) * 0.8, Math.cos(an) * sp, rr(0.4, 1.4), Math.sin(an) * sp, rr(0.9, 1.6), size * 0.45, size, prof.c0, prof.c1, prof.a * 0.9, 2.2, -0.3); }
        }
        function landing(evt, race) {
            const v = carOf(race, evt.car), prof = v ? profOf(v) : SURF.asphalt, q2 = evt.quality;
            const big = q2 === 'crash' ? 1.5 : q2 === 'perfect' ? 0.9 : q2 === 'good' ? 1 : 0.8;
            dustRing(evt.x, evt.y, 14 * big, 7 * big, prof, 2.8 * big);
            ring(evt.x, 0, evt.y, 0.6, 6.5 * big, 0.45, q2 === 'perfect' ? GOLD : WHITE, 0.55);
            if (q2 === 'crash') {
                for (let k = cnt(26); k > 0; k--) { const an = rnd() * TAU, sp = rr(6, 18); spark(evt.x, 0.4, evt.y, Math.cos(an) * sp, rr(3, 9), Math.sin(an) * sp, rr(0.4, 0.9), 0.4, HOT, FIRE); }
                for (let k = cnt(5); k > 0; k--) puff(evt.x + rr(-1, 1), 0.8, evt.y + rr(-1, 1), rr(-2, 2), rr(1, 3), rr(-2, 2), 2, 1.5, 5, SOOT0, SOOT1, 0.6, 0.8, -0.4);
            } else if (q2 === 'perfect') {
                for (let k = cnt(12); k > 0; k--) { const an = rnd() * TAU, sp = rr(3, 8); bit(F.STAR, evt.x, 0.8, evt.y, Math.cos(an) * sp, rr(3, 9), Math.sin(an) * sp, rr(0.7, 1.2), rr(0.4, 0.8), GOLD, GOLD, 8, 0, 1.2, 1); }
                ringBb(evt.x, 1, evt.y, 1, 9, 0.5, GOLD, 0.8);
            }
        }
        function confetti(x, z, race, big) {
            const sm = AD.sampleTrack(race.track, x, z), hw = sm.halfWidth + 2.5;
            const shot = (px, pz, vx, vz, n) => {
                for (let k = cnt(n); k > 0; k--) {
                    const c = PARTY[(Math.random() * PARTY.length) | 0], sp = rr(0.4, 1);
                    bit(F.CONFETTI, px, rr(0.5, 2), pz, (vx + rr(-5, 5)) * sp, rr(12, 26) * sp, (vz + rr(-5, 5)) * sp, rr(2.4, 4), rr(0.28, 0.46), c, c, 8, K_FLUTTER, 1.5);
                }
            };
            for (const side of [-1, 1]) shot(sm.x + sm.nx * hw * side, sm.y + sm.ny * hw * side, -sm.nx * side * 5 + sm.tx * 2, -sm.ny * side * 5 + sm.ty * 2, big ? 55 : 12);
            for (let k = cnt(big ? 60 : 12); k > 0; k--) {
                const c = PARTY[(Math.random() * PARTY.length) | 0];
                bit(F.CONFETTI, x + rr(-9, 9), rr(10, 16), z + rr(-9, 9), rr(-3, 3), rr(-2, 1), rr(-3, 3), rr(2.6, 4.2), rr(0.28, 0.46), c, c, 3, K_FLUTTER, 1.6);
            }
            if (big) for (let k = cnt(10); k > 0; k--) { const an = rnd() * TAU, sp = rr(4, 11); bit(F.STAR, x, 1.5, z, Math.cos(an) * sp, rr(5, 12), Math.sin(an) * sp, rr(0.8, 1.4), rr(0.5, 0.9), GOLD, GOLD, 8, 0, 1.2, 1); }
        }
        const FEATHER_SETS = { cow: ['#ffffff', '#262626'], sheep: ['#fbfbf6', '#e9e6dc'], penguin: ['#16181c', '#f5f7fa'], llama: ['#e7d2b0', '#c9a97a'], crab: ['#ff7a45', '#ffd2b0'] };
        function creatureBurst(x, z, kind, ducks) {
            const set = ducks ? ['#ffe14a', '#fff2a8'] : (FEATHER_SETS[kind] || FEATHER_SETS.cow);
            const a = col(set[0]), b = col(set[1]);
            for (let k = cnt(ducks ? 14 : 22); k > 0; k--) {
                const an = rnd() * TAU, sp = rr(2, 9);
                if (kind === 'sheep' && !ducks && k % 2) puff(x, 1.3, z, Math.cos(an) * sp * 0.5, rr(2, 5), Math.sin(an) * sp * 0.5, rr(1, 1.7), 0.5, 1.5, a, b, 0.8, 1.6, 1);
                else bit(F.FEATHER, x, rr(0.8, 1.8), z, Math.cos(an) * sp, rr(3, 9), Math.sin(an) * sp, rr(1.6, 2.8), rr(0.55, 0.9), k % 2 ? a : b, b, 4.5, K_FLUTTER, 1.1);
            }
            for (let k = cnt(7); k > 0; k--) { const an = rnd() * TAU, sp = rr(2, 6); bit(F.STAR, x, 1.8, z, Math.cos(an) * sp, rr(4, 9), Math.sin(an) * sp, rr(0.7, 1.2), rr(0.45, 0.8), GOLD, GOLD, 6, 0, 1.1, 1); }
            dustRing(x, z, 6, 4, SURF.sand, 1.8);
        }
        const SMASH = {
            cone: [col('#ff7a1a'), col('#ffffff'), F.SHARD], hay: [STRAW0, STRAW1, F.LEAF], barrel: [col('#d83a3a'), col('#8f9aa8'), F.SHARD], tyres: [col('#1a1a1c'), col('#3a3a3e'), F.SHARD]
        };
        function smash(evt, race) {
            const s = SMASH[evt.kind] || SMASH.cone, v = carOf(race, evt.car), prof = v ? profOf(v) : SURF.asphalt;
            for (let k = cnt(evt.kind === 'hay' ? 18 : 12); k > 0; k--) { const an = rnd() * TAU, sp = rr(3, 10); bit(s[2], evt.x, rr(0.3, 1), evt.y, Math.cos(an) * sp, rr(4, 10), Math.sin(an) * sp, rr(1.0, 1.8), rr(0.28, 0.55), k % 2 ? s[0] : s[1], s[1], 18, K_BOUNCE, 0.2); }
            for (let k = cnt(3); k > 0; k--) { const an = rnd() * TAU; bit(F.STAR, evt.x, 1.2, evt.y, Math.cos(an) * 3, rr(4, 8), Math.sin(an) * 3, 0.8, 0.6, GOLD, GOLD, 8, 0, 1.2, 1); }
            dustRing(evt.x, evt.y, 5, 3, evt.kind === 'tyres' ? { c0: SOOT0, c1: SOOT1, a: 0.6 } : evt.kind === 'hay' ? { c0: STRAW0, c1: STRAW1, a: 0.5 } : prof, 1.8);
        }
        function crateBurst(evt) {
            glow(evt.x, 1, evt.y, 0, 0, 0, 0.2, 2, 5, HOT, GOLD, 0.9);
            ring(evt.x, 0, evt.y, 0.5, 4.5, 0.4, GOLD, 0.6);
            for (let k = cnt(10); k > 0; k--) { const an = rnd() * TAU, sp = rr(3, 8); bit(F.SHARD, evt.x, 1, evt.y, Math.cos(an) * sp, rr(4, 9), Math.sin(an) * sp, rr(0.8, 1.4), rr(0.25, 0.45), WOOD, WOOD, 20, K_BOUNCE, 0.2); }
            for (let k = cnt(9); k > 0; k--) { const an = rnd() * TAU, sp = rr(2, 7); bit(F.STAR, evt.x, 1, evt.y, Math.cos(an) * sp, rr(3, 8), Math.sin(an) * sp, rr(0.6, 1.1), rr(0.4, 0.7), GOLD, GOLD, 6, 0, 1.2, 1); }
        }
        function emp(evt, race) {
            const r = evt.r || 20;
            ring(evt.x, 0, evt.y, 1, r, 0.6, ELEC, 0.9);
            ringBb(evt.x, 1.5, evt.y, 2, r * 1.1, 0.5, ELEC, 0.5);
            glow(evt.x, 1.2, evt.y, 0, 0, 0, 0.2, 2, 6, WHITE, ELEC, 0.7);
            for (let k = cnt(22); k > 0; k--) { const an = rnd() * TAU, d = r * rr(0.55, 1); glow(evt.x + Math.cos(an) * d, rr(0.2, 2.5), evt.y + Math.sin(an) * d, 0, 0, 0, rr(0.15, 0.35), rr(1.2, 2.6), 1, ELEC, ELEC, 0.95, 0, F.BOLT); }
            for (let k = cnt(8); k > 0; k--) { const an = rnd() * TAU; glow(evt.x, 1, evt.y, Math.cos(an) * r * 1.2, 0, Math.sin(an) * r * 1.2, 0.5, 0.5, 0.9, ELEC, ELEC, 0.6, 0, F.HEX); }
            addLight(evt.x, 2, evt.y, ELEC, 260, 0.5, 30);
        }
        function railFire(evt) {
            const dx = evt.x2 - evt.x, dz = evt.y2 - evt.y, len = Math.hypot(dx, dz) || 1, steps = Math.min(40, Math.ceil(len / 6));
            for (let k = 0; k < steps; k++) {
                const t = k / steps;
                glow(evt.x + dx * t, 0.9, evt.y + dz * t, rr(-1, 1), rr(0, 1.5), rr(-1, 1), rr(0.3, 0.6), rr(0.8, 1.6), 0.2, ELEC, PINK, 0.8, 0.5);
            }
            glow(evt.x, 0.9, evt.y, 0, 0, 0, 0.18, 2, 6, WHITE, ELEC, 1);
            glow(evt.x2, 0.9, evt.y2, 0, 0, 0, 0.2, 3, 9, WHITE, PINK, 1);
            for (let k = cnt(16); k > 0; k--) { const an = rnd() * TAU, sp = rr(6, 20); spark(evt.x2, 0.9, evt.y2, Math.cos(an) * sp, rr(2, 10), Math.sin(an) * sp, rr(0.3, 0.7), 0.35, WHITE, PINK); }
            ring(evt.x2, 0, evt.y2, 0.5, 6, 0.35, PINK, 0.8);
            addLight(evt.x, 1.5, evt.y, ELEC, 120, 0.3, 24);
            addLight(evt.x2, 1.5, evt.y2, PINK, 150, 0.3, 24);
        }
        function hitFx(evt, race) {
            const v = carOf(race, evt.car), h = (v ? v.z : 0) + 0.7, w = evt.weapon;
            if (w === 'mg') {
                for (let k = cnt(7); k > 0; k--) { const an = rnd() * TAU, sp = rr(5, 14); spark(evt.x, h, evt.y, Math.cos(an) * sp, rr(2, 8), Math.sin(an) * sp, rr(0.25, 0.5), 0.3, HOT, FIRE); }
                glow(evt.x, h, evt.y, 0, 0, 0, 0.08, 1, 2.2, HOT, FIRE, 0.9);
            } else if (w === 'ducks') {
                creatureBurst(evt.x, evt.y, 'duck', true);
            } else if (w === 'glove') {
                for (let k = cnt(8); k > 0; k--) { const an = rnd() * TAU; bit(F.STAR, evt.x, h, evt.y, Math.cos(an) * 5, rr(3, 8), Math.sin(an) * 5, 0.8, 0.7, GOLD, GOLD, 8, 0, 1.2, 1); }
                ringBb(evt.x, h, evt.y, 0.8, 5, 0.3, WHITE, 0.8);
            } else if (w === 'harpoon') {
                for (let k = cnt(10); k > 0; k--) { const an = rnd() * TAU, sp = rr(5, 12); spark(evt.x, h, evt.y, Math.cos(an) * sp, rr(2, 7), Math.sin(an) * sp, rr(0.3, 0.6), 0.32, WHITE, FIRE); }
            } else {
                explode(evt.x, evt.y, h, evt.heavy ? 1.5 : 0.9);
            }
        }
        function eliminated(evt) {
            glow(evt.x, 1.5, evt.y, 0, 0, 0, 0.3, 2, 9, WHITE, GOLD, 0.9);
            ringBb(evt.x, 1.5, evt.y, 1, 10, 0.5, GOLD, 0.8);
            for (let k = cnt(22); k > 0; k--) { const an = rnd() * TAU, sp = rr(5, 16), c = PARTY[(Math.random() * PARTY.length) | 0]; bit(k % 3 ? F.STAR : F.CONFETTI, evt.x, 1.5, evt.y, Math.cos(an) * sp, rr(4, 14), Math.sin(an) * sp, rr(1.2, 2.2), rr(0.5, 0.9), c, c, 9, K_FLUTTER, 1.0, k % 3 ? 1 : 0); }
            for (let k = cnt(8); k > 0; k--) { const an = rnd() * TAU, sp = rr(2, 6); puff(evt.x, 1.5, evt.y, Math.cos(an) * sp, rr(1, 4), Math.sin(an) * sp, rr(1, 1.8), 1.5, 4.5, WHITE, SMOKE1, 0.85, 1.6, -0.3); }
        }
        function frostBurst(evt) {
            const r = evt.r || 8;
            ring(evt.x, 0, evt.y, 1, r * 1.3, 0.5, ICE0, 0.8);
            glow(evt.x, 0.8, evt.y, 0, 0, 0, 0.25, 2, r * 1.2, WHITE, ICE1, 0.8);
            for (let k = cnt(24); k > 0; k--) { const an = rnd() * TAU, sp = rr(4, 14); bit(F.SHARD, evt.x, 0.6, evt.y, Math.cos(an) * sp, rr(4, 11), Math.sin(an) * sp, rr(0.9, 1.6), rr(0.25, 0.55), ICE0, ICE1, 20, K_BOUNCE, 0.3); }
            for (let k = cnt(12); k > 0; k--) { const an = rnd() * TAU, sp = rr(1, 5); puff(evt.x + Math.cos(an) * 2, 0.5, evt.y + Math.sin(an) * 2, Math.cos(an) * sp, rr(0.5, 2), Math.sin(an) * sp, rr(1.4, 2.4), 1.5, 4.5, WHITE, ICE1, 0.6, 1.4, -0.2); }
            for (let k = cnt(10); k > 0; k--) glow(evt.x + rr(-r, r) * 0.6, rr(0.5, 2.5), evt.y + rr(-r, r) * 0.6, 0, rr(0.5, 1.5), 0, rr(0.8, 1.4), 0.7, 0.1, WHITE, ICE0, 0.95, 0, F.STAR);
            addLight(evt.x, 1.5, evt.y, ICE0, 90, 0.4, 20);
        }
        function sparkle(x, z, c, n) {
            for (let k = cnt(n); k > 0; k--) { const an = rnd() * TAU, d = rr(0.4, 1.8); glow(x + Math.cos(an) * d, rr(0, 1), z + Math.sin(an) * d, 0, rr(2, 6), 0, rr(0.6, 1.1), rr(0.35, 0.7), 0.1, c, c, 0.95, 0.3, k % 2 ? F.STAR : F.GLOW); }
        }
        function blackHole(evt, close) {
            const r = evt.r || 26;
            if (close) {
                glow(evt.x, 2, evt.y, 0, 0, 0, 0.3, 3, 16, WHITE, PURPLE, 1);
                ring(evt.x, 0, evt.y, r, 1, 0.5, PURPLE, 0.8);
                for (let k = cnt(30); k > 0; k--) { const an = rnd() * TAU, d = rr(0.4, 1) * r; glow(evt.x + Math.cos(an) * d, rr(0.3, 3), evt.y + Math.sin(an) * d, -Math.cos(an) * d * 2.2, 0, -Math.sin(an) * d * 2.2, 0.45, 0.9, 0.1, PURPLE, PINK, 0.9, 0); }
                addLight(evt.x, 3, evt.y, PURPLE, 220, 0.4, 28);
            } else {
                glow(evt.x, 2, evt.y, 0, 0, 0, 0.4, 2, 16, WHITE, PURPLE, 0.9);
                ring(evt.x, 0, evt.y, 1, r, 0.7, PURPLE, 0.8);
                ringBb(evt.x, 2.5, evt.y, 2, r * 1.2, 0.6, PURPLE, 0.6);
            }
        }

        /* ---- event dispatch --------------------------------------------------------- */
        function event(evt, race) {
            if (!race || !S) return;
            use(race);
            const v = evt.car !== undefined ? carOf(race, evt.car) : null;
            switch (evt.type) {
                case 'perfect_start': if (v) { turboBurst(v, 1, 1.4); smokeBurst(v, 18); } break;
                case 'wheelspin': if (v && near(v.x, v.y, 220)) smokeBurst(v, evt.car === 0 ? 16 : 7); break;
                case 'drift_start': if (v) smokeBurst(v, 5); break;
                case 'drift_level': if (v) driftFlare(v, evt.level); break;
                case 'turbo_burst': if (v) turboBurst(v, evt.level || 1, 0.8 + 0.3 * (evt.level || 1)); break;
                case 'wall_hit': wallSparks(race, evt.x, evt.y, evt.speed || 20, evt.heavy, evt.car); break;
                case 'car_hit':
                    for (let k = cnt(evt.heavy ? 22 : 10); k > 0; k--) { const an = rnd() * TAU, sp = rr(4, 14); spark(evt.x, rr(0.3, 0.9), evt.y, Math.cos(an) * sp, rr(1, 6), Math.sin(an) * sp, rr(0.3, 0.7), 0.3, HOT, FIRE); }
                    glow(evt.x, 0.7, evt.y, 0, 0, 0, 0.1, 1.2, 3, HOT, FIRE, 0.8);
                    break;
                case 'slingshot': if (v) windStreaks(v, 18); break;
                case 'takeoff': if (v) { dustRing(evt.x, evt.y, 8, 4, profOf(v), 2.4); windStreaks(v, 8); } break;
                case 'landing': landing(evt, race); break;
                case 'boost_pad':
                    glow(evt.x, 0.6, evt.y, 0, 0, 0, 0.25, 2, 7, WHITE, TURBO[0], 0.9);
                    ring(evt.x, 0, evt.y, 0.5, 4.5, 0.4, TURBO[0], 0.7);
                    if (v) windStreaks(v, 14);
                    addLight(evt.x, 1.2, evt.y, TURBO[0], 70, 0.35, 18);
                    break;
                case 'pickup': crateBurst(evt); break;
                case 'smash': smash(evt, race); break;
                case 'animal_hit': creatureBurst(evt.x, evt.y, evt.kind, false); break;
                case 'duck_hit': creatureBurst(evt.x, evt.y, 'duck', true); break;
                case 'finish': if (v) { const big = evt.car === 0; confetti(v.x, v.y, race, big); if (big) { later(0.5, () => confetti(v.x + Math.cos(v.psi) * 20, v.y + Math.sin(v.psi) * 20, race, true)); later(1.1, () => confetti(v.x + Math.cos(v.psi) * 40, v.y + Math.sin(v.psi) * 40, race, true)); } } break;
                case 'respawn': ring(evt.x, 0, evt.y, 0.5, 4, 0.6, TEAL, 0.7); sparkle(evt.x, evt.y, TEAL, 22); break;
                case 'ufo_abduct': ufos.push({ x: evt.x, z: evt.y, t0: time, dur: 5.5 }); break;
                case 'hit': hitFx(evt, race); break;
                case 'shield':
                    ring(evt.x, 0, evt.y, 0.5, 4.5, 0.5, TEAL, 0.7); sparkle(evt.x, evt.y, TEAL, 12);
                    break;
                case 'shield_block':
                    ringBb(evt.x, 1, evt.y, 1.5, evt.broken ? 9 : 6, 0.35, TEAL, 0.9);
                    for (let k = cnt(evt.broken ? 18 : 8); k > 0; k--) { const an = rnd() * TAU, sp = rr(4, 11); if (evt.broken) bit(F.HEX, evt.x, 1, evt.y, Math.cos(an) * sp, rr(2, 7), Math.sin(an) * sp, 0.7, 0.7, TEAL, TEAL, 8, 0, 1.2, 1); else spark(evt.x, 1, evt.y, Math.cos(an) * sp, rr(1, 5), Math.sin(an) * sp, 0.35, 0.3, TEAL, TEAL); }
                    break;
                case 'emp_charge': ringBb(evt.x, 1, evt.y, 9, 1, 0.28, ELEC, 0.8); break;
                case 'emp_pulse': emp(evt, race); break;
                case 'emp_hit':
                    for (let k = cnt(9); k > 0; k--) { const an = rnd() * TAU, sp = rr(3, 9); spark(evt.x, 0.9, evt.y, Math.cos(an) * sp, rr(2, 6), Math.sin(an) * sp, rr(0.2, 0.45), 0.3, ELEC, ELEC); }
                    glow(evt.x, 1, evt.y, 0, 0, 0, 0.2, 1.5, 3.5, ELEC, ELEC, 0.9, 0, F.BOLT);
                    break;
                case 'rail_charge': glow(evt.x, 0.9, evt.y, 0, 0, 0, 0.35, 0.5, 2.4, ELEC, WHITE, 0.9); break;
                case 'rail_fire': railFire(evt); break;
                case 'flame_hit': if (carOf(race, evt.car)) embers(carOf(race, evt.car), 10); break;
                case 'toast': if (v) for (let k = cnt(10); k > 0; k--) puff(v.x + rr(-1, 1), v.z + 1, v.y + rr(-1, 1), rr(-1.5, 1.5), rr(2, 4), rr(-1.5, 1.5), rr(1.4, 2.4), 1.2, 4, SOOT0, SOOT1, 0.8, 0.8, -0.5); break;
                case 'frost_throw': glow(evt.x, 1.2, evt.y, 0, 0, 0, 0.2, 0.8, 2.2, ICE0, ICE1, 0.8); break;
                case 'frost_burst': frostBurst(evt); break;
                case 'horn':
                    ring(evt.x, 0, evt.y, 1, evt.r || 20, 0.55, HOT, 0.7);
                    if (v) { ringBb(evt.x + Math.cos(v.psi) * 2, 1, evt.y + Math.sin(v.psi) * 2, 1, 8, 0.4, WHITE, 0.7); dustRing(evt.x, evt.y, 14, 12, SURF.asphalt, 2.4); }
                    break;
                case 'hole_open': blackHole(evt, false); break;
                case 'hole_close': blackHole(evt, true); break;
                case 'disco_on': sparkle(evt.x, evt.y, PINK, 20); ring(evt.x, 0, evt.y, 1, evt.r || 22, 0.7, PINK, 0.6); break;
                case 'glove':
                    if (evt.hit) { for (let k = cnt(9); k > 0; k--) { const an = rnd() * TAU; bit(F.STAR, evt.x, 1, evt.y, Math.cos(an) * 6, rr(3, 8), Math.sin(an) * 6, 0.8, 0.7, GOLD, GOLD, 8, 0, 1.2, 1); } ringBb(evt.x, 1, evt.y, 1, 6, 0.3, WHITE, 0.9); }
                    else if (v) ringBb(v.x + Math.cos(v.psi) * 2.5, 1, v.y + Math.sin(v.psi) * 2.5, 2, 0.6, 0.18, WHITE, 0.7);
                    break;
                case 'harpoon_break': { const t = carOf(race, evt.target); if (v && t) for (let k = cnt(12); k > 0; k--) { const an = rnd() * TAU, sp = rr(4, 10); spark((v.x + t.x) / 2, 0.9, (v.y + t.y) / 2, Math.cos(an) * sp, rr(1, 6), Math.sin(an) * sp, 0.4, 0.3, WHITE, FIRE); } break; }
                case 'cow_land': {
                    const r = evt.r || 9;
                    ring(evt.x, 0, evt.y, 1, r * 1.4, 0.55, WHITE, 0.6);
                    dustRing(evt.x, evt.y, 22, 9, SURF.sand, 3.2);
                    for (let k = cnt(14); k > 0; k--) { const an = rnd() * TAU, sp = rr(4, 12); bit(F.SHARD, evt.x, 0.5, evt.y, Math.cos(an) * sp, rr(5, 12), Math.sin(an) * sp, rr(0.9, 1.5), rr(0.3, 0.6), ROCK, ROCK, 22, K_BOUNCE, 0.2); }
                    for (let k = cnt(8); k > 0; k--) { const an = rnd() * TAU; bit(F.STAR, evt.x, 1.5, evt.y, Math.cos(an) * 4, rr(5, 10), Math.sin(an) * 4, 0.9, 0.7, GOLD, GOLD, 8, 0, 1.2, 1); }
                    break;
                }
                case 'overdrive_on': if (v) turboBurst(v, 2, 1.1); break;
                case 'eliminated': eliminated(evt); break;
                case 'bomb_pass':
                    for (let k = cnt(10); k > 0; k--) { const an = rnd() * TAU, sp = rr(3, 8); spark(evt.x, 1.5, evt.y, Math.cos(an) * sp, rr(2, 6), Math.sin(an) * sp, 0.4, 0.3, GOLD, FIRE); }
                    ringBb(evt.x, 1.5, evt.y, 0.8, 5, 0.3, GOLD, 0.8);
                    break;
                case 'bomb_blast': explode(evt.x, evt.y, 0.8, clamp((evt.r || 8) / 6, 1, 2.2)); break;
                default: break;
            }
        }
        function smokeBurst(v, n) {
            const g = geoOf(v), prof = profOf(v), cp = Math.cos(v.psi), sp = Math.sin(v.psi);
            for (const side of [-1, 1]) {
                const x = v.x - cp * g[0] - sp * g[2] * side, z = v.y - sp * g[0] + cp * g[2] * side;
                for (let k = cnt(n); k > 0; k--) puff(x + rr(-0.3, 0.3), 0.3, z + rr(-0.3, 0.3), -sp * side * rr(0.5, 3) - cp * rr(0, 3), rr(0.5, 1.8), cp * side * rr(0.5, 3) - sp * rr(0, 3), prof.life * rr(0.8, 1.2), prof.s0, prof.s1 * 1.1, prof.c0, prof.c1, prof.a, 1.5, -0.3);
            }
        }
        function driftFlare(v, level) {
            const g = geoOf(v), c = TURBO[clamp(level - 1, 0, 2)], cp = Math.cos(v.psi), sp = Math.sin(v.psi);
            for (const side of [-1, 1]) {
                const x = v.x - cp * g[0] - sp * g[2] * side, z = v.y - sp * g[0] + cp * g[2] * side;
                glow(x, 0.35, z, 0, 0, 0, 0.22, 1.2, 3.2, WHITE, c, 0.95);
                for (let k = cnt(10); k > 0; k--) { const an = rnd() * TAU, s2 = rr(3, 9); spark(x, 0.3, z, Math.cos(an) * s2, rr(1, 5), Math.sin(an) * s2, rr(0.25, 0.5), 0.3, c, c); }
            }
        }
        function windStreaks(v, n) {
            const cp = Math.cos(v.psi), sp = Math.sin(v.psi);
            for (let k = cnt(n); k > 0; k--) {
                const side = rr(-2.6, 2.6), fwd = rr(-1, 4), sd = Math.abs(v.vx) * rr(0.5, 0.9);
                spawn(F.STREAK, 1, v.x + cp * fwd - sp * side, v.z + rr(0.2, 1.8), v.y + sp * fwd + cp * side, -cp * sd, 0, -sp * sd, rr(0.25, 0.45), 2.2, 2.2, WHITE, WHITE, 0.5, 0.2, 0, 1.5, 0, 0, 0);
            }
        }
        function embers(v, n) {
            for (let k = cnt(n); k > 0; k--) glow(v.x + rr(-1, 1), v.z + rr(0.3, 1.2), v.y + rr(-1, 1), rr(-2, 2), rr(1, 4), rr(-2, 2), rr(0.6, 1.2), 0.22, 0.05, FIRE, EMBER, 0.95, 0.4, F.DOT);
        }

        /* ---- continuous emission ------------------------------------------------------ */
        function carEmit(race, car, i, dt) {
            const v = car.v;
            if (v.out || !near(v.x, v.y, i === 0 ? 220 : 130)) return;
            const cs = carState(i), g = geoOf(v), prof = profOf(v), surf = v.surface || 'asphalt';
            const cp = Math.cos(v.psi), sp = Math.sin(v.psi), rx = -sp, rz = cp, speed = Math.abs(v.vx);
            const wvx = cp * v.vx + rx * v.vy, wvz = sp * v.vx + rz * v.vy, w = i === 0 ? 1 : 0.45;
            const gnd = !v.airborne;
            const slideK = gnd && speed > 7 ? clamp((v.slip - 2.2) / 7, 0, 1) : 0;
            const hb = gnd && v.handbrake && speed > 6 ? 0.65 : 0;
            const k = Math.max(slideK, hb);
            const brakeK = gnd && v.brake > 0.8 && speed > 22 ? 0.45 : 0;
            for (let n = 0; n < 2; n++) {
                const side = n ? 1 : -1, st = n ? cs.wr : cs.wl;
                const wx = v.x - cp * g[0] + rx * g[2] * side, wz = v.y - sp * g[0] + rz * g[2] * side;
                if (Math.max(k, brakeK) > 0.12) skidAdd(st, wx, wz, 0.3, (SKID[surf] || SKID.asphalt));
                else st.has = false;
                if (!gnd) continue;
                /* Tyre smoke / dust / spray, sized and lit by the surface. */
                const loose = prof.powder || prof.clods || prof.stones || surf === 'sand';
                let rate = 0;
                if (k > 0.05) rate = prof.rate * (0.3 + 0.7 * k);
                else if (loose && speed > 10) rate = prof.rate * 0.45 * clamp(speed / 40, 0.3, 1);
                else if (prof.spray && speed > 14) rate = prof.rate * 0.5 * clamp(speed / 45, 0.3, 1);
                rate *= dens * w;
                for (let c = N(rate * dt); c > 0; c--) {
                    puff(wx + rr(-0.25, 0.25) - wvx * rr(0, dt), 0.4, wz + rr(-0.25, 0.25) - wvz * rr(0, dt), wvx * 0.3 + rx * side * rr(0, prof.sp) + rr(-0.6, 0.6), rr(0.3, 1) * prof.up, wvz * 0.3 + rz * side * rr(0, prof.sp) + rr(-0.6, 0.6),
                        prof.life * rr(0.75, 1.25), prof.s0 * rr(0.8, 1.2), prof.s1 * rr(0.8, 1.2), prof.c0, prof.c1, prof.a * rr(0.8, 1), 1.3, -0.25 * prof.up);
                }
                const spray = (k > 0.2 || speed > 18) ? dens * w : 0;
                if (spray && prof.stones) for (let c = N(55 * k * spray * dt); c > 0; c--) bit(F.SHARD, wx, 0.2, wz, -cp * speed * 0.2 + rx * side * rr(1, 6), rr(3, 7), -sp * speed * 0.2 + rz * side * rr(1, 6), rr(0.5, 0.9), rr(0.1, 0.2), ROCK, ROCK, 17, K_BOUNCE, 0.1);
                if (spray && prof.clods) for (let c = N(40 * (k + 0.3) * spray * dt); c > 0; c--) bit(F.DROP, wx, 0.2, wz, -cp * speed * 0.25 + rx * side * rr(0, 4), rr(4, 9), -sp * speed * 0.25 + rz * side * rr(0, 4), rr(0.6, 1.0), rr(0.25, 0.45), prof.c0, prof.c1, 19, K_DIE, 0.1);
                if (spray && prof.spray) for (let c = N(90 * (speed / 40) * spray * dt); c > 0; c--) spawn(F.DROP, 0, wx, 0.25, wz, -cp * speed * 0.3 + rx * side * rr(1, 5), rr(2, 6), -sp * speed * 0.3 + rz * side * rr(1, 5), rr(0.4, 0.8), 0.28, 0.12, WHITE, prof.c1, 0.55, 0.2, 14, 1.4, 0, 0, K_DIE);
                if (spray && prof.powder) for (let c = N(30 * k * spray * dt); c > 0; c--) glow(wx, 0.5, wz, rr(-3, 3), rr(1, 4), rr(-3, 3), rr(0.5, 0.9), 0.14, 0.05, WHITE, ICE0, 0.95, 0.5, F.DOT);
                /* Turbo sparks at the rear wheels, in the colour of the charge level. */
                if (v.driftLevel > 0 && slideK > 0.08) {
                    const lv = clamp(v.driftLevel - 1, 0, 2);
                    for (let c = N((60 + 40 * lv) * dens * w * dt); c > 0; c--) {
                        const sd = rr(3, 9);
                        spark(wx - wvx * rr(0, dt), rr(0.1, 0.35), wz - wvz * rr(0, dt), wvx * 0.6 - cp * sd * 0.7 + rx * side * rr(1, 6), rr(1, 5), wvz * 0.6 - sp * sd * 0.7 + rz * side * rr(1, 6), rr(0.22, 0.5), rr(0.28, 0.45), TURBO[lv], TURBO_SOFT[lv]);
                    }
                    if (rnd() < 0.6) glow(wx, 0.3, wz, 0, 0, 0, 0.08, 0.9, 1.6, TURBO[lv], TURBO[lv], 0.55);
                }
            }
            if (i === 0 && v.driftLevel > 0 && slideK > 0.08 && gnd) addLight(v.x, 0.9, v.y, TURBO[clamp(v.driftLevel - 1, 0, 2)], 12, 1, 12, true);
            /* Nitro, overdrive and pad/turbo boost: thick flames at the exhausts with a heat flicker. */
            if ((v.boost || v.overdrive > 0 || v.burst > 0) && speed > 6) {
                const over = v.overdrive > 0, outer = over ? OVER_OUT : NITRO_OUT, core = over ? OVER_CORE : NITRO_CORE;
                const flick = 0.75 + 0.25 * Math.sin(time * 52 + i * 3);
                for (const side of [-0.5, 0.5]) {
                    const ex = v.x - cp * g[1] + rx * g[2] * side, ez = v.y - sp * g[1] + rz * g[2] * side, h = v.z + 0.5;
                    /* World-space blobs spawned along the car's path and carried with it, so the jet is one continuous, attached tongue. */
                    for (let c = N(150 * dens * w * dt * flick); c > 0; c--) {
                        const sd = rr(7, 13), bk = rr(0, dt);
                        glow(ex - wvx * bk + rr(-0.1, 0.1), h + rr(-0.12, 0.12), ez - wvz * bk + rr(-0.1, 0.1), wvx * 0.92 - cp * sd + rx * rr(-1, 1), rr(-0.3, 1), wvz * 0.92 - sp * sd + rz * rr(-1, 1), rr(0.22, 0.4), rr(1.8, 2.5), 0.3, outer, NITRO_END, 0.9 * flick, 0.6, F.FLAME);
                    }
                    for (let c = N(110 * dens * w * dt); c > 0; c--) { const bk = rr(0, dt); glow(ex - wvx * bk, h, ez - wvz * bk, wvx * 0.95 - cp * 5, 0, wvz * 0.95 - sp * 5, rr(0.1, 0.2), 0.9, 0.2, core, outer, 0.95 * flick, 0, F.FLAME); }
                    if (rnd() < 0.7 * dens * w) glow(ex - cp * 0.5, h, ez - sp * 0.5, wvx, 0, wvz, 0.07, 1.8, 3, core, core, 0.45 * flick);
                }
                if (i === 0) addLight(v.x - cp * (g[1] + 1.5), v.z + 0.8, v.y - sp * (g[1] + 1.5), outer, 22 + 12 * flick, 1, 14, true);
            }
            /* Wall scrape sparks, damage smoke, soot, frost. */
            if (v.impact > 0.35 && speed > 8) {
                const sm = AD.sampleTrack(race.track, v.x, v.y), sd = sm.offset >= 0 ? 1 : -1;
                for (let c = N(70 * v.impact * dens * dt); c > 0; c--) spark(v.x - sm.nx * sd * 0.9, rr(0.15, 0.7), v.y - sm.ny * sd * 0.9, wvx * rr(0.2, 0.6) - sm.nx * sd * rr(2, 6), rr(1, 6), wvz * rr(0.2, 0.6) - sm.ny * sd * rr(2, 6), rr(0.25, 0.6), rr(0.25, 0.4), HOT, FIRE);
            }
            if (v.damage > 62 && speed > 4) for (let c = N(14 * dens * w * dt); c > 0; c--) puff(v.x + cp * g[0] * 0.7, v.z + 0.9, v.y + sp * g[0] * 0.7, rr(-0.5, 0.5) - cp * 3, rr(1, 2.5), rr(-0.5, 0.5) - sp * 3, rr(1, 1.8), 0.6, 2.2, SOOT0, SOOT1, 0.55, 1, -0.3);
            if (v.toast > 0) for (let c = N(22 * dens * w * dt); c > 0; c--) puff(v.x + rr(-0.8, 0.8), v.z + 1.4, v.y + rr(-0.8, 0.8), rr(-0.8, 0.8) - cp * 2, rr(1.5, 3), rr(-0.8, 0.8) - sp * 2, rr(1.2, 2), 0.8, 2.8, SOOT0, SOOT1, 0.7, 1, -0.5);
            if (v.frozen > 0) for (let c = N(24 * dt); c > 0; c--) glow(v.x + rr(-1.4, 1.4), v.z + rr(0.2, 1.4), v.y + rr(-1.4, 1.4), 0, rr(0.3, 1), 0, rr(0.5, 0.9), 0.5, 0.1, ICE0, WHITE, 0.95, 0, F.STAR);
            if (car.flame > 0) {
                const nx = v.x + cp * (g[1] + 0.6), nz = v.y + sp * (g[1] + 0.6);
                for (let c = N(260 * dens * dt); c > 0; c--) {
                    const sd = rr(24, 36), bk = rr(0, dt);
                    glow(nx - wvx * bk, v.z + 0.7, nz - wvz * bk, cp * sd + wvx * 0.5 + rr(-2, 2) * rx, rr(-0.3, 2), sp * sd + wvz * 0.5 + rr(-2, 2) * rz, rr(0.3, 0.55), rr(1.2, 1.8), rr(3.6, 5), c % 6 ? FIRE : HOT, EMBER, 0.6, 1.5, F.FLAME);
                }
                for (let c = N(30 * dens * dt); c > 0; c--) puff(nx + cp * rr(4, 12), v.z + 1.2, nz + sp * rr(4, 12), rr(-1, 1), rr(1, 3), rr(-1, 1), rr(0.9, 1.5), 1.2, 3.4, SOOT0, SOOT1, 0.5, 1, -0.4);
                if (i === 0 || near(v.x, v.y, 80)) addLight(nx + cp * 5, v.z + 1, nz + sp * 5, FIRE, 40 + rnd() * 15, 1, 16, true);
            }
            if (i === 0 && v.slipstream > 0.3 && speed > 38) windStreaks(v, N(20 * dt));
        }
        function projectileEmit(race, dt) {
            for (const p of race.projectiles) {
                const cp = Math.cos(p.psi), sp = Math.sin(p.psi), h = (p.z || 0) + 0.8;
                switch (p.type) {
                    case 'rocket': {
                        const tx = p.x - cp * 1.3, tz = p.y - sp * 1.3;
                        for (let c = N(70 * dens * dt); c > 0; c--) glow(tx, h, tz, -cp * 6 + rr(-1, 1), rr(-0.5, 0.8), -sp * 6 + rr(-1, 1), rr(0.1, 0.2), rr(0.7, 1), 0.2, HOT, FIRE, 0.9, 1, F.FLAME);
                        for (let c = N(55 * dens * dt); c > 0; c--) puff(tx + rr(-0.15, 0.15), h, tz + rr(-0.15, 0.15), -cp * 2 + rr(-0.6, 0.6), rr(0.2, 1), -sp * 2 + rr(-0.6, 0.6), rr(1.0, 1.7), 0.5, 2.2, SMOKE0, SMOKE1, 0.55, 1.3, -0.15);
                        addLight(tx, h, tz, FIRE, 28, 1, 14, true);
                        break;
                    }
                    case 'bullet': if (rnd() < 0.7) glow(p.x - cp, h, p.y - sp, 0, 0, 0, 0.1, 0.4, 0.1, HOT, FIRE, 0.7); break;
                    case 'grenade':
                        for (let c = N(40 * dt); c > 0; c--) glow(p.x + rr(-0.3, 0.3), h + rr(-0.3, 0.3), p.y + rr(-0.3, 0.3), rr(-1, 1), rr(-1, 1), rr(-1, 1), rr(0.4, 0.8), 0.4, 0.05, ICE0, ICE1, 0.9, 0, F.STAR);
                        break;
                    case 'hole_shot': case 'disco_shot': {
                        for (let c = N(70 * dt); c > 0; c--) {
                            const cc = p.type === 'hole_shot' ? PURPLE : PARTY[(Math.random() * PARTY.length) | 0];
                            glow(p.x + rr(-0.4, 0.4), h + rr(-0.4, 0.4), p.y + rr(-0.4, 0.4), rr(-2, 2), rr(-1, 2), rr(-2, 2), rr(0.3, 0.7), 0.7, 0.1, cc, cc, 0.85, 0.5);
                        }
                        break;
                    }
                    case 'cow':
                        for (let c = N(30 * dt); c > 0; c--) puff(p.x, h, p.y, rr(-1, 1), rr(-0.5, 0.5), rr(-1, 1), 0.9, 0.6, 2, WHITE, SMOKE1, 0.45, 1, 0);
                        if (rnd() < 0.5) bit(F.STAR, p.x, h, p.y, rr(-2, 2), rr(0, 2), rr(-2, 2), 0.6, 0.45, GOLD, GOLD, 4, 0, 1, 1);
                        break;
                    case 'emp_charge':
                        if (rnd() < 14 * dt) ringBb(p.x, 1, p.y, 8, 1.2, 0.22, ELEC, 0.7);
                        for (let c = N(40 * dt); c > 0; c--) { const an = rnd() * TAU; glow(p.x + Math.cos(an) * 4, 1 + rr(-0.5, 1), p.y + Math.sin(an) * 4, -Math.cos(an) * 16, 0, -Math.sin(an) * 16, 0.22, 1.2, 0.6, ELEC, ELEC, 0.9, 0, F.BOLT); }
                        break;
                    case 'rail_charge':
                        for (let c = N(70 * dt); c > 0; c--) { const an = rnd() * TAU, d = rr(2, 4); glow(p.x + Math.cos(an) * d, 0.9 + rr(-1, 1), p.y + Math.sin(an) * d, -Math.cos(an) * d * 3.5, 0, -Math.sin(an) * d * 3.5, 0.28, 0.5, 0.1, PINK, ELEC, 0.9, 0); }
                        glow(p.x, 0.9, p.y, 0, 0, 0, 0.05, 1.2 + (1 - Math.min(1, p.ttl / 0.35)) * 2, 1.4, WHITE, ELEC, 0.7);
                        break;
                    default: break;
                }
            }
        }
        function hazardEmit(race, dt) {
            for (const hz of race.hazards) {
                if (!near(hz.x, hz.y, 260)) continue;
                if (hz.type === 'black_hole') {
                    const r = hz.r || 26;
                    for (let c = N(70 * dens * dt); c > 0; c--) {
                        const an = rnd() * TAU, d = r * rr(0.3, 1), tx = -Math.sin(an), tz = Math.cos(an), sd = rr(8, 16), inw = rr(5, 12);
                        const cc = rnd() < 0.5 ? PURPLE : PINK;
                        glow(hz.x + Math.cos(an) * d, rr(0.2, 2.8), hz.y + Math.sin(an) * d, tx * sd - Math.cos(an) * inw, 0, tz * sd - Math.sin(an) * inw, rr(0.5, 0.9), rr(1.2, 2.2), 0.2, cc, BLACK, 0.9, 0.3);
                    }
                    for (let c = N(9 * dt); c > 0; c--) spawn(F.PUFF, 0, hz.x + rr(-1, 1), 1.8, hz.y + rr(-1, 1), 0, 0, 0, 0.6, 7, 4, BLACK, BLACK, 0.55, 0, 0, 0, 0, rr(-1, 1), 0);
                    addLight(hz.x, 3, hz.y, PURPLE, 40, 1, 24, true);
                } else if (hz.type === 'disco') {
                    for (let c = N(22 * dt); c > 0; c--) { const an = rnd() * TAU, d = (hz.r || 22) * Math.sqrt(rnd()), cc = PARTY[(Math.random() * PARTY.length) | 0]; glow(hz.x + Math.cos(an) * d, rr(1.5, 8), hz.y + Math.sin(an) * d, rr(-1, 1), rr(-1, 1), rr(-1, 1), rr(0.6, 1.2), 0.8, 0.1, cc, cc, 0.95, 0.3, F.STAR); }
                } else if (hz.type === 'ice') {
                    for (let c = N(5 * dt); c > 0; c--) { const an = rnd() * TAU, d = (hz.r || 8) * Math.sqrt(rnd()); glow(hz.x + Math.cos(an) * d, 0.3, hz.y + Math.sin(an) * d, 0, rr(0.3, 1), 0, rr(0.6, 1.2), 0.45, 0.1, ICE0, WHITE, 0.9, 0, F.STAR); }
                }
            }
        }
        function beamsEmit(race) {
            beamN = 0;
            for (const b of race.beams) {
                if (b.type === 'rail') beam(0, b.x, 0.9, b.y, b.x2, 0.9, b.y2, 0.45, 0.45, ELEC, clamp(b.ttl / 0.25, 0, 1));
                else if (b.type === 'rail_aim') beam(0, b.x, 0.9, b.y, b.x2, 0.9, b.y2, 0.06, 0.06, RAIL_AIM, 0.6 + 0.4 * Math.sin(time * 50));
                else if (b.type === 'tether') {
                    const a = carOf(race, b.a), c = carOf(race, b.b);
                    if (a && c) beam(1, a.x, a.z + 0.9, a.y, c.x, c.z + 0.9, c.y, 0.12, 0.12, ROPE, 0.95);
                } else if (b.type === 'flame') {
                    const v = carOf(race, b.car);
                    if (v) {
                        const g = geoOf(v), cp = Math.cos(v.psi), sp = Math.sin(v.psi);
                        for (let f = 1; f <= 2; f++) beam(3, v.x + cp * g[1], v.z + 0.7, v.y + sp * g[1], v.x + cp * (g[1] + 14), v.z + 0.7, v.y + sp * (g[1] + 14), 0.2, 2.4, WHITE, 0.6, f);
                    }
                }
            }
            for (const p of race.projectiles) {
                if (p.type !== 'harpoon') continue;
                const o = carOf(race, p.owner);
                if (o) beam(1, o.x + Math.cos(o.psi) * 2, o.z + 0.9, o.y + Math.sin(o.psi) * 2, p.x, (p.z || 0) + 0.9, p.y, 0.1, 0.1, ROPE, 0.9);
            }
            for (let i = ufos.length - 1; i >= 0; i--) {
                const u = ufos[i], age = time - u.t0;
                if (age > u.dur) { ufos.splice(i, 1); continue; }
                const f = Math.min(1, age / 0.6) * Math.min(1, (u.dur - age) / 0.8);
                beam(2, u.x, 24, u.z, u.x, 0, u.z, 1.6, 7.5, UFO0, 0.55 * f);
                if (rnd() < 0.7 && near(u.x, u.z, 700)) glow(u.x + rr(-4, 4), 0.3, u.z + rr(-4, 4), 0, rr(4, 9), 0, rr(1, 2), 0.7, 0.2, UFO1, WHITE, 0.8, 0.2);
            }
            for (let i = beamN; i < beams.length; i++) beams[i].mesh.visible = false;
        }

        /* ---- frame update ------------------------------------------------------------------ */
        function stepParticles(dt) {
            const cap = S.cap, pa = pools[0], pb = pools[1];
            pa.n = 0; pb.n = 0;
            let live = 0;
            for (let k = 0; k < cap; k++) {
                const i = (S.cur - 1 - k + cap) % cap;
                const life = S.life[i];
                if (life <= 0) continue;
                const age = S.age[i] + dt;
                if (age >= life) { S.life[i] = 0; continue; }
                S.age[i] = age;
                const fl = S.fl[i], kind = fl >> 1;
                let x = S.x[i], y = S.y[i], z = S.z[i], vx = S.vx[i], vy = S.vy[i], vz = S.vz[i];
                if (dt > 0) {
                    const dr = Math.exp(-S.drag[i] * dt);
                    vx *= dr; vy = vy * dr - S.grav[i] * dt; vz *= dr;
                    if (kind === K_FLUTTER) { vx += Math.sin(age * 6 + i) * 5 * dt; vz += Math.cos(age * 5.3 + i * 1.7) * 5 * dt; }
                    x += vx * dt; y += vy * dt; z += vz * dt;
                    if (y < 0.03) {
                        if (kind === K_DIE) { S.life[i] = 0; continue; }
                        if (kind === K_BOUNCE && vy < 0) { y = 0.03; vy *= -0.4; vx *= 0.72; vz *= 0.72; } else if (vy < 0) { y = 0.03; vy = 0; vx *= 0.92; vz *= 0.92; }
                    }
                    S.x[i] = x; S.y[i] = y; S.z[i] = z; S.vx[i] = vx; S.vy[i] = vy; S.vz[i] = vz;
                    S.rot[i] += S.spin[i] * dt;
                }
                live++;
                const t = age / life, lit = S.lit[i];
                const size = S.s0[i] + (S.s1[i] - S.s0[i]) * (1 - (1 - t) * (1 - t));
                const pool = fl & 1 ? pb : pa, j = pool.n++, o4 = j * 4, o3 = j * 3, c3 = i * 3;
                const yy = lit > 0 && y < size * 0.3 ? size * 0.3 : y;
                const al = S.a[i] * Math.min(1, t * 9) * (lit > 0 ? Math.pow(1 - t, 1.5) : (1 - t));
                const pa4 = pool.pos.array, pc = pool.color.array, pm = pool.misc.array, pv = pool.vel.array;
                pa4[o4] = x; pa4[o4 + 1] = yy; pa4[o4 + 2] = z; pa4[o4 + 3] = size;
                pc[o4] = S.c0[c3] + (S.c1[c3] - S.c0[c3]) * t; pc[o4 + 1] = S.c0[c3 + 1] + (S.c1[c3 + 1] - S.c0[c3 + 1]) * t; pc[o4 + 2] = S.c0[c3 + 2] + (S.c1[c3 + 2] - S.c0[c3 + 2]) * t; pc[o4 + 3] = al;
                pm[o4] = S.rot[i]; pm[o4 + 1] = S.fr[i]; pm[o4 + 2] = S.str[i]; pm[o4 + 3] = lit;
                pv[o3] = vx; pv[o3 + 1] = vy; pv[o3 + 2] = vz;
            }
            S.live = live;
            for (const p of pools) {
                p.geo.instanceCount = p.n;
                p.mesh.visible = p.n > 0;
                if (p.n > 0) for (const at of [p.pos, p.color, p.misc, p.vel]) { at.clearUpdateRanges(); at.addUpdateRange(0, p.n * at.itemSize); at.needsUpdate = true; }
            }
        }
        function stepRings(dt) {
            for (const r of rings) {
                if (!r.on) continue;
                r.age += dt;
                const t = r.age / r.life;
                if (t >= 1) { r.on = false; r.mesh.visible = false; continue; }
                const R = r.r0 + (r.r1 - r.r0) * (1 - (1 - t) * (1 - t));
                r.mesh.scale.setScalar(Math.max(0.01, R * 2.5));
                r.mat.uniforms.uAlpha.value = r.a * Math.pow(1 - t, 1.3);
            }
        }

        const api = {
            event: event,
            emit(race, dt) {
                if (!race || !S) return;
                use(race);
                dt = Math.max(0, dt || 0);
                if (race.state !== 'finished' && race.state !== 'running' && race.state !== 'countdown') return;
                if (dt > 0) {
                    for (let i = 0; i < race.cars.length; i++) carEmit(race, race.cars[i], i, dt);
                    projectileEmit(race, dt);
                    hazardEmit(race, dt);
                }
                beamsEmit(race);
                flushSkid();
            },
            update(dt, pose) {
                if (!S) return;
                if (pose && pose.eye) { eyeX = pose.eye[0]; eyeZ = pose.eye[2]; eyeSet = true; }
                time += dt;
                if (scene.fog) { U.uFog.value.copy(scene.fog.color); U.uFogR.value.set(scene.fog.near, scene.fog.far); }
                for (let i = timers.length - 1; i >= 0; i--) { timers[i].t -= dt; if (timers[i].t <= 0) { const fn = timers[i].fn; timers.splice(i, 1); fn(); } }
                stepParticles(dt);
                stepRings(dt);
                for (const b of beams) b.mat.uniforms.uTime.value = time;
                applyLights(dt);
                flushSkid();
            },
            setQuality(next) {
                const grew = next.particles !== gl.particles, skidChanged = next.skid !== gl.skid;
                gl = next;
                if (grew) buildPools();
                if (skidChanged) buildSkid();
                for (const l of LT) l.on = false;
            },
            stats() {
                return { particles: S ? S.live : 0, capacity: S ? S.cap : 0, skid: skid ? (skid.full ? skid.cap : skid.filled) : 0, beams: beamN, rings: rings.filter(r => r.on).length, lights: LT.filter(l => l.on).length };
            },
            dispose() {
                for (const l of LT) l.on = false;
                const pool = scene.userData.lightPool;
                if (pool) for (const l of pool) l.intensity = 0;
                for (const p of pools) { p.geo.dispose(); p.mat.dispose(); }
                skid.geo.dispose(); skid.mat.dispose();
                for (const r of rings) r.mat.dispose();
                for (const b of beams) { b.mesh.geometry.dispose(); b.mat.dispose(); }
                ringGeo.dispose(); atlas.dispose();
                root.removeFromParent();
                S = null; pools = []; timers.length = 0; ufos.length = 0;
            }
        };
        buildPools();
        buildSkid();
        return api;
    };
})();
