(function () {
    'use strict';

    let suppressVisibleSync = false;

    function currentSessionID() {
        if (typeof window.activeSessionId === 'function') {
            try { return window.activeSessionId() || 'default'; } catch (_) { }
        }
        try { return localStorage.getItem('aurago-session-id') || 'default'; } catch (_) { return 'default'; }
    }

    function installMessageBridge() {
        if (typeof window.appendMessage !== 'function' || window.appendMessage.__realtimeSpeechBridge) return;
        const original = window.appendMessage;
        const bridged = function (role, content, timestamp) {
            const result = original.call(this, role, content, timestamp);
            if (!suppressVisibleSync && (role === 'user' || role === 'assistant')) {
                window.dispatchEvent(new CustomEvent('aurago:chat-visible-message', {
                    detail: {
                        role,
                        content,
                        surface: 'webchat',
                        chatSessionId: currentSessionID(),
                        source: 'webchat'
                    }
                }));
            }
            return result;
        };
        bridged.__realtimeSpeechBridge = true;
        bridged.__original = original;
        window.appendMessage = bridged;
    }

    function initialize() {
        installMessageBridge();
        const launcher = document.getElementById('realtime-speech-btn');
        const overlay = document.getElementById('realtime-speech-overlay');
        const panel = document.getElementById('realtime-speech-webchat-panel');
        const close = document.getElementById('realtime-speech-overlay-close');
        if (!launcher || !overlay || !panel || !window.AuraRealtimeSpeechUI) return;

        window.AuraRealtimeSpeechUI.mount(panel, {
            surface: 'webchat',
            compact: true,
            chatSessionId: currentSessionID
        });

        const setOpen = open => {
            overlay.classList.toggle('is-open', !!open);
            overlay.setAttribute('aria-hidden', open ? 'false' : 'true');
            launcher.setAttribute('aria-expanded', open ? 'true' : 'false');
            if (open) {
                const focusTarget = overlay.querySelector('select, button');
                if (focusTarget) focusTarget.focus();
            }
        };
        launcher.addEventListener('click', () => setOpen(!overlay.classList.contains('is-open')));
        close.addEventListener('click', () => setOpen(false));
        overlay.addEventListener('click', event => {
            if (event.target === overlay) setOpen(false);
        });
        document.addEventListener('keydown', event => {
            if (event.key === 'Escape' && overlay.classList.contains('is-open')) setOpen(false);
        });

        window.AuraRealtimeSpeech.addEventListener('display', event => {
            const detail = event.detail || {};
            if (detail.surface !== 'webchat' || !detail.content || typeof window.appendMessage !== 'function') return;
            suppressVisibleSync = true;
            try {
                window.appendMessage(detail.role === 'assistant' ? 'assistant' : 'user', detail.content);
            } finally {
                suppressVisibleSync = false;
            }
        });
        window.AuraRealtimeSpeech.addEventListener('repeat', event => {
            if (!event.detail || !event.detail.message || typeof window.showToast !== 'function') return;
            window.showToast(event.detail.message, 'warning');
        });
    }

    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', initialize, { once: true });
    else initialize();
})();
