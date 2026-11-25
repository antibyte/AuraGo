// HomeDash dashboard view (F2; design-frontend 6, 9.1). create(opts) owns the content of one .hd-root: root attributes from the document look
// and the host (scheme, motion), page tabs, the scroll stage with one hd-render page, the live region, and E.widgets.applyLook after every
// render and look change. banners(state) is the pure banner list the hosts draw (at most two, by priority). View mode only in M2.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.render || !E.doc || !E.colour || !E.widgets) throw new Error('hd-view.js: load hd-render.js and the engine first');
    const { el, setAttr, setVars, uid } = E.core;
    const LOOK_OWNED = ['accent', 'light-on', 'focus', 'ink-on-accent', 'hud']; // palette token names E.widgets.applyLook writes itself
    function banners(s) {
        const x = s || {}, c = x.connection || {}, dash = x.mode === 'dashboard', out = [];
        if (c.phase === 'stopped' && c.reason === 'auth_lost') out.push({ id: 'auth_lost', kind: 'auth_lost', tone: 'danger', role: 'alert' });
        else if (dash && c.offline) out.push({ id: 'offline', kind: 'offline', tone: 'warn', role: 'status', action: 'retry' });
        if (dash) for (const src of Array.isArray(x.sources) ? x.sources : []) {
            if (!src || typeof src.source !== 'string') continue;
            if (src.state === 'unavailable') out.push({ id: 'source:' + src.source, kind: 'source_unavailable', tone: 'warn', role: 'status', source: src.source });
            else if (src.state === 'degraded') out.push({ id: 'source:' + src.source, kind: 'source_degraded', tone: 'warn', role: 'status', source: src.source });
        }
        if (dash && x.schemaTooNew) out.push({ id: 'schema_too_new', kind: 'schema_too_new', tone: 'info', role: 'status' });
        if (dash && x.desktopReadonly) out.push({ id: 'readonly', kind: 'readonly', tone: 'info', role: 'status' });
        return out.slice(0, 2);
    }
    function create(opts) {
        const o = opts, root = o.root, t = o.t, id = uid('hd-view'), life = E.core.bag();
        const tabs = el('div', { class: 'hd-tabs', role: 'tablist', 'aria-label': t('homedash.view.pages'), hidden: 'true' });
        const stage = el('div', { class: 'hd-stage' }), live = el('div', { class: 'hd-live hd-sr-only', 'aria-live': 'polite' });
        root.replaceChildren(tabs, stage, live);
        let doc = null, page = null, pageId = null;
        const scheme = () => (typeof o.scheme === 'function' && o.scheme() === 'light' ? 'light' : 'dark');
        const motion = () => (typeof o.motion === 'function' && o.motion() === 'reduced' ? 'reduced' : 'full');
        function applyRoot() {
            const look = doc.look, mode = look.palette.mode;
            const sch = mode === 'light' || mode === 'dark' ? mode : mode === 'custom' ? (look.palette.scheme === 'light' ? 'light' : 'dark') : scheme();
            const attrs = { 'data-hd-family': look.family, 'data-hd-palette': mode, 'data-hd-scheme': sch, 'data-hd-density': look.density, 'data-hd-backdrop': (look.backdrop && look.backdrop.kind) || 'theme', 'data-hd-motion': motion() };
            for (const [k, v] of Object.entries(attrs)) setAttr(root, k, v);
            setVars(root, { '--hd-radius-scale': look.radius_scale === 1 ? null : look.radius_scale });
            // L1: applyLook owns the inline --hd-accent/-light-on/-focus/-ink-on-accent/-hud, so those names never reach applyPalette. The solved accent
            // and its ink go into the neutral layer (--hd-n-accent, --hd-n-ink-on-accent) that the bridge maps to them; --hd-focus follows --hd-accent.
            if (mode === 'custom') {
                const tokens = Object.assign({}, E.colour.solvePalette(look.palette).tokens);
                setVars(root, { '--hd-n-accent': tokens.accent, '--hd-n-ink-on-accent': tokens['ink-on-accent'] });
                for (const n of LOOK_OWNED) delete tokens[n];
                E.colour.applyPalette(root, tokens);
            } else { E.colour.clearPalette(root); setVars(root, { '--hd-n-accent': null, '--hd-n-ink-on-accent': null }); }
        }
        function renderTabs() {
            const many = doc.pages.length > 1; setAttr(tabs, 'hidden', !many); tabs.replaceChildren();
            if (!many) return;
            for (const p of doc.pages) { const on = p.id === pageId;
                tabs.appendChild(el('button', { type: 'button', class: 'hd-tab', role: 'tab', id: id + '-tab-' + p.id, 'data-page': p.id, 'aria-selected': on ? 'true' : 'false',
                    'aria-controls': id + '-page', tabindex: on ? '0' : '-1', text: p.name || t('homedash.home.page_main') })); }
        }
        function renderPage() {
            if (page) page.destroy();
            const pg = doc.pages.find(p => p.id === pageId), many = doc.pages.length > 1;
            page = E.render.page({ doc, page: pg, stage, store: o.store, catalog: o.catalog, t, lang: o.lang, id: id + '-page', role: many ? 'tabpanel' : 'region',
                labelledBy: many ? id + '-tab-' + pg.id : null, label: many ? null : doc.name, raf: o.raf, caf: o.caf, measure: o.measure,
                announce: text => { live.textContent = text; }, onLayout: m => { setAttr(root, 'data-hd-bp', m.bp); setAttr(root, 'data-hd-fit', m.fit); } });
        }
        function select(next, focus) {
            if (!doc || next === pageId || !doc.pages.some(p => p.id === next)) return;
            pageId = next; renderTabs(); renderPage(); E.widgets.applyLook(root);
            if (focus) { const tab = Array.from(tabs.children).find(b => b.getAttribute('data-page') === next); if (tab) tab.focus(); }
            if (typeof o.onPage === 'function') o.onPage(pageId);
        }
        life.on(tabs, 'click', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('.hd-tab') : null; if (b) select(b.getAttribute('data-page'), false); });
        life.on(tabs, 'keydown', ev => {
            if (!doc) return;
            const ids = doc.pages.map(p => p.id), i = ids.indexOf(pageId);
            const next = ev.key === 'ArrowRight' ? ids[(i + 1) % ids.length] : ev.key === 'ArrowLeft' ? ids[(i - 1 + ids.length) % ids.length] : ev.key === 'Home' ? ids[0] : ev.key === 'End' ? ids[ids.length - 1] : null;
            if (next === null) return; ev.preventDefault(); select(next, true);
        });
        return Object.freeze({
            show(raw, wantPage) {
                doc = E.doc.normalise(raw, { sizeOf: o.catalog.defaultSize });
                pageId = doc.pages.some(p => p.id === wantPage) ? wantPage : doc.pages.some(p => p.id === pageId) ? pageId : doc.pages[0].id;
                applyRoot(); renderTabs(); renderPage(); E.widgets.applyLook(root);
            },
            pageId: () => pageId, entityIds: () => (page ? page.entityIds() : []),
            setConnection(c) { setAttr(root, 'data-hd-conn', c && c.phase === 'live' ? 'live' : c && c.offline ? 'offline' : 'connecting'); },
            setReadonly(flag) { setAttr(root, 'data-hd-readonly', !!flag); },
            refreshLook() { if (!doc) return; applyRoot(); E.widgets.applyLook(root); if (page) page.layout(); },
            destroy() { life.dispose(); if (page) page.destroy(); page = null; doc = null; root.replaceChildren(); }
        });
    }
    E.view = Object.freeze({ create, banners });
})();
