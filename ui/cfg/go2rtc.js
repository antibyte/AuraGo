// cfg/go2rtc.js — managed go2rtc sidecar configuration

let _go2rtcSection = null;

function go2rtcTuyaOpen(mode) {
	window.AuraGoTuya.open({ mode, t,
		onCreated(result) { showToast(t('common.go2rtc_tuya.' + (result.runtime_reconciled === false ? 'saved_pending' : 'saved')), result.runtime_reconciled === false ? 'warning' : 'success'); },
		onChanged(result) {
			const data = go2rtcEnsureData();
			const state = window.AuraConfigState;
			if (state && state.syncFromDOM) state.syncFromDOM();
			if (result.accounts) {
				data.tuya_accounts = result.accounts;
				if (state && state.markSaved) state.markSaved('go2rtc.tuya_accounts', result.accounts);
			}
			if (result.stream) {
				const draft = JSON.parse(JSON.stringify(go2rtcStreams()));
				const saved = JSON.parse(JSON.stringify(state && state.get('go2rtc.streams', { saved: true }) || []));
				for (const list of [draft, saved]) if (!list.some(s => s.id === result.stream.id)) list.push(result.stream);
				if (state && state.markSaved) state.markSaved('go2rtc.streams', saved);
				data.streams = draft;
				if (state && state.replace) state.replace('go2rtc.streams', draft, () => renderGo2RTCSection(null));
			}
		},
		onClose() { if (window.AuraConfigState && window.AuraConfigState.syncFromDOM) window.AuraConfigState.syncFromDOM(); renderGo2RTCSection(null); }
	});
}

function go2rtcEnsureData() {
    if (!configData.go2rtc) configData.go2rtc = {};
    const data = configData.go2rtc;
    if (!data.webrtc) data.webrtc = {};
    if (!data.hardware) data.hardware = { backend: 'software', render_node: '/dev/dri/renderD128', render_group: '', device_id: '0' };
    if (!data.hardware.backend) data.hardware.backend = 'software';
    const state = window.AuraConfigState;
    const backendPath = 'go2rtc.hardware.backend';
    if (state && state.markSaved && [undefined, ''].includes(state.get(backendPath, { saved: true })) && [undefined, ''].includes(state.get(backendPath))) state.markSaved(backendPath, 'software');
    if (!Array.isArray(data.streams)) data.streams = [];
    if (!data.image) data.image = 'alexxit/go2rtc:1.9.14@sha256:675c318b23c06fd862a61d262240c9a63436b4050d177ffc68a32710d9e05bae';
    if (!data.container_name) data.container_name = 'aurago_go2rtc';
    if (!data.api_host_port) data.api_host_port = 1984;
    if (!data.webrtc.port) data.webrtc.port = 8555;
    return data;
}

// The server replaces go2rtc.streams wholesale, so the draft treats the list as
// one value. Stream cards edit the draft; adopt it before rendering or changing
// the list so typed edits and fields the cards do not show (detection) survive.
function go2rtcStreams() {
    const data = go2rtcEnsureData();
    const state = window.AuraConfigState;
    if (state && typeof state.markAtomic === 'function') {
        state.markAtomic('go2rtc.streams');
        const draft = state.get('go2rtc.streams');
        if (Array.isArray(draft)) data.streams = JSON.parse(JSON.stringify(draft));
    }
    // Legacy server models serialize unset media enums as empty strings. Give
    // neutral defaults the same baseline without accepting any actual edits.
    const saved = state && state.get ? state.get('go2rtc.streams', { saved: true }) : null;
    data.streams.forEach((stream, index) => {
        for (const [key, value] of Object.entries(go2rtcStreamDefaults())) {
            const neutral = value => value === undefined || (value === '' && !!go2rtcMediaChoices[key]);
            if (!neutral(stream[key])) continue;
            stream[key] = value;
            const path = 'go2rtc.streams.' + index + '.' + key;
            if (state && state.markSaved && saved && saved[index] && saved[index].id === stream.id && neutral(saved[index][key]) && neutral(state.get(path))) state.markSaved(path, value);
        }
    });
    return data.streams;
}

function go2rtcSetStreams(streams) {
    go2rtcEnsureData().streams = streams;
    const render = () => renderGo2RTCSection(null);
    // Render before any listener reads the controls back into the draft.
    if (window.AuraConfigState) window.AuraConfigState.replace('go2rtc.streams', streams, render);
    else render();
    setDirty(true);
}

