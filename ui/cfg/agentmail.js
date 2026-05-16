// cfg/agentmail.js — AgentMail integration section module

function renderAgentMailSection(section) {
    const data = configData['agentmail'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const relayOn = data.relay_to_agent === true;
    const wsOn = data.use_websocket !== false;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.agentmail.api_key_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div id="agentmail-status-banner" class="adg-status-banner">' + t('config.agentmail.checking') + '</div>';

    html += agentMailToggle('agentmail.enabled', enabledOn, t('config.agentmail.enabled_label'), t('help.agentmail.enabled'));
    html += agentMailToggle('agentmail.readonly', readonlyOn, t('config.agentmail.readonly_label'), t('help.agentmail.readonly'));

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.agentmail.api_key_label') + '</div>';
    html += '<div class="field-help">' + t('help.agentmail.api_key') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap" style="flex:1;">';
    html += '<input class="field-input adg-password-input" type="password" id="agentmail-api-key" value="' + escapeAttr(cfgSecretValue(data.api_key)) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="agentMailSaveAPIKey()">' + t('config.agentmail.save_icon') + ' ' + t('config.agentmail.save_vault') + '</button>';
    html += '</div></div>';

    html += agentMailInput('agentmail.inbox_id', data.inbox_id || '', t('config.agentmail.inbox_id_label'), t('help.agentmail.inbox_id'), 'text', 'inbox_...');
    html += agentMailToggle('agentmail.auto_create_inbox', data.auto_create_inbox === true, t('config.agentmail.auto_create_label'), t('help.agentmail.auto_create_inbox'));
    html += agentMailInput('agentmail.username', data.username || '', t('config.agentmail.username_label'), t('help.agentmail.username'), 'text', 'aurago');
    html += agentMailInput('agentmail.domain', data.domain || '', t('config.agentmail.domain_label'), t('help.agentmail.domain'), 'text', 'agentmail.to');
    html += agentMailInput('agentmail.display_name', data.display_name || '', t('config.agentmail.display_name_label'), t('help.agentmail.display_name'), 'text', 'AuraGo');

    html += agentMailToggle('agentmail.relay_to_agent', relayOn, t('config.agentmail.relay_label'), t('help.agentmail.relay_to_agent'));
    html += agentMailToggle('agentmail.use_websocket', wsOn, t('config.agentmail.websocket_label'), t('help.agentmail.use_websocket'));
    html += agentMailInput('agentmail.poll_interval_seconds', String(data.poll_interval_seconds || 120), t('config.agentmail.poll_interval_label'), t('help.agentmail.poll_interval_seconds'), 'number', '120', '30', '1');
    html += agentMailInput('agentmail.max_attachment_mb', String(data.max_attachment_mb || 10), t('config.agentmail.max_attachment_label'), t('help.agentmail.max_attachment_mb'), 'number', '10', '0', '1');
    html += agentMailInput('agentmail.base_url', data.base_url || 'https://api.agentmail.to', t('config.agentmail.base_url_label'), t('help.agentmail.base_url'), 'text', 'https://api.agentmail.to');
    html += agentMailInput('agentmail.websocket_url', data.websocket_url || 'wss://ws.agentmail.to/v0', t('config.agentmail.websocket_url_label'), t('help.agentmail.websocket_url'), 'text', 'wss://ws.agentmail.to/v0');

    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="agentMailTestConnection()" id="agentmail-test-btn">🔌 ' + t('config.agentmail.test_btn') + '</button>';
    html += '<span id="agentmail-test-result" class="adg-test-result"></span>';
    html += '</div>';

    html += '<div id="agentmail-summary" class="adg-quick-stats is-hidden">';
    html += '<div class="adg-stats-title">' + t('config.agentmail.summary_title') + '</div>';
    html += '<div id="agentmail-summary-content" class="adg-stats-grid"></div>';
    html += '</div>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
    agentMailCheckStatus();
}

function agentMailToggle(path, on, label, help) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + label + '</div>';
    html += '<div class="field-help">' + help + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (on ? ' on' : '') + '" data-path="' + path + '" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (on ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';
    return html;
}

