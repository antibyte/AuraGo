/* AuraGo – Knowledge Center: To-Dos */
/* global t, esc, showToast, closeModal */

let allTodos = [];
let todoSearchTimer = null;
let todoDraftItems = [];
let todoDraftFocusIndex = -1;

function debounceTodoSearch() {
    clearTimeout(todoSearchTimer);
    todoSearchTimer = setTimeout(() => loadTodos(), 300);
}

async function loadTodos() {
    const q = (document.getElementById('todos-search')?.value || '').trim();
    const status = document.getElementById('todos-filter')?.value || '';
    let url = '/api/todos?';
    if (q) url += 'q=' + encodeURIComponent(q) + '&';
    if (status) url += 'status=' + encodeURIComponent(status);

    try {
        const response = await fetch(url);
        if (!response.ok) throw new Error(response.status + ' ' + response.statusText);
        allTodos = await response.json() || [];
        renderTodos();
    } catch (error) {
        console.error('Failed to load todos:', error);
        showToast(t('common.error') + ': ' + error.message, 'error');
    }
}

function renderTodos() {
    const list = document.getElementById('todos-list');
    const empty = document.getElementById('todos-empty');
    const summary = document.getElementById('todos-summary');

    if (!allTodos.length) {
        list.innerHTML = '';
        list.classList.add('is-hidden');
        empty.classList.remove('is-hidden');
        if (summary) {
            summary.innerHTML = '';
            summary.classList.add('is-hidden');
        }
        return;
    }

    list.classList.remove('is-hidden');
    empty.classList.add('is-hidden');
    if (summary) {
        summary.innerHTML = renderTodoSummary();
        summary.classList.remove('is-hidden');
    }
    list.innerHTML = allTodos.map(renderTodoCard).join('');
}

function renderTodoSummary() {
    const activeFilter = document.getElementById('todos-filter')?.value || '';
    const openCount = allTodos.filter(todo => todo.status === 'open').length;
    const inProgressCount = allTodos.filter(todo => todo.status === 'in_progress').length;
    const doneCount = allTodos.filter(todo => todo.status === 'done').length;
    const avgProgress = allTodos.length
        ? Math.round(allTodos.reduce((sum, todo) => sum + (Number(todo.progress_percent) || 0), 0) / allTodos.length)
        : 0;

    return [
        summaryCard('📥', t('knowledge.todos_status_open'), openCount, t('knowledge.todos_filter_open'), 'open', activeFilter === 'open'),
        summaryCard('⚙️', t('knowledge.todos_status_in_progress'), inProgressCount, t('knowledge.todos_filter_in_progress'), 'in_progress', activeFilter === 'in_progress'),
        summaryCard('✅', t('knowledge.todos_status_done'), doneCount, t('knowledge.todos_filter_done'), 'done', activeFilter === 'done'),
        summaryCard('📊', t('knowledge.todos_summary_progress'), avgProgress + '%', t('knowledge.todos_summary_total').replace('{count}', String(allTodos.length)), '', activeFilter === ''),
    ].join('');
}

function summaryCard(icon, label, value, subtext, filterValue, isActive) {
    return `
        <button type="button" class="kc-todo-summary-card ${isActive ? 'is-active' : ''}" onclick='applyTodoSummaryFilter(${quoteJS(filterValue || "")})'>
            <div class="kc-todo-summary-label"><span>${icon}</span><span>${esc(label)}</span></div>
            <div class="kc-todo-summary-value">${esc(String(value))}</div>
            <div class="kc-todo-summary-subtext">${esc(subtext || '')}</div>
        </button>
    `;
}

function applyTodoSummaryFilter(value) {
    const filter = document.getElementById('todos-filter');
    if (!filter) return;
    filter.value = value || '';
    loadTodos();
}

