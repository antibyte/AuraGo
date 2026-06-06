// cfg/composio.js - Composio external tool-source integration

let composioState = {
    status: null,
    selection: null,
    toolkits: [],
    tools: [],
    accounts: [],
    authConfigs: [],
    selectedSlug: '',
    nextCursor: '',
    query: '',
    filter: 'all',
    loading: false
};

async function renderComposioSection(section) {
    const cmp = composioConfig();
    const apiPlaceholder = cmp.configured ? t('config.composio.api_key_existing') : t('config.composio.api_key_placeholder');
    const selectedCount = Array.isArray(cmp.toolkits) ? cmp.toolkits.filter(tk => tk && tk.enabled).length : 0;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div id="composio-status" class="cmp-status-line">' + t('config.composio.status_loading') + '</div>';

    html += '<div class="cmp-toolbar">';
    html += '<button class="btn-save cfg-save-btn-sm" onclick="composioOpenModal()">' + t('config.composio.open_picker') + '</button>';
    html += '<button class="btn-secondary cfg-save-btn-sm" onclick="composioTestConnection()">' + t('config.composio.test_connection') + '</button>';
    html += '<span class="cmp-selected-pill">' + t('config.composio.selected_count', { count: selectedCount }) + '</span>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.composio.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.composio.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (cmp.enabled ? ' on' : '') + '" data-path="composio.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (cmp.enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.composio.api_key') + '</div>';
    html += '<div class="field-help">' + t('help.composio.api_key') + '</div>';
    html += '<div class="password-wrap cmp-secret-row">';
    html += '<input class="cfg-input" type="password" id="composio-api-key" data-path="composio.api_key" value="" placeholder="' + escapeAttr(apiPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '<button type="button" class="btn-save cfg-save-btn-sm" onclick="composioSaveAPIKey()">' + t('config.composio.save_api_key') + '</button>';
    html += '</div></div>';

    html += '<div class="cmp-grid">';
    html += composioInput('base_url', 'config.composio.base_url', 'help.composio.base_url', cmp.base_url || 'https://backend.composio.dev/api/v3.1', 'composio.base_url');
    html += composioInput('user_id', 'config.composio.user_id', 'help.composio.user_id', cmp.user_id || 'aurago-default', 'composio.user_id');
    html += composioInput('request_timeout_seconds', 'config.composio.timeout', 'help.composio.timeout', String(cmp.request_timeout_seconds || 60), 'composio.request_timeout_seconds', 'number');
    html += composioInput('cache_ttl_seconds', 'config.composio.cache_ttl', 'help.composio.cache_ttl', String(cmp.cache_ttl_seconds || 300), 'composio.cache_ttl_seconds', 'number');
    html += composioInput('max_result_bytes', 'config.composio.max_result', 'help.composio.max_result', String(cmp.max_result_bytes || 262144), 'composio.max_result_bytes', 'number');
    html += '</div>';

    html += '<div class="cmp-policy-row">';
    html += composioToggle('composio.read_only', cmp.read_only !== false, 'config.composio.read_only', 'help.composio.read_only');
    html += composioToggle('composio.allow_destructive', cmp.allow_destructive === true, 'config.composio.allow_destructive', 'help.composio.allow_destructive');
    html += composioToggle('composio.allow_natural_language_input', cmp.allow_natural_language_input === true, 'config.composio.allow_nl', 'help.composio.allow_nl');
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.composio.enabled_toolkits') + '</div>';
    html += '<div class="field-group-desc">' + t('config.composio.enabled_toolkits_desc') + '</div>';
    html += '<div id="composio-selected-list" class="cmp-selected-list">' + composioSelectedSummary(cmp.toolkits || []) + '</div>';
    html += '</div>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    await composioRefreshStatus();
}

function composioConfig() {
    configData.composio = configData.composio || {};
    const cmp = configData.composio;
    if (!Array.isArray(cmp.toolkits)) cmp.toolkits = [];
    if (!cmp.base_url) cmp.base_url = 'https://backend.composio.dev/api/v3.1';
    if (!cmp.user_id) cmp.user_id = 'aurago-default';
    if (cmp.read_only === undefined) cmp.read_only = true;
    if (!cmp.request_timeout_seconds) cmp.request_timeout_seconds = 60;
    if (!cmp.cache_ttl_seconds) cmp.cache_ttl_seconds = 300;
    if (!cmp.max_result_bytes) cmp.max_result_bytes = 262144;
    return cmp;
}

function composioInput(id, labelKey, helpKey, value, path, type = 'text') {
    return '<label class="cmp-field"><span class="field-label">' + t(labelKey) + '</span>' +
        '<span class="field-help">' + t(helpKey) + '</span>' +
        '<input class="field-input" type="' + type + '" id="composio-' + id + '" data-path="' + path + '" value="' + escapeAttr(value) + '"></label>';
}

function composioToggle(path, enabled, labelKey, helpKey) {
    return '<div class="cmp-policy-item"><div class="field-label">' + t(labelKey) + '</div>' +
        '<div class="field-help">' + t(helpKey) + '</div>' +
        '<div class="toggle-wrap"><div class="toggle' + (enabled ? ' on' : '') + '" data-path="' + path + '" onclick="toggleBool(this)"></div>' +
        '<span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span></div></div>';
}

function composioSelectedSummary(toolkits) {
    const enabled = (toolkits || []).filter(tk => tk && tk.enabled);
    if (enabled.length === 0) return '<div class="cmp-empty">' + t('config.composio.no_toolkits_selected') + '</div>';
    return enabled.map(tk => '<span class="cmp-chip">' + escapeAttr(tk.slug || '') + '</span>').join('');
}

function composioJSArg(value) {
    return escapeAttr(JSON.stringify(String(value || '')));
}

function composioFirstText(...values) {
    for (const value of values) {
        const text = String(value || '').trim();
        if (text) return text;
    }
    return '';
}

function composioToolkitCategory(tk) {
    const categories = tk && tk.meta && Array.isArray(tk.meta.categories) ? tk.meta.categories : [];
    for (const category of categories) {
        const text = composioFirstText(category && category.name, category && category.slug, category && category.id);
        if (text) return text;
    }
    return '';
}

function composioToolkitDescription(tk) {
    return composioFirstText(
        tk && tk.description,
        tk && tk.meta && tk.meta.description,
        tk && tk.category,
        composioToolkitCategory(tk),
        tk && tk.slug
    );
}

function composioToolDescription(tool) {
    return composioFirstText(
        tool && tool.description,
        tool && tool.human_description,
        tool && tool.meta && tool.meta.description,
        tool && tool.display_name
    );
}

function composioAuthEnabled(auth) {
    return auth && (auth.enabled === true || String(auth.status || '').toUpperCase() === 'ENABLED');
}

function composioToolSortScore(item) {
    const decision = item && item.policy_decision;
    if (decision && decision.allowed === true) return 0;
    if (!decision || typeof decision.allowed !== 'boolean') return 1;
    return 2;
}

async function composioRefreshStatus() {
    try {
        const resp = await fetch('/api/composio/status');
        const data = resp.ok ? await resp.json() : null;
        composioState.status = data;
        const el = document.getElementById('composio-status');
        if (!el || !data) return;
        const cls = data.status === 'ready' ? 'ok' : (data.status === 'missing_api_key' ? 'warn' : 'muted');
        el.className = 'cmp-status-line ' + cls;
        el.textContent = t('config.composio.status_' + data.status);
        const cmp = composioConfig();
        cmp.configured = !!data.configured;
    } catch (_) {
        const el = document.getElementById('composio-status');
        if (el) {
            el.className = 'cmp-status-line warn';
            el.textContent = t('config.composio.status_error');
        }
    }
}

async function composioSaveAPIKey() {
    const input = document.getElementById('composio-api-key');
    const value = input ? input.value.trim() : '';
    if (!value) {
        showToast(t('config.composio.api_key_empty'), 'warn');
        return;
    }
    try {
        const resp = await fetch('/api/vault/secrets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: 'composio_api_key', value })
        });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || data.message || t('config.common.error'));
        }
        cfgMarkSecretStored(input, 'composio.api_key');
        composioConfig().configured = true;
        showToast(t('config.composio.api_key_saved'), 'success');
        await composioRefreshStatus();
    } catch (e) {
        showToast(e.message || t('config.composio.api_key_save_failed'), 'error');
    }
}

