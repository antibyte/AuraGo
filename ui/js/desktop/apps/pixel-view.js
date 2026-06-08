(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installView = function installView(runtime) {
        Object.assign(runtime, {
            toolSvg: Pixel.bindRuntime(runtime, function toolSvg(name) {                     return this.TOOL_SVGS[name] || '';
            }),
            buildDrawPanelHTML: Pixel.bindRuntime(runtime, function buildDrawPanelHTML() {
                                const tools = [
                                    ['select-rect', this.t('pixel.select_rect', 'Rectangle Select')],
                                    ['select-ellipse', this.t('pixel.select_ellipse', 'Ellipse Select')],
                                    ['brush', this.t('pixel.brush', 'Brush')],
                                    ['eraser', this.t('pixel.eraser', 'Eraser')],
                                    ['pencil', this.t('pixel.pencil', 'Pencil')],
                                    ['line', this.t('pixel.line', 'Line')],
                                    ['rectangle', this.t('pixel.rectangle', 'Rectangle')],
                                    ['ellipse', this.t('pixel.ellipse', 'Ellipse')],
                                    ['arrow', this.t('pixel.arrow', 'Arrow')],
                                    ['text', this.t('pixel.text', 'Text')],
                                    ['fill', this.t('pixel.fill', 'Fill')],
                                    ['eyedropper', this.t('pixel.eyedropper', 'Eyedropper')]
                                ];
                                const toolGrid = tools.map(([id, label]) =>
                                    `<button class="pixel-tool-btn${this.activeTool === id ? ' active' : ''}" type="button" data-draw-tool="${id}" title="${this.esc(label)}">${this.toolSvg(id)}</button>`
                                ).join('');

                                const paletteColors = this.PRESET_COLORS.map(c =>
                                    `<button class="pixel-palette-swatch" type="button" data-palette-color="${c}" style="background:${c}" title="${c}"></button>`
                                ).join('');

                                const recentHtml = this.recentColors.slice(0, 8).map(c =>
                                    `<button class="pixel-recent-swatch" type="button" data-recent-color="${c}" style="background:${c}" title="${c}"></button>`
                                ).join('');

                                return `<div class="pixel-panel-section pixel-panel-draw" data-section="draw" hidden>
                                    <div class="pixel-draw-tools-grid">${toolGrid}</div>
                                    <hr class="pixel-divider">
                                    <div class="pixel-color-section">
                                        <div class="pixel-color-row">
                                            <div class="pixel-color-swatch-wrap">
                                                <div class="pixel-color-swatch pixel-color-primary" data-color-primary style="background:${this.primaryColor}" title="${this.esc(this.t('pixel.color_primary', 'Primary Color'))}"></div>
                                                <div class="pixel-color-swatch pixel-color-secondary" data-color-secondary style="background:${this.secondaryColor}" title="${this.esc(this.t('pixel.color_secondary', 'Secondary Color'))}"></div>
                                                <button class="pixel-color-swap" type="button" data-action="swap-colors" title="${this.esc(this.t('pixel.swap_colors', 'Swap Colors'))}">⇄</button>
                                            </div>
                                            <div class="pixel-color-inputs">
                                                <input type="text" class="pixel-hex-input" data-hex-input value="${this.primaryColor}" maxlength="7" spellcheck="false" title="${this.esc(this.t('pixel.hex_value', 'Hex Color'))}">
                                                <input type="range" class="pixel-slider" data-opacity-slider min="0" max="100" value="${this.brushOpacity}" title="${this.esc(this.t('pixel.brush_opacity', 'Opacity'))}">
                                                <span class="pixel-slider-value">${this.brushOpacity}%</span>
                                            </div>
                                        </div>
                                        <div class="pixel-palette-grid">${paletteColors}</div>
                                        ${recentHtml ? `<div class="pixel-recent-colors"><span class="pixel-label">${this.esc(this.t('pixel.recent_colors', 'Recent'))}</span><div class="pixel-recent-row">${recentHtml}</div></div>` : ''}
                                    </div>
                                    <hr class="pixel-divider">
                                    <div class="pixel-draw-options" data-draw-options>
                                        ${this.buildDrawOptionsHTML()}
                                    </div>
                                    <hr class="pixel-divider">
                                    <div class="pixel-panel-actions">
                                        <button class="pixel-btn" type="button" data-action="clear-this.selection">${this.esc(this.t('pixel.clear_selection', 'Clear Selection'))}</button>
                                    </div>
                                </div>`;
            }),
            buildDrawOptionsHTML: Pixel.bindRuntime(runtime, function buildDrawOptionsHTML() {
                                let html = '';
                                const isBrushType = this.activeTool === 'brush' || this.activeTool === 'eraser' || this.activeTool === 'pencil';
                                const isShapeType = this.activeTool === 'line' || this.activeTool === 'rectangle' || this.activeTool === 'ellipse' || this.activeTool === 'arrow';
                                const isTextType = this.activeTool === 'text';
                                const isFillType = this.activeTool === 'fill';

                                if (isBrushType) {
                                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.brush_size', 'Size'))}</span><input type="range" class="pixel-slider" data-brush-size min="1" max="200" value="${this.brushSize}"><span class="pixel-slider-value" data-brush-size-val>${this.brushSize}px</span></div>`;
                                    if (this.activeTool !== 'pencil') {
                                        html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.brush_opacity', 'Opacity'))}</span><input type="range" class="pixel-slider" data-brush-opacity min="1" max="100" value="${this.brushOpacity}"><span class="pixel-slider-value" data-brush-opacity-val>${this.brushOpacity}%</span></div>`;
                                        html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.brush_hardness', 'Hardness'))}</span><input type="range" class="pixel-slider" data-brush-hardness min="0" max="100" value="${this.brushHardness}"><span class="pixel-slider-value" data-brush-hardness-val>${this.brushHardness}%</span></div>`;
                                    }
                                } else if (isShapeType) {
                                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.stroke_width', 'Stroke'))}</span><input type="range" class="pixel-slider" data-shape-stroke min="1" max="20" value="${this.shapeStrokeWidth}"><span class="pixel-slider-value" data-shape-stroke-val>${this.shapeStrokeWidth}px</span></div>`;
                                    if (this.activeTool === 'rectangle' || this.activeTool === 'ellipse') {
                                        html += `<label class="pixel-checkbox-row"><input type="checkbox" data-shape-fill ${this.shapeFill ? 'checked' : ''}> ${this.esc(this.t('pixel.fill_mode', 'Fill shape'))}</label>`;
                                    }
                                } else if (isTextType) {
                                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.font_size', 'Font Size'))}</span><input type="range" class="pixel-slider" data-font-size min="8" max="200" value="${this.fontSize}"><span class="pixel-slider-value" data-font-size-val>${this.fontSize}px</span></div>`;
                                    html += `<label class="pixel-label">${this.esc(this.t('pixel.font_family', 'Font'))}</label><select class="pixel-select" data-font-family>${this.FONT_FAMILIES.map(f => `<option value="${f}" ${f === this.fontFamily ? 'selected' : ''}>${f}</option>`).join('')}</select>`;
                                } else if (isFillType) {
                                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${this.esc(this.t('pixel.tolerance', 'Tolerance'))}</span><input type="range" class="pixel-slider" data-fill-tolerance min="0" max="128" value="${this.fillTolerance}"><span class="pixel-slider-value" data-fill-tolerance-val>${this.fillTolerance}</span></div>`;
                                }

                                if (!html) {
                                    html = `<p class="pixel-draw-hint">${this.esc(this.t('pixel.select_tool_hint', 'Select a drawing tool'))}</p>`;
                                }
                                return html;
            }),
            buildLayersPanelHTML: Pixel.bindRuntime(runtime, function buildLayersPanelHTML() {
                                const layerItems = this.layers.map((layer, i) => {
                                    const isActive = i === this.activeLayerIdx;
                                    return `<div class="pixel-layer-item${isActive ? ' active' : ''}" data-layer-idx="${i}">
                                        <button class="pixel-layer-vis" type="button" data-layer-vis="${i}" title="${this.esc(this.t('pixel.toggle_visibility', 'Toggle Visibility'))}">${layer.visible ? '👁' : '◻'}</button>
                                        <span class="pixel-layer-name" data-layer-name="${i}">${this.esc(layer.name)}</span>
                                        <input type="range" class="pixel-slider pixel-layer-opacity" data-layer-opacity="${i}" min="0" max="100" value="${Math.round(layer.opacity * 100)}" title="${this.esc(this.t('pixel.layer_opacity', 'Opacity'))}">
                                    </div>`;
                                }).reverse().join('');

                                return `<div class="pixel-panel-section pixel-panel-layers" data-section="layers" hidden>
                                    <div class="pixel-layer-list" data-layer-list>${layerItems}</div>
                                    <div class="pixel-layer-actions">
                                        <button class="pixel-btn-icon" type="button" data-action="add-layer" title="${this.esc(this.t('pixel.new_layer', 'Add Layer'))}">+</button>
                                        <button class="pixel-btn-icon" type="button" data-action="delete-layer" title="${this.esc(this.t('pixel.delete_layer', 'Delete Layer'))}">−</button>
                                        <button class="pixel-btn-icon" type="button" data-action="duplicate-layer" title="${this.esc(this.t('pixel.duplicate_layer', 'Duplicate'))}">⧉</button>
                                        <button class="pixel-btn-icon" type="button" data-action="merge-layers" title="${this.esc(this.t('pixel.merge_layers', 'Merge Down'))}">⤓</button>
                                        <button class="pixel-btn-icon" type="button" data-action="flatten-layers" title="${this.esc(this.t('pixel.flatten', 'Flatten'))}">▭</button>
                                    </div>
                                </div>`;
            }),
            buildPanelHTML: Pixel.bindRuntime(runtime, function buildPanelHTML() {
                                return `<div class="pixel-panel-section pixel-panel-adjust" data-section="adjust">
                                    ${this.buildSlider('brightness', this.t('pixel.brightness', 'Brightness'), -100, 100, 0)}
                                    ${this.buildSlider('contrast', this.t('pixel.contrast', 'Contrast'), -100, 100, 0)}
                                    ${this.buildSlider('saturation', this.t('pixel.saturation', 'Saturation'), -100, 100, 0)}
                                    ${this.buildSlider('exposure', this.t('pixel.exposure', 'Exposure'), -100, 100, 0)}
                                    ${this.buildSlider('sharpness', this.t('pixel.sharpness', 'Sharpness'), 0, 100, 0)}
                                    ${this.buildSlider('temperature', this.t('pixel.temperature', 'Temperature'), -100, 100, 0)}
                                    ${this.buildSlider('shadows', this.t('pixel.shadows', 'Shadows'), -100, 100, 0)}
                                    ${this.buildSlider('highlights', this.t('pixel.highlights', 'Highlights'), -100, 100, 0)}
                                    <div class="pixel-panel-actions"><button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-adjust">${this.esc(this.t('pixel.apply', 'Apply'))}</button><button class="pixel-btn" type="button" data-action="reset-adjust">${this.esc(this.t('pixel.reset', 'Reset'))}</button><button class="pixel-btn" type="button" data-action="compare-toggle">${this.esc(this.t('pixel.compare', 'Compare'))}</button></div>
                                </div>
                                <div class="pixel-panel-section pixel-panel-filters" data-section="filters" hidden>
                                    <div class="pixel-filter-grid" data-filter-grid>
                                        ${this.buildFilterCard('grayscale', 'Grayscale')}
                                        ${this.buildFilterCard('sepia', 'Sepia')}
                                        ${this.buildFilterCard('invert', 'Invert')}
                                        ${this.buildFilterCard('blur', 'Blur')}
                                        ${this.buildFilterCard('vintage', 'Vintage')}
                                        ${this.buildFilterCard('vignette', 'Vignette')}
                                        ${this.buildFilterCard('warm', 'Warm')}
                                        ${this.buildFilterCard('cool', 'Cool')}
                                        ${this.buildFilterCard('high-contrast', 'High Contrast')}
                                        ${this.buildFilterCard('emboss', 'Emboss')}
                                    </div>
                                </div>
                                <div class="pixel-panel-section pixel-panel-transform" data-section="transform" hidden>
                                    <div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="rotate-cw">${this.iconMarkup('redo', 'CW')} ${this.esc(this.t('pixel.rotate_cw', 'Rotate CW'))}</button><button class="pixel-btn" type="button" data-action="rotate-ccw">${this.iconMarkup('undo', 'CCW')} ${this.esc(this.t('pixel.rotate_ccw', 'Rotate CCW'))}</button></div>
                                    <div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="flip-h">${this.iconMarkup('sort', 'H')} ${this.esc(this.t('pixel.flip_h', 'Flip H'))}</button><button class="pixel-btn" type="button" data-action="flip-v">${this.iconMarkup('sort', 'V')} ${this.esc(this.t('pixel.flip_v', 'Flip V'))}</button></div>
                                    <hr class="pixel-divider">
                                    <button class="pixel-btn" type="button" data-action="crop">${this.iconMarkup('scissors', 'C')} ${this.esc(this.t('pixel.crop', 'Crop'))}</button>
                                    <div class="pixel-crop-actions" data-crop-actions hidden>
                                        <button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-crop">${this.esc(this.t('pixel.apply_crop', 'Apply Crop'))}</button>
                                        <button class="pixel-btn" type="button" data-action="cancel-crop">${this.esc(this.t('pixel.cancel_crop', 'Cancel'))}</button>
                                    </div>
                                    <hr class="pixel-divider">
                                    <button class="pixel-btn" type="button" data-action="resize">${this.iconMarkup('maximize', 'R')} ${this.esc(this.t('pixel.resize', 'Resize'))}</button>
                                </div>
                                ${this.buildDrawPanelHTML()}
                                ${this.buildLayersPanelHTML()}
                                <div class="pixel-panel-section pixel-panel-ai" data-section="ai" hidden>
                                    <div class="pixel-ai-status" data-ai-status></div>
                                    <div class="pixel-ai-panel">
                                        <label class="pixel-label">${this.esc(this.t('pixel.prompt', 'Prompt'))}</label>
                                        <textarea class="pixel-ai-prompt" data-ai-prompt rows="3" placeholder="${this.esc(this.t('pixel.prompt_placeholder', 'Describe the image to generate...'))}"></textarea>
                                        <label class="pixel-label">${this.esc(this.t('pixel.ai_negative_prompt', 'Negative Prompt'))}</label>
                                        <textarea class="pixel-ai-prompt" data-ai-negative rows="2" placeholder="${this.esc(this.t('pixel.negative_placeholder', 'What to exclude...'))}"></textarea>
                                        <div class="pixel-ai-options">
                                            <label class="pixel-label">${this.esc(this.t('pixel.ai_size', 'Size'))}</label>
                                            <select class="pixel-select" data-ai-size><option value="1024x1024">1024×1024</option><option value="1024x1792">1024×1792</option><option value="1792x1024">1792×1024</option><option value="512x512">512×512</option></select>
                                            <label class="pixel-label">${this.esc(this.t('pixel.ai_quality', 'Quality'))}</label>
                                            <select class="pixel-select" data-ai-quality><option value="standard">Standard</option><option value="hd">HD</option></select>
                                            <label class="pixel-label">${this.esc(this.t('pixel.ai_style', 'Style'))}</label>
                                            <select class="pixel-select" data-ai-style><option value="vivid">${this.esc(this.t('pixel.style_vivid', 'Vivid'))}</option><option value="natural">${this.esc(this.t('pixel.style_natural', 'Natural'))}</option></select>
                                        </div>
                                        <button class="pixel-btn pixel-btn-primary pixel-btn-full" type="button" data-action="ai-generate">${this.esc(this.t('pixel.generate', 'Generate'))}</button>
                                    </div>
                                    <hr class="pixel-divider">
                                    <div class="pixel-ai-panel">
                                        <label class="pixel-label">${this.esc(this.t('pixel.enhance', 'Enhance Image'))}</label>
                                        <textarea class="pixel-ai-prompt" data-enhance-prompt rows="2" placeholder="${this.esc(this.t('pixel.prompt_placeholder', 'Describe enhancements...'))}"></textarea>
                                        <label class="pixel-label">${this.esc(this.t('pixel.ai_strength', 'Strength'))} <span data-strength-val>0.7</span></label>
                                        <input type="range" class="pixel-slider" data-enhance-strength min="0.1" max="1" step="0.05" value="0.7">
                                        <button class="pixel-btn pixel-btn-full" type="button" data-action="ai-enhance">${this.esc(this.t('pixel.enhance', 'Enhance'))}</button>
                                    </div>
                                </div>`;
            }),
            buildSlider: Pixel.bindRuntime(runtime, function buildSlider(key, label, min, max, def) {
                                return `<div class="pixel-slider-row">
                                    <span class="pixel-slider-label">${this.esc(label)}</span>
                                    <input type="range" class="pixel-slider" data-adjust="${key}" min="${min}" max="${max}" value="${def}">
                                    <span class="pixel-slider-value" data-val-${key}>${def}</span>
                                </div>`;
            }),
            buildFilterCard: Pixel.bindRuntime(runtime, function buildFilterCard(key, label) {
                                return `<button class="pixel-filter-card" type="button" data-filter="${key}"><span class="pixel-filter-preview pixel-filter-${key}"></span><span class="pixel-filter-name">${this.esc(this.t('pixel.filter_' + key.replace('-', '_'), label))}</span></button>`;
            })
        });
    };
})();
