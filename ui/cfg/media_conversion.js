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
    html += '<div class="section-header">' + section.label + '</div>';
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
    html += '<div class="field-label">' + t('config.media_conversion.max_file_size_label') + '</div>';
    const helpMaxSize = t('help.media_conversion.max_file_size_mb');
    if (helpMaxSize) html += '<div class="field-help">' + helpMaxSize + '</div>';
    html += '<div class="dc-timeout-row">';
    html += '<input class="field-input dc-timeout-input" type="number" min="0" max="102400" data-path="tools.media_conversion.max_file_size_mb" value="' + (data.max_file_size_mb || 0) + '">';
    html += '<span class="dc-seconds-label">' + t('config.media_conversion.megabytes') + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<button class="btn-save dc-test-btn" onclick="mcTestTools()" id="mc-test-btn" data-config-action-draft="true">🔌 ' + t('config.media_conversion.test_button') + '</button>';
    html += cfgActionResult.html('mc-test-result', 'dc-test-result');
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

// Checks the tool paths typed in (unsaved included); the server accepts this body shape.
async function mcTestTools() {
    const toolLine = (label, info) => ' ' + label + ': ' + (info && info.available
        ? (info.version || info.path || t('config.media_conversion.available'))
        : ((info && info.message) || t('config.media_conversion.missing')));
    return cfgActionResult(document.getElementById('mc-test-btn'), () => {
        const patch = buildConfigPatchFromForm();
        const mediaConversion = patch.tools?.media_conversion || {};
        return fetch('/api/media-conversion/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ media_conversion: mediaConversion })
        });
    }, {
        target: 'mc-test-result',
        pending: t('config.media_conversion.loading'),
        success: outcome => outcome.reason,
        // Message plus the FFmpeg / ImageMagick lines, also on failure.
        interpret: outcome => {
            const body = outcome.body || {};
            const message = body.message || (outcome.state === 'error' ? 'HTTP ' + outcome.httpStatus : '');
            const detail = (message + toolLine('FFmpeg', body.ffmpeg) + toolLine('ImageMagick', body.imagemagick)).trim();
            if (!outcome.body) return null;
            return { reason: detail };
        }
    });
}
