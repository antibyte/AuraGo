// cfg/local_llm.js — AuraGo-Qwen managed local test/fallback model

let _localLLMSection = null;
let _localLLMStatus = null;
let _localLLMPollTimer = null;
let _localLLMInstallPending = false;

function localLLMEnsureData() {
    if (!configData.local_llm) configData.local_llm = {};
    const data = configData.local_llm;
    if (!data.backend) data.backend = 'auto';
    if (!data.model_variant) data.model_variant = 'q4_k_m';
    if (!data.mtp) data.mtp = 'off';
    if (!data.context_size || Number(data.context_size) === 2048 || Number(data.context_size) === 8192) {
        data.context_size = 16384;
    }
    if (!data.idle_timeout_minutes) data.idle_timeout_minutes = 15;
    if (!data.listen_port) data.listen_port = 18081;
    return data;
}

function renderLocalLLMSection(section) {
    if (section) _localLLMSection = section;
    section = section || _localLLMSection;
    const data = localLLMEnsureData();
    const status = _localLLMStatus || {};
    const providers = (configData.providers || []).filter(provider => provider && provider.id && provider.id !== 'aurago-qwen-local');
    const selectedRegular = localLLMRegularProvider(status.role, providers);

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-warning"><strong>' + t('config.local_llm.purpose_title') + '</strong><br>' + t('config.local_llm.purpose') + '</div>';
    html += '<div class="cfg-note-banner cfg-note-banner-info">' + t('config.local_llm.hardware') + '</div>';
    html += '<div class="cfg-note-banner">' + t('config.local_llm.quality') + '</div>';

    html += localLLMToggle('local_llm.enabled', data.enabled === true, 'config.local_llm.enabled');
    html += localLLMSelect('local_llm.backend', data.backend, 'config.local_llm.backend', [
        ['auto', t('config.local_llm.backend_auto')], ['cuda', 'CUDA'], ['sycl', 'SYCL / Intel Arc'],
        ['vulkan', 'Vulkan'], ['cpu', 'CPU (' + t('config.local_llm.experimental') + ')']
    ]);
    html += localLLMSelect('local_llm.model_variant', data.model_variant, 'config.local_llm.model_variant', [
        ['q4_k_m', 'Q4_K_M · 2.59 GiB'], ['q8_0', 'Q8_0 · 4.29 GiB']
    ]);
    html += localLLMSelect('local_llm.context_size', String(data.context_size), 'config.local_llm.context_size', [
        ['16384', '16K'], ['32768', '32K']
    ], 'number');
    html += localLLMSelect('local_llm.mtp', data.mtp, 'config.local_llm.mtp', [
        ['off', t('config.local_llm.mtp_off')], ['auto', t('config.local_llm.mtp_auto')],
        ['mtp2', 'MTP-2 (' + t('config.local_llm.experimental') + ')']
    ]);
    html += '<div class="field-help">' + t('config.local_llm.mtp_storage_note') + '</div>';
    html += localLLMNumber('local_llm.idle_timeout_minutes', data.idle_timeout_minutes, 'config.local_llm.idle_timeout', 1, 1440);
    if (typeof isDockerRuntime !== 'function' || !isDockerRuntime()) {
        html += localLLMNumber('local_llm.listen_port', data.listen_port, 'config.local_llm.listen_port', 1, 65535);
    }
    html += '<div class="field-help">' + t('config.local_llm.storage_note') + '</div>';

    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.local_llm.compatibility') + '</div>';
    html += '<div id="local-llm-status" class="cfg-note-banner ' + localLLMStatusClass(status) + '">' + localLLMStatusText(status) + '</div>';
    html += localLLMPromptCacheStatus(status);
    if (status.acknowledgement_required) {
        html += '<div class="cfg-note-banner cfg-note-banner-warning">' + t('config.local_llm.ack_warning') + '</div>';
        html += '<button type="button" class="btn-secondary" data-local-llm-saved-action data-policy-disabled="false" onclick="localLLMAcknowledge()">' + t('config.local_llm.ack') + '</button> ';
    }
    html += '<div class="field-group">';
    html += localLLMActionButton('probe', 'config.local_llm.probe', false);
    html += localLLMActionButton('install', 'config.local_llm.install', !status.release_manifest_ready);
    html += localLLMActionButton('start', 'config.local_llm.start', false);
    html += localLLMActionButton('stop', 'config.local_llm.stop', false);
    html += localLLMActionButton('recreate', 'config.local_llm.recreate', false);
    html += localLLMActionButton('smoke_test', 'config.local_llm.smoke_test', false);
    html += localLLMActionButton('benchmark', 'config.local_llm.benchmark', false);
    html += '</div>';
    if (status.release_manifest_ready === false) {
        html += '<div class="cfg-note-banner cfg-note-banner-warning">' + t('config.local_llm.release_gate') + '</div>';
    }

    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.local_llm.routing') + '</div>';
    html += localLLMPlainSelect('local-llm-role', status.role || 'test_only', 'config.local_llm.role', [
        ['test_only', t('config.local_llm.role_test')], ['fallback', t('config.local_llm.role_fallback')],
        ['primary', t('config.local_llm.role_primary')]
    ], 'localLLMRenderRoutingPreview()');
    html += localLLMPlainSelect('local-llm-regular-provider', selectedRegular, 'config.local_llm.regular_provider',
        providers.map(provider => [provider.id, provider.name || provider.id]), 'localLLMRenderRoutingPreview()');
    html += '<div id="local-llm-routing-preview" class="cfg-note-banner"></div>';
    html += '<button type="button" class="btn-save" data-local-llm-saved-action data-policy-disabled="false" onclick="localLLMApplyRole()">' + t('config.local_llm.apply_role') + '</button>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    localLLMRenderRoutingPreview();
    localLLMUpdateSavedActionState();
    localLLMRefreshStatus();
}

