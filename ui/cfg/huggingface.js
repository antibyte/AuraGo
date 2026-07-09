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
    if (!Array.isArray(cfg.allowed_namespaces)) cfg.allowed_namespaces = [];
    if (!Array.isArray(cfg.allowed_repos)) cfg.allowed_repos = [];
    if (!Array.isArray(cfg.allowed_hardware)) cfg.allowed_hardware = ['cpu-basic'];
    if (!cfg.max_download_mb) cfg.max_download_mb = 512;
    if (!cfg.max_upload_mb) cfg.max_upload_mb = 512;
    if (!cfg.max_dataset_rows) cfg.max_dataset_rows = 100;
    if (!cfg.job_default_timeout_minutes) cfg.job_default_timeout_minutes = 30;
    if (!cfg.job_max_runtime_minutes) cfg.job_max_runtime_minutes = 120;
    if (!cfg.request_timeout_seconds) cfg.request_timeout_seconds = 60;
    if (!cfg.max_result_bytes) cfg.max_result_bytes = 524288;
    if (!cfg.hub_base_url) cfg.hub_base_url = 'https://huggingface.co';
    if (!cfg.dataset_base_url) cfg.dataset_base_url = 'https://datasets-server.huggingface.co';
    if (!cfg.jobs_base_url) cfg.jobs_base_url = 'https://huggingface.co/api/jobs';
    if (!cfg.router_base_url) cfg.router_base_url = 'https://router.huggingface.co/v1';
    return cfg;
}

async function renderHuggingFaceSection(section) {
    const cfg = huggingFaceConfig();
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div id="huggingface-status-banner" class="adg-status-banner">' + t('config.huggingface.status_loading') + '</div>';
    html += '<div class="cfg-actions-row">';
    html += '<button class="btn-save adg-test-btn" id="huggingface-test-btn" onclick="huggingFaceTestConnection()">' + t('config.huggingface.test_connection') + '</button>';
    html += '<span id="huggingface-test-result" class="adg-test-result"></span></div>';

    html += '<div class="field-group">' + huggingFaceToggle('huggingface.enabled', cfg.enabled, 'config.huggingface.enabled', 'help.huggingface.enabled') + '</div>';
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.huggingface.token') + '</div>';
    html += '<div class="field-help">' + t('help.huggingface.token') + '</div>';
    html += '<div class="adg-password-row"><div class="password-wrap cfg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="huggingface-token" value="" placeholder="' + escapeAttr(cfg.configured ? t('config.huggingface.token_existing') : t('config.huggingface.token_placeholder')) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>';
    html += '<button type="button" class="btn-save adg-save-btn" onclick="huggingFaceSaveToken()">' + t('config.huggingface.save_token') + '</button></div>';
    html += '<div id="huggingface-token-status" class="adg-test-result"></div></div>';

    html += '<div class="field-grid two-cols">';
    html += huggingFaceNumber('max_download_mb', 'config.huggingface.max_download_mb', 'help.huggingface.max_download_mb', cfg.max_download_mb);
    html += huggingFaceNumber('max_upload_mb', 'config.huggingface.max_upload_mb', 'help.huggingface.max_upload_mb', cfg.max_upload_mb);
    html += huggingFaceNumber('max_dataset_rows', 'config.huggingface.max_dataset_rows', 'help.huggingface.max_dataset_rows', cfg.max_dataset_rows);
    html += huggingFaceNumber('job_default_timeout_minutes', 'config.huggingface.job_default_timeout', 'help.huggingface.job_default_timeout', cfg.job_default_timeout_minutes);
    html += huggingFaceNumber('job_max_runtime_minutes', 'config.huggingface.job_max_runtime', 'help.huggingface.job_max_runtime', cfg.job_max_runtime_minutes);
    html += huggingFaceNumber('request_timeout_seconds', 'config.huggingface.request_timeout', 'help.huggingface.request_timeout', cfg.request_timeout_seconds);
    html += huggingFaceNumber('max_result_bytes', 'config.huggingface.max_result_bytes', 'help.huggingface.max_result_bytes', cfg.max_result_bytes);
    html += '</div>';

    html += '<div class="field-group-title cfg-group-title-top">' + t('config.huggingface.policy_title') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += huggingFaceToggle('huggingface.read_only', cfg.read_only, 'config.huggingface.read_only', 'help.huggingface.read_only');
    html += huggingFaceToggle('huggingface.allow_writes', cfg.allow_writes, 'config.huggingface.allow_writes', 'help.huggingface.allow_writes');
    html += huggingFaceToggle('huggingface.allow_delete', cfg.allow_delete, 'config.huggingface.allow_delete', 'help.huggingface.allow_delete');
    html += huggingFaceToggle('huggingface.allow_jobs', cfg.allow_jobs, 'config.huggingface.allow_jobs', 'help.huggingface.allow_jobs');
    html += huggingFaceToggle('huggingface.allow_scheduled_jobs', cfg.allow_scheduled_jobs, 'config.huggingface.allow_scheduled_jobs', 'help.huggingface.allow_scheduled_jobs');
    html += '</div>';

    html += '<div class="field-group-title cfg-group-title-top">' + t('config.huggingface.allowlist_title') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += huggingFaceArray('allowed_namespaces', 'config.huggingface.allowed_namespaces', 'help.huggingface.allowed_namespaces', cfg.allowed_namespaces);
    html += huggingFaceArray('allowed_repos', 'config.huggingface.allowed_repos', 'help.huggingface.allowed_repos', cfg.allowed_repos);
    html += huggingFaceArray('allowed_hardware', 'config.huggingface.allowed_hardware', 'help.huggingface.allowed_hardware', cfg.allowed_hardware);
    html += '</div>';

    html += '<div class="field-group-title cfg-group-title-top">' + t('config.huggingface.endpoints_title') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += huggingFaceText('hub_base_url', 'config.huggingface.hub_base_url', 'help.huggingface.hub_base_url', cfg.hub_base_url);
    html += huggingFaceText('dataset_base_url', 'config.huggingface.dataset_base_url', 'help.huggingface.dataset_base_url', cfg.dataset_base_url);
    html += huggingFaceText('jobs_base_url', 'config.huggingface.jobs_base_url', 'help.huggingface.jobs_base_url', cfg.jobs_base_url);
    html += huggingFaceText('router_base_url', 'config.huggingface.router_base_url', 'help.huggingface.router_base_url', cfg.router_base_url);
    html += '</div></div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    await huggingFaceRefreshStatus();
}

