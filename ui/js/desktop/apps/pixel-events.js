(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installEvents = function installEvents(runtime) {
        Object.assign(runtime, {
            onCanvasMouseDown: Pixel.bindRuntime(runtime, function onCanvasMouseDown(e) {
                                if (!this.canvas.width || e.button === 1) return;
                                const isDrawingTool = this.activeTool && ['brush', 'eraser', 'pencil'].includes(this.activeTool);
                                const isShapeTool = this.activeTool && ['line', 'rectangle', 'ellipse', 'arrow'].includes(this.activeTool);
                                const isSelectionTool = this.activeTool && ['select-rect', 'select-ellipse'].includes(this.activeTool);
                                const coords = this.canvasCoords(e);

                                if (this.activeTool === 'eyedropper') {
                                    const pixel = this.cctx.getImageData(coords.x, coords.y, 1, 1).data;
                                    const hex = this.rgbToHex(pixel[0], pixel[1], pixel[2]);
                                    this.setPrimaryColor(hex);
                                    this.addRecentColor(hex);
                                    this.setActiveTool(null);
                                    return;
                                }

                                if (this.activeTool === 'fill') {
                                    this.floodFill(coords.x, coords.y, this.primaryColor);
                                    this.addRecentColor(this.primaryColor);
                                    this.pushHistory('fill');
                                    return;
                                }

                                if (this.activeTool === 'text') {
                                    showTextInput(coords.x, coords.y);
                                    return;
                                }

                                if (isDrawingTool) {
                                    this.isDrawing = true;
                                    this.lastDrawX = coords.x;
                                    this.lastDrawY = coords.y;
                                    const actx = this.getActiveCtx();
                                    const op = this.activeTool === 'eraser' ? 'destination-out' : 'source-over';
                                    const size = this.activeTool === 'pencil' ? 1 : this.brushSize;
                                    const opacity = this.activeTool === 'pencil' ? 100 : this.brushOpacity;
                                    const color = this.activeTool === 'eraser' ? '#000000' : this.primaryColor;
                                    this.drawBrushStroke(actx, coords.x, coords.y, coords.x, coords.y, size, opacity, color, op, this.brushHardness);
                                    if (this.layers.length > 1) this.compositeLayers();
                                    return;
                                }

                                if (isShapeTool) {
                                    this.isDrawing = true;
                                    this.drawStartX = coords.x;
                                    this.drawStartY = coords.y;
                                    return;
                                }

                                if (isSelectionTool) {
                                    if (this.selection && this.selImageData) {
                                        const inSel = coords.x >= this.selection.x && coords.x <= this.selection.x + this.selection.w &&
                                                      coords.y >= this.selection.y && coords.y <= this.selection.y + this.selection.h;
                                        if (inSel) {
                                            this.isMovingSel = true;
                                            this.selDragOfsX = coords.x - this.selection.x;
                                            this.selDragOfsY = coords.y - this.selection.y;
                                            return;
                                        }
                                    }
                                    this.deselect();
                                    this.isDrawing = true;
                                    this.drawStartX = coords.x;
                                    this.drawStartY = coords.y;
                                    return;
                                }
            }),
            onCanvasMouseMove: Pixel.bindRuntime(runtime, function onCanvasMouseMove(e) {
                                const coords = this.canvasCoords(e);

                                if (this.statusCursor && this.imgWidth) {
                                    this.statusCursor.textContent = `${coords.x}, ${coords.y}`;
                                }
                                if (this.statusColorSwatch && this.imgWidth && coords.x >= 0 && coords.x < this.imgWidth && coords.y >= 0 && coords.y < this.imgHeight) {
                                    const pixel = this.cctx.getImageData(Math.min(coords.x, this.imgWidth - 1), Math.min(coords.y, this.imgHeight - 1), 1, 1).data;
                                    const hex = this.rgbToHex(pixel[0], pixel[1], pixel[2]);
                                    if (this.statusColorSwatch) this.statusColorSwatch.style.background = hex;
                                    if (this.statusColorHex) this.statusColorHex.textContent = hex;
                                }

                                if (this.compareMode) {
                                    if (e.buttons === 1) {
                                        const rect = this.canvas.getBoundingClientRect();
                                        this.comparePos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
                                        this.renderCompare();
                                    }
                                    return;
                                }

                                const isDrawingTool = this.activeTool && ['brush', 'eraser', 'pencil'].includes(this.activeTool);
                                const isShapeTool = this.activeTool && ['line', 'rectangle', 'ellipse', 'arrow'].includes(this.activeTool);
                                const isSelectionTool = this.activeTool && ['select-rect', 'select-ellipse'].includes(this.activeTool);

                                if (this.isDrawing && isDrawingTool) {
                                    const actx = this.getActiveCtx();
                                    const op = this.activeTool === 'eraser' ? 'destination-out' : 'source-over';
                                    const size = this.activeTool === 'pencil' ? 1 : this.brushSize;
                                    const opacity = this.activeTool === 'pencil' ? 100 : this.brushOpacity;
                                    const color = this.activeTool === 'eraser' ? '#000000' : this.primaryColor;
                                    this.drawBrushStroke(actx, this.lastDrawX, this.lastDrawY, coords.x, coords.y, size, opacity, color, op, this.brushHardness);
                                    if (this.layers.length > 1) this.compositeLayers();
                                    this.lastDrawX = coords.x;
                                    this.lastDrawY = coords.y;
                                    return;
                                }

                                if (this.isDrawing && isShapeTool) {
                                    this.drawShapePreview(this.drawStartX, this.drawStartY, coords.x, coords.y);
                                    return;
                                }

                                if (this.isDrawing && isSelectionTool) {
                                    const sx = Math.min(this.drawStartX, coords.x);
                                    const sy = Math.min(this.drawStartY, coords.y);
                                    const sw = Math.abs(coords.x - this.drawStartX);
                                    const sh = Math.abs(coords.y - this.drawStartY);
                                    this.selection = { type: this.activeTool === 'select-rect' ? 'rect' : 'ellipse', x: sx, y: sy, w: sw, h: sh };
                                    this.startMarchingAnts();
                                    return;
                                }

                                if (this.isMovingSel && e.buttons === 1) {
                                    this.selection.x = coords.x - this.selDragOfsX;
                                    this.selection.y = coords.y - this.selDragOfsY;
                                    this.startMarchingAnts();
                                    return;
                                }
            }),
            onCanvasMouseUp: Pixel.bindRuntime(runtime, function onCanvasMouseUp(e) {
                                const coords = this.canvasCoords(e);
                                const isDrawingTool = this.activeTool && ['brush', 'eraser', 'pencil'].includes(this.activeTool);
                                const isShapeTool = this.activeTool && ['line', 'rectangle', 'ellipse', 'arrow'].includes(this.activeTool);
                                const isSelectionTool = this.activeTool && ['select-rect', 'select-ellipse'].includes(this.activeTool);

                                if (this.isDrawing && isDrawingTool) {
                                    this.isDrawing = false;
                                    this.addRecentColor(this.primaryColor);
                                    this.pushHistory('draw:' + this.activeTool);
                                    return;
                                }

                                if (this.isDrawing && isShapeTool) {
                                    this.isDrawing = false;
                                    this.commitShape(this.drawStartX, this.drawStartY, coords.x, coords.y);
                                    this.clearOverlay();
                                    this.addRecentColor(this.primaryColor);
                                    this.pushHistory('draw:' + this.activeTool);
                                    return;
                                }

                                if (this.isDrawing && isSelectionTool) {
                                    this.isDrawing = false;
                                    if (this.selection && this.selection.w > 2 && this.selection.h > 2) {
                                        const actx = this.getActiveCtx();
                                        const sx = Math.max(0, Math.round(Math.min(this.selection.x, this.selection.x + this.selection.w)));
                                        const sy = Math.max(0, Math.round(Math.min(this.selection.y, this.selection.y + this.selection.h)));
                                        const sw = Math.round(Math.abs(this.selection.w));
                                        const sh = Math.round(Math.abs(this.selection.h));
                                        this.selImageData = actx.getImageData(sx, sy, sw, sh);
                                    }
                                    return;
                                }

                                if (this.isMovingSel) {
                                    this.isMovingSel = false;
                                    return;
                                }
            }),
            showTextInput: Pixel.bindRuntime(runtime, function showTextInput(x, y) {
                                const inputWrap = document.createElement('div');
                                inputWrap.className = 'pixel-text-overlay';
                                const scale = this.zoom;
                                const rect = this.canvas.getBoundingClientRect();
                                const left = rect.left + x * scale - this.canvasArea.getBoundingClientRect().left + this.canvasArea.scrollLeft;
                                const top = rect.top + y * scale - this.canvasArea.getBoundingClientRect().top + this.canvasArea.scrollTop;
                                inputWrap.style.cssText = `position:absolute;left:${left}px;top:${top}px;z-index:25;`;
                                inputWrap.innerHTML = `<textarea class="pixel-text-field" style="font:${this.fontSize * scale}px ${this.fontFamily};color:${this.primaryColor};background:rgba(0,0,0,0.3);border:1px dashed var(--pixel-accent);outline:none;resize:both;min-width:60px;min-height:30px;padding:2px;" rows="2"></textarea>`;
                                this.canvasArea.appendChild(inputWrap);
                                const textarea = inputWrap.querySelector('textarea');
                                textarea.focus();
                                const stopTextShortcutPropagation = ev => {
                                    ev.stopPropagation();
                                    ev.stopImmediatePropagation();
                                };
                                textarea.addEventListener('keydown', ev => {
                                    stopTextShortcutPropagation(ev);
                                    if (ev.key === 'Enter' && !ev.shiftKey) {
                                        ev.preventDefault();
                                        this.commitTextToCanvas(textarea.value, x, y);
                                        inputWrap.remove();
                                    }
                                    if (ev.key === 'Escape') { inputWrap.remove(); }
                                });
                                textarea.addEventListener('keyup', stopTextShortcutPropagation);
                                textarea.addEventListener('keypress', stopTextShortcutPropagation);
                                textarea.addEventListener('blur', () => {
                                    if (textarea.value) this.commitTextToCanvas(textarea.value, x, y);
                                    inputWrap.remove();
                                });
            }),
            onCropMouseDown: Pixel.bindRuntime(runtime, function onCropMouseDown(e) {
                                if (!this.cropState || !this.cropState.active) return;
                                e.preventDefault();
                                const rect = this.canvas.getBoundingClientRect();
                                this.cropState.startX = e.clientX - rect.left;
                                this.cropState.startY = e.clientY - rect.top;
                                this.cropState.endX = this.cropState.startX;
                                this.cropState.endY = this.cropState.startY;
                                this.cropState.canvasLeft = rect.left - this.canvasArea.getBoundingClientRect().left + this.canvasArea.scrollLeft;
                                this.cropState.canvasTop = rect.top - this.canvasArea.getBoundingClientRect().top + this.canvasArea.scrollTop;
                                this.updateCropSelection();
                                const onMove = ev => {
                                    this.cropState.endX = Math.max(0, Math.min(rect.width, ev.clientX - rect.left));
                                    this.cropState.endY = Math.max(0, Math.min(rect.height, ev.clientY - rect.top));
                                    this.updateCropSelection();
                                };
                                const onUp = () => { document.removeEventListener('mousemove', onMove); document.removeEventListener('mouseup', onUp); };
                                document.addEventListener('mousemove', onMove);
                                document.addEventListener('mouseup', onUp);
            }),
            updateCropSelection: Pixel.bindRuntime(runtime, function updateCropSelection() {
                                const sel = this.host.querySelector('[data-crop-sel]');
                                if (!sel || !this.cropState) return;
                                const x = Math.min(this.cropState.startX, this.cropState.endX) + (this.cropState.canvasLeft || 0);
                                const y = Math.min(this.cropState.startY, this.cropState.endY) + (this.cropState.canvasTop || 0);
                                const w = Math.abs(this.cropState.endX - this.cropState.startX);
                                const h = Math.abs(this.cropState.endY - this.cropState.startY);
                                sel.style.left = x + 'px';
                                sel.style.top = y + 'px';
                                sel.style.width = w + 'px';
                                sel.style.height = h + 'px';
            }),
            showShortcutsModal: Pixel.bindRuntime(runtime, function showShortcutsModal() {
                                const shortcuts = [
                                    [this.t('pixel.shortcuts_file'), [
                                        ['Ctrl+O', this.t('pixel.open')],
                                        ['Ctrl+S', this.t('pixel.save')],
                                        ['Ctrl+Shift+S', this.t('pixel.save_as')]
                                    ]],
                                    [this.t('pixel.shortcuts_edit'), [
                                        ['Ctrl+Z', this.t('pixel.undo')],
                                        ['Ctrl+Shift+Z', this.t('pixel.redo')],
                                        ['Ctrl+C', this.t('pixel.copy')],
                                        ['Ctrl+V', this.t('pixel.paste')],
                                        ['Ctrl+X', this.t('pixel.cut')],
                                        ['Delete', this.t('pixel.delete_selection')],
                                        ['Ctrl+A', this.t('pixel.select_all')],
                                        ['Ctrl+D', this.t('pixel.deselect')]
                                    ]],
                                    [this.t('pixel.shortcuts_view'), [
                                        ['Ctrl++', this.t('pixel.zoom_in')],
                                        ['Ctrl+-', this.t('pixel.zoom_out')],
                                        ['Ctrl+0', this.t('pixel.zoom_fit')],
                                        ['Space+Drag', this.t('pixel.pan')],
                                        ['Ctrl+Wheel', this.t('pixel.scroll_zoom')]
                                    ]],
                                    [this.t('pixel.shortcuts_tools'), [
                                        ['B', this.t('pixel.brush')],
                                        ['E', this.t('pixel.eraser')],
                                        ['L', this.t('pixel.line')],
                                        ['R', this.t('pixel.rectangle')],
                                        ['O', this.t('pixel.ellipse')],
                                        ['T', this.t('pixel.text')],
                                        ['G', this.t('pixel.fill')],
                                        ['I', this.t('pixel.eyedropper')],
                                        ['V', this.t('pixel.select_rect')],
                                        ['?', this.t('pixel.shortcuts')]
                                    ]]
                                ];
                                const dlg = document.createElement('div');
                                dlg.className = 'vd-modal-backdrop';
                                dlg.innerHTML = `<div class="vd-modal pixel-shortcuts-modal" role="dialog">
                                    <div class="vd-modal-title">${this.esc(this.t('pixel.keyboard_shortcuts'))}</div>
                                    <div class="pixel-shortcuts-body">${shortcuts.map(([cat, items]) =>
                                        `<div class="pixel-shortcut-group"><h4 class="pixel-shortcut-cat">${this.esc(cat)}</h4>${items.map(([key, desc]) =>
                                            `<div class="pixel-shortcut-row"><kbd class="pixel-kbd">${this.esc(key)}</kbd><span>${this.esc(desc)}</span></div>`
                                        ).join('')}</div>`
                                    ).join('')}</div>
                                    <div class="vd-modal-actions"><button type="button" class="vd-button vd-button-primary" data-close>${this.esc(this.t('pixel.close'))}</button></div>
                                </div>`;
                                document.body.appendChild(dlg);
                                dlg.querySelector('[data-close]').addEventListener('click', () => dlg.remove());
                                dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });
            }),
            showContextMenu: Pixel.bindRuntime(runtime, function showContextMenu(e) {
                                e.preventDefault();
                                const existing = this.host.querySelector('.pixel-ctx-menu');
                                if (existing) existing.remove();
                                const menu = document.createElement('div');
                                menu.className = 'pixel-ctx-menu';
                                const items = [];
                                if (this.canvas.width) {
                                    items.push({ label: this.t('pixel.copy'), shortcut: 'Ctrl+C', action: this.copySelection });
                                    items.push({ label: this.t('pixel.paste'), shortcut: 'Ctrl+V', action: this.pasteClipboard });
                                    items.push({ type: 'sep' });
                                    items.push({ label: this.t('pixel.select_all'), shortcut: 'Ctrl+A', action: this.selectAll });
                                    items.push({ label: this.t('pixel.deselect'), shortcut: 'Ctrl+D', action: this.deselect });
                                    items.push({ type: 'sep' });
                                    items.push({ label: this.t('pixel.zoom_in'), shortcut: 'Ctrl++', action: () => this.zoomTo(this.zoom * 1.25) });
                                    items.push({ label: this.t('pixel.zoom_out'), shortcut: 'Ctrl+-', action: () => this.zoomTo(this.zoom / 1.25) });
                                    items.push({ label: this.t('pixel.zoom_fit'), shortcut: 'Ctrl+0', action: this.zoomFit });
                                    items.push({ label: this.t('pixel.zoom_100'), action: () => this.zoomTo(1) });
                                    items.push({ type: 'sep' });
                                    items.push({ label: `${this.imgWidth} × ${this.imgHeight}`, disabled: true });
                                }
                                menu.innerHTML = items.map(item => {
                                    if (item.type === 'sep') return '<hr class="pixel-ctx-sep">';
                                    return `<button class="pixel-ctx-item${item.disabled ? ' disabled' : ''}" type="button" ${item.disabled ? 'disabled' : ''}>${this.esc(item.label)}${item.shortcut ? `<span class="pixel-ctx-shortcut">${this.esc(item.shortcut)}</span>` : ''}</button>`;
                                }).join('');
                                const rect = this.canvasArea.getBoundingClientRect();
                                menu.style.cssText = `position:absolute;left:${e.clientX - rect.left + this.canvasArea.scrollLeft}px;top:${e.clientY - rect.top + this.canvasArea.scrollTop}px;z-index:100;`;
                                this.canvasArea.appendChild(menu);
                                const btns = menu.querySelectorAll('.pixel-ctx-item:not([disabled])');
                                const actionItems = items.filter(i => !i.type && !i.disabled);
                                btns.forEach((btn, idx) => {
                                    btn.addEventListener('click', () => { if (actionItems[idx] && actionItems[idx].action) actionItems[idx].action(); menu.remove(); });
                                });
                                const closeMenu = ev => { if (!menu.contains(ev.target)) { menu.remove(); document.removeEventListener('mousedown', closeMenu); } };
                                setTimeout(() => document.addEventListener('mousedown', closeMenu), 0);
            }),
            wireDrawOptionEvents: Pixel.bindRuntime(runtime, function wireDrawOptionEvents() {
                                const sizeSlider = this.host.querySelector('[data-brush-size]');
                                if (sizeSlider) {
                                    sizeSlider.addEventListener('input', () => { this.brushSize = Number(sizeSlider.value); const el = this.host.querySelector('[data-brush-size-val]'); if (el) el.textContent = this.brushSize + 'px'; });
                                }
                                const opacitySlider = this.host.querySelector('[data-brush-opacity]');
                                if (opacitySlider) {
                                    opacitySlider.addEventListener('input', () => { this.brushOpacity = Number(opacitySlider.value); const el = this.host.querySelector('[data-brush-opacity-val]'); if (el) el.textContent = this.brushOpacity + '%'; });
                                }
                                const hardnessSlider = this.host.querySelector('[data-brush-hardness]');
                                if (hardnessSlider) {
                                    hardnessSlider.addEventListener('input', () => { this.brushHardness = Number(hardnessSlider.value); const el = this.host.querySelector('[data-brush-hardness-val]'); if (el) el.textContent = this.brushHardness + '%'; });
                                }
                                const strokeSlider = this.host.querySelector('[data-shape-stroke]');
                                if (strokeSlider) {
                                    strokeSlider.addEventListener('input', () => { this.shapeStrokeWidth = Number(strokeSlider.value); const el = this.host.querySelector('[data-shape-stroke-val]'); if (el) el.textContent = this.shapeStrokeWidth + 'px'; });
                                }
                                const fillCheck = this.host.querySelector('[data-shape-fill]');
                                if (fillCheck) {
                                    fillCheck.addEventListener('change', () => { this.shapeFill = fillCheck.checked; });
                                }
                                const fsSlider = this.host.querySelector('[data-font-size]');
                                if (fsSlider) {
                                    fsSlider.addEventListener('input', () => { this.fontSize = Number(fsSlider.value); const el = this.host.querySelector('[data-font-size-val]'); if (el) el.textContent = this.fontSize + 'px'; });
                                }
                                const ffSelect = this.host.querySelector('[data-font-family]');
                                if (ffSelect) {
                                    ffSelect.addEventListener('change', () => { this.fontFamily = ffSelect.value; });
                                }
                                const tolSlider = this.host.querySelector('[data-fill-tolerance]');
                                if (tolSlider) {
                                    tolSlider.addEventListener('input', () => { this.fillTolerance = Number(tolSlider.value); const el = this.host.querySelector('[data-fill-tolerance-val]'); if (el) el.textContent = this.fillTolerance; });
                                }
            })
        });
    };
})();
