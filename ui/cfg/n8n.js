// cfg/n8n.js — n8n workflow automation integration section

async function renderN8nSection(section) {
    const data = configData['n8n'] || {};
    const enabledOn      = data.enabled       === true;
    const readonlyOn     = data.readonly       === true;
    const requireTokenOn = data.require_token  !== false; // default: true

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Info banner ──
    html += '<div class="n8n-info-banner">';
    html += t('config.n8n.info_text');
    html += '</div>';

    // ── Enabled toggle ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.n8n.enabled_label') + '</div>';
    const helpEnabled = t('help.n8n.enabled');
    if (helpEnabled && helpEnabled !== 'help.n8n.enabled') html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="n8n.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.n8n.readonly_label') + '</div>';
    html += '<div class="field-help">' + t('config.n8n.readonly_hint') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="n8n.readonly" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Require token ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.n8n.require_token_label') + '</div>';
    html += '<div class="field-help">' + t('config.n8n.require_token_hint') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (requireTokenOn ? ' on' : '') + '" data-path="n8n.require_token" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (requireTokenOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Webhook Base URL ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.n8n.webhook_url_label') + '</div>';
    html += '<div class="field-help">' + t('config.n8n.webhook_url_hint') + '</div>';
    html += '<input class="field-input" type="url" data-path="n8n.webhook_base_url" value="' + escapeAttr(data.webhook_base_url || '') + '" placeholder="https://n8n.example.com/webhook/" oninput="markDirty()">';
    html += '</div>';

    // ── Rate limit ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.n8n.rate_limit_label') + '</div>';
    html += '<div class="field-help">' + t('config.n8n.rate_limit_hint') + '</div>';
    html += '<input class="field-input n8n-rate-input" type="number" min="0" max="1000" data-path="n8n.rate_limit_rps" value="' + escapeAttr(String(data.rate_limit_rps ?? 10)) + '" oninput="markDirty()">';
    html += '</div>';

    // ── API Token (vault) ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.n8n.token_label') + ' <span class="n8n-lock-icon">🔒</span></div>';
    html += '<div class="field-help">' + t('config.n8n.token_hint') + '</div>';
    html += '<div class="n8n-token-actions">';
    html += '<code id="n8n-token-display" class="n8n-token-display">' + t('config.n8n.token_loading') + '</code>';
    html += '<button class="btn-save n8n-token-btn" onclick="n8nGenerateToken()">⚡ ' + t('config.n8n.token_generate') + '</button>';
    html += '<button class="n8n-token-delete-btn" onclick="n8nDeleteToken()">🗑️ ' + t('config.n8n.token_delete') + '</button>';
    html += '</div>';
    html += '<div id="n8n-token-status" class="n8n-token-status"></div>';
    html += '</div>';

    html += '</div>'; // end cfg-section
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Load current token state
    _n8nLoadToken();
}

async function _n8nLoadToken() {
    const display = document.getElementById('n8n-token-display');
    if (!display) return;
    try {
        const resp = await fetch('/api/n8n/token');
        if (resp.status === 404) {
            display.textContent = t('config.n8n.token_none');
            return;
        }
        const data = await resp.json();
        display.textContent = data.token || t('config.n8n.token_none');
    } catch (e) {
        display.textContent = t('config.n8n.token_error');
    }
}

async function n8nGenerateToken() {
    const status = document.getElementById('n8n-token-status');
    const display = document.getElementById('n8n-token-display');
    try {
        const resp = await fetch('/api/n8n/token', { method: 'POST' });
        if (!resp.ok) throw new Error(resp.statusText);
        const data = await resp.json();
        if (display) display.textContent = data.token || '•••';
        if (status) {
            status.className = 'n8n-token-status n8n-token-status-success';
            status.textContent = '✓ ' + t('config.n8n.token_generated');
            setTimeout(() => { if (status) status.textContent = ''; }, 4000);
        }
    } catch (e) {
        if (status) {
            status.className = 'n8n-token-status n8n-token-status-error';
            status.textContent = '✗ ' + t('config.n8n.token_error');
        }
    }
}

async function n8nDeleteToken() {
    const status = document.getElementById('n8n-token-status');
    const display = document.getElementById('n8n-token-display');
    try {
        const resp = await fetch('/api/n8n/token', { method: 'DELETE' });
        if (!resp.ok && resp.status !== 204) throw new Error(resp.statusText);
        if (display) display.textContent = t('config.n8n.token_none');
        if (status) {
            status.className = 'n8n-token-status n8n-token-status-muted';
            status.textContent = t('config.n8n.token_deleted');
            setTimeout(() => { if (status) status.textContent = ''; }, 3000);
        }
    } catch (e) {
        if (status) {
            status.className = 'n8n-token-status n8n-token-status-error';
            status.textContent = '✗ ' + t('config.n8n.token_error');
        }
    }
}
