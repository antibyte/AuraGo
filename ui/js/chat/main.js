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
    document.title = t('chat.page_title');
    /* Header pills & controls */
    document.getElementById('theme-toggle').title = t('common.toggle_theme');
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
    /* Lightbox */
    const lbc = document.getElementById('img-lightbox-close');
    if (lbc) lbc.title = t('chat.lightbox_close_title');
}

function chatSetHidden(el, hidden) {
    if (!el) return;
    el.classList.toggle('is-hidden', hidden);
}

function closeComposerPanel() {
    if (!composerMoreBtn || !composerPanel) return;
    composerPanel.classList.add('is-hidden');
    composerMoreBtn.classList.remove('is-open');
    composerMoreBtn.setAttribute('aria-expanded', 'false');
}

function closeMoodFeedbackRow() {
    chatSetHidden(moodFeedbackRow, true);
}

function toggleComposerPanel(forceOpen) {
    if (!composerMoreBtn || !composerPanel) return;
    const shouldOpen = typeof forceOpen === 'boolean' ? forceOpen : composerPanel.classList.contains('is-hidden');
    composerPanel.classList.toggle('is-hidden', !shouldOpen);
    composerMoreBtn.classList.toggle('is-open', shouldOpen);
    composerMoreBtn.setAttribute('aria-expanded', shouldOpen ? 'true' : 'false');
}


/* ── Modal (replaces confirm / alert) ── */
function showModal(title, message, isConfirm) {
    return new Promise(resolve => {
        const overlay = document.getElementById('modal-overlay');
        const titleEl = document.getElementById('modal-title');
        const msgEl = document.getElementById('modal-message');
        const confirmBtn = document.getElementById('modal-confirm');
        const cancelBtn = document.getElementById('modal-cancel');

        titleEl.textContent = title;
        msgEl.textContent = message;
        chatSetHidden(cancelBtn, !isConfirm);
        overlay.classList.add('active');

        function cleanup(result) {
            overlay.classList.remove('active');
            confirmBtn.removeEventListener('click', onConfirm);
            cancelBtn.removeEventListener('click', onCancel);
            overlay.removeEventListener('click', onOverlay);
            document.removeEventListener('keydown', onKey);
            resolve(result);
        }

        function onConfirm() { cleanup(true); }
        function onCancel() { cleanup(false); }
        function onOverlay(e) { if (e.target === overlay) cleanup(false); }
        function onKey(e) {
            if (e.key === 'Escape') cleanup(false);
            if (e.key === 'Enter') cleanup(true);
        }

        confirmBtn.addEventListener('click', onConfirm);
        cancelBtn.addEventListener('click', onCancel);
        overlay.addEventListener('click', onOverlay);
        document.addEventListener('keydown', onKey);
    });
}

function showConfirm(title, msg) { return showModal(title, msg, true); }
function showAlert(title, msg) { return showModal(title, msg, false); }

/* ── Data ── */
let conversation = [];
// Tracks /files/ paths already rendered via SSE 'image' events
// so appendMessage() can skip the duplicate markdown image.
let seenSSEImages = new Set();
let seenSSEAudios = new Set();
let seenSSEDocuments = new Set();
let currentPlanState = null;

function nextAnimationFrame() {
    return new Promise((resolve) => window.requestAnimationFrame(resolve));
}

async function renderHistoryMessagesBatched(history) {
    if (!Array.isArray(history) || history.length === 0) return;
    const renderBatchSize = 20;
    for (let index = 0; index < history.length; index++) {
        const msg = history[index];
        if (!debugMode && isDebugOnlyHistoryMessage(msg)) {
            conversation.push(msg);
        } else if (msg.role === 'user' || msg.role === 'assistant') {
            // Skip tool output user messages in the visible transcript, but keep them
            // in local conversation state for debugging and parity with loaded history.
            if (msg.role === 'user' && msg.content &&
                (msg.content.startsWith('[Tool Output]') ||
                 msg.content.startsWith('Tool Output:'))) {
                conversation.push(msg);
            } else {
                appendMessage(msg.role, msg.content);
                conversation.push(msg);
            }
        }

        if ((index + 1) % renderBatchSize === 0) {
            await nextAnimationFrame();
        }
    }
}

