/* HomeDash desktop host (F2; design-frontend 8). window.HomeDashApp = {render, dispose, visibilitySignal}. render(host, id, ctx) builds the
   window chrome (bar, banners, home view, dashboard view, state panel, confirm sheet), mounts the engine (HomeDashEngine.mount) with the
   desktop transport (ctx.api and EventSource), the visibility signal, the scheme and motion of the desktop body, the read-only policy event,
   window menus and the window context (dashboard_id, page_id) for session restore. Chrome is painted from the desktop theme
   (desktop-app-homedash.css); the dashboard canvas is the engine's .hd-root. Server text reaches the page only through textContent. */
(function () {
    'use strict';
    const instances = new Map();
    function dispose(id) { const fn = instances.get(id); instances.delete(id); if (fn) fn(); }
    // Visible while the tab is shown, the .vd-window is not hidden, minimised (no client rects) or on another space, and the app root
    // intersects the viewport (docker-manager-store.js attach()). Returns {visible(), subscribe(fn) -> unsubscribe}.
    function visibilitySignal(root, win, signal) {
        const listeners = new Set();
        let inView = null, shown = false; // null: no report yet, counts as visible (first values)
        const compute = () => !document.hidden && inView !== false && (!win || (!win.hidden && !win.classList.contains('vd-space-hidden') && win.style.display !== 'none' && win.getClientRects().length > 0));
        const check = () => { const next = compute(); if (next === shown) return; shown = next; for (const fn of [...listeners]) { try { fn(shown); } catch (err) { console.error(err); } } };
        document.addEventListener('visibilitychange', check, { signal });
        const mo = typeof MutationObserver === 'function' && win ? new MutationObserver(check) : null;
        if (mo) mo.observe(win, { attributes: true, attributeFilter: ['style', 'class', 'hidden', 'data-space-hidden'] });
        const io = typeof IntersectionObserver === 'function' ? new IntersectionObserver(entries => { inView = !!(entries.length && entries[entries.length - 1].isIntersecting); check(); }, { threshold: 0.01 }) : null;
        if (io) io.observe(root);
        signal.addEventListener('abort', () => { if (mo) mo.disconnect(); if (io) io.disconnect(); listeners.clear(); }, { once: true });
        shown = compute();
        return { visible: () => shown, subscribe(fn) { listeners.add(fn); return () => listeners.delete(fn); } };
    }
    function render(host, id, ctx) {
        dispose(id);
        const E = window.HomeDashEngine, Home = window.HomeDashHome, el = E.el, setAttr = E.core.setAttr, icon = name => E.icons.icon(name);
        const t = (key, vars) => ctx.t(key, vars), lifetime = new AbortController(), signal = lifetime.signal, lang = document.documentElement.lang || 'en';
        const BANNER_ICONS = { auth_lost: 'lock', offline: 'signal-off', source_unavailable: 'alert', source_degraded: 'clock', schema_too_new: 'info', readonly: 'lock', source_catalog_failed: 'alert', template_omitted: 'info' };
        let inst = null, lastBanners = '', lastState = '', lastMenus = '', lookFrame = 0;
        const title = el('h1', { class: 'hdx-title', text: t('desktop.app_homedash') });
        const back = el('button', { type: 'button', class: 'hdx-back', 'data-action': 'home', hidden: 'true' }, [icon('arrow-left'), el('span', { text: t('homedash.view.back') })]);
        const connText = el('span', { class: 'hdx-conn-text' });
        const conn = el('span', { class: 'hdx-conn', 'data-conn': 'connecting', hidden: 'true' }, [el('span', { class: 'hdx-conn-dot', 'aria-hidden': 'true' }), connText]);
        const banners = el('div', { class: 'hdx-banners' });
        const viewHost = el('div', { class: 'hdx-view', hidden: 'true' }), statePanel = el('div', { class: 'hdx-state', hidden: 'true' });
        const home = Home.create({ t, lang, onOpen: dashboardId => inst.open(dashboardId), onStarter: () => inst.createStarter(), onEmpty: () => inst.createEmpty(), onHa: () => inst.createHomeAssistant(), sourceName: src => sourceName(src), onDelete: summary => askDelete(summary) });
        setAttr(home.el, 'hidden', true);
        const root = el('div', { class: 'hdx-app', 'data-hdx-view': 'loading' }, [el('header', { class: 'hdx-bar' }, [back, el('div', { class: 'hdx-heading' }, title), conn]), banners,
            el('main', { class: 'hdx-main' }, [home.el, viewHost, statePanel])]);
        host.replaceChildren(root);
        const sheet = E.sheet.create(root), win = host.closest('.vd-window'), subtitleEl = win ? win.querySelector('.vd-window-subtitle') : null;
        const subtitle = text => { if (subtitleEl) subtitleEl.textContent = text; };
        const motion = () => (document.body.dataset.animations === 'false' || (typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches) ? 'reduced' : 'full');
        const sourceName = src => (src === 'system' ? t('homedash.source.system') : src === 'ha' ? t('homedash.source.ha') : String(src));
        function bannerText(b) {
            switch (b.kind) {
                case 'auth_lost': return t('homedash.banner.auth_lost');
                case 'offline': return t('homedash.banner.offline');
                case 'source_unavailable': return t('homedash.banner.source_unavailable', { source: sourceName(b.source) });
                case 'source_degraded': return t('homedash.banner.source_degraded', { source: sourceName(b.source) });
                case 'source_catalog_failed': return t('homedash.banner.source_catalog_failed', { source: sourceName(b.source) });
                case 'template_omitted': return t('homedash.banner.template_omitted', { count: b.count });
                case 'schema_too_new': return t('homedash.banner.schema_too_new');
                default: return t('homedash.banner.readonly');
            }
        }
        function paintBanners(list) {
            const sig = JSON.stringify(list.map(b => [b.id, b.kind])); if (sig === lastBanners) return; lastBanners = sig;
            banners.replaceChildren(...list.map(b => el('div', { class: 'hdx-banner', 'data-banner': b.kind, 'data-tone': b.tone, role: b.role }, [icon(BANNER_ICONS[b.kind] || 'info'),
                el('span', { class: 'hdx-banner-text', text: bannerText(b) }),
                b.action === 'retry' ? el('button', { type: 'button', class: 'hdx-btn hdx-banner-action', 'data-action': 'reconnect', text: t('homedash.banner.retry') }) : null])));
        }
        function paintState(s) {
            const kind = ['loading', 'disabled', 'error'].includes(s.mode) ? s.mode : '', key = kind + ':' + (s.error ? s.error.code : '');
            if (key === lastState) return; lastState = key; statePanel.replaceChildren(); setAttr(statePanel, 'data-kind', kind || null);
            if (kind === 'loading') statePanel.append(el('div', { class: 'hdx-spinner', role: 'status', 'aria-label': t('homedash.app.loading') }));
            if (kind === 'disabled') statePanel.append(icon('power'), el('h2', { class: 'hdx-state-title', text: t('homedash.disabled.title') }),
                el('p', { class: 'hdx-state-text', text: t('homedash.disabled.text') }), el('button', { type: 'button', class: 'hdx-btn', 'data-action': 'retry', text: t('homedash.disabled.retry') }));
            if (kind === 'error') statePanel.append(icon('alert'), el('h2', { class: 'hdx-state-title', text: t('homedash.error.title') }),
                el('p', { class: 'hdx-state-text', text: s.error && s.error.code === 'storage_unavailable' ? t('homedash.error.storage') : t('homedash.error.generic', { code: (s.error && s.error.code) || '?' }) }),
                el('button', { type: 'button', class: 'hdx-btn', 'data-action': 'retry', text: t('homedash.home.retry') }));
        }
        function menus(s) {
            const ha = Array.isArray(s.templates) && s.templates.includes('home_assistant'), sig = s.mode + '|' + !!s.desktopReadonly + '|' + !!s.busy + '|' + ha; if (sig === lastMenus) return; lastMenus = sig;
            const blocked = !!s.desktopReadonly || !!s.busy || s.mode === 'disabled' || s.mode === 'error';
            ctx.setWindowMenus?.(id, [{ id: 'homedash', label: t('desktop.app_homedash'), items: [
                { label: t('homedash.menu.all'), disabled: s.mode === 'disabled', action: () => inst.home() },
                { type: 'separator' },
                { label: t('homedash.menu.new_starter'), icon: 'plus', disabled: blocked, action: () => inst.createStarter() },
                { label: t('homedash.menu.new_empty'), disabled: blocked, action: () => inst.createEmpty() },
                ...(ha ? [{ label: t('homedash.menu.new_ha'), icon: 'home', disabled: blocked, action: () => inst.createHomeAssistant() }] : []),
                { type: 'separator' },
                { label: t('homedash.menu.reload'), icon: 'refresh', action: () => { const st = inst.state(); if (st.mode === 'dashboard' && st.dashboard) inst.open(st.dashboard.id, st.pageId); else inst.retry(); } }] }]);
        }
        function paint(s) {
            setAttr(root, 'data-hdx-view', s.mode);
            const dash = s.mode === 'dashboard' && s.dashboard, c = s.connection || {}, cs = c.phase === 'live' ? 'live' : c.offline ? 'offline' : 'connecting';
            title.textContent = dash ? s.dashboard.name : t('desktop.app_homedash');
            setAttr(back, 'hidden', s.mode !== 'dashboard'); setAttr(conn, 'hidden', s.mode !== 'dashboard'); setAttr(conn, 'data-conn', cs);
            connText.textContent = cs === 'live' ? t('homedash.conn.live') : cs === 'offline' ? t('homedash.conn.offline') : t('homedash.conn.connecting');
            paintBanners(s.banners || []);
            setAttr(home.el, 'hidden', s.mode !== 'home'); setAttr(viewHost, 'hidden', s.mode !== 'dashboard'); setAttr(statePanel, 'hidden', !['loading', 'disabled', 'error'].includes(s.mode));
            if (s.mode === 'home') home.render(s);
            paintState(s); subtitle(dash ? s.dashboard.name : ''); menus(s);
        }
        function askDelete(summary) {
            sheet.confirm({ title: t('homedash.delete.title'), text: [t('homedash.delete.body', { name: summary.name || summary.id })], confirmLabel: t('homedash.delete.confirm'), cancelLabel: t('homedash.delete.cancel'), tone: 'danger' })
                .then(yes => { if (yes) inst.remove(summary); });
        }
        inst = E.mount(viewHost, { host: 'desktop', base: '/api/desktop/homedash', t, lang, readonly: !!ctx.readonly,
            transport: { request: (url, init) => ctx.api(url, init), eventSource: url => new EventSource(url) },
            dashboardId: typeof ctx.dashboard_id === 'string' && ctx.dashboard_id ? ctx.dashboard_id : null, pageId: typeof ctx.page_id === 'string' ? ctx.page_id : null,
            visibility: visibilitySignal(root, win, signal), scheme: () => (document.body.dataset.scheme === 'light' ? 'light' : 'dark'), motion,
            onContext: patch => { if (typeof ctx.updateWindowContext === 'function') ctx.updateWindowContext(id, patch); } });
        root.addEventListener('click', event => {
            const b = event.target && event.target.closest ? event.target.closest('[data-action]') : null;
            if (!b || b.closest('.hdx-home') || b.closest('.hd-sheet-layer')) return;
            const action = b.getAttribute('data-action');
            if (action === 'home') inst.home(); else if (action === 'reconnect') inst.reconnect(); else if (action === 'retry') inst.retry();
        }, { signal });
        document.addEventListener('aurago:desktop-policy', event => {
            const d = (event && event.detail) || {};
            const ro = d.readonly === true || d.read_only === true ? true : d.readonly === false || d.read_only === false ? false : null;
            if (ro !== null) inst.setReadonly(ro);
        }, { signal });
        const themeObserver = typeof MutationObserver === 'function' ? new MutationObserver(() => {
            if (lookFrame) return; lookFrame = requestAnimationFrame(() => { lookFrame = 0; inst.refreshLook(); });
        }) : null;
        if (themeObserver) themeObserver.observe(document.body, { attributes: true, attributeFilter: ['data-theme', 'data-scheme', 'data-fruity-mode', 'data-accent', 'data-animations', 'style', 'class'] });
        inst.on('state', paint); paint(inst.state());
        instances.set(id, () => {
            lifetime.abort(); if (themeObserver) themeObserver.disconnect(); if (lookFrame) cancelAnimationFrame(lookFrame);
            inst.dispose(); sheet.destroy(); home.destroy(); ctx.setWindowMenus?.(id, []); subtitle(''); root.replaceChildren(); root.remove();
        });
    }
    window.HomeDashApp = { render, dispose, visibilitySignal };
})();
