// cfg/adguard.js — AdGuard Home integration section module

function renderAdGuardSection(section) {
    const data = configData['adguard'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Connection status banner ──
    html += '<div id="adg-status-banner" style="margin-bottom:1rem;padding:0.8rem 1rem;border-radius:10px;font-size:0.84rem;background:var(--bg-tertiary);color:var(--text-secondary);">' + t('config.adguard.checking') + '</div>';

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
    html += '<input class="field-input" type="text" data-path="adguard.url" value="' + escAttr(data.url || '') + '" placeholder="http://192.168.1.1:3000">';
    html += '</div>';

    // ── Username ──
    const helpUser = t('help.adguard.username');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.adguard.username_label') + '</div>';
    if (helpUser) html += '<div class="field-help">' + helpUser + '</div>';
    html += '<input class="field-input" type="text" data-path="adguard.username" value="' + escAttr(data.username || '') + '" placeholder="admin">';
    html += '</div>';

    // ── Password (vault) ──
    const helpPass = t('help.adguard.password');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.adguard.password_label') + '</div>';
    if (helpPass) html += '<div class="field-help">' + helpPass + '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;">';
    html += '<input class="field-input" type="password" id="adg-password" placeholder="' + t('config.adguard.password_placeholder') + '" style="flex:1;">';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;" onclick="adgSavePassword()">💾 ' + t('config.adguard.save_vault') + '</button>';
    html += '</div></div>';

    // ── Test connection button ──
    html += '<div class="field-group">';
    html += '<button class="btn-save" style="padding:0.5rem 1.2rem;font-size:0.85rem;" onclick="adgTestConnection()" id="adg-test-btn">🔌 ' + t('config.adguard.test_btn') + '</button>';
    html += '<span id="adg-test-result" style="margin-left:0.8rem;font-size:0.83rem;"></span>';
    html += '</div>';

    // ── Quick stats (populated when connected) ──
    html += '<div id="adg-quick-stats" style="display:none;margin-top:1.5rem;">';
    html += '<div style="font-weight:600;font-size:0.95rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;margin-bottom:0.8rem;">📊 ' + t('config.adguard.stats_title') + '</div>';
    html += '<div id="adg-stats-content" style="display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:0.8rem;"></div>';
    html += '</div>';

    html += '</div>';

    section._el.innerHTML = html;

    // Auto-check status on load
    if (enabledOn && data.url) {
        adgCheckStatus();
    }
}

function adgCheckStatus() {
    const banner = document.getElementById('adg-status-banner');
    if (!banner) return;
    banner.style.background = 'var(--bg-tertiary)';
    banner.style.color = 'var(--text-secondary)';
    banner.textContent = t('config.adguard.checking');

    fetch('/api/adguard/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                banner.textContent = '⚪ ' + t('config.adguard.status_disabled');
                return;
            }
            if (res.status === 'no_url') {
                banner.textContent = '⚪ ' + t('config.adguard.status_no_url');
                return;
            }
            if (res.status === 'ok' && res.data) {
                const d = res.data;
                const version = d.version || '?';
                const running = d.running === true;
                banner.style.background = running ? 'rgba(52,199,89,0.12)' : 'rgba(255,149,0,0.12)';
                banner.style.color = running ? 'var(--success, #34c759)' : 'var(--warning, #ff9500)';
                banner.textContent = (running ? '🟢' : '🟡') + ' AdGuard Home v' + version + ' — ' + (running ? t('config.adguard.running') : t('config.adguard.not_running'));

                // Load quick stats
                adgLoadQuickStats();
                return;
            }
            banner.style.background = 'rgba(255,59,48,0.1)';
            banner.style.color = 'var(--error, #ff3b30)';
            banner.textContent = '🔴 ' + (res.message || t('config.adguard.connection_failed'));
        })
        .catch(() => {
            banner.style.background = 'rgba(255,59,48,0.1)';
            banner.style.color = 'var(--error, #ff3b30)';
            banner.textContent = '🔴 ' + t('config.adguard.connection_failed');
        });
}

function adgLoadQuickStats() {
    // We reuse the status endpoint — stats require a separate call via tool
    const wrap = document.getElementById('adg-quick-stats');
    const content = document.getElementById('adg-stats-content');
    if (!wrap || !content) return;
    wrap.style.display = 'block';
    content.innerHTML = '<div style="color:var(--text-tertiary);font-size:0.83rem;">' + t('config.adguard.stats_loaded') + '</div>';
}

function adgTestConnection() {
    const btn = document.getElementById('adg-test-btn');
    const result = document.getElementById('adg-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.textContent = '⏳ ...'; result.style.color = 'var(--text-secondary)'; }

    fetch('/api/adguard/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.style.color = 'var(--success, #34c759)';
                result.textContent = '✅ ' + t('config.adguard.test_ok');
                adgCheckStatus();
            } else {
                result.style.color = 'var(--error, #ff3b30)';
                result.textContent = '❌ ' + (res.message || t('config.adguard.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.style.color = 'var(--error, #ff3b30)';
                result.textContent = '❌ ' + t('config.adguard.test_fail');
            }
        });
}

function adgSavePassword() {
    const input = document.getElementById('adg-password');
    const pw = input ? input.value.trim() : '';
    if (!pw) { showToast(t('config.adguard.password_empty'), 'error'); return; }

    fetch('/api/vault', {
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
