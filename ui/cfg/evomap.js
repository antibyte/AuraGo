// cfg/evomap.js - EvoMap optional external AI integration

function evomapConfig() {
    configData.evomap = configData.evomap || {};
    const data = configData.evomap;
    if (data.readonly === undefined) data.readonly = true;
    if (!data.base_url) data.base_url = 'https://evomap.ai';
    if (!data.request_timeout_seconds) data.request_timeout_seconds = 30;
    if (!data.max_result_bytes) data.max_result_bytes = 262144;
    return data;
}

function renderEvomapSection(section) {
    const data = evomapConfig();
    const form = window.AuraConfigForm;
    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        beforeHTML: form.statusBanner({ id: 'evomap-status-banner', state: data.enabled === true ? 'checking' : 'off' }),
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.evomap.enabled_label'), help: t('help.evomap.enabled'), path: 'evomap.enabled', value: data.enabled === true }),
            form.toggle({ label: t('config.evomap.readonly_label'), help: t('help.evomap.readonly'), path: 'evomap.readonly', value: data.readonly !== false }),
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.evomap.base_url_label'), help: t('help.evomap.base_url'), path: 'evomap.base_url', value: data.base_url || 'https://evomap.ai', placeholder: 'https://evomap.ai' }),
            form.field({ label: t('config.evomap.node_id_label'), help: t('help.evomap.node_id'), path: 'evomap.node_id', value: data.node_id || '', placeholder: t('config.evomap.node_id_placeholder') }),
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.password({
                label: t('config.evomap.api_key_label'),
                help: t('help.evomap.api_key'),
                id: 'evomap-api-key',
                path: 'evomap.api_key',
                value: data.api_key,
                placeholder: cfgSecretPlaceholder(data.api_key, t('config.evomap.api_key_placeholder'))
            }),
            form.password({
                label: t('config.evomap.node_secret_label'),
                help: t('help.evomap.node_secret'),
                id: 'evomap-node-secret',
                path: 'evomap.node_secret',
                value: data.node_secret,
                placeholder: cfgSecretPlaceholder(data.node_secret, t('config.evomap.node_secret_placeholder'))
            }),
        ] }, { titleKey: 'config.refresh.runtime', fields: [
            form.toggle({ label: t('config.evomap.kg_enabled'), help: t('help.evomap.kg_enabled'), path: 'evomap.kg_enabled', value: data.kg_enabled === true }),
            form.number({ label: t('config.evomap.timeout_label'), help: t('help.evomap.timeout'), path: 'evomap.request_timeout_seconds', value: String(data.request_timeout_seconds || 30), min: 1, step: 1 }),
            form.number({ label: t('config.evomap.max_result_label'), help: t('help.evomap.max_result'), path: 'evomap.max_result_bytes', value: String(data.max_result_bytes || 262144), min: 1024, step: 1024 })
        ] }],
        afterHTML: form.actions([
            { html: '<button type="button" class="btn-save adg-test-btn" id="evomap-test-btn" data-config-action-draft="true" onclick="evomapTestConnection()">' + t('config.evomap.test_btn') + '</button>' },
            { html: '<button type="button" class="btn-save adg-test-btn" id="evomap-register-btn" data-config-action-draft="false" onclick="evomapRegisterNode()">' + t('config.evomap.register_btn') + '</button>' },
            { html: form.actionResult({ id: 'evomap-test-result' }) }
        ]) + '<div id="evomap-secret-state" class="adg-quick-stats is-hidden"></div>'
    });

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    evomapRefreshStatus();
}

function evomapRenderSecretState(data) {
    const el = document.getElementById('evomap-secret-state');
    if (!el) return;
    el.classList.remove('is-hidden');
    const nodeSecret = data && data.node_secret_configured ? t('config.evomap.node_secret_existing') : t('config.evomap.node_secret_missing');
    const apiKey = data && data.api_key_configured ? t('config.evomap.api_key_existing') : t('config.evomap.api_key_missing');
    el.innerHTML = '<div class="adg-stats-title">' + t('config.evomap.vault_status') + '</div>' +
        '<div class="adg-stats-grid">' +
        '<div><strong>' + t('config.evomap.node_secret_label') + '</strong><br>' + escapeHtml(nodeSecret) + '</div>' +
        '<div><strong>' + t('config.evomap.api_key_label') + '</strong><br>' + escapeHtml(apiKey) + '</div>' +
        '</div>';
}

async function evomapRefreshStatus() {
    const banner = document.getElementById('evomap-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/evomap/status'), data => {
        if (!data) return null;
        const status = data.status || 'error';
        const cfg = evomapConfig();
        cfg.node_secret = data.node_secret_configured ? cfgMaskedSecretFallback : '';
        cfg.api_key = data.api_key_configured ? cfgMaskedSecretFallback : '';
        evomapRenderSecretState(data);
        if (status === 'ready') return { state: 'connected', detail: t('config.evomap.status_ready') };
        if (status === 'missing_node_secret') return { state: 'unconfigured', detail: t('config.evomap.status_missing_node_secret') };
        if (status === 'disabled') return { state: 'off' };
        return { state: 'error', detail: data.message || t('config.evomap.status_error') };
    });
}

function evomapTestConnection() {
    const btn = document.getElementById('evomap-test-btn');
    return cfgActionResult(btn, () => fetch('/api/evomap/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['evomap']) })
    }), {
        target: 'evomap-test-result',
        pending: t('config.evomap.testing'),
        success: t('config.evomap.test_ok'),
        failure: t('config.evomap.test_fail'),
        lock: '#evomap-register-btn',
        onSuccess: () => evomapRefreshStatus()
    });
}

// Registers the node with the saved configuration (the server reads the stored values).
function evomapRegisterNode() {
    const btn = document.getElementById('evomap-register-btn');
    return cfgActionResult(btn, () => fetch('/api/evomap/register', { method: 'POST' }), {
        target: 'evomap-test-result',
        pending: t('config.evomap.registering'),
        success: t('config.evomap.register_ok'),
        failure: t('config.evomap.register_fail'),
        lock: '#evomap-test-btn',
        onSuccess: outcome => {
            const data = outcome.body || {};
            if (data.node_id) {
                setNestedValue(configData, 'evomap.node_id', data.node_id);
                const input = document.querySelector('[data-path="evomap.node_id"]');
                if (input) input.value = data.node_id;
            }
            cfgMarkSecretStored(document.getElementById('evomap-node-secret'), 'evomap.node_secret');
            evomapRefreshStatus();
        }
    });
}
