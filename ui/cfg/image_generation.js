let _imggenSection = null;

async function renderImageGenerationSection(section) {
    if (section) _imggenSection = section; else section = _imggenSection;
    const cfg = configData.image_generation || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.image_generation.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="image_generation.enabled" aria-describedby="imggen-enabled-help" onclick="toggleBool(this);setNestedValue(configData,'image_generation.enabled',this.classList.contains('on'));renderImageGenerationSection(null)"></div>
    </div>
    <div class="field-help" id="imggen-enabled-help">${t('config.image_generation.enabled_help')}</div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>🎨</span>
            <div>
                <strong>${t('config.image_generation.disabled_notice')}</strong><br>
                <small>${t('config.image_generation.disabled_desc')}</small>
            </div>
        </div>`;
        html += `</div>`;
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.provider_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.provider_desc')}</div>`;

    const curProvider = cfg.provider || '';
    html += `<div class="field-grid two-cols">`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.image_generation.provider_label')}</div>
        <div class="field-help">${t('config.image_generation.provider_help')}</div>
        <select class="field-select" data-path="image_generation.provider">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.image_generation.select_provider')}</option>`;
    configGeneralProviders().forEach(p => {
        const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        const name = p.name || p.id;
        const badge = p.type ? (' [' + p.type + ']') : '';
        const model = p.model ? (' — ' + p.model) : '';
        html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
    });
    html += configPreservedProviderOptionHTML(curProvider, configGeneralProviders());
    html += `</select></div>`;

    const curModel = cfg.model || '';
    html += `<div class="field-group">
        <div class="field-label">${t('config.image_generation.model_label')}</div>
        <div class="field-help">${t('config.image_generation.model_hint')}</div>
        <input type="text" class="field-input" data-path="image_generation.model" value="${escapeAttr(curModel)}"
            placeholder="dall-e-3, stable-diffusion-3, imagen-3.0-generate-002...">
    </div></div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.defaults_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.defaults_desc')}</div>`;

    const sizes = ['256x256', '512x512', '1024x1024', '1024x1792', '1792x1024'];
    const curSize = cfg.default_size || '1024x1024';
    html += `<div class="field-grid two-cols">`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.image_generation.size_label')}</div>
        <div class="field-help">${t('config.image_generation.size_help')}</div>
        <select class="field-select" data-path="image_generation.default_size">`;
    sizes.forEach(s => {
        const sel = (curSize === s) ? ' selected' : '';
        html += `<option value="${s}"${sel}>${s}</option>`;
    });
    html += `</select></div>`;

    const qualities = ['standard', 'hd'];
    const curQuality = cfg.default_quality || 'standard';
    html += `<div class="field-group">
        <div class="field-label">${t('config.image_generation.quality_label')}</div>
        <div class="field-help">${t('config.image_generation.quality_help')}</div>
        <select class="field-select" data-path="image_generation.default_quality">`;
    qualities.forEach(q => {
        const sel = (curQuality === q) ? ' selected' : '';
        html += `<option value="${q}"${sel}>${q}</option>`;
    });
    html += `</select></div>`;

    const styles = ['natural', 'vivid'];
    const curStyle = cfg.default_style || 'natural';
    html += `<div class="field-group">
        <div class="field-label">${t('config.image_generation.style_label')}</div>
        <div class="field-help">${t('config.image_generation.style_help')}</div>
        <select class="field-select" data-path="image_generation.default_style">`;
    styles.forEach(st => {
        const sel = (curStyle === st) ? ' selected' : '';
        html += `<option value="${st}"${sel}>${st}</option>`;
    });
    html += `</select></div></div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.enhancement_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.enhancement_desc')}</div>`;

    const enhanceOn = cfg.prompt_enhancement === true;
    html += `<div class="ig-toggle-row">
        <span class="ig-label-text">${t('config.image_generation.prompt_enhancement_label')}</span>
        <div class="toggle ${enhanceOn ? 'on' : ''}" data-path="image_generation.prompt_enhancement" aria-describedby="imggen-enhance-help" onclick="toggleBool(this)"></div>
    </div>
    <div class="field-help" id="imggen-enhance-help">${t('config.image_generation.prompt_enhancement_help')}</div>`;

    const curMax = cfg.max_monthly || 0;
    const curMaxDaily = cfg.max_daily || 0;
    html += `<div class="field-grid two-cols">`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.image_generation.max_monthly_label')}</div>
        <div class="field-help">${t('config.image_generation.max_monthly_hint')}</div>
        <input type="number" class="field-input" data-path="image_generation.max_monthly" value="${curMax}" min="0" placeholder="0">
    </div>`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.image_generation.max_daily_label')}</div>
        <div class="field-help">${t('config.image_generation.max_daily_hint')}</div>
        <input type="number" class="field-input" data-path="image_generation.max_daily" value="${curMaxDaily}" min="0" placeholder="0">
    </div></div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.test_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.test_desc')}</div>
        <div class="cfg-actions-row">
            <button class="btn-save adg-test-btn" id="imggen-test-btn" onclick="imggenTestConnection()" data-config-action-draft="true">
                🧪 ${t('config.image_generation.test_btn')}
            </button>
            ${cfgActionResult.html('imggen-test-result')}
        </div>
        <div id="imggen-test-preview" class="ig-test-preview"></div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
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
