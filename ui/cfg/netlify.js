// cfg/netlify.js — Netlify integration section module

let nfStatusCache = null;

async function renderNetlifySection(section) {
    if (nfStatusCache === null) {
        try {
            const resp = await fetch('/api/netlify/status');
            nfStatusCache = resp.ok ? await resp.json() : {};
        } catch (_) { nfStatusCache = {}; }
    }

    const cfg = configData.netlify || {};
    const tokenPlaceholder = cfgSecretPlaceholder(cfg.token);
    const nfEnabled = cfg.enabled === true;
    const st = nfStatusCache || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    if (nfEnabled && st.status === 'ok') {
        const name = st.full_name || st.email || 'Connected';
        html += `<div class="wh-notice nf-status-ok">
            <span>✅</span>
            <div>
                <strong>${t('config.netlify.status_connected')}</strong><br>
                <small>${escapeAttr(name)}${st.site_count !== undefined ? ' · ' + st.site_count + ' ' + t('config.netlify.sites') : ''}</small>
            </div>
        </div>`;
    } else if (nfEnabled && st.status === 'no_token') {
        html += `<div class="wh-notice nf-status-warn">
            <span>🔑</span>
            <div>
                <strong>${t('config.netlify.no_token')}</strong><br>
                <small>${t('config.netlify.no_token_desc')}</small>
            </div>
        </div>`;
    } else if (nfEnabled && st.status === 'error') {
        html += `<div class="wh-notice nf-status-error">
            <span>❌</span>
            <div>
                <strong>${t('config.netlify.status_error')}</strong><br>
                <small>${escapeAttr(st.message || '')}</small>
            </div>
        </div>`;
    }

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

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🔐 ${t('config.netlify.permissions_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.netlify.permissions_desc')}</div>`;

    html += `<div class="nf-grid-2col">`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.readonly ? 'on' : ''}" data-path="netlify.readonly" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.netlify.readonly')}</span>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_deploy !== false ? 'on' : ''}" data-path="netlify.allow_deploy" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.netlify.allow_deploy')}</span>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_site_management ? 'on' : ''}" data-path="netlify.allow_site_management" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.netlify.allow_site_management')}</span>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_env_management ? 'on' : ''}" data-path="netlify.allow_env_management" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.netlify.allow_env_management')}</span>
    </div>`;

    html += `</div>`;

    html += `</div>`;

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🌐 ${t('config.netlify.site_config_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.netlify.site_config_desc')}</div>`;

    html += `<div class="nf-grid-2col-wide">`;

    html += `<label>
        <span class="cfg-label">${t('config.netlify.default_site_id')}</span>
        <input class="cfg-input cfg-input-full" data-path="netlify.default_site_id" value="${escapeAttr(cfg.default_site_id || '')}" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            onchange="setNestedValue(configData,'netlify.default_site_id',this.value);setDirty(true)">
    </label>`;

    html += `<label>
        <span class="cfg-label">${t('config.netlify.team_slug')}</span>
        <input class="cfg-input cfg-input-full" data-path="netlify.team_slug" value="${escapeAttr(cfg.team_slug || '')}" placeholder="my-team"
            onchange="setNestedValue(configData,'netlify.team_slug',this.value);setDirty(true)">
    </label>`;

    html += `</div>`;

    html += `</div>`;

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🔑 ${t('config.netlify.token_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.netlify.token_desc')}</div>`;

    html += `<label>
        <span class="cfg-label">${t('config.netlify.token_label')}  <small class="hp-text-tertiary">🔐 vault</small></span>
        <div class="password-wrap">
            <input class="field-input" type="password" id="nf-token" value="${escapeAttr(cfgSecretValue(cfg.token))}" placeholder="${escapeAttr(tokenPlaceholder)}" autocomplete="off">
            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
        </div>
    </label>`;

    html += `<div class="adg-password-row">
        <button class="btn-save adg-save-btn" onclick="nfSaveToken()">💾 ${t('config.netlify.save_token')}</button>
        <span id="nf-token-status" class="adg-test-result"></span>
    </div>`;

    html += `</div>`;

    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">🔌 ${t('config.netlify.test_title')}</div>
        <div class="field-group-desc">${t('config.netlify.test_desc')}</div>
        <div class="cfg-actions-row">
            <button class="btn-save adg-test-btn" id="nf-test-btn" onclick="nfTestConnection()">🔌 ${t('config.netlify.test_btn')}</button>
            <span id="nf-test-result" class="adg-test-result"></span>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

async function nfSaveToken() {
    const statusEl = document.getElementById('nf-token-status');
    const token = document.getElementById('nf-token').value;

    if (!token) {
        if (statusEl) {
            statusEl.textContent = t('config.netlify.token_empty');
            statusEl.className = 'adg-test-result is-danger';
        }
        return;
    }

    if (statusEl) {
        statusEl.textContent = t('config.netlify.saving');
        statusEl.className = 'adg-test-result';
    }

    try {
        const resp = await fetch('/api/vault/secrets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: 'netlify_token', value: token })
        });
        if (!resp.ok) {
            const txt = await resp.text();
            if (statusEl) {
                statusEl.textContent = txt;
                statusEl.className = 'adg-test-result is-danger';
            }
        } else {
            if (statusEl) {
                statusEl.textContent = t('config.netlify.token_saved');
                statusEl.className = 'adg-test-result is-success';
            }
            cfgMarkSecretStored(document.getElementById('nf-token'), 'netlify.token');
            nfStatusCache = null;
        }
    } catch (e) {
        if (statusEl) {
            statusEl.textContent = e.message;
            statusEl.className = 'adg-test-result is-danger';
        }
    }
}

async function nfTestConnection() {
    const btn = document.getElementById('nf-test-btn');
    const result = document.getElementById('nf-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.className = 'adg-test-result';
        result.textContent = t('config.netlify.connecting');
    }

    try {
        const resp = await fetch('/api/netlify/test-connection', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: '{}'
        });
        const json = await resp.json();
        if (!result) return;
        if (json.status === 'ok') {
            let details = json.full_name || json.email || '';
            if (json.site_count !== undefined) {
                details += (details ? ' · ' : '') + json.site_count + ' ' + t('config.netlify.sites');
            }
            result.className = 'adg-test-result is-success';
            result.textContent = (json.message || t('config.netlify.test_success')) + (details ? ' — ' + details : '');
        } else {
            result.className = 'adg-test-result is-danger';
            result.textContent = json.message || t('config.netlify.test_failed');
        }
    } catch (e) {
        if (result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = e.message;
        }
    } finally {
        if (btn) btn.disabled = false;
    }
}
