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
    const form = window.AuraConfigForm;
    const key = name => t('config.music_gen.' + name);

    // The switch re-renders the section: the provider groups only exist while the feature is on.
    const enableToggle = form.toggle({ label: key('enabled_label'), help: key('enabled_help'), path: 'music_generation.enabled', value: enabled, onclick: "toggleBool(this);setNestedValue(configData,'music_generation.enabled',this.classList.contains('on'));renderMusicGenerationSection(null)" });
    const groups = [{ titleKey: 'config.refresh.operation', fields: [enableToggle] }];
    let after = '';
    const curProvider = cfg.provider || '';
    const local = curProvider === 'aurago-acestep-local';

    if (!enabled) {
        after = form.emptyState({ title: key('disabled_notice'), desc: key('disabled_desc') });
    } else {
        const providers = configGeneralProviders();
        const providerOptions = [
            { value: '', label: key('select_provider') },
            { value: 'aurago-acestep-local', label: key('local_provider') }
        ].concat(providers.map(p => ({
            value: p.id,
            label: (p.name || p.id) + (p.type ? ' [' + p.type + ']' : '') + (p.model ? ' — ' + p.model : '')
        })));
        const providerSelect = form.select({
            label: key('provider_label'), help: key('provider_help'), path: 'music_generation.provider', value: curProvider, options: providerOptions,
            onchange: "setNestedValue(configData,'music_generation.provider',this.value);renderMusicGenerationSection(null)",
            extraOptionsHTML: configPreservedProviderOptionHTML(curProvider, providers)
        });
        const providerFields = [providerSelect];
        if (!local) providerFields.push(form.field({ label: key('model_label'), help: key('model_hint'), path: 'music_generation.model', value: cfg.model || '', placeholder: 'music-2.5+, lyria-3-clip-preview...' }));
        groups.push({ title: key('provider_label'), desc: key('provider_desc'), fields: providerFields });

        if (local) {
            const options = cfg.local || {};
            const backends = ['auto', 'cuda', 'rocm', 'xpu', 'vulkan', 'cpu'].map(value =>
                `<option value="${value}"${(options.backend || 'auto') === value ? ' selected' : ''}>${value === 'auto' ? key('automatic') : value.toUpperCase()}</option>`).join('');
            const actionButtons = ['start', 'stop', 'recheck'].map(action => `<button type="button" class="btn-secondary" data-music-local-action="${action}">${key(action)}</button>`).join('');
            groups.push({ title: key('local_provider'), desc: key('local_help'), content: `<div class="field-grid two-cols">`
                + `<label>${key('backend')}<select class="field-select" data-path="music_generation.local.backend">${backends}</select><span class="field-help">${key('vulkan_help')}</span></label>`
                + `<label>${key('device')}<input class="field-input" list="music-local-devices" data-path="music_generation.local.device" value="${escapeAttr(options.device || 'auto')}"><datalist id="music-local-devices"><option value="auto"></option></datalist><span class="field-help">${key('device_help')}</span></label>`
                + `<label>${key('reserve')}<input type="number" class="field-input" data-path="music_generation.local.vram_reserve_gb" min="0" max="256" step="0.25" value="${escapeAttr(options.vram_reserve_gb ?? 1)}"><span class="field-help">${key('reserve_help')}</span></label>`
                + `<label>${key('timeout')}<input type="number" class="field-input" data-path="music_generation.local.timeout_seconds" min="30" max="1800" value="${escapeAttr(options.timeout_seconds || 1800)}"><span class="field-help">${key('timeout_help')}</span></label></div>`
                + `<p id="music-local-status" role="status" aria-live="polite">${key('testing')}</p><progress id="music-local-progress" aria-labelledby="music-local-status" hidden></progress>`
                + `<div id="music-local-actions" class="cfg-actions-row">${actionButtons}</div>`
                + `<p id="music-local-feedback" class="cfg-action-result" role="status" aria-live="polite" aria-atomic="true" hidden></p>` });
        }

        groups.push({ title: key('limits_title'), desc: key('limits_desc'), fields: [
            form.number({ label: key('max_daily_label'), help: key('max_daily_help'), path: 'music_generation.max_daily', value: cfg.max_daily || 0, min: 0 })
        ] });
        groups.push({ title: key('test_title'), desc: key('test_desc'), content: form.actions([
            { html: '<button class="btn-save cfg-test-btn" id="music-test-btn" onclick="musicTestConnection()" data-config-action-draft="true">' + cfgIcon('plug') + '<span>' + escapeHtml(key('test_btn')) + '</span></button>' },
            { html: cfgActionResult.html('music-test-result') }
        ]) });
    }

    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc, groups, afterHTML: after });
    attachChangeListeners();
    if (local && enabled) {
        document.querySelectorAll('[data-music-local-action]').forEach(button => button.addEventListener('click', () => musicLocalAction(button.dataset.musicLocalAction)));
        musicLocalRenderFeedback();
        musicLocalRefresh();
    }
}

function musicLocalRenderFeedback() {
    const feedback = document.getElementById('music-local-feedback');
    if (!feedback) return;
    // The result line shares the data-state contract of cfgActionResult.
    const feedbackState = _musicLocalFeedback.state === 'is-success' ? 'success' : (_musicLocalFeedback.state === 'is-danger' ? 'error' : 'pending');
    cfgActionResult.show(feedback, _musicLocalFeedback.message ? feedbackState : '', _musicLocalFeedback.message);
    feedback.hidden = !feedback.textContent;
    const working = _musicLocalSending || !!_musicLocalAction || _musicLocalStatus?.pending;
    document.getElementById('music-local-actions').setAttribute('aria-busy', String(!!working));
    document.querySelectorAll('[data-music-local-action]').forEach(button => {
        const action = button.dataset.musicLocalAction;
        button.disabled = _musicLocalSending || !!working && (action !== 'stop' || _musicLocalAction === 'stop');
        button.setAttribute('aria-busy', String(action === _musicLocalAction && !!working));
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

// Checks the provider selected in the form (unsaved included).
function musicTestConnection() {
    return cfgActionResult(document.getElementById('music-test-btn'), () => fetch('/api/music-generation/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['music_generation']) })
    }), {
        target: 'music-test-result',
        pending: t('config.music_gen.testing'),
        success: outcome => outcome.reason || t('config.music_gen.test_success'),
        failure: t('config.music_gen.test_failed')
    });
}
