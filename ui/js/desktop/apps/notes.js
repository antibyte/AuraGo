(function () {
    'use strict';

    const instances = new Map();
    const SAVE_DEBOUNCE_MS = 800;
    const SIDECAR_DEBOUNCE_MS = 1000;
    const SSE_DEBOUNCE_MS = 500;
    const SEARCH_DEBOUNCE_MS = 250;
    const LIST_UPDATE_DEBOUNCE_MS = 150;
    const INDEX_TTL_MS = 60000;
    const NOTES_DIR = 'Documents/Notes';
    const SIDECAR_PATH = NOTES_DIR + '/notes.meta.json';
    const NEW_NOTE_ID = '__new__';
    const RECENT_WRITE_WINDOW_MS = 6000;
    const MAX_INDEX_FILES = 500;
    const MAX_INDEX_BYTES = 256 * 1024;
    const ENRICH_BATCH = 4;
    const DELETE_CONFIRM_MS = 5000;

    function defaultMeta() {
        return { version: 1, pinned: [], sort: 'modified', last_note: '' };
    }

    function fetchJSON(url, options) {
        return fetch(url, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}))
            .then(async resp => {
                let body = {};
                try { body = await resp.json(); } catch (_) {}
                if (!resp.ok) {
                    const err = new Error(body.error || body.message || ('HTTP ' + resp.status));
                    err.body = body;
                    throw err;
                }
                return body;
            });
    }

    function slugify(text) {
        return String(text || '')
            .toLowerCase()
            .replace(/[^\p{L}\p{N}]+/gu, '-')
            .replace(/^-+|-+$/g, '')
            .slice(0, 60);
    }

    function baseName(path) {
        const parts = String(path || '').split('/');
        return parts[parts.length - 1] || '';
    }

    function dirName(path) {
        const idx = String(path || '').lastIndexOf('/');
        return idx === -1 ? NOTES_DIR : path.slice(0, idx);
    }

    function collapseWhitespace(text) {
        return String(text || '').replace(/\s+/g, ' ').trim();
    }

    function snippetOf(content) {
        return collapseWhitespace(window.NotesFrontmatter.strip(content)).slice(0, 140);
    }

    function deriveTitle(content, fallbackName) {
        return window.NotesFrontmatter.deriveTitle(content, fallbackName);
    }

    function isRecentWrite(state, path) {
        if (!path) return false;
        const until = state.recentWrites.get(path);
        if (!until) return false;
        if (Date.now() > until) {
            state.recentWrites.delete(path);
            return false;
        }
        return true;
    }

    function markRecentWrite(state, path) {
        state.recentWrites.set(path, Date.now() + RECENT_WRITE_WINDOW_MS);
    }

    function readFile(state, path) {
        return state.api('/api/desktop/file?path=' + encodeURIComponent(path))
            .then(body => ({ content: String(body.content || ''), entry: body.entry || null }));
    }

    function hostActive(state) {
        const host = state.host;
        if (!host || !host.isConnected) return false;
        const active = document.activeElement;
        if (active && host.contains(active)) return true;
        const winEl = host.closest('.vd-window');
        if (!winEl) return true;
        const z = Number(winEl.style.zIndex || 0);
        const all = document.querySelectorAll('.vd-window');
        for (const other of all) {
            if (other === winEl || !other.offsetParent) continue;
            if (Number(other.style.zIndex || 0) > z) return false;
        }
        return true;
    }

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        const ctx = context || {};
        const esc = ctx.esc || (value => String(value == null ? '' : value));
        const t = ctx.t || (key => key);

        const state = {
            host,
            windowId,
            esc,
            t,
            api: ctx.api || fetchJSON,
            iconMarkup: ctx.iconMarkup || null,
            notify: ctx.notify || (() => {}),
            readonly: !!ctx.readonly,
            recordRecentFile: ctx.recordRecentFile || null,
            promptDialog: ctx.promptDialog || null,
            initialPath: (ctx.path && String(ctx.path)) || '',
            notes: [],
            noteCache: new Map(),
            meta: defaultMeta(),
            current: null,
            currentPath: '',
            currentIsNew: false,
            viewMode: 'split',
            searchQuery: '',
            sortMode: 'modified',
            tagFilter: '',
            contentResults: null,
            indexBuiltAt: 0,
            indexStale: false,
            saveTimer: null,
            sidecarTimer: null,
            sseTimer: null,
            searchTimer: null,
            listTimer: null,
            deleteTimer: null,
            recentWrites: new Map(),
            enrichToken: 0,
            sidecarWarned: false,
            disposed: false,
            cleanups: [],
            editor: null,
            list: null
        };
        instances.set(windowId, state);
        state.addCleanup = fn => { if (typeof fn === 'function') state.cleanups.push(fn); };

        host.innerHTML = `<div class="vd-notes-app" data-notes-root="${esc(windowId)}">
            <div class="vd-notes-loading">${esc(t('desktop.loading'))}</div>
        </div>`;

        boot(state);
    }

    async function boot(state) {
        await Promise.all([loadSidecar(state), refreshNotes(state)]);
        if (state.disposed) return;
        renderShell(state);
        enrichNotes(state);
        await selectInitialNote(state);
        if (state.disposed) return;
        state.list.update();
    }

    function renderShell(state) {
        const esc = state.esc;
        state.sortMode = state.meta.sort === 'name' ? 'name' : 'modified';
        state.host.innerHTML = `<div class="vd-notes-app" data-notes-root="${esc(state.windowId)}">
            <div class="vd-notes-side-slot" data-notes-side-slot></div>
            <div class="vd-notes-main-slot" data-notes-main-slot></div>
        </div>`;

        const sideSlot = state.host.querySelector('[data-notes-side-slot]');
        const mainSlot = state.host.querySelector('[data-notes-main-slot]');

        state.editor = window.NotesEditor.create(state, mainSlot);
        if (window.NotesToolbar) {
            window.NotesToolbar.mount(state, state.editor.toolbarSlot);
            window.NotesToolbar.bindShortcuts(state);
        }
        state.list = window.NotesList.mount(state, sideSlot);
        state.editor.setReadonly(state.readonly);

        wireEditorCallbacks(state);
        wireTagEditing(state);
        wireListCallbacks(state);
        wireActions(state);
        wireShortcuts(state);
        wireSSE(state);
    }

    function wireEditorCallbacks(state) {
        state.onTitleInput = value => {
            if (!state.current) return;
            state.current.title = value;
            markDirty(state);
            scheduleSave(state);
            scheduleListUpdate(state);
        };
        state.onContentInput = value => {
            if (!state.current) return;
            state.current.content = value;
            markDirty(state);
            scheduleSave(state);
        };
        state.onViewModeChange = () => { flushSave(state).catch(() => {}); };
        state.onReloadFromDisk = () => { reloadCurrentFromDisk(state).catch(() => {}); };
    }

    function wireListCallbacks(state) {
        state.onNewNote = () => { newNote(state); };
        state.onSelectNote = path => {
            if (!path || path === NEW_NOTE_ID) return;
            if (state.currentPath === path && !(state.current && state.current.isNew)) return;
            selectNote(state, path).catch(() => {});
        };
        state.onTogglePin = path => {
            if (state.readonly || !path || path === NEW_NOTE_ID) return;
            const pinned = state.meta.pinned || [];
            const idx = pinned.indexOf(path);
            if (idx >= 0) pinned.splice(idx, 1);
            else pinned.push(path);
            scheduleSidecarWrite(state);
            scheduleListUpdate(state);
        };
        state.onSortChange = mode => {
            state.meta.sort = mode === 'name' ? 'name' : 'modified';
            scheduleSidecarWrite(state);
        };
        state.onSearchInput = value => {
            if (state.searchTimer) { clearTimeout(state.searchTimer); state.searchTimer = null; }
            const q = value.trim();
            if (q.length >= 3) {
                state.searchTimer = setTimeout(() => {
                    state.searchTimer = null;
                    runContentSearch(state, q).catch(() => {});
                }, SEARCH_DEBOUNCE_MS);
            } else if (state.contentResults) {
                state.contentResults = null;
                scheduleListUpdate(state);
            }
        };
        state.onSearchSubmit = value => {
            if (state.searchTimer) { clearTimeout(state.searchTimer); state.searchTimer = null; }
            const q = value.trim();
            if (!q) {
                state.contentResults = null;
                state.list.update();
                return;
            }
            runContentSearch(state, q).catch(() => {});
        };
        state.onSearchClear = () => {
            if (state.searchTimer) { clearTimeout(state.searchTimer); state.searchTimer = null; }
            state.contentResults = null;
        };
    }

    function wireActions(state) {
        state.onActionClick = event => {
            const btn = event.target.closest('[data-action]');
            if (!btn) return;
            switch (btn.dataset.action) {
                case 'save': flushSave(state).catch(() => {}); break;
                case 'rename': renameNote(state).catch(err => notifyError(state, err)); break;
                case 'duplicate': duplicateNote(state).catch(err => notifyError(state, err)); break;
                case 'delete': requestDelete(state); break;
                case 'confirm-delete': confirmDelete(state).catch(err => { disarmDeleteConfirm(state); notifyError(state, err); }); break;
                case 'cancel-delete': disarmDeleteConfirm(state); break;
                case 'export': exportNote(state); break;
            }
        };
        const actionsRow = state.host.querySelector('.vd-notes-actions');
        if (actionsRow) actionsRow.addEventListener('click', state.onActionClick);
        state.addCleanup(() => {
            if (actionsRow && state.onActionClick) actionsRow.removeEventListener('click', state.onActionClick);
        });
    }

    function wireShortcuts(state) {
        state.onDocKeydown = event => {
            if (state.disposed || !hostActive(state)) return;
            if ((event.ctrlKey || event.metaKey) && !event.shiftKey && (event.key === 's' || event.key === 'S')) {
                event.preventDefault();
                flushSave(state).catch(() => {});
                return;
            }
            if (event.altKey && !event.ctrlKey && !event.metaKey && (event.key === 'n' || event.key === 'N')) {
                event.preventDefault();
                newNote(state);
                return;
            }
            if (event.key === '/' && !event.ctrlKey && !event.metaKey && !event.altKey) {
                const target = event.target;
                if (target && target.matches && target.matches('input, textarea, select, [contenteditable="true"]')) return;
                event.preventDefault();
                state.list.focusSearch();
                return;
            }
            if (event.key === 'Escape') {
                if (state.hideTagsPopover) state.hideTagsPopover();
                if (state.deleteTimer) disarmDeleteConfirm(state);
                const active = document.activeElement;
                const inSearch = active && active.matches && active.matches('[data-notes-search]');
                if (inSearch || state.contentResults) state.list.clearSearch();
            }
        };
        document.addEventListener('keydown', state.onDocKeydown);
        state.addCleanup(() => {
            if (state.onDocKeydown) document.removeEventListener('keydown', state.onDocKeydown);
        });
    }

    function wireSSE(state) {
        state.onDesktopEvent = payload => {
            if (!payload || payload.type !== 'desktop_changed') return;
            const detail = payload.payload || {};
            const op = detail.operation;
            if (op !== 'write_file' && op !== 'move_path' && op !== 'delete_path') return;
            const prefix = NOTES_DIR + '/';
            const paths = [detail.path, detail.old_path, detail.new_path].filter(Boolean);
            if (!paths.some(p => p === NOTES_DIR || p.startsWith(prefix))) return;
            if (paths.every(p => isRecentWrite(state, p))) return;
            if (state.sseTimer) clearTimeout(state.sseTimer);
            state.sseTimer = setTimeout(() => {
                state.sseTimer = null;
                handleDesktopChange(state, detail).catch(() => {});
            }, SSE_DEBOUNCE_MS);
        };
        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
            window.AuraSSE.on('virtual_desktop_event', state.onDesktopEvent);
        }
        state.addCleanup(() => {
            if (window.AuraSSE && typeof window.AuraSSE.off === 'function' && state.onDesktopEvent) {
                window.AuraSSE.off('virtual_desktop_event', state.onDesktopEvent);
            }
        });
    }

    async function handleDesktopChange(state, detail) {
        if (state.disposed) return;
        const op = detail.operation;
        if (op === 'delete_path' && detail.path) state.noteCache.delete(detail.path);
        if (op === 'write_file' && detail.path) { state.noteCache.delete(detail.path); state.indexStale = true; }
        if (op === 'move_path' && detail.old_path) {
            const cached = state.noteCache.get(detail.old_path);
            if (cached) {
                state.noteCache.delete(detail.old_path);
                state.noteCache.set(detail.new_path, cached);
            }
        }
        await refreshNotes(state);
        enrichNotes(state);
        const cur = state.current;
        if (cur && !cur.isNew && cur.path) {
            const touchesCurrent = (op === 'write_file' && detail.path === cur.path)
                || (op === 'move_path' && detail.old_path === cur.path)
                || (op === 'delete_path' && detail.path === cur.path);
            if (touchesCurrent) {
                if (op === 'delete_path') {
                    state.current = null;
                    state.currentPath = '';
                    state.editor.load(null);
                    const next = nextNoteAfter(state, cur.path);
                    if (next) await selectNote(state, next.path);
                } else if (op === 'move_path' && detail.new_path) {
                    cur.path = detail.new_path;
                    state.currentPath = detail.new_path;
                    state.editor.setPath(detail.new_path);
                    if (state.recordRecentFile) state.recordRecentFile(detail.new_path, 'notes');
                } else if (cur.dirty) {
                    state.editor.showBanner();
                } else {
                    await reloadCurrentFromDisk(state);
                }
            }
        }
        scheduleListUpdate(state);
    }

    async function reloadCurrentFromDisk(state) {
        const cur = state.current;
        if (!cur || cur.isNew || !cur.path) return;
        try {
            const data = await readFile(state, cur.path);
            if (state.disposed) return;
            cur.content = data.content;
            cur.tags = window.NotesFrontmatter.parse(data.content).tags;
            cur.title = deriveTitle(data.content, baseName(cur.path));
            cur.dirty = false;
            cur.lastSavedAt = Date.now();
            applyCachedContent(state, cur.path, data.content, cur.modTime || 0);
            state.editor.load(cur);
        } catch (err) {
            notifyError(state, err);
        }
    }

    // ── Notes list / metadata ────────────────────────────────────────────

    async function refreshNotes(state) {
        let files = [];
        try {
            const body = await state.api('/api/desktop/files?path=' + encodeURIComponent(NOTES_DIR) + '&recursive=true&limit=' + MAX_INDEX_FILES);
            files = (body.files || []).filter(f => f.type === 'file' && /\.md$/i.test(f.name || ''));
        } catch (_) {
            try {
                const body = await state.api('/api/desktop/files?path=' + encodeURIComponent(NOTES_DIR));
                files = (body.files || []).filter(f => f.type === 'file' && /\.md$/i.test(f.name || ''));
            } catch (_) { files = []; }
        }
        if (state.disposed) return;
        const next = [];
        const seen = new Set();
        for (const f of files) {
            if (!f.path || f.path === SIDECAR_PATH || seen.has(f.path)) continue;
            seen.add(f.path);
            const modTime = Date.parse(f.mod_time || f.modified || '') || 0;
            const cached = state.noteCache.get(f.path);
            next.push({
                path: f.path,
                name: f.name,
                title: cached ? cached.title : (f.name || '').replace(/\.md$/i, ''),
                tags: cached ? cached.tags.slice() : [],
                modTime,
                size: Number(f.size) || 0,
                snippet: cached ? cached.snippet : '',
                isNew: false,
                dirty: false
            });
        }
        if (state.currentIsNew) {
            const pseudo = state.notes.find(n => n.isNew);
            if (pseudo) next.push(pseudo);
        }
        const cur = state.current;
        if (cur && !cur.isNew) {
            const summary = next.find(n => n.path === cur.path);
            if (summary) summary.dirty = cur.dirty;
        }
        state.notes = next;
    }

    async function enrichNotes(state) {
        const token = ++state.enrichToken;
        const pending = () => state.notes.filter(n => !n.isNew && !state.noteCache.has(n.path));
        let batch = pending().slice(0, ENRICH_BATCH);
        while (batch.length) {
            if (state.disposed || token !== state.enrichToken) return;
            const results = await Promise.all(batch.map(note =>
                readFile(state, note.path).then(data => ({ note, data })).catch(() => ({ note, data: null }))
            ));
            if (state.disposed || token !== state.enrichToken) return;
            for (const { note, data } of results) {
                if (!data) {
                    state.noteCache.set(note.path, { content: null, title: note.name.replace(/\.md$/i, ''), tags: [], snippet: '', modTime: note.modTime });
                    continue;
                }
                applyCachedContent(state, note.path, data.content, note.modTime);
            }
            scheduleListUpdate(state);
            batch = pending().slice(0, ENRICH_BATCH);
        }
    }

    function applyCachedContent(state, path, content, modTime) {
        const fm = window.NotesFrontmatter.parse(content);
        const title = fm.title || deriveTitle(content, baseName(path));
        const entry = {
            content: content.length <= MAX_INDEX_BYTES ? content : null,
            title,
            tags: fm.tags.slice(),
            snippet: snippetOf(content),
            modTime: modTime || 0
        };
        state.noteCache.set(path, entry);
        const summary = state.notes.find(n => n.path === path);
        if (summary) {
            summary.title = title;
            summary.tags = entry.tags;
            summary.snippet = entry.snippet;
        }
        return entry;
    }

    function scheduleListUpdate(state) {
        if (state.listTimer) clearTimeout(state.listTimer);
        state.listTimer = setTimeout(() => {
            state.listTimer = null;
            if (!state.disposed && state.list) state.list.update();
        }, LIST_UPDATE_DEBOUNCE_MS);
    }

    function orderedNotes(state) {
        return window.NotesList.sortNotes(
            state.notes.filter(n => !n.isNew),
            state.sortMode,
            state.meta.pinned || []
        );
    }

    function nextNoteAfter(state, path) {
        const ordered = orderedNotes(state);
        const idx = ordered.findIndex(n => n.path === path);
        if (idx === -1) return ordered[0] || null;
        return ordered[idx + 1] || ordered[idx - 1] || null;
    }

    // ── Selection / editor loading ───────────────────────────────────────

    async function selectInitialNote(state) {
        if (state.initialPath && /\.md$/i.test(state.initialPath)) {
            await selectNote(state, state.initialPath);
            return;
        }
        const last = state.meta.last_note;
        if (last && state.notes.some(n => n.path === last)) {
            await selectNote(state, last);
            return;
        }
        const ordered = orderedNotes(state);
        if (ordered.length) {
            await selectNote(state, ordered[0].path);
            return;
        }
        state.editor.load(null);
    }

    async function selectNote(state, path) {
        if (state.current && !state.current.isNew && state.current.path === path) return;
        await flushSave(state);
        if (state.disposed) return;
        let content = '';
        let modTime = 0;
        const summary = state.notes.find(n => n.path === path);
        if (summary) modTime = summary.modTime;
        const cached = state.noteCache.get(path);
        if (cached && cached.content !== null && cached.content !== undefined && (!summary || cached.modTime === modTime)) {
            content = cached.content;
        } else {
            try {
                const data = await readFile(state, path);
                if (state.disposed) return;
                content = data.content;
                applyCachedContent(state, path, content, modTime);
            } catch (err) {
                notifyError(state, err);
                return;
            }
        }
        const fresh = state.noteCache.get(path) || {};
        state.currentIsNew = false;
        state.currentPath = path;
        state.current = {
            path,
            title: fresh.title || deriveTitle(content, baseName(path)),
            content,
            tags: (fresh.tags || []).slice(),
            isNew: false,
            dirty: false,
            lastSavedAt: modTime || null,
            modTime
        };
        state.editor.load(state.current);
        if (state.recordRecentFile) state.recordRecentFile(path, 'notes');
        if (state.meta.last_note !== path) {
            state.meta.last_note = path;
            scheduleSidecarWrite(state);
        }
        state.list.update();
    }

    // ── New / dirty / save ───────────────────────────────────────────────

    function newNote(state) {
        if (state.readonly || state.disposed) return;
        const flushPromise = flushSave(state);
        state.currentIsNew = true;
        state.currentPath = NEW_NOTE_ID;
        state.current = { path: '', title: '', content: '', tags: [], isNew: true, dirty: false, lastSavedAt: null };
        state.notes = state.notes.filter(n => !n.isNew);
        state.notes.push({
            path: NEW_NOTE_ID,
            name: state.t('desktop.notes_unsaved'),
            title: state.t('desktop.notes_unsaved'),
            tags: [],
            modTime: 0,
            size: 0,
            snippet: '',
            isNew: true,
            dirty: false
        });
        state.editor.load(state.current);
        state.editor.focusTitle();
        state.list.update();
        flushPromise.catch(() => {});
    }

    function markDirty(state) {
        const cur = state.current;
        if (!cur || cur.dirty) return;
        cur.dirty = true;
        state.editor.setDirty(true);
        const summary = state.notes.find(n => n.path === cur.path || (n.isNew && cur.isNew));
        if (summary) summary.dirty = true;
        scheduleListUpdate(state);
    }

    function scheduleSave(state) {
        if (state.readonly) return;
        if (state.saveTimer) clearTimeout(state.saveTimer);
        state.saveTimer = setTimeout(() => {
            state.saveTimer = null;
            commitSave(state).catch(() => {});
        }, SAVE_DEBOUNCE_MS);
    }

    async function flushSave(state) {
        if (state.saveTimer) { clearTimeout(state.saveTimer); state.saveTimer = null; }
        if (state.current && state.current.dirty) await commitSave(state);
    }

    async function commitSave(state) {
        const note = state.current;
        if (!note || !note.dirty || state.readonly || state.disposed) return;
        const content = note.content;
        const wasNew = note.isNew;
        let path = note.path;
        if (wasNew && !path) {
            path = await uniqueNotePath(state, note);
            note.path = path;
            note.isNew = false;
            state.currentPath = path;
            await ensureFolder(state);
        }
        state.editor.updateSaveStatus('saving');
        try {
            markRecentWrite(state, path);
            await state.api('/api/desktop/file', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path, content })
            });
            if (state.disposed) return;
            markRecentWrite(state, path);
            note.dirty = false;
            note.lastSavedAt = Date.now();
            note.modTime = note.lastSavedAt;
            upsertSummary(state, path, content, false);
            applyCachedContent(state, path, content, note.lastSavedAt);
            state.editor.updateSaveStatus('saved', note.lastSavedAt);
            state.editor.setDirty(false);
            state.editor.setPath(path);
            if (wasNew) {
                if (state.current === note) state.currentIsNew = false;
                const pseudoActive = !!(state.current && state.current.isNew);
                state.notes = state.notes.filter(n => !n.isNew || pseudoActive);
                state.meta.last_note = path;
                scheduleSidecarWrite(state);
                if (state.recordRecentFile) state.recordRecentFile(path, 'notes');
            }
            scheduleListUpdate(state);
        } catch (err) {
            if (wasNew) {
                note.isNew = true;
                note.path = '';
                if (state.current === note) state.currentPath = NEW_NOTE_ID;
            }
            state.editor.updateSaveStatus('error');
        }
    }

    async function uniqueNotePath(state, note) {
        const explicit = (note.title || '').trim();
        const derived = explicit || deriveTitle(note.content, '') || state.t('desktop.notes_unsaved');
        const base = slugify(derived) || 'note';
        const existing = new Set(state.notes.map(n => String(n.path).toLowerCase()));
        let candidate = NOTES_DIR + '/' + base + '.md';
        let counter = 2;
        while (existing.has(candidate.toLowerCase())) {
            candidate = NOTES_DIR + '/' + base + '-' + counter + '.md';
            counter++;
        }
        return candidate;
    }

    async function ensureFolder(state) {
        try {
            await state.api('/api/desktop/directory', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path: NOTES_DIR })
            });
        } catch (_) { /* already exists is fine */ }
    }

    function upsertSummary(state, path, content, dirty) {
        const fm = window.NotesFrontmatter.parse(content);
        const title = fm.title || deriveTitle(content, baseName(path));
        const summary = {
            path,
            name: baseName(path),
            title,
            tags: fm.tags.slice(),
            modTime: Date.now(),
            size: content.length,
            snippet: snippetOf(content),
            isNew: false,
            dirty: !!dirty
        };
        const idx = state.notes.findIndex(n => n.path === path);
        if (idx === -1) state.notes.push(summary);
        else state.notes[idx] = summary;
        return summary;
    }

    // ── Rename / duplicate / delete / export ─────────────────────────────

    function promptInline(state, title, value) {
        return new Promise(resolve => {
            const overlay = document.createElement('div');
            overlay.className = 'vd-notes-modal';
            overlay.innerHTML = `<div class="vd-notes-modal-card">
                <div class="vd-notes-modal-title">${state.esc(title)}</div>
                <input type="text" class="vd-notes-modal-input" maxlength="160">
                <div class="vd-notes-modal-actions">
                    <button type="button" class="vd-notes-btn" data-modal-cancel>${state.esc(state.t('desktop.cancel'))}</button>
                    <button type="button" class="vd-notes-btn vd-notes-btn-primary" data-modal-ok>${state.esc(state.t('desktop.save'))}</button>
                </div>
            </div>`;
            const input = overlay.querySelector('.vd-notes-modal-input');
            input.value = value || '';
            const done = result => {
                overlay.remove();
                document.removeEventListener('keydown', onKey, true);
                resolve(result);
            };
            const submit = () => done(input.value.trim());
            const onKey = e => {
                if (e.key === 'Escape') { e.stopPropagation(); done(null); }
                else if (e.key === 'Enter') { e.stopPropagation(); submit(); }
            };
            overlay.querySelector('[data-modal-ok]').addEventListener('click', submit);
            overlay.querySelector('[data-modal-cancel]').addEventListener('click', () => done(null));
            overlay.addEventListener('pointerdown', e => { if (e.target === overlay) done(null); });
            document.addEventListener('keydown', onKey, true);
            state.host.appendChild(overlay);
            input.focus();
            input.select();
        });
    }

    async function renameNote(state) {
        if (state.readonly || !state.current || state.current.isNew || !state.current.path) return;
        await flushSave(state);
        const oldPath = state.current.path;
        const oldName = baseName(oldPath).replace(/\.md$/i, '');
        const raw = typeof state.promptDialog === 'function'
            ? await state.promptDialog(state.t('desktop.notes_rename_prompt'), oldName)
            : await promptInline(state, state.t('desktop.notes_rename_prompt'), oldName);
        if (state.disposed || raw === null || raw === undefined) return;
        const clean = String(raw).trim().replace(/[\\/]+/g, ' ');
        if (!clean) return;
        const newName = /\.md$/i.test(clean) ? clean : clean + '.md';
        const newPath = dirName(oldPath) + '/' + newName;
        if (newPath === oldPath) return;
        if (state.notes.some(n => n.path.toLowerCase() === newPath.toLowerCase())) {
            throw new Error(state.t('desktop.notes_rename_exists', { name: newName }));
        }
        await state.api('/api/desktop/file', {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ old_path: oldPath, new_path: newPath })
        });
        markRecentWrite(state, newPath);
        const cur = state.current;
        cur.path = newPath;
        state.currentPath = newPath;
        const cached = state.noteCache.get(oldPath);
        if (cached) {
            state.noteCache.delete(oldPath);
            state.noteCache.set(newPath, cached);
        }
        const summary = state.notes.find(n => n.path === oldPath);
        if (summary) {
            summary.path = newPath;
            summary.name = newName;
        }
        state.meta.pinned = (state.meta.pinned || []).map(p => (p === oldPath ? newPath : p));
        if (state.meta.last_note === oldPath) state.meta.last_note = newPath;
        scheduleSidecarWrite(state);
        state.editor.setPath(newPath);
        if (state.recordRecentFile) state.recordRecentFile(newPath, 'notes');
        state.list.update();
    }

    async function duplicateNote(state) {
        if (state.readonly || !state.current || state.current.isNew || !state.current.path) return;
        await flushSave(state);
        const srcPath = state.current.path;
        const srcName = baseName(srcPath).replace(/\.md$/i, '');
        const existing = new Set(state.notes.map(n => String(n.path).toLowerCase()));
        let copyPath = dirName(srcPath) + '/' + srcName + ' (2).md';
        let counter = 3;
        while (existing.has(copyPath.toLowerCase())) {
            copyPath = dirName(srcPath) + '/' + srcName + ' (' + counter + ').md';
            counter++;
        }
        const data = await readFile(state, srcPath);
        markRecentWrite(state, copyPath);
        await state.api('/api/desktop/file', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: copyPath, content: data.content })
        });
        if (state.disposed) return;
        markRecentWrite(state, copyPath);
        upsertSummary(state, copyPath, data.content, false);
        applyCachedContent(state, copyPath, data.content, Date.now());
        state.list.update();
        await selectNote(state, copyPath);
    }

    function requestDelete(state) {
        if (state.readonly || !state.current) return;
        if (state.current.isNew) {
            discardNewNote(state);
            return;
        }
        const row = state.host.querySelector('.vd-notes-actions');
        if (!row || row.dataset.confirmDelete) return;
        row.dataset.confirmDelete = '1';
        const confirmBtn = document.createElement('button');
        confirmBtn.type = 'button';
        confirmBtn.className = 'vd-notes-btn vd-notes-btn-danger-solid';
        confirmBtn.dataset.action = 'confirm-delete';
        confirmBtn.textContent = state.t('desktop.notes_delete_confirm');
        const cancelBtn = document.createElement('button');
        cancelBtn.type = 'button';
        cancelBtn.className = 'vd-notes-btn';
        cancelBtn.dataset.action = 'cancel-delete';
        cancelBtn.textContent = state.t('desktop.cancel');
        row.appendChild(confirmBtn);
        row.appendChild(cancelBtn);
        state.deleteTimer = setTimeout(() => disarmDeleteConfirm(state), DELETE_CONFIRM_MS);
    }

    function disarmDeleteConfirm(state) {
        if (state.deleteTimer) { clearTimeout(state.deleteTimer); state.deleteTimer = null; }
        const row = state.host.querySelector('.vd-notes-actions');
        if (!row) return;
        delete row.dataset.confirmDelete;
        row.querySelectorAll('[data-action="confirm-delete"], [data-action="cancel-delete"]').forEach(btn => btn.remove());
    }

    function discardNewNote(state) {
        disarmDeleteConfirm(state);
        state.notes = state.notes.filter(n => !n.isNew);
        state.currentIsNew = false;
        state.current = null;
        state.currentPath = '';
        state.editor.load(null);
        const ordered = orderedNotes(state);
        if (ordered.length) selectNote(state, ordered[0].path).catch(() => {});
        state.list.update();
    }

    async function confirmDelete(state) {
        const cur = state.current;
        if (!cur || cur.isNew || !cur.path) { disarmDeleteConfirm(state); return; }
        const path = cur.path;
        disarmDeleteConfirm(state);
        await state.api('/api/desktop/file?path=' + encodeURIComponent(path), { method: 'DELETE' });
        if (state.disposed) return;
        markRecentWrite(state, path);
        state.noteCache.delete(path);
        state.notes = state.notes.filter(n => n.path !== path);
        state.meta.pinned = (state.meta.pinned || []).filter(p => p !== path);
        const next = nextNoteAfter(state, path);
        if (state.meta.last_note === path) state.meta.last_note = next ? next.path : '';
        scheduleSidecarWrite(state);
        state.current = null;
        state.currentPath = '';
        if (next) await selectNote(state, next.path);
        else {
            state.editor.load(null);
            state.list.update();
        }
        state.notify({ title: state.t('desktop.notification'), message: state.t('desktop.notes_deleted') });
    }

    function exportNote(state) {
        const cur = state.current;
        if (!cur) return;
        const name = cur.path
            ? baseName(cur.path)
            : (slugify(cur.title || deriveTitle(cur.content, '')) || 'note') + '.md';
        const blob = new Blob([cur.content || ''], { type: 'text/markdown;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = name;
        document.body.appendChild(link);
        link.click();
        link.remove();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
    }

    // ── Tags ─────────────────────────────────────────────────────────────

    function wireTagEditing(state) {
        state.getTags = () => (state.current && state.current.tags) || [];
        state.setTags = tags => {
            if (state.readonly || !state.current) return;
            state.current.tags = (tags || []).slice(0, 8);
            state.current.content = window.NotesFrontmatter.updateTags(state.current.content, state.current.tags);
            markDirty(state);
            scheduleSave(state);
            scheduleListUpdate(state);
        };
    }

    // ── Sidecar meta ─────────────────────────────────────────────────────

    async function loadSidecar(state) {
        try {
            const body = await readFile(state, SIDECAR_PATH);
            const parsed = JSON.parse(body.content || '');
            if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
                state.meta = {
                    version: 1,
                    pinned: Array.isArray(parsed.pinned) ? parsed.pinned.filter(p => typeof p === 'string') : [],
                    sort: parsed.sort === 'name' ? 'name' : 'modified',
                    last_note: typeof parsed.last_note === 'string' ? parsed.last_note : ''
                };
            }
        } catch (_) { /* missing or corrupt: keep defaults */ }
    }

    function scheduleSidecarWrite(state) {
        if (state.readonly) return;
        if (state.sidecarTimer) clearTimeout(state.sidecarTimer);
        state.sidecarTimer = setTimeout(() => {
            state.sidecarTimer = null;
            if (state.disposed) return;
            writeSidecar(state).catch(() => {});
        }, SIDECAR_DEBOUNCE_MS);
    }

    async function writeSidecar(state) {
        try {
            markRecentWrite(state, SIDECAR_PATH);
            await state.api('/api/desktop/file', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path: SIDECAR_PATH, content: JSON.stringify(state.meta) })
            });
            markRecentWrite(state, SIDECAR_PATH);
        } catch (err) {
            if (!state.sidecarWarned) {
                state.sidecarWarned = true;
                notifyError(state, err);
            }
        }
    }

    // ── Content search ───────────────────────────────────────────────────

    async function ensureContentIndex(state) {
        if (state.indexBuiltAt && !state.indexStale && Date.now() - state.indexBuiltAt < INDEX_TTL_MS) return;
        state.indexStale = false;
        await refreshNotes(state);
        await enrichNotes(state);
        state.indexBuiltAt = Date.now();
    }

    async function runContentSearch(state, query) {
        const q = String(query || '').trim().toLowerCase();
        if (!q) {
            state.contentResults = null;
            if (state.list) state.list.update();
            return;
        }
        await ensureContentIndex(state);
        if (state.disposed) return;
        const results = [];
        for (const note of state.notes) {
            if (note.isNew) continue;
            const cached = state.noteCache.get(note.path);
            if (!cached) continue;
            const content = cached.content || '';
            const idx = content.toLowerCase().indexOf(q);
            if (idx === -1 && !(cached.title || '').toLowerCase().includes(q)) continue;
            results.push({
                path: note.path,
                title: cached.title || note.title,
                name: note.name,
                snippet: idx >= 0 ? searchSurround(content, idx, q.length) : cached.snippet,
                modTime: note.modTime
            });
            if (results.length >= 50) break;
        }
        state.contentResults = results;
        if (state.list) state.list.update();
    }

    function searchSurround(content, idx, len) {
        const start = Math.max(0, idx - 60);
        const end = Math.min(content.length, idx + len + 60);
        return (start > 0 ? '…' : '') + collapseWhitespace(content.slice(start, end)) + (end < content.length ? '…' : '');
    }

    function notifyError(state, err) {
        state.notify({ title: state.t('desktop.notification'), message: (err && err.message) || String(err) });
    }

    // ── Lifecycle ────────────────────────────────────────────────────────

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        if (state.current && state.current.dirty && !state.readonly) {
            commitSave(state).catch(() => {});
        }
        state.disposed = true;
        if (state.saveTimer) { clearTimeout(state.saveTimer); state.saveTimer = null; }
        if (state.sidecarTimer) { clearTimeout(state.sidecarTimer); state.sidecarTimer = null; }
        if (state.sseTimer) { clearTimeout(state.sseTimer); state.sseTimer = null; }
        if (state.searchTimer) { clearTimeout(state.searchTimer); state.searchTimer = null; }
        if (state.listTimer) { clearTimeout(state.listTimer); state.listTimer = null; }
        if (state.deleteTimer) { clearTimeout(state.deleteTimer); state.deleteTimer = null; }
        if (state.current && state.current.dirty && !state.readonly) {
            commitSave(state).catch(() => {});
        }
        if (state.editor) state.editor.dispose();
        state.cleanups.forEach(fn => {
            try { fn(); } catch (_) {}
        });
        state.cleanups = [];
        instances.delete(windowId);
    }

    window.NotesApp = { render, dispose, instances };
})();
