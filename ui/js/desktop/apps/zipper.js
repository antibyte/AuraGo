(function () {
    'use strict';

    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        const state = { container: host };
        instances.set(windowId, state);
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const t = ctx.t || ((key, fallback) => fallback || key);
        const api = ctx.api || fetchJSON;
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span>${esc(fallback || key || '')}</span>`);
        const notify = ctx.notify || (() => {});
        const openApp = ctx.openApp || (() => {});
        const fileOps = ctx.fileOps || window.AuraDesktopFileOps || null;
        const openFileDialog = ctx.openFileDialog || null;
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        let zipPath = ctx.path || '';
        let entries = [];
        let filteredEntries = [];
        let currentDir = '';
        let sortCol = 'name';
        let sortAsc = true;
        let selected = new Set();

        host.innerHTML = `<div class="zipper-app">
            <div class="vd-toolbar zipper-toolbar">
                <button class="vd-tool-button vd-tool-button-icon" type="button" data-action="open" title="${esc(t('zipper.open'))}">${iconMarkup('folder-open', 'Open', 'vd-tool-icon', 15)}</button>
                <button class="vd-tool-button vd-tool-button-icon" type="button" data-action="extract-here" title="${esc(t('zipper.extract_here'))}">${iconMarkup('download', 'Extract', 'vd-tool-icon', 15)}</button>
                <button class="vd-tool-button vd-tool-button-icon" type="button" data-action="extract-to" title="${esc(t('zipper.extract_to'))}">${iconMarkup('folder', 'Extract To', 'vd-tool-icon', 15)}</button>
                <button class="vd-tool-button vd-tool-button-icon" type="button" data-action="new-archive" title="${esc(t('zipper.new_archive'))}">${iconMarkup('archive', 'New', 'vd-tool-icon', 15)}</button>
                <span class="zipper-path vd-path">${esc(zipPath || t('zipper.no_archive'))}</span>
            </div>
            <div class="zipper-breadcrumb" data-breadcrumb></div>
            <div class="zipper-list" data-list>
                <table class="zipper-table">
                    <thead>
                        <tr>
                            <th class="zipper-col-check"><input type="checkbox" data-select-all></th>
                            <th class="zipper-col-name" data-sort="name">${esc(t('zipper.name'))}</th>
                            <th class="zipper-col-size" data-sort="size">${esc(t('zipper.size'))}</th>
                            <th class="zipper-col-compressed" data-sort="compressed">${esc(t('zipper.compressed'))}</th>
                            <th class="zipper-col-modified" data-sort="modified">${esc(t('zipper.modified'))}</th>
                        </tr>
                    </thead>
                    <tbody data-tbody></tbody>
                </table>
            </div>
            <div class="zipper-status" data-status></div>
            <div class="zipper-preview" data-preview hidden>
                <div class="zipper-preview-bar">
                    <span class="zipper-preview-name" data-preview-name></span>
                    <button class="vd-tool-button" type="button" data-action="close-preview">${esc(t('zipper.close_preview'))}</button>
                </div>
                <div class="zipper-preview-body" data-preview-body></div>
            </div>
        </div>`;

        const listHost = host.querySelector('[data-list]');
        const tbody = host.querySelector('[data-tbody]');
        const statusNode = host.querySelector('[data-status]');
        const breadcrumbNode = host.querySelector('[data-breadcrumb]');
        const selectAllCheckbox = host.querySelector('[data-select-all]');
        const previewNode = host.querySelector('[data-preview]');
        const previewNameNode = host.querySelector('[data-preview-name]');
        const previewBodyNode = host.querySelector('[data-preview-body]');

        function fmtBytes(n) {
            n = Number(n || 0);
            if (n < 1024) return t('desktop.bytes').replace('{{count}}', n);
            if (n < 1024 * 1024) return t('desktop.kib').replace('{{count}}', (n / 1024).toFixed(1));
            if (n < 1024 * 1024 * 1024) return t('desktop.mib').replace('{{count}}', (n / (1024 * 1024)).toFixed(1));
            if (n < 1024 * 1024 * 1024 * 1024) return t('desktop.gib').replace('{{count}}', (n / (1024 * 1024 * 1024)).toFixed(1));
            return t('desktop.tib').replace('{{count}}', (n / (1024 * 1024 * 1024 * 1024)).toFixed(1));
        }

        function setStatus(msg) {
            if (statusNode) statusNode.textContent = msg || '';
        }

        function normalizePath(path) {
            return String(path || '').replace(/\\/g, '/').replace(/^\/+/, '').replace(/\/+/g, '/').replace(/\/+$/, '');
        }

        function joinPath(base, name) {
            const left = normalizePath(base);
            const right = normalizePath(name);
            return left ? (right ? left + '/' + right : left) : right;
        }

        function baseName(path) {
            const parts = normalizePath(path).split('/').filter(Boolean);
            return parts.pop() || '';
        }

        function dirName(path) {
            const parts = normalizePath(path).split('/').filter(Boolean);
            parts.pop();
            return parts.join('/');
        }

        function ensureZipExtension(path) {
            const value = normalizePath(path);
            return /\.zip$/i.test(value) ? value : value + '.zip';
        }

        function defaultArchiveName(paths) {
            if (paths.length === 1) {
                const name = baseName(paths[0]) || 'archive';
                const dot = name.lastIndexOf('.');
                const stem = dot > 0 ? name.slice(0, dot) : name;
                return stem + '.zip';
            }
            return 'archive.zip';
        }

        function updateBreadcrumb() {
            if (!breadcrumbNode) return;
            const parts = currentDir ? currentDir.split('/').filter(Boolean) : [];
            let html = `<button class="zipper-crumb" type="button" data-dir="">${esc(t('zipper.title'))}</button>`;
            let acc = '';
            for (const p of parts) {
                acc += (acc ? '/' : '') + p;
                html += ` <span class="zipper-crumb-sep">/</span> <button class="zipper-crumb" type="button" data-dir="${esc(acc)}">${esc(p)}</button>`;
            }
            breadcrumbNode.innerHTML = html;
            breadcrumbNode.querySelectorAll('[data-dir]').forEach(btn => {
                btn.addEventListener('click', () => { currentDir = btn.dataset.dir || ''; selected.clear(); applyFilter(); });
            });
        }

        function applyFilter() {
            const prefix = currentDir ? currentDir + '/' : '';
            filteredEntries = entries.filter(e => {
                if (currentDir && !e.name.startsWith(prefix)) return false;
                if (currentDir) {
                    const rest = e.name.slice(prefix.length);
                    if (!e.is_dir && rest.includes('/')) return false;
                    if (e.is_dir && rest.replace(/\/$/, '').includes('/')) return false;
                } else {
                    if (!e.is_dir && e.name.includes('/')) return false;
                    if (e.is_dir && e.name.replace(/\/$/, '').includes('/')) return false;
                }
                return true;
            });
            sortEntries();
            updateBreadcrumb();
            renderTable();
            updateStatus();
        }

        function sortEntries() {
            filteredEntries.sort((a, b) => {
                if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
                let va, vb;
                if (sortCol === 'size') { va = a.size; vb = b.size; }
                else if (sortCol === 'compressed') { va = a.compressed_size; vb = b.compressed_size; }
                else if (sortCol === 'modified') { va = a.mod_time; vb = b.mod_time; }
                else { va = a.name.toLowerCase(); vb = b.name.toLowerCase(); }
                if (va < vb) return sortAsc ? -1 : 1;
                if (va > vb) return sortAsc ? 1 : -1;
                return 0;
            });
        }

        function fileExtension(name) {
            const value = String(name || '').split('/').pop() || '';
            const dot = value.lastIndexOf('.');
            return dot > 0 ? value.slice(dot + 1).toLowerCase() : '';
        }

        function archiveEntryURL(entryName) {
            return '/api/desktop/archive/entry?path=' + encodeURIComponent(zipPath) + '&entry=' + encodeURIComponent(entryName);
        }

        function previewKind(name) {
            const ext = fileExtension(name);
            if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'tif', 'tiff', 'avif'].includes(ext)) return 'image';
            if (['mp3', 'm4a', 'ogg', 'opus', 'wav', 'flac'].includes(ext)) return 'audio';
            if (['mp4', 'webm', 'mkv', 'mov'].includes(ext)) return 'video';
            if (['pdf', 'md', 'docx', 'xlsx', 'xlsm', 'csv'].includes(ext)) return 'document';
            if (ext === 'stl') return 'model';
            if (['txt', 'log', 'json', 'xml', 'yaml', 'yml', 'toml', 'ini', 'cfg', 'conf', 'html', 'htm', 'css', 'js', 'mjs', 'ts', 'tsx', 'jsx', 'go', 'py', 'sh', 'ps1', 'sql'].includes(ext)) return 'text';
            return '';
        }

        function entryIconKey(entry) {
            if (entry.is_dir) return 'folder';
            const kind = previewKind(entry.name);
            if (kind === 'image') return 'image';
            if (kind === 'audio') return 'audio';
            if (kind === 'video') return 'video';
            if (kind === 'model') return 'theme-threedee';
            const ext = fileExtension(entry.name);
            if (ext === 'pdf') return 'pdf';
            if (ext === 'md') return 'markdown';
            if (ext === 'docx') return 'documents';
            if (['xlsx', 'xlsm', 'csv'].includes(ext)) return 'spreadsheet';
            if (kind === 'text') return 'text';
            return 'file';
        }

        function closePreview() {
            if (!previewNode) return;
            previewNode.hidden = true;
            if (previewBodyNode) previewBodyNode.innerHTML = '';
            if (previewNameNode) previewNameNode.textContent = '';
        }

        async function showInlinePreview(entry, kind) {
            if (!previewNode || !previewBodyNode) return;
            previewNode.hidden = false;
            if (previewNameNode) previewNameNode.textContent = displayName(entry) || entry.name;
            previewBodyNode.innerHTML = `<div class="zipper-preview-loading">${esc(t('zipper.previewing'))}</div>`;
            const src = archiveEntryURL(entry.name);
            try {
                if (kind === 'image') {
                    previewBodyNode.innerHTML = `<img class="zipper-preview-image" src="${esc(src)}" alt="${esc(displayName(entry) || entry.name)}">`;
                    const image = previewBodyNode.querySelector('img');
                    if (image) {
                        image.addEventListener('error', () => {
                            previewBodyNode.innerHTML = `<div class="zipper-preview-error">${esc(t('zipper.error_preview'))}</div>`;
                        });
                    }
                    return;
                }
                if (kind === 'audio') {
                    previewBodyNode.innerHTML = `<audio class="zipper-preview-media" controls src="${esc(src)}"></audio>`;
                    return;
                }
                if (kind === 'video') {
                    previewBodyNode.innerHTML = `<video class="zipper-preview-media" controls src="${esc(src)}"></video>`;
                    return;
                }
                const resp = await fetch(src);
                if (!resp.ok) {
                    const body = await resp.json().catch(() => ({}));
                    throw new Error(body.error || body.message || ('HTTP ' + resp.status));
                }
                const text = await resp.text();
                previewBodyNode.innerHTML = `<pre class="zipper-preview-text">${esc(text)}</pre>`;
            } catch (err) {
                previewBodyNode.innerHTML = `<div class="zipper-preview-error">${esc(t('zipper.error_preview'))}: ${esc(err.message || String(err))}</div>`;
                notify({ type: 'error', message: err.message || String(err) });
            }
        }

        function openArchiveMember(entry) {
            if (!entry) return;
            if (entry.is_dir) {
                currentDir = entry.name.replace(/\/$/, '');
                selected.clear();
                closePreview();
                applyFilter();
                return;
            }
            if (!zipPath) {
                notify({ type: 'error', message: t('zipper.no_archive') });
                return;
            }
            const kind = previewKind(entry.name);
            if (!kind) {
                notify({ type: 'info', message: t('zipper.preview_unsupported') });
                return;
            }
            setStatus(t('zipper.previewing'));
            if (kind === 'document') {
                closePreview();
                openApp('viewer', { path: zipPath, archiveEntry: entry.name, forceNew: true });
                return;
            }
            if (kind === 'model') {
                closePreview();
                openApp('viewer-3d', { path: zipPath, archiveEntry: entry.name, forceNew: true });
                return;
            }
            showInlinePreview(entry, kind);
        }

        function openSelectedMember() {
            const chosen = filteredEntries.find(e => selected.has(e.name));
            if (chosen) openArchiveMember(chosen);
        }

        function displayName(entry) {
            const prefix = currentDir ? currentDir + '/' : '';
            let name = entry.name.startsWith(prefix) ? entry.name.slice(prefix.length) : entry.name;
            if (entry.is_dir) name = name.replace(/\/$/, '');
            return name;
        }

        function renderTable() {
            if (!tbody) return;
            if (filteredEntries.length === 0) {
                tbody.innerHTML = `<tr><td colspan="5" class="zipper-empty">${esc(t('zipper.empty_archive'))}</td></tr>`;
                return;
            }
            tbody.innerHTML = filteredEntries.map((e, i) => {
                const name = displayName(e);
                const checked = selected.has(e.name) ? 'checked' : '';
                const icon = iconMarkup(entryIconKey(e), e.is_dir ? 'Dir' : 'File');
                return `<tr data-idx="${i}" class="${selected.has(e.name) ? 'zipper-selected' : ''}">
                    <td class="zipper-col-check"><input type="checkbox" data-check="${esc(e.name)}" ${checked}></td>
                    <td class="zipper-col-name"><span class="zipper-icon">${icon}</span> ${esc(name)}</td>
                    <td class="zipper-col-size">${e.is_dir ? '—' : fmtBytes(e.size)}</td>
                    <td class="zipper-col-compressed">${e.is_dir ? '—' : fmtBytes(e.compressed_size)}</td>
                    <td class="zipper-col-modified">${e.mod_time ? new Date(e.mod_time).toLocaleDateString() : ''}</td>
                </tr>`;
            }).join('');

            tbody.querySelectorAll('[data-check]').forEach(cb => {
                cb.addEventListener('change', () => {
                    const name = cb.dataset.check;
                    if (cb.checked) selected.add(name); else selected.delete(name);
                    renderTable();
                    updateStatus();
                });
            });

            tbody.querySelectorAll('tr[data-idx]').forEach(row => {
                row.addEventListener('dblclick', () => {
                    const idx = Number(row.dataset.idx);
                    const entry = filteredEntries[idx];
                    if (!entry) return;
                    openArchiveMember(entry);
                });
                row.addEventListener('click', (ev) => {
                    if (ev.target.tagName === 'INPUT') return;
                    const idx = Number(row.dataset.idx);
                    const entry = filteredEntries[idx];
                    if (!entry) return;
                    if (ev.ctrlKey || ev.metaKey) {
                        if (selected.has(entry.name)) selected.delete(entry.name); else selected.add(entry.name);
                    } else {
                        selected.clear();
                        selected.add(entry.name);
                    }
                    renderTable();
                    updateStatus();
                });
            });
        }

        function updateStatus() {
            const totalSize = entries.filter(e => !e.is_dir).reduce((s, e) => s + e.size, 0);
            const totalCompressed = entries.filter(e => !e.is_dir).reduce((s, e) => s + e.compressed_size, 0);
            const count = entries.filter(e => !e.is_dir).length;
            const msg = [
                t('zipper.items').replace('{{count}}', count),
                t('zipper.total_size').replace('{{size}}', fmtBytes(totalSize)),
                t('zipper.compressed_size').replace('{{size}}', fmtBytes(totalCompressed))
            ];
            if (selected.size > 0) {
                msg.unshift(t('zipper.selected').replace('{{count}}', selected.size));
            }
            setStatus(msg.join('  ·  '));
        }

        async function openFile() {
            if (!openFileDialog) return;
            const result = await openFileDialog({ filters: [{ name: 'ZIP Archives', extensions: ['zip'] }] });
            if (result && !result.canceled && result.path) {
                openZipPath(result.path);
            }
        }

        function openZipPath(newPath) {
            zipPath = newPath;
            currentDir = '';
            selected.clear();
            const pathSpan = host.querySelector('.zipper-path');
            if (pathSpan) pathSpan.textContent = zipPath;
            load();
        }

        async function load() {
            if (!zipPath) { setStatus(t('zipper.no_archive')); return; }
            setStatus(t('zipper.extracting'));
            try {
                const body = await api('/api/desktop/archive/list?path=' + encodeURIComponent(zipPath));
                entries = (body.entries || []).map(e => ({
                    name: e.name,
                    size: e.size || 0,
                    compressed_size: e.compressed_size || 0,
                    is_dir: !!e.is_dir,
                    mod_time: e.mod_time || ''
                }));
                const dirs = new Set();
                for (const e of entries) {
                    const parts = e.name.split('/').filter(Boolean);
                    parts.pop();
                    let acc = '';
                    for (const p of parts) {
                        acc += (acc ? '/' : '') + p;
                        dirs.add(acc);
                    }
                }
                for (const d of dirs) {
                    if (!entries.some(e => e.name === d + '/' || e.name === d) && !entries.some(e => e.name === d && e.is_dir)) {
                        entries.push({ name: d + '/', size: 0, compressed_size: 0, is_dir: true, mod_time: '' });
                    }
                }
                currentDir = '';
                selected.clear();
                applyFilter();
            } catch (err) {
                setStatus(t('zipper.error_list'));
                notify({ type: 'error', message: err.message || String(err) });
            }
        }

        async function uploadExternalFilesForArchive(files) {
            const uploadDir = 'Downloads';
            const uploadedPaths = [];
            for (const file of files) {
                const form = new FormData();
                form.append('path', uploadDir);
                form.append('file', file);
                const body = await api('/api/desktop/upload', { method: 'POST', body: form });
                uploadedPaths.push(body.path || joinPath(uploadDir, file.name || 'file'));
            }
            if (typeof ctx.loadBootstrap === 'function') await ctx.loadBootstrap();
            return uploadedPaths;
        }

        async function createArchiveFromHostFiles(files) {
            const externalFiles = Array.from(files || []).filter(Boolean);
            if (!externalFiles.length) return false;
            try {
                const paths = await uploadExternalFilesForArchive(externalFiles);
                return await createArchiveFromPaths(paths);
            } catch (err) {
                setStatus(t('zipper.error_create'));
                notify({ type: 'error', message: err.message || String(err) });
                return false;
            }
        }

        async function createArchiveFromPaths(paths) {
            const cleanPaths = [...new Set((paths || []).map(normalizePath).filter(Boolean))];
            if (!cleanPaths.length) return false;
            if (cleanPaths.length === 1 && /\.zip$/i.test(cleanPaths[0])) {
                openZipPath(cleanPaths[0]);
                return true;
            }
            const prompt = ctx.promptDialog || (async () => null);
            const defaultDir = dirName(cleanPaths[0]) || 'Documents';
            let dest = await prompt(t('zipper.new_archive'), joinPath(defaultDir, defaultArchiveName(cleanPaths)));
            if (!dest) return false;
            dest = ensureZipExtension(dest);
            setStatus(t('zipper.creating'));
            try {
                await api('/api/desktop/archive', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ paths: cleanPaths, dest: dest })
                });
                setStatus(t('zipper.created'));
                notify({ type: 'success', message: t('zipper.created') });
                if (typeof ctx.loadBootstrap === 'function') await ctx.loadBootstrap();
                openZipPath(dest);
                return true;
            } catch (err) {
                setStatus(t('zipper.error_create'));
                notify({ type: 'error', message: err.message || String(err) });
                return false;
            }
        }

        async function extractHere() {
            if (!zipPath) return;
            const dest = zipPath.split('/').slice(0, -1).join('/') || '.';
            await extractTo(dest);
        }

        async function extractTo(dest) {
            if (!zipPath) return;
            if (!dest) {
                const prompt = ctx.promptDialog || (async () => null);
                dest = await prompt(t('zipper.extract_to'), zipPath.split('/').slice(0, -1).join('/') || 'Documents');
                if (!dest) return;
            }
            setStatus(t('zipper.extracting'));
            try {
                await api('/api/desktop/extract', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: zipPath, dest: dest })
                });
                setStatus(t('zipper.extracted'));
                notify({ type: 'success', message: t('zipper.extracted') });
                if (typeof ctx.loadBootstrap === 'function') await ctx.loadBootstrap();
                if (selected.size > 0) {
                    openApp('files', { path: dest });
                }
            } catch (err) {
                setStatus(t('zipper.error_extract'));
                notify({ type: 'error', message: err.message || String(err) });
            }
        }

        async function newArchive() {
            const prompt = ctx.promptDialog || (async () => null);
            const name = await prompt(t('zipper.new_archive'), 'Documents/archive.zip');
            if (!name) return;
            setStatus(t('zipper.creating'));
            try {
                await api('/api/desktop/archive', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ paths: [], dest: String(name).trim() })
                });
                setStatus(t('zipper.created'));
                notify({ type: 'success', message: t('zipper.created') });
                if (typeof ctx.loadBootstrap === 'function') await ctx.loadBootstrap();
            } catch (err) {
                setStatus(t('zipper.error_create'));
                notify({ type: 'error', message: err.message || String(err) });
            }
        }

        host.querySelector('[data-action="open"]').addEventListener('click', () => openFile());
        host.querySelector('[data-action="extract-here"]').addEventListener('click', () => extractHere());
        host.querySelector('[data-action="extract-to"]').addEventListener('click', () => extractTo(''));
        host.querySelector('[data-action="new-archive"]').addEventListener('click', () => newArchive());
        const closePreviewBtn = host.querySelector('[data-action="close-preview"]');
        if (closePreviewBtn) closePreviewBtn.addEventListener('click', () => closePreview());

        host.querySelectorAll('[data-sort]').forEach(th => {
            th.addEventListener('click', () => {
                const col = th.dataset.sort;
                if (sortCol === col) sortAsc = !sortAsc; else { sortCol = col; sortAsc = true; }
                applyFilter();
            });
        });

        if (selectAllCheckbox) {
            selectAllCheckbox.addEventListener('change', () => {
                if (selectAllCheckbox.checked) {
                    filteredEntries.forEach(e => selected.add(e.name));
                } else {
                    selected.clear();
                }
                renderTable();
                updateStatus();
            });
        }

        if (typeof ctx.setWindowMenus === 'function') {
            ctx.setWindowMenus(windowId, [
                {
                    id: 'file',
                    labelKey: 'desktop.menu_file',
                    items: [
                        { id: 'open', labelKey: 'zipper.open', icon: 'folder-open', shortcut: 'Ctrl+O', action: () => openFile() },
                        { id: 'open-entry', labelKey: 'zipper.open_entry', icon: 'folder-open', shortcut: 'Enter', action: () => openSelectedMember() },
                        { id: 'extract-here', labelKey: 'zipper.extract_here', icon: 'download', action: () => extractHere() },
                        { id: 'extract-to', labelKey: 'zipper.extract_to', icon: 'folder', action: () => extractTo('') },
                        { type: 'separator' },
                        { id: 'new-archive', labelKey: 'zipper.new_archive', icon: 'archive', action: () => newArchive() }
                    ]
                },
                {
                    id: 'edit',
                    labelKey: 'desktop.menu_edit',
                    items: [
                        { id: 'select-all', labelKey: 'desktop.fm.select_all', icon: 'check-square', shortcut: 'Ctrl+A', action: () => { filteredEntries.forEach(e => selected.add(e.name)); renderTable(); updateStatus(); } },
                        { id: 'select-none', labelKey: 'desktop.fm.select_none', icon: 'x', action: () => { selected.clear(); renderTable(); updateStatus(); } }
                    ]
                }
            ]);
        }

        state.dropDesktopFiles = createArchiveFromPaths;
        state.dropHostFiles = createArchiveFromHostFiles;
        load();

        const appEl = host.querySelector('.zipper-app');
        if (appEl) {
            appEl.addEventListener('dragover', event => {
                if (!event.dataTransfer) return;
                const types = Array.from(event.dataTransfer.types || []);
                const hasFileDrag = fileOps && typeof fileOps.hasDragPayload === 'function'
                    ? fileOps.hasDragPayload(event)
                    : types.includes('application/x-aurago-desktop-files');
                const hasPlainFile = types.includes('Files');
                const hasPlainPath = types.includes('text/plain');
                if (hasFileDrag || hasPlainFile || hasPlainPath) {
                    event.preventDefault();
                    event.dataTransfer.dropEffect = 'copy';
                    appEl.classList.add('zipper-drop-target');
                }
            });
            appEl.addEventListener('dragleave', event => {
                if (event.currentTarget === event.target || !appEl.contains(event.relatedTarget)) {
                    appEl.classList.remove('zipper-drop-target');
                }
            });
            appEl.addEventListener('drop', async event => {
                appEl.classList.remove('zipper-drop-target');
                event.preventDefault();
                event.stopPropagation();
                let paths = [];
                const payload = fileOps && typeof fileOps.readDragPayload === 'function' ? fileOps.readDragPayload(event) : null;
                if (payload && Array.isArray(payload.paths)) paths = payload.paths;
                const externalFiles = Array.from((event.dataTransfer && event.dataTransfer.files) || []);
                if (!paths.length && externalFiles.length) {
                    await createArchiveFromHostFiles(externalFiles);
                    return;
                }
                if (!paths.length) {
                    const text = event.dataTransfer.getData('text/plain');
                    if (text) paths = [text];
                }
                if (paths.length) await createArchiveFromPaths(paths);
            });
        }

        function onKeyDown(e) {
            if (e.target.closest('input, textarea, select')) return;
            if (e.key === 'Escape' && previewNode && !previewNode.hidden) {
                e.preventDefault();
                closePreview();
                return;
            }
            if (e.key === 'Enter') {
                e.preventDefault();
                openSelectedMember();
                return;
            }
            if (e.ctrlKey || e.metaKey) {
                if (e.key === 'o') { e.preventDefault(); openFile(); }
            }
        }
        host.addEventListener('keydown', onKeyDown);
    }

    function dispose(windowId) {
        instances.delete(windowId);
    }

    async function dropDesktopFiles(windowId, paths) {
        const state = instances.get(windowId);
        if (!state || typeof state.dropDesktopFiles !== 'function') return false;
        return !!(await state.dropDesktopFiles(paths));
    }

    async function dropHostFiles(windowId, files) {
        const state = instances.get(windowId);
        if (!state || typeof state.dropHostFiles !== 'function') return false;
        return !!(await state.dropHostFiles(files));
    }

    async function fetchJSON(url, options) {
        const resp = await fetch(url, options);
        const body = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(body.error || body.message || ('HTTP ' + resp.status));
        return body;
    }

    window.ZipperApp = window.ZipperApp || {};
    window.ZipperApp.render = render;
    window.ZipperApp.dispose = dispose;
    window.ZipperApp.dropDesktopFiles = dropDesktopFiles;
    window.ZipperApp.dropHostFiles = dropHostFiles;
})();
