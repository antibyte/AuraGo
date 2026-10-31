function renderFritzBoxSection(section) {
    const data = configData['fritzbox'] || {};
    const enabled = data.enabled === true;
    const passwordPlaceholder = cfgSecretPlaceholder(data.password, t('config.fritzbox.password_placeholder'));
    const form = window.AuraConfigForm;

    const sys = (data.system) || {};
    const net = (data.network) || {};
    const tel = (data.telephony) || {};
    const sh  = (data.smart_home) || {};
    const sto = (data.storage) || {};
    const tv  = (data.tv) || {};

    function featureGroup(key, titleKey, icon, data) {
        const on = data.enabled === true;
        const ro = data.readonly === true;
        let h = '<div class="fb-feature-card">';
        const feature = t(titleKey);
        h += '<div class="fb-feature-title" id="fb-feature-title-' + key + '">' + cfgIcon(icon) + '<span>' + escapeHtml(feature) + '</span></div>';
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

    let features = '';
    features += featureGroup('system',     'config.fritzbox.group_system',    'monitor', sys);
    features += featureGroup('network',    'config.fritzbox.group_network',   'wifi',    net);
    features += featureGroup('telephony',  'config.fritzbox.group_telephony', 'phone',   tel);
    features += featureGroup('smart_home', 'config.fritzbox.group_smarthome', 'home',    sh);
    features += featureGroup('storage',    'config.fritzbox.group_storage',   'database', sto);
    features += featureGroup('tv',         'config.fritzbox.group_tv',        'video',   tv);

    const pollingEnabled = (tel.polling || {}).enabled === true;
    const pollingInterval = (tel.polling || {}).interval_seconds || 60;
    features += '<div class="fb-feature-card">';
    features += '<div class="fb-feature-title">' + cfgIcon('phone') + '<span>' + escapeHtml(t('config.fritzbox.group_polling')) + '</span></div>';
    features += '<div class="field-group"><div class="cfg-toggle-row-compact">';
    features += '<div class="toggle toggle-sm' + (pollingEnabled ? ' on' : '') + '" data-path="fritzbox.telephony.polling.enabled" onclick="toggleBool(this)"></div>';
    features += '<span class="cfg-toggle-label">' + t('config.fritzbox.polling_enabled') + '</span>';
    features += '</div><div class="field-help">' + t('config.fritzbox.polling_enabled_help') + '</div></div>';
    features += '<div class="cfg-input-row">';
    features += '<label class="cfg-label" for="fb-poll-interval">' + t('config.fritzbox.polling_interval') + '</label>';
    features += '<input class="field-input fb-poll-input" id="fb-poll-interval" type="number" min="10" max="3600" data-path="fritzbox.telephony.polling.interval_seconds" aria-describedby="fb-poll-interval-help" value="' + escapeAttr(String(pollingInterval)) + '">';
    features += '<span class="fb-hint">' + t('config.fritzbox.polling_seconds') + '</span>';
    features += '</div><div class="field-help" id="fb-poll-interval-help">' + t('config.fritzbox.polling_interval_help') + '</div></div>';

    const html = form.renderSpec({
        label: section.label,
        desc: section.desc,
        beforeHTML: cfgStatusBanner.html('fb-status-banner', 'checking', { detail: t('config.fritzbox.checking') }),
        groups: [{ titleKey: 'config.refresh.operation', fields: [
            form.toggle({ label: t('config.fritzbox.enabled_label'), help: t('help.fritzbox.enabled'), path: 'fritzbox.enabled', value: enabled })
        ] }, { titleKey: 'config.refresh.connection', fields: [
            form.field({ label: t('config.fritzbox.host_label'), help: t('help.fritzbox.host'), path: 'fritzbox.host', value: data.host || 'fritz.box', placeholder: 'fritz.box' }),
            form.number({ label: t('config.fritzbox.port_label'), help: t('help.fritzbox.port'), path: 'fritzbox.port', value: String(data.port || '49000'), min: 1, max: 65535, step: 1 }),
            form.number({ label: t('config.fritzbox.web_port_label'), help: t('help.fritzbox.web_port'), path: 'fritzbox.web_port', value: String(data.web_port || '0'), min: 0, max: 65535, step: 1 }),
            form.toggle({ label: t('config.fritzbox.https_label'), help: t('help.fritzbox.https'), path: 'fritzbox.https', value: data.https === true }),
            form.toggle({ label: t('config.fritzbox.insecure_skip_verify_label'), help: t('help.fritzbox.insecure_skip_verify'), path: 'fritzbox.insecure_skip_verify', value: data.insecure_skip_verify === true })
        ] }, { titleKey: 'config.refresh.credentials', fields: [
            form.field({ label: t('config.fritzbox.username_label'), help: t('help.fritzbox.username'), path: 'fritzbox.username', value: data.username || '' }),
            form.password({
                label: t('config.fritzbox.password_label'),
                help: t('help.fritzbox.password'),
                id: 'fb-password',
                path: 'fritzbox.password',
                value: data.password,
                placeholder: passwordPlaceholder
            })
        ] }],
        afterHTML: form.actions([
            { html: '<button class="btn-save cfg-test-btn" onclick="fbTestConnection()" id="fb-test-btn" data-config-action-draft="true">' + cfgIcon('plug') + '<span>' + t('config.fritzbox.test_btn') + '</span></button>' },
            { html: cfgActionResult.html('fb-test-result') }
        ]) + form.panel({ title: t('config.fritzbox.features_title'), content: features })
    });

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
