function renderFritzBoxSection(section) {
    const data = configData['fritzbox'] || {};
    const enabled = data.enabled === true;
    const passwordPlaceholder = cfgSecretPlaceholder(data.password, t('config.fritzbox.password_placeholder'));

    const sys = (data.system) || {};
    const net = (data.network) || {};
    const tel = (data.telephony) || {};
    const sh  = (data.smart_home) || {};
    const sto = (data.storage) || {};
    const tv  = (data.tv) || {};

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += cfgStatusBanner.html('fb-status-banner', 'checking', { detail: t('config.fritzbox.checking') });
    html += '<div class="cfg-group-title">' + t('config.refresh.operation') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabled ? ' on' : '') + '" data-path="fritzbox.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="cfg-group-title">' + t('config.refresh.connection') + '</div>';
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.host_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.host') + '</div>';
    html += '<input class="field-input" type="text" data-path="fritzbox.host" value="' + escapeAttr(data.host || 'fritz.box') + '" placeholder="fritz.box">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.port_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.port') + '</div>';
    html += '<input class="field-input" type="number" data-path="fritzbox.port" value="' + escapeAttr(String(data.port || '49000')) + '" placeholder="49000">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.web_port_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.web_port') + '</div>';
    html += '<input class="field-input" type="number" min="0" data-path="fritzbox.web_port" value="' + escapeAttr(String(data.web_port || '0')) + '" placeholder="0">';
    html += '</div>';

    const httpsOn = data.https === true;
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.https_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.https') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (httpsOn ? ' on' : '') + '" data-path="fritzbox.https" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (httpsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    const insecureOn = data.insecure_skip_verify === true;
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.insecure_skip_verify_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.insecure_skip_verify') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (insecureOn ? ' on' : '') + '" data-path="fritzbox.insecure_skip_verify" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (insecureOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="cfg-group-title">' + t('config.refresh.credentials') + '</div>';
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.username_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.username') + '</div>';
    html += '<input class="field-input" type="text" data-path="fritzbox.username" value="' + escapeAttr(data.username || '') + '" placeholder="">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.password_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.password') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap cfg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="fb-password" data-path="fritzbox.password" value="' + escapeAttr(cfgSecretValue(data.password)) + '" placeholder="' + escapeAttr(passwordPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '</div></div>';

    html += '<div class="cfg-actions-row">';
    html += '<button class="btn-save adg-test-btn" onclick="fbTestConnection()" id="fb-test-btn" data-config-action-draft="true">🔌 ' + t('config.fritzbox.test_btn') + '</button>';
    html += cfgActionResult.html('fb-test-result');
    html += '</div>';

    html += '<hr class="cfg-section-hr">';
    html += '<div class="cfg-section-title">⚙️ ' + t('config.fritzbox.features_title') + '</div>';

    function featureGroup(key, titleKey, data) {
        const on = data.enabled === true;
        const ro = data.readonly === true;
        let h = '<div class="fb-feature-card">';
        const feature = t(titleKey).replace(/^[^\p{L}]+/u, '');
        h += '<div class="fb-feature-title" id="fb-feature-title-' + key + '">' + t(titleKey) + '</div>';
        h += '<div class="fb-feature-toggles">';
        // Each switch is named by its card title plus its own label and carries its own description.
        h += '<div class="field-group"><div class="cfg-toggle-row-compact">';
        h += '<div class="toggle toggle-sm' + (on ? ' on' : '') + '" data-path="fritzbox.' + key + '.enabled" aria-labelledby="fb-feature-title-' + key + ' fb-feature-' + key + '-enabled" onclick="toggleBool(this)"></div>';
        h += '<span class="cfg-toggle-label" id="fb-feature-' + key + '-enabled">' + t('config.fritzbox.feature_enabled') + '</span>';
        h += '</div><div class="field-help">' + t('config.fritzbox.feature_enabled_help', { feature }) + '</div></div>';
        h += '<div class="field-group"><div class="cfg-toggle-row-compact">';
        h += '<div class="toggle toggle-sm' + (ro ? ' on' : '') + '" data-path="fritzbox.' + key + '.readonly" aria-labelledby="fb-feature-title-' + key + ' fb-feature-' + key + '-readonly" onclick="toggleBool(this)"></div>';
        h += '<span class="cfg-toggle-label" id="fb-feature-' + key + '-readonly">' + t('config.fritzbox.feature_readonly') + '</span>';
        h += '</div><div class="field-help">' + t('config.fritzbox.feature_readonly_help', { feature }) + '</div></div>';
        h += '</div></div>';
        return h;
    }

    html += featureGroup('system',     'config.fritzbox.group_system',     sys);
    html += featureGroup('network',    'config.fritzbox.group_network',    net);
    html += featureGroup('telephony',  'config.fritzbox.group_telephony',  tel);
    html += featureGroup('smart_home', 'config.fritzbox.group_smarthome',  sh);
    html += featureGroup('storage',    'config.fritzbox.group_storage',    sto);
    html += featureGroup('tv',         'config.fritzbox.group_tv',         tv);

    const pollingEnabled = (tel.polling || {}).enabled === true;
    const pollingInterval = (tel.polling || {}).interval_seconds || 60;
    html += '<div class="fb-feature-card">';
    html += '<div class="fb-feature-title">📞 ' + t('config.fritzbox.group_polling') + '</div>';
    html += '<div class="field-group"><div class="cfg-toggle-row-compact">';
    html += '<div class="toggle toggle-sm' + (pollingEnabled ? ' on' : '') + '" data-path="fritzbox.telephony.polling.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="cfg-toggle-label">' + t('config.fritzbox.polling_enabled') + '</span>';
    html += '</div><div class="field-help">' + t('config.fritzbox.polling_enabled_help') + '</div></div>';
    html += '<div class="cfg-input-row">';
    html += '<label class="cfg-label" for="fb-poll-interval">' + t('config.fritzbox.polling_interval') + '</label>';
    html += '<input class="field-input fb-poll-input" id="fb-poll-interval" type="number" min="10" max="3600" data-path="fritzbox.telephony.polling.interval_seconds" aria-describedby="fb-poll-interval-help" value="' + escapeAttr(String(pollingInterval)) + '">';
    html += '<span class="fb-hint">' + t('config.fritzbox.polling_seconds') + '</span>';
    html += '</div><div class="field-help" id="fb-poll-interval-help">' + t('config.fritzbox.polling_interval_help') + '</div></div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    if (enabled) {
        if (data.host) fbCheckStatus();
        else fbSetBanner('unconfigured', t('config.fritzbox.status_not_configured'));
    } else {
        fbSetBanner('off', t('config.fritzbox.status_disabled'));
    }
}

