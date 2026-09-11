/**
 * Gallery menu builders (window menubar, sort popover, context menus).
 *
 * Every builder receives a menu model `m`:
 *   { t, g, readonly, tileSizes, sorts, itemKind, actions }
 * where `g` is the live gallery state and `actions` exposes the gallery's
 * operations (setTab, setSort, setGroup, setInfoOpen, setTileIndex, reload,
 * openItem, downloadItems, editInPixel, showInFiles, copyLink, renameItem,
 * deleteItems, selectAll, clearSelection, setSelectMode, toggleSelection,
 * targetItems). Builders return shell menu item arrays. Every item carries a
 * stable `id`: the shell derives action keys from it, so items without one
 * would collide when their handlers share the same source text.
 */
(function () {
    'use strict';

    function sortMenuItems(m) {
        return m.sorts.map(sort => ({
            id: `sort-${sort}`,
            label: m.t(`desktop.gallery_sort_${sort}`),
            checked: m.g.sort === sort,
            action: () => m.actions.setSort(sort)
        }));
    }

    function groupItem(m) {
        const { t, g, actions } = m;
        return { id: 'group', label: t('desktop.gallery_group_by_date'), checked: g.group, disabled: !(g.sort === 'newest' || g.sort === 'oldest'), action: () => actions.setGroup(!g.group) };
    }

    /** Context menus have no checkmark column; mirror `checked` into the icon slot. */
    function withCheckIcons(items) {
        return items.map(item => {
            if (!item || item.separator || typeof item.checked !== 'boolean') return item;
            return Object.assign({}, item, {
                icon: item.checked ? 'check' : (item.icon || 'square'),
                fallback: item.checked ? '✓' : (item.fallback || '☐')
            });
        });
    }

    function sortPopoverItems(m) {
        return withCheckIcons(sortMenuItems(m).concat([{ separator: true }, groupItem(m)]));
    }

    function viewMenuItems(m) {
        const { t, g, actions, tileSizes } = m;
        return [
            { id: 'refresh', label: t('desktop.gallery_refresh'), icon: 'refresh', fallback: '↻', shortcut: 'F5', action: () => actions.reload() },
            { separator: true },
            { id: 'tab-photos', label: t('desktop.gallery_photos'), icon: 'image', fallback: 'P', checked: g.tab === 'Photos', action: () => actions.setTab('Photos') },
            { id: 'tab-videos', label: t('desktop.gallery_videos'), icon: 'video', fallback: 'V', checked: g.tab === 'Videos', action: () => actions.setTab('Videos') },
            { separator: true },
            { id: 'sort', label: t('desktop.gallery_sort'), icon: 'sort', fallback: '⇅', items: sortMenuItems(m) },
            groupItem(m),
            { id: 'info', label: t('desktop.gallery_info'), icon: 'info', fallback: 'i', checked: g.infoOpen, shortcut: 'I', action: () => actions.setInfoOpen(!g.infoOpen) },
            { separator: true },
            { id: 'tile-larger', label: t('desktop.gallery_tile_larger'), icon: 'zoom-in', fallback: '+', shortcut: '+', disabled: g.tileIndex >= tileSizes.length - 1, action: () => actions.setTileIndex(g.tileIndex + 1) },
            { id: 'tile-smaller', label: t('desktop.gallery_tile_smaller'), icon: 'zoom-out', fallback: '−', shortcut: '-', disabled: g.tileIndex <= 0, action: () => actions.setTileIndex(g.tileIndex - 1) }
        ];
    }

    function windowMenus(m) {
        const { t, g, readonly, actions, itemKind } = m;
        const hasTarget = g.selection.size > 0 || !!g.byPath.get(g.focusPath);
        const single = g.selection.size <= 1 ? g.byPath.get(g.focusPath) : null;
        return [
            {
                id: 'file',
                label: t('desktop.menu_file'),
                items: [
                    { id: 'open', label: t('desktop.gallery_open'), icon: 'eye', fallback: '○', disabled: !single, action: () => { if (single) actions.openItem(single.path); } },
                    { id: 'download', label: t('desktop.gallery_download'), icon: 'gallery-action-download', fallback: '↓', disabled: !hasTarget, action: () => actions.downloadItems(actions.targetItems()) },
                    { id: 'edit-pixel', label: t('desktop.gallery_edit_pixel'), icon: 'gallery-action-edit', fallback: '✎', disabled: !single || itemKind(single, g.tab) !== 'image', action: () => actions.editInPixel(single) },
                    { id: 'show-in-files', label: t('desktop.gallery_show_in_files'), icon: 'folder', fallback: '▤', disabled: !single, action: () => actions.showInFiles(single) },
                    { id: 'copy-link', label: t('desktop.gallery_copy_link'), icon: 'link', fallback: '⛓', disabled: !single, action: () => actions.copyLink(single) },
                    { separator: true },
                    { id: 'rename', label: t('desktop.gallery_rename'), icon: 'gallery-action-edit', fallback: '✎', shortcut: 'F2', disabled: readonly || !single, action: () => actions.renameItem(single) },
                    { id: 'delete', label: t('desktop.gallery_delete'), icon: 'gallery-action-delete', fallback: '🗑', shortcut: 'Delete', disabled: readonly || !hasTarget, action: () => actions.deleteItems(actions.targetItems()) }
                ]
            },
            {
                id: 'edit',
                label: t('desktop.menu_edit'),
                items: [
                    { id: 'select-all', label: t('desktop.gallery_select_all'), icon: 'check-square', fallback: '☑', shortcut: 'Ctrl+A', disabled: !g.visible.length, action: () => actions.selectAll() },
                    { id: 'select-none', label: t('desktop.gallery_select_none'), icon: 'square', fallback: '☐', shortcut: 'Esc', disabled: !g.selection.size, action: () => { actions.setSelectMode(false); actions.clearSelection(); } },
                    { id: 'select-mode', label: t('desktop.gallery_select'), icon: 'check-square', fallback: '☑', checked: g.selectMode, action: () => actions.setSelectMode(!g.selectMode) }
                ]
            },
            {
                id: 'view',
                label: t('desktop.menu_view'),
                items: viewMenuItems(m)
            }
        ];
    }

    function itemContextItems(m, item) {
        const { t, g, readonly, actions, itemKind } = m;
        const targets = actions.targetItems(item.path);
        const multi = targets.length > 1;
        const kind = itemKind(item, g.tab);
        const selected = g.selection.has(item.path);
        const items = [
            { id: 'open', label: t('desktop.gallery_open'), icon: 'eye', fallback: '○', disabled: multi, action: () => actions.openItem(item.path) },
            { id: 'download', label: t('desktop.gallery_download'), icon: 'gallery-action-download', fallback: '↓', action: () => actions.downloadItems(targets) },
            { id: 'toggle-select', label: t(selected ? 'desktop.gallery_select_none' : 'desktop.gallery_select'), icon: selected ? 'square' : 'check-square', fallback: selected ? '☐' : '☑', action: () => actions.toggleSelection(item.path) },
            { separator: true }
        ];
        if (!multi && kind === 'image') items.push({ id: 'edit-pixel', label: t('desktop.gallery_edit_pixel'), icon: 'gallery-action-edit', fallback: '✎', action: () => actions.editInPixel(item) });
        if (!multi) {
            items.push({ id: 'show-in-files', label: t('desktop.gallery_show_in_files'), icon: 'folder', fallback: '▤', action: () => actions.showInFiles(item) });
            items.push({ id: 'copy-link', label: t('desktop.gallery_copy_link'), icon: 'link', fallback: '⛓', action: () => actions.copyLink(item) });
            items.push({ id: 'info', label: t('desktop.gallery_info'), icon: g.infoOpen ? 'check' : 'info', fallback: g.infoOpen ? '✓' : 'i', action: () => actions.setInfoOpen(!g.infoOpen) });
        }
        items.push({ separator: true });
        if (!multi) items.push({ id: 'rename', label: t('desktop.gallery_rename'), icon: 'gallery-action-edit', fallback: '✎', shortcut: 'F2', disabled: readonly, action: () => actions.renameItem(item) });
        items.push({ id: 'delete', label: t('desktop.gallery_delete'), icon: 'gallery-action-delete', fallback: '🗑', shortcut: 'Delete', disabled: readonly, action: () => actions.deleteItems(targets) });
        return items;
    }

    function backgroundContextItems(m) {
        const { t, g, actions } = m;
        const view = viewMenuItems(m).map(item => {
            if (item && item.id === 'sort') return Object.assign({}, item, { items: withCheckIcons(item.items) });
            return item;
        });
        return [
            { id: 'select-all', label: t('desktop.gallery_select_all'), icon: 'check-square', fallback: '☑', shortcut: 'Ctrl+A', disabled: !g.visible.length, action: () => actions.selectAll() },
            { separator: true }
        ].concat(withCheckIcons(view));
    }

    window.GalleryMenus = {
        sortMenuItems,
        sortPopoverItems,
        viewMenuItems,
        windowMenus,
        itemContextItems,
        backgroundContextItems
    };
})();
