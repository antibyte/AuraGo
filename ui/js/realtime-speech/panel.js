(function () {
    'use strict';

    const runtime = window.AuraRealtimeSpeech;
    const mounts = new Map();

    function text(key, fallback) {
        const value = typeof window.t === 'function' ? window.t(key) : '';
        return value && value !== key ? value : fallback;
    }

    function escapeHTML(value) {
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    const stateLabels = {
        idle: ['chat.realtime_state_idle', 'Ready'],
        connecting: ['chat.realtime_state_connecting', 'Connecting'],
        listening: ['chat.realtime_state_listening', 'Listening'],
        speaking: ['chat.realtime_state_speaking', 'Speaking'],
        executing: ['chat.realtime_state_executing', 'Working'],
        parked: ['chat.realtime_state_parked', 'Parked'],
        reconnecting: ['chat.realtime_state_reconnecting', 'Reconnecting'],
        error: ['chat.realtime_state_error', 'Error']
    };

    function stateText(state) {
        const entry = stateLabels[state] || stateLabels.idle;
        return text(entry[0], entry[1]);
    }

    function enabledProfiles() {
        return ((runtime.config && runtime.config.profiles) || []).filter(profile => profile.enabled && profile.api_key_set);
    }

    function updatePanel(root, options) {
        const currentState = runtime.state || 'idle';
        const active = !!runtime.sessionId;
        const profiles = enabledProfiles();
        const selected = (runtime.profile && runtime.profile.id) ||
            (runtime.config && runtime.config.default_profile) ||
            (profiles[0] && profiles[0].id) || '';
        const errorMessage = currentState === 'error' ? String(runtime.lastErrorMessage || '') : '';
        const panel = root.querySelector('[data-realtime-panel]');
        if (!panel) return;

        panel.classList.toggle('compact', !!options.compact);
        panel.dataset.state = currentState;
        const privacy = root.querySelector('[data-realtime-privacy]');
        if (privacy) {
            privacy.textContent = active
                ? text('chat.realtime_privacy_active', 'The microphone is processed locally. Only detected speech is sent.')
                : text('chat.realtime_privacy_idle', 'Audio stays local until a live session is started.');
        }
        const state = root.querySelector('[data-realtime-state]');
        if (state) state.textContent = stateText(currentState);

        const profile = root.querySelector('[data-realtime-profile]');
        if (profile) {
            const signature = profiles.map(item => [item.id, item.name, item.provider].join('\u0000')).join('\u0001');
            if (profile.dataset.signature !== signature) {
                const previous = profile.value;
                profile.innerHTML = profiles.map(item =>
                    `<option value="${escapeHTML(item.id)}">${escapeHTML(item.name)} · ${escapeHTML(item.provider)}</option>`
                ).join('');
                profile.dataset.signature = signature;
                const available = profiles.some(item => item.id === previous);
                profile.value = available ? previous : selected;
            } else if (active && selected) {
                profile.value = selected;
            }
            profile.disabled = active;
        }

        const start = root.querySelector('[data-realtime-start]');
        if (start) {
            start.disabled = profiles.length === 0;
            const icon = start.querySelector('[data-realtime-start-icon]');
            const label = start.querySelector('[data-realtime-start-label]');
            if (icon) icon.textContent = active ? '■' : '●';
            if (label) label.textContent = active
                ? text('chat.realtime_stop', 'Stop')
                : text('chat.realtime_start', 'Start');
        }

        const mute = root.querySelector('[data-realtime-mute]');
        if (mute) {
            mute.disabled = !active;
            const icon = mute.querySelector('[data-realtime-mute-icon]');
            const label = mute.querySelector('[data-realtime-mute-label]');
            if (icon) icon.textContent = runtime.muted ? '🔇' : '🎙';
            if (label) label.textContent = runtime.muted
                ? text('chat.realtime_unmute', 'Unmute')
                : text('chat.realtime_mute', 'Mute');
        }

        const cancel = root.querySelector('[data-realtime-cancel]');
        if (cancel) cancel.disabled = !runtime.actionActive;
        const parkedNotice = root.querySelector('[data-realtime-notice]');
        if (parkedNotice) parkedNotice.hidden = currentState !== 'parked';
        const errorNotice = root.querySelector('[data-realtime-error]');
        if (errorNotice) {
            errorNotice.hidden = !errorMessage;
            const message = errorNotice.querySelector('[data-realtime-error-message]');
            if (message) message.textContent = errorMessage;
        }
    }

    function render(root, options) {
        root.innerHTML = `<section class="realtime-speech-panel" data-realtime-panel data-state="idle">
            <header class="realtime-speech-panel-header">
                <div class="realtime-speech-mark" aria-hidden="true"><span></span><span></span><span></span><span></span><span></span></div>
                <div>
                    <h3>${escapeHTML(text('chat.realtime_title', 'Live Speech'))}</h3>
                    <p data-realtime-privacy></p>
                </div>
            </header>
            <div class="realtime-speech-status-row">
                <span class="realtime-speech-state-dot"></span>
                <strong data-realtime-state></strong>
                <span class="realtime-speech-live-caption" data-realtime-caption aria-live="polite"></span>
            </div>
            <label class="realtime-speech-profile-label">
                <span>${escapeHTML(text('chat.realtime_profile', 'Profile'))}</span>
                <select data-realtime-profile></select>
            </label>
            <div class="realtime-speech-wave" aria-hidden="true">
                ${Array.from({ length: 24 }, (_, index) => `<i style="--bar:${index}"></i>`).join('')}
            </div>
            <div class="realtime-speech-controls">
                <button type="button" class="realtime-speech-primary" data-realtime-start>
                    <span class="realtime-speech-control-icon" data-realtime-start-icon aria-hidden="true"></span>
                    <span data-realtime-start-label></span>
                </button>
                <button type="button" data-realtime-mute>
                    <span data-realtime-mute-icon aria-hidden="true"></span>
                    <span data-realtime-mute-label></span>
                </button>
                <button type="button" data-realtime-cancel>
                    <span aria-hidden="true">×</span>
                    <span>${escapeHTML(text('chat.realtime_cancel_task', 'Cancel task'))}</span>
                </button>
            </div>
            <div class="realtime-speech-notice" data-realtime-notice hidden>
                <span aria-hidden="true">◉</span>
                ${escapeHTML(text('chat.realtime_parked_privacy', 'Parked: the provider connection is paused, while local voice detection keeps the microphone ready.'))}
            </div>
            <div class="realtime-speech-notice realtime-speech-error" data-realtime-error role="alert" hidden>
                <span aria-hidden="true">!</span>
                <span data-realtime-error-message></span>
            </div>
        </section>`;

        const start = root.querySelector('[data-realtime-start]');
        const profile = root.querySelector('[data-realtime-profile]');
        const mute = root.querySelector('[data-realtime-mute]');
        const cancel = root.querySelector('[data-realtime-cancel]');
        start.addEventListener('click', async () => {
            start.disabled = true;
            try {
                if (runtime.sessionId) await runtime.stop();
                else await runtime.start({
                    profileId: profile.value,
                    surface: options.surface || 'webchat',
                    chatSessionId: typeof options.chatSessionId === 'function' ? options.chatSessionId() : options.chatSessionId
                });
            } catch (error) {
                runtime.lastErrorMessage = String(runtime.lastErrorMessage || error && error.message || '');
            } finally {
                refreshAll();
            }
        });
        mute.addEventListener('click', () => {
            runtime.setMuted(!runtime.muted);
            refreshAll();
        });
        cancel.addEventListener('click', async () => {
            await runtime.cancelCurrentAction();
            refreshAll();
        });
        updatePanel(root, options);
    }

    function refreshAll() {
        mounts.forEach((options, root) => {
            if (!root.isConnected) {
                mounts.delete(root);
                return;
            }
            updatePanel(root, options);
        });
        document.querySelectorAll('[data-realtime-speech-launcher]').forEach(button => {
            button.dataset.active = runtime.sessionId ? 'true' : 'false';
            button.dataset.state = runtime.state || 'idle';
            button.setAttribute('aria-pressed', runtime.sessionId ? 'true' : 'false');
        });
    }

    function updateCaption(detail) {
        mounts.forEach((_options, root) => {
            const caption = root.querySelector('[data-realtime-caption]');
            if (!caption) return;
            caption.textContent = detail && detail.text ? detail.text : '';
        });
    }

    function mount(root, options) {
        if (!root) return () => { };
        mounts.set(root, Object.assign({ surface: 'webchat', compact: false }, options || {}));
        void runtime.initialize().catch(error => {
            root.innerHTML = `<div class="realtime-speech-load-error">${escapeHTML(error.message)}</div>`;
        }).finally(refreshAll);
        render(root, mounts.get(root));
        return () => {
            mounts.delete(root);
            root.innerHTML = '';
        };
    }

    runtime.addEventListener('state', refreshAll);
    runtime.addEventListener('config', refreshAll);
    runtime.addEventListener('mute', refreshAll);
    runtime.addEventListener('action', refreshAll);
    runtime.addEventListener('transcript', event => updateCaption(event.detail || {}));
    runtime.addEventListener('repeat', event => {
        mounts.forEach((_options, root) => {
            const notice = root.querySelector('[data-realtime-notice]');
            if (notice) {
                notice.hidden = false;
                notice.textContent = event.detail.message;
            }
        });
    });

    window.AuraRealtimeSpeechUI = { mount, refresh: refreshAll, stateText };
})();