/* ── Debug mode ── */
// If user has explicitly set a preference in localStorage, use it.
// Only fall back to server defaults if no preference has been saved yet.
const _storedDebug = localStorage.getItem('aurago-debug');
let debugMode = _storedDebug !== null ? (_storedDebug === 'true') : (SHOW_TOOL_RESULTS || AGENT_DEBUG_MODE);
function updateDebugPill() {
    const pill = document.getElementById('debug-pill');
    if (!pill) return;
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

function appendToolOutput(text, label) {
    if (!text || !debugMode) return;
    const greet = chatContent.querySelector('[data-greeting]');
    if (greet) greet.remove();
    const escaped = escapeHtml(text);
    const lbl = label || t('chat.tool_output_label');
    const row = document.createElement('div');
    row.className = 'tool-output-row';
    row.innerHTML = `
                <div class="avatar bot">⚙️</div>
                <div class="tool-output-block">
                    <details>
                        <summary>⚙️ ${lbl}</summary>
                        <div class="tool-output-content">${escaped}</div>
                    </details>
                </div>
            `;
    chatContent.appendChild(row);
    chatBox.scrollTop = chatBox.scrollHeight;
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

/* ── Load history & notifications ── */
async function initPage() {
    applyI18n();

    // Check auth status to show/hide logout button
    try {
        const authRes = await fetch('/api/auth/status');
        if (authRes.ok) {
            const authData = await authRes.json();
            if (authData.enabled) {
                const lb = document.getElementById('logout-btn');
                const rl = document.getElementById('radialLogout');
                chatSetHidden(lb, false);
                chatSetHidden(rl, false);
            }
        }
    } catch (e) { /* auth endpoint unavailable, hide button */ }

    try {
        const res = await fetch('/history');
        if (res.ok) {
            const history = await res.json();
            if (history && history.length > 0) {
                chatContent.innerHTML = '';
                await renderHistoryMessagesBatched(history);
            }
        }
    } catch (err) {
        console.error("Failed to load history:", err);
    }

    try {
        const res = await fetch('/api/plans/active?session_id=default');
        if (res.ok) {
            const data = await res.json();
            if (data && data.plan) {
                updatePlanPanel(data.plan);
            }
        }
    } catch (err) {
        console.error('Failed to load active plan:', err);
    }

    try {
        const res = await fetch('/notifications');
        if (res.ok) {
            const notes = await res.json();
            if (notes && notes.length > 0) {
                const greet = chatContent.querySelector('[data-greeting]');
                if (greet) greet.remove();
                notes.forEach(note => {
                    appendMessage('assistant', t('chat.system_briefing_prefix') + note);
                });
                fetch('/notifications/read', { method: 'POST' });
            }
        }
    } catch (err) {
        console.error("Failed to load notifications:", err);
    }

    /* Load personalities */
    try {
        const res = await fetch('/api/personalities');
        if (res.ok) {
            const data = await res.json();
            const select = document.getElementById('personality-select');
            if (select) {
                select.innerHTML = '';
                data.personalities.forEach(p => {
                    const opt = document.createElement('option');
                    opt.value = p.name;
                    opt.textContent = p.name.charAt(0).toUpperCase() + p.name.slice(1);
                    if (p.name === data.active) opt.selected = true;
                    select.appendChild(opt);
                });
            }
        }
    } catch (err) {
        console.error("Failed to load personalities:", err);
    }
}

// Robust init: fire immediately if DOM already ready, else wait for event
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initPage);
} else {
    initPage();
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
            const res = await fetch('/clear', { method: 'DELETE' });
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
            const res = await fetch('/api/admin/stop', { method: 'POST' });
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

/* ── Append message ── */
function appendMessage(role, text) {
    if (!text || typeof text !== 'string') text = '';

    const greet = chatContent.querySelector('[data-greeting]');
    if (greet) greet.remove();

    const isUser = role === 'user';
    let isTechnical = false;

    let displayContent = text;
    if (!isUser) {
        // Pre-emptively strip to see if there's any conversational text left
        let strippedContent = displayContent
            .replace(/<tool_call>[\s\S]*?<\/tool_call>/g, '')
            .replace(/```(?:json)?\s*\{\s*"action"[\s\S]*?\}\s*```/g, '')
            .replace(/^```(?:json)?\n\{[\s\S]*?\}\n```$/gm, '')
            .replace(/^\{"action"\s*:[^\n]*\}\s*$/gm, '')  // inline single-line action JSON
            .replace(/^\[Tool Output\]\s*$/gm, '')           // [Tool Output] header lines
            .replace(/^Tool Output:.*$/gm, '')
            .trim();

        if (!strippedContent && (text.includes('Tool Output:') || text.includes('{"action":'))) {
            // Entire message was just a tool output or tool call with no other text
            isTechnical = true;
        } else {
            // If there's conversational text, we show the stripped version
            displayContent = strippedContent;
        }
    }

    displayContent = displayContent.trim();
    if (!displayContent) return;

    // Remove markdown images already shown live via SSE 'image' event
    if (!isTechnical && seenSSEImages.size > 0) {
        displayContent = displayContent.replace(/!\[[^\]]*\]\(([^)]+)\)/g, (match, url) =>
            seenSSEImages.has(url) ? '' : match
        ).trim();
        if (!displayContent) return; // nothing left to show
    }

    let finalHTML = displayContent;
    if (isTechnical) {
        finalHTML = `<pre>${escapeHtml(displayContent)}</pre>`;
    } else {
        try {
            if (typeof window.markdownit !== 'undefined') {
                const md = window.markdownit({
                    html: false,
                    breaks: true,
                    linkify: true,
                    highlight: function (str, lang) {
                        // Handle mermaid diagrams first
                        if (lang === 'mermaid') {
                            return `<div class="mermaid-raw">${escapeHtml(str)}</div>`;
                        }
                        
                        // Use enhanced code blocks for other languages
                        if (window.CodeBlocks) {
                            return window.CodeBlocks.createCodeBlock(str, lang);
                        }
                        
                        // Fallback to basic highlighting
                        if (lang && window.hljs && hljs.getLanguage(lang)) {
                            try {
                                return '<pre class="hljs"><code>' +
                                    hljs.highlight(str, { language: lang, ignoreIllegals: true }).value +
                                    '</code></pre>';
                            } catch (__) { }
                        }
                        return '<pre class="hljs"><code>' + escapeHtml(str) + '</code></pre>';
                    }
                });

                // Extract <thinking>/<think> blocks and replace with block-level placeholders
                // so markdown-it doesn't wrap them in <p> tags.
                const thinkingBlocks = [];
                const contentForRender = displayContent.replace(
                    /<(thinking|think)>([\s\S]*?)<\/\1>/gi,
                    (match, _tag, inner) => {
                        const idx = thinkingBlocks.length;
                        thinkingBlocks.push(inner.trim());
                        return `\n\n%%THINKING_BLOCK_${idx}%%\n\n`;
                    }
                );

                finalHTML = md.render(contentForRender);

                // Add target="_blank" to all links (external and internal)
                finalHTML = finalHTML.replace(/<a(\s+[^>]*)?\s+href="([^"]+)"/g, '<a$1href="$2" target="_blank" rel="noopener noreferrer"');

                // Replace placeholders with collapsible <details> elements
                thinkingBlocks.forEach((innerText, idx) => {
                    const innerHtml = md.render(innerText);
                    const label = (typeof t === 'function') ? t('chat.thinking_label') : 'Reasoning';
                    const detailsHtml = `<details class="thinking-block"><summary>🧠 ${label}</summary><div class="thinking-content">${innerHtml}</div></details>`;
                    // Replace whether it is wrapped in paragraph or not
                    finalHTML = finalHTML.replace(new RegExp(`<p>%%THINKING_BLOCK_${idx}%%</p>`, 'g'), detailsHtml);
                    finalHTML = finalHTML.replace(new RegExp(`%%THINKING_BLOCK_${idx}%%`, 'g'), detailsHtml);
                });

                finalHTML = sanitizeRenderedHTML(finalHTML);
            }
        } catch (e) {
            console.error("Markdown parsing failed:", e);
        }
    }

    const side = isUser ? 'user' : 'bot';
    const avatarIcon = isUser ? '🧑' : '🤖';
    const bubbleClass = isTechnical ? 'bubble bot technical' : `bubble ${side}`;

    const msgHTML = `
                <div class="msg-row ${side}">
                    <div class="avatar ${isUser ? 'human' : 'bot'}">${avatarIcon}</div>
                    <div class="${bubbleClass}">${finalHTML}</div>
                </div>
            `;
    chatContent.insertAdjacentHTML('beforeend', msgHTML);
    
    // Render mermaid diagrams if available
    if (window.MermaidLoader) {
        const newMessage = chatContent.lastElementChild;
        if (newMessage) {
            window.MermaidLoader.processBlocks(newMessage);
        }
    }
    
    // Use SmartScroller or fallback
    if (window.SmartScroller) {
        window.SmartScroller.onNewMessage();
    } else {
        chatBox.scrollTop = chatBox.scrollHeight;
    }
}

