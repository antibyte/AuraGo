            if (base === 8) return ch >= '0' && ch <= '7';
            if (base === 10) return ch >= '0' && ch <= '9';
            if (base === 16) return /[0-9A-Fa-f]/.test(ch);
            return false;
        };
        while (index < expression.length) {
            const ch = expression[index];
            if (ch === ' ' || ch === '\t') { index++; continue; }
            if (isDigit(ch)) {
                let value = '';
                while (index < expression.length && isDigit(expression[index])) {
                    value += expression[index];
                    index++;
                }
                tokens.push({ type: 'number', value: parseInt(value, base) });
                continue;
            }
            if (ch === 'N' && expression.slice(index, index + 3) === 'NOT') {
                tokens.push({ type: 'operator', value: 'NOT' });
                index += 3;
                continue;
            }
            if (ch === 'A' && expression.slice(index, index + 3) === 'AND') {
                tokens.push({ type: 'operator', value: 'AND' });
                index += 3;
                continue;
            }
            if (ch === 'O' && expression.slice(index, index + 2) === 'OR') {
                tokens.push({ type: 'operator', value: 'OR' });
                index += 2;
                continue;
            }
            if (ch === 'X' && expression.slice(index, index + 3) === 'XOR') {
                tokens.push({ type: 'operator', value: 'XOR' });
                index += 3;
                continue;
            }
            if (ch === 'M' && expression.slice(index, index + 3) === 'MOD') {
                tokens.push({ type: 'operator', value: 'MOD' });
                index += 3;
                continue;
            }
            if (ch === 'S' && expression.slice(index, index + 3) === 'SHL') {
                tokens.push({ type: 'operator', value: 'SHL' });
                index += 3;
                continue;
            }
            if (ch === 'S' && expression.slice(index, index + 3) === 'SHR') {
                tokens.push({ type: 'operator', value: 'SHR' });
                index += 3;
                continue;
            }
            if (ch === '<' && expression[index + 1] === '<') {
                tokens.push({ type: 'operator', value: 'SHL' });
                index += 2;
                continue;
            }
            if (ch === '>' && expression[index + 1] === '>') {
                tokens.push({ type: 'operator', value: 'SHR' });
                index += 2;
                continue;
            }
            if (ch === '&') { tokens.push({ type: 'operator', value: 'AND' }); index++; continue; }
            if (ch === '|') { tokens.push({ type: 'operator', value: 'OR' }); index++; continue; }
            if (ch === '^') { tokens.push({ type: 'operator', value: 'XOR' }); index++; continue; }
            if (ch === '~') { tokens.push({ type: 'operator', value: 'NOT' }); index++; continue; }
            if (ch === '%') { tokens.push({ type: 'operator', value: 'MOD' }); index++; continue; }
            if (ch === '+') { tokens.push({ type: 'operator', value: '+' }); index++; continue; }
            if (ch === '-') { tokens.push({ type: 'operator', value: '-' }); index++; continue; }
            if (ch === '*' || ch === '×') { tokens.push({ type: 'operator', value: '*' }); index++; continue; }
            if (ch === '/' || ch === '÷') { tokens.push({ type: 'operator', value: '/' }); index++; continue; }
            if (ch === '(') { tokens.push({ type: 'lparen', value: '(' }); index++; continue; }
            if (ch === ')') { tokens.push({ type: 'rparen', value: ')' }); index++; continue; }
            throw new Error('Invalid expression');
        }
        tokens.push({ type: 'eof', value: '' });
        return tokens;
    }

    function parseProgrammerExpression(tokens) {
        let index = 0;
        const current = () => tokens[index];
        const consume = () => tokens[index++];
        const expect = type => {
            if (current().type !== type) throw new Error('Invalid expression');
            return consume();
        };

        const parseExpression = () => parseOrExpression();

        const parseOrExpression = () => {
            let left = parseXorExpression();
            while (current().value === 'OR') {
                consume();
                left = left | parseXorExpression();
            }
            return left;
        };

        const parseXorExpression = () => {
            let left = parseAndExpression();
            while (current().value === 'XOR') {
                consume();
                left = left ^ parseAndExpression();
            }
            return left;
        };

        const parseAndExpression = () => {
            let left = parseShiftExpression();
            while (current().value === 'AND') {
                consume();
                left = left & parseShiftExpression();
            }
            return left;
        };

        const parseShiftExpression = () => {
            let left = parseAdditiveExpression();
            while (['SHL', 'SHR'].includes(current().value)) {
                const op = consume().value;
                const right = parseAdditiveExpression();
                left = op === 'SHL' ? left << right : left >> right;
            }
            return left;
        };

        const parseAdditiveExpression = () => {
            let left = parseMultiplicativeExpression();
            while (['+', '-'].includes(current().value)) {
                const op = consume().value;
                const right = parseMultiplicativeExpression();
                left = op === '+' ? left + right : left - right;
            }
            return left;
        };

        const parseMultiplicativeExpression = () => {
            let left = parseUnaryExpression();
            while (['*', '/', 'MOD'].includes(current().value)) {
                const op = consume().value;
                const right = parseUnaryExpression();
                rejectZeroDivisor(op, right);
                if (op === '*') left = left * right;
                else if (op === '/') {
                    left = Math.trunc(left / right);
                } else {
                    left = left % right;
                }
            }
            return left;
        };

        const parseUnaryExpression = () => {
            if (current().value === 'NOT') {
                consume();
                return ~parseUnaryExpression();
            }
            if (current().value === '-') {
                consume();
                return -parseUnaryExpression();
            }
            return parsePrimaryExpression();
        };

        const parsePrimaryExpression = () => {
            if (current().type === 'number') {
                return consume().value;
            }
            if (current().type === 'lparen') {
                consume();
                const value = parseExpression();
                expect('rparen');
                return value;
            }
            throw new Error('Invalid expression');
        };

        const result = parseExpression();
        if (current().type !== 'eof') throw new Error('Invalid expression');
        if (!Number.isFinite(result) || !Number.isInteger(result)) throw new Error('Invalid expression');
        return result;
    }
    async function renderTodo(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.todoFilter = host.dataset.todoFilter || 'all';
        host.innerHTML = `<div class="vd-todo"><aside class="vd-todo-sidebar">
            ${['all', 'open', 'in_progress', 'done'].map(status => `<button type="button" data-filter="${status}" class="${host.dataset.todoFilter === status ? 'active' : ''}">${esc(t('desktop.todo_' + status))}</button>`).join('')}
        </aside><main class="vd-todo-main"><form class="vd-todo-add"><input placeholder="${esc(t('desktop.todo_title_placeholder'))}"><select><option value="low">${esc(t('desktop.todo_priority_low'))}</option><option value="medium" selected>${esc(t('desktop.todo_priority_medium'))}</option><option value="high">${esc(t('desktop.todo_priority_high'))}</option></select><button class="vd-button vd-button-primary">${esc(t('desktop.todo_add'))}</button></form><div class="vd-todo-list">${esc(t('desktop.loading'))}</div></main><section class="vd-todo-detail"><div class="vd-empty">${esc(t('desktop.todo_select_task'))}</div></section></div>`;
        const showTodoContextMenu = (event, todo, reload) => {
            event.preventDefault();
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.context_open', icon: 'folder-open', action: () => renderTodoDetail(host, todo, reload) },
                { labelKey: 'desktop.todo_complete', icon: 'check-square', disabled: todo.status === 'done', action: async () => { await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id); } },
                { separator: true },
                { labelKey: 'desktop.delete', icon: 'trash', action: async () => { if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await api('/api/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); } } }
            ]);
            return true;
        };
        wireContextMenuBoundary(host);
        const load = async (selectedID) => {
            const todos = await api('/api/todos?status=all');
            const filtered = todos.filter(todo => host.dataset.todoFilter === 'all' || todo.status === host.dataset.todoFilter)
                .sort((a, b) => (({ high: 0, medium: 1, low: 2 }[a.priority] ?? 3) - (({ high: 0, medium: 1, low: 2 }[b.priority] ?? 3)) || String(a.due_date || '9999').localeCompare(String(b.due_date || '9999'))));
            const list = host.querySelector('.vd-todo-list');
            list.innerHTML = filtered.length ? filtered.map(todo => renderTodoCard(todo, selectedID)).join('') : `<div class="vd-empty">${esc(t('desktop.empty_folder'))}</div>`;
            list.querySelectorAll('[data-todo-id]').forEach(card => card.addEventListener('click', () => renderTodoDetail(host, todos.find(todo => todo.id === card.dataset.todoId), load)));
            list.querySelectorAll('[data-todo-status-toggle]').forEach(input => input.addEventListener('click', event => event.stopPropagation()));
            list.querySelectorAll('[data-todo-status-toggle]').forEach(input => input.addEventListener('change', async event => {
                event.stopPropagation();
                const todo = todos.find(item => item.id === input.dataset.todoStatusToggle);
                if (todo) await setTodoDone(todo, input.checked, load);
            }));
            list.querySelectorAll('[data-todo-id]').forEach(card => card.addEventListener('contextmenu', event => {
                const todo = todos.find(item => item.id === card.dataset.todoId);
                if (todo) showTodoContextMenu(event, todo, load);
            }));
            const selected = todos.find(todo => todo.id === selectedID) || filtered[0];
            if (selected) renderTodoDetail(host, selected, load);
        };
        host.querySelectorAll('[data-filter]').forEach(btn => btn.addEventListener('click', () => {
            host.dataset.todoFilter = btn.dataset.filter;
            renderTodo(id);
        }));
        host.querySelector('.vd-todo-add').addEventListener('submit', async event => {
            event.preventDefault();
            const input = event.currentTarget.querySelector('input');
            const title = input.value.trim();
            if (!title) return;
            const result = await plannerJSON('/api/todos', 'POST', { title, priority: event.currentTarget.querySelector('select').value, status: 'open' });
            input.value = '';
            await load(result.id);
        });
        try { await load(); } catch (err) { host.querySelector('.vd-todo-list').innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`; }
    }

    function renderTodoCard(todo, selectedID) {
        const due = todo.due_date ? new Date(todo.due_date) : null;
        const overdue = due && due < new Date() && todo.status !== 'done';
        return `<article class="vd-todo-card ${todo.id === selectedID ? 'active' : ''} ${overdue ? 'overdue' : ''}" data-todo-id="${esc(todo.id)}">
            <div class="vd-todo-card-grid">
                <input class="vd-todo-card-done" type="checkbox" data-todo-status-toggle="${esc(todo.id)}" ${todo.status === 'done' ? 'checked' : ''} aria-label="${esc(t('desktop.todo_complete'))}">
                <div class="vd-todo-card-copy">
                    <strong>${esc(todo.title)}</strong>
                    <small>${esc(t('desktop.todo_' + todo.status))}${due ? ' &middot; ' + esc(due.toLocaleDateString()) : ''}${overdue ? ' &middot; ' + esc(t('desktop.todo_overdue')) : ''}</small>
                </div>
                <span class="vd-todo-priority ${esc(todo.priority)}">${esc(t('desktop.todo_priority_' + todo.priority))}</span>
            </div>
            <div class="vd-todo-progress"><span style="width:${Number(todo.progress_percent) || 0}%"></span></div>
        </article>`;
    }

    async function setTodoDone(todo, done, reload) {
        if (!todo) return;
        if (done) {
            await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true });
            await reload(todo.id);
            return;
        }
        const payload = { status: 'open' };
        if (Array.isArray(todo.items) && todo.items.length) {
            payload.items = todo.items.map(item => Object.assign({}, item, { is_done: false }));
        }
        await plannerJSON('/api/todos/' + encodeURIComponent(todo.id), 'PUT', payload);
        await reload(todo.id);
    }

    async function updateTodoItem(todo, itemID, patch, reload) {
        if (!todo || !itemID) return;
        await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/items/' + encodeURIComponent(itemID), 'PUT', patch);
        await reload(todo.id);
    }

    function renderTodoDetail(host, todo, reload) {
        const pane = host.querySelector('.vd-todo-detail');
        const items = todo.items || [];
        pane.innerHTML = `<form class="vd-todo-form"><input name="title" value="${esc(todo.title)}"><textarea name="description" placeholder="${esc(t('desktop.todo_description'))}">${esc(todo.description || '')}</textarea><div class="vd-todo-form-row"><label>${esc(t('desktop.todo_priority'))}<select name="priority">${['low','medium','high'].map(p => `<option value="${p}" ${todo.priority === p ? 'selected' : ''}>${esc(t('desktop.todo_priority_' + p))}</option>`).join('')}</select></label><label>${esc(t('desktop.todo_due_date'))}<input type="date" name="due_date" value="${esc(todo.due_date || '')}"></label></div><label class="vd-check"><input type="checkbox" name="remind_daily" ${todo.remind_daily ? 'checked' : ''}>${esc(t('desktop.todo_remind_daily'))}</label><div class="vd-todo-actions"><button class="vd-button vd-button-primary" data-action="save">${esc(t('desktop.save'))}</button><button type="button" class="vd-button" data-action="complete">${esc(t('desktop.todo_complete'))}</button><button type="button" class="vd-button" data-action="delete">${esc(t('desktop.delete'))}</button></div></form><h3>${esc(t('desktop.todo_items'))}</h3><form class="vd-todo-item-add"><input placeholder="${esc(t('desktop.todo_add_item'))}"><button class="vd-button">${esc(t('desktop.todo_add_item'))}</button></form><div class="vd-todo-items">${items.map(item => `<div class="vd-todo-item" data-item-id="${esc(item.id)}"><input class="vd-todo-item-check" type="checkbox" data-item-toggle="${esc(item.id)}" ${item.is_done ? 'checked' : ''} aria-label="${esc(t('desktop.todo_complete'))}"><input class="vd-todo-item-title" data-item-title="${esc(item.id)}" value="${esc(item.title)}" aria-label="${esc(t('desktop.todo_add_item'))}" spellcheck="true"><button type="button" class="vd-todo-item-delete" data-item-delete="${esc(item.id)}" title="${esc(t('desktop.delete'))}">${iconMarkup('x', 'X', 'vd-todo-action-icon', 13)}</button></div>`).join('')}</div>`;
        pane.querySelector('.vd-todo-form').addEventListener('submit', async event => {
            event.preventDefault();
            const form = event.currentTarget;
            await plannerJSON('/api/todos/' + encodeURIComponent(todo.id), 'PUT', { title: form.title.value.trim(), description: form.description.value, priority: form.priority.value, due_date: form.due_date.value, remind_daily: form.remind_daily.checked });
            await reload(todo.id);
        });
        pane.querySelector('[data-action="complete"]').addEventListener('click', async () => { await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id); });
        pane.querySelector('[data-action="delete"]').addEventListener('click', async () => { if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await api('/api/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); } });
        pane.querySelector('.vd-todo-item-add').addEventListener('submit', async event => { event.preventDefault(); const input = event.currentTarget.querySelector('input'); if (!input.value.trim()) return; await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/items', 'POST', { title: input.value.trim() }); await reload(todo.id); });
        pane.querySelectorAll('[data-item-toggle]').forEach(input => input.addEventListener('change', async () => { await updateTodoItem(todo, input.dataset.itemToggle, { is_done: input.checked }, reload); }));
        pane.querySelectorAll('[data-item-title]').forEach(titleInput => {
            titleInput.addEventListener('keydown', async event => {
                if (event.key === 'Enter') {
                    event.preventDefault();
                    titleInput.blur();
                } else if (event.key === 'Escape') {
                    event.preventDefault();
                    const item = items.find(entry => entry.id === titleInput.dataset.itemTitle);
                    titleInput.value = item ? item.title : '';
                    titleInput.blur();
                }
            });
            titleInput.addEventListener('change', async () => {
                const title = titleInput.value.trim();
                const item = items.find(entry => entry.id === titleInput.dataset.itemTitle);
                if (!title || !item || title === item.title) {
                    titleInput.value = item ? item.title : '';
                    return;
                }
                await updateTodoItem(todo, titleInput.dataset.itemTitle, { title: titleInput.value.trim() }, reload);
            });
        });
        pane.querySelectorAll('[data-item-delete]').forEach(btn => btn.addEventListener('click', async () => { await api('/api/todos/' + encodeURIComponent(todo.id) + '/items/' + encodeURIComponent(btn.dataset.itemDelete), { method: 'DELETE' }); await reload(todo.id); }));
        setTodoMenus(host, todo, reload);
    }

    function setTodoMenus(host, todo, reload) {
        const win = host && host.closest && host.closest('.vd-window');
        const id = win && win.dataset.windowId;
        if (!id || !todo) return;
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'save', labelKey: 'desktop.save', icon: 'save', shortcut: 'Ctrl+S', action: () => {
                        const form = host.querySelector('.vd-todo-form');
                        if (form) form.requestSubmit();
                    } }
                ]
            },
            {
                id: 'edit',
                labelKey: 'desktop.menu_edit',
                items: [
                    { id: 'complete', labelKey: 'desktop.todo_complete', icon: 'check-square', action: async () => { await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id); } },
                    { id: 'delete', labelKey: 'desktop.delete', icon: 'trash', action: async () => { if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await api('/api/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); } } }
                ]
            }
        ]);
    }

    async function renderCalendar(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.calView = host.dataset.calView || 'month';
        host.dataset.calDate = host.dataset.calDate || isoDate(new Date());
        const activeDate = new Date(host.dataset.calDate + 'T12:00:00');
        host.innerHTML = `<div class="vd-calendar"><div class="vd-calendar-toolbar"><button class="vd-calendar-icon-button" type="button" data-cal-nav="prev">${iconMarkup('chevron-left', 'L', 'vd-calendar-action-icon', 15)}</button><button type="button" data-cal-today>${iconMarkup('calendar', 'C', 'vd-calendar-action-icon', 15)}<span>${esc(t('desktop.cal_today'))}</span></button><button class="vd-calendar-icon-button" type="button" data-cal-nav="next">${iconMarkup('chevron-right', 'R', 'vd-calendar-action-icon', 15)}</button><strong>${esc(activeDate.toLocaleDateString(undefined, { month: 'long', year: 'numeric' }))}</strong><span></span>${['month','week','day'].map(view => `<button type="button" data-cal-view="${view}" class="${host.dataset.calView === view ? 'active' : ''}">${esc(t('desktop.cal_' + view))}</button>`).join('')}</div><div class="vd-calendar-body">${esc(t('desktop.loading'))}</div></div>`;
        const showCalendarContextMenu = (event, appointments, render) => {
            const apptEl = event.target.closest('[data-appt-id]');
            const cellEl = event.target.closest('[data-cal-date]');
            if (!apptEl && !cellEl) return false;
            const appt = apptEl ? appointments.find(item => item.id === apptEl.dataset.apptId) : null;
            const date = cellEl ? cellEl.dataset.calDate : isoDate(activeDate);
            showContextMenu(event.clientX, event.clientY, [
                appt
                    ? { labelKey: 'desktop.launchpad_edit', icon: 'edit', action: () => openAppointmentModal(host, appt, '', render) }
                    : { labelKey: 'desktop.cal_new_appointment', icon: 'calendar', action: () => openAppointmentModal(host, null, date, render) },
                { labelKey: 'desktop.context_refresh', icon: 'refresh', action: render }
            ]);
            return true;
        };
        wireContextMenuBoundary(host);
        const render = async () => {
            const appointments = await api('/api/appointments?status=all');
            const body = host.querySelector('.vd-calendar-body');
            body.innerHTML = host.dataset.calView === 'month' ? calendarMonthHTML(activeDate, appointments) : calendarAgendaHTML(activeDate, appointments, host.dataset.calView);
            body.querySelectorAll('[data-cal-date]').forEach(cell => cell.addEventListener('click', () => openAppointmentModal(host, null, cell.dataset.calDate, render)));
            body.querySelectorAll('[data-appt-id]').forEach(btn => btn.addEventListener('click', event => { event.stopPropagation(); openAppointmentModal(host, appointments.find(a => a.id === btn.dataset.apptId), '', render); }));
            body.oncontextmenu = event => {
                if (showCalendarContextMenu(event, appointments, render)) event.preventDefault();
            };
        };
        host.querySelectorAll('[data-cal-view]').forEach(btn => btn.addEventListener('click', () => { host.dataset.calView = btn.dataset.calView; renderCalendar(id); }));
        host.querySelector('[data-cal-today]').addEventListener('click', () => { host.dataset.calDate = isoDate(new Date()); renderCalendar(id); });
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

    function calendarMonthHTML(activeDate, appointments) {
        const first = new Date(activeDate.getFullYear(), activeDate.getMonth(), 1);
        const start = new Date(first);
        start.setDate(first.getDate() - ((first.getDay() + 6) % 7));
        const today = isoDate(new Date());
        const cells = Array.from({ length: 42 }, (_, i) => { const d = new Date(start); d.setDate(start.getDate() + i); return d; });
        return `<div class="vd-calendar-month">${cells.map(d => { const key = isoDate(d); const dayItems = appointments.filter(a => String(a.date_time || '').startsWith(key)); return `<button type="button" class="vd-calendar-cell ${d.getMonth() !== activeDate.getMonth() ? 'muted' : ''} ${key === today ? 'today' : ''}" data-cal-date="${key}"><span>${d.getDate()}</span>${dayItems.slice(0, 3).map(a => `<i class="${esc(a.status || 'upcoming')}" data-appt-id="${esc(a.id)}">${esc(a.title)}</i>`).join('')}</button>`; }).join('')}</div><aside class="vd-calendar-upcoming"><h3>${esc(t('desktop.cal_new_appointment'))}</h3>${appointments.slice(0, 8).map(a => `<button type="button" data-appt-id="${esc(a.id)}">${esc(new Date(a.date_time).toLocaleString())} · ${esc(a.title)}</button>`).join('')}</aside>`;
    }

    function calendarAgendaHTML(activeDate, appointments, view) {
        const days = view === 'week' ? Array.from({ length: 7 }, (_, i) => { const d = new Date(activeDate); d.setDate(activeDate.getDate() - ((activeDate.getDay() + 6) % 7) + i); return d; }) : [activeDate];
        return `<div class="vd-calendar-agenda ${view}">${days.map(day => { const key = isoDate(day); const dayItems = appointments.filter(a => String(a.date_time || '').startsWith(key)); return `<section><h3>${esc(day.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric' }))}</h3>${Array.from({ length: 24 }, (_, hour) => `<div class="vd-calendar-hour" data-cal-date="${key}T${String(hour).padStart(2, '0')}:00"><span>${String(hour).padStart(2, '0')}:00</span>${dayItems.filter(a => new Date(a.date_time).getHours() === hour).map(a => `<button type="button" class="${esc(a.status || 'upcoming')}" data-appt-id="${esc(a.id)}">${esc(a.title)}</button>`).join('')}</div>`).join('')}</section>`; }).join('')}</div>`;
    }

    function openAppointmentModal(host, appointment, dateHint, reload) {
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop';
        const initial = appointment || { title: '', description: '', status: 'upcoming', date_time: dateHint ? fromLocalDateTime(dateHint.includes('T') ? dateHint : dateHint + 'T09:00') : new Date().toISOString(), wake_agent: false };
        overlay.innerHTML = `<form class="vd-modal vd-calendar-modal"><div class="vd-modal-title">${esc(t(appointment ? 'desktop.cal_edit_appointment' : 'desktop.cal_new_appointment'))}</div><input name="title" class="vd-modal-input" placeholder="${esc(t('desktop.cal_title'))}" value="${esc(initial.title)}"><input name="date_time" class="vd-modal-input" type="datetime-local" value="${esc(dateTimeLocalValue(initial.date_time))}"><textarea name="description" class="vd-modal-input" placeholder="${esc(t('desktop.cal_description'))}">${esc(initial.description || '')}</textarea><select name="status" class="vd-modal-input">${['upcoming','overdue','completed','cancelled'].map(status => `<option value="${status}" ${initial.status === status ? 'selected' : ''}>${esc(t('desktop.cal_status_' + status))}</option>`).join('')}</select><label class="vd-check"><input name="wake_agent" type="checkbox" ${initial.wake_agent ? 'checked' : ''}>${esc(t('desktop.cal_notification'))}</label><div class="vd-modal-actions">${appointment ? `<button type="button" class="vd-button" data-delete>${iconMarkup('trash', 'X', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.delete'))}</span></button>` : ''}<button type="button" class="vd-button" data-cancel>${iconMarkup('x', 'X', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.cancel'))}</span></button><button class="vd-button vd-button-primary">${iconMarkup('save', 'S', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.save'))}</span></button></div></form>`;
        document.body.appendChild(overlay);
        const close = () => overlay.remove();
        overlay.querySelector('[data-cancel]').addEventListener('click', close);
        overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
        const del = overlay.querySelector('[data-delete]');
        if (del) del.addEventListener('click', async () => { if (await confirmDialog(t('desktop.cal_delete_confirm'), appointment.title)) { await api('/api/appointments/' + encodeURIComponent(appointment.id), { method: 'DELETE' }); close(); await reload(); } });
        overlay.querySelector('form').addEventListener('submit', async event => {
            event.preventDefault();
            const form = event.currentTarget;
            const payload = { title: form.title.value.trim(), date_time: fromLocalDateTime(form.date_time.value), description: form.description.value, status: form.status.value, wake_agent: form.wake_agent.checked };
            if (!payload.title) return;
            if (appointment) await plannerJSON('/api/appointments/' + encodeURIComponent(appointment.id), 'PUT', payload);
            else await plannerJSON('/api/appointments', 'POST', payload);
            close();
            await reload();
        });
    }

    async function renderGallery(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.galleryTab = host.dataset.galleryTab || 'Photos';
        host.dataset.galleryOffset = '0';
        host.innerHTML = `<div class="vd-gallery">
            <div class="vd-toolbar vd-gallery-toolbar">
                <div class="vd-segmented">
                    <button class="vd-tool-button" type="button" data-gallery-tab="Photos">${iconMarkup('image', 'P', 'vd-tool-icon', 15)}<span>${esc(t('desktop.gallery_photos'))}</span></button>
                    <button class="vd-tool-button" type="button" data-gallery-tab="Videos">${iconMarkup('video', 'V', 'vd-tool-icon', 15)}<span>${esc(t('desktop.gallery_videos'))}</span></button>
                </div>
                <span class="vd-path">${esc(t('desktop.gallery_title'))}</span>
            </div>
            <div class="vd-gallery-grid" data-gallery-grid>${esc(t('desktop.loading'))}</div>
            <div class="vd-gallery-footer">
                <button class="vd-button" type="button" data-gallery-more hidden>${esc(t('desktop.gallery_load_more'))}</button>
            </div>
        </div>`;

        const grid = host.querySelector('[data-gallery-grid]');
        const moreButton = host.querySelector('[data-gallery-more]');
        let visibleItems = [];
        const showGalleryContextMenu = (event, file, refreshGallery) => {
            event.preventDefault();
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.gallery_open', icon: 'folder-open', action: () => openMediaPreview(file) },
                { labelKey: 'desktop.gallery_download', icon: 'download', action: () => downloadMediaPath(file.web_path, file.name) },
                { labelKey: 'desktop.gallery_rename', icon: 'edit', action: async () => { await renamePath(file.path); await refreshGallery(false); } },
                { separator: true },
                { labelKey: 'desktop.gallery_delete', icon: 'trash', action: async () => { await deletePath(file.path); await refreshGallery(false); } }
            ]);
            return true;
        };
        wireContextMenuBoundary(host);

        const renderItems = (items, kind) => {
            grid.innerHTML = items.length ? items.map(file => {
                const preview = kind === 'video'
                    ? `<video src="${esc(file.web_path)}" preload="metadata" muted></video>`
                    : `<img src="${esc(file.web_path)}" alt="${esc(file.name)}" loading="lazy" decoding="async">`;
                return `<article class="vd-gallery-card" data-gallery-item data-path="${esc(file.path)}" data-web-path="${esc(file.web_path)}" data-media-kind="${esc(file.media_kind || kind)}" data-mime-type="${esc(file.mime_type || '')}" data-name="${esc(file.name)}">
                    <button type="button" class="vd-gallery-preview" data-gallery-open>${preview}</button>
                    <div class="vd-gallery-card-meta">
                        <span>${esc(file.name)}</span>
                        <div class="vd-gallery-actions">
                            <button type="button" class="vd-icon-button" data-gallery-open title="${esc(t('desktop.gallery_open'))}">${iconMarkup('folder-open', 'O', 'vd-gallery-action-icon', 14)}</button>
                            <a class="vd-icon-button" data-gallery-download href="${esc(file.web_path)}" download="${esc(file.name)}" title="${esc(t('desktop.gallery_download'))}">${iconMarkup('download', 'D', 'vd-gallery-action-icon', 14)}</a>
                            <button type="button" class="vd-icon-button" data-gallery-rename title="${esc(t('desktop.gallery_rename'))}">${iconMarkup('edit', 'E', 'vd-gallery-action-icon', 14)}</button>
                            <button type="button" class="vd-icon-button danger" data-gallery-delete title="${esc(t('desktop.gallery_delete'))}">${iconMarkup('trash', 'X', 'vd-gallery-action-icon', 14)}</button>
                        </div>
                    </div>
                </article>`;
            }).join('') : `<div class="vd-empty">${esc(t('desktop.gallery_empty'))}</div>`;
            grid.querySelectorAll('[data-gallery-item]').forEach(card => {
                const file = {
                    name: card.dataset.name,
                    path: card.dataset.path,
                    web_path: card.dataset.webPath,
                    media_kind: card.dataset.mediaKind,
                    mime_type: card.dataset.mimeType
                };
                card.querySelectorAll('[data-gallery-open]').forEach(btn => btn.addEventListener('click', () => openMediaPreview(file)));
                const rename = card.querySelector('[data-gallery-rename]');
                if (rename) rename.addEventListener('click', async () => { await renamePath(file.path); await loadGallery(false); });
                const del = card.querySelector('[data-gallery-delete]');
                if (del) del.addEventListener('click', async () => { await deletePath(file.path); await loadGallery(false); });
                card.addEventListener('contextmenu', event => showGalleryContextMenu(event, file, loadGallery));
            });
        };

        const loadGallery = async (append) => {
            const tab = host.dataset.galleryTab || 'Photos';
            host.querySelectorAll('[data-gallery-tab]').forEach(btn => btn.classList.toggle('active', btn.dataset.galleryTab === tab));
            const offset = append ? Number(host.dataset.galleryOffset || 0) : 0;
            if (!append) grid.innerHTML = esc(t('desktop.loading'));
            moreButton.hidden = true;
            try {
                const kind = tab === 'Videos' ? 'video' : 'image';
                const params = new URLSearchParams({ path: tab, recursive: 'true', limit: String(GALLERY_PAGE_SIZE), offset: String(offset) });
                const body = await api('/api/desktop/files?' + params.toString());
                const items = (body.files || []).filter(file => file.type === 'file' && file.web_path && (!file.media_kind || file.media_kind === kind));
                visibleItems = append ? visibleItems.concat(items) : items;
                host.dataset.galleryOffset = String(offset + GALLERY_PAGE_SIZE);
                renderItems(visibleItems, kind);
                moreButton.hidden = !body.has_more;
            } catch (err) {
                grid.innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
            }
        };

        host.querySelectorAll('[data-gallery-tab]').forEach(btn => {
            btn.addEventListener('click', () => {
                host.dataset.galleryTab = btn.dataset.galleryTab;
                host.dataset.galleryOffset = '0';
                visibleItems = [];
                loadGallery(false);
            });
        });
        moreButton.addEventListener('click', () => loadGallery(true));
        setGalleryMenus(id, host, () => {
            host.dataset.galleryOffset = '0';
            visibleItems = [];
            loadGallery(false);
        });
        await loadGallery(false);
    }

    function setGalleryMenus(id, host, refreshGallery) {
        setWindowMenus(id, [
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'refresh', labelKey: 'desktop.gallery_refresh', icon: 'refresh', shortcut: 'F5', action: refreshGallery }
                ]
            }
        ]);
    }

    function webampHostNode() {
        const parent = $('vd-window-layer') || document.body;
        let host = $('vd-webamp-host');
        if (!host || host.parentElement !== parent) {
            if (host) host.remove();
            host = document.createElement('div');
            host.id = 'vd-webamp-host';
            host.className = 'vd-webamp-host';
            parent.appendChild(host);
        }
        return host;
    }

    function disposeWebampMusic(windowId, options) {
        const current = state.webampMusic;
        if (!current) return;
        if (windowId && current.windowId && current.windowId !== windowId) return;
        if (current.unsubscribeClose) {
            try { current.unsubscribeClose(); } catch (_) {}
        }
        if (!options || !options.fromWebampClose) {
            if (current.instance && typeof current.instance.dispose === 'function') {
                try { current.instance.dispose(); } catch (_) {}
            }
        }
        state.webampMusic = null;
        const host = $('vd-webamp-host');
        if (host) host.remove();
    }

    async function loadWebampConstructor() {
        const mod = await import(WEBAMP_MODULE_PATH);
        return mod.default || mod.Webamp || mod;
    }

    function webampTrackTitle(name) {
        return String(name || '').replace(/\.[^.]+$/, '') || String(name || '');
    }

    async function renderMusicPlayer(id) {
        const host = contentEl(id);
        if (!host) return;
        const win = state.windows.get(id);
        if (win && win.element) {
            win.element.style.minWidth = '380px';
            win.element.style.minHeight = '220px';
        }

        let currentFolder = 'Music';
        let currentTracks = [];

        host.innerHTML = `<div class="vd-webamp-launcher">
            <div class="vd-webamp-launcher-header">
                ${iconMarkup('audio-player', 'MP', 'vd-sprite-start-item', 34)}
                <div class="vd-webamp-launcher-copy">
                    <strong>${esc(t('desktop.app_music_player'))}</strong>
                    <span data-status>${esc(t('desktop.loading'))}</span>
                </div>
            </div>
            <div class="vd-webamp-status">
                <span data-track-count>0 ${esc(t('desktop.winamp_tracks'))}</span>
                <span data-folder>Music</span>
            </div>
        </div>`;

        const statusEl = host.querySelector('[data-status]');
        const countEl = host.querySelector('[data-track-count]');
        const folderEl = host.querySelector('[data-folder]');

        const setStatus = message => {
            if (statusEl) statusEl.textContent = message;
        };

        const renderLauncherState = () => {
            if (countEl) countEl.textContent = currentTracks.length + ' ' + t('desktop.winamp_tracks');
            if (folderEl) folderEl.textContent = currentFolder;
        };

        const notifyError = err => {
            const message = err && err.message ? err.message : String(err);
            setStatus(message);
            showDesktopNotification({ title: t('desktop.notification'), message });
        };

        const scanMusicFolder = async folder => {
            const params = new URLSearchParams({ path: folder, recursive: 'true', limit: String(WEBAMP_TRACK_SCAN_LIMIT) });
            const body = await api('/api/desktop/files?' + params.toString());
            const files = body.files || [];
            const tracks = [];
            for (const file of files) {
                if (file.type === 'file' && WEBAMP_AUDIO_PATTERN.test(file.name)) {
                    tracks.push({
                        url: file.web_path || await desktopEmbedURL(file.path),
                        metaData: { title: webampTrackTitle(file.name) }
                    });
                }
            }
            return tracks;
        };

        const ensureWebamp = async tracks => {
            if (!tracks.length) {
                disposeWebampMusic(id);
                setStatus(t('desktop.winamp_no_tracks'));
                return;
            }

            const Webamp = await loadWebampConstructor();
            if (typeof Webamp.browserIsSupported === 'function' && !Webamp.browserIsSupported()) {
                throw new Error('Webamp is not supported in this browser.');
            }

            const current = state.webampMusic;
            if (current && current.instance && current.windowId === id) {
                if (typeof current.instance.reopen === 'function') current.instance.reopen();
                if (typeof current.instance.setTracksToPlay === 'function') {
                    current.instance.setTracksToPlay(tracks);
                    setStatus(t('desktop.done'));
                    return;
                }
                disposeWebampMusic(id);
            } else if (current && current.instance) {
                disposeWebampMusic(current.windowId);
            }

            const webamp = new Webamp({ initialTracks: tracks });
            state.webampMusic = { instance: webamp, windowId: id, unsubscribeClose: null };
            if (typeof webamp.onClose === 'function') {
                state.webampMusic.unsubscribeClose = webamp.onClose(() => {
                    disposeWebampMusic(id, { fromWebampClose: true });
                    closeWindow(id);
                });
            }
            await webamp.renderWhenReady(webampHostNode());
            setStatus(t('desktop.done'));
        };

        const loadMusicLibrary = async folder => {
            currentFolder = folder || 'Music';
            setStatus(t('desktop.loading'));
            currentTracks = await scanMusicFolder(currentFolder);
            renderLauncherState();
            await ensureWebamp(currentTracks);
        };

        const showMusicPlayerContextMenu = event => {
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.menu_load_folder', icon: 'folder-open', action: async () => {
                    const folder = await promptDialog(t('desktop.winamp_load_folder'), currentFolder || 'Music');
                    if (folder != null) loadMusicLibrary(folder).catch(notifyError);
                } },
                { labelKey: 'desktop.context_refresh', icon: 'refresh', action: () => loadMusicLibrary('Music').catch(notifyError) },
                { separator: true },
                { labelKey: 'desktop.menu_reopen_player', icon: 'audio-player', action: () => {
                    const current = state.webampMusic;
                    if (current && current.instance && typeof current.instance.reopen === 'function') current.instance.reopen();
                    else loadMusicLibrary(currentFolder || 'Music').catch(notifyError);
                } }
            ]);
            return true;
        };
        wireContextMenuBoundary(host, { onContextMenu: showMusicPlayerContextMenu });

        setMusicPlayerMenus(id, host, {
            refresh: () => loadMusicLibrary('Music').catch(notifyError),
            loadFolder: async () => {
                const folder = await promptDialog(t('desktop.winamp_load_folder'), currentFolder || 'Music');
                if (folder == null) return;
                loadMusicLibrary(folder).catch(notifyError);
            },
            reopen: () => {
                const current = state.webampMusic;
                if (current && current.instance && typeof current.instance.reopen === 'function') {
                    current.instance.reopen();
                    return;
                }
                loadMusicLibrary(currentFolder || 'Music').catch(notifyError);
            }
        });
        renderLauncherState();
        loadMusicLibrary('Music').catch(notifyError);
    }

    function setMusicPlayerMenus(id, host, actions) {
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'load-folder', labelKey: 'desktop.menu_load_folder', icon: 'folder-open', action: actions.loadFolder },
                    { id: 'refresh-music', labelKey: 'desktop.context_refresh', icon: 'refresh', shortcut: 'F5', action: actions.refresh }
                ]
            },
            {
                id: 'playback',
                labelKey: 'desktop.menu_playback',
                items: [
                    { id: 'reopen-webamp', labelKey: 'desktop.menu_reopen_player', icon: 'audio-player', action: actions.reopen }
                ]
            }
        ]);
    }

    function renderQuickConnect(id) {
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-quick-connect">
            <div class="vd-qc-sidebar">
                <div class="vd-qc-sidebar-header">
                    <span class="vd-qc-title">${esc(t('desktop.qc_title'))}</span>
                </div>
                <div class="vd-qc-search">
                    <input type="search" autocomplete="off" spellcheck="false" data-i18n-placeholder="desktop.qc_search_placeholder">
                </div>
                <div class="vd-qc-device-list" data-device-list>${esc(t('desktop.loading'))}</div>
            </div>
            <div class="vd-qc-terminal-area" data-terminal-area>
                <div class="vd-qc-placeholder">
                    <span class="vd-qc-placeholder-icon">${iconMarkup('terminal', 'T', 'vd-qc-placeholder-papirus-icon', 42)}</span>
                    <span class="vd-qc-placeholder-text">${esc(t('desktop.qc_select_device'))}</span>
                </div>
            </div>
        </div>`;
        wireContextMenuBoundary(host);

        const searchInput = host.querySelector('.vd-qc-search input');
        searchInput.placeholder = t('desktop.qc_search_placeholder');
        const deviceList = host.querySelector('[data-device-list]');
        const terminalArea = host.querySelector('[data-terminal-area]');
        let activeWS = null;
        let activeTerm = null;
        let activeFitAddon = null;
        let activeResizeObserver = null;
        let cachedDevices = null;
        let cachedCredentials = null;

        registerWindowCleanup(id, () => {
            if (activeWS) { try { activeWS.close(); } catch(_) {} activeWS = null; }
            if (activeTerm) { activeTerm.dispose(); activeTerm = null; }
            if (activeResizeObserver) { activeResizeObserver.disconnect(); activeResizeObserver = null; }
        });

        setQuickConnectMenus(id, host, loadAll, showServerModal);
        loadAll();

        searchInput.addEventListener('input', () => filterDevices());

        async function loadAll() {
            deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.loading'))}</div>`;
            try {
                const [devBody, credBody] = await Promise.all([
                    api('/api/devices'),
                    api('/api/credentials')
                ]);
                cachedDevices = withAuraGoHostDevice((devBody.devices || devBody || []).filter(d => d.type === 'server' || d.type === 'generic' || d.type === 'linux' || d.type === 'vm' || !d.type));
                cachedCredentials = credBody || [];
                if (!cachedDevices.length) {
                    deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.qc_no_devices'))}</div>`;
                    return;
                }
                renderDeviceList(cachedDevices);
            } catch (err) {
                deviceList.innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
            }
        }

        function auraGoHostName() {
            return (location.hostname || location.host || 'localhost').replace(/^\[|\]$/g, '');
        }

        function withAuraGoHostDevice(devices) {
            const hostName = auraGoHostName();
            const normalizedHost = hostName.toLowerCase();
            const hasAuraHost = devices.some(device => {
                const address = String(device.ip_address || '').toLowerCase();
                const name = String(device.name || '').toLowerCase();
                return address === normalizedHost || name === 'aurago host';
            });
            if (hasAuraHost) return devices;
            return [{
                id: '__aurago-host__',
                name: 'AuraGo Host',
                type: 'server',
                ip_address: hostName,
                port: 22,
                description: 'Current AuraGo web host',
                is_template: true
            }, ...devices];
        }

        function renderDeviceList(devices) {
            const query = searchInput.value.trim().toLowerCase();
            const filtered = query ? devices.filter(d =>
                (d.name || '').toLowerCase().includes(query) ||
                (d.ip_address || '').toLowerCase().includes(query) ||
                (d.description || '').toLowerCase().includes(query)
            ) : devices;
            if (!filtered.length) {
                deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.qc_no_devices'))}</div>`;
                return;
            }
            deviceList.innerHTML = filtered.map(d => {
                const cred = d.credential_id && cachedCredentials ? cachedCredentials.find(c => c.id === d.credential_id) : null;
                const endpoint = `${d.ip_address || ''}${d.port && d.port !== 22 ? ':' + d.port : ''}`;
                return `<button class="vd-qc-device${d.is_template ? ' template' : ''}" type="button" data-device-id="${esc(d.id)}">
                    <span class="vd-qc-device-icon">${iconMarkup(d.is_template ? 'home' : 'server', 'S', 'vd-qc-device-papirus-icon', 22)}</span>
                    <div class="vd-qc-device-main">
                        <div class="vd-qc-device-name">${esc(d.name)}</div>
                        <div class="vd-qc-device-meta">${esc(endpoint || '')}</div>
                        ${d.description ? `<div class="vd-qc-device-desc">${esc(d.description)}</div>` : ''}
                    </div>
                    <div class="vd-qc-device-badges">
                        ${d.is_template ? '<span class="vd-qc-badge vd-qc-badge-info">Setup</span>' : (d.credential_id ? '<span class="vd-qc-badge vd-qc-badge-ok">SSH</span>' : '<span class="vd-qc-badge vd-qc-badge-warn">?</span>')}
                    </div>
                </button>`;
            }).join('');
            deviceList.querySelectorAll('.vd-qc-device').forEach(btn => {
                btn.addEventListener('click', () => {
                    const dev = cachedDevices.find(d => d.id === btn.dataset.deviceId);
                    if (dev && dev.is_template) showServerModal(dev);
                    else connectToDevice(btn.dataset.deviceId);
                });
                btn.addEventListener('contextmenu', (e) => {
                    e.preventDefault();
                    const dev = cachedDevices.find(d => d.id === btn.dataset.deviceId);
                    if (dev) showDeviceContextMenu(e.clientX, e.clientY, dev);
                });
            });
        }

        function filterDevices() {
            if (cachedDevices) renderDeviceList(cachedDevices);
        }

        function showDeviceContextMenu(x, y, device) {
            closeContextMenu();
            const items = [
                { label: device.is_template ? t('desktop.qc_add_server') : t('desktop.qc_connect'), icon: device.is_template ? 'server' : 'terminal', fallback: 'T', action: () => device.is_template ? showServerModal(device) : connectToDevice(device.id) },
                { label: t('desktop.qc_edit'), icon: 'edit', fallback: 'E', action: () => showServerModal(device) }
            ];
            if (!device.is_template) {
                items.push({ separator: true }, { label: t('desktop.qc_delete'), icon: 'trash', fallback: 'X', action: () => confirmDeleteDevice(device) });
            }
            showContextMenu(x, y, items);
        }

        async function confirmDeleteDevice(device) {
            const ok = await showConfirmModal(t('desktop.qc_delete_confirm'), t('desktop.qc_delete_confirm_msg').replace('{{name}}', device.name));
            if (!ok) return;
            try {
                await api('/api/devices/' + device.id, { method: 'DELETE' });
                await loadAll();
            } catch (err) {
                showNotify(t('desktop.qc_delete_error') + ': ' + err.message);
            }
