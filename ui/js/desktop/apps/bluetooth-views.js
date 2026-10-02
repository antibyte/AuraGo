(function () {
    'use strict';

    const GLYPHS = {
        headphones: '<path d="M4 14a8 8 0 0 1 16 0"/><rect x="3" y="14" width="4" height="7" rx="1.5"/><rect x="17" y="14" width="4" height="7" rx="1.5"/>',
        headset: '<path d="M4 14a8 8 0 0 1 16 0"/><rect x="3" y="14" width="4" height="6" rx="1.5"/><rect x="17" y="14" width="4" height="6" rx="1.5"/><path d="M19 20c0 2-2 2-5 2"/>',
        speaker: '<rect x="6" y="3" width="12" height="18" rx="2"/><circle cx="12" cy="14" r="3"/><circle cx="12" cy="7.5" r="1"/>',
        keyboard: '<rect x="2" y="6" width="20" height="12" rx="2"/><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M7 14h10"/>',
        mouse: '<rect x="6" y="3" width="12" height="18" rx="6"/><path d="M12 7v4"/>',
        gamepad: '<path d="M6 9h12a4 4 0 0 1 4 4v1a3 3 0 0 1-5 2l-2-2H9l-2 2a3 3 0 0 1-5-2v-1a4 4 0 0 1 4-4z"/><path d="M8 11v4M6 13h4"/>',
        phone: '<rect x="7" y="2" width="10" height="20" rx="2"/><path d="M11 18h2"/>',
        computer: '<rect x="3" y="4" width="18" height="12" rx="1.5"/><path d="M8 20h8M12 16v4"/>',
        display: '<rect x="2" y="4" width="20" height="13" rx="1.5"/><path d="M9 21h6"/>',
        other: '<path d="M7 7l10 10-5 5V2l5 5L7 17"/>'
    };
    const OPERATION_KEYS = {
        pairing: 'bluetooth.op_pairing',
        connecting: 'bluetooth.op_connecting',
        disconnecting: 'bluetooth.op_disconnecting',
        removing: 'bluetooth.op_removing',
        trusting: 'bluetooth.op_trusting'
    };
    const PRIMARY_KEYS = { pair: 'bluetooth.pair', connect: 'bluetooth.connect', disconnect: 'bluetooth.disconnect' };

    function glyph(type) {
        return `<svg class="bt-glyph" viewBox="0 0 24 24" aria-hidden="true">${GLYPHS[type] || GLYPHS.other}</svg>`;
    }

    function deviceName(device) {
        return (device && (device.alias || device.name || device.address)) || '';
    }

    // BlueZ uses the address with dashes as alias when a device has no name.
    function isNamed(device) {
        const label = device.alias || device.name || '';
        return !!label && label.replace(/[-:]/g, '').toUpperCase() !== String(device.address || '').replace(/[-:]/g, '').toUpperCase();
    }

    function remaining(state, elapsedSeconds) {
        if (!state || !state.active) return 0;
        return Math.max(0, (state.remaining_seconds || 0) - elapsedSeconds);
    }

    function clock(seconds) {
        return Math.floor(seconds / 60) + ':' + String(seconds % 60).padStart(2, '0');
    }

    function errorFor(t, code) {
        if (code) {
            const key = 'bluetooth.error_' + code;
            const text = t(key);
            if (text && text !== key) return text;
        }
        return t('bluetooth.error_generic');
    }

    function signalLevel(rssi) {
        if (rssi >= -55) return 4;
        if (rssi >= -67) return 3;
        if (rssi >= -80) return 2;
        return 1;
    }

    function shell(esc, t) {
        return `<section class="bt-app" data-state="loading" aria-label="${esc(t('desktop.app_bluetooth'))}">
            <header class="bt-adapter" data-bt="adapter"><p class="bt-muted">${esc(t('bluetooth.loading'))}</p></header>
            <div class="bt-banner" data-bt="banner" role="alert" hidden></div>
            <main class="bt-body" data-bt="body"></main>
            <div class="bt-dialog-layer" data-bt="dialog" hidden></div>
        </section>`;
    }

    function adapterBar(esc, t, data, options) {
        const adapter = data.adapter || {};
        if (!data.present) return `<div class="bt-adapter-info"><strong>${esc(t('desktop.app_bluetooth'))}</strong></div>`;
        const powered = !!adapter.powered;
        const scanning = !!(data.discovery && data.discovery.active);
        const visible = !!(data.discoverable && data.discoverable.active);
        const details = [adapter.path ? adapter.path.split('/').pop() : '', adapter.address || '', t(powered ? 'bluetooth.adapter_on' : 'bluetooth.adapter_off')].filter(Boolean).join(' · ');
        const menu = options.visibleMenu
            ? `<div class="bt-menu" role="menu">${options.minutes.map(m => `<button type="button" role="menuitem" data-action="visible" data-minutes="${m}">${esc(t('bluetooth.visible_minutes', { minutes: m }))}</button>`).join('')}</div>`
            : '';
        const visibility = visible
            ? `<span class="bt-badge" data-bt="visible-badge">${esc(t('bluetooth.visible_for', { time: clock(remaining(data.discoverable, options.elapsed)) }))}</span><button type="button" data-action="hide-visible">${esc(t('bluetooth.stop_visible'))}</button>`
            : `<span class="bt-menu-anchor"><button type="button" data-action="visible-menu" aria-haspopup="menu" aria-expanded="${options.visibleMenu}"${powered ? '' : ' disabled'}>${esc(t('bluetooth.make_visible'))}</button>${menu}</span>`;
        const scan = scanning
            ? `<button type="button" data-action="stop-scan">${esc(t('bluetooth.stop_scan', { seconds: remaining(data.discovery, options.elapsed) }))}</button>`
            : `<button type="button" data-action="scan"${powered ? '' : ' disabled'}>${esc(t('bluetooth.scan'))}</button>`;
        return `<div class="bt-adapter-info"><strong>${esc(adapter.name || t('desktop.app_bluetooth'))}</strong><span class="bt-muted">${esc(details)}</span></div>
            ${visibility}${scan}
            <label class="bt-switch"><input type="checkbox" role="switch" data-action="power"${powered ? ' checked' : ''}><span aria-hidden="true"></span><span class="bt-sr">${esc(t('bluetooth.power'))}</span></label>`;
    }

    function deviceRow(esc, t, device, options) {
        const address = esc(device.address);
        const busy = !!device.operation;
        const state = [];
        if (busy) state.push(t(OPERATION_KEYS[device.operation] || 'bluetooth.op_trusting'));
        else if (device.connected) state.push(t('bluetooth.connected'));
        else if (device.paired) state.push(t('bluetooth.paired'));
        if (!busy && device.paired && device.trusted) state.push(t('bluetooth.trusted'));
        if (typeof device.battery === 'number') state.push(t('bluetooth.battery', { percent: device.battery }));
        const serverError = device.error && !options.dismissed.has(device.address + '|' + device.error) ? errorFor(t, device.error) : '';
        const error = options.localErrors.get(device.address) || serverError;
        let primary = '';
        if (device.operation === 'pairing') {
            primary = `<button type="button" data-action="device" data-op="cancel_pairing" data-address="${address}">${esc(t('bluetooth.cancel'))}</button>`;
        } else if (!busy && options.powered) {
            const op = !device.paired ? 'pair' : (device.connected ? 'disconnect' : 'connect');
            primary = `<button type="button" data-action="device" data-op="${op}" data-address="${address}">${esc(t(PRIMARY_KEYS[op]))}</button>`;
        }
        const tone = !busy && device.connected && device.audio && options.audioUsable
            ? `<button type="button" data-action="test-tone" data-address="${address}">${esc(t('bluetooth.test_tone'))}</button>` : '';
        const menuItems = `<button type="button" role="menuitem" data-action="device" data-op="${device.trusted ? 'untrust' : 'trust'}" data-address="${address}">${esc(t(device.trusted ? 'bluetooth.untrust' : 'bluetooth.trust'))}</button>`
            + `<button type="button" role="menuitem" class="bt-danger" data-action="device" data-op="remove" data-address="${address}">${esc(t('bluetooth.remove'))}</button>`;
        const menu = device.paired && !busy
            ? `<span class="bt-menu-anchor"><button type="button" class="bt-more" data-action="menu" data-address="${address}" aria-haspopup="menu" aria-expanded="${options.menuOpen}" aria-label="${esc(t('bluetooth.more'))}">⋯</button>${options.menuOpen ? `<div class="bt-menu" role="menu">${menuItems}</div>` : ''}</span>` : '';
        const signal = !device.paired && typeof device.rssi === 'number'
            ? `<span class="bt-signal" data-level="${signalLevel(device.rssi)}" aria-hidden="true"><i></i><i></i><i></i><i></i></span>` : '';
        return `<li class="bt-device${busy ? ' is-busy' : ''}" data-address="${address}">${glyph(device.type)}
            <div class="bt-device-text"><span class="bt-device-name">${esc(deviceName(device))}</span>
                <span class="bt-device-state${device.connected && !busy ? ' is-connected' : ''}">${esc(state.join(' · '))}</span>
                ${error ? `<span class="bt-device-error" role="alert">${esc(error)} <button type="button" class="bt-link" data-action="dismiss-error" data-address="${address}">${esc(t('bluetooth.dismiss'))}</button></span>` : ''}</div>
            ${signal}${tone}${primary}${menu}</li>`;
    }

    function body(esc, t, data, options) {
        if (!data.present) {
            return `<div class="bt-empty">${glyph('other')}<h2>${esc(t('bluetooth.unavailable_title'))}</h2><p>${esc(t('bluetooth.unavailable_hint'))}</p></div>`;
        }
        const devices = Array.isArray(data.devices) ? data.devices : [];
        const powered = !!(data.adapter && data.adapter.powered);
        const row = device => deviceRow(esc, t, device, Object.assign({}, options, { powered, menuOpen: options.menuFor === device.address }));
        const paired = devices.filter(d => d.paired);
        const found = devices.filter(d => !d.paired).sort((a, b) => (b.rssi ?? -999) - (a.rssi ?? -999));
        const named = found.filter(isNamed);
        const unnamed = found.filter(d => !isNamed(d));
        const scanning = !!(data.discovery && data.discovery.active);
        const off = powered ? '' : `<div class="bt-off"><p><strong>${esc(t('bluetooth.off_title'))}</strong> ${esc(t('bluetooth.off_hint'))}</p><button type="button" data-action="turn-on">${esc(t('bluetooth.turn_on'))}</button></div>`;
        const mine = paired.length ? `<ul class="bt-list">${paired.map(row).join('')}</ul>` : `<p class="bt-muted bt-pad">${esc(t('bluetooth.no_devices'))}</p>`;
        const unnamedBlock = unnamed.length
            ? `<button type="button" class="bt-unnamed" data-action="toggle-unnamed" aria-expanded="${options.unnamedOpen}">${esc(t('bluetooth.unnamed_devices', { count: unnamed.length }))}</button>${options.unnamedOpen ? `<ul class="bt-list">${unnamed.map(row).join('')}</ul>` : ''}`
            : '';
        const foundBlock = scanning || found.length
            ? `<section class="bt-section"><h3>${esc(t('bluetooth.found_devices'))}${scanning ? ` <span class="bt-muted">${esc(t('bluetooth.scanning'))}</span>` : ''}</h3>${named.length ? `<ul class="bt-list">${named.map(row).join('')}</ul>` : `<p class="bt-muted bt-pad">${esc(t('bluetooth.nothing_found'))}</p>`}${unnamedBlock}</section>`
            : '';
        return `${off}<section class="bt-section"><h3>${esc(t('bluetooth.my_devices'))}</h3>${mine}</section>${foundBlock}`;
    }

    function answerButtons(esc, t, acceptLabel) {
        return `<button type="button" data-action="answer" data-accept="false">${esc(t('bluetooth.reject'))}</button><button type="button" class="bt-primary" data-action="answer" data-accept="true">${esc(acceptLabel)}</button>`;
    }

    function spaced(passkey) {
        const value = String(passkey || '');
        return value.length === 6 ? value.slice(0, 3) + ' ' + value.slice(3) : value;
    }

    function dialog(esc, t, view) {
        const name = view.device_name || view.device_address || '';
        const close = `<button type="button" data-action="close-dialog">${esc(t('bluetooth.close'))}</button>`;
        const field = (label, attributes) => `<label><span>${esc(label)}</span><input type="text" autocomplete="off" ${attributes}></label><p class="bt-field-error" data-bt="dialog-error" role="alert"></p>`;
        let content = '';
        let actions = close;
        switch (view.kind) {
        case 'confirm_passkey':
            content = `<p>${esc(t('bluetooth.confirm_passkey'))}</p><p class="bt-passkey">${esc(spaced(view.passkey))}</p>`;
            actions = answerButtons(esc, t, t('bluetooth.accept'));
            break;
        case 'enter_passkey':
            content = field(t('bluetooth.enter_passkey'), 'inputmode="numeric" maxlength="6"');
            actions = answerButtons(esc, t, t('bluetooth.submit'));
            break;
        case 'enter_pin':
            content = field(t('bluetooth.enter_pin'), 'maxlength="16"');
            actions = answerButtons(esc, t, t('bluetooth.submit'));
            break;
        case 'display_passkey':
            content = `<p>${esc(t('bluetooth.display_passkey'))}</p><p class="bt-passkey">${esc(spaced(view.passkey))}</p>`;
            break;
        case 'display_pin':
            content = `<p>${esc(t('bluetooth.display_pin'))}</p><p class="bt-passkey">${esc(view.pin || '')}</p>`;
            break;
        case 'authorize_pairing':
            content = `<p>${esc(t('bluetooth.authorize_pairing'))}</p>`;
            actions = answerButtons(esc, t, t('bluetooth.accept'));
            break;
        case 'authorize_service':
            content = `<p>${esc(t('bluetooth.authorize_service', { service: view.service || '' }))}</p>`;
            actions = answerButtons(esc, t, t('bluetooth.allow'));
            break;
        }
        return `<div class="bt-dialog" role="dialog" aria-modal="true" aria-labelledby="bt-dialog-title">
            <h2 id="bt-dialog-title">${esc(t('bluetooth.interaction_title', { name }))}</h2>${content}
            <div class="bt-progress" aria-hidden="true"><i data-bt="progress"></i></div><p class="bt-muted" data-bt="expires"></p>
            <div class="bt-dialog-actions">${actions}</div></div>`;
    }

    window.BluetoothViews = { shell, adapterBar, body, dialog, deviceName, remaining, clock, errorFor };
})();