/* ── Helpers ── */
function isDebugOnlyHistoryMessage(msg) {
    if (!msg || typeof msg.content !== 'string') return false;
    const text = msg.content.trim();
    if (!text) return false;

    if (msg.role === 'user') {
        if (/^ERROR:\s+/i.test(text)) return true;
        if (/invalid function arguments json|raw JSON object ONLY|markdown fences|tool call/i.test(text)) return true;
        return false;
    }

    if (msg.role !== 'assistant' && msg.role !== 'system') return false;
    if (text === '[TOOL_CALL]') return true;
    if (/^\[TOOL_CALL\]/i.test(text)) return true;
    if (/<tool_call>/i.test(text)) return true;
    if (/^\{[\s\S]*"(action|tool_call|tool_name)"\s*:/i.test(text)) return true;
    if (/^(Tool Output:|\[Tool Output\])/i.test(text)) return true;

    // Legacy leaked orchestration/progress messages from pre-tool assistant turns.
    // Keep this conservative and only hide short operational updates, not normal answers.
    const lower = text.toLowerCase();
    const operationalHints = [
        'container', 'build', 'deploy', 'install', 'npm ', 'docker', 'script ',
        'command', 'logs', 'warte', 'wait', 'läuft', 'running', 'fertig',
        'copied', 'kopiert', 'ansatz', 'approach'
    ];
    if (text.length <= 240 && /[:：]\s*$/.test(text) && operationalHints.some(h => lower.includes(h))) {
        return true;
    }

    return false;
}

function escapeHtml(str) {
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function isSafeHref(url, allowRelative = true) {
    if (!url || typeof url !== 'string') return false;
    const trimmed = url.trim();
    if (!trimmed) return false;
    if (allowRelative && (trimmed.startsWith('/') || trimmed.startsWith('./') || trimmed.startsWith('../'))) {
        return true;
    }
    try {
        const parsed = new URL(trimmed, window.location.origin);
        return parsed.protocol === 'http:' || parsed.protocol === 'https:';
    } catch (_err) {
        return false;
    }
}

function sanitizeRenderedHTML(html) {
    const template = document.createElement('template');
    template.innerHTML = html;
    template.content.querySelectorAll('*').forEach((node) => {
        Array.from(node.attributes).forEach((attr) => {
            const name = attr.name.toLowerCase();
            if (name.startsWith('on')) {
                node.removeAttribute(attr.name);
                return;
            }
            if ((name === 'href' || name === 'src') && !isSafeHref(attr.value, true)) {
                node.removeAttribute(attr.name);
            }
        });
    });
    return template.innerHTML;
}

/* ── File Upload ── */
const fileInput = document.getElementById('file-input');
const uploadBtn = document.getElementById('upload-btn');
const attachChip = document.getElementById('attachment-chip');
const attachName = document.getElementById('attachment-name');
const attachClear = document.getElementById('attachment-clear');
let pendingAttachment = null; // { path, filename }

uploadBtn.addEventListener('click', () => {
    closeComposerPanel();
    fileInput.click();
});

attachClear.addEventListener('click', () => {
    pendingAttachment = null;
    fileInput.value = '';
    chatSetHidden(attachChip, true);
    uploadBtn.classList.remove('has-file');
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
        }
    });
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

fileInput.addEventListener('change', async () => {
    const file = fileInput.files[0];
    if (!file) return;
    const formData = new FormData();
    formData.append('file', file);
    uploadBtn.disabled = true;
    try {
        const res = await fetch('/api/upload', { method: 'POST', body: formData });
        if (!res.ok) throw new Error(await res.text());
        const data = await res.json();
        pendingAttachment = { path: data.path, filename: data.filename };
        attachName.textContent = data.filename;
        chatSetHidden(attachChip, false);
        uploadBtn.classList.add('has-file');
    } catch (err) {
        console.error('Upload error:', err);
        appendMessage('assistant', t('chat.upload_failed') + err.message);
    } finally {
        uploadBtn.disabled = false;
    }
});

/* ── Form submit ── */
chatForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    closeComposerPanel();
    closeMoodFeedbackRow();
    let message = userInput.value.trim();
    if (!message && !pendingAttachment) return;
    if (!message) message = t('chat.file_sent');

    // Append attachment info for the agent
    let agentMessage = message;
    if (pendingAttachment) {
        agentMessage += '\n\n' + t('chat.file_attached_prefix') + pendingAttachment.path +
            '\n' + t('chat.file_attached_instructions');
        pendingAttachment = null;
        fileInput.value = '';
        chatSetHidden(attachChip, true);
        uploadBtn.classList.remove('has-file');
    }

    // Show only the user's typed message in chat (not the agent path)
    // For /sudopwd, mask the password argument to avoid showing it in chat
    const displayMessage = /^\/sudopwd\s+\S/i.test(message)
        ? '/sudopwd ***'
        : message;
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

    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 10 * 60 * 1000); // 10 min
        let response;
        try {
            response = await fetch('/v1/chat/completions', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
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
        seenSSEImages.clear(); // reset after final response is rendered
        seenSSEAudios.clear();
        seenSSEDocuments.clear();
        conversation.push(assistantMessage);
        // Cap to last 200 messages to prevent unbounded memory growth
        if (conversation.length > 200) { conversation = conversation.slice(-200); }

    } catch (error) {
        if (error.name === 'AbortError') {
            appendMessage('assistant', t('chat.error_timeout'));
        } else {
            appendMessage('assistant', t('chat.error_connection') + error.message);
        }
    } finally {
        userInput.disabled = false;
        sendBtn.disabled = false;
        userInput.focus();
        chatSetHidden(document.getElementById('agentStatusContainer'), true);
        stopBtn.disabled = true;
    }
});

