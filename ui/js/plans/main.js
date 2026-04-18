const sessionInput = document.getElementById('session-id');
const statusFilter = document.getElementById('status-filter');
const includeArchivedInput = document.getElementById('include-archived');
const refreshBtn = document.getElementById('refresh-btn');
const planListEl = document.getElementById('plan-list');
const planDetailEl = document.getElementById('plan-detail');
const blockerReasonEl = document.getElementById('blocker-reason');
const blockerCancelBtn = document.getElementById('blocker-cancel');
const blockerConfirmBtn = document.getElementById('blocker-confirm');
const splitItemsEl = document.getElementById('split-items');
const splitCancelBtn = document.getElementById('split-cancel');
const splitConfirmBtn = document.getElementById('split-confirm');

let selectedPlanId = null;
let selectedPlan = null;
let listedPlans = [];
let evtSource = null;
let pendingBlockTaskId = null;
let pendingSplitTaskId = null;

function plansApi(url, options) {
    return fetch(url, Object.assign({ credentials: 'same-origin' }, options || {})).then(async (res) => {
        if (res.status === 401) {
            window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
            return null;
        }
        const data = await res.json().catch(() => null);
        if (!res.ok) {
            throw new Error((data && data.message) || ('HTTP ' + res.status));
        }
        return data;
    });
}

function escapeHtml(str) {
    return String(str == null ? '' : str)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}

function badge(status, label) {
    const resolved = status || 'draft';
    return `<span class="badge status-${escapeHtml(resolved)}">${escapeHtml(label || resolved)}</span>`;
}

function currentSessionId() {
    return (sessionInput.value || 'default').trim() || 'default';
}

function includeArchived() {
    return !!(includeArchivedInput && includeArchivedInput.checked);
}

function renderPlanList(plans) {
    listedPlans = Array.isArray(plans) ? plans : [];
    if (!listedPlans.length) {
        planListEl.innerHTML = `<div class="plan-row muted">${escapeHtml(t('plans.no_plans'))}</div>`;
        return;
    }
    planListEl.innerHTML = listedPlans.map((plan) => `
        <div class="plan-row ${plan.id === selectedPlanId ? 'active' : ''}" data-plan-id="${escapeHtml(plan.id)}">
            <div class="plan-row-title">
                <span>${escapeHtml(plan.title || 'Plan')}</span>
                <div class="plan-badges">
                    ${badge(plan.status)}
                    ${plan.archived ? badge('cancelled', t('plans.archived_badge')) : ''}
                </div>
            </div>
            <div class="plan-row-meta">${escapeHtml(plan.current_task || plan.recommendation || plan.description || '')}</div>
            <div class="plan-row-meta">${escapeHtml((plan.task_counts?.completed || 0) + '/' + (plan.task_counts?.total || 0))} · ${escapeHtml(plan.updated_at || '')}</div>
        </div>
    `).join('');

    planListEl.querySelectorAll('.plan-row[data-plan-id]').forEach((row) => {
        row.addEventListener('click', () => {
            selectedPlanId = row.dataset.planId;
            loadPlanDetail(selectedPlanId).catch(console.error);
        });
    });
}

function renderArtifacts(task) {
    if (!task || !Array.isArray(task.artifacts) || !task.artifacts.length) return '';
    return `<div class="task-artifacts">${task.artifacts.map((artifact) => `
        <div>${escapeHtml((artifact.label || artifact.type || 'artifact') + ': ' + (artifact.value || ''))}</div>
    `).join('')}</div>`;
}

function siblingTasks(plan, task) {
    const tasks = Array.isArray(plan?.tasks) ? plan.tasks : [];
    return tasks.filter((candidate) => (candidate.parent_task_id || '') === (task.parent_task_id || ''));
}

