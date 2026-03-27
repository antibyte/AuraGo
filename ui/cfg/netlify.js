// cfg/netlify.js — Netlify integration section module

let nfStatusCache = null;

async function renderNetlifySection(section) {
    // Lazy-load status
    if (nfStatusCache === null) {
        try {
            const resp = await fetch('/api/netlify/status');
            nfStatusCache = resp.ok ? await resp.json() : {};
        } catch (_) { nfStatusCache = {}; }
    }

    const cfg = configData.netlify || {};
    const nfEnabled = cfg.enabled === true;
    const st = nfStatusCache || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Status banner ──
    if (nfEnabled && st.status === 'ok') {
        const name = st.full_name || st.email || 'Connected';
        html += `<div class="wh-notice" style="border-color:var(--success);background:rgba(34,197,94,0.06);">
            <span>✅</span>
            <div>
                <strong>${t('config.netlify.status_connected')}</strong><br>
                <small>${escapeAttr(name)}${st.site_count !== undefined ? ' · ' + st.site_count + ' ' + t('config.netlify.sites') : ''}</small>
            </div>
        </div>`;
    } else if (nfEnabled && st.status === 'no_token') {
        html += `<div class="wh-notice" style="border-color:var(--warning);background:rgba(234,179,8,0.06);">
            <span>🔑</span>
            <div>
                <strong>${t('config.netlify.no_token')}</strong><br>
                <small>${t('config.netlify.no_token_desc')}</small>
            </div>
        </div>`;
    } else if (nfEnabled && st.status === 'error') {
        html += `<div class="wh-notice" style="border-color:var(--danger);background:rgba(239,68,68,0.06);">
            <span>❌</span>
            <div>
                <strong>${t('config.netlify.status_error')}</strong><br>
                <small>${escapeAttr(st.message || '')}</small>
            </div>
        </div>`;
    }

    // ── Enabled toggle ──
    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.netlify.enabled_label')}</span>
        <div class="toggle ${nfEnabled ? 'on' : ''}" data-path="netlify.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    if (!nfEnabled) {
        html += `<div class="wh-notice">
            <span>🚀</span>
            <div>
                <strong>${t('config.netlify.disabled_notice')}</strong><br>
                <small>${t('config.netlify.disabled_desc')}</small>
            </div>
        </div>`;
    }

    // ── Permission toggles ──
    html += `<div style="margin-top:1.2rem;margin-bottom:0.5rem;font-weight:600;font-size:0.9rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;">🔐 ${t('config.netlify.permissions_title')}</div>`;
    html += `<div class="field-help" style="margin-bottom:0.8rem;">${t('config.netlify.permissions_desc')}</div>`;

    html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.6rem 1.2rem;">`;

    // Read Only
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.readonly ? 'on' : ''}" data-path="netlify.readonly" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.netlify.readonly')}</span>
    </div>`;

    // Allow Deploy
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_deploy !== false ? 'on' : ''}" data-path="netlify.allow_deploy" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.netlify.allow_deploy')}</span>
    </div>`;

    // Allow Site Management
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_site_management ? 'on' : ''}" data-path="netlify.allow_site_management" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.netlify.allow_site_management')}</span>
    </div>`;

    // Allow Env Management
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_env_management ? 'on' : ''}" data-path="netlify.allow_env_management" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.netlify.allow_env_management')}</span>
    </div>`;

    html += `</div>`;

    // ── Site Configuration ──
    html += `<div style="margin-top:1.5rem;margin-bottom:0.5rem;font-weight:600;font-size:0.9rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;">🌐 ${t('config.netlify.site_config_title')}</div>`;
    html += `<div class="field-help" style="margin-bottom:0.8rem;">${t('config.netlify.site_config_desc')}</div>`;

    html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem 1.2rem;">`;

    // Default Site ID
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.netlify.default_site_id')}</span>
        <input class="cfg-input" data-path="netlify.default_site_id" value="${escapeAttr(cfg.default_site_id || '')}" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" style="width:100%;margin-top:0.2rem;"
            onchange="setNestedValue(configData,'netlify.default_site_id',this.value);setDirty(true)">
    </label>`;

    // Team Slug
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.netlify.team_slug')}</span>
        <input class="cfg-input" data-path="netlify.team_slug" value="${escapeAttr(cfg.team_slug || '')}" placeholder="my-team" style="width:100%;margin-top:0.2rem;"
            onchange="setNestedValue(configData,'netlify.team_slug',this.value);setDirty(true)">
    </label>`;

    html += `</div>`;

    // ── API Token (vault-stored) ──
    html += `<div style="margin-top:1.2rem;padding:0.8rem 1rem;border-radius:10px;border:1px dashed var(--border-subtle);background:var(--bg-tertiary);">`;
    html += `<div style="font-size:0.82rem;font-weight:600;color:var(--text-secondary);margin-bottom:0.6rem;">🔑 ${t('config.netlify.token_title')}</div>`;
    html += `<div class="field-help" style="margin-bottom:0.8rem;">${t('config.netlify.token_desc')}</div>`;

    // Token field
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.netlify.token_label')}  <small style="color:var(--text-tertiary);">🔐 vault</small></span>
        <div class="password-wrap" style="margin-top:0.2rem;">
            <input class="field-input" type="password" id="nf-token" value="" placeholder="••••••••" autocomplete="off" style="width:100%;">
            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
        </div>
    </label>`;

    // Save token button
    html += `<div style="display:flex;gap:0.6rem;align-items:center;margin-top:0.8rem;flex-wrap:wrap;">
        <button class="btn-save" style="padding:0.4rem 1.1rem;font-size:0.78rem;" onclick="nfSaveToken()">${t('config.netlify.save_token')}</button>
        <span id="nf-token-status" style="font-size:0.75rem;color:var(--text-tertiary);"></span>
    </div>`;

    html += `</div>`; // end token box

    // ── Test Connection ──
    html += `<div id="nf-test-block" style="margin-top:1rem;padding:1rem 1.2rem;border:1px solid var(--border-subtle);border-radius:12px;background:var(--bg-secondary);">
        <div style="font-size:0.8rem;font-weight:600;color:var(--accent);margin-bottom:0.5rem;">🔌 ${t('config.netlify.test_title')}</div>
        <div style="font-size:0.78rem;color:var(--text-secondary);margin-bottom:0.8rem;">${t('config.netlify.test_desc')}</div>
        <div style="display:flex;gap:0.6rem;align-items:center;flex-wrap:wrap;">
            <button class="btn-save" style="padding:0.4rem 1.1rem;font-size:0.78rem;" onclick="nfTestConnection()">${t('config.netlify.test_btn')}</button>
            <span id="nf-test-spinner" style="display:none;font-size:0.75rem;color:var(--text-secondary);">⏳ ${t('config.netlify.connecting')}</span>
        </div>
        <div id="nf-test-result" style="margin-top:0.8rem;display:none;">
            <div id="nf-test-msg" style="font-size:0.82rem;padding:0.45rem 0.7rem;border-radius:7px;"></div>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