function renderGo2RTCSection(section) {
    if (section) { _go2rtcSection = section; go2rtcSourceForms = {}; }
    section = section || _go2rtcSection;
    const data = go2rtcEnsureData();
    const streams = go2rtcStreams();

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">🎥 ' + t('config.go2rtc.proxy_note') + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info" id="go2rtc-detection-hint">' + t('config.go2rtc.detection_hint') + ' <a href="/desktop?app=network-cameras" target="_blank" rel="noopener">' + t('config.go2rtc.open_camera_app') + '</a></div>';
    html += go2rtcToggle('go2rtc.enabled', data.enabled === true, 'config.go2rtc.enabled', 'help.go2rtc.enabled', true);
    html += go2rtcToggle('go2rtc.auto_start', data.auto_start !== false, 'config.go2rtc.auto_start', 'help.go2rtc.auto_start');
    html += go2rtcToggle('go2rtc.agent_access', data.agent_access !== false, 'config.go2rtc.agent_access', 'help.go2rtc.agent_access');
    html += go2rtcToggle('go2rtc.store_media', data.store_media !== false, 'config.go2rtc.store_media', 'help.go2rtc.store_media');
    html += go2rtcToggle('go2rtc.web_ui_enabled', data.web_ui_enabled === true, 'config.go2rtc.web_ui_enabled', 'help.go2rtc.web_ui_enabled');

    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.go2rtc.runtime_title') + '</div>';
    // These actions work on the running sidecar: unsaved settings lock them (data-config-action-draft="false").
    html += '<div class="field-group">' + cfgStatusBanner.html('go2rtc-runtime-status', 'unknown', { detail: t('config.go2rtc.status_unknown') });
    html += '<a class="btn-secondary" href="/desktop?app=network-cameras" target="_blank" rel="noopener">' + t('config.go2rtc.open_camera_app') + '</a> ';
    html += '<button type="button" class="btn-save" onclick="go2rtcTest()" id="go2rtc-test" data-config-action-draft="false">' + t('config.go2rtc.test') + '</button> ';
    html += '<button type="button" class="btn-secondary" onclick="go2rtcControl(\'start\')" id="go2rtc-start" data-config-action-draft="false">' + t('config.go2rtc.start') + '</button> ';
    html += '<button type="button" class="btn-secondary" onclick="go2rtcControl(\'stop\')" id="go2rtc-stop" data-config-action-draft="false">' + t('config.go2rtc.stop') + '</button> ';
    html += '<button type="button" class="btn-secondary" onclick="go2rtcControl(\'restart\')" id="go2rtc-restart" data-config-action-draft="false">' + t('config.go2rtc.restart') + '</button>';
    if (data.web_ui_enabled === true) {
        html += ' <a class="btn-secondary" href="/api/go2rtc/proxy/" target="_blank" rel="noopener">' + t('config.go2rtc.open_web_ui') + '</a>';
    }
    html += cfgActionResult.html('go2rtc-runtime-result');
    html += '</div>';

    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.go2rtc.streams_title') + '</div>';
	 html += '<div class="field-group"><button type="button" class="btn-secondary" onclick="go2rtcTuyaOpen(\'accounts\')">' + t('common.go2rtc_tuya.accounts') + '</button> <button type="button" class="btn-secondary" onclick="go2rtcTuyaOpen(\'setup\')">' + t('common.go2rtc_tuya.add_camera') + '</button></div>';
    html += '<div class="field-help">' + t('help.go2rtc.streams') + '</div>';
    streams.forEach((stream, index) => {
        const base = 'go2rtc.streams.' + index;
        const lockedID = stream.source_configured === true;
        html += '<div class="cfg-card go2rtc-stream-card">';
        html += '<div class="cfg-card-title">' + escapeHtml(stream.name || stream.id || (t('config.go2rtc.stream') + ' ' + (index + 1))) + '</div>';
        html += go2rtcToggle(base + '.enabled', stream.enabled !== false, 'config.go2rtc.stream_enabled', 'help.go2rtc.stream_enabled');
        html += go2rtcField(base + '.id', stream.id || '', 'config.go2rtc.stream_id', 'help.go2rtc.stream_id', 'front-door', lockedID);
        html += go2rtcField(base + '.name', stream.name || '', 'config.go2rtc.stream_name', 'help.go2rtc.stream_name', t('config.go2rtc.stream_name_placeholder'), false);
		if (stream.tuya) {
			html += go2rtcSelect(base + '.tuya.account_id', stream.tuya.account_id, 'common.go2rtc_tuya.account', (data.tuya_accounts || []).map(a => [a.id, a.name]));
			html += go2rtcSelect(base + '.tuya.main_resolution', stream.tuya.main_resolution, 'common.go2rtc_tuya.main', [['hd', 'HD'], ['sd', 'SD']]);
			html += go2rtcSelect(base + '.tuya.sub_resolution', stream.tuya.sub_resolution || '', 'common.go2rtc_tuya.sub', [['', t('common.go2rtc_tuya.none')], ['sd', 'SD'], ['hd', 'HD']]);
			html += '<p class="field-help">' + t('common.go2rtc_tuya.cloud_notice') + '</p>';
		} else html += go2rtcSourceBlock(stream, index, base);
        html += '<details class="cfg-advanced-panel"><summary class="cfg-advanced-summary">' + t('config.go2rtc.media_settings') + '</summary><div class="cfg-advanced-body">';
        for (const key of ['sub_source', 'snapshot_source']) {
			if (stream.tuya) continue;
            html += go2rtcSecret(base + '.' + key, stream[key] || '', 'config.go2rtc.' + key, '', t('config.go2rtc.keep_source'));
            html += go2rtcToggle(base + '.clear_' + key, stream['clear_' + key] === true, 'config.go2rtc.clear_' + key, '');
        }
		for (const [key, options] of Object.entries(go2rtcMediaChoices)) {
			if (stream.tuya && key === 'transport') continue;
			html += go2rtcSelect(base + '.' + key, stream[key] || options[0][0], 'config.go2rtc.' + key, options.filter(([value]) => !(stream.tuya && key === 'snapshot_mode' && value === 'camera')));
		}
        html += go2rtcToggle(base + '.preload', stream.preload === true, 'config.go2rtc.preload', '');
        html += go2rtcToggle(base + '.talk_enabled', stream.talk_enabled === true, 'config.go2rtc.talk_enabled', '');
        html += '</div></details>';
        html += '<div class="field-group">';
        html += '<button type="button" class="btn-save" id="go2rtc-snapshot-' + index + '" data-config-action-draft="false" onclick="go2rtcSnapshot(' + index + ')">' + t('config.go2rtc.snapshot') + '</button> ';
        html += '<button type="button" class="btn-secondary" data-config-action-draft="false" onclick="go2rtcOpenViewer(' + index + ')">' + t('config.go2rtc.viewer') + '</button> ';
        html += '<button type="button" class="btn-secondary" id="go2rtc-diagnose-' + index + '" data-config-action-draft="false" onclick="go2rtcDiagnose(' + index + ')">' + t('config.go2rtc.diagnose') + '</button> ';
        html += '<button type="button" class="btn-secondary" onclick="go2rtcRemoveStream(' + index + ')">' + t('config.go2rtc.remove') + '</button>';
        html += cfgActionResult.html('go2rtc-stream-result-' + index);
        html += '<div id="go2rtc-stream-preview-' + index + '"></div>';
        html += '</div></div>';
    });
    html += '<div class="field-group"><button type="button" class="btn-save" onclick="go2rtcAddStream()">' + t('config.go2rtc.add_stream') + '</button></div>';

    html += '<details class="cfg-advanced-panel"><summary class="cfg-advanced-summary">⚙️ ' + t('config.go2rtc.advanced') + '</summary><div class="cfg-advanced-body">';
    html += go2rtcSelect('go2rtc.hardware.backend', data.hardware.backend || 'software', 'config.go2rtc.hardware_backend', [['software', 'CPU'], ['vaapi', 'VAAPI'], ['cuda', 'CUDA / NVENC']], true, 'config.go2rtc.hardware_help');
    if (data.hardware.backend === 'vaapi') {
        html += go2rtcField('go2rtc.hardware.render_node', data.hardware.render_node || '/dev/dri/renderD128', 'config.go2rtc.hardware_node', '', '/dev/dri/renderD128', false);
        html += go2rtcField('go2rtc.hardware.render_group', data.hardware.render_group || '', 'config.go2rtc.hardware_group', '', '993', false);
    }
    if (data.hardware.backend === 'cuda') html += go2rtcField('go2rtc.hardware.device_id', data.hardware.device_id || '0', 'config.go2rtc.hardware_device', '', '0', false);
    html += '<div class="field-group"><button type="button" class="btn-secondary" id="go2rtc-hardware-test" data-config-action-draft="false" onclick="go2rtcHardwareTest()">' + t('config.go2rtc.hardware_test') + '</button>' + cfgActionResult.html('go2rtc-hardware-result') + '</div>';
    html += go2rtcField('go2rtc.image', data.image, 'config.go2rtc.image', 'help.go2rtc.image', '', false);
    html += go2rtcField('go2rtc.container_name', data.container_name, 'config.go2rtc.container_name', 'help.go2rtc.container_name', 'aurago_go2rtc', false);
    html += go2rtcToggle('go2rtc.webrtc.enabled', data.webrtc.enabled === true, 'config.go2rtc.webrtc_enabled', 'help.go2rtc.webrtc_enabled', true);
    if (data.webrtc.enabled === true) {
        html += go2rtcField('go2rtc.webrtc.bind_address', data.webrtc.bind_address || '', 'config.go2rtc.webrtc_bind', 'help.go2rtc.webrtc_bind', '192.168.1.10', false);
        html += go2rtcNumber('go2rtc.webrtc.port', data.webrtc.port || 8555, 'config.go2rtc.webrtc_port', 'help.go2rtc.webrtc_port', 1, 65535);
    }
    html += '</div></details></div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    go2rtcBindSourceForms();
    go2rtcRefreshStatus();
    if (go2rtcSourceTypes === null) {
        go2rtcSourceTypes = [];
        go2rtcLoadSourceTypes().then(types => {
            go2rtcSourceTypes = types;
            if (!go2rtcIsActive()) return;
            if (window.AuraConfigState && window.AuraConfigState.syncFromDOM) window.AuraConfigState.syncFromDOM();
            renderGo2RTCSection(null);
        }).catch(() => {
            go2rtcSourceTypes = null;
            if (!go2rtcLoadFailureShown) { go2rtcLoadFailureShown = true; showToast(t('common.go2rtc_source.load_failed'), 'error'); }
        });
    }
}

