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
                p = p * 2.05 + vec2(9.2, 4.7);
                amp *= 0.5;
            }
            return value;
        }

        vec4 renderSun(vec2 uv, vec2 center, float radius, float phase, float side) {
            vec2 p = uv - center;
            float aspect = u_res.x / max(u_res.y, 1.0);
            p.x *= aspect * 0.72;

            float t = u_time * (0.85 + phase * 0.08);
            float dist = length(p);
            float angle = atan(p.y, p.x);
            float pulse = 0.94 + 0.1 * sin(t * 1.5 + phase * 3.1);

            float core = smoothstep(radius * pulse, radius * 0.16, dist);
            float inner = smoothstep(radius * 1.55 * pulse, radius * 0.34, dist);
            float coronaNoise = fbm(vec2(angle * 2.6 + phase * 4.0, dist * 8.5 - t * 2.2));
            float corona = smoothstep(radius * 2.75 * pulse, radius * 0.74, dist) * (0.45 + coronaNoise * 0.9);

            float flareH = exp(-30.0 * abs(p.y + sin(t * 1.7 + p.x * 16.0) * 0.045));
            float flareD = exp(-24.0 * abs(p.y * 0.78 - p.x * side * 0.42 - sin(t * 0.9 + phase) * 0.12));
            float flare = flareH * 0.24 + flareD * 0.18;

            float sunspots = smoothstep(0.52, 0.74, fbm(p * 18.0 + vec2(t * 0.55, -t * 0.35))) * inner;
            float plume = fbm(vec2(p.x * 7.0 * side + t * 1.1, p.y * 4.5 - t * 0.7));
            float rim = exp(-abs(dist - radius * pulse * 0.96) * 24.0) * (0.62 + plume * 0.55);

            vec3 deep = vec3(0.08, 0.02, 0.01);
            vec3 ember = vec3(0.8, 0.18, 0.08);
            vec3 orange = vec3(0.98, 0.43, 0.14);
            vec3 gold = vec3(1.0, 0.76, 0.34);

            vec3 color =
                deep * 0.08 +
                ember * corona * 0.92 +
                orange * inner * 1.18 +
                gold * core * 1.14 +
                gold * rim * 0.55 +
                orange * flare * 0.72;

            color *= 1.0 - sunspots * 0.34;

            float alpha =
                corona * 0.24 +
                inner * 0.2 +
                core * 0.16 +
                rim * 0.12 +
                flare * 0.24;

            return vec4(color, alpha);
        }

        void main() {
            vec2 uv = v_uv;
            float t = u_time;
            float edgeMask = pow(smoothstep(0.06, 1.0, abs(uv.x - 0.5) * 2.0), 0.9);

            vec4 leftSun = renderSun(
                uv,
                vec2(-0.08 + sin(t * 0.08) * 0.014, 0.28 + sin(t * 0.19) * 0.026),
                0.29,
                0.15,
                -1.0
            );
            vec4 rightSun = renderSun(
                uv,
                vec2(1.08 + cos(t * 0.07) * 0.014, 0.7 + cos(t * 0.17) * 0.024),
                0.34,
                1.9,
                1.0
            );

            float haze = fbm(vec2(uv.y * 5.2 + t * 0.28, abs(uv.x - 0.5) * 9.0 - t * 0.46));
            vec2 emberGrid = vec2(uv.x * 34.0, uv.y * 46.0) + vec2(t * 1.8, -t * 4.2);
            vec2 emberCell = floor(emberGrid);
            vec2 emberLocal = fract(emberGrid) - 0.5;
            float emberSeed = hash(emberCell);
            float emberRadius = mix(0.08, 0.2, hash(emberCell + vec2(4.2, 9.1)));
            float emberShape = smoothstep(emberRadius, emberRadius * 0.32, length(emberLocal));
            float ember = smoothstep(0.972, 0.998, emberSeed) * emberShape * edgeMask;

            vec3 emberColor = vec3(1.0, 0.62, 0.22);
            vec3 hazeColor = vec3(0.96, 0.28, 0.08) * haze * edgeMask * 0.14;

            vec3 color =
                (leftSun.rgb + rightSun.rgb) * edgeMask +
                hazeColor +
                emberColor * ember * 1.15;

            float alpha =
                (leftSun.a + rightSun.a) * edgeMask * 1.28 +
                haze * edgeMask * 0.1 +
                ember * 0.18;

            float bounds =
                smoothstep(0.0, 0.05, uv.x) *
                smoothstep(0.0, 0.03, uv.y) *
                smoothstep(0.0, 0.05, 1.0 - uv.x) *
                smoothstep(0.0, 0.03, 1.0 - uv.y);
            alpha *= bounds;

            gl_FragColor = vec4(color, alpha);
        }
    `;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'dark-sun' &&
            !prefersReducedMotion() &&
            window.innerWidth >= 768;
    }

    function createShader(type, source) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);

        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            console.warn('[DarkSunShader] Shader compile error:', gl.getShaderInfoLog(shader));
            gl.deleteShader(shader);
            return null;
        }

        return shader;
    }

    function createCanvas() {
        canvas = document.createElement('canvas');
        canvas.id = 'dark-sun-overlay';
        Object.assign(canvas.style, {
            position: 'fixed',
            top: '0',
            left: '0',
            width: '0',
            height: '0',
            pointerEvents: 'none',
            zIndex: '2',
            opacity: '0.96',
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
            console.warn('[DarkSunShader] WebGL not available');
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
            console.warn('[DarkSunShader] Program link error:', gl.getProgramInfoLog(program));
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

    window.AuraGoDarkSun = { start, stop, sync };
    init();
})();
