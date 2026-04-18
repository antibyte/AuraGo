// AuraGo – index page logic
// Extracted from index.html

/* ── DOM refs ── */
const chatBox = document.getElementById('chat-box');
const chatContent = document.getElementById('chat-content');
const chatForm = document.getElementById('chat-form');
const userInput = document.getElementById('user-input');
const sendBtn = document.getElementById('send-btn');
const stopBtn = document.getElementById('stop-btn');
const composerMoreBtn = document.getElementById('composer-more-btn');
const composerPanel = document.getElementById('composer-panel');
const feedbackToggleBtn = document.getElementById('feedback-toggle-btn');
const moodFeedbackRow = document.getElementById('mood-feedback-row');
const cheatsheetPickerBtn = document.getElementById('cheatsheet-picker-btn');

const cheatsheetPickerOverlay = document.getElementById('cheatsheet-picker-overlay');
const cheatsheetPickerList = document.getElementById('cheatsheet-picker-list');
const cheatsheetPickerSendBtn = document.getElementById('cheatsheet-picker-send');
const cheatsheetPickerCancelBtn = document.getElementById('cheatsheet-picker-cancel');
const cheatsheetPickerCloseXBtn = document.getElementById('cheatsheet-picker-close-x');

let cheatsheetPickerItems = [];
let selectedCheatsheetId = '';


/* ── Mood Feedback Buttons (insert emoji + personality feedback) ── */
const moodEmojiMap = {
    positive: '👍',
    negative: '👎',
    angry: '😡',
    laughing: '😂',
    crying: '😢',
    amazed: '😲'
};
document.querySelectorAll('.mood-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
        const feedbackType = btn.dataset.feedback;
        const emoji = moodEmojiMap[feedbackType] || '';

        // Insert emoji at cursor position in the textarea
        const ta = document.getElementById('user-input');
        const start = ta.selectionStart;
        const end = ta.selectionEnd;
        const before = ta.value.substring(0, start);
        const after = ta.value.substring(end);
        ta.value = before + emoji + after;
        ta.selectionStart = ta.selectionEnd = start + emoji.length;
        ta.focus();
        autoResize();

        // Send personality feedback to backend
        btn.disabled = true;
        try {
            const res = await fetch('/api/personality/feedback', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ type: feedbackType })
            });
            if (res.ok) {
                btn.classList.add('flash-ok');
                setTimeout(() => btn.classList.remove('flash-ok'), 500);
            }
        } catch (e) {
            console.error('Feedback error:', e);
        } finally {
            btn.disabled = false;
        }
    });
});

/* ── Auto-resize textarea (max 5 lines) ── */
function autoResize() {
    userInput.style.height = 'auto';
    const maxH = parseFloat(getComputedStyle(userInput).maxHeight);
    userInput.style.height = Math.min(userInput.scrollHeight, maxH) + 'px';
}
userInput.addEventListener('input', autoResize);

/* ── Enter submits, Shift+Enter inserts newline ── */
userInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        chatForm.requestSubmit();
    }
});

function updateChatInputPlaceholder() {
    if (!userInput) return;
    const isMobile = window.matchMedia && window.matchMedia('(max-width: 639px)').matches;
    userInput.placeholder = isMobile ? t('chat.input_placeholder_mobile') : t('chat.input_placeholder');
}

window.addEventListener('resize', updateChatInputPlaceholder);

/* ── i18n — I18N and t() now provided by page head + shared.js ── */

