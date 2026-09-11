    // Calendar appointment editor, quick-peek popover and recurring creation.
    // Continuation of the shared Desktop IIFE; calendar.js owns the session state.

    const CAL_REMINDER_PRESETS = ['none', '0', '15', '60', '1440', 'custom'];
    const CAL_REPEAT_LIMIT = 30;

    function calendarReminderPreset(appointment) {
        if (!appointment || !appointment.notification_at) return 'none';
        const start = calendarDate(appointment.date_time);
        const reminder = calendarDate(appointment.notification_at);
        if (!start || !reminder) return 'custom';
        const minutes = Math.round((start - reminder) / 60000);
        return ['0', '15', '60', '1440'].includes(String(minutes)) ? String(minutes) : 'custom';
    }

    function calendarReminderLabel(preset) {
        const keys = { none: 'desktop.cal_reminder_none', '0': 'desktop.cal_reminder_at_time', '15': 'desktop.cal_reminder_15m', '60': 'desktop.cal_reminder_1h', '1440': 'desktop.cal_reminder_1d', custom: 'desktop.cal_reminder_custom' };
        return t(keys[preset] || keys.none);
    }

    function calendarReminderFromPreset(preset, start, customValue) {
        if (preset === 'none' || !start) return '';
        if (preset === 'custom') {
            const custom = customValue ? fromLocalDateTime(customValue) : '';
            return custom || '';
        }
        return new Date(start.getTime() - Number(preset) * 60000).toISOString();
    }

    function calendarInputParts(date) {
        const pad = n => String(n).padStart(2, '0');
        return { date: isoDate(date), time: `${pad(date.getHours())}:${pad(date.getMinutes())}` };
    }

    function calendarDateFromParts(dateValue, timeValue) {
        const day = calParseISODate(dateValue);
        if (Number.isNaN(day.getTime())) return null;
        const match = /^(\d{1,2}):(\d{2})/.exec(String(timeValue || ''));
        if (!match) return null;
        day.setHours(Number(match[1]), Number(match[2]), 0, 0);
        return day;
    }

    function calendarDefaultStart(dateHint) {
        if (dateHint instanceof Date) return new Date(dateHint);
        if (typeof dateHint === 'string' && dateHint.length > 10) {
            const parsed = new Date(dateHint);
            if (!Number.isNaN(parsed.getTime())) return parsed;
        }
        let base = typeof dateHint === 'string' && dateHint ? calParseISODate(dateHint) : new Date();
        if (Number.isNaN(base.getTime())) base = new Date();
        if (calSameDay(base, new Date())) {
            // Today: propose the next half hour instead of a slot in the past.
            const now = new Date();
            now.setMinutes(now.getMinutes() >= 30 ? 60 : 30, 0, 0);
            return now;
        }
        base.setHours(9, 0, 0, 0);
        return base;
    }

    function calendarShiftDate(date, repeat, step) {
        if (repeat === 'daily') return calAddDays(date, step);
        if (repeat === 'weekly') return calAddDays(date, step * 7);
        if (repeat === 'monthly') return calAddMonths(date, step);
        return new Date(date);
    }

    async function createRecurringAppointments(payload, repeat, count) {
        const total = Math.max(1, Math.min(CAL_REPEAT_LIMIT, Number(count) || 1));
        const start = calendarDate(payload.date_time);
        if (!start || repeat === 'none' || total <= 1) {
            await plannerJSON('/api/appointments', 'POST', payload);
            return 1;
        }
        const reminderOffset = payload.notification_at ? start - new Date(payload.notification_at) : null;
        let created = 0;
        for (let index = 0; index < total; index += 1) {
            const when = calendarShiftDate(start, repeat, index);
            const body = Object.assign({}, payload, { date_time: when.toISOString() });
            if (reminderOffset !== null) body.notification_at = new Date(when.getTime() - reminderOffset).toISOString();
            await plannerJSON('/api/appointments', 'POST', body);
            created += 1;
        }
        return created;
    }

    function loadCalendarContacts(session) {
        if (!session.contactsPromise) {
            session.contactsPromise = api('/api/contacts').then(data => {
                const list = Array.isArray(data) ? data : (data && (data.contacts || data.items)) || [];
                return list.filter(c => c && c.id && c.name).map(c => ({ id: String(c.id), name: String(c.name), email: c.email || '', relationship: c.relationship || '' }));
            }).catch(() => []);
        }
        return session.contactsPromise;
    }

    function calendarParticipantChipsHTML(selected) {
        return selected.map(person => `<span class="vd-calendar-chip" data-participant-id="${esc(person.id)}"><span class="vd-calendar-chip-avatar" aria-hidden="true">${esc(String(person.name || '?').trim().charAt(0).toUpperCase())}</span><span>${esc(person.name)}</span><button type="button" class="vd-calendar-chip-remove" data-participant-remove="${esc(person.id)}" aria-label="${esc(t('desktop.remove'))}">×</button></span>`).join('');
    }

    function calendarStatusOptionsHTML(current) {
        const active = current === 'overdue' ? 'upcoming' : current;
        return ['upcoming', 'completed', 'cancelled'].map(status => `<button type="button" class="vd-calendar-segment ${active === status ? 'is-active' : ''}" role="radio" aria-checked="${active === status ? 'true' : 'false'}" data-cal-status-option="${status}"><span class="vd-calendar-event-dot is-${status}" aria-hidden="true"></span>${esc(calendarStatusLabel(status))}</button>`).join('');
    }

    function calendarEditorHTML(appointment, start) {
        const isNew = !appointment;
        const parts = calendarInputParts(start);
        const preset = calendarReminderPreset(appointment);
        const status = appointment ? appointment.status : 'upcoming';
        return `<form class="vd-modal vd-calendar-modal vd-calendar-editor" novalidate autocomplete="off">
            <header class="vd-calendar-editor-head">
                <span class="vd-calendar-editor-kicker">${esc(t(isNew ? 'desktop.cal_new_appointment' : 'desktop.cal_edit_appointment'))}</span>
                <button type="button" class="vd-calendar-icon-button" data-cancel aria-label="${esc(t('desktop.close'))}">${iconMarkup('x', 'x', 'vd-calendar-action-icon', 14)}</button>
            </header>
            <input name="title" class="vd-calendar-editor-title" maxlength="200" placeholder="${esc(t('desktop.cal_title_placeholder'))}" value="${esc(appointment ? appointment.title : '')}" required>
            <div class="vd-calendar-form-error" data-form-error role="alert" hidden></div>
            <div class="vd-calendar-editor-grid">
                <label class="vd-calendar-field"><span>${esc(t('desktop.cal_date'))}</span><input type="date" name="date" value="${parts.date}" required></label>
                <label class="vd-calendar-field"><span>${esc(t('desktop.cal_time'))}</span><input type="time" name="time" value="${parts.time}" step="300" required></label>
                <label class="vd-calendar-field"><span>${esc(t('desktop.cal_reminder'))}</span>
                    <select name="reminder">${CAL_REMINDER_PRESETS.map(value => `<option value="${value}" ${value === preset ? 'selected' : ''}>${esc(calendarReminderLabel(value))}</option>`).join('')}</select>
                </label>
                <label class="vd-calendar-field" data-reminder-custom ${preset === 'custom' ? '' : 'hidden'}><span>${esc(t('desktop.cal_reminder_time'))}</span><input type="datetime-local" name="notification_at" value="${appointment && appointment.notification_at ? esc(dateTimeLocalValue(appointment.notification_at)) : ''}"></label>
            </div>
            <div class="vd-calendar-field vd-calendar-participants">
                <span>${esc(t('desktop.cal_participants'))}</span>
                <div class="vd-calendar-chip-input" data-participants>
                    ${calendarParticipantChipsHTML(appointment ? appointment.participants || [] : [])}
                    <input type="text" data-participant-search inputmode="search" enterkeyhint="done" placeholder="${esc(t('desktop.cal_participants_placeholder'))}" autocomplete="off" spellcheck="false">
                </div>
                <div class="vd-calendar-chip-menu" data-participant-menu hidden></div>
            </div>
            <label class="vd-calendar-field"><span>${esc(t('desktop.cal_description'))}</span><textarea name="description" rows="3" placeholder="${esc(t('desktop.cal_description_placeholder'))}">${esc(appointment ? appointment.description : '')}</textarea></label>
            <section class="vd-calendar-editor-section">
                <label class="vd-calendar-switch">
                    <input type="checkbox" name="wake_agent" ${appointment && appointment.wake_agent ? 'checked' : ''}>
                    <span class="vd-calendar-switch-track" aria-hidden="true"><span class="vd-calendar-switch-thumb"></span></span>
                    <span class="vd-calendar-switch-text"><b>${esc(t('desktop.cal_wake_agent'))}</b><small>${esc(t('desktop.cal_wake_agent_hint'))}</small></span>
                </label>
                <textarea name="agent_instruction" data-agent-instruction rows="2" placeholder="${esc(t('desktop.cal_agent_instruction_placeholder'))}" ${appointment && appointment.wake_agent ? '' : 'hidden'}>${esc(appointment ? appointment.agent_instruction : '')}</textarea>
            </section>
            ${isNew ? `<section class="vd-calendar-editor-section vd-calendar-recurring">
                <div class="vd-calendar-editor-grid">
                    <label class="vd-calendar-field"><span>${esc(t('desktop.cal_recurring'))}</span>
                        <select name="repeat">${['none', 'daily', 'weekly', 'monthly'].map(value => `<option value="${value}">${esc(t(`desktop.cal_repeat_${value}`))}</option>`).join('')}</select>
                    </label>
                    <label class="vd-calendar-field" data-repeat-count hidden><span>${esc(t('desktop.cal_repeat_count'))}</span><input type="number" name="repeat_count" min="2" max="${CAL_REPEAT_LIMIT}" value="4"></label>
                </div>
                <p class="vd-calendar-field-hint" data-repeat-preview hidden></p>
            </section>` : `<section class="vd-calendar-editor-section vd-calendar-editor-status">
                <span class="vd-calendar-field-label">${esc(t('desktop.cal_status'))}</span>
                <div class="vd-calendar-segmented" role="radiogroup" data-status-group data-status="${esc(status)}">${calendarStatusOptionsHTML(status)}</div>
            </section>`}
            <footer class="vd-calendar-editor-actions">
                ${isNew ? '' : `<button type="button" class="vd-button vd-calendar-button-danger" data-delete>${iconMarkup('trash', 'D', 'vd-calendar-action-icon', 14)}<span>${esc(t('desktop.delete'))}</span></button>`}
                <span class="vd-calendar-editor-spacer"></span>
                <button type="button" class="vd-button" data-cancel>${esc(t('desktop.cancel'))}</button>
                <button type="submit" class="vd-button vd-button-primary" data-submit><span>${esc(t(isNew ? 'desktop.cal_create' : 'desktop.save'))}</span><kbd>Ctrl+↵</kbd></button>
            </footer>
        </form>`;
    }

    function calendarEditorUpdateRepeatPreview(form) {
        const repeat = form.elements.repeat, count = form.elements.repeat_count, preview = form.querySelector('[data-repeat-preview]');
        if (!repeat || !preview) return;
        const wrapper = form.querySelector('[data-repeat-count]');
        const active = repeat.value !== 'none';
        if (wrapper) wrapper.hidden = !active;
        preview.hidden = !active;
        if (!active) return;
        const start = calendarDateFromParts(form.elements.date.value, form.elements.time.value);
        const total = Math.max(2, Math.min(CAL_REPEAT_LIMIT, Number(count.value) || 2));
        if (!start) { preview.textContent = ''; return; }
        const last = calendarShiftDate(start, repeat.value, total - 1);
        preview.textContent = t('desktop.cal_repeat_preview', { count: total, until: calFormat(last, { day: 'numeric', month: 'long', year: 'numeric' }) });
    }

    function wireCalendarParticipants(session, form, selected) {
        const container = form.querySelector('[data-participants]');
        const search = form.querySelector('[data-participant-search]');
        const menu = form.querySelector('[data-participant-menu]');
        if (!container || !search || !menu) return;
        let contacts = null;
        let highlighted = 0;

        const renderChips = () => {
            container.querySelectorAll('.vd-calendar-chip').forEach(chip => chip.remove());
            container.insertAdjacentHTML('afterbegin', calendarParticipantChipsHTML(selected));
        };
        const matches = () => {
            if (!contacts) return [];
            const query = search.value.trim().toLowerCase();
            const chosen = new Set(selected.map(p => String(p.id)));
            return contacts.filter(c => !chosen.has(c.id) && (!query || c.name.toLowerCase().includes(query) || c.email.toLowerCase().includes(query))).slice(0, 8);
        };
        const renderMenu = () => {
            if (document.activeElement !== search) { menu.hidden = true; return; }
            if (contacts === null) { menu.hidden = false; menu.innerHTML = `<div class="vd-calendar-chip-menu-empty">${esc(t('desktop.loading'))}</div>`; return; }
            const list = matches();
            highlighted = Math.min(highlighted, Math.max(0, list.length - 1));
            menu.hidden = false;
            if (!list.length) { menu.innerHTML = `<div class="vd-calendar-chip-menu-empty">${esc(t(contacts.length ? 'desktop.cal_no_contacts' : 'desktop.cal_no_contacts_yet'))}</div>`; return; }
            menu.innerHTML = list.map((c, index) => `<button type="button" class="vd-calendar-chip-option ${index === highlighted ? 'is-active' : ''}" data-participant-pick="${esc(c.id)}"><span class="vd-calendar-chip-avatar" aria-hidden="true">${esc(c.name.charAt(0).toUpperCase())}</span><span class="vd-calendar-chip-option-body"><b>${esc(c.name)}</b>${c.email || c.relationship ? `<small>${esc(c.email || c.relationship)}</small>` : ''}</span></button>`).join('');
        };
        const pick = id => {
            const contact = (contacts || []).find(c => c.id === String(id));
            if (!contact || selected.some(p => String(p.id) === contact.id)) return;
            selected.push({ id: contact.id, name: contact.name });
            search.value = '';
            highlighted = 0;
            renderChips();
            renderMenu();
        };

        search.addEventListener('focus', () => {
            renderMenu();
            loadCalendarContacts(session).then(list => { contacts = list; renderMenu(); });
        });
        search.addEventListener('input', () => { highlighted = 0; renderMenu(); });
        search.addEventListener('blur', () => { setTimeout(() => { if (!menu.contains(document.activeElement)) menu.hidden = true; }, 120); });
        search.addEventListener('keydown', event => {
            const list = matches();
            if (event.key === 'ArrowDown' && list.length) { event.preventDefault(); highlighted = (highlighted + 1) % list.length; renderMenu(); }
            else if (event.key === 'ArrowUp' && list.length) { event.preventDefault(); highlighted = (highlighted - 1 + list.length) % list.length; renderMenu(); }
            else if (event.key === 'Enter') { event.preventDefault(); if (list[highlighted]) pick(list[highlighted].id); }
            else if (event.key === 'Backspace' && !search.value && selected.length) { selected.pop(); renderChips(); renderMenu(); }
            else if (event.key === 'Escape' && !menu.hidden) { event.preventDefault(); event.stopPropagation(); menu.hidden = true; }
        });
        menu.addEventListener('mousedown', event => event.preventDefault());
        menu.addEventListener('click', event => {
            const option = event.target.closest('[data-participant-pick]');
            if (option) { pick(option.dataset.participantPick); search.focus(); }
        });
        container.addEventListener('click', event => {
            const remove = event.target.closest('[data-participant-remove]');
            if (remove) {
                const id = remove.dataset.participantRemove;
                const index = selected.findIndex(p => String(p.id) === id);
                if (index >= 0) selected.splice(index, 1);
                renderChips();
                search.focus();
                return;
            }
            if (event.target === container) search.focus();
        });
    }

    function calendarEditorPayload(form, selected) {
        const title = form.elements.title.value.trim();
        if (!title) return { error: t('desktop.cal_title_required'), field: form.elements.title };
        const start = calendarDateFromParts(form.elements.date.value, form.elements.time.value);
        if (!start) return { error: t('desktop.cal_date_required'), field: form.elements.date };
        const wakeAgent = !!form.elements.wake_agent.checked;
        const payload = {
            title,
            description: form.elements.description.value.trim(),
            date_time: start.toISOString(),
            notification_at: calendarReminderFromPreset(form.elements.reminder.value, start, form.elements.notification_at.value),
            wake_agent: wakeAgent,
            agent_instruction: wakeAgent ? form.elements.agent_instruction.value.trim() : '',
            contact_ids: selected.map(p => String(p.id))
        };
        const group = form.querySelector('[data-status-group]');
        if (group) {
            const status = group.dataset.status || 'upcoming';
            // Overdue is server-derived; rescheduling into the future reopens the appointment.
            payload.status = status === 'overdue' && start > new Date() ? 'upcoming' : status;
        }
        return { payload, start };
    }

    function closeCalendarEditor(session) {
        const backdrop = session.editor;
        if (!backdrop) return;
        session.editor = null;
        const restore = backdrop.__calendarRestoreFocus;
        backdrop.classList.add('is-closing');
        const finish = () => backdrop.remove();
        if (document.body.dataset.animations === 'false') finish(); else setTimeout(finish, 140);
        if (restore && typeof restore.focus === 'function' && document.contains(restore)) restore.focus({ preventScroll: true });
    }

    function openAppointmentEditor(session, options) {
        options = options || {};
        const appointment = options.appointment || null;
        closeCalendarEditor(session);
        closeCalendarPeek(session);
        const start = appointment ? (calendarDate(appointment.date_time) || new Date()) : calendarDefaultStart(options.dateHint);
        const backdrop = document.createElement('div');
        backdrop.className = 'vd-modal-backdrop vd-calendar-editor-backdrop';
        backdrop.setAttribute('role', 'dialog');
        backdrop.setAttribute('aria-modal', 'true');
        backdrop.innerHTML = calendarEditorHTML(appointment, start);
        backdrop.__calendarRestoreFocus = document.activeElement;
        document.body.appendChild(backdrop);
        session.editor = backdrop;

        const form = backdrop.querySelector('form');
        const selected = (appointment && appointment.participants ? appointment.participants : []).filter(p => p && p.id).map(p => ({ id: String(p.id), name: p.name || '' }));
        const errorBox = form.querySelector('[data-form-error]');
        const showError = (message, field) => {
            errorBox.textContent = message;
            errorBox.hidden = !message;
            if (field) { field.classList.add('is-invalid'); field.focus(); field.addEventListener('input', () => field.classList.remove('is-invalid'), { once: true }); }
        };
        const setBusy = busy => {
            form.classList.toggle('is-busy', busy);
            form.querySelectorAll('button, input, select, textarea').forEach(el => { el.disabled = busy; });
        };

        wireCalendarParticipants(session, form, selected);
        calendarEditorUpdateRepeatPreview(form);

        form.addEventListener('change', event => {
            if (event.target.name === 'reminder') form.querySelector('[data-reminder-custom]').hidden = event.target.value !== 'custom';
            if (event.target.name === 'wake_agent') {
                const box = form.querySelector('[data-agent-instruction]');
                box.hidden = !event.target.checked;
                // The agent is woken by the reminder, so make sure one exists.
                if (event.target.checked && form.elements.reminder.value === 'none') form.elements.reminder.value = '0';
                if (event.target.checked) box.focus();
            }
            if (['repeat', 'repeat_count', 'date', 'time'].includes(event.target.name)) calendarEditorUpdateRepeatPreview(form);
        });
        form.addEventListener('input', event => {
            if (event.target.name === 'repeat_count') calendarEditorUpdateRepeatPreview(form);
        });
        form.addEventListener('click', async event => {
            const option = event.target.closest('[data-cal-status-option]');
            if (option) {
                const group = form.querySelector('[data-status-group]');
                group.dataset.status = option.dataset.calStatusOption;
                group.innerHTML = calendarStatusOptionsHTML(group.dataset.status);
                return;
            }
            if (event.target.closest('[data-cancel]')) { closeCalendarEditor(session); return; }
            if (event.target.closest('[data-delete]') && appointment) {
                if (!(await confirmDialog(t('desktop.delete'), t('desktop.cal_delete_confirm')))) return;
                setBusy(true);
                try {
                    await deleteCalendarAppointment(session, appointment);
                    closeCalendarEditor(session);
                } catch (err) {
                    setBusy(false);
                    showError(err && err.message ? err.message : t('desktop.request_failed'));
                }
            }
        });
        form.addEventListener('keydown', event => {
            if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closeCalendarEditor(session); }
            else if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) { event.preventDefault(); form.requestSubmit(); }
            else if (event.key === 'Enter' && event.target === form.elements.title) { event.preventDefault(); form.elements.date.focus(); }
        });
        form.addEventListener('submit', async event => {
            event.preventDefault();
            const result = calendarEditorPayload(form, selected);
            if (result.error) { showError(result.error, result.field); return; }
            setBusy(true);
            try {
                if (appointment) {
                    await plannerJSON(`/api/appointments/${encodeURIComponent(appointment.id)}`, 'PUT', result.payload);
                    closeCalendarEditor(session);
                    await session.reload({ silent: true });
                    session.snack({ message: t('desktop.cal_saved') });
                } else {
                    const repeat = form.elements.repeat ? form.elements.repeat.value : 'none';
                    const count = form.elements.repeat_count ? form.elements.repeat_count.value : 1;
                    const created = await createRecurringAppointments(result.payload, repeat, count);
                    closeCalendarEditor(session);
                    session.focusDate(result.start);
                    await session.reload({ silent: true });
                    session.snack({ message: created > 1 ? t('desktop.cal_created_many', { count: created }) : t('desktop.cal_created') });
                }
                desktopSound('notify.info');
            } catch (err) {
                setBusy(false);
                showError(err && err.message ? err.message : t('desktop.request_failed'));
            }
        });
        backdrop.addEventListener('mousedown', event => { if (event.target === backdrop) backdrop.dataset.dismiss = 'true'; });
        backdrop.addEventListener('mouseup', event => {
            if (event.target === backdrop && backdrop.dataset.dismiss === 'true') closeCalendarEditor(session);
            delete backdrop.dataset.dismiss;
        });
        requestAnimationFrame(() => {
            const title = form.elements.title;
            title.focus({ preventScroll: true });
            if (appointment) title.setSelectionRange(title.value.length, title.value.length);
        });
    }

    function closeCalendarPeek(session) {
        if (!session.peek) return;
        const peek = session.peek;
        session.peek = null;
        if (session.peekDismiss) { document.removeEventListener('pointerdown', session.peekDismiss, true); document.removeEventListener('keydown', session.peekKeydown, true); }
        session.peekDismiss = null;
        session.peekKeydown = null;
        const anchor = peek.__calendarAnchor;
        const hadFocus = peek.contains(document.activeElement);
        peek.remove();
        session.host.querySelectorAll('.vd-calendar-event.is-peeking').forEach(el => el.classList.remove('is-peeking'));
        if (hadFocus && anchor && document.contains(anchor)) anchor.focus({ preventScroll: true });
    }

    function calendarPeekHTML(item) {
        const date = calendarDate(item.date_time);
        const canComplete = item.status === 'upcoming' || item.status === 'overdue';
        const canCancel = item.status === 'upcoming' || item.status === 'overdue';
        const canReopen = item.status === 'completed' || item.status === 'cancelled';
        const participants = (item.participants || []).map(p => p.name).filter(Boolean);
        return `<div class="vd-calendar-peek-accent is-${item.status}" aria-hidden="true"></div>
            <div class="vd-calendar-peek-body">
                <header class="vd-calendar-peek-head">
                    <h3>${esc(item.title)}</h3>
                    <span class="vd-calendar-status-pill is-${item.status}">${esc(calendarStatusLabel(item.status))}</span>
                </header>
                <p class="vd-calendar-peek-when">${iconMarkup('clock', 'T', 'vd-calendar-mini-icon', 13)}<span>${esc(date ? `${calFormat(date, { weekday: 'long', day: 'numeric', month: 'long' })} · ${calFormatTime(date)}` : '')}</span><small>${esc(date ? calRelativeTime(date) : '')}</small></p>
                ${item.notification_at ? `<p class="vd-calendar-peek-meta">${iconMarkup('bell', 'R', 'vd-calendar-mini-icon', 13)}<span>${esc(t('desktop.cal_reminder'))}: ${esc(calendarDateTimeLabel(item.notification_at))}</span></p>` : ''}
                ${participants.length ? `<p class="vd-calendar-peek-meta">${iconMarkup('users', 'P', 'vd-calendar-mini-icon', 13)}<span>${esc(participants.join(', '))}</span></p>` : ''}
                ${item.wake_agent ? `<p class="vd-calendar-peek-meta is-agent">${iconMarkup('chat', 'A', 'vd-calendar-mini-icon', 13)}<span>${esc(item.agent_instruction || t('desktop.cal_wake_agent'))}</span></p>` : ''}
                ${item.description ? `<p class="vd-calendar-peek-description">${esc(item.description)}</p>` : ''}
                <footer class="vd-calendar-peek-actions">
                    ${canComplete ? `<button type="button" class="vd-button vd-button-primary" data-cal-status-action="completed" data-appt-id="${esc(item.id)}">${iconMarkup('check', 'OK', 'vd-calendar-action-icon', 14)}<span>${esc(t('desktop.cal_mark_complete'))}</span></button>` : ''}
                    ${canReopen ? `<button type="button" class="vd-button" data-cal-status-action="upcoming" data-appt-id="${esc(item.id)}">${iconMarkup('undo', 'U', 'vd-calendar-action-icon', 14)}<span>${esc(t('desktop.cal_reopen'))}</span></button>` : ''}
                    <button type="button" class="vd-button" data-cal-edit="${esc(item.id)}">${iconMarkup('edit', 'E', 'vd-calendar-action-icon', 14)}<span>${esc(t('desktop.launchpad_edit'))}</span></button>
                    ${canCancel ? `<button type="button" class="vd-calendar-icon-button" data-cal-status-action="cancelled" data-appt-id="${esc(item.id)}" title="${esc(t('desktop.cal_cancel_appointment'))}" aria-label="${esc(t('desktop.cal_cancel_appointment'))}">${iconMarkup('x', 'x', 'vd-calendar-action-icon', 14)}</button>` : ''}
                    <button type="button" class="vd-calendar-icon-button is-danger" data-cal-delete="${esc(item.id)}" title="${esc(t('desktop.delete'))}" aria-label="${esc(t('desktop.delete'))}">${iconMarkup('trash', 'D', 'vd-calendar-action-icon', 14)}</button>
                </footer>
            </div>`;
    }

    function openAppointmentPeek(session, item, anchor) {
        closeCalendarPeek(session);
        const shell = session.host.querySelector('.vd-calendar-shell');
        if (!shell || !item) return;
        const peek = document.createElement('div');
        peek.className = 'vd-calendar-peek';
        peek.setAttribute('role', 'dialog');
        peek.innerHTML = calendarPeekHTML(item);
        shell.appendChild(peek);
        session.peek = peek;
        peek.__calendarAnchor = anchor || null;
        if (anchor) anchor.classList.add('is-peeking');
        // Position next to the anchor inside the shell, flipping when there is no room.
        const shellRect = shell.getBoundingClientRect();
        const anchorRect = anchor ? anchor.getBoundingClientRect() : shellRect;
        const width = peek.offsetWidth, height = peek.offsetHeight;
        let left = anchorRect.right - shellRect.left + 10;
        if (left + width > shellRect.width - 8) left = anchorRect.left - shellRect.left - width - 10;
        if (left < 8) left = Math.max(8, Math.min(shellRect.width - width - 8, anchorRect.left - shellRect.left));
        let top = anchorRect.top - shellRect.top - 8;
        if (top + height > shellRect.height - 8) top = Math.max(8, shellRect.height - height - 8);
        peek.style.left = `${Math.round(left)}px`;
        peek.style.top = `${Math.round(top)}px`;
        session.peekDismiss = event => { if (!peek.contains(event.target)) closeCalendarPeek(session); };
        session.peekKeydown = event => { if (event.key === 'Escape') { event.stopPropagation(); closeCalendarPeek(session); } };
        document.addEventListener('pointerdown', session.peekDismiss, true);
        document.addEventListener('keydown', session.peekKeydown, true);
        const first = peek.querySelector('button');
        if (first) first.focus({ preventScroll: true });
    }
