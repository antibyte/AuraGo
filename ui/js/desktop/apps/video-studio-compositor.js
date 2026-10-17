(function () {
    'use strict';

    // The WebGL2 preview painter (index §4.8). frame(scene) runs six named stages; later slices extend a stage, never
    // the loop. Layers composite in paint order into a canvas-size target that starts opaque black; present() copies it
    // to the canvas and read() reads that target, so the canvas needs no preserveDrawingBuffer.
    const GL = () => window.VideoStudioGL;
    const S = () => window.VideoStudioShaders;
    const E = () => window.VideoStudioEffects;
    const Pic = () => window.VideoStudioPicture;
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
    const isVideo = el => typeof HTMLVideoElement === 'function' && el instanceof HTMLVideoElement;
    const isImage = el => typeof HTMLImageElement === 'function' && el instanceof HTMLImageElement;
    const api = { supported, create, lastError: '' };
    // Three context losses within 60 s give up on GL (spec §6.6).
    const LOSS_WINDOW_MS = 60000, MAX_LOSSES = 3;
    const NO_CROP = Object.freeze({ l: 0, t: 0, r: 0, b: 0 });

    function supported() { return typeof WebGL2RenderingContext === 'function'; }

    // The unit quad centred on (cx, cy), w × h canvas pixels, rotated by angle (radians, clockwise on screen like canvas
    // rotate and FFmpeg rotate), in clip space of a top-down width × height target. Column-major mat3.
    function placeMatrix(cx, cy, w, h, angle, width, height) {
        const cos = Math.cos(angle), sin = Math.sin(angle), sx = 2 / width, sy = 2 / height;
        return new Float32Array([
            sx * cos * w, sy * sin * w, 0,
            -sx * sin * h, sy * cos * h, 0,
            sx * (cx - 0.5 * cos * w + 0.5 * sin * h) - 1, sy * (cy - 0.5 * sin * w - 0.5 * cos * h) - 1, 1
        ]);
    }

    function fitRect(rect, width, height) {
        const x = clamp(Math.floor(Number(rect && rect.x) || 0), 0, width), y = clamp(Math.floor(Number(rect && rect.y) || 0), 0, height);
        const w = rect && rect.width != null ? Math.floor(Number(rect.width) || 0) : width - x;
        const h = rect && rect.height != null ? Math.floor(Number(rect.height) || 0) : height - y;
        return { x, y, width: clamp(w, 0, width - x), height: clamp(h, 0, height - y) };
    }

    // A rect {x, y, w, h} in pixels of size {width, height} as texture coordinates [x, y, width, height].
    function uvRect(rect, size) { return [rect.x / size.width, rect.y / size.height, rect.w / size.width, rect.h / size.height]; }

    // A parsed table {size, data} in the form ctx.lut takes. VideoStudioPicture.parseCube's red-fastest RGB triples gain
    // an alpha of 1, and RGBA data is used as it is. null for a malformed table.
    function lutData(table) {
        const n = Math.round(Number(table && table.size) || 0), data = table && table.data, count = n * n * n;
        if (n < 2 || !data || typeof data.length !== 'number') return null;
        if (data.length === count * 4) return data instanceof Float32Array ? data : Float32Array.from(data);
        if (data.length !== count * 3) return null;
        const out = new Float32Array(count * 4);
        for (let i = 0; i < count; i++) {
            out[i * 4] = data[i * 3]; out[i * 4 + 1] = data[i * 3 + 1]; out[i * 4 + 2] = data[i * 3 + 2]; out[i * 4 + 3] = 1;
        }
        return out;
    }

    // Returns the GL painter, or null with lastError 'unsupported' (no WebGL2 context) or 'init_failed' (a base program
    // does not compile or a render target is incomplete). options:
    // { onLost, onRestored, onFallback(reason), onProgramFailed(kind, id),
    //   loadLUT(ref) -> Promise<{size, domainMin, domainMax, data}>, onResource() }.
    // The preview resolves LUT refs; onResource repaints after a load finished.
    function create(canvas, options) {
        const hooks = options || {};
        api.lastError = '';
        if (!supported() || !GL() || !S() || !E() || !Pic()) { api.lastError = 'unsupported'; return null; }
        const ctx = GL().create(canvas, { onLost: () => handleLost(), onRestored: () => handleRestored() });
        if (!ctx) { api.lastError = 'unsupported'; return null; }
        // clip id -> { texture, frame (a frame was uploaded), el, time (currentTime at upload), dirty, armed,
        //   out (the clip-local Target clipPasses returned last, kept until the clip's next passes or release) }
        const clips = new Map();
        const reported = new Set(); // 'kind:id' of program failures already reported
        // LUT ref -> { state: 'loading' | 'ready' | 'failed', table: {size, domainMin, domainMax}, texture (ctx.lut) }. The
        // parsed data stays with the texture object, which rebuilds itself after a context restore.
        const luts = new Map();
        let composite = null, current = null, disposed = false, loading = 0;
        if (!baseReady()) { ctx.dispose(); api.lastError = 'init_failed'; return null; }

        function placeProgram() { return ctx.program('place', S().PLACE_VERTEX, S().PLACE); }
        function copyProgram() { return ctx.program('copy', S().QUAD_VERTEX, S().COPY); }
        function pictureProgram() { return ctx.program('picture', S().QUAD_VERTEX, S().PICTURE); }
        function baseReady() {
            const probe = ctx.target(2, 2);
            if (!probe) return false;
            ctx.release(probe);
            return !!(placeProgram() && copyProgram() && pictureProgram());
        }

        // Stage 1: the clip's texture. A ready element that is not seeking is uploaded (a video only when it shows a new
        // frame: after its requestVideoFrameCallback, or after a paused seek); otherwise the clip keeps its last texture
        // (hold last frame). A clip that never had a frame has no texture and is not drawn.
        function sourceTexture(item) {
            const id = item && item.clip && item.clip.id;
            if (!id) return null;
            let entry = clips.get(id);
            if (!entry) { entry = { texture: ctx.texture(), frame: false, el: null, time: -1, dirty: true, armed: false, out: null }; clips.set(id, entry); }
            const el = item.el;
            if (el !== entry.el) { entry.el = el; entry.dirty = true; entry.armed = false; }
            if (el && item.ready && !el.seeking && wantsUpload(entry, el) && entry.texture.upload(el)) {
                entry.frame = true; entry.dirty = false; entry.time = Number(el.currentTime) || 0;
            }
            if (el) arm(entry, el);
            return entry.frame && entry.texture.width ? entry.texture : null;
        }
        function wantsUpload(entry, el) {
            if (!entry.frame || entry.dirty) return true;
            if (isImage(el)) return false;
            if (!isVideo(el) || typeof el.requestVideoFrameCallback !== 'function') return true;
            return el.paused && Math.abs((Number(el.currentTime) || 0) - entry.time) > 1e-4;
        }
        function arm(entry, el) {
            if (entry.armed || !isVideo(el) || typeof el.requestVideoFrameCallback !== 'function') return;
            entry.armed = true;
            el.requestVideoFrameCallback(() => { if (entry.el === el) { entry.armed = false; entry.dirty = true; } });
        }

        function canvasSize() { return composite ? { width: composite.width, height: composite.height } : { width: canvas.width, height: canvas.height }; }

        // The loaded LUT of a ref; null while it loads, after a failed load, and for '' (no LUT chosen). The first request
        // starts hooks.loadLUT(ref). pending() counts loads in flight, and hooks.onResource() repaints after each one. A
        // failed load passes its input through for the life of this painter.
        function lutFor(ref) {
            const key = String(ref || '');
            if (!key) return null;
            let entry = luts.get(key);
            if (!entry) {
                entry = { state: 'loading', table: null, texture: null };
                luts.set(key, entry);
                if (typeof hooks.loadLUT !== 'function') { entry.state = 'failed'; return null; }
                loading++;
                let request;
                try { request = Promise.resolve(hooks.loadLUT(key)); } catch (error) { request = Promise.reject(error); }
                request.then(parsed => {
                    // The preview samples the table the export stages, resampled onto the unit domain
                    // (VideoStudioPicture.unitDomain, Go unitDomainLUT; FFmpeg's lut3d ignores DOMAIN_MIN). So the pass's
                    // domain uniforms are always 0 and 1; they stay for 2d/2f.
                    const table = parsed ? Pic().unitDomain(parsed) : null;
                    const data = lutData(table);
                    const texture = data && !disposed ? ctx.lut(table.size, data) : null;
                    if (!texture) { entry.state = 'failed'; return; }
                    entry.table = { size: texture.size, domainMin: [0, 0, 0], domainMax: [1, 1, 1] };
                    entry.texture = texture;
                    entry.state = 'ready';
                }).catch(() => { entry.state = 'failed'; }).then(() => {
                    loading = Math.max(0, loading - 1);
                    if (!disposed && typeof hooks.onResource === 'function') hooks.onResource();
                });
            }
            return entry.state === 'ready' ? entry : null;
        }

        // The drawable passes of a clip's effect stack, in stack order: [{program, uniforms, textures}].
        // - Ids without a shader (later slices) and identity entries add nothing: passUniforms returns [].
        // - A lut entry draws once its table is loaded.
        // - A program that does not compile is skipped, so its input passes through, and is reported once as
        //   ('effect', id).
        function effectPasses(effects, size) {
            const passes = [];
            (effects || []).forEach(effect => {
                const sources = effect && S().effect(effect.id);
                if (!sources || !Array.isArray(sources.passes) || !sources.passes.length) return;
                const programs = sources.passes.map((source, index) => ctx.program('effect:' + effect.id + ':' + index, S().QUAD_VERTEX, source));
                if (programs.some(program => !program)) { reportFailure('effect', effect.id); return; }
                let values = effect.values || {}, textures = {};
                if (effect.id === 'lut') {
                    const loaded = lutFor(values.lut);
                    if (!loaded) return;
                    values = Object.assign({}, values, { table: loaded.table });
                    textures = { u_lut: loaded.texture };
                }
                const uniforms = Pic().passUniforms(effect.id, values, size);
                if (!Array.isArray(uniforms) || uniforms.length !== programs.length) return;
                uniforms.forEach((set, index) => passes.push({ program: programs[index], uniforms: set, textures }));
            });
            return passes;
        }

        // Stage 2 (2c, index §4.8): crop, flip and the effect stack from item.state. It sets item.picture =
        // {width, height, uv}:
        // - width x height is the frame's crop rect in decoded pixels (VideoStudioPicture.decodedSize), the size fit uses;
        // - uv is the part of the returned Texture or Target that placeClip samples.
        // There are three cases:
        // - Without effects or flips nothing is drawn. uv is the crop rect, so the picture is resampled once.
        // - Static crop: PICTURE crops and flips into a target of capSize(cropped texture px, canvas). The effect passes
        //   ping-pong in that size, and uv is the whole target.
        // - Crop keyframes: PICTURE flips the whole picture and the effects run on all of it. uv is the frame's crop rect
        //   mirrored for the flips (mirroredCrop), so the crop follows the effects as in the export.
        // The returned target stays the clip's (entry.out) until its next passes or release(clipId).
        function clipPasses(item, texture) {
            const state = item.state || {}, entry = clips.get(item.clip.id);
            const decoded = Pic().decodedSize(item.asset, texture.width, texture.height);
            const crop = state.crop || NO_CROP, flipH = !!state.flipH, flipV = !!state.flipV;
            const rect = Pic().cropRect(crop, decoded.width, decoded.height);
            const cropUV = uvRect(rect, decoded);
            item.picture = { width: rect.w, height: rect.h, uv: cropUV };
            if (entry && entry.out) { ctx.release(entry.out); entry.out = null; }
            const animated = !!E().cropAnimated(item.clip);
            const source = animated ? [0, 0, 1, 1] : cropUV;
            const size = Pic().capSize(texture.width * source[2], texture.height * source[3], canvasSize());
            const passes = effectPasses(state.effects, size);
            if (!passes.length && !flipH && !flipV) return texture;
            let input = ctx.target(size.width, size.height), spare = null;
            if (!input) return texture;
            ctx.draw(pictureProgram(), input, { u_flipY: 0, u_uvRect: source, u_flip: [flipH ? 1 : 0, flipV ? 1 : 0] }, { u_source: texture });
            for (const pass of passes) {
                if (!spare) spare = ctx.target(size.width, size.height);
                if (!spare) break;
                ctx.draw(pass.program, spare, Object.assign({ u_flipY: 0, u_size: [size.width, size.height] }, pass.uniforms), Object.assign({ u_source: input }, pass.textures));
                const done = spare;
                spare = input;
                input = done;
            }
            ctx.release(spare);
            if (entry) entry.out = input;
            item.picture.uv = animated ? uvRect(Pic().cropRect(Pic().mirroredCrop(crop, flipH, flipV), decoded.width, decoded.height), decoded) : [0, 0, 1, 1];
            return input;
        }

        // Stage 3: transform and opacity from item.state.
        // - The box uses the export's even rounding (VideoStudioEffects.placement) and fits item.picture, the crop rect in
        //   decoded pixels set by clipPasses.
        // - Contain draws the fitted picture centred in the box. Cover draws the box and samples the matching part. Both
        //   sample inside item.picture.uv of the texture.
        // - The quad rotates about the box centre.
        function placeClip(item, texture, target) {
            const state = item.state || {};
            const opacity = clamp(Number(state.opacity == null ? 1 : state.opacity), 0, 1);
            const project = (current && current.project) || {};
            const picture = item.picture || { width: texture.width, height: texture.height, uv: [0, 0, 1, 1] };
            const place = E().placement(state, { width: target.width, height: target.height }, { width: Number(project.width) || target.width, height: Number(project.height) || target.height }, { width: picture.width, height: picture.height });
            if (opacity <= 0 || !place || !place.box || !place.draw) return;
            const box = place.box, draw = place.draw, cover = !!place.clip;
            const w = cover ? box.w : draw.w, h = cover ? box.h : draw.h;
            if (!(w > 0 && h > 0 && draw.w > 0 && draw.h > 0)) return;
            const local = cover ? [clamp((draw.w - box.w) / 2 / draw.w, 0, 1), clamp((draw.h - box.h) / 2 / draw.h, 0, 1), clamp(box.w / draw.w, 0, 1), clamp(box.h / draw.h, 0, 1)] : [0, 0, 1, 1];
            const base = picture.uv;
            const uv = [base[0] + local[0] * base[2], base[1] + local[1] * base[3], local[2] * base[2], local[3] * base[3]];
            const angle = (Number(state.rotation) || 0) * Math.PI / 180;
            const matrix = placeMatrix(box.x + box.w / 2, box.y + box.h / 2, w, h, angle, target.width, target.height);
            ctx.draw(placeProgram(), target, { u_matrix: matrix, u_uvRect: uv, u_opacity: opacity }, { u_source: texture }, { blend: 'normal' });
        }

        // Stage 4: compositing into the destination. 2a: premultiplied normal blending (ONE, ONE_MINUS_SRC_ALPHA, the
        // export's straight-alpha overlay) inside placeClip. 2d renders other blend modes through VideoStudioShaders.blend.
        function compositeClip(item, texture, target) { placeClip(item, texture, target); }

        // Stage 5: a transition pair. Both clips run stages 1–3 into transparent canvas-size targets, the transition
        // shader mixes them (xfade on premultiplied layers) and the result composites with normal blending. An id without
        // a shader draws as a dissolve; a shader that does not compile draws a hard cut at p >= 0.5.
        function transitionProgram(type) {
            const id = S().transition(type) ? type : 'dissolve';
            return ctx.program('transition:' + id, S().QUAD_VERTEX, S().transition(id));
        }
        function transitionUniforms(type, p, width) {
            if (type === 'black') { const w = E().fadeBlackWeights(p); return { u_weights: [w.out, w.inc, w.black] }; }
            if (type === 'wipeleft' || type === 'wiperight') { const edge = E().wipeEdge(type, p, width); return { u_edge: [edge.left, edge.reveal] }; }
            return {};
        }
        function drawTransition(transition, target) {
            const width = target.width, height = target.height;
            const from = ctx.target(width, height), to = ctx.target(width, height);
            if (!from || !to) { ctx.release(from); ctx.release(to); return; }
            try {
                [[transition.out, from], [transition.inc, to]].forEach(([item, layer]) => {
                    ctx.clear(layer, [0, 0, 0, 0]);
                    const texture = item ? sourceTexture(item) : null;
                    if (texture) placeClip(item, clipPasses(item, texture), layer);
                });
                const p = clamp(Number(transition.progress) || 0, 0, 1);
                const program = transitionProgram(transition.type);
                if (program) ctx.draw(program, target, Object.assign({ u_progress: p, u_size: [width, height] }, transitionUniforms(transition.type, p, width)), { u_from: from, u_to: to }, { blend: 'normal' });
                else {
                    // The shader did not compile: a hard cut at p >= 0.5, and the stage shows a notice.
                    reportFailure('transition', transition.type);
                    ctx.draw(copyProgram(), target, { u_flipY: 0 }, { u_source: p >= 0.5 ? to : from }, { blend: 'normal' });
                }
            } finally {
                ctx.release(from);
                ctx.release(to);
            }
        }

        // Stage 6: copy the composite to the canvas, flipped (the canvas shows row 0 at the bottom).
        function present() { ctx.draw(copyProgram(), null, { u_flipY: 1 }, { u_source: composite }); }

        function frame(scene) {
            if (disposed || ctx.lost() || !scene) return;
            const width = Math.max(1, Math.round(Number(scene.width) || canvas.width)), height = Math.max(1, Math.round(Number(scene.height) || canvas.height));
            // Painters own their canvas size (index §4.5): the backing store follows the scene; the preview never sets it.
            if (canvas.width !== width) canvas.width = width;
            if (canvas.height !== height) canvas.height = height;
            if (composite && (composite.width !== width || composite.height !== height)) { ctx.release(composite); composite = null; }
            if (!composite) composite = ctx.target(width, height);
            if (!composite) return;
            current = scene;
            try {
                ctx.clear(composite, [0, 0, 0, 1]);
                (scene.layers || []).forEach(layer => {
                    (layer.items || []).forEach(item => {
                        const texture = sourceTexture(item);
                        if (texture) compositeClip(item, clipPasses(item, texture), composite);
                    });
                    if (layer.transition) drawTransition(layer.transition, composite);
                });
                present();
            } finally {
                current = null;
            }
        }

        function read(rect) {
            const width = composite ? composite.width : canvas.width, height = composite ? composite.height : canvas.height;
            const r = fitRect(rect, width, height);
            const data = composite && !disposed && !ctx.lost() ? ctx.read(composite, r) : new Uint8ClampedArray(r.width * r.height * 4);
            return { width: r.width, height: r.height, data };
        }

        // LUT loads in flight (index §4.5): the preview's settle() waits until this is 0.
        function pending() { return disposed ? 0 : loading; }

        function release(clipId) {
            const entry = clips.get(clipId);
            if (!entry) return;
            entry.texture.dispose();
            if (entry.out) ctx.release(entry.out);
            clips.delete(clipId);
        }

        // A lost context drops every GPU resource (LUT objects keep their data and rebuild on the next bind). The third
        // loss within 60 s gives up on GL (index §4.5). Otherwise the restore rebuilds the programs, and a rebuild that
        // fails gives up too.
        function handleLost() {
            clips.forEach(entry => entry.texture.dispose());
            clips.clear();
            composite = null;
            if (ctx.lossesWithin(LOSS_WINDOW_MS) >= MAX_LOSSES) { giveUp('context_lost'); return; }
            if (typeof hooks.onLost === 'function') hooks.onLost();
        }
        function handleRestored() {
            if (disposed) return;
            if (!baseReady()) { giveUp('init_failed'); return; }
            if (typeof hooks.onRestored === 'function') hooks.onRestored();
        }
        // The preview switches to the 2D painter in onFallback and disposes this painter.
        function giveUp(reason) { if (!disposed && typeof hooks.onFallback === 'function') hooks.onFallback(reason); }
        // A program that failed to compile is reported once per kind and id ('transition' since 2a, 'effect' since 2c).
        function reportFailure(kind, id) {
            const key = kind + ':' + id;
            if (reported.has(key)) return;
            reported.add(key);
            if (typeof hooks.onProgramFailed === 'function') hooks.onProgramFailed(kind, id);
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            clips.forEach(entry => { entry.texture.dispose(); if (entry.out) ctx.release(entry.out); });
            clips.clear();
            luts.forEach(entry => { if (entry.texture) entry.texture.dispose(); });
            luts.clear();
            if (composite) ctx.release(composite);
            composite = null;
            ctx.dispose();
        }

        return { kind: 'gl', canvas, frame, read, release, pending, dispose };
    }

    window.VideoStudioCompositor = api;
})();
