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
        icon: section.icon,
        label: section.label,
        desc: section.desc,
        beforeHTML: '<div id="evomap-status-banner" class="adg-status-banner">' + t('config.evomap.checking') + '</div>',
        fields: [
            form.toggle({ label: t('config.evomap.enabled_label'), help: t('help.evomap.enabled'), path: 'evomap.enabled', value: data.enabled === true }),
            form.toggle({ label: t('config.evomap.readonly_label'), help: t('help.evomap.readonly'), path: 'evomap.readonly', value: data.readonly !== false }),
            form.field({ label: t('config.evomap.base_url_label'), help: t('help.evomap.base_url'), path: 'evomap.base_url', value: data.base_url || 'https://evomap.ai', placeholder: 'https://evomap.ai' }),
            form.field({ label: t('config.evomap.node_id_label'), help: t('help.evomap.node_id'), path: 'evomap.node_id', value: data.node_id || '', placeholder: t('config.evomap.node_id_placeholder') }),
            form.password({
                label: t('config.evomap.api_key_label'),
                help: t('help.evomap.api_key'),
                id: 'evomap-api-key',
                value: data.api_key,
                placeholder: cfgSecretPlaceholder(data.api_key, t('config.evomap.api_key_placeholder')),
                actionHTML: '<button type="button" class="btn-save adg-save-btn" onclick="evomapSaveSecret(\'evomap_api_key\', \'evomap.api_key\', \'evomap-api-key\')">' + t('config.evomap.save_vault') + '</button>'
            }),
            form.password({
                label: t('config.evomap.node_secret_label'),
                help: t('help.evomap.node_secret'),
                id: 'evomap-node-secret',
                value: data.node_secret,
                placeholder: cfgSecretPlaceholder(data.node_secret, t('config.evomap.node_secret_placeholder')),
                actionHTML: '<button type="button" class="btn-save adg-save-btn" onclick="evomapSaveSecret(\'evomap_node_secret\', \'evomap.node_secret\', \'evomap-node-secret\')">' + t('config.evomap.save_vault') + '</button>'
            }),
            form.toggle({ label: t('config.evomap.kg_enabled'), help: t('help.evomap.kg_enabled'), path: 'evomap.kg_enabled', value: data.kg_enabled === true }),
            form.number({ label: t('config.evomap.timeout_label'), help: t('help.evomap.timeout'), path: 'evomap.request_timeout_seconds', value: String(data.request_timeout_seconds || 30), min: 1, step: 1 }),
            form.number({ label: t('config.evomap.max_result_label'), help: t('help.evomap.max_result'), path: 'evomap.max_result_bytes', value: String(data.max_result_bytes || 262144), min: 1024, step: 1024 })
        ],
        afterHTML: form.actions([
            { html: '<button type="button" class="btn-save adg-test-btn" id="evomap-test-btn" onclick="evomapTestConnection()">' + t('config.evomap.test_btn') + '</button>' },
            { html: '<button type="button" class="btn-save adg-test-btn" id="evomap-register-btn" onclick="evomapRegisterNode()">' + t('config.evomap.register_btn') + '</button>' },
            { html: '<span id="evomap-test-result" class="adg-test-result"></span>' }
        ]) + '<div id="evomap-secret-state" class="adg-quick-stats is-hidden"></div>'
    });

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    evomapRefreshStatus();
}

function evomapSetBanner(state, text) {
    const banner = document.getElementById('evomap-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner' + (state ? ' is-' + state : '');
    banner.textContent = text;
}

function evomapSetResult(kind, text) {
    const result = document.getElementById('evomap-test-result');
    if (!result) return;
    const cls = kind === 'success' ? ' is-success' : (kind === 'danger' ? ' is-danger' : (kind === 'warning' ? ' is-warning' : ''));
    result.className = 'adg-test-result' + cls;
    result.textContent = text || '';
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
    evomapSetBanner('neutral', t('config.evomap.checking'));
    try {
        const resp = await fetch('/api/evomap/status');
        const data = await resp.json().catch(() => ({}));
        const status = data.status || 'error';
        if (status === 'ready') {
            evomapSetBanner('success', t('config.evomap.status_ready'));
        } else if (status === 'missing_node_secret') {
            evomapSetBanner('warning', t('config.evomap.status_missing_node_secret'));
        } else if (status === 'disabled') {
            evomapSetBanner('neutral', t('config.evomap.status_disabled'));
        } else {
            evomapSetBanner('danger', data.message || t('config.evomap.status_error'));
        }
        const cfg = evomapConfig();
        cfg.node_secret = data.node_secret_configured ? cfgMaskedSecretFallback : '';
        cfg.api_key = data.api_key_configured ? cfgMaskedSecretFallback : '';
        evomapRenderSecretState(data);
    } catch (_) {
        evomapSetBanner('danger', t('config.evomap.status_error'));
    }
}

async function evomapTestConnection() {
    const btn = document.getElementById('evomap-test-btn');
    if (btn) btn.disabled = true;
    evomapSetResult('', t('config.evomap.testing'));
    try {
        const resp = await fetch('/api/evomap/test', { method: 'POST' });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok || data.status !== 'ok') throw new Error(data.message || t('config.evomap.test_fail'));
        evomapSetResult('success', t('config.evomap.test_ok'));
        await evomapRefreshStatus();
    } catch (e) {
        evomapSetResult('danger', e.message || t('config.evomap.test_fail'));
    } finally {
        if (btn) btn.disabled = false;
    }
}

async function evomapRegisterNode() {
    const btn = document.getElementById('evomap-register-btn');
    if (btn) btn.disabled = true;
    evomapSetResult('', t('config.evomap.registering'));
    try {
        const resp = await fetch('/api/evomap/register', { method: 'POST' });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok || data.status !== 'ok') throw new Error(data.message || t('config.evomap.register_fail'));
        if (data.node_id) {
            setNestedValue(configData, 'evomap.node_id', data.node_id);
            const input = document.querySelector('[data-path="evomap.node_id"]');
            if (input) input.value = data.node_id;
        }
        cfgMarkSecretStored(document.getElementById('evomap-node-secret'), 'evomap.node_secret');
        evomapSetResult('success', t('config.evomap.register_ok'));
        await evomapRefreshStatus();
    } catch (e) {
        evomapSetResult('danger', e.message || t('config.evomap.register_fail'));
    } finally {
        if (btn) btn.disabled = false;
    }
}

async function evomapSaveSecret(vaultKey, configPath, inputID) {
    const input = document.getElementById(inputID);
    const value = input ? input.value.trim() : '';
    if (!value) {
        evomapSetResult('warning', t('config.evomap.secret_empty'));
        return;
    }
    try {
        const resp = await fetch('/api/vault/secrets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: vaultKey, value })
        });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok || (data.status && data.status !== 'ok')) throw new Error(data.message || t('config.evomap.secret_save_fail'));
        cfgMarkSecretStored(input, configPath);
        evomapSetResult('success', t('config.evomap.secret_saved'));
        await evomapRefreshStatus();
    } catch (e) {
        evomapSetResult('danger', e.message || t('config.evomap.secret_save_fail'));
    }
}
