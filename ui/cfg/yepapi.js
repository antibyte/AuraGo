let _yepapiSection = null;

async function renderYepAPISection(section) {
    if (section) _yepapiSection = section; else section = _yepapiSection;
    const cfg = configData.yepapi || {};
    const enabled = cfg.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Main toggle ──
    html += '<div class="cfg-toggle-row-highlight">';
    html += '<span class="cfg-toggle-label">' + t('config.yepapi.enabled_label') + '</span>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="yepapi.enabled" onclick="toggleBool(this);setNestedValue(configData,\'yepapi.enabled\',this.classList.contains(\'on\'));renderYepAPISection(null)"></div>';
    html += '</div>';

    if (!enabled) {
        html += '<div class="wh-notice">';
        html += '<span>🌐</span>';
        html += '<div>';
        html += '<strong>' + t('config.yepapi.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.yepapi.disabled_desc') + '</small>';
        html += '</div></div>';
        html += '</div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    // ── Provider select ──
    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.yepapi.provider_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.yepapi.provider_desc') + '</div>';

    let currentProvider = cfg.provider || '';
    // Auto-select first yepapi provider if none selected
    if (!currentProvider && typeof providersCache !== 'undefined' && providersCache) {
        const yepProvider = providersCache.find(p => p.type === 'yepapi');
        if (yepProvider) {
            currentProvider = yepProvider.id;
            setNestedValue(configData, 'yepapi.provider', currentProvider);
        }
    }

    html += '<label class="ig-label">';
    html += '<span class="ig-label-text">' + t('config.yepapi.provider_label') + '</span>';
    html += '<select class="cfg-input cfg-input-full" data-path="yepapi.provider" onchange="setNestedValue(configData,\'yepapi.provider\',this.value);setDirty(true)">';
    html += '<option value=""' + (!currentProvider ? ' selected' : '') + '>' + t('config.yepapi.provider_none') + '</option>';
    if (typeof providersCache !== 'undefined' && providersCache) {
        providersCache.forEach(p => {
            const sel = (String(currentProvider) === String(p.id)) ? ' selected' : '';
            const name = p.name || p.id;
            const badge = p.type ? (' [' + p.type + ']') : '';
            const model = p.model ? (' — ' + p.model) : '';
            html += '<option value="' + escapeAttr(p.id) + '"' + sel + '>' + escapeAttr(name + badge + model) + '</option>';
        });
    }
    html += '</select></label>';
    html += '<div class="field-help" style="margin-top:0.4rem;font-size:0.82rem;color:var(--text-secondary);">' + t('config.yepapi.provider_help') + '</div>';
    html += '</div>';

    // ── Services ──
    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.yepapi.services_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.yepapi.services_desc') + '</div>';

    const services = [
        { key: 'seo', label: 'seo_label', hint: 'seo_hint' },
        { key: 'serp', label: 'serp_label', hint: 'serp_hint' },
        { key: 'scraping', label: 'scraping_label', hint: 'scraping_hint' },
        { key: 'youtube', label: 'youtube_label', hint: 'youtube_hint' },
        { key: 'tiktok', label: 'tiktok_label', hint: 'tiktok_hint' },
        { key: 'instagram', label: 'instagram_label', hint: 'instagram_hint' },
        { key: 'amazon', label: 'amazon_label', hint: 'amazon_hint' }
    ];

    services.forEach(svc => {
        const svcCfg = cfg[svc.key] || {};
        const svcEnabled = svcCfg.enabled === true;
        html += '<div class="ig-toggle-row" style="padding:0.5rem 0;border-bottom:1px solid var(--border-subtle);">';
        html += '<div style="flex:1;">';
        html += '<div style="font-weight:500;">' + t('config.yepapi.' + svc.label) + '</div>';
        html += '<div style="font-size:0.82rem;color:var(--text-secondary);">' + t('config.yepapi.' + svc.hint) + '</div>';
        html += '</div>';
        html += '<div class="toggle ' + (svcEnabled ? 'on' : '') + '" data-path="yepapi.' + svc.key + '.enabled" onclick="toggleBool(this)"></div>';
        html += '</div>';
    });
    html += '</div>';

    // ── Test connection ──
    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.yepapi.test_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.yepapi.test_desc') + '</div>';
    html += '<div class="ig-flex-row">';
    html += '<button class="btn-save cfg-save-btn-sm" id="yepapi-test-btn" onclick="yepapiTestConnection()">';
    html += '🧪 ' + t('config.yepapi.test_btn');
    html += '</button>';
    html += '<span id="yepapi-test-status" class="ig-test-status"></span>';
    html += '</div>';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

async function yepapiTestConnection() {
    const btn = document.getElementById('yepapi-test-btn');
    const statusEl = document.getElementById('yepapi-test-status');
    if (!btn || !statusEl) return;

    btn.disabled = true;
    btn.textContent = '⏳ ' + t('config.yepapi.testing');
    statusEl.textContent = '';
    statusEl.classList.remove('ig-status-success', 'ig-status-error');

    try {
        const resp = await fetch('/api/yepapi/test');
        const data = await resp.json();
        if (data.status === 'ok') {
            statusEl.classList.add('ig-status-success');
            statusEl.textContent = '✓ ' + (data.message || t('config.yepapi.test_success'));
        } else {
            statusEl.classList.add('ig-status-error');
            statusEl.textContent = '✗ ' + (data.message || t('config.yepapi.test_error'));
        }
    } catch (e) {
        statusEl.classList.add('ig-status-error');
        statusEl.textContent = '✗ ' + e.message;
    } finally {
        btn.disabled = false;
        btn.textContent = '🧪 ' + t('config.yepapi.test_btn');
    }
}
