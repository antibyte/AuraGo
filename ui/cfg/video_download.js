// cfg/video_download.js — YouTube and video download config section module

let _vdSection = null;
const VD_PINNED_IMAGE = 'ghcr.io/jauderho/yt-dlp@sha256:90cd9016efc4451153c8bc35cc7a0d9a8cba0e796818e52f3b4d28b9736ac91c';
const VD_DEFAULT_FORMAT = 'bestvideo[height<=720]+bestaudio/best[height<=720]/best';

function vdEnsureData() {
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.video_download) configData.tools.video_download = {};
    if (!configData.tools.send_youtube_video) configData.tools.send_youtube_video = {};
    const data = configData.tools.video_download;
    if (!data.mode) data.mode = 'docker';
    if (!data.download_dir) data.download_dir = 'data/downloads';
    if (typeof data.max_file_size_mb !== 'number') data.max_file_size_mb = 500;
    if (!data.timeout_seconds) data.timeout_seconds = 300;
    if (!data.default_format) data.default_format = VD_DEFAULT_FORMAT;
    if (!data.max_search_results) data.max_search_results = 10;
    if (!data.container_image) data.container_image = VD_PINNED_IMAGE;
    if (typeof data.auto_pull !== 'boolean') data.auto_pull = true;
    if (typeof data.enabled !== 'boolean') data.enabled = true;
    return data;
}

function vdSendData() {
    vdEnsureData();
    return configData.tools.send_youtube_video;
}

async function renderVideoDownloadSection(section) {
    if (section) _vdSection = section; else section = _vdSection;
    const data = vdEnsureData();
    const send = vdSendData();
    const enabled = data.enabled !== false;
    const mode = data.mode || 'docker';
    const form = window.AuraConfigForm;
    const key = name => t('config.video_download.' + name);
    const toggle = (name, helpKey, on, path) => form.toggle({ label: key(name + '_label'), help: t('help.video_download.' + helpKey), path, value: on });

    // The switch re-renders the section: the download settings only exist while the feature is on.
    const enableToggle = form.toggle({ label: key('enabled_label'), help: t('help.video_download.enabled'), path: 'tools.video_download.enabled', value: enabled })
        .replace('onclick="toggleBool(this)"', `onclick="toggleBool(this);vdUpdateEnabled(this.classList.contains('on'))"`);
    const operation = [enableToggle, toggle('send_enabled', 'send_enabled', send.enabled !== false, 'tools.send_youtube_video.enabled')];
    const groups = [{ titleKey: 'config.refresh.operation', fields: operation }];
    let after = '';

    if (!enabled) {
        after = form.emptyState({ title: key('disabled_notice'), desc: key('disabled_desc') });
    } else {
        operation.push(
            toggle('allow_download', 'allow_download', data.allow_download === true, 'tools.video_download.allow_download'),
            toggle('allow_transcribe', 'allow_transcribe', data.allow_transcribe === true, 'tools.video_download.allow_transcribe'),
            toggle('readonly', 'readonly', data.readonly === true, 'tools.video_download.readonly'));
        const settings = [
            form.select({
                label: key('mode_label'), help: t('help.video_download.mode'), path: 'tools.video_download.mode', value: mode,
                onchange: 'vdSwitchMode(this.value)',
                options: [{ value: 'docker', label: key('mode_docker') }, { value: 'native', label: key('mode_native') }]
            }),
            form.field({ label: key('download_dir_label'), help: t('help.video_download.download_dir'), path: 'tools.video_download.download_dir', value: data.download_dir || 'data/downloads', placeholder: 'data/downloads' }),
            form.field({ label: key('default_format_label'), help: t('help.video_download.default_format'), path: 'tools.video_download.default_format', value: data.default_format || VD_DEFAULT_FORMAT, placeholder: VD_DEFAULT_FORMAT }),
            form.number({ label: key('max_file_size_label'), help: t('help.video_download.max_file_size_mb'), path: 'tools.video_download.max_file_size_mb', value: typeof data.max_file_size_mb === 'number' ? data.max_file_size_mb : 500, min: 0, max: 10000 }),
            form.number({ label: key('timeout_label'), help: t('help.video_download.timeout_seconds'), path: 'tools.video_download.timeout_seconds', value: data.timeout_seconds || 300, min: 10, max: 3600 }),
            form.number({ label: key('max_search_results_label'), help: t('help.video_download.max_search_results'), path: 'tools.video_download.max_search_results', value: data.max_search_results || 10, min: 1, max: 25 })
        ];
        if (mode === 'native') {
            settings.push(form.field({ label: key('yt_dlp_path_label'), help: t('help.video_download.yt_dlp_path'), path: 'tools.video_download.yt_dlp_path', value: data.yt_dlp_path || '', placeholder: 'yt-dlp' }));
        } else {
            settings.push(
                form.field({ label: key('container_image_label'), help: t('help.video_download.container_image'), path: 'tools.video_download.container_image', value: data.container_image || VD_PINNED_IMAGE, placeholder: VD_PINNED_IMAGE }),
                toggle('auto_pull', 'auto_pull', data.auto_pull !== false, 'tools.video_download.auto_pull'));
        }
        groups.push({ titleKey: 'config.refresh.settings', fields: settings });
    }

    document.getElementById('content').innerHTML = form.renderSpec({
        label: section.label, desc: section.desc, groups, afterHTML: after,
        beforeHTML: '<div class="cfg-note-banner cfg-note-banner-info">' + escapeHtml(key('note')) + '</div>'
    });
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
