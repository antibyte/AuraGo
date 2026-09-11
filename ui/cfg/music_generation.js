// cfg/music_generation.js — Music Generation config section

let _musicSection = null;
let _musicLocalTimer = null;
let _musicLocalRequest = 0;
let _musicLocalStatus = null;
let _musicLocalAction = null;
let _musicLocalSending = false;
let _musicLocalFeedback = {message: '', state: ''};

function renderMusicGenerationSection(section) {
    clearTimeout(_musicLocalTimer);
    ++_musicLocalRequest;
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
        ['auto', 'cuda', 'rocm', 'xpu', 'vulkan', 'cpu'].forEach(value => {
            html += `<option value="${value}"${(options.backend || 'auto') === value ? ' selected' : ''}>${value === 'auto' ? t('config.music_gen.automatic') : value.toUpperCase()}</option>`;
        });
        html += `</select><span class="field-help">${t('config.music_gen.vulkan_help')}</span></label><label>${t('config.music_gen.device')}<input class="field-input" list="music-local-devices" data-path="music_generation.local.device" value="${escapeAttr(options.device || 'auto')}"><datalist id="music-local-devices"><option value="auto"></option></datalist></label>
            <label>${t('config.music_gen.reserve')}<input type="number" class="field-input" data-path="music_generation.local.vram_reserve_gb" min="0" max="256" step="0.25" value="${escapeAttr(options.vram_reserve_gb ?? 1)}"></label>
            <label>${t('config.music_gen.timeout')}<input type="number" class="field-input" data-path="music_generation.local.timeout_seconds" min="30" max="1800" value="${escapeAttr(options.timeout_seconds || 1800)}"></label></div>
            <p id="music-local-status" role="status" aria-live="polite">${t('config.music_gen.testing')}</p><progress id="music-local-progress" aria-labelledby="music-local-status" hidden></progress>
            <div id="music-local-actions" class="cfg-actions-row">${['start','stop','recheck'].map(action => `<button type="button" class="btn-secondary" data-music-local-action="${action}">${t('config.music_gen.' + action)}</button>`).join('')}</div>
            <p id="music-local-feedback" class="adg-test-result" role="status" aria-live="polite" aria-atomic="true" hidden></p></div>`;
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
    if (local) {
        document.querySelectorAll('[data-music-local-action]').forEach(button => button.addEventListener('click', () => musicLocalAction(button.dataset.musicLocalAction)));
        musicLocalRenderFeedback();
        musicLocalRefresh();
    }
}

function musicLocalRenderFeedback() {
    const feedback = document.getElementById('music-local-feedback');
    if (!feedback) return;
    feedback.textContent = _musicLocalFeedback.message;
    feedback.className = 'adg-test-result ' + _musicLocalFeedback.state;
    feedback.hidden = !feedback.textContent;
    const working = _musicLocalSending || !!_musicLocalAction || _musicLocalStatus?.pending;
    document.getElementById('music-local-actions').setAttribute('aria-busy', String(!!working));
    document.querySelectorAll('[data-music-local-action]').forEach(button => {
        const action = button.dataset.musicLocalAction;
        button.disabled = _musicLocalSending || !!working && (action !== 'stop' || _musicLocalAction === 'stop');
        button.textContent = t('config.music_gen.' + (action === _musicLocalAction ? 'action_' + action : action));
    });
}

function musicLocalError(code) {
    if (code === 'acestep_no_compatible_gpu') return t('config.music_gen.no_gpu');
    if (code === 'acestep_release_not_published') return t('config.music_gen.release_missing');
    return code || t('config.music_gen.test_failed');
}

