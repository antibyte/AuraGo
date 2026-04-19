(function () {
    'use strict';

    // ═══════════════════════════════════════════════════════════════
    //  SANDSTORM PARTICLE + WEBGL FOG ENGINE
    // ═══════════════════════════════════════════════════════════════

    // ─── 2D Particle Config ───
    const MAX_CLOUDS = 5;
    const MAX_FLYING = 320;
    const MAX_TRAILS = 35;
    const MAX_GROUND = 2500;

    const GRAVITY = 0.42;
    const TERMINAL_VELOCITY = 5.5;
    const BASE_WIND = 0.45;
    const STORM_WIND = 6.5;
    const STORM_LIFT = -5.0;
    const IDLE_MIN = 30000;
    const IDLE_MAX = 120000;
    const STORM_DURATION = 4000;
    const GROUND_RES = 3;

    // ─── WebGL Fog Config ───
    const FOG_VERTEX = `
        attribute vec2 a_position;
        void main() {
            gl_Position = vec4(a_position, 0.0, 1.0);
        }
    `;

    const FOG_FRAGMENT = `
        precision mediump float;
        uniform float u_time;
        uniform vec2 u_resolution;
        uniform float u_storm;

        float hash(vec2 p) {
            return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
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
            for (int i = 0; i < 5; i++) {
                v += a * noise(p);
                p *= 2.03;
                a *= 0.5;
            }
            return v;
        }

        void main() {
            vec2 uv = gl_FragCoord.xy / u_resolution;
            vec2 aspect = vec2(u_resolution.x / u_resolution.y, 1.0);

            // Much faster drift so fog is clearly animated
            float t = u_time * 1.2;
            vec2 q = vec2(
                fbm(uv * aspect * 2.0 + vec2(t * 0.35, t * 0.18)),
                fbm(uv * aspect * 2.0 + vec2(5.2, 1.3) + vec2(t * 0.22, t * 0.28))
            );

            vec2 r = vec2(
                fbm(uv * aspect * 2.0 + 3.0 * q + vec2(1.7, 9.2) + t * 0.20),
                fbm(uv * aspect * 2.0 + 3.0 * q + vec2(8.3, 2.8) + t * 0.24)
            );

            float f = fbm(uv * aspect * 2.0 + 2.0 * r);

            // Storm turbulence
            float stormT = u_storm * 0.8;
            float stormNoise = fbm(uv * aspect * 4.0 + t * 2.0) * stormT;
            f += stormNoise;

            float detail = fbm(uv * aspect * 5.0 + 5.0 * r + t * 0.15) * 0.35;
            f += detail * (1.0 + stormT);

            // Brighter warm sand colors
            vec3 c1 = vec3(0.95, 0.82, 0.58);
            vec3 c2 = vec3(0.82, 0.62, 0.36);
            vec3 c3 = vec3(0.62, 0.42, 0.24);
            vec3 c4 = vec3(0.42, 0.28, 0.15);

            vec3 col = mix(c4, c3, smoothstep(0.0, 0.3, f));
            col = mix(col, c2, smoothstep(0.3, 0.55, f));
            col = mix(col, c1, smoothstep(0.55, 0.85, f));

            col += vec3(0.08, 0.03, 0.0) * stormT;
            col = clamp(col, 0.0, 1.0);

            // Stronger alpha so fog is actually visible
            float alpha = f * 0.25;
            alpha *= (0.35 + 0.65 * smoothstep(0.0, 0.5, 1.0 - uv.y));
            alpha += u_storm * 0.10;
            alpha = clamp(alpha, 0.0, 0.50);

            gl_FragColor = vec4(col, alpha);
        }
    `;

    // ─── Shared State ───
    let chatBox;
    let resizeObserver = null;
    let animationId = null;
    let active = false;
    let lastTime = 0;

    // ─── 2D Canvas State ───
    let canvas, ctx;

    // Flying particles
    let fx, fy, fvx, fvy, fs, fa, fd, fk;
    let fCount = 0;

    // Trails
    let tx, ty, tvx, tvy, tl, ta, td;
    let tCount = 0;

    // Ground particles
    let gx, gy, gs, ga, gd;
    let gCount = 0;

    // Background clouds
    let cx, cy, cr, ca, cvx, cvy;
    let cCount = 0;

    // Ground height map
    let groundHeight = [];

    // Storm state
    let stormActive = false;
    let stormEndTime = 0;
    let nextStormTime = 0;

    // ─── WebGL Fog State ───
    let fogCanvas, gl, fogProgram;
    let uTime, uRes, uStorm;
    let fogPositionBuffer;
    let fogActive = false;

    // ═══════════════════════════════════════════════════════════════
    //  UTILITIES
    // ═══════════════════════════════════════════════════════════════

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'sandstorm' &&
            !prefersReducedMotion() &&
            window.innerWidth >= 640;
    }

    function rand(min, max) {
        return min + Math.random() * (max - min);
    }

    // ═══════════════════════════════════════════════════════════════
    //  CANVAS & WEBGL SETUP
    // ═══════════════════════════════════════════════════════════════

    function ensureCanvas() {
        if (canvas) return;
        canvas = document.createElement('canvas');
        canvas.id = 'sandstorm-overlay';
        Object.assign(canvas.style, {
            position: 'fixed', top: '0', left: '0', width: '0', height: '0',
            pointerEvents: 'none', zIndex: '2', opacity: '1',
            mixBlendMode: 'screen', display: 'none'
        });
        document.body.appendChild(canvas);
        ctx = canvas.getContext('2d', { alpha: true, desynchronized: true });
    }

    function initFogGL() {
        if (fogCanvas) return true;

        fogCanvas = document.createElement('canvas');
        fogCanvas.id = 'sandstorm-fog';
        Object.assign(fogCanvas.style, {
            position: 'fixed', top: '0', left: '0', width: '0', height: '0',
            pointerEvents: 'none', zIndex: '2', opacity: '1',
            mixBlendMode: 'normal', display: 'none'
        });
        document.body.appendChild(fogCanvas);

        gl = fogCanvas.getContext('webgl', {
            alpha: true,
            premultipliedAlpha: false,
            antialias: false,
            preserveDrawingBuffer: false
        });
        if (!gl) {
            console.warn('[Sandstorm] WebGL not available, fog disabled');
            return false;
        }

        const vs = gl.createShader(gl.VERTEX_SHADER);
        gl.shaderSource(vs, FOG_VERTEX);
        gl.compileShader(vs);
        if (!gl.getShaderParameter(vs, gl.COMPILE_STATUS)) {
            console.error('Fog VS error:', gl.getShaderInfoLog(vs));
            return false;
        }

        const fs = gl.createShader(gl.FRAGMENT_SHADER);
        gl.shaderSource(fs, FOG_FRAGMENT);
        gl.compileShader(fs);
        if (!gl.getShaderParameter(fs, gl.COMPILE_STATUS)) {
            console.error('Fog FS error:', gl.getShaderInfoLog(fs));
            return false;
        }

        fogProgram = gl.createProgram();
        gl.attachShader(fogProgram, vs);
        gl.attachShader(fogProgram, fs);
        gl.linkProgram(fogProgram);
        if (!gl.getProgramParameter(fogProgram, gl.LINK_STATUS)) {
            console.error('Fog link error:', gl.getProgramInfoLog(fogProgram));
            return false;
        }

        uTime = gl.getUniformLocation(fogProgram, 'u_time');
        uRes = gl.getUniformLocation(fogProgram, 'u_resolution');
        uStorm = gl.getUniformLocation(fogProgram, 'u_storm');

        const posLoc = gl.getAttribLocation(fogProgram, 'a_position');
        fogPositionBuffer = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, fogPositionBuffer);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([
            -1, -1,  1, -1,  -1, 1,
            -1,  1,  1, -1,   1, 1
        ]), gl.STATIC_DRAW);
        gl.enableVertexAttribArray(posLoc);
        gl.vertexAttribPointer(posLoc, 2, gl.FLOAT, false, 0, 0);

        gl.enable(gl.BLEND);
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

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
        if (!rect.width || !rect.height) {
            if (canvas) { canvas.style.width = '0'; canvas.style.height = '0'; }
            if (fogCanvas) { fogCanvas.style.width = '0'; fogCanvas.style.height = '0'; }
            return false;
        }
        const left = `${Math.round(rect.left)}px`;
        const top = `${Math.round(rect.top)}px`;
        const w = `${Math.round(rect.width)}px`;
        const h = `${Math.round(rect.height + extraH)}px`;

        if (canvas) {
            canvas.style.left = left; canvas.style.top = top;
            canvas.style.width = w; canvas.style.height = h;
        }
        if (fogCanvas) {
            fogCanvas.style.left = left; fogCanvas.style.top = top;
            fogCanvas.style.width = w; fogCanvas.style.height = h;
        }
        return true;
    }

    // ═══════════════════════════════════════════════════════════════
    //  2D PARTICLE SYSTEM
    // ═══════════════════════════════════════════════════════════════

    function initGround(width) {
        const buckets = Math.ceil(width / GROUND_RES) + 2;
        groundHeight = new Float32Array(buckets);
        for (let i = 0; i < buckets; i++) groundHeight[i] = 0;
    }

    function getGroundY(x, canvasHeight) {
        const idx = Math.floor(x / GROUND_RES);
        if (idx < 0 || idx >= groundHeight.length) return canvasHeight;
        return canvasHeight - groundHeight[idx];
    }

    function addGroundHeight(x, amount) {
        const idx = Math.floor(x / GROUND_RES);
        if (idx >= 0 && idx < groundHeight.length) {
            groundHeight[idx] = Math.min(groundHeight[idx] + amount, 35);
        }
        if (idx > 0) groundHeight[idx - 1] = Math.min(groundHeight[idx - 1] + amount * 0.25, 35);
        if (idx + 1 < groundHeight.length) groundHeight[idx + 1] = Math.min(groundHeight[idx + 1] + amount * 0.25, 35);
    }

    function removeGroundHeight(x, amount) {
        const idx = Math.floor(x / GROUND_RES);
        if (idx >= 0 && idx < groundHeight.length) {
            groundHeight[idx] = Math.max(0, groundHeight[idx] - amount);
        }
        if (idx > 0) groundHeight[idx - 1] = Math.max(0, groundHeight[idx - 1] - amount * 0.3);
        if (idx + 1 < groundHeight.length) groundHeight[idx + 1] = Math.max(0, groundHeight[idx + 1] - amount * 0.3);
    }

    function spawnFlying(i, width, height, fromEdge) {
        fx[i] = fromEdge ? rand(-width * 0.2, -5) : rand(-width * 0.05, width * 1.02);
        fy[i] = fromEdge ? rand(-height * 0.35, height * 0.25) : rand(-height * 0.25, -5);
        fvx[i] = rand(BASE_WIND * 0.4, BASE_WIND * 1.3);
        fvy[i] = rand(-0.8, 1.2);
        fs[i] = rand(1.0, 2.8);
        fa[i] = rand(0.25, 0.7);
        fd[i] = rand(0.8, 1.3);
        fk[i] = Math.random() > 0.88 ? 1 : 0;
    }

    function spawnTrail(i, width, height, fromEdge) {
        tx[i] = fromEdge ? rand(-width * 0.25, -5) : rand(-width * 0.05, width);
        ty[i] = rand(-height * 0.2, height * 0.85);
        tvx[i] = rand(1.0, 2.8);
        tvy[i] = rand(-0.4, 0.4);
        tl[i] = rand(10, 28);
        ta[i] = rand(0.04, 0.13);
        td[i] = rand(0.8, 1.4);
    }

    function spawnCloud(i, width, height) {
        cx[i] = rand(-width * 0.3, width * 1.1);
        cy[i] = rand(-height * 0.1, height * 0.6);
        cr[i] = rand(width * 0.06, width * 0.2);
        ca[i] = rand(0.025, 0.065);
        cvx[i] = rand(0.015, 0.05);
        cvy[i] = rand(-0.008, 0.008);
    }

    function addToGround(x, y, size, alpha) {
        if (gCount >= MAX_GROUND) {
            gx[0] = x; gy[0] = y; gs[0] = size; ga[0] = alpha; gd[0] = rand(0, Math.PI * 2);
            addGroundHeight(x, size * 0.35);
            return;
        }
        gx[gCount] = x; gy[gCount] = y; gs[gCount] = size; ga[gCount] = alpha; gd[gCount] = rand(0, Math.PI * 2);
        gCount++;
        addGroundHeight(x, size * 0.35);
    }

    function rebuildPools(width, height) {
        const area = width * height;
        fCount = Math.max(50, Math.min(MAX_FLYING, Math.round(area / 7000)));
        tCount = Math.max(12, Math.min(MAX_TRAILS, Math.round(area / 30000)));
        cCount = Math.max(2, Math.min(MAX_CLOUDS, Math.round(area / 180000)));

        fx = new Float32Array(MAX_FLYING); fy = new Float32Array(MAX_FLYING);
        fvx = new Float32Array(MAX_FLYING); fvy = new Float32Array(MAX_FLYING);
        fs = new Float32Array(MAX_FLYING); fa = new Float32Array(MAX_FLYING);
        fd = new Float32Array(MAX_FLYING); fk = new Uint8Array(MAX_FLYING);

        tx = new Float32Array(MAX_TRAILS); ty = new Float32Array(MAX_TRAILS);
        tvx = new Float32Array(MAX_TRAILS); tvy = new Float32Array(MAX_TRAILS);
        tl = new Float32Array(MAX_TRAILS); ta = new Float32Array(MAX_TRAILS);
        td = new Float32Array(MAX_TRAILS);

        gx = new Float32Array(MAX_GROUND); gy = new Float32Array(MAX_GROUND);
        gs = new Float32Array(MAX_GROUND); ga = new Float32Array(MAX_GROUND);
        gd = new Float32Array(MAX_GROUND);
        gCount = 0;

        cx = new Float32Array(MAX_CLOUDS); cy = new Float32Array(MAX_CLOUDS);
        cr = new Float32Array(MAX_CLOUDS); ca = new Float32Array(MAX_CLOUDS);
        cvx = new Float32Array(MAX_CLOUDS); cvy = new Float32Array(MAX_CLOUDS);

        for (let i = 0; i < fCount; i++) spawnFlying(i, width, height, false);
        for (let i = 0; i < tCount; i++) spawnTrail(i, width, height, false);
        for (let i = 0; i < cCount; i++) spawnCloud(i, width, height);

        initGround(width);
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
        rebuildPools(w, h);

        // Resize WebGL fog canvas
        if (fogActive && gl && fogCanvas) {
            const fogDpr = Math.min(window.devicePixelRatio || 1, 1.5);
            const fRect = fogCanvas.getBoundingClientRect();
            const fw = Math.max(1, Math.round(fRect.width));
            const fh = Math.max(1, Math.round(fRect.height));
            fogCanvas.width = Math.floor(fw * fogDpr);
            fogCanvas.height = Math.floor(fh * fogDpr);
            gl.viewport(0, 0, fogCanvas.width, fogCanvas.height);
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  2D DRAWING
    // ═══════════════════════════════════════════════════════════════

    function drawClouds(width, height, time) {
        for (let i = 0; i < cCount; i++) {
            const g = ctx.createRadialGradient(cx[i], cy[i], 0, cx[i], cy[i], cr[i]);
            const alpha = ca[i] * (0.5 + Math.sin(time * 0.0002 + i * 3.1) * 0.5);
            g.addColorStop(0, `rgba(210, 178, 132, ${alpha})`);
            g.addColorStop(0.5, `rgba(180, 142, 96, ${alpha * 0.4})`);
            g.addColorStop(1, `rgba(150, 112, 68, 0)`);
            ctx.fillStyle = g;
            ctx.fillRect(cx[i] - cr[i], cy[i] - cr[i], cr[i] * 2, cr[i] * 2);
        }
    }

    function updateClouds(dt, width, height) {
        const wind = stormActive ? STORM_WIND * 0.25 : BASE_WIND * 0.15;
        for (let i = 0; i < cCount; i++) {
            cx[i] += (cvx[i] + wind) * dt;
            cy[i] += cvy[i] * dt;
            if (cx[i] - cr[i] > width * 1.2) {
                cx[i] = -cr[i] - rand(width * 0.05, width * 0.25);
                cy[i] = rand(-height * 0.1, height * 0.6);
            }
        }
    }

    function drawGroundPile(width, height) {
        let hasSand = false;
        for (let i = 0; i < groundHeight.length; i++) {
            if (groundHeight[i] > 0.5) { hasSand = true; break; }
        }
        if (!hasSand) return;
        ctx.beginPath();
        ctx.moveTo(0, height);
        for (let x = 0; x <= width; x += 2) {
            const idx = Math.floor(x / GROUND_RES);
            let h = 0;
            if (idx >= 0 && idx < groundHeight.length) {
                h = groundHeight[idx];
                const nextIdx = Math.min(idx + 1, groundHeight.length - 1);
                const frac = (x % GROUND_RES) / GROUND_RES;
                h += (groundHeight[nextIdx] - h) * frac;
            }
            const noise = Math.sin(x * 0.12) * 1.2 + Math.sin(x * 0.05 + 0.8) * 2.0;
            ctx.lineTo(x, height - Math.max(0, h + noise * 0.25));
        }
        ctx.lineTo(width, height);
        ctx.closePath();

        const grad = ctx.createLinearGradient(0, height - 20, 0, height);
        grad.addColorStop(0, 'rgba(195, 160, 115, 0.35)');
        grad.addColorStop(0.35, 'rgba(175, 138, 92, 0.55)');
        grad.addColorStop(0.75, 'rgba(155, 118, 75, 0.75)');
        grad.addColorStop(1, 'rgba(135, 100, 62, 0.9)');
        ctx.fillStyle = grad;
        ctx.fill();
    }

    function drawGroundParticles() {
        for (let i = 0; i < gCount; i++) {
            const drift = Math.sin(gd[i] + performance.now() * 0.0008) * 0.25;
            const x = gx[i] + drift, y = gy[i], s = gs[i];
            ctx.fillStyle = `rgba(185, 148, 105, ${ga[i] * 0.85})`;
            ctx.fillRect(x, y, s, s);
            ctx.fillStyle = `rgba(215, 182, 138, ${ga[i] * 0.35})`;
            ctx.fillRect(x + s * 0.1, y + s * 0.1, s * 0.45, s * 0.45);
        }
    }

    function updateAndDrawFlying(dt, time, width, height) {
        const storm = stormActive;
        const wind = storm
            ? STORM_WIND * (0.7 + Math.sin(time * 0.012) * 0.3)
            : BASE_WIND + Math.sin(time * 0.00015) * 0.15;

        for (let i = 0; i < fCount; i++) {
            const groundDist = Math.max(0, height - fy[i]);
            const heightFactor = Math.min(1, groundDist / (height * 0.35));
            const swirl = Math.sin(fy[i] * 0.01 + time * 0.0009 + fd[i]) * 0.18;

            fvx[i] += ((wind * heightFactor + swirl) - fvx[i]) * 0.038 * dt;

            let targetVy = GRAVITY;
            if (storm && fy[i] > height * 0.5) {
                targetVy = STORM_LIFT * (0.4 + Math.random() * 0.3);
            }
            fvy[i] += (targetVy - fvy[i]) * 0.06 * dt;
            fvy[i] = Math.max(-TERMINAL_VELOCITY, Math.min(TERMINAL_VELOCITY, fvy[i]));

            fx[i] += fvx[i] * fd[i] * dt * 2.4;
            fy[i] += fvy[i] * fd[i] * dt * 2.2;

            const groundY = getGroundY(fx[i], height);
            if (!storm && fy[i] >= groundY - 2 && fvy[i] > 0) {
                addToGround(fx[i], groundY - rand(0, 3), fs[i], fa[i]);
                spawnFlying(i, width, height, true);
                continue;
            }

            if (fx[i] > width * 1.12 || fy[i] < -height * 0.35 || fy[i] > height + 15) {
                spawnFlying(i, width, height, true);
            }

            if (fk[i] === 1) {
                ctx.strokeStyle = `rgba(235, 208, 165, ${fa[i] * (storm ? 0.65 : 0.45)})`;
                ctx.lineWidth = Math.max(0.5, fs[i] * 0.35);
                ctx.beginPath();
                ctx.moveTo(fx[i], fy[i]);
                ctx.lineTo(fx[i] - fvx[i] * 6, fy[i] - fvy[i] * 4.5);
                ctx.stroke();
            } else {
                ctx.fillStyle = `rgba(225, 192, 145, ${fa[i] * (storm ? 0.95 : 0.8)})`;
                ctx.fillRect(fx[i], fy[i], fs[i], fs[i]);
            }
        }
    }

    function updateAndDrawTrails(dt, time, width, height) {
        const wind = stormActive ? STORM_WIND * 0.7 : BASE_WIND;
        for (let i = 0; i < tCount; i++) {
            const swirl = Math.sin(ty[i] * 0.007 + time * 0.0007 + td[i]) * 0.25;
            tvx[i] += ((wind + swirl) - tvx[i]) * 0.028 * dt;
            tvy[i] += (Math.cos(tx[i] * 0.004 + time * 0.0004 + td[i]) * 0.04 - tvy[i]) * 0.018 * dt;

            tx[i] += tvx[i] * dt * 2.4;
            ty[i] += tvy[i] * dt * 1.8;

            if (tx[i] > width * 1.12 || ty[i] < -height * 0.25 || ty[i] > height * 1.08) {
                spawnTrail(i, width, height, true);
            }

            ctx.strokeStyle = `rgba(230, 200, 155, ${ta[i] * (stormActive ? 1.1 : 0.85)})`;
            ctx.lineWidth = stormActive ? 1.3 : 0.9;
            ctx.beginPath();
            ctx.moveTo(tx[i], ty[i]);
            ctx.lineTo(tx[i] - tl[i], ty[i] - tvy[i] * 5);
            ctx.stroke();
        }
    }

    function whirlGroundSand(width, height) {
        // Whirl a large chunk of ground sand back into the air
        const toWhirl = Math.min(gCount, Math.ceil(gCount * 0.35) + 5);
        for (let i = 0; i < toWhirl && gCount > 0; i++) {
            const idx = Math.floor(rand(0, gCount));
            if (fCount >= MAX_FLYING) break;

            fx[fCount] = gx[idx];
            fy[fCount] = gy[idx] - rand(10, 50);
            fvx[fCount] = rand(STORM_WIND * 0.5, STORM_WIND * 1.1);
            fvy[fCount] = rand(STORM_LIFT * 0.6, STORM_LIFT * 0.1);
            fs[fCount] = gs[idx];
            fa[fCount] = Math.min(0.95, ga[idx] * 1.2);
            fd[fCount] = rand(0.8, 1.3);
            fk[fCount] = 0;
            fCount++;

            removeGroundHeight(gx[idx], gs[idx] * 1.2);

            gx[idx] = gx[gCount - 1]; gy[idx] = gy[gCount - 1];
            gs[idx] = gs[gCount - 1]; ga[idx] = ga[gCount - 1];
            gd[idx] = gd[gCount - 1];
            gCount--;
        }
    }

    function decayGround() {
        for (let i = 0; i < groundHeight.length; i++) {
            groundHeight[i] *= 0.9997;
            if (groundHeight[i] < 0.05) groundHeight[i] = 0;
        }
    }

    function erodeGroundDuringStorm() {
        // Aggressively flatten the sand pile during a storm
        for (let i = 0; i < groundHeight.length; i++) {
            groundHeight[i] *= 0.985;
            if (groundHeight[i] < 0.3) groundHeight[i] = 0;
        }
    }

    function drawStormHaze(width, height, time) {
        if (!stormActive) return;
        const elapsed = time - (stormEndTime - STORM_DURATION);
        const buildUp = Math.min(1, elapsed / 800);
        const fadeOut = Math.min(1, (STORM_DURATION - elapsed) / 900);
        const alpha = buildUp * fadeOut * 0.05;

        const grad = ctx.createLinearGradient(0, 0, width, 0);
        grad.addColorStop(0, `rgba(225, 190, 140, 0)`);
        grad.addColorStop(0.35, `rgba(215, 178, 128, ${alpha})`);
        grad.addColorStop(0.65, `rgba(205, 168, 118, ${alpha * 0.7})`);
        grad.addColorStop(1, `rgba(190, 152, 105, 0)`);
        ctx.fillStyle = grad;
        ctx.fillRect(0, 0, width, height);
    }

    // ═══════════════════════════════════════════════════════════════
    //  WEBGL FOG RENDER
    // ═══════════════════════════════════════════════════════════════

    function renderFog(now) {
        if (!fogActive || !gl) return;

        const elapsed = stormActive ? (now - (stormEndTime - STORM_DURATION)) : 0;
        const stormIntensity = stormActive
            ? Math.min(1, elapsed / 800) * Math.min(1, (STORM_DURATION - elapsed) / 900)
            : 0;

        gl.useProgram(fogProgram);
        gl.uniform1f(uTime, now * 0.001);
        gl.uniform2f(uRes, fogCanvas.width, fogCanvas.height);
        gl.uniform1f(uStorm, stormIntensity);

        // Re-bind buffer and attrib pointer to be safe
        const posLoc = gl.getAttribLocation(fogProgram, 'a_position');
        gl.bindBuffer(gl.ARRAY_BUFFER, fogPositionBuffer);
        gl.enableVertexAttribArray(posLoc);
        gl.vertexAttribPointer(posLoc, 2, gl.FLOAT, false, 0, 0);

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
        const dt = Math.min(2.5, (time - lastTime || 16.6) / 16.6);
        lastTime = time;

        // Storm logic
        if (time >= nextStormTime && !stormActive) {
            stormActive = true;
            stormEndTime = time + STORM_DURATION;
            nextStormTime = time + STORM_DURATION + rand(IDLE_MIN, IDLE_MAX);
        }
        if (stormActive && time >= stormEndTime) {
            stormActive = false;
        }

        // WebGL fog (rendered first, behind particles)
        renderFog(time);

        // 2D particles
        ctx.clearRect(0, 0, width, height);

        drawClouds(width, height, time);
        updateClouds(dt, width, height);

        drawGroundPile(width, height);
        drawGroundParticles();

        updateAndDrawTrails(dt, time, width, height);
        updateAndDrawFlying(dt, time, width, height);

        if (stormActive) {
            whirlGroundSand(width, height);
            erodeGroundDuringStorm();
            drawStormHaze(width, height, time);
        } else {
            decayGround();
        }

        animationId = window.requestAnimationFrame(render);
    }

    function start() {
        if (active || !shouldRun()) return;
        ensureCanvas();
        if (!ctx) return;

        // Try to init WebGL fog (graceful fallback if it fails)
        if (initFogGL()) {
            fogActive = true;
        }

        active = true;
        canvas.style.display = 'block';
        if (fogCanvas) fogCanvas.style.display = 'block';
        resize();
        nextStormTime = performance.now() + rand(8000, 20000);
        lastTime = 0;
        animationId = window.requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
        if (canvas) canvas.style.display = 'none';
        if (fogCanvas) fogCanvas.style.display = 'none';
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

    window.AuraGoSandstorm = { start, stop, sync };
    init();
})();
