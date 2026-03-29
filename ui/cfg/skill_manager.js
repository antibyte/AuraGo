// cfg/skill_manager.js — Skill Manager configuration section module

function renderSkillManagerSection(section) {
    const data = configData['skill_manager'] || {};
    const enabledOn = data.enabled === true;
    const allowUploadsOn = data.allow_uploads === true;
    const readonlyOn = data.read_only === true;
    const requireScanOn = data.require_scan !== false;
    const requireSandboxOn = data.require_sandbox === true;
    const autoEnableOn = data.auto_enable_clean === true;
    const guardianOn = data.scan_with_guardian === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    const helpEnabled = t('help.skill_manager.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="skill_manager.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Allow Uploads toggle ──
    const helpUploads = t('help.skill_manager.allow_uploads');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.allow_uploads_label') + '</div>';
    if (helpUploads) html += '<div class="field-help">' + helpUploads + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (allowUploadsOn ? ' on' : '') + '" data-path="skill_manager.allow_uploads" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (allowUploadsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    const helpReadonly = t('help.skill_manager.read_only');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.read_only_label') + '</div>';
    if (helpReadonly) html += '<div class="field-help">' + helpReadonly + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="skill_manager.read_only" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Require Scan toggle ──
    const helpScan = t('help.skill_manager.require_scan');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.require_scan_label') + '</div>';
    if (helpScan) html += '<div class="field-help">' + helpScan + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (requireScanOn ? ' on' : '') + '" data-path="skill_manager.require_scan" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (requireScanOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Require Sandbox toggle ──
    const helpSandbox = t('help.skill_manager.require_sandbox');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.require_sandbox_label') + '</div>';
    if (helpSandbox) html += '<div class="field-help">' + helpSandbox + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (requireSandboxOn ? ' on' : '') + '" data-path="skill_manager.require_sandbox" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (requireSandboxOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Auto-enable Clean toggle ──
    const helpAutoEnable = t('help.skill_manager.auto_enable_clean');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.auto_enable_clean_label') + '</div>';
    if (helpAutoEnable) html += '<div class="field-help">' + helpAutoEnable + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (autoEnableOn ? ' on' : '') + '" data-path="skill_manager.auto_enable_clean" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (autoEnableOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Scan with Guardian toggle ──
    const helpGuardian = t('help.skill_manager.scan_with_guardian');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.scan_with_guardian_label') + '</div>';
    if (helpGuardian) html += '<div class="field-help">' + helpGuardian + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (guardianOn ? ' on' : '') + '" data-path="skill_manager.scan_with_guardian" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (guardianOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Max Upload Size ──
    const helpMaxSize = t('help.skill_manager.max_upload_size_mb');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.max_upload_size_mb_label') + '</div>';
    if (helpMaxSize) html += '<div class="field-help">' + helpMaxSize + '</div>';
    html += '<input class="field-input" type="number" min="1" max="50" data-path="skill_manager.max_upload_size_mb" value="' + escapeAttr(data.max_upload_size_mb || 1) + '">';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
}
