    // A relative path is meaningful only together with its provider and mount.
    const STORAGE_DRAG_TYPE = 'application/x-aurago-storage-files';

    function storageLocation(path = fm.currentPath, instance = fm) {
        const value = path && typeof path === 'object' ? path : { ...instance.storage, path: String(path || '') };
        const loc = { provider: value.provider || 'local', mount_id: value.mount_id || '', path: String(value.path || '') };
        if (loc.provider === 'gog') {
            loc.account_id = value.account_id || '';
            const known = instance.gogLocations?.get(loc.account_id + '/' + loc.path);
            const same = typeof path === 'object' || loc.path === instance.currentPath;
            loc.item_id = typeof path === 'object' && Object.hasOwn(value, 'item_id') ? value.item_id : known?.item_id || (same ? value.item_id : '') || loc.path.split('/').pop() || 'root';
            loc.parent_id = known?.parent_id || (same ? value.parent_id : '') || '';
            loc.name = known?.name || (same ? value.name : '') || '';
        }
        return loc;
    }

    function storageChildLocation(parent, name) {
        const child = { ...parent, path: joinPath(parent.path, name) };
        if (parent.provider === 'gog') Object.assign(child, { item_id: '', parent_id: parent.item_id || 'root', name });
        return child;
    }

    function localLocation(path = '') { return { provider: 'local', mount_id: '', path }; }
    function isRemote(instance = fm) { return !!instance.storage && instance.storage.provider !== 'local'; }
    function locationKey(loc) { return JSON.stringify([loc.provider || 'local', loc.mount_id || '', loc.account_id || '', loc.item_id || loc.path || '']); }
    function savedLocation(value) { return typeof value === 'string' ? localLocation(value) : storageLocation(value); }
    function storageURL(action, loc) { return '/api/desktop/storage/' + action + '?' + new URLSearchParams(loc).toString(); }
    function storageName(instance = fm) {
        if (!isRemote(instance)) return t('desktop.fm.workspace_root');
        if (instance.storage.provider === 'gog') return 'Google Drive · ' + (instance.storagePolicy?.gog_accounts?.find(a => a.id === instance.storage.account_id)?.name || instance.storage.account_id);
        return instance.storage.provider === 'webdav' ? 'WebDAV' : 'Koofr · ' + (instance.storageMounts?.find(m => m.id === instance.storage.mount_id)?.name || instance.storage.mount_id);
    }
    function storageReadonly(instance = fm) {
        const p = instance.storagePolicy || {};
        if (p.readonly) return true;
        if (!isRemote(instance)) return false;
        if (!p[instance.storage.provider] || p[instance.storage.provider + '_readonly']) return true;
        if (instance.storage.provider === 'gog') return !p.gog_accounts?.find(a => a.id === instance.storage.account_id)?.write;
        if (instance.storage.provider === 'koofr') {
            const mount = instance.storageMounts?.find(m => m.id === instance.storage.mount_id);
            return !mount || !mount.write || mount.online === false;
        }
        return false;
    }

    async function loadStorageSources(instance, refresh = false) {
        const generation = instance.storagePolicyGeneration = (instance.storagePolicyGeneration || 0) + 1;
        try {
            const response = await fetch('/api/desktop/storage/sources', { credentials: 'same-origin', signal: instance.storageLifetime.signal });
            const policy = response.ok ? await response.json() : {};
            if (!isLiveInstance(instance) || instance.storagePolicyGeneration !== generation) return;
            const previous = instance.storagePolicy || {};
            const changed = refresh && ['webdav', 'koofr', 'readonly', 'webdav_readonly', 'koofr_readonly', 'gog', 'gog_readonly'].some(key => !!previous[key] !== !!policy[key]) || refresh && JSON.stringify(previous.gog_accounts) !== JSON.stringify(policy.gog_accounts);
            if (changed) {
                instance.storageBusy?.abort();
                if (isRemote(instance)) instance.quickLookOverlay?.remove();
                for (const pane of [instance.leftPane, instance.rightPane]) {
                    if (!pane || !isRemote(pane)) continue;
                    pane.files = [];
                    pane.filteredFiles = null;
                    pane.selectedPaths.clear();
                    pane.loading = true;
                }
            }
            instance.storagePolicy = policy;
            if (typeof policy.readonly === 'boolean') instance.callbacks.readonly = policy.readonly;
            if (!policy.koofr) {
                instance.storageMounts = [];
                instance.storageMountsOpen = false;
            } else if (changed && instance.storageMountsOpen && !!previous.koofr_readonly !== !!policy.koofr_readonly) {
                openStorageMounts(instance, true);
            }
            withInstance(instance, () => {
                if (isRemote() && !policy[instance.storage.provider]) navigate(localLocation());
                else if (changed && isRemote()) navigate(storageLocation(), false);
                else renderAll();
            });
        } catch (_) { /* Disposing the window cancels its metadata request. */ }
    }

    async function openStorageMounts(instance, reload = false) {
        instance.storageMountsOpen = reload || !instance.storageMountsOpen;
        if (!instance.storageMountsOpen) { withInstance(instance, renderAll); return; }
        instance.storageMountsLoading = true;
        withInstance(instance, renderAll);
        try {
            const result = await storageJSON('/api/desktop/storage/mounts', { signal: instance.storageLifetime.signal });
            if (isLiveInstance(instance)) instance.storageMounts = result.mounts || [];
        } catch (err) {
            storageNotify(instance, err);
        } finally {
            instance.storageMountsLoading = false;
            if (isLiveInstance(instance)) withInstance(instance, renderAll);
        }
    }

    function renderStorageSidebar() {
        const policy = fm.storagePolicy || {};
        const button = (provider, mount, name, disabled, status) => `<button type="button" class="fm-sidebar-item${fm.storage?.provider === provider && (fm.storage.account_id || fm.storage.mount_id || '') === mount ? ' active' : ''}" data-storage-provider="${esc(provider)}" data-storage-mount="${esc(mount)}" ${disabled ? 'disabled' : ''} title="${esc(status || name)}">${iconMarkup('cloud', '☁', 'fm-sidebar-icon', 18)}<span class="fm-sidebar-label">${esc(name)}${status ? `<small class="fm-storage-status">${esc(status)}</small>` : ''}</span></button>`;
        const mounts = fm.storageMountsLoading ? `<div class="fm-sidebar-empty" role="status">${esc(t('desktop.fm.loading'))}</div>` : (fm.storageMounts || []).map(m => button('koofr', m.id, m.name, m.online === false || !m.read, m.online === false ? t('desktop.fm.storage_offline') : (!m.write || policy.koofr_readonly ? t('desktop.fm.storage_readonly') : t('desktop.fm.storage_writable')))).join('');
        return `<div class="fm-sidebar-section">${button('local', '', t('desktop.fm.workspace_root'), false, '')}${(policy.gog_accounts || []).map(a => button('gog', a.id, 'Google Drive · ' + a.name, false, a.write ? '' : t('desktop.fm.storage_readonly'))).join('')}${policy.webdav ? button('webdav', '', 'WebDAV', false, policy.webdav_readonly ? t('desktop.fm.storage_readonly') : '') : ''}${policy.koofr ? `<button type="button" class="fm-sidebar-item" data-storage-group aria-expanded="${!!fm.storageMountsOpen}">${iconMarkup('cloud', '☁', 'fm-sidebar-icon', 18)}<span class="fm-sidebar-label">Koofr</span><span>${fm.storageMountsOpen ? '−' : '+'}</span></button>${fm.storageMountsOpen ? mounts || `<div class="fm-sidebar-empty">${esc(t('desktop.fm.empty_folder'))}</div>` : ''}` : ''}</div>`;
    }

    function attachStorageEvents(root) {
        const instance = fm;
        const dropLocation = target => {
            const paneName = target.closest('.fm-pane')?.dataset.pane;
            const pane = (paneName === 'right' ? instance.rightPane : paneName === 'left' ? instance.leftPane : null) || instance;
            const button = target.closest('[data-storage-provider]');
            if (button) return { provider: button.dataset.storageProvider, mount_id: button.dataset.storageProvider === 'gog' ? '' : button.dataset.storageMount, account_id: button.dataset.storageProvider === 'gog' ? button.dataset.storageMount : '', item_id: button.dataset.storageProvider === 'gog' ? 'root' : '', path: '' };
            const directory = target.closest('[data-type="directory"], [data-breadcrumb-path]');
            return storageLocation(directory?.dataset.breadcrumbPath ?? directory?.dataset.path ?? pane.currentPath, pane);
        };
        root.querySelectorAll('[data-remove-storage-favorite]').forEach(button => button.addEventListener('click', event => {
            event.stopPropagation();
            instance.favorites.splice(Number(button.dataset.removeStorageFavorite), 1);
            localStorage.setItem('aurago.fm.favorites', JSON.stringify(instance.favorites));
            withInstance(instance, renderAll);
        }));
        root.querySelector('[data-storage-group]')?.addEventListener('click', () => openStorageMounts(instance));
        root.querySelectorAll('[data-storage-provider]').forEach(button => button.addEventListener('click', () => withInstance(instance, () => {
            instance.sidebarOpen = false;
            navigate({ provider: button.dataset.storageProvider, mount_id: button.dataset.storageProvider === 'gog' ? '' : button.dataset.storageMount, account_id: button.dataset.storageProvider === 'gog' ? button.dataset.storageMount : '', item_id: button.dataset.storageProvider === 'gog' ? 'root' : '', path: '' });
        })));
        // Capture remote payloads before legacy local-only file and window drop handlers.
        root.addEventListener('dragover', event => {
            const destination = dropLocation(event.target);
            const types = Array.from(event.dataTransfer?.types || []);
            if (!types.includes(STORAGE_DRAG_TYPE) && !(destination.provider !== 'local' && types.includes(DESKTOP_FILE_DRAG_TYPE))) return;
            event.preventDefault(); event.stopPropagation();
            event.dataTransfer.dropEffect = event.target.closest('.fm-favorites-section') ? 'copy' : storageReadonly({ ...instance, storage: destination }) ? 'none' : (event.ctrlKey ? 'copy' : 'move');
        }, true);
        root.addEventListener('drop', event => {
            const destination = dropLocation(event.target);
            const types = Array.from(event.dataTransfer?.types || []);
            if (!types.includes(STORAGE_DRAG_TYPE) && !(destination.provider !== 'local' && types.includes(DESKTOP_FILE_DRAG_TYPE))) return;
            event.preventDefault(); event.stopImmediatePropagation();
            try {
                const payload = JSON.parse(event.dataTransfer.getData(types.includes(STORAGE_DRAG_TYPE) ? STORAGE_DRAG_TYPE : DESKTOP_FILE_DRAG_TYPE));
                const entries = payload.entries || (payload.paths || []).map(localLocation);
                if (event.target.closest('.fm-favorites-section')) {
                    entries.filter(entry => entry.type === 'directory').forEach(entry => {
                        const loc = storageLocation(entry);
                        if (!instance.favorites.some(fav => locationKey(savedLocation(fav)) === locationKey(loc))) instance.favorites.push(loc);
                    });
                    localStorage.setItem('aurago.fm.favorites', JSON.stringify(instance.favorites));
                    withInstance(instance, renderAll);
                    return;
                }
                if (storageReadonly({ ...instance, storage: destination })) return;
                storageTransferSelection(instance, entries, destination, event.ctrlKey ? 'copy' : 'move');
            } catch (err) { storageNotify(instance, err); }
        }, true);
    }

    function storageNotify(instance, err) {
        if (!isLiveInstance(instance)) return;
        withInstance(instance, () => showNotification({ type: 'error', message: err.name === 'AbortError' ? t('desktop.fm.storage_cancelled') : (err.partial ? t('desktop.fm.storage_partial') + ' ' : '') + (err.message === 'storage_source_retained' ? t('desktop.fm.storage_retained') : err.message || String(err)) }));
    }

    async function storageJSON(url, options = {}) {
        const response = await fetch(url, { credentials: 'same-origin', ...options });
        const body = await response.json();
        if (!response.ok) { const err = new Error(body.error || body.message || response.statusText); err.body = body; err.status = response.status; throw err; }
        return body;
    }

    function storageCopyPath(path, n) {
        const name = baseName(path), dot = name.lastIndexOf('.');
        return joinPath(parentPath(path), (dot > 0 ? name.slice(0, dot) : name) + ' (' + n + ')' + (dot > 0 ? name.slice(dot) : ''));
    }

    async function storageConflictRequest(instance, destination, execute, signal) {
        const original = destination.path;
        let version = '', copy = 0;
        for (;;) {
            if (signal.aborted) throw new DOMException('Cancelled', 'AbortError');
            try { return await execute(destination, version); }
            catch (err) {
                if (err.status !== 409 || !err.body?.conflict) throw err;
                const conflict = err.body.conflict;
                const decision = copy ? 'copy' : await instance.callbacks.storageConflict?.(conflict, signal);
                if (!decision) throw new DOMException('Cancelled', 'AbortError');
                if (decision === 'replace' && conflict.version && !conflict.directory) version = conflict.version;
                else {
                    if (++copy > 999) throw err;
                    destination = { ...destination, path: storageCopyPath(original, copy) };
                    if (destination.provider === 'gog') Object.assign(destination, { item_id: '', name: baseName(destination.path) });
                    version = '';
                }
            }
        }
    }

    async function storageOperation(instance, work) {
        if (!isLiveInstance(instance) || instance.storageBusy) return false;
        const controller = new AbortController();
        instance.storageBusy = controller;
        const overlay = document.createElement('div');
        overlay.className = 'fm-upload-overlay';
        overlay.innerHTML = `<div class="fm-upload-panel" role="dialog" aria-modal="true" aria-label="${esc(t('desktop.fm.storage_transfer'))}"><div class="fm-upload-title">${esc(t('desktop.fm.storage_transfer'))}</div><div role="status" aria-live="polite" data-storage-progress>${esc(t('desktop.fm.loading'))}</div><button type="button" class="fm-btn" data-storage-cancel>${esc(t('desktop.cancel'))}</button></div>`;
        (document.fullscreenElement || document.body).appendChild(overlay);
        overlay.querySelector('button').addEventListener('click', () => controller.abort());
        overlay.querySelector('button').focus();
        let complete = false;
        try {
            await work(controller.signal, message => { overlay.querySelector('[data-storage-progress]').textContent = message; });
            complete = true;
        } catch (err) { storageNotify(instance, err); }
        finally {
            overlay.remove(); instance.storageBusy = null;
            instances.forEach(other => {
                if (!isLiveInstance(other)) return;
                for (const pane of [other.leftPane, other.rightPane]) if (pane) pane.loading = true;
                withInstance(other, () => navigate(storageLocation(), false));
            });
        }
        return complete;
    }

    async function storageTransfer(instance, source, destination, mode, signal, progress) {
        return storageConflictRequest(instance, destination, async (target, version) => {
            const response = await fetch('/api/desktop/storage/transfer', { method: 'POST', credentials: 'same-origin', signal, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ location: source, destination: target, mode, version }) });
            if (!response.ok) {
                const body = await response.json(); const err = new Error(body.error || response.statusText); err.status = response.status; err.body = body; throw err;
            }
            const reader = response.body.getReader(), decoder = new TextDecoder();
            let buffer = '', completed = false;
            try {
                for (;;) {
                    const chunk = await reader.read();
                    buffer += decoder.decode(chunk.value || new Uint8Array(), { stream: !chunk.done });
                    let end;
                    while ((end = buffer.indexOf('\n\n')) >= 0) {
                        const block = buffer.slice(0, end); buffer = buffer.slice(end + 2);
                        if (!block.startsWith('data: ')) continue;
                        const event = JSON.parse(block.slice(6));
                        if (event.status === 'error') { const err = new Error(event.error); err.partial = event.partial; throw err; }
                        if (event.status === 'done') completed = true;
                        if (event.total) progress(baseName(source.path) + ' · ' + (event.done || 0) + '/' + event.total);
                    }
                    if (chunk.done) break;
                }
            } finally { await reader.cancel().catch(() => {}); reader.releaseLock(); }
            if (!completed) { const err = new Error(t('desktop.fm.storage_partial')); err.partial = true; throw err; }
            return target;
        }, signal);
    }

    function storageTransferSelection(instance, entries, destination, mode, clipboard) {
        return storageOperation(instance, async (signal, progress) => {
            for (const entry of entries) await storageTransfer(instance, savedLocation(entry), storageChildLocation(destination, entry.name || baseName(entry.path)), mode, signal, progress);
            if (mode === 'move' && clipboard && window.AuraDesktopFileClipboard === clipboard) window.AuraDesktopFileClipboard = null;
        });
    }
