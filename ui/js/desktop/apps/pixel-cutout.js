// Pixel module "cutout" (WP8a): background removal as a non-destructive layer mask. Engines: the local AI models (on demand
// download, /api/pixel/cutout), the classic on-device engine (GrabCut, pixel-lib-cutout.js) and a colour key. The result
// lives on the active layer's mask together with the raw matte, so edge shift, feather, smoothing and the keep/remove
// brush can be changed afterwards (cutout.refine runs on the stored matte).
(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const PROXY_EDGE = 512;       // the classic engine works on a picture this size
    const UPLOAD_EDGE = 2048;     // the AI models work at 1024 x 1024, more pixels only cost upload time
    const PREVIEW_EDGE = 1600;    // live refine previews of larger documents run on a reduced copy
    const REMEMBER_LIMIT = 24;    // refine contexts kept per layer (one per history step is plenty)
    const POLL_MS = 1000;
    const PREVIEW_DELAY_MS = 40;
    const COMMIT_IDLE_MS = 450;
    const AI_MODELS = ['standard', 'precise'];
    const BRUSH_SVG = '<svg width="16" height="16" viewBox="0 0 16 16"><circle cx="8" cy="8" r="5.5" fill="none" stroke="currentColor" stroke-width="1.3" stroke-dasharray="2.4 1.8"/><path d="M5 8.3l2 2 4-4.6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>';
    // Generators offered as one-click backgrounds, when the generator package is present.
    const BACKGROUND_GENERATORS = ['clouds', 'gradient', 'plasma', 'nebula', 'perlin', 'fbm', 'marble', 'paper', 'bokeh'];

    // --- parameter schemas (the shared param kit builds and wires the controls) ---

    const ENGINE_SCHEMA = [
        { key: 'engine', type: 'select', labelKey: 'pixel.cutout_engine', fallback: 'Engine', default: 'auto', options: [
            { value: 'auto', labelKey: 'pixel.cutout_engine_auto', fallback: 'Automatic' },
            { value: 'standard', labelKey: 'pixel.cutout_engine_standard', fallback: 'AI standard' },
            { value: 'precise', labelKey: 'pixel.cutout_engine_precise', fallback: 'AI precise' },
            { value: 'classic', labelKey: 'pixel.cutout_engine_classic', fallback: 'Classic (on device)' },
            { value: 'key', labelKey: 'pixel.cutout_engine_key', fallback: 'Color key' }
        ] }
    ];
    const KEY_SCHEMA = [
        { key: 'keyAuto', type: 'bool', labelKey: 'pixel.cutout_key_auto', fallback: 'Detect from the edges', default: true },
        { key: 'keyColor', type: 'color', labelKey: 'pixel.cutout_key_color', fallback: 'Key color', default: '#00c820', showIf: v => !v.keyAuto },
        { key: 'tolerance', type: 'range', labelKey: 'pixel.cutout_key_tolerance', fallback: 'Tolerance', min: 0, max: 100, step: 1, default: 40 },
        { key: 'softness', type: 'range', labelKey: 'pixel.cutout_key_softness', fallback: 'Softness', min: 0, max: 100, step: 1, default: 20 }
    ];
    const BLUR_SCHEMA = [
        { key: 'blurStrength', type: 'range', labelKey: 'pixel.cutout_blur_strength', fallback: 'Blur strength', min: 1, max: 100, step: 1, default: 40 }
    ];
    const REFINE_SCHEMA = [
        { key: 'shift', type: 'range', labelKey: 'pixel.cutout_shift', fallback: 'Edge shift', min: -10, max: 10, step: 1, unit: 'px', default: 0 },
        { key: 'feather', type: 'range', labelKey: 'pixel.cutout_feather', fallback: 'Feather', min: 0, max: 20, step: 1, unit: 'px', default: 0 },
        { key: 'smooth', type: 'range', labelKey: 'pixel.cutout_smooth', fallback: 'Smooth', min: 0, max: 10, step: 1, default: 0 },
        { key: 'decontaminate', type: 'bool', labelKey: 'pixel.cutout_decontaminate', fallback: 'Remove color fringe', default: true },
        { key: 'decontaminateRadius', type: 'range', labelKey: 'pixel.cutout_decontaminate_radius', fallback: 'Fringe radius', min: 1, max: 20, step: 1, unit: 'px', default: 6, showIf: v => !!v.decontaminate },
        { key: 'view', type: 'select', labelKey: 'pixel.cutout_view', fallback: 'View', default: 'normal', options: [
            { value: 'normal', labelKey: 'pixel.cutout_view_normal', fallback: 'Normal' },
            { value: 'checker', labelKey: 'pixel.cutout_view_checker', fallback: 'Checkerboard' },
            { value: 'black', labelKey: 'pixel.cutout_view_black', fallback: 'On black' },
            { value: 'white', labelKey: 'pixel.cutout_view_white', fallback: 'On white' },
            { value: 'red', labelKey: 'pixel.cutout_view_red', fallback: 'Red overlay' }
        ] }
    ];
    const REFINE_MASK_KEYS = ['shift', 'feather', 'smooth'];
    const REFINE_KEYS = REFINE_MASK_KEYS.concat(['decontaminate', 'decontaminateRadius']);
    const BRUSH_DEFAULTS = { mode: 'keep', size: 40, hardness: 60, smart: false };

    // The generator package registers Pixel.GENERATORS (array or map) and the gen.render op; both are optional.
    function backgroundGenerators() {
        const lib = window.AuraPixelLib;
        if (!Pixel.GENERATORS || !lib || !lib.ops || typeof lib.ops['gen.render'] !== 'function') return [];
        const all = Array.isArray(Pixel.GENERATORS) ? Pixel.GENERATORS : Object.keys(Pixel.GENERATORS).map(id => Object.assign({ id }, Pixel.GENERATORS[id]));
        const chosen = BACKGROUND_GENERATORS.map(id => all.find(def => def && def.id === id)).filter(Boolean);
        return (chosen.length ? chosen : all.filter(def => def && def.id).slice(0, 6));
    }

    function replaceSchema() {
        const generators = backgroundGenerators();
        const modes = [
            { value: 'solid', labelKey: 'pixel.cutout_replace_solid', fallback: 'Solid color' },
            { value: 'transparent', labelKey: 'pixel.cutout_replace_transparent', fallback: 'Transparent' }
        ];
        if (generators.length) modes.push({ value: 'generated', labelKey: 'pixel.cutout_replace_generated', fallback: 'Generated' });
        const schema = [
            { key: 'replaceMode', type: 'select', labelKey: 'pixel.cutout_replace_mode', fallback: 'Replace with', default: 'solid', options: modes },
            { key: 'replaceColor', type: 'color', labelKey: 'pixel.cutout_replace_color', fallback: 'Color', default: '#ffffff', showIf: v => v.replaceMode === 'solid' }
        ];
        if (generators.length) {
            schema.push({ key: 'replaceGenerator', type: 'select', labelKey: 'pixel.cutout_replace_generator', fallback: 'Background', default: generators[0].id, showIf: v => v.replaceMode === 'generated',
                options: generators.map(def => ({ value: def.id, labelKey: def.labelKey, fallback: def.fallback || def.name || def.id })) });
        }
        return schema;
    }

    // --- small helpers ---

    function tx(r, key, fallback, vars) {
        let text = r.t(key, fallback);
        if (typeof text !== 'string' || !text || text === key) text = fallback;
        if (vars) Object.keys(vars).forEach(name => { text = text.split('{' + name + '}').join(String(vars[name])); });
        return text;
    }

    const isAbort = err => !!err && err.name === 'AbortError';
    const gone = r => !!(r.state && r.state.disposed);

    function docToken(r) {
        return { revision: r.state.documentRevision || 0, layers: r.layers, w: r.imgWidth, h: r.imgHeight };
    }

    function tokenStale(r, token) {
        return gone(r) || token.revision !== (r.state.documentRevision || 0) || token.layers !== r.layers || token.w !== r.imgWidth || token.h !== r.imgHeight;
    }

    function scaledCanvas(r, source, edge, backdrop) {
        const long = Math.max(source.width, source.height);
        const scale = Math.min(1, edge / long);
        if (scale >= 1 && !backdrop) return source;
        const w = Math.max(1, Math.round(source.width * scale)), h = Math.max(1, Math.round(source.height * scale));
        const out = r.makeCanvas(w, h);
        const ctx = out.getContext('2d');
        if (backdrop) { ctx.fillStyle = backdrop; ctx.fillRect(0, 0, w, h); }
        ctx.imageSmoothingEnabled = true;
        ctx.imageSmoothingQuality = 'high';
        ctx.drawImage(source, 0, 0, w, h);
        return out;
    }

    function alphaOfCanvas(r, canvas) {
        const img = r.imageFromCanvas(canvas);
        const out = new Uint8ClampedArray(img.width * img.height);
        for (let i = 0, q = 3; i < out.length; i++, q += 4) out[i] = img.data[q];
        return out;
    }

    // White canvas whose alpha channel is the matte: the layer mask format (opaque = visible).
    function paintMask(r, canvas, alpha, width, height) {
        const ctx = canvas.getContext('2d');
        const image = ctx.createImageData(width, height);
        const d = image.data;
        for (let i = 0, q = 0; i < alpha.length; i++, q += 4) { d[q] = 255; d[q + 1] = 255; d[q + 2] = 255; d[q + 3] = alpha[i]; }
        ctx.putImageData(image, 0, 0);
    }

    function writeMask(r, layer, alpha, width, height) {
        const w = r.imgWidth, h = r.imgHeight;
        if (!layer.maskCanvas || layer.maskCanvas.width !== w || layer.maskCanvas.height !== h) layer.maskCanvas = r.makeCanvas(w, h);
        layer.maskEnabled = true;
        if (width === w && height === h) {
            paintMask(r, layer.maskCanvas, alpha, w, h);
            return;
        }
        // a preview computed on a reduced copy: scale it up into the mask
        const small = r.makeCanvas(width, height);
        paintMask(r, small, alpha, width, height);
        const ctx = layer.maskCanvas.getContext('2d');
        ctx.save();
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalCompositeOperation = 'copy';
        ctx.imageSmoothingEnabled = true;
        ctx.imageSmoothingQuality = 'high';
        ctx.drawImage(small, 0, 0, w, h);
        ctx.restore();
    }

    // --- brush strokes on the mask (also replayed after every recomputation) ---

    function stamp(ctx, x, y, radius, hardness) {
        ctx.beginPath();
        ctx.arc(x, y, radius, 0, Math.PI * 2);
        if (hardness >= 99) {
            ctx.fillStyle = '#ffffff';
        } else {
            const gradient = ctx.createRadialGradient(x, y, radius * hardness / 100, x, y, radius);
            gradient.addColorStop(0, 'rgba(255,255,255,1)');
            gradient.addColorStop(1, 'rgba(255,255,255,0)');
            ctx.fillStyle = gradient;
        }
        ctx.fill();
    }

    function paintSegment(ctx, x0, y0, x1, y1, size, hardness, mode) {
        const radius = Math.max(0.5, size / 2);
        const spacing = Math.max(1, size * 0.12);
        const length = Math.hypot(x1 - x0, y1 - y0);
        const steps = Math.max(1, Math.ceil(length / spacing));
        ctx.save();
        ctx.globalAlpha = 1;
        ctx.globalCompositeOperation = mode === 'remove' ? 'destination-out' : 'source-over';
        // a segment starts where the previous one ended, so only a lone point stamps its own start
        const first = length === 0 ? 0 : 1, lastStep = length === 0 ? 0 : steps;
        for (let i = first; i <= lastStep; i++) {
            const t = i / steps;
            stamp(ctx, x0 + (x1 - x0) * t, y0 + (y1 - y0) * t, radius, hardness);
        }
        ctx.restore();
    }

    function replayStrokes(ctx, strokes) {
        (strokes || []).forEach(stroke => {
            const pts = stroke.pts;
            if (!pts.length) return;
            paintSegment(ctx, pts[0][0], pts[0][1], pts[0][0], pts[0][1], stroke.size, stroke.hardness, stroke.mode);
            for (let i = 1; i < pts.length; i++) paintSegment(ctx, pts[i - 1][0], pts[i - 1][1], pts[i][0], pts[i][1], stroke.size, stroke.hardness, stroke.mode);
        });
    }

    // --- per-window state ---

    function createState(r) {
        const refineValues = r.defaultParams(REFINE_SCHEMA);
        return {
            engineValues: r.defaultParams(ENGINE_SCHEMA),
            keyValues: r.defaultParams(KEY_SCHEMA),
            blurValues: r.defaultParams(BLUR_SCHEMA),
            replaceSchema: replaceSchema(),
            replaceValues: null,
            refineValues,
            brushValues: Object.assign({}, BRUSH_DEFAULTS),
            status: null,
            statusError: '',
            statusLoading: false,
            statusLoaded: false,
            statusAt: 0,
            downloading: {},
            pollTimer: 0,
            busy: null,
            seq: 0,
            remembered: new Map(),
            session: null,
            inputSeq: 0,
            previewSeq: 0,
            previewTimer: 0,
            commitTimer: 0,
            committing: false,
            guide: null,
            viewCache: null,
            viewOverlay: false,
            stroke: null,
            hoverAlt: false,
            refreshFrame: 0,
            wires: {},
            ui: null
        };
    }

    const stateOf = r => r.cutoutState;

    // Refine contexts: layer id -> (mask revision -> context). The revision of a history snapshot comes back on undo,
    // so the sliders keep working on states the user steps through.
    function lookupContext(r, layer) {
        const st = stateOf(r);
        if (!layer || !layer.maskCanvas) return null;
        const byRev = st.remembered.get(layer.id);
        return (byRev && byRev.get(layer.maskRev)) || null;
    }

    function rememberContext(r, layer, cx) {
        const st = stateOf(r);
        let byRev = st.remembered.get(layer.id);
        if (!byRev) st.remembered.set(layer.id, byRev = new Map());
        byRev.delete(layer.maskRev);
        byRev.set(layer.maskRev, cx);
        while (byRev.size > REMEMBER_LIMIT) byRev.delete(byRev.keys().next().value);
    }

    // --- server status (AI models) ---

    function modelInfo(st, id) {
        const list = st.status && Array.isArray(st.status.models) ? st.status.models : [];
        return list.find(model => model.id === id) || null;
    }

    function aiAvailable(r) {
        const st = stateOf(r);
        return !!(st.status && st.status.available && !r.ctx.readonly);
    }

    function readyModel(r, preferred) {
        const st = stateOf(r);
        if (!aiAvailable(r)) return null;
        const order = preferred ? [preferred] : AI_MODELS;
        const found = order.find(id => { const info = modelInfo(st, id); return info && info.ready; });
        return found || null;
    }

    async function loadStatus(r) {
        const st = stateOf(r);
        if (st.statusLoading || gone(r)) return st.status;
        st.statusLoading = true;
        try {
            const next = await r.api('/api/pixel/cutout/status');
            if (gone(r)) return null;
            const was = st.downloading;
            st.downloading = {};
            const models = next && Array.isArray(next.models) ? next.models : [];
            models.forEach(model => {
                if (model.downloading) st.downloading[model.id] = true;
                if (was[model.id] && model.ready && !model.downloading) {
                    r.notify({ type: 'success', message: tx(r, 'pixel.cutout_model_ready_note', 'AI model ready: {name}.', { name: model.name || model.id }) });
                }
            });
            st.status = next;
            st.statusError = '';
        } catch (err) {
            if (!gone(r)) st.statusError = (err && err.message) || 'error';
        } finally {
            st.statusLoading = false;
            st.statusLoaded = true;
            st.statusAt = Date.now();
        }
        if (gone(r)) return null;
        schedulePoll(r);
        refreshPanel(r);
        return st.status;
    }

    function schedulePoll(r) {
        const st = stateOf(r);
        if (st.pollTimer) { clearTimeout(st.pollTimer); st.pollTimer = 0; }
        if (gone(r) || !Object.keys(st.downloading).length || r.activePanels.inspector !== 'cutout') return;
        st.pollTimer = setTimeout(() => { st.pollTimer = 0; loadStatus(r); }, POLL_MS);
    }

    // The only place that asks the server to download anything: a click on a model's download button.
    async function downloadModel(r, id) {
        const st = stateOf(r);
        if (r.ctx.readonly || !AI_MODELS.includes(id)) return;
        st.statusError = '';
        try {
            await r.api('/api/pixel/cutout/prepare', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ model: id }) });
        } catch (err) {
            if (!gone(r)) st.statusError = (err && err.message) || 'error';
        }
        await loadStatus(r);
    }

    // --- busy state and progress ---

    function beginBusy(r, label) {
        const st = stateOf(r);
        if (st.busy) {
            r.notify({ type: 'info', message: tx(r, 'pixel.cutout_busy', 'A cutout is already running.') });
            return null;
        }
        if (st.stroke) return null;
        const ticket = { id: ++st.seq, label, ac: new AbortController(), progress: null, token: docToken(r) };
        st.busy = ticket;
        refreshPanel(r);
        return ticket;
    }

    function endBusy(r, ticket) {
        const st = stateOf(r);
        if (ticket && st.busy === ticket) {
            st.busy = null;
            if (!gone(r)) refreshPanel(r);
        }
    }

    function setProgress(r, ticket, fraction) {
        const st = stateOf(r);
        if (!ticket || st.busy !== ticket) return;
        ticket.progress = fraction;
        paintProgress(st);
    }

    function paintProgress(st) {
        const ui = st.ui;
        if (!ui || !ui.barFill) return;
        const value = st.busy ? st.busy.progress : null;
        if (ui.bar) ui.bar.classList.toggle('is-indeterminate', value == null);
        ui.barFill.style.width = value == null ? '' : Math.round(Math.max(0, Math.min(1, value)) * 100) + '%';
    }

    function failNote(r, err) {
        if (isAbort(err) || gone(r)) return;
        r.notify({ type: 'error', message: tx(r, 'pixel.cutout_failed', 'The cutout failed.') });
        if (window.console) console.error('pixel cutout', err);
    }

    // --- engines: each returns { matte, mw, mh, engine } ---

    function downscaledImage(r, canvas, edge) {
        return r.imageFromCanvas(scaledCanvas(r, canvas, edge, null));
    }

    async function runClassic(r, source, opts) {
        const proxy = downscaledImage(r, source, PROXY_EDGE);
        const args = { image: proxy };
        if (opts.scribbles) args.scribbles = opts.scribbles;
        if (opts.init) args.init = opts.init;
        const res = await r.runJob('cutout.classic', args, { transfer: [proxy.data.buffer], signal: opts.signal, key: 'cutout-engine', onProgress: opts.onProgress });
        return { matte: res.alpha, mw: proxy.width, mh: proxy.height, engine: 'classic' };
    }

    async function runKey(r, source, opts) {
        const st = stateOf(r);
        const image = r.imageFromCanvas(source);
        const key = st.keyValues;
        const res = await r.runJob('cutout.chroma', {
            image, color: key.keyAuto ? 'auto' : key.keyColor, tolerance: key.tolerance, softness: key.softness, despill: 50
        }, { transfer: [image.data.buffer], signal: opts.signal, key: 'cutout-engine', onProgress: opts.onProgress });
        return { matte: res.alpha, mw: image.width, mh: image.height, engine: 'key' };
    }

    async function decodeMask(r, dataURL, width, height) {
        const img = await r.loadImage(dataURL);
        const canvas = r.makeCanvas(width, height);
        const ctx = canvas.getContext('2d');
        ctx.drawImage(img, 0, 0, width, height);
        const d = ctx.getImageData(0, 0, width, height).data;
        const out = new Uint8ClampedArray(width * height);
        for (let i = 0; i < out.length; i++) out[i] = d[i * 4];
        return out;
    }

    async function runAI(r, source, model, opts) {
        const upload = scaledCanvas(r, source, UPLOAD_EDGE, '#ffffff');
        const resp = await r.api('/api/pixel/cutout', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ model, source_data: upload.toDataURL('image/png') }),
            signal: opts.signal
        });
        if (!resp || !resp.mask) throw new Error('no mask');
        const width = resp.width || upload.width, height = resp.height || upload.height;
        const matte = await decodeMask(r, resp.mask, width, height);
        return { matte, mw: width, mh: height, engine: model };
    }

    function aiFailureReason(r, err) {
        const status = err && err.status;
        const kind = err && err.body && err.body.status;
        if (status === 409 && kind === 'unavailable') return tx(r, 'pixel.cutout_reason_unavailable', 'not available on this device');
        if (status === 409) return tx(r, 'pixel.cutout_reason_not_ready', 'the model is not downloaded');
        if (status === 429) return tx(r, 'pixel.cutout_reason_busy', 'another cutout is running');
        if (status === 504) return tx(r, 'pixel.cutout_reason_timeout', 'it took too long');
        if (status === 403) return tx(r, 'pixel.cutout_reason_readonly', 'not allowed in read-only mode');
        return tx(r, 'pixel.cutout_reason_error', 'the server could not run it');
    }

    // Picks what 'auto' (or an explicit AI choice that is not downloaded) really runs.
    async function chooseEngine(r, requested) {
        const st = stateOf(r);
        if (requested === 'classic' || requested === 'key') return { kind: requested };
        if (!st.statusLoaded) await loadStatus(r);
        if (requested === 'standard' || requested === 'precise') {
            const model = readyModel(r, requested);
            if (model) return { kind: 'ai', model };
            r.notify({ type: 'info', message: tx(r, 'pixel.cutout_ai_fallback', 'The AI cutout failed ({reason}). Using the classic engine instead.', { reason: tx(r, 'pixel.cutout_reason_not_ready', 'the model is not downloaded') }) });
            return { kind: 'classic' };
        }
        const model = readyModel(r, null);
        return model ? { kind: 'ai', model } : { kind: 'classic' };
    }

    async function computeMatte(r, source, requested, opts) {
        const choice = await chooseEngine(r, requested);
        if (choice.kind === 'key') return runKey(r, source, opts);
        if (choice.kind === 'ai') {
            try {
                return await runAI(r, source, choice.model, opts);
            } catch (err) {
                if (isAbort(err) || gone(r)) throw err;
                r.notify({ type: 'info', message: tx(r, 'pixel.cutout_ai_fallback', 'The AI cutout failed ({reason}). Using the classic engine instead.', { reason: aiFailureReason(r, err) }) });
                if (err && err.status === 409) loadStatus(r);
            }
        }
        return runClassic(r, source, opts);
    }

    // Matte of the whole image: the engine's raw matte plus the document-sized alpha after the default refinement.
    async function subjectMatte(r, source, requested, opts) {
        const raw = await computeMatte(r, source, requested, opts);
        const guide = r.imageFromCanvas(source);
        const res = await r.runJob('cutout.refine', {
            image: guide, alpha: raw.matte, alphaWidth: raw.mw, alphaHeight: raw.mh
        }, { transfer: [guide.data.buffer], signal: opts.signal, key: 'cutout-refine' });
        return Object.assign({ alpha: res.alpha, width: source.width, height: source.height }, raw);
    }

    function sourceCanvas(r, kind, layer) {
        r.commitPendingPreviews();
        return kind === 'layer' && layer ? layer.canvas : r.flattenDocument();
    }

    // --- applying a matte to a layer ---

    function defaultParams() {
        return { shift: 0, feather: 0, smooth: 0, decontaminate: true, decontaminateRadius: 6 };
    }

    // Puts refine values into the shared values object and, when the panel is mounted, into the controls.
    function setRefineValues(r, params) {
        const st = stateOf(r);
        REFINE_KEYS.forEach(key => {
            if (st.wires.refine) st.wires.refine.set(key, params[key]);
            else st.refineValues[key] = params[key];
        });
    }

    function applyMatte(r, layer, result, label) {
        const st = stateOf(r);
        const params = defaultParams();
        setRefineValues(r, params);
        writeMask(r, layer, result.alpha, result.width, result.height);
        r.markLayerDirty(layer, { mask: true });
        r.compositeLayers();
        r.pushHistory(label);
        rememberContext(r, layer, { matte: result.matte, mw: result.mw, mh: result.mh, engine: result.engine, params, strokes: [] });
        st.session = null;
        r.refreshLayerPanel();
        syncControls(r);
    }

    function guardLayer(r) {
        if (!r.imgWidth || !r.imgHeight) {
            r.notify({ type: 'info', message: tx(r, 'pixel.cutout_need_image', 'Open an image first.') });
            return null;
        }
        const layer = r.activeLayer();
        if (!layer || !r.requireRasterLayer(layer)) return null;
        return layer;
    }

    // Runs fn(ticket) with the busy indicator; fn returns true for success. Errors become one notification.
    async function guarded(r, label, fn) {
        const ticket = beginBusy(r, label);
        if (!ticket) return false;
        try {
            return await fn(ticket);
        } catch (err) {
            failNote(r, err);
            return false;
        } finally {
            endBusy(r, ticket);
        }
    }

    async function removeBackground(r) {
        await flushRefine(r);
        const layer = guardLayer(r);
        if (!layer) return false;
        const st = stateOf(r);
        return guarded(r, tx(r, 'pixel.cutout_busy_remove', 'Removing the background…'), async ticket => {
            const source = sourceCanvas(r, 'layer', layer);
            const result = await subjectMatte(r, source, st.engineValues.engine, {
                signal: ticket.ac.signal, onProgress: f => setProgress(r, ticket, f)
            });
            if (tokenStale(r, ticket.token) || r.layerById(layer.id) !== layer) return false;
            applyMatte(r, layer, result, 'cutout:remove-bg');
            r.notify({ type: 'success', message: tx(r, 'pixel.cutout_done', 'Background removed.') });
            return true;
        });
    }

    async function selectSubject(r) {
        await flushRefine(r);
        if (!r.imgWidth) { r.notify({ type: 'info', message: tx(r, 'pixel.cutout_need_image', 'Open an image first.') }); return false; }
        const layer = r.activeLayer();
        // a layer that was cut out already: its mask is the subject
        if (layer && lookupContext(r, layer)) {
            r.commitPendingPreviews();
            r.setSelectionMask(alphaOfCanvas(r, layer.maskCanvas), { mode: 'replace' });
            r.notify({ type: 'success', message: tx(r, 'pixel.cutout_selected', 'Subject selected.') });
            return true;
        }
        const st = stateOf(r);
        return guarded(r, tx(r, 'pixel.cutout_busy_select', 'Finding the subject…'), async ticket => {
            const result = await subjectMatte(r, sourceCanvas(r, 'document', null), st.engineValues.engine, {
                signal: ticket.ac.signal, onProgress: f => setProgress(r, ticket, f)
            });
            if (tokenStale(r, ticket.token)) return false;
            r.setSelectionMask(result.alpha, { mode: 'replace' });
            r.notify({ type: 'success', message: tx(r, 'pixel.cutout_selected', 'Subject selected.') });
            return true;
        });
    }

    // Alpha of the subject for the active layer: its mask when it has one, else a fresh removal (which writes the mask).
    async function subjectAlpha(r, layer) {
        if (!layer.maskCanvas) {
            if (!await removeBackground(r)) return null;
        }
        return layer.maskCanvas ? alphaOfCanvas(r, layer.maskCanvas) : null;
    }

    function insertLayer(r, layer, index, label, keepActive) {
        r.layers.splice(index, 0, layer);
        r.activeLayerIdx = keepActive;
        r.editMaskMode = false;
        r.layersStructureChanged();
        r.pushHistory(label);
    }

    async function cutOutToLayer(r) {
        await flushRefine(r);
        const layer = guardLayer(r);
        if (!layer) return false;
        if (r.layers.length >= r.MAX_LAYERS) { r.notify({ type: 'error', message: r.t('pixel.max_layers') }); return false; }
        const alpha = await subjectAlpha(r, layer);
        if (!alpha || r.layerById(layer.id) !== layer) return false;
        return guarded(r, tx(r, 'pixel.cutout_busy_layer', 'Cutting out…'), async ticket => {
            const out = await cutoutCanvas(r, layer, alpha, ticket);
            if (tokenStale(r, ticket.token) || r.layerById(layer.id) !== layer) return false;
            const created = r.createLayer({ name: layer.name + ' ' + tx(r, 'pixel.cutout_layer_suffix', 'cutout'), canvas: out.canvas });
            const index = r.layerIndexById(layer.id);
            layer.visible = false;
            insertLayer(r, created, index + 1, 'cutout:to-layer', index + 1);
            r.notify({ type: 'success', message: tx(r, 'pixel.cutout_layer_done', 'Cut out to a new layer.') });
            return true;
        });
    }

    // Layer pixels x matte as { canvas } (document-sized); the colour fringe estimate runs when the option is on.
    async function cutoutCanvas(r, layer, alpha, ticket) {
        const st = stateOf(r);
        const values = st.refineValues;
        if (values.decontaminate) {
            const image = r.imageFromCanvas(layer.canvas);
            const res = await r.runJob('cutout.decontaminate', { image, alpha, radius: values.decontaminateRadius }, { transfer: [image.data.buffer], signal: ticket.ac.signal, onProgress: f => setProgress(r, ticket, f) });
            return { canvas: r.canvasFromImage(res.image) };
        }
        const out = r.copyCanvas(layer.canvas);
        const ctx = out.getContext('2d');
        ctx.globalCompositeOperation = 'destination-in';
        ctx.drawImage(layer.maskCanvas, 0, 0);
        ctx.globalCompositeOperation = 'source-over';
        return { canvas: out };
    }

    async function blurBackground(r) {
        await flushRefine(r);
        const layer = guardLayer(r);
        if (!layer) return false;
        if (r.layers.length >= r.MAX_LAYERS) { r.notify({ type: 'error', message: r.t('pixel.max_layers') }); return false; }
        const alpha = await subjectAlpha(r, layer);
        if (!alpha || r.layerById(layer.id) !== layer) return false;
        const st = stateOf(r);
        return guarded(r, tx(r, 'pixel.cutout_busy_blur', 'Blurring the background…'), async ticket => {
            const image = r.imageFromCanvas(layer.canvas);
            const radius = Math.max(2, Math.round(Math.max(r.imgWidth, r.imgHeight) * st.blurValues.blurStrength / 100 * 0.03));
            const res = await r.runJob('cutout.blur', { image, alpha, radius }, { transfer: [image.data.buffer], signal: ticket.ac.signal, onProgress: f => setProgress(r, ticket, f) });
            if (tokenStale(r, ticket.token) || r.layerById(layer.id) !== layer) return false;
            const created = r.createLayer({ name: tx(r, 'pixel.cutout_layer_blur', 'Blurred background'), canvas: r.canvasFromImage(res.image) });
            const index = r.layerIndexById(layer.id);
            insertLayer(r, created, index, 'cutout:blur-bg', index + 1);
            r.notify({ type: 'success', message: tx(r, 'pixel.cutout_blur_done', 'Background blurred.') });
            return true;
        });
    }

    async function replaceBackground(r) {
        await flushRefine(r);
        const layer = guardLayer(r);
        if (!layer) return false;
        const st = stateOf(r);
        const values = st.replaceValues;
        if (values.replaceMode === 'transparent') {
            // the mask already makes the background transparent
            if (layer.maskCanvas) { r.notify({ type: 'info', message: tx(r, 'pixel.cutout_done', 'Background removed.') }); return true; }
            return removeBackground(r);
        }
        if (r.layers.length >= r.MAX_LAYERS) { r.notify({ type: 'error', message: r.t('pixel.max_layers') }); return false; }
        const alpha = await subjectAlpha(r, layer);
        if (!alpha || r.layerById(layer.id) !== layer) return false;
        return guarded(r, tx(r, 'pixel.cutout_busy_replace', 'Replacing the background…'), async ticket => {
            let canvas;
            if (values.replaceMode === 'generated') {
                const def = backgroundGenerators().find(entry => entry.id === values.replaceGenerator);
                if (!def) return false;
                const res = await r.runJob('gen.render', {
                    id: def.id, params: r.defaultParams(Array.isArray(def.params) ? def.params : []), width: r.imgWidth, height: r.imgHeight
                }, { signal: ticket.ac.signal, onProgress: f => setProgress(r, ticket, f), key: 'cutout-background' });
                const image = res && res.image ? res.image : res;
                if (!image || !image.data) throw new Error('no background');
                canvas = r.canvasFromImage(image);
            } else {
                canvas = r.makeCanvas(r.imgWidth, r.imgHeight);
                const ctx = canvas.getContext('2d');
                ctx.fillStyle = values.replaceColor;
                ctx.fillRect(0, 0, canvas.width, canvas.height);
            }
            if (tokenStale(r, ticket.token) || r.layerById(layer.id) !== layer) return false;
            const created = r.createLayer({ name: tx(r, 'pixel.cutout_layer_background', 'New background'), canvas });
            const index = r.layerIndexById(layer.id);
            insertLayer(r, created, index, 'cutout:replace-bg', index + 1);
            r.notify({ type: 'success', message: tx(r, 'pixel.cutout_replace_done', 'Background replaced.') });
            return true;
        });
    }

    // Burns the mask (and, with the fringe option, the decontaminated colours) into the layer pixels.
    async function applyMask(r) {
        await flushRefine(r);
        const layer = r.activeLayer();
        if (!layer || !layer.maskCanvas) {
            r.notify({ type: 'info', message: tx(r, 'pixel.cutout_no_mask', 'Remove the background first.') });
            return false;
        }
        if (layer.lockPixels || layer.lockAlpha) { r.notify({ type: 'info', message: r.t('pixel.layer_locked', 'This layer is locked.') }); return false; }
        if (!r.requireRasterLayer(layer)) return false;
        r.commitPendingPreviews();
        return guarded(r, tx(r, 'pixel.cutout_busy_apply', 'Applying the mask…'), async ticket => {
            const alpha = alphaOfCanvas(r, layer.maskCanvas);
            const baked = await cutoutCanvas(r, layer, alpha, ticket);
            if (tokenStale(r, ticket.token) || r.layerById(layer.id) !== layer) return false;
            const oldMask = layer.maskCanvas, oldRev = layer.maskRev;
            layer.maskCanvas = null;
            layer.maskEnabled = true;
            layer.maskRev = r.nextRev();
            r.editMaskMode = false;
            const ok = r.commitLayerPixels(layer, baked.canvas, { label: 'cutout:apply-mask', respectSelection: false });
            if (!ok) {
                layer.maskCanvas = oldMask;
                layer.maskRev = oldRev;
                return false;
            }
            r.refreshLayerPanel();
            r.notify({ type: 'success', message: tx(r, 'pixel.cutout_applied', 'Mask applied.') });
            return true;
        });
    }

    // --- refine (recompute the mask from the stored matte) ---

    function guideFor(r, st, layer, scale) {
        const W = r.imgWidth, H = r.imgHeight;
        const key = layer.id + ':' + layer.rev + ':' + W + 'x' + H + ':' + scale;
        if (st.guide && st.guide.key === key) return st.guide.image;
        const small = r.makeCanvas(Math.max(1, Math.round(W * scale)), Math.max(1, Math.round(H * scale)));
        const ctx = small.getContext('2d');
        ctx.imageSmoothingEnabled = true;
        ctx.imageSmoothingQuality = 'high';
        ctx.drawImage(layer.canvas, 0, 0, small.width, small.height);
        st.guide = { key, image: r.imageFromCanvas(small) };
        return st.guide.image;
    }

    async function recompute(r, layer, cx, params, preview, signal) {
        const st = stateOf(r);
        const W = r.imgWidth, H = r.imgHeight;
        const long = Math.max(W, H);
        const scale = preview && long > PREVIEW_EDGE ? PREVIEW_EDGE / long : 1;
        let guide, transfer = [];
        if (scale < 1) {
            guide = guideFor(r, st, layer, scale);
        } else {
            guide = r.imageFromCanvas(layer.canvas);
            transfer = [guide.data.buffer];
        }
        const res = await r.runJob('cutout.refine', {
            image: guide, alpha: cx.matte, alphaWidth: cx.mw, alphaHeight: cx.mh,
            shift: params.shift * scale, feather: params.feather * scale, smooth: params.smooth * scale
        }, { transfer, signal, key: 'cutout-refine' });
        return { alpha: res.alpha, width: guide.width, height: guide.height };
    }

    function paintRefined(r, layer, cx, res) {
        writeMask(r, layer, res.alpha, res.width, res.height);
        replayStrokes(layer.maskCanvas.getContext('2d'), cx.strokes);
        r.markLayerDirty(layer, { mask: true });
        r.compositeLayers();
    }

    function currentParams(st) {
        const out = {};
        REFINE_KEYS.forEach(key => { out[key] = st.refineValues[key]; });
        return out;
    }

    function beginSession(r) {
        const st = stateOf(r);
        const layer = r.activeLayer();
        const cx = layer ? lookupContext(r, layer) : null;
        if (st.session && layer && st.session.layerId === layer.id) return st.session;
        if (!cx) return null;
        st.session = { layerId: layer.id, cx, dirty: false, maskDirty: false, dead: false };
        return st.session;
    }

    function onRefineInput(r, key) {
        const st = stateOf(r);
        if (key === 'view') { updateViewOverlay(r); return; }
        const session = beginSession(r);
        if (!session) return;
        session.dirty = true;
        st.inputSeq++;
        if (REFINE_MASK_KEYS.includes(key)) {
            session.maskDirty = true;
            clearTimeout(st.previewTimer);
            st.previewTimer = setTimeout(() => { st.previewTimer = 0; runPreview(r); }, PREVIEW_DELAY_MS);
        }
        clearTimeout(st.commitTimer);
        st.commitTimer = setTimeout(() => { st.commitTimer = 0; commitRefine(r); }, COMMIT_IDLE_MS);
    }

    async function runPreview(r) {
        const st = stateOf(r);
        const session = st.session;
        if (!session || gone(r)) return;
        const layer = r.layerById(session.layerId);
        if (!layer) return;
        const seq = ++st.previewSeq;
        try {
            const res = await recompute(r, layer, session.cx, currentParams(st), true);
            if (seq !== st.previewSeq || gone(r) || st.session !== session) return;
            paintRefined(r, layer, session.cx, res);
        } catch (err) {
            if (!isAbort(err)) failNote(r, err);
        }
    }

    // Full-size recomputation of the pending slider changes, one history step.
    async function commitRefine(r) {
        const st = stateOf(r);
        const session = st.session;
        clearTimeout(st.commitTimer);
        st.commitTimer = 0;
        clearTimeout(st.previewTimer);
        st.previewTimer = 0;
        if (!session || !session.dirty || session.dead || st.committing || gone(r)) return;
        const layer = r.layerById(session.layerId);
        if (!layer) { session.dead = true; if (st.session === session) st.session = null; return; }
        st.committing = true;
        const seen = st.inputSeq;
        const params = currentParams(st);
        try {
            st.previewSeq++;
            if (session.maskDirty) {
                const res = await recompute(r, layer, session.cx, params, false);
                if (gone(r) || session.dead) return;
                paintRefined(r, layer, session.cx, res);
                r.pushHistory('cutout:refine');
            }
            // fringe settings change nothing on the mask, they only travel with the context
            const next = Object.assign({}, session.cx, { params });
            rememberContext(r, layer, next);
            session.cx = next;
            session.dirty = st.inputSeq !== seen;
            session.maskDirty = session.dirty && session.maskDirty;
        } catch (err) {
            if (!isAbort(err)) failNote(r, err);
        } finally {
            st.committing = false;
        }
        if (st.session === session && session.dirty) {
            st.commitTimer = setTimeout(() => { st.commitTimer = 0; commitRefine(r); }, PREVIEW_DELAY_MS);
        }
    }

    async function flushRefine(r) {
        const st = stateOf(r);
        if (st.session && st.session.dirty) await commitRefine(r);
        while (st.committing) await new Promise(resolve => setTimeout(resolve, 20));
    }

    async function resetRefine(r) {
        await flushRefine(r);
        const layer = r.activeLayer();
        const cx = layer ? lookupContext(r, layer) : null;
        if (!cx) { r.notify({ type: 'info', message: tx(r, 'pixel.cutout_no_mask', 'Remove the background first.') }); return false; }
        const st = stateOf(r);
        const params = defaultParams();
        setRefineValues(r, params);
        const next = Object.assign({}, cx, { params, strokes: [] });
        return guarded(r, tx(r, 'pixel.cutout_busy_refine', 'Refining…'), async ticket => {
            const res = await recompute(r, layer, next, params, false, ticket.ac.signal);
            if (tokenStale(r, ticket.token)) return false;
            paintRefined(r, layer, next, res);
            r.pushHistory('cutout:reset');
            rememberContext(r, layer, next);
            st.session = null;
            return true;
        });
    }

    // The brush in smart mode: the strokes become hints for the classic engine, which starts from the current matte.
    async function smartRefine(r, layer, cx) {
        const st = stateOf(r);
        await guarded(r, tx(r, 'pixel.cutout_busy_classic', 'Analyzing the image…'), async ticket => {
            const source = layer.canvas;
            const proxy = downscaledImage(r, source, PROXY_EDGE);
            const pw = proxy.width, ph = proxy.height;
            const plane = (strokes, mode) => {
                const canvas = r.makeCanvas(pw, ph);
                const ctx = canvas.getContext('2d');
                ctx.scale(pw / r.imgWidth, ph / r.imgHeight);
                replayStrokes(ctx, strokes.filter(stroke => stroke.mode === mode));
                return alphaOfCanvas(r, canvas);
            };
            const matteCanvas = r.makeCanvas(cx.mw, cx.mh);
            paintMask(r, matteCanvas, cx.matte, cx.mw, cx.mh);
            const init = alphaOfCanvas(r, scaledCanvas(r, matteCanvas, PROXY_EDGE, null));
            const res = await r.runJob('cutout.classic', {
                image: proxy, init, scribbles: { fg: plane(cx.strokes, 'keep'), bg: plane(cx.strokes, 'remove') }
            }, { transfer: [proxy.data.buffer], signal: ticket.ac.signal, key: 'cutout-engine', onProgress: f => setProgress(r, ticket, f) });
            if (tokenStale(r, ticket.token) || r.layerById(layer.id) !== layer) return false;
            const next = Object.assign({}, cx, { matte: res.alpha, mw: pw, mh: ph, engine: 'classic', strokes: [] });
            const out = await recompute(r, layer, next, next.params, false, ticket.ac.signal);
            paintRefined(r, layer, next, out);
            r.pushHistory('cutout:smart');
            rememberContext(r, layer, next);
            st.session = null;
            return true;
        });
    }

    // --- view modes (an overlay below the selection outline) ---

    function viewCanvas(r, st, layer, view) {
        const W = r.imgWidth, H = r.imgHeight;
        const key = [layer.id, layer.rev, layer.maskRev, view, W, H].join(':');
        if (st.viewCache && st.viewCache.key === key) return st.viewCache.canvas;
        const canvas = st.viewCache && st.viewCache.canvas.width === W && st.viewCache.canvas.height === H ? st.viewCache.canvas : r.makeCanvas(W, H);
        const ctx = canvas.getContext('2d');
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalCompositeOperation = 'source-over';
        ctx.clearRect(0, 0, W, H);
        if (view === 'red') {
            ctx.drawImage(layer.canvas, 0, 0);
            const tint = r.makeCanvas(W, H);
            const tctx = tint.getContext('2d');
            tctx.fillStyle = 'rgba(255, 40, 40, 0.6)';
            tctx.fillRect(0, 0, W, H);
            tctx.globalCompositeOperation = 'destination-out';
            tctx.drawImage(layer.maskCanvas, 0, 0);
            ctx.drawImage(tint, 0, 0);
        } else {
            if (view === 'checker') {
                const cell = Math.max(8, Math.round(Math.max(W, H) / 80));
                for (let y = 0; y * cell < H; y++) {
                    for (let x = 0; x * cell < W; x++) {
                        ctx.fillStyle = (x + y) % 2 ? '#8a8a8a' : '#c8c8c8';
                        ctx.fillRect(x * cell, y * cell, cell, cell);
                    }
                }
            } else {
                ctx.fillStyle = view === 'white' ? '#ffffff' : '#000000';
                ctx.fillRect(0, 0, W, H);
            }
            const subject = r.copyCanvas(layer.canvas);
            const sctx = subject.getContext('2d');
            sctx.globalCompositeOperation = 'destination-in';
            sctx.drawImage(layer.maskCanvas, 0, 0);
            ctx.drawImage(subject, 0, 0);
        }
        st.viewCache = { key, canvas };
        return canvas;
    }

    // The overlay only exists while a view mode other than "normal" is chosen.
    function updateViewOverlay(r) {
        const st = stateOf(r);
        const wanted = st.refineValues.view !== 'normal';
        if (wanted && !st.viewOverlay) {
            st.viewOverlay = true;
            r.registerOverlay('cutout-view', drawView, 40);
        } else if (!wanted && st.viewOverlay) {
            st.viewOverlay = false;
            r.unregisterOverlay('cutout-view');
        } else {
            r.requestOverlay();
        }
    }

    function drawView(ctx, r) {
        const st = stateOf(r);
        const view = st.refineValues.view;
        if (view === 'normal' || !r.imgWidth) return;
        const layer = r.activeLayer();
        if (!layer || !layer.maskCanvas || !layer.maskEnabled) return;
        ctx.drawImage(viewCanvas(r, st, layer, view), 0, 0);
    }

    // --- panel ---

    function modelCards(r, st) {
        const esc = r.esc;
        if (!st.status) {
            return `<p class="pixel-draw-hint">${esc(st.statusError ? tx(r, 'pixel.cutout_status_error', 'Could not read the AI status.') : tx(r, 'pixel.cutout_status_loading', 'Checking the AI models…'))}</p>`;
        }
        if (!st.status.available) {
            return `<p class="pixel-draw-hint">${esc(tx(r, 'pixel.cutout_ai_unavailable', 'AI cutout is not available on this device. The classic engine still works.'))}</p>`;
        }
        const models = Array.isArray(st.status.models) ? st.status.models : [];
        const cards = models.map(model => {
            const percent = Math.round(Math.max(0, Math.min(1, Number(model.progress) || 0)) * 100);
            const size = Math.round((Number(model.size_bytes) || 0) / 1e6);
            let state;
            if (model.ready) state = `<span class="pixel-cutout-state is-ready">${esc(tx(r, 'pixel.cutout_model_ready', 'Ready'))}</span>`;
            else if (model.downloading) state = `<span class="pixel-cutout-state">${esc(tx(r, 'pixel.cutout_model_downloading', 'Downloading {percent}%', { percent }))}</span>`;
            else state = `<span class="pixel-cutout-state">${esc(tx(r, 'pixel.cutout_model_missing', 'Not downloaded'))}</span>`;
            const title = tx(r, model.id === 'precise' ? 'pixel.cutout_engine_precise' : 'pixel.cutout_engine_standard', model.id === 'precise' ? 'AI precise' : 'AI standard');
            let action = '';
            if (model.downloading) {
                action = `<div class="pixel-cutout-bar"><span style="width:${percent}%"></span></div>`;
            } else if (!model.ready && !r.ctx.readonly) {
                action = `<button class="pixel-btn pixel-btn-full" type="button" data-cutout-action="download" data-cutout-model="${esc(model.id)}">${esc(tx(r, 'pixel.cutout_model_download', 'Download model ({size} MB)', { size }))}</button>`;
            }
            const error = model.error ? `<div class="pixel-cutout-note is-error">${esc(tx(r, 'pixel.cutout_model_error', 'Error: {error}', { error: model.error }))}</div>` : '';
            return `<div class="pixel-cutout-model" data-cutout-model-card="${esc(model.id)}">
                <div class="pixel-cutout-model-head"><span>${esc(title)}</span>${state}</div>
                <div class="pixel-cutout-note">${esc(model.name || '')} · ${esc(tx(r, 'pixel.cutout_model_license', 'License'))}: ${esc(model.license || '')} · ${size} MB</div>
                ${action}${error}
            </div>`;
        }).join('');
        const readonly = r.ctx.readonly ? `<p class="pixel-draw-hint">${esc(tx(r, 'pixel.cutout_ai_readonly', 'Model downloads are disabled in read-only mode.'))}</p>` : '';
        const failure = st.statusError ? `<div class="pixel-cutout-note is-error">${esc(st.statusError)}</div>` : '';
        return cards + readonly + failure;
    }

    function buildPanel(r) {
        const st = stateOf(r);
        const esc = r.esc;
        const label = (key, fallback) => esc(tx(r, key, fallback));
        return `<div class="pixel-cutout" data-cutout-root>
            <p class="pixel-draw-hint pixel-cutout-hint">${label('pixel.cutout_hint', 'Removes the background without touching the pixels: the result is a mask on the active layer.')}</p>
            <section class="pixel-cutout-group">
                <div class="pixel-cutout-title">${label('pixel.cutout_engine', 'Engine')}</div>
                <div data-cutout-params="engine">${r.buildParamControls(ENGINE_SCHEMA, st.engineValues, { idPrefix: 'pixel-cutout-engine' })}</div>
                <div data-cutout-params="key" hidden>${r.buildParamControls(KEY_SCHEMA, st.keyValues, { idPrefix: 'pixel-cutout-key' })}</div>
                <div class="pixel-cutout-models" data-cutout-models></div>
            </section>
            <section class="pixel-cutout-group">
                <div class="pixel-cutout-actions">
                    <button class="pixel-btn pixel-btn-primary pixel-cutout-wide" type="button" data-cutout-action="remove">${label('pixel.cutout_remove_bg', 'Remove background')}</button>
                    <button class="pixel-btn" type="button" data-cutout-action="select">${label('pixel.cutout_select_subject', 'Select subject')}</button>
                    <button class="pixel-btn" type="button" data-cutout-action="layer">${label('pixel.cutout_to_layer', 'Cut out to new layer')}</button>
                </div>
                <hr class="pixel-divider">
                <div data-cutout-params="blur">${r.buildParamControls(BLUR_SCHEMA, st.blurValues, { idPrefix: 'pixel-cutout-blur' })}</div>
                <button class="pixel-btn pixel-btn-full" type="button" data-cutout-action="blur">${label('pixel.cutout_blur_bg', 'Blur background')}</button>
                <hr class="pixel-divider">
                <div data-cutout-params="replace">${r.buildParamControls(st.replaceSchema, st.replaceValues, { idPrefix: 'pixel-cutout-replace' })}</div>
                <button class="pixel-btn pixel-btn-full" type="button" data-cutout-action="replace">${label('pixel.cutout_replace_bg', 'Replace background')}</button>
            </section>
            <div class="pixel-cutout-busy" data-cutout-busy hidden>
                <span data-cutout-busy-label aria-live="polite"></span>
                <div class="pixel-cutout-bar is-indeterminate" role="progressbar" data-cutout-bar><span data-cutout-bar-fill></span></div>
                <button class="pixel-btn" type="button" data-cutout-action="cancel">${label('pixel.cutout_cancel', 'Cancel')}</button>
            </div>
            <section class="pixel-cutout-group pixel-cutout-refine" data-cutout-refine>
                <div class="pixel-cutout-title">${label('pixel.cutout_refine', 'Refine mask')}</div>
                <p class="pixel-draw-hint" data-cutout-refine-hint>${label('pixel.cutout_refine_hint', 'Available once the active layer has a cutout mask.')}</p>
                <button class="pixel-btn pixel-btn-full" type="button" data-cutout-action="brush">${r.toolSvg('cutout-brush')} ${label('pixel.cutout_brush', 'Cutout brush')}</button>
                <div class="pixel-cutout-refine-body" data-cutout-refine-body>
                    <div data-cutout-params="refine">${r.buildParamControls(REFINE_SCHEMA, st.refineValues, { idPrefix: 'pixel-cutout-refine' })}</div>
                    <div class="pixel-panel-actions">
                        <button class="pixel-btn" type="button" data-cutout-action="apply">${label('pixel.cutout_apply_mask', 'Apply mask')}</button>
                        <button class="pixel-btn" type="button" data-cutout-action="reset">${label('pixel.cutout_reset', 'Reset')}</button>
                    </div>
                </div>
            </section>
        </div>`;
    }

    function mountPanel(r, el) {
        const st = stateOf(r);
        const part = name => el.querySelector('[data-cutout-params="' + name + '"]');
        st.ui = {
            el,
            models: el.querySelector('[data-cutout-models]'),
            busy: el.querySelector('[data-cutout-busy]'),
            busyLabel: el.querySelector('[data-cutout-busy-label]'),
            bar: el.querySelector('[data-cutout-bar]'),
            barFill: el.querySelector('[data-cutout-bar-fill]'),
            refine: el.querySelector('[data-cutout-refine]'),
            refineBody: el.querySelector('[data-cutout-refine-body]'),
            refineHint: el.querySelector('[data-cutout-refine-hint]'),
            key: part('key'),
            actions: () => Array.from(el.querySelectorAll('[data-cutout-action]'))
        };
        st.wires.engine = r.wireParamControls(part('engine'), ENGINE_SCHEMA, st.engineValues, () => refreshPanel(r));
        st.wires.key = r.wireParamControls(st.ui.key, KEY_SCHEMA, st.keyValues, () => {});
        st.wires.blur = r.wireParamControls(part('blur'), BLUR_SCHEMA, st.blurValues, () => {});
        st.wires.replace = r.wireParamControls(part('replace'), st.replaceSchema, st.replaceValues, () => {});
        st.wires.refine = r.wireParamControls(part('refine'), REFINE_SCHEMA, st.refineValues, key => onRefineInput(r, key));
        el.addEventListener('click', event => {
            const button = event.target && event.target.closest ? event.target.closest('[data-cutout-action]') : null;
            if (!button || button.disabled) return;
            runAction(r, button.dataset.cutoutAction, button.dataset.cutoutModel);
        });
        // a slider released: do not wait for the idle timer
        el.addEventListener('change', event => {
            if (event.target && event.target.closest && event.target.closest('[data-cutout-params="refine"]')) commitRefine(r);
        });
        refreshPanel(r);
    }

    function runAction(r, action, model) {
        switch (action) {
            case 'remove': removeBackground(r); break;
            case 'select': selectSubject(r); break;
            case 'layer': cutOutToLayer(r); break;
            case 'blur': blurBackground(r); break;
            case 'replace': replaceBackground(r); break;
            case 'apply': applyMask(r); break;
            case 'reset': resetRefine(r); break;
            case 'brush': r.setActiveTool('cutout-brush'); break;
            case 'download': downloadModel(r, model); break;
            case 'cancel': if (stateOf(r).busy) stateOf(r).busy.ac.abort(); break;
            default: break;
        }
    }

    function refreshPanel(r) {
        const st = stateOf(r);
        if (!st.ui || gone(r) || st.stroke || st.refreshFrame) return;
        const run = () => { st.refreshFrame = 0; if (!gone(r)) paintPanel(r); };
        if (typeof requestAnimationFrame === 'function') st.refreshFrame = requestAnimationFrame(run);
        else run();
    }

    function paintPanel(r) {
        const st = stateOf(r);
        const ui = st.ui;
        if (!ui) return;
        const cards = modelCards(r, st);
        if (ui.cardsHtml !== cards) {
            ui.cardsHtml = cards;
            ui.models.innerHTML = cards;
        }
        ui.key.hidden = st.engineValues.engine !== 'key';
        const hasDoc = !!r.imgWidth;
        const busy = !!st.busy;
        ui.actions().forEach(button => {
            const action = button.dataset.cutoutAction;
            if (action === 'cancel') return;
            if (action === 'download') { button.disabled = busy; return; }
            button.disabled = !hasDoc || busy;
        });
        ui.busy.hidden = !busy;
        if (busy) ui.busyLabel.textContent = st.busy.label;
        paintProgress(st);
        const layer = r.activeLayer();
        const refinable = !!(layer && layer.maskCanvas && (lookupContext(r, layer) || (st.session && st.session.layerId === layer.id)));
        ui.refine.classList.toggle('is-disabled', !refinable);
        ui.refineBody.inert = !refinable;
        ui.refineHint.hidden = refinable;
        syncControls(r);
        paintToolButton(r);
    }

    function paintToolButton(r) {
        const st = stateOf(r);
        if (!st.ui) return;
        st.ui.el.querySelectorAll('[data-cutout-action="brush"]').forEach(button => button.classList.toggle('active', r.activeTool === 'cutout-brush'));
    }

    // Slider positions follow the active refine context (undo, redo, another layer).
    function syncControls(r) {
        const st = stateOf(r);
        const wire = st.wires.refine;
        const layer = r.activeLayer();
        if (!wire || !layer) return;
        if (st.session && st.session.dirty) return;
        const cx = lookupContext(r, layer);
        if (!cx) return;
        REFINE_KEYS.forEach(key => {
            if (st.refineValues[key] !== cx.params[key]) wire.set(key, cx.params[key]);
        });
    }

    // --- the brush tool ---

    function brushOptions(r) {
        const st = stateOf(r);
        const v = st.brushValues;
        const esc = r.esc;
        const label = (key, fallback) => esc(tx(r, key, fallback));
        const seg = (mode, key, fallback) => `<button type="button" class="pixel-param-seg-btn${v.mode === mode ? ' active' : ''}" data-cutout-mode="${mode}" aria-pressed="${v.mode === mode ? 'true' : 'false'}">${label(key, fallback)}</button>`;
        return `<div class="pixel-param-segmented" role="group" data-cutout-brush-mode>${seg('keep', 'pixel.cutout_brush_keep', 'Keep')}${seg('remove', 'pixel.cutout_brush_remove', 'Remove')}</div>
            <div class="pixel-slider-row"><span class="pixel-slider-label">${label('pixel.cutout_brush_size', 'Size')}</span><input type="range" class="pixel-slider" data-cutout-size min="2" max="300" value="${v.size}"><span class="pixel-slider-value" data-cutout-size-val>${v.size}px</span></div>
            <div class="pixel-slider-row"><span class="pixel-slider-label">${label('pixel.cutout_brush_hardness', 'Hardness')}</span><input type="range" class="pixel-slider" data-cutout-hardness min="0" max="100" value="${v.hardness}"><span class="pixel-slider-value" data-cutout-hardness-val>${v.hardness}%</span></div>
            <label class="pixel-checkbox-row"><input type="checkbox" data-cutout-smart${v.smart ? ' checked' : ''}> ${label('pixel.cutout_brush_smart', 'Smart: re-run the engine on release')}</label>
            <p class="pixel-draw-hint">${label('pixel.cutout_brush_hint', 'Hold Alt to switch between keep and remove.')}</p>`;
    }

    function wireBrushOptions(r, bar) {
        const st = stateOf(r);
        const v = st.brushValues;
        const modes = Array.from(bar.querySelectorAll('[data-cutout-mode]'));
        modes.forEach(button => button.addEventListener('click', () => {
            v.mode = button.dataset.cutoutMode;
            modes.forEach(other => {
                const on = other.dataset.cutoutMode === v.mode;
                other.classList.toggle('active', on);
                other.setAttribute('aria-pressed', on ? 'true' : 'false');
            });
            r.requestOverlay();
        }));
        const bind = (selector, valueSelector, key, suffix) => {
            const input = bar.querySelector(selector), out = bar.querySelector(valueSelector);
            if (!input) return;
            input.addEventListener('input', () => {
                v[key] = Number(input.value);
                if (out) out.textContent = v[key] + suffix;
                r.requestOverlay();
            });
        };
        bind('[data-cutout-size]', '[data-cutout-size-val]', 'size', 'px');
        bind('[data-cutout-hardness]', '[data-cutout-hardness-val]', 'hardness', '%');
        const smart = bar.querySelector('[data-cutout-smart]');
        if (smart) smart.addEventListener('change', () => { v.smart = !!smart.checked; });
    }

    function strokeMode(st, alt) {
        const base = st.brushValues.mode;
        return alt ? (base === 'keep' ? 'remove' : 'keep') : base;
    }

    function brushDown(r, e, p) {
        const st = stateOf(r);
        if (e.button !== 0 || st.busy || st.committing) return;
        const layer = r.activeLayer();
        if (!layer || !r.requireRasterLayer(layer)) return;
        r.commitPendingPreviews();
        const before = lookupContext(r, layer);
        let created = false;
        if (!layer.maskCanvas) {
            // painting on a layer without a mask starts one that shows everything
            layer.maskCanvas = r.makeCanvas(r.imgWidth, r.imgHeight);
            const mctx = layer.maskCanvas.getContext('2d');
            mctx.fillStyle = '#ffffff';
            mctx.fillRect(0, 0, layer.maskCanvas.width, layer.maskCanvas.height);
            created = true;
        }
        layer.maskEnabled = true;
        const v = st.brushValues;
        st.stroke = { layerId: layer.id, before, created, record: { mode: strokeMode(st, p.alt), size: v.size, hardness: v.hardness, pts: [[p.x, p.y]] } };
        const rec = st.stroke.record;
        paintSegment(layer.maskCanvas.getContext('2d'), p.x, p.y, p.x, p.y, rec.size, rec.hardness, rec.mode);
        r.markLayerDirty(layer, { mask: true });
        r.compositeLayers();
    }

    function brushMove(r, e, p) {
        const st = stateOf(r);
        r.cursorPos = { x: p.x, y: p.y };
        const stroke = st.stroke;
        if (!stroke) { r.requestOverlay(); return; }
        const layer = r.layerById(stroke.layerId);
        if (!layer || !layer.maskCanvas) return;
        const rec = stroke.record;
        const last = rec.pts[rec.pts.length - 1];
        if (last[0] === p.x && last[1] === p.y) return;
        paintSegment(layer.maskCanvas.getContext('2d'), last[0], last[1], p.x, p.y, rec.size, rec.hardness, rec.mode);
        rec.pts.push([p.x, p.y]);
        r.markLayerDirty(layer, { mask: true });
        r.compositeLayers();
        r.requestOverlay();
    }

    function brushUp(r) {
        const st = stateOf(r);
        const stroke = st.stroke;
        st.stroke = null;
        if (!stroke) return;
        const layer = r.layerById(stroke.layerId);
        if (!layer) return;
        r.markLayerDirty(layer, { mask: true });
        r.compositeLayers();
        r.pushHistory('cutout:brush');
        r.refreshLayerPanel();
        let next = null;
        if (stroke.before) {
            next = Object.assign({}, stroke.before, { strokes: stroke.before.strokes.concat([stroke.record]) });
            rememberContext(r, layer, next);
        }
        st.session = null;
        refreshPanel(r);
        if (next && st.brushValues.smart) smartRefine(r, layer, next);
    }

    function paintBrushRing(ctx, r) {
        const st = stateOf(r);
        const pos = r.cursorPos;
        if (!pos) return;
        const mode = st.stroke ? st.stroke.record.mode : strokeMode(st, st.hoverAlt);
        const radius = Math.max(1, st.brushValues.size / 2);
        ctx.lineWidth = 2;
        ctx.strokeStyle = 'rgba(0, 0, 0, 0.7)';
        ctx.beginPath();
        ctx.arc(pos.x, pos.y, radius + 1, 0, Math.PI * 2);
        ctx.stroke();
        ctx.lineWidth = 1;
        ctx.strokeStyle = mode === 'keep' ? 'rgba(90, 235, 140, 0.95)' : 'rgba(255, 110, 110, 0.95)';
        ctx.beginPath();
        ctx.arc(pos.x, pos.y, radius, 0, Math.PI * 2);
        ctx.stroke();
    }

    // --- registrations ---

    Pixel.registerPanel({
        id: 'cutout', labelKey: 'pixel.cutout_panel', fallback: 'Cutout', order: 75, dock: 'inspector',
        build: r => buildPanel(r),
        mount: (r, el) => mountPanel(r, el),
        onShow: r => {
            const st = stateOf(r);
            if (!st.statusLoaded || st.statusError || Date.now() - st.statusAt > 15000) loadStatus(r); else schedulePoll(r);
            refreshPanel(r);
        },
        onHide: r => {
            const st = stateOf(r);
            if (st.pollTimer) { clearTimeout(st.pollTimer); st.pollTimer = 0; }
            if (st.refineValues.view !== 'normal') {
                if (st.wires.refine) st.wires.refine.set('view', 'normal'); else st.refineValues.view = 'normal';
                updateViewOverlay(r);
            }
            commitRefine(r);
        }
    });

    Pixel.registerTool({
        id: 'cutout-brush', group: 'retouch', order: 20, svg: BRUSH_SVG, labelKey: 'pixel.cutout_brush', fallback: 'Cutout brush',
        raster: false, cursor: 'none',
        options: r => brushOptions(r),
        wireOptions: (r, bar) => wireBrushOptions(r, bar),
        onActivate: r => {
            const layer = r.activeLayer();
            if (layer && !layer.maskCanvas) r.notify({ type: 'info', message: tx(r, 'pixel.cutout_brush_nomask', 'The brush paints on the layer mask. The first stroke creates one.') });
        },
        onDeactivate: r => { r.cursorPos = null; paintToolButton(r); refreshPanel(r); },
        onDown: (r, e, p) => brushDown(r, e, p),
        onMove: (r, e, p) => brushMove(r, e, p),
        onUp: r => brushUp(r),
        onHover: (r, e, p) => { stateOf(r).hoverAlt = !!p.alt; r.cursorPos = { x: p.x, y: p.y }; r.requestOverlay(); },
        overlay: (ctx, r) => paintBrushRing(ctx, r)
    });

    // Menu and command entries open the panel first, so the progress and the cancel button are in view. The Select menu has
    // no cutout item: "Select subject" there belongs to the selection package, which calls runtime.computeSubjectMask.
    const needsImage = r => !!r.imgWidth;
    const inPanel = run => r => { r.showPanel('cutout'); return run(r); };
    Pixel.registerMenuItems('ai', [
        { id: 'cutout-remove-bg', labelKey: 'pixel.cutout_remove_bg', fallback: 'Remove background', icon: 'image', keywords: ['cutout', 'background', 'matte', 'transparent'], enabled: needsImage, action: inPanel(removeBackground) },
        { id: 'cutout-to-layer', labelKey: 'pixel.cutout_to_layer', fallback: 'Cut out to new layer', icon: 'scissors', keywords: ['cutout', 'subject', 'extract'], enabled: needsImage, action: inPanel(cutOutToLayer) },
        { id: 'cutout-blur-bg', labelKey: 'pixel.cutout_blur_bg', fallback: 'Blur background', icon: 'image', keywords: ['portrait', 'bokeh', 'cutout'], enabled: needsImage, action: inPanel(blurBackground) },
        { id: 'cutout-panel', labelKey: 'pixel.cutout_panel', fallback: 'Cutout', icon: 'image', keywords: ['background', 'mask', 'refine'], action: r => r.showPanel('cutout') }
    ], { order: 50 });

    Pixel.registerModule('cutout', function install(r) {
        const st = createState(r);
        st.replaceValues = r.defaultParams(st.replaceSchema);
        r.cutoutState = st;

        Object.assign(r, {
            // Document-sized matte of the flattened document (source 'document', the default) or of the active layer
            // (source 'layer'): Promise of { alpha, width, height, engine }. engine: auto | standard | precise | classic | key.
            computeSubjectMask: Pixel.bindRuntime(r, async function computeSubjectMask(options) {
                const o = options || {};
                if (!this.imgWidth || !this.imgHeight) throw new Error('no document');
                const layer = this.activeLayer();
                const ticket = beginBusy(this, tx(this, 'pixel.cutout_busy_select', 'Finding the subject…'));
                if (!ticket) throw new Error('busy');
                try {
                    if (o.signal) o.signal.addEventListener('abort', () => ticket.ac.abort(), { once: true });
                    const source = sourceCanvas(this, o.source === 'layer' ? 'layer' : 'document', layer);
                    const result = await subjectMatte(this, source, o.engine || 'auto', {
                        signal: ticket.ac.signal,
                        onProgress: f => { setProgress(this, ticket, f); if (typeof o.onProgress === 'function') o.onProgress(f); }
                    });
                    return { alpha: result.alpha, width: result.width, height: result.height, engine: result.engine };
                } finally {
                    endBusy(this, ticket);
                }
            }),
            removeBackground: Pixel.bindRuntime(r, function removeBackgroundCommand() { return removeBackground(this); }),
            cutoutSelectSubject: Pixel.bindRuntime(r, function cutoutSelectSubject() { return selectSubject(this); }),
            cutoutToLayer: Pixel.bindRuntime(r, function cutoutToLayer() { return cutOutToLayer(this); }),
            cutoutBlurBackground: Pixel.bindRuntime(r, function cutoutBlurBackground() { return blurBackground(this); }),
            cutoutApplyMask: Pixel.bindRuntime(r, function cutoutApplyMask() { return applyMask(this); }),
            // options: { mode: 'solid' | 'transparent' | 'generated', color: '#rrggbb', generator: id } (defaults: the panel's values)
            cutoutReplaceBackground: Pixel.bindRuntime(r, function cutoutReplaceBackground(options) {
                const o = options || {};
                if (o.mode) st.replaceValues.replaceMode = o.mode;
                if (o.color) st.replaceValues.replaceColor = o.color;
                if (o.generator) st.replaceValues.replaceGenerator = o.generator;
                return replaceBackground(this);
            }),
            // Sets one refine value (shift, feather, smooth, decontaminate, decontaminateRadius, view) like its slider does:
            // live preview now, one history step once the value has settled.
            cutoutSetRefine: Pixel.bindRuntime(r, function cutoutSetRefine(key, value) {
                if (!REFINE_KEYS.concat(['view']).includes(key)) return false;
                if (st.wires.refine) st.wires.refine.set(key, value); else st.refineValues[key] = value;
                onRefineInput(this, key);
                return true;
            })
        });

        const scheduleRefresh = () => refreshPanel(r);
        r.on('layers-changed', () => {
            if (st.session && !st.session.dirty && !st.committing) st.session = null;
            if (st.refineValues.view !== 'normal') r.requestOverlay();
            scheduleRefresh();
        });
        r.on('active-layer-changed', () => {
            if (st.session && st.session.dirty) commitRefine(r);
            st.session = null;
            st.guide = null;
            scheduleRefresh();
        });
        r.on('history-changed', scheduleRefresh);
        r.on('document-changed', event => {
            if (!event || event.reason !== 'history') {
                if (st.session) st.session.dead = true;
                st.remembered.clear();
                st.session = null;
                st.guide = null;
                st.viewCache = null;
            }
            scheduleRefresh();
        });
        r.on('tool-changed', () => { paintToolButton(r); });

        const baseDispose = r.disposeCore;
        r.disposeCore = Pixel.bindRuntime(r, function disposeCore() {
            clearTimeout(st.pollTimer);
            clearTimeout(st.previewTimer);
            clearTimeout(st.commitTimer);
            if (st.refreshFrame && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(st.refreshFrame);
            if (st.busy) st.busy.ac.abort();
            st.remembered.clear();
            st.session = null;
            st.guide = null;
            st.viewCache = null;
            return baseDispose.apply(this, arguments);
        });
    });
})();
