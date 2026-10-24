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
    const isTyping = target => !!target?.matches?.('textarea, select, [contenteditable="true"], input:not([type="checkbox"])');

    function create(app) {
        const { store, state, t, esc, model } = app;
        const labels = { name: t('desktop.infra_name'), kind: t('desktop.infra_type'), place: t('desktop.infra_place'), address: t('desktop.infra_address'), hardware: t('desktop.infra_group_hardware'), source: t('desktop.infra_source'), status: t('desktop.infra_status'), power: t('desktop.infra_column_power') };
        const prefs = model.readTablePrefs(app.readStorage(PREFS_KEY));
        const expandedRows = new Set();
        const heads = COLUMNS.map(key => (key === 'hardware'
            ? `<th scope="col" data-sort-col="${key}">${esc(labels[key])}</th>`
            : `<th scope="col" data-sort-col="${key}"><button type="button" class="infra-sort" data-sort="${key}" title="${esc(model.fill(t('desktop.infra_sort_by'), { column: labels[key] }))}">${esc(labels[key])}<span class="infra-sort-arrow" aria-hidden="true"></span></button></th>`)).join('');
        const holder = document.createElement('div');
        holder.innerHTML = `<section class="infra-table-view" data-table-view hidden aria-label="${esc(t('desktop.infra_inventory'))}"><div class="infra-table-head"><h2>${esc(t('desktop.infra_inventory'))}</h2><span data-table-total></span></div>`
            + `<div class="infra-bulkbar" data-bulkbar role="group" aria-label="${esc(t('desktop.infra_bulk_actions'))}" hidden><span class="infra-bulkbar-count" data-bulk-count></span><button type="button" class="infra-button" data-action="bulk-move" aria-haspopup="dialog">${esc(t('desktop.infra_move_to_place'))}</button><button type="button" class="infra-button infra-danger-button" data-action="bulk-ignore">${esc(t('desktop.infra_remove_and_ignore'))}</button><button type="button" class="infra-button infra-danger-button" data-action="bulk-remove">${esc(t('desktop.infra_remove_node'))}</button><button type="button" class="infra-button" data-action="bulk-clear">${esc(t('desktop.infra_clear_selection'))}</button></div>`
            + `<div class="infra-table-scroll" data-table-scroll tabindex="0"><table class="infra-table"><thead><tr><th class="infra-col-check" scope="col"><input type="checkbox" data-table-check-all aria-label="${esc(model.countText(0, t('desktop.infra_select_all_rows_one'), t('desktop.infra_select_all_rows')))}"></th>${heads}</tr></thead><tbody data-table-body></tbody></table></div></section>`;
        const el = holder.firstElementChild;
        const scroll = el.querySelector('[data-table-scroll]'), body = el.querySelector('[data-table-body]'), total = el.querySelector('[data-table-total]');
        const checkAll = el.querySelector('[data-table-check-all]'), bulkbar = el.querySelector('[data-bulkbar]'), bulkCount = el.querySelector('[data-bulk-count]');
        let memo = null, anchor = '';

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
        function rowMarkup(row) {
            const node = store.nodeById(row.id);
            if (!node) return '';
            const selected = state.selection.has(node.id), id = esc(node.id), name = esc(node.name);
            const powerKey = model.powerLabel(node.power);
            const power = powerKey ? `<span class="infra-power-pill" data-power="${esc(node.power)}">${esc(t(powerKey))}</span>` : '—';
            const nodeState = model.sourceState(node);
            const place = memo.placeOf(node.id) || '—';
            const disclosure = row.guests
                ? `<button type="button" class="infra-library-disclosure" data-row-expand="${id}" aria-expanded="${row.expanded}" aria-label="${esc(model.fill(row.expanded ? t('desktop.infra_hide_guests') : t('desktop.infra_show_guests'), { name: node.name }))}">${app.glyph('chevron')}</button>`
                : '<span class="infra-disclosure-spacer" aria-hidden="true"></span>';
            return `<tr data-row="${id}" data-depth="${row.depth}" class="${selected ? 'is-selected' : ''}${row.context ? ' is-context' : ''}">`
                + `<td class="infra-col-check"><input type="checkbox" data-row-check="${id}" aria-label="${esc(model.fill(t('desktop.infra_select_row'), { name: node.name }))}"${selected ? ' checked' : ''}></td>`
                + `<td class="infra-col-name"><div class="infra-table-name-cell" style="--infra-depth:${row.depth}">${disclosure}<button type="button" class="infra-table-name" data-select="${id}" aria-current="${selected ? 'true' : 'false'}" title="${name}">${app.icon(node.icon)}<span>${name}</span>${row.guests ? `<small class="infra-library-count">${row.guests}</small>` : ''}</button></div></td>`
                + `<td>${esc(app.labelForKind(node.kind))}</td><td title="${esc(place)}">${esc(place)}</td><td class="infra-col-address">${esc(node.profile?.ip_address || node.profile?.mac_address || '—')}</td>`
                + `<td>${esc([node.profile?.vendor, node.profile?.model].filter(Boolean).join(' · ') || '—')}</td><td>${esc(memo.sourceOf(node) || t('desktop.infra_manual'))}</td>`
                + `<td><span class="infra-state-pill" data-state="${esc(nodeState)}"><i></i>${esc(app.statusLabel(nodeState))}</span></td><td>${power}</td></tr>`;
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
        function render() {
            const snapshot = focusSnapshot(), current = list();
            const slice = model.virtualSlice(current.rows, scroll.scrollTop, scroll.clientHeight || 600);
            total.textContent = model.countText(current.matching, t('desktop.infra_asset_count_one'), t('desktop.infra_asset_count'));
            body.innerHTML = `<tr class="infra-virtual-spacer"><td colspan="9" style="height:${slice.before}px"></td></tr>${slice.rows.map(rowMarkup).join('')}<tr class="infra-virtual-spacer"><td colspan="9" style="height:${slice.after}px"></td></tr>`;
            paintHead(current.rows);
            paintBulk();
            restoreFocus(snapshot);
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
            const select = target.closest('[data-select]');
            if (select) {
                store.select([select.dataset.select]); // the selection event repaints the rows and keeps the focus
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
        const onKeyDown = event => {
            if ((event.key !== 'Delete' && event.key !== 'Backspace') || event.altKey || event.ctrlKey || event.metaKey || isTyping(event.target)) return;
            const row = event.target.closest?.('[data-row]');
            if (!row || !store.canWrite()) return;
            const id = row.dataset.row;
            event.preventDefault();
            void app.dialogs.deleteNodes(state.selection.has(id) && state.selection.size > 1 ? selectedIds() : [id]);
        };
        el.addEventListener('click', onClick);
        body.addEventListener('keydown', onKeyDown);
        scroll.addEventListener('scroll', render, { passive: true });
        function dispose() {
            el.removeEventListener('click', onClick);
            body.removeEventListener('keydown', onKeyDown);
            scroll.removeEventListener('scroll', render);
            el.remove();
        }

        return { el, render, dispose };
    }

    window.InfraAppTable = { create };
})();
