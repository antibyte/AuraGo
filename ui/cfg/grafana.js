// cfg/grafana.js — Grafana integration section module

function renderGrafanaSection(section) {
    const data = configData['grafana'] || {};
    const enabledOn = data.enabled === true;
    const insecureOn = data.insecure_ssl === true;
    const readonlyOn = data.readonly !== false;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.grafana.api_key_placeholder'));
    const form = window.AuraConfigForm;
    const html = window.AuraConfigForm.renderSpec({
        icon: section.icon,
        label: section.label,
        desc: section.desc,
        beforeHTML: '<div id="grafana-status-banner" class="adg-status-banner">' + t('config.grafana.checking') + '</div>',
        fields: [
            form.toggle({ label: t('config.grafana.enabled_label'), help: t('help.grafana.enabled'), path: 'grafana.enabled', value: enabledOn }),
            form.field({ label: t('config.grafana.base_url_label'), help: t('help.grafana.base_url'), path: 'grafana.base_url', value: data.base_url || '', placeholder: 'http://grafana.local:3000' }),
            form.password({
                label: t('config.grafana.api_key_label'),
                help: t('help.grafana.api_key'),
                id: 'grafana-api-key',
                value: data.api_key,
                placeholder: apiKeyPlaceholder,
                actionHTML: '<button class="btn-save adg-save-btn" onclick="grafanaSaveAPIKey()">' + t('config.grafana.save_icon') + ' ' + t('config.grafana.save_vault') + '</button>'
            }),
            form.toggle({ label: t('config.grafana.readonly_label'), help: t('help.grafana.readonly'), path: 'grafana.readonly', value: readonlyOn }),
            form.toggle({ label: t('config.grafana.insecure_ssl_label'), help: t('help.grafana.insecure_ssl'), path: 'grafana.insecure_ssl', value: insecureOn }),
            form.number({ label: t('config.grafana.request_timeout_label'), help: t('help.grafana.request_timeout'), path: 'grafana.request_timeout', value: String(data.request_timeout || 15), min: 1, step: 1 })
        ],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="grafanaTestConnection()" id="grafana-test-btn">🔌 ' + t('config.grafana.test_btn') + '</button>' },
            { html: '<span id="grafana-test-result" class="adg-test-result"></span>' }
        ]) + '<div id="grafana-summary" class="adg-quick-stats is-hidden">'
            + '<div class="adg-stats-title">📊 ' + t('config.grafana.summary_title') + '</div>'
            + '<div id="grafana-summary-content" class="adg-stats-grid"></div>'
            + '</div>'
    });

    document.getElementById('content').innerHTML = html;

    if (enabledOn && data.base_url) {
        grafanaCheckStatus();
    } else {
        grafanaSetBanner('neutral', enabledOn ? '⚪ ' + t('config.grafana.status_no_url') : '⚪ ' + t('config.grafana.status_disabled'));
    }
}

function grafanaSetBanner(state, text) {
    const banner = document.getElementById('grafana-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function grafanaCheckStatus() {
    grafanaSetBanner('neutral', t('config.grafana.checking'));
    fetch('/api/grafana/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                grafanaSetBanner('neutral', '⚪ ' + t('config.grafana.status_disabled'));
                return;
            }
            if (res.status === 'no_url') {
                grafanaSetBanner('neutral', '⚪ ' + t('config.grafana.status_no_url'));
                return;
            }
            if (res.status === 'no_api_key') {
                grafanaSetBanner('warning', '🟡 ' + t('config.grafana.status_no_api_key'));
                return;
            }
            if (res.status !== 'ok' || !res.data) {
                grafanaSetBanner('danger', '🔴 ' + (res.message || t('config.grafana.connection_failed')));
                return;
            }
            grafanaSetBanner('success', '🟢 ' + t('config.grafana.connected'));
            grafanaRenderSummary(res.data.summary || {});
        })
        .catch(() => grafanaSetBanner('danger', '🔴 ' + t('config.grafana.connection_failed')));
}

function grafanaRenderSummary(summary) {
    const wrap = document.getElementById('grafana-summary');
    const content = document.getElementById('grafana-summary-content');
    if (!wrap || !content) return;
    wrap.classList.remove('is-hidden');
    let html = '';
    html += '<div><strong>' + t('config.grafana.summary_dashboards') + '</strong><br>' + escapeHtml(String(summary.dashboards || 0)) + '</div>';
    html += '<div><strong>' + t('config.grafana.summary_datasources') + '</strong><br>' + escapeHtml(String(summary.datasources || 0)) + '</div>';
    html += '<div><strong>' + t('config.grafana.summary_alerts') + '</strong><br>' + escapeHtml(String(summary.alerts || 0)) + '</div>';
    html += '<div><strong>' + t('config.grafana.summary_org') + '</strong><br>' + escapeHtml(summary.org || '-') + '</div>';
    content.innerHTML = html || '<div class="adg-stats-loaded">' + t('config.grafana.summary_empty') + '</div>';
}

function grafanaTestConnection() {
    const btn = document.getElementById('grafana-test-btn');
    const result = document.getElementById('grafana-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.textContent = t('config.grafana.loading');
        result.className = 'adg-test-result';
    }
    fetch('/api/grafana/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.grafana.status_success') + ' ' + t('config.grafana.test_ok');
                grafanaCheckStatus();
            } else {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.grafana.status_error') + ' ' + (res.message || t('config.grafana.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.grafana.status_error') + ' ' + t('config.grafana.test_fail');
            }
        });
}

function grafanaSaveAPIKey() {
    const input = document.getElementById('grafana-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.grafana.api_key_empty'), 'error');
        return;
    }
    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'grafana_api_key', value: value })
    })
        .then(r => r.json())
        .then(res => {
            if (res.status === 'ok' || res.success) {
                showToast(t('config.grafana.api_key_saved'), 'success');
                cfgMarkSecretStored(input, 'grafana.api_key');
                grafanaCheckStatus();
            } else {
                showToast(res.message || t('config.grafana.api_key_save_failed'), 'error');
            }
        })
        .catch(() => showToast(t('config.grafana.api_key_save_failed'), 'error'));
}
