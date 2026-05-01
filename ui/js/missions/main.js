// AuraGo – missions_v2 page logic
// Extracted from missions_v2.html

/* ── i18n now in <head> ── */

// State
let missions = [];
let queue = { items: [], running: '' };
let webhooks = [];
let currentFilter = 'all';
let editingId = null;
let initialLoad = false; // Track if first load completed
let gridRendered = false; // Track if grid has been rendered at least once (for enter animation)
let viewMode = localStorage.getItem('missions-view-mode') || 'auto'; // 'grid' | 'list' | 'auto'
let expandedCards = new Set(); // Track expanded card IDs in grid view
let lastRenderedDataHash = ''; // Used to skip re-renders when nothing changed
let remoteTargets = [];

const remoteAllowedTriggers = new Set(['system_startup', 'mqtt_message', 'home_assistant_state']);

// Extract displayable text from mission last_output.
// Handles legacy entries where the raw OpenAI-format JSON was stored.
function extractLastOutput(raw) {
    if (!raw) return '';
    // Fast path: if it doesn't start with '{' it's already plain text
    if (!raw.trimStart().startsWith('{')) return raw;
    try {
        const obj = JSON.parse(raw);
        if (obj.choices && obj.choices.length > 0 && obj.choices[0].message) {
            return obj.choices[0].message.content || raw;
        }
    } catch (_) { /* not JSON */ }
    return raw;
}

// Icons
const icons = {
    play: '▶️',
    pause: '⏸️',
    stop: '⏹️',
    edit: '✏️',
    delete: '🗑️',
    lock: '🔒',
    unlock: '🔓',
    duplicate: '📄',
    manual: '👆',
    scheduled: '📅',
    triggered: '⚡',
    running: '🔄',
    queued: '⏳',
    waiting: '⏸️',
    success: '✅',
    error: '❌'
};

// Inline SVG icon set — heroicons style (20×20 viewBox, filled)
const svgIcons = {
    play: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M6.3 2.84A1.5 1.5 0 004 4.11v11.78a1.5 1.5 0 002.3 1.27l9.344-5.891a1.5 1.5 0 000-2.538L6.3 2.84z"/></svg>`,
    edit: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M5.433 13.917l1.262-3.155A4 4 0 017.58 9.42l6.92-6.918a2.121 2.121 0 013 3l-6.92 6.918c-.383.383-.84.685-1.343.886l-3.154 1.262a.5.5 0 01-.65-.65z"/><path d="M3.5 5.75c0-.69.56-1.25 1.25-1.25H10A.75.75 0 0010 3H4.75A2.75 2.75 0 002 5.75v9.5A2.75 2.75 0 004.75 18h9.5A2.75 2.75 0 0017 15.25V10a.75.75 0 00-1.5 0v5.25c0 .69-.56 1.25-1.25 1.25h-9.5c-.69 0-1.25-.56-1.25-1.25v-9.5z"/></svg>`,
    trash: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M8.75 1A2.75 2.75 0 006 3.75v.443c-.795.077-1.584.176-2.365.298a.75.75 0 10.23 1.482l.149-.022.841 10.518A2.75 2.75 0 007.596 19h4.807a2.75 2.75 0 002.742-2.53l.841-10.52.149.023a.75.75 0 00.23-1.482A41.03 41.03 0 0014 4.193V3.75A2.75 2.75 0 0011.25 1h-2.5zM10 4c.84 0 1.673.025 2.5.075V3.75c0-.69-.56-1.25-1.25-1.25h-2.5c-.69 0-1.25.56-1.25 1.25v.325C8.327 4.025 9.16 4 10 4zM8.58 7.72a.75.75 0 00-1.5.06l.3 7.5a.75.75 0 101.5-.06l-.3-7.5zm4.34.06a.75.75 0 10-1.5-.06l-.3 7.5a.75.75 0 101.5.06l.3-7.5z" clip-rule="evenodd"/></svg>`,
    copy: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M7 3.5A1.5 1.5 0 018.5 2h3.879a1.5 1.5 0 011.06.44l3.122 3.12A1.5 1.5 0 0117 6.622V12.5a1.5 1.5 0 01-1.5 1.5h-1v-3.379a3 3 0 00-.879-2.121L10.5 5.379A3 3 0 008.379 4.5H7v-1z"/><path d="M4.5 6A1.5 1.5 0 003 7.5v9A1.5 1.5 0 004.5 18h7a1.5 1.5 0 001.5-1.5v-5.879a1.5 1.5 0 00-.44-1.06L9.44 6.439A1.5 1.5 0 008.378 6H4.5z"/></svg>`,
    lockIcon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 1a4.5 4.5 0 00-4.5 4.5V9H5a2 2 0 00-2 2v6a2 2 0 002 2h10a2 2 0 002-2v-6a2 2 0 00-2-2h-.5V5.5A4.5 4.5 0 0010 1zm3 8V5.5a3 3 0 10-6 0V9h6z" clip-rule="evenodd"/></svg>`,
    cog: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M7.84 1.804A1 1 0 018.82 1h2.36a1 1 0 01.98.804l.331 1.652a6.993 6.993 0 011.929 1.115l1.598-.54a1 1 0 011.186.447l1.18 2.044a1 1 0 01-.205 1.251l-1.267 1.113a7.047 7.047 0 010 2.228l1.267 1.113a1 1 0 01.206 1.25l-1.18 2.045a1 1 0 01-1.187.447l-1.598-.54a6.993 6.993 0 01-1.929 1.115l-.33 1.652a1 1 0 01-.98.804H8.82a1 1 0 01-.98-.804l-.331-1.652a6.993 6.993 0 01-1.929-1.115l-1.598.54a1 1 0 01-1.186-.447l-1.18-2.044a1 1 0 01.205-1.251l1.267-1.114a7.05 7.05 0 010-2.227L1.821 7.773a1 1 0 01-.206-1.25l1.18-2.045a1 1 0 011.187-.447l1.598.54A6.993 6.993 0 017.51 3.456l.33-1.652zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd"/></svg>`,
    info: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z" clip-rule="evenodd"/></svg>`,
    refresh: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M15.312 11.424a5.5 5.5 0 01-9.201 2.466l-.312-.311h2.433a.75.75 0 000-1.5H3.989a.75.75 0 00-.75.75v4.242a.75.75 0 001.5 0v-2.43l.31.31a7 7 0 0011.712-3.138.75.75 0 00-1.449-.39zm1.23-3.723a.75.75 0 00.219-.53V2.929a.75.75 0 00-1.5 0V5.36l-.31-.31A7 7 0 003.239 8.188a.75.75 0 101.448.389A5.5 5.5 0 0113.89 6.11l.311.31h-2.432a.75.75 0 000 1.5h4.243a.75.75 0 00.53-.219z" clip-rule="evenodd"/></svg>`,
    chevron: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M8.22 5.22a.75.75 0 011.06 0l4.25 4.25a.75.75 0 010 1.06l-4.25 4.25a.75.75 0 01-1.06-1.06L11.94 10 8.22 6.28a.75.75 0 010-1.06z" clip-rule="evenodd"/></svg>`,
    chevronDown: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M5.22 8.22a.75.75 0 011.06 0L10 11.94l3.72-3.72a.75.75 0 111.06 1.06l-4.25 4.25a.75.75 0 01-1.06 0L5.22 9.28a.75.75 0 010-1.06z" clip-rule="evenodd"/></svg>`,
    more: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M6 10a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zM11.5 10a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zM17 10a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z"/></svg>`,
    clock: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm.75-13a.75.75 0 00-1.5 0v5c0 .2.08.39.22.53l3 3a.75.75 0 101.06-1.06L10.75 9.69V5z" clip-rule="evenodd"/></svg>`,
    calendar: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M5.75 2a.75.75 0 01.75.75V4h7V2.75a.75.75 0 011.5 0V4h.25A2.75 2.75 0 0118 6.75v8.5A2.75 2.75 0 0115.25 18H4.75A2.75 2.75 0 012 15.25v-8.5A2.75 2.75 0 014.75 4H5V2.75A.75.75 0 015.75 2zm-1 5.5c-.69 0-1.25.56-1.25 1.25v6.5c0 .69.56 1.25 1.25 1.25h10.5c.69 0 1.25-.56 1.25-1.25v-6.5c0-.69-.56-1.25-1.25-1.25H4.75z" clip-rule="evenodd"/></svg>`,
    bolt: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M11.983 1.907a.75.75 0 00-1.292-.657l-8.5 9.5A.75.75 0 002.75 12h6.572l-1.305 6.093a.75.75 0 001.292.657l8.5-9.5A.75.75 0 0017.25 8h-6.572l1.305-6.093z" clip-rule="evenodd"/></svg>`,
    hand: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M7 2a1 1 0 00-1 1v8.5L4.78 10.28a1 1 0 10-1.56 1.44l3 4A1 1 0 007 16h6a3 3 0 003-3V8a1 1 0 10-2 0V6a1 1 0 10-2 0V4a1 1 0 10-2 0V3a1 1 0 10-2 0v8a.5.5 0 11-1 0V3a1 1 0 00-1-1z"/></svg>`,
    checkCircle: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd"/></svg>`,
    xCircle: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z" clip-rule="evenodd"/></svg>`,
    fileText: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 6a1 1 0 011-1h6a1 1 0 110 2H7a1 1 0 01-1-1zm1 3a1 1 0 100 2h6a1 1 0 100-2H7z" clip-rule="evenodd"/></svg>`,
};

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    bindMissionUI();
    updateViewToggle();
    loadData();
    // Live updates pushed via SSE — no more polling.
    window.AuraSSE.on('mission_update', function (payload) {
        if (!initialLoad) return; // wait for initial REST load
        missions = (payload && payload.missions) || [];
        queue = (payload && payload.queue) || { items: [], running: '' };
        render();
    });
});

