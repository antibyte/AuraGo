// cfg/go2rtc.js — managed go2rtc sidecar configuration

let _go2rtcSection = null;

function go2rtcEnsureData() {
    if (!configData.go2rtc) configData.go2rtc = {};
    const data = configData.go2rtc;
    if (!data.webrtc) data.webrtc = {};
    if (!Array.isArray(data.streams)) data.streams = [];
    if (!data.image) data.image = 'alexxit/go2rtc:1.9.14@sha256:675c318b23c06fd862a61d262240c9a63436b4050d177ffc68a32710d9e05bae';
    if (!data.container_name) data.container_name = 'aurago_go2rtc';
    if (!data.api_host_port) data.api_host_port = 1984;
    if (!data.webrtc.port) data.webrtc.port = 8555;
    return data;
}

function renderGo2RTCSection(section) {
    if (section) _go2rtcSection = section;
    section = section || _go2rtcSection;
    const data = go2rtcEnsureData();
    const streams = data.streams;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">🎥 ' + t('config.go2rtc.proxy_note') + '</div>';
    html += go2rtcToggle('go2rtc.enabled', data.enabled === true, 'config.go2rtc.enabled', 'help.go2rtc.enabled', true);
    html += go2rtcToggle('go2rtc.auto_start', data.auto_start !== false, 'config.go2rtc.auto_start', 'help.go2rtc.auto_start');
    html += go2rtcToggle('go2rtc.agent_access', data.agent_access !== false, 'config.go2rtc.agent_access', 'help.go2rtc.agent_access');
    html += go2rtcToggle('go2rtc.store_media', data.store_media !== false, 'config.go2rtc.store_media', 'help.go2rtc.store_media');
    html += go2rtcToggle('go2rtc.web_ui_enabled', data.web_ui_enabled === true, 'config.go2rtc.web_ui_enabled', 'help.go2rtc.web_ui_enabled');

    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.go2rtc.runtime_title') + '</div>';
    html += '<div class="field-group"><div id="go2rtc-runtime-status" class="cfg-note-banner">' + t('config.go2rtc.status_unknown') + '</div>';
    html += '<button type="button" class="btn-save" onclick="go2rtcTest()" id="go2rtc-test">' + t('config.go2rtc.test') + '</button> ';
    html += '<button type="button" class="btn-secondary" onclick="go2rtcControl(\'start\')">' + t('config.go2rtc.start') + '</button> ';
    html += '<button type="button" class="btn-secondary" onclick="go2rtcControl(\'stop\')">' + t('config.go2rtc.stop') + '</button> ';
    html += '<button type="button" class="btn-secondary" onclick="go2rtcControl(\'restart\')">' + t('config.go2rtc.restart') + '</button>';
    if (data.web_ui_enabled === true) {
        html += ' <a class="btn-secondary" href="/api/go2rtc/proxy/" target="_blank" rel="noopener">' + t('config.go2rtc.open_web_ui') + '</a>';
    }
    html += '</div>';

    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.go2rtc.streams_title') + '</div>';
    html += '<div class="field-help">' + t('help.go2rtc.streams') + '</div>';
    streams.forEach((stream, index) => {
        const base = 'go2rtc.streams.' + index;
        const lockedID = stream.source_configured === true;
        html += '<div class="cfg-card go2rtc-stream-card">';
        html += '<div class="cfg-card-title">' + escapeHtml(stream.name || stream.id || (t('config.go2rtc.stream') + ' ' + (index + 1))) + '</div>';
        html += go2rtcToggle(base + '.enabled', stream.enabled !== false, 'config.go2rtc.stream_enabled', 'help.go2rtc.stream_enabled');
        html += go2rtcField(base + '.id', stream.id || '', 'config.go2rtc.stream_id', 'help.go2rtc.stream_id', 'front-door', lockedID);
        html += go2rtcField(base + '.name', stream.name || '', 'config.go2rtc.stream_name', 'help.go2rtc.stream_name', t('config.go2rtc.stream_name_placeholder'), false);
        html += go2rtcSecret(base + '.source', stream.source || '', 'config.go2rtc.source', 'help.go2rtc.source', 'rtsp://camera.example/stream');
        html += '<div class="field-group">';
        html += '<button type="button" class="btn-save" onclick="go2rtcSnapshot(' + index + ')">' + t('config.go2rtc.snapshot') + '</button> ';
        html += '<button type="button" class="btn-secondary" onclick="go2rtcOpenViewer(' + index + ')">' + t('config.go2rtc.viewer') + '</button> ';
        html += '<button type="button" class="btn-secondary" onclick="go2rtcRemoveStream(' + index + ')">' + t('config.go2rtc.remove') + '</button>';
        html += '<div id="go2rtc-stream-result-' + index + '" class="adg-test-result"></div>';
        html += '<div id="go2rtc-stream-preview-' + index + '"></div>';
        html += '</div></div>';
    });
    html += '<div class="field-group"><button type="button" class="btn-save" onclick="go2rtcAddStream()">' + t('config.go2rtc.add_stream') + '</button></div>';

    html += '<details class="cfg-advanced-panel"><summary class="cfg-advanced-summary">⚙️ ' + t('config.go2rtc.advanced') + '</summary><div class="cfg-advanced-body">';
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
    go2rtcRefreshStatus();
}

