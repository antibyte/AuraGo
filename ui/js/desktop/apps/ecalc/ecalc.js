/* ECalc app: the composition root. It fetches the catalog of the desktop language once per open window, shows a retry
   screen when that fails, hands the data to the shell (rail, list, page) and cleans up when the window closes. The
   math lives in ecalc-expr/units/eseries/solve.js, the page in ecalc-calc-page.js, the shell in ecalc-shell.js.
   Ctrl/Cmd+K belongs to the app while the focus is inside it: the desktop's Spotlight listens on the document, so the
   event is stopped here. The desktop can open the app on a pre-filled page: ctx.launch (an object or undefined) at render
   time, and launch(windowId, payload) for a window that is already open; the payload is
   {page: "<item id>", values: {"<var>": "<raw text>"}, choices: {"<choice>": "<option>"}} (ECalcShell.launch applies it;
   a payload that arrives while the catalog is still loading waits for it).
   Publishes window.ECalcApp = { render(hostEl, windowId, ctx), dispose(windowId), launch(windowId, payload) }. */
(function () {
    'use strict';

    const instances = new Map();
    // Languages that write the decimal comma. The core always uses the point; this only changes what is displayed.
    const COMMA_LANGUAGES = ['cs', 'da', 'de', 'el', 'es', 'fr', 'it', 'nl', 'no', 'pl', 'pt', 'sv'];
    const catalogURL = lang => '/api/ecalc/catalog?lang=' + encodeURIComponent(lang);

    function dispose(id) {
        const entry = instances.get(id);
        instances.delete(id);
        if (entry) entry.stop();
    }

    // Applies a launch payload to an already open window. Returns false when there is no such window or no payload.
    function launch(id, payload) {
        const entry = instances.get(id);
        if (!entry || !payload || typeof payload !== 'object' || Array.isArray(payload)) return false;
        entry.launch(payload);
        return true;
    }

    // The house translator inserts {name} and {{name}}; a stand-in translator may only know one of them.
    function fill(text, vars) {
        let out = String(text);
        for (const [name, value] of Object.entries(vars || {})) out = out.split('{{' + name + '}}').join(String(value)).split('{' + name + '}').join(String(value));
        return out;
    }

    function render(host, id, ctx) {
        dispose(id);
        const lang = String(ctx.lang || window.SYSTEM_LANG || document.documentElement.lang || 'en').slice(0, 2).toLowerCase();
        const uid = 'ec-' + String(id).replace(/[^\w-]/g, '-');
        const t = (key, vars) => fill(ctx.t(key, vars), vars);
        const esc = ctx.esc || window.ECalcTypeset.escape;
        const store = window.ECalcStore.create();
        const root = document.createElement('div');
        root.className = 'ec-app';
        root.dataset.compact = 'false';
        root.dataset.view = 'page';
        host.replaceChildren(root);
        let disposed = false, shell = null;
        // A launch payload waits here until the catalog has arrived (and again after a failed load that is retried).
        let pending = ctx.launch && typeof ctx.launch === 'object' && !Array.isArray(ctx.launch) ? ctx.launch : null;

        async function load() {
            root.dataset.state = 'loading';
            root.innerHTML = '<div class="ec-status" role="status" aria-busy="true"><span class="ec-spinner"></span></div>';
            try {
                // The desktop api() defaults to cache:'no-store', which would skip the ETag of the route: 'no-cache' keeps
                // the response in the browser cache and revalidates it, so an unchanged catalog costs a 304. Every item
                // carries its English name (name_en), which the search uses, so one request is enough.
                const catalog = await ctx.api(catalogURL(lang), { cache: 'no-cache' });
                if (disposed) return;
                if (!catalog || !Array.isArray(catalog.items) || !catalog.units) throw new Error('unexpected catalog');
                root.dataset.state = 'ready';
                shell = window.ECalcShell.create({ root, catalog, t, store, schematics: window.ECalcSchematics || {}, decimalComma: COMMA_LANGUAGES.includes(lang), uid, launch: pending });
                pending = null;
            } catch (error) {
                if (disposed) return;
                shell = null;
                root.dataset.state = 'failed';
                root.innerHTML = '<div class="ec-fail" role="alert"><p>' + esc(t('desktop.ecalc_load_failed')) + '</p><button type="button" class="ec-btn" data-action="retry">' + esc(t('desktop.ecalc_retry')) + '</button></div>';
            }
        }

        root.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'k') {
                event.preventDefault();
                event.stopPropagation();
                if (shell) shell.focusSearch();
            }
        });
        root.addEventListener('click', event => {
            const retry = event.target.closest('[data-action="retry"]');
            if (retry && root.contains(retry) && !shell) void load();
        });

        instances.set(id, {
            stop() {
                if (disposed) return;
                disposed = true;
                if (shell) shell.dispose();
                shell = null;
                store.dispose();
                root.replaceChildren();
            },
            launch(payload) {
                if (disposed) return;
                if (shell) shell.launch(payload); else pending = payload;
            }
        });
        void load();
    }

    window.ECalcApp = { render, dispose, launch };
})();
