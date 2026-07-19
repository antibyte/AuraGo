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

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.video_gen.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="video_generation.enabled" onclick="toggleBool(this);setNestedValue(configData,'video_generation.enabled',this.classList.contains('on'));renderVideoGenerationSection(null)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>🎬</span>
            <div>
                <strong>${t('config.video_gen.disabled_notice')}</strong><br>
                <small>${t('config.video_gen.disabled_desc')}</small>
            </div>
        </div>`;
        html += `</div>`;
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.video_gen.provider_title')}</div>
        <div class="field-group-desc">${t('config.video_gen.provider_desc')}</div>`;

    const curProvider = cfg.provider || '';
    const isAgnes = videoGenerationProviderType(curProvider) === 'agnes';
    const resolutionOptions = isAgnes ? ['480p','720p','768P','1080p'] : ['768P','1080P','720p','4k'];
    const aspectRatioOptions = isAgnes ? ['16:9','9:16','1:1','4:3','3:4'] : ['16:9','9:16','1:1'];
    const currentResolution = String(cfg.default_resolution || '768P').toLowerCase();
    const currentAspectRatio = String(cfg.default_aspect_ratio || '16:9');
    html += `<div class="field-grid two-cols">`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.video_gen.provider_label')}</div>
        <select class="field-select" data-path="video_generation.provider" onchange="videoGenerationProviderChanged(this)">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.video_gen.select_provider')}</option>`;
    providersCache.forEach(p => {
        const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        const name = p.name || p.id;
        const badge = p.type ? (' [' + p.type + ']') : '';
        const model = p.model ? (' - ' + p.model) : '';
        html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
    });
    html += `</select></div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.video_gen.model_label')}</div>
        <div class="field-help">${t('config.video_gen.model_hint')}</div>
        <input type="text" class="field-input" data-path="video_generation.model" value="${escapeAttr(cfg.model || '')}"
            placeholder="MiniMax-Hailuo-2.3, veo-3.1-generate-preview...">
    </div></div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.video_gen.defaults_title')}</div>
        <div class="field-group-desc">${t('config.video_gen.defaults_desc')}</div>
        <div class="field-grid two-cols">
            <div class="field-group">
                <div class="field-label">${t('config.video_gen.duration_label')}</div>
                <input type="number" class="field-input" data-path="video_generation.default_duration_seconds" value="${cfg.default_duration_seconds || 6}" min="1" max="30">
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.video_gen.resolution_label')}</div>
                <select class="field-select" data-path="video_generation.default_resolution">
                    ${resolutionOptions.map(v => `<option value="${v}"${currentResolution === v.toLowerCase() ? ' selected' : ''}>${v}</option>`).join('')}
                </select>
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.video_gen.aspect_ratio_label')}</div>
                <select class="field-select" data-path="video_generation.default_aspect_ratio">
                    ${aspectRatioOptions.map(v => `<option value="${v}"${currentAspectRatio === v ? ' selected' : ''}>${v}</option>`).join('')}
                </select>
            </div>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.video_gen.limits_title')}</div>
        <div class="field-group-desc">${t('config.video_gen.limits_desc')}</div>
        <div class="field-grid two-cols">
            <div class="field-group">
                <div class="field-label">${t('config.video_gen.poll_interval_label')}</div>
                <input type="number" class="field-input" data-path="video_generation.poll_interval_seconds" value="${cfg.poll_interval_seconds || 10}" min="1" max="120">
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.video_gen.timeout_label')}</div>
                <input type="number" class="field-input" data-path="video_generation.timeout_seconds" value="${cfg.timeout_seconds || 600}" min="30" max="3600">
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.video_gen.max_daily_label')}</div>
                <input type="number" class="field-input" data-path="video_generation.max_daily" value="${cfg.max_daily || 0}" min="0" placeholder="0">
            </div>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.video_gen.test_title')}</div>
        <div class="field-group-desc">${t('config.video_gen.test_desc')}</div>
        <div class="cfg-actions-row">
            <button class="btn-save adg-test-btn" id="video-test-btn" onclick="videoTestConnection()">
                🔌 ${t('config.video_gen.test_btn')}
            </button>
            <span id="video-test-result" class="adg-test-result"></span>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function videoTestConnection() {
    const btn = document.getElementById('video-test-btn');
    const result = document.getElementById('video-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.className = 'adg-test-result';
        result.textContent = t('config.video_gen.testing');
    }

    fetch('/api/video-generation/test')
    .then(r => r.json())
    .then(res => {
        if (!result) return;
        if (res.status === 'ok') {
            result.className = 'adg-test-result is-success';
            result.textContent = res.message || t('config.video_gen.test_success');
        } else {
            result.className = 'adg-test-result is-danger';
            result.textContent = res.message || t('config.video_gen.test_failed');
        }
    })
    .catch(err => {
        if (result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = err.message;
        }
    })
    .finally(() => { if (btn) btn.disabled = false; });
}