function renderTodoCard(todo) {
    const isDone = todo.status === 'done';
    const priorityClass = todoPriorityClass(todo.priority);
    const priorityLabel = todoPriorityLabel(todo.priority);
    const statusLabel = todoStatusLabel(todo.status);
    const due = todo.due_date ? formatTodoDueDate(todo.due_date) : '';
    const isOverdue = todo.due_date && !isDone && new Date(todo.due_date) < new Date();
    const progress = Number.isFinite(todo.progress_percent) ? todo.progress_percent : 0;
    const hasItems = Array.isArray(todo.items) && todo.items.length > 0;
    const detailsOpen = !isDone || todo.status === 'in_progress';
    const reminderBadge = todo.remind_daily
        ? `<span class="kc-status-pill kc-todo-reminder">🔔 ${t('knowledge.todos_remind_daily_badge')}</span>`
        : '';

    return `
    <div class="kc-todo-item ${isDone ? 'kc-todo-done' : ''} ${isOverdue ? 'kc-todo-overdue' : ''}">
        <div class="kc-todo-header">
            <div class="kc-todo-check">
                <input type="checkbox" ${isDone ? 'checked' : ''}
                    onchange='toggleTodoStatus(${quoteJS(todo.id)}, this.checked)'
                    title="${isDone ? esc(t('knowledge.todos_reopen')) : esc(t('knowledge.todos_mark_done'))}">
            </div>
            <div class="kc-todo-main">
                <div class="kc-todo-title-row">
                    <span class="kc-todo-title ${isDone ? 'kc-todo-title-done' : ''}">${esc(todo.title)}</span>
                    <span class="kc-priority-badge ${priorityClass}">${esc(priorityLabel)}</span>
                    <span class="kc-status-pill ${todo.status === 'in_progress' ? 'kc-status-inprogress' : ''}">${esc(statusLabel)}</span>
                    ${reminderBadge}
                </div>
                ${todo.description ? `<p class="kc-todo-desc">${esc(todo.description)}</p>` : ''}
                <div class="kc-todo-progress-row">
                    <div class="kc-todo-progress-bar" aria-hidden="true">
                        <div class="kc-todo-progress-fill" style="width:${Math.max(0, Math.min(progress, 100))}%"></div>
                    </div>
                    <span class="kc-todo-progress-text">${progress}%</span>
                </div>
                <div class="kc-todo-meta">
                    ${hasItems ? `<span class="kc-todo-meta-item">☑️ ${esc(t('knowledge.todos_items_progress').replace('{done}', String(todo.done_item_count || 0)).replace('{total}', String(todo.item_count || 0)))}</span>` : `<span class="kc-todo-meta-item">${esc(t('knowledge.todos_no_items'))}</span>`}
                    ${due ? `<span class="kc-todo-meta-item ${isOverdue ? 'kc-todo-overdue-text' : ''}">📅 ${esc(due)}</span>` : ''}
                    <span class="kc-todo-meta-item kc-todo-created">🕐 ${esc(formatTodoDate(todo.created_at))}</span>
                </div>
                ${hasItems ? renderTodoItems(todo, detailsOpen) : ''}
            </div>
            <div class="kc-todo-actions">
                ${!isDone && todo.status !== 'in_progress' ? `<button class="btn btn-sm btn-secondary" onclick='setTodoInProgress(${quoteJS(todo.id)})' title="${esc(t('knowledge.todos_start'))}">▶️</button>` : ''}
                <button class="btn btn-sm btn-secondary" onclick='editTodo(${quoteJS(todo.id)})' title="${esc(t('common.btn_edit'))}">✏️</button>
                <button class="btn btn-sm btn-danger" onclick='askDeleteTodo(${quoteJS(todo.id)}, ${quoteJS(todo.title)})' title="${esc(t('common.btn_delete'))}">🗑️</button>
            </div>
        </div>
    </div>`;
}

function renderTodoItems(todo, detailsOpen) {
    const items = (todo.items || []).map(item => `
        <label class="kc-todo-subtask ${item.is_done ? 'kc-todo-subtask-done' : ''}">
            <input type="checkbox" ${item.is_done ? 'checked' : ''}
                onchange='toggleTodoItem(${quoteJS(todo.id)}, ${quoteJS(item.id)}, this.checked)'>
            <span class="kc-todo-subtask-title">
                ${esc(item.title)}
                ${item.description ? `<span class="kc-todo-subtask-desc">${esc(item.description)}</span>` : ''}
            </span>
            <span class="kc-todo-subtask-meta">${item.is_done ? esc(t('knowledge.todos_item_done')) : esc(t('knowledge.todos_item_open'))}</span>
        </label>
    `).join('');

    return `
    <details class="kc-todo-subtasks" ${detailsOpen ? 'open' : ''}>
        <summary>
            <span>${esc(t('knowledge.todos_items_label'))}</span>
            <span class="kc-todo-subtasks-count">${esc(t('knowledge.todos_items_progress').replace('{done}', String(todo.done_item_count || 0)).replace('{total}', String(todo.item_count || 0)))}</span>
        </summary>
        <div class="kc-todo-subtasks-list">${items}</div>
    </details>`;
}

