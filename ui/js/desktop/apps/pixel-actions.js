(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installActions = function installActions(runtime) {
        Object.assign(runtime, {
            showNewImageDialog: Pixel.bindRuntime(runtime, function showNewImageDialog() {
                                const dlg = document.createElement('div');
                                dlg.className = 'vd-modal-backdrop';
                                dlg.innerHTML = `<form class="vd-modal" role="dialog">
                                    <div class="vd-modal-title">${this.esc(this.t('pixel.new_image'))}</div>
                                    <div class="pixel-resize-form">
                                        <label class="pixel-label">${this.esc(this.t('pixel.width'))}</label>
                                        <input class="vd-modal-input" type="number" data-new-w value="1024" min="1" max="8192">
                                        <label class="pixel-label">${this.esc(this.t('pixel.height'))}</label>
                                        <input class="vd-modal-input" type="number" data-new-h value="1024" min="1" max="8192">
                                    </div>
                                    <div class="vd-modal-actions">
                                        <button type="button" class="vd-button" data-cancel>${this.esc(this.t('pixel.cancel'))}</button>
                                        <button type="submit" class="vd-button vd-button-primary">${this.esc(this.t('pixel.create'))}</button>
                                    </div>
                                </form>`;
                                document.body.appendChild(dlg);
                                dlg.querySelector('form').addEventListener('submit', e => {
                                    e.preventDefault();
                                    const w = parseInt(dlg.querySelector('[data-new-w]').value) || 1024;
                                    const h = parseInt(dlg.querySelector('[data-new-h]').value) || 1024;
                                    this.state.imageRead?.abort();
                                    this.filePath = '';
                                    this.state.fileVersion = '';
                                    this.state.versionPath = '';
                                    this.newBlankCanvas(Math.min(w, 8192), Math.min(h, 8192));
                                    dlg.remove();
                                });
                                dlg.querySelector('[data-cancel]').addEventListener('click', () => dlg.remove());
                                dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });
            }),
            loadRecentFiles: Pixel.bindRuntime(runtime, function loadRecentFiles() {
                                try {
                                    const stored = localStorage.getItem('pixel_recent_files');
                                    return stored ? JSON.parse(stored) : [];
                                } catch (_) { return []; }
            }),
            saveRecentFile: Pixel.bindRuntime(runtime, function saveRecentFile(path) {
                                let recent = this.loadRecentFiles().filter(p => p !== path);
                                recent.unshift(path);
                                if (recent.length > 5) recent = recent.slice(0, 5);
                                try { localStorage.setItem('pixel_recent_files', JSON.stringify(recent)); } catch (_) {}
            }),
            renderRecentFiles: Pixel.bindRuntime(runtime, function renderRecentFiles() {
                                const recent = this.loadRecentFiles();
                                const container = this.host.querySelector('[data-recent-files]');
                                if (!container || !recent.length) { if (container) container.innerHTML = ''; return; }
                                container.innerHTML = `<span class="pixel-label">${this.esc(this.t('pixel.recent_files'))}</span>` +
                                    recent.map(p => `<button class="pixel-recent-file-btn" type="button" data-recent-path="${this.esc(p)}" title="${this.esc(p)}">${this.esc(p.split('/').pop())}</button>`).join('');
            }),
            loadDesktopImagePath: Pixel.bindRuntime(runtime, async function loadDesktopImagePath(path, options) {
                                const nextPath = String(path || '').trim();
                                if (!nextPath) return;
                                // Layered OpenRaster projects are fetched and decoded by pixel-ora.js.
                                if (/\.ora$/i.test(nextPath) && this.loadOraProject) return this.loadOraProject(nextPath, options);
                                this.state.imageRead?.abort();
                                const controller = new AbortController();
                                this.state.imageRead = controller;
                                let objectURL = '';
                                try {
                                    const response = await fetch('/api/desktop/preview?path=' + encodeURIComponent(nextPath), { credentials: 'same-origin', cache: 'no-store', signal: controller.signal });
                                    if (!response.ok) throw new Error('image');
                                    objectURL = URL.createObjectURL(await response.blob());
                                    const current = () => !controller.signal.aborted && !this.state.disposed && this.state.imageRead === controller;
                                    if (!await this.loadImageToCanvas(objectURL, current)) return;
                                    this.filePath = nextPath;
                                    this.fileName = nextPath.split('/').pop();
                                    this.state.fileVersion = response.headers.get('ETag') || '';
                                    this.state.versionPath = nextPath;
                                    this.isDirty = false;
                                    if (!options || options.saveRecent !== false) this.saveRecentFile(nextPath);
                                    this.updateStatus();
                                } catch (err) {
                                    if (err.name !== 'AbortError' && !this.state.disposed) this.notify({ type: 'error', message: this.t('pixel.error_load') });
                                } finally { if (objectURL) URL.revokeObjectURL(objectURL); }
            }),
            loadPhotos: Pixel.bindRuntime(runtime, async function loadPhotos() {
                                const grid = this.host.querySelector('[data-photos-grid]');
                                if (!grid) return;
                                try {
                                    const resp = await this.api('/api/desktop/files?path=Pictures&recursive=true&limit=30');
                                    if (!resp || !resp.files || !resp.files.length) { grid.innerHTML = `<span class="pixel-photos-empty">${this.esc(this.t('pixel.no_photos'))}</span>`; return; }
                                    // .ora projects have no raster preview, so they stay out of the photo thumbnails (they open from the dialog and the recent list).
                                    const imageFiles = resp.files.filter(f => f.is_dir === false && this.IMAGE_EXTS.some(ext => ext !== 'ora' && (f.name || '').toLowerCase().endsWith('.' + ext)));
                                    if (!imageFiles.length) { grid.innerHTML = `<span class="pixel-photos-empty">${this.esc(this.t('pixel.no_photos'))}</span>`; return; }
                                    grid.innerHTML = imageFiles.slice(0, 12).map(f => {
                                        const name = f.name || f.path || '';
                                        const previewUrl = '/api/desktop/preview?path=' + encodeURIComponent(f.path) + '&thumb=1';
                                        return `<button class="pixel-photo-thumb" type="button" data-photo-path="${this.esc(f.path)}" title="${this.esc(name)}"><img src="${this.esc(previewUrl)}" alt="${this.esc(name)}" loading="lazy"><span class="pixel-photo-name">${this.esc(name)}</span></button>`;
                                    }).join('');
                                } catch (_) { grid.innerHTML = ''; }
            }),
            openFile: Pixel.bindRuntime(runtime, async function openFile() {
                                if (!this.ctx.openFileDialog) return;
                                const filters = [{ label: this.t('desktop.file_dialog_images'), extensions: this.IMAGE_EXTS }];
                                if (this.loadOraProject) filters.push({ label: (this.oraText || this.t).call(this, 'desktop.file_dialog_ora', 'OpenRaster project (.ora)'), extensions: ['ora'] });
                                const result = await this.ctx.openFileDialog({ title: this.t('pixel.open'), initialPath: 'Photos', filters });
                                if (result && !result.canceled && result.path) {
                                    await this.loadDesktopImagePath(result.path);
                                }
            }),
            saveFile: Pixel.bindRuntime(runtime, async function saveFile(destination) {
                                if (!this.canvas.width || this.ctx.readonly || this.state.saveBusy) return;
                                const path = typeof destination === 'string' ? destination : this.filePath;
                                if (!path || /^(Photos|Music|Videos|AuraGo Documents)\//i.test(path)) { await this.saveFileAs(); return; }
                                const previousPath = this.filePath;
                                const historyEntry = this.history[this.historyIdx];
                                const version = this.state.versionPath === path ? this.state.fileVersion : '';
                                const ext = (path.split('.').pop() || 'png').toLowerCase();
                                const isJPEG = ext === 'jpg' || ext === 'jpeg';
                                // .ora keeps layers, masks and blend modes (pixel-ora.js); PNG/JPEG save the flattened picture.
                                const isORA = ext === 'ora' && !!this.oraDataURL;
                                this.state.saveBusy = true;
                                this.setStatus(this.t('pixel.status_saving'));
                                try {
                                    const dataURL = isORA ? await this.oraDataURL() : (isJPEG ? this.flattenToDataURL('image/jpeg', 0.92) : this.flattenToDataURL('image/png'));
                                    const saved = await this.api('/api/pixel/save', {
                                        method: 'POST',
                                        headers: Object.assign({ 'Content-Type': 'application/json' }, version ? { 'If-Match': version } : { 'If-None-Match': '*' }),
                                        body: JSON.stringify({ path, data: dataURL, format: isORA ? 'ora' : (isJPEG ? 'jpeg' : 'png') })
                                    });
                                    if (this.state.disposed || this.filePath !== previousPath) return;
                                    this.filePath = saved.path || path;
                                    this.fileName = this.filePath.split('/').pop();
                                    this.state.fileVersion = saved.version || '';
                                    this.state.versionPath = this.filePath;
                                    this.isDirty = this.history[this.historyIdx] !== historyEntry;
                                    this.updateStatus();
                                    this.notify({ type: 'success', message: this.t('pixel.saved') });
                                    if (!isORA && this.noteFlattenedLayers) this.noteFlattenedLayers();
                                } catch (err) {
                                    if (err.name === 'AbortError' || this.state.disposed) return;
                                    const failure = isORA && this.oraErrorMessage ? this.oraErrorMessage(err, 'save') : this.t('pixel.error_save');
                                    this.notify({ type: 'error', message: failure });
                                    this.setStatus(failure);
                                } finally { this.state.saveBusy = false; }
            }),
            saveFileAs: Pixel.bindRuntime(runtime, async function saveFileAs() {
                                if (!this.canvas.width || this.ctx.readonly || this.state.saveBusy) return;
                                if (!this.ctx.saveFileDialog) return;
                                const filters = [
                                    { label: this.t('desktop.file_dialog_png'), extensions: ['png'] },
                                    { label: this.t('desktop.file_dialog_jpeg'), extensions: ['jpg', 'jpeg'] }
                                ];
                                if (this.oraDataURL) {
                                    const project = { label: (this.oraText || this.t).call(this, 'desktop.file_dialog_ora', 'OpenRaster project (.ora)'), extensions: ['ora'] };
                                    // A document with layers or masks offers the project format first, so it is the default type.
                                    if (this.layers.length > 1 || this.layers.some(layer => layer.maskCanvas)) filters.unshift(project); else filters.push(project);
                                }
                                // A name typed without an extension gets the extension of the first (default) type.
                                const result = await this.ctx.saveFileDialog({ initialPath: 'Pictures', defaultExtension: filters[0].extensions[0], filters });
                                if (result && !result.canceled && result.path) {
                                    await this.saveFile(result.path);
                                }
            }),
            exportFile: Pixel.bindRuntime(runtime, function exportFile() {
                                this.showExportDialog();
            }),
            showExportDialog: Pixel.bindRuntime(runtime, function showExportDialog() {
                                if (!this.canvas.width) return;
                                const dlg = document.createElement('div');
                                dlg.className = 'vd-modal-backdrop';
                                dlg.innerHTML = `<form class="vd-modal" role="dialog">
                                    <div class="vd-modal-title">${this.esc(this.t('pixel.export'))}</div>
                                    <div class="pixel-resize-form">
                                        <label class="pixel-label">${this.esc(this.t('pixel.export_format'))}</label>
                                        <select class="pixel-select" data-export-format>
                                            <option value="png">${this.esc(this.t('pixel.export_png'))}</option>
                                            <option value="jpeg">${this.esc(this.t('pixel.export_jpeg'))}</option>
                                            <option value="webp">${this.esc(this.t('pixel.export_webp'))}</option>
                                        </select>
                                        <label class="pixel-label">${this.esc(this.t('pixel.export_quality'))}</label>
                                        <input class="vd-modal-input" type="range" data-export-quality min="50" max="100" value="92">
                                        <span data-export-quality-val>92%</span>
                                    </div>
                                    <div class="vd-modal-actions">
                                        <button type="button" class="vd-button" data-cancel>${this.esc(this.t('pixel.cancel'))}</button>
                                        <button type="submit" class="vd-button vd-button-primary">${this.esc(this.t('pixel.export'))}</button>
                                    </div>
                                </form>`;
                                document.body.appendChild(dlg);
                                const qSlider = dlg.querySelector('[data-export-quality]');
                                const qVal = dlg.querySelector('[data-export-quality-val]');
                                qSlider.addEventListener('input', () => { qVal.textContent = qSlider.value + '%'; });
                                dlg.querySelector('form').addEventListener('submit', e => {
                                    e.preventDefault();
                                    const fmt = dlg.querySelector('[data-export-format]').value;
                                    const quality = Number(qSlider.value) / 100;
                                    this.exportWithFormat(fmt, quality);
                                    dlg.remove();
                                });
                                dlg.querySelector('[data-cancel]').addEventListener('click', () => dlg.remove());
                                dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });
            }),
            exportWithFormat: Pixel.bindRuntime(runtime, function exportWithFormat(format, quality) {
                                const mime = format === 'jpeg' ? 'image/jpeg' : (format === 'webp' ? 'image/webp' : 'image/png');
                                const ext = format === 'jpeg' ? 'jpg' : format;
                                const link = document.createElement('a');
                                link.download = (this.fileName || 'image').replace(/\.[^.]+$/, '') + '.' + ext;
                                link.href = this.flattenToDataURL(mime, quality || 0.92);
                                link.click();
            }),
            // PNG/JPEG/WebP data URL of the visible layers merged, without any adjust or filter preview.
            flattenToDataURL: Pixel.bindRuntime(runtime, function flattenToDataURL(mime, quality) {
                                const flat = this.flattenDocument(this.acquireTempCanvas(this.canvas.width, this.canvas.height));
                                const url = flat.toDataURL(mime || 'image/png', quality);
                                this.releaseTempCanvas(flat);
                                return url;
            }),
            applyAIResult: Pixel.bindRuntime(runtime, async function applyAIResult(resp) {
                                if (!resp || !resp.url) return;
                                const mode = (this.host.querySelector('[data-ai-result-mode]') || {}).value || 'layer';
                                if (mode === 'layer' && this.layers.length < this.MAX_LAYERS) {
                                    const img = await this.loadImage(resp.url);
                                    const nw = img.naturalWidth;
                                    const nh = img.naturalHeight;
                                    if (nw > this.canvas.width || nh > this.canvas.height) {
                                        this.expandCanvasToFit(nw, nh);
                                    }
                                    this.commitPendingPreviews();
                                    // Layers are document-sized, so the result is drawn at the origin after the document has grown.
                                    this.layers.push(this.createLayer({ name: this.t('pixel.ai_layer'), source: img }));
                                    this.activeLayerIdx = this.layers.length - 1;
                                    this.editMaskMode = false;
                                    this.layersStructureChanged();
                                    this.pushHistory('ai');
                                } else {
                                    await this.loadImageToCanvas(resp.url);
                                    this.filePath = resp.path || '';
                                    this.fileName = this.filePath ? this.filePath.split('/').pop() : '';
                                }
            }),
            aiGenerate: Pixel.bindRuntime(runtime, async function aiGenerate() {
                                const prompt = (this.host.querySelector('[data-ai-prompt]') || {}).value || '';
                                if (!prompt.trim()) return;
                                const size = (this.host.querySelector('[data-ai-size]') || {}).value || '1024x1024';
                                const quality = (this.host.querySelector('[data-ai-quality]') || {}).value || 'standard';
                                const style = (this.host.querySelector('[data-ai-style]') || {}).value || 'vivid';
                                const statusEl = this.host.querySelector('[data-ai-status]');
                                if (statusEl) statusEl.textContent = this.t('pixel.generating');
                                try {
                                    this.abortCtrl = new AbortController();
                                    const resp = await this.api('/api/pixel/generate', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ prompt, size, quality, style }),
                                        signal: this.abortCtrl.signal
                                    });
                                    if (resp && resp.url) {
                                        await this.loadImageToCanvas(resp.url);
                                        this.filePath = resp.path || '';
                                        this.fileName = this.filePath ? this.filePath.split('/').pop() : '';
                                    }
                                    if (statusEl) statusEl.textContent = '';
                                    this.notify({ type: 'success', message: this.t('pixel.generated') });
                                } catch (err) {
                                    if (statusEl) statusEl.textContent = '';
                                    if (err.name !== 'AbortError') this.notify({ type: 'error', message: this.t('pixel.error_generate') });
                                } finally { this.abortCtrl = null; }
            }),
            aiEnhance: Pixel.bindRuntime(runtime, async function aiEnhance() {
                                const prompt = (this.host.querySelector('[data-enhance-prompt]') || {}).value || '';
                                const strength = parseFloat((this.host.querySelector('[data-enhance-strength]') || {}).value) || 0.7;
                                if (!this.canvas.width) return;
                                const statusEl = this.host.querySelector('[data-ai-status]');
                                if (statusEl) statusEl.textContent = this.t('pixel.generating');
                                try {
                                    const dataURL = this.flattenToDataURL('image/png');
                                    this.abortCtrl = new AbortController();
                                    const resp = await this.api('/api/pixel/enhance', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ source_data: dataURL, prompt, strength }),
                                        signal: this.abortCtrl.signal
                                    });
                                    if (resp && resp.url) {
                                        await this.loadImageToCanvas(resp.url);
                                        this.filePath = resp.path || '';
                                        this.fileName = this.filePath ? this.filePath.split('/').pop() : '';
                                    }
                                    if (statusEl) statusEl.textContent = '';
                                    this.notify({ type: 'success', message: this.t('pixel.enhanced') });
                                } catch (err) {
                                    if (statusEl) statusEl.textContent = '';
                                    if (err.name !== 'AbortError') this.notify({ type: 'error', message: this.t('pixel.error_generate') });
                                } finally { this.abortCtrl = null; }
            }),
            aiRemoveBG: Pixel.bindRuntime(runtime, async function aiRemoveBG() {
                                if (!this.canvas.width) return;
                                const statusEl = this.host.querySelector('[data-ai-status]');
                                if (statusEl) statusEl.textContent = this.t('pixel.generating');
                                try {
                                    const dataURL = this.flattenToDataURL('image/png');
                                    this.abortCtrl = new AbortController();
                                    const resp = await this.api('/api/pixel/remove-bg', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ source_data: dataURL }),
                                        signal: this.abortCtrl.signal
                                    });
                                    await this.applyAIResult(resp);
                                    if (statusEl) statusEl.textContent = '';
                                    this.notify({ type: 'success', message: this.t('pixel.remove_bg_done') });
                                } catch (err) {
                                    if (statusEl) statusEl.textContent = '';
                                    if (err.name !== 'AbortError') this.notify({ type: 'error', message: this.t('pixel.error_generate') });
                                } finally { this.abortCtrl = null; }
            }),
            aiUpscale: Pixel.bindRuntime(runtime, async function aiUpscale() {
                                if (!this.canvas.width) return;
                                const statusEl = this.host.querySelector('[data-ai-status]');
                                if (statusEl) statusEl.textContent = this.t('pixel.generating');
                                try {
                                    const dataURL = this.flattenToDataURL('image/png');
                                    this.abortCtrl = new AbortController();
                                    const resp = await this.api('/api/pixel/upscale', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ source_data: dataURL, scale: 2 }),
                                        signal: this.abortCtrl.signal
                                    });
                                    await this.applyAIResult(resp);
                                    if (statusEl) statusEl.textContent = '';
                                    this.notify({ type: 'success', message: this.t('pixel.upscale_done') });
                                } catch (err) {
                                    if (statusEl) statusEl.textContent = '';
                                    if (err.name !== 'AbortError') this.notify({ type: 'error', message: this.t('pixel.error_generate') });
                                } finally { this.abortCtrl = null; }
            }),
            checkAIConfig: Pixel.bindRuntime(runtime, async function checkAIConfig() {
                                try {
                                    const cfg = await this.api('/api/pixel/config');
                                    this.aiConfigured = cfg && cfg.enabled;
                                    this.aiSupportsRemoveBG = !!(cfg && cfg.supports_remove_bg);
                                    this.aiSupportsUpscale = !!(cfg && cfg.supports_upscale);
                                    const statusEl = this.host.querySelector('[data-ai-status]');
                                    if (!this.aiConfigured && statusEl) {
                                        statusEl.textContent = this.t('pixel.no_ai_config');
                                    }
                                    const removeBtn = this.host.querySelector('[data-ai-remove-bg]');
                                    const upscaleBtn = this.host.querySelector('[data-ai-upscale]');
                                    if (removeBtn) {
                                        removeBtn.disabled = !this.aiSupportsRemoveBG;
                                        removeBtn.title = this.aiSupportsRemoveBG ? '' : this.t('pixel.remove_bg_unavailable');
                                    }
                                    if (upscaleBtn) {
                                        upscaleBtn.disabled = !this.aiSupportsUpscale;
                                        upscaleBtn.title = this.aiSupportsUpscale ? '' : this.t('pixel.upscale_unavailable');
                                    }
                                } catch (_) {}
            })
        });
    };
})();
