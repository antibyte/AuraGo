            if (base === 8) return ch >= '0' && ch <= '7';
            if (base === 10) return ch >= '0' && ch <= '9';
            if (base === 16) return /[0-9A-Fa-f]/.test(ch);
            return false;
        };
        while (index < expression.length) {
            const ch = expression[index];
            if (ch === ' ' || ch === '\t') { index++; continue; }
            if (isDigit(ch)) {
                let value = '';
                while (index < expression.length && isDigit(expression[index])) {
                    value += expression[index];
                    index++;
                }
                tokens.push({ type: 'number', value: parseInt(value, base) });
                continue;
            }
            if (ch === 'N' && expression.slice(index, index + 3) === 'NOT') {
                tokens.push({ type: 'operator', value: 'NOT' });
                index += 3;
                continue;
            }
            if (ch === 'A' && expression.slice(index, index + 3) === 'AND') {
                tokens.push({ type: 'operator', value: 'AND' });
                index += 3;
                continue;
            }
            if (ch === 'O' && expression.slice(index, index + 2) === 'OR') {
                tokens.push({ type: 'operator', value: 'OR' });
                index += 2;
                continue;
            }
            if (ch === 'X' && expression.slice(index, index + 3) === 'XOR') {
                tokens.push({ type: 'operator', value: 'XOR' });
                index += 3;
                continue;
            }
            if (ch === 'M' && expression.slice(index, index + 3) === 'MOD') {
                tokens.push({ type: 'operator', value: 'MOD' });
                index += 3;
                continue;
            }
            if (ch === 'S' && expression.slice(index, index + 3) === 'SHL') {
                tokens.push({ type: 'operator', value: 'SHL' });
                index += 3;
                continue;
            }
            if (ch === 'S' && expression.slice(index, index + 3) === 'SHR') {
                tokens.push({ type: 'operator', value: 'SHR' });
                index += 3;
                continue;
            }
            if (ch === '<' && expression[index + 1] === '<') {
                tokens.push({ type: 'operator', value: 'SHL' });
                index += 2;
                continue;
            }
            if (ch === '>' && expression[index + 1] === '>') {
                tokens.push({ type: 'operator', value: 'SHR' });
                index += 2;
                continue;
            }
            if (ch === '&') { tokens.push({ type: 'operator', value: 'AND' }); index++; continue; }
            if (ch === '|') { tokens.push({ type: 'operator', value: 'OR' }); index++; continue; }
            if (ch === '^') { tokens.push({ type: 'operator', value: 'XOR' }); index++; continue; }
            if (ch === '~') { tokens.push({ type: 'operator', value: 'NOT' }); index++; continue; }
            if (ch === '%') { tokens.push({ type: 'operator', value: 'MOD' }); index++; continue; }
            if (ch === '+') { tokens.push({ type: 'operator', value: '+' }); index++; continue; }
            if (ch === '-') { tokens.push({ type: 'operator', value: '-' }); index++; continue; }
            if (ch === '*' || ch === '×') { tokens.push({ type: 'operator', value: '*' }); index++; continue; }
            if (ch === '/' || ch === '÷') { tokens.push({ type: 'operator', value: '/' }); index++; continue; }
            if (ch === '(') { tokens.push({ type: 'lparen', value: '(' }); index++; continue; }
            if (ch === ')') { tokens.push({ type: 'rparen', value: ')' }); index++; continue; }
            throw new Error('Invalid expression');
        }
        tokens.push({ type: 'eof', value: '' });
        return tokens;
    }

    function parseProgrammerExpression(tokens) {
        let index = 0;
        const current = () => tokens[index];
        const consume = () => tokens[index++];
        const expect = type => {
            if (current().type !== type) throw new Error('Invalid expression');
            return consume();
        };

        const parseExpression = () => parseOrExpression();

        const parseOrExpression = () => {
            let left = parseXorExpression();
            while (current().value === 'OR') {
                consume();
                left = left | parseXorExpression();
            }
            return left;
        };

        const parseXorExpression = () => {
            let left = parseAndExpression();
            while (current().value === 'XOR') {
                consume();
                left = left ^ parseAndExpression();
            }
            return left;
        };

        const parseAndExpression = () => {
            let left = parseShiftExpression();
            while (current().value === 'AND') {
                consume();
                left = left & parseShiftExpression();
            }
            return left;
        };

        const parseShiftExpression = () => {
            let left = parseAdditiveExpression();
            while (['SHL', 'SHR'].includes(current().value)) {
                const op = consume().value;
                const right = parseAdditiveExpression();
                left = op === 'SHL' ? left << right : left >> right;
            }
            return left;
        };

        const parseAdditiveExpression = () => {
            let left = parseMultiplicativeExpression();
            while (['+', '-'].includes(current().value)) {
                const op = consume().value;
                const right = parseMultiplicativeExpression();
                left = op === '+' ? left + right : left - right;
            }
            return left;
        };

        const parseMultiplicativeExpression = () => {
            let left = parseUnaryExpression();
            while (['*', '/', 'MOD'].includes(current().value)) {
                const op = consume().value;
                const right = parseUnaryExpression();
                rejectZeroDivisor(op, right);
                if (op === '*') left = left * right;
                else if (op === '/') {
                    left = Math.trunc(left / right);
                } else {
                    left = left % right;
                }
            }
            return left;
        };

        const parseUnaryExpression = () => {
            if (current().value === 'NOT') {
                consume();
                return ~parseUnaryExpression();
            }
            if (current().value === '-') {
                consume();
                return -parseUnaryExpression();
            }
            return parsePrimaryExpression();
        };

        const parsePrimaryExpression = () => {
            if (current().type === 'number') {
                return consume().value;
            }
            if (current().type === 'lparen') {
                consume();
                const value = parseExpression();
                expect('rparen');
                return value;
            }
            throw new Error('Invalid expression');
        };

        const result = parseExpression();
        if (current().type !== 'eof') throw new Error('Invalid expression');
        if (!Number.isFinite(result) || !Number.isInteger(result)) throw new Error('Invalid expression');
        return result;
    }

    async function plannerJSON(url, method, payload) {
        return api(url, {
            method: method || 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload || {})
        });
    }

    async function renderTodo(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.todoFilter = host.dataset.todoFilter || 'all';
        host.innerHTML = `<div class="vd-todo"><aside class="vd-todo-sidebar">
            ${['all', 'open', 'in_progress', 'done'].map(status => `<button type="button" data-filter="${status}" class="${host.dataset.todoFilter === status ? 'active' : ''}">${esc(t('desktop.todo_' + status))}</button>`).join('')}
        </aside><main class="vd-todo-main"><form class="vd-todo-add"><input placeholder="${esc(t('desktop.todo_title_placeholder'))}"><select><option value="low">${esc(t('desktop.todo_priority_low'))}</option><option value="medium" selected>${esc(t('desktop.todo_priority_medium'))}</option><option value="high">${esc(t('desktop.todo_priority_high'))}</option></select><button class="vd-button vd-button-primary">${esc(t('desktop.todo_add'))}</button></form><div class="vd-todo-list">${esc(t('desktop.loading'))}</div></main><section class="vd-todo-detail"><div class="vd-empty">${esc(t('desktop.todo_select_task'))}</div></section></div>`;
        const showTodoContextMenu = (event, todo, reload) => {
            event.preventDefault();
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.context_open', icon: 'folder-open', action: () => renderTodoDetail(host, todo, reload) },
                { labelKey: 'desktop.todo_complete', icon: 'check-square', disabled: todo.status === 'done', action: async () => { await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id); } },
                { separator: true },
                { labelKey: 'desktop.delete', icon: 'trash', action: async () => { if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await api('/api/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); } } }
            ]);
            return true;
        };
        wireContextMenuBoundary(host);
        const load = async (selectedID) => {
            const todos = await api('/api/todos?status=all');
            const filtered = todos.filter(todo => host.dataset.todoFilter === 'all' || todo.status === host.dataset.todoFilter)
                .sort((a, b) => (({ high: 0, medium: 1, low: 2 }[a.priority] ?? 3) - (({ high: 0, medium: 1, low: 2 }[b.priority] ?? 3)) || String(a.due_date || '9999').localeCompare(String(b.due_date || '9999'))));
            const list = host.querySelector('.vd-todo-list');
            list.innerHTML = filtered.length ? filtered.map(todo => renderTodoCard(todo, selectedID)).join('') : `<div class="vd-empty">${esc(t('desktop.empty_folder'))}</div>`;
            list.querySelectorAll('[data-todo-id]').forEach(card => card.addEventListener('click', () => renderTodoDetail(host, todos.find(todo => todo.id === card.dataset.todoId), load)));
            list.querySelectorAll('[data-todo-status-toggle]').forEach(input => input.addEventListener('click', event => event.stopPropagation()));
            list.querySelectorAll('[data-todo-status-toggle]').forEach(input => input.addEventListener('change', async event => {
                event.stopPropagation();
                const todo = todos.find(item => item.id === input.dataset.todoStatusToggle);
                if (todo) await setTodoDone(todo, input.checked, load);
            }));
            list.querySelectorAll('[data-todo-id]').forEach(card => card.addEventListener('contextmenu', event => {
                const todo = todos.find(item => item.id === card.dataset.todoId);
                if (todo) showTodoContextMenu(event, todo, load);
            }));
            const selected = todos.find(todo => todo.id === selectedID) || filtered[0];
            if (selected) renderTodoDetail(host, selected, load);
        };
        host.querySelectorAll('[data-filter]').forEach(btn => btn.addEventListener('click', () => {
            host.dataset.todoFilter = btn.dataset.filter;
            renderTodo(id);
        }));
        host.querySelector('.vd-todo-add').addEventListener('submit', async event => {
            event.preventDefault();
            const input = event.currentTarget.querySelector('input');
            const title = input.value.trim();
            if (!title) return;
            const result = await plannerJSON('/api/todos', 'POST', { title, priority: event.currentTarget.querySelector('select').value, status: 'open' });
            input.value = '';
            await load(result.id);
        });
        try { await load(); } catch (err) { host.querySelector('.vd-todo-list').innerHTML = `<div class="vd-empty">${esc(t('desktop.load_failed'))}</div>`; }
    }

    function renderTodoCard(todo, selectedID) {
        const due = todo.due_date ? new Date(todo.due_date) : null;
        const overdue = due && due < new Date() && todo.status !== 'done';
        return `<article class="vd-todo-card ${todo.id === selectedID ? 'active' : ''} ${overdue ? 'overdue' : ''}" data-todo-id="${esc(todo.id)}">
            <div class="vd-todo-card-grid">
                <input class="vd-todo-card-done" type="checkbox" data-todo-status-toggle="${esc(todo.id)}" ${todo.status === 'done' ? 'checked' : ''} aria-label="${esc(t('desktop.todo_complete'))}">
                <div class="vd-todo-card-copy">
                    <strong>${esc(todo.title)}</strong>
                    <small>${esc(t('desktop.todo_' + todo.status))}${due ? ' &middot; ' + esc(due.toLocaleDateString()) : ''}${overdue ? ' &middot; ' + esc(t('desktop.todo_overdue')) : ''}</small>
                </div>
                <span class="vd-todo-priority ${esc(todo.priority)}">${esc(t('desktop.todo_priority_' + todo.priority))}</span>
            </div>
            <div class="vd-todo-progress"><span style="width:${Number(todo.progress_percent) || 0}%"></span></div>
        </article>`;
    }

    async function setTodoDone(todo, done, reload) {
        if (!todo) return;
        if (done) {
            await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true });
            await reload(todo.id);
            return;
        }
        const payload = { status: 'open' };
        if (Array.isArray(todo.items) && todo.items.length) {
            payload.items = todo.items.map(item => Object.assign({}, item, { is_done: false }));
        }
        await plannerJSON('/api/todos/' + encodeURIComponent(todo.id), 'PUT', payload);
        await reload(todo.id);
    }

    async function updateTodoItem(todo, itemID, patch, reload) {
        if (!todo || !itemID) return;
        await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/items/' + encodeURIComponent(itemID), 'PUT', patch);
        await reload(todo.id);
    }

    function renderTodoDetail(host, todo, reload) {
        const pane = host.querySelector('.vd-todo-detail');
        const items = todo.items || [];
        pane.innerHTML = `<form class="vd-todo-form"><input name="title" value="${esc(todo.title)}"><textarea name="description" placeholder="${esc(t('desktop.todo_description'))}">${esc(todo.description || '')}</textarea><div class="vd-todo-form-row"><label>${esc(t('desktop.todo_priority'))}<select name="priority">${['low','medium','high'].map(p => `<option value="${p}" ${todo.priority === p ? 'selected' : ''}>${esc(t('desktop.todo_priority_' + p))}</option>`).join('')}</select></label><label>${esc(t('desktop.todo_due_date'))}<input type="date" name="due_date" value="${esc(todo.due_date || '')}"></label></div><label class="vd-check"><input type="checkbox" name="remind_daily" ${todo.remind_daily ? 'checked' : ''}>${esc(t('desktop.todo_remind_daily'))}</label><div class="vd-todo-actions"><button class="vd-button vd-button-primary" data-action="save">${esc(t('desktop.save'))}</button><button type="button" class="vd-button" data-action="complete">${esc(t('desktop.todo_complete'))}</button><button type="button" class="vd-button" data-action="delete">${esc(t('desktop.delete'))}</button></div></form><h3>${esc(t('desktop.todo_items'))}</h3><form class="vd-todo-item-add"><input placeholder="${esc(t('desktop.todo_add_item'))}"><button class="vd-button">${esc(t('desktop.todo_add_item'))}</button></form><div class="vd-todo-items">${items.map(item => `<div class="vd-todo-item" data-item-id="${esc(item.id)}"><input class="vd-todo-item-check" type="checkbox" data-item-toggle="${esc(item.id)}" ${item.is_done ? 'checked' : ''} aria-label="${esc(t('desktop.todo_complete'))}"><input class="vd-todo-item-title" data-item-title="${esc(item.id)}" value="${esc(item.title)}" aria-label="${esc(t('desktop.todo_add_item'))}" spellcheck="true"><button type="button" class="vd-todo-item-delete" data-item-delete="${esc(item.id)}" title="${esc(t('desktop.delete'))}">${iconMarkup('x', 'X', 'vd-todo-action-icon', 13)}</button></div>`).join('')}</div>`;
        pane.querySelector('.vd-todo-form').addEventListener('submit', async event => {
            event.preventDefault();
            const form = event.currentTarget;
            await plannerJSON('/api/todos/' + encodeURIComponent(todo.id), 'PUT', { title: form.title.value.trim(), description: form.description.value, priority: form.priority.value, due_date: form.due_date.value, remind_daily: form.remind_daily.checked });
            await reload(todo.id);
        });
        pane.querySelector('[data-action="complete"]').addEventListener('click', async () => { await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id); });
        pane.querySelector('[data-action="delete"]').addEventListener('click', async () => { if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await api('/api/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); } });
        pane.querySelector('.vd-todo-item-add').addEventListener('submit', async event => { event.preventDefault(); const input = event.currentTarget.querySelector('input'); if (!input.value.trim()) return; await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/items', 'POST', { title: input.value.trim() }); await reload(todo.id); });
        pane.querySelectorAll('[data-item-toggle]').forEach(input => input.addEventListener('change', async () => { await updateTodoItem(todo, input.dataset.itemToggle, { is_done: input.checked }, reload); }));
        pane.querySelectorAll('[data-item-title]').forEach(titleInput => {
            titleInput.addEventListener('keydown', async event => {
                if (event.key === 'Enter') {
                    event.preventDefault();
                    titleInput.blur();
                } else if (event.key === 'Escape') {
                    event.preventDefault();
                    const item = items.find(entry => entry.id === titleInput.dataset.itemTitle);
                    titleInput.value = item ? item.title : '';
                    titleInput.blur();
                }
            });
            titleInput.addEventListener('change', async () => {
                const title = titleInput.value.trim();
                const item = items.find(entry => entry.id === titleInput.dataset.itemTitle);
                if (!title || !item || title === item.title) {
                    titleInput.value = item ? item.title : '';
                    return;
                }
                await updateTodoItem(todo, titleInput.dataset.itemTitle, { title: titleInput.value.trim() }, reload);
            });
        });
        pane.querySelectorAll('[data-item-delete]').forEach(btn => btn.addEventListener('click', async () => { await api('/api/todos/' + encodeURIComponent(todo.id) + '/items/' + encodeURIComponent(btn.dataset.itemDelete), { method: 'DELETE' }); await reload(todo.id); }));
        setTodoMenus(host, todo, reload);
    }

    function setTodoMenus(host, todo, reload) {
        const win = host && host.closest && host.closest('.vd-window');
        const id = win && win.dataset.windowId;
        if (!id || !todo) return;
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'save', labelKey: 'desktop.save', icon: 'save', shortcut: 'Ctrl+S', action: () => {
                        const form = host.querySelector('.vd-todo-form');
                        if (form) form.requestSubmit();
                    } }
                ]
            },
            {
                id: 'edit',
                labelKey: 'desktop.menu_edit',
                items: [
                    { id: 'complete', labelKey: 'desktop.todo_complete', icon: 'check-square', action: async () => { await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id); } },
                    { id: 'delete', labelKey: 'desktop.delete', icon: 'trash', action: async () => { if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await api('/api/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); } } }
                ]
            }
        ]);
    }

    function galleryAppContext(context) {
        return Object.assign({}, context || {}, {
            t,
            esc,
            api,
            iconMarkup,
            fmtBytes,
            notify: showDesktopNotification,
            mediaPreviewURL,
            mediaDownloadURL,
            mediaPreviewKind,
            readonly: desktopReadonly(),
            pageSize: GALLERY_PAGE_SIZE,
            animationsEnabled,
            wireContextMenuBoundary,
            setWindowMenus,
            clearWindowMenus,
            showContextMenu,
            registerWindowCleanup,
            confirmDialog,
            promptDialog,
            settingBool,
            desktopSound,
            openApp,
            downloadMediaPath,
            openMediaPreview,
            afterFileChange: refreshDesktopAfterFileChange
        });
    }

    async function renderGallery(id, context) {
        const host = contentEl(id);
        if (!host) return;
        const app = window.GalleryApp;
        if (!app || typeof app.render !== 'function') {
            host.innerHTML = `<div class="vd-empty">${esc(t('desktop.load_failed'))}</div>`;
            return;
        }
        const ctx = galleryAppContext(context);
        if (!ctx.tab && host.dataset.galleryTab) ctx.tab = host.dataset.galleryTab;
        return app.render(host, id, ctx);
    }

    async function renameMediaFile(file) {
        if (!file || !file.path) return null;
        const current = String(file.path).split('/').pop();
        const name = await promptDialog(t('desktop.rename'), current);
        if (!name || name === current) return null;
        const newPath = workspaceJoinPath(pathDir(file.path), name);
        try {
            await api('/api/desktop/file', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ old_path: file.path, new_path: newPath })
            });
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err && err.message ? err.message : t('desktop.load_failed') });
            return null;
        }
        refreshDesktopAfterFileChange();
        const updated = Object.assign({}, file, { path: newPath, name });
        if (file.web_path) updated.web_path = String(file.web_path).replace(/[^/]+$/, encodeURIComponent(name));
        return updated;
    }

    async function deleteMediaFile(file) {
        if (!file || !file.path) return false;
        if (settingBool('files.confirm_delete')) {
            const confirmed = await confirmDialog(t('desktop.gallery_delete_one_title', { name: file.name || file.path }), t('desktop.gallery_delete_msg'));
            if (!confirmed) return false;
        }
        try {
            await api('/api/desktop/file?path=' + encodeURIComponent(file.path), { method: 'DELETE' });
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err && err.message ? err.message : t('desktop.load_failed') });
            return false;
        }
        desktopSound('file.delete');
        refreshDesktopAfterFileChange();
        return true;
    }

    function openMediaLightbox(file) {
        const modules = window.AuraDesktopModules;
        const ready = window.GalleryLightbox && typeof window.GalleryLightbox.open === 'function';
        const load = ready || !modules || typeof modules.loadAppAssets !== 'function'
            ? Promise.resolve()
            : modules.loadAppAssets('gallery');
        return load.then(() => {
            const lightbox = window.GalleryLightbox;
            if (!lightbox || typeof lightbox.open !== 'function') return false;
            const helpers = window.GalleryApp || {};
            const readonly = desktopReadonly();
            lightbox.open({
                items: [file],
                index: 0,
                context: {
                    t,
                    esc,
                    iconMarkup,
                    fmtBytes,
                    mediaPreviewURL,
                    mediaDownloadURL,
                    mediaPreviewKind,
                    readonly,
                    animationsEnabled: animationsEnabled(),
                    formatDateTime: helpers.formatDateTime,
                    formatDuration: helpers.formatDuration,
                    kindLabel: kind => t(kind === 'video' ? 'desktop.gallery_kind_video' : kind === 'audio' ? 'desktop.gallery_kind_audio' : 'desktop.gallery_kind_image')
                },
                actions: {
                    download: item => downloadMediaPath(mediaDownloadURL(item), item && item.name),
                    rename: readonly ? null : renameMediaFile,
                    remove: readonly ? null : deleteMediaFile,
                    edit: item => { if (item && item.path) openApp('pixel', { path: item.path }); },
                    reveal: item => { if (item && item.path) openApp('files', { path: pathDir(item.path) }); }
                }
            });
            return true;
        });
    }


    function webampHostNode() {
        const parent = $('vd-window-layer') || document.body;
        let host = $('vd-webamp-host');
        if (!host || host.parentElement !== parent) {
            if (host) host.remove();
            host = document.createElement('div');
            host.id = 'vd-webamp-host';
            host.className = 'vd-webamp-host';
            parent.appendChild(host);
        }
        return host;
    }

    function disposeWebampMusic(windowId, options) {
        const current = state.webampMusic;
        if (!current) return;
        if (windowId && current.windowId !== windowId) return;
        if (current.unsubscribeClose) {
            try { current.unsubscribeClose(); } catch (_) {}
        }
        if (!options || !options.fromWebampClose) {
            if (current.instance && typeof current.instance.dispose === 'function') {
                try { current.instance.dispose(); } catch (_) {}
            }
        }
        state.webampMusic = null;
        notifyWebampMediaSessionStopped();
        const host = $('vd-webamp-host');
        if (host) host.remove();
    }

    async function loadWebampConstructor() {
        const mod = await import(WEBAMP_MODULE_PATH);
        return mod.default || mod.Webamp || mod;
    }

    function webampTrackTitle(name) {
        return String(name || '').replace(/\.[^.]+$/, '') || String(name || '');
    }

    async function scanWebampTracks(folder) {
        const params = new URLSearchParams({ path: folder, recursive: 'true', limit: String(WEBAMP_TRACK_SCAN_LIMIT) });
        const body = await api('/api/desktop/files?' + params.toString());
        const files = body.files || [];
        const tracks = [];
        for (const file of files) {
            if (file.type === 'file' && WEBAMP_AUDIO_PATTERN.test(file.name)) {
                tracks.push({
                    url: file.web_path || await desktopEmbedURL(file.path),
                    metaData: { title: webampTrackTitle(file.name) }
                });
            }
        }
        return tracks;
    }

    async function ensureWebampMusic(tracks, ownerWindowId, callbacks) {
        const hooks = callbacks || {};
        if (!tracks.length) {
            disposeWebampMusic(ownerWindowId);
            if (typeof hooks.setStatus === 'function') hooks.setStatus(t('desktop.winamp_no_tracks'));
            if (hooks.notifyEmpty !== false) showDesktopNotification({ title: t('desktop.notification'), message: t('desktop.winamp_no_tracks') });
            return false;
        }

        const Webamp = await loadWebampConstructor();
        if (typeof Webamp.browserIsSupported === 'function' && !Webamp.browserIsSupported()) {
            throw new Error(t('desktop.winamp_unsupported'));
        }

        const current = state.webampMusic;
        if (current && current.instance && (!ownerWindowId || current.windowId === ownerWindowId)) {
            if (typeof current.instance.reopen === 'function') current.instance.reopen();
            if (typeof current.instance.setTracksToPlay === 'function') {
                current.instance.setTracksToPlay(tracks);
                if (typeof hooks.setStatus === 'function') hooks.setStatus(t('desktop.done'));
                notifyWebampMediaSessionChanged();
                return true;
            }
            disposeWebampMusic(ownerWindowId || current.windowId);
        } else if (current && current.instance) {
            disposeWebampMusic(current.windowId);
        }

        const webamp = new Webamp({ initialTracks: tracks });
        state.webampMusic = { instance: webamp, windowId: ownerWindowId || '', unsubscribeClose: null };
        if (typeof webamp.onClose === 'function') {
            state.webampMusic.unsubscribeClose = webamp.onClose(() => {
                disposeWebampMusic(ownerWindowId || '', { fromWebampClose: true });
                if (ownerWindowId) closeWindow(ownerWindowId);
            });
        }
        await webamp.renderWhenReady(webampHostNode());
        if (typeof hooks.setStatus === 'function') hooks.setStatus(t('desktop.done'));
        notifyWebampMediaSessionChanged();
        return true;
    }

    async function launchStandaloneWebamp(context) {
        const folder = normalizeDesktopPath((context && context.path) || 'Music') || 'Music';
        const tracks = await scanWebampTracks(folder);
        await ensureWebampMusic(tracks, '', { notifyEmpty: true });
    }

    async function renderMusicPlayer(id) {
        const host = contentEl(id);
        if (!host) return;
        const win = state.windows.get(id);
        if (win && win.element) {
            win.element.style.minWidth = '380px';
            win.element.style.minHeight = '220px';
        }

        let currentFolder = 'Music';
        let currentTracks = [];

        host.innerHTML = `<div class="vd-webamp-launcher">
            <div class="vd-webamp-launcher-header">
                ${iconMarkup('audio-player', 'MP', 'vd-sprite-start-item', 34)}
                <div class="vd-webamp-launcher-copy">
                    <strong>${esc(t('desktop.app_music_player'))}</strong>
                    <span data-status>${esc(t('desktop.loading'))}</span>
                </div>
            </div>
            <div class="vd-webamp-status">
                <span data-track-count>0 ${esc(t('desktop.winamp_tracks'))}</span>
                <span data-folder>Music</span>
            </div>
        </div>`;

        const statusEl = host.querySelector('[data-status]');
        const countEl = host.querySelector('[data-track-count]');
        const folderEl = host.querySelector('[data-folder]');

        const setStatus = message => {
            if (statusEl) statusEl.textContent = message;
        };

        const renderLauncherState = () => {
            if (countEl) countEl.textContent = currentTracks.length + ' ' + t('desktop.winamp_tracks');
            if (folderEl) folderEl.textContent = currentFolder;
        };

        const notifyError = err => {
            const raw = err && err.message ? String(err.message) : String(err || '');
            const unsupported = t('desktop.winamp_unsupported');
            const message = (raw === 'Webamp is not supported in this browser.' || raw === unsupported)
                ? unsupported
                : t('desktop.load_failed');
            setStatus(message);
            showDesktopNotification({ title: t('desktop.notification'), message });
        };

        const loadMusicLibrary = async folder => {
            currentFolder = folder || 'Music';
            setStatus(t('desktop.loading'));
            currentTracks = await scanWebampTracks(currentFolder);
            renderLauncherState();
            await ensureWebampMusic(currentTracks, id, { setStatus, notifyEmpty: false });
        };

        const showMusicPlayerContextMenu = event => {
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.menu_load_folder', icon: 'folder-open', action: async () => {
                    const folder = await promptDialog(t('desktop.winamp_load_folder'), currentFolder || 'Music');
                    if (folder != null) loadMusicLibrary(folder).catch(notifyError);
                } },
                { labelKey: 'desktop.context_refresh', icon: 'refresh', action: () => loadMusicLibrary('Music').catch(notifyError) },
                { separator: true },
                { labelKey: 'desktop.menu_reopen_player', icon: 'audio-player', action: () => {
                    const current = state.webampMusic;
                    if (current && current.instance && typeof current.instance.reopen === 'function') current.instance.reopen();
                    else loadMusicLibrary(currentFolder || 'Music').catch(notifyError);
                } }
            ]);
            return true;
        };
        wireContextMenuBoundary(host, { onContextMenu: showMusicPlayerContextMenu });

        setMusicPlayerMenus(id, host, {
            refresh: () => loadMusicLibrary('Music').catch(notifyError),
            loadFolder: async () => {
                const folder = await promptDialog(t('desktop.winamp_load_folder'), currentFolder || 'Music');
                if (folder == null) return;
                loadMusicLibrary(folder).catch(notifyError);
            },
            reopen: () => {
                const current = state.webampMusic;
                if (current && current.instance && typeof current.instance.reopen === 'function') {
                    current.instance.reopen();
                    return;
                }
                loadMusicLibrary(currentFolder || 'Music').catch(notifyError);
            }
        });
        renderLauncherState();
        loadMusicLibrary('Music').catch(notifyError);
    }

    function setMusicPlayerMenus(id, host, actions) {
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'load-folder', labelKey: 'desktop.menu_load_folder', icon: 'folder-open', action: actions.loadFolder },
                    { id: 'refresh-music', labelKey: 'desktop.context_refresh', icon: 'refresh', shortcut: 'F5', action: actions.refresh }
                ]
            },
            {
                id: 'playback',
                labelKey: 'desktop.menu_playback',
                items: [
                    { id: 'reopen-webamp', labelKey: 'desktop.menu_reopen_player', icon: 'audio-player', action: actions.reopen }
                ]
            }
        ]);
    }

    function renderQuickConnect(id) {
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-quick-connect">
            <div class="vd-qc-sidebar">
                <div class="vd-qc-sidebar-header">
                    <span class="vd-qc-title">${esc(t('desktop.qc_title'))}</span>
                </div>
                <div class="vd-qc-search">
                    <input type="search" autocomplete="off" spellcheck="false" data-i18n-placeholder="desktop.qc_search_placeholder">
                </div>
                <div class="vd-qc-filters" role="group" aria-label="${esc(t('desktop.qc_filter_protocol'))}">
                    <button class="vd-qc-filter active" type="button" data-qc-filter="all">${iconMarkup('server', 'A', 'vd-qc-filter-icon', 13)}<span>${esc(t('desktop.qc_filter_all'))}</span></button>
                    <button class="vd-qc-filter" type="button" data-qc-filter="ssh">${iconMarkup('terminal', 'T', 'vd-qc-filter-icon', 13)}<span>${esc(t('desktop.qc_protocol_ssh'))}</span></button>
                    <button class="vd-qc-filter" type="button" data-qc-filter="vnc">${iconMarkup('monitor', 'V', 'vd-qc-filter-icon', 13)}<span>${esc(t('desktop.qc_protocol_vnc'))}</span></button>
                </div>
                <div class="vd-qc-device-list" data-device-list>${esc(t('desktop.loading'))}</div>
            </div>
            <div class="vd-qc-terminal-area" data-terminal-area>
                <div class="vd-qc-tabs" data-qc-tabs hidden>
                    <button class="vd-qc-tab active" data-tab="terminal">${iconMarkup('terminal', 'T', 'vd-qc-tab-icon', 14)}<span>${esc(t('desktop.qc_tab_terminal'))}</span></button>
                    <button class="vd-qc-tab" data-tab="files">${iconMarkup('folder', 'F', 'vd-qc-tab-icon', 14)}<span>${esc(t('desktop.qc_tab_files'))}</span></button>
                </div>
                <div class="vd-qc-tab-content" data-tab-content>
                    <div class="vd-qc-placeholder">
                        <span class="vd-qc-placeholder-icon">${iconMarkup('server', 'S', 'vd-qc-placeholder-papirus-icon', 42)}</span>
                        <span class="vd-qc-placeholder-text">${esc(t('desktop.qc_select_device'))}</span>
                    </div>
                </div>
            </div>
        </div>`;
        wireContextMenuBoundary(host);

        const searchInput = host.querySelector('.vd-qc-search input');
        searchInput.placeholder = t('desktop.qc_search_placeholder');
        const deviceList = host.querySelector('[data-device-list]');
        const terminalArea = host.querySelector('[data-terminal-area]');
        const tabContent = host.querySelector('[data-tab-content]');
        const qcTabs = host.querySelector('[data-qc-tabs]');
        const filterButtons = Array.from(host.querySelectorAll('[data-qc-filter]'));
        let activeWS = null;
        let activeTerm = null;
        let activeFitAddon = null;
        let activeResizeObserver = null;
        let cachedDevices = null;
        let cachedCredentials = null;
        let activeTab = 'terminal';
        let activeProtocolFilter = 'all';
        let connectedDeviceId = null;
        let connectedProtocol = null;

        function switchTab(tab) {
            activeTab = tab;
            qcTabs.querySelectorAll('.vd-qc-tab').forEach(btn => btn.classList.toggle('active', btn.dataset.tab === tab));
            if (tab === 'files' && connectedDeviceId && connectedProtocol === 'ssh') {
                openSFTPPanel(connectedDeviceId, tabContent);
            } else if (tab === 'terminal') {
                closeSFTPPanel(tabContent);
            }
        }

        qcTabs.querySelectorAll('.vd-qc-tab').forEach(btn => {
            btn.addEventListener('click', () => switchTab(btn.dataset.tab));
        });

        registerWindowCleanup(id, () => {
            if (activeWS) { try { activeWS.close(); } catch(_) {} activeWS = null; }
            if (activeTerm) { activeTerm.dispose(); activeTerm = null; }
            if (activeResizeObserver) { activeResizeObserver.disconnect(); activeResizeObserver = null; }
            host.querySelectorAll('.vd-qc-modal-overlay, .vd-qc-notify').forEach(el => el.remove());
        });

        setQuickConnectMenus(id, host, loadAll, showServerModal, () => switchTab('files'));
        loadAll();

        searchInput.addEventListener('input', () => filterDevices());
        filterButtons.forEach(btn => {
            btn.addEventListener('click', () => {
                activeProtocolFilter = btn.dataset.qcFilter || 'all';
                filterButtons.forEach(item => item.classList.toggle('active', item === btn));
                filterDevices();
            });
        });

        async function loadAll() {
            deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.loading'))}</div>`;
            try {
                const [devBody, credBody] = await Promise.all([
                    api('/api/devices'),
                    api('/api/credentials')
                ]);
                cachedDevices = withAuraGoHostDevice((devBody.devices || devBody || []).filter(d => d.protocol === 'vnc' || d.type === 'server' || d.type === 'generic' || d.type === 'linux' || d.type === 'vm' || !d.type));
                cachedCredentials = credBody || [];
                if (!cachedDevices.length) {
                    deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.qc_no_devices'))}</div>`;
                    return;
                }
                renderDeviceList(cachedDevices);
            } catch (err) {
                deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.load_failed'))}</div>`;
            }
        }

        function auraGoHostName() {
            return (location.hostname || location.host || 'localhost').replace(/^\[|\]$/g, '');
        }

        function withAuraGoHostDevice(devices) {
            const hostName = auraGoHostName();
            const normalizedHost = hostName.toLowerCase();
            const hasAuraHost = devices.some(device => {
                const address = String(device.ip_address || '').toLowerCase();
                const name = String(device.name || '').toLowerCase();
                const id = String(device.id || '');
                return id === '__aurago-host__' || address === normalizedHost || name === 'aurago host';
            });
            if (hasAuraHost) return devices;
            return [{
                id: '__aurago-host__',
                name: t('desktop.qc_aurago_host'),
                type: 'server',
                protocol: 'ssh',
                ip_address: hostName,
                port: 22,
                description: t('desktop.qc_aurago_host_description'),
                is_template: true
            }, ...devices];
        }

        function renderDeviceList(devices) {
            const query = searchInput.value.trim().toLowerCase();
            const filtered = devices.filter(d => protocolMatchesFilter(d) && (!query ||
                (d.name || '').toLowerCase().includes(query) ||
                (d.ip_address || '').toLowerCase().includes(query) ||
                (d.description || '').toLowerCase().includes(query)
            ));
            if (!filtered.length) {
                deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.qc_no_devices'))}</div>`;
                return;
            }
            deviceList.innerHTML = filtered.map(d => {
                const cred = d.credential_id && cachedCredentials ? cachedCredentials.find(c => c.id === d.credential_id) : null;
                const endpoint = `${d.ip_address || ''}${d.port && d.port !== 22 && d.port !== 5900 ? ':' + d.port : ''}`;
                const protoLabel = d.protocol === 'vnc' ? 'VNC' : 'SSH';
                const protoIcon = d.protocol === 'vnc' ? 'monitor' : 'terminal';
                return `<button class="vd-qc-device${d.is_template ? ' template' : ''}" type="button" data-device-id="${esc(d.id)}">
                    <span class="vd-qc-device-icon">${iconMarkup(d.is_template ? 'home' : protoIcon, 'S', 'vd-qc-device-papirus-icon', 22)}</span>
                    <div class="vd-qc-device-main">
                        <div class="vd-qc-device-name">${esc(d.name)}</div>
                        <div class="vd-qc-device-meta">${esc(endpoint || '')}</div>
                        ${d.description ? `<div class="vd-qc-device-desc">${esc(d.description)}</div>` : ''}
                    </div>
                    <div class="vd-qc-device-badges">
                        ${d.is_template ? '<span class="vd-qc-badge vd-qc-badge-info">' + esc(t('desktop.qc_badge_setup')) + '</span>' : (d.credential_id ? '<span class="vd-qc-badge vd-qc-badge-ok">' + esc(protoLabel) + '</span>' : '<span class="vd-qc-badge vd-qc-badge-warn">?</span>')}
                    </div>
                </button>`;
            }).join('');
            deviceList.querySelectorAll('.vd-qc-device').forEach(btn => {
                btn.addEventListener('click', () => {
                    const dev = cachedDevices.find(d => d.id === btn.dataset.deviceId);
                    if (dev && dev.is_template) showServerModal(dev);
                    else connectToDevice(btn.dataset.deviceId);
                });
                btn.addEventListener('contextmenu', (e) => {
                    e.preventDefault();
                    const dev = cachedDevices.find(d => d.id === btn.dataset.deviceId);
                    if (dev) showDeviceContextMenu(e.clientX, e.clientY, dev);
                });
            });
        }

        function protocolMatchesFilter(device) {
            if (activeProtocolFilter === 'all') return true;
            return String(device && device.protocol || 'ssh').toLowerCase() === activeProtocolFilter;
        }

        function filterDevices() {
            if (cachedDevices) renderDeviceList(cachedDevices);
        }

        function showDeviceContextMenu(x, y, device) {
            closeContextMenu();
            const items = [
                { label: device.is_template ? t('desktop.qc_add_server') : t('desktop.qc_connect'), icon: device.is_template ? 'server' : (device.protocol === 'vnc' ? 'monitor' : 'terminal'), fallback: 'T', action: () => device.is_template ? showServerModal(device) : connectToDevice(device.id) },
                { label: t('desktop.qc_edit'), icon: 'edit', fallback: 'E', action: () => showServerModal(device) }
            ];
            if (!device.is_template) {
                items.push({ separator: true }, { label: t('desktop.qc_delete'), icon: 'trash', fallback: 'X', action: () => confirmDeleteDevice(device) });
            }
            showContextMenu(x, y, items);
        }

        async function confirmDeleteDevice(device) {
            const ok = await showConfirmModal(t('desktop.qc_delete_confirm'), t('desktop.qc_delete_confirm_msg').replace('{{name}}', device.name));
            if (!ok) return;
            try {
                await api('/api/devices/' + device.id, { method: 'DELETE' });
                await loadAll();
            } catch (err) {
                showNotify(t('desktop.qc_delete_error') + ': ' + err.message);
            }