function taskActionButtons(plan, task) {
    if (!plan || !task || plan.archived) return '';
    const buttons = [];
    const siblings = siblingTasks(plan, task);
    const siblingIndex = siblings.findIndex((candidate) => candidate.id === task.id);

    if (siblingIndex > 0) {
        buttons.push(`<button class="btn" data-task-action="move_up" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.move_up'))}</button>`);
    }
    if (siblingIndex >= 0 && siblingIndex < siblings.length - 1) {
        buttons.push(`<button class="btn" data-task-action="move_down" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.move_down'))}</button>`);
    }
    if (task.status !== 'completed' && task.status !== 'failed' && task.status !== 'skipped') {
        buttons.push(`<button class="btn" data-task-action="split" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.split'))}</button>`);
    }
    if (plan.status === 'active') {
        if (task.status === 'pending') {
            buttons.push(`<button class="btn" data-task-action="in_progress" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.start_task'))}</button>`);
        }
        if (task.status === 'in_progress') {
            buttons.push(`<button class="btn btn-primary" data-task-action="advance" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.advance'))}</button>`);
            buttons.push(`<button class="btn" data-task-action="block" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.block'))}</button>`);
        }
        if (task.status === 'blocked') {
            buttons.push(`<button class="btn" data-task-action="unblock" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.unblock'))}</button>`);
        }
    }
    if (task.status !== 'completed' && task.status !== 'skipped') {
        buttons.push(`<button class="btn" data-task-action="completed" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.mark_done'))}</button>`);
    }
    if (task.status !== 'failed' && task.status !== 'completed') {
        buttons.push(`<button class="btn" data-task-action="failed" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.mark_failed'))}</button>`);
    }
    if (task.status !== 'skipped' && task.status !== 'completed') {
        buttons.push(`<button class="btn" data-task-action="skipped" data-task-id="${escapeHtml(task.id)}">${escapeHtml(t('plans.mark_skipped'))}</button>`);
    }
    return buttons.join('');
}

function renderTaskTree(plan, tasks, parentTaskId, depth) {
    return tasks
        .filter((task) => (task.parent_task_id || '') === (parentTaskId || ''))
        .map((task) => {
            const children = renderTaskTree(plan, tasks, task.id, depth + 1);
            return `
                <div class="task-tree">
                    <div class="task-item ${depth > 0 ? 'subtask' : ''}">
                        <div class="task-head">
                            <div>
                                <div class="task-title">${escapeHtml(task.title || '')}</div>
                                <div class="task-meta">${escapeHtml(task.description || '')}</div>
                                ${task.acceptance_criteria ? `<div class="task-meta">${escapeHtml(t('plans.acceptance'))}: ${escapeHtml(task.acceptance_criteria)}</div>` : ''}
                                ${task.owner ? `<div class="task-meta">${escapeHtml(t('plans.owner'))}: ${escapeHtml(task.owner)}</div>` : ''}
                                ${task.blocker_reason ? `<div class="task-meta">${escapeHtml(t('plans.blocked_reason'))}: ${escapeHtml(task.blocker_reason)}</div>` : ''}
                            </div>
                            <div class="plan-badges">
                                ${badge(task.status)}
                                ${depth > 0 ? badge('draft', t('plans.subtask_badge')) : ''}
                            </div>
                        </div>
                        ${renderArtifacts(task)}
                        <div class="task-actions">${taskActionButtons(plan, task)}</div>
                    </div>
                    ${children}
                </div>
            `;
        })
        .join('');
}