async function composioTestConnection() {
    try {
        const resp = await fetch('/api/composio/test', { method: 'POST' });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok || data.status === 'error') throw new Error(data.message || data.error || t('config.common.error'));
        showToast(data.message || t('config.composio.test_ok'), 'success');
    } catch (e) {
        showToast(t('config.composio.test_failed') + ': ' + (e.message || t('config.common.error')), 'error');
    }
}

async function composioOpenModal() {
    composioEnsureModal();
    document.getElementById('composio-modal-overlay').classList.add('active');
    composioState.query = '';
    composioState.filter = 'all';
    await composioLoadSelection();
    await composioLoadToolkits(true);
}

function composioCloseModal() {
    const el = document.getElementById('composio-modal-overlay');
    if (el) el.classList.remove('active');
}

function composioEnsureModal() {
    if (document.getElementById('composio-modal-overlay')) return;
    const wrap = document.createElement('div');
    wrap.id = 'composio-modal-overlay';
    wrap.className = 'cmp-modal-overlay';
    wrap.innerHTML = '<div class="cmp-modal">' +
        '<div class="cmp-modal-head"><div><div class="cmp-modal-title">' + t('config.composio.modal_title') + '</div><div class="cmp-modal-subtitle">' + t('config.composio.modal_subtitle') + '</div></div>' +
        '<button class="cmp-icon-btn" onclick="composioCloseModal()" title="' + escapeAttr(t('config.composio.close')) + '">x</button></div>' +
        '<div class="cmp-modal-controls"><input id="composio-search" class="field-input" placeholder="' + escapeAttr(t('config.composio.search_placeholder')) + '" oninput="composioSearchChanged(this.value)">' +
        '<select id="composio-filter" class="field-input" onchange="composioFilterChanged(this.value)"><option value="all">' + t('config.composio.filter_all') + '</option><option value="selected">' + t('config.composio.filter_selected') + '</option><option value="connected">' + t('config.composio.filter_connected') + '</option></select>' +
        '<button class="btn-secondary cfg-save-btn-sm" onclick="composioLoadToolkits(true)">' + t('config.composio.refresh') + '</button>' +
        '<button class="btn-save cfg-save-btn-sm" onclick="composioSaveSelection(true)">' + t('config.composio.save_selection') + '</button>' +
        '<span id="composio-modal-count" class="cmp-selected-pill"></span></div>' +
        '<div class="cmp-modal-body"><div class="cmp-list-panel"><div id="composio-toolkit-list" class="cmp-toolkit-list"></div><button id="composio-load-more" class="btn-secondary cfg-save-btn-sm cmp-load-more" onclick="composioLoadMore()">' + t('config.composio.load_more') + '</button></div>' +
        '<div id="composio-detail" class="cmp-detail-panel"></div></div></div>';
    document.body.appendChild(wrap);
    wrap.addEventListener('click', function (ev) {
        if (ev.target === wrap) composioCloseModal();
    });
}

