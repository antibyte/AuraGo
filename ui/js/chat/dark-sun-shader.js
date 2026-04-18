(function () {
    'use strict';

    let canvas;
    let gl;
    let program;
    let animationId = null;
    let active = false;
    let uniforms = {};

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

        void main() {
            vec2 uv = v_uv;
            vec2 p = uv * 2.0 - 1.0;
            float aspect = u_res.x / max(u_res.y, 1.0);
            p.x *= aspect;

            float t = u_time * 0.22;
            float radius = length(p);
            float angle = atan(p.y, p.x);

            float coronaNoise = fbm(vec2(angle * 1.5, radius * 3.5 - t * 2.4));
            float corona = smoothstep(1.08, 0.18, radius) * coronaNoise;
            float core = smoothstep(0.54, 0.04, radius);

            float flareA = exp(-28.0 * abs(p.y + sin(t * 1.4 + p.x * 6.0) * 0.05));
            float flareB = exp(-36.0 * abs(p.x * 0.76 - p.y * 0.38 - sin(t * 0.7) * 0.15));
            float flare = max(flareA * 0.42, flareB * 0.3);

            float sunspots = smoothstep(0.48, 0.72, fbm(vec2(p * 8.0 + t * 0.8)));
            float haze = fbm(vec2(p * 5.2 + vec2(t * 1.1, -t * 0.6)));
            float heat = sin((uv.y + haze * 0.08 + t * 0.25) * u_res.y * 0.012) * 0.5 + 0.5;

            vec2 emberCell = floor(uv * vec2(72.0, 42.0) + vec2(t * 12.0, -t * 8.0));
            float emberSeed = hash(emberCell);
            float ember = smoothstep(0.985, 1.0, emberSeed) * smoothstep(1.25, 0.16, radius);

            vec3 deep = vec3(0.08, 0.02, 0.01);
            vec3 emberRed = vec3(0.84, 0.2, 0.09);
            vec3 orange = vec3(0.98, 0.45, 0.15);
            vec3 gold = vec3(1.0, 0.74, 0.28);

            vec3 color =
                deep * 0.22 +
                emberRed * corona * 1.2 +
                orange * core * 1.08 +
                gold * flare * 0.96 +
                orange * heat * 0.14 +
                gold * ember * 1.25;

            color *= 1.0 - sunspots * 0.3;

            float alpha =
                corona * 0.28 +
                core * 0.14 +
                flare * 0.28 +
                ember * 0.2 +
                heat * 0.06;

            float vignette = smoothstep(1.45, 0.25, dot(p, p));
            alpha *= vignette;

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
            inset: '0',
            width: '100vw',
            height: '100vh',
            pointerEvents: 'none',
            zIndex: '0',
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

    function resize() {
        if (!canvas || !gl) return;
        const dpr = Math.min(window.devicePixelRatio || 1, 2);
        canvas.width = Math.floor(window.innerWidth * dpr);
        canvas.height = Math.floor(window.innerHeight * dpr);
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
