(function () {
    'use strict';

    function translate(key, fallback) {
        const value = typeof window.t === 'function' ? window.t(key) : '';
        return value && value !== key ? value : fallback;
    }

    function setText(id, value) {
        const node = document.getElementById(id);
        if (node) node.textContent = value;
    }

    function sessionLabel(state) {
        const normalized = String(state || 'idle').toLowerCase();
        return translate('dashboard.realtime_speech_state_' + normalized, normalized);
    }

    async function loadRealtimeSpeechStatus() {
        const card = document.getElementById('card-realtime-speech');
        if (!card) return;
        card.setAttribute('data-state', 'loading');
        try {
            const response = await fetch('/api/realtime-speech/status', {
                credentials: 'same-origin',
                cache: 'no-store'
            });
            if (!response.ok) throw new Error('status unavailable');
            const status = await response.json();
            setText('rs-status-enabled', status.enabled
                ? translate('dashboard.realtime_speech_on', 'On')
                : translate('dashboard.realtime_speech_off', 'Off'));
            setText('rs-status-profiles', String(status.profile_count || 0));
            setText('rs-status-profile', status.active_profile || '—');
            setText('rs-status-session', sessionLabel(status.session_state));
            card.setAttribute('data-state', 'loaded');
        } catch (_error) {
            setText('rs-status-enabled', translate('dashboard.realtime_speech_unavailable', 'Unavailable'));
            setText('rs-status-profiles', '—');
            setText('rs-status-profile', '—');
            setText('rs-status-session', '—');
            card.setAttribute('data-state', 'error');
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', loadRealtimeSpeechStatus, { once: true });
    } else {
        void loadRealtimeSpeechStatus();
    }
})();
