/* Docker Manager app: the composition root. It builds the skeleton (rail, toolbar, banners, the Stacks list and the stack
   pane, the container table and its drawer, the new-stack view, the progress drawer, the dialog host), wires the store to the
   views, keeps the focus across repaints and owns the keyboard handling, the context menus, the window menus, the file drop
   and the toasts. Pure logic lives in docker-manager-model.js and -stack-model.js, requests and polling in -store.js, markup
   in -views.js and -stacks.js, the editors in -editor.js, the progress drawer in -ops.js, the command palette in -palette.js,
   the new-stack flow in -newstack.js and the dialogs in -dialogs.js. */
(function () {
    'use strict';

    const PREFS_KEY = 'docker-manager.ui.v1';
    const instances = new Map(), warned = new Set();
    function dispose(id) { instances.get(id)?.(); instances.delete(id); }

    function loadPrefs(model, sm) {
        const fallback = { scope: 'all', filter: 'all', collapsed: [], section: 'stacks' };
        try {
            const saved = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
            return {
                scope: typeof saved.scope === 'string' && saved.scope ? saved.scope : 'all',
                filter: model.STATE_FILTERS.includes(saved.filter) ? saved.filter : 'all',
                collapsed: Array.isArray(saved.collapsed) ? saved.collapsed.filter(key => typeof key === 'string').slice(0, 200) : [],
                section: sm.SECTIONS.includes(saved.section) ? saved.section : 'stacks'
            };
        } catch (error) { return fallback; }
    }
    function savePrefs(prefs) { try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch (error) { /* storage may be blocked */ } }

    function render(host, id, ctx) {
        dispose(id);
        const w = window;
        const model = w.DockerManagerModel, sm = w.DockerManagerStackModel, Views = w.DockerManagerViews, Stacks = w.DockerManagerStacks, Editor = w.DockerManagerEditor, Ops = w.DockerManagerOps;
        const Palette = w.DockerManagerPalette, NewStack = w.DockerManagerNewStack, Dialogs = w.DockerManagerDialogs, Store = w.DockerManagerStore;
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
        const prefs = loadPrefs(model, sm);
        const lifetime = new AbortController(), signal = lifetime.signal;
        const store = Store.create({ api: ctx.api, readonly: !!ctx.readonly, scope: prefs.scope, section: prefs.section });
        const state = store.state;
        const views = Views.create({ t, esc, model, stackModel: sm, lang, uid, logTails: Store.LOG_TAILS });
        const stacks = Stacks.create({ t, esc, model, stackModel: sm, views, uid });
        const ui = { query: '', filter: prefs.filter, section: prefs.section, collapsed: new Set(prefs.collapsed), activeKey: '', stackNav: '', rail: '', banners: '', tbody: '', empty: '', drawer: '', list: '', top: '', overview: '', shellKey: '', ops: '', menus: '',
            logMark: '', focusDrawer: false, focusStack: false, pendingFocus: '', opener: '', stackOpener: '', mark: null, count: '' };
        let disposed = false, frame = 0, announceTimer = 0, prefsTimer = 0, lastCount = '';
        const toastTimers = new Map();

        // ---- skeleton --------------------------------------------------------------------------------------------

        const filterButtons = model.STATE_FILTERS.map(name => '<button type="button" class="dm-filter" data-filter="' + name + '" aria-pressed="false"><span>' + esc(t('filter_' + name)) + '</span><span class="dm-filter-count" data-count="' + name + '">0</span></button>').join('');
        const columns = ['status', 'name', 'image', 'ports'].map(name => '<th scope="col" role="columnheader" class="dm-col-' + name + '">' + esc(t('col_' + name)) + '</th>').join('')
            + '<th scope="col" role="columnheader" class="dm-col-actions"><span class="dm-sr-only">' + esc(t('col_actions')) + '</span></th>';
        host.replaceChildren();
        host.innerHTML = '<div class="dm-app" data-dm-app data-drawer-state="closed" data-section="' + ui.section + '"><div class="dm-shell">'
            + '<aside class="dm-rail" aria-label="' + esc(t('rail_label')) + '" data-rail></aside>'
            + '<section class="dm-main" aria-label="' + esc(t('nav_stacks')) + '" data-main>'
            + '<div class="dm-toolbar"><div class="dm-search">' + views.glyph('search') + '<input type="search" class="dm-search-input" data-search autocomplete="off" spellcheck="false" placeholder="' + esc(t('search_placeholder')) + '" aria-label="' + esc(t('search_label')) + '"></div>'
            + '<button type="button" class="dm-btn is-primary" data-action="new-stack" data-new-stack data-focus-id="new-stack">' + views.glyph('plus') + '<span>' + esc(t('new_stack')) + '</span></button>'
            + '<div class="dm-filters" role="group" aria-label="' + esc(t('filter_group')) + '" data-filters>' + filterButtons + '</div>'
            + '<span class="dm-result-count" data-result-count></span><span class="dm-readonly-chip" data-readonly-chip hidden>' + esc(t('readonly_chip')) + '</span>'
            + '<button type="button" class="dm-icon-btn" data-action="palette" data-focus-id="palette" aria-label="' + esc(t('palette_button')) + '" title="' + esc(t('palette_button')) + ' (Ctrl+K)">' + views.glyph('command') + '</button>'
            + '<button type="button" class="dm-icon-btn" data-action="refresh" data-refresh aria-label="' + esc(t('refresh')) + '" title="' + esc(t('refresh')) + '">' + views.glyph('refresh') + '</button></div>'
            + '<div class="dm-banners" data-banners></div>'
            + '<div class="dm-content"><div class="dm-table-scroll" data-table-scroll>'
            + '<table class="dm-table" role="treegrid" aria-label="' + esc(t('nav_containers')) + '" data-table hidden><thead><tr role="row">' + columns + '</tr></thead><tbody data-tbody></tbody></table>'
            + '<div class="dm-empty" data-empty hidden></div></div>'
            + '<div class="dm-stack-scroll" data-stack-scroll hidden><div class="dm-stack-list" data-stack-list aria-label="' + esc(t('nav_stacks')) + '"></div></div>'
            + '<section class="dm-drawer" data-drawer aria-labelledby="' + uid + '-drawer-title" hidden></section>'
            + '<section class="dm-stackpane" data-stack-pane aria-labelledby="' + uid + '-stack-title" hidden><div data-stack-top></div><div class="dm-sd-body" data-stack-shell></div></section>'
            + '<section class="dm-ns-wrap" data-newstack hidden></section></div>'
            + '<section class="dm-ops" data-ops aria-label="' + esc(t('ops_label')) + '"></section></section></div>'
            + '<dialog class="dm-dialog" data-dialog aria-labelledby="' + uid + '-dialog-title"><button type="button" class="dm-icon-btn dm-dialog-close" data-dialog-dismiss aria-label="' + esc(dt('desktop.close')) + '">' + views.glyph('close') + '</button><div data-dialog-content></div></dialog>'
            + '<div class="dm-toasts" data-toasts></div><div class="dm-sr-only" role="status" aria-live="polite" data-live></div></div>';
        const root = host.querySelector('[data-dm-app]');
        const q = selector => root.querySelector(selector);
        const railEl = q('[data-rail]'), searchEl = q('[data-search]'), bannersEl = q('[data-banners]'), scrollEl = q('[data-table-scroll]'), tableEl = q('[data-table]'), tbodyEl = q('[data-tbody]');
        const emptyEl = q('[data-empty]'), drawerEl = q('[data-drawer]'), toastsEl = q('[data-toasts]'), liveEl = q('[data-live]'), refreshEl = q('[data-refresh]');
        const countEl = q('[data-result-count]'), readonlyEl = q('[data-readonly-chip]'), mainEl = q('[data-main]'), filtersEl = q('[data-filters]'), newStackBtn = q('[data-new-stack]');
        const stackScrollEl = q('[data-stack-scroll]'), stackListEl = q('[data-stack-list]'), paneEl = q('[data-stack-pane]'), topEl = q('[data-stack-top]'), shellEl = q('[data-stack-shell]');
        const newstackEl = q('[data-newstack]'), opsEl = q('[data-ops]'), dialogEl = q('[data-dialog]');

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
            const place = dialogs.isOpen() ? dialogEl : root;
            if (toastsEl.parentNode !== place) place.append(toastsEl);
            toastsEl.append(el);
            while (toastsEl.children.length > 3) removeToast(toastsEl.firstElementChild);
            toastTimers.set(el, setTimeout(() => removeToast(el), tone === 'error' ? 8000 : 4000));
        }
        // ...and back into the app when the dialog closes, so the toasts raised meanwhile do not stay hidden in a closed dialog.
        dialogEl.addEventListener('close', () => { if (toastsEl.parentNode !== root) root.append(toastsEl); }, { signal });
        const currentPrefs = () => ({ scope: state.scope, filter: ui.filter, collapsed: [...ui.collapsed].slice(0, 200), section: ui.section });
        function savePrefsSoon() {
            clearTimeout(prefsTimer);
            prefsTimer = setTimeout(() => savePrefs(currentPrefs()), 300);
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
        const splitKey = key => ({ hostId: key.slice(0, key.indexOf('/')), name: key.slice(key.indexOf('/') + 1) });
        const hostOf = hostId => state.hosts.find(item => item.id === hostId) || { id: hostId, name: hostId };

        const dialogs = Dialogs.create({ t, dt, esc, model, stackModel: sm, store, root, views, toast, announce, uid, restoreFocus });
        const editorFactory = Editor.create({ t, esc, stackModel: sm, views, uid });
        const ops = Ops.create({ t, esc, model, stackModel: sm, views, store, toast, announce, uid });
        const palette = Palette.create({ t, esc, stackModel: sm, views, root, uid });
        const workbench = editorFactory.createWorkbench({ store, dialogs, toast, announce, canMutate: () => store.canMutate(), onChange: () => schedulePaint(), onDeploy: () => void stackOp(state.stack && state.stack.key, 'deploy') });
        const newStack = NewStack.create({
            t, esc, model, stackModel: sm, views, editor: editorFactory, createLinter: Editor.createLinter, store, dialogs, toast, announce, uid, el: newstackEl,
            canMutate: () => store.canMutate(), requestClose: () => closeNewStack(), onClosed: () => schedulePaint(), onPrepare: hostId => ops.prepare(hostId),
            onCreated: (hostId, name, body) => {
                if (body && body.operation) ops.setOpen(true);
                setSection('stacks');
                store.openStack(hostId, name, 'overview');
                schedulePaint();
            }
        });

        // ---- snapshot and painting -------------------------------------------------------------------------------

        function snapshot() {
            const hosts = state.hosts;
            const scopeHosts = state.scope === 'all' ? hosts : hosts.filter(item => item.id === state.scope);
            const rows = model.buildRows(hosts, state.byHost, state.scope);
            const visible = model.filterRows(rows, { query: ui.query, filter: ui.filter });
            const multi = state.scope === 'all' && hosts.length > 1;
            const groups = model.groupRows(visible, { multiHost: multi, hostOrder: hosts.map(item => item.id) });
            const hostStates = {}, stackOps = {}, prepareActive = {};
            for (const item of scopeHosts) hostStates[item.id] = sm.hostStacksState(state.stacks[item.id]);
            for (const entry of state.ops) {
                if (!sm.isActiveOp(entry.op)) continue;
                if (entry.op.kind === 'prepare') prepareActive[entry.op.host_id] = true;
                else if (entry.op.stack) stackOps[entry.op.host_id + '/' + entry.op.stack] = entry;
            }
            return {
                state, hosts, scopeHosts, rows, visible, groups, headers: model.showGroupHeaders(groups), collapsed: ui.collapsed, multi, counts: model.counts(rows, ui.query),
                canMutate: store.canMutate(), canEditHosts: store.canEditHosts(), section: ui.section, query: ui.query, hostStates, stackOps, prepareActive,
                sgroups: sm.stackGroups(hosts, state.stacks, rows, { scope: state.scope, query: ui.query }), stackCounts: sm.stackCounts(hosts, state.stacks, state.scope),
                dirty: workbench.dirtyState(), pageHost: window.location.hostname, now: Date.now()
            };
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
        function applyStackRoving() {
            const items = [...stackListEl.querySelectorAll('[data-snav]')];
            if (!items.length) return;
            const active = items.find(el => el.dataset.snav === ui.stackNav) || items[0];
            ui.stackNav = active.dataset.snav;
            for (const el of items) el.tabIndex = el === active ? 0 : -1;
        }

        function paintToolbar(snap) {
            const stacksView = ui.section === 'stacks';
            for (const button of root.querySelectorAll('[data-filter]')) button.setAttribute('aria-pressed', String(button.dataset.filter === ui.filter));
            for (const el of root.querySelectorAll('[data-count]')) el.textContent = String(snap.counts[el.dataset.count]);
            filtersEl.hidden = stacksView;
            newStackBtn.hidden = !stacksView;
            newStackBtn.disabled = !snap.canMutate;
            const g = snap.sgroups;
            const text = stacksView ? (snap.state.hostsLoaded ? t('count_stacks', { shown: g.stacks.length + g.external.length, total: snap.stackCounts.total }) : '')
                : snap.rows.length || snap.state.hostsLoaded ? t('count_containers', { shown: snap.visible.length, total: snap.rows.length }) : '';
            countEl.textContent = text;
            readonlyEl.hidden = snap.canMutate;
            readonlyEl.title = snap.state.dockerReadonly ? t('readonly_note') : t('readonly_desktop');
            const busy = store.pending();
            refreshEl.classList.toggle('is-spinning', busy);
            refreshEl.setAttribute('aria-busy', String(busy));
            mainEl.setAttribute('aria-label', t(stacksView ? 'nav_stacks' : 'nav_containers'));
            searchEl.placeholder = stacksView ? t('search_placeholder_stacks') : t('search_placeholder');
            searchEl.setAttribute('aria-label', stacksView ? t('search_label_stacks') : t('search_label'));
            if (text && text !== lastCount && (ui.query || (!stacksView && ui.filter !== 'all'))) announce(text);
            lastCount = text;
        }

        function paintDrawer(snap) {
            const detail = state.detail;
            if (!detail) {
                if (!drawerEl.hidden) { drawerEl.hidden = true; drawerEl.innerHTML = ''; ui.drawer = ''; }
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
            drawerEl.hidden = !!newStack.isOpen();
            const log = drawerEl.querySelector('[data-log]');
            const mark = detail.key + ':' + detail.logs.text.length;
            if (log && mark !== ui.logMark) { ui.logMark = mark; log.scrollTop = log.scrollHeight; }
            if (opening || ui.focusDrawer) {
                if (ui.focusDrawer) drawerEl.querySelector('[role="tab"][tabindex="0"]')?.focus({ preventScroll: true });
                ui.focusDrawer = false;
            }
        }

        function paintContainers(snap) {
            const emptyHtml = views.empty(snap);
            const bodyHtml = emptyHtml || !snap.visible.length ? '' : views.tbody(snap);
            if (bodyHtml !== ui.tbody) { ui.tbody = bodyHtml; keepFocus(tbodyEl, () => { tbodyEl.innerHTML = bodyHtml; }); applyRoving(); }
            tableEl.hidden = !bodyHtml;
            if (emptyHtml !== ui.empty) { ui.empty = emptyHtml; keepFocus(emptyEl, () => { emptyEl.innerHTML = emptyHtml; }); }
            emptyEl.hidden = !emptyHtml;
            if (ui.pendingFocus) { rowEl(ui.pendingFocus)?.focus({ preventScroll: false }); ui.pendingFocus = ''; }
        }

        function paintStacks(snap) {
            const html = views.gate(snap) || stacks.list(snap);
            if (html !== ui.list) { ui.list = html; keepFocus(stackListEl, () => { stackListEl.innerHTML = html; }); applyStackRoving(); }
        }

        // The pane of the open stack: header and tabs repaint when their markup changes; the panels (and the editors in them)
        // are built once per opened stack and only shown or hidden afterwards.
        function paintStackPane(snap) {
            const stack = state.stack, show = !!stack && !state.detail && ui.section === 'stacks' && !newStack.isOpen();
            paneEl.hidden = !show;
            if (!stack) {
                if (ui.shellKey) { workbench.close(); ui.shellKey = ''; shellEl.replaceChildren(); topEl.replaceChildren(); ui.top = ui.overview = ''; }
                return;
            }
            if (ui.shellKey && ui.shellKey !== stack.key) { workbench.close(); ui.shellKey = ''; shellEl.replaceChildren(); ui.top = ui.overview = ''; }
            if (stack.status !== 'ok') {
                if (ui.shellKey) { workbench.close(); ui.shellKey = ''; shellEl.replaceChildren(); ui.overview = ''; }
                const problem = stacks.problem(snap);
                if (problem !== ui.top) { ui.top = problem; keepFocus(topEl, () => { topEl.innerHTML = problem; }); }
                return;
            }
            if (ui.shellKey !== stack.key) {
                shellEl.innerHTML = stacks.shell(stack);
                workbench.open(shellEl, stack);
                ui.shellKey = stack.key;
                ui.overview = '';
            }
            workbench.sync();
            const headHtml = stacks.head(snap);
            if (headHtml !== ui.top) { ui.top = headHtml; keepFocus(topEl, () => { topEl.innerHTML = headHtml; }); }
            const overviewEl = shellEl.querySelector('[data-stack-overview]'), overviewHtml = stacks.overview(snap);
            if (overviewEl && overviewHtml !== ui.overview) { ui.overview = overviewHtml; keepFocus(overviewEl, () => { overviewEl.innerHTML = overviewHtml; }); }
            for (const panel of shellEl.querySelectorAll('[data-panel]')) panel.hidden = panel.dataset.panel !== stack.tab;
            if (show) workbench.showTab(stack.tab);
            if (show && ui.mark && ui.mark.key === stack.key) {
                workbench.showTab('compose');
                workbench.markLine(ui.mark.line, ui.mark.message);
                ui.mark = null;
            }
            if (show && ui.focusStack) { topEl.querySelector('[role="tab"][tabindex="0"]')?.focus({ preventScroll: true }); ui.focusStack = false; }
        }

        function paintOps(snap) {
            const html = ops.html(snap);
            if (html !== ui.ops) {
                const keep = new Map([...opsEl.querySelectorAll('[data-op-output]')].map(el => [el.dataset.opOutput, { top: el.scrollTop, bottom: el.scrollHeight - el.scrollTop - el.clientHeight < 24 }]));
                ui.ops = html;
                keepFocus(opsEl, () => { opsEl.innerHTML = html; });
                for (const el of opsEl.querySelectorAll('[data-op-output]')) {
                    const kept = keep.get(el.dataset.opOutput);
                    el.scrollTop = !kept || kept.bottom ? el.scrollHeight : kept.top;
                }
            }
            root.dataset.opsOpen = String(ops.isOpen());
            if (store.activeOps().length) ops.startTicker(opsEl);
        }

        function paint() {
            frame = 0;
            if (disposed) return;
            const snap = snapshot(), creating = newStack.isOpen(), stacksView = ui.section === 'stacks';
            root.dataset.readonly = String(!snap.canMutate);
            root.dataset.multi = String(snap.multi);
            root.dataset.disabled = String(state.disabled);
            root.dataset.section = ui.section;
            root.dataset.creating = String(creating);
            const railHtml = views.rail(snap);
            if (railHtml !== ui.rail) { ui.rail = railHtml; keepFocus(railEl, () => { railEl.innerHTML = railHtml; }); }
            paintToolbar(snap);
            const bannerHtml = state.disabled ? '' : views.banners(snap);
            if (bannerHtml !== ui.banners) { ui.banners = bannerHtml; keepFocus(bannersEl, () => { bannersEl.innerHTML = bannerHtml; }); }
            scrollEl.hidden = stacksView || creating;
            stackScrollEl.hidden = !stacksView || creating;
            if (stacksView) paintStacks(snap); else paintContainers(snap);
            paintDrawer(snap);
            root.dataset.drawerState = state.detail && !creating ? 'open' : 'closed';
            paintStackPane(snap);
            paintOps(snap);
            const menuSig = ui.section + '|' + ui.filter + '|' + snap.canEditHosts + '|' + snap.canMutate;
            if (menuSig !== ui.menus) { ui.menus = menuSig; syncMenus(); }
        }
        function schedulePaint() {
            if (disposed || frame) return;
            frame = requestAnimationFrame(paint);
        }

        function syncMenus() {
            const mutate = store.canMutate();
            ctx.setWindowMenus?.(id, [{ id: 'docker-manager', label: dt('desktop.app_docker_manager'), items: [
                { label: t('refresh'), icon: 'refresh', action: () => void store.refresh({ force: true }) },
                { label: t('new_stack'), icon: 'plus', disabled: !mutate, action: () => void openNewStack() },
                { label: t('palette_button'), action: () => openPalette() },
                { type: 'separator' },
                { label: t('manage_hosts'), icon: 'server', action: () => dialogs.openHosts() },
                { label: t('host_add'), disabled: !store.canEditHosts(), action: () => dialogs.openHostForm(null) },
                { type: 'separator' },
                ...sm.SECTIONS.map(name => ({ label: t('nav_' + name), checked: ui.section === name, action: () => void setSection(name) })),
                { type: 'separator' },
                ...model.STATE_FILTERS.map(name => ({ label: t('filter_' + name), checked: ui.filter === name, action: () => setFilter(name) }))
            ] }]);
        }

        // ---- sections, hosts, filters ----------------------------------------------------------------------------

        function setFilter(name) {
            if (!model.STATE_FILTERS.includes(name)) return;
            ui.filter = name;
            savePrefsSoon();
            schedulePaint();
        }
        function setSection(name) {
            if (!sm.SECTIONS.includes(name) || name === ui.section) return;
            if (state.detail) store.closeDetail();
            ui.section = name;
            store.setSection(name);
            savePrefsSoon();
            schedulePaint();
        }
        async function pickHost(hostId) {
            if (state.detail && hostId !== 'all' && state.detail.hostId !== hostId) store.closeDetail();
            if (state.stack && hostId !== 'all' && state.stack.hostId !== hostId) { if (!await leaveStack()) return; store.closeStack(); }
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
            requestAnimationFrame(() => { if (!disposed) (rowEl(key) || stackListEl.querySelector('[data-action="open-container"][data-row-key="' + CSS.escape(key) + '"]') || searchEl).focus({ preventScroll: true }); });
        }

        // ---- containers ------------------------------------------------------------------------------------------

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
            if (ui.section === 'stacks') await store.loadStacks(hostId, true);
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

        // ---- stacks ----------------------------------------------------------------------------------------------

        // {hostId, name, hostName, dir, status} of a stack from the open pane or the list, or null.
        function stackRef(key) {
            if (!key) return null;
            const open = state.stack && state.stack.key === key ? state.stack.detail : null;
            const listed = sm.stackRows(state.hosts, state.stacks, 'all').find(row => row.key === key);
            const found = open || (listed && listed.stack);
            if (!found) return null;
            const ids = splitKey(key);
            return { hostId: ids.hostId, name: ids.name, hostName: hostOf(ids.hostId).name, dir: found.dir || '', status: found.status };
        }

        async function leaveStack() {
            if (!state.stack || !workbench.anyDirty()) return true;
            return dialogs.confirm({ title: t('discard_title'), message: t('discard_body', { name: state.stack.name }), confirmLabel: t('discard_confirm'), danger: true });
        }
        async function openStack(key, tab, focus) {
            const ref = splitKey(key);
            if (state.stack && state.stack.key !== key && !await leaveStack()) return;
            if (disposed) return;
            if (newStack.isOpen() && !await closeNewStack()) return;
            if (state.detail) store.closeDetail();
            ui.stackOpener = key;
            ui.focusStack = !!focus;
            store.openStack(ref.hostId, ref.name, tab);
            schedulePaint();
        }
        async function closeStackPane() {
            if (!state.stack || !await leaveStack() || disposed) return;
            const key = ui.stackOpener || state.stack.key;
            store.closeStack();
            requestAnimationFrame(() => { if (!disposed) (stackListEl.querySelector('[data-snav="' + CSS.escape(key) + '"]') || searchEl).focus({ preventScroll: true }); });
        }

        // action: deploy | update | restart | stop | pull | down | down_volumes | delete. Destructive ones ask first; deploy and
        // update save the open stack's unsaved edits before they run.
        async function stackOp(key, action) {
            const ref = stackRef(key);
            if (!ref || !store.canMutate()) return;
            if (action === 'delete') {
                const choice = await dialogs.confirmDelete(ref);
                if (choice && !disposed) await ops.remove(ref.hostId, ref.name, choice.volumes);
            } else if (action === 'down_volumes') {
                if (await dialogs.confirmDownVolumes(ref) && !disposed) await ops.run(ref.hostId, ref.name, 'down', { volumes: true });
            } else if (action === 'down') {
                if (await dialogs.confirm({ title: t('down_title', { name: ref.name }), message: t('down_body', { name: ref.name, host: ref.hostName }), confirmLabel: t('action_down'), danger: true }) && !disposed) await ops.run(ref.hostId, ref.name, 'down');
            } else {
                if ((action === 'deploy' || action === 'update') && state.stack && state.stack.key === key && workbench.anyDirty() && !await workbench.save(['compose', 'env'], false)) return;
                await ops.run(ref.hostId, ref.name, sm.OP_FOR_ACTION[action] || action);
            }
            schedulePaint();
        }

        function stackMenuItems(key) {
            const ref = stackRef(key);
            if (!ref) return [];
            const items = [{ label: t('action_open_details'), icon: 'info', action: () => void openStack(key, 'overview', true) }];
            if (!store.canMutate()) return items;
            const a = sm.stackActions(ref);
            items.push({ separator: true }, { label: t('action_' + a.primary), icon: 'run', action: () => void stackOp(key, a.primary) });
            if (a.restart) items.push({ label: t('action_restart'), icon: 'refresh', action: () => void stackOp(key, 'restart') });
            if (a.stop) items.push({ label: t('action_stop'), icon: 'stop', action: () => void stackOp(key, 'stop') });
            if (a.pull) items.push({ label: t('action_pull'), icon: 'refresh', action: () => void stackOp(key, 'pull') });
            if (a.down) items.push({ label: t('action_down'), icon: 'stop', action: () => void stackOp(key, 'down') }, { label: t('action_down_volumes'), icon: 'trash', danger: true, action: () => void stackOp(key, 'down_volumes') });
            items.push({ separator: true }, { label: t('action_delete'), icon: 'trash', danger: true, action: () => void stackOp(key, 'delete') });
            return items;
        }
        function showStackMenu(key, x, y) {
            const items = stackMenuItems(key);
            if (items.length && typeof ctx.showContextMenu === 'function') ctx.showContextMenu(x, y, items);
        }

        async function projectAction(hostId, project, action) {
            try {
                await store.projectAction(hostId, project, action);
                toast(t('action_done', { action: t('action_' + action), name: project }), 'success');
            } catch (error) {
                reportFailure(t('action_' + action), project, error.dm);
            }
        }

        // ---- new stack, palette, drop ----------------------------------------------------------------------------

        async function openNewStack(options) {
            if (!store.canMutate()) { toast(t('readonly_chip'), 'error'); return; }
            if (newStack.isOpen()) { newStack.open(options); return; }
            if (state.detail) store.closeDetail();
            if (ui.section !== 'stacks') setSection('stacks');
            const scoped = state.scope !== 'all';
            newStack.open(Object.assign({ lockHost: scoped, hostId: scoped ? state.scope : undefined }, options));
            schedulePaint();
        }
        async function closeNewStack() {
            if (!newStack.isOpen()) return true;
            if (newStack.isDirty() && !await dialogs.confirm({ title: t('discard_title'), message: t('discard_new_body'), confirmLabel: t('discard_confirm'), danger: true })) return false;
            newStack.close();
            schedulePaint();
            requestAnimationFrame(() => { if (!disposed) newStackBtn.focus({ preventScroll: true }); });
            return true;
        }

        function openPalette() {
            if (disposed || dialogs.isOpen() || palette.isOpen()) return;
            palette.open(() => {
                const hostStates = {};
                for (const item of state.hosts) hostStates[item.id] = sm.hostStacksState(state.stacks[item.id]);
                return sm.paletteItems({
                    hosts: state.hosts, stackRows: sm.stackRows(state.hosts, state.stacks, state.scope), multi: state.scope === 'all' && state.hosts.length > 1, hostStates, scope: state.scope,
                    canMutate: store.canMutate(), label: (action, params) => t('palette_' + action, params)
                });
            }, runCommand);
        }
        function runCommand(item) {
            switch (item.action) {
                case 'open': setSection('stacks'); void openStack(item.stackKey, 'overview', true); break;
                case 'deploy': case 'update': case 'restart': case 'stop': void stackOp(item.stackKey, item.action); break;
                case 'new_stack': void openNewStack({ hostId: item.hostId, lockHost: false }); break;
                case 'prepare': void ops.prepare(item.hostId); break;
                case 'switch_host': case 'switch_all': void pickHost(item.hostId); break;
                case 'refresh': void store.refresh({ force: true }); break;
                default: break;
            }
        }

        // Files dragged from the computer onto the window become the compose file and the .env of a new stack. Other drags
        // (and every drag in a read-only window) are left to the desktop.
        const hasFiles = event => !!event.dataTransfer && Array.from(event.dataTransfer.types || []).includes('Files');
        root.addEventListener('dragover', event => {
            if (!hasFiles(event) || !store.canMutate()) return;
            event.preventDefault();
            event.stopPropagation();
            event.dataTransfer.dropEffect = 'copy';
            root.dataset.dropping = 'true';
        }, { signal });
        root.addEventListener('dragleave', event => { if (!root.contains(event.relatedTarget)) delete root.dataset.dropping; }, { signal });
        root.addEventListener('drop', async event => {
            if (!hasFiles(event) || !store.canMutate()) return;
            event.preventDefault();
            event.stopPropagation();
            delete root.dataset.dropping;
            const files = Array.from(event.dataTransfer.files || []);
            if (!newStack.isOpen()) await openNewStack();
            if (newStack.isOpen() && !disposed) await newStack.acceptFiles(files);
        }, { signal });

        // ---- events ----------------------------------------------------------------------------------------------

        function onAction(el, event) {
            const key = el.dataset.rowKey || '';
            switch (el.dataset.action) {
                case 'pick-host': void pickHost(el.dataset.hostId); break;
                case 'pick-section': setSection(el.dataset.section); break;
                case 'manage-hosts': dialogs.openHosts(); break;
                case 'refresh': void store.refresh({ force: true }); break;
                case 'toggle-group': toggleGroup(el.dataset.groupKey); break;
                case 'act': void performAction(key, el.dataset.act); break;
                case 'remove': void removeRow(key); break;
                case 'logs': openDetail(key, 'logs', true); break;
                // The desktop closes its context menu on every click it sees, so the click that opens one must not reach it.
                case 'more': { event.stopPropagation(); const rect = el.getBoundingClientRect(); showRowMenu(key, rect.left, rect.bottom + 4); break; }
                case 'close-detail': closeDetail(); break;
                case 'tab': store.setDetailTab(el.dataset.tab); break;
                case 'logs-reload': void store.loadLogs(); break;
                case 'copy-logs': if (state.detail) void copyText(views.stripAnsi(state.detail.logs.text)); break;
                case 'copy-inspect': if (state.detail && state.detail.inspect.status === 'ok') void copyText(JSON.stringify(state.detail.inspect.data, null, 2)); break;
                case 'retry-host': void retryHost(el.dataset.hostId); break;
                case 'edit-host': { const target = state.hosts.find(item => item.id === el.dataset.hostId); if (target) dialogs.openHostForm(target); break; }
                case 'clear-filters': ui.query = ''; searchEl.value = ''; ui.filter = 'all'; savePrefsSoon(); schedulePaint(); break;
                case 'open-settings': window.open('/config#docker', '_blank', 'noopener'); break;
                case 'new-stack': void openNewStack(); break;
                case 'palette': openPalette(); break;
                case 'open-stack': void openStack(el.dataset.stackKey, 'overview', false); break;
                case 'open-container': openDetail(key, 'overview', false); break;
                case 'show-project': ui.query = el.dataset.project; searchEl.value = ui.query; setSection('containers'); break;
                case 'project-action': void projectAction(el.dataset.hostId, el.dataset.project, el.dataset.act); break;
                case 'prepare-host': void ops.prepare(el.dataset.hostId); break;
                case 'stack-op': void stackOp(el.dataset.stackKey, el.dataset.op); break;
                case 'stack-more': { event.stopPropagation(); const rect = el.getBoundingClientRect(); showStackMenu(el.dataset.stackKey, rect.left, rect.bottom + 4); break; }
                case 'close-stack': void closeStackPane(); break;
                case 'stack-tab': store.setStackTab(el.dataset.tab); break;
                case 'retry-stack': void store.loadStack(); break;
                case 'save-file': void workbench.save([el.dataset.file], false); break;
                case 'save-deploy': void workbench.save(['compose', 'env'], true); break;
                case 'open-service-container': {
                    const row = model.buildRows(state.hosts, state.byHost, 'all').find(r => r.hostId === (state.stack && state.stack.hostId) && r.c.id === el.dataset.containerId);
                    if (row) openDetail(row.key, 'overview', true);
                    break;
                }
                case 'ops-toggle': ops.setOpen(!ops.isOpen()); schedulePaint(); break;
                case 'op-output': ops.toggleOutput(el.dataset.opId); schedulePaint(); break;
                case 'op-goto-line': {
                    const entry = state.ops.find(item => item.id === el.dataset.opId);
                    if (entry) showErrorLine(entry);
                    break;
                }
                default: break;
            }
        }

        // A failed operation names a line of the compose file: reopen the stack on the Compose tab with that line marked.
        function showErrorLine(entry) {
            const op = entry.op;
            if (!op.stack || !(op.error_line > 0)) return;
            setSection('stacks');
            ui.mark = { key: op.host_id + '/' + op.stack, line: op.error_line, message: op.error };
            ui.focusStack = false;
            store.openStack(op.host_id, op.stack, 'compose');
            schedulePaint();
        }
        const unsubscribeDone = store.onOperationDone(entry => {
            ops.done(entry);
            if (entry.op.state === 'failed' && entry.op.error_line > 0 && entry.op.kind !== 'delete') showErrorLine(entry);
            schedulePaint();
        });

        root.addEventListener('click', event => {
            const el = event.target.closest('[data-action]');
            if (el && root.contains(el)) { onAction(el, event); return; }
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
        stackListEl.addEventListener('focusin', event => {
            const nav = event.target.closest('[data-snav]');
            if (nav && nav.dataset.snav !== ui.stackNav) { ui.stackNav = nav.dataset.snav; applyStackRoving(); }
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

        // The stack list: arrows move between the rows (Enter and Space click the focused button), the menu key opens the row menu.
        stackListEl.addEventListener('keydown', event => {
            const nav = event.target.closest('[data-snav]');
            if (!nav || event.ctrlKey || event.metaKey || event.altKey) return;
            const items = [...stackListEl.querySelectorAll('[data-snav]')], index = items.indexOf(nav);
            const to = { ArrowDown: index + 1, ArrowUp: index - 1, Home: 0, End: items.length - 1 }[event.key];
            if (to !== undefined) { event.preventDefault(); items[Math.max(0, Math.min(items.length - 1, to))]?.focus(); return; }
            if ((event.key === 'ContextMenu' || (event.key === 'F10' && event.shiftKey)) && nav.dataset.stackKey) {
                event.preventDefault();
                const rect = nav.getBoundingClientRect();
                showStackMenu(nav.dataset.stackKey, rect.left + 24, rect.bottom - 4);
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
            if (target && target !== option) { void pickHost(target.dataset.hostId); requestAnimationFrame(() => railEl.querySelector('[data-focus-id="host:' + CSS.escape(target.dataset.hostId) + '"]')?.focus()); }
        }, { signal });

        function tabKeys(container, setTab) {
            container.addEventListener('keydown', event => {
                const tab = event.target.closest('[role="tab"]');
                if (!tab || !container.contains(tab)) return;
                const tabs = [...tab.parentElement.querySelectorAll('[role="tab"]')], index = tabs.indexOf(tab);
                const to = { ArrowRight: (index + 1) % tabs.length, ArrowLeft: (index - 1 + tabs.length) % tabs.length, Home: 0, End: tabs.length - 1 }[event.key];
                if (to === undefined) return;
                event.preventDefault();
                setTab(tabs[to].dataset.tab);
                requestAnimationFrame(() => container.querySelector('[role="tab"][aria-selected="true"]')?.focus({ preventScroll: true }));
            }, { signal });
        }
        tabKeys(drawerEl, tab => store.setDetailTab(tab));
        tabKeys(topEl, tab => store.setStackTab(tab));

        // Ctrl/Cmd+K belongs to the app while the focus is inside it: the desktop's Spotlight listens on the document, so the
        // event is stopped here. Escape closes the innermost thing: the drawer of operations, the new-stack view, a pane.
        root.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'k') {
                event.preventDefault();
                event.stopPropagation();
                openPalette();
                return;
            }
            const inEditor = !!event.target.closest?.('.dm-ed-surface');
            if (event.key === 'Escape' && !dialogs.isOpen()) {
                if (ops.isOpen() && opsEl.contains(event.target)) { event.preventDefault(); event.stopPropagation(); ops.setOpen(false); schedulePaint(); requestAnimationFrame(() => opsEl.querySelector('[data-action="ops-toggle"]')?.focus()); return; }
                if (newStack.isOpen() && newstackEl.contains(event.target) && !inEditor) { event.preventDefault(); event.stopPropagation(); void closeNewStack(); return; }
                if (event.target === searchEl && searchEl.value) { event.preventDefault(); searchEl.value = ''; ui.query = ''; schedulePaint(); return; }
                if (state.detail && (drawerEl.contains(event.target) || scrollEl.contains(event.target) || stackListEl.contains(event.target))) { event.preventDefault(); event.stopPropagation(); closeDetail(); return; }
                if (state.stack && !paneEl.hidden && paneEl.contains(event.target) && !inEditor && !isTextField(event.target)) { event.preventDefault(); event.stopPropagation(); void closeStackPane(); }
                return;
            }
            if (event.key === '/' && !event.ctrlKey && !event.metaKey && !event.altKey && !isTextField(event.target) && !dialogs.isOpen()) { event.preventDefault(); searchEl.focus(); }
        }, { signal });

        if (typeof ctx.wireContextMenuBoundary === 'function') {
            ctx.wireContextMenuBoundary(root, {
                onContextMenu: event => {
                    const stackNode = event.target.closest('[data-stack-key].dm-srow-main');
                    if (stackNode) { showStackMenu(stackNode.dataset.stackKey, event.clientX, event.clientY); return true; }
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
            savePrefs(currentPrefs());
            for (const el of [...toastTimers.keys()]) removeToast(el);
            unsubscribe();
            unsubscribeDone();
            ops.stopTicker();
            palette.dispose();
            newStack.dispose();
            workbench.close();
            dialogs.dispose();
            store.dispose();
            ctx.setWindowMenus?.(id, []);
            root.replaceChildren();
        });
    }

    window.DockerManagerApp = { render, dispose };
})();
