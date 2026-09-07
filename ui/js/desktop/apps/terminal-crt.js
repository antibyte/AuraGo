(function () {
    'use strict';

    const VERT = [
        'attribute vec2 a_pos;',
        'varying vec2 v_uv;',
        'void main() {',
        '    v_uv = a_pos * 0.5 + 0.5;',
        '    gl_Position = vec4(a_pos, 0.0, 1.0);',
        '}'
    ].join('\n');

    const FRAG = [
        'precision mediump float;',
        'varying vec2 v_uv;',
        'uniform sampler2D u_tex;',
        'uniform sampler2D u_prev;',
        'uniform vec2 u_res;',
        'uniform float u_time;',
        'uniform vec3 u_phosphor;',
        'uniform float u_curve;',
        'uniform float u_bloom;',
        'uniform float u_burn;',
        'uniform float u_noise;',
        'uniform float u_flicker;',
        'uniform float u_mask;',
        'uniform float u_alpha;',
        'uniform float u_motion;',
        'uniform float u_scan;',
        'float hash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }',
        'void main() {',
        '    vec2 cc = v_uv - 0.5;',
        '    float r2 = dot(cc, cc);',
        '    vec2 curved = cc * (1.0 + r2 * u_curve) + 0.5;',
        '    if (curved.x < 0.0 || curved.x > 1.0 || curved.y < 0.0 || curved.y > 1.0) {',
        '        gl_FragColor = vec4(0.0, 0.0, 0.0, u_alpha);',
        '        return;',
        '    }',
        '    vec3 src = texture2D(u_tex, curved).rgb;',
        '    vec3 prev = texture2D(u_prev, curved).rgb;',
        '    vec2 px = 1.0 / max(u_res, vec2(1.0));',
        '    vec3 bloom = (texture2D(u_tex, curved + vec2(px.x, 0.0)).rgb + texture2D(u_tex, curved - vec2(px.x, 0.0)).rgb + texture2D(u_tex, curved + vec2(0.0, px.y)).rgb + texture2D(u_tex, curved - vec2(0.0, px.y)).rgb) * 0.25;',
        '    float scan = 0.55 + 0.45 * sin(curved.y * u_res.y * 3.14159);',
        '    float flicker = 1.0 - u_flicker * u_motion * (0.5 + 0.5 * sin(u_time * 37.0));',
        '    float grain = (hash(curved * u_res + u_time) - 0.5) * u_noise;',
        '    float mask = 1.0 - u_mask * 0.35 * abs(sin(curved.x * u_res.x * 3.14159));',
        '    vec3 color = mix(src, bloom, u_bloom);',
        '    color = mix(color, prev, u_burn * u_motion);',
        '    color *= mix(1.0, scan, u_scan);',
        '    color *= flicker * mask;',
        '    color += grain;',
        '    color *= u_phosphor;',
        '    color *= smoothstep(0.95, 0.35, length(cc));',
        '    gl_FragColor = vec4(color, u_alpha);',
        '}'
    ].join('\n');

    function reducedMotion() {
        return (document.body && document.body.dataset.animations === 'false')
            || (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function compile(gl, type, src) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, src);
        gl.compileShader(shader);
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            gl.deleteShader(shader);
            return null;
        }
        return shader;
    }

    window.TerminalCrt = {
        create(opts) {
            const host = opts.host;
            const screen = opts.screen;
            const getWindowEl = opts.getWindowEl || function () {
                return host && host.closest ? host.closest('.vd-window') : null;
            };
            let enabled = false;
            let disposed = false;
            let fallback = false;
            let profile = { phosphor: [1, 1, 1], curve: 0, bloom: 0, burn: 0, noise: 0, flicker: 0, mask: 0, alpha: 1, scan: 0 };
            let raf = 0;
            let gl = null;
            let program = null;
            let buffer = null;
            let sourceTex = null;
            let ping = null;
            let pong = null;
            let writePing = true;
            let overlay = document.createElement('canvas');
            overlay.className = 'vd-terminal-crt-overlay';
            overlay.setAttribute('aria-hidden', 'true');
            const scratch = document.createElement('canvas');
            const scratchCtx = scratch.getContext('2d', { alpha: true });
            const onLost = function (event) {
                event.preventDefault();
                useFallback();
            };

            function useFallback() {
                fallback = true;
                enabled = false;
                if (host) host.setAttribute('data-terminal-fallback', 'css');
                stopLoop();
                destroyGl();
            }

            function destroyGl() {
                if (overlay) overlay.removeEventListener('webglcontextlost', onLost);
                if (gl && program) gl.deleteProgram(program);
                if (gl && buffer) gl.deleteBuffer(buffer);
                if (gl && sourceTex) gl.deleteTexture(sourceTex);
                if (gl && ping) gl.deleteTexture(ping);
                if (gl && pong) gl.deleteTexture(pong);
                program = null;
                buffer = null;
                sourceTex = null;
                ping = null;
                pong = null;
                gl = null;
            }

            function createTexture(targetGl) {
                const tex = targetGl.createTexture();
                targetGl.bindTexture(targetGl.TEXTURE_2D, tex);
                targetGl.texParameteri(targetGl.TEXTURE_2D, targetGl.TEXTURE_MIN_FILTER, targetGl.LINEAR);
                targetGl.texParameteri(targetGl.TEXTURE_2D, targetGl.TEXTURE_MAG_FILTER, targetGl.LINEAR);
                targetGl.texParameteri(targetGl.TEXTURE_2D, targetGl.TEXTURE_WRAP_S, targetGl.CLAMP_TO_EDGE);
                targetGl.texParameteri(targetGl.TEXTURE_2D, targetGl.TEXTURE_WRAP_T, targetGl.CLAMP_TO_EDGE);
                return tex;
            }

            function initGl() {
                if (!screen) return false;
                screen.appendChild(overlay);
                gl = overlay.getContext('webgl') || overlay.getContext('experimental-webgl');
                if (!gl) return false;
                overlay.addEventListener('webglcontextlost', onLost, false);
                const vs = compile(gl, gl.VERTEX_SHADER, VERT);
                const fs = compile(gl, gl.FRAGMENT_SHADER, FRAG);
                if (!vs || !fs) return false;
                program = gl.createProgram();
                gl.attachShader(program, vs);
                gl.attachShader(program, fs);
                gl.bindAttribLocation(program, 0, 'a_pos');
                gl.linkProgram(program);
                gl.deleteShader(vs);
                gl.deleteShader(fs);
                if (!gl.getProgramParameter(program, gl.LINK_STATUS)) return false;
                buffer = gl.createBuffer();
                gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
                gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
                sourceTex = createTexture(gl);
                ping = createTexture(gl);
                pong = createTexture(gl);
                return true;
            }

            if (!initGl()) useFallback();

            function visible() {
                if (disposed || !enabled || fallback) return false;
                if (document.hidden) return false;
                const win = getWindowEl();
                if (win && (win.style.display === 'none' || win.classList.contains('vd-space-hidden'))) return false;
                return true;
            }

            function resize() {
                if (!overlay || !screen) return;
                const rect = screen.getBoundingClientRect();
                const dpr = Math.min(window.devicePixelRatio || 1, 1.25);
                const width = Math.max(2, Math.floor(rect.width * dpr));
                const height = Math.max(2, Math.floor(rect.height * dpr));
                overlay.width = width;
                overlay.height = height;
                overlay.style.width = '100%';
                overlay.style.height = '100%';
                scratch.width = width;
                scratch.height = height;
                if (gl) {
                    gl.viewport(0, 0, width, height);
                    const burnW = Math.max(1, Math.floor(width / 2));
                    const burnH = Math.max(1, Math.floor(height / 2));
                    [ping, pong].forEach(function (tex) {
                        gl.bindTexture(gl.TEXTURE_2D, tex);
                        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, burnW, burnH, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
                    });
                }
            }

            function sourceCanvas() {
                if (!screen) return null;
                const canvases = screen.querySelectorAll('canvas');
                const layers = [];
                for (let i = 0; i < canvases.length; i += 1) {
                    if (canvases[i] !== overlay && canvases[i].width && canvases[i].height) layers.push(canvases[i]);
                }
                if (!layers.length || !scratchCtx) return null;
                scratchCtx.clearRect(0, 0, scratch.width, scratch.height);
                layers.forEach(function (layer) {
                    scratchCtx.drawImage(layer, 0, 0, scratch.width, scratch.height);
                });
                return scratch;
            }

            function loc(name) {
                return gl.getUniformLocation(program, name);
            }

            function frame(now) {
                raf = 0;
                if (!visible() || !gl || !program) return;
                const src = sourceCanvas();
                if (!src) {
                    startLoop();
                    return;
                }
                gl.useProgram(program);
                gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
                gl.enableVertexAttribArray(0);
                gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
                gl.activeTexture(gl.TEXTURE0);
                gl.bindTexture(gl.TEXTURE_2D, sourceTex);
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, src);
                gl.uniform1i(loc('u_tex'), 0);
                gl.activeTexture(gl.TEXTURE1);
                gl.bindTexture(gl.TEXTURE_2D, writePing ? ping : pong);
                gl.uniform1i(loc('u_prev'), 1);
                gl.uniform2f(loc('u_res'), overlay.width, overlay.height);
                gl.uniform1f(loc('u_time'), now / 1000);
                gl.uniform3f(loc('u_phosphor'), profile.phosphor[0], profile.phosphor[1], profile.phosphor[2]);
                gl.uniform1f(loc('u_curve'), profile.curve);
                gl.uniform1f(loc('u_bloom'), profile.bloom);
                gl.uniform1f(loc('u_burn'), profile.burn);
                gl.uniform1f(loc('u_noise'), profile.noise);
                gl.uniform1f(loc('u_flicker'), profile.flicker);
                gl.uniform1f(loc('u_mask'), profile.mask);
                gl.uniform1f(loc('u_alpha'), profile.alpha);
                gl.uniform1f(loc('u_motion'), reducedMotion() ? 0.0 : 1.0);
                gl.uniform1f(loc('u_scan'), profile.scan);
                gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
                if (!reducedMotion()) writePing = !writePing;
                startLoop();
            }

            function startLoop() {
                if (raf || disposed || !enabled || fallback) return;
                raf = window.requestAnimationFrame(frame);
            }

            function stopLoop() {
                if (raf) window.cancelAnimationFrame(raf);
                raf = 0;
            }

            function setProfile(next) {
                if (next && next.crt) profile = next.crt;
            }

            function setEnabled(next) {
                enabled = !!next && !fallback;
                if (enabled) {
                    if (host) host.removeAttribute('data-terminal-fallback');
                    resize();
                    startLoop();
                } else {
                    stopLoop();
                }
            }

            function dispose() {
                disposed = true;
                enabled = false;
                stopLoop();
                destroyGl();
                if (overlay && overlay.parentNode) overlay.parentNode.removeChild(overlay);
            }

            resize();
            return { setProfile, setEnabled, resize, dispose, usesFallback: function () { return fallback; } };
        }
    };
})();
