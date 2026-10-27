/* Docker Manager app: the composition root. It builds the skeleton (rail, toolbar, banners, container table, detail
   drawer, dialog host), wires the store to the views, keeps the focus across repaints and owns the keyboard handling,
   the context menu, the window menus and the toasts. Pure logic lives in docker-manager-model.js, requests and polling
   in docker-manager-store.js, markup in docker-manager-views.js and the dialogs in docker-manager-dialogs.js. */
(function () {
    'use strict';

    const PREFS_KEY = 'docker-manager.ui.v1';
    const instances = new Map(), warned = new Set();
    function dispose(id) { instances.get(id)?.(); instances.delete(id); }

    function loadPrefs(model) {
        const fallback = { scope: 'all', filter: 'all', collapsed: [] };
        try {
            const saved = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
            return {
                scope: typeof saved.scope === 'string' && saved.scope ? saved.scope : 'all',
                filter: model.STATE_FILTERS.includes(saved.filter) ? saved.filter : 'all',
                collapsed: Array.isArray(saved.collapsed) ? saved.collapsed.filter(key => typeof key === 'string').slice(0, 200) : []
            };
        } catch (error) { return fallback; }
    }
    function savePrefs(prefs) { try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch (error) { /* storage may be blocked */ } }

    function render(host, id, ctx) {
        dispose(id);
        const model = window.DockerManagerModel, Views = window.DockerManagerViews, Dialogs = window.DockerManagerDialogs, Store = window.DockerManagerStore;
        const esc = ctx.esc;
        // A missing key shows its own name and is reported once, never a misleading label.
        const t = (key, vars) => {
            const full = 'docker_manager.' + key, value = ctx.t(full, vars);
            if (value === full && !warned.has(full)) { warned.add(full); console.warn('Docker Manager: missing translation ' + full); }
            return value;
        };
        const dt = (key, vars) => ctx.t(key, vars);
        const lang = document.documentElement.lang || 'en';
        const uid = 'dm-' + String(id).replace(/[^\w-]/g, '-');
        const prefs = loadPrefs(model);
        const lifetime = new AbortController(), signal = lifetime.signal;
        const store = Store.create({ api: ctx.api, readonly: !!ctx.readonly, scope: prefs.scope });
        const state = store.state;
        const views = Views.create({ t, esc, model, lang, uid, logTails: Store.LOG_TAILS });
        const ui = { query: '', filter: prefs.filter, collapsed: new Set(prefs.collapsed), activeKey: '', rail: '', banners: '', tbody: '', empty: '', drawer: '', menus: '', logMark: '', focusDrawer: false, pendingFocus: '', opener: '' };
        let disposed = false, frame = 0, announceTimer = 0, prefsTimer = 0, lastCount = '';
        const toastTimers = new Map();

        // ---- skeleton --------------------------------------------------------------------------------------------

        const filterButtons = model.STATE_FILTERS.map(name => '<button type="button" class="dm-filter" data-filter="' + name + '" aria-pressed="false"><span>' + esc(t('filter_' + name)) + '</span><span class="dm-filter-count" data-count="' + name + '">0</span></button>').join('');
        const columns = ['status', 'name', 'image', 'ports'].map(name => '<th scope="col" role="columnheader" class="dm-col-' + name + '">' + esc(t('col_' + name)) + '</th>').join('')
            + '<th scope="col" role="columnheader" class="dm-col-actions"><span class="dm-sr-only">' + esc(t('col_actions')) + '</span></th>';
        host.replaceChildren();
        host.innerHTML = '<div class="dm-app" data-dm-app data-drawer-state="closed"><div class="dm-shell">'
            + '<aside class="dm-rail" aria-label="' + esc(t('rail_label')) + '" data-rail></aside>'
            + '<section class="dm-main" aria-label="' + esc(t('nav_containers')) + '">'
            + '<div class="dm-toolbar"><div class="dm-search">' + views.glyph('search') + '<input type="search" class="dm-search-input" data-search autocomplete="off" spellcheck="false" placeholder="' + esc(t('search_placeholder')) + '" aria-label="' + esc(t('search_label')) + '"></div>'
            + '<div class="dm-filters" role="group" aria-label="' + esc(t('filter_group')) + '">' + filterButtons + '</div>'
            + '<span class="dm-result-count" data-result-count></span><span class="dm-readonly-chip" data-readonly-chip hidden>' + esc(t('readonly_chip')) + '</span>'
            + '<button type="button" class="dm-icon-btn" data-action="refresh" data-refresh aria-label="' + esc(t('refresh')) + '" title="' + esc(t('refresh')) + '">' + views.glyph('refresh') + '</button></div>'
            + '<div class="dm-banners" data-banners></div>'
            + '<div class="dm-content"><div class="dm-table-scroll" data-table-scroll>'
            + '<table class="dm-table" role="treegrid" aria-label="' + esc(t('nav_containers')) + '" data-table hidden><thead><tr role="row">' + columns + '</tr></thead><tbody data-tbody></tbody></table>'
            + '<div class="dm-empty" data-empty hidden></div></div>'
            + '<section class="dm-drawer" data-drawer aria-labelledby="' + uid + '-drawer-title" hidden></section></div></section></div>'
            + '<dialog class="dm-dialog" data-dialog aria-labelledby="' + uid + '-dialog-title"><button type="button" class="dm-icon-btn dm-dialog-close" data-dialog-dismiss aria-label="' + esc(dt('desktop.close')) + '">' + views.glyph('close') + '</button><div data-dialog-content></div></dialog>'
            + '<div class="dm-toasts" data-toasts></div><div class="dm-sr-only" role="status" aria-live="polite" data-live></div></div>';
        const root = host.querySelector('[data-dm-app]');
        const q = selector => root.querySelector(selector);
        const railEl = q('[data-rail]'), searchEl = q('[data-search]'), bannersEl = q('[data-banners]'), scrollEl = q('[data-table-scroll]'), tableEl = q('[data-table]'), tbodyEl = q('[data-tbody]');
        const emptyEl = q('[data-empty]'), drawerEl = q('[data-drawer]'), toastsEl = q('[data-toasts]'), liveEl = q('[data-live]'), refreshEl = q('[data-refresh]');
        const countEl = q('[data-result-count]'), readonlyEl = q('[data-readonly-chip]');

        // ---- helpers ---------------------------------------------------------------------------------------------

        function announce(text) {
            if (disposed || !text) return;
            clearTimeout(announceTimer);
            announceTimer = setTimeout(() => { if (!disposed) liveEl.textContent = text; }, 150);
        }
        function removeToast(el) {
            clearTimeout(toastTimers.get(el));
            toastTimers.delete(el);
            el.remove();
        }
        function toast(message, tone) {
            if (disposed || !message) return;
            const el = document.createElement('div');
            el.className = 'dm-toast';
            el.dataset.tone = tone || 'info';
            el.setAttribute('role', tone === 'error' ? 'alert' : 'status');
            el.textContent = message;
            // A modal dialog sits in the top layer above the app, so toasts raised while it is open go inside it.
            const place = dialogs.isOpen() ? q('[data-dialog]') : root;
            if (toastsEl.parentNode !== place) place.append(toastsEl);
            toastsEl.append(el);
            while (toastsEl.children.length > 3) removeToast(toastsEl.firstElementChild);
            toastTimers.set(el, setTimeout(() => removeToast(el), tone === 'error' ? 8000 : 4000));
        }
        function savePrefsSoon() {
            clearTimeout(prefsTimer);
            prefsTimer = setTimeout(() => savePrefs({ scope: state.scope, filter: ui.filter, collapsed: [...ui.collapsed].slice(0, 200) }), 300);
        }
        // Keeps the focus on the same control when a part of the app is repainted (controls carry data-focus-id).
        function keepFocus(container, mutate) {
            const active = document.activeElement;
            const focusId = active && container.contains(active) ? active.dataset.focusId || '' : '';
            mutate();
            if (!focusId) return;
            const find = value => container.querySelector('[data-focus-id="' + CSS.escape(value) + '"]');
            let next = find(focusId);
            if (!next && focusId.startsWith('act:')) next = find('row:' + focusId.slice(4, focusId.lastIndexOf(':')));
            next?.focus({ preventScroll: true });
        }
        function restoreFocus(target) {
            if (disposed) return;
            if (target && target.isConnected && typeof target.focus === 'function') { target.focus({ preventScroll: true }); return; }
            (tbodyEl.querySelector('[data-nav-key][tabindex="0"]') || searchEl).focus({ preventScroll: true });
        }
        const findRow = key => model.buildRows(state.hosts, state.byHost, 'all').find(row => row.key === key) || null;
        const rowEl = key => tbodyEl.querySelector('[data-nav-key="' + CSS.escape(key) + '"]');
        const isTextField = el => !!el && (el.matches?.('input, textarea, select') || el.isContentEditable);

        const dialogs = Dialogs.create({ t, dt, esc, model, store, root, views, toast, announce, uid, restoreFocus });

        // ---- snapshot and painting -------------------------------------------------------------------------------

        function snapshot() {
            const hosts = state.hosts;
            const scopeHosts = state.scope === 'all' ? hosts : hosts.filter(item => item.id === state.scope);
            const rows = model.buildRows(hosts, state.byHost, state.scope);
            const visible = model.filterRows(rows, { query: ui.query, filter: ui.filter });
            const multi = state.scope === 'all' && hosts.length > 1;
            const groups = model.groupRows(visible, { multiHost: multi, hostOrder: hosts.map(item => item.id) });
            return { state, hosts, scopeHosts, rows, visible, groups, headers: model.showGroupHeaders(groups), collapsed: ui.collapsed, multi, counts: model.counts(rows, ui.query), canMutate: store.canMutate(), canEditHosts: store.canEditHosts() };
        }

        function applyRoving() {
            const items = [...tbodyEl.querySelectorAll('[data-nav-key]')];
            if (!items.length) return;
            const active = items.find(el => el.dataset.navKey === ui.activeKey) || items[0];
            ui.activeKey = active.dataset.navKey;
            for (const el of items) {
                const on = el === active;
                el.tabIndex = on ? 0 : -1;
                for (const button of el.querySelectorAll('[data-row-btn]')) button.tabIndex = on ? 0 : -1;
            }
        }

        function paintToolbar(snap) {
            for (const button of root.querySelectorAll('[data-filter]')) button.setAttribute('aria-pressed', String(button.dataset.filter === ui.filter));
            for (const el of root.querySelectorAll('[data-count]')) el.textContent = String(snap.counts[el.dataset.count]);
            const text = snap.rows.length || snap.state.hostsLoaded ? t('count_containers', { shown: snap.visible.length, total: snap.rows.length }) : '';
            countEl.textContent = text;
            readonlyEl.hidden = snap.canMutate;
            readonlyEl.title = snap.state.dockerReadonly ? t('readonly_note') : t('readonly_desktop');
            const busy = store.pending();
            refreshEl.classList.toggle('is-spinning', busy);
            refreshEl.setAttribute('aria-busy', String(busy));
            if (text && text !== lastCount && (ui.query || ui.filter !== 'all')) announce(text);
            lastCount = text;
        }

        function paintDrawer(snap) {
            const detail = state.detail;
            if (!detail) {
                if (!drawerEl.hidden) { drawerEl.hidden = true; drawerEl.innerHTML = ''; ui.drawer = ''; root.dataset.drawerState = 'closed'; }
                return;
            }
            const row = snap.rows.find(item => item.key === detail.key);
            if (!row) { store.closeDetail(); return; }
            const html = views.drawer(snap, row);
            const opening = drawerEl.hidden;
            if (html !== ui.drawer) {
                ui.drawer = html;
                keepFocus(drawerEl, () => { drawerEl.innerHTML = html; });
            }
            drawerEl.hidden = false;
            root.dataset.drawerState = 'open';
            const log = drawerEl.querySelector('[data-log]');
            const mark = detail.key + ':' + detail.logs.text.length;
            if (log && mark !== ui.logMark) { ui.logMark = mark; log.scrollTop = log.scrollHeight; }
            if (opening || ui.focusDrawer) {
                if (ui.focusDrawer) drawerEl.querySelector('[role="tab"][tabindex="0"]')?.focus({ preventScroll: true });
                ui.focusDrawer = false;
            }
        }

        function paint() {
            frame = 0;
            if (disposed) return;
            const snap = snapshot();
            root.dataset.readonly = String(!snap.canMutate);
            root.dataset.multi = String(snap.multi);
            root.dataset.disabled = String(state.disabled);
            const railHtml = views.rail(snap);
            if (railHtml !== ui.rail) { ui.rail = railHtml; keepFocus(railEl, () => { railEl.innerHTML = railHtml; }); }
            paintToolbar(snap);
            const bannerHtml = state.disabled ? '' : views.banners(snap);
            if (bannerHtml !== ui.banners) { ui.banners = bannerHtml; keepFocus(bannersEl, () => { bannersEl.innerHTML = bannerHtml; }); }
            const emptyHtml = views.empty(snap);
            const bodyHtml = emptyHtml || !snap.visible.length ? '' : views.tbody(snap);
            if (bodyHtml !== ui.tbody) { ui.tbody = bodyHtml; keepFocus(tbodyEl, () => { tbodyEl.innerHTML = bodyHtml; }); applyRoving(); }
            tableEl.hidden = !bodyHtml;
            if (emptyHtml !== ui.empty) { ui.empty = emptyHtml; keepFocus(emptyEl, () => { emptyEl.innerHTML = emptyHtml; }); }
            emptyEl.hidden = !emptyHtml;
            if (ui.pendingFocus) { rowEl(ui.pendingFocus)?.focus({ preventScroll: false }); ui.pendingFocus = ''; }
            paintDrawer(snap);
            const menuSig = ui.filter + '|' + snap.canEditHosts;
            if (menuSig !== ui.menus) { ui.menus = menuSig; syncMenus(snap); }
        }
        function schedulePaint() {
            if (disposed || frame) return;
            frame = requestAnimationFrame(paint);
        }

        function syncMenus() {
            ctx.setWindowMenus?.(id, [{ id: 'docker-manager', label: dt('desktop.app_docker_manager'), items: [
                { label: t('refresh'), icon: 'refresh', action: () => void store.refresh() },
                { type: 'separator' },
                { label: t('manage_hosts'), icon: 'server', action: () => dialogs.openHosts() },
                { label: t('host_add'), disabled: !store.canEditHosts(), action: () => dialogs.openHostForm(null) },
                { type: 'separator' },
                ...model.STATE_FILTERS.map(name => ({ label: t('filter_' + name), checked: ui.filter === name, action: () => setFilter(name) }))
            ] }]);
        }

        // ---- actions ---------------------------------------------------------------------------------------------

        function setFilter(name) {
            if (!model.STATE_FILTERS.includes(name)) return;
            ui.filter = name;
            savePrefsSoon();
            schedulePaint();
        }
        function pickHost(hostId) {
            if (state.detail && hostId !== 'all' && state.detail.hostId !== hostId) store.closeDetail();
            store.setScope(hostId);
            savePrefsSoon();
            schedulePaint();
        }
        function toggleGroup(key) {
            if (ui.collapsed.has(key)) ui.collapsed.delete(key); else ui.collapsed.add(key);
            savePrefsSoon();
            schedulePaint();
        }
        function openDetail(key, tab, focus) {
            const row = findRow(key);
            if (!row) return;
            ui.opener = key;
            ui.focusDrawer = !!focus;
            store.openDetail(row, tab);
        }
        function closeDetail() {
            if (!state.detail) return;
            const key = ui.opener || state.detail.key;
            store.closeDetail();
            requestAnimationFrame(() => { if (!disposed) (rowEl(key) || searchEl).focus({ preventScroll: true }); });
        }

        function reportFailure(label, name, error) {
            toast(t('action_failed', { action: label, name, message: views.errorText(error) }), 'error');
            announce(t('action_failed', { action: label, name, message: views.errorText(error) }));
        }

        // Stop, restart, pause and kill are never confirmed by the server; the client asks first for a protected container.
        async function performAction(key, action) {
            const row = findRow(key);
            if (!row || !store.canMutate()) return;
            const name = model.containerName(row.c), cid = model.containerId(row.c), label = views.actionLabel(action);
            if (model.needsProtectedConfirm(row.c, action) && (!await dialogs.confirmProtectedAction(row, action) || disposed)) return;
            const result = await store.act(row.hostId, cid, action);
            if (disposed) return;
            if (result.ok) { toast(t('action_done', { action: label, name }), 'success'); announce(t('action_done', { action: label, name })); } else if (result.error.code !== 'aborted' && result.error.code !== 'busy') reportFailure(label, name, result.error);
        }

        // A protected container is removed with two requests: the first answers 409 container_protected_confirmation_required
        // with the owner and a message, the repeat carries ?confirm=protected. When the client already knows the container is
        // protected, the remove dialog says so and the first request carries the confirmation.
        async function removeRow(key) {
            const row = findRow(key);
            if (!row || !store.canMutate()) return;
            const name = model.containerName(row.c), cid = model.containerId(row.c);
            if (!await dialogs.confirmRemove(row) || disposed) return;
            const rows = [...tbodyEl.querySelectorAll('.dm-row')], at = rows.findIndex(el => el.dataset.rowKey === key);
            const sibling = rows[at + 1] || rows[at - 1], neighbour = sibling ? sibling.dataset.rowKey : '';
            const running = ['running', 'paused', 'restarting'].includes(String(row.c.state).toLowerCase());
            let result = await store.remove(row.hostId, cid, { force: running, confirmProtected: model.isProtected(row.c) });
            if (disposed) return;
            if (!result.ok && result.error.code === 'container_protected_confirmation_required') {
                if (!await dialogs.confirmProtectedAction(row, 'remove', result.error) || disposed) return;
                result = await store.remove(row.hostId, cid, { force: running, confirmProtected: true });
                if (disposed) return;
            }
            const label = views.actionLabel('remove');
            if (result.ok) {
                if (state.detail && state.detail.key === key) store.closeDetail();
                const done = t('action_done', { action: label, name });
                ui.pendingFocus = neighbour;
                toast(done, 'success');
                announce(done);
                schedulePaint();
            } else if (result.error.code !== 'aborted' && result.error.code !== 'busy') reportFailure(label, name, result.error);
        }

        async function retryHost(hostId) {
            const hostRecord = state.hosts.find(item => item.id === hostId);
            if (!hostRecord) return;
            try { await store.testHost(hostId); } catch (error) { /* the banner shows the reason after the reload */ }
            await store.reloadContainers(hostId);
        }

        async function copyText(text) {
            try {
                if (!navigator.clipboard || !navigator.clipboard.writeText) throw new Error('no clipboard');
                await navigator.clipboard.writeText(text);
                toast(t('copied'), 'success');
            } catch (error) { toast(t('copy_failed'), 'error'); }
        }

        function showRowMenu(key, x, y) {
            const row = findRow(key);
            if (!row || typeof ctx.showContextMenu !== 'function') return;
            const actions = model.actionsFor(row.c, !store.canMutate());
            const glyphKey = { start: 'run', unpause: 'run', stop: 'stop', restart: 'refresh', kill: 'x' };
            const entry = (action, danger) => ({ label: views.actionLabel(action), icon: glyphKey[action], action: () => void performAction(key, action), danger: !!danger });
            const items = [
                { label: t('action_open_details'), icon: 'info', action: () => openDetail(key, 'overview', true) },
                { label: t('action_logs'), icon: 'terminal', action: () => openDetail(key, 'logs', true) },
                { label: t('action_inspect'), icon: 'info', action: () => openDetail(key, 'inspect', true) }
            ];
            const mutating = [];
            if (actions.toggle) mutating.push(entry(actions.toggle));
            if (actions.restart) mutating.push(entry('restart'));
            for (const action of actions.menu) mutating.push(entry(action, action === 'kill'));
            if (actions.remove) mutating.push({ label: views.actionLabel('remove'), icon: 'trash', action: () => void removeRow(key) });
            if (mutating.length) items.push({ separator: true }, ...mutating);
            ctx.showContextMenu(x, y, items);
        }

        // ---- events ----------------------------------------------------------------------------------------------

        function onAction(el) {
            const key = el.dataset.rowKey || '';
            switch (el.dataset.action) {
                case 'pick-host': pickHost(el.dataset.hostId); break;
                case 'manage-hosts': dialogs.openHosts(); break;
                case 'refresh': void store.refresh(); break;
                case 'toggle-group': toggleGroup(el.dataset.groupKey); break;
                case 'act': void performAction(key, el.dataset.act); break;
                case 'remove': void removeRow(key); break;
                case 'logs': openDetail(key, 'logs', true); break;
                case 'more': { const rect = el.getBoundingClientRect(); showRowMenu(key, rect.left, rect.bottom + 4); break; }
                case 'close-detail': closeDetail(); break;
                case 'tab': store.setDetailTab(el.dataset.tab); break;
                case 'logs-reload': void store.loadLogs(); break;
                case 'copy-logs': if (state.detail) void copyText(views.stripAnsi(state.detail.logs.text)); break;
                case 'copy-inspect': if (state.detail && state.detail.inspect.status === 'ok') void copyText(JSON.stringify(state.detail.inspect.data, null, 2)); break;
                case 'retry-host': void retryHost(el.dataset.hostId); break;
                case 'edit-host': { const target = state.hosts.find(item => item.id === el.dataset.hostId); if (target) dialogs.openHostForm(target); break; }
                case 'clear-filters': ui.query = ''; searchEl.value = ''; ui.filter = 'all'; savePrefsSoon(); schedulePaint(); break;
                case 'open-settings': window.open('/config#docker', '_blank', 'noopener'); break;
                default: break;
            }
        }

        root.addEventListener('click', event => {
            const el = event.target.closest('[data-action]');
            if (el && root.contains(el)) { onAction(el); return; }
            const filter = event.target.closest('[data-filter]');
            if (filter) { setFilter(filter.dataset.filter); return; }
            const rowNode = event.target.closest('.dm-row');
            if (rowNode && !event.target.closest('button, a, input, select, summary')) openDetail(rowNode.dataset.rowKey, undefined, false);
        }, { signal });
        searchEl.addEventListener('input', () => { ui.query = searchEl.value; schedulePaint(); }, { signal });
        root.addEventListener('change', event => {
            if (event.target.matches('[data-log-tail]')) void store.loadLogs(Number(event.target.value));
        }, { signal });
        tbodyEl.addEventListener('focusin', event => {
            const nav = event.target.closest('[data-nav-key]');
            if (nav && nav.dataset.navKey !== ui.activeKey) { ui.activeKey = nav.dataset.navKey; applyRoving(); }
        }, { signal });

        function moveFocus(to) {
            const items = [...tbodyEl.querySelectorAll('[data-nav-key]')];
            const target = items[Math.max(0, Math.min(items.length - 1, to))];
            if (target) target.focus();
        }
        scrollEl.addEventListener('keydown', event => {
            const nav = event.target.closest('[data-nav-key]');
            if (!nav || event.ctrlKey || event.metaKey || event.altKey) return;
            const items = [...tbodyEl.querySelectorAll('[data-nav-key]')], index = items.indexOf(nav);
            const onControl = event.target !== nav;
            const isGroup = nav.classList.contains('dm-group-row');
            switch (event.key) {
                case 'ArrowDown': event.preventDefault(); moveFocus(index + 1); break;
                case 'ArrowUp': event.preventDefault(); moveFocus(index - 1); break;
                case 'Home': event.preventDefault(); moveFocus(0); break;
                case 'End': event.preventDefault(); moveFocus(items.length - 1); break;
                case 'ArrowRight': if (isGroup && !onControl) { event.preventDefault(); if (ui.collapsed.has(nav.dataset.groupKey)) toggleGroup(nav.dataset.groupKey); } break;
                case 'ArrowLeft':
                    if (isGroup && !onControl) { event.preventDefault(); if (!ui.collapsed.has(nav.dataset.groupKey)) toggleGroup(nav.dataset.groupKey); }
                    else if (!isGroup && !onControl) {
                        const group = items.slice(0, index).reverse().find(el => el.classList.contains('dm-group-row'));
                        if (group) { event.preventDefault(); group.focus(); }
                    }
                    break;
                case 'Enter': case ' ':
                    if (onControl) break;
                    event.preventDefault();
                    if (isGroup) toggleGroup(nav.dataset.groupKey); else openDetail(nav.dataset.rowKey, undefined, true);
                    break;
                case 'Delete': if (!isGroup) { event.preventDefault(); void removeRow(nav.dataset.rowKey); } break;
                case 'ContextMenu': case 'F10':
                    if (event.key === 'F10' && !event.shiftKey) break;
                    if (!isGroup) { event.preventDefault(); const rect = nav.getBoundingClientRect(); showRowMenu(nav.dataset.rowKey, rect.left + 24, rect.bottom - 4); }
                    break;
                default: break;
            }
        }, { signal });

        railEl.addEventListener('keydown', event => {
            const option = event.target.closest('[role="option"]');
            if (!option) return;
            const options = [...railEl.querySelectorAll('[role="option"]')], index = options.indexOf(option);
            const to = { ArrowDown: index + 1, ArrowUp: index - 1, Home: 0, End: options.length - 1 }[event.key];
            if (to === undefined) return;
            event.preventDefault();
            const target = options[Math.max(0, Math.min(options.length - 1, to))];
            if (target && target !== option) { pickHost(target.dataset.hostId); requestAnimationFrame(() => railEl.querySelector('[data-focus-id="host:' + CSS.escape(target.dataset.hostId) + '"]')?.focus()); }
        }, { signal });

        drawerEl.addEventListener('keydown', event => {
            const tab = event.target.closest('[role="tab"]');
            if (!tab) return;
            const tabs = [...drawerEl.querySelectorAll('[role="tab"]')], index = tabs.indexOf(tab);
            const to = { ArrowRight: (index + 1) % tabs.length, ArrowLeft: (index - 1 + tabs.length) % tabs.length, Home: 0, End: tabs.length - 1 }[event.key];
            if (to === undefined) return;
            event.preventDefault();
            store.setDetailTab(tabs[to].dataset.tab);
            requestAnimationFrame(() => drawerEl.querySelector('[role="tab"][aria-selected="true"]')?.focus({ preventScroll: true }));
        }, { signal });

        root.addEventListener('keydown', event => {
            if (event.key === 'Escape' && !dialogs.isOpen()) {
                if (event.target === searchEl && searchEl.value) { event.preventDefault(); searchEl.value = ''; ui.query = ''; schedulePaint(); return; }
                if (state.detail && (drawerEl.contains(event.target) || scrollEl.contains(event.target))) { event.preventDefault(); event.stopPropagation(); closeDetail(); }
                return;
            }
            if (event.key === '/' && !event.ctrlKey && !event.metaKey && !event.altKey && !isTextField(event.target) && !dialogs.isOpen()) { event.preventDefault(); searchEl.focus(); }
        }, { signal });

        if (typeof ctx.wireContextMenuBoundary === 'function') {
            ctx.wireContextMenuBoundary(root, {
                onContextMenu: event => {
                    const node = event.target.closest('.dm-row');
                    if (!node) return false;
                    showRowMenu(node.dataset.rowKey, event.clientX, event.clientY);
                    return true;
                }
            });
        }

        // ---- start -----------------------------------------------------------------------------------------------

        const unsubscribe = store.subscribe(schedulePaint);
        searchEl.value = ui.query;
        paint();
        store.attach({ root, win: host.closest('.vd-window') });

        instances.set(id, () => {
            if (disposed) return;
            disposed = true;
            lifetime.abort();
            cancelAnimationFrame(frame);
            clearTimeout(announceTimer);
            clearTimeout(prefsTimer);
            savePrefs({ scope: state.scope, filter: ui.filter, collapsed: [...ui.collapsed].slice(0, 200) });
            for (const el of [...toastTimers.keys()]) removeToast(el);
            unsubscribe();
            dialogs.dispose();
            store.dispose();
            ctx.setWindowMenus?.(id, []);
            root.replaceChildren();
        });
    }

    window.DockerManagerApp = { render, dispose };
})();
