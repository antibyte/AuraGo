(function () {
    'use strict';

    const instances = new Map();

    function hostFor(windowId, context) {
        const runtime = context && context.__desktopRuntime;
        if (runtime && typeof runtime.contentEl === 'function') return runtime.contentEl(windowId);
        const win = document.querySelector('.vd-window[data-window-id="' + String(windowId).replace(/"/g, '\\"') + '"]');
        return win && win.querySelector('[data-window-content]');
    }

    function render(windowId, context) {
        dispose(windowId, { keepSession: true });
        const host = hostFor(windowId, context || {});
        if (!host) throw new Error('Live Speech window content is not available');
        host.innerHTML = `<div class="vd-live-speech-app">
            <div class="vd-live-speech-intro">
                <span class="vd-live-speech-kicker" data-i18n="desktop.live_speech_kicker">AuraGo voice interface</span>
                <h2 data-i18n="desktop.live_speech_title">Live Speech</h2>
                <p data-i18n="desktop.live_speech_description">Talk naturally with AuraGo and carry out tasks without leaving the conversation.</p>
            </div>
            <div data-live-speech-panel></div>
        </div>`;
        if (typeof window.applyI18n === 'function') window.applyI18n(host);
        const panel = host.querySelector('[data-live-speech-panel]');
        const unmount = window.AuraRealtimeSpeechUI.mount(panel, {
            surface: 'desktop',
            compact: false,
            chatSessionId: 'virtual-desktop'
        });
        instances.set(String(windowId), { host, unmount });
    }

    function dispose(windowId, options) {
        const key = String(windowId || '');
        const instance = instances.get(key);
        if (instance && typeof instance.unmount === 'function') {
            try { instance.unmount(); } catch (_) { }
        }
        instances.delete(key);
        if (!(options && options.keepSession) && window.AuraRealtimeSpeech && window.AuraRealtimeSpeech.sessionId) {
            void window.AuraRealtimeSpeech.stop();
        }
    }

    window.LiveSpeechApp = { render, dispose };
})();
