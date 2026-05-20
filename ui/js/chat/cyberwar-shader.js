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
        const vec2 RADAR_CENTER = vec2(0.5, 0.5);
        const float RADAR_PI = 3.1415926;
        const float RADAR_TAU = 6.2831853;
        const float RADAR_BEAM_HIT_WIDTH = 0.075;

        float hash(vec2 p) {
            return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
        }

        float angleDistance(float a, float b) {
            return abs(mod(a - b + RADAR_PI, RADAR_TAU) - RADAR_PI);
        }

        vec2 movingRadarTarget(float t, float radius, float speed, float phase, float wobble) {
            float angle = t * speed + phase + sin(t * speed * 0.41 + phase) * wobble;
            float drift = radius + sin(t * speed * 0.73 + phase * 1.9) * 0.055;
            return vec2(cos(angle) * drift, sin(angle) * drift * 0.72);
        }

        float radarTarget(vec2 p, vec2 target, float beamAngle, float scale) {
            float dist = length(p - target);
            float body = smoothstep(0.034 * scale, 0.0, dist);
            float halo = exp(-dist * dist * 120.0 / max(scale, 0.18));
            float targetAngle = atan(target.y, target.x);
            float targetHit = smoothstep(RADAR_BEAM_HIT_WIDTH, 0.0, angleDistance(targetAngle, beamAngle));
            float rayTrack = smoothstep(0.022, 0.0, angleDistance(atan(p.y, p.x), targetAngle));
            float rangeTrack = exp(-96.0 * abs(length(p) - length(target)));
            float hitTrail = rayTrack * rangeTrack * targetHit * 0.45;
            return body * (0.42 + targetHit * 2.4) + halo * (0.06 + targetHit * 0.82) + hitTrail;
        }

        void main() {
            vec2 uv = v_uv;
            float aspect = u_res.x / max(u_res.y, 1.0);
            vec2 p = vec2((uv.x - RADAR_CENTER.x) * 2.0 * aspect, (uv.y - RADAR_CENTER.y) * 2.0);
            float radarRadius = length(p);
            float beamAngle = fract(u_time * 0.075) * RADAR_TAU - RADAR_PI;
            float beamDelta = angleDistance(atan(p.y, p.x), beamAngle);
            float radarSweep = exp(-31.0 * beamDelta);
            radarSweep *= smoothstep(1.14, 0.08, radarRadius);
            float radarRings = smoothstep(0.985, 1.0, abs(sin(radarRadius * 42.0))) * smoothstep(1.12, 0.08, radarRadius);
            float radarPips = smoothstep(0.996, 1.0, hash(floor((uv + vec2(u_time * 0.015, 0.0)) * vec2(36.0, 24.0))));
            radarPips *= smoothstep(1.08, 0.22, radarRadius) * 0.65;
            float radarCross =
                exp(-820.0 * abs(p.x)) * smoothstep(1.08, 0.14, abs(p.y)) +
                exp(-820.0 * abs(p.y)) * smoothstep(1.08, 0.14, abs(p.x));

            vec2 targetA = movingRadarTarget(u_time, 0.34, 0.23, 0.5, 0.16);
            vec2 targetB = movingRadarTarget(u_time, 0.57, -0.16, 2.35, 0.12);
            vec2 targetC = movingRadarTarget(u_time, 0.76, 0.12, 4.1, 0.1);
            vec2 targetD = movingRadarTarget(u_time, 0.48, -0.27, 5.45, 0.18);
            float radarTargets =
                radarTarget(p, targetA, beamAngle, 0.9) +
                radarTarget(p, targetB, beamAngle, 1.05) +
                radarTarget(p, targetC, beamAngle, 1.0);
            float radarThreats = radarTarget(p, targetD, beamAngle, 1.18);

            float vignette = smoothstep(1.55, 0.18, dot(p, p));
            float gridX = smoothstep(0.986, 1.0, abs(sin((uv.x + u_time * 0.009) * u_res.x * 0.028)));
            float gridY = smoothstep(0.988, 1.0, abs(sin((uv.y - u_time * 0.012) * u_res.y * 0.03)));
            float grid = (gridX + gridY) * 0.16;

            float sweep = exp(-44.0 * abs(uv.y - fract(u_time * 0.052)));
            float diag = exp(-70.0 * abs((uv.x * 0.92 + uv.y * 0.34) - fract(u_time * 0.041)));
            float pulse = 0.5 + 0.5 * sin(u_time * 1.3 + uv.x * 8.0);

            vec3 cyan = vec3(0.29, 0.97, 1.0);
            vec3 magenta = vec3(1.0, 0.28, 0.76);
            vec3 blue = vec3(0.45, 0.48, 1.0);

            vec2 coarseCell = floor(uv * vec2(42.0, 28.0) + floor(u_time * 0.75));
            float noise = hash(coarseCell);
            float spark = smoothstep(0.992, 1.0, noise) * 0.08;

            vec3 fog =
                cyan * (0.08 + 0.04 * sin(u_time * 0.7 + uv.y * 9.0)) +
                magenta * (0.05 + 0.05 * sin(u_time * 0.43 + uv.x * 7.0 + 1.4)) +
                blue * (0.06 + 0.04 * sin(u_time * 0.58 + (uv.x + uv.y) * 6.0));

            vec3 color =
                fog * 0.44 +
                cyan * sweep * 0.12 +
                magenta * diag * 0.16 +
                blue * grid * (0.45 + pulse * 0.18) +
                vec3(0.05, 1.0, 0.42) * radarSweep * radarRings * 0.18 +
                vec3(0.08, 0.95, 0.52) * radarSweep * radarCross * 0.08 +
                vec3(0.05, 1.0, 0.42) * radarSweep * radarPips * 0.1 +
                vec3(0.13, 1.0, 0.52) * radarTargets * 0.18 +
                vec3(1.0, 0.22, 0.68) * radarThreats * 0.16 +
                vec3(1.0, 0.9, 1.0) * spark;

            float alpha =
                vignette * 0.18 +
                sweep * 0.025 +
                diag * 0.035 +
                grid * 0.22 +
                radarSweep * radarRings * 0.035 +
                radarSweep * radarCross * 0.018 +
                radarPips * 0.04 +
                radarTargets * 0.085 +
                radarThreats * 0.078 +
                spark * 0.12;

            gl_FragColor = vec4(color, clamp(alpha, 0.0, 0.38));
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