function renderPlanDetail(plan) {
    selectedPlan = plan || null;
    if (!plan) {
        planDetailEl.className = 'plan-detail empty-state';
        planDetailEl.textContent = t('plans.empty_detail');
        return;
    }

    const tasks = Array.isArray(plan.tasks) ? plan.tasks : [];
    const events = Array.isArray(plan.events) ? plan.events : [];
    const actions = [];
    if (!plan.archived) {
        actions.push(`<button class="btn btn-primary" data-plan-action="active">${escapeHtml(t('plans.activate'))}</button>`);
        actions.push(`<button class="btn" data-plan-action="paused">${escapeHtml(t('plans.pause'))}</button>`);
        actions.push(`<button class="btn" data-plan-action="completed">${escapeHtml(t('plans.complete'))}</button>`);
        actions.push(`<button class="btn" data-plan-action="cancelled">${escapeHtml(t('plans.cancel'))}</button>`);
        actions.push(`<button class="btn" data-plan-action="advance">${escapeHtml(t('plans.advance'))}</button>`);
        if (plan.status === 'completed' || plan.status === 'cancelled') {
            actions.push(`<button class="btn" data-plan-action="archive">${escapeHtml(t('plans.archive'))}</button>`);
        }
    }

    planDetailEl.className = 'plan-detail';
    planDetailEl.innerHTML = `
        <div class="detail-toolbar">
            <div class="plan-head">
                <div>
                    <h2>${escapeHtml(plan.title || 'Plan')}</h2>
                    <p class="status-line">${escapeHtml(plan.description || '')}</p>
                    <p class="status-line"><strong>${escapeHtml(t('plans.progress'))}:</strong> ${escapeHtml(String(plan.progress_pct || 0))}% (${escapeHtml(String(plan.task_counts?.completed || 0))}/${escapeHtml(String(plan.task_counts?.total || 0))})</p>
                    ${plan.current_task ? `<p class="status-line"><strong>${escapeHtml(t('plans.current_task'))}:</strong> ${escapeHtml(plan.current_task)}</p>` : ''}
                    ${plan.blocked_reason ? `<p class="status-line"><strong>${escapeHtml(t('plans.blocked_reason'))}:</strong> ${escapeHtml(plan.blocked_reason)}</p>` : ''}
                    ${plan.recommendation ? `<p class="status-line"><strong>${escapeHtml(t('plans.recommendation'))}:</strong> ${escapeHtml(plan.recommendation)}</p>` : ''}
                </div>
                <div class="plan-badges">
                    ${badge(plan.status)}
                    ${badge('draft', `${t('plans.priority')}: ${String(plan.priority || 2)}`)}
                    ${plan.archived ? badge('cancelled', t('plans.archived_badge')) : ''}
                </div>
            </div>
            <div class="plan-actions">${actions.join('')}</div>
        </div>
        <div class="plan-grid">
            <div class="card">
                <h3>${escapeHtml(t('plans.tasks_heading'))}</h3>
                <div class="task-list">
                    ${renderTaskTree(plan, tasks, '', 0) || `<div class="muted">${escapeHtml(t('plans.no_tasks'))}</div>`}
                </div>
            </div>
            <div class="card">
                <h3>${escapeHtml(t('plans.events_heading'))}</h3>
                <div class="event-list">
                    ${events.length ? events.map((evt) => `
                        <div class="event-item">
                            <div class="event-type">${escapeHtml(evt.event_type || '')}</div>
                            <div>${escapeHtml(evt.message || '')}</div>
                            <div class="muted">${escapeHtml(evt.created_at || '')}</div>
                        </div>
                    `).join('') : `<div class="muted">${escapeHtml(t('plans.no_events'))}</div>`}
                </div>
            </div>
        </div>
    `;

    planDetailEl.querySelectorAll('[data-plan-action]').forEach((btn) => {
        btn.addEventListener('click', async () => {
            const action = btn.dataset.planAction;
            try {
                if (action === 'advance') {
                    await plansApi(`/api/plans/${encodeURIComponent(plan.id)}/advance`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ result: '' })
                    });
                } else if (action === 'archive') {
                    await plansApi(`/api/plans/${encodeURIComponent(plan.id)}/archive`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({})
                    });
                } else {
                    await plansApi(`/api/plans/${encodeURIComponent(plan.id)}/status`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ status: action, note: '' })
                    });
                }
                await loadPlans();
            } catch (err) {
                showToast(err.message || String(err), 'error');
            }
        });
    });

    planDetailEl.querySelectorAll('[data-task-action]').forEach((btn) => {
        btn.addEventListener('click', async () => {
            const action = btn.dataset.taskAction;
            const taskId = btn.dataset.taskId;
            try {
                if (action === 'advance') {
                    await plansApi(`/api/plans/${encodeURIComponent(plan.id)}/advance`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ result: '' })
                    });
                } else if (action === 'block') {
                    openBlockerDialog(taskId);
                    return;
                } else if (action === 'unblock') {
                    await plansApi(`/api/plans/${encodeURIComponent(plan.id)}/tasks/${encodeURIComponent(taskId)}/unblock`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ note: '' })
                    });
                } else if (action === 'split') {
                    openSplitDialog(taskId);
                    return;
                } else if (action === 'move_up' || action === 'move_down') {
                    await moveTask(taskId, action === 'move_up' ? -1 : 1);
                } else {
                    await plansApi(`/api/plans/${encodeURIComponent(plan.id)}/tasks/${encodeURIComponent(taskId)}/status`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ status: action, result: '', error: '' })
                    });
                }
                await loadPlans();
            } catch (err) {
                showToast(err.message || String(err), 'error');
            }
        });
    });
}

function findSwapTask(plan, taskId, offset) {
    const tasks = Array.isArray(plan?.tasks) ? plan.tasks : [];
    const task = tasks.find((entry) => entry.id === taskId);
    if (!task) return null;
    const siblings = tasks.filter((entry) => (entry.parent_task_id || '') === (task.parent_task_id || ''));
    const index = siblings.findIndex((entry) => entry.id === taskId);
    const target = siblings[index + offset];
    return target || null;
}

async function moveTask(taskId, offset) {
    if (!selectedPlan) return;
    const target = findSwapTask(selectedPlan, taskId, offset);
    if (!target) return;
    const order = selectedPlan.tasks.map((task) => task.id);
    const fromIndex = order.indexOf(taskId);
    const targetIndex = order.indexOf(target.id);
    if (fromIndex < 0 || targetIndex < 0) return;
    order[fromIndex] = target.id;
    order[targetIndex] = taskId;
    await plansApi(`/api/plans/${encodeURIComponent(selectedPlan.id)}/reorder`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ task_ids: order })
    });
}