// ── Save token to vault ──
async function nfSaveToken() {
    const statusEl = document.getElementById('nf-token-status');
    const token = document.getElementById('nf-token').value;

    if (!token) {
        if (statusEl) { statusEl.textContent = '⚠️ ' + t('config.netlify.token_empty'); statusEl.style.color = 'var(--warning)'; }
        return;
    }

    if (statusEl) { statusEl.textContent = '⏳ ' + t('config.netlify.saving'); statusEl.style.color = 'var(--text-tertiary)'; }

    try {
        const resp = await fetch('/api/vault/secrets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: 'netlify_token', value: token })
        });
        if (!resp.ok) {
            const txt = await resp.text();
            if (statusEl) { statusEl.textContent = '❌ ' + txt; statusEl.style.color = 'var(--danger)'; }
        } else {
            if (statusEl) { statusEl.textContent = '✅ ' + t('config.netlify.token_saved'); statusEl.style.color = 'var(--success)'; }
            document.getElementById('nf-token').value = '';
            nfStatusCache = null; // invalidate cache
        }
    } catch (e) {
        if (statusEl) { statusEl.textContent = '❌ ' + e.message; statusEl.style.color = 'var(--danger)'; }
    }
}

// ── Test Connection ──
async function nfTestConnection() {
    const spinner = document.getElementById('nf-test-spinner');
    const resultDiv = document.getElementById('nf-test-result');
    const msgDiv = document.getElementById('nf-test-msg');
    if (!spinner) return;

    spinner.style.display = 'inline';
    resultDiv.style.display = 'none';

    try {
        const resp = await fetch('/api/netlify/test-connection', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: '{}'
        });
        const json = await resp.json();

        resultDiv.style.display = 'block';
        if (json.status === 'ok') {
            let details = json.full_name || json.email || '';
            if (json.site_count !== undefined) {
                details += (details ? ' · ' : '') + json.site_count + ' ' + t('config.netlify.sites');
            }
            msgDiv.style.background = 'rgba(34,197,94,0.12)';
            msgDiv.style.color = 'var(--success, #22c55e)';
            msgDiv.style.border = '1px solid rgba(34,197,94,0.3)';
            msgDiv.textContent = '✅ ' + (json.message || t('config.netlify.test_success')) + (details ? ' — ' + details : '');
        } else {
            msgDiv.style.background = 'rgba(239,68,68,0.10)';
            msgDiv.style.color = 'var(--danger, #ef4444)';
            msgDiv.style.border = '1px solid rgba(239,68,68,0.25)';
            msgDiv.textContent = '❌ ' + (json.message || t('config.netlify.test_failed'));
        }
    } catch (e) {
        resultDiv.style.display = 'block';
        msgDiv.style.background = 'rgba(239,68,68,0.10)';
        msgDiv.style.color = 'var(--danger, #ef4444)';
        msgDiv.style.border = '1px solid rgba(239,68,68,0.25)';
        msgDiv.textContent = '❌ ' + e.message;
    } finally {
        spinner.style.display = 'none';
    }
}
