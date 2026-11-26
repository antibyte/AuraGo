// HomeDash dashboard view (F2; design-frontend 6, 9.1). create(opts) owns the content of one .hd-root: root attributes from the document look
// and the host (scheme, motion), page tabs, the scroll stage with one hd-render page, the live region, and E.widgets.applyLook after every
// render and look change. banners(state) is the pure banner list the hosts draw (at most two, by priority). View mode is the default; the designer (M4) switches
// the view to design mode (setMode), re-renders live (update), forces a breakpoint or fit (setBreakpoint, setFit) and shows toasts.
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
        const used = Array.isArray(x.usedSources) ? x.usedSources : null;   // banners only for sources the open dashboard uses (when given)
        if (dash) for (const src of Array.isArray(x.sources) ? x.sources : []) {
            if (!src || typeof src.source !== 'string' || (used && !used.includes(src.source))) continue;
            if (src.state === 'unavailable' && src.reason === 'catalog_failed') out.push({ id: 'source:' + src.source, kind: 'source_catalog_failed', tone: 'warn', role: 'status', source: src.source });
            else if (src.state === 'unavailable') out.push({ id: 'source:' + src.source, kind: 'source_unavailable', tone: 'warn', role: 'status', source: src.source });
            else if (src.state === 'degraded') out.push({ id: 'source:' + src.source, kind: 'source_degraded', tone: 'warn', role: 'status', source: src.source });
        }
        if (dash && x.schemaTooNew) out.push({ id: 'schema_too_new', kind: 'schema_too_new', tone: 'info', role: 'status' });
        if (dash && x.desktopReadonly) out.push({ id: 'readonly', kind: 'readonly', tone: 'info', role: 'status' });
        if (dash && x.notice === 'template_omitted' && Number(x.noticeCount) > 0) out.push({ id: 'omitted', kind: 'template_omitted', tone: 'info', role: 'status', count: Number(x.noticeCount) });
        return out.slice(0, 2);
    }
    function create(opts) {
        const o = opts, root = o.root, t = o.t, id = uid('hd-view'), life = E.core.bag();
        const tabs = el('div', { class: 'hd-tabs', role: 'tablist', 'aria-label': t('homedash.view.pages'), hidden: 'true' });
        const stage = el('div', { class: 'hd-stage' }), live = el('div', { class: 'hd-live hd-sr-only', 'aria-live': 'polite' }), toasts = el('div', { class: 'hd-toasts', 'aria-hidden': 'true' });
        root.replaceChildren(tabs, stage, live, toasts);
        const moreinfo = E.moreinfo && typeof E.moreinfo.create === 'function' ? E.moreinfo.create({ root }) : null;
        let doc = null, page = null, pageId = null, pageMany = false, mode = 'view', forcedBp = null, fitOverride = null, zoomActual = false, describe = null, emptyAction = null;
        const bus = E.core.emitter(), toastTimers = new Map();   // toast node -> its removal timer; one disposer clears what is left
        life.add(() => { for (const h of toastTimers.values()) clearTimeout(h); toastTimers.clear(); });
        const scheme = () => (typeof o.scheme === 'function' && o.scheme() === 'light' ? 'light' : 'dark');
        const motion = () => (typeof o.motion === 'function' && o.motion() === 'reduced' ? 'reduced' : 'full');
        function applyRoot() {
            const look = doc.look, mode = look.palette.mode;
            const custom = mode === 'custom' ? (look.palette.custom && typeof look.palette.custom === 'object' ? look.palette.custom : look.palette) : null;
            const sch = mode === 'light' || mode === 'dark' ? mode : mode === 'custom' ? (custom.scheme === 'light' ? 'light' : 'dark') : scheme();
            const attrs = { 'data-hd-family': look.family, 'data-hd-palette': mode, 'data-hd-scheme': sch, 'data-hd-density': look.density, 'data-hd-backdrop': (look.backdrop && look.backdrop.kind) || 'theme', 'data-hd-motion': motion() };
            for (const [k, v] of Object.entries(attrs)) setAttr(root, k, v);
            setVars(root, { '--hd-radius-scale': look.radius_scale === 1 ? null : look.radius_scale });
            // L1: applyLook owns the inline --hd-accent/-light-on/-focus/-ink-on-accent/-hud, so the view never writes or clears those names. The solved accent
            // and its ink go into the neutral layer (--hd-n-accent, --hd-n-ink-on-accent) that the bridge maps to them; --hd-focus follows --hd-accent.
            // Neither applyPalette nor clearPalette is used: both would also write (or remove) the five owned names, so the view builds its own map.
            const solved = custom ? E.colour.solvePalette(custom).tokens : null, vars = {};
            for (const n of E.colour.PALETTE_TOKENS) if (!LOOK_OWNED.includes(n)) vars['--hd-' + n] = solved ? solved[n] : null;
            vars['--hd-n-accent'] = solved ? solved.accent : null; vars['--hd-n-ink-on-accent'] = solved ? solved['ink-on-accent'] : null;
            setVars(root, vars);
        }
        function renderTabs() {
            const many = doc.pages.length > 1; setAttr(tabs, 'hidden', !many || mode === 'design'); tabs.replaceChildren();
            if (!many) return;
            for (const p of doc.pages) { const on = p.id === pageId;
                tabs.appendChild(el('button', { type: 'button', class: 'hd-tab', role: 'tab', id: id + '-tab-' + p.id, 'data-page': p.id, 'aria-selected': on ? 'true' : 'false',
                    'aria-controls': id + '-page', tabindex: on ? '0' : '-1', text: p.name || t('homedash.home.page_main') })); }
        }
        function renderPage() {
            if (moreinfo) moreinfo.close();
            if (E.popover) E.popover.close(root);
            if (page) page.destroy();
            const pg = doc.pages.find(p => p.id === pageId), many = doc.pages.length > 1; pageMany = many;
            page = E.render.page({ doc, page: pg, stage, mode, bp: forcedBp, fit: fitOverride, zoomActual, emptyAction, describe: (w, it, ent) => (describe ? describe(w, it, ent) : ''), store: o.store, catalog: o.catalog, t, lang: o.lang, id: id + '-page', role: many ? 'tabpanel' : 'region',
                labelledBy: many ? id + '-tab-' + pg.id : null, label: many ? null : doc.name, raf: o.raf, caf: o.caf, measure: o.measure,
                commands: o.commands || null, moreInfo: moreinfo ? req => moreinfo.open(req) : null,
                announce: text => { live.textContent = text; }, onLayout: m => { setAttr(root, 'data-hd-bp', m.bp); setAttr(root, 'data-hd-fit', m.fit); bus.emit('layout', m); } });
            bus.emit('page', page); if (typeof o.onEntities === 'function') o.onEntities(page.entityIds());
        }
        function select(next, focus) {
            if (!doc || next === pageId || !doc.pages.some(p => p.id === next)) return;
            pageId = next; renderTabs(); renderPage(); E.widgets.applyLook(root);
            if (focus) { const tab = Array.from(tabs.children).find(b => b.getAttribute('data-page') === next); if (tab) tab.focus(); }
            if (typeof o.onPage === 'function') o.onPage(pageId);
        }
        life.on(tabs, 'click', ev => { if (mode !== 'view') return; const b = ev.target && ev.target.closest ? ev.target.closest('.hd-tab') : null; if (b) select(b.getAttribute('data-page'), false); });
        life.on(tabs, 'keydown', ev => {
            if (!doc || mode !== 'view') return;
            const ids = doc.pages.map(p => p.id), i = ids.indexOf(pageId);
            const next = ev.key === 'ArrowRight' ? ids[(i + 1) % ids.length] : ev.key === 'ArrowLeft' ? ids[(i - 1 + ids.length) % ids.length] : ev.key === 'Home' ? ids[0] : ev.key === 'End' ? ids[ids.length - 1] : null;
            if (next === null) return; ev.preventDefault(); select(next, true);
        });
        return Object.freeze({
            show(raw, wantPage) {
                doc = E.doc.normalise(raw, { sizeOf: o.catalog.defaultSize, constraints: o.catalog.constraints });
                pageId = doc.pages.some(p => p.id === wantPage) ? wantPage : doc.pages.some(p => p.id === pageId) ? pageId : doc.pages[0].id;
                applyRoot(); renderTabs(); renderPage(); E.widgets.applyLook(root);
            },
            pageId: () => pageId, entityIds: () => (page ? page.entityIds() : []),
            sources: () => {   // the prefixes of every entity bound anywhere in the open document, so a tab change never flips a banner
                const ids = []; if (doc) for (const pg of doc.pages) for (const w of pg.widgets || []) for (const v of Object.values(w.bindings || {})) if (typeof v === 'string' && v.indexOf(':') > 0) ids.push(v);
                return [...new Set(ids.map(e => e.slice(0, e.indexOf(':'))))];
            },
            setConnection(c) { setAttr(root, 'data-hd-conn', c && c.phase === 'live' ? 'live' : c && c.offline ? 'offline' : 'connecting'); },
            setReadonly(flag) { setAttr(root, 'data-hd-readonly', !!flag); },
            setMode(m) {
                mode = m === 'design' ? 'design' : 'view'; if (mode === 'design' && moreinfo) moreinfo.close(); root.classList.toggle('hd-backdrop', mode === 'view'); stage.classList.toggle('hd-backdrop', mode === 'design');
                if (doc) renderTabs(); if (page) page.setMode(mode);
            },
            // update(raw, {normalised}): the live document of the designer. The page node stays while its id exists (widgets are diffed by the renderer);
            // a vanished page falls back to the first one and renders afresh; a changed look re-applies the root attributes and the look.
            update(raw, uo) {
                const prev = doc; doc = uo && uo.normalised ? raw : E.doc.normalise(raw, { sizeOf: o.catalog.defaultSize, constraints: o.catalog.constraints });
                if (!doc.pages.some(p => p.id === pageId)) { pageId = doc.pages[0].id; applyRoot(); renderTabs(); renderPage(); E.widgets.applyLook(root); return; }
                const lookChanged = !prev || E.doc.keyOf(prev.look) !== E.doc.keyOf(doc.look); if (lookChanged) applyRoot(); renderTabs();
                if (!page || (doc.pages.length > 1) !== pageMany) renderPage(); else page.update(doc, doc.pages.find(p => p.id === pageId), { keepBox: !!(uo && uo.keepBox) });
                if (lookChanged) E.widgets.applyLook(root); if (typeof o.onEntities === 'function') o.onEntities(page.entityIds());
            },
            select(next, so) { select(next, !!(so && so.focus)); },
            setBreakpoint(b) { forcedBp = b || null; if (page) page.setBreakpoint(forcedBp); }, setFit(v) { fitOverride = v || null; if (page) page.setFit(fitOverride); }, setZoomActual(v) { zoomActual = !!v; if (page) page.setZoomActual(zoomActual); },
            setDescribe(fn) { describe = typeof fn === 'function' ? fn : null; }, setEmptyAction(a) { emptyAction = a || null; if (page) page.setEmptyAction(emptyAction); },
            page: () => page, get stage() { return stage; }, doc: () => doc, announce(text) { live.textContent = String(text); },
            toast(text, to) {   // at most three toasts at once, each removed after ms; the text is also announced
                const n = el('div', { class: 'hd-toast', 'data-tone': (to && to.tone) || 'info', text: String(text) }); toasts.appendChild(n);
                while (toasts.children.length > 3) { const old = toasts.firstElementChild; clearTimeout(toastTimers.get(old)); toastTimers.delete(old); old.remove(); }
                toastTimers.set(n, setTimeout(() => { toastTimers.delete(n); n.remove(); }, (to && to.ms) || 4000)); live.textContent = String(text);
            },
            on: bus.on, off: bus.off,
            refreshLook() { if (!doc) return; applyRoot(); E.widgets.applyLook(root); if (page) page.layout(); },
            destroy() { life.dispose(); if (moreinfo) moreinfo.destroy(); if (E.popover) E.popover.close(root); if (page) page.destroy(); page = null; doc = null; root.replaceChildren(); }
        });
    }
    E.view = Object.freeze({ create, banners });
})();