async function composioLoadSelection() {
    try {
        const resp = await fetch('/api/composio/selection');
        const data = resp.ok ? await resp.json() : composioConfig();
        composioState.selection = data;
        configData.composio = Object.assign(composioConfig(), data);
    } catch (_) {
        composioState.selection = composioConfig();
    }
}

async function composioLoadToolkits(reset) {
    composioState.loading = true;
    composioRenderModal();
    try {
        const q = document.getElementById('composio-search') ? document.getElementById('composio-search').value.trim() : composioState.query;
        const params = new URLSearchParams({ limit: '60' });
        if (q) params.set('q', q);
        if (!reset && composioState.nextCursor) params.set('cursor', composioState.nextCursor);
        const resp = await fetch('/api/composio/toolkits?' + params.toString());
        if (!resp.ok) throw new Error(await resp.text());
        const data = await resp.json();
        composioState.toolkits = reset ? (data.items || []) : composioState.toolkits.concat(data.items || []);
        composioState.nextCursor = data.next_cursor || '';
        if (!composioState.selectedSlug && composioState.toolkits.length > 0) {
            composioState.selectedSlug = composioState.toolkits[0].slug || '';
            composioLoadToolkitDetail(composioState.selectedSlug);
        }
    } catch (e) {
        showToast(t('config.composio.load_failed') + ': ' + (e.message || t('config.common.error')), 'error');
    } finally {
        composioState.loading = false;
        composioRenderModal();
    }
}