function applyI18n() {
    if (typeof window._auragoApplySharedI18n === 'function') {
        window._auragoApplySharedI18n();
    }
    document.title = t('chat.page_title');
    /* Header pills & controls */
    const sessionToggleBtn = document.getElementById('session-toggle-btn');
    if (sessionToggleBtn) sessionToggleBtn.title = t('chat.sessions_title');
    const themeToggleEl = document.getElementById('theme-toggle');
    if (themeToggleEl) themeToggleEl.title = t('common.toggle_theme');
    document.getElementById('speaker-toggle').title = speakerMode ? t('chat.speaker_on_title') : t('chat.speaker_off_title');
    document.getElementById('tokenCounter').textContent = t('chat.token_counter_default');
    document.getElementById('budgetPill').title = t('chat.budget_pill_title');
    document.getElementById('creditsPill').title = t('chat.credits_pill_title');
    document.getElementById('personality-select').title = t('chat.personality_select_title');
    const psOpt = document.querySelector('#personality-select option[disabled]');
    if (psOpt) psOpt.textContent = t('chat.personality_loading');
    document.getElementById('moodToggle').title = t('chat.mood_toggle_title');
    document.getElementById('moodText').textContent = t('chat.mood_default_text');
    document.getElementById('moodPanelLabel').textContent = t('chat.mood_default_text');
    document.getElementById('debug-pill').title = t('chat.debug_pill_title');
    document.getElementById('debug-pill').textContent = t('chat.debug_pill');
    // connectionPill state is managed exclusively by setConnectionState() — do not override here
    const clearBtnLabel = document.querySelector('#clear-btn .tool-label');
    if (clearBtnLabel) clearBtnLabel.textContent = t('chat.btn_clear');
    const uploadBtnLabel = document.querySelector('#upload-btn .tool-label');
    if (uploadBtnLabel) uploadBtnLabel.textContent = t('chat.upload_btn_title');
    const cheatsheetBtnLabel = document.querySelector('#cheatsheet-picker-btn .tool-label');
    if (cheatsheetBtnLabel) cheatsheetBtnLabel.textContent = t('chat.cheatsheet_picker_button');
    const pushBtnLabel = document.querySelector('#push-btn .tool-label');
    if (pushBtnLabel) pushBtnLabel.textContent = t('pwa.btn_push_title');
    const stopBtnLabel = document.querySelector('#stop-btn .tool-label');
    if (stopBtnLabel) stopBtnLabel.textContent = t('chat.stop_btn_title');
    const feedbackBtnLabel = document.querySelector('#feedback-toggle-btn .tool-label');
    if (feedbackBtnLabel) feedbackBtnLabel.textContent = t('chat.feedback_toggle_title');
    const logoutBtn = document.getElementById('logout-btn');
    if (logoutBtn) { logoutBtn.title = t('chat.logout_title'); logoutBtn.textContent = t('chat.logout_label'); }
    /* Radial nav */
    const rt = document.getElementById('radialTrigger');
    if (rt) rt.setAttribute('aria-label', t('common.nav_aria_label'));
    document.querySelectorAll('.radial-item').forEach(el => {
        const href = el.getAttribute('href');
        const lbl = el.querySelector('.radial-item-label');
        if (!lbl) return;
        if (href === '/') lbl.textContent = t('common.nav_chat');
        else if (href === '/dashboard') lbl.textContent = t('common.nav_dashboard');
        else if (href === '/plans') lbl.textContent = t('common.nav_plans');
        else if (href === '/missions') lbl.textContent = t('common.nav_missions');
        else if (href === '/config') lbl.textContent = t('common.nav_config');
        else if (href === '/invasion') lbl.textContent = t('common.nav_invasion');
        else if (href === '/auth/logout') lbl.textContent = t('common.nav_logout');
    });
    /* Greeting */
    const greetText = document.querySelector('.greeting-text');
    if (greetText) greetText.textContent = t('chat.greeting');
    /* Input area */
    updateChatInputPlaceholder();
    document.getElementById('upload-btn').title = t('chat.upload_btn_title');
    const cheatsheetPickerButton = document.getElementById('cheatsheet-picker-btn');
    if (cheatsheetPickerButton) cheatsheetPickerButton.title = t('chat.cheatsheet_picker_button_title');
    document.getElementById('send-btn').title = t('chat.send_btn_title');
    document.getElementById('stop-btn').title = t('chat.stop_btn_title');
    if (composerMoreBtn) composerMoreBtn.title = t('chat.more_actions_title');
    const pushBtn = document.getElementById('push-btn');
    if (pushBtn) pushBtn.title = t('pwa.btn_push_title');
    if (feedbackToggleBtn) feedbackToggleBtn.title = t('chat.feedback_toggle_title');
    /* Feedback buttons */
    document.querySelectorAll('.mood-btn').forEach(btn => {
        const fb = btn.dataset.feedback;
        btn.title = t('chat.feedback_' + fb + '_title');
    });
    /* Attachment chip */
    const ac = document.getElementById('attachment-clear');
    if (ac) ac.title = t('chat.attachment_clear_title');
    /* Modal */
    document.getElementById('modal-cancel').textContent = t('common.btn_cancel');
    document.getElementById('modal-confirm').textContent = t('common.btn_ok');
    if (cheatsheetPickerCancelBtn) cheatsheetPickerCancelBtn.textContent = t('chat.close');
    if (cheatsheetPickerSendBtn) cheatsheetPickerSendBtn.textContent = t('chat.cheatsheet_picker_send');
    const cheatsheetPickerTitle = document.querySelector('[data-i18n="chat.cheatsheet_picker_title"]');
    if (cheatsheetPickerTitle) cheatsheetPickerTitle.textContent = t('chat.cheatsheet_picker_title');
    const cheatsheetPickerCloseX = document.getElementById('cheatsheet-picker-close-x');
    if (cheatsheetPickerCloseX) cheatsheetPickerCloseX.setAttribute('aria-label', t('chat.close'));
    /* Lightbox */
    const lbc = document.getElementById('img-lightbox-close');
    if (lbc) lbc.title = t('chat.lightbox_close_title');
    /* Generic data-i18n attributes (drawer, etc.) */
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        if (key) el.textContent = t(key);
    });
    document.querySelectorAll('[data-i18n-title]').forEach(el => {
        const key = el.getAttribute('data-i18n-title');
        if (key) el.title = t(key);
    });
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const key = el.getAttribute('data-i18n-placeholder');
        if (key) el.placeholder = t(key);
    });
}

function chatSetHidden(el, hidden) {
    if (!el) return;
    el.classList.toggle('is-hidden', hidden);
}

/* ── Desktop detection for inline footer buttons ── */
const _desktopMQ = window.matchMedia('(min-width: 768px)');
function isDesktopView() {
    return _desktopMQ.matches;
}

function closeComposerPanel() {
    if (!composerMoreBtn || !composerPanel) return;
    /* On desktop the panel is always visible – ignore close requests */
    if (isDesktopView()) return;
    composerPanel.classList.add('is-hidden');
    composerMoreBtn.classList.remove('is-open');
    composerMoreBtn.setAttribute('aria-expanded', 'false');
}

function closeMoodFeedbackRow() {
    chatSetHidden(moodFeedbackRow, true);
}

function toggleComposerPanel(forceOpen) {
    if (!composerMoreBtn || !composerPanel) return;
    /* On desktop the panel is always visible – no toggle needed */
    if (isDesktopView()) return;
    const shouldOpen = typeof forceOpen === 'boolean' ? forceOpen : composerPanel.classList.contains('is-hidden');
    composerPanel.classList.toggle('is-hidden', !shouldOpen);
    composerMoreBtn.classList.toggle('is-open', shouldOpen);
    composerMoreBtn.setAttribute('aria-expanded', shouldOpen ? 'true' : 'false');
}


function closeCheatsheetPicker() {
    if (!cheatsheetPickerOverlay) return;
    cheatsheetPickerOverlay.classList.remove('active');
    selectedCheatsheetId = '';
    if (cheatsheetPickerSendBtn) cheatsheetPickerSendBtn.disabled = true;
}

