// cfg/koofr.js — Koofr integration config panel

function renderKoofrSection(section) {
    const data = configData.koofr || {};
    const enabled = data.enabled === true;
    const readonly = data.readonly === true;
    const form = window.AuraConfigForm;
    const startState = !enabled ? 'off' : 'checking';

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        beforeHTML: form.statusBanner({ id: 'koofr-status-banner', state: startState }),
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.koofr.enabled_label'), help: t('help.koofr.enabled'), path: 'koofr.enabled', value: enabled }),
            form.toggle({ label: t('config.koofr.readonly_label'), help: t('help.koofr.read_only'), path: 'koofr.readonly', value: readonly }),
            form.toggle({ label: t('config.koofr.show_in_filemanager'), help: t('config.koofr.filemanager_help'), path: 'koofr.show_in_filemanager', value: data.show_in_filemanager === true }),
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.koofr.base_url_label'), help: t('help.koofr.base_url'), path: 'koofr.base_url', value: data.base_url || 'https://app.koofr.net', placeholder: 'https://app.koofr.net' })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.field({ label: t('config.koofr.username_label'), help: t('help.koofr.username'), path: 'koofr.username', value: data.username || '', placeholder: 'name@example.com' }),
            form.password({
                label: t('config.koofr.app_password_label'),
                help: t('help.koofr.app_password'),
                id: 'koofr-app-password',
                path: 'koofr.app_password',
                value: data.app_password,
                placeholder: cfgSecretPlaceholder(data.app_password, t('config.koofr.app_password_placeholder'))
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="koofrTestConnection()" id="koofr-test-btn" data-config-action-draft="true">' + t('config.koofr.test_btn') + '</button>' },
            { html: form.actionResult({ id: 'koofr-test-result' }) }
        ])
    });

    document.getElementById('content').innerHTML = html;

    if (enabled) koofrCheckStatus();
}

function koofrCheckStatus() {
    const banner = document.getElementById('koofr-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/koofr/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'no_credentials') return { state: 'unconfigured', detail: t('config.koofr.status_no_credentials') };
        if (res.status === 'ok') return { state: 'connected', detail: t('config.koofr.status_ok') };
        return { state: 'error', detail: res.message || t('config.koofr.connection_failed') };
    });
}

function koofrTestConnection() {
    const btn = document.getElementById('koofr-test-btn');
    return cfgActionResult(btn, () => fetch('/api/koofr/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['koofr']) })
    }), {
        target: 'koofr-test-result',
        success: t('config.koofr.test_ok'),
        failure: t('config.koofr.test_fail'),
        onSuccess: () => koofrCheckStatus()
    });
}
