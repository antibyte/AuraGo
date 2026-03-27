// cfg/image_generation.js — Image Generation config section module

let _imggenSection = null;

async function renderImageGenerationSection(section) {
    if (section) _imggenSection = section; else section = _imggenSection;
    const cfg = configData.image_generation || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Enabled toggle ──
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

    // ── Provider ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.provider_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.provider_desc')}</div>`;

    const curProvider = cfg.provider || '';
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.image_generation.provider_label')}</span>
        <select class="cfg-input" data-path="image_generation.provider" style="width:100%;margin-top:0.2rem;"
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

    // Model
    const curModel = cfg.model || '';
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.image_generation.model_label')} <small style="color:var(--text-tertiary);">(${t('config.image_generation.model_hint')})</small></span>
        <input type="text" class="cfg-input" data-path="image_generation.model" value="${escapeAttr(curModel)}"
            placeholder="dall-e-3, stable-diffusion-3, imagen-3.0-generate-002..."
            style="width:100%;margin-top:0.2rem;">
    </label>`;
    html += `</div>`;

    // ── Defaults ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.defaults_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.defaults_desc')}</div>`;

    // Size
    const sizes = ['256x256', '512x512', '1024x1024', '1024x1792', '1792x1024'];
    const curSize = cfg.default_size || '1024x1024';
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.image_generation.size_label')}</span>
        <select class="cfg-input" data-path="image_generation.default_size" style="width:100%;margin-top:0.2rem;"
            onchange="setNestedValue(configData,'image_generation.default_size',this.value);setDirty(true)">`;
    sizes.forEach(s => {
        const sel = (curSize === s) ? ' selected' : '';
        html += `<option value="${s}"${sel}>${s}</option>`;
    });
    html += `</select></label>`;

    // Quality
    const qualities = ['standard', 'hd'];
    const curQuality = cfg.default_quality || 'standard';
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.image_generation.quality_label')}</span>
        <select class="cfg-input" data-path="image_generation.default_quality" style="width:100%;margin-top:0.2rem;"
            onchange="setNestedValue(configData,'image_generation.default_quality',this.value);setDirty(true)">`;
    qualities.forEach(q => {
        const sel = (curQuality === q) ? ' selected' : '';
        html += `<option value="${q}"${sel}>${q}</option>`;
    });
    html += `</select></label>`;

    // Style
    const styles = ['natural', 'vivid'];
    const curStyle = cfg.default_style || 'natural';
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.image_generation.style_label')}</span>
        <select class="cfg-input" data-path="image_generation.default_style" style="width:100%;margin-top:0.2rem;"
            onchange="setNestedValue(configData,'image_generation.default_style',this.value);setDirty(true)">`;
    styles.forEach(st => {
        const sel = (curStyle === st) ? ' selected' : '';
        html += `<option value="${st}"${sel}>${st}</option>`;
    });
    html += `</select></label>`;
    html += `</div>`;

    // ── Enhancement & Limits ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.enhancement_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.enhancement_desc')}</div>`;

    // Prompt enhancement toggle
    const enhanceOn = cfg.prompt_enhancement === true;
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.image_generation.prompt_enhancement_label')}</span>
        <div class="toggle ${enhanceOn ? 'on' : ''}" data-path="image_generation.prompt_enhancement" onclick="toggleBool(this)"></div>
    </div>`;

    // Max monthly
    const curMax = cfg.max_monthly || 0;
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.image_generation.max_monthly_label')} <small style="color:var(--text-tertiary);">(${t('config.image_generation.max_monthly_hint')})</small></span>
        <input type="number" class="cfg-input" data-path="image_generation.max_monthly" value="${curMax}" min="0"
            placeholder="0"
            style="width:100%;margin-top:0.2rem;">
    </label>`;
    html += `</div>`;

    // ── Test Generation ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.image_generation.test_title')}</div>
        <div class="field-group-desc">${t('config.image_generation.test_desc')}</div>
        <div style="display:flex;align-items:center;gap:0.8rem;">
            <button class="btn-save" id="imggen-test-btn"
                onclick="imggenTestConnection()"
                style="padding:0.45rem 1rem;font-size:0.82rem;">
                🧪 ${t('config.image_generation.test_btn')}
            </button>
            <span id="imggen-test-status" style="font-size:0.8rem;"></span>
        </div>
        <div id="imggen-test-preview" style="margin-top:0.6rem;"></div>
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
    previewEl.innerHTML = '';
    try {
        const resp = await fetch('/api/image-generation/test');
        const data = await resp.json();
        if (data.status === 'ok') {
            statusEl.style.color = 'var(--success)';
            statusEl.textContent = '✓ ' + (data.message || t('config.image_generation.test_success'));
            if (data.web_path) {
                previewEl.innerHTML = `<img src="${escapeAttr(data.web_path)}" style="max-width:256px;max-height:256px;border-radius:8px;border:1px solid var(--border);">`;
            }
        } else {
            statusEl.style.color = 'var(--danger)';
            statusEl.textContent = '✗ ' + (data.message || t('config.image_generation.test_failed'));
        }
    } catch (e) {
        statusEl.style.color = 'var(--danger)';
        statusEl.textContent = '✗ ' + e.message;
    } finally {
        btn.disabled = false;
        btn.textContent = '🧪 ' + t('config.image_generation.test_btn');
    }
}
