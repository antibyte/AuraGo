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
            loadPhotos: Pixel.bindRuntime(runtime, async function loadPhotos() {
                                const grid = this.host.querySelector('[data-photos-grid]');
                                if (!grid) return;
                                try {
                                    const resp = await this.api('/api/desktop/files?path=Pictures&recursive=true&limit=30');
                                    if (!resp || !resp.files || !resp.files.length) { grid.innerHTML = `<span class="pixel-photos-empty">${this.esc(this.t('pixel.no_photos'))}</span>`; return; }
                                    const imageFiles = resp.files.filter(f => f.is_dir === false && this.IMAGE_EXTS.some(ext => (f.name || '').toLowerCase().endsWith('.' + ext)));
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
                                const result = await this.ctx.openFileDialog({ title: this.t('pixel.open'), initialPath: 'Photos', filters: [{ name: 'Images', extensions: this.IMAGE_EXTS }] });
                                if (result && !result.canceled && result.path) {
                                    this.filePath = result.path;
                                    this.fileName = this.filePath.split('/').pop();
                                    this.isDirty = false;
                                    this.saveRecentFile(this.filePath);
                                    try {
                                        const preview = await this.api('/api/desktop/preview?path=' + encodeURIComponent(this.filePath));
                                        if (preview && preview.url) {
                                            await this.loadImageToCanvas(preview.url);
                                        } else {
                                            await this.loadImageToCanvas('/api/desktop/preview?path=' + encodeURIComponent(this.filePath) + '&raw=1');
                                        }
                                    } catch (_) {
                                        this.notify({ type: 'error', message: this.t('pixel.error_load') });
                                    }
                                }
            }),
            saveFile: Pixel.bindRuntime(runtime, async function saveFile() {
                                if (!this.canvas.width) return;
                                if (!this.filePath) { await this.saveFileAs(); return; }
                                this.setStatus(this.t('pixel.status_saving'));
                                try {
                                    const tmpC = this.acquireTempCanvas(this.canvas.width, this.canvas.height);
                                    const tmpX = tmpC.getContext('2d');
                                    for (let i = 0; i < this.layers.length; i++) {
                                        if (!this.layers[i].visible) continue;
                                        tmpX.globalAlpha = this.layers[i].opacity;
                                        if (this.layers[i].canvas) tmpX.drawImage(this.layers[i].canvas, 0, 0);
                                        else { tmpX.globalAlpha = 1; tmpX.drawImage(this.canvas, 0, 0); }
                                    }
                                    tmpX.globalAlpha = 1;
                                    const ext = (this.filePath.split('.').pop() || 'png').toLowerCase();
                                    const isJPEG = ext === 'jpg' || ext === 'jpeg';
                                    const dataURL = isJPEG ? tmpC.toDataURL('image/jpeg', 0.92) : tmpC.toDataURL('image/png');
                                    this.releaseTempCanvas(tmpC);
                                    await this.api('/api/pixel/save', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ path: this.filePath, data: dataURL, format: isJPEG ? 'jpeg' : 'png' })
                                    });
                                    this.isDirty = false;
                                    this.updateStatus();
                                    this.notify({ type: 'success', message: this.t('pixel.saved') });
                                } catch (err) {
                                    this.notify({ type: 'error', message: this.t('pixel.error_save') });
                                    this.setStatus(this.t('pixel.error_save'));
                                }
            }),
            saveFileAs: Pixel.bindRuntime(runtime, async function saveFileAs() {
                                if (!this.canvas.width) return;
                                if (!this.ctx.saveFileDialog) return;
                                const result = await this.ctx.saveFileDialog({ filters: [{ name: 'PNG Image', extensions: ['png'] }, { name: 'JPEG Image', extensions: ['jpg'] }, { name: 'WebP Image', extensions: ['webp'] }] });
                                if (result && !result.canceled && result.path) {
                                    this.filePath = result.path;
                                    this.fileName = this.filePath.split('/').pop();
                                    await this.saveFile();
                                }
            }),
            exportFile: Pixel.bindRuntime(runtime, function exportFile() {
                                if (!this.canvas.width) return;
                                const tmpC = this.acquireTempCanvas(this.canvas.width, this.canvas.height);
                                const tmpX = tmpC.getContext('2d');
                                for (let i = 0; i < this.layers.length; i++) {
                                    if (!this.layers[i].visible) continue;
                                    tmpX.globalAlpha = this.layers[i].opacity;
                                    if (this.layers[i].canvas) tmpX.drawImage(this.layers[i].canvas, 0, 0);
                                }
                                tmpX.globalAlpha = 1;
                                const link = document.createElement('a');
                                link.download = this.fileName || 'image.png';
                                link.href = tmpC.toDataURL('image/png');
                                link.click();
                                this.releaseTempCanvas(tmpC);
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
                                    const dataURL = this.canvas.toDataURL('image/png');
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
            checkAIConfig: Pixel.bindRuntime(runtime, async function checkAIConfig() {
                                try {
                                    const cfg = await this.api('/api/pixel/config');
                                    this.aiConfigured = cfg && cfg.enabled;
                                    const statusEl = this.host.querySelector('[data-ai-status]');
                                    if (!this.aiConfigured && statusEl) {
                                        statusEl.textContent = this.t('pixel.no_ai_config');
                                    }
                                } catch (_) {}
            })
        });
    };
})();
