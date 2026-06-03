(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installView = function installView(runtime) {
        Object.assign(runtime, {
            toolSvg: Pixel.bindRuntime(runtime, function toolSvg(name) {                     return TOOL_SVGS[name] || '';
            }),
            buildDrawPanelHTML: Pixel.bindRuntime(runtime, function buildDrawPanelHTML() {
                                const tools = [
                                    ['select-rect', t('pixel.select_rect', 'Rectangle Select')],
                                    ['select-ellipse', t('pixel.select_ellipse', 'Ellipse Select')],
                                    ['brush', t('pixel.brush', 'Brush')],
                                    ['eraser', t('pixel.eraser', 'Eraser')],
                                    ['pencil', t('pixel.pencil', 'Pencil')],
                                    ['line', t('pixel.line', 'Line')],
                                    ['rectangle', t('pixel.rectangle', 'Rectangle')],
                                    ['ellipse', t('pixel.ellipse', 'Ellipse')],
                                    ['arrow', t('pixel.arrow', 'Arrow')],
                                    ['text', t('pixel.text', 'Text')],
                                    ['fill', t('pixel.fill', 'Fill')],
                                    ['eyedropper', t('pixel.eyedropper', 'Eyedropper')]
                                ];
                                const toolGrid = tools.map(([id, label]) =>
                                    `<button class="pixel-tool-btn${activeTool === id ? ' active' : ''}" type="button" data-draw-tool="${id}" title="${esc(label)}">${toolSvg(id)}</button>`
                                ).join('');

                                const paletteColors = PRESET_COLORS.map(c =>
                                    `<button class="pixel-palette-swatch" type="button" data-palette-color="${c}" style="background:${c}" title="${c}"></button>`
                                ).join('');

                                const recentHtml = recentColors.slice(0, 8).map(c =>
                                    `<button class="pixel-recent-swatch" type="button" data-recent-color="${c}" style="background:${c}" title="${c}"></button>`
                                ).join('');

                                return `<div class="pixel-panel-section pixel-panel-draw" data-section="draw" hidden>
                                    <div class="pixel-draw-tools-grid">${toolGrid}</div>
                                    <hr class="pixel-divider">
                                    <div class="pixel-color-section">
                                        <div class="pixel-color-row">
                                            <div class="pixel-color-swatch-wrap">
                                                <div class="pixel-color-swatch pixel-color-primary" data-color-primary style="background:${primaryColor}" title="${esc(t('pixel.color_primary', 'Primary Color'))}"></div>
                                                <div class="pixel-color-swatch pixel-color-secondary" data-color-secondary style="background:${secondaryColor}" title="${esc(t('pixel.color_secondary', 'Secondary Color'))}"></div>
                                                <button class="pixel-color-swap" type="button" data-action="swap-colors" title="${esc(t('pixel.swap_colors', 'Swap Colors'))}">⇄</button>
                                            </div>
                                            <div class="pixel-color-inputs">
                                                <input type="text" class="pixel-hex-input" data-hex-input value="${primaryColor}" maxlength="7" spellcheck="false" title="${esc(t('pixel.hex_value', 'Hex Color'))}">
                                                <input type="range" class="pixel-slider" data-opacity-slider min="0" max="100" value="${brushOpacity}" title="${esc(t('pixel.brush_opacity', 'Opacity'))}">
                                                <span class="pixel-slider-value">${brushOpacity}%</span>
                                            </div>
                                        </div>
                                        <div class="pixel-palette-grid">${paletteColors}</div>
                                        ${recentHtml ? `<div class="pixel-recent-colors"><span class="pixel-label">${esc(t('pixel.recent_colors', 'Recent'))}</span><div class="pixel-recent-row">${recentHtml}</div></div>` : ''}
                                    </div>
                                    <hr class="pixel-divider">
                                    <div class="pixel-draw-options" data-draw-options>
                                        ${buildDrawOptionsHTML()}
                                    </div>
                                    <hr class="pixel-divider">
                                    <div class="pixel-panel-actions">
                                        <button class="pixel-btn" type="button" data-action="clear-selection">${esc(t('pixel.clear_selection', 'Clear Selection'))}</button>
                                    </div>
                                </div>`;
            }),
            buildDrawOptionsHTML: Pixel.bindRuntime(runtime, function buildDrawOptionsHTML() {
                                let html = '';
                                const isBrushType = activeTool === 'brush' || activeTool === 'eraser' || activeTool === 'pencil';
                                const isShapeType = activeTool === 'line' || activeTool === 'rectangle' || activeTool === 'ellipse' || activeTool === 'arrow';
                                const isTextType = activeTool === 'text';
                                const isFillType = activeTool === 'fill';

                                if (isBrushType) {
                                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${esc(t('pixel.brush_size', 'Size'))}</span><input type="range" class="pixel-slider" data-brush-size min="1" max="200" value="${brushSize}"><span class="pixel-slider-value" data-brush-size-val>${brushSize}px</span></div>`;
                                    if (activeTool !== 'pencil') {
                                        html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${esc(t('pixel.brush_opacity', 'Opacity'))}</span><input type="range" class="pixel-slider" data-brush-opacity min="1" max="100" value="${brushOpacity}"><span class="pixel-slider-value" data-brush-opacity-val>${brushOpacity}%</span></div>`;
                                        html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${esc(t('pixel.brush_hardness', 'Hardness'))}</span><input type="range" class="pixel-slider" data-brush-hardness min="0" max="100" value="${brushHardness}"><span class="pixel-slider-value" data-brush-hardness-val>${brushHardness}%</span></div>`;
                                    }
                                } else if (isShapeType) {
                                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${esc(t('pixel.stroke_width', 'Stroke'))}</span><input type="range" class="pixel-slider" data-shape-stroke min="1" max="20" value="${shapeStrokeWidth}"><span class="pixel-slider-value" data-shape-stroke-val>${shapeStrokeWidth}px</span></div>`;
                                    if (activeTool === 'rectangle' || activeTool === 'ellipse') {
                                        html += `<label class="pixel-checkbox-row"><input type="checkbox" data-shape-fill ${shapeFill ? 'checked' : ''}> ${esc(t('pixel.fill_mode', 'Fill shape'))}</label>`;
                                    }
                                } else if (isTextType) {
                                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${esc(t('pixel.font_size', 'Font Size'))}</span><input type="range" class="pixel-slider" data-font-size min="8" max="200" value="${fontSize}"><span class="pixel-slider-value" data-font-size-val>${fontSize}px</span></div>`;
                                    html += `<label class="pixel-label">${esc(t('pixel.font_family', 'Font'))}</label><select class="pixel-select" data-font-family>${FONT_FAMILIES.map(f => `<option value="${f}" ${f === fontFamily ? 'selected' : ''}>${f}</option>`).join('')}</select>`;
                                } else if (isFillType) {
                                    html += `<div class="pixel-slider-row"><span class="pixel-slider-label">${esc(t('pixel.tolerance', 'Tolerance'))}</span><input type="range" class="pixel-slider" data-fill-tolerance min="0" max="128" value="${fillTolerance}"><span class="pixel-slider-value" data-fill-tolerance-val>${fillTolerance}</span></div>`;
                                }

                                if (!html) {
                                    html = `<p class="pixel-draw-hint">${esc(t('pixel.select_tool_hint', 'Select a drawing tool'))}</p>`;
                                }
                                return html;
            }),
            buildLayersPanelHTML: Pixel.bindRuntime(runtime, function buildLayersPanelHTML() {
                                const layerItems = layers.map((layer, i) => {
                                    const isActive = i === activeLayerIdx;
                                    return `<div class="pixel-layer-item${isActive ? ' active' : ''}" data-layer-idx="${i}">
                                        <button class="pixel-layer-vis" type="button" data-layer-vis="${i}" title="${esc(t('pixel.toggle_visibility', 'Toggle Visibility'))}">${layer.visible ? '👁' : '◻'}</button>
                                        <span class="pixel-layer-name" data-layer-name="${i}">${esc(layer.name)}</span>
                                        <input type="range" class="pixel-slider pixel-layer-opacity" data-layer-opacity="${i}" min="0" max="100" value="${Math.round(layer.opacity * 100)}" title="${esc(t('pixel.layer_opacity', 'Opacity'))}">
                                    </div>`;
                                }).reverse().join('');

                                return `<div class="pixel-panel-section pixel-panel-layers" data-section="layers" hidden>
                                    <div class="pixel-layer-list" data-layer-list>${layerItems}</div>
                                    <div class="pixel-layer-actions">
                                        <button class="pixel-btn-icon" type="button" data-action="add-layer" title="${esc(t('pixel.new_layer', 'Add Layer'))}">+</button>
                                        <button class="pixel-btn-icon" type="button" data-action="delete-layer" title="${esc(t('pixel.delete_layer', 'Delete Layer'))}">−</button>
                                        <button class="pixel-btn-icon" type="button" data-action="duplicate-layer" title="${esc(t('pixel.duplicate_layer', 'Duplicate'))}">⧉</button>
                                        <button class="pixel-btn-icon" type="button" data-action="merge-layers" title="${esc(t('pixel.merge_layers', 'Merge Down'))}">⤓</button>
                                        <button class="pixel-btn-icon" type="button" data-action="flatten-layers" title="${esc(t('pixel.flatten', 'Flatten'))}">▭</button>
                                    </div>
                                </div>`;
            }),
            buildPanelHTML: Pixel.bindRuntime(runtime, function buildPanelHTML() {
                                return `<div class="pixel-panel-section pixel-panel-adjust" data-section="adjust">
                                    ${buildSlider('brightness', t('pixel.brightness', 'Brightness'), -100, 100, 0)}
                                    ${buildSlider('contrast', t('pixel.contrast', 'Contrast'), -100, 100, 0)}
                                    ${buildSlider('saturation', t('pixel.saturation', 'Saturation'), -100, 100, 0)}
                                    ${buildSlider('exposure', t('pixel.exposure', 'Exposure'), -100, 100, 0)}
                                    ${buildSlider('sharpness', t('pixel.sharpness', 'Sharpness'), 0, 100, 0)}
                                    ${buildSlider('temperature', t('pixel.temperature', 'Temperature'), -100, 100, 0)}
                                    ${buildSlider('shadows', t('pixel.shadows', 'Shadows'), -100, 100, 0)}
                                    ${buildSlider('highlights', t('pixel.highlights', 'Highlights'), -100, 100, 0)}
                                    <div class="pixel-panel-actions"><button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-adjust">${esc(t('pixel.apply', 'Apply'))}</button><button class="pixel-btn" type="button" data-action="reset-adjust">${esc(t('pixel.reset', 'Reset'))}</button><button class="pixel-btn" type="button" data-action="compare-toggle">${esc(t('pixel.compare', 'Compare'))}</button></div>
                                </div>
                                <div class="pixel-panel-section pixel-panel-filters" data-section="filters" hidden>
                                    <div class="pixel-filter-grid" data-filter-grid>
                                        ${buildFilterCard('grayscale', 'Grayscale')}
                                        ${buildFilterCard('sepia', 'Sepia')}
                                        ${buildFilterCard('invert', 'Invert')}
                                        ${buildFilterCard('blur', 'Blur')}
                                        ${buildFilterCard('vintage', 'Vintage')}
                                        ${buildFilterCard('vignette', 'Vignette')}
                                        ${buildFilterCard('warm', 'Warm')}
                                        ${buildFilterCard('cool', 'Cool')}
                                        ${buildFilterCard('high-contrast', 'High Contrast')}
                                        ${buildFilterCard('emboss', 'Emboss')}
                                    </div>
                                </div>
                                <div class="pixel-panel-section pixel-panel-transform" data-section="transform" hidden>
                                    <div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="rotate-cw">${iconMarkup('redo', 'CW')} ${esc(t('pixel.rotate_cw', 'Rotate CW'))}</button><button class="pixel-btn" type="button" data-action="rotate-ccw">${iconMarkup('undo', 'CCW')} ${esc(t('pixel.rotate_ccw', 'Rotate CCW'))}</button></div>
                                    <div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="flip-h">${iconMarkup('sort', 'H')} ${esc(t('pixel.flip_h', 'Flip H'))}</button><button class="pixel-btn" type="button" data-action="flip-v">${iconMarkup('sort', 'V')} ${esc(t('pixel.flip_v', 'Flip V'))}</button></div>
                                    <hr class="pixel-divider">
                                    <button class="pixel-btn" type="button" data-action="crop">${iconMarkup('scissors', 'C')} ${esc(t('pixel.crop', 'Crop'))}</button>
                                    <div class="pixel-crop-actions" data-crop-actions hidden>
                                        <button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-crop">${esc(t('pixel.apply_crop', 'Apply Crop'))}</button>
                                        <button class="pixel-btn" type="button" data-action="cancel-crop">${esc(t('pixel.cancel_crop', 'Cancel'))}</button>
                                    </div>
                                    <hr class="pixel-divider">
                                    <button class="pixel-btn" type="button" data-action="resize">${iconMarkup('maximize', 'R')} ${esc(t('pixel.resize', 'Resize'))}</button>
                                </div>
                                ${buildDrawPanelHTML()}
                                ${buildLayersPanelHTML()}
                                <div class="pixel-panel-section pixel-panel-ai" data-section="ai" hidden>
                                    <div class="pixel-ai-status" data-ai-status></div>
                                    <div class="pixel-ai-panel">
                                        <label class="pixel-label">${esc(t('pixel.prompt', 'Prompt'))}</label>
                                        <textarea class="pixel-ai-prompt" data-ai-prompt rows="3" placeholder="${esc(t('pixel.prompt_placeholder', 'Describe the image to generate...'))}"></textarea>
                                        <label class="pixel-label">${esc(t('pixel.ai_negative_prompt', 'Negative Prompt'))}</label>
                                        <textarea class="pixel-ai-prompt" data-ai-negative rows="2" placeholder="${esc(t('pixel.negative_placeholder', 'What to exclude...'))}"></textarea>
                                        <div class="pixel-ai-options">
                                            <label class="pixel-label">${esc(t('pixel.ai_size', 'Size'))}</label>
                                            <select class="pixel-select" data-ai-size><option value="1024x1024">1024×1024</option><option value="1024x1792">1024×1792</option><option value="1792x1024">1792×1024</option><option value="512x512">512×512</option></select>
                                            <label class="pixel-label">${esc(t('pixel.ai_quality', 'Quality'))}</label>
                                            <select class="pixel-select" data-ai-quality><option value="standard">Standard</option><option value="hd">HD</option></select>
                                            <label class="pixel-label">${esc(t('pixel.ai_style', 'Style'))}</label>
                                            <select class="pixel-select" data-ai-style><option value="vivid">${esc(t('pixel.style_vivid', 'Vivid'))}</option><option value="natural">${esc(t('pixel.style_natural', 'Natural'))}</option></select>
                                        </div>
                                        <button class="pixel-btn pixel-btn-primary pixel-btn-full" type="button" data-action="ai-generate">${esc(t('pixel.generate', 'Generate'))}</button>
                                    </div>
                                    <hr class="pixel-divider">
                                    <div class="pixel-ai-panel">
                                        <label class="pixel-label">${esc(t('pixel.enhance', 'Enhance Image'))}</label>
                                        <textarea class="pixel-ai-prompt" data-enhance-prompt rows="2" placeholder="${esc(t('pixel.prompt_placeholder', 'Describe enhancements...'))}"></textarea>
                                        <label class="pixel-label">${esc(t('pixel.ai_strength', 'Strength'))} <span data-strength-val>0.7</span></label>
                                        <input type="range" class="pixel-slider" data-enhance-strength min="0.1" max="1" step="0.05" value="0.7">
                                        <button class="pixel-btn pixel-btn-full" type="button" data-action="ai-enhance">${esc(t('pixel.enhance', 'Enhance'))}</button>
                                    </div>
                                </div>`;
            }),
            buildSlider: Pixel.bindRuntime(runtime, function buildSlider(key, label, min, max, def) {
                                return `<div class="pixel-slider-row">
                                    <span class="pixel-slider-label">${esc(label)}</span>
                                    <input type="range" class="pixel-slider" data-adjust="${key}" min="${min}" max="${max}" value="${def}">
                                    <span class="pixel-slider-value" data-val-${key}>${def}</span>
                                </div>`;
            }),
            buildFilterCard: Pixel.bindRuntime(runtime, function buildFilterCard(key, label) {
                                return `<button class="pixel-filter-card" type="button" data-filter="${key}"><span class="pixel-filter-preview pixel-filter-${key}"></span><span class="pixel-filter-name">${esc(t('pixel.filter_' + key.replace('-', '_'), label))}</span></button>`;
            })
        });
    };
})();
