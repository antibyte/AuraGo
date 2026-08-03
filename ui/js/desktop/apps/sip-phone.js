(function () {
    'use strict';

    const instances = new Map();
    const dialKeys = [
        ['1', ''],
        ['2', 'ABC'],
        ['3', 'DEF'],
        ['4', 'GHI'],
        ['5', 'JKL'],
        ['6', 'MNO'],
        ['7', 'PQRS'],
        ['8', 'TUV'],
        ['9', 'WXYZ'],
        ['*', ''],
        ['0', '+'],
        ['#', '']
    ];
    const dtmfFrequencies = {
        1: [697, 1209], 2: [697, 1336], 3: [697, 1477],
        4: [770, 1209], 5: [770, 1336], 6: [770, 1477],
        7: [852, 1209], 8: [852, 1336], 9: [852, 1477],
        '*': [941, 1209], 0: [941, 1336], '#': [941, 1477]
    };

    function text(instance, key, fallback, params) {
        const translationKey = 'desktop.sip_phone_' + key;
        const value = instance.context.t(translationKey, params || {});
        return value === translationKey ? fallback : value;
    }

    function normalizeParty(value) {
        let result = String(value || '').trim().toLowerCase();
        if (result.startsWith('sip:')) result = result.slice(4);
        return result.replace(/[\s().-]/g, '');
    }

    function contactName(instance, remoteParty) {
        const target = normalizeParty(remoteParty);
        const user = target.includes('@') ? target.split('@')[0] : target;
        const contact = instance.contacts.find(item => {
            const candidates = [item.sip_address, item.email, item.phone, item.mobile]
                .filter(Boolean)
                .map(normalizeParty);
            return candidates.some(candidate => candidate === target || candidate === user);
        });
        return contact ? (contact.name || [contact.first_name, contact.last_name].filter(Boolean).join(' ')) : '';
    }

    function partyLabel(instance, remoteParty) {
        return contactName(instance, remoteParty) || String(remoteParty || text(instance, 'unknown_party', 'Unknown caller'));
    }

    function avatarHue(label) {
        const value = String(label || '');
        let hash = 0;
        for (let index = 0; index < value.length; index += 1) {
            hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
        }
        return hash % 360;
    }

    function initialOf(label) {
        return String(label || '').trim().slice(0, 1).toUpperCase() || '?';
    }

    function formatDuration(seconds) {
        const value = Math.max(0, Number(seconds) || 0);
        const hours = Math.floor(value / 3600);
        const minutes = Math.floor((value % 3600) / 60);
        const remainder = Math.floor(value % 60);
        return hours
            ? [hours, minutes, remainder].map(part => String(part).padStart(2, '0')).join(':')
            : [minutes, remainder].map(part => String(part).padStart(2, '0')).join(':');
    }

    function callDuration(call) {
        if (!call || !call.answered_at) return 0;
        const end = call.ended_at ? new Date(call.ended_at).getTime() : Date.now();
        return Math.max(0, Math.floor((end - new Date(call.answered_at).getTime()) / 1000));
    }

    function callTime(call) {
        const value = new Date(call.started_at);
        return Number.isNaN(value.getTime()) ? '' : value.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    }

    function backspaceGlyph() {
        return '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M8.2 5h10.3a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H8.2L2 12l6.2-7z" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/><path d="m11 9.2 5.6 5.6m0-5.6L11 14.8" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/></svg>';
    }

    function ensureToneContext(instance) {
        const AudioContextClass = window.AudioContext || window.webkitAudioContext;
        if (!AudioContextClass) return null;
        if (!instance.toneContext) {
            try {
                instance.toneContext = new AudioContextClass();
            } catch (_) {
                return null;
            }
        }
        if (instance.toneContext.state === 'suspended') instance.toneContext.resume().catch(() => {});
        return instance.toneContext;
    }

    function startKeyTone(instance, digit) {
        const frequencies = dtmfFrequencies[digit];
        const context = ensureToneContext(instance);
        if (!frequencies || !context) return;
        stopKeyTone(instance);
        const gain = context.createGain();
        gain.gain.setValueAtTime(0, context.currentTime);
        gain.gain.linearRampToValueAtTime(0.16, context.currentTime + 0.006);
        gain.connect(context.destination);
        const oscillators = frequencies.map(frequency => {
            const oscillator = context.createOscillator();
            oscillator.type = 'sine';
            oscillator.frequency.value = frequency;
            oscillator.connect(gain);
            oscillator.start();
            return oscillator;
        });
        const timeout = setTimeout(() => stopKeyTone(instance), 650);
        instance.keyTone = { gain, oscillators, timeout, startedAt: context.currentTime };
    }

    function stopKeyTone(instance) {
        const tone = instance.keyTone;
        const context = instance.toneContext;
        if (!tone || !context) return;
        instance.keyTone = null;
        clearTimeout(tone.timeout);
        const stopAt = Math.max(context.currentTime + 0.02, tone.startedAt + 0.07);
        tone.gain.gain.cancelScheduledValues(context.currentTime);
        tone.gain.gain.setValueAtTime(tone.gain.gain.value, context.currentTime);
        tone.gain.gain.linearRampToValueAtTime(0, stopAt);
        tone.oscillators.forEach(oscillator => {
            try {
                oscillator.stop(stopAt + 0.02);
            } catch (_) {}
        });
    }

    function wireKeyTones(instance, button) {
        button.addEventListener('pointerdown', () => {
            if (button.disabled) return;
            instance.keyToneFromPointer = true;
            startKeyTone(instance, button.dataset.sipDigit);
        });
        ['pointerup', 'pointerleave', 'pointercancel'].forEach(type => button.addEventListener(type, () => stopKeyTone(instance)));
    }

    function render(instance, snapshot) {
        if (!instance.host || !instance.host.isConnected) return;
        const appState = snapshot.appState || {};
        const status = appState.status || {};
        const call = snapshot.call;
        const capabilities = appState.capabilities || {};
        instance.snapshot = snapshot;
        instance.host.innerHTML = `
            <div class="sip-phone ${call ? 'has-call' : ''} ${call && call.state === 'ringing' ? 'is-ringing' : ''}">
                <div class="sip-phone-audio-viz" aria-hidden="true">
                    <div class="sip-phone-viz-ring sip-phone-viz-mic"></div>
                    <div class="sip-phone-viz-ring sip-phone-viz-speaker"></div>
                </div>
                <div class="sip-phone-device">
                    <span class="sip-phone-hw sip-phone-hw-mute" aria-hidden="true"></span>
                    <span class="sip-phone-hw sip-phone-hw-vol-up" aria-hidden="true"></span>
                    <span class="sip-phone-hw sip-phone-hw-vol-down" aria-hidden="true"></span>
                    <span class="sip-phone-hw sip-phone-hw-power" aria-hidden="true"></span>
                    <div class="sip-phone-screen">
                        <div class="sip-phone-wallpaper"></div>
                        ${renderStatusBar(instance, status, call)}
                        ${call ? renderActiveCall(instance, snapshot) : renderViews(instance, snapshot, capabilities, status)}
                        <div class="sip-phone-glare" aria-hidden="true"></div>
                    </div>
                </div>
            </div>`;
        wireEvents(instance);
        updateDuration(instance);
    }

    function renderStatusBar(instance, status, call) {
        const level = !status.enabled ? 0 : (status.registered ? 4 : 1);
        return `<header class="sip-phone-statusbar">
            <span class="sip-phone-clock" data-sip-phone-clock>${instance.context.esc(new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }))}</span>
            <div class="sip-phone-island">${call
                ? `<span class="sip-phone-island-live"><span class="sip-phone-island-pulse"></span><span class="sip-phone-island-label">${instance.context.esc(partyLabel(instance, call.remote_party))}</span></span>`
                : '<span class="sip-phone-island-camera"></span>'}</div>
            <span class="sip-phone-status-right">
                <span class="sip-phone-signal" aria-hidden="true">${[1, 2, 3, 4].map(bar => `<i class="${bar <= level ? 'is-on' : ''}"></i>`).join('')}</span>
                <span class="sip-phone-carrier">${instance.context.esc(text(instance, 'carrier', 'AuraGo'))}</span>
                <span class="sip-phone-battery" aria-hidden="true"><i></i></span>
            </span>
        </header>`;
    }

    function renderViews(instance, snapshot, capabilities, status) {
        const tab = instance.tab || 'keypad';
        let view;
        if (tab === 'favorites') view = renderFavoritesView(instance, (snapshot.preferences || {}).favorites || []);
        else if (tab === 'recents') view = renderHistory(instance, (snapshot.appState || {}).recent_calls || []);
        else if (tab === 'settings') view = renderSettingsView(instance, snapshot, status);
        else view = renderKeypadView(instance, snapshot, capabilities, status);
        return `<div class="sip-phone-views">${view}</div>
            ${renderTabBar(instance, tab)}
            <div class="sip-phone-home"></div>`;
    }

    function renderTabBar(instance, active) {
        const tabs = [
            ['favorites', 'star', 'F', text(instance, 'favorites', 'Favorites')],
            ['recents', 'clock', 'R', text(instance, 'tab_recents', 'Recents')],
            ['keypad', 'grid', 'K', text(instance, 'keypad', 'Keypad')],
            ['settings', 'settings', 'S', text(instance, 'settings', 'Settings')]
        ];
        return `<nav class="sip-phone-tabbar">${tabs.map(([id, icon, fallback, label]) => `
            <button type="button" data-sip-tab="${id}" class="${id === active ? 'is-active' : ''}" aria-pressed="${id === active}">
                ${instance.context.iconMarkup(icon, fallback, 'sip-phone-glyph', 22)}<span>${instance.context.esc(label)}</span>
            </button>`).join('')}
        </nav>`;
    }

    function setupBlocker(instance, snapshot) {
        const appState = snapshot.appState || {};
        const blockers = appState.blockers || [];
        if (!snapshot.secureContext) return ['insecure_context', text(instance, 'insecure_context', 'Open AuraGo over HTTPS or localhost to use the microphone.')];
        if (blockers.includes('disabled')) return ['disabled', text(instance, 'disabled_detail', 'Enable the SIP endpoint before using the phone.')];
        if (blockers.includes('readonly')) return ['readonly', text(instance, 'readonly_detail', 'SIP is in read-only mode. Calling controls are disabled.')];
        if (blockers.includes('browser_media_disabled')) return ['browser_media_disabled', text(instance, 'browser_media_disabled_detail', 'Enable browser media for the Virtual Desktop phone.')];
        if (blockers.includes('browser_media_restart_required')) return ['browser_media_restart_required', text(instance, 'browser_media_restart_required_detail', 'Restart AuraGo to apply the saved browser media settings.')];
        if (blockers.includes('outbound_policy_migration_required')) return ['outbound_policy_migration_required', text(instance, 'outbound_policy_migration_required_detail', 'Replace legacy wildcard outbound rules with exact destinations in SIP settings.')];
        if (blockers.includes('not_registered')) return ['not_registered', text(instance, 'not_registered_detail', 'The SIP account is not registered yet. Check the account and PBX connection.')];
        if (blockers.includes('outbound_disabled')) return ['outbound_disabled', text(instance, 'outbound_disabled_detail', 'Outgoing calls are not enabled or no allowed destinations are configured.')];
        return null;
    }

    function renderKeypadView(instance, snapshot, capabilities, status) {
        const blocker = setupBlocker(instance, snapshot);
        const disabled = blocker || !capabilities.dial;
        const registered = !!status.registered;
        const state = status.enabled
            ? (registered ? text(instance, 'registered', 'Registered') : text(instance, 'not_registered', 'Not registered'))
            : text(instance, 'disabled', 'Disabled');
        return `<section class="sip-phone-view sip-phone-view-keypad">
            <div class="sip-phone-account-pill"><span class="sip-phone-status-dot ${registered ? 'is-ready' : ''}"></span><span>${instance.context.esc(state)}</span></div>
            ${blocker ? `<div class="sip-phone-blocker" data-blocker="${instance.context.esc(blocker[0])}">
                <strong>${instance.context.esc(text(instance, 'unavailable', 'Calling is unavailable'))}</strong>
                <span>${instance.context.esc(blocker[1])}</span>
                <a href="/config#sip" target="_blank" rel="noopener">${instance.context.esc(text(instance, 'open_configuration', 'Open Configuration → SIP Phone'))}</a>
            </div>` : ''}
            ${snapshot.error ? `<div class="sip-phone-inline-error" role="alert">${instance.context.esc(snapshot.error)}</div>` : ''}
            <div class="sip-phone-target">
                <button type="button" data-sip-phone-action="favorite" aria-label="${instance.context.esc(text(instance, 'add_favorite', 'Add to favorites'))}" ${disabled ? 'disabled' : ''}>${instance.context.iconMarkup('star', 'F', 'sip-phone-glyph', 15)}</button>
                <input type="text" data-sip-phone="target" value="${instance.context.esc(instance.target)}"
                    placeholder="${instance.context.esc(text(instance, 'target_placeholder', 'SIP number or name'))}"
                    autocomplete="off" spellcheck="false" ${disabled ? 'disabled' : ''}>
                <button type="button" data-sip-phone-action="clear" aria-label="${instance.context.esc(text(instance, 'clear', 'Clear'))}" ${disabled ? 'disabled' : ''}>×</button>
            </div>
            ${renderKeypad(instance, disabled, false)}
            <div class="sip-phone-call-row">
                <span></span>
                <button type="button" class="sip-phone-call-button" data-sip-phone-action="dial" ${disabled ? 'disabled' : ''}>
                    ${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 30)}
                    <span>${instance.context.esc(text(instance, snapshot.phase === 'preparing' ? 'preparing' : 'call', snapshot.phase === 'preparing' ? 'Preparing…' : 'Call'))}</span>
                </button>
                <button type="button" class="sip-phone-backspace" data-sip-phone-action="backspace" aria-label="${instance.context.esc(text(instance, 'backspace', 'Delete'))}" ${disabled ? 'disabled' : ''}>${backspaceGlyph()}</button>
            </div>
            <p class="sip-phone-ready">${instance.context.esc(disabled ? text(instance, 'unavailable', 'Calling is unavailable') : text(instance, 'ready', 'Ready to call'))}</p>
        </section>`;
    }

    function renderKeypad(instance, disabled, compact) {
        return `<div class="sip-phone-keypad ${compact ? 'is-compact' : ''}">
            ${dialKeys.map(([digit, letters]) => `<button type="button" data-sip-digit="${digit}" ${disabled ? 'disabled' : ''}>
                <strong>${digit}</strong><small>${letters || '&nbsp;'}</small>
            </button>`).join('')}
        </div>`;
    }

    function renderFavoritesView(instance, favorites) {
        return `<section class="sip-phone-view sip-phone-view-favorites">
            <div class="sip-phone-view-header"><h2>${instance.context.esc(text(instance, 'favorites', 'Favorites'))}</h2><span>${favorites.length}/24</span></div>
            <div class="sip-phone-favorite-list">
                ${favorites.length
                    ? favorites.map((favorite, index) => {
                        const label = String(favorite.label || favorite.target);
                        return `<button type="button" data-sip-favorite="${index}" title="${instance.context.esc(favorite.target)}">
                        <span class="sip-phone-avatar" style="--sip-hue:${avatarHue(label)}">${instance.context.esc(initialOf(label))}</span>
                        <span><strong>${instance.context.esc(label)}</strong><small>${instance.context.esc(favorite.target)}</small></span>
                        <span>${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 14)}</span>
                    </button>`;
                    }).join('')
                    : `<p>${instance.context.esc(text(instance, 'favorites_empty', 'Save frequently used numbers for one-click calling.'))}</p>`}
            </div>
        </section>`;
    }

    function renderSettingsView(instance, snapshot, status) {
        const preferences = snapshot.preferences || {};
        const inputs = snapshot.devices.inputs || [];
        const outputs = snapshot.devices.outputs || [];
        const registered = !!status.registered;
        const state = status.enabled
            ? (registered ? text(instance, 'registered', 'Registered') : text(instance, 'not_registered', 'Not registered'))
            : text(instance, 'disabled', 'Disabled');
        return `<section class="sip-phone-view sip-phone-view-settings">
            <div class="sip-phone-view-header"><h2>${instance.context.esc(text(instance, 'settings', 'Settings'))}</h2></div>
            <div class="sip-phone-card">
                <h3>${instance.context.esc(text(instance, 'account', 'Account'))}</h3>
                <div class="sip-phone-account-row">
                    <span class="sip-phone-status-dot ${registered ? 'is-ready' : ''}"></span>
                    <div><strong>${instance.context.esc(state)}</strong>
                        <small>${instance.context.esc(status.enabled ? (status.bind_address || '') : text(instance, 'configure_hint', 'Configure SIP to start calling.'))}</small>
                    </div>
                </div>
            </div>
            <div class="sip-phone-card">
                <h3>${instance.context.esc(text(instance, 'devices', 'Devices'))}</h3>
                <label class="sip-phone-row">
                    <span class="sip-phone-device-icon">${instance.context.iconMarkup('microphone', 'M', 'sip-phone-glyph', 16)}</span>
                    <span><strong>${instance.context.esc(text(instance, 'microphone', 'Microphone'))}</strong>
                        <select data-sip-phone="input-device" aria-label="${instance.context.esc(text(instance, 'microphone', 'Microphone'))}">
                            <option value="">${instance.context.esc(text(instance, 'system_default', 'System default'))}</option>
                            ${inputs.map((device, index) => `<option value="${instance.context.esc(device.deviceId)}" ${device.deviceId === preferences.input_device ? 'selected' : ''}>${instance.context.esc(device.label || text(instance, 'microphone_number', 'Microphone {number}', { number: index + 1 }))}</option>`).join('')}
                        </select>
                    </span>
                </label>
                <label class="sip-phone-row">
                    <span class="sip-phone-device-icon">${instance.context.iconMarkup('audio-player', 'S', 'sip-phone-glyph', 16)}</span>
                    <span><strong>${instance.context.esc(text(instance, 'speaker', 'Speaker'))}</strong>
                        ${snapshot.sinkSelectionSupported
                            ? `<select data-sip-phone="output-device" aria-label="${instance.context.esc(text(instance, 'speaker', 'Speaker'))}">
                                <option value="">${instance.context.esc(text(instance, 'system_default', 'System default'))}</option>
                                ${outputs.map((device, index) => `<option value="${instance.context.esc(device.deviceId)}" ${device.deviceId === preferences.output_device ? 'selected' : ''}>${instance.context.esc(device.label || text(instance, 'speaker_number', 'Speaker {number}', { number: index + 1 }))}</option>`).join('')}
                            </select>`
                            : `<small>${instance.context.esc(text(instance, 'speaker_browser_default', 'Browser default output'))}</small>`}
                    </span>
                </label>
                <label class="sip-phone-row sip-phone-ringtone">
                    <span class="sip-phone-device-icon">${instance.context.iconMarkup('audio', 'R', 'sip-phone-glyph', 16)}</span>
                    <span><strong>${instance.context.esc(text(instance, 'ringtone', 'Ringtone'))}</strong>
                        <input type="checkbox" data-sip-phone="ringtone" ${preferences.ringtone_enabled !== false ? 'checked' : ''}>
                    </span>
                </label>
            </div>
            <div class="sip-phone-card">
                <a class="sip-phone-settings-link" href="/config#sip" target="_blank" rel="noopener">
                    <span>${instance.context.esc(text(instance, 'open_configuration', 'Open Configuration → SIP Phone'))}</span>
                    ${instance.context.iconMarkup('settings', 'S', 'sip-phone-glyph', 16)}
                </a>
            </div>
        </section>`;
    }

    function renderActiveCall(instance, snapshot) {
        const call = snapshot.call;
        const name = partyLabel(instance, call.remote_party);
        const subtitle = name === call.remote_party ? '' : call.remote_party;
        const status = call.direction === 'outbound' && call.state === 'ringing'
            ? text(instance, 'outgoing_ringing', 'The other phone is ringing')
            : call.direction === 'outbound' && call.state === 'connecting'
                ? text(instance, 'calling', 'Calling…')
                : call.state === 'ringing'
                    ? text(instance, 'incoming_title', 'Incoming call')
                    : text(instance, 'call_active', 'Call active');
        const incomingRinging = call.direction === 'inbound' && call.state === 'ringing' && !snapshot.observer;
        return `<div class="sip-phone-active-call">
            ${snapshot.observer ? `<div class="sip-phone-observer">${instance.context.esc(text(instance, 'observer_mode', 'This tab cannot take over the call audio. You can still hang up here.'))}</div>` : ''}
            ${snapshot.error ? `<div class="sip-phone-inline-error" role="alert">${instance.context.esc(snapshot.error)}</div>` : ''}
            ${snapshot.audioPlaybackBlocked ? `<div class="sip-phone-audio-gate" role="status">
                <span>${instance.context.esc(text(instance, 'audio_blocked', 'The browser blocked call audio.'))}</span>
                <button type="button" data-sip-phone-action="enable-audio">${instance.context.esc(text(instance, 'enable_audio', 'Enable sound'))}</button>
            </div>` : ''}
            <div class="sip-phone-call-avatar" style="--sip-hue:${avatarHue(name)}">${instance.context.esc(initialOf(name))}</div>
            <p aria-live="polite">${instance.context.esc(status)}</p>
            <h1>${instance.context.esc(name)}</h1>
            ${subtitle ? `<span>${instance.context.esc(subtitle)}</span>` : ''}
            ${call.answered_at ? `<time data-sip-phone-duration>${formatDuration(callDuration(call))}</time>` : ''}
            ${incomingRinging ? `<div class="sip-phone-call-actions">
                <button type="button" class="sip-phone-call-action sip-phone-decline" data-sip-phone-action="reject">
                    <span>${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 27)}</span><span>${instance.context.esc(text(instance, 'reject', 'Reject'))}</span>
                </button>
                <button type="button" class="sip-phone-call-action sip-phone-answer" data-sip-phone-action="answer">
                    <span>${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 27)}</span><span>${instance.context.esc(text(instance, 'answer', 'Answer'))}</span>
                </button>
            </div>` : `<div class="sip-phone-call-controls">
                <button type="button" data-sip-phone-action="mute" class="${snapshot.muted ? 'is-active' : ''}" ${snapshot.observer ? 'disabled' : ''}>
                    ${instance.context.iconMarkup('microphone', 'M', 'sip-phone-glyph', 22)}<span>${instance.context.esc(text(instance, snapshot.muted ? 'unmute' : 'mute', snapshot.muted ? 'Unmute' : 'Mute'))}</span>
                </button>
                <button type="button" data-sip-phone-action="toggle-keypad" ${snapshot.observer ? 'disabled' : ''}>
                    ${instance.context.iconMarkup('grid', 'K', 'sip-phone-glyph', 22)}<span>${instance.context.esc(text(instance, 'keypad', 'Keypad'))}</span>
                </button>
            </div>
            <label class="sip-phone-volume"><span>${instance.context.esc(text(instance, 'speaker_volume', 'Speaker volume'))}</span>
                <input type="range" data-sip-phone="volume" min="0" max="1" step="0.05" value="${Number(snapshot.preferences.volume || 0)}" ${snapshot.observer ? 'disabled' : ''}>
            </label>
            <div class="sip-phone-active-keypad" ${instance.keypadOpen ? '' : 'hidden'}>${renderKeypad(instance, snapshot.observer, true)}</div>
            <button type="button" class="sip-phone-hangup" data-sip-phone-action="hangup">
                ${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 27)}<span>${instance.context.esc(text(instance, 'hangup', 'Hang up'))}</span>
            </button>`}
        </div>`;
    }

    function renderHistory(instance, calls) {
        const runtime = window.SipPhoneRuntime;
        return `<section class="sip-phone-view sip-phone-view-recents">
            <div class="sip-phone-view-header"><h2>${instance.context.esc(text(instance, 'recent_calls', 'Recent calls'))}</h2><span>${Math.min(calls.length, 50)}</span></div>
            <div class="sip-phone-history-list">
                ${calls.length ? calls.slice(0, 50).map(call => {
                    const label = partyLabel(instance, call.remote_party);
                    const statusClass = call.direction === 'inbound' ? 'is-inbound' : 'is-outbound';
                    const endReason = runtime && typeof runtime.describeEndReason === 'function'
                        ? runtime.describeEndReason(call.end_reason)
                        : '';
                    return `<article class="sip-phone-history-item ${statusClass}">
                        <button type="button" class="sip-phone-history-main" data-sip-redial="${instance.context.esc(call.remote_party)}">
                            <span class="sip-phone-avatar-wrap"><span class="sip-phone-avatar" style="--sip-hue:${avatarHue(label)}">${instance.context.esc(initialOf(label))}</span><span class="sip-phone-history-icon">${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 9)}</span></span>
                            <span><strong>${instance.context.esc(label)}</strong><small>${instance.context.esc(call.remote_party)}</small></span>
                            <time>${instance.context.esc(callTime(call))}</time>
                        </button>
                        <div>
                            <small>${instance.context.esc(text(instance, call.direction === 'inbound' ? 'incoming' : 'outgoing', call.direction === 'inbound' ? 'Incoming' : 'Outgoing'))} · ${formatDuration(callDuration(call))}${endReason ? ` · ${instance.context.esc(endReason)}` : ''}</small>
                            <button type="button" data-sip-copy="${instance.context.esc(call.remote_party)}" title="${instance.context.esc(text(instance, 'copy', 'Copy'))}">${instance.context.iconMarkup('copy', 'C', 'sip-phone-glyph', 13)}</button>
                        </div>
                    </article>`;
                }).join('') : `<p class="sip-phone-history-empty">${instance.context.esc(text(instance, 'history_empty', 'No calls yet.'))}</p>`}
            </div>
        </section>`;
    }

    function wireEvents(instance) {
        const runtime = window.SipPhoneRuntime;
        const input = instance.host.querySelector('[data-sip-phone="target"]');
        if (input) {
            input.addEventListener('input', event => { instance.target = event.target.value; });
            input.addEventListener('keydown', event => {
                if (event.key === 'Enter') startDial(instance);
            });
        }
        instance.host.querySelectorAll('[data-sip-digit="0"]').forEach(button => {
            let longPress = null;
            let triggered = false;
            const insertPlus = () => {
                triggered = true;
                instance.target += '+';
                const target = instance.host.querySelector('[data-sip-phone="target"]');
                if (target) {
                    target.value = instance.target;
                    target.focus();
                }
            };
            button.addEventListener('pointerdown', () => {
                if (instance.snapshot.call) return;
                triggered = false;
                longPress = setTimeout(insertPlus, 550);
            });
            ['pointerup', 'pointerleave', 'pointercancel'].forEach(type => button.addEventListener(type, () => {
                if (longPress) {
                    clearTimeout(longPress);
                    longPress = null;
                }
            }));
            button.addEventListener('click', event => {
                if (triggered) {
                    triggered = false;
                    event.preventDefault();
                    event.stopImmediatePropagation();
                }
            }, true);
        });
        instance.host.querySelectorAll('[data-sip-digit]').forEach(button => {
            wireKeyTones(instance, button);
            button.addEventListener('click', () => {
                const digit = button.dataset.sipDigit;
                if (instance.keyToneFromPointer) instance.keyToneFromPointer = false;
                else if (!button.disabled) {
                    startKeyTone(instance, digit);
                    setTimeout(() => stopKeyTone(instance), 140);
                }
                if (instance.snapshot.call) runtime.sendDTMF(digit).catch(error => showError(instance, error));
                else {
                    instance.target += digit;
                    const target = instance.host.querySelector('[data-sip-phone="target"]');
                    if (target) {
                        target.value = instance.target;
                        target.focus();
                    }
                }
            });
        });
        instance.host.querySelectorAll('[data-sip-tab]').forEach(button => button.addEventListener('click', () => {
            instance.tab = button.dataset.sipTab || 'keypad';
            render(instance, runtime.getState());
        }));
        instance.host.querySelector('[data-sip-phone-action="clear"]')?.addEventListener('click', () => {
            instance.target = '';
            const target = instance.host.querySelector('[data-sip-phone="target"]');
            if (target) target.value = '';
        });
        instance.host.querySelector('[data-sip-phone-action="backspace"]')?.addEventListener('click', () => {
            if (instance.snapshot.call) return;
            instance.target = instance.target.slice(0, -1);
            const target = instance.host.querySelector('[data-sip-phone="target"]');
            if (target) {
                target.value = instance.target;
                target.focus();
            }
        });
        instance.host.querySelector('[data-sip-phone-action="dial"]')?.addEventListener('click', () => startDial(instance));
        instance.host.querySelector('[data-sip-phone-action="favorite"]')?.addEventListener('click', () => addFavorite(instance));
        instance.host.querySelector('[data-sip-phone-action="mute"]')?.addEventListener('click', () => runtime.setMuted(!instance.snapshot.muted));
        instance.host.querySelector('[data-sip-phone-action="toggle-keypad"]')?.addEventListener('click', () => {
            instance.keypadOpen = !instance.keypadOpen;
            render(instance, runtime.getState());
        });
        instance.host.querySelector('[data-sip-phone-action="hangup"]')?.addEventListener('click', () => runtime.hangup().catch(error => showError(instance, error)));
        instance.host.querySelector('[data-sip-phone-action="answer"]')?.addEventListener('click', () => runtime.answer().catch(error => showError(instance, error)));
        instance.host.querySelector('[data-sip-phone-action="reject"]')?.addEventListener('click', () => runtime.reject().catch(error => showError(instance, error)));
        instance.host.querySelector('[data-sip-phone-action="enable-audio"]')?.addEventListener('click', () => runtime.enableAudio().catch(error => showError(instance, error)));
        instance.host.querySelector('[data-sip-phone="input-device"]')?.addEventListener('change', event => runtime.setInputDevice(event.target.value).catch(error => showError(instance, error)));
        instance.host.querySelector('[data-sip-phone="output-device"]')?.addEventListener('change', event => runtime.setOutputDevice(event.target.value).catch(error => showError(instance, error)));
        instance.host.querySelector('[data-sip-phone="volume"]')?.addEventListener('input', event => runtime.setVolume(event.target.value));
        instance.host.querySelector('[data-sip-phone="ringtone"]')?.addEventListener('change', event => runtime.setPreferences({ ringtone_enabled: event.target.checked }));
        instance.host.querySelectorAll('[data-sip-favorite]').forEach(button => button.addEventListener('click', () => {
            const favorite = (instance.snapshot.preferences.favorites || [])[Number(button.dataset.sipFavorite)];
            if (!favorite) return;
            instance.target = favorite.target;
            if (!instance.snapshot.call) startDial(instance);
        }));
        instance.host.querySelectorAll('[data-sip-redial]').forEach(button => button.addEventListener('click', () => {
            instance.target = button.dataset.sipRedial || '';
            if (!instance.snapshot.call) startDial(instance);
        }));
        instance.host.querySelectorAll('[data-sip-copy]').forEach(button => button.addEventListener('click', () => {
            navigator.clipboard.writeText(button.dataset.sipCopy || '').catch(() => {});
        }));
    }

    async function startDial(instance) {
        try {
            await window.SipPhoneRuntime.dial(instance.target);
        } catch (error) {
            showError(instance, error);
        }
    }

    function addFavorite(instance) {
        if (!instance.target.trim()) return;
        const preferences = instance.snapshot.preferences || {};
        const favorites = Array.isArray(preferences.favorites) ? preferences.favorites.slice() : [];
        if (favorites.length >= 24) {
            showError(instance, new Error(text(instance, 'favorites_limit', 'A maximum of 24 favorites is supported.')));
            return;
        }
        if (!favorites.some(item => normalizeParty(item.target) === normalizeParty(instance.target))) {
            favorites.push({ target: instance.target.trim(), label: partyLabel(instance, instance.target.trim()) });
            window.SipPhoneRuntime.setPreferences({ favorites });
        }
    }

    function showError(instance, error) {
        if (instance.context.notify) {
            instance.context.notify({ title: text(instance, 'app_name', 'Phone'), message: error.message || String(error) });
        }
    }

    function updateDuration(instance) {
        const timer = instance.host.querySelector('[data-sip-phone-duration]');
        if (timer && instance.snapshot.call) timer.textContent = formatDuration(callDuration(instance.snapshot.call));
        const clock = instance.host.querySelector('[data-sip-phone-clock]');
        if (clock) clock.textContent = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    }

    async function loadContacts(instance) {
        try {
            const response = await instance.context.api('/api/contacts');
            instance.contacts = Array.isArray(response) ? response : (response.contacts || []);
            render(instance, window.SipPhoneRuntime.getState());
        } catch (_) {
            instance.contacts = [];
        }
    }

    function mount(host, windowId, context) {
        dispose(windowId);
        const runtime = window.SipPhoneRuntime;
        if (!runtime) throw new Error('SIP phone runtime is unavailable');
        const instance = {
            host,
            windowId,
            context,
            contacts: [],
            target: '',
            tab: 'keypad',
            keypadOpen: false,
            snapshot: runtime.getState(),
            unsubscribe: null,
            timer: null,
            toneContext: null,
            keyTone: null,
            keyToneFromPointer: false
        };
        instances.set(windowId, instance);
        instance.unsubscribe = runtime.subscribe(snapshot => render(instance, snapshot));
        instance.timer = setInterval(() => updateDuration(instance), 1000);
        runtime.initialize();
        runtime.refreshDevices().catch(() => {});
        loadContacts(instance);
    }

    function dispose(windowId) {
        const instance = instances.get(windowId);
        if (!instance) return;
        if (instance.unsubscribe) instance.unsubscribe();
        if (instance.timer) clearInterval(instance.timer);
        stopKeyTone(instance);
        if (instance.toneContext) {
            instance.toneContext.close().catch(() => {});
            instance.toneContext = null;
        }
        instances.delete(windowId);
    }

    window.SipPhoneApp = { render: mount, dispose };
})();
