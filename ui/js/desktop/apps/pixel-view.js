(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installView = function installView(runtime) {
        Object.assign(runtime, {
            toolSvg: Pixel.bindRuntime(runtime, function toolSvg(name) {
                return this.TOOL_SVGS[name] || '';
            }),
            toolLabel: Pixel.bindRuntime(runtime, function toolLabel(id) {
                return this.t('pixel.' + String(id).replace(/-/g, '_'), id);
            }),
            buildToolRailHTML: Pixel.bindRuntime(runtime, function buildToolRailHTML() {
                const groups = Pixel.TOOL_GROUPS.map(group =>
                    `<div class="pixel-rail-group">${group.map(id =>
                        `<button class="pixel-tool-btn${this.activeTool === id ? ' active' : ''}" type="button" data-draw-tool="${id}" title="${this.esc(this.toolLabel(id))}${Pixel.TOOL_SHORTCUTS[id] ? ' (' + Pixel.TOOL_SHORTCUTS[id] + ')' : ''}">${this.toolSvg(id)}</button>`
                    ).join('')}</div>`
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
            buildColorsPanelHTML: Pixel.bindRuntime(runtime, function buildColorsPanelHTML() {
                const paletteColors = this.PRESET_COLORS.map(c =>
                    `<button class="pixel-palette-swatch" type="button" data-palette-color="${c}" style="background:${c}" title="${c}"></button>`
                ).join('');
                return `<div class="pixel-panel-section pixel-panel-colors" data-section="colors" hidden>
                    <div class="pixel-color-section">
                        <div class="pixel-color-inputs">
                            <input type="text" class="pixel-hex-input" data-hex-input value="${this.primaryColor}" maxlength="7" spellcheck="false" title="${this.esc(this.t('pixel.hex_value'))}">
                        </div>
                        <div class="pixel-palette-grid">${paletteColors}</div>
                        <div class="pixel-recent-colors"><span class="pixel-label">${this.esc(this.t('pixel.recent_colors'))}</span><div class="pixel-recent-row" data-recent-colors></div></div>
                    </div>
                </div>`;
            }),
            buildHistoryPanelHTML: Pixel.bindRuntime(runtime, function buildHistoryPanelHTML() {
                return `<div class="pixel-panel-section pixel-panel-history" data-section="history" hidden>
                    <div class="pixel-history-list" data-history-list></div>
                </div>`;
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

                return `<div class="pixel-panel-section pixel-panel-layers" data-section="layers" hidden>
                    <div class="pixel-layer-list" data-layer-list>${layerItems}</div>
                    <div class="pixel-layer-actions">
                        <button class="pixel-btn-icon" type="button" data-action="add-layer" title="${this.esc(this.t('pixel.new_layer'))}">+</button>
                        <button class="pixel-btn-icon" type="button" data-action="delete-layer" title="${this.esc(this.t('pixel.delete_layer'))}">−</button>
                        <button class="pixel-btn-icon" type="button" data-action="duplicate-layer" title="${this.esc(this.t('pixel.duplicate_layer'))}">⧉</button>
                        <button class="pixel-btn-icon" type="button" data-action="merge-layers" title="${this.esc(this.t('pixel.merge_layers'))}">⤓</button>
                        <button class="pixel-btn-icon" type="button" data-action="flatten-layers" title="${this.esc(this.t('pixel.flatten'))}">▭</button>
                    </div>
                </div>`;
            }),
            buildPanelHTML: Pixel.bindRuntime(runtime, function buildPanelHTML() {
                return `<div class="pixel-panel-section pixel-panel-adjust" data-section="adjust">
                    ${this.buildSlider('brightness', this.t('pixel.brightness'), -100, 100, 0)}
                    ${this.buildSlider('contrast', this.t('pixel.contrast'), -100, 100, 0)}
                    ${this.buildSlider('saturation', this.t('pixel.saturation'), -100, 100, 0)}
                    ${this.buildSlider('exposure', this.t('pixel.exposure'), -100, 100, 0)}
                    ${this.buildSlider('sharpness', this.t('pixel.sharpness'), 0, 100, 0)}
                    ${this.buildSlider('temperature', this.t('pixel.temperature'), -100, 100, 0)}
                    ${this.buildSlider('shadows', this.t('pixel.shadows'), -100, 100, 0)}
                    ${this.buildSlider('highlights', this.t('pixel.highlights'), -100, 100, 0)}
                    <div class="pixel-panel-actions"><button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-adjust">${this.esc(this.t('pixel.apply'))}</button><button class="pixel-btn" type="button" data-action="reset-adjust">${this.esc(this.t('pixel.reset'))}</button><button class="pixel-btn" type="button" data-action="compare-toggle">${this.esc(this.t('pixel.compare'))}</button></div>
                </div>
                ${this.buildFilterGalleryHTML()}
                ${this.buildColorsPanelHTML()}
                <div class="pixel-panel-section pixel-panel-transform" data-section="transform" hidden>
                    <div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="rotate-cw">${this.iconMarkup('redo', 'CW')} ${this.esc(this.t('pixel.rotate_cw'))}</button><button class="pixel-btn" type="button" data-action="rotate-ccw">${this.iconMarkup('undo', 'CCW')} ${this.esc(this.t('pixel.rotate_ccw'))}</button></div>
                    <div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="flip-h">${this.iconMarkup('sort', 'H')} ${this.esc(this.t('pixel.flip_h'))}</button><button class="pixel-btn" type="button" data-action="flip-v">${this.iconMarkup('sort', 'V')} ${this.esc(this.t('pixel.flip_v'))}</button></div>
                    <hr class="pixel-divider">
                    <button class="pixel-btn" type="button" data-action="crop">${this.iconMarkup('scissors', 'C')} ${this.esc(this.t('pixel.crop'))}</button>
                    <div class="pixel-crop-actions" data-crop-actions hidden>
                        <button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-crop">${this.esc(this.t('pixel.apply_crop'))}</button>
                        <button class="pixel-btn" type="button" data-action="cancel-crop">${this.esc(this.t('pixel.cancel_crop'))}</button>
                    </div>
                    <hr class="pixel-divider">
                    <button class="pixel-btn" type="button" data-action="resize">${this.iconMarkup('maximize', 'R')} ${this.esc(this.t('pixel.resize'))}</button>
                </div>
                ${this.buildLayersPanelHTML()}
                ${this.buildHistoryPanelHTML()}
                <div class="pixel-panel-section pixel-panel-ai" data-section="ai" hidden>
                    <div class="pixel-ai-status" data-ai-status></div>
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
                    </div>
                </div>`;
            }),
            buildSlider: Pixel.bindRuntime(runtime, function buildSlider(key, label, min, max, def) {
                return `<div class="pixel-slider-row">
                    <span class="pixel-slider-label">${this.esc(label)}</span>
                    <input type="range" class="pixel-slider" data-adjust="${key}" min="${min}" max="${max}" value="${def}">
                    <span class="pixel-slider-value" data-val-${key}>${def}</span>
                </div>`;
            })
        });
    };
})();
