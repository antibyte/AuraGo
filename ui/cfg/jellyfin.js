// cfg/jellyfin.js — Jellyfin integration section module

function renderJellyfinSection(section) {
    const data = configData['jellyfin'] || {};
    const enabledOn = data.enabled === true;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.jellyfin.apikey_placeholder'));
    const form = window.AuraConfigForm;
    const checking = enabledOn && data.host;

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        banner: { id: 'jellyfin-status-banner', state: checking ? 'checking' : (enabledOn ? 'unconfigured' : 'off'), detail: enabledOn && !data.host ? t('config.jellyfin.status_no_url') : '' },
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.jellyfin.enabled_label'), help: t('help.jellyfin.enabled'), path: 'jellyfin.enabled', value: enabledOn }),
            form.toggle({ label: t('config.jellyfin.readonly_label'), help: t('help.jellyfin.readonly'), path: 'jellyfin.readonly', value: data.readonly === true || data.read_only === true }),
            form.toggle({ label: t('config.jellyfin.destructive_label'), help: t('help.jellyfin.allow_destructive'), path: 'jellyfin.allow_destructive', value: data.allow_destructive === true })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.jellyfin.host_label'), help: t('help.jellyfin.host'), path: 'jellyfin.host', value: data.host || '', placeholder: 'jellyfin.local' }),
            form.number({ label: t('config.jellyfin.port_label'), help: t('help.jellyfin.port'), path: 'jellyfin.port', value: String(data.port || '8096'), min: 1, max: 65535, step: 1 }),
            form.toggle({ label: t('config.jellyfin.https_label'), help: t('help.jellyfin.use_https'), path: 'jellyfin.use_https', value: data.use_https === true }),
            form.toggle({ label: t('config.jellyfin.insecure_label'), help: t('help.jellyfin.insecure_ssl'), path: 'jellyfin.insecure_ssl', value: data.insecure_ssl === true })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.password({
                label: t('config.jellyfin.apikey_label'),
                help: t('help.jellyfin.api_key'),
                id: 'jellyfin-apikey',
                path: 'jellyfin.api_key',
                value: data.api_key,
                placeholder: apiKeyPlaceholder
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save cfg-test-btn" onclick="jellyfinTestConnection()" id="jellyfin-test-btn" data-config-action-draft="true">' + t('config.jellyfin.test_btn') + '</button>' },
            { html: form.actionResult({ id: 'jellyfin-test-result' }) }
        ])
    });

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (checking) jellyfinCheckStatus();
}

function jellyfinCheckStatus() {
    const banner = document.getElementById('jellyfin-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/jellyfin/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'error' || res.status === 'offline') return { state: 'error', detail: res.error || t('config.jellyfin.connection_failed') };
        if (res.status === 'online') return { state: 'connected', detail: t('config.jellyfin.product_name') + ' ' + (res.version || '') + ' — ' + (res.server_name || '') };
        if (res.status === 'no_url' || res.status === 'no_host') return { state: 'unconfigured', detail: t('config.jellyfin.status_no_url') };
        return null;
    });
}

// Tests the values typed in (not yet saved); the server does not persist them.
function jellyfinTestConnection() {
    const btn = document.getElementById('jellyfin-test-btn');
    return cfgActionResult(btn, () => fetch('/api/jellyfin/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['jellyfin']) })
    }), {
        target: 'jellyfin-test-result',
        success: t('config.jellyfin.test_ok'),
        failure: t('config.jellyfin.test_fail'),
        onSuccess: () => jellyfinCheckStatus()
    });
}
