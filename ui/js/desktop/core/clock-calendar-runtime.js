    let clockCalendarPanel = null;
    let clockCalendarOpen = false;
    let clockCalendarAnchor = null;
    let clockCalendarCursor = new Date(new Date().getFullYear(), new Date().getMonth(), 1);
    let clockCalendarSelected = isoDate(new Date());
    let clockCalendarAppointments = [];
    let clockCalendarLoading = false;
    let clockCalendarLoadError = false;
    let clockCalendarLoaded = false;
    let clockCalendarRequest = 0;
    let clockCalendarAbort = null;
    let clockCalendarMidnightTimer = 0;
    let clockCalendarKeyHandler = null;

    function ensureClockCalendarPanel() {
        if (clockCalendarPanel) return clockCalendarPanel;
        const panel = document.createElement('section');
        panel.id = 'vd-clock-popup';
        panel.className = 'vd-clock-popup vd-tray-popover vd-clock-calendar';
        panel.hidden = true;
        panel.tabIndex = -1;
        panel.setAttribute('role', 'dialog');
        panel.setAttribute('aria-modal', 'false');
        panel.setAttribute('aria-labelledby', 'vd-clock-calendar-title');
        document.body.appendChild(panel);
        panel.addEventListener('click', onClockCalendarClick);
        clockCalendarPanel = panel;
        return panel;
    }

    function clockCalendarDayItems(date) {
        return calendarDayItems(clockCalendarAppointments.filter(item => item.status !== 'cancelled' && item.status !== 'completed'), date);
    }

    function clockCalendarGridHTML() {
        const weekdays = calendarWeekDays(new Date()).map(day => `<span>${esc(calFormat(day, { weekday: 'short' }))}</span>`).join('');
        const today = new Date();
        const cells = calendarMonthCells(clockCalendarCursor).cells.map(day => {
            const key = isoDate(day);
            const items = clockCalendarDayItems(day);
            const classes = ['vd-clock-calendar-day'];
            if (day.getMonth() !== clockCalendarCursor.getMonth()) classes.push('is-other-month');
            if (key === isoDate(today)) classes.push('is-today');
            if (key === clockCalendarSelected) classes.push('is-selected');
            if (items.length) classes.push('has-events');
            return `<button type="button" class="${classes.join(' ')}" data-clock-day="${key}" aria-label="${esc(calFormat(day, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }))}" aria-pressed="${key === clockCalendarSelected ? 'true' : 'false'}" ${key === isoDate(today) ? 'aria-current="date"' : ''}>
                <span>${day.getDate()}</span>${items.length ? `<i aria-hidden="true"></i>` : ''}
            </button>`;
        }).join('');
        return `<div class="vd-clock-calendar-weekdays" aria-hidden="true">${weekdays}</div><div class="vd-clock-calendar-grid" role="group" aria-label="${esc(calendarRangeLabel('month', clockCalendarCursor))}">${cells}</div>`;
    }

    function clockCalendarAgendaHTML() {
        const selectedDate = calParseISODate(clockCalendarSelected);
        const dateLabel = calFormat(selectedDate, { weekday: 'long', day: 'numeric', month: 'long' });
        let body = '';
        if (clockCalendarLoading) body = `<p class="vd-clock-calendar-message" role="status">${esc(t('desktop.loading'))}</p>`;
        else if (clockCalendarLoadError) body = `<div class="vd-clock-calendar-error" role="alert"><p>${esc(t('desktop.clock_calendar_load_failed'))}</p><button type="button" class="vd-clock-calendar-retry" data-clock-action="retry">${esc(t('desktop.spotlight_retry'))}</button></div>`;
        else if (clockCalendarLoaded) {
            const items = clockCalendarDayItems(selectedDate);
            body = items.length ? `<ul class="vd-clock-calendar-events">${items.map(item => {
                const date = calendarDate(item.date_time);
                return `<li><time datetime="${esc(item.date_time)}">${esc(date ? calFormatTime(date) : '')}</time><span>${esc(item.title || '')}</span></li>`;
            }).join('')}</ul>` : `<p class="vd-clock-calendar-message">${esc(t('desktop.clock_no_events'))}</p>`;
        }
        return `<section class="vd-clock-calendar-agenda" aria-labelledby="vd-clock-calendar-agenda-title"><h3 id="vd-clock-calendar-agenda-title">${esc(dateLabel)}</h3>${body}</section>`;
    }

    function renderClockCalendar(focusSelector) {
        const panel = clockCalendarPanel;
        if (!panel) return;
        const active = document.activeElement;
        const keepFocus = !!(panel.contains && panel.contains(active));
        const activeSelector = keepFocus && active.dataset?.clockAction
            ? `[data-clock-action="${active.dataset.clockAction}"]`
            : keepFocus && active.dataset?.clockDay ? `[data-clock-day="${active.dataset.clockDay}"]` : '';
        panel.innerHTML = `<header class="vd-clock-calendar-header">
                <h2 id="vd-clock-calendar-title">${esc(calendarRangeLabel('month', clockCalendarCursor))}</h2>
                <div class="vd-clock-calendar-nav" role="group" aria-label="${esc(t('desktop.clock_calendar_month_navigation'))}">
                    <button type="button" data-clock-action="previous-month" aria-label="${esc(t('desktop.cal_previous'))}" title="${esc(t('desktop.cal_previous'))}">‹</button>
                    <button type="button" data-clock-action="today">${esc(t('desktop.cal_today'))}</button>
                    <button type="button" data-clock-action="next-month" aria-label="${esc(t('desktop.cal_next'))}" title="${esc(t('desktop.cal_next'))}">›</button>
                </div>
            </header>
            ${clockCalendarGridHTML()}
            ${clockCalendarAgendaHTML()}
            <button type="button" class="vd-clock-popup-open" data-clock-action="open-calendar">${esc(t('desktop.clock_open_calendar'))}</button>`;
        if (keepFocus) {
            const selector = focusSelector || activeSelector;
            (selector && panel.querySelector(selector) || panel.querySelector('[data-clock-action="today"]') || panel).focus();
        }
    }

    function scheduleClockCalendarMidnight() {
        if (clockCalendarMidnightTimer) window.clearTimeout(clockCalendarMidnightTimer);
        const now = new Date();
        const next = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, 0, 0, 0, 100);
        clockCalendarMidnightTimer = window.setTimeout(() => {
            clockCalendarMidnightTimer = 0;
            if (!clockCalendarOpen) return;
            const previousToday = isoDate(new Date(next.getFullYear(), next.getMonth(), next.getDate() - 1));
            const today = new Date();
            const todayKey = isoDate(today);
            if (clockCalendarSelected === previousToday) clockCalendarSelected = todayKey;
            if (clockCalendarCursor.getFullYear() === Number(previousToday.slice(0, 4)) && clockCalendarCursor.getMonth() === Number(previousToday.slice(5, 7)) - 1) {
                clockCalendarCursor = new Date(today.getFullYear(), today.getMonth(), 1);
            }
            renderClockCalendar();
            scheduleClockCalendarMidnight();
        }, Math.max(100, next.getTime() - now.getTime()));
    }

    async function loadClockCalendarAppointments() {
        if (!clockCalendarOpen) return;
        clockCalendarAbort?.abort();
        const controller = new AbortController();
        clockCalendarAbort = controller;
        const request = ++clockCalendarRequest;
        clockCalendarLoading = true;
        clockCalendarLoaded = false;
        clockCalendarLoadError = false;
        renderClockCalendar();
        try {
            const appointments = await api('/api/desktop/integrations/appointments?status=all', { signal: controller.signal });
            if (!clockCalendarOpen || controller.signal.aborted || request !== clockCalendarRequest) return;
            clockCalendarAppointments = normalizeCalendarAppointments(appointments);
            clockCalendarLoaded = true;
        } catch (_) {
            if (!clockCalendarOpen || controller.signal.aborted || request !== clockCalendarRequest) return;
            clockCalendarLoadError = true;
        } finally {
            if (clockCalendarOpen && request === clockCalendarRequest && !controller.signal.aborted) {
                clockCalendarLoading = false;
                renderClockCalendar();
            }
        }
    }

    function onClockCalendarClick(event) {
        const target = event.target?.closest?.('[data-clock-action], [data-clock-day]');
        if (!target) return;
        if (target.dataset.clockDay) {
            clockCalendarSelected = target.dataset.clockDay;
            const selected = calParseISODate(clockCalendarSelected);
            if (!Number.isNaN(selected.getTime()) && (selected.getMonth() !== clockCalendarCursor.getMonth() || selected.getFullYear() !== clockCalendarCursor.getFullYear())) {
                clockCalendarCursor = new Date(selected.getFullYear(), selected.getMonth(), 1);
            }
            renderClockCalendar(`[data-clock-day="${clockCalendarSelected}"]`);
            return;
        }
        const action = target.dataset.clockAction;
        if (action === 'previous-month' || action === 'next-month') {
            clockCalendarCursor = calAddMonths(clockCalendarCursor, action === 'previous-month' ? -1 : 1);
            renderClockCalendar(`[data-clock-action="${action}"]`);
        } else if (action === 'today') {
            const today = new Date();
            clockCalendarCursor = new Date(today.getFullYear(), today.getMonth(), 1);
            clockCalendarSelected = isoDate(today);
            renderClockCalendar('[data-clock-action="today"]');
        } else if (action === 'retry') {
            loadClockCalendarAppointments();
        } else if (action === 'open-calendar') {
            closeClockCalendarPopup();
            openApp('calendar');
        }
    }

    function closeClockCalendarPopup(restoreFocus = false) {
        if (!clockCalendarPanel) return;
        clockCalendarPanel.hidden = true;
        clockCalendarOpen = false;
        clockCalendarRequest++;
        clockCalendarAbort?.abort();
        clockCalendarAbort = null;
        if (clockCalendarMidnightTimer) window.clearTimeout(clockCalendarMidnightTimer);
        clockCalendarMidnightTimer = 0;
        clockCalendarAnchor?.setAttribute('aria-expanded', 'false');
        if (clockCalendarKeyHandler) document.removeEventListener('keydown', clockCalendarKeyHandler, true);
        clockCalendarKeyHandler = null;
        if (restoreFocus && clockCalendarAnchor?.isConnected) clockCalendarAnchor.focus();
        clockCalendarAnchor = null;
    }

    function openClockCalendarPopup(anchor) {
        closeClockCalendarPopup();
        const panel = ensureClockCalendarPanel();
        clockCalendarOpen = true;
        clockCalendarAnchor = anchor || document.activeElement;
        const now = new Date();
        clockCalendarCursor = new Date(now.getFullYear(), now.getMonth(), 1);
        clockCalendarSelected = isoDate(now);
        clockCalendarAppointments = [];
        clockCalendarLoading = false;
        clockCalendarLoadError = false;
        clockCalendarLoaded = false;
        panel.hidden = false;
        clockCalendarAnchor?.setAttribute('aria-expanded', 'true');
        placeTrayPopover(panel, anchor || clockCalendarAnchor);
        renderClockCalendar();
        clockCalendarKeyHandler = event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            closeClockCalendarPopup(true);
        };
        document.addEventListener('keydown', clockCalendarKeyHandler, true);
        panel.querySelector('[data-clock-action="previous-month"]')?.focus();
        scheduleClockCalendarMidnight();
        loadClockCalendarAppointments();
    }
