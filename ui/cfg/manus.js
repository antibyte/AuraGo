// cfg/manus.js - Native Manus v2 task integration

const manusCatalogState = {
    projects: [],
    connectors: [],
    skills: [],
    loading: false
};

function manusConfig() {
    configData.manus = configData.manus || {};
    const cfg = configData.manus;
    if (cfg.enabled === undefined) cfg.enabled = false;
    if (cfg.read_only === undefined) cfg.read_only = true;
    ['allow_create_tasks', 'allow_send_messages', 'allow_stop_tasks', 'allow_file_uploads', 'allow_file_downloads'].forEach(key => {
        if (cfg[key] === undefined) cfg[key] = false;
    });
    ['allowed_project_ids', 'allowed_connector_ids', 'allowed_skill_ids'].forEach(key => {
        if (!Array.isArray(cfg[key])) cfg[key] = [];
    });
    if (!cfg.default_agent_profile) cfg.default_agent_profile = 'manus-1.6';
    if (cfg.default_locale === undefined) cfg.default_locale = '';
    if (!cfg.request_timeout_seconds) cfg.request_timeout_seconds = 60;
    if (!cfg.poll_interval_seconds) cfg.poll_interval_seconds = 5;
    if (!cfg.max_wait_seconds) cfg.max_wait_seconds = 60;
    if (!cfg.max_result_bytes) cfg.max_result_bytes = 262144;
    if (!cfg.max_file_size_mb) cfg.max_file_size_mb = 20;
    return cfg;
}

async function renderManusSection(section) {
    const cfg = manusConfig();
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div id="manus-status-banner" class="adg-status-banner">' + t('config.manus.status_loading') + '</div>';
    html += '<div class="cfg-actions-row"><button class="btn-save adg-test-btn" id="manus-test-btn" onclick="manusTestConnection()">' + t('config.manus.test_connection') + '</button>';
    html += '<button class="btn-save btn-secondary" onclick="manusLoadCatalogs(true)">' + t('config.manus.load_catalogs') + '</button>';
    html += '<span id="manus-test-result" class="adg-test-result"></span></div>';

    html += manusToggle('manus.enabled', cfg.enabled, 'config.manus.enabled', 'help.manus.enabled', false);
    html += '<div class="field-group"><div class="field-label">' + t('config.manus.api_key') + '</div><div class="field-help">' + t('help.manus.api_key') + '</div>';
    html += '<div class="adg-password-row"><div class="password-wrap cfg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="manus-api-key" data-path="manus.api_key" value="" placeholder="' + escapeAttr(cfg.configured ? t('config.manus.api_key_existing') : t('config.manus.api_key_placeholder')) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>';
    html += '<button type="button" class="btn-save adg-save-btn" onclick="manusSaveAPIKey()">' + t('config.manus.save_api_key') + '</button></div>';
    html += '<div id="manus-api-key-status" class="adg-test-result"></div></div>';

    html += '<div class="field-grid two-cols">';
    html += manusProfileSelect(cfg.default_agent_profile);
    html += manusText('default_locale', 'config.manus.default_locale', 'help.manus.default_locale', cfg.default_locale);
    html += manusNumber('request_timeout_seconds', 'config.manus.request_timeout_seconds', 'help.manus.request_timeout_seconds', cfg.request_timeout_seconds, 1, 300);
    html += manusNumber('poll_interval_seconds', 'config.manus.poll_interval_seconds', 'help.manus.poll_interval_seconds', cfg.poll_interval_seconds, 1, 60);
    html += manusNumber('max_wait_seconds', 'config.manus.max_wait_seconds', 'help.manus.max_wait_seconds', cfg.max_wait_seconds, 1, 60);
    html += manusNumber('max_result_bytes', 'config.manus.max_result_bytes', 'help.manus.max_result_bytes', cfg.max_result_bytes, 1024, 10485760);
    html += manusNumber('max_file_size_mb', 'config.manus.max_file_size_mb', 'help.manus.max_file_size_mb', cfg.max_file_size_mb, 1, 100);
    html += '</div>';

    html += '<div class="field-group-title cfg-group-title-top">' + t('config.manus.permissions_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.manus.permissions_desc') + '</div><div class="field-grid two-cols">';
    html += manusToggle('manus.read_only', cfg.read_only, 'config.manus.read_only', 'help.manus.read_only', false);
    html += manusToggle('manus.allow_create_tasks', cfg.allow_create_tasks, 'config.manus.allow_create_tasks', 'help.manus.allow_create_tasks', true);
    html += manusToggle('manus.allow_send_messages', cfg.allow_send_messages, 'config.manus.allow_send_messages', 'help.manus.allow_send_messages', true);
    html += manusToggle('manus.allow_stop_tasks', cfg.allow_stop_tasks, 'config.manus.allow_stop_tasks', 'help.manus.allow_stop_tasks', true);
    html += manusToggle('manus.allow_file_uploads', cfg.allow_file_uploads, 'config.manus.allow_file_uploads', 'help.manus.allow_file_uploads', true);
    html += manusToggle('manus.allow_file_downloads', cfg.allow_file_downloads, 'config.manus.allow_file_downloads', 'help.manus.allow_file_downloads', true);
    html += '</div>';

    html += '<div class="field-group-title cfg-group-title-top">' + t('config.manus.allowlists_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.manus.allowlists_desc') + '</div>';
    html += manusCatalogGroup('projects', 'allowed_project_ids', 'config.manus.allowed_projects', 'help.manus.allowed_projects', cfg.allowed_project_ids);
    html += manusCatalogGroup('connectors', 'allowed_connector_ids', 'config.manus.allowed_connectors', 'help.manus.allowed_connectors', cfg.allowed_connector_ids);
    html += manusCatalogGroup('skills', 'allowed_skill_ids', 'config.manus.allowed_skills', 'help.manus.allowed_skills', cfg.allowed_skill_ids);
    html += '<div class="adg-status-banner is-warning manus-default-skills-note">' + t('config.manus.default_skills_note') + '</div></div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    await manusRefreshStatus();
}

