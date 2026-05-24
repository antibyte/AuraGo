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

        vec2 hash2(vec2 p) {
            p = vec2(dot(p, vec2(127.1, 311.7)), dot(p, vec2(269.5, 183.3)));
            return fract(sin(p) * 43758.5453123);
        }

        float noise(vec2 p) {
            vec2 i = floor(p);
            vec2 f = fract(p);
            float a = hash(i);
            float b = hash(i + vec2(1.0, 0.0));
            float c = hash(i + vec2(0.0, 1.0));
            float d = hash(i + vec2(1.0, 1.0));
            vec2 u = f * f * (3.0 - 2.0 * f);
            return mix(a, b, u.x) + (c - a) * u.y * (1.0 - u.x) + (d - b) * u.x * u.y;
        }

        float fbm(vec2 p) {
            float v = 0.0;
            float a = 0.5;
            for (int i = 0; i < 5; i++) {
                v += a * noise(p);
                p = p * 2.02 + vec2(4.1, 7.3);
                a *= 0.5;
            }
            return v;
        }

        float voronoi(vec2 p) {
            vec2 n = floor(p);
            vec2 f = fract(p);
            float md = 8.0;
            for (int j = -1; j <= 1; j++) {
                for (int i = -1; i <= 1; i++) {
                    vec2 g = vec2(float(i), float(j));
                    vec2 o = hash2(n + g);
                    o = 0.5 + 0.5 * sin(u_time * 0.22 + 6.2831 * o);
                    vec2 r = g + o - f;
                    float d = dot(r, r);
                    md = min(md, d);
                }
            }
            return md;
        }

        float caustics(vec2 uv, float scale, float speed) {
            vec2 p = uv * scale;
            float t = u_time * speed;
            float v1 = voronoi(p + vec2(t * 0.07, t * 0.05));
            float v2 = voronoi(p * 1.4 + vec2(-t * 0.04, t * 0.08) + 5.0);
            float c = sqrt(v1) * sqrt(v2);
            return smoothstep(0.0, 0.35, c);
        }

        float ripple(vec2 uv, vec2 center, float radius, float frequency, float speed, float phase) {
            vec2 p = uv - center;
            p.x *= u_res.x / max(u_res.y, 1.0);
            float dist = length(p);
            float envelope = exp(-dist * radius);
            float wave = sin(dist * frequency - u_time * speed + phase);
            float rings = smoothstep(0.36, 0.92, wave) + smoothstep(-1.0, -0.58, wave) * 0.65;
            return rings * envelope;
        }

        float lightShaft(vec2 uv, float angle, float width, float speed, float sway) {
            float c = cos(angle);
            float s = sin(angle);
            vec2 ruv = vec2(uv.x * c - uv.y * s, uv.x * s + uv.y * c);
            float swayAmt = sin(u_time * sway) * 0.04;
            float ray = smoothstep(width, 0.0, abs(ruv.x - 0.25 + swayAmt));
            float falloff = smoothstep(1.0, 0.15, ruv.y);
            float shimmer = sin(ruv.y * 18.0 - u_time * speed) * 0.35 + 0.65;
            float flicker = sin(u_time * speed * 1.7 + ruv.y * 6.0) * 0.15 + 0.85;
            return ray * falloff * shimmer * flicker;
        }

        float bubbleParticle(vec2 uv, vec2 center, float size, float speed, float phase) {
            vec2 p = uv - center;
            p.x *= u_res.x / max(u_res.y, 1.0);
            float floatY = mod(u_time * speed + phase, 1.6) - 0.3;
            p.y -= floatY;
            p.x += sin(floatY * 8.0 + phase) * 0.012;
            float dist = length(p);
            float outline = smoothstep(size, size * 0.7, dist) * (1.0 - smoothstep(size * 0.7, size * 0.3, dist));
            float highlight = smoothstep(size * 0.55, 0.0, length(p - vec2(-size * 0.28, size * 0.28))) * 0.5;
            float visible = step(-0.1, floatY) * step(floatY, 1.3);
            return (outline * 0.6 + highlight) * visible;
        }

        void main() {
            vec2 uv = v_uv;
            float t = u_time;

            vec2 c1 = vec2(0.24 + sin(t * 0.07) * 0.05, 0.22 + cos(t * 0.09) * 0.03);
            vec2 c2 = vec2(0.76 + cos(t * 0.05) * 0.04, 0.54 + sin(t * 0.06) * 0.04);
            vec2 c3 = vec2(0.48 + sin(t * 0.04) * 0.03, 0.82 + cos(t * 0.08) * 0.03);
            vec2 c4 = vec2(0.58 + cos(t * 0.06) * 0.03, 0.34 + sin(t * 0.05) * 0.03);

            float r1 = ripple(uv, c1, 5.0, 56.0, 0.95, 0.3);
            float r2 = ripple(uv, c2, 4.6, 49.0, 0.78, 1.6);
            float r3 = ripple(uv, c3, 5.2, 46.0, 0.86, 2.8);
            float r4 = ripple(uv, c4, 5.4, 52.0, 0.72, 4.1);
            float rippleField = r1 + r2 + r3 + r4;

            float causticA = caustics(uv, 4.0, 0.18);
            float causticB = caustics(uv + 3.7, 5.5, 0.12);
            float causticField = causticA * 0.6 + causticB * 0.4;

            float softBand = fbm(vec2(uv.x * 2.2 - t * 0.02, uv.y * 4.8 + t * 0.01));
            float shimmer = sin((uv.y + causticField * 0.08) * 28.0 - t * 0.42) * 0.5 + 0.5;
            float horizon = smoothstep(0.0, 0.06, uv.y) * (1.0 - smoothstep(0.9, 1.0, uv.y));
            float sideFade = smoothstep(0.0, 0.06, uv.x) * smoothstep(0.0, 0.06, 1.0 - uv.x);

            vec3 aqua = vec3(0.55, 0.88, 0.98);
            vec3 blue = vec3(0.28, 0.58, 0.82);
            vec3 mist = vec3(0.92, 1.0, 1.0);
            vec3 teal = vec3(0.3, 0.85, 0.78);

            float s1 = lightShaft(uv, 0.32, 0.04, 0.3, 0.18);
            float s2 = lightShaft(uv, 0.44, 0.025, 0.42, 0.25);
            float s3 = lightShaft(uv, 0.52, 0.018, 0.35, 0.22);
            float shaftField = s1 * 0.5 + s2 * 0.32 + s3 * 0.18;

            float b1 = bubbleParticle(uv, vec2(0.15, 0.5), 0.007, 0.09, 0.0);
            float b2 = bubbleParticle(uv, vec2(0.4, 0.3), 0.005, 0.11, 1.8);
            float b3 = bubbleParticle(uv, vec2(0.65, 0.6), 0.006, 0.08, 3.5);
            float b4 = bubbleParticle(uv, vec2(0.82, 0.4), 0.005, 0.12, 5.2);
            float b5 = bubbleParticle(uv, vec2(0.5, 0.75), 0.005, 0.07, 2.3);
            float bubbleField = b1 + b2 + b3 + b4 + b5;

            vec3 color =
                aqua * (0.10 + causticField * 0.18 + rippleField * 0.14) +
                blue * (0.06 + softBand * 0.08 + causticField * 0.04) +
                mist * (shimmer * 0.04 + causticField * 0.06 + rippleField * 0.06) +
                teal * (shaftField * 0.22) +
                vec3(0.9, 1.0, 1.0) * bubbleField * 0.4;

            float alpha =
                0.06 +
                causticField * 0.12 +
                rippleField * 0.14 +
                softBand * 0.04 +
                shimmer * 0.03 +
                shaftField * 0.08 +
                bubbleField * 0.25;

            alpha *= horizon * sideFade;
            alpha = clamp(alpha, 0.0, 0.35);

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
