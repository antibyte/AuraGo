// cfg/telnyx.js — Telnyx SMS & Voice integration config panel

function renderTelnyxSection(section) {
    const data = configData['telnyx'] || {};
    const enabled = data.enabled === true;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.telnyx.api_key_placeholder'));
    const form = window.AuraConfigForm;
    const nums = (data.allowed_numbers || []).join(', ');
    // The allowed numbers are parsed into a list on change (no data-path of their own).
    const allowedNumbers = '<div class="field-group pw-field">'
        + '<div class="field-label" id="telnyx-allowed-numbers-label">' + escapeHtml(t('config.telnyx.allowed_numbers_label')) + '</div>'
        + '<input class="field-input" type="text" id="telnyx-allowed-numbers" aria-labelledby="telnyx-allowed-numbers-label" aria-describedby="telnyx-allowed-numbers-help" value="' + escapeAttr(nums) + '" placeholder="+15551234567, +15559876543" onchange="parseTelnyxAllowedNumbers(this)">'
        + '<div class="field-help" id="telnyx-allowed-numbers-help">' + escapeHtml(t('help.telnyx.allowed_numbers')) + '</div>'
        + '</div>';
    const voiceLanguage = '<div class="field-group pw-field">'
        + '<div class="field-label" id="telnyx-voice-language-label">' + escapeHtml(t('config.telnyx.voice_language_label')) + '</div>'
        + telnyxVoiceLanguageSelect(data.voice_language || 'en')
        + '<div class="field-help">' + escapeHtml(t('help.telnyx.voice_language')) + '</div>'
        + '</div>';

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        banner: { id: 'telnyx-status-banner', state: enabled ? 'checking' : 'off' },
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.telnyx.enabled_label'), help: t('help.telnyx.enabled'), path: 'telnyx.enabled', value: enabled }),
            form.toggle({ label: t('config.telnyx.read_only_label'), help: t('help.telnyx.read_only'), path: 'telnyx.read_only', value: data.read_only === true }),
            form.toggle({ label: t('config.telnyx.relay_to_agent_label'), help: t('help.telnyx.relay_to_agent'), path: 'telnyx.relay_to_agent', value: data.relay_to_agent !== false })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.password({
                label: t('config.telnyx.api_key_label'),
                help: t('help.telnyx.api_key'),
                id: 'telnyx-api-key',
                path: 'telnyx.api_key',
                value: data.api_key,
                placeholder: apiKeyPlaceholder
            }),
            form.field({ label: t('config.telnyx.webhook_public_key_label'), help: t('help.telnyx.webhook_public_key'), path: 'telnyx.webhook_public_key', value: data.webhook_public_key || '' })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.telnyx.phone_number_label'), help: t('help.telnyx.phone_number'), path: 'telnyx.phone_number', value: data.phone_number || '', placeholder: '+15551234567' }),
            form.field({ label: t('config.telnyx.messaging_profile_id_label'), help: t('help.telnyx.messaging_profile_id'), path: 'telnyx.messaging_profile_id', value: data.messaging_profile_id || '' }),
            form.field({ label: t('config.telnyx.connection_id_label'), help: t('help.telnyx.connection_id'), path: 'telnyx.connection_id', value: data.connection_id || '' }),
            form.field({ label: t('config.telnyx.webhook_path_label'), help: t('help.telnyx.webhook_path'), path: 'telnyx.webhook_path', value: data.webhook_path || '/api/telnyx/webhook', placeholder: '/api/telnyx/webhook' }),
            allowedNumbers
        ] }, { title: t('config.telnyx.voice_settings_title'), fields: [
            voiceLanguage,
            form.select({
                label: t('config.telnyx.voice_gender_label'),
                help: t('config.telnyx.voice_gender_help'),
                path: 'telnyx.voice_gender',
                value: data.voice_gender === 'male' ? 'male' : 'female',
                options: [{ value: 'female', label: t('config.telnyx.voice_female') }, { value: 'male', label: t('config.telnyx.voice_male') }]
            }),
            form.number({ label: t('config.telnyx.call_timeout_label'), help: t('help.telnyx.call_timeout'), path: 'telnyx.call_timeout', value: String(data.call_timeout || 300), min: 30, max: 3600 }),
            form.number({ label: t('config.telnyx.max_concurrent_calls_label'), help: t('config.telnyx.max_concurrent_calls_help'), path: 'telnyx.max_concurrent_calls', value: String(data.max_concurrent_calls || 3), min: 1, max: 20 }),
            form.number({ label: t('config.telnyx.max_sms_per_minute_label'), help: t('config.telnyx.max_sms_per_minute_help'), path: 'telnyx.max_sms_per_minute', value: String(data.max_sms_per_minute || 10), min: 1, max: 60 }),
            form.toggle({ label: t('config.telnyx.record_calls_label'), help: t('help.telnyx.record_calls'), path: 'telnyx.record_calls', value: data.record_calls === true })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save cfg-test-btn" onclick="telnyxTestConnection()" id="telnyx-test-btn" data-config-action-draft="true">' + t('config.telnyx.test_btn') + '</button>' },
            { html: form.actionResult({ id: 'telnyx-test-result' }) }
        ])
    });

    document.getElementById('content').innerHTML = html;

    if (enabled) telnyxCheckStatus();
}

function telnyxVoiceLanguageSelect(selected) {
    const languages = ['en', 'de', 'fr', 'es', 'it', 'pt', 'nl', 'ja', 'zh'];
    const customOption = typeof CFG_OPTION_OTHER_CUSTOM === 'string' ? CFG_OPTION_OTHER_CUSTOM : 'Other / Custom';
    const current = String(selected || '').trim();
    const isCustom = current && !languages.includes(current);
    let html = '<select class="field-select" data-path="telnyx.voice_language" aria-labelledby="telnyx-voice-language-label" onchange="cfgToggleCustomInput(this)">';
    languages.forEach(code => {
        html += '<option value="' + code + '"' + (current === code ? ' selected' : '') + '>' + code + '</option>';
    });
    html += '<option value="' + escapeAttr(customOption) + '"' + (isCustom ? ' selected' : '') + '>' + cfgFieldOptionLabel(customOption) + '</option>';
    html += '</select>';
    html += '<input class="field-input cfg-custom-input' + (isCustom ? '' : ' is-hidden') + '" type="text" data-custom-for="telnyx.voice_language" value="' + escapeAttr(isCustom ? current : '') + '" placeholder="en">';
    return html;
}

function telnyxCheckStatus() {
    const banner = document.getElementById('telnyx-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/telnyx/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'no_credentials') return { state: 'unconfigured', detail: t('config.telnyx.status_no_credentials') };
        if (res.status === 'ok') {
            let detail = t('config.telnyx.status_ok');
            if (res.balance) detail += ' (' + res.balance + (res.currency ? ' ' + res.currency : '') + ')';
            return { state: 'connected', detail: detail };
        }
        return { state: 'error', detail: res.message || t('config.telnyx.connection_failed') };
    });
}

function telnyxTestConnection() {
    const btn = document.getElementById('telnyx-test-btn');
    return cfgActionResult(btn, () => fetch('/api/telnyx/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['telnyx']) })
    }), {
        target: 'telnyx-test-result',
        success: t('config.telnyx.test_ok'),
        failure: t('config.telnyx.test_fail'),
        onSuccess: () => telnyxCheckStatus()
    });
}

function parseTelnyxAllowedNumbers(el) {
    const nums = el.value.split(',').map(s => s.trim()).filter(Boolean);
    setNestedValue(configData, 'telnyx.allowed_numbers', nums);
}