function bindMissionUI() {
    if (typeof window._auragoApplySharedI18n === 'function') {
        window._auragoApplySharedI18n();
    }

    document.getElementById('open-mission-modal-btn')?.addEventListener('click', () => openMissionModal());
    document.getElementById('mission-form')?.addEventListener('submit', (event) => event.preventDefault());
    document.getElementById('cron-preset')?.addEventListener('change', (event) => applyCronPreset(event.target.value));
    document.getElementById('mission-modal-cancel-btn')?.addEventListener('click', () => closeModal('modal'));
    document.getElementById('mission-save-btn')?.addEventListener('click', saveMission);
    document.getElementById('prep-modal-cancel-btn')?.addEventListener('click', () => closeModal('prep-modal'));

    document.addEventListener('click', (event) => {
        const filterBtn = event.target.closest('.tab[data-filter]');
        if (filterBtn) {
            filterMissions(filterBtn.dataset.filter);
            return;
        }

        const viewBtn = event.target.closest('[data-view-mode]');
        if (viewBtn) {
            setViewMode(viewBtn.dataset.viewMode);
            return;
        }

        const closeBtn = event.target.closest('[data-modal-close]');
        if (closeBtn) {
            closeModal(closeBtn.dataset.modalClose);
            return;
        }

        const execType = event.target.closest('.exec-type-option[data-exec-type]');
        if (execType) {
            selectExecType(execType.dataset.execType);
            return;
        }

        const runnerType = event.target.closest('.mission-runner-option[data-runner-type]');
        if (runnerType) {
            selectRunnerType(runnerType.dataset.runnerType);
            return;
        }

        const triggerBtn = event.target.closest('.trigger-type-btn[data-trigger]');
        if (triggerBtn) {
            if (triggerBtn.disabled) return;
            selectTriggerType(triggerBtn.dataset.trigger);
            return;
        }

        const missionAction = event.target.closest('[data-mission-action]');
        if (!missionAction) return;

        if (missionAction.classList.contains('card-compact') && event.target.closest('.card-actions')) {
            return;
        }

        const missionId = missionAction.dataset.missionId;
        switch (missionAction.dataset.missionAction) {
            case 'remove-from-queue':
                removeFromQueue(missionId);
                break;
            case 'open-edit':
                editMission(missionId);
                break;
            case 'run':
                runMission(missionId);
                break;
            case 'duplicate':
                duplicateMission(missionId);
                break;
            case 'delete':
                deleteMission(missionId);
                break;
            case 'toggle-expand':
                toggleCardExpand(missionId);
                break;
            case 'toggle-log':
                missionToggleLog(missionAction);
                break;
            case 'view-prepared':
                viewPreparedContext(missionId);
                break;
            case 'invalidate-prepared':
                invalidatePreparation(missionId);
                break;
            case 'prepare':
                prepareMission(missionId);
                break;
        }
    });
}

// Show loading skeleton
function showLoading() {
    const container = document.getElementById('missions-grid');
    container.innerHTML = `
                <div class="mission-card skeleton mission-card-skeleton"></div>
                <div class="mission-card skeleton mission-card-skeleton"></div>
                <div class="mission-card skeleton mission-card-skeleton"></div>
            `;
}

// Load data from API
async function loadData() {
    try {
        if (!initialLoad) {
            showLoading();
        }
        const response = await fetch('/api/missions/v2');
        const data = await response.json();
        missions = data.missions || [];
        queue = data.queue || { items: [], running: '' };
        initialLoad = true;
        try {
            render();
        } catch (renderErr) {
            console.error('Failed to render missions:', renderErr);
            document.getElementById('missions-grid').innerHTML = `
                <div class="empty-state">
                    <div class="icon">⚠️</div>
                    <p>${t('missions.empty_load_error')}</p>
                </div>`;
        }
    } catch (err) {
        console.error('Failed to load missions:', err);
        if (!initialLoad) {
            // Show error state on initial load failure
            document.getElementById('missions-grid').innerHTML = `
                        <div class="empty-state">
                            <div class="icon">⚠️</div>
                            <p>${t('missions.empty_load_error')}</p>
                        </div>
                    `;
        }
    }
}

// Render everything
function render() {
    // Build a hash of the current data to avoid unnecessary DOM re-renders.
    // renderStatusBar() and updateViewToggle() only patch textContent/classes (no flicker).
    // renderMissions() and renderQueue() replace innerHTML — skip them when nothing changed.
    const dataHash = JSON.stringify(missions) + '||' + JSON.stringify(queue) + '||' + currentFilter;
    const dataChanged = dataHash !== lastRenderedDataHash;
    lastRenderedDataHash = dataHash;

    renderStatusBar();
    if (dataChanged) {
        renderQueue();
        renderMissions();
    }
    updateViewToggle();
}

// ═══════════════════════════════════════════════════════════════
// VIEW MODE FUNCTIONS
// ═══════════════════════════════════════════════════════════════

function getEffectiveViewMode() {
    if (viewMode !== 'auto') return viewMode;
    // Auto: use list view when > 8 missions
    return missions.length > 8 ? 'list' : 'grid';
}

function setViewMode(mode) {
    viewMode = mode;
    localStorage.setItem('missions-view-mode', mode);
    renderMissions();
    updateViewToggle();
}

function updateViewToggle() {
    const effective = getEffectiveViewMode();
    document.querySelectorAll('#view-toggle button').forEach(btn => {
        const isActive = (viewMode === 'auto' && btn.dataset.mode === effective) ||
                        (viewMode !== 'auto' && btn.dataset.mode === viewMode);
        btn.classList.toggle('active', isActive);
    });
    // Update container class
    const container = document.getElementById('missions-grid');
    if (container) {
        container.classList.toggle('list-view', effective === 'list');
    }
}

