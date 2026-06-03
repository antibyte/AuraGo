(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installTools = function installTools(runtime) {
        Object.assign(runtime, {
            showPanel: Pixel.bindRuntime(runtime, function showPanel(name) {
                                activePanel = name;
                                host.querySelectorAll('.pixel-tab').forEach(b => b.classList.toggle('active', b.dataset.panel === name));
                                host.querySelectorAll('[data-section]').forEach(s => { s.hidden = s.dataset.section !== name; });
                                if (name !== 'transform' && cropState) cancelCrop();
                                if (name === 'draw' && !activeTool) setActiveTool('brush');
                                if (name === 'layers') refreshLayerPanel();
            }),
            refreshLayerPanel: Pixel.bindRuntime(runtime, function refreshLayerPanel() {
                                const section = host.querySelector('[data-section="layers"]');
                                if (!section) return;
                                const list = section.querySelector('[data-layer-list]');
                                if (!list) return;
                                list.innerHTML = layers.map((layer, i) => {
                                    const isActive = i === activeLayerIdx;
                                    return `<div class="pixel-layer-item${isActive ? ' active' : ''}" data-layer-idx="${i}">
                                        <button class="pixel-layer-vis" type="button" data-layer-vis="${i}" title="${esc(t('pixel.toggle_visibility', 'Toggle Visibility'))}">${layer.visible ? '👁' : '◻'}</button>
                                        <span class="pixel-layer-name" data-layer-name="${i}">${esc(layer.name)}</span>
                                        <input type="range" class="pixel-slider pixel-layer-opacity" data-layer-opacity="${i}" min="0" max="100" value="${Math.round(layer.opacity * 100)}">
                                    </div>`;
                                }).reverse().join('');
            }),
            setActiveTool: Pixel.bindRuntime(runtime, function setActiveTool(tool) {
                                if (activeTool === tool) { activeTool = null; } else { activeTool = tool; }
                                host.querySelectorAll('[data-draw-tool]').forEach(b => b.classList.toggle('active', b.dataset.drawTool === activeTool));
                                const optsContainer = host.querySelector('[data-draw-options]');
                                if (optsContainer) optsContainer.innerHTML = buildDrawOptionsHTML();
                                wireDrawOptionEvents();
                                updateStatus();
                                if (statusTool) statusTool.textContent = activeTool ? t('pixel.' + activeTool, activeTool) : '';
                                canvas.style.cursor = getToolCursor();
            }),
            getToolCursor: Pixel.bindRuntime(runtime, function getToolCursor() {
                                if (!activeTool || activeTool === 'select-rect' || activeTool === 'select-ellipse') return 'crosshair';
                                if (activeTool === 'eyedropper') return 'crosshair';
                                if (activeTool === 'text') return 'text';
                                if (activeTool === 'fill') return 'crosshair';
                                return 'crosshair';
            }),
            clearOverlay: Pixel.bindRuntime(runtime, function clearOverlay() {
                                if (olCtx) olCtx.clearRect(0, 0, overlayCanvas.width, overlayCanvas.height);
            }),
            drawMarchingAnts: Pixel.bindRuntime(runtime, function drawMarchingAnts() {
                                if (!selection) { clearOverlay(); return; }
                                marchingOfs = (marchingOfs + 0.5) % 8;
                                olCtx.clearRect(0, 0, overlayCanvas.width, overlayCanvas.height);
                                olCtx.setLineDash([4, 4]);
                                olCtx.lineDashOffset = -marchingOfs;
                                olCtx.strokeStyle = '#ffffff';
                                olCtx.lineWidth = 1;
                                if (selection.type === 'rect') {
                                    olCtx.strokeRect(selection.x, selection.y, selection.w, selection.h);
                                } else if (selection.type === 'ellipse') {
                                    olCtx.beginPath();
                                    olCtx.ellipse(selection.x + selection.w / 2, selection.y + selection.h / 2, Math.abs(selection.w / 2), Math.abs(selection.h / 2), 0, 0, Math.PI * 2);
                                    olCtx.stroke();
                                }
                                olCtx.setLineDash([]);
                                olCtx.lineDashOffset = 0;
                                olCtx.strokeStyle = '#000000';
                                olCtx.lineWidth = 1;
                                olCtx.setLineDash([4, 4]);
                                olCtx.lineDashOffset = -marchingOfs + 4;
                                if (selection.type === 'rect') {
                                    olCtx.strokeRect(selection.x, selection.y, selection.w, selection.h);
                                } else if (selection.type === 'ellipse') {
                                    olCtx.beginPath();
                                    olCtx.ellipse(selection.x + selection.w / 2, selection.y + selection.h / 2, Math.abs(selection.w / 2), Math.abs(selection.h / 2), 0, 0, Math.PI * 2);
                                    olCtx.stroke();
                                }
                                olCtx.setLineDash([]);
                                marchingRAF = requestAnimationFrame(drawMarchingAnts);
            }),
            startMarchingAnts: Pixel.bindRuntime(runtime, function startMarchingAnts() {
                                stopMarchingAnts();
                                marchingOfs = 0;
                                drawMarchingAnts();
            }),
            stopMarchingAnts: Pixel.bindRuntime(runtime, function stopMarchingAnts() {
                                if (marchingRAF) { cancelAnimationFrame(marchingRAF); marchingRAF = null; }
            }),
            selectAll: Pixel.bindRuntime(runtime, function selectAll() {
                                if (!canvas.width) return;
                                selection = { type: 'rect', x: 0, y: 0, w: canvas.width, h: canvas.height };
                                startMarchingAnts();
            }),
            deselect: Pixel.bindRuntime(runtime, function deselect() {
                                selection = null;
                                selImageData = null;
                                stopMarchingAnts();
                                clearOverlay();
            }),
            copySelection: Pixel.bindRuntime(runtime, function copySelection() {
                                if (!selection || !canvas.width) return;
                                const sx = Math.max(0, Math.round(Math.min(selection.x, selection.x + selection.w)));
                                const sy = Math.max(0, Math.round(Math.min(selection.y, selection.y + selection.h)));
                                const sw = Math.round(Math.abs(selection.w));
                                const sh = Math.round(Math.abs(selection.h));
                                if (sw < 1 || sh < 1) return;
                                const actx = getActiveCtx();
                                selImageData = actx.getImageData(sx, sy, sw, sh);
                                try {
                                    const tmpC = acquireTempCanvas(sw, sh);
                                    tmpC.getContext('2d').putImageData(selImageData, 0, 0);
                                    tmpC.toBlob(blob => {
                                        if (blob) navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]).catch(() => {});
                                    }, 'image/png');
                                    releaseTempCanvas(tmpC);
                                } catch (_) {}
                                notify({ type: 'success', message: t('pixel.copied', 'Selection copied') });
            }),
            cutSelection: Pixel.bindRuntime(runtime, function cutSelection() {
                                if (!selection || !canvas.width) return;
                                copySelection();
                                const sx = Math.max(0, Math.round(Math.min(selection.x, selection.x + selection.w)));
                                const sy = Math.max(0, Math.round(Math.min(selection.y, selection.y + selection.h)));
                                const sw = Math.round(Math.abs(selection.w));
                                const sh = Math.round(Math.abs(selection.h));
                                if (sw < 1 || sh < 1) return;
                                const actx = getActiveCtx();
                                actx.clearRect(sx, sy, sw, sh);
                                if (layers.length > 1) compositeLayers();
                                pushHistory('cut');
            }),
            pasteClipboard: Pixel.bindRuntime(runtime, async function pasteClipboard() {
                                try {
                                    const items = await navigator.clipboard.read();
                                    for (const item of items) {
                                        for (const type of item.types) {
                                            if (type.startsWith('image/')) {
                                                const blob = await item.getType(type);
                                                const url = URL.createObjectURL(blob);
                                                const img = await loadImage(url);
                                                URL.revokeObjectURL(url);
                                                newBlankCanvas(Math.max(canvas.width || 0, img.naturalWidth), Math.max(canvas.height || 0, img.naturalHeight));
                                                const actx = getActiveCtx();
                                                actx.drawImage(img, 0, 0);
                                                if (layers.length > 1) compositeLayers();
                                                pushHistory('paste');
                                                return;
                                            }
                                        }
                                    }
                                } catch (_) {}
            }),
            deleteSelection: Pixel.bindRuntime(runtime, function deleteSelection() {
                                if (!selection || !canvas.width) return;
                                const sx = Math.max(0, Math.round(Math.min(selection.x, selection.x + selection.w)));
                                const sy = Math.max(0, Math.round(Math.min(selection.y, selection.y + selection.h)));
                                const sw = Math.round(Math.abs(selection.w));
                                const sh = Math.round(Math.abs(selection.h));
                                if (sw < 1 || sh < 1) return;
                                const actx = getActiveCtx();
                                actx.clearRect(sx, sy, sw, sh);
                                if (layers.length > 1) compositeLayers();
                                pushHistory('delete');
            }),
            addRecentColor: Pixel.bindRuntime(runtime, function addRecentColor(c) {
                                if (!c || c.length < 4) return;
                                recentColors = recentColors.filter(x => x !== c);
                                recentColors.unshift(c);
                                if (recentColors.length > 8) recentColors = recentColors.slice(0, 8);
            }),
            setPrimaryColor: Pixel.bindRuntime(runtime, function setPrimaryColor(c) {
                                primaryColor = c;
                                const el = host.querySelector('[data-color-primary]');
                                if (el) el.style.background = c;
                                const hex = host.querySelector('[data-hex-input]');
                                if (hex) hex.value = c;
            }),
            setSecondaryColor: Pixel.bindRuntime(runtime, function setSecondaryColor(c) {
                                secondaryColor = c;
                                const el = host.querySelector('[data-color-secondary]');
                                if (el) el.style.background = c;
            }),
            swapColors: Pixel.bindRuntime(runtime, function swapColors() {
                                const tmp = primaryColor;
                                setPrimaryColor(secondaryColor);
                                setSecondaryColor(tmp);
            }),
            drawBrushStroke: Pixel.bindRuntime(runtime, function drawBrushStroke(ctx, x0, y0, x1, y1, size, opacity, color, compositeOp, hardness) {
                                ctx.save();
                                ctx.globalCompositeOperation = compositeOp || 'source-over';
                                ctx.globalAlpha = opacity / 100;
                                ctx.strokeStyle = color;
                                ctx.lineWidth = size;
                                ctx.lineCap = 'round';
                                ctx.lineJoin = 'round';
                                if (hardness !== undefined && hardness < 100) {
                                    ctx.shadowBlur = (100 - hardness) * size * 0.01;
                                    ctx.shadowColor = color;
                                }
                                ctx.beginPath();
                                ctx.moveTo(x0, y0);
                                ctx.lineTo(x1, y1);
                                ctx.stroke();
                                ctx.restore();
            }),
            drawShapePreview: Pixel.bindRuntime(runtime, function drawShapePreview(x0, y0, x1, y1) {
                                clearOverlay();
                                olCtx.save();
                                olCtx.strokeStyle = primaryColor;
                                olCtx.fillStyle = primaryColor;
                                olCtx.lineWidth = shapeStrokeWidth;
                                olCtx.lineCap = 'round';
                                olCtx.lineJoin = 'round';
                                olCtx.globalAlpha = brushOpacity / 100;
                                const w = x1 - x0;
                                const h = y1 - y0;
                                if (activeTool === 'line') {
                                    olCtx.beginPath();
                                    olCtx.moveTo(x0, y0);
                                    olCtx.lineTo(x1, y1);
                                    olCtx.stroke();
                                } else if (activeTool === 'arrow') {
                                    olCtx.beginPath();
                                    olCtx.moveTo(x0, y0);
                                    olCtx.lineTo(x1, y1);
                                    olCtx.stroke();
                                    const angle = Math.atan2(y1 - y0, x1 - x0);
                                    const headLen = Math.max(10, shapeStrokeWidth * 4);
                                    olCtx.beginPath();
                                    olCtx.moveTo(x1, y1);
                                    olCtx.lineTo(x1 - headLen * Math.cos(angle - Math.PI / 6), y1 - headLen * Math.sin(angle - Math.PI / 6));
                                    olCtx.moveTo(x1, y1);
                                    olCtx.lineTo(x1 - headLen * Math.cos(angle + Math.PI / 6), y1 - headLen * Math.sin(angle + Math.PI / 6));
                                    olCtx.stroke();
                                } else if (activeTool === 'rectangle') {
                                    if (shapeFill) olCtx.fillRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                                    else olCtx.strokeRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                                } else if (activeTool === 'ellipse') {
                                    olCtx.beginPath();
                                    olCtx.ellipse(x0 + w / 2, y0 + h / 2, Math.abs(w / 2), Math.abs(h / 2), 0, 0, Math.PI * 2);
                                    if (shapeFill) olCtx.fill();
                                    else olCtx.stroke();
                                }
                                olCtx.restore();
            }),
            commitShape: Pixel.bindRuntime(runtime, function commitShape(x0, y0, x1, y1) {
                                const actx = getActiveCtx();
                                actx.save();
                                actx.strokeStyle = primaryColor;
                                actx.fillStyle = primaryColor;
                                actx.lineWidth = shapeStrokeWidth;
                                actx.lineCap = 'round';
                                actx.lineJoin = 'round';
                                actx.globalAlpha = brushOpacity / 100;
                                const w = x1 - x0;
                                const h = y1 - y0;
                                if (activeTool === 'line') {
                                    actx.beginPath();
                                    actx.moveTo(x0, y0);
                                    actx.lineTo(x1, y1);
                                    actx.stroke();
                                } else if (activeTool === 'arrow') {
                                    actx.beginPath();
                                    actx.moveTo(x0, y0);
                                    actx.lineTo(x1, y1);
                                    actx.stroke();
                                    const angle = Math.atan2(y1 - y0, x1 - x0);
                                    const headLen = Math.max(10, shapeStrokeWidth * 4);
                                    actx.beginPath();
                                    actx.moveTo(x1, y1);
                                    actx.lineTo(x1 - headLen * Math.cos(angle - Math.PI / 6), y1 - headLen * Math.sin(angle - Math.PI / 6));
                                    actx.moveTo(x1, y1);
                                    actx.lineTo(x1 - headLen * Math.cos(angle + Math.PI / 6), y1 - headLen * Math.sin(angle + Math.PI / 6));
                                    actx.stroke();
                                } else if (activeTool === 'rectangle') {
                                    if (shapeFill) actx.fillRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                                    else actx.strokeRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                                } else if (activeTool === 'ellipse') {
                                    actx.beginPath();
                                    actx.ellipse(x0 + w / 2, y0 + h / 2, Math.abs(w / 2), Math.abs(h / 2), 0, 0, Math.PI * 2);
                                    if (shapeFill) actx.fill();
                                    else actx.stroke();
                                }
                                actx.restore();
                                if (layers.length > 1) compositeLayers();
            }),
            floodFill: Pixel.bindRuntime(runtime, function floodFill(sx, sy, fillColor) {
                                const actx = getActiveCtx();
                                const w = canvas.width;
                                const h = canvas.height;
                                const imgData = actx.getImageData(0, 0, w, h);
                                const data = imgData.data;
                                const idx = (sy * w + sx) * 4;
                                const sr = data[idx], sg = data[idx + 1], sb = data[idx + 2], sa = data[idx + 3];
                                const fc = hexToRgb(fillColor);
                                if (sr === fc.r && sg === fc.g && sb === fc.b && sa === 255) return;
                                const visited = new Uint8Array(w * h);
                                const stack = [sx, sy];
                                const tol = fillTolerance;
                                while (stack.length > 0) {
                                    const cy = stack.pop();
                                    const cx = stack.pop();
                                    if (cx < 0 || cx >= w || cy < 0 || cy >= h) continue;
                                    const pos = cy * w + cx;
                                    if (visited[pos]) continue;
                                    const pi = pos * 4;
                                    if (colorDist(data[pi], data[pi + 1], data[pi + 2], sr, sg, sb) > tol) continue;
                                    visited[pos] = 1;
                                    data[pi] = fc.r;
                                    data[pi + 1] = fc.g;
                                    data[pi + 2] = fc.b;
                                    data[pi + 3] = 255;
                                    stack.push(cx + 1, cy);
                                    stack.push(cx - 1, cy);
                                    stack.push(cx, cy + 1);
                                    stack.push(cx, cy - 1);
                                }
                                actx.putImageData(imgData, 0, 0);
                                if (layers.length > 1) compositeLayers();
            }),
            commitTextToCanvas: Pixel.bindRuntime(runtime, function commitTextToCanvas(text, x, y) {
                                if (!text) return;
                                const actx = getActiveCtx();
                                actx.save();
                                actx.font = `${fontSize}px ${fontFamily}`;
                                actx.fillStyle = primaryColor;
                                actx.globalAlpha = brushOpacity / 100;
                                actx.textBaseline = 'top';
                                const lines = text.split('\n');
                                for (let i = 0; i < lines.length; i++) {
                                    actx.fillText(lines[i], x, y + i * fontSize * 1.2);
                                }
                                actx.restore();
                                if (layers.length > 1) compositeLayers();
                                pushHistory('text');
            }),
            addLayer: Pixel.bindRuntime(runtime, function addLayer() {
                                if (layers.length >= 10) { notify({ type: 'error', message: t('pixel.max_layers', 'Maximum 10 layers') }); return; }
                                ensureBackgroundMigrated();
                                const c = document.createElement('canvas');
                                c.width = imgWidth;
                                c.height = imgHeight;
                                layers.push({ canvas: c, name: t('pixel.layer', 'Layer') + ' ' + (layers.length + 1), visible: true, opacity: 1 });
                                activeLayerIdx = layers.length - 1;
                                compositeLayers();
                                refreshLayerPanel();
                                pushHistory('add layer');
            }),
            deleteLayer: Pixel.bindRuntime(runtime, function deleteLayer() {
                                if (layers.length <= 1) return;
                                layers.splice(activeLayerIdx, 1);
                                activeLayerIdx = Math.min(activeLayerIdx, layers.length - 1);
                                if (layers.length === 1 && layers[0].canvas) {
                                    cctx.clearRect(0, 0, canvas.width, canvas.height);
                                    cctx.drawImage(layers[0].canvas, 0, 0);
                                    layers[0].canvas = null;
                                }
                                compositeLayers();
                                refreshLayerPanel();
                                pushHistory('delete layer');
            }),
            duplicateLayer: Pixel.bindRuntime(runtime, function duplicateLayer() {
                                if (layers.length >= 10) return;
                                const src = layers[activeLayerIdx];
                                const c = document.createElement('canvas');
                                c.width = imgWidth;
                                c.height = imgHeight;
                                if (src.canvas) c.getContext('2d').drawImage(src.canvas, 0, 0);
                                else c.getContext('2d').drawImage(canvas, 0, 0);
                                layers.splice(activeLayerIdx + 1, 0, { canvas: c, name: src.name + ' copy', visible: true, opacity: src.opacity });
                                activeLayerIdx = activeLayerIdx + 1;
                                compositeLayers();
                                refreshLayerPanel();
                                pushHistory('duplicate layer');
            }),
            mergeDown: Pixel.bindRuntime(runtime, function mergeDown() {
                                if (activeLayerIdx <= 0 || layers.length < 2) return;
                                const upper = layers[activeLayerIdx];
                                const lower = layers[activeLayerIdx - 1];
                                const targetCanvas = lower.canvas || (() => { lower.canvas = document.createElement('canvas'); lower.canvas.width = imgWidth; lower.canvas.height = imgHeight; lower.canvas.getContext('2d').drawImage(canvas, 0, 0); return lower.canvas; })();
                                const tx = targetCanvas.getContext('2d');
                                tx.globalAlpha = upper.opacity;
                                if (upper.canvas) tx.drawImage(upper.canvas, 0, 0);
                                tx.globalAlpha = 1;
                                layers.splice(activeLayerIdx, 1);
                                activeLayerIdx--;
                                compositeLayers();
                                refreshLayerPanel();
                                pushHistory('merge down');
            }),
            flattenLayers: Pixel.bindRuntime(runtime, function flattenLayers() {
                                if (layers.length <= 1) return;
                                compositeLayers();
                                layers = [{ canvas: null, name: t('pixel.layer_background', 'Background'), visible: true, opacity: 1 }];
                                activeLayerIdx = 0;
                                refreshLayerPanel();
                                pushHistory('flatten');
            })
        });
    };
})();
