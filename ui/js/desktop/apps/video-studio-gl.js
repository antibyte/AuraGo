(function () {
    'use strict';

    // AuraGo's own small WebGL2 helper for the Video Studio compositor (index §4.6); it does not use the screensaver GL
    // kit. Targets store images top-down (row 0 is the top row), like uploaded textures, so reading a target needs no
    // flip; only the canvas, which shows row 0 at the bottom, is flipped on present (u_flipY) and on read.
    const ATTRIBUTES = { alpha: false, antialias: false, depth: false, stencil: false, premultipliedAlpha: true, preserveDrawingBuffer: false, powerPreference: 'high-performance' };
    const QUAD = new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]);
    const POOL_LIMIT = 4, LOSS_HISTORY = 16;

    function sourceSize(source) {
        if (!source) return [0, 0];
        const width = Number(source.videoWidth || source.naturalWidth || source.displayWidth || source.width) || 0;
        const height = Number(source.videoHeight || source.naturalHeight || source.displayHeight || source.height) || 0;
        return [width, height];
    }

    function fitRect(rect, width, height) {
        const x = Math.max(0, Math.min(width, Math.floor(Number(rect && rect.x) || 0)));
        const y = Math.max(0, Math.min(height, Math.floor(Number(rect && rect.y) || 0)));
        const w = rect && rect.width != null ? Math.floor(Number(rect.width) || 0) : width - x;
        const h = rect && rect.height != null ? Math.floor(Number(rect.height) || 0) : height - y;
        return { x, y, width: Math.max(0, Math.min(width - x, w)), height: Math.max(0, Math.min(height - y, h)) };
    }

    // Returns the helper for canvas, or null when the browser gives no WebGL2 context. options: { onLost, onRestored }.
    function create(canvas, options) {
        const hooks = options || {};
        let gl = null;
        try { gl = canvas.getContext('webgl2', ATTRIBUTES); } catch (_) { gl = null; }
        if (!gl) return null;
        const programs = new Map(), failures = new Set(), textures = new Set(), luts = new Set(), pool = new Map(), losses = [];
        let lost = false, disposed = false, generation = 0, quad = null;

        function isLost() { return lost || gl.isContextLost(); }

        function setupQuad() {
            const vao = gl.createVertexArray(), buffer = gl.createBuffer();
            gl.bindVertexArray(vao);
            gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
            gl.bufferData(gl.ARRAY_BUFFER, QUAD, gl.STATIC_DRAW);
            gl.enableVertexAttribArray(0);
            gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
            gl.bindVertexArray(null);
            gl.bindBuffer(gl.ARRAY_BUFFER, null);
            quad = { vao, buffer };
        }

        function configure(handle) {
            gl.bindTexture(gl.TEXTURE_2D, handle);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
        }

        function compile(type, source) {
            const shader = gl.createShader(type);
            if (!shader) return null;
            gl.shaderSource(shader, String(source || ''));
            gl.compileShader(shader);
            if (gl.getShaderParameter(shader, gl.COMPILE_STATUS)) return shader;
            gl.deleteShader(shader);
            return null;
        }

        // Program: use() and set(name, value); the uniform's declared type picks the call (float, vec2-4, mat3, int,
        // bool and 2D/3D sampler units). Unknown or optimised-out names are ignored.
        function wrap(handle) {
            const uniforms = new Map();
            const count = gl.getProgramParameter(handle, gl.ACTIVE_UNIFORMS);
            for (let i = 0; i < count; i++) {
                const info = gl.getActiveUniform(handle, i);
                if (info) uniforms.set(info.name.replace(/\[0\]$/, ''), { location: gl.getUniformLocation(handle, info.name), type: info.type });
            }
            return {
                handle,
                use() { gl.useProgram(handle); },
                set(name, value) {
                    const uniform = uniforms.get(name);
                    if (!uniform || value == null) return;
                    const { location, type } = uniform;
                    if (type === gl.FLOAT) gl.uniform1f(location, Number(value));
                    else if (type === gl.FLOAT_VEC2) gl.uniform2fv(location, value);
                    else if (type === gl.FLOAT_VEC3) gl.uniform3fv(location, value);
                    else if (type === gl.FLOAT_VEC4) gl.uniform4fv(location, value);
                    else if (type === gl.FLOAT_MAT3) gl.uniformMatrix3fv(location, false, value);
                    else if (type === gl.INT || type === gl.BOOL || type === gl.SAMPLER_2D || type === gl.SAMPLER_3D) gl.uniform1i(location, Number(value) | 0);
                }
            };
        }

        // Compiles and links once per key. A failure returns null, is remembered (failed(key)) and never throws. While the
        // context is lost nothing compiles and nothing is marked failed.
        function program(key, vertexSrc, fragmentSrc) {
            if (disposed || isLost()) return null;
            if (programs.has(key)) return programs.get(key);
            if (failures.has(key)) return null;
            let vertex = null, fragment = null, built = null;
            try {
                vertex = compile(gl.VERTEX_SHADER, vertexSrc);
                fragment = compile(gl.FRAGMENT_SHADER, fragmentSrc);
                if (vertex && fragment) {
                    const handle = gl.createProgram();
                    gl.attachShader(handle, vertex);
                    gl.attachShader(handle, fragment);
                    gl.linkProgram(handle);
                    if (gl.getProgramParameter(handle, gl.LINK_STATUS)) built = wrap(handle);
                    else gl.deleteProgram(handle);
                }
            } catch (_) { built = null; }
            if (vertex) gl.deleteShader(vertex);
            if (fragment) gl.deleteShader(fragment);
            if (!built) { if (!isLost()) failures.add(key); return null; }
            programs.set(key, built);
            return built;
        }

        // Texture: upload(source) takes a video frame, image or canvas as straight-alpha RGBA8 without a Y flip (the
        // shaders premultiply; UV math handles orientation) and returns false when it cannot.
        function texture() {
            const tex = {
                width: 0, height: 0, handle: null, generation: -1,
                upload(source) {
                    const [width, height] = sourceSize(source);
                    if (disposed || isLost() || !width || !height) return false;
                    // An image still loading reports its size but cannot be uploaded: texImage2D sets INVALID_VALUE without
                    // throwing, which would leave a blank texture behind a true result.
                    if (typeof HTMLImageElement === 'function' && source instanceof HTMLImageElement && !source.complete) return false;
                    if (!tex.handle || tex.generation !== generation) { tex.handle = gl.createTexture(); tex.generation = generation; configure(tex.handle); }
                    try {
                        gl.bindTexture(gl.TEXTURE_2D, tex.handle);
                        gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, false);
                        gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false);
                        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, gl.RGBA, gl.UNSIGNED_BYTE, source);
                    } catch (_) { return false; }
                    tex.width = width;
                    tex.height = height;
                    return true;
                },
                bind(unit) { gl.activeTexture(gl.TEXTURE0 + unit); gl.bindTexture(gl.TEXTURE_2D, tex.generation === generation ? tex.handle : null); },
                dispose() {
                    if (tex.handle && tex.generation === generation && !isLost()) gl.deleteTexture(tex.handle);
                    tex.handle = null; tex.width = 0; tex.height = 0;
                    textures.delete(tex);
                }
            };
            textures.add(tex);
            return tex;
        }

        // LUT (index §4.6): a TEXTURE_3D RGBA16F of size³ texels (LINEAR, CLAMP_TO_EDGE) from data, a Float32Array RGBA with
        // red fastest. The object keeps data and builds the texture on its first bind, and again after a context restore,
        // so the caller's parsed table is all it needs. null when disposed or when size and data do not fit.
        function lut(size, data) {
            const n = Math.round(Number(size) || 0);
            if (disposed || n < 2 || !data || data.length !== n * n * n * 4) return null;
            const table = {
                size: n, data, handle: null, generation: -1,
                bind(unit) {
                    gl.activeTexture(gl.TEXTURE0 + unit);
                    if (disposed || isLost()) { gl.bindTexture(gl.TEXTURE_3D, null); return; }
                    if (!table.handle || table.generation !== generation) buildLUT(table);
                    gl.bindTexture(gl.TEXTURE_3D, table.handle);
                },
                dispose() {
                    if (table.handle && table.generation === generation && !isLost()) gl.deleteTexture(table.handle);
                    table.handle = null;
                    luts.delete(table);
                }
            };
            luts.add(table);
            return table;
        }
        function buildLUT(table) {
            const handle = gl.createTexture();
            gl.bindTexture(gl.TEXTURE_3D, handle);
            gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_WRAP_R, gl.CLAMP_TO_EDGE);
            // Typed-array 3D uploads refuse the DOM-source unpack flags (INVALID_OPERATION when they are set).
            gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, false);
            gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false);
            gl.texImage3D(gl.TEXTURE_3D, 0, gl.RGBA16F, table.size, table.size, table.size, 0, gl.RGBA, gl.FLOAT, table.data);
            table.handle = handle;
            table.generation = generation;
        }

        // Target: an RGBA8 framebuffer from a size-keyed pool; null when the framebuffer is incomplete or the context
        // is lost.
        function target(width, height) {
            if (disposed || isLost()) return null;
            const w = Math.max(1, Math.round(Number(width) || 1)), h = Math.max(1, Math.round(Number(height) || 1)), key = w + 'x' + h;
            const free = pool.get(key);
            if (free && free.length) return free.pop();
            const tex = gl.createTexture();
            configure(tex);
            gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
            const fbo = gl.createFramebuffer();
            gl.bindFramebuffer(gl.FRAMEBUFFER, fbo);
            gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0);
            const complete = gl.checkFramebufferStatus(gl.FRAMEBUFFER) === gl.FRAMEBUFFER_COMPLETE;
            gl.bindFramebuffer(gl.FRAMEBUFFER, null);
            if (!complete) { gl.deleteFramebuffer(fbo); gl.deleteTexture(tex); return null; }
            const made = {
                fbo, texture: tex, width: w, height: h, key, generation,
                bind(unit) { gl.activeTexture(gl.TEXTURE0 + unit); gl.bindTexture(gl.TEXTURE_2D, made.generation === generation ? tex : null); }
            };
            return made;
        }

        function release(t) {
            if (!t || disposed || t.generation !== generation) return;
            const free = pool.get(t.key) || [];
            if (free.includes(t)) return;
            if (free.length >= POOL_LIMIT) { gl.deleteFramebuffer(t.fbo); gl.deleteTexture(t.texture); return; }
            free.push(t);
            pool.set(t.key, free);
        }

        function bindTarget(t) {
            gl.bindFramebuffer(gl.FRAMEBUFFER, t ? t.fbo : null);
            gl.viewport(0, 0, t ? t.width : gl.drawingBufferWidth, t ? t.height : gl.drawingBufferHeight);
        }

        // Fills t (null: the canvas) with rgba (0..1, default transparent).
        function clear(t, rgba) {
            if (disposed || isLost() || (t && t.generation !== generation)) return;
            const colour = rgba || [0, 0, 0, 0];
            bindTarget(t);
            gl.clearColor(colour[0], colour[1], colour[2], colour[3]);
            gl.clear(gl.COLOR_BUFFER_BIT);
        }

        // Draws the full quad with prog into t (null: the canvas). uniforms go through Program.set; inputs map sampler
        // names to Textures or Targets on consecutive units. drawOptions.blend 'normal' composites premultiplied colour
        // (ONE, ONE_MINUS_SRC_ALPHA); otherwise the output replaces the destination.
        function draw(prog, t, uniforms, inputs, drawOptions) {
            if (disposed || isLost() || !prog || !quad || (t && t.generation !== generation)) return false;
            bindTarget(t);
            prog.use();
            Object.keys(uniforms || {}).forEach(name => prog.set(name, uniforms[name]));
            let unit = 0;
            Object.keys(inputs || {}).forEach(name => {
                const input = inputs[name];
                if (!input || typeof input.bind !== 'function') return;
                input.bind(unit);
                prog.set(name, unit);
                unit++;
            });
            if (drawOptions && drawOptions.blend === 'normal') { gl.enable(gl.BLEND); gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA); }
            else gl.disable(gl.BLEND);
            gl.bindVertexArray(quad.vao);
            gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
            gl.bindVertexArray(null);
            gl.disable(gl.BLEND);
            for (let i = 0; i < unit; i++) { gl.activeTexture(gl.TEXTURE0 + i); gl.bindTexture(gl.TEXTURE_2D, null); gl.bindTexture(gl.TEXTURE_3D, null); }
            return true;
        }

        // Top-down RGBA of rect ({x, y, width, height}, default whole) from t, or from the canvas when t is null.
        function read(t, rect) {
            const width = t ? t.width : gl.drawingBufferWidth, height = t ? t.height : gl.drawingBufferHeight;
            const r = fitRect(rect, width, height);
            const out = new Uint8ClampedArray(r.width * r.height * 4);
            if (disposed || isLost() || !r.width || !r.height || (t && t.generation !== generation)) return out;
            gl.bindFramebuffer(gl.FRAMEBUFFER, t ? t.fbo : null);
            if (t) {
                gl.readPixels(r.x, r.y, r.width, r.height, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array(out.buffer));
                return out;
            }
            // The canvas is bottom-up: read the mirrored rows and flip them.
            const raw = new Uint8Array(out.length), row = r.width * 4;
            gl.readPixels(r.x, height - r.y - r.height, r.width, r.height, gl.RGBA, gl.UNSIGNED_BYTE, raw);
            for (let y = 0; y < r.height; y++) out.set(raw.subarray((r.height - 1 - y) * row, (r.height - y) * row), y * row);
            return out;
        }

        function lossesWithin(ms) {
            const since = performance.now() - Number(ms || 0);
            return losses.filter(time => time >= since).length;
        }

        // A loss is always prevented (so the context can come back) and counted; every handle becomes invalid.
        function onLost(event) {
            event.preventDefault();
            losses.push(performance.now());
            if (losses.length > LOSS_HISTORY) losses.shift();
            lost = true;
            generation++;
            quad = null;
            programs.clear(); failures.clear(); pool.clear();
            textures.forEach(tex => { tex.handle = null; tex.width = 0; tex.height = 0; });
            luts.forEach(table => { table.handle = null; });
            if (typeof hooks.onLost === 'function') hooks.onLost();
        }
        function onRestored() {
            if (disposed) return;
            lost = false;
            generation++;
            setupQuad();
            if (typeof hooks.onRestored === 'function') hooks.onRestored();
        }

        function dispose() {
            if (disposed) return;
            canvas.removeEventListener('webglcontextlost', onLost);
            canvas.removeEventListener('webglcontextrestored', onRestored);
            if (!isLost()) {
                programs.forEach(item => gl.deleteProgram(item.handle));
                textures.forEach(tex => { if (tex.handle && tex.generation === generation) gl.deleteTexture(tex.handle); });
                luts.forEach(table => { if (table.handle && table.generation === generation) gl.deleteTexture(table.handle); });
                pool.forEach(list => list.forEach(item => { gl.deleteFramebuffer(item.fbo); gl.deleteTexture(item.texture); }));
                if (quad) { gl.deleteVertexArray(quad.vao); gl.deleteBuffer(quad.buffer); }
            }
            disposed = true;
            programs.clear(); textures.clear(); luts.clear(); pool.clear(); quad = null;
            // The element is discarded with this helper: give the GPU context back at once (browsers cap live contexts).
            const lose = isLost() ? null : gl.getExtension('WEBGL_lose_context');
            if (lose) lose.loseContext();
        }

        canvas.addEventListener('webglcontextlost', onLost);
        canvas.addEventListener('webglcontextrestored', onRestored);
        setupQuad();
        return { gl, program, failed: key => failures.has(key), texture, lut, target, release, clear, draw, read, lost: isLost, lossesWithin, dispose };
    }

    window.VideoStudioGL = { create };
})();