/* ── SSE (Server-Sent Events) ── */
const agentStatusDiv = document.getElementById('agentStatusContainer');
const agentStatusText = document.getElementById('agentStatusText');

/* ── Floating action icons ── */
const TOOL_ICONS = {
    execute_shell: '🖥️', execute_python: '🐍', execute_sandbox: '📦',
    filesystem: '📁', system_metrics: '📊', process_management: '⚙️',
    follow_up: '🔄', analyze_image: '🔍', transcribe_audio: '🎤',
    send_image: '🖼️', execute_skill: '🎯', list_skills: '📜',
    save_tool: '💾', remote_execution: '🌐', api_request: '🔗',
    manage_memory: '🧠', query_memory: '🧠', memory_reflect: '💭',
    cheatsheet: '📋', knowledge_graph: '🕸️', secrets_vault: '🔐',
    cron_scheduler: '⏰', manage_notes: '📝', manage_journal: '📓',
    manage_missions: '🎯', query_inventory: '📋', register_device: '📱',
    home_assistant: '🏠', meshcentral: '🖧', wake_on_lan: '⚡',
    docker: '🐳', co_agent: '🤖', homepage: '🌍', homepage_registry: '📚',
    call_webhook: '🪝', manage_outgoing_webhooks: '🪝', manage_webhooks: '🪝',
    netlify: '🚀', manage_updates: '🔄', execute_sudo: '🛡️',
    proxmox: '🖥️', ollama: '🦙', tailscale: '🔒',
    cloudflare_tunnel: '☁️', fetch_email: '📧', send_email: '📧',
    list_email_accounts: '📧', firewall: '🧱', ansible: '🔧',
    invasion_control: '🥚', github: '🐙', generate_image: '🎨',
    mqtt_publish: '📡', mqtt_subscribe: '📡', mqtt_unsubscribe: '📡',
    mqtt_get_messages: '📡', mcp_call: '🔌', adguard: '🛡️',
    google_workspace: '📊', remote_control: '🎮', media_registry: '🎬',
    thinking: '💡', coding: '💻', co_agent_spawn: '🤖',
    _default: '✨'
};

