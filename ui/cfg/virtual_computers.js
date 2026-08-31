// cfg/virtual_computers.js - Virtual Computers config section module

let _virtualComputersSection = null;
let _vcSudoPasswordStored = null;
let _vcSudoPasswordStatusGeneration = 0;

function vcCfgEnsureData() {
    if (!configData.virtual_computers) configData.virtual_computers = {};
    if (!configData.virtual_computers.control_plane) configData.virtual_computers.control_plane = {};
	if (!configData.virtual_computers.storage) configData.virtual_computers.storage = {};
	if (!configData.virtual_computers.agent_control) configData.virtual_computers.agent_control = {};
	if (!configData.virtual_computers.agent_control.network) configData.virtual_computers.agent_control.network = {};
	if (!configData.virtual_computers.agent_control.credentials) configData.virtual_computers.agent_control.credentials = {};
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.virtual_computers) configData.tools.virtual_computers = {};
    const data = configData.virtual_computers;
    const cp = data.control_plane;
	const storage = data.storage;
	const agentControl = data.agent_control;
    if (!data.provider) data.provider = 'boring_computers';
    if (!data.default_template) data.default_template = 'python';
    if (!data.default_ttl_seconds) data.default_ttl_seconds = 600;
    if (!data.max_ttl_seconds) data.max_ttl_seconds = 900;
    if (!data.max_running_machines) data.max_running_machines = 3;
    if (!data.max_forks) data.max_forks = 3;
    if (!cp.mode) cp.mode = 'ssh_host';
    if (!cp.ssh_port) cp.ssh_port = 22;
    if (!cp.install_dir) cp.install_dir = '/opt/boring-computers';
    if (!cp.boringd_url) cp.boringd_url = 'http://127.0.0.1:18082';
	if (!storage.mode) storage.mode = 'managed_garage';
	if (!storage.bucket) storage.bucket = 'boring-volumes';
	if (typeof storage.use_ssl !== 'boolean') storage.use_ssl = storage.mode === 'external_s3';
	if (!agentControl.default_template) agentControl.default_template = 'desktop';
	if (!agentControl.max_active_workspaces) agentControl.max_active_workspaces = 2;
	if (!agentControl.idle_ttl_seconds) agentControl.idle_ttl_seconds = 600;
	if (!agentControl.max_workspace_seconds) agentControl.max_workspace_seconds = 7200;
	if (!agentControl.max_job_seconds) agentControl.max_job_seconds = 3600;
	if (!agentControl.max_job_output_mb) agentControl.max_job_output_mb = 4;
	if (!agentControl.jobs_per_workspace) agentControl.jobs_per_workspace = 2;
	if (!agentControl.browser_sessions_per_workspace) agentControl.browser_sessions_per_workspace = 1;
	if (!agentControl.network.default_profile) agentControl.network.default_profile = 'internet_lan';
	if (!Array.isArray(agentControl.network.allowed_private_cidrs)) agentControl.network.allowed_private_cidrs = [];
	if (typeof agentControl.credentials.enabled !== 'boolean') agentControl.credentials.enabled = true;
	if (!agentControl.credentials.grant_ttl_seconds) agentControl.credentials.grant_ttl_seconds = 900;
    return data;
}

function vcCfgAgentProviderOptions(selectedProvider) {
    const providers = (typeof providersCache !== 'undefined' && Array.isArray(providersCache) ? providersCache : [])
        .filter(function(provider) {
            return String(provider.type || '').trim().toLowerCase() === 'anthropic' &&
                String(provider.auth_type || 'api_key').trim().toLowerCase() !== 'oauth2';
        });
    let html = '<option value="">' + escapeAttr(t('config.virtual_computers.agent_provider_none')) + '</option>';
    providers.forEach(function(provider) {
        const id = String(provider.id || '').trim();
        if (!id) return;
        const configured = Boolean(provider.api_key);
        const name = String(provider.name || id);
        const model = String(provider.model || '').trim();
        const label = name + (model ? ' — ' + model : '') + (configured ? '' : ' — ' + t('config.virtual_computers.agent_provider_key_missing'));
        html += '<option value="' + escapeAttr(id) + '"' + (id === selectedProvider ? ' selected' : '') + (configured ? '' : ' disabled') + '>' + escapeAttr(label) + '</option>';
    });
    return html;
}

