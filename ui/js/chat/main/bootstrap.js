const moodStateIconMap = {
    curious: 'mood-curious', focused: 'mood-focused', creative: 'mood-creative',
    analytical: 'mood-analytical', cautious: 'mood-cautious', playful: 'mood-playful',
    frustrated: 'mood-cautious', concerned: 'mood-cautious', relaxed: 'mood-brain'
};
const moodNameKeys = {
    curious: 'chat.mood_curious', focused: 'chat.mood_focused', creative: 'chat.mood_creative',
    analytical: 'chat.mood_analytical', cautious: 'chat.mood_cautious', playful: 'chat.mood_playful',
    frustrated: 'chat.mood_frustrated', concerned: 'chat.mood_concerned', relaxed: 'chat.mood_relaxed'
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
    const iconKey = moodStateIconMap[data.mood] || 'mood-brain';
    const moodLabel = t(moodNameKeys[data.mood] || 'chat.mood_default_text');
    applyChatIcon(document.getElementById('moodEmoji'), iconKey);
    document.getElementById('moodText').textContent = moodLabel;
    applyChatIcon(document.getElementById('moodPanelEmoji'), iconKey);
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

    function toggleMoodPanel(e) {
        e.stopPropagation();
        document.getElementById('moodPanel').classList.toggle('open');
        document.getElementById('moodToggle').classList.toggle('open');
    }
    bindHeaderActivation(document.getElementById('moodToggle'), toggleMoodPanel);
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
const THEME_ICON_KEYS = {
    'dark': 'theme-dark',
    'light': 'theme-light',
    'retro-crt': 'theme-retro-crt',
    'cyberwar': 'theme-cyberwar',
    'lollipop': 'theme-lollipop',
    'dark-sun': 'theme-dark-sun',
    'ocean': 'theme-ocean',
    'sandstorm': 'theme-sandstorm',
    'papyrus': 'theme-papyrus',
    'threedee': 'theme-threedee',
    'black-matrix': 'theme-black-matrix',
    '8bit': 'theme-8bit'
};

function initChatThemePicker() {
    const picker = document.getElementById('chat-theme-picker');
    const btn = document.getElementById('chat-theme-btn');
    const dropdown = document.getElementById('chat-theme-dropdown');
    const icon = document.getElementById('chat-theme-icon');
    if (!picker || !btn || !dropdown || !icon) return;
    if (picker.dataset.initialized === 'true') return;
    picker.dataset.initialized = 'true';

    function _themeLabel(labelKey, fallbackLabel) {
        const translatedLabel = typeof t === 'function' ? t(labelKey) : fallbackLabel;
        return translatedLabel === labelKey ? fallbackLabel : translatedLabel;
    }

    function _renderThemeOptions() {
        const definitions = Array.isArray(window.AuraChatThemes) && window.AuraChatThemes.length
            ? window.AuraChatThemes
            : [
                { theme: 'dark', icon: 'theme-dark', labelKey: 'chat.theme_standard', fallbackLabel: 'Standard' },
                { theme: 'light', icon: 'theme-light', labelKey: 'chat.theme_light', fallbackLabel: 'Light' },
                { theme: 'retro-crt', icon: 'theme-retro-crt', labelKey: 'chat.theme_retro_crt', fallbackLabel: 'Retro CRT' },
                { theme: '8bit', icon: 'theme-8bit', labelKey: 'chat.theme_8bit', fallbackLabel: '8Bit' },
                { theme: 'cyberwar', icon: 'theme-cyberwar', labelKey: 'chat.theme_cyberwar', fallbackLabel: 'Cyberwar' },
                { theme: 'lollipop', icon: 'theme-lollipop', labelKey: 'chat.theme_lollipop', fallbackLabel: 'Lollipop' },
                { theme: 'dark-sun', icon: 'theme-dark-sun', labelKey: 'chat.theme_dark_sun', fallbackLabel: 'Dark Sun' },
                { theme: 'ocean', icon: 'theme-ocean', labelKey: 'chat.theme_ocean', fallbackLabel: 'Ocean' },
                { theme: 'sandstorm', icon: 'theme-sandstorm', labelKey: 'chat.theme_sandstorm', fallbackLabel: 'Sandstorm' },
                { theme: 'papyrus', icon: 'theme-papyrus', labelKey: 'chat.theme_papyrus', fallbackLabel: 'Papyrus' },
                { theme: 'threedee', icon: 'theme-threedee', labelKey: 'chat.theme_threedee', fallbackLabel: 'ThreeDee' },
                { theme: 'black-matrix', icon: 'theme-black-matrix', labelKey: 'chat.theme_black_matrix', fallbackLabel: 'Black Matrix' },
            ];

        dropdown.replaceChildren();
        definitions.forEach((definition) => {
            const option = document.createElement('button');
            option.type = 'button';
            option.className = 'chat-theme-option';
            option.dataset.theme = definition.theme;
            option.setAttribute('role', 'option');

            const optionIcon = document.createElement('span');
            optionIcon.className = 'chat-theme-option-icon';
            optionIcon.dataset.chatIcon = definition.icon;

            const label = document.createElement('span');
            label.className = 'chat-theme-option-label';
            label.dataset.i18n = definition.labelKey;
            label.textContent = _themeLabel(definition.labelKey, definition.fallbackLabel);

            option.append(optionIcon, label);
            dropdown.appendChild(option);
        });

        if (window.AuraChatIcons) window.AuraChatIcons.hydrate(dropdown);
    }

    _renderThemeOptions();

    function _ensureThemeOption(theme, iconKey, labelKey, fallbackLabel) {
        if (dropdown.querySelector(`.chat-theme-option[data-theme="${theme}"]`)) return;
        const option = document.createElement('button');
        option.type = 'button';
        option.className = 'chat-theme-option';
        option.dataset.theme = theme;
        option.setAttribute('role', 'option');

        const optionIcon = document.createElement('span');
        optionIcon.className = 'chat-theme-option-icon';
        optionIcon.dataset.chatIcon = iconKey;

        const label = document.createElement('span');
        label.className = 'chat-theme-option-label';
        label.dataset.i18n = labelKey;
        label.textContent = _themeLabel(labelKey, fallbackLabel);

        option.append(optionIcon, label);
        dropdown.appendChild(option);
        if (window.AuraChatIcons) window.AuraChatIcons.hydrate(option);
    }

    _ensureThemeOption('8bit', 'theme-8bit', 'chat.theme_8bit', '8Bit');

    function _refreshIcon(theme) {
        applyChatIcon(icon, THEME_ICON_KEYS[theme] || 'theme-dark');
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

    function toggleChatThemeDropdown(e) {
        e.stopPropagation();
        const isOpen = !dropdown.hidden;
        dropdown.hidden = isOpen;
        btn.setAttribute('aria-expanded', String(!isOpen));
    }
    bindHeaderActivation(btn, toggleChatThemeDropdown);

    dropdown.querySelectorAll('.chat-theme-option').forEach(opt => {
        bindHeaderActivation(opt, () => {
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
