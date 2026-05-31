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
                    state.missions = (payload && payload.missions) || [];
                    state.queue = (payload && payload.queue) || { items: [], running: '' };
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
                state.missions = data.missions || [];
                state.queue = data.queue || { items: [], running: '' };
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
                    <input type="text" class="${P}-search" data-mc="search" placeholder="${esc(t('desktop.mc_search_placeholder', 'Search missions...'))}">
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
        function openModal(editId, duplicateSource) {
            state.editingId = editId;
            const isEdit = !!editId;
            const mission = isEdit ? state.missions.find(m => m.id === editId) : null;
            const source = duplicateSource || mission;

            const overlay = document.createElement('div');
            overlay.className = P + '-modal-overlay';
            overlay.innerHTML = buildModalForm(source, isEdit);
            container.appendChild(overlay);

            const closeModalFn = () => { if (overlay.parentNode) overlay.parentNode.removeChild(overlay); };
            overlay.addEventListener('click', (e) => { if (e.target === overlay) closeModalFn(); });
            overlay.querySelector('[data-mc="modal-close"]').addEventListener('click', closeModalFn);
            overlay.querySelector('[data-mc="modal-cancel"]').addEventListener('click', closeModalFn);
            overlay.querySelector('[data-mc="modal-save"]').addEventListener('click', () => saveMission(overlay, closeModalFn));

            bindModalEvents(overlay, source);

            if (source) {
                fillFormFromMission(overlay, source);
            }
        }

        function buildModalForm(source, isEdit) {
            const title = isEdit ? t('missions.modal_title_edit', 'Edit Mission') : t('missions.modal_title_new', 'New Mission');
            return `
                <div class="${P}-modal">
                    <div class="${P}-modal-header">
                        <h2 class="${P}-modal-title">${esc(title)}</h2>
                        <button type="button" class="${P}-modal-close" data-mc="modal-close">${SVG.x}</button>
                    </div>
                    <div class="${P}-modal-body">
                        <input type="hidden" data-mc="form-id" value="${escAttr(source ? source.id : '')}">

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_name_label', 'Name'))}</label>
                            <input type="text" class="${P}-form-input" data-mc="form-name" placeholder="${esc(t('missions.form_name_placeholder', 'e.g. Daily Report'))}" value="${escAttr(source ? source.name : '')}">
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_prompt_label', 'Prompt'))}</label>
                            <textarea class="${P}-form-textarea" data-mc="form-prompt" placeholder="${esc(t('missions.form_prompt_placeholder', 'Describe the task...'))}">${esc(source ? source.prompt : '')}</textarea>
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_runner_label', 'Run Location'))}</label>
                            <div class="${P}-runner-selector">
                                <label class="${P}-runner-option${!source || source.runner_type !== 'remote' ? ' active' : ''}" data-mc-runner="local"><input type="radio" name="mc-runner" value="local" ${!source || source.runner_type !== 'remote' ? 'checked' : ''}><span class="${P}-runner-label">${esc(t('missions.form_runner_local_label', 'Local'))}</span><span class="${P}-runner-desc">${esc(t('missions.form_runner_local_desc', 'Run on this instance'))}</span></label>
                                <label class="${P}-runner-option${source && source.runner_type === 'remote' ? ' active' : ''}" data-mc-runner="remote"><input type="radio" name="mc-runner" value="remote" ${source && source.runner_type === 'remote' ? 'checked' : ''}><span class="${P}-runner-label">${esc(t('missions.form_runner_remote_label', 'Remote Egg'))}</span><span class="${P}-runner-desc">${esc(t('missions.form_runner_remote_desc', 'Run on Invasion egg'))}</span></label>
                            </div>
                            <div class="${P}-form-group" data-mc="remote-target-group" style="display:${source && source.runner_type === 'remote' ? '' : 'none'}">
                                <label>${esc(t('missions.form_remote_target_label', 'Remote Egg'))}</label>
                                <select class="${P}-form-select" data-mc="form-remote-target"><option value="">${esc(t('missions.form_remote_target_loading', 'Loading...'))}</option></select>
                                <div class="${P}-form-hint">${esc(t('missions.form_remote_target_hint', 'Only connected eggs.'))}</div>
                            </div>
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_priority_label', 'Priority'))}</label>
                            <select class="${P}-form-select" data-mc="form-priority">
                                <option value="low">${esc(t('missions.form_priority_low', 'Low'))}</option>
                                <option value="medium" selected>${esc(t('missions.form_priority_medium', 'Medium'))}</option>
                                <option value="high">${esc(t('missions.form_priority_high', 'High'))}</option>
                            </select>
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_exec_type_label', 'Execution Type'))}</label>
                            <div class="${P}-exec-selector">
                                <label class="${P}-exec-option active" data-mc-exec="manual"><input type="radio" name="mc-exec" value="manual" checked><span class="${P}-exec-icon">👆</span><span class="${P}-exec-label">${esc(t('missions.form_exec_manual_label', 'Manual'))}</span><span class="${P}-exec-desc">${esc(t('missions.form_exec_manual_desc', 'On demand'))}</span></label>
                                <label class="${P}-exec-option" data-mc-exec="scheduled"><input type="radio" name="mc-exec" value="scheduled"><span class="${P}-exec-icon">📅</span><span class="${P}-exec-label">${esc(t('missions.form_exec_scheduled_label', 'Scheduled'))}</span><span class="${P}-exec-desc">${esc(t('missions.form_exec_scheduled_desc', 'Cron'))}</span></label>
                                <label class="${P}-exec-option" data-mc-exec="triggered"><input type="radio" name="mc-exec" value="triggered"><span class="${P}-exec-icon">⚡</span><span class="${P}-exec-label">${esc(t('missions.form_exec_triggered_label', 'Triggered'))}</span><span class="${P}-exec-desc">${esc(t('missions.form_exec_triggered_desc', 'Event'))}</span></label>
                            </div>
                        </div>

                        <div class="${P}-form-group" data-mc="config-scheduled" style="display:none">
                            <label>${esc(t('missions.form_cron_preset_label', 'Presets'))}</label>
                            <select class="${P}-form-select" data-mc="cron-preset">${CRON_PRESETS.map(p => `<option value="${escAttr(p.value)}">${esc(t(p.labelKey, p.value || '-- Custom --'))}</option>`).join('')}</select>
                            <div style="margin-top:6px">
                                <label>${esc(t('missions.form_cron_label', 'Cron Expression'))}</label>
                                <input type="text" class="${P}-form-input" data-mc="form-cron" placeholder="${esc(t('missions.form_cron_placeholder', '0 9 * * *'))}">
                                <div class="${P}-form-hint">${esc(t('missions.form_cron_hint', 'Format: Min Hour Day Month Weekday'))}</div>
                            </div>
                        </div>

                        <div class="${P}-form-group" data-mc="config-triggered" style="display:none">
                            <label>${esc(t('missions.form_exec_triggered_label', 'Trigger Type'))}</label>
                            <div class="${P}-trigger-grid">${TRIGGER_TYPES.map(tr => `<button type="button" class="${P}-trigger-btn" data-mc-trigger="${tr.key}">${tr.icon} ${esc(t(tr.labelKey, tr.key))}</button>`).join('')}</div>
                            <div class="${P}-form-group">
                                <label>${esc(t('missions.trigger_min_interval_label', 'Min interval'))}</label>
                                <input type="number" class="${P}-form-input" data-mc="form-min-interval" min="0" max="86400" value="0" placeholder="${esc(t('missions.trigger_min_interval_placeholder', 'e.g. 60'))}">
                                <div class="${P}-form-hint">${esc(t('missions.trigger_min_interval_hint', 'Seconds before re-trigger.'))}</div>
                            </div>
                            ${buildTriggerFields()}
                        </div>

                        <div class="${P}-toggle-row">
                            <label class="${P}-toggle"><input type="checkbox" data-mc="form-locked"><span class="${P}-toggle-slider"></span></label>
                            <div><div class="${P}-toggle-text">${esc(t('missions.form_lock_label', 'Lock'))}</div><div class="${P}-toggle-hint">${esc(t('missions.form_lock_hint', 'Prevents deletion'))}</div></div>
                        </div>
                        <div class="${P}-toggle-row">
                            <label class="${P}-toggle"><input type="checkbox" data-mc="form-auto-prepare"><span class="${P}-toggle-slider"></span></label>
                            <div><div class="${P}-toggle-text">${esc(t('missions.prep_auto', 'Auto-prepare'))}</div><div class="${P}-toggle-hint">${esc(t('missions.prep_auto_hint', 'Prepare before runs'))}</div></div>
                        </div>

                        <div class="${P}-form-group">
                            <label>${esc(t('missions.form_cheatsheets_label', 'Cheat Sheets'))}</label>
                            <div class="${P}-cheatsheet-picker" data-mc="cheatsheet-picker"><div class="${P}-cheatsheet-empty">${esc(t('missions.form_cheatsheets_loading', 'Loading...'))}</div></div>
                            <div class="${P}-form-hint">${esc(t('missions.form_cheatsheets_hint', 'Include as context.'))}</div>
                        </div>
                    </div>
                    <div class="${P}-modal-actions">
                        <button type="button" class="${P}-btn" data-mc="modal-cancel">${esc(t('missions.modal_btn_cancel', 'Cancel'))}</button>
                        <button type="button" class="${P}-btn ${P}-btn-primary" data-mc="modal-save">${esc(t('missions.modal_btn_save', 'Save'))}</button>
                    </div>
                </div>`;
        }

        function buildTriggerFields() {
            return `
                <div class="${P}-trigger-fields" data-mc-trigger-fields="mission_completed">
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_source_mission_label', 'Source Mission'))}</label><div class="${P}-mission-selector" data-mc="mission-selector"></div></div>
                    <div class="${P}-toggle-row"><label class="${P}-toggle"><input type="checkbox" data-mc="form-require-success"><span class="${P}-toggle-slider"></span></label><span class="${P}-toggle-text">${esc(t('missions.trigger_require_success', 'Only on success'))}</span></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="email_received">
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_email_folder_label', 'Folder'))}</label><select class="${P}-form-select" data-mc="form-email-folder"><option value="INBOX">${esc(t('missions.trigger_email_folder_inbox', 'Inbox'))}</option><option value="Sent">${esc(t('missions.trigger_email_folder_sent', 'Sent'))}</option></select></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_email_subject_label', 'Subject'))}</label><input type="text" class="${P}-form-input" data-mc="form-email-subject" placeholder="${esc(t('missions.trigger_email_subject_placeholder', 'Order'))}"></div></div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_email_from_label', 'From'))}</label><input type="text" class="${P}-form-input" data-mc="form-email-from" placeholder="${esc(t('missions.trigger_email_from_placeholder', '@company.com'))}"></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="webhook">
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_webhook_label', 'Webhook'))}</label><select class="${P}-form-select" data-mc="form-webhook"><option value="">${esc(t('missions.trigger_webhook_loading', 'Loading...'))}</option></select><div class="${P}-form-hint">${esc(t('missions.trigger_webhook_hint', 'Choose webhook.'))}</div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="egg_hatched">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_egg_hatched_hint', 'When an egg hatches.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_egg_select_label', 'Egg'))}</label><select class="${P}-form-select" data-mc="form-egg"><option value="">${esc(t('missions.trigger_egg_any', 'Any'))}</option></select></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_nest_select_label', 'Nest'))}</label><select class="${P}-form-select" data-mc="form-egg-nest"><option value="">${esc(t('missions.trigger_nest_any', 'Any'))}</option></select></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="nest_cleared">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_nest_cleared_hint', 'When a nest is cleared.'))}</div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_nest_select_label', 'Nest'))}</label><select class="${P}-form-select" data-mc="form-nest"><option value="">${esc(t('missions.trigger_nest_any', 'Any'))}</option></select></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="mqtt_message">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_mqtt_hint', 'MQTT messages.'))}</div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_mqtt_topic_label', 'Topic'))}</label><input type="text" class="${P}-form-input" data-mc="form-mqtt-topic" placeholder="${esc(t('missions.trigger_mqtt_topic_placeholder', 'home/sensors/#'))}"></div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_mqtt_payload_label', 'Payload'))}</label><input type="text" class="${P}-form-input" data-mc="form-mqtt-payload" placeholder="${esc(t('missions.trigger_mqtt_payload_placeholder', 'alarm'))}"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_mqtt_min_interval_label', 'Min interval'))}</label><input type="number" class="${P}-form-input" data-mc="form-mqtt-interval" min="0" value="0"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="system_startup"><div class="${P}-form-hint">${esc(t('missions.trigger_system_startup_hint', 'Runs on startup.'))}</div></div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="home_assistant_state">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_home_assistant_state_hint', 'HA entity state change.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_ha_entity_id_label', 'Entity ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-ha-entity" placeholder="${esc(t('missions.trigger_ha_entity_id_placeholder', 'binary_sensor.door'))}"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_ha_state_equals_label', 'State equals'))}</label><input type="text" class="${P}-form-input" data-mc="form-ha-state" placeholder="${esc(t('missions.trigger_ha_state_equals_placeholder', 'on'))}"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="device_connected">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_device_connected_hint', 'Device connects.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_device_id_label', 'Device ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-device-conn-id"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_device_name_label', 'Name'))}</label><input type="text" class="${P}-form-input" data-mc="form-device-conn-name"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="device_disconnected">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_device_disconnected_hint', 'Device disconnects.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_device_id_label', 'Device ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-device-disc-id"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_device_name_label', 'Name'))}</label><input type="text" class="${P}-form-input" data-mc="form-device-disc-name"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="fritzbox_call">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_fritzbox_call_hint', 'Fritz!Box calls.'))}</div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_fritzbox_call_type_label', 'Type'))}</label><select class="${P}-form-select" data-mc="form-fritzbox-type"><option value="">${esc(t('missions.trigger_fritzbox_call_type_any', 'Any'))}</option><option value="call">${esc(t('missions.trigger_fritzbox_call_type_call', 'Call'))}</option><option value="tam_message">${esc(t('missions.trigger_fritzbox_call_type_tam', 'TAM'))}</option></select></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="budget_warning"><div class="${P}-form-hint">${esc(t('missions.trigger_budget_warning_hint', 'Budget warning.'))}</div></div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="budget_exceeded"><div class="${P}-form-hint">${esc(t('missions.trigger_budget_exceeded_hint', 'Budget exceeded.'))}</div></div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="planner_appointment_due">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_planner_appointment_due_hint', 'Appointment due.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_appointment_id_label', 'ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-appt-id"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_title_contains_label', 'Title'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-appt-title"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="planner_todo_overdue">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_planner_todo_overdue_hint', 'Todo overdue.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_todo_id_label', 'ID'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-todo-id"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_title_contains_label', 'Title'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-todo-title"></div></div>
                </div>
                <div class="${P}-trigger-fields" data-mc-trigger-fields="planner_operational_issue">
                    <div class="${P}-form-hint">${esc(t('missions.trigger_planner_operational_issue_hint', 'Operational issue.'))}</div>
                    <div class="${P}-form-row"><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_issue_source_label', 'Source'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-issue-source" placeholder="${esc(t('missions.trigger_planner_issue_source_placeholder', 'mission'))}"></div><div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_issue_severity_label', 'Severity'))}</label><select class="${P}-form-select" data-mc="form-planner-issue-severity"><option value="">${esc(t('missions.trigger_planner_issue_severity_any', 'Any'))}</option><option value="warning">${esc(t('missions.trigger_planner_issue_severity_warning', 'Warning'))}</option><option value="error">${esc(t('missions.trigger_planner_issue_severity_error', 'Error'))}</option></select></div></div>
                    <div class="${P}-form-group"><label>${esc(t('missions.trigger_planner_title_contains_label', 'Title'))}</label><input type="text" class="${P}-form-input" data-mc="form-planner-issue-title"></div>
                </div>
            `;
        }

        function bindModalEvents(overlay, source) {
            // Exec type
            overlay.querySelectorAll('[data-mc-exec]').forEach(el => {
                el.addEventListener('click', () => {
                    overlay.querySelectorAll('[data-mc-exec]').forEach(e => e.classList.remove('active'));
                    el.classList.add('active');
                    el.querySelector('input').checked = true;
                    const v = el.dataset.mcExec;
                    const schedEl = overlay.querySelector('[data-mc="config-scheduled"]');
                    const trigEl = overlay.querySelector('[data-mc="config-triggered"]');
                    if (schedEl) schedEl.style.display = v === 'scheduled' ? '' : 'none';
                    if (trigEl) trigEl.style.display = v === 'triggered' ? '' : 'none';
                });
            });

            // Runner type
            overlay.querySelectorAll('[data-mc-runner]').forEach(el => {
                el.addEventListener('click', () => {
                    overlay.querySelectorAll('[data-mc-runner]').forEach(e => e.classList.remove('active'));
                    el.classList.add('active');
                    el.querySelector('input').checked = true;
                    const rg = overlay.querySelector('[data-mc="remote-target-group"]');
                    if (rg) rg.style.display = el.dataset.mcRunner === 'remote' ? '' : 'none';
                });
            });

            // Cron preset
            const cronPreset = overlay.querySelector('[data-mc="cron-preset"]');
            const cronInput = overlay.querySelector('[data-mc="form-cron"]');
            if (cronPreset && cronInput) {
                cronPreset.addEventListener('change', () => { if (cronPreset.value) cronInput.value = cronPreset.value; });
            }

            // Trigger type
            overlay.querySelectorAll('[data-mc-trigger]').forEach(btn => {
                btn.addEventListener('click', () => {
                    overlay.querySelectorAll('[data-mc-trigger]').forEach(b => b.classList.remove('active'));
                    btn.classList.add('active');
                    overlay.querySelectorAll('[data-mc-trigger-fields]').forEach(f => f.classList.remove('active'));
                    const panel = overlay.querySelector(`[data-mc-trigger-fields="${btn.dataset.mcTrigger}"]`);
                    if (panel) panel.classList.add('active');
                });
            });

            // Load data for selectors
            loadWebhooksForModal(overlay);
            loadInvasionDataForModal(overlay);
            loadRemoteTargetsForModal(overlay, source);
            loadCheatsheetsForModal(overlay, source ? (source.cheatsheet_ids || []) : []);
            loadMissionSelectorForModal(overlay);
        }

        async function loadWebhooksForModal(overlay) {
            try {
                const data = await api('/api/webhooks');
                state.webhooks = Array.isArray(data) ? data : [];
                const sel = overlay.querySelector('[data-mc="form-webhook"]');
                if (sel) {
                    sel.innerHTML = state.webhooks.length === 0
                        ? '<option value="">' + esc(t('missions.trigger_webhook_none', 'None')) + '</option>'
                        : state.webhooks.map(w => '<option value="' + escAttr(w.id) + '" data-slug="' + escAttr(w.slug) + '">' + esc(w.name) + ' (' + esc(w.slug) + ')</option>').join('');
                }
            } catch (_) { /* webhooks unavailable */ }
        }

        async function loadInvasionDataForModal(overlay) {
            try {
                const [eggsResp, nestsResp] = await Promise.all([api('/api/invasion/eggs').catch(() => null), api('/api/invasion/nests').catch(() => null)]);
                const eggs = eggsResp ? (eggsResp.eggs || eggsResp || []) : [];
                const nests = nestsResp ? (nestsResp.nests || nestsResp || []) : [];
                const eggOpts = '<option value="">' + esc(t('missions.trigger_egg_any', 'Any')) + '</option>' + eggs.map(e => '<option value="' + escAttr(e.id) + '" data-name="' + escAttr(e.name) + '">' + esc(e.name) + '</option>').join('');
                const nestOpts = '<option value="">' + esc(t('missions.trigger_nest_any', 'Any')) + '</option>' + nests.map(n => '<option value="' + escAttr(n.id) + '" data-name="' + escAttr(n.name) + '">' + esc(n.name) + '</option>').join('');
                const eggSel = overlay.querySelector('[data-mc="form-egg"]');
                const eggNestSel = overlay.querySelector('[data-mc="form-egg-nest"]');
                const nestSel = overlay.querySelector('[data-mc="form-nest"]');
                if (eggSel) eggSel.innerHTML = eggOpts;
                if (eggNestSel) eggNestSel.innerHTML = nestOpts;
                if (nestSel) nestSel.innerHTML = nestOpts;
            } catch (_) { /* invasion not available */ }
        }

        async function loadRemoteTargetsForModal(overlay, source) {
            const sel = overlay.querySelector('[data-mc="form-remote-target"]');
            if (!sel) return;
            try {
                const data = await api('/api/missions/v2/remote-targets');
                state.remoteTargets = data.targets || [];
                if (state.remoteTargets.length === 0) { sel.innerHTML = '<option value="">' + esc(t('missions.form_remote_target_none', 'None')) + '</option>'; return; }
                sel.innerHTML = '<option value="">' + esc(t('missions.form_remote_target_placeholder', 'Select...')) + '</option>' + state.remoteTargets.map(tgt => '<option value="' + escAttr(tgt.nest_id) + '" data-egg-id="' + escAttr(tgt.egg_id) + '" data-nest-name="' + escAttr(tgt.nest_name || '') + '" data-egg-name="' + escAttr(tgt.egg_name || '') + '">' + esc((tgt.nest_name || tgt.nest_id) + ' · ' + (tgt.egg_name || tgt.egg_id)) + '</option>').join('');
                if (source && source.remote_nest_id) sel.value = source.remote_nest_id;
            } catch (_) { sel.innerHTML = '<option value="">' + esc(t('missions.form_remote_target_unavailable', 'Unavailable')) + '</option>'; }
        }

        async function loadCheatsheetsForModal(overlay, selectedIds) {
            const picker = overlay.querySelector('[data-mc="cheatsheet-picker"]');
            if (!picker) return;
            try {
                const sheets = await api('/api/cheatsheets?active=true&created_by=user');
                if (!sheets || sheets.length === 0) { picker.innerHTML = '<div class="' + P + '-cheatsheet-empty">' + esc(t('missions.form_cheatsheets_none', 'None')) + '</div>'; return; }
                picker.innerHTML = sheets.map(s => {
                    const checked = selectedIds.includes(s.id) ? 'checked' : '';
                    const abstract = s.abstract ? '<div class="' + P + '-cheatsheet-preview">' + esc(s.abstract) + '</div>' : '';
                    return '<div class="' + P + '-cheatsheet-item"><input type="checkbox" id="mc-cs-' + s.id + '" value="' + s.id + '" ' + checked + '><label for="mc-cs-' + s.id + '">' + esc(s.name) + abstract + '</label></div>';
                }).join('');
            } catch (_) { picker.innerHTML = '<div class="' + P + '-cheatsheet-empty">' + esc(t('missions.form_cheatsheets_none', 'None')) + '</div>'; }
        }

        async function loadMissionSelectorForModal(overlay) {
            const el = overlay.querySelector('[data-mc="mission-selector"]');
            if (!el) return;
            const manual = state.missions.filter(m => m.execution_type === 'manual' || m.execution_type === 'scheduled');
            if (manual.length === 0) { el.innerHTML = '<div class="' + P + '-cheatsheet-empty">' + esc(t('missions.trigger_no_suitable_missions', 'None')) + '</div>'; return; }
            el.innerHTML = manual.map(m => '<label class="' + P + '-mission-option"><input type="radio" name="mc-source-mission" value="' + m.id + '" data-name="' + escAttr(m.name) + '"><div><div class="' + P + '-mission-option-name">' + esc(m.name) + '</div><div class="' + P + '-mission-option-meta">' + m.execution_type + ' · ' + m.priority + '</div></div></label>').join('');
        }

        function fillFormFromMission(overlay, mission) {
            const q = (sel) => overlay.querySelector(sel);
            const v = (sel, val) => { const el = q(sel); if (el) el.value = val || ''; };
            const c = (sel, val) => { const el = q(sel); if (el) el.checked = !!val; };

            v('[data-mc="form-priority"]', mission.priority);
            c('[data-mc="form-locked"]', mission.locked);
            c('[data-mc="form-auto-prepare"]', mission.auto_prepare);

            // Exec type
            const execBtn = overlay.querySelector(`[data-mc-exec="${mission.execution_type}"]`);
            if (execBtn) execBtn.click();

            if (mission.execution_type === 'scheduled') {
                v('[data-mc="form-cron"]', mission.schedule);
                const preset = overlay.querySelector('[data-mc="cron-preset"]');
                if (preset) { const match = Array.from(preset.options).find(o => o.value === mission.schedule); preset.value = match ? mission.schedule : ''; }
            } else if (mission.execution_type === 'triggered' && mission.trigger_type) {
                const trigBtn = overlay.querySelector(`[data-mc-trigger="${mission.trigger_type}"]`);
                if (trigBtn) trigBtn.click();
                fillTriggerConfig(overlay, mission.trigger_config, mission.trigger_type);
            }

            // Runner
            if (mission.runner_type === 'remote') {
                const rBtn = overlay.querySelector('[data-mc-runner="remote"]');
                if (rBtn) rBtn.click();
            }
        }

        function fillTriggerConfig(overlay, cfg, type) {
            if (!cfg) return;
            const q = (sel) => overlay.querySelector(sel);
            const v = (sel, val) => { const el = q(sel); if (el) el.value = val || ''; };
            const c = (sel, val) => { const el = q(sel); if (el) el.checked = !!val; };

            v('[data-mc="form-min-interval"]', cfg.min_interval_seconds || 0);

            switch (type) {
                case 'mission_completed':
                    if (cfg.source_mission_id) { const r = overlay.querySelector('input[name="mc-source-mission"][value="' + cfg.source_mission_id + '"]'); if (r) { r.checked = true; r.closest('.' + P + '-mission-option')?.classList.add('selected'); } }
                    c('[data-mc="form-require-success"]', cfg.require_success); break;
                case 'email_received': v('[data-mc="form-email-folder"]', cfg.email_folder); v('[data-mc="form-email-subject"]', cfg.email_subject_contains); v('[data-mc="form-email-from"]', cfg.email_from_contains); break;
                case 'webhook': v('[data-mc="form-webhook"]', cfg.webhook_id); break;
                case 'egg_hatched': v('[data-mc="form-egg"]', cfg.egg_id); v('[data-mc="form-egg-nest"]', cfg.nest_id); break;
                case 'nest_cleared': v('[data-mc="form-nest"]', cfg.nest_id); break;
                case 'mqtt_message': v('[data-mc="form-mqtt-topic"]', cfg.mqtt_topic); v('[data-mc="form-mqtt-payload"]', cfg.mqtt_payload_contains); v('[data-mc="form-mqtt-interval"]', cfg.mqtt_min_interval_seconds); break;
                case 'home_assistant_state': v('[data-mc="form-ha-entity"]', cfg.ha_entity_id); v('[data-mc="form-ha-state"]', cfg.ha_state_equals); break;
                case 'device_connected': v('[data-mc="form-device-conn-id"]', cfg.device_id); v('[data-mc="form-device-conn-name"]', cfg.device_name); break;
                case 'device_disconnected': v('[data-mc="form-device-disc-id"]', cfg.device_id); v('[data-mc="form-device-disc-name"]', cfg.device_name); break;
                case 'fritzbox_call': v('[data-mc="form-fritzbox-type"]', cfg.call_type); break;
                case 'planner_appointment_due': v('[data-mc="form-planner-appt-id"]', cfg.planner_appointment_id); v('[data-mc="form-planner-appt-title"]', cfg.planner_title_contains); break;
                case 'planner_todo_overdue': v('[data-mc="form-planner-todo-id"]', cfg.planner_todo_id); v('[data-mc="form-planner-todo-title"]', cfg.planner_title_contains); break;
                case 'planner_operational_issue': v('[data-mc="form-planner-issue-source"]', cfg.planner_issue_source); v('[data-mc="form-planner-issue-severity"]', cfg.planner_issue_severity); v('[data-mc="form-planner-issue-title"]', cfg.planner_title_contains); break;
            }
        }

        async function saveMission(overlay, closeModalFn) {
            const q = (sel) => overlay.querySelector(sel);
            const name = (q('[data-mc="form-name"]')?.value || '').trim();
            const prompt = (q('[data-mc="form-prompt"]')?.value || '').trim();
            if (!name || !prompt) { notify(t('missions.toast_name_prompt_required', 'Name and prompt required'), 'error'); return; }

            const execType = overlay.querySelector('input[name="mc-exec"]:checked')?.value || 'manual';
            const runnerType = overlay.querySelector('input[name="mc-runner"]:checked')?.value || 'local';

            const mission = {
                name, prompt,
                priority: q('[data-mc="form-priority"]')?.value || 'medium',
                execution_type: execType,
                runner_type: runnerType,
                enabled: true,
                locked: q('[data-mc="form-locked"]')?.checked || false,
                auto_prepare: q('[data-mc="form-auto-prepare"]')?.checked || false,
                cheatsheet_ids: Array.from(overlay.querySelectorAll('[data-mc="cheatsheet-picker"] input[type="checkbox"]:checked')).map(c => c.value)
            };

            if (runnerType === 'remote') {
                const rSel = q('[data-mc="form-remote-target"]');
                const opt = rSel?.options[rSel.selectedIndex];
                if (!rSel?.value || !opt?.dataset?.eggId) { notify(t('missions.toast_select_remote_target', 'Select remote target'), 'error'); return; }
                mission.remote_nest_id = rSel.value;
                mission.remote_nest_name = opt.dataset.nestName || '';
                mission.remote_egg_id = opt.dataset.eggId;
                mission.remote_egg_name = opt.dataset.eggName || '';
            }

            if (execType === 'scheduled') {
                mission.schedule = q('[data-mc="form-cron"]')?.value || '';
                mission.trigger_type = '';
                mission.trigger_config = null;
            } else if (execType === 'triggered') {
                const trigBtn = overlay.querySelector('[data-mc-trigger].active');
                if (!trigBtn) { notify(t('missions.toast_select_trigger_type', 'Select trigger'), 'error'); return; }
                mission.trigger_type = trigBtn.dataset.mcTrigger;
                mission.trigger_config = buildTriggerConfig(overlay, mission.trigger_type);
                mission.schedule = '';
            } else {
                mission.schedule = '';
                mission.trigger_type = '';
                mission.trigger_config = null;
            }

            try {
                const editId = q('[data-mc="form-id"]')?.value;
                const url = editId ? '/api/missions/v2/' + editId : '/api/missions/v2';
                const method = editId ? 'PUT' : 'POST';
                await api(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(mission) });
                notify(editId ? t('missions.toast_mission_updated', 'Updated') : t('missions.toast_mission_created', 'Created'));
                closeModalFn();
                loadData();
            } catch (err) { notify(t('missions.toast_error_prefix', 'Error: ') + err.message, 'error'); }
        }

        function buildTriggerConfig(overlay, type) {
            const q = (sel) => overlay.querySelector(sel);
            const config = { min_interval_seconds: parseInt(q('[data-mc="form-min-interval"]')?.value || '0', 10) || 0 };
            switch (type) {
                case 'mission_completed': {
                    const sel = overlay.querySelector('input[name="mc-source-mission"]:checked');
                    if (sel) { config.source_mission_id = sel.value; config.source_mission_name = sel.dataset.name; }
                    config.require_success = q('[data-mc="form-require-success"]')?.checked || false;
                    break;
                }
                case 'email_received': config.email_folder = q('[data-mc="form-email-folder"]')?.value || ''; config.email_subject_contains = q('[data-mc="form-email-subject"]')?.value || ''; config.email_from_contains = q('[data-mc="form-email-from"]')?.value || ''; break;
                case 'webhook': { const ws = q('[data-mc="form-webhook"]'); config.webhook_id = ws?.value || ''; config.webhook_slug = ws?.options[ws.selectedIndex]?.dataset?.slug || ''; break; }
                case 'egg_hatched': { const es = q('[data-mc="form-egg"]'); const ns = q('[data-mc="form-egg-nest"]'); config.egg_id = es?.value || ''; config.egg_name = es?.options[es.selectedIndex]?.dataset?.name || ''; config.nest_id = ns?.value || ''; config.nest_name = ns?.options[ns.selectedIndex]?.dataset?.name || ''; break; }
                case 'nest_cleared': { const ns = q('[data-mc="form-nest"]'); config.nest_id = ns?.value || ''; config.nest_name = ns?.options[ns.selectedIndex]?.dataset?.name || ''; break; }
                case 'mqtt_message': config.mqtt_topic = (q('[data-mc="form-mqtt-topic"]')?.value || '').trim(); config.mqtt_payload_contains = (q('[data-mc="form-mqtt-payload"]')?.value || '').trim(); config.mqtt_min_interval_seconds = parseInt(q('[data-mc="form-mqtt-interval"]')?.value || '0', 10) || 0; break;
                case 'home_assistant_state': config.ha_entity_id = (q('[data-mc="form-ha-entity"]')?.value || '').trim(); config.ha_state_equals = (q('[data-mc="form-ha-state"]')?.value || '').trim(); break;
                case 'device_connected': config.device_id = (q('[data-mc="form-device-conn-id"]')?.value || '').trim(); config.device_name = (q('[data-mc="form-device-conn-name"]')?.value || '').trim(); break;
                case 'device_disconnected': config.device_id = (q('[data-mc="form-device-disc-id"]')?.value || '').trim(); config.device_name = (q('[data-mc="form-device-disc-name"]')?.value || '').trim(); break;
                case 'fritzbox_call': config.call_type = q('[data-mc="form-fritzbox-type"]')?.value || ''; break;
                case 'planner_appointment_due': config.planner_appointment_id = (q('[data-mc="form-planner-appt-id"]')?.value || '').trim(); config.planner_title_contains = (q('[data-mc="form-planner-appt-title"]')?.value || '').trim(); break;
                case 'planner_todo_overdue': config.planner_todo_id = (q('[data-mc="form-planner-todo-id"]')?.value || '').trim(); config.planner_title_contains = (q('[data-mc="form-planner-todo-title"]')?.value || '').trim(); break;
                case 'planner_operational_issue': config.planner_issue_source = (q('[data-mc="form-planner-issue-source"]')?.value || '').trim(); config.planner_issue_severity = q('[data-mc="form-planner-issue-severity"]')?.value || ''; config.planner_title_contains = (q('[data-mc="form-planner-issue-title"]')?.value || '').trim(); break;
            }
            return config;
        }

        function showInfoModal(title, body) {
            const overlay = document.createElement('div');
            overlay.className = P + '-modal-overlay';
            overlay.innerHTML = `<div class="${P}-modal" style="max-width:600px"><div class="${P}-modal-header"><h2 class="${P}-modal-title">${esc(title)}</h2><button type="button" class="${P}-modal-close" data-mc="info-close">${SVG.x}</button></div><div class="${P}-modal-body"><pre style="white-space:pre-wrap;font-size:11px;color:var(--vd-text-muted,#9aa3ad);line-height:1.5;max-height:400px;overflow-y:auto;margin:0">${esc(body)}</pre></div></div>`;
            container.appendChild(overlay);
            const close = () => { if (overlay.parentNode) overlay.parentNode.removeChild(overlay); };
            overlay.addEventListener('click', (e) => { if (e.target === overlay) close(); });
            overlay.querySelector('[data-mc="info-close"]').addEventListener('click', close);
        }

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
