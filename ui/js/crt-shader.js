/**
 * AuraGo CRT Shader — realistic CRT screen simulation overlay
 * Effects: aperture grille, scanlines, barrel vignette, corner shadow,
 * analog noise, flicker, hum bars, horizontal jitter, phosphor glow,
 * glass reflections, traveling retrace line.
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
            return mix(a, b, u.x) + (c - a) * u.y * (1.0 - u.x) + (d - b) * u.x * u.y;
        }

        void main() {
            float t = u_time;
            vec2 uv = v_uv;
            vec2 cc = uv - 0.5;
            float aspect = u_res.x / max(u_res.y, 1.0);
            vec2 acc = vec2(cc.x * aspect, cc.y);
            float dist = length(acc);

            // Barrel distortion
            float r2 = dot(acc, acc);
            float bow = 1.0 + r2 * 0.60 + r2 * r2 * 0.14;
            vec2 curved = cc * bow + 0.5;

            // Corner rounding
            vec2 edgeDist = min(curved, 1.0 - curved);
            float corner = smoothstep(0.0, 0.035, min(edgeDist.x, edgeDist.y));

            // Off-curve = dark bezel
            if (curved.x < -0.01 || curved.x > 1.01 || curved.y < -0.01 || curved.y > 1.01) {
                gl_FragColor = vec4(0.0, 0.0, 0.0, 0.92);
                return;
            }

            // ---- SCANLINES ----
            float scanY = curved.y * u_res.y;
            float scan = sin(scanY * 3.14159265) * 0.5 + 0.5;
            float scanline = pow(scan, 1.45);
            float scanDarken = 0.48 + 0.52 * scanline;
            // Interlace alternation
            float field = mod(floor(scanY * 0.5), 2.0);
            float interlace = field < 1.0 ? 0.965 : 1.0;
            scanDarken *= interlace;

            // ---- APERTURE GRILLE / SHADOW MASK ----
            float maskX = curved.x * u_res.x;
            float triad = fract(maskX / 3.0);
            float aperture = 0.74 + 0.26 * smoothstep(0.30, 0.70, triad);
            float slotY = fract(scanY * 0.5);
            float slot = 0.84 + 0.16 * smoothstep(0.40, 0.60, slotY);

            // ---- VIGNETTE & CORNER DARKENING ----
            float vignette = smoothstep(1.05, 0.18, dist * 1.18);
            float cornerDark = (1.0 - corner) * 0.82;

            // ---- ANALOG NOISE ----
            float grain = noise(vec2(
                floor(curved.x * u_res.x * 0.3),
                floor(curved.y * u_res.y * 0.3)
            ) + floor(t * 8.0) * 29.0);
            grain = (grain - 0.5) * 0.065;
            float staticNoise = (noise(curved * u_res * 0.5 + fract(t * 50.0) * 73.0) - 0.5) * 0.035;

            // ---- FLICKER ----
            float flicker = 0.92 + 0.08 * sin(t * 37.0) * sin(t * 23.0);

            // ---- HUM BARS (rolling bands) ----
            float humPos = fract(t * 0.052);
            float hum = exp(-abs(curved.y - humPos) * 26.0) * 0.11;
            float humPos2 = fract(humPos + 0.43);
            float hum2 = exp(-abs(curved.y - humPos2) * 38.0) * 0.055;

            // ---- HORIZONTAL JITTER (sync instability) ----
            float jitterLine = floor(scanY * 0.25);
            float jitter = (noise(vec2(jitterLine, fract(t * 18.0) * 67.0)) - 0.5) * 0.003 * u_motion;
            float jitterBand = exp(-abs(curved.y - fract(t * 0.11 + 0.3)) * 14.0) * 0.075 * u_motion;

            // ---- TRAVELING RETRACE LINE ----
            float rollCenter = fract(t * 0.078 + u_activity * 0.038);
            float rollBand = exp(-abs(curved.y - rollCenter) * 40.0);
            float travelingLine = rollBand * (0.13 + u_activity * 0.20);

            // ---- PHOSPHOR GLOW (additive energy) ----
            float phosphorEnergy = (0.07 + u_activity * 0.22 + u_theme_pulse * 0.18 + travelingLine * 0.42);
            phosphorEnergy *= smoothstep(0.92, 0.08, dist);

            // ---- GLASS REFLECTIONS ----
            float refl1 = exp(-abs(curved.y - 0.11 - sin(t * 0.30) * 0.028) * 13.0) * 0.085;
            float refl2 = exp(-abs(curved.x - 0.76 + cos(t * 0.20) * 0.028) * 20.0) * 0.04;
            float refl3 = exp(-abs(curved.y - 0.89 + sin(t * 0.13) * 0.02) * 16.0) * 0.028;
            float reflection = refl1 + refl2 + refl3;

            // ---- EDGE CHROMATICS (brightness fringe for green theme) ----
            float edgeFringe = pow(dist, 2.8) * 0.10;

            // Combine all darkening
            float darken = scanDarken * aperture * slot * vignette * flicker;
            darken -= hum;
            darken -= hum2;
            darken += grain;
            darken += staticNoise;
            darken -= jitterBand;
            darken = clamp(darken, 0.0, 1.0);

            // Colors
            vec3 phosphor = vec3(0.10, 0.88, 0.06);
            vec3 glass = vec3(0.68, 0.95, 0.74);
            vec3 amber = vec3(0.95, 0.50, 0.08);

            // Darkening alpha
            float darkAlpha = (1.0 - darken) * 0.30 + cornerDark * 0.85;
            darkAlpha = clamp(darkAlpha, 0.0, 0.72);

            // Glow
            vec3 glow = phosphor * phosphorEnergy * 0.9;
            glow += glass * reflection * vignette * 0.20;
            glow += amber * edgeFringe * (0.5 + u_theme_pulse * 0.5);
            glow += phosphor * travelingLine * 0.13 * vignette;

            float glowAlpha = phosphorEnergy * 0.48 + reflection * 0.24 + travelingLine * 0.13;
            glowAlpha = clamp(glowAlpha, 0.0, 0.40);

            // Final output: normal alpha blending handles both darkening and brightening
            vec3 finalColor = glow;
            float finalAlpha = mix(darkAlpha, glowAlpha, 0.42);
            finalAlpha = clamp(finalAlpha, 0.0, 0.68);

            gl_FragColor = vec4(finalColor, finalAlpha);
        }
    `;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function motionLevel() {
        return prefersReducedMotion() ? 0.12 : 1.0;
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
            imageRendering: 'auto'
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
