// cfg/video_download.js — YouTube and video download config section module

let _vdSection = null;

function vdEnsureData() {
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.video_download) configData.tools.video_download = {};
    if (!configData.tools.send_youtube_video) configData.tools.send_youtube_video = {};
    const data = configData.tools.video_download;
    if (!data.mode) data.mode = 'docker';
    if (!data.download_dir) data.download_dir = 'data/downloads';
    if (typeof data.max_file_size_mb !== 'number') data.max_file_size_mb = 500;
    if (!data.timeout_seconds) data.timeout_seconds = 300;
    if (!data.default_format) data.default_format = 'best';
    if (!data.max_search_results) data.max_search_results = 10;
    if (!data.container_image) data.container_image = 'ghcr.io/jauderho/yt-dlp:latest';
    if (typeof data.auto_pull !== 'boolean') data.auto_pull = true;
    if (typeof data.enabled !== 'boolean') data.enabled = true;
    return data;
}

function vdSendData() {
    vdEnsureData();
    return configData.tools.send_youtube_video;
}

function vdField(labelKey, helpKey, inputHtml) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

function vdToggleRow(labelKey, helpKey, enabled, path) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    const helpText = t(helpKey);
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="toggleBool(this)"></div>';
    html += '</div>';
    return html;
}

async function renderVideoDownloadSection(section) {
    if (section) _vdSection = section; else section = _vdSection;
    const data = vdEnsureData();
    const send = vdSendData();
    const enabled = data.enabled !== false;
    const mode = data.mode || 'docker';

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">▶️ ' + t('config.video_download.note') + '</div>';

    html += vdToggleRow('config.video_download.send_enabled_label', 'help.video_download.send_enabled', send.enabled !== false, 'tools.send_youtube_video.enabled');

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.video_download.enabled_label') + '</div>';
    const helpEnabled = t('help.video_download.enabled');
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="tools.video_download.enabled" onclick="toggleBool(this);vdUpdateEnabled(this.classList.contains(\'on\'))"></div>';
    html += '</div>';

    if (!enabled) {
        html += '<div class="wh-notice"><span>▶️</span><div>';
        html += '<strong>' + t('config.video_download.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.video_download.disabled_desc') + '</small>';
        html += '</div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.video_download.mode_label') + '</div>';
    const helpMode = t('help.video_download.mode');
    if (helpMode) html += '<div class="field-help">' + helpMode + '</div>';
    html += '<select class="field-input" data-path="tools.video_download.mode" onchange="vdSwitchMode(this.value)">';
    html += '<option value="docker"' + (mode === 'docker' ? ' selected' : '') + '>' + t('config.video_download.mode_docker') + '</option>';
    html += '<option value="native"' + (mode === 'native' ? ' selected' : '') + '>' + t('config.video_download.mode_native') + '</option>';
    html += '</select></div>';

    html += vdToggleRow('config.video_download.allow_download_label', 'help.video_download.allow_download', data.allow_download === true, 'tools.video_download.allow_download');
    html += vdToggleRow('config.video_download.allow_transcribe_label', 'help.video_download.allow_transcribe', data.allow_transcribe === true, 'tools.video_download.allow_transcribe');

    html += vdField('config.video_download.download_dir_label', 'help.video_download.download_dir',
        '<input class="field-input" type="text" data-path="tools.video_download.download_dir" value="' + escapeAttr(data.download_dir || 'data/downloads') + '" placeholder="data/downloads">');

    html += vdField('config.video_download.default_format_label', 'help.video_download.default_format',
        '<input class="field-input" type="text" data-path="tools.video_download.default_format" value="' + escapeAttr(data.default_format || 'best') + '" placeholder="best">');

    html += '<div class="field-grid two-col">';
    html += vdField('config.video_download.max_file_size_label', 'help.video_download.max_file_size_mb',
        '<input class="field-input" type="number" min="0" max="10000" data-path="tools.video_download.max_file_size_mb" value="' + (typeof data.max_file_size_mb === 'number' ? data.max_file_size_mb : 500) + '">');
    html += vdField('config.video_download.timeout_label', 'help.video_download.timeout_seconds',
        '<input class="field-input" type="number" min="10" max="3600" data-path="tools.video_download.timeout_seconds" value="' + (data.timeout_seconds || 300) + '">');
    html += vdField('config.video_download.max_search_results_label', 'help.video_download.max_search_results',
        '<input class="field-input" type="number" min="1" max="25" data-path="tools.video_download.max_search_results" value="' + (data.max_search_results || 10) + '">');
    html += '</div>';

    if (mode === 'native') {
        html += vdField('config.video_download.yt_dlp_path_label', 'help.video_download.yt_dlp_path',
            '<input class="field-input" type="text" data-path="tools.video_download.yt_dlp_path" value="' + escapeAttr(data.yt_dlp_path || '') + '" placeholder="yt-dlp">');
    } else {
        html += vdField('config.video_download.container_image_label', 'help.video_download.container_image',
            '<input class="field-input" type="text" data-path="tools.video_download.container_image" value="' + escapeAttr(data.container_image || 'ghcr.io/jauderho/yt-dlp:latest') + '" placeholder="ghcr.io/jauderho/yt-dlp:latest">');
        html += vdToggleRow('config.video_download.auto_pull_label', 'help.video_download.auto_pull', data.auto_pull !== false, 'tools.video_download.auto_pull');
    }

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function vdSwitchMode(value) {
    const data = vdEnsureData();
    data.mode = value === 'native' ? 'native' : 'docker';
    if (typeof setNestedValue === 'function') {
        setNestedValue(configData, 'tools.video_download.mode', data.mode);
    }
    setDirty(true);
    renderVideoDownloadSection(null);
}

function vdUpdateEnabled(on) {
    const data = vdEnsureData();
    data.enabled = on;
    if (typeof setNestedValue === 'function') {
        setNestedValue(configData, 'tools.video_download.enabled', on);
    }
    setDirty(true);
    renderVideoDownloadSection(null);
}