function renderVirtualComputersSection(section) {
    if (section) _virtualComputersSection = section; else section = _virtualComputersSection;
    const data = vcCfgEnsureData();
    const cp = data.control_plane || {};
    const enabled = data.enabled === true;
    const mode = cp.mode || 'ssh_host';
    const isLocalHost = mode === 'local_host';

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += vcCfgToggleRow('config.virtual_computers.enabled_label', 'help.virtual_computers.enabled', enabled, 'virtual_computers.enabled', 'vcCfgToggleEnabled(this)');

    if (!enabled) {
        html += '<div class="wh-notice"><span>i</span><div>';
        html += '<strong>' + t('config.virtual_computers.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.virtual_computers.disabled_desc') + '</small>';
        html += '</div></div>';
        html += '</div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">' + t('config.virtual_computers.private_note') + '</div>';
    html += vcCfgToggleRow('config.virtual_computers.readonly_label', 'help.virtual_computers.readonly', data.readonly === true, 'virtual_computers.readonly');
    html += vcCfgToggleRow('config.virtual_computers.tool_enabled_label', 'help.virtual_computers.tool_enabled', configData.tools.virtual_computers.enabled === true, 'tools.virtual_computers.enabled');
    html += vcCfgToggleRow('config.virtual_computers.auto_setup_label', 'help.virtual_computers.auto_setup', data.auto_setup === true, 'virtual_computers.auto_setup');

	const agentControl = data.agent_control || {};
	const agentNetwork = agentControl.network || {};
	const agentCredentials = agentControl.credentials || {};
	html += '<div class="field-group">';
	html += '<div class="field-group-title">' + t('config.virtual_computers.agent_control_title') + '</div>';
	html += '<div class="field-group-desc">' + t('config.virtual_computers.agent_control_desc') + '</div>';
	html += vcCfgToggleRow('config.virtual_computers.agent_control_enabled_label', 'help.virtual_computers.agent_control_enabled', agentControl.enabled === true, 'virtual_computers.agent_control.enabled', 'vcCfgToggleAgentControl(this)');
	if (agentControl.enabled === true) {
		html += '<div class="cfg-note-banner cfg-note-banner-info">' + t('config.virtual_computers.agent_control_gate_note') + '</div>';
		html += '<div class="field-grid two-cols">';
		html += vcCfgField('config.virtual_computers.agent_control_template_label', 'help.virtual_computers.agent_control_template',
			'<select class="field-select" data-path="virtual_computers.agent_control.default_template"><option value="desktop"' + ((agentControl.default_template || 'desktop') === 'desktop' ? ' selected' : '') + '>desktop</option><option value="python"' + (agentControl.default_template === 'python' ? ' selected' : '') + '>python</option></select>');
		html += vcCfgField('config.virtual_computers.agent_control_max_workspaces_label', 'help.virtual_computers.agent_control_max_workspaces',
			'<input class="field-input" type="number" min="1" max="16" data-path="virtual_computers.agent_control.max_active_workspaces" value="' + escapeAttr(agentControl.max_active_workspaces || 2) + '">');
		html += vcCfgField('config.virtual_computers.agent_control_idle_ttl_label', 'help.virtual_computers.agent_control_idle_ttl',
			'<input class="field-input" type="number" min="30" max="900" data-path="virtual_computers.agent_control.idle_ttl_seconds" value="' + escapeAttr(agentControl.idle_ttl_seconds || 600) + '">');
		html += vcCfgField('config.virtual_computers.agent_control_max_workspace_label', 'help.virtual_computers.agent_control_max_workspace',
			'<input class="field-input" type="number" min="60" max="86400" data-path="virtual_computers.agent_control.max_workspace_seconds" value="' + escapeAttr(agentControl.max_workspace_seconds || 7200) + '">');
		html += vcCfgField('config.virtual_computers.agent_control_max_job_label', 'help.virtual_computers.agent_control_max_job',
			'<input class="field-input" type="number" min="1" max="86400" data-path="virtual_computers.agent_control.max_job_seconds" value="' + escapeAttr(agentControl.max_job_seconds || 3600) + '">');
		html += vcCfgField('config.virtual_computers.agent_control_output_label', 'help.virtual_computers.agent_control_output',
			'<input class="field-input" type="number" min="1" max="64" data-path="virtual_computers.agent_control.max_job_output_mb" value="' + escapeAttr(agentControl.max_job_output_mb || 4) + '">');
		html += vcCfgField('config.virtual_computers.agent_control_jobs_label', 'help.virtual_computers.agent_control_jobs',
			'<input class="field-input" type="number" min="1" max="2" data-path="virtual_computers.agent_control.jobs_per_workspace" value="' + escapeAttr(agentControl.jobs_per_workspace || 2) + '">');
		html += vcCfgField('config.virtual_computers.agent_control_browsers_label', 'help.virtual_computers.agent_control_browsers',
			'<input class="field-input" type="number" min="1" max="1" data-path="virtual_computers.agent_control.browser_sessions_per_workspace" value="' + escapeAttr(agentControl.browser_sessions_per_workspace || 1) + '">');
		html += vcCfgField('config.virtual_computers.agent_control_network_label', 'help.virtual_computers.agent_control_network',
			'<select class="field-select" data-path="virtual_computers.agent_control.network.default_profile"><option value="internet_lan" selected>internet_lan</option></select>');
		html += '</div>';
		html += vcCfgField('config.virtual_computers.agent_control_cidrs_label', 'help.virtual_computers.agent_control_cidrs',
			'<textarea class="field-input" rows="3" data-path="virtual_computers.agent_control.network.allowed_private_cidrs" data-type="array-lines" placeholder="192.168.10.0/24">' + escapeHtml((agentNetwork.allowed_private_cidrs || []).join('\n')) + '</textarea>');
		if (!agentNetwork.allowed_private_cidrs || agentNetwork.allowed_private_cidrs.length === 0) {
			html += '<div class="cfg-note-banner cfg-note-banner-warning">' + t('config.virtual_computers.agent_control_no_lan_warning') + '</div>';
		}
		html += '<div class="field-grid two-cols">';
		html += vcCfgToggleRow('config.virtual_computers.agent_control_credentials_label', 'help.virtual_computers.agent_control_credentials', agentCredentials.enabled === true, 'virtual_computers.agent_control.credentials.enabled');
		html += vcCfgField('config.virtual_computers.agent_control_grant_ttl_label', 'help.virtual_computers.agent_control_grant_ttl',
			'<input class="field-input" type="number" min="60" max="3600" data-path="virtual_computers.agent_control.credentials.grant_ttl_seconds" value="' + escapeAttr(agentCredentials.grant_ttl_seconds || 900) + '">');
		html += '</div>';
	}
	html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.virtual_computers.control_plane_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.virtual_computers.control_plane_desc') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += vcCfgField('config.virtual_computers.provider_label', 'help.virtual_computers.provider',
        '<select class="field-select" data-path="virtual_computers.provider"><option value="boring_computers" selected>boring_computers</option></select>');
    html += vcCfgField('config.virtual_computers.mode_label', 'help.virtual_computers.mode',
        '<select class="field-select" data-path="virtual_computers.control_plane.mode" onchange="vcCfgOnModeChange(this)">' +
        '<option value="ssh_host"' + (mode === 'ssh_host' ? ' selected' : '') + '>' + t('config.virtual_computers.mode_ssh_host') + '</option>' +
        '<option value="local_host"' + (isLocalHost ? ' selected' : '') + '>' + t('config.virtual_computers.mode_local_host') + '</option>' +
        '</select>');
    if (isLocalHost) {
        html += '</div>';
        html += '<div class="cfg-note-banner cfg-note-banner-info">' + t('config.virtual_computers.local_host_note') + '</div>';
        html += '<div class="field-group">';
        html += '<div class="field-label">' + t('config.virtual_computers.sudo_password_label') + '</div>';
        html += '<div class="field-help">' + t('help.virtual_computers.sudo_password') + '</div>';
        html += '<div class="cfg-password-row">';
        html += '<div class="password-wrap cfg-password-input">';
        html += '<input class="field-input adg-password-input" type="password" id="vc-sudo-password-input" value="" autocomplete="off">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
        html += '<button type="button" class="btn-save adg-save-btn" onclick="vcCfgSaveSudoPassword()">' + t('config.virtual_computers.save_vault') + '</button>';
        html += '</div>';
        html += '<div id="vc-sudo-password-status" class="adg-test-result' + (_vcSudoPasswordStored === true ? ' is-success' : '') + '">' +
            (_vcSudoPasswordStored === true ? t('config.virtual_computers.sudo_password_stored') : (_vcSudoPasswordStored === false ? t('config.virtual_computers.sudo_password_missing') : t('config.virtual_computers.status_loading'))) + '</div>';
        html += '</div>';
        html += '<div class="field-grid two-cols">';
    } else {
        html += vcCfgField('config.virtual_computers.host_label', 'help.virtual_computers.host',
            '<input class="field-input" type="text" data-path="virtual_computers.control_plane.host" value="' + escapeAttr(cp.host || '') + '" placeholder="vm-host.local">');
        html += vcCfgField('config.virtual_computers.ssh_port_label', 'help.virtual_computers.ssh_port',
            '<input class="field-input" type="number" min="1" max="65535" data-path="virtual_computers.control_plane.ssh_port" value="' + escapeAttr(cp.ssh_port || 22) + '">');
        html += vcCfgField('config.virtual_computers.credential_id_label', 'help.virtual_computers.credential_id',
            '<input class="field-input" type="text" data-path="virtual_computers.control_plane.credential_id" value="' + escapeAttr(cp.credential_id || '') + '" placeholder="credential-id">');
    }
    html += vcCfgField('config.virtual_computers.install_dir_label', 'help.virtual_computers.install_dir',
        '<input class="field-input" type="text" data-path="virtual_computers.control_plane.install_dir" value="' + escapeAttr(cp.install_dir || '/opt/boring-computers') + '">');
    html += vcCfgField('config.virtual_computers.boringd_url_label', 'help.virtual_computers.boringd_url',
        '<input class="field-input" type="url" data-path="virtual_computers.control_plane.boringd_url" value="' + escapeAttr(cp.boringd_url || 'http://127.0.0.1:18082') + '">');
    html += '</div>';
    html += '</div>';

	const storage = data.storage || {};
	const storageMode = (storage.mode || 'managed_garage') === 'external_s3' ? 'external_s3' : 'managed_garage';
	const installDir = (cp.install_dir || '/opt/boring-computers').replace(/\/+$/, '');
	html += '<div class="field-group">';
	html += '<div class="field-group-title">' + t('config.virtual_computers.storage_title') + '</div>';
	html += '<div class="field-group-desc">' + t('config.virtual_computers.storage_desc') + '</div>';
	html += '<div class="field-grid two-cols">';
	html += vcCfgField('config.virtual_computers.storage_mode_label', 'help.virtual_computers.storage_mode',
		'<select class="field-select" data-path="virtual_computers.storage.mode" onchange="vcCfgOnStorageModeChange(this)">' +
		'<option value="managed_garage"' + (storageMode === 'managed_garage' ? ' selected' : '') + '>' + escapeAttr(t('config.virtual_computers.storage_mode_managed')) + '</option>' +
		'<option value="external_s3"' + (storageMode === 'external_s3' ? ' selected' : '') + '>' + escapeAttr(t('config.virtual_computers.storage_mode_external')) + '</option>' +
		'</select>');
	html += '</div>';
	if (storageMode === 'managed_garage') {
		html += '<div class="field-help" id="vc-storage-managed-note">' + t('config.virtual_computers.storage_managed_note') + '</div>';
		html += '<div class="field-grid two-cols">';
		html += vcCfgField('config.virtual_computers.storage_managed_endpoint_label', 'help.virtual_computers.storage_managed_endpoint',
			'<input class="field-input" type="text" value="127.0.0.1:3900" disabled readonly>');
		html += vcCfgField('config.virtual_computers.storage_managed_path_label', 'help.virtual_computers.storage_managed_path',
			'<input class="field-input" type="text" value="' + escapeAttr(installDir + '/data/sidecars/garage') + '" disabled readonly>');
		html += vcCfgField('config.virtual_computers.storage_managed_image_label', 'help.virtual_computers.storage_managed_image',
			'<input class="field-input" type="text" value="dxflrs/garage:v2.3.0" disabled readonly>');
		html += '</div>';
		html += '<div class="field-help">' + t('config.virtual_computers.storage_managed_single_node_warn') + '</div>';
		html += '<div class="field-help">' + t('config.virtual_computers.storage_managed_creds_note') + '</div>';
	} else {
		html += '<div class="field-grid two-cols">';
		html += vcCfgField('config.virtual_computers.storage_endpoint_label', 'help.virtual_computers.storage_endpoint',
			'<input class="field-input" type="text" data-path="virtual_computers.storage.endpoint" value="' + escapeAttr(storage.endpoint || '') + '" placeholder="minio.local:9000">');
		html += vcCfgField('config.virtual_computers.storage_bucket_label', 'help.virtual_computers.storage_bucket',
			'<input class="field-input" type="text" data-path="virtual_computers.storage.bucket" value="' + escapeAttr(storage.bucket || 'boring-volumes') + '">');
		html += vcCfgField('config.virtual_computers.storage_region_label', 'help.virtual_computers.storage_region',
			'<input class="field-input" type="text" data-path="virtual_computers.storage.region" value="' + escapeAttr(storage.region || '') + '">');
		html += vcCfgToggleRow('config.virtual_computers.storage_ssl_label', 'help.virtual_computers.storage_ssl', storage.use_ssl === true, 'virtual_computers.storage.use_ssl');
		html += '</div>';
		html += '<div class="field-label">' + t('config.virtual_computers.s3_access_key_label') + '</div>';
		html += '<div class="field-help">' + t('help.virtual_computers.s3_access_key') + '</div>';
		html += '<div class="adg-password-row"><div class="password-wrap cfg-password-input">';
		html += '<input class="field-input adg-password-input" type="password" id="vc-s3-access-input" value="' + escapeAttr(cfgSecretValue(data.s3_access_key_id)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(data.s3_access_key_id, 'access-key')) + '" autocomplete="off">';
		html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>';
		html += '<button class="btn-save adg-save-btn" onclick="vcCfgSaveSecret(\'vc-s3-access-input\', \'virtual_computers_s3_access_key_id\', \'virtual_computers.s3_access_key_id\', \'vc-s3-access-status\')">' + t('config.virtual_computers.save_vault') + '</button></div><div id="vc-s3-access-status" class="adg-test-result"></div>';
		html += '<div class="field-label">' + t('config.virtual_computers.s3_secret_key_label') + '</div>';
		html += '<div class="field-help">' + t('help.virtual_computers.s3_secret_key') + '</div>';
		html += '<div class="adg-password-row"><div class="password-wrap cfg-password-input">';
		html += '<input class="field-input adg-password-input" type="password" id="vc-s3-secret-input" value="' + escapeAttr(cfgSecretValue(data.s3_secret_key)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(data.s3_secret_key, 'secret-key')) + '" autocomplete="off">';
		html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>';
		html += '<button class="btn-save adg-save-btn" onclick="vcCfgSaveSecret(\'vc-s3-secret-input\', \'virtual_computers_s3_secret_key\', \'virtual_computers.s3_secret_key\', \'vc-s3-secret-status\')">' + t('config.virtual_computers.save_vault') + '</button></div><div id="vc-s3-secret-status" class="adg-test-result"></div>';
	}
	html += '<button class="btn-save dc-test-btn" onclick="vcCfgTestStorage()" id="vc-storage-test-btn">' + t('config.virtual_computers.storage_test_button') + '</button>';
	html += '<span id="vc-storage-test-result" class="dc-test-result"></span>';
	html += '<div id="vc-storage-test-lock" class="field-help"></div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.virtual_computers.limits_title') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += vcCfgField('config.virtual_computers.default_template_label', 'help.virtual_computers.default_template',
        '<select class="field-select" data-path="virtual_computers.default_template">' +
        '<option value="python"' + ((data.default_template || 'python') === 'python' ? ' selected' : '') + '>python</option>' +
        '<option value="desktop"' + (data.default_template === 'desktop' ? ' selected' : '') + '>desktop</option>' +
        '</select>');
    html += vcCfgField('config.virtual_computers.default_ttl_label', 'help.virtual_computers.default_ttl_seconds',
        '<input class="field-input" type="number" min="15" max="900" data-path="virtual_computers.default_ttl_seconds" value="' + escapeAttr(data.default_ttl_seconds || 600) + '">');
    html += vcCfgField('config.virtual_computers.max_ttl_label', 'help.virtual_computers.max_ttl_seconds',
        '<input class="field-input" type="number" min="15" max="900" data-path="virtual_computers.max_ttl_seconds" value="' + escapeAttr(data.max_ttl_seconds || 900) + '">');
    html += vcCfgField('config.virtual_computers.max_running_label', 'help.virtual_computers.max_running_machines',
        '<input class="field-input" type="number" min="1" max="50" data-path="virtual_computers.max_running_machines" value="' + escapeAttr(data.max_running_machines || 3) + '">');
    html += vcCfgField('config.virtual_computers.max_forks_label', 'help.virtual_computers.max_forks',
        '<input class="field-input" type="number" min="0" max="50" data-path="virtual_computers.max_forks" value="' + escapeAttr(data.max_forks || 3) + '">');
    html += '</div>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.virtual_computers.gates_title') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += vcCfgToggleRow('config.virtual_computers.allow_internet_label', 'help.virtual_computers.allow_internet', data.allow_internet === true, 'virtual_computers.allow_internet');
    html += vcCfgToggleRow('config.virtual_computers.allow_persistent_label', 'help.virtual_computers.allow_persistent', data.allow_persistent === true, 'virtual_computers.allow_persistent');
    html += vcCfgToggleRow('config.virtual_computers.allow_publish_label', 'help.virtual_computers.allow_publish', data.allow_publish === true, 'virtual_computers.allow_publish');
    html += vcCfgToggleRow('config.virtual_computers.allow_volumes_label', 'help.virtual_computers.allow_volumes', data.allow_volumes === true, 'virtual_computers.allow_volumes');
    html += vcCfgToggleRow('config.virtual_computers.allow_agent_tasks_label', 'help.virtual_computers.allow_agent_tasks', data.allow_agent_tasks === true, 'virtual_computers.allow_agent_tasks', 'vcCfgToggleAgentTasks(this)');
    html += '</div>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.virtual_computers.agent_provider_title') + '</div>';
    html += '<div class="field-label">' + t('config.virtual_computers.agent_provider_label') + '</div>';
    html += '<div class="field-help">' + t('help.virtual_computers.agent_provider') + '</div>';
    html += '<select class="field-select" data-path="virtual_computers.agent_provider"' + (data.allow_agent_tasks === true ? '' : ' disabled') + '>';
    html += vcCfgAgentProviderOptions(String(data.agent_provider || '').trim());
    html += '</select>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.virtual_computers.boring_token_label') + '</div>';
    html += '<div class="field-help">' + t('help.virtual_computers.boring_token') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap cfg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="vc-boring-token-input" value="' + escapeAttr(cfgSecretValue(data.boring_token)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(data.boring_token, 'boring-token')) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="vcCfgSaveSecret(\'vc-boring-token-input\', \'virtual_computers_boring_token\', \'virtual_computers.boring_token\', \'vc-boring-token-status\')">' + t('config.virtual_computers.save_vault') + '</button>';
    html += '</div><div id="vc-boring-token-status" class="adg-test-result"></div>';
    html += '</div>';

    html += '<div class="cfg-note-banner cfg-note-banner-info">' + t('config.virtual_computers.tailscale_note') + ' <a href="/config#tailscale">' + t('config.virtual_computers.open_tailscale_settings') + '</a></div>';
    html += '<div class="field-group">';
    html += '<button class="btn-save dc-test-btn" onclick="vcCfgCheckStatus()" id="vc-status-btn">' + t('config.virtual_computers.status_button') + '</button>';
    html += '<button class="btn-save dc-test-btn" onclick="vcCfgPreflight()" id="vc-preflight-btn">' + t('config.virtual_computers.preflight_button') + '</button>';
    html += '<button class="btn-save dc-test-btn" onclick="vcCfgInstall()" id="vc-install-btn">' + t('config.virtual_computers.install_button') + '</button>';
    html += '<span id="vc-cfg-result" class="dc-test-result"></span>';
    html += '</div></div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    if (isLocalHost) void vcCfgRefreshSudoPasswordStatus();
}

