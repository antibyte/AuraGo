(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installCanvas = function installCanvas(runtime) {
        Object.assign(runtime, {
            setStatus: Pixel.bindRuntime(runtime, function setStatus(msg) {                     if (statusText) statusText.textContent = msg || '';
            }),
            updateStatus: Pixel.bindRuntime(runtime, function updateStatus() {
                                if (!imgWidth || !imgHeight) { setStatus(t('pixel.status_ready', 'Ready')); return; }
                                const parts = [`${imgWidth} × ${imgHeight}`];
                                if (filePath) parts.push(filePath.split('/').pop());
                                if (isDirty) parts.push('● ' + t('pixel.unsaved_changes', 'Unsaved'));
                                setStatus(parts.join('  ·  '));
                                if (statusTool) statusTool.textContent = activeTool ? t('pixel.' + activeTool, activeTool) : '';
            }),
            updateZoomLabel: Pixel.bindRuntime(runtime, function updateZoomLabel() {                     if (zoomLabel) zoomLabel.textContent = Math.round(zoom * 100) + '%';
            }),
            canvasCoords: Pixel.bindRuntime(runtime, function canvasCoords(e) {
                                const rect = canvas.getBoundingClientRect();
                                return {
                                    x: Math.round((e.clientX - rect.left) * (canvas.width / rect.width)),
                                    y: Math.round((e.clientY - rect.top) * (canvas.height / rect.height))
                                };
            }),
            getActiveCtx: Pixel.bindRuntime(runtime, function getActiveCtx() {
                                if (layers.length === 1 || !layers[activeLayerIdx].canvas) return cctx;
                                return layers[activeLayerIdx].canvas.getContext('2d');
            }),
            ensureBackgroundMigrated: Pixel.bindRuntime(runtime, function ensureBackgroundMigrated() {
                                if (layers.length <= 1) return;
                                if (layers[0].canvas) return;
                                const bgCanvas = acquireTempCanvas(imgWidth || canvas.width, imgHeight || canvas.height);
                                bgCanvas.getContext('2d').drawImage(canvas, 0, 0);
                                layers[0].canvas = bgCanvas;
            }),
            compositeLayers: Pixel.bindRuntime(runtime, function compositeLayers() {
                                cctx.clearRect(0, 0, canvas.width, canvas.height);
                                for (let i = 0; i < layers.length; i++) {
                                    const layer = layers[i];
                                    if (!layer.visible) continue;
                                    cctx.globalAlpha = layer.opacity;
                                    if (layer.canvas) {
                                        cctx.drawImage(layer.canvas, 0, 0);
                                    } else if (i === 0 && layers.length === 1) {
                                    }
                                }
                                cctx.globalAlpha = 1;
            }),
            pushHistory: Pixel.bindRuntime(runtime, function pushHistory(label) {
                                if (!canvas.width || !canvas.height) return;
                                const layerStates = layers.map(l => {
                                    const src = l.canvas || (layers.length > 1 ? null : canvas);
                                    if (!src) return null;
                                    const tmp = acquireTempCanvas(src.width || canvas.width, src.height || canvas.height);
                                    tmp.getContext('2d').drawImage(src, 0, 0);
                                    return tmp;
                                });
                                if (historyIdx < history.length - 1) history = history.slice(0, historyIdx + 1);
                                history.push({ layerStates, width: canvas.width, height: canvas.height, label: label || '', layerMeta: layers.map(l => ({ name: l.name, visible: l.visible, opacity: l.opacity })) });
                                if (history.length > MAX_HISTORY) {
                                    const removed = history.shift();
                                    removed.layerStates.forEach(s => { if (s) releaseTempCanvas(s); });
                                }
                                historyIdx = history.length - 1;
                                isDirty = true;
                                updateStatus();
            }),
            restoreHistory: Pixel.bindRuntime(runtime, function restoreHistory(entry) {
                                canvas.width = entry.width;
                                canvas.height = entry.height;
                                imgWidth = entry.width;
                                imgHeight = entry.height;
                                overlayCanvas.width = entry.width;
                                overlayCanvas.height = entry.height;
                                layers = entry.layerMeta.map((meta, i) => ({
                                    canvas: entry.layerStates[i] ? (() => { const tc = acquireTempCanvas(entry.width, entry.height); tc.getContext('2d').drawImage(entry.layerStates[i], 0, 0); return tc; })() : null,
                                    name: meta.name,
                                    visible: meta.visible,
                                    opacity: meta.opacity
                                }));
                                activeLayerIdx = Math.min(activeLayerIdx, layers.length - 1);
                                compositeLayers();
                                applyZoom();
            }),
            undo: Pixel.bindRuntime(runtime, function undo() {
                                if (historyIdx <= 0) return;
                                historyIdx--;
                                restoreHistory(history[historyIdx]);
                                updateStatus();
            }),
            redo: Pixel.bindRuntime(runtime, function redo() {
                                if (historyIdx >= history.length - 1) return;
                                historyIdx++;
                                restoreHistory(history[historyIdx]);
                                updateStatus();
            }),
            applyZoom: Pixel.bindRuntime(runtime, function applyZoom() {
                                const w = imgWidth * zoom;
                                const h = imgHeight * zoom;
                                canvas.style.width = w + 'px';
                                canvas.style.height = h + 'px';
                                overlayCanvas.style.width = w + 'px';
                                overlayCanvas.style.height = h + 'px';
                                canvasWrap.style.width = w + 'px';
                                canvasWrap.style.height = h + 'px';
                                updateZoomLabel();
            }),
            zoomTo: Pixel.bindRuntime(runtime, function zoomTo(z) {
                                zoom = Math.max(0.05, Math.min(20, z));
                                applyZoom();
            }),
            zoomFit: Pixel.bindRuntime(runtime, function zoomFit() {
                                if (!imgWidth || !imgHeight) return;
                                const areaW = canvasArea.clientWidth - 20;
                                const areaH = canvasArea.clientHeight - 20;
                                zoom = Math.min(areaW / imgWidth, areaH / imgHeight, 1);
                                applyZoom();
            }),
            loadImage: Pixel.bindRuntime(runtime, function loadImage(src) {
                                return new Promise((resolve, reject) => {
                                    const img = new Image();
                                    img.crossOrigin = 'anonymous';
                                    img.onload = () => resolve(img);
                                    img.onerror = () => reject(new Error('Failed to load image'));
                                    img.src = src;
                                });
            }),
            loadImageToCanvas: Pixel.bindRuntime(runtime, async function loadImageToCanvas(src) {
                                const img = await loadImage(src);
                                originalImage = img;
                                canvas.width = img.naturalWidth;
                                canvas.height = img.naturalHeight;
                                overlayCanvas.width = img.naturalWidth;
                                overlayCanvas.height = img.naturalHeight;
                                cctx.clearRect(0, 0, canvas.width, canvas.height);
                                cctx.drawImage(img, 0, 0);
                                imgWidth = img.naturalWidth;
                                imgHeight = img.naturalHeight;
                                layers = [{ canvas: null, name: t('pixel.layer_background', 'Background'), visible: true, opacity: 1 }];
                                activeLayerIdx = 0;
                                history = [];
                                historyIdx = -1;
                                pushHistory('open');
                                canvasWrap.hidden = false;
                                emptyState.hidden = true;
                                selection = null;
                                clearOverlay();
                                zoomFit();
                                updateStatus();
            }),
            newBlankCanvas: Pixel.bindRuntime(runtime, function newBlankCanvas(w, h) {
                                originalImage = null;
                                canvas.width = w;
                                canvas.height = h;
                                overlayCanvas.width = w;
                                overlayCanvas.height = h;
                                cctx.fillStyle = '#ffffff';
                                cctx.fillRect(0, 0, w, h);
                                imgWidth = w;
                                imgHeight = h;
                                layers = [{ canvas: null, name: t('pixel.layer_background', 'Background'), visible: true, opacity: 1 }];
                                activeLayerIdx = 0;
                                history = [];
                                historyIdx = -1;
                                pushHistory('new');
                                canvasWrap.hidden = false;
                                emptyState.hidden = true;
                                selection = null;
                                clearOverlay();
                                zoomFit();
                                updateStatus();
            }),
            applyFilterPreview: Pixel.bindRuntime(runtime, function applyFilterPreview(name) {
                                if (!canvas.width) return;
                                const isMultiLayer = layers.length > 1;
                                if (isMultiLayer) ensureBackgroundMigrated();
                                const src = originalImage || canvas;
                                const tmpCanvas = acquireTempCanvas(imgWidth, imgHeight);
                                const tmpCtx = tmpCanvas.getContext('2d');
                                switch (name) {
                                    case 'grayscale': tmpCtx.filter = 'grayscale(100%)'; break;
                                    case 'sepia': tmpCtx.filter = 'sepia(100%)'; break;
                                    case 'invert': tmpCtx.filter = 'invert(100%)'; break;
                                    case 'blur': tmpCtx.filter = 'blur(2px)'; break;
                                    case 'vintage': tmpCtx.filter = 'sepia(60%) contrast(80%) brightness(90%)'; break;
                                    case 'warm': tmpCtx.filter = 'saturate(1.3) brightness(1.05)'; break;
                                    case 'cool': tmpCtx.filter = 'saturate(0.8) hue-rotate(20deg)'; break;
                                    case 'high-contrast': tmpCtx.filter = 'contrast(150%)'; break;
                                    default: tmpCtx.filter = 'none';
                                }
                                tmpCtx.drawImage(src, 0, 0, imgWidth, imgHeight);
                                if (name === 'vignette') applyVignette(tmpCtx);
                                if (name === 'emboss') applyEmboss(tmpCtx);
                                if (isMultiLayer) {
                                    const actx = getActiveCtx();
                                    actx.clearRect(0, 0, canvas.width, canvas.height);
                                    actx.drawImage(tmpCanvas, 0, 0);
                                    compositeLayers();
                                } else {
                                    cctx.clearRect(0, 0, canvas.width, canvas.height);
                                    cctx.drawImage(tmpCanvas, 0, 0);
                                }
                                releaseTempCanvas(tmpCanvas);
                                pushHistory('filter:' + name);
            }),
            applyVignette: Pixel.bindRuntime(runtime, function applyVignette(ctx) {
                                const w = canvas.width, h = canvas.height;
                                const grd = ctx.createRadialGradient(w / 2, h / 2, w * 0.25, w / 2, h / 2, w * 0.7);
                                grd.addColorStop(0, 'rgba(0,0,0,0)');
                                grd.addColorStop(1, 'rgba(0,0,0,0.6)');
                                ctx.fillStyle = grd;
                                ctx.fillRect(0, 0, w, h);
            }),
            applyEmboss: Pixel.bindRuntime(runtime, function applyEmboss(ctx) {
                                const imgData = ctx.getImageData(0, 0, canvas.width, canvas.height);
                                const kernel = [-2, -1, 0, -1, 1, 1, 0, 1, 2];
                                convolve(imgData, kernel, canvas.width, canvas.height);
                                ctx.putImageData(imgData, 0, 0);
            }),
            convolve: Pixel.bindRuntime(runtime, function convolve(imgData, kernel, w, h) {
                                const src = new Uint8ClampedArray(imgData.data);
                                const d = imgData.data;
                                const kSize = Math.sqrt(kernel.length) | 0;
                                const half = (kSize / 2) | 0;
                                for (let y = half; y < h - half; y++) {
                                    for (let x = half; x < w - half; x++) {
                                        let r = 0, g = 0, b = 0;
                                        for (let ky = 0; ky < kSize; ky++) {
                                            for (let kx = 0; kx < kSize; kx++) {
                                                const idx = ((y + ky - half) * w + (x + kx - half)) * 4;
                                                const kv = kernel[ky * kSize + kx];
                                                r += src[idx] * kv; g += src[idx + 1] * kv; b += src[idx + 2] * kv;
                                            }
                                        }
                                        const idx = (y * w + x) * 4;
                                        d[idx] = clamp255(r + 128); d[idx + 1] = clamp255(g + 128); d[idx + 2] = clamp255(b + 128);
                                    }
                                }
            }),
            buildFilterString: Pixel.bindRuntime(runtime, function buildFilterString() {
                                const parts = [];
                                const b = 1 + adjustments.brightness / 100;
                                const c = 1 + adjustments.contrast / 100;
                                const s = 1 + adjustments.saturation / 100;
                                const e = 1 + adjustments.exposure / 200;
                                if (b !== 1) parts.push(`brightness(${b})`);
                                if (c !== 1) parts.push(`contrast(${c})`);
                                if (s !== 1) parts.push(`saturate(${s})`);
                                if (e !== 1) parts.push(`brightness(${e})`);
                                if (adjustments.sharpness > 0) parts.push(`contrast(${1 + adjustments.sharpness / 200})`);
                                return parts.join(' ') || 'none';
            }),
            applyAdjustmentsPreview: Pixel.bindRuntime(runtime, function applyAdjustmentsPreview() {
                                if (!originalImage || !canvas.width) return;
                                const isMultiLayer = layers.length > 1;
                                if (isMultiLayer) ensureBackgroundMigrated();
                                const tmpCanvas = acquireTempCanvas(imgWidth, imgHeight);
                                const tmpCtx = tmpCanvas.getContext('2d');
                                tmpCtx.filter = buildFilterString();
                                tmpCtx.drawImage(originalImage, 0, 0, imgWidth, imgHeight);
                                applyCustomAdjustments(tmpCtx);
                                if (isMultiLayer) {
                                    const actx = getActiveCtx();
                                    actx.clearRect(0, 0, canvas.width, canvas.height);
                                    actx.drawImage(tmpCanvas, 0, 0);
                                    compositeLayers();
                                } else {
                                    cctx.clearRect(0, 0, canvas.width, canvas.height);
                                    cctx.drawImage(tmpCanvas, 0, 0);
                                }
                                releaseTempCanvas(tmpCanvas);
            }),
            applyCustomAdjustments: Pixel.bindRuntime(runtime, function applyCustomAdjustments(ctx) {
                                if (adjustments.temperature === 0 && adjustments.shadows === 0 && adjustments.highlights === 0) return;
                                const imgData = ctx.getImageData(0, 0, canvas.width, canvas.height);
                                const d = imgData.data;
                                const temp = adjustments.temperature * 0.5;
                                const shadows = adjustments.shadows * 0.5;
                                const highlights = adjustments.highlights * 0.5;
                                for (let i = 0; i < d.length; i += 4) {
                                    d[i] = clamp255(d[i] + temp);
                                    d[i + 2] = clamp255(d[i + 2] - temp);
                                    const lum = (d[i] + d[i + 1] + d[i + 2]) / 3;
                                    if (lum < 128) { d[i] = clamp255(d[i] + shadows); d[i + 1] = clamp255(d[i + 1] + shadows); d[i + 2] = clamp255(d[i + 2] + shadows); }
                                    if (lum > 128) { d[i] = clamp255(d[i] + highlights); d[i + 1] = clamp255(d[i + 1] + highlights); d[i + 2] = clamp255(d[i + 2] + highlights); }
                                }
                                ctx.putImageData(imgData, 0, 0);
            }),
            applyAdjustments: Pixel.bindRuntime(runtime, function applyAdjustments() {
                                if (!originalImage) return;
                                applyAdjustmentsPreview();
                                const dataURL = canvas.toDataURL();
                                const img = new Image();
                                img.onload = function () { originalImage = img; };
                                img.src = dataURL;
                                pushHistory('adjust');
                                resetAdjustments();
            }),
            resetAdjustments: Pixel.bindRuntime(runtime, function resetAdjustments() {
                                for (const k in adjustments) adjustments[k] = 0;
                                host.querySelectorAll('[data-adjust]').forEach(s => { s.value = 0; });
                                host.querySelectorAll('[data-val-brightness],[data-val-contrast],[data-val-saturation],[data-val-exposure],[data-val-sharpness],[data-val-temperature],[data-val-shadows],[data-val-highlights]').forEach(el => { el.textContent = '0'; });
                                if (originalImage && canvas.width) {
                                    if (layers.length > 1) {
                                        ensureBackgroundMigrated();
                                        const actx = layers[0].canvas.getContext('2d');
                                        actx.clearRect(0, 0, canvas.width, canvas.height);
                                        actx.drawImage(originalImage, 0, 0);
                                        compositeLayers();
                                    } else {
                                        cctx.clearRect(0, 0, canvas.width, canvas.height);
                                        cctx.drawImage(originalImage, 0, 0);
                                    }
                                }
            }),
            toggleCompare: Pixel.bindRuntime(runtime, function toggleCompare() {
                                if (!originalImage || !canvas.width) return;
                                compareMode = !compareMode;
                                if (compareMode) {
                                    comparePos = 0.5;
                                    compareOrigData = cctx.getImageData(0, 0, canvas.width, canvas.height);
                                    applyAdjustmentsPreview();
                                    renderCompare();
                                    compareDivider.hidden = false;
                                } else {
                                    compareOrigData = null;
                                    compareDivider.hidden = true;
                                    clearOverlay();
                                    if (originalImage && canvas.width) {
                                        if (layers.length > 1) {
                                            ensureBackgroundMigrated();
                                            const actx = layers[0].canvas.getContext('2d');
                                            actx.clearRect(0, 0, canvas.width, canvas.height);
                                            actx.drawImage(originalImage, 0, 0);
                                            compositeLayers();
                                        } else {
                                            cctx.clearRect(0, 0, canvas.width, canvas.height);
                                            cctx.drawImage(originalImage, 0, 0);
                                        }
                                    }
                                }
            }),
            renderCompare: Pixel.bindRuntime(runtime, function renderCompare() {
                                if (!compareMode || !compareOrigData) return;
                                const splitX = Math.round(canvas.width * comparePos);
                                const current = cctx.getImageData(0, 0, canvas.width, canvas.height);
                                cctx.putImageData(compareOrigData, 0, 0);
                                cctx.save();
                                cctx.beginPath();
                                cctx.rect(splitX, 0, canvas.width - splitX, canvas.height);
                                cctx.clip();
                                cctx.putImageData(current, 0, 0);
                                cctx.restore();
                                olCtx.clearRect(0, 0, overlayCanvas.width, overlayCanvas.height);
                                olCtx.strokeStyle = '#ffffff';
                                olCtx.lineWidth = 2;
                                olCtx.beginPath();
                                olCtx.moveTo(splitX, 0);
                                olCtx.lineTo(splitX, canvas.height);
                                olCtx.stroke();
                                olCtx.strokeStyle = '#000000';
                                olCtx.lineWidth = 1;
                                olCtx.beginPath();
                                olCtx.moveTo(splitX + 1, 0);
                                olCtx.lineTo(splitX + 1, canvas.height);
                                olCtx.stroke();
            }),
            rotateCanvas: Pixel.bindRuntime(runtime, function rotateCanvas(deg) {
                                if (!canvas.width) return;
                                const nextW = deg === 90 || deg === -90 ? canvas.height : canvas.width;
                                const nextH = deg === 90 || deg === -90 ? canvas.width : canvas.height;
                                const tmpCanvas = acquireTempCanvas(nextW, nextH);
                                const tmpCtx = tmpCanvas.getContext('2d');
                                tmpCtx.save();
                                if (deg === 90) { tmpCtx.translate(tmpCanvas.width, 0); tmpCtx.rotate(Math.PI / 2); }
                                else if (deg === -90) { tmpCtx.translate(0, tmpCanvas.height); tmpCtx.rotate(-Math.PI / 2); }
                                else if (deg === 180) { tmpCtx.translate(tmpCanvas.width, tmpCanvas.height); tmpCtx.rotate(Math.PI); }
                                layers.forEach(l => {
                                    if (l.canvas) {
                                        const lt = acquireTempCanvas(nextW, nextH);
                                        const lx = lt.getContext('2d');
                                        lx.save();
                                        if (deg === 90) { lx.translate(nextW, 0); lx.rotate(Math.PI / 2); }
                                        else if (deg === -90) { lx.translate(0, nextH); lx.rotate(-Math.PI / 2); }
                                        else if (deg === 180) { lx.translate(nextW, nextH); lx.rotate(Math.PI); }
                                        lx.drawImage(l.canvas, 0, 0);
                                        lx.restore();
                                        l.canvas.width = nextW;
                                        l.canvas.height = nextH;
                                        l.canvas.getContext('2d').drawImage(lt, 0, 0);
                                        releaseTempCanvas(lt);
                                    }
                                });
                                tmpCtx.drawImage(canvas, 0, 0);
                                tmpCtx.restore();
                                canvas.width = nextW;
                                canvas.height = nextH;
                                overlayCanvas.width = nextW;
                                overlayCanvas.height = nextH;
                                cctx.drawImage(tmpCanvas, 0, 0);
                                releaseTempCanvas(tmpCanvas);
                                imgWidth = nextW;
                                imgHeight = nextH;
                                if (layers.length > 1) compositeLayers();
                                originalImage = new Image();
                                originalImage.src = canvas.toDataURL();
                                pushHistory('rotate ' + deg);
                                applyZoom();
            }),
            flipCanvas: Pixel.bindRuntime(runtime, function flipCanvas(horizontal) {
                                if (!canvas.width) return;
                                layers.forEach(l => {
                                    if (!l.canvas) return;
                                    const tmp = acquireTempCanvas(l.canvas.width, l.canvas.height);
                                    const tx = tmp.getContext('2d');
                                    tx.save();
                                    if (horizontal) { tx.translate(l.canvas.width, 0); tx.scale(-1, 1); }
                                    else { tx.translate(0, l.canvas.height); tx.scale(1, -1); }
                                    tx.drawImage(l.canvas, 0, 0);
                                    tx.restore();
                                    l.canvas.getContext('2d').clearRect(0, 0, l.canvas.width, l.canvas.height);
                                    l.canvas.getContext('2d').drawImage(tmp, 0, 0);
                                    releaseTempCanvas(tmp);
                                });
                                const tmpCanvas = acquireTempCanvas(canvas.width, canvas.height);
                                const tmpCtx = tmpCanvas.getContext('2d');
                                tmpCtx.save();
                                if (horizontal) { tmpCtx.translate(canvas.width, 0); tmpCtx.scale(-1, 1); }
                                else { tmpCtx.translate(0, canvas.height); tmpCtx.scale(1, -1); }
                                tmpCtx.drawImage(canvas, 0, 0);
                                tmpCtx.restore();
                                cctx.clearRect(0, 0, canvas.width, canvas.height);
                                cctx.drawImage(tmpCanvas, 0, 0);
                                releaseTempCanvas(tmpCanvas);
                                if (layers.length > 1) compositeLayers();
                                pushHistory(horizontal ? 'flip-h' : 'flip-v');
            }),
            startCrop: Pixel.bindRuntime(runtime, function startCrop() {
                                if (!canvas.width) return;
                                cropState = { active: true, startX: 0, startY: 0, endX: 0, endY: 0 };
                                cropOverlay.hidden = false;
                                cropOverlay.innerHTML = '<div class="pixel-crop-selection" data-crop-sel></div>';
                                host.querySelector('[data-crop-actions]').hidden = false;
                                setStatus(t('pixel.crop_hint', 'Drag to select crop area'));
            }),
            cancelCrop: Pixel.bindRuntime(runtime, function cancelCrop() {
                                cropState = null;
                                cropOverlay.hidden = true;
                                cropOverlay.innerHTML = '';
                                const ca = host.querySelector('[data-crop-actions]');
                                if (ca) ca.hidden = true;
                                updateStatus();
            }),
            applyCrop: Pixel.bindRuntime(runtime, function applyCrop() {
                                if (!cropState) return;
                                const sel = host.querySelector('[data-crop-sel]');
                                if (!sel) { cancelCrop(); return; }
                                const displayW = parseFloat(canvas.style.width) || canvas.width;
                                const displayH = parseFloat(canvas.style.height) || canvas.height;
                                const scaleX = canvas.width / displayW;
                                const scaleY = canvas.height / displayH;
                                const sx = Math.max(0, Math.round(Math.min(cropState.startX, cropState.endX) * scaleX));
                                const sy = Math.max(0, Math.round(Math.min(cropState.startY, cropState.endY) * scaleY));
                                const sw = Math.round(Math.abs(cropState.endX - cropState.startX) * scaleX);
                                const sh = Math.round(Math.abs(cropState.endY - cropState.startY) * scaleY);
                                if (sw < 2 || sh < 2) { cancelCrop(); return; }
                                layers.forEach(l => {
                                    if (l.canvas) {
                                        const imgData = l.canvas.getContext('2d').getImageData(sx, sy, sw, sh);
                                        l.canvas.width = sw;
                                        l.canvas.height = sh;
                                        l.canvas.getContext('2d').putImageData(imgData, 0, 0);
                                    }
                                });
                                const imgData = cctx.getImageData(sx, sy, sw, sh);
                                canvas.width = sw;
                                canvas.height = sh;
                                overlayCanvas.width = sw;
                                overlayCanvas.height = sh;
                                cctx.putImageData(imgData, 0, 0);
                                imgWidth = sw;
                                imgHeight = sh;
                                originalImage = null;
                                if (layers.length > 1) compositeLayers();
                                pushHistory('crop');
                                cancelCrop();
                                zoomFit();
            }),
            resizeImage: Pixel.bindRuntime(runtime, function resizeImage() {
                                if (!canvas.width) return;
                                const dlg = document.createElement('div');
                                dlg.className = 'vd-modal-backdrop';
                                dlg.innerHTML = `<form class="vd-modal" role="dialog">
                                    <div class="vd-modal-title">${esc(t('pixel.resize', 'Resize'))}</div>
                                    <div class="pixel-resize-form">
                                        <label class="pixel-label">${esc(t('pixel.width', 'Width'))}</label>
                                        <input class="vd-modal-input" type="number" data-resize-w value="${imgWidth}" min="1">
                                        <label class="pixel-label">${esc(t('pixel.height', 'Height'))}</label>
                                        <input class="vd-modal-input" type="number" data-resize-h value="${imgHeight}" min="1">
                                        <label class="pixel-checkbox-row"><input type="checkbox" data-lock-ratio checked> ${esc(t('pixel.lock_ratio', 'Lock aspect ratio'))}</label>
                                    </div>
                                    <div class="vd-modal-actions">
                                        <button type="button" class="vd-button" data-cancel>${esc(t('pixel.cancel', 'Cancel'))}</button>
                                        <button type="submit" class="vd-button vd-button-primary">${esc(t('pixel.apply', 'Apply'))}</button>
                                    </div>
                                </form>`;
                                document.body.appendChild(dlg);
                                const wInput = dlg.querySelector('[data-resize-w]');
                                const hInput = dlg.querySelector('[data-resize-h]');
                                const lock = dlg.querySelector('[data-lock-ratio]');
                                const ratio = imgWidth / imgHeight;
                                wInput.addEventListener('input', () => { if (lock.checked) hInput.value = Math.round(wInput.value / ratio); });
                                hInput.addEventListener('input', () => { if (lock.checked) wInput.value = Math.round(hInput.value * ratio); });
                                dlg.querySelector('form').addEventListener('submit', e => {
                                    e.preventDefault();
                                    const nw = parseInt(wInput.value) || imgWidth;
                                    const nh = parseInt(hInput.value) || imgHeight;
                                    if (nw < 1 || nh < 1) return;
                                    layers.forEach(l => {
                                        if (l.canvas) {
                                            const tmp = acquireTempCanvas(nw, nh);
                                            tmp.getContext('2d').drawImage(l.canvas, 0, 0, nw, nh);
                                            l.canvas.width = nw;
                                            l.canvas.height = nh;
                                            l.canvas.getContext('2d').drawImage(tmp, 0, 0);
                                            releaseTempCanvas(tmp);
                                        }
                                    });
                                    const tmpCanvas = acquireTempCanvas(nw, nh);
                                    tmpCanvas.getContext('2d').drawImage(canvas, 0, 0, nw, nh);
                                    canvas.width = nw;
                                    canvas.height = nh;
                                    overlayCanvas.width = nw;
                                    overlayCanvas.height = nh;
                                    cctx.drawImage(tmpCanvas, 0, 0);
                                    releaseTempCanvas(tmpCanvas);
                                    imgWidth = nw;
                                    imgHeight = nh;
                                    if (layers.length > 1) compositeLayers();
                                    pushHistory('resize');
                                    zoomFit();
                                    dlg.remove();
                                });
                                dlg.querySelector('[data-cancel]').addEventListener('click', () => dlg.remove());
                                dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });
            })
        });
    };
})();
