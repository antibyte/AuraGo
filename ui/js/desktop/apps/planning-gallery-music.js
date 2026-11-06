    async function plannerJSON(url, method, payload) {
        return api(url, {
            method: method || 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload || {})
        });
    }

    function todoDueCivilDate(value) {
        const text = String(value || '').trim();
        const civil = /^(\d{4}-\d{2}-\d{2})T00:00:00Z$/.exec(text);
        if (civil) return civil[1];
        if (!text) return '';
        const date = new Date(text);
        if (Number.isNaN(date.getTime())) return '';
        const pad = n => String(n).padStart(2, '0');
        return date.getFullYear() + '-' + pad(date.getMonth() + 1) + '-' + pad(date.getDate());
    }

    function todoIsOverdue(todo, now) {
        if (!todo || todo.status === 'done' || !todo.due_date) return false;
        const text = String(todo.due_date).trim();
        const civil = /^(\d{4}-\d{2}-\d{2})T00:00:00Z$/.exec(text);
        if (civil) {
            const pad = n => String(n).padStart(2, '0');
            const today = now.getFullYear() + '-' + pad(now.getMonth() + 1) + '-' + pad(now.getDate());
            return civil[1] < today;
        }
        const due = new Date(text);
        if (Number.isNaN(due.getTime())) return false;
        return due < now;
    }

    function todoDueLabel(value) {
        if (!value) return '';
        const text = String(value).trim();
        const civil = /^(\d{4}-\d{2}-\d{2})T00:00:00Z$/.exec(text);
        if (civil) {
            const [year, month, day] = civil[1].split('-').map(Number);
            return new Date(year, month - 1, day).toLocaleDateString();
        }
        const date = new Date(text);
        if (Number.isNaN(date.getTime())) return text;
        return date.toLocaleDateString();
    }

    function todoDueFieldValue(originalDue, inputValue) {
        if (todoDueCivilDate(originalDue) === String(inputValue || '').trim()) {
            return { omit: true };
        }
        return { omit: false, value: inputValue };
    }

    const TODO_REFRESH_MS = 60000;
    const TODO_FILTERS = ['all', 'open', 'in_progress', 'done'];
    const TODO_STATUS_TONES = { open: 'info', in_progress: 'busy', done: 'ok' };
    const TODO_PRIORITY_TONES = { high: 'bad', medium: 'warn', low: 'muted' };

    function todoStatusTone(status) {
        return TODO_STATUS_TONES[status] || 'muted';
    }

    function todoPriorityTone(priority) {
        return TODO_PRIORITY_TONES[priority] || 'muted';
    }

    function todoCountLabel(count) {
        return t('desktop.todo_count', { count: String(count) });
    }

    // The search term narrows the visible list by title and description.
    function todoMatchesQuery(todo, query) {
        const needle = String(query || '').trim().toLowerCase();
        if (!needle) return true;
        return String(todo.title || '').toLowerCase().includes(needle)
            || String(todo.description || '').toLowerCase().includes(needle);
    }

    function todoFocusTarget(root, attribute, value) {
        const nodes = root.querySelectorAll('[' + attribute + ']');
        for (const node of nodes) {
            if (node.getAttribute(attribute) === value) return node;
        }
        return null;
    }

    // A reload rebuilds the panes, so the focused field and the caret inside it are captured before the rebuild and
    // restored afterwards. Without this, ticking a checklist point throws the cursor out of the title field the user
    // is typing in.
    function captureTodoFocus(root) {
        const active = root && root.contains(document.activeElement) ? document.activeElement : null;
        if (!active || typeof active.getAttribute !== 'function') return null;
        const state = {
            name: active.getAttribute('name') || '',
            item: active.getAttribute('data-item-title') || '',
            check: active.getAttribute('data-item-toggle') || '',
            card: active.getAttribute('data-todo-open') || ''
        };
        if (typeof active.selectionStart === 'number') {
            state.start = active.selectionStart;
            state.end = active.selectionEnd;
        }
        return state;
    }

    function restoreTodoFocus(root, state) {
        if (!root || !state) return;
        let target = null;
        if (state.item) target = todoFocusTarget(root, 'data-item-title', state.item);
        else if (state.check) target = todoFocusTarget(root, 'data-item-toggle', state.check);
        else if (state.card) target = todoFocusTarget(root, 'data-todo-open', state.card);
        else if (state.name) target = todoFocusTarget(root, 'name', state.name);
        if (!target) return;
        try { target.focus({ preventScroll: true }); } catch (_) { target.focus(); }
        if (typeof state.start === 'number' && typeof target.setSelectionRange === 'function') {
            try { target.setSelectionRange(state.start, state.end); } catch (_) { /* date fields carry no selection */ }
        }
    }

    function todoEmptyState(title, hint, icon, action) {
        const text = hint ? `<p>${esc(hint)}</p>` : '';
        return `<div class="vd-todo-empty">${iconMarkup(icon, 'T', 'vd-todo-action-icon', 20)}<strong>${esc(title)}</strong>${text}${action || ''}</div>`;
    }

    function setTodoNotice(host, tone, message) {
        const notice = host.querySelector('.vd-todo-notice');
        if (!notice) return;
        const text = host.querySelector('.vd-todo-notice-text');
        if (!message) {
            notice.hidden = true;
            if (text) text.textContent = '';
            return;
        }
        notice.dataset.tone = tone || 'info';
        if (text) text.textContent = message;
        notice.hidden = false;
    }

    function clearTodoDetail(host) {
        const pane = host.querySelector('.vd-todo-detail');
        if (pane) {
            pane.innerHTML = todoEmptyState(t('desktop.todo_select_task'), t('desktop.todo_select_hint'), 'todo');
        }
        const win = host && host.closest && host.closest('.vd-window');
        const windowId = win && win.dataset.windowId;
        if (windowId) setWindowMenus(windowId, []);
    }

    function detailIsDirty(host, todo) {
        const form = host.querySelector('.vd-todo-form');
        if (!form || !todo) return false;
        if (form.title.value.trim() !== String(todo.title || '').trim()) return true;
        if (form.description.value !== String(todo.description || '')) return true;
        if (form.priority.value !== todo.priority) return true;
        if (form.status.value !== todo.status) return true;
        if (!!form.remind_daily.checked !== !!todo.remind_daily) return true;
        if (todoDueCivilDate(todo.due_date) !== String(form.due_date.value || '').trim()) return true;
        return false;
    }

    function editedTodoFields(host) {
        const form = host.querySelector('.vd-todo-form');
        const shown = host._todoShown;
        if (!form || !shown) return null;
        const edits = {};
        if (form.title.value.trim() !== String(shown.title || '').trim()) edits.title = form.title.value.trim();
        if (form.description.value !== String(shown.description || '')) edits.description = form.description.value;
        if (form.priority.value !== shown.priority) edits.priority = form.priority.value;
        if (form.status.value !== shown.status) edits.status = form.status.value;
        if (!!form.remind_daily.checked !== !!shown.remind_daily) edits.remind_daily = form.remind_daily.checked;
        if (String(form.due_date.value || '').trim() !== String(shown.due || '')) edits.due = String(form.due_date.value || '').trim();
        return edits;
    }

    function applyEditedTodoFields(host, edits) {
        const form = host.querySelector('.vd-todo-form');
        if (!form || !edits) return;
        if ('title' in edits) form.title.value = edits.title;
        if ('description' in edits) form.description.value = edits.description;
        if ('priority' in edits) form.priority.value = edits.priority;
        if ('status' in edits) form.status.value = edits.status;
        if ('remind_daily' in edits) form.remind_daily.checked = edits.remind_daily;
        if ('due' in edits) form.due_date.value = edits.due;
    }

    // The detail pane marks the form as changed as soon as a field differs from the stored task, so an edit is never
    // lost silently when the pane is rebuilt or another task is selected.
    function syncTodoDirtyState(host, todo) {
        const badge = host.querySelector('.vd-todo-dirty');
        if (!badge) return;
        const dirty = detailIsDirty(host, todo);
        badge.hidden = !dirty;
        host.dataset.todoDirty = dirty ? '1' : '';
    }

    function revertTodoEdits(host) {
        const form = host.querySelector('.vd-todo-form');
        const shown = host._todoShown;
        if (!form || !shown) return;
        form.title.value = shown.title;
        form.description.value = shown.description;
        form.priority.value = shown.priority;
        form.status.value = shown.status;
        form.remind_daily.checked = shown.remind_daily;
        form.due_date.value = shown.due;
        const badge = host.querySelector('.vd-todo-dirty');
        if (badge) badge.hidden = true;
        host.dataset.todoDirty = '';
    }

    async function renderTodo(id) {
        const host = contentEl(id);
        if (!host) return;
        const todoReadonly = desktopReadonly();
        host.dataset.todoFilter = host.dataset.todoFilter || 'all';
        const todoAddDisabled = todoReadonly ? ' disabled' : '';
        host.innerHTML = `<div class="vd-todo-frame"><div class="vd-todo"><aside class="vd-todo-sidebar">
            ${TODO_FILTERS.map(status => `<button type="button" data-filter="${status}" class="${host.dataset.todoFilter === status ? 'active' : ''}" aria-pressed="${host.dataset.todoFilter === status ? 'true' : 'false'}"><span class="vd-todo-filter-label">${esc(t('desktop.todo_' + status))}</span><span class="vd-todo-filter-count" aria-hidden="true">0</span></button>`).join('')}
        </aside><main class="vd-todo-main"><div class="vd-todo-notice" data-tone="info" role="status" hidden><span class="vd-todo-notice-text"></span><button type="button" class="vd-todo-notice-close" aria-label="${esc(t('desktop.todo_dismiss'))}">${iconMarkup('x', 'X', 'vd-todo-action-icon', 12)}</button></div><form class="vd-todo-add"><input placeholder="${esc(t('desktop.todo_title_placeholder'))}" aria-label="${esc(t('desktop.todo_add'))}"${todoAddDisabled}><select${todoAddDisabled} aria-label="${esc(t('desktop.todo_priority'))}"><option value="low">${esc(t('desktop.todo_priority_low'))}</option><option value="medium" selected>${esc(t('desktop.todo_priority_medium'))}</option><option value="high">${esc(t('desktop.todo_priority_high'))}</option></select><button class="vd-button vd-button-primary"${todoAddDisabled}>${esc(t('desktop.todo_add'))}</button></form><div class="vd-todo-toolbar"><div class="vd-todo-search"><span class="vd-todo-search-icon" aria-hidden="true">${iconMarkup('search', 'S', 'vd-todo-action-icon', 14)}</span><input type="search" class="vd-todo-search-input" placeholder="${esc(t('desktop.todo_search_placeholder'))}" aria-label="${esc(t('desktop.todo_search_placeholder'))}"></div><span class="vd-todo-list-count"></span></div><div class="vd-todo-list">${esc(t('desktop.loading'))}</div></main><section class="vd-todo-detail">${todoEmptyState(t('desktop.todo_select_task'), t('desktop.todo_select_hint'), 'todo')}</section></div></div>`;
        registerWindowCleanup(id, window.AuraGogDesktop?.attachSources(host, 'tasks', { t }) || (() => {}));
        if (host._todoTimer) clearInterval(host._todoTimer);
        const noticeClose = host.querySelector('.vd-todo-notice-close');
        if (noticeClose) {
            noticeClose.addEventListener('click', () => setTodoNotice(host, '', ''));
        }
        async function runTodoAction(action, successMessage) {
            try {
                await action();
                if (successMessage) setTodoNotice(host, 'ok', successMessage);
            } catch (err) {
                setTodoNotice(host, 'bad', t('desktop.todo_failed'));
                showDesktopNotification({ title: t('desktop.notification'), message: t('desktop.request_failed') });
                await load(host.dataset.todoSelected, { replaceDetail: true });
            }
        }
        const showTodoContextMenu = (event, todo, reload) => {
            event.preventDefault();
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.context_open', icon: 'folder-open', action: () => renderTodoDetail(host, todo, reload, todoReadonly, runTodoAction) },
                { labelKey: 'desktop.todo_complete', icon: 'check-square', disabled: todoReadonly || todo.status === 'done', action: async () => { if (todoReadonly) return; await runTodoAction(async () => { await plannerJSON('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id, { preserveEdits: true }); }, t('desktop.todo_completed')); } },
                { separator: true },
                { labelKey: 'desktop.delete', icon: 'trash', disabled: todoReadonly, action: async () => { if (todoReadonly) return; if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await runTodoAction(async () => { await api('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); }, t('desktop.todo_deleted')); } } }
            ]);
            return true;
        };
        wireContextMenuBoundary(host);
        async function load(nextSelectedID, options = {}) {
            const priorSelectedID = host.dataset.todoSelected || '';
            host.dataset.todoGeneration = String(Number(host.dataset.todoGeneration || 0) + 1);
            const generation = host.dataset.todoGeneration;
            if (nextSelectedID !== undefined) {
                host.dataset.todoSelected = nextSelectedID || '';
            }
            let todos;
            try {
                todos = await api('/api/desktop/integrations/todos?status=all');
            } catch (err) {
                if (host.dataset.todoGeneration !== generation) return;
                const failedList = host.querySelector('.vd-todo-list');
                if (failedList) {
                    failedList.innerHTML = `<div class="vd-empty">${esc(t('desktop.load_failed'))}</div>`;
                }
                return;
            }
            if (host.dataset.todoGeneration !== generation) return;
            const selectedID = host.dataset.todoSelected || '';
            const selectionChanged = nextSelectedID !== undefined && String(nextSelectedID || '') !== String(priorSelectedID);
            const searchField = host.querySelector('.vd-todo-search-input');
            const searchTerm = searchField ? searchField.value : '';
            const filtered = todos.filter(todo => host.dataset.todoFilter === 'all' || todo.status === host.dataset.todoFilter)
                .filter(todo => todoMatchesQuery(todo, searchTerm))
                .sort((a, b) => (({ high: 0, medium: 1, low: 2 }[a.priority] ?? 3) - (({ high: 0, medium: 1, low: 2 }[b.priority] ?? 3)) || String(a.due_date || '9999').localeCompare(String(b.due_date || '9999'))));
            updateTodoFilterCounts(host, todos);
            updateTodoListMeta(host, filtered.length);
            const list = host.querySelector('.vd-todo-list');
            const listFocus = captureTodoFocus(list);
            list.innerHTML = renderTodoList(todos, filtered, selectedID, todoReadonly, searchTerm);
            restoreTodoFocus(list, listFocus);
            const clearSearch = list.querySelector('[data-todo-clear-search]');
            if (clearSearch) {
                clearSearch.addEventListener('click', () => {
                    const field = host.querySelector('.vd-todo-search-input');
                    if (field) field.value = '';
                    load(host.dataset.todoSelected);
                });
            }
            list.querySelectorAll('[data-todo-id]').forEach(card => card.addEventListener('click', () => {
                host.dataset.todoSelected = card.dataset.todoId;
                list.querySelectorAll('.vd-todo-card').forEach(other => other.classList.remove('active'));
                card.classList.toggle('active', true);
                renderTodoDetail(host, todos.find(todo => todo.id === card.dataset.todoId), load, todoReadonly, runTodoAction);
            }));
            list.querySelectorAll('[data-todo-status-toggle]').forEach(input => input.addEventListener('click', event => event.stopPropagation()));
            list.querySelectorAll('[data-todo-status-toggle]').forEach(input => input.addEventListener('change', async event => {
                event.stopPropagation();
                const todo = todos.find(item => item.id === input.dataset.todoStatusToggle);
                if (todo) await setTodoDone(todo, input.checked, load, todoReadonly, runTodoAction);
            }));
            list.querySelectorAll('[data-todo-id]').forEach(card => card.addEventListener('contextmenu', event => {
                const todo = todos.find(item => item.id === card.dataset.todoId);
                if (todo) showTodoContextMenu(event, todo, load);
            }));
            const selected = filtered.find(todo => todo.id === selectedID);
            if (selected) {
                let pendingEdits = null;
                if (!options.replaceDetail && !selectionChanged) {
                    pendingEdits = editedTodoFields(host);
                }
                const hasEdits = pendingEdits && Object.keys(pendingEdits).length > 0;
                if (hasEdits && !options.preserveEdits) {
                    // keep the open form; checklist-only refresh waits for a quiet form or preserveEdits reload
                } else {
                    renderTodoDetail(host, selected, load, todoReadonly, runTodoAction);
                    if (hasEdits && options.preserveEdits) {
                        applyEditedTodoFields(host, pendingEdits);
                    }
                }
            } else {
                clearTodoDetail(host);
            }
            host.dataset.todoLoadedAt = String(Date.now());
        }
        host._todoTimer = setInterval(() => {
            if (document.hidden) return;
            if (Date.now() - Number(host.dataset.todoLoadedAt || 0) < TODO_REFRESH_MS) return;
            load(host.dataset.todoSelected);
        }, TODO_REFRESH_MS);
        registerWindowCleanup(id, () => {
            clearInterval(host._todoTimer);
            host._todoTimer = null;
            host.dataset.todoGeneration = String(Number(host.dataset.todoGeneration || 0) + 1);
        });
        host.querySelectorAll('[data-filter]').forEach(btn => btn.addEventListener('click', () => {
            host.dataset.todoFilter = btn.dataset.filter;
            host.querySelectorAll('[data-filter]').forEach(other => {
                const active = other === btn;
                other.classList.toggle('active', active);
                other.setAttribute('aria-pressed', active ? 'true' : 'false');
            });
            load(host.dataset.todoSelected);
        }));
        const searchField = host.querySelector('.vd-todo-search-input');
        if (searchField) {
            let searchTimer = null;
            searchField.addEventListener('input', () => {
                if (searchTimer) clearTimeout(searchTimer);
                searchTimer = setTimeout(() => load(host.dataset.todoSelected), 180);
            });
        }
        const listElement = host.querySelector('.vd-todo-list');
        if (listElement) {
            // The arrow keys walk the task list, so the list is reachable without a pointer.
            listElement.addEventListener('keydown', event => {
                if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
                const title = event.target && event.target.closest ? event.target.closest('.vd-todo-card-title') : null;
                if (!title) return;
                event.preventDefault();
                const titles = Array.from(listElement.querySelectorAll('.vd-todo-card-title'));
                const index = titles.indexOf(title);
                if (index < 0) return;
                const step = event.key === 'ArrowDown' ? 1 : -1;
                const next = titles[Math.min(titles.length - 1, Math.max(0, index + step))];
                if (next) next.focus();
            });
        }
        // Ctrl+S saves the open task, Escape leaves the search, the form or the list, whichever has the focus.
        if (host._todoKeyHandler) host.removeEventListener('keydown', host._todoKeyHandler);
        host._todoKeyHandler = event => {
            if ((event.ctrlKey || event.metaKey) && String(event.key || '').toLowerCase() === 's') {
                const form = host.querySelector('.vd-todo-form');
                if (form && !todoReadonly) {
                    event.preventDefault();
                    form.requestSubmit();
                }
                return;
            }
            if (event.key !== 'Escape') return;
            const active = document.activeElement;
            const field = host.querySelector('.vd-todo-search-input');
            if (field && active === field) {
                if (field.value) {
                    event.preventDefault();
                    field.value = '';
                    load(host.dataset.todoSelected);
                }
                return;
            }
            const form = host.querySelector('.vd-todo-form');
            if (form && form.contains(active)) {
                event.preventDefault();
                revertTodoEdits(host);
                return;
            }
            const list = host.querySelector('.vd-todo-list');
            if (list && list.contains(active)) {
                event.preventDefault();
                load('');
            }
        };
        host.addEventListener('keydown', host._todoKeyHandler);
        host.querySelector('.vd-todo-add').addEventListener('submit', async event => {
            event.preventDefault();
            if (todoReadonly) return;
            const input = event.currentTarget.querySelector('input');
            const title = input.value.trim();
            if (!title) return;
            await runTodoAction(async () => {
                const result = await plannerJSON('/api/desktop/integrations/todos', 'POST', { title, priority: event.currentTarget.querySelector('select').value, status: 'open' });
                input.value = '';
                await load(result.id);
                const titleField = host.querySelector('.vd-todo-form [name="title"]');
                if (titleField) titleField.focus({ preventScroll: true });
            }, t('desktop.todo_created'));
        });
        await load();
    }

    function updateTodoFilterCounts(host, todos) {
        const counts = { all: todos.length, open: 0, in_progress: 0, done: 0 };
        todos.forEach(todo => {
            if (Object.prototype.hasOwnProperty.call(counts, todo.status)) counts[todo.status] += 1;
        });
        host.querySelectorAll('[data-filter]').forEach(btn => {
            const count = counts[btn.dataset.filter] || 0;
            const badge = btn.querySelector('.vd-todo-filter-count');
            if (badge) badge.textContent = String(count);
            btn.setAttribute('aria-label', t('desktop.todo_' + btn.dataset.filter) + ': ' + todoCountLabel(count));
        });
    }

    function updateTodoListMeta(host, shown) {
        const meta = host.querySelector('.vd-todo-list-count');
        if (meta) meta.textContent = todoCountLabel(shown);
    }

    // The list explains why it is empty: no task exists at all, the active filter hides every task, or the search
    // term matches nothing. Each case offers the way out of it.
    function renderTodoList(todos, filtered, selectedID, todoReadonly, searchTerm) {
        if (filtered.length) return filtered.map(todo => renderTodoCard(todo, selectedID, todoReadonly)).join('');
        if (!todos.length) {
            return todoEmptyState(t('desktop.todo_empty_title'), t('desktop.todo_empty_hint'), 'todo');
        }
        if (String(searchTerm || '').trim()) {
            return todoEmptyState(t('desktop.todo_no_results', { query: String(searchTerm).trim() }), '', 'search',
                `<button type="button" class="vd-button vd-todo-clear-search" data-todo-clear-search>${esc(t('desktop.todo_clear_search'))}</button>`);
        }
        return `<div class="vd-empty">${esc(t('desktop.todo_empty'))}</div>`;
    }

    function renderTodoCard(todo, selectedID, todoReadonly) {
        const now = new Date();
        const overdue = todoIsOverdue(todo, now);
        const dueLabel = todo.due_date ? todoDueLabel(todo.due_date) : '';
        const cardDoneDisabled = todoReadonly ? ' disabled' : '';
        const percent = Math.max(0, Math.min(100, Math.round(Number(todo.progress_percent) || 0)));
        const chips = [`<span class="vd-todo-chip" data-tone="${todoStatusTone(todo.status)}">${esc(t('desktop.todo_' + todo.status))}</span>`];
        if (todo.remind_daily) {
            chips.push(`<span class="vd-todo-chip" data-tone="muted">${iconMarkup('bell', 'B', 'vd-todo-action-icon', 12)}<span>${esc(t('desktop.todo_remind_daily'))}</span></span>`);
        }
        if (dueLabel) {
            chips.push(`<span class="vd-todo-chip" data-tone="muted">${iconMarkup('calendar', 'C', 'vd-todo-action-icon', 12)}<span>${esc(dueLabel)}</span></span>`);
        }
        if (overdue) {
            chips.push(`<span class="vd-todo-chip" data-tone="bad">${esc(t('desktop.todo_overdue'))}</span>`);
        }
        const progress = percent
            ? `<div class="vd-todo-progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${percent}" aria-label="${esc(t('desktop.todo_progress_aria', { percent: String(percent) }))}"><span style="width:${percent}%"></span></div>`
            : '';
        return `<article class="vd-todo-card ${todo.id === selectedID ? 'active' : ''} ${overdue ? 'overdue' : ''}" data-todo-id="${esc(todo.id)}">
            <div class="vd-todo-card-grid">
                <input class="vd-todo-card-done" type="checkbox" data-todo-status-toggle="${esc(todo.id)}" ${todo.status === 'done' ? 'checked' : ''}${cardDoneDisabled} aria-label="${esc(t('desktop.todo_complete'))}">
                <div class="vd-todo-card-copy">
                    <button type="button" class="vd-todo-card-title" data-todo-open="${esc(todo.id)}">${esc(todo.title)}</button>
                    <div class="vd-todo-card-chips">${chips.join('')}</div>
                </div>
                <span class="vd-todo-priority ${esc(todo.priority)}" data-tone="${todoPriorityTone(todo.priority)}">${esc(t('desktop.todo_priority_' + todo.priority))}</span>
            </div>
            ${progress}
        </article>`;
    }

    async function setTodoDone(todo, done, reload, todoReadonly, runTodoAction) {
        if (todoReadonly) return;
        if (!todo) return;
        if (!runTodoAction) return;
        await runTodoAction(async () => {
            if (done) {
                await plannerJSON('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true });
                await reload(todo.id, { preserveEdits: true });
                return;
            }
            await plannerJSON('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id), 'PUT', { status: 'open' });
            await reload(todo.id, { preserveEdits: true });
        }, done ? t('desktop.todo_completed') : t('desktop.todo_reopened'));
    }

    async function updateTodoItem(todo, itemID, patch, reload) {
        if (!todo || !itemID) return;
        await plannerJSON('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id) + '/items/' + encodeURIComponent(itemID), 'PUT', patch);
        await reload(todo.id, { preserveEdits: true });
    }

    // The checklist titles the user is typing in are kept until they are saved, so a reload of the pane does not
    // swallow a half-written point.
    function todoPendingItemTitles(host, todoID) {
        const store = host._todoItemTitles || {};
        if (store.id !== todoID) {
            store.id = todoID;
            store.titles = {};
            host._todoItemTitles = store;
        }
        return store.titles;
    }

    function renderTodoDetail(host, todo, reload, todoReadonly, runTodoAction) {
        host.dataset.todoSelected = todo.id;
        const pane = host.querySelector('.vd-todo-detail');
        const items = todo.items || [];
        const todoDetailDisabled = todoReadonly ? ' disabled' : '';
        const readonlyHint = todoReadonly ? `<p class="vd-hint">${esc(t('desktop.todo_readonly'))}</p>` : '';
        const focus = captureTodoFocus(pane);
        const pendingTitles = todoPendingItemTitles(host, todo.id);
        pane.innerHTML = `${readonlyHint}<header class="vd-todo-detail-head"><h2 class="vd-todo-detail-title">${esc(todo.title)}</h2><span class="vd-todo-chip" data-tone="${todoStatusTone(todo.status)}">${esc(t('desktop.todo_' + todo.status))}</span><span class="vd-todo-dirty" hidden>${esc(t('desktop.todo_unsaved_changes'))}</span></header><form class="vd-todo-form"><label class="vd-todo-field"><span class="vd-todo-label">${esc(t('desktop.todo_title_label'))}</span><input name="title" value="${esc(todo.title)}"${todoDetailDisabled}></label><label class="vd-todo-field"><span class="vd-todo-label">${esc(t('desktop.todo_description'))}</span><textarea name="description"${todoDetailDisabled}>${esc(todo.description || '')}</textarea></label><div class="vd-todo-form-row"><label class="vd-todo-field"><span class="vd-todo-label">${esc(t('desktop.todo_priority'))}</span><select name="priority"${todoDetailDisabled}>${['low','medium','high'].map(p => `<option value="${p}" ${todo.priority === p ? 'selected' : ''}>${esc(t('desktop.todo_priority_' + p))}</option>`).join('')}</select></label><label class="vd-todo-field"><span class="vd-todo-label">${esc(t('desktop.cal_status'))}</span><select name="status"${todoDetailDisabled}>${['open','in_progress','done'].map(s => `<option value="${s}" ${todo.status === s ? 'selected' : ''}>${esc(t('desktop.todo_' + s))}</option>`).join('')}</select></label><label class="vd-todo-field"><span class="vd-todo-label">${esc(t('desktop.todo_due_date'))}</span><input type="date" name="due_date" value="${esc(todoDueCivilDate(todo.due_date))}"${todoDetailDisabled}></label></div><label class="vd-check"><input type="checkbox" name="remind_daily" ${todo.remind_daily ? 'checked' : ''}${todoDetailDisabled}>${esc(t('desktop.todo_remind_daily'))}</label><div class="vd-todo-actions"><button class="vd-button vd-button-primary" data-action="save"${todoDetailDisabled}>${esc(t('desktop.save'))}</button><button type="button" class="vd-button" data-action="complete"${todoDetailDisabled}>${esc(t('desktop.todo_complete'))}</button><button type="button" class="vd-button vd-todo-button-danger" data-action="delete"${todoDetailDisabled}>${esc(t('desktop.delete'))}</button></div></form><h3 class="vd-todo-subhead">${esc(t('desktop.todo_items'))}</h3><form class="vd-todo-item-add"><input placeholder="${esc(t('desktop.todo_add_item'))}"${todoDetailDisabled}><button class="vd-button"${todoDetailDisabled}>${esc(t('desktop.todo_add_item'))}</button></form><div class="vd-todo-items">${items.length ? items.map(item => `<div class="vd-todo-item" data-item-id="${esc(item.id)}"><input class="vd-todo-item-check" type="checkbox" data-item-toggle="${esc(item.id)}" ${item.is_done ? 'checked' : ''}${todoDetailDisabled} aria-label="${esc(t('desktop.todo_complete'))}"><input class="vd-todo-item-title" data-item-title="${esc(item.id)}" value="${esc(pendingTitles[item.id] !== undefined ? pendingTitles[item.id] : item.title)}" aria-label="${esc(t('desktop.todo_add_item'))}" spellcheck="true"${todoDetailDisabled}><button type="button" class="vd-todo-item-delete" data-item-delete="${esc(item.id)}" title="${esc(t('desktop.delete'))}"${todoDetailDisabled}>${iconMarkup('x', 'X', 'vd-todo-action-icon', 13)}</button></div>`).join('') : `<p class="vd-todo-items-empty">${esc(t('desktop.todo_checklist_empty'))}</p>`}</div>`;
        restoreTodoFocus(pane, focus);
        syncTodoDirtyState(host, todo);
        pane.querySelector('.vd-todo-form').addEventListener('submit', async event => {
            event.preventDefault();
            if (todoReadonly) return;
            await runTodoAction(async () => {
                const form = event.currentTarget;
                const body = {
                    title: form.title.value.trim(),
                    description: form.description.value,
                    priority: form.priority.value,
                    status: form.status.value,
                    remind_daily: form.remind_daily.checked
                };
                const due = todoDueFieldValue(todo.due_date, form.due_date.value);
                if (!due.omit) body.due_date = due.value;
                await plannerJSON('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id), 'PUT', body);
                await reload(todo.id, { replaceDetail: true });
            }, t('desktop.todo_saved'));
        });
        pane.querySelector('.vd-todo-form').addEventListener('input', () => syncTodoDirtyState(host, todo));
        pane.querySelector('.vd-todo-form').addEventListener('change', () => syncTodoDirtyState(host, todo));
        pane.querySelector('[data-action="complete"]').addEventListener('click', async () => { if (todoReadonly) return; await runTodoAction(async () => { await plannerJSON('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id, { preserveEdits: true }); }, t('desktop.todo_completed')); });
        pane.querySelector('[data-action="delete"]').addEventListener('click', async () => { if (todoReadonly) return; if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await runTodoAction(async () => { await api('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); }, t('desktop.todo_deleted')); } });
        pane.querySelector('.vd-todo-item-add').addEventListener('submit', async event => { event.preventDefault(); if (todoReadonly) return; const input = event.currentTarget.querySelector('input'); const title = input.value.trim(); if (!title) return; await runTodoAction(async () => { await plannerJSON('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id) + '/items', 'POST', { title }); await reload(todo.id, { preserveEdits: true }); const next = host.querySelector('.vd-todo-item-add input'); if (next) next.focus({ preventScroll: true }); }); });
        pane.querySelectorAll('[data-item-toggle]').forEach(input => input.addEventListener('change', async () => { if (todoReadonly) return; await runTodoAction(async () => { await updateTodoItem(todo, input.dataset.itemToggle, { is_done: input.checked }, reload); }); }));
        pane.querySelectorAll('[data-item-title]').forEach(titleInput => {
            titleInput.addEventListener('input', () => {
                pendingTitles[titleInput.dataset.itemTitle] = titleInput.value;
            });
            titleInput.addEventListener('keydown', async event => {
                if (event.key === 'Enter') {
                    event.preventDefault();
                    titleInput.blur();
                } else if (event.key === 'Escape') {
                    event.preventDefault();
                    event.stopPropagation();
                    const item = items.find(entry => entry.id === titleInput.dataset.itemTitle);
                    titleInput.value = item ? item.title : '';
                    delete pendingTitles[titleInput.dataset.itemTitle];
                    titleInput.blur();
                }
            });
            titleInput.addEventListener('change', async () => {
                if (todoReadonly) return;
                const title = titleInput.value.trim();
                const item = items.find(entry => entry.id === titleInput.dataset.itemTitle);
                if (!title || !item || title === item.title) {
                    titleInput.value = item ? item.title : '';
                    return;
                }
                delete pendingTitles[titleInput.dataset.itemTitle];
                await runTodoAction(async () => { await updateTodoItem(todo, titleInput.dataset.itemTitle, { title: titleInput.value.trim() }, reload); });
            });
        });
        pane.querySelectorAll('[data-item-delete]').forEach(btn => btn.addEventListener('click', async () => { if (todoReadonly) return; delete pendingTitles[btn.dataset.itemDelete]; await runTodoAction(async () => { await api('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id) + '/items/' + encodeURIComponent(btn.dataset.itemDelete), { method: 'DELETE' }); await reload(todo.id, { preserveEdits: true }); }); }));
        host._todoShown = {
            title: todo.title,
            description: todo.description || '',
            priority: todo.priority,
            status: todo.status,
            remind_daily: !!todo.remind_daily,
            due: todoDueCivilDate(todo.due_date)
        };
        setTodoMenus(host, todo, reload, todoReadonly, runTodoAction);
    }

    function setTodoMenus(host, todo, reload, todoReadonly, runTodoAction) {
        const win = host && host.closest && host.closest('.vd-window');
        const id = win && win.dataset.windowId;
        if (!id || !todo) return;
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'save', labelKey: 'desktop.save', icon: 'save', shortcut: 'Ctrl+S', disabled: todoReadonly, action: () => {
                        if (todoReadonly) return;
                        const form = host.querySelector('.vd-todo-form');
                        if (form) form.requestSubmit();
                    } }
                ]
            },
            {
                id: 'edit',
                labelKey: 'desktop.menu_edit',
                items: [
                    { id: 'complete', labelKey: 'desktop.todo_complete', icon: 'check-square', disabled: todoReadonly, action: async () => { if (todoReadonly) return; await runTodoAction(async () => { await plannerJSON('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id, { preserveEdits: true }); }, t('desktop.todo_completed')); } },
                    { id: 'delete', labelKey: 'desktop.delete', icon: 'trash', disabled: todoReadonly, action: async () => { if (todoReadonly) return; if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await runTodoAction(async () => { await api('/api/desktop/integrations/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); }, t('desktop.todo_deleted')); } } }
                ]
            }
        ]);
    }

    function galleryAppContext(context) {
        return Object.assign({}, context || {}, {
            t,
            esc,
            api,
            iconMarkup,
            fmtBytes,
            notify: showDesktopNotification,
            mediaPreviewURL,
            mediaDownloadURL,
            mediaPreviewKind,
            readonly: desktopReadonly(),
            pageSize: GALLERY_PAGE_SIZE,
            animationsEnabled,
            wireContextMenuBoundary,
            setWindowMenus,
            clearWindowMenus,
            showContextMenu,
            registerWindowCleanup,
            confirmDialog,
            promptDialog,
            settingBool,
            desktopSound,
            openApp,
            downloadMediaPath,
            openMediaPreview,
            afterFileChange: refreshDesktopAfterFileChange
        });
    }

    async function renderGallery(id, context) {
        const host = contentEl(id);
        if (!host) return;
        const app = window.GalleryApp;
        if (!app || typeof app.render !== 'function') {
            host.innerHTML = `<div class="vd-empty">${esc(t('desktop.load_failed'))}</div>`;
            return;
        }
        const ctx = galleryAppContext(context);
        if (!ctx.tab && host.dataset.galleryTab) ctx.tab = host.dataset.galleryTab;
        return app.render(host, id, ctx);
    }

    async function renameMediaFile(file) {
        if (!file || !file.path) return null;
        const current = String(file.path).split('/').pop();
        const name = await promptDialog(t('desktop.rename'), current);
        if (!name || name === current) return null;
        const newPath = workspaceJoinPath(pathDir(file.path), name);
        try {
            await api('/api/desktop/file', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ old_path: file.path, new_path: newPath })
            });
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err && err.message ? err.message : t('desktop.load_failed') });
            return null;
        }
        refreshDesktopAfterFileChange();
        const updated = Object.assign({}, file, { path: newPath, name });
        if (file.web_path) updated.web_path = String(file.web_path).replace(/[^/]+$/, encodeURIComponent(name));
        return updated;
    }

    async function deleteMediaFile(file) {
        if (!file || !file.path) return false;
        if (settingBool('files.confirm_delete')) {
            const confirmed = await confirmDialog(t('desktop.gallery_delete_one_title', { name: file.name || file.path }), t('desktop.gallery_delete_msg'));
            if (!confirmed) return false;
        }
        try {
            await api('/api/desktop/file?path=' + encodeURIComponent(file.path), { method: 'DELETE' });
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err && err.message ? err.message : t('desktop.load_failed') });
            return false;
        }
        desktopSound('file.delete');
        refreshDesktopAfterFileChange();
        return true;
    }

    function openMediaLightbox(file) {
        const modules = window.AuraDesktopModules;
        const ready = window.GalleryLightbox && typeof window.GalleryLightbox.open === 'function';
        const load = ready || !modules || typeof modules.loadAppAssets !== 'function'
            ? Promise.resolve()
            : modules.loadAppAssets('gallery');
        return load.then(() => {
            const lightbox = window.GalleryLightbox;
            if (!lightbox || typeof lightbox.open !== 'function') return false;
            const helpers = window.GalleryApp || {};
            const readonly = desktopReadonly();
            lightbox.open({
                items: [file],
                index: 0,
                context: {
                    t,
                    esc,
                    iconMarkup,
                    fmtBytes,
                    notify: showDesktopNotification,
                    mediaPreviewURL,
                    mediaDownloadURL,
                    mediaPreviewKind,
                    readonly,
                    animationsEnabled: animationsEnabled(),
                    formatDateTime: helpers.formatDateTime,
                    formatDuration: helpers.formatDuration,
                    kindLabel: kind => t(kind === 'video' ? 'desktop.gallery_kind_video' : kind === 'audio' ? 'desktop.gallery_kind_audio' : 'desktop.gallery_kind_image')
                },
                actions: {
                    download: item => downloadMediaPath(mediaDownloadURL(item), item && item.name),
                    rename: readonly ? null : renameMediaFile,
                    remove: readonly ? null : deleteMediaFile,
                    edit: item => { if (item && item.path) openApp('pixel', { path: item.path }); },
                    reveal: item => { if (item && item.path) openApp('files', { path: pathDir(item.path) }); }
                }
            });
            return true;
        });
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
        if (windowId && current.windowId !== windowId) return;
        if (current.unsubscribeClose) {
            try { current.unsubscribeClose(); } catch (_) {}
        }
        if (!options || !options.fromWebampClose) {
            if (current.instance && typeof current.instance.dispose === 'function') {
                try { current.instance.dispose(); } catch (_) {}
            }
        }
        state.webampMusic = null;
        notifyWebampMediaSessionStopped();
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

    async function scanWebampTracks(folder) {
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
    }

    async function ensureWebampMusic(tracks, ownerWindowId, callbacks) {
        const hooks = callbacks || {};
        if (!tracks.length) {
            disposeWebampMusic(ownerWindowId);
            if (typeof hooks.setStatus === 'function') hooks.setStatus(t('desktop.winamp_no_tracks'));
            if (hooks.notifyEmpty !== false) showDesktopNotification({ title: t('desktop.notification'), message: t('desktop.winamp_no_tracks') });
            return false;
        }

        const Webamp = await loadWebampConstructor();
        if (typeof Webamp.browserIsSupported === 'function' && !Webamp.browserIsSupported()) {
            throw new Error(t('desktop.winamp_unsupported'));
        }

        const current = state.webampMusic;
        if (current && current.instance && (!ownerWindowId || current.windowId === ownerWindowId)) {
            if (typeof current.instance.reopen === 'function') current.instance.reopen();
            if (typeof current.instance.setTracksToPlay === 'function') {
                current.instance.setTracksToPlay(tracks);
                if (typeof hooks.setStatus === 'function') hooks.setStatus(t('desktop.done'));
                notifyWebampMediaSessionChanged();
                return true;
            }
            disposeWebampMusic(ownerWindowId || current.windowId);
        } else if (current && current.instance) {
            disposeWebampMusic(current.windowId);
        }

        const webamp = new Webamp({ initialTracks: tracks });
        state.webampMusic = { instance: webamp, windowId: ownerWindowId || '', unsubscribeClose: null };
        if (typeof webamp.onClose === 'function') {
            state.webampMusic.unsubscribeClose = webamp.onClose(() => {
                disposeWebampMusic(ownerWindowId || '', { fromWebampClose: true });
                if (ownerWindowId) closeWindow(ownerWindowId);
            });
        }
        await webamp.renderWhenReady(webampHostNode());
        if (typeof hooks.setStatus === 'function') hooks.setStatus(t('desktop.done'));
        notifyWebampMediaSessionChanged();
        return true;
    }

    async function launchStandaloneWebamp(context) {
        const folder = normalizeDesktopPath((context && context.path) || 'Music') || 'Music';
        const tracks = await scanWebampTracks(folder);
        await ensureWebampMusic(tracks, '', { notifyEmpty: true });
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
            const raw = err && err.message ? String(err.message) : String(err || '');
            const unsupported = t('desktop.winamp_unsupported');
            const message = (raw === 'Webamp is not supported in this browser.' || raw === unsupported)
                ? unsupported
                : t('desktop.load_failed');
            setStatus(message);
            showDesktopNotification({ title: t('desktop.notification'), message });
        };

        const loadMusicLibrary = async folder => {
            currentFolder = folder || 'Music';
            setStatus(t('desktop.loading'));
            currentTracks = await scanWebampTracks(currentFolder);
            renderLauncherState();
            await ensureWebampMusic(currentTracks, id, { setStatus, notifyEmpty: false });
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
