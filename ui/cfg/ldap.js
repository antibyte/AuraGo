// cfg/ldap.js — LDAP/Active Directory integration section module

function renderLDAPSection(section) {
    const data = configData['ldap'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const passwordPlaceholder = cfgSecretPlaceholder(data.bind_password, t('config.ldap.password_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    const helpEnabled = t('help.ldap.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="ldap.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    const helpReadonly = t('help.ldap.readonly');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.readonly_label') + '</div>';
    if (helpReadonly) html += '<div class="field-help">' + helpReadonly + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="ldap.readonly" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Host ──
    const helpHost = t('help.ldap.host');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.host_label') + '</div>';
    if (helpHost) html += '<div class="field-help">' + helpHost + '</div>';
    html += '<input class="field-input" type="text" data-path="ldap.host" value="' + escapeAttr(data.host || '') + '" placeholder="ldap.example.com">';
    html += '</div>';

    // ── Port ──
    const helpPort = t('help.ldap.port');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.port_label') + '</div>';
    if (helpPort) html += '<div class="field-help">' + helpPort + '</div>';
    html += '<input class="field-input" type="number" data-path="ldap.port" value="' + (data.port || 636) + '" placeholder="636">';
    html += '</div>';

    // Transport selection keeps legacy use_tls configurations readable.
    const tlsMode = data.tls_mode || (data.use_tls === false ? 'plain' : 'ldaps');
    html += '<div class="field-group">';
    html += '<label class="field-label" for="ldap-transport">' + t('config.ldap.transport_label') + '</label>';
    html += '<div class="field-help">' + t('help.ldap.tls_mode') + '</div>';
    html += '<select id="ldap-transport" class="field-select" data-path="ldap.tls_mode" onchange="ldapTransportChanged(this,event)">';
    [['ldaps', 'LDAPS'], ['starttls', 'StartTLS'], ['plain', t('config.ldap.plain_label')]].forEach(([value, label]) => {
        html += '<option value="' + value + '"' + (tlsMode === value ? ' selected' : '') + '>' + escapeHtml(label) + '</option>';
    });
    html += '</select></div>';

    // ── Insecure Skip Verify toggle ──
    const helpInsecure = t('help.ldap.insecure_skip_verify');
    const insecureOn = data.insecure_skip_verify === true;
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.insecure_skip_verify_label') + '</div>';
    if (helpInsecure) html += '<div class="field-help">' + helpInsecure + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (insecureOn ? ' on' : '') + '" data-path="ldap.insecure_skip_verify" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (insecureOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Base DN ──
    const helpBaseDN = t('help.ldap.base_dn');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.base_dn_label') + '</div>';
    if (helpBaseDN) html += '<div class="field-help">' + helpBaseDN + '</div>';
    html += '<input class="field-input" type="text" data-path="ldap.base_dn" value="' + escapeAttr(data.base_dn || '') + '" placeholder="dc=example,dc=com">';
    html += '</div>';

    // ── Bind DN ──
    const helpBindDN = t('help.ldap.bind_dn');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.bind_dn_label') + '</div>';
    if (helpBindDN) html += '<div class="field-help">' + helpBindDN + '</div>';
    html += '<input class="field-input" type="text" data-path="ldap.bind_dn" value="' + escapeAttr(data.bind_dn || '') + '" placeholder="cn=admin,dc=example,dc=com">';
    html += '</div>';

    // ── Bind Password (vault) ──
    const helpPass = t('help.ldap.bind_password');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.bind_password_label') + '</div>';
    if (helpPass) html += '<div class="field-help">' + helpPass + '</div>';
    html += '<div class="ldap-password-row">';
    // With data-path the password joins the save dock (the server writes it to the vault).
    html += '<div class="password-wrap pw-u-flex-1">';
    html += '<input class="field-input ldap-password-input" type="password" id="ldap-password" data-path="ldap.bind_password" value="' + escapeAttr(cfgSecretValue(data.bind_password)) + '" placeholder="' + escapeAttr(passwordPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '</div></div>';

    // ── User Search Base ──
    const helpUserSearch = t('help.ldap.user_search_base');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.user_search_base_label') + '</div>';
    if (helpUserSearch) html += '<div class="field-help">' + helpUserSearch + '</div>';
    html += '<input class="field-input" type="text" data-path="ldap.user_search_base" value="' + escapeAttr(data.user_search_base || '') + '" placeholder="ou=users,dc=example,dc=com">';
    html += '</div>';

    // ── Group Search Base ──
    const helpGroupSearch = t('help.ldap.group_search_base');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.group_search_base_label') + '</div>';
    if (helpGroupSearch) html += '<div class="field-help">' + helpGroupSearch + '</div>';
    html += '<input class="field-input" type="text" data-path="ldap.group_search_base" value="' + escapeAttr(data.group_search_base || '') + '" placeholder="ou=groups,dc=example,dc=com">';
    html += '</div>';

    // ── Test connection button ──
    html += '<div class="field-group">';
    html += '<button class="btn-save ldap-test-btn" onclick="ldapTestConnection()" id="ldap-test-btn" data-config-action-draft="true">🔌 ' + t('config.ldap.test_btn') + '</button>';
    html += cfgActionResult.html('ldap-test-result');
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;
}

function ldapTransportChanged(select, event) {
    const port = document.querySelector('[data-path="ldap.port"]');
    if (port && (port.value === '389' || port.value === '636')) {
        port.value = select.value === 'ldaps' ? '636' : '389';
    }
    markDirty(event);
}

// Tests the values typed in (unsaved included); nothing is saved by the test.
function ldapTestConnection() {
    return cfgActionResult(document.getElementById('ldap-test-btn'), () => fetch('/api/ldap/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['ldap']) })
    }), {
        target: 'ldap-test-result',
        pending: t('config.ldap.loading'),
        success: t('config.ldap.test_ok'),
        failure: t('config.ldap.test_fail')
    });
}
