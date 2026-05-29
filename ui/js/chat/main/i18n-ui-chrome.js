/* ── Mood Feedback Buttons ── */
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
    const psBtn = document.getElementById('personality-select');
    if (psBtn) psBtn.title = t('chat.personality_select_title');
    const psLabel = document.getElementById('personality-label');
    if (psLabel) psLabel.textContent = t('chat.personality_loading');
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
        const previewText = String(sheet.abstract || sheet.content || '').replace(/\s+/g, ' ').trim();
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
        const res = await fetch('/api/cheatsheets?active=true&created_by=user');
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