let go2rtcSourceTypes = null;
let go2rtcLoadFailureShown = false;

function go2rtcIsActive() {
    return (typeof activeSection === 'undefined' || activeSection === 'go2rtc') && !!document.querySelector('[data-go2rtc-source]');
}
let go2rtcSourceForms = {};

function go2rtcLoadSourceTypes() {
    return window.AuraGoSourceForm.loadCatalog(() => fetch('/api/go2rtc/source-types', { credentials: 'same-origin' }).then(r => r.ok ? r.json() : Promise.reject(new Error(String(r.status)))));
}

// The main source is a guided form. The hidden data-path input carries the value
// the config save already understands: the saved (masked) source until the form
// builds a new URL. Secrets typed into the form live only in the form's memory.
function go2rtcSourceBlock(stream, index, base) {
    const state = go2rtcSourceForms[index] || (go2rtcSourceForms[index] = window.AuraGoSourceForm.newState(''));
    const saved = window.AuraConfigState && window.AuraConfigState.get ? window.AuraConfigState.get('go2rtc.streams', { saved: true }) : null;
    const savedStream = Array.isArray(saved) ? saved.find(item => item && item.id === stream.id) : null;
    const original = savedStream ? (savedStream.source || '') : (stream.source || '');
    const badge = stream.source_type ? '<span class="go2rtc-source-badge">' + escapeHtml(window.AuraGoSourceForm.label(go2rtcSourceTypes, stream.source_type)) + '</span>' : '';
    return '<div class="field-group"><div class="field-label">' + t('config.go2rtc.source') + badge + '</div><div class="field-help">' + t('help.go2rtc.source') + '</div>' +
        '<input type="hidden" data-path="' + escapeAttr(base + '.source') + '" data-original="' + escapeAttr(original) + '" value="">' +
        '<div data-go2rtc-source="' + index + '">' + go2rtcSourceMarkup(index, state) + '</div></div>';
}

