// cfg/grafana.js — Grafana integration section module

function renderGrafanaSection(section) {
    const data = configData['grafana'] || {};
    const enabledOn = data.enabled === true;
    const insecureOn = data.insecure_ssl === true;
    const readonlyOn = data.readonly !== false;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.grafana.api_key_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div id="grafana-status-banner" class="adg-status-banner">' + t('config.grafana.checking') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.grafana.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.grafana.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="grafana.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.grafana.base_url_label') + '</div>';
    html += '<div class="field-help">' + t('help.grafana.base_url') + '</div>';
    html += '<input class="field-input" type="text" data-path="grafana.base_url" value="' + escapeAttr(data.base_url || '') + '" placeholder="http://grafana.local:3000">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.grafana.api_key_label') + '</div>';
    html += '<div class="field-help">' + t('help.grafana.api_key') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap" style="flex:1;">';
    html += '<input class="field-input adg-password-input" type="password" id="grafana-api-key" value="' + escapeAttr(cfgSecretValue(data.api_key)) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="grafanaSaveAPIKey()">' + t('config.grafana.save_icon') + ' ' + t('config.grafana.save_vault') + '</button>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.grafana.readonly_label') + '</div>';
    html += '<div class="field-help">' + t('help.grafana.readonly') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="grafana.readonly" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.grafana.insecure_ssl_label') + '</div>';
    html += '<div class="field-help">' + t('help.grafana.insecure_ssl') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (insecureOn ? ' on' : '') + '" data-path="grafana.insecure_ssl" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (insecureOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.grafana.request_timeout_label') + '</div>';
    html += '<div class="field-help">' + t('help.grafana.request_timeout') + '</div>';
    html += '<input class="field-input" type="number" min="1" step="1" data-path="grafana.request_timeout" value="' + escapeAttr(String(data.request_timeout || 15)) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="grafanaTestConnection()" id="grafana-test-btn">🔌 ' + t('config.grafana.test_btn') + '</button>';
    html += '<span id="grafana-test-result" class="adg-test-result"></span>';
    html += '</div>';

    html += '<div id="grafana-summary" class="adg-quick-stats is-hidden">';
    html += '<div class="adg-stats-title">📊 ' + t('config.grafana.summary_title') + '</div>';
    html += '<div id="grafana-summary-content" class="adg-stats-grid"></div>';
    html += '</div>';
    html += '</div>';

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