function spawnFloatingIcon(toolName) {
    const pill = agentStatusDiv.querySelector('.status-pill');
    if (!pill || agentStatusDiv.classList.contains('is-hidden')) return;
    // Throttle: max one icon per 800ms per tool
    const now = Date.now();
    const key = '_lastIcon_' + toolName;
    if (spawnFloatingIcon[key] && now - spawnFloatingIcon[key] < 800) return;
    spawnFloatingIcon[key] = now;
    const icon = document.createElement('span');
    icon.className = 'floating-icon';
    icon.textContent = TOOL_ICONS[toolName] || TOOL_ICONS._default;
    const pillW = pill.offsetWidth;
    const randomX = Math.random() * Math.max(pillW - 16, 20);
    icon.style.left = randomX + 'px';
    agentStatusDiv.appendChild(icon);
    icon.addEventListener('animationend', () => icon.remove());
}


/* ── Connection status pills ── */
function setConnectionState(state) {
    // state: 'connected' | 'disconnected' | 'reconnecting'
    const pills = ['connectionPill', 'connectionPill-m'].map(id => document.getElementById(id)).filter(Boolean);
    pills.forEach(pill => {
        pill.classList.remove('pill-active', 'pill-disconnected', 'pill-reconnecting');
        if (state === 'connected') {
            pill.classList.add('pill-active');
            pill.textContent = t('chat.agent_connected');
        } else if (state === 'reconnecting') {
            pill.classList.add('pill-reconnecting');
            pill.textContent = t('chat.agent_reconnecting');
        } else {
            pill.classList.add('pill-disconnected');
            pill.textContent = t('chat.agent_disconnected');
        }
    });
}

