// Infra menus: context menus on cards, places, connections, library and List rows and the empty
// map, the asset menu behind the inspector's "more" button, the keyboard shortcut sheet and the
// legend. Menus are drawn by the shell's ctx.showContextMenu; writes are disabled with a hint
// while the window is read-only, and the store checks the permission again when an item runs.
(function () {
    'use strict';

    const LEGEND_STATES = ['current', 'stale', 'unknown', 'removed', 'manual'];
    const LEGEND_MARKS = { current: '●', stale: '◐', unknown: '○', removed: '✕', manual: '' };
    const LEGEND_POWER = ['running', 'paused', 'stopped'];
    const LEGEND_EDGES = ['ethernet', 'fiber', 'wifi', 'depends_on', 'powered_by'];
    const isEditable = target => !!target?.closest?.('input, textarea, select, [contenteditable="true"], [contenteditable=""]');

    function create(app) {
        const { ctx, store, state, t, esc, model } = app;
        const canWrite = () => store.canWrite();
        const appLabel = appId => t(`desktop.infra_app_${String(appId).replaceAll('-', '_')}`);
        const selectedNodes = () => [...state.selection].filter(id => store.nodeById(id));
        const show = (x, y, items) => {
            if (typeof ctx.showContextMenu !== 'function') return false;
            ctx.showContextMenu(x, y, items);
            return true;
        };
        // An item hands the focus back to the element the menu belongs to before it acts, so a
        // keyboard user lands where they were and the import sheet remembers a real element.
        const focusFirst = (anchor, run) => () => {
            if (anchor?.isConnected && typeof anchor.focus === 'function') anchor.focus({ preventScroll: true });
            run();
        };
        const write = (item, anchor) => (canWrite()
            ? { ...item, action: focusFirst(anchor, item.action) }
            : { ...item, disabled: true, disabledHint: t('desktop.infra_read_only'), action: () => {} });
        const plain = (item, anchor) => ({ ...item, action: focusFirst(anchor, item.action) });
        const separator = { type: 'separator' };
        // Keyboards may also raise the browser's own contextmenu event right after Shift+F10; it must not replace the menu.
        let keyboardAt = -1000;

        // ---- item builders -----------------------------------------------------------------
        // The menu of one asset acts on the whole selection when the asset is part of a multi-selection.
        function assetItems(id, anchor, { inspector = false, rect = null } = {}) {
            const node = store.nodeById(id);
            if (!node) return [];
            const ids = state.selection.has(id) && state.selection.size > 1 ? selectedNodes() : [id];
            const single = ids.length === 1, view = app.mapView();
            rect = rect || anchor?.getBoundingClientRect?.() || null;
            const items = [];
            if (single && !inspector) items.push(plain({ id: 'open', label: t('desktop.infra_open'), icon: 'info', action: () => app.inspector.open(id, 'overview') }, anchor));
            items.push(write({ id: 'move', label: t('desktop.infra_move_to_place'), icon: 'folder', action: () => app.picker.openPlacePicker({ anchor, rect, exclude: ids, onPick: placeId => { void app.gestures.moveIntoPlace(ids, placeId); } }) }, anchor));
            if (single) items.push(write({ id: 'connect', label: t('desktop.infra_connect_to'), icon: 'link', action: () => app.picker.openAssetPicker({ anchor, rect, label: t('desktop.infra_connect_to'), accept: other => other.id !== id, onPick: option => app.dialogs.openEdgeDialog('', id, option.id) }) }, anchor));
            if (single) (node.app_links || []).filter(link => model.APP_LINK_TARGETS.has(link.app)).forEach((link, index) => items.push(plain({ id: `app-${index}`, label: model.fill(t('desktop.infra_open_in_app'), { app: appLabel(link.app) }), icon: 'apps', action: () => app.openAppLink(link) }, anchor)));
            if (state.view !== 'table' && store.features.unplace && ids.some(item => store.effectivePlacement(item, view))) items.push(write({ id: 'unplace', label: t('desktop.infra_remove_from_map'), icon: 'x', action: () => { void app.gestures.removeFromMap(ids); } }, anchor));
            items.push(separator, write({ id: 'remove', label: t('desktop.infra_remove_node'), icon: 'trash', action: () => { void app.dialogs.deleteNodes(ids); } }, anchor));
            if (ids.some(item => (store.nodeById(item)?.sources || []).length)) items.push(write({ id: 'remove-ignore', label: t('desktop.infra_remove_and_ignore'), icon: 'trash', action: () => { void app.dialogs.removeAndIgnore(ids); } }, anchor));
            return items;
        }
        function placeItems(id, anchor, { inspector = false, rect = null } = {}) {
            if (!store.nodeById(id)) return [];
            const view = app.mapView();
            rect = rect || anchor?.getBoundingClientRect?.() || null;
            const items = [];
            if (!inspector) items.push(plain({ id: 'open', label: t('desktop.infra_open'), icon: 'info', action: () => app.inspector.open(id, 'overview') }, anchor));
            items.push(write({ id: 'rename', label: t('desktop.infra_rename'), icon: 'edit', action: () => { app.inspector.open(id, 'details'); app.inspector.focusName(); } }, anchor));
            items.push(write({ id: 'move', label: t('desktop.infra_move_to_place'), icon: 'folder', action: () => app.picker.openPlacePicker({ anchor, rect, exclude: [id], onPick: placeId => { void app.gestures.moveIntoPlace([id], placeId); } }) }, anchor));
            if (state.view !== 'table' && store.features.unplace && store.effectivePlacement(id, view)) items.push(write({ id: 'unplace', label: t('desktop.infra_remove_from_map'), icon: 'x', action: () => { void app.gestures.removeFromMap([id]); } }, anchor));
            items.push(separator, write({ id: 'remove', label: t('desktop.infra_remove_node'), icon: 'trash', action: () => { void app.dialogs.deleteNodes([id]); } }, anchor));
            return items;
        }
        // Places only get their own menu while they are the one thing selected.
        function itemsFor(id, anchor, options) {
            const node = store.nodeById(id);
            if (!node) return [];
            const multi = state.selection.has(id) && state.selection.size > 1;
            return model.isContainer(node.kind) && !multi ? placeItems(id, anchor, options) : assetItems(id, anchor, options);
        }
        function edgeItems(edgeId, anchor) {
            const edge = state.edges.find(item => item.id === edgeId);
            if (!edge) return [];
            const locked = item => (edge.source
                ? { ...item, disabled: true, disabledHint: t('desktop.infra_connection_readonly'), action: () => {} }
                : write(item, anchor));
            return [
                locked({ id: 'edit', label: t('desktop.infra_edit_connection'), icon: 'edit', action: () => { store.selectEdge(edgeId); app.inspector.focusEdgeForm(); } }),
                locked({ id: 'remove', label: t('desktop.infra_remove_connection'), icon: 'trash', action: () => { void app.dialogs.removeEdge(edgeId); } }),
            ];
        }
        function canvasItems(anchor) {
            return [
                write({ id: 'add', label: t('desktop.infra_new_asset'), icon: 'plus', action: () => app.dialogs.openNodeDialog('device') }, anchor),
                write({ id: 'add-place', label: t('desktop.infra_new_place'), icon: 'folder', action: () => app.dialogs.openNodeDialog('room', { title: t('desktop.infra_new_place') }) }, anchor),
                write({ id: 'import', label: t('desktop.infra_import_discoveries'), icon: 'download', action: () => app.importer.open() }, anchor),
                separator,
                plain({ id: 'fit', label: t('desktop.infra_zoom_fit'), icon: 'grid', action: () => app.canvas.fit({ animate: true }) }, anchor),
            ];
        }
        // Source actions are only offered to operators with control access; they were the body of the old node dialog.
        function sourceActionItems(node, anchor) {
            if (!store.canControl()) return [];
            const items = [];
            (node.sources || []).forEach(source => {
                model.sourceActions(source.kind).filter(action => store.allowedAction(node, action, source)).forEach(action => {
                    const blocked = state.actionBlocks.has(store.actionBlockKey(node.id, source, action));
                    items.push(plain({ id: `act-${items.length}`, label: `${t(`desktop.infra_action_${action}`)} · ${t(`desktop.infra_source_${source.kind}`)}`, icon: 'settings', disabled: blocked, action: () => { void app.dialogs.runAction(node.id, source, action); } }, anchor));
                });
            });
            return items.length ? [separator, ...items] : [];
        }

        // ---- targets -----------------------------------------------------------------------
        // resolve names what an element belongs to: an asset, a place, a connection or the empty map.
        // scope and selector (with {id}) find the anchor again when selecting repaints its pane.
        function resolve(element) {
            if (!(element instanceof Element)) return null;
            const canvas = app.canvas.el;
            const hit = (scope, match, type, id, selector, anchor = match) => (match && scope.contains(match) ? { type, id, scope, selector, anchor } : null);
            const chip = element.closest('[data-hosted-service]'), card = element.closest('[data-node]'), header = element.closest('[data-region-header]');
            const edge = element.closest('[data-edge]'), item = element.closest('[data-library-item]'), row = element.closest('[data-row]');
            return hit(canvas, chip, 'node', chip?.dataset.hostedService, '[data-hosted-service="{id}"]')
                || hit(canvas, card, 'node', card?.dataset.node, '[data-node="{id}"]')
                || hit(canvas, header, 'node', header?.dataset.regionHeader, '[data-region-header="{id}"]')
                || hit(canvas, edge, 'edge', edge?.dataset.edge, '[data-edge="{id}"]')
                || hit(app.library.el, item, 'node', item?.dataset.libraryItem, '[data-library-select="{id}"]', item?.querySelector('[data-library-select]') || item)
                || hit(app.table.el, row, 'node', row?.dataset.row, '[data-row="{id}"] [data-select]', row?.querySelector('[data-select]') || row)
                || hit(canvas, element.closest('[data-stage]'), 'canvas', '', '');
        }
        const refind = target => (target.anchor.isConnected || !target.id
            ? target.anchor
            : target.scope.querySelector(target.selector.replace('{id}', CSS.escape(target.id))) || target.anchor);
        // build selects the target first (a menu on a selected asset keeps the whole selection) and lists its items.
        function build(target, rect) {
            if (target.type === 'edge') {
                if (state.selectedEdge !== target.id) store.selectEdge(target.id);
                return edgeItems(target.id, refind(target));
            }
            if (target.type === 'node') {
                if (!store.nodeById(target.id)) return [];
                if (!state.selection.has(target.id)) store.select([target.id]);
                return itemsFor(target.id, refind(target), { rect });
            }
            return canvasItems(target.anchor);
        }
        function onContextMenu(event) {
            if (state.interacting || isEditable(event.target) || typeof ctx.showContextMenu !== 'function') return false;
            if (performance.now() - keyboardAt < 500) return true;
            const target = resolve(event.target);
            if (!target) return false;
            const items = build(target, target.anchor.getBoundingClientRect());
            return items.length ? show(event.clientX, event.clientY, items) : false;
        }
        // openForElement serves Shift+F10 and the context-menu key: the same menu, anchored below the element.
        function openForElement(element) {
            const target = resolve(element);
            if (!target || typeof ctx.showContextMenu !== 'function') return false;
            const rect = target.anchor.getBoundingClientRect(), items = build(target, rect);
            if (!items.length) return false;
            keyboardAt = performance.now();
            return show(rect.left + Math.min(24, rect.width / 2), rect.bottom, items);
        }
        // openNodeMenu is the inspector's "more" button: the asset menu plus the source actions.
        function openNodeMenu(id, button) {
            const node = store.nodeById(id);
            if (!node) return;
            if (typeof ctx.showContextMenu !== 'function') { app.dialogs.openNodeActions(id); return; }
            const rect = button.getBoundingClientRect();
            const items = [...itemsFor(id, button, { inspector: true }), ...sourceActionItems(node, button)];
            show(rect.left, rect.bottom, items);
        }

        // ---- shortcut sheet and legend ---------------------------------------------------------------------
        const kbd = text => `<kbd>${esc(text)}</kbd>`;
        const combo = (...keys) => keys.map(kbd).join(' ');
        function openShortcuts() {
            const ctrl = t('desktop.infra_kbd_ctrl'), shift = t('desktop.infra_kbd_shift'), space = t('desktop.infra_kbd_space');
            const drag = esc(t('desktop.infra_key_drag')), wheel = esc(t('desktop.infra_key_wheel'));
            const row = (keys, text) => `<dt>${keys}</dt><dd>${esc(text)}</dd>`;
            const section = (title, rows) => `<section><h3>${esc(title)}</h3><dl>${rows.join('')}</dl></section>`;
            const html = `<div class="infra-shortcuts">${section(t('desktop.infra_keys_general'), [
                row(combo(ctrl, 'K'), t('desktop.infra_key_search')),
                row(`${combo(ctrl, 'Z')} / ${combo(ctrl, shift, 'Z')}`, t('desktop.infra_key_undo')),
                row(combo(ctrl, 'S'), t('desktop.infra_key_save')),
                row(combo(t('desktop.infra_kbd_esc')), t('desktop.infra_key_escape')),
                row(combo('?'), t('desktop.infra_key_help')),
            ])}${section(t('desktop.infra_view_map'), [
                row(combo('←', '↑', '→', '↓'), t('desktop.infra_key_arrows')),
                row(`${combo(t('desktop.infra_kbd_enter'))} / ${combo(space)}`, t('desktop.infra_key_enter')),
                row(combo('M'), t('desktop.infra_key_move')),
                row(combo('P'), t('desktop.infra_key_place')),
                row(combo(ctrl, 'A'), t('desktop.infra_key_select_all')),
                row(combo(t('desktop.infra_kbd_delete')), t('desktop.infra_key_delete')),
                row(combo(shift, 'F10'), t('desktop.infra_key_menu')),
                row(combo('F'), t('desktop.infra_key_fit')),
                row(combo('+', '-', '0'), t('desktop.infra_key_zoom')),
                row(combo('V', 'H', 'C'), t('desktop.infra_key_tools')),
            ])}${section(t('desktop.infra_keys_pointer'), [
                row(`${kbd(shift)} + ${drag}`, t('desktop.infra_key_marquee')),
                row(`${kbd(space)} + ${drag}`, t('desktop.infra_key_pan')),
                row(`${kbd(ctrl)} + ${wheel}`, t('desktop.infra_key_wheel_zoom')),
            ])}</div>`;
            return app.dialogs.openDialog(t('desktop.infra_shortcuts'), html);
        }
        function legendHTML() {
            const states = LEGEND_STATES.map(name => `<li><span class="infra-legend-ring" data-state="${name}" aria-hidden="true">${LEGEND_MARKS[name]}</span>${esc(app.statusLabel(name))}</li>`).join('');
            const powers = LEGEND_POWER.map(power => `<li><i class="infra-legend-dot" data-power="${power}" aria-hidden="true"></i>${esc(t(model.powerLabel(power)))}</li>`).join('');
            const edges = LEGEND_EDGES.map(kind => `<li><svg class="infra-legend-line" data-kind="${kind}" viewBox="0 0 40 8" width="40" height="8" aria-hidden="true" focusable="false"><line x1="2" y1="4" x2="38" y2="4"/></svg>${esc(t(`desktop.infra_edge_${kind}`))}</li>`).join('');
            return `<section><h3>${esc(t('desktop.infra_legend_status'))}</h3><ul>${states}</ul></section>`
                + `<section><h3>${esc(t('desktop.infra_legend_power'))}</h3><ul>${powers}</ul></section>`
                + `<section><h3>${esc(t('desktop.infra_tab_connections'))}</h3><ul>${edges}</ul></section>`;
        }

        return { onContextMenu, openForElement, openNodeMenu, openShortcuts, legendHTML, dispose() {} };
    }

    window.InfraAppMenus = { create };
})();
