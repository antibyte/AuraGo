(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installActions = function installActions(runtime) {
        Object.assign(runtime, {
            showNewImageDialog: Pixel.bindRuntime(runtime, function showNewImageDialog() {
                                const dlg = document.createElement('div');
                                dlg.className = 'vd-modal-backdrop';
                                dlg.innerHTML = `<form class="vd-modal" role="dialog">
                                    <div class="vd-modal-title">${esc(t('pixel.new_image', 'New Image'))}</div>
                                    <div class="pixel-resize-form">
                                        <label class="pixel-label">${esc(t('pixel.width', 'Width'))}</label>
                                        <input class="vd-modal-input" type="number" data-new-w value="1024" min="1" max="8192">
                                        <label class="pixel-label">${esc(t('pixel.height', 'Height'))}</label>
                                        <input class="vd-modal-input" type="number" data-new-h value="1024" min="1" max="8192">
                                    </div>
                                    <div class="vd-modal-actions">
                                        <button type="button" class="vd-button" data-cancel>${esc(t('pixel.cancel', 'Cancel'))}</button>
                                        <button type="submit" class="vd-button vd-button-primary">${esc(t('pixel.create', 'Create'))}</button>
                                    </div>
                                </form>`;
                                document.body.appendChild(dlg);
                                dlg.querySelector('form').addEventListener('submit', e => {
                                    e.preventDefault();
                                    const w = parseInt(dlg.querySelector('[data-new-w]').value) || 1024;
                                    const h = parseInt(dlg.querySelector('[data-new-h]').value) || 1024;
                                    newBlankCanvas(Math.min(w, 8192), Math.min(h, 8192));
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
                                let recent = loadRecentFiles().filter(p => p !== path);
                                recent.unshift(path);
                                if (recent.length > 5) recent = recent.slice(0, 5);
                                try { localStorage.setItem('pixel_recent_files', JSON.stringify(recent)); } catch (_) {}
            }),
            renderRecentFiles: Pixel.bindRuntime(runtime, function renderRecentFiles() {
                                const recent = loadRecentFiles();
                                const container = host.querySelector('[data-recent-files]');
                                if (!container || !recent.length) { if (container) container.innerHTML = ''; return; }
                                container.innerHTML = `<span class="pixel-label">${esc(t('pixel.recent_files', 'Recent'))}</span>` +
                                    recent.map(p => `<button class="pixel-recent-file-btn" type="button" data-recent-path="${esc(p)}" title="${esc(p)}">${esc(p.split('/').pop())}</button>`).join('');
            }),
            loadPhotos: Pixel.bindRuntime(runtime, async function loadPhotos() {
                                const grid = host.querySelector('[data-photos-grid]');
                                if (!grid) return;
                                try {
                                    const resp = await api('/api/desktop/files?path=Pictures&recursive=true&limit=30');
                                    if (!resp || !resp.files || !resp.files.length) { grid.innerHTML = `<span class="pixel-photos-empty">${esc(t('pixel.no_photos', 'No photos found'))}</span>`; return; }
                                    const imageFiles = resp.files.filter(f => f.is_dir === false && IMAGE_EXTS.some(ext => (f.name || '').toLowerCase().endsWith('.' + ext)));
                                    if (!imageFiles.length) { grid.innerHTML = `<span class="pixel-photos-empty">${esc(t('pixel.no_photos', 'No photos found'))}</span>`; return; }
                                    grid.innerHTML = imageFiles.slice(0, 12).map(f => {
                                        const name = f.name || f.path || '';
                                        const previewUrl = '/api/desktop/preview?path=' + encodeURIComponent(f.path) + '&thumb=1';
                                        return `<button class="pixel-photo-thumb" type="button" data-photo-path="${esc(f.path)}" title="${esc(name)}"><img src="${esc(previewUrl)}" alt="${esc(name)}" loading="lazy"><span class="pixel-photo-name">${esc(name)}</span></button>`;
                                    }).join('');
                                } catch (_) { grid.innerHTML = ''; }
            }),
            openFile: Pixel.bindRuntime(runtime, async function openFile() {
                                if (!ctx.openFileDialog) return;
                                const result = await ctx.openFileDialog({ filters: [{ name: 'Images', extensions: IMAGE_EXTS }] });
                                if (result && !result.canceled && result.path) {
                                    filePath = result.path;
                                    fileName = filePath.split('/').pop();
                                    isDirty = false;
                                    saveRecentFile(filePath);
                                    try {
                                        const preview = await api('/api/desktop/preview?path=' + encodeURIComponent(filePath));
                                        if (preview && preview.url) {
                                            await loadImageToCanvas(preview.url);
                                        } else {
                                            await loadImageToCanvas('/api/desktop/preview?path=' + encodeURIComponent(filePath) + '&raw=1');
                                        }
                                    } catch (_) {
                                        notify({ type: 'error', message: t('pixel.error_load', 'Failed to load image') });
                                    }
                                }
            }),
            saveFile: Pixel.bindRuntime(runtime, async function saveFile() {
                                if (!canvas.width) return;
                                if (!filePath) { await saveFileAs(); return; }
                                setStatus(t('pixel.status_saving', 'Saving...'));
                                try {
                                    const tmpC = acquireTempCanvas(canvas.width, canvas.height);
                                    const tmpX = tmpC.getContext('2d');
                                    for (let i = 0; i < layers.length; i++) {
                                        if (!layers[i].visible) continue;
                                        tmpX.globalAlpha = layers[i].opacity;
                                        if (layers[i].canvas) tmpX.drawImage(layers[i].canvas, 0, 0);
                                        else { tmpX.globalAlpha = 1; tmpX.drawImage(canvas, 0, 0); }
                                    }
                                    tmpX.globalAlpha = 1;
                                    const ext = (filePath.split('.').pop() || 'png').toLowerCase();
                                    const isJPEG = ext === 'jpg' || ext === 'jpeg';
                                    const dataURL = isJPEG ? tmpC.toDataURL('image/jpeg', 0.92) : tmpC.toDataURL('image/png');
                                    releaseTempCanvas(tmpC);
                                    await api('/api/pixel/save', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ path: filePath, data: dataURL, format: isJPEG ? 'jpeg' : 'png' })
                                    });
                                    isDirty = false;
                                    updateStatus();
                                    notify({ type: 'success', message: t('pixel.saved', 'Image saved') });
                                } catch (err) {
                                    notify({ type: 'error', message: t('pixel.error_save', 'Failed to save') });
                                    setStatus(t('pixel.error_save', 'Failed to save'));
                                }
            }),
            saveFileAs: Pixel.bindRuntime(runtime, async function saveFileAs() {
                                if (!canvas.width) return;
                                if (!ctx.saveFileDialog) return;
                                const result = await ctx.saveFileDialog({ filters: [{ name: 'PNG Image', extensions: ['png'] }, { name: 'JPEG Image', extensions: ['jpg'] }, { name: 'WebP Image', extensions: ['webp'] }] });
                                if (result && !result.canceled && result.path) {
                                    filePath = result.path;
                                    fileName = filePath.split('/').pop();
                                    await saveFile();
                                }
            }),
            exportFile: Pixel.bindRuntime(runtime, function exportFile() {
                                if (!canvas.width) return;
                                const tmpC = acquireTempCanvas(canvas.width, canvas.height);
                                const tmpX = tmpC.getContext('2d');
                                for (let i = 0; i < layers.length; i++) {
                                    if (!layers[i].visible) continue;
                                    tmpX.globalAlpha = layers[i].opacity;
                                    if (layers[i].canvas) tmpX.drawImage(layers[i].canvas, 0, 0);
                                }
                                tmpX.globalAlpha = 1;
                                const link = document.createElement('a');
                                link.download = fileName || 'image.png';
                                link.href = tmpC.toDataURL('image/png');
                                link.click();
                                releaseTempCanvas(tmpC);
            }),
            aiGenerate: Pixel.bindRuntime(runtime, async function aiGenerate() {
                                const prompt = (host.querySelector('[data-ai-prompt]') || {}).value || '';
                                if (!prompt.trim()) return;
                                const size = (host.querySelector('[data-ai-size]') || {}).value || '1024x1024';
                                const quality = (host.querySelector('[data-ai-quality]') || {}).value || 'standard';
                                const style = (host.querySelector('[data-ai-style]') || {}).value || 'vivid';
                                const statusEl = host.querySelector('[data-ai-status]');
                                if (statusEl) statusEl.textContent = t('pixel.generating', 'Generating...');
                                try {
                                    abortCtrl = new AbortController();
                                    const resp = await api('/api/pixel/generate', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ prompt, size, quality, style }),
                                        signal: abortCtrl.signal
                                    });
                                    if (resp && resp.url) {
                                        await loadImageToCanvas(resp.url);
                                        filePath = resp.path || '';
                                        fileName = filePath ? filePath.split('/').pop() : '';
                                    }
                                    if (statusEl) statusEl.textContent = '';
                                    notify({ type: 'success', message: t('pixel.generated', 'Image generated') });
                                } catch (err) {
                                    if (statusEl) statusEl.textContent = '';
                                    if (err.name !== 'AbortError') notify({ type: 'error', message: t('pixel.error_generate', 'Generation failed') });
                                } finally { abortCtrl = null; }
            }),
            aiEnhance: Pixel.bindRuntime(runtime, async function aiEnhance() {
                                const prompt = (host.querySelector('[data-enhance-prompt]') || {}).value || '';
                                const strength = parseFloat((host.querySelector('[data-enhance-strength]') || {}).value) || 0.7;
                                if (!canvas.width) return;
                                const statusEl = host.querySelector('[data-ai-status]');
                                if (statusEl) statusEl.textContent = t('pixel.generating', 'Enhancing...');
                                try {
                                    const dataURL = canvas.toDataURL('image/png');
                                    abortCtrl = new AbortController();
                                    const resp = await api('/api/pixel/enhance', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ source_data: dataURL, prompt, strength }),
                                        signal: abortCtrl.signal
                                    });
                                    if (resp && resp.url) {
                                        await loadImageToCanvas(resp.url);
                                        filePath = resp.path || '';
                                        fileName = filePath ? filePath.split('/').pop() : '';
                                    }
                                    if (statusEl) statusEl.textContent = '';
                                    notify({ type: 'success', message: t('pixel.enhanced', 'Image enhanced') });
                                } catch (err) {
                                    if (statusEl) statusEl.textContent = '';
                                    if (err.name !== 'AbortError') notify({ type: 'error', message: t('pixel.error_generate', 'Enhancement failed') });
                                } finally { abortCtrl = null; }
            }),
            checkAIConfig: Pixel.bindRuntime(runtime, async function checkAIConfig() {
                                try {
                                    const cfg = await api('/api/pixel/config');
                                    aiConfigured = cfg && cfg.enabled;
                                    const statusEl = host.querySelector('[data-ai-status]');
                                    if (!aiConfigured && statusEl) {
                                        statusEl.textContent = t('pixel.no_ai_config', 'Image generation is not configured');
                                    }
                                } catch (_) {}
            })
        });
    };
})();
