// Standalone fixture of TestDesktopCYDCreatorDesignerBrowser: the real cyd-creator-core.js (P.util,
// P.i18n, P.createBus) is loaded before the designer modules; this file adds a fake shell instance
// with the 4a seam (layout store, capabilities, bus events lock:changed(locked, jobId) and
// readonly:changed(bool), asset upload that replaces the store remotely like plan 4a's window,
// hooks, confirm) and mounts the designer into the shell's workbench columns inside .cydc-app.
// Keys Delete, Backspace, Enter and F2 that reach the document (where the desktop's own handler
// listens) are recorded in pageKeys. Test hooks: window.cydFixture.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const errors = [];
    window.addEventListener('error', e => errors.push(String(e.message)));
    window.addEventListener('unhandledrejection', e => errors.push(String(e.reason && e.reason.message || e.reason)));
    // Designer modules report caught failures with console.error (bus handlers, store subscribers).
    const consoleError = console.error.bind(console);
    console.error = (...args) => { errors.push(args.map(a => (a && a.stack) || String(a)).join(' ')); consoleError(...args); };

    const pageKeys = [];
    document.addEventListener('keydown', e => {
        if (!['Delete', 'Backspace', 'Enter', 'F2'].includes(e.key)) return;
        const el = e.target;
        const name = el && el.getAttribute ? (el.getAttribute('data-cc-focus') || el.getAttribute('data-cc-palette') || el.className || el.tagName) : String(el);
        pageKeys.push(e.key + '@' + name);
    });

    const project = {
        format: 1, template_version: '1.0.0', name: 'Wetterstation', board: 'esp32-2432s028r', orientation: 'landscape', theme: 'night',
        language: 'de', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3',
        settings: { wifi: 'optional', aurago_link: false, libraries: ['arduinojson'], backlight: 80, statusbar_clock: true },
        start_screen: 's_home',
        screens: [
            { id: 's_home', name: 'home', title: 'Start', transition: 'fade', style: {}, widgets: [
                { id: 'w_status', type: 'statusbar', name: 'status_1', x: 0, y: 0, w: 320, h: 24, props: { title: 'Wetter' } },
                { id: 'w_btn', type: 'button', name: 'btn_1', x: 16, y: 40, w: 120, h: 40, props: { text: 'Details', symbol: 'right' } },
                { id: 'w_lbl', type: 'label', name: 'lbl_1', x: 160, y: 48, w: 120, h: 24, props: { text: '22,5 °C', size: 20 } },
                { id: 'w_pnl', type: 'panel', name: 'pnl_1', x: 16, y: 104, w: 144, h: 120, children: [] },
                { id: 'w_tabs', type: 'tabview', name: 'tabs_1', x: 176, y: 104, w: 136, h: 120, props: { tabs: ['A', 'B'], bar_size: 28 }, children: [
                    { id: 'w_sw', type: 'switch', name: 'sw_1', x: 8, y: 8, w: 56, h: 28 },
                    { id: 'w_led', type: 'led', name: 'led_1', x: 8, y: 8, w: 24, h: 24, tab: 1 }] }] },
            { id: 's_details', name: 'details', title: 'Details', transition: 'slide_left', style: {}, widgets: [
                { id: 'w_gauge', type: 'gauge', name: 'gauge_1', x: 90, y: 40, w: 140, h: 140 },
                { id: 'w_img', type: 'image', name: 'img_1', x: 8, y: 8, w: 32, h: 32, props: { asset: 'a_sun' } }] }],
        assets: [{ id: 'a_sun', file: 'assets/sun.png', w: 32, h: 32, alpha: true }]
    };

    async function start() {
        const params = new URLSearchParams(location.search);
        const lang = params.get('lang') || 'en';
        const [caps, words] = await Promise.all([
            fetch('/testdata/cyd_creator_designer_caps.json').then(r => r.json()),
            fetch('/lang/cyd_creator/' + lang + '.json').then(r => r.json())
        ]);
        document.documentElement.lang = lang;
        window.I18N = words;
        const t = P.i18n(null);
        const bus = P.createBus();
        let locked = false;
        let version = 1;
        const fx = window.cydFixture = { errors, pageKeys, notices: [], uploads: [], files: [], hooks: [], confirms: [], events: [], saves: 0, ready: false };
        // The shell events the fixture sends, in order (the designer must not need any other).
        ['layout:issues', 'lock:changed', 'readonly:changed', 'project:changed', 'mode:changed', 'designer:reveal'].forEach(type => bus.on(type, (...args) => fx.events.push([type].concat(args))));
        const store = P.model.createStore(project, { limit: 300, coalesceMs: 1200 });
        // Like plan 4a's autosave: only local edits, undo and redo are saved, never while read-only.
        store.subscribe((p, info) => { if (!inst.readonly && ['local', 'undo', 'redo'].includes(info.origin)) fx.saves++; });
        const inst = {
            windowId: 'fixture', host: document.body, ctx: {}, t, caps, projectId: 'ccp_fixture',
            project: { id: 'ccp_fixture', name: 'Wetterstation', board: 'esp32-2432s028r', theme: 'night', version: 1, locked_by_job: '' },
            layout: () => ({ version, project: store.get() }), layoutStore: store,
            isLocked: () => locked, readonly: false,
            assetUrl: id => '/fixture-asset/' + encodeURIComponent(id),
            bus,
            confirm: async spec => { fx.confirms.push(spec); return true; },
            notify: (kind, text) => fx.notices.push([kind, text]),
            flushAll: async () => true,
            // uploadAsset models plan 4a's window: the server lists the asset in the layout and
            // announces project_changed (what: assets) before it answers; the window re-reads the
            // layout and replaces its store (origin remote, undo history kept), then returns the asset.
            uploadAsset: async file => {
                fx.uploads.push(file.name + ':' + file.type);
                fx.files.push(file);
                await new Promise(r => setTimeout(r, 20));
                const asset = { id: 'a_up', file: 'assets/up.png', w: 48, h: 40, alpha: true };
                version++;
                bus.emit('project:changed', { version, origin: 'user', what: 'assets' });
                const cur = store.get();
                store.replace(Object.assign({}, cur, { assets: (cur.assets || []).concat([asset]) }), { resetHistory: false });
                bus.emit('layout:issues', []);
                return asset;
            },
            // ensureHook resolves true once the stub exists (plan 4a code.js); failures notify there.
            code: { ensureHook: async arg => { fx.hooks.push(arg); return true; } }
        };
        const designer = P.designer.create(inst);
        designer.mountScreens(document.getElementById('screens'));
        designer.mountPalette(document.getElementById('palette'));
        designer.mountCanvas(document.getElementById('canvas'));
        designer.mountInspector(document.getElementById('inspector'));
        Object.assign(fx, {
            designer, store, inst, bus, state: designer.state, t,
            widget: id => { const f = P.model.findWidget(store.get(), id); return f ? f.widget : null; },
            parentOf: id => { const f = P.model.findWidget(store.get(), id); return f && f.parent ? f.parent.id : ''; },
            abs: id => P.model.absRect(store.get(), id, designer.state.env()),
            client(x, y) { const r = document.querySelector('.cydc-screen-host').getBoundingClientRect(); const z = designer.state.zoom; return { x: r.left + x * z, y: r.top + y * z }; },
            // byFocus finds a control by its data-cc-focus key.
            byFocus: key => Array.from(document.querySelectorAll('[data-cc-focus]')).find(el => el.getAttribute('data-cc-focus') === key) || null,
            // setLocked and setReadonly send the events plan 4a sends (app.js lock:changed, setReadonly).
            setLocked(v) { locked = !!v; bus.emit('lock:changed', locked, locked ? 'ccj_fixture' : ''); },
            setReadonly(v) { inst.readonly = !!v; bus.emit('readonly:changed', !!v); },
            // remote applies a change made elsewhere (another tab, the agent): a store replace.
            remote(fn) { const next = P.util.deepClone(store.get()); fn(next); version++; store.replace(next, { resetHistory: false }); },
            ready: true
        });
    }
    start().catch(err => errors.push('fixture: ' + err.message));
})();
