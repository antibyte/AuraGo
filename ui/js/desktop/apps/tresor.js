(function () {
    'use strict';

    const instances = new Map();
    const base = '/api/desktop/tresor';
    const maxFile = 50 * 1024 * 1024;
    const encoder = new TextEncoder();
    const decoder = new TextDecoder();
    let cryptoModule;

    async function crypt() {
        if (!cryptoModule) cryptoModule = import('/js/desktop/apps/tresor-crypto.js');
        return cryptoModule;
    }

    async function request(path, method = 'GET', body, revision) {
        const headers = {};
        if (body !== undefined) headers['Content-Type'] = 'application/json';
        if (revision) headers['If-Match'] = `"${revision}"`;
        const response = await fetch(base + path, {
            method, credentials: 'same-origin', cache: 'no-store',
            headers, body: body === undefined ? undefined : JSON.stringify(body)
        });
        if (!response.ok) {
            const error = new Error(response.status === 412 ? 'conflict' : response.status === 403 ? 'https_required' : response.status === 401 ? 'admin_required' : 'request_failed');
            error.status = response.status;
            throw error;
        }
        return response.status === 204 ? null : response.json();
    }

    function render(host, windowId, context = {}) {
        if (!host) return;
        dispose(windowId);
        const events = new AbortController();
        const listen = (name, handler, options = {}) => host.addEventListener(name, handler, { ...options, signal: events.signal });
        const common = { cancel: 'desktop.cancel', delete: 'desktop.delete', file: 'desktop.file_dialog_file', save: 'desktop.save', search: 'desktop.search', new_note: 'desktop.notes_new', loading: 'desktop.loading', export: 'desktop.file_dialog_export' };
        const tr = (key, fallback) => context.t ? context.t(common[key] || 'tresor.' + key, fallback) : fallback;
        const esc = context.esc || (value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])));
        const state = { host, phase: 'loading', header: null, master: null, items: [], selected: null, noteTitle: '', noteText: '', dirty: false, search: '', pending: null, busy: false, timer: null, saveTimer: null, savePromise: null, draftPromise: Promise.resolve(), editVersion: 0, draftReadyVersion: 0, message: '', error: false, modal: '', disposed: false, epoch: 0 };
        instances.set(windowId, state);
        const label = key => esc(tr(key, key));
        const button = (action, key, className = '') => `<button type="button" class="${className}" data-action="${action}">${label(key)}</button>`;
        function taskGuard() {
            const epoch = state.epoch;
            return () => !state.disposed && state.epoch === epoch;
        }

        function announce(message, error = false) {
            if (state.disposed) return;
            state.message = message;
            state.error = error;
            host.querySelectorAll('[data-status]').forEach(line => {
                line.textContent = message;
                line.dataset.error = error ? 'true' : 'false';
            });
        }

        function forget() {
            state.epoch++;
            if (state.timer) clearTimeout(state.timer);
            if (state.saveTimer) clearTimeout(state.saveTimer);
            if (state.master) state.master.fill(0);
            state.master = null;
            state.items = [];
            state.selected = null;
            state.noteTitle = '';
            state.noteText = '';
            state.search = '';
            state.dirty = false;
            state.pending = null;
            state.modal = '';
            state.busy = false;
            state.savePromise = null;
            state.message = '';
            state.error = false;
        }
        state.forget = forget;

        function lock() {
            forget();
            if (!state.disposed) { state.phase = state.header ? 'locked' : 'setup'; draw(); }
        }

        function activity() {
            if (state.phase !== 'open') return;
            if (state.timer) clearTimeout(state.timer);
            state.timer = setTimeout(lock, 5 * 60 * 1000);
        }

        function door() {
            const setup = state.phase === 'setup';
            const confirm = state.phase === 'confirm';
            return `<div class="tresor-entrance">
                <div class="tresor-art"><img src="/img/tresor/vault-door.svg" alt="" draggable="false"><span class="tresor-art-glow"></span></div>
                <div class="tresor-entry-panel"><p class="tresor-kicker">AURAGO · ${label('private')}</p><h1>${label('title')}</h1>
                <p class="tresor-intro">${label(setup ? 'setup_intro' : confirm ? 'confirm_intro' : 'unlock_intro')}</p>
                ${state.phase === 'insecure' ? `<p class="tresor-warning">${label('https_help')}</p>` : ''}
                ${setup ? `<form data-form="setup"><label>${label('password')}<input name="password" type="password" minlength="16" required autocomplete="new-password"></label><label>${label('password_confirm')}<input name="repeat" type="password" minlength="16" required autocomplete="new-password"></label><button class="tresor-primary" type="submit">${label('create')}</button></form>` : ''}
                ${confirm ? `<div class="tresor-recovery"><span>${label('recovery_key')}</span><output>${esc(state.pending.recoveryHex.match(/.{1,4}/g).join(' '))}</output><p>${label('recovery_warning')}</p></div><form data-form="confirm"><label>${label('recovery_confirm')}<input name="recovery" type="text" required spellcheck="false" autocomplete="off"></label><button class="tresor-primary" type="submit">${label('confirm_setup')}</button></form>` : ''}
                ${state.phase === 'locked' ? `<form data-form="unlock"><label>${label('password')}<input name="password" type="password" required autocomplete="current-password"></label><button class="tresor-primary" type="submit">${label('unlock')}</button></form><details><summary>${label('use_recovery')}</summary><form data-form="recover"><label>${label('recovery_key')}<input name="recovery" type="text" required spellcheck="false" autocomplete="off"></label><button type="submit">${label('unlock_recovery')}</button></form></details>` : ''}
                ${state.phase === 'loading' ? `<p>${label('loading')}</p>` : ''}
                <p class="tresor-security-note">${label('security_limit')}</p><p class="tresor-status" role="status" aria-live="polite" data-status data-error="${state.error}">${esc(state.message)}</p>
                </div></div>`;
        }

        function listRows() {
            const query = state.search.toLocaleLowerCase();
            const visible = state.items.filter(item => (item.title || '').toLocaleLowerCase().includes(query));
            if (!visible.length) return `<p class="tresor-empty">${label(state.items.length ? 'no_results' : 'empty')}</p>`;
            return visible.map(item => `<button type="button" class="tresor-item ${state.selected?.id === item.id ? 'is-selected' : ''}" data-action="select" data-id="${esc(item.id)}"><span class="tresor-item-glyph">${item.type === 'file' ? '▣' : '✎'}</span><span><strong>${esc(item.title)}</strong><small>${label(item.type === 'file' ? 'file' : 'note')}</small></span></button>`).join('');
        }

        function workspace() {
            const current = state.selected;
            return `<div class="tresor-workspace"><header class="tresor-topbar"><div><span class="tresor-mark">✦</span><span class="tresor-brand">${label('title')}</span><small>${label('private')}</small></div><div>${button('lock', 'lock')}${button('password', 'change_password')}</div></header>
                <div class="tresor-main"><aside class="tresor-sidebar"><div class="tresor-sidebar-head"><span>${label('collection')}</span><small>${state.items.length}</small></div><label class="tresor-search"><span class="sr-only">${label('search')}</span><input data-search type="search" placeholder="${label('search')}" value="${esc(state.search)}"></label><div class="tresor-actions">${button('new-note', 'new_note')}${button('device', 'import_device')}${button('desktop', 'import_desktop')}</div><input data-file type="file" hidden><nav aria-label="${label('collection')}">${listRows()}</nav></aside>
                <section class="tresor-content">${!current ? `<div class="tresor-welcome"><img src="/img/tresor/vault-door.svg" alt=""><h2>${label('welcome')}</h2><p>${label('welcome_help')}</p></div>` : current.corrupt ? `<div class="tresor-welcome"><h2>${label('integrity_error')}</h2><p>${label('integrity_help')}</p>${button('delete', 'delete')}</div>` : `<div class="tresor-editor-head"><div><span class="tresor-eyebrow">${label(current.type === 'file' ? 'file' : 'note')}</span>${current.type === 'note' ? `<label class="tresor-title-label"><span class="sr-only">${label('note_title')}</span><input data-title aria-label="${label('note_title')}" maxlength="200" value="${esc(current.title)}"></label>` : `<h2>${esc(current.title)}</h2>`}</div><div>${current.type === 'note' ? button('save', 'save', 'tresor-primary') : ''}${button('export', 'export')}${button('delete', 'delete')}</div></div>${current.type === 'note' ? `<label class="tresor-note-label">${label('note_content')}<textarea data-note spellcheck="true" placeholder="${label('note_placeholder')}"></textarea></label>` : `<div class="tresor-file-card"><span class="tresor-file-icon">▣</span><strong>${esc(current.title)}</strong><span>${esc(formatSize(current.size || 0))}</span><p>${label('export_warning')}</p></div>`}`}</section></div>
                <footer class="tresor-footer"><span>${label('encrypted_local')}</span><span class="tresor-status" role="status" aria-live="polite" data-status data-error="${state.error}">${esc(state.message)}</span></footer>
                ${state.modal ? `<div class="tresor-modal-backdrop"><div class="tresor-modal" role="dialog" aria-modal="true" aria-labelledby="tresor-modal-title"><h2 id="tresor-modal-title">${label(state.modal === 'export' ? 'export' : state.modal === 'delete' ? 'delete' : 'change_password')}</h2><p>${label(state.modal === 'export' ? 'export_warning' : state.modal === 'delete' ? 'delete_warning' : 'password_warning')}</p>${state.modal === 'password' ? `<form data-form="password"><label>${label('new_password')}<input name="password" type="password" minlength="16" required autocomplete="new-password"></label><button class="tresor-primary" type="submit">${label('change_password')}</button></form>` : button('confirm-' + state.modal, state.modal === 'export' ? 'export' : 'delete', 'tresor-primary')}${button('cancel', 'cancel')}<p class="tresor-status" role="status" aria-live="polite" data-status data-error="${state.error}">${esc(state.message)}</p></div></div>` : ''}
            </div>`;
        }

        function draw() {
            if (state.disposed) return;
            host.innerHTML = `<div class="tresor-app ${state.phase === 'open' ? 'is-open' : ''}">${state.phase === 'open' ? workspace() : door()}</div>`;
            const area = host.querySelector('[data-note]');
            if (area) area.value = state.noteText;
            const title = host.querySelector('[data-title]');
            if (title) title.value = state.noteTitle;
            setBusyControls(state.busy);
        }

        function setBusyControls(busy) {
            host.querySelector('.tresor-app')?.setAttribute('aria-busy', String(busy));
            host.querySelectorAll('button:not([data-action=lock])').forEach(button => { button.disabled = busy; });
            host.querySelectorAll('input, textarea').forEach(field => { field.readOnly = busy; });
        }

        async function refresh() {
            const current = taskGuard();
            const c = await crypt();
            const rows = await request('/items');
            if (!current()) return;
            const items = await Promise.all(rows.map(async row => {
                try {
                    const plain = await c.open(state.master, c.fromBase64(row.meta), c.recordContext(row.id, 'meta'));
                    if (!current()) { plain.fill(0); return null; }
                    const meta = JSON.parse(decoder.decode(plain));
                    plain.fill(0);
                    if (!['note', 'file'].includes(meta.type) || typeof meta.title !== 'string') throw new Error('invalid_metadata');
                    return { ...meta, id: row.id, revision: row.revision };
                } catch (_) { return { id: row.id, revision: row.revision, title: tr('integrity_error'), corrupt: true }; }
            }));
            if (!current()) return;
            state.items = items;
            state.items.sort((a, b) => a.title.localeCompare(b.title));
            if (state.selected) state.selected = state.items.find(item => item.id === state.selected.id) || null;
            draw();
        }

        function formatSize(bytes) { return bytes < 1024 ? `${bytes} B` : bytes < 1048576 ? `${Math.ceil(bytes / 1024)} KiB` : `${(bytes / 1048576).toFixed(1)} MiB`; }
        async function unlock(master, current) {
            if (!current()) { master.fill(0); return; }
            state.master = master;
            state.phase = 'open';
            activity();
            try { await refresh(); }
            catch (error) {
                if (!current()) return;
                forget();
                state.phase = state.header ? 'locked' : 'setup';
                draw();
                announce(tr(error.message === 'conflict' ? 'conflict' : 'request_failed'), true);
                throw error;
            }
            if (!current()) return;
            host.querySelector('.tresor-app')?.classList.add('is-opening');
        }

        async function saveRecord(type, title, bytes, mime = '') {
            const current = taskGuard();
            const c = await crypt();
            if (!current()) { bytes.fill(0); return; }
            const id = crypto.randomUUID();
            const meta = { type, title, size: bytes.length, mime };
            let encryptedBody;
            try { encryptedBody = await c.seal(state.master, bytes, c.recordContext(id, 'body')); }
            finally { bytes.fill(0); }
            if (!current()) return;
            const encryptedMeta = await c.seal(state.master, encoder.encode(JSON.stringify(meta)), c.recordContext(id, 'meta'));
            if (!current()) return;
            await request('/items', 'POST', {
                id,
                meta: c.toBase64(encryptedMeta),
                body: c.toBase64(encryptedBody)
            });
            if (!current()) return;
            await refresh();
            if (!current()) return;
            await select(id);
        }

        async function select(id) {
            const current = taskGuard();
            await saveNote();
            if (!current()) return;
            const item = state.items.find(item => item.id === id) || null;
            let noteTitle = item?.title || '';
            let noteText = '';
            let dirty = false;
            if (item?.type === 'note' && !item.corrupt) {
                const c = await crypt();
                const row = await request('/items/' + id);
                if (!current()) return;
                const plain = await c.open(state.master, c.fromBase64(row.body), c.recordContext(id, 'body'));
                if (!current()) { plain.fill(0); return; }
                noteText = decoder.decode(plain);
                plain.fill(0);
                try {
                    const saved = JSON.parse(localStorage.getItem(draftKey(id)) || 'null');
                    if (saved?.cipher && saved.revision === item.revision) {
                        const bytes = await c.open(state.master, c.fromBase64(saved.cipher), c.recordContext(id, 'draft'));
                        if (!current()) { bytes.fill(0); return; }
                        const draft = JSON.parse(decoder.decode(bytes));
                        bytes.fill(0);
                        if (typeof draft.noteText === 'string' && typeof draft.title === 'string') {
                            dirty = draft.noteText !== noteText || draft.title !== noteTitle;
                            noteText = draft.noteText;
                            noteTitle = draft.title;
                        }
                    } else if (saved?.cipher) announce(tr('conflict'), true);
                } catch (_) { announce(tr('request_failed'), true); }
            }
            if (!current()) return;
            state.selected = item;
            state.noteTitle = noteTitle;
            state.noteText = noteText;
            state.dirty = dirty;
            state.editVersion++;
            draw();
            if (state.dirty) { announce(tr('draft_recovered')); scheduleSave(); }
        }

        async function importFile(file) {
            const current = taskGuard();
            if (state.disposed) return;
            if (file.size > maxFile) throw new Error('file_too_large');
            const bytes = new Uint8Array(await file.arrayBuffer());
            if (!current()) { bytes.fill(0); return; }
            await saveRecord('file', file.name, bytes, file.type || 'application/octet-stream');
            if (current()) announce(tr('imported'));
        }

        async function importDesktop() {
            const current = taskGuard();
            if (typeof context.openFileDialog !== 'function') throw new Error('desktop_unavailable');
            const choice = await context.openFileDialog({});
            if (!current() || !choice || choice.canceled || !choice.path) return;
            const response = await fetch('/api/desktop/download?path=' + encodeURIComponent(choice.path), { credentials: 'same-origin', cache: 'no-store' });
            if (!response.ok) throw new Error('request_failed');
            if (Number(response.headers.get('Content-Length') || 0) > maxFile) throw new Error('file_too_large');
            const blob = await response.blob();
            if (!current()) return;
            const name = choice.path.split(/[\\/]/).pop();
            await importFile(new File([blob], name, { type: blob.type || 'application/octet-stream' }));
        }

        const draftKey = id => 'aurago:tresor:draft:' + id;

        function encryptDraft() {
            const item = state.selected;
            if (!item || item.type !== 'note' || !state.master) return;
            const version = state.editVersion;
            const payload = { title: state.noteTitle, noteText: state.noteText };
            const key = state.master;
            const revision = item.revision;
            state.draftPromise = state.draftPromise.catch(() => {}).then(async () => {
                if (state.disposed || state.master !== key || version !== state.editVersion || !state.dirty) return;
                const c = await crypt();
                const plain = encoder.encode(JSON.stringify(payload));
                let sealed;
                try { sealed = await c.seal(key, plain, c.recordContext(item.id, 'draft')); }
                finally { plain.fill(0); }
                if (state.disposed || state.master !== key || version !== state.editVersion || !state.dirty) return;
                localStorage.setItem(draftKey(item.id), JSON.stringify({ revision, cipher: c.toBase64(sealed) }));
                state.draftReadyVersion = version;
            });
            const current = taskGuard();
            state.draftPromise.catch(() => { if (current()) announce(tr('request_failed'), true); });
        }

        function scheduleSave() {
            const current = taskGuard();
            clearTimeout(state.saveTimer);
            state.saveTimer = setTimeout(() => saveNote().catch(() => { if (current()) announce(tr('request_failed'), true); }), 600);
        }

        async function persistNote() {
            const current = taskGuard();
            const item = state.selected;
            if (!item || item.type !== 'note' || !state.dirty) return;
            const title = state.noteTitle.trim();
            if (!title) throw new Error('title_required');
            const version = state.editVersion;
            const noteText = state.noteText;
            const c = await crypt();
            if (!current()) return;
            const bytes = encoder.encode(noteText);
            const meta = { type: 'note', title, size: bytes.length, mime: 'text/plain' };
            let encryptedBody;
            try { encryptedBody = await c.seal(state.master, bytes, c.recordContext(item.id, 'body')); }
            finally { bytes.fill(0); }
            if (!current()) return;
            const encryptedMeta = await c.seal(state.master, encoder.encode(JSON.stringify(meta)), c.recordContext(item.id, 'meta'));
            if (!current()) return;
            await request('/items/' + item.id, 'PUT', {
                id: item.id,
                meta: c.toBase64(encryptedMeta),
                body: c.toBase64(encryptedBody)
            }, item.revision);
            if (!current()) return;
            item.revision++;
            item.title = title;
            state.dirty = state.editVersion !== version;
            if (!state.dirty) {
                state.editVersion++;
                try { localStorage.removeItem(draftKey(item.id)); } catch (_) { /* Storage may be disabled. */ }
            }
            host.querySelector('.tresor-sidebar nav').innerHTML = listRows();
            announce(tr('saved'));
        }

        async function saveNote() {
            const current = taskGuard();
            clearTimeout(state.saveTimer);
            if (state.savePromise) {
                await state.savePromise;
                if (current() && state.dirty) return saveNote();
                return;
            }
            if (!state.dirty) return;
            const pending = persistNote();
            state.savePromise = pending;
            try { await pending; }
            finally { if (state.savePromise === pending) state.savePromise = null; }
            if (current() && state.dirty) return saveNote();
        }

        async function exportSelected() {
            const current = taskGuard();
            const item = state.selected;
            if (!item) return;
            const c = await crypt();
            const row = await request('/items/' + item.id);
            if (!current()) return;
            const bytes = await c.open(state.master, c.fromBase64(row.body), c.recordContext(item.id, 'body'));
            if (!current()) { bytes.fill(0); return; }
            const blob = new Blob([bytes], { type: item.mime || 'application/octet-stream' });
            bytes.fill(0);
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = item.type === 'note' ? item.title + '.txt' : item.title;
            document.body.appendChild(link);
            link.click();
            link.remove();
            setTimeout(() => URL.revokeObjectURL(url), 60000);
            state.modal = '';
            draw();
        }

        async function action(name, node) {
            const current = taskGuard();
            if (name === 'lock') { await lock(); return; }
            if (['new-note', 'device', 'desktop', 'export', 'delete', 'select'].includes(name)) await saveNote();
            if (!current()) return;
            if (name === 'cancel') { state.modal = ''; draw(); return; }
            if (['export', 'delete', 'password'].includes(name)) { state.modal = name; draw(); host.querySelector('.tresor-modal button, .tresor-modal input')?.focus(); return; }
            if (name === 'select') { await select(node.dataset.id); return; }
            if (name === 'new-note') { await saveRecord('note', tr('untitled'), new Uint8Array(), 'text/plain'); return; }
            if (name === 'device') { host.querySelector('[data-file]')?.click(); return; }
            if (name === 'desktop') { await importDesktop(); return; }
            if (name === 'save') { await saveNote(); return; }
            if (name === 'confirm-export') { await exportSelected(); return; }
            if (name === 'confirm-delete' && state.selected) {
                await request('/items/' + state.selected.id, 'DELETE', undefined, state.selected.revision);
                if (!current()) return;
                state.selected = null; state.modal = ''; state.noteText = ''; await refresh();
            }
        }

        async function submit(form) {
            const current = taskGuard();
            const data = new FormData(form);
            const c = await crypt();
            if (!current()) return;
            if (form.dataset.form === 'setup') {
                const password = String(data.get('password'));
                if (password !== data.get('repeat')) throw new Error('password_mismatch');
                const pending = await c.createHeader(password);
                if (!current()) return;
                state.pending = pending;
                form.reset(); state.phase = 'confirm'; draw();
            } else if (form.dataset.form === 'confirm') {
                const pending = state.pending;
                const master = await c.unlockWithRecovery(pending.header, String(data.get('recovery')));
                try {
                    if (!current()) return;
                    await request('', 'POST', pending.header);
                    if (!current()) return;
                    state.header = { ...pending.header, revision: 1 };
                    state.pending = null;
                    await unlock(master, current);
                } finally { if (state.master !== master) master.fill(0); }
            } else if (form.dataset.form === 'unlock' || form.dataset.form === 'recover') {
                const status = await request('');
                if (!current()) return;
                state.header = status.initialized ? status.header : null;
                if (!state.header) throw new Error('conflict');
                const master = form.dataset.form === 'unlock'
                    ? await c.unlockWithPassword(state.header, String(data.get('password')))
                    : await c.unlockWithRecovery(state.header, String(data.get('recovery')));
                form.reset(); await unlock(master, current);
            } else if (form.dataset.form === 'password') {
                const status = await request('');
                if (!current() || !status.initialized) throw new Error('conflict');
                state.header = status.header;
                const next = await c.changePassword(state.header, state.master, String(data.get('password')));
                if (!current()) return;
                await request('', 'PUT', next, state.header.revision);
                if (!current()) return;
                state.header = { ...next, revision: state.header.revision + 1 };
                state.modal = ''; draw(); announce(tr('password_changed'));
            }
        }

        async function perform(fn) {
            if (state.busy) return;
            const current = taskGuard();
            state.busy = true;
            setBusyControls(true);
            announce(tr('working'));
            try { await fn(); if (current() && state.message === tr('working')) announce(''); }
            catch (error) {
                if (!current()) return;
                if (error.message === 'admin_required' || error.message === 'https_required') { forget(); state.phase = 'locked'; draw(); }
                host.querySelectorAll('input[type=password], input[name=recovery]').forEach(field => { field.value = ''; });
                const key = ['invalid_recovery_key', 'invalid_password', 'conflict', 'https_required', 'admin_required', 'file_too_large', 'password_mismatch', 'desktop_unavailable', 'title_required'].includes(error.message)
                    ? error.message : error.name === 'OperationError' ? 'wrong_key' : 'request_failed';
                announce(tr(key), true);
            } finally {
                if (!current()) return;
                state.busy = false;
                setBusyControls(false);
                const modal = host.querySelector('.tresor-modal');
                if (modal && !modal.contains(document.activeElement)) modal.querySelector('button, input')?.focus();
            }
        }

        listen('click', event => {
            const node = event.target.closest('[data-action]');
            if (node && host.contains(node)) {
                if (node.dataset.action === 'lock') lock();
                else perform(() => action(node.dataset.action, node));
            }
        });
        listen('submit', event => {
            const form = event.target.closest('[data-form]');
            if (form) { event.preventDefault(); perform(() => submit(form)); }
        });
        listen('input', event => {
            if (event.target.matches('[data-search]')) { state.search = event.target.value; host.querySelector('.tresor-sidebar nav').innerHTML = listRows(); }
            if (event.target.matches('[data-note], [data-title]')) {
                if (event.target.matches('[data-note]')) state.noteText = event.target.value;
                if (event.target.matches('[data-title]')) state.noteTitle = event.target.value;
                state.dirty = true;
                state.editVersion++;
                encryptDraft();
                scheduleSave();
            }
            activity();
        });
        listen('change', event => {
            if (event.target.matches('[data-file]') && event.target.files?.[0]) perform(() => importFile(event.target.files[0]));
        });
        listen('pointerdown', activity);
        listen('wheel', activity, { passive: true });
        listen('keydown', event => {
            activity();
            if (!state.modal) return;
            if (event.key === 'Escape') { event.preventDefault(); state.modal = ''; draw(); return; }
            if (event.key !== 'Tab') return;
            const focusable = [...host.querySelectorAll('.tresor-modal button, .tresor-modal input')];
            if (!focusable.length) return;
            const first = focusable[0], last = focusable[focusable.length - 1];
            if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
            else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
        });
        const onPageHide = () => lock();
        window.addEventListener('pagehide', onPageHide, { signal: events.signal });
        window.addEventListener('beforeunload', event => {
            if (state.dirty && state.draftReadyVersion !== state.editVersion) { event.preventDefault(); event.returnValue = ''; }
        }, { signal: events.signal });
        context.setWindowBeforeClose?.(windowId, async () => {
            try { await saveNote(); return !state.dirty; }
            catch (error) { announce(tr(error.message === 'conflict' ? 'conflict' : 'request_failed'), true); return false; }
        });
        state.cleanup = () => events.abort();
        draw();
        (async () => {
            const current = taskGuard();
            if (!window.isSecureContext || !crypto?.subtle || !WebAssembly) { state.phase = 'insecure'; draw(); return; }
            try {
                const status = await request('');
                if (!current()) return;
                state.header = status.initialized ? status.header : null;
                state.phase = state.header ? 'locked' : 'setup';
                draw();
            } catch (error) {
                if (!current()) return;
                state.phase = error.message === 'https_required' ? 'insecure' : 'loading';
                draw(); announce(tr(error.message === 'https_required' ? 'https_help' : error.message === 'admin_required' ? 'admin_required' : 'request_failed'), true);
            }
        })();
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        state.forget();
        state.cleanup?.();
        state.host.replaceChildren();
        instances.delete(windowId);
    }

    window.TresorApp = { render, dispose };
})();