function vcCfgOnModeChange(el) {
    const value = el && el.value ? el.value : 'ssh_host';
    const data = vcCfgEnsureData();
    data.control_plane.mode = value;
    setNestedValue(configData, 'virtual_computers.control_plane.mode', value);
    if (window.AuraConfigState) {
        window.AuraConfigState.set('virtual_computers.control_plane.mode', value);
    }
    renderVirtualComputersSection(null);
    setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
}

function vcCfgToggleRow(labelKey, helpKey, enabled, path, onclick) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="' + (onclick || 'toggleBool(this)') + '"></div>';
    html += '</div>';
    return html;
}

function vcCfgField(labelKey, helpKey, inputHtml) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

function vcCfgToggleEnabled(el) {
    const isOn = el && el.classList ? el.classList.contains('on') : false;
    const nextEnabled = !isOn;
    const data = vcCfgEnsureData();
    data.enabled = nextEnabled;
    setNestedValue(configData, 'virtual_computers.enabled', data.enabled);
    if (window.AuraConfigState) {
        window.AuraConfigState.set('virtual_computers.enabled', nextEnabled);
    }
    renderVirtualComputersSection(null);
    setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
}

function vcCfgSetResult(text, state) {
    const result = document.getElementById('vc-cfg-result');
    if (!result) return;
    result.className = 'dc-test-result' + (state ? ' is-' + state : '');
    result.textContent = text || '';
}

