// cfg/jellyfin.js — Jellyfin integration section module

function renderJellyfinSection(section) {
    const data = configData['jellyfin'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true || data.read_only === true;
    const destructiveOn = data.allow_destructive === true;
    const httpsOn = data.use_https === true;
    const insecureOn = data.insecure_ssl === true;
    const apiKeyPlaceholder = cfgSecretPlaceholder(data.api_key, t('config.jellyfin.apikey_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Connection status banner ──
    html += cfgStatusBanner.html('jellyfin-status-banner', enabledOn && data.host ? 'checking' : (enabledOn ? 'unconfigured' : 'off'), { detail: enabledOn && !(enabledOn && data.host) ? t('config.jellyfin.status_no_url') : '' });

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
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="jellyfin.readonly" onclick="toggleBool(this)"></div>';
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
        html += '<div class="password-wrap pw-u-flex-1">';
        html += '<input class="field-input adg-password-input" type="password" id="jellyfin-apikey" data-path="jellyfin.api_key" autocomplete="off" value="' + escapeAttr(cfgSecretValue(data.api_key)) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '</div></div>';

    // ── Test connection button ──
    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="jellyfinTestConnection()" id="jellyfin-test-btn" data-config-action-draft="true">' + t('config.jellyfin.test_btn') + '</button>';
    html += cfgActionResult.html('jellyfin-test-result');
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (enabledOn && data.host) jellyfinCheckStatus();
}

function jellyfinCheckStatus() {
    const banner = document.getElementById('jellyfin-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/jellyfin/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'error' || res.status === 'offline') return { state: 'error', detail: res.error || t('config.jellyfin.connection_failed') };
        if (res.status === 'online') return { state: 'connected', detail: t('config.jellyfin.product_name') + ' ' + (res.version || '') + ' — ' + (res.server_name || '') };
        if (res.status === 'no_url' || res.status === 'no_host') return { state: 'unconfigured', detail: t('config.jellyfin.status_no_url') };
        return null;
    });
}

// Tests the values typed in (not yet saved); the server does not persist them.
function jellyfinTestConnection() {
    const btn = document.getElementById('jellyfin-test-btn');
    return cfgActionResult(btn, () => fetch('/api/jellyfin/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['jellyfin']) })
    }), {
        target: 'jellyfin-test-result',
        success: t('config.jellyfin.test_ok'),
        failure: t('config.jellyfin.test_fail'),
        onSuccess: () => jellyfinCheckStatus()
    });
}