function go2rtcToggle(path, on, labelKey, helpKey, rerender) {
    let html = '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div>';
    html += '<div class="field-help">' + t(helpKey) + '</div>';
    html += '<div class="toggle-wrap"><div class="toggle' + (on ? ' on' : '') + '" data-path="' + escapeAttr(path) + '" onclick="toggleBool(this);setNestedValue(configData,\'' + path + '\',this.classList.contains(\'on\'));' + (rerender ? 'renderGo2RTCSection(null);' : '') + '"></div>';
    html += '<span class="toggle-label">' + (on ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span></div></div>';
    return html;
}

function go2rtcField(path, value, labelKey, helpKey, placeholder, readonly) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div>' +
        '<input class="field-input" type="text" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value || '') + '" placeholder="' + escapeAttr(placeholder || '') + '"' + (readonly ? ' readonly' : '') + '></div>';
}

function go2rtcSecret(path, value, labelKey, helpKey, placeholder) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div>' +
        '<input class="field-input" type="password" autocomplete="off" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value || '') + '" placeholder="' + escapeAttr(placeholder || '') + '"></div>';
}

function go2rtcNumber(path, value, labelKey, helpKey, min, max) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div>' +
        '<input class="field-input" type="number" min="' + min + '" max="' + max + '" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value) + '"></div>';
}

function go2rtcAddStream() {
    const data = go2rtcEnsureData();
    let suffix = data.streams.length + 1;
    let id = 'camera-' + suffix;
    while (data.streams.some(stream => stream.id === id)) {
        suffix += 1;
        id = 'camera-' + suffix;
    }
    data.streams.push({ id, name: t('config.go2rtc.stream_name_placeholder'), enabled: true, source: '', source_configured: false });
    setDirty(true);
    renderGo2RTCSection(null);
}

function go2rtcRemoveStream(index) {
    go2rtcEnsureData().streams.splice(index, 1);
    setDirty(true);
    renderGo2RTCSection(null);
}

function go2rtcSavedOnly(result) {
    if (!isDirty) return true;
    if (result) {
        result.textContent = t('config.go2rtc.save_first');
        result.className = 'adg-test-result is-danger';
    }
    return false;
}

async function go2rtcRefreshStatus() {
    const target = document.getElementById('go2rtc-runtime-status');
    if (!target) return;
    try {
        const response = await fetch('/api/go2rtc/status');
        const data = await response.json();
        if (!response.ok) throw new Error(data.message || ('HTTP ' + response.status));
        target.className = 'cfg-note-banner ' + (data.api_usable ? 'cfg-note-banner-success' : 'cfg-note-banner-warning');
        target.textContent = t('config.go2rtc.status_summary')
            .replace('{container}', data.container_running ? t('config.go2rtc.running') : t('config.go2rtc.stopped'))
            .replace('{api}', data.api_usable ? t('config.go2rtc.available') : t('config.go2rtc.unavailable'));
    } catch (error) {
        target.className = 'cfg-note-banner cfg-note-banner-warning';
        target.textContent = t('config.go2rtc.status_error') + ': ' + error.message;
    }
}

async function go2rtcTest() {
    const result = document.getElementById('go2rtc-runtime-status');
    if (!go2rtcSavedOnly(result)) return;
    await go2rtcRequest('/api/go2rtc/test', result);
    go2rtcRefreshStatus();
}

async function go2rtcControl(action) {
    const result = document.getElementById('go2rtc-runtime-status');
    if (!go2rtcSavedOnly(result)) return;
    await go2rtcRequest('/api/go2rtc/' + action, result);
    go2rtcRefreshStatus();
}

async function go2rtcRequest(path, result) {
    if (result) result.textContent = t('config.go2rtc.working');
    try {
        const response = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
        const data = await response.json();
        if (!response.ok || data.status === 'error') throw new Error(data.message || ('HTTP ' + response.status));
        if (result) {
            result.textContent = t('config.go2rtc.action_ok');
            result.className = 'adg-test-result is-success';
        }
        return data;
    } catch (error) {
        if (result) {
            result.textContent = t('config.go2rtc.status_error') + ': ' + error.message;
            result.className = 'adg-test-result is-danger';
        }
        return null;
    }
}

async function go2rtcSnapshot(index) {
    const stream = go2rtcEnsureData().streams[index] || {};
    const result = document.getElementById('go2rtc-stream-result-' + index);
    if (!go2rtcSavedOnly(result)) return;
    try {
        const response = await fetch('/api/go2rtc/snapshot', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ stream_id: stream.id, cache_seconds: 2 })
        });
        const data = await response.json();
        if (!response.ok || data.status === 'error') throw new Error(data.message || ('HTTP ' + response.status));
        result.textContent = t('config.go2rtc.snapshot_ok');
        result.className = 'adg-test-result is-success';
        const preview = document.getElementById('go2rtc-stream-preview-' + index);
        const src = data.web_path || ('/api/go2rtc/proxy/api/frame.jpeg?src=' + encodeURIComponent(stream.id));
        preview.innerHTML = '<img class="go2rtc-stream-preview-image" src="' + escapeAttr(src) + '" alt="' + escapeAttr(stream.name || stream.id) + '">';
    } catch (error) {
        result.textContent = t('config.go2rtc.status_error') + ': ' + error.message;
        result.className = 'adg-test-result is-danger';
    }
}

function go2rtcOpenViewer(index) {
    const stream = go2rtcEnsureData().streams[index] || {};
    const result = document.getElementById('go2rtc-stream-result-' + index);
    if (!go2rtcSavedOnly(result)) return;
    window.open('/api/go2rtc/viewer/' + encodeURIComponent(stream.id), '_blank', 'noopener');
}
