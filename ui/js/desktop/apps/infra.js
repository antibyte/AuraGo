// Infra composition root. render(host, windowId, ctx) builds the shell (command
// bar, menus, toasts, announcer, dialog host), creates the store and the canvas,
// table, library, inspector, dialogs, import sheet and gestures modules around one shared
// `app` context, and routes store events to them. Everything below the shell
// lives in infra-*.js; this file owns lifecycle, window menus and chrome only.
(function () {
    'use strict';

    const model = window.InfraAppModel, lanes = window.InfraAppLanes;
    const INSTANCES = new Map();
    const KINDS = ['device', 'vm', 'container', 'service', 'network', 'site', 'room', 'rack', 'group'];
    const SOURCE_KINDS = ['registry', 'docker', 'proxmox', 'fritzbox'];
    const POWER_VALUES = ['running', 'stopped', 'paused', 'unknown', 'none'];
    const PREFS_KEY = 'infra.layout.v1';
    const INSPECTOR_MIN = 320, INSPECTOR_MAX = 520, INSPECTOR_STEP = 16;
    const SEARCH_MIN = 220, CHIP_GAP = 4, SEARCH_COLLAPSE = 560;
    const WRITE_ACTIONS = ['add', 'add-menu', 'add-place', 'import', 'add-empty', 'import-empty'];
    const MENU_TRIGGERS = { add: '[data-action="add-menu"]', more: '[data-action="more"]', legend: '[data-action="more"]', filters: '[data-action="filters"]' };
    const FILTER_SELECTS = { kind: '[data-kind-filter]', source: '[data-source-filter]', power: '[data-power-filter]' };
    const clampInspector = value => Math.min(INSPECTOR_MAX, Math.max(INSPECTOR_MIN, Math.round(Number.isFinite(Number(value)) ? Number(value) : 360)));

    function render(host, windowId, context = {}) {
        dispose(windowId);
        if (!host) return;
        const ctx = context;
        const esc = ctx.esc || model.esc;
        const t = key => ctx.t?.(key) || key;
        const store = window.InfraAppStore.create({ ctx, t });
        const state = store.state;
        const S = String(windowId).replace(/[^a-zA-Z0-9_-]/g, '');
        const glyph = name => model.glyph(name);
        const option = (value, label) => `<option value="${esc(value)}">${esc(label)}</option>`;
        host.innerHTML = `<section class="infra-app" data-infra-app="${esc(S)}" aria-label="${esc(t('desktop.app_infra'))}" data-inspector-open="false" data-library-collapsed="false" data-library-sheet="closed" data-search-open="false" data-readonly="false">
  <header class="infra-commandbar" aria-label="${esc(t('desktop.infra_actions'))}">
    <div class="infra-brand"><img class="infra-brand-mark" src="${esc(model.assetURL('icon'))}" alt="" draggable="false"><span class="infra-brand-name">${esc(t('desktop.app_infra'))}</span></div>
    <button type="button" class="infra-cmd-icon" data-action="toggle-library" aria-controls="infra-library-${esc(S)}" aria-expanded="true" aria-label="${esc(t('desktop.infra_library'))}" title="${esc(t('desktop.infra_library'))}">${glyph('library')}</button>
    <div class="infra-view-switch" role="group" aria-label="${esc(t('desktop.infra_view'))}">
      <button type="button" data-view="atlas" aria-pressed="true">${esc(t('desktop.infra_view_map'))}</button>
      <button type="button" data-view="topology" aria-pressed="false">${esc(t('desktop.infra_topology'))}</button>
      <button type="button" data-view="table" aria-pressed="false">${esc(t('desktop.infra_view_list'))}</button>
    </div>
    <div class="infra-search">
      <button type="button" class="infra-cmd-icon infra-search-open" data-action="open-search" aria-expanded="false" aria-label="${esc(t('desktop.infra_search'))}" title="${esc(t('desktop.infra_search'))}">${glyph('search')}<span class="infra-filter-dot" data-search-dot hidden></span></button>
      <span class="infra-search-icon" aria-hidden="true">${glyph('search')}</span>
      <input type="search" data-search aria-label="${esc(t('desktop.infra_search'))}" placeholder="${esc(t('desktop.infra_search_jump'))}" autocomplete="off" aria-keyshortcuts="Control+K">
      <kbd class="infra-search-kbd" aria-hidden="true">Ctrl K</kbd>
      <button type="button" class="infra-cmd-icon" data-action="filters" aria-haspopup="true" aria-expanded="false" aria-controls="infra-filters-${esc(S)}" aria-label="${esc(t('desktop.infra_filters'))}" title="${esc(t('desktop.infra_filters'))}">${glyph('filter')}<span class="infra-filter-dot" data-filter-dot hidden></span></button>
    </div>
    <div class="infra-filter-chips" data-filter-chips role="group" aria-label="${esc(t('desktop.infra_active_filters'))}" hidden></div>
    <div class="infra-live-status" data-live data-sync="syncing" data-refreshed-at="0" title=""><i aria-hidden="true"></i><span class="infra-sync-label">${esc(t('desktop.infra_sync_syncing'))}</span></div>
    <div class="infra-history" role="group">
      <button type="button" class="infra-cmd-icon" data-action="undo" aria-disabled="true" aria-keyshortcuts="Control+Z" aria-label="${esc(t('desktop.infra_undo'))}" title="${esc(t('desktop.infra_undo_none'))}">${glyph('undo')}</button>
      <button type="button" class="infra-cmd-icon" data-action="redo" aria-disabled="true" aria-keyshortcuts="Control+Shift+Z" aria-label="${esc(t('desktop.infra_redo'))}" title="${esc(t('desktop.infra_redo_none'))}">${glyph('redo')}</button>
    </div>
    <div class="infra-split" role="group">
      <button type="button" class="infra-button infra-button-primary" data-action="add" aria-label="${esc(t('desktop.infra_new_asset'))}" title="${esc(t('desktop.infra_new_asset'))}" disabled>${glyph('plus')}<span class="infra-cmd-label">${esc(t('desktop.infra_new_asset'))}</span></button>
      <button type="button" class="infra-button infra-button-primary infra-split-more" data-action="add-menu" aria-haspopup="menu" aria-expanded="false" aria-controls="infra-add-menu-${esc(S)}" aria-label="${esc(t('desktop.infra_add_menu'))}" title="${esc(t('desktop.infra_add_menu'))}" disabled>${glyph('chevron')}</button>
    </div>
    <button type="button" class="infra-cmd-icon" data-action="more" aria-haspopup="menu" aria-expanded="false" aria-controls="infra-more-menu-${esc(S)}" aria-label="${esc(t('desktop.infra_more_actions'))}" title="${esc(t('desktop.infra_more_actions'))}">${glyph('more')}</button>
  </header>
  <div class="infra-body" data-body>
    <div class="infra-scrim" data-scrim></div>
    <div class="infra-splitter" data-splitter="inspector" role="separator" aria-orientation="vertical" aria-controls="infra-inspector-${esc(S)}" aria-valuemin="${INSPECTOR_MIN}" aria-valuemax="${INSPECTOR_MAX}" aria-valuenow="360" aria-label="${esc(t('desktop.infra_resize_inspector'))}" tabindex="0"></div>
  </div>
  <div class="infra-layer" data-layer>
    <div class="infra-menu" id="infra-add-menu-${esc(S)}" data-menu="add" role="menu" hidden>
      <button type="button" role="menuitem" data-action="add-place" disabled>${esc(t('desktop.infra_new_place'))}</button>
      <button type="button" role="menuitem" data-action="import" disabled>${esc(t('desktop.infra_import_discoveries'))}</button>
    </div>
    <div class="infra-menu" id="infra-more-menu-${esc(S)}" data-menu="more" role="menu" hidden>
      <button type="button" role="menuitem" data-action="refresh">${esc(t('desktop.infra_refresh'))}</button>
      <button type="button" role="menuitem" data-action="arrange" disabled>${esc(t('desktop.infra_arrange'))}</button>
      <button type="button" role="menuitem" data-action="shortcuts" aria-keyshortcuts="Shift+?">${esc(t('desktop.infra_shortcuts'))}</button>
      <button type="button" role="menuitem" data-action="legend" aria-haspopup="dialog">${esc(t('desktop.infra_legend'))}</button>
      <button type="button" role="menuitem" data-action="history-undo" data-overflow-only>${esc(t('desktop.infra_undo'))}</button>
      <button type="button" role="menuitem" data-action="history-redo" data-overflow-only>${esc(t('desktop.infra_redo'))}</button>
    </div>
    <div class="infra-popover infra-filters" id="infra-filters-${esc(S)}" data-popover="filters" hidden>
      <label class="infra-select-label"><span>${esc(t('desktop.infra_type'))}</span><select data-kind-filter aria-label="${esc(t('desktop.infra_type_filter'))}"><option value="">${esc(t('desktop.infra_all_types'))}</option>${KINDS.map(kind => option(kind, t(`desktop.infra_kind_${kind}`))).join('')}</select></label>
      <label class="infra-select-label"><span>${esc(t('desktop.infra_source'))}</span><select data-source-filter aria-label="${esc(t('desktop.infra_source_filter'))}"><option value="">${esc(t('desktop.infra_all_sources'))}</option>${SOURCE_KINDS.map(kind => option(kind, t(`desktop.infra_source_${kind}`))).join('')}</select></label>
      <label class="infra-select-label"><span>${esc(t('desktop.infra_power_filter'))}</span><select data-power-filter aria-label="${esc(t('desktop.infra_power_filter'))}"><option value="">${esc(t('desktop.infra_power_all'))}</option>${POWER_VALUES.map(value => option(value, t(`desktop.infra_power_${value}`))).join('')}</select></label>
      <button type="button" class="infra-button" data-action="clear-filters" disabled>${esc(t('desktop.infra_clear_filters'))}</button>
    </div>
    <div class="infra-popover infra-legend" id="infra-legend-${esc(S)}" data-popover="legend" role="dialog" aria-label="${esc(t('desktop.infra_legend'))}" tabindex="-1" hidden></div>
  </div>
  <div class="infra-toasts" data-toasts></div>
  <div class="infra-sr-only" data-announce aria-live="polite" aria-atomic="true"></div>
  <dialog class="infra-dialog" data-dialog aria-labelledby="infra-dialog-title-${esc(S)}"><form method="dialog"><button class="infra-dialog-close" aria-label="${esc(t('desktop.close'))}">×</button></form><div data-dialog-content></div></dialog>
</section>`;

        const root = host.querySelector('.infra-app');
        const q = selector => root.querySelector(selector);
        const body = q('[data-body]'), scrim = q('[data-scrim]'), inspectorSplitter = q('[data-splitter="inspector"]');
        const toggleLibraryButton = q('[data-action="toggle-library"]');
        const searchInput = q('[data-search]'), liveEl = q('[data-live]'), filterDot = q('[data-filter-dot]');
        const searchGroup = q('.infra-search'), searchOpenButton = q('[data-action="open-search"]'), searchDot = q('[data-search-dot]');
        const chipsEl = q('[data-filter-chips]'), clearFiltersButton = q('[data-action="clear-filters"]');
        let chipsKey = '', chipsFrame = 0, disposed = false, deferred = false, searchFrame = 0, searchValue = '', announceTimer = 0, prefsTimer = 0, openMenuName = '', resizeFrame = 0, unsubscribe = null, resizeObserver = null;
        const toastTimers = new Map();

        // ---- preferences ---------------------------------------------------------------
        const readStorage = key => { try { return window.localStorage.getItem(key); } catch (_) { return null; } };
        const writeStorage = (key, value) => { try { window.localStorage.setItem(key, value); } catch (_) { /* storage may be unavailable */ } };
        const prefs = model.readLayoutPrefs(readStorage(PREFS_KEY));
        const flushPrefs = () => {
            clearTimeout(prefsTimer); prefsTimer = 0;
            writeStorage(PREFS_KEY, JSON.stringify(prefs));
        };
        const setPref = (key, value) => {
            prefs[key] = value;
            clearTimeout(prefsTimer);
            prefsTimer = setTimeout(flushPrefs, 300);
        };

        // ---- the shared app context -----------------------------------------------------
        const mapView = () => (state.view === 'table' ? 'atlas' : state.view);
        const ensureMapView = () => { if (state.view === 'table') store.setView('atlas'); };
        const animationsEnabled = () => document.body?.dataset.animations !== 'false' && !(typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches);
        function announce(text) {
            const live = q('[data-announce]');
            if (!live || !text) return;
            live.textContent = '';
            clearTimeout(announceTimer);
            announceTimer = setTimeout(() => { live.textContent = String(text); }, 30);
        }
        function removeToast(element) {
            clearTimeout(toastTimers.get(element));
            toastTimers.delete(element);
            element.remove();
        }
        function toast(message, kind = 'info') {
            const holder = q('[data-toasts]');
            if (!holder || !message) return;
            const text = String(message);
            for (const element of [...holder.children]) if (element.firstElementChild?.textContent === text) removeToast(element);
            const wrapper = document.createElement('div');
            wrapper.innerHTML = `<div class="infra-toast" data-toast data-kind="${esc(kind)}"><span>${esc(text)}</span><button type="button" class="infra-toast-close" data-action="dismiss-toast" aria-label="${esc(t('desktop.close'))}">${glyph('close')}</button></div>`;
            const element = wrapper.firstElementChild;
            holder.appendChild(element);
            while (holder.children.length > 3) removeToast(holder.firstElementChild);
            if (kind !== 'error') toastTimers.set(element, setTimeout(() => removeToast(element), 4000));
            announce(text);
        }
        const app = {
            ctx, t, esc, windowId, safeId: S, root, q, model, lanes, store, state, prefs, setPref,
            icon: value => `<img src="${esc(model.iconURL(value))}" alt="" loading="lazy" draggable="false">`,
            glyph,
            labelForKind: kind => t(`desktop.infra_kind_${kind}`),
            laneLabel: lane => ({ internet: t('desktop.infra_lane_internet'), network: t('desktop.infra_lane_network'), compute: t('desktop.infra_lane_compute'), clients: t('desktop.infra_lane_clients') })[lane] || lane,
            statusLabel: status => t(`desktop.infra_state_${status}`),
            mapView, ensureMapView, toast, announce, animationsEnabled, focusSearch: () => focusSearch(),
            setLibraryCollapsed: on => setLibraryCollapsed(on),
            readStorage, writeStorage,
            openAppLink: link => ctx.openApp?.(link.app, model.appLinkContext(link)),
        };
        app.picker = window.InfraAppPicker.create(app);
        app.canvas = window.InfraAppCanvas.create(app);
        app.table = window.InfraAppTable.create(app);
        app.cleanup = window.InfraAppCleanup.create(app);
        app.library = window.InfraAppLibrary.create(app);
        app.forms = window.InfraAppInspectorForms.create(app);
        app.inspector = window.InfraAppInspector.create(app);
        app.dialogs = window.InfraAppDialogs.create(app);
        app.importer = window.InfraAppImport.create(app);
        app.menus = window.InfraAppMenus.create(app);
        app.gestures = window.InfraAppGestures.create(app);
        const { canvas, table, library, inspector, dialogs, importer, menus, gestures } = app;
        q('[data-popover="legend"]').innerHTML = menus.legendHTML();
        inspector.el.id = `infra-inspector-${S}`;
        inspector.el.tabIndex = -1;
        scrim.before(library.el, library.splitter, canvas.el, table.el);
        inspectorSplitter.after(inspector.el);
        body.appendChild(importer.el);

        // ---- library and inspector sizing --------------------------------------------------
        const isNarrow = () => root.clientWidth < 760;
        function syncLibraryToggle() {
            const open = isNarrow() ? root.dataset.librarySheet === 'open' : root.dataset.libraryCollapsed !== 'true';
            toggleLibraryButton.setAttribute('aria-expanded', String(open));
        }
        function applyLibraryCollapsed(on) {
            root.dataset.libraryCollapsed = String(!!on);
            library.renderNow();
            syncLibraryToggle();
        }
        function setLibraryCollapsed(on) {
            applyLibraryCollapsed(on);
            setPref('libraryCollapsed', !!on);
        }
        function setLibrarySheet(open) {
            root.dataset.librarySheet = open ? 'open' : 'closed';
            syncLibraryToggle();
        }
        let inspectorWidth = clampInspector(prefs.inspectorWidth), splitDrag = null;
        function setInspectorWidth(next, persist = false) {
            inspectorWidth = clampInspector(next);
            root.style.setProperty('--infra-inspector-w', `${inspectorWidth}px`);
            inspectorSplitter.setAttribute('aria-valuenow', String(inspectorWidth));
            if (persist) setPref('inspectorWidth', inspectorWidth);
        }
        const onSplitDown = event => {
            if (event.button !== 0) return;
            event.preventDefault();
            splitDrag = { x: event.clientX, width: inspectorWidth };
            try { inspectorSplitter.setPointerCapture?.(event.pointerId); } catch (_) { /* the pointer is already gone */ }
            root.classList.add('is-resizing');
        };
        const onSplitMove = event => { if (splitDrag) setInspectorWidth(splitDrag.width - (event.clientX - splitDrag.x)); };
        const onSplitEnd = () => {
            if (!splitDrag) return;
            splitDrag = null;
            root.classList.remove('is-resizing');
            setPref('inspectorWidth', inspectorWidth);
        };
        const onSplitKey = event => {
            const next = { ArrowLeft: inspectorWidth + INSPECTOR_STEP, ArrowRight: inspectorWidth - INSPECTOR_STEP, Home: INSPECTOR_MIN, End: INSPECTOR_MAX }[event.key];
            if (next === undefined) return;
            event.preventDefault();
            setInspectorWidth(next, true);
        };
        inspectorSplitter.addEventListener('pointerdown', onSplitDown);
        inspectorSplitter.addEventListener('pointermove', onSplitMove);
        for (const type of ['pointerup', 'pointercancel', 'lostpointercapture']) inspectorSplitter.addEventListener(type, onSplitEnd);
        inspectorSplitter.addEventListener('keydown', onSplitKey);
        setInspectorWidth(inspectorWidth);
        library.setWidth(prefs.libraryWidth);
        applyLibraryCollapsed(prefs.libraryCollapsed);

        // ---- shell painting ----------------------------------------------------------------
        // Arrange needs a card to arrange. In Topology it hands cards back to the automatic layout (A22): it needs a
        // server that can unplace and a card with a saved placement, and then it reads "Arrange automatically".
        const hasPlacedCards = () => { const view = mapView(); return view === 'topology' ? gestures.canArrangeTopology() : state.nodes.some(node => store.effectivePlacement(node.id, view)); };
        function paintArrange() {
            const write = store.canWrite(), topology = mapView() === 'topology', label = topology ? t('desktop.infra_arrange_auto') : t('desktop.infra_arrange');
            const blocked = !write || state.busy.has('layout') || !hasPlacedCards();
            for (const button of root.querySelectorAll('[data-action="arrange"]')) {
                button.disabled = blocked;
                if (button.textContent !== label) button.textContent = label;
                if (topology && write && !store.features.unplace) button.title = t('desktop.infra_arrange_unsupported'); else button.removeAttribute('title');
            }
        }
        function paintShell() {
            if (disposed || !root.isConnected) return;
            const write = store.canWrite(), view = state.view, history = mapView();
            root.dataset.readonly = String(!write);
            for (const button of root.querySelectorAll('.infra-view-switch [data-view]')) button.setAttribute('aria-pressed', String(button.dataset.view === view));
            canvas.el.hidden = view === 'table';
            table.el.hidden = view !== 'table';
            for (const action of WRITE_ACTIONS) for (const button of root.querySelectorAll(`[data-action="${action}"]`)) button.disabled = !write;
            paintArrange();
            for (const button of root.querySelectorAll('[data-action="refresh"]')) button.disabled = state.busy.has('snapshot');
            const canUndo = store.canUndo(history), canRedo = store.canRedo(history);
            const undo = q('.infra-history [data-action="undo"]'), redo = q('.infra-history [data-action="redo"]');
            undo.setAttribute('aria-disabled', String(!canUndo));
            undo.title = canUndo ? t('desktop.infra_undo') : t('desktop.infra_undo_none');
            redo.setAttribute('aria-disabled', String(!canRedo));
            redo.title = canRedo ? t('desktop.infra_redo') : t('desktop.infra_redo_none');
            q('[data-action="history-undo"]').disabled = !canUndo;
            q('[data-action="history-redo"]').disabled = !canRedo;
            const sync = state.busy.has('snapshot') ? 'syncing' : state.sync;
            liveEl.dataset.sync = sync;
            liveEl.classList.toggle('is-error', state.sync === 'error' || state.sync === 'paused');
            liveEl.querySelector('.infra-sync-label').textContent = sync === 'live' ? t('desktop.infra_sync_live') : sync === 'error' ? t('desktop.infra_sync_error') : sync === 'paused' ? t('desktop.infra_sync_paused') : t('desktop.infra_sync_syncing');
            liveEl.dataset.refreshedAt = String(state.refreshAt);
            // Ignored sources are not part of the freshness ledger.
            const counted = state.sources.filter(source => !source.ignored);
            const current = counted.filter(source => source.state === 'current').length;
            const updated = state.refreshAt ? t('desktop.infra_updated').replace('{time}', new Date(state.refreshAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })) : '';
            const sources = counted.length ? model.fill(counted.length === 1 ? t('desktop.infra_sources_count_one') : t('desktop.infra_sources_count'), { current, total: counted.length }) : t('desktop.infra_sources_waiting');
            liveEl.title = updated ? `${updated} · ${sources}` : sources;
            filterDot.hidden = !(state.filters.kind || state.filters.source || state.filters.power);
            searchDot.hidden = !(String(state.filters.query || '').trim() || state.filters.kind || state.filters.source || state.filters.power);
            paintFilterChips();
            scheduleChipFit();
        }
        // One removable chip per active filter (wide windows only; the dot on the filter button says the same below that).
        // Only as many chips as the bar can spare are shown: the search keeps SEARCH_MIN px, the rest sit behind a "+N" chip.
        function paintFilterChips() {
            const { kind, source, power } = state.filters, key = [kind, source, power].join('\x1f');
            if (key === chipsKey) return;
            chipsKey = key;
            const chips = [];
            if (kind) chips.push(['kind', `${t('desktop.infra_type')}: ${app.labelForKind(kind)}`]);
            if (source) chips.push(['source', `${t('desktop.infra_source')}: ${t(`desktop.infra_source_${source}`)}`]);
            if (power) chips.push(['power', `${t('desktop.infra_power_filter')}: ${power === 'none' ? t('desktop.infra_power_none') : t(model.powerLabel(power))}`]);
            chipsEl.innerHTML = chips.map(([name, label]) => {
                const remove = esc(model.fill(t('desktop.infra_remove_filter'), { filter: label }));
                return `<button type="button" class="infra-filter-chip" data-remove-filter="${name}" aria-label="${remove}" title="${remove}"><span>${esc(label)}</span>${glyph('close')}</button>`;
            }).join('') + (chips.length ? `<button type="button" class="infra-filter-chip infra-filter-more" data-filter-overflow data-action="filters" aria-haspopup="true" hidden></button>` : '');
            chipsEl.hidden = !chips.length;
            clearFiltersButton.disabled = !chips.length;
            fitFilterChips();
        }
        // fitSearchBadge (A27) hides the Ctrl K badge whenever the field is too narrow for the placeholder next to it:
        // the badge is a hint, the placeholder is the label. Below 1100 px a container rule hides it anyway.
        let placeholderProbe = null;
        function fitSearchBadge() {
            if (disposed || root.clientWidth < SEARCH_COLLAPSE) return;
            const kbd = searchGroup.querySelector('.infra-search-kbd');
            searchGroup.classList.remove('is-kbd-hidden');
            if (!kbd || kbd.offsetWidth === 0) return;
            const style = getComputedStyle(searchInput);
            placeholderProbe = placeholderProbe || document.createElement('canvas').getContext('2d');
            placeholderProbe.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
            const needed = Math.ceil(placeholderProbe.measureText(searchInput.placeholder).width) + 6;
            searchGroup.classList.toggle('is-kbd-hidden', searchInput.clientWidth < needed);
        }
        function fitFilterChips() { fitChips(); fitSearchBadge(); }
        function fitChips() {
            const chips = [...chipsEl.querySelectorAll('[data-remove-filter]')], more = chipsEl.querySelector('[data-filter-overflow]');
            if (!more) return;
            for (const chip of chips) chip.hidden = false;
            more.hidden = true;
            if (chipsEl.hidden || chipsEl.offsetParent === null) return;
            // What the bar has left for the chips: its width minus every other item (the search counts as SEARCH_MIN) and the gaps.
            const bar = chipsEl.parentElement, style = getComputedStyle(bar);
            let used = (parseFloat(style.paddingLeft) || 0) + (parseFloat(style.paddingRight) || 0), gaps = 0;
            for (const child of bar.children) {
                if (child === chipsEl || child.offsetParent === null) continue;
                used += child.classList.contains('infra-search') ? SEARCH_MIN : child.offsetWidth;
                gaps += 1;
            }
            const budget = bar.clientWidth - used - (parseFloat(style.columnGap) || 0) * gaps;
            const widths = chips.map(chip => chip.offsetWidth);
            if (widths.reduce((sum, width) => sum + width, 0) + CHIP_GAP * (chips.length - 1) <= budget) return;
            more.hidden = false;
            more.textContent = `+${chips.length}`;
            let taken = more.offsetWidth, shown = 0;
            for (const width of widths) { if (taken + CHIP_GAP + width > budget) break; taken += CHIP_GAP + width; shown += 1; }
            chips.forEach((chip, index) => { chip.hidden = index >= shown; });
            const rest = chips.length - shown, label = model.countText(rest, t('desktop.infra_filters_more_one'), t('desktop.infra_filters_more'));
            more.textContent = `+${rest}`;
            more.setAttribute('aria-label', label);
            more.title = label;
        }
        function scheduleChipFit() {
            if (chipsFrame || chipsEl.hidden) return;
            chipsFrame = requestAnimationFrame(() => { chipsFrame = 0; if (!disposed) fitFilterChips(); });
        }
        const renderTable = () => { if (state.view === 'table') table.render(); };
        function renderData() { canvas.render(); library.render(); renderTable(); }
        const syncMenus = () => ctx.setWindowMenus?.(windowId, [{ id: 'infra', labelKey: 'desktop.app_infra', items: [
            { labelKey: 'desktop.infra_refresh', icon: 'refresh', action: () => void store.loadSnapshot(true) },
            { type: 'separator' },
            { labelKey: 'desktop.infra_undo', disabled: !store.canUndo(mapView()), action: () => void store.undo(mapView()) },
            { labelKey: 'desktop.infra_redo', disabled: !store.canRedo(mapView()), action: () => void store.redo(mapView()) },
            { type: 'separator' },
            { labelKey: 'desktop.infra_import_title', icon: 'download', disabled: !store.canWrite(), action: () => importer.open() },
            { labelKey: 'desktop.infra_new_asset', icon: 'plus', disabled: !store.canWrite(), action: () => dialogs.openNodeDialog('device') },
            { labelKey: 'desktop.infra_new_place', disabled: !store.canWrite(), action: () => dialogs.openNodeDialog('room', { title: t('desktop.infra_new_place') }) },
            { labelKey: mapView() === 'topology' ? 'desktop.infra_arrange_auto' : 'desktop.infra_arrange', icon: 'grid', disabled: !store.canWrite() || (mapView() === 'topology' && !store.features.unplace), action: () => gestures.arrange() },
        ] }]);

        // ---- menus and popovers --------------------------------------------------------------
        const menuEl = name => q(`[data-menu="${name}"], [data-popover="${name}"]`);
        function onMenuOutside(event) {
            const el = menuEl(openMenuName), trigger = q(MENU_TRIGGERS[openMenuName]);
            if (el?.contains(event.target) || trigger?.contains(event.target) || event.target.closest?.('[data-filter-overflow]')) return;
            closeMenus();
        }
        function closeMenus({ restoreFocus = false } = {}) {
            const name = openMenuName;
            if (!name) return;
            openMenuName = '';
            menuEl(name).hidden = true;
            const trigger = q(MENU_TRIGGERS[name]);
            trigger?.setAttribute('aria-expanded', 'false');
            document.removeEventListener('pointerdown', onMenuOutside, true);
            if (restoreFocus) trigger?.focus();
        }
        function openMenu(name) {
            if (openMenuName === name) { closeMenus({ restoreFocus: true }); return; }
            closeMenus();
            const el = menuEl(name), trigger = q(MENU_TRIGGERS[name]);
            if (!el || !trigger || trigger.disabled) return;
            const appRect = root.getBoundingClientRect(), triggerRect = trigger.getBoundingClientRect();
            el.style.top = `${Math.round(triggerRect.bottom - appRect.top + 6)}px`;
            if (name === 'filters') { el.style.left = `${Math.round(triggerRect.left - appRect.left)}px`; el.style.right = ''; }
            else { el.style.right = `${Math.round(appRect.right - triggerRect.right)}px`; el.style.left = ''; }
            el.hidden = false;
            trigger.setAttribute('aria-expanded', 'true');
            openMenuName = name;
            document.addEventListener('pointerdown', onMenuOutside, true);
            // A popover without items (the legend) takes the focus itself.
            (el.querySelector('[role="menuitem"]:not(:disabled), select, button:not(:disabled)') || (el.hasAttribute('tabindex') ? el : null))?.focus();
        }
        const onLayerKeydown = event => {
            if (!openMenuName) return;
            if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closeMenus({ restoreFocus: true }); return; }
            if (event.key === 'Tab') { closeMenus(); return; }
            if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
            const items = [...menuEl(openMenuName).querySelectorAll('[role="menuitem"]:not(:disabled)')].filter(item => item.offsetParent !== null);
            if (!items.length) return;
            event.preventDefault();
            const index = items.indexOf(document.activeElement);
            const next = event.key === 'ArrowDown' ? (index + 1) % items.length : (index - 1 + items.length) % items.length;
            items[next].focus();
        };
        const onFilterChange = event => {
            const select = event.target.closest('select');
            if (!select) return;
            if (select.hasAttribute('data-kind-filter')) store.setFilters({ kind: select.value });
            else if (select.hasAttribute('data-source-filter')) store.setFilters({ source: select.value });
            else if (select.hasAttribute('data-power-filter')) store.setFilters({ power: select.value });
        };
        q('[data-layer]').addEventListener('keydown', onLayerKeydown);
        q('[data-layer]').addEventListener('change', onFilterChange);

        // ---- clicks -------------------------------------------------------------------------------
        const OWN_CLICKS = '[data-inspector], [data-dialog], [data-connect-popover], [data-stage], [data-library], [data-table-view], [data-import-sheet]';
        function clearSearch() {
            cancelAnimationFrame(searchFrame); searchFrame = 0;
            searchInput.value = '';
            store.setFilters({ query: '' });
        }
        function toggleLibrary() {
            if (!isNarrow()) { setLibraryCollapsed(root.dataset.libraryCollapsed !== 'true'); return; }
            const open = root.dataset.librarySheet !== 'open';
            if (open) inspector.close({ restoreFocus: false });
            setLibrarySheet(open);
        }
        // Resetting a filter also resets its select, which the filters popover keeps in step by hand.
        function resetFilters(keys) {
            for (const key of keys) q(FILTER_SELECTS[key]).value = '';
            store.setFilters(Object.fromEntries(keys.map(key => [key, ''])));
        }
        function removeFilter(key) {
            const index = [...chipsEl.querySelectorAll('[data-remove-filter]')].findIndex(chip => chip.dataset.removeFilter === key);
            resetFilters([key]);
            const left = [...chipsEl.querySelectorAll('[data-remove-filter]:not([hidden])')];
            (left[Math.min(index, left.length - 1)] || chipsEl.querySelector('[data-filter-overflow]:not([hidden])') || searchInput).focus();
        }
        const onRootClick = event => {
            if (event.target.closest(OWN_CLICKS)) return;
            const chip = event.target.closest('[data-remove-filter]');
            if (chip) { removeFilter(chip.dataset.removeFilter); return; }
            const viewButton = event.target.closest('.infra-view-switch [data-view]');
            if (viewButton) { store.setView(viewButton.dataset.view); return; }
            const button = event.target.closest('[data-action]');
            if (!button || button.disabled) return;
            const view = mapView();
            const menuItem = !!button.closest('[role="menuitem"]');
            switch (button.dataset.action) {
                case 'refresh': void store.loadSnapshot(true); break;
                case 'add': case 'add-empty': dialogs.openNodeDialog('device'); break;
                case 'add-place': dialogs.openNodeDialog('room', { title: t('desktop.infra_new_place') }); break;
                case 'open-search': focusSearch(); return;
                case 'add-menu': openMenu('add'); return;
                case 'more': openMenu('more'); return;
                case 'filters': openMenu('filters'); return;
                // The sheet remembers the focused element: take the focus back to the trigger first.
                case 'import': case 'import-empty': if (menuItem) closeMenus({ restoreFocus: true }); importer.open(); return;
                case 'arrange': gestures.arrange(); break;
                case 'clear-filters': resetFilters(Object.keys(FILTER_SELECTS)); return;
                case 'shortcuts': if (menuItem) closeMenus({ restoreFocus: true }); menus.openShortcuts(); return;
                case 'legend': if (menuItem) closeMenus(); openMenu('legend'); return;
                case 'undo': case 'history-undo': if (store.canUndo(view)) void store.undo(view); break;
                case 'redo': case 'history-redo': if (store.canRedo(view)) void store.redo(view); break;
                case 'toggle-library': toggleLibrary(); return;
                case 'retry-layout': void store.retryLayout(view); break;
                case 'clear-search': clearSearch(); break;
                case 'reset-filters': resetFilters(Object.keys(FILTER_SELECTS)); break;
                case 'clear-search-filters': clearSearch(); resetFilters(Object.keys(FILTER_SELECTS)); break;
                case 'dismiss-toast': { const element = button.closest('[data-toast]'); if (element) removeToast(element); return; }
                default: return;
            }
            if (menuItem) closeMenus({ restoreFocus: true });
        };
        root.addEventListener('click', onRootClick);
        scrim.addEventListener('click', () => { inspector.close({ restoreFocus: false }); setLibrarySheet(false); });

        // ---- search ---------------------------------------------------------------------------------
        // Below SEARCH_COLLAPSE px the search is one icon button; it expands over the command bar until the focus
        // leaves it or Escape is pressed (the dot on the icon says a query is still applied).
        const searchCollapsed = () => root.clientWidth < SEARCH_COLLAPSE;
        function setSearchOpen(open, { restoreFocus = false } = {}) {
            const on = !!open && searchCollapsed();
            root.dataset.searchOpen = String(on);
            searchOpenButton.setAttribute('aria-expanded', String(on));
            if (!on && restoreFocus) searchOpenButton.focus();
        }
        function focusSearch() {
            setSearchOpen(true);
            searchInput.focus();
            searchInput.select?.();
        }
        root.addEventListener('focusout', event => {
            if (root.dataset.searchOpen !== 'true') return;
            const next = event.relatedTarget;
            if (next && (searchGroup.contains(next) || q('[data-popover="filters"]')?.contains(next))) return;
            setSearchOpen(false);
        });
        function firstMatchId() {
            const matching = node => !!node && store.matches(node);
            if (state.view === 'table') return state.nodes.filter(matching).sort((a, b) => a.name.localeCompare(b.name))[0]?.id || '';
            const scene = canvas.sceneData();
            const ordered = scene.cards.slice().sort((a, b) => a.y - b.y || a.x - b.x);
            const card = ordered.find(item => matching(store.nodeById(item.id)));
            if (card) return card.id;
            const hostOf = store.guests().hostOf;
            const guest = state.nodes.find(node => matching(node) && scene.cardById.has(hostOf.get(node.id)));
            return guest ? hostOf.get(guest.id) : '';
        }
        function jumpToSearchMatch() {
            if (searchFrame) { cancelAnimationFrame(searchFrame); searchFrame = 0; }
            if (state.filters.query !== searchInput.value) store.setFilters({ query: searchInput.value });
            canvas.renderNow();
            const id = firstMatchId();
            if (!id) return;
            store.select([id]);
            if (state.view === 'table') return;
            canvas.centerOn(id, { animate: animationsEnabled() });
            canvas.renderNow();
            canvas.setRovingCard(id);
            canvas.cardEl(id)?.focus({ preventScroll: true });
        }
        searchInput.addEventListener('input', () => {
            searchValue = searchInput.value;
            // One filter change per animation frame keeps typing responsive on large maps.
            if (searchFrame) return;
            searchFrame = requestAnimationFrame(() => {
                searchFrame = 0;
                if (!disposed) store.setFilters({ query: searchValue });
            });
        });
        searchInput.addEventListener('keydown', event => {
            if (event.key === 'Enter') { event.preventDefault(); jumpToSearchMatch(); }
            else if (event.key === 'Escape' && root.dataset.searchOpen === 'true') { event.preventDefault(); event.stopPropagation(); setSearchOpen(false, { restoreFocus: true }); }
            else if (event.key === 'Escape' && searchInput.value) { event.preventDefault(); event.stopPropagation(); clearSearch(); }
        });

        // ---- store events ---------------------------------------------------------------------------
        function paintSnapshot() { renderData(); paintShell(); inspector.paintIfIdle(); }
        function route(change) {
            if (disposed) return;
            switch (change.type) {
                case 'snapshot':
                    if (state.interacting) { deferred = deferred || !!(change.structural || change.explicit); return; }
                    if (!change.structural && !change.explicit) paintShell(); else paintSnapshot();
                    break;
                case 'interaction-end':
                    if (deferred) { deferred = false; paintSnapshot(); }
                    break;
                case 'layout': renderData(); paintShell(); break;
                case 'nodes': case 'edges': renderData(); paintShell(); inspector.paintIfIdle(); break;
                case 'expanded': renderData(); break;
                case 'layout-status': canvas.render(); library.render(); paintShell(); break;
                case 'sources': library.render(); paintShell(); importer.render(); break;
                case 'source-hosts': library.render(); canvas.render(); importer.render(); break;
                case 'candidates': importer.render(); break;
                case 'selection':
                    canvas.render(); library.renderSelection(); renderTable(); inspector.paint(); paintArrange();
                    if (isNarrow() && root.dataset.inspectorOpen === 'true') setLibrarySheet(false);
                    break;
                case 'view': paintShell(); canvas.render({ force: true }); library.render(); renderTable(); inspector.paint(); syncMenus(); break;
                case 'filters': renderData(); paintShell(); break;
                case 'tool': canvas.applyTool(); break;
                case 'busy': paintShell(); inspector.onBusy(change.key); dialogs.onBusy(change.key); importer.onBusy(change.key); break;
                case 'caps':
                    if (!store.canWrite()) gestures.cancel();
                    app.picker.close({ restoreFocus: false });
                    syncMenus(); paintShell(); canvas.render({ force: true }); library.render(); inspector.paint(); importer.render();
                    break;
                case 'history': paintShell(); syncMenus(); break;
                case 'notice': toast(change.message, change.kind); break;
                case 'sync': paintShell(); if (change.announce) announce(change.announce); break;
                case 'targets': case 'credentials': inspector.paintIfIdle(); break;
                case 'manifest': canvas.render({ force: true }); library.render(); break;
                default: break;
            }
        }
        unsubscribe = store.subscribe(route);

        // ---- window wiring ---------------------------------------------------------------------------
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(root, { onContextMenu: event => menus.onContextMenu(event) });
        else root.addEventListener('contextmenu', event => { if (menus.onContextMenu(event)) { event.preventDefault(); event.stopPropagation(); } });
        root.addEventListener('focusout', () => inspector.flushStale());
        root.addEventListener('keydown', event => gestures.onRootKeydown(event));
        if (typeof ResizeObserver === 'function') {
            resizeObserver = new ResizeObserver(() => {
                if (resizeFrame) return;
                resizeFrame = requestAnimationFrame(() => {
                    resizeFrame = 0;
                    if (disposed) return;
                    if (root.dataset.searchOpen === 'true' && !searchCollapsed()) setSearchOpen(false);
                    fitFilterChips();
                    if (!isNarrow() && root.dataset.librarySheet === 'open') root.dataset.librarySheet = 'closed';
                    syncLibraryToggle();
                });
            });
            resizeObserver.observe(root);
        }
        gestures.attach();
        syncMenus();
        ctx.setWindowBeforeClose?.(windowId, async () => (!store.hasUnsavedWork() && !state.busy.has('layout')
            ? true
            : dialogs.confirmAction(t('desktop.app_infra'), t('desktop.infra_unsaved_close'), { confirmLabel: t('desktop.infra_discard') })));
        store.attach({ root, win: host.closest('.vd-window') });
        paintShell(); canvas.render(); library.render(); renderTable(); inspector.paint();

        INSTANCES.set(windowId, {
            dispose() {
                if (disposed) return;
                disposed = true;
                closeMenus();
                app.picker.dispose();
                app.cleanup.dispose();
                gestures.dispose(); menus.dispose(); canvas.dispose(); library.dispose(); inspector.dispose(); dialogs.dispose(); importer.dispose(); table.dispose();
                for (const element of [...toastTimers.keys()]) removeToast(element);
                clearTimeout(announceTimer); cancelAnimationFrame(searchFrame); cancelAnimationFrame(resizeFrame); cancelAnimationFrame(chipsFrame);
                resizeObserver?.disconnect();
                if (prefsTimer) flushPrefs();
                unsubscribe?.();
                store.dispose();
                ctx.setWindowBeforeClose?.(windowId, null);
                ctx.setWindowMenus?.(windowId, []);
                root.replaceChildren();
            },
        });
    }

    function dispose(windowId) {
        const instance = INSTANCES.get(windowId);
        if (!instance) return;
        instance.dispose();
        INSTANCES.delete(windowId);
    }

    window.InfraApp = { render, dispose };
})();
