(function () {
    'use strict';

    const instances = new Map();
    const P = 'vd-mc';

    const SVG = {
        play: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M6.3 2.84A1.5 1.5 0 004 4.11v11.78a1.5 1.5 0 002.3 1.27l9.344-5.891a1.5 1.5 0 000-2.538L6.3 2.84z"/></svg>',
        edit: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M5.433 13.917l1.262-3.155A4 4 0 017.58 9.42l6.92-6.918a2.121 2.121 0 013 3l-6.92 6.918c-.383.383-.84.685-1.343.886l-3.154 1.262a.5.5 0 01-.65-.65z"/><path d="M3.5 5.75c0-.69.56-1.25 1.25-1.25H10A.75.75 0 0010 3H4.75A2.75 2.75 0 002 5.75v9.5A2.75 2.75 0 004.75 18h9.5A2.75 2.75 0 0017 15.25V10a.75.75 0 00-1.5 0v5.25c0 .69-.56 1.25-1.25 1.25h-9.5c-.69 0-1.25-.56-1.25-1.25v-9.5z"/></svg>',
        trash: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M8.75 1A2.75 2.75 0 006 3.75v.443c-.795.077-1.584.176-2.365.298a.75.75 0 10.23 1.482l.149-.022.841 10.518A2.75 2.75 0 007.596 19h4.807a2.75 2.75 0 002.742-2.53l.841-10.52.149.023a.75.75 0 00.23-1.482A41.03 41.03 0 0014 4.193V3.75A2.75 2.75 0 0011.25 1h-2.5zM10 4c.84 0 1.673.025 2.5.075V3.75c0-.69-.56-1.25-1.25-1.25h-2.5c-.69 0-1.25.56-1.25 1.25v.325C8.327 4.025 9.16 4 10 4zM8.58 7.72a.75.75 0 00-1.5.06l.3 7.5a.75.75 0 101.5-.06l-.3-7.5zm4.34.06a.75.75 0 10-1.5-.06l-.3 7.5a.75.75 0 101.5.06l.3-7.5z" clip-rule="evenodd"/></svg>',
        copy: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M7 3.5A1.5 1.5 0 018.5 2h3.879a1.5 1.5 0 011.06.44l3.122 3.12A1.5 1.5 0 0117 6.622V12.5a1.5 1.5 0 01-1.5 1.5h-1v-3.379a3 3 0 00-.879-2.121L10.5 5.379A3 3 0 008.379 4.5H7v-1z"/><path d="M4.5 6A1.5 1.5 0 003 7.5v9A1.5 1.5 0 004.5 18h7a1.5 1.5 0 001.5-1.5v-5.879a1.5 1.5 0 00-.44-1.06L9.44 6.439A1.5 1.5 0 008.378 6H4.5z"/></svg>',
        chevron: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M5.22 8.22a.75.75 0 011.06 0L10 11.94l3.72-3.72a.75.75 0 111.06 1.06l-4.25 4.25a.75.75 0 01-1.06 0L5.22 9.28a.75.75 0 010-1.06z" clip-rule="evenodd"/></svg>',
        clock: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm.75-13a.75.75 0 00-1.5 0v5c0 .2.08.39.22.53l3 3a.75.75 0 101.06-1.06L10.75 9.69V5z" clip-rule="evenodd"/></svg>',
        bolt: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M11.983 1.907a.75.75 0 00-1.292-.657l-8.5 9.5A.75.75 0 002.75 12h6.572l-1.305 6.093a.75.75 0 001.292.657l8.5-9.5A.75.75 0 0017.25 8h-6.572l1.305-6.093z" clip-rule="evenodd"/></svg>',
        calendar: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M5.75 2a.75.75 0 01.75.75V4h7V2.75a.75.75 0 011.5 0V4h.25A2.75 2.75 0 0118 6.75v8.5A2.75 2.75 0 0115.25 18H4.75A2.75 2.75 0 012 15.25v-8.5A2.75 2.75 0 014.75 4H5V2.75A.75.75 0 015.75 2zm-1 5.5c-.69 0-1.25.56-1.25 1.25v6.5c0 .69.56 1.25 1.25 1.25h10.5c.69 0 1.25-.56 1.25-1.25v-6.5c0-.69-.56-1.25-1.25-1.25H4.75z" clip-rule="evenodd"/></svg>',
        hand: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M7 2a1 1 0 00-1 1v8.5L4.78 10.28a1 1 0 10-1.56 1.44l3 4A1 1 0 007 16h6a3 3 0 003-3V8a1 1 0 10-2 0V6a1 1 0 10-2 0V4a1 1 0 10-2 0V3a1 1 0 10-2 0v8a.5.5 0 11-1 0V3a1 1 0 00-1-1z"/></svg>',
        checkCircle: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd"/></svg>',
        xCircle: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z" clip-rule="evenodd"/></svg>',
        cog: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M7.84 1.804A1 1 0 018.82 1h2.36a1 1 0 01.98.804l.331 1.652a6.993 6.993 0 011.929 1.115l1.598-.54a1 1 0 011.186.447l1.18 2.044a1 1 0 01-.205 1.251l-1.267 1.113a7.047 7.047 0 010 2.228l1.267 1.113a1 1 0 01.206 1.25l-1.18 2.045a1 1 0 01-1.187.447l-1.598-.54a6.993 6.993 0 01-1.929 1.115l-.33 1.652a1 1 0 01-.98.804H8.82a1 1 0 01-.98-.804l-.331-1.652a6.993 6.993 0 01-1.929-1.115l-1.598.54a1 1 0 01-1.186-.447l-1.18-2.044a1 1 0 01.205-1.251l1.267-1.114a7.05 7.05 0 010-2.227L1.821 7.773a1 1 0 01-.206-1.25l1.18-2.045a1 1 0 011.187-.447l1.598.54A6.993 6.993 0 017.51 3.456l.33-1.652zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd"/></svg>',
        info: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z" clip-rule="evenodd"/></svg>',
        refresh: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M15.312 11.424a5.5 5.5 0 01-9.201 2.466l-.312-.311h2.433a.75.75 0 000-1.5H3.989a.75.75 0 00-.75.75v4.242a.75.75 0 001.5 0v-2.43l.31.31a7 7 0 0011.712-3.138.75.75 0 00-1.449-.39zm1.23-3.723a.75.75 0 00.219-.53V2.929a.75.75 0 00-1.5 0V5.36l-.31-.31A7 7 0 003.239 8.188a.75.75 0 101.448.389A5.5 5.5 0 0113.89 6.11l.311.31h-2.432a.75.75 0 000 1.5h4.243a.75.75 0 00.53-.219z" clip-rule="evenodd"/></svg>',
        fileText: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 6a1 1 0 011-1h6a1 1 0 110 2H7a1 1 0 01-1-1zm1 3a1 1 0 100 2h6a1 1 0 100-2H7z" clip-rule="evenodd"/></svg>',
        lock: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 1a4.5 4.5 0 00-4.5 4.5V9H5a2 2 0 00-2 2v6a2 2 0 002 2h10a2 2 0 002-2v-6a2 2 0 00-2-2h-.5V5.5A4.5 4.5 0 0010 1zm3 8V5.5a3 3 0 10-6 0V9h6z" clip-rule="evenodd"/></svg>',
        x: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z"/></svg>'
    };

    const TRIGGER_TYPES = [
        { key: 'mission_completed', icon: '✅', labelKey: 'missions.trigger_mission_completed' },
        { key: 'email_received', icon: '📧', labelKey: 'missions.trigger_email_received' },
        { key: 'webhook', icon: '🪝', labelKey: 'missions.trigger_webhook' },
        { key: 'egg_hatched', icon: '🥚', labelKey: 'missions.trigger_egg_hatched' },
        { key: 'nest_cleared', icon: '🪺', labelKey: 'missions.trigger_nest_cleared' },
        { key: 'mqtt_message', icon: '📡', labelKey: 'missions.trigger_mqtt_message' },
        { key: 'system_startup', icon: '🟢', labelKey: 'missions.trigger_system_startup' },
        { key: 'home_assistant_state', icon: '🏠', labelKey: 'missions.trigger_home_assistant_state' },
        { key: 'device_connected', icon: '🔌', labelKey: 'missions.trigger_device_connected' },
        { key: 'device_disconnected', icon: '⚡', labelKey: 'missions.trigger_device_disconnected' },
        { key: 'fritzbox_call', icon: '📞', labelKey: 'missions.trigger_fritzbox_call' },
        { key: 'budget_warning', icon: '💰', labelKey: 'missions.trigger_budget_warning' },
        { key: 'budget_exceeded', icon: '🚫', labelKey: 'missions.trigger_budget_exceeded' },
        { key: 'planner_appointment_due', icon: '📅', labelKey: 'missions.trigger_planner_appointment_due' },
        { key: 'planner_todo_overdue', icon: '📝', labelKey: 'missions.trigger_planner_todo_overdue' },
        { key: 'planner_operational_issue', icon: '🧯', labelKey: 'missions.trigger_planner_operational_issue' }
    ];

    const REMOTE_ALLOWED_TRIGGERS = new Set(['system_startup', 'mqtt_message', 'home_assistant_state']);

    const CRON_PRESETS = [
        { value: '', labelKey: 'missions.form_cron_preset_custom' },
        { value: '*/5 * * * *', labelKey: 'missions.cron_preset_every_5min' },
        { value: '*/15 * * * *', labelKey: 'missions.cron_preset_every_15min' },
        { value: '*/30 * * * *', labelKey: 'missions.cron_preset_every_30min' },
        { value: '0 * * * *', labelKey: 'missions.cron_preset_every_hour' },
        { value: '0 */6 * * *', labelKey: 'missions.cron_preset_every_6hours' },
        { value: '0 0 * * *', labelKey: 'missions.cron_preset_daily_midnight' },
        { value: '0 9 * * *', labelKey: 'missions.cron_preset_daily_9am' },
        { value: '0 9 * * 1', labelKey: 'missions.cron_preset_weekly_monday' },
        { value: '0 0 1 * *', labelKey: 'missions.cron_preset_monthly_first' }
    ];

    function render(container, windowId, context) {
        dispose(windowId);

        const { esc, t, api, notify, readonly, setWindowMenus, clearWindowMenus, wireContextMenuBoundary, confirmDialog } = context;

        const state = {
            missions: [],
            queue: { items: [], running: '' },
            webhooks: [],
            currentFilter: 'all',
            viewMode: localStorage.getItem('mc-view-mode') || 'grid',
            searchQuery: '',
            expandedCards: new Set(),
            editingId: null,
            initialLoad: false,
            lastDataHash: '',
            remoteTargets: [],
            disposed: false,
            sseHandler: null
        };
        instances.set(windowId, state);

        const $ = (suffix) => container.querySelector(`[data-mc="${suffix}"]`);
        const $$ = (suffix) => container.querySelectorAll(`[data-mc="${suffix}"]`);

        container.innerHTML = `<div class="${P}"><div class="${P}-loading" data-mc="loading">${esc(t('desktop.loading', 'Loading...'))}</div></div>`;

        setupSSE();
        loadData();
        setupMenus();
        setupContextMenu();

        function setupSSE() {
            if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
                state.sseHandler = function (payload) {
                    if (!state.initialLoad || state.disposed) return;
                    const normalized = normalizeMissionControlPayload(payload);
                    state.missions = normalized.missions;
                    state.queue = normalized.queue;
                    renderAll();
                };
                window.AuraSSE.on('mission_update', state.sseHandler);
            }
        }

        function setupMenus() {
            if (typeof setWindowMenus !== 'function') return;
            setWindowMenus(windowId, [
                {
                    id: 'file', label: 'File', items: [
                        { id: 'new', labelKey: 'desktop.mc_new_mission', icon: 'plus', shortcut: 'Ctrl+N', action: () => openModal() }
                    ]
                },
                {
                    id: 'view', label: 'View', items: [
                        { id: 'grid', labelKey: 'desktop.mc_view_grid', checked: () => state.viewMode === 'grid', action: () => setViewMode('grid') },
                        { id: 'list', labelKey: 'desktop.mc_view_list', checked: () => state.viewMode === 'list', action: () => setViewMode('list') },
                        { type: 'separator' },
                        { id: 'filter-all', labelKey: 'missions.filter_all', checked: () => state.currentFilter === 'all', action: () => setFilter('all') },
                        { id: 'filter-manual', labelKey: 'missions.filter_manual', checked: () => state.currentFilter === 'manual', action: () => setFilter('manual') },
                        { id: 'filter-scheduled', labelKey: 'missions.filter_scheduled', checked: () => state.currentFilter === 'scheduled', action: () => setFilter('scheduled') },
                        { id: 'filter-triggered', labelKey: 'missions.filter_triggered', checked: () => state.currentFilter === 'triggered', action: () => setFilter('triggered') }
                    ]
                }
            ]);
        }

        function setupContextMenu() {
            if (typeof wireContextMenuBoundary === 'function') {
                wireContextMenuBoundary(container);
            }
            container.addEventListener('contextmenu', (e) => {
                const card = e.target.closest(`.${P}-card, .${P}-card-list`);
                if (!card) return;
                e.preventDefault();
                const mid = card.dataset.missionId;
                const mission = state.missions.find(m => m.id === mid);
                if (!mission) return;
                const isRunning = mid === state.queue.running;
                const items = [
                    { icon: 'play', label: t('missions.card_btn_run_title', 'Run'), action: () => runMission(mid), disabled: isRunning },
                    { icon: 'edit', label: t('missions.card_btn_edit_title', 'Edit'), action: () => openModal(mid) },
                    { icon: 'copy', label: t('missions.card_btn_duplicate_title', 'Duplicate'), action: () => duplicateMission(mid) },
                    { separator: true },
                    { icon: 'trash', label: t('missions.card_btn_delete_title', 'Delete'), action: () => deleteMission(mid), disabled: !!mission.locked }
                ];
                if (typeof context.showContextMenu === 'function') {
                    context.showContextMenu(e.clientX, e.clientY, items);
                }
            });
        }

        function handleKeydown(e) {
            if (state.disposed) return;
            if (e.ctrlKey && e.key === 'n') { e.preventDefault(); openModal(); }
        }
        document.addEventListener('keydown', handleKeydown);

        async function loadData() {
            try {
                const data = await api('/api/missions/v2');
                const normalized = normalizeMissionControlPayload(data);
                state.missions = normalized.missions;
                state.queue = normalized.queue;
                state.initialLoad = true;
                renderAll();
            } catch (err) {
                console.error('MC: load failed', err);
                const host = container.querySelector(`.${P}`);
                if (host) {
                    const detail = err && err.message ? ': ' + esc(err.message) : '';
                    host.innerHTML = `<div class="${P}-empty"><div class="${P}-empty-icon">⚠️</div><div class="${P}-empty-text">${esc(t('missions.empty_load_error', 'Error loading missions'))}${detail}</div><button type="button" class="${P}-btn" data-mc="retry" style="margin-top:8px">${esc(t('desktop.retry', 'Retry'))}</button></div>`;
                    const retry = host.querySelector('[data-mc="retry"]');
                    if (retry) retry.addEventListener('click', () => { host.innerHTML = `<div class="${P}-loading">${esc(t('desktop.loading', 'Loading...'))}</div>`; loadData(); });
                }
            }
        }

        function normalizeMissionControlPayload(data) {
            data = data || {};
            const missions = Array.isArray(data.missions)
                ? data.missions
                : (Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []));
            return {
                missions,
                queue: normalizeMissionQueue(data.queue)
            };
        }

        function normalizeMissionQueue(queue) {
            queue = queue || {};
            return {
                items: Array.isArray(queue.items) ? queue.items : [],
                running: typeof queue.running === 'string' ? queue.running : ''
            };
        }

        function renderAll() {
            if (state.disposed) return;
            const dataHash = JSON.stringify(state.missions) + '||' + JSON.stringify(state.queue) + '||' + state.currentFilter + '||' + state.searchQuery;
            const changed = dataHash !== state.lastDataHash;
            state.lastDataHash = dataHash;

            const host = container.querySelector(`.${P}`);
            if (!host) return;

            if (!state.initialLoad) {
                host.innerHTML = `<div class="${P}-skeleton"></div><div class="${P}-skeleton"></div><div class="${P}-skeleton"></div>`;
                return;
            }

            const hasContent = host.querySelector(`.${P}-toolbar`);
            if (!hasContent) {
                host.innerHTML = buildLayout();
                bindLayoutEvents();
            }

            renderStatusBar();
            if (changed) {
                renderQueue();
                renderTabs();
                renderGrid();
            }
        }

        function buildLayout() {
            return `
                <div class="${P}-toolbar" data-mc="toolbar">
                    <button type="button" class="${P}-btn ${P}-btn-primary" data-mc="btn-new">+ ${esc(t('desktop.mc_new_mission', 'New Mission'))}</button>
                    <input type="text" class="${P}-search" data-mc="search" placeholder="${esc(t('desktop.mc_search_placeholder', 'Search missions...'))}" inputmode="search" enterkeyhint="search" autocapitalize="off">
                    <select class="${P}-filter-select" data-mc="filter-select">
                        <option value="all">${esc(t('missions.filter_all', 'All'))}</option>
                        <option value="manual">${esc(t('missions.filter_manual', 'Manual'))}</option>
                        <option value="scheduled">${esc(t('missions.filter_scheduled', 'Scheduled'))}</option>
                        <option value="triggered">${esc(t('missions.filter_triggered', 'Triggered'))}</option>
                    </select>
                    <div class="${P}-view-toggle">
                        <button type="button" class="${P}-view-btn${state.viewMode === 'grid' ? ' active' : ''}" data-mc="view-grid" title="Grid">▦</button>
                        <button type="button" class="${P}-view-btn${state.viewMode === 'list' ? ' active' : ''}" data-mc="view-list" title="List">☰</button>
                    </div>
                </div>
                <div class="${P}-status-bar" data-mc="status-bar"></div>
                <div class="${P}-queue" data-mc="queue" style="display:none"></div>
                <div class="${P}-tabs" data-mc="tabs"></div>
                <div class="${P}-grid${state.viewMode === 'list' ? ' list-view' : ''}" data-mc="grid"></div>
            `;
        }

        function bindLayoutEvents() {
            const btnNew = $('btn-new');
            if (btnNew) btnNew.addEventListener('click', () => openModal());

            const search = $('search');
            if (search) search.addEventListener('input', (e) => { state.searchQuery = e.target.value.toLowerCase(); renderGrid(); });

            const filterSel = $('filter-select');
            if (filterSel) filterSel.addEventListener('change', (e) => setFilter(e.target.value));

            const viewGrid = $('view-grid');
            const viewList = $('view-list');
            if (viewGrid) viewGrid.addEventListener('click', () => setViewMode('grid'));
            if (viewList) viewList.addEventListener('click', () => setViewMode('list'));

            const grid = $('grid');
            if (grid) {
                grid.addEventListener('click', handleGridClick);
            }

            const queueEl = $('queue');
            if (queueEl) {
                queueEl.addEventListener('click', (e) => {
                    const btn = e.target.closest(`[data-mc-action="remove-queue"]`);
                    if (btn) removeFromQueue(btn.dataset.missionId);
                });
            }
        }

        function handleGridClick(e) {
            const emptyBtn = e.target.closest('[data-mc="btn-new-empty"]');
            if (emptyBtn) { openModal(); return; }
            const actionEl = e.target.closest('[data-mc-action]');
            if (!actionEl) return;
            const mid = actionEl.dataset.missionId;
            const action = actionEl.dataset.mcAction;
            switch (action) {
                case 'toggle-expand': toggleExpand(mid); break;
                case 'run': runMission(mid); break;
                case 'edit': openModal(mid); break;
                case 'duplicate': duplicateMission(mid); break;
                case 'delete': deleteMission(mid); break;
                case 'prepare': prepareMission(mid); break;
                case 'invalidate-prep': invalidatePrep(mid); break;
                case 'view-prep': viewPrep(mid); break;
                case 'toggle-log':
                    actionEl.classList.toggle('open');
                    const log = actionEl.nextElementSibling;
                    if (log) log.style.display = log.style.display === 'none' ? '' : 'none';
                    break;
            }
        }

        function setFilter(f) {
            state.currentFilter = f;
            const sel = $('filter-select');
            if (sel) sel.value = f;
            renderTabs();
            renderGrid();
            setupMenus();
        }

        function setViewMode(m) {
            state.viewMode = m;
            localStorage.setItem('mc-view-mode', m);
            const grid = $('grid');
            if (grid) grid.classList.toggle('list-view', m === 'list');
            $$('view-grid').forEach(b => b.classList.toggle('active', m === 'grid'));
            $$('view-list').forEach(b => b.classList.toggle('active', m === 'list'));
            renderGrid();
            setupMenus();
        }

        function toggleExpand(id) {
            if (state.expandedCards.has(id)) state.expandedCards.delete(id);
            else state.expandedCards.add(id);
            const card = container.querySelector(`[data-mission-id="${CSS.escape(id)}"]`);
            if (card) card.classList.toggle('expanded', state.expandedCards.has(id));
        }

        // ── Status Bar ──
        function renderStatusBar() {
            const bar = $('status-bar');
            if (!bar) return;
            const total = state.missions.length;
            const running = state.missions.filter(m => m.status === 'running').length + (state.queue.running ? 1 : 0);
            const queued = state.queue.items.length;
            const triggered = state.missions.filter(m => m.execution_type === 'triggered').length;
            bar.innerHTML = `
                <div class="${P}-stat"><span class="${P}-stat-icon">📋</span><span><span class="${P}-stat-value">${total}</span><br><span class="${P}-stat-label">${esc(t('missions.status_total', 'Total'))}</span></span></div>
                <div class="${P}-stat${running > 0 ? ' running' : ''}"><span class="${P}-stat-icon">▶️</span><span><span class="${P}-stat-value">${running}</span><br><span class="${P}-stat-label">${esc(t('missions.status_running', 'Running'))}</span></span></div>
                <div class="${P}-stat"><span class="${P}-stat-icon">⏳</span><span><span class="${P}-stat-value">${queued}</span><br><span class="${P}-stat-label">${esc(t('missions.status_queue', 'Queue'))}</span></span></div>
                <div class="${P}-stat"><span class="${P}-stat-icon">⚡</span><span><span class="${P}-stat-value">${triggered}</span><br><span class="${P}-stat-label">${esc(t('missions.status_triggered', 'Triggered'))}</span></span></div>
            `;
        }

        // ── Queue ──
        function renderQueue() {
            const el = $('queue');
            if (!el) return;
            if (state.queue.items.length === 0 && !state.queue.running) {
                el.style.display = 'none';
                return;
            }
            el.style.display = '';
            let html = `<div class="${P}-queue-header"><span>${esc(t('missions.queue_title', 'Queue'))}</span><span class="${P}-queue-badge">${esc(t('missions.queue_serial_badge', 'Serial'))}</span></div><div class="${P}-queue-items">`;

            if (state.queue.running) {
                const rm = state.missions.find(m => m.id === state.queue.running);
                if (rm) {
                    html += `<div class="${P}-queue-item running"><span class="${P}-queue-pos">▶</span><span class="${P}-queue-name">${esc(rm.name)}</span><span class="${P}-queue-meta">${esc(t('missions.queue_running_now', 'Running...'))}</span></div>`;
                }
            }
            state.queue.items.forEach((item, idx) => {
                const m = state.missions.find(ms => ms.id === item.mission_id);
                if (!m) return;
                html += `<div class="${P}-queue-item"><span class="${P}-queue-pos">${idx + 1}</span><span class="${P}-queue-name">${esc(m.name)}</span><span class="${P}-queue-meta">${esc(t('missions.queue_priority_prefix', 'Prio:'))} ${m.priority}</span><button type="button" class="${P}-queue-remove" data-mc-action="remove-queue" data-mission-id="${escAttr(m.id)}" title="${esc(t('missions.queue_remove_title', 'Remove'))}">${SVG.x}</button></div>`;
            });
            html += '</div>';
            el.innerHTML = html;
        }

        // ── Tabs ──
        function renderTabs() {
            const el = $('tabs');
            if (!el) return;
            const filters = [
                { key: 'all', labelKey: 'missions.filter_all', fallback: 'All' },
                { key: 'manual', labelKey: 'missions.filter_manual', fallback: 'Manual' },
                { key: 'scheduled', labelKey: 'missions.filter_scheduled', fallback: 'Scheduled' },
                { key: 'triggered', labelKey: 'missions.filter_triggered', fallback: 'Triggered' }
            ];
            el.innerHTML = filters.map(f =>
                `<button type="button" class="${P}-tab${state.currentFilter === f.key ? ' active' : ''}" data-mc-filter="${f.key}">${esc(t(f.labelKey, f.fallback))}</button>`
            ).join('');
            el.onclick = (e) => {
                const btn = e.target.closest(`.${P}-tab`);
                if (btn) setFilter(btn.dataset.mcFilter);
            };
        }

        // ── Grid ──
        function renderGrid() {
            const el = $('grid');
            if (!el) return;
            let filtered = state.missions;
            if (state.currentFilter !== 'all') filtered = filtered.filter(m => m.execution_type === state.currentFilter);
            if (state.searchQuery) filtered = filtered.filter(m => (m.name || '').toLowerCase().includes(state.searchQuery) || (m.prompt || '').toLowerCase().includes(state.searchQuery));

            if (filtered.length === 0) {
                el.innerHTML = `<div class="${P}-empty" style="grid-column:1/-1"><div class="${P}-empty-icon">🚀</div><div class="${P}-empty-text">${esc(state.currentFilter === 'all' ? t('missions.empty_create_first', 'Create your first mission') : t('missions.empty_no_missions_of_type', 'No missions of this type'))}</div><button type="button" class="${P}-btn ${P}-btn-primary" data-mc="btn-new-empty">+ ${esc(t('desktop.mc_new_mission', 'New Mission'))}</button></div>`;
                return;
            }

            if (state.viewMode === 'list') {
                el.innerHTML = filtered.map(m => renderListCard(m)).join('');
            } else {
                el.innerHTML = filtered.map(m => renderGridCard(m)).join('');
            }
        }

        // ── Card Rendering ──
        function renderGridCard(mission) {
            const isRunning = mission.id === state.queue.running;
            const isQueued = state.queue.items.some(i => i.mission_id === mission.id);
            const isExpanded = state.expandedCards.has(mission.id);
            const mid = escAttr(mission.id);
            const statusKind = isRunning ? 'running' : isQueued ? 'queued' : (mission.execution_type || 'manual');
            const statusClass = isRunning ? ' running' : isQueued ? ' queued' : '';

            const chip = renderStatusChip(mission, isRunning, isQueued);
            const remoteBadge = mission.runner_type === 'remote' ? `<span class="${P}-remote-badge">${esc(mission.remote_egg_name || mission.remote_nest_name || t('missions.card_remote_badge', 'Remote'))}</span>` : '';
            const prepBadge = renderPrepBadge(mission);

            let triggerPill = '';
            if (mission.execution_type === 'triggered' && mission.trigger_config) {
                const txt = renderTriggerText(mission);
                if (txt) triggerPill = `<div class="${P}-trigger-pill" title="${escAttr(txt.replace(/<[^>]+>/g, ''))}">${SVG.bolt}<span>${txt}</span></div>`;
            }

            const lastRun = mission.last_run ? formatTime(mission.last_run) : t('missions.card_last_run_never', 'Never');
            const hasError = !isRunning && mission.last_result === 'error';
            const resultIcon = hasError ? SVG.xCircle : (mission.last_result === 'success' ? SVG.checkCircle : '');
            const resultClass = hasError ? `${P}-meta-item--error` : (mission.last_result === 'success' ? `${P}-meta-item--ok` : '');
            const lockedMark = mission.locked ? `<span class="${P}-card-name-lock" title="${esc(t('missions.card_locked_title', 'Locked'))}">${SVG.lock}</span>` : '';

            return `
                <article class="${P}-card${statusClass}${isExpanded ? ' expanded' : ''}" data-priority="${escAttr(mission.priority)}" data-status="${statusKind}" data-mission-id="${mid}">
                    <div class="${P}-card-header" data-mc-action="toggle-expand" data-mission-id="${mid}">
                        <div class="${P}-card-header-left">${chip}${remoteBadge}${prepBadge}</div>
                        <button type="button" class="${P}-expand-btn" data-mc-action="toggle-expand" data-mission-id="${mid}" aria-expanded="${isExpanded}">${SVG.chevron}</button>
                    </div>
                    <div class="${P}-card-body" data-mc-action="edit" data-mission-id="${mid}">
                        <div class="${P}-card-name"><span>${esc(mission.name)}</span>${lockedMark}</div>
                        ${triggerPill}
                        <p class="${P}-card-prompt">${esc(mission.prompt)}</p>
                        <div class="${P}-meta-row">
                            <div class="${P}-meta">
                                <span class="${P}-meta-item ${resultClass}">${resultIcon ? `<span class="${P}-meta-icon">${resultIcon}</span>` : `<span>${SVG.clock}</span>`}<span>${lastRun}</span></span>
                                <span class="${P}-meta-item">${esc(t('missions.meta_run_count', { count: mission.run_count }) || (mission.run_count + 'x'))}</span>
                            </div>
                            <div style="display:flex;gap:4px;align-items:center">
                                <button type="button" class="${P}-run-btn" data-mc-action="run" data-mission-id="${mid}" ${isRunning ? 'disabled' : ''}>${SVG.play}<span>${esc(t('missions.card_run_label', 'Run'))}</span></button>
                            </div>
                        </div>
                    </div>
                    <div class="${P}-card-expand">
                        <div class="${P}-card-expand-inner">
                            ${mission.prompt ? `<div class="${P}-prompt-full">${esc(mission.prompt)}</div>` : ''}
                            ${mission.last_output ? `<div class="${P}-log-block"><div class="${P}-log-head" data-mc-action="toggle-log">${SVG.fileText}<span>${esc(t('missions.card_view_log', 'View Log'))}</span></div><pre class="${P}-log-body" style="display:none">${esc(extractLastOutput(mission.last_output))}</pre></div>` : ''}
                            <div class="${P}-actions-secondary">
                                ${renderPrepButtons(mission, isRunning)}
                                <button type="button" class="${P}-action-btn" data-mc-action="duplicate" data-mission-id="${mid}" title="${esc(t('missions.card_btn_duplicate_title', 'Duplicate'))}">${SVG.copy}</button>
                                <button type="button" class="${P}-action-btn" data-mc-action="edit" data-mission-id="${mid}" title="${esc(t('missions.card_btn_edit_title', 'Edit'))}">${SVG.edit}</button>
                                <button type="button" class="${P}-action-btn ${P}-action-btn--danger" data-mc-action="delete" data-mission-id="${mid}" title="${esc(t('missions.card_btn_delete_title', 'Delete'))}" ${mission.locked ? 'disabled' : ''}>${SVG.trash}</button>
                            </div>
                        </div>
                    </div>
                </article>`;
        }

        function renderListCard(mission) {
            const isRunning = mission.id === state.queue.running;
            const isQueued = state.queue.items.some(i => i.mission_id === mission.id);
            const mid = escAttr(mission.id);
            const typeIcon = { manual: '👆', scheduled: '📅', triggered: '⚡' }[mission.execution_type] || '👆';
            const statusBadge = isRunning ? `<span class="${P}-chip ${P}-chip--running">${esc(t('missions.card_badge_running', 'running'))}</span>` : isQueued ? `<span class="${P}-chip ${P}-chip--queued">${esc(t('missions.card_badge_queued', 'Queued'))}</span>` : '';
            const prepBadge = renderPrepBadge(mission);
            const runnerBadge = mission.runner_type === 'remote' ? `<span class="${P}-remote-badge">${esc(mission.remote_egg_name || t('missions.card_remote_badge', 'Remote'))}</span>` : '';

            return `
                <div class="${P}-card-list${isRunning ? ' running' : ''}" data-mission-id="${mid}" data-mc-action="edit" data-mc-action-param="${mid}">
                    <span class="${P}-card-list-icon">${typeIcon}</span>
                    <span class="${P}-card-list-name">${esc(mission.name)}</span>
                    ${mission.locked ? `<span class="${P}-card-name-lock">${SVG.lock}</span>` : ''}
                    <div class="${P}-card-list-badges">${statusBadge}${prepBadge}${runnerBadge}</div>
                    <div class="${P}-card-list-actions">
                        <button type="button" class="${P}-action-btn" data-mc-action="run" data-mission-id="${mid}" title="${esc(t('missions.card_btn_run_title', 'Run'))}" ${isRunning ? 'disabled' : ''}>${SVG.play}</button>
                        <button type="button" class="${P}-action-btn" data-mc-action="edit" data-mission-id="${mid}" title="${esc(t('missions.card_btn_edit_title', 'Edit'))}">${SVG.edit}</button>
                        <button type="button" class="${P}-action-btn" data-mc-action="duplicate" data-mission-id="${mid}" title="${esc(t('missions.card_btn_duplicate_title', 'Duplicate'))}">${SVG.copy}</button>
                        <button type="button" class="${P}-action-btn ${P}-action-btn--danger" data-mc-action="delete" data-mission-id="${mid}" title="${esc(t('missions.card_btn_delete_title', 'Delete'))}" ${mission.locked ? 'disabled' : ''}>${SVG.trash}</button>
                    </div>
                </div>`;
        }

        // ── Status Chip ──
        function renderStatusChip(mission, isRunning, isQueued) {
            let kind, label, icon;
            if (isRunning) { kind = 'running'; label = t('missions.card_badge_running', 'running'); icon = SVG.play; }
            else if (isQueued) { kind = 'queued'; label = t('missions.card_badge_queued', 'Queued'); icon = SVG.clock; }
            else {
                kind = mission.execution_type || 'manual';
                if (kind === 'scheduled') { label = t('missions.filter_scheduled', 'Scheduled'); icon = SVG.calendar; }
                else if (kind === 'triggered') { label = t('missions.filter_triggered', 'Triggered'); icon = SVG.bolt; }
                else { label = t('missions.filter_manual', 'Manual'); icon = SVG.hand; }
            }
            return `<span class="${P}-chip ${P}-chip--${kind}"><span class="${P}-chip-priority" data-priority="${escAttr(mission.priority)}"></span><span class="${P}-chip-icon">${icon}</span><span>${esc(label)}</span></span>`;
        }

        function renderPrepBadge(mission) {
            const s = mission.preparation_status;
            if (!s || s === 'none') return '';
            const label = t('missions.prep_status_' + s, s);
            return `<span class="${P}-prep-badge ${s}">${esc(label)}</span>`;
        }

        function renderPrepButtons(mission, isRunning) {
            const s = mission.preparation_status || 'none';
            const mid = escAttr(mission.id);
            if (s === 'prepared') {
                return `<button type="button" class="${P}-action-btn" data-mc-action="view-prep" data-mission-id="${mid}" title="${esc(t('missions.prep_view_title', 'View'))}">${SVG.info}</button><button type="button" class="${P}-action-btn" data-mc-action="invalidate-prep" data-mission-id="${mid}" title="${esc(t('missions.prep_btn_invalidate', 'Invalidate'))}">${SVG.refresh}</button>`;
            }
            return `<button type="button" class="${P}-action-btn" data-mc-action="prepare" data-mission-id="${mid}" title="${esc(t('missions.prep_btn_prepare', 'Prepare'))}" ${s === 'preparing' || isRunning ? 'disabled' : ''}>${SVG.cog}</button>`;
        }

        // ── Trigger Text ──
        function renderTriggerText(mission) {
            const cfg = mission.trigger_config || {};
            let txt = '';
            switch (mission.trigger_type) {
                case 'mission_completed': {
                    const src = cfg.source_mission_name || cfg.source_mission_id || t('missions.trigger_info_unknown_mission', 'Unknown');
                    txt = t('missions.trigger_info_when_completed', { name: src }) || ('When "' + src + '" completed');
                    if (cfg.require_success) txt += ' ' + t('missions.trigger_info_only_on_success', '(only on success)');
                    break;
                }
                case 'email_received': {
                    const parts = [];
                    if (cfg.email_folder) parts.push(t('missions.trigger_info_folder_prefix', 'Folder:') + ' ' + cfg.email_folder);
                    if (cfg.email_subject_contains) parts.push(t('missions.trigger_info_subject_prefix', 'Subject:') + ' "' + cfg.email_subject_contains + '"');
                    if (cfg.email_from_contains) parts.push(t('missions.trigger_info_from_prefix', 'From:') + ' "' + cfg.email_from_contains + '"');
                    txt = parts.length > 0 ? parts.join(' | ') : t('missions.trigger_info_any_email', 'On any email');
                    break;
                }
                case 'webhook': txt = t('missions.trigger_info_webhook_prefix', 'Webhook:') + ' ' + (cfg.webhook_slug || cfg.webhook_id || t('missions.trigger_info_webhook_unknown', 'Unknown')); break;
                case 'egg_hatched': {
                    const egg = cfg.egg_name || cfg.egg_id ? t('missions.trigger_info_egg_prefix', 'Egg:') + ' ' + (cfg.egg_name || cfg.egg_id) : t('missions.trigger_info_any_egg', 'Any egg');
                    const nest = cfg.nest_name || cfg.nest_id ? ', ' + t('missions.trigger_info_nest_prefix', 'Nest:') + ' ' + (cfg.nest_name || cfg.nest_id) : '';
                    txt = '🥚 ' + egg + nest; break;
                }
                case 'nest_cleared': txt = '🪺 ' + (cfg.nest_name || cfg.nest_id ? t('missions.trigger_info_nest_prefix', 'Nest:') + ' ' + (cfg.nest_name || cfg.nest_id) : t('missions.trigger_info_any_nest', 'Any nest')); break;
                case 'mqtt_message': {
                    const parts = [t('missions.trigger_info_mqtt_topic_prefix', 'Topic:') + ' ' + (cfg.mqtt_topic || '#')];
                    if (cfg.mqtt_payload_contains) parts.push(t('missions.trigger_info_mqtt_payload_prefix', 'Payload:') + ' "' + cfg.mqtt_payload_contains + '"');
                    txt = '📡 ' + parts.join(' | '); break;
                }
                case 'system_startup': txt = t('missions.trigger_system_startup_badge', 'On System Startup'); break;
                case 'home_assistant_state': {
                    const parts = [t('missions.trigger_info_ha_entity_prefix', 'Entity:') + ' ' + (cfg.ha_entity_id || t('missions.trigger_info_ha_any_entity', 'Any'))];
                    if (cfg.ha_state_equals) parts.push(t('missions.trigger_info_ha_state_prefix', 'State:') + ' "' + cfg.ha_state_equals + '"');
                    txt = '🏠 ' + parts.join(' | '); break;
                }
                case 'device_connected': txt = '🔌 ' + t('missions.trigger_info_device_connected_prefix', 'Connected:') + ' ' + (cfg.device_name || cfg.device_id || t('missions.trigger_info_any_device', 'Any')); break;
                case 'device_disconnected': txt = '⚡ ' + t('missions.trigger_info_device_disconnected_prefix', 'Disconnected:') + ' ' + (cfg.device_name || cfg.device_id || t('missions.trigger_info_any_device', 'Any')); break;
                case 'fritzbox_call': txt = '📞 ' + t('missions.trigger_info_fritzbox_prefix', 'Fritz!Box:') + ' ' + (cfg.call_type || t('missions.trigger_info_fritzbox_any', 'Any')); break;
                case 'budget_warning': txt = '💰 ' + t('missions.trigger_budget_warning_badge', 'Budget warning'); break;
                case 'budget_exceeded': txt = '🚫 ' + t('missions.trigger_budget_exceeded_badge', 'Budget exceeded'); break;
                case 'planner_appointment_due': {
                    const parts = [];
                    if (cfg.planner_appointment_id) parts.push(t('missions.trigger_info_planner_appointment_id_prefix', 'Appointment:') + ' ' + cfg.planner_appointment_id);
                    if (cfg.planner_title_contains) parts.push(t('missions.trigger_info_planner_title_prefix', 'Title:') + ' "' + cfg.planner_title_contains + '"');
                    txt = '📅 ' + (parts.length > 0 ? parts.join(' | ') : t('missions.trigger_info_planner_any_appointment', 'Any appointment')); break;
                }
                case 'planner_todo_overdue': {
                    const parts = [];
                    if (cfg.planner_todo_id) parts.push(t('missions.trigger_info_planner_todo_id_prefix', 'Todo:') + ' ' + cfg.planner_todo_id);
                    if (cfg.planner_title_contains) parts.push(t('missions.trigger_info_planner_title_prefix', 'Title:') + ' "' + cfg.planner_title_contains + '"');
                    txt = '📝 ' + (parts.length > 0 ? parts.join(' | ') : t('missions.trigger_info_planner_any_todo', 'Any overdue todo')); break;
                }
                case 'planner_operational_issue': {
                    const parts = [];
                    if (cfg.planner_issue_source) parts.push(t('missions.trigger_info_planner_issue_source_prefix', 'Source:') + ' ' + cfg.planner_issue_source);
                    if (cfg.planner_issue_severity) parts.push(t('missions.trigger_info_planner_issue_severity_prefix', 'Severity:') + ' ' + cfg.planner_issue_severity);
                    txt = '🧯 ' + (parts.length > 0 ? parts.join(' | ') : t('missions.trigger_info_planner_any_issue', 'Any issue')); break;
                }
            }
            if (cfg.min_interval_seconds) {
                const intv = t('missions.trigger_info_min_interval_prefix', 'Min interval:') + ' ' + cfg.min_interval_seconds + 's';
                txt = txt ? txt + ' | ' + intv : intv;
            }
            return txt;
        }

        // ── API Actions ──
        async function runMission(id) {
            try {
                await api('/api/missions/v2/' + id + '/run', { method: 'POST' });
                notify(t('missions.toast_queued', 'Mission queued'));
                loadData();
            } catch (err) { notify(t('missions.toast_error_prefix', 'Error: ') + err.message, 'error'); }
        }

        async function removeFromQueue(id) {
            try {
                await api('/api/missions/v2/' + id + '/queue', { method: 'DELETE' });
                notify(t('missions.toast_removed_from_queue', 'Removed'));
                loadData();
            } catch (err) { notify(t('missions.toast_error_prefix', 'Error: ') + err.message, 'error'); }
        }

        async function deleteMission(id) {
            const m = state.missions.find(x => x.id === id);
            if (!m) return;
            if (typeof confirmDialog === 'function') {
                const ok = await confirmDialog(t('missions.confirm_delete', { name: m.name }) || ('Delete "' + m.name + '"?'));
                if (!ok) return;
            }
            try {
                await api('/api/missions/v2/' + encodeURIComponent(id), { method: 'DELETE' });
                notify(t('missions.toast_mission_deleted', 'Mission deleted'));
                loadData();
            } catch (err) { notify(t('missions.toast_error_prefix', 'Error: ') + err.message, 'error'); }
        }

        function duplicateMission(id) {
            const m = state.missions.find(x => x.id === id);
            if (!m) return;
            openModal(null, m);
        }

        async function prepareMission(id) {
            try {
                await api('/api/missions/v2/' + id + '/prepare', { method: 'POST' });
                notify(t('missions.prep_toast_started', 'Preparation started'));
                loadData();
            } catch (err) { notify(t('missions.prep_toast_error', 'Error') + ': ' + err.message, 'error'); }
        }

        async function invalidatePrep(id) {
            try {
                await api('/api/missions/v2/' + id + '/prepared', { method: 'DELETE' });
                notify(t('missions.prep_toast_invalidated', 'Invalidated'));
                loadData();
            } catch (err) { notify(t('missions.prep_toast_error', 'Error') + ': ' + err.message, 'error'); }
        }

        async function viewPrep(id) {
            try {
                const data = await api('/api/missions/v2/' + id + '/prepared');
                let content = '';
                if (data.analysis) {
                    const a = data.analysis;
                    if (a.summary) content += a.summary + '\n\n';
                    if (a.essential_tools && a.essential_tools.length) { content += '── Tools ──\n'; a.essential_tools.forEach(tool => { content += '• ' + tool.tool_name + ': ' + tool.purpose + '\n'; }); content += '\n'; }
                    if (a.step_plan && a.step_plan.length) { content += '── Steps ──\n'; a.step_plan.forEach((s, i) => { content += (i + 1) + '. ' + s.action + (s.expectation ? ' — ' + s.expectation : '') + '\n'; }); content += '\n'; }
                    if (a.pitfalls && a.pitfalls.length) { content += '── Pitfalls ──\n'; a.pitfalls.forEach(p => { content += '⚠ ' + p.risk + (p.mitigation ? ' → ' + p.mitigation : '') + '\n'; }); }
                    if (data.confidence) content += t('missions.prep_confidence', 'Confidence') + ': ' + Math.round(data.confidence * 100) + '%';
                } else { content = JSON.stringify(data, null, 2); }
                showInfoModal(t('missions.prep_view_title', 'Prepared Context'), content);
            } catch (err) { notify(t('missions.prep_toast_error', 'Error') + ': ' + err.message, 'error'); }
        }

        // ── Modal ──
        // Modal functions are provided by mission-control-modal.js (loaded first via module-loader.js).
        const { openModal, showInfoModal } = window.MissionControlModal.createModal(
            { P, SVG, TRIGGER_TYPES, CRON_PRESETS, esc, t, api, notify, state, escAttr, loadData, container });

        // ── Helpers ──
        function escAttr(s) { return String(s ?? '').replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }

        function extractLastOutput(raw) {
            if (!raw) return '';
            if (!raw.trimStart().startsWith('{')) return raw;
            try { const obj = JSON.parse(raw); if (obj.choices && obj.choices[0]?.message) return obj.choices[0].message.content || raw; } catch (_) { }
            return raw;
        }

        function formatTime(iso) {
            if (!iso) return t('missions.time_never', 'Never');
            const diff = Date.now() - new Date(iso).getTime();
            const min = Math.floor(diff / 60000);
            if (min < 1) return t('missions.time_just_now', 'Just now');
            if (min < 60) return t('missions.time_minutes_ago', { n: min }) || (min + 'm ago');
            const hrs = Math.floor(diff / 3600000);
            if (hrs < 24) return t('missions.time_hours_ago', { n: hrs }) || (hrs + 'h ago');
            const days = Math.floor(diff / 86400000);
            if (days < 7) return t('missions.time_days_ago', { n: days }) || (days + 'd ago');
            return new Date(iso).toLocaleDateString();
        }

        // ── Dispose ──
        dispose = function (wid) {
            const st = instances.get(wid);
            if (!st) return;
            st.disposed = true;
            if (st.sseHandler && window.AuraSSE && typeof window.AuraSSE.off === 'function') {
                window.AuraSSE.off('mission_update', st.sseHandler);
            }
            document.removeEventListener('keydown', handleKeydown);
            instances.delete(wid);
        }.bind(null, windowId);
    }

    function dispose(windowId) {
        const st = instances.get(windowId);
        if (!st) return;
        st.disposed = true;
        if (st.sseHandler && window.AuraSSE && typeof window.AuraSSE.off === 'function') {
            window.AuraSSE.off('mission_update', st.sseHandler);
        }
        instances.delete(windowId);
    }

    window.MissionControlApp = { render, dispose };
})();
