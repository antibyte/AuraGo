// Mission Control – inline icon set, window menus and context menu builders.
(function () {
    'use strict';

    const A = 'xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"';
    const ICONS = {
        play: `<svg ${A}><polygon points="6 4 20 12 6 20 6 4"/></svg>`,
        stop: `<svg ${A}><rect x="5" y="5" width="14" height="14" rx="2"/></svg>`,
        pause: `<svg ${A}><rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/></svg>`,
        plus: `<svg ${A}><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>`,
        copy: `<svg ${A}><rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`,
        edit: `<svg ${A}><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>`,
        trash: `<svg ${A}><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6"/><path d="M14 11v6"/><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>`,
        lock: `<svg ${A}><rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>`,
        unlock: `<svg ${A}><rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 7.5-2"/></svg>`,
        refresh: `<svg ${A}><path d="M21 12a9 9 0 1 1-2.6-6.4"/><polyline points="21 3 21 9 15 9"/></svg>`,
        search: `<svg ${A}><circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.7" y2="16.7"/></svg>`,
        x: `<svg ${A}><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`,
        chevronDown: `<svg ${A}><polyline points="6 9 12 15 18 9"/></svg>`,
        chevronUp: `<svg ${A}><polyline points="18 15 12 9 6 15"/></svg>`,
        chevronLeft: `<svg ${A}><polyline points="15 18 9 12 15 6"/></svg>`,
        chevronRight: `<svg ${A}><polyline points="9 18 15 12 9 6"/></svg>`,
        more: `<svg ${A}><circle cx="5" cy="12" r="1.6"/><circle cx="12" cy="12" r="1.6"/><circle cx="19" cy="12" r="1.6"/></svg>`,
        check: `<svg ${A}><polyline points="20 6 9 17 4 12"/></svg>`,
        alert: `<svg ${A}><path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`,
        info: `<svg ${A}><circle cx="12" cy="12" r="9"/><line x1="12" y1="11" x2="12" y2="16"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>`,
        clock: `<svg ${A}><circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 14"/></svg>`,
        calendar: `<svg ${A}><rect x="3" y="5" width="18" height="16" rx="2"/><line x1="16" y1="3" x2="16" y2="7"/><line x1="8" y1="3" x2="8" y2="7"/><line x1="3" y1="11" x2="21" y2="11"/></svg>`,
        bolt: `<svg ${A}><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>`,
        hand: `<svg ${A}><path d="M18 11V6a2 2 0 0 0-4 0v1"/><path d="M14 10V4a2 2 0 0 0-4 0v2"/><path d="M10 10.5V6a2 2 0 0 0-4 0v8"/><path d="M18 8a2 2 0 1 1 4 0v6a8 8 0 0 1-8 8h-2c-2.8 0-4.5-.9-5.9-2.6L3.2 15.4a2 2 0 0 1 3.1-2.5L8 15"/></svg>`,
        sidebar: `<svg ${A}><rect x="3" y="4" width="18" height="16" rx="2"/><line x1="9" y1="4" x2="9" y2="20"/></svg>`,
        history: `<svg ${A}><path d="M3 12a9 9 0 1 0 3-6.7"/><polyline points="3 3 3 9 9 9"/><polyline points="12 7 12 12 16 14"/></svg>`,
        sparkles: `<svg ${A}><path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8Z"/><path d="M19 17l.8 2.2L22 20l-2.2.8L19 23l-.8-2.2L16 20l2.2-.8Z"/></svg>`,
        mail: `<svg ${A}><rect x="3" y="5" width="18" height="14" rx="2"/><polyline points="3 7 12 13 21 7"/></svg>`,
        webhook: `<svg ${A}><path d="M18 16.5a3 3 0 1 0 3-3h-8.5"/><path d="M6 16.5a3 3 0 1 0 3 3v-8"/><path d="M12 5a3 3 0 1 1-3 3l4.3 7.5"/></svg>`,
        phone: `<svg ${A}><path d="M22 16.9v3a2 2 0 0 1-2.2 2 19.8 19.8 0 0 1-8.6-3.1 19.5 19.5 0 0 1-6-6A19.8 19.8 0 0 1 2.1 4.2 2 2 0 0 1 4.1 2h3a2 2 0 0 1 2 1.7c.1.9.4 1.8.7 2.7a2 2 0 0 1-.5 2.1L8.1 9.8a16 16 0 0 0 6 6l1.3-1.3a2 2 0 0 1 2.1-.4c.9.3 1.8.6 2.7.7a2 2 0 0 1 1.7 2.1Z"/></svg>`,
        radio: `<svg ${A}><circle cx="12" cy="12" r="2"/><path d="M16.2 7.8a6 6 0 0 1 0 8.4"/><path d="M7.8 16.2a6 6 0 0 1 0-8.4"/><path d="M19.1 4.9a10 10 0 0 1 0 14.2"/><path d="M4.9 19.1a10 10 0 0 1 0-14.2"/></svg>`,
        home: `<svg ${A}><path d="M3 11 12 3l9 8"/><path d="M5 10v10a1 1 0 0 0 1 1h4v-6h4v6h4a1 1 0 0 0 1-1V10"/></svg>`,
        plug: `<svg ${A}><path d="M12 22v-5"/><path d="M9 8V2"/><path d="M15 8V2"/><path d="M18 8v5a6 6 0 0 1-12 0V8Z"/></svg>`,
        plugOff: `<svg ${A}><path d="M12 22v-5"/><path d="M9 8V2"/><path d="M15 8V2"/><path d="M18 8v5a6 6 0 0 1-12 0V8Z"/><line x1="3" y1="3" x2="21" y2="21"/></svg>`,
        wallet: `<svg ${A}><path d="M20 7H5a2 2 0 0 1 0-4h13v4"/><path d="M3 5v14a2 2 0 0 0 2 2h16V7"/><circle cx="16" cy="14" r="1.2"/></svg>`,
        walletOff: `<svg ${A}><path d="M20 7H5a2 2 0 0 1 0-4h13v4"/><path d="M3 5v14a2 2 0 0 0 2 2h16V7"/><line x1="3" y1="3" x2="21" y2="21"/></svg>`,
        power: `<svg ${A}><path d="M18.4 6.6a9 9 0 1 1-12.8 0"/><line x1="12" y1="2" x2="12" y2="12"/></svg>`,
        egg: `<svg ${A}><path d="M12 22c4.4 0 7-3.2 7-8 0-5.1-3.4-12-7-12S5 8.9 5 14c0 4.8 2.6 8 7 8Z"/></svg>`,
        nest: `<svg ${A}><path d="M3 13c0 4 4 7 9 7s9-3 9-7"/><path d="M5 13c0-2 3-4 7-4s7 2 7 4"/><path d="M8 9c1-3 2.5-5 4-5s3 2 4 5"/></svg>`,
        listCheck: `<svg ${A}><polyline points="3 6 4.5 7.5 7 5"/><polyline points="3 12 4.5 13.5 7 11"/><polyline points="3 18 4.5 19.5 7 17"/><line x1="10" y1="6" x2="21" y2="6"/><line x1="10" y1="12" x2="21" y2="12"/><line x1="10" y1="18" x2="21" y2="18"/></svg>`,
        sliders: `<svg ${A}><line x1="4" y1="21" x2="4" y2="14"/><line x1="4" y1="10" x2="4" y2="3"/><line x1="12" y1="21" x2="12" y2="12"/><line x1="12" y1="8" x2="12" y2="3"/><line x1="20" y1="21" x2="20" y2="16"/><line x1="20" y1="12" x2="20" y2="3"/><line x1="1" y1="14" x2="7" y2="14"/><line x1="9" y1="8" x2="15" y2="8"/><line x1="17" y1="16" x2="23" y2="16"/></svg>`,
        globe: `<svg ${A}><circle cx="12" cy="12" r="9"/><line x1="3" y1="12" x2="21" y2="12"/><path d="M12 3a14 14 0 0 1 0 18"/><path d="M12 3a14 14 0 0 0 0 18"/></svg>`,
        queue: `<svg ${A}><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><polygon points="3 4 6 6 3 8 3 4"/></svg>`,
        arrowUp: `<svg ${A}><line x1="12" y1="19" x2="12" y2="5"/><polyline points="5 12 12 5 19 12"/></svg>`,
        workflow: `<svg ${A}><rect x="3" y="3" width="6" height="6" rx="1.5"/><rect x="15" y="15" width="6" height="6" rx="1.5"/><path d="M9 6h4a2 2 0 0 1 2 2v7"/></svg>`
    };

    function act(m, name) { return () => { const fn = m.actions && m.actions[name]; if (typeof fn === 'function') fn(); }; }

    // Snapshot helpers – `m.s` is the shell's live state view.
    function sel(m) { return m.s && m.s.selected ? m.s.selected : null; }
    function isRemote(mission) { return !!mission && mission.runner_type === 'remote'; }

    function windowMenus(m) {
        const t = m.t;
        const ro = () => !!m.readonly;
        const none = () => !sel(m);
        const running = () => !!(m.s && m.s.running);
        const queued = () => !!(m.s && m.s.queued);
        const prep = () => (sel(m) && sel(m).preparation_status) || 'none';
        return [
            {
                id: 'file', labelKey: 'desktop.menu_file', items: [
                    { id: 'new-mission', labelKey: 'desktop.mc_new_mission', icon: 'plus', shortcut: 'Ctrl+N', disabled: ro, action: act(m, 'newMission') },
                    { id: 'duplicate', labelKey: 'desktop.mc_action_duplicate', icon: 'copy', shortcut: 'Ctrl+D', disabled: () => ro() || none(), action: act(m, 'duplicate') },
                    { type: 'separator' },
                    { id: 'run', labelKey: 'desktop.mc_action_run', icon: 'play', shortcut: 'Ctrl+Enter', disabled: () => ro() || none() || running() || queued(), action: act(m, 'run') },
                    { id: 'cancel-run', labelKey: 'desktop.mc_action_cancel', icon: 'stop', disabled: () => ro() || !(m.s && m.s.canCancel), action: act(m, 'cancelRun') },
                    { id: 'remove-from-queue', labelKey: 'desktop.mc_action_remove_queue', icon: 'list', disabled: () => ro() || !queued(), action: act(m, 'removeFromQueue') },
                    { type: 'separator' },
                    { id: 'pause-resume', label: sel(m) && sel(m).enabled === false ? t('desktop.mc_action_resume') : t('desktop.mc_action_pause'), icon: sel(m) && sel(m).enabled === false ? 'play' : 'pause', disabled: () => ro() || none() || running(), action: act(m, 'togglePause') },
                    { id: 'lock-toggle', label: sel(m) && sel(m).locked ? t('desktop.mc_action_unlock') : t('desktop.mc_action_lock'), icon: 'key', disabled: () => ro() || none(), action: act(m, 'toggleLock') },
                    { id: 'prepare', labelKey: 'desktop.mc_action_prepare', icon: 'star', disabled: () => ro() || none() || running() || prep() === 'preparing' || isRemote(sel(m)), action: act(m, 'prepare') },
                    { id: 'invalidate-prep', labelKey: 'desktop.mc_action_invalidate_prep', icon: 'refresh', disabled: () => ro() || none() || prep() === 'none' || prep() === 'preparing', action: act(m, 'invalidatePrep') },
                    { type: 'separator' },
                    { id: 'edit', labelKey: 'desktop.mc_action_edit', icon: 'edit', shortcut: 'Ctrl+E', disabled: () => ro() || none(), action: act(m, 'edit') },
                    { id: 'delete', labelKey: 'desktop.mc_action_delete', icon: 'trash', shortcut: 'Del', disabled: () => ro() || none() || !!(sel(m) && sel(m).locked) || running(), action: act(m, 'delete') }
                ]
            },
            {
                id: 'view', labelKey: 'desktop.menu_view', items: [
                    { id: 'refresh', labelKey: 'desktop.mc_toolbar_refresh', icon: 'refresh', shortcut: 'F5', action: act(m, 'refresh') },
                    { type: 'separator' },
                    { id: 'filter-all', labelKey: 'desktop.mc_filter_all', checked: () => m.s.filter === 'all', action: () => m.actions.setFilter('all') },
                    { id: 'filter-manual', labelKey: 'desktop.mc_filter_manual', checked: () => m.s.filter === 'manual', action: () => m.actions.setFilter('manual') },
                    { id: 'filter-scheduled', labelKey: 'desktop.mc_filter_scheduled', checked: () => m.s.filter === 'scheduled', action: () => m.actions.setFilter('scheduled') },
                    { id: 'filter-triggered', labelKey: 'desktop.mc_filter_triggered', checked: () => m.s.filter === 'triggered', action: () => m.actions.setFilter('triggered') },
                    { id: 'filter-errors', labelKey: 'desktop.mc_filter_errors', checked: () => m.s.filter === 'errors', action: () => m.actions.setFilter('errors') },
                    { type: 'separator' },
                    { id: 'sort-name', labelKey: 'desktop.mc_sort_name', checked: () => m.s.sort === 'name', action: () => m.actions.setSort('name') },
                    { id: 'sort-last-run', labelKey: 'desktop.mc_sort_last_run', checked: () => m.s.sort === 'last_run', action: () => m.actions.setSort('last_run') },
                    { id: 'sort-next-run', labelKey: 'desktop.mc_sort_next_run', checked: () => m.s.sort === 'next_run', action: () => m.actions.setSort('next_run') },
                    { id: 'sort-priority', labelKey: 'desktop.mc_sort_priority', checked: () => m.s.sort === 'priority', action: () => m.actions.setSort('priority') },
                    { type: 'separator' },
                    { id: 'tab-overview', labelKey: 'desktop.mc_tab_overview', shortcut: 'Ctrl+1', checked: () => m.s.tab === 'overview', disabled: none, action: () => m.actions.setTab('overview') },
                    { id: 'tab-history', labelKey: 'desktop.mc_tab_history', shortcut: 'Ctrl+2', checked: () => m.s.tab === 'history', disabled: none, action: () => m.actions.setTab('history') },
                    { type: 'separator' },
                    { id: 'list-panel', labelKey: 'desktop.mc_menu_list_panel', icon: 'columns', shortcut: 'Ctrl+B', checked: () => !m.s.listCollapsed, action: act(m, 'toggleList') }
                ]
            }
        ];
    }

    // Context menu items use the desktop showContextMenu shape: { icon, label, action, disabled } | { separator: true }.
    // NOTE: icon values in window/context menus are DESKTOP icon keys (mini symbols / Papirus set: play, pause, stop,
    // plus, copy, edit, trash, refresh, list, key, star, columns, ...). They are unrelated to the inline ICONS map above,
    // which is only used inside the app's own DOM. Unknown desktop keys render as a 3-letter text fallback - never use them.
    function missionContextItems(m, mission) {
        const t = m.t;
        const ro = !!m.readonly;
        const running = !!(m.s && m.s.running && sel(m) && sel(m).id === mission.id) || mission.status === 'running';
        const queued = !!(m.s && m.s.queuedIds && m.s.queuedIds.has(mission.id));
        const a = (name) => () => m.actions[name] && m.actions[name](mission.id);
        const items = [
            { icon: 'play', label: t('desktop.mc_action_run'), action: a('runMission'), disabled: ro || running || queued }
        ];
        if (running && !isRemote(mission)) items.push({ icon: 'stop', label: t('desktop.mc_action_cancel'), action: a('cancelMission'), disabled: ro });
        if (queued) items.push({ icon: 'list', label: t('desktop.mc_action_remove_queue'), action: a('removeMissionFromQueue'), disabled: ro });
        items.push({ separator: true });
        items.push({ icon: 'edit', label: t('desktop.mc_action_edit'), action: a('editMission'), disabled: ro });
        items.push({ icon: 'copy', label: t('desktop.mc_action_duplicate'), action: a('duplicateMission'), disabled: ro });
        items.push({ icon: mission.enabled === false ? 'play' : 'pause', label: t(mission.enabled === false ? 'desktop.mc_action_resume' : 'desktop.mc_action_pause'), action: a('togglePauseMission'), disabled: ro || running });
        items.push({ icon: 'key', label: t(mission.locked ? 'desktop.mc_action_unlock' : 'desktop.mc_action_lock'), action: a('toggleLockMission'), disabled: ro });
        items.push({ separator: true });
        items.push({ icon: 'trash', label: t('desktop.mc_action_delete'), action: a('deleteMission'), disabled: ro || !!mission.locked || running });
        return items;
    }

    function queueContextItems(m, missionId) {
        const t = m.t;
        return [
            { icon: 'list', label: t('desktop.mc_action_remove_queue'), action: () => m.actions.removeMissionFromQueue && m.actions.removeMissionFromQueue(missionId), disabled: !!m.readonly },
            { icon: 'edit', label: t('desktop.mc_action_edit'), action: () => m.actions.editMission && m.actions.editMission(missionId), disabled: !!m.readonly }
        ];
    }

    function listContextItems(m) {
        const t = m.t;
        return [
            { icon: 'plus', label: t('desktop.mc_new_mission'), action: act(m, 'newMission'), disabled: !!m.readonly },
            { icon: 'refresh', label: t('desktop.mc_toolbar_refresh'), action: act(m, 'refresh') }
        ];
    }

    window.MissionControlMenus = { ICONS, windowMenus, missionContextItems, queueContextItems, listContextItems };
})();
