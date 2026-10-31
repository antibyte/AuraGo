// cfg/paperless.js — Paperless-ngx integration section module

function renderPaperlessSection(section) {
    const data = configData['paperless_ngx'] || {};
    const form = window.AuraConfigForm;

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        key: 'paperless_ngx',
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.paperless.enabled_label'), help: t('help.paperless_ngx.enabled'), path: 'paperless_ngx.enabled', value: data.enabled === true }),
            form.toggle({ label: t('config.paperless.readonly_label'), help: t('help.paperless_ngx.read_only'), path: 'paperless_ngx.readonly', value: data.readonly === true })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ type: 'url', label: t('config.paperless.url_label'), help: t('help.paperless_ngx.url'), path: 'paperless_ngx.url', value: data.url || '', placeholder: 'https://paperless.example.com' })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            // With data-path the token joins the save dock (the server writes it to the vault).
            form.password({
                label: t('config.paperless.token_label'),
                help: t('help.paperless_ngx.api_token'),
                id: 'paperless-token-input',
                path: 'paperless_ngx.api_token',
                value: data.api_token,
                placeholder: cfgSecretPlaceholder(data.api_token, '••••••••')
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button id="paperless-test-btn" class="btn-save cfg-test-btn" onclick="paperlessTestConnection()" data-config-action-draft="true">' + cfgIcon('plug') + '<span>' + escapeHtml(t('config.paperless.test_connection')) + '</span></button>' },
            { html: form.actionResult({ id: 'paperless-test-status' }) }
        ])
    });

    document.getElementById('content').innerHTML = html;
}

// Tests the values typed in (unsaved included); nothing is saved by the test.
function paperlessTestConnection() {
    return cfgActionResult(document.getElementById('paperless-test-btn'), () => fetch('/api/paperless/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['paperless_ngx']) })
    }), {
        target: 'paperless-test-status',
        pending: t('config.paperless.testing'),
        success: outcome => outcome.reason || t('config.paperless.test_success'),
        failure: t('config.paperless.test_failed')
    });
}
