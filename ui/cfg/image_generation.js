let _imggenSection = null;

function renderImageGenerationSection(section) {
    if (section) _imggenSection = section; else section = _imggenSection;
    const cfg = configData.image_generation || {};
    const enabled = cfg.enabled === true;
    const form = window.AuraConfigForm;
    const key = name => t('config.image_generation.' + name);

    // The switch re-renders the section: the provider groups only exist while the feature is on.
    const enableToggle = form.toggle({ label: key('enabled_label'), help: key('enabled_help'), path: 'image_generation.enabled', value: enabled })
        .replace('onclick="toggleBool(this)"', 'onclick="toggleBool(this);setNestedValue(configData,\'image_generation.enabled\',this.classList.contains(\'on\'));renderImageGenerationSection(null)"');
    const groups = [{ titleKey: 'config.refresh.operation', fields: [enableToggle] }];
    let after = '';

    if (!enabled) {
        after = form.emptyState({ title: key('disabled_notice'), desc: key('disabled_desc') });
    } else {
        const curProvider = cfg.provider || '';
        const providers = configGeneralProviders();
        const providerOptions = [{ value: '', label: key('select_provider') }].concat(providers.map(p => ({
            value: p.id,
            label: (p.name || p.id) + (p.type ? ' [' + p.type + ']' : '') + (p.model ? ' — ' + p.model : '')
        })));
        const providerSelect = form.select({ label: key('provider_label'), help: key('provider_help'), path: 'image_generation.provider', value: curProvider, options: providerOptions })
            .replace('</select>', configPreservedProviderOptionHTML(curProvider, providers) + '</select>');
        const choices = values => values.map(v => ({ value: v, label: v }));
        groups.push({ title: key('provider_title'), desc: key('provider_desc'), fields: [
            providerSelect,
            form.field({ label: key('model_label'), help: key('model_hint'), path: 'image_generation.model', value: cfg.model || '', placeholder: 'dall-e-3, stable-diffusion-3, imagen-3.0-generate-002...' })
        ] });
        groups.push({ title: key('defaults_title'), desc: key('defaults_desc'), fields: [
            form.select({ label: key('size_label'), help: key('size_help'), path: 'image_generation.default_size', value: cfg.default_size || '1024x1024', options: choices(['256x256', '512x512', '1024x1024', '1024x1792', '1792x1024']) }),
            form.select({ label: key('quality_label'), help: key('quality_help'), path: 'image_generation.default_quality', value: cfg.default_quality || 'standard', options: choices(['standard', 'hd']) }),
            form.select({ label: key('style_label'), help: key('style_help'), path: 'image_generation.default_style', value: cfg.default_style || 'natural', options: choices(['natural', 'vivid']) })
        ] });
        groups.push({ title: key('enhancement_title'), desc: key('enhancement_desc'), fields: [
            form.toggle({ label: key('prompt_enhancement_label'), help: key('prompt_enhancement_help'), path: 'image_generation.prompt_enhancement', value: cfg.prompt_enhancement === true }),
            form.number({ label: key('max_monthly_label'), help: key('max_monthly_hint'), path: 'image_generation.max_monthly', value: cfg.max_monthly || 0, min: 0 }),
            form.number({ label: key('max_daily_label'), help: key('max_daily_hint'), path: 'image_generation.max_daily', value: cfg.max_daily || 0, min: 0 })
        ] });
        groups.push({ title: key('test_title'), desc: key('test_desc'), content: form.actions([
            { html: '<button class="btn-save cfg-test-btn" id="imggen-test-btn" onclick="imggenTestConnection()" data-config-action-draft="true">' + cfgIcon('flask') + '<span>' + escapeHtml(key('test_btn')) + '</span></button>' },
            { html: cfgActionResult.html('imggen-test-result') }
        ]) + '<div id="imggen-test-preview" class="ig-test-preview"></div>' });
    }

    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc, groups, afterHTML: after });
    attachChangeListeners();
}

// Generates one real test image with the provider selected in the form (unsaved included).
async function imggenTestConnection() {
    const previewEl = document.getElementById('imggen-test-preview');
    if (previewEl) previewEl.innerHTML = '';
    return cfgActionResult(document.getElementById('imggen-test-btn'), () => fetch('/api/image-generation/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['image_generation']) })
    }), {
        target: 'imggen-test-result',
        pending: t('config.image_generation.testing'),
        success: t('config.image_generation.test_success'),
        failure: t('config.image_generation.test_failed'),
        onSuccess: outcome => {
            const webPath = outcome.body && outcome.body.web_path;
            const preview = document.getElementById('imggen-test-preview');
            if (preview && webPath) preview.innerHTML = `<img src="${escapeAttr(webPath)}" class="ig-preview-img">`;
        }
    });
}
