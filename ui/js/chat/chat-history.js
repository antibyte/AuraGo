// AuraGo Chat — History loading, session management, recovery

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

async function tryRecoverFromHistory() {
    try {
        const res = await fetch(buildHistoryUrl());
        if (!res.ok) return;
        const history = await res.json();
        if (!Array.isArray(history) || history.length === 0) return;
        let lastAssistant = null;
        for (let i = history.length - 1; i >= 0; i--) {
            const m = history[i];
            if (m.role === 'assistant' && !m.is_internal) {
                lastAssistant = m;
                break;
            }
        }
        if (!lastAssistant || !lastAssistant.content) return;
        const alreadyShown = conversation.some(
            m => m.role === 'assistant' && m.content === lastAssistant.content
        );
        if (alreadyShown) return;
        appendMessage('assistant', lastAssistant.content);
        conversation.push({ role: 'assistant', content: lastAssistant.content });
        if (conversation.length > 200) { conversation = conversation.slice(-200); }
        _httpResponseRendered = true;
    } catch (e) { }
}

function getActiveSessionId() {
    return (window.SessionDrawer && window.SessionDrawer.getActiveSessionId()) || 'default';
}

function isCurrentSession(payload) {
    if (!payload) return true;
    const sid = payload.session_id;
    if (!sid) return true;
    return sid === getActiveSessionId();
}

function buildHistoryUrl() {
    const sid = getActiveSessionId();
    if (sid && sid !== 'default') {
        return '/history?session_id=' + encodeURIComponent(sid);
    }
    return '/history';
}

function buildClearUrl() {
    const sid = getActiveSessionId();
    if (sid && sid !== 'default') {
        return '/clear?session_id=' + encodeURIComponent(sid);
    }
    return '/clear';
}

const PERSONA_PREVIEW_FALLBACK = 'custom';
const PERSONA_PREVIEW_KEYS = new Set([
    'evil', 'friend', 'mcp', 'mistress', 'neutral', 'professional', 'psycho',
    'punk', 'secretary', 'servant', 'terminator', 'thinker',
]);

function personaPreviewKey(name, isCore) {
    const key = String(name || '').toLowerCase();
    if (!isCore) return PERSONA_PREVIEW_FALLBACK;
    return PERSONA_PREVIEW_KEYS.has(key) ? key : PERSONA_PREVIEW_FALLBACK;
}

function showPersonaPreview(previewKey) {
    const personaPreview = document.getElementById('personality-preview');
    const img = document.getElementById('personality-preview-image');
    if (!personaPreview || !img) return;
    const key = previewKey || PERSONA_PREVIEW_FALLBACK;
    img.src = `/img/personas/${key}.png`;
    personaPreview.hidden = false;
}

function hidePersonaPreview() {
    const personaPreview = document.getElementById('personality-preview');
    if (personaPreview) personaPreview.hidden = true;
}

window._hidePersonalityPreview = hidePersonaPreview;

window.onSessionSwitch = async function (sessionId) {
    chatContent.innerHTML = '';
    conversation = [];
    hideTodoPanel();
    try {
        const url = sessionId && sessionId !== 'default'
            ? '/history?session_id=' + encodeURIComponent(sessionId)
            : '/history';
        const res = await fetch(url);
        if (res.ok) {
            const history = await res.json();
            if (history && history.length > 0) {
                if (window.ChatRobotMascot && typeof window.ChatRobotMascot.anchorImmediately === 'function') {
                    window.ChatRobotMascot.anchorImmediately();
                }
                await renderHistoryMessagesBatched(history);
            } else {
                appendMessage('assistant', t('chat.greeting'));
            }
        }
    } catch (err) {
        console.error('Failed to load session history:', err);
        appendMessage('assistant', t('chat.greeting'));
    }
};

async function initPage() {
    applyI18n();
    if (window.SessionDrawer) {
        window.SessionDrawer.init();
    }
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
    } catch (e) { }
    try {
        const res = await fetch(buildHistoryUrl());
        if (res.ok) {
            const history = await res.json();
            if (history && history.length > 0) {
                if (window.ChatRobotMascot && typeof window.ChatRobotMascot.anchorImmediately === 'function') {
                    window.ChatRobotMascot.anchorImmediately();
                }
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
                if (greet && window.ChatRobotMascot && typeof window.ChatRobotMascot.launchToAnchor === 'function') {
                    window.ChatRobotMascot.launchToAnchor();
                }
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
    try {
        const res = await fetch('/api/personalities');
        if (res.ok) {
            const data = await res.json();
            const btn = document.getElementById('personality-select');
            const dropdown = document.getElementById('personality-dropdown');
            const label = document.getElementById('personality-label');
            if (btn && dropdown) {
                dropdown.innerHTML = '';
                data.personalities.forEach(p => {
                    const opt = document.createElement('button');
                    opt.type = 'button';
                    opt.className = 'personality-option';
                    opt.dataset.value = p.name;
                    opt.dataset.previewKey = personaPreviewKey(p.name, p.core);
                    opt.setAttribute('role', 'option');
                    const icon = document.createElement('span');
                    icon.className = 'personality-option-icon';
                    icon.textContent = '\u{1F916}';
                    opt.appendChild(icon);
                    const lbl = document.createElement('span');
                    lbl.className = 'personality-option-label';
                    lbl.textContent = p.name.charAt(0).toUpperCase() + p.name.slice(1);
                    opt.appendChild(lbl);
                    if (p.name === data.active) {
                        opt.classList.add('active');
                        opt.setAttribute('aria-selected', 'true');
                        if (label) label.textContent = p.name.charAt(0).toUpperCase() + p.name.slice(1);
                    }
                    opt.addEventListener('click', () => {
                        if (typeof window._selectPersonality === 'function') {
                            window._selectPersonality(p.name);
                        }
                    });
                    opt.addEventListener('mouseenter', () => showPersonaPreview(opt.dataset.previewKey));
                    opt.addEventListener('focus', () => showPersonaPreview(opt.dataset.previewKey));
                    dropdown.appendChild(opt);
                });
                dropdown.addEventListener('mouseleave', hidePersonaPreview);
            }
        }
    } catch (err) {
        console.error("Failed to load personalities:", err);
    }
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initPage);
} else {
    initPage();
}
