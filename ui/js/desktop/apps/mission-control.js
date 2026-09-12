// Mission Control - Virtual Desktop app shell (master-detail workbench).
// Composes MissionControlSchedule/Triggers/Menus/List/Detail/Editor.
(function () {
    'use strict';

    const instances = new Map();
    const PREFS_KEY = 'aurago.desktop.mission-control.prefs';
    const COMPACT_BREAKPOINT = 720;
    const LIST_MIN = 260, LIST_MAX = 460, LIST_DEFAULT = 320;
    const HISTORY_PAGE = 25;
    const FILTERS = ['all', 'manual', 'scheduled', 'triggered', 'errors'];
    const SORTS = ['name', 'last_run', 'next_run', 'priority'];

    function clamp(n, lo, hi) { n = Number(n); if (Number.isNaN(n)) return lo; return Math.min(hi, Math.max(lo, n)); }

    function loadPrefs() {
        let raw = {};
        try { raw = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}') || {}; } catch (_) { raw = {}; }
        return {
            filter: FILTERS.includes(raw.filter) ? raw.filter : 'all',
            sort: SORTS.includes(raw.sort) ? raw.sort : 'name',
            listWidth: clamp(raw.listWidth || LIST_DEFAULT, LIST_MIN, LIST_MAX),
            listCollapsed: !!raw.listCollapsed
        };
    }

    function savePrefs(state) {
        try { localStorage.setItem(PREFS_KEY, JSON.stringify({ filter: state.filter, sort: state.sort, listWidth: state.listWidth, listCollapsed: state.listCollapsed })); } catch (_) { /* storage unavailable */ }
    }

    function render(container, windowId, context) {
        dispose(windowId);

        const { esc, t, api, notify, readonly, setWindowMenus, clearWindowMenus, wireContextMenuBoundary, confirmDialog, showContextMenu, setWindowBeforeClose, isActive } = context;
        const S = window.MissionControlSchedule, TR = window.MissionControlTriggers, MN = window.MissionControlMenus;
        if (!S || !TR || !MN || !window.MissionControlList || !window.MissionControlDetail || !window.MissionControlEditor) {
            container.innerHTML = `<div class="vd-mc vd-mc--fatal">${esc(t('desktop.mc_load_error'))}</div>`;
            return;
        }
        const lang = String(window.SYSTEM_LANG || document.documentElement.lang || 'en');
        const svg = MN.ICONS;
        const prefs = loadPrefs();

        const state = {
            missions: [], queue: { items: [], running: '' },
            selectedId: '', filter: prefs.filter, sort: prefs.sort, query: '', tab: 'overview',
            listWidth: prefs.listWidth, listCollapsed: prefs.listCollapsed, compact: false, compactView: 'list',
            editing: null, busy: '', cancelling: new Set(), runStarted: new Map(), preparedOpen: false,
            history: { missionId: '', items: [], total: 0, loading: false, error: '', filter: 'all' },
            initialLoad: false, live: false, disposed: false,
            sseHandler: null, keydownHandler: null, timer: null, resizeObserver: null, menuSignature: ''
        };
        instances.set(windowId, state);

        // ── formatting helpers shared with list/detail ──
        const fmt = {
            relative(iso) {
                const ts = Date.parse(iso || '');
                if (!ts) return '';
                const diff = ts - Date.now();
                const abs = Math.abs(diff);
                if (abs < 45000) return t('desktop.mc_time_now');
                const value = abs < 3600000 ? t('desktop.rel_time_minutes', { count: Math.max(1, Math.round(abs / 60000)) })
                    : abs < 86400000 ? t('desktop.rel_time_hours', { count: Math.round(abs / 3600000) })
                        : t('desktop.rel_time_days', { count: Math.round(abs / 86400000) });
                return diff > 0 ? t('desktop.mc_time_in', { value }) : t('desktop.mc_time_ago', { value });
            },
            dateTime(iso) {
                const d = new Date(iso);
                if (Number.isNaN(d.getTime())) return '';
                try { return d.toLocaleString(lang, { dateStyle: 'medium', timeStyle: 'short' }); } catch (_) { return d.toLocaleString(); }
            },
            duration(ms) {
                const s = Math.max(1, Math.round((Number(ms) || 0) / 1000));
                if (s < 60) return t('desktop.rel_time_seconds', { count: s });
                const m = Math.floor(s / 60);
                if (m < 60) return `${t('desktop.rel_time_minutes', { count: m })} ${t('desktop.rel_time_seconds', { count: s % 60 })}`;
                return `${t('desktop.rel_time_hours', { count: Math.floor(m / 60) })} ${t('desktop.rel_time_minutes', { count: m % 60 })}`;
            },
            clock(ms) {
                const s = Math.max(0, Math.floor(ms / 1000));
                const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), sec = s % 60;
                return (h ? `${h}:` : '') + `${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`;
            }
        };

        // ── layout ──
        container.innerHTML = `
        <div class="vd-mc" data-mc-root style="--mc-list-width:${state.listWidth}px">
            <div class="vd-mc-toolbar" role="toolbar" aria-label="Mission Control">
                <button type="button" class="vd-mc-btn vd-mc-btn--icon" data-mc-list-toggle title="${esc(t(state.listCollapsed ? 'desktop.mc_list_expand' : 'desktop.mc_list_collapse'))}" aria-label="${esc(t('desktop.mc_toolbar_list_toggle'))}" aria-pressed="${!state.listCollapsed}">${svg.sidebar}</button>
                <label class="vd-mc-search">${svg.search}<input type="search" data-mc-search placeholder="${esc(t('desktop.mc_search_placeholder'))}" aria-label="${esc(t('desktop.mc_search_placeholder'))}" inputmode="search" enterkeyhint="search" autocomplete="off" spellcheck="false"><button type="button" class="vd-mc-search-clear" data-mc-search-clear hidden aria-label="${esc(t('desktop.mc_list_clear_filter'))}">${svg.x}</button></label>
                <label class="vd-mc-toolbar-select"><span>${esc(t('desktop.mc_filter_label'))}</span><select data-mc-filter aria-label="${esc(t('desktop.mc_filter_label'))}">${FILTERS.map(f => `<option value="${f}">${esc(t('desktop.mc_filter_' + f))}</option>`).join('')}</select></label>
                <label class="vd-mc-toolbar-select"><span>${esc(t('desktop.mc_sort_label'))}</span><select data-mc-sort aria-label="${esc(t('desktop.mc_sort_label'))}">${SORTS.map(s => `<option value="${s}">${esc(t('desktop.mc_sort_' + s))}</option>`).join('')}</select></label>
                <span class="vd-mc-toolbar-spacer"></span>
                <button type="button" class="vd-mc-btn vd-mc-btn--icon" data-mc-refresh title="${esc(t('desktop.mc_toolbar_refresh'))}" aria-label="${esc(t('desktop.mc_toolbar_refresh'))}">${svg.refresh}</button>
                ${readonly ? '' : `<button type="button" class="vd-mc-btn vd-mc-btn--primary" data-mc-new>${svg.plus}<span>${esc(t('desktop.mc_new_mission'))}</span></button>`}
            </div>
            ${readonly ? `<div class="vd-mc-banner" role="status">${svg.info}<span>${esc(t('desktop.mc_readonly_banner'))}</span></div>` : ''}
            <div class="vd-mc-body" data-mc-body>
                <aside class="vd-mc-listpane" data-mc-listpane></aside>
                <div class="vd-mc-splitter" data-mc-splitter role="separator" aria-orientation="vertical" tabindex="0" aria-valuemin="${LIST_MIN}" aria-valuemax="${LIST_MAX}" aria-valuenow="${state.listWidth}" aria-label="${esc(t('desktop.mc_menu_list_panel'))}"></div>
                <main class="vd-mc-main" data-mc-main>
                    <button type="button" class="vd-mc-back" data-mc-back hidden>${svg.chevronLeft}<span>${esc(t('desktop.mc_back_to_list'))}</span></button>
                    <div class="vd-mc-loading" data-mc-loading>${esc(t('desktop.loading'))}</div>
                    <div class="vd-mc-loaderror" data-mc-loaderror hidden>${svg.alert}<span>${esc(t('desktop.mc_load_error'))}</span><span class="vd-mc-muted" data-mc-loaderror-detail></span><button type="button" class="vd-mc-btn" data-mc-retry>${esc(t('desktop.mc_retry'))}</button></div>
                </main>
            </div>
            <footer class="vd-mc-statusbar" role="status" aria-live="polite">
                <span data-mc-status-counts></span>
                <span data-mc-status-next></span>
                <span class="vd-mc-status-live" data-mc-status-live>${esc(t('desktop.mc_status_offline'))}</span>
            </footer>
        </div>`;
        const root = container.querySelector('[data-mc-root]');
        const $ = (sel) => root.querySelector(sel);
        const main = $('[data-mc-main]');

        const list = window.MissionControlList.create({ esc, t, lang, svg, triggers: TR, schedule: S, readonly, fmt });
        const detail = window.MissionControlDetail.create({ esc, t, lang, svg, triggers: TR, schedule: S, readonly, fmt });
        const editor = window.MissionControlEditor.create({ esc, t, lang, svg, request: api, schedule: S, triggers: TR, missions: () => state.missions, readonly });
        $('[data-mc-listpane]').appendChild(list.element);
        main.appendChild(detail.element);
        main.appendChild(editor.element);
        detail.element.hidden = true;
        editor.element.hidden = true;
        $('[data-mc-filter]').value = state.filter;
        $('[data-mc-sort]').value = state.sort;
        list.setFilter(state.filter);
        list.setSort(state.sort);
        root.classList.toggle('is-list-collapsed', state.listCollapsed);

        // ── payload normalizers (kept for TestDesktopMissionControlNormalizesListPayload) ──
        function normalizeMissionControlPayload(data) {
            data = data || {};
            const missions = Array.isArray(data.missions)
                ? data.missions
                : (Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []));
            return { missions, queue: normalizeMissionQueue(data.queue) };
        }
        function normalizeMissionQueue(queue) {
            queue = queue || {};
            return { items: Array.isArray(queue.items) ? queue.items : [], running: typeof queue.running === 'string' ? queue.running : '' };
        }
        function missionIsRunning(mission, queueState = state.queue) {
            if (!mission) return false;
            return mission.id === queueState.running || mission.status === 'running';
        }
        function getRunningMissions(missionsList = state.missions, queueState = state.queue) {
            const seen = new Set();
            const running = [];
            const add = (mission) => { if (!mission || seen.has(mission.id)) return; seen.add(mission.id); running.push(mission); };
            if (queueState.running) add(missionsList.find(m => m.id === queueState.running));
            missionsList.forEach(m => { if (m.status === 'running') add(m); });
            return running;
        }
        function toastForMissionDispatch(data) {
            const status = data && data.status ? data.status : 'queued';
            if (status === 'running') return t('desktop.mc_toast_run_started');
            if (status === 'skipped') return t('desktop.mc_toast_run_skipped');
            return t('desktop.mc_toast_run_queued');
        }
        const selected = () => state.missions.find(m => m.id === state.selectedId) || null;
        const byId = (id) => state.missions.find(m => m.id === id) || null;
        const queuePosition = (id) => { const idx = state.queue.items.findIndex(item => item.mission_id === id); return idx < 0 ? 0 : idx + 1; };
        const failToast = (err) => notify(t('desktop.mc_toast_action_failed', { error: (err && err.message) || String(err) }), 'error');

        // ── data ──
        async function loadData() {
            try {
                const data = await api('/api/missions/v2');
                if (state.disposed) return;
                applyData(data);
                state.initialLoad = true;
                $('[data-mc-loading]').hidden = true;
                $('[data-mc-loaderror]').hidden = true;
                if (!state.editing) detail.element.hidden = false;
                list.setData({ missions: state.missions, queue: state.queue });
                if (!state.selectedId && !state.compact && list.visibleIds().length) selectMission(list.visibleIds()[0], false);
                syncAll();
            } catch (err) {
                if (state.disposed) return;
                console.error('MC: load failed', err);
                if (!state.initialLoad) {
                    $('[data-mc-loading]').hidden = true;
                    $('[data-mc-loaderror]').hidden = false;
                    $('[data-mc-loaderror-detail]').textContent = err && err.message ? err.message : '';
                } else failToast(err);
            }
        }

        function applyData(data) {
            const normalized = normalizeMissionControlPayload(data);
            const before = new Set(getRunningMissions().map(m => m.id));
            state.missions = normalized.missions;
            state.queue = normalized.queue;
            const after = new Set(getRunningMissions().map(m => m.id));
            after.forEach(id => { if (!state.runStarted.has(id)) state.runStarted.set(id, Date.now()); });
            before.forEach(id => {
                if (after.has(id)) return;
                state.runStarted.delete(id);
                state.cancelling.delete(id);
                if (state.history.missionId === id) loadHistory(true);
            });
            if (state.selectedId && !byId(state.selectedId)) {
                state.selectedId = '';
                if (state.editing && state.editing.mode === 'edit') closeEditor(true);
            }
        }

        // ── sync ──
        function syncAll() {
            if (state.disposed) return;
            list.setData({ missions: state.missions, queue: state.queue });
            list.setSelected(state.selectedId);
            syncDetail();
            syncStatus();
            syncMenus();
            syncCompact();
        }

        function syncDetail() {
            if (state.editing) return;
            const m = selected();
            if (!m) { detail.showEmpty(); stopTimer(); return; }
            const running = missionIsRunning(m);
            detail.setMission(m, { running, queuePosition: queuePosition(m.id), cancelling: state.cancelling.has(m.id), busy: state.busy });
            detail.setTab(state.tab);
            if (running) startTimer(); else stopTimer();
        }

        function syncStatus() {
            const counts = list.counts();
            const parts = [counts.total === 1 ? t('desktop.mc_status_missions_one') : t('desktop.mc_status_missions', { count: counts.total })];
            if (counts.shown !== counts.total) parts.push(t('desktop.mc_status_filtered', { shown: counts.shown, total: counts.total }));
            if (counts.running) parts.push(t('desktop.mc_status_running', { count: counts.running }));
            if (counts.waiting) parts.push(t('desktop.mc_status_waiting', { count: counts.waiting }));
            if (!counts.running && !counts.waiting && counts.total) parts.push(t('desktop.mc_status_idle'));
            $('[data-mc-status-counts]').textContent = parts.join(' · ');
            const next = state.missions.filter(m => m.execution_type === 'scheduled' && m.enabled !== false && m.next_run)
                .sort((a, b) => Date.parse(a.next_run) - Date.parse(b.next_run))[0];
            $('[data-mc-status-next]').textContent = next ? t('desktop.mc_status_next', { name: next.name, when: fmt.relative(next.next_run) }) : '';
            const live = $('[data-mc-status-live]');
            live.textContent = t(state.live ? 'desktop.mc_status_live' : 'desktop.mc_status_offline');
            live.classList.toggle('is-live', state.live);
        }

        const menuModel = { t, readonly, s: {}, actions: {} };
        function syncMenus() {
            if (typeof setWindowMenus !== 'function') return;
            const m = selected();
            const running = !!(m && missionIsRunning(m));
            menuModel.s = {
                selected: m, running, queued: !!(m && queuePosition(m.id)),
                queuedIds: new Set(state.queue.items.map(item => item.mission_id)),
                canCancel: !!(m && running && m.runner_type !== 'remote' && !state.cancelling.has(m.id)),
                filter: state.filter, sort: state.sort, tab: state.tab, listCollapsed: state.listCollapsed, editing: !!state.editing
            };
            const sig = JSON.stringify([m && m.id, m && m.enabled, m && m.locked, m && m.preparation_status, m && m.runner_type, running, menuModel.s.queued, menuModel.s.canCancel, state.filter, state.sort, state.tab, state.listCollapsed, !!state.editing]);
            if (sig === state.menuSignature) return;
            state.menuSignature = sig;
            setWindowMenus(windowId, MN.windowMenus(menuModel));
        }

        function syncCompact() {
            const showDetail = state.compactView === 'detail' && (state.selectedId || state.editing);
            root.classList.toggle('is-compact', state.compact);
            root.classList.toggle('is-compact-detail', state.compact && !!showDetail);
            $('[data-mc-back]').hidden = !(state.compact && showDetail);
        }

        // ── running timer ──
        function startTimer() {
            if (state.timer) return;
            const tick = () => {
                const m = selected();
                if (!m || !missionIsRunning(m)) { stopTimer(); return; }
                detail.setElapsed(fmt.clock(Date.now() - (state.runStarted.get(m.id) || Date.now())));
            };
            tick();
            state.timer = setInterval(tick, 1000);
        }
        function stopTimer() { if (state.timer) { clearInterval(state.timer); state.timer = null; } }

        // ── selection / tabs / history ──
        async function selectMission(id, fromUser) {
            if (!id || id === state.selectedId) { if (state.compact && fromUser) { state.compactView = 'detail'; syncCompact(); } return; }
            if (state.editing && !(await closeEditor(false))) { list.setSelected(state.selectedId); return; }
            state.selectedId = id;
            state.tab = 'overview';
            state.preparedOpen = false;
            state.history = { missionId: '', items: [], total: 0, loading: false, error: '', filter: 'all' };
            if (state.compact && fromUser) state.compactView = 'detail';
            list.setSelected(id);
            detail.element.hidden = false;
            syncDetail(); syncMenus(); syncCompact();
        }

        function setTab(tab) {
            if (!selected()) return;
            state.tab = tab === 'history' ? 'history' : 'overview';
            detail.setTab(state.tab);
            if (state.tab === 'history' && state.history.missionId !== state.selectedId) loadHistory(true);
            syncMenus();
        }

        async function loadHistory(reset) {
            const m = selected();
            if (!m) return;
            const h = state.history;
            if (reset) Object.assign(h, { missionId: m.id, items: [], total: 0, error: '' });
            h.loading = true;
            detail.setHistory(h);
            const params = new URLSearchParams({ mission_id: m.id, limit: String(HISTORY_PAGE), offset: String(h.items.length) });
            if (h.filter === 'success') params.set('result', 'success');
            if (h.filter === 'error' || h.filter === 'cancelled') params.set('result', 'error');
            try {
                const data = await api('/api/missions/v2/history?' + params.toString());
                if (state.disposed || h.missionId !== state.selectedId) return;
                const entries = Array.isArray(data && data.entries) ? data.entries : [];
                h.items = reset ? entries : h.items.concat(entries);
                h.total = Number(data && data.total) || h.items.length;
                h.error = '';
            } catch (err) {
                h.error = (err && err.message) || 'error';
            } finally {
                h.loading = false;
                if (!state.disposed && h.missionId === state.selectedId) detail.setHistory(h);
            }
        }

        // ── actions ──
        async function withBusy(name, fn) {
            if (readonly) { notify(t('desktop.mc_toast_readonly'), 'error'); return; }
            state.busy = name;
            syncDetail();
            try { await fn(); } catch (err) { failToast(err); } finally { state.busy = ''; if (!state.disposed) { syncDetail(); syncMenus(); } }
        }
        const actions = {
            refresh: () => loadData(),
            newMission: () => openEditor('new', null),
            edit: () => { const m = selected(); if (m) openEditor('edit', m); },
            duplicate: () => { const m = selected(); if (m) openEditor('duplicate', m); },
            run: () => actions.runMission(state.selectedId),
            cancelRun: () => actions.cancelMission(state.selectedId),
            removeFromQueue: () => actions.removeMissionFromQueue(state.selectedId),
            togglePause: () => actions.togglePauseMission(state.selectedId),
            toggleLock: () => actions.toggleLockMission(state.selectedId),
            delete: () => actions.deleteMission(state.selectedId),
            prepare: () => actions.prepareMission(state.selectedId),
            invalidatePrep: () => actions.invalidatePrepMission(state.selectedId),
            setFilter: (f) => { state.filter = FILTERS.includes(f) ? f : 'all'; $('[data-mc-filter]').value = state.filter; list.setFilter(state.filter); savePrefs(state); syncStatus(); syncMenus(); },
            setSort: (s) => { state.sort = SORTS.includes(s) ? s : 'name'; $('[data-mc-sort]').value = state.sort; list.setSort(state.sort); savePrefs(state); syncMenus(); },
            setTab,
            toggleList: () => { state.listCollapsed = !state.listCollapsed; root.classList.toggle('is-list-collapsed', state.listCollapsed); const tb = $('[data-mc-list-toggle]'); tb.setAttribute('aria-pressed', String(!state.listCollapsed)); tb.title = t(state.listCollapsed ? 'desktop.mc_list_expand' : 'desktop.mc_list_collapse'); savePrefs(state); syncMenus(); },
            runMission: (id) => withBusy('run', async () => {
                const data = await api('/api/missions/v2/' + encodeURIComponent(id) + '/run', { method: 'POST' });
                notify(toastForMissionDispatch(data || {}));
                await loadData();
            }),
            cancelMission: (id) => withBusy('cancel', async () => {
                await api('/api/missions/v2/' + encodeURIComponent(id) + '/cancel', { method: 'POST' });
                state.cancelling.add(id);
                notify(t('desktop.mc_toast_cancel_requested'));
            }),
            removeMissionFromQueue: (id) => withBusy('removeQueue', async () => {
                await api('/api/missions/v2/' + encodeURIComponent(id) + '/queue', { method: 'DELETE' });
                notify(t('desktop.mc_toast_removed_queue'));
                await loadData();
            }),
            togglePauseMission: (id) => withBusy('pause', async () => {
                const m = byId(id); if (!m) return;
                await putMission(m, { enabled: m.enabled === false });
                notify(t(m.enabled === false ? 'desktop.mc_toast_resumed' : 'desktop.mc_toast_paused'));
                await loadData();
            }),
            toggleLockMission: (id) => withBusy('lock', async () => {
                const m = byId(id); if (!m) return;
                await putMission(m, { locked: !m.locked });
                notify(t(m.locked ? 'desktop.mc_toast_unlocked' : 'desktop.mc_toast_locked'));
                await loadData();
            }),
            deleteMission: async (id) => {
                const m = byId(id); if (!m || readonly) return;
                const ok = await confirmDialog(t('desktop.mc_delete_title'), t('desktop.mc_delete_message', { name: m.name }));
                if (!ok) return;
                await withBusy('delete', async () => {
                    await api('/api/missions/v2/' + encodeURIComponent(id), { method: 'DELETE' });
                    notify(t('desktop.mc_toast_deleted'));
                    if (state.selectedId === id) state.selectedId = '';
                    await loadData();
                });
            },
            prepareMission: (id) => withBusy('prepare', async () => {
                await api('/api/missions/v2/' + encodeURIComponent(id) + '/prepare', { method: 'POST' });
                notify(t('desktop.mc_toast_prepare_started'));
                await loadData();
            }),
            invalidatePrepMission: (id) => withBusy('invalidatePrep', async () => {
                await api('/api/missions/v2/' + encodeURIComponent(id) + '/prepared', { method: 'DELETE' });
                state.preparedOpen = false;
                detail.setPrepared(null, false);
                notify(t('desktop.mc_toast_prep_discarded'));
                await loadData();
            }),
            viewPrep: async (id) => {
                if (state.preparedOpen) { state.preparedOpen = false; detail.setPrepared(null, false); return; }
                detail.setPrepared(null, true);
                try {
                    const data = await api('/api/missions/v2/' + encodeURIComponent(id) + '/prepared');
                    if (state.selectedId !== id) return;
                    state.preparedOpen = true;
                    detail.setPrepared(data || {}, false);
                } catch (err) { detail.setPrepared(null, false); failToast(err); }
            },
            copyOutput: async (id) => {
                const m = byId(id); if (!m) return;
                try {
                    await navigator.clipboard.writeText(window.MissionControlDetail.extractLastOutput(m.last_output));
                    notify(t('desktop.mc_overview_copied'));
                } catch (err) { failToast(err); }
            },
            editMission: (id) => { const m = byId(id); if (m) openEditor('edit', m); },
            duplicateMission: (id) => { const m = byId(id); if (m) openEditor('duplicate', m); }
        };
        menuModel.actions = actions;

        function putMission(mission, patch) {
            const body = Object.assign({}, mission, patch);
            delete body.next_run;
            return api('/api/missions/v2/' + encodeURIComponent(mission.id), { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        }

        // ── editor ──
        async function openEditor(mode, mission) {
            if (readonly) { notify(t('desktop.mc_toast_readonly'), 'error'); return; }
            if (state.editing && !(await closeEditor(false))) return;
            state.editing = { mode, id: mode === 'edit' && mission ? mission.id : '' };
            if (mode === 'edit' && mission) { state.selectedId = mission.id; list.setSelected(mission.id); }
            stopTimer();
            detail.element.hidden = true;
            editor.open({ mode, mission });
            if (state.compact) state.compactView = 'detail';
            syncMenus(); syncCompact();
        }

        // Returns true when the editor is closed (either clean, confirmed discard or forced).
        async function closeEditor(force) {
            if (!state.editing) return true;
            if (!force && editor.isDirty()) {
                const ok = await confirmDialog(t('desktop.mc_editor_discard_title'), t('desktop.mc_editor_discard_message'));
                if (!ok) return false;
            }
            editor.close();
            state.editing = null;
            if (state.initialLoad) detail.element.hidden = false;
            if (state.compact && !state.selectedId) state.compactView = 'list';
            syncDetail(); syncMenus(); syncCompact();
            return true;
        }

        editor.on('cancel', () => closeEditor(false));
        editor.on('save', async ({ mode, id, payload }) => {
            editor.setSaving(true);
            editor.setServerError('');
            try {
                const url = mode === 'edit' ? '/api/missions/v2/' + encodeURIComponent(id) : '/api/missions/v2';
                const res = await api(url, { method: mode === 'edit' ? 'PUT' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
                notify(t(mode === 'edit' ? 'desktop.mc_toast_saved' : 'desktop.mc_toast_created'));
                const newId = mode === 'edit' ? id : (res && res.id) || '';
                editor.close();
                state.editing = null;
                await loadData();
                if (newId && byId(newId)) { state.selectedId = newId; list.setSelected(newId); }
                detail.element.hidden = false;
                if (state.compact) state.compactView = 'detail';
                syncAll();
                list.focusSelected();
            } catch (err) {
                editor.setSaving(false);
                editor.setServerError((err && err.message) || String(err));
            }
        });
        if (typeof setWindowBeforeClose === 'function') {
            setWindowBeforeClose(windowId, async () => {
                if (!state.editing || !editor.isDirty()) return true;
                return confirmDialog(t('desktop.mc_editor_discard_title'), t('desktop.mc_editor_discard_message'));
            });
        }

        // ── list / detail events ──
        list.on('select', (id) => selectMission(id, true));
        list.on('activate', (id) => { selectMission(id, true); if (!state.editing) { const tab = detail.element.querySelector('[data-mc-tab="overview"]'); if (tab) tab.focus(); } });
        list.on('run', (id) => actions.runMission(id));
        list.on('delete', (id) => actions.deleteMission(id));
        list.on('new', () => actions.newMission());
        list.on('clearFilter', () => { actions.setFilter('all'); setQuery(''); });
        list.on('contextmenu', ({ x, y, missionId, queued }) => {
            if (typeof showContextMenu !== 'function') return;
            const m = byId(missionId);
            if (!m) { showContextMenu(x, y, MN.listContextItems(menuModel)); return; }
            syncMenus();
            showContextMenu(x, y, queued ? MN.queueContextItems(menuModel, missionId).concat([{ separator: true }], MN.missionContextItems(menuModel, m)) : MN.missionContextItems(menuModel, m));
        });
        detail.on('tab', setTab);
        detail.on('historyFilter', (f) => { state.history.filter = f; loadHistory(true); });
        detail.on('historyMore', () => loadHistory(false));
        detail.on('historyRetry', () => loadHistory(true));
        detail.on('contextmenu', ({ x, y, missionId }) => { const m = byId(missionId); if (m && typeof showContextMenu === 'function') { syncMenus(); showContextMenu(x, y, MN.missionContextItems(menuModel, m)); } });
        detail.on('action', ({ name, missionId, x, y }) => {
            const map = { run: 'runMission', cancel: 'cancelMission', removeQueue: 'removeMissionFromQueue', edit: 'editMission', duplicate: 'duplicateMission', delete: 'deleteMission', pause: 'togglePauseMission', resume: 'togglePauseMission', lock: 'toggleLockMission', unlock: 'toggleLockMission', prepare: 'prepareMission', invalidatePrep: 'invalidatePrepMission', viewPrep: 'viewPrep', copyOutput: 'copyOutput' };
            if (name === 'more') { const m = byId(missionId); if (m && typeof showContextMenu === 'function') { syncMenus(); showContextMenu(x, y, MN.missionContextItems(menuModel, m)); } return; }
            if (map[name]) actions[map[name]](missionId);
        });

        // ── toolbar ──
        const searchInput = $('[data-mc-search]');
        function setQuery(text) {
            state.query = text || '';
            if (searchInput.value !== state.query) searchInput.value = state.query;
            $('[data-mc-search-clear]').hidden = !state.query;
            list.setQuery(state.query);
            syncStatus();
        }
        let searchTimer = null;
        searchInput.addEventListener('input', () => { clearTimeout(searchTimer); searchTimer = setTimeout(() => setQuery(searchInput.value), 120); });
        searchInput.addEventListener('keydown', (event) => {
            if (event.key === 'Escape') { event.stopPropagation(); setQuery(''); }
            if (event.key === 'ArrowDown' || event.key === 'Enter') { event.preventDefault(); list.element.focus(); }
        });
        $('[data-mc-search-clear]').addEventListener('click', () => { setQuery(''); searchInput.focus(); });
        $('[data-mc-filter]').addEventListener('change', (event) => actions.setFilter(event.target.value));
        $('[data-mc-sort]').addEventListener('change', (event) => actions.setSort(event.target.value));
        $('[data-mc-refresh]').addEventListener('click', () => actions.refresh());
        $('[data-mc-list-toggle]').addEventListener('click', () => actions.toggleList());
        $('[data-mc-retry]').addEventListener('click', () => { $('[data-mc-loaderror]').hidden = true; $('[data-mc-loading]').hidden = false; loadData(); });
        $('[data-mc-back]').addEventListener('click', async () => { if (state.editing && !(await closeEditor(false))) return; state.compactView = 'list'; syncCompact(); list.focusSelected(); });
        const newBtn = $('[data-mc-new]');
        if (newBtn) newBtn.addEventListener('click', () => actions.newMission());

        // ── splitter (pointer + keyboard) ──
        const splitter = $('[data-mc-splitter]');
        function setListWidth(px, persist) {
            state.listWidth = clamp(Math.round(px), LIST_MIN, LIST_MAX);
            root.style.setProperty('--mc-list-width', state.listWidth + 'px');
            splitter.setAttribute('aria-valuenow', String(state.listWidth));
            if (persist) savePrefs(state);
        }
        splitter.addEventListener('pointerdown', (event) => {
            if (state.compact) return;
            event.preventDefault();
            const startX = event.clientX, startWidth = state.listWidth;
            root.classList.add('is-resizing');
            splitter.setPointerCapture(event.pointerId);
            const move = (ev) => setListWidth(startWidth + (ev.clientX - startX), false);
            const up = () => { root.classList.remove('is-resizing'); splitter.removeEventListener('pointermove', move); splitter.removeEventListener('pointerup', up); splitter.removeEventListener('pointercancel', up); savePrefs(state); };
            splitter.addEventListener('pointermove', move);
            splitter.addEventListener('pointerup', up);
            splitter.addEventListener('pointercancel', up);
        });
        splitter.addEventListener('keydown', (event) => {
            if (event.key === 'ArrowLeft') { event.preventDefault(); setListWidth(state.listWidth - 16, true); }
            else if (event.key === 'ArrowRight') { event.preventDefault(); setListWidth(state.listWidth + 16, true); }
            else if (event.key === 'Home') { event.preventDefault(); setListWidth(LIST_MIN, true); }
            else if (event.key === 'End') { event.preventDefault(); setListWidth(LIST_MAX, true); }
        });
        splitter.addEventListener('dblclick', () => setListWidth(LIST_DEFAULT, true));

        // ── compact mode ──
        if (typeof ResizeObserver === 'function') {
            state.resizeObserver = new ResizeObserver((entries) => {
                const width = entries[0] && entries[0].contentRect ? entries[0].contentRect.width : root.clientWidth;
                const compact = width > 0 && width < COMPACT_BREAKPOINT;
                if (compact === state.compact) return;
                state.compact = compact;
                if (compact && !state.selectedId && !state.editing) state.compactView = 'list';
                syncCompact();
            });
            state.resizeObserver.observe(root);
        }

        // ── keyboard shortcuts (window-scoped) ──
        function handleKeydown(e) {
            if (state.disposed) return;
            if (typeof isActive === 'function' && !isActive()) return;
            const inField = e.target && e.target.closest && e.target.closest('input, textarea, select, [contenteditable="true"]');
            const mod = e.ctrlKey || e.metaKey;
            if (mod && e.key.toLowerCase() === 'f') { e.preventDefault(); searchInput.focus(); searchInput.select(); return; }
            if (mod && e.key.toLowerCase() === 'b') { e.preventDefault(); actions.toggleList(); return; }
            if (e.key === 'F5') { e.preventDefault(); actions.refresh(); return; }
            if (state.editing) return; // the editor owns Esc / Ctrl+Enter / Ctrl+S
            if (mod && e.key.toLowerCase() === 'n') { e.preventDefault(); actions.newMission(); return; }
            if (mod && e.key.toLowerCase() === 'e') { e.preventDefault(); actions.edit(); return; }
            if (mod && e.key.toLowerCase() === 'd') { e.preventDefault(); actions.duplicate(); return; }
            if (mod && e.key === 'Enter') { e.preventDefault(); if (menuModel.s.selected && !menuModel.s.running && !menuModel.s.queued) actions.run(); return; }
            if (mod && e.key === '1') { e.preventDefault(); setTab('overview'); return; }
            if (mod && e.key === '2') { e.preventDefault(); setTab('history'); return; }
            if (!inField && e.key === 'Escape' && state.compact && state.compactView === 'detail') { e.preventDefault(); state.compactView = 'list'; syncCompact(); list.focusSelected(); }
        }
        state.keydownHandler = handleKeydown;
        document.addEventListener('keydown', handleKeydown);

        // ── SSE ──
        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
            state.sseHandler = function (payload) {
                if (!state.initialLoad || state.disposed) return;
                state.live = true;
                applyData(payload);
                syncAll();
            };
            window.AuraSSE.on('mission_update', state.sseHandler);
            state.live = true;
        }

        if (typeof wireContextMenuBoundary === 'function') wireContextMenuBoundary(container);
        root.addEventListener('contextmenu', (event) => {
            // Blank areas of the main pane: generic list actions. List/detail handle their own targets.
            if (event.target.closest('.vd-mc-list, .vd-mc-detail, .vd-mc-editor, input, textarea, select')) return;
            event.preventDefault();
            if (typeof showContextMenu === 'function') showContextMenu(event.clientX, event.clientY, MN.listContextItems(menuModel));
        });

        state.cleanup = () => {
            stopTimer();
            clearTimeout(searchTimer);
            if (state.resizeObserver) { state.resizeObserver.disconnect(); state.resizeObserver = null; }
            list.dispose(); detail.dispose(); editor.dispose();
            if (typeof clearWindowMenus === 'function') clearWindowMenus(windowId);
        };

        syncMenus();
        loadData();
    }

    function dispose(windowId) {
        const st = instances.get(windowId);
        if (!st) return;
        st.disposed = true;
        if (st.sseHandler && window.AuraSSE && typeof window.AuraSSE.off === 'function') {
            window.AuraSSE.off('mission_update', st.sseHandler);
        }
        if (st.keydownHandler) {
            document.removeEventListener('keydown', st.keydownHandler);
            st.keydownHandler = null;
        }
        if (typeof st.cleanup === 'function') { try { st.cleanup(); } catch (err) { console.warn('MC: cleanup failed', err); } }
        instances.delete(windowId);
    }

    window.MissionControlApp = { render, dispose };
})();
