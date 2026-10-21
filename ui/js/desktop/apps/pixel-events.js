(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const STROKE_TOOLS = ['brush', 'eraser', 'pencil', 'clone-stamp'];
    const POINT_TOOLS = ['airbrush', 'dodge-burn', 'blur-brush'];
    const SHAPE_TOOLS = ['line', 'rectangle', 'ellipse', 'arrow', 'gradient'];
    const SELECTION_TOOLS = ['select-rect', 'select-ellipse'];

    // Runs a registered tool's handler (onDown, onMove, ...) with the event and the pointer info; a broken tool must not break the editor.
    function runToolHandler(runtime, def, hook, e) {
        try {
            return def[hook](runtime, e, runtime.pointerInfo(e));
        } catch (err) {
            if (window.console) console.error('pixel tool ' + def.id + '.' + hook, err);
            return undefined;
        }
    }

    Pixel.installEvents = function installEvents(runtime) {
        Object.assign(runtime, {
            snapEnd: Pixel.bindRuntime(runtime, function snapEnd(x0, y0, x1, y1) {
                const dx = x1 - x0, dy = y1 - y0;
                if (this.activeTool === 'line' || this.activeTool === 'arrow' || this.activeTool === 'gradient') {
                    const len = Math.hypot(dx, dy);
                    if (!len) return { x: x1, y: y1 };
                    const ang = Math.round(Math.atan2(dy, dx) / (Math.PI / 4)) * (Math.PI / 4);
                    return { x: Math.round(x0 + len * Math.cos(ang)), y: Math.round(y0 + len * Math.sin(ang)) };
                }
                const m = Math.max(Math.abs(dx), Math.abs(dy));
                return { x: x0 + m * (dx < 0 ? -1 : 1), y: y0 + m * (dy < 0 ? -1 : 1) };
            }),
            // Brush, pencil and eraser parameters. In mask-edit mode strokes paint alpha: opaque white reveals, the eraser hides.
            brushStrokeSpec: Pixel.bindRuntime(runtime, function brushStrokeSpec() {
                const tool = this.activeTool;
                const layer = this.activeLayer();
                const inMask = !!(this.editMaskMode && layer && layer.maskCanvas);
                let op = tool === 'eraser' ? 'destination-out' : 'source-over';
                if (op === 'source-over' && !inMask && layer && layer.lockAlpha) op = 'source-atop';
                return {
                    op,
                    size: tool === 'pencil' ? 1 : this.brushSize,
                    opacity: tool === 'pencil' ? 100 : this.brushOpacity,
                    color: tool === 'eraser' ? '#000000' : (inMask ? '#ffffff' : this.primaryColor)
                };
            }),
            // Ring of the built-in brush tools: remembers the pointer, the 'cursor' overlay (z 90) paints it.
            drawBrushCursor: Pixel.bindRuntime(runtime, function drawBrushCursor(x, y) {
                this.cursorPos = { x, y };
                this.requestOverlay();
            }),
            paintBrushCursor: Pixel.bindRuntime(runtime, function paintBrushCursor(ctx) {
                const pos = this.cursorPos;
                const def = this.activeToolDef();
                if (!pos || !def || def.cursor !== 'none' || Pixel.toolHasHandlers(def)) return;
                const size = this.activeTool === 'pencil' ? 2 : this.brushSize;
                const r = Math.max(1, size / 2);
                ctx.strokeStyle = 'rgba(0,0,0,0.7)';
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.arc(pos.x, pos.y, r + 1, 0, Math.PI * 2);
                ctx.stroke();
                ctx.strokeStyle = 'rgba(255,255,255,0.9)';
                ctx.lineWidth = 1;
                ctx.beginPath();
                ctx.arc(pos.x, pos.y, r, 0, Math.PI * 2);
                ctx.stroke();
            }),
            // Pointer position in document coordinates (floats) plus the modifier state, as handed to registered tools.
            pointerInfo: Pixel.bindRuntime(runtime, function pointerInfo(e) {
                const rect = this.canvas.getBoundingClientRect();
                const sx = rect.width ? this.canvas.width / rect.width : 1;
                const sy = rect.height ? this.canvas.height / rect.height : 1;
                return {
                    x: (e.clientX - rect.left) * sx,
                    y: (e.clientY - rect.top) * sy,
                    pressure: typeof e.pressure === 'number' ? e.pressure : 0.5,
                    buttons: e.buttons || 0,
                    shift: !!e.shiftKey,
                    alt: !!e.altKey,
                    ctrl: !!(e.ctrlKey || e.metaKey),
                    pointerType: e.pointerType || 'mouse',
                    pointerId: e.pointerId
                };
            }),
            // The pointer left the canvas: hide the ring (a captured drag does not get here until it is released).
            onCanvasLeave: Pixel.bindRuntime(runtime, function onCanvasLeave() {
                this.cursorPos = null;
                this.requestOverlay();
            }),
            applyPointTool: Pixel.bindRuntime(runtime, function applyPointTool(x, y) {
                if (this.activeTool === 'airbrush') this.sprayAt(x, y);
                else if (this.activeTool === 'dodge-burn') this.dodgeBurnAt(x, y);
                else if (this.activeTool === 'blur-brush') this.blurAt(x, y);
            }),
            onCanvasMouseDown: Pixel.bindRuntime(runtime, function onCanvasMouseDown(e) {
                const toolDef = this.activeToolDef();
                // A registered tool with handlers owns the whole gesture (down, move, up); everything else is the legacy chain below.
                if (Pixel.toolHasHandlers(toolDef)) {
                    if (!this.canvas.width || e.button === 1) return;
                    if (toolDef.raster && !this.canPaintActiveLayer(toolDef.id)) return;
                    if (toolDef.raster) this.commitPendingPreviews();
                    this.gesture = { def: toolDef, pointerId: e.pointerId };
                    if (typeof toolDef.onDown === 'function') runToolHandler(this, toolDef, 'onDown', e);
                    return;
                }
                if (toolDef && toolDef.raster && !this.canPaintActiveLayer(this.activeTool)) return;
                if (!this.canvas.width || e.button === 1) return;
                const isDrawingTool = this.activeTool && STROKE_TOOLS.includes(this.activeTool);
                const isPointTool = this.activeTool && POINT_TOOLS.includes(this.activeTool);
                const isShapeTool = this.activeTool && SHAPE_TOOLS.includes(this.activeTool);
                const isSelectionTool = this.activeTool && SELECTION_TOOLS.includes(this.activeTool);
                const coords = this.canvasCoords(e);
                // The pixels these tools read must be the ones on screen, so a pending adjust/filter preview is baked in first.
                if (isDrawingTool || isPointTool || isShapeTool || this.activeTool === 'fill') this.commitPendingPreviews();

                if (this.activeTool === 'eyedropper') {
                    const pixel = this.sampleComposite(coords.x, coords.y);
                    const hex = this.rgbToHex(pixel[0], pixel[1], pixel[2]);
                    this.setPrimaryColor(hex);
                    this.addRecentColor(hex);
                    this.setActiveTool(null);
                    return;
                }

                if (this.activeTool === 'magic-wand') {
                    this.magicWandSelect(coords.x, coords.y);
                    return;
                }

                if (this.activeTool === 'fill') {
                    this.floodFill(coords.x, coords.y, this.primaryColor);
                    this.addRecentColor(this.primaryColor);
                    return;
                }

                if (this.activeTool === 'text') {
                    this.showTextInput(coords.x, coords.y);
                    return;
                }

                if (this.activeTool === 'lasso') {
                    this.lassoPath = [{ x: coords.x, y: coords.y }];
                    this.isDrawing = true;
                    return;
                }

                if (this.activeTool === 'move') {
                    if (this.selection && !this.requireRasterLayer()) return;
                    this.moveLayerStartX = coords.x;
                    this.moveLayerStartY = coords.y;
                    if (this.selection) {
                        this.isMovingSel = true;
                        this.selDragOfsX = coords.x - this.selection.x;
                        this.selDragOfsY = coords.y - this.selection.y;
                        this.beginMoveSelection();
                    } else {
                        this.isMovingSel = true;
                        this.moveLayerSnapshot = { x: coords.x, y: coords.y };
                    }
                    return;
                }

                if (this.activeTool === 'clone-stamp' && e.altKey) {
                    this.cloneSourceX = coords.x;
                    this.cloneSourceY = coords.y;
                    this.notify({ type: 'info', message: this.t('pixel.clone_source_set') });
                    return;
                }

                if (isDrawingTool) {
                    this.isDrawing = true;
                    this.lastDrawX = coords.x;
                    this.lastDrawY = coords.y;
                    if (this.activeTool === 'clone-stamp') {
                        this.cloneStampUsed = false;
                        if (this.cloneSampleCanvas) this.releaseTempCanvas(this.cloneSampleCanvas);
                        this.cloneSampleCanvas = null;
                        if (this.cloneSourceX != null && this.cloneSourceY != null) {
                            this.cloneStrokeOffX = coords.x - this.cloneSourceX;
                            this.cloneStrokeOffY = coords.y - this.cloneSourceY;
                            this.cloneSampleCanvas = this.acquireTempCanvas(this.canvas.width, this.canvas.height);
                            this.cloneSampleCanvas.getContext('2d').drawImage(this.getActiveReadCtx().canvas, 0, 0);
                        }
                    } else {
                        const actx = this.getActiveCtx();
                        const spec = this.brushStrokeSpec();
                        this.drawBrushStroke(actx, coords.x, coords.y, coords.x, coords.y, spec.size, spec.opacity, spec.color, spec.op, this.brushHardness);
                    }
                    this.compositeLayers();
                    return;
                }

                if (isPointTool) {
                    this.isDrawing = true;
                    this.lastDrawX = coords.x;
                    this.lastDrawY = coords.y;
                    this.applyPointTool(coords.x, coords.y);
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
                    const pixel = this.sampleComposite(Math.min(coords.x, this.imgWidth - 1), Math.min(coords.y, this.imgHeight - 1));
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

                if (this.gesture) {
                    if (typeof this.gesture.def.onMove === 'function') runToolHandler(this, this.gesture.def, 'onMove', e);
                    return;
                }
                const toolDef = this.activeToolDef();
                if (Pixel.toolHasHandlers(toolDef)) {
                    if (typeof toolDef.onHover === 'function') runToolHandler(this, toolDef, 'onHover', e);
                    if (typeof toolDef.overlay === 'function') this.requestOverlay();
                    return;
                }

                const isDrawingTool = this.activeTool && STROKE_TOOLS.includes(this.activeTool);
                const isPointTool = this.activeTool && POINT_TOOLS.includes(this.activeTool);
                const isShapeTool = this.activeTool && SHAPE_TOOLS.includes(this.activeTool);
                const isSelectionTool = this.activeTool && SELECTION_TOOLS.includes(this.activeTool);

                // The brush ring follows the pointer, also while a stroke is drawn.
                const ringTool = !!(toolDef && toolDef.cursor === 'none' && !this.cropState);
                if (ringTool) this.drawBrushCursor(coords.x, coords.y);
                if (ringTool && !this.isDrawing && !this.isMovingSel) return;

                if (this.isDrawing && isDrawingTool) {
                    const actx = this.getActiveCtx();
                    if (this.activeTool === 'clone-stamp') {
                        this.cloneStampAt(coords.x, coords.y);
                    } else {
                        const spec = this.brushStrokeSpec();
                        this.drawBrushStroke(actx, this.lastDrawX, this.lastDrawY, coords.x, coords.y, spec.size, spec.opacity, spec.color, spec.op, this.brushHardness);
                    }
                    this.compositeLayers();
                    this.lastDrawX = coords.x;
                    this.lastDrawY = coords.y;
                    return;
                }

                if (this.isDrawing && this.activeTool === 'lasso') {
                    this.lassoPath.push({ x: coords.x, y: coords.y });
                    this.drawLassoPreview();
                    return;
                }

                if (this.isDrawing && isPointTool) {
                    const step = Math.max(1, this.brushSize / 6);
                    if (Math.hypot(coords.x - this.lastDrawX, coords.y - this.lastDrawY) >= step) {
                        this.applyPointTool(coords.x, coords.y);
                        this.lastDrawX = coords.x;
                        this.lastDrawY = coords.y;
                    }
                    return;
                }

                if (this.isDrawing && isShapeTool) {
                    let ex = coords.x, ey = coords.y;
                    if (e.shiftKey) {
                        const s = this.snapEnd(this.drawStartX, this.drawStartY, ex, ey);
                        ex = s.x; ey = s.y;
                    }
                    this.drawShapePreview(this.drawStartX, this.drawStartY, ex, ey);
                    return;
                }

                if (this.isDrawing && isSelectionTool) {
                    let ex = coords.x, ey = coords.y;
                    if (e.shiftKey) {
                        const s = this.snapEnd(this.drawStartX, this.drawStartY, ex, ey);
                        ex = s.x; ey = s.y;
                    }
                    const sx = Math.min(this.drawStartX, ex);
                    const sy = Math.min(this.drawStartY, ey);
                    const sw = Math.abs(ex - this.drawStartX);
                    const sh = Math.abs(ey - this.drawStartY);
                    this.selection = { type: this.activeTool === 'select-rect' ? 'rect' : 'ellipse', x: sx, y: sy, w: sw, h: sh };
                    this.startMarchingAnts();
                    return;
                }

                if (this.isMovingSel && e.buttons === 1) {
                    if (this.activeTool === 'move' && this.moveLayerSnapshot && !this.selection) {
                        return;
                    }
                    if (this.selection) {
                        this.selection = Object.assign({}, this.selection, { x: coords.x - this.selDragOfsX, y: coords.y - this.selDragOfsY });
                        this.startMarchingAnts();
                    }
                    return;
                }
            }),
            onCanvasMouseUp: Pixel.bindRuntime(runtime, function onCanvasMouseUp(e) {
                if (this.gesture) {
                    const gesture = this.gesture;
                    this.gesture = null;
                    if (typeof gesture.def.onUp === 'function') runToolHandler(this, gesture.def, 'onUp', e);
                    return;
                }
                const coords = this.canvasCoords(e);
                const isDrawingTool = this.activeTool && STROKE_TOOLS.includes(this.activeTool);
                const isPointTool = this.activeTool && POINT_TOOLS.includes(this.activeTool);
                const isShapeTool = this.activeTool && SHAPE_TOOLS.includes(this.activeTool);
                const isSelectionTool = this.activeTool && SELECTION_TOOLS.includes(this.activeTool);

                if (this.isDrawing && isDrawingTool) {
                    this.isDrawing = false;
                    if (this.activeTool === 'clone-stamp') {
                        if (this.cloneSampleCanvas) {
                            this.releaseTempCanvas(this.cloneSampleCanvas);
                            this.cloneSampleCanvas = null;
                        }
                        if (this.cloneStampUsed) this.pushHistory('draw:clone-stamp');
                    } else {
                        if (this.activeTool !== 'clone-stamp') this.addRecentColor(this.primaryColor);
                        this.pushHistory('draw:' + this.activeTool);
                    }
                    return;
                }

                if (this.isDrawing && this.activeTool === 'lasso') {
                    this.isDrawing = false;
                    this.commitLassoPath();
                    return;
                }

                if (this.isDrawing && isPointTool) {
                    this.isDrawing = false;
                    this.addRecentColor(this.primaryColor);
                    this.pushHistory('draw:' + this.activeTool);
                    return;
                }

                if (this.isDrawing && isShapeTool) {
                    this.isDrawing = false;
                    let ex = coords.x, ey = coords.y;
                    if (e.shiftKey) {
                        const s = this.snapEnd(this.drawStartX, this.drawStartY, ex, ey);
                        ex = s.x; ey = s.y;
                    }
                    this.commitShape(this.drawStartX, this.drawStartY, ex, ey);
                    this.clearOverlay();
                    if (this.activeTool !== 'gradient') {
                        this.addRecentColor(this.primaryColor);
                        this.pushHistory('draw:' + this.activeTool);
                    }
                    return;
                }

                if (this.isDrawing && isSelectionTool) {
                    this.isDrawing = false;
                    if (this.selection && this.selection.w > 2 && this.selection.h > 2) {
                        const actx = this.getActiveReadCtx();
                        const sx = Math.max(0, Math.round(Math.min(this.selection.x, this.selection.x + this.selection.w)));
                        const sy = Math.max(0, Math.round(Math.min(this.selection.y, this.selection.y + this.selection.h)));
                        const sw = Math.round(Math.abs(this.selection.w));
                        const sh = Math.round(Math.abs(this.selection.h));
                        this.selImageData = actx.getImageData(sx, sy, sw, sh);
                    }
                    return;
                }

                if (this.isMovingSel) {
                    if (this.activeTool === 'move') {
                        if (this.moveSelFloating) {
                            this.commitMoveSelection();
                        } else if (this.moveLayerSnapshot && !this.selection) {
                            const dx = coords.x - this.moveLayerStartX;
                            const dy = coords.y - this.moveLayerStartY;
                            this.commitMoveLayer(dx, dy);
                            this.moveLayerSnapshot = null;
                        }
                    }
                    this.isMovingSel = false;
                    return;
                }
            }),
            // Keyboard: Space pans, the active tool's onKey(r, e) may consume the key (return true), everything else is a
            // command shortcut from the registry (menu items, tools, panels and registered commands).
            handleKeydown: Pixel.bindRuntime(runtime, function handleKeydown(e) {
                if (this.state.disposed || !this.isActiveWindow()) return;
                const target = e.target;
                if (target && target.closest && target.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"]')) return;
                if (e.key === ' ' && !e.ctrlKey && !e.metaKey && !e.altKey) {
                    e.preventDefault();
                    this.spaceHeld = true;
                    this.canvasArea.style.cursor = 'grab';
                    return;
                }
                // The desktop shell already ran a menu shortcut (it calls preventDefault before the app sees the key).
                if (e.defaultPrevented) return;
                const def = this.activeToolDef();
                if (def && typeof def.onKey === 'function') {
                    let handled = false;
                    try { handled = def.onKey(this, e) === true; } catch (err) { if (window.console) console.error('pixel tool ' + def.id + '.onKey', err); }
                    if (handled) { e.preventDefault(); return; }
                }
                this.runShortcut(e);
            }),
            handleKeyup: Pixel.bindRuntime(runtime, function handleKeyup(e) {
                if (e.key === ' ') {
                    this.spaceHeld = false;
                    if (!this.isPanning && this.canvasArea) this.canvasArea.style.cursor = '';
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
                        ['Ctrl+Wheel', this.t('pixel.scroll_zoom')],
                        ['Shift+Drag', this.t('pixel.snap_shapes')]
                    ]],
                    [this.t('pixel.shortcuts_tools'), Pixel.listTools()
                        .filter(def => def.shortcut || Pixel.TOOL_SHORTCUTS[def.id])
                        .map(def => [def.shortcut || Pixel.TOOL_SHORTCUTS[def.id], this.toolLabel(def.id)])
                        .concat([['?', this.t('pixel.shortcuts')]])]
                ];
                // Shortcuts of registered commands that the lists above do not mention yet.
                const listed = new Set();
                shortcuts.forEach(([, items]) => items.forEach(([key]) => listed.add(Pixel.normalizeShortcut(key))));
                const more = this.listCommands()
                    .filter(cmd => cmd.source !== 'tool' && cmd.source !== 'panel' && cmd.shortcut && !cmd.shortcuts.every(key => listed.has(key)))
                    .map(cmd => [cmd.shortcut, cmd.label]);
                if (more.length) shortcuts.push([this.t('pixel.shortcuts_more', 'More'), more]);
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
                const densitySlider = this.host.querySelector('[data-spray-density]');
                if (densitySlider) {
                    densitySlider.addEventListener('input', () => { this.sprayDensity = Number(densitySlider.value); const el = this.host.querySelector('[data-spray-density-val]'); if (el) el.textContent = this.sprayDensity; });
                }
                const dodgeSelect = this.host.querySelector('[data-dodge-mode]');
                if (dodgeSelect) {
                    dodgeSelect.addEventListener('change', () => { this.dodgeMode = dodgeSelect.value; });
                }
                const gradientSelect = this.host.querySelector('[data-gradient-mode]');
                if (gradientSelect) {
                    gradientSelect.addEventListener('change', () => { this.gradientMode = gradientSelect.value; });
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

        // The brush ring is drawn above everything else on the overlay canvas.
        runtime.registerOverlay('cursor', ctx => runtime.paintBrushCursor(ctx), 90);
    };
})();
