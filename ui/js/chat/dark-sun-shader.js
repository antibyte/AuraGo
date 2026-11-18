(function () {
    'use strict';

    // ═══════════════════════════════════════════════════════════════
    //  DARK SUN — ECLIPSE SCENE ENGINE
    //
    //  Layers (back to front, see css/chat-themes.css):
    //    #dark-sun-sky       WebGL  stars, corona, prominences, the dark disc,
    //                               diamond-ring glint, eruption plume, light wave
    //    #dark-sun-scene     2D     basalt plain with glowing cracks, ridge
    //                               silhouettes, crack pulse, 2D eclipse fallback
    //    (chat content)
    //    #dark-sun-overlay   2D     embers, sparks, pointer heat and the light
    //                               wave over the chat (screen blend)
    //  Weather state for CSS: html[data-darksun="calm"|"flare"] while live,
    //  --darksun-flash on .app-header/.app-footer while the light wave passes.
    // ═══════════════════════════════════════════════════════════════

    const MAX_EMBERS = 160;
    const MAX_SPARKS = 240;
    const SUN_X = 0.78;                  // fraction of the canvas width
    const SUN_Y = 0.30;                  // fraction of the canvas height
    const SUN_RADIUS = 0.15;             // fraction of the canvas height
    const FLARE_MIN = 20000;
    const FLARE_MAX = 35000;
    const FLARE_DURATION = 5000;
    const FLARE_RISE = 1200;
    const FLARE_HOLD_UNTIL = 2000;
    const WAVE_START = 1000;
    const WAVE_DURATION = 1700;
    const HEAT_RADIUS = 90;
    const SKY_MAX_W = 1280;
    const SKY_MAX_H = 720;
    const CRACK_COUNT = 30;

    const SKY_VERTEX = `
        attribute vec2 a_position;
        void main() {
            gl_Position = vec4(a_position, 0.0, 1.0);
        }
    `;

    const SKY_FRAGMENT = `
        #ifdef GL_FRAGMENT_PRECISION_HIGH
        precision highp float;
        #else
        precision mediump float;
        #endif
        uniform float u_time;
        uniform vec2 u_res;
        uniform vec2 u_sun;
        uniform float u_radius;
        uniform float u_flare;
        uniform float u_flareAngle;
        uniform float u_wave;
        uniform float u_glint;
        uniform float u_flash;
        uniform float u_glow;

        float hash(vec2 p) {
            vec3 h = fract(vec3(p.xyx) * 0.1031);
            h += dot(h, h.yzx + 33.33);
            return fract((h.x + h.y) * h.z);
        }

        float noise(vec2 p) {
            vec2 i = floor(p);
            vec2 f = fract(p);
            f = f * f * (3.0 - 2.0 * f);
            float a = hash(i);
            float b = hash(i + vec2(1.0, 0.0));
            float c = hash(i + vec2(0.0, 1.0));
            float d = hash(i + vec2(1.0, 1.0));
            return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
        }

        float fbm(vec2 p) {
            float v = 0.0;
            float a = 0.5;
            for (int i = 0; i < 4; i++) {
                v += a * noise(p);
                p = mat2(0.8, -0.6, 0.6, 0.8) * p * 2.03;
                a *= 0.5;
            }
            return v;
        }

        float wrapAngle(float a) {
            return atan(sin(a), cos(a));
        }

        // Premultiplied "over" compositing into the accumulator.
        void over(inout vec3 col, inout float a, vec3 c, float alpha) {
            alpha = clamp(alpha, 0.0, 1.0);
            col = col * (1.0 - alpha) + c * alpha;
            a = a * (1.0 - alpha) + alpha;
        }

        void main() {
            vec2 uv = gl_FragCoord.xy / u_res;
            float aspect = u_res.x / u_res.y;
            vec2 p = vec2(uv.x * aspect, 1.0 - uv.y);
            vec2 q = p - u_sun;
            float d = length(q);
            float ang = atan(q.y, q.x);
            float R = u_radius;
            float t = u_time;

            vec3 col = vec3(0.0);
            float a = 0.0;

            // Deep astronomical twilight: indigo-obsidian zenith warming toward the caldera horizon.
            float skyGrad = smoothstep(0.0, 0.92, 1.0 - uv.y);
            vec3 skyZenith = vec3(0.022, 0.013, 0.042);
            vec3 skyMid = vec3(0.058, 0.022, 0.044);
            vec3 skyLow = vec3(0.16, 0.048, 0.032);
            vec3 skyBase = mix(mix(skyZenith, skyMid, smoothstep(0.0, 0.58, skyGrad)), skyLow, smoothstep(0.52, 1.0, skyGrad));
            over(col, a, skyBase, 0.52);

            // Volcanic ash & solar dust veil, illuminated near the eclipse rather than muddying the whole sky.
            float neb = fbm(p * 1.65 + vec2(t * 0.011, -t * 0.006));
            float nebDetail = fbm(p * 3.4 - vec2(t * 0.016, t * 0.008));
            float sunProx = exp(-max(0.0, d - R) * 1.65);
            vec3 nebCol = mix(vec3(0.11, 0.035, 0.075), vec3(0.56, 0.19, 0.07), clamp(neb * 0.65 + sunProx * 0.55, 0.0, 1.0));
            over(col, a, nebCol, (0.03 + neb * nebDetail * 0.11) * (0.45 + 0.85 * sunProx));

            // Dual-scale starfield, naturally veiled inside the bright inner corona glare.
            float glareMask = smoothstep(R * 1.08, R * 2.55, d) * smoothstep(0.10, 0.58, uv.y);
            vec2 sg = p * 96.0;
            vec2 sc = floor(sg);
            vec2 sf = fract(sg) - 0.5;
            float sh = hash(sc);
            float star = smoothstep(0.985, 1.0, sh) * (1.0 - smoothstep(0.0, 0.065 + sh * 0.045, length(sf)));
            float twinkle = 0.74 + 0.26 * sin(t * (0.45 + sh * 0.75) + sh * 43.0);
            vec3 starTint = mix(vec3(0.86, 0.90, 1.0), vec3(1.0, 0.88, 0.74), fract(sh * 17.3));
            over(col, a, starTint, star * twinkle * glareMask * 0.88);

            vec2 sg2 = p * 43.0 + vec2(13.7, 7.1);
            vec2 sc2 = floor(sg2);
            vec2 sf2 = fract(sg2) - 0.5;
            float sh2 = hash(sc2);
            float brightStar = smoothstep(0.992, 1.0, sh2) * exp(-length(sf2) * 26.0);
            over(col, a, vec3(1.0, 0.95, 0.88), brightStar * glareMask * 0.75);

            // Magma glow rising from the plain, bent by heat shimmer and stronger beneath the eclipse.
            float shimmer = noise(vec2(p.x * 7.5 + t * 0.32, uv.y * 34.0 - t * 1.25)) - 0.5;
            float low = exp(-(uv.y + shimmer * 0.035) * 6.8);
            float sunColumn = 0.72 + 0.52 * exp(-pow((uv.x - u_sun.x / max(aspect, 0.001)) * 2.4, 2.0));
            over(col, a, mix(vec3(0.76, 0.22, 0.08), vec3(1.0, 0.48, 0.14), low), low * sunColumn * (0.34 + u_flare * 0.32));

            // Magnetic K-corona: tilted dipole helmet streamers + fine polar plumes around the disc.
            // Sampled on the unit circle so there is never an atan seam at ±π.
            float axisAng = ang - 0.24;
            float eqWing = pow(abs(cos(axisAng)), 1.35);
            float polarPlume = pow(abs(sin(axisAng)), 1.8) * (0.55 + 0.45 * sin(axisAng * 18.0 + fbm(q * 9.0) * 3.0));
            float rot = t * 0.018;
            vec2 dir = q / max(d, 0.0001);
            vec2 dirA = vec2(dir.x * cos(rot) - dir.y * sin(rot), dir.x * sin(rot) + dir.y * cos(rot));
            float rotB = -rot * 1.55;
            vec2 dirB = vec2(dir.x * cos(rotB) - dir.y * sin(rotB), dir.x * sin(rotB) + dir.y * cos(rotB));
            float streak = fbm(dirA * 3.1 + vec2(d * 2.6 - t * 0.028));
            float streak2 = fbm(dirB * 6.8 + vec2(d * 5.4, 3.1));
            float breath = 0.92 + 0.08 * sin(t * 0.35);
            float wingFalloff = mix(6.8, 3.85, eqWing) / breath;
            float radial = exp(-max(0.0, d - R) * wingFalloff);
            float innerGlow = exp(-max(0.0, d - R) * 16.5);
            float filaments = (0.28 + 0.72 * pow(streak, 1.45)) * (0.62 + 0.38 * streak2) + polarPlume * 0.16;
            float corona = (radial * filaments + innerGlow * 0.62) * (1.0 + u_glow * 0.58 + u_flare * 0.52);
            float outside = smoothstep(R - 0.008, R + 0.008, d);
            vec3 outerCol = vec3(0.68, 0.16, 0.09);
            vec3 midCol = vec3(0.98, 0.46, 0.14);
            vec3 coreCol = vec3(1.0, 0.93, 0.78);
            vec3 coronaCol = mix(mix(outerCol, midCol, clamp(radial * 1.35, 0.0, 1.0)), coreCol, clamp(innerGlow * 1.15 + radial * radial * 0.45, 0.0, 1.0));
            over(col, a, coronaCol, clamp(corona * 0.92, 0.0, 1.0) * outside);

            // H-alpha prominence loops resting on the limb (coral-rose rim warming to incandescent gold).
            for (int i = 0; i < 3; i++) {
                float fi = float(i);
                float pa = 0.85 + fi * 2.12 + sin(t * 0.065 + fi) * 0.14;
                float da = wrapAngle(ang - pa);
                float loopH = R * (0.24 + 0.10 * sin(t * 0.28 + fi * 1.3));
                float arch = sin(clamp((d - R) / max(loopH, 0.001), 0.0, 1.0) * 3.14159);
                float prominence = exp(-pow(da * 8.5, 2.0)) * smoothstep(R + loopH * 1.18, R + loopH * 0.12, d) * outside;
                prominence *= (0.52 + 0.48 * fbm(vec2(da * 22.0, d * 32.0 - t * 0.42))) * (0.65 + 0.35 * arch);
                vec3 promCol = mix(vec3(0.98, 0.34, 0.20), vec3(1.0, 0.78, 0.44), clamp(1.0 - (d - R) / max(loopH, 0.001), 0.0, 1.0));
                over(col, a, promCol, prominence * 0.76);
            }

            // Eruption plume at the flare angle.
            float fa = wrapAngle(ang - u_flareAngle);
            float reach = R * (0.3 + 2.35 * u_flare);
            float plumeShape = smoothstep(R + reach, R + reach * 0.06, d) * outside;
            float plume = exp(-pow(fa * (3.5 + 4.0 * (1.0 - u_flare)), 2.0)) * plumeShape;
            plume *= (0.48 + 0.52 * fbm(vec2(fa * 14.0 + t * 0.8, d * 18.0 - t * 1.5))) * u_flare;
            float plumeCore = exp(-pow(fa * 11.5, 2.0)) * plumeShape * u_flare;
            over(col, a, vec3(1.0, 0.56, 0.22), plume * 0.95);
            over(col, a, vec3(1.0, 0.93, 0.74), plumeCore * 0.84);

            // Light wave: an expanding shock ring launched by the eruption.
            if (u_wave > 0.0) {
                float wr = u_wave * 2.2;
                float ring = exp(-pow((d - wr) * 15.0, 2.0)) * (1.0 - u_wave);
                over(col, a, vec3(1.0, 0.82, 0.58), ring * 0.44);
            }

            // The eclipsed lunar disc with subtle Earthshine Maria relief + razor-sharp chromosphere rim.
            float disc = 1.0 - smoothstep(R - 0.0032, R + 0.0032, d);
            vec2 moonUV = q / max(R, 0.001);
            float maria = fbm(moonUV * 2.6 + vec2(1.7, 3.1));
            float limbGlow = smoothstep(0.35, 1.0, length(moonUV));
            vec3 moonCol = vec3(0.010, 0.006, 0.015) + vec3(0.010, 0.007, 0.014) * maria + vec3(0.022, 0.010, 0.006) * limbGlow;
            over(col, a, moonCol, disc);

            float chromo = exp(-abs(d - R) * 68.0) * (0.78 + 0.22 * streak2);
            over(col, a, vec3(1.0, 0.91, 0.74), chromo * 0.94 * (1.0 - disc * 0.45));

            // Baily's beads + cinema anamorphic diamond-ring glint travelling along the lunar limb.
            float ga = wrapAngle(ang - u_glint);
            float beads = exp(-pow(ga * 9.5, 2.0)) * exp(-abs(d - R) * 105.0) * smoothstep(0.42, 0.82, noise(vec2(ang * 28.0, 4.7)));
            float glintCore = exp(-pow(ga * 24.0, 2.0)) * exp(-abs(d - R) * 95.0);
            vec2 gp = u_sun + vec2(cos(u_glint), sin(u_glint)) * R;
            vec2 gq = p - gp;
            // Tilted anamorphic lens flare (-7.5 deg) + soft secondary diffraction ray + warm iris halo.
            vec2 ar = vec2(gq.x * 0.991 - gq.y * 0.131, gq.x * 0.131 + gq.y * 0.991);
            float anamorphic = exp(-abs(ar.y) * 195.0) * exp(-abs(ar.x) * 6.2);
            float crossRay = exp(-abs(ar.x) * 220.0) * exp(-abs(ar.y) * 14.0) * 0.42;
            float glintHalo = exp(-length(gq) * 18.0) * 0.34;
            float glint = (glintCore * 1.28 + beads * 0.55 + anamorphic * 0.62 + crossRay + glintHalo) * (0.78 + 0.22 * sin(t * 0.8));
            vec3 glintTint = mix(vec3(1.0, 0.82, 0.56), vec3(1.0, 0.98, 0.93), clamp(glintCore + beads, 0.0, 1.0));
            over(col, a, glintTint, clamp(glint, 0.0, 1.0));

            // Flash tint while the eruption wave passes.
            over(col, a, vec3(1.0, 0.85, 0.65), u_flash * 0.18);
            a = min(a, 0.98);

            gl_FragColor = vec4(col, a);
        }
    `;

    // ─── Shared State ───
    let chatBox;
    let resizeObserver = null;
    let animationId = null;
    let active = false;
    let lastTime = 0;
    let sceneTime = 0;
    let headerEl = null, footerEl = null;
    let groundLine = 0;

    // ─── Canvases ───
    let canvas, ctx;              // overlay: embers, sparks, wave (above the chat)
    let sceneCanvas, sctx;        // scene: plain, ridges, cracks, 2D fallback (behind the chat)
    let skyCanvas, gl, skyProgram;
    let uTime, uRes, uSun, uRadius, uFlare, uFlareAngle, uWave, uGlint, uFlash, uGlow;
    let skyPositionBuffer;
    let skyActive = false;

    // ─── Embers ───
    let ex, ey, evx, evy, es, el, ed, ep, eb;
    let emberCount = 0;
    let spawnCarry = 0;

    // ─── Sparks ───
    let sx, sy, svx, svy, sl, sd;
    let sparkCount = 0;

    // ─── Scene caches ───
    let plainCache = null;        // { image, top, height }
    let crackCache = null;        // glow layer drawn over the plain
    let crackPoints = [];         // spawn points for embers
    let coronaCache = null;       // 2D fallback streamers
    const ridgePhases = new Float32Array(6);
    let emberSprites = [];

    // ─── Bubble tracking ───
    let cachedBubbles = [];
    let lastBubbleQueryTime = 0;

    // ─── Eruption state ───
    let flareActive = false;
    let flareStart = 0;
    let nextFlareTime = 0;
    let flareIntensity = 0;
    let flareAngle = 0;
    let waveProgress = 0;
    let flash = 0;
    let flashApplied = false;
    let afterglow = 0;
    let glintAngle = -0.6;
    let flareCount = 0;

    // ─── Pointer heat ───
    let heatX = 0, heatY = 0, heatVX = 0, heatStrength = 0, heatLastTime = 0;

    // ═══════════════════════════════════════════════════════════════
    //  UTILITIES
    // ═══════════════════════════════════════════════════════════════

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'dark-sun' &&
            !document.hidden &&
            !prefersReducedMotion() &&
            window.innerWidth >= 768;
    }

    function rand(min, max) {
        return min + Math.random() * (max - min);
    }

    function smooth(t) {
        t = Math.max(0, Math.min(1, t));
        return t * t * (3 - 2 * t);
    }

    function setSceneState(state) {
        const root = document.documentElement;
        if (!state) {
            root.removeAttribute('data-darksun');
        } else if (root.getAttribute('data-darksun') !== state) {
            root.setAttribute('data-darksun', state);
        }
    }

    function applyFlashVariable(value) {
        if (!headerEl) headerEl = document.querySelector('.app-header');
        if (!footerEl) footerEl = document.querySelector('.app-footer');
        const text = value > 0.01 ? value.toFixed(3) : '0';
        if (headerEl) headerEl.style.setProperty('--darksun-flash', text);
        if (footerEl) footerEl.style.setProperty('--darksun-flash', text);
        flashApplied = value > 0.01;
    }

    // ═══════════════════════════════════════════════════════════════
    //  CANVAS & WEBGL SETUP
    // ═══════════════════════════════════════════════════════════════

    function makeLayer(id, zIndex, blend) {
        const el = document.createElement('canvas');
        el.id = id;
        el.setAttribute('aria-hidden', 'true');
        Object.assign(el.style, {
            position: 'fixed', top: '0', left: '0', width: '0', height: '0',
            pointerEvents: 'none', zIndex: String(zIndex), opacity: '1',
            mixBlendMode: blend || 'normal', display: 'none'
        });
        return el;
    }

    function ensureCanvas() {
        if (canvas) return;
        sceneCanvas = makeLayer('dark-sun-scene', 0, 'normal');
        canvas = makeLayer('dark-sun-overlay', 2, 'screen');
        document.body.appendChild(sceneCanvas);
        document.body.appendChild(canvas);
        sctx = sceneCanvas.getContext('2d', { alpha: true, desynchronized: true });
        ctx = canvas.getContext('2d', { alpha: true, desynchronized: true });
    }

    function initSkyGL() {
        if (skyCanvas) return !!(gl && skyProgram && gl.getProgramParameter(skyProgram, gl.LINK_STATUS));

        skyCanvas = makeLayer('dark-sun-sky', 0, 'normal');
        document.body.insertBefore(skyCanvas, sceneCanvas || canvas);

        gl = skyCanvas.getContext('webgl', {
            alpha: true,
            premultipliedAlpha: true,
            antialias: false,
            preserveDrawingBuffer: false
        });
        if (!gl) {
            console.warn('[DarkSun] WebGL not available, sky disabled');
            return false;
        }

        const compile = (type, source) => {
            const shader = gl.createShader(type);
            gl.shaderSource(shader, source);
            gl.compileShader(shader);
            if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
                console.warn('[DarkSun] Shader compile error:', gl.getShaderInfoLog(shader));
                gl.deleteShader(shader);
                return null;
            }
            return shader;
        };
        const vs = compile(gl.VERTEX_SHADER, SKY_VERTEX);
        const fs = compile(gl.FRAGMENT_SHADER, SKY_FRAGMENT);
        if (!vs || !fs) return false;

        skyProgram = gl.createProgram();
        gl.attachShader(skyProgram, vs);
        gl.attachShader(skyProgram, fs);
        gl.linkProgram(skyProgram);
        if (!gl.getProgramParameter(skyProgram, gl.LINK_STATUS)) {
            console.warn('[DarkSun] Program link error:', gl.getProgramInfoLog(skyProgram));
            return false;
        }

        uTime = gl.getUniformLocation(skyProgram, 'u_time');
        uRes = gl.getUniformLocation(skyProgram, 'u_res');
        uSun = gl.getUniformLocation(skyProgram, 'u_sun');
        uRadius = gl.getUniformLocation(skyProgram, 'u_radius');
        uFlare = gl.getUniformLocation(skyProgram, 'u_flare');
        uFlareAngle = gl.getUniformLocation(skyProgram, 'u_flareAngle');
        uWave = gl.getUniformLocation(skyProgram, 'u_wave');
        uGlint = gl.getUniformLocation(skyProgram, 'u_glint');
        uFlash = gl.getUniformLocation(skyProgram, 'u_flash');
        uGlow = gl.getUniformLocation(skyProgram, 'u_glow');
        gl.deleteShader(vs);
        gl.deleteShader(fs);

        const posLoc = gl.getAttribLocation(skyProgram, 'a_position');
        skyPositionBuffer = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, skyPositionBuffer);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([
            -1, -1,  1, -1,  -1, 1,
            -1,  1,  1, -1,   1, 1
        ]), gl.STATIC_DRAW);
        gl.enableVertexAttribArray(posLoc);
        gl.vertexAttribPointer(posLoc, 2, gl.FLOAT, false, 0, 0);

        // The fullscreen pass writes premultiplied sky; nothing to blend against.
        gl.disable(gl.BLEND);
        return true;
    }

    function updateBounds() {
        if (!chatBox) return false;
        const rect = chatBox.getBoundingClientRect();
        const footer = document.querySelector('.app-footer');
        let extraH = 0;
        if (footer) {
            const fr = footer.getBoundingClientRect();
            extraH = Math.max(0, fr.bottom - rect.bottom);
        }
        const layers = [canvas, sceneCanvas, skyCanvas];
        if (!rect.width || !rect.height) {
            for (let i = 0; i < layers.length; i++) {
                if (layers[i]) { layers[i].style.width = '0'; layers[i].style.height = '0'; }
            }
            return false;
        }
        const left = `${Math.round(rect.left)}px`;
        const top = `${Math.round(rect.top)}px`;
        const w = `${Math.round(rect.width)}px`;
        const h = `${Math.round(rect.height + extraH)}px`;
        groundLine = Math.round(rect.height);
        for (let i = 0; i < layers.length; i++) {
            const el = layers[i];
            if (!el) continue;
            el.style.left = left; el.style.top = top;
            el.style.width = w; el.style.height = h;
        }
        return true;
    }

    function rebuildPools() {
        ex = new Float32Array(MAX_EMBERS); ey = new Float32Array(MAX_EMBERS);
        evx = new Float32Array(MAX_EMBERS); evy = new Float32Array(MAX_EMBERS);
        es = new Float32Array(MAX_EMBERS); el = new Float32Array(MAX_EMBERS);
        ed = new Float32Array(MAX_EMBERS); ep = new Float32Array(MAX_EMBERS);
        eb = new Float32Array(MAX_EMBERS);
        emberCount = 0;
        sx = new Float32Array(MAX_SPARKS); sy = new Float32Array(MAX_SPARKS);
        svx = new Float32Array(MAX_SPARKS); svy = new Float32Array(MAX_SPARKS);
        sl = new Float32Array(MAX_SPARKS); sd = new Float32Array(MAX_SPARKS);
        sparkCount = 0;
        spawnCarry = 0;
    }

    function resize() {
        ensureCanvas();
        if (!ctx || !updateBounds()) return;
        const dpr = Math.min(window.devicePixelRatio || 1, 1.75);
        const rect = canvas.getBoundingClientRect();
        const w = Math.max(1, Math.round(rect.width));
        const h = Math.max(1, Math.round(rect.height));
        canvas.width = Math.floor(w * dpr);
        canvas.height = Math.floor(h * dpr);
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        if (sctx) {
            sceneCanvas.width = canvas.width;
            sceneCanvas.height = canvas.height;
            sctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        }
        rebuildPools();
        buildEmberSprites(dpr);
        buildPlain(w, h, dpr);
        buildCorona(h, dpr);
        // Seed the sky with embers so the plain is alive immediately.
        for (let i = 0; i < Math.min(MAX_EMBERS, 70); i++) spawnEmber(w, h, true);

        if (skyActive && gl && skyCanvas) {
            const sRect = skyCanvas.getBoundingClientRect();
            const sw = Math.max(1, Math.round(sRect.width));
            const sh = Math.max(1, Math.round(sRect.height));
            // The corona is soft; it does not need device-pixel resolution.
            const skyScale = Math.min(0.75, SKY_MAX_W / sw, SKY_MAX_H / sh);
            skyCanvas.width = Math.max(1, Math.floor(sw * skyScale));
            skyCanvas.height = Math.max(1, Math.floor(sh * skyScale));
            gl.viewport(0, 0, skyCanvas.width, skyCanvas.height);
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  SCENE: BASALT PLAIN, CRACKS, RIDGES, 2D CORONA (cached per resize)
    // ═══════════════════════════════════════════════════════════════

    const RIDGES = [
        { crest: 184, amp: [15, 7], wave: [620, 230], fillTop: '#160b12', fillBot: '#0c0508', edgeWarm: '255, 168, 86', edgeCool: '164, 58, 34', edgeAlpha: 0.24 },
        { crest: 122, amp: [22, 10], wave: [490, 185], fillTop: '#110709', fillBot: '#070305', edgeWarm: '255, 152, 68', edgeCool: '198, 66, 28', edgeAlpha: 0.38 },
        { crest: 58, amp: [27, 13], wave: [410, 148], fillTop: '#0a0406', fillBot: '#030102', edgeWarm: '255, 194, 112', edgeCool: '224, 82, 32', edgeAlpha: 0.56 }
    ];

    function ridgeProfile(index, x) {
        const r = RIDGES[index];
        let y = 0;
        for (let k = 0; k < 2; k++) {
            const t = (x / r.wave[k]) * Math.PI * 2 + ridgePhases[index * 2 + k];
            // Volcanic caldera silhouettes: sculpted faulted crags rather than uniform sine waves.
            y += r.amp[k] * (Math.abs(Math.sin(t)) * 1.38 - 0.68 + 0.32 * Math.sin(2.0 * t + 0.65));
        }
        return y;
    }

    function buildPlain(width, height, dpr) {
        plainCache = null;
        crackCache = null;
        crackPoints = [];
        if (!sctx) return;
        const top = Math.max(0, Math.floor(groundLine - RIDGES[0].crest - 64));
        const imgH = Math.max(1, height - top);

        const image = document.createElement('canvas');
        image.width = Math.max(1, Math.floor(width * dpr));
        image.height = Math.max(1, Math.floor(imgH * dpr));
        const c = image.getContext('2d');
        if (!c) return;
        c.setTransform(dpr, 0, 0, dpr, 0, 0);

        const sunX = width * SUN_X;

        // Atmospheric mist between distant volcanic ridges, lit from the eclipse azimuth.
        for (let r = 0; r < RIDGES.length; r++) {
            const ridge = RIDGES[r];
            const crest = groundLine - ridge.crest - top;

            if (r > 0) {
                const hazeY = crest - 26;
                const mist = c.createRadialGradient(sunX, crest + 18, 10, sunX, crest + 24, width * 0.62);
                mist.addColorStop(0, `rgba(255, 118, 46, ${0.06 + r * 0.025})`);
                mist.addColorStop(0.5, `rgba(128, 36, 22, ${0.035 + r * 0.015})`);
                mist.addColorStop(1, 'rgba(20, 8, 12, 0)');
                c.fillStyle = mist;
                c.fillRect(0, Math.max(0, hazeY), width, imgH - Math.max(0, hazeY));
            }

            c.beginPath();
            c.moveTo(0, imgH);
            for (let x = 0; x <= width; x += 2) c.lineTo(x, crest + ridgeProfile(r, x));
            c.lineTo(width, imgH);
            c.closePath();
            const bodyGrad = c.createLinearGradient(0, crest - 30, 0, imgH);
            bodyGrad.addColorStop(0, ridge.fillTop);
            bodyGrad.addColorStop(1, ridge.fillBot);
            c.fillStyle = bodyGrad;
            c.fill();

            // Directionally rim-lit ridge crest: brightest gold directly beneath the eclipsed sun,
            // cooling to smoked ember-crimson toward the shadowed western flank.
            c.beginPath();
            for (let x = 0; x <= width; x += 2) {
                const y = crest + ridgeProfile(r, x);
                if (x === 0) c.moveTo(x, y); else c.lineTo(x, y);
            }
            const rimGrad = c.createLinearGradient(0, 0, width, 0);
            const sunStop = Math.max(0.15, Math.min(0.92, SUN_X));
            rimGrad.addColorStop(0, `rgba(${ridge.edgeCool}, ${ridge.edgeAlpha * 0.36})`);
            rimGrad.addColorStop(Math.max(0.05, sunStop - 0.28), `rgba(${ridge.edgeCool}, ${ridge.edgeAlpha * 0.68})`);
            rimGrad.addColorStop(sunStop, `rgba(${ridge.edgeWarm}, ${ridge.edgeAlpha})`);
            rimGrad.addColorStop(1, `rgba(${ridge.edgeCool}, ${ridge.edgeAlpha * 0.54})`);
            c.strokeStyle = rimGrad;
            c.lineWidth = r === 2 ? 1.45 : 1.15;
            c.shadowBlur = r === 2 ? 10 : 6;
            c.shadowColor = `rgba(${ridge.edgeWarm}, ${ridge.edgeAlpha * 0.65})`;
            c.stroke();
            c.shadowBlur = 0;
        }

        // Cooled obsidian plain below the foreground ridge.
        const plainTop = groundLine - RIDGES[RIDGES.length - 1].crest - top + 8;
        const basalt = c.createLinearGradient(0, plainTop, 0, imgH);
        basalt.addColorStop(0, 'rgba(18, 9, 10, 0)');
        basalt.addColorStop(0.32, 'rgba(13, 7, 8, 0.92)');
        basalt.addColorStop(1, 'rgba(4, 2, 3, 1)');
        c.fillStyle = basalt;
        c.fillRect(0, plainTop, width, imgH - plainTop);

        // Cracks: perspective-foreshortened magma fissures across the basalt plates.
        const glow = document.createElement('canvas');
        glow.width = image.width;
        glow.height = image.height;
        const g = glow.getContext('2d');
        if (!g) {
            plainCache = { image: image, top: top, height: imgH };
            return;
        }
        g.setTransform(dpr, 0, 0, dpr, 0, 0);
        g.lineCap = 'round';
        g.lineJoin = 'round';
        c.lineCap = 'round';
        c.lineJoin = 'round';
        const bandTop = plainTop + 12;
        for (let n = 0; n < CRACK_COUNT; n++) {
            const points = [];
            let x = rand(-20, width + 20);
            let y = rand(bandTop, imgH - 4);
            const dir = Math.random() > 0.5 ? 1 : -1;
            const segments = 8 + Math.floor(rand(0, 16));
            points.push([x, y]);
            for (let i = 0; i < segments; i++) {
                const depthStep = (y - bandTop) / Math.max(1, imgH - bandTop);
                const len = rand(7, 20) * (0.72 + depthStep * 0.45);
                const angle = rand(-0.36, 0.36) + (Math.random() > 0.85 ? rand(-0.9, 0.9) : 0);
                x += Math.cos(angle) * len * dir;
                y += Math.sin(angle) * len * (0.34 + depthStep * 0.28);
                y = Math.max(bandTop, Math.min(imgH - 2, y));
                points.push([x, y]);
                if (i % 3 === 0) crackPoints.push([x, y + top]);
            }
            const depth = (y - bandTop) / Math.max(1, imgH - bandTop);
            // Dark fractured basalt lip on the static plain cache.
            c.beginPath();
            c.moveTo(points[0][0], points[0][1]);
            for (let i = 1; i < points.length; i++) c.lineTo(points[i][0], points[i][1]);
            c.lineWidth = 3.2 + depth * 3.0;
            c.strokeStyle = 'rgba(2, 1, 2, 0.72)';
            c.stroke();

            const stroke = (pts, widthPx, color, blur) => {
                g.beginPath();
                g.moveTo(pts[0][0], pts[0][1]);
                for (let i = 1; i < pts.length; i++) g.lineTo(pts[i][0], pts[i][1]);
                g.lineWidth = widthPx;
                g.strokeStyle = color;
                g.shadowBlur = blur;
                g.shadowColor = color;
                g.stroke();
            };
            stroke(points, 2.8 + depth * 2.8, 'rgba(224, 62, 18, 0.36)', 16);
            stroke(points, 1.5 + depth * 1.6, 'rgba(255, 118, 38, 0.62)', 8);
            stroke(points, 0.65 + depth * 0.75, 'rgba(255, 232, 182, 0.92)', 3);
            if (Math.random() > 0.45 && points.length > 4) {
                const from = points[2 + Math.floor(rand(0, points.length - 3))];
                const branch = [[from[0], from[1]]];
                let bx = from[0], by = from[1];
                for (let i = 0; i < 4; i++) {
                    bx += rand(5, 13) * (Math.random() > 0.5 ? 1 : -1);
                    by += rand(-4, 6);
                    branch.push([bx, by]);
                }
                stroke(branch, 1.15, 'rgba(255, 134, 52, 0.52)', 7);
                stroke(branch, 0.5, 'rgba(255, 218, 158, 0.72)', 2);
            }
        }
        g.shadowBlur = 0;
        plainCache = { image: image, top: top, height: imgH };
        crackCache = glow;
    }

    function buildCorona(height, dpr) {
        const radius = Math.max(150, height * 0.58);
        const size = radius * 2;
        coronaCache = document.createElement('canvas');
        coronaCache.width = Math.max(1, Math.floor(size * dpr));
        coronaCache.height = Math.max(1, Math.floor(size * dpr));
        const c = coronaCache.getContext('2d');
        if (!c) { coronaCache = null; return; }
        c.setTransform(dpr, 0, 0, dpr, 0, 0);
        c.translate(radius, radius);
        const g = c.createRadialGradient(0, 0, 0, 0, 0, radius);
        g.addColorStop(0, 'rgba(255, 240, 204, 0.82)');
        g.addColorStop(0.24, 'rgba(255, 162, 68, 0.36)');
        g.addColorStop(0.62, 'rgba(224, 78, 32, 0.11)');
        g.addColorStop(1, 'rgba(180, 48, 24, 0)');
        c.fillStyle = g;
        for (let i = 0; i < 42; i++) {
            const a = (i / 42) * Math.PI * 2 + rand(-0.035, 0.035);
            const eq = Math.pow(Math.abs(Math.cos(a - 0.24)), 1.35);
            const w = rand(0.018, 0.052) * (0.75 + eq * 0.5);
            const len = radius * rand(0.46, 0.74 + eq * 0.26);
            c.beginPath();
            c.moveTo(0, 0);
            c.lineTo(Math.cos(a - w) * len, Math.sin(a - w) * len);
            c.lineTo(Math.cos(a + w) * len, Math.sin(a + w) * len);
            c.closePath();
            c.globalAlpha = rand(0.28, 0.82);
            c.fill();
        }
        c.globalAlpha = 1;
    }

    function buildEmberSprites(dpr) {
        emberSprites = [];
        const specs = [
            { size: 8, core: '255, 250, 236', mid: '255, 212, 134', outer: '255, 124, 44' },
            { size: 14, core: '255, 242, 214', mid: '255, 180, 92', outer: '244, 92, 32' },
            { size: 22, core: '255, 226, 184', mid: '255, 142, 58', outer: '214, 58, 22' }
        ];
        for (let i = 0; i < specs.length; i++) {
            const spec = specs[i];
            const s = spec.size;
            const sprite = document.createElement('canvas');
            sprite.width = Math.max(1, Math.floor(s * dpr));
            sprite.height = sprite.width;
            const c = sprite.getContext('2d');
            if (!c) continue;
            c.setTransform(dpr, 0, 0, dpr, 0, 0);
            const g = c.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2);
            g.addColorStop(0, `rgba(${spec.core}, 1)`);
            g.addColorStop(0.22, `rgba(${spec.mid}, 0.9)`);
            g.addColorStop(0.52, `rgba(${spec.outer}, 0.42)`);
            g.addColorStop(1, 'rgba(180, 42, 16, 0)');
            c.fillStyle = g;
            c.fillRect(0, 0, s, s);
            emberSprites.push({ image: sprite, size: s });
        }
    }

    function drawPlain(width, height) {
        if (!sctx || !plainCache) return;
        sctx.drawImage(plainCache.image, 0, plainCache.top, width, plainCache.height);
        // Directional corona reflection + magma heat breathing above the caldera.
        const glowTop = groundLine - 155;
        const glow = sctx.createLinearGradient(0, glowTop, 0, groundLine + 24);
        const strength = 0.09 + 0.035 * Math.sin(sceneTime * 0.5) + flareIntensity * 0.22 + afterglow * 0.06;
        glow.addColorStop(0, 'rgba(255, 96, 30, 0)');
        glow.addColorStop(1, `rgba(255, 92, 28, ${strength})`);
        sctx.fillStyle = glow;
        sctx.fillRect(0, glowTop, width, groundLine + 24 - glowTop);

        const sunPool = sctx.createRadialGradient(width * SUN_X, groundLine + 10, 8, width * SUN_X, groundLine + 10, width * 0.44);
        const poolAlpha = 0.08 + 0.025 * Math.sin(sceneTime * 0.35) + flareIntensity * 0.16;
        sunPool.addColorStop(0, `rgba(255, 168, 78, ${poolAlpha})`);
        sunPool.addColorStop(0.55, `rgba(230, 82, 28, ${poolAlpha * 0.42})`);
        sunPool.addColorStop(1, 'rgba(160, 40, 16, 0)');
        sctx.fillStyle = sunPool;
        sctx.fillRect(0, glowTop, width, height - glowTop);

        if (crackCache) {
            sctx.globalAlpha = Math.min(1, 0.58 + 0.16 * Math.sin(sceneTime * 0.45) + flareIntensity * 0.48 + afterglow * 0.15);
            sctx.drawImage(crackCache, 0, plainCache.top, width, plainCache.height);
            sctx.globalAlpha = 1;
        }
    }

    function drawEclipse2D(width, height) {
        // Without WebGL the scene canvas paints the eclipse itself.
        const cxp = width * SUN_X, cyp = height * SUN_Y, R = height * SUN_RADIUS;
        const breath = 0.92 + 0.08 * Math.sin(sceneTime * 0.35);
        if (coronaCache) {
            sctx.save();
            sctx.translate(cxp, cyp);
            sctx.rotate(sceneTime * 0.018);
            sctx.globalAlpha = (0.62 + afterglow * 0.28 + flareIntensity * 0.3) * breath;
            const r = coronaCache.width / (Math.min(window.devicePixelRatio || 1, 1.75) * 2);
            sctx.drawImage(coronaCache, -r, -r, r * 2, r * 2);
            sctx.restore();
        }
        const halo = sctx.createRadialGradient(cxp, cyp, R * 0.98, cxp, cyp, R * 2.8 * breath);
        halo.addColorStop(0, 'rgba(255, 232, 184, 0.62)');
        halo.addColorStop(0.24, 'rgba(255, 138, 52, 0.3)');
        halo.addColorStop(0.65, 'rgba(196, 60, 26, 0.09)');
        halo.addColorStop(1, 'rgba(140, 36, 18, 0)');
        sctx.fillStyle = halo;
        sctx.fillRect(cxp - R * 3, cyp - R * 3, R * 6, R * 6);
        if (flareIntensity > 0.01) {
            sctx.save();
            sctx.translate(cxp, cyp);
            sctx.rotate(flareAngle);
            const reach = R * (0.3 + 1.85 * flareIntensity);
            const plume = sctx.createLinearGradient(R, 0, R + reach, 0);
            plume.addColorStop(0, `rgba(255, 224, 152, ${0.88 * flareIntensity})`);
            plume.addColorStop(0.45, `rgba(255, 132, 46, ${0.55 * flareIntensity})`);
            plume.addColorStop(1, 'rgba(210, 62, 24, 0)');
            sctx.fillStyle = plume;
            sctx.beginPath();
            sctx.moveTo(R * 0.95, -R * 0.22);
            sctx.lineTo(R + reach, -R * 0.07);
            sctx.lineTo(R + reach, R * 0.07);
            sctx.lineTo(R * 0.95, R * 0.22);
            sctx.closePath();
            sctx.fill();
            sctx.restore();
        }
        if (waveProgress > 0 && waveProgress < 1) {
            const wr = waveProgress * 2.2 * height;
            const ring = sctx.createRadialGradient(cxp, cyp, Math.max(0, wr - 60), cxp, cyp, wr + 60);
            const ra = (1 - waveProgress) * 0.4;
            ring.addColorStop(0, 'rgba(255, 200, 140, 0)');
            ring.addColorStop(0.5, `rgba(255, 200, 140, ${ra})`);
            ring.addColorStop(1, 'rgba(255, 200, 140, 0)');
            sctx.fillStyle = ring;
            sctx.fillRect(0, 0, width, height);
        }
        const moonGrad = sctx.createRadialGradient(cxp - R * 0.25, cyp - R * 0.2, R * 0.1, cxp, cyp, R);
        moonGrad.addColorStop(0, '#040208');
        moonGrad.addColorStop(0.82, '#07040a');
        moonGrad.addColorStop(1, '#120809');
        sctx.beginPath();
        sctx.arc(cxp, cyp, R, 0, Math.PI * 2);
        sctx.fillStyle = moonGrad;
        sctx.fill();
        sctx.lineWidth = 1.8;
        sctx.strokeStyle = 'rgba(255, 236, 194, 0.94)';
        sctx.shadowBlur = 18;
        sctx.shadowColor = 'rgba(255, 176, 84, 0.9)';
        sctx.stroke();
        sctx.shadowBlur = 0;

        const gx = cxp + Math.cos(glintAngle) * R, gy = cyp + Math.sin(glintAngle) * R;
        const glint = sctx.createRadialGradient(gx, gy, 0, gx, gy, R * 0.55);
        glint.addColorStop(0, 'rgba(255, 253, 244, 0.98)');
        glint.addColorStop(0.18, 'rgba(255, 228, 174, 0.54)');
        glint.addColorStop(1, 'rgba(255, 186, 104, 0)');
        sctx.fillStyle = glint;
        sctx.fillRect(gx - R, gy - R, R * 2, R * 2);

        // Tilted anamorphic diamond-ring streak matching the WebGL sky shader.
        sctx.save();
        sctx.translate(gx, gy);
        sctx.rotate(-0.13);
        const streak = sctx.createLinearGradient(-R * 2.2, 0, R * 2.2, 0);
        streak.addColorStop(0, 'rgba(255, 210, 150, 0)');
        streak.addColorStop(0.42, 'rgba(255, 234, 196, 0.34)');
        streak.addColorStop(0.5, 'rgba(255, 252, 242, 0.72)');
        streak.addColorStop(0.58, 'rgba(255, 234, 196, 0.34)');
        streak.addColorStop(1, 'rgba(255, 210, 150, 0)');
        sctx.fillStyle = streak;
        sctx.fillRect(-R * 2.2, -1.6, R * 4.4, 3.2);
        sctx.restore();

        if (flash > 0.01) {
            sctx.fillStyle = `rgba(255, 210, 160, ${0.12 * flash})`;
            sctx.fillRect(0, 0, width, height);
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  EMBERS, SPARKS AND BUBBLE CONTACT
    // ═══════════════════════════════════════════════════════════════

    function updateBubbleBounds(time) {
        if (!chatBox || time - lastBubbleQueryTime < 150) return;
        lastBubbleQueryTime = time;
        const bubbleElements = chatBox.querySelectorAll('.bubble');
        const chatBoxRect = chatBox.getBoundingClientRect();
        cachedBubbles = [];
        for (let i = 0; i < bubbleElements.length; i++) {
            const rect = bubbleElements[i].getBoundingClientRect();
            const top = rect.top - chatBoxRect.top;
            const bottom = rect.bottom - chatBoxRect.top;
            if (bottom < -20 || top > chatBoxRect.height + 20 || rect.width < 20) continue;
            cachedBubbles.push({
                left: rect.left - chatBoxRect.left,
                right: rect.right - chatBoxRect.left,
                top: top,
                bottom: bottom
            });
        }
    }

    function spawnEmber(width, height, scatter) {
        if (emberCount >= MAX_EMBERS) return false;
        const i = emberCount++;
        if (crackPoints.length && Math.random() > 0.25) {
            const p = crackPoints[Math.floor(rand(0, crackPoints.length))];
            ex[i] = p[0] + rand(-6, 6);
            ey[i] = Math.min(height - 2, p[1] + rand(-4, 4));
        } else {
            ex[i] = rand(0, width);
            ey[i] = rand(groundLine - 40, height - 4);
        }
        if (scatter) ey[i] = rand(height * 0.25, height);
        ed[i] = rand(0.55, 1.45);
        es[i] = rand(1.2, 3.4) * (0.7 + ed[i] * 0.3);
        evx[i] = rand(-0.25, 0.25);
        evy[i] = -rand(0.45, 1.3) * ed[i];
        el[i] = scatter ? rand(0.2, 0.8) : 0;
        ep[i] = rand(0, Math.PI * 2);
        eb[i] = rand(0.55, 1);
        return true;
    }

    function killEmber(i) {
        emberCount--;
        if (i !== emberCount) {
            ex[i] = ex[emberCount]; ey[i] = ey[emberCount];
            evx[i] = evx[emberCount]; evy[i] = evy[emberCount];
            es[i] = es[emberCount]; el[i] = el[emberCount];
            ed[i] = ed[emberCount]; ep[i] = ep[emberCount];
            eb[i] = eb[emberCount];
        }
    }

    function spawnSparks(x, y, count) {
        for (let k = 0; k < count; k++) {
            if (sparkCount >= MAX_SPARKS) return;
            const i = sparkCount++;
            sx[i] = x;
            sy[i] = y;
            svx[i] = rand(-1.6, 1.6);
            svy[i] = rand(-2.2, 0.4);
            sl[i] = 1;
            sd[i] = rand(0.6, 1.1);
        }
    }

    function updateEmbers(dt, width, height) {
        const rate = 0.32 + flareIntensity * 0.75 + afterglow * 0.15;
        spawnCarry += dt * rate;
        while (spawnCarry >= 1) {
            spawnCarry -= 1;
            spawnEmber(width, height, false);
        }
        const lifeRate = 1 / (60 * 7);
        const wind = Math.sin(sceneTime * 0.18) * 0.22;
        for (let i = 0; i < emberCount; i++) {
            el[i] += dt * lifeRate * ed[i];
            // Buoyancy, a soft wind field and turbulence.
            const turb = Math.sin(ey[i] * 0.012 + sceneTime * 0.9 + ep[i]) * 0.14;
            evx[i] += (wind + turb - evx[i]) * 0.04 * dt;
            evy[i] += (-(0.55 + 0.9 * (1 - ed[i] * 0.3)) * (1 + flareIntensity * 0.6) - evy[i]) * 0.03 * dt;
            if (heatStrength > 0.01) {
                const hx = ex[i] - heatX, hy = ey[i] - heatY;
                const d2 = hx * hx + hy * hy;
                const r2 = HEAT_RADIUS * HEAT_RADIUS;
                if (d2 < r2) {
                    const f = (1 - d2 / r2) * heatStrength;
                    evx[i] += (hx / HEAT_RADIUS) * f * 1.4 + heatVX * f * 0.3;
                    evy[i] -= f * 1.1;
                }
            }
            ex[i] += evx[i] * dt * 1.6;
            ey[i] += evy[i] * dt * 1.6;

            let burst = false;
            for (let b = 0; b < cachedBubbles.length; b++) {
                const bb = cachedBubbles[b];
                if (ex[i] >= bb.left && ex[i] <= bb.right && ey[i] >= bb.top && ey[i] <= bb.bottom) {
                    spawnSparks(ex[i], Math.min(bb.bottom + 2, ey[i] + 3), 4 + Math.floor(rand(0, 3)));
                    burst = true;
                    break;
                }
            }
            if (burst || el[i] >= 1 || ey[i] < -20 || ex[i] < -40 || ex[i] > width + 40) {
                killEmber(i);
                i--;
            }
        }
    }

    function drawEmbers() {
        if (!emberSprites.length) return;
        ctx.save();
        ctx.globalCompositeOperation = 'lighter';
        for (let i = 0; i < emberCount; i++) {
            const life = el[i];
            const fade = Math.sin(Math.min(1, Math.max(0, life)) * Math.PI);
            const glow = eb[i] * (0.82 + 0.18 * Math.sin(sceneTime * 2.2 + ep[i]));
            const alpha = fade * glow * (0.76 + flareIntensity * 0.24);
            if (alpha <= 0.01) continue;
            // Newborn embers leave the cracks white-hot, cooling into amber and ruby cinders aloft.
            const tier = life < 0.24 ? 0 : (es[i] < 2.2 && life < 0.62 ? 1 : 2);
            const sprite = emberSprites[Math.min(emberSprites.length - 1, tier)];
            const size = sprite.size * (0.56 + es[i] * 0.21);
            ctx.globalAlpha = Math.min(1, alpha);
            ctx.drawImage(sprite.image, ex[i] - size / 2, ey[i] - size / 2, size, size);
        }
        ctx.restore();
    }

    function updateAndDrawSparks(dt) {
        ctx.save();
        ctx.globalCompositeOperation = 'lighter';
        ctx.lineCap = 'round';
        for (let i = 0; i < sparkCount; i++) {
            sl[i] -= dt * 0.06 * sd[i];
            svy[i] += 0.08 * dt;
            svx[i] *= 1 - 0.03 * dt;
            sx[i] += svx[i] * dt * 1.5;
            sy[i] += svy[i] * dt * 1.5;
            if (sl[i] <= 0) {
                sparkCount--;
                if (i !== sparkCount) {
                    sx[i] = sx[sparkCount]; sy[i] = sy[sparkCount];
                    svx[i] = svx[sparkCount]; svy[i] = svy[sparkCount];
                    sl[i] = sl[sparkCount]; sd[i] = sd[sparkCount];
                }
                i--;
                continue;
            }
            ctx.strokeStyle = `rgba(255, ${Math.round(190 + 60 * sl[i])}, ${Math.round(120 * sl[i])}, ${0.9 * sl[i]})`;
            ctx.lineWidth = 1.2;
            ctx.beginPath();
            ctx.moveTo(sx[i], sy[i]);
            ctx.lineTo(sx[i] - svx[i] * 2.2, sy[i] - svy[i] * 2.2);
            ctx.stroke();
        }
        ctx.restore();
    }

    function drawWaveOverlay(width, height) {
        if (!(waveProgress > 0 && waveProgress < 1)) return;
        const cxp = width * SUN_X, cyp = height * SUN_Y;
        const wr = waveProgress * 1.4 * Math.max(width, height);
        const ring = ctx.createRadialGradient(cxp, cyp, Math.max(0, wr - 110), cxp, cyp, wr + 110);
        const ra = (1 - waveProgress) * 0.34;
        ring.addColorStop(0, 'rgba(255, 190, 120, 0)');
        ring.addColorStop(0.5, `rgba(255, 190, 120, ${ra})`);
        ring.addColorStop(1, 'rgba(255, 190, 120, 0)');
        ctx.save();
        ctx.globalCompositeOperation = 'lighter';
        ctx.fillStyle = ring;
        ctx.fillRect(0, 0, width, height);
        ctx.restore();
    }

    // ═══════════════════════════════════════════════════════════════
    //  ERUPTION CYCLE
    // ═══════════════════════════════════════════════════════════════

    function updateFlare(dt, time) {
        if (time >= nextFlareTime && !flareActive) {
            flareActive = true;
            flareStart = time;
            nextFlareTime = time + FLARE_DURATION + rand(FLARE_MIN, FLARE_MAX);
            // Erupt toward the open sky, away from the bottom right corner.
            flareAngle = rand(Math.PI * 0.55, Math.PI * 1.9);
            flareCount++;
            setSceneState('flare');
        }
        const elapsed = time - flareStart;
        if (flareActive && elapsed >= FLARE_DURATION) {
            flareActive = false;
            afterglow = 1;
            setSceneState('calm');
        }
        if (flareActive) {
            const rise = smooth(elapsed / FLARE_RISE);
            const decay = 1 - smooth((elapsed - FLARE_HOLD_UNTIL) / (FLARE_DURATION - FLARE_HOLD_UNTIL));
            flareIntensity = rise * decay;
            waveProgress = elapsed >= WAVE_START ? Math.min(1, (elapsed - WAVE_START) / WAVE_DURATION) : 0;
        } else {
            flareIntensity = 0;
            waveProgress = 0;
        }
        flash = waveProgress > 0 && waveProgress < 1 ? (1 - waveProgress) * 0.9 : 0;
        if (flash > 0.01 || flashApplied) applyFlashVariable(flash);
        afterglow *= Math.exp(-dt / 60 * 0.25);
        if (afterglow < 0.005) afterglow = 0;
        sceneTime += dt / 60;
        glintAngle += dt / 60 * 0.06;
        heatStrength *= Math.exp(-dt * 0.1);
    }

    // ═══════════════════════════════════════════════════════════════
    //  WEBGL SKY RENDER
    // ═══════════════════════════════════════════════════════════════

    function renderSky() {
        if (!skyActive || !gl) return;
        const aspect = skyCanvas.width / Math.max(1, skyCanvas.height);
        gl.useProgram(skyProgram);
        gl.uniform1f(uTime, sceneTime);
        gl.uniform2f(uRes, skyCanvas.width, skyCanvas.height);
        gl.uniform2f(uSun, aspect * SUN_X, SUN_Y);
        gl.uniform1f(uRadius, SUN_RADIUS);
        gl.uniform1f(uFlare, flareIntensity);
        gl.uniform1f(uFlareAngle, flareAngle);
        gl.uniform1f(uWave, waveProgress);
        gl.uniform1f(uGlint, glintAngle);
        gl.uniform1f(uFlash, flash);
        gl.uniform1f(uGlow, afterglow);
        gl.drawArrays(gl.TRIANGLES, 0, 6);
    }

    // ═══════════════════════════════════════════════════════════════
    //  MAIN LOOP
    // ═══════════════════════════════════════════════════════════════

    function render(time) {
        if (!active || !ctx) return;
        const rect = canvas.getBoundingClientRect();
        const width = Math.max(1, Math.round(rect.width));
        const height = Math.max(1, Math.round(rect.height));
        const dt = lastTime ? Math.max(0, Math.min(2.5, (time - lastTime) / (1000 / 60))) : 1;
        lastTime = time;

        updateFlare(dt, time);
        updateBubbleBounds(time);

        renderSky();

        if (sctx) {
            sctx.clearRect(0, 0, width, height);
            if (!skyActive) drawEclipse2D(width, height);
            drawPlain(width, height);
        }

        ctx.clearRect(0, 0, width, height);
        updateEmbers(dt, width, height);
        drawEmbers();
        updateAndDrawSparks(dt);
        drawWaveOverlay(width, height);

        animationId = window.requestAnimationFrame(render);
    }

    function onPointerMove(event) {
        if (!active || !canvas) return;
        const rect = canvas.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        const now = performance.now();
        const elapsed = Math.max(8, now - heatLastTime);
        if (heatLastTime) {
            const vx = (x - heatX) / elapsed * 16;
            const vy = (y - heatY) / elapsed * 16;
            heatVX = vx;
            heatStrength = Math.min(1, heatStrength + Math.hypot(vx, vy) * 0.05);
        }
        heatX = x;
        heatY = y;
        heatLastTime = now;
    }

    function onPointerLeave() {
        heatStrength = 0;
        heatLastTime = 0;
    }

    function start() {
        if (active || !shouldRun()) return;
        ensureCanvas();
        if (!ctx) return;

        if (initSkyGL()) {
            skyActive = true;
        }

        active = true;
        canvas.style.display = 'block';
        if (sceneCanvas) sceneCanvas.style.display = 'block';
        if (skyCanvas) skyCanvas.style.display = 'block';
        for (let i = 0; i < ridgePhases.length; i++) ridgePhases[i] = rand(0, Math.PI * 2);
        flareActive = false;
        flareIntensity = 0;
        waveProgress = 0;
        flash = 0;
        afterglow = 0;
        heatStrength = 0;
        heatLastTime = 0;
        glintAngle = rand(-1.2, 0.2);
        resize();
        nextFlareTime = performance.now() + rand(6000, 11000);
        lastTime = 0;
        setSceneState('calm');
        animationId = window.requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
        if (canvas) canvas.style.display = 'none';
        if (sceneCanvas) sceneCanvas.style.display = 'none';
        if (skyCanvas) skyCanvas.style.display = 'none';
        flash = 0;
        if (flashApplied) applyFlashVariable(0);
        setSceneState(null);
    }

    function sync() {
        if (shouldRun()) {
            start();
        } else {
            stop();
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  INIT
    // ═══════════════════════════════════════════════════════════════

    function init() {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', init, { once: true });
            return;
        }

        chatBox = document.getElementById('chat-box');

        if (typeof ResizeObserver !== 'undefined' && chatBox) {
            resizeObserver = new ResizeObserver(() => {
                if (active) resize();
            });
            resizeObserver.observe(chatBox);
        }

        const footer = document.querySelector('.app-footer');
        if (footer && typeof ResizeObserver !== 'undefined') {
            new ResizeObserver(() => { if (active) resize(); }).observe(footer);
        }

        if (chatBox) {
            chatBox.addEventListener('pointermove', onPointerMove, { passive: true });
            chatBox.addEventListener('pointerleave', onPointerLeave, { passive: true });
        }

        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', () => {
            if (active) resize();
            sync();
        });

        document.addEventListener('visibilitychange', () => {
            if (document.hidden) stop(); else sync();
        });

        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            if (mq.addEventListener) mq.addEventListener('change', sync);
            else if (mq.addListener) mq.addListener(sync);
        }

        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true, attributeFilter: ['data-theme']
            });
        }

        sync();
    }

    window.AuraGoDarkSun = { start, stop, sync };
    init();
})();