function renderCheatsheetPickerList() {
    if (!cheatsheetPickerList) return;
    const activeSheets = cheatsheetPickerItems.filter((sheet) => sheet && sheet.active !== false);
    if (!activeSheets.length) {
        cheatsheetPickerList.innerHTML = `<div class="cheatsheet-picker-empty">${escapeHtml(t('chat.cheatsheet_picker_empty'))}</div>`;
        if (cheatsheetPickerSendBtn) cheatsheetPickerSendBtn.disabled = true;
        return;
    }

    cheatsheetPickerList.innerHTML = activeSheets.map((sheet) => {
        const previewText = String(sheet.content || '').replace(/\s+/g, ' ').trim();
        const preview = previewText.length > 180 ? previewText.slice(0, 177) + '...' : previewText;
        const checked = sheet.id === selectedCheatsheetId ? 'checked' : '';
        return `
            <label class="cheatsheet-picker-item">
                <input type="radio" name="chat-cheatsheet-choice" value="${escapeAttr(sheet.id || '')}" ${checked}>
                <div class="cheatsheet-picker-item-meta">
                    <div class="cheatsheet-picker-item-name">${escapeHtml(sheet.name || t('chat.cheatsheet_picker_unnamed'))}</div>
                    <div class="cheatsheet-picker-item-preview">${escapeHtml(preview || t('chat.cheatsheet_picker_no_content'))}</div>
                </div>
            </label>
        `;
    }).join('');

    cheatsheetPickerList.querySelectorAll('input[name="chat-cheatsheet-choice"]').forEach((input) => {
        input.addEventListener('change', () => {
            selectedCheatsheetId = input.value;
            if (cheatsheetPickerSendBtn) cheatsheetPickerSendBtn.disabled = !selectedCheatsheetId;
        });
    });
    if (cheatsheetPickerSendBtn) cheatsheetPickerSendBtn.disabled = !selectedCheatsheetId;
}

async function openCheatsheetPicker() {
    if (!cheatsheetPickerOverlay || !cheatsheetPickerList) return;
    closeComposerPanel();
    selectedCheatsheetId = '';
    cheatsheetPickerOverlay.classList.add('active');
    cheatsheetPickerList.innerHTML = `<div class="cheatsheet-picker-empty">${escapeHtml(t('chat.cheatsheet_picker_loading'))}</div>`;
    if (cheatsheetPickerSendBtn) cheatsheetPickerSendBtn.disabled = true;

    try {
        const res = await fetch('/api/cheatsheets?active=true');
        if (!res.ok) throw new Error(res.statusText || 'Failed to fetch cheatsheets');
        const data = await res.json();
        cheatsheetPickerItems = Array.isArray(data) ? data : [];
        renderCheatsheetPickerList();
    } catch (_error) {
        cheatsheetPickerItems = [];
        cheatsheetPickerList.innerHTML = `<div class="cheatsheet-picker-empty">${escapeHtml(t('chat.cheatsheet_picker_error'))}</div>`;
    }
}

function buildCheatsheetAgentMessage(sheet) {
    const title = sheet?.name || t('chat.cheatsheet_picker_unnamed');
    const content = String(sheet?.content || '').trim();
    return `${t('chat.cheatsheet_picker_prompt_prefix')} "${title}"\n\n<cheatsheet name="${title}">\n${content}\n</cheatsheet>\n\n${t('chat.cheatsheet_picker_prompt_suffix')}`;
}

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
let seenSSEDocuments = new Set();
let currentPlanState = null;

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
document.getElementById('debug-pill').addEventListener('click', () => {
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
});

/* ── Speaker mode (TTS auto-play) ── */
let speakerMode = localStorage.getItem('aurago-speaker') === 'true';
const _audioQueue = [];
let _audioPlaying = false;

function updateSpeakerButton() {
    const btn = document.getElementById('speaker-toggle');
    if (!btn) return;
    btn.textContent = speakerMode ? '🔊' : '🔇';
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

document.getElementById('speaker-toggle').addEventListener('click', () => {
    speakerMode = !speakerMode;
    localStorage.setItem('aurago-speaker', speakerMode);
    updateSpeakerButton();
    // Notify backend so prompt hint can be adjusted
    fetch('/api/preferences', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ speaker_mode: speakerMode })
    }).catch(() => { });
});

function _playNextInQueue() {
    if (_audioQueue.length === 0) { _audioPlaying = false; return; }
    _audioPlaying = true;
    const src = _audioQueue.shift();
    const audio = new Audio(src);
    audio.addEventListener('ended', _playNextInQueue);
    audio.addEventListener('error', _playNextInQueue);
    audio.play().catch(_playNextInQueue);
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
    const icon = status === 'completed' ? '✅'
        : status === 'in_progress' ? '⟳'
        : status === 'blocked' ? '⛔'
        : status === 'failed' ? '⚠️'
        : status === 'skipped' ? '⏭'
        : '⬜';
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
        ? `<div class="todo-item todo-pending">🎯 ${escapeHtml(plan.current_task)}</div>`
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
            <div class="todo-item todo-pending">📈 ${progressLabel}: ${progress}% (${counts.completed || 0}/${counts.total || tasks.length || 0})</div>
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
                + (done ? '✅' : '⬜') + ' ' + escapeHtml(text) + '</div>';
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
async function changePersonality(newId, triggerSelect) {
    try {
        const res = await fetch('/api/personality', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: newId })
        });
        if (res.ok) {
            const sel = document.getElementById('personality-select');
            if (sel) sel.value = newId;
            // Visual feedback: brief flash on the triggering select
            if (triggerSelect) {
                triggerSelect.style.borderColor = '#22c55e';
                triggerSelect.style.boxShadow = '0 0 6px rgba(34,197,94,0.4)';
                setTimeout(() => {
                    triggerSelect.style.borderColor = '';
                    triggerSelect.style.boxShadow = '';
                }, 800);
            }
            console.log("Personality updated to:", newId);
        } else {
            console.error("Failed to update personality:", res.status);
            if (triggerSelect) {
                triggerSelect.style.borderColor = '#ef4444';
                setTimeout(() => { triggerSelect.style.borderColor = ''; }, 800);
            }
        }
    } catch (err) {
        console.error("Error updating personality:", err);
    }
}

document.getElementById('personality-select').addEventListener('change', function (e) {
    changePersonality(e.target.value, e.target);
});

