// cfg/video_generation.js - Video Generation config section

let _videoSection = null;

function videoGenerationProviderType(providerId) {
    const provider = providersCache.find(p => String(p.id) === String(providerId));
    return String(provider && provider.type ? provider.type : '').trim().toLowerCase();
}

function videoGenerationProviderChanged(select) {
    setNestedValue(configData, 'video_generation.provider', select.value);
    if (typeof markDirty === 'function') markDirty();
    renderVideoGenerationSection(null);
}

function renderVideoGenerationSection(section) {
    if (section) _videoSection = section; else section = _videoSection;
    const cfg = configData['video_generation'] || {};
    const enabled = cfg.enabled === true;
    const form = window.AuraConfigForm;
    const key = name => t('config.video_gen.' + name);

    // The switch re-renders the section: the provider groups only exist while the feature is on.
    const enableToggle = form.toggle({ label: key('enabled_label'), help: key('enabled_help'), path: 'video_generation.enabled', value: enabled, onclick: "toggleBool(this);setNestedValue(configData,'video_generation.enabled',this.classList.contains('on'));renderVideoGenerationSection(null)" });
    const groups = [{ titleKey: 'config.refresh.operation', fields: [enableToggle] }];
    let after = '';

    if (!enabled) {
        after = form.emptyState({ title: key('disabled_notice'), desc: key('disabled_desc') });
    } else {
        const curProvider = cfg.provider || '';
        const providers = configGeneralProviders();
        const isAgnes = videoGenerationProviderType(curProvider) === 'agnes';
        const resolutionOptions = isAgnes ? ['480p','720p','768P','1080p'] : ['768P','1080P','720p','4k'];
        const aspectRatioOptions = isAgnes ? ['16:9','9:16','1:1','4:3','3:4'] : ['16:9','9:16','1:1'];
        const currentResolution = String(cfg.default_resolution || '768P').toLowerCase();
        const currentAspectRatio = String(cfg.default_aspect_ratio || '16:9');
        const providerOptions = [{ value: '', label: key('select_provider') }].concat(providers.map(p => ({
            value: p.id,
            label: (p.name || p.id) + (p.type ? ' [' + p.type + ']' : '') + (p.model ? ' - ' + p.model : '')
        })));
        const providerSelect = form.select({ label: key('provider_label'), help: key('provider_help'), path: 'video_generation.provider', value: curProvider, options: providerOptions, onchange: 'videoGenerationProviderChanged(this)', extraOptionsHTML: configPreservedProviderOptionHTML(curProvider, providers) });
        groups.push({ title: key('provider_title'), desc: key('provider_desc'), fields: [
            providerSelect,
            form.field({ label: key('model_label'), help: key('model_hint'), path: 'video_generation.model', value: cfg.model || '', placeholder: 'MiniMax-Hailuo-2.3, veo-3.1-generate-preview...' })
        ] });
        groups.push({ title: key('defaults_title'), desc: key('defaults_desc'), fields: [
            form.number({ label: key('duration_label'), help: key('duration_help'), path: 'video_generation.default_duration_seconds', value: cfg.default_duration_seconds || 6, min: 1, max: 30 }),
            form.select({ label: key('resolution_label'), help: key('resolution_help'), path: 'video_generation.default_resolution', value: (resolutionOptions.find(v => v.toLowerCase() === currentResolution) || ''), options: resolutionOptions.map(v => ({ value: v, label: v })) }),
            form.select({ label: key('aspect_ratio_label'), help: key('aspect_ratio_help'), path: 'video_generation.default_aspect_ratio', value: currentAspectRatio, options: aspectRatioOptions.map(v => ({ value: v, label: v })) })
        ] });
        groups.push({ title: key('limits_title'), desc: key('limits_desc'), fields: [
            form.number({ label: key('poll_interval_label'), help: key('poll_interval_help'), path: 'video_generation.poll_interval_seconds', value: cfg.poll_interval_seconds || 10, min: 1, max: 120 }),
            form.number({ label: key('timeout_label'), help: key('timeout_help'), path: 'video_generation.timeout_seconds', value: cfg.timeout_seconds || 600, min: 30, max: 3600 }),
            form.number({ label: key('max_daily_label'), help: key('max_daily_help'), path: 'video_generation.max_daily', value: cfg.max_daily || 0, min: 0 })
        ] });
        groups.push({ title: key('test_title'), desc: key('test_desc'), content: form.actions([
            { html: '<button class="btn-save cfg-test-btn" id="video-test-btn" onclick="videoTestConnection()" data-config-action-draft="true">' + cfgIcon('plug') + '<span>' + escapeHtml(key('test_btn')) + '</span></button>' },
            { html: cfgActionResult.html('video-test-result') }
        ]) });
    }

    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc, groups, afterHTML: after });
    attachChangeListeners();
}

// Checks the provider selected in the form (unsaved included).
function videoTestConnection() {
    return cfgActionResult(document.getElementById('video-test-btn'), () => fetch('/api/video-generation/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['video_generation']) })
    }), {
        target: 'video-test-result',
        pending: t('config.video_gen.testing'),
        success: outcome => outcome.reason || t('config.video_gen.test_success'),
        failure: t('config.video_gen.test_failed')
    });
}
