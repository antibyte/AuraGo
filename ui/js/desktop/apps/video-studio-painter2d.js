(function () {
    'use strict';

    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
    const NO_CROP = Object.freeze({ l: 0, t: 0, r: 0, b: 0 });

    // The 2D painter (phase 2 index §4.5, §4.8): the preview's previous drawing behind the painter interface. It keeps
    // the phase 0/1 geometry (no even rounding) and draws transform, opacity, contain/cover and the four transitions
    // from a frame description built by video-studio-preview.js. Effects, blend modes, masks and key are never drawn
    // here (spec §6.6); a transition without a 2D implementation draws as a dissolve. A clip without a decoded frame is
    // skipped as before (hold-last-frame is a compositor feature).
    function create(canvas) {
        const context = canvas.getContext('2d', { alpha: false });
        let layer = null, layerContext = null;

        // The track layer a transition pair composes in, sized to the canvas and cleared.
        function trackLayer() {
            if (!layer) {
                layer = typeof OffscreenCanvas === 'function' ? new OffscreenCanvas(canvas.width, canvas.height) : document.createElement('canvas');
                layerContext = layer.getContext('2d');
            }
            if (layer.width !== canvas.width) layer.width = canvas.width;
            if (layer.height !== canvas.height) layer.height = canvas.height;
            layerContext.clearRect(0, 0, layer.width, layer.height);
            return layerContext;
        }

        // Draws one item into ctx; alphaScale carries the share of a transition the clip owns. Crop and flip follow
        // index §4.8 and §4.20:
        // - The crop rect is rounded in decoded pixels (VideoStudioPicture.decodedSize: the asset size, swapped for a
        //   rotated element). It maps to the element's pixels (a proxy is smaller) and becomes drawImage's source
        //   rectangle.
        // - item.state.crop is evaluated per frame, so an animated crop uses the same per-frame rect.
        // - Contain and cover fit that rectangle.
        // - Flips mirror about the box centre with a negative scale. Crop fractions refer to the unflipped source, so
        //   the source rectangle does not change.
        // - Effects are never drawn here.
        function drawItem(ctx, item, alphaScale) {
            if (!item || !item.ready || !item.el) return;
            const el = item.el, asset = item.asset || {}, state = item.state || {}, picture = window.VideoStudioPicture;
            const iw = Number(el.videoWidth || el.naturalWidth || asset.width || 0);
            const ih = Number(el.videoHeight || el.naturalHeight || asset.height || 0);
            if (!iw || !ih) return;
            const decoded = picture.decodedSize(asset, iw, ih);
            const rect = picture.cropRect(state.crop || NO_CROP, decoded.width, decoded.height);
            const sx = rect.x * iw / decoded.width, sy = rect.y * ih / decoded.height;
            const sw = rect.w * iw / decoded.width, sh = rect.h * ih / decoded.height;
            const cw = canvas.width, ch = canvas.height;
            const x = Number(state.x || 0) * cw, y = Number(state.y || 0) * ch;
            const w = Number(state.width || 1) * cw, h = Number(state.height || 1) * ch;
            const cover = state.fit === 'cover';
            const scale = cover ? Math.max(w / sw, h / sh) : Math.min(w / sw, h / sh);
            const dw = sw * scale, dh = sh * scale;
            ctx.save();
            ctx.globalAlpha = clamp(Number(state.opacity == null ? 1 : state.opacity), 0, 1) * (alphaScale == null ? 1 : alphaScale);
            ctx.translate(x + w / 2, y + h / 2);
            ctx.rotate(Number(state.rotation || 0) * Math.PI / 180);
            if (cover) { ctx.beginPath(); ctx.rect(-w / 2, -h / 2, w, h); ctx.clip(); }
            if (state.flipH || state.flipV) ctx.scale(state.flipH ? -1 : 1, state.flipV ? -1 : 1);
            ctx.drawImage(el, sx, sy, sw, sh, -dw / 2, -dh / 2, dw, dh);
            ctx.restore();
        }

        // Composes a transition pair in the track layer like the export's xfade on the premultiplied layers, then draws
        // the layer onto the canvas.
        function drawTransition(transition) {
            const ctx = trackLayer(), W = canvas.width, H = canvas.height, p = clamp(Number(transition.progress) || 0, 0, 1);
            const effects = window.VideoStudioEffects, wipe = effects.wipeEdge(transition.type, p, W);
            ctx.globalCompositeOperation = 'source-over';
            if (transition.type === 'black') {
                // out*w.out + in*w.inc + opaque black*w.black: 'lighter' adds premultiplied colour and alpha, so black adds
                // only coverage and darkens the tracks below exactly like fadeblack's opaque black.
                const weights = effects.fadeBlackWeights(p);
                if (weights.out > 0) drawItem(ctx, transition.out, weights.out);
                ctx.globalCompositeOperation = 'lighter';
                if (weights.inc > 0) drawItem(ctx, transition.inc, weights.inc);
                if (weights.black > 0) { ctx.save(); ctx.globalAlpha = weights.black; ctx.fillStyle = '#000'; ctx.fillRect(0, 0, W, H); ctx.restore(); }
            } else if (wipe) {
                drawItem(ctx, transition.out, 1);
                ctx.clearRect(wipe.left, 0, wipe.reveal, H);
                ctx.save(); ctx.beginPath(); ctx.rect(wipe.left, 0, wipe.reveal, H); ctx.clip();
                drawItem(ctx, transition.inc, 1);
                ctx.restore();
            } else {
                // Dissolve, and any transition without a 2D implementation: the outgoing layer fades out while the
                // incoming one adds in.
                drawItem(ctx, transition.out, 1 - p);
                ctx.globalCompositeOperation = 'lighter';
                drawItem(ctx, transition.inc, p);
            }
            ctx.globalCompositeOperation = 'source-over';
            context.globalAlpha = 1; context.globalCompositeOperation = 'source-over';
            context.drawImage(layer, 0, 0);
        }

        function frame(scene) {
            if (canvas.width !== scene.width) canvas.width = scene.width;
            if (canvas.height !== scene.height) canvas.height = scene.height;
            // The export composes onto black, so empty and letterboxed areas are black here too.
            context.globalAlpha = 1; context.globalCompositeOperation = 'source-over';
            context.fillStyle = '#000'; context.fillRect(0, 0, canvas.width, canvas.height);
            (scene.layers || []).forEach(track => {
                (track.items || []).forEach(item => drawItem(context, item));
                if (track.transition) drawTransition(track.transition);
            });
        }

        // Top-down RGBA of the last frame; rect {x, y, width, height} in backing pixels, default the whole canvas.
        function read(rect) {
            const r = rect || {};
            const x = clamp(Math.floor(Number(r.x) || 0), 0, canvas.width), y = clamp(Math.floor(Number(r.y) || 0), 0, canvas.height);
            const width = clamp(Math.floor(r.width == null ? canvas.width - x : Number(r.width) || 0), 0, canvas.width - x);
            const height = clamp(Math.floor(r.height == null ? canvas.height - y : Number(r.height) || 0), 0, canvas.height - y);
            if (!width || !height) return { width: 0, height: 0, data: new Uint8ClampedArray(0) };
            const image = context.getImageData(x, y, width, height);
            return { width: image.width, height: image.height, data: image.data };
        }

        return {
            kind: '2d', canvas, frame, read,
            release() {}, // the 2D painter keeps no per-clip resources
            pending() { return 0; }, // nothing loads: effects (and their LUTs) are never drawn in 2D
            dispose() { layer = null; layerContext = null; context.clearRect(0, 0, canvas.width, canvas.height); }
        };
    }

    window.VideoStudioPainter2D = { create };
})();
