    const fileDialogDefaultRoots = ['Desktop', 'Documents', 'Downloads', 'Pictures', 'Music', 'Videos', 'Apps', 'Widgets', 'Shared'];

    function fileDialogText(key, fallback, vars) {
        let value = '';
        if (typeof t === 'function') {
            try { value = t(key); } catch (_) { value = ''; }
        }
        if (!value || value === key) value = fallback || key;
        Object.entries(vars || {}).forEach(([name, replacement]) => {
            value = value.replaceAll('{{' + name + '}}', String(replacement == null ? '' : replacement));
        });
        return value;
    }

    function fileDialogEscape(value) {
        return typeof esc === 'function' ? esc(value) : String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
        }[ch]));
    }

    function fileDialogNotify(message) {
        if (typeof showDesktopNotification === 'function') {
            showDesktopNotification({ title: fileDialogText('desktop.notification', 'Notification'), message });
        }
    }

    function normalizeFileDialogPath(path) {
        return String(path || '').replace(/\\/g, '/').replace(/^\/+/, '').replace(/\/+/g, '/').replace(/\/+$/, '');
    }

    function fileDialogJoinPath(base, name) {
        const left = normalizeFileDialogPath(base);
        const right = normalizeFileDialogPath(name);
        return left ? (right ? left + '/' + right : left) : right;
    }

    function fileDialogBaseName(path) {
        const parts = normalizeFileDialogPath(path).split('/').filter(Boolean);
        return parts.pop() || '';
    }

    function fileDialogDirName(path) {
        const parts = normalizeFileDialogPath(path).split('/').filter(Boolean);
        parts.pop();
        return parts.join('/');
    }

    function fileDialogHasExtension(name) {
        const base = fileDialogBaseName(name);
        return /\.[^./\\]+$/.test(base);
    }

    function normalizeFileDialogExtension(ext) {
        const value = String(ext || '').trim().toLowerCase();
        if (!value) return '';
        return value.startsWith('.') ? value : '.' + value;
    }

    function normalizeFileDialogFilters(filters) {
        return (Array.isArray(filters) ? filters : []).map((filter, index) => {
            const extensions = (filter.extensions || filter.ext || []).map(normalizeFileDialogExtension).filter(Boolean);
            const mimeTypes = (filter.mimeTypes || filter.mime_types || []).map(item => String(item || '').toLowerCase()).filter(Boolean);
            const label = filter.label || filter.name || (extensions.length ? extensions.join(', ') : fileDialogText('desktop.file_dialog_filter', 'Filter')) || String(index + 1);
            return { label, extensions, mimeTypes };
        }).filter(filter => filter.extensions.length || filter.mimeTypes.length || filter.label);
    }

    function fileDialogAcceptFromFilters(filters, accept) {
        if (accept) return String(accept);
        const parts = [];
        (filters || []).forEach(filter => {
            parts.push(...(filter.extensions || []), ...(filter.mimeTypes || []));
        });
        return [...new Set(parts)].join(',');
    }

    function fileDialogEntryPath(entry, currentPath) {
        return normalizeFileDialogPath(entry.path || entry.full_path || fileDialogJoinPath(currentPath, entry.name));
    }

    function fileDialogEntryType(entry) {
        return String(entry.type || entry.kind || '').toLowerCase();
    }

    function fileDialogIsDirectory(entry) {
        const type = fileDialogEntryType(entry);
        return type === 'directory' || type === 'folder' || entry.is_dir === true || entry.directory === true;
    }

    function fileDialogEntryMime(entry) {
        return String(entry.mime_type || entry.mimeType || entry.mime || '').toLowerCase();
    }

    function fileDialogEntryMatchesFilter(entry, filter) {
        if (!filter || fileDialogIsDirectory(entry)) return true;
        const name = String(entry.name || fileDialogBaseName(entry.path)).toLowerCase();
        const mime = fileDialogEntryMime(entry);
        if ((filter.mimeTypes || []).some(item => item === mime || (item.endsWith('/*') && mime.startsWith(item.slice(0, -1))))) return true;
        return (filter.extensions || []).some(ext => name.endsWith(ext));
    }

    function fileDialogSortEntries(entries) {
        return [...(entries || [])].sort((a, b) => {
            const ad = fileDialogIsDirectory(a) ? 0 : 1;
            const bd = fileDialogIsDirectory(b) ? 0 : 1;
            if (ad !== bd) return ad - bd;
            return String(a.name || '').localeCompare(String(b.name || ''), undefined, { sensitivity: 'base' });
        });
    }

    function fileDialogRoots() {
        const boot = state.bootstrap || {};
        const candidates = [];
        const add = value => {
            const path = normalizeFileDialogPath(typeof value === 'string' ? value : (value && (value.path || value.name)));
            if (path && !candidates.some(item => item.toLowerCase() === path.toLowerCase())) candidates.push(path);
        };
        (boot.workspace_directories || boot.workspaceDirectories || boot.directories || []).forEach(add);
        fileDialogDefaultRoots.forEach(add);
        return candidates;
    }

    async function fileDialogList(path, options) {
        const endpoint = options.filesEndpoint || '/api/desktop/files';
        const body = await api(endpoint + '?path=' + encodeURIComponent(normalizeFileDialogPath(path)));
        return fileDialogSortEntries(Array.isArray(body.files) ? body.files : []);
    }

    function fileDialogReadonly(options) {
        return !!(options && options.readonly) || !!((state.bootstrap || {}).readonly);
    }

    function ensureFileDialogExtension(name, defaultExtension) {
        const ext = normalizeFileDialogExtension(defaultExtension);
        const value = String(name || '').trim();
        if (!value || !ext || fileDialogHasExtension(value)) return value;
        return value + ext;
    }

    async function confirmOverwrite(path, options) {
        if (options && options.confirmOverwrite === false) return true;
        const parent = fileDialogDirName(path);
        const name = fileDialogBaseName(path).toLowerCase();
        try {
            const entries = await fileDialogList(parent, options || {});
            const exists = entries.some(entry => !fileDialogIsDirectory(entry) && String(entry.name || fileDialogBaseName(entry.path)).toLowerCase() === name);
            if (!exists) return true;
        } catch (_) {
            return true;
        }
        if (typeof confirmDialog === 'function') {
            return !!(await confirmDialog(
                fileDialogText('desktop.file_dialog_overwrite_title', 'Overwrite file?'),
                fileDialogText('desktop.file_dialog_overwrite_message', 'A file named "{{name}}" already exists. Replace it?', { name: fileDialogBaseName(path) })
            ));
        }
        return window.confirm(fileDialogText('desktop.file_dialog_overwrite_message', 'A file named "{{name}}" already exists. Replace it?', { name: fileDialogBaseName(path) }));
    }

    function fileDialogTopPath(path) {
        return normalizeFileDialogPath(path).split('/').filter(Boolean)[0] || '';
    }

    function fileDialogIcon(entry) {
        if (typeof iconForFile === 'function' && typeof iconMarkup === 'function') {
            return iconMarkup(iconForFile(entry), fileDialogIsDirectory(entry) ? 'D' : 'F', 'vd-file-dialog-entry-icon-img', 18);
        }
        return fileDialogIsDirectory(entry) ? 'D' : 'F';
    }

    function openDesktopFileDialog(options) {
        return showDesktopFileDialog(Object.assign({}, options || {}, {
            mode: options && options.multiple ? 'open-files' : ((options && options.mode) || 'open-file')
        }));
    }

    function saveDesktopFileDialog(options) {
        return showDesktopFileDialog(Object.assign({}, options || {}, { mode: 'save-file' }));
    }

    function showDesktopFileDialog(options) {
        options = options || {};
        const mode = options.mode || 'open-file';
        const isSave = mode === 'save-file';
        const isMulti = mode === 'open-files';
        const isFolder = mode === 'select-folder';
        const filters = normalizeFileDialogFilters(options.filters);
        let currentPath = normalizeFileDialogPath(options.initialPath || options.path || state.filesPath || 'Documents');
        if (!currentPath) currentPath = 'Documents';
        if (!isSave && options.initialFile) currentPath = fileDialogDirName(options.initialFile);
        let entries = [];
        let selected = new Set();
        let activeFilterIndex = '';
        let search = '';

        const overlay = document.createElement('div');
        overlay.className = 'vd-file-dialog-backdrop';
        overlay.innerHTML = `<form class="vd-file-dialog" role="dialog" aria-modal="true">
            <header class="vd-file-dialog-header">
                <div>
                    <div class="vd-file-dialog-title">${fileDialogEscape(options.title || fileDialogText(isSave ? 'desktop.file_dialog_save' : (isFolder ? 'desktop.file_dialog_select_folder' : 'desktop.file_dialog_open'), isSave ? 'Save' : (isFolder ? 'Select folder' : 'Open')))}</div>
                    <div class="vd-file-dialog-status" data-file-dialog-status></div>
                </div>
                <button type="button" class="vd-file-dialog-close" data-file-dialog-cancel aria-label="${fileDialogEscape(fileDialogText('desktop.cancel', 'Cancel'))}">&times;</button>
            </header>
            <div class="vd-file-dialog-layout">
                <aside class="vd-file-dialog-sidebar" data-file-dialog-sidebar></aside>
                <section class="vd-file-dialog-main">
                    <nav class="vd-file-dialog-breadcrumbs" data-file-dialog-breadcrumbs></nav>
                    <div class="vd-file-dialog-tools">
                        <input class="vd-file-dialog-search" data-file-dialog-search type="search" placeholder="${fileDialogEscape(fileDialogText('desktop.file_dialog_search', 'Search'))}" autocomplete="off">
                        <select class="vd-file-dialog-filter" data-file-dialog-filter></select>
                    </div>
                    <div class="vd-file-dialog-list" data-file-dialog-list data-file-dialog-multi-select="${isMulti ? 'true' : 'false'}" role="listbox"></div>
                </section>
            </div>
            <footer class="vd-file-dialog-footer">
                <label class="vd-file-dialog-name ${isSave ? '' : 'is-hidden'}">
                    <span>${fileDialogEscape(fileDialogText('desktop.file_dialog_name', 'File name'))}</span>
                    <input data-file-dialog-filename type="text" autocomplete="off" value="${fileDialogEscape(options.defaultName || options.filename || fileDialogBaseName(options.path))}">
                </label>
                <div class="vd-file-dialog-actions">
                    <button type="button" class="vd-button" data-file-dialog-import>${fileDialogEscape(fileDialogText('desktop.file_dialog_import', 'Import'))}</button>
                    <button type="button" class="vd-button" data-file-dialog-cancel>${fileDialogEscape(fileDialogText('desktop.cancel', 'Cancel'))}</button>
                    <button type="submit" class="vd-button vd-button-primary" data-file-dialog-confirm>${fileDialogEscape(fileDialogText(isSave ? 'desktop.file_dialog_save' : (isFolder ? 'desktop.file_dialog_select_folder' : 'desktop.file_dialog_open'), isSave ? 'Save' : (isFolder ? 'Select folder' : 'Open')))}</button>
                </div>
            </footer>
        </form>`;
        document.body.appendChild(overlay);

        const form = overlay.querySelector('form');
        const list = overlay.querySelector('[data-file-dialog-list]');
        const sidebar = overlay.querySelector('[data-file-dialog-sidebar]');
        const breadcrumbs = overlay.querySelector('[data-file-dialog-breadcrumbs]');
        const searchInput = overlay.querySelector('[data-file-dialog-search]');
        const filterSelect = overlay.querySelector('[data-file-dialog-filter]');
        const filenameInput = overlay.querySelector('[data-file-dialog-filename]');
        const statusEl = overlay.querySelector('[data-file-dialog-status]');
        const importButton = overlay.querySelector('[data-file-dialog-import]');
        const confirmButton = overlay.querySelector('[data-file-dialog-confirm]');
        const readonly = fileDialogReadonly(options);
        if (readonly && (isSave || options.importDisabled)) {
            statusEl.textContent = fileDialogText('desktop.file_dialog_readonly', 'Read-only mode: saving and importing are disabled.');
        }
        importButton.disabled = readonly;
        if (isSave && readonly) confirmButton.disabled = true;

        function setStatus(message) {
            statusEl.textContent = message || '';
        }

        function currentFilter() {
            const index = activeFilterIndex === '' ? -1 : Number(activeFilterIndex);
            return index >= 0 ? filters[index] : null;
        }

        function renderSidebar() {
            sidebar.innerHTML = fileDialogRoots().map(root => {
                const active = fileDialogTopPath(currentPath).toLowerCase() === root.toLowerCase() ? ' active' : '';
                return `<button type="button" class="vd-file-dialog-sidebar-item${active}" data-root="${fileDialogEscape(root)}">${fileDialogEscape(fileDialogBaseName(root) || root)}</button>`;
            }).join('');
            sidebar.querySelectorAll('[data-root]').forEach(btn => {
                btn.addEventListener('click', () => navigate(btn.dataset.root || ''));
            });
        }

        function renderBreadcrumbs() {
            const parts = currentPath.split('/').filter(Boolean);
            const crumbs = [''].concat(parts);
            breadcrumbs.innerHTML = crumbs.map((_, index) => {
                const path = parts.slice(0, index).join('/');
                const label = index === 0 ? fileDialogText('desktop.file_dialog_workspace', 'Workspace') : parts[index - 1];
                return `<button type="button" data-crumb="${fileDialogEscape(path)}">${fileDialogEscape(label)}</button>`;
            }).join('<span>/</span>');
            breadcrumbs.querySelectorAll('[data-crumb]').forEach(btn => {
                btn.addEventListener('click', () => navigate(btn.dataset.crumb || ''));
            });
        }

        function renderFilters() {
            filterSelect.innerHTML = `<option value="">${fileDialogEscape(fileDialogText('desktop.file_dialog_all_files', 'All files'))}</option>` +
                filters.map((filter, index) => `<option value="${index}">${fileDialogEscape(filter.label)}</option>`).join('');
            filterSelect.value = activeFilterIndex;
            filterSelect.disabled = !filters.length;
        }

        function filteredEntries() {
            const filter = currentFilter();
            const query = search.trim().toLowerCase();
            return entries.filter(entry => {
                if (!fileDialogEntryMatchesFilter(entry, filter)) return false;
                if (!query) return true;
                return String(entry.name || fileDialogBaseName(entry.path)).toLowerCase().includes(query);
            });
        }

        function selectedEntries() {
            return entries.filter(entry => selected.has(fileDialogEntryPath(entry, currentPath)));
        }

        function renderList() {
            const visible = filteredEntries();
            if (!visible.length) {
                list.innerHTML = `<div class="vd-file-dialog-empty">${fileDialogEscape(search ? fileDialogText('desktop.file_dialog_no_match', 'No matching files') : fileDialogText('desktop.file_dialog_empty', 'This folder is empty'))}</div>`;
                return;
            }
            list.innerHTML = visible.map((entry, index) => {
                const path = fileDialogEntryPath(entry, currentPath);
                const selectedClass = selected.has(path) ? ' selected' : '';
                const type = fileDialogIsDirectory(entry) ? fileDialogText('desktop.file_dialog_folder', 'Folder') : (entry.media_kind || fileDialogEntryMime(entry) || fileDialogText('desktop.file_dialog_file', 'File'));
                return `<button type="button" class="vd-file-dialog-row${selectedClass}" data-entry-index="${index}" data-path="${fileDialogEscape(path)}" role="option" aria-selected="${selected.has(path) ? 'true' : 'false'}">
                    <span class="vd-file-dialog-entry-icon">${fileDialogIcon(entry)}</span>
                    <span class="vd-file-dialog-entry-name">${fileDialogEscape(entry.name || fileDialogBaseName(path))}</span>
                    <span class="vd-file-dialog-entry-type">${fileDialogEscape(type)}</span>
                    <span class="vd-file-dialog-entry-size">${fileDialogEscape(entry.size_human || entry.sizeHuman || '')}</span>
                </button>`;
            }).join('');
            list.querySelectorAll('[data-entry-index]').forEach((btn, index) => {
                const entry = visible[index];
                btn.addEventListener('click', event => selectEntry(entry, event));
                btn.addEventListener('dblclick', () => activateEntry(entry));
            });
        }

        function render() {
            renderSidebar();
            renderBreadcrumbs();
            renderFilters();
            renderList();
        }

        async function reload() {
            try {
                setStatus(fileDialogText('desktop.loading', 'Loading...'));
                entries = await fileDialogList(currentPath, options);
                selected = new Set([...selected].filter(path => entries.some(entry => fileDialogEntryPath(entry, currentPath) === path)));
                setStatus(readonly && isSave ? fileDialogText('desktop.file_dialog_readonly', 'Read-only mode: saving and importing are disabled.') : '');
                render();
            } catch (err) {
                setStatus(err.message || fileDialogText('desktop.file_dialog_error', 'Could not load folder.'));
                entries = [];
                render();
            }
        }

        function navigate(path) {
            currentPath = normalizeFileDialogPath(path);
            selected.clear();
            reload();
        }

        function selectEntry(entry, event) {
            const path = fileDialogEntryPath(entry, currentPath);
            if (fileDialogIsDirectory(entry) && !isFolder) {
                if (isSave) {
                    selected = new Set([path]);
                    filenameInput.value = entry.name || fileDialogBaseName(path);
                } else if (!event || !event.ctrlKey) {
                    selected = new Set([path]);
                }
            } else if (isMulti && event && (event.ctrlKey || event.metaKey)) {
                selected.has(path) ? selected.delete(path) : selected.add(path);
            } else {
                selected = new Set([path]);
                if (isSave && !fileDialogIsDirectory(entry)) filenameInput.value = entry.name || fileDialogBaseName(path);
            }
            renderList();
        }

        function activateEntry(entry) {
            if (fileDialogIsDirectory(entry)) {
                navigate(fileDialogEntryPath(entry, currentPath));
                return;
            }
            if (!isSave) finishSelection();
        }

        async function finishSelection() {
            if (isSave) {
                const filename = ensureFileDialogExtension(filenameInput.value, options.defaultExtension);
                if (!filename) {
                    setStatus(fileDialogText('desktop.file_dialog_name_required', 'Enter a file name.'));
                    filenameInput.focus();
                    return;
                }
                const path = fileDialogJoinPath(currentPath, filename);
                if (!(await confirmOverwrite(path, options))) return;
                if (typeof options.content === 'string') {
                    await api(options.fileEndpoint || '/api/desktop/file', {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ path, content: options.content })
                    });
                }
                finish({ canceled: false, path, name: fileDialogBaseName(path) });
                return;
            }
            if (isFolder) {
                const picked = selectedEntries().find(fileDialogIsDirectory);
                const path = picked ? fileDialogEntryPath(picked, currentPath) : currentPath;
                finish({ canceled: false, path, folder: { path, name: fileDialogBaseName(path) || path } });
                return;
            }
            const files = selectedEntries().filter(entry => !fileDialogIsDirectory(entry));
            if (!files.length) {
                setStatus(fileDialogText('desktop.file_dialog_no_file_selected', 'Select a file.'));
                return;
            }
            const mapped = files.map(entry => Object.assign({}, entry, { path: fileDialogEntryPath(entry, currentPath) }));
            finish({ canceled: false, file: mapped[0] || null, files: isMulti ? mapped : mapped.slice(0, 1), path: mapped[0] ? mapped[0].path : '', paths: mapped.map(item => item.path) });
        }

        function finish(result) {
            document.removeEventListener('keydown', onKeydown);
            overlay.remove();
            resolveDialog(result);
        }

        let resolveDialog;
        const promise = new Promise(resolve => { resolveDialog = resolve; });
        const cancel = () => finish({ canceled: true });
        const onKeydown = event => { if (event.key === 'Escape') cancel(); };
        document.addEventListener('keydown', onKeydown);
        overlay.querySelectorAll('[data-file-dialog-cancel]').forEach(btn => btn.addEventListener('click', cancel));
        overlay.addEventListener('click', event => { if (event.target === overlay) cancel(); });
        form.addEventListener('submit', event => {
            event.preventDefault();
            finishSelection().catch(err => setStatus(err.message || fileDialogText('desktop.file_dialog_error', 'File dialog failed.')));
        });
        searchInput.addEventListener('input', () => {
            search = searchInput.value || '';
            renderList();
        });
        filterSelect.addEventListener('change', () => {
            activeFilterIndex = filterSelect.value || '';
            renderList();
        });
        importButton.addEventListener('click', async () => {
            if (readonly) {
                setStatus(fileDialogText('desktop.file_dialog_readonly', 'Read-only mode: saving and importing are disabled.'));
                return;
            }
            const result = await importHostFiles(Object.assign({}, options, { path: currentPath, accept: fileDialogAcceptFromFilters(filters, options.accept) }));
            if (!result || result.canceled) return;
            await reload();
        });
        render();
        reload();
        window.setTimeout(() => (isSave ? filenameInput : searchInput).focus(), 0);
        return promise;
    }

    function importHostFiles(options) {
        options = options || {};
        if (fileDialogReadonly(options)) {
            const message = fileDialogText('desktop.file_dialog_readonly', 'Read-only mode: saving and importing are disabled.');
            fileDialogNotify(message);
            return Promise.resolve({ canceled: true, error: message });
        }
        const filters = normalizeFileDialogFilters(options.filters);
        const input = document.createElement('input');
        input.type = 'file';
        input.hidden = true;
        if (options.multiple !== false) input.multiple = true;
        input.accept = fileDialogAcceptFromFilters(filters, options.accept);
        document.body.appendChild(input);
        return new Promise(resolve => {
            input.addEventListener('change', async () => {
                const files = Array.from(input.files || []);
                input.remove();
                if (!files.length) {
                    resolve({ canceled: true });
                    return;
                }
                try {
                    const uploaded = [];
                    for (const file of files) {
                        const form = new FormData();
                        form.append('path', normalizeFileDialogPath(options.path || options.initialPath || state.filesPath || 'Documents'));
                        form.append('file', file);
                        await api(options.uploadURL || options.uploadEndpoint || '/api/desktop/upload', { method: 'POST', body: form });
                        uploaded.push({ name: file.name, path: fileDialogJoinPath(options.path || options.initialPath || state.filesPath || 'Documents', file.name), size: file.size, type: file.type });
                    }
                    if (typeof loadBootstrap === 'function') loadBootstrap().catch(() => {});
                    resolve({ canceled: false, files: uploaded, paths: uploaded.map(item => item.path) });
                } catch (err) {
                    fileDialogNotify(err.message || fileDialogText('desktop.file_dialog_upload_error', 'Import failed.'));
                    resolve({ canceled: true, error: err.message || String(err) });
                }
            }, { once: true });
            input.click();
        });
    }

    function exportWorkspaceFile(options) {
        const opts = typeof options === 'string' ? { path: options } : (options || {});
        const path = normalizeFileDialogPath(opts.path || opts.workspacePath || '');
        const url = opts.url || opts.href || ('/api/desktop/download?path=' + encodeURIComponent(path));
        const link = document.createElement('a');
        link.href = url;
        link.download = opts.name || opts.filename || fileDialogBaseName(path) || '';
        link.rel = 'noopener';
        document.body.appendChild(link);
        link.click();
        link.remove();
        return Promise.resolve({ exported: true, path, url });
    }

    window.AuraDesktopFileDialogs = {
        open: openDesktopFileDialog,
        save: saveDesktopFileDialog,
        importHostFiles,
        exportWorkspaceFile
    };
