// cfg/netlify.js — Netlify integration section module

let nfStatusCache = null;

// The account status is cached for the page lifetime. A config save can change
// the token or the enabled state, so the cached answer is dropped on that event.
if (!window.__auragoNetlifyStatusInvalidate) {
    window.__auragoNetlifyStatusInvalidate = true;
    document.addEventListener('aurago:config-saved', () => { nfStatusCache = null; });
}

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
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    let bannerState = 'off';
    let bannerDetail = t('config.netlify.disabled_desc');
    if (nfEnabled && st.status === 'ok') {
        const name = st.full_name || st.email || t('config.netlify.status_connected');
        const siteCount = st.site_count !== undefined
            ? String(st.site_count) + (st.site_count_truncated ? '+' : '')
            : '';
        bannerState = 'connected';
        bannerDetail = name + (siteCount ? ' · ' + siteCount + ' ' + t('config.netlify.sites') : '');
    } else if (nfEnabled && st.status === 'no_token') {
        bannerState = 'unconfigured';
        bannerDetail = t('config.netlify.no_token_desc');
    } else if (nfEnabled && st.status === 'error') {
        bannerState = 'error';
        bannerDetail = st.message || t('config.netlify.status_error');
    } else if (nfEnabled) {
        bannerState = 'unknown';
        bannerDetail = '';
    }
    html += cfgStatusBanner.html('nf-status-banner', bannerState, { detail: bannerDetail });

    html += `<div class="cfg-toggle-row-highlight">
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.netlify.enabled_label')}</span>
            <div class="field-help" id="nf-netlify-enabled-help">${t('config.netlify.enabled_help')}</div>
        </div>
        <div class="toggle ${nfEnabled ? 'on' : ''}" data-path="netlify.enabled" aria-describedby="nf-netlify-enabled-help" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🔐 ${t('config.netlify.permissions_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.netlify.permissions_desc')}</div>`;

    html += `<div class="field-grid two-cols">`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.readonly ? 'on' : ''}" data-path="netlify.readonly" aria-describedby="nf-netlify-readonly-help" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.netlify.readonly')}</span>
            <div class="field-help" id="nf-netlify-readonly-help">${t('config.netlify.readonly_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_deploy === true ? 'on' : ''}" data-path="netlify.allow_deploy" aria-describedby="nf-netlify-allow-deploy-help" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.netlify.allow_deploy')}</span>
            <div class="field-help" id="nf-netlify-allow-deploy-help">${t('config.netlify.allow_deploy_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_site_management === true ? 'on' : ''}" data-path="netlify.allow_site_management" aria-describedby="nf-netlify-allow-site-management-help" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.netlify.allow_site_management')}</span>
            <div class="field-help" id="nf-netlify-allow-site-management-help">${t('config.netlify.allow_site_management_help')}</div>
        </div>
    </div>`;

    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${cfg.allow_env_management === true ? 'on' : ''}" data-path="netlify.allow_env_management" aria-describedby="nf-netlify-allow-env-management-help" onclick="toggleBool(this)"></div>
        <div class="cfg-toggle-copy">
            <span class="cfg-toggle-label">${t('config.netlify.allow_env_management')}</span>
            <div class="field-help" id="nf-netlify-allow-env-management-help">${t('config.netlify.allow_env_management_help')}</div>
        </div>
    </div>`;

    html += `</div>`;

    html += `</div>`;

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🌐 ${t('config.netlify.site_config_title')}</div>`;
    html += `<div class="field-group-desc">${t('config.netlify.site_config_desc')}</div>`;

    html += `<div class="field-grid two-cols">`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.netlify.default_site_id')}</div>
        <div class="field-help">${t('config.netlify.default_site_id_help')}</div>
        <input class="field-input" type="text" data-path="netlify.default_site_id" value="${escapeAttr(cfg.default_site_id || '')}" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx">
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.netlify.team_slug')}</div>
        <div class="field-help">${t('config.netlify.team_slug_help')}</div>
        <input class="field-input" type="text" data-path="netlify.team_slug" value="${escapeAttr(cfg.team_slug || '')}" placeholder="my-team">
    </div>`;

    html += `</div>`;

    html += `</div>`;

    html += `<div class="field-group">`;
    html += `<div class="field-group-title">🔑 ${t('config.netlify.token_title')}</div>`;
    html += `<div class="field-group-desc" id="nf-token-desc">${t('config.netlify.token_desc')}</div>`;

    html += `<label>
        <span class="cfg-label">${t('config.netlify.token_label')}  <small class="hp-text-tertiary">🔐 ${t('config.netlify.vault_badge')}</small></span>
        <div class="password-wrap">
            <input class="field-input" type="password" id="nf-token" data-path="netlify.token" aria-describedby="nf-token-desc" value="${escapeAttr(cfgSecretValue(cfg.token))}" placeholder="${escapeAttr(tokenPlaceholder)}" autocomplete="off">
            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
        </div>
    </label>`;

    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">🔌 ${t('config.netlify.test_title')}</div>
        <div class="field-group-desc">${t('config.netlify.test_desc')}</div>
        <div class="cfg-actions-row">
            <button class="btn-save adg-test-btn" id="nf-test-btn" data-config-action-draft="true" onclick="nfTestConnection()">🔌 ${t('config.netlify.test_btn')}</button>
            ${cfgActionResult.html('nf-test-result')}
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    if (nfEnabled && st.status === 'error') {
        cfgStatusBanner(document.getElementById('nf-status-banner'), 'error', {
            detail: bannerDetail,
            retry: () => { nfStatusCache = null; renderNetlifySection(section); }
        });
    }
}

function nfTestConnection() {
    const btn = document.getElementById('nf-test-btn');
    return cfgActionResult(btn, () => fetch('/api/netlify/test-connection', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['netlify']) })
    }), {
        target: 'nf-test-result',
        pending: t('config.netlify.connecting'),
        failure: t('config.netlify.test_failed'),
        success: outcome => {
            const json = outcome.body || {};
            let details = json.account_email || json.account_slug || json.full_name || json.email || '';
            if (json.site_count !== undefined) {
                details += (details ? ' · ' : '') + json.site_count + ' ' + t('config.netlify.sites');
            }
            return (json.message || t('config.netlify.test_success')) + (details ? ' — ' + details : '');
        }
    });
}
