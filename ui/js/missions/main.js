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

// Icons
const icons = {
    play: '▶️',
    pause: '⏸️',
    stop: '⏹️',
    edit: '✏️',
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

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    loadData();
    setInterval(loadData, 2000); // Refresh every 2 seconds
});

// Show loading skeleton
function showLoading() {
    const container = document.getElementById('missions-grid');
    container.innerHTML = `
                <div class="mission-card skeleton" style="height: 120px;"></div>
                <div class="mission-card skeleton" style="height: 120px;"></div>
                <div class="mission-card skeleton" style="height: 120px;"></div>
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
        render();
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
    renderStatusBar();
    renderQueue();
    renderMissions();
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
        section.style.display = 'none';
        return;
    }

    section.style.display = 'block';
    let html = '';

    // Show running mission first
    if (queue.running) {
        const runningMission = missions.find(m => m.id === queue.running);
        if (runningMission) {
            html += `
                        <div class="queue-item priority-${runningMission.priority}" style="border-left-color: var(--accent-primary);">
                            <div class="queue-position">${icons.running}</div>
                            <div class="queue-info">
                                <div class="queue-name">${escapeHtml(runningMission.name)}</div>
                                <div class="queue-meta">${t('missions.queue_running_now')} | ${t('missions.queue_priority_prefix')} ${runningMission.priority}</div>
                            </div>
                            <span class="queue-trigger" style="background: var(--accent-primary);">${t('missions.queue_active_badge')}</span>
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
                        <button class="icon-btn" onclick="removeFromQueue('${mission.id}')" title="${t('missions.queue_remove_title')}">
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
    container.innerHTML = filtered.map(mission => {
        const isRunning = mission.id === queue.running;
        const isQueued = queue.items.some(i => i.mission_id === mission.id);
        const statusClass = isRunning ? 'running' : isQueued ? 'queued' : mission.status === 'waiting' ? 'waiting' : '';

        const priorityBadge = `<span class="badge badge-priority-${mission.priority}">${mission.priority}</span>`;
        const typeBadge = `<span class="badge badge-type-${mission.execution_type}">${mission.execution_type}</span>`;
        const statusBadge = isRunning ? `<span class="badge badge-running">${t('missions.card_badge_running')}</span>` : '';

        let triggerInfo = '';
        if (mission.execution_type === 'triggered' && mission.trigger_config) {
            triggerInfo = renderTriggerInfo(mission);
        }

        const lastRun = mission.last_run ? formatTime(mission.last_run) : t('missions.card_last_run_never');
        const resultIcon = mission.last_result === 'success' ? icons.success : mission.last_result === 'error' ? icons.error : '';

        return `
                    <div class="mission-card ${statusClass}${isFirstRender ? ' entering' : ''}">`
                        <div class="mission-header">
                            <div class="mission-title">
                                <span class="mission-name">${escapeHtml(mission.name)}</span>
                                ${mission.locked ? `<span class="mission-locked" title="${t('missions.card_locked_title')}">${icons.lock}</span>` : ''}
                            </div>
                            <div class="mission-badges">
                                ${priorityBadge}
                                ${typeBadge}
                                ${statusBadge}
                            </div>
                        <div class="mission-body">
                            <div class="mission-prompt">${escapeHtml(mission.prompt)}</div>
                            ${triggerInfo}
                            <div class="mission-log-wrapper" style="margin-top: 10px;">
                                <div class="mission-log-toggle" onclick="this.nextElementSibling.style.display = this.nextElementSibling.style.display === 'none' ? 'block' : 'none'" style="cursor:pointer; font-size: 0.8rem; color: var(--accent); opacity: 0.8;">
                                    📝 <span style="text-decoration: underline;">${t('missions.card_view_log')}</span>
                                </div>
                                <div class="mission-log-content" style="display: none; margin-top: 8px; background: rgba(0,0,0,0.2); padding: 8px; border-radius: 4px; font-family: monospace; font-size: 0.75rem; white-space: pre-wrap; word-break: break-all; max-height: 150px; overflow-y: auto;">
                                    ${escapeHtml(mission.last_output || t('missions.card_no_output'))}
                                </div>
                            </div>
                        </div>
                        <div class="mission-footer">
                            <div class="mission-stats">
                                <span>${resultIcon} ${lastRun}</span>
                                <span>📊 ${t('missions.meta_run_count', { count: mission.run_count })}</span>
                            </div>
                                <button class="icon-btn" onclick="runMission('${mission.id}')" title="${t('missions.card_btn_run_title')}" ${isRunning ? 'disabled' : ''}>
                                    ${icons.play}
                                </button>
                                <button class="icon-btn" onclick="duplicateMission('${mission.id}')" title="${t('missions.card_btn_duplicate_title')}">
                                    ${icons.duplicate}
                                </button>
                                <button class="icon-btn" onclick="editMission('${mission.id}')" title="${t('missions.card_btn_edit_title')}">
                                    ${icons.edit}
                                </button>
                                <button class="icon-btn" onclick="deleteMission('${mission.id}')" title="${t('missions.card_btn_delete_title')}" ${mission.locked ? 'disabled' : ''}>
                                    ${icons.delete}
                                </button>
                            </div>
                        </div>
                    </div>
                `;
    }).join('');
}

function renderTriggerInfo(mission) {
    const cfg = mission.trigger_config;
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
            triggerText = `📡 ${mqttParts.join(' | ')}`;
            break;
        case 'system_startup':
            triggerText = `${t('missions.trigger_system_startup_badge')}`;
            break;
    }

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

            selectExecType(mission.execution_type);

            if (mission.execution_type === 'scheduled') {
                document.getElementById('cron-schedule').value = mission.schedule || '';
            } else if (mission.execution_type === 'triggered') {
                selectTriggerType(mission.trigger_type);
                fillTriggerConfig(mission.trigger_config, mission.trigger_type);
            }
        }
    } else {
        document.getElementById('mission-form').reset();
        document.getElementById('mission-id').value = '';
        selectExecType('manual');
    }

    // Load mission selector for triggers
    loadMissionSelector();
    // Load webhooks
    loadWebhooks();
    // Load invasion eggs/nests for triggers
    loadInvasionData();
    // Load cheatsheet picker
    loadCheatsheetPicker(missionId ? (missions.find(m => m.id === missionId)?.cheatsheet_ids || []) : []);

    openModal('modal');
}

// Execution Type Selection
function selectExecType(type) {
    document.querySelectorAll('.exec-type-option').forEach(opt => {
        opt.classList.remove('selected');
        if (opt.querySelector('input').value === type) {
            opt.classList.add('selected');
            opt.querySelector('input').checked = true;
        }
    });

    document.getElementById('config-scheduled').style.display = type === 'scheduled' ? 'block' : 'none';
    document.getElementById('config-triggered').style.display = type === 'triggered' ? 'block' : 'none';
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
        container.innerHTML = '<div style="padding: 12px; color: var(--text-secondary);">' + t('missions.trigger_no_suitable_missions') + '</div>';
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
            : webhooks.map(w => `<option value="${w.id}" data-slug="${w.slug}">${escapeHtml(w.name)} (${w.slug})</option>`).join('');
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

// Fill trigger config when editing
function fillTriggerConfig(cfg, type) {
    if (!cfg) return;

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
            break;
    }
}

// Cheatsheet Picker
async function loadCheatsheetPicker(selectedIds = []) {
    const container = document.getElementById('cheatsheet-picker');
    try {
        const resp = await fetch('/api/cheatsheets?active=true');
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
    const mission = {
        name,
        prompt,
        priority: document.getElementById('mission-priority').value,
        execution_type: execType,
        enabled: true,
        locked: document.getElementById('mission-locked').checked,
        cheatsheet_ids: getSelectedCheatsheetIds()
    };

    // Add execution-specific config
    if (execType === 'scheduled') {
        mission.schedule = document.getElementById('cron-schedule').value;
    } else if (execType === 'triggered') {
        const triggerType = document.querySelector('.trigger-type-btn.active')?.dataset.trigger;
        if (!triggerType) {
            showToast(t('missions.toast_select_trigger_type'), 'error');
            return;
        }
        mission.trigger_type = triggerType;
        mission.trigger_config = buildTriggerConfig(triggerType);
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
            break;
        }
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
    selectExecType(m.execution_type);
    if (m.execution_type === 'scheduled') {
        document.getElementById('cron-schedule').value = m.schedule || '';
    } else if (m.execution_type === 'triggered') {
        selectTriggerType(m.trigger_type);
        fillTriggerConfig(m.trigger_config, m.trigger_type);
    }
}

async function deleteMission(id) {
    const mission = missions.find(m => m.id === id);
    if (!mission) return;

    if (!confirm(t('missions.confirm_delete', { name: mission.name }))) {
        return;
    }

    try {
        const response = await fetch(`/api/missions/v2/${id}`, { method: 'DELETE' });
        if (!response.ok) throw new Error(await response.text());
        showToast(t('missions.toast_mission_deleted'), 'success');
        loadData();
    } catch (err) {
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
