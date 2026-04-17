// cfg/ldap.js — LDAP/Active Directory integration section module

function renderLDAPSection(section) {
    const data = configData['ldap'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const passwordPlaceholder = cfgSecretPlaceholder(data.bind_password, t('config.ldap.password_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
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

    // ── Use TLS toggle ──
    const helpUseTLS = t('help.ldap.use_tls');
    const useTLSOn = data.use_tls !== false;
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ldap.use_tls_label') + '</div>';
    if (helpUseTLS) html += '<div class="field-help">' + helpUseTLS + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (useTLSOn ? ' on' : '') + '" data-path="ldap.use_tls" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (useTLSOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

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
        html += '<div class="password-wrap" style="flex:1;">';
        html += '<input class="field-input ldap-password-input" type="password" id="ldap-password" value="' + escapeAttr(cfgSecretValue(data.bind_password)) + '" placeholder="' + escapeAttr(passwordPlaceholder) + '">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '<button class="btn-save ldap-save-btn" onclick="ldapSavePassword()">' + t('config.ldap.save_icon') + ' ' + t('config.ldap.save_vault') + '</button>';
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
    html += '<button class="btn-save ldap-test-btn" onclick="ldapTestConnection()" id="ldap-test-btn">🔌 ' + t('config.ldap.test_btn') + '</button>';
    html += '<span id="ldap-test-result" class="ldap-test-result"></span>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;
}

function ldapTestConnection() {
    const btn = document.getElementById('ldap-test-btn');
    const result = document.getElementById('ldap-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.textContent = t('config.ldap.loading'); result.className = 'ldap-test-result'; }

    fetch('/api/ldap/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'success') {
                result.className = 'ldap-test-result is-success';
                result.textContent = t('config.ldap.status_success') + ' ' + t('config.ldap.test_ok');
            } else {
                result.className = 'ldap-test-result is-danger';
                result.textContent = t('config.ldap.status_error') + ' ' + (res.message || t('config.ldap.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'ldap-test-result is-danger';
                result.textContent = t('config.ldap.status_error') + ' ' + t('config.ldap.test_fail');
            }
        });
}

function ldapSavePassword() {
    const input = document.getElementById('ldap-password');
    const pw = input ? input.value.trim() : '';
    if (!pw) { showToast(t('config.ldap.password_empty'), 'error'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'ldap_bind_password', value: pw })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            showToast(t('config.ldap.password_saved'), 'success');
            cfgMarkSecretStored(input, 'ldap.bind_password');
        } else {
            showToast(res.message || t('config.ldap.password_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.ldap.password_save_failed'), 'error'));
}