function go2rtcSourceMarkup(index, state) {
    const stream = (go2rtcEnsureData().streams || [])[index] || {};
    const empty = t(stream.source_configured ? 'common.go2rtc_source.keep_current' : 'common.go2rtc_source.choose_type');
    return window.AuraGoSourceForm.markup(go2rtcSourceTypes || [], state, { t, emptyLabel: empty, classes: { field: 'field-group', label: 'field-label', input: 'field-input' } });
}

function go2rtcApplySource(index) {
    const state = go2rtcSourceForms[index];
    const hidden = document.querySelector('[data-path="go2rtc.streams.' + index + '.source"]');
    const root = document.querySelector('[data-go2rtc-source="' + index + '"]');
    if (!state || !hidden || !root) return;
    const built = state.type ? window.AuraGoSourceForm.buildURL(go2rtcSourceTypes || [], state) : null;
    hidden.value = built && built.url ? built.url : hidden.dataset.original;
    const error = root.querySelector('.go2rtc-source-error');
    if (error) {
        error.textContent = built && built.error ? window.AuraGoSourceForm.errorText(t, built) : '';
        error.hidden = !(built && built.error);
    }
    hidden.dispatchEvent(new Event('change', { bubbles: true }));
}

// Binds one camera's form; a type change redraws and rebinds only that form.
function go2rtcBindSourceForm(root) {
    const index = Number(root.dataset.go2rtcSource);
    const state = go2rtcSourceForms[index];
    const form = root.querySelector('[data-source-form]');
    if (!state || !form) return;
    window.AuraGoSourceForm.bind(form, state, redraw => {
        if (redraw) {
            root.innerHTML = go2rtcSourceMarkup(index, state);
            go2rtcBindSourceForm(root);
        }
        go2rtcApplySource(index);
    });
}