function localLLMToggle(path, checked, labelKey) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="toggle-wrap">' +
        '<div class="toggle' + (checked ? ' on' : '') + '" data-path="' + escapeAttr(path) +
        '" onclick="toggleBool(this);setNestedValue(configData,\'' + path + '\',this.classList.contains(\'on\'))"></div>' +
        '<span class="toggle-label">' + (checked ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span></div></div>';
}

function localLLMSelect(path, value, labelKey, options, dataType) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><select class="field-select" data-path="' +
        escapeAttr(path) + '"' + (dataType ? ' data-type="' + escapeAttr(dataType) + '"' : '') + '>' +
        options.map(option => '<option value="' + escapeAttr(option[0]) + '"' +
        (String(value) === String(option[0]) ? ' selected' : '') + (option[2] ? ' disabled' : '') + '>' +
        escapeHtml(option[1]) + '</option>').join('') +
        '</select></div>';
}

function localLLMPlainSelect(id, value, labelKey, options, onChange) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><select class="field-select" id="' +
        escapeAttr(id) + '" onchange="' + onChange + '">' + options.map(option => '<option value="' + escapeAttr(option[0]) + '"' +
        (String(value) === String(option[0]) ? ' selected' : '') + '>' + escapeHtml(option[1]) + '</option>').join('') +
        '</select></div>';
}

function localLLMNumber(path, value, labelKey, min, max) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><input class="field-input" type="number" min="' +
        min + '" max="' + max + '" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(value) + '"></div>';
}

function localLLMActionButton(action, labelKey, disabled) {
    const primary = action === 'install' || action === 'smoke_test';
    return '<button type="button" class="' + (primary ? 'btn-save' : 'btn-secondary') +
        '" data-local-llm-saved-action data-policy-disabled="' + (disabled ? 'true' : 'false') +
        '" onclick="localLLMAction(\'' + action + '\')"' + (disabled ? ' disabled' : '') + '>' + t(labelKey) + '</button> ';
}

function localLLMUpdateSavedActionState() {
    document.querySelectorAll('[data-local-llm-saved-action]').forEach(button => {
        button.disabled = isDirty || button.dataset.policyDisabled === 'true';
    });
}

function localLLMStatusClass(status) {
    if (status.state === 'running') return 'cfg-note-banner-success';
    if (status.state === 'error') return 'cfg-note-banner-warning';
    return '';
}

function localLLMStatusText(status) {
    if (!status || !status.state) return t('config.local_llm.status_unknown');
    const state = localLLMLocalizedRuntimeValue('state', status.state);
    const compatibility = localLLMLocalizedRuntimeValue('compat', status.compatibility || 'unknown');
    let text = t('config.local_llm.status_summary')
        .replace('{state}', state)
        .replace('{compatibility}', compatibility)
        .replace('{backend}', status.backend || '—');
    if (status.error_code) text += ' · ' + t('config.local_llm.error_prefix') + ': ' + status.error_code;
    if (status.resolved_profile) text += ' · ' + t('config.local_llm.resolved_profile') + ': ' + status.resolved_profile;
    if (status.active_requests) text += ' · ' + t('config.local_llm.active_requests') + ': ' + status.active_requests;
    return text;
}

