/* AuraGo – Knowledge Center: To-Dos */
/* global t, esc, showToast, closeModal */

// ═══════════════════════════════════════════════════════════════
// STATE
// ═══════════════════════════════════════════════════════════════
let allTodos = [];
let todoSearchTimer = null;

// ═══════════════════════════════════════════════════════════════
// LOAD & RENDER
// ═══════════════════════════════════════════════════════════════

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
        const r = await fetch(url);
        if (!r.ok) throw new Error(r.status + ' ' + r.statusText);
        const resp = await r.json();
        allTodos = resp || [];
        renderTodos();
    } catch (e) {
        console.error('Failed to load todos:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function renderTodos() {
    const list = document.getElementById('todos-list');
    const empty = document.getElementById('todos-empty');

    if (!allTodos.length) {
        list.innerHTML = '';
        list.classList.add('is-hidden');
        empty.classList.remove('is-hidden');
        return;
    }
    list.classList.remove('is-hidden');
    empty.classList.add('is-hidden');

    list.innerHTML = allTodos.map(td => {
        const isDone = td.status === 'done';
        const priorityClass = todoPriorityClass(td.priority);
        const priorityLabel = todoPriorityLabel(td.priority);
        const statusLabel = todoStatusLabel(td.status);
        const due = td.due_date ? formatTodoDueDate(td.due_date) : '';
        const isOverdue = td.due_date && !isDone && new Date(td.due_date) < new Date();

        return `
        <div class="kc-todo-item ${isDone ? 'kc-todo-done' : ''} ${isOverdue ? 'kc-todo-overdue' : ''}">
            <div class="kc-todo-check">
                <input type="checkbox" ${isDone ? 'checked' : ''}
                    onchange="toggleTodoStatus('${esc(td.id)}', this.checked)"
                    title="${isDone ? t('knowledge.todos_reopen') : t('knowledge.todos_mark_done')}">
            </div>
            <div class="kc-todo-content">
                <div class="kc-todo-title-row">
                    <span class="kc-todo-title ${isDone ? 'kc-todo-title-done' : ''}">${esc(td.title)}</span>
                    <span class="kc-priority-badge ${priorityClass}">${priorityLabel}</span>
                    ${td.status === 'in_progress' ? `<span class="kc-status-pill kc-status-inprogress">${statusLabel}</span>` : ''}
                </div>
                ${td.description ? `<p class="kc-todo-desc">${esc(td.description)}</p>` : ''}
                <div class="kc-todo-meta">
                    ${due ? `<span class="kc-todo-meta-item ${isOverdue ? 'kc-todo-overdue-text' : ''}">📅 ${due}</span>` : ''}
                    <span class="kc-todo-meta-item kc-todo-created">🕐 ${formatTodoDate(td.created_at)}</span>
                </div>
            </div>
            <div class="kc-todo-actions">
                ${!isDone && td.status !== 'in_progress' ? `
                    <button class="btn btn-sm btn-secondary" onclick="setTodoInProgress('${esc(td.id)}')" title="${t('knowledge.todos_start')}">▶️</button>
                ` : ''}
                <button class="btn btn-sm btn-secondary" onclick="editTodo('${esc(td.id)}')" title="${t('common.btn_edit')}">✏️</button>
                <button class="btn btn-sm btn-danger" onclick="askDeleteTodo('${esc(td.id)}', '${esc(td.title)}')" title="${t('common.btn_delete')}">🗑️</button>
            </div>
        </div>`;
    }).join('');
}

// ═══════════════════════════════════════════════════════════════
// MODAL
// ═══════════════════════════════════════════════════════════════

function openTodoModal(todo) {
    const modal = document.getElementById('todo-modal');
    const title = document.getElementById('todo-modal-title');

    document.getElementById('todo-id').value = todo ? todo.id : '';
    document.getElementById('todo-title').value = todo ? todo.title : '';
    document.getElementById('todo-description').value = todo ? todo.description || '' : '';
    document.getElementById('todo-priority').value = todo ? todo.priority : 'medium';
    document.getElementById('todo-due-date').value = todo && todo.due_date ? toLocalDateInput(todo.due_date) : '';

    title.textContent = todo ? t('knowledge.todos_edit') : t('knowledge.todos_add');
    modal.classList.add('active');
}

function editTodo(id) {
    const td = allTodos.find(x => x.id === id);
    if (td) openTodoModal(td);
}

async function saveTodo() {
    const id = document.getElementById('todo-id').value;
    const dueVal = document.getElementById('todo-due-date').value;
    const data = {
        title: document.getElementById('todo-title').value.trim(),
        description: document.getElementById('todo-description').value.trim(),
        priority: document.getElementById('todo-priority').value,
        due_date: fromLocalDateInput(dueVal),
    };

    if (!data.title) {
        showToast(t('knowledge.todos_title_required'), 'error');
        return;
    }
    if (data.priority && !['low', 'medium', 'high'].includes(data.priority)) {
        showToast(t('knowledge.todos_invalid_priority') || 'Invalid priority value', 'error');
        return;
    }

    // Preserve existing status on edit, default "open" for new
    if (id) {
        const existing = allTodos.find(x => x.id === id);
        data.status = existing ? existing.status : 'open';
    } else {
        data.status = 'open';
    }

    try {
        let resp;
        if (id) {
            data.id = id;
            resp = await fetch('/api/todos/' + encodeURIComponent(id), {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        } else {
            resp = await fetch('/api/todos', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        }

        if (!resp.ok) {
            const err = await resp.text();
            throw new Error(err);
        }

        closeModal('todo-modal');
        showToast(t('common.success'), 'success');
        loadTodos();
    } catch (e) {
        console.error('Save todo failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

// ═══════════════════════════════════════════════════════════════
// STATUS ACTIONS
// ═══════════════════════════════════════════════════════════════

async function toggleTodoStatus(id, checked) {
    await updateTodoStatus(id, checked ? 'done' : 'open');
}

async function setTodoInProgress(id) {
    await updateTodoStatus(id, 'in_progress');
}

async function updateTodoStatus(id, status) {
    const td = allTodos.find(x => x.id === id);
    if (!td) return;

    const data = Object.assign({}, td, { status });

    try {
        const resp = await fetch('/api/todos/' + encodeURIComponent(id), {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        if (!resp.ok) throw new Error(await resp.text());
        showToast(t('common.success'), 'success');
        loadTodos();
    } catch (e) {
        console.error('Update todo status failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function askDeleteTodo(id, title) {
    document.getElementById('delete-target-id').value = id;
    document.getElementById('delete-target-type').value = 'todo';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.todos_delete_confirm').replace('{name}', title);
    document.getElementById('delete-modal').classList.add('active');
}

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

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
        const d = new Date(iso);
        return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    } catch { return iso; }
}

function formatTodoDate(iso) {
    if (!iso) return '';
    try {
        const d = new Date(iso);
        return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
    } catch { return iso; }
}

function toLocalDateInput(iso) {
    if (!iso) return '';
    try {
        const d = new Date(iso);
        const pad = n => String(n).padStart(2, '0');
        return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate());
    } catch { return ''; }
}

function fromLocalDateInput(val) {
    if (!val) return '';
    try {
        const [year, month, day] = val.split('-').map(Number);
        if (!year || !month || !day) return '';
        return new Date(year, month - 1, day).toISOString();
    } catch { return ''; }
}
