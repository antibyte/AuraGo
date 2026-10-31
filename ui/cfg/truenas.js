// cfg/truenas.js — TrueNAS integration section module

function renderTrueNASSection(section) {
    const data = configData['truenas'] || {};
    const enabledOn = data.enabled === true;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.truenas.apikey_placeholder'));
    const form = window.AuraConfigForm;
    const checking = enabledOn && data.host;

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        banner: { id: 'truenas-status-banner', state: checking ? 'checking' : (enabledOn ? 'unconfigured' : 'off'), detail: enabledOn && !data.host ? t('config.truenas.status_no_url') : '' },
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.truenas.enabled_label'), help: t('help.truenas.enabled'), path: 'truenas.enabled', value: enabledOn }),
            form.toggle({ label: t('config.truenas.readonly_label'), help: t('help.truenas.readonly'), path: 'truenas.readonly', value: data.readonly === true }),
            form.toggle({ label: t('config.truenas.destructive_label'), help: t('help.truenas.allow_destructive'), path: 'truenas.allow_destructive', value: data.allow_destructive === true })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.truenas.host_label'), help: t('help.truenas.host'), path: 'truenas.host', value: data.host || '', placeholder: 'truenas.local' }),
            form.toggle({ label: t('config.truenas.https_label'), help: t('help.truenas.use_https'), path: 'truenas.use_https', value: data.use_https !== false }),
            form.toggle({ label: t('config.truenas.insecure_label'), help: t('help.truenas.insecure_ssl'), path: 'truenas.insecure_ssl', value: data.insecure_ssl === true })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.password({
                label: t('config.truenas.apikey_label'),
                help: t('help.truenas.api_key'),
                id: 'truenas-apikey',
                path: 'truenas.api_key',
                value: data.api_key,
                placeholder: apiKeyPlaceholder
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save cfg-test-btn" onclick="truenasTestConnection()" id="truenas-test-btn" data-config-action-draft="true">' + t('config.truenas.test_btn') + '</button>' },
            { html: form.actionResult({ id: 'truenas-test-result' }) },
            { html: '<a href="/truenas" class="btn-secondary pw-u-button-link">' + cfgIcon('chart') + '<span>' + t('config.truenas.open_ui') + '</span></a>' }
        ])
    });

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (checking) truenasCheckStatus();
}

function truenasCheckStatus() {
    const banner = document.getElementById('truenas-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/truenas/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'error' || res.status === 'offline') return { state: 'error', detail: res.error || t('config.truenas.connection_failed') };
        if (res.status === 'online') return { state: 'connected', detail: 'TrueNAS ' + (res.version || '') + ' — ' + (res.hostname || '') };
        if (res.status === 'no_url' || res.status === 'no_host') return { state: 'unconfigured', detail: t('config.truenas.status_no_url') };
        return null;
    });
}

// Tests the values typed in (not yet saved); the server does not persist them.
function truenasTestConnection() {
    const btn = document.getElementById('truenas-test-btn');
    return cfgActionResult(btn, () => fetch('/api/truenas/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['truenas']) })
    }), {
        target: 'truenas-test-result',
        success: t('config.truenas.test_ok'),
        failure: t('config.truenas.test_fail'),
        onSuccess: () => truenasCheckStatus()
    });
}
