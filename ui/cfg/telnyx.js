// cfg/telnyx.js — Telnyx SMS & Voice integration config panel

function renderTelnyxSection(section) {
    const data = configData['telnyx'] || {};
    const enabled = data.enabled === true;
    const readOnly = data.read_only === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabled ? ' on' : '') + '" data-path="telnyx.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.read_only_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.read_only') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readOnly ? ' on' : '') + '" data-path="telnyx.read_only" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readOnly ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── API Key (Vault) ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.api_key_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.api_key') + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;">';
    html += '<input class="field-input" type="password" id="telnyx-api-key" placeholder="' + t('config.telnyx.api_key_placeholder') + '" style="flex:1;">';
    html += '<button class="btn btn-sm" onclick="saveTelnyxVault(\'api_key\')">' + t('config.telnyx.save_vault') + '</button>';
    html += '</div></div>';

    // ── Phone Number ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.phone_number_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.phone_number') + '</div>';
    html += '<input class="field-input" type="text" data-path="telnyx.phone_number" value="' + escapeAttr(data.phone_number || '') + '" placeholder="+15551234567">';
    html += '</div>';

    // ── Messaging Profile ID ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.messaging_profile_id_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.messaging_profile_id') + '</div>';
    html += '<input class="field-input" type="text" data-path="telnyx.messaging_profile_id" value="' + escapeAttr(data.messaging_profile_id || '') + '" placeholder="">';
    html += '</div>';

    // ── Connection ID (for voice calls) ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.connection_id_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.connection_id') + '</div>';
    html += '<input class="field-input" type="text" data-path="telnyx.connection_id" value="' + escapeAttr(data.connection_id || '') + '" placeholder="">';
    html += '</div>';

    // ── Webhook Path ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.webhook_path_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.webhook_path') + '</div>';
    html += '<input class="field-input" type="text" data-path="telnyx.webhook_path" value="' + escapeAttr(data.webhook_path || '/api/telnyx/webhook') + '" placeholder="/api/telnyx/webhook">';
    html += '</div>';

    // ── Allowed Numbers ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.allowed_numbers_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.allowed_numbers') + '</div>';
    const nums = (data.allowed_numbers || []).join(', ');
    html += '<input class="field-input" type="text" id="telnyx-allowed-numbers" value="' + escapeAttr(nums) + '" placeholder="+15551234567, +15559876543" onchange="parseTelnyxAllowedNumbers(this)">';
    html += '</div>';

    // ── Voice Settings ──
    html += '<div class="field-group" style="margin-top:1.5rem;">';
    html += '<div class="field-label" style="font-weight:600;font-size:0.95rem;">' + t('config.telnyx.voice_settings_title') + '</div>';
    html += '</div>';

    // Voice Language
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.voice_language_label') + '</div>';
    html += '<input class="field-input" type="text" data-path="telnyx.voice_language" value="' + escapeAttr(data.voice_language || 'en') + '" placeholder="en">';
    html += '</div>';

    // Voice Gender
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.voice_gender_label') + '</div>';
    html += '<select class="field-input" data-path="telnyx.voice_gender">';
    html += '<option value="female"' + ((data.voice_gender || 'female') === 'female' ? ' selected' : '') + '>' + t('config.telnyx.voice_female') + '</option>';
    html += '<option value="male"' + (data.voice_gender === 'male' ? ' selected' : '') + '>' + t('config.telnyx.voice_male') + '</option>';
    html += '</select>';
    html += '</div>';

    // Call Timeout
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.call_timeout_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.call_timeout') + '</div>';
    html += '<input class="field-input" type="number" data-path="telnyx.call_timeout" value="' + escapeAttr(String(data.call_timeout || 300)) + '" min="30" max="3600">';
    html += '</div>';

    // Max Concurrent Calls
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.max_concurrent_calls_label') + '</div>';
    html += '<input class="field-input" type="number" data-path="telnyx.max_concurrent_calls" value="' + escapeAttr(String(data.max_concurrent_calls || 3)) + '" min="1" max="20">';
    html += '</div>';

    // Max SMS per Minute
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.max_sms_per_minute_label') + '</div>';
    html += '<input class="field-input" type="number" data-path="telnyx.max_sms_per_minute" value="' + escapeAttr(String(data.max_sms_per_minute || 10)) + '" min="1" max="60">';
    html += '</div>';

    // ── Recording & Transcription toggles ──
    const recordCalls = data.record_calls === true;
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.record_calls_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.record_calls') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (recordCalls ? ' on' : '') + '" data-path="telnyx.record_calls" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (recordCalls ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    const relayOn = data.relay_to_agent !== false; // default true
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.telnyx.relay_to_agent_label') + '</div>';
    html += '<div class="field-help">' + t('help.telnyx.relay_to_agent') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (relayOn ? ' on' : '') + '" data-path="telnyx.relay_to_agent" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (relayOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '</div>'; // close cfg-section
    return html;
}

function saveTelnyxVault(field) {
    const el = document.getElementById('telnyx-api-key');
    if (!el || !el.value.trim()) {
        showToast(t('config.telnyx.vault_empty'), 'warn');
        return;
    }
    fetch('/api/vault', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'telnyx_' + field, value: el.value.trim() })
    }).then(r => {
        if (r.ok) {
            el.value = '';
            showToast(t('config.telnyx.vault_saved'), 'success');
        } else {
            showToast(t('config.telnyx.vault_failed'), 'error');
        }
    }).catch(() => showToast(t('config.telnyx.vault_failed'), 'error'));
}

function parseTelnyxAllowedNumbers(el) {
    const nums = el.value.split(',').map(s => s.trim()).filter(Boolean);
    setNestedValue(configData, 'telnyx.allowed_numbers', nums);
}
