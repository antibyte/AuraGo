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
        icon: section.icon,
        label: section.label,
        desc: section.desc,
        beforeHTML: '<div id="webdav-status-banner" class="adg-status-banner">' + t('config.webdav.checking') + '</div>',
        fields: [
            form.toggle({ label: t('config.webdav.enabled_label'), help: t('help.webdav.enabled'), path: 'webdav.enabled', value: enabled }),
            form.toggle({ label: t('config.webdav.readonly_label'), help: t('help.webdav.read_only'), path: 'webdav.readonly', value: readonly }),
            form.field({ label: t('config.webdav.url_label'), help: t('help.webdav.url'), path: 'webdav.url', value: data.url || '', placeholder: 'https://cloud.example.com/remote.php/dav/files/user/' }),
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
                value: secretValue,
                placeholder: cfgSecretPlaceholder(secretValue, authType === 'bearer' ? t('config.webdav.token_placeholder') : t('config.webdav.password_placeholder')),
                actionHTML: '<button class="btn-save adg-save-btn" onclick="webdavSaveSecret()">💾 ' + t('config.webdav.save_vault') + '</button>'
            })
        ],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="webdavTestConnection()" id="webdav-test-btn">🔌 ' + t('config.webdav.test_btn') + '</button>' },
            { html: '<span id="webdav-test-result" class="adg-test-result"></span>' }
        ])
    });

    document.getElementById('content').innerHTML = html;

    const usernameInput = document.querySelector('[data-path="webdav.username"]');
    if (usernameInput && authType === 'bearer') {
        usernameInput.disabled = true;
    }

    if (enabled) {
        webdavCheckStatus();
    } else {
        webdavSetBanner('neutral', '⚪ ' + t('config.webdav.status_disabled'));
    }
}

function webdavSetBanner(state, text) {
    const banner = document.getElementById('webdav-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function webdavCheckStatus() {
    webdavSetBanner('neutral', t('config.webdav.checking'));
    fetch('/api/webdav/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                webdavSetBanner('neutral', '⚪ ' + t('config.webdav.status_disabled'));
                return;
            }
            if (res.status === 'no_url') {
                webdavSetBanner('neutral', '⚪ ' + t('config.webdav.status_no_url'));
                return;
            }
            if (res.status === 'no_credentials') {
                webdavSetBanner('warning', '🟡 ' + t('config.webdav.status_no_credentials'));
                return;
            }
            if (res.status === 'ok') {
                webdavSetBanner('success', '🟢 ' + t('config.webdav.status_ok'));
                return;
            }
            webdavSetBanner('danger', '🔴 ' + (res.message || t('config.webdav.connection_failed')));
        })
        .catch(() => webdavSetBanner('danger', '🔴 ' + t('config.webdav.connection_failed')));
}

function webdavTestConnection() {
    const btn = document.getElementById('webdav-test-btn');
    const result = document.getElementById('webdav-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.textContent = t('config.webdav.loading');
        result.className = 'adg-test-result';
    }

    fetch('/api/webdav/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.webdav.status_success') + ' ' + t('config.webdav.test_ok');
                webdavCheckStatus();
            } else {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.webdav.status_error') + ' ' + (res.message || t('config.webdav.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.webdav.status_error') + ' ' + t('config.webdav.test_fail');
            }
        });
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
                webdavCheckStatus();
            } else {
                showToast(res.message || (authType === 'bearer' ? t('config.webdav.token_save_failed') : t('config.webdav.password_save_failed')), 'error');
            }
        })
        .catch(() => showToast(authType === 'bearer' ? t('config.webdav.token_save_failed') : t('config.webdav.password_save_failed'), 'error'));
}