/* ── Clear session ── */
document.getElementById('clear-btn').addEventListener('click', async () => {
    closeComposerPanel();
    const ok = await showConfirm(
        t('chat.confirm_clear_title'),
        t('chat.confirm_clear_msg')
    );
    if (ok) {
        try {
            const res = await fetch(buildClearUrl(), { method: 'DELETE' });
            if (res.ok) {
                chatContent.innerHTML = '';
                conversation = [];
                hideTodoPanel();
                appendMessage('assistant', t('chat.greeting'));
            }
        } catch (err) {
            console.error("Failed to clear session:", err);
        }
    }
});

/* ── Stop agent ── */
document.getElementById('stop-btn').addEventListener('click', async () => {
    closeComposerPanel();
    const ok = await showConfirm(
        t('chat.confirm_stop_title'),
        t('chat.confirm_stop_msg')
    );
    if (ok) {
        try {
            const sid = typeof getActiveSessionId === 'function' ? getActiveSessionId() : 'default';
            const res = await fetch('/api/admin/stop', {
                method: 'POST',
                headers: { 'X-Session-ID': sid }
            });
            if (res.ok) {
                await showAlert(
                    t('chat.alert_stopped_title'),
                    t('chat.alert_stopped_msg')
                );
            }
        } catch (err) {
            console.error("Failed to stop agent:", err);
        }
    }
});

/* ── File Upload ── */
const fileInput = document.getElementById('file-input');
const uploadBtn = document.getElementById('upload-btn');
const attachChip = document.getElementById('attachment-chip');
const attachName = document.getElementById('attachment-name');
const attachClear = document.getElementById('attachment-clear');
const attachmentsPanel = document.getElementById('attachments-panel');
let pendingAttachments = []; // [{ path, filename, localUrl, kind }]

function _attachmentKindFromFile(file) {
    const t = (file && file.type) ? file.type : '';
    if (t.startsWith('image/')) return 'image';
    if (t.startsWith('audio/')) return 'audio';
    if (t.startsWith('video/')) return 'video';
    return 'file';
}

function _makeAttachmentLabel() {
    if (!pendingAttachments.length) return '';
    const first = pendingAttachments[0]?.filename || t('chat.file_sent');
    if (pendingAttachments.length === 1) return first;
    return `${first} (+${pendingAttachments.length - 1})`;
}

function _revokeAttachmentURLs() {
    pendingAttachments.forEach(a => {
        try { if (a.localUrl) URL.revokeObjectURL(a.localUrl); } catch (_e) { }
    });
}

function clearPendingAttachments() {
    _revokeAttachmentURLs();
    pendingAttachments = [];
    if (fileInput) fileInput.value = '';
    chatSetHidden(attachChip, true);
    chatSetHidden(attachmentsPanel, true);
    uploadBtn.classList.remove('has-file');
    if (attachmentsPanel) attachmentsPanel.innerHTML = '';
}

function renderPendingAttachments() {
    if (!pendingAttachments.length) {
        chatSetHidden(attachChip, true);
        chatSetHidden(attachmentsPanel, true);
        uploadBtn.classList.remove('has-file');
        if (attachmentsPanel) attachmentsPanel.innerHTML = '';
        return;
    }

    attachName.textContent = _makeAttachmentLabel();
    chatSetHidden(attachChip, false);
    uploadBtn.classList.add('has-file');

    if (!attachmentsPanel) return;
    chatSetHidden(attachmentsPanel, false);

    const itemsHTML = pendingAttachments.map((a, idx) => {
        const safeName = escapeHtml(a.filename || '');
        const safePath = escapeHtml(a.path || '');
        const removeTitle = escapeAttr(t('chat.attachment_clear_title') || 'Remove');
        let thumb = `<div class="attachment-thumb"></div>`;
        if (a.kind === 'image' && a.localUrl) {
            thumb = `<img class="attachment-thumb" src="${escapeAttr(a.localUrl)}" alt="${safeName}">`;
        } else if (a.kind === 'audio' && a.localUrl) {
            thumb = `<audio class="attachment-thumb" src="${escapeAttr(a.localUrl)}" controls></audio>`;
        } else if (a.kind === 'video' && a.localUrl) {
            thumb = `<video class="attachment-thumb" src="${escapeAttr(a.localUrl)}" controls muted></video>`;
        }
        return `
            <div class="attachment-item" data-idx="${idx}">
                ${thumb}
                <div class="attachment-meta">
                    <div class="attachment-filename" title="${safeName}">${safeName}</div>
                    <div class="attachment-path" title="${safePath}">${safePath}</div>
                </div>
                <button type="button" class="attachment-remove" data-idx="${idx}" title="${removeTitle}">✕</button>
            </div>
        `;
    }).join('');

    attachmentsPanel.innerHTML = `<div class="attachments-grid">${itemsHTML}</div>`;
    attachmentsPanel.querySelectorAll('.attachment-remove').forEach(btn => {
        btn.addEventListener('click', () => {
            const idx = Number(btn.dataset.idx);
            const item = pendingAttachments[idx];
            if (item && item.localUrl) {
                try { URL.revokeObjectURL(item.localUrl); } catch (_e) { }
            }
            pendingAttachments = pendingAttachments.filter((_, i) => i !== idx);
            renderPendingAttachments();
        });
    });
}

uploadBtn.addEventListener('click', () => {
    closeComposerPanel();
    fileInput.click();
});

if (cheatsheetPickerBtn) {
    cheatsheetPickerBtn.addEventListener('click', () => {
        openCheatsheetPicker();
    });
}

if (cheatsheetPickerCancelBtn) {
    cheatsheetPickerCancelBtn.addEventListener('click', closeCheatsheetPicker);
}
if (cheatsheetPickerCloseXBtn) {
    cheatsheetPickerCloseXBtn.addEventListener('click', closeCheatsheetPicker);
}
if (cheatsheetPickerOverlay) {
    cheatsheetPickerOverlay.addEventListener('click', (event) => {
        if (event.target === cheatsheetPickerOverlay) {
            closeCheatsheetPicker();
        }
    });
}
if (cheatsheetPickerSendBtn) {
    cheatsheetPickerSendBtn.addEventListener('click', async () => {
        if (!selectedCheatsheetId) return;
        const selectedSheet = cheatsheetPickerItems.find((sheet) => sheet && sheet.id === selectedCheatsheetId);
        if (!selectedSheet) return;
        closeCheatsheetPicker();
        const messageForAgent = buildCheatsheetAgentMessage(selectedSheet);
        const visibleMessage = `${t('chat.cheatsheet_picker_sent_prefix')} ${selectedSheet.name || t('chat.cheatsheet_picker_unnamed')}`;
        await handleOutgoingMessage(messageForAgent, visibleMessage);
    });
}

