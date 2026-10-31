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
        label: section.label,
        desc: section.desc,
        beforeHTML: form.statusBanner({ id: 'agentmail-status-banner', state: enabledOn ? 'checking' : 'off' }),
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.agentmail.enabled_label'), help: t('help.agentmail.enabled'), path: 'agentmail.enabled', value: enabledOn }),
            form.toggle({ label: t('config.agentmail.readonly_label'), help: t('help.agentmail.readonly'), path: 'agentmail.readonly', value: readonlyOn }),
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.password({
                label: t('config.agentmail.api_key_label'),
                help: t('help.agentmail.api_key'),
                id: 'agentmail-api-key',
                path: 'agentmail.api_key',
                value: data.api_key,
                placeholder: apiKeyPlaceholder
            }),
        ] }, { titleKey: 'config.agentmail.inbox_id_label', fields: [
            form.field({ label: t('config.agentmail.inbox_id_label'), help: t('help.agentmail.inbox_id'), path: 'agentmail.inbox_id', value: data.inbox_id || '', placeholder: 'inbox_...' }),
            form.toggle({ label: t('config.agentmail.auto_create_label'), help: t('help.agentmail.auto_create_inbox'), path: 'agentmail.auto_create_inbox', value: data.auto_create_inbox === true }),
            form.field({ label: t('config.agentmail.username_label'), help: t('help.agentmail.username'), path: 'agentmail.username', value: data.username || '', placeholder: 'aurago' }),
            form.field({ label: t('config.agentmail.domain_label'), help: t('help.agentmail.domain'), path: 'agentmail.domain', value: data.domain || '', placeholder: 'agentmail.to' }),
            form.field({ label: t('config.agentmail.display_name_label'), help: t('help.agentmail.display_name'), path: 'agentmail.display_name', value: data.display_name || '', placeholder: 'AuraGo' }),
        ] }, { titleKey: 'config.refresh.runtime', fields: [
            form.toggle({ label: t('config.agentmail.relay_label'), help: t('help.agentmail.relay_to_agent'), path: 'agentmail.relay_to_agent', value: relayOn }),
            agentMailAllowedSendersField(data.allowed_senders),
            agentMailCheatsheetSelect(data.relay_cheatsheet_id || ''),
            form.toggle({ label: t('config.agentmail.websocket_label'), help: t('help.agentmail.use_websocket'), path: 'agentmail.use_websocket', value: wsOn }),
            form.number({ label: t('config.agentmail.poll_interval_label'), help: t('help.agentmail.poll_interval_seconds'), path: 'agentmail.poll_interval_seconds', value: String(data.poll_interval_seconds || 120), min: 30, step: 1 }),
            form.number({ label: t('config.agentmail.max_attachment_label'), help: t('help.agentmail.max_attachment_mb'), path: 'agentmail.max_attachment_mb', value: String(data.max_attachment_mb || 10), min: 0, step: 1 }),
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.agentmail.base_url_label'), help: t('help.agentmail.base_url'), path: 'agentmail.base_url', value: data.base_url || 'https://api.agentmail.to', placeholder: 'https://api.agentmail.to' }),
            form.field({ label: t('config.agentmail.websocket_url_label'), help: t('help.agentmail.websocket_url'), path: 'agentmail.websocket_url', value: data.websocket_url || 'wss://ws.agentmail.to/v0', placeholder: 'wss://ws.agentmail.to/v0' })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save adg-test-btn" onclick="agentMailTestConnection()" id="agentmail-test-btn" data-config-action-draft="true">' + t('config.agentmail.test_btn') + '</button>' },
            { html: form.actionResult({ id: 'agentmail-test-result' }) }
        ]) + '<div id="agentmail-summary" class="adg-quick-stats is-hidden">'
            + '<div class="adg-stats-title">' + t('config.agentmail.summary_title') + '</div>'
            + '<div id="agentmail-summary-content" class="adg-stats-grid"></div>'
            + '</div>'
    });

    document.getElementById('content').innerHTML = html;
    agentMailLoadCheatsheets(data.relay_cheatsheet_id || '');
    if (enabledOn) agentMailCheckStatus();
}

// One address or @domain per line; the shared draft reads the textarea as a list of lines.
function agentMailAllowedSendersField(senders) {
    return window.AuraConfigForm.textarea({
        label: t('config.agentmail.allowed_senders_label'),
        help: t('help.agentmail.allowed_senders'),
        path: 'agentmail.allowed_senders',
        value: Array.isArray(senders) ? senders.join('\n') : '',
        rows: 4,
        placeholder: 'anna@example.com\n@partner.example'
    }).replace('<textarea ', '<textarea data-type="array-lines" ');
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

function agentMailCheckStatus() {
    const banner = document.getElementById('agentmail-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/agentmail/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'no_api_key') return { state: 'unconfigured', detail: t('config.agentmail.status_no_api_key') };
        if (res.status === 'no_inbox') return { state: 'degraded', detail: t('config.agentmail.status_no_inbox') };
        agentMailRenderSummary(res);
        return { state: 'connected', detail: t('config.agentmail.status_ready') };
    });
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
    return cfgActionResult(btn, () => fetch('/api/agentmail/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['agentmail']) })
    }), {
        target: 'agentmail-test-result',
        success: outcome => t('config.agentmail.test_ok', { count: (outcome.body && outcome.body.inbox_count) || 0 }),
        failure: t('config.agentmail.test_fail'),
        onSuccess: () => agentMailCheckStatus()
    });
}