function go2rtcBindSourceForms() {
    // The draft value (possibly a just-built plaintext URL) is set as a DOM property, never in markup.
    const streams = go2rtcEnsureData().streams || [];
    streams.forEach((stream, index) => {
        const hidden = document.querySelector('[data-path="go2rtc.streams.' + index + '.source"]');
        if (hidden) hidden.value = stream.source || '';
    });
    document.querySelectorAll('[data-go2rtc-source]').forEach(go2rtcBindSourceForm);
}

function go2rtcToggle(path, on, labelKey, helpKey, rerender) {
    let html = '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div>';
    if (helpKey) html += '<div class="field-help">' + t(helpKey) + '</div>';
    html += '<div class="toggle-wrap"><div class="toggle' + (on ? ' on' : '') + '" data-path="' + escapeAttr(path) + '" onclick="toggleBool(this);setNestedValue(configData,\'' + path + '\',this.classList.contains(\'on\'));' + (rerender ? 'renderGo2RTCSection(null);' : '') + '"></div>';
    html += '<span class="toggle-label">' + (on ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span></div></div>';
    return html;
}

function go2rtcField(path, value, labelKey, helpKey, placeholder, readonly) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div>' + (helpKey ? '<div class="field-help">' + t(helpKey) + '</div>' : '') +
        '<input class="field-input" type="text" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value || '') + '" placeholder="' + escapeAttr(placeholder || '') + '"' + (readonly ? ' readonly' : '') + '></div>';
}

function go2rtcSecret(path, value, labelKey, helpKey, placeholder) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div>' + (helpKey ? '<div class="field-help">' + t(helpKey) + '</div>' : '') +
        '<input class="field-input" type="password" autocomplete="off" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value || '') + '" placeholder="' + escapeAttr(placeholder || '') + '"></div>';
}

function go2rtcNumber(path, value, labelKey, helpKey, min, max) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div>' + (helpKey ? '<div class="field-help">' + t(helpKey) + '</div>' : '') +
        '<input class="field-input" type="number" min="' + min + '" max="' + max + '" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value) + '"></div>';
}

function go2rtcAddStream() {
    const streams = go2rtcStreams();
    go2rtcSourceForms = {};
    let suffix = streams.length + 1;
    let id = 'camera-' + suffix;
    while (streams.some(stream => stream.id === id)) {
        suffix += 1;
        id = 'camera-' + suffix;
    }
    streams.push(Object.assign(go2rtcStreamDefaults(), { id, name: t('config.go2rtc.stream_name_placeholder'), enabled: true, source: '', source_configured: false }));
    go2rtcSetStreams(streams);
}

