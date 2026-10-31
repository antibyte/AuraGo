// cfg/webdav.js — WebDAV integration config panel

let _wbdSection = null;

function renderWebDAVSection(section) {
    if (section) _wbdSection = section; else section = _wbdSection;
    const data = configData.webdav || {};
    const enabled = data.enabled === true;
    const readonly = data.readonly === true;
    const authType = data.auth_type === 'bearer' ? 'bearer' : 'basic';
    const secretValue = authType === 'bearer' ? data.token : data.password;
    const form = window.AuraConfigForm;

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        beforeHTML: form.statusBanner({ id: 'webdav-status-banner', state: enabled ? 'checking' : 'off' }),
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.webdav.enabled_label'), help: t('help.webdav.enabled'), path: 'webdav.enabled', value: enabled }),
            form.toggle({ label: t('config.webdav.readonly_label'), help: t('help.webdav.read_only'), path: 'webdav.readonly', value: readonly }),
            form.toggle({ label: t('config.webdav.show_in_filemanager'), help: t('config.webdav.filemanager_help'), path: 'webdav.show_in_filemanager', value: data.show_in_filemanager === true }),
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.webdav.url_label'), help: t('help.webdav.url'), path: 'webdav.url', value: data.url || '', placeholder: 'https://cloud.example.com/remote.php/dav/files/user/' })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.select({
                label: t('config.webdav.auth_type_label'),
                help: t('help.webdav.auth_type'),
                path: 'webdav.auth_type',
                value: authType,
                onchange: "setNestedValue(configData,'webdav.auth_type',this.value);markDirty();renderWebDAVSection(null)",
                options: [
                    { value: 'basic', label: t('config.webdav.auth_type_basic') },
                    { value: 'bearer', label: t('config.webdav.auth_type_bearer') }
                ]
            }),
            form.field({
                label: t('config.webdav.username_label'),
                help: t('help.webdav.username'),
                path: 'webdav.username',
                value: data.username || '',
                placeholder: '',
                className: authType === 'bearer' ? 'is-disabled-field' : ''
            }),
            form.password({
                label: authType === 'bearer' ? t('config.webdav.token_label') : t('config.webdav.password_label'),
                help: authType === 'bearer' ? t('help.webdav.token') : t('help.webdav.password'),
                id: 'webdav-secret',
                // The data path follows the auth type; switching the type re-renders the section.
                path: authType === 'bearer' ? 'webdav.token' : 'webdav.password',
                value: secretValue,
                placeholder: cfgSecretPlaceholder(secretValue, authType === 'bearer' ? t('config.webdav.token_placeholder') : t('config.webdav.password_placeholder'))
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="webdavTestConnection()" id="webdav-test-btn" data-config-action-draft="true">' + t('config.webdav.test_btn') + '</button>' },
            { html: form.actionResult({ id: 'webdav-test-result' }) }
        ])
    });

    document.getElementById('content').innerHTML = html;

    const usernameInput = document.querySelector('[data-path="webdav.username"]');
    if (usernameInput && authType === 'bearer') {
        usernameInput.disabled = true;
    }

    if (enabled) webdavCheckStatus();
}

function webdavCheckStatus() {
    const banner = document.getElementById('webdav-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/webdav/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'no_url') return { state: 'unconfigured', detail: t('config.webdav.status_no_url') };
        if (res.status === 'no_credentials') return { state: 'unconfigured', detail: t('config.webdav.status_no_credentials') };
        if (res.status === 'ok') return { state: 'connected', detail: t('config.webdav.status_ok') };
        return { state: 'error', detail: res.message || t('config.webdav.connection_failed') };
    });
}

function webdavTestConnection() {
    const btn = document.getElementById('webdav-test-btn');
    return cfgActionResult(btn, () => fetch('/api/webdav/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['webdav']) })
    }), {
        target: 'webdav-test-result',
        success: t('config.webdav.test_ok'),
        failure: t('config.webdav.test_fail'),
        onSuccess: () => webdavCheckStatus()
    });
}
