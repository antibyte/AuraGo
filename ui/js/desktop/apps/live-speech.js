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
            <div class="vd-live-speech-lab" data-live-speech-lab hidden>
                <p data-live-speech-lab-status></p>
                <div class="vd-live-speech-lab-actions">
                    <button type="button" class="wh-btn wh-btn-sm" data-live-speech-lab-start hidden></button>
                    <a href="/config#speech_lab" data-live-speech-lab-config></a>
                </div>
            </div>
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
        const stopLab = bindSpeechLabStatus(host);
        instances.set(String(windowId), { host, unmount, stopLab });
    }

    function text(key, fallback) {
        const value = typeof window.t === 'function' ? window.t(key) : '';
        return value && value !== key ? value : (fallback || key);
    }

    function bindSpeechLabStatus(host) {
        const box = host.querySelector('[data-live-speech-lab]');
        const status = host.querySelector('[data-live-speech-lab-status]');
        const start = host.querySelector('[data-live-speech-lab-start]');
        const configLink = host.querySelector('[data-live-speech-lab-config]');
        if (!box || !status) return function () {};
        if (configLink) configLink.textContent = text('desktop.live_speech_lab_open_config', 'Speech Lab settings');
        if (start) start.textContent = text('desktop.live_speech_lab_start', 'Start Speech Lab container');
        let timer = 0;
        let starting = false;

        async function refresh() {
            try {
                const response = await fetch('/api/speech-lab/status', { credentials: 'same-origin', cache: 'no-store' });
                const data = response.ok ? await response.json() : {};
                box.hidden = false;
                const managed = !!(data.deployment && data.deployment.managed);
                const deployState = String((data.deployment && data.deployment.state) || '');
                if (!data.enabled) {
                    status.textContent = text('desktop.live_speech_lab_disabled', 'Speech Lab is disabled. Enable it under Media → Speech Lab.');
                    if (start) start.hidden = true;
                    return;
                }
                if (data.ready && data.asr_ok && data.tts_ok) {
                    const voice = data.voice ? ' · ' + data.voice : '';
                    status.textContent = text('desktop.live_speech_lab_ready', 'Speech Lab container is ready') + voice;
                    if (start) start.hidden = true;
                    return;
                }
                status.textContent = starting
                    ? text('desktop.live_speech_lab_starting', 'Starting the Speech Lab container…')
                    : text('desktop.live_speech_lab_not_ready', 'Speech Lab is not ready. Start the container or check Media → Speech Lab.');
                if (start) start.hidden = !(managed && !starting && deployState !== 'running' && deployState !== 'starting');
            } catch (_) {
                box.hidden = false;
                status.textContent = text('desktop.live_speech_lab_not_ready', 'Speech Lab is not ready. Start the container or check Media → Speech Lab.');
            }
        }

        if (start) {
            start.addEventListener('click', async () => {
                starting = true;
                start.hidden = true;
                status.textContent = text('desktop.live_speech_lab_starting', 'Starting the Speech Lab container…');
                try {
                    const response = await fetch('/api/speech-lab/deployment/start', {
                        method: 'POST',
                        credentials: 'same-origin',
                        headers: { 'Content-Type': 'application/json' },
                        body: '{}'
                    });
                    if (!response.ok) {
                        const body = await response.json().catch(() => ({}));
                        throw new Error(body.message || body.error || ('HTTP ' + response.status));
                    }
                } catch (error) {
                    status.textContent = error.message || text('desktop.live_speech_lab_not_ready', 'Speech Lab is not ready.');
                } finally {
                    starting = false;
                    void refresh();
                }
            });
        }
        void refresh();
        timer = window.setInterval(() => void refresh(), 8000);
        return function () {
            window.clearInterval(timer);
        };
    }

    function dispose(windowId, options) {
        const key = String(windowId || '');
        const instance = instances.get(key);
        if (instance && typeof instance.stopLab === 'function') {
            try { instance.stopLab(); } catch (_) { }
        }
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
