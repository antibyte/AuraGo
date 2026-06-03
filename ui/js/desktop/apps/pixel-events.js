(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installEvents = function installEvents(runtime) {
        Object.assign(runtime, {
            onCanvasMouseDown: Pixel.bindRuntime(runtime, function onCanvasMouseDown(e) {
                                if (!canvas.width || e.button === 1) return;
                                const isDrawingTool = activeTool && ['brush', 'eraser', 'pencil'].includes(activeTool);
                                const isShapeTool = activeTool && ['line', 'rectangle', 'ellipse', 'arrow'].includes(activeTool);
                                const isSelectionTool = activeTool && ['select-rect', 'select-ellipse'].includes(activeTool);
                                const coords = canvasCoords(e);

                                if (activeTool === 'eyedropper') {
                                    const pixel = cctx.getImageData(coords.x, coords.y, 1, 1).data;
                                    const hex = rgbToHex(pixel[0], pixel[1], pixel[2]);
                                    setPrimaryColor(hex);
                                    addRecentColor(hex);
                                    setActiveTool(null);
                                    return;
                                }

                                if (activeTool === 'fill') {
                                    floodFill(coords.x, coords.y, primaryColor);
                                    addRecentColor(primaryColor);
                                    pushHistory('fill');
                                    return;
                                }

                                if (activeTool === 'text') {
                                    showTextInput(coords.x, coords.y);
                                    return;
                                }

                                if (isDrawingTool) {
                                    isDrawing = true;
                                    lastDrawX = coords.x;
                                    lastDrawY = coords.y;
                                    const actx = getActiveCtx();
                                    const op = activeTool === 'eraser' ? 'destination-out' : 'source-over';
                                    const size = activeTool === 'pencil' ? 1 : brushSize;
                                    const opacity = activeTool === 'pencil' ? 100 : brushOpacity;
                                    const color = activeTool === 'eraser' ? '#000000' : primaryColor;
                                    drawBrushStroke(actx, coords.x, coords.y, coords.x, coords.y, size, opacity, color, op, brushHardness);
                                    if (layers.length > 1) compositeLayers();
                                    return;
                                }

                                if (isShapeTool) {
                                    isDrawing = true;
                                    drawStartX = coords.x;
                                    drawStartY = coords.y;
                                    return;
                                }

                                if (isSelectionTool) {
                                    if (selection && selImageData) {
                                        const inSel = coords.x >= selection.x && coords.x <= selection.x + selection.w &&
                                                      coords.y >= selection.y && coords.y <= selection.y + selection.h;
                                        if (inSel) {
                                            isMovingSel = true;
                                            selDragOfsX = coords.x - selection.x;
                                            selDragOfsY = coords.y - selection.y;
                                            return;
                                        }
                                    }
                                    deselect();
                                    isDrawing = true;
                                    drawStartX = coords.x;
                                    drawStartY = coords.y;
                                    return;
                                }
            }),
            onCanvasMouseMove: Pixel.bindRuntime(runtime, function onCanvasMouseMove(e) {
                                const coords = canvasCoords(e);

                                if (statusCursor && imgWidth) {
                                    statusCursor.textContent = `${coords.x}, ${coords.y}`;
                                }
                                if (statusColorSwatch && imgWidth && coords.x >= 0 && coords.x < imgWidth && coords.y >= 0 && coords.y < imgHeight) {
                                    const pixel = cctx.getImageData(Math.min(coords.x, imgWidth - 1), Math.min(coords.y, imgHeight - 1), 1, 1).data;
                                    const hex = rgbToHex(pixel[0], pixel[1], pixel[2]);
                                    if (statusColorSwatch) statusColorSwatch.style.background = hex;
                                    if (statusColorHex) statusColorHex.textContent = hex;
                                }

                                if (compareMode) {
                                    if (e.buttons === 1) {
                                        const rect = canvas.getBoundingClientRect();
                                        comparePos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
                                        renderCompare();
                                    }
                                    return;
                                }

                                const isDrawingTool = activeTool && ['brush', 'eraser', 'pencil'].includes(activeTool);
                                const isShapeTool = activeTool && ['line', 'rectangle', 'ellipse', 'arrow'].includes(activeTool);
                                const isSelectionTool = activeTool && ['select-rect', 'select-ellipse'].includes(activeTool);

                                if (isDrawing && isDrawingTool) {
                                    const actx = getActiveCtx();
                                    const op = activeTool === 'eraser' ? 'destination-out' : 'source-over';
                                    const size = activeTool === 'pencil' ? 1 : brushSize;
                                    const opacity = activeTool === 'pencil' ? 100 : brushOpacity;
                                    const color = activeTool === 'eraser' ? '#000000' : primaryColor;
                                    drawBrushStroke(actx, lastDrawX, lastDrawY, coords.x, coords.y, size, opacity, color, op, brushHardness);
                                    if (layers.length > 1) compositeLayers();
                                    lastDrawX = coords.x;
                                    lastDrawY = coords.y;
                                    return;
                                }

                                if (isDrawing && isShapeTool) {
                                    drawShapePreview(drawStartX, drawStartY, coords.x, coords.y);
                                    return;
                                }

                                if (isDrawing && isSelectionTool) {
                                    const sx = Math.min(drawStartX, coords.x);
                                    const sy = Math.min(drawStartY, coords.y);
                                    const sw = Math.abs(coords.x - drawStartX);
                                    const sh = Math.abs(coords.y - drawStartY);
                                    selection = { type: activeTool === 'select-rect' ? 'rect' : 'ellipse', x: sx, y: sy, w: sw, h: sh };
                                    startMarchingAnts();
                                    return;
                                }

                                if (isMovingSel && e.buttons === 1) {
                                    selection.x = coords.x - selDragOfsX;
                                    selection.y = coords.y - selDragOfsY;
                                    startMarchingAnts();
                                    return;
                                }
            }),
            onCanvasMouseUp: Pixel.bindRuntime(runtime, function onCanvasMouseUp(e) {
                                const coords = canvasCoords(e);
                                const isDrawingTool = activeTool && ['brush', 'eraser', 'pencil'].includes(activeTool);
                                const isShapeTool = activeTool && ['line', 'rectangle', 'ellipse', 'arrow'].includes(activeTool);
                                const isSelectionTool = activeTool && ['select-rect', 'select-ellipse'].includes(activeTool);

                                if (isDrawing && isDrawingTool) {
                                    isDrawing = false;
                                    addRecentColor(primaryColor);
                                    pushHistory('draw:' + activeTool);
                                    return;
                                }

                                if (isDrawing && isShapeTool) {
                                    isDrawing = false;
                                    commitShape(drawStartX, drawStartY, coords.x, coords.y);
                                    clearOverlay();
                                    addRecentColor(primaryColor);
                                    pushHistory('draw:' + activeTool);
                                    return;
                                }

                                if (isDrawing && isSelectionTool) {
                                    isDrawing = false;
                                    if (selection && selection.w > 2 && selection.h > 2) {
                                        const actx = getActiveCtx();
                                        const sx = Math.max(0, Math.round(Math.min(selection.x, selection.x + selection.w)));
                                        const sy = Math.max(0, Math.round(Math.min(selection.y, selection.y + selection.h)));
                                        const sw = Math.round(Math.abs(selection.w));
                                        const sh = Math.round(Math.abs(selection.h));
                                        selImageData = actx.getImageData(sx, sy, sw, sh);
                                    }
                                    return;
                                }

                                if (isMovingSel) {
                                    isMovingSel = false;
                                    return;
                                }
            }),
            showTextInput: Pixel.bindRuntime(runtime, function showTextInput(x, y) {
                                const inputWrap = document.createElement('div');
                                inputWrap.className = 'pixel-text-overlay';
                                const scale = zoom;
                                const rect = canvas.getBoundingClientRect();
                                const left = rect.left + x * scale - canvasArea.getBoundingClientRect().left + canvasArea.scrollLeft;
                                const top = rect.top + y * scale - canvasArea.getBoundingClientRect().top + canvasArea.scrollTop;
                                inputWrap.style.cssText = `position:absolute;left:${left}px;top:${top}px;z-index:25;`;
                                inputWrap.innerHTML = `<textarea class="pixel-text-field" style="font:${fontSize * scale}px ${fontFamily};color:${primaryColor};background:rgba(0,0,0,0.3);border:1px dashed var(--pixel-accent);outline:none;resize:both;min-width:60px;min-height:30px;padding:2px;" rows="2"></textarea>`;
                                canvasArea.appendChild(inputWrap);
                                const textarea = inputWrap.querySelector('textarea');
                                textarea.focus();
                                textarea.addEventListener('keydown', ev => {
                                    if (ev.key === 'Enter' && !ev.shiftKey) {
                                        ev.preventDefault();
                                        commitTextToCanvas(textarea.value, x, y);
                                        inputWrap.remove();
                                    }
                                    if (ev.key === 'Escape') { inputWrap.remove(); }
                                });
                                textarea.addEventListener('blur', () => {
                                    if (textarea.value) commitTextToCanvas(textarea.value, x, y);
                                    inputWrap.remove();
                                });
            }),
            onCropMouseDown: Pixel.bindRuntime(runtime, function onCropMouseDown(e) {
                                if (!cropState || !cropState.active) return;
                                e.preventDefault();
                                const rect = canvas.getBoundingClientRect();
                                cropState.startX = e.clientX - rect.left;
                                cropState.startY = e.clientY - rect.top;
                                cropState.endX = cropState.startX;
                                cropState.endY = cropState.startY;
                                cropState.canvasLeft = rect.left - canvasArea.getBoundingClientRect().left + canvasArea.scrollLeft;
                                cropState.canvasTop = rect.top - canvasArea.getBoundingClientRect().top + canvasArea.scrollTop;
                                updateCropSelection();
                                const onMove = ev => {
                                    cropState.endX = Math.max(0, Math.min(rect.width, ev.clientX - rect.left));
                                    cropState.endY = Math.max(0, Math.min(rect.height, ev.clientY - rect.top));
                                    updateCropSelection();
                                };
                                const onUp = () => { document.removeEventListener('mousemove', onMove); document.removeEventListener('mouseup', onUp); };
                                document.addEventListener('mousemove', onMove);
                                document.addEventListener('mouseup', onUp);
            }),
            updateCropSelection: Pixel.bindRuntime(runtime, function updateCropSelection() {
                                const sel = host.querySelector('[data-crop-sel]');
                                if (!sel || !cropState) return;
                                const x = Math.min(cropState.startX, cropState.endX) + (cropState.canvasLeft || 0);
                                const y = Math.min(cropState.startY, cropState.endY) + (cropState.canvasTop || 0);
                                const w = Math.abs(cropState.endX - cropState.startX);
                                const h = Math.abs(cropState.endY - cropState.startY);
                                sel.style.left = x + 'px';
                                sel.style.top = y + 'px';
                                sel.style.width = w + 'px';
                                sel.style.height = h + 'px';
            }),
            showShortcutsModal: Pixel.bindRuntime(runtime, function showShortcutsModal() {
                                const shortcuts = [
                                    [t('pixel.shortcuts_file', 'File'), [
                                        ['Ctrl+O', t('pixel.open', 'Open')],
                                        ['Ctrl+S', t('pixel.save', 'Save')],
                                        ['Ctrl+Shift+S', t('pixel.save_as', 'Save As')]
                                    ]],
                                    [t('pixel.shortcuts_edit', 'Edit'), [
                                        ['Ctrl+Z', t('pixel.undo', 'Undo')],
                                        ['Ctrl+Shift+Z', t('pixel.redo', 'Redo')],
                                        ['Ctrl+C', t('pixel.copy', 'Copy')],
                                        ['Ctrl+V', t('pixel.paste', 'Paste')],
                                        ['Ctrl+X', t('pixel.cut', 'Cut')],
                                        ['Delete', t('pixel.delete_selection', 'Delete Selection')],
                                        ['Ctrl+A', t('pixel.select_all', 'Select All')],
                                        ['Ctrl+D', t('pixel.deselect', 'Deselect')]
                                    ]],
                                    [t('pixel.shortcuts_view', 'View'), [
                                        ['Ctrl++', t('pixel.zoom_in', 'Zoom In')],
                                        ['Ctrl+-', t('pixel.zoom_out', 'Zoom Out')],
                                        ['Ctrl+0', t('pixel.zoom_fit', 'Fit to Window')],
                                        ['Space+Drag', t('pixel.pan', 'Pan')],
                                        ['Ctrl+Wheel', t('pixel.scroll_zoom', 'Scroll Zoom')]
                                    ]],
                                    [t('pixel.shortcuts_tools', 'Tools'), [
                                        ['B', t('pixel.brush', 'Brush')],
                                        ['E', t('pixel.eraser', 'Eraser')],
                                        ['L', t('pixel.line', 'Line')],
                                        ['R', t('pixel.rectangle', 'Rectangle')],
                                        ['O', t('pixel.ellipse', 'Ellipse')],
                                        ['T', t('pixel.text', 'Text')],
                                        ['G', t('pixel.fill', 'Fill')],
                                        ['I', t('pixel.eyedropper', 'Eyedropper')],
                                        ['V', t('pixel.select_rect', 'Rectangle Select')],
                                        ['?', t('pixel.shortcuts', 'Shortcuts')]
                                    ]]
                                ];
                                const dlg = document.createElement('div');
                                dlg.className = 'vd-modal-backdrop';
                                dlg.innerHTML = `<div class="vd-modal pixel-shortcuts-modal" role="dialog">
                                    <div class="vd-modal-title">${esc(t('pixel.keyboard_shortcuts', 'Keyboard Shortcuts'))}</div>
                                    <div class="pixel-shortcuts-body">${shortcuts.map(([cat, items]) =>
                                        `<div class="pixel-shortcut-group"><h4 class="pixel-shortcut-cat">${esc(cat)}</h4>${items.map(([key, desc]) =>
                                            `<div class="pixel-shortcut-row"><kbd class="pixel-kbd">${esc(key)}</kbd><span>${esc(desc)}</span></div>`
                                        ).join('')}</div>`
                                    ).join('')}</div>
                                    <div class="vd-modal-actions"><button type="button" class="vd-button vd-button-primary" data-close>${esc(t('pixel.close', 'Close'))}</button></div>
                                </div>`;
                                document.body.appendChild(dlg);
                                dlg.querySelector('[data-close]').addEventListener('click', () => dlg.remove());
                                dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });
            }),
            showContextMenu: Pixel.bindRuntime(runtime, function showContextMenu(e) {
                                e.preventDefault();
                                const existing = host.querySelector('.pixel-ctx-menu');
                                if (existing) existing.remove();
                                const menu = document.createElement('div');
                                menu.className = 'pixel-ctx-menu';
                                const items = [];
                                if (canvas.width) {
                                    items.push({ label: t('pixel.copy', 'Copy'), shortcut: 'Ctrl+C', action: copySelection });
                                    items.push({ label: t('pixel.paste', 'Paste'), shortcut: 'Ctrl+V', action: pasteClipboard });
                                    items.push({ type: 'sep' });
                                    items.push({ label: t('pixel.select_all', 'Select All'), shortcut: 'Ctrl+A', action: selectAll });
                                    items.push({ label: t('pixel.deselect', 'Deselect'), shortcut: 'Ctrl+D', action: deselect });
                                    items.push({ type: 'sep' });
                                    items.push({ label: t('pixel.zoom_in', 'Zoom In'), shortcut: 'Ctrl++', action: () => zoomTo(zoom * 1.25) });
                                    items.push({ label: t('pixel.zoom_out', 'Zoom Out'), shortcut: 'Ctrl+-', action: () => zoomTo(zoom / 1.25) });
                                    items.push({ label: t('pixel.zoom_fit', 'Fit'), shortcut: 'Ctrl+0', action: zoomFit });
                                    items.push({ label: t('pixel.zoom_100', '100%'), action: () => zoomTo(1) });
                                    items.push({ type: 'sep' });
                                    items.push({ label: `${imgWidth} × ${imgHeight}`, disabled: true });
                                }
                                menu.innerHTML = items.map(item => {
                                    if (item.type === 'sep') return '<hr class="pixel-ctx-sep">';
                                    return `<button class="pixel-ctx-item${item.disabled ? ' disabled' : ''}" type="button" ${item.disabled ? 'disabled' : ''}>${esc(item.label)}${item.shortcut ? `<span class="pixel-ctx-shortcut">${esc(item.shortcut)}</span>` : ''}</button>`;
                                }).join('');
                                const rect = canvasArea.getBoundingClientRect();
                                menu.style.cssText = `position:absolute;left:${e.clientX - rect.left + canvasArea.scrollLeft}px;top:${e.clientY - rect.top + canvasArea.scrollTop}px;z-index:100;`;
                                canvasArea.appendChild(menu);
                                const btns = menu.querySelectorAll('.pixel-ctx-item:not([disabled])');
                                const actionItems = items.filter(i => !i.type && !i.disabled);
                                btns.forEach((btn, idx) => {
                                    btn.addEventListener('click', () => { if (actionItems[idx] && actionItems[idx].action) actionItems[idx].action(); menu.remove(); });
                                });
                                const closeMenu = ev => { if (!menu.contains(ev.target)) { menu.remove(); document.removeEventListener('mousedown', closeMenu); } };
                                setTimeout(() => document.addEventListener('mousedown', closeMenu), 0);
            }),
            wireDrawOptionEvents: Pixel.bindRuntime(runtime, function wireDrawOptionEvents() {
                                const sizeSlider = host.querySelector('[data-brush-size]');
                                if (sizeSlider) {
                                    sizeSlider.addEventListener('input', () => { brushSize = Number(sizeSlider.value); const el = host.querySelector('[data-brush-size-val]'); if (el) el.textContent = brushSize + 'px'; });
                                }
                                const opacitySlider = host.querySelector('[data-brush-opacity]');
                                if (opacitySlider) {
                                    opacitySlider.addEventListener('input', () => { brushOpacity = Number(opacitySlider.value); const el = host.querySelector('[data-brush-opacity-val]'); if (el) el.textContent = brushOpacity + '%'; });
                                }
                                const hardnessSlider = host.querySelector('[data-brush-hardness]');
                                if (hardnessSlider) {
                                    hardnessSlider.addEventListener('input', () => { brushHardness = Number(hardnessSlider.value); const el = host.querySelector('[data-brush-hardness-val]'); if (el) el.textContent = brushHardness + '%'; });
                                }
                                const strokeSlider = host.querySelector('[data-shape-stroke]');
                                if (strokeSlider) {
                                    strokeSlider.addEventListener('input', () => { shapeStrokeWidth = Number(strokeSlider.value); const el = host.querySelector('[data-shape-stroke-val]'); if (el) el.textContent = shapeStrokeWidth + 'px'; });
                                }
                                const fillCheck = host.querySelector('[data-shape-fill]');
                                if (fillCheck) {
                                    fillCheck.addEventListener('change', () => { shapeFill = fillCheck.checked; });
                                }
                                const fsSlider = host.querySelector('[data-font-size]');
                                if (fsSlider) {
                                    fsSlider.addEventListener('input', () => { fontSize = Number(fsSlider.value); const el = host.querySelector('[data-font-size-val]'); if (el) el.textContent = fontSize + 'px'; });
                                }
                                const ffSelect = host.querySelector('[data-font-family]');
                                if (ffSelect) {
                                    ffSelect.addEventListener('change', () => { fontFamily = ffSelect.value; });
                                }
                                const tolSlider = host.querySelector('[data-fill-tolerance]');
                                if (tolSlider) {
                                    tolSlider.addEventListener('input', () => { fillTolerance = Number(tolSlider.value); const el = host.querySelector('[data-fill-tolerance-val]'); if (el) el.textContent = fillTolerance; });
                                }
            })
        });
    };
})();
