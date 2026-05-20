(function () {
    'use strict';

    let canvas;
    let gl;
    let program;
    let animationId = null;
    let active = false;
    let uniforms = {};
    let chatBox;
    let resizeObserver = null;

    const VERT = `
        attribute vec2 a_pos;
        varying vec2 v_uv;

        void main() {
            v_uv = a_pos * 0.5 + 0.5;
            gl_Position = vec4(a_pos, 0.0, 1.0);
        }
    `;

    // ── Premium Cinematic Radar Fragment Shader ──────────────────────────
    const FRAG = `
        precision highp float;
        varying vec2 v_uv;
        uniform float u_time;
        uniform vec2 u_res;

        const float PI  = 3.1415926;
        const float TAU = 6.2831853;
        const vec2  CTR = vec2(0.5, 0.5);

        /* ── Utility ────────────────────────────────────────────────── */

        float hash(vec2 p) {
            return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
        }

        float hash1(float p) {
            return fract(sin(p * 127.1) * 43758.5453);
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

        float angleDist(float a, float b) {
            return abs(mod(a - b + PI, TAU) - PI);
        }

        float sweepLag(float target, float beam) {
            return mod(beam - target + TAU, TAU);
        }

        /* ── Moving Target Trajectories ─────────────────────────────── */

        vec2 orbitTarget(float t, float r, float spd, float ph, float wobble) {
            float a = t * spd + ph + sin(t * spd * 0.41 + ph) * wobble;
            float drift = r + sin(t * spd * 0.73 + ph * 1.9) * 0.06;
            return vec2(cos(a) * drift, sin(a) * drift * 0.72);
        }

        // Evasive target — jinks erratically
        vec2 evasiveTarget(float t, float r, float ph) {
            float a = t * 0.19 + ph;
            a += sin(t * 1.7 + ph) * 0.35 + cos(t * 2.9 + ph * 0.7) * 0.18;
            float d = r + sin(t * 0.43 + ph * 2.1) * 0.09 + cos(t * 1.1) * 0.04;
            return vec2(cos(a) * d, sin(a) * d * 0.68);
        }

        // Inbound threat — spiraling inward
        vec2 inboundTarget(float t, float startR, float ph) {
            float spiral = startR - mod(t * 0.022 + ph * 0.1, startR) * 0.7;
            spiral = max(spiral, 0.08);
            float a = t * 0.35 + ph + sin(t * 0.6 + ph) * 0.15;
            return vec2(cos(a) * spiral, sin(a) * spiral * 0.75);
        }

        // Patrol — figure-8 / lissajous
        vec2 patrolTarget(float t, float r, float ph) {
            float a = t * 0.14 + ph;
            return vec2(
                cos(a) * r + cos(a * 2.0 + ph) * r * 0.25,
                (sin(a * 2.0) * r * 0.55 + sin(a * 0.7 + ph) * r * 0.12) * 0.72
            );
        }

        /* ── Target Rendering ───────────────────────────────────────── */

        float renderTarget(vec2 p, vec2 tgt, float beam, float sc, float glow) {
            float dist = length(p - tgt);
            // Main body — sharp bright core
            float body = smoothstep(0.028 * sc, 0.0, dist);
            // Outer halo bloom
            float halo = exp(-dist * dist * (110.0 / max(sc, 0.15)));

            float tAngle = atan(tgt.y, tgt.x);
            float hitDelta = angleDist(tAngle, beam);
            float hit = smoothstep(0.14, 0.0, hitDelta);

            float lag = sweepLag(tAngle, beam);
            // Cinematic afterglow — long bright tail that fades
            float afterglow = exp(-lag * 5.5) * step(lag, 1.2);
            // Secondary dim persistence
            float persist = exp(-lag * 1.8) * step(lag, 3.0) * 0.15;

            // Ray track line from center to target when lit
            float rayTrack = smoothstep(0.018, 0.0, angleDist(atan(p.y, p.x), tAngle));
            float rangeTrack = exp(-80.0 * abs(length(p) - length(tgt)));
            float trail = rayTrack * rangeTrack * (hit * 0.5 + afterglow * 0.3);

            float brightness = hit * 5.0 + afterglow * 2.2 + persist;
            return body * (0.6 + brightness) +
                   halo * (0.08 + hit * glow + afterglow * 0.5) +
                   trail;
        }

        // Threat reticle — pulsing diamond lock-on indicator
        float threatReticle(vec2 p, vec2 tgt, float beam, float t) {
            vec2 d = p - tgt;
            float dist = length(d);
            float diamond = abs(d.x) + abs(d.y * 1.4);
            float pulse = 0.5 + 0.5 * sin(t * 6.0);
            float sz = 0.05 + pulse * 0.012;
            float ring = smoothstep(sz + 0.008, sz, diamond) * smoothstep(sz - 0.012, sz - 0.004, diamond);

            // Corner brackets
            float bx = step(abs(abs(d.x) - sz * 0.9), 0.006) * step(abs(d.y), sz * 0.35);
            float by = step(abs(abs(d.y * 1.4) - sz * 0.9), 0.006) * step(abs(d.x), sz * 0.35);

            float tAngle = atan(tgt.y, tgt.x);
            float lag = sweepLag(tAngle, beam);
            float vis = exp(-lag * 3.0) * step(lag, 2.5) + 0.15;

            return (ring * 0.7 + (bx + by) * 0.9) * vis * (0.7 + pulse * 0.3);
        }

        /* ── Main ───────────────────────────────────────────────────── */

        void main() {
            vec2 uv = v_uv;
            float aspect = u_res.x / max(u_res.y, 1.0);
            vec2 p = vec2((uv.x - CTR.x) * 2.0 * aspect, (uv.y - CTR.y) * 2.0);
            float r = length(p);
            float pAngle = atan(p.y, p.x);
            float t = u_time;

            // ── Sweep Beam ──────────────────────────────────────────
            float beamAngle = fract(t * 0.07) * TAU - PI;
            float beamDelta = angleDist(pAngle, beamAngle);
            float reach = smoothstep(1.18, 0.06, r);

            // Multi-layered beam for cinematic bloom
            float beamCore    = exp(-140.0 * beamDelta);
            float beamMid     = exp(-40.0 * beamDelta);
            float beamOuter   = exp(-12.0 * beamDelta);
            float beamBloom   = exp(-5.0 * beamDelta);

            float beam = (beamCore * 1.0 + beamMid * 0.55 + beamOuter * 0.25 + beamBloom * 0.08) * reach;

            // Afterglow trail behind the beam — cinema-grade fade
            float trailLag = mod(beamAngle - pAngle + TAU, TAU);
            float trailGlow = exp(-trailLag * 2.8) * step(trailLag, 2.5) * reach;
            float trailDim  = exp(-trailLag * 0.7) * step(trailLag, 5.0) * reach * 0.12;

            // ── Range Rings ─────────────────────────────────────────
            float ringBase = smoothstep(0.985, 1.0, abs(sin(r * 38.0))) * smoothstep(1.16, 0.06, r);
            // Animated pulse ring expanding outward
            float pulseR1 = mod(t * 0.18, 1.3);
            float pulseR2 = mod(t * 0.18 + 0.65, 1.3);
            float pulse1 = exp(-50.0 * abs(r - pulseR1)) * smoothstep(1.2, 0.0, pulseR1) * 0.6;
            float pulse2 = exp(-50.0 * abs(r - pulseR2)) * smoothstep(1.2, 0.0, pulseR2) * 0.35;

            // Major range rings — thicker, brighter at key distances
            float majorRing = 0.0;
            majorRing += exp(-600.0 * abs(r - 0.25)) * 0.5;
            majorRing += exp(-600.0 * abs(r - 0.5))  * 0.6;
            majorRing += exp(-600.0 * abs(r - 0.75)) * 0.5;
            majorRing += exp(-600.0 * abs(r - 1.0))  * 0.7;
            majorRing *= smoothstep(1.16, 0.06, r);

            // ── Crosshairs ──────────────────────────────────────────
            float cross =
                exp(-1200.0 * abs(p.x)) * smoothstep(1.1, 0.1, abs(p.y)) +
                exp(-1200.0 * abs(p.y)) * smoothstep(1.1, 0.1, abs(p.x));
            // Tick marks along axes
            float tickX = smoothstep(0.994, 1.0, abs(sin(p.x * 80.0))) * exp(-800.0 * abs(p.y)) * step(r, 1.1);
            float tickY = smoothstep(0.994, 1.0, abs(sin(p.y * 80.0))) * exp(-800.0 * abs(p.x)) * step(r, 1.1);

            // ── Compass Rose / Bearing Marks ────────────────────────
            float compass = 0.0;
            for (int i = 0; i < 36; i++) {
                float ca = float(i) * TAU / 36.0 - PI;
                float ad = angleDist(pAngle, ca);
                float thick = (mod(float(i), 9.0) == 0.0) ? 0.004 : 0.002;
                float len = (mod(float(i), 9.0) == 0.0) ? 0.08 : 0.04;
                compass += exp(-ad * ad / (thick * thick)) *
                           smoothstep(1.02 - len, 1.02, r) *
                           smoothstep(1.12, 1.02, r);
            }

            // ── Noise / Static / Grain ──────────────────────────────
            float grain = (hash(uv * u_res + t * 7.3) - 0.5) * 0.04;
            float staticNoise = noise(uv * vec2(120.0, 80.0) + t * 3.0) * 0.03 * reach;

            // Electronic warfare interference zones
            float ewAngle1 = sin(t * 0.3) * 0.8 + 1.2;
            float ewAngle2 = cos(t * 0.23 + 2.0) * 0.6 - 0.5;
            float ew1 = exp(-8.0 * angleDist(pAngle, ewAngle1)) * step(0.3, r) * step(r, 0.85) *
                        (0.3 + 0.7 * noise(p * 40.0 + t * 2.0)) * 0.12;
            float ew2 = exp(-6.0 * angleDist(pAngle, ewAngle2)) * step(0.4, r) * step(r, 0.7) *
                        (0.3 + 0.7 * noise(p * 55.0 - t * 1.5)) * 0.08;

            // ── Particle Debris Field ───────────────────────────────
            float debris = 0.0;
            for (int i = 0; i < 20; i++) {
                float fi = float(i);
                float dAngle = hash1(fi * 3.7) * TAU + t * (0.03 + hash1(fi * 1.1) * 0.04) * (hash1(fi * 2.2) > 0.5 ? 1.0 : -1.0);
                float dR = 0.15 + hash1(fi * 5.3) * 0.8;
                vec2 dp = vec2(cos(dAngle) * dR, sin(dAngle) * dR * 0.72);
                float dd = length(p - dp);
                float dBright = smoothstep(0.012, 0.0, dd);
                float dAngleVal = atan(dp.y, dp.x);
                float dLag = sweepLag(dAngleVal, beamAngle);
                float dVis = exp(-dLag * 4.0) * step(dLag, 1.5);
                debris += dBright * (0.15 + dVis * 0.85);
            }

            // ── Targets ─────────────────────────────────────────────
            // Friendlies (green)
            vec2 tA = orbitTarget(t, 0.34, 0.21, 0.5, 0.18);
            vec2 tB = patrolTarget(t, 0.52, 2.35);
            vec2 tC = orbitTarget(t, 0.72, 0.14, 4.1, 0.12);

            float friendlies =
                renderTarget(p, tA, beamAngle, 0.85, 1.2) +
                renderTarget(p, tB, beamAngle, 1.0, 1.0) +
                renderTarget(p, tC, beamAngle, 0.95, 0.9);

            // Threats (red) — hostile inbound
            vec2 tD = evasiveTarget(t, 0.58, 5.45);
            vec2 tE = inboundTarget(t, 0.9, 1.8);

            float threats =
                renderTarget(p, tD, beamAngle, 1.15, 1.8) +
                renderTarget(p, tE, beamAngle, 1.1, 1.6);

            float threatReticles =
                threatReticle(p, tD, beamAngle, t) +
                threatReticle(p, tE, beamAngle, t);

            // Unknown contacts (amber/orange)
            vec2 tF = orbitTarget(t, 0.42, -0.11, 3.2, 0.08);
            vec2 tG = patrolTarget(t, 0.68, 0.8);
            vec2 tH = evasiveTarget(t, 0.3, 6.1);

            float unknowns =
                renderTarget(p, tF, beamAngle, 0.9, 1.0) +
                renderTarget(p, tG, beamAngle, 0.85, 0.8) +
                renderTarget(p, tH, beamAngle, 0.75, 0.7);

            // ── Missile Trajectory Lines (dashed) ───────────────────
            float missile1 = 0.0;
            {
                vec2 src = tE;
                vec2 dir = normalize(-src); // heading toward center
                float along = dot(p - src, dir);
                float perp = length(p - src - dir * along);
                float dash = step(0.5, fract(along * 20.0 - t * 3.0));
                missile1 = exp(-perp * perp * 8000.0) * step(0.0, along) * step(along, length(src)) * dash;
                float tAngle2 = atan(src.y, src.x);
                float mLag = sweepLag(tAngle2, beamAngle);
                missile1 *= (exp(-mLag * 3.5) * step(mLag, 2.0) + 0.08);
            }

            // ── Perimeter Alert Flash ───────────────────────────────
            float perimDist = abs(r - 1.0);
            float perimAlert = exp(-perimDist * perimDist * 2000.0) *
                               (0.3 + 0.7 * step(0.0, sin(t * 4.0))) *
                               smoothstep(0.0, 0.1, sin(pAngle * 8.0 + t * 2.0) * 0.5 + 0.5) * 0.3;

            // ── Data Readout Grid Overlay ────────────────────────────
            float gridX = smoothstep(0.988, 1.0, abs(sin((uv.x + t * 0.006) * u_res.x * 0.026)));
            float gridY = smoothstep(0.99, 1.0, abs(sin((uv.y - t * 0.008) * u_res.y * 0.028)));
            float grid = (gridX + gridY) * 0.12;

            // Moving scan lines
            float hScan = exp(-50.0 * abs(uv.y - fract(t * 0.04)));
            float dScan = exp(-80.0 * abs((uv.x * 0.9 + uv.y * 0.35) - fract(t * 0.035)));

            // Coarse data sparkle
            vec2 sparkCell = floor(uv * vec2(48.0, 32.0) + floor(t * 0.9));
            float sparkle = smoothstep(0.993, 1.0, hash(sparkCell)) * 0.1;

            // ── Vignette ────────────────────────────────────────────
            float vignette = smoothstep(1.6, 0.15, dot(p, p));

            // ── Color Palette ───────────────────────────────────────
            vec3 cCyan    = vec3(0.29, 0.97, 1.0);
            vec3 cGreen   = vec3(0.08, 1.0, 0.48);
            vec3 cBright  = vec3(0.5, 1.0, 0.85);
            vec3 cRed     = vec3(1.0, 0.15, 0.55);
            vec3 cOrange  = vec3(1.0, 0.65, 0.15);
            vec3 cMagenta = vec3(1.0, 0.28, 0.76);
            vec3 cBlue    = vec3(0.4, 0.5, 1.0);
            vec3 cWhite   = vec3(1.0, 0.95, 1.0);

            // ── Ambient Fog ─────────────────────────────────────────
            float pulseSlow = 0.5 + 0.5 * sin(t * 0.5);
            vec3 fog =
                cCyan    * (0.06 + 0.03 * sin(t * 0.7 + uv.y * 9.0)) +
                cMagenta * (0.04 + 0.04 * sin(t * 0.43 + uv.x * 7.0 + 1.4)) +
                cBlue    * (0.05 + 0.03 * sin(t * 0.58 + (uv.x + uv.y) * 6.0));

            // ── Composite ───────────────────────────────────────────
            vec3 color = fog * 0.35;

            // Scan lines
            color += cCyan * hScan * 0.08;
            color += cMagenta * dScan * 0.1;

            // Grid
            color += cBlue * grid * (0.4 + pulseSlow * 0.15);

            // Beam — multi-layered glow
            color += cGreen * beam * 0.5;
            color += cBright * beamCore * reach * 0.35;
            color += vec3(0.2, 1.0, 0.7) * beamBloom * reach * 0.06;

            // Afterglow trail
            color += cGreen * trailGlow * 0.25;
            color += vec3(0.05, 0.6, 0.3) * trailDim;

            // Range rings
            color += cGreen * ringBase * 0.12;
            color += cGreen * majorRing * 0.22;
            color += cCyan * pulse1;
            color += cCyan * pulse2;

            // Crosshairs
            color += cGreen * cross * 0.06;
            color += cGreen * (tickX + tickY) * 0.08;

            // Compass
            color += cCyan * compass * 0.2;

            // Perimeter
            color += cRed * perimAlert;

            // Targets
            color += cGreen * friendlies * 0.32;
            color += cRed * threats * 0.35;
            color += cRed * threatReticles * 0.45;
            color += cOrange * unknowns * 0.28;

            // Missile trajectory
            color += cRed * missile1 * 0.4;

            // Debris
            color += cCyan * debris * 0.2;

            // EW interference
            color += cMagenta * ew1;
            color += vec3(0.6, 0.3, 1.0) * ew2;

            // Sparkle & noise
            color += cWhite * sparkle;
            color += vec3(grain) * 0.5;
            color += vec3(staticNoise) * 0.3;

            // ── Alpha ───────────────────────────────────────────────
            float alpha =
                vignette * 0.16 +
                hScan * 0.02 +
                dScan * 0.025 +
                grid * 0.18 +
                beam * 0.22 +
                trailGlow * 0.12 +
                trailDim * 0.06 +
                ringBase * 0.03 +
                majorRing * 0.06 +
                pulse1 * 0.15 +
                pulse2 * 0.1 +
                cross * 0.015 +
                compass * 0.05 +
                friendlies * 0.15 +
                threats * 0.18 +
                threatReticles * 0.14 +
                unknowns * 0.12 +
                missile1 * 0.12 +
                debris * 0.05 +
                perimAlert * 0.1 +
                ew1 * 0.08 +
                ew2 * 0.06 +
                sparkle * 0.12;

            gl_FragColor = vec4(color, clamp(alpha, 0.0, 0.52));
        }
    `;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'cyberwar' &&
            !prefersReducedMotion() &&
            window.innerWidth >= 768;
    }

    function createShader(type, source) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);

        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            console.warn('[CyberwarShader] Shader compile error:', gl.getShaderInfoLog(shader));
            gl.deleteShader(shader);
            return null;
        }

        return shader;
    }

    function createCanvas() {
        canvas = document.createElement('canvas');
        canvas.id = 'cyberwar-overlay';
        Object.assign(canvas.style, {
            position: 'fixed',
            top: '0',
            left: '0',
            width: '0',
            height: '0',
            pointerEvents: 'none',
            zIndex: '2',
            opacity: '0.88',
            mixBlendMode: 'screen',
            display: 'none'
        });
        document.body.appendChild(canvas);
    }

    function initGL() {
        if (!canvas) createCanvas();

        gl = canvas.getContext('webgl', {
            alpha: true,
            antialias: false,
            premultipliedAlpha: false,
            powerPreference: 'low-power'
        });

        if (!gl) {
            console.warn('[CyberwarShader] WebGL not available');
            return false;
        }

        const vertexShader = createShader(gl.VERTEX_SHADER, VERT);
        const fragmentShader = createShader(gl.FRAGMENT_SHADER, FRAG);
        if (!vertexShader || !fragmentShader) {
            return false;
        }

        program = gl.createProgram();
        gl.attachShader(program, vertexShader);
        gl.attachShader(program, fragmentShader);
        gl.linkProgram(program);

        if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
            console.warn('[CyberwarShader] Program link error:', gl.getProgramInfoLog(program));
            return false;
        }

        gl.useProgram(program);

        const quad = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, quad);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([
            -1, -1,
             1, -1,
            -1,  1,
             1,  1
        ]), gl.STATIC_DRAW);

        const position = gl.getAttribLocation(program, 'a_pos');
        gl.enableVertexAttribArray(position);
        gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);

        uniforms.time = gl.getUniformLocation(program, 'u_time');
        uniforms.resolution = gl.getUniformLocation(program, 'u_res');

        gl.enable(gl.BLEND);
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
        return true;
    }

    function updateBounds() {
        if (!canvas || !chatBox) return false;
        const rect = chatBox.getBoundingClientRect();
        if (!rect.width || !rect.height) {
            canvas.style.width = '0';
            canvas.style.height = '0';
            return false;
        }

        canvas.style.left = `${Math.round(rect.left)}px`;
        canvas.style.top = `${Math.round(rect.top)}px`;
        canvas.style.width = `${Math.round(rect.width)}px`;
        canvas.style.height = `${Math.round(rect.height)}px`;
        return true;
    }

    function resize() {
        if (!canvas || !gl || !updateBounds()) return;
        const dpr = Math.min(window.devicePixelRatio || 1, 2);
        const width = Math.max(1, Math.round(canvas.getBoundingClientRect().width));
        const height = Math.max(1, Math.round(canvas.getBoundingClientRect().height));
        canvas.width = Math.floor(width * dpr);
        canvas.height = Math.floor(height * dpr);
        gl.viewport(0, 0, canvas.width, canvas.height);
    }

    function render(time) {
        if (!active || !gl) return;

        gl.uniform1f(uniforms.time, time * 0.001);
        gl.uniform2f(uniforms.resolution, canvas.width, canvas.height);
        gl.clearColor(0, 0, 0, 0);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
        animationId = window.requestAnimationFrame(render);
    }

    function start() {
        if (active || !shouldRun()) return;
        if (!gl && !initGL()) return;

        active = true;
        canvas.style.display = 'block';
        resize();
        animationId = window.requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
        if (canvas) {
            canvas.style.display = 'none';
        }
    }

    function sync() {
        if (shouldRun()) {
            start();
        } else {
            stop();
        }
    }

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

        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', () => {
            if (active) resize();
            sync();
        });
        window.addEventListener('scroll', () => {
            if (active) resize();
        }, { passive: true });

        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            if (mq.addEventListener) {
                mq.addEventListener('change', sync);
            } else if (mq.addListener) {
                mq.addListener(sync);
            }
        }

        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true,
                attributeFilter: ['data-theme']
            });
        }

        sync();
    }

    window.AuraGoCyberwar = { start, stop, sync };
    init();
})();