function toggleCardExpand(id) {
    if (expandedCards.has(id)) {
        expandedCards.delete(id);
    } else {
        expandedCards.add(id);
    }
    renderMissions();
}

function missionToggleLog(toggleEl) {
    if (!toggleEl) return;
    toggleEl.classList.toggle('open');
    const log = toggleEl.nextElementSibling;
    if (log) log.classList.toggle('is-hidden');
}

// Status Bar
function renderStatusBar() {
    const total = missions.length;
    const running = missions.filter(m => m.status === 'running').length;
    const queued = queue.items.length;
    const triggered = missions.filter(m => m.execution_type === 'triggered').length;

    document.querySelector('#status-total .status-value').textContent = total;
    document.querySelector('#status-running .status-value').textContent = running;
    document.querySelector('#status-queued .status-value').textContent = queued;
    document.querySelector('#status-triggered .status-value').textContent = triggered;

    // Highlight running card
    const runningCard = document.getElementById('status-running');
    if (running > 0) {
        runningCard.classList.add('running');
    } else {
        runningCard.classList.remove('running');
    }
}

// Queue
function renderQueue() {
    const section = document.getElementById('queue-section');
    const container = document.getElementById('queue-items');

    if (queue.items.length === 0 && !queue.running) {
        section.classList.add('is-hidden');
        return;
    }

    section.classList.remove('is-hidden');
    let html = '';

    // Show running mission first
    if (queue.running) {
        const runningMission = missions.find(m => m.id === queue.running);
        if (runningMission) {
            html += `
                        <div class="queue-item queue-item-running priority-${runningMission.priority}">
                            <div class="queue-position">${icons.running}</div>
                            <div class="queue-info">
                                <div class="queue-name">${escapeHtml(runningMission.name)}</div>
                                <div class="queue-meta">${t('missions.queue_running_now')} | ${t('missions.queue_priority_prefix')} ${runningMission.priority}</div>
                            </div>
                            <span class="queue-trigger queue-trigger-active">${t('missions.queue_active_badge')}</span>
                        </div>
                    `;
        }
    }

    // Show queued items
    queue.items.forEach((item, index) => {
        const mission = missions.find(m => m.id === item.mission_id);
        if (!mission) return;

        const priorityClass = item.priority === 3 ? 'high' : item.priority === 2 ? 'medium' : 'low';
        const triggerLabel = item.trigger_type ? `[${item.trigger_type}]` : '';

        html += `
                    <div class="queue-item priority-${priorityClass}">
                        <div class="queue-position">${index + 1}</div>
                        <div class="queue-info">
                            <div class="queue-name">${escapeHtml(mission.name)}</div>
                            <div class="queue-meta">${t('missions.queue_waiting_since')} ${formatTime(item.enqueued_at)} | ${t('missions.queue_priority_prefix')} ${mission.priority}</div>
                        </div>
                        ${triggerLabel ? `<span class="queue-trigger">${triggerLabel}</span>` : ''}
                        <button class="icon-btn" data-mission-action="remove-from-queue" data-mission-id="${escapeAttr(mission.id)}" title="${t('missions.queue_remove_title')}">
                            ${icons.stop}
                        </button>
                    </div>
                `;
    });

    container.innerHTML = html;
}

