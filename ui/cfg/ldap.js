// cfg/ldap.js — LDAP/Active Directory integration section module

function renderLDAPSection(section) {
    const data = configData['ldap'] || {};
    const enabledOn = data.enabled === true;
    const passwordPlaceholder = cfgSecretPlaceholder(data.bind_password, t('config.ldap.password_placeholder'));
    const form = window.AuraConfigForm;
    // Transport selection keeps legacy use_tls configurations readable.
    const tlsMode = data.tls_mode || (data.use_tls === false ? 'plain' : 'ldaps');

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.ldap.enabled_label'), help: t('help.ldap.enabled'), path: 'ldap.enabled', value: enabledOn }),
            form.toggle({ label: t('config.ldap.readonly_label'), help: t('help.ldap.readonly'), path: 'ldap.readonly', value: data.readonly === true })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.ldap.host_label'), help: t('help.ldap.host'), path: 'ldap.host', value: data.host || '', placeholder: 'ldap.example.com' }),
            form.number({ label: t('config.ldap.port_label'), help: t('help.ldap.port'), path: 'ldap.port', value: String(data.port || 636), min: 1, max: 65535, step: 1 }),
            form.select({
                label: t('config.ldap.transport_label'),
                help: t('help.ldap.tls_mode'),
                id: 'ldap-transport',
                path: 'ldap.tls_mode',
                value: tlsMode,
                onchange: 'ldapTransportChanged(this,event)',
                options: [{ value: 'ldaps', label: 'LDAPS' }, { value: 'starttls', label: 'StartTLS' }, { value: 'plain', label: t('config.ldap.plain_label') }]
            }),
            form.toggle({ label: t('config.ldap.insecure_skip_verify_label'), help: t('help.ldap.insecure_skip_verify'), path: 'ldap.insecure_skip_verify', value: data.insecure_skip_verify === true })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.field({ label: t('config.ldap.bind_dn_label'), help: t('help.ldap.bind_dn'), path: 'ldap.bind_dn', value: data.bind_dn || '', placeholder: 'cn=admin,dc=example,dc=com' }),
            // With data-path the password joins the save dock (the server writes it to the vault).
            form.password({
                label: t('config.ldap.bind_password_label'),
                help: t('help.ldap.bind_password'),
                id: 'ldap-password',
                path: 'ldap.bind_password',
                value: data.bind_password,
                placeholder: passwordPlaceholder
            })
        ] }, { titleKey: 'config.refresh.settings', fields: [
            form.field({ label: t('config.ldap.base_dn_label'), help: t('help.ldap.base_dn'), path: 'ldap.base_dn', value: data.base_dn || '', placeholder: 'dc=example,dc=com' }),
            form.field({ label: t('config.ldap.user_search_base_label'), help: t('help.ldap.user_search_base'), path: 'ldap.user_search_base', value: data.user_search_base || '', placeholder: 'ou=users,dc=example,dc=com' }),
            form.field({ label: t('config.ldap.group_search_base_label'), help: t('help.ldap.group_search_base'), path: 'ldap.group_search_base', value: data.group_search_base || '', placeholder: 'ou=groups,dc=example,dc=com' })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save ldap-test-btn" onclick="ldapTestConnection()" id="ldap-test-btn" data-config-action-draft="true">' + cfgIcon('plug') + '<span>' + t('config.ldap.test_btn') + '</span></button>' },
            { html: form.actionResult({ id: 'ldap-test-result' }) }
        ])
    });

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