let sseReconnectTimer = null;
let _chatSSERegistered = false;

function connectSSE() {
    if (_chatSSERegistered) return;
    _chatSSERegistered = true;
    // Use the shared AuraSSE singleton instead of a dedicated EventSource.
    setConnectionState(window.AuraSSE.isConnected() ? 'connected' : 'reconnecting');

    window.AuraSSE.on('_open', function () {
        setConnectionState('connected');
        if (sseReconnectTimer) { clearTimeout(sseReconnectTimer); sseReconnectTimer = null; }
    });

    window.AuraSSE.on('_error', function (readyState) {
        if (readyState === EventSource.CLOSED) {
            setConnectionState('disconnected');
        } else {
            setConnectionState('reconnecting');
        }
        // AuraSSE handles reconnection internally; just track UI timer.
        if (!sseReconnectTimer) {
            sseReconnectTimer = setTimeout(function () { sseReconnectTimer = null; }, 5000);
        }
    });

    window.AuraSSE.onLegacy(handleSSEMessage);

    // Check auth on SSE error (may indicate 401)
    window.AuraSSE.on('_error', function () {
        fetch('/api/auth/status', { credentials: 'same-origin' }).then(function (r) {
            if (r.status === 401) {
                window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
            }
        }).catch(function () { });
    });
}

// Start in connecting state until onopen fires
setConnectionState('reconnecting');
connectSSE();

