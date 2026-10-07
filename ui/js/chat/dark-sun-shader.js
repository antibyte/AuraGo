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

            // Sky: a thin ember nebula over the violet-black backdrop.
            float neb = fbm(p * 1.5 + vec2(t * 0.012, -t * 0.007));
            vec3 nebCol = mix(vec3(0.17, 0.05, 0.11), vec3(0.44, 0.13, 0.06), neb);
            over(col, a, nebCol, 0.04 + neb * 0.1);

            // Stars with a slow, gentle twinkle.
            vec2 sg = p * 92.0;
            vec2 sc = floor(sg);
            vec2 sf = fract(sg) - 0.5;
            float sh = hash(sc);
            float star = smoothstep(0.986, 1.0, sh) * (1.0 - smoothstep(0.0, 0.07 + sh * 0.05, length(sf)));
            float twinkle = 0.72 + 0.28 * sin(t * (0.45 + sh * 0.7) + sh * 40.0);
            star *= twinkle * smoothstep(0.08, 0.6, uv.y);
            over(col, a, vec3(1.0, 0.92, 0.82), star * 0.9);

            // Lava glow rising from the plain, bent by heat shimmer.
            float shimmer = noise(vec2(p.x * 7.0 + t * 0.35, uv.y * 34.0 - t * 1.3)) - 0.5;
            float low = exp(-(uv.y + shimmer * 0.04) * 6.5);
            over(col, a, vec3(0.92, 0.36, 0.1), low * (0.36 + u_flare * 0.3));

            // Corona: breathing streamers around the eclipsed disc. The angular noise is
            // sampled on the unit circle (not on atan) so it has no seam at ±π.
            float rot = t * 0.02;
            vec2 dir = q / max(d, 0.0001);
            vec2 dirA = vec2(dir.x * cos(rot) - dir.y * sin(rot), dir.x * sin(rot) + dir.y * cos(rot));
            float rotB = -rot * 1.7;
            vec2 dirB = vec2(dir.x * cos(rotB) - dir.y * sin(rotB), dir.x * sin(rotB) + dir.y * cos(rotB));
            float streak = fbm(dirA * 2.8 + vec2(d * 3.0 - t * 0.03));
            float streak2 = fbm(dirB * 6.0 + vec2(d * 6.0, 3.1));
            float breath = 0.9 + 0.1 * sin(t * 0.35);
            float radial = exp(-max(0.0, d - R) * (5.5 / breath));
            float corona = radial * (0.25 + 0.75 * pow(streak, 1.6)) * (0.6 + 0.4 * streak2);
            corona *= 1.0 + u_glow * 0.6 + u_flare * 0.5;
            float outside = smoothstep(R - 0.01, R + 0.01, d);
            vec3 coronaCol = mix(vec3(0.95, 0.42, 0.12), vec3(1.0, 0.86, 0.6), radial * radial);
            over(col, a, coronaCol, clamp(corona * 0.9, 0.0, 1.0) * outside);

            // Prominence loops resting on the rim.
            for (int i = 0; i < 3; i++) {
                float fi = float(i);
                float pa = 0.9 + fi * 2.1 + sin(t * 0.07 + fi) * 0.15;
                float da = wrapAngle(ang - pa);
                float loopH = R * (0.26 + 0.1 * sin(t * 0.3 + fi * 1.3));
                float prominence = exp(-pow(da * 8.0, 2.0)) * smoothstep(R + loopH * 1.15, R + loopH * 0.2, d) * outside;
                prominence *= 0.55 + 0.45 * fbm(vec2(da * 20.0, d * 30.0 - t * 0.4));
                over(col, a, vec3(1.0, 0.56, 0.2), prominence * 0.72);
            }

            // Eruption plume at the flare angle.
            float fa = wrapAngle(ang - u_flareAngle);
            float reach = R * (0.3 + 2.3 * u_flare);
            float plumeShape = smoothstep(R + reach, R + reach * 0.08, d) * outside;
            float plume = exp(-pow(fa * (3.6 + 4.0 * (1.0 - u_flare)), 2.0)) * plumeShape;
            plume *= (0.5 + 0.5 * fbm(vec2(fa * 14.0 + t * 0.8, d * 18.0 - t * 1.5))) * u_flare;
            float plumeCore = exp(-pow(fa * 11.0, 2.0)) * plumeShape * u_flare;
            over(col, a, vec3(1.0, 0.62, 0.26), plume * 0.95);
            over(col, a, vec3(1.0, 0.9, 0.66), plumeCore * 0.8);

            // Light wave: an expanding ring launched by the eruption.
            if (u_wave > 0.0) {
                float wr = u_wave * 2.2;
                float ring = exp(-pow((d - wr) * 14.0, 2.0)) * (1.0 - u_wave);
                over(col, a, vec3(1.0, 0.8, 0.55), ring * 0.42);
            }

            // The dark sun itself and its burning chromosphere rim.
            float disc = 1.0 - smoothstep(R - 0.004, R + 0.004, d);
            over(col, a, vec3(0.012, 0.006, 0.012), disc);
            float chromo = exp(-abs(d - R) * 55.0) * (0.8 + 0.2 * streak2);
            over(col, a, vec3(1.0, 0.9, 0.7), chromo * 0.9 * (1.0 - disc * 0.5));

            // Diamond-ring glint travelling along the rim.
            float ga = wrapAngle(ang - u_glint);
            float glintCore = exp(-pow(ga * 22.0, 2.0)) * exp(-abs(d - R) * 90.0);
            vec2 gp = u_sun + vec2(cos(u_glint), sin(u_glint)) * R;
            vec2 gq = p - gp;
            float streakH = exp(-abs(gq.y) * 160.0) * exp(-abs(gq.x) * 7.0);
            float streakV = exp(-abs(gq.x) * 160.0) * exp(-abs(gq.y) * 9.0);
            float glint = (glintCore * 1.2 + (streakH + streakV) * 0.5) * (0.75 + 0.25 * sin(t * 0.8));
            over(col, a, vec3(1.0, 0.97, 0.9), clamp(glint, 0.0, 1.0));

            // Flash tint while the wave passes.
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
        { crest: 150, amp: [18, 9], wave: [520, 190], fill: '#120a0a', edge: '255, 138, 58', edgeAlpha: 0.32 },
        { crest: 62, amp: [26, 12], wave: [420, 150], fill: '#090405', edge: '255, 110, 40', edgeAlpha: 0.5 }
    ];

    function ridgeProfile(index, x) {
        const r = RIDGES[index];
        let y = 0;
        for (let k = 0; k < 2; k++) {
            const t = (x / r.wave[k]) * Math.PI * 2 + ridgePhases[index * 3 + k];
            // Volcanic ridges: sharp folded peaks rather than soft waves.
            y += r.amp[k] * (Math.abs(Math.sin(t)) * 1.4 - 0.7 + 0.3 * Math.sin(2.0 * t + 0.6));
        }
        return y;
    }

    function buildPlain(width, height, dpr) {
        plainCache = null;
        crackCache = null;
        crackPoints = [];
        if (!sctx) return;
        const top = Math.max(0, Math.floor(groundLine - RIDGES[0].crest - 60));
        const imgH = Math.max(1, height - top);

        const image = document.createElement('canvas');
        image.width = Math.max(1, Math.floor(width * dpr));
        image.height = Math.max(1, Math.floor(imgH * dpr));
        const c = image.getContext('2d');
        if (!c) return;
        c.setTransform(dpr, 0, 0, dpr, 0, 0);

        // Ridges, far to near, with lava-lit crest edges.
        for (let r = 0; r < RIDGES.length; r++) {
            const ridge = RIDGES[r];
            const crest = groundLine - ridge.crest - top;
            c.beginPath();
            c.moveTo(0, imgH);
            for (let x = 0; x <= width; x += 2) c.lineTo(x, crest + ridgeProfile(r, x));
            c.lineTo(width, imgH);
            c.closePath();
            c.fillStyle = ridge.fill;
            c.fill();
            c.beginPath();
            for (let x = 0; x <= width; x += 2) {
                const y = crest + ridgeProfile(r, x);
                if (x === 0) c.moveTo(x, y); else c.lineTo(x, y);
            }
            c.strokeStyle = `rgba(${ridge.edge}, ${ridge.edgeAlpha})`;
            c.lineWidth = 1.3;
            c.shadowBlur = 8;
            c.shadowColor = `rgba(${ridge.edge}, ${ridge.edgeAlpha * 0.8})`;
            c.stroke();
            c.shadowBlur = 0;
        }

        // The basalt plain below the near ridge.
        const plainTop = groundLine - RIDGES[1].crest - top + 10;
        const basalt = c.createLinearGradient(0, plainTop, 0, imgH);
        basalt.addColorStop(0, 'rgba(22, 12, 11, 0)');
        basalt.addColorStop(0.35, 'rgba(16, 9, 8, 0.9)');
        basalt.addColorStop(1, 'rgba(4, 2, 2, 1)');
        c.fillStyle = basalt;
        c.fillRect(0, plainTop, width, imgH - plainTop);
        plainCache = { image: image, top: top, height: imgH };

        // Cracks: jagged random walks across the plain, cached as a glow layer.
        const glow = document.createElement('canvas');
        glow.width = image.width;
        glow.height = image.height;
        const g = glow.getContext('2d');
        if (!g) return;
        g.setTransform(dpr, 0, 0, dpr, 0, 0);
        g.lineCap = 'round';
        g.lineJoin = 'round';
        const bandTop = plainTop + 14;
        for (let n = 0; n < CRACK_COUNT; n++) {
            const points = [];
            let x = rand(-20, width + 20);
            let y = rand(bandTop, imgH - 4);
            let dir = Math.random() > 0.5 ? 1 : -1;
            const segments = 8 + Math.floor(rand(0, 16));
            points.push([x, y]);
            for (let i = 0; i < segments; i++) {
                const len = rand(6, 18);
                const angle = rand(-0.45, 0.45) + (Math.random() > 0.85 ? rand(-1.0, 1.0) : 0);
                x += Math.cos(angle) * len * dir;
                y += Math.sin(angle) * len * 0.55;
                y = Math.max(bandTop, Math.min(imgH - 2, y));
                points.push([x, y]);
                if (i % 3 === 0) crackPoints.push([x, y + top]);
            }
            // Deeper cracks nearer the viewer glow wider.
            const depth = (y - bandTop) / Math.max(1, imgH - bandTop);
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
            stroke(points, 2.2 + depth * 2.4, 'rgba(255, 110, 40, 0.5)', 14);
            stroke(points, 0.8 + depth * 0.9, 'rgba(255, 214, 150, 0.9)', 4);
            if (Math.random() > 0.5 && points.length > 4) {
                const from = points[2 + Math.floor(rand(0, points.length - 3))];
                const branch = [[from[0], from[1]]];
                let bx = from[0], by = from[1];
                for (let i = 0; i < 4; i++) {
                    bx += rand(4, 12) * (Math.random() > 0.5 ? 1 : -1);
                    by += rand(-6, 8);
                    branch.push([bx, by]);
                }
                stroke(branch, 1.2, 'rgba(255, 140, 60, 0.55)', 8);
            }
        }
        g.shadowBlur = 0;
        crackCache = glow;
    }

    function buildCorona(height, dpr) {
        const radius = Math.max(140, height * 0.55);
        const size = radius * 2;
        coronaCache = document.createElement('canvas');
        coronaCache.width = Math.max(1, Math.floor(size * dpr));
        coronaCache.height = Math.max(1, Math.floor(size * dpr));
        const c = coronaCache.getContext('2d');
        if (!c) { coronaCache = null; return; }
        c.setTransform(dpr, 0, 0, dpr, 0, 0);
        c.translate(radius, radius);
        const g = c.createRadialGradient(0, 0, 0, 0, 0, radius);
        g.addColorStop(0, 'rgba(255, 226, 170, 0.75)');
        g.addColorStop(0.3, 'rgba(255, 150, 60, 0.3)');
        g.addColorStop(1, 'rgba(240, 92, 43, 0)');
        c.fillStyle = g;
        for (let i = 0; i < 34; i++) {
            const a = (i / 34) * Math.PI * 2 + rand(-0.04, 0.04);
            const w = rand(0.02, 0.06);
            const len = radius * rand(0.5, 1);
            c.beginPath();
            c.moveTo(0, 0);
            c.lineTo(Math.cos(a - w) * len, Math.sin(a - w) * len);
            c.lineTo(Math.cos(a + w) * len, Math.sin(a + w) * len);
            c.closePath();
            c.globalAlpha = rand(0.3, 0.85);
            c.fill();
        }
        c.globalAlpha = 1;
    }

    function buildEmberSprites(dpr) {
        emberSprites = [];
        const sizes = [8, 14, 22];
        for (let i = 0; i < sizes.length; i++) {
            const s = sizes[i];
            const sprite = document.createElement('canvas');
            sprite.width = Math.max(1, Math.floor(s * dpr));
            sprite.height = sprite.width;
            const c = sprite.getContext('2d');
            if (!c) continue;
            c.setTransform(dpr, 0, 0, dpr, 0, 0);
            const g = c.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2);
            g.addColorStop(0, 'rgba(255, 244, 220, 1)');
            g.addColorStop(0.22, 'rgba(255, 196, 110, 0.9)');
            g.addColorStop(0.5, 'rgba(255, 110, 40, 0.42)');
            g.addColorStop(1, 'rgba(200, 60, 20, 0)');
            c.fillStyle = g;
            c.fillRect(0, 0, s, s);
            emberSprites.push({ image: sprite, size: s });
        }
    }

    function drawPlain(width, height) {
        if (!sctx || !plainCache) return;
        sctx.drawImage(plainCache.image, 0, plainCache.top, width, plainCache.height);
        // Lava light breathing above the plain, brighter during an eruption.
        const glowTop = groundLine - 140;
        const glow = sctx.createLinearGradient(0, glowTop, 0, groundLine + 20);
        const strength = 0.1 + 0.04 * Math.sin(sceneTime * 0.5) + flareIntensity * 0.22 + afterglow * 0.06;
        glow.addColorStop(0, 'rgba(255, 96, 30, 0)');
        glow.addColorStop(1, `rgba(255, 96, 30, ${strength})`);
        sctx.fillStyle = glow;
        sctx.fillRect(0, glowTop, width, groundLine + 20 - glowTop);
        if (crackCache) {
            sctx.globalAlpha = Math.min(1, 0.55 + 0.18 * Math.sin(sceneTime * 0.45) + flareIntensity * 0.5 + afterglow * 0.15);
            sctx.drawImage(crackCache, 0, plainCache.top, width, plainCache.height);
            sctx.globalAlpha = 1;
        }
    }

    function drawEclipse2D(width, height) {
        // Without WebGL the scene canvas paints the eclipse itself.
        const cxp = width * SUN_X, cyp = height * SUN_Y, R = height * SUN_RADIUS;
        const breath = 0.9 + 0.1 * Math.sin(sceneTime * 0.35);
        if (coronaCache) {
            sctx.save();
            sctx.translate(cxp, cyp);
            sctx.rotate(sceneTime * 0.02);
            sctx.globalAlpha = (0.55 + afterglow * 0.3 + flareIntensity * 0.3) * breath;
            const r = coronaCache.width / (Math.min(window.devicePixelRatio || 1, 1.75) * 2);
            sctx.drawImage(coronaCache, -r, -r, r * 2, r * 2);
            sctx.restore();
        }
        const halo = sctx.createRadialGradient(cxp, cyp, R, cxp, cyp, R * 2.6 * breath);
        halo.addColorStop(0, 'rgba(255, 200, 120, 0.55)');
        halo.addColorStop(0.3, 'rgba(255, 120, 45, 0.26)');
        halo.addColorStop(1, 'rgba(240, 92, 43, 0)');
        sctx.fillStyle = halo;
        sctx.fillRect(cxp - R * 3, cyp - R * 3, R * 6, R * 6);
        if (flareIntensity > 0.01) {
            sctx.save();
            sctx.translate(cxp, cyp);
            sctx.rotate(flareAngle);
            const reach = R * (0.3 + 1.7 * flareIntensity);
            const plume = sctx.createLinearGradient(R, 0, R + reach, 0);
            plume.addColorStop(0, `rgba(255, 200, 110, ${0.85 * flareIntensity})`);
            plume.addColorStop(1, 'rgba(255, 120, 40, 0)');
            sctx.fillStyle = plume;
            sctx.beginPath();
            sctx.moveTo(R * 0.95, -R * 0.22);
            sctx.lineTo(R + reach, -R * 0.08);
            sctx.lineTo(R + reach, R * 0.08);
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
        sctx.beginPath();
        sctx.arc(cxp, cyp, R, 0, Math.PI * 2);
        sctx.fillStyle = '#030204';
        sctx.fill();
        sctx.lineWidth = 2;
        sctx.strokeStyle = 'rgba(255, 230, 180, 0.9)';
        sctx.shadowBlur = 18;
        sctx.shadowColor = 'rgba(255, 180, 90, 0.9)';
        sctx.stroke();
        sctx.shadowBlur = 0;
        const gx = cxp + Math.cos(glintAngle) * R, gy = cyp + Math.sin(glintAngle) * R;
        const glint = sctx.createRadialGradient(gx, gy, 0, gx, gy, R * 0.5);
        glint.addColorStop(0, 'rgba(255, 252, 240, 0.95)');
        glint.addColorStop(0.2, 'rgba(255, 230, 180, 0.5)');
        glint.addColorStop(1, 'rgba(255, 200, 120, 0)');
        sctx.fillStyle = glint;
        sctx.fillRect(gx - R, gy - R, R * 2, R * 2);
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
            const alpha = fade * glow * (0.75 + flareIntensity * 0.25);
            if (alpha <= 0.01) continue;
            const sprite = emberSprites[es[i] < 1.9 ? 0 : es[i] < 2.8 ? 1 : 2];
            const size = sprite.size * (0.6 + es[i] * 0.22);
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
