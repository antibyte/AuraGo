let _yepapiSection = null;

function yepapiToggleEnabled(el) {
    toggleBool(el);
    setNestedValue(configData, 'yepapi.enabled', el.classList.contains('on'));
    renderYepAPISection(null);
}

async function renderYepAPISection(section) {
    if (section) _yepapiSection = section; else section = _yepapiSection;
    const cfg = configData.yepapi || {};
    const enabled = cfg.enabled === true;
    const form = window.AuraConfigForm;

    // The enable switch re-renders the section: the other settings only show while it is on.
    const enableToggle = form.toggle({ label: t('config.yepapi.enabled_label'), help: t('config.yepapi.enabled_help'), path: 'yepapi.enabled', value: enabled })
        .replace('onclick="toggleBool(this)"', 'onclick="yepapiToggleEnabled(this)"');

    if (!enabled) {
        document.getElementById('content').innerHTML = form.renderSpec({
            label: section.label,
            desc: section.desc,
            key: 'yepapi',
            fields: [enableToggle],
            afterHTML: form.emptyState({ title: t('config.yepapi.disabled_notice'), desc: t('config.yepapi.disabled_desc') })
        });
        attachChangeListeners();
        return;
    }

    let currentProvider = cfg.provider || '';
    // Auto-select first yepapi provider if none selected
    if (!currentProvider && typeof providersCache !== 'undefined' && providersCache) {
        const yepProvider = providersCache.find(p => String(p.type || '').toLowerCase() === 'yepapi');
        if (yepProvider) {
            currentProvider = yepProvider.id;
            setNestedValue(configData, 'yepapi.provider', currentProvider);
        }
    }

    const providerOptions = [{ value: '', label: t('config.yepapi.provider_none') }];
    if (typeof providersCache !== 'undefined' && providersCache) {
        providersCache.forEach(p => {
            if (String(p.type || '').toLowerCase() !== 'yepapi') return;
            const model = p.model ? (' — ' + p.model) : '';
            providerOptions.push({ value: p.id, label: (p.name || p.id) + (p.type ? ' [' + p.type + ']' : '') + model });
        });
    }

    const services = [
        { key: 'seo', label: 'seo_label', hint: 'seo_hint' },
        { key: 'serp', label: 'serp_label', hint: 'serp_hint' },
        { key: 'scraping', label: 'scraping_label', hint: 'scraping_hint' },
        { key: 'youtube', label: 'youtube_label', hint: 'youtube_hint' },
        { key: 'tiktok', label: 'tiktok_label', hint: 'tiktok_hint' },
        { key: 'instagram', label: 'instagram_label', hint: 'instagram_hint' },
        { key: 'amazon', label: 'amazon_label', hint: 'amazon_hint' }
    ];

    document.getElementById('content').innerHTML = form.renderSpec({
        label: section.label,
        desc: section.desc,
        key: 'yepapi',
        fields: [enableToggle],
        groups: [{
            title: t('config.yepapi.provider_title'),
            desc: t('config.yepapi.provider_desc'),
            fields: [form.select({
                label: t('config.yepapi.provider_label'),
                help: t('config.yepapi.provider_help'),
                path: 'yepapi.provider',
                value: String(currentProvider),
                options: providerOptions
            })]
        }, {
            title: t('config.yepapi.services_title'),
            desc: t('config.yepapi.services_desc'),
            fields: services.map(svc => form.toggle({
                id: 'yepapi-' + svc.key + '-toggle',
                label: t('config.yepapi.' + svc.label),
                help: t('config.yepapi.' + svc.hint),
                path: 'yepapi.' + svc.key + '.enabled',
                value: (cfg[svc.key] || {}).enabled === true
            }))
        }, {
            title: t('config.yepapi.test_title'),
            desc: t('config.yepapi.test_desc'),
            content: form.actions([
                { html: '<button class="btn-save cfg-test-btn" id="yepapi-test-btn" onclick="yepapiTestConnection()" data-config-action-draft="true">' + cfgIcon('flask') + '<span>' + escapeHtml(t('config.yepapi.test_btn')) + '</span></button>' },
                { html: cfgActionResult.html('yepapi-test-result') }
            ])
        }]
    });
    attachChangeListeners();
}

// Probes with the provider selected in the form (unsaved included).
async function yepapiTestConnection() {
    return cfgActionResult(document.getElementById('yepapi-test-btn'), () => fetch('/api/yepapi/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['yepapi']) })
    }), {
        target: 'yepapi-test-result',
        pending: t('config.yepapi.testing'),
        success: t('config.yepapi.test_success'),
        failure: t('config.yepapi.test_error')
    });
}
