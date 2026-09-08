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
        'precision highp float;',
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
        'uniform float u_mono;',
        'uniform vec2 u_size;',
        'float hash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }',
        'vec3 phosphor(vec3 c) {',
        '    float light = max(c.r, max(c.g, c.b));',
        '    return mix(c, u_phosphor * light, u_mono);',
        '}',
        'void main() {',
        '    vec2 cc = v_uv - 0.5;',
        '    vec2 curved = cc * (1.0 + cc.yx * cc.yx * u_curve) + 0.5;',
        '    if (curved.x < 0.0 || curved.x > 1.0 || curved.y < 0.0 || curved.y > 1.0) {',
        '        gl_FragColor = vec4(0.0, 0.0, 0.0, u_alpha);',
        '        return;',
        '    }',
        '    vec3 src = texture2D(u_tex, curved).rgb;',
        '    vec3 prev = texture2D(u_prev, curved).rgb;',
        '    vec3 bloom = prev;',
        '    float scan = 1.0 - u_scan * (0.14 + 0.14 * cos(curved.y * u_size.y * 3.14159));',
        '    float flicker = 1.0 - u_flicker * u_motion * (0.5 + 0.5 * sin(u_time * 47.0));',
        '    float grain = (hash(floor(v_uv * u_res) + floor(u_time * 24.0)) - 0.5) * u_noise;',
        '    float mask = 1.0 - u_mask * 0.12 * (0.5 + 0.5 * cos(curved.x * u_size.x * 3.14159));',
        '    vec3 light = max(src, prev * u_burn * u_motion);',
        '    vec3 color = phosphor(light) * 1.18 + phosphor(bloom) * u_bloom * 1.35;',
        '    color += pow(max(light.r, max(light.g, light.b)), 3.0) * u_phosphor * 0.12;',
        '    color *= scan * flicker * mask;',
        '    float vignette = 1.0 - 0.48 * smoothstep(0.15, 0.72, length(cc));',
        '    float edge = min(min(curved.x, 1.0 - curved.x), min(curved.y, 1.0 - curved.y));',
        '    color += u_phosphor * (0.018 + grain * 0.35);',
        '    color *= vignette * smoothstep(0.0, 0.018, edge);',
        '    gl_FragColor = vec4(color, u_alpha);',
        '}'
    ].join('\n');

    const SOURCE_WAIT_MS = 2000;

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
            let background = '#000000';
            let monochrome = true;
            let raf = 0;
            let gl = null;
            let program = null;
            let buffer = null;
            let sourceTex = null;
            let historyTex = null;
            let lastFrame = 0;
            let burnW = 1;
            let burnH = 1;
            let sourceWaitStarted = 0;
            let observer = null;
            let overlay = document.createElement('canvas');
            overlay.className = 'vd-terminal-crt-overlay';
            overlay.setAttribute('aria-hidden', 'true');
            const scratch = document.createElement('canvas');
            const scratchCtx = scratch.getContext('2d', { alpha: true });
            const burnCanvas = document.createElement('canvas');
            const burnCtx = burnCanvas.getContext('2d', { alpha: true });
            const onLost = function (event) {
                event.preventDefault();
                useFallback();
            };

            function hideOverlay() {
                if (host) host.removeAttribute('data-terminal-renderer');
                if (!overlay) return;
                overlay.style.display = 'none';
                if (overlay.parentNode) overlay.parentNode.removeChild(overlay);
            }

            function showOverlay() {
                if (!overlay || fallback || !screen) return;
                overlay.style.display = '';
                if (!overlay.parentNode) screen.appendChild(overlay);
            }

            function useFallback() {
                fallback = true;
                enabled = false;
                if (host) host.setAttribute('data-terminal-fallback', 'css');
                stopLoop();
                destroyGl();
                hideOverlay();
            }

            function destroyGl() {
                if (overlay) overlay.removeEventListener('webglcontextlost', onLost);
                if (gl && program) gl.deleteProgram(program);
                if (gl && buffer) gl.deleteBuffer(buffer);
                if (gl && sourceTex) gl.deleteTexture(sourceTex);
                if (gl && historyTex) gl.deleteTexture(historyTex);
                program = null;
                buffer = null;
                sourceTex = null;
                historyTex = null;
                gl = null;
            }

            function createTexture(targetGl, nearest) {
                const tex = targetGl.createTexture();
                const filter = nearest ? targetGl.NEAREST : targetGl.LINEAR;
                targetGl.bindTexture(targetGl.TEXTURE_2D, tex);
                targetGl.texParameteri(targetGl.TEXTURE_2D, targetGl.TEXTURE_MIN_FILTER, filter);
                targetGl.texParameteri(targetGl.TEXTURE_2D, targetGl.TEXTURE_MAG_FILTER, filter);
                targetGl.texParameteri(targetGl.TEXTURE_2D, targetGl.TEXTURE_WRAP_S, targetGl.CLAMP_TO_EDGE);
                targetGl.texParameteri(targetGl.TEXTURE_2D, targetGl.TEXTURE_WRAP_T, targetGl.CLAMP_TO_EDGE);
                return tex;
            }

            function initGl() {
                if (!screen || !scratchCtx || !burnCtx) return false;
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
                sourceTex = createTexture(gl, true);
                historyTex = createTexture(gl, false);
                return true;
            }

            if (!initGl()) useFallback();

            function isOnScreen() {
                if (document.hidden) return false;
                const win = getWindowEl();
                if (win && (win.style.display === 'none' || win.classList.contains('vd-space-hidden'))) return false;
                return true;
            }

            function capturePreviousOutput(src, elapsed, motion) {
                if (!gl || !historyTex || !burnCtx) return;
                // Accumulate unwarped text, never feed the distorted glass back into itself.
                burnCtx.globalCompositeOperation = 'source-over';
                burnCtx.filter = 'none';
                burnCtx.fillStyle = background;
                burnCtx.globalAlpha = motion ? 1 - Math.exp(-elapsed / (45 + profile.burn * 220)) : 1;
                burnCtx.fillRect(0, 0, burnW, burnH);
                burnCtx.globalAlpha = 1;
                burnCtx.globalCompositeOperation = 'lighten';
                burnCtx.filter = 'blur(2px)';
                burnCtx.drawImage(src, 0, 0, burnW, burnH);
                gl.bindTexture(gl.TEXTURE_2D, historyTex);
                gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, burnCanvas);
            }

            function resize() {
                if (!overlay || !screen || fallback) return;
                const rect = screen.getBoundingClientRect();
                const dpr = Math.min(window.devicePixelRatio || 1, 1.25);
                const width = Math.max(2, Math.floor(rect.width * dpr));
                const height = Math.max(2, Math.floor(rect.height * dpr));
                if (overlay.width === width && overlay.height === height) return;
                overlay.width = width;
                overlay.height = height;
                overlay.style.width = '100%';
                overlay.style.height = '100%';
                scratch.width = width;
                scratch.height = height;
                burnW = Math.max(1, Math.floor(width / 2));
                burnH = Math.max(1, Math.floor(height / 2));
                burnCanvas.width = burnW;
                burnCanvas.height = burnH;
                if (gl) {
                    gl.viewport(0, 0, width, height);
                }
            }

            function sourceCanvas() {
                if (!screen) return null;
                const canvases = screen.querySelectorAll('canvas');
                const layers = [];
                for (let i = 0; i < canvases.length; i += 1) {
                    const node = canvases[i];
                    const cls = node.className || '';
                    if (node === overlay || cls.indexOf('xterm-') < 0 || cls.indexOf('-layer') < 0) continue;
                    if (!node.width || !node.height) continue;
                    layers.push(node);
                }
                if (!layers.length || !scratchCtx) return null;
                const rect = screen.getBoundingClientRect();
                if (!rect.width || !rect.height) return null;
                const scaleX = scratch.width / rect.width;
                const scaleY = scratch.height / rect.height;
                scratchCtx.fillStyle = background;
                scratchCtx.fillRect(0, 0, scratch.width, scratch.height);
                layers.forEach(function (layer) {
                    const bounds = layer.getBoundingClientRect();
                    scratchCtx.drawImage(layer, (bounds.left - rect.left) * scaleX, (bounds.top - rect.top) * scaleY, bounds.width * scaleX, bounds.height * scaleY);
                });
                return scratch;
            }

            function loc(name) {
                return gl.getUniformLocation(program, name);
            }

            function frame(now) {
                raf = 0;
                if (disposed || fallback || !enabled) return;
                if (!isOnScreen() || !gl || !program) {
                    stopLoop();
                    return;
                }
                if (lastFrame && now - lastFrame < 1000 / 30) { startLoop(); return; }
                const elapsed = lastFrame ? now - lastFrame : 1000;
                lastFrame = now;
                const src = sourceCanvas();
                if (!src) {
                    const stamp = now || (window.performance && performance.now()) || Date.now();
                    if (!sourceWaitStarted) sourceWaitStarted = stamp;
                    if (stamp - sourceWaitStarted >= SOURCE_WAIT_MS) {
                        useFallback();
                        return;
                    }
                    startLoop();
                    return;
                }
                sourceWaitStarted = 0;
                const motion = reducedMotion() ? 0.0 : 1.0;
                gl.useProgram(program);
                gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
                gl.enableVertexAttribArray(0);
                gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
                gl.activeTexture(gl.TEXTURE0);
                gl.bindTexture(gl.TEXTURE_2D, sourceTex);
                gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, src);
                gl.uniform1i(loc('u_tex'), 0);
                gl.activeTexture(gl.TEXTURE1);
                capturePreviousOutput(src, elapsed, motion);
                gl.uniform1i(loc('u_prev'), 1);
                gl.uniform2f(loc('u_res'), overlay.width, overlay.height);
                gl.uniform2f(loc('u_size'), screen.clientWidth, screen.clientHeight);
                gl.uniform1f(loc('u_time'), motion ? now / 1000 : 0);
                gl.uniform1f(loc('u_mono'), monochrome ? 1 : 0);
                gl.uniform3f(loc('u_phosphor'), profile.phosphor[0], profile.phosphor[1], profile.phosphor[2]);
                gl.uniform1f(loc('u_curve'), profile.curve);
                gl.uniform1f(loc('u_bloom'), profile.bloom);
                gl.uniform1f(loc('u_burn'), profile.burn);
                gl.uniform1f(loc('u_noise'), profile.noise);
                gl.uniform1f(loc('u_flicker'), profile.flicker);
                gl.uniform1f(loc('u_mask'), profile.mask);
                gl.uniform1f(loc('u_alpha'), profile.alpha);
                gl.uniform1f(loc('u_motion'), motion);
                gl.uniform1f(loc('u_scan'), profile.scan);
                gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
                if (host) host.setAttribute('data-terminal-renderer', 'webgl');
                startLoop();
            }

            function startLoop() {
                if (raf || disposed || !enabled || fallback || !isOnScreen()) return;
                raf = window.requestAnimationFrame(frame);
            }

            function stopLoop() {
                if (raf) window.cancelAnimationFrame(raf);
                raf = 0;
            }

            function onVisibility() {
                if (disposed || !enabled || fallback) return;
                if (isOnScreen()) startLoop();
                else stopLoop();
            }

            function watchWindow() {
                if (observer) {
                    observer.disconnect();
                    observer = null;
                }
                const win = getWindowEl();
                if (!win || typeof MutationObserver === 'undefined') return;
                observer = new MutationObserver(onVisibility);
                observer.observe(win, { attributes: true, attributeFilter: ['class', 'style', 'data-space-hidden'] });
            }

            function setProfile(next) {
                if (next && next.crt) {
                    profile = next.crt;
                    background = next.theme.background;
                    monochrome = next.id !== 'commodore64';
                    if (burnCtx) burnCtx.clearRect(0, 0, burnW, burnH);
                    lastFrame = 0;
                }
            }

            function setEnabled(next) {
                enabled = !!next && !fallback;
                if (enabled) {
                    if (host) host.removeAttribute('data-terminal-fallback');
                    sourceWaitStarted = 0;
                    showOverlay();
                    watchWindow();
                    resize();
                    startLoop();
                } else {
                    stopLoop();
                    hideOverlay();
                }
            }

            function dispose() {
                disposed = true;
                enabled = false;
                stopLoop();
                document.removeEventListener('visibilitychange', onVisibility);
                if (observer) {
                    observer.disconnect();
                    observer = null;
                }
                destroyGl();
                hideOverlay();
            }

            document.addEventListener('visibilitychange', onVisibility);
            watchWindow();
            resize();
            if (!enabled && !fallback) hideOverlay();
            return { setProfile, setEnabled, resize, dispose, usesFallback: function () { return fallback; } };
        }
    };
})();