function openTodoModal(todo) {
    const modal = document.getElementById('todo-modal');
    const title = document.getElementById('todo-modal-title');
    const remindDailyInput = document.getElementById('todo-remind-daily');

    document.getElementById('todo-id').value = todo ? todo.id : '';
    document.getElementById('todo-title').value = todo ? todo.title : '';
    document.getElementById('todo-description').value = todo ? todo.description || '' : '';
    document.getElementById('todo-priority').value = todo ? todo.priority : 'medium';
    document.getElementById('todo-due-date').value = todo && todo.due_date ? toLocalDateInput(todo.due_date) : '';
    if (remindDailyInput) {
        remindDailyInput.checked = !!(todo && todo.remind_daily);
    }
    todoDraftItems = normalizeTodoDraftItems(todo && Array.isArray(todo.items) ? todo.items : []);
    renderTodoDraftItems();

    title.textContent = todo ? t('knowledge.todos_edit') : t('knowledge.todos_add');
    modal.classList.add('active');
}

function editTodo(id) {
    fetchTodoAndOpenModal(id).catch(error => {
        console.error('Load todo for edit failed:', error);
        showToast(t('common.error') + ': ' + error.message, 'error');
    });
}

async function fetchTodoAndOpenModal(id) {
    const localTodo = allTodos.find(entry => entry.id === id);
    const controller = typeof AbortController !== 'undefined' ? new AbortController() : null;
    const timeout = controller ? setTimeout(() => controller.abort(), 10000) : null;
    try {
        const response = await fetch('/api/todos/' + encodeURIComponent(id), {
            signal: controller ? controller.signal : undefined,
        });
        if (!response.ok) throw new Error(await response.text());
        const todo = await response.json();
        openTodoModal(todo);
    } catch (error) {
        if (localTodo) {
            openTodoModal(localTodo);
            showToast((t('common.warning') || 'Warning') + ': ' + error.message, 'warning');
            return;
        }
        throw error;
    } finally {
        if (timeout) clearTimeout(timeout);
    }
}

function addTodoDraftItem(item) {
    todoDraftItems.push({
        id: item && item.id ? item.id : '',
        title: item && item.title ? item.title : '',
        description: item && item.description ? item.description : '',
        is_done: !!(item && item.is_done),
        position: item && Number.isFinite(item.position) ? item.position : todoDraftItems.length,
    });
    todoDraftFocusIndex = todoDraftItems.length - 1;
    renderTodoDraftItems();
}

function updateTodoDraftItem(index, field, value) {
    if (!todoDraftItems[index]) return;
    todoDraftItems[index][field] = value;
}

function toggleTodoDraftItemDone(index, checked) {
    if (!todoDraftItems[index]) return;
    todoDraftItems[index].is_done = !!checked;
    renderTodoDraftItems();
}

function moveTodoDraftItem(index, direction) {
    const targetIndex = index + direction;
    if (targetIndex < 0 || targetIndex >= todoDraftItems.length) return;
    const current = todoDraftItems[index];
    todoDraftItems[index] = todoDraftItems[targetIndex];
    todoDraftItems[targetIndex] = current;
    renderTodoDraftItems();
}

function removeTodoDraftItem(index) {
    todoDraftItems.splice(index, 1);
    renderTodoDraftItems();
}

