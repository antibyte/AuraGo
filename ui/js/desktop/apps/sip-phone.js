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

    function render(instance, snapshot) {
        if (!instance.host || !instance.host.isConnected) return;
        const appState = snapshot.appState || {};
        const status = appState.status || {};
        const call = snapshot.call;
        const preferences = snapshot.preferences || {};
        const capabilities = appState.capabilities || {};
        const sidebarsOpen = typeof window.matchMedia !== 'function' || !window.matchMedia('(max-width: 820px)').matches;
        instance.snapshot = snapshot;
        instance.host.innerHTML = `
            <div class="sip-phone ${call ? 'has-call' : ''}">
                <details class="sip-phone-sidebar sip-phone-sidebar-left" ${sidebarsOpen ? 'open' : ''}>
                    <summary>${instance.context.esc(text(instance, 'account_and_devices', 'Account and devices'))}</summary>
                    ${renderAccount(instance, status)}
                    ${renderDevices(instance, snapshot)}
                    ${renderFavorites(instance, preferences.favorites || [])}
                    <a class="sip-phone-settings-link" href="/config#sip" target="_blank" rel="noopener">
                        ${instance.context.iconMarkup('settings', 'S', 'sip-phone-glyph', 18)}
                        <span>${instance.context.esc(text(instance, 'settings', 'Settings'))}</span>
                    </a>
                </details>
                <main class="sip-phone-main">
                    ${call ? renderActiveCall(instance, snapshot) : renderDialer(instance, snapshot, capabilities)}
                </main>
                <details class="sip-phone-sidebar sip-phone-history" ${sidebarsOpen ? 'open' : ''}>
                    <summary>${instance.context.esc(text(instance, 'recent_calls', 'Recent calls'))}</summary>
                    ${renderHistory(instance, appState.recent_calls || [])}
                </details>
            </div>`;
        wireEvents(instance);
        updateDuration(instance);
    }

    function renderAccount(instance, status) {
        const registered = !!status.registered;
        const state = status.enabled
            ? (registered ? text(instance, 'registered', 'Registered') : text(instance, 'not_registered', 'Not registered'))
            : text(instance, 'disabled', 'Disabled');
        return `<section class="sip-phone-panel-section sip-phone-account">
            <h2>${instance.context.esc(text(instance, 'account', 'Account'))}</h2>
            <div class="sip-phone-account-row">
                <span class="sip-phone-status-dot ${registered ? 'is-ready' : ''}"></span>
                <div><strong>${instance.context.esc(state)}</strong>
                    <small>${instance.context.esc(status.enabled ? (status.bind_address || '') : text(instance, 'configure_hint', 'Configure SIP to start calling.'))}</small>
                </div>
            </div>
        </section>`;
    }

    function renderDevices(instance, snapshot) {
        const preferences = snapshot.preferences || {};
        const inputs = snapshot.devices.inputs || [];
        const outputs = snapshot.devices.outputs || [];
        return `<section class="sip-phone-panel-section sip-phone-devices">
            <h2>${instance.context.esc(text(instance, 'devices', 'Devices'))}</h2>
            <label>
                <span class="sip-phone-device-icon">${instance.context.iconMarkup('audio', 'A', 'sip-phone-glyph', 17)}</span>
                <span><strong>${instance.context.esc(text(instance, 'microphone', 'Microphone'))}</strong>
                    <select data-sip-phone="input-device" aria-label="${instance.context.esc(text(instance, 'microphone', 'Microphone'))}">
                        <option value="">${instance.context.esc(text(instance, 'system_default', 'System default'))}</option>
                        ${inputs.map((device, index) => `<option value="${instance.context.esc(device.deviceId)}" ${device.deviceId === preferences.input_device ? 'selected' : ''}>${instance.context.esc(device.label || text(instance, 'microphone_number', 'Microphone {number}', { number: index + 1 }))}</option>`).join('')}
                    </select>
                </span>
            </label>
            <label>
                <span class="sip-phone-device-icon">${instance.context.iconMarkup('audio-player', 'A', 'sip-phone-glyph', 17)}</span>
                <span><strong>${instance.context.esc(text(instance, 'speaker', 'Speaker'))}</strong>
                    ${snapshot.sinkSelectionSupported
                        ? `<select data-sip-phone="output-device" aria-label="${instance.context.esc(text(instance, 'speaker', 'Speaker'))}">
                            <option value="">${instance.context.esc(text(instance, 'system_default', 'System default'))}</option>
                            ${outputs.map((device, index) => `<option value="${instance.context.esc(device.deviceId)}" ${device.deviceId === preferences.output_device ? 'selected' : ''}>${instance.context.esc(device.label || text(instance, 'speaker_number', 'Speaker {number}', { number: index + 1 }))}</option>`).join('')}
                        </select>`
                        : `<small>${instance.context.esc(text(instance, 'speaker_browser_default', 'Browser default output'))}</small>`}
                </span>
            </label>
            <label class="sip-phone-ringtone">
                <span class="sip-phone-device-icon">${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 17)}</span>
                <span><strong>${instance.context.esc(text(instance, 'ringtone', 'Ringtone'))}</strong>
                    <input type="checkbox" data-sip-phone="ringtone" ${preferences.ringtone_enabled !== false ? 'checked' : ''}>
                </span>
            </label>
        </section>`;
    }

    function renderFavorites(instance, favorites) {
        return `<section class="sip-phone-panel-section sip-phone-favorites">
            <div class="sip-phone-section-heading"><h2>${instance.context.esc(text(instance, 'favorites', 'Favorites'))}</h2><span>${favorites.length}/24</span></div>
            <div class="sip-phone-favorite-list">
                ${favorites.length
                    ? favorites.map((favorite, index) => `<button type="button" data-sip-favorite="${index}" title="${instance.context.esc(favorite.target)}">
                        <span>${instance.context.esc(String(favorite.label || favorite.target).slice(0, 1).toUpperCase())}</span>
                        <strong>${instance.context.esc(favorite.label || favorite.target)}</strong>
                    </button>`).join('')
                    : `<p>${instance.context.esc(text(instance, 'favorites_empty', 'Save frequently used numbers for one-click calling.'))}</p>`}
            </div>
        </section>`;
    }

    function setupBlocker(instance, snapshot) {
        const appState = snapshot.appState || {};
        const blockers = appState.blockers || [];
        if (!snapshot.secureContext) return ['insecure_context', text(instance, 'insecure_context', 'Open AuraGo over HTTPS or localhost to use the microphone.')];
        if (blockers.includes('disabled')) return ['disabled', text(instance, 'disabled_detail', 'Enable the SIP endpoint before using the phone.')];
        if (blockers.includes('readonly')) return ['readonly', text(instance, 'readonly_detail', 'SIP is in read-only mode. Calling controls are disabled.')];
        if (blockers.includes('browser_media_disabled')) return ['browser_media_disabled', text(instance, 'browser_media_disabled_detail', 'Enable browser media for the Virtual Desktop phone.')];
        if (blockers.includes('browser_media_restart_required')) return ['browser_media_restart_required', text(instance, 'browser_media_restart_required_detail', 'Restart AuraGo to apply the saved browser media settings.')];
        if (blockers.includes('not_registered')) return ['not_registered', text(instance, 'not_registered_detail', 'The SIP account is not registered yet. Check the account and PBX connection.')];
        if (blockers.includes('outbound_disabled')) return ['outbound_disabled', text(instance, 'outbound_disabled_detail', 'Outgoing calls are not enabled or no allowed destinations are configured.')];
        return null;
    }

    function renderDialer(instance, snapshot, capabilities) {
        const blocker = setupBlocker(instance, snapshot);
        const disabled = blocker || !capabilities.dial;
        return `<div class="sip-phone-dialer">
            <header>
                <h1>${instance.context.esc(text(instance, 'dial_prompt', 'Enter a SIP number or name'))}</h1>
                <p>${instance.context.esc(text(instance, 'dial_example', 'For example 102, sip:102@example.local or +49 30 1234567'))}</p>
            </header>
            ${blocker ? `<div class="sip-phone-blocker" data-blocker="${instance.context.esc(blocker[0])}">
                <strong>${instance.context.esc(text(instance, 'unavailable', 'Calling is unavailable'))}</strong>
                <span>${instance.context.esc(blocker[1])}</span>
                <a href="/config#sip" target="_blank" rel="noopener">${instance.context.esc(text(instance, 'open_configuration', 'Open Configuration → SIP Phone'))}</a>
            </div>` : ''}
            ${snapshot.error ? `<div class="sip-phone-inline-error" role="alert">${instance.context.esc(snapshot.error)}</div>` : ''}
            <div class="sip-phone-target">
                <input type="text" data-sip-phone="target" value="${instance.context.esc(instance.target)}"
                    placeholder="${instance.context.esc(text(instance, 'target_placeholder', 'SIP number or name'))}"
                    autocomplete="off" spellcheck="false" ${disabled ? 'disabled' : ''}>
                <button type="button" data-sip-phone-action="clear" aria-label="${instance.context.esc(text(instance, 'clear', 'Clear'))}" ${disabled ? 'disabled' : ''}>×</button>
                <button type="button" data-sip-phone-action="favorite" aria-label="${instance.context.esc(text(instance, 'add_favorite', 'Add to favorites'))}" ${disabled ? 'disabled' : ''}>${instance.context.iconMarkup('copy', 'F', 'sip-phone-glyph', 15)}</button>
            </div>
            ${renderKeypad(instance, disabled, false)}
            <button type="button" class="sip-phone-call-button" data-sip-phone-action="dial" ${disabled ? 'disabled' : ''}>
                ${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 30)}
                <span>${instance.context.esc(text(instance, snapshot.phase === 'preparing' ? 'preparing' : 'call', snapshot.phase === 'preparing' ? 'Preparing…' : 'Call'))}</span>
            </button>
            <p class="sip-phone-ready">${instance.context.esc(disabled ? text(instance, 'unavailable', 'Calling is unavailable') : text(instance, 'ready', 'Ready to call'))}</p>
        </div>`;
    }

    function renderKeypad(instance, disabled, compact) {
        return `<div class="sip-phone-keypad ${compact ? 'is-compact' : ''}">
            ${dialKeys.map(([digit, letters]) => `<button type="button" data-sip-digit="${digit}" ${disabled ? 'disabled' : ''}>
                <strong>${digit}</strong><small>${letters || '&nbsp;'}</small>
            </button>`).join('')}
        </div>`;
    }

    function renderActiveCall(instance, snapshot) {
        const call = snapshot.call;
        const name = partyLabel(instance, call.remote_party);
        const subtitle = name === call.remote_party ? '' : call.remote_party;
        return `<div class="sip-phone-active-call">
            ${snapshot.observer ? `<div class="sip-phone-observer">${instance.context.esc(text(instance, 'observer_mode', 'This tab can observe the call but cannot take over its audio.'))}</div>` : ''}
            ${snapshot.error ? `<div class="sip-phone-inline-error" role="alert">${instance.context.esc(snapshot.error)}</div>` : ''}
            <div class="sip-phone-call-avatar">${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 44)}</div>
            <p>${instance.context.esc(text(instance, call.state === 'ringing' ? 'incoming_title' : 'call_active', call.state === 'ringing' ? 'Incoming call' : 'Call active'))}</p>
            <h1>${instance.context.esc(name)}</h1>
            ${subtitle ? `<span>${instance.context.esc(subtitle)}</span>` : ''}
            <time data-sip-phone-duration>${formatDuration(callDuration(call))}</time>
            <div class="sip-phone-call-controls">
                <button type="button" data-sip-phone-action="mute" class="${snapshot.muted ? 'is-active' : ''}" ${snapshot.observer ? 'disabled' : ''}>
                    ${instance.context.iconMarkup('audio', 'A', 'sip-phone-glyph', 21)}<span>${instance.context.esc(text(instance, snapshot.muted ? 'unmute' : 'mute', snapshot.muted ? 'Unmute' : 'Mute'))}</span>
                </button>
                <button type="button" data-sip-phone-action="toggle-keypad" ${snapshot.observer ? 'disabled' : ''}>
                    ${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 21)}<span>${instance.context.esc(text(instance, 'keypad', 'Keypad'))}</span>
                </button>
            </div>
            <label class="sip-phone-volume"><span>${instance.context.esc(text(instance, 'speaker_volume', 'Speaker volume'))}</span>
                <input type="range" data-sip-phone="volume" min="0" max="1" step="0.05" value="${Number(snapshot.preferences.volume || 0)}" ${snapshot.observer ? 'disabled' : ''}>
            </label>
            <div class="sip-phone-active-keypad" ${instance.keypadOpen ? '' : 'hidden'}>${renderKeypad(instance, snapshot.observer, true)}</div>
            <button type="button" class="sip-phone-hangup" data-sip-phone-action="hangup" ${snapshot.observer ? 'disabled' : ''}>
                ${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 19)}<span>${instance.context.esc(text(instance, 'hangup', 'Hang up'))}</span>
            </button>
        </div>`;
    }

    function renderHistory(instance, calls) {
        return `<section class="sip-phone-history-section">
            <div class="sip-phone-section-heading"><h2>${instance.context.esc(text(instance, 'recent_calls', 'Recent calls'))}</h2><span>${Math.min(calls.length, 50)}</span></div>
            <div class="sip-phone-history-list">
                ${calls.length ? calls.slice(0, 50).map(call => {
                    const label = partyLabel(instance, call.remote_party);
                    const statusClass = call.direction === 'inbound' ? 'is-inbound' : 'is-outbound';
                    return `<article class="sip-phone-history-item ${statusClass}">
                        <button type="button" class="sip-phone-history-main" data-sip-redial="${instance.context.esc(call.remote_party)}">
                            <span class="sip-phone-history-icon">${instance.context.iconMarkup('phone', 'P', 'sip-phone-glyph', 17)}</span>
                            <span><strong>${instance.context.esc(label)}</strong><small>${instance.context.esc(call.remote_party)}</small></span>
                            <time>${instance.context.esc(callTime(call))}</time>
                        </button>
                        <div>
                            <small>${instance.context.esc(text(instance, call.direction === 'inbound' ? 'incoming' : 'outgoing', call.direction === 'inbound' ? 'Incoming' : 'Outgoing'))} · ${formatDuration(callDuration(call))}</small>
                            <button type="button" data-sip-copy="${instance.context.esc(call.remote_party)}" title="${instance.context.esc(text(instance, 'copy', 'Copy'))}">${instance.context.iconMarkup('copy', 'C', 'sip-phone-glyph', 14)}</button>
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
        instance.host.querySelectorAll('[data-sip-digit]').forEach(button => button.addEventListener('click', () => {
            const digit = button.dataset.sipDigit;
            if (instance.snapshot.call) runtime.sendDTMF(digit).catch(error => showError(instance, error));
            else {
                instance.target += digit;
                const target = instance.host.querySelector('[data-sip-phone="target"]');
                if (target) {
                    target.value = instance.target;
                    target.focus();
                }
            }
        }));
        instance.host.querySelector('[data-sip-phone-action="clear"]')?.addEventListener('click', () => {
            instance.target = '';
            const target = instance.host.querySelector('[data-sip-phone="target"]');
            if (target) target.value = '';
        });
        instance.host.querySelector('[data-sip-phone-action="dial"]')?.addEventListener('click', () => startDial(instance));
        instance.host.querySelector('[data-sip-phone-action="favorite"]')?.addEventListener('click', () => addFavorite(instance));
        instance.host.querySelector('[data-sip-phone-action="mute"]')?.addEventListener('click', () => runtime.setMuted(!instance.snapshot.muted));
        instance.host.querySelector('[data-sip-phone-action="toggle-keypad"]')?.addEventListener('click', () => {
            instance.keypadOpen = !instance.keypadOpen;
            render(instance, runtime.getState());
        });
        instance.host.querySelector('[data-sip-phone-action="hangup"]')?.addEventListener('click', () => runtime.hangup().catch(error => showError(instance, error)));
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
            keypadOpen: false,
            snapshot: runtime.getState(),
            unsubscribe: null,
            timer: null
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
        instances.delete(windowId);
    }

    window.SipPhoneApp = { render: mount, dispose };
})();
