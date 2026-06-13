// cfg/koofr.js — Koofr integration config panel

function renderKoofrSection(section) {
    const data = configData.koofr || {};
    const enabled = data.enabled === true;
    const readonly = data.readonly === true;
    const form = window.AuraConfigForm;

    const html = form.renderSpec({
        icon: section.icon,
        label: section.label,
        desc: section.desc,
        beforeHTML: '<div id="koofr-status-banner" class="adg-status-banner">' + t('config.koofr.checking') + '</div>',
        fields: [
            form.toggle({ label: t('config.koofr.enabled_label'), help: t('help.koofr.enabled'), path: 'koofr.enabled', value: enabled }),
            form.toggle({ label: t('config.koofr.readonly_label'), help: t('help.koofr.read_only'), path: 'koofr.readonly', value: readonly }),
            form.field({ label: t('config.koofr.base_url_label'), help: t('help.koofr.base_url'), path: 'koofr.base_url', value: data.base_url || 'https://app.koofr.net', placeholder: 'https://app.koofr.net' }),
            form.field({ label: t('config.koofr.username_label'), help: t('help.koofr.username'), path: 'koofr.username', value: data.username || '', placeholder: 'name@example.com' }),
            form.password({
                label: t('config.koofr.app_password_label'),
                help: t('help.koofr.app_password'),
                id: 'koofr-app-password',
                value: data.app_password,
                placeholder: cfgSecretPlaceholder(data.app_password, t('config.koofr.app_password_placeholder')),
                actionHTML: '<button class="btn-save adg-save-btn" onclick="koofrSaveAppPassword()">💾 ' + t('config.koofr.save_vault') + '</button>'
            })
        ],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="koofrTestConnection()" id="koofr-test-btn">🔌 ' + t('config.koofr.test_btn') + '</button>' },
            { html: '<span id="koofr-test-result" class="adg-test-result"></span>' }
        ])
    });

    document.getElementById('content').innerHTML = html;

    if (enabled) {
        koofrCheckStatus();
    } else {
        koofrSetBanner('neutral', '⚪ ' + t('config.koofr.status_disabled'));
    }
}

function koofrSetBanner(state, text) {
    const banner = document.getElementById('koofr-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function koofrCheckStatus() {
    koofrSetBanner('neutral', t('config.koofr.checking'));
    fetch('/api/koofr/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                koofrSetBanner('neutral', '⚪ ' + t('config.koofr.status_disabled'));
                return;
            }
            if (res.status === 'no_credentials') {
                koofrSetBanner('warning', '🟡 ' + t('config.koofr.status_no_credentials'));
                return;
            }
            if (res.status === 'ok') {
                koofrSetBanner('success', '🟢 ' + t('config.koofr.status_ok'));
                return;
            }
            koofrSetBanner('danger', '🔴 ' + (res.message || t('config.koofr.connection_failed')));
        })
        .catch(() => koofrSetBanner('danger', '🔴 ' + t('config.koofr.connection_failed')));
}

function koofrTestConnection() {
    const btn = document.getElementById('koofr-test-btn');
    const result = document.getElementById('koofr-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.textContent = t('config.koofr.loading');
        result.className = 'adg-test-result';
    }

    fetch('/api/koofr/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.koofr.status_success') + ' ' + t('config.koofr.test_ok');
                koofrCheckStatus();
            } else {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.koofr.status_error') + ' ' + (res.message || t('config.koofr.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.koofr.status_error') + ' ' + t('config.koofr.test_fail');
            }
        });
}

function koofrSaveAppPassword() {
    const input = document.getElementById('koofr-app-password');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.koofr.app_password_empty'), 'error');
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'koofr_password', value })
    })
        .then(r => r.json())
        .then(res => {
            if (res.status === 'ok' || res.success) {
                showToast(t('config.koofr.app_password_saved'), 'success');
                cfgMarkSecretStored(input, 'koofr.app_password');
                koofrCheckStatus();
            } else {
                showToast(res.message || t('config.koofr.app_password_save_failed'), 'error');
            }
        })
        .catch(() => showToast(t('config.koofr.app_password_save_failed'), 'error'));
}