function musicLocalRenderStatus(status) {
    _musicLocalStatus = status;
    const target = document.getElementById('music-local-status');
    if (!target) return;
    const profile = status.profile;
    const parts = [t('config.music_gen.state_' + status.state)];
    if (status.pending) parts.push(t('config.music_gen.action_pending'));
    if (status.state === 'downloading' && status.total_bytes) parts.push(`${Math.min(100, Math.round((status.downloaded_bytes || 0) / status.total_bytes * 100))}%`);
    if (profile) parts.push(profile.device.backend?.toUpperCase(), profile.device.name, profile.model, profile.lm_model || t('config.music_gen.no_lm'), `${profile.device.free_gb.toFixed(1)} GiB`, `${profile.max_duration} s`, profile.quantization || 'FP', profile.offload ? 'CPU offload' : '');
    if (status.error_code) parts.push(musicLocalError(status.error_code));
    if (!status.release_ready && status.error_code !== 'acestep_release_not_published') parts.push(t('config.music_gen.release_missing'));
    target.textContent = parts.filter(Boolean).join(' · ');
    if (_musicLocalAction && !_musicLocalSending && !status.pending) {
        if (status.error_code || status.state === 'error') {
            _musicLocalFeedback = {message: musicLocalError(status.error_code), state: 'is-danger'};
            _musicLocalAction = null;
        } else if (_musicLocalAction === 'stop' ? ['stopped', 'disabled'].includes(status.state) : status.ready) {
            _musicLocalFeedback = {message: t('config.music_gen.action_' + _musicLocalAction + '_done'), state: 'is-success'};
            _musicLocalAction = null;
        }
    }
    const progress = document.getElementById('music-local-progress');
    progress.hidden = !status.pending && !['probing', 'starting', 'downloading', 'loading', 'testing'].includes(status.state);
    if (status.state === 'downloading' && status.total_bytes) {
        progress.max = status.total_bytes; progress.value = status.downloaded_bytes || 0;
    } else { progress.removeAttribute('value'); }
    document.getElementById('music-local-devices').innerHTML = '<option value="auto"></option>' + (status.devices || []).map(device => `<option value="${escapeAttr(device.id)}">${escapeAttr(device.name)}</option>`).join('');
    musicLocalRenderFeedback();
}

async function musicLocalRefresh() {
    clearTimeout(_musicLocalTimer);
    const request = ++_musicLocalRequest;
    const target = document.getElementById('music-local-status');
    if (!target) return;
    try {
        const response = await fetch('/api/music-generation/local/status', {signal: AbortSignal.timeout(15000)});
        const status = await response.json();
        if (!response.ok) throw new Error(status.message || String(response.status));
        if (request !== _musicLocalRequest || !target.isConnected) return;
        musicLocalRenderStatus(status);
    } catch (error) {
        if (request === _musicLocalRequest && target.isConnected) target.textContent = t('config.music_gen.status_unavailable');
    }
    if (request === _musicLocalRequest && target.isConnected) _musicLocalTimer = setTimeout(musicLocalRefresh, 2000);
}

async function musicLocalAction(action) {
    if (!['start', 'stop', 'recheck'].includes(action) || _musicLocalSending || _musicLocalAction && (action !== 'stop' || _musicLocalAction === 'stop')) return;
    if (isDirty) {
        _musicLocalFeedback = {message: t('config.music_gen.save_first'), state: 'is-danger'};
        musicLocalRenderFeedback(); return;
    }
    clearTimeout(_musicLocalTimer);
    ++_musicLocalRequest;
    _musicLocalAction = action;
    _musicLocalSending = true;
    _musicLocalFeedback = {message: t('config.music_gen.action_' + action), state: ''};
    musicLocalRenderFeedback();
    try {
        const response = await fetch('/api/music-generation/local/action', { method: 'POST', headers: {'Content-Type':'application/json'}, body:JSON.stringify({action}), signal: AbortSignal.timeout(15000) });
        const result = await response.json();
        if (!response.ok) throw new Error(result.message || result.error || String(response.status));
        _musicLocalSending = false;
        musicLocalRenderStatus(result);
    } catch (error) {
        _musicLocalAction = null;
        _musicLocalFeedback = {message: t('config.music_gen.action_failed') + ' ' + musicLocalError(error.message), state: 'is-danger'};
    } finally {
        _musicLocalSending = false;
        musicLocalRenderFeedback();
        musicLocalRefresh();
    }
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

