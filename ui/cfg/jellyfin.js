// cfg/jellyfin.js — Jellyfin integration section module

function renderJellyfinSection(section) {
    const data = configData['jellyfin'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.read_only === true;
    const destructiveOn = data.allow_destructive === true;
    const httpsOn = data.use_https === true;
    const insecureOn = data.insecure_ssl === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Connection status banner ──
    html += '<div id="jellyfin-status-banner" class="adg-status-banner">' + t('config.jellyfin.checking') + '</div>';

    // ── Enabled toggle ──
    const helpEnabled = t('help.jellyfin.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.jellyfin.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="jellyfin.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    const helpReadonly = t('help.jellyfin.readonly');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.jellyfin.readonly_label') + '</div>';
    if (helpReadonly) html += '<div class="field-help">' + helpReadonly + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="jellyfin.read_only" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Allow destructive toggle ──
    const helpDestructive = t('help.jellyfin.allow_destructive');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.jellyfin.destructive_label') + '</div>';
    if (helpDestructive) html += '<div class="field-help">' + helpDestructive + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (destructiveOn ? ' on' : '') + '" data-path="jellyfin.allow_destructive" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (destructiveOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Host ──
    const helpHost = t('help.jellyfin.host');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.jellyfin.host_label') + '</div>';
    if (helpHost) html += '<div class="field-help">' + helpHost + '</div>';
    html += '<input class="field-input" type="text" data-path="jellyfin.host" value="' + escapeAttr(data.host || '') + '" placeholder="jellyfin.local">';
    html += '</div>';

    // ── Port ──
    const helpPort = t('help.jellyfin.port');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.jellyfin.port_label') + '</div>';
    if (helpPort) html += '<div class="field-help">' + helpPort + '</div>';
    html += '<input class="field-input" type="number" data-path="jellyfin.port" value="' + escapeAttr(data.port || '8096') + '" placeholder="8096">';
    html += '</div>';

    // ── HTTPS toggle ──
    const helpHTTPS = t('help.jellyfin.use_https');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.jellyfin.https_label') + '</div>';
    if (helpHTTPS) html += '<div class="field-help">' + helpHTTPS + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (httpsOn ? ' on' : '') + '" data-path="jellyfin.use_https" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (httpsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Insecure SSL toggle ──
    const helpInsecure = t('help.jellyfin.insecure_ssl');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.jellyfin.insecure_label') + '</div>';
    if (helpInsecure) html += '<div class="field-help">' + helpInsecure + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (insecureOn ? ' on' : '') + '" data-path="jellyfin.insecure_ssl" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (insecureOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── API Key (vault) ──
    const helpKey = t('help.jellyfin.api_key');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.jellyfin.apikey_label') + '</div>';
    if (helpKey) html += '<div class="field-help">' + helpKey + '</div>';
    html += '<div class="adg-password-row">';
        html += '<div class="password-wrap" style="flex:1;">';
        html += '<input class="field-input adg-password-input" type="password" id="jellyfin-apikey" placeholder="' + t('config.jellyfin.apikey_placeholder') + '">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="jellyfinSaveKey()">💾 ' + t('config.jellyfin.save_vault') + '</button>';
    html += '</div></div>';

    // ── Test connection button ──
    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="jellyfinTestConnection()" id="jellyfin-test-btn">🔌 ' + t('config.jellyfin.test_btn') + '</button>';
    html += '<span id="jellyfin-test-result" class="adg-test-result"></span>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (enabledOn && data.host) {
        jellyfinCheckStatus();
    }
}

function jellyfinSetBannerState(banner, state, text) {
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function jellyfinCheckStatus() {
    const banner = document.getElementById('jellyfin-status-banner');
    if (!banner) return;
    jellyfinSetBannerState(banner, 'neutral', t('config.jellyfin.checking'));

    fetch('/api/jellyfin/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                jellyfinSetBannerState(banner, 'neutral', '⚪ ' + t('config.jellyfin.status_disabled'));
                return;
            }
            if (res.status === 'error' || res.status === 'offline') {
                jellyfinSetBannerState(banner, 'danger', '🔴 ' + (res.error || t('config.jellyfin.connection_failed')));
                return;
            }
            if (res.status === 'online') {
                jellyfinSetBannerState(banner, 'success', '🟢 ' + t('config.jellyfin.product_name') + ' ' + (res.version || '') + ' — ' + (res.server_name || ''));
            }
        })
        .catch(() => {
            jellyfinSetBannerState(banner, 'danger', '🔴 ' + t('config.jellyfin.connection_failed'));
        });
}

function jellyfinTestConnection() {
    const btn = document.getElementById('jellyfin-test-btn');
    const result = document.getElementById('jellyfin-test-result');
    if (!btn || !result) return;

    btn.disabled = true;
    result.textContent = t('config.jellyfin.checking');

    fetch('/api/jellyfin/status')
        .then(r => r.json())
        .then(res => {
            btn.disabled = false;
            if (res.status === 'online') {
                result.textContent = '✅ ' + t('config.jellyfin.test_ok') + ' (' + (res.version || '') + ')';
            } else {
                result.textContent = '❌ ' + (res.error || t('config.jellyfin.test_fail'));
            }
        })
        .catch(() => {
            btn.disabled = false;
            result.textContent = '❌ ' + t('config.jellyfin.test_fail');
        });
}

function jellyfinSaveKey() {
    const key = document.getElementById('jellyfin-apikey')?.value;
    if (!key) { showToast(t('config.jellyfin.apikey_empty'), 'error'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'jellyfin_api_key', value: key })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok') {
            showToast(t('config.jellyfin.apikey_saved'), 'success');
            document.getElementById('jellyfin-apikey').value = '';
        } else {
            showToast(res.message || t('config.jellyfin.apikey_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.jellyfin.apikey_save_failed'), 'error'));
}
