// cfg/truenas.js — TrueNAS integration section module

function renderTrueNASSection(section) {
    const data = configData['truenas'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const destructiveOn = data.allow_destructive === true;
    const httpsOn = data.use_https !== false;
    const insecureOn = data.insecure_ssl === true;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.truenas.apikey_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Connection status banner ──
    html += '<div id="truenas-status-banner" class="adg-status-banner">' + t('config.truenas.checking') + '</div>';

    // ── Enabled toggle ──
    const helpEnabled = t('help.truenas.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.truenas.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="truenas.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    const helpReadonly = t('help.truenas.readonly');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.truenas.readonly_label') + '</div>';
    if (helpReadonly) html += '<div class="field-help">' + helpReadonly + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="truenas.readonly" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Allow destructive toggle ──
    const helpDestructive = t('help.truenas.allow_destructive');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.truenas.destructive_label') + '</div>';
    if (helpDestructive) html += '<div class="field-help">' + helpDestructive + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (destructiveOn ? ' on' : '') + '" data-path="truenas.allow_destructive" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (destructiveOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── HTTPS toggle ──
    const helpHTTPS = t('help.truenas.use_https');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.truenas.https_label') + '</div>';
    if (helpHTTPS) html += '<div class="field-help">' + helpHTTPS + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (httpsOn ? ' on' : '') + '" data-path="truenas.use_https" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (httpsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Insecure SSL toggle ──
    const helpInsecure = t('help.truenas.insecure_ssl');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.truenas.insecure_label') + '</div>';
    if (helpInsecure) html += '<div class="field-help">' + helpInsecure + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (insecureOn ? ' on' : '') + '" data-path="truenas.insecure_ssl" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (insecureOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Host ──
    const helpHost = t('help.truenas.host');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.truenas.host_label') + '</div>';
    if (helpHost) html += '<div class="field-help">' + helpHost + '</div>';
    html += '<input class="field-input" type="text" data-path="truenas.host" value="' + escapeAttr(data.host || '') + '" placeholder="truenas.local">';
    html += '</div>';

    // ── API Key (vault) ──
    const helpKey = t('help.truenas.api_key');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.truenas.apikey_label') + '</div>';
    if (helpKey) html += '<div class="field-help">' + helpKey + '</div>';
    html += '<div class="adg-password-row">';
        html += '<div class="password-wrap" style="flex:1;">';
        html += '<input class="field-input adg-password-input" type="password" id="truenas-apikey" value="' + escapeAttr(cfgSecretValue(data.api_key)) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="truenasSaveKey()">💾 ' + t('config.truenas.save_vault') + '</button>';
    html += '</div></div>';

    // ── Test connection button ──
    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="truenasTestConnection()" id="truenas-test-btn">🔌 ' + t('config.truenas.test_btn') + '</button>';
    html += '<span id="truenas-test-result" class="adg-test-result"></span>';
    html += '</div>';

    // ── Link to full UI ──
    html += '<div class="field-group">';
    html += '<a href="/truenas" class="btn-save" style="text-decoration: none; display: inline-block; text-align: center;">📊 ' + t('config.truenas.open_ui') + '</a>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (enabledOn && data.host) {
        truenasCheckStatus();
    }
}

function truenasSetBannerState(banner, state, text) {
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function truenasCheckStatus() {
    const banner = document.getElementById('truenas-status-banner');
    if (!banner) return;
    truenasSetBannerState(banner, 'neutral', t('config.truenas.checking'));

    fetch('/api/truenas/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                truenasSetBannerState(banner, 'neutral', '⚪ ' + t('config.truenas.status_disabled'));
                return;
            }
            if (res.status === 'error' || res.status === 'offline') {
                truenasSetBannerState(banner, 'danger', '🔴 ' + (res.error || t('config.truenas.connection_failed')));
                return;
            }
            if (res.status === 'online') {
                truenasSetBannerState(banner, 'success', '🟢 TrueNAS ' + (res.version || '') + ' — ' + res.hostname);
            }
        })
        .catch(() => {
            truenasSetBannerState(banner, 'danger', '🔴 ' + t('config.truenas.connection_failed'));
        });
}

function truenasTestConnection() {
    const btn = document.getElementById('truenas-test-btn');
    const result = document.getElementById('truenas-test-result');
    if (!btn || !result) return;

    btn.disabled = true;
    result.textContent = t('config.truenas.checking');

    fetch('/api/truenas/status')
        .then(r => r.json())
        .then(res => {
            btn.disabled = false;
            if (res.status === 'online') {
                result.textContent = '✅ ' + t('config.truenas.test_ok') + ' (' + res.version + ')';
            } else {
                result.textContent = '❌ ' + (res.error || t('config.truenas.test_fail'));
            }
        })
        .catch(() => {
            btn.disabled = false;
            result.textContent = '❌ ' + t('config.truenas.test_fail');
        });
}

function truenasSaveKey() {
    const key = document.getElementById('truenas-apikey')?.value;
    if (!key) { showToast(t('config.truenas.apikey_empty'), 'error'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'truenas_api_key', value: key })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok') {
            showToast(t('config.truenas.apikey_saved'), 'success');
            cfgMarkSecretStored(document.getElementById('truenas-apikey'), 'truenas.api_key');
        } else {
            showToast(res.message || t('config.truenas.apikey_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.truenas.apikey_save_failed'), 'error'));
}
