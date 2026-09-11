// cfg/music_generation.js — Music Generation config section

let _musicSection = null;
let _musicLocalTimer = null;

function renderMusicGenerationSection(section) {
    clearTimeout(_musicLocalTimer);
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
    const local = curProvider === 'aurago-acestep-local';
    html += `<div class="field-grid two-cols">`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.music_gen.provider_label')}</div>
        <select class="field-select" data-path="music_generation.provider" onchange="setNestedValue(configData,'music_generation.provider',this.value);renderMusicGenerationSection(null)">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.music_gen.select_provider')}</option>
            <option value="aurago-acestep-local"${local ? ' selected' : ''}>${t('config.music_gen.local_provider')}</option>`;
    providersCache.forEach(p => {
        const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        const name = p.name || p.id;
        const badge = p.type ? (' [' + p.type + ']') : '';
        const model = p.model ? (' — ' + p.model) : '';
        html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
    });
    html += `</select></div>`;

    const curModel = cfg.model || '';
    if (!local) html += `<div class="field-group">
        <div class="field-label">${t('config.music_gen.model_label')}</div>
        <div class="field-help">${t('config.music_gen.model_hint')}</div>
        <input type="text" class="field-input" data-path="music_generation.model" value="${escapeAttr(curModel)}"
            placeholder="music-2.5+, lyria-3-clip-preview...">
    </div>`;
    html += `</div>`;
    html += `</div>`;

    if (local) {
        const options = cfg.local || {};
        html += `<div class="field-group"><div class="field-group-title">${t('config.music_gen.local_provider')}</div>
            <p class="field-help">${t('config.music_gen.local_help')}</p><div class="field-grid two-cols">`;
        html += `<label>${t('config.music_gen.backend')}<select class="field-select" data-path="music_generation.local.backend">`;
        ['auto', 'cuda', 'rocm', 'xpu', 'cpu'].forEach(value => {
            html += `<option value="${value}"${(options.backend || 'auto') === value ? ' selected' : ''}>${value === 'auto' ? t('config.music_gen.automatic') : value.toUpperCase()}</option>`;
        });
        html += `</select></label><label>${t('config.music_gen.device')}<input class="field-input" list="music-local-devices" data-path="music_generation.local.device" value="${escapeAttr(options.device || 'auto')}"><datalist id="music-local-devices"><option value="auto"></option></datalist></label>
            <label>${t('config.music_gen.reserve')}<input type="number" class="field-input" data-path="music_generation.local.vram_reserve_gb" min="0" max="256" step="0.25" value="${escapeAttr(options.vram_reserve_gb ?? 1)}"></label>
            <label>${t('config.music_gen.timeout')}<input type="number" class="field-input" data-path="music_generation.local.timeout_seconds" min="30" max="1800" value="${escapeAttr(options.timeout_seconds || 1800)}"></label></div>
            <p id="music-local-status" role="status" aria-live="polite"></p><progress id="music-local-progress" hidden></progress>
            <div class="cfg-actions-row">${['start','stop','recheck'].map(action => `<button type="button" class="btn-secondary" onclick="musicLocalAction('${action}')">${t('config.music_gen.' + action)}</button>`).join('')}</div></div>`;
    }

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
    if (local) musicLocalRefresh();
}

async function musicLocalRefresh() {
    clearTimeout(_musicLocalTimer);
    const target = document.getElementById('music-local-status');
    if (!target) return;
    try {
        const response = await fetch('/api/music-generation/local/status');
        const status = await response.json();
        if (!response.ok) throw new Error(status.message || String(response.status));
        if (!target.isConnected) return;
        const profile = status.profile;
        const label = t('config.music_gen.state_' + status.state);
        const parts = [label];
        if (profile) parts.push(profile.device.name, profile.model, profile.lm_model || t('config.music_gen.no_lm'), `${profile.device.free_gb.toFixed(1)} GiB`, `${profile.max_duration} s`, profile.quantization || 'FP', profile.offload ? 'CPU offload' : '');
        if (status.error_code) parts.push(status.error_code === 'acestep_no_compatible_gpu' ? t('config.music_gen.no_gpu') : status.error_code);
        if (!status.release_ready) parts.push(t('config.music_gen.release_missing'));
        target.textContent = parts.filter(Boolean).join(' · ');
        const progress = document.getElementById('music-local-progress');
        progress.hidden = !status.total_bytes || status.ready;
        progress.max = status.total_bytes || 1; progress.value = status.downloaded_bytes || 0;
        document.getElementById('music-local-devices').innerHTML = '<option value="auto"></option>' + (status.devices || []).map(device => `<option value="${escapeAttr(device.id)}">${escapeAttr(device.name)}</option>`).join('');
    } catch (error) { if (target.isConnected) target.textContent = error.message; }
    if (target.isConnected) _musicLocalTimer = setTimeout(musicLocalRefresh, 3000);
}

async function musicLocalAction(action) {
    const target = document.getElementById('music-local-status');
    if (isDirty) { if (target) target.textContent = t('config.music_gen.save_first'); return; }
    try {
        const response = await fetch('/api/music-generation/local/action', { method: 'POST', headers: {'Content-Type':'application/json'}, body:JSON.stringify({action}) });
        const result = await response.json();
        if (!response.ok) throw new Error(result.message || String(response.status));
        musicLocalRefresh();
    } catch (error) { if (target) target.textContent = error.message; }
}

document.addEventListener('aurago:config-saved', () => { if (document.getElementById('music-local-status')) musicLocalRefresh(); });

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

