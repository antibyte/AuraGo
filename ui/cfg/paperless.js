// cfg/paperless.js — Paperless-ngx integration section module

function renderPaperlessSection(section) {
    const data = configData['paperless_ngx'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
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
    html += '<div class="field-label">' + t('config.paperless.token_label') + ' <span style="font-size:0.65rem;color:var(--warning);">🔒</span></div>';
    const helpToken = t('help.paperless_ngx.api_token');
    if (helpToken) html += '<div class="field-help">' + helpToken + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;">';
    html += '<div class="password-wrap" style="flex:1;">';
    html += '<input class="field-input" type="password" id="paperless-token-input" placeholder="••••••••••••••••••••" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;white-space:nowrap;" onclick="paperlessSaveToken()">💾 ' + t('config.paperless.save_vault') + '</button>';
    html += '</div>';
    html += '<div id="paperless-token-status" style="margin-top:0.4rem;font-size:0.78rem;display:none;"></div>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;
}

function paperlessSaveToken() {
    const input = document.getElementById('paperless-token-input');
    const status = document.getElementById('paperless-token-status');
    const token = input ? input.value.trim() : '';
    if (!token) {
        showToast(t('config.paperless.token_empty'), 'error');
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'paperless_ngx_api_token', value: token })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            showToast(t('config.paperless.token_saved'), 'success');
            if (input) input.value = '';
            if (status) { status.style.display = 'none'; }
        } else {
            showToast(res.message || t('config.paperless.token_save_failed'), 'error');
        }
    })
    .catch(() => showToast(t('config.paperless.token_save_failed'), 'error'));
}