attachClear.addEventListener('click', () => {
    clearPendingAttachments();
});

if (composerMoreBtn && composerPanel) {
    composerMoreBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        closeMoodFeedbackRow();
        toggleComposerPanel();
    });
    document.addEventListener('click', (e) => {
        if (composerPanel.classList.contains('is-hidden')) return;
        if (e.target.closest('#composer-panel') || e.target.closest('#composer-more-btn')) return;
        closeComposerPanel();
    });
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            closeComposerPanel();
            closeMoodFeedbackRow();
            closeCheatsheetPicker();
        }
    });
}

/* ── Desktop: auto-open composer panel & handle resize ── */
if (composerPanel) {
    function applyDesktopComposerState() {
        if (isDesktopView()) {
            /* Ensure panel is visible on desktop */
            composerPanel.classList.remove('is-hidden');
        } else {
            /* On mobile, start with panel hidden unless already open */
            if (composerMoreBtn && !composerMoreBtn.classList.contains('is-open')) {
                composerPanel.classList.add('is-hidden');
            }
        }
    }
    /* Apply on load */
    applyDesktopComposerState();
    /* React to viewport changes (e.g. resize, orientation change) */
    _desktopMQ.addEventListener('change', applyDesktopComposerState);
}

if (feedbackToggleBtn && moodFeedbackRow) {
    feedbackToggleBtn.addEventListener('click', () => {
        const willOpen = moodFeedbackRow.classList.contains('is-hidden');
        chatSetHidden(moodFeedbackRow, !willOpen);
        closeComposerPanel();
    });
    document.addEventListener('click', (e) => {
        if (moodFeedbackRow.classList.contains('is-hidden')) return;
        if (e.target.closest('#mood-feedback-row') || e.target.closest('#feedback-toggle-btn')) return;
        closeMoodFeedbackRow();
    });
}

async function uploadSingleAttachment(file) {
    const formData = new FormData();
    formData.append('file', file);
    const res = await fetch('/api/upload', { method: 'POST', body: formData });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    const kind = _attachmentKindFromFile(file);
    const localUrl = (kind !== 'file') ? URL.createObjectURL(file) : null;
    pendingAttachments.push({ path: data.path, filename: data.filename, localUrl, kind });
    renderPendingAttachments();
}

fileInput.addEventListener('change', async () => {
    const files = Array.from(fileInput.files || []);
    if (!files.length) return;
    uploadBtn.disabled = true;
    try {
        for (const file of files) {
            await uploadSingleAttachment(file);
        }
    } catch (err) {
        console.error('Upload error:', err);
        appendMessage('assistant', t('chat.upload_failed') + err.message);
    } finally {
        uploadBtn.disabled = false;
        fileInput.value = ''; // allow re-upload of same file(s)
    }
});

async function handleOutgoingMessage(inputMessage, displayMessageOverride = '') {
    closeComposerPanel();
    closeMoodFeedbackRow();
    let message = String(inputMessage || '').trim();
    if (!message && !pendingAttachments.length) return;
    const hasTypedInput = message.length > 0;
    if (!message) {
        message = t('chat.file_sent');
    }

    // Append attachment info for the agent
    let agentMessage = message;
    if (pendingAttachments.length) {
        for (const a of pendingAttachments) {
            if (!a || !a.path) continue;
            agentMessage += '\n\n' + t('chat.file_attached_prefix') + a.path;
        }
        agentMessage += '\n' + t('chat.file_attached_instructions');
        clearPendingAttachments();
    }

    // Show only the user's typed message in chat (not the agent path)
    // For /sudopwd, mask the password argument to avoid showing it in chat
    let displayMessage = displayMessageOverride || message;
    if (!displayMessageOverride && hasTypedInput) {
        displayMessage = /^\/sudopwd\s+\S/i.test(message)
            ? '/sudopwd ***'
            : message;
    }
    appendMessage('user', displayMessage);
    userInput.value = '';
    userInput.style.height = ''; // Reset height to CSS default (min-height from stylesheet)

    // Use agentMessage (with path) for the API call below
    const _origMessage = message; // keep ref
    message = agentMessage;

    /* ── /debug command intercept ── */
    /* Toggle local UI display state, then let the message propagate to the server
       so the agent's system prompt debug mode is also updated there. */
    if (/^\/debug(\s+(on|off))?$/i.test(userInput.value || _origMessage)) {
        const arg = _origMessage.split(/\s+/)[1];
        debugMode = arg ? (arg.toLowerCase() === 'on') : !debugMode;
        localStorage.setItem('aurago-debug', debugMode);
        updateDebugPill();
        // Fall through — do NOT return; the server command handler responds
    }

    userInput.disabled = true;
    sendBtn.disabled = true;
    stopBtn.disabled = false;

    conversation.push({ role: 'user', content: message });

    /* Show status bar immediately for instant feedback */
    agentStatusText.textContent = t('chat.sse_thinking');
    chatSetHidden(agentStatusDiv, false);

    // Reset per-request response tracking for the SSE fallback
    _httpResponseRendered = false;

    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 10 * 60 * 1000); // 10 min
        let response;
        try {
            const sessionHeaders = { 'Content-Type': 'application/json' };
            const sid = getActiveSessionId();
            if (sid && sid !== 'default') {
                sessionHeaders['X-Session-ID'] = sid;
            }
            response = await fetch('/v1/chat/completions', {
                method: 'POST',
                headers: sessionHeaders,
                body: JSON.stringify({
                    model: 'aurago',
                    messages: [{ role: 'user', content: message }]
                }),
                signal: controller.signal
            });
        } finally {
            clearTimeout(timeoutId);
        }

        if (!response.ok) throw new Error('API Error: ' + response.statusText);

        const data = await response.json();
        const assistantMessage = data.choices[0].message;

        appendMessage('assistant', assistantMessage.content);
        _httpResponseRendered = true;
        seenSSEImages.clear(); // reset after final response is rendered
        seenSSEAudios.clear();
        seenSSEDocuments.clear();
        conversation.push(assistantMessage);
        // Cap to last 200 messages to prevent unbounded memory growth
        if (conversation.length > 200) { conversation = conversation.slice(-200); }

    } catch (error) {
        if (error.name === 'AbortError') {
            appendMessage('assistant', t('chat.error_timeout'));
        } else if (error instanceof TypeError && window.AuraSSE && window.AuraSSE.isConnected()) {
            // Network-level connection drop while SSE is still alive — the agent is
            // still running on the backend.  Suppress the error message and keep the
            // status bar visible; the SSE 'done' event will recover the response.
            _fetchConnectionLost = true;
        } else {
            // Try to recover the response from /history before showing an error.
            // This handles the case where the HTTP connection was lost during a
            // long agent run but the agent did complete and persisted the answer.
            await tryRecoverFromHistory();
            if (!_httpResponseRendered) {
                appendMessage('assistant', t('chat.error_connection') + error.message);
            }
        }
    } finally {
        userInput.disabled = false;
        sendBtn.disabled = false;
        userInput.focus();
        if (!_fetchConnectionLost) {
            chatSetHidden(document.getElementById('agentStatusContainer'), true);
            stopBtn.disabled = true;
        }
    }
}

