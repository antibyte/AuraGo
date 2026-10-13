// Configuration uses the shared saved/draft transaction; rendering never probes RuView.
function renderRuViewSection(section) {
    const c = { enabled: false, readonly: true, agent_access: false, mode: 'docker', source: 'esp32', http_port: 3022, udp_port: 5005, storage_mb: 1024, ...configData.ruview };
    const form = window.AuraConfigForm;
    const tr = key => t('ruview.' + key);
    const field = (key, label, options = {}) => form.field({
        id: 'ruview-' + key, path: 'ruview.' + key, label: tr(label), value: c[key], ...options
    });
    const toggle = (key, label, help) => field(key, label, { type: 'toggle', help: help ? tr(help) : '', value: c[key] === true });
    const select = (key, values) => field(key, key, {
        type: 'select', help: tr(key + '_help_' + c[key]),
        options: values.map(value => ({ value, label: tr(value) }))
    });
    const button = (id, label, primary = false) => `<button id="ruview-${id}" type="button" class="${primary ? 'btn-save' : 'btn-secondary'}">${escapeHtml(tr(label))}</button>`;

    document.getElementById('content').innerHTML = form.renderSpec({
        label: section.label,
        desc: tr('quality'),
        groups: [
            { titleKey: 'config.refresh.operation', content:
                toggle('enabled', 'enabled') +
                toggle('readonly', 'readonly', 'readonly_help') +
                '<div id="ruview-readonly-notice" class="cfg-note is-warning" role="status">' + escapeHtml(tr('readonly_notice')) + '</div>' +
                '<div class="field-grid two-cols">' + select('mode', ['docker', 'external', 'native']) + select('source', ['esp32', 'wifi', 'simulated']) + '</div>' +
                '<div id="ruview-mode-notice" class="cfg-note is-warning" role="status" hidden></div>' +
                form.disclosure({ title: tr('permissions'), content:
                    toggle('agent_access', 'agent_access', 'agent_help')
                })
            },
            { titleKey: 'config.refresh.connection', content:
                '<div id="ruview-external-fields">' +
                    field('url', 'url', { type: 'url', placeholder: 'http://192.168.1.20:3000' }) +
                    field('api_token', 'token', { type: 'password', help: tr('token_help') }) +
                    toggle('allow_private', 'allow_private') +
                '</div>' +
                '<div id="ruview-managed-fields">' + form.note({ text: tr('managed_help') }) +
                    form.disclosure({ content: '<div class="field-grid two-cols">' +
                        field('http_port', 'http_port', { type: 'number', min: 1024, max: 65534, step: 1 }) +
                        field('storage_mb', 'storage', { type: 'number', min: 64, max: 1048576, step: 1 }) + '</div>'
                    }) +
                '</div>' +
                form.actions([{ html: button('check', 'test', true) }, { html:
                    `<a class="btn-secondary" href="/desktop?app=ruview" target="_blank" rel="noopener">${escapeHtml(tr('open'))}</a>`
                }]) +
                '<p id="ruview-check-reason" class="field-help" aria-live="polite"></p>' +
                '<div id="ruview-result" class="pw-status" role="status" aria-live="polite" hidden></div>'
            },
            { title: tr('configure'), desc: tr('setup_help'), className: 'ruview-board-panel', content:
                form.note({ text: tr('receiver_purpose') }) +
                '<div class="field-grid two-cols">' +
                    field('receiver_ip', 'receiver', { help: tr('receiver_help'), placeholder: '192.168.1.20' }) +
                    field('udp_port', 'udp_port', { type: 'number', min: 1024, max: 65535, step: 1 }) +
                '</div>' +
                '<div id="ruview-receiver-suggestion" hidden>' +
                    form.actions([{ html: button('use-receiver', 'use_receiver') }]) +
                    '<p class="field-help">' + escapeHtml(tr('receiver_suggestion_help')) + '</p>' +
                '</div>' +
                '<div id="ruview-managed-udp">' + toggle('udp_enabled', 'udp_enabled', 'udp_help') +
                    '<div id="ruview-udp-fields">' +
                        field('udp_bind', 'udp_bind', { placeholder: '192.168.1.20' }) +
                        field('udp_allow', 'udp_allow', { value: (c.udp_allow || []).join(', '), help: tr('udp_allow_help'), placeholder: '192.168.1.50, 192.168.1.51' }) +
                    '</div>' +
                '</div>' +
                form.actions([{ html: button('setup', 'configure', true) }]) +
                '<p id="ruview-setup-reason" class="field-help" aria-live="polite"></p>' +
                '<div id="ruview-setup-result" class="pw-status" role="status" aria-live="polite" hidden></div>'
            }
        ]
    });
    const root = document.querySelector('#content .cfg-section');
    root.id = 'ruview-config';
    root.querySelector('#ruview-udp_allow').dataset.type = 'array';
    root.querySelector('#ruview-api_token').dataset.path = 'ruview.api_token';
    root.querySelector('#ruview-api_token').autocomplete = 'new-password';

    let privateIPv4;
    // Keep conditional fields mounted so changing modes never loses a draft or its bindings.
    const sync = () => {
        const mode = root.querySelector('#ruview-mode').value;
        const source = root.querySelector('#ruview-source').value;
        const external = mode === 'external';
        root.querySelector('#ruview-readonly-notice').hidden = !root.querySelector('[data-path="ruview.readonly"]').classList.contains('on');
        root.querySelector('#ruview-external-fields').hidden = !external;
        root.querySelector('#ruview-managed-fields').hidden = external;
        root.querySelector('.ruview-board-panel').hidden = source !== 'esp32';
        root.querySelector('#ruview-managed-udp').hidden = external;
        root.querySelector('#ruview-udp-fields').hidden = !root.querySelector('[data-path="ruview.udp_enabled"]').classList.contains('on');
        root.querySelector('#ruview-mode-help').textContent = tr('mode_help_' + mode);
        root.querySelector('#ruview-source-help').textContent = tr('source_help_' + source);
        root.querySelector('#ruview-source option[value="wifi"]').disabled = mode === 'docker';
        const notice = root.querySelector('#ruview-mode-notice');
        notice.hidden = mode !== 'docker' || source !== 'wifi';
        notice.textContent = tr('wifi_mode');
        let endpointHost = '';
        try {
            const endpoint = external ? root.querySelector('#ruview-url').value : window.AuraConfigState.get('docker.host');
            endpointHost = new URL(endpoint).hostname;
        } catch (_) { /* Socket endpoints have no IP address. */ }
        const candidates = external ? [endpointHost] : [root.querySelector('#ruview-udp_bind').value, ...(mode === 'docker' ? [endpointHost] : []), location.hostname];
        const address = candidates.find(ip => privateIPv4?.(ip)) || '';
        root.querySelector('#ruview-receiver-suggestion').hidden = !address || !!root.querySelector('#ruview-receiver_ip').value;
        const useReceiver = root.querySelector('#ruview-use-receiver');
        useReceiver.dataset.address = address;
        useReceiver.textContent = tr('use_receiver').replace('{ip}', address);
        for (const action of ['check', 'setup']) {
            const reason = ruviewConfigActionReason(action);
            const control = root.querySelector('#ruview-' + action);
            control.disabled = !!reason || control.getAttribute('aria-busy') === 'true';
            control.setAttribute('aria-describedby', 'ruview-' + action + '-reason');
            const hint = root.querySelector('#ruview-' + action + '-reason');
            hint.textContent = reason;
            hint.hidden = !reason;
        }
    };
    const lifecycle = new AbortController();
    document.addEventListener('cfg:statechange', sync, { signal: lifecycle.signal });
    window.addEventListener('cfg:section-leave', () => lifecycle.abort(), { once: true, signal: lifecycle.signal });
    root.querySelector('#ruview-check').addEventListener('click', ruviewConfigTest);
    root.querySelector('#ruview-setup').addEventListener('click', ruviewConfigFlash);
    root.querySelector('#ruview-use-receiver').addEventListener('click', event => {
        const address = event.currentTarget.dataset.address;
        if (!privateIPv4?.(address)) return;
        for (const key of root.querySelector('#ruview-mode').value === 'external' ? ['receiver_ip'] : ['receiver_ip', 'udp_bind']) {
            const input = root.querySelector('#ruview-' + key);
            if (input.value) continue;
            input.value = address;
            input.dispatchEvent(new Event('input', { bubbles: true }));
        }
    });
    import('/js/ruview/protocol.mjs?v=' + encodeURIComponent(CONFIG_ASSET_VERSION)).then(module => {
        if (!root.isConnected) return;
        privateIPv4 = module.privateIPv4;
        sync();
    }).catch(() => { /* Manual address entry remains available if the optional suggestion cannot load. */ });
    sync();
}

