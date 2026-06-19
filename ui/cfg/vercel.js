// cfg/vercel.js — Vercel integration section module

let vercelStatusCache = null;

async function renderVercelSection(section) {
    if (vercelStatusCache === null) {
        try {
            const resp = await fetch('/api/vercel/status');
            vercelStatusCache = resp.ok ? await resp.json() : {};
        } catch (_) { vercelStatusCache = {}; }
    }

    const cfg = configData.vercel || {};
    const tokenPlaceholder = cfgSecretPlaceholder(cfg.token);
    const enabled = cfg.enabled === true;
    const st = vercelStatusCache || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    if (enabled && st.status === 'ok') {
        const name = st.name || st.username || st.email || 'Connected';
        html += `<div class="wh-notice nf-status-ok">
            <span>✅</span>
            <div>
                <strong>${t('config.vercel.status_connected')}</strong><br>
                <small>${escapeAttr(name)}${st.project_count !== undefined ? ' · ' + st.project_count + ' ' + t('config.vercel.projects') : ''}</small>
            </div>
        </div>`;
    } else if (enabled && st.status === 'no_token') {
        html += `<div class="wh-notice nf-status-warn">
            <span>🔑</span>
            <div>
                <strong>${t('config.vercel.no_token')}</strong><br>
                <small>${t('config.vercel.no_token_desc')}</small>
            </div>
        </div>`;
    } else if (enabled && st.status === 'error') {
        html += `<div class="wh-notice nf-status-error">
            <span>❌</span>
            <div>
                <strong>${t('config.vercel.status_error')}</strong><br>
                <small>${escapeAttr(st.message || '')}</small>
            </div>
        </div>`;
    }

    html += `<div class="cfg-toggle-row-highlight">
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.enabled_label')}</span>
            <div class="field-help">${t('config.vercel.enabled_help')}</div>
        </div>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="vercel.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>▲</span>
            <div>
                <strong>${t('config.vercel.disabled_notice')}</strong><br>
                <small>${t('config.vercel.disabled_desc')}</small>
            </div>
        </div>`;
    }

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🔐 ${t('config.vercel.permissions_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.vercel.permissions_desc')}</div>`;
    html += `<div class="field-grid two-cols">`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.readonly ? 'on' : ''}" data-path="vercel.readonly" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.readonly')}</span>
            <div class="field-help">${t('config.vercel.readonly_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_deploy !== false ? 'on' : ''}" data-path="vercel.allow_deploy" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.allow_deploy')}</span>
            <div class="field-help">${t('config.vercel.allow_deploy_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_project_management ? 'on' : ''}" data-path="vercel.allow_project_management" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.allow_project_management')}</span>
            <div class="field-help">${t('config.vercel.allow_project_management_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_env_management ? 'on' : ''}" data-path="vercel.allow_env_management" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.allow_env_management')}</span>
            <div class="field-help">${t('config.vercel.allow_env_management_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_domain_management ? 'on' : ''}" data-path="vercel.allow_domain_management" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.allow_domain_management')}</span>
            <div class="field-help">${t('config.vercel.allow_domain_management_help')}</div>
        </div>
    </div>`;

    html += `</div>`;
    html += `</div>`;

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🌐 ${t('config.vercel.project_config_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.vercel.project_config_desc')}</div>`;
    html += `<div class="field-grid two-cols">`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.vercel.default_project_id')}</div>
        <div class="field-help">${t('config.vercel.default_project_id_help')}</div>
        <input class="field-input" type="text" data-path="vercel.default_project_id" value="${escapeAttr(cfg.default_project_id || '')}" placeholder="prj_xxxxxxxxxxxxxxxxxxxxxxxxx">
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.vercel.team_id')}</div>
        <div class="field-help">${t('config.vercel.team_id_help')}</div>
        <input class="field-input" type="text" data-path="vercel.team_id" value="${escapeAttr(cfg.team_id || '')}" placeholder="team_xxxxxxxxxxxxxxxxxxxxxxxxx">
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.vercel.team_slug')}</div>
        <div class="field-help">${t('config.vercel.team_slug_help')}</div>
        <input class="field-input" type="text" data-path="vercel.team_slug" value="${escapeAttr(cfg.team_slug || '')}" placeholder="my-team">
    </div>`;

    html += `</div>`;
    html += `</div>`;

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🔑 ${t('config.vercel.token_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.vercel.token_desc')}</div>`;

    html += `<label>
        <span class="cfg-label">${t('config.vercel.token_label')} <small class="hp-text-tertiary">🔐 vault</small></span>
        <div class="password-wrap">
            <input class="field-input" type="password" id="vercel-token" value="${escapeAttr(cfgSecretValue(cfg.token))}" placeholder="${escapeAttr(tokenPlaceholder)}" autocomplete="off">
            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
        </div>
    </label>`;

    html += `<div class="adg-password-row">
        <button class="btn-save adg-save-btn" onclick="vercelSaveToken()">💾 ${t('config.vercel.save_token')}</button>
        <span id="vercel-token-status" class="adg-test-result"></span>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">🔌 ${t('config.vercel.test_title')}</div>
        <div class="field-group-desc">${t('config.vercel.test_desc')}</div>
        <div class="cfg-actions-row">
            <button class="btn-save adg-test-btn" id="vercel-test-btn" onclick="vercelTestConnection()">🔌 ${t('config.vercel.test_btn')}</button>
            <span id="vercel-test-result" class="adg-test-result"></span>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

async function vercelSaveToken() {
    const statusEl = document.getElementById('vercel-token-status');
    const token = document.getElementById('vercel-token').value;

    if (!token) {
        if (statusEl) {
            statusEl.textContent = t('config.vercel.token_empty');
            statusEl.className = 'adg-test-result is-danger';
        }
        return;
    }

    if (statusEl) {
        statusEl.textContent = t('config.vercel.saving');
        statusEl.className = 'adg-test-result';
    }

    try {
        const resp = await fetch('/api/vault/secrets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: 'vercel_token', value: token })
        });
        if (!resp.ok) {
            const txt = await resp.text();
            if (statusEl) {
                statusEl.textContent = txt;
                statusEl.className = 'adg-test-result is-danger';
            }
        } else {
            if (statusEl) {
                statusEl.textContent = t('config.vercel.token_saved');
                statusEl.className = 'adg-test-result is-success';
            }
            cfgMarkSecretStored(document.getElementById('vercel-token'), 'vercel.token');
            vercelStatusCache = null;
        }
    } catch (e) {
        if (statusEl) {
            statusEl.textContent = e.message;
            statusEl.className = 'adg-test-result is-danger';
        }
    }
}

async function vercelTestConnection() {
    const btn = document.getElementById('vercel-test-btn');
    const result = document.getElementById('vercel-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.className = 'adg-test-result';
        result.textContent = t('config.vercel.connecting');
    }

    try {
        const resp = await fetch('/api/vercel/test-connection', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: '{}'
        });
        const json = await resp.json();
        if (!result) return;
        if (json.status === 'ok') {
            let details = json.name || json.username || json.email || '';
            if (json.project_count !== undefined) {
                details += (details ? ' · ' : '') + json.project_count + ' ' + t('config.vercel.projects');
            }
            result.className = 'adg-test-result is-success';
            result.textContent = (json.message || t('config.vercel.test_success')) + (details ? ' — ' + details : '');
        } else {
            result.className = 'adg-test-result is-danger';
            result.textContent = json.message || t('config.vercel.test_failed');
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
