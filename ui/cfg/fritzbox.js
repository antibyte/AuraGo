// cfg/fritzbox.js — Fritz!Box integration config panel

function renderFritzBoxSection(section) {
    const data = configData['fritzbox'] || {};
    const enabled = data.enabled === true;

    // Feature groups
    const sys = (data.system) || {};
    const net = (data.network) || {};
    const tel = (data.telephony) || {};
    const sh  = (data.smart_home) || {};
    const sto = (data.storage) || {};
    const tv  = (data.tv) || {};

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Connection status banner ──
    html += '<div id="fb-status-banner" style="margin-bottom:1rem;padding:0.8rem 1rem;border-radius:10px;font-size:0.84rem;background:var(--bg-tertiary);color:var(--text-secondary);">' + t('config.fritzbox.checking') + '</div>';

    // ── Master enabled toggle ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabled ? ' on' : '') + '" data-path="fritzbox.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Host ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.host_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.host') + '</div>';
    html += '<input class="field-input" type="text" data-path="fritzbox.host" value="' + escapeAttr(data.host || 'fritz.box') + '" placeholder="fritz.box">';
    html += '</div>';

    // ── Port ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.port_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.port') + '</div>';
    html += '<input class="field-input" type="number" data-path="fritzbox.port" value="' + escapeAttr(String(data.port || '49000')) + '" placeholder="49000">';
    html += '</div>';

    // ── HTTPS toggle ──
    const httpsOn = data.https !== false; // default true
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.https_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.https') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (httpsOn ? ' on' : '') + '" data-path="fritzbox.https" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (httpsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Username ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.username_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.username') + '</div>';
    html += '<input class="field-input" type="text" data-path="fritzbox.username" value="' + escapeAttr(data.username || '') + '" placeholder="">';
    html += '</div>';

    // ── Password (vault) ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.fritzbox.password_label') + '</div>';
    html += '<div class="field-help">' + t('help.fritzbox.password') + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;">';
    html += '<input class="field-input" type="password" id="fb-password" placeholder="' + t('config.fritzbox.password_placeholder') + '" style="flex:1;">';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;" onclick="fbSavePassword()">💾 ' + t('config.fritzbox.save_vault') + '</button>';
    html += '</div></div>';

    // ── Test connection ──
    html += '<div class="field-group">';
    html += '<button class="btn-save" style="padding:0.5rem 1.2rem;font-size:0.85rem;" onclick="fbTestConnection()" id="fb-test-btn">🔌 ' + t('config.fritzbox.test_btn') + '</button>';
    html += '<span id="fb-test-result" style="margin-left:0.8rem;font-size:0.83rem;"></span>';
    html += '</div>';

    // ── Feature groups ──────────────────────────────────────────────────────
    html += '<hr style="border:none;border-top:1px solid var(--border-subtle);margin:1.5rem 0;">';
    html += '<div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:1rem;">⚙️ ' + t('config.fritzbox.features_title') + '</div>';

    // Helper: render a feature group with enabled + readonly toggles
    function featureGroup(key, titleKey, data) {
        const on = data.enabled === true;
        const ro = data.readonly === true;
        let h = '<div style="background:var(--bg-tertiary);border-radius:10px;padding:1rem;margin-bottom:0.8rem;">';
        h += '<div style="font-weight:600;font-size:0.88rem;color:var(--text-primary);margin-bottom:0.7rem;">' + t(titleKey) + '</div>';
        h += '<div style="display:flex;gap:1.5rem;flex-wrap:wrap;">';
        // Enabled
        h += '<div style="display:flex;align-items:center;gap:0.5rem;">';
        h += '<div class="toggle toggle-sm' + (on ? ' on' : '') + '" data-path="fritzbox.' + key + '.enabled" onclick="toggleBool(this)"></div>';
        h += '<span style="font-size:0.82rem;color:var(--text-secondary);">' + t('config.fritzbox.feature_enabled') + '</span>';
        h += '</div>';
        // Read-only
        h += '<div style="display:flex;align-items:center;gap:0.5rem;">';
        h += '<div class="toggle toggle-sm' + (ro ? ' on' : '') + '" data-path="fritzbox.' + key + '.readonly" onclick="toggleBool(this)"></div>';
        h += '<span style="font-size:0.82rem;color:var(--text-secondary);">' + t('config.fritzbox.feature_readonly') + '</span>';
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

    // ── Telephony polling ──
    const pollingEnabled = (tel.polling || {}).enabled === true;
    const pollingInterval = (tel.polling || {}).interval_seconds || 60;
    html += '<div style="background:var(--bg-tertiary);border-radius:10px;padding:1rem;margin-bottom:0.8rem;">';
    html += '<div style="font-weight:600;font-size:0.88rem;color:var(--text-primary);margin-bottom:0.7rem;">📞 ' + t('config.fritzbox.group_polling') + '</div>';
    html += '<div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.6rem;">';
    html += '<div class="toggle toggle-sm' + (pollingEnabled ? ' on' : '') + '" data-path="fritzbox.telephony.polling.enabled" onclick="toggleBool(this)"></div>';
    html += '<span style="font-size:0.82rem;color:var(--text-secondary);">' + t('config.fritzbox.polling_enabled') + '</span>';
    html += '</div>';
    html += '<div style="display:flex;align-items:center;gap:0.6rem;">';
    html += '<span style="font-size:0.82rem;color:var(--text-secondary);">' + t('config.fritzbox.polling_interval') + '</span>';
    html += '<input class="field-input" type="number" min="10" max="3600" data-path="fritzbox.telephony.polling.interval_seconds" value="' + escapeAttr(String(pollingInterval)) + '" style="width:90px;">';
    html += '<span style="font-size:0.8rem;color:var(--text-tertiary);">' + t('config.fritzbox.polling_seconds') + '</span>';
    html += '</div></div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;

    // Auto-check status if enabled and host set
    if (enabled && data.host) {
        fbCheckStatus();
    }
}

function fbCheckStatus() {
    const banner = document.getElementById('fb-status-banner');
    if (!banner) return;
    banner.style.background = 'var(--bg-tertiary)';
    banner.style.color = 'var(--text-secondary)';
    banner.textContent = t('config.fritzbox.checking');

    fetch('/api/fritzbox/status')
        .then(r => r.json())
        .then(res => {
            if (!res.enabled) {
                banner.textContent = '⚪ ' + t('config.fritzbox.status_disabled');
                return;
            }
            if (!res.configured) {
                banner.textContent = '⚪ ' + t('config.fritzbox.status_not_configured');
                return;
            }
            banner.style.background = 'rgba(52,199,89,0.1)';
            banner.style.color = 'var(--success, #34c759)';
            banner.textContent = '🟢 ' + t('config.fritzbox.status_ok') + ' — ' + (res.host || '');
        })
        .catch(() => {
            banner.style.background = 'rgba(255,59,48,0.1)';
            banner.style.color = 'var(--error, #ff3b30)';
            banner.textContent = '🔴 ' + t('config.fritzbox.status_error');
        });
}

function fbTestConnection() {
    const btn = document.getElementById('fb-test-btn');
    const result = document.getElementById('fb-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.textContent = '⏳ ...'; result.style.color = 'var(--text-secondary)'; }

    fetch('/api/fritzbox/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.style.color = 'var(--success, #34c759)';
                result.textContent = '✅ ' + t('config.fritzbox.test_ok') + (res.model ? ' — ' + res.model : '');
                fbCheckStatus();
            } else {
                result.style.color = 'var(--error, #ff3b30)';
                result.textContent = '❌ ' + (res.message || t('config.fritzbox.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.style.color = 'var(--error, #ff3b30)';
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
            if (input) input.value = '';
        } else {
            showToast(res.message || t('config.fritzbox.password_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.fritzbox.password_save_failed'), 'error'));
}