function composioLoadMore() {
    if (composioState.nextCursor) composioLoadToolkits(false);
}

function composioSearchChanged(value) {
    composioState.query = value;
    clearTimeout(composioState.searchTimer);
    composioState.searchTimer = setTimeout(() => composioLoadToolkits(true), 250);
}

function composioFilterChanged(value) {
    composioState.filter = value || 'all';
    composioRenderModal();
}

function composioRenderModal() {
    const list = document.getElementById('composio-toolkit-list');
    const detail = document.getElementById('composio-detail');
    const count = document.getElementById('composio-modal-count');
    const loadMore = document.getElementById('composio-load-more');
    if (!list || !detail) return;

    const selected = composioSelectedMap();
    const rows = composioFilteredToolkits(selected);
    if (count) count.textContent = t('config.composio.selected_count', { count: Object.keys(selected).length });
    list.innerHTML = rows.length ? rows.map(tk => composioToolkitRow(tk, selected)).join('') : '<div class="cmp-empty">' + (composioState.loading ? t('config.common.loading') : t('config.composio.no_results')) + '</div>';
    if (loadMore) loadMore.style.display = composioState.nextCursor ? 'block' : 'none';
    composioRenderDetail();
}

function composioFilteredToolkits(selected) {
    let rows = composioState.toolkits || [];
    if (composioState.filter === 'selected') {
        rows = rows.filter(tk => selected[(tk.slug || '').toLowerCase()]);
    }
    if (composioState.filter === 'connected') {
        const connectedSlugs = new Set((composioState.accounts || []).map(a => (a.toolkit_slug || '').toLowerCase()));
        rows = rows.filter(tk => connectedSlugs.has((tk.slug || '').toLowerCase()));
    }
    return rows.slice(0, 120);
}

function composioToolkitRow(tk, selected) {
    const slug = tk.slug || '';
    const isSelected = !!selected[slug.toLowerCase()];
    const active = slug === composioState.selectedSlug ? ' active' : '';
    return '<div class="cmp-toolkit-row' + active + '" onclick="composioLoadToolkitDetail(' + composioJSArg(slug) + ')">' +
        '<div class="cmp-toolkit-main"><div class="cmp-toolkit-name">' + escapeAttr(tk.name || slug || '-') + '</div>' +
        '<div class="cmp-toolkit-desc">' + escapeAttr(composioToolkitDescription(tk)) + '</div></div>' +
        '<button class="cmp-small-toggle ' + (isSelected ? 'on' : '') + '" onclick="event.stopPropagation(); composioToggleToolkit(' + composioJSArg(slug) + ')">' + (isSelected ? t('config.composio.enabled_short') : t('config.composio.enable_short')) + '</button>' +
        '</div>';
}