function agentMailInput(path, value, label, help, type, placeholder, min, step) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + label + '</div>';
    html += '<div class="field-help">' + help + '</div>';
    html += '<input class="field-input" type="' + (type || 'text') + '" data-path="' + path + '" value="' + escapeAttr(value || '') + '" placeholder="' + escapeAttr(placeholder || '') + '"';
    if (min !== undefined) html += ' min="' + escapeAttr(String(min)) + '"';
    if (step !== undefined) html += ' step="' + escapeAttr(String(step)) + '"';
    html += '>';
    html += '</div>';
    return html;
}

function agentMailSetBanner(state, text) {
    const banner = document.getElementById('agentmail-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function agentMailCheckStatus() {
    agentMailSetBanner('neutral', t('config.agentmail.checking'));
    fetch('/api/agentmail/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                agentMailSetBanner('neutral', '⚪ ' + t('config.agentmail.status_disabled'));
                return;
            }
            if (res.status === 'no_api_key') {
                agentMailSetBanner('warning', '🟡 ' + t('config.agentmail.status_no_api_key'));
                return;
            }
            if (res.status === 'no_inbox') {
                agentMailSetBanner('warning', '🟡 ' + t('config.agentmail.status_no_inbox'));
                return;
            }
            agentMailSetBanner(res.running ? 'success' : 'neutral', (res.running ? '🟢 ' : '⚪ ') + t('config.agentmail.status_ready'));
            agentMailRenderSummary(res);
        })
        .catch(() => agentMailSetBanner('danger', '🔴 ' + t('config.agentmail.connection_failed')));
}

function agentMailRenderSummary(status) {
    const wrap = document.getElementById('agentmail-summary');
    const content = document.getElementById('agentmail-summary-content');
    if (!wrap || !content) return;
    wrap.classList.remove('is-hidden');
    let html = '';
    html += '<div><strong>' + t('config.agentmail.summary_inbox') + '</strong><br>' + escapeHtml(status.inbox_id || '-') + '</div>';
    html += '<div><strong>' + t('config.agentmail.summary_relay') + '</strong><br>' + escapeHtml(status.relay_to_agent ? t('config.agentmail.yes') : t('config.agentmail.no')) + '</div>';
    html += '<div><strong>' + t('config.agentmail.summary_transport') + '</strong><br>' + escapeHtml(status.use_websocket ? 'WebSocket' : 'Polling') + '</div>';
    html += '<div><strong>' + t('config.agentmail.summary_mode') + '</strong><br>' + escapeHtml(status.readonly ? t('config.agentmail.mode_readonly') : t('config.agentmail.mode_full')) + '</div>';
    content.innerHTML = html;
}

function agentMailTestConnection() {
    const btn = document.getElementById('agentmail-test-btn');
    const result = document.getElementById('agentmail-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.textContent = t('config.agentmail.loading');
        result.className = 'adg-test-result';
    }
    fetch('/api/agentmail/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.agentmail.test_ok', { count: res.inbox_count || 0 });
                agentMailCheckStatus();
            } else {
                result.className = 'adg-test-result is-danger';
                result.textContent = res.message || t('config.agentmail.test_fail');
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.agentmail.test_fail');
            }
        });
}

function agentMailSaveAPIKey() {
    const input = document.getElementById('agentmail-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.agentmail.api_key_empty'), 'error');
        return;
    }
    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'agentmail_api_key', value: value })
    })
        .then(r => r.json())
        .then(res => {
            if (res.status === 'ok' || res.success) {
                showToast(t('config.agentmail.api_key_saved'), 'success');
                cfgMarkSecretStored(input, 'agentmail.api_key');
                agentMailCheckStatus();
            } else {
                showToast(res.message || t('config.agentmail.api_key_save_failed'), 'error');
            }
        })
        .catch(() => showToast(t('config.agentmail.api_key_save_failed'), 'error'));
}
