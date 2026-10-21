(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const DEFAULT_DOCK_SPLIT = 0.45;
    const DOCK_SPLIT_KEY = 'pixel.dock_split';
    // One wiring per root element: wiring the same root again replaces the earlier listeners.
    const paramWires = new WeakMap();

    // --- parameter controls (shared by filters, generators, cutout and tool options) ---

    const hexColor = /^#(?:[0-9a-f]{3}|[0-9a-f]{6})$/i;

    function clampNumber(v, lo, hi) {
        return v < lo ? lo : (v > hi ? hi : v);
    }

    function decimalsOf(step) {
        const text = String(step);
        return text.includes('.') ? text.split('.')[1].length : 0;
    }

    function cloneValue(value) {
        return value && typeof value === 'object' ? JSON.parse(JSON.stringify(value)) : value;
    }

    function normalizeHex(text) {
        let v = String(text == null ? '' : text).trim().toLowerCase();
        if (!v.startsWith('#')) v = '#' + v;
        if (!hexColor.test(v)) return null;
        if (v.length === 4) v = '#' + v[1] + v[1] + v[2] + v[2] + v[3] + v[3];
        return v;
    }

    function paramRange(def) {
        const min = def.min != null ? Number(def.min) : 0;
        const max = def.max != null ? Number(def.max) : 100;
        const step = def.step != null ? Number(def.step) : (Number.isInteger(min) && Number.isInteger(max) ? 1 : 0.01);
        return { min, max, step };
    }

    function gradientCss(stops) {
        const list = (stops || []).slice().sort((a, b) => a.pos - b.pos);
        if (!list.length) return 'linear-gradient(90deg,#000,#fff)';
        return 'linear-gradient(90deg,' + list.map(s => (normalizeHex(s.color) || '#000000') + ' ' + Math.round(clampNumber(Number(s.pos) || 0, 0, 1) * 1000) / 10 + '%').join(',') + ')';
    }

    function sameStops(a, b) {
        return a.length === b.length && a.every((s, i) => s.color.toLowerCase() === b[i].color.toLowerCase() && Math.abs(s.pos - b[i].pos) < 0.0005);
    }

    function presetStops(id) {
        const lib = window.AuraPixelLib;
        const preset = lib && lib.gradientPreset ? lib.gradientPreset(id) : null;
        return preset ? lib.cloneStops(preset.stops) : null;
    }

    // Default value of one schema entry; objects and arrays are copies, so mutating values never touches the schema.
    function paramDefault(def) {
        if (def.default !== undefined) {
            if (def.type === 'gradient' && typeof def.default === 'string') return presetStops(def.default) || [{ pos: 0, color: '#000000' }, { pos: 1, color: '#ffffff' }];
            return cloneValue(def.default);
        }
        switch (def.type) {
            case 'select': case 'segmented': return def.options && def.options.length ? def.options[0].value : '';
            case 'bool': return false;
            case 'color': return '#000000';
            case 'angle': return 0;
            case 'seed': return 1;
            case 'point': return { x: 0.5, y: 0.5 };
            case 'gradient': return presetStops('grayscale') || [{ pos: 0, color: '#000000' }, { pos: 1, color: '#ffffff' }];
            case 'custom': return undefined;
            default: return paramRange(def).min;
        }
    }

    function stopRowsHTML(r, stops) {
        return stops.map((stop, i) =>
            `<div class="pixel-param-stop" data-param-stop="${i}"><input type="color" class="pixel-param-stop-color" data-stop-color value="${normalizeHex(stop.color) || '#000000'}" title="${r.esc(r.t('pixel.param_stop_color', 'Stop color'))}"><input type="number" class="pixel-param-number" data-stop-pos min="0" max="100" step="1" value="${Math.round(clampNumber(Number(stop.pos) || 0, 0, 1) * 100)}" title="${r.esc(r.t('pixel.param_stop_position', 'Stop position (%)'))}"><span class="pixel-param-unit">%</span><button type="button" class="pixel-param-mini" data-stop-remove title="${r.esc(r.t('pixel.param_remove_stop', 'Remove stop'))}">×</button></div>`
        ).join('');
    }

    const DICE_SVG = '<svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true"><rect x="2" y="2" width="12" height="12" rx="2.5" fill="none" stroke="currentColor" stroke-width="1.4"/><circle cx="5.5" cy="5.5" r="1.1" fill="currentColor"/><circle cx="10.5" cy="5.5" r="1.1" fill="currentColor"/><circle cx="8" cy="8" r="1.1" fill="currentColor"/><circle cx="5.5" cy="10.5" r="1.1" fill="currentColor"/><circle cx="10.5" cy="10.5" r="1.1" fill="currentColor"/></svg>';

    Pixel.paramDefault = paramDefault;
    Pixel.paramVisible = function paramVisible(def, values) {
        if (typeof def.showIf !== 'function') return true;
        try { return !!def.showIf(values); } catch (_) { return true; }
    };

    Pixel.installView = function installView(runtime) {
        // Number of decimals shown for a schema entry.
        const readout = (def, value) => {
            const { step } = paramRange(def);
            const n = Number(value);
            return Number.isFinite(n) ? n.toFixed(decimalsOf(step)) : '';
        };

        Object.assign(runtime, {
            activePanels: {},
            dockSplit: DEFAULT_DOCK_SPLIT,
            toolSvg: Pixel.bindRuntime(runtime, function toolSvg(name) {
                const def = Pixel.getTool(name);
                return (def && def.svg) || this.TOOL_SVGS[name] || '';
            }),
            toolLabel: Pixel.bindRuntime(runtime, function toolLabel(id) {
                const def = Pixel.getTool(id);
                if (def && def.labelKey) return this.t(def.labelKey, def.fallback || id);
                return this.t('pixel.' + String(id).replace(/-/g, '_'), (def && def.fallback) || id);
            }),
            panelLabel: Pixel.bindRuntime(runtime, function panelLabel(def) {
                return this.t(def.labelKey || ('pixel.' + def.id), def.fallback || def.id);
            }),
            // Tool rail: one group per registry group (select, paint, retouch, fill, shape, type, view), empty groups skipped.
            buildToolRailHTML: Pixel.bindRuntime(runtime, function buildToolRailHTML() {
                const groups = Pixel.toolGroups().map(({ tools }) =>
                    `<div class="pixel-rail-group">${tools.map(def => {
                        const shortcut = def.shortcut || Pixel.TOOL_SHORTCUTS[def.id];
                        return `<button class="pixel-tool-btn${this.activeTool === def.id ? ' active' : ''}" type="button" data-draw-tool="${this.esc(def.id)}" title="${this.esc(this.toolLabel(def.id))}${shortcut ? ' (' + this.esc(shortcut) + ')' : ''}">${this.toolSvg(def.id)}</button>`;
                    }).join('')}</div>`
                ).join('<div class="pixel-rail-sep"></div>');
                return `<div class="pixel-tool-rail" data-tool-rail>
                    ${groups}
                    <div class="pixel-rail-spacer"></div>
                    <div class="pixel-rail-colors">
                        <div class="pixel-color-swatch-wrap">
                            <div class="pixel-color-swatch pixel-color-primary" data-color-primary style="background:${this.primaryColor}" title="${this.esc(this.t('pixel.color_primary'))}"></div>
                            <div class="pixel-color-swatch pixel-color-secondary" data-color-secondary style="background:${this.secondaryColor}" title="${this.esc(this.t('pixel.color_secondary'))}"></div>
                        </div>
                        <button class="pixel-color-swap" type="button" data-action="swap-colors" title="${this.esc(this.t('pixel.swap_colors'))}">⇄</button>
                    </div>
                </div>`;
            }),
            // Options of the built-in tools. A registered tool with options(r) replaces this for itself.
            buildDrawOptionsHTML: Pixel.bindRuntime(runtime, function buildDrawOptionsHTML() {
                let html = '';
                const isBrushType = this.activeTool === 'brush' || this.activeTool === 'eraser' || this.activeTool === 'pencil';
                const isShapeType = this.activeTool === 'line' || this.activeTool === 'rectangle' || this.activeTool === 'ellipse' || this.activeTool === 'arrow';
                const isTextType = this.activeTool === 'text';
                const isFillType = this.activeTool === 'fill' || this.activeTool === 'magic-wand';
                const isSelectType = this.activeTool === 'select-rect' || this.activeTool === 'select-ellipse';
                const isCloneType = this.activeTool === 'clone-stamp';

                if (isBrushType || this.activeTool === 'airbrush' || this.activeTool === 'dodge-burn' || this.activeTool === 'blur-brush' || isCloneType) {
                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.brush_size'))}</span><input type="range" class="pixel-slider" data-brush-size min="1" max="200" value="${this.brushSize}"><span class="pixel-slider-value" data-brush-size-val>${this.brushSize}px</span></div>`;
                    if (this.activeTool !== 'pencil') {
                        html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.brush_opacity'))}</span><input type="range" class="pixel-slider" data-brush-opacity min="1" max="100" value="${this.brushOpacity}"><span class="pixel-slider-value" data-brush-opacity-val>${this.brushOpacity}%</span></div>`;
                        html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.brush_hardness'))}</span><input type="range" class="pixel-slider" data-brush-hardness min="0" max="100" value="${this.brushHardness}"><span class="pixel-slider-value" data-brush-hardness-val>${this.brushHardness}%</span></div>`;
                    }
                    if (this.activeTool === 'airbrush') {
                        html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.spray_density'))}</span><input type="range" class="pixel-slider" data-spray-density min="5" max="100" value="${this.sprayDensity}"><span class="pixel-slider-value" data-spray-density-val>${this.sprayDensity}</span></div>`;
                    }
                    if (this.activeTool === 'dodge-burn') {
                        html += `<label class="pixel-label">${this.esc(this.t('pixel.mode'))}</label><select class="pixel-select" data-dodge-mode><option value="dodge"${this.dodgeMode === 'dodge' ? ' selected' : ''}>${this.esc(this.t('pixel.dodge'))}</option><option value="burn"${this.dodgeMode === 'burn' ? ' selected' : ''}>${this.esc(this.t('pixel.burn'))}</option></select>`;
                    }
                    if (isCloneType) {
                        html += `<p class="pixel-draw-hint">${this.esc(this.t('pixel.clone_stamp_hint'))}</p>`;
                    }
                } else if (isShapeType) {
                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.stroke_width'))}</span><input type="range" class="pixel-slider" data-shape-stroke min="1" max="20" value="${this.shapeStrokeWidth}"><span class="pixel-slider-value" data-shape-stroke-val>${this.shapeStrokeWidth}px</span></div>`;
                    if (this.activeTool === 'rectangle' || this.activeTool === 'ellipse') {
                        html += `<label class="pixel-checkbox-row"><input type="checkbox" data-shape-fill ${this.shapeFill ? 'checked' : ''}> ${this.esc(this.t('pixel.fill_mode'))}</label>`;
                    }
                } else if (isTextType) {
                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.font_size'))}</span><input type="range" class="pixel-slider" data-font-size min="8" max="200" value="${this.fontSize}"><span class="pixel-slider-value" data-font-size-val>${this.fontSize}px</span></div>`;
                    html += `<label class="pixel-label">${this.esc(this.t('pixel.font_family'))}</label><select class="pixel-select" data-font-family>${this.FONT_FAMILIES.map(f => `<option value="${f}" ${f === this.fontFamily ? 'selected' : ''}>${f}</option>`).join('')}</select>`;
                } else if (isFillType) {
                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.tolerance'))}</span><input type="range" class="pixel-slider" data-fill-tolerance min="0" max="128" value="${this.fillTolerance}"><span class="pixel-slider-value" data-fill-tolerance-val>${this.fillTolerance}</span></div>`;
                } else if (this.activeTool === 'gradient') {
                    html += `<label class="pixel-label">${this.esc(this.t('pixel.gradient_mode'))}</label><select class="pixel-select" data-gradient-mode><option value="linear"${this.gradientMode === 'linear' ? ' selected' : ''}>${this.esc(this.t('pixel.gradient_linear'))}</option><option value="radial"${this.gradientMode === 'radial' ? ' selected' : ''}>${this.esc(this.t('pixel.gradient_radial'))}</option></select>`;
                }

                if (isSelectType || this.activeTool === 'magic-wand' || this.activeTool === 'lasso') {
                    html += `<button class="pixel-btn" type="button" data-action="clear-selection">${this.esc(this.t('pixel.clear_selection'))}</button>`;
                }
                if (this.activeTool === 'move') {
                    html += `<p class="pixel-draw-hint">${this.esc(this.t('pixel.move_hint'))}</p>`;
                }

                if (!html) {
                    html = `<p class="pixel-draw-hint">${this.esc(this.t('pixel.select_tool_hint'))}</p>`;
                }
                return html;
            }),

            // --- panel bodies (inner html: the dock wraps each in <div class="pixel-panel-section" data-section="id">) ---
            buildAdjustPanelHTML: Pixel.bindRuntime(runtime, function buildAdjustPanelHTML() {
                return `${this.buildSlider('brightness', this.t('pixel.brightness'), -100, 100, 0)}
                    ${this.buildSlider('contrast', this.t('pixel.contrast'), -100, 100, 0)}
                    ${this.buildSlider('saturation', this.t('pixel.saturation'), -100, 100, 0)}
                    ${this.buildSlider('exposure', this.t('pixel.exposure'), -100, 100, 0)}
                    ${this.buildSlider('sharpness', this.t('pixel.sharpness'), 0, 100, 0)}
                    ${this.buildSlider('temperature', this.t('pixel.temperature'), -100, 100, 0)}
                    ${this.buildSlider('shadows', this.t('pixel.shadows'), -100, 100, 0)}
                    ${this.buildSlider('highlights', this.t('pixel.highlights'), -100, 100, 0)}
                    <div class="pixel-panel-actions"><button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-adjust">${this.esc(this.t('pixel.apply'))}</button><button class="pixel-btn" type="button" data-action="reset-adjust">${this.esc(this.t('pixel.reset'))}</button><button class="pixel-btn" type="button" data-action="compare-toggle">${this.esc(this.t('pixel.compare'))}</button></div>`;
            }),
            buildColorsPanelHTML: Pixel.bindRuntime(runtime, function buildColorsPanelHTML() {
                const paletteColors = this.PRESET_COLORS.map(c =>
                    `<button class="pixel-palette-swatch" type="button" data-palette-color="${c}" style="background:${c}" title="${c}"></button>`
                ).join('');
                return `<div class="pixel-color-section">
                        <div class="pixel-color-inputs">
                            <input type="text" class="pixel-hex-input" data-hex-input value="${this.primaryColor}" maxlength="7" spellcheck="false" title="${this.esc(this.t('pixel.hex_value'))}">
                        </div>
                        <div class="pixel-palette-grid">${paletteColors}</div>
                        <div class="pixel-recent-colors"><span class="pixel-label">${this.esc(this.t('pixel.recent_colors'))}</span><div class="pixel-recent-row" data-recent-colors></div></div>
                    </div>`;
            }),
            buildTransformPanelHTML: Pixel.bindRuntime(runtime, function buildTransformPanelHTML() {
                return `<div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="rotate-cw">${this.iconMarkup('redo', 'CW')} ${this.esc(this.t('pixel.rotate_cw'))}</button><button class="pixel-btn" type="button" data-action="rotate-ccw">${this.iconMarkup('undo', 'CCW')} ${this.esc(this.t('pixel.rotate_ccw'))}</button></div>
                    <div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="flip-h">${this.iconMarkup('sort', 'H')} ${this.esc(this.t('pixel.flip_h'))}</button><button class="pixel-btn" type="button" data-action="flip-v">${this.iconMarkup('sort', 'V')} ${this.esc(this.t('pixel.flip_v'))}</button></div>
                    <hr class="pixel-divider">
                    <button class="pixel-btn" type="button" data-action="crop">${this.iconMarkup('scissors', 'C')} ${this.esc(this.t('pixel.crop'))}</button>
                    <div class="pixel-crop-actions" data-crop-actions hidden>
                        <button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-crop">${this.esc(this.t('pixel.apply_crop'))}</button>
                        <button class="pixel-btn" type="button" data-action="cancel-crop">${this.esc(this.t('pixel.cancel_crop'))}</button>
                    </div>
                    <hr class="pixel-divider">
                    <button class="pixel-btn" type="button" data-action="resize">${this.iconMarkup('maximize', 'R')} ${this.esc(this.t('pixel.resize'))}</button>`;
            }),
            buildHistoryPanelHTML: Pixel.bindRuntime(runtime, function buildHistoryPanelHTML() {
                return '<div class="pixel-history-list" data-history-list></div>';
            }),
            buildLayersPanelHTML: Pixel.bindRuntime(runtime, function buildLayersPanelHTML() {
                const layerItems = this.layers.map((layer, i) => {
                    const isActive = i === this.activeLayerIdx;
                    return `<div class="pixel-layer-item${isActive ? ' active' : ''}" data-layer-idx="${i}">
                        <button class="pixel-layer-vis" type="button" data-layer-vis="${i}" title="${this.esc(this.t('pixel.toggle_visibility'))}">${layer.visible ? '👁' : '◻'}</button>
                        <span class="pixel-layer-name" data-layer-name="${i}">${this.esc(layer.name)}</span>
                        <input type="range" class="pixel-slider pixel-layer-opacity" data-layer-opacity="${i}" min="0" max="100" value="${Math.round(layer.opacity * 100)}" title="${this.esc(this.t('pixel.layer_opacity'))}">
                    </div>`;
                }).reverse().join('');

                return `<div class="pixel-layer-list" data-layer-list>${layerItems}</div>
                    <div class="pixel-layer-actions">
                        <button class="pixel-btn-icon" type="button" data-action="add-layer" title="${this.esc(this.t('pixel.new_layer'))}">+</button>
                        <button class="pixel-btn-icon" type="button" data-action="delete-layer" title="${this.esc(this.t('pixel.delete_layer'))}">−</button>
                        <button class="pixel-btn-icon" type="button" data-action="duplicate-layer" title="${this.esc(this.t('pixel.duplicate_layer'))}">⧉</button>
                        <button class="pixel-btn-icon" type="button" data-action="merge-layers" title="${this.esc(this.t('pixel.merge_layers'))}">⤓</button>
                        <button class="pixel-btn-icon" type="button" data-action="flatten-layers" title="${this.esc(this.t('pixel.flatten'))}">▭</button>
                    </div>`;
            }),
            buildAIPanelHTML: Pixel.bindRuntime(runtime, function buildAIPanelHTML() {
                return `<div class="pixel-ai-status" data-ai-status></div>
                    <div class="pixel-ai-panel">
                        <label class="pixel-label">${this.esc(this.t('pixel.prompt'))}</label>
                        <textarea class="pixel-ai-prompt" data-ai-prompt rows="3" placeholder="${this.esc(this.t('pixel.prompt_placeholder'))}"></textarea>
                        <label class="pixel-label">${this.esc(this.t('pixel.ai_negative_prompt'))}</label>
                        <textarea class="pixel-ai-prompt" data-ai-negative rows="2" placeholder="${this.esc(this.t('pixel.negative_placeholder'))}"></textarea>
                        <div class="pixel-ai-options">
                            <label class="pixel-label">${this.esc(this.t('pixel.ai_size'))}</label>
                            <select class="pixel-select" data-ai-size><option value="1024x1024">1024×1024</option><option value="1024x1792">1024×1792</option><option value="1792x1024">1792×1024</option><option value="512x512">512×512</option></select>
                            <label class="pixel-label">${this.esc(this.t('pixel.ai_quality'))}</label>
                            <select class="pixel-select" data-ai-quality><option value="standard">Standard</option><option value="hd">HD</option></select>
                            <label class="pixel-label">${this.esc(this.t('pixel.ai_style'))}</label>
                            <select class="pixel-select" data-ai-style><option value="vivid">${this.esc(this.t('pixel.style_vivid'))}</option><option value="natural">${this.esc(this.t('pixel.style_natural'))}</option></select>
                        </div>
                        <button class="pixel-btn pixel-btn-primary pixel-btn-full" type="button" data-action="ai-generate">${this.esc(this.t('pixel.generate'))}</button>
                    </div>
                    <hr class="pixel-divider">
                    <div class="pixel-ai-panel">
                        <label class="pixel-label">${this.esc(this.t('pixel.enhance'))}</label>
                        <textarea class="pixel-ai-prompt" data-enhance-prompt rows="2" placeholder="${this.esc(this.t('pixel.prompt_placeholder'))}"></textarea>
                        <label class="pixel-label">${this.esc(this.t('pixel.ai_strength'))} <span data-strength-val>0.7</span></label>
                        <input type="range" class="pixel-slider" data-enhance-strength min="0.1" max="1" step="0.05" value="0.7">
                        <button class="pixel-btn pixel-btn-full" type="button" data-action="ai-enhance">${this.esc(this.t('pixel.enhance'))}</button>
                    </div>
                    <hr class="pixel-divider">
                    <div class="pixel-ai-panel">
                        <label class="pixel-label">${this.esc(this.t('pixel.ai_result_mode'))}</label>
                        <select class="pixel-select" data-ai-result-mode>
                            <option value="layer">${this.esc(this.t('pixel.ai_as_layer'))}</option>
                            <option value="replace">${this.esc(this.t('pixel.ai_replace'))}</option>
                        </select>
                        <button class="pixel-btn pixel-btn-full" type="button" data-action="ai-remove-bg" data-ai-remove-bg>${this.esc(this.t('pixel.remove_bg'))}</button>
                        <button class="pixel-btn pixel-btn-full" type="button" data-action="ai-upscale" data-ai-upscale>${this.esc(this.t('pixel.upscale_2x'))}</button>
                    </div>`;
            }),
            buildSlider: Pixel.bindRuntime(runtime, function buildSlider(key, label, min, max, def) {
                return `<div class="pixel-slider-row">
                    <span class="pixel-slider-label">${this.esc(label)}</span>
                    <input type="range" class="pixel-slider" data-adjust="${key}" min="${min}" max="${max}" value="${def}">
                    <span class="pixel-slider-value" data-val-${key}>${def}</span>
                </div>`;
            }),

            // --- docks: inspector (top) and bottom dock in the right column, built from the panel registry ---
            loadDockSplit: Pixel.bindRuntime(runtime, function loadDockSplit() {
                let value = DEFAULT_DOCK_SPLIT;
                try {
                    const stored = parseFloat(localStorage.getItem(DOCK_SPLIT_KEY));
                    if (Number.isFinite(stored)) value = clampNumber(stored, 0.15, 0.8);
                } catch (_) { /* storage may be blocked */ }
                this.dockSplit = value;
                return value;
            }),
            defaultPanelFor: Pixel.bindRuntime(runtime, function defaultPanelFor(dock, panels) {
                const preferred = dock === 'bottom' ? 'layers' : 'adjust';
                return (panels.find(def => def.id === preferred) || panels[0]).id;
            }),
            buildDocksHTML: Pixel.bindRuntime(runtime, function buildDocksHTML() {
                if (!this.activePanels) this.activePanels = {};
                const split = this.dockSplit || this.loadDockSplit();
                const docks = Pixel.DOCKS.map(dock => ({ dock, panels: Pixel.listPanels(dock) })).filter(entry => entry.panels.length);
                docks.forEach(entry => {
                    if (!entry.panels.some(def => def.id === this.activePanels[entry.dock])) this.activePanels[entry.dock] = this.defaultPanelFor(entry.dock, entry.panels);
                });
                this.activePanel = this.activePanels.inspector || (docks[0] && this.activePanels[docks[0].dock]) || '';
                return docks.map((entry, index) => {
                    const active = this.activePanels[entry.dock];
                    const tabs = entry.panels.map(def =>
                        `<button class="pixel-tab${def.id === active ? ' active' : ''}" type="button" role="tab" data-panel="${this.esc(def.id)}">${def.icon ? this.iconMarkup(def.icon, '', 'pixel-tab-icon', 14) : ''}${this.esc(this.panelLabel(def))}</button>`
                    ).join('');
                    const sections = entry.panels.map(def => {
                        let body = '';
                        try { body = def.build ? def.build(this) || '' : ''; } catch (err) { if (window.console) console.error('pixel panel ' + def.id, err); }
                        return `<div class="pixel-panel-section pixel-panel-${this.esc(def.id)}" data-section="${this.esc(def.id)}"${def.id === active ? '' : ' hidden'}>${body}</div>`;
                    }).join('');
                    const style = entry.dock === 'bottom' && docks.length > 1 ? ` style="flex-basis:${(split * 100).toFixed(2)}%"` : '';
                    const splitter = index > 0
                        ? `<div class="pixel-splitter" data-dock-splitter role="separator" aria-orientation="horizontal" tabindex="0" title="${this.esc(this.t('pixel.dock_resize', 'Drag to resize the panels'))}"></div>`
                        : '';
                    return `${splitter}<div class="pixel-dock pixel-dock-${entry.dock}" data-dock="${entry.dock}"${style}>
                        <div class="pixel-tabs" role="tablist" data-dock-tabs="${entry.dock}">${tabs}</div>
                        <div class="pixel-dock-body">${sections}</div>
                    </div>`;
                }).join('');
            }),
            // Runs mount(r, sectionEl) of every registered panel once, right after the markup exists.
            mountPanels: Pixel.bindRuntime(runtime, function mountPanels() {
                Pixel.listPanels().forEach(def => {
                    if (typeof def.mount !== 'function') return;
                    const section = this.host.querySelector('[data-section="' + String(def.id).replace(/"/g, '') + '"]');
                    if (!section) return;
                    try { def.mount(this, section); } catch (err) { if (window.console) console.error('pixel panel mount ' + def.id, err); }
                });
            }),
            // Tab clicks, the draggable splitter between the docks and the initially active panels.
            initDocks: Pixel.bindRuntime(runtime, function initDocks() {
                this.host.querySelectorAll('.pixel-tab').forEach(btn => {
                    btn.addEventListener('click', () => this.showPanel(btn.dataset.panel));
                });
                const splitter = this.host.querySelector('[data-dock-splitter]');
                const bottom = this.host.querySelector('[data-dock="bottom"]');
                const column = this.panelContainer;
                if (splitter && bottom && column) {
                    let dragging = false;
                    const apply = (fraction, persist) => {
                        this.dockSplit = clampNumber(fraction, 0.15, 0.8);
                        bottom.style.flexBasis = (this.dockSplit * 100).toFixed(2) + '%';
                        if (persist) { try { localStorage.setItem(DOCK_SPLIT_KEY, String(Math.round(this.dockSplit * 1000) / 1000)); } catch (_) { /* optional */ } }
                    };
                    splitter.addEventListener('pointerdown', e => {
                        if (e.button !== 0) return;
                        dragging = true;
                        try { splitter.setPointerCapture(e.pointerId); } catch (_) { /* synthetic pointer */ }
                        splitter.classList.add('dragging');
                        e.preventDefault();
                    });
                    splitter.addEventListener('pointermove', e => {
                        if (!dragging) return;
                        const rect = column.getBoundingClientRect();
                        if (rect.height > 0) apply((rect.bottom - e.clientY) / rect.height, false);
                    });
                    const finish = () => {
                        if (!dragging) return;
                        dragging = false;
                        splitter.classList.remove('dragging');
                        apply(this.dockSplit, true);
                    };
                    splitter.addEventListener('pointerup', finish);
                    splitter.addEventListener('pointercancel', finish);
                    splitter.addEventListener('dblclick', () => apply(DEFAULT_DOCK_SPLIT, true));
                    splitter.addEventListener('keydown', e => {
                        if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return;
                        e.preventDefault();
                        apply(this.dockSplit + (e.key === 'ArrowUp' ? 0.03 : -0.03), true);
                    });
                }
                Pixel.DOCKS.forEach(dock => {
                    const id = this.activePanels && this.activePanels[dock];
                    if (id) this.showPanel(id);
                });
            }),

            // --- built-in panel wiring (each panel owns the listeners of its own section) ---
            mountAdjustPanel: Pixel.bindRuntime(runtime, function mountAdjustPanel(el) {
                const click = (selector, handler) => el.querySelectorAll(selector).forEach(b => b.addEventListener('click', handler));
                click('[data-action="apply-adjust"]', () => this.applyAdjustments());
                click('[data-action="reset-adjust"]', () => this.resetAdjustments());
                click('[data-action="compare-toggle"]', () => this.toggleCompare());
                el.querySelectorAll('[data-adjust]').forEach(slider => {
                    const show = value => { const valEl = el.querySelector(`[data-val-${slider.dataset.adjust}]`); if (valEl) valEl.textContent = String(value); };
                    slider.addEventListener('input', () => {
                        this.adjustments[slider.dataset.adjust] = Number(slider.value);
                        show(slider.value);
                        this.applyAdjustmentsPreview();
                    });
                    slider.addEventListener('dblclick', () => {
                        slider.value = 0;
                        this.adjustments[slider.dataset.adjust] = 0;
                        show('0');
                        this.applyAdjustmentsPreview();
                    });
                });
            }),
            mountFilterPanel: Pixel.bindRuntime(runtime, function mountFilterPanel(el) {
                const click = (selector, handler) => el.querySelectorAll(selector).forEach(b => b.addEventListener('click', handler));
                click('[data-action="apply-filter"]', () => this.applyFilter());
                click('[data-action="reset-filter"]', () => this.resetFilterPreview());
                click('[data-action="filter-compare"]', () => this.toggleFilterCompare());
                this.wireFilterPanel();
            }),
            mountColorsPanel: Pixel.bindRuntime(runtime, function mountColorsPanel(el) {
                el.querySelectorAll('[data-palette-color]').forEach(b => {
                    b.addEventListener('click', () => { this.setPrimaryColor(b.dataset.paletteColor); this.addRecentColor(b.dataset.paletteColor); });
                    b.addEventListener('contextmenu', e => { e.preventDefault(); e.stopPropagation(); this.setSecondaryColor(b.dataset.paletteColor); });
                });
                el.addEventListener('click', e => {
                    const recent = e.target.closest('[data-recent-color]');
                    if (recent) this.setPrimaryColor(recent.dataset.recentColor);
                });
                const hex = el.querySelector('[data-hex-input]');
                if (hex) {
                    hex.addEventListener('change', () => {
                        let v = hex.value.trim();
                        if (!v.startsWith('#')) v = '#' + v;
                        if (/^#[0-9a-fA-F]{6}$/.test(v)) { this.setPrimaryColor(v); this.addRecentColor(v); }
                        else hex.value = this.primaryColor;
                    });
                }
                this.renderRecentColors();
            }),
            mountTransformPanel: Pixel.bindRuntime(runtime, function mountTransformPanel(el) {
                const click = (selector, handler) => el.querySelectorAll(selector).forEach(b => b.addEventListener('click', handler));
                click('[data-action="rotate-cw"]', () => this.rotateCanvas(90));
                click('[data-action="rotate-ccw"]', () => this.rotateCanvas(-90));
                click('[data-action="flip-h"]', () => this.flipCanvas(true));
                click('[data-action="flip-v"]', () => this.flipCanvas(false));
                click('[data-action="crop"]', () => this.startCrop());
                click('[data-action="apply-crop"]', () => this.applyCrop());
                click('[data-action="cancel-crop"]', () => this.cancelCrop());
                click('[data-action="resize"]', () => this.resizeImage());
            }),
            mountLayersPanel: Pixel.bindRuntime(runtime, function mountLayersPanel(el) {
                const click = (selector, handler) => el.querySelectorAll(selector).forEach(b => b.addEventListener('click', handler));
                click('[data-action="add-layer"]', () => this.addLayer());
                click('[data-action="delete-layer"]', () => this.deleteLayer());
                click('[data-action="duplicate-layer"]', () => this.duplicateLayer());
                click('[data-action="merge-layers"]', () => this.mergeDown());
                click('[data-action="flatten-layers"]', () => this.flattenLayers());
                el.addEventListener('click', e => {
                    const maskBtn = e.target.closest('[data-layer-mask]');
                    if (maskBtn) {
                        this.toggleLayerMask(parseInt(maskBtn.dataset.layerMask, 10));
                        return;
                    }
                    const visBtn = e.target.closest('[data-layer-vis]');
                    if (visBtn) {
                        const layer = this.layers[parseInt(visBtn.dataset.layerVis, 10)];
                        if (layer) {
                            layer.visible = !layer.visible;
                            this.compositeLayers();
                            this.refreshLayerPanel();
                            this.notifyLayersChanged();
                        }
                        return;
                    }
                    const nameEl = e.target.closest('[data-layer-name]');
                    if (nameEl) {
                        const idx = parseInt(nameEl.dataset.layerName, 10);
                        this.resetFilterPreview();
                        this.commitPendingPreviews();
                        this.activeLayerIdx = idx;
                        this.cancelStickerGesture();
                        if (this.layers[idx] && this.layers[idx].sticker && this.activeTool !== 'move') this.setActiveTool('move');
                        if (this.editMaskMode) {
                            this.editMaskMode = false;
                            this.notify({ type: 'info', message: this.t('pixel.mask_edit_off') });
                        }
                        this.captureAdjustmentSource();
                        this.refreshLayerPanel();
                    }
                });
                el.addEventListener('input', e => {
                    const slider = e.target.closest('[data-layer-opacity]');
                    if (!slider) return;
                    const layer = this.layers[parseInt(slider.dataset.layerOpacity, 10)];
                    if (layer) {
                        layer.opacity = Number(slider.value) / 100;
                        this.compositeLayers();
                        this.notifyLayersChanged();
                    }
                });
            }),
            mountHistoryPanel: Pixel.bindRuntime(runtime, function mountHistoryPanel(el) {
                el.addEventListener('click', e => {
                    const item = e.target.closest('[data-history-idx]');
                    if (item) this.jumpToHistory(Number(item.dataset.historyIdx));
                });
            }),
            mountAIPanel: Pixel.bindRuntime(runtime, function mountAIPanel(el) {
                const click = (selector, handler) => el.querySelectorAll(selector).forEach(b => b.addEventListener('click', handler));
                click('[data-action="ai-generate"]', () => this.aiGenerate());
                click('[data-action="ai-enhance"]', () => this.aiEnhance());
                click('[data-action="ai-remove-bg"]', () => this.aiRemoveBG());
                click('[data-action="ai-upscale"]', () => this.aiUpscale());
                const strength = el.querySelector('[data-enhance-strength]');
                if (strength) {
                    strength.addEventListener('input', () => {
                        const label = el.querySelector('[data-strength-val]');
                        if (label) label.textContent = strength.value;
                    });
                }
            }),
            // Tool buttons and the colour block of the rail.
            mountToolRail: Pixel.bindRuntime(runtime, function mountToolRail() {
                this.host.querySelectorAll('[data-draw-tool]').forEach(b => {
                    b.addEventListener('click', () => this.setActiveTool(b.dataset.drawTool));
                });
                this.host.querySelectorAll('[data-action="swap-colors"]').forEach(b => b.addEventListener('click', () => this.swapColors()));
                const pick = (swatch, current, apply, remember) => {
                    if (!swatch) return;
                    swatch.addEventListener('click', () => {
                        const input = document.createElement('input');
                        input.type = 'color';
                        input.value = current();
                        input.addEventListener('input', () => apply(input.value));
                        input.addEventListener('change', () => { if (remember) this.addRecentColor(input.value); input.remove(); });
                        input.click();
                    });
                };
                pick(this.host.querySelector('[data-color-primary]'), () => this.primaryColor, v => this.setPrimaryColor(v), true);
                pick(this.host.querySelector('[data-color-secondary]'), () => this.secondaryColor, v => this.setSecondaryColor(v), false);
            }),

            // --- parameter controls ---
            // schema: [{ key, type, labelKey, fallback, default, min, max, step, unit, options: [{ value, labelKey, fallback }],
            // showIf(values), render(values) / wire(el, values, change) for 'custom' }]. Types: range, int, select, segmented,
            // bool, color, angle (0 = right, clockwise), seed, point ({ x, y } in 0..1), gradient (array of { pos 0..1, color }),
            // custom. Values live in a plain object that wireParamControls mutates in place.
            defaultParams: Pixel.bindRuntime(runtime, function defaultParams(schema) {
                const values = {};
                (schema || []).forEach(def => { if (def && def.key) values[def.key] = paramDefault(def); });
                return values;
            }),
            buildParamControls: Pixel.bindRuntime(runtime, function buildParamControls(schema, values, options) {
                const prefix = (options && options.idPrefix) || 'pixel-param';
                const vals = values || this.defaultParams(schema);
                const label = def => this.esc(def.labelKey ? this.t(def.labelKey, def.fallback || def.key) : (def.fallback || def.key));
                const optionLabel = opt => this.esc(opt.labelKey ? this.t(opt.labelKey, opt.fallback != null ? opt.fallback : String(opt.value)) : (opt.fallback != null ? opt.fallback : String(opt.value)));
                const body = (def, id, value) => {
                    const { min, max, step } = paramRange(def);
                    const unit = def.unit ? `<span class="pixel-param-unit">${this.esc(def.unit)}</span>` : '';
                    switch (def.type) {
                        case 'int':
                            return `<div class="pixel-param-row"><input type="number" class="pixel-param-number" id="${id}" data-param-input min="${min}" max="${max}" step="${def.step != null ? step : 1}" value="${this.esc(value)}">${unit}</div>`;
                        case 'select':
                            return `<select class="pixel-select" id="${id}" data-param-input>${(def.options || []).map((opt, i) => `<option value="${i}"${opt.value === value ? ' selected' : ''}>${optionLabel(opt)}</option>`).join('')}</select>`;
                        case 'segmented':
                            return `<div class="pixel-param-segmented" role="group" id="${id}">${(def.options || []).map((opt, i) => `<button type="button" class="pixel-param-seg-btn${opt.value === value ? ' active' : ''}" data-param-seg="${i}" aria-pressed="${opt.value === value ? 'true' : 'false'}">${optionLabel(opt)}</button>`).join('')}</div>`;
                        case 'bool':
                            return `<label class="pixel-param-switch"><input type="checkbox" role="switch" id="${id}" data-param-input${value ? ' checked' : ''}><span class="pixel-param-switch-track"><span class="pixel-param-switch-thumb"></span></span></label>`;
                        case 'color': {
                            const hex = normalizeHex(value) || '#000000';
                            return `<div class="pixel-param-row">
                                <button type="button" class="pixel-param-swatch" data-param-swatch style="background:${hex}" title="${this.esc(this.t('pixel.param_pick_color', 'Pick a color'))}"></button>
                                <input type="text" class="pixel-param-hex" id="${id}" data-param-hex maxlength="7" spellcheck="false" value="${hex}">
                                <input type="color" class="pixel-param-native" data-param-native value="${hex}" tabindex="-1" aria-hidden="true">
                                <button type="button" class="pixel-param-mini" data-param-use="primary" title="${this.esc(this.t('pixel.param_use_primary', 'Use primary color'))}"><span class="pixel-param-chip" style="background:${this.esc(this.primaryColor)}"></span>1</button>
                                <button type="button" class="pixel-param-mini" data-param-use="secondary" title="${this.esc(this.t('pixel.param_use_secondary', 'Use secondary color'))}"><span class="pixel-param-chip" style="background:${this.esc(this.secondaryColor)}"></span>2</button>
                            </div>`;
                        }
                        case 'angle':
                            return `<div class="pixel-param-row">
                                <div class="pixel-param-dial" data-param-dial tabindex="0" role="slider" aria-valuemin="0" aria-valuemax="360" aria-valuenow="${this.esc(value)}"><span class="pixel-param-dial-needle" data-param-needle style="transform:rotate(${Number(value) || 0}deg)"></span></div>
                                <input type="number" class="pixel-param-number" id="${id}" data-param-input min="0" max="360" step="1" value="${this.esc(value)}"><span class="pixel-param-unit">°</span>
                            </div>`;
                        case 'seed':
                            return `<div class="pixel-param-row"><input type="number" class="pixel-param-number pixel-param-seed" id="${id}" data-param-input min="0" max="4294967295" step="1" value="${this.esc(value)}"><button type="button" class="pixel-param-mini" data-param-dice title="${this.esc(this.t('pixel.param_randomize', 'Randomize'))}">${DICE_SVG}</button></div>`;
                        case 'point': {
                            const pt = value && typeof value === 'object' ? value : { x: 0.5, y: 0.5 };
                            return `<div class="pixel-param-row">
                                <div class="pixel-param-pad" data-param-pad tabindex="0"><span class="pixel-param-pad-dot" data-param-dot style="left:${clampNumber(pt.x, 0, 1) * 100}%;top:${clampNumber(pt.y, 0, 1) * 100}%"></span></div>
                                <label class="pixel-param-axis">X<input type="number" class="pixel-param-number" data-param-x min="0" max="1" step="0.01" value="${this.esc(Number(pt.x).toFixed(2))}"></label>
                                <label class="pixel-param-axis">Y<input type="number" class="pixel-param-number" data-param-y min="0" max="1" step="0.01" value="${this.esc(Number(pt.y).toFixed(2))}"></label>
                            </div>`;
                        }
                        case 'gradient': {
                            const stops = Array.isArray(value) ? value : [];
                            const lib = window.AuraPixelLib;
                            const presets = lib && lib.GRADIENT_PRESETS ? lib.GRADIENT_PRESETS : [];
                            const current = presets.find(p => sameStops(stops, p.stops));
                            return `<div class="pixel-param-gradient">
                                <select class="pixel-select" id="${id}" data-param-preset><option value="">${this.esc(this.t('pixel.param_custom', 'Custom'))}</option>${presets.map(p => `<option value="${this.esc(p.id)}"${current && current.id === p.id ? ' selected' : ''}>${this.esc(this.t('pixel.gradient_preset_' + p.id, p.id.charAt(0).toUpperCase() + p.id.slice(1)))}</option>`).join('')}</select>
                                <div class="pixel-param-gradient-bar" data-param-bar style="background:${gradientCss(stops)}"></div>
                                <div class="pixel-param-stops" data-param-stops>${stopRows(stops)}</div>
                                <button type="button" class="pixel-param-mini pixel-param-add-stop" data-param-add-stop>+ ${this.esc(this.t('pixel.param_add_stop', 'Add stop'))}</button>
                            </div>`;
                        }
                        case 'custom':
                            return `<div class="pixel-param-custom" data-param-custom>${typeof def.render === 'function' ? def.render(vals) : ''}</div>`;
                        default:
                            return `<div class="pixel-param-row"><input type="range" class="pixel-slider pixel-param-slider" id="${id}" data-param-input min="${min}" max="${max}" step="${step}" value="${this.esc(value)}"><input type="number" class="pixel-param-number" data-param-number min="${min}" max="${max}" step="${step}" value="${this.esc(readout(def, value))}">${unit}</div>`;
                    }
                };
                const stopRows = stops => stopRowsHTML(this, stops);
                return `<div class="pixel-param-list" data-param-root>${(schema || []).map(def => {
                    if (!def || !def.key) return '';
                    const type = def.type || 'range';
                    const entry = Object.assign({}, def, { type });
                    const value = vals[def.key] !== undefined ? vals[def.key] : paramDefault(entry);
                    if (vals[def.key] === undefined && value !== undefined) vals[def.key] = value;
                    const id = this.esc(prefix + '-' + def.key);
                    const hidden = Pixel.paramVisible(def, vals) ? '' : ' hidden';
                    return `<div class="pixel-param pixel-param-type-${type}" data-param="${this.esc(def.key)}" data-param-type="${type}"${hidden}>
                        <div class="pixel-param-head"><label class="pixel-param-label" for="${id}">${label(def)}</label></div>
                        ${body(entry, id, value)}
                    </div>`;
                }).join('')}</div>`;
            }),
            // Attaches the listeners for markup from buildParamControls. onChange(key, value, values) fires on every input
            // (callers debounce). Returns { set(key, value), refresh(), dispose() }; wiring the same root again replaces the old wiring.
            wireParamControls: Pixel.bindRuntime(runtime, function wireParamControls(rootEl, schema, values, onChange) {
                if (!rootEl) return null;
                if (paramWires.has(rootEl)) paramWires.get(rootEl).dispose();
                const r = this;
                const vals = values || {};
                const cleanups = [];
                const setters = {};
                const on = (el, type, fn, opts) => {
                    if (!el) return;
                    el.addEventListener(type, fn, opts);
                    cleanups.push(() => el.removeEventListener(type, fn, opts));
                };
                const find = key => rootEl.querySelector('[data-param="' + String(key).replace(/"/g, '') + '"]');
                const syncVisibility = () => {
                    (schema || []).forEach(def => {
                        if (!def || !def.key || typeof def.showIf !== 'function') return;
                        const el = find(def.key);
                        if (el) el.hidden = !Pixel.paramVisible(def, vals);
                    });
                };
                const emit = (def, value) => {
                    vals[def.key] = value;
                    syncVisibility();
                    if (typeof onChange === 'function') onChange(def.key, value, vals);
                };
                const reset = def => { const v = paramDefault(def); setters[def.key](v); emit(def, v); };

                (schema || []).forEach(def => {
                    if (!def || !def.key) return;
                    const el = find(def.key);
                    if (!el) return;
                    const type = def.type || 'range';
                    const { min, max, step } = paramRange(def);
                    const input = el.querySelector('[data-param-input]');
                    switch (type) {
                        case 'int': {
                            const read = () => clampNumber(Math.round(Number(input.value)), min, max);
                            on(input, 'input', () => { if (Number.isFinite(Number(input.value)) && input.value !== '') emit(def, read()); });
                            on(input, 'change', () => { const v = Number.isFinite(Number(input.value)) && input.value !== '' ? read() : paramDefault(def); input.value = String(v); emit(def, v); });
                            setters[def.key] = v => { input.value = String(v); };
                            break;
                        }
                        case 'select': {
                            on(input, 'change', () => { const opt = (def.options || [])[input.selectedIndex]; if (opt) emit(def, opt.value); });
                            setters[def.key] = v => { input.selectedIndex = Math.max(0, (def.options || []).findIndex(o => o.value === v)); };
                            break;
                        }
                        case 'segmented': {
                            const buttons = Array.from(el.querySelectorAll('[data-param-seg]'));
                            const paint = v => buttons.forEach((b, i) => {
                                const isOn = (def.options || [])[i] && def.options[i].value === v;
                                b.classList.toggle('active', !!isOn);
                                b.setAttribute('aria-pressed', isOn ? 'true' : 'false');
                            });
                            buttons.forEach(b => on(b, 'click', () => {
                                const opt = (def.options || [])[Number(b.dataset.paramSeg)];
                                if (!opt) return;
                                paint(opt.value);
                                emit(def, opt.value);
                            }));
                            setters[def.key] = paint;
                            break;
                        }
                        case 'bool': {
                            on(input, 'change', () => emit(def, !!input.checked));
                            setters[def.key] = v => { input.checked = !!v; };
                            break;
                        }
                        case 'color': {
                            const swatch = el.querySelector('[data-param-swatch]');
                            const hexInput = el.querySelector('[data-param-hex]');
                            const native = el.querySelector('[data-param-native]');
                            const paint = hex => { swatch.style.background = hex; hexInput.value = hex; native.value = hex; };
                            on(swatch, 'click', () => native.click());
                            on(native, 'input', () => { paint(native.value); emit(def, native.value); });
                            on(hexInput, 'change', () => {
                                const hex = normalizeHex(hexInput.value);
                                if (!hex) { hexInput.value = normalizeHex(vals[def.key]) || '#000000'; return; }
                                paint(hex);
                                emit(def, hex);
                            });
                            el.querySelectorAll('[data-param-use]').forEach(b => on(b, 'click', () => {
                                const hex = normalizeHex(b.dataset.paramUse === 'secondary' ? r.secondaryColor : r.primaryColor);
                                if (hex) { paint(hex); emit(def, hex); }
                            }));
                            setters[def.key] = v => paint(normalizeHex(v) || '#000000');
                            break;
                        }
                        case 'angle': {
                            const dial = el.querySelector('[data-param-dial]');
                            const needle = el.querySelector('[data-param-needle]');
                            const paint = v => {
                                const deg = clampNumber(Number(v) || 0, 0, 360);
                                input.value = String(Math.round(deg * 10) / 10);
                                needle.style.transform = 'rotate(' + deg + 'deg)';
                                dial.setAttribute('aria-valuenow', String(Math.round(deg)));
                            };
                            const set = v => { const deg = ((Math.round(v * 10) / 10) % 360 + 360) % 360; paint(deg); emit(def, deg); };
                            on(input, 'input', () => { if (input.value !== '' && Number.isFinite(Number(input.value))) { const deg = clampNumber(Number(input.value), 0, 360); needle.style.transform = 'rotate(' + deg + 'deg)'; emit(def, deg); } });
                            on(input, 'change', () => { const deg = clampNumber(Number(input.value) || 0, 0, 360); paint(deg); emit(def, deg); });
                            let dragging = false;
                            const fromPointer = e => {
                                const rect = dial.getBoundingClientRect();
                                let deg = Math.atan2(e.clientY - (rect.top + rect.height / 2), e.clientX - (rect.left + rect.width / 2)) * 180 / Math.PI;
                                if (e.shiftKey) deg = Math.round(deg / 15) * 15;
                                set(deg);
                            };
                            on(dial, 'pointerdown', e => { dragging = true; try { dial.setPointerCapture(e.pointerId); } catch (_) { /* synthetic */ } fromPointer(e); e.preventDefault(); });
                            on(dial, 'pointermove', e => { if (dragging) fromPointer(e); });
                            on(dial, 'pointerup', () => { dragging = false; });
                            on(dial, 'pointercancel', () => { dragging = false; });
                            on(dial, 'keydown', e => {
                                const stepBy = e.shiftKey ? 15 : 1;
                                if (e.key === 'ArrowRight' || e.key === 'ArrowUp') { e.preventDefault(); set((Number(vals[def.key]) || 0) + stepBy); }
                                if (e.key === 'ArrowLeft' || e.key === 'ArrowDown') { e.preventDefault(); set((Number(vals[def.key]) || 0) - stepBy); }
                            });
                            setters[def.key] = paint;
                            break;
                        }
                        case 'seed': {
                            const read = () => clampNumber(Math.floor(Number(input.value) || 0), 0, 4294967295);
                            on(input, 'input', () => { if (input.value !== '' && Number.isFinite(Number(input.value))) emit(def, read()); });
                            on(input, 'change', () => { const v = read(); input.value = String(v); emit(def, v); });
                            on(el.querySelector('[data-param-dice]'), 'click', () => {
                                const v = Math.floor(Math.random() * 4294967296);
                                input.value = String(v);
                                emit(def, v);
                            });
                            setters[def.key] = v => { input.value = String(v); };
                            break;
                        }
                        case 'point': {
                            const pad = el.querySelector('[data-param-pad]');
                            const dot = el.querySelector('[data-param-dot]');
                            const xi = el.querySelector('[data-param-x]');
                            const yi = el.querySelector('[data-param-y]');
                            const paint = pt => {
                                dot.style.left = clampNumber(pt.x, 0, 1) * 100 + '%';
                                dot.style.top = clampNumber(pt.y, 0, 1) * 100 + '%';
                                xi.value = Number(pt.x).toFixed(2);
                                yi.value = Number(pt.y).toFixed(2);
                            };
                            const set = (x, y) => { const pt = { x: Math.round(clampNumber(x, 0, 1) * 1000) / 1000, y: Math.round(clampNumber(y, 0, 1) * 1000) / 1000 }; paint(pt); emit(def, pt); };
                            const fromPointer = e => {
                                const rect = pad.getBoundingClientRect();
                                if (rect.width > 0 && rect.height > 0) set((e.clientX - rect.left) / rect.width, (e.clientY - rect.top) / rect.height);
                            };
                            let dragging = false;
                            on(pad, 'pointerdown', e => { dragging = true; try { pad.setPointerCapture(e.pointerId); } catch (_) { /* synthetic */ } fromPointer(e); e.preventDefault(); });
                            on(pad, 'pointermove', e => { if (dragging) fromPointer(e); });
                            on(pad, 'pointerup', () => { dragging = false; });
                            on(pad, 'pointercancel', () => { dragging = false; });
                            const fromFields = () => set(Number(xi.value) || 0, Number(yi.value) || 0);
                            on(xi, 'change', fromFields);
                            on(yi, 'change', fromFields);
                            setters[def.key] = v => paint(v && typeof v === 'object' ? v : { x: 0.5, y: 0.5 });
                            break;
                        }
                        case 'gradient': {
                            const preset = el.querySelector('[data-param-preset]');
                            const bar = el.querySelector('[data-param-bar]');
                            const list = el.querySelector('[data-param-stops]');
                            const stops = () => vals[def.key];
                            const repaint = (rebuildRows) => {
                                bar.style.background = gradientCss(stops());
                                const lib = window.AuraPixelLib;
                                const match = lib && lib.GRADIENT_PRESETS ? lib.GRADIENT_PRESETS.find(p => sameStops(stops(), p.stops)) : null;
                                preset.value = match ? match.id : '';
                                if (rebuildRows) { list.innerHTML = stopRowsHTML(r, stops()); bindRows(); }
                            };
                            const commit = rebuildRows => { repaint(rebuildRows); emit(def, vals[def.key]); };
                            const bindRows = () => {
                                list.querySelectorAll('[data-param-stop]').forEach(row => {
                                    const i = Number(row.dataset.paramStop);
                                    const color = row.querySelector('[data-stop-color]');
                                    const pos = row.querySelector('[data-stop-pos]');
                                    on(color, 'input', () => { stops()[i].color = color.value; commit(false); });
                                    on(pos, 'input', () => { if (pos.value !== '') { stops()[i].pos = clampNumber(Number(pos.value) / 100, 0, 1); commit(false); } });
                                    on(pos, 'change', () => { stops()[i].pos = clampNumber(Number(pos.value) / 100 || 0, 0, 1); stops().sort((a, b) => a.pos - b.pos); commit(true); });
                                    on(row.querySelector('[data-stop-remove]'), 'click', () => {
                                        if (stops().length <= 2) return;
                                        stops().splice(i, 1);
                                        commit(true);
                                    });
                                });
                            };
                            on(preset, 'change', () => {
                                const next = presetStops(preset.value);
                                if (!next) return;
                                vals[def.key] = next;
                                commit(true);
                            });
                            on(el.querySelector('[data-param-add-stop]'), 'click', () => {
                                const cur = stops().slice().sort((a, b) => a.pos - b.pos);
                                const points = [0].concat(cur.map(stop => stop.pos), [1]);
                                let at = 0.5, widest = -1;
                                for (let i = 0; i < points.length - 1; i++) {
                                    if (points[i + 1] - points[i] > widest) { widest = points[i + 1] - points[i]; at = (points[i] + points[i + 1]) / 2; }
                                }
                                const lib = window.AuraPixelLib;
                                let color = cur.length ? cur[0].color : '#808080';
                                if (lib && lib.gradientLUT) {
                                    const lut = lib.gradientLUT(cur, 256);
                                    const o = Math.round(at * 255) * 4;
                                    color = lib.toHex(lut[o], lut[o + 1], lut[o + 2]);
                                }
                                stops().push({ pos: Math.round(at * 1000) / 1000, color });
                                stops().sort((a, b) => a.pos - b.pos);
                                commit(true);
                            });
                            bindRows();
                            setters[def.key] = v => { vals[def.key] = Array.isArray(v) ? v : []; repaint(true); };
                            break;
                        }
                        case 'custom': {
                            const host = el.querySelector('[data-param-custom]');
                            if (typeof def.wire === 'function' && host) {
                                def.wire(host, vals, (value, key) => {
                                    const target = key || def.key;
                                    vals[target] = value;
                                    syncVisibility();
                                    if (typeof onChange === 'function') onChange(target, value, vals);
                                });
                            }
                            setters[def.key] = v => { vals[def.key] = v; if (typeof def.render === 'function' && host) host.innerHTML = def.render(vals); };
                            break;
                        }
                        default: {
                            const number = el.querySelector('[data-param-number]');
                            const parse = text => clampNumber(Number(text), min, max);
                            const decimals = decimalsOf(step);
                            const show = v => { number.value = Number(v).toFixed(decimals); };
                            on(input, 'input', () => { const v = parse(input.value); show(v); emit(def, v); });
                            on(number, 'input', () => { if (number.value !== '' && Number.isFinite(Number(number.value))) { const v = parse(number.value); input.value = String(v); emit(def, v); } });
                            on(number, 'change', () => { const v = Number.isFinite(Number(number.value)) && number.value !== '' ? parse(number.value) : paramDefault(def); input.value = String(v); show(v); emit(def, v); });
                            on(input, 'dblclick', () => reset(def));
                            on(el.querySelector('.pixel-param-label'), 'dblclick', () => reset(def));
                            setters[def.key] = v => { input.value = String(v); show(v); };
                        }
                    }
                });
                syncVisibility();
                const handle = {
                    set(key, value) {
                        if (!setters[key]) return;
                        vals[key] = value;
                        setters[key](value);
                        syncVisibility();
                    },
                    refresh() {
                        Object.keys(setters).forEach(key => { if (vals[key] !== undefined) setters[key](vals[key]); });
                        syncVisibility();
                    },
                    dispose() {
                        cleanups.splice(0).forEach(fn => fn());
                        if (paramWires.get(rootEl) === handle) paramWires.delete(rootEl);
                    }
                };
                paramWires.set(rootEl, handle);
                return handle;
            })
        });
    };

    // --- built-in registrations (replaced by feature modules through the same calls) ---

    Pixel.registerPanel({ id: 'adjust', labelKey: 'pixel.adjust', fallback: 'Adjust', order: 10, dock: 'inspector',
        build: r => r.buildAdjustPanelHTML(),
        mount: (r, el) => r.mountAdjustPanel(el),
        onShow: (r, previous) => {
            if (previous === 'adjust') return;
            r.commitPendingPreviews();
            r.captureAdjustmentSource();
            r.resetAdjustmentControls();
        },
        onHide: r => { if (r.compareMode) r.exitCompareMode({ preservePreview: true }); }
    });
    Pixel.registerPanel({ id: 'filters', labelKey: 'pixel.filters', fallback: 'Filters', order: 20, dock: 'inspector',
        build: r => (typeof r.buildFilterGalleryHTML === 'function' ? r.buildFilterGalleryHTML() : ''),
        mount: (r, el) => r.mountFilterPanel(el),
        onShow: r => r.refreshFilterThumbnails(),
        onHide: r => r.resetFilterPreview()
    });
    Pixel.registerPanel({ id: 'colors', labelKey: 'pixel.tab_colors', fallback: 'Colors', order: 30, dock: 'inspector',
        build: r => r.buildColorsPanelHTML(),
        mount: (r, el) => r.mountColorsPanel(el)
    });
    Pixel.registerPanel({ id: 'transform', labelKey: 'pixel.transform', fallback: 'Transform', order: 40, dock: 'inspector',
        build: r => r.buildTransformPanelHTML(),
        mount: (r, el) => r.mountTransformPanel(el),
        onHide: r => { if (r.cropState) r.cancelCrop(); }
    });
    Pixel.registerPanel({ id: 'ai', labelKey: 'pixel.ai_generate', fallback: 'AI', order: 80, dock: 'inspector',
        build: r => r.buildAIPanelHTML(),
        mount: (r, el) => r.mountAIPanel(el)
    });
    Pixel.registerPanel({ id: 'layers', labelKey: 'pixel.layers', fallback: 'Layers', order: 10, dock: 'bottom',
        build: r => r.buildLayersPanelHTML(),
        mount: (r, el) => r.mountLayersPanel(el),
        onShow: r => r.refreshLayerPanel()
    });
    Pixel.registerPanel({ id: 'history', labelKey: 'pixel.tab_history', fallback: 'History', order: 20, dock: 'bottom',
        build: r => r.buildHistoryPanelHTML(),
        mount: (r, el) => r.mountHistoryPanel(el),
        onShow: r => r.refreshHistoryPanel()
    });

    // Tools: the groups of TOOL_GROUPS become registry groups; no handlers, so pixel-events.js keeps handling them.
    (function registerBuiltinTools() {
        const raster = new Set(['brush', 'pencil', 'airbrush', 'eraser', 'clone-stamp', 'dodge-burn', 'blur-brush', 'fill', 'gradient', 'line', 'arrow', 'rectangle', 'ellipse', 'text']);
        const hiddenCursor = new Set(['brush', 'pencil', 'airbrush', 'eraser', 'clone-stamp', 'dodge-burn', 'blur-brush']);
        Pixel.TOOL_GROUPS.forEach((ids, groupIndex) => {
            ids.forEach((id, index) => {
                Pixel.registerTool({
                    id, group: Pixel.BUILTIN_TOOL_GROUP_NAMES[groupIndex] || 'paint', order: (index + 1) * 10,
                    svg: Pixel.TOOL_SVGS[id], shortcut: Pixel.TOOL_SHORTCUTS[id],
                    labelKey: 'pixel.' + id.replace(/-/g, '_'), fallback: id,
                    raster: raster.has(id),
                    cursor: hiddenCursor.has(id) ? 'none' : (id === 'text' ? 'text' : (id === 'move' ? 'move' : 'crosshair'))
                });
            });
        });
    })();

    // Window menus. Items keep the literal { id, labelKey, icon } shape; actions receive the window's runtime.
    Pixel.registerMenuItems('file', [
        { id: 'open', labelKey: 'pixel.open', icon: 'folder-open', shortcut: 'Ctrl+O', action: r => r.openFile() },
        { id: 'save', labelKey: 'pixel.save', icon: 'save', shortcut: 'Ctrl+S', action: r => r.saveFile() },
        { id: 'save-as', labelKey: 'pixel.save_as', icon: 'save', shortcut: 'Ctrl+Shift+S', action: r => r.saveFileAs() },
        { id: 'export', labelKey: 'pixel.export', icon: 'download', action: r => r.exportFile() },
        { type: 'separator' },
        { id: 'new-image', labelKey: 'pixel.new_image', icon: 'image', action: r => r.showNewImageDialog() }
    ], { order: 0 });
    Pixel.registerMenuItems('edit', [
        { id: 'undo', labelKey: 'pixel.undo', icon: 'undo', shortcut: 'Ctrl+Z', action: r => r.undo() },
        { id: 'redo', labelKey: 'pixel.redo', icon: 'redo', shortcut: 'Ctrl+Shift+Z', action: r => r.redo() },
        { type: 'separator' },
        { id: 'copy', labelKey: 'pixel.copy', icon: 'image', shortcut: 'Ctrl+C', action: r => r.copySelection() },
        { id: 'cut', labelKey: 'pixel.cut', icon: 'scissors', shortcut: 'Ctrl+X', action: r => r.cutSelection() },
        { id: 'paste', labelKey: 'pixel.paste', icon: 'image', shortcut: 'Ctrl+V', action: r => r.pasteClipboard() },
        { type: 'separator' },
        { id: 'select-all', labelKey: 'pixel.select_all', icon: 'image', shortcut: 'Ctrl+A', action: r => r.selectAll() },
        { id: 'deselect', labelKey: 'pixel.deselect', icon: 'image', shortcut: 'Ctrl+D', action: r => r.deselect() }
    ], { order: 0 });
    Pixel.registerMenuItems('view', [
        { id: 'zoom-in', labelKey: 'pixel.zoom_in', icon: 'zoom-in', shortcut: 'Ctrl++', action: r => r.zoomTo(r.zoom * 1.25) },
        { id: 'zoom-out', labelKey: 'pixel.zoom_out', icon: 'zoom-out', shortcut: 'Ctrl+-', action: r => r.zoomTo(r.zoom / 1.25) },
        { id: 'zoom-fit', labelKey: 'pixel.zoom_fit', icon: 'maximize', shortcut: 'Ctrl+0', action: r => r.zoomFit() },
        { id: 'zoom-100', labelKey: 'pixel.zoom_100', icon: 'zoom-in', action: r => r.zoomTo(1) },
        { type: 'separator' },
        { id: 'shortcuts', labelKey: 'pixel.shortcuts', icon: 'image', shortcut: '?', shortcuts: ['F1'], action: r => r.showShortcutsModal() }
    ], { order: 0 });
    Pixel.registerMenuItems('image', [
        { id: 'rotate-cw', labelKey: 'pixel.rotate_cw', icon: 'redo', action: r => r.rotateCanvas(90) },
        { id: 'rotate-ccw', labelKey: 'pixel.rotate_ccw', icon: 'undo', action: r => r.rotateCanvas(-90) },
        { id: 'flip-h', labelKey: 'pixel.flip_h', icon: 'sort', action: r => r.flipCanvas(true) },
        { id: 'flip-v', labelKey: 'pixel.flip_v', icon: 'sort', action: r => r.flipCanvas(false) },
        { type: 'separator' },
        { id: 'crop', labelKey: 'pixel.crop', icon: 'scissors', action: r => r.startCrop() },
        { id: 'resize', labelKey: 'pixel.resize', icon: 'maximize', action: r => r.resizeImage() }
    ], { order: 0 });
    Pixel.registerMenuItems('ai', [
        { id: 'generate', labelKey: 'pixel.generate', icon: 'image', action: r => r.showPanel('ai') },
        { id: 'enhance', labelKey: 'pixel.enhance', icon: 'image', action: r => r.showPanel('ai') }
    ], { order: 0 });

    // Keys that are not tied to a menu item or a tool.
    Pixel.registerCommand({ id: 'edit.cancel', labelKey: 'pixel.cmd_cancel', fallback: 'Cancel crop / deselect', group: 'edit', shortcut: 'Escape',
        run: r => { if (r.cropState) r.cancelCrop(); if (r.selection) r.deselect(); },
        enabled: r => !!(r.cropState || r.selection) });
    Pixel.registerCommand({ id: 'edit.delete-selection', labelKey: 'pixel.delete_selection', fallback: 'Delete selection', group: 'edit', shortcut: 'Delete',
        run: r => { r.deleteSelection(); r.deselect(); },
        enabled: r => !!r.selection });
})();