async function vcCfgCheckStatus() {
    const btn = document.getElementById('vc-status-btn');
    if (btn) btn.disabled = true;
    vcCfgSetResult(t('config.virtual_computers.status_loading'), '');
    try {
        const resp = await fetch('/api/virtual-computers/setup/status');
        const body = await resp.json();
        if (!resp.ok || body.error) throw new Error(body.error || resp.statusText);
        vcCfgSetSudoPasswordStatus(body.sudo_password_stored === true);
        vcCfgSetResult(body.enabled ? t('config.virtual_computers.status_ok') : t('config.virtual_computers.status_disabled'), body.enabled ? 'success' : '');
    } catch (e) {
        vcCfgSetResult(t('config.virtual_computers.status_error') + ' ' + e.message, 'danger');
    } finally {
        if (btn) btn.disabled = false;
    }
}

function vcCfgSetSudoPasswordStatus(stored, messageKey, state) {
    _vcSudoPasswordStored = stored === true;
    const statusEl = document.getElementById('vc-sudo-password-status');
    if (!statusEl) return;
    statusEl.className = 'adg-test-result' + (state ? ' is-' + state : (_vcSudoPasswordStored ? ' is-success' : ''));
    statusEl.textContent = t(messageKey || (_vcSudoPasswordStored ? 'config.virtual_computers.sudo_password_stored' : 'config.virtual_computers.sudo_password_missing'));
}

