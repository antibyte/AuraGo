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
        resetSSEDedupSets(); // reset after final response is rendered
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

function refreshBudgetPills() {
    fetch('/api/budget').then(r => r.json()).then(updateBudgetPills).catch(() => { });
}
bindHeaderActivation(document.getElementById('budgetPill'), refreshBudgetPills);
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
            setIconPillText(el, 'credit-card', t('chat.credits_pill_text', { amount: c.balance.toFixed(2) }));
            el.title = t('chat.credits_tooltip_used_limit', { usage: c.usage.toFixed(2), limit: c.limit.toFixed(2) });
            el.classList.remove('budget-warning', 'budget-exceeded');
            const pct = c.usage / c.limit;
            if (pct >= 1.0) el.classList.add('budget-exceeded');
            else if (pct >= 0.8) el.classList.add('budget-warning');
        } else {
            setIconPillText(el, 'credit-card', t('chat.credits_pill_text', { amount: c.usage.toFixed(2) }));
            el.title = t('chat.credits_tooltip_payg', { usage: c.usage.toFixed(2) });
        }
    }
}
function refreshCreditsPills() {
    fetch('/api/credits').then(r => r.json()).then(updateCreditsPills).catch(() => { });
}
bindHeaderActivation(document.getElementById('creditsPill'), refreshCreditsPills);
fetch('/api/credits').then(r => r.json()).then(updateCreditsPills).catch(() => { });

/* ── Mood Widget ── */