function fbSetBanner(state, text) {
    cfgStatusBanner('fb-status-banner', state, { detail: text, retry: state === 'error' || state === 'unknown' ? fbCheckStatus : null });
}

// Live check of the saved configuration; request errors and timeouts are handled by cfgStatusBanner.check.
function fbCheckStatus() {
    if (!document.getElementById('fb-status-banner')) return;
    return cfgStatusBanner.check('fb-status-banner', signal => fetch('/api/fritzbox/status?check=1', { signal }), res => {
        if (!res.enabled) return { state: 'off', detail: t('config.fritzbox.status_disabled') };
        if (!res.configured) return { state: 'unconfigured', detail: t('config.fritzbox.status_not_configured') };
        if (res.connected) return { state: 'connected', detail: t('config.fritzbox.status_ok') + ' — ' + (res.model || res.host || '') };
        return { state: 'error', detail: res.message || t('config.fritzbox.status_error') };
    }, { checkingDetail: t('config.fritzbox.checking') });
}

// Tests with the values typed in; the password follows the target (stored one only while it is unchanged).
function fbTestConnection() {
    const btn = document.getElementById('fb-test-btn');
    const body = { draft: cfgDraftPatch(['fritzbox']) };
    return cfgActionResult(btn, () => fetch('/api/fritzbox/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    }), {
        target: 'fb-test-result',
        pending: t('config.fritzbox.testing'),
        success: o => t('config.fritzbox.test_ok') + (o.body && o.body.model ? ' — ' + o.body.model : ''),
        failure: t('config.fritzbox.test_fail'),
        onSuccess: outcome => {
            const res = outcome.body || {};
            if (res.status === 'ok') {
                fbSetBanner('connected', t('config.fritzbox.status_ok') + (res.model ? ' — ' + res.model : ''));
            } else {
                fbSetBanner('degraded', outcome.reason);
            }
        }
    });
}
