(function () {
    'use strict';

    // WebGL2 renderer of the Green Slime effect (see green_slime_gl_shaders.js for the material). It draws
    // the scene contract of green_slime_sim.js: per window up to five strip quads (top, bottom with the drip
    // zone, left, right, strand box; never a full-window quad), the window occluders and the radius table as
    // uniforms/R32F texture, then free drops, spray and taskbar splats as one instanced draw behind a stencil
    // of all drop occluders. A renderer is { kind, render(scene), resize(cssWidth, cssHeight, dpr, scale),
    // setBackdrop(source), inspect(), dispose() }; create() returns null when WebGL2 is unavailable or the
    // programs do not build, the caller then continues with a fresh Canvas 2D canvas.

    const shaders = window.AuraGreenSlimeGLShaders;
    const kit = window.AuraGreenSlimeGeometry;
    const geometry = kit.geometry;
    const TABLE = kit.constants.TABLE_SAMPLES;

    const MAX_WINDOWS = 16;
    const MAX_DRIPS = 6;
    const MAX_BUBBLES = 3;
    const MAX_OCCLUDERS = 24;
    const MAX_UNDER = 16;
    const MAX_DROPS = 108;
    const MAX_MASKS = 1024;
    const MIN_UNIFORM_VECTORS = 120;
    const COMPILE_TIMEOUT_MS = 30000;
    const SOFTWARE_GPU = /swiftshader|llvmpipe|lavapipe|softpipe|software|basic render/i;
    const STAGE = { nucleate: 0, grow: 1, hold: 2, burst: 3, crater: 4 };
    const CONTEXT_ATTRIBUTES = { alpha: true, premultipliedAlpha: true, antialias: false, preserveDrawingBuffer: true,
        desynchronized: false, stencil: true, depth: false, powerPreference: 'low-power' };

    let lastError = '';

    function now() { return typeof performance !== 'undefined' ? performance.now() : Date.now(); }

    // A program build is two steps: begin() hands the sources to the driver and links (with
    // KHR_parallel_shader_compile this returns at once and the driver works on its own threads),
    // finish() reads the verdict once the driver is done.
    function begin(gl, vertexSource, fragmentSource, label, attributes) {
        const program = gl.createProgram();
        const shaderList = [];
        for (const [type, source, kind] of [[gl.VERTEX_SHADER, vertexSource, 'vertex'], [gl.FRAGMENT_SHADER, fragmentSource, 'fragment']]) {
            const shader = gl.createShader(type);
            gl.shaderSource(shader, source);
            gl.compileShader(shader);
            gl.attachShader(program, shader);
            shaderList.push([shader, kind]);
        }
        attributes.forEach((name, index) => gl.bindAttribLocation(program, index, name));
        gl.linkProgram(program);
        return { program, shaderList, label };
    }

    function finish(gl, job) {
        let failure = '';
        for (const [shader, kind] of job.shaderList) {
            if (!failure && !gl.getShaderParameter(shader, gl.COMPILE_STATUS)) failure = job.label + ' ' + kind + ' shader: ' + String(gl.getShaderInfoLog(shader) || '').slice(0, 600);
            gl.detachShader(job.program, shader);
            gl.deleteShader(shader);
        }
        if (!failure && !gl.getProgramParameter(job.program, gl.LINK_STATUS)) failure = job.label + ' program: ' + String(gl.getProgramInfoLog(job.program) || '').slice(0, 600);
        if (failure) throw new Error(failure);
        return job.program;
    }

    function locations(gl, program, names) {
        const map = {};
        for (const name of names) map[name] = gl.getUniformLocation(program, name);
        return map;
    }

    function create(canvas, options) {
        const opts = options || {};
        lastError = '';
        let gl = null;
        try { gl = canvas.getContext('webgl2', CONTEXT_ATTRIBUTES); } catch (_) { gl = null; }
        if (!gl) { lastError = 'webgl2 unavailable'; return null; }
        const created = [];
        const track = (kind, handle) => { created.push([kind, handle]); return handle; };
        const debugInfo = gl.getExtension('WEBGL_debug_renderer_info');
        const gpu = String(gl.getParameter(debugInfo ? debugInfo.UNMASKED_RENDERER_WEBGL : gl.RENDERER) || '');
        let lost = false;
        let disposed = false;
        let ready = false;
        let compileMs = 0;
        let blockedMs = 0;
        let pollTimer = 0;
        let firstRenderMs = 0;
        let frames = 0;
        let lastRenderMs = 0;
        let averageMs = 0;
        let drawCalls = 0;
        let quadCount = 0;
        let backdropSource = null;
        let backdropReady = false;
        let view = { sx: 1, sy: 1, bufferHeight: 1, width: 1, height: 1, scale: 1 };

        function fail(error) {
            lastError = String(error && error.message ? error.message : error);
            disposed = true;
            try { gl.getExtension('WEBGL_lose_context')?.loseContext(); } catch (_) { /* nothing to release */ }
            return null;
        }

        const onLost = event => {
            event.preventDefault();
            lost = true;
            if (!disposed && typeof opts.onContextLost === 'function') opts.onContextLost();
        };
        canvas.addEventListener('webglcontextlost', onLost, { signal: opts.signal });

        // Software WebGL (SwiftShader, llvmpipe, a VM without a GPU) draws the gel at about one frame per
        // second, far slower than the Canvas 2D fallback: leave it to the fallback. The test hook
        // window.__auraSlimeSoftwareGL lets a browser test run the real gel on software anyway.
        if (SOFTWARE_GPU.test(gpu) && !opts.allowSoftware && window.__auraSlimeSoftwareGL !== true) return fail(new Error('software WebGL (' + gpu + ')'));

        let gelProgram, dropProgram, maskProgram, gelU, dropU, maskU;
        const started = now();
        const parallel = gl.getExtension('KHR_parallel_shader_compile');
        const jobs = [];
        try {
            if (gl.getParameter(gl.MAX_FRAGMENT_UNIFORM_VECTORS) < MIN_UNIFORM_VECTORS) throw new Error('too few uniform vectors');
            jobs.push(begin(gl, shaders.gelVertex, shaders.gelFragment, 'gel', ['aPos']));
            jobs.push(begin(gl, shaders.dropVertex, shaders.dropFragment, 'drop', ['aA', 'aB']));
            jobs.push(begin(gl, shaders.maskVertex, shaders.maskFragment, 'mask', ['aRect', 'aRad']));
            for (const job of jobs) track('program', job.program);
        } catch (error) { return fail(error); }
        blockedMs = now() - started;

        // ---- Resources ----------------------------------------------------------------------------
        const quadBuffer = track('buffer', gl.createBuffer());
        const gelVao = track('vao', gl.createVertexArray());
        gl.bindVertexArray(gelVao);
        gl.bindBuffer(gl.ARRAY_BUFFER, quadBuffer);
        gl.enableVertexAttribArray(0);
        gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);

        const dropBuffer = track('buffer', gl.createBuffer());
        const dropData = new Float32Array(MAX_DROPS * 8);
        gl.bindBuffer(gl.ARRAY_BUFFER, dropBuffer);
        gl.bufferData(gl.ARRAY_BUFFER, dropData.byteLength, gl.DYNAMIC_DRAW);
        function dropVao(firstInstance) {
            const vao = track('vao', gl.createVertexArray());
            gl.bindVertexArray(vao);
            gl.bindBuffer(gl.ARRAY_BUFFER, dropBuffer);
            for (const [index, offset] of [[0, 0], [1, 16]]) {
                gl.enableVertexAttribArray(index);
                gl.vertexAttribPointer(index, 4, gl.FLOAT, false, 32, firstInstance * 32 + offset);
                gl.vertexAttribDivisor(index, 1);
            }
            return vao;
        }
        const dropsVao = dropVao(0);
        const splatsVao = dropVao(0);

        const maskBuffer = track('buffer', gl.createBuffer());
        const maskData = new Float32Array(MAX_MASKS * 5);
        const maskVao = track('vao', gl.createVertexArray());
        gl.bindVertexArray(maskVao);
        gl.bindBuffer(gl.ARRAY_BUFFER, maskBuffer);
        gl.bufferData(gl.ARRAY_BUFFER, maskData.byteLength, gl.DYNAMIC_DRAW);
        gl.enableVertexAttribArray(0);
        gl.vertexAttribPointer(0, 4, gl.FLOAT, false, 20, 0);
        gl.vertexAttribDivisor(0, 1);
        gl.enableVertexAttribArray(1);
        gl.vertexAttribPointer(1, 1, gl.FLOAT, false, 20, 16);
        gl.vertexAttribDivisor(1, 1);
        gl.bindVertexArray(null);

        // The radius table: one row of 128 samples per window. R32F is not filterable, so NEAREST and
        // the shader interpolates itself.
        const radiusTexture = track('texture', gl.createTexture());
        const radiusData = new Float32Array(TABLE * MAX_WINDOWS);
        gl.activeTexture(gl.TEXTURE1);
        gl.bindTexture(gl.TEXTURE_2D, radiusTexture);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.R32F, TABLE, MAX_WINDOWS, 0, gl.RED, gl.FLOAT, radiusData);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);

        const backdropTexture = track('texture', gl.createTexture());
        gl.activeTexture(gl.TEXTURE0);
        gl.bindTexture(gl.TEXTURE_2D, backdropTexture);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([40, 60, 20, 255]));
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);

        gl.disable(gl.DEPTH_TEST);
        gl.disable(gl.CULL_FACE);
        gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);

        const quadData = new Float32Array(MAX_WINDOWS * 5 * 12);
        const uDrip = new Float32Array(MAX_DRIPS * 8);
        const uBub = new Float32Array(MAX_BUBBLES * 16);
        const uStrand = new Float32Array(12);
        const uOcc = new Float32Array(MAX_OCCLUDERS * 4);
        const uOccR = new Float32Array(MAX_OCCLUDERS);
        const uUnder = new Float32Array(MAX_UNDER * 4);

        function setBackdrop(source) {
            if (lost || disposed) return;
            backdropSource = source || null;
            if (!source) { backdropReady = false; return; }
            try {
                gl.activeTexture(gl.TEXTURE0);
                gl.bindTexture(gl.TEXTURE_2D, backdropTexture);
                gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, false);
                gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false);
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, gl.RGBA, gl.UNSIGNED_BYTE, source);
                gl.generateMipmap(gl.TEXTURE_2D);
                gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
                backdropReady = true;
            } catch (error) {
                backdropReady = false;
                lastError = 'backdrop upload: ' + (error && error.message ? error.message : error);
            }
        }

        function resize(cssWidth, cssHeight, dpr, scale) {
            const factor = (dpr || 1) * (scale || 1);
            canvas.width = Math.max(1, Math.round(cssWidth * factor));
            canvas.height = Math.max(1, Math.round(cssHeight * factor));
            view = { sx: cssWidth / canvas.width, sy: cssHeight / canvas.height, bufferHeight: canvas.height, width: cssWidth, height: cssHeight, scale: scale || 1 };
            if (!lost) gl.viewport(0, 0, canvas.width, canvas.height);
        }

        // ---- Per-window packing ---------------------------------------------------------------------
        function pushQuad(offset, x0, y0, x1, y1) {
            x0 = Math.max(0, x0); y0 = Math.max(0, y0);
            x1 = Math.min(view.width, x1); y1 = Math.min(view.height, y1);
            if (x1 - x0 < .5 || y1 - y0 < .5) return offset;
            quadData.set([x0, y0, x1, y0, x0, y1, x1, y0, x1, y1, x0, y1], offset);
            return offset + 12;
        }

        function windowBox(win, rmax) {
            const rect = win.rect;
            let x0 = rect.x - rmax - 12, x1 = rect.x + rect.width + rmax + 24;
            let y0 = rect.y - rmax - 8, y1 = rect.y + rect.height + rmax + 28;
            for (const drip of win.drips.slice(0, MAX_DRIPS)) {
                const reach = Math.max(drip.root, drip.bulb) + 12;
                x0 = Math.min(x0, drip.x + Math.min(0, drip.sway) - reach);
                x1 = Math.max(x1, drip.x + Math.max(0, drip.sway) + reach + 24);
                y1 = Math.max(y1, drip.y + drip.length + drip.bulb + 40);
            }
            return { x0, y0, x1, y1 };
        }

        function packWindow(win, row, strand) {
            const lay = geometry.layout(win.rect, win.radii);
            const segs = lay.segs;
            gl.uniform4f(gelU.uRect, lay.x, lay.y, lay.w, lay.h);
            gl.uniform4f(gelU.uRadii, lay.radii[0], lay.radii[1], lay.radii[2], lay.radii[3]);
            gl.uniform4f(gelU.uLenA, segs[0].len, segs[1].len, segs[2].len, segs[3].len);
            gl.uniform4f(gelU.uLenB, segs[4].len, segs[5].len, segs[6].len, segs[7].len);
            const seed = (kit.hashString(String(win.id)) % 997) / 997;
            gl.uniform4f(gelU.uWin, row, seed, Math.max(lay.length, 1), strand ? 1 : 0);
            const drips = win.drips.slice(0, MAX_DRIPS);
            uDrip.fill(0);
            drips.forEach((drip, i) => uDrip.set([drip.x, drip.y, drip.length, drip.sway, drip.root, drip.neck, drip.bulb, 0], i * 8));
            gl.uniform4fv(gelU.uDrip, uDrip);
            const bubbles = win.bubbles.slice(0, MAX_BUBBLES);
            uBub.fill(0);
            bubbles.forEach((b, i) => uBub.set([b.x, b.y, b.base, b.r, b.dome, STAGE[b.stage] || 0, b.t, b.film,
                b.hx, b.hy, b.holeR, b.rim, b.crater, b.rebound, b.interference, 0], i * 16));
            gl.uniform4fv(gelU.uBub, uBub);
            uStrand.fill(0);
            if (strand) {
                const margin = Math.max(strand.root, strand.mid, strand.tip * 1.15) + 24;
                const droop = Math.min(16, Math.hypot(strand.bx - strand.ax, strand.by - strand.ay) * .09);
                uStrand.set([strand.ax, strand.ay, strand.bx, strand.by, strand.root, strand.mid, strand.tip, strand.beads,
                    Math.min(strand.ax, strand.bx) - margin, Math.min(strand.ay, strand.by) - margin,
                    Math.max(strand.ax, strand.bx) + margin + 20, Math.max(strand.ay, strand.by) + droop + margin + 26]);
            }
            gl.uniform4fv(gelU.uStrand, uStrand);
            const occluders = win.occluders.slice(0, MAX_OCCLUDERS);
            uOcc.fill(0);
            uOccR.fill(0);
            occluders.forEach((o, i) => { uOcc.set([o.x, o.y, o.x + o.width, o.y + o.height], i * 4); uOccR[i] = o.radius || 0; });
            gl.uniform4fv(gelU.uOcc, uOcc);
            gl.uniform4fv(gelU.uOccR, uOccR);
            const under = win.under.slice(0, MAX_UNDER);
            uUnder.fill(0);
            under.forEach((o, i) => uUnder.set([o.x, o.y, o.x + o.width, o.y + o.height], i * 4));
            gl.uniform4fv(gelU.uUnder, uUnder);
            gl.uniform4f(gelU.uCounts, drips.length, bubbles.length, occluders.length, under.length);
            return lay;
        }

        function renderWindows(scene, windows) {
            gl.bindVertexArray(gelVao);
            gl.useProgram(gelProgram);
            gl.uniform4f(gelU.uView, view.sx, view.sy, view.bufferHeight, 0);
            gl.uniform2f(gelU.uSize, view.width, view.height);
            const textured = scene.backdrop && scene.backdrop.mode === 'texture' && backdropReady;
            gl.uniform4f(gelU.uFrame, scene.time || 0, scene.motion ? 1 : 0, textured ? 1 : 0, 0);
            const rect = scene.backdrop && scene.backdrop.rect ? scene.backdrop.rect : { x: 0, y: 0, w: view.width, h: view.height };
            gl.uniform4f(gelU.uBack, rect.x, rect.y, Math.max(rect.w, 1), Math.max(rect.h, 1));
            // Quads first (one upload), then one or two draws per window.
            const plan = [];
            let offset = 0;
            windows.forEach((win, row) => {
                radiusData.set(win.radius.length === TABLE ? win.radius : win.radius.subarray(0, TABLE), row * TABLE);
                let rmax = 0;
                for (let k = 0; k < TABLE; k++) if (win.radius[k] > rmax) rmax = win.radius[k];
                const box = windowBox(win, rmax);
                const rect2 = win.rect;
                const rc = Math.max(0, Math.min(Math.max(...win.radii), rect2.height / 2, rect2.width / 2));
                const first = offset;
                offset = pushQuad(offset, box.x0, box.y0, box.x1, rect2.y + rc);
                offset = pushQuad(offset, box.x0, rect2.y + rect2.height - rc, box.x1, box.y1);
                offset = pushQuad(offset, box.x0, rect2.y + rc, rect2.x + rc, rect2.y + rect2.height - rc);
                offset = pushQuad(offset, rect2.x + rect2.width - rc, rect2.y + rc, box.x1, rect2.y + rect2.height - rc);
                const strand = win.strands && win.strands.length ? win.strands[0] : null;
                const strandFirst = offset;
                if (strand) {
                    const margin = Math.max(strand.root, strand.mid, strand.tip * 1.15) + 24;
                    const droop = Math.min(16, Math.hypot(strand.bx - strand.ax, strand.by - strand.ay) * .09);
                    offset = pushQuad(offset, Math.min(strand.ax, strand.bx) - margin, Math.min(strand.ay, strand.by) - margin,
                        Math.max(strand.ax, strand.bx) + margin + 20, Math.max(strand.ay, strand.by) + droop + margin + 26);
                }
                plan.push({ win, row, strand, box, first: first / 2, count: (strandFirst - first) / 2, strandFirst: strandFirst / 2, strandCount: (offset - strandFirst) / 2 });
            });
            gl.activeTexture(gl.TEXTURE1);
            gl.bindTexture(gl.TEXTURE_2D, radiusTexture);
            gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, TABLE, Math.max(1, windows.length), gl.RED, gl.FLOAT, radiusData);
            gl.activeTexture(gl.TEXTURE0);
            gl.bindTexture(gl.TEXTURE_2D, backdropTexture);
            gl.bindBuffer(gl.ARRAY_BUFFER, quadBuffer);
            gl.bufferData(gl.ARRAY_BUFFER, quadData.subarray(0, Math.max(offset, 12)), gl.DYNAMIC_DRAW);
            gl.enable(gl.BLEND);
            for (const item of plan) {
                packWindow(item.win, item.row, item.strand);
                gl.uniform4f(gelU.uBand, item.box.x0, item.box.y0, item.box.x1, item.box.y1);
                if (item.count > 0) {
                    gl.uniform1f(gelU.uPass, 0);
                    gl.drawArrays(gl.TRIANGLES, item.first, item.count);
                    drawCalls++;
                }
                if (item.strandCount > 0) {
                    gl.uniform1f(gelU.uPass, 1);
                    gl.drawArrays(gl.TRIANGLES, item.strandFirst, item.strandCount);
                    drawCalls++;
                }
                quadCount += (item.count + item.strandCount) / 6;
            }
        }

        function renderDrops(scene) {
            const list = [];
            for (const drop of scene.drops || []) list.push(drop);
            const drops = list.slice(0, MAX_DROPS);
            const splats = (scene.splats || []).slice(0, Math.max(0, MAX_DROPS - drops.length));
            if (!drops.length && !splats.length) return;
            let index = 0;
            for (const drop of drops) {
                dropData.set([drop.x, drop.y, drop.r, drop.spray ? 1 : 0, drop.vx, drop.vy, drop.wobble, drop.alpha], index * 8);
                index++;
            }
            const splatFirst = index;
            for (const splat of splats) {
                dropData.set([splat.x, splat.y, splat.r, 2, 0, 0, splat.spread, splat.alpha], index * 8);
                index++;
            }
            const occluders = scene.dropOccluders || [];
            let masks = 0;
            const writeMasks = taskbar => {
                let written = 0;
                for (const item of occluders) {
                    if (!!item.taskbar !== taskbar || masks >= MAX_MASKS) continue;
                    maskData.set([item.x, item.y, item.x + item.width, item.y + item.height, item.radius || 0], masks * 5);
                    masks++;
                    written++;
                }
                return written;
            };
            const plain = writeMasks(false);
            const bars = writeMasks(true);
            gl.disable(gl.BLEND);
            if (masks) {
                gl.useProgram(maskProgram);
                gl.uniform2f(maskU.uSize, view.width, view.height);
                gl.bindVertexArray(maskVao);
                gl.bindBuffer(gl.ARRAY_BUFFER, maskBuffer);
                gl.bufferSubData(gl.ARRAY_BUFFER, 0, maskData.subarray(0, masks * 5));
                gl.enable(gl.STENCIL_TEST);
                gl.colorMask(false, false, false, false);
                gl.stencilOp(gl.KEEP, gl.KEEP, gl.REPLACE);
                if (plain) {
                    gl.stencilMask(3);
                    gl.stencilFunc(gl.ALWAYS, 3, 255);
                    gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, plain);
                    drawCalls++;
                }
                if (bars) {
                    gl.stencilMask(1);
                    gl.stencilFunc(gl.ALWAYS, 1, 255);
                    gl.bindBuffer(gl.ARRAY_BUFFER, maskBuffer);
                    gl.vertexAttribPointer(0, 4, gl.FLOAT, false, 20, plain * 20);
                    gl.vertexAttribPointer(1, 1, gl.FLOAT, false, 20, plain * 20 + 16);
                    gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, bars);
                    gl.vertexAttribPointer(0, 4, gl.FLOAT, false, 20, 0);
                    gl.vertexAttribPointer(1, 1, gl.FLOAT, false, 20, 16);
                    drawCalls++;
                }
                gl.colorMask(true, true, true, true);
                gl.stencilOp(gl.KEEP, gl.KEEP, gl.KEEP);
                gl.stencilMask(255);
            }
            gl.useProgram(dropProgram);
            gl.uniform4f(dropU.uView, view.sx, view.sy, view.bufferHeight, 0);
            gl.uniform2f(dropU.uSize, view.width, view.height);
            const textured = scene.backdrop && scene.backdrop.mode === 'texture' && backdropReady;
            gl.uniform4f(dropU.uFrame, scene.time || 0, scene.motion ? 1 : 0, textured ? 1 : 0, 0);
            const rect = scene.backdrop && scene.backdrop.rect ? scene.backdrop.rect : { x: 0, y: 0, w: view.width, h: view.height };
            gl.uniform4f(dropU.uBack, rect.x, rect.y, Math.max(rect.w, 1), Math.max(rect.h, 1));
            gl.activeTexture(gl.TEXTURE0);
            gl.bindTexture(gl.TEXTURE_2D, backdropTexture);
            gl.bindBuffer(gl.ARRAY_BUFFER, dropBuffer);
            gl.bufferSubData(gl.ARRAY_BUFFER, 0, dropData.subarray(0, index * 8));
            gl.enable(gl.BLEND);
            if (drops.length) {
                gl.bindVertexArray(dropsVao);
                if (masks) { gl.stencilFunc(gl.EQUAL, 0, 1); }
                gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, drops.length);
                drawCalls++;
            }
            if (splats.length) {
                gl.bindVertexArray(splatsVao);
                gl.bindBuffer(gl.ARRAY_BUFFER, dropBuffer);
                for (const [slot, offset] of [[0, 0], [1, 16]]) gl.vertexAttribPointer(slot, 4, gl.FLOAT, false, 32, splatFirst * 32 + offset);
                if (masks) { gl.stencilFunc(gl.EQUAL, 0, 2); }
                gl.drawArraysInstanced(gl.TRIANGLE_STRIP, 0, 4, splats.length);
                drawCalls++;
            }
            gl.disable(gl.STENCIL_TEST);
        }

        function render(scene) {
            if (lost || disposed || !ready) return;
            const begun = now();
            drawCalls = 0;
            quadCount = 0;
            if (scene.backdrop && scene.backdrop.image !== backdropSource) setBackdrop(scene.backdrop.image);
            gl.viewport(0, 0, canvas.width, canvas.height);
            gl.disable(gl.STENCIL_TEST);
            gl.stencilMask(255);
            gl.clearColor(0, 0, 0, 0);
            gl.clear(gl.COLOR_BUFFER_BIT | gl.STENCIL_BUFFER_BIT);
            const windows = scene.windows.length > MAX_WINDOWS
                ? scene.windows.slice().sort((a, b) => b.z - a.z).slice(0, MAX_WINDOWS).sort((a, b) => a.z - b.z)
                : scene.windows;
            if (windows.length) renderWindows(scene, windows);
            renderDrops(scene);
            gl.bindVertexArray(null);
            lastRenderMs = now() - begun;
            frames++;
            averageMs += (lastRenderMs - averageMs) * Math.max(.05, 1 / frames);
            if (!firstRenderMs) firstRenderMs = lastRenderMs;
        }

        function inspect() {
            return {
                kind: 'webgl2', gpu, ready, compileMs: Math.round(compileMs * 10) / 10, blockedMs: Math.round(blockedMs * 10) / 10,
                parallelCompile: !!parallel, firstRenderMs: Math.round(firstRenderMs * 10) / 10,
                lastRenderMs: Math.round(lastRenderMs * 100) / 100, averageRenderMs: Math.round(averageMs * 100) / 100,
                drawCalls, quadCount, contextLost: lost, backdropReady, bufferWidth: canvas.width, bufferHeight: canvas.height,
                maxTextureSize: gl && !lost ? gl.getParameter(gl.MAX_TEXTURE_SIZE) : 0,
                uniformVectors: gl && !lost ? gl.getParameter(gl.MAX_FRAGMENT_UNIFORM_VECTORS) : 0
            };
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            if (pollTimer) clearTimeout(pollTimer);
            pollTimer = 0;
            canvas.removeEventListener('webglcontextlost', onLost);
            if (!lost) {
                try {
                    gl.bindVertexArray(null);
                    for (const [kind, handle] of created) {
                        if (kind === 'program') gl.deleteProgram(handle);
                        else if (kind === 'buffer') gl.deleteBuffer(handle);
                        else if (kind === 'texture') gl.deleteTexture(handle);
                        else if (kind === 'vao') gl.deleteVertexArray(handle);
                    }
                    gl.getExtension('WEBGL_lose_context')?.loseContext();
                } catch (_) { /* the context may already be gone */ }
            }
            canvas.remove();
        }

        // ---- Build completion -----------------------------------------------------------------------
        function failAsync(error) {
            lastError = String(error && error.message ? error.message : error);
            if (!disposed && typeof opts.onFailure === 'function') opts.onFailure(lastError);
        }

        function complete() {
            const mark = now();
            try {
                [gelProgram, dropProgram, maskProgram] = jobs.map(job => finish(gl, job));
                gelU = locations(gl, gelProgram, ['uView', 'uSize', 'uFrame', 'uBack', 'uBackdrop', 'uRadius', 'uRect', 'uRadii', 'uLenA', 'uLenB',
                    'uWin', 'uCounts', 'uDrip', 'uBub', 'uStrand', 'uOcc', 'uOccR', 'uUnder', 'uBand', 'uPass']);
                dropU = locations(gl, dropProgram, ['uView', 'uSize', 'uFrame', 'uBack', 'uBackdrop']);
                maskU = locations(gl, maskProgram, ['uSize']);
                gl.useProgram(gelProgram);
                gl.uniform1i(gelU.uBackdrop, 0);
                gl.uniform1i(gelU.uRadius, 1);
                gl.useProgram(dropProgram);
                gl.uniform1i(dropU.uBackdrop, 0);
            } catch (error) { failAsync(error); return; }
            ready = true;
            blockedMs += now() - mark;
            compileMs = now() - started;
            if (typeof opts.onReady === 'function') opts.onReady();
        }

        function poll() {
            pollTimer = 0;
            if (disposed || lost) return;
            const mark = now();
            const done = jobs.every(job => gl.getProgramParameter(job.program, parallel.COMPLETION_STATUS_KHR));
            blockedMs += now() - mark;
            if (done) { complete(); return; }
            if (now() - started > COMPILE_TIMEOUT_MS) { failAsync(new Error('shader build timed out')); return; }
            pollTimer = setTimeout(poll, 8);
        }

        // Without the extension the link verdict is a blocking call, so there is nothing to wait for.
        if (parallel) pollTimer = setTimeout(poll, 0);
        else complete();
        if (!parallel && !ready) { dispose(); return null; }

        return { kind: 'webgl2', render, resize, setBackdrop, inspect, dispose,
            get ready() { return ready; },
            get maxTextureSize() { return lost ? 0 : gl.getParameter(gl.MAX_TEXTURE_SIZE); } };
    }

    window.AuraGreenSlimeGL = Object.freeze({ create, get lastError() { return lastError; } });
})();
