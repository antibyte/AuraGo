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
    // Engine icon ids (C3) -> names of the desktop icon sets (iconMarkup draws `fallback`, the first letter of the label, for an unknown name).
    const ICONS = { more_info: 'info', edit: 'edit', duplicate: 'copy', copy: 'copy', cut: 'scissors', paste: 'clipboard', delete: 'trash', variant: 'widgets', add_widget: 'file-plus', check: 'check-square', rename: 'text', icon: 'image', left: 'chevron-left', right: 'chevron-right' };
    // Display-only shortcut hints of the window menus (the desktop matches `shortcut` only; the engine owns the keys, C14).
    const HINTS = { edit: 'Ctrl+E', undo: 'Ctrl+Z', redo: 'Ctrl+Shift+Z', cut: 'Ctrl+X', copy: 'Ctrl+C', paste: 'Ctrl+V', duplicate: 'Ctrl+D', delete: 'Del', select_all: 'Ctrl+A' };
    // Engine menu items -> items of the desktop's showContextMenu. menus-and-routing.js tests `item.separator` (not `type`) and does not render
    // `checked`: a checked item carries the `check` icon. `icon` is the engine icon id, mapped onto ICONS.
    function toDesktopItems(items) {
        return (items || []).map(it => (it.separator ? { separator: true } : {
            id: it.id, label: it.label, icon: ICONS[it.icon] || it.icon, fallback: it.label ? String(it.label).charAt(0) : '', shortcut: it.shortcut || it.shortcutHint || '',
            disabled: !!it.disabled, disabledHint: it.disabledHint, danger: !!it.danger, items: it.items ? toDesktopItems(it.items) : undefined,
            action: () => { if (typeof it.run === 'function') it.run(); }
        }));
    }
    // A menu list without empty groups: null entries (hidden items) go, separators neither lead, trail nor double up.
    function tidy(items) {
        const out = [];
        for (const it of items) { if (!it) continue; if (it.type === 'separator' && (!out.length || out[out.length - 1].type === 'separator')) continue; out.push(it); }
        while (out.length && out[out.length - 1].type === 'separator') out.pop();
        return out;
    }
    function render(host, id, ctx) {
        dispose(id);
        const E = window.HomeDashEngine, Home = window.HomeDashHome, el = E.el, setAttr = E.core.setAttr, icon = name => E.icons.icon(name);
        const t = (key, vars) => ctx.t(key, vars), lifetime = new AbortController(), signal = lifetime.signal, lang = document.documentElement.lang || 'en';
        const BANNER_ICONS = { auth_lost: 'lock', offline: 'signal-off', source_unavailable: 'alert', source_degraded: 'clock', schema_too_new: 'info', readonly: 'lock', source_catalog_failed: 'alert', template_omitted: 'info' };
        let inst = null, lastBanners = '', lastState = '', lastMenus = '', lastMenuBase = '', menuTimer = 0, lookFrame = 0, pressedAt = 0, releasing = false;
        const title = el('h1', { class: 'hdx-title', text: t('desktop.app_homedash') });
        const back = el('button', { type: 'button', class: 'hdx-back', 'data-action': 'home', hidden: 'true' }, [icon('arrow-left'), el('span', { text: t('homedash.view.back') })]);
        const connText = el('span', { class: 'hdx-conn-text' });
        const conn = el('span', { class: 'hdx-conn', 'data-conn': 'connecting', hidden: 'true' }, [el('span', { class: 'hdx-conn-dot', 'aria-hidden': 'true' }), connText]);
        const editBtn = el('button', { type: 'button', class: 'hdx-btn hdx-edit', 'data-action': 'edit', 'aria-keyshortcuts': 'Control+E', title: t('homedash.designer.edit_hint'), hidden: 'true' }, [icon('edit'), el('span', { text: t('homedash.designer.edit') })]);
        const elsewhere = el('div', { class: 'hdx-elsewhere', hidden: 'true' }, [el('span', { class: 'hdx-elsewhere-text', text: t('homedash.designer.elsewhere') }),
            el('button', { type: 'button', class: 'hdx-btn', 'data-action': 'show-editor', text: t('homedash.designer.show_window') })]);
        const saveText = el('span', { class: 'hdx-save-text' }), saveRetry = el('button', { type: 'button', class: 'hdx-btn', 'data-action': 'retry-save', text: t('homedash.save.retry_now'), hidden: 'true' });
        const saveChip = el('div', { class: 'hdx-save', 'aria-label': t('homedash.save.label'), hidden: 'true' }, [saveText, saveRetry]);
        const banners = el('div', { class: 'hdx-banners' });
        const viewHost = el('div', { class: 'hdx-view', hidden: 'true' }), statePanel = el('div', { class: 'hdx-state', hidden: 'true' });
        // "New empty dashboard" (home card and window menu): create it, open it and drop straight into the designer.
        const createEmptyAndEdit = () => inst.createEmpty().then(() => { const s = inst.state(); if (s.mode === 'dashboard' && s.edit && s.edit.canEdit && s.edit.mode === 'view') return inst.enterEdit(); });
        const home = Home.create({ t, lang, onOpen: dashboardId => inst.open(dashboardId), onStarter: () => inst.createStarter(), onEmpty: () => createEmptyAndEdit(), onHa: () => inst.createHomeAssistant(), onImport: () => inst.run('import'), sourceName: src => sourceName(src), onDelete: summary => askDelete(summary) });
        setAttr(home.el, 'hidden', true);
        const root = el('div', { class: 'hdx-app', 'data-hdx-view': 'loading' }, [el('header', { class: 'hdx-bar' }, [back, el('div', { class: 'hdx-heading' }, title), conn, saveChip, elsewhere, editBtn]), banners,
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
            /* The container stays; banners that persist keep their node (no re-announcement), only new ones are inserted and gone ones removed. */
            const have = new Map(Array.from(banners.children).map(n => [n.getAttribute('data-key'), n])), want = new Set();
            list.forEach((b, i) => {
                const key = b.id + '|' + b.kind; want.add(key);
                const node = have.get(key) || el('div', { class: 'hdx-banner', 'data-key': key, 'data-banner': b.kind, 'data-tone': b.tone, role: b.role }, [icon(BANNER_ICONS[b.kind] || 'info'),
                    el('span', { class: 'hdx-banner-text', text: bannerText(b) }),
                    b.action === 'retry' ? el('button', { type: 'button', class: 'hdx-btn hdx-banner-action', 'data-action': 'reconnect', text: t('homedash.banner.retry') }) : null]);
                const at = banners.children[i] || null; if (at !== node) banners.insertBefore(node, at);
            });
            for (const [key, n] of have) if (!want.has(key)) n.remove();
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
        // Rebuilding the window menus replaces ~250 nodes of the shell. A change of the edit actions that arrives while the primary button is down on the dashboard, or in the task of its release
        // (a selection, a commit, an undo step), waits for its own task, so it never lands in the press or the release of a drag. Every other change (a context menu may be open) rebuilds at once.
        const pressed = () => pressedAt > 0 && Date.now() - pressedAt < 10000;
        function menus(s, now) {
            const base = [s.mode, !!s.desktopReadonly, !!s.busy].join('|');
            if (!now && base === lastMenuBase && (pressed() || releasing)) { if (!menuTimer && !pressed()) menuTimer = setTimeout(() => { menuTimer = 0; menus(inst.state(), true); }, 0); return; }
            if (menuTimer) { clearTimeout(menuTimer); menuTimer = 0; } lastMenuBase = base;
            const ed = s.edit || null, acts = (ed && ed.actions) || {}, ha = Array.isArray(s.templates) && s.templates.includes('home_assistant'), dash = s.mode === 'dashboard' && !!s.dashboard;
            const sig = [s.mode, !!s.desktopReadonly, !!s.busy, ha, ed && ed.mode, ed && ed.bp, ed && ed.fit, ed && ed.undoLabel, ed && ed.redoLabel, JSON.stringify(acts)].join('|'); if (sig === lastMenus) return; lastMenus = sig;
            const blocked = !!s.desktopReadonly || !!s.busy || s.mode === 'disabled' || s.mode === 'error', sep = { type: 'separator' };
            // An item of an engine action: left out when the engine reports it unavailable (no E.ddialogs yet), greyed unless it reports it enabled.
            const act = (name, label, extra) => { const a = acts[name] || {}; if (a.available === false) return null;
                return Object.assign({ id: name.replace(/_/g, '-'), label, disabled: a.enabled !== true, checked: !!a.checked, shortcutHint: HINTS[name] || '', action: () => inst.run?.(name) }, extra); };
            const newItem = acts.new && acts.new.available !== false ? act('new', t('homedash.menu.new')) : { id: 'new', label: t('homedash.menu.new'), icon: 'plus', items: [
                { id: 'new-starter', label: t('homedash.menu.new_starter'), disabled: blocked, action: () => inst.createStarter() },
                { id: 'new-empty', label: t('homedash.menu.new_empty'), disabled: blocked, action: () => createEmptyAndEdit() },
                ...(ha ? [{ id: 'new-ha', label: t('homedash.menu.new_ha'), icon: 'home', disabled: blocked, action: () => inst.createHomeAssistant() }] : [])] };
            const reload = { id: 'reload', label: t('homedash.menu.reload'), icon: 'refresh', action: () => { const st = inst.state(); if (st.mode === 'dashboard' && st.dashboard) inst.open(st.dashboard.id, st.pageId); else inst.retry(); } };
            const open = { id: 'open', label: t('homedash.menu.open'), disabled: s.mode === 'disabled', action: () => inst.home() };
            const menusOut = [{ id: 'homedash-dashboard', label: t('homedash.menu.dashboard'), items: tidy(dash
                ? [newItem, open, sep, act('rename', t('homedash.menu.rename')), act('duplicate_dashboard', t('homedash.menu.duplicate_dashboard')), act('delete_dashboard', t('homedash.menu.delete')), sep, act('import', t('homedash.menu.import')), act('export', t('homedash.menu.export')), sep, reload]
                : [newItem, open, sep, act('import', t('homedash.menu.import')), sep, reload]) }];
            if (dash && ed) {
                menusOut.push({ id: 'homedash-edit', label: t('homedash.menu.edit'), items: tidy([
                    act('undo', ed.undoLabel ? t('homedash.designer.undo_label', { label: ed.undoLabel }) : t('homedash.menu.undo'), { icon: 'undo' }),
                    act('redo', ed.redoLabel ? t('homedash.designer.redo_label', { label: ed.redoLabel }) : t('homedash.menu.redo'), { icon: 'redo' }), sep,
                    act('cut', t('homedash.menu.cut'), { icon: 'scissors' }), act('copy', t('homedash.menu.copy'), { icon: 'copy' }), act('paste', t('homedash.menu.paste'), { icon: 'clipboard' }),
                    act('duplicate', t('homedash.menu.duplicate')), act('delete', t('homedash.menu.delete_widgets'), { icon: 'trash' }), sep, act('select_all', t('homedash.menu.select_all')), sep, act('look', t('homedash.designer.look'))]) });
                menusOut.push({ id: 'homedash-view', label: t('homedash.menu.view'), items: tidy([act('edit', t('homedash.menu.edit_mode'), { id: 'edit-mode', icon: 'edit' }), sep,
                    { id: 'layout', label: t('homedash.menu.layout'), items: [act('layout_auto', t('homedash.bp.auto')), act('layout_lg', t('homedash.bp.lg')), act('layout_md', t('homedash.bp.md')), act('layout_sm', t('homedash.bp.sm'))].filter(Boolean) },
                    act('zoom_fit', t('homedash.menu.zoom_fit')), act('zoom_actual', t('homedash.designer.zoom_actual')),
                    act('fit', t('homedash.menu.fit')), act('present', t('homedash.menu.present'))]) });
            }
            ctx.setWindowMenus?.(id, menusOut);
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
            paintState(s); subtitle(dash ? s.dashboard.name : ''); menus(s); paintEdit(s, dash);
        }
        // Edit chips of the bar (C3): the Edit button, "being edited in another window" and the save state of a lane that is not saved (view mode).
        function saveLabel(ed) {
            switch (ed.phase) {
                case 'retrying': return t('homedash.save.retrying', { seconds: Math.max(1, Math.ceil(((ed.nextRetryAt || 0) - Date.now()) / 1000)) });
                case 'invalid': return t('homedash.save.invalid', { reason: E.save.errorText(t, { code: ed.errorCode }) });
                case 'conflict': return t('homedash.save.conflict');
                case 'readonly': return t('homedash.save.readonly');
                case 'gone': return t('homedash.save.gone');
                default: return t('homedash.save.saving');
            }
        }
        function paintEdit(s, dash) {
            const ed = (dash && s.edit) || null, design = !!ed && ed.mode === 'design', view = !!ed && !design, unsaved = view && !!ed.phase && ed.phase !== 'saved';
            setAttr(root, 'data-hdx-edit', design ? 'design' : 'view');
            setAttr(editBtn, 'hidden', !(view && ed.canEdit && !ed.editElsewhere)); setAttr(elsewhere, 'hidden', !(view && ed.editElsewhere));
            setAttr(saveChip, 'hidden', !unsaved); setAttr(saveChip, 'data-phase', unsaved ? ed.phase : null);
            if (unsaved) saveText.textContent = saveLabel(ed);
            setAttr(saveRetry, 'hidden', !(unsaved && ed.phase === 'retrying'));
        }
        function askDelete(summary) {
            sheet.confirm({ title: t('homedash.delete.title'), text: [t('homedash.delete.body', { name: summary.name || summary.id })], confirmLabel: t('homedash.delete.confirm'), cancelLabel: t('homedash.delete.cancel'), tone: 'danger' })
                .then(yes => { if (yes) inst.remove(summary); });
        }
        inst = E.mount(viewHost, { host: 'desktop', base: '/api/desktop/homedash', t, lang, readonly: !!ctx.readonly,
            transport: { request: (url, init) => ctx.api(url, init), eventSource: url => new EventSource(url) },
            dashboardId: typeof ctx.dashboard_id === 'string' && ctx.dashboard_id ? ctx.dashboard_id : null, pageId: typeof ctx.page_id === 'string' ? ctx.page_id : null,
            visibility: visibilitySignal(root, win, signal), scheme: () => (document.body.dataset.scheme === 'light' ? 'light' : 'dark'), motion,
            canEdit: true, ui: { menu: (x, y, items) => ctx.showContextMenu?.(x, y, toDesktopItems(items)), focusWindow: () => ctx.focusWindow?.(id), sheetHost: () => root },
            onContext: patch => { if (typeof ctx.updateWindowContext === 'function') ctx.updateWindowContext(id, patch); } });
        ctx.wireContextMenuBoundary?.(root, { onContextMenu: ev => {
            const m = inst.contextMenu ? inst.contextMenu(ev) : null; if (!m || !ctx.showContextMenu) return false;
            ctx.showContextMenu(m.x, m.y, toDesktopItems(m.items)); return true; } });
        ctx.setWindowBeforeClose?.(id, () => (inst.beforeClose ? inst.beforeClose() : true));
        root.addEventListener('click', event => {
            const b = event.target && event.target.closest ? event.target.closest('[data-action]') : null;
            if (!b || b.closest('.hdx-home') || b.closest('.hd-sheet-layer')) return;
            const action = b.getAttribute('data-action');
            if (action === 'home') inst.home(); else if (action === 'reconnect') inst.reconnect(); else if (action === 'retry') inst.retry();
            else if (action === 'edit') inst.run?.('edit'); else if (action === 'show-editor') inst.run?.('show_editor'); else if (action === 'retry-save') inst.run?.('retry_save');
        }, { signal });
        root.addEventListener('pointerdown', event => { if (event.button === 0) pressedAt = Date.now(); }, { capture: true, signal });
        const released = () => { if (!pressedAt) return; pressedAt = 0; releasing = true; if (menuTimer) clearTimeout(menuTimer); menuTimer = setTimeout(() => { menuTimer = 0; releasing = false; menus(inst.state(), true); }, 0); };
        document.addEventListener('pointerup', released, { capture: true, signal }); document.addEventListener('pointercancel', released, { capture: true, signal });
        document.addEventListener('aurago:desktop-policy', event => {
            const d = (event && event.detail) || {};
            const ro = d.readonly === true || d.read_only === true ? true : d.readonly === false || d.read_only === false ? false : null;
            if (ro !== null) inst.setReadonly(ro);
        }, { signal });
        // refreshLook re-reads the theme and relays the page: only a change of what it reads (the theme attributes, the body class and the body style) runs it, once per frame.
        const lookSig = () => ['data-theme', 'data-scheme', 'data-fruity-mode', 'data-accent', 'data-animations'].map(a => document.body.getAttribute(a)).join('|') + '|' + document.body.className + '|' + (document.body.getAttribute('style') || '');
        let lastLook = lookSig();
        const themeObserver = typeof MutationObserver === 'function' ? new MutationObserver(() => {
            if (lookFrame) return; lookFrame = requestAnimationFrame(() => { lookFrame = 0; const sig = lookSig(); if (sig === lastLook) return; lastLook = sig; inst.refreshLook(); });
        }) : null;
        if (themeObserver) themeObserver.observe(document.body, { attributes: true, attributeFilter: ['data-theme', 'data-scheme', 'data-fruity-mode', 'data-accent', 'data-animations', 'style', 'class'] });
        inst.on('state', paint); paint(inst.state());
        instances.set(id, () => {
            lifetime.abort(); if (themeObserver) themeObserver.disconnect(); if (lookFrame) cancelAnimationFrame(lookFrame); if (menuTimer) clearTimeout(menuTimer);
            ctx.setWindowBeforeClose?.(id, null); inst.dispose(); sheet.destroy(); home.destroy(); ctx.setWindowMenus?.(id, []); subtitle(''); root.replaceChildren(); root.remove();
        });
    }
    window.HomeDashApp = { render, dispose, visibilitySignal, toDesktopItems };
})();
