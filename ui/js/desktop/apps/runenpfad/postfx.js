/* Runenpfad GPU compositor: plain WebGL2, no libraries.
 * The 2D canvas is uploaded as a texture; bloom, god rays, shockwaves, heat haze,
 * chromatic aberration, per-biome grading, vignette and grain happen in shaders.
 * Every failure path calls onFallback once and leaves the 2D canvas visible. */
(function (root) {
    'use strict';

    const MAX_RINGS = 4;
    const RING_LIFE = 0.6;
    const WATCH_FRAMES = 120;
    const WATCH_LIMIT_MS = 6;
    const WATCH_SPIKE_MS = 250;

    // lift (rgb), gain (rgb), gamma, saturation per biome; deliberately subtle.
    const GRADES = {
        woodland: { lift: [0.000, 0.004, 0.000], gain: [1.02, 1.03, 0.97], gamma: 1.00, sat: 1.08 },
        cavern: { lift: [0.004, 0.000, 0.012], gain: [0.96, 0.98, 1.06], gamma: 0.97, sat: 0.98 },
        desert: { lift: [0.008, 0.003, 0.000], gain: [1.06, 1.00, 0.90], gamma: 1.02, sat: 1.06 },
        glacier: { lift: [0.000, 0.004, 0.010], gain: [0.94, 1.00, 1.08], gamma: 1.02, sat: 0.97 },
        citadel: { lift: [0.006, 0.000, 0.006], gain: [1.04, 0.96, 1.02], gamma: 1.04, sat: 1.04 }
    };

    const VERT = '#version 300 es\n' +
        'out vec2 vUv;\n' +
        'void main(){ vec2 p = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));\n' +
        'vUv = p; gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0); }\n';

    const HEAD = '#version 300 es\nprecision highp float;\nin vec2 vUv;\nout vec4 o;\n';

    // vUv is GL-native (v = 0 at the bottom). The source canvas texture has its top row at v = 0,
    // so the source is sampled with the flipped coordinate sv; intermediate targets stay native.
    const FRAG_BRIGHT = HEAD +
        'uniform sampler2D uSrc; uniform vec2 uTexel; uniform float uThr;\n' +
        'vec3 tap(vec2 sv){ vec3 c = texture(uSrc, sv).rgb; float b = max(c.r, max(c.g, c.b));\n' +
        '  float k = 0.12; float s = clamp(b - uThr + k, 0.0, 2.0 * k); s = s * s / (4.0 * k + 1e-4);\n' +
        '  float w = max(s, b - uThr) / max(b, 1e-4); return c * w; }\n' +
        'void main(){ vec2 sv = vec2(vUv.x, 1.0 - vUv.y); vec2 t = uTexel * 1.5;\n' +
        '  vec3 a = tap(sv + vec2(-t.x, -t.y)) + tap(sv + vec2(t.x, -t.y)) + tap(sv + vec2(-t.x, t.y)) + tap(sv + vec2(t.x, t.y));\n' +
        '  o = vec4(a * 0.25, 1.0); }\n';

    const FRAG_BLUR = HEAD +
        'uniform sampler2D uSrc; uniform vec2 uDir;\n' +
        'void main(){ vec3 a = texture(uSrc, vUv).rgb * 0.227027;\n' +
        '  a += (texture(uSrc, vUv + uDir * 1.3846153).rgb + texture(uSrc, vUv - uDir * 1.3846153).rgb) * 0.3162162;\n' +
        '  a += (texture(uSrc, vUv + uDir * 3.2307692).rgb + texture(uSrc, vUv - uDir * 3.2307692).rgb) * 0.0702702;\n' +
        '  o = vec4(a, 1.0); }\n';

    const FRAG_RAYS = HEAD +
        'uniform sampler2D uSrc; uniform vec2 uSun;\n' +
        'void main(){ vec2 d = (uSun - vUv) * 0.85 / 16.0; vec2 p = vUv; float w = 1.0; vec3 a = vec3(0.0);\n' +
        '  for (int i = 0; i < 16; i++) { p += d; a += texture(uSrc, p).rgb * w; w *= 0.92; }\n' +
        '  o = vec4(a / 16.0, 1.0); }\n';

    const FRAG_COMP = HEAD +
        'uniform sampler2D uSrc; uniform sampler2D uBloom; uniform sampler2D uRays;\n' +
        'uniform vec4 uRing[4]; uniform float uAspect; uniform float uTime; uniform float uHeat; uniform float uHurt;\n' +
        'uniform float uLowHp; uniform float uSunK; uniform float uGrain; uniform float uMotion; uniform vec2 uSize;\n' +
        'uniform vec3 uLift; uniform vec3 uGain; uniform float uGamma; uniform float uSat;\n' +
        'float hash(vec2 p){ vec3 q = fract(vec3(p.xyx) * 0.1031); q += dot(q, q.yzx + 33.33); return fract((q.x + q.y) * q.z); }\n' +
        'void main(){\n' +
        '  vec2 uv = vec2(vUv.x, 1.0 - vUv.y); vec2 off = vec2(0.0);\n' +
        '  if (uMotion > 0.5) {\n' +
        '    for (int i = 0; i < 4; i++) { vec4 r = uRing[i]; if (r.w > 0.0) {\n' +
        '      vec2 d = uv - r.xy; d.x *= uAspect; float len = length(d); float rad = r.z * 0.55;\n' +
        '      float band = exp(-pow((len - rad) / 0.035, 2.0)) * (1.0 - r.z) * r.w;\n' +
        '      vec2 dir = d / max(len, 1e-4); dir.x /= uAspect; off += dir * band * 0.03; } }\n' +
        '    float m = smoothstep(0.33, 0.62, uv.y) * uHeat;\n' +
        '    off += vec2(sin(uv.y * 70.0 + uTime * 3.1) * 0.0016, sin(uv.x * 45.0 + uTime * 2.3) * 0.0008) * m;\n' +
        '  }\n' +
        '  vec2 su = clamp(uv + off, 0.001, 0.999);\n' +
        '  vec3 c;\n' +
        '  if (uHurt > 0.01 && uMotion > 0.5) { vec2 ab = (su - 0.5) * 0.02 * uHurt;\n' +
        '    c = vec3(texture(uSrc, su + ab).r, texture(uSrc, su).g, texture(uSrc, su - ab).b); }\n' +
        '  else c = texture(uSrc, su).rgb;\n' +
        '  vec2 bv = vec2(su.x, 1.0 - su.y);\n' +
        '  c += texture(uBloom, bv).rgb * 0.65;\n' +
        '  c += texture(uRays, bv).rgb * vec3(1.0, 0.86, 0.62) * uSunK;\n' +
        '  c = c * uGain + uLift; c = pow(max(c, 0.0), vec3(1.0 / uGamma));\n' +
        '  float l = dot(c, vec3(0.299, 0.587, 0.114)); c = mix(vec3(l), c, uSat);\n' +
        '  vec2 q = uv - 0.5; q.x *= uAspect * 0.8; float dist = length(q) * 1.45;\n' +
        '  float pulse = 0.5 + 0.5 * sin(uTime * 4.2);\n' +
        '  float v = smoothstep(0.45, 1.1, dist);\n' +
        '  c += vec3(0.55, 0.02, 0.03) * v * v * uLowHp * (0.3 + 0.45 * pulse * uMotion + 0.2 * (1.0 - uMotion));\n' +
        '  c *= 1.0 - 0.3 * v;\n' +
        '  c += (hash(uv * uSize + fract(uTime) * 91.7) - 0.5) * uGrain * (0.4 + 0.6 * (1.0 - l));\n' +
        '  o = vec4(clamp(c, 0.0, 1.0), 1.0); }\n';

    function median(list) {
        const a = list.slice().sort(function (x, y) { return x - y; });
        return a.length ? a[a.length >> 1] : 0;
    }

    function create(opts) {
        opts = opts || {};
        const host = opts.host;
        const source = opts.source;
        const reduced = !!opts.reducedMotion;
        const onFallback = typeof opts.onFallback === 'function' ? opts.onFallback : function () {};
        const doc = root.document;
        let canvas = null;
        let gl = null;
        let failed = false;
        let disposed = false;
        let enabled = true;
        let shown = false;

        const res = { programs: [], textures: [], fbos: [] };
        let progs = null;
        let srcTex = null;
        let srcW = 0;
        let srcH = 0;
        let qW = 0;
        let qH = 0;
        let texA = null;
        let texB = null;
        let fboA = null;
        let fboB = null;
        const rings = [];
        const frames = [];
        let slowRun = 0;
        let sinceMedian = 0;
        let lastMs = 0;
        const api = {
            frameMs: 0,
            isEnabled: function () { return enabled && !failed && !disposed; },
            setEnabled: function (on) {
                enabled = !!on;
                if (canvas) canvas.style.display = enabled && !failed ? '' : 'none';
                slowRun = 0;
            },
            resize: function () { /* backing size follows the source canvas every frame */ },
            pushShockwave: function (x, y, strength) {
                if (failed || disposed || reduced) return;
                const ring = { x: +x || 0, y: +y || 0, s: Math.max(0.1, Math.min(2, +strength || 1)), age: 0 };
                if (rings.length >= MAX_RINGS) rings.shift();
                rings.push(ring);
            },
            render: render,
            dispose: dispose
        };

        function fail(reason) {
            if (failed || disposed) return;
            failed = true;
            teardown();
            try { onFallback(String(reason || 'fallback')); } catch (err) { /* ignore */ }
        }

        function teardown() {
            try {
                if (gl) {
                    res.fbos.forEach(function (f) { gl.deleteFramebuffer(f); });
                    res.textures.forEach(function (t) { gl.deleteTexture(t); });
                    res.programs.forEach(function (p) { gl.deleteProgram(p); });
                    const ext = gl.getExtension('WEBGL_lose_context');
                    if (ext) ext.loseContext();
                }
            } catch (err) { /* ignore */ }
            res.fbos.length = res.textures.length = res.programs.length = 0;
            gl = null;
            if (canvas && canvas.parentNode) canvas.parentNode.removeChild(canvas);
            canvas = null;
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            teardown();
        }

        function program(fragSrc) {
            const vs = shader(gl.VERTEX_SHADER, VERT);
            const fs = shader(gl.FRAGMENT_SHADER, fragSrc);
            const p = gl.createProgram();
            gl.attachShader(p, vs);
            gl.attachShader(p, fs);
            gl.linkProgram(p);
            gl.deleteShader(vs);
            gl.deleteShader(fs);
            if (!gl.getProgramParameter(p, gl.LINK_STATUS)) throw new Error('link');
            res.programs.push(p);
            const cache = {};
            return {
                p: p,
                u: function (n) {
                    if (!(n in cache)) cache[n] = gl.getUniformLocation(p, n);
                    return cache[n];
                }
            };
        }

        function shader(type, src) {
            const s = gl.createShader(type);
            gl.shaderSource(s, src);
            gl.compileShader(s);
            if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) throw new Error('compile');
            return s;
        }

        function makeTex(w, h) {
            const t = gl.createTexture();
            res.textures.push(t);
            gl.bindTexture(gl.TEXTURE_2D, t);
            gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
            return t;
        }

        function makeFbo(tex) {
            const f = gl.createFramebuffer();
            res.fbos.push(f);
            gl.bindFramebuffer(gl.FRAMEBUFFER, f);
            gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0);
            if (gl.checkFramebufferStatus(gl.FRAMEBUFFER) !== gl.FRAMEBUFFER_COMPLETE) throw new Error('framebuffer');
            return f;
        }

        function drop(list, item) {
            const i = list.indexOf(item);
            if (i >= 0) { list.splice(i, 1); }
        }

        function allocTargets(w, h) {
            const max = gl.getParameter(gl.MAX_TEXTURE_SIZE);
            if (w > max || h > max) throw new Error('size');
            [srcTex, texA, texB].forEach(function (t) { if (t) { gl.deleteTexture(t); drop(res.textures, t); } });
            [fboA, fboB].forEach(function (f) { if (f) { gl.deleteFramebuffer(f); drop(res.fbos, f); } });
            srcW = w;
            srcH = h;
            qW = Math.max(2, Math.ceil(w / 4));
            qH = Math.max(2, Math.ceil(h / 4));
            srcTex = makeTex(w, h);
            texA = makeTex(qW, qH);
            texB = makeTex(qW, qH);
            fboA = makeFbo(texA);
            fboB = makeFbo(texB);
            canvas.width = w;
            canvas.height = h;
        }

        function bind(unit, tex, loc) {
            gl.activeTexture(gl.TEXTURE0 + unit);
            gl.bindTexture(gl.TEXTURE_2D, tex);
            gl.uniform1i(loc, unit);
        }

        function pass(pr, fbo, w, h) {
            gl.useProgram(pr.p);
            gl.bindFramebuffer(gl.FRAMEBUFFER, fbo);
            gl.viewport(0, 0, w, h);
            return pr;
        }

        // Watches the CPU time spent inside render() (upload + draw calls; GPU backpressure blocks
        // there). The rAF interval is ignored so throttled pages do not look slow.
        function watch(cost) {
            if (!(cost >= 0)) return;
            slowRun = cost > WATCH_SPIKE_MS ? slowRun + 1 : 0;
            if (slowRun >= 3) { fail('slow-frames ' + Math.round(cost) + 'ms'); return; }
            frames.push(cost);
            if (frames.length > WATCH_FRAMES) frames.shift();
            if (++sinceMedian >= 15 || frames.length === WATCH_FRAMES) {
                sinceMedian = 0;
                lastMs = median(frames);
                api.frameMs = Math.round(lastMs * 10) / 10;
                if (frames.length >= WATCH_FRAMES && lastMs > WATCH_LIMIT_MS) fail('slow-frames ' + api.frameMs + 'ms');
            }
        }

        function render(frame) {
            if (failed || disposed || !enabled || !gl) return;
            try {
                frame = frame || {};
                const w = source.width | 0;
                const h = source.height | 0;
                if (w < 2 || h < 2) return;
                if (gl.isContextLost()) { fail('context-lost'); return; }
                const t0 = root.performance.now();
                if (w !== srcW || h !== srcH) {
                    allocTargets(w, h);
                    gl.activeTexture(gl.TEXTURE0);
                    gl.bindTexture(gl.TEXTURE_2D, srcTex);
                    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, false);
                    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, source);
                } else {
                    gl.activeTexture(gl.TEXTURE0);
                    gl.bindTexture(gl.TEXTURE_2D, srcTex);
                    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, false);
                    gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, gl.RGBA, gl.UNSIGNED_BYTE, source);
                }
                const dt = Math.max(0, Math.min(0.1, +frame.dt || 0));
                const cssW = (host && host.clientWidth) || w;
                const cssH = (host && host.clientHeight) || h;

                // 1. bright pass to quarter resolution
                let p = pass(progs.bright, fboA, qW, qH);
                bind(0, srcTex, p.u('uSrc'));
                gl.uniform2f(p.u('uTexel'), 1 / w, 1 / h);
                gl.uniform1f(p.u('uThr'), 0.72);
                gl.drawArrays(gl.TRIANGLES, 0, 3);

                // 2. separable blur, two iterations (A -> B -> A, wider the second time)
                const blurs = [[fboB, texA, 1 / qW, 0], [fboA, texB, 0, 1 / qH], [fboB, texA, 1.8 / qW, 0], [fboA, texB, 0, 1.8 / qH]];
                p = progs.blur;
                for (let i = 0; i < blurs.length; i++) {
                    const b = blurs[i];
                    pass(p, b[0], qW, qH);
                    bind(0, b[1], p.u('uSrc'));
                    gl.uniform2f(p.u('uDir'), b[2], b[3]);
                    gl.drawArrays(gl.TRIANGLES, 0, 3);
                }

                // 3. god rays from the sun (bloom in A -> rays in B)
                const sun = frame.sun;
                let sunK = 0;
                if (sun && isFinite(sun.x) && isFinite(sun.y)) {
                    const sx = sun.x / cssW;
                    const sy = 1 - sun.y / cssH;
                    const edge = Math.max(0, 1 - Math.max(Math.abs(sx - 0.5) - 0.5, Math.abs(sy - 0.5) - 0.5) * 2);
                    sunK = 0.5 * Math.min(1, edge) * (typeof sun.strength === 'number' ? sun.strength : 1);
                    p = pass(progs.rays, fboB, qW, qH);
                    bind(0, texA, p.u('uSrc'));
                    gl.uniform2f(p.u('uSun'), sx, sy);
                    gl.drawArrays(gl.TRIANGLES, 0, 3);
                }

                // 4. composite to the canvas
                p = pass(progs.comp, null, w, h);
                bind(0, srcTex, p.u('uSrc'));
                bind(1, texA, p.u('uBloom'));
                bind(2, texB, p.u('uRays'));
                const ringData = new Float32Array(16);
                for (let i = rings.length - 1; i >= 0; i--) {
                    rings[i].age += dt;
                    if (rings[i].age >= RING_LIFE) rings.splice(i, 1);
                }
                for (let i = 0; i < rings.length; i++) {
                    const r = rings[i];
                    ringData.set([r.x / cssW, r.y / cssH, r.age / RING_LIFE, r.s], i * 4);
                }
                const g = GRADES[frame.biomeId] || GRADES.woodland;
                const motion = reduced ? 0 : 1;
                gl.uniform4fv(p.u('uRing'), ringData);
                gl.uniform1f(p.u('uAspect'), cssW / cssH);
                gl.uniform1f(p.u('uTime'), (+frame.t || 0) % 1000);
                gl.uniform1f(p.u('uHeat'), Math.max(0, Math.min(1, +frame.heat || 0)));
                gl.uniform1f(p.u('uHurt'), Math.max(0, Math.min(1, +frame.hurt || 0)));
                gl.uniform1f(p.u('uLowHp'), frame.lowHp ? 1 : 0);
                gl.uniform1f(p.u('uSunK'), sunK);
                gl.uniform1f(p.u('uGrain'), reduced ? 0 : 0.035);
                gl.uniform1f(p.u('uMotion'), motion);
                gl.uniform2f(p.u('uSize'), w, h);
                gl.uniform3fv(p.u('uLift'), g.lift);
                gl.uniform3fv(p.u('uGain'), g.gain);
                gl.uniform1f(p.u('uGamma'), g.gamma);
                gl.uniform1f(p.u('uSat'), g.sat);
                gl.drawArrays(gl.TRIANGLES, 0, 3);
                if (!shown) { shown = true; canvas.style.visibility = ''; }
                watch(root.performance.now() - t0);
            } catch (err) {
                fail('render-error');
            }
        }

        try {
            if (!doc || !host || !source) throw new Error('no-host');
            canvas = doc.createElement('canvas');
            canvas.className = 'rp-gpu-canvas';
            canvas.setAttribute('aria-hidden', 'true');
            canvas.style.visibility = 'hidden';
            canvas.width = Math.max(2, source.width | 0);
            canvas.height = Math.max(2, source.height | 0);
            gl = canvas.getContext('webgl2', { alpha: false, antialias: false, depth: false, stencil: false, premultipliedAlpha: false, preserveDrawingBuffer: false, powerPreference: 'high-performance' });
            if (!gl) throw new Error('no-webgl2');
            canvas.addEventListener('webglcontextlost', function (ev) {
                if (ev && ev.preventDefault) ev.preventDefault();
                fail('context-lost');
            });
            host.appendChild(canvas);
            progs = { bright: program(FRAG_BRIGHT), blur: program(FRAG_BLUR), rays: program(FRAG_RAYS), comp: program(FRAG_COMP) };
            gl.disable(gl.BLEND);
            gl.disable(gl.DEPTH_TEST);
            allocTargets(Math.max(2, source.width | 0), Math.max(2, source.height | 0));
        } catch (err) {
            const reason = err && err.message ? err.message : 'init-error';
            failed = true;
            teardown();
            try { onFallback(String(reason)); } catch (e2) { /* ignore */ }
            return Promise.resolve(null);
        }
        return Promise.resolve(api);
    }

    root.RunenpfadPostfx = { create: function (opts) {
        try { return create(opts); } catch (err) { return Promise.resolve(null); }
    } };
})(typeof window !== 'undefined' ? window : globalThis);