function huggingFaceToggle(path, enabled, labelKey, helpKey) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div>' +
        '<div class="toggle-wrap"><div class="toggle' + (enabled ? ' on' : '') + '" data-path="' + path + '" onclick="toggleBool(this)"></div><span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span></div></div>';
}

function huggingFaceNumber(key, labelKey, helpKey, value) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div><input class="field-input" type="number" min="0" data-path="huggingface.' + key + '" value="' + escapeAttr(value) + '"></div>';
}

function huggingFaceText(key, labelKey, helpKey, value) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div><input class="field-input" type="text" data-path="huggingface.' + key + '" value="' + escapeAttr(value) + '"></div>';
}

function huggingFaceArray(key, labelKey, helpKey, values) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div><input class="field-input" type="text" data-type="array" data-path="huggingface.' + key + '" value="' + escapeAttr((values || []).join(', ')) + '"></div>';
}

async function huggingFaceRefreshStatus() {
    const banner = document.getElementById('huggingface-status-banner');
    try {
        const resp = await fetch('/api/huggingface/status');
        const data = resp.ok ? await resp.json() : null;
        if (!data) throw new Error(t('config.huggingface.status_error'));
        const cfg = huggingFaceConfig();
        cfg.configured = !!data.configured;
        const key = data.status === 'ready' ? 'status_ready' : (data.status === 'disabled' ? 'status_disabled' : 'status_public_read_only');
        if (banner) {
            banner.className = 'adg-status-banner' + (data.status === 'ready' ? ' is-success' : (data.status === 'disabled' ? '' : ' is-warning'));
            banner.textContent = t('config.huggingface.' + key);
        }
    } catch (_) {
        if (banner) { banner.className = 'adg-status-banner is-danger'; banner.textContent = t('config.huggingface.status_error'); }
    }
}

async function huggingFaceSaveToken() {
    const input = document.getElementById('huggingface-token');
    const status = document.getElementById('huggingface-token-status');
    const value = input ? input.value.trim() : '';
    if (!value) { if (status) { status.className = 'adg-test-result is-danger'; status.textContent = t('config.huggingface.token_empty'); } return; }
    try {
        const resp = await fetch('/api/vault/secrets', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ key: 'huggingface_token', value }) });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || data.message || t('config.huggingface.token_save_failed'));
        cfgMarkSecretStored(input);
        huggingFaceConfig().configured = true;
        if (status) { status.className = 'adg-test-result is-success'; status.textContent = t('config.huggingface.token_saved'); }
        await huggingFaceRefreshStatus();
    } catch (error) {
        if (status) { status.className = 'adg-test-result is-danger'; status.textContent = error.message || t('config.huggingface.token_save_failed'); }
    }
}

async function huggingFaceTestConnection() {
    const btn = document.getElementById('huggingface-test-btn');
    const result = document.getElementById('huggingface-test-result');
    if (btn) btn.disabled = true;
    try {
        const resp = await fetch('/api/huggingface/test', { method: 'POST' });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok || data.status === 'error') throw new Error(data.message || t('config.huggingface.test_failed'));
        showToast(data.message || t('config.huggingface.test_ok'), 'success');
        if (result) { result.className = 'adg-test-result is-success'; result.textContent = t('config.huggingface.test_ok'); }
    } catch (error) {
        showToast(error.message || t('config.huggingface.test_failed'), 'error');
        if (result) { result.className = 'adg-test-result is-danger'; result.textContent = t('config.huggingface.test_failed'); }
    } finally {
        if (btn) btn.disabled = false;
    }
}
