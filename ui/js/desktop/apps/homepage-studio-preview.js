/* Homepage Studio preview chrome module: device width switcher and fullscreen
   for the preview zone. The iframe itself (sandbox, src, URL validation) is
   owned by homepage-studio.js — this module only shapes the frame around it. */
(function () {
    'use strict';

    const DEVICES = ['desktop', 'tablet', 'mobile'];

    function create(deps) {
        const root = deps.root;
        const zone = deps.zone;
        const stage = deps.stage;
        const onDeviceChange = typeof deps.onDeviceChange === 'function' ? deps.onDeviceChange : null;
        const listeners = [];
        let device = 'desktop';

        function on(target, event, handler, options) {
            if (!target) return;
            target.addEventListener(event, handler, options);
            listeners.push(() => target.removeEventListener(event, handler, options));
        }

        function deviceButtons() {
            return Array.from(root.querySelectorAll('[data-hp-device]'));
        }

        function syncDeviceButtons() {
            deviceButtons().forEach(btn => {
                const active = btn.getAttribute('data-hp-device') === device;
                btn.classList.toggle('active', active);
                btn.setAttribute('aria-pressed', active ? 'true' : 'false');
            });
        }

        function setDevice(next) {
            if (!DEVICES.includes(next)) next = 'desktop';
            device = next;
            if (stage) stage.dataset.device = next;
            syncDeviceButtons();
            if (onDeviceChange) onDeviceChange(next);
        }

        function fullscreenButton() {
            return root.querySelector('[data-hp-fullscreen]');
        }

        function syncFullscreenButton() {
            const btn = fullscreenButton();
            if (!btn) return;
            const active = document.fullscreenElement === zone;
            btn.classList.toggle('active', active);
            btn.setAttribute('aria-pressed', active ? 'true' : 'false');
        }

        function toggleFullscreen() {
            if (!zone || !zone.requestFullscreen) return;
            if (document.fullscreenElement === zone) {
                if (document.exitFullscreen) document.exitFullscreen().catch(() => {});
            } else {
                zone.requestFullscreen().catch(() => {});
            }
        }

        deviceButtons().forEach(btn => {
            on(btn, 'click', () => setDevice(btn.getAttribute('data-hp-device')));
        });

        const fsBtn = fullscreenButton();
        if (fsBtn) on(fsBtn, 'click', toggleFullscreen);
        on(document, 'fullscreenchange', syncFullscreenButton);

        if (stage) stage.dataset.device = device;
        syncDeviceButtons();
        syncFullscreenButton();

        return {
            setDevice,
            toggleFullscreen,
            getDevice() { return device; },
            dispose() {
                listeners.forEach(off => off());
                listeners.length = 0;
                if (document.fullscreenElement === zone && document.exitFullscreen) {
                    document.exitFullscreen().catch(() => {});
                }
            }
        };
    }

    window.HomepageStudioPreview = { create, DEVICES };
})();