function handleSSEMessage(e) {
    try {
        const data = JSON.parse(e.data);
        let message = '';

        // Raw OpenAI streaming chunks (broker.SendJSON) and BroadcastType messages
        // ({"type":"system_metrics",...}) have no "event" field — skip them here;
        // streaming content is already rendered live via the chunk passthrough.
        if (!data.event) return;

        // Make the status bar visible early so floating icons can render
        if (data.event === 'thinking' || data.event === 'tool_start' || data.event === 'co_agent_spawn' || data.event === 'coding') {
            chatSetHidden(agentStatusDiv, false);
        }

        if (data.event === 'thinking') {
            stopBtn.disabled = false;
            message = data.detail || t('chat.sse_thinking');
            spawnFloatingIcon('thinking');
        } else if (data.event === 'tool_start') {
            if (data.detail === 'execute_skill') {
                message = t('chat.sse_execute_skill') + data.detail;
            } else if (data.detail === 'list_skills') {
                message = t('chat.sse_list_skills');
            } else if (data.detail === 'co_agent' || data.detail === 'co_agents') {
                return; // suppress generic "Using tool: co_agent" — co_agent_spawn event will follow
            } else {
                message = t('chat.sse_tool_start') + data.detail;
            }
            spawnFloatingIcon(data.detail);
        } else if (data.event === 'co_agent_spawn') {
            message = t('chat.sse_co_agent_spawn') + data.detail;
            spawnFloatingIcon('co_agent_spawn');
        } else if (data.event === 'workflow_plan') {
            message = t('chat.sse_workflow_plan');
        } else if (data.event === 'tool_end') {
            if (data.detail === 'co_agent' || data.detail === 'co_agents') {
                return; // suppress generic "Tool completed: co_agent"
            }
            message = t('chat.sse_tool_end') + data.detail;
        } else if (data.event === 'coding') {
            message = t('chat.sse_coding');
            spawnFloatingIcon('coding');
        } else if (data.event === 'error_recovery') {
            message = t('chat.sse_error_recovery');
        } else if (data.event === 'tool_call') {
            if (debugMode) {
                appendToolOutput(data.detail, t('chat.tool_call_label'));
            }
            // Extract human-readable text before/after the JSON tool call block and show it.
            // Only display if it's substantive (≥6 words) to prevent short preamble phrases
            // like "Ich wechsle in den Maintenance Mode." from leaking into the chat.
            const thinkingText = (data.detail || '')
                .replace(/```json[\s\S]*?```/g, '')   // strip ```json ... ``` fences
                .replace(/`[^`]*`/g, '')               // strip inline backtick code
                .replace(/\{[\s\S]*"action"\s*:[\s\S]*/g, '')    // strip {"action":...} to end
                .replace(/\{[\s\S]*"tool_call"\s*:[\s\S]*/g, '') // strip {"tool_call":...} to end (MiniMax format)
                .replace(/\{[\s\S]*"tool_name"\s*:[\s\S]*/g, '') // strip {"tool_name":...} to end
                .trim();
            if (thinkingText && thinkingText.split(/\s+/).filter(Boolean).length >= 6) {
                appendMessage('assistant', thinkingText);
            }
            return;
        } else if (data.event === 'tool_output') {
            if (debugMode) {
                appendToolOutput(data.detail, t('chat.tool_output_label'));
            }
            return;
        } else if (data.event === 'budget_update') {
            try {
                const b = JSON.parse(data.message || '{}');
                updateBudgetPills(b);
            } catch (_) { }
            return;
        } else if (data.event === 'budget_warning') {
            appendMessage('system', '\u26a0\ufe0f ' + (data.message || t('chat.budget_warning')));
            return;
        } else if (data.event === 'budget_blocked') {
            appendMessage('system', '\ud83d\udeab ' + (data.message || t('chat.budget_blocked')));
            return;
        } else if (data.event === 'todo_update') {
            updateTodoPanel(data.detail);
            return;
        } else if (data.event === 'plan_update') {
            try {
                const payload = JSON.parse(data.detail || '{}');
                updatePlanPanel(payload.plan || null);
            } catch (_) {
                updatePlanPanel(null);
            }
            return;
        } else if (data.event === 'image') {
            try {
                const imgData = JSON.parse(data.detail);
                if (imgData && imgData.path) {
                    seenSSEImages.add(imgData.path); // remember to skip duplicate in appendMessage
                    const cap = imgData.caption ? escapeHtml(imgData.caption) : '';
                    const safePath = escapeHtml(imgData.path);
                    const imgHTML = `
                                <div class="msg-row bot">
                                    <div class="avatar bot">🤖</div>
                                    <div class="bubble bot"><img class="chat-zoomable-image" src="${safePath}" alt="${cap}" title="${cap}" loading="lazy"></div>
                                </div>`;
                    chatContent.insertAdjacentHTML('beforeend', imgHTML);
                    chatBox.scrollTop = chatBox.scrollHeight;
                }
            } catch (e) { /* ignore */ }
            return;
        } else if (data.event === 'audio') {
            try {
                const audioData = JSON.parse(data.detail);
                if (audioData && audioData.path && !seenSSEAudios.has(audioData.path)) {
                    seenSSEAudios.add(audioData.path);
                    if (speakerMode) {
                        // Auto-play: queue audio, no visible player widget
                        enqueueAutoPlay(audioData.path);
                    } else {
                        // Manual mode: show inline player widget
                        const wrapper = document.createElement('div');
                        wrapper.className = 'chat-audio-wrapper';
                        if (audioData.title) {
                            const titleEl = document.createElement('div');
                            titleEl.className = 'chat-audio-title';
                            titleEl.textContent = audioData.title;
                            wrapper.appendChild(titleEl);
                        }
                        const player = new ChatAudioPlayer(audioData.path);
                        wrapper.appendChild(player.element);
                        const row = document.createElement('div');
                        row.className = 'msg-row bot';
                        row.innerHTML = '<div class="avatar bot">🤖</div><div class="bubble bot"></div>';
                        row.querySelector('.bubble').appendChild(wrapper);
                        chatContent.appendChild(row);
                        chatBox.scrollTop = chatBox.scrollHeight;
                    }
                }
            } catch (e) { /* ignore */ }
            return;
        } else if (data.event === 'document') {
            try {
                const docData = JSON.parse(data.detail);
                if (docData && docData.path && !seenSSEDocuments.has(docData.path)) {
                    seenSSEDocuments.add(docData.path);
                    const title = escapeHtml(docData.title || docData.filename || 'Document');
                    const fmt = escapeHtml((docData.format || '').toUpperCase() || 'FILE');
                    const docIcon = docFormatIcon(docData.format);
                    const previewUrl = isSafeHref(docData.preview_url, true) ? docData.preview_url : '';
                    const downloadPath = isSafeHref(docData.path, true) ? docData.path : '';
                    const openBtn = previewUrl
                        ? `<a href="${escapeHtml(previewUrl)}" target="_blank" rel="noopener noreferrer" title="Open">🔍</a>`
                        : '';
                    const dlBtn = downloadPath
                        ? `<a href="${escapeHtml(downloadPath)}" download="${escapeHtml(docData.filename || 'document')}" title="Download">⬇</a>`
                        : '';
                    const cardHTML = `
                        <div class="chat-document-card">
                            <div class="chat-document-icon">${docIcon}</div>
                            <div class="chat-document-info">
                                <div class="chat-document-title">${title}</div>
                                <div class="chat-document-format">${fmt}</div>
                            </div>
                            <div class="chat-document-actions">${openBtn}${dlBtn}</div>
                        </div>`;
                    const row = document.createElement('div');
                    row.className = 'msg-row bot';
                    row.innerHTML = '<div class="avatar bot">🤖</div><div class="bubble bot"></div>';
                    row.querySelector('.bubble').insertAdjacentHTML('beforeend', cardHTML);
                    chatContent.appendChild(row);
                    chatBox.scrollTop = chatBox.scrollHeight;
                }
            } catch (e) { /* ignore */ }
            return;
        } else if (data.event === 'done') {
            chatSetHidden(agentStatusDiv, true);
            stopBtn.disabled = true;
            hideTodoPanel();
            return;
        } else if (data.event === 'tokens') {
            const tokenEl = document.getElementById('tokenCounter');
            const session = data.session_total || 0;
            const est = data.is_estimated ? ' ~' : '';
            tokenEl.textContent = t('chat.token_counter_format', { count: `${session.toLocaleString()}${est}` });
            return;
        }

        if (message) {
            agentStatusText.textContent = message;
            chatSetHidden(agentStatusDiv, false);
        }
    } catch (err) { /* ignore parse errors */ }
}
/* ── Image lightbox ── */

/** Returns an emoji icon for common document formats. */
function docFormatIcon(fmt) {
    switch ((fmt || '').toLowerCase()) {
        case 'pdf': return '📄';
        case 'docx': case 'doc': return '📝';
        case 'xlsx': case 'xls': return '📊';
        case 'pptx': case 'ppt': return '📑';
        case 'csv': return '📋';
        case 'md': return '📓';
        case 'txt': return '📃';
        case 'json': return '🔧';
        case 'xml': return '🗂️';
        case 'html': case 'htm': return '🌐';
        default: return '📎';
    }
}

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
function initPushUI() {
    const btn = document.getElementById('push-btn');
    if (!btn) return;

    function applyState() {
        btn.classList.remove('push-granted', 'push-denied', 'push-unavailable');
        if (!window.getPushStatus) {
            // PWA not supported
            btn.classList.add('push-unavailable');
            btn.title = t('pwa.notifications_unavailable');
            btn.disabled = true;
            return;
        }
        const { available, permission } = window.getPushStatus();
        if (!available) {
            btn.classList.add('push-unavailable');
            btn.title = t('pwa.notifications_unavailable');
            btn.disabled = true;
        } else if (permission === 'granted') {
            btn.classList.add('push-granted');
            btn.title = t('pwa.notifications_enabled');
        } else if (permission === 'denied') {
            btn.classList.add('push-denied');
            btn.title = t('pwa.notifications_denied');
        } else {
            btn.title = t('pwa.btn_push_title');
        }
    }

    applyState();

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
            // Toggle off — unsubscribe
            if (window.revokePushPermission) {
                await window.revokePushPermission();
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

/* ── Initialize Chat Modules ── */
document.addEventListener('DOMContentLoaded', () => {
    // Smart Scroller
    if (window.SmartScroller) {
        window.SmartScroller.init(document.getElementById('chat-box'));
    }
    
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
                pendingAttachment = { path: data.path, filename: data.filename };
                attachName.textContent = data.filename;
                chatSetHidden(attachChip, false);
                uploadBtn.classList.add('has-file');
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
