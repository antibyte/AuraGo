let _imggenSection = null;

async function renderImageGenerationSection(section) {
    if (section) _imggenSection = section; else section = _imggenSection;
    const cfg = configData.image_generation || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.image_generation.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="image_generation.enabled" onclick="toggleBool(this);setNestedValue(configData,'image_generation.enabled',this.classList.contains('on'));renderImageGenerationSection(null)"></div>
    </div>`;

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
    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.image_generation.provider_label')}</span>
        <select class="cfg-input cfg-input-full" data-path="image_generation.provider"
            onchange="setNestedValue(configData,'image_generation.provider',this.value);setDirty(true)">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.image_generation.select_provider')}</option>`;
    providersCache.forEach(p => {
        const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        const name = p.name || p.id;
        const badge = p.type ? (' [' + p.type + ']') : '';
        const model = p.model ? (' — ' + p.model) : '';
        html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
    });
    html += `</select></label>`;

    const curModel = cfg.model || '';
    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.image_generation.model_label')} <small class="ig-hint">(${t('config.image_generation.model_hint')})</small></span>
        <input type="text" class="cfg-input cfg-input-full" data-path="image_generation.model" value="${escapeAttr(curModel)}"
            placeholder="dall-e-3, stable-diffusion-3, imagen-3.0-generate-002...">
    </label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.defaults_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.defaults_desc')}</div>`;

    const sizes = ['256x256', '512x512', '1024x1024', '1024x1792', '1792x1024'];
    const curSize = cfg.default_size || '1024x1024';
    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.image_generation.size_label')}</span>
        <select class="cfg-input cfg-input-full" data-path="image_generation.default_size"
            onchange="setNestedValue(configData,'image_generation.default_size',this.value);setDirty(true)">`;
    sizes.forEach(s => {
        const sel = (curSize === s) ? ' selected' : '';
        html += `<option value="${s}"${sel}>${s}</option>`;
    });
    html += `</select></label>`;

    const qualities = ['standard', 'hd'];
    const curQuality = cfg.default_quality || 'standard';
    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.image_generation.quality_label')}</span>
        <select class="cfg-input cfg-input-full" data-path="image_generation.default_quality"
            onchange="setNestedValue(configData,'image_generation.default_quality',this.value);setDirty(true)">`;
    qualities.forEach(q => {
        const sel = (curQuality === q) ? ' selected' : '';
        html += `<option value="${q}"${sel}>${q}</option>`;
    });
    html += `</select></label>`;

    const styles = ['natural', 'vivid'];
    const curStyle = cfg.default_style || 'natural';
    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.image_generation.style_label')}</span>
        <select class="cfg-input cfg-input-full" data-path="image_generation.default_style"
            onchange="setNestedValue(configData,'image_generation.default_style',this.value);setDirty(true)">`;
    styles.forEach(st => {
        const sel = (curStyle === st) ? ' selected' : '';
        html += `<option value="${st}"${sel}>${st}</option>`;
    });
    html += `</select></label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.enhancement_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.enhancement_desc')}</div>`;

    const enhanceOn = cfg.prompt_enhancement === true;
    html += `<div class="ig-toggle-row">
        <span class="ig-label-text">${t('config.image_generation.prompt_enhancement_label')}</span>
        <div class="toggle ${enhanceOn ? 'on' : ''}" data-path="image_generation.prompt_enhancement" onclick="toggleBool(this)"></div>
    </div>`;

    const curMax = cfg.max_monthly || 0;
    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.image_generation.max_monthly_label')} <small class="ig-hint">(${t('config.image_generation.max_monthly_hint')})</small></span>
        <input type="number" class="cfg-input cfg-input-full" data-path="image_generation.max_monthly" value="${curMax}" min="0"
            placeholder="0">
    </label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.test_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.test_desc')}</div>
        <div class="ig-flex-row">
            <button class="btn-save cfg-save-btn-sm" id="imggen-test-btn"
                onclick="imggenTestConnection()">
                🧪 ${t('config.image_generation.test_btn')}
            </button>
            <span id="imggen-test-status" class="ig-test-status"></span>
        </div>
        <div id="imggen-test-preview" class="ig-test-preview"></div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

async function imggenTestConnection() {
    const btn = document.getElementById('imggen-test-btn');
    const statusEl = document.getElementById('imggen-test-status');
    const previewEl = document.getElementById('imggen-test-preview');
    btn.disabled = true;
    btn.textContent = '⏳ ' + t('config.image_generation.testing');
    statusEl.textContent = '';
    statusEl.classList.remove('ig-status-success', 'ig-status-error');
    previewEl.innerHTML = '';
    try {
        const resp = await fetch('/api/image-generation/test');
        const data = await resp.json();
        if (data.status === 'ok') {
            statusEl.classList.add('ig-status-success');
            statusEl.textContent = '✓ ' + (data.message || t('config.image_generation.test_success'));
            if (data.web_path) {
                previewEl.innerHTML = `<img src="${escapeAttr(data.web_path)}" class="ig-preview-img">`;
            }
        } else {
            statusEl.classList.add('ig-status-error');
            statusEl.textContent = '✗ ' + (data.message || t('config.image_generation.test_failed'));
        }
    } catch (e) {
        statusEl.classList.add('ig-status-error');
        statusEl.textContent = '✗ ' + e.message;
    } finally {
        btn.disabled = false;
        btn.textContent = '🧪 ' + t('config.image_generation.test_btn');
    }
}