async function vcCfgRefreshSudoPasswordStatus() {
    const requestGeneration = ++_vcSudoPasswordStatusGeneration;
    try {
        const resp = await fetch('/api/virtual-computers/setup/status');
        const body = await resp.json();
        if (!resp.ok || body.error) throw new Error(body.error || resp.statusText);
        if (requestGeneration !== _vcSudoPasswordStatusGeneration) return;
        vcCfgSetSudoPasswordStatus(body.sudo_password_stored === true);
    } catch (_) {
        if (requestGeneration !== _vcSudoPasswordStatusGeneration) return;
        const statusEl = document.getElementById('vc-sudo-password-status');
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.virtual_computers.status_error');
        }
    }
}

async function vcCfgSaveSudoPassword() {
    const input = document.getElementById('vc-sudo-password-input');
    const value = input ? input.value : '';
    if (!value) {
        vcCfgSetSudoPasswordStatus(
            _vcSudoPasswordStored,
            _vcSudoPasswordStored ? 'config.virtual_computers.sudo_password_stored' : 'config.virtual_computers.sudo_password_missing',
            _vcSudoPasswordStored ? 'success' : 'danger'
        );
        return;
    }
    ++_vcSudoPasswordStatusGeneration;
    try {
        const resp = await fetch('/api/vault/secrets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: 'sudo_password', value })
        });
        const body = await resp.json().catch(() => ({}));
        if (!resp.ok || body.error || (body.status && body.status !== 'ok')) {
            throw new Error(body.error || body.message || resp.statusText);
        }
        input.value = '';
        vcCfgSetSudoPasswordStatus(true, 'config.virtual_computers.sudo_password_saved', 'success');
    } catch (_) {
        vcCfgSetSudoPasswordStatus(_vcSudoPasswordStored, 'config.virtual_computers.sudo_password_save_failed', 'danger');
    }
}

