(function () {
    'use strict';

    // The WebGL2 preview painter (index §4.8). frame(scene) runs six named stages; later slices extend a stage, never
    // the loop. Layers composite in paint order into a canvas-size target that starts opaque black; present() copies it
    // to the canvas and read() reads that target, so the canvas needs no preserveDrawingBuffer.
    const GL = () => window.VideoStudioGL;
    const S = () => window.VideoStudioShaders;
    const E = () => window.VideoStudioEffects;
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
    const isVideo = el => typeof HTMLVideoElement === 'function' && el instanceof HTMLVideoElement;
    const isImage = el => typeof HTMLImageElement === 'function' && el instanceof HTMLImageElement;
    const api = { supported, create, lastError: '' };
    // Three context losses within 60 s give up on GL (spec §6.6).
    const LOSS_WINDOW_MS = 60000, MAX_LOSSES = 3;

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

    // Returns the GL painter, or null with lastError 'unsupported' (no WebGL2 context) or 'init_failed' (a base program
    // does not compile or a render target is incomplete). options: { onLost, onRestored, onFallback(reason), onProgramFailed(kind, id) }.
    function create(canvas, options) {
        const hooks = options || {};
        api.lastError = '';
        if (!supported() || !GL() || !S() || !E()) { api.lastError = 'unsupported'; return null; }
        const ctx = GL().create(canvas, { onLost: () => handleLost(), onRestored: () => handleRestored() });
        if (!ctx) { api.lastError = 'unsupported'; return null; }
        // clip id -> { texture, frame (a frame was uploaded), el, time (currentTime at upload), dirty, armed }
        const clips = new Map();
        const reported = new Set(); // 'kind:id' of program failures already reported
        let composite = null, current = null, disposed = false;
        if (!baseReady()) { ctx.dispose(); api.lastError = 'init_failed'; return null; }

        function placeProgram() { return ctx.program('place', S().PLACE_VERTEX, S().PLACE); }
        function copyProgram() { return ctx.program('copy', S().QUAD_VERTEX, S().COPY); }
        function baseReady() {
            const probe = ctx.target(2, 2);
            if (!probe) return false;
            ctx.release(probe);
            return !!(placeProgram() && copyProgram());
        }

        // Stage 1: the clip's texture. A ready element that is not seeking is uploaded (a video only when it shows a new
        // frame: after its requestVideoFrameCallback, or after a paused seek); otherwise the clip keeps its last texture
        // (hold last frame). A clip that never had a frame has no texture and is not drawn.
        function sourceTexture(item) {
            const id = item && item.clip && item.clip.id;
            if (!id) return null;
            let entry = clips.get(id);
            if (!entry) { entry = { texture: ctx.texture(), frame: false, el: null, time: -1, dirty: true, armed: false }; clips.set(id, entry); }
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

        // Stage 2: clip-local passes (2c crop, flip and effects; 2d key and mask; 2e blur_in). The identity in 2a.
        function clipPasses(item, texture) { return texture; }

        // Stage 3: transform and opacity from item.state. The box uses the export's even rounding
        // (VideoStudioEffects.placement); contain draws the fitted picture centred in the box, cover draws the box and
        // samples the matching part of the source. The quad rotates about the box centre.
        function placeClip(item, texture, target) {
            const state = item.state || {};
            const opacity = clamp(Number(state.opacity == null ? 1 : state.opacity), 0, 1);
            const project = (current && current.project) || {};
            const place = E().placement(state, { width: target.width, height: target.height }, { width: Number(project.width) || target.width, height: Number(project.height) || target.height }, { width: texture.width, height: texture.height });
            if (opacity <= 0 || !place || !place.box || !place.draw) return;
            const box = place.box, draw = place.draw, cover = !!place.clip;
            const w = cover ? box.w : draw.w, h = cover ? box.h : draw.h;
            if (!(w > 0 && h > 0 && draw.w > 0 && draw.h > 0)) return;
            const uv = cover ? [clamp((draw.w - box.w) / 2 / draw.w, 0, 1), clamp((draw.h - box.h) / 2 / draw.h, 0, 1), clamp(box.w / draw.w, 0, 1), clamp(box.h / draw.h, 0, 1)] : [0, 0, 1, 1];
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

        function release(clipId) {
            const entry = clips.get(clipId);
            if (!entry) return;
            entry.texture.dispose();
            clips.delete(clipId);
        }

        // A lost context drops every GPU resource. The third loss within 60 s gives up on GL (index §4.5); otherwise the
        // restore rebuilds the programs, and a rebuild that fails gives up too.
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
        // A program that failed to compile is reported once per kind and id (2a: transitions; 2c adds effects).
        function reportFailure(kind, id) {
            const key = kind + ':' + id;
            if (reported.has(key)) return;
            reported.add(key);
            if (typeof hooks.onProgramFailed === 'function') hooks.onProgramFailed(kind, id);
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            clips.forEach(entry => entry.texture.dispose());
            clips.clear();
            if (composite) ctx.release(composite);
            composite = null;
            ctx.dispose();
        }

        return { kind: 'gl', canvas, frame, read, release, dispose };
    }

    window.VideoStudioCompositor = api;
})();
