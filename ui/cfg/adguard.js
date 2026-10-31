// cfg/adguard.js — AdGuard Home integration section module

function renderAdGuardSection(section) {
    const data = configData['adguard'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const passwordPlaceholder = cfgSecretPlaceholder(data.password, t('config.adguard.password_placeholder'));
    const form = window.AuraConfigForm;
    const checking = enabledOn && data.url;

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        beforeHTML: form.statusBanner({ id: 'adg-status-banner', state: checking ? 'checking' : (enabledOn ? 'unconfigured' : 'off'), detail: enabledOn && !data.url ? t('config.adguard.status_no_url') : '' }),
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.adguard.enabled_label'), help: t('help.adguard.enabled'), path: 'adguard.enabled', value: enabledOn }),
            form.toggle({ label: t('config.adguard.readonly_label'), help: t('help.adguard.readonly'), path: 'adguard.readonly', value: readonlyOn })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.adguard.url_label'), help: t('help.adguard.url'), path: 'adguard.url', value: data.url || '', placeholder: 'http://192.168.1.1:3000' })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.field({ label: t('config.adguard.username_label'), help: t('help.adguard.username'), path: 'adguard.username', value: data.username || '', placeholder: 'admin' }),
            form.password({
                label: t('config.adguard.password_label'),
                help: t('help.adguard.password'),
                id: 'adg-password',
                path: 'adguard.password',
                value: data.password,
                placeholder: passwordPlaceholder
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="adgTestConnection()" id="adg-test-btn" data-config-action-draft="true">' + t('config.adguard.test_btn') + '</button>' },
            { html: form.actionResult({ id: 'adg-test-result' }) }
        ]) + '<div id="adg-quick-stats" class="adg-quick-stats is-hidden">'
            + '<div class="adg-stats-title">' + t('config.adguard.stats_title') + '</div>'
            + '<div id="adg-stats-content" class="adg-stats-grid"></div>'
            + '</div>'
    });

    document.getElementById('content').innerHTML = html;

    if (checking) adgCheckStatus();
}

function adgCheckStatus() {
    const banner = document.getElementById('adg-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/adguard/status'), body => {
        if (!body) return null;
        if (body.status === 'disabled') return { state: 'off' };
        if (body.status === 'no_url') return { state: 'unconfigured', detail: t('config.adguard.status_no_url') };
        if (body.status === 'ok' && body.data) {
            const running = body.data.running === true;
            adgLoadQuickStats();
            return {
                state: running ? 'connected' : 'degraded',
                detail: 'AdGuard Home v' + (body.data.version || '?') + ' — ' + (running ? t('config.adguard.running') : t('config.adguard.not_running'))
            };
        }
        if (body.status === 'error' || body.message) return { state: 'error', detail: body.message || t('config.adguard.connection_failed') };
        return null;
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
    return cfgActionResult(btn, () => fetch('/api/adguard/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['adguard']) })
    }), {
        target: 'adg-test-result',
        success: t('config.adguard.test_ok'),
        failure: t('config.adguard.test_fail'),
        onSuccess: () => adgCheckStatus()
    });
}
