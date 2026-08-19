(function () {
    'use strict';

    const instances = new Map();
    const FX_STORAGE_KEY = 'aurago.desktop.livespeech.fx';

    function fxEnabledDefault() {
        try { return localStorage.getItem(FX_STORAGE_KEY) !== '0'; } catch (_) { return true; }
    }

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
            <canvas class="vd-live-speech-fx" aria-hidden="true"></canvas>
            <div class="vd-live-speech-content">
                <header class="vd-live-speech-header">
                    <h2 data-i18n="desktop.live_speech_title">Live Speech</h2>
                    <button type="button" class="vd-live-speech-fx-toggle" data-live-speech-fx-toggle aria-pressed="true">
                        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3l1.7 4.6L18 9.3l-4.3 1.7L12 15.6l-1.7-4.6L6 9.3l4.3-1.7z"/><path d="M18.5 14.5l.9 2.3 2.3.9-2.3.9-.9 2.3-.9-2.3-2.3-.9 2.3-.9z"/></svg>
                    </button>
                </header>
                <div class="vd-live-speech-lab" data-live-speech-lab hidden>
                    <p data-live-speech-lab-status></p>
                    <div class="vd-live-speech-lab-actions">
                        <button type="button" class="wh-btn wh-btn-sm" data-live-speech-lab-start hidden></button>
                        <button type="button" class="wh-btn wh-btn-sm" data-live-speech-lab-activate hidden></button>
                        <a href="/config#speech_lab" data-live-speech-lab-config></a>
                    </div>
                </div>
                <div data-live-speech-panel></div>
            </div>
        </div>`;
        if (typeof window.applyI18n === 'function') window.applyI18n(host);

        const canvas = host.querySelector('.vd-live-speech-fx');
        let fxEnabled = fxEnabledDefault();
        const fx = window.LiveSpeechFX
            ? window.LiveSpeechFX.create({ canvas, runtime: window.AuraRealtimeSpeech, enabled: fxEnabled })
            : null;
        const fxToggle = host.querySelector('[data-live-speech-fx-toggle]');
        if (fxToggle) {
            fxToggle.title = text('desktop.live_speech_fx_toggle', 'Background effects');
            fxToggle.setAttribute('aria-label', text('desktop.live_speech_fx_toggle', 'Background effects'));
            fxToggle.setAttribute('aria-pressed', String(fxEnabled));
            fxToggle.addEventListener('click', () => {
                fxEnabled = !fxEnabled;
                try { localStorage.setItem(FX_STORAGE_KEY, fxEnabled ? '1' : '0'); } catch (_) { }
                fxToggle.setAttribute('aria-pressed', String(fxEnabled));
                if (fx) fx.setEnabled(fxEnabled);
            });
        }

        const panel = host.querySelector('[data-live-speech-panel]');
        const unmount = window.AuraRealtimeSpeechUI.mount(panel, {
            surface: 'desktop',
            compact: true,
            chatSessionId: 'virtual-desktop'
        });
        const stopLab = bindSpeechLabStatus(host);
        instances.set(String(windowId), { host, unmount, stopLab, fx });
    }

    function text(key, fallback) {
        const value = typeof window.t === 'function' ? window.t(key) : '';
        return value && value !== key ? value : (fallback || key);
    }

    function bindSpeechLabStatus(host) {
        const box = host.querySelector('[data-live-speech-lab]');
        const status = host.querySelector('[data-live-speech-lab-status]');
        const start = host.querySelector('[data-live-speech-lab-start]');
        const activate = host.querySelector('[data-live-speech-lab-activate]');
        const configLink = host.querySelector('[data-live-speech-lab-config]');
        if (!box || !status) return function () {};
        if (configLink) configLink.textContent = text('desktop.live_speech_lab_open_config', 'Speech Lab settings');
        if (start) start.textContent = text('desktop.live_speech_lab_start', 'Start Speech Lab container');
        if (activate) activate.textContent = text('desktop.live_speech_lab_activate', 'Use in Live Speech');
        let timer = 0;
        let starting = false;
        let activating = false;

        async function refresh() {
            try {
                const [response, realtime] = await Promise.all([
                    fetch('/api/speech-lab/status', { credentials: 'same-origin', cache: 'no-store' }),
                    window.AuraRealtimeSpeech.initialize().catch(() => null)
                ]);
                const data = response.ok ? await response.json() : {};
                box.hidden = false;
                const managed = !!(data.deployment && data.deployment.managed);
                const deployState = String((data.deployment && data.deployment.state) || '');
                const realtimeConfig = realtime && realtime.config;
                const selectable = !!(realtimeConfig && realtimeConfig.enabled &&
                    (realtimeConfig.profiles || []).some(profile => profile.enabled && profile.provider === 'speech_lab'));
                if (!data.enabled) {
                    status.textContent = text('desktop.live_speech_lab_disabled', 'Speech Lab is disabled. Enable it under Media → Speech Lab.');
                    if (start) start.hidden = true;
                    if (activate) activate.hidden = true;
                    return;
                }
                if (data.ready && data.asr_ok && data.tts_ok) {
                    const voice = data.voice ? ' · ' + data.voice : '';
                    status.textContent = text('desktop.live_speech_lab_ready', 'Speech Lab container is ready') + voice;
                    if (start) start.hidden = true;
                    if (activate) activate.hidden = selectable || activating;
                    return;
                }
                status.textContent = starting
                    ? text('desktop.live_speech_lab_starting', 'Starting the Speech Lab container…')
                    : text('desktop.live_speech_lab_not_ready', 'Speech Lab is not ready. Start the container or check Media → Speech Lab.');
                if (start) start.hidden = !(managed && !starting && deployState !== 'running' && deployState !== 'starting');
                if (activate) activate.hidden = true;
            } catch (_) {
                box.hidden = false;
                status.textContent = text('desktop.live_speech_lab_not_ready', 'Speech Lab is not ready. Start the container or check Media → Speech Lab.');
                if (activate) activate.hidden = true;
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
        if (activate) {
            activate.addEventListener('click', async () => {
                activating = true;
                activate.hidden = true;
                try {
                    const response = await fetch('/api/realtime-speech/speech-lab/activate', {
                        method: 'POST',
                        credentials: 'same-origin',
                        headers: { 'Content-Type': 'application/json' },
                        body: '{}'
                    });
                    if (!response.ok) {
                        const body = await response.json().catch(() => ({}));
                        throw new Error(body.message || body.error || ('HTTP ' + response.status));
                    }
                    const body = await response.json();
                    await window.AuraRealtimeSpeech.initialize(true);
                    if (window.AuraRealtimeSpeechUI && typeof window.AuraRealtimeSpeechUI.refresh === 'function') {
                        window.AuraRealtimeSpeechUI.refresh();
                    }
                    const profile = host.querySelector('[data-realtime-profile]');
                    if (profile && body.profile_id) profile.value = body.profile_id;
                } catch (error) {
                    status.textContent = error.message || text('desktop.live_speech_lab_not_ready', 'Speech Lab is not ready.');
                } finally {
                    activating = false;
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
        if (instance && instance.fx && typeof instance.fx.dispose === 'function') {
            try { instance.fx.dispose(); } catch (_) { }
        }
        instances.delete(key);
        if (!(options && options.keepSession) && window.AuraRealtimeSpeech && window.AuraRealtimeSpeech.sessionId) {
            void window.AuraRealtimeSpeech.stop();
        }
    }

    window.LiveSpeechApp = { render, dispose };
})();
