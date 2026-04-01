// cfg/adguard.js — AdGuard Home integration section module

function renderAdGuardSection(section) {
    const data = configData['adguard'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Connection status banner ──
    html += '<div id="adg-status-banner" class="adg-status-banner">' + t('config.adguard.checking') + '</div>';

    // ── Enabled toggle ──
    const helpEnabled = t('help.adguard.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.adguard.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="adguard.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    const helpReadonly = t('help.adguard.readonly');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.adguard.readonly_label') + '</div>';
    if (helpReadonly) html += '<div class="field-help">' + helpReadonly + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="adguard.readonly" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── URL ──
    const helpURL = t('help.adguard.url');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.adguard.url_label') + '</div>';
    if (helpURL) html += '<div class="field-help">' + helpURL + '</div>';
    html += '<input class="field-input" type="text" data-path="adguard.url" value="' + escapeAttr(data.url || '') + '" placeholder="http://192.168.1.1:3000">';
    html += '</div>';

    // ── Username ──
    const helpUser = t('help.adguard.username');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.adguard.username_label') + '</div>';
    if (helpUser) html += '<div class="field-help">' + helpUser + '</div>';
    html += '<input class="field-input" type="text" data-path="adguard.username" value="' + escapeAttr(data.username || '') + '" placeholder="admin">';
    html += '</div>';

    // ── Password (vault) ──
    const helpPass = t('help.adguard.password');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.adguard.password_label') + '</div>';
    if (helpPass) html += '<div class="field-help">' + helpPass + '</div>';
    html += '<div class="adg-password-row">';
        html += '<div class="password-wrap" style="flex:1;">';
        html += '<input class="field-input adg-password-input" type="password" id="adg-password" placeholder="' + t('config.adguard.password_placeholder') + '">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="adgSavePassword()">💾 ' + t('config.adguard.save_vault') + '</button>';
    html += '</div></div>';

    // ── Test connection button ──
    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="adgTestConnection()" id="adg-test-btn">🔌 ' + t('config.adguard.test_btn') + '</button>';
    html += '<span id="adg-test-result" class="adg-test-result"></span>';
    html += '</div>';

    // ── Quick stats (populated when connected) ──
    html += '<div id="adg-quick-stats" class="adg-quick-stats is-hidden">';
    html += '<div class="adg-stats-title">📊 ' + t('config.adguard.stats_title') + '</div>';
    html += '<div id="adg-stats-content" class="adg-stats-grid"></div>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (enabledOn && data.url) {
        adgCheckStatus();
    }
}

function adgSetBannerState(banner, state, text) {
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function adgCheckStatus() {
    const banner = document.getElementById('adg-status-banner');
    if (!banner) return;
    adgSetBannerState(banner, 'neutral', t('config.adguard.checking'));

    fetch('/api/adguard/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                adgSetBannerState(banner, 'neutral', '⚪ ' + t('config.adguard.status_disabled'));
                return;
            }
            if (res.status === 'no_url') {
                adgSetBannerState(banner, 'neutral', '⚪ ' + t('config.adguard.status_no_url'));
                return;
            }
            if (res.status === 'ok' && res.data) {
                const d = res.data;
                const version = d.version || '?';
                const running = d.running === true;
                adgSetBannerState(
                    banner,
                    running ? 'success' : 'warning',
                    (running ? '🟢' : '🟡') + ' AdGuard Home v' + version + ' — ' + (running ? t('config.adguard.running') : t('config.adguard.not_running'))
                );

                // Load quick stats
                adgLoadQuickStats();
                return;
            }
            adgSetBannerState(banner, 'danger', '🔴 ' + (res.message || t('config.adguard.connection_failed')));
        })
        .catch(() => {
            adgSetBannerState(banner, 'danger', '🔴 ' + t('config.adguard.connection_failed'));
        });
}

function adgLoadQuickStats() {
    // We reuse the status endpoint — stats require a separate call via tool
    const wrap = document.getElementById('adg-quick-stats');
    const content = document.getElementById('adg-stats-content');
    if (!wrap || !content) return;
    wrap.classList.remove('is-hidden');
    content.innerHTML = '<div class="adg-stats-loaded">' + t('config.adguard.stats_loaded') + '</div>';
}

function adgTestConnection() {
    const btn = document.getElementById('adg-test-btn');
    const result = document.getElementById('adg-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.textContent = '⏳ ...'; result.className = 'adg-test-result'; }

    fetch('/api/adguard/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = '✅ ' + t('config.adguard.test_ok');
                adgCheckStatus();
            } else {
                result.className = 'adg-test-result is-danger';
                result.textContent = '❌ ' + (res.message || t('config.adguard.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = '❌ ' + t('config.adguard.test_fail');
            }
        });
}

function adgSavePassword() {
    const input = document.getElementById('adg-password');
    const pw = input ? input.value.trim() : '';
    if (!pw) { showToast(t('config.adguard.password_empty'), 'error'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'adguard_password', value: pw })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            showToast(t('config.adguard.password_saved'), 'success');
            if (input) input.value = '';
        } else {
            showToast(res.message || t('config.adguard.password_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.adguard.password_save_failed'), 'error'));
}