function openBlockerDialog(taskId) {
    pendingBlockTaskId = taskId;
    blockerReasonEl.value = '';
    openModal('blocker-modal');
    blockerReasonEl.focus();
}

function openSplitDialog(taskId) {
    pendingSplitTaskId = taskId;
    splitItemsEl.value = '';
    openModal('split-modal');
    splitItemsEl.focus();
}

async function confirmBlockerDialog() {
    if (!selectedPlan || !pendingBlockTaskId) return;
    const reason = (blockerReasonEl.value || '').trim();
    if (!reason) {
        showToast(t('plans.block_reason_required'), 'warning');
        return;
    }
    try {
        await plansApi(`/api/plans/${encodeURIComponent(selectedPlan.id)}/tasks/${encodeURIComponent(pendingBlockTaskId)}/block`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ reason })
        });
        closeModal('blocker-modal');
        pendingBlockTaskId = null;
        await loadPlans();
    } catch (err) {
        showToast(err.message || String(err), 'error');
    }
}

async function confirmSplitDialog() {
    if (!selectedPlan || !pendingSplitTaskId) return;
    const lines = (splitItemsEl.value || '')
        .split(/\r?\n/)
        .map((line) => line.trim())
        .filter(Boolean);
    if (lines.length < 2) {
        showToast(t('plans.split_require_two'), 'warning');
        return;
    }
    try {
        await plansApi(`/api/plans/${encodeURIComponent(selectedPlan.id)}/tasks/${encodeURIComponent(pendingSplitTaskId)}/split`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                items: lines.map((title) => ({ title }))
            })
        });
        closeModal('split-modal');
        pendingSplitTaskId = null;
        await loadPlans();
    } catch (err) {
        showToast(err.message || String(err), 'error');
    }
}

async function loadPlans() {
    const data = await plansApi(`/api/plans?session_id=${encodeURIComponent(currentSessionId())}&status=${encodeURIComponent(statusFilter.value || 'all')}&include_archived=${includeArchived() ? '1' : '0'}&limit=50`);
    if (!data) return;
    renderPlanList(data.plans || []);
    if (selectedPlanId) {
        const stillExists = (data.plans || []).some((plan) => plan.id === selectedPlanId);
        if (!stillExists) {
            selectedPlanId = data.plans && data.plans[0] ? data.plans[0].id : null;
        }
    } else if (data.plans && data.plans[0]) {
        selectedPlanId = data.plans[0].id;
    }
    if (selectedPlanId) {
        await loadPlanDetail(selectedPlanId);
    } else {
        renderPlanDetail(null);
    }
}

async function loadPlanDetail(id) {
    if (!id) {
        renderPlanDetail(null);
        return;
    }
    const data = await plansApi(`/api/plans/${encodeURIComponent(id)}`);
    if (!data) return;
    selectedPlanId = id;
    renderPlanDetail(data.plan || null);
    renderPlanList(listedPlans);
}

function initSSE() {
    if (evtSource) evtSource.close();
    evtSource = new EventSource('/events');
    evtSource.onmessage = async (e) => {
        try {
            const data = JSON.parse(e.data);
            if (data.event === 'plan_update') {
                await loadPlans();
            }
        } catch (_) { }
    };
}

function applyPlansI18n() {
    if (typeof window._auragoApplySharedI18n === 'function') {
        window._auragoApplySharedI18n();
    }
    const trigger = document.getElementById('radialTrigger');
    if (trigger) trigger.setAttribute('aria-label', t('common.nav_aria_label'));
    document.querySelectorAll('[data-i18n]').forEach((el) => {
        const key = el.getAttribute('data-i18n');
        if (key) el.textContent = t(key);
    });
}

document.addEventListener('DOMContentLoaded', async () => {
    ensureBrandIcons();
    initTheme();
    injectRadialMenu();
    applyPlansI18n();
    const themeToggle = document.getElementById('theme-toggle');
    if (themeToggle) themeToggle.addEventListener('click', toggleTheme);
    initModals();
    refreshBtn.addEventListener('click', () => loadPlans().catch(console.error));
    statusFilter.addEventListener('change', () => loadPlans().catch(console.error));
    sessionInput.addEventListener('change', () => loadPlans().catch(console.error));
    includeArchivedInput.addEventListener('change', () => loadPlans().catch(console.error));
    blockerCancelBtn.addEventListener('click', () => { pendingBlockTaskId = null; closeModal('blocker-modal'); });
    blockerConfirmBtn.addEventListener('click', () => confirmBlockerDialog());
    splitCancelBtn.addEventListener('click', () => { pendingSplitTaskId = null; closeModal('split-modal'); });
    splitConfirmBtn.addEventListener('click', () => confirmSplitDialog());
    initSSE();
    await loadPlans();
});
