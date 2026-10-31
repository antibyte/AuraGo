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
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Connection status banner ──
    html += cfgStatusBanner.html('truenas-status-banner', enabledOn && data.host ? 'checking' : (enabledOn ? 'unconfigured' : 'off'), { detail: enabledOn && !(enabledOn && data.host) ? t('config.truenas.status_no_url') : '' });

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
        html += '<div class="password-wrap pw-u-flex-1">';
        html += '<input class="field-input adg-password-input" type="password" id="truenas-apikey" data-path="truenas.api_key" autocomplete="off" value="' + escapeAttr(cfgSecretValue(data.api_key)) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    html += '</div></div>';

    // ── Test connection button ──
    html += '<div class="field-group">';
    html += '<button class="btn-save adg-test-btn" onclick="truenasTestConnection()" id="truenas-test-btn" data-config-action-draft="true">' + t('config.truenas.test_btn') + '</button>';
    html += cfgActionResult.html('truenas-test-result');
    html += '</div>';

    // ── Link to full UI ──
    html += '<div class="field-group">';
    html += '<a href="/truenas" class="btn-save pw-u-button-link">📊 ' + t('config.truenas.open_ui') + '</a>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // Auto-check status on load
    if (enabledOn && data.host) truenasCheckStatus();
}

function truenasCheckStatus() {
    const banner = document.getElementById('truenas-status-banner');
    if (!banner) return;
    return cfgStatusBanner.check(banner, () => fetch('/api/truenas/status'), res => {
        if (!res) return null;
        if (res.status === 'disabled') return { state: 'off' };
        if (res.status === 'error' || res.status === 'offline') return { state: 'error', detail: res.error || t('config.truenas.connection_failed') };
        if (res.status === 'online') return { state: 'connected', detail: 'TrueNAS ' + (res.version || '') + ' — ' + (res.hostname || '') };
        if (res.status === 'no_url' || res.status === 'no_host') return { state: 'unconfigured', detail: t('config.truenas.status_no_url') };
        return null;
    });
}

// Tests the values typed in (not yet saved); the server does not persist them.
function truenasTestConnection() {
    const btn = document.getElementById('truenas-test-btn');
    return cfgActionResult(btn, () => fetch('/api/truenas/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['truenas']) })
    }), {
        target: 'truenas-test-result',
        success: t('config.truenas.test_ok'),
        failure: t('config.truenas.test_fail'),
        onSuccess: () => truenasCheckStatus()
    });
}
