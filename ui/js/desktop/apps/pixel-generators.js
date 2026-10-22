// Pixel module "generators" (WP6): the Generate panel, the Generate menu and one command per generator.
// The renderers live in pixel-lib-gen.js (gen.render, runs in the worker); Pixel.GENERATORS is its catalog:
// { id, cat, labelKey, fallback, params (schema for runtime.buildParamControls), presets: [{ id, labelKey, fallback, values }] }.
(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const lib = () => window.AuraPixelLib || null;
    const CATS = ['noise', 'texture', 'pattern', 'math'];
    const CAT_NAMES = { noise: 'Noise', texture: 'Textures', pattern: 'Patterns', math: 'Math' };
    const BLEND_NAMES = {
        'normal': 'Normal', 'multiply': 'Multiply', 'screen': 'Screen', 'overlay': 'Overlay', 'darken': 'Darken', 'lighten': 'Lighten',
        'color-dodge': 'Color dodge', 'color-burn': 'Color burn', 'hard-light': 'Hard light', 'soft-light': 'Soft light',
        'difference': 'Difference', 'exclusion': 'Exclusion', 'hue': 'Hue', 'saturation': 'Saturation', 'color': 'Color', 'luminosity': 'Luminosity'
    };
    const THUMB = 64;
    const PREVIEW_EDGE = 300;
    const MAX_DOC = 8192;

    Pixel.GENERATORS = (lib() && lib().GENERATORS) || [];
    Pixel.generatorById = id => Pixel.GENERATORS.find(def => def.id === id) || null;

    // Thumbnails of the default look are the same in every window: rendered once, painted from the cache afterwards.
    const thumbCache = new Map();
    const states = new WeakMap();

    const keyPart = text => String(text).toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');
    const isAbort = err => !!(err && err.name === 'AbortError');
    const clone = value => (value && typeof value === 'object' ? JSON.parse(JSON.stringify(value)) : value);
    const clampSize = value => Math.max(16, Math.min(MAX_DOC, Math.round(Number(value)) || 1024));

    function stateOf(r) {
        let st = states.get(r);
        if (!st) {
            st = {
                el: null, cat: 'all', query: '', id: '', values: null, memory: {}, wire: null,
                output: 'layer', blend: 'normal', opacity: 100, docW: 1024, docH: 1024, live: false,
                transient: null, previewLayer: null, liveCanvas: null, composeCanvas: null,
                previewTimer: 0, liveTimer: 0, liveToken: 0, previewToken: 0, thumbToken: 0, thumbKeySuffix: '', schemas: null, controller: null, working: false
            };
            states.set(r, st);
        }
        return st;
    }

    // The desktop t() returns the key itself for a missing translation (its 2nd argument is a replacement map), so every
    // string falls back to its English text here until the key exists in the language files.
    const tx = (r, key, fallback) => {
        const text = r.t(key, fallback);
        return text && text !== key ? text : fallback;
    };

    // The shared parameter kit translates through t() as well: hand it schemas whose labels are resolved already.
    function localSchema(r, st, def) {
        if (!st.schemas) st.schemas = {};
        if (!st.schemas[def.id]) {
            st.schemas[def.id] = def.params.map(p => {
                const copy = Object.assign({}, p, { labelKey: '', fallback: tx(r, p.labelKey, p.fallback) });
                if (p.options) copy.options = p.options.map(o => Object.assign({}, o, { labelKey: '', fallback: tx(r, o.labelKey, o.fallback) }));
                return copy;
            });
        }
        return st.schemas[def.id];
    }

    const q = (st, selector) => (st.el ? st.el.querySelector(selector) : null);
    const generatorName = (r, def) => tx(r, def.labelKey, def.fallback);

    // --- panel markup ---

    function chipsHTML(r, st) {
        const chip = (id, label) => `<button class="pixel-chip${st.cat === id ? ' active' : ''}" type="button" data-gen-cat="${r.esc(id)}">${r.esc(label)}</button>`;
        return chip('all', tx(r, 'pixel.cat_all', 'All')) + CATS.map(cat => chip(cat, tx(r, 'pixel.gcat_' + cat, CAT_NAMES[cat]))).join('');
    }

    function buildHTML(r) {
        const st = stateOf(r);
        const t = (key, fallback) => r.esc(tx(r, key, fallback));
        const blends = (r.BLEND_MODES || Object.keys(BLEND_NAMES)).map(mode =>
            `<option value="${r.esc(mode)}">${r.esc(tx(r, 'pixel.blend_' + keyPart(mode), BLEND_NAMES[mode] || mode))}</option>`).join('');
        const outputs = [['layer', 'pixel.gen_out_layer', 'New layer'], ['replace', 'pixel.gen_out_replace', 'Replace layer'], ['selection', 'pixel.gen_out_selection', 'Into selection']];
        return `<div class="pixel-gen" data-gen-root>
            <div class="pixel-gen-browse" data-gen-browse>
                <input type="search" class="pixel-gen-search" data-gen-search placeholder="${t('pixel.gen_search', 'Search generators')}" aria-label="${t('pixel.gen_search', 'Search generators')}" spellcheck="false" autocomplete="off">
                <div class="pixel-filter-chips" data-gen-cats>${chipsHTML(r, st)}</div>
                <div class="pixel-gen-list" data-gen-list></div>
            </div>
            <div class="pixel-gen-detail" data-gen-detail hidden>
                <div class="pixel-gen-head">
                    <button class="pixel-btn" type="button" data-gen-back title="${t('pixel.gen_back', 'All generators')}">&larr; ${t('pixel.gen_back', 'All generators')}</button>
                    <span class="pixel-gen-title" data-gen-title></span>
                </div>
                <div class="pixel-gen-preview-wrap">
                    <canvas class="pixel-gen-preview" data-gen-preview width="1" height="1"></canvas>
                    <div class="pixel-gen-spinner" data-gen-spinner hidden></div>
                </div>
                <div class="pixel-gen-presets" data-gen-presets></div>
                <div class="pixel-gen-params" data-gen-params></div>
                <div class="pixel-gen-output">
                    <span class="pixel-label">${t('pixel.gen_output', 'Output')}</span>
                    <div class="pixel-param-segmented" role="group" data-gen-outputs>${outputs.map(([id, key, fallback], i) =>
                        `<button type="button" class="pixel-param-seg-btn${i === 0 ? ' active' : ''}" data-gen-out="${id}" aria-pressed="${i === 0 ? 'true' : 'false'}">${t(key, fallback)}</button>`).join('')}</div>
                    <div class="pixel-gen-layer-opts" data-gen-layer-opts>
                        <select class="pixel-select" data-gen-blend aria-label="${t('pixel.gen_blend', 'Blend mode')}">${blends}</select>
                        <div class="pixel-slider-row"><span class="pixel-slider-label">${t('pixel.layer_opacity', 'Opacity')}</span><input type="range" class="pixel-slider" data-gen-opacity min="1" max="100" value="100"><span class="pixel-slider-value" data-gen-opacity-val>100%</span></div>
                    </div>
                    <div class="pixel-gen-size" data-gen-size hidden>
                        <label class="pixel-param-axis">${t('pixel.width', 'Width')}<input type="number" class="pixel-param-number" data-gen-width min="16" max="${MAX_DOC}" step="1" value="1024"></label>
                        <label class="pixel-param-axis">${t('pixel.height', 'Height')}<input type="number" class="pixel-param-number" data-gen-height min="16" max="${MAX_DOC}" step="1" value="1024"></label>
                    </div>
                    <label class="pixel-gen-switch"><span>${t('pixel.gen_live', 'Preview on canvas')}</span><span class="pixel-param-switch"><input type="checkbox" role="switch" data-gen-live><span class="pixel-param-switch-track"><span class="pixel-param-switch-thumb"></span></span></span></label>
                    <p class="pixel-draw-hint" data-gen-hint hidden></p>
                </div>
                <div class="pixel-gen-actions">
                    <button class="pixel-btn pixel-btn-primary" type="button" data-gen-go>${t('pixel.generate', 'Generate')}</button>
                    <button class="pixel-btn" type="button" data-gen-cancel hidden>${t('pixel.cancel', 'Cancel')}</button>
                </div>
                <div class="pixel-gen-progress" data-gen-progress hidden><span data-gen-progress-bar></span></div>
            </div>
        </div>`;
    }

    // --- list and thumbnails ---

    function visibleGenerators(r, st) {
        const query = st.query.trim().toLowerCase();
        return Pixel.GENERATORS.filter(def => {
            if (st.cat !== 'all' && def.cat !== st.cat) return false;
            if (!query) return true;
            return generatorName(r, def).toLowerCase().includes(query) || def.fallback.toLowerCase().includes(query) || def.id.includes(query);
        });
    }

    function paintThumb(canvas, img) {
        if (!canvas || !img || typeof ImageData !== 'function') return;
        canvas.width = img.width;
        canvas.height = img.height;
        canvas.getContext('2d').putImageData(new ImageData(img.data, img.width, img.height), 0, 0);
    }

    function renderList(r, st) {
        const list = q(st, '[data-gen-list]');
        if (!list) return;
        const items = visibleGenerators(r, st);
        list.innerHTML = items.length ? items.map(def =>
            `<button type="button" class="pixel-gen-card${def.id === st.id ? ' active' : ''}" data-gen-id="${r.esc(def.id)}" title="${r.esc(generatorName(r, def))}"><canvas class="pixel-gen-thumb" width="${THUMB}" height="${THUMB}" data-gen-thumb="${r.esc(def.id)}"></canvas><span class="pixel-gen-name">${r.esc(generatorName(r, def))}</span></button>`).join('')
            : `<p class="pixel-draw-hint">${r.esc(tx(r, 'pixel.gen_empty', 'No generator matches.'))}</p>`;
        queueThumbs(r, st, list);
    }

    // One job at a time, in list order; a newer list replaces the running chain (token) and its job (same key).
    function queueThumbs(r, st, list) {
        const token = ++st.thumbToken;
        const todo = [];
        Array.from(list.querySelectorAll('[data-gen-thumb]')).forEach(canvas => {
            const cached = thumbCache.get(canvas.dataset.genThumb);
            if (cached) paintThumb(canvas, cached); else todo.push(canvas);
        });
        const next = () => {
            if (token !== st.thumbToken || !todo.length) return;
            const canvas = todo.shift(), id = canvas.dataset.genThumb;
            const params = lib() ? lib().generatorDefaults(id) : {};
            r.runJob('gen.render', { id, params, width: THUMB, height: THUMB, draft: true }, { key: 'gen-thumb:' + st.thumbKeySuffix })
                .then(res => {
                    const img = res && (res.image || res);
                    if (img && img.data) { thumbCache.set(id, img); paintThumb(canvas, img); }
                    next();
                }, err => { if (!isAbort(err) || token === st.thumbToken) next(); });
        };
        next();
    }

    // --- detail view ---

    function resolveGradient(value) {
        if (typeof value === 'string') {
            const preset = lib() && lib().gradientPreset(value);
            return preset ? lib().cloneStops(preset.stops) : value;
        }
        return clone(value);
    }

    function selectGenerator(r, st, id) {
        const def = Pixel.generatorById(id);
        if (!def || !st.el) return;
        if (st.id && st.values) st.memory[st.id] = st.values;
        clearLive(r, st);
        st.id = id;
        st.values = st.memory[id] || r.defaultParams(def.params);
        const browse = q(st, '[data-gen-browse]'), detail = q(st, '[data-gen-detail]');
        if (browse) browse.hidden = true;
        if (detail) detail.hidden = false;
        const title = q(st, '[data-gen-title]');
        if (title) title.textContent = generatorName(r, def);
        renderPresets(r, st, def);
        const params = q(st, '[data-gen-params]');
        if (params) {
            const schema = localSchema(r, st, def);
            params.innerHTML = r.buildParamControls(schema, st.values, { idPrefix: 'pixel-gen-' + keyPart(id) });
            st.wire = r.wireParamControls(params, schema, st.values, () => { schedulePreview(r, st); scheduleLive(r, st); });
        }
        syncOutputUI(r, st);
        schedulePreview(r, st, 0);
        scheduleLive(r, st);
    }

    function showBrowse(r, st) {
        if (st.id && st.values) st.memory[st.id] = st.values;
        clearLive(r, st);
        st.id = '';
        const browse = q(st, '[data-gen-browse]'), detail = q(st, '[data-gen-detail]');
        if (browse) browse.hidden = false;
        if (detail) detail.hidden = true;
        renderList(r, st);
    }

    function renderPresets(r, st, def) {
        const box = q(st, '[data-gen-presets]');
        if (!box) return;
        const chips = [`<button class="pixel-chip" type="button" data-gen-preset="">${r.esc(tx(r, 'pixel.gen_preset_default', 'Default'))}</button>`]
            .concat(def.presets.map(preset => `<button class="pixel-chip" type="button" data-gen-preset="${r.esc(preset.id)}">${r.esc(tx(r, preset.labelKey, preset.fallback))}</button>`));
        box.innerHTML = `<span class="pixel-label">${r.esc(tx(r, 'pixel.gen_presets', 'Presets'))}</span><div class="pixel-filter-chips">${chips.join('')}</div>`;
    }

    // Applies a preset on top of the defaults; the seed stays, so a lucky seed survives switching presets.
    function applyPreset(r, st, presetId) {
        const def = Pixel.generatorById(st.id);
        if (!def || !st.values) return;
        const preset = def.presets.find(item => item.id === presetId) || null;
        const base = r.defaultParams(def.params);
        def.params.forEach(p => {
            if (p.type === 'seed') return;
            const raw = preset && preset.values[p.key] !== undefined ? preset.values[p.key] : base[p.key];
            st.values[p.key] = p.type === 'gradient' ? resolveGradient(raw) : clone(raw);
        });
        if (st.wire) st.wire.refresh();
        schedulePreview(r, st, 0);
        scheduleLive(r, st);
    }

    // --- document geometry ---

    const hasDocument = r => r.imgWidth > 0 && r.imgHeight > 0;

    function referenceSize(r, st) {
        return hasDocument(r) ? { w: r.imgWidth, h: r.imgHeight } : { w: clampSize(st.docW), h: clampSize(st.docH) };
    }

    // --- panel preview (<= 300 px, same look as the full render: the reference size carries the aspect) ---

    function setSpinner(st, on) {
        const spinner = q(st, '[data-gen-spinner]');
        if (spinner) spinner.hidden = !on;
    }

    function schedulePreview(r, st, delay) {
        clearTimeout(st.previewTimer);
        st.previewTimer = setTimeout(() => runPreview(r, st), delay == null ? 160 : delay);
    }

    // Nothing is rendered while another panel is in front; onShow brings the preview back.
    const panelVisible = r => !!(r.activePanels && r.activePanels.inspector === 'generate');

    function runPreview(r, st) {
        const def = Pixel.generatorById(st.id);
        const canvas = q(st, '[data-gen-preview]');
        if (!def || !canvas || !st.values || !panelVisible(r)) return;
        const ref = referenceSize(r, st), token = ++st.previewToken;
        const k = PREVIEW_EDGE / Math.max(ref.w, ref.h);
        const w = Math.max(8, Math.round(ref.w * k)), h = Math.max(8, Math.round(ref.h * k));
        setSpinner(st, true);
        r.runJob('gen.render', { id: def.id, params: clone(st.values), width: w, height: h, refWidth: ref.w, refHeight: ref.h }, { key: 'gen-preview:' + st.thumbKeySuffix })
            .then(res => {
                if (token !== st.previewToken) return;
                setSpinner(st, false);
                const img = res && (res.image || res);
                if (img && img.data && !res.cancelled) paintThumb(canvas, img);
            }, err => {
                if (isAbort(err)) return;
                if (token === st.previewToken) setSpinner(st, false);
                if (window.console) console.error('pixel generator preview', err);
            });
    }

    // --- live preview on the canvas (transient layer, or the preview of the target layer) ---

    function liveEnabled(r, st) {
        return !!(st.live && st.id && st.values && hasDocument(r));
    }

    function scheduleLive(r, st) {
        clearTimeout(st.liveTimer);
        if (!liveEnabled(r, st)) return;
        st.liveTimer = setTimeout(() => runLive(r, st), 320);
    }

    function runLive(r, st) {
        if (!liveEnabled(r, st)) return;
        const def = Pixel.generatorById(st.id), token = ++st.liveToken, w = r.imgWidth, h = r.imgHeight;
        setSpinner(st, true);
        r.runJob('gen.render', { id: def.id, params: clone(st.values), width: w, height: h }, { key: 'gen-live:' + st.thumbKeySuffix })
            .then(res => {
                if (token !== st.liveToken) return;
                setSpinner(st, false);
                const img = res && (res.image || res);
                if (img && img.data && !res.cancelled) showLive(r, st, img);
            }, err => {
                if (isAbort(err)) return;
                if (token === st.liveToken) setSpinner(st, false);
                if (window.console) console.error('pixel generator live preview', err);
            });
    }

    function ensureTransient(r, st, def) {
        if (st.transient && r.layers.includes(st.transient)) return st.transient;
        if (r.layers.length >= r.MAX_LAYERS) return null;
        const layer = r.createLayer({ name: generatorName(r, def), opacity: st.opacity / 100, blend: st.blend });
        layer.transient = true;
        r.layers.splice(r.activeLayerIdx + 1, 0, layer);
        st.transient = layer;
        r.layersStructureChanged();
        return layer;
    }

    function removeTransient(r, st) {
        const layer = st.transient;
        if (!layer) return;
        st.transient = null;
        const index = r.layers.indexOf(layer);
        if (index < 0) return;
        const keep = r.activeLayer();
        r.layers.splice(index, 1);
        r.activeLayerIdx = keep === layer ? Math.max(0, Math.min(index - 1, r.layers.length - 1)) : Math.max(0, r.layers.indexOf(keep));
        r.layersStructureChanged();
    }

    function dropTargetPreview(r, st) {
        const layer = st.previewLayer;
        st.previewLayer = null;
        if (layer && layer.preview && (layer.preview === st.liveCanvas || layer.preview === st.composeCanvas)) r.setLayerPreview(layer, null);
    }

    function clearLive(r, st) {
        st.liveToken++;
        clearTimeout(st.liveTimer);
        dropTargetPreview(r, st);
        removeTransient(r, st);
        setSpinner(st, false);
    }

    function putImage(r, canvasRef, img) {
        let canvas = canvasRef;
        if (!canvas || canvas.width !== img.width || canvas.height !== img.height) canvas = r.makeCanvas(img.width, img.height);
        canvas.getContext('2d').putImageData(new ImageData(img.data, img.width, img.height), 0, 0);
        return canvas;
    }

    // The generated pixels as they would land on the layer when the output is "into selection".
    function maskedImage(r, img) {
        const mask = r.getSelectionMask();
        if (!mask) return img;
        const data = new Uint8ClampedArray(img.data), w = img.width;
        for (let y = mask.y; y < mask.y + mask.h; y++) {
            for (let p = y * w + mask.x, end = p + mask.w; p < end; p++) data[p * 4 + 3] = img.data[p * 4 + 3] * mask.alpha[p] / 255;
        }
        for (let y = 0; y < img.height; y++) {
            if (y >= mask.y && y < mask.y + mask.h) {
                const row = y * w;
                for (let x = 0; x < mask.x; x++) data[(row + x) * 4 + 3] = 0;
                for (let x = mask.x + mask.w; x < w; x++) data[(row + x) * 4 + 3] = 0;
            } else {
                for (let x = 0; x < w; x++) data[(y * w + x) * 4 + 3] = 0;
            }
        }
        return { width: img.width, height: img.height, data };
    }

    function showLive(r, st, img) {
        const def = Pixel.generatorById(st.id);
        if (!def || !liveEnabled(r, st) || img.width !== r.imgWidth || img.height !== r.imgHeight) return;
        if (st.output === 'layer') {
            dropTargetPreview(r, st);
            const layer = ensureTransient(r, st, def);
            if (!layer) return;
            layer.opacity = st.opacity / 100;
            layer.blend = st.blend;
            st.liveCanvas = putImage(r, st.liveCanvas, img);
            r.setLayerPreview(layer, st.liveCanvas);
            return;
        }
        removeTransient(r, st);
        const target = r.activeLayer();
        if (!target || target.sticker || target.lockPixels) return;
        if (st.previewLayer && st.previewLayer !== target) dropTargetPreview(r, st);
        st.previewLayer = target;
        if (st.output === 'replace' || !r.getSelectionMask()) {
            st.liveCanvas = putImage(r, st.liveCanvas, img);
            r.setLayerPreview(target, st.liveCanvas);
            return;
        }
        const generated = r.makeCanvas(img.width, img.height);
        generated.getContext('2d').putImageData(new ImageData(maskedImage(r, img).data, img.width, img.height), 0, 0);
        if (!st.composeCanvas || st.composeCanvas.width !== img.width || st.composeCanvas.height !== img.height) st.composeCanvas = r.makeCanvas(img.width, img.height);
        const cctx = st.composeCanvas.getContext('2d');
        cctx.clearRect(0, 0, img.width, img.height);
        cctx.drawImage(target.canvas, 0, 0);
        cctx.drawImage(generated, 0, 0);
        r.setLayerPreview(target, st.composeCanvas);
    }

    function setLive(r, st, on) {
        st.live = !!on;
        const box = q(st, '[data-gen-live]');
        if (box) box.checked = st.live;
        if (!st.live) { clearLive(r, st); return; }
        scheduleLive(r, st);
    }

    // The transient layer can disappear without us (undo, open, new image): then the toggle falls back to off.
    function syncLive(r, st) {
        if (!st.live) return;
        const gone = st.transient && !r.layers.includes(st.transient);
        if (gone || !hasDocument(r)) {
            st.transient = null;
            setLive(r, st, false);
        }
    }

    // History entries pushed while a preview layer exists must not remember it.
    function stripTransientFromHistory(r, st) {
        if (!st.transient || !r.layers.includes(st.transient)) return;
        const entry = r.history[r.historyIdx];
        if (entry) entry.layers = entry.layers.filter(item => item.meta.id !== st.transient.id);
    }

    // --- output controls ---

    function syncOutputUI(r, st) {
        const box = q(st, '[data-gen-outputs]');
        if (box) {
            Array.from(box.querySelectorAll('[data-gen-out]')).forEach(btn => {
                const on = btn.dataset.genOut === st.output;
                btn.classList.toggle('active', on);
                btn.setAttribute('aria-pressed', on ? 'true' : 'false');
            });
        }
        const layerOpts = q(st, '[data-gen-layer-opts]');
        if (layerOpts) layerOpts.hidden = st.output !== 'layer';
        const size = q(st, '[data-gen-size]');
        if (size) size.hidden = hasDocument(r);
        const hint = q(st, '[data-gen-hint]');
        if (hint) {
            const noSelection = hasDocument(r) && st.output === 'selection' && !r.selection;
            hint.hidden = !noSelection;
            hint.textContent = noSelection ? tx(r, 'pixel.gen_no_selection', 'No selection: the whole layer is filled.') : '';
        }
    }

    function setOutput(r, st, mode) {
        st.output = mode;
        syncOutputUI(r, st);
        if (st.live) { clearLive(r, st); scheduleLive(r, st); }
    }

    function setProgress(st, fraction) {
        const bar = q(st, '[data-gen-progress-bar]'), wrap = q(st, '[data-gen-progress]');
        if (wrap) wrap.hidden = false;
        if (bar) bar.style.width = Math.round(Math.max(0, Math.min(1, fraction)) * 100) + '%';
    }

    function setWorking(st, on) {
        st.working = on;
        const go = q(st, '[data-gen-go]'), cancel = q(st, '[data-gen-cancel]'), wrap = q(st, '[data-gen-progress]');
        if (go) go.disabled = on;
        if (cancel) cancel.hidden = !on;
        if (wrap) wrap.hidden = !on;
        if (on) setProgress(st, 0);
    }

    // Generated pixels become a new layer above the active one (history label generate:<id>).
    function addGeneratedLayer(r, st, def, canvas) {
        if (r.layers.length >= r.MAX_LAYERS) { r.notify({ type: 'error', message: r.t('pixel.layer_max', 'Maximum number of layers reached ({n}).', { n: r.MAX_LAYERS }) }); return false; }
        r.commitPendingPreviews();
        const layer = r.createLayer({ name: generatorName(r, def), opacity: st.opacity / 100, blend: st.blend });
        layer.canvas.getContext('2d').drawImage(canvas, 0, 0);
        r.layers.splice(r.activeLayerIdx + 1, 0, layer);
        r.activeLayerIdx = r.layers.indexOf(layer);
        r.editMaskMode = false;
        r.layersStructureChanged();
        r.pushHistory('generate:' + def.id);
        return true;
    }

    function commitGenerated(r, st, def, canvas) {
        if (!hasDocument(r)) return false;
        if (st.output === 'layer') return addGeneratedLayer(r, st, def, canvas);
        r.commitPendingPreviews();
        if (r.editMaskMode) r.leaveMaskEdit();
        const layer = r.activeLayer();
        if (!layer || !r.requireRasterLayer(layer)) return false;
        return r.commitLayerPixels(layer, canvas, { label: 'generate:' + def.id, respectSelection: st.output === 'selection' });
    }

    async function generate(r, st) {
        const def = Pixel.generatorById(st.id);
        if (!def || st.working || !st.values) return;
        const existing = hasDocument(r);
        const ref = referenceSize(r, st);
        if (st.live) setLive(r, st, false); else clearLive(r, st);
        const controller = new AbortController();
        st.controller = controller;
        setWorking(st, true);
        try {
            const res = await r.runJob('gen.render', { id: def.id, params: clone(st.values), width: ref.w, height: ref.h },
                { key: 'gen-full:' + st.thumbKeySuffix, signal: controller.signal, onProgress: fraction => setProgress(st, fraction) });
            const img = res && (res.image || res);
            if (!img || !img.data || res.cancelled) return;
            if (existing && (r.imgWidth !== ref.w || r.imgHeight !== ref.h)) {
                r.notify({ type: 'error', message: tx(r, 'pixel.gen_failed', 'Generator failed') });
                return;
            }
            const canvas = r.canvasFromImage(img);
            if (existing) {
                commitGenerated(r, st, def, canvas);
            } else {
                // No document yet: the generated image becomes the background of a new one.
                if (r.state.imageRead) r.state.imageRead.abort();
                r.filePath = '';
                r.state.fileVersion = '';
                r.state.versionPath = '';
                r.newBlankCanvas(ref.w, ref.h);
                r.commitLayerPixels(r.activeLayer(), canvas, { label: 'generate:' + def.id, respectSelection: false });
                r.zoomFit();
            }
        } catch (err) {
            if (!isAbort(err)) {
                if (window.console) console.error('pixel generator', err);
                r.notify({ type: 'error', message: tx(r, 'pixel.gen_failed', 'Generator failed') + (err && err.message ? ': ' + err.message : '') });
            }
        } finally {
            st.controller = null;
            setWorking(st, false);
            syncOutputUI(r, st);
        }
    }

    function cancelGenerate(st) {
        if (st.controller) st.controller.abort();
    }

    // --- wiring ---

    function mount(r, el) {
        const st = stateOf(r);
        st.el = el;
        st.thumbKeySuffix = String(r.windowId);
        renderList(r, st);

        el.addEventListener('click', e => {
            const target = e.target;
            if (!target || !target.closest) return;
            const cat = target.closest('[data-gen-cat]');
            if (cat) {
                st.cat = cat.dataset.genCat;
                const chips = q(st, '[data-gen-cats]');
                if (chips) chips.innerHTML = chipsHTML(r, st);
                renderList(r, st);
                return;
            }
            const card = target.closest('[data-gen-id]');
            if (card) { selectGenerator(r, st, card.dataset.genId); return; }
            if (target.closest('[data-gen-back]')) { showBrowse(r, st); return; }
            const preset = target.closest('[data-gen-preset]');
            if (preset) { applyPreset(r, st, preset.dataset.genPreset); return; }
            const out = target.closest('[data-gen-out]');
            if (out) { setOutput(r, st, out.dataset.genOut); return; }
            if (target.closest('[data-gen-go]')) { generate(r, st); return; }
            if (target.closest('[data-gen-cancel]')) cancelGenerate(st);
        });
        el.addEventListener('input', e => {
            const target = e.target;
            if (!target || !target.matches) return;
            if (target.matches('[data-gen-search]')) {
                st.query = target.value || '';
                renderList(r, st);
            } else if (target.matches('[data-gen-opacity]')) {
                st.opacity = Number(target.value) || 100;
                const label = q(st, '[data-gen-opacity-val]');
                if (label) label.textContent = st.opacity + '%';
                if (st.transient) { st.transient.opacity = st.opacity / 100; r.compositeLayers(); r.notifyLayersChanged(); }
            }
        });
        el.addEventListener('change', e => {
            const target = e.target;
            if (!target || !target.matches) return;
            if (target.matches('[data-gen-blend]')) {
                st.blend = target.value || 'normal';
                if (st.transient) { st.transient.blend = st.blend; r.compositeLayers(); r.notifyLayersChanged(); }
            } else if (target.matches('[data-gen-live]')) {
                setLive(r, st, !!target.checked);
            } else if (target.matches('[data-gen-width]')) {
                st.docW = clampSize(target.value);
                target.value = String(st.docW);
                schedulePreview(r, st, 0);
            } else if (target.matches('[data-gen-height]')) {
                st.docH = clampSize(target.value);
                target.value = String(st.docH);
                schedulePreview(r, st, 0);
            }
        });

        r.on('layers-changed', () => syncLive(r, st));
        r.on('document-changed', () => { syncLive(r, st); syncOutputUI(r, st); if (st.id) schedulePreview(r, st, 0); });
        r.on('history-changed', () => { stripTransientFromHistory(r, st); syncLive(r, st); });
        r.on('selection-changed', () => { syncOutputUI(r, st); if (st.live && st.output === 'selection') scheduleLive(r, st); });
        r.on('tool-changed', () => { if (st.live) setLive(r, st, false); });
        r.on('active-layer-changed', () => { if (st.live && st.output !== 'layer') { clearLive(r, st); scheduleLive(r, st); } });
    }

    Object.assign(Pixel, { generatorCategories: CATS });

    // --- registration ---

    Pixel.registerModule('generators', function install(r) {
        Object.assign(r, {
            // Opens the Generate panel on one generator (menu items, commands and other modules use this).
            openGenerator: Pixel.bindRuntime(r, function openGenerator(id) {
                const st = stateOf(this);
                this.showPanel('generate');
                if (st.el && Pixel.generatorById(id)) selectGenerator(this, st, id);
            }),
            // Renders a generator in the worker and resolves with { canvas, image } (image: { width, height, data } RGBA). params override the defaults; options: { width,
            // height (default: the document), refWidth, refHeight, draft, signal, onProgress }. Used by other modules that
            // want a generated image (backgrounds, textures) without the panel.
            renderGenerator: Pixel.bindRuntime(r, function renderGenerator(id, params, options) {
                const def = Pixel.generatorById(id);
                if (!def) return Promise.reject(new Error('unknown generator ' + id));
                const o = options || {};
                const width = o.width || this.imgWidth || 1024, height = o.height || this.imgHeight || 1024;
                const values = Object.assign(this.defaultParams(def.params), params || {});
                return this.runJob('gen.render', { id, params: values, width, height, refWidth: o.refWidth, refHeight: o.refHeight, draft: !!o.draft },
                    { signal: o.signal, onProgress: o.onProgress }).then(res => {
                    const img = res && (res.image || res);
                    if (!img || !img.data) throw new Error('generator returned no image');
                    return { canvas: this.canvasFromImage(img), image: img };
                });
            }),
            // The Generate button: renders at the document size (or the size fields) and writes the chosen output.
            generateNow: Pixel.bindRuntime(r, function generateNow() {
                return generate(this, stateOf(this));
            }),
            setGeneratorLive: Pixel.bindRuntime(r, function setGeneratorLive(on) {
                setLive(this, stateOf(this), on);
            }),
            // Panel state (selected generator, values, output mode); read-only use, for tests and other modules.
            generatorState: Pixel.bindRuntime(r, function generatorState() {
                return stateOf(this);
            })
        });
    });

    Pixel.registerPanel({
        id: 'generate', labelKey: 'pixel.generate', fallback: 'Generate', order: 50, dock: 'inspector',
        build: r => buildHTML(r),
        mount: (r, el) => mount(r, el),
        onShow: r => {
            const st = stateOf(r);
            syncOutputUI(r, st);
            if (st.id) { schedulePreview(r, st, 0); scheduleLive(r, st); }
        },
        onHide: r => {
            const st = stateOf(r);
            if (st.live) setLive(r, st, false);
            else clearLive(r, st);
        }
    });

    if (Pixel.GENERATORS.length) {
        Pixel.registerMenuItems('generate', CATS.map(cat => ({
            id: 'cat-' + cat, labelKey: 'pixel.gcat_' + cat, label: CAT_NAMES[cat], fallback: CAT_NAMES[cat], icon: 'image',
            items: Pixel.GENERATORS.filter(def => def.cat === cat).map(def => ({
                id: 'gen-' + def.id, labelKey: def.labelKey, label: def.fallback, fallback: def.fallback,
                action: r => r.openGenerator(def.id)
            }))
        })).concat([{ type: 'separator' }, {
            id: 'panel', labelKey: 'pixel.gen_open_panel', label: 'Generator panel', fallback: 'Generator panel', icon: 'image', action: r => r.showPanel('generate')
        }]), { order: 0 });

        Pixel.GENERATORS.forEach(def => {
            Pixel.registerCommand({
                id: 'generate:' + def.id, labelKey: 'pixel.cmd_gen_' + keyPart(def.id), fallback: 'Generate: ' + def.fallback,
                keywords: ['generate', def.cat, def.id, def.fallback], group: 'generate',
                run: r => r.openGenerator(def.id)
            });
        });
    }
})();
