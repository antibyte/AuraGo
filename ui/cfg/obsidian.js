// cfg/obsidian.js — Obsidian integration section module

function renderObsidianSection(section) {
    const data = configData['obsidian'] || {};
    const enabledOn = data.enabled === true;
    const form = window.AuraConfigForm;
    const setupHint = t('help.obsidian.setup_hint');

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        key: 'obsidian',
        banner: { id: 'obsidian-status-banner', state: enabledOn ? 'checking' : 'off' },
        // The hint carries markup (<strong>), so it is inserted as HTML like the other help texts.
        beforeHTML: setupHint ? '<div class="field-group"><div class="field-help">' + setupHint + '</div></div>' : '',
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.obsidian.enabled_label'), help: t('help.obsidian.enabled'), path: 'obsidian.enabled', value: enabledOn }),
            form.toggle({ label: t('config.obsidian.readonly_label'), help: t('help.obsidian.readonly'), path: 'obsidian.readonly', value: data.readonly === true || data.read_only === true }),
            form.toggle({ label: t('config.obsidian.destructive_label'), help: t('help.obsidian.allow_destructive'), path: 'obsidian.allow_destructive', value: data.allow_destructive === true })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.obsidian.host_label'), help: t('help.obsidian.host'), path: 'obsidian.host', value: data.host || '', placeholder: '127.0.0.1' }),
            form.number({ label: t('config.obsidian.port_label'), help: t('help.obsidian.port'), path: 'obsidian.port', value: data.port || '27124', min: 1, max: 65535 }),
            form.toggle({ label: t('config.obsidian.https_label'), help: t('help.obsidian.use_https'), path: 'obsidian.use_https', value: data.use_https !== false }),
            form.toggle({ label: t('config.obsidian.insecure_label'), help: t('help.obsidian.insecure_ssl'), path: 'obsidian.insecure_ssl', value: data.insecure_ssl !== false }),
            form.number({ label: t('config.obsidian.connect_timeout_label'), help: t('help.obsidian.connect_timeout'), path: 'obsidian.connect_timeout', value: data.connect_timeout || '10', min: 0 }),
            form.number({ label: t('config.obsidian.request_timeout_label'), help: t('help.obsidian.request_timeout'), path: 'obsidian.request_timeout', value: data.request_timeout || '30', min: 0 })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.password({
                label: t('config.obsidian.apikey_label'),
                help: t('help.obsidian.api_key'),
                id: 'obsidian-apikey',
                path: 'obsidian.api_key',
                value: data.api_key,
                placeholder: cfgSecretPlaceholder(data.api_key, t('config.obsidian.apikey_placeholder'))
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save cfg-test-btn" onclick="obsidianTestConnection()" id="obsidian-test-btn" data-config-action-draft="true">' + escapeHtml(t('config.obsidian.test_btn')) + '</button>' },
            { html: form.actionResult({ id: 'obsidian-test-result' }) }
        ])
    });

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (enabledOn) obsidianCheckStatus();
}

function obsidianCheckStatus() {
    const banner = document.getElementById('obsidian-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/obsidian/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'error' || res.status === 'offline') return { state: 'error', detail: res.error || t('config.obsidian.connection_failed') };
        if (res.status === 'online') return { state: 'connected', detail: t('config.obsidian.status_online').replace('{api}', res.api_version || '').replace('{obsidian}', res.obsidian_version || '') };
        if (res.status === 'no_url' || res.status === 'no_host') return { state: 'unconfigured', detail: t('config.obsidian.status_no_url') };
        return null;
    });
}

// Tests the values typed in (not yet saved); the server does not persist them.
function obsidianTestConnection() {
    const btn = document.getElementById('obsidian-test-btn');
    return cfgActionResult(btn, () => fetch('/api/obsidian/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['obsidian']) })
    }), {
        target: 'obsidian-test-result',
        success: t('config.obsidian.test_ok'),
        failure: t('config.obsidian.test_fail'),
        onSuccess: () => obsidianCheckStatus()
    });
}