async function composioLoadToolkitDetail(slug) {
    composioState.selectedSlug = slug;
    composioState.tools = [];
    composioState.accounts = [];
    composioState.authConfigs = [];
    composioRenderModal();
    if (!slug) return;
    try {
        const [toolsResp, accountsResp, authResp] = await Promise.all([
            fetch('/api/composio/tools?toolkit_slug=' + encodeURIComponent(slug) + '&limit=25&preview=1'),
            fetch('/api/composio/connected-accounts?toolkit_slug=' + encodeURIComponent(slug)),
            fetch('/api/composio/auth-configs?toolkit_slug=' + encodeURIComponent(slug))
        ]);
        if (toolsResp.ok) {
            const data = await toolsResp.json();
            composioState.tools = data.items || [];
        }
        if (accountsResp.ok) {
            const data = await accountsResp.json();
            composioState.accounts = data.items || [];
        }
        if (authResp.ok) {
            const data = await authResp.json();
            composioState.authConfigs = data.items || [];
        }
    } catch (e) {
        showToast(t('config.composio.detail_failed') + ': ' + (e.message || t('config.common.error')), 'error');
    }
    composioRenderModal();
}

function composioRenderDetail() {
    const detail = document.getElementById('composio-detail');
    if (!detail) return;
    const slug = composioState.selectedSlug;
    const tk = (composioState.toolkits || []).find(item => item.slug === slug) || {};
    if (!slug) {
        detail.innerHTML = '<div class="cmp-empty">' + t('config.composio.select_toolkit') + '</div>';
        return;
    }
    const selected = composioSelectedMap();
    const isSelected = !!selected[slug.toLowerCase()];
    const accounts = composioState.accounts || [];
    const tools = composioState.tools || [];
    const description = composioToolkitDescription(tk);
    let html = '<div class="cmp-detail-head"><div><div class="cmp-detail-title">' + escapeAttr(tk.name || slug) + '</div><div class="cmp-detail-slug">' + escapeAttr(slug) + '</div></div>' +
        '<button class="btn-save cfg-save-btn-sm" onclick="composioToggleToolkit(' + composioJSArg(slug) + ')">' + (isSelected ? t('config.composio.disable_toolkit') : t('config.composio.enable_toolkit')) + '</button></div>';
    html += '<div class="cmp-detail-desc">' + escapeAttr(description || t('config.composio.no_description')) + '</div>';
    html += '<div class="cmp-detail-actions"><button class="btn-secondary cfg-save-btn-sm" onclick="composioConnectToolkit(' + composioJSArg(slug) + ')">' + t('config.composio.connect') + '</button>' +
        '<button class="btn-secondary cfg-save-btn-sm" onclick="composioLoadToolkitDetail(' + composioJSArg(slug) + ')">' + t('config.composio.refresh') + '</button></div>';
    html += '<div class="cmp-detail-grid"><div><div class="cmp-mini-title">' + t('config.composio.accounts') + '</div>' + composioAccountsHTML(accounts) + '</div>' +
        '<div><div class="cmp-mini-title">' + t('config.composio.tools_preview') + '</div>' + composioToolsHTML(tools) + '</div></div>';
    detail.innerHTML = html;
}

function composioAccountsHTML(accounts) {
    if (!accounts || accounts.length === 0) return '<div class="cmp-empty">' + t('config.composio.no_accounts') + '</div>';
    return accounts.map(a => '<div class="cmp-account-row"><span>' + escapeAttr(a.id || '-') + '</span><span>' + escapeAttr(a.status || '-') + '</span></div>').join('');
}

