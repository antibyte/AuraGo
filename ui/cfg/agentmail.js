// cfg/agentmail.js — AgentMail integration section module

function renderAgentMailSection(section) {
    const data = configData['agentmail'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const relayOn = data.relay_to_agent === true;
    const wsOn = data.use_websocket !== false;
    const form = window.AuraConfigForm;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.agentmail.api_key_placeholder'));

    const html = form.renderSpec({
        icon: section.icon,
        label: section.label,
        desc: section.desc,
        beforeHTML: '<div id="agentmail-status-banner" class="adg-status-banner">' + t('config.agentmail.checking') + '</div>',
        fields: [
            form.toggle({ label: t('config.agentmail.enabled_label'), help: t('help.agentmail.enabled'), path: 'agentmail.enabled', value: enabledOn }),
            form.toggle({ label: t('config.agentmail.readonly_label'), help: t('help.agentmail.readonly'), path: 'agentmail.readonly', value: readonlyOn }),
            form.password({
                label: t('config.agentmail.api_key_label'),
                help: t('help.agentmail.api_key'),
                id: 'agentmail-api-key',
                value: data.api_key,
                placeholder: apiKeyPlaceholder,
                actionHTML: '<button class="btn-save adg-save-btn" onclick="agentMailSaveAPIKey()">' + t('config.agentmail.save_icon') + ' ' + t('config.agentmail.save_vault') + '</button>'
            }),
            form.field({ label: t('config.agentmail.inbox_id_label'), help: t('help.agentmail.inbox_id'), path: 'agentmail.inbox_id', value: data.inbox_id || '', placeholder: 'inbox_...' }),
            form.toggle({ label: t('config.agentmail.auto_create_label'), help: t('help.agentmail.auto_create_inbox'), path: 'agentmail.auto_create_inbox', value: data.auto_create_inbox === true }),
            form.field({ label: t('config.agentmail.username_label'), help: t('help.agentmail.username'), path: 'agentmail.username', value: data.username || '', placeholder: 'aurago' }),
            form.field({ label: t('config.agentmail.domain_label'), help: t('help.agentmail.domain'), path: 'agentmail.domain', value: data.domain || '', placeholder: 'agentmail.to' }),
            form.field({ label: t('config.agentmail.display_name_label'), help: t('help.agentmail.display_name'), path: 'agentmail.display_name', value: data.display_name || '', placeholder: 'AuraGo' }),
            form.toggle({ label: t('config.agentmail.relay_label'), help: t('help.agentmail.relay_to_agent'), path: 'agentmail.relay_to_agent', value: relayOn }),
            agentMailCheatsheetSelect(data.relay_cheatsheet_id || ''),
            form.toggle({ label: t('config.agentmail.websocket_label'), help: t('help.agentmail.use_websocket'), path: 'agentmail.use_websocket', value: wsOn }),
            form.number({ label: t('config.agentmail.poll_interval_label'), help: t('help.agentmail.poll_interval_seconds'), path: 'agentmail.poll_interval_seconds', value: String(data.poll_interval_seconds || 120), min: 30, step: 1 }),
            form.number({ label: t('config.agentmail.max_attachment_label'), help: t('help.agentmail.max_attachment_mb'), path: 'agentmail.max_attachment_mb', value: String(data.max_attachment_mb || 10), min: 0, step: 1 }),
            form.field({ label: t('config.agentmail.base_url_label'), help: t('help.agentmail.base_url'), path: 'agentmail.base_url', value: data.base_url || 'https://api.agentmail.to', placeholder: 'https://api.agentmail.to' }),
            form.field({ label: t('config.agentmail.websocket_url_label'), help: t('help.agentmail.websocket_url'), path: 'agentmail.websocket_url', value: data.websocket_url || 'wss://ws.agentmail.to/v0', placeholder: 'wss://ws.agentmail.to/v0' })
        ],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="agentMailTestConnection()" id="agentmail-test-btn">🔌 ' + t('config.agentmail.test_btn') + '</button>' },
            { html: '<span id="agentmail-test-result" class="adg-test-result"></span>' }
        ]) + '<div id="agentmail-summary" class="adg-quick-stats is-hidden">'
            + '<div class="adg-stats-title">' + t('config.agentmail.summary_title') + '</div>'
            + '<div id="agentmail-summary-content" class="adg-stats-grid"></div>'
            + '</div>'
    });

    document.getElementById('content').innerHTML = html;
    agentMailLoadCheatsheets(data.relay_cheatsheet_id || '');
    agentMailCheckStatus();
}

function agentMailCheatsheetSelect(selectedID) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t('config.agentmail.relay_cheatsheet_label') + '</div>';
    html += '<div class="field-help">' + t('help.agentmail.relay_cheatsheet_id') + '</div>';
    html += '<select class="field-select" id="agentmail-relay-cheatsheet" data-path="agentmail.relay_cheatsheet_id" data-selected="' + escapeAttr(selectedID || '') + '" onchange="setNestedValue(configData,\'agentmail.relay_cheatsheet_id\',this.value);setDirty(true)">';
    html += '<option value="">' + escapeHtml(t('config.agentmail.loading')) + '</option>';
    html += '</select>';
    html += '</div>';
    return html;
}

function agentMailLoadCheatsheets(selectedID) {
    const select = document.getElementById('agentmail-relay-cheatsheet');
    if (!select) return;
    fetch('/api/cheatsheets')
        .then(r => {
            if (!r.ok) throw new Error(r.statusText || 'Failed to load cheat sheets');
            return r.json();
        })
        .then(sheets => {
            const list = Array.isArray(sheets) ? sheets.slice() : [];
            list.sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')));
            let html = '<option value="">' + escapeHtml(t('config.agentmail.relay_cheatsheet_none')) + '</option>';
            if (selectedID && !list.some(s => s.id === selectedID)) {
                html += '<option value="' + escapeAttr(selectedID) + '">' + escapeHtml(selectedID) + '</option>';
            }
            list.forEach(sheet => {
                html += '<option value="' + escapeAttr(sheet.id) + '">' + escapeHtml(sheet.name || sheet.id) + '</option>';
            });
            if (list.length === 0 && !selectedID) {
                html += '<option value="" disabled>' + escapeHtml(t('config.agentmail.relay_cheatsheet_empty')) + '</option>';
            }
            select.innerHTML = html;
            select.value = selectedID || '';
        })
        .catch(() => {
            select.innerHTML = '<option value="' + escapeAttr(selectedID || '') + '">' + escapeHtml(t('config.agentmail.relay_cheatsheet_error')) + '</option>';
            select.value = selectedID || '';
        });
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