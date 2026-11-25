// HomeDash mount (F2; design-frontend 1.4). mount(host, options) is the composition root of one HomeDash window. Windows of one JS realm
// share one API client, stream and store per base path (realmFor). It loads bootstrap and the widget catalog, lists the dashboards, creates
// the starter (template "system") or an empty dashboard, deletes one, opens one dashboard in an hd-view and keeps its entities subscribed while
// the host's visibility signal says visible, refetches the list 250 ms after dashboard_changed (once per realm) and reconciles the open
// dashboard by revision. The host draws its chrome from the frozen 'state' object. View only in M2.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.api || !E.stream || !E.store || !E.catalog || !E.view || !E.command) throw new Error('hd-app.js: load the HomeDash engine first');
    const realms = new Map(), LIST_COALESCE_MS = 250;
    function realmFor(base, transport, timers) {
        const have = realms.get(base);
        if (have) return have;
        const tm = timers || {}, api = E.api.create({ base, transport }), clock = { setTimeout: tm.setTimeout, clearTimeout: tm.clearTimeout, now: tm.now };
        const diagnose = () => api.bootstrap().then(() => 'ok', err => (err.code === 'homedash_disabled' ? 'disabled' : err.status === 401 ? 'auth_lost' : 'error'));
        const stream = E.stream.shared(Object.assign({ api, random: tm.random, diagnose }, clock));
        const store = E.store.shared(Object.assign({ api, stream, raf: tm.raf, caf: tm.caf }, clock));
        const setT = tm.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = tm.clearTimeout || (id => clearTimeout(id));
        const realm = { api, stream, store, commands: E.command.shared(Object.assign({ api, stream, store }, clock)), list: null, load: null, next: null, timer: 0, listeners: new Set(), refresh: null };
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
        realms.set(base, realm);
        return realm;
    }
    function mount(hostEl, options) {
        const o = options || {}, t = typeof o.t === 'function' ? o.t : (k => k), lang = o.lang || 'en';
        const realm = realmFor(o.base || '/api/desktop/homedash', o.transport, o.timers), api = realm.api, store = realm.store;
        const bus = E.core.emitter(), life = E.core.bag(), vis = o.visibility || { visible: () => true, subscribe: () => () => {} };
        const root = E.el('div', { class: 'hd-root hd-backdrop', 'data-hd-host': o.host || 'desktop', 'data-hd-mode': 'view' });
        hostEl.appendChild(root);
        const watch = store.watch(E.core.uid('hd-win'), { ids: [], active: vis.visible() });
        life.add(vis.subscribe(on => { watch.setActive(on); if (on && state.mode === 'home') refreshTemplates(); }));
        let state = Object.freeze({ mode: 'loading', error: null, notice: null, busy: false, dashboards: null, dashboard: null, pageId: null, connection: store.connection(), desktopReadonly: !!o.readonly, banners: [], templates: [], noticeSource: null, noticeCount: 0 });
        let hostRo = !!o.readonly; // live host read-only flag: starts as the open-time snapshot, kept current by setReadonly
        let wasLive = false, haState = ''; // transitions that clear read-only locks / re-read templates
        let catalog = null, view = null, doc = null, disposed = false, openSeq = 0, late = null; // late: a list that arrived while an open was in flight
        function set(patch) {
            if (disposed) return;
            const next = Object.assign({}, state, patch);
            next.banners = E.view.banners({ mode: next.mode, connection: next.connection, sources: store.sources(), usedSources: view ? view.sources() : [], notice: next.notice, noticeCount: next.noticeCount, desktopReadonly: next.desktopReadonly, schemaTooNew: !!(next.dashboard && next.dashboard.schemaTooNew) });
            state = Object.freeze(next); bus.emit('state', state);
        }
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
                const ro = hostRo || b.desktop_readonly === true;
                store.setDesktopReadonly(ro);
                const cat = first && first.hash === b.catalog_hash ? first : await store.widgetCatalog(b.catalog_hash);
                if (disposed) return;
                catalog = E.catalog.create(cat);
                set({ desktopReadonly: ro, templates: templatesOf(b) });
                if (id) await open(id, o.pageId, docP); else await home(null);
            } catch (err) { fail(err, e => set({ mode: 'error', error: Object.freeze({ code: e.code, status: e.status }) })); }
        }
        // Bootstrap again: the template list (and the source states) follow the server, e.g. once Home Assistant is configured.
        function refreshTemplates() {
            return api.bootstrap().then(b => { if (!disposed) { store.setSources(b.sources); realm.commands.clearLocks(); set({ templates: templatesOf(b) }); } }, () => {});
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
                measure: o.measure, commands: realm.commands, onPage: p => { watch.set(view.entityIds()); set({ pageId: p }); context(); } });
            doc = d; view.show(d, pageId); view.setConnection(store.connection()); view.setReadonly(state.desktopReadonly);
            watch.set(view.entityIds());
            set({ mode: 'dashboard', notice: null, noticeSource: null, noticeCount: 0, busy: false, pageId: view.pageId(),
                dashboard: Object.freeze({ id: d.id, name: typeof d.name === 'string' && d.name ? d.name : d.id, revision: d.revision, savedBy: d.saved_by, schemaTooNew }) });
            context();
        }
        function closeView() { if (view) { view.destroy(); view = null; } doc = null; watch.set([]); }
        function reconcile(list) {
            if (disposed) return;
            if (state.mode === 'home') { set({ dashboards: list }); return; }
            if (state.mode !== 'dashboard' || !doc) return;
            const s = list.find(x => x.id === doc.id);
            if (!s) home('dashboard_deleted');
            else if (Number(s.revision) > Number(doc.revision)) open(doc.id, view ? view.pageId() : null);
        }
        async function create(kind) {
            if (state.busy || state.desktopReadonly || !catalog) return;
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
            try { await api.deleteDashboard(summary.id, summary.revision); set({ busy: false }); await realm.refresh().catch(() => {}); return true; }
            catch (err) {
                fail(err, e => set({ busy: false, notice: e.code === 'revision_conflict' ? 'delete_conflict' : e.code === 'desktop_readonly' ? 'readonly' : 'delete_failed' }));
                realm.refresh().catch(() => {}); return false;
            }
        }
        function applyReadonly(ro) { store.setDesktopReadonly(ro); if (view) view.setReadonly(ro); if (ro !== state.desktopReadonly) set({ desktopReadonly: ro }); }
        function setReadonly(flag) { hostRo = !!flag; applyReadonly(hostRo); if (!hostRo) realm.commands.clearLocks('virtual_desktop.readonly'); } // the host's live flag; a retry's boot() reads it, not the open-time snapshot
        boot();
        return Object.freeze({
            on: bus.on, off: bus.off, state: () => state, open, home: () => home(null), createStarter: () => create('starter'), createEmpty: () => create('empty'), createHomeAssistant: () => create('home_assistant'), remove, setReadonly,
            refreshLook() { if (view) view.refreshLook(); }, retry: () => boot(),
            reconnect() { const s = realm.stream.status(); if (s.phase === 'stopped') realm.stream.resume(); else realm.stream.retryNow(); },
            dispose() { if (disposed) return; disposed = true; openSeq += 1; life.dispose(); if (view) view.destroy(); view = null; root.remove(); }
        });
    }
    E.mount = mount;
})();
