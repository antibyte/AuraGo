// cfg/music_generation.js — Music Generation config section

let _musicSection = null;

function renderMusicGenerationSection(section) {
    if (section) _musicSection = section; else section = _musicSection;
    const cfg = configData['music_generation'] || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.music_gen.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="music_generation.enabled" onclick="toggleBool(this);setNestedValue(configData,'music_generation.enabled',this.classList.contains('on'));renderMusicGenerationSection(null)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>🎵</span>
            <div>
                <strong>${t('config.music_gen.disabled_notice')}</strong><br>
                <small>${t('config.music_gen.disabled_desc')}</small>
            </div>
        </div>`;
        html += `</div>`;
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.music_gen.provider_label')}</div>
        <div class="field-group-desc">${t('config.music_gen.provider_desc')}</div>`;

    const curProvider = cfg.provider || '';
    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.music_gen.provider_label')}</span>
        <select class="cfg-input cfg-input-full" data-path="music_generation.provider"
            onchange="setNestedValue(configData,'music_generation.provider',this.value);setDirty(true)">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.music_gen.select_provider')}</option>`;
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
        <span class="ig-label-text">${t('config.music_gen.model_label')} <small class="ig-hint">(${t('config.music_gen.model_hint')})</small></span>
        <input type="text" class="cfg-input cfg-input-full" data-path="music_generation.model" value="${escapeAttr(curModel)}"
            placeholder="music-2.5+, lyria-3-clip-preview...">
    </label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.music_gen.limits_title')}</div>
        <div class="field-group-desc">${t('config.music_gen.limits_desc')}</div>`;

    const curMaxDaily = cfg.max_daily || 0;
    html += `<label class="ig-label">
        <span class="ig-label-text">${t('config.music_gen.max_daily_label')} <small class="ig-hint">(${t('config.music_gen.max_daily_help')})</small></span>
        <input type="number" class="cfg-input cfg-input-full" data-path="music_generation.max_daily" value="${curMaxDaily}" min="0"
            placeholder="0">
    </label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.music_gen.test_title')}</div>
        <div class="field-group-desc">${t('config.music_gen.test_desc')}</div>
        <div class="ig-flex-row">
            <button class="btn-save cfg-save-btn-sm" id="music-test-btn"
                onclick="musicTestConnection()">
                🔌 ${t('config.music_gen.test_btn')}
            </button>
            <span id="music-test-status" class="ig-test-status"></span>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function musicTestConnection() {
    const btn = document.getElementById('music-test-btn');
    const statusEl = document.getElementById('music-test-status');
    if (btn) btn.disabled = true;
    if (statusEl) { statusEl.textContent = '⏳ ' + t('config.music_gen.testing'); statusEl.className = 'ig-test-status'; }

    fetch('/api/music-generation/test')
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok') {
            if (statusEl) { statusEl.textContent = '✓ ' + (res.message || t('config.music_gen.test_success')); statusEl.classList.add('ig-status-success'); }
        } else {
            if (statusEl) { statusEl.textContent = '✗ ' + (res.message || t('config.music_gen.test_failed')); statusEl.classList.add('ig-status-error'); }
        }
    })
    .catch(err => {
        if (statusEl) { statusEl.textContent = '✗ ' + err.message; statusEl.classList.add('ig-status-error'); }
    })
    .finally(() => { if (btn) btn.disabled = false; });
}