function manusToggle(path, enabled, labelKey, helpKey, dangerous) {
    const handler = dangerous ? 'manusTogglePolicy(this)' : 'toggleBool(this)';
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div>' +
        '<div class="toggle-wrap"><div class="toggle' + (enabled ? ' on' : '') + '" data-path="' + path + '" onclick="' + handler + '"></div><span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span></div></div>';
}

async function manusTogglePolicy(el) {
    if (!el || el.classList.contains('on')) { if (el) toggleBool(el); return; }
    const allowed = await showConfirm(t('config.manus.danger_confirm_title'), t('config.manus.danger_confirm'));
    if (allowed) toggleBool(el);
}

function manusProfileSelect(value) {
    const profiles = ['manus-1.6', 'manus-1.6-lite', 'manus-1.6-max'];
    return '<div class="field-group"><div class="field-label">' + t('config.manus.default_agent_profile') + '</div><div class="field-help">' + t('help.manus.default_agent_profile') + '</div>' +
        '<select class="field-select" data-path="manus.default_agent_profile">' + profiles.map(profile => '<option value="' + profile + '"' + (profile === value ? ' selected' : '') + '>' + profile + '</option>').join('') + '</select></div>';
}

function manusText(key, labelKey, helpKey, value) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div><input class="field-input" type="text" data-path="manus.' + key + '" value="' + escapeAttr(value || '') + '"></div>';
}

function manusNumber(key, labelKey, helpKey, value, min, max) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div><input class="field-input" type="number" min="' + min + '" max="' + max + '" data-path="manus.' + key + '" value="' + escapeAttr(value) + '"></div>';
}

function manusCatalogGroup(kind, configKey, labelKey, helpKey, selected) {
    return '<div class="field-group manus-catalog-group"><div class="field-label">' + t(labelKey) + '</div><div class="field-help">' + t(helpKey) + '</div>' +
        '<input class="field-input is-hidden" type="text" data-type="array" data-path="manus.' + configKey + '" value="' + escapeAttr((selected || []).join(', ')) + '">' +
        '<div id="manus-' + kind + '-catalog" class="manus-catalog-list"><span class="manus-catalog-empty">' + t('config.manus.catalog_not_loaded') + '</span></div></div>';
}

async function manusRefreshStatus() {
    const banner = document.getElementById('manus-status-banner');
    try {
        const resp = await fetch('/api/manus/status');
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || data.message || t('config.manus.status_error'));
        const cfg = manusConfig();
        cfg.configured = !!data.configured;
        const state = data.status || 'error';
        if (banner) {
            banner.className = 'adg-status-banner' + (state === 'ready' ? ' is-success' : (state === 'disabled' ? '' : ' is-warning'));
            banner.textContent = t('config.manus.status_' + state);
        }
    } catch (_) {
        if (banner) { banner.className = 'adg-status-banner is-danger'; banner.textContent = t('config.manus.status_error'); }
    }
}

