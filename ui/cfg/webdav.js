// cfg/webdav.js — WebDAV integration config panel

let _wbdSection = null;

function renderWebDAVSection(section) {
    if (section) _wbdSection = section; else section = _wbdSection;
    const data = configData.webdav || {};
    const enabled = data.enabled === true;
    const readonly = data.readonly === true;
    const authType = data.auth_type === 'bearer' ? 'bearer' : 'basic';
    const secretValue = authType === 'bearer' ? data.token : data.password;
    const placeholder = cfgSecretPlaceholder(secretValue, authType === 'bearer' ? t('config.webdav.token_placeholder') : t('config.webdav.password_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.webdav.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.webdav.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabled ? ' on' : '') + '" data-path="webdav.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.webdav.readonly_label') + '</div>';
    html += '<div class="field-help">' + t('help.webdav.read_only') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonly ? ' on' : '') + '" data-path="webdav.readonly" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonly ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.webdav.url_label') + '</div>';
    html += '<div class="field-help">' + t('help.webdav.url') + '</div>';
    html += '<input class="field-input" type="text" data-path="webdav.url" value="' + escapeAttr(data.url || '') + '" placeholder="https://cloud.example.com/remote.php/dav/files/user/">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.webdav.auth_type_label') + '</div>';
    html += '<div class="field-help">' + t('help.webdav.auth_type') + '</div>';
    html += '<select class="field-input" onchange="setNestedValue(configData,\'webdav.auth_type\',this.value);markDirty();renderWebDAVSection(null)">';
    html += '<option value="basic"' + (authType === 'basic' ? ' selected' : '') + '>' + t('config.webdav.auth_type_basic') + '</option>';
    html += '<option value="bearer"' + (authType === 'bearer' ? ' selected' : '') + '>' + t('config.webdav.auth_type_bearer') + '</option>';
    html += '</select>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.webdav.username_label') + '</div>';
    html += '<div class="field-help">' + t('help.webdav.username') + '</div>';
    html += '<input class="field-input" type="text" data-path="webdav.username" value="' + escapeAttr(data.username || '') + '" placeholder=""' + (authType === 'bearer' ? ' disabled style="opacity:0.55;"' : '') + '>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + (authType === 'bearer' ? t('config.webdav.token_label') : t('config.webdav.password_label')) + '</div>';
    html += '<div class="field-help">' + (authType === 'bearer' ? t('help.webdav.token') : t('help.webdav.password')) + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap;">';
        html += '<div class="password-wrap" style="flex:1;min-width:240px;">';
        html += '<input class="field-input" type="password" id="webdav-secret" value="' + escapeAttr(cfgSecretValue(secretValue)) + '" placeholder="' + escapeAttr(placeholder) + '" style="flex:1;min-width:240px;">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;white-space:nowrap;" onclick="webdavSaveSecret()">💾 ' + t('config.webdav.save_vault') + '</button>';
    html += '</div></div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
}

function webdavSaveSecret() {
    const authType = (configData.webdav && configData.webdav.auth_type === 'bearer') ? 'bearer' : 'basic';
    const input = document.getElementById('webdav-secret');
    const secret = input ? input.value.trim() : '';
    if (!secret) {
        showToast(t('config.webdav.secret_empty'), 'error');
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: authType === 'bearer' ? 'webdav_token' : 'webdav_password', value: secret })
    })
        .then(r => r.json())
        .then(res => {
            if (res.status === 'ok' || res.success) {
                showToast(authType === 'bearer' ? t('config.webdav.token_saved') : t('config.webdav.password_saved'), 'success');
                cfgMarkSecretStored(input, authType === 'bearer' ? 'webdav.token' : 'webdav.password');
            } else {
                showToast(res.message || (authType === 'bearer' ? t('config.webdav.token_save_failed') : t('config.webdav.password_save_failed')), 'error');
            }
        })
        .catch(() => showToast(authType === 'bearer' ? t('config.webdav.token_save_failed') : t('config.webdav.password_save_failed'), 'error'));
}