function composioToolsHTML(items) {
    if (!items || items.length === 0) return '<div class="cmp-empty">' + t('config.composio.no_tools') + '</div>';
    return items.slice().sort((a, b) => composioToolSortScore(a) - composioToolSortScore(b)).slice(0, 12).map(item => {
        const tool = item.tool || item;
        const decision = item.policy_decision;
        const status = decision && decision.allowed === true ? t('config.composio.allowed') : (decision && decision.allowed === false ? t('config.composio.blocked') : '');
        const title = decision && decision.reason ? ' title="' + escapeAttr(decision.reason) + '"' : '';
        return '<div class="cmp-tool-row"' + title + '><div><strong>' + escapeAttr(tool.slug || tool.name || '-') + '</strong><small>' + escapeAttr(composioToolDescription(tool)) + '</small></div><span>' + escapeAttr(status) + '</span></div>';
    }).join('');
}

function composioSelectedMap() {
    const cfg = composioConfig();
    const map = {};
    (cfg.toolkits || []).forEach(tk => {
        if (tk && tk.enabled && tk.slug) map[tk.slug.toLowerCase()] = tk;
    });
    return map;
}

function composioToggleToolkit(slug) {
    const cfg = composioConfig();
    const normalized = (slug || '').trim();
    if (!normalized) return;
    let entry = cfg.toolkits.find(tk => (tk.slug || '').toLowerCase() === normalized.toLowerCase());
    if (!entry) {
        entry = { slug: normalized, enabled: true, allowed_tool_slugs: [], blocked_tool_slugs: [] };
        cfg.toolkits.push(entry);
    } else {
        entry.enabled = !entry.enabled;
    }
    composioRenderModal();
    const summary = document.getElementById('composio-selected-list');
    if (summary) summary.innerHTML = composioSelectedSummary(cfg.toolkits);
}

async function composioSaveSelection(toast) {
    const cfg = composioConfig();
    try {
        const resp = await fetch('/api/composio/selection', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                enabled: cfg.enabled === true,
                base_url: cfg.base_url || 'https://backend.composio.dev/api/v3.1',
                user_id: cfg.user_id || 'aurago-default',
                read_only: cfg.read_only !== false,
                allow_destructive: cfg.allow_destructive === true,
                allow_natural_language_input: cfg.allow_natural_language_input === true,
                request_timeout_seconds: Number(cfg.request_timeout_seconds || 60),
                cache_ttl_seconds: Number(cfg.cache_ttl_seconds || 300),
                max_result_bytes: Number(cfg.max_result_bytes || 262144),
                toolkits: cfg.toolkits || []
            })
        });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || data.message || t('config.common.error'));
        configData.composio = Object.assign(composioConfig(), data);
        if (toast) showToast(t('config.composio.selection_saved'), 'success');
        await composioRefreshStatus();
    } catch (e) {
        showToast(t('config.composio.selection_save_failed') + ': ' + (e.message || t('config.common.error')), 'error');
    }
}

async function composioConnectToolkit(slug) {
    const auth = composioState.authConfigs || [];
    const preferred = auth.find(a => composioAuthEnabled(a) && a.is_composio_managed) || auth.find(a => composioAuthEnabled(a)) || auth[0];
    if (!preferred || !preferred.id) {
        showToast(t('config.composio.no_auth_config'), 'warn');
        return;
    }
    const popup = window.open('about:blank', '_blank');
    try {
        const resp = await fetch('/api/composio/connect-link', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ toolkit_slug: slug, auth_config_id: preferred.id })
        });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || data.message || t('config.common.error'));
        const link = data.link || {};
        const url = link.redirect_url || link.link;
        if (!url) throw new Error(t('config.composio.no_connect_url'));
        if (!popup) throw new Error(t('config.composio.connect_failed'));
        popup.opener = null;
        popup.location.href = url;
        showToast(t('config.composio.connect_opened'), 'success');
    } catch (e) {
        if (popup && !popup.closed) popup.close();
        showToast(t('config.composio.connect_failed') + ': ' + (e.message || t('config.common.error')), 'error');
    }
}