function go2rtcRemoveStream(index) {
    const doomed = go2rtcStreams()[index] || {};
    // A camera without a source has nothing to lose, so it goes without asking.
    if (!doomed.source_configured && !String(doomed.source || '').trim()) {
        go2rtcDropStream(index);
        return;
    }
    const label = doomed.name || doomed.id || String(index + 1);
    showConfirm(t('config.go2rtc.confirm_remove_title'), t('config.go2rtc.confirm_remove_message', { name: label }))
        .then(ok => { if (ok) go2rtcDropStream(index); });
}

function go2rtcDropStream(index) {
    const streams = go2rtcStreams();
    go2rtcSourceForms = {};
    streams.splice(index, 1);
    go2rtcSetStreams(streams);
}

function go2rtcSavedOnly(resultID) {
    if (!isDirty) return true;
    if (resultID) cfgActionResult.show(resultID, 'warning', t('config.go2rtc.save_first'));
    return false;
}

async function go2rtcRefreshStatus() {
    const target = document.getElementById('go2rtc-runtime-status');
    if (!target) return;
    return cfgStatusBanner.check(target, () => fetch('/api/go2rtc/status'), body => ({
        state: body.api_usable ? 'connected' : 'degraded',
        detail: t('config.go2rtc.status_summary')
            .replace('{container}', body.container_running ? t('config.go2rtc.running') : t('config.go2rtc.stopped'))
            .replace('{api}', body.api_usable ? t('config.go2rtc.available') : t('config.go2rtc.unavailable'))
    }));
}

async function go2rtcTest() {
    if (!go2rtcSavedOnly('go2rtc-runtime-result')) return;
    await go2rtcRequest('/api/go2rtc/test', document.getElementById('go2rtc-test'), 'go2rtc-runtime-result');
    go2rtcRefreshStatus();
}

async function go2rtcControl(action) {
    if (!go2rtcSavedOnly('go2rtc-runtime-result')) return;
    await go2rtcRequest('/api/go2rtc/' + action, document.getElementById('go2rtc-' + action), 'go2rtc-runtime-result');
    go2rtcRefreshStatus();
}

// POST to the running sidecar; the server reason is shown next to the button.
function go2rtcRequest(path, button, resultID) {
    return cfgActionResult(button, async () => {
        const response = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
        const data = await response.json();
        if (!response.ok || data.status === 'error') throw new Error(data.message || ('HTTP ' + response.status));
        return data;
    }, { target: resultID, pending: t('config.go2rtc.working'), success: t('config.go2rtc.action_ok'), failure: t('config.go2rtc.status_error') + ':' });
}

function go2rtcSnapshot(index) {
    const stream = go2rtcEnsureData().streams[index] || {};
    const resultID = 'go2rtc-stream-result-' + index;
    if (!go2rtcSavedOnly(resultID)) return;
    return cfgActionResult(document.getElementById('go2rtc-snapshot-' + index), async () => {
        const response = await fetch('/api/go2rtc/snapshot', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ stream_id: stream.id, cache_seconds: 2 })
        });
        const data = await response.json();
        if (!response.ok || data.status === 'error') throw new Error(data.message || ('HTTP ' + response.status));
        return data;
    }, {
        target: resultID,
        pending: t('config.go2rtc.working'),
        success: t('config.go2rtc.snapshot_ok'),
        failure: t('config.go2rtc.status_error') + ':',
        onSuccess: outcome => {
            const preview = document.getElementById('go2rtc-stream-preview-' + index);
            if (!preview) return;
            const src = outcome.body.web_path || ('/api/go2rtc/proxy/api/frame.jpeg?src=' + encodeURIComponent(stream.id));
            preview.innerHTML = '<img class="go2rtc-stream-preview-image" src="' + escapeAttr(src) + '" alt="' + escapeAttr(stream.name || stream.id) + '">';
        }
    });
}

function go2rtcOpenViewer(index) {
    const stream = go2rtcEnsureData().streams[index] || {};
    if (!go2rtcSavedOnly('go2rtc-stream-result-' + index)) return;
    window.open('/api/go2rtc/viewer/' + encodeURIComponent(stream.id), '_blank', 'noopener');
}

