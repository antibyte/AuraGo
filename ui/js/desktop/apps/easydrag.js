// EasyDrag app shell: window lifecycle (render, open, dispose), catalog loading and routing
// between the start page and the editor.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const CATALOG_MAX_AGE_MS = 60 * 1000;
    const instances = new Map();

    // alive reports whether inst still owns its window: an answer that arrives after dispose (or
    // after a new render of the window) must not build a screen on it.
    function alive(inst) { return instances.get(inst.windowId) === inst; }

    function buildCatalog(res) {
        const list = (res && res.node_types) || [];
        return { types: new Map(list.map(info => [info.type, info])), list, categories: (res && res.categories) || [] };
    }

    // ensureCatalog loads the node catalog; it is refreshed when older than a minute so that
    // integrations configured meanwhile appear when the next flow opens.
    async function ensureCatalog(inst, force) {
        if (!force && inst.catalog && Date.now() - inst.catalogAt < CATALOG_MAX_AGE_MS) return inst.catalog;
        inst.catalog = buildCatalog(await inst.api.nodeTypes());
        inst.catalogAt = Date.now();
        return inst.catalog;
    }

    function clearScreen(inst) {
        if (inst.screen) { inst.screen.dispose(); inst.screen = null; }
        inst.root.querySelectorAll('.ed-shell-state').forEach(n => n.remove());
    }

    function showLoading(inst) {
        clearScreen(inst);
        inst.root.appendChild(ED.core.el('<div class="ed-shell-state ed-loading" role="status"><span class="ed-spinner" aria-hidden="true"></span><span>' + inst.esc(inst.t('easydrag.ui.loading')) + '</span></div>'));
    }

    function showError(inst, err) {
        const core = ED.core;
        const { t, esc } = inst;
        clearScreen(inst);
        const disabled = core.errorCode(err) === 'FLOWS_DISABLED';
        const box = core.el('<div class="ed-shell-state ed-shell-error"><div class="ed-empty-card">' + core.icon(disabled ? 'lock' : 'alert', 'ed-empty-icon') +
            '<h3>' + esc(disabled ? t('easydrag.ui.disabled_title') : t('easydrag.ui.load_failed')) + '</h3>' +
            '<p>' + esc(disabled ? t('easydrag.ui.disabled_text') : core.errorText(t, err)) + '</p>' +
            '<div class="ed-row">' + (disabled ? '<button type="button" class="ed-btn" data-ed-shell="config">' + core.icon('settings') + '<span>' + esc(t('easydrag.ui.open_settings')) + '</span></button>' : '') +
            '<button type="button" class="ed-btn ed-btn--primary" data-ed-shell="retry">' + core.icon('refresh') + '<span>' + esc(t('easydrag.ui.retry')) + '</span></button></div></div></div>');
        box.addEventListener('click', (event) => {
            const b = event.target.closest('[data-ed-shell]');
            if (!b) return;
            if (b.dataset.edShell === 'config') core.openFlowSettings();
            else start(inst, inst.lastRoute || {});
        });
        inst.root.appendChild(box);
        // The screen that had the focus is gone: Retry takes it (in the active window only).
        if (windowActive(inst)) box.querySelector('[data-ed-shell="retry"]').focus();
    }

    // windowActive: focus may move inside this window (it is the active one, or the desktop
    // cannot tell).
    function windowActive(inst) { return typeof inst.ctx.isActive !== 'function' || inst.ctx.isActive(); }

    function setHomeMenus(inst) {
        if (typeof inst.ctx.setWindowMenus !== 'function') return;
        inst.ctx.setWindowMenus(inst.windowId, [{
            id: 'flow', labelKey: 'easydrag.ui.menu_flow', items: [
                { id: 'new', labelKey: 'easydrag.ui.home_new', icon: 'plus', disabled: !!inst.ctx.readonly, action: () => clickHome(inst, '[data-ed-new]') },
                { id: 'import', labelKey: 'easydrag.ui.home_import', icon: 'upload', disabled: !!inst.ctx.readonly, action: () => clickHome(inst, '[data-ed-import]') },
                { type: 'separator' },
                { id: 'reload', labelKey: 'easydrag.ui.reload', icon: 'refresh', action: () => inst.screen && inst.screen.reload && inst.screen.reload() }
            ]
        }]);
    }

    function clickHome(inst, selector) {
        const b = inst.root.querySelector('.ed-home ' + selector);
        if (b) b.click();
    }

    // showHome shows the start page. Keyboard focus goes to New flow and, once the list is loaded,
    // to the card of the flow just left (from the editor), unless opts.focusNew keeps it on New flow
    // (Mission Control's New flow) or opts.section scrolls to the templates.
    function showHome(inst, opts) {
        if (!alive(inst)) return;
        const o = opts || {};
        const left = inst.screen && inst.screen.ed && !o.focusNew && o.section !== 'templates' ? inst.screen.ed.flow.id : '';
        inst.nav += 1;
        clearScreen(inst);
        inst.lastRoute = {};
        inst.screen = ED.home.create(inst.app, { focusFlow: left });
        inst.root.appendChild(inst.screen.el);
        const add = inst.screen.el.querySelector('[data-ed-new]');
        if (add && !add.disabled && windowActive(inst)) add.focus({ preventScroll: true });
        if (o.section === 'templates') {
            const grid = inst.screen.el.querySelector('.ed-template-grid');
            if (grid) requestAnimationFrame(() => grid.parentElement.scrollIntoView({ block: 'start' }));
        }
        // A notification's flow_id/run_id are cleared too: a later render from the window context
        // must not route back to them (updateWindowContext merges, so null clears a key).
        if (typeof inst.ctx.updateWindowContext === 'function') inst.ctx.updateWindowContext(inst.windowId, { flowId: null, flow_id: null, run_id: null });
        setHomeMenus(inst);
    }

    async function showFlow(inst, flowId, opts) {
        const nav = ++inst.nav;
        if (inst.screen && inst.screen.leave && !(await inst.screen.leave())) return;
        if (nav !== inst.nav || !alive(inst)) return;
        inst.lastRoute = { flowId, runId: opts && opts.runId };
        showLoading(inst);
        let loaded;
        try {
            const [res] = await Promise.all([inst.api.get(flowId), ensureCatalog(inst)]);
            loaded = res;
        } catch (err) {
            if (nav !== inst.nav || !alive(inst)) return;
            // An unknown or malformed id (a stale route, a hand-edited session) goes to the start
            // page; for the user both mean that the flow does not exist.
            const code = ED.core.errorCode(err);
            if (code === 'FLOW_NOT_FOUND' || code === 'FLOW_BAD_REQUEST') {
                inst.ctx.notify({ title: 'EasyDrag', message: inst.t('easydrag.ui.error_flow_not_found'), type: 'error' });
                showHome(inst);
            } else {
                showError(inst, err);
            }
            return;
        }
        if (nav !== inst.nav || !alive(inst)) return;
        clearScreen(inst);
        inst.screen = ED.editor.create(inst.app, loaded, opts || {});
        inst.root.appendChild(inst.screen.el);
    }

    // start loads the catalog, then shows the route. A navigation or dispose during the load wins.
    async function start(inst, route) {
        const nav = ++inst.nav;
        showLoading(inst);
        try {
            await ensureCatalog(inst, true);
        } catch (err) {
            if (nav === inst.nav && alive(inst)) showError(inst, err);
            return;
        }
        if (nav !== inst.nav || !alive(inst)) return;
        if (route.flowId) showFlow(inst, route.flowId, { runId: route.runId });
        else showHome(inst);
    }

    // goHome is a second launch's way to the start page (Mission Control's New flow). Like showFlow
    // it waits for the editor's leave(): a pending change is saved first, a failed save asks, and
    // "stay" keeps the editor. A start page that is shown already stays; New flow gets the focus.
    async function goHome(inst) {
        const nav = ++inst.nav;
        if (inst.screen && inst.screen.leave && !(await inst.screen.leave())) return;
        if (nav !== inst.nav || !alive(inst)) return;
        if (!inst.screen || inst.screen.ed) showHome(inst, { focusNew: true });
        const add = inst.root.querySelector('.ed-home [data-ed-new]');
        if (add && !add.disabled && typeof add.focus === 'function') add.focus();
    }

    // routeOf reads a launch context: a flow (and run) wins over section "home".
    function routeOf(context) {
        const c = context || {};
        const flowId = c.flowId || c.flow_id || '';
        return { flowId, runId: c.runId || c.run_id || '', home: !flowId && c.section === 'home' };
    }

    function render(container, windowId, context) {
        dispose(windowId);
        const ctx = context || {};
        const core = ED.core;
        const root = core.el('<div class="ed-app"></div>');
        container.innerHTML = '';
        container.appendChild(root);
        const inst = { windowId, ctx, root, t: ctx.t, esc: ctx.esc || core.esc, api: core.createApi(ctx.api), catalog: null, catalogAt: 0, screen: null, nav: 0, lastRoute: {} };
        inst.app = {
            ctx, t: inst.t, esc: inst.esc, api: inst.api, windowId, readonly: !!ctx.readonly,
            get catalog() { return inst.catalog; },
            openHome: opts => showHome(inst, opts),
            openFlow: (id, opts) => showFlow(inst, id, opts)
        };
        instances.set(windowId, inst);
        // A new window keeps its launch context, so section "home" is dropped once read: a later
        // render from that context must not lead back to the start page (the start page is the
        // default without a flow anyway).
        if (ctx.section && typeof ctx.updateWindowContext === 'function') ctx.updateWindowContext(windowId, { section: null });
        start(inst, routeOf(ctx));
    }

    // open handles a second launch into an existing window (notification click, Mission Control).
    // The shell does not store this context; section "home" shows the start page.
    function open(windowId, context) {
        const inst = instances.get(windowId);
        if (!inst) return;
        const route = routeOf(context);
        if (!route.flowId) {
            if (route.home) goHome(inst);
            return;
        }
        const editor = inst.screen && inst.screen.ed;
        if (editor && editor.flow.id === route.flowId) {
            if (route.runId) inst.screen.showRun(route.runId);
            return;
        }
        showFlow(inst, route.flowId, { runId: route.runId });
    }

    function dispose(windowId) {
        const inst = instances.get(windowId);
        if (!inst) return;
        instances.delete(windowId);
        inst.nav += 1;
        if (inst.screen) { inst.screen.dispose(); inst.screen = null; }
        if (typeof inst.ctx.clearWindowMenus === 'function') inst.ctx.clearWindowMenus(windowId);
        inst.root.remove();
    }

    window.EasyDragApp = { render, open, dispose, _instances: instances };
})();
