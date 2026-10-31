// cfg/huggingface.js - Hugging Face platform integration

function huggingFaceConfig() {
    configData.huggingface = configData.huggingface || {};
    const cfg = configData.huggingface;
    if (cfg.enabled === undefined) cfg.enabled = false;
    if (cfg.read_only === undefined) cfg.read_only = true;
    if (cfg.allow_writes === undefined) cfg.allow_writes = false;
    if (cfg.allow_delete === undefined) cfg.allow_delete = false;
    if (cfg.allow_jobs === undefined) cfg.allow_jobs = false;
    if (cfg.allow_scheduled_jobs === undefined) cfg.allow_scheduled_jobs = false;
    if (cfg.allow_job_token_injection === undefined) cfg.allow_job_token_injection = false;
    if (!Array.isArray(cfg.allowed_namespaces)) cfg.allowed_namespaces = [];
    if (!Array.isArray(cfg.allowed_repos)) cfg.allowed_repos = [];
    if (!Array.isArray(cfg.allowed_hardware)) cfg.allowed_hardware = ['cpu-basic'];
    if (!cfg.max_download_mb) cfg.max_download_mb = 512;
    if (!cfg.max_upload_mb || cfg.max_upload_mb > 10) cfg.max_upload_mb = 10;
    if (!cfg.max_dataset_rows) cfg.max_dataset_rows = 100;
    if (!cfg.job_default_timeout_minutes) cfg.job_default_timeout_minutes = 30;
    if (!cfg.job_max_runtime_minutes) cfg.job_max_runtime_minutes = 120;
    if (!cfg.request_timeout_seconds) cfg.request_timeout_seconds = 60;
    if (!cfg.max_result_bytes) cfg.max_result_bytes = 524288;
    if (!cfg.hub_base_url) cfg.hub_base_url = 'https://huggingface.co';
    if (!cfg.dataset_base_url) cfg.dataset_base_url = 'https://datasets-server.huggingface.co';
    if (!cfg.jobs_base_url) cfg.jobs_base_url = 'https://huggingface.co/api/jobs';
    if (cfg.job_namespace === undefined) cfg.job_namespace = '';
    return cfg;
}

async function renderHuggingFaceSection(section) {
    const cfg = huggingFaceConfig();
    const form = window.AuraConfigForm;
    document.getElementById('content').innerHTML = form.renderSpec({
        key: 'huggingface',
        label: section.label,
        desc: section.desc,
        banner: { id: 'huggingface-status-banner', state: 'checking', detail: t('config.huggingface.status_loading') },
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            huggingFaceToggle('huggingface.enabled', cfg.enabled, 'config.huggingface.enabled', 'help.huggingface.enabled')
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.password({
                label: t('config.huggingface.token'),
                help: t('help.huggingface.token'),
                id: 'huggingface-token',
                path: 'huggingface.token',
                value: '',
                placeholder: cfg.configured ? t('config.huggingface.token_existing') : t('config.huggingface.token_placeholder')
            })
        ] }, { titleKey: 'config.refresh.runtime', fields: [
            huggingFaceNumber('max_download_mb', 'config.huggingface.max_download_mb', 'help.huggingface.max_download_mb', cfg.max_download_mb, 2048),
            huggingFaceNumber('max_upload_mb', 'config.huggingface.max_upload_mb', 'help.huggingface.max_upload_mb', cfg.max_upload_mb, 10),
            huggingFaceNumber('max_dataset_rows', 'config.huggingface.max_dataset_rows', 'help.huggingface.max_dataset_rows', cfg.max_dataset_rows),
            huggingFaceNumber('job_default_timeout_minutes', 'config.huggingface.job_default_timeout', 'help.huggingface.job_default_timeout', cfg.job_default_timeout_minutes),
            huggingFaceNumber('job_max_runtime_minutes', 'config.huggingface.job_max_runtime', 'help.huggingface.job_max_runtime', cfg.job_max_runtime_minutes),
            huggingFaceNumber('request_timeout_seconds', 'config.huggingface.request_timeout', 'help.huggingface.request_timeout', cfg.request_timeout_seconds),
            huggingFaceNumber('max_result_bytes', 'config.huggingface.max_result_bytes', 'help.huggingface.max_result_bytes', cfg.max_result_bytes)
        ] }, { title: t('config.huggingface.policy_title'), fields: [
            huggingFaceToggle('huggingface.read_only', cfg.read_only, 'config.huggingface.read_only', 'help.huggingface.read_only'),
            huggingFaceToggle('huggingface.allow_writes', cfg.allow_writes, 'config.huggingface.allow_writes', 'help.huggingface.allow_writes'),
            huggingFaceToggle('huggingface.allow_delete', cfg.allow_delete, 'config.huggingface.allow_delete', 'help.huggingface.allow_delete'),
            huggingFaceToggle('huggingface.allow_jobs', cfg.allow_jobs, 'config.huggingface.allow_jobs', 'help.huggingface.allow_jobs'),
            huggingFaceToggle('huggingface.allow_scheduled_jobs', cfg.allow_scheduled_jobs, 'config.huggingface.allow_scheduled_jobs', 'help.huggingface.allow_scheduled_jobs'),
            huggingFaceToggle('huggingface.allow_job_token_injection', cfg.allow_job_token_injection, 'config.huggingface.allow_job_token_injection', 'help.huggingface.allow_job_token_injection')
        ] }, { title: t('config.huggingface.allowlist_title'), fields: [
            huggingFaceArray('allowed_namespaces', 'config.huggingface.allowed_namespaces', 'help.huggingface.allowed_namespaces', cfg.allowed_namespaces),
            huggingFaceArray('allowed_repos', 'config.huggingface.allowed_repos', 'help.huggingface.allowed_repos', cfg.allowed_repos),
            huggingFaceArray('allowed_hardware', 'config.huggingface.allowed_hardware', 'help.huggingface.allowed_hardware', cfg.allowed_hardware)
        ] }, { title: t('config.huggingface.endpoints_title'), fields: [
            huggingFaceText('hub_base_url', 'config.huggingface.hub_base_url', 'help.huggingface.hub_base_url', cfg.hub_base_url),
            huggingFaceText('dataset_base_url', 'config.huggingface.dataset_base_url', 'help.huggingface.dataset_base_url', cfg.dataset_base_url),
            huggingFaceText('jobs_base_url', 'config.huggingface.jobs_base_url', 'help.huggingface.jobs_base_url', cfg.jobs_base_url),
            huggingFaceText('job_namespace', 'config.huggingface.job_namespace', 'help.huggingface.job_namespace', cfg.job_namespace)
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save cfg-test-btn" id="huggingface-test-btn" data-config-action-draft="true" onclick="huggingFaceTestConnection()">' + escapeHtml(t('config.huggingface.test_connection')) + '</button>' },
            { html: form.actionResult({ id: 'huggingface-test-result' }) }
        ])
    });
    attachChangeListeners();
    await huggingFaceRefreshStatus();
}