function renderTodoDraftItems() {
    const list = document.getElementById('todo-items-editor');
    const empty = document.getElementById('todo-items-empty');
    if (!list || !empty) return;

    if (!todoDraftItems.length) {
        list.innerHTML = '';
        empty.classList.remove('is-hidden');
        return;
    }

    empty.classList.add('is-hidden');
    list.innerHTML = todoDraftItems.map((item, index) => `
        <div class="kc-todo-editor-item">
            <div class="kc-todo-editor-fields">
                <div class="kc-todo-editor-head">
                    <label class="kc-todo-editor-check">
                        <input type="checkbox" ${item.is_done ? 'checked' : ''} onchange="toggleTodoDraftItemDone(${index}, this.checked)">
                        <span>${esc(item.is_done ? t('knowledge.todos_item_done') : t('knowledge.todos_item_open'))}</span>
                    </label>
                </div>
                <input type="text"
                    id="todo-item-title-${index}"
                    value="${esc(item.title || '')}"
                    maxlength="200"
                    placeholder="${esc(t('knowledge.todos_item_title_placeholder'))}"
                    oninput="updateTodoDraftItem(${index}, 'title', this.value)"
                    onkeydown="handleTodoDraftTitleKeydown(event, ${index})">
                <input type="text"
                    value="${esc(item.description || '')}"
                    maxlength="300"
                    placeholder="${esc(t('knowledge.todos_item_desc_placeholder'))}"
                    oninput="updateTodoDraftItem(${index}, 'description', this.value)">
            </div>
            <div class="kc-todo-editor-actions">
                <button type="button" class="btn btn-sm btn-secondary" onclick="moveTodoDraftItem(${index}, -1)" title="${esc(t('knowledge.todos_item_move_up'))}" ${index === 0 ? 'disabled' : ''}>↑</button>
                <button type="button" class="btn btn-sm btn-secondary" onclick="moveTodoDraftItem(${index}, 1)" title="${esc(t('knowledge.todos_item_move_down'))}" ${index === todoDraftItems.length - 1 ? 'disabled' : ''}>↓</button>
                <button type="button" class="btn btn-sm btn-danger" onclick="removeTodoDraftItem(${index})" title="${esc(t('knowledge.todos_item_remove'))}">✕</button>
            </div>
        </div>
    `).join('');

    if (todoDraftFocusIndex >= 0) {
        const focusTarget = document.getElementById('todo-item-title-' + todoDraftFocusIndex);
        const focusIndex = todoDraftFocusIndex;
        todoDraftFocusIndex = -1;
        if (focusTarget) {
            setTimeout(() => {
                focusTarget.focus();
                focusTarget.setSelectionRange(focusTarget.value.length, focusTarget.value.length);
            }, 0);
        } else {
            todoDraftFocusIndex = focusIndex;
        }
    }
}

function handleTodoDraftTitleKeydown(event, index) {
    if (event.key !== 'Enter' || event.shiftKey) return;
    event.preventDefault();
    if (index === todoDraftItems.length - 1) {
        addTodoDraftItem();
        return;
    }
    const next = document.getElementById('todo-item-title-' + (index + 1));
    if (next) next.focus();
}

async function saveTodo() {
    const saveButton = document.querySelector('#todo-modal .modal-actions .btn-primary');
    const id = document.getElementById('todo-id').value;
    const dueVal = document.getElementById('todo-due-date').value;
    const remindDailyInput = document.getElementById('todo-remind-daily');
    const data = {
        title: document.getElementById('todo-title').value.trim(),
        description: document.getElementById('todo-description').value.trim(),
        priority: document.getElementById('todo-priority').value,
        due_date: fromLocalDateInput(dueVal),
        remind_daily: !!(remindDailyInput && remindDailyInput.checked),
        items: collectTodoDraftItems(),
    };

    if (!data.title) {
        showToast(t('knowledge.todos_title_required'), 'error');
        return;
    }
    if (data.priority && !['low', 'medium', 'high'].includes(data.priority)) {
        showToast(t('knowledge.todos_invalid_priority') || 'Invalid priority value', 'error');
        return;
    }

    const existing = id ? allTodos.find(entry => entry.id === id) : null;
    data.status = existing ? existing.status : 'open';
    const controller = typeof AbortController !== 'undefined' ? new AbortController() : null;
    const timeout = controller ? setTimeout(() => controller.abort(), 15000) : null;

    try {
        if (saveButton) saveButton.disabled = true;
        const response = await fetch(id ? '/api/todos/' + encodeURIComponent(id) : '/api/todos', {
            method: id ? 'PUT' : 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
            signal: controller ? controller.signal : undefined,
        });
        if (!response.ok) throw new Error(await response.text());

        closeModal('todo-modal');
        showToast(t('common.success'), 'success');
        loadTodos();
    } catch (error) {
        console.error('Save todo failed:', error);
        showToast(t('common.error') + ': ' + error.message, 'error');
    } finally {
        if (timeout) clearTimeout(timeout);
        if (saveButton) saveButton.disabled = false;
    }
}

