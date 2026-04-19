/**
 * AuraGo CRT Persistence Shader — event-driven phosphor afterglow & hero FX
 * Handles: phosphor persistence sweeps, degauss edge pulses, power-on bloom,
 * activity-reactive brightness surges.
 */
(function () {
    'use strict';

    let canvas;
    let gl;
    let program;
    let uniforms = {};
    let animId = null;
    let active = false;
    let lastFrame = 0;
    let themePulse = 0;
    let activityPulse = 0;
    let chatObserver = null;
    let lastMutationAt = 0;

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
        uniform float u_theme_pulse;
        uniform float u_activity;

        float band(float y, float center, float width) {
            return exp(-abs(y - center) * width);
        }

        void main() {
            vec2 uv = v_uv;
            float t = u_time;
            float aspect = u_res.x / max(u_res.y, 1.0);
            vec2 cc = vec2((uv.x - 0.5) * aspect, uv.y - 0.5);
            float dist = length(cc);
            float rollCenter = fract(t * 0.095 + u_activity * 0.05);

            // Main phosphor sweep
            float sweepPos = fract(t * 0.065 + u_activity * 0.08);
            float sweep = band(uv.y, sweepPos, 16.0) * (0.25 + u_activity * 0.75);

            // Secondary sweep (ghost echo)
            float sweepPos2 = fract(sweepPos + 0.28);
            float sweep2 = band(uv.y, sweepPos2, 24.0) * u_activity * 0.22;

            // Traveling retrace line
            float travelingLine = band(uv.y, rollCenter, 48.0) * (0.12 + u_activity * 0.14);
            float lineGlow = band(uv.y, rollCenter, 14.0) * (0.07 + u_activity * 0.10);

            // Degauss-inspired edge pulse rings
            float degaussRing = exp(-abs(dist - 0.52) * 20.0) * u_theme_pulse;
            float degaussCore = smoothstep(0.38, 0.0, dist) * u_theme_pulse * 0.14;

            // Corner power surge
            float edgeSurge =
                exp(-uv.x * 10.0) +
                exp(-(1.0 - uv.x) * 10.0) +
                exp(-uv.y * 14.0) +
                exp(-(1.0 - uv.y) * 14.0);
            edgeSurge *= u_theme_pulse * 0.10;

            // Phosphor afterglow spill
            float afterglow = smoothstep(0.65, 0.0, dist) * u_activity * 0.08;

            vec3 phosphor = vec3(0.22, 1.0, 0.14);
            vec3 amber = vec3(0.95, 0.58, 0.14);
            vec3 cool = vec3(0.55, 0.85, 1.0);

            vec3 color =
                phosphor * (sweep + sweep2 + travelingLine + degaussCore * 0.6 + afterglow) +
                vec3(0.75, 1.0, 0.70) * lineGlow * 0.10 +
                amber * degaussRing * 0.16 +
                cool * edgeSurge * 0.08;

            float alpha =
                sweep * 0.14 +
                sweep2 * 0.08 +
                travelingLine * 0.09 +
                lineGlow * 0.09 +
                degaussRing * 0.12 +
                degaussCore * 0.09 +
                edgeSurge * 0.06 +
                afterglow * 0.5;

            alpha = clamp(alpha, 0.0, 0.32);
            gl_FragColor = vec4(color, alpha);
        }
    `;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'retro-crt' && !prefersReducedMotion();
    }

    function createShader(type, src) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, src);
        gl.compileShader(shader);
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            console.warn('[CRTPersistence] Shader error:', gl.getShaderInfoLog(shader));
            gl.deleteShader(shader);
            return null;
        }
        return shader;
    }

    function boostTheme(amount) {
        themePulse = Math.min(1.45, Math.max(themePulse, amount || 1.0));
    }

    function boostActivity(amount) {
        activityPulse = Math.min(1.6, Math.max(activityPulse, amount || 0.8));
    }

    function observeChat() {
        if (chatObserver || typeof MutationObserver === 'undefined') return;
        const chatContent = document.getElementById('chat-content');
        if (!chatContent) return;

        chatObserver = new MutationObserver((mutations) => {
            const now = Date.now();
            if (now - lastMutationAt < 120 || !active) return;
            for (let i = 0; i < mutations.length; i++) {
                const mutation = mutations[i];
                if (mutation.type === 'childList' || mutation.type === 'characterData') {
                    lastMutationAt = now;
                    boostActivity(mutation.type === 'childList' ? 1.0 : 0.7);
                    break;
                }
            }
        });
        chatObserver.observe(chatContent, {
            childList: true,
            subtree: true,
            characterData: true
        });
    }

    function initGL() {
        canvas = document.createElement('canvas');
        canvas.id = 'crt-persistence-overlay';
        Object.assign(canvas.style, {
            position: 'fixed',
            inset: '0',
            width: '100vw',
            height: '100vh',
            pointerEvents: 'none',
            zIndex: '999995'
        });
        document.documentElement.appendChild(canvas);

        gl = canvas.getContext('webgl', {
            alpha: true,
            premultipliedAlpha: false,
            antialias: false,
            powerPreference: 'high-performance'
        });
        if (!gl) {
            console.warn('[CRTPersistence] WebGL not available');
            return false;
        }

        const vs = createShader(gl.VERTEX_SHADER, VERT);
        const fs = createShader(gl.FRAGMENT_SHADER, FRAG);
        if (!vs || !fs) return false;

        program = gl.createProgram();
        gl.attachShader(program, vs);
        gl.attachShader(program, fs);
        gl.linkProgram(program);
        if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
            console.warn('[CRTPersistence] Link error:', gl.getProgramInfoLog(program));
            return false;
        }

        gl.useProgram(program);
        const buffer = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
        const aPos = gl.getAttribLocation(program, 'a_pos');
        gl.enableVertexAttribArray(aPos);
        gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

        uniforms.time = gl.getUniformLocation(program, 'u_time');
        uniforms.resolution = gl.getUniformLocation(program, 'u_res');
        uniforms.themePulse = gl.getUniformLocation(program, 'u_theme_pulse');
        uniforms.activity = gl.getUniformLocation(program, 'u_activity');

        gl.enable(gl.BLEND);
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
        return true;
    }

    function resize() {
        if (!canvas) return;
        const dpr = Math.min(window.devicePixelRatio || 1, 2);
        canvas.width = Math.floor(window.innerWidth * dpr);
        canvas.height = Math.floor(window.innerHeight * dpr);
        if (gl) gl.viewport(0, 0, canvas.width, canvas.height);
    }

    function render(time) {
        if (!active || !gl) return;

        const dt = lastFrame ? Math.min(0.05, (time - lastFrame) * 0.001) : 0.016;
        lastFrame = time;
        themePulse = Math.max(0.0, themePulse - dt * 0.34);
        activityPulse = Math.max(0.0, activityPulse - dt * 0.55);

        gl.uniform1f(uniforms.time, time * 0.001);
        gl.uniform2f(uniforms.resolution, canvas.width, canvas.height);
        gl.uniform1f(uniforms.themePulse, themePulse);
        gl.uniform1f(uniforms.activity, activityPulse);
        gl.clearColor(0, 0, 0, 0);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
        animId = requestAnimationFrame(render);
    }

    function start() {
        if (active) return;
        if (!canvas && !initGL()) return;
        observeChat();
        active = true;
        canvas.style.display = 'block';
        resize();
        boostTheme(1.2);
        animId = requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animId) cancelAnimationFrame(animId);
        animId = null;
        lastFrame = 0;
        if (canvas) canvas.style.display = 'none';
    }

    function sync(event) {
        if (shouldRun()) {
            if (event && event.detail && event.detail.theme === 'retro-crt') {
                boostTheme(1.25);
            }
            if (!active) start();
        } else if (active) {
            stop();
        }
    }

    function init() {
        window.addEventListener('aurago:themechange', sync);

        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true,
                attributeFilter: ['data-theme']
            });
        }

        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => {
                observeChat();
                sync();
            }, { once: true });
        } else {
            observeChat();
            sync();
        }

        window.addEventListener('resize', () => {
            if (active) resize();
        });
    }

    window.AuraGoCRTPersistence = { start, stop, sync, boostTheme, boostActivity };
    init();
})();
