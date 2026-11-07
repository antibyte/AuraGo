/**
 * Theme Optics — shared WebGL backdrop engine.
 *
 * One renderer, many themes. The engine draws a single fullscreen quad with a
 * fragment shader picked from a small family of "optics" (aurora, caustic,
 * lattice, plasma, ...). Every theme only supplies a recipe: which optics to
 * run, the palette, the speed, the strength and how the layer composites.
 *
 * The engine is deliberately defensive:
 *   - no WebGL / no context  -> it stays invisible, the theme still works
 *   - prefers-reduced-motion -> it renders one static frame and stops the loop
 *   - hidden tab             -> the loop parks instead of burning frames
 *   - lost context           -> it stops instead of throwing every frame
 *
 * It owns no DOM policy beyond the canvas it creates: the caller passes the
 * container, decides the CSS and picks the scroll behaviour. Both the chat page
 * and the virtual desktop use it that way.
 */
(function () {
    'use strict';

    const VERTEX_SHADER = `
        attribute vec2 a_position;
        void main() {
            gl_Position = vec4(a_position, 0.0, 1.0);
        }
    `;

    // Shared noise helpers. Every optics shader includes this prelude, so the
    // recipes can reuse one noise vocabulary and stay visually consistent.
    const NOISE_PRELUDE = `
        float hash11(float p) {
            p = fract(p * 0.1031);
            p *= p + 33.33;
            p *= p + p;
            return fract(p);
        }

        float hash21(vec2 p) {
            vec3 h = fract(vec3(p.xyx) * 0.1031);
            h += vec3(dot(h, h.yzx + 33.33));
            return fract((h.x + h.y) * h.z);
        }

        vec2 hash22(vec2 p) {
            vec3 h = fract(vec3(p.xyx) * vec3(0.1031, 0.1030, 0.0973));
            h += vec3(dot(h, h.yzx + 33.33));
            return fract((h.xx + h.yz) * h.zy);
        }

        float vnoise(vec2 p) {
            vec2 i = floor(p);
            vec2 f = fract(p);
            f = f * f * (3.0 - 2.0 * f);
            float a = hash21(i);
            float b = hash21(i + vec2(1.0, 0.0));
            float c = hash21(i + vec2(0.0, 1.0));
            float d = hash21(i + vec2(1.0, 1.0));
            return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
        }

        float fbm(vec2 p) {
            float v = 0.0;
            float a = 0.5;
            for (int i = 0; i < 5; i++) {
                v += a * vnoise(p);
                p = mat2(0.80, -0.60, 0.60, 0.80) * p * 2.02;
                a *= 0.5;
            }
            return v;
        }

        float worley(vec2 p) {
            vec2 i = floor(p);
            vec2 f = fract(p);
            float d = 1.0;
            for (int y = -1; y <= 1; y++) {
                for (int x = -1; x <= 1; x++) {
                    vec2 g = vec2(float(x), float(y));
                    vec2 o = hash22(i + g);
                    vec2 r = g + o - f;
                    d = min(d, dot(r, r));
                }
            }
            return sqrt(d);
        }

        // The pointer nudges the field a little, so the light feels connected
        // to the cursor instead of running on a timer alone.
        vec2 pointerShift(vec2 p, float amount) {
            return p + (u_pointer.xy - vec2(0.5)) * amount;
        }
    `;

    /**
     * The optics library. Each optics is a fragment shader body that writes a
     * colour into `col` and its coverage into `alpha`.
     *
     * Uniforms every optics may use:
     *   u_time      seconds since start
     *   u_res      canvas size in device pixels
     *   u_pointer  pointer position in 0..1 canvas space
     *   u_color_a  primary theme colour
     *   u_color_b  secondary theme colour
     *   u_color_c  accent theme colour
     *   u_intensity overall strength, 0..1
     *
     * The three colours are separate vec3 uniforms on purpose: ESSL 1.00 has
     * no implicit float-to-vector conversion, so a single vec3 palette would
     * have to be addressed component by component and could not be mixed with
     * vec3 colours directly.
     */
    const OPTICS = {
        // Slow drifting colour bands. The calm default for paper and light themes.
        aurora: `
            float t = u_time * 0.06;
            vec2 p = uv * vec2(u_res.x / u_res.y, 1.0) * 1.6;
            p = pointerShift(p, 0.16);
            float bands = 0.0;
            for (int i = 0; i < 3; i++) {
                float fi = float(i);
                float speed = 0.55 + fi * 0.32;
                float wobble = fbm(p * (1.1 + fi * 0.55) + vec2(t * speed, -t * speed * 0.6));
                float wave = sin((p.y * (2.1 + fi * 1.35)) + wobble * 3.4 + t * (0.9 + fi * 0.4));
                bands += smoothstep(0.05, 0.95, wave * 0.5 + 0.5) / (1.0 + fi);
            }
            bands = clamp(bands, 0.0, 1.0);
            vec3 c = mix(u_color_a, u_color_b, smoothstep(0.1, 0.9, bands));
            c = mix(c, u_color_c, smoothstep(0.55, 1.0, bands) * 0.65);
            float glow = smoothstep(0.35, 1.0, bands) * 0.5;
            col = c;
            alpha = (0.18 + glow * 0.42) * u_intensity;
        `,

        // Light refracted through moving water.
        caustic: `
            vec2 p = uv * vec2(u_res.x / u_res.y, 1.0) * 3.2;
            p = pointerShift(p, 0.22);
            float t = u_time * 0.22;
            float c1 = worley(p + vec2(t, t * 0.6));
            float c2 = worley(p * 1.7 - vec2(t * 0.8, t * 0.35));
            float lines = 1.0 - smoothstep(0.0, 0.42, min(c1, c2));
            float shimmer = fbm(p * 1.4 + vec2(t * 0.4, -t * 0.3));
            float energy = lines * (0.55 + shimmer * 0.75);
            col = mix(u_color_a, u_color_c, smoothstep(0.2, 0.9, energy));
            alpha = energy * 0.38 * u_intensity;
        `,

        // A receding grid with a glowing horizon.
        lattice: `
            vec2 p = uv * vec2(u_res.x / u_res.y, 1.0);
            p.x += (u_pointer.x - 0.5) * 0.06;
            float horizon = 0.52;
            float depth = max(0.0001, p.y - horizon);
            vec2 grid = vec2(p.x * 9.0 / depth, u_time * 1.6 / depth);
            vec2 cell = abs(fract(grid) - 0.5);
            float line = smoothstep(0.46, 0.5, max(cell.x, cell.y));
            float fade = smoothstep(0.55, 0.03, depth);
            float sky = smoothstep(0.0, 0.55, horizon - p.y);
            float pulse = 0.6 + 0.4 * sin(u_time * 1.7 + p.x * 4.0);
            vec3 c = mix(u_color_b, u_color_c, fade);
            col = c * (0.7 + pulse * 0.5);
            alpha = (line * fade * 0.75 + sky * 0.22) * u_intensity;
        `,

        // Interfering waves, for synth and plasma themes.
        plasma: `
            vec2 p = uv * vec2(u_res.x / u_res.y, 1.0) * 4.0;
            p = pointerShift(p, 0.3);
            float t = u_time * 0.7;
            float v = sin(p.x * 2.1 + t);
            v += sin(p.y * 2.7 - t * 0.8);
            v += sin((p.x + p.y) * 1.9 + t * 0.5);
            v += sin(length(p - vec2(0.5)) * 3.4 - t * 1.2);
            v = v * 0.25 + 0.5;
            vec3 c = mix(u_color_a, u_color_b, v);
            c = mix(c, u_color_c, smoothstep(0.55, 1.0, v));
            float core = smoothstep(0.62, 1.0, v);
            col = c;
            alpha = (0.16 + core * 0.42) * u_intensity;
        `,

        // Slow drifting dust and motes.
        motes: `
            vec2 p = uv * vec2(u_res.x / u_res.y, 1.0);
            p = pointerShift(p, 0.18);
            float t = u_time * 0.16;
            float acc = 0.0;
            for (int i = 0; i < 3; i++) {
                float fi = float(i);
                vec2 q = p * (2.6 + fi * 2.3) + vec2(t * (0.4 + fi * 0.3), -t * (0.25 + fi * 0.2));
                vec2 id = floor(q);
                vec2 gv = fract(q) - 0.5;
                vec2 off = hash22(id + fi * 13.0) - 0.5;
                float d = length(gv - off * 0.7);
                float m = smoothstep(0.16, 0.0, d) * (0.4 + hash11(id.x + id.y + fi) * 0.6);
                acc += m / (1.0 + fi);
            }
            acc = clamp(acc, 0.0, 1.0);
            col = mix(u_color_a, u_color_c, acc);
            alpha = acc * 0.34 * u_intensity;
        `,

        // Faceted crystal, for voxel and prism themes.
        facet: `
            vec2 p = uv * vec2(u_res.x / u_res.y, 1.0) * 3.4;
            p = pointerShift(p, 0.12);
            float t = u_time * 0.24;
            float d = worley(p + vec2(sin(t) * 0.25, cos(t * 0.8) * 0.25));
            float edge = smoothstep(0.34, 0.06, d);
            float body = smoothstep(0.5, 0.12, d);
            float shimmer = fbm(p * 2.2 - vec2(t * 0.5, t * 0.3));
            vec3 c = mix(u_color_a, u_color_b, shimmer);
            c = mix(c, u_color_c, edge);
            col = c;
            alpha = (body * 0.3 + edge * 0.5) * u_intensity;
        `,

        // Falling data rain, for the cyber themes.
        rain: `
            vec2 p = uv * vec2(u_res.x / u_res.y, 1.0);
            p.x += (u_pointer.x - 0.5) * 0.04;
            float acc = 0.0;
            for (int i = 0; i < 2; i++) {
                float fi = float(i);
                vec2 q = vec2(p.x * (24.0 + fi * 16.0), p.y * (2.2 + fi * 1.4) + u_time * (0.55 + fi * 0.35));
                vec2 id = floor(q);
                float h = hash21(id);
                if (h < 0.72) continue;
                float tail = smoothstep(0.0, 0.35, fract(q.y)) * smoothstep(1.0, 0.55, fract(q.y));
                float head = smoothstep(0.09, 0.0, abs(fract(q.y) - 0.02));
                acc += (tail * 0.6 + head) * (0.35 + h * 0.65) / (1.0 + fi);
            }
            acc = clamp(acc, 0.0, 1.0);
            col = mix(u_color_b, u_color_c, acc);
            alpha = acc * 0.4 * u_intensity;
        `,

        // Hot updraught embers, for the desert and ember themes.
        ember: `
            vec2 p = uv * vec2(u_res.x / u_res.y, 1.0);
            p = pointerShift(p, 0.14);
            float t = u_time * 0.3;
            float acc = 0.0;
            for (int i = 0; i < 2; i++) {
                float fi = float(i);
                vec2 q = vec2(p.x * (11.0 + fi * 7.0), p.y * (7.0 + fi * 5.0) + t * (0.9 + fi * 0.5));
                vec2 id = floor(q);
                vec2 gv = fract(q) - 0.5;
                float h = hash21(id);
                if (h < 0.86) continue;
                float d = length(gv - (hash22(id + fi * 7.0) - 0.5) * 0.6);
                float m = smoothstep(0.2, 0.0, d);
                float life = fract(h * 7.3 + u_time * (0.2 + fi * 0.12));
                acc += m * (1.0 - life) / (1.0 + fi);
            }
            acc = clamp(acc, 0.0, 1.0);
            col = mix(u_color_a, u_color_c, acc);
            alpha = acc * 0.42 * u_intensity;
        `
    };

    const PALETTE_KEYS = ['primary', 'secondary', 'accent'];

    function clamp01(value) {
        const n = Number(value);
        if (!isFinite(n)) return 0;
        return n < 0 ? 0 : n > 1 ? 1 : n;
    }

    function parseColor(value) {
        const raw = String(value || '').trim();
        if (!raw) return null;
        const hex = raw.replace('#', '');
        if (hex.length === 3) {
            const r = parseInt(hex[0] + hex[0], 16);
            const g = parseInt(hex[1] + hex[1], 16);
            const b = parseInt(hex[2] + hex[2], 16);
            if (isFinite(r) && isFinite(g) && isFinite(b)) return [r / 255, g / 255, b / 255];
            return null;
        }
        if (hex.length === 6) {
            const r = parseInt(hex.slice(0, 2), 16);
            const g = parseInt(hex.slice(2, 4), 16);
            const b = parseInt(hex.slice(4, 6), 16);
            if (isFinite(r) && isFinite(g) && isFinite(b)) return [r / 255, g / 255, b / 255];
        }
        const rgb = raw.match(/rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)/i);
        if (rgb) {
            return [clamp01(Number(rgb[1]) / 255), clamp01(Number(rgb[2]) / 255), clamp01(Number(rgb[3]) / 255)];
        }
        return null;
    }

    function compileShader(gl, type, source) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            const log = gl.getShaderInfoLog(shader);
            gl.deleteShader(shader);
            throw new Error('Theme optics shader failed: ' + log);
        }
        return shader;
    }

    function buildFragmentSource(optics) {
        return `
            #ifdef GL_FRAGMENT_PRECISION_HIGH
            precision highp float;
            #else
            precision mediump float;
            #endif

            uniform float u_time;
            uniform vec2 u_res;
            uniform vec3 u_pointer;
            uniform vec3 u_color_a;
            uniform vec3 u_color_b;
            uniform vec3 u_color_c;
            uniform float u_intensity;

            ${NOISE_PRELUDE}

            void main() {
                vec2 uv = gl_FragCoord.xy / u_res;
                vec3 col = vec3(0.0);
                float alpha = 0.0;
                ${optics}
                alpha = clamp(alpha, 0.0, 1.0);
                if (alpha <= 0.002) discard;
                gl_FragColor = vec4(col, alpha);
            }
        `;
    }

    /**
     * Creates a renderer bound to a container element.
     *
     * @param {object} options
     * @param {HTMLElement} options.container  element the canvas is appended to
     * @param {object} options.recipe          { optics, palette, intensity, speed, blend }
     * @param {string} [options.canvasId]      id for the canvas
     * @param {string} [options.className]     extra class for the canvas
     * @param {number} [options.maxDpr]        device pixel ratio ceiling
     * @param {boolean} [options.pinScroll]    keep the canvas aligned while the container scrolls
     * @returns {object|null} renderer handle or null when WebGL is unavailable
     */
    function createRenderer(options) {
        const container = options && options.container;
        if (!container || typeof container.appendChild !== 'function') return null;

        const canvas = document.createElement('canvas');
        if (options.canvasId) canvas.id = options.canvasId;
        canvas.setAttribute('aria-hidden', 'true');
        canvas.className = 'aurago-theme-optics' + (options.className ? ' ' + options.className : '');

        let gl = null;
        try {
            gl = canvas.getContext('webgl', {
                alpha: true,
                antialias: false,
                depth: false,
                stencil: false,
                premultipliedAlpha: false,
                powerPreference: 'low-power',
                preserveDrawingBuffer: false
            }) || canvas.getContext('experimental-webgl', { alpha: true, antialias: false });
        } catch (_) {
            gl = null;
        }
        if (!gl) {
            canvas.remove();
            return null;
        }

        // The layer joins the container as its first child, so it sits behind
        // the content the container already holds.
        container.insertBefore(canvas, container.firstChild);

        let program = null;
        let buffer = null;
        let programKey = '';
        let currentOptics = '';
        // Uniform locations belong to the program they came from, so the cache
        // is thrown away whenever a new program is linked.
        let uniforms = {};

        function buildProgram(opticsKey) {
            const source = OPTICS[opticsKey];
            if (!source) return false;
            try {
                const vs = compileShader(gl, gl.VERTEX_SHADER, VERTEX_SHADER);
                const fs = compileShader(gl, gl.FRAGMENT_SHADER, buildFragmentSource(source));
                const prog = gl.createProgram();
                gl.attachShader(prog, vs);
                gl.attachShader(prog, fs);
                gl.bindAttribLocation(prog, 0, 'a_position');
                gl.linkProgram(prog);
                gl.deleteShader(vs);
                gl.deleteShader(fs);
                if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
                    const log = gl.getProgramInfoLog(prog);
                    gl.deleteProgram(prog);
                    throw new Error('Theme optics program failed: ' + log);
                }
                program = prog;
                programKey = opticsKey;
                uniforms = {};
                return true;
            } catch (err) {
                console.warn('[AuraGo] Theme optics program failed', err);
                window.AuraThemeOptics.lastError = String(err && err.message ? err.message : err);
                program = null;
                programKey = '';
                return false;
            }
        }

        function ensureProgram(opticsKey) {
            if (!OPTICS[opticsKey]) return false;
            if (programKey === opticsKey && program) return true;
            if (program) gl.deleteProgram(program);
            return buildProgram(opticsKey);
        }

        buffer = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
        gl.enableVertexAttribArray(0);
        gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);

        const maxDpr = Math.min(Number(options.maxDpr) || 1.5, 2);
        const motionQuery = window.matchMedia ? window.matchMedia('(prefers-reduced-motion: reduce)') : null;
        const pinScroll = options.pinScroll === true;
        let width = 0;
        let height = 0;
        let running = false;
        let paused = false;
        let disposed = false;
        let frame = 0;
        let startTime = 0;
        let elapsed = 0;
        let lastFrame = 0;
        let intensity = 0;
        let speed = 1;
        let palette = [[1, 1, 1], [1, 1, 1], [1, 1, 1]];
        const pointer = { x: 0.5, y: 0.5 };

        function syncScroll() {
            if (!pinScroll) return;
            const top = container.scrollTop || 0;
            canvas.style.top = top + 'px';
        }

        function resize() {
            if (disposed) return;
            const rect = container.getBoundingClientRect();
            const w = Math.max(1, Math.round((rect.width || container.clientWidth || 1) * maxDpr));
            const h = Math.max(1, Math.round((rect.height || container.clientHeight || 1) * maxDpr));
            if (w === width && h === height) return;
            width = w;
            height = h;
            canvas.width = w;
            canvas.height = h;
            if (gl) gl.viewport(0, 0, w, h);
        }

        function readPalette(next) {
            const source = (next && next.palette) || {};
            const out = [];
            for (let i = 0; i < PALETTE_KEYS.length; i++) {
                const parsed = parseColor(source[PALETTE_KEYS[i]]);
                out.push(parsed || [1, 1, 1]);
            }
            return out;
        }

        function applyRecipe(next) {
            if (!next) {
                intensity = 0;
                currentOptics = '';
                canvas.style.display = 'none';
                canvas.classList.remove('is-ready');
                return;
            }
            canvas.style.display = '';
            intensity = clamp01(next.intensity);
            speed = Number(next.speed) || 1;
            palette = readPalette(next);
            canvas.style.mixBlendMode = next.blend || 'screen';
            const opticsKey = String(next.optics || '');
            if (opticsKey !== currentOptics) {
                currentOptics = opticsKey;
                // A new optics restarts the fade-in, so the change reads as a
                // deliberate switch instead of a silent repaint.
                canvas.classList.remove('is-ready');
                if (opticsKey) ensureProgram(opticsKey);
            }
        }

        function draw() {
            if (disposed || !gl || !program || !currentOptics) return;
            if (width !== canvas.width || height !== canvas.height) resize();
            if (!width || !height) return;

            gl.useProgram(program);
            gl.viewport(0, 0, width, height);
            gl.clearColor(0, 0, 0, 0);
            gl.clear(gl.COLOR_BUFFER_BIT);

            if (!uniforms.u_time) {
                uniforms.u_time = gl.getUniformLocation(program, 'u_time');
                uniforms.u_res = gl.getUniformLocation(program, 'u_res');
                uniforms.u_pointer = gl.getUniformLocation(program, 'u_pointer');
                uniforms.u_color_a = gl.getUniformLocation(program, 'u_color_a');
                uniforms.u_color_b = gl.getUniformLocation(program, 'u_color_b');
                uniforms.u_color_c = gl.getUniformLocation(program, 'u_color_c');
                uniforms.u_intensity = gl.getUniformLocation(program, 'u_intensity');
            }

            gl.uniform1f(uniforms.u_time, elapsed * speed);
            gl.uniform2f(uniforms.u_res, width, height);
            gl.uniform3f(uniforms.u_pointer, pointer.x, pointer.y, 0);
            gl.uniform3f(uniforms.u_color_a, palette[0][0], palette[0][1], palette[0][2]);
            gl.uniform3f(uniforms.u_color_b, palette[1][0], palette[1][1], palette[1][2]);
            gl.uniform3f(uniforms.u_color_c, palette[2][0], palette[2][1], palette[2][2]);
            gl.uniform1f(uniforms.u_intensity, intensity);

            gl.enable(gl.BLEND);
            gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
            gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
            gl.enableVertexAttribArray(0);
            gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
            gl.drawArrays(gl.TRIANGLES, 0, 3);

            // The first painted frame reveals the layer with its fade-in.
            if (!canvas.classList.contains('is-ready')) canvas.classList.add('is-ready');
        }

        function tick(timeMs) {
            if (disposed || !running) return;
            frame = window.requestAnimationFrame(tick);
            if (paused) return;
            if (!startTime) {
                startTime = timeMs;
                lastFrame = timeMs;
            }
            const delta = Math.min(0.1, (timeMs - lastFrame) / 1000);
            lastFrame = timeMs;
            elapsed += delta;
            draw();
        }

        function renderStatic() {
            if (disposed || !gl) return;
            resize();
            draw();
        }

        function start() {
            if (disposed || running) return;
            running = true;
            if (motionQuery && motionQuery.matches) {
                // Reduced motion: one still frame instead of a moving loop.
                renderStatic();
                return;
            }
            frame = window.requestAnimationFrame(tick);
        }

        function stop() {
            running = false;
            if (frame) {
                window.cancelAnimationFrame(frame);
                frame = 0;
            }
        }

        function setPaused(next) {
            paused = !!next;
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            stop();
            if (resizeObserver) resizeObserver.disconnect();
            else window.removeEventListener('resize', resize);
            if (pinScroll) container.removeEventListener('scroll', syncScroll);
            container.removeEventListener('pointermove', onPointerMove);
            document.removeEventListener('visibilitychange', onVisibility);
            if (motionQuery && typeof motionQuery.removeEventListener === 'function') {
                motionQuery.removeEventListener('change', onMotionChange);
            }
            if (gl) {
                if (buffer) gl.deleteBuffer(buffer);
                if (program) gl.deleteProgram(program);
                const lose = gl.getExtension('WEBGL_lose_context');
                if (lose && typeof lose.loseContext === 'function') lose.loseContext();
            }
            if (canvas.parentNode) canvas.parentNode.removeChild(canvas);
        }

        function onPointerMove(event) {
            const rect = container.getBoundingClientRect();
            if (!rect.width || !rect.height) return;
            pointer.x = (event.clientX - rect.left) / rect.width;
            pointer.y = 1 - (event.clientY - rect.top) / rect.height;
        }

        const resizeObserver = typeof ResizeObserver === 'function'
            ? new ResizeObserver(() => { resize(); if (!running) renderStatic(); })
            : null;
        if (resizeObserver) resizeObserver.observe(container);
        else window.addEventListener('resize', resize, { passive: true });

        if (pinScroll) {
            container.addEventListener('scroll', syncScroll, { passive: true });
            syncScroll();
        }
        container.addEventListener('pointermove', onPointerMove, { passive: true });

        const onVisibility = () => setPaused(document.hidden);
        document.addEventListener('visibilitychange', onVisibility);

        const onMotionChange = () => {
            if (motionQuery && motionQuery.matches) {
                stop();
                renderStatic();
            } else {
                start();
            }
        };
        if (motionQuery && typeof motionQuery.addEventListener === 'function') {
            motionQuery.addEventListener('change', onMotionChange);
        }

        resize();

        return {
            canvas,
            applyRecipe,
            start,
            stop,
            setPaused,
            dispose,
            resize,
            renderStatic,
            isRunning: () => running,
            getRecipe: () => ({ optics: currentOptics, intensity })
        };
    }

    window.AuraThemeOptics = {
        createRenderer,
        optics: Object.keys(OPTICS),
        hasOptics: name => Object.prototype.hasOwnProperty.call(OPTICS, name),
        // The last shader error, for debugging a theme that stays dark.
        lastError: ''
    };
})();