function ruviewConfigActionReason(action) {
    const c = window.AuraConfigState.get('ruview') || {};
    const tr = key => t('ruview.' + key);
    if (hasUnsavedConfigChanges()) return tr('save_first');
    if (!c.enabled) return tr('enable_first');
    if (c.mode === 'docker' && c.source === 'wifi') return tr('wifi_mode');
    if (c.mode === 'external' && !c.url) return t('config.precision.action_missing_fields');
    if (action === 'setup') {
        if (c.readonly) return tr('write_first');
        if (c.source !== 'esp32') return tr('source_help_esp32');
    }
    return '';
}

async function ruviewConfigTest() {
    const button = document.getElementById('ruview-check');
    const out = document.getElementById('ruview-result');
    if (!button || !out || button.getAttribute('aria-busy') === 'true') return;
    const reason = ruviewConfigActionReason('check');
    out.hidden = false;
    if (reason) { out.textContent = reason; return; }
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    out.textContent = t('ruview.working');
    try {
        const response = await fetch('/api/ruview/test', { method: 'POST' });
        const data = await response.json();
        out.textContent = response.ok
            ? t('ruview.' + (data.fresh ? 'received' : data.data_state || 'unchecked'))
            : data.message || data.error || t('ruview.error');
    } catch (_) {
        out.textContent = t('ruview.error');
    } finally {
        button.setAttribute('aria-busy', 'false');
        button.disabled = !!ruviewConfigActionReason('check');
    }
}

async function ruviewConfigFlash() {
    const button = document.getElementById('ruview-setup');
    const out = document.getElementById('ruview-setup-result');
    if (!button || !out || button.getAttribute('aria-busy') === 'true') return;
    const reason = ruviewConfigActionReason('setup');
    if (reason) { out.hidden = false; out.textContent = reason; return; }
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    out.hidden = false;
    out.textContent = t('ruview.working');
    try {
        const version = '?v=' + encodeURIComponent(CONFIG_ASSET_VERSION);
        if (!window.AuraLazyAssets) await import('/js/shared/lazy-assets.js' + version);
        await window.AuraLazyAssets.loadStyle('/css/ruview.css' + version);
        const { openWizard } = await import('/js/ruview/wizard.mjs' + version);
        if (!button.isConnected) return;
        const currentReason = ruviewConfigActionReason('setup');
        if (currentReason) { out.textContent = currentReason; return; }
        out.hidden = true;
        await openWizard({ t, receiverIP: configData.ruview.receiver_ip, port: configData.ruview.udp_port });
    } catch (_) {
        out.hidden = false;
        out.textContent = t('ruview.error');
    } finally {
        button.setAttribute('aria-busy', 'false');
        button.disabled = !!ruviewConfigActionReason('setup');
    }
}
