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

    const FRAG = `
        precision mediump float;
        varying vec2 v_uv;
        uniform float u_time;
        uniform vec2 u_res;

        float hash(vec2 p) {
            return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
        }

        float noise(vec2 p) {
            vec2 i = floor(p);
            vec2 f = fract(p);

            float a = hash(i);
            float b = hash(i + vec2(1.0, 0.0));
            float c = hash(i + vec2(0.0, 1.0));
            float d = hash(i + vec2(1.0, 1.0));

            vec2 u = f * f * (3.0 - 2.0 * f);
            return mix(a, b, u.x) +
                (c - a) * u.y * (1.0 - u.x) +
                (d - b) * u.x * u.y;
        }

        float fbm(vec2 p) {
            float value = 0.0;
            float amp = 0.5;
            for (int i = 0; i < 5; i++) {
                value += amp * noise(p);
                p = p * 2.02 + vec2(4.1, 7.3);
                amp *= 0.5;
            }
            return value;
        }

        float ripple(vec2 uv, vec2 center, float radius, float frequency, float speed, float phase) {
            vec2 p = uv - center;
            p.x *= u_res.x / max(u_res.y, 1.0);
            float dist = length(p);
            float envelope = exp(-dist * radius);
            float wave = sin(dist * frequency - u_time * speed + phase);
            float rings = smoothstep(0.42, 0.98, wave) + smoothstep(-1.0, -0.68, wave) * 0.55;
            return rings * envelope;
        }

        void main() {
            vec2 uv = v_uv;
            float t = u_time;

            vec2 c1 = vec2(0.24 + sin(t * 0.07) * 0.05, 0.22 + cos(t * 0.09) * 0.03);
            vec2 c2 = vec2(0.76 + cos(t * 0.05) * 0.04, 0.54 + sin(t * 0.06) * 0.04);
            vec2 c3 = vec2(0.48 + sin(t * 0.04) * 0.03, 0.82 + cos(t * 0.08) * 0.03);
            vec2 c4 = vec2(0.58 + cos(t * 0.06) * 0.03, 0.34 + sin(t * 0.05) * 0.03);

            float r1 = ripple(uv, c1, 6.0, 56.0, 0.95, 0.3);
            float r2 = ripple(uv, c2, 5.6, 49.0, 0.78, 1.6);
            float r3 = ripple(uv, c3, 6.2, 46.0, 0.86, 2.8);
            float r4 = ripple(uv, c4, 6.4, 52.0, 0.72, 4.1);
            float rippleField = r1 + r2 + r3 + r4;

            float caustic = fbm(vec2(uv.x * 3.4 + t * 0.05, uv.y * 7.4 - t * 0.03));
            float shimmer = sin((uv.y + caustic * 0.1) * 28.0 - t * 0.42) * 0.5 + 0.5;
            float horizon = smoothstep(0.0, 0.08, uv.y) * (1.0 - smoothstep(0.88, 1.0, uv.y));
            float sideFade = smoothstep(0.0, 0.08, uv.x) * smoothstep(0.0, 0.08, 1.0 - uv.x);

            vec3 aqua = vec3(0.48, 0.79, 0.9);
            vec3 blue = vec3(0.22, 0.46, 0.68);
            vec3 mist = vec3(0.84, 0.96, 1.0);

            float softBand = fbm(vec2(uv.x * 2.2 - t * 0.02, uv.y * 4.8 + t * 0.01));
            vec3 color =
                aqua * (0.08 + rippleField * 0.18) +
                blue * (0.05 + softBand * 0.1) +
                mist * (shimmer * 0.05 + rippleField * 0.08);

            float alpha =
                0.05 +
                rippleField * 0.16 +
                softBand * 0.06 +
                shimmer * 0.035;

            alpha *= horizon * sideFade;
            alpha = clamp(alpha, 0.0, 0.26);

            gl_FragColor = vec4(color, alpha);
        }
    `;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'ocean' &&
            !prefersReducedMotion() &&
            window.innerWidth >= 520;
    }

    function createShader(type, source) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);

        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            console.warn('[OceanShader] Shader compile error:', gl.getShaderInfoLog(shader));
            gl.deleteShader(shader);
            return null;
        }

        return shader;
    }

    function createCanvas() {
        canvas = document.createElement('canvas');
        canvas.id = 'ocean-overlay';
        Object.assign(canvas.style, {
            position: 'fixed',
            top: '0',
            left: '0',
            width: '0',
            height: '0',
            pointerEvents: 'none',
            zIndex: '2',
            opacity: '1',
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
            powerPreference: 'high-performance'
        });

        if (!gl) {
            console.warn('[OceanShader] WebGL not available');
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
            console.warn('[OceanShader] Program link error:', gl.getProgramInfoLog(program));
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

    window.AuraGoOcean = { start, stop, sync };
    init();
})();
