// Server-owned organization and version annotations, independent of graph editing and evidence.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const modals = new WeakMap();
    const trim = value => ED.template.applyFilter('trim', value, []);
    function textFits(value, points, bytes, multiline = false) {
        if (typeof value !== 'string' || value.length > points * 2) return false;
        let count = 0, size = 0;
        for (const ch of value) { const cp = ch.codePointAt(0); if (cp >= 0xD800 && cp <= 0xDFFF || (cp < 32 || cp >= 127 && cp <= 159) && !(multiline && [9, 10, 13].includes(cp))) return false; size += cp < 128 ? 1 : cp < 2048 ? 2 : cp < 65536 ? 3 : 4; if (++count > points || size > bytes) return false; }
        return true;
    }
    function clean(value) { return typeof value === 'string' && !value.includes('[redacted]'); }
    function writable(env) { return env.root.isConnected && !env.readonly && (!env.canWrite || env.canWrite()); }
    function owned(env, key, options) {
        let rows = modals.get(env.root); if (!rows) { rows = new Map(); modals.set(env.root, rows); }
        if (rows.has(key)) return null;
        const view = ED.core.modal(env.root, Object.assign({ closeLabel: env.t('easydrag.ui.close'), className: 'ed-modal--metadata' }, options, { onAction: (action, dialog) => life.writesRevoked && action === 'save' ? false : options.onAction ? options.onAction(action, dialog) : true }));
        const controller = new AbortController(), bag = ED.core.bag(); let closed = false;
        const life = { view, signal: controller.signal, bag, writesRevoked: false, live: () => !closed && !controller.signal.aborted && env.root.isConnected && view.el.isConnected, cancel() { view.close(null); }, abort() { controller.abort(); } };
        rows.set(key, life);
        const close = view.close;
        view.close = result => { closed = true; controller.abort(); if (rows.get(key) === life) rows.delete(key); close(result); };
        life.updateSave = () => view.el.querySelectorAll('[data-ed-action="save"]').forEach(button => { button.disabled = life.writesRevoked || controller.signal.aborted || !!(life.saveBlocked && life.saveBlocked()); });
        const setBusy = view.setBusy; view.setBusy = busy => { setBusy(busy); if (!busy) life.updateSave(); };
        view.done.then(() => { closed = true; controller.abort(); bag.dispose(); if (rows.get(key) === life) rows.delete(key); });
        return life;
    }
    function showError(env, view, err, writing = false) {
        const code = ED.core.errorCode(err), rows = modals.get(env.root), life = Array.from(rows?.values() || []).find(item => item.view === view);
        const unauthorized = err && err.status === 401 || code === 'unauthorized';
        const forbidden = err && err.status === 403 || ['forbidden', 'permission_denied', 'desktop_scope_required'].includes(code);
        if (unauthorized || forbidden && !writing) {
            if (rows) Array.from(rows.values()).forEach(owner => {
                owner.writesRevoked = true; owner.abort(); if (owner.clear) owner.clear(err);
                owner.view.body.innerHTML = feedback(env); const error = owner.view.body.querySelector('[data-ed-metadata-error]'); error.hidden = false; error.textContent = ED.core.errorText(env.t, err); owner.view.setBusy(false);
            });
            if (env.onAuthLost) env.onAuthLost(err);
            return;
        }
        if (life && writing && (forbidden || code === 'desktop_readonly')) {
            life.writesRevoked = true; view.body.querySelectorAll('input,textarea,select,button').forEach(control => { control.disabled = true; }); view.setBusy(false);
            if (!view.body.querySelector('[data-ed-write-locked]')) view.body.appendChild(ED.core.el('<p class="ed-hint" role="status" data-ed-write-locked>' + env.esc(env.t('easydrag.ui.metadata_write_locked')) + '</p>'));
            // A write denial does not prove loss of read access. Never rebase or retry the edit.
            if (forbidden && code !== 'desktop_readonly' && life.probeRead && !life.probing) {
                life.probing = true;
                Promise.resolve().then(() => life.probeRead()).then(result => { if (life.live() && life.onProbe) life.onProbe(result); }).catch(probeError => { if (life.live() && probeError.name !== 'AbortError') showError(env, view, probeError); }).finally(() => { life.probing = false; });
            }
        }
        const box = view.body.querySelector('[data-ed-metadata-error]'); box.hidden = false; box.textContent = typeof err === 'string' ? env.t(err) : ED.core.errorText(env.t, err);
        const reload = view.body.querySelector('[data-ed-metadata-reload]'); if (reload) reload.hidden = !String(ED.core.errorCode(err) || '').includes('CONFLICT');
    }
    function feedback(env) { return '<p class="ed-error" role="alert" data-ed-metadata-error hidden></p><button type="button" class="ed-btn" data-ed-metadata-reload hidden>' + env.esc(env.t('easydrag.ui.metadata_reload')) + '</button>'; }
    function help(env, key) { return '<span class="ed-help" tabindex="0" role="note" aria-label="' + env.esc(env.t(key)) + '" title="' + env.esc(env.t(key)) + '">' + ED.core.icon('info') + '</span>'; }
    function field(env, label, markup, hint) { return '<label class="ed-label">' + env.esc(env.t(label)) + (hint ? help(env, hint) : '') + markup + '</label>'; }
    function masked(env) { return '<p class="ed-callout ed-callout--warn">' + env.esc(env.t('easydrag.ui.metadata_redacted')) + '</p>'; }
    function reloadAfterReview(env, life, action) {
        let asking = false;
        return async () => {
            if (asking || !life.live()) return;
            asking = true;
            try { if (await env.ctx.confirmDialog(env.t('easydrag.ui.metadata_reload'), env.t('easydrag.ui.metadata_reload_confirm'))) { if (life.live()) action(); } }
            finally { asking = false; }
        };
    }

    function organization(env, id, onSaved) {
        let library, folders = [], tags = [], folderChanged = false, tagsChanged = false, replacing = false, composing = false, busy = false, sequence = 0;
        const life = owned(env, 'library:' + id, { title: env.t('easydrag.ui.organize'), body: '<p class="ed-hint">' + env.esc(env.t('easydrag.ui.organization_hint')) + '</p><div data-ed-organization-form></div>' + feedback(env), actions: env.readonly ? [{ id: 'close-dialog', label: env.t('easydrag.ui.close') }] : [{ id: 'cancel', label: env.t('easydrag.ui.cancel') }, { id: 'save', label: env.t('easydrag.ui.save_now'), primary: true }], onAction: async action => {
            if (action !== 'save') return true;
            if (!life.live() || !writable(env) || busy || !library || !addPending()) return false;
            const body = { base_revision: library.revision };
            if (folderChanged) body.folder_id = life.view.body.querySelector('[data-ed-organization-folder]').value || null;
            if (tagsChanged) body.tags = tags.slice();
            if (!folderChanged && !tagsChanged) return true;
            busy = true; lock();
            try { const result = await env.api.saveLibrary(id, body, life.signal); if (life.live() && onSaved) onSaved(result.library); return true; }
            catch (err) { if (life.live() && err.name !== 'AbortError') showError(env, life.view, err, true); return false; }
            finally { busy = false; if (life.live()) lock(); }
        } });
        if (!life) return null;
        life.clear = () => { library = undefined; folders = []; tags = []; folderChanged = false; tagsChanged = false; };
        life.saveBlocked = () => busy || !library || !writable(env);
        life.probeRead = async () => { const result = await Promise.all([env.api.library(id, life.signal), env.api.folders(life.signal)]); return { library: result[0].library, folders: result[1].folders || [] }; };
        life.onProbe = result => {
            const select = life.view.body.querySelector('[data-ed-organization-folder]'), chosen = select && select.value;
            folders = result.folders;
            if (library) library = Object.assign({}, library, { tags: result.library.tags, tags_redacted: result.library.tags_redacted, tags_editable: result.library.tags_editable });
            if (!tagsChanged) tags = (result.library.tags || []).slice();
            if (select) { select.innerHTML = '<option value="">' + env.esc(env.t('easydrag.ui.folder_none')) + '</option>' + folders.map(row => '<option value="' + env.esc(row.id) + '">' + env.esc(row.name) + '</option>').join(''); if (chosen && !folders.some(row => row.id === chosen)) select.appendChild(ED.core.el('<option value="' + env.esc(chosen) + '">' + env.esc(env.t('easydrag.ui.folder_missing')) + '</option>')); select.value = chosen; }
            renderTags(); if (env.onLibraryRead) env.onLibraryRead(id, result.library, folders);
        };
        function lock() {
            const host = life.view.body.querySelector('[data-ed-organization-form]');
            if (!host) return;
            host.querySelectorAll('input,select,button').forEach(control => { control.disabled = busy || life.writesRevoked || !writable(env) || !library || ((control.matches('[data-ed-tag-input], [data-ed-tag-add], [data-ed-tag-remove]')) && (library.tags_redacted || library.tags_editable === false) && !replacing); });
            life.updateSave();
        }
        function renderTags() {
            const host = life.view.body.querySelector('[data-ed-organization-tags]');
            host.innerHTML = tags.map((tag, index) => '<span class="ed-tag">' + env.esc(tag) + (!env.readonly && (!(library.tags_redacted || library.tags_editable === false) || replacing) ? '<button type="button" class="ed-icon-btn" data-ed-tag-remove="' + index + '" aria-label="' + env.esc(env.t('easydrag.ui.remove_tag', { name: tag })) + '">' + ED.core.icon('x') + '</button>' : '') + '</span>').join(''); lock();
        }
        function addPending() {
            const input = life.view.body.querySelector('[data-ed-tag-input]');
            if (!input || input.disabled) return true;
            if (composing) return false;
            if (input.value.length > 4096) { showError(env, life.view, 'easydrag.ui.metadata_text_invalid'); return false; }
            const tag = trim(input.value);
            if (!tag) return true;
            if (!textFits(tag, 40, 160) || tags.length >= 16 || !clean(tag)) { showError(env, life.view, 'easydrag.ui.tags_limit'); return false; }
            if (tags.includes(tag)) { showError(env, life.view, 'easydrag.ui.tags_duplicate'); return false; }
            tags.push(tag); tagsChanged = true; input.value = ''; renderTags(); return true;
        }
        function render() {
            const host = life.view.body.querySelector('[data-ed-organization-form]');
            host.innerHTML = field(env, 'easydrag.ui.folder', '<select class="ed-input" data-ed-organization-folder><option value="">' + env.esc(env.t('easydrag.ui.folder_none')) + '</option>' + folders.map(folder => '<option value="' + env.esc(folder.id) + '">' + env.esc(folder.name) + '</option>').join('') + '</select>') + '<div class="ed-label">' + env.esc(env.t('easydrag.ui.tags')) + help(env, 'easydrag.ui.tags_limit') + '</div>' + ((library.tags_redacted || library.tags_editable === false) ? masked(env) + (!env.readonly ? '<button type="button" class="ed-btn" data-ed-tags-replace>' + env.esc(env.t('easydrag.ui.metadata_replace')) + '</button>' : '') : '') + '<div class="ed-tags-list" data-ed-organization-tags></div><div class="ed-metadata-tag-add"><input class="ed-input" data-ed-tag-input aria-label="' + env.esc(env.t('easydrag.ui.tags_add')) + '" enterkeyhint="done"><button type="button" class="ed-btn" data-ed-tag-add>' + env.esc(env.t('easydrag.ui.add')) + '</button></div>';
            const select = host.querySelector('[data-ed-organization-folder]');
            if (library.folder_id && !folders.some(row => row.id === library.folder_id)) select.appendChild(ED.core.el('<option value="' + env.esc(library.folder_id) + '">' + env.esc(library.folder_id) + '</option>'));
            select.value = library.folder_id || ''; renderTags();
        }
        async function load() {
            const mine = ++sequence;
            busy = true; lock();
            try { const res = await Promise.all([env.api.library(id, life.signal), env.api.folders(life.signal)]); if (!life.live() || mine !== sequence) return; library = res[0].library; folders = res[1].folders || []; tags = (library.tags || []).slice(); folderChanged = false; tagsChanged = false; replacing = false; composing = false; life.view.body.querySelector('[data-ed-metadata-error]').hidden = true; life.view.body.querySelector('[data-ed-metadata-reload]').hidden = true; render(); }
            catch (err) { if (life.live() && mine === sequence && err.name !== 'AbortError') showError(env, life.view, err); }
            finally { if (mine === sequence) { busy = false; if (life.live()) lock(); } }
        }
        life.bag.listen(life.view.body, 'change', event => { if (event.target.closest('[data-ed-organization-folder]')) folderChanged = true; });
        life.bag.listen(life.view.body, 'compositionstart', event => { if (event.target.matches('[data-ed-tag-input]')) composing = true; });
        life.bag.listen(life.view.body, 'compositionend', () => { composing = false; });
        life.bag.listen(life.view.body, 'keydown', event => { if (event.key === 'Enter' && event.target.matches('[data-ed-tag-input]') && !event.isComposing && !composing) { event.preventDefault(); addPending(); } });
        life.bag.listen(life.view.body, 'click', event => {
            if (!life.live() || life.writesRevoked || !writable(env) || busy || !library) return;
            if (event.target.closest('[data-ed-tag-add]')) addPending();
            const remove = event.target.closest('[data-ed-tag-remove]'); if (remove) { tags.splice(Number(remove.dataset.edTagRemove), 1); tagsChanged = true; renderTags(); }
            if (event.target.closest('[data-ed-tags-replace]')) { replacing = true; tags = []; tagsChanged = true; renderTags(); event.target.closest('button').disabled = true; }
        });
        life.bag.listen(life.view.body.querySelector('[data-ed-metadata-reload]'), 'click', reloadAfterReview(env, life, load));
        load(); return life.view;
    }

    function folderEditor(env, row, changed) {
        let busy = false;
        const life = owned(env, 'folder-edit:' + (row ? row.id : 'new'), { title: env.t(row ? 'easydrag.ui.rename' : 'easydrag.ui.folder_new'), body: (row && (row.redacted || row.editable === false) ? masked(env) : '') + field(env, 'easydrag.ui.folder_name', '<input class="ed-input" data-ed-folder-name>', 'easydrag.ui.folder_limit') + feedback(env), actions: [{ id: 'cancel', label: env.t('easydrag.ui.cancel') }, { id: 'save', label: env.t('easydrag.ui.save_now'), primary: true }], onAction: async action => {
            if (action !== 'save') return true;
            const input = life.view.body.querySelector('[data-ed-folder-name]');
            if (!life.live() || !writable(env) || busy || input.edComposing) return false;
            const name = input.value.length <= 4096 ? trim(input.value) : '';
            if (!name || !textFits(name, 80, 320) || !clean(name)) { showError(env, life.view, 'easydrag.ui.folder_limit'); return false; }
            busy = true; input.disabled = true;
            try { if (row) await env.api.renameFolder(row.id, name, row.revision, life.signal); else await env.api.createFolder(name, life.signal); if (life.live()) changed(); return true; }
            catch (err) { if (life.live() && err.name !== 'AbortError') showError(env, life.view, err, true); return false; }
            finally { busy = false; if (life.live()) input.disabled = life.writesRevoked || !writable(env); }
        } });
        if (!life) return;
        life.probeRead = () => env.api.folders(life.signal);
        life.onProbe = result => {
            const next = row && (result.folders || []).find(item => item.id === row.id);
            if (next && input.value === row.name) input.value = next.name;
            Array.from(modals.get(env.root)?.values() || []).forEach(owner => { if (owner.receiveFolders) owner.receiveFolders(result.folders || []); });
        };
        const input = life.view.body.querySelector('[data-ed-folder-name]'); input.value = row && !row.redacted && row.editable !== false ? row.name : '';
        life.bag.listen(input, 'compositionstart', () => { input.edComposing = true; }); life.bag.listen(input, 'compositionend', () => { input.edComposing = false; });
        life.bag.listen(life.view.body.querySelector('[data-ed-metadata-reload]'), 'click', reloadAfterReview(env, life, () => { life.view.close(null); changed(); }));
        return life.view;
    }
    function folders(env, changed) {
        let rows = [], limit = 50, busy = false, sequence = 0;
        const life = owned(env, 'folders', { title: env.t('easydrag.ui.folders_manage'), body: '<div class="ed-row"><button type="button" class="ed-btn" data-ed-folder-new>' + ED.core.icon('plus') + env.esc(env.t('easydrag.ui.folder_new')) + '</button></div><p class="ed-hint">' + env.esc(env.t('easydrag.ui.folder_delete_hint')) + '</p><div data-ed-folder-list></div><button type="button" class="ed-btn" data-ed-folder-more hidden>' + env.esc(env.t('easydrag.ui.block_load_more')) + '</button>' + feedback(env), actions: [{ id: 'close-dialog', label: env.t('easydrag.ui.close') }] });
        if (!life) return null;
        life.clear = () => { rows = []; };
        life.probeRead = () => env.api.folders(life.signal);
        life.receiveFolders = next => { rows = next; if (changed) changed(rows); paint(); };
        life.onProbe = result => life.receiveFolders(result.folders || []);
        function paint() {
            const host = life.view.body.querySelector('[data-ed-folder-list]');
            host.innerHTML = !rows.length ? '<p class="ed-hint">' + env.esc(env.t('easydrag.ui.folders_empty')) + '</p>' : rows.slice(0, limit).map(row => '<div class="ed-folder-row"><div><strong>' + env.esc(row.name) + '</strong><p class="ed-hint">' + env.esc(env.t('easydrag.ui.folder_flows', { count: row.flow_count })) + '</p></div><button type="button" class="ed-btn" data-ed-folder-rename="' + env.esc(row.id) + '"' + (!writable(env) || busy ? ' disabled' : '') + '>' + env.esc(env.t('easydrag.ui.rename')) + '</button><button type="button" class="ed-icon-btn" data-ed-folder-delete="' + env.esc(row.id) + '" aria-label="' + env.esc(env.t('easydrag.ui.delete')) + '"' + (!writable(env) || busy || row.flow_count ? ' disabled' : '') + '>' + ED.core.icon('trash') + '</button></div>').join('');
            life.view.body.querySelector('[data-ed-folder-more]').hidden = rows.length <= limit;
            life.view.body.querySelector('[data-ed-folder-new]').disabled = !writable(env) || busy || rows.length >= 1000;
            if (life.writesRevoked) life.view.body.querySelectorAll('input,select,button').forEach(control => { control.disabled = true; });
        }
        async function load() { const mine = ++sequence; busy = true; paint(); try { const res = await env.api.folders(life.signal); if (life.live() && mine === sequence) { rows = res.folders || []; if (changed) changed(rows); } } catch (err) { if (life.live() && mine === sequence && err.name !== 'AbortError') showError(env, life.view, err); } finally { if (mine === sequence) { busy = false; if (life.live()) paint(); } } }
        life.bag.listen(life.view.body, 'click', async event => {
            if (event.target.closest('[data-ed-folder-more]')) { limit += 50; paint(); return; }
            if (!life.live() || life.writesRevoked || !writable(env) || busy) return;
            if (event.target.closest('[data-ed-folder-new]')) folderEditor(env, null, load);
            const rename = event.target.closest('[data-ed-folder-rename]'); if (rename) folderEditor(env, rows.find(row => row.id === rename.dataset.edFolderRename), load);
            const remove = event.target.closest('[data-ed-folder-delete]');
            if (remove) {
                const row = rows.find(row => row.id === remove.dataset.edFolderDelete); if (!row || row.flow_count) return;
                busy = true; paint();
                try { const ok = await env.ctx.confirmDialog(env.t('easydrag.ui.delete'), row.name); if (!ok || !life.live() || !writable(env)) return; await env.api.deleteFolder(row.id, row.revision, life.signal); if (life.live()) await load(); }
                catch (err) { if (life.live() && err.name !== 'AbortError') showError(env, life.view, err, true); }
                finally { busy = false; if (life.live()) paint(); }
            }
        });
        life.bag.listen(life.view.body.querySelector('[data-ed-metadata-reload]'), 'click', load); load(); return life.view;
    }
    function note(env, revision, edit, onUpdate, onDenied) {
        let record, replacing = false, busy = false, composing = false, sequence = 0;
        const life = owned(env, 'note:' + env.flow.id + ':' + revision, { title: env.t('easydrag.ui.version_note') + ' · v' + revision, body: '<div data-ed-note-form></div>' + feedback(env), actions: edit && !env.readonly ? [{ id: 'cancel', label: env.t('easydrag.ui.cancel') }, { id: 'save', label: env.t('easydrag.ui.save_now'), primary: true }] : [{ id: 'close-dialog', label: env.t('easydrag.ui.close') }], onAction: async action => {
            if (action !== 'save') return true;
            if (!life.live() || !writable(env) || !record || busy || composing || ((!record.editable || record.redacted) && !replacing)) return false;
            const area = life.view.body.querySelector('[data-ed-note-text]'), value = area.value;
            if (!textFits(value, 2048, 4096, true) || !clean(value)) { showError(env, life.view, 'easydrag.ui.note_limit'); return false; }
            if (!replacing && value === record.note) return true;
            busy = true; area.disabled = true;
            try { const result = await env.api.saveVersionNote(env.flow.id, revision, value, record.revision, life.signal); if (life.live() && onUpdate) onUpdate(result); return true; }
            catch (err) { if (life.live() && err.name !== 'AbortError') showError(env, life.view, err, true); return false; }
            finally { busy = false; if (life.live()) { area.disabled = life.writesRevoked || !writable(env); life.updateSave(); } }
        } });
        if (!life) return null;
        life.clear = err => { record = undefined; if (onDenied) onDenied(err); };
        life.saveBlocked = () => busy || !record || !writable(env) || ((!record.editable || record.redacted) && !replacing);
        life.probeRead = () => env.api.versionNote(env.flow.id, revision, life.signal);
        life.onProbe = result => {
            const area = life.view.body.querySelector('[data-ed-note-text]');
            if (record) { if (area && !replacing && area.value === record.note) area.value = result.note || ''; record = Object.assign({}, record, { note: result.note, redacted: result.redacted, editable: result.editable }); }
            if (onUpdate) onUpdate(result);
        };
        function paint() {
            const host = life.view.body.querySelector('[data-ed-note-form]'), hidden = record.redacted || record.editable === false;
            host.innerHTML = (hidden ? masked(env) : '') + (edit && !env.readonly ? field(env, 'easydrag.ui.version_note', '<textarea class="ed-input" rows="7" data-ed-note-text></textarea>', 'easydrag.ui.note_limit') + (hidden ? '<button type="button" class="ed-btn" data-ed-note-replace>' + env.esc(env.t('easydrag.ui.metadata_replace')) + '</button>' : '') + '<p class="ed-hint">' + env.esc(env.t('easydrag.ui.note_clear_hint')) + '</p>' : '<div class="ed-note-text" data-ed-note-display></div>');
            const area = host.querySelector('[data-ed-note-text]');
            if (area) { area.value = record.note || ''; area.readOnly = hidden; area.disabled = !writable(env); }
            const display = host.querySelector('[data-ed-note-display]'); if (display) display.textContent = record.note || env.t('easydrag.ui.note_empty');
            life.updateSave();
        }
        async function load() {
            const mine = ++sequence; busy = true; life.updateSave(); const area = life.view.body.querySelector('[data-ed-note-text]'); if (area) area.disabled = true;
            try { const result = await env.api.versionNote(env.flow.id, revision, life.signal); if (!life.live() || mine !== sequence) return; record = result; replacing = false; composing = false; paint(); if (onUpdate) onUpdate(result); life.view.body.querySelector('[data-ed-metadata-error]').hidden = true; life.view.body.querySelector('[data-ed-metadata-reload]').hidden = true; }
            catch (err) { if (life.live() && mine === sequence && err.name !== 'AbortError') showError(env, life.view, err); }
            finally { if (mine === sequence && life.live()) { busy = false; const input = life.view.body.querySelector('[data-ed-note-text]'); if (input) input.disabled = life.writesRevoked || !writable(env); life.updateSave(); } }
        }
        life.bag.listen(life.view.body, 'compositionstart', () => { composing = true; }); life.bag.listen(life.view.body, 'compositionend', () => { composing = false; });
        life.bag.listen(life.view.body, 'click', event => { if (event.target.closest('[data-ed-note-replace]') && life.live() && !life.writesRevoked && writable(env) && !busy && record) { const area = life.view.body.querySelector('[data-ed-note-text]'); replacing = true; area.value = ''; area.readOnly = false; area.focus(); event.target.closest('button').disabled = true; life.updateSave(); } });
        life.bag.listen(life.view.body.querySelector('[data-ed-metadata-reload]'), 'click', reloadAfterReview(env, life, load));
        load(); return life.view;
    }
    function noteMarkup(env, row) {
        if (row.note_error) return '<p class="ed-error">' + env.esc(ED.core.errorText(env.t, row.note_error)) + '</p>';
        const preview = row.note_preview || '', hidden = row.note_redacted || row.note_editable === false;
        return '<div class="ed-version-note"><p class="ed-note-preview">' + env.esc(preview || env.t('easydrag.ui.note_empty')) + '</p>' + (hidden ? '<span class="ed-hint">' + env.esc(env.t('easydrag.ui.metadata_hidden')) + '</span>' : '') + '<div class="ed-row"><button type="button" class="ed-btn ed-btn--small" data-ed-note-read="' + env.esc(row.revision) + '">' + env.esc(env.t('easydrag.ui.note_read')) + '</button>' + (!env.readonly && !env.runView ? '<button type="button" class="ed-btn ed-btn--small" data-ed-note-edit="' + env.esc(row.revision) + '">' + env.esc(env.t('easydrag.ui.note_edit')) + '</button>' : '') + '</div></div>';
    }
    function bindNotes(env, view, rows, paint) {
        const listener = event => {
            const button = event.target.closest('[data-ed-note-read], [data-ed-note-edit]'); if (!button) return;
            const revision = Number(button.dataset.edNoteRead || button.dataset.edNoteEdit), row = (typeof rows === 'function' ? rows() : rows).find(item => item.revision === revision); if (!row) return;
            const child = note(env, revision, !!button.dataset.edNoteEdit, result => { const line = (result.note || '').split('\n', 1)[0].replace(/\r$/, ''), points = Array.from(line); row.note_preview = points.length > 120 ? points.slice(0, 120).join('') + '…' : line; row.note_revision = result.revision; row.note_redacted = result.redacted; row.note_editable = result.editable; delete row.note_error; if (view.el.isConnected) paint(); }, err => { (typeof rows === 'function' ? rows() : rows).forEach(item => { item.note_preview = ''; item.note_redacted = true; item.note_editable = false; item.note_error = err; }); if (view.el.isConnected) paint(); });
            if (child) child.done.then(() => { if (view.el.isConnected) { const next = Array.from(view.body.querySelectorAll('[data-ed-note-read], [data-ed-note-edit]')).find(item => Number(item.dataset.edNoteRead || item.dataset.edNoteEdit) === revision); if (next && (!document.activeElement || document.activeElement === document.body || !document.activeElement.isConnected)) next.focus(); } });
        };
        view.body.addEventListener('click', listener); view.done.then(() => view.body.removeEventListener('click', listener));
    }
    function publicationNote(env, preview) {
        const core = ED.core, controller = new AbortController(); let closed = false, composing = false;
        const known = Number.isInteger(preview.live_revision) && Number.isInteger(preview.published_draft_revision);
        const repair = known && preview.live_revision > 0 && preview.published_draft_revision === preview.draft_revision;
        const el = core.el('<section class="ed-publication-note"><h4>' + env.esc(env.t('easydrag.ui.version_note')) + '</h4></section>');
        if (!known) el.appendChild(core.el('<p class="ed-hint">' + env.esc(env.t('easydrag.ui.note_unavailable')) + '</p>'));
        else if (repair) {
            const text = core.el('<p class="ed-note-text"></p>'); text.textContent = env.t('easydrag.ui.loading');
            el.appendChild(core.el('<p class="ed-hint">' + env.esc(env.t('easydrag.ui.note_repair_hint')) + '</p>')); el.appendChild(text);
            env.api.versionNote(env.flow.id, preview.live_revision, controller.signal).then(result => { if (!closed) { text.textContent = result.note || env.t('easydrag.ui.note_empty'); if (result.redacted || result.editable === false) el.appendChild(core.el(masked(env))); } }).catch(err => { if (!closed && err.name !== 'AbortError') text.textContent = core.errorText(env.t, err); });
        } else {
            el.appendChild(core.el(field(env, 'easydrag.ui.note_optional', '<textarea class="ed-input" rows="3" data-ed-publish-note></textarea>', 'easydrag.ui.note_limit')));
            const area = el.querySelector('textarea'); area.addEventListener('compositionstart', () => { composing = true; }); area.addEventListener('compositionend', () => { composing = false; });
            el.appendChild(core.el('<p class="ed-error" role="alert" hidden></p>'));
        }
        return { el, value() { const area = el.querySelector('textarea'); return known && !repair && area && area.value !== '' ? area.value : undefined; }, flush() { const area = el.querySelector('textarea'); if (!area) return true; const valid = !composing && textFits(area.value, 2048, 4096, true) && clean(area.value); const error = el.querySelector('.ed-error'); error.hidden = valid; error.textContent = valid ? '' : env.t('easydrag.ui.note_limit'); return valid; }, dispose() { closed = true; controller.abort(); el.remove(); } };
    }
    function dispose(root) { const rows = modals.get(root); if (rows) { Array.from(rows.values()).forEach(life => life.cancel()); rows.clear(); modals.delete(root); } }
    ED.metadata = { organization, folders, note, noteMarkup, bindNotes, publicationNote, textFits, clean, trim, dispose };
})();
