// AuraGo – index page logic
// Extracted from index.html

/* ── DOM refs ── */
const chatBox = document.getElementById('chat-box');
const chatContent = document.getElementById('chat-content');
const chatForm = document.getElementById('chat-form');
const userInput = document.getElementById('user-input');
const sendBtn = document.getElementById('send-btn');
const stopBtn = document.getElementById('stop-btn');


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

/* ── i18n — I18N and t() now provided by page head + shared.js ── */

function applyI18n() {
    document.title = t('chat.page_title');
    /* Header pills & controls */
    document.getElementById('theme-toggle').title = t('common.toggle_theme');
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
    document.getElementById('clear-btn').textContent = t('chat.btn_clear');
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
        else if (href === '/missions') lbl.textContent = t('common.nav_missions');
        else if (href === '/config') lbl.textContent = t('common.nav_config');
        else if (href === '/invasion') lbl.textContent = t('common.nav_invasion');
        else if (href === '/auth/logout') lbl.textContent = t('common.nav_logout');
    });
    /* Greeting */
    const greetText = document.querySelector('.greeting-text');
    if (greetText) greetText.textContent = t('chat.greeting');
    /* Input area */
    document.getElementById('user-input').placeholder = t('chat.input_placeholder');
    document.getElementById('upload-btn').title = t('chat.upload_btn_title');
    document.getElementById('send-btn').title = t('chat.send_btn_title');
    document.getElementById('stop-btn').title = t('chat.stop_btn_title');
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
        cancelBtn.style.display = isConfirm ? '' : 'none';
        overlay.style.display = 'flex';
        overlay.style.opacity = '1';

        function cleanup(result) {
            overlay.style.display = 'none';
            overlay.style.opacity = '0';
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

function appendToolOutput(text, label) {
    if (!text || !debugMode) return;
    const greet = chatContent.querySelector('[data-greeting]');
    if (greet) greet.remove();
    const escaped = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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

/* ── Session Todo Panel (debug mode only) ── */
function updateTodoPanel(todoText) {
    if (!debugMode || !todoText) { hideTodoPanel(); return; }
    let panel = document.getElementById('todo-debug-panel');
    if (!panel) {
        panel = document.createElement('div');
        panel.id = 'todo-debug-panel';
        panel.className = 'todo-debug-panel';
        panel.innerHTML = '<div class="todo-debug-header">' + escapeHtml(t('chat.todo_panel_title')) + '</div><div class="todo-debug-body"></div>';
        // Insert before the agent status container
        const statusContainer = document.getElementById('agentStatusContainer');
        if (statusContainer) statusContainer.parentNode.insertBefore(panel, statusContainer);
        else document.body.appendChild(panel);
    }
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
    panel.style.display = 'block';
}

function hideTodoPanel() {
    const panel = document.getElementById('todo-debug-panel');
    if (panel) panel.style.display = 'none';
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
                if (lb) lb.style.display = '';
                if (rl) rl.style.display = '';
            }
        }
    } catch (e) { /* auth endpoint unavailable, hide button */ }

    try {
        const res = await fetch('/history');
        if (res.ok) {
            const history = await res.json();
            if (history && history.length > 0) {
                chatContent.innerHTML = '';
                history.forEach(msg => {
                    if (msg.role === 'user' || msg.role === 'assistant') {
                        appendMessage(msg.role, msg.content);
                        conversation.push(msg);
                    }
                });
            }
        }
    } catch (err) {
        console.error("Failed to load history:", err);
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
                    breaks: true,
                    linkify: true,
                    highlight: function (str, lang) {
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
                finalHTML = md.render(displayContent);
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
    chatBox.scrollTop = chatBox.scrollHeight;
}

/* ── Helpers ── */
function escapeHtml(str) {
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

/* ── File Upload ── */
const fileInput = document.getElementById('file-input');
const uploadBtn = document.getElementById('upload-btn');
const attachChip = document.getElementById('attachment-chip');
const attachName = document.getElementById('attachment-name');
const attachClear = document.getElementById('attachment-clear');
let pendingAttachment = null; // { path, filename }

uploadBtn.addEventListener('click', () => fileInput.click());

attachClear.addEventListener('click', () => {
    pendingAttachment = null;
    fileInput.value = '';
    attachChip.style.display = 'none';
    uploadBtn.classList.remove('has-file');
});

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
        attachChip.style.display = 'flex';
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
        attachChip.style.display = 'none';
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
    agentStatusDiv.style.display = 'flex';

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
                    messages: conversation
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
        conversation.push(assistantMessage);

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
        document.getElementById('agentStatusContainer').style.display = 'none';
        stopBtn.disabled = true;
    }
});

/* ── SSE (Server-Sent Events) ── */
const agentStatusDiv = document.getElementById('agentStatusContainer');
const agentStatusText = document.getElementById('agentStatusText');


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
let currentEventSource = null;

