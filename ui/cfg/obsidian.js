// cfg/obsidian.js — Obsidian integration section module

function renderObsidianSection(section) {
    const data = configData['obsidian'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true || data.read_only === true;
    const destructiveOn = data.allow_destructive === true;
    const httpsOn = data.use_https !== false; // default true
    const insecureOn = data.insecure_ssl !== false; // default true
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.obsidian.apikey_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    const setupHint = t('help.obsidian.setup_hint');
    if (setupHint) {
        html += '<div class="field-group"><div class="field-help">' + setupHint + '</div></div>';
    }

    // ── Connection status banner ──
    html += cfgStatusBanner.html('obsidian-status-banner', enabledOn ? 'checking' : (enabledOn ? 'unconfigured' : 'off'), { detail: enabledOn && !(enabledOn) ? t('config.obsidian.status_no_url') : '' });

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
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="obsidian.readonly" onclick="toggleBool(this)"></div>';
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
    html += '<div class="field-label">' + (t('config.obsidian.connect_timeout_label')) + '</div>';
    if (helpConnectTimeout) html += '<div class="field-help">' + helpConnectTimeout + '</div>';
    html += '<input class="field-input" type="number" data-path="obsidian.connect_timeout" value="' + escapeAttr(data.connect_timeout || '10') + '" placeholder="10">';
    html += '</div>';

    // ── Request Timeout ──
    const helpRequestTimeout = t('help.obsidian.request_timeout');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + (t('config.obsidian.request_timeout_label')) + '</div>';
    if (helpRequestTimeout) html += '<div class="field-help">' + helpRequestTimeout + '</div>';
    html += '<input class="field-input" type="number" data-path="obsidian.request_timeout" value="' + escapeAttr(data.request_timeout || '30') + '" placeholder="30">';
    html += '</div>';

    // ── API Key (vault) ──
    const helpKey = t('help.obsidian.api_key');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.obsidian.apikey_label') + '</div>';
    if (helpKey) html += '<div class="field-help">' + helpKey + '</div>';
    html += '<div class="adg-password-row">';
        html += '<div class="password-wrap pw-u-flex-1">';
        html += '<input class="field-input adg-password-input" type="password" id="obsidian-apikey" data-path="obsidian.api_key" autocomplete="off" value="' + escapeAttr(cfgSecretValue(data.api_key)) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '</div></div>';

    // ── Test connection button ──
    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="obsidianTestConnection()" id="obsidian-test-btn" data-config-action-draft="true">' + t('config.obsidian.test_btn') + '</button>';
    html += cfgActionResult.html('obsidian-test-result');
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (enabledOn) obsidianCheckStatus();
}

function obsidianCheckStatus() {
    const banner = document.getElementById('obsidian-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/obsidian/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'error' || res.status === 'offline') return { state: 'error', detail: res.error || t('config.obsidian.connection_failed') };
        if (res.status === 'online') return { state: 'connected', detail: 'Obsidian REST API v' + (res.api_version || '') + ' — Obsidian ' + (res.obsidian_version || '') };
        if (res.status === 'no_url' || res.status === 'no_host') return { state: 'unconfigured', detail: t('config.obsidian.status_no_url') };
        return null;
    });
}

// Tests the values typed in (not yet saved); the server does not persist them.
function obsidianTestConnection() {
    const btn = document.getElementById('obsidian-test-btn');
    return cfgActionResult(btn, () => fetch('/api/obsidian/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['obsidian']) })
    }), {
        target: 'obsidian-test-result',
        success: t('config.obsidian.test_ok'),
        failure: t('config.obsidian.test_fail'),
        onSuccess: () => obsidianCheckStatus()
    });
}
