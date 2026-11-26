// HomeDash mount (F2; design-frontend 1.4; M4 contract C2, C7, C8). mount(host, options) is the composition root of one HomeDash window. Windows of one JS realm
// share one API client, stream and store per base path (realmFor), plus the autosave lanes (one per dashboard, they outlive a closed window while dirty) and the
// clipboard channel. It loads bootstrap and the widget catalog, lists the dashboards, creates the starter (template "system") or an empty dashboard, deletes one,
// opens one dashboard in an hd-view and keeps its entities subscribed while the host's visibility signal says visible, refetches the list 250 ms after
// dashboard_changed (once per realm) and reconciles the open dashboard by revision. The edit surface (enterEdit, exitEdit, run, contextMenu, beforeClose) mounts
// E.designer on the open view; one window per dashboard edits (store.docs lock). The host draws its chrome from the frozen 'state' object (state.edit, C2).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.api || !E.stream || !E.store || !E.catalog || !E.view || !E.command || !E.save || !E.clipboard) throw new Error('hd-app.js: load the HomeDash engine first');
    const realms = new Map(), LIST_COALESCE_MS = 250, PHASES = Object.freeze(['saved', 'pending', 'saving', 'retrying', 'invalid', 'conflict', 'readonly', 'gone']);
    const safeLocalStorage = () => { try { return typeof localStorage !== 'undefined' ? localStorage : null; } catch (err) { return null; } };
    function realmFor(base, transport, timers, storage) {
        const have = realms.get(base);
        if (have) return have;
        const tm = timers || {}, api = E.api.create({ base, transport }), clock = { setTimeout: tm.setTimeout, clearTimeout: tm.clearTimeout, now: tm.now };
        const diagnose = () => api.bootstrap().then(() => 'ok', err => (err.code === 'homedash_disabled' ? 'disabled' : err.status === 401 ? 'auth_lost' : 'error'));
        const stream = E.stream.shared(Object.assign({ api, random: tm.random, diagnose }, clock));
        const store = E.store.shared(Object.assign({ api, stream, raf: tm.raf, caf: tm.caf }, clock));
        const setT = tm.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = tm.clearTimeout || (id => clearTimeout(id)), win = typeof window !== 'undefined' ? window : null;
        const realm = { api, stream, store, commands: E.command.shared(Object.assign({ api, stream, store }, clock)), list: null, load: null, next: null, timer: 0, listeners: new Set(), refresh: null,
            lanes: new Map(), roDesktop: false, roHome: false, clipboard: E.clipboard.channel({ storage: storage || safeLocalStorage(), win }) };
        const run = () => { const p = api.dashboards().then(answer => {
            realm.list = Object.freeze((answer && Array.isArray(answer.dashboards) ? answer.dashboards : []).filter(d => d && typeof d.id === 'string'));
            for (const fn of [...realm.listeners]) fn(realm.list);
            return realm.list;
        }).finally(() => { if (realm.load === p) realm.load = null; }); return p; };
        // A refresh asked for while a fetch is in flight may follow a mutation that fetch predates: exactly one more fetch runs after it settles and all such callers share it.
        realm.refresh = () => {
            if (realm.next) return realm.next;
            if (!realm.load) { realm.load = run(); return realm.load; }
            realm.next = realm.load.catch(() => {}).then(() => { realm.next = null; realm.load = run(); return realm.load; });
            return realm.next;
        };
        store.on('dashboards', () => { clearT(realm.timer); realm.timer = setT(() => { realm.timer = 0; realm.refresh().catch(() => {}); }, LIST_COALESCE_MS); });
        // A tab that unloads flushes every dirty lane (keepalive PUT, up to 61 440 bytes). Changes keepalive cannot guarantee (a body over the gate, an open PUT with a
        // queued follow-up) make the browser ask before a close or reload (beforeunload).
        if (win && typeof win.addEventListener === 'function') {
            win.addEventListener('pagehide', () => { for (const l of realm.lanes.values()) if (!l.disposed && l.state().dirty) l.flush({ keepalive: true }); });
            win.addEventListener('beforeunload', ev => {
                let risk = false; for (const l of realm.lanes.values()) { try { if (l.atRisk()) { risk = true; break; } } catch (err) { risk = true; break; } }
                if (!risk || !ev) return; if (typeof ev.preventDefault === 'function') ev.preventDefault(); ev.returnValue = '';
            });
        }
        realms.set(base, realm);
        return realm;
    }
    function mount(hostEl, options) {
        const o = options || {}, t = typeof o.t === 'function' ? o.t : (k => k), lang = o.lang || 'en';
        const realm = realmFor(o.base || '/api/desktop/homedash', o.transport, o.timers, o.storage), api = realm.api, store = realm.store;
        const bus = E.core.emitter(), life = E.core.bag(), vis = o.visibility || { visible: () => true, subscribe: () => () => {} }, ownerId = E.core.uid('hd-win'), tm = o.timers || {};
        const root = E.el('div', { class: 'hd-root hd-backdrop', 'data-hd-host': o.host || 'desktop', 'data-hd-mode': 'view' });
        hostEl.appendChild(root);
        const watch = store.watch(ownerId, { ids: [], active: vis.visible() });
        store.docs.presence(ownerId, { focus: () => { if (o.ui && typeof o.ui.focusWindow === 'function') o.ui.focusWindow(); } }); life.add(() => store.docs.presence(ownerId, null));
        life.add(vis.subscribe(on => { watch.setActive(on); if (on && state.mode === 'home') refreshTemplates(); }));
        let state = Object.freeze({ mode: 'loading', error: null, notice: null, busy: false, dashboards: null, dashboard: null, pageId: null, connection: store.connection(), desktopReadonly: !!o.readonly, banners: [], templates: [], noticeSource: null, noticeCount: 0 });
        let hostRo = !!o.readonly, homeRo = false; // live host read-only flag: starts as the open-time snapshot, kept current by setReadonly; homeRo: bootstrap says HomeDash itself is read-only
        let wasLive = false, haState = ''; // transitions that clear read-only locks / re-read templates
        let catalog = null, view = null, doc = null, disposed = false, openSeq = 0, late = null; // late: a list that arrived while an open was in flight
        let designer = null, viewBp = null, fitOv = null, lastSnap = {}, watched = null, offLane = null, sheetInst = null;
        const canEditOpt = o.canEdit !== false;
        const laneOf = () => { const l = doc ? realm.lanes.get(doc.id) : null; return l && !l.disposed ? l : null; }, docEl = () => root.ownerDocument;
        // ---- edit state (C2): recomputed in every set(); actions: {available hides, enabled greys, checked}
        function actionsOf(s, e) {
            const dash = s.mode === 'dashboard' && !!s.dashboard, ro = !!s.desktopReadonly, free = !ro && !s.busy, dd = !!E.ddialogs, design = e.mode === 'design', ds = designer ? designer.state() : null;
            const A = (available, enabled, checked) => Object.freeze({ available: !!available, enabled: !!(available && enabled), checked: !!checked }), sel = ds ? ds.selection.length : 0, pages = view && view.doc() ? view.doc().pages.length : 0;
            const out = { new: A(dd, free), new_starter: A(true, free), new_empty: A(true, free), new_ha: A((s.templates || []).includes('home_assistant'), free), open: A(true, true),
                rename: A(dd, dash && !ro), look: A(dd, dash && !ro), add_page: A(dd, dash && !ro), export: A(dd, dash), import: A(dd, !ro), duplicate_dashboard: A(true, dash && !ro), delete_dashboard: A(true, dash && !ro),
                undo: A(true, design && ds.canUndo), redo: A(true, design && ds.canRedo), cut: A(true, design && sel > 0), copy: A(true, design && sel > 0), paste: A(true, design && e.hasClipboard), duplicate: A(true, design && sel > 0), delete: A(true, design && sel > 0),
                select_all: A(true, design && ds.items.length > 0), select_none: A(true, design && sel > 0), reset_layout: A(true, design && ds.bp !== 'lg' && !!ds.override[ds.bp]), retry_save: A(true, ['retrying', 'readonly', 'pending'].includes(e.phase)),
                resolve_conflict: A(true, design && !!ds.conflict), show_draft: A(true, design && e.drafts > 0), add_widget: A(!!E.palette, design), focus_inspector: A(!!E.inspector, design), toggle_palette: A(!!E.palette, design), toggle_inspector: A(!!E.inspector, design),
                edit: A(true, e.canEdit && !e.editElsewhere, design), exit_edit: A(true, design), show_editor: A(e.editElsewhere, e.editElsewhere), layout_auto: A(true, dash && !design, !design && e.bp === null), fit: A(true, dash && !design, e.fit === 'fit_screen'),
                present: A(true, dash), page_prev: A(true, pages > 1), page_next: A(true, pages > 1) };
            for (const b of ['lg', 'md', 'sm']) out['layout_' + b] = A(true, dash, e.bp === b);
            return Object.freeze(out);
        }
        function editState(s) {
            const id = s.dashboard && s.dashboard.id, ed = id ? store.docs.editor(id) : null, d = view && view.doc(), ds = designer ? designer.state() : null, l = laneOf(), ls = l ? l.state() : null;
            const reason = !E.designer || !canEditOpt ? 'no_designer' : s.mode === 'disabled' ? 'disabled' : s.mode !== 'dashboard' ? 'not_dashboard' : s.desktopReadonly ? 'readonly' : homeRo ? 'homedash_readonly' : s.dashboard && s.dashboard.schemaTooNew ? 'schema_too_new' : '';
            const e = { mode: designer ? 'design' : 'view', canEdit: !reason, reason, editElsewhere: !!(ed && ed !== ownerId), dirty: !!(ls && ls.dirty), saving: !!(ls && ls.saving), phase: ls && PHASES.includes(ls.phase) ? ls.phase : 'saved', nextRetryAt: ls ? ls.nextRetryAt : 0, errorCode: ls && ls.error ? ls.error.code : '',
                canUndo: !!(ds && ds.canUndo), canRedo: !!(ds && ds.canRedo), undoLabel: ds ? ds.undoLabel : '', redoLabel: ds ? ds.redoLabel : '', selection: ds ? ds.selection : [], bp: ds ? ds.bp : (d ? viewBp : null), fit: !d ? null : designer ? 'scroll' : fitOv || d.grid.fit,
                drafts: ds ? ds.drafts : 0, hasClipboard: !!realm.clipboard.has(), actions: null };
            e.actions = actionsOf(s, e); return Object.freeze(e);
        }
        function followLane() {   // the lane of the open dashboard drives the save chip of the view mode too
            const l = laneOf(); if (l === watched) return; if (offLane) offLane(); watched = l; offLane = null;
            if (l) offLane = l.on('state', () => { bumpRevision(l.id, l.revision()); set({}); });
        }
        function bumpRevision(id, rev) {
            if (!doc || doc.id !== id || !(rev > Number(doc.revision))) return;
            doc = Object.assign({}, doc, { revision: rev }); if (state.dashboard && state.dashboard.id === id) state = Object.freeze(Object.assign({}, state, { dashboard: Object.freeze(Object.assign({}, state.dashboard, { revision: rev })) }));
        }
        function syncEmpty() { if (!view || designer) return; const e = state.edit; view.setEmptyAction(e && e.canEdit && !e.editElsewhere ? { cmd: 'edit', label: t('homedash.designer.edit') } : null); }
        function set(patch) {
            if (disposed) return;
            const next = Object.assign({}, state, patch);
            // While the designer edits, the window title follows the name of the document in the lane (a rename of your own or of a foreign save).
            const ld = designer && next.dashboard ? laneOf() : null, ln = ld && ld.doc() && ld.doc().name; if (typeof ln === 'string' && ln && ln !== next.dashboard.name) next.dashboard = Object.freeze(Object.assign({}, next.dashboard, { name: ln }));
            next.banners = E.view.banners({ mode: next.mode, connection: next.connection, sources: store.sources(), usedSources: view ? view.sources() : [], notice: next.notice, noticeCount: next.noticeCount, desktopReadonly: next.desktopReadonly, schemaTooNew: !!(next.dashboard && next.dashboard.schemaTooNew) });
            next.edit = editState(next); state = Object.freeze(next); E.core.setAttr(root, 'data-hd-mode', next.edit.mode); followLane(); syncEmpty(); bus.emit('state', state);
        }
        state = Object.freeze(Object.assign({}, state, { edit: editState(state) }));
        life.add(() => { if (offLane) offLane(); });
        const context = () => { if (typeof o.onContext === 'function') o.onContext({ dashboard_id: doc ? doc.id : null, page_id: view && doc ? view.pageId() : null }); };
        const hook = (type, fn) => { store.on(type, fn); life.add(() => store.off(type, fn)); };
        // A 403 read_only lock must not outlive a policy change: it is dropped when the stream is live again after a reconnect, on every bootstrap re-read and when the desktop becomes writable.
        life.add(realm.commands.on('readonly', ev => { if (ev && ev.desktop) applyReadonly(true); }));
        hook('connection', c => { const live = c.phase === 'live'; if (live && !wasLive) realm.commands.clearLocks(); wasLive = live; if (view) view.setConnection(c); set({ connection: c }); if (c.phase === 'stopped' && c.reason === 'disabled') recheck(); });
        hook('sources', list => {
            const ha = (Array.isArray(list) ? list : []).find(x => x && x.source === 'ha'), now = ha && typeof ha.state === 'string' ? ha.state : '';
            const becameLive = now === 'live' && haState !== 'live'; haState = now;
            set({}); if (becameLive && state.mode === 'home') refreshTemplates();
        });
        hook('lock', () => set({}));
        hook('doc', ev => {   // another window of this realm saved: a view window shows the cached document when it is newer than its own
            if (disposed || designer || !doc || !view || state.mode !== 'dashboard' || !ev || ev.id !== doc.id) return;
            const c = store.docs.get(ev.id); if (c && c.revision > Number(doc.revision)) applyCached(c);
        });
        life.add(realm.clipboard.onChange(() => set({})));
        const onList = list => { if (state.mode === 'loading') late = list; else reconcile(list); };
        realm.listeners.add(onList); life.add(() => realm.listeners.delete(onList)); life.add(() => watch.release());
        function fail(err, fallback) {
            const e = E.api.normalizeError(err);
            if (e.code === 'aborted' || disposed) return;
            if (e.code === 'homedash_disabled') { closeView(); set({ mode: 'disabled', busy: false }); return; }
            if (e.code === 'desktop_readonly') applyReadonly(true);
            fallback(e);
        }
        const templatesOf = b => (Array.isArray(b && b.templates) ? b.templates.filter(x => typeof x === 'string') : []);
        async function boot() {
            set({ mode: 'loading', error: null, notice: null });
            const id = o.dashboardId || null, docP = id ? api.dashboard(id).then(a => ({ a }), e => ({ e })) : null;   // the document does not wait for bootstrap
            try {
                const [b, first] = await Promise.all([api.bootstrap(), store.widgetCatalog()]);
                if (disposed) return;
                store.setSources(b.sources); realm.commands.clearLocks();
                realm.stream.resume(); // a stream stopped by revoked disabled/auth_lost restarts once bootstrap answers again (no-op otherwise)
                const ro = hostRo || b.desktop_readonly === true; homeRo = b.readonly === true;
                syncLanes(ro, homeRo); store.setDesktopReadonly(ro);
                const cat = first && first.hash === b.catalog_hash ? first : await store.widgetCatalog(b.catalog_hash);
                if (disposed) return;
                catalog = E.catalog.create(cat);
                set({ desktopReadonly: ro, templates: templatesOf(b) });
                if (id) await open(id, o.pageId, docP); else await home(null);
            } catch (err) { fail(err, e => set({ mode: 'error', error: Object.freeze({ code: e.code, status: e.status }) })); }
        }
        // Bootstrap again: the template list (and the source states) follow the server, e.g. once Home Assistant is configured.
        function refreshTemplates() {
            return api.bootstrap().then(b => { if (!disposed) { store.setSources(b.sources); realm.commands.clearLocks(); homeRo = b.readonly === true; syncLanes(undefined, homeRo); set({ templates: templatesOf(b) }); } }, () => {});
        }
        function recheck() { api.bootstrap().then(() => realm.stream.resume(), err => fail(err, () => {})); }
        async function home(notice) {
            if (disposed) return;
            openSeq += 1; closeView(); late = null;
            set({ mode: 'home', busy: false, dashboard: null, pageId: null, notice: notice || null, dashboards: realm.list });
            context();
            try { set({ dashboards: await realm.refresh() }); } catch (err) { fail(err, () => set({ notice: 'list_failed', dashboards: realm.list || [] })); }
            refreshTemplates();
        }
        async function open(id, pageId, pre) {
            const mine = ++openSeq;
            if (state.mode !== 'dashboard') set({ mode: 'loading', notice: null });
            let answer;
            try { if (pre) { const r = await pre; if (r.e) throw r.e; answer = r.a; } else answer = await api.dashboard(id); }
            catch (err) { if (mine !== openSeq) return; fail(err, e => home(e.code === 'dashboard_not_found' || e.code === 'bad_request' ? 'dashboard_missing' : 'open_failed')); return; }
            if (mine !== openSeq || disposed) return;
            show(answer && answer.dashboard, pageId, !!(answer && answer.schema_too_new === true));
            const l = late; late = null; if (l && state.mode === 'dashboard') reconcile(l);
        }
        function show(d, pageId, schemaTooNew) {
            if (!d || typeof d.id !== 'string' || !catalog) { home('open_failed'); return; }
            if (!view) view = E.view.create({ root, store, catalog, t, lang, host: o.host || 'desktop', scheme: o.scheme, motion: o.motion, raf: o.timers && o.timers.raf, caf: o.timers && o.timers.caf,
                measure: o.measure, commands: realm.commands, onEntities: ids => watch.set(ids), onPage: p => { watch.set(view.entityIds()); set({ pageId: p }); context(); } });
            if (!doc || doc.id !== d.id) { viewBp = null; fitOv = null; view.setBreakpoint(null); view.setFit(null); }
            const l = realm.lanes.get(d.id), local = !!(l && !l.disposed && l.state().dirty);   // a dirty lane that outlived its window holds the newer local document
            store.docs.put(d.id, d); doc = d; view.show(local ? E.doc.forSave(l.doc(), { catalog }) : d, pageId); view.setConnection(store.connection()); view.setReadonly(state.desktopReadonly);
            watch.set(view.entityIds());
            set({ mode: 'dashboard', notice: null, noticeSource: null, noticeCount: 0, busy: false, pageId: view.pageId(),
                dashboard: Object.freeze({ id: d.id, name: typeof d.name === 'string' && d.name ? d.name : d.id, revision: d.revision, savedBy: d.saved_by, schemaTooNew }) });
            context();
        }
        function applyCached(c) {
            doc = c.doc; view.update(c.doc);
            set({ dashboard: Object.freeze(Object.assign({}, state.dashboard, { name: typeof c.doc.name === 'string' && c.doc.name ? c.doc.name : c.doc.id, revision: c.revision, savedBy: c.doc.saved_by })) });
        }
        function closeView() { stopDesign(); if (view) { view.destroy(); view = null; } doc = null; viewBp = null; fitOv = null; watch.set([]); }
        function dropLane(l) { if (!l) return; l.dispose(); if (realm.lanes.get(l.id) === l) realm.lanes.delete(l.id); }
        function reconcile(list) {
            if (disposed) return;
            if (state.mode === 'home') { set({ dashboards: list }); return; }
            if (state.mode !== 'dashboard' || !doc) return;
            const s = list.find(x => x.id === doc.id);
            if (!s) { if (designer) { designer.reconcile({ gone: true }); return; } dropLane(laneOf()); store.docs.drop(doc.id); home('dashboard_deleted'); return; }
            const rev = Number(s.revision); if (!(rev > Number(doc.revision))) return;
            if (designer) { designer.reconcile({ revision: rev, savedBy: s.saved_by, updatedAt: s.updated_at }); return; }
            const c = store.docs.get(doc.id); if (c && c.revision >= rev) applyCached(c); else open(doc.id, view ? view.pageId() : null);
        }
        async function create(kind) {
            if (state.busy || state.desktopReadonly || !catalog) return;
            if (designer && (await exitEdit()) !== 'left') return;
            set({ busy: true, notice: null });
            try {
                const answer = kind === 'starter' ? await api.createFromTemplate('system', t('homedash.home.starter_name'))
                    : kind === 'home_assistant' ? await api.createFromTemplate('home_assistant', { name: t('homedash.home.ha_name'), unassigned_title: t('homedash.home.ha_unassigned') })
                    : await api.createDocument({ name: t('homedash.home.empty_name'), pages: [{ id: 'main', name: t('homedash.home.page_main') }] });
                if (disposed) return;
                openSeq += 1; show(answer && answer.dashboard, null, false);
                if (kind === 'home_assistant' && Number(answer && answer.omitted) > 0) set({ notice: 'template_omitted', noticeCount: Number(answer.omitted) });
                realm.refresh().catch(() => {});
            } catch (err) { fail(err, e => set({ busy: false, noticeSource: e.code === 'source_unavailable' ? ((e.data && e.data.source) || 'ha') : null,
                notice: e.code === 'limit_exceeded' ? 'limit' : e.code === 'desktop_readonly' ? 'readonly' : e.code === 'template_empty' ? 'template_empty' : e.code === 'source_unavailable' ? 'source_unavailable' : 'create_failed' })); }
        }
        async function remove(summary) {
            if (state.busy || state.desktopReadonly || !summary || typeof summary.id !== 'string') return false;
            set({ busy: true, notice: null });
            try {
                await api.deleteDashboard(summary.id, summary.revision); dropLane(realm.lanes.get(summary.id)); store.docs.drop(summary.id);
                set({ busy: false }); await realm.refresh().catch(() => {}); return true; }
            catch (err) {
                fail(err, e => set({ busy: false, notice: e.code === 'revision_conflict' ? 'delete_conflict' : e.code === 'desktop_readonly' ? 'readonly' : 'delete_failed' }));
                realm.refresh().catch(() => {}); return false;
            }
        }
        // Lanes of the realm do not send while the desktop or HomeDash is read-only (the server would answer 403); lifting both resumes them.
        function syncLanes(desktop, home) { if (desktop !== undefined) realm.roDesktop = !!desktop; if (home !== undefined) realm.roHome = !!home; const on = realm.roDesktop || realm.roHome; for (const l of realm.lanes.values()) if (!l.disposed) l.setReadonly(on); }
        function applyReadonly(ro) { syncLanes(ro); store.setDesktopReadonly(ro); if (view) view.setReadonly(ro); if (ro !== state.desktopReadonly) set({ desktopReadonly: ro }); if (ro && designer) exitEdit({ force: true }); }
        function setReadonly(flag) { hostRo = !!flag; applyReadonly(hostRo); if (!hostRo) { realm.commands.clearLocks('virtual_desktop.readonly'); for (const l of realm.lanes.values()) l.retryNow(); } } // the host's live flag; a retry's boot() reads it, not the open-time snapshot
        // ---- edit surface (C2)
        const appSheet = () => { if (!sheetInst) { sheetInst = E.sheet.create(root); life.add(() => sheetInst.destroy()); } return sheetInst; }, sheetFor = () => (designer ? designer.sheet() : appSheet());
        function snapshot() { if (root.isConnected !== false) lastSnap = E.widgets.paletteSnapshot(root, catalog ? catalog.paletteTokens() : []); return lastSnap; }
        function afterLane(l) { if (!l || l.disposed) return; const s = l.state(); if (!l.owner && !s.dirty && !s.saving) dropLane(l); }   // a saved, detached lane is disposed
        function laneFor(id, revision, ndoc) {
            const have = realm.lanes.get(id); if (have && !have.disposed) return have;
            const l = E.save.lane({ api, id, revision, doc: ndoc, prepare: d => E.doc.forSave(d, { catalog, snapshot: snapshot() }), timers: o.timers });
            realm.lanes.set(id, l); l.setReadonly(realm.roDesktop || realm.roHome);
            l.on('saved', ev => { bumpRevision(id, ev.revision); store.docs.put(id, Object.assign({}, ev.body, { revision: ev.revision })); afterLane(l); });
            return l;
        }
        async function enterEdit(eo) {
            if (disposed || designer || !view || !doc || !state.edit.canEdit || state.edit.editElsewhere) return false;
            const id = doc.id; if (!store.docs.lock(id, ownerId)) { set({}); return false; }
            const l = laneFor(id, Number(doc.revision), view.doc()); l.attach(ownerId); let d = null;
            try {
                d = E.designer.create({ root, view, store, api, catalog, lane: l, clipboard: realm.clipboard, t, lang, ownerId, doc: l.doc(), widgetId: eo && eo.widgetId, ui: o.ui || {}, timers: tm, raf: tm.raf, caf: tm.caf,
                    summary: () => (realm.list || []).find(x => x.id === id) || null, request: (action, arg) => run(action, arg), onState: () => { if (designer) set({}); }, openCopy: (newId, name) => openCopy(newId, name) });
            } catch (err) { console.error('HomeDash: the designer could not start', err); store.docs.unlock(id, ownerId); l.detach(ownerId); afterLane(l); set({}); return false; }
            designer = d; set({}); context(); return true;
        }
        function stopDesign() {   // synchronous teardown shared by exit, close, dispose and read-only; the lane stays in the realm
            const d = designer; if (!d) return null; const l = laneOf(), act = docEl().activeElement, had = !!(act && root.contains(act));
            designer = null; d.destroy(); if (view) { view.setBreakpoint(viewBp); view.setFit(fitOv); }   // the designer forced lg and scrolling: the view's own override returns
            if (doc) store.docs.unlock(doc.id, ownerId); if (l) l.detach(ownerId); return { lane: l, had };
        }
        function focusRoot() { if (typeof root.focus !== 'function') return; if (!root.hasAttribute('tabindex')) root.setAttribute('tabindex', '-1'); root.focus({ preventScroll: true }); }
        async function exitEdit(xo) {
            const d = designer; if (!d) return 'left'; let verdict = 'leave';
            if (!(xo && xo.force)) { verdict = await d.prepareExit(); if (designer !== d) return 'left'; if (verdict === 'keep') return 'kept'; }
            const r = stopDesign(), l = r && r.lane, id = doc ? doc.id : null, pid = view ? view.pageId() : null;
            if (verdict === 'discard' && l) { if (view) view.update(E.doc.forSave(l.saved(), { catalog })); dropLane(l); store.docs.drop(id); set({}); if (r.had) focusRoot(); await open(id, pid); return 'left'; }
            if (view && doc) view.update(E.doc.forSave(l ? l.doc() : view.doc(), { catalog }));   // drafts are dropped on leaving
            afterLane(l); set({}); context(); if (r && r.had) focusRoot(); return 'left';
        }
        const guarded = fn => (designer ? exitEdit().then(r => (r === 'left' ? fn() : undefined)) : fn());
        async function beforeClose() {
            if (disposed) return true; const l = laneOf();
            if (designer) { const n = E.doc.drafts(designer.doc(), catalog).size; if (n) { const c = await E.save.draftsSheet({ sheet: designer.sheet(), t, count: n }); if (c !== 'leave') { designer.run('show_draft'); return false; } } }
            if (!l) return true;
            l.flush({ keepalive: true }); const phase = await l.settle(1500); if (phase === 'saved' && !l.state().dirty) return true;
            const st = l.state(), c = await E.save.leaveSheet({ sheet: sheetFor(), t, phase: st.phase, reason: st.error ? E.save.errorText(t, st.error) : '', mode: 'close' });
            if (c === 'leave') return true; if (c === 'discard') { const id = l.id; dropLane(l); store.docs.drop(id); return true; } return false;
        }
        function present() {
            if (!view || !doc) return false;
            if (typeof root.requestFullscreen === 'function') { const p = root.requestFullscreen(); if (p && typeof p.catch === 'function') p.catch(() => view && view.toast(t('homedash.designer.present_failed'), { tone: 'warn' })); return true; }
            view.toast(t('homedash.designer.present_failed'), { tone: 'warn' }); return false;
        }
        async function duplicateDashboard() {
            if (!doc || state.busy || state.desktopReadonly) return false; const id = doc.id;
            try {
                const l = laneOf(); if (designer && l) await l.flush();
                set({ busy: true }); const answer = await api.duplicateDashboard(id), d = answer && answer.dashboard; if (!d || disposed) { set({ busy: false }); return false; }
                if (designer) await exitEdit({ force: true });
                await realm.refresh().catch(() => {}); await open(d.id, null); set({ busy: false }); if (view) view.toast(t('homedash.designer.duplicated', { name: d.name || d.id }), { tone: 'info' }); return true;
            } catch (err) { set({ busy: false }); if (view) view.toast(E.save.errorText(t, E.api.normalizeError(err)), { tone: 'warn' }); return false; }
        }
        async function deleteDashboard() {
            if (!doc || state.busy || state.desktopReadonly) return false; const id = doc.id, name = state.dashboard ? state.dashboard.name : id;
            const ok = await sheetFor().confirm({ title: t('homedash.delete.title'), text: [t('homedash.delete.body', { name })], confirmLabel: t('homedash.delete.confirm'), cancelLabel: t('homedash.delete.cancel'), tone: 'danger' }); if (!ok || disposed || !doc || doc.id !== id) return false;
            if (designer) await exitEdit({ force: true });
            const done = await remove({ id, revision: doc.revision }); if (done && !disposed && state.mode === 'dashboard') await home(null); return done;
        }
        async function openCopy(id, name) {   // "save mine as a copy": the original lane is discarded, the copy opens in edit mode
            if (disposed) return; const old = doc ? doc.id : null; if (designer) await exitEdit({ force: true });
            if (old) { dropLane(realm.lanes.get(old)); store.docs.drop(old); }
            await realm.refresh().catch(() => {}); await open(id, null);
            if (state.mode === 'dashboard' && view) { await enterEdit(); view.toast(t('homedash.conflict.copy_done', { name: name || '' }), { tone: 'info' }); }
        }
        function stepViewPage(dir) { const ids = view && view.doc() ? view.doc().pages.map(p => p.id) : []; if (ids.length < 2) return false; view.select(ids[(ids.indexOf(view.pageId()) + dir + ids.length) % ids.length], {}); return true; }
        const ctx = () => ({ root, t, lang, api, store, catalog, designer, mount: self, state: () => state, open: (id, p) => guarded(() => open(id, p)), home: () => guarded(() => home(null)), sheet: appSheet(),
            toast: (text, to) => { if (view) view.toast(text, to); }, refresh: () => realm.refresh(), sheetHost: o.ui && o.ui.sheetHost });
        async function run(action, arg) {
            if (disposed || !state.edit) return false; const act = state.edit.actions[action]; if (act && (!act.available || !act.enabled)) return false;
            switch (action) {
                case 'new_starter': await create('starter'); return true; case 'new_empty': await create('empty'); return true; case 'new_ha': await create('home_assistant'); return true;
                case 'open': await guarded(() => home(null)); return true;
                case 'edit': return designer ? (await exitEdit()) === 'left' : enterEdit(arg);
                case 'exit_edit': return (await exitEdit()) === 'left';
                case 'show_editor': return !!(doc && store.docs.focusEditor(doc.id));
                case 'layout_auto': viewBp = null; if (view) view.setBreakpoint(null); set({}); return true;
                case 'layout_lg': case 'layout_md': case 'layout_sm': if (designer) return designer.run(action, arg); viewBp = action.slice(7); view.setBreakpoint(viewBp); set({}); return true;
                case 'fit': fitOv = state.edit.fit === 'fit_screen' ? 'scroll' : 'fit_screen'; view.setFit(fitOv); set({}); return true;
                case 'present': return present();
                case 'page_prev': case 'page_next': return designer ? designer.run(action, arg) : stepViewPage(action === 'page_next' ? 1 : -1);
                case 'duplicate_dashboard': return duplicateDashboard(); case 'delete_dashboard': return deleteDashboard();
                case 'retry_save': { const l = laneOf(); if (l) l.retryNow(); return !!l; }
                default: break;
            }
            if (designer && designer.handles(action)) return !!designer.run(action, arg);
            if (E.ddialogs && typeof E.ddialogs.run === 'function') return !!(await E.ddialogs.run(ctx(), action, arg));
            return false;
        }
        function contextMenu(ev) {   // design: the designer's menu; view: more info for a light frame whose more button is shown, edit widget when editing is possible
            if (disposed || !ev) return null; if (designer) return designer.contextMenu(ev);
            const tg = ev.target, fr = state.mode === 'dashboard' && tg && typeof tg.closest === 'function' ? tg.closest('.hd-widget') : null; if (!fr || !fr.closest('.hd-page')) return null;
            const items = [], more = fr.querySelector('.hd-more'), e = state.edit;
            if (more && !more.hasAttribute('hidden')) items.push({ id: 'more_info', label: t('homedash.ctx.more_info'), icon: 'more_info', run: () => more.click() });
            if (e.canEdit && !e.editElsewhere) items.push({ id: 'edit_widget', label: t('homedash.ctx.edit_widget'), icon: 'edit', run: () => { enterEdit({ widgetId: fr.getAttribute('data-hd-wid') }); } });
            if (!items.length) return null; let x = ev.clientX, y = ev.clientY;
            if (!x && !y) { const r = fr.getBoundingClientRect(); x = r.left + 8; y = r.top + 8; }
            return { x, y, items };
        }
        life.on(root, 'keydown', ev => { if ((ev.ctrlKey || ev.metaKey) && !ev.altKey && !ev.shiftKey && String(ev.key).toLowerCase() === 'e' && state.mode === 'dashboard') { ev.preventDefault(); run('edit'); } });
        life.on(root, 'click', ev => {   // data-hd-cmd buttons outside the edit bar (the empty page, the conflict bar) run an action
            const b = ev.target && typeof ev.target.closest === 'function' ? ev.target.closest('[data-hd-cmd]') : null;
            if (!b || b.closest('.hd-editbar') || b.getAttribute('aria-disabled') === 'true') return; run(b.getAttribute('data-hd-cmd'));
        });
        boot();
        const self = Object.freeze({
            on: bus.on, off: bus.off, state: () => state, open: (id, p) => guarded(() => open(id, p)), home: () => guarded(() => home(null)), createStarter: () => create('starter'), createEmpty: () => create('empty'), createHomeAssistant: () => create('home_assistant'), remove, setReadonly,
            refreshLook() { if (view) view.refreshLook(); }, retry: () => boot(), enterEdit, exitEdit, run, contextMenu, beforeClose,
            reconnect() { const s = realm.stream.status(); if (s.phase === 'stopped') realm.stream.resume(); else realm.stream.retryNow(); },
            dispose() {
                if (disposed) return; const l = designer ? laneOf() : null; if (l && l.state().dirty) l.flush({ keepalive: true });   // the realm keeps a dirty lane; its PUT survives the unload
                stopDesign(); disposed = true; openSeq += 1; life.dispose(); if (view) view.destroy(); view = null; root.remove();
            }
        });
        return self;
    }
    E.mount = mount;
})();
