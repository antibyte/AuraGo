// cfg/grafana.js — Grafana integration section module

function renderGrafanaSection(section) {
    const data = configData['grafana'] || {};
    const enabledOn = data.enabled === true;
    const insecureOn = data.insecure_ssl === true;
    const readonlyOn = data.readonly !== false;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.grafana.api_key_placeholder'));
    const form = window.AuraConfigForm;
    const checking = enabledOn && data.base_url;
    const html = window.AuraConfigForm.renderSpec({
        label: section.label,
        desc: section.desc,
        beforeHTML: form.statusBanner({ id: 'grafana-status-banner', state: checking ? 'checking' : (enabledOn ? 'unconfigured' : 'off'), detail: enabledOn && !data.base_url ? t('config.grafana.status_no_url') : '' }),
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.grafana.enabled_label'), help: t('help.grafana.enabled'), path: 'grafana.enabled', value: enabledOn }),
            form.toggle({ label: t('config.grafana.readonly_label'), help: t('help.grafana.readonly'), path: 'grafana.readonly', value: readonlyOn })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.grafana.base_url_label'), help: t('help.grafana.base_url'), path: 'grafana.base_url', value: data.base_url || '', placeholder: 'http://grafana.local:3000' }),
            form.toggle({ label: t('config.grafana.insecure_ssl_label'), help: t('help.grafana.insecure_ssl'), path: 'grafana.insecure_ssl', value: insecureOn }),
            form.number({ label: t('config.grafana.request_timeout_label'), help: t('help.grafana.request_timeout'), path: 'grafana.request_timeout', value: String(data.request_timeout || 15), min: 1, step: 1 })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.password({
                label: t('config.grafana.api_key_label'),
                help: t('help.grafana.api_key'),
                id: 'grafana-api-key',
                path: 'grafana.api_key',
                value: data.api_key,
                placeholder: apiKeyPlaceholder
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="grafanaTestConnection()" id="grafana-test-btn" data-config-action-draft="true">' + t('config.grafana.test_btn') + '</button>' },
            { html: form.actionResult({ id: 'grafana-test-result' }) }
        ]) + '<div id="grafana-summary" class="adg-quick-stats is-hidden">'
            + '<div class="adg-stats-title">' + t('config.grafana.summary_title') + '</div>'
            + '<div id="grafana-summary-content" class="adg-stats-grid"></div>'
            + '</div>'
    });

    document.getElementById('content').innerHTML = html;

    if (checking) grafanaCheckStatus();
}

function grafanaCheckStatus() {
    const banner = document.getElementById('grafana-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/grafana/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'no_url') return { state: 'unconfigured', detail: t('config.grafana.status_no_url') };
        if (res.status === 'no_api_key') return { state: 'unconfigured', detail: t('config.grafana.status_no_api_key') };
        if (res.status !== 'ok' || !res.data) return { state: 'error', detail: res.message || t('config.grafana.connection_failed') };
        grafanaRenderSummary(res.data.summary || {}, res.data.partial_errors || []);
        return { state: 'connected', detail: t('config.grafana.connected') };
    });
}

function grafanaRenderSummary(summary, partialErrors) {
    const wrap = document.getElementById('grafana-summary');
    const content = document.getElementById('grafana-summary-content');
    if (!wrap || !content) return;
    wrap.classList.remove('is-hidden');
    let html = '';
    html += '<div><strong>' + t('config.grafana.summary_dashboards') + '</strong><br>' + escapeHtml(String(summary.dashboards || 0)) + '</div>';
    html += '<div><strong>' + t('config.grafana.summary_datasources') + '</strong><br>' + escapeHtml(String(summary.datasources || 0)) + '</div>';
    html += '<div><strong>' + t('config.grafana.summary_alerts') + '</strong><br>' + escapeHtml(String(summary.alerts || 0)) + '</div>';
    html += '<div><strong>' + t('config.grafana.summary_org') + '</strong><br>' + escapeHtml(summary.org || '-') + '</div>';
    if (Array.isArray(partialErrors) && partialErrors.length > 0) {
        html += '<div class="adg-stats-loaded"><strong>' + t('config.grafana.partial_warning') + '</strong><br>' + escapeHtml(partialErrors.join(' | ')) + '</div>';
    }
    content.innerHTML = html || '<div class="adg-stats-loaded">' + t('config.grafana.summary_empty') + '</div>';
}

function grafanaTestConnection() {
    const btn = document.getElementById('grafana-test-btn');
    return cfgActionResult(btn, () => fetch('/api/grafana/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['grafana']) })
    }), {
        target: 'grafana-test-result',
        success: t('config.grafana.test_ok'),
        failure: t('config.grafana.test_fail'),
        onSuccess: () => grafanaCheckStatus()
    });
}
