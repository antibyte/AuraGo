// cfg/adguard.js — AdGuard Home integration section module

function renderAdGuardSection(section) {
    const data = configData['adguard'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const passwordPlaceholder = cfgSecretPlaceholder(data.password, t('config.adguard.password_placeholder'));
    const form = window.AuraConfigForm;

    const html = form.renderSpec({
        icon: section.icon,
        label: section.label,
        desc: section.desc,
        beforeHTML: '<div id="adg-status-banner" class="adg-status-banner">' + t('config.adguard.checking') + '</div>',
        fields: [
            form.toggle({ label: t('config.adguard.enabled_label'), help: t('help.adguard.enabled'), path: 'adguard.enabled', value: enabledOn }),
            form.toggle({ label: t('config.adguard.readonly_label'), help: t('help.adguard.readonly'), path: 'adguard.readonly', value: readonlyOn }),
            form.field({ label: t('config.adguard.url_label'), help: t('help.adguard.url'), path: 'adguard.url', value: data.url || '', placeholder: 'http://192.168.1.1:3000' }),
            form.field({ label: t('config.adguard.username_label'), help: t('help.adguard.username'), path: 'adguard.username', value: data.username || '', placeholder: 'admin' }),
            form.password({
                label: t('config.adguard.password_label'),
                help: t('help.adguard.password'),
                id: 'adg-password',
                value: data.password,
                placeholder: passwordPlaceholder,
                actionHTML: '<button class="btn-save adg-save-btn" onclick="adgSavePassword()">' + t('config.adguard.save_icon') + ' ' + t('config.adguard.save_vault') + '</button>'
            })
        ],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="adgTestConnection()" id="adg-test-btn">🔌 ' + t('config.adguard.test_btn') + '</button>' },
            { html: '<span id="adg-test-result" class="adg-test-result"></span>' }
        ]) + '<div id="adg-quick-stats" class="adg-quick-stats is-hidden">'
            + '<div class="adg-stats-title">📊 ' + t('config.adguard.stats_title') + '</div>'
            + '<div id="adg-stats-content" class="adg-stats-grid"></div>'
            + '</div>'
    });

    document.getElementById('content').innerHTML = html;

    if (enabledOn && data.url) {
        adgCheckStatus();
    }
}

function adgSetBannerState(banner, state, text) {
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function adgCheckStatus() {
    const banner = document.getElementById('adg-status-banner');
    if (!banner) return;
    adgSetBannerState(banner, 'neutral', t('config.adguard.checking'));

    fetch('/api/adguard/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                adgSetBannerState(banner, 'neutral', '⚪ ' + t('config.adguard.status_disabled'));
                return;
            }
            if (res.status === 'no_url') {
                adgSetBannerState(banner, 'neutral', '⚪ ' + t('config.adguard.status_no_url'));
                return;
            }
            if (res.status === 'ok' && res.data) {
                const d = res.data;
                const version = d.version || '?';
                const running = d.running === true;
                adgSetBannerState(
                    banner,
                    running ? 'success' : 'warning',
                    (running ? '🟢' : '🟡') + ' AdGuard Home v' + version + ' — ' + (running ? t('config.adguard.running') : t('config.adguard.not_running'))
                );
                adgLoadQuickStats();
                return;
            }
            adgSetBannerState(banner, 'danger', '🔴 ' + (res.message || t('config.adguard.connection_failed')));
        })
        .catch(() => {
            adgSetBannerState(banner, 'danger', '🔴 ' + t('config.adguard.connection_failed'));
        });
}

function adgLoadQuickStats() {
    const wrap = document.getElementById('adg-quick-stats');
    const content = document.getElementById('adg-stats-content');
    if (!wrap || !content) return;
    wrap.classList.remove('is-hidden');
    content.innerHTML = '<div class="adg-stats-loaded">' + t('config.adguard.stats_loaded') + '</div>';
}

function adgTestConnection() {
    const btn = document.getElementById('adg-test-btn');
    const result = document.getElementById('adg-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.textContent = t('config.adguard.loading'); result.className = 'adg-test-result'; }

    fetch('/api/adguard/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.adguard.status_success') + ' ' + t('config.adguard.test_ok');
                adgCheckStatus();
            } else {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.adguard.status_error') + ' ' + (res.message || t('config.adguard.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.adguard.status_error') + ' ' + t('config.adguard.test_fail');
            }
        });
}

function adgSavePassword() {
    const input = document.getElementById('adg-password');
    const pw = input ? input.value.trim() : '';
    if (!pw) { showToast(t('config.adguard.password_empty'), 'error'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'adguard_password', value: pw })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            showToast(t('config.adguard.password_saved'), 'success');
            cfgMarkSecretStored(input, 'adguard.password');
        } else {
            showToast(res.message || t('config.adguard.password_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.adguard.password_save_failed'), 'error'));
}