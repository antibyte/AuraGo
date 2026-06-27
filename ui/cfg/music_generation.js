// cfg/music_generation.js — Music Generation config section

let _musicSection = null;

function renderMusicGenerationSection(section) {
    if (section) _musicSection = section; else section = _musicSection;
    const cfg = configData['music_generation'] || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
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
    html += `<div class="field-grid two-cols">`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.music_gen.provider_label')}</div>
        <select class="field-select" data-path="music_generation.provider">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.music_gen.select_provider')}</option>`;
    providersCache.forEach(p => {
        const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        const name = p.name || p.id;
        const badge = p.type ? (' [' + p.type + ']') : '';
        const model = p.model ? (' — ' + p.model) : '';
        html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
    });
    html += `</select></div>`;

    const curModel = cfg.model || '';
    html += `<div class="field-group">
        <div class="field-label">${t('config.music_gen.model_label')}</div>
        <div class="field-help">${t('config.music_gen.model_hint')}</div>
        <input type="text" class="field-input" data-path="music_generation.model" value="${escapeAttr(curModel)}"
            placeholder="music-2.5+, lyria-3-clip-preview...">
    </div></div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.music_gen.limits_title')}</div>
        <div class="field-group-desc">${t('config.music_gen.limits_desc')}</div>`;

    const curMaxDaily = cfg.max_daily || 0;
    html += `<div class="field-group">
        <div class="field-label">${t('config.music_gen.max_daily_label')}</div>
        <div class="field-help">${t('config.music_gen.max_daily_help')}</div>
        <input type="number" class="field-input" data-path="music_generation.max_daily" value="${curMaxDaily}" min="0" placeholder="0">
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.music_gen.test_title')}</div>
        <div class="field-group-desc">${t('config.music_gen.test_desc')}</div>
        <div class="cfg-actions-row">
            <button class="btn-save adg-test-btn" id="music-test-btn" onclick="musicTestConnection()">
                🔌 ${t('config.music_gen.test_btn')}
            </button>
            <span id="music-test-result" class="adg-test-result"></span>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function musicTestConnection() {
    const btn = document.getElementById('music-test-btn');
    const result = document.getElementById('music-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.className = 'adg-test-result';
        result.textContent = t('config.music_gen.testing');
    }

    fetch('/api/music-generation/test')
    .then(r => r.json())
    .then(res => {
        if (!result) return;
        if (res.status === 'ok') {
            result.className = 'adg-test-result is-success';
            result.textContent = res.message || t('config.music_gen.test_success');
        } else {
            result.className = 'adg-test-result is-danger';
            result.textContent = res.message || t('config.music_gen.test_failed');
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