function localLLMPromptCacheStatus(status) {
    const cache = status && status.prompt_cache ? status.prompt_cache : {};
    const state = localLLMLocalizedRuntimeValue('cache_state', cache.state || 'disabled');
    const profile = status && status.performance_profile ? status.performance_profile : '—';
    const hitRate = Math.max(0, Math.min(1, Number(cache.hit_rate) || 0));
    let text = '<strong>' + escapeHtml(t('config.local_llm.prompt_cache')) + '</strong><br>' +
        escapeHtml(t('config.local_llm.performance_profile')) + ': ' + escapeHtml(profile) + ' · ' +
        escapeHtml(t('config.local_llm.cache_state')) + ': ' + escapeHtml(state) + ' · ' +
        escapeHtml(t('config.local_llm.cache_hit_rate')) + ': ' + escapeHtml((hitRate * 100).toFixed(1) + ' %');
    text += '<br>' +
        escapeHtml(t('config.local_llm.cache_qualification')) + ': ' +
        escapeHtml(t(cache.qualified ? 'common.yes' : 'common.no')) + ' · ' +
        escapeHtml(t('config.local_llm.cache_decision_persistence')) + ': ' +
        escapeHtml(t(cache.decision_persisted ? 'common.yes' : 'common.no'));
    if (Number(cache.cache_ram_mib) > 0) {
        text += ' · RAM: ' + escapeHtml(String(cache.cache_ram_mib) + ' MiB');
    }
    if (Number(cache.cold_ttft_ms) > 0 || Number(cache.warm_ttft_ms) > 0) {
        text += '<br>' + escapeHtml(t('config.local_llm.cache_ttft')
            .replace('{cold}', localLLMFormatMilliseconds(cache.cold_ttft_ms))
            .replace('{warm}', localLLMFormatMilliseconds(cache.warm_ttft_ms)));
    }
    text += '<br><span class="field-help">' + escapeHtml(t('config.local_llm.prompt_cache_first_start')) + '</span>';
    if (cache.error_code) {
        text += '<br>' + escapeHtml(t('config.local_llm.error_prefix') + ': ' + localLLMCacheErrorText(cache.error_code));
    }
    return '<div class="cfg-note-banner">' + text + '</div>';
}

function localLLMCacheErrorText(errorCode) {
    const code = String(errorCode || '');
    let key = 'config.local_llm.cache_error_runtime';
    if (code === 'prompt_cache_probe_tool_unavailable') key = 'config.local_llm.cache_error_probe_tool';
    else if (code === 'prompt_cache_semantic_mismatch') key = 'config.local_llm.cache_error_semantic';
    else if (code === 'prompt_cache_reuse_below_80_percent') key = 'config.local_llm.cache_error_reuse';
    else if (code === 'prompt_cache_ttft_gain_below_70_percent') key = 'config.local_llm.cache_error_ttft';
    else if (code === 'prompt_cache_qualification_timeout') key = 'config.local_llm.cache_error_timeout';
    else if (code === 'prompt_cache_decision_write_failed') key = 'config.local_llm.cache_error_persistence';
    const translated = t(key);
    return translated && translated !== key ? translated : code;
}

function localLLMFormatMilliseconds(value) {
    const numeric = Number(value);
    if (!Number.isFinite(numeric) || numeric <= 0) return '—';
    return numeric >= 1000 ? (numeric / 1000).toFixed(2) + ' s' : numeric.toFixed(0) + ' ms';
}

function localLLMLocalizedRuntimeValue(kind, value) {
    const normalized = String(value || 'unknown').toLowerCase().replace(/[^a-z0-9_]/g, '_');
    const key = 'config.local_llm.' + kind + '_' + normalized;
    const translated = t(key);
    return translated && translated !== key ? translated : String(value || 'unknown');
}

function localLLMSavedOnly() {
    const target = document.getElementById('local-llm-status');
    if (!isDirty) return true;
    if (target) {
        target.textContent = t('config.local_llm.save_first');
        target.className = 'cfg-note-banner cfg-note-banner-warning';
    }
    return false;
}

async function localLLMRefreshStatus() {
    try {
        const response = await fetch('/api/local-llm/status');
        const data = await response.json();
        if (!response.ok) throw new Error(data.message || ('HTTP ' + response.status));
        const changed = !_localLLMStatus || JSON.stringify(_localLLMStatus) !== JSON.stringify(data);
        _localLLMStatus = data;
        if (_localLLMInstallPending &&
            (data.state === 'running' && Number(data.progress) >= 1 || data.state === 'error')) {
            _localLLMInstallPending = false;
        }
        if (changed && document.getElementById('local-llm-status')) renderLocalLLMSection(null);
        localLLMSchedulePolling();
    } catch (error) {
        const target = document.getElementById('local-llm-status');
        if (target) target.textContent = t('config.local_llm.error_prefix') + ': ' + error.message;
    }
}

