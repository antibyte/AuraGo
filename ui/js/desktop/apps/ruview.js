/* RuView app: the composition root. It builds the command bar, the three panels and the single live region, wires the
   store to the views, and owns the menus, the keyboard handling and the action routing.
   Pure logic lives in ruview-model.js, requests and polling in ruview-store.js, the panels in ruview-views.js. */
(function () {
    'use strict';
    const BASE = '/api/desktop/integrations/ruview/';
    const instances = new Map(), warned = new Set();
    function dispose(id) { instances.get(id)?.(); instances.delete(id); }

    function render(host, id, ctx) {
        dispose(id);
        const model = window.RuViewModel, Views = window.RuViewViews, Store = window.RuViewStore, esc = ctx.esc;
        // A missing key shows its own name, never a misleading label, and is reported once.
        const t = key => {
            const full = 'ruview.' + key, value = ctx.t(full);
            if (value === full && !warned.has(full)) { warned.add(full); console.warn('RuView: missing translation ' + full); }
            return value;
        };
        const lang = document.documentElement.lang || 'en';
        const uid = 'ruview-' + String(id).replace(/[^\w-]/g, '-');
        const kit = Views.kit(t, lang, model);
        const { set, show, attr } = kit;
        const lifetime = new AbortController(), signal = lifetime.signal;
        const store = Store.create({ base: BASE, desktopReadonly: !!ctx.readonly });
        const state = store.state;
        let disposed = false, wizardClose = null, tickTimer = 0, lastId = null, shownView = 'status';

        // ---- skeleton --------------------------------------------------------------------------------------------

        const viewName = view => kit.pick(kit.copy.VIEW, view, () => view);
        const tabs = model.VIEWS.map(view => '<button type="button" role="tab" class="ruview-tab" data-view="' + view + '" id="' + uid + '-tab-' + view + '" aria-controls="' + uid + '-panel-' + view +
            '" aria-selected="false" tabindex="-1" hidden>' + esc(viewName(view)) + '</button>').join('');
        const viewOptions = model.VIEWS.map(view => '<button type="button" role="menuitemradio" class="ruview-menuitem" data-view-option="' + view + '" aria-checked="false" tabindex="-1">' + esc(viewName(view)) + '</button>').join('');
        // Stop and Install are added to the menu only while they apply (see updateBar).
        // A blocked item shows its reason as a second line, which its description points to (aria-describedby).
        const menuReasonID = action => uid + '-menu-reason-' + action.replace('/', '-');
        const menuButton = (action, label) => '<button type="button" role="menuitem" class="ruview-menuitem" data-action="' + action + '" tabindex="-1"><span class="ruview-menuitem-label">' +
            esc(label) + '</span><span class="ruview-menuitem-reason" id="' + menuReasonID(action) + '" aria-hidden="true" hidden></span></button>';
        const menuItems = menuButton('test', t('check_now')) +
            '<a role="menuitem" class="ruview-menuitem" data-action="open-settings" href="/config#ruview" target="_blank" rel="noopener" tabindex="-1">' + esc(t('settings')) + '</a>' +
            menuButton('copy-diagnostics', t('menu_copy_diagnostics')) + menuButton('about', t('menu_about'));
        host.replaceChildren();
        host.innerHTML = '<div class="ruview-app" data-ruview-app>' +
            '<header class="ruview-commandbar" data-ruview-commandbar>' +
                '<div class="ruview-brand"><span class="ruview-brand-glyph" aria-hidden="true">◉</span><span class="ruview-brand-name">RuView</span><h1 class="ruview-sr-only">RuView</h1></div>' +
                '<div class="ruview-tabs" role="tablist" data-ruview-tablist aria-label="' + esc(t('nav_label')) + '">' + tabs + '</div>' +
                '<button type="button" class="ruview-view-menu" data-ruview-view-menu aria-haspopup="menu" aria-expanded="false" aria-controls="' + uid + '-view-list"><span data-view-menu-label></span><span aria-hidden="true">▾</span></button>' +
                '<button type="button" class="ruview-pill" data-ruview-pill data-tone="neutral"><span data-ruview-pill-glyph aria-hidden="true">○</span><span data-ruview-pill-label></span><span data-ruview-pill-age aria-live="off" hidden></span></button>' +
                '<div class="ruview-primary-slot" data-ruview-primary-slot></div>' +
                '<button type="button" class="ruview-more" data-ruview-more aria-haspopup="menu" aria-expanded="false" aria-controls="' + uid + '-menu" aria-label="' + esc(t('more_actions')) + '">⋯</button>' +
            '</header>' +
            '<div class="ruview-menu ruview-menu-view" role="menu" id="' + uid + '-view-list" data-ruview-view-list aria-label="' + esc(t('nav_label')) + '" hidden>' + viewOptions + '</div>' +
            '<div class="ruview-menu ruview-menu-more" role="menu" id="' + uid + '-menu" data-ruview-menu aria-label="' + esc(t('more_actions')) + '" hidden>' + menuItems + '</div>' +
            '<div class="ruview-body" data-ruview-body>' + model.VIEWS.map(view => '<section role="tabpanel" class="ruview-panel" id="' + uid + '-panel-' + view + '" aria-labelledby="' + uid + '-tab-' + view +
                '" data-panel="' + view + '" tabindex="-1"' + (view === 'status' ? '' : ' hidden') + '></section>').join('') + '</div>' +
            '<div class="ruview-sr-only" role="status" aria-live="polite" data-ruview-live></div></div>';
        const root = host.querySelector('[data-ruview-app]');
        const q = selector => root.querySelector(selector);
        const tablist = q('[data-ruview-tablist]'), live = q('[data-ruview-live]');
        const tabEls = [...root.querySelectorAll('[data-view]')], panels = {};
        for (const view of model.VIEWS) panels[view] = q('[data-panel="' + view + '"]');
        const pill = q('[data-ruview-pill]'), slot = q('[data-ruview-primary-slot]');
        const viewMenu = q('[data-ruview-view-menu]'), viewList = q('[data-ruview-view-list]'), more = q('[data-ruview-more]'), moreList = q('[data-ruview-menu]');

        // ---- views -----------------------------------------------------------------------------------------------

        const deps = { t, esc, lang, model, kit, uid, base: BASE, onAction: (action, extra) => onAction(action, extra) };
        const views = {
            status: Views.status.mount(panels.status, deps),
            boards: Views.boards.mount(panels.boards, deps),
            lab: Views.lab.mount(panels.lab, deps)
        };

        // ---- state -> snapshot -----------------------------------------------------------------------------------

        const statusOf = () => state.statusLoaded && state.status && typeof state.status === 'object' ? state.status : null;
        const blockContext = () => ({ status: state.statusLoaded ? state.status : null, pending: state.pending, desktopReadonly: state.desktopReadonly });
        function snapshot() {
            const now = Date.now(), nowPerf = performance.now(), status = statusOf();
            const availableViews = model.availableViews(status);
            const nodes = Object.assign({}, state.nodes, { liveCount: model.liveBoardCount(state.nodes, state.status, nowPerf) });
            const derived = model.deriveState({
                status: state.status, statusLoaded: state.statusLoaded, statusFailed: state.statusFailed, statusAt: state.statusAt, now, nodes,
                views: availableViews, desktopReadonly: state.desktopReadonly, pending: state.pending, lang
            });
            const steps = state.statusLoaded && !state.statusFailed ? model.setupSteps(state.status, nodes, { desktopReadonly: state.desktopReadonly }) : null;
            return { state, derived, steps, views: availableViews, nodes, status, now, nowPerf, view: availableViews.includes(state.view) ? state.view : 'status' };
        }

        // ---- command bar -----------------------------------------------------------------------------------------

        function updateBar(snap) {
            const { derived, view } = snap;
            for (const tab of tabEls) {
                const available = snap.views.includes(tab.dataset.view), selected = tab.dataset.view === view;
                show(tab, available);
                attr(tab, 'aria-selected', String(selected));
                attr(tab, 'tabindex', selected ? '0' : '-1');
            }
            for (const option of root.querySelectorAll('[data-view-option]')) {
                show(option, snap.views.includes(option.dataset.viewOption));
                attr(option, 'aria-checked', String(option.dataset.viewOption === view));
            }
            // The visible text is the view's name; the accessible name adds what the control is.
            set(q('[data-view-menu-label]'), viewName(view));
            attr(viewMenu, 'aria-label', model.fill(t('view_menu_label'), { view: viewName(view) }));
            const label = kit.pillText(derived.pill), age = derived.pillAgeMs != null ? kit.ageText(derived.pillAgeMs) : '';
            attr(pill, 'data-tone', derived.tone);
            set(q('[data-ruview-pill-glyph]'), derived.glyph);
            set(q('[data-ruview-pill-label]'), label);
            set(q('[data-ruview-pill-age]'), age);
            show(q('[data-ruview-pill-age]'), age !== '');
            // The visible label stays part of the accessible name.
            attr(pill, 'aria-label', t('pill_open_status') + ': ' + label);

            // Boards is the one view with a primary action; it is blocked with a visible reason, never silently.
            let primary = q('[data-ruview-primary]');
            if (view === 'boards') {
                if (!primary) {
                    primary = document.createElement('button');
                    primary.type = 'button';
                    primary.className = 'ruview-primary';
                    primary.setAttribute('data-ruview-primary', '');
                    primary.setAttribute('data-action', 'flash');
                    primary.innerHTML = '<span class="ruview-primary-icon" aria-hidden="true">＋</span><span class="ruview-primary-label"></span>';
                    slot.append(primary);
                }
                const block = model.blockReason('flash', blockContext());
                set(primary.querySelector('.ruview-primary-label'), t('action_add_board'));
                attr(primary, 'aria-label', t('action_add_board'));
                attr(primary, 'aria-disabled', block ? 'true' : null);
                attr(primary, 'aria-describedby', block ? uid + '-add-reason' : null);
            } else primary?.remove();

            const s = snap.status, rt = model.runtimeOf(s), external = !s || s.mode === 'external';
            const item = action => moreList.querySelector('[data-action="' + action + '"]');
            const toggleItem = (action, wanted, label) => {
                let el = item(action);
                if (wanted && !el) {
                    moreList.insertBefore(document.createRange().createContextualFragment(menuButton(action, label)), item('open-settings'));
                    el = item(action);
                } else if (!wanted && el) el.remove();
                return el;
            };
            toggleItem('runtime/stop', !external && ['running', 'starting', 'stop_unconfirmed', 'exited'].includes(rt), t('menu_stop_service'));
            toggleItem('runtime/install', !!s && s.mode === 'native', t('menu_install'));
            for (const action of ['test', 'runtime/stop', 'runtime/install']) {
                const el = item(action);
                if (!el) continue;
                const block = model.blockReason(action, blockContext());
                const reason = el.querySelector('.ruview-menuitem-reason');
                attr(el, 'aria-disabled', block ? 'true' : null);
                attr(el, 'aria-describedby', block ? reason.id : null);
                set(reason, block ? kit.reasonText(block) : '');
                show(reason, !!block);
            }
        }

        // Menus: role=menu with arrow keys, Escape back to the trigger, and a click or tab away closes them.
        const menus = [];
        function menu(trigger, list) {
            const entries = () => [...list.querySelectorAll('[role^="menuitem"]')].filter(el => !el.hidden);
            const api = {
                trigger, list,
                open() {
                    for (const other of menus) if (other !== api) other.close(false);
                    if (!list.hidden) return;
                    if (list === viewList) list.style.left = Math.max(6, trigger.getBoundingClientRect().left - root.getBoundingClientRect().left) + 'px';
                    list.hidden = false;
                    trigger.setAttribute('aria-expanded', 'true');
                    (entries().find(el => el.getAttribute('aria-checked') === 'true') || entries()[0])?.focus();
                },
                close(restore) {
                    if (list.hidden) return;
                    list.hidden = true;
                    trigger.setAttribute('aria-expanded', 'false');
                    if (restore) trigger.focus();
                }
            };
            trigger.addEventListener('click', () => (list.hidden ? api.open() : api.close(true)), { signal });
            trigger.addEventListener('keydown', event => { if (event.key === 'ArrowDown' && list.hidden) { event.preventDefault(); api.open(); } }, { signal });
            list.addEventListener('keydown', event => {
                const items = entries(), index = items.indexOf(document.activeElement);
                const move = { ArrowDown: (index + 1) % items.length, ArrowUp: (index - 1 + items.length) % items.length, Home: 0, End: items.length - 1 }[event.key];
                if (move !== undefined) { event.preventDefault(); items[move]?.focus(); }
                else if (event.key === 'Escape') { event.preventDefault(); api.close(true); }
                else if (event.key === 'Tab') api.close(false);
            }, { signal });
            menus.push(api);
            return api;
        }
        const viewMenuApi = menu(viewMenu, viewList), moreApi = menu(more, moreList);
        document.addEventListener('pointerdown', event => { for (const m of menus) if (!m.list.contains(event.target) && !m.trigger.contains(event.target)) m.close(false); }, { signal });

        function setView(view, focusPanel) {
            if (!snapshot().views.includes(view)) return;
            store.setView(view);
            if (focusPanel) panels[view].focus({ preventScroll: true });
        }
        tablist.addEventListener('click', event => { const tab = event.target.closest('[data-view]'); if (tab) setView(tab.dataset.view, false); }, { signal });
        tablist.addEventListener('keydown', event => {
            const visible = tabEls.filter(tab => !tab.hidden), index = visible.indexOf(document.activeElement);
            if (index < 0) return;
            const next = { ArrowRight: (index + 1) % visible.length, ArrowLeft: (index - 1 + visible.length) % visible.length, Home: 0, End: visible.length - 1 }[event.key];
            if (next === undefined) return;
            event.preventDefault();
            visible[next].focus();
            setView(visible[next].dataset.view, false);
        }, { signal });
        viewList.addEventListener('click', event => {
            const option = event.target.closest('[data-view-option]');
            if (!option) return;
            viewMenuApi.close(true);
            setView(option.dataset.viewOption, false);
        }, { signal });
        pill.addEventListener('click', () => setView('status', true), { signal });
        slot.addEventListener('click', event => activate(event.target.closest('[data-action]')), { signal });
        moreList.addEventListener('click', event => {
            const el = event.target.closest('[data-action]');
            if (!el || el.getAttribute('aria-disabled') === 'true') return;
            moreApi.close(true);
            activate(el, true);
        }, { signal });
        function activate(el, fromMenu) {
            if (!el || el.getAttribute('aria-disabled') === 'true') return;
            // The settings entry is a real link: the browser opens it.
            if (!(fromMenu && el.tagName === 'A')) onAction(el.dataset.action);
        }

        // ---- actions ---------------------------------------------------------------------------------------------

        function announce(text) { if (!disposed && text) live.textContent = text; }
        async function runAction(action) {
            const ok = await store.run(action);
            if (disposed) return;
            if (ok && action === 'test') announce(t('announce_checked'));
            else if (!ok && state.actionError) announce(kit.errorText(state.actionError.code));
        }
        async function openWizard(nodeID) {
            const { openWizard: open } = await import(window.AuraLazyAssets.versionedURL('/js/ruview/wizard.mjs'));
            if (disposed) return;
            wizardClose = await open(nodeID === undefined ? { base: BASE, t: ctx.t } : { base: BASE, t: ctx.t, nodeID });
            if (disposed) wizardClose?.();
        }
        // The desktop modal resolves the chosen value ('confirm'), and false on Cancel, Escape or a click outside: only 'confirm' is a yes.
        // Without the modal the plain confirm dialog answers yes or no.
        async function confirmChange(title, message, label) {
            if (typeof ctx.modalDialog === 'function') return (await ctx.modalDialog({ title, message, choices: [{ value: 'confirm', label }] })) === 'confirm';
            return (await ctx.confirmDialog?.(title, message)) === true;
        }
        async function allowChanges() {
            if (state.desktopReadonly) return;
            const ok = await confirmChange(t('allow_changes_title'), t('allow_changes_body'), t('allow_changes_confirm'));
            if (!ok || disposed) return;
            const saved = await store.allowChanges();
            if (!disposed) announce(saved ? t('allow_changes_done') : t('allow_changes_failed') + (state.actionError ? ' ' + kit.errorText(state.actionError.code) : ''));
        }
        async function copyDiagnostics() {
            const snap = snapshot();
            const text = model.diagnostics(state.status, snap.nodes, Date.now(), { statusLoaded: state.statusLoaded, statusFailed: state.statusFailed, statusAt: state.statusAt });
            try {
                if (!navigator.clipboard?.writeText) throw new Error('no clipboard');
                await navigator.clipboard.writeText(text);
                announce(t('diagnostics_copied'));
            } catch (error) {
                setView('status', false);
                views.status.showDiagnostics(text);
                announce(t('diagnostics_failed'));
            }
        }
        function onAction(action, extra) {
            if (disposed || model.blockReason(action, blockContext())) return;
            switch (action) {
                case 'test': case 'runtime/start': case 'runtime/stop': case 'runtime/install': void runAction(action); break;
                case 'flash': void openWizard(extra && Number.isInteger(extra.nodeID) ? extra.nodeID : undefined); break;
                case 'allow-changes': void allowChanges(); break;
                case 'open-settings': window.open('/config#ruview', '_blank', 'noopener'); break;
                case 'show-boards': setView('boards', true); break;
                case 'refresh': void store.refresh(); break;
                case 'copy-diagnostics': void copyDiagnostics(); break;
                case 'about': setView('status', false); views.status.openAbout(); break;
                default: break; // open-live arrives with the Live view
            }
        }

        // ---- render loop -----------------------------------------------------------------------------------------

        function paint(tickOnly) {
            if (disposed) return;
            const snap = snapshot();
            // A tab that went away (Boards after a source change, Lab after turning RuView off) falls back to Status.
            if (state.view !== snap.view) { store.setView(snap.view); return; }
            updateBar(snap);
            if (shownView === 'lab' && snap.view !== 'lab') views.lab.hide();
            for (const view of model.VIEWS) show(panels[view], view === snap.view);
            shownView = snap.view;
            if (snap.view === 'boards' && tickOnly) views.boards.tick(); else views[snap.view].update(snap);
            // The headline is announced when the state changes, never on a routine poll; the first answer is silent.
            if (snap.derived.id !== 'loading') {
                if (lastId !== null && lastId !== snap.derived.id) announce(kit.headlineText(snap.derived));
                lastId = snap.derived.id;
            }
            const needsAge = state.visible && (snap.view === 'boards' || snap.derived.pillAgeMs != null
                || ['live', 'stale', 'external_check_old', 'starting', 'installing'].includes(snap.derived.id));
            if (needsAge && !tickTimer) tickTimer = setInterval(tick, 1000);
            else if (!needsAge && tickTimer) { clearInterval(tickTimer); tickTimer = 0; }
        }
        function tick() {
            if (disposed || document.hidden || (typeof root.checkVisibility === 'function' && !root.checkVisibility())) return;
            paint(true);
        }
        store.subscribe(() => paint(false));
        paint(false);
        store.attach({ root, win: host.closest('.vd-window') });

        const closed = () => {
            if (disposed) return;
            disposed = true;
            lifetime.abort();
            clearInterval(tickTimer);
            store.dispose();
            for (const view of Object.values(views)) view.dispose();
            wizardClose?.();
        };
        instances.set(id, closed);
    }
    window.RuViewApp = { render, dispose };
})();
