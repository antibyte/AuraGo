/**
 * AuraGo CRT Shader — upgraded WebGL surface for premium retro CRT simulation
 * Handles: glass curvature, scanlines, slot-mask feel, vignette, analog shimmer,
 * line wobble, phosphor bloom energy and theme/activity pulse response.
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
    let contentObserver = null;
    let statusObserver = null;
    let lastActivityAt = 0;

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
        uniform float u_motion;

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
                p = p * 2.02 + vec2(5.17, 7.41);
                amp *= 0.52;
            }
            return value;
        }

        void main() {
            float t = u_time * (0.3 + 0.7 * u_motion);
            vec2 uv = v_uv;
            vec2 cc = uv - 0.5;
            float aspect = u_res.x / max(u_res.y, 1.0);
            vec2 aspectCC = vec2(cc.x * aspect, cc.y);

            float r2 = dot(aspectCC, aspectCC);
            float bow = 1.0 + r2 * 0.88 + r2 * r2 * 0.18;
            vec2 curved = cc * bow + 0.5;

            float lineDrift =
                sin(curved.y * u_res.y * 0.018 + t * 1.3) * 0.0018 * u_motion * (0.45 + u_activity * 0.8) +
                (noise(vec2(curved.y * 140.0, t * 1.6)) - 0.5) * 0.002 * u_motion;
            curved.x += lineDrift;

            if (curved.x < 0.0 || curved.x > 1.0 || curved.y < 0.0 || curved.y > 1.0) {
                gl_FragColor = vec4(0.0, 0.0, 0.0, 0.0);
                return;
            }

            float scanY = curved.y * u_res.y;
            float scan = 0.84 + 0.16 * sin(scanY * 3.14159 * 0.92 + t * 1.4);
            float verticalMask = 0.95 + 0.05 * sin(curved.x * u_res.x * 0.18);

            float grain = fbm(vec2(curved.x * u_res.x * 0.012, curved.y * u_res.y * 0.018 + t * 0.9));
            float staticNoise = (noise(vec2(curved * u_res * 0.32 + t * 0.6)) - 0.5);
            float signalShimmer = sin((curved.y + grain * 0.04) * 420.0 - t * 15.0) * 0.5 + 0.5;

            float dist = length(aspectCC * 1.12);
            float vignette = smoothstep(0.96, 0.2, dist);
            vec2 edgeDist = min(curved, 1.0 - curved);
            float corner = smoothstep(0.0, 0.055, min(edgeDist.x, edgeDist.y));
            float fresnel = pow(1.0 - max(0.0, vignette), 1.6);

            float reflection = exp(-abs(curved.y - 0.14 - sin(t * 0.35) * 0.02) * 14.0) * (0.10 + u_theme_pulse * 0.12);
            reflection += exp(-abs(curved.x - 0.76 + sin(t * 0.22) * 0.02) * 20.0) * 0.035;

            float phosphorGlow =
                (0.06 + u_activity * 0.22 + u_theme_pulse * 0.18) *
                smoothstep(0.88, 0.08, dist) *
                (0.72 + signalShimmer * 0.28);

            float energy = scan * verticalMask;
            energy *= 0.82 + grain * 0.2 + staticNoise * 0.05;
            energy *= vignette * corner;

            vec3 phosphor = vec3(0.14, 0.82, 0.08);
            vec3 glass = vec3(0.72, 0.92, 0.78);
            vec3 amber = vec3(0.54, 0.26, 0.08);

            vec3 color =
                phosphor * phosphorGlow * energy * 0.95 +
                phosphor * staticNoise * 0.02 * vignette +
                glass * reflection * vignette * 0.08 +
                amber * fresnel * 0.018 * (0.6 + u_theme_pulse * 0.8);

            float alpha =
                (1.0 - vignette) * 0.22 +
                phosphorGlow * 0.42 +
                reflection * 0.18 +
                grain * 0.035;
            alpha *= corner;
            alpha = clamp(alpha, 0.0, 0.28 + u_activity * 0.05 + u_theme_pulse * 0.04);

            gl_FragColor = vec4(color, alpha);
        }
    `;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function motionLevel() {
        return prefersReducedMotion() ? 0.15 : 1.0;
    }

    function createShader(type, src) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, src);
        gl.compileShader(shader);
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            console.warn('[CRT] Shader error:', gl.getShaderInfoLog(shader));
            gl.deleteShader(shader);
            return null;
        }
        return shader;
    }

    function boostTheme(amount) {
        themePulse = Math.min(1.5, Math.max(themePulse, amount || 1.0));
    }

    function boostActivity(amount) {
        activityPulse = Math.min(1.7, Math.max(activityPulse, amount || 0.8));
    }

    function noteActivity(amount) {
        const now = Date.now();
        if (now - lastActivityAt < 90) return;
        lastActivityAt = now;
        boostActivity(amount || 0.9);
    }

    function attachActivityObservers() {
        if (contentObserver || typeof MutationObserver === 'undefined') return;

        const chatContent = document.getElementById('chat-content');
        if (chatContent) {
            contentObserver = new MutationObserver((mutations) => {
                if (!active) return;
                for (let i = 0; i < mutations.length; i++) {
                    const mutation = mutations[i];
                    if (mutation.type === 'childList') {
                        if (mutation.addedNodes.length || mutation.removedNodes.length) {
                            noteActivity(1.0);
                            break;
                        }
                    } else if (mutation.type === 'characterData') {
                        noteActivity(0.75);
                        break;
                    }
                }
            });
            contentObserver.observe(chatContent, {
                childList: true,
                subtree: true,
                characterData: true
            });
        }

        const agentStatus = document.getElementById('agentStatusContainer');
        if (agentStatus) {
            statusObserver = new MutationObserver(() => {
                if (active) noteActivity(0.55);
            });
            statusObserver.observe(agentStatus, {
                attributes: true,
                attributeFilter: ['class'],
                childList: true,
                subtree: true
            });
        }
    }

    function initGL() {
        canvas = document.createElement('canvas');
        canvas.id = 'crt-overlay';
        Object.assign(canvas.style, {
            position: 'fixed',
            inset: '0',
            width: '100vw',
            height: '100vh',
            pointerEvents: 'none',
            zIndex: '999996',
            imageRendering: 'auto',
            mixBlendMode: 'screen'
        });
        document.documentElement.appendChild(canvas);

        gl = canvas.getContext('webgl', {
            alpha: true,
            premultipliedAlpha: false,
            antialias: false,
            powerPreference: 'high-performance'
        });
        if (!gl) {
            console.warn('[CRT] WebGL not available');
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
            console.warn('[CRT] Link error:', gl.getProgramInfoLog(program));
            return false;
        }
        gl.useProgram(program);

        const buf = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, buf);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
        const aPos = gl.getAttribLocation(program, 'a_pos');
        gl.enableVertexAttribArray(aPos);
        gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

        uniforms.time = gl.getUniformLocation(program, 'u_time');
        uniforms.resolution = gl.getUniformLocation(program, 'u_res');
        uniforms.themePulse = gl.getUniformLocation(program, 'u_theme_pulse');
        uniforms.activity = gl.getUniformLocation(program, 'u_activity');
        uniforms.motion = gl.getUniformLocation(program, 'u_motion');

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

        const seconds = time * 0.001;
        const dt = lastFrame ? Math.min(0.05, (time - lastFrame) * 0.001) : 0.016;
        lastFrame = time;
        themePulse = Math.max(0.0, themePulse - dt * 0.3);
        activityPulse = Math.max(0.0, activityPulse - dt * 0.75);

        gl.uniform1f(uniforms.time, seconds);
        gl.uniform2f(uniforms.resolution, canvas.width, canvas.height);
        gl.uniform1f(uniforms.themePulse, themePulse);
        gl.uniform1f(uniforms.activity, activityPulse);
        gl.uniform1f(uniforms.motion, motionLevel());
        gl.clearColor(0, 0, 0, 0);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
        animId = requestAnimationFrame(render);
    }

    function start() {
        if (active) return;
        if (!canvas && !initGL()) return;
        attachActivityObservers();
        active = true;
        canvas.style.display = 'block';
        resize();
        boostTheme(1.0);
        animId = requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animId) cancelAnimationFrame(animId);
        animId = null;
        lastFrame = 0;
        if (canvas) canvas.style.display = 'none';
    }

    function checkTheme(event) {
        const isCRT = document.documentElement.getAttribute('data-theme') === 'retro-crt';
        if (isCRT) {
            if (event && event.detail && event.detail.theme === 'retro-crt') {
                boostTheme(1.15);
            }
            if (!active) start();
        } else if (active) {
            stop();
        }
    }

    function init() {
        window.addEventListener('aurago:themechange', checkTheme);

        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(checkTheme).observe(document.documentElement, {
                attributes: true,
                attributeFilter: ['data-theme']
            });
        }

        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => {
                attachActivityObservers();
                checkTheme();
            }, { once: true });
        } else {
            attachActivityObservers();
            checkTheme();
        }

        window.addEventListener('resize', () => {
            if (active) resize();
        });

        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            const syncMotion = () => {
                if (active) {
                    boostTheme(0.4);
                }
            };
            if (mq.addEventListener) {
                mq.addEventListener('change', syncMotion);
            } else if (mq.addListener) {
                mq.addListener(syncMotion);
            }
        }
    }

    window.AuraGoCRT = { start, stop, checkTheme, boostTheme, boostActivity };
    init();
})();