/* ── Form submit ── */
chatForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    await handleOutgoingMessage(userInput.value.trim());
});

/* ── Image lightbox ── */

const lightbox = document.getElementById('img-lightbox');
const lightboxImg = document.getElementById('img-lightbox-img');
const lightboxClose = document.getElementById('img-lightbox-close');

chatContent.addEventListener('click', (e) => {
    if (e.target.tagName === 'IMG' && e.target.closest('.bubble')) {
        if (lightboxImg) lightboxImg.src = e.target.src;
        if (lightbox) lightbox.classList.add('open');
    }
});

if (lightbox) lightbox.addEventListener('click', () => lightbox.classList.remove('open'));
if (lightboxClose) lightboxClose.addEventListener('click', () => lightbox && lightbox.classList.remove('open'));
document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && lightbox) lightbox.classList.remove('open'); });

/* ── Budget pill ── */
function updateBudgetPills(b) {
    if (!b || !b.enabled) return;
    const cost = (b.spent_usd || b.total_cost_usd || 0).toFixed(3);
    const limit = (b.daily_limit_usd || 0).toFixed(2);
    const bp = document.getElementById('budgetPill');
    if (bp) {
        chatSetHidden(bp, false);
        bp.textContent = t('chat.budget_pill_format', { cost: cost, limit: limit });
        bp.title = t('chat.budget_tooltip_template', { cost: cost, limit: limit, enforcement: b.enforcement });
        bp.classList.remove('budget-warning', 'budget-exceeded');
        if (b.is_exceeded || b.exceeded) bp.classList.add('budget-exceeded');
        else if (b.is_warning || b.warning_sent) bp.classList.add('budget-warning');
    }
}

document.getElementById('budgetPill').addEventListener('click', () => {
    fetch('/api/budget').then(r => r.json()).then(updateBudgetPills).catch(() => { });
});
// Fetch initial budget status on page load
fetch('/api/budget').then(r => r.json()).then(updateBudgetPills).catch(() => { });

/* ── OpenRouter Credits Pill ── */
function updateCreditsPills(c) {
    if (!c || !c.available) return;
    if (c.error) return;
    const el = document.getElementById('creditsPill');
    if (el) {
        chatSetHidden(el, false);
        if (c.limit > 0) {
            el.textContent = t('chat.credits_pill_text', { amount: c.balance.toFixed(2) });
            el.title = t('chat.credits_tooltip_used_limit', { usage: c.usage.toFixed(2), limit: c.limit.toFixed(2) });
            el.classList.remove('budget-warning', 'budget-exceeded');
            const pct = c.usage / c.limit;
            if (pct >= 1.0) el.classList.add('budget-exceeded');
            else if (pct >= 0.8) el.classList.add('budget-warning');
        } else {
            el.textContent = t('chat.credits_pill_text', { amount: c.usage.toFixed(2) });
            el.title = t('chat.credits_tooltip_payg', { usage: c.usage.toFixed(2) });
        }
    }
}
document.getElementById('creditsPill').addEventListener('click', () => {
    fetch('/api/credits').then(r => r.json()).then(updateCreditsPills).catch(() => { });
});
fetch('/api/credits').then(r => r.json()).then(updateCreditsPills).catch(() => { });

/* ── Mood Widget ── */
const moodStateEmojiMap = {
    curious: '🔍', focused: '🎯', creative: '🎨',
    analytical: '📊', cautious: '🛡️', playful: '🎮'
};
const moodNameKeys = {
    curious: 'chat.mood_curious', focused: 'chat.mood_focused', creative: 'chat.mood_creative',
    analytical: 'chat.mood_analytical', cautious: 'chat.mood_cautious', playful: 'chat.mood_playful'
};
const traitOrder = ['curiosity', 'thoroughness', 'creativity', 'empathy', 'confidence', 'affinity', 'loneliness'];

function summarizeMoodEmotion(text, maxLen = 140) {
    if (!text) return '';
    const normalized = String(text)
        .replace(/<(thinking|think)>[\s\S]*?<\/\1>/gi, ' ')
        .replace(/\s+/g, ' ')
        .trim();
    if (normalized.length <= maxLen) return normalized;
    return normalized.slice(0, maxLen).trimEnd() + '…';
}