function huggingFaceToggle(path, enabled, labelKey, helpKey) {
    return window.AuraConfigForm.toggle({ label: t(labelKey), help: t(helpKey), path: path, value: enabled === true });
}

function huggingFaceNumber(key, labelKey, helpKey, value, maxValue) {
    return window.AuraConfigForm.number({ label: t(labelKey), help: t(helpKey), path: 'huggingface.' + key, value: value, min: 0, max: Number.isFinite(maxValue) ? maxValue : undefined });
}

function huggingFaceText(key, labelKey, helpKey, value) {
    return window.AuraConfigForm.field({ label: t(labelKey), help: t(helpKey), path: 'huggingface.' + key, value: value });
}

// Comma separated list; data-type="array" makes the save path split it.
function huggingFaceArray(key, labelKey, helpKey, values) {
    return window.AuraConfigForm.field({ label: t(labelKey), help: t(helpKey), path: 'huggingface.' + key, value: (values || []).join(', '), dataType: 'array' });
}

async function huggingFaceRefreshStatus() {
    if (!document.getElementById('huggingface-status-banner')) return;
    // Times out to "unknown" with a retry instead of hanging on "checking".
    await cfgStatusBanner.check('huggingface-status-banner', signal => fetch('/api/huggingface/status', { signal }), data => {
        if (!data || typeof data !== 'object') return null;
        if (!['ready', 'disabled', 'public_read_only'].includes(data.status)) {
            return { state: 'error', detail: t('config.huggingface.status_error') + (data.message || data.error || data.status ? ': ' + (data.message || data.error || data.status) : '') };
        }
        huggingFaceConfig().configured = !!data.configured;
        const key = data.status === 'ready' ? 'status_ready' : (data.status === 'disabled' ? 'status_disabled' : 'status_public_read_only');
        const state = data.status === 'ready' ? 'connected' : (data.status === 'disabled' ? 'off' : 'degraded');
        return { state: state, detail: t('config.huggingface.' + key) };
    }, { checkingDetail: t('config.huggingface.status_loading'), retry: huggingFaceRefreshStatus });
}

// Tests with the values typed in; the server falls back to the stored token while the endpoints are unchanged.
async function huggingFaceTestConnection() {
    const btn = document.getElementById('huggingface-test-btn');
    return cfgActionResult(btn, () => fetch('/api/huggingface/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['huggingface']) })
    }), {
        target: 'huggingface-test-result',
        pending: t('config.huggingface.status_loading'),
        success: t('config.huggingface.test_ok'),
        failure: t('config.huggingface.test_failed')
    });
}
