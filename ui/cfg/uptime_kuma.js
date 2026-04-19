// cfg/uptime_kuma.js — Uptime Kuma integration section module

function renderUptimeKumaSection(section) {
    const data = configData['uptime_kuma'] || {};
    const enabledOn = data.enabled === true;
    const insecureOn = data.insecure_ssl === true;
    const relayOn = data.relay_to_agent === true;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.uptime_kuma.api_key_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div id="uk-status-banner" class="adg-status-banner">' + t('config.uptime_kuma.checking') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.uptime_kuma.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.uptime_kuma.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="uptime_kuma.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.uptime_kuma.base_url_label') + '</div>';
    html += '<div class="field-help">' + t('help.uptime_kuma.base_url') + '</div>';
    html += '<input class="field-input" type="text" data-path="uptime_kuma.base_url" value="' + escapeAttr(data.base_url || '') + '" placeholder="https://uptime-kuma.local:3001">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.uptime_kuma.api_key_label') + '</div>';
    html += '<div class="field-help">' + t('help.uptime_kuma.api_key') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap" style="flex:1;">';
    html += '<input class="field-input adg-password-input" type="password" id="uptime-kuma-api-key" value="' + escapeAttr(cfgSecretValue(data.api_key)) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="uptimeKumaSaveAPIKey()">' + t('config.uptime_kuma.save_icon') + ' ' + t('config.uptime_kuma.save_vault') + '</button>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.uptime_kuma.insecure_ssl_label') + '</div>';
    html += '<div class="field-help">' + t('help.uptime_kuma.insecure_ssl') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (insecureOn ? ' on' : '') + '" data-path="uptime_kuma.insecure_ssl" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (insecureOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.uptime_kuma.request_timeout_label') + '</div>';
    html += '<div class="field-help">' + t('help.uptime_kuma.request_timeout') + '</div>';
    html += '<input class="field-input" type="number" min="1" step="1" data-path="uptime_kuma.request_timeout" value="' + escapeAttr(String(data.request_timeout || 15)) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.uptime_kuma.poll_interval_label') + '</div>';
    html += '<div class="field-help">' + t('help.uptime_kuma.poll_interval_seconds') + '</div>';
    html += '<input class="field-input" type="number" min="5" step="1" data-path="uptime_kuma.poll_interval_seconds" value="' + escapeAttr(String(data.poll_interval_seconds || 30)) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.uptime_kuma.relay_to_agent_label') + '</div>';
    html += '<div class="field-help">' + t('help.uptime_kuma.relay_to_agent') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (relayOn ? ' on' : '') + '" data-path="uptime_kuma.relay_to_agent" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (relayOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="uptimeKumaTestConnection()" id="uptime-kuma-test-btn">🔌 ' + t('config.uptime_kuma.test_btn') + '</button>';
    html += '<span id="uptime-kuma-test-result" class="adg-test-result"></span>';
    html += '</div>';

    html += '<div id="uptime-kuma-summary" class="adg-quick-stats is-hidden">';
    html += '<div class="adg-stats-title">📊 ' + t('config.uptime_kuma.summary_title') + '</div>';
    html += '<div id="uptime-kuma-summary-content" class="adg-stats-grid"></div>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    if (enabledOn && data.base_url) {
        uptimeKumaCheckStatus();
    } else {
        uptimeKumaSetBanner('neutral', enabledOn ? '⚪ ' + t('config.uptime_kuma.status_no_url') : '⚪ ' + t('config.uptime_kuma.status_disabled'));
    }
}