// Missions Grid
function renderMissions() {
    const container = document.getElementById('missions-grid');
    const mode = getEffectiveViewMode();

    let filtered = missions;
    if (currentFilter !== 'all') {
        filtered = missions.filter(m => m.execution_type === currentFilter);
    }

    if (filtered.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="icon">🚀</div>
                <p>${currentFilter === 'all' ? t('missions.empty_create_first') : t('missions.empty_no_missions_of_type')}</p>
            </div>
        `;
        return;
    }

    const isFirstRender = !gridRendered;
    gridRendered = true;

    if (mode === 'list') {
        container.innerHTML = filtered.map(mission => renderMissionCompact(mission)).join('');
    } else {
        container.innerHTML = filtered.map(mission => renderMissionGrid(mission, isFirstRender)).join('');
    }
}

// Compact List View Card
function renderMissionCompact(mission) {
    const isRunning = mission.id === queue.running;
    const isQueued = queue.items.some(i => i.mission_id === mission.id);
    const typeIcon = icons[mission.execution_type] || icons.manual;
    const statusBadge = isRunning ? `<span class="badge badge-running">${t('missions.card_badge_running')}</span>` :
                       isQueued ? `<span class="badge badge-warning">${t('missions.card_badge_queued')}</span>` : '';
    const prepBadge = renderPrepBadge(mission);
    const runnerBadge = mission.runner_type === 'remote'
        ? `<span class="badge badge-remote">${escapeHtml(mission.remote_egg_name || mission.remote_nest_name || t('missions.card_remote_badge'))}</span>`
        : '';

    const mid = escapeAttr(mission.id);
    return `
        <div class="card-compact" data-mission-id="${mid}" data-mission-action="open-edit">
            <span class="card-icon" title="${escapeAttr(mission.execution_type)}">${typeIcon}</span>
            <span class="card-name">${escapeHtml(mission.name)}</span>
            ${mission.locked ? `<span class="card-icon" title="${t('missions.card_locked_title')}">${svgIcons.lockIcon}</span>` : ''}
            <div class="card-badges">${statusBadge}${prepBadge}${runnerBadge}</div>
            <div class="card-actions">
                <button class="mc-btn mc-btn-run" data-mission-action="run" data-mission-id="${mid}" title="${t('missions.card_btn_run_title')}" ${isRunning ? 'disabled' : ''}>${svgIcons.play}</button>
                ${renderPrepButton(mission, isRunning)}
                <button class="mc-btn" data-mission-action="duplicate" data-mission-id="${mid}" title="${t('missions.card_btn_duplicate_title')}">${svgIcons.copy}</button>
                <button class="mc-btn" data-mission-action="open-edit" data-mission-id="${mid}" title="${t('missions.card_btn_edit_title')}">${svgIcons.edit}</button>
                <button class="mc-btn mc-btn-danger" data-mission-action="delete" data-mission-id="${mid}" title="${t('missions.card_btn_delete_title')}" ${mission.locked ? 'disabled' : ''}>${svgIcons.trash}</button>
            </div>
        </div>
    `;
}

// Grid View Card (Expandable) — redesigned for clarity, hierarchy, touch-friendliness.
function renderMissionGrid(mission, isFirstRender) {
    const isRunning = mission.id === queue.running;
    const isQueued = queue.items.some(i => i.mission_id === mission.id);
    const isWaiting = !isRunning && !isQueued && mission.status === 'waiting';
    const statusKind = isRunning ? 'running'
        : isQueued ? 'queued'
        : isWaiting ? 'waiting'
        : (mission.execution_type || 'manual');
    const statusClass = isRunning ? 'running' : isQueued ? 'queued' : isWaiting ? 'waiting' : '';
    const isExpanded = expandedCards.has(mission.id);
    const hasError = !isRunning && mission.last_result === 'error';

    const mid = escapeAttr(mission.id);
    const statusChip = renderStatusChip(mission, isRunning, isQueued, isWaiting);
    const remoteBadge = mission.runner_type === 'remote'
        ? `<span class="badge badge-remote mc-remote-badge">${escapeHtml(mission.remote_egg_name || mission.remote_nest_name || t('missions.card_remote_badge'))}</span>`
        : '';
    const prepBadge = renderPrepBadge(mission);

    let triggerPill = '';
    if (mission.execution_type === 'triggered' && mission.trigger_config) {
        const text = renderTriggerText(mission);
        if (text) {
            triggerPill = `<div class="mc-trigger-pill" title="${escapeAttr(text.replace(/<[^>]+>/g, ''))}">${svgIcons.bolt}<span>${text}</span></div>`;
        }
    }

    const lastRun = mission.last_run ? formatTime(mission.last_run) : t('missions.card_last_run_never');
    const resultIcon = hasError ? svgIcons.xCircle : (mission.last_result === 'success' ? svgIcons.checkCircle : '');
    const resultClass = hasError ? 'mc-meta-item--error' : (mission.last_result === 'success' ? 'mc-meta-item--ok' : '');

    const lockedMark = mission.locked
        ? `<span class="mc-name-lock" title="${t('missions.card_locked_title')}">${svgIcons.lockIcon}</span>`
        : '';

    const fullPrompt = escapeHtml(mission.prompt || '');

    return `
        <article class="mission-card mc-card${statusClass ? ' ' + statusClass : ''}${isFirstRender ? ' entering' : ''}${isExpanded ? ' expanded' : ''}"
                 data-priority="${escapeAttr(mission.priority)}"
                 data-status="${statusKind}"
                 data-mission-id="${mid}">
            <header class="mc-header" data-mission-action="toggle-expand" data-mission-id="${mid}">
                <div class="mc-header-left">
                    ${statusChip}
                    ${remoteBadge}
                    ${prepBadge}
                </div>
                <button type="button"
                        class="mc-overflow-btn"
                        data-mission-action="toggle-expand"
                        data-mission-id="${mid}"
                        aria-label="${t('missions.card_more_actions')}"
                        title="${t('missions.card_more_actions')}"
                        aria-expanded="${isExpanded ? 'true' : 'false'}">
                    ${svgIcons.chevronDown}
                </button>
            </header>
            <div class="mc-body">
                <h3 class="mc-name" title="${escapeAttr(mission.name)}">
                    <span class="mc-name-text">${escapeHtml(mission.name)}</span>
                    ${lockedMark}
                </h3>
                ${triggerPill}
                <p class="mc-prompt">${escapeHtml(mission.prompt)}</p>
                <div class="mc-meta-row">
                    <div class="mc-meta">
                        <span class="mc-meta-item ${resultClass}">${resultIcon ? `<span class="mc-meta-icon">${resultIcon}</span>` : `<span class="mc-meta-icon">${svgIcons.clock}</span>`}<span>${lastRun}</span></span>
                        <span class="mc-meta-item">${t('missions.meta_run_count', { count: mission.run_count })}</span>
                    </div>
                    <button type="button"
                            class="mc-cta-run"
                            data-mission-action="run"
                            data-mission-id="${mid}"
                            title="${t('missions.card_btn_run_title')}"
                            ${isRunning ? 'disabled' : ''}>
                        ${svgIcons.play}
                        <span class="mc-cta-label">${t('missions.card_run_label')}</span>
                    </button>
                </div>
            </div>
            <div class="mc-expand">
                <div class="mc-expand-inner">
                    ${mission.prompt ? `<div class="mc-prompt-full">${fullPrompt}</div>` : ''}
                    ${triggerPill ? `<div class="mc-trigger-full">
                        <div class="trigger-label">${t('missions.card_trigger_label')}</div>
                        <div class="trigger-value">${renderTriggerText(mission)}</div>
                    </div>` : ''}
                    ${mission.last_output ? `
                        <div class="mc-log-block">
                            <div class="mc-log-head">${svgIcons.fileText}<span>${t('missions.card_view_log')}</span></div>
                            <pre class="mc-log-body">${escapeHtml(extractLastOutput(mission.last_output))}</pre>
                        </div>` : ''}
                    <div class="mc-actions-secondary">
                        ${renderPrepButton(mission, isRunning)}
                        <button type="button" class="mc-btn" data-mission-action="duplicate" data-mission-id="${mid}" title="${t('missions.card_btn_duplicate_title')}">${svgIcons.copy}</button>
                        <button type="button" class="mc-btn" data-mission-action="open-edit" data-mission-id="${mid}" title="${t('missions.card_btn_edit_title')}">${svgIcons.edit}</button>
                        <button type="button" class="mc-btn mc-btn-danger" data-mission-action="delete" data-mission-id="${mid}" title="${t('missions.card_btn_delete_title')}" ${mission.locked ? 'disabled' : ''}>${svgIcons.trash}</button>
                    </div>
                </div>
            </div>
        </article>
    `;
}

// Status chip combines status (running/queued/waiting) and execution type into one element.
function renderStatusChip(mission, isRunning, isQueued, isWaiting) {
    let kind, label, icon;
    if (isRunning) {
        kind = 'running';
        label = t('missions.card_badge_running');
        icon = svgIcons.play;
    } else if (isQueued) {
        kind = 'queued';
        label = t('missions.card_badge_queued');
        icon = svgIcons.clock;
    } else if (isWaiting) {
        kind = 'waiting';
        label = t('missions.card_chip_waiting');
        icon = svgIcons.clock;
    } else {
        kind = mission.execution_type || 'manual';
        if (kind === 'scheduled') {
            label = t('missions.filter_scheduled');
            icon = svgIcons.calendar;
        } else if (kind === 'triggered') {
            label = t('missions.filter_triggered');
            icon = svgIcons.bolt;
        } else {
            label = t('missions.filter_manual');
            icon = svgIcons.hand;
        }
    }
    const priorityDot = `<span class="mc-status-chip__priority" data-priority="${escapeAttr(mission.priority)}" aria-hidden="true"></span>`;
    return `<span class="mc-status-chip mc-status-chip--${kind}">${priorityDot}<span class="mc-status-chip__icon">${icon}</span><span class="mc-status-chip__label">${escapeHtml(label)}</span></span>`;
}

function renderTriggerText(mission) {
    const cfg = mission.trigger_config || {};
    let triggerText = '';

    switch (mission.trigger_type) {
        case 'mission_completed':
            const sourceName = cfg.source_mission_name || cfg.source_mission_id || t('missions.trigger_info_unknown_mission');
            const successText = cfg.require_success ? ' ' + t('missions.trigger_info_only_on_success') : '';
            triggerText = t('missions.trigger_info_when_completed', { name: escapeHtml(sourceName) }) + successText;
            break;
        case 'email_received':
            const filters = [];
            if (cfg.email_folder) filters.push(`${t('missions.trigger_info_folder_prefix')} ${cfg.email_folder}`);
            if (cfg.email_subject_contains) filters.push(`${t('missions.trigger_info_subject_prefix')} "${cfg.email_subject_contains}"`);
            if (cfg.email_from_contains) filters.push(`${t('missions.trigger_info_from_prefix')} "${cfg.email_from_contains}"`);
            triggerText = filters.length > 0 ? filters.join(' | ') : t('missions.trigger_info_any_email');
            break;
        case 'webhook':
            triggerText = `${t('missions.trigger_info_webhook_prefix')} ${cfg.webhook_slug || cfg.webhook_id || t('missions.trigger_info_webhook_unknown')}`;
            break;
        case 'egg_hatched':
            const eggLabel = cfg.egg_name || cfg.egg_id ? `${t('missions.trigger_info_egg_prefix')} ${cfg.egg_name || cfg.egg_id}` : t('missions.trigger_info_any_egg');
            const nestEggLabel = cfg.nest_name || cfg.nest_id ? `, ${t('missions.trigger_info_nest_prefix')} ${cfg.nest_name || cfg.nest_id}` : '';
            triggerText = `🥚 ${eggLabel}${nestEggLabel}`;
            break;
        case 'nest_cleared':
            triggerText = `🪺 ${cfg.nest_name || cfg.nest_id ? `${t('missions.trigger_info_nest_prefix')} ${cfg.nest_name || cfg.nest_id}` : t('missions.trigger_info_any_nest')}`;
            break;
        case 'mqtt_message':
            const mqttParts = [`${t('missions.trigger_info_mqtt_topic_prefix')} ${cfg.mqtt_topic || '#'}`];
            if (cfg.mqtt_payload_contains) mqttParts.push(`${t('missions.trigger_info_mqtt_payload_prefix')} "${cfg.mqtt_payload_contains}"`);
            if (cfg.mqtt_min_interval_seconds) mqttParts.push(`${t('missions.trigger_info_mqtt_min_interval_prefix')} ${cfg.mqtt_min_interval_seconds}s`);
            triggerText = `📡 ${mqttParts.join(' | ')}`;
            break;
        case 'system_startup':
            triggerText = `${t('missions.trigger_system_startup_badge')}`;
            break;
        case 'home_assistant_state': {
            const parts = [`${t('missions.trigger_info_ha_entity_prefix')} ${escapeHtml(cfg.ha_entity_id || t('missions.trigger_info_ha_any_entity'))}`];
            if (cfg.ha_state_equals) parts.push(`${t('missions.trigger_info_ha_state_prefix')} "${escapeHtml(cfg.ha_state_equals)}"`);
            triggerText = `🏠 ${parts.join(' | ')}`;
            break;
        }
        case 'device_connected': {
            const devName = cfg.device_name || cfg.device_id || t('missions.trigger_info_any_device');
            triggerText = `🔌 ${t('missions.trigger_info_device_connected_prefix')} ${escapeHtml(devName)}`;
            break;
        }
        case 'device_disconnected': {
            const devName2 = cfg.device_name || cfg.device_id || t('missions.trigger_info_any_device');
            triggerText = `⚡ ${t('missions.trigger_info_device_disconnected_prefix')} ${escapeHtml(devName2)}`;
            break;
        }
        case 'fritzbox_call': {
            const typeLabel = cfg.call_type ? cfg.call_type : t('missions.trigger_info_fritzbox_any');
            triggerText = `📞 ${t('missions.trigger_info_fritzbox_prefix')} ${typeLabel}`;
            break;
        }
        case 'budget_warning':
            triggerText = `💰 ${t('missions.trigger_budget_warning_badge')}`;
            break;
        case 'budget_exceeded':
            triggerText = `🚫 ${t('missions.trigger_budget_exceeded_badge')}`;
            break;
        case 'planner_appointment_due': {
            const parts = [];
            if (cfg.planner_appointment_id) parts.push(`${t('missions.trigger_info_planner_appointment_id_prefix')} ${escapeHtml(cfg.planner_appointment_id)}`);
            if (cfg.planner_title_contains) parts.push(`${t('missions.trigger_info_planner_title_prefix')} "${escapeHtml(cfg.planner_title_contains)}"`);
            triggerText = `📅 ${parts.length > 0 ? parts.join(' | ') : t('missions.trigger_info_planner_any_appointment')}`;
            break;
        }
        case 'planner_todo_overdue': {
            const parts = [];
            if (cfg.planner_todo_id) parts.push(`${t('missions.trigger_info_planner_todo_id_prefix')} ${escapeHtml(cfg.planner_todo_id)}`);
            if (cfg.planner_title_contains) parts.push(`${t('missions.trigger_info_planner_title_prefix')} "${escapeHtml(cfg.planner_title_contains)}"`);
            triggerText = `📝 ${parts.length > 0 ? parts.join(' | ') : t('missions.trigger_info_planner_any_todo')}`;
            break;
        }
        case 'planner_operational_issue': {
            const parts = [];
            if (cfg.planner_issue_source) parts.push(`${t('missions.trigger_info_planner_issue_source_prefix')} ${escapeHtml(cfg.planner_issue_source)}`);
            if (cfg.planner_issue_severity) parts.push(`${t('missions.trigger_info_planner_issue_severity_prefix')} ${escapeHtml(cfg.planner_issue_severity)}`);
            if (cfg.planner_title_contains) parts.push(`${t('missions.trigger_info_planner_title_prefix')} "${escapeHtml(cfg.planner_title_contains)}"`);
            triggerText = `🧯 ${parts.length > 0 ? parts.join(' | ') : t('missions.trigger_info_planner_any_issue')}`;
            break;
        }
    }
    if (cfg.min_interval_seconds) {
        const intervalText = `${t('missions.trigger_info_min_interval_prefix')} ${cfg.min_interval_seconds}s`;
        triggerText = triggerText ? `${triggerText} | ${intervalText}` : intervalText;
    }
    return triggerText;
}

function renderTriggerInfo(mission) {
    const triggerText = renderTriggerText(mission);
    return `
                <div class="mission-trigger">
                    <div class="trigger-label">${t('missions.card_trigger_label')}</div>
                    <div class="trigger-value">${triggerText}</div>
                </div>
            `;
}

// Filter tabs
function filterMissions(type) {
    currentFilter = type;
    document.querySelectorAll('.tab').forEach(tab => {
        tab.classList.remove('active');
    });
    const tabId = 'tab-' + type;
    const activeTab = document.getElementById(tabId);
    if (activeTab) activeTab.classList.add('active');
    renderMissions();
}

// Modal
function openMissionModal(missionId = null) {
    editingId = missionId;
    document.getElementById('modal-title').textContent = missionId ? t('missions.modal_title_edit') : t('missions.modal_title_new');

    if (missionId) {
        const mission = missions.find(m => m.id === missionId);
        if (mission) {
            document.getElementById('mission-id').value = mission.id;
            document.getElementById('mission-name').value = mission.name;
            document.getElementById('mission-prompt').value = mission.prompt;
            document.getElementById('mission-priority').value = mission.priority;
            document.getElementById('mission-locked').checked = mission.locked;
            document.getElementById('mission-auto-prepare').checked = mission.auto_prepare || false;

            selectRunnerType(mission.runner_type || 'local');
            selectExecType(mission.execution_type);

            if (mission.execution_type === 'scheduled') {
                document.getElementById('cron-schedule').value = mission.schedule || '';
                syncCronPreset(mission.schedule || '');
            } else if (mission.execution_type === 'triggered') {
                selectTriggerType(mission.trigger_type);
                fillTriggerConfig(mission.trigger_config, mission.trigger_type);
            }
        }
    } else {
        document.getElementById('mission-form').reset();
        document.getElementById('mission-id').value = '';
        syncCronPreset('');
        selectRunnerType('local');
        selectExecType('manual');
    }

    // Load mission selector for triggers
    loadMissionSelector();
    // Load webhooks
    loadWebhooks();
    // Load invasion eggs/nests for triggers
    loadInvasionData();
    // Load connected eggs for remote mission targets
    loadRemoteTargets(missionId ? missions.find(m => m.id === missionId) : null);
    // Load cheatsheet picker
    loadCheatsheetPicker(missionId ? (missions.find(m => m.id === missionId)?.cheatsheet_ids || []) : []);

    openModal('modal');
}

// Execution Type Selection
function selectExecType(type) {
    document.querySelectorAll('.exec-type-option').forEach(opt => {
        if (opt.querySelector('input').value === type) {
            opt.querySelector('input').checked = true;
        }
    });

    document.getElementById('config-scheduled').classList.toggle('is-hidden', type !== 'scheduled');
    document.getElementById('config-triggered').classList.toggle('is-hidden', type !== 'triggered');
    updateRemoteTriggerAvailability();
}

function selectRunnerType(type) {
    const normalized = type === 'remote' ? 'remote' : 'local';
    document.querySelectorAll('.mission-runner-option').forEach(opt => {
        const input = opt.querySelector('input');
        input.checked = input.value === normalized;
        opt.classList.toggle('active', input.checked);
    });
    document.getElementById('remote-target-group')?.classList.toggle('is-hidden', normalized !== 'remote');
    updateRemoteTriggerAvailability();
}

function currentRunnerType() {
    return document.querySelector('input[name="runner-type"]:checked')?.value || 'local';
}

function updateRemoteTriggerAvailability() {
    const isRemote = currentRunnerType() === 'remote';
    document.querySelectorAll('.trigger-type-btn').forEach(btn => {
        const disabled = isRemote && !remoteAllowedTriggers.has(btn.dataset.trigger);
        btn.disabled = disabled;
        btn.classList.toggle('is-disabled', disabled);
        btn.title = disabled ? t('missions.trigger_remote_disabled_title') : '';
    });

    const active = document.querySelector('.trigger-type-btn.active');
    if (active && active.disabled) {
        active.classList.remove('active');
        document.querySelectorAll('.trigger-fields').forEach(field => field.classList.remove('active'));
    }
}

// Cron Preset Selection
function applyCronPreset(value) {
    if (value) {
        document.getElementById('cron-schedule').value = value;
    }
}

function syncCronPreset(schedule) {
    const sel = document.getElementById('cron-preset');
    if (!sel) return;
    const match = Array.from(sel.options).find(o => o.value === schedule);
    sel.value = match ? schedule : '';
}

// Trigger Type Selection
function selectTriggerType(type) {
    document.querySelectorAll('.trigger-type-btn').forEach(btn => {
        btn.classList.remove('active');
        if (btn.dataset.trigger === type) {
            btn.classList.add('active');
        }
    });

    document.querySelectorAll('.trigger-fields').forEach(field => {
        field.classList.remove('active');
    });

    const fieldId = 'trigger-' + type;
    const field = document.getElementById(fieldId);
    if (field) {
        field.classList.add('active');
    }
}

// Load mission selector
function loadMissionSelector() {
    const container = document.getElementById('mission-selector');
    const manualMissions = missions.filter(m => m.execution_type === 'manual' || m.execution_type === 'scheduled');

    if (manualMissions.length === 0) {
        container.innerHTML = '<div class="mission-trigger-empty">' + t('missions.trigger_no_suitable_missions') + '</div>';
        return;
    }

    container.innerHTML = manualMissions.map(m => `
                <label class="mission-option">
                    <input type="radio" name="source-mission" value="${m.id}" data-name="${escapeHtml(m.name)}">
                    <div class="mission-option-info">
                        <div class="mission-option-name">${escapeHtml(m.name)}</div>
                        <div class="mission-option-meta">${m.execution_type} • ${m.priority} • ${t('missions.meta_run_count', { count: m.run_count })}</div>
                    </div>
                </label>
            `).join('');
}

// Load webhooks
async function loadWebhooks() {
    try {
        const response = await fetch('/api/webhooks');
        webhooks = await response.json();

        const select = document.getElementById('webhook-select');
        select.innerHTML = webhooks.length === 0
            ? '<option value="">' + t('missions.trigger_webhook_none') + '</option>'
            : webhooks.map(w => `<option value="${escapeAttr(w.id)}" data-slug="${escapeAttr(w.slug)}">${escapeHtml(w.name)} (${escapeHtml(w.slug)})</option>`).join('');
    } catch (err) {
        console.error('Failed to load webhooks:', err);
    }
}

// Load invasion eggs and nests for trigger selectors
async function loadInvasionData() {
    try {
        const [eggsResp, nestsResp] = await Promise.all([
            fetch('/api/invasion/eggs'),
            fetch('/api/invasion/nests')
        ]);
        if (!eggsResp.ok || !nestsResp.ok) return;
        const eggs = await eggsResp.json();
        const nests = await nestsResp.json();

        const eggOptions = '<option value="">' + t('missions.trigger_egg_any') + '</option>' +
            (eggs.eggs || eggs || []).map(e => `<option value="${e.id}" data-name="${escapeHtml(e.name)}">${escapeHtml(e.name)}</option>`).join('');
        const nestOptions = '<option value="">' + t('missions.trigger_nest_any') + '</option>' +
            (nests.nests || nests || []).map(n => `<option value="${n.id}" data-name="${escapeHtml(n.name)}">${escapeHtml(n.name)}</option>`).join('');

        document.getElementById('egg-hatched-egg-select').innerHTML = eggOptions;
        document.getElementById('egg-hatched-nest-select').innerHTML = nestOptions;
        document.getElementById('nest-cleared-nest-select').innerHTML = nestOptions;
    } catch (err) {
        // Invasion not enabled — selectors stay with default "Beliebiges" option
    }
}

async function loadRemoteTargets(selectedMission = null) {
    const select = document.getElementById('remote-target-select');
    if (!select) return;
    select.innerHTML = `<option value="">${t('missions.form_remote_target_loading')}</option>`;
    try {
        const resp = await fetch('/api/missions/v2/remote-targets');
        if (!resp.ok) throw new Error();
        const data = await resp.json();
        remoteTargets = data.targets || [];
        if (remoteTargets.length === 0) {
            select.innerHTML = `<option value="">${t('missions.form_remote_target_none')}</option>`;
            return;
        }
        select.innerHTML = `<option value="">${t('missions.form_remote_target_placeholder')}</option>` +
            remoteTargets.map(target => {
                const label = `${target.nest_name || target.nest_id} · ${target.egg_name || target.egg_id}`;
                return `<option value="${escapeAttr(target.nest_id)}" data-egg-id="${escapeAttr(target.egg_id)}" data-nest-name="${escapeAttr(target.nest_name || '')}" data-egg-name="${escapeAttr(target.egg_name || '')}">${escapeHtml(label)}</option>`;
            }).join('');
        if (selectedMission?.remote_nest_id) {
            select.value = selectedMission.remote_nest_id;
        }
    } catch (err) {
        remoteTargets = [];
        select.innerHTML = `<option value="">${t('missions.form_remote_target_unavailable')}</option>`;
    }
}

// Fill trigger config when editing
function fillTriggerConfig(cfg, type) {
    if (!cfg) return;

    document.getElementById('trigger-min-interval').value = cfg.min_interval_seconds || 0;

    switch (type) {
        case 'mission_completed':
            if (cfg.source_mission_id) {
                const radio = document.querySelector(`input[name="source-mission"][value="${cfg.source_mission_id}"]`);
                if (radio) {
                    radio.checked = true;
                    radio.closest('.mission-option').classList.add('selected');
                }
            }
            document.getElementById('require-success').checked = cfg.require_success || false;
            break;
        case 'email_received':
            document.getElementById('email-folder').value = cfg.email_folder || 'INBOX';
            document.getElementById('email-subject').value = cfg.email_subject_contains || '';
            document.getElementById('email-from').value = cfg.email_from_contains || '';
            break;
        case 'webhook':
            document.getElementById('webhook-select').value = cfg.webhook_id || '';
            break;
        case 'egg_hatched':
            document.getElementById('egg-hatched-egg-select').value = cfg.egg_id || '';
            document.getElementById('egg-hatched-nest-select').value = cfg.nest_id || '';
            break;
        case 'nest_cleared':
            document.getElementById('nest-cleared-nest-select').value = cfg.nest_id || '';
            break;
        case 'mqtt_message':
            document.getElementById('mqtt-topic').value = cfg.mqtt_topic || '';
            document.getElementById('mqtt-payload-contains').value = cfg.mqtt_payload_contains || '';
            document.getElementById('mqtt-min-interval').value = cfg.mqtt_min_interval_seconds || 0;
            break;
        case 'home_assistant_state':
            document.getElementById('ha-entity-id').value = cfg.ha_entity_id || '';
            document.getElementById('ha-state-equals').value = cfg.ha_state_equals || '';
            break;
        case 'device_connected':
            document.getElementById('device-connected-id').value = cfg.device_id || '';
            document.getElementById('device-connected-name').value = cfg.device_name || '';
            break;
        case 'device_disconnected':
            document.getElementById('device-disconnected-id').value = cfg.device_id || '';
            document.getElementById('device-disconnected-name').value = cfg.device_name || '';
            break;
        case 'fritzbox_call':
            document.getElementById('fritzbox-call-type').value = cfg.call_type || '';
            break;
        case 'planner_appointment_due':
            document.getElementById('planner-appointment-id').value = cfg.planner_appointment_id || '';
            document.getElementById('planner-title-contains').value = cfg.planner_title_contains || '';
            break;
        case 'planner_todo_overdue':
            document.getElementById('planner-todo-id').value = cfg.planner_todo_id || '';
            document.getElementById('planner-todo-title-contains').value = cfg.planner_title_contains || '';
            break;
        case 'planner_operational_issue':
            document.getElementById('planner-issue-source').value = cfg.planner_issue_source || '';
            document.getElementById('planner-issue-severity').value = cfg.planner_issue_severity || '';
            document.getElementById('planner-issue-title-contains').value = cfg.planner_title_contains || '';
            break;
    }
}

// Cheatsheet Picker
async function loadCheatsheetPicker(selectedIds = []) {
    const container = document.getElementById('cheatsheet-picker');
    try {
        const resp = await fetch('/api/cheatsheets?active=true&created_by=user');
        if (!resp.ok) throw new Error();
        const sheets = await resp.json();

        if (!sheets || sheets.length === 0) {
            container.innerHTML = `<div class="cheatsheet-picker-empty">${t('missions.form_cheatsheets_none')}</div>`;
            return;
        }

        container.innerHTML = sheets.map(s => {
            const checked = selectedIds.includes(s.id) ? 'checked' : '';
            return `<div class="cheatsheet-picker-item">
                <input type="checkbox" id="cs-${s.id}" value="${s.id}" ${checked}>
                <label for="cs-${s.id}">${escapeHtml(s.name)}</label>
            </div>`;
        }).join('');
    } catch (e) {
        container.innerHTML = `<div class="cheatsheet-picker-empty">${t('missions.form_cheatsheets_none')}</div>`;
    }
}

function getSelectedCheatsheetIds() {
    const checks = document.querySelectorAll('#cheatsheet-picker input[type="checkbox"]:checked');
    return Array.from(checks).map(c => c.value);
}

// Save mission
async function saveMission() {
    const name = document.getElementById('mission-name').value.trim();
    const prompt = document.getElementById('mission-prompt').value.trim();

    if (!name || !prompt) {
        showToast(t('missions.toast_name_prompt_required'), 'error');
        return;
    }

    const checkedRadio = document.querySelector('input[name="exec-type"]:checked');
    if (!checkedRadio) {
        showToast(t('missions.toast_select_exec_type'), 'error');
        return;
    }
    const execType = checkedRadio.value;
    const runnerType = currentRunnerType();
    const mission = {
        name,
        prompt,
        priority: document.getElementById('mission-priority').value,
        execution_type: execType,
        runner_type: runnerType,
        enabled: true,
        locked: document.getElementById('mission-locked').checked,
        auto_prepare: document.getElementById('mission-auto-prepare').checked,
        cheatsheet_ids: getSelectedCheatsheetIds()
    };
    if (runnerType === 'remote') {
        const remoteSelect = document.getElementById('remote-target-select');
        const option = remoteSelect?.options[remoteSelect.selectedIndex];
        if (!remoteSelect?.value || !option?.dataset?.eggId) {
            showToast(t('missions.toast_select_remote_target'), 'error');
            return;
        }
        mission.remote_nest_id = remoteSelect.value;
        mission.remote_nest_name = option.dataset.nestName || '';
        mission.remote_egg_id = option.dataset.eggId;
        mission.remote_egg_name = option.dataset.eggName || '';
    }

    // Add execution-specific config
    if (execType === 'scheduled') {
        mission.schedule = document.getElementById('cron-schedule').value;
        mission.trigger_type = '';
        mission.trigger_config = null;
    } else if (execType === 'triggered') {
        const triggerType = document.querySelector('.trigger-type-btn.active')?.dataset.trigger;
        if (!triggerType) {
            showToast(t('missions.toast_select_trigger_type'), 'error');
            return;
        }
        mission.trigger_type = triggerType;
        mission.trigger_config = buildTriggerConfig(triggerType);
        mission.schedule = '';
    } else {
        // manual — clear both
        mission.schedule = '';
        mission.trigger_type = '';
        mission.trigger_config = null;
    }

    try {
        const url = editingId ? `/api/missions/v2/${editingId}` : '/api/missions/v2';
        const method = editingId ? 'PUT' : 'POST';

        const response = await fetch(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(mission)
        });

        if (!response.ok) {
            throw new Error(await response.text());
        }

        showToast(editingId ? t('missions.toast_mission_updated') : t('missions.toast_mission_created'), 'success');
        closeModal('modal');
        loadData();
    } catch (err) {
        showToast(t('missions.toast_error_prefix') + err.message, 'error');
    }
}

function buildTriggerConfig(type) {
    const config = {};
    config.min_interval_seconds = parseInt(document.getElementById('trigger-min-interval').value || '0', 10) || 0;

    switch (type) {
        case 'mission_completed':
            const selectedMission = document.querySelector('input[name="source-mission"]:checked');
            if (selectedMission) {
                config.source_mission_id = selectedMission.value;
                config.source_mission_name = selectedMission.dataset.name;
            }
            config.require_success = document.getElementById('require-success').checked;
            break;
        case 'email_received':
            config.email_folder = document.getElementById('email-folder').value;
            config.email_subject_contains = document.getElementById('email-subject').value;
            config.email_from_contains = document.getElementById('email-from').value;
            break;
        case 'webhook': {
            const webhookSelect = document.getElementById('webhook-select');
            config.webhook_id = webhookSelect.value;
            config.webhook_slug = webhookSelect.options[webhookSelect.selectedIndex]?.dataset?.slug || '';
            break;
        }
        case 'egg_hatched': {
            const eggSel = document.getElementById('egg-hatched-egg-select');
            const nestSel = document.getElementById('egg-hatched-nest-select');
            config.egg_id = eggSel.value;
            config.egg_name = eggSel.options[eggSel.selectedIndex]?.dataset?.name || '';
            config.nest_id = nestSel.value;
            config.nest_name = nestSel.options[nestSel.selectedIndex]?.dataset?.name || '';
            break;
        }
        case 'nest_cleared': {
            const nestSel = document.getElementById('nest-cleared-nest-select');
            config.nest_id = nestSel.value;
            config.nest_name = nestSel.options[nestSel.selectedIndex]?.dataset?.name || '';
            break;
        }
        case 'mqtt_message': {
            config.mqtt_topic = document.getElementById('mqtt-topic').value.trim();
            config.mqtt_payload_contains = document.getElementById('mqtt-payload-contains').value.trim();
            config.mqtt_min_interval_seconds = parseInt(document.getElementById('mqtt-min-interval').value || '0', 10) || 0;
            break;
        }
        case 'home_assistant_state':
            config.ha_entity_id = document.getElementById('ha-entity-id').value.trim();
            config.ha_state_equals = document.getElementById('ha-state-equals').value.trim();
            break;
        case 'device_connected':
            config.device_id = document.getElementById('device-connected-id').value.trim();
            config.device_name = document.getElementById('device-connected-name').value.trim();
            break;
        case 'device_disconnected':
            config.device_id = document.getElementById('device-disconnected-id').value.trim();
            config.device_name = document.getElementById('device-disconnected-name').value.trim();
            break;
        case 'fritzbox_call':
            config.call_type = document.getElementById('fritzbox-call-type').value;
            break;
        case 'planner_appointment_due':
            config.planner_appointment_id = document.getElementById('planner-appointment-id').value.trim();
            config.planner_title_contains = document.getElementById('planner-title-contains').value.trim();
            break;
        case 'planner_todo_overdue':
            config.planner_todo_id = document.getElementById('planner-todo-id').value.trim();
            config.planner_title_contains = document.getElementById('planner-todo-title-contains').value.trim();
            break;
        case 'planner_operational_issue':
            config.planner_issue_source = document.getElementById('planner-issue-source').value.trim();
            config.planner_issue_severity = document.getElementById('planner-issue-severity').value;
            config.planner_title_contains = document.getElementById('planner-issue-title-contains').value.trim();
            break;
    }

    return config;
}

// Actions
async function runMission(id) {
    try {
        const response = await fetch(`/api/missions/v2/${id}/run`, { method: 'POST' });
        if (!response.ok) throw new Error(await response.text());
        showToast(t('missions.toast_queued'), 'success');
        loadData();
    } catch (err) {
        showToast(t('missions.toast_error_prefix') + err.message, 'error');
    }
}

async function removeFromQueue(id) {
    try {
        const response = await fetch(`/api/missions/v2/${id}/queue`, { method: 'DELETE' });
        if (!response.ok) throw new Error(await response.text());
        showToast(t('missions.toast_removed_from_queue'), 'success');
        loadData();
    } catch (err) {
        showToast(t('missions.toast_error_prefix') + err.message, 'error');
    }
}

function editMission(id) {
    openMissionModal(id);
}

function duplicateMission(id) {
    const m = missions.find(x => x.id === id);
    if (!m) return;
    openMissionModal(); // Opens in 'new' mode
    document.getElementById('mission-name').value = m.name + ' (Copy)';
    document.getElementById('mission-prompt').value = m.prompt;
    document.getElementById('mission-priority').value = m.priority;
    document.getElementById('mission-locked').checked = false;
    document.getElementById('mission-auto-prepare').checked = m.auto_prepare || false;
    selectRunnerType(m.runner_type || 'local');
    loadRemoteTargets(m);
    selectExecType(m.execution_type);
    if (m.execution_type === 'scheduled') {
        document.getElementById('cron-schedule').value = m.schedule || '';
        syncCronPreset(m.schedule || '');
    } else if (m.execution_type === 'triggered') {
        selectTriggerType(m.trigger_type);
        fillTriggerConfig(m.trigger_config, m.trigger_type);
    }
    // Copy cheatsheet selections
    if (m.cheatsheet_ids && m.cheatsheet_ids.length) {
        loadCheatsheetPicker(m.cheatsheet_ids);
    }
}

async function deleteMission(id) {
    const mission = missions.find(m => m.id === id);
    if (!mission) return;

    const confirmed = await showConfirm(t('common.confirm_title'), t('missions.confirm_delete', { name: mission.name }));
    if (!confirmed) {
        return;
    }

    try {
        const response = await fetch(`/api/missions/v2/${encodeURIComponent(id)}`, { method: 'DELETE' });
        if (!response.ok) throw new Error(await response.text());
        showToast(t('missions.toast_mission_deleted'), 'success');
        loadData();
    } catch (err) {
        const detail = err?.message || '';
        const isRemoteMission = mission.runner_type === 'remote' || !!mission.remote_nest_id;
        const canForceDelete = isRemoteMission && /not connected|remote nest|timed out/i.test(detail);
        if (canForceDelete) {
            const forceConfirmed = await showConfirm(
                t('common.confirm_title'),
                t('missions.confirm_force_delete_remote', { name: mission.name })
            );
            if (!forceConfirmed) {
                return;
            }
            try {
                const forceResponse = await fetch(`/api/missions/v2/${encodeURIComponent(id)}?force=true`, { method: 'DELETE' });
                if (!forceResponse.ok) throw new Error(await forceResponse.text());
                showToast(t('missions.toast_mission_deleted'), 'success');
                loadData();
                return;
            } catch (forceErr) {
                showToast(t('missions.toast_error_prefix') + forceErr.message, 'error');
                return;
            }
        }
        showToast(t('missions.toast_error_prefix') + err.message, 'error');
    }
}

// Helpers
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function escapeAttr(s) {
    return String(s ?? '')
        .replace(/&/g, '&amp;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}

function formatTime(isoString) {
    if (!isoString) return t('missions.time_never');
    const date = new Date(isoString);
    const now = new Date();
    const diff = now - date;

    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(diff / 3600000);
    const days = Math.floor(diff / 86400000);

    if (minutes < 1) return t('missions.time_just_now');
    if (minutes < 60) return t('missions.time_minutes_ago', { n: minutes });
    if (hours < 24) return t('missions.time_hours_ago', { n: hours });
    if (days < 7) return t('missions.time_days_ago', { n: days });
    return date.toLocaleDateString(document.documentElement.lang || 'de-DE');
}

// ═══════════════════════════════════════════════════════════════
// MISSION PREPARATION
// ═══════════════════════════════════════════════════════════════

function renderPrepBadge(mission) {
    const status = mission.preparation_status;
    if (!status || status === 'none') return '';
    const label = t('missions.prep_status_' + status);
    return `<span class="badge badge-prep-${status}">${label}</span>`;
}

function renderPrepButton(mission, isRunning) {
    const status = mission.preparation_status || 'none';
    const isPreparing = status === 'preparing';
    const mid = escapeAttr(mission.id);

    if (status === 'prepared') {
    return `<button class="mc-btn" data-mission-action="view-prepared" data-mission-id="${mid}" title="${t('missions.prep_view_title')}">${svgIcons.info}</button>` +
               `<button class="mc-btn" data-mission-action="invalidate-prepared" data-mission-id="${mid}" title="${t('missions.prep_btn_invalidate')}">${svgIcons.refresh}</button>`;
    }
    return `<button class="mc-btn" data-mission-action="prepare" data-mission-id="${mid}" title="${t('missions.prep_btn_prepare')}" ${isPreparing || isRunning ? 'disabled' : ''}>${svgIcons.cog}</button>`;
}

async function prepareMission(id) {
    try {
        const response = await fetch(`/api/missions/v2/${id}/prepare`, { method: 'POST' });
        if (!response.ok) throw new Error(await response.text());
        showToast(t('missions.prep_toast_started'), 'success');
        loadData();
    } catch (err) {
        showToast(t('missions.prep_toast_error') + ': ' + err.message, 'error');
    }
}

async function invalidatePreparation(id) {
    try {
        const response = await fetch(`/api/missions/v2/${id}/prepared`, { method: 'DELETE' });
        if (!response.ok) throw new Error(await response.text());
        showToast(t('missions.prep_toast_invalidated'), 'success');
        loadData();
    } catch (err) {
        showToast(t('missions.prep_toast_error') + ': ' + err.message, 'error');
    }
}

async function viewPreparedContext(id) {
    try {
        const response = await fetch(`/api/missions/v2/${id}/prepared`);
        if (!response.ok) throw new Error(await response.text());
        const data = await response.json();
        document.getElementById('prep-modal-title').textContent = t('missions.prep_view_title');
        let content = '';
        if (data.analysis) {
            const a = data.analysis;
            if (a.summary) content += a.summary + '\n\n';
            if (a.essential_tools && a.essential_tools.length) {
                content += '── Tools ──\n';
                a.essential_tools.forEach(tool => { content += `• ${tool.tool_name}: ${tool.purpose}\n`; });
                content += '\n';
            }
            if (a.step_plan && a.step_plan.length) {
                content += '── Steps ──\n';
                a.step_plan.forEach((s, i) => { content += `${i+1}. ${s.action}${s.expectation ? ' — ' + s.expectation : ''}\n`; });
                content += '\n';
            }
            if (a.pitfalls && a.pitfalls.length) {
                content += '── Pitfalls ──\n';
                a.pitfalls.forEach(p => { content += `⚠ ${p.risk}${p.mitigation ? ' → ' + p.mitigation : ''}\n`; });
                content += '\n';
            }
            if (data.confidence) content += `${t('missions.prep_confidence')}: ${Math.round(data.confidence * 100)}%\n`;
        } else {
            content = JSON.stringify(data, null, 2);
        }
        document.getElementById('prep-context-body').textContent = content;
        openModal('prep-modal');
    } catch (err) {
        showToast(t('missions.prep_toast_error') + ': ' + err.message, 'error');
    }
}