const go2rtcMediaChoices = {
    preview_source: [['auto', 'automatic'], ['main', 'main_stream'], ['sub', 'sub_stream']],
    detection_source: [['auto', 'automatic'], ['main', 'main_stream'], ['sub', 'sub_stream']],
    snapshot_mode: [['auto', 'automatic'], ['camera', 'snapshot_source'], ['stream', 'stream_image']],
    codec_profile: [['original', 'original'], ['h264_opus', 'H.264 / Opus'], ['h264_aac', 'H.264 / AAC']],
    transport: [['inherit', 'automatic'], ['tcp', 'TCP'], ['udp', 'UDP']],
    playback_mode: [['auto', 'automatic'], ['webrtc_tcp', 'WebRTC / TCP'], ['mse', 'MSE'], ['hls', 'HLS'], ['mp4', 'MP4'], ['mjpeg', 'MJPEG']]
};

function go2rtcStreamDefaults() {
    const values = { sub_source: '', snapshot_source: '', clear_sub_source: false, clear_snapshot_source: false, preload: false, talk_enabled: false };
    for (const [key, options] of Object.entries(go2rtcMediaChoices)) values[key] = options[0][0];
    return values;
}

function go2rtcSelect(path, value, label, choices, rerender, helpKey) {
	const literalCaptions = path.includes('.tuya.');
    return '<div class="field-group"><label class="field-label" for="' + escapeAttr(path) + '">' + t(label) + '</label><select class="field-select" id="' + escapeAttr(path) + '" data-path="' + escapeAttr(path) + '"' + (rerender ? ' onchange="setNestedValue(configData,\'' + path + '\',this.value);renderGo2RTCSection(null)"' : '') + '>' + choices.map(([key, caption]) => '<option value="' + key + '" ' + (key === value ? 'selected' : '') + '>' + escapeHtml(literalCaptions || /[A-Z]/.test(caption) ? caption : t('config.go2rtc.' + caption)) + '</option>').join('') + '</select>' + (helpKey ? '<div class="field-help">' + t(helpKey) + '</div>' : '') + '</div>';
}

// The probe verdict is the whole answer (no server reason line), so the text names it exactly.
function go2rtcHardwareTest() {
    if (!go2rtcSavedOnly('go2rtc-hardware-result')) return;
    return cfgActionResult(document.getElementById('go2rtc-hardware-test'), async () => {
        const response = await fetch('/api/go2rtc/hardware/test', {method:'POST',headers:{'Content-Type':'application/json'},body:'{}'});
        const result = await response.json().catch(() => ({}));
        return { status: response.ok && result.verified ? 'ok' : 'error' };
    }, {
        target: 'go2rtc-hardware-result',
        pending: t('config.go2rtc.working'),
        success: t('config.go2rtc.hardware_verified'),
        failure: t('config.go2rtc.hardware_failed')
    });
}

function go2rtcDiagnosisText(key, result) {
    let text = t(key).replace('{width}', result.width || 0).replace('{height}', result.height || 0).replace('{ms}', result.duration_ms || 0);
    if (result.error_code) text += ' (' + result.error_code + ')';
    if (result.effective_transcoding_backend) text += ' · ' + (result.effective_transcoding_backend === 'software' ? 'CPU' : result.effective_transcoding_backend);
    return text;
}

function go2rtcDiagnose(index) {
    const resultID = 'go2rtc-stream-result-' + index;
    if (!go2rtcSavedOnly(resultID)) return;
    const stream = go2rtcEnsureData().streams[index];
    return cfgActionResult(document.getElementById('go2rtc-diagnose-' + index), async () => {
        const response = await fetch('/api/go2rtc/diagnostics/' + encodeURIComponent(stream.id), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
        return response.ok ? response.json() : { status: 'error' };
    }, {
        target: resultID,
        pending: t('config.go2rtc.working'),
        success: outcome => go2rtcDiagnosisText('config.go2rtc.diagnostics_success', outcome.body),
        failure: outcome => go2rtcDiagnosisText('config.go2rtc.diagnostics_failed', outcome.body || {}),
        // The verdict is in the body; its codes are already part of the text.
        interpret: outcome => outcome.body && typeof outcome.body === 'object' ? { state: outcome.body.status === 'ok' ? 'success' : 'error', reason: '' } : null
    });
}
