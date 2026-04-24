// cfg/video_generation.js - Video Generation config section

let _videoSection = null;

function renderVideoGenerationSection(section) {
    if (section) _videoSection = section; else section = _videoSection;
    const cfg = configData['video_generation'] || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
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
    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.video_gen.provider_label')}</span>
        <select class="cfg-input cfg-input-full" data-path="video_generation.provider"
            onchange="setNestedValue(configData,'video_generation.provider',this.value);setDirty(true)">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.video_gen.select_provider')}</option>`;
    providersCache.forEach(p => {
        const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        const name = p.name || p.id;
        const badge = p.type ? (' [' + p.type + ']') : '';
        const model = p.model ? (' - ' + p.model) : '';
        html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
    });
    html += `</select></label>`;

    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.video_gen.model_label')} <small class="ig-hint">(${t('config.video_gen.model_hint')})</small></span>
        <input type="text" class="cfg-input cfg-input-full" data-path="video_generation.model" value="${escapeAttr(cfg.model || '')}"
            placeholder="Hailuo-2.3-768P, veo-3.1-generate-preview...">
    </label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.video_gen.defaults_title')}</div>
        <div class="field-group-desc">${t('config.video_gen.defaults_desc')}</div>
        <div class="ig-grid-3">
            <label class="ig-label">
                <span class="ig-label-text">${t('config.video_gen.duration_label')}</span>
                <input type="number" class="cfg-input cfg-input-full" data-path="video_generation.default_duration_seconds" value="${cfg.default_duration_seconds || 6}" min="1" max="30">
            </label>
            <label class="ig-label">
                <span class="ig-label-text">${t('config.video_gen.resolution_label')}</span>
                <select class="cfg-input cfg-input-full" data-path="video_generation.default_resolution">
                    ${['768P','1080P','720p','4k'].map(v => `<option value="${v}"${String(cfg.default_resolution || '768P') === v ? ' selected' : ''}>${v}</option>`).join('')}
                </select>
            </label>
            <label class="ig-label">
                <span class="ig-label-text">${t('config.video_gen.aspect_ratio_label')}</span>
                <select class="cfg-input cfg-input-full" data-path="video_generation.default_aspect_ratio">
                    ${['16:9','9:16','1:1'].map(v => `<option value="${v}"${String(cfg.default_aspect_ratio || '16:9') === v ? ' selected' : ''}>${v}</option>`).join('')}
                </select>
            </label>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.video_gen.limits_title')}</div>
        <div class="field-group-desc">${t('config.video_gen.limits_desc')}</div>
        <div class="ig-grid-3">
            <label class="ig-label">
                <span class="ig-label-text">${t('config.video_gen.poll_interval_label')}</span>
                <input type="number" class="cfg-input cfg-input-full" data-path="video_generation.poll_interval_seconds" value="${cfg.poll_interval_seconds || 10}" min="1" max="120">
            </label>
            <label class="ig-label">
                <span class="ig-label-text">${t('config.video_gen.timeout_label')}</span>
                <input type="number" class="cfg-input cfg-input-full" data-path="video_generation.timeout_seconds" value="${cfg.timeout_seconds || 600}" min="30" max="3600">
            </label>
            <label class="ig-label">
                <span class="ig-label-text">${t('config.video_gen.max_daily_label')}</span>
                <input type="number" class="cfg-input cfg-input-full" data-path="video_generation.max_daily" value="${cfg.max_daily || 0}" min="0" placeholder="0">
            </label>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.video_gen.test_title')}</div>
        <div class="field-group-desc">${t('config.video_gen.test_desc')}</div>
        <div class="ig-flex-row">
            <button class="btn-save cfg-save-btn-sm" id="video-test-btn" onclick="videoTestConnection()">
                🔌 ${t('config.video_gen.test_btn')}
            </button>
            <span id="video-test-status" class="ig-test-status"></span>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function videoTestConnection() {
    const btn = document.getElementById('video-test-btn');
    const statusEl = document.getElementById('video-test-status');
    if (btn) btn.disabled = true;
    if (statusEl) { statusEl.textContent = '⏳ ' + t('config.video_gen.testing'); statusEl.className = 'ig-test-status'; }

    fetch('/api/video-generation/test')
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok') {
            if (statusEl) { statusEl.textContent = '✓ ' + (res.message || t('config.video_gen.test_success')); statusEl.classList.add('ig-status-success'); }
        } else {
            if (statusEl) { statusEl.textContent = '✗ ' + (res.message || t('config.video_gen.test_failed')); statusEl.classList.add('ig-status-error'); }
        }
    })
    .catch(err => {
        if (statusEl) { statusEl.textContent = '✗ ' + err.message; statusEl.classList.add('ig-status-error'); }
    })
    .finally(() => { if (btn) btn.disabled = false; });
}