async function vcCfgPreflight() {
    await vcCfgPostSetup('/api/virtual-computers/setup/preflight', 'vc-preflight-btn');
}

async function vcCfgInstall() {
    await vcCfgPostSetup('/api/virtual-computers/setup/install', 'vc-install-btn', true);
}

function vcCfgToggleAgentTasks(el) {
    toggleBool(el);
    const enabled = Boolean(el && el.classList && el.classList.contains('on'));
    setNestedValue(configData, 'virtual_computers.allow_agent_tasks', enabled);
    renderVirtualComputersSection(null);
    setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
}

function vcCfgToggleAgentControl(el) {
	toggleBool(el);
	const enabled = Boolean(el && el.classList && el.classList.contains('on'));
	setNestedValue(configData, 'virtual_computers.agent_control.enabled', enabled);
	renderVirtualComputersSection(null);
	setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
}

function vcCfgOnStorageModeChange(el) {
	const mode = el && el.value === 'external_s3' ? 'external_s3' : 'managed_garage';
	setNestedValue(configData, 'virtual_computers.storage.mode', mode);
	renderVirtualComputersSection(null);
	setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
	const lock = document.getElementById('vc-storage-test-lock');
	if (lock) {
		lock.textContent = t('config.virtual_computers.storage_switch_hint');
	}
}

