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
        <span class="cfg-toggle-label">${t('config.vercel.enabled_label')}</span>
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
    html += `<div class="nf-grid-2col">`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.readonly ? 'on' : ''}" data-path="vercel.readonly" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.vercel.readonly')}</span>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_deploy !== false ? 'on' : ''}" data-path="vercel.allow_deploy" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.vercel.allow_deploy')}</span>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_project_management ? 'on' : ''}" data-path="vercel.allow_project_management" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.vercel.allow_project_management')}</span>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_env_management ? 'on' : ''}" data-path="vercel.allow_env_management" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.vercel.allow_env_management')}</span>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_domain_management ? 'on' : ''}" data-path="vercel.allow_domain_management" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.vercel.allow_domain_management')}</span>
    </div>`;

    html += `</div>`;
    html += `</div>`;

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🌐 ${t('config.vercel.project_config_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.vercel.project_config_desc')}</div>`;
    html += `<div class="nf-grid-2col-wide">`;

    html += `<label>
        <span class="cfg-label">${t('config.vercel.default_project_id')}</span>
        <input class="cfg-input cfg-input-full" data-path="vercel.default_project_id" value="${escapeAttr(cfg.default_project_id || '')}" placeholder="prj_xxxxxxxxxxxxxxxxxxxxxxxxx"
            onchange="setNestedValue(configData,'vercel.default_project_id',this.value);setDirty(true)">
    </label>`;

    html += `<label>
        <span class="cfg-label">${t('config.vercel.team_id')}</span>
        <input class="cfg-input cfg-input-full" data-path="vercel.team_id" value="${escapeAttr(cfg.team_id || '')}" placeholder="team_xxxxxxxxxxxxxxxxxxxxxxxxx"
            onchange="setNestedValue(configData,'vercel.team_id',this.value);setDirty(true)">
    </label>`;

    html += `<label>
        <span class="cfg-label">${t('config.vercel.team_slug')}</span>
        <input class="cfg-input cfg-input-full" data-path="vercel.team_slug" value="${escapeAttr(cfg.team_slug || '')}" placeholder="my-team"
            onchange="setNestedValue(configData,'vercel.team_slug',this.value);setDirty(true)">
    </label>`;

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

    html += `<div class="cfg-field-row">
        <button class="btn-save cfg-save-btn-sm" onclick="vercelSaveToken()">${t('config.vercel.save_token')}</button>
        <span id="vercel-token-status" class="cfg-status-text"></span>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">🔌 ${t('config.vercel.test_title')}</div>
        <div class="field-group-desc">${t('config.vercel.test_desc')}</div>
        <div class="cfg-field-row">
            <button class="btn-save cfg-save-btn-sm" onclick="vercelTestConnection()">${t('config.vercel.test_btn')}</button>
            <span id="vercel-test-spinner" class="is-hidden cfg-status-text">⏳ ${t('config.vercel.connecting')}</span>
        </div>
        <div id="vercel-test-result" class="is-hidden hp-help-mt-sm">
            <div id="vercel-test-msg" class="nf-test-msg"></div>
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
        if (statusEl) { statusEl.textContent = '⚠️ ' + t('config.vercel.token_empty'); statusEl.className = 'cfg-status-text cfg-status-warning'; }
        return;
    }

    if (statusEl) { statusEl.textContent = '⏳ ' + t('config.vercel.saving'); statusEl.className = 'cfg-status-text'; }

    try {
        const resp = await fetch('/api/vault/secrets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: 'vercel_token', value: token })
        });
        if (!resp.ok) {
            const txt = await resp.text();
            if (statusEl) { statusEl.textContent = '❌ ' + txt; statusEl.className = 'cfg-status-text cfg-status-error'; }
        } else {
            if (statusEl) { statusEl.textContent = '✅ ' + t('config.vercel.token_saved'); statusEl.className = 'cfg-status-text cfg-status-success'; }
            cfgMarkSecretStored(document.getElementById('vercel-token'), 'vercel.token');
            vercelStatusCache = null;
        }
    } catch (e) {
        if (statusEl) { statusEl.textContent = '❌ ' + e.message; statusEl.className = 'cfg-status-text cfg-status-error'; }
    }
}

async function vercelTestConnection() {
    const spinner = document.getElementById('vercel-test-spinner');
    const resultDiv = document.getElementById('vercel-test-result');
    const msgDiv = document.getElementById('vercel-test-msg');
    if (!spinner) return;

    setHidden(spinner, false);
    setHidden(resultDiv, true);

    try {
        const resp = await fetch('/api/vercel/test-connection', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: '{}'
        });
        const json = await resp.json();

        setHidden(resultDiv, false);
        if (json.status === 'ok') {
            let details = json.name || json.username || json.email || '';
            if (json.project_count !== undefined) {
                details += (details ? ' · ' : '') + json.project_count + ' ' + t('config.vercel.projects');
            }
            msgDiv.className = 'nf-test-msg cfg-status-success';
            msgDiv.textContent = '✅ ' + (json.message || t('config.vercel.test_success')) + (details ? ' — ' + details : '');
        } else {
            msgDiv.className = 'nf-test-msg cfg-status-error';
            msgDiv.textContent = '❌ ' + (json.message || t('config.vercel.test_failed'));
        }
    } catch (e) {
        setHidden(resultDiv, false);
        msgDiv.className = 'nf-test-msg cfg-status-error';
        msgDiv.textContent = '❌ ' + e.message;
    } finally {
        setHidden(spinner, true);
    }
}
