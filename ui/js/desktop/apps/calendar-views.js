    // Calendar view helpers: date math, formatting and pure HTML renderers for
    // the month grid, time grid (week/day), agenda list, mini month and skeletons.
    // Continuation of the shared Desktop IIFE; calendar.js owns state and wiring.

    const CAL_HOUR_HEIGHT = 56;
    const CAL_EVENT_MINUTES = 45;
    const CAL_STATUSES = ['upcoming', 'completed', 'cancelled', 'overdue'];

    function calendarLocale() {
        return String(window.SYSTEM_LANG || document.documentElement.lang || navigator.language || 'en');
    }

    function calFormat(date, options) {
        try { return date.toLocaleDateString(calendarLocale(), options); } catch (_) { return date.toLocaleDateString(undefined, options); }
    }

    function calFormatTime(date) {
        try { return date.toLocaleTimeString(calendarLocale(), { hour: '2-digit', minute: '2-digit' }); } catch (_) { return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' }); }
    }

    function isoDate(date) {
        const d = date instanceof Date ? date : new Date(date);
        const pad = n => String(n).padStart(2, '0');
        return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
    }

    function calParseISODate(value) {
        const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(String(value || ''));
        if (!match) return new Date(NaN);
        return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
    }

    function calStartOfDay(date) { const d = new Date(date); d.setHours(0, 0, 0, 0); return d; }
    function calAddDays(date, days) { const d = new Date(date); d.setDate(d.getDate() + days); return d; }
    function calAddMonths(date, months) { const d = new Date(date); const day = d.getDate(); d.setDate(1); d.setMonth(d.getMonth() + months); d.setDate(Math.min(day, new Date(d.getFullYear(), d.getMonth() + 1, 0).getDate())); return d; }
    function calSameDay(a, b) { return isoDate(a) === isoDate(b); }

    function calFirstWeekday() {
        try {
            const locale = new Intl.Locale(calendarLocale());
            const info = typeof locale.getWeekInfo === 'function' ? locale.getWeekInfo() : locale.weekInfo;
            if (info && info.firstDay >= 1 && info.firstDay <= 7) return info.firstDay % 7; // 0 = Sunday
        } catch (_) { /* fall back to Monday */ }
        return 1;
    }

    function calStartOfWeek(date) {
        const first = calFirstWeekday();
        const d = calStartOfDay(date);
        const diff = (d.getDay() - first + 7) % 7;
        return calAddDays(d, -diff);
    }

    function calendarWeekDays(date) {
        const start = calStartOfWeek(date);
        return Array.from({ length: 7 }, (_, i) => calAddDays(start, i));
    }

    function calIsoWeek(date) {
        const d = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
        const day = d.getUTCDay() || 7;
        d.setUTCDate(d.getUTCDate() + 4 - day);
        const yearStart = new Date(Date.UTC(d.getUTCFullYear(), 0, 1));
        return Math.ceil((((d - yearStart) / 86400000) + 1) / 7);
    }

    function calIsWeekend(date) { const day = date.getDay(); return day === 0 || day === 6; }

    function calendarDate(value) {
        const d = new Date(value);
        return Number.isNaN(d.getTime()) ? null : d;
    }

    function calMinutesOfDay(date) { return date.getHours() * 60 + date.getMinutes(); }

    function calRelativeDayLabel(date) {
        const today = calStartOfDay(new Date());
        const diff = Math.round((calStartOfDay(date) - today) / 86400000);
        if (diff === 0) return t('desktop.cal_today');
        if (diff === 1) return t('desktop.cal_tomorrow');
        if (diff === -1) return t('desktop.cal_yesterday');
        return '';
    }

    function calRelativeTime(date) {
        const diffMs = date.getTime() - Date.now();
        const abs = Math.abs(diffMs);
        let value, unit;
        if (abs < 60 * 60000) { value = Math.round(diffMs / 60000); unit = 'minute'; }
        else if (abs < 24 * 3600000) { value = Math.round(diffMs / 3600000); unit = 'hour'; }
        else if (abs < 30 * 86400000) { value = Math.round(diffMs / 86400000); unit = 'day'; }
        else if (abs < 365 * 86400000) { value = Math.round(diffMs / (30 * 86400000)); unit = 'month'; }
        else { value = Math.round(diffMs / (365 * 86400000)); unit = 'year'; }
        try {
            return new Intl.RelativeTimeFormat(calendarLocale(), { numeric: 'auto' }).format(value, unit);
        } catch (_) {
            return value === 0 ? t('desktop.cal_now') : `${value} ${unit}`;
        }
    }

    function normalizeCalendarAppointments(list) {
        return (Array.isArray(list) ? list : []).map(item => ({
            id: item.id,
            title: item.title || '',
            description: item.description || '',
            date_time: item.date_time || '',
            notification_at: item.notification_at || '',
            wake_agent: !!item.wake_agent,
            agent_instruction: item.agent_instruction || '',
            status: CAL_STATUSES.includes(item.status) ? item.status : 'upcoming',
            notified: !!item.notified,
            contact_ids: Array.isArray(item.contact_ids) ? item.contact_ids : [],
            participants: Array.isArray(item.participants) ? item.participants : [],
            created_at: item.created_at || '',
            updated_at: item.updated_at || ''
        })).filter(item => item.id && calendarDate(item.date_time))
            .sort((a, b) => new Date(a.date_time) - new Date(b.date_time));
    }

    function calendarStatusLabel(status) {
        const key = `desktop.cal_status_${status}`;
        const label = t(key);
        return label && label !== key ? label : status;
    }

    function calendarTimeLabel(value) {
        const date = calendarDate(value);
        return date ? calFormatTime(date) : '';
    }

    function calendarDateTimeLabel(value) {
        const date = calendarDate(value);
        if (!date) return '';
        return `${calFormat(date, { weekday: 'short', day: 'numeric', month: 'short' })} · ${calFormatTime(date)}`;
    }

    function calendarRangeLabel(view, cursor) {
        if (view === 'day') return calFormat(cursor, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
        if (view === 'week') {
            const days = calendarWeekDays(cursor);
            const first = days[0], last = days[6];
            if (first.getMonth() === last.getMonth()) return `${first.getDate()} – ${last.getDate()}. ${calFormat(last, { month: 'long', year: 'numeric' })}`;
            return `${calFormat(first, { day: 'numeric', month: 'short' })} – ${calFormat(last, { day: 'numeric', month: 'short', year: 'numeric' })}`;
        }
        if (view === 'agenda') return t('desktop.cal_agenda');
        return calFormat(cursor, { month: 'long', year: 'numeric' });
    }

    function calendarVisibleAppointments(session) {
        const query = String(session.query || '').trim().toLowerCase();
        return session.appointments.filter(item => {
            if (item.status === 'completed' && !session.filters.completed) return false;
            if (item.status === 'cancelled' && !session.filters.cancelled) return false;
            if (!query) return true;
            const haystack = [item.title, item.description, item.agent_instruction, ...(item.participants || []).map(p => p.name || '')].join(' ').toLowerCase();
            return haystack.includes(query);
        });
    }

    function calendarDayItems(items, date) {
        const key = isoDate(date);
        return items.filter(item => isoDate(new Date(item.date_time)) === key);
    }

    function calendarBadgesHTML(item, options) {
        const compact = options && options.compact;
        const parts = [];
        if (item.wake_agent) parts.push(`<span class="vd-calendar-badge vd-calendar-badge-agent" title="${esc(t('desktop.cal_wake_agent'))}">${iconMarkup('chat', 'A', 'vd-calendar-mini-icon', 11)}${compact ? '' : `<span>${esc(t('desktop.cal_agent_short'))}</span>`}</span>`);
        if (item.notification_at) parts.push(`<span class="vd-calendar-badge" title="${esc(t('desktop.cal_reminder'))}: ${esc(calendarDateTimeLabel(item.notification_at))}">${iconMarkup('bell', 'R', 'vd-calendar-mini-icon', 11)}</span>`);
        if (item.participants && item.participants.length) parts.push(`<span class="vd-calendar-badge" title="${esc(item.participants.map(p => p.name).filter(Boolean).join(', '))}">${iconMarkup('users', 'P', 'vd-calendar-mini-icon', 11)}${compact ? '' : `<span>${item.participants.length}</span>`}</span>`);
        return parts.length ? `<span class="vd-calendar-badges">${parts.join('')}</span>` : '';
    }

    function calendarEventTooltip(item) {
        const lines = [item.title, calendarDateTimeLabel(item.date_time), calendarStatusLabel(item.status)];
        if (item.description) lines.push(item.description.slice(0, 140));
        return lines.filter(Boolean).join('\n');
    }

    function calendarEventButtonHTML(item, options) {
        options = options || {};
        const date = calendarDate(item.date_time);
        const draggable = options.draggable !== false && item.status !== 'completed' && item.status !== 'cancelled';
        const style = options.style ? ` style="${esc(options.style)}"` : '';
        const classes = ['vd-calendar-event', `is-${item.status}`, options.variant ? `vd-calendar-event-${options.variant}` : ''].filter(Boolean).join(' ');
        return `<button type="button" class="${classes}" data-appt-id="${esc(item.id)}" draggable="${draggable ? 'true' : 'false'}" title="${esc(calendarEventTooltip(item))}"${style}>
            <span class="vd-calendar-event-dot" aria-hidden="true"></span>
            <span class="vd-calendar-event-time">${esc(date ? calFormatTime(date) : '')}</span>
            <span class="vd-calendar-event-title">${esc(item.title || t('desktop.cal_new_appointment'))}</span>
            ${calendarBadgesHTML(item, { compact: options.variant !== 'agenda' })}
        </button>`;
    }

    function calendarWeekdaysHTML(days) {
        return `<div class="vd-calendar-weekdays" aria-hidden="true">${days.map(day => `<span class="${calIsWeekend(day) ? 'is-weekend' : ''}">${esc(calFormat(day, { weekday: 'short' }))}</span>`).join('')}</div>`;
    }

    function calendarMonthCells(cursor) {
        const first = new Date(cursor.getFullYear(), cursor.getMonth(), 1);
        const start = calStartOfWeek(first);
        const lastOfMonth = new Date(cursor.getFullYear(), cursor.getMonth() + 1, 0);
        const spanDays = Math.round((calStartOfDay(lastOfMonth) - start) / 86400000) + 1;
        const rows = Math.max(5, Math.ceil(spanDays / 7));
        return { start, rows, cells: Array.from({ length: rows * 7 }, (_, i) => calAddDays(start, i)) };
    }

    function calendarMonthHTML(session, items) {
        const cursor = session.cursor;
        const { rows, cells } = calendarMonthCells(cursor);
        const today = new Date();
        const maxVisible = Math.max(1, Number(session.monthDensity) || 3);
        const weekdays = calendarWeekdaysHTML(calendarWeekDays(today));
        const body = cells.map((day, index) => {
            const dayItems = calendarDayItems(items, day);
            const key = isoDate(day);
            const classes = ['vd-calendar-cell'];
            if (day.getMonth() !== cursor.getMonth()) classes.push('is-other');
            if (calSameDay(day, today)) classes.push('is-today');
            if (calIsWeekend(day)) classes.push('is-weekend');
            if (key === session.selected) classes.push('is-selected');
            if (dayItems.length) classes.push('has-events');
            const visible = dayItems.slice(0, maxVisible);
            const hidden = dayItems.length - visible.length;
            const showMonth = day.getDate() === 1 || index === 0;
            const label = `${calFormat(day, { weekday: 'long', day: 'numeric', month: 'long' })}${dayItems.length ? ` · ${t('desktop.cal_event_count', { count: dayItems.length })}` : ''}`;
            return `<div class="${classes.join(' ')}" role="gridcell" tabindex="${key === session.selected ? '0' : '-1'}" data-cal-date="${key}" data-cal-drop-date="${key}" aria-label="${esc(label)}" aria-selected="${key === session.selected ? 'true' : 'false'}">
                <div class="vd-calendar-cell-head">
                    <button type="button" class="vd-calendar-daynum" data-cal-open-day="${key}" tabindex="-1" title="${esc(t('desktop.cal_open_day'))}">${showMonth ? `<small>${esc(calFormat(day, { month: 'short' }))}</small>` : ''}<b>${day.getDate()}</b></button>
                    <button type="button" class="vd-calendar-cell-add" data-cal-add="${key}" tabindex="-1" aria-label="${esc(t('desktop.cal_new_appointment'))}">${iconMarkup('plus', '+', 'vd-calendar-action-icon', 12)}</button>
                </div>
                <div class="vd-calendar-cell-events">
                    ${visible.map(item => calendarEventButtonHTML(item, { variant: 'chip' })).join('')}
                    ${hidden > 0 ? `<button type="button" class="vd-calendar-more" data-cal-more="${key}">${esc(t('desktop.cal_more_events', { count: hidden }))}</button>` : ''}
                </div>
            </div>`;
        }).join('');
        return `${weekdays}<div class="vd-calendar-month" role="grid" style="--vd-calendar-rows:${rows}">${body}</div>`;
    }

    function calendarLayoutColumns(dayItems) {
        // Assign overlapping timed events to side-by-side columns.
        const placed = [];
        dayItems.forEach(item => {
            const start = calMinutesOfDay(new Date(item.date_time));
            const end = start + CAL_EVENT_MINUTES;
            const overlapping = placed.filter(p => p.start < end && start < p.end);
            const used = new Set(overlapping.map(p => p.column));
            let column = 0;
            while (used.has(column)) column += 1;
            placed.push({ item, start, end, column });
        });
        // Determine the width for every cluster of overlapping events.
        placed.forEach(entry => {
            const cluster = placed.filter(p => p.start < entry.end && entry.start < p.end);
            entry.columns = Math.max(1, ...cluster.map(p => p.column + 1));
        });
        return placed;
    }

    function calendarTimedEventsHTML(dayItems) {
        return calendarLayoutColumns(dayItems).map(({ item, start, column, columns }) => {
            const top = (start / 60) * CAL_HOUR_HEIGHT;
            const height = Math.max(24, (CAL_EVENT_MINUTES / 60) * CAL_HOUR_HEIGHT - 2);
            const width = 100 / columns;
            const style = `top:${top.toFixed(1)}px;height:${height.toFixed(1)}px;left:calc(${(width * column).toFixed(2)}% + 2px);width:calc(${width.toFixed(2)}% - 4px)`;
            return calendarEventButtonHTML(item, { variant: 'timed', style });
        }).join('');
    }

    function calendarNowLineHTML(date) {
        const now = new Date();
        if (!calSameDay(now, date)) return '';
        const top = (calMinutesOfDay(now) / 60) * CAL_HOUR_HEIGHT;
        return `<div class="vd-calendar-now-line" data-cal-now style="top:${top.toFixed(1)}px"><span>${esc(calFormatTime(now))}</span></div>`;
    }

    function calendarTimeGridHTML(session, items, days) {
        const today = new Date();
        const heads = days.map(day => {
            const key = isoDate(day);
            const count = calendarDayItems(items, day).length;
            return `<button type="button" class="vd-calendar-day-head ${calSameDay(day, today) ? 'is-today' : ''} ${calIsWeekend(day) ? 'is-weekend' : ''}" data-cal-open-day="${key}" title="${esc(t('desktop.cal_open_day'))}">
                <span class="vd-calendar-day-head-name">${esc(calFormat(day, { weekday: 'short' }))}</span>
                <span class="vd-calendar-day-head-num">${day.getDate()}</span>
                ${count ? `<span class="vd-calendar-day-head-count">${count}</span>` : ''}
            </button>`;
        }).join('');
        const gutter = Array.from({ length: 24 }, (_, hour) => `<div class="vd-calendar-hour-label" style="height:${CAL_HOUR_HEIGHT}px"><span>${esc(calFormatTime(new Date(2000, 0, 1, hour, 0)))}</span></div>`).join('');
        const columns = days.map(day => {
            const key = isoDate(day);
            const dayItems = calendarDayItems(items, day);
            const slots = Array.from({ length: 24 }, (_, hour) => `<div class="vd-calendar-hour-slot" data-cal-hour="${hour}" style="height:${CAL_HOUR_HEIGHT}px"></div>`).join('');
            return `<div class="vd-calendar-day-column ${calSameDay(day, today) ? 'is-today' : ''} ${calIsWeekend(day) ? 'is-weekend' : ''}" data-cal-column="${key}" data-cal-drop-date="${key}" style="height:${CAL_HOUR_HEIGHT * 24}px">
                ${slots}
                <div class="vd-calendar-column-events">${calendarTimedEventsHTML(dayItems)}</div>
                ${calendarNowLineHTML(day)}
            </div>`;
        }).join('');
        return `<div class="vd-calendar-time-grid" style="--vd-calendar-days:${days.length};--vd-calendar-hour:${CAL_HOUR_HEIGHT}px">
            <div class="vd-calendar-time-head">
                <div class="vd-calendar-time-gutter-head"><span>${esc(t('desktop.cal_week_short', { week: calIsoWeek(days[0]) }))}</span></div>
                <div class="vd-calendar-time-head-days">${heads}</div>
            </div>
            <div class="vd-calendar-time-scroll" data-cal-time-scroll>
                <div class="vd-calendar-time-body">
                    <div class="vd-calendar-time-gutter">${gutter}</div>
                    <div class="vd-calendar-time-columns">${columns}</div>
                </div>
            </div>
        </div>`;
    }

    function calendarEmptyStateHTML(title, hint, actionKey) {
        return `<div class="vd-calendar-empty">
            <div class="vd-calendar-empty-art" aria-hidden="true">${iconMarkup('calendar', 'C', 'vd-calendar-empty-icon', 40)}</div>
            <strong>${esc(title)}</strong>
            ${hint ? `<p>${esc(hint)}</p>` : ''}
            ${actionKey ? `<button type="button" class="vd-button vd-button-primary" data-cal-create>${iconMarkup('plus', '+', 'vd-calendar-action-icon', 14)}<span>${esc(t(actionKey))}</span></button>` : ''}
        </div>`;
    }

    function calendarAgendaGroups(items, options) {
        options = options || {};
        const groups = new Map();
        items.forEach(item => {
            const date = calendarDate(item.date_time);
            if (!date) return;
            if (options.from && date < options.from) return;
            if (options.to && date >= options.to) return;
            const key = isoDate(date);
            if (!groups.has(key)) groups.set(key, { date: calStartOfDay(date), items: [] });
            groups.get(key).items.push(item);
        });
        return Array.from(groups.values()).sort((a, b) => a.date - b.date);
    }

    function calendarAgendaRowHTML(item) {
        const date = calendarDate(item.date_time);
        const canComplete = item.status === 'upcoming' || item.status === 'overdue';
        return `<div class="vd-calendar-agenda-row is-${item.status}" data-appt-row="${esc(item.id)}">
            <div class="vd-calendar-agenda-time"><b>${esc(date ? calFormatTime(date) : '')}</b><span>${esc(calRelativeTime(date))}</span></div>
            ${calendarEventButtonHTML(item, { variant: 'agenda', draggable: false })}
            <div class="vd-calendar-agenda-actions">
                ${canComplete ? `<button type="button" class="vd-calendar-icon-button" data-cal-status-action="completed" data-appt-id="${esc(item.id)}" title="${esc(t('desktop.cal_mark_complete'))}" aria-label="${esc(t('desktop.cal_mark_complete'))}">${iconMarkup('check', 'OK', 'vd-calendar-action-icon', 14)}</button>` : ''}
                <button type="button" class="vd-calendar-icon-button" data-cal-edit="${esc(item.id)}" title="${esc(t('desktop.cal_edit_appointment'))}" aria-label="${esc(t('desktop.cal_edit_appointment'))}">${iconMarkup('edit', 'E', 'vd-calendar-action-icon', 14)}</button>
            </div>
        </div>`;
    }

    function calendarAgendaHTML(session, items) {
        const query = String(session.query || '').trim();
        const from = query ? null : calStartOfDay(new Date());
        const groups = calendarAgendaGroups(items, { from });
        if (!groups.length) {
            if (query) return calendarEmptyStateHTML(t('desktop.cal_no_results'), t('desktop.cal_no_results_hint'));
            return calendarEmptyStateHTML(t('desktop.cal_empty_agenda'), t('desktop.cal_empty_hint'), 'desktop.cal_new_appointment');
        }
        const total = groups.reduce((sum, group) => sum + group.items.length, 0);
        const heading = query
            ? `<div class="vd-calendar-agenda-summary">${esc(t('desktop.cal_search_results', { query, count: total }))}</div>`
            : `<div class="vd-calendar-agenda-summary">${esc(t('desktop.cal_agenda_summary', { count: total }))}</div>`;
        return `<div class="vd-calendar-agenda">${heading}${groups.map(group => {
            const relative = calRelativeDayLabel(group.date);
            const today = calSameDay(group.date, new Date());
            return `<section class="vd-calendar-agenda-day ${today ? 'is-today' : ''}">
                <header class="vd-calendar-agenda-head">
                    <button type="button" class="vd-calendar-agenda-date" data-cal-open-day="${isoDate(group.date)}">
                        <span class="vd-calendar-agenda-daynum">${group.date.getDate()}</span>
                        <span class="vd-calendar-agenda-daymeta"><b>${esc(relative || calFormat(group.date, { weekday: 'long' }))}</b><span>${esc(calFormat(group.date, { day: 'numeric', month: 'long', year: 'numeric' }))}</span></span>
                    </button>
                </header>
                <div class="vd-calendar-agenda-rows">${group.items.map(calendarAgendaRowHTML).join('')}</div>
            </section>`;
        }).join('')}</div>`;
    }

    function calendarMiniMonthHTML(session) {
        const cursor = session.miniCursor || session.cursor;
        const { cells } = calendarMonthCells(cursor);
        const today = new Date();
        const busy = new Set(session.appointments.filter(item => item.status !== 'cancelled').map(item => isoDate(new Date(item.date_time))));
        const weekDays = calendarWeekDays(today);
        let rangeStart = null, rangeEnd = null;
        if (session.view === 'week') { rangeStart = isoDate(calStartOfWeek(session.cursor)); rangeEnd = isoDate(calAddDays(calStartOfWeek(session.cursor), 6)); }
        return `<div class="vd-calendar-mini">
            <div class="vd-calendar-mini-head">
                <button type="button" class="vd-calendar-icon-button" data-cal-mini-nav="-1" aria-label="${esc(t('desktop.cal_previous'))}">${iconMarkup('chevron-left', '<', 'vd-calendar-action-icon', 14)}</button>
                <button type="button" class="vd-calendar-mini-title" data-cal-mini-jump="${isoDate(new Date(cursor.getFullYear(), cursor.getMonth(), 1))}">${esc(calFormat(cursor, { month: 'long', year: 'numeric' }))}</button>
                <button type="button" class="vd-calendar-icon-button" data-cal-mini-nav="1" aria-label="${esc(t('desktop.cal_next'))}">${iconMarkup('chevron-right', '>', 'vd-calendar-action-icon', 14)}</button>
            </div>
            <div class="vd-calendar-mini-weekdays" aria-hidden="true">${weekDays.map(day => `<span>${esc(calFormat(day, { weekday: 'narrow' }))}</span>`).join('')}</div>
            <div class="vd-calendar-mini-grid">${cells.map(day => {
                const key = isoDate(day);
                const classes = ['vd-calendar-mini-day'];
                if (day.getMonth() !== cursor.getMonth()) classes.push('is-other');
                if (calSameDay(day, today)) classes.push('is-today');
                if (key === session.selected) classes.push('is-selected');
                if (busy.has(key)) classes.push('has-events');
                if (rangeStart && key >= rangeStart && key <= rangeEnd) classes.push('in-range');
                return `<button type="button" class="${classes.join(' ')}" data-cal-mini-date="${key}" aria-label="${esc(calFormat(day, { weekday: 'long', day: 'numeric', month: 'long' }))}"><span>${day.getDate()}</span></button>`;
            }).join('')}</div>
        </div>`;
    }

    function calendarSidebarItemHTML(item) {
        const date = calendarDate(item.date_time);
        return `<button type="button" class="vd-calendar-side-item is-${item.status}" data-appt-id="${esc(item.id)}" title="${esc(calendarEventTooltip(item))}">
            <span class="vd-calendar-event-dot" aria-hidden="true"></span>
            <span class="vd-calendar-side-item-body">
                <span class="vd-calendar-side-item-title">${esc(item.title)}</span>
                <span class="vd-calendar-side-item-meta">${esc(date ? `${calFormat(date, { weekday: 'short', day: 'numeric', month: 'short' })} · ${calFormatTime(date)}` : '')}</span>
            </span>
            ${calendarBadgesHTML(item, { compact: true })}
        </button>`;
    }

    function calendarSidebarAgendaHTML(session) {
        const now = new Date();
        const active = session.appointments.filter(item => item.status === 'upcoming' || item.status === 'overdue');
        const todayItems = calendarDayItems(active, now);
        const overdue = active.filter(item => item.status === 'overdue');
        const nextUp = active.filter(item => item.status === 'upcoming' && new Date(item.date_time) >= now)[0] || null;
        const weekEnd = calAddDays(calStartOfDay(now), 8);
        const upcoming = active.filter(item => item.status === 'upcoming' && !calSameDay(new Date(item.date_time), now) && new Date(item.date_time) >= now && new Date(item.date_time) < weekEnd).slice(0, 6);
        const section = (titleKey, list, emptyKey, extraClass) => `<section class="vd-calendar-side-section ${extraClass || ''}">
            <h4>${esc(t(titleKey))}${list.length ? `<span class="vd-calendar-side-count">${list.length}</span>` : ''}</h4>
            ${list.length ? list.map(calendarSidebarItemHTML).join('') : `<p class="vd-calendar-side-empty">${esc(t(emptyKey))}</p>`}
        </section>`;
        const hero = nextUp ? `<button type="button" class="vd-calendar-next-up" data-appt-id="${esc(nextUp.id)}">
            <span class="vd-calendar-next-up-label">${esc(t('desktop.cal_next_up'))}</span>
            <strong>${esc(nextUp.title)}</strong>
            <span class="vd-calendar-next-up-when">${esc(calRelativeTime(new Date(nextUp.date_time)))} · ${esc(calendarDateTimeLabel(nextUp.date_time))}</span>
        </button>` : '';
        return `${hero}
            ${overdue.length ? section('desktop.cal_overdue', overdue.slice(0, 5), 'desktop.cal_no_events', 'is-overdue') : ''}
            ${section('desktop.cal_today_panel', todayItems, 'desktop.clock_no_events')}
            ${section('desktop.cal_upcoming', upcoming, 'desktop.cal_empty_agenda')}`;
    }

    function calendarSkeletonHTML(view) {
        if (view === 'agenda') {
            return `<div class="vd-calendar-skeleton vd-calendar-skeleton-agenda" aria-hidden="true">${Array.from({ length: 4 }, () => '<div class="vd-calendar-skeleton-row"><i></i><i></i><i></i></div>').join('')}</div>`;
        }
        if (view === 'month') {
            return `<div class="vd-calendar-skeleton vd-calendar-skeleton-month" aria-hidden="true">${Array.from({ length: 35 }, () => '<i></i>').join('')}</div>`;
        }
        return `<div class="vd-calendar-skeleton vd-calendar-skeleton-grid" aria-hidden="true" style="--vd-calendar-days:${view === 'day' ? 1 : 7}">${Array.from({ length: view === 'day' ? 1 : 7 }, () => '<i></i>').join('')}</div>`;
    }
