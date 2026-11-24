// Infra List view: a sortable, virtualized tree of the inventory. Hosts open to
// show their guests, rows carry a checkbox for multi-selection (Shift selects a
// range) and a bulk bar acts on the selection. infra.js inserts `el`, shows it
// in the list view and calls render() on data, filter and selection events;
// scrolling re-renders the visible slice by itself. Writes go through app.dialogs
// and app.gestures, never from here.
(function () {
    'use strict';

    const PREFS_KEY = 'infra.table.v1';
    const COLUMNS = ['name', 'kind', 'place', 'address', 'hardware', 'source', 'status', 'power'];
    const FOCUS = { select: '[data-select]', rowCheck: '[data-row-check]', rowExpand: '[data-row-expand]' };
    // Column widths in px; a measured header label may widen its column. Columns drop in DROP order until the rest fits.
    const BASE = { kind: 110, place: 150, address: 120, hardware: 130, source: 130, status: 120, power: 75 };
    const DROP = ['power', 'hardware', 'source', 'place', 'kind', 'address', 'status'];
    const NAME_MIN = 170, CHECK_W = 40, ROW_H = 54;
    const ROW_SKIP = 'a, .infra-col-check, [data-row-expand]';
    const isTyping = target => !!target?.matches?.('textarea, select, [contenteditable="true"], input:not([type="checkbox"])');

    function create(app) {
        const { store, state, t, esc, model } = app;
        const labels = { name: t('desktop.infra_name'), kind: t('desktop.infra_type'), place: t('desktop.infra_place'), address: t('desktop.infra_address'), hardware: t('desktop.infra_group_hardware'), source: t('desktop.infra_source'), status: t('desktop.infra_status'), power: t('desktop.infra_col_power') };
        const titles = { ...labels, power: t('desktop.infra_power_filter') }; // the header says "Betrieb"; the tooltip names the full term
        const prefs = model.readTablePrefs(app.readStorage(PREFS_KEY));
        const expandedRows = new Set();
        const heads = COLUMNS.map(key => `<th scope="col" data-col="${key}" data-sort-col="${key}"><button type="button" class="infra-sort" data-sort="${key}" title="${esc(model.fill(t('desktop.infra_sort_by'), { column: titles[key] }))}"><span class="infra-sort-label">${esc(labels[key])}</span><span class="infra-sort-arrow" aria-hidden="true"></span></button></th>`).join('');
        const holder = document.createElement('div');
        holder.innerHTML = `<section class="infra-table-view" data-table-view hidden aria-label="${esc(t('desktop.infra_inventory'))}"><div class="infra-table-head"><h2>${esc(t('desktop.infra_inventory'))}</h2><span data-table-total></span></div>`
            + `<div class="infra-bulkbar" data-bulkbar role="group" aria-label="${esc(t('desktop.infra_bulk_actions'))}" hidden><span class="infra-bulkbar-count" data-bulk-count></span><button type="button" class="infra-button" data-action="bulk-move" aria-haspopup="dialog">${esc(t('desktop.infra_move_to_place'))}</button><button type="button" class="infra-button infra-danger-button" data-action="bulk-ignore">${esc(t('desktop.infra_remove_and_ignore'))}</button><button type="button" class="infra-button infra-danger-button" data-action="bulk-remove">${esc(t('desktop.infra_remove_node'))}</button><button type="button" class="infra-button" data-action="bulk-clear">${esc(t('desktop.infra_clear_selection'))}</button></div>`
            + `<div class="infra-table-scroll" data-table-scroll><table class="infra-table"><thead><tr><th class="infra-col-check" scope="col"><input type="checkbox" data-table-check-all aria-label="${esc(model.countText(0, t('desktop.infra_select_all_rows_one'), t('desktop.infra_select_all_rows')))}"></th>${heads}</tr></thead><tbody data-table-body></tbody></table>
              <div class="infra-table-empty" data-table-empty hidden><span class="infra-table-empty-icon" aria-hidden="true">${app.glyph('search')}</span><p data-table-message></p><div class="infra-empty-actions">
                <button type="button" class="infra-button infra-button-primary" data-table-reset hidden></button>
                <button type="button" class="infra-button infra-button-primary" data-action="refresh" hidden>${esc(t('desktop.infra_retry'))}</button>
                <button type="button" class="infra-button infra-button-primary" data-action="import-empty" hidden>${esc(t('desktop.infra_import_discoveries'))}</button>
                <button type="button" class="infra-button" data-action="add-empty" hidden>${esc(t('desktop.infra_new_asset'))}</button>
              </div></div></div></section>`;
        const el = holder.firstElementChild;
        const scroll = el.querySelector('[data-table-scroll]'), table = el.querySelector('.infra-table'), body = el.querySelector('[data-table-body]'), total = el.querySelector('[data-table-total]'), emptyEl = el.querySelector('[data-table-empty]');
        const checkAll = el.querySelector('[data-table-check-all]'), bulkbar = el.querySelector('[data-bulkbar]'), bulkCount = el.querySelector('[data-bulk-count]');
        const resetButton = el.querySelector('[data-table-reset]'), retryButton = emptyEl.querySelector('[data-action="refresh"]');
        let memo = null, anchor = '', rovingId = '', widths = null, hiddenCols = '';

        // ---- columns ---------------------------------------------------------------------------------
        // The header labels are fixed when the app is created (a language change recreates the window), so the
        // widths are measured once, at the first visible paint. Columns that do not fit are hidden by priority.
        function layoutColumns() {
            const room = scroll.clientWidth;
            if (!room) return;
            if (!widths) {
                const label = key => el.querySelector(`[data-sort="${key}"] .infra-sort-label`);
                if (!label('kind')?.offsetParent) return;
                widths = Object.fromEntries(Object.entries(BASE).map(([key, base]) => [key, Math.max(base, Math.ceil(label(key).scrollWidth) + 37)]));
                for (const [key, value] of Object.entries(widths)) table.style.setProperty(`--infra-col-${key}`, `${value}px`);
            }
            const shown = new Set(Object.keys(BASE)), need = () => CHECK_W + NAME_MIN + [...shown].reduce((sum, key) => sum + widths[key], 0);
            for (const key of DROP) { if (need() <= room) break; shown.delete(key); }
            const next = Object.keys(BASE).filter(key => !shown.has(key)).join(' ');
            if (next !== hiddenCols) { hiddenCols = next; table.dataset.hiddenCols = next; }
        }
        const resize = typeof ResizeObserver === 'function' ? new ResizeObserver(layoutColumns) : null;
        resize?.observe(scroll);

        // ---- rows ------------------------------------------------------------------------------------
        // The sorted tree is rebuilt only when the data, the filters, the sort or the open hosts change.
        function list() {
            const key = [state.nodes, state.edges, JSON.stringify(state.filters), prefs.sort, prefs.dir, JSON.stringify([...expandedRows].sort())];
            if (memo && memo.key.every((part, index) => part === key[index])) return memo;
            const places = new Map(), sourceTexts = new Map();
            const placeOf = id => {
                if (!places.has(id)) places.set(id, model.placePath(id, store.nodeById).map(item => item.name).join(' › '));
                return places.get(id);
            };
            const sourceOf = node => {
                if (!sourceTexts.has(node.id)) sourceTexts.set(node.id, (node.sources || []).map(source => t(`desktop.infra_source_${source.kind}`)).join(', '));
                return sourceTexts.get(node.id);
            };
            const compare = model.tableComparator(prefs.sort, prefs.dir, { labelOf: app.labelForKind, placeOf, sourceOf });
            const rows = model.tableRows(state.nodes, { ...store.guests(), compare, matches: store.matches, expanded: expandedRows, filtering: model.filtersActive(state.filters) });
            memo = { key, rows, placeOf, sourceOf, matching: state.nodes.filter(node => store.matches(node)).length };
            return memo;
        }
        // The body has one Tab stop: the name button of the roving row. Checkbox and expander are reached by arrow keys, Space and click.
        function rowMarkup(row, stop) {
            const node = store.nodeById(row.id);
            if (!node) return '';
            const selected = state.selection.has(node.id), id = esc(node.id), name = esc(node.name);
            const powerKey = model.powerLabel(node.power);
            const power = powerKey ? `<span class="infra-power-pill" data-power="${esc(node.power)}">${esc(t(powerKey))}</span>` : '—';
            const nodeState = model.sourceState(node);
            const live = model.liveState(node);
            const liveLabel = live ? t(model.liveLabelKey(live)) : '';
            const warn = nodeState === 'stale' || nodeState === 'removed', stateText = warn ? app.statusLabel(nodeState) : '';
            const place = memo.placeOf(node.id) || '—';
            const address = node.profile?.ip_address || node.profile?.mac_address || '—';
            const hardware = [node.profile?.vendor, node.profile?.model].filter(Boolean).join(' · ') || '—';
            const source = memo.sourceOf(node) || t('desktop.infra_manual');
            const tip = text => text === '—' ? '' : ` title="${esc(text)}"`;
            const disclosure = row.guests
                ? `<button type="button" class="infra-library-disclosure" data-row-expand="${id}" tabindex="-1" aria-expanded="${row.expanded}" aria-label="${esc(model.fill(row.expanded ? t('desktop.infra_hide_guests') : t('desktop.infra_show_guests'), { name: node.name }))}">${app.glyph('chevron')}</button>`
                : '<span class="infra-disclosure-spacer" aria-hidden="true"></span>';
            const status = `<span class="infra-status-cell">${live ? `<span class="infra-live-pill" data-live="${esc(live)}" title="${esc(liveLabel)}"><i></i><span>${esc(liveLabel)}</span></span>` : '<span aria-hidden="true">—</span>'}`
                + `${warn ? `<span class="infra-status-warn" data-state="${esc(nodeState)}" title="${esc(stateText)}"><span aria-hidden="true">!</span><span class="infra-sr-only">${esc(stateText)}</span></span>` : ''}</span>`;
            return `<tr data-row="${id}" data-depth="${row.depth}" data-kind="${esc(node.kind)}" data-state="${esc(nodeState)}"${live ? ` data-reach="${esc(live)}"` : ''} class="${selected ? 'is-selected' : ''}${row.context ? ' is-context' : ''}">`
                + `<td class="infra-col-check"><input type="checkbox" data-row-check="${id}" tabindex="-1" aria-label="${esc(model.fill(t('desktop.infra_select_row'), { name: node.name }))}"${selected ? ' checked' : ''}></td>`
                + `<td class="infra-col-name" data-col="name"><div class="infra-table-name-cell" style="--infra-depth:${row.depth}">${disclosure}<button type="button" class="infra-table-name" data-select="${id}" tabindex="${node.id === stop ? 0 : -1}" aria-current="${selected ? 'true' : 'false'}" title="${name}"><i class="infra-table-icon" aria-hidden="true">${app.icon(node.icon)}${live ? `<i class="infra-live-dot" data-live="${esc(live)}"></i>` : ''}</i><span>${name}</span>${row.guests ? `<small class="infra-library-count">${row.guests}</small>` : ''}</button>${liveLabel ? `<span class="infra-sr-only">· ${esc(liveLabel)}</span>` : ''}</div></td>`
                + `<td data-col="kind"><span class="infra-kind-tag" data-kind="${esc(node.kind)}"><i aria-hidden="true"></i>${esc(app.labelForKind(node.kind))}</span></td><td data-col="place"${tip(place)}>${esc(place)}</td><td class="infra-col-address" data-col="address"${tip(address)}>${esc(address)}</td>`
                + `<td data-col="hardware"${tip(hardware)}>${esc(hardware)}</td><td data-col="source"${tip(source)}>${esc(source)}</td>`
                + `<td data-col="status">${status}</td><td data-col="power">${power}</td></tr>`;
        }

        // ---- painting --------------------------------------------------------------------------------
        function focusSnapshot() {
            const active = document.activeElement;
            if (!active || !body.contains(active)) return null;
            const attr = Object.keys(FOCUS).find(name => active.dataset?.[name] !== undefined);
            return attr ? { attr, id: active.dataset[attr] } : null;
        }
        function restoreFocus(snapshot) {
            if (!snapshot) return;
            [...body.querySelectorAll(FOCUS[snapshot.attr])].find(item => item.dataset[snapshot.attr] === snapshot.id)?.focus({ preventScroll: true });
        }
        function paintHead(rows) {
            for (const th of el.querySelectorAll('[data-sort-col]')) {
                if (th.dataset.sortCol === prefs.sort) th.setAttribute('aria-sort', prefs.dir === 'desc' ? 'descending' : 'ascending'); else th.removeAttribute('aria-sort');
            }
            const selected = rows.filter(row => state.selection.has(row.id)).length;
            checkAll.checked = rows.length > 0 && selected === rows.length;
            checkAll.indeterminate = selected > 0 && selected < rows.length;
            checkAll.disabled = rows.length === 0;
            checkAll.setAttribute('aria-label', model.countText(rows.length, t('desktop.infra_select_all_rows_one'), t('desktop.infra_select_all_rows')));
        }
        function paintBulk() {
            const count = [...state.selection].filter(id => store.nodeById(id)).length, write = store.canWrite();
            bulkbar.hidden = count === 0;
            bulkCount.textContent = model.fill(t('desktop.infra_selected_count'), { count });
            for (const button of bulkbar.querySelectorAll('[data-action]')) {
                const locked = !write && button.dataset.action !== 'bulk-clear';
                button.disabled = locked;
                if (locked) button.title = t('desktop.infra_read_only'); else button.removeAttribute('title');
            }
        }
        // Status updates keep the rows and focused empty-state controls mounted during polling.
        function paintStatus() {
            const current = list(), loaded = !!state.refreshAt, loading = state.busy.has('snapshot');
            const query = !!state.filters.query.trim(), filters = !!(state.filters.kind || state.filters.source || state.filters.power);
            el.setAttribute('aria-busy', String(loading));
            total.hidden = !loaded;
            total.textContent = query || filters ? model.fill(t('desktop.infra_visible_count'), { count: current.matching })
                : model.countText(current.matching, t('desktop.infra_asset_count_one'), t('desktop.infra_asset_count'));
            emptyEl.hidden = current.rows.length > 0;
            table.hidden = !emptyEl.hidden;
            if (emptyEl.hidden) return;
            const waiting = !loaded && (loading || state.sync === 'syncing');
            el.querySelector('[data-table-message]').textContent = !loaded
                ? t(waiting ? 'desktop.infra_sync_syncing' : state.sync === 'paused' ? (state.accessKey || 'desktop.infra_access_unavailable') : 'desktop.infra_sync_error')
                : t(query || filters ? 'desktop.infra_no_match' : 'desktop.infra_list_none');
            resetButton.hidden = !loaded || !(query || filters);
            resetButton.dataset.action = query ? (filters ? 'clear-search-filters' : 'clear-search') : 'reset-filters';
            resetButton.textContent = t(query ? (filters ? 'desktop.infra_clear_search_filters' : 'desktop.infra_clear_search') : 'desktop.infra_clear_filters');
            retryButton.hidden = loaded || state.sync === 'syncing';
            retryButton.disabled = loading;
            for (const button of emptyEl.querySelectorAll('[data-action="import-empty"], [data-action="add-empty"]')) {
                button.hidden = !loaded || query || filters || !store.canWrite();
                button.disabled = !store.canWrite();
            }
        }
        function render() {
            const snapshot = focusSnapshot(), current = list();
            const slice = model.virtualSlice(current.rows, scroll.scrollTop, scroll.clientHeight || 600);
            const stop = slice.rows.some(row => row.id === rovingId) ? rovingId : slice.rows[0]?.id || '';
            body.innerHTML = `<tr class="infra-virtual-spacer"><td style="height:${slice.before}px"></td></tr>${slice.rows.map(row => rowMarkup(row, stop)).join('')}<tr class="infra-virtual-spacer"><td style="height:${slice.after}px"></td></tr>`;
            paintHead(current.rows);
            paintBulk();
            paintStatus();
            restoreFocus(snapshot);
            layoutColumns();
        }
        // Scrolls the row into the body of the scroller (below the sticky header), repaints and focuses its name button.
        function focusRow(id) {
            rovingId = id;
            const index = list().rows.findIndex(row => row.id === id);
            if (index < 0) return;
            const head = el.querySelector('thead').offsetHeight, top = head + index * ROW_H;
            if (top < scroll.scrollTop + head) scroll.scrollTop = index * ROW_H;
            else if (top + ROW_H > scroll.scrollTop + scroll.clientHeight) scroll.scrollTop = top + ROW_H - scroll.clientHeight;
            render();
            body.querySelector(`[data-row="${CSS.escape(id)}"] [data-select]`)?.focus({ preventScroll: true });
        }

        // ---- events ----------------------------------------------------------------------------------------
        const selectedIds = () => [...state.selection].filter(id => store.nodeById(id));
        function onRowCheck(box, event) {
            const id = box.dataset.rowCheck, rows = list().rows;
            const from = rows.findIndex(row => row.id === anchor), to = rows.findIndex(row => row.id === id);
            if (event.shiftKey && from >= 0 && to >= 0) store.select(rows.slice(Math.min(from, to), Math.max(from, to) + 1).map(row => row.id), { additive: true, origin: 'check' });
            else store.select([id], { toggle: true, origin: 'check' });
            anchor = id;
        }
        const onClick = event => {
            const target = event.target;
            const sort = target.closest('[data-sort]');
            if (sort) {
                const key = sort.dataset.sort;
                prefs.dir = prefs.sort === key && prefs.dir === 'asc' ? 'desc' : 'asc';
                prefs.sort = key;
                app.writeStorage(PREFS_KEY, JSON.stringify({ sort: prefs.sort, dir: prefs.dir }));
                render();
                return;
            }
            const check = target.closest('[data-row-check]');
            if (check) { onRowCheck(check, event); return; }
            if (target.closest('[data-table-check-all]')) {
                const ids = list().rows.map(row => row.id);
                if (ids.length && ids.every(id => state.selection.has(id))) store.clearSelection(); else store.select(ids, { origin: 'check' });
                return;
            }
            const expand = target.closest('[data-row-expand]');
            if (expand) {
                const id = expand.dataset.rowExpand;
                if (expandedRows.has(id)) expandedRows.delete(id); else expandedRows.add(id);
                render();
                return;
            }
            const row = target.closest('[data-row]');
            if (row && !target.closest(ROW_SKIP)) {
                store.select([row.dataset.row]); // the selection event repaints the rows and keeps the focus
                return;
            }
            const button = target.closest('[data-bulkbar] [data-action]');
            if (!button || button.disabled) return;
            const ids = selectedIds();
            switch (button.dataset.action) {
                case 'bulk-move': if (ids.length) app.picker.openPlacePicker({ anchor: button, exclude: ids, onPick: placeId => { void app.gestures.moveIntoPlace(ids, placeId); } }); break;
                case 'bulk-ignore': if (ids.length) void app.dialogs.removeAndIgnore(ids); break;
                case 'bulk-remove': if (ids.length) void app.dialogs.deleteNodes(ids); break;
                case 'bulk-clear': store.clearSelection(); break;
                default: break;
            }
        };
        // Arrow keys move between rows, Left and Right fold a host or step to the host, Space toggles the checkbox, Enter opens the inspector.
        function onRowKey(event, row) {
            const id = row.dataset.row, rows = list().rows, index = rows.findIndex(item => item.id === id), key = event.key, target = event.target;
            const go = to => { const next = rows[Math.max(0, Math.min(rows.length - 1, to))]; event.preventDefault(); if (next) focusRow(next.id); };
            if (key === 'ArrowDown') go(index + 1);
            else if (key === 'ArrowUp') go(index - 1);
            else if (key === 'Home') go(0);
            else if (key === 'End') go(rows.length - 1);
            else if (key === 'ArrowRight' || key === 'ArrowLeft') {
                const expand = row.querySelector('[data-row-expand]'), open = expand?.getAttribute('aria-expanded') === 'true';
                if (expand && (key === 'ArrowRight' ? !open : open)) { event.preventDefault(); expand.click(); return; }
                const hostId = key === 'ArrowLeft' ? store.guests().hostOf.get(id) : '';
                if (hostId && rows.some(item => item.id === hostId)) { event.preventDefault(); focusRow(hostId); }
            } else if (key === ' ' && target.closest('[data-select]')) {
                event.preventDefault();
                row.querySelector('[data-row-check]')?.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, shiftKey: event.shiftKey }));
            } else if (key === 'Enter' && !target.closest('button')) store.select([id]);
        }
        const onKeyDown = event => {
            if (event.altKey || event.ctrlKey || event.metaKey || isTyping(event.target)) return;
            const row = event.target.closest?.('[data-row]');
            if (!row) return;
            if (event.key !== 'Delete' && event.key !== 'Backspace') { onRowKey(event, row); return; }
            if (!store.canWrite()) return;
            const id = row.dataset.row;
            event.preventDefault();
            void app.dialogs.deleteNodes(state.selection.has(id) && state.selection.size > 1 ? selectedIds() : [id]);
        };
        // A button's own Space activation fires on keyup in Chrome; keydown already toggled the checkbox.
        const onKeyUp = event => { if (event.key === ' ' && event.target.matches?.('[data-select]')) event.preventDefault(); };
        const onFocusIn = event => {
            const row = event.target.closest?.('[data-row]');
            if (!row) return;
            rovingId = row.dataset.row;
            for (const button of body.querySelectorAll('[data-select]')) button.tabIndex = button.dataset.select === rovingId ? 0 : -1;
        };
        const onDblClick = event => {
            const row = event.target.closest('[data-row]');
            if (!row || event.target.closest(ROW_SKIP)) return;
            const id = row.dataset.row, hostId = store.guests().hostOf.get(id);
            if (!store.effectivePlacement(id, 'atlas') && !(hostId && store.effectivePlacement(hostId, 'atlas'))) {
                if (state.selected !== id || state.selection.size !== 1) store.select([id]);
                return;
            }
            store.select([id]);
            app.ensureMapView();
            app.canvas?.renderNow();
            app.canvas?.centerOn(id, { animate: true });
        };
        const onMouseDown = event => { if (event.detail > 1 && event.target.closest('[data-row]')) event.preventDefault(); };
        el.addEventListener('click', onClick);
        el.addEventListener('dblclick', onDblClick);
        el.addEventListener('mousedown', onMouseDown);
        body.addEventListener('keydown', onKeyDown);
        body.addEventListener('keyup', onKeyUp);
        body.addEventListener('focusin', onFocusIn);
        scroll.addEventListener('scroll', render, { passive: true });
        function dispose() {
            resize?.disconnect();
            el.removeEventListener('click', onClick);
            el.removeEventListener('dblclick', onDblClick);
            el.removeEventListener('mousedown', onMouseDown);
            body.removeEventListener('keydown', onKeyDown);
            body.removeEventListener('keyup', onKeyUp);
            body.removeEventListener('focusin', onFocusIn);
            scroll.removeEventListener('scroll', render);
            el.remove();
        }

        return { el, render, paintStatus, dispose };
    }

    window.InfraAppTable = { create };
})();
