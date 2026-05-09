    async function renderCalendar(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.calView = host.dataset.calView || 'month';
        host.dataset.calDate = host.dataset.calDate || isoDate(new Date());
        const activeDate = new Date(host.dataset.calDate + 'T12:00:00');
        const view = host.dataset.calView;
        host.innerHTML = `<div class="vd-calendar-shell">
            <header class="vd-calendar-command">
                <div class="vd-calendar-titlebar">
                    <div class="vd-calendar-nav">
                        <button class="vd-calendar-icon-button" type="button" data-cal-nav="prev" title="${esc(t('desktop.cal_previous'))}">${iconMarkup('chevron-left', 'L', 'vd-calendar-action-icon', 15)}</button>
                        <button class="vd-calendar-icon-button" type="button" data-cal-nav="next" title="${esc(t('desktop.cal_next'))}">${iconMarkup('chevron-right', 'R', 'vd-calendar-action-icon', 15)}</button>
                    </div>
                    <div class="vd-calendar-heading">
                        <strong>${esc(calendarRangeLabel(activeDate, view))}</strong>
                        <span>${esc(t('desktop.cal_drag_hint'))}</span>
                    </div>
                </div>
                <div class="vd-calendar-actions">
                    <button class="vd-button vd-button-primary" type="button" data-cal-create>${iconMarkup('calendar', 'C', 'vd-calendar-action-icon', 15)}<span>${esc(t('desktop.cal_new'))}</span></button>
                    <button class="vd-button" type="button" data-cal-today>${iconMarkup('calendar', 'C', 'vd-calendar-action-icon', 15)}<span>${esc(t('desktop.cal_today'))}</span></button>
                    <div class="vd-calendar-view-switch" role="group" aria-label="${esc(t('desktop.menu_view'))}">
                        ${['month','week','day'].map(item => `<button type="button" data-cal-view="${item}" class="${view === item ? 'active' : ''}">${esc(t('desktop.cal_' + item))}</button>`).join('')}
                    </div>
                </div>
            </header>
            <div class="vd-calendar-stage">
                <div class="vd-calendar-body" data-cal-body>${esc(t('desktop.loading'))}</div>
                <aside class="vd-calendar-sidebar" data-cal-sidebar>${esc(t('desktop.loading'))}</aside>
            </div>
        </div>`;
        const showCalendarContextMenu = (event, appointments, render) => {
            const apptEl = event.target.closest('[data-appt-id]');
            const cellEl = event.target.closest('[data-cal-date]');
            if (!apptEl && !cellEl) return false;
            const appt = apptEl ? appointments.find(item => item.id === apptEl.dataset.apptId) : null;
            const date = cellEl ? cellEl.dataset.calDate : isoDate(activeDate);
            const items = [
                appt
                    ? { labelKey: 'desktop.launchpad_edit', icon: 'edit', action: () => openAppointmentModal(host, appt, '', render) }
                    : { labelKey: 'desktop.cal_new_appointment', icon: 'calendar', action: () => openAppointmentModal(host, null, date, render) }
            ];
            if (appt && (appt.status === 'upcoming' || appt.status === 'overdue')) {
                items.push(
                    { labelKey: 'desktop.cal_mark_complete', icon: 'check-square', action: async () => { await updateAppointmentStatus(appt, 'completed', render); } },
                    { labelKey: 'desktop.cal_cancel_appointment', icon: 'x', action: async () => { await updateAppointmentStatus(appt, 'cancelled', render); } }
                );
            }
            items.push({ separator: true }, { labelKey: 'desktop.context_refresh', icon: 'refresh', action: render });
            showContextMenu(event.clientX, event.clientY, items);
            return true;
        };
        wireContextMenuBoundary(host);
        const render = async () => {
            const appointments = normalizeCalendarAppointments(await api('/api/appointments?status=all'));
            const body = host.querySelector('.vd-calendar-body');
            const sidebar = host.querySelector('[data-cal-sidebar]');
            body.innerHTML = host.dataset.calView === 'month' ? calendarMonthHTML(activeDate, appointments) : calendarAgendaHTML(activeDate, appointments, host.dataset.calView);
            sidebar.innerHTML = calendarSidebarHTML(activeDate, appointments);
            wireCalendarBody(host, appointments, render);
            body.oncontextmenu = event => {
                if (showCalendarContextMenu(event, appointments, render)) event.preventDefault();
            };
            sidebar.oncontextmenu = event => {
                if (showCalendarContextMenu(event, appointments, render)) event.preventDefault();
            };
        };
        host.querySelectorAll('[data-cal-view]').forEach(btn => btn.addEventListener('click', () => { host.dataset.calView = btn.dataset.calView; renderCalendar(id); }));
        host.querySelector('[data-cal-today]').addEventListener('click', () => { host.dataset.calDate = isoDate(new Date()); renderCalendar(id); });
        host.querySelector('[data-cal-create]').addEventListener('click', () => openAppointmentModal(host, null, isoDate(activeDate), render));
        host.querySelectorAll('[data-cal-nav]').forEach(btn => btn.addEventListener('click', () => {
            const delta = btn.dataset.calNav === 'next' ? 1 : -1;
            if (host.dataset.calView === 'month') activeDate.setMonth(activeDate.getMonth() + delta);
            else activeDate.setDate(activeDate.getDate() + delta * (host.dataset.calView === 'week' ? 7 : 1));
            host.dataset.calDate = isoDate(activeDate);
            renderCalendar(id);
        }));
        setCalendarMenus(id, host, activeDate, render);
        try { await render(); } catch (err) { host.querySelector('.vd-calendar-body').innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`; }
    }

    function setCalendarMenus(id, host, activeDate, render) {
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'new-appointment', labelKey: 'desktop.cal_new_appointment', icon: 'calendar', shortcut: 'Ctrl+N', action: () => openAppointmentModal(host, null, isoDate(activeDate), render) }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'today', labelKey: 'desktop.cal_today', icon: 'calendar', action: () => { host.dataset.calDate = isoDate(new Date()); renderCalendar(id); } },
                    { id: 'refresh', labelKey: 'desktop.context_refresh', icon: 'refresh', shortcut: 'F5', action: render }
                ]
            }
        ]);
    }

    function normalizeCalendarAppointments(appointments) {
        return (appointments || []).map(item => Object.assign({
            title: '',
            description: '',
            status: 'upcoming',
            participants: [],
            contact_ids: []
        }, item || {})).sort((left, right) => calendarDate(left).getTime() - calendarDate(right).getTime());
    }

    function calendarDate(appointment) {
        const d = new Date((appointment && appointment.date_time) || Date.now());
        return Number.isNaN(d.getTime()) ? new Date() : d;
    }

    function calendarRangeLabel(activeDate, view) {
        if (view === 'day') return activeDate.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });
        if (view === 'week') {
            const days = calendarWeekDays(activeDate);
            const first = days[0].toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
            const last = days[6].toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
            return `${first} - ${last}`;
        }
        return activeDate.toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
    }

    function calendarWeekDays(activeDate) {
        return Array.from({ length: 7 }, (_, i) => {
            const d = new Date(activeDate);
            d.setDate(activeDate.getDate() - ((activeDate.getDay() + 6) % 7) + i);
            return d;
        });
    }

    function calendarDayItems(appointments, date) {
        const key = isoDate(date);
        return appointments.filter(a => String(a.date_time || '').startsWith(key));
    }

    function calendarStatusLabel(status) {
        const key = 'desktop.cal_status_' + (status || 'upcoming');
        const label = t(key);
        return label === key ? String(status || '') : label;
    }

    function calendarTimeLabel(value) {
        const d = new Date(value);
        if (Number.isNaN(d.getTime())) return '';
        return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
    }

    function calendarDateTimeLabel(value) {
        const d = new Date(value);
        if (Number.isNaN(d.getTime())) return '';
        return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
    }

    function calendarEventButtonHTML(appointment, mode) {
        const status = esc(appointment.status || 'upcoming');
        const participants = (appointment.participants || []).length;
        const time = calendarTimeLabel(appointment.date_time);
        return `<button type="button" draggable="true" class="vd-calendar-event ${status} ${mode || ''}" data-appt-id="${esc(appointment.id)}" title="${esc(appointment.title)}">
            <span class="vd-calendar-event-time">${esc(time)}</span>
            <span class="vd-calendar-event-title">${esc(appointment.title)}</span>
            <span class="vd-calendar-event-badges">${appointment.wake_agent ? iconMarkup('agent', 'A', 'vd-calendar-mini-icon', 12) : ''}${participants ? `<em>${esc(String(participants))}</em>` : ''}</span>
        </button>`;
    }

    function calendarMonthHTML(activeDate, appointments) {
        const first = new Date(activeDate.getFullYear(), activeDate.getMonth(), 1);
        const start = new Date(first);
        start.setDate(first.getDate() - ((first.getDay() + 6) % 7));
        const today = isoDate(new Date());
        const cells = Array.from({ length: 42 }, (_, i) => { const d = new Date(start); d.setDate(start.getDate() + i); return d; });
        const weekdays = calendarWeekDays(new Date()).map(d => `<span>${esc(d.toLocaleDateString(undefined, { weekday: 'short' }))}</span>`).join('');
        return `<div class="vd-calendar-month-wrap">
            <div class="vd-calendar-weekdays">${weekdays}</div>
            <div class="vd-calendar-month">${cells.map(d => {
                const key = isoDate(d);
                const dayItems = calendarDayItems(appointments, d);
                const hidden = Math.max(0, dayItems.length - 3);
                return `<div role="button" tabindex="0" class="vd-calendar-cell ${d.getMonth() !== activeDate.getMonth() ? 'muted' : ''} ${key === today ? 'today' : ''}" data-cal-date="${key}" data-cal-drop-date="${key}">
                    <div class="vd-calendar-cell-head"><span>${d.getDate()}</span>${dayItems.length ? `<em>${dayItems.length}</em>` : ''}</div>
                    <div class="vd-calendar-cell-events">${dayItems.slice(0, 3).map(a => calendarEventButtonHTML(a, 'compact')).join('')}${hidden ? `<button type="button" class="vd-calendar-more" data-cal-date="${key}">${esc(t('desktop.cal_more_events')).replace('{{count}}', String(hidden))}</button>` : ''}</div>
                </div>`;
            }).join('')}</div>
        </div>`;
    }

    function calendarAgendaHTML(activeDate, appointments, view) {
        const days = view === 'week' ? calendarWeekDays(activeDate) : [activeDate];
        const hours = Array.from({ length: 17 }, (_, i) => i + 6);
        return `<div class="vd-calendar-time-grid ${esc(view)}" style="--vd-calendar-days:${days.length}">
            <div class="vd-calendar-time-corner">${esc(t('desktop.cal_schedule'))}</div>
            ${days.map(day => `<div class="vd-calendar-day-head ${isoDate(day) === isoDate(new Date()) ? 'today' : ''}"><strong>${esc(day.toLocaleDateString(undefined, { weekday: 'short' }))}</strong><span>${esc(day.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }))}</span></div>`).join('')}
            ${hours.map(hour => `<div class="vd-calendar-time-label">${String(hour).padStart(2, '0')}:00</div>${days.map(day => {
                const key = `${isoDate(day)}T${String(hour).padStart(2, '0')}:00`;
                const dayItems = calendarDayItems(appointments, day).filter(a => calendarDate(a).getHours() === hour);
                return `<div class="vd-calendar-hour" data-cal-date="${key}" data-cal-drop-date="${key}">
                    ${dayItems.map(a => calendarEventButtonHTML(a, 'wide')).join('') || `<span class="vd-calendar-empty-slot">${esc(t('desktop.cal_no_events'))}</span>`}
                </div>`;
            }).join('')}`).join('')}
        </div>`;
    }

    function calendarSidebarHTML(activeDate, appointments) {
        const now = new Date();
        const todayItems = calendarDayItems(appointments, new Date());
        const upcoming = appointments.filter(a => calendarDate(a) >= now && a.status !== 'cancelled' && a.status !== 'completed').slice(0, 8);
        const overdue = appointments.filter(a => (a.status === 'overdue') || (a.status === 'upcoming' && calendarDate(a) < now));
        return `<section class="vd-calendar-side-section">
            <div class="vd-calendar-side-title"><span>${esc(t('desktop.cal_today_panel'))}</span><strong>${todayItems.length}</strong></div>
            <div class="vd-calendar-side-list">${todayItems.length ? todayItems.map(a => calendarSidebarItemHTML(a)).join('') : `<p>${esc(t('desktop.cal_no_events'))}</p>`}</div>
        </section>
        <section class="vd-calendar-side-section">
            <div class="vd-calendar-side-title"><span>${esc(t('desktop.cal_upcoming'))}</span><strong>${upcoming.length}</strong></div>
            <div class="vd-calendar-side-list">${upcoming.length ? upcoming.map(a => calendarSidebarItemHTML(a)).join('') : `<p>${esc(t('desktop.cal_no_events'))}</p>`}</div>
        </section>
        <section class="vd-calendar-side-section compact">
            <div class="vd-calendar-side-title overdue"><span>${esc(t('desktop.cal_overdue'))}</span><strong>${overdue.length}</strong></div>
        </section>`;
    }

    function calendarSidebarItemHTML(appointment) {
        const participants = (appointment.participants || []).map(p => p.name).filter(Boolean).slice(0, 2).join(', ');
        return `<button type="button" class="vd-calendar-side-item ${esc(appointment.status || 'upcoming')}" data-appt-id="${esc(appointment.id)}">
            <strong>${esc(calendarDateTimeLabel(appointment.date_time))}</strong>
            <span>${esc(appointment.title)}</span>
            ${participants ? `<small>${esc(participants)}</small>` : ''}
            ${appointment.wake_agent ? `<em>${esc(t('desktop.cal_agent_instruction'))}</em>` : ''}
        </button>`;
    }

    function wireCalendarBody(host, appointments, reload) {
        const root = host.querySelector('.vd-calendar-stage');
        if (!root) return;
        root.querySelectorAll('[data-cal-date]').forEach(cell => {
            cell.addEventListener('click', event => {
                if (event.target.closest('[data-appt-id]')) return;
                openAppointmentModal(host, null, cell.dataset.calDate, reload);
            });
            cell.addEventListener('keydown', event => {
                if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    openAppointmentModal(host, null, cell.dataset.calDate, reload);
                }
            });
        });
        root.querySelectorAll('[data-appt-id]').forEach(btn => {
            const appointment = appointments.find(a => a.id === btn.dataset.apptId);
            btn.addEventListener('click', event => {
                event.stopPropagation();
                if (appointment) openAppointmentModal(host, appointment, '', reload);
            });
            btn.addEventListener('dragstart', event => {
                event.dataTransfer.setData('text/plain', btn.dataset.apptId || '');
                event.dataTransfer.effectAllowed = 'move';
                btn.classList.add('dragging');
            });
            btn.addEventListener('dragend', () => btn.classList.remove('dragging'));
        });
        root.querySelectorAll('[data-cal-drop-date]').forEach(zone => {
            zone.addEventListener('dragover', event => {
                event.preventDefault();
                zone.classList.add('drop-target');
            });
            zone.addEventListener('dragleave', () => zone.classList.remove('drop-target'));
            zone.addEventListener('drop', async event => {
                event.preventDefault();
                zone.classList.remove('drop-target');
                const id = event.dataTransfer.getData('text/plain');
                const appointment = appointments.find(a => a.id === id);
                if (appointment) await updateAppointmentDateTime(appointment, zone.dataset.calDropDate, reload);
            });
        });
    }

    async function updateAppointmentDateTime(appointment, dateHint, reload) {
        const previous = calendarDate(appointment);
        let next;
        if (String(dateHint || '').includes('T')) {
            next = new Date(dateHint);
            next.setMinutes(previous.getMinutes(), 0, 0);
        } else {
            next = new Date(`${dateHint}T${String(previous.getHours()).padStart(2, '0')}:${String(previous.getMinutes()).padStart(2, '0')}:00`);
        }
        if (Number.isNaN(next.getTime())) return;
        await plannerJSON('/api/appointments/' + encodeURIComponent(appointment.id), 'PUT', { date_time: next.toISOString() });
        await reload();
    }

    async function updateAppointmentStatus(appointment, status, reload) {
        await plannerJSON('/api/appointments/' + encodeURIComponent(appointment.id), 'PUT', { status });
        await reload();
    }

    function calendarOptionalDateTime(value) {
        return value ? new Date(value).toISOString() : '';
    }

    function shiftCalendarDate(value, repeat, amount) {
        const date = new Date(value);
        if (repeat === 'daily') date.setDate(date.getDate() + amount);
        if (repeat === 'weekly') date.setDate(date.getDate() + amount * 7);
        if (repeat === 'monthly') date.setMonth(date.getMonth() + amount);
        return date.toISOString();
    }

    async function createRecurringAppointments(payload, repeat, count) {
        const total = Math.max(1, Math.min(Number(count) || 1, 30));
        for (let i = 0; i < total; i++) {
            const item = Object.assign({}, payload, {
                date_time: shiftCalendarDate(payload.date_time, repeat, i)
            });
            if (payload.notification_at) item.notification_at = shiftCalendarDate(payload.notification_at, repeat, i);
            await plannerJSON('/api/appointments', 'POST', item);
        }
    }

    function openAppointmentModal(host, appointment, dateHint, reload) {
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop';
        const initial = appointment || { title: '', description: '', status: 'upcoming', date_time: dateHint ? fromLocalDateTime(dateHint.includes('T') ? dateHint : dateHint + 'T09:00') : new Date().toISOString(), wake_agent: false };
        const participants = (initial.participants || []).map(p => p.name).filter(Boolean).join(', ');
        overlay.innerHTML = `<form class="vd-modal vd-calendar-modal"><div class="vd-modal-title">${esc(t(appointment ? 'desktop.cal_edit_appointment' : 'desktop.cal_new_appointment'))}</div>
            <div class="vd-calendar-modal-grid">
                <label><span>${esc(t('desktop.cal_title'))}</span><input name="title" class="vd-modal-input" value="${esc(initial.title)}"></label>
                <label><span>${esc(t('desktop.cal_date_time'))}</span><input name="date_time" class="vd-modal-input" type="datetime-local" value="${esc(dateTimeLocalValue(initial.date_time))}"></label>
                <label><span>${esc(t('desktop.cal_reminder'))}</span><input name="notification_at" class="vd-modal-input" type="datetime-local" value="${esc(initial.notification_at ? dateTimeLocalValue(initial.notification_at) : '')}"></label>
                <label><span>${esc(t('desktop.cal_status'))}</span><select name="status" class="vd-modal-input">${['upcoming','overdue','completed','cancelled'].map(status => `<option value="${status}" ${initial.status === status ? 'selected' : ''}>${esc(calendarStatusLabel(status))}</option>`).join('')}</select></label>
            </div>
            <label class="vd-calendar-modal-block"><span>${esc(t('desktop.cal_description'))}</span><textarea name="description" class="vd-modal-input">${esc(initial.description || '')}</textarea></label>
            <label class="vd-check vd-calendar-wake"><input name="wake_agent" type="checkbox" ${initial.wake_agent ? 'checked' : ''}>${esc(t('desktop.cal_notification'))}</label>
            <label class="vd-calendar-modal-block"><span>${esc(t('desktop.cal_agent_instruction'))}</span><textarea name="agent_instruction" class="vd-modal-input">${esc(initial.agent_instruction || '')}</textarea></label>
            ${participants ? `<div class="vd-calendar-participants"><strong>${esc(t('desktop.cal_participants'))}</strong><span>${esc(participants)}</span></div>` : ''}
            ${appointment ? '' : `<div class="vd-calendar-recurring">
                <label><span>${esc(t('desktop.cal_recurring'))}</span><select name="repeat" class="vd-modal-input">
                    <option value="none">${esc(t('desktop.cal_repeat_none'))}</option>
                    <option value="daily">${esc(t('desktop.cal_repeat_daily'))}</option>
                    <option value="weekly">${esc(t('desktop.cal_repeat_weekly'))}</option>
                    <option value="monthly">${esc(t('desktop.cal_repeat_monthly'))}</option>
                </select></label>
                <label><span>${esc(t('desktop.cal_repeat_count'))}</span><input name="repeat_count" class="vd-modal-input" type="number" min="1" max="30" value="1"></label>
            </div>`}
            ${appointment && (initial.status === 'upcoming' || initial.status === 'overdue') ? `<div class="vd-calendar-status-actions">
                <button type="button" class="vd-button" data-cal-status-action="completed">${iconMarkup('check-square', 'C', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.cal_mark_complete'))}</span></button>
                <button type="button" class="vd-button" data-cal-status-action="cancelled">${iconMarkup('x', 'X', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.cal_cancel_appointment'))}</span></button>
            </div>` : ''}
            <div class="vd-modal-actions">${appointment ? `<button type="button" class="vd-button" data-delete>${iconMarkup('trash', 'X', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.delete'))}</span></button>` : ''}<button type="button" class="vd-button" data-cancel>${iconMarkup('x', 'X', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.cancel'))}</span></button><button class="vd-button vd-button-primary">${iconMarkup('save', 'S', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.save'))}</span></button></div></form>`;
        document.body.appendChild(overlay);
        const close = () => overlay.remove();
        overlay.querySelector('[data-cancel]').addEventListener('click', close);
        overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
        const del = overlay.querySelector('[data-delete]');
        if (del) del.addEventListener('click', async () => { if (await confirmDialog(t('desktop.cal_delete_confirm'), appointment.title)) { await api('/api/appointments/' + encodeURIComponent(appointment.id), { method: 'DELETE' }); close(); await reload(); } });
        overlay.querySelectorAll('[data-cal-status-action]').forEach(btn => btn.addEventListener('click', async () => {
            await updateAppointmentStatus(appointment, btn.dataset.calStatusAction, reload);
            close();
        }));
        overlay.querySelector('form').addEventListener('submit', async event => {
            event.preventDefault();
            const form = event.currentTarget;
            const payload = { title: form.title.value.trim(), date_time: fromLocalDateTime(form.date_time.value), notification_at: calendarOptionalDateTime(form.notification_at.value), description: form.description.value, status: form.status.value, wake_agent: form.wake_agent.checked, agent_instruction: form.agent_instruction.value.trim() };
            if (!payload.title) return;
            if (appointment) await plannerJSON('/api/appointments/' + encodeURIComponent(appointment.id), 'PUT', payload);
            else if (form.repeat.value !== 'none' && Number(form.repeat_count.value) > 1) await createRecurringAppointments(payload, form.repeat.value, form.repeat_count.value);
            else await plannerJSON('/api/appointments', 'POST', payload);
            close();
            await reload();
        });
    }