function updateMoodWidget(data) {
    if (!data || !data.enabled) return;
    const toggle = document.getElementById('moodToggle');
    const emotionEl = document.getElementById('moodEmotion');
    const emoji = moodStateEmojiMap[data.mood] || '🧠';
    const moodLabel = t(moodNameKeys[data.mood] || 'chat.mood_default_text');
    document.getElementById('moodEmoji').textContent = emoji;
    document.getElementById('moodText').textContent = moodLabel;
    document.getElementById('moodPanelEmoji').textContent = emoji;
    document.getElementById('moodPanelLabel').textContent = moodLabel;
    if (emotionEl) {
        if (data.current_emotion) {
            emotionEl.textContent = summarizeMoodEmotion(data.current_emotion);
            chatSetHidden(emotionEl, false);
        } else {
            emotionEl.textContent = '';
            chatSetHidden(emotionEl, true);
        }
    }
    const traitsEl = document.getElementById('moodTraits');
    traitsEl.innerHTML = '';
    traitOrder.forEach(tr => {
        const v = (data.traits && data.traits[tr] != null) ? data.traits[tr] : 0.5;
        const pct = Math.round(v * 100);
        const row = document.createElement('div');
        row.className = 'trait-row';
        row.innerHTML = `<span class="trait-label">${t('chat.trait_' + tr)}</span>` +
            `<div class="trait-bar-bg"><div class="trait-bar-fill" data-trait="${tr}" data-percent="${pct}"></div></div>` +
            `<span class="trait-value">${v.toFixed(2)}</span>`;
        const barFill = row.querySelector('.trait-bar-fill');
        if (barFill) barFill.classList.add('w-pct-' + (barFill.dataset.percent || 0));
        traitsEl.appendChild(row);
    });
    // Show the mood toggle - CSS has display:none by default, so we need to override it
    if (toggle) {
        toggle.style.display = 'flex';
        chatSetHidden(toggle, false);
    }
}

if (PERSONALITY_ENABLED) {
    fetch('/api/personality/state').then(r => r.json()).then(updateMoodWidget).catch(() => { });
    // Live mood updates via SSE — no more 30s polling.
    window.AuraSSE.on('personality_update', function (payload) {
        updateMoodWidget(payload);
    });

    document.getElementById('moodToggle').addEventListener('click', (e) => {
        e.stopPropagation();
        document.getElementById('moodPanel').classList.toggle('open');
        document.getElementById('moodToggle').classList.toggle('open');
    });
    document.addEventListener('click', (e) => {
        const panel = document.getElementById('moodPanel');
        if (panel.classList.contains('open') && !e.target.closest('.mood-widget')) {
            panel.classList.remove('open');
            document.getElementById('moodToggle').classList.remove('open');
        }
    });
}

/* ── Push Notification Bell Button ── */
const PUSH_MUTED_KEY = 'aurago-push-muted';

function initPushUI() {
    const btn = document.getElementById('push-btn');
    if (!btn) return;

    function applyState() {
        btn.classList.remove('push-granted', 'push-denied', 'push-unavailable', 'push-muted');
        if (!window.getPushStatus) {
            // PWA init may still be in progress; keep button inactive but not permanently disabled
            btn.classList.add('push-unavailable');
            btn.title = t('pwa.btn_push_title');
            btn.disabled = true;
            return false;
        }
        const { available, permission } = window.getPushStatus();
        if (!available) {
            btn.classList.add('push-unavailable');
            btn.title = t('pwa.notifications_unavailable');
            btn.disabled = true;
        } else if (permission === 'granted') {
            const muted = localStorage.getItem(PUSH_MUTED_KEY) === '1';
            if (muted) {
                btn.classList.add('push-muted');
                btn.title = t('pwa.notifications_disabled');
            } else {
                btn.classList.add('push-granted');
                btn.title = t('pwa.notifications_enabled');
            }
            btn.disabled = false;
        } else if (permission === 'denied') {
            btn.classList.add('push-denied');
            btn.title = t('pwa.notifications_denied');
            btn.disabled = false;
        } else {
            btn.title = t('pwa.btn_push_title');
            btn.disabled = false;
        }
        return true;
    }

    // PWA init is async in shared.js; poll briefly until getPushStatus is ready
    if (!applyState()) {
        let attempts = 0;
        const timer = setInterval(() => {
            attempts++;
            if (applyState() || attempts > 30) {
                clearInterval(timer);
                if (attempts > 30 && !window.getPushStatus) {
                    btn.classList.add('push-unavailable');
                    btn.title = t('pwa.notifications_unavailable');
                    btn.disabled = true;
                }
            }
        }, 100);
    }

    // Re-evaluate once the Service Worker has finished registering
    // (initPWA is async and may complete after DOMContentLoaded)
    window.addEventListener('pwa-ready', () => applyState(), { once: true });

    btn.addEventListener('click', async () => {
        closeComposerPanel();
        const status = window.getPushStatus ? window.getPushStatus() : null;
        if (!status || !status.available) return;

        if (status.permission === 'denied') {
            if (window.showToast) {
                window.showToast(t('pwa.notifications_denied'), 'warning');
            } else {
                await showAlert(t('pwa.notifications_denied'), '');
            }
            return;
        }

        if (status.permission === 'granted') {
            const muted = localStorage.getItem(PUSH_MUTED_KEY) === '1';
            if (muted) {
                localStorage.removeItem(PUSH_MUTED_KEY);
                if (window.requestPushPermission) await window.requestPushPermission();
                applyState();
                window.showToast ? window.showToast(t('pwa.notifications_enabled'), 'success') : null;
            } else {
                localStorage.setItem(PUSH_MUTED_KEY, '1');
                if (window.revokePushPermission) await window.revokePushPermission();
                applyState();
                window.showToast ? window.showToast(t('pwa.notifications_disabled'), 'info') : null;
            }
            return;
        }

        // Default — request permission
        const result = await window.requestPushPermission();
        applyState();
        if (result && result.success) {
            window.showToast ? window.showToast(t('pwa.notifications_enabled'), 'success') : null;
        } else if (result && result.reason === 'denied') {
            window.showToast ? window.showToast(t('pwa.notifications_denied'), 'warning') : null;
        }
    });
}