function localLLMSchedulePolling() {
    if (_localLLMPollTimer) {
        clearTimeout(_localLLMPollTimer);
        _localLLMPollTimer = null;
    }
    if (!document.getElementById('local-llm-status') || !_localLLMStatus) return;
    const activeStates = ['downloading', 'pulling', 'starting', 'testing', 'benchmarking'];
    if (_localLLMInstallPending || _localLLMStatus.operation_in_progress || activeStates.includes(_localLLMStatus.state)) {
        _localLLMPollTimer = setTimeout(localLLMRefreshStatus, 2000);
    }
}

async function localLLMAction(action) {
    if (!localLLMSavedOnly()) return;
    const path = action === 'probe' ? '/api/local-llm/probe' : (action === 'install' ? '/api/local-llm/install' : '/api/local-llm/action');
    const body = action === 'probe' || action === 'install' ? {} : { action };
    const target = document.getElementById('local-llm-status');
    if (target) target.textContent = t('config.local_llm.working');
    if (action === 'install') _localLLMInstallPending = true;
    if (_localLLMPollTimer) clearTimeout(_localLLMPollTimer);
    _localLLMPollTimer = setTimeout(localLLMRefreshStatus, 500);
    try {
        const response = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        const data = await response.json();
        if (!response.ok) throw new Error(data.message || data.error || ('HTTP ' + response.status));
        await localLLMRefreshStatus();
    } catch (error) {
        if (action === 'install') _localLLMInstallPending = false;
        if (_localLLMPollTimer) {
            clearTimeout(_localLLMPollTimer);
            _localLLMPollTimer = null;
        }
        if (target) {
            target.textContent = t('config.local_llm.error_prefix') + ': ' + error.message;
            target.className = 'cfg-note-banner cfg-note-banner-warning';
        }
    }
}

async function localLLMAcknowledge() {
    if (!localLLMSavedOnly() || !_localLLMStatus) return;
    const response = await fetch('/api/local-llm/acknowledgement', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ fingerprint: _localLLMStatus.hardware_fingerprint })
    });
    if (response.ok) await localLLMRefreshStatus();
}

function localLLMRegularProvider(role, providers) {
    const main = configData.llm && configData.llm.provider;
    const fallback = configData.fallback_llm && configData.fallback_llm.provider;
    const candidate = role === 'primary' ? fallback : main;
    if (providers.some(provider => provider.id === candidate)) return candidate;
    return providers.length ? providers[0].id : '';
}

function localLLMRenderRoutingPreview() {
    const role = document.getElementById('local-llm-role');
    const provider = document.getElementById('local-llm-regular-provider');
    const target = document.getElementById('local-llm-routing-preview');
    if (!role || !provider || !target) return;
    target.textContent = t('config.local_llm.routing_preview')
        .replace('{role}', t('config.local_llm.role_' + (role.value === 'test_only' ? 'test' : role.value)))
        .replace('{provider}', provider.options[provider.selectedIndex] ? provider.options[provider.selectedIndex].text : '—');
    const currentFallback = configData.fallback_llm || {};
    const replacesFallback = currentFallback.enabled && currentFallback.provider &&
        currentFallback.provider !== 'aurago-qwen-local' &&
        (role.value === 'fallback' || role.value === 'primary' && currentFallback.provider !== provider.value);
    if (replacesFallback) {
        target.textContent += ' ' + t('config.local_llm.replaces_fallback');
        target.className = 'cfg-note-banner cfg-note-banner-warning';
    } else {
        target.className = 'cfg-note-banner';
    }
}

async function localLLMApplyRole() {
    if (!localLLMSavedOnly() || !_localLLMStatus) return;
    const role = document.getElementById('local-llm-role').value;
    const regularProvider = document.getElementById('local-llm-regular-provider').value;
    const target = document.getElementById('local-llm-status');
    try {
        const response = await fetch('/api/local-llm/role', {
            method: 'POST', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ role, regular_provider: regularProvider, config_revision: _localLLMStatus.config_revision })
        });
        const data = await response.json();
        if (!response.ok) throw new Error(data.message || data.error || ('HTTP ' + response.status));
        window.location.reload();
    } catch (error) {
        target.textContent = t('config.local_llm.error_prefix') + ': ' + error.message;
        target.className = 'cfg-note-banner cfg-note-banner-warning';
    }
}

document.addEventListener('aurago:config-saved', () => {
    localLLMUpdateSavedActionState();
    localLLMRefreshStatus();
});

document.addEventListener('input', () => window.setTimeout(localLLMUpdateSavedActionState, 0));
document.addEventListener('change', () => window.setTimeout(localLLMUpdateSavedActionState, 0));

window.addEventListener('cfg:section-leave', () => {
    if (_localLLMPollTimer) {
        clearTimeout(_localLLMPollTimer);
        _localLLMPollTimer = null;
    }
});

window.addEventListener('beforeunload', () => {
    if (_localLLMPollTimer) clearTimeout(_localLLMPollTimer);
});
