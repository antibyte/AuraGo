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
                <button class="vd-tool-button vd-tool-button-icon" type="button" data-action="open" title="${esc(t('zipper.open', 'Open Archive'))}">${iconMarkup('folder-open', 'Open', 'vd-tool-icon', 15)}</button>
                <button class="vd-tool-button vd-tool-button-icon" type="button" data-action="extract-here" title="${esc(t('zipper.extract_here', 'Extract Here'))}">${iconMarkup('download', 'Extract', 'vd-tool-icon', 15)}</button>
                <button class="vd-tool-button vd-tool-button-icon" type="button" data-action="extract-to" title="${esc(t('zipper.extract_to', 'Extract To...'))}">${iconMarkup('folder', 'Extract To', 'vd-tool-icon', 15)}</button>
                <button class="vd-tool-button vd-tool-button-icon" type="button" data-action="new-archive" title="${esc(t('zipper.new_archive', 'New Archive'))}">${iconMarkup('archive', 'New', 'vd-tool-icon', 15)}</button>
                <span class="zipper-path vd-path">${esc(zipPath || t('zipper.no_archive', 'No archive open'))}</span>
            </div>
            <div class="zipper-breadcrumb" data-breadcrumb></div>
            <div class="zipper-list" data-list>
                <table class="zipper-table">
                    <thead>
                        <tr>
                            <th class="zipper-col-check"><input type="checkbox" data-select-all></th>
                            <th class="zipper-col-name" data-sort="name">${esc(t('zipper.name', 'Name'))}</th>
                            <th class="zipper-col-size" data-sort="size">${esc(t('zipper.size', 'Size'))}</th>
                            <th class="zipper-col-compressed" data-sort="compressed">${esc(t('zipper.compressed', 'Compressed'))}</th>
                            <th class="zipper-col-modified" data-sort="modified">${esc(t('zipper.modified', 'Modified'))}</th>
                        </tr>
                    </thead>
                    <tbody data-tbody></tbody>
                </table>
            </div>
            <div class="zipper-status" data-status></div>
        </div>`;

        const listHost = host.querySelector('[data-list]');
        const tbody = host.querySelector('[data-tbody]');
        const statusNode = host.querySelector('[data-status]');
        const breadcrumbNode = host.querySelector('[data-breadcrumb]');
        const selectAllCheckbox = host.querySelector('[data-select-all]');

        function fmtBytes(n) {
            n = Number(n || 0);
            if (n < 1024) return n + ' B';
            if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KiB';
            return (n / 1024 / 1024).toFixed(1) + ' MiB';
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
            let html = `<button class="zipper-crumb" type="button" data-dir="">${esc(t('zipper.title', 'Zipper'))}</button>`;
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

        function displayName(entry) {
            const prefix = currentDir ? currentDir + '/' : '';
            let name = entry.name.startsWith(prefix) ? entry.name.slice(prefix.length) : entry.name;
            if (entry.is_dir) name = name.replace(/\/$/, '');
            return name;
        }

        function renderTable() {
            if (!tbody) return;
            if (filteredEntries.length === 0) {
                tbody.innerHTML = `<tr><td colspan="5" class="zipper-empty">${esc(t('zipper.empty_archive', 'This archive is empty'))}</td></tr>`;
                return;
            }
            tbody.innerHTML = filteredEntries.map((e, i) => {
                const name = displayName(e);
                const checked = selected.has(e.name) ? 'checked' : '';
                const icon = e.is_dir ? iconMarkup('folder', 'Dir') : iconMarkup('archive', 'File');
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
                    if (entry.is_dir) {
                        currentDir = entry.name.replace(/\/$/, '');
                        selected.clear();
                        applyFilter();
                    }
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
                t('zipper.items', '{{count}} items').replace('{{count}}', count),
                t('zipper.total_size', '{{size}} total').replace('{{size}}', fmtBytes(totalSize)),
                fmtBytes(totalCompressed) + ' compressed'
            ].join('  ·  ');
            setStatus((selected.size > 0 ? selected.size + ' selected  ·  ' : '') + msg);
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
            if (!zipPath) { setStatus(t('zipper.no_archive', 'No archive open')); return; }
            setStatus(t('zipper.extracting', 'Loading...'));
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
                setStatus(t('zipper.error_list', 'Failed to read archive'));
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
                setStatus(t('zipper.error_create', 'Failed to create archive'));
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
            let dest = await prompt(t('zipper.new_archive', 'New Archive'), joinPath(defaultDir, defaultArchiveName(cleanPaths)));
            if (!dest) return false;
            dest = ensureZipExtension(dest);
            setStatus(t('zipper.creating', 'Creating archive...'));
            try {
                await api('/api/desktop/archive', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ paths: cleanPaths, dest: dest })
                });
                setStatus(t('zipper.created', 'Archive created'));
                notify({ type: 'success', message: t('zipper.created', 'Archive created') });
                if (typeof ctx.loadBootstrap === 'function') await ctx.loadBootstrap();
                openZipPath(dest);
                return true;
            } catch (err) {
                setStatus(t('zipper.error_create', 'Failed to create archive'));
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
                dest = await prompt(t('zipper.extract_to', 'Extract To...'), zipPath.split('/').slice(0, -1).join('/') || 'Documents');
                if (!dest) return;
            }
            setStatus(t('zipper.extracting', 'Extracting...'));
            try {
                await api('/api/desktop/extract', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: zipPath, dest: dest })
                });
                setStatus(t('zipper.extracted', 'Extraction complete'));
                notify({ type: 'success', message: t('zipper.extracted', 'Extraction complete') });
                if (typeof ctx.loadBootstrap === 'function') await ctx.loadBootstrap();
                if (selected.size > 0) {
                    openApp('files', { path: dest });
                }
            } catch (err) {
                setStatus(t('zipper.error_extract', 'Failed to extract archive'));
                notify({ type: 'error', message: err.message || String(err) });
            }
        }

        async function newArchive() {
            const prompt = ctx.promptDialog || (async () => null);
            const name = await prompt(t('zipper.new_archive', 'New Archive'), 'Documents/archive.zip');
            if (!name) return;
            setStatus(t('zipper.creating', 'Creating archive...'));
            try {
                await api('/api/desktop/archive', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ paths: [], dest: String(name).trim() })
                });
                setStatus(t('zipper.created', 'Archive created'));
                notify({ type: 'success', message: t('zipper.created', 'Archive created') });
                if (typeof ctx.loadBootstrap === 'function') await ctx.loadBootstrap();
            } catch (err) {
                setStatus(t('zipper.error_create', 'Failed to create archive'));
                notify({ type: 'error', message: err.message || String(err) });
            }
        }

        host.querySelector('[data-action="open"]').addEventListener('click', () => openFile());
        host.querySelector('[data-action="extract-here"]').addEventListener('click', () => extractHere());
        host.querySelector('[data-action="extract-to"]').addEventListener('click', () => extractTo(''));
        host.querySelector('[data-action="new-archive"]').addEventListener('click', () => newArchive());

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
