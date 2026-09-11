    // Calendar app shell: per-window session state, incremental painting,
    // navigation, keyboard shortcuts, drag & drop with undo and mutations.
    // View renderers live in calendar-views.js, the editor in calendar-editor.js.

    const calendarSessions = new Map();
    const CAL_VIEWS = ['day', 'week', 'month', 'agenda'];
    const CAL_PREFS_KEY = 'aurago.desktop.calendar.prefs';
    const CAL_REFRESH_AFTER_MS = 60000;

    function calendarPrefs() {
        try { return JSON.parse(localStorage.getItem(CAL_PREFS_KEY) || '{}') || {}; } catch (_) { return {}; }
    }

    function saveCalendarPrefs(patch) {
        try { localStorage.setItem(CAL_PREFS_KEY, JSON.stringify(Object.assign(calendarPrefs(), patch))); } catch (_) { /* storage unavailable */ }
    }

    function calendarShellHTML(session) {
        const viewButtons = CAL_VIEWS.map(view => `<button type="button" class="vd-calendar-view-button" role="tab" data-cal-view="${view}" aria-selected="false" title="${esc(t(`desktop.cal_${view}`))} (${view.charAt(0).toUpperCase()})">${esc(t(`desktop.cal_${view}`))}</button>`).join('');
        return `<div class="vd-calendar-shell" data-cal-view="${esc(session.view)}">
            <header class="vd-calendar-command">
                <div class="vd-calendar-command-group">
                    <button type="button" class="vd-calendar-icon-button" data-cal-sidebar-toggle aria-pressed="${session.sidebarOpen ? 'true' : 'false'}" title="${esc(t('desktop.cal_toggle_sidebar'))}" aria-label="${esc(t('desktop.cal_toggle_sidebar'))}">${iconMarkup('columns', '|', 'vd-calendar-action-icon', 15)}</button>
                    <button type="button" class="vd-button vd-calendar-today-button" data-cal-today title="${esc(t('desktop.cal_today'))} (T)">${esc(t('desktop.cal_today'))}</button>
                    <div class="vd-calendar-nav" role="group">
                        <button type="button" class="vd-calendar-icon-button" data-cal-nav="-1" title="${esc(t('desktop.cal_previous'))}" aria-label="${esc(t('desktop.cal_previous'))}">${iconMarkup('chevron-left', '<', 'vd-calendar-action-icon', 14)}</button>
                        <button type="button" class="vd-calendar-icon-button" data-cal-nav="1" title="${esc(t('desktop.cal_next'))}" aria-label="${esc(t('desktop.cal_next'))}">${iconMarkup('chevron-right', '>', 'vd-calendar-action-icon', 14)}</button>
                    </div>
                    <div class="vd-calendar-range">
                        <h2 class="vd-calendar-range-title" data-cal-title aria-live="polite"></h2>
                        <button type="button" class="vd-calendar-icon-button vd-calendar-jump" data-cal-jump title="${esc(t('desktop.cal_go_to_date'))}" aria-label="${esc(t('desktop.cal_go_to_date'))}">${iconMarkup('calendar', 'C', 'vd-calendar-action-icon', 14)}</button>
                        <input type="date" class="vd-calendar-jump-input" data-cal-jump-input tabindex="-1" aria-hidden="true">
                    </div>
                </div>
                <div class="vd-calendar-command-group vd-calendar-command-end">
                    <label class="vd-calendar-search">
                        ${iconMarkup('search', 'S', 'vd-calendar-action-icon', 14)}
                        <input type="search" data-cal-search inputmode="search" enterkeyhint="search" placeholder="${esc(t('desktop.cal_search_placeholder'))}" aria-label="${esc(t('desktop.search'))}" autocomplete="off" spellcheck="false">
                        <button type="button" class="vd-calendar-search-clear" data-cal-search-clear aria-label="${esc(t('desktop.clear'))}" hidden>×</button>
                    </label>
                    <div class="vd-calendar-view-switch" role="tablist" aria-label="${esc(t('desktop.menu_view'))}">${viewButtons}</div>
                    <button type="button" class="vd-button vd-button-primary vd-calendar-create" data-cal-create title="${esc(t('desktop.cal_new_appointment'))} (N)">${iconMarkup('plus', '+', 'vd-calendar-action-icon', 14)}<span>${esc(t('desktop.cal_new'))}</span></button>
                </div>
                <div class="vd-calendar-progress" data-cal-progress hidden><i></i></div>
            </header>
            <div class="vd-calendar-stage">
                <aside class="vd-calendar-sidebar" data-cal-sidebar>
                    <div class="vd-calendar-side-mini" data-cal-mini></div>
                    <div class="vd-calendar-side-agenda" data-cal-side-agenda></div>
                    <div class="vd-calendar-side-filters">
                        <label class="vd-calendar-check"><input type="checkbox" data-cal-filter="completed"><span class="vd-calendar-event-dot is-completed" aria-hidden="true"></span><span>${esc(t('desktop.cal_show_completed'))}</span></label>
                        <label class="vd-calendar-check"><input type="checkbox" data-cal-filter="cancelled"><span class="vd-calendar-event-dot is-cancelled" aria-hidden="true"></span><span>${esc(t('desktop.cal_show_cancelled'))}</span></label>
                    </div>
                </aside>
                <div class="vd-calendar-body" data-cal-body></div>
            </div>
            <div class="vd-calendar-snackbar" data-cal-snackbar hidden></div>
        </div>`;
    }

    async function renderCalendar(id) {
        const host = contentEl(id);
        if (!host) return;
        const existing = calendarSessions.get(id);
        if (existing) existing.dispose();
        const prefs = calendarPrefs();
        const session = {
            id, host,
            view: CAL_VIEWS.includes(prefs.view) ? prefs.view : 'month',
            cursor: new Date(),
            selected: isoDate(new Date()),
            miniCursor: null,
            appointments: [],
            loaded: false, loading: false, error: null, lastLoadedAt: 0,
            query: '', viewBeforeSearch: null,
            filters: { completed: prefs.showCompleted !== false, cancelled: !!prefs.showCancelled },
            sidebarOpen: prefs.sidebar !== false && !isCompactViewport(),
            monthDensity: 3,
            timeScrollTop: null,
            timers: [], listeners: [], observer: null,
            editor: null, peek: null, contactsPromise: null,
            dragId: null, snackTimer: null, snackAction: null
        };
        session.reload = options => loadCalendarAppointments(session, options);
        session.snack = options => showCalendarSnack(session, options);
        session.focusDate = (date, options) => calendarFocusDate(session, date, options);
        session.paint = () => paintCalendar(session);
        session.dispose = () => disposeCalendarSession(session);
        calendarSessions.set(id, session);

        host.innerHTML = calendarShellHTML(session);
        wireCalendarShell(session);
        // Bound once per host: the handler resolves the live session so re-renders keep working.
        wireContextMenuBoundary(host, { onContextMenu: event => showCalendarContextMenu(calendarSessions.get(id), event) });
        setCalendarMenus(id, session);
        registerWindowCleanup(id, () => {
            session.dispose();
            calendarSessions.delete(id);
            document.querySelectorAll('.vd-modal-backdrop.vd-calendar-editor-backdrop').forEach(el => el.remove());
        });
        paintCalendar(session);
        await loadCalendarAppointments(session, { initial: true });
    }

    function disposeCalendarSession(session) {
        session.timers.forEach(timer => clearInterval(timer));
        session.timers = [];
        session.listeners.forEach(([target, type, handler, options]) => target.removeEventListener(type, handler, options));
        session.listeners = [];
        if (session.observer) { session.observer.disconnect(); session.observer = null; }
        clearTimeout(session.snackTimer);
        closeCalendarPeek(session);
        closeCalendarEditor(session);
    }

    function calendarListen(session, target, type, handler, options) {
        target.addEventListener(type, handler, options);
        session.listeners.push([target, type, handler, options]);
    }

    function setCalendarMenus(id, session) {
        setWindowMenus(id, [
            {
                id: 'file', labelKey: 'desktop.menu_file', items: [
                    { id: 'new-appointment', labelKey: 'desktop.cal_new_appointment', icon: 'plus', shortcut: 'Ctrl+N', action: () => openAppointmentEditor(session, { dateHint: session.selected }) },
                    { type: 'separator' },
                    { id: 'refresh', labelKey: 'desktop.context_refresh', icon: 'refresh', shortcut: 'F5', action: () => loadCalendarAppointments(session, { silent: true }) }
                ]
            },
            {
                id: 'view', labelKey: 'desktop.menu_view', items: [
                    { id: 'today', labelKey: 'desktop.cal_today', icon: 'calendar', action: () => calendarGoToday(session) },
                    { type: 'separator' },
                    ...CAL_VIEWS.map(view => ({ id: `view-${view}`, labelKey: `desktop.cal_${view}`, checked: session.view === view, action: () => setCalendarView(session, view) })),
                    { type: 'separator' },
                    { id: 'sidebar', labelKey: 'desktop.cal_toggle_sidebar', icon: 'columns', checked: session.sidebarOpen, action: () => toggleCalendarSidebar(session) }
                ]
            }
        ]);
    }

    async function loadCalendarAppointments(session, options) {
        options = options || {};
        if (session.loading) return;
        session.loading = true;
        const progress = session.host.querySelector('[data-cal-progress]');
        if (progress && session.loaded) progress.hidden = false;
        if (!session.loaded) paintCalendar(session);
        try {
            const data = await api('/api/appointments?status=all');
            session.appointments = normalizeCalendarAppointments(data);
            session.loaded = true;
            session.error = null;
            session.lastLoadedAt = Date.now();
        } catch (err) {
            session.error = err;
            if (session.loaded && !options.initial) session.snack({ type: 'error', message: t('desktop.load_failed') });
        } finally {
            session.loading = false;
            if (progress) progress.hidden = true;
        }
        paintCalendar(session);
    }

    function calendarMonthDensity(session) {
        const body = session.host.querySelector('[data-cal-body]');
        if (!body) return 3;
        const rows = calendarMonthCells(session.cursor).rows;
        const cellHeight = Math.max(76, (body.clientHeight - 30) / rows);
        return Math.max(1, Math.min(8, Math.floor((cellHeight - 30) / 23)));
    }

    function paintCalendar(session) {
        const host = session.host;
        const shell = host.querySelector('.vd-calendar-shell');
        if (!shell) return;
        const searching = !!session.query.trim();
        shell.dataset.calView = session.view;
        shell.classList.toggle('is-sidebar-collapsed', !session.sidebarOpen);
        shell.classList.toggle('is-searching', searching);
        shell.classList.toggle('is-loading', session.loading && !session.loaded);

        const title = host.querySelector('[data-cal-title]');
        if (title) title.textContent = searching ? t('desktop.cal_search_results_title') : calendarRangeLabel(session.view, session.cursor);
        host.querySelectorAll('[data-cal-nav], [data-cal-today]').forEach(button => { button.disabled = searching; });
        host.querySelectorAll('[data-cal-view]').forEach(button => {
            const active = button.dataset.calView === session.view;
            button.classList.toggle('is-active', active);
            button.setAttribute('aria-selected', active ? 'true' : 'false');
            button.tabIndex = active ? 0 : -1;
        });
        const toggle = host.querySelector('[data-cal-sidebar-toggle]');
        if (toggle) toggle.setAttribute('aria-pressed', session.sidebarOpen ? 'true' : 'false');
        const clear = host.querySelector('[data-cal-search-clear]');
        if (clear) clear.hidden = !searching;

        const body = host.querySelector('[data-cal-body]');
        if (body) {
            const scroll = body.querySelector('[data-cal-time-scroll]');
            if (scroll) session.timeScrollTop = scroll.scrollTop;
            if (!session.loaded && session.loading) {
                body.innerHTML = calendarSkeletonHTML(session.view);
            } else if (!session.loaded && session.error) {
                body.innerHTML = `<div class="vd-calendar-empty is-error"><strong>${esc(t('desktop.load_failed'))}</strong><p>${esc(session.error.message || '')}</p><button type="button" class="vd-button" data-cal-retry>${iconMarkup('refresh', 'R', 'vd-calendar-action-icon', 14)}<span>${esc(t('desktop.retry'))}</span></button></div>`;
            } else {
                const items = calendarVisibleAppointments(session);
                if (session.view === 'month') {
                    session.monthDensity = calendarMonthDensity(session);
                    body.innerHTML = calendarMonthHTML(session, items);
                } else if (session.view === 'week') {
                    body.innerHTML = calendarTimeGridHTML(session, items, calendarWeekDays(session.cursor));
                } else if (session.view === 'day') {
                    body.innerHTML = calendarTimeGridHTML(session, items, [calStartOfDay(session.cursor)]);
                } else {
                    body.innerHTML = calendarAgendaHTML(session, items);
                }
                calendarRestoreTimeScroll(session);
            }
        }

        const mini = host.querySelector('[data-cal-mini]');
        if (mini) mini.innerHTML = calendarMiniMonthHTML(session);
        const side = host.querySelector('[data-cal-side-agenda]');
        if (side) side.innerHTML = session.loaded ? calendarSidebarAgendaHTML(session) : '';
        host.querySelectorAll('[data-cal-filter]').forEach(input => { input.checked = !!session.filters[input.dataset.calFilter]; });
    }

    function calendarRestoreTimeScroll(session) {
        const scroll = session.host.querySelector('[data-cal-time-scroll]');
        if (!scroll) return;
        if (session.timeScrollTop !== null && session.timeScrollTop >= 0) {
            scroll.scrollTop = session.timeScrollTop;
            return;
        }
        const now = new Date();
        const days = session.view === 'day' ? [session.cursor] : calendarWeekDays(session.cursor);
        const includesToday = days.some(day => calSameDay(day, now));
        const anchorHour = includesToday ? Math.max(0, now.getHours() - 2) : 7;
        scroll.scrollTop = anchorHour * CAL_HOUR_HEIGHT;
    }

    function calendarScrollToNow(session) {
        const scroll = session.host.querySelector('[data-cal-time-scroll]');
        if (!scroll) return;
        const target = Math.max(0, (calMinutesOfDay(new Date()) / 60) * CAL_HOUR_HEIGHT - scroll.clientHeight / 2);
        scroll.scrollTo({ top: target, behavior: document.body.dataset.animations === 'false' ? 'auto' : 'smooth' });
    }

    function calendarUpdateNowLine(session) {
        const now = new Date();
        if (isoDate(now) !== session.todayKey) {
            session.todayKey = isoDate(now);
            paintCalendar(session);
            return;
        }
        session.host.querySelectorAll('[data-cal-now]').forEach(line => {
            line.style.top = `${((calMinutesOfDay(now) / 60) * CAL_HOUR_HEIGHT).toFixed(1)}px`;
            const label = line.querySelector('span');
            if (label) label.textContent = calFormatTime(now);
        });
    }

    function setCalendarView(session, view, options) {
        if (!CAL_VIEWS.includes(view)) return;
        options = options || {};
        if (session.view !== view) session.timeScrollTop = null;
        session.view = view;
        if (!options.transient) saveCalendarPrefs({ view });
        setCalendarMenus(session.id, session);
        paintCalendar(session);
        calendarFlashBody(session);
    }

    function calendarFlashBody(session) {
        const body = session.host.querySelector('[data-cal-body]');
        if (!body || document.body.dataset.animations === 'false') return;
        body.classList.remove('is-entering');
        void body.offsetWidth;
        body.classList.add('is-entering');
    }

    function calendarFocusDate(session, date, options) {
        options = options || {};
        const target = date instanceof Date ? date : calParseISODate(date);
        if (Number.isNaN(target.getTime())) return;
        session.cursor = new Date(target);
        session.selected = isoDate(target);
        session.miniCursor = null;
        if (options.view) session.view = options.view;
        if (options.view) { session.timeScrollTop = null; saveCalendarPrefs({ view: options.view }); setCalendarMenus(session.id, session); }
        paintCalendar(session);
        if (options.focusCell) {
            const cell = session.host.querySelector(`.vd-calendar-cell[data-cal-date="${session.selected}"]`);
            if (cell) cell.focus({ preventScroll: true });
        }
    }

    function shiftCalendarPeriod(session, direction) {
        const cursor = new Date(session.cursor);
        if (session.view === 'month') session.cursor = calAddMonths(cursor, direction);
        else if (session.view === 'week') session.cursor = calAddDays(cursor, 7 * direction);
        else if (session.view === 'day') session.cursor = calAddDays(cursor, direction);
        else return;
        session.selected = isoDate(session.cursor);
        session.miniCursor = null;
        paintCalendar(session);
        calendarFlashBody(session);
    }

    function calendarGoToday(session) {
        const today = new Date();
        session.cursor = today;
        session.selected = isoDate(today);
        session.miniCursor = null;
        session.timeScrollTop = null;
        paintCalendar(session);
        calendarFlashBody(session);
        if (session.view === 'week' || session.view === 'day') calendarScrollToNow(session);
        const cell = session.host.querySelector('.vd-calendar-cell.is-today');
        if (cell) { cell.classList.add('is-pulse'); setTimeout(() => cell.classList.remove('is-pulse'), 900); }
    }

    function toggleCalendarSidebar(session) {
        session.sidebarOpen = !session.sidebarOpen;
        saveCalendarPrefs({ sidebar: session.sidebarOpen });
        setCalendarMenus(session.id, session);
        paintCalendar(session);
    }

    function calendarSetQuery(session, value) {
        const next = String(value || '');
        const wasSearching = !!session.query.trim();
        const isSearching = !!next.trim();
        session.query = next;
        if (isSearching && !wasSearching) {
            session.viewBeforeSearch = session.view;
            if (session.view !== 'agenda') setCalendarView(session, 'agenda', { transient: true });
        } else if (!isSearching && wasSearching) {
            const restore = session.viewBeforeSearch;
            session.viewBeforeSearch = null;
            if (restore && restore !== session.view) setCalendarView(session, restore, { transient: true });
        }
        paintCalendar(session);
    }

    function calendarFindAppointment(session, id) {
        return session.appointments.find(item => String(item.id) === String(id)) || null;
    }

    // Moves the selection highlight without repainting the grid (keyboard roving and cell clicks).
    function calendarMarkSelected(session) {
        session.host.querySelectorAll('.vd-calendar-cell.is-selected').forEach(el => { el.classList.remove('is-selected'); el.tabIndex = -1; el.setAttribute('aria-selected', 'false'); });
        const el = session.host.querySelector(`.vd-calendar-cell[data-cal-date="${session.selected}"]`);
        if (el) { el.classList.add('is-selected'); el.tabIndex = 0; el.setAttribute('aria-selected', 'true'); }
        const mini = session.host.querySelector('[data-cal-mini]');
        if (mini) mini.innerHTML = calendarMiniMonthHTML(session);
    }

    function calendarSlotDate(column, slot, clientY) {
        const day = calParseISODate(column.dataset.calColumn || column.dataset.calDropDate);
        const rect = slot.getBoundingClientRect();
        const ratio = Math.max(0, Math.min(0.999, (clientY - rect.top) / Math.max(1, rect.height)));
        const minutes = Math.floor(ratio * 4) * 15;
        day.setHours(Number(slot.dataset.calHour) || 0, minutes, 0, 0);
        return day;
    }

    function calendarDropDate(target, clientY, appointment) {
        const source = calendarDate(appointment.date_time) || new Date();
        const column = target.closest('[data-cal-column]');
        if (column) {
            const rect = column.getBoundingClientRect();
            const minutes = Math.max(0, Math.min(24 * 60 - 15, Math.round(((clientY - rect.top) / CAL_HOUR_HEIGHT) * 60 / 15) * 15));
            const day = calParseISODate(column.dataset.calColumn);
            day.setHours(Math.floor(minutes / 60), minutes % 60, 0, 0);
            return day;
        }
        const cell = target.closest('[data-cal-drop-date]');
        if (!cell) return null;
        const day = calParseISODate(cell.dataset.calDropDate);
        day.setHours(source.getHours(), source.getMinutes(), 0, 0);
        return day;
    }

    function calendarClearDropState(session) {
        session.host.querySelectorAll('.is-drop-target').forEach(el => el.classList.remove('is-drop-target'));
        session.host.querySelectorAll('.vd-calendar-drop-ghost').forEach(el => el.remove());
    }

    function calendarShowDropGhost(session, column, clientY) {
        const rect = column.getBoundingClientRect();
        const minutes = Math.max(0, Math.min(24 * 60 - 15, Math.round(((clientY - rect.top) / CAL_HOUR_HEIGHT) * 60 / 15) * 15));
        let ghost = column.querySelector('.vd-calendar-drop-ghost');
        if (!ghost) {
            session.host.querySelectorAll('.vd-calendar-drop-ghost').forEach(el => el.remove());
            ghost = document.createElement('div');
            ghost.className = 'vd-calendar-drop-ghost';
            ghost.innerHTML = '<span></span>';
            column.appendChild(ghost);
        }
        ghost.style.top = `${((minutes / 60) * CAL_HOUR_HEIGHT).toFixed(1)}px`;
        ghost.style.height = `${((CAL_EVENT_MINUTES / 60) * CAL_HOUR_HEIGHT).toFixed(1)}px`;
        ghost.querySelector('span').textContent = calFormatTime(new Date(2000, 0, 1, Math.floor(minutes / 60), minutes % 60));
    }

    async function updateAppointmentDateTime(session, appointment, target) {
        if (!appointment || !(target instanceof Date) || Number.isNaN(target.getTime())) return false;
        const previousDate = calendarDate(appointment.date_time);
        if (previousDate && previousDate.getTime() === target.getTime()) return false;
        const previous = { date_time: appointment.date_time, notification_at: appointment.notification_at, status: appointment.status };
        const patch = { date_time: target.toISOString() };
        if (appointment.notification_at && previousDate) {
            const reminder = calendarDate(appointment.notification_at);
            if (reminder) patch.notification_at = new Date(reminder.getTime() + (target - previousDate)).toISOString();
        }
        if (appointment.status === 'overdue' && target > new Date()) patch.status = 'upcoming';
        Object.assign(appointment, patch);
        session.appointments.sort((a, b) => new Date(a.date_time) - new Date(b.date_time));
        paintCalendar(session);
        const url = `/api/appointments/${encodeURIComponent(appointment.id)}`;
        try {
            await plannerJSON(url, 'PUT', patch);
        } catch (err) {
            Object.assign(appointment, previous);
            paintCalendar(session);
            session.snack({ type: 'error', message: err && err.message ? err.message : t('desktop.request_failed') });
            return false;
        }
        session.snack({
            message: t('desktop.cal_rescheduled', { date: calendarDateTimeLabel(patch.date_time) }),
            actionLabel: t('desktop.cal_undo'),
            onAction: async () => {
                await plannerJSON(url, 'PUT', previous);
                await loadCalendarAppointments(session, { silent: true });
                session.snack({ message: t('desktop.cal_restored') });
            }
        });
        return true;
    }

    async function updateAppointmentStatus(session, appointment, status) {
        if (!appointment || appointment.status === status) return;
        const previous = appointment.status;
        appointment.status = status;
        paintCalendar(session);
        closeCalendarPeek(session);
        const url = `/api/appointments/${encodeURIComponent(appointment.id)}`;
        try {
            await plannerJSON(url, 'PUT', { status });
        } catch (err) {
            appointment.status = previous;
            paintCalendar(session);
            session.snack({ type: 'error', message: err && err.message ? err.message : t('desktop.request_failed') });
            return;
        }
        const messages = { completed: 'desktop.cal_completed_toast', cancelled: 'desktop.cal_cancelled_toast', upcoming: 'desktop.cal_reopened_toast' };
        session.snack({
            message: t(messages[status] || 'desktop.cal_saved'),
            actionLabel: t('desktop.cal_undo'),
            onAction: async () => {
                await plannerJSON(url, 'PUT', { status: previous });
                await loadCalendarAppointments(session, { silent: true });
                session.snack({ message: t('desktop.cal_restored') });
            }
        });
        loadCalendarAppointments(session, { silent: true });
    }

    async function deleteCalendarAppointment(session, appointment) {
        if (!appointment) return;
        await api(`/api/appointments/${encodeURIComponent(appointment.id)}`, { method: 'DELETE' });
        session.appointments = session.appointments.filter(item => item.id !== appointment.id);
        closeCalendarPeek(session);
        paintCalendar(session);
        const restore = {
            title: appointment.title, description: appointment.description, date_time: appointment.date_time,
            notification_at: appointment.notification_at, wake_agent: appointment.wake_agent,
            agent_instruction: appointment.agent_instruction, status: appointment.status,
            contact_ids: (appointment.participants || []).map(p => p.id).filter(Boolean)
        };
        session.snack({
            message: t('desktop.cal_deleted'),
            actionLabel: t('desktop.cal_undo'),
            onAction: async () => {
                await plannerJSON('/api/appointments', 'POST', restore);
                await loadCalendarAppointments(session, { silent: true });
                session.snack({ message: t('desktop.cal_restored') });
            }
        });
    }

    function hideCalendarSnack(session) {
        const bar = session.host.querySelector('[data-cal-snackbar]');
        clearTimeout(session.snackTimer);
        session.snackAction = null;
        if (!bar) return;
        bar.classList.remove('is-visible');
        setTimeout(() => { if (!bar.classList.contains('is-visible')) bar.hidden = true; }, 200);
    }

    function showCalendarSnack(session, options) {
        const bar = session.host.querySelector('[data-cal-snackbar]');
        if (!bar) return;
        clearTimeout(session.snackTimer);
        options = options || {};
        session.snackAction = typeof options.onAction === 'function' ? options.onAction : null;
        bar.hidden = false;
        bar.className = `vd-calendar-snackbar is-${options.type || 'info'}`;
        bar.innerHTML = `<span class="vd-calendar-snackbar-text">${esc(options.message || '')}</span>
            ${session.snackAction ? `<button type="button" class="vd-calendar-snackbar-action" data-cal-snack-action>${esc(options.actionLabel || t('desktop.cal_undo'))}</button>` : ''}
            <button type="button" class="vd-calendar-snackbar-close" data-cal-snack-close aria-label="${esc(t('desktop.close'))}">×</button>`;
        requestAnimationFrame(() => bar.classList.add('is-visible'));
        session.snackTimer = setTimeout(() => hideCalendarSnack(session), options.duration || (session.snackAction ? 8000 : 3500));
    }

    async function runCalendarSnackAction(session, button) {
        const action = session.snackAction;
        if (!action) return;
        button.disabled = true;
        hideCalendarSnack(session);
        try { await action(); } catch (err) { session.snack({ type: 'error', message: err && err.message ? err.message : t('desktop.request_failed') }); }
    }

    function showCalendarContextMenu(session, event) {
        if (!session) return false;
        const eventEl = event.target.closest('[data-appt-id]');
        if (eventEl) {
            const item = calendarFindAppointment(session, eventEl.dataset.apptId);
            if (!item) return false;
            const open = item.status === 'upcoming' || item.status === 'overdue';
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.cal_edit_appointment', icon: 'edit', action: () => openAppointmentEditor(session, { appointment: item }) },
                { labelKey: 'desktop.cal_mark_complete', icon: 'check', hidden: !open, action: () => updateAppointmentStatus(session, item, 'completed') },
                { labelKey: 'desktop.cal_cancel_appointment', icon: 'x', hidden: !open, action: () => updateAppointmentStatus(session, item, 'cancelled') },
                { labelKey: 'desktop.cal_reopen', icon: 'undo', hidden: open, action: () => updateAppointmentStatus(session, item, 'upcoming') },
                { separator: true },
                { labelKey: 'desktop.delete', icon: 'trash', action: async () => { if (await confirmDialog(t('desktop.delete'), t('desktop.cal_delete_confirm'))) deleteCalendarAppointment(session, item).catch(err => session.snack({ type: 'error', message: err.message || t('desktop.request_failed') })); } }
            ]);
            return true;
        }
        const cell = event.target.closest('[data-cal-drop-date]');
        if (cell) {
            const date = cell.dataset.calDropDate;
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.cal_new_appointment', icon: 'plus', action: () => openAppointmentEditor(session, { dateHint: date }) },
                { labelKey: 'desktop.cal_open_day', icon: 'calendar', action: () => calendarFocusDate(session, date, { view: 'day' }) }
            ]);
            return true;
        }
        return false;
    }

    function calendarHandleKeydown(session, event) {
        if (session.editor || session.peek) return;
        const target = event.target;
        const editable = target && target.closest && target.closest('input, textarea, select, [contenteditable="true"]');
        if (editable) {
            if (event.key === 'Escape' && target.matches('[data-cal-search]')) { event.preventDefault(); target.value = ''; calendarSetQuery(session, ''); target.blur(); }
            return;
        }
        if (event.ctrlKey || event.metaKey || event.altKey) return;
        const key = event.key;
        // Roving tabs: arrow keys move between the view buttons (WAI-ARIA tabs pattern).
        const viewTab = target && target.closest ? target.closest('[data-cal-view]') : null;
        if (viewTab && (key === 'ArrowLeft' || key === 'ArrowRight')) {
            event.preventDefault();
            const index = CAL_VIEWS.indexOf(session.view);
            const next = CAL_VIEWS[(index + (key === 'ArrowRight' ? 1 : CAL_VIEWS.length - 1)) % CAL_VIEWS.length];
            setCalendarView(session, next);
            const button = session.host.querySelector(`[data-cal-view="${next}"]`);
            if (button) button.focus();
            return;
        }
        const cell = target && target.closest ? target.closest('.vd-calendar-cell[data-cal-date]') : null;
        const moveSelection = days => {
            const next = calAddDays(calParseISODate(session.selected), days);
            const monthChanged = next.getMonth() !== session.cursor.getMonth() || next.getFullYear() !== session.cursor.getFullYear();
            session.selected = isoDate(next);
            if (monthChanged) { session.cursor = new Date(next); paintCalendar(session); }
            else calendarMarkSelected(session);
            const el = session.host.querySelector(`.vd-calendar-cell[data-cal-date="${session.selected}"]`);
            if (el) el.focus({ preventScroll: true });
        };
        if (cell && session.view === 'month') {
            if (key === 'ArrowLeft') { event.preventDefault(); moveSelection(-1); return; }
            if (key === 'ArrowRight') { event.preventDefault(); moveSelection(1); return; }
            if (key === 'ArrowUp') { event.preventDefault(); moveSelection(-7); return; }
            if (key === 'ArrowDown') { event.preventDefault(); moveSelection(7); return; }
            if (key === 'Enter' || key === ' ') { event.preventDefault(); openAppointmentEditor(session, { dateHint: cell.dataset.calDate }); return; }
        }
        const lower = key.length === 1 ? key.toLowerCase() : key;
        if (lower === 't' || key === 'Home') { event.preventDefault(); calendarGoToday(session); }
        else if (lower === 'n') { event.preventDefault(); openAppointmentEditor(session, { dateHint: session.selected }); }
        else if (lower === 'd') { event.preventDefault(); setCalendarView(session, 'day'); }
        else if (lower === 'w') { event.preventDefault(); setCalendarView(session, 'week'); }
        else if (lower === 'm') { event.preventDefault(); setCalendarView(session, 'month'); }
        else if (lower === 'a') { event.preventDefault(); setCalendarView(session, 'agenda'); }
        else if (lower === '/') { event.preventDefault(); const search = session.host.querySelector('[data-cal-search]'); if (search) search.focus(); }
        else if (key === 'ArrowLeft' || key === 'PageUp') { event.preventDefault(); shiftCalendarPeriod(session, -1); }
        else if (key === 'ArrowRight' || key === 'PageDown') { event.preventDefault(); shiftCalendarPeriod(session, 1); }
        else if (key === 'Escape' && session.query) { event.preventDefault(); const search = session.host.querySelector('[data-cal-search]'); if (search) search.value = ''; calendarSetQuery(session, ''); }
    }

    function wireCalendarShell(session) {
        const host = session.host;
        const shell = host.querySelector('.vd-calendar-shell');
        if (!shell) return;
        session.todayKey = isoDate(new Date());

        shell.addEventListener('click', async event => {
            const target = event.target;
            const closest = selector => target.closest(selector);
            const nav = closest('[data-cal-nav]');
            if (nav) { shiftCalendarPeriod(session, Number(nav.dataset.calNav) || 1); return; }
            if (closest('[data-cal-today]')) { calendarGoToday(session); return; }
            const viewButton = closest('[data-cal-view]');
            if (viewButton) { setCalendarView(session, viewButton.dataset.calView); return; }
            if (closest('[data-cal-sidebar-toggle]')) { toggleCalendarSidebar(session); return; }
            if (closest('[data-cal-jump]')) {
                const input = host.querySelector('[data-cal-jump-input]');
                if (!input) return;
                input.value = session.selected;
                try { if (typeof input.showPicker === 'function') input.showPicker(); else input.click(); } catch (_) { input.click(); }
                return;
            }
            if (closest('[data-cal-search-clear]')) { const search = host.querySelector('[data-cal-search]'); if (search) { search.value = ''; search.focus(); } calendarSetQuery(session, ''); return; }
            if (closest('[data-cal-retry]')) { loadCalendarAppointments(session, { initial: true }); return; }
            const miniNav = closest('[data-cal-mini-nav]');
            if (miniNav) {
                session.miniCursor = calAddMonths(session.miniCursor || session.cursor, Number(miniNav.dataset.calMiniNav) || 1);
                const mini = host.querySelector('[data-cal-mini]');
                if (mini) mini.innerHTML = calendarMiniMonthHTML(session);
                return;
            }
            const miniJump = closest('[data-cal-mini-jump]');
            if (miniJump) { calendarFocusDate(session, miniJump.dataset.calMiniJump, { view: session.view === 'agenda' ? 'month' : undefined }); return; }
            const miniDate = closest('[data-cal-mini-date]');
            if (miniDate) { calendarFocusDate(session, miniDate.dataset.calMiniDate, { view: session.view === 'agenda' ? 'day' : undefined }); return; }
            const openDay = closest('[data-cal-open-day]');
            if (openDay) { calendarFocusDate(session, openDay.dataset.calOpenDay, { view: 'day' }); return; }
            const more = closest('[data-cal-more]');
            if (more) { calendarFocusDate(session, more.dataset.calMore, { view: 'day' }); return; }
            const snackAction = closest('[data-cal-snack-action]');
            if (snackAction) { runCalendarSnackAction(session, snackAction); return; }
            if (closest('[data-cal-snack-close]')) { hideCalendarSnack(session); return; }
            const filter = closest('[data-cal-filter]');
            if (filter) {
                session.filters[filter.dataset.calFilter] = filter.checked;
                saveCalendarPrefs({ showCompleted: session.filters.completed, showCancelled: session.filters.cancelled });
                paintCalendar(session);
                return;
            }
            const add = closest('[data-cal-add]');
            if (add) { event.stopPropagation(); session.selected = add.dataset.calAdd; calendarMarkSelected(session); openAppointmentEditor(session, { dateHint: add.dataset.calAdd }); return; }
            const statusAction = closest('[data-cal-status-action]');
            if (statusAction) {
                const item = calendarFindAppointment(session, statusAction.dataset.apptId);
                if (item) updateAppointmentStatus(session, item, statusAction.dataset.calStatusAction);
                return;
            }
            const edit = closest('[data-cal-edit]');
            if (edit) { const item = calendarFindAppointment(session, edit.dataset.calEdit); if (item) openAppointmentEditor(session, { appointment: item }); return; }
            const remove = closest('[data-cal-delete]');
            if (remove) {
                const item = calendarFindAppointment(session, remove.dataset.calDelete);
                if (item && await confirmDialog(t('desktop.delete'), t('desktop.cal_delete_confirm'))) {
                    deleteCalendarAppointment(session, item).catch(err => session.snack({ type: 'error', message: err && err.message ? err.message : t('desktop.request_failed') }));
                }
                return;
            }
            const eventButton = closest('[data-appt-id]');
            if (eventButton && !closest('.vd-calendar-peek')) {
                const item = calendarFindAppointment(session, eventButton.dataset.apptId);
                if (item) openAppointmentPeek(session, item, eventButton);
                return;
            }
            if (closest('[data-cal-create]')) { openAppointmentEditor(session, { dateHint: session.selected }); return; }
            const slot = closest('.vd-calendar-hour-slot');
            if (slot) {
                const column = slot.closest('[data-cal-column]');
                if (column) { session.selected = column.dataset.calColumn; openAppointmentEditor(session, { dateHint: calendarSlotDate(column, slot, event.clientY) }); }
                return;
            }
            const cell = closest('.vd-calendar-cell[data-cal-date]');
            if (cell) {
                session.selected = cell.dataset.calDate;
                calendarMarkSelected(session);
                openAppointmentEditor(session, { dateHint: cell.dataset.calDate });
            }
        });

        shell.addEventListener('keydown', event => calendarHandleKeydown(session, event));

        const jumpInput = host.querySelector('[data-cal-jump-input]');
        if (jumpInput) jumpInput.addEventListener('change', () => { if (jumpInput.value) calendarFocusDate(session, jumpInput.value, { view: session.view === 'agenda' ? 'month' : undefined }); });

        const search = host.querySelector('[data-cal-search]');
        if (search) {
            let debounce = null;
            search.addEventListener('input', () => {
                clearTimeout(debounce);
                debounce = setTimeout(() => calendarSetQuery(session, search.value), 140);
            });
        }

        shell.addEventListener('scroll', event => {
            if (event.target && event.target.matches && event.target.matches('[data-cal-time-scroll]')) session.timeScrollTop = event.target.scrollTop;
        }, true);

        shell.addEventListener('dragstart', event => {
            const el = event.target.closest && event.target.closest('.vd-calendar-event[draggable="true"]');
            if (!el) return;
            session.dragId = el.dataset.apptId;
            el.classList.add('is-dragging');
            shell.classList.add('is-dragging');
            closeCalendarPeek(session);
            if (event.dataTransfer) { event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('text/plain', session.dragId); }
        });
        shell.addEventListener('dragend', () => {
            session.dragId = null;
            shell.classList.remove('is-dragging');
            shell.querySelectorAll('.vd-calendar-event.is-dragging').forEach(el => el.classList.remove('is-dragging'));
            calendarClearDropState(session);
        });
        shell.addEventListener('dragover', event => {
            if (!session.dragId) return;
            const column = event.target.closest('[data-cal-column]');
            const cell = column || event.target.closest('[data-cal-drop-date]');
            if (!cell) return;
            event.preventDefault();
            if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
            if (!cell.classList.contains('is-drop-target')) {
                shell.querySelectorAll('.is-drop-target').forEach(el => el.classList.remove('is-drop-target'));
                cell.classList.add('is-drop-target');
            }
            if (column) calendarShowDropGhost(session, column, event.clientY);
        });
        shell.addEventListener('dragleave', event => {
            const cell = event.target.closest && event.target.closest('[data-cal-drop-date]');
            if (cell && !cell.contains(event.relatedTarget)) { cell.classList.remove('is-drop-target'); cell.querySelectorAll('.vd-calendar-drop-ghost').forEach(el => el.remove()); }
        });
        shell.addEventListener('drop', event => {
            const id = session.dragId || (event.dataTransfer && event.dataTransfer.getData('text/plain'));
            const item = calendarFindAppointment(session, id);
            const dropTarget = event.target.closest('[data-cal-drop-date]');
            if (!item || !dropTarget) return;
            event.preventDefault();
            const target = calendarDropDate(dropTarget, event.clientY, item);
            calendarClearDropState(session);
            shell.classList.remove('is-dragging');
            session.dragId = null;
            if (target) updateAppointmentDateTime(session, item, target);
        });

        // Keep the current-time indicator and "today" markers fresh.
        session.timers.push(setInterval(() => calendarUpdateNowLine(session), 60000));

        // Refresh quietly when the desktop becomes visible again after a while.
        calendarListen(session, document, 'visibilitychange', () => {
            if (!document.hidden && session.loaded && Date.now() - session.lastLoadedAt > CAL_REFRESH_AFTER_MS) loadCalendarAppointments(session, { silent: true });
        });

        // Re-flow the month density when the window is resized.
        if (typeof ResizeObserver === 'function') {
            let resizeTimer = null;
            let lastWidth = shell.clientWidth;
            session.observer = new ResizeObserver(() => {
                clearTimeout(resizeTimer);
                resizeTimer = setTimeout(() => {
                    const width = shell.clientWidth;
                    const shrunk = width < lastWidth;
                    lastWidth = width;
                    if (!session.loaded) return;
                    if (shrunk && session.sidebarOpen && width < 640) { session.sidebarOpen = false; paintCalendar(session); return; }
                    if (session.view === 'month' && calendarMonthDensity(session) !== session.monthDensity) paintCalendar(session);
                }, 120);
            });
            session.observer.observe(shell);
        }
    }