async function manusSaveAPIKey() {
    const input = document.getElementById('manus-api-key');
    const status = document.getElementById('manus-api-key-status');
    const value = input ? input.value.trim() : '';
    if (!value) { if (status) { status.className = 'adg-test-result is-danger'; status.textContent = t('config.manus.api_key_empty'); } return; }
    try {
        const resp = await fetch('/api/vault/secrets', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ key: 'manus_api_key', value }) });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || data.message || t('config.manus.api_key_save_failed'));
        cfgMarkSecretStored(input, 'manus.api_key');
        manusConfig().configured = true;
        if (status) { status.className = 'adg-test-result is-success'; status.textContent = t('config.manus.api_key_saved'); }
        await manusRefreshStatus();
    } catch (error) {
        if (status) { status.className = 'adg-test-result is-danger'; status.textContent = error.message || t('config.manus.api_key_save_failed'); }
    }
}

async function manusTestConnection() {
    const btn = document.getElementById('manus-test-btn');
    const result = document.getElementById('manus-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.className = 'adg-test-result'; result.textContent = t('config.manus.testing'); }
    try {
        const resp = await fetch('/api/manus/test', { method: 'POST' });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || data.message || t('config.manus.test_failed'));
        const credits = data.credits && (data.credits.total_credits ?? data.credits.available_credits ?? data.credits.credits);
        const suffix = credits === undefined ? '' : ' · ' + t('config.manus.credits', { count: credits });
        if (result) { result.className = 'adg-test-result is-success'; result.textContent = t('config.manus.test_ok') + suffix; }
        await manusLoadCatalogs(false);
    } catch (error) {
        if (result) { result.className = 'adg-test-result is-danger'; result.textContent = t('config.manus.test_failed') + ': ' + (error.message || t('config.common.error')); }
    } finally {
        if (btn) btn.disabled = false;
    }
}

async function manusLoadCatalogs(showErrors) {
    if (manusCatalogState.loading) return;
    manusCatalogState.loading = true;
    try {
        const responses = await Promise.all([
            fetch('/api/manus/projects'),
            fetch('/api/manus/connectors'),
            fetch('/api/manus/skills')
        ]);
        const payloads = await Promise.all(responses.map(resp => resp.json().catch(() => ({}))));
        const failed = responses.findIndex(resp => !resp.ok);
        if (failed >= 0) throw new Error(payloads[failed].error || payloads[failed].message || t('config.manus.catalog_load_failed'));
        manusCatalogState.projects = payloads[0].items || [];
        manusCatalogState.connectors = payloads[1].items || [];
        manusCatalogState.skills = payloads[2].items || [];
        manusRenderCatalogs();
    } catch (error) {
        if (showErrors) showToast(t('config.manus.catalog_load_failed') + ': ' + (error.message || t('config.common.error')), 'error');
    } finally {
        manusCatalogState.loading = false;
    }
}

function manusRenderCatalogs() {
    const cfg = manusConfig();
    manusRenderCatalog('projects', 'allowed_project_ids', cfg.allowed_project_ids);
    manusRenderCatalog('connectors', 'allowed_connector_ids', cfg.allowed_connector_ids);
    manusRenderCatalog('skills', 'allowed_skill_ids', cfg.allowed_skill_ids);
}

function manusRenderCatalog(kind, configKey, selectedIDs) {
    const container = document.getElementById('manus-' + kind + '-catalog');
    if (!container) return;
    const items = manusCatalogState[kind] || [];
    const selected = new Set((selectedIDs || []).map(String));
    if (!items.length) { container.innerHTML = '<span class="manus-catalog-empty">' + t('config.manus.catalog_empty') + '</span>'; return; }
    container.innerHTML = items.map(item => {
        const id = String(item.id || '');
        const name = String(item.name || id);
        const description = String(item.description || item.instruction || item.category || '');
        const active = selected.has(id);
        return '<button type="button" class="manus-catalog-item' + (active ? ' is-selected' : '') + '" onclick="manusToggleCatalogItem(' + manusJSArg(kind) + ', ' + manusJSArg(configKey) + ', ' + manusJSArg(id) + ')" aria-pressed="' + active + '"><strong>' + escapeHtml(name) + '</strong><small>' + escapeHtml(description || id) + '</small></button>';
    }).join('');
}

function manusJSArg(value) {
    return escapeAttr(JSON.stringify(String(value || '')));
}

function manusToggleCatalogItem(kind, configKey, id) {
    const cfg = manusConfig();
    const current = new Set((cfg[configKey] || []).map(String));
    if (current.has(id)) current.delete(id); else current.add(id);
    cfg[configKey] = Array.from(current);
    const input = document.querySelector('[data-path="manus.' + configKey + '"]');
    if (input) input.value = cfg[configKey].join(', ');
    markDirty();
    manusRenderCatalog(kind, configKey, cfg[configKey]);
}