function uptimeKumaSetBanner(state, text) {
    const banner = document.getElementById('uk-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function uptimeKumaCheckStatus() {
    uptimeKumaSetBanner('neutral', t('config.uptime_kuma.checking'));
    fetch('/api/uptime-kuma/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                uptimeKumaSetBanner('neutral', '⚪ ' + t('config.uptime_kuma.status_disabled'));
                return;
            }
            if (res.status === 'no_url') {
                uptimeKumaSetBanner('neutral', '⚪ ' + t('config.uptime_kuma.status_no_url'));
                return;
            }
            if (res.status === 'no_api_key') {
                uptimeKumaSetBanner('warning', '🟡 ' + t('config.uptime_kuma.status_no_api_key'));
                return;
            }
            if (res.status !== 'ok' || !res.data) {
                uptimeKumaSetBanner('danger', '🔴 ' + (res.message || t('config.uptime_kuma.connection_failed')));
                return;
            }

            const summary = res.data.summary || {};
            const down = Number(summary.down || 0);
            const state = down > 0 ? 'danger' : 'success';
            const text = down > 0
                ? '🔴 ' + t('config.uptime_kuma.connected') + ' - ' + down + ' ' + t('config.uptime_kuma.down_monitors')
                : '🟢 ' + t('config.uptime_kuma.connected');
            uptimeKumaSetBanner(state, text);
            uptimeKumaRenderSummary(summary);
        })
        .catch(() => {
            uptimeKumaSetBanner('danger', '🔴 ' + t('config.uptime_kuma.connection_failed'));
        });
}

function uptimeKumaRenderSummary(summary) {
    const wrap = document.getElementById('uptime-kuma-summary');
    const content = document.getElementById('uptime-kuma-summary-content');
    if (!wrap || !content) return;
    wrap.classList.remove('is-hidden');

    const downNames = Array.isArray(summary.down_monitor_names) ? summary.down_monitor_names : [];
    let html = '';
    html += '<div><strong>' + t('config.uptime_kuma.summary_up') + '</strong><br>' + escapeHtml(String(summary.up || 0)) + '</div>';
    html += '<div><strong>' + t('config.uptime_kuma.summary_down') + '</strong><br>' + escapeHtml(String(summary.down || 0)) + '</div>';
    html += '<div><strong>' + t('config.uptime_kuma.summary_unknown') + '</strong><br>' + escapeHtml(String(summary.unknown || 0)) + '</div>';
    html += '<div><strong>' + t('config.uptime_kuma.summary_total') + '</strong><br>' + escapeHtml(String(summary.total || 0)) + '</div>';
    if (downNames.length > 0) {
        html += '<div style="grid-column:1 / -1;"><strong>' + t('config.uptime_kuma.down_monitors') + '</strong><br>' + escapeHtml(downNames.join(', ')) + '</div>';
    }
    content.innerHTML = html || '<div class="adg-stats-loaded">' + t('config.uptime_kuma.summary_empty') + '</div>';
}

function uptimeKumaTestConnection() {
    const btn = document.getElementById('uptime-kuma-test-btn');
    const result = document.getElementById('uptime-kuma-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.textContent = t('config.uptime_kuma.loading');
        result.className = 'adg-test-result';
    }

    fetch('/api/uptime-kuma/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.uptime_kuma.status_success') + ' ' + t('config.uptime_kuma.test_ok');
                uptimeKumaCheckStatus();
            } else {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.uptime_kuma.status_error') + ' ' + (res.message || t('config.uptime_kuma.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.uptime_kuma.status_error') + ' ' + t('config.uptime_kuma.test_fail');
            }
        });
}

function uptimeKumaSaveAPIKey() {
    const input = document.getElementById('uptime-kuma-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.uptime_kuma.api_key_empty'), 'error');
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'uptime_kuma_api_key', value: value })
    })
        .then(r => r.json())
        .then(res => {
            if (res.status === 'ok' || res.success) {
                showToast(t('config.uptime_kuma.api_key_saved'), 'success');
                cfgMarkSecretStored(input, 'uptime_kuma.api_key');
                uptimeKumaCheckStatus();
            } else {
                showToast(res.message || t('config.uptime_kuma.api_key_save_failed'), 'error');
            }
        })
        .catch(() => showToast(t('config.uptime_kuma.api_key_save_failed'), 'error'));
}
