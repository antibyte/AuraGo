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
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div id="fb-status-banner" class="cfg-status-banner">' + t('config.fritzbox.checking') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabled ? ' on' : '') + '" data-path="fritzbox.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

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

    const httpsOn = data.https !== false;
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.https_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.https') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (httpsOn ? ' on' : '') + '" data-path="fritzbox.https" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (httpsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.username_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.username') + '</div>';
    html += '<input class="field-input" type="text" data-path="fritzbox.username" value="' + escapeAttr(data.username || '') + '" placeholder="">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.password_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.password') + '</div>';
    html += '<div class="cfg-field-row">';
    html += '<div class="password-wrap cfg-password-input">';
    html += '<input class="field-input cfg-password-input" type="password" id="fb-password" value="' + escapeAttr(cfgSecretValue(data.password)) + '" placeholder="' + escapeAttr(passwordPlaceholder) + '">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save cfg-save-btn-sm" onclick="fbSavePassword()">💾 ' + t('config.fritzbox.save_vault') + '</button>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<button class="btn-save cfg-save-btn-sm" onclick="fbTestConnection()" id="fb-test-btn">🔌 ' + t('config.fritzbox.test_btn') + '</button>';
    html += '<span id="fb-test-result" class="cfg-status-text"></span>';
    html += '</div>';

    html += '<hr class="cfg-section-hr">';
    html += '<div class="cfg-section-title">⚙️ ' + t('config.fritzbox.features_title') + '</div>';

    function featureGroup(key, titleKey, data) {
        const on = data.enabled === true;
        const ro = data.readonly === true;
        let h = '<div class="fb-feature-card">';
        h += '<div class="fb-feature-title">' + t(titleKey) + '</div>';
        h += '<div class="fb-feature-toggles">';
        h += '<div class="cfg-toggle-row-compact">';
        h += '<div class="toggle toggle-sm' + (on ? ' on' : '') + '" data-path="fritzbox.' + key + '.enabled" onclick="toggleBool(this)"></div>';
        h += '<span class="cfg-toggle-label">' + t('config.fritzbox.feature_enabled') + '</span>';
        h += '</div>';
        h += '<div class="cfg-toggle-row-compact">';
        h += '<div class="toggle toggle-sm' + (ro ? ' on' : '') + '" data-path="fritzbox.' + key + '.readonly" onclick="toggleBool(this)"></div>';
        h += '<span class="cfg-toggle-label">' + t('config.fritzbox.feature_readonly') + '</span>';
        h += '</div>';
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
    html += '<div class="cfg-toggle-row-compact">';
    html += '<div class="toggle toggle-sm' + (pollingEnabled ? ' on' : '') + '" data-path="fritzbox.telephony.polling.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="cfg-toggle-label">' + t('config.fritzbox.polling_enabled') + '</span>';
    html += '</div>';
    html += '<div class="cfg-input-row">';
    html += '<span class="cfg-label">' + t('config.fritzbox.polling_interval') + '</span>';
    html += '<input class="field-input fb-poll-input" type="number" min="10" max="3600" data-path="fritzbox.telephony.polling.interval_seconds" value="' + escapeAttr(String(pollingInterval)) + '">';
    html += '<span class="fb-hint">' + t('config.fritzbox.polling_seconds') + '</span>';
    html += '</div></div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;

    if (enabled && data.host) {
        fbCheckStatus();
    }
}

function fbCheckStatus() {
    const banner = document.getElementById('fb-status-banner');
    if (!banner) return;
    banner.className = 'cfg-status-banner';
    banner.textContent = t('config.fritzbox.checking');

    fetch('/api/fritzbox/status')
        .then(r => r.json())
        .then(res => {
            if (!res.enabled) {
                banner.className = 'cfg-status-banner';
                banner.textContent = '⚪ ' + t('config.fritzbox.status_disabled');
                return;
            }
            if (!res.configured) {
                banner.className = 'cfg-status-banner';
                banner.textContent = '⚪ ' + t('config.fritzbox.status_not_configured');
                return;
            }
            banner.className = 'cfg-status-banner cfg-status-success';
            banner.textContent = '🟢 ' + t('config.fritzbox.status_ok') + ' — ' + (res.host || '');
        })
        .catch(() => {
            banner.className = 'cfg-status-banner cfg-status-error';
            banner.textContent = '🔴 ' + t('config.fritzbox.status_error');
        });
}

function fbTestConnection() {
    const btn = document.getElementById('fb-test-btn');
    const result = document.getElementById('fb-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.textContent = '⏳ ...'; result.className = 'cfg-status-text'; }

    fetch('/api/fritzbox/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'cfg-status-text cfg-status-success';
                result.textContent = '✅ ' + t('config.fritzbox.test_ok') + (res.model ? ' — ' + res.model : '');
                fbCheckStatus();
            } else {
                result.className = 'cfg-status-text cfg-status-error';
                result.textContent = '❌ ' + (res.message || t('config.fritzbox.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'cfg-status-text cfg-status-error';
                result.textContent = '❌ ' + t('config.fritzbox.test_fail');
            }
        });
}

function fbSavePassword() {
    const input = document.getElementById('fb-password');
    const pw = input ? input.value.trim() : '';
    if (!pw) { showToast(t('config.fritzbox.password_empty'), 'error'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'fritzbox_password', value: pw })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            showToast(t('config.fritzbox.password_saved'), 'success');
            cfgMarkSecretStored(input, 'fritzbox.password');
        } else {
            showToast(res.message || t('config.fritzbox.password_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.fritzbox.password_save_failed'), 'error'));
}
