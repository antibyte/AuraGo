// Configuration uses the shared saved/draft transaction; rendering never probes RuView.
function renderRuViewSection(section) {
    const c = { enabled: false, readonly: true, agent_access: false, mode: 'docker', source: 'esp32', http_port: 3022, udp_port: 5005, storage_mb: 1024, ...configData.ruview };
    const form = window.AuraConfigForm;
    const tr = key => t('ruview.' + key);
    // Literal maps keep every key visible to the static i18n lint; callers pass translated labels.
    const OPTION = {
        docker: () => tr('docker'), external: () => tr('external'), native: () => tr('native'),
        esp32: () => tr('esp32'), wifi: () => tr('wifi'), simulated: () => tr('simulated')
    };
    const MODE_HELP = { docker: () => tr('mode_help_docker'), external: () => tr('mode_help_external'), native: () => tr('mode_help_native') };
    const SOURCE_HELP = { esp32: () => tr('source_help_esp32'), wifi: () => tr('source_help_wifi'), simulated: () => tr('source_help_simulated') };
    const pick = (map, key) => Object.hasOwn(map, key) ? map[key]() : '';
    const field = (key, label, options = {}) => form.field({
        id: 'ruview-' + key, path: 'ruview.' + key, label, value: c[key], ...options
    });
    const toggle = (key, label, help) => field(key, label, { type: 'toggle', help: help || '', value: c[key] === true });
    const select = (key, label, helpMap, values) => field(key, label, {
        type: 'select', help: pick(helpMap, c[key]),
        options: values.map(value => ({ value, label: OPTION[value]() }))
    });
    const button = (id, label, primary = false) => `<button id="ruview-${id}" type="button" class="${primary ? 'btn-save' : 'btn-secondary'}">${escapeHtml(label)}</button>`;

    document.getElementById('content').innerHTML = form.renderSpec({
        label: section.label,
        desc: tr('quality'),
        groups: [
            { titleKey: 'config.refresh.operation', content:
                toggle('enabled', tr('enabled')) +
                toggle('readonly', tr('readonly'), tr('readonly_help')) +
                '<div id="ruview-readonly-notice" class="cfg-note is-warning" role="status">' + escapeHtml(tr('readonly_notice')) + '</div>' +
                '<div class="field-grid two-cols">' + select('mode', tr('mode'), MODE_HELP, ['docker', 'external', 'native']) + select('source', tr('source'), SOURCE_HELP, ['esp32', 'wifi', 'simulated']) + '</div>' +
                '<div id="ruview-mode-notice" class="cfg-note is-warning" role="status" hidden></div>' +
                form.disclosure({ title: tr('permissions'), content:
                    toggle('agent_access', tr('agent_access'), tr('agent_help'))
                })
            },
            { titleKey: 'config.refresh.connection', content:
                '<div id="ruview-external-fields">' +
                    field('url', tr('url'), { type: 'url', placeholder: 'http://192.168.1.20:3000' }) +
                    field('api_token', tr('token'), { type: 'password', help: tr('token_help') }) +
                    toggle('allow_private', tr('allow_private')) +
                '</div>' +
                '<div id="ruview-managed-fields">' + form.note({ text: tr('managed_help') }) +
                    form.disclosure({ content: '<div class="field-grid two-cols">' +
                        field('http_port', tr('http_port'), { type: 'number', min: 1024, max: 65534, step: 1 }) +
                        field('storage_mb', tr('storage'), { type: 'number', min: 64, max: 1048576, step: 1 }) + '</div>'
                    }) +
                '</div>' +
                form.actions([{ html: button('check', tr('test'), true) }, { html:
                    `<a class="btn-secondary" href="/desktop?app=ruview" target="_blank" rel="noopener">${escapeHtml(tr('open'))}</a>`
                }]) +
                '<p id="ruview-check-reason" class="field-help" aria-live="polite"></p>' +
                '<div id="ruview-result" class="pw-status" role="status" aria-live="polite" hidden></div>'
            },
            { title: tr('configure'), desc: tr('setup_help'), className: 'ruview-board-panel', content:
                form.note({ text: tr('receiver_purpose') }) +
                '<div class="field-grid two-cols">' +
                    field('receiver_ip', tr('receiver'), { help: tr('receiver_help'), placeholder: '192.168.1.20' }) +
                    field('udp_port', tr('udp_port'), { type: 'number', min: 1024, max: 65535, step: 1 }) +
                '</div>' +
                '<div id="ruview-receiver-suggestion" hidden>' +
                    form.actions([{ html: button('use-receiver', tr('use_receiver')) }]) +
                    '<p class="field-help">' + escapeHtml(tr('receiver_suggestion_help')) + '</p>' +
                '</div>' +
                '<div id="ruview-managed-udp">' + toggle('udp_enabled', tr('udp_enabled'), tr('udp_help')) +
                    '<div id="ruview-udp-fields">' +
                        field('udp_bind', tr('udp_bind'), { placeholder: '192.168.1.20' }) +
                        field('udp_allow', tr('udp_allow'), { value: (c.udp_allow || []).join(', '), help: tr('udp_allow_help'), placeholder: '192.168.1.50, 192.168.1.51' }) +
                    '</div>' +
                '</div>' +
                form.actions([{ html: button('setup', tr('configure'), true) }]) +
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
        root.querySelector('#ruview-mode-help').textContent = pick(MODE_HELP, mode);
        root.querySelector('#ruview-source-help').textContent = pick(SOURCE_HELP, source);
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

// ruviewTestOutcome turns a POST /api/ruview/test answer into text and a state.
// Only fresh real data counts as success: a reachable service without data is a warning.
function ruviewTestOutcome(ok, data) {
    // Literal maps keep every key visible to the static i18n lint.
    const errors = {
        source_mismatch: () => t('ruview.source_mismatch'),
        unexpected_simulation: () => t('ruview.unexpected_simulation'),
        stop_failed: () => t('ruview.stop_failed'),
        process_exited: () => t('ruview.process_exited')
    };
    const states = {
        unchecked: () => t('ruview.unchecked'),
        disabled: () => t('ruview.disabled'),
        starting: () => t('ruview.starting'),
        stopping: () => t('ruview.stopping'),
        stopped: () => t('ruview.stopped_state'),
        disconnected: () => t('ruview.disconnected'),
        stale: () => t('ruview.stale'),
        synthetic: () => t('ruview.demo'),
        live_unverified: () => t('ruview.unavailable'),
        live: () => t('ruview.unavailable')
    };
    const pick = (map, key, fallback) => typeof key === 'string' && Object.hasOwn(map, key) ? map[key]() : fallback();
    // A different signal source is a configuration mismatch to look at, not a failed test.
    const failure = code => ({ state: code === 'source_mismatch' ? 'warning' : 'error', text: pick(errors, code, () => String(code)) });
    if (!ok) return typeof data.error === 'string' && Object.hasOwn(errors, data.error)
        ? failure(data.error)
        : { state: 'error', text: data.message || data.error || t('ruview.error') };
    if (data.error) return failure(data.error);
    if (data.fresh && !data.simulated) return { state: 'success', text: t('ruview.received') };
    if (data.simulated) return { state: 'warning', text: t('ruview.demo') };
    if (data.waiting_for_frames) return { state: 'warning', text: t('ruview.waiting_for_frames') };
    return { state: 'warning', text: pick(states, data.data_state, () => t('ruview.unchecked')) };
}

function ruviewShowOutcome(out, outcome) {
    out.textContent = outcome ? outcome.text : '';
    if (outcome) out.dataset.state = outcome.state; else delete out.dataset.state;
    out.classList.toggle('is-success', outcome?.state === 'success');
    out.classList.toggle('is-error', outcome?.state === 'error');
}

async function ruviewConfigTest() {
    const button = document.getElementById('ruview-check');
    const out = document.getElementById('ruview-result');
    if (!button || !out || button.getAttribute('aria-busy') === 'true') return;
    const reason = ruviewConfigActionReason('check');
    out.hidden = false;
    if (reason) { ruviewShowOutcome(out, null); out.textContent = reason; return; }
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    ruviewShowOutcome(out, null);
    out.textContent = t('ruview.working');
    try {
        const response = await fetch('/api/ruview/test', { method: 'POST' });
        const data = await response.json();
        ruviewShowOutcome(out, ruviewTestOutcome(response.ok, data));
    } catch (_) {
        ruviewShowOutcome(out, { state: 'error', text: t('ruview.error') });
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
