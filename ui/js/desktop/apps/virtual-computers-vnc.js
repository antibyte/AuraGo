(function () {
    'use strict';

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>'"]/g, ch => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
        }[ch]));
    }

    function applyVNCPreferences(rfb, preferences) {
        const fit = preferences.scaleMode === 'fit';
        rfb.viewOnly = preferences.viewOnly === true;
        rfb.scaleViewport = fit;
        rfb.resizeSession = fit;
    }

    function mount(container, options) {
        if (!container) throw new Error('VNC container is required');
        const settings = options || {};
        const t = typeof settings.t === 'function' ? settings.t : key => key;
        const notify = typeof settings.notify === 'function' ? settings.notify : function () {};
        const toggleMaximize = typeof settings.toggleMaximize === 'function' ? settings.toggleMaximize : null;
        const onClose = typeof settings.onClose === 'function' ? settings.onClose : function () {};
        let rfb = null;
        let lastError = '';
        let appMaximized = false;
        let fullscreenListening = false;
        const preferences = { viewOnly: false, scaleMode: 'fit' };

        container.classList.add('vc-vnc-session');
        container.dataset.machineId = String(settings.machineId || '');
        container.innerHTML = `
            <div class="vc-vnc-toolbar" role="toolbar" aria-label="${esc(t('desktop.virtual_computers_vnc_live'))}">
                <span class="vc-vnc-status" data-role="vnc-status" aria-live="polite"></span>
                <div class="vc-vnc-toolbar-group">
                    <button type="button" class="vc-vnc-tool active" data-vnc-scale="fit" aria-pressed="true">${esc(t('desktop.virtual_computers_vnc_fit'))}</button>
                    <button type="button" class="vc-vnc-tool" data-vnc-scale="one-to-one" aria-pressed="false">${esc(t('desktop.virtual_computers_vnc_one_to_one'))}</button>
                    <button type="button" class="vc-vnc-tool" data-vnc-action="view-only" aria-pressed="false">${esc(t('desktop.virtual_computers_vnc_view_only'))}</button>
                    <button type="button" class="vc-vnc-tool" data-vnc-action="ctrl-alt-del">${esc(t('desktop.virtual_computers_vnc_ctrl_alt_del'))}</button>
                </div>
                <div class="vc-vnc-toolbar-group vc-vnc-session-actions">
                    <button type="button" class="vc-vnc-tool" data-vnc-action="reconnect">${esc(t('desktop.virtual_computers_vnc_reconnect'))}</button>
                    <button type="button" class="vc-vnc-tool" data-vnc-action="disconnect">${esc(t('desktop.virtual_computers_vnc_disconnect'))}</button>
                    ${toggleMaximize ? `<button type="button" class="vc-vnc-tool" data-vnc-action="maximize" aria-pressed="false">${esc(t('desktop.virtual_computers_vnc_maximize'))}</button>` : ''}
                    <button type="button" class="vc-vnc-tool" data-vnc-action="fullscreen" aria-pressed="false">${esc(t('desktop.virtual_computers_vnc_fullscreen'))}</button>
                </div>
            </div>
            <div class="vc-vnc-stage">
                <div class="vc-vnc-canvas" data-role="vnc-canvas"></div>
                <div class="vc-vnc-message" data-role="vnc-message" hidden></div>
            </div>`;

        const canvas = container.querySelector('[data-role="vnc-canvas"]');
        const status = container.querySelector('[data-role="vnc-status"]');
        const message = container.querySelector('[data-role="vnc-message"]');
        const maximizeButton = container.querySelector('[data-vnc-action="maximize"]');
        const fullscreenButton = container.querySelector('[data-vnc-action="fullscreen"]');

        function updateMaximizeButton() {
            if (!maximizeButton) return;
            const appWindow = container.closest('.vd-window');
            if (appWindow) appMaximized = appWindow.classList.contains('maximized');
            maximizeButton.classList.toggle('active', appMaximized);
            maximizeButton.setAttribute('aria-pressed', appMaximized ? 'true' : 'false');
            maximizeButton.textContent = t(appMaximized ? 'desktop.virtual_computers_vnc_restore' : 'desktop.virtual_computers_vnc_maximize');
        }

        function setStatus(key, type, detail) {
            const label = t(key);
            status.textContent = detail ? label + ': ' + detail : label;
            status.dataset.state = type || '';
        }

        function setMessage(text) {
            message.textContent = text || '';
            message.hidden = !text;
        }

        function updateFullscreenButton() {
            const active = document.fullscreenElement === container;
            if (!fullscreenButton) return;
            fullscreenButton.classList.toggle('active', active);
            fullscreenButton.setAttribute('aria-pressed', active ? 'true' : 'false');
            fullscreenButton.textContent = t(active ? 'desktop.virtual_computers_vnc_exit_fullscreen' : 'desktop.virtual_computers_vnc_fullscreen');
        }

        function listenForFullscreen() {
            if (fullscreenListening) return;
            document.addEventListener('fullscreenchange', updateFullscreenButton);
            fullscreenListening = true;
        }

        function stopListeningForFullscreen() {
            if (!fullscreenListening) return;
            document.removeEventListener('fullscreenchange', updateFullscreenButton);
            fullscreenListening = false;
        }

        function disconnectConnection() {
            const current = rfb;
            rfb = null;
            if (current && typeof current.disconnect === 'function') {
                try { current.disconnect(); } catch (_) {}
            }
        }

        function fail(key) {
            lastError = t(key);
            setStatus(key, 'error');
            setMessage(lastError);
            notify(lastError, 'error');
            disconnectConnection();
        }

        function connect() {
            disconnectConnection();
            stopListeningForFullscreen();
            listenForFullscreen();
            lastError = '';
            setMessage('');
            setStatus('desktop.virtual_computers_vnc_connecting', 'connecting');
            canvas.replaceChildren();
            if (!window.RFB) {
                fail('desktop.virtual_computers_vnc_unavailable');
                return;
            }
            try {
                const nextRFB = new window.RFB(canvas, settings.url, { wsProtocols: ['binary'] });
                rfb = nextRFB;
                applyVNCPreferences(nextRFB, preferences);
                nextRFB.addEventListener('connect', () => {
                    if (rfb !== nextRFB) return;
                    lastError = '';
                    setMessage('');
                    setStatus('desktop.virtual_computers_vnc_connected', 'connected');
                });
                nextRFB.addEventListener('disconnect', () => {
                    if (rfb !== nextRFB) return;
                    rfb = null;
                    if (lastError) {
                        setStatus('desktop.virtual_computers_vnc_security_error', 'error', lastError);
                        setMessage(lastError);
                        return;
                    }
                    setStatus('desktop.virtual_computers_vnc_disconnected', 'disconnected');
                });
                nextRFB.addEventListener('credentialsrequired', () => {
                    if (rfb === nextRFB) fail('desktop.virtual_computers_vnc_credentials_error');
                });
                nextRFB.addEventListener('securityfailure', () => {
                    if (rfb === nextRFB) fail('desktop.virtual_computers_vnc_security_error');
                });
            } catch (_) {
                fail('desktop.virtual_computers_vnc_unavailable');
            }
        }

        container.querySelectorAll('[data-vnc-scale]').forEach(button => {
            button.addEventListener('click', () => {
                if (!rfb) return;
                const fit = button.dataset.vncScale === 'fit';
                preferences.scaleMode = fit ? 'fit' : 'one-to-one';
                applyVNCPreferences(rfb, preferences);
                container.querySelectorAll('[data-vnc-scale]').forEach(item => {
                    const active = item === button;
                    item.classList.toggle('active', active);
                    item.setAttribute('aria-pressed', active ? 'true' : 'false');
                });
            });
        });

        container.querySelector('[data-vnc-action="view-only"]')?.addEventListener('click', event => {
            if (!rfb) return;
            preferences.viewOnly = !preferences.viewOnly;
            applyVNCPreferences(rfb, preferences);
            event.currentTarget.classList.toggle('active', preferences.viewOnly);
            event.currentTarget.setAttribute('aria-pressed', preferences.viewOnly ? 'true' : 'false');
        });
        container.querySelector('[data-vnc-action="ctrl-alt-del"]')?.addEventListener('click', () => {
            if (rfb && typeof rfb.sendCtrlAltDel === 'function') rfb.sendCtrlAltDel();
        });
        container.querySelector('[data-vnc-action="reconnect"]')?.addEventListener('click', () => connect());
        container.querySelector('[data-vnc-action="disconnect"]')?.addEventListener('click', () => {
            disconnectConnection();
            setStatus('desktop.virtual_computers_vnc_disconnected', 'disconnected');
            onClose();
        });
        maximizeButton?.addEventListener('click', () => {
            toggleMaximize();
            appMaximized = !appMaximized;
            updateMaximizeButton();
        });
        fullscreenButton?.addEventListener('click', () => {
            if (document.fullscreenElement === container) {
                if (typeof document.exitFullscreen === 'function') {
                    const exiting = document.exitFullscreen();
                    if (exiting && typeof exiting.catch === 'function') exiting.catch(function () {});
                }
                return;
            }
            if (typeof container.requestFullscreen === 'function') {
                const entering = container.requestFullscreen();
                if (entering && typeof entering.catch === 'function') entering.catch(function () {});
            }
        });

        function disconnect() {
            disconnectConnection();
            stopListeningForFullscreen();
            if (document.fullscreenElement === container && typeof document.exitFullscreen === 'function') {
                const exiting = document.exitFullscreen();
                if (exiting && typeof exiting.catch === 'function') exiting.catch(function () {});
            }
        }

        function reconnect() {
            connect();
        }

        updateMaximizeButton();
        connect();
        return { disconnect, reconnect };
    }

    window.VirtualComputersVNC = { mount };
}());
