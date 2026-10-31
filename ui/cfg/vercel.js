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
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    let bannerState = 'off';
    let bannerDetail = t('config.vercel.disabled_desc');
    if (enabled && st.status === 'ok') {
        const name = st.name || st.username || st.email || t('config.vercel.status_connected');
        bannerState = 'connected';
        bannerDetail = name + (st.project_count !== undefined ? ' · ' + st.project_count + ' ' + t('config.vercel.projects') : '');
    } else if (enabled && st.status === 'no_token') {
        bannerState = 'unconfigured';
        bannerDetail = t('config.vercel.no_token_desc');
    } else if (enabled && st.status === 'error') {
        bannerState = 'error';
        bannerDetail = st.message || t('config.vercel.status_error');
    } else if (enabled) {
        bannerState = 'unknown';
        bannerDetail = '';
    }
    html += cfgStatusBanner.html('vercel-status-banner', bannerState, { detail: bannerDetail });

    html += `<div class="cfg-toggle-row-highlight">
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.enabled_label')}</span>
            <div class="field-help" id="vc-vercel-enabled-help">${t('config.vercel.enabled_help')}</div>
        </div>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="vercel.enabled" aria-describedby="vc-vercel-enabled-help" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🔐 ${t('config.vercel.permissions_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.vercel.permissions_desc')}</div>`;
    html += `<div class="field-grid two-cols">`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.readonly ? 'on' : ''}" data-path="vercel.readonly" aria-describedby="vc-vercel-readonly-help" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.readonly')}</span>
            <div class="field-help" id="vc-vercel-readonly-help">${t('config.vercel.readonly_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_deploy === true ? 'on' : ''}" data-path="vercel.allow_deploy" aria-describedby="vc-vercel-allow-deploy-help" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.allow_deploy')}</span>
            <div class="field-help" id="vc-vercel-allow-deploy-help">${t('config.vercel.allow_deploy_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_project_management ? 'on' : ''}" data-path="vercel.allow_project_management" aria-describedby="vc-vercel-allow-project-management-help" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.allow_project_management')}</span>
            <div class="field-help" id="vc-vercel-allow-project-management-help">${t('config.vercel.allow_project_management_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_env_management ? 'on' : ''}" data-path="vercel.allow_env_management" aria-describedby="vc-vercel-allow-env-management-help" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.allow_env_management')}</span>
            <div class="field-help" id="vc-vercel-allow-env-management-help">${t('config.vercel.allow_env_management_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_domain_management ? 'on' : ''}" data-path="vercel.allow_domain_management" aria-describedby="vc-vercel-allow-domain-management-help" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.vercel.allow_domain_management')}</span>
            <div class="field-help" id="vc-vercel-allow-domain-management-help">${t('config.vercel.allow_domain_management_help')}</div>
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
    html += `<div class="field-group-desc" id="vc-token-desc">${t('config.vercel.token_desc')}</div>`;

    html += `<label>
        <span class="cfg-label">${t('config.vercel.token_label')} <small class="hp-text-tertiary">🔐 ${t('config.vercel.vault_badge')}</small></span>
        <div class="password-wrap">
            <input class="field-input" type="password" id="vercel-token" data-path="vercel.token" aria-describedby="vc-token-desc" value="${escapeAttr(cfgSecretValue(cfg.token))}" placeholder="${escapeAttr(tokenPlaceholder)}" autocomplete="off">
            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
        </div>
    </label>`;

    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">🔌 ${t('config.vercel.test_title')}</div>
        <div class="field-group-desc">${t('config.vercel.test_desc')}</div>
        <div class="cfg-actions-row">
            <button class="btn-save adg-test-btn" id="vercel-test-btn" data-config-action-draft="true" onclick="vercelTestConnection()">🔌 ${t('config.vercel.test_btn')}</button>
            ${cfgActionResult.html('vercel-test-result')}
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    if (bannerState === 'error' || bannerState === 'unknown') {
        cfgStatusBanner(document.getElementById('vercel-status-banner'), bannerState, {
            detail: bannerDetail,
            retry: () => { vercelStatusCache = null; renderVercelSection(section); }
        });
    }
}

function vercelTestConnection() {
    const btn = document.getElementById('vercel-test-btn');
    return cfgActionResult(btn, () => fetch('/api/vercel/test-connection', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['vercel']) })
    }), {
        target: 'vercel-test-result',
        pending: t('config.vercel.connecting'),
        failure: t('config.vercel.test_failed'),
        success: outcome => {
            const json = outcome.body || {};
            let details = json.name || json.username || json.email || '';
            if (json.project_count !== undefined) {
                details += (details ? ' · ' : '') + json.project_count + ' ' + t('config.vercel.projects');
            }
            return (json.message || t('config.vercel.test_success')) + (details ? ' — ' + details : '');
        }
    });
}

// A config save can change the token or the enabled state: drop the cached account status.
if (!window.__auragoVercelStatusInvalidate) {
    window.__auragoVercelStatusInvalidate = true;
    document.addEventListener('aurago:config-saved', () => { vercelStatusCache = null; });
}
