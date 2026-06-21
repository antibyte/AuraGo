    // Preview Panel & Column Resize module

    function isTextFile(name) {
        const ext = String(name || '').split('.').pop().toLowerCase();
        const textExts = new Set(['txt', 'log', 'md', 'json', 'yaml', 'yml', 'sh', 'py', 'go', 'js', 'css', 'html', 'xml', 'ini', 'conf']);
        return textExts.has(ext);
    }

    async function loadTextExcerpt(path) {
        try {
            const res = await fetch('/api/desktop/file-content?path=' + encodeURIComponent(path));
            if (!res.ok) throw new Error();
            const text = await res.text();
            const excerpt = text.length > 500 ? text.slice(0, 500) + '...' : text;
            
            const container = fm.host ? fm.host.querySelector(`.fm-preview-text-wrap[data-preview-path="${path.replace(/"/g, '\\"')}"]`) : null;
            if (container) {
                container.innerHTML = `<pre class="fm-preview-text-pre">${esc(excerpt)}</pre>`;
            }
        } catch (err) {
            const container = fm.host ? fm.host.querySelector(`.fm-preview-text-wrap[data-preview-path="${path.replace(/"/g, '\\"')}"]`) : null;
            if (container) {
                container.innerHTML = `<div class="fm-preview-text-error">Cannot display preview</div>`;
            }
        }
    }

    function renderPreviewPanelHtml() {
        const selected = getSelectedFiles();
        
        let header = '';
        let body = '';
        
        if (selected.length === 0) {
            header = `<div class="fm-preview-header"><h3>${esc(t('desktop.fm.preview_properties'))}</h3></div>`;
            body = `<div class="fm-preview-empty">${esc(t('desktop.fm.no_file_selected'))}</div>`;
        } else if (selected.length > 1) {
            header = `<div class="fm-preview-header"><h3>${esc(t('desktop.fm.preview_properties'))}</h3></div>`;
            const size = getSelectedSize();
            body = `<div class="fm-preview-multi">
                <div class="fm-preview-icon-large">${iconMarkup('copy', '📋', '', 48)}</div>
                <div class="fm-preview-name">${esc(t('desktop.fm.selected', { count: selected.length }))}</div>
                <div class="fm-preview-info-row">
                    <span class="fm-preview-info-label">${esc(t('desktop.fm.prop_size'))}:</span>
                    <span class="fm-preview-info-val">${esc(fmtBytes(size))}</span>
                </div>
            </div>`;
        } else {
            const file = selected[0];
            header = `<div class="fm-preview-header">
                <h3>${esc(t('desktop.fm.preview_properties'))}</h3>
                <button type="button" class="fm-preview-close" data-action="toggle-preview" title="${esc(t('desktop.close'))}">&times;</button>
            </div>`;
            
            const isImg = isPreviewableImage(file);
            const isMedia = isMediaFile(file.name);
            const isText = isTextFile(file.name);
            
            let previewContent = '';
            if (isImg) {
                previewContent = `<div class="fm-preview-media-wrap">
                    <img src="${previewURL(file)}" alt="${esc(file.name)}" class="fm-preview-img-preview" />
                </div>`;
            } else if (isMedia) {
                const mime = file.mime || '';
                const isAudio = mime.startsWith('audio/') || String(file.name).endsWith('.mp3') || String(file.name).endsWith('.wav') || String(file.name).endsWith('.ogg');
                if (isAudio) {
                    previewContent = `<div class="fm-preview-media-wrap">
                        <audio src="${previewURL(file)}" controls class="fm-preview-audio-preview"></audio>
                    </div>`;
                } else {
                    previewContent = `<div class="fm-preview-media-wrap">
                        <video src="${previewURL(file)}" controls class="fm-preview-video-preview"></video>
                    </div>`;
                }
            } else if (isText) {
                previewContent = `<div class="fm-preview-text-wrap" data-preview-path="${esc(file.path)}">
                    <div class="fm-preview-text-loading">${esc(t('desktop.fm.loading'))}</div>
                </div>`;
                loadTextExcerpt(file.path);
            } else {
                previewContent = `<div class="fm-preview-icon-large">${thumbnailMarkup(file, iconForFile(file), '', 'grid')}</div>`;
            }
            
            body = `<div class="fm-preview-details">
                ${previewContent}
                <div class="fm-preview-name-title" title="${esc(file.name)}">${esc(file.name)}</div>
                <div class="fm-preview-info-row">
                    <span class="fm-preview-info-label">${esc(t('desktop.fm.prop_type'))}:</span>
                    <span class="fm-preview-info-val">${esc(file.type === 'directory' ? t('desktop.fm.prop_folder') : t('desktop.fm.prop_file'))}</span>
                </div>
                <div class="fm-preview-info-row">
                    <span class="fm-preview-info-label">${esc(t('desktop.fm.prop_size'))}:</span>
                    <span class="fm-preview-info-val">${esc(file.type === 'directory' ? '' : fmtBytes(file.size || 0))}</span>
                </div>
                <div class="fm-preview-info-row">
                    <span class="fm-preview-info-label">${esc(t('desktop.fm.prop_modified'))}:</span>
                    <span class="fm-preview-info-val">${esc(formatDate(file.modified))}</span>
                </div>
                ${file.type === 'directory' ? `
                <div class="fm-preview-info-row">
                    <span class="fm-preview-info-label">${esc(t('desktop.fm.folder_size'))}:</span>
                    <span class="fm-preview-info-val" data-preview-folder-size="${esc(file.path)}">
                        <button type="button" class="fm-preview-calc-size-btn" data-action="calc-folder-size" data-path="${esc(file.path)}">${esc(t('desktop.fm.calculate_folder_size'))}</button>
                    </span>
                </div>` : ''}
            </div>`;
        }
        
        const width = fm.previewWidth || 250;
        
        return `<div class="fm-preview-panel" style="width: ${width}px" role="complementary">
            <div class="fm-preview-resize" data-resize-handle="preview"></div>
            <div class="fm-preview-container">
                ${header}
                <div class="fm-preview-body">${body}</div>
            </div>
        </div>`;
    }

    function initPreviewResize(root) {
        const handle = root.querySelector('[data-resize-handle="preview"]');
        if (!handle) return;
        
        let startX = 0;
        let startWidth = 0;
        
        function onPointerMove(e) {
            const dx = startX - e.clientX;
            const newWidth = Math.max(200, Math.min(500, startWidth + dx));
            fm.previewWidth = newWidth;
            const panel = root.querySelector('.fm-preview-panel');
            if (panel) panel.style.width = newWidth + 'px';
        }
        
        function onPointerUp() {
            document.removeEventListener('pointermove', onPointerMove);
            document.removeEventListener('pointerup', onPointerUp);
            localStorage.setItem('aurago.fm.previewWidth', fm.previewWidth);
        }
        
        handle.addEventListener('pointerdown', e => {
            e.preventDefault();
            startX = e.clientX;
            const panel = root.querySelector('.fm-preview-panel');
            startWidth = panel ? panel.offsetWidth : (fm.previewWidth || 250);
            document.addEventListener('pointermove', onPointerMove);
            document.addEventListener('pointerup', onPointerUp);
        });
    }

    function initColumnResize(root) {
        const header = root.querySelector('.fm-list-header');
        if (!header) return;
        
        const cols = ['name', 'size', 'date', 'type'];
        cols.forEach(col => {
            const val = localStorage.getItem(`aurago.fm.col.${col}`);
            if (val) {
                root.style.setProperty(`--fm-col-width-${col}`, val);
            }
        });
        
        const handles = root.querySelectorAll('.fm-col-resize-handle');
        handles.forEach(handle => {
            handle.addEventListener('pointerdown', e => {
                e.preventDefault();
                e.stopPropagation();
                
                const colKey = handle.dataset.colKey;
                const cell = handle.closest('.fm-list-cell');
                const startX = e.clientX;
                const startWidth = cell.offsetWidth;
                
                function onPointerMove(ev) {
                    const dx = ev.clientX - startX;
                    const newWidth = Math.max(50, startWidth + dx) + 'px';
                    root.style.setProperty(`--fm-col-width-${colKey}`, newWidth);
                }
                
                function onPointerUp() {
                    document.removeEventListener('pointermove', onPointerMove);
                    document.removeEventListener('pointerup', onPointerUp);
                    const finalWidth = root.style.getPropertyValue(`--fm-col-width-${colKey}`);
                    localStorage.setItem(`aurago.fm.col.${colKey}`, finalWidth);
                }
                
                document.addEventListener('pointermove', onPointerMove);
                document.addEventListener('pointerup', onPointerUp);
            });
        });
    }
