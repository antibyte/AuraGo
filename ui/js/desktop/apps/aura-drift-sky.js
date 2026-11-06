/* Aura Drift - 3D sky: gradient dome with sun or moon and lit clouds, stars, aurora, neon haze, a far
 * horizon silhouette ring, harbour water, the salt mirror and GPU weather streaks.
 *
 * AD.createSky(T, scene, look, q, track) -> { update(dt, pose, conditions), setDisco(on), setReduced(on),
 *   setQuality(q), emitters(list), dispose() }. Three space is [x, up, y_sim]. All noise comes from one
 * tileable texture (texture lookups only, no integer hashing in GLSL). When the terrain or road module is
 * missing a plain ground and ribbon stand in so the scene is never empty. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    /* Per biome sky settings. lit/dark colour the cloud sunlit and shaded side, neon* tint the horizon haze. */
    const SKY = {
        alpine: { cover: 0.50, lit: '#fff1d6', dark: '#7f84b0', stars: 0, sun: 0.034, haze: 0 },
        desert: { cover: 0.60, lit: '#ffd9a8', dark: '#a0627a', stars: 0, sun: 0.036, haze: 0.0 },
        harbour: { cover: 0.43, lit: '#ff9fb4', dark: '#262c5a', stars: 700, sun: 0.04, haze: 0.62, na: '#ff4fa3', nb: '#2de2ff', nc: '#ffb347' },
        salt: { cover: 0.60, lit: '#fff6ec', dark: '#b6a6c8', stars: 0, sun: 0.034, haze: 0 },
        forest: { cover: 0.40, lit: '#fff0c4', dark: '#7e9486', stars: 0, sun: 0.04, haze: 0 },
        industry: { cover: 0.38, lit: '#ff9a58', dark: '#3d2c3a', stars: 280, sun: 0.04, haze: 0.5, na: '#ff7a2a', nb: '#ffb347', nc: '#ff3a1a' },
        glacier: { cover: 0.70, lit: '#7f9cc4', dark: '#14213c', stars: 1300, sun: 0.045, haze: 0, aurora: 1 },
        metro: { cover: 0.46, lit: '#c860e0', dark: '#241a4a', stars: 380, sun: 0.04, haze: 0.95, na: '#ff3ec8', nb: '#5a7bff', nc: '#31f5ff' }
    };

    function seeded(seed) {
        let s = (seed >>> 0) || 1;
        return () => { s = (Math.imul(s, 1664525) + 1013904223) >>> 0; return s / 4294967296; };
    }

    /* ---- tileable value noise (CPU, cached across skies) ----------------- */
    let noiseCache = null;
    function lattice(rnd, p) { const a = new Float32Array(p * p); for (let i = 0; i < a.length; i++) a[i] = rnd(); return a; }
    function fbm(n, periods, seed, ridged) {
        const rnd = seeded(seed);
        const lats = periods.map(p => lattice(rnd, p));
        const out = new Float32Array(n * n);
        let lo = 1e9, hi = -1e9;
        for (let y = 0; y < n; y++) {
            for (let x = 0; x < n; x++) {
                let v = 0, amp = 1, tot = 0;
                for (let o = 0; o < periods.length; o++) {
                    const p = periods[o], l = lats[o];
                    const fx = x / n * p, fy = y / n * p;
                    const x0 = Math.floor(fx), y0 = Math.floor(fy);
                    let tx = fx - x0, ty = fy - y0;
                    tx = tx * tx * (3 - 2 * tx); ty = ty * ty * (3 - 2 * ty);
                    const x1 = (x0 + 1) % p, y1 = (y0 + 1) % p, xa = x0 % p, ya = y0 % p;
                    let s = (l[ya * p + xa] * (1 - tx) + l[ya * p + x1] * tx) * (1 - ty) + (l[y1 * p + xa] * (1 - tx) + l[y1 * p + x1] * tx) * ty;
                    if (ridged) s = 1 - Math.abs(2 * s - 1);
                    v += s * amp; tot += amp; amp *= 0.5;
                }
                v /= tot;
                out[y * n + x] = v;
                if (v < lo) lo = v;
                if (v > hi) hi = v;
            }
        }
        const k = 1 / Math.max(1e-6, hi - lo);
        for (let i = 0; i < out.length; i++) out[i] = (out[i] - lo) * k;
        return out;
    }
    function noiseData() {
        if (noiseCache) return noiseCache;
        const N = 512, M = 256;
        const r = fbm(N, [3, 6, 12, 24, 48, 96], 1101, false), g = fbm(N, [4, 8, 16, 32, 64], 2202, false);
        const b = fbm(N, [4, 8, 16, 32], 3303, true), a = fbm(N, [16, 64, 128], 4404, false);
        const rgba = new Uint8Array(N * N * 4);
        for (let i = 0; i < N * N; i++) {
            rgba[i * 4] = r[i] * 255; rgba[i * 4 + 1] = g[i] * 255; rgba[i * 4 + 2] = b[i] * 255; rgba[i * 4 + 3] = a[i] * 255;
        }
        const h = fbm(M, [4, 8, 16, 32, 64], 5505, false);
        const nrm = new Uint8Array(M * M * 4);
        for (let y = 0; y < M; y++) {
            for (let x = 0; x < M; x++) {
                const dx = h[y * M + (x + 1) % M] - h[y * M + (x + M - 1) % M], dy = h[((y + 1) % M) * M + x] - h[((y + M - 1) % M) * M + x];
                const i = (y * M + x) * 4;
                nrm[i] = clamp(128 + dx * 900, 0, 255); nrm[i + 1] = clamp(128 + dy * 900, 0, 255); nrm[i + 2] = h[y * M + x] * 255; nrm[i + 3] = 255;
            }
        }
        noiseCache = { N: N, rgba: rgba, M: M, nrm: nrm };
        return noiseCache;
    }

    /* ---- GLSL ----------------------------------------------------------------- */
    const COMMON = [
        'uniform sampler2D uNoise;',
        'uniform float uTime, uCover, uNight, uAurora, uHaze, uSunBoost, uSunSize, uFogMix;',
        'uniform vec3 uTop, uMid, uHorizon, uFog, uGround, uSun, uSunDir, uCloudLit, uCloudDark, uNeonA, uNeonB, uNeonC;',
        'vec3 skyGradient(vec3 d) {',
        '  float h = d.y; float up = clamp(h, 0.0, 1.0);',
        '  vec3 col = mix(uHorizon, uMid, smoothstep(0.0, 0.26, up));',
        '  col = mix(col, uTop, smoothstep(0.10, 0.9, up));',
        '  col = mix(col, uGround, smoothstep(0.0, 0.35, -h));',
        '  vec2 az = normalize(d.xz + vec2(1e-5)); vec2 sz = normalize(uSunDir.xz + vec2(1e-5));',
        '  float facing = max(dot(az, sz), 0.0); float hp = max(h, 0.0);',
        '  col += uSun * (pow(facing, 3.0) * exp(-hp * 5.0) * 0.5 + pow(facing, 16.0) * exp(-hp * 9.0) * 0.7) * uSunBoost;',
        '  return col;',
        '}',
        'vec3 sunLight(vec3 d) {',
        '  float sd = clamp(dot(d, uSunDir), -1.0, 1.0); if (sd < 0.0) return vec3(0.0);',
        '  float ang = acos(sd);',
        '  float disc = 1.0 - smoothstep(uSunSize * 0.9, uSunSize, ang);',
        '  float halo = exp(-ang * 9.0) * 0.45 + exp(-ang * 34.0) * 0.9;',
        '  float body = disc * (uNight > 0.5 ? 2.6 : 9.0);',
        '  if (uNight > 0.5 && disc > 0.0) {',
        '    vec3 t = normalize(cross(uSunDir, vec3(0.0, 1.0, 0.0))); vec3 b = cross(t, uSunDir);',
        '    vec2 loc = vec2(dot(d, t), dot(d, b)) / uSunSize;',
        '    body *= mix(0.55, 1.0, smoothstep(0.3, 0.7, texture2D(uNoise, loc * 0.22 + 0.5).b));',
        '  }',
        '  return uSun * (body + halo) * uSunBoost;',
        '}',
        'vec3 auroraBands(vec3 d) {',
        '  float h = d.y; if (h < 0.015) return vec3(0.0);',
        '  float u = atan(d.z, d.x) * 0.15915494; vec3 acc = vec3(0.0);',
        '  for (int i = 0; i < 3; i++) {',
        '    float fi = float(i); float drift = uTime * (0.004 + fi * 0.0023);',
        '    float bn = texture2D(uNoise, vec2(u * (2.0 + fi) + drift + fi * 0.37, 0.13 + fi * 0.31)).r;',
        '    float yc = 0.045 + fi * 0.085 + 0.2 * bn; float dy = h - yc;',
        '    float lower = smoothstep(-0.012, 0.012, dy); float upper = exp(-max(dy, 0.0) * (3.6 + fi * 0.9));',
        '    float rays = texture2D(uNoise, vec2(u * (24.0 + fi * 12.0) + fi * 0.5 + drift * 3.0, dy * 0.4 + fi * 0.2)).a;',
        '    rays = pow(rays, 1.7) * 1.7;',
        '    float gap = smoothstep(0.3, 0.64, texture2D(uNoise, vec2(u * (3.0 + fi) - drift * 2.0, 0.55 + fi * 0.2)).b);',
        '    float inten = lower * upper * (0.12 + rays) * (0.2 + 1.1 * gap);',
        '    vec3 c = mix(vec3(0.5, 1.0, 0.3), vec3(0.08, 1.0, 0.42), smoothstep(0.0, 0.05, dy));',
        '    c = mix(c, vec3(0.06, 0.7, 0.95), smoothstep(0.08, 0.24, dy));',
        '    c = mix(c, vec3(0.72, 0.2, 1.0), smoothstep(0.2, 0.5, dy));',
        '    acc += c * inten * (1.15 - 0.25 * fi) + vec3(0.35, 1.0, 0.6) * lower * exp(-abs(dy) * 80.0) * 0.35 * gap;',
        '  }',
        '  return acc * smoothstep(0.015, 0.1, h);',
        '}',
        'vec3 neonHaze(vec3 d) {',
        '  float h = max(d.y, 0.0); float az = atan(d.z, d.x) * 0.15915494;',
        '  float n1 = texture2D(uNoise, vec2(az * 4.0 + uTime * 0.003, 0.2)).r;',
        '  float n2 = texture2D(uNoise, vec2(az * 6.0 - uTime * 0.002, 0.7)).g;',
        '  vec3 c = mix(uNeonA, uNeonB, smoothstep(0.35, 0.65, n2));',
        '  c = mix(c, uNeonC, smoothstep(0.55, 0.85, n1) * 0.7);',
        '  return c * (exp(-h * 7.0) * 0.8 + exp(-h * 2.4) * 0.22) * (0.55 + 0.9 * n1) * uHaze;',
        '}',
        'vec4 cloudLayer(vec3 d) {',
        '  if (d.y < 0.004) return vec4(0.0);',
        '  vec2 uv = d.xz / (d.y + 0.16) * 0.5 + vec2(uTime * 0.0035, uTime * 0.0014);',
        '  float dens = texture2D(uNoise, uv * 0.33).r * 0.62 + texture2D(uNoise, uv * 0.93 + 0.31).g * 0.38;',
        '  vec2 sdir = normalize(uSunDir.xz + vec2(1e-5)) * 0.045;',
        '  float densS = texture2D(uNoise, (uv + sdir) * 0.33).r * 0.62 + texture2D(uNoise, (uv + sdir) * 0.93 + 0.31).g * 0.38;',
        '  float cov = smoothstep(uCover, uCover + 0.2, dens);',
        '  float shade = clamp(0.62 + (dens - densS) * 4.5, 0.0, 1.0);',
        '  float sd = max(dot(d, uSunDir), 0.0);',
        '  vec3 col = mix(uCloudDark, uCloudLit, shade);',
        '  float rim = (1.0 - cov) * smoothstep(uCover - 0.04, uCover + 0.05, dens);',
        '  col += uSun * (rim * (0.45 + pow(sd, 5.0) * 2.2)) * uSunBoost;',
        '  float fade = smoothstep(0.0, 0.14, d.y) * (0.92 - 0.25 * smoothstep(0.5, 1.0, d.y));',
        '  return vec4(col, cov * fade);',
        '}',
        'vec3 skyColor(vec3 d) {',
        '  vec3 col = skyGradient(d);',
        '  if (uAurora > 0.0) col += auroraBands(d) * uAurora;',
        '  if (uHaze > 0.0) col += neonHaze(d);',
        '  vec4 cl = cloudLayer(d);',
        '  col = mix(col, cl.rgb, cl.a * (uAurora > 0.0 ? 0.55 : 1.0));',
        '  col += sunLight(d) * (1.0 - cl.a * 0.55);',
        '  col = mix(col, uFog, exp(-abs(d.y) * 15.0) * uFogMix);',
        '  return col;',
        '}'
    ].join('\n');
    const FINISH = '\n#include <tonemapping_fragment>\n#include <colorspace_fragment>\n';
    const DOME_V = 'varying vec3 vDir;\nvoid main() { vDir = normalize(position); gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }';
    const DOME_F = COMMON + '\nvarying vec3 vDir;\nvoid main() {\n  gl_FragColor = vec4(skyColor(normalize(vDir)), 1.0);' + FINISH + '}';

    /* Water (harbour) and salt mirror share one shader: wave normals from two scrolling lookups, the sky
       reflected, fresnel, sun glitter and vertical streaks for up to 12 glowing emitters. */
    const MIRROR_V = 'varying vec3 vWorld;\nvoid main() { vec4 w = modelMatrix * vec4(position, 1.0); vWorld = w.xyz; gl_Position = projectionMatrix * viewMatrix * w; }';
    const MIRROR_F = COMMON + '\n' + [
        'uniform sampler2D uWave; uniform vec3 uDeep, uShallow; uniform float uWaveAmp, uBase, uFogNear, uFogFar, uCrack, uGloss, uWaterY;',
        'uniform vec4 uEmit[12]; uniform vec3 uEmitCol[12];',
        'varying vec3 vWorld;',
        'void main() {',
        '  vec2 p = vWorld.xz;',
        '  vec4 w1 = texture2D(uWave, p * 0.05 + vec2(uTime * 0.021, uTime * 0.013));',
        '  vec4 w2 = texture2D(uWave, p * 0.13 + vec2(-uTime * 0.03, uTime * 0.019));',
        '  vec2 g = (w1.xy + w2.xy - 1.0) * uWaveAmp;',
        '  vec3 n = normalize(vec3(g.x, 1.0, g.y));',
        '  vec3 V = cameraPosition - vWorld; float dist = length(V); V /= dist;',
        '  float ndv = clamp(dot(n, V), 0.0, 1.0);',
        '  float F = uBase + (1.0 - uBase) * pow(1.0 - ndv, 4.0);',
        '  vec3 R = reflect(-V, n); R.y = abs(R.y) + 0.002;',
        '  vec3 refl = skyColor(normalize(R));',
        '  float sd = max(dot(normalize(R), uSunDir), 0.0);',
        '  float sparkle = smoothstep(0.55, 0.95, texture2D(uNoise, p * 0.31 + uTime * 0.02).a);',
        '  refl += uSun * (pow(sd, 380.0) * 22.0 * (0.4 + sparkle) + pow(sd, 40.0) * 0.5) * uSunBoost;',
        '  vec3 col = mix(mix(uDeep, uShallow, w1.z * 0.5), refl, F);',
        '  vec2 toP = vWorld.xz - cameraPosition.xz; float dP = length(toP);',
        '  for (int i = 0; i < 12; i++) {',
        '    vec4 e = uEmit[i]; if (e.w <= 0.0) continue;',
        '    vec2 toE = e.xz - cameraPosition.xz; float dE = length(toE);',
        '    float cs = dot(toP, toE) / max(dP * dE, 1e-3);',
        '    float lat = sqrt(max(1.0 - cs * cs, 0.0)) * dP;',
        '    float spread = 0.35 + 0.7 * (g.x + g.y) * 0.5 + e.w * 0.05;',
        '    float cam = max(cameraPosition.y - uWaterY, 1.0);',
        '    float near = dE * cam / (cam + e.y);',
        '    float along = smoothstep(near - 6.0, near + 4.0, dP) * (1.0 - smoothstep(dE - 2.0, dE + 8.0, dP));',
        '    float ripple = 0.55 + 0.45 * texture2D(uNoise, vec2(p.x * 0.02 + p.y * 0.17, uTime * 0.05 + float(i))).r;',
        '    col += uEmitCol[i] * exp(-lat * lat / (spread * spread + 0.0001)) * along * ripple * step(0.0, cs) * 1.7;',
        '  }',
        '  float fogK = smoothstep(uFogNear, uFogFar, dist);',
        '  float alpha = 1.0;',
        '  if (uCrack > 0.0) {',
        '    float cell = texture2D(uNoise, p * 0.012).b; float cr = smoothstep(0.78, 0.9, cell);',
        '    col = mix(col, col * 0.78 + uShallow * 0.3, cr * 0.5 * (1.0 - fogK));',
        '    alpha = clamp(F * 1.1 + 0.28 + uGloss, 0.0, 0.96);',
        '  }',
        '  col = mix(col, uFog, fogK);',
        '  gl_FragColor = vec4(col, alpha);'
    ].join('\n') + FINISH + '}';

    /* GPU weather: every particle is a camera facing quad stretched along its apparent velocity. */
    const WX_V = [
        'uniform float uTime, uFlutter, uLenK; uniform vec3 uFall, uBox, uCamVel; uniform vec2 uSize;',
        'attribute vec4 aSeed; varying vec2 vUv; varying float vFade;',
        'void main() {',
        '  vec3 vel = uFall * (0.8 + 0.4 * aSeed.w);',
        '  vec3 origin = cameraPosition - uBox * 0.5;',
        '  vec3 p = origin + mod(aSeed.xyz * uBox - vel * uTime - origin, uBox);',
        '  p.x += sin(uTime * 1.3 + aSeed.w * 40.0) * uFlutter; p.z += cos(uTime * 1.1 + aSeed.w * 31.0) * uFlutter;',
        '  vec3 rel = vel - uCamVel; float sp = length(rel); vec3 ax = rel / max(sp, 0.001);',
        '  vec3 toCam = cameraPosition - p; float d = length(toCam);',
        '  vec3 side = normalize(cross(ax, toCam / max(d, 0.001)) + vec3(1e-5));',
        '  float len = clamp(sp * uLenK, 0.05, 5.0);',
        '  vec3 world = p + ax * position.y * len + side * position.x * uSize.x * (0.6 + 0.8 * aSeed.w);',
        '  vUv = position.xy + 0.5;',
        '  vFade = smoothstep(uBox.x * 0.5, uBox.x * 0.28, d) * smoothstep(0.6, 3.0, d);',
        '  gl_Position = projectionMatrix * viewMatrix * vec4(world, 1.0);',
        '}'
    ].join('\n');
    const WX_F = [
        'uniform vec3 uColor; uniform float uAlpha, uRound, uTime;',
        'varying vec2 vUv; varying float vFade;',
        'void main() {',
        '  vec2 c = vUv * 2.0 - 1.0;',
        '  float a = uRound > 0.5 ? 1.0 - smoothstep(0.35, 1.0, length(c)) : (1.0 - smoothstep(0.2, 1.0, abs(c.x))) * (1.0 - 0.6 * abs(c.y));',
        '  gl_FragColor = vec4(uColor, a * uAlpha * vFade);',
        '}'
    ].join('\n');
    const WEATHER = {
        rain: { max: 1500, fall: [-1.2, -17, 0.6], box: [36, 24, 36], size: [0.05, 0], lenK: 0.06, flutter: 0, color: null, alpha: 0.6, round: 0, additive: false },
        snow: { max: 1000, fall: [1.1, -1.7, 0.6], box: [30, 18, 30], size: [0.22, 0], lenK: 0.012, flutter: 0.7, color: '#ffffff', alpha: 0.95, round: 1, additive: false },
        sand: { max: 400, fall: [-11, -0.6, 3], box: [40, 12, 40], size: [0.06, 0], lenK: 0.05, flutter: 0.2, color: null, alpha: 0.5, round: 0, additive: false },
        sparks: { max: 400, fall: [0.5, 3.2, 0.4], box: [60, 26, 60], size: [0.12, 0], lenK: 0.02, flutter: 1.1, color: '#ff9a3a', alpha: 1.0, round: 1, additive: true },
        motes: { max: 400, fall: [0.4, -0.15, 0.2], box: [50, 16, 50], size: [0.09, 0], lenK: 0.006, flutter: 1.4, color: '#fff6e0', alpha: 0.9, round: 1, additive: true }
    };

    /* ---- helpers ----------------------------------------------------------- */
    function rgb(hex) { const n = parseInt(hex.slice(1), 16); return [(n >> 16) & 255, (n >> 8) & 255, n & 255]; }
    function mixHex(a, b, t) { const A = rgb(a), B = rgb(b); return 'rgb(' + A.map((v, i) => Math.round(v + (B[i] - v) * t)).join(',') + ')'; }
    function shade(hex, k) { const A = rgb(hex); return 'rgb(' + A.map(v => Math.round(clamp(v * k, 0, 255))).join(',') + ')'; }

    /* Periodic 1D noise so the silhouette closes around the ring. */
    function ridge1D(rnd, W, periods, ridged) {
        const lats = periods.map(p => { const a = new Float32Array(p); for (let i = 0; i < p; i++) a[i] = rnd(); return a; });
        return x => {
            let v = 0, amp = 1, tot = 0;
            for (let o = 0; o < periods.length; o++) {
                const p = periods[o], f = x / W * p, i0 = Math.floor(f), t0 = f - i0, t = t0 * t0 * (3 - 2 * t0);
                let s = lats[o][i0 % p] * (1 - t) + lats[o][(i0 + 1) % p] * t;
                if (ridged) s = 1 - Math.abs(2 * s - 1);
                v += s * amp; tot += amp; amp *= 0.5;
            }
            return v / tot;
        };
    }
    function drawRing(biomeId, look) {
        const W = 2048, H = 256;
        const c = document.createElement('canvas');
        c.width = W; c.height = H;
        const g = c.getContext('2d');
        const rnd = seeded(AD.hashString('ring/' + biomeId));
        const night = !!look.night;
        const farC = night ? mixHex(look.fog, '#000000', 0.2) : mixHex(look.fog, look.skyTop, 0.22);
        const midC = night ? mixHex(look.fog, '#000000', 0.45) : mixHex(look.fog, look.skyTop, 0.1);
        const nearC = night ? mixHex(look.fog, '#000000', 0.62) : shade(look.fog, 0.62);
        const baseY = H - 18;
        const poly = (fn, color, snow) => {
            g.beginPath(); g.moveTo(0, H);
            for (let x = 0; x <= W; x += 4) g.lineTo(x, fn(x));
            g.lineTo(W, H); g.closePath();
            g.fillStyle = color; g.fill();
            if (snow) {
                g.save(); g.globalCompositeOperation = 'source-atop';
                const gr = g.createLinearGradient(0, 0, 0, H);
                gr.addColorStop(0, snow); gr.addColorStop(0.5, 'rgba(255,255,255,0)');
                g.fillStyle = gr; g.fillRect(0, 0, W, H); g.restore();
            }
        };
        const mountains = (color, amp, base, periods, ridged, snow) => {
            const f = ridge1D(rnd, W, periods, ridged);
            poly(x => base - amp * Math.pow(f(x), 1.25), color, snow);
        };
        const skyline = (color, minH, maxH, wMin, wMax, lights, density) => {
            let x = 0;
            while (x < W) {
                const w = wMin + rnd() * (wMax - wMin);
                const h = minH + Math.pow(rnd(), 1.6) * (maxH - minH);
                for (const ox of [0, -W]) {
                    g.fillStyle = color; g.fillRect(x + ox, baseY - h, w, h + 20);
                    if (rnd() < 0.3) g.fillRect(x + ox + w * 0.3, baseY - h - 5 - rnd() * 14, w * 0.4, 14);
                    if (lights && w > 6) {
                        for (let yy = baseY - h + 4; yy < baseY - 3; yy += 5) {
                            for (let xx = x + 2; xx < x + w - 2; xx += 4) {
                                if (rnd() > density) continue;
                                g.fillStyle = lights[Math.floor(rnd() * lights.length)];
                                g.fillRect(xx + ox, yy, 2, 3);
                            }
                        }
                    }
                }
                x += w + rnd() * 3;
            }
        };
        const win = ['#ffd9a0', '#ffc070', '#9fe8ff', '#ff8ad0', '#fff2cc'];
        if (biomeId === 'alpine') {
            mountains(farC, 150, baseY - 20, [3, 7, 14, 28], true, 'rgba(255,244,230,0.92)');
            mountains(midC, 105, baseY, [4, 9, 19, 38], true, 'rgba(235,240,255,0.7)');
            mountains(nearC, 55, baseY + 6, [6, 13, 29], false, null);
        } else if (biomeId === 'glacier') {
            mountains(farC, 170, baseY - 20, [3, 8, 16, 32], true, 'rgba(190,225,255,0.95)');
            mountains(midC, 120, baseY, [5, 11, 21, 41], true, 'rgba(160,210,255,0.8)');
            mountains(nearC, 60, baseY + 6, [7, 15, 31], true, 'rgba(120,180,240,0.4)');
        } else if (biomeId === 'desert') {
            const mesa = (color, amp, base, n, q) => {
                const f = ridge1D(rnd, W, [n, n * 2, n * 5], false);
                poly(x => base - Math.round(amp * Math.pow(f(x), 1.2) / q) * q, color, null);
            };
            mesa(farC, 120, baseY - 10, 3, 24);
            mesa(midC, 90, baseY, 5, 18);
            mesa(nearC, 40, baseY + 8, 8, 10);
        } else if (biomeId === 'salt') {
            mountains(mixHex(look.fog, '#9a8ab8', 0.35), 85, baseY - 6, [4, 9, 18], true, 'rgba(255,255,255,0.35)');
            mountains(mixHex(look.fog, '#7a6a9a', 0.3), 45, baseY + 4, [6, 14, 30], true, null);
        } else if (biomeId === 'forest') {
            mountains(farC, 70, baseY - 6, [3, 7, 15], false, null);
            mountains(midC, 40, baseY + 2, [5, 12, 26], false, null);
            const f = ridge1D(rnd, W, [40, 90], false);
            poly(x => baseY + 6 - 26 * f(x) - 12 * Math.abs(Math.sin(x * 0.55 + f(x) * 9)), nearC, null);
        } else if (biomeId === 'harbour') {
            mountains(farC, 60, baseY - 6, [3, 8, 17], false, null);
            skyline(midC, 14, 55, 6, 20, win, 0.35);
            skyline(nearC, 8, 28, 5, 14, win, 0.5);
            g.fillStyle = nearC;
            for (let x = 60; x < W; x += 300 + rnd() * 380) { g.fillRect(x, baseY - 70, 3, 70); g.fillRect(x - 14, baseY - 70, 30, 3); g.fillRect(x + 14, baseY - 70, 2, 24); }
        } else if (biomeId === 'metro') {
            skyline(farC, 40, 150, 10, 26, win, 0.3);
            skyline(midC, 30, 120, 12, 30, win, 0.42);
            skyline(nearC, 14, 70, 14, 34, win, 0.5);
            for (let i = 0; i < 26; i++) { g.fillStyle = '#ff2a1a'; g.fillRect(rnd() * W, baseY - 140 - rnd() * 40, 2, 2); }
        } else if (biomeId === 'industry') {
            skyline(midC, 10, 40, 14, 38, ['#ffb060', '#ffd9a0'], 0.12);
            const smoke = (x, h, w) => {
                g.fillStyle = farC; g.fillRect(x, baseY - h, w, h + 20);
                g.fillStyle = nearC; g.fillRect(x, baseY - h, w, 4);
            };
            for (let x = 80; x < W; x += 140 + rnd() * 260) smoke(x, 70 + rnd() * 90, 7 + rnd() * 6);
            skyline(nearC, 8, 30, 20, 50, ['#ffb060'], 0.06);
            for (let k = 0; k < 6; k++) {
                const x = 160 + k * 340 + rnd() * 100, y = baseY - 100 - rnd() * 40;
                const gr = g.createRadialGradient(x, y, 1, x, y, 38);
                gr.addColorStop(0, 'rgba(255,200,100,0.95)'); gr.addColorStop(0.3, 'rgba(255,110,30,0.5)'); gr.addColorStop(1, 'rgba(255,60,0,0)');
                g.fillStyle = gr; g.fillRect(x - 38, y - 38, 76, 76);
                g.fillStyle = nearC; g.fillRect(x - 1, y + 4, 3, baseY - y);
            }
        }
        /* The foot dissolves into the horizon haze. */
        g.save(); g.globalCompositeOperation = 'destination-out';
        const fade = g.createLinearGradient(0, H - 52, 0, H);
        fade.addColorStop(0, 'rgba(0,0,0,0)'); fade.addColorStop(1, 'rgba(0,0,0,0.92)');
        g.fillStyle = fade; g.fillRect(0, H - 52, W, 52);
        g.restore();
        return c;
    }

    /* ---- the sky ------------------------------------------------------------------ */
    AD.createSky = function (T, scene, look, q, track) {
        const biomeId = track && track.biomeId ? track.biomeId : 'alpine';
        const cfg = SKY[biomeId] || SKY.alpine;
        const night = !!look.night;
        let reduced = !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
        let glq = q;
        const col = hex => new T.Color(hex);
        const disposables = [];
        const own = o => { disposables.push(o); return o; };
        const root = new T.Group();
        root.name = 'sky';
        scene.add(root);

        const nd = noiseData();
        const mkTex = (data, n) => {
            const tex = new T.DataTexture(data, n, n, T.RGBAFormat);
            tex.wrapS = tex.wrapT = T.RepeatWrapping;
            tex.magFilter = T.LinearFilter; tex.minFilter = T.LinearMipmapLinearFilter;
            tex.generateMipmaps = true;
            tex.needsUpdate = true;
            return own(tex);
        };
        const noiseTex = mkTex(nd.rgba, nd.N), waveTex = mkTex(nd.nrm, nd.M);

        const sd = AD.sunDirection(look, 0);
        const top = col(look.skyTop), horizon = col(look.skyHorizon);
        const U = {
            uNoise: { value: noiseTex }, uTime: { value: 0 }, uCover: { value: cfg.cover }, uNight: { value: night ? 1 : 0 },
            uAurora: { value: cfg.aurora ? 1 : 0 }, uHaze: { value: cfg.haze }, uSunBoost: { value: 1 }, uSunSize: { value: cfg.sun }, uFogMix: { value: 0.55 },
            uTop: { value: top }, uMid: { value: top.clone().lerp(horizon, 0.28) }, uHorizon: { value: horizon }, uFog: { value: col(look.fog) },
            uGround: { value: col(look.ground).multiplyScalar(0.7) }, uSun: { value: col(look.sun) },
            uSunDir: { value: new T.Vector3(sd[0], sd[1], sd[2]) },
            uCloudLit: { value: col(cfg.lit) }, uCloudDark: { value: col(cfg.dark) },
            uNeonA: { value: col(cfg.na || '#ffffff') }, uNeonB: { value: col(cfg.nb || '#ffffff') }, uNeonC: { value: col(cfg.nc || '#ffffff') }
        };
        const baseCover = cfg.cover, baseFog = U.uFog.value.clone();

        /* Dome. */
        const domeMat = own(new T.ShaderMaterial({ uniforms: Object.assign({}, U), vertexShader: DOME_V, fragmentShader: DOME_F, side: T.BackSide, depthWrite: false, depthTest: false, fog: false }));
        const dome = new T.Mesh(own(new T.SphereGeometry(3000, 48, 24)), domeMat);
        dome.frustumCulled = false; dome.renderOrder = -10; dome.name = 'sky-dome';
        root.add(dome);

        /* Far silhouette ring. */
        let ring = null;
        if (['alpine', 'glacier', 'desert', 'salt', 'forest', 'harbour', 'metro', 'industry'].indexOf(biomeId) >= 0) {
            const tex = own(new T.CanvasTexture(drawRing(biomeId, look)));
            tex.colorSpace = T.SRGBColorSpace; tex.wrapS = T.RepeatWrapping; tex.repeat.set(2, 1); tex.anisotropy = 4;
            ring = new T.Mesh(own(new T.CylinderGeometry(2300, 2300, 520, 64, 1, true)),
                own(new T.MeshBasicMaterial({ map: tex, transparent: true, depthWrite: false, side: T.BackSide, fog: false })));
            ring.position.y = 190; ring.frustumCulled = false; ring.renderOrder = -9; ring.name = 'sky-ring';
            root.add(ring);
        }

        /* Stars. */
        let stars = null;
        const starCount = Math.round(cfg.stars * (glq && glq.weather ? clamp(0.5 + glq.weather * 0.5, 0.6, 1.2) : 1));
        if (starCount > 0) {
            const rnd = seeded(AD.hashString('stars/' + biomeId)), pos = new Float32Array(starCount * 3), dat = new Float32Array(starCount * 3);
            for (let i = 0; i < starCount; i++) {
                const y = Math.pow(rnd(), 0.8) * 0.97 + 0.03, a = rnd() * Math.PI * 2, r = Math.sqrt(1 - y * y);
                pos[i * 3] = Math.cos(a) * r * 2900; pos[i * 3 + 1] = y * 2900; pos[i * 3 + 2] = Math.sin(a) * r * 2900;
                dat[i * 3] = 1.3 + Math.pow(rnd(), 3) * 2.4; dat[i * 3 + 1] = rnd() * 6.28; dat[i * 3 + 2] = rnd();
            }
            const geo = own(new T.BufferGeometry());
            geo.setAttribute('position', new T.BufferAttribute(pos, 3));
            geo.setAttribute('aStar', new T.BufferAttribute(dat, 3));
            const starMat = own(new T.ShaderMaterial({
                uniforms: { uTime: U.uTime, uDpr: { value: glq && glq.dpr ? glq.dpr : 1 }, uDim: { value: cfg.haze > 0.8 ? 0.45 : 1 } },
                vertexShader: 'uniform float uTime, uDpr; attribute vec3 aStar; varying float vA; varying float vH;\nvoid main() { vec3 p = normalize(position); vH = p.y; vA = 0.6 + 0.4 * sin(uTime * (1.4 + aStar.z * 2.0) + aStar.y * 5.0); gl_PointSize = aStar.x * uDpr; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
                fragmentShader: 'uniform float uDim; varying float vA; varying float vH;\nvoid main() { float d = length(gl_PointCoord - 0.5) * 2.0; float a = (1.0 - smoothstep(0.2, 1.0, d)) * vA * uDim * smoothstep(0.03, 0.3, vH); gl_FragColor = vec4(vec3(0.85, 0.92, 1.0) * 1.4, a); }',
                transparent: true, depthWrite: false, depthTest: false, blending: T.AdditiveBlending, fog: false
            }));
            stars = new T.Points(geo, starMat);
            stars.frustumCulled = false; stars.renderOrder = -9.5;
            root.add(stars);
        }

        /* Mirror planes: harbour water, salt flat. */
        let mirror = null;
        const emitU = [], emitC = [];
        for (let i = 0; i < 12; i++) { emitU.push(new T.Vector4(0, 0, 0, 0)); emitC.push(new T.Color(0, 0, 0)); }
        if (biomeId === 'harbour' || biomeId === 'salt') {
            const water = biomeId === 'harbour';
            const mu = Object.assign({}, U, {
                uWave: { value: waveTex }, uDeep: { value: water ? col('#050c1c') : col(look.ground) }, uShallow: { value: water ? col('#0d3a52') : col('#e9e2d8') },
                uWaveAmp: { value: water ? 1.9 : 0.12 }, uBase: { value: water ? 0.05 : 0.2 }, uFogNear: { value: look.fogFar * look.fogNear }, uFogFar: { value: look.fogFar },
                uCrack: { value: water ? 0 : 1 }, uGloss: { value: water ? 0 : 0.18 }, uWaterY: { value: water ? -1.6 : -0.25 }, uEmit: { value: emitU }, uEmitCol: { value: emitC }
            });
            const mat = own(new T.ShaderMaterial({ uniforms: mu, vertexShader: MIRROR_V, fragmentShader: MIRROR_F, transparent: !water, depthWrite: water, fog: false }));
            mirror = new T.Mesh(own(new T.PlaneGeometry(7000, 7000).rotateX(-Math.PI / 2)), mat);
            mirror.position.y = water ? -1.6 : -0.25; mirror.frustumCulled = false; mirror.name = water ? 'sky-water' : 'sky-salt-mirror';
            mirror.renderOrder = water ? -5 : 2;
            root.add(mirror);
        }

        /* Disco beams (additive cones), hidden until setDisco(true). */
        const disco = { on: false, level: 0, group: new T.Group(), beams: [] };
        {
            const geo = own(new T.ConeGeometry(5, 80, 14, 1, true).translate(0, -40, 0));
            for (let i = 0; i < 6; i++) {
                const mat = own(new T.ShaderMaterial({
                    uniforms: { uColor: { value: new T.Color().setHSL(i / 6, 1, 0.6) }, uLevel: { value: 0 } },
                    vertexShader: 'varying float vY;\nvoid main() { vY = position.y; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
                    fragmentShader: 'uniform vec3 uColor; uniform float uLevel; varying float vY;\nvoid main() { float k = smoothstep(-80.0, -4.0, vY) * (1.0 - smoothstep(-6.0, 0.0, vY)); gl_FragColor = vec4(uColor * 2.2, k * 0.34 * uLevel); }',
                    transparent: true, depthWrite: false, blending: T.AdditiveBlending, side: T.DoubleSide, fog: false
                }));
                const m = new T.Mesh(geo, mat);
                m.frustumCulled = false;
                disco.group.add(m); disco.beams.push(m);
            }
            disco.group.visible = false;
            root.add(disco.group);
        }
        const hemi = scene.children.find(o => o.isHemisphereLight) || null;
        const hemiBase = hemi ? hemi.color.clone() : null, auroraTint = col('#46e8a0');

        /* Weather streaks. */
        const wx = {};
        const mkWeather = (kind) => {
            const w = WEATHER[kind];
            const plane = new T.PlaneGeometry(1, 1), geo = own(new T.InstancedBufferGeometry());
            geo.index = plane.index;
            geo.setAttribute('position', plane.attributes.position);
            const n = w.max, seeds = new Float32Array(n * 4), rnd = seeded(AD.hashString('wx/' + kind));
            for (let i = 0; i < seeds.length; i++) seeds[i] = rnd();
            geo.setAttribute('aSeed', new T.InstancedBufferAttribute(seeds, 4));
            geo.instanceCount = 0;
            const tint = kind === 'rain' ? col(look.skyHorizon).lerp(col('#ffffff'), night ? 0.5 : 0.65) : kind === 'sand' ? col(look.terrain.mid) : col(w.color);
            const mat = own(new T.ShaderMaterial({
                uniforms: {
                    uTime: U.uTime, uFlutter: { value: w.flutter }, uLenK: { value: w.lenK }, uFall: { value: new T.Vector3(w.fall[0], w.fall[1], w.fall[2]) },
                    uBox: { value: new T.Vector3(w.box[0], w.box[1], w.box[2]) }, uCamVel: { value: new T.Vector3() }, uSize: { value: new T.Vector2(w.size[0], w.size[1]) },
                    uColor: { value: tint }, uAlpha: { value: w.alpha }, uRound: { value: w.round }
                },
                vertexShader: WX_V, fragmentShader: WX_F, transparent: true, depthWrite: false, fog: false,
                blending: w.additive ? T.AdditiveBlending : T.NormalBlending, side: T.DoubleSide
            }));
            const mesh = new T.Mesh(geo, mat);
            mesh.frustumCulled = false; mesh.renderOrder = 30; mesh.visible = false;
            root.add(mesh);
            wx[kind] = { mesh: mesh, geo: geo, level: 0 };
        };
        const kindsHere = ['rain', 'snow'];
        if (biomeId === 'desert') kindsHere.push('sand');
        if (biomeId === 'industry') kindsHere.push('sparks');
        if (biomeId === 'salt') kindsHere.push('motes');
        for (const k of kindsHere) mkWeather(k);
        const ambient = { sand: biomeId === 'desert' ? 0.15 : 0, sparks: biomeId === 'industry' ? 0.4 : 0, motes: biomeId === 'salt' ? 0.55 : 0, snow: biomeId === 'glacier' ? 0.1 : 0 };

        /* Stand-in ground and road when those modules are missing. */
        let standIn = null;
        if (track && track.samples && (typeof AD.createTerrain !== 'function' || typeof AD.createRoad !== 'function')) {
            standIn = new T.Group();
            standIn.name = 'sky-standin';
            if (typeof AD.createTerrain !== 'function') {
                const gm = own(new T.MeshStandardMaterial({ color: col(look.ground).multiplyScalar(night ? 1.6 : 1.1), roughness: 0.95 }));
                if (biomeId === 'harbour') {
                    let cx = 0, cy = 0;
                    for (const s of track.samples) { cx += s.x; cy += s.y; }
                    cx /= track.samples.length; cy /= track.samples.length;
                    const pos = [], idx = [];
                    track.samples.forEach((s, i) => {
                        const side = Math.sign((cx - s.x) * s.nx + (cy - s.y) * s.ny) || 1;
                        for (const off of [side * (s.halfWidth + 1), side * (s.halfWidth + 420)]) pos.push(s.x + s.nx * off, -0.4, s.y + s.ny * off);
                        const n = track.samples.length, a = i * 2, b = ((i + 1) % n) * 2;
                        idx.push(a, a + 1, b, b, a + 1, b + 1, a, b, a + 1, b, b + 1, a + 1);
                    });
                    const geo = own(new T.BufferGeometry());
                    geo.setAttribute('position', new T.Float32BufferAttribute(pos, 3)); geo.setIndex(idx); geo.computeVertexNormals();
                    const m = new T.Mesh(geo, gm); m.receiveShadow = true; standIn.add(m);
                } else {
                    const m = new T.Mesh(own(new T.PlaneGeometry(9000, 9000).rotateX(-Math.PI / 2)), gm);
                    m.position.set(0, biomeId === 'salt' ? -0.3 : -0.15, 0); m.receiveShadow = true; standIn.add(m);
                }
            }
            if (typeof AD.createRoad !== 'function') {
                const pos = [], idx = [], n = track.samples.length;
                track.samples.forEach((s, i) => {
                    pos.push(s.x + s.nx * s.halfWidth, 0.02, s.y + s.ny * s.halfWidth, s.x - s.nx * s.halfWidth, 0.02, s.y - s.ny * s.halfWidth);
                    const a = i * 2, b = ((i + 1) % n) * 2;
                    idx.push(a, b, a + 1, b, b + 1, a + 1);
                });
                const geo = own(new T.BufferGeometry());
                geo.setAttribute('position', new T.Float32BufferAttribute(pos, 3)); geo.setIndex(idx); geo.computeVertexNormals();
                const m = new T.Mesh(geo, own(new T.MeshStandardMaterial({ color: col(track.biome.road).multiplyScalar(1.3), roughness: 0.85, side: T.DoubleSide })));
                m.receiveShadow = true; standIn.add(m);
            }
            root.add(standIn);
        }

        /* ---- per frame --------------------------------------------------------------------- */
        let time = 0, discoTime = 0, prevEye = null, camVel = new T.Vector3(), fogLevel = 0;
        const fog = scene.fog || null, fogNear0 = fog ? fog.near : 0, fogFar0 = fog ? fog.far : 0;

        function updateWeather(dt, cond, eye) {
            if (prevEye && dt > 1e-4) {
                const k = Math.min(1, dt * 8);
                camVel.x += ((eye[0] - prevEye[0]) / dt - camVel.x) * k; camVel.y += ((eye[1] - prevEye[1]) / dt - camVel.y) * k; camVel.z += ((eye[2] - prevEye[2]) / dt - camVel.z) * k;
            }
            prevEye = [eye[0], eye[1], eye[2]];
            const kind = cond ? cond.kind : 'clear', inten = cond ? clamp(cond.intensity, 0, 1) : 0;
            const want = { rain: kind === 'rain' ? inten : 0, snow: kind === 'snow' ? inten : 0, sand: kind === 'heat' ? inten : 0, sparks: kind === 'haze' ? inten * 0.6 : 0, motes: kind === 'glare' ? inten * 0.6 : 0 };
            let fogWant = (kind === 'fog' || kind === 'mist' || kind === 'haze') ? inten : (kind === 'heat' ? inten * 0.25 : 0);
            const rate = Math.min(1, dt * 0.9);
            let veil = 0;
            for (const k of Object.keys(wx)) {
                const w = wx[k], target = clamp(Math.max(want[k] || 0, ambient[k] || 0), 0, 1);
                w.level += (target - w.level) * rate;
                const n = reduced ? 0 : Math.floor(WEATHER[k].max * clamp(glq.weather || 0.7, 0.2, 1.3) * Math.min(1, w.level * 1.15));
                w.geo.instanceCount = Math.min(n, WEATHER[k].max);
                w.mesh.visible = n > 4;
                if (k === 'rain' || k === 'snow') veil = Math.max(veil, w.level * (reduced ? 0.4 : 0.15));
                w.mesh.material.uniforms.uCamVel.value.copy(camVel);
            }
            fogWant = clamp(fogWant + veil, 0, 1);
            fogLevel += (fogWant - fogLevel) * rate;
            if (fog) {
                fog.far = fogFar0 * (1 - 0.58 * fogLevel); fog.near = fogNear0 * (1 - 0.7 * fogLevel);
                fog.color.copy(baseFog).lerp(U.uHorizon.value, fogLevel * 0.25);
                U.uFog.value.copy(fog.color);
            }
            const bad = kind === 'rain' || kind === 'snow' || kind === 'fog' ? Math.max(inten, 0) : 0;
            const cover = clamp(baseCover - 0.2 * bad, 0.2, 0.9);
            U.uCover.value = cover;
            const glare = kind === 'glare' ? 1 + inten * 0.8 : 1;
            U.uSunBoost.value = glare * (1 - 0.7 * (kind === 'rain' ? inten : 0));
        }

        const api = {
            update(dt, pose, cond) {
                dt = clamp(dt || 0, 0, 0.1);
                if (!reduced) time += dt;
                U.uTime.value = time;
                const e = pose && pose.eye ? pose.eye : [0, 2, 0];
                dome.position.set(e[0], e[1], e[2]);
                if (ring) ring.position.set(e[0], 190, e[2]);
                if (stars) stars.position.set(e[0], e[1], e[2]);
                if (mirror) mirror.position.set(e[0], mirror.position.y, e[2]);
                updateWeather(dt, cond, e);
                if (hemi && cfg.aurora && !disco.on) {
                    const pulse = 0.5 + 0.5 * Math.sin(time * 0.35);
                    hemi.color.copy(hemiBase).lerp(auroraTint, 0.16 + 0.1 * pulse);
                }
                discoTime += dt;
                disco.level += ((disco.on ? 1 : 0) - disco.level) * Math.min(1, dt * 4);
                disco.group.visible = disco.level > 0.02;
                if (disco.group.visible) {
                    const t0 = pose && pose.target ? pose.target : e;
                    disco.group.position.set(t0[0], 62, t0[2]);
                    disco.beams.forEach((m, i) => {
                        const a = discoTime * 0.9 + i * Math.PI / 3, tilt = 0.5 + 0.25 * Math.sin(discoTime * 1.7 + i);
                        m.rotation.set(Math.cos(a) * tilt, 0, Math.sin(a) * tilt);
                        m.material.uniforms.uLevel.value = disco.level;
                        m.material.uniforms.uColor.value.setHSL((discoTime * 0.2 + i / 6) % 1, 1, 0.58);
                    });
                    if (hemi) hemi.color.copy(hemiBase).lerp(new T.Color().setHSL((discoTime * 0.25) % 1, 1, 0.55), 0.55 * disco.level);
                } else if (hemi && !cfg.aurora) hemi.color.copy(hemiBase);
            },
            setDisco(on) { disco.on = !!on; },
            setReduced(on) { reduced = !!on; },
            setQuality(next) { glq = next || glq; },
            /* list: [{ x, y, z, h, color }] in three space; the nearest twelve glow in the water or salt mirror. */
            emitters(list) {
                for (let i = 0; i < 12; i++) {
                    const e = list && list[i];
                    if (e) { emitU[i].set(e.x, e.h || 8, e.z, 1); emitC[i].set(e.color || '#ffffff'); } else emitU[i].set(0, 0, 0, 0);
                }
            },
            dispose() {
                root.removeFromParent();
                if (scene.userData.agdriftSky === api) delete scene.userData.agdriftSky;
                if (fog) { fog.far = fogFar0; fog.near = fogNear0; fog.color.copy(baseFog); }
                if (hemi && hemiBase) hemi.color.copy(hemiBase);
                for (const o of disposables) if (o && o.dispose) o.dispose();
                disposables.length = 0;
            }
        };
        api.update(0, { eye: [0, 2, 0] }, null);
        scene.userData.agdriftSky = api;
        return api;
    };
})();
