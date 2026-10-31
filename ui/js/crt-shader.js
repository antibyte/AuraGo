/**
 * AuraGo CRT Shader — dynamic CRT glow, P31 phosphor triad & surface FX layer
 * Renders with mixBlendMode:'screen' for additive luminance only.
 * CSS handles darkening (scanlines, vignette); this shader handles light.
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
    let pointerX = 0.5;
    let pointerY = 0.5;
    let targetPointerX = 0.5;
    let targetPointerY = 0.5;
    let pointerEnergy = 0;
    let contentObserver = null;
    let statusObserver = null;
    let listenersAttached = false;
    let lastActivityAt = 0;
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
        uniform vec2 u_pointer;
        uniform float u_pointer_energy;
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

            // Magnetic degauss wave distortion on theme switch
            float degaussWave = sin(dist * 20.0 - t * 15.0) * u_theme_pulse * 0.014 * u_motion;
            float hsyncShear = sin(uv.y * 55.0 + t * 28.0) * (u_theme_pulse * 0.004 + u_activity * 0.0012) * u_motion;

            // Barrel distortion hint (authentic curved glass contour)
            float r2 = dot(acc, acc);
            float bow = 1.0 + r2 * 0.46 + r2 * r2 * 0.11;
            vec2 curved = (cc + vec2(hsyncShear + degaussWave * cc.y, -degaussWave * cc.x)) * bow + 0.5;

            // Bezel mask & curved corner cutoff
            vec2 edgeDist = min(curved, 1.0 - curved);
            float minEdge = min(edgeDist.x, edgeDist.y);
            float corner = smoothstep(0.0, 0.036, minEdge);
            if (curved.x < 0.0 || curved.x > 1.0 || curved.y < 0.0 || curved.y > 1.0) {
                gl_FragColor = vec4(0.0, 0.0, 0.0, 0.0);
                return;
            }

            float scanY = curved.y * u_res.y;

            // ---- GLOWING SCANLINE PEAKS ----
            float scan = sin(scanY * 3.14159265) * 0.5 + 0.5;
            float scanBright = pow(scan, 3.8) * 0.24;

            // ---- VERTICAL APERTURE GRILLE GLOW ----
            float maskX = curved.x * u_res.x;
            float triad = fract(maskX / 3.0);
            float apertureBright = smoothstep(0.18, 0.82, triad) * 0.09;

            // ---- ANALOG NOISE (controlled phosphor grain) ----
            float grain = noise(vec2(
                floor(curved.x * u_res.x * 0.35),
                floor(curved.y * u_res.y * 0.35)
            ) + floor(t * 12.0) * 37.0);
            float staticNoise = (grain - 0.5) * 0.075;
            float fineNoise = (noise(curved * u_res * 0.75 + fract(t * 42.0) * 71.0) - 0.5) * 0.04;

            // ---- BRIGHTNESS FLICKER (stable high-frequency cathode hum) ----
            float flicker = 0.90 + 0.10 * sin(t * 37.0) * sin(t * 17.0) * u_motion;

            // ---- HUM BARS (rolling AC synchronous light bands) ----
            float humPos = fract(t * 0.052 * u_motion);
            float hum = exp(-abs(curved.y - humPos) * 18.0) * 0.18;
            float humPos2 = fract(humPos + 0.48);
            float hum2 = exp(-abs(curved.y - humPos2) * 28.0) * 0.08;

            // ---- JITTER BAND ----
            float jitterBand = exp(-abs(curved.y - fract(t * 0.11)) * 11.0) * 0.11 * u_motion;

            // ---- TRAVELING RETRACE LINE ----
            float rollCenter = fract(t * 0.072 * u_motion + u_activity * 0.035);
            float rollBand = exp(-abs(curved.y - rollCenter) * 34.0);
            float travelingLine = rollBand * (0.24 + u_activity * 0.32);

            // ---- COMPOSER TYPING CARRIER EXCITATION ----
            float typingBand = exp(-abs(curved.y - 0.11) * 11.0) * smoothstep(0.48, 0.05, abs(curved.x - 0.5));
            float typingGlow = typingBand * u_typing * 0.24;

            // ---- INTERACTIVE POINTER ELECTROSTATIC WAKE ----
            vec2 ptrDelta = vec2((curved.x - u_pointer.x) * aspect, curved.y - u_pointer.y);
            float ptrDist = length(ptrDelta);
            float pointerWake = exp(-ptrDist * 7.5) * u_pointer_energy * 0.18;
            float pointerRing = exp(-abs(ptrDist - 0.08) * 32.0) * u_pointer_energy * 0.07;

            // ---- PHOSPHOR GLOW (reactive radial field) ----
            float phosphorEnergy = (0.10 + u_activity * 0.28 + u_theme_pulse * 0.26 + travelingLine * 0.50 + typingGlow);
            phosphorEnergy *= smoothstep(0.95, 0.06, dist);

            // ---- CURVED GLASS REFLECTIONS & BEZEL SPECULAR RIM ----
            float refl1 = exp(-abs(curved.y - 0.10 - sin(t * 0.24) * 0.025) * 9.5) * 0.14;
            float refl2 = exp(-abs(curved.x - 0.78 + cos(t * 0.16) * 0.028) * 14.0) * 0.08;
            float refl3 = exp(-abs(curved.y - 0.90 + sin(t * 0.12) * 0.020) * 11.0) * 0.05;
            float bezelRim = exp(-minEdge * 48.0) * (0.12 + u_theme_pulse * 0.14);
            float reflection = refl1 + refl2 + refl3 + bezelRim * 0.45;

            // ---- DEGAUSS RING & EDGE FLARE on theme activation ----
            float degaussRadius = max(0.05, (1.15 - u_theme_pulse * 0.75));
            float degaussRing = exp(-abs(dist - degaussRadius) * 18.0) * u_theme_pulse * 0.28;
            float edgeFlare = pow(dist, 2.3) * 0.10 * u_theme_pulse + degaussRing;

            // Combine brightness
            float brightness = (scanBright + apertureBright + staticNoise + fineNoise);
            brightness += hum + hum2 + jitterBand + travelingLine + pointerWake + pointerRing;
            brightness *= flicker;
            brightness += phosphorEnergy + reflection * 0.42 + edgeFlare;
            brightness = max(0.0, brightness);

            // Colors (authentic P31 green phosphor + warm amber degauss surge)
            vec3 phosphor = vec3(0.16, 0.97, 0.10);
            vec3 peakTint = vec3(0.82, 1.00, 0.78);
            vec3 glass = vec3(0.76, 0.98, 0.82);
            vec3 amber = vec3(1.00, 0.64, 0.18);

            vec3 color = phosphor * brightness;
            color += peakTint * (pointerWake * 0.55 + typingGlow * 0.45 + travelingLine * 0.18);
            color += glass * reflection * 0.32;
            color += amber * edgeFlare * 1.15;

            float alpha = brightness * 0.86 + reflection * 0.26 + pointerWake * 0.25;
            alpha = clamp(alpha, 0.0, 0.66) * corner;

            gl_FragColor = vec4(color, alpha);
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

    function noteTyping(amount) {
        const now = Date.now();
        if (now - lastTypingAt < 65) return;
        lastTypingAt = now;
        typingPulse = Math.min(1.4, Math.max(typingPulse, amount || 0.85));
        boostActivity(0.45);
    }

    function attachInteractiveListeners() {
        if (listenersAttached) return;
        listenersAttached = true;

        window.addEventListener('pointermove', (e) => {
            if (!active || prefersReducedMotion()) return;
            const w = Math.max(window.innerWidth, 1);
            const h = Math.max(window.innerHeight, 1);
            targetPointerX = Math.min(1, Math.max(0, e.clientX / w));
            targetPointerY = Math.min(1, Math.max(0, 1.0 - (e.clientY / h)));
            pointerEnergy = Math.min(1.0, pointerEnergy + 0.18);
        }, { passive: true });

        document.addEventListener('input', (e) => {
            if (!active) return;
            if (e.target && e.target.id === 'user-input') {
                noteTyping(0.95);
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

    function attachActivityObservers() {
        attachInteractiveListeners();
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
        uniforms.typing = gl.getUniformLocation(program, 'u_typing');
        uniforms.pointer = gl.getUniformLocation(program, 'u_pointer');
        uniforms.pointerEnergy = gl.getUniformLocation(program, 'u_pointer_energy');
        uniforms.motion = gl.getUniformLocation(program, 'u_motion');

        gl.enable(gl.BLEND);
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
        return true;
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
            console.warn('[CRT] WebGL not available');
            return false;
        }

        return setupGLResources();
    }

    function resize() {
        if (!canvas) return;
        const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
        canvas.width = Math.min(1920, Math.floor(window.innerWidth * dpr));
        canvas.height = Math.min(1080, Math.floor(window.innerHeight * dpr));
        if (gl) gl.viewport(0, 0, canvas.width, canvas.height);
    }

    function render(time) {
        if (!active || !gl) return;
        if (document.hidden) {
            animId = null;
            return;
        }

        const seconds = time * 0.001;
        const dt = lastFrame ? Math.min(0.05, (time - lastFrame) * 0.001) : 0.016;
        lastFrame = time;
        themePulse = Math.max(0.0, themePulse - dt * 0.32);
        activityPulse = Math.max(0.0, activityPulse - dt * 0.72);
        typingPulse = Math.max(0.0, typingPulse - dt * 1.35);
        pointerEnergy = Math.max(0.0, pointerEnergy - dt * 0.85);

        const lerpFactor = Math.min(1.0, dt * 12.0);
        pointerX += (targetPointerX - pointerX) * lerpFactor;
        pointerY += (targetPointerY - pointerY) * lerpFactor;

        gl.uniform1f(uniforms.time, seconds);
        gl.uniform2f(uniforms.resolution, canvas.width, canvas.height);
        gl.uniform1f(uniforms.themePulse, themePulse);
        gl.uniform1f(uniforms.activity, activityPulse);
        gl.uniform1f(uniforms.typing, typingPulse);
        gl.uniform2f(uniforms.pointer, pointerX, pointerY);
        gl.uniform1f(uniforms.pointerEnergy, pointerEnergy);
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
        boostTheme(1.05);
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

    function checkTheme(event) {
        const isCRT = document.documentElement.getAttribute('data-theme') === 'retro-crt';
        if (isCRT) {
            if (event && event.detail && event.detail.theme === 'retro-crt') {
                boostTheme(1.2);
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