async function toggleTodoStatus(id, checked) {
    const todo = allTodos.find(entry => entry.id === id);
    if (!todo) return;

    if (checked) {
        try {
            const response = await fetch('/api/todos/' + encodeURIComponent(id) + '/complete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ complete_items_too: true }),
            });
            if (!response.ok) throw new Error(await response.text());
            showToast(t('common.success'), 'success');
            loadTodos();
        } catch (error) {
            console.error('Complete todo failed:', error);
            showToast(t('common.error') + ': ' + error.message, 'error');
        }
        return;
    }

    const payload = { status: 'open' };
    if (Array.isArray(todo.items) && todo.items.length) {
        payload.items = todo.items.map(item => ({
            id: item.id,
            title: item.title,
            description: item.description || '',
            is_done: false,
            position: item.position,
        }));
    }
    await patchTodo(id, payload);
}

async function setTodoInProgress(id) {
    await patchTodo(id, { status: 'in_progress' });
}

async function toggleTodoItem(todoID, itemID, checked) {
    try {
        const response = await fetch('/api/todos/' + encodeURIComponent(todoID) + '/items/' + encodeURIComponent(itemID), {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ is_done: checked }),
        });
        if (!response.ok) throw new Error(await response.text());
        showToast(t('common.success'), 'success');
        loadTodos();
    } catch (error) {
        console.error('Update todo item failed:', error);
        showToast(t('common.error') + ': ' + error.message, 'error');
    }
}

async function patchTodo(id, payload) {
    const controller = typeof AbortController !== 'undefined' ? new AbortController() : null;
    const timeout = controller ? setTimeout(() => controller.abort(), 15000) : null;
    try {
        const response = await fetch('/api/todos/' + encodeURIComponent(id), {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
            signal: controller ? controller.signal : undefined,
        });
        if (!response.ok) throw new Error(await response.text());
        showToast(t('common.success'), 'success');
        loadTodos();
    } catch (error) {
        console.error('Update todo failed:', error);
        showToast(t('common.error') + ': ' + error.message, 'error');
    } finally {
        if (timeout) clearTimeout(timeout);
    }
}

function askDeleteTodo(id, title) {
    document.getElementById('delete-target-id').value = id;
    document.getElementById('delete-target-type').value = 'todo';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.todos_delete_confirm').replace('{name}', title);
    document.getElementById('delete-modal').classList.add('active');
}

function collectTodoDraftItems() {
    return todoDraftItems
        .map((item, index) => ({
            id: item.id || undefined,
            title: (item.title || '').trim(),
            description: (item.description || '').trim(),
            is_done: !!item.is_done,
            position: index,
        }))
        .filter(item => item.title);
}

function normalizeTodoDraftItems(items) {
    return (items || []).map(item => ({
        id: item.id || '',
        title: item.title || '',
        description: item.description || '',
        position: item.position || 0,
        is_done: !!item.is_done,
    }));
}

function todoPriorityClass(priority) {
    switch (priority) {
        case 'high': return 'kc-priority-high';
        case 'medium': return 'kc-priority-medium';
        default: return 'kc-priority-low';
    }
}

function todoPriorityLabel(priority) {
    switch (priority) {
        case 'high': return t('knowledge.todos_priority_high');
        case 'medium': return t('knowledge.todos_priority_medium');
        default: return t('knowledge.todos_priority_low');
    }
}

function todoStatusLabel(status) {
    switch (status) {
        case 'in_progress': return t('knowledge.todos_status_in_progress');
        case 'done': return t('knowledge.todos_status_done');
        default: return t('knowledge.todos_status_open');
    }
}

function formatTodoDueDate(iso) {
    if (!iso) return '';
    try {
        const date = new Date(iso);
        return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    } catch {
        return iso;
    }
}

function formatTodoDate(iso) {
    if (!iso) return '';
    try {
        const date = new Date(iso);
        return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
    } catch {
        return iso;
    }
}

function toLocalDateInput(iso) {
    if (!iso) return '';
    try {
        const date = new Date(iso);
        const pad = value => String(value).padStart(2, '0');
        return date.getFullYear() + '-' + pad(date.getMonth() + 1) + '-' + pad(date.getDate());
    } catch {
        return '';
    }
}

function fromLocalDateInput(value) {
    if (!value) return '';
    try {
        const [year, month, day] = value.split('-').map(Number);
        if (!year || !month || !day) return '';
        return new Date(year, month - 1, day).toISOString();
    } catch {
        return '';
    }
}

function quoteJS(value) {
    return JSON.stringify(String(value ?? ''));
}
