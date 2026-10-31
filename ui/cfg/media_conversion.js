// cfg/media_conversion.js — Media Conversion config section module

let _mcSection = null;

function mcEnsureData() {
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.media_conversion) configData.tools.media_conversion = {};
    return configData.tools.media_conversion;
}

async function renderMediaConversionSection(section) {
    if (section) _mcSection = section; else section = _mcSection;
    const data = mcEnsureData();
    const enabled = data.enabled === true;
    const form = window.AuraConfigForm;
    const key = name => t('config.media_conversion.' + name);

    // The switch re-renders the section: the tool fields only exist while the feature is on.
    const enableToggle = form.toggle({ label: key('enabled_label'), help: t('help.media_conversion.enabled'), path: 'tools.media_conversion.enabled', value: enabled, onclick: "toggleBool(this);mcUpdateEnabled(this.classList.contains('on'))" });
    const groups = [{ titleKey: 'config.refresh.operation', fields: [enableToggle] }];
    let after = '';

    if (!enabled) {
        after = form.emptyState({ title: key('disabled_notice'), desc: key('disabled_desc') });
    } else {
        groups[0].fields.push(form.toggle({ label: key('readonly_label'), help: t('help.media_conversion.readonly'), path: 'tools.media_conversion.readonly', value: data.readonly === true }));
        groups.push({ titleKey: 'config.refresh.runtime', fields: [
            form.field({ label: key('ffmpeg_path_label'), help: t('help.media_conversion.ffmpeg_path'), path: 'tools.media_conversion.ffmpeg_path', value: data.ffmpeg_path || '', placeholder: 'ffmpeg' }),
            form.field({ label: key('imagemagick_path_label'), help: t('help.media_conversion.imagemagick_path'), path: 'tools.media_conversion.imagemagick_path', value: data.imagemagick_path || '', placeholder: 'magick' }),
            form.number({ label: key('timeout_label'), help: t('help.media_conversion.timeout_seconds'), path: 'tools.media_conversion.timeout_seconds', value: data.timeout_seconds || 120, min: 5, max: 1800, unit: key('seconds') }),
            form.number({ label: key('max_file_size_label'), help: t('help.media_conversion.max_file_size_mb'), path: 'tools.media_conversion.max_file_size_mb', value: data.max_file_size_mb || 0, min: 0, max: 102400, unit: key('megabytes') })
        ], content: form.actions([
            { html: '<button class="btn-save cfg-test-btn" onclick="mcTestTools()" id="mc-test-btn" data-config-action-draft="true">' + cfgIcon('plug') + '<span>' + escapeHtml(key('test_button')) + '</span></button>' },
            { html: cfgActionResult.html('mc-test-result') }
        ]) });
    }

    document.getElementById('content').innerHTML = form.renderSpec({
        label: section.label, desc: section.desc, groups, afterHTML: after,
        beforeHTML: '<div class="cfg-note-banner cfg-note-banner-info">' + escapeHtml(key('note')) + '</div>'
    });
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