function vcCfgStorageActionsLocked() {
	if (typeof isDirty !== 'undefined' && isDirty) {
		return t('config.virtual_computers.storage_test_dirty_lock');
	}
	return '';
}

async function vcCfgTestStorage() {
	const btn = document.getElementById('vc-storage-test-btn');
	const result = document.getElementById('vc-storage-test-result');
	const lock = document.getElementById('vc-storage-test-lock');
	const lockReason = vcCfgStorageActionsLocked();
	if (lockReason) {
		if (lock) lock.textContent = lockReason;
		if (result) { result.className = 'dc-test-result is-danger'; result.textContent = lockReason; }
		return;
	}
	if (lock) lock.textContent = '';
	if (btn) btn.disabled = true;
	try {
		const resp = await fetch('/api/virtual-computers/storage/test', { method: 'POST' });
		const body = await resp.json().catch(() => ({}));
		if (!resp.ok || body.error) throw new Error(body.error || resp.statusText);
		if (result) { result.className = 'dc-test-result is-success'; result.textContent = t('config.virtual_computers.storage_test_ok'); }
	} catch (e) {
		if (result) { result.className = 'dc-test-result is-danger'; result.textContent = t('config.virtual_computers.storage_test_failed') + ' ' + e.message; }
	} finally {
		if (btn) btn.disabled = false;
	}
}

