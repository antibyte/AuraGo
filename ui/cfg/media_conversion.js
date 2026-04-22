// cfg/media_conversion.js — Media Conversion config section module

let _mcSection = null;

function mcEnsureData() {
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.media_conversion) configData.tools.media_conversion = {};
    return configData.tools.media_conversion;
}

function mcField(labelKey, helpKey, inputHtml) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

function mcToggleRow(labelKey, helpKey, enabled, path) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    const helpText = t(helpKey);
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="toggleBool(this)"></div>';
    html += '</div>';
    return html;
}

async function renderMediaConversionSection(section) {
    if (section) _mcSection = section; else section = _mcSection;
    const data = mcEnsureData();
    const enabled = data.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">🎬 ' + t('config.media_conversion.note') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.media_conversion.enabled_label') + '</div>';
    const helpEnabled = t('help.media_conversion.enabled');
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="tools.media_conversion.enabled" onclick="toggleBool(this);mcUpdateEnabled(this.classList.contains(\'on\'))"></div>';
    html += '</div>';

    if (!enabled) {
        html += '<div class="wh-notice"><span>🎞️</span><div>';
        html += '<strong>' + t('config.media_conversion.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.media_conversion.disabled_desc') + '</small>';
        html += '</div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += mcToggleRow('config.media_conversion.readonly_label', 'help.media_conversion.readonly', data.readonly === true, 'tools.media_conversion.readonly');

    html += mcField('config.media_conversion.ffmpeg_path_label', 'help.media_conversion.ffmpeg_path',
        '<input class="field-input" type="text" data-path="tools.media_conversion.ffmpeg_path" value="' + escapeAttr(data.ffmpeg_path || '') + '" placeholder="ffmpeg">');

    html += mcField('config.media_conversion.imagemagick_path_label', 'help.media_conversion.imagemagick_path',
        '<input class="field-input" type="text" data-path="tools.media_conversion.imagemagick_path" value="' + escapeAttr(data.imagemagick_path || '') + '" placeholder="magick">');

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.media_conversion.timeout_label') + '</div>';
    const helpTimeout = t('help.media_conversion.timeout_seconds');
    if (helpTimeout) html += '<div class="field-help">' + helpTimeout + '</div>';
    html += '<div class="dc-timeout-row">';
    html += '<input class="field-input dc-timeout-input" type="number" min="5" max="1800" data-path="tools.media_conversion.timeout_seconds" value="' + (data.timeout_seconds || 120) + '">';
    html += '<span class="dc-seconds-label">' + t('config.media_conversion.seconds') + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<button class="btn-save dc-test-btn" onclick="mcTestTools()" id="mc-test-btn">🔌 ' + t('config.media_conversion.test_button') + '</button>';
    html += '<span id="mc-test-result" class="dc-test-result"></span>';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function mcUpdateEnabled(on) {
    const data = mcEnsureData();
    data.enabled = on;
    setDirty(true);
    renderMediaConversionSection(null);
}

async function mcTestTools() {
    const btn = document.getElementById('mc-test-btn');
    const result = document.getElementById('mc-test-result');
    btn.disabled = true;
    result.textContent = t('config.media_conversion.loading');
    result.className = 'dc-test-result';

    try {
        const patch = buildConfigPatchFromForm();
        const mediaConversion = patch.tools?.media_conversion || {};
        const resp = await fetch('/api/media-conversion/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ media_conversion: mediaConversion })
        });
        const body = await resp.json();
        const ffmpegLine = body.ffmpeg && body.ffmpeg.available
            ? ' FFmpeg: ' + (body.ffmpeg.version || body.ffmpeg.path || t('config.media_conversion.available'))
            : ' FFmpeg: ' + ((body.ffmpeg && body.ffmpeg.message) || t('config.media_conversion.missing'));
        const imageMagickLine = body.imagemagick && body.imagemagick.available
            ? ' ImageMagick: ' + (body.imagemagick.version || body.imagemagick.path || t('config.media_conversion.available'))
            : ' ImageMagick: ' + ((body.imagemagick && body.imagemagick.message) || t('config.media_conversion.missing'));
        if (resp.ok && body.status === 'success') {
            result.className = 'dc-test-result is-success';
            result.textContent = t('config.media_conversion.status_success') + ' ' + (body.message || '') + ffmpegLine + imageMagickLine;
        } else if (body.status === 'warning') {
            result.className = 'dc-test-result';
            result.textContent = t('config.media_conversion.status_warning') + ' ' + (body.message || '') + ffmpegLine + imageMagickLine;
        } else {
            result.className = 'dc-test-result is-danger';
            result.textContent = t('config.media_conversion.status_error') + ' ' + (body.message || ('HTTP ' + resp.status)) + ffmpegLine + imageMagickLine;
        }
    } catch (e) {
        result.className = 'dc-test-result is-danger';
        result.textContent = t('config.media_conversion.status_error') + ' ' + e.message;
    } finally {
        btn.disabled = false;
    }
}
