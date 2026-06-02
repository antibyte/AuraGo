    // Advanced File Operations & Favorites for File Manager

    function toggleFavorite(path) {
        if (!fm.favorites) fm.favorites = [];
        const idx = fm.favorites.indexOf(path);
        if (idx === -1) {
            fm.favorites.push(path);
        } else {
            fm.favorites.splice(idx, 1);
        }
        localStorage.setItem('aurago.fm.favorites', JSON.stringify(fm.favorites));
        renderAll();
    }

    let dragFavSourceIndex = null;
    
    function handleFavDragStart(e) {
        dragFavSourceIndex = parseInt(e.currentTarget.dataset.favIndex);
        e.dataTransfer.effectAllowed = 'move';
        e.stopPropagation();
    }
    
    function handleFavDragOver(e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        e.stopPropagation();
    }
    
    function handleFavDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        const targetIndex = parseInt(e.currentTarget.dataset.favIndex);
        if (dragFavSourceIndex !== null && dragFavSourceIndex !== targetIndex) {
            const moved = fm.favorites.splice(dragFavSourceIndex, 1)[0];
            fm.favorites.splice(targetIndex, 0, moved);
            localStorage.setItem('aurago.fm.favorites', JSON.stringify(fm.favorites));
            renderAll();
        }
        dragFavSourceIndex = null;
    }

    function handleFavoritesSectionDragOver(e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'copy';
        e.stopPropagation();
    }
    
    function handleFavoritesSectionDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        const payload = fileManagerDragPayloadFromEvent(e);
        if (payload) {
            let updated = false;
            payload.paths.forEach(p => {
                // If it's a directory in the files list, or we can check via type if it exists
                const file = fm.files.find(f => f.path === p);
                const isDir = file ? file.type === 'directory' : !p.includes('.'); // fallback check
                if (isDir && !fm.favorites.includes(p)) {
                    fm.favorites.push(p);
                    updated = true;
                }
            });
            if (updated) {
                localStorage.setItem('aurago.fm.favorites', JSON.stringify(fm.favorites));
                renderAll();
            }
        }
    }

    async function compressSelectionToZip() {
        const selected = getSelectedFiles();
        if (selected.length === 0) return;
        
        let defaultName = 'archive.zip';
        if (selected.length === 1) {
            const base = baseName(selected[0].path);
            defaultName = base.includes('.') ? base.slice(0, base.lastIndexOf('.')) + '.zip' : base + '.zip';
        }
        
        const zipName = await promptDialog(t('desktop.fm.compress_zip', 'Compress to ZIP'), defaultName);
        if (!zipName) return;
        
        const destPath = joinPath(fm.currentPath, zipName);
        
        try {
            showNotification({ type: 'info', message: t('desktop.fm.copy_progress', 'Copying files...') });
            await api('/api/desktop/archive', {
                method: 'POST',
                body: JSON.stringify({
                    paths: selected.map(f => f.path),
                    dest: destPath
                })
            });
            showNotification({ type: 'success', message: 'ZIP created successfully' });
            refresh();
        } catch (err) {
            showNotification({ type: 'error', message: err.message || String(err) });
        }
    }

    async function extractZip(file, extractHere = true) {
        let dest = fm.currentPath;
        if (!extractHere) {
            const destPrompt = await promptDialog(t('desktop.fm.extract_zip_to', 'Extract ZIP to...'), fm.currentPath);
            if (!destPrompt) return;
            dest = destPrompt;
        }
        
        try {
            await api('/api/desktop/extract', {
                method: 'POST',
                body: JSON.stringify({
                    path: file.path,
                    dest: dest
                })
            });
            showNotification({ type: 'success', message: 'ZIP extracted successfully' });
            refresh();
        } catch (err) {
            showNotification({ type: 'error', message: err.message || String(err) });
        }
    }

    function showBatchRenameDialog() {
        const selected = getSelectedFiles();
        if (selected.length === 0) return;
        
        return new Promise(resolve => {
            const overlay = document.createElement('div');
            overlay.className = 'fm-modal-overlay';
            
            function generatePreviews(prefix, suffix, findText, replaceText, numbering, startNum) {
                return selected.map((file, idx) => {
                    const origName = file.name;
                    const extIndex = origName.lastIndexOf('.');
                    const ext = extIndex !== -1 ? origName.slice(extIndex) : '';
                    let base = extIndex !== -1 ? origName.slice(0, extIndex) : origName;
                    
                    if (findText) {
                        try {
                            const regex = new RegExp(findText, 'g');
                            base = base.replace(regex, replaceText);
                        } catch (e) {
                            base = base.replaceAll(findText, replaceText);
                        }
                    }
                    
                    let numberStr = '';
                    if (numbering) {
                        const num = parseInt(startNum || '1') + idx;
                        numberStr = String(num).padStart(3, '0');
                    }
                    
                    const newName = `${prefix}${base}${suffix}${numberStr}${ext}`;
                    return { file, origName, newName };
                });
            }
            
            function updatePreviewTable() {
                const prefix = overlay.querySelector('[name="prefix"]').value;
                const suffix = overlay.querySelector('[name="suffix"]').value;
                const findText = overlay.querySelector('[name="find"]').value;
                const replaceText = overlay.querySelector('[name="replace"]').value;
                const numbering = overlay.querySelector('[name="numbering"]').checked;
                const startNum = overlay.querySelector('[name="startNum"]').value;
                
                const previews = generatePreviews(prefix, suffix, findText, replaceText, numbering, startNum);
                const tableBody = overlay.querySelector('.fm-batch-rename-table-body');
                if (tableBody) {
                    tableBody.innerHTML = previews.map(p => `
                        <div class="fm-batch-rename-row" style="display:grid;grid-template-columns:1fr 1fr;gap:8px;padding:4px 6px;border-bottom:1px solid rgba(255,255,255,0.03)">
                            <div class="text-ellipsis" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:0.75rem;color:var(--ds-color-fg-muted)" title="${esc(p.origName)}">${esc(p.origName)}</div>
                            <div class="text-ellipsis" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:0.75rem;color:var(--ds-color-accent-500)" title="${esc(p.newName)}">${esc(p.newName)}</div>
                        </div>
                    `).join('');
                }
            }
            
            overlay.innerHTML = `<div class="fm-modal fm-batch-rename-modal" style="max-width: 650px; width: 90%">
                <div class="fm-modal-title">${esc(t('desktop.fm.batch_rename_title', 'Batch Rename Files'))}</div>
                <div class="fm-batch-rename-grid" style="display:grid;grid-template-columns: 240px 1fr; gap:16px; margin: 12px 0;">
                    <div class="fm-batch-rename-fields" style="display:flex;flex-direction:column;gap:8px">
                        <div class="fm-field-group" style="display:flex;flex-direction:column;gap:2px">
                            <label style="font-size:0.7rem;color:var(--ds-color-fg-muted)">${esc(t('desktop.fm.batch_rename_prefix', 'Prefix'))}</label>
                            <input type="text" name="prefix" value="" autocomplete="off" spellcheck="false" style="width:100%;box-sizing:border-box">
                        </div>
                        <div class="fm-field-group" style="display:flex;flex-direction:column;gap:2px">
                            <label style="font-size:0.7rem;color:var(--ds-color-fg-muted)">${esc(t('desktop.fm.batch_rename_suffix', 'Suffix'))}</label>
                            <input type="text" name="suffix" value="" autocomplete="off" spellcheck="false" style="width:100%;box-sizing:border-box">
                        </div>
                        <div class="fm-field-group" style="display:flex;flex-direction:column;gap:2px">
                            <label style="font-size:0.7rem;color:var(--ds-color-fg-muted)">${esc(t('desktop.fm.batch_rename_find', 'Find'))}</label>
                            <input type="text" name="find" value="" autocomplete="off" spellcheck="false" style="width:100%;box-sizing:border-box">
                        </div>
                        <div class="fm-field-group" style="display:flex;flex-direction:column;gap:2px">
                            <label style="font-size:0.7rem;color:var(--ds-color-fg-muted)">${esc(t('desktop.fm.batch_rename_replace', 'Replace'))}</label>
                            <input type="text" name="replace" value="" autocomplete="off" spellcheck="false" style="width:100%;box-sizing:border-box">
                        </div>
                        <div class="fm-field-group fm-checkbox-group" style="display:flex;align-items:center;gap:6px;margin:4px 0">
                            <input type="checkbox" name="numbering" id="fm-rename-num">
                            <label for="fm-rename-num" style="font-size:0.75rem;user-select:none">${esc(t('desktop.fm.batch_rename_numbering', 'Numbering (001...)'))}</label>
                        </div>
                        <div class="fm-field-group" style="display:none;flex-direction:column;gap:2px" id="fm-rename-start-group">
                            <label style="font-size:0.7rem;color:var(--ds-color-fg-muted)">Start Number</label>
                            <input type="number" name="startNum" value="1" min="1" step="1" style="width:100%;box-sizing:border-box">
                        </div>
                    </div>
                    <div class="fm-batch-rename-preview-panel" style="border:1px solid var(--ds-color-border-subtle);background:rgba(0,0,0,0.15);border-radius:6px;display:flex;flex-direction:column;height:240px;overflow:hidden">
                        <div class="fm-batch-rename-table-header" style="display:grid;grid-template-columns:1fr 1fr;gap:8px;padding:6px;background:rgba(255,255,255,0.02);border-bottom:1px solid var(--ds-color-border-subtle);font-size:0.75rem;color:var(--ds-color-fg-muted);font-weight:600">
                            <div>Original</div>
                            <div>New Name</div>
                        </div>
                        <div class="fm-batch-rename-table-body" style="flex:1;overflow-y:auto;padding:2px 0"></div>
                    </div>
                </div>
                <div class="fm-modal-actions">
                    <button type="button" class="fm-btn" data-cancel>${esc(t('desktop.cancel', 'Cancel'))}</button>
                    <button type="button" class="fm-btn primary" data-rename>${esc(t('desktop.fm.batch_rename', 'Rename'))}</button>
                </div>
            </div>`;
            
            document.body.appendChild(overlay);
            
            const inputs = overlay.querySelectorAll('input');
            inputs.forEach(input => {
                input.addEventListener('input', updatePreviewTable);
            });
            
            const numCheck = overlay.querySelector('[name="numbering"]');
            numCheck.addEventListener('change', e => {
                const startGroup = overlay.querySelector('#fm-rename-start-group');
                startGroup.style.display = e.target.checked ? 'flex' : 'none';
                updatePreviewTable();
            });
            
            updatePreviewTable();
            
            const cleanup = result => { overlay.remove(); resolve(result); };
            overlay.querySelector('[data-cancel]').addEventListener('click', () => cleanup(null));
            overlay.querySelector('[data-rename]').addEventListener('click', () => {
                const prefix = overlay.querySelector('[name="prefix"]').value;
                const suffix = overlay.querySelector('[name="suffix"]').value;
                const findText = overlay.querySelector('[name="find"]').value;
                const replaceText = overlay.querySelector('[name="replace"]').value;
                const numbering = numCheck.checked;
                const startNum = overlay.querySelector('[name="startNum"]').value;
                
                const previews = generatePreviews(prefix, suffix, findText, replaceText, numbering, startNum);
                const payload = previews.map(p => ({
                    old_path: p.file.path,
                    new_name: p.newName
                }));
                cleanup(payload);
            });
            
            overlay.addEventListener('click', e => { if (e.target === overlay) cleanup(null); });
        });
    }

    async function executeBatchRename() {
        if (isReadonly()) return;
        const payload = await showBatchRenameDialog();
        if (!payload || !payload.length) return;
        
        try {
            await api('/api/desktop/batch-rename', {
                method: 'POST',
                body: JSON.stringify({ operations: payload })
            });
            showNotification({ type: 'success', message: 'Files renamed successfully' });
            refresh();
        } catch (err) {
            showNotification({ type: 'error', message: err.message || String(err) });
        }
    }

    async function createNewFileWithTemplate() {
        if (isReadonly()) return;
        
        return new Promise(resolve => {
            const overlay = document.createElement('div');
            overlay.className = 'fm-modal-overlay';
            
            const templates = [
                { ext: 'txt', label: 'Plain Text', content: '' },
                { ext: 'md', label: 'Markdown', content: '# Document Title\n\n' },
                { ext: 'py', label: 'Python Script', content: '#!/usr/bin/env python3\n\ndef main():\n    print("Hello, World!")\n\nif __name__ == "__main__":\n    main()\n' },
                { ext: 'go', label: 'Go Source', content: 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("Hello, World!")\n}\n' },
                { ext: 'js', label: 'JavaScript', content: '// JavaScript Document\nconsole.log("Hello, World!");\n' },
                { ext: 'json', label: 'JSON Config', content: '{\n  "key": "value"\n}\n' },
                { ext: 'yaml', label: 'YAML Config', content: 'key: value\n' },
                { ext: 'sh', label: 'Shell Script', content: '#!/bin/bash\necho "Hello, World!"\n' },
                { ext: 'html', label: 'HTML Page', content: '<!DOCTYPE html>\n<html>\n<head>\n    <title>Page Title</title>\n</head>\n<body>\n    <h1>Hello, World!</h1>\n</body>\n</html>\n' },
                { ext: 'css', label: 'CSS Stylesheet', content: 'body {\n    background-color: #f0f0f0;\n}\n' }
            ];
            
            const options = templates.map(t => `<option value="${t.ext}">${t.label} (.${t.ext})</option>`).join('');
            
            overlay.innerHTML = `<form class="fm-modal">
                <div class="fm-modal-title">${esc(t('desktop.fm.new_file_template', 'Create File from Template'))}</div>
                <div class="fm-field-group" style="margin-bottom: 12px; display:flex; flex-direction:column; gap:4px">
                    <label style="font-size:0.75rem;color:var(--ds-color-fg-muted)">${esc(t('desktop.fm.new_file_prompt', 'File name'))}</label>
                    <input type="text" name="filename" value="new-file.txt" autocomplete="off" spellcheck="false" style="width:100%;box-sizing:border-box">
                </div>
                <div class="fm-field-group" style="margin-bottom: 16px; display:flex; flex-direction:column; gap:4px">
                    <label style="font-size:0.75rem;color:var(--ds-color-fg-muted)">Template</label>
                    <select name="template" style="width:100%;box-sizing:border-box;background:#1a1a1a;color:var(--ds-color-fg-primary);border:1px solid var(--ds-color-border-subtle);padding:6px;border-radius:4px">${options}</select>
                </div>
                <div class="fm-modal-actions">
                    <button type="button" class="fm-btn" data-cancel>${esc(t('desktop.cancel', 'Cancel'))}</button>
                    <button type="submit" class="fm-btn primary">${esc(t('desktop.ok', 'OK'))}</button>
                </div>
            </form>`;
            
            document.body.appendChild(overlay);
            const input = overlay.querySelector('[name="filename"]');
            const select = overlay.querySelector('[name="template"]');
            
            select.addEventListener('change', e => {
                const ext = e.target.value;
                const currentName = input.value;
                const dotIdx = currentName.lastIndexOf('.');
                const base = dotIdx !== -1 ? currentName.slice(0, dotIdx) : currentName;
                input.value = base + '.' + ext;
            });
            
            const cleanup = result => { overlay.remove(); resolve(result); };
            overlay.querySelector('form').addEventListener('submit', e => {
                e.preventDefault();
                const name = input.value.trim();
                const ext = select.value;
                const tmpl = templates.find(t => t.ext === ext);
                cleanup(name ? { name, content: tmpl ? tmpl.content : '' } : null);
            });
            overlay.querySelector('[data-cancel]').addEventListener('click', () => cleanup(null));
            overlay.addEventListener('click', e => { if (e.target === overlay) cleanup(null); });
            
            input.focus();
            input.select();
        });
    }