async function vcCfgPostSetup(url, buttonID, showElapsed) {
    const btn = document.getElementById(buttonID);
    if (btn) btn.disabled = true;
    vcCfgSetResult(t('config.virtual_computers.status_loading'), '');
    let elapsedTimer = null;
    if (showElapsed) {
        const startedAt = Date.now();
        const updateElapsed = () => {
            const seconds = Math.max(0, Math.floor((Date.now() - startedAt) / 1000));
            vcCfgSetResult(t('config.virtual_computers.install_button') + '… ' + seconds + 's', '');
        };
        updateElapsed();
        elapsedTimer = window.setInterval(updateElapsed, 1000);
    }
    try {
        const resp = await fetch(url, { method: 'POST' });
        const body = await resp.json();
        if (!resp.ok || body.error || body.status === 'unhealthy') {
            throw new Error(body.error || (body.setup && body.setup.message) || body.message || resp.statusText);
        }
        const warnings = (body.result && body.result.warnings) || (body.setup && body.setup.preflight && body.setup.preflight.warnings) || [];
        const message = body.message || t('config.virtual_computers.status_ok');
        vcCfgSetResult(warnings.length ? message + ' ' + warnings.join('; ') : message, warnings.length ? 'warning' : 'success');
    } catch (e) {
        vcCfgSetResult(t('config.virtual_computers.status_error') + ' ' + e.message, 'danger');
    } finally {
        if (elapsedTimer !== null) window.clearInterval(elapsedTimer);
        if (btn) btn.disabled = false;
    }
}

function vcCfgSaveSecret(inputID, vaultKey, configPath, statusID) {
    const input = document.getElementById(inputID);
    const statusEl = document.getElementById(statusID);
    const value = input ? input.value.trim() : '';
    if (!value) {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.virtual_computers.token_empty');
        }
        return;
    }
    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: vaultKey, value })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            if (statusEl) {
                statusEl.className = 'adg-test-result is-success';
                statusEl.textContent = t('config.virtual_computers.token_saved');
            }
            cfgMarkSecretStored(input, configPath);
        } else if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = res.message || t('config.virtual_computers.token_save_failed');
        }
    })
    .catch(() => {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.virtual_computers.token_save_failed');
        }
    });
}
