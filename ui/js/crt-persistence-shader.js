/**
 * AuraGo CRT Persistence Shader — event-driven phosphor afterglow, cathode warm-up & hero FX
 * Renders with mixBlendMode:'screen' for additive luminance only.
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
    let typingPulse = 0;
    let warmupPulse = 0;
    let chatObserver = null;
    let listenersAttached = false;
    let lastMutationAt = 0;
    let lastTypingAt = 0;

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
        uniform float u_typing;
        uniform float u_warmup;

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
            float sweepPos = fract(t * 0.072 + u_activity * 0.09);
            float sweep = band(uv.y, sweepPos, 13.0) * (0.36 + u_activity * 0.85);

            // Secondary sweep (ghost echo)
            float sweepPos2 = fract(sweepPos + 0.26);
            float sweep2 = band(uv.y, sweepPos2, 19.0) * u_activity * 0.28;

            // Traveling retrace line
            float travelingLine = band(uv.y, rollCenter, 36.0) * (0.18 + u_activity * 0.24);
            float lineGlow = band(uv.y, rollCenter, 10.5) * (0.10 + u_activity * 0.14);

            // Cathode horizontal warm-up line flare on theme activation
            float warmupWidth = 22.0 + (1.0 - u_warmup) * 58.0;
            float warmupLine = band(uv.y, 0.5, warmupWidth) * smoothstep(0.62, 0.02, abs(uv.x - 0.5)) * u_warmup * 0.52;
            float warmupCore = exp(-dist * 7.5) * u_warmup * 0.30;

            // Composer typing carrier afterglow
            float typingTrail = band(uv.y, 0.11, 15.0) * smoothstep(0.46, 0.04, abs(uv.x - 0.5)) * u_typing * 0.32;

            // Dual harmonic degauss rings
            float ringRadius = max(0.08, 0.68 - u_theme_pulse * 0.22);
            float degaussRing = exp(-abs(dist - ringRadius) * 16.0) * u_theme_pulse;
            float degaussInner = exp(-abs(dist - ringRadius * 0.58) * 22.0) * u_theme_pulse * 0.55;
            float degaussCore = smoothstep(0.38, 0.0, dist) * u_theme_pulse * 0.18;

            // Corner electromagnetic surge
            float edgeSurge =
                exp(-uv.x * 8.5) +
                exp(-(1.0 - uv.x) * 8.5) +
                exp(-uv.y * 12.0) +
                exp(-(1.0 - uv.y) * 12.0);
            edgeSurge *= u_theme_pulse * 0.12;

            // Phosphor persistence afterglow spill
            float afterglow = smoothstep(0.62, 0.0, dist) * (u_activity * 0.12 + u_typing * 0.06);

            vec3 phosphor = vec3(0.22, 0.99, 0.14);
            vec3 peakWhiteGreen = vec3(0.84, 1.0, 0.78);
            vec3 amber = vec3(1.0, 0.64, 0.18);
            vec3 cool = vec3(0.58, 0.92, 0.96);

            vec3 color =
                phosphor * (sweep + sweep2 + travelingLine + degaussCore * 0.7 + afterglow + typingTrail) +
                peakWhiteGreen * (lineGlow * 0.18 + warmupLine * 0.85 + warmupCore * 0.65) +
                amber * (degaussRing * 0.24 + degaussInner * 0.16) +
                cool * edgeSurge * 0.10;

            float alpha =
                sweep * 0.20 +
                sweep2 * 0.11 +
                travelingLine * 0.13 +
                lineGlow * 0.11 +
                warmupLine * 0.32 +
                warmupCore * 0.18 +
                typingTrail * 0.20 +
                degaussRing * 0.17 +
                degaussInner * 0.10 +
                degaussCore * 0.11 +
                edgeSurge * 0.07 +
                afterglow * 0.55;

            alpha = clamp(alpha, 0.0, 0.52);
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
        warmupPulse = Math.min(1.2, Math.max(warmupPulse, (amount || 1.0) * 0.95));
    }

    function boostActivity(amount) {
        activityPulse = Math.min(1.6, Math.max(activityPulse, amount || 0.8));
    }

    function attachListeners() {
        if (listenersAttached) return;
        listenersAttached = true;

        document.addEventListener('input', (e) => {
            if (!active) return;
            if (e.target && e.target.id === 'user-input') {
                const now = Date.now();
                if (now - lastTypingAt < 70) return;
                lastTypingAt = now;
                typingPulse = Math.min(1.3, Math.max(typingPulse, 0.85));
                boostActivity(0.45);
            }
        }, { passive: true });

        document.addEventListener('visibilitychange', () => {
            if (!active) return;
            if (document.hidden) {
                if (animId) {
                    cancelAnimationFrame(animId);
                    animId = null;
                }
                lastFrame = 0;
            } else if (!animId) {
                lastFrame = 0;
                animId = requestAnimationFrame(render);
            }
        });
    }

    function observeChat() {
        attachListeners();
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

    function setupGLResources() {
        if (!gl) return false;
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
        uniforms.typing = gl.getUniformLocation(program, 'u_typing');
        uniforms.warmup = gl.getUniformLocation(program, 'u_warmup');

        gl.enable(gl.BLEND);
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
        return true;
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
            zIndex: '999995',
            mixBlendMode: 'screen'
        });
        document.documentElement.appendChild(canvas);

        canvas.addEventListener('webglcontextlost', (e) => {
            e.preventDefault();
            if (animId) {
                cancelAnimationFrame(animId);
                animId = null;
            }
        });

        canvas.addEventListener('webglcontextrestored', () => {
            if (setupGLResources() && active && !document.hidden) {
                resize();
                lastFrame = 0;
                animId = requestAnimationFrame(render);
            }
        });

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

        return setupGLResources();
    }

    function resize() {
        if (!canvas) return;
        const dpr = Math.min(window.devicePixelRatio || 1, 1.25);
        canvas.width = Math.min(1440, Math.floor(window.innerWidth * dpr));
        canvas.height = Math.min(900, Math.floor(window.innerHeight * dpr));
        if (gl) gl.viewport(0, 0, canvas.width, canvas.height);
    }

    function render(time) {
        if (!active || !gl) return;
        if (document.hidden) {
            animId = null;
            return;
        }

        const dt = lastFrame ? Math.min(0.05, (time - lastFrame) * 0.001) : 0.016;
        lastFrame = time;
        themePulse = Math.max(0.0, themePulse - dt * 0.34);
        activityPulse = Math.max(0.0, activityPulse - dt * 0.55);
        typingPulse = Math.max(0.0, typingPulse - dt * 1.15);
        warmupPulse = Math.max(0.0, warmupPulse - dt * 0.95);

        gl.uniform1f(uniforms.time, time * 0.001);
        gl.uniform2f(uniforms.resolution, canvas.width, canvas.height);
        gl.uniform1f(uniforms.themePulse, themePulse);
        gl.uniform1f(uniforms.activity, activityPulse);
        gl.uniform1f(uniforms.typing, typingPulse);
        gl.uniform1f(uniforms.warmup, warmupPulse);
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
        if (!document.hidden) {
            animId = requestAnimationFrame(render);
        }
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
