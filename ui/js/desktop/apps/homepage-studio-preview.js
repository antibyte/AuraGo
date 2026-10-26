/* Homepage Studio workbench chrome module: the preview zone frame (device width
   switcher, fullscreen) and the window layout (panel splitters, collapsed
   state, scaled device viewport). The iframe itself (sandbox, src, URL
   validation) is owned by homepage-studio.js — this module only shapes the
   frame around it. */
(function () {
    'use strict';

    const DEVICES = ['desktop', 'tablet', 'mobile'];
    const DEVICE_WIDTHS = { desktop: 1280, tablet: 768, mobile: 390 };
    const MIN_PANEL_WIDTH = 240;
    const MAX_PANEL_WIDTH = 560;

    function clampPanelWidth(px) {
        return Math.max(MIN_PANEL_WIDTH, Math.min(MAX_PANEL_WIDTH, px));
    }

    function create(deps) {
        const root = deps.root;
        const zone = deps.zone;
        const stage = deps.stage;
        const state = deps.state || {};
        const t = deps.t;
        const onDeviceChange = typeof deps.onDeviceChange === 'function' ? deps.onDeviceChange : null;
        const onLayoutChange = typeof deps.onLayoutChange === 'function' ? deps.onLayoutChange : null;
        const listeners = [];
        let resizeObserver = null;

        function on(target, event, handler, options) {
            if (!target) return;
            target.addEventListener(event, handler, options);
            listeners.push(() => target.removeEventListener(event, handler, options));
        }

        function deviceButtons() {
            return Array.from(root.querySelectorAll('[data-hp-device]'));
        }

        function currentDevice() {
            return DEVICES.includes(state.device) ? state.device : 'desktop';
        }

        function syncDeviceButtons() {
            const active = currentDevice();
            deviceButtons().forEach(btn => {
                const isActive = btn.getAttribute('data-hp-device') === active;
                btn.classList.toggle('active', isActive);
                btn.setAttribute('aria-checked', isActive ? 'true' : 'false');
            });
        }

        function setDevice(next) {
            if (!DEVICES.includes(next)) next = 'desktop';
            state.device = next;
            if (stage) stage.dataset.device = next;
            syncDeviceButtons();
            applyDeviceViewport();
            if (onDeviceChange) onDeviceChange(next);
        }

        /* The previewed page renders at the real device width and is scaled into
           the pane, so every device produces a distinct viewport. */
        function applyDeviceViewport() {
            const frame = root.querySelector('.vd-hp-preview-frame');
            const panel = root.querySelector('.vd-hp-preview-panel');
            const label = root.querySelector('.vd-hp-device-width');
            if (!frame || !panel) return;
            const deviceWidth = DEVICE_WIDTHS[currentDevice()];
            const pane = panel.getBoundingClientRect();
            if (!pane.width || !pane.height) return;
            const scale = Math.min(1, pane.width / deviceWidth);
            frame.style.setProperty('--hp-device-w', deviceWidth + 'px');
            frame.style.setProperty('--hp-device-h', (pane.height / scale) + 'px');
            frame.style.setProperty('--hp-device-scale', String(scale));
            if (label) label.textContent = t('homepage_studio.device_width_value', { width: deviceWidth });
        }

        function panelWidth(side) {
            return side === 'chat'
                ? (state.chatWidth || 320)
                : (state.inspectorWidth || 300);
        }

        function applyWidths() {
            const chatSplitter = root.querySelector('[data-hp-splitter="chat"]');
            const inspectorSplitter = root.querySelector('[data-hp-splitter="inspector"]');
            if (state.chatWidth) root.style.setProperty('--hp-chat-w', state.chatWidth + 'px');
            if (state.inspectorWidth) root.style.setProperty('--hp-inspector-w', state.inspectorWidth + 'px');
            if (chatSplitter) chatSplitter.setAttribute('aria-valuenow', String(panelWidth('chat')));
            if (inspectorSplitter) inspectorSplitter.setAttribute('aria-valuenow', String(panelWidth('inspector')));
            applyDeviceViewport();
        }

        function setCollapsed(side, collapsed) {
            const isChat = side === 'chat';
            if (isChat) state.chatCollapsed = collapsed; else state.inspectorCollapsed = collapsed;
            root.classList.toggle('chat-collapsed', !!state.chatCollapsed);
            root.classList.toggle('inspector-collapsed', !!state.inspectorCollapsed);
            const toggle = root.querySelector(isChat ? '[data-hp-toggle="chat"]' : '[data-hp-toggle="inspector"]');
            if (toggle) {
                toggle.classList.toggle('active', !collapsed);
                toggle.setAttribute('aria-expanded', collapsed ? 'false' : 'true');
            }
            if (onLayoutChange) onLayoutChange();
        }

        function wireSplitter(splitter, side) {
            if (!splitter) return;
            const main = root.querySelector('.vd-hp-main');
            on(splitter, 'pointerdown', event => {
                if (event.button !== 0 || !main) return;
                event.preventDefault();
                splitter.setPointerCapture(event.pointerId);
                splitter.classList.add('dragging');
                const mainRect = main.getBoundingClientRect();
                const onMove = moveEvent => {
                    const px = side === 'chat'
                        ? moveEvent.clientX - mainRect.left
                        : mainRect.right - moveEvent.clientX;
                    const width = clampPanelWidth(px);
                    if (side === 'chat') state.chatWidth = width; else state.inspectorWidth = width;
                    applyWidths();
                };
                const onUp = () => {
                    splitter.classList.remove('dragging');
                    splitter.removeEventListener('pointermove', onMove);
                    splitter.removeEventListener('pointerup', onUp);
                    splitter.removeEventListener('pointercancel', onUp);
                    if (onLayoutChange) onLayoutChange();
                };
                splitter.addEventListener('pointermove', onMove);
                splitter.addEventListener('pointerup', onUp);
                splitter.addEventListener('pointercancel', onUp);
            });
            on(splitter, 'dblclick', () => {
                setCollapsed(side, side === 'chat' ? !state.chatCollapsed : !state.inspectorCollapsed);
            });
            on(splitter, 'keydown', event => {
                if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
                event.preventDefault();
                const delta = event.key === 'ArrowRight' ? 28 : -28;
                const next = side === 'chat' ? panelWidth('chat') + delta : panelWidth('inspector') - delta;
                const width = clampPanelWidth(next);
                if (side === 'chat') state.chatWidth = width; else state.inspectorWidth = width;
                applyWidths();
                if (onLayoutChange) onLayoutChange();
            });
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

        /* The device switcher is a single-select radiogroup: arrow keys move it. */
        const deviceGroup = root.querySelector('.vd-hp-device-seg');
        if (deviceGroup) {
            on(deviceGroup, 'keydown', event => {
                if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight' && event.key !== 'Home' && event.key !== 'End') return;
                const buttons = deviceButtons();
                const index = buttons.indexOf(document.activeElement);
                if (index < 0) return;
                event.preventDefault();
                let next = index;
                if (event.key === 'ArrowRight') next = (index + 1) % buttons.length;
                else if (event.key === 'ArrowLeft') next = (index - 1 + buttons.length) % buttons.length;
                else if (event.key === 'Home') next = 0;
                else next = buttons.length - 1;
                const target = buttons[next];
                if (!target) return;
                setDevice(target.getAttribute('data-hp-device'));
                target.focus();
            });
        }

        wireSplitter(root.querySelector('[data-hp-splitter="chat"]'), 'chat');
        wireSplitter(root.querySelector('[data-hp-splitter="inspector"]'), 'inspector');

        const fsBtn = fullscreenButton();
        if (fsBtn) on(fsBtn, 'click', toggleFullscreen);
        on(document, 'fullscreenchange', syncFullscreenButton);

        if (stage) stage.dataset.device = currentDevice();
        syncDeviceButtons();
        syncFullscreenButton();
        applyWidths();

        const panel = root.querySelector('.vd-hp-preview-panel');
        if (panel && typeof window.ResizeObserver === 'function') {
            resizeObserver = new window.ResizeObserver(() => applyDeviceViewport());
            resizeObserver.observe(panel);
        }

        return {
            setDevice,
            setCollapsed,
            applyWidths,
            applyDeviceViewport,
            toggleFullscreen,
            getDevice() { return currentDevice(); },
            dispose() {
                listeners.forEach(off => off());
                listeners.length = 0;
                if (resizeObserver) {
                    resizeObserver.disconnect();
                    resizeObserver = null;
                }
                if (document.fullscreenElement === zone && document.exitFullscreen) {
                    document.exitFullscreen().catch(() => {});
                }
            }
        };
    }

    window.HomepageStudioPreview = { create, DEVICES, DEVICE_WIDTHS, clampPanelWidth, MIN_PANEL_WIDTH, MAX_PANEL_WIDTH };
})();