/* ── Chat Theme Picker ── */
const THEME_ICONS = {
    'dark': '◐',
    'light': '☀',
    'retro-crt': '▣',
    'cyberwar': '✦',
    'lollipop': '✿',
    'dark-sun': '☼',
    'ocean': '◌'
};

function initChatThemePicker() {
    const picker = document.getElementById('chat-theme-picker');
    const btn = document.getElementById('chat-theme-btn');
    const dropdown = document.getElementById('chat-theme-dropdown');
    const icon = document.getElementById('chat-theme-icon');
    if (!picker || !btn || !dropdown || !icon) return;
    if (picker.dataset.initialized === 'true') return;
    picker.dataset.initialized = 'true';

    function _refreshIcon(theme) {
        icon.textContent = THEME_ICONS[theme] || '◐';
    }

    function _selectOption(theme) {
        setChatTheme(theme);
        _refreshIcon(theme);
        dropdown.hidden = true;
        btn.setAttribute('aria-expanded', 'false');
        // Mark active option
        dropdown.querySelectorAll('.chat-theme-option').forEach(opt => {
            opt.classList.toggle('active', opt.dataset.theme === theme);
        });
    }

    btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const isOpen = !dropdown.hidden;
        dropdown.hidden = isOpen;
        btn.setAttribute('aria-expanded', String(!isOpen));
    });

    dropdown.querySelectorAll('.chat-theme-option').forEach(opt => {
        opt.addEventListener('click', () => {
            const theme = opt.dataset.theme;
            if (theme) _selectOption(theme);
        });
    });

    // Close on outside click
    document.addEventListener('click', (e) => {
        if (!picker.contains(e.target)) {
            dropdown.hidden = true;
            btn.setAttribute('aria-expanded', 'false');
        }
    });

    // Escape key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && !dropdown.hidden) {
            dropdown.hidden = true;
            btn.setAttribute('aria-expanded', 'false');
            btn.focus();
        }
    });

    // Sync icon with current theme on init
    _refreshIcon(getCurrentChatTheme());
    _selectOption(getCurrentChatTheme());

    // React to external theme changes (e.g. aurago:themechange)
    window.addEventListener('aurago:themechange', (e) => {
        const theme = e.detail && e.detail.theme;
        if (theme) {
            _refreshIcon(theme);
            dropdown.querySelectorAll('.chat-theme-option').forEach(opt => {
                opt.classList.toggle('active', opt.dataset.theme === theme);
            });
        }
    });
}

/* ── Initialize Chat Modules ── */
document.addEventListener('DOMContentLoaded', () => {
    // Smart Scroller
    if (window.SmartScroller) {
        window.SmartScroller.init(document.getElementById('chat-box'));
    }

    // Chat Theme Picker
    initChatThemePicker();
    
    // Code Blocks
    if (window.CodeBlocks) {
        window.CodeBlocks.init();
    }
    
    // Voice / Speech-to-Text
    const voiceBtn = document.getElementById('voice-btn');
    if (voiceBtn) {
        const isSecure = window.location.protocol === 'https:' || 
                        window.location.hostname === 'localhost' || 
                        window.location.hostname === '127.0.0.1';
        
        if (!isSecure) {
            voiceBtn.disabled = true;
            voiceBtn.classList.add('btn-disabled');
            voiceBtn.title = 'Voice recording requires HTTPS connection';
        } else {
            const _populateInput = (text) => {
                const input = document.getElementById('user-input');
                input.value = text;
                input.style.height = 'auto';
                input.style.height = Math.min(input.scrollHeight, 200) + 'px';
                input.focus();
            };
            const _showError = async (msg) => {
                if (window.showToast) { window.showToast(msg, 'error'); } else { await showAlert(msg, ''); }
            };

            // Prefer browser-native Speech-to-Text (Chrome, Edge, Android)
            const useBrowserSTT = window.SpeechToText && window.SpeechToText.isSupported;

            if (useBrowserSTT) {
                window.SpeechToText.init({
                    // Don't touch textarea during live recognition —
                    // the overlay displays the streaming transcript.
                    // Only populate the textarea when STT finishes.
                    onInterimResult: () => {},
                    onFinalResult: () => {},
                    onEnd: (text) => {
                        voiceBtn.classList.remove('btn-active');
                        if (text) { _populateInput(text); }
                    },
                    onError: (msg) => {
                        voiceBtn.classList.remove('btn-active');
                        _showError(msg);
                    }
                });
            }

            // Always init VoiceRecorder as fallback
            if (window.VoiceRecorder) {
                window.VoiceRecorder.init({
                    onTranscription: _populateInput,
                    onError: _showError
                });
            }

            voiceBtn.addEventListener('click', () => {
                if (useBrowserSTT) {
                    if (window.SpeechToText.isActive) {
                        window.SpeechToText.stop();
                        voiceBtn.classList.remove('btn-active');
                    } else {
                        window.SpeechToText.start();
                        voiceBtn.classList.add('btn-active');
                    }
                } else if (window.VoiceRecorder) {
                    if (window.VoiceRecorder.isRecording) {
                        window.VoiceRecorder.send();
                    } else {
                        window.VoiceRecorder.start();
                    }
                }
            });
        }
    }
    
    // Drag & Drop
    if (window.DragDrop) {
        window.DragDrop.init({
            container: document.getElementById('chat-box'),
            onUpload: (data) => {
                const file = data.file || null;
                const kind = _attachmentKindFromFile(file);
                const localUrl = (file && kind !== 'file') ? URL.createObjectURL(file) : null;
                pendingAttachments.push({ path: data.path, filename: data.filename, localUrl, kind });
                renderPendingAttachments();
            },
            onError: (msg) => {
                if (window.showToast) {
                    window.showToast(msg, 'error');
                }
            }
        });
    }
    
    // Mermaid Loader
    if (window.MermaidLoader) {
        window.MermaidLoader.init();
    }

    // Push Notification Bell Button
    initPushUI();
});
