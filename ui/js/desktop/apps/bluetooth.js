(function () {
    'use strict';

    const API = '/api/desktop/integrations/bluetooth/';
    const VISIBLE_MINUTES = [1, 3, 5, 10];
    const INPUT_KINDS = new Set(['enter_passkey', 'enter_pin']);
    const DISPLAY_KINDS = new Set(['display_passkey', 'display_pin']);
    const instances = new Map();

    function render(host, windowId, ctx) {
        dispose(windowId);
        const views = window.BluetoothViews;
        const { esc, t } = ctx;
        const s = {
            disposed: false, data: null, revision: -1, fetchedAt: 0, loading: false, again: false,
            unnamedOpen: false, menuFor: '', visibleMenu: false,
            localErrors: new Map(), dismissed: new Set(),
            interaction: null, interactionAt: 0, dismissedInteraction: '', ticker: 0
        };
        host.innerHTML = views.shell(esc, t);
        const root = host.querySelector('.bt-app');
        const part = name => root.querySelector(`[data-bt="${name}"]`);
        const request = (path, options) => ctx.api(API + path, options);
        const post = (path, body) => request(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body || {}) });
        const errorText = err => views.errorFor(t, err && err.body && err.body.code);
        const elapsed = () => Math.floor((Date.now() - s.fetchedAt) / 1000);
        const devices = () => (s.data && Array.isArray(s.data.devices) ? s.data.devices : []);

        function banner(message) {
            const el = part('banner');
            el.textContent = message || '';
            el.hidden = !message;
        }

        function paint() {
            const data = s.data || {};
            const adapter = data.adapter || {};
            root.dataset.state = !data.present ? 'unavailable' : (adapter.powered ? 'ready' : 'off');
            const options = {
                elapsed: elapsed(), visibleMenu: s.visibleMenu, minutes: VISIBLE_MINUTES, unnamedOpen: s.unnamedOpen,
                menuFor: s.menuFor, localErrors: s.localErrors, dismissed: s.dismissed,
                audioUsable: !!(data.status && data.status.audio && data.status.audio.usable)
            };
            part('adapter').innerHTML = views.adapterBar(esc, t, data, options);
            part('body').innerHTML = views.body(esc, t, data, options);
            schedule();
        }

        // Each fetch returns the server's current state; fetches never overlap,
        // so the newest response always wins.
        async function refresh() {
            if (s.disposed) return;
            if (s.loading) { s.again = true; return; }
            s.loading = true;
            try {
                const data = await request('status');
                if (s.disposed) return;
                s.data = data;
                s.revision = typeof data.revision === 'number' ? data.revision : 0;
                s.fetchedAt = Date.now();
                banner('');
                paint();
                await syncInteraction(data.interaction_id || '');
            } catch (err) {
                if (!s.disposed) banner(errorText(err));
            } finally {
                s.loading = false;
                if (s.again && !s.disposed) { s.again = false; refresh(); }
            }
        }

        async function syncInteraction(id) {
            if (!id) { closeDialog(); return; }
            if (id === s.dismissedInteraction) return;
            if (s.interaction && s.interaction.id === id && INPUT_KINDS.has(s.interaction.kind)) return;
            try {
                const result = await request('interactions/' + encodeURIComponent(id));
                if (!s.disposed && result && result.interaction) openDialog(result.interaction);
            } catch (err) {
                closeDialog();
            }
        }

        function openDialog(view) {
            const layer = part('dialog');
            const keepInput = s.interaction && s.interaction.id === view.id && INPUT_KINDS.has(view.kind);
            s.interaction = view;
            s.interactionAt = Date.now();
            if (!keepInput) {
                layer.innerHTML = views.dialog(esc, t, view);
                layer.hidden = false;
                const focus = layer.querySelector('input') || layer.querySelector('.bt-primary') || layer.querySelector('button');
                if (focus) focus.focus();
            }
            tick();
            schedule();
        }

        function closeDialog() {
            s.interaction = null;
            const layer = part('dialog');
            layer.hidden = true;
            layer.innerHTML = '';
        }

        async function answer(accept) {
            const view = s.interaction;
            if (!view) return;
            let value = '';
            const input = part('dialog').querySelector('input');
            if (accept && input) {
                value = input.value.trim();
                const valid = view.kind === 'enter_passkey' ? /^\d{1,6}$/.test(value) : /^[\x20-\x7e]{1,16}$/.test(value);
                if (!valid) {
                    part('dialog').querySelector('[data-bt="dialog-error"]').textContent = view.kind === 'enter_passkey' ? t('bluetooth.invalid_passkey') : t('bluetooth.invalid_pin');
                    input.focus();
                    return;
                }
            }
            closeDialog();
            try {
                await post('interactions/' + encodeURIComponent(view.id), { accept, value });
            } catch (err) {
                banner(errorText(err));
            }
            refresh();
        }

        function schedule() {
            const data = s.data || {};
            const needed = (data.discovery && data.discovery.active) || (data.discoverable && data.discoverable.active) || !!s.interaction;
            if (needed && !s.ticker) s.ticker = setInterval(tick, 1000);
            if (!needed && s.ticker) { clearInterval(s.ticker); s.ticker = 0; }
        }

        function tick() {
            if (s.disposed) return;
            const data = s.data || {};
            const passed = elapsed();
            const scanLeft = views.remaining(data.discovery, passed);
            const visibleLeft = views.remaining(data.discoverable, passed);
            const scan = part('adapter').querySelector('[data-action="stop-scan"]');
            if (scan) scan.textContent = t('bluetooth.stop_scan', { seconds: scanLeft });
            const badge = part('adapter').querySelector('[data-bt="visible-badge"]');
            if (badge) badge.textContent = t('bluetooth.visible_for', { time: views.clock(visibleLeft) });
            if (s.interaction) {
                const total = Math.max(1, s.interaction.remaining_seconds || 20);
                const left = Math.max(0, total - Math.floor((Date.now() - s.interactionAt) / 1000));
                const expires = part('dialog').querySelector('[data-bt="expires"]');
                const bar = part('dialog').querySelector('[data-bt="progress"]');
                if (expires) expires.textContent = t('bluetooth.expires', { seconds: left });
                if (bar) bar.style.width = Math.round((left / 20) * 100) + '%';
                if (left === 0) { closeDialog(); refresh(); }
            }
            if ((data.discovery && data.discovery.active && scanLeft === 0) || (data.discoverable && data.discoverable.active && visibleLeft === 0)) refresh();
        }

        async function run(task, address) {
            try {
                await task();
            } catch (err) {
                if (address) { s.localErrors.set(address, errorText(err)); paint(); } else banner(errorText(err));
            }
            refresh();
        }

        async function deviceAction(op, address) {
            s.menuFor = '';
            s.localErrors.delete(address);
            if (op === 'remove') {
                const device = devices().find(d => d.address === address);
                const confirmed = await ctx.confirmDialog(t('bluetooth.remove_title', { name: views.deviceName(device) }), t('bluetooth.remove_message'));
                if (!confirmed || s.disposed) { paint(); return; }
            }
            return run(() => post('devices/action', { operation: op, address, wait: false, interactive: op === 'pair' }), address);
        }

        function act(action, el) {
            const address = el.dataset.address || '';
            switch (action) {
            case 'scan': return run(() => post('discovery', { action: 'start' }));
            case 'stop-scan': return run(() => post('discovery', { action: 'stop' }));
            case 'turn-on': return run(() => post('power', { powered: true }));
            case 'visible-menu': s.visibleMenu = !s.visibleMenu; s.menuFor = ''; return paint();
            case 'visible':
                s.visibleMenu = false;
                return run(() => post('discoverable', { enabled: true, duration_seconds: Number(el.dataset.minutes) * 60 }));
            case 'hide-visible': return run(() => post('discoverable', { enabled: false }));
            case 'menu': s.menuFor = s.menuFor === address ? '' : address; s.visibleMenu = false; return paint();
            case 'toggle-unnamed': s.unnamedOpen = !s.unnamedOpen; return paint();
            case 'dismiss-error': {
                s.localErrors.delete(address);
                const device = devices().find(d => d.address === address);
                if (device && device.error) s.dismissed.add(address + '|' + device.error);
                return paint();
            }
            case 'device': return deviceAction(el.dataset.op, address);
            case 'test-tone': return run(() => post('audio/test', { device: address }), address);
            case 'answer': return answer(el.dataset.accept === 'true');
            case 'close-dialog':
                s.dismissedInteraction = s.interaction ? s.interaction.id : '';
                return closeDialog();
            }
            return undefined;
        }

        function onClick(event) {
            const el = event.target.closest('[data-action]');
            if (!el || !root.contains(el)) {
                if (s.menuFor || s.visibleMenu) { s.menuFor = ''; s.visibleMenu = false; paint(); }
                return;
            }
            if (el.disabled || el.matches('input[data-action="power"]')) return;
            event.preventDefault();
            act(el.dataset.action, el);
        }

        function onChange(event) {
            const el = event.target;
            if (el.matches && el.matches('input[data-action="power"]')) run(() => post('power', { powered: el.checked }));
        }

        function onKeydown(event) {
            if (!s.interaction) return;
            if (event.key === 'Enter' && event.target.matches('[data-bt="dialog"] input')) {
                event.preventDefault();
                answer(true);
            } else if (event.key === 'Escape') {
                event.preventDefault();
                if (DISPLAY_KINDS.has(s.interaction.kind)) { s.dismissedInteraction = s.interaction.id; closeDialog(); } else answer(false);
            }
        }

        function onBluetoothChange(event) {
            const revision = event.detail && typeof event.detail.revision === 'number' ? event.detail.revision : null;
            if (revision === null || revision !== s.revision) refresh();
        }

        function onBluetoothInteraction() {
            refresh();
        }

        root.addEventListener('click', onClick);
        root.addEventListener('change', onChange);
        root.addEventListener('keydown', onKeydown);
        document.addEventListener('aurago:bluetooth-change', onBluetoothChange);
        document.addEventListener('aurago:bluetooth-interaction', onBluetoothInteraction);
        instances.set(windowId, () => {
            s.disposed = true;
            if (s.ticker) clearInterval(s.ticker);
            root.removeEventListener('click', onClick);
            root.removeEventListener('change', onChange);
            root.removeEventListener('keydown', onKeydown);
            document.removeEventListener('aurago:bluetooth-change', onBluetoothChange);
            document.removeEventListener('aurago:bluetooth-interaction', onBluetoothInteraction);
        });
        refresh();
    }

    function dispose(windowId) {
        const cleanup = instances.get(windowId);
        if (cleanup) cleanup();
        instances.delete(windowId);
    }

    window.BluetoothApp = { render, dispose };
})();
