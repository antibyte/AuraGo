// cfg/obsidian.js — Obsidian integration section module

function renderObsidianSection(section) {
    const data = configData['obsidian'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.read_only === true;
    const destructiveOn = data.allow_destructive === true;
    const httpsOn = data.use_https !== false; // default true
    const insecureOn = data.insecure_ssl !== false; // default true
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.obsidian.apikey_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    const setupHint = t('help.obsidian.setup_hint');
    if (setupHint) {
        html += '<div class="field-group"><div class="field-help">' + setupHint + '</div></div>';
    }

    // ── Connection status banner ──
    html += '<div id="obsidian-status-banner" class="adg-status-banner">' + t('config.obsidian.checking') + '</div>';

    // ── Enabled toggle ──
    const helpEnabled = t('help.obsidian.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.obsidian.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="obsidian.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    const helpReadonly = t('help.obsidian.readonly');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.obsidian.readonly_label') + '</div>';
    if (helpReadonly) html += '<div class="field-help">' + helpReadonly + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="obsidian.read_only" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Allow destructive toggle ──
    const helpDestructive = t('help.obsidian.allow_destructive');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.obsidian.destructive_label') + '</div>';
    if (helpDestructive) html += '<div class="field-help">' + helpDestructive + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (destructiveOn ? ' on' : '') + '" data-path="obsidian.allow_destructive" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (destructiveOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Host ──
    const helpHost = t('help.obsidian.host');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.obsidian.host_label') + '</div>';
    if (helpHost) html += '<div class="field-help">' + helpHost + '</div>';
    html += '<input class="field-input" type="text" data-path="obsidian.host" value="' + escapeAttr(data.host || '') + '" placeholder="127.0.0.1">';
    html += '</div>';

    // ── Port ──
    const helpPort = t('help.obsidian.port');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.obsidian.port_label') + '</div>';
    if (helpPort) html += '<div class="field-help">' + helpPort + '</div>';
    html += '<input class="field-input" type="number" data-path="obsidian.port" value="' + escapeAttr(data.port || '27124') + '" placeholder="27124">';
    html += '</div>';

    // ── HTTPS toggle ──
    const helpHTTPS = t('help.obsidian.use_https');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.obsidian.https_label') + '</div>';
    if (helpHTTPS) html += '<div class="field-help">' + helpHTTPS + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (httpsOn ? ' on' : '') + '" data-path="obsidian.use_https" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (httpsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Insecure SSL toggle ──
    const helpInsecure = t('help.obsidian.insecure_ssl');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.obsidian.insecure_label') + '</div>';
    if (helpInsecure) html += '<div class="field-help">' + helpInsecure + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (insecureOn ? ' on' : '') + '" data-path="obsidian.insecure_ssl" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (insecureOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Connect Timeout ──
    const helpConnectTimeout = t('help.obsidian.connect_timeout');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + (t('config.obsidian.connect_timeout_label') || 'Connect Timeout') + '</div>';
    if (helpConnectTimeout) html += '<div class="field-help">' + helpConnectTimeout + '</div>';
    html += '<input class="field-input" type="number" data-path="obsidian.connect_timeout" value="' + escapeAttr(data.connect_timeout || '10') + '" placeholder="10">';
    html += '</div>';

    // ── Request Timeout ──
    const helpRequestTimeout = t('help.obsidian.request_timeout');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + (t('config.obsidian.request_timeout_label') || 'Request Timeout') + '</div>';
    if (helpRequestTimeout) html += '<div class="field-help">' + helpRequestTimeout + '</div>';
    html += '<input class="field-input" type="number" data-path="obsidian.request_timeout" value="' + escapeAttr(data.request_timeout || '30') + '" placeholder="30">';
    html += '</div>';

    // ── API Key (vault) ──
    const helpKey = t('help.obsidian.api_key');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.obsidian.apikey_label') + '</div>';
    if (helpKey) html += '<div class="field-help">' + helpKey + '</div>';
    html += '<div class="adg-password-row">';
        html += '<div class="password-wrap" style="flex:1;">';
        html += '<input class="field-input adg-password-input" type="password" id="obsidian-apikey" value="' + escapeAttr(cfgSecretValue(data.api_key)) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="obsidianSaveKey()">💾 ' + t('config.obsidian.save_vault') + '</button>';
    html += '</div></div>';

    // ── Test connection button ──
    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="obsidianTestConnection()" id="obsidian-test-btn">🔌 ' + t('config.obsidian.test_btn') + '</button>';
    html += '<span id="obsidian-test-result" class="adg-test-result"></span>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (enabledOn) {
        obsidianCheckStatus();
    }
}

function obsidianSetBannerState(banner, state, text) {
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function obsidianCheckStatus() {
    const banner = document.getElementById('obsidian-status-banner');
    if (!banner) return;
    obsidianSetBannerState(banner, 'neutral', t('config.obsidian.checking'));

    fetch('/api/obsidian/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                obsidianSetBannerState(banner, 'neutral', '⚪ ' + t('config.obsidian.status_disabled'));
                return;
            }
            if (res.status === 'error' || res.status === 'offline') {
                obsidianSetBannerState(banner, 'danger', '🔴 ' + (res.error || t('config.obsidian.connection_failed')));
                return;
            }
            if (res.status === 'online') {
                obsidianSetBannerState(banner, 'success', '🟢 Obsidian REST API v' + (res.api_version || '') + ' — Obsidian ' + (res.obsidian_version || ''));
            }
        })
        .catch(() => {
            obsidianSetBannerState(banner, 'danger', '🔴 ' + t('config.obsidian.connection_failed'));
        });
}

function obsidianTestConnection() {
    const btn = document.getElementById('obsidian-test-btn');
    const result = document.getElementById('obsidian-test-result');
    if (!btn || !result) return;

    btn.disabled = true;
    result.textContent = t('config.obsidian.checking');

    fetch('/api/obsidian/status')
        .then(r => r.json())
        .then(res => {
            btn.disabled = false;
            if (res.status === 'online') {
                result.textContent = '✅ ' + t('config.obsidian.test_ok') + ' (API v' + (res.api_version || '') + ')';
            } else {
                result.textContent = '❌ ' + (res.error || t('config.obsidian.test_fail'));
            }
        })
        .catch(() => {
            btn.disabled = false;
            result.textContent = '❌ ' + t('config.obsidian.test_fail');
        });
}

function obsidianSaveKey() {
    const key = document.getElementById('obsidian-apikey')?.value;
    if (!key) { showToast(t('config.obsidian.apikey_empty'), 'error'); return; }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'obsidian_api_key', value: key })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok') {
            showToast(t('config.obsidian.apikey_saved'), 'success');
            cfgMarkSecretStored(document.getElementById('obsidian-apikey'), 'obsidian.api_key');
        } else {
            showToast(res.message || t('config.obsidian.apikey_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.obsidian.apikey_save_failed'), 'error'));
}
