// EasyDrag app shell: window lifecycle (render, open, dispose), catalog loading and routing
// between the start page and the editor.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const CATALOG_MAX_AGE_MS = 60 * 1000;
    const instances = new Map();

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
            if (b.dataset.edShell === 'config') window.open('/config#flows', '_blank', 'noopener');
            else start(inst, inst.lastRoute || {});
        });
        inst.root.appendChild(box);
    }

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

    function showHome(inst, opts) {
        inst.nav += 1;
        clearScreen(inst);
        inst.lastRoute = {};
        inst.screen = ED.home.create(inst.app);
        inst.root.appendChild(inst.screen.el);
        if (opts && opts.section === 'templates') {
            const grid = inst.screen.el.querySelector('.ed-template-grid');
            if (grid) requestAnimationFrame(() => grid.parentElement.scrollIntoView({ block: 'start' }));
        }
        if (typeof inst.ctx.updateWindowContext === 'function') inst.ctx.updateWindowContext(inst.windowId, { flowId: null });
        setHomeMenus(inst);
    }

    async function showFlow(inst, flowId, opts) {
        const nav = ++inst.nav;
        if (inst.screen && inst.screen.leave && !(await inst.screen.leave())) return;
        if (nav !== inst.nav) return;
        inst.lastRoute = { flowId, runId: opts && opts.runId };
        showLoading(inst);
        let loaded;
        try {
            const [res] = await Promise.all([inst.api.get(flowId), ensureCatalog(inst)]);
            loaded = res;
        } catch (err) {
            if (nav !== inst.nav) return;
            if (ED.core.errorCode(err) === 'FLOW_NOT_FOUND') {
                inst.ctx.notify({ title: 'EasyDrag', message: ED.core.errorText(inst.t, err), type: 'error' });
                showHome(inst);
            } else {
                showError(inst, err);
            }
            return;
        }
        if (nav !== inst.nav || !instances.has(inst.windowId)) return;
        clearScreen(inst);
        inst.screen = ED.editor.create(inst.app, loaded, opts || {});
        inst.root.appendChild(inst.screen.el);
    }

    async function start(inst, route) {
        showLoading(inst);
        try {
            await ensureCatalog(inst, true);
        } catch (err) {
            showError(inst, err);
            return;
        }
        if (route.flowId) showFlow(inst, route.flowId, { runId: route.runId });
        else showHome(inst);
    }

    function routeOf(context) {
        const c = context || {};
        return { flowId: c.flowId || c.flow_id || '', runId: c.runId || c.run_id || '' };
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
        start(inst, routeOf(ctx));
    }

    // open handles a second launch into an existing window (notification click, Mission Control).
    function open(windowId, context) {
        const inst = instances.get(windowId);
        if (!inst) return;
        const route = routeOf(context);
        if (!route.flowId) return;
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
