(function () {
    'use strict';

    // NoisemakerMenus - window menubar and context menu builders for the Noisemaker app.
    // Every builder receives a menu model m = { t, tFull, s, readonly, actions }:
    //   t(key, params, fallback) resolves desktop.noisemaker_<key>, tFull(key) any key,
    //   s is a state snapshot (see noisemaker.js menuModel()), actions are shell operations.
    // Items carry stable ids because the shell derives action keys from them.

    function withCheckIcons(items) {
        return items.map(item => {
            if (!item || item.separator || typeof item.checked !== 'boolean') return item;
            return Object.assign({}, item, {
                icon: item.checked ? 'check' : (item.icon || 'square'),
                fallback: item.checked ? 'V' : (item.fallback || '')
            });
        });
    }

    function repeatItems(m) {
        const { t, s, actions } = m;
        return ['off', 'all', 'one'].map(mode => ({
            id: 'repeat-' + mode,
            label: t('player_repeat_' + mode),
            icon: 'refresh',
            fallback: mode === 'one' ? '1' : 'R',
            checked: s.repeat === mode,
            action: () => actions.setRepeat(mode)
        }));
    }

    function windowMenus(m) {
        const { t, tFull, s, readonly, actions } = m;
        return [
            {
                id: 'file',
                label: tFull('desktop.menu_file'),
                items: [
                    { id: 'new-song', label: t('menu_new_song'), icon: 'plus', fallback: '+', action: () => actions.newSong() },
                    { id: 'download', label: t('track_download'), icon: 'download', fallback: 'D', disabled: !s.hasTarget, action: () => actions.downloadTargets() },
                    { id: 'template', label: t('track_use_template'), icon: 'edit', fallback: 'T', disabled: !s.singleTarget, action: () => actions.templateTarget() },
                    { separator: true },
                    { id: 'delete', label: t('track_delete'), icon: 'trash', fallback: 'X', disabled: readonly || !s.hasTarget, action: () => actions.deleteTargets() }
                ]
            },
            {
                id: 'edit',
                label: tFull('desktop.menu_edit'),
                items: withCheckIcons([
                    { id: 'select-all', label: t('select_all'), icon: 'check-square', fallback: 'A', disabled: !s.hasTracks, action: () => actions.selectAll() },
                    { id: 'select-none', label: t('select_none'), icon: 'square', fallback: '0', disabled: !s.selectionCount, action: () => actions.clearSelection() },
                    { id: 'select-mode', label: t('select'), icon: 'check-square', fallback: 'S', checked: !!s.selectMode, action: () => actions.setSelectMode(!s.selectMode) },
                    { separator: true },
                    { id: 'favorite-toggle', label: t(s.targetFavorite ? 'favorite_remove' : 'favorite_add'), icon: 'heart', fallback: '<3', disabled: readonly || !s.singleTarget, action: () => actions.toggleFavoriteTarget() }
                ])
            },
            {
                id: 'view',
                label: tFull('desktop.menu_view'),
                items: withCheckIcons([
                    { id: 'refresh', label: t('refresh'), icon: 'refresh', fallback: 'R', action: () => actions.refresh() },
                    { separator: true },
                    { id: 'view-grid', label: t('view_grid'), icon: 'grid', fallback: 'G', checked: s.view === 'grid', action: () => actions.setView('grid') },
                    { id: 'view-list', label: t('view_list'), icon: 'list', fallback: 'L', checked: s.view === 'list', action: () => actions.setView('list') },
                    { separator: true },
                    { id: 'filter-all', label: t('filter_all'), icon: 'audio', fallback: '*', checked: s.filter === 'all', action: () => actions.setFilter('all') },
                    { id: 'filter-favorites', label: t('filter_favorites'), icon: 'heart', fallback: '<3', checked: s.filter === 'favorites', action: () => actions.setFilter('favorites') },
                    { separator: true },
                    { id: 'create-panel', label: t('create_panel'), icon: 'columns', fallback: '|', checked: !s.createCollapsed, disabled: !!s.compact, action: () => actions.setCreateCollapsed(!s.createCollapsed) },
                    { id: 'now-playing', label: t('now_playing'), icon: 'layout', fallback: 'N', checked: !!s.nowPlayingOpen, disabled: !s.hasCurrent, action: () => actions.setNowPlayingOpen(!s.nowPlayingOpen) },
                    { separator: true },
                    { id: 'visualizer', label: t('visualizer'), icon: 'audio-player', fallback: '~', checked: !!s.visualizer, disabled: !s.visualizerAvailable, action: () => actions.setVisualizer(!s.visualizer) }
                ])
            },
            {
                id: 'playback',
                label: t('menu_playback'),
                items: withCheckIcons([
                    { id: 'play-pause', label: t('menu_play_pause'), icon: 'audio-player', fallback: '>', disabled: !s.hasCurrent && !s.hasTracks, action: () => actions.togglePlay() },
                    { id: 'next', label: t('menu_next'), icon: 'chevron-right', fallback: '>|', disabled: !s.hasCurrent, action: () => actions.next() },
                    { id: 'prev', label: t('menu_previous'), icon: 'chevron-left', fallback: '|<', disabled: !s.hasCurrent, action: () => actions.prev() },
                    { separator: true },
                    { id: 'shuffle', label: t('player_shuffle'), icon: 'sort', fallback: '%', checked: !!s.shuffle, action: () => actions.setShuffle(!s.shuffle) }
                ].concat(repeatItems(m), [
                    { separator: true },
                    { id: 'clear-queue', label: t('queue_clear'), icon: 'x', fallback: 'x', disabled: !s.queueLength, action: () => actions.clearQueue() }
                ]))
            }
        ];
    }

    function trackContextItems(m, track) {
        const { t, readonly, actions } = m;
        const targets = actions.targetTracks(track);
        const multi = targets.length > 1;
        const selected = actions.isSelected(track);
        const items = [
            { id: 'play', label: t('player_play'), icon: 'audio-player', fallback: '>', action: () => (multi ? actions.playTracks(targets) : actions.playTrack(track)) },
            { id: 'enqueue', label: t('enqueue'), icon: 'plus', fallback: '+', action: () => actions.enqueueTracks(targets) }
        ];
        if (!multi) {
            items.push({ id: 'favorite', label: t(track.favorite ? 'favorite_remove' : 'favorite_add'), icon: 'heart', fallback: '<3', checked: !!track.favorite, disabled: readonly, action: () => actions.toggleFavorite(track) });
        }
        items.push({ separator: true });
        if (!multi) items.push({ id: 'template', label: t('track_use_template'), icon: 'edit', fallback: 'T', action: () => actions.useTemplate(track) });
        items.push({ id: 'download', label: t('track_download'), icon: 'download', fallback: 'D', action: () => actions.downloadTracks(targets) });
        if (!multi) items.push({ id: 'details', label: t('details'), icon: 'info', fallback: 'i', action: () => actions.openDetails(track) });
        items.push({ separator: true });
        items.push({ id: 'toggle-select', label: t(selected ? 'select_none' : 'select'), icon: selected ? 'square' : 'check-square', fallback: selected ? '0' : 'S', action: () => actions.toggleSelection(track) });
        items.push({ separator: true });
        items.push({ id: 'delete', label: t('track_delete'), icon: 'trash', fallback: 'X', disabled: readonly, action: () => actions.deleteTracks(targets) });
        return withCheckIcons(items);
    }

    function libraryContextItems(m) {
        const { t, s, actions } = m;
        return withCheckIcons([
            { id: 'refresh', label: t('refresh'), icon: 'refresh', fallback: 'R', action: () => actions.refresh() },
            { separator: true },
            { id: 'view-grid', label: t('view_grid'), icon: 'grid', fallback: 'G', checked: s.view === 'grid', action: () => actions.setView('grid') },
            { id: 'view-list', label: t('view_list'), icon: 'list', fallback: 'L', checked: s.view === 'list', action: () => actions.setView('list') },
            { separator: true },
            { id: 'select-all', label: t('select_all'), icon: 'check-square', fallback: 'A', disabled: !s.hasTracks, action: () => actions.selectAll() }
        ]);
    }

    window.NoisemakerMenus = { windowMenus, trackContextItems, libraryContextItems, withCheckIcons };
})();
