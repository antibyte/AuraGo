// cfg/paperless.js — Paperless-ngx integration section module

function renderPaperlessSection(section) {
    const data = configData['paperless_ngx'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const tokenPlaceholder = cfgSecretPlaceholder(data.api_token, '••••••••••••••••••••');

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.paperless.enabled_label') + '</div>';
    const helpEnabled = t('help.paperless_ngx.enabled');
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="paperless_ngx.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.paperless.readonly_label') + '</div>';
    const helpReadonly = t('help.paperless_ngx.read_only');
    if (helpReadonly) html += '<div class="field-help">' + helpReadonly + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="paperless_ngx.readonly" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── URL ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.paperless.url_label') + '</div>';
    const helpURL = t('help.paperless_ngx.url');
    if (helpURL) html += '<div class="field-help">' + helpURL + '</div>';
    html += '<input class="field-input" type="text" data-path="paperless_ngx.url" value="' + escapeAttr(data.url || '') + '" placeholder="https://paperless.example.com" oninput="markDirty()">';
    html += '</div>';

    // ── API Token (vault) ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.paperless.token_label') + ' <span class="pl-lock-icon">🔒</span></div>';
    const helpToken = t('help.paperless_ngx.api_token');
    if (helpToken) html += '<div class="field-help">' + helpToken + '</div>';
    html += '<div class="pl-token-row">';
    // With data-path the token joins the save dock (the server writes it to the vault).
    html += '<div class="password-wrap pl-password-wrap">';
    html += '<input class="field-input" type="password" id="paperless-token-input" data-path="paperless_ngx.api_token" value="' + escapeAttr(cfgSecretValue(data.api_token)) + '" placeholder="' + escapeAttr(tokenPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '</div>';
    html += '</div>';

    // ── Test Connection ──
    html += '<div class="field-group">';
    html += '<button id="paperless-test-btn" class="btn-save pw-u-mt-4" onclick="paperlessTestConnection()" data-config-action-draft="true">🔌 ' + t('config.paperless.test_connection') + '</button>';
    html += cfgActionResult.html('paperless-test-status', 'pw-u-mt-6');
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;
}

// Tests the values typed in (unsaved included); nothing is saved by the test.
function paperlessTestConnection() {
    return cfgActionResult(document.getElementById('paperless-test-btn'), () => fetch('/api/paperless/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['paperless_ngx']) })
    }), {
        target: 'paperless-test-status',
        pending: t('config.paperless.testing'),
        success: outcome => outcome.reason || t('config.paperless.test_success'),
        failure: t('config.paperless.test_failed')
    });
}
