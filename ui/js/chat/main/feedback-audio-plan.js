/* ── Data ── */
let conversation = [];
// Tracks /files/ paths already rendered via SSE 'image' events
// so appendMessage() can skip the duplicate markdown image.
let seenSSEImages = new Set();
// Tracks whether the HTTP response for the current request has been rendered.
// Used by the SSE 'done' fallback to avoid duplicate rendering.
let _httpResponseRendered = false;
// Set when the HTTP fetch fails with a network error while SSE is still alive.
// Suppresses the immediate error message and keeps the status bar visible so the
// SSE 'done' handler can recover the response once the agent finishes.
let _fetchConnectionLost = false;
let seenSSEAudios = new Set();
let seenSSEVideos = new Set();
let seenSSELiveStreams = new Set();
let seenSSEYouTubeVideos = new Set();
let seenSSEDocuments = new Set();
let seenSSESTLs = new Set();
let currentPlanState = null;

function resetSSEDedupSets() {
    const sets = [
        seenSSEImages,
        seenSSEAudios,
        seenSSEVideos,
        seenSSELiveStreams,
        seenSSEYouTubeVideos,
        seenSSEDocuments,
        seenSSESTLs
    ];
    if (typeof seenSSEAudioPlayers !== 'undefined') {
        sets.push(seenSSEAudioPlayers);
    }
    sets.forEach(set => {
        if (set && typeof set.clear === 'function') set.clear();
    });
    if (typeof pendingAutoplayAudios !== 'undefined' && pendingAutoplayAudios && typeof pendingAutoplayAudios.clear === 'function') {
        pendingAutoplayAudios.clear();
    }
}

// If user has explicitly set a preference in localStorage, use it.
// Only fall back to server defaults if no preference has been saved yet.
const _storedDebug = localStorage.getItem('aurago-debug');
let debugMode = _storedDebug !== null ? (_storedDebug === 'true') : (SHOW_TOOL_RESULTS || AGENT_DEBUG_MODE);
function updateDebugPill() {
    const pill = document.getElementById('debug-pill');
    if (!pill) return;
    if (!AGENT_DEBUG_MODE) { pill.style.display = 'none'; return; }
    if (debugMode) { pill.classList.add('debug-on'); pill.textContent = t('chat.debug_pill_on'); }
    else { pill.classList.remove('debug-on'); pill.textContent = t('chat.debug_pill'); }
}
updateDebugPill();
function toggleDebugMode() {
    debugMode = !debugMode;
    localStorage.setItem('aurago-debug', debugMode);
    updateDebugPill();
    // Sync to server so new messages also reflect debug state
    const cmd = debugMode ? '/debug on' : '/debug off';
    fetch('/v1/chat/completions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ model: 'aurago', messages: [{ role: 'user', content: cmd }] })
    }).catch(() => { }); // fire-and-forget
}
bindHeaderActivation(document.getElementById('debug-pill'), toggleDebugMode);

/* ── Speaker mode (TTS auto-play) ── */
let speakerMode = localStorage.getItem('aurago-speaker') === 'true';
const _audioQueue = [];
let _audioPlaying = false;
const _audioAutoplayFailedEvent = 'aurago-audio-autoplay-failed';

function updateSpeakerButton() {
    const btn = document.getElementById('speaker-toggle');
    if (!btn) return;
    setIconButton(btn, speakerMode ? 'speaker' : 'speaker-muted');
    btn.title = speakerMode ? t('chat.speaker_on_title') : t('chat.speaker_off_title');
    btn.classList.toggle('speaker-active', speakerMode);
}
updateSpeakerButton();

// Notify backend of current state on initial load
fetch('/api/preferences', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ speaker_mode: speakerMode })
}).catch(() => { });

function toggleSpeakerMode() {
    speakerMode = !speakerMode;
    localStorage.setItem('aurago-speaker', speakerMode);
    updateSpeakerButton();
    // Notify backend so prompt hint can be adjusted
    fetch('/api/preferences', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ speaker_mode: speakerMode })
    }).catch(() => { });
}
bindHeaderActivation(document.getElementById('speaker-toggle'), toggleSpeakerMode);

function _playNextInQueue() {
    if (_audioQueue.length === 0) { _audioPlaying = false; return; }
    _audioPlaying = true;
    const src = _audioQueue.shift();
    const audio = new Audio(src);
    let settled = false;
    const next = () => {
        if (settled) return;
        settled = true;
        _playNextInQueue();
    };
    const fail = () => {
        if (typeof window !== 'undefined' && typeof window.dispatchEvent === 'function') {
            window.dispatchEvent(new CustomEvent(_audioAutoplayFailedEvent, { detail: { src } }));
        }
        next();
    };
    audio.addEventListener('ended', next, { once: true });
    audio.addEventListener('error', fail, { once: true });
    audio.play().catch(fail);
}

function enqueueAutoPlay(src) {
    _audioQueue.push(src);
    if (!_audioPlaying) _playNextInQueue();
}

