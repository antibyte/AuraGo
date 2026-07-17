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

    function render(root, options) {
        const currentState = runtime.state || 'idle';
        const active = !!runtime.sessionId;
        const profiles = enabledProfiles();
        const selected = (runtime.profile && runtime.profile.id) ||
            (runtime.config && runtime.config.default_profile) ||
            (profiles[0] && profiles[0].id) || '';
        const compact = options.compact ? ' compact' : '';
        root.innerHTML = `<section class="realtime-speech-panel${compact}" data-state="${escapeHTML(currentState)}">
            <header class="realtime-speech-panel-header">
                <div class="realtime-speech-mark" aria-hidden="true"><span></span><span></span><span></span><span></span><span></span></div>
                <div>
                    <h3>${escapeHTML(text('chat.realtime_title', 'Live Speech'))}</h3>
                    <p data-realtime-privacy>${escapeHTML(active
                        ? text('chat.realtime_privacy_active', 'The microphone is processed locally. Only detected speech is sent.')
                        : text('chat.realtime_privacy_idle', 'Audio stays local until a live session is started.'))}</p>
                </div>
            </header>
            <div class="realtime-speech-status-row">
                <span class="realtime-speech-state-dot"></span>
                <strong data-realtime-state>${escapeHTML(stateText(currentState))}</strong>
                <span class="realtime-speech-live-caption" data-realtime-caption aria-live="polite"></span>
            </div>
            <label class="realtime-speech-profile-label">
                <span>${escapeHTML(text('chat.realtime_profile', 'Profile'))}</span>
                <select data-realtime-profile ${active ? 'disabled' : ''}>
                    ${profiles.map(profile => `<option value="${escapeHTML(profile.id)}" ${profile.id === selected ? 'selected' : ''}>${escapeHTML(profile.name)} · ${escapeHTML(profile.provider)}</option>`).join('')}
                </select>
            </label>
            <div class="realtime-speech-wave" aria-hidden="true">
                ${Array.from({ length: 24 }, (_, index) => `<i style="--bar:${index}"></i>`).join('')}
            </div>
            <div class="realtime-speech-controls">
                <button type="button" class="realtime-speech-primary" data-realtime-start ${profiles.length ? '' : 'disabled'}>
                    <span class="realtime-speech-control-icon" aria-hidden="true">${active ? '■' : '●'}</span>
                    <span>${escapeHTML(active ? text('chat.realtime_stop', 'Stop') : text('chat.realtime_start', 'Start'))}</span>
                </button>
                <button type="button" data-realtime-mute ${active ? '' : 'disabled'}>
                    <span aria-hidden="true">${runtime.muted ? '🔇' : '🎙'}</span>
                    <span>${escapeHTML(runtime.muted ? text('chat.realtime_unmute', 'Unmute') : text('chat.realtime_mute', 'Mute'))}</span>
                </button>
                <button type="button" data-realtime-cancel ${runtime.actionActive ? '' : 'disabled'}>
                    <span aria-hidden="true">×</span>
                    <span>${escapeHTML(text('chat.realtime_cancel_task', 'Cancel task'))}</span>
                </button>
            </div>
            <div class="realtime-speech-notice" data-realtime-notice ${currentState === 'parked' ? '' : 'hidden'}>
                <span aria-hidden="true">◉</span>
                ${escapeHTML(text('chat.realtime_parked_privacy', 'Parked: the provider connection is paused, while local voice detection keeps the microphone ready.'))}
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
                const caption = root.querySelector('[data-realtime-caption]');
                if (caption) caption.textContent = error.message;
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
    }

    function refreshAll() {
        mounts.forEach((options, root) => {
            if (!root.isConnected) {
                mounts.delete(root);
                return;
            }
            render(root, options);
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