function connectSSE() {
    if (currentEventSource) {
        currentEventSource.close();
    }
    const es = new EventSource('/events');
    currentEventSource = es;

    es.onopen = function () {
        setConnectionState('connected');
        if (sseReconnectTimer) {
            clearTimeout(sseReconnectTimer);
            sseReconnectTimer = null;
        }
    };

    es.onerror = function () {
        if (es.readyState === EventSource.CLOSED) {
            setConnectionState('disconnected');
        } else {
            // CONNECTING — browser is already trying to reconnect
            setConnectionState('reconnecting');
        }
        // Force a clean reconnect after 5 s if still not back
        if (!sseReconnectTimer) {
            sseReconnectTimer = setTimeout(() => {
                sseReconnectTimer = null;
                if (currentEventSource.readyState !== EventSource.OPEN) {
                    setConnectionState('reconnecting');
                    connectSSE();
                }
            }, 5000);
        }
    };

    es.onmessage = handleSSEMessage;
    return es;
}

// Start in connecting state until onopen fires
setConnectionState('reconnecting');
const eventSource = connectSSE();

function handleSSEMessage(e) {
    try {
        const data = JSON.parse(e.data);
        let message = '';

        if (data.event === 'thinking') {
            stopBtn.disabled = false;
            message = data.detail || t('chat.sse_thinking');
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
        } else if (data.event === 'co_agent_spawn') {
            message = t('chat.sse_co_agent_spawn') + data.detail;
        } else if (data.event === 'workflow_plan') {
            message = t('chat.sse_workflow_plan');
        } else if (data.event === 'tool_end') {
            if (data.detail === 'co_agent' || data.detail === 'co_agents') {
                return; // suppress generic "Tool completed: co_agent"
            }
            message = t('chat.sse_tool_end') + data.detail;
        } else if (data.event === 'coding') {
            message = t('chat.sse_coding');
        } else if (data.event === 'error_recovery') {
            message = t('chat.sse_error_recovery');
        } else if (data.event === 'tool_call') {
            if (debugMode) {
                appendToolOutput(data.detail, t('chat.tool_call_label'));
            }
            // Extract human-readable text before/after the JSON tool call block and show it
            const thinkingText = (data.detail || '')
                .replace(/```json[\s\S]*?```/g, '')   // strip ```json ... ``` fences
                .replace(/`[^`]*`/g, '')               // strip inline backtick code
                .replace(/\{[\s\S]*"action"\s*:[\s\S]*/g, '') // strip from {"action": to end (greedy, handles nested objects)
                .trim();
            if (thinkingText) {
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
        } else if (data.event === 'image') {
            try {
                const imgData = JSON.parse(data.detail);
                if (imgData && imgData.path) {
                    seenSSEImages.add(imgData.path); // remember to skip duplicate in appendMessage
                    const cap = imgData.caption ? escapeHtml(imgData.caption) : '';
                    const imgHTML = `
                                <div class="msg-row bot">
                                    <div class="avatar bot">🤖</div>
                                    <div class="bubble bot"><img src="${imgData.path}" alt="${cap}" title="${cap}" loading="lazy" style="cursor:zoom-in;"></div>
                                </div>`;
                    chatContent.insertAdjacentHTML('beforeend', imgHTML);
                    chatBox.scrollTop = chatBox.scrollHeight;
                }
            } catch (e) { /* ignore */ }
            return;
        } else if (data.event === 'done') {
            agentStatusDiv.style.display = 'none';
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

        agentStatusText.textContent = message;
        agentStatusDiv.style.display = 'flex';
    } catch (err) { /* ignore parse errors */ }
}
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
        bp.style.display = 'inline-flex';
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
        el.style.display = 'inline-flex';
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

function updateMoodWidget(data) {
    if (!data || !data.enabled) return;
    const toggle = document.getElementById('moodToggle');
    const emoji = moodStateEmojiMap[data.mood] || '🧠';
    const moodLabel = t(moodNameKeys[data.mood] || 'chat.mood_default_text');
    document.getElementById('moodEmoji').textContent = emoji;
    document.getElementById('moodText').textContent = moodLabel;
    document.getElementById('moodPanelEmoji').textContent = emoji;
    document.getElementById('moodPanelLabel').textContent = moodLabel;
    const traitsEl = document.getElementById('moodTraits');
    traitsEl.innerHTML = '';
    traitOrder.forEach(tr => {
        const v = (data.traits && data.traits[tr] != null) ? data.traits[tr] : 0.5;
        const pct = Math.round(v * 100);
        const row = document.createElement('div');
        row.className = 'trait-row';
        row.innerHTML = `<span class="trait-label">${t('chat.trait_' + tr)}</span>` +
            `<div class="trait-bar-bg"><div class="trait-bar-fill" data-trait="${tr}" style="width:${pct}%"></div></div>` +
            `<span class="trait-value">${v.toFixed(2)}</span>`;
        traitsEl.appendChild(row);
    });
    toggle.style.display = 'inline-flex';
}

if (PERSONALITY_ENABLED) {
    fetch('/api/personality/state').then(r => r.json()).then(updateMoodWidget).catch(() => { });
    setInterval(() => {
        fetch('/api/personality/state').then(r => r.json()).then(updateMoodWidget).catch(() => { });
    }, 30000);

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