function ensureTaskPanel() {
    let panel = document.getElementById('todo-debug-panel');
    if (!panel) {
        panel = document.createElement('div');
        panel.id = 'todo-debug-panel';
        panel.className = 'todo-debug-panel';
        const statusContainer = document.getElementById('agentStatusContainer');
        if (statusContainer) statusContainer.parentNode.insertBefore(panel, statusContainer);
        else document.body.appendChild(panel);
    }
    return panel;
}

function renderPlanStatusBadge(status) {
    const key = status ? `plans.status_${status}` : '';
    const fallback = status || '';
    const label = key && typeof t === 'function' ? t(key) : fallback;
    return `<span class="badge badge-secondary">${escapeHtml(label || fallback)}</span>`;
}

function renderPlanTask(task) {
    const status = task && task.status ? task.status : 'pending';
    const icon = chatIconMarkup(status === 'completed' ? 'complete'
        : status === 'in_progress' ? 'in-progress'
        : status === 'blocked' ? 'blocked'
        : status === 'failed' ? 'error'
        : status === 'skipped' ? 'skipped'
        : 'pending');
    const desc = task && task.description ? `<div class="todo-item-meta">${escapeHtml(task.description)}</div>` : '';
    const blocker = task && task.blocker_reason ? `<div class="todo-item-meta">Blocker: ${escapeHtml(task.blocker_reason)}</div>` : '';
    const artifacts = task && Array.isArray(task.artifacts) && task.artifacts.length
        ? task.artifacts.slice(-2).map(a => `<div class="todo-item-meta">Artifact: ${escapeHtml((a.label || a.type || 'artifact') + ' = ' + (a.value || ''))}</div>`).join('')
        : '';
    return '<div class="todo-item ' + (status === 'completed' ? 'todo-done' : 'todo-pending') + '">'
        + icon + ' ' + escapeHtml(task.title || '') + desc + blocker + artifacts + '</div>';
}

function updatePlanPanel(plan) {
    currentPlanState = plan || null;
    if (!plan) {
        hideTodoPanel();
        return;
    }
    const panel = ensureTaskPanel();
    const events = Array.isArray(plan.events) ? plan.events : [];
    const tasks = Array.isArray(plan.tasks) ? plan.tasks : [];
    const counts = plan.task_counts || {};
    const progress = Number.isFinite(plan.progress_pct) ? plan.progress_pct : 0;
    const currentTask = plan.current_task
        ? `<div class="todo-item todo-pending">${chatIconMarkup('target')} ${escapeHtml(plan.current_task)}</div>`
        : '';
    const blocked = plan.blocked_reason
        ? `<div class="todo-item-meta">${escapeHtml(t('plans.blocked_reason'))}: ${escapeHtml(plan.blocked_reason)}</div>`
        : '';
    const recommendation = plan.recommendation
        ? `<div class="todo-item-meta">${escapeHtml((typeof t === 'function' ? t('plans.recommendation') : 'Recommended next step') + ': ' + plan.recommendation)}</div>`
        : '';
    const latestEvent = events.length > 0 && events[0].message
        ? `<div class="todo-item-meta">${escapeHtml(events[0].message)}</div>`
        : '';
    const headerTitle = escapeHtml(plan.title || t('common.nav_plans'));
    const progressLabel = escapeHtml(t('plans.progress'));
    panel.innerHTML = `
        <div class="todo-debug-header"><span>${headerTitle}</span>${renderPlanStatusBadge(plan.status)}</div>
        <div class="todo-debug-body">
            <div class="todo-item todo-pending">${chatIconMarkup('activity')} ${progressLabel}: ${progress}% (${counts.completed || 0}/${counts.total || tasks.length || 0})</div>
            ${currentTask}
            ${blocked}
            ${recommendation}
            ${tasks.map(renderPlanTask).join('')}
            ${latestEvent}
        </div>`;
    chatSetHidden(panel, false);
}

/* ── Session Todo Panel ── */
function updateTodoPanel(todoText) {
    if (currentPlanState) return;
    if (!todoText) { hideTodoPanel(); return; }
    let panel = ensureTaskPanel();
    panel.innerHTML = '<div class="todo-debug-header">' + escapeHtml(t('chat.todo_panel_title')) + '</div><div class="todo-debug-body"></div>';
    const body = panel.querySelector('.todo-debug-body');
    if (body) {
        // Parse markdown-style todo items into HTML
        const lines = todoText.split('\n').filter(l => l.trim());
        body.innerHTML = lines.map(line => {
            const done = /^\s*-\s*\[x\]/i.test(line);
            const text = line.replace(/^\s*-\s*\[[ x]\]\s*/i, '').trim();
            return '<div class="todo-item ' + (done ? 'todo-done' : 'todo-pending') + '">'
                + chatIconMarkup(done ? 'complete' : 'pending') + ' ' + escapeHtml(text) + '</div>';
        }).join('');
    }
    chatSetHidden(panel, false);
}

function hideTodoPanel() {
    currentPlanState = null;
    const panel = document.getElementById('todo-debug-panel');
    chatSetHidden(panel, true);
}

/* Handle personality change (shared helper for both selects) */
