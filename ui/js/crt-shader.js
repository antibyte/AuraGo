/**
 * AuraGo CRT Shader — WebGL overlay for authentic retro CRT monitor effect
 * Applies: barrel distortion illusion, scanlines, vignette, phosphor glow, flicker
 * Only active when [data-theme="retro-crt"] is set on <html>
 */
(function () {
    'use strict';

    let canvas, gl, program, uniforms = {};
    let animId = null;
    let active = false;

    /* ── Shaders ── */
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

        void main() {
            vec2 uv = v_uv;

            /* ── Barrel distortion (CRT curvature) ── */
            vec2 cc = uv - 0.5;                    // centered coords -0.5..0.5
            float r2 = dot(cc, cc);
            float distort = 1.0 + r2 * 0.8;        // barrel strength
            vec2 curved = cc * distort + 0.5;

            /* Outside curved screen → keep transparent so the effect never hides chat content */
            if (curved.x < 0.0 || curved.x > 1.0 ||
                curved.y < 0.0 || curved.y > 1.0) {
                gl_FragColor = vec4(0.0, 0.0, 0.0, 0.0);
                return;
            }

            /* ── Scanlines (very subtle) ── */
            float scanY = curved.y * u_res.y;
            float scanline = 0.92 + 0.08 * sin(scanY * 3.14159);

            /* ── Vignette (CRT edge darkening) ── */
            float dist = length(cc);
            float vignette = smoothstep(0.75, 0.25, dist);

            /* ── Phosphor glow (green tint at center) ── */
            float glow = smoothstep(0.6, 0.0, dist) * 0.04;

            /* ── Subtle flicker ── */
            float flicker = 0.995 + 0.005 * sin(u_time * 12.0);

            /* ── Rounded screen corners ── */
            vec2 edgeDist = min(curved, 1.0 - curved);
            float corner = smoothstep(0.0, 0.04, min(edgeDist.x, edgeDist.y));

            /* ── Compose ── */
            float brightness = scanline * vignette * flicker * corner;

            /* Green phosphor overlay color */
            vec3 phosphor = vec3(0.12, 0.55, 0.06);

            /* Final color: mostly transparent with subtle green phosphor + darkening at edges */
            float alpha = (1.0 - vignette) * 0.16 + glow;
            alpha *= corner;

            gl_FragColor = vec4(phosphor * glow, alpha);
        }
    `;

    /* ── WebGL helpers ── */
    function createShader(type, src) {
        const s = gl.createShader(type);
        gl.shaderSource(s, src);
        gl.compileShader(s);
        if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) {
            console.warn('[CRT] Shader error:', gl.getShaderInfoLog(s));
            gl.deleteShader(s);
            return null;
        }
        return s;
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
            zIndex: '999999',
            imageRendering: 'auto'
        });
        document.documentElement.appendChild(canvas);

        gl = canvas.getContext('webgl', { alpha: true, premultipliedAlpha: false, antialias: false });
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

        /* Full-screen quad */
        const buf = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, buf);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1, 1,-1, -1,1, 1,1]), gl.STATIC_DRAW);
        const aPos = gl.getAttribLocation(program, 'a_pos');
        gl.enableVertexAttribArray(aPos);
        gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

        uniforms.u_time = gl.getUniformLocation(program, 'u_time');
        uniforms.u_res = gl.getUniformLocation(program, 'u_res');

        gl.enable(gl.BLEND);
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

        return true;
    }

    function resize() {
        if (!canvas) return;
        const dpr = window.devicePixelRatio || 1;
        canvas.width = window.innerWidth * dpr;
        canvas.height = window.innerHeight * dpr;
        if (gl) gl.viewport(0, 0, canvas.width, canvas.height);
    }

    function render(time) {
        if (!active || !gl) return;
        const t = time * 0.001;
        gl.uniform1f(uniforms.u_time, t);
        gl.uniform2f(uniforms.u_res, canvas.width, canvas.height);
        gl.clearColor(0, 0, 0, 0);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
        animId = requestAnimationFrame(render);
    }

    function start() {
        if (active) return;
        if (!canvas && !initGL()) return;
        active = true;
        canvas.style.display = 'block';
        resize();
        animId = requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animId) cancelAnimationFrame(animId);
        animId = null;
        if (canvas) canvas.style.display = 'none';
    }

    function checkTheme() {
        const isCRT = document.documentElement.getAttribute('data-theme') === 'retro-crt';
        if (isCRT && !active) start();
        else if (!isCRT && active) stop();
    }

    /* ── Init ── */
    function init() {
        /* Listen for theme changes */
        window.addEventListener('aurago:themechange', checkTheme);

        /* Also observe data-theme attribute changes */
        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(checkTheme).observe(
                document.documentElement,
                { attributes: true, attributeFilter: ['data-theme'] }
            );
        }

        /* Check initial state */
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', checkTheme);
        } else {
            checkTheme();
        }

        /* Resize handler */
        window.addEventListener('resize', () => { if (active) resize(); });
    }

    /* Expose for external control */
    window.AuraGoCRT = { start, stop, checkTheme };

    init();
})();
