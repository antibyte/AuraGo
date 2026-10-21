// cfg/ollama.js — Ollama config section with managed container control
/* global configData, t, escapeAttr, toggleBool, renderFields, schema, helpTexts, lang */
'use strict';
let ollamaPollTimer = 0;
let ollamaPollController = null;
let ollamaPollGeneration = 0;
let ollamaExpectedOperation = '';
let ollamaExternalURL = '';
function ollamaStopPolling() { clearTimeout(ollamaPollTimer); ollamaPollController?.abort(); ollamaPollGeneration++; }
window.addEventListener('hashchange', ollamaStopPolling);
window.addEventListener('beforeunload', ollamaStopPolling);


async function renderOllamaSection(section) {
    ollamaStopPolling();
    ollamaExpectedOperation = '';
    ollamaExternalURL = (configData.ollama || {}).url || '';
    const data = configData['ollama'] || {};
    const miData = data.managed_instance || {};
    const managedEnabled = miData.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Basic Ollama settings ──
    html += _ollamaField('ollama.enabled', t('config.ollama.enabled_label'), t('config.ollama.enabled_help'),
        _ollamaToggle('ollama.enabled', data.enabled === true));

    html += _ollamaField('ollama.readonly', t('config.ollama.readonly_label'), t('config.ollama.readonly_help'),
        _ollamaToggle('ollama.readonly', data.readonly === true));

    html += _ollamaField('ollama.url', t('config.ollama.url_label'), t('config.ollama.url_help'),
        '<input class="field-input" type="text" data-path="ollama.url" value="' + escapeAttr(managedEnabled ? ollamaEffectiveURL(miData) : (data.url || '')) + '" placeholder="http://localhost:11434"' + (managedEnabled ? ' readonly' : '') + '>');

    html += '<div class="field-help">' + t('config.ollama.endpoint_help') + '</div>';
    html += '<div class="pw-u-inline-actions"><button id="ollama-test-btn" type="button" class="cfg-btn">' + t('config.ollama.test_connection') + '</button><span id="ollama-test-result" role="status" aria-live="polite"></span></div>';

    // ── Managed Instance section header ──
    html += '<div class="pw-u-subsection-title">🐳 ' + t('config.ollama.managed_title') + '</div>';

    // Status banner (only shown when managed is enabled)
    html += '<div id="ollama-managed-banner" class="pw-u-status-banner">' + t('config.ollama.managed_checking') + '</div>';

    html += _ollamaField('ollama.managed_instance.enabled', t('config.ollama.managed_enabled_label'), t('config.ollama.managed_enabled_help'),
        _ollamaToggle('ollama.managed_instance.enabled', managedEnabled, 'ollamaManagedToggled(this)'));

    html += _ollamaField('ollama.managed_instance.container_port', t('config.ollama.managed_port_label'), t('config.ollama.managed_port_help'),
        '<input class="field-input" type="number" data-path="ollama.managed_instance.container_port" value="' + (miData.container_port || 11434) + '" min="1" max="65535">');

    html += _ollamaField('ollama.managed_instance.use_host_gpu', t('config.ollama.managed_gpu_label'), t('config.ollama.managed_gpu_help'),
        _ollamaToggle('ollama.managed_instance.use_host_gpu', miData.use_host_gpu === true));

    // GPU backend dropdown
    const gpuVal = miData.gpu_backend || 'auto';
    const gpuOptions = ['auto', 'nvidia', 'amd', 'intel', 'vulkan'];
    let gpuSelect = '<select class="field-select" data-path="ollama.managed_instance.gpu_backend">';
    for (const opt of gpuOptions) {
        gpuSelect += '<option value="' + opt + '"' + (gpuVal === opt ? ' selected' : '') + '>' + opt + '</option>';
    }
    gpuSelect += '</select>';
    html += _ollamaField('ollama.managed_instance.gpu_backend', t('config.ollama.managed_gpu_backend_label'), t('config.ollama.managed_gpu_backend_help'), gpuSelect);

    html += _ollamaField('ollama.managed_instance.default_models', t('config.ollama.managed_models_label'), t('config.ollama.managed_models_help'),
        '<input class="field-input" type="text" data-type="array" data-path="ollama.managed_instance.default_models" value="' + escapeAttr((miData.default_models || []).join(',')) + '" placeholder="llama3,mistral">');

    html += _ollamaField('ollama.managed_instance.memory_limit', t('config.ollama.managed_memory_label'), t('config.ollama.managed_memory_help'),
        '<input class="field-input" type="text" data-path="ollama.managed_instance.memory_limit" value="' + escapeAttr(miData.memory_limit || '') + '" placeholder="8g">');

    html += _ollamaField('ollama.managed_instance.volume_path', t('config.ollama.managed_volume_label'), t('config.ollama.managed_volume_help'),
        '<input class="field-input" type="text" data-path="ollama.managed_instance.volume_path" value="' + escapeAttr(miData.volume_path || '') + '" placeholder="">');

    // ── Recreate button (shown when managed is enabled) ──
    html += '<div id="ollama-recreate-wrap" class="pw-u-mt-100' + (managedEnabled ? '' : ' pw-u-hidden') + '">';
    html += '<button class="btn-save pw-u-btn-md" id="ollama-recreate-btn" type="button">';
    html += '🔄 ' + t('config.ollama.managed_recreate_btn') + '</button>';
    html += '<span id="ollama-recreate-status" class="pw-u-inline-status"></span>';
    html += '</div>';

    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    if (typeof applyManagedDockerGuards === 'function') {
        applyManagedDockerGuards('ollama');
    }

    if (window.AuraConfigActions) {
        AuraConfigActions.register('ollama-test', { elementId: 'ollama-test-btn', requiresSaved: true, run: ollamaTestConnection });
        AuraConfigActions.register('ollama-recreate', { elementId: 'ollama-recreate-btn', requiresSaved: true, run: ollamaManagedRecreate });
    }
    const recreate = document.getElementById('ollama-recreate-btn');
    if (recreate && managedDockerUnavailableReason()) { recreate.disabled = true; recreate.title = managedDockerUnavailableReason(); }
    // Auto-check status
    ollamaManagedCheckStatus(managedEnabled);
}

function _ollamaField(path, label, help, inputHtml) {
    let h = '<div class="field-group">';
    h += '<div class="field-label">' + label + '</div>';
    if (help) h += '<div class="field-help">' + help + '</div>';
    h += inputHtml;
    h += '</div>';
    return h;
}

function _ollamaToggle(path, isOn, extra) {
    const onchange = extra ? ' onclick="toggleBool(this);' + extra + '"' : ' onclick="toggleBool(this)"';
    return '<div class="toggle-wrap">'
        + '<div class="toggle' + (isOn ? ' on' : '') + '" data-path="' + path + '"' + onchange + '></div>'
        + '<span class="toggle-label">' + (isOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>'
        + '</div>';
}

// eslint-disable-next-line no-unused-vars
function ollamaManagedToggled(el) {
    const isOn = el.classList.contains('on');
    ollamaExpectedOperation = '';
    const wrap = document.getElementById('ollama-recreate-wrap');
    if (wrap) wrap.classList.toggle('pw-u-hidden', !isOn);
    const input = document.querySelector('[data-path="ollama.url"]');
    if (input) { input.readOnly = isOn; input.value = isOn ? ollamaEffectiveURL((configData.ollama || {}).managed_instance || {}) : ollamaExternalURL; }
    ollamaManagedCheckStatus(isOn);
}

function ollamaSetManagedBanner(banner, kind, message) {
    banner.className = 'pw-u-status-banner ollama-managed-banner is-' + kind;
    banner.textContent = message;
}

function ollamaEffectiveURL(mi) {
    return isDockerRuntime() ? 'http://aurago_ollama_managed:11434' : 'http://127.0.0.1:' + (mi.container_port || 11434);
}

function ollamaOperationText(op) {
    const models = Object.entries(op.models || {}).map(([model,state]) => model + ': ' + t('config.ollama.state_' + state)).join(' · ');
    return t('config.ollama.state_' + op.state) + (op.model ? ': ' + op.model : '') + (models ? ' · ' + models : '') + (op.error ? ' — ' + op.error : '');
}

async function ollamaManagedCheckStatus(enabled) {
    ollamaStopPolling();
    const generation = ollamaPollGeneration;
    const banner = document.getElementById('ollama-managed-banner');
    if (!banner) return;
    if (!enabled) ollamaSetManagedBanner(banner, 'neutral', t('config.ollama.managed_status_disabled'));
    const controller = new AbortController();
    ollamaPollController = controller;
    const timeout = setTimeout(() => controller.abort(), 10000);
    try {
        const response = await fetch('/api/ollama/managed/status', { signal: controller.signal });
        const res = await response.json();
        if (!banner.isConnected || generation !== ollamaPollGeneration) return;
        if (!response.ok) throw new Error(res.message || t('config.ollama.managed_status_error'));
        const operations = (res.operations || []).sort((a, b) => new Date(b.updated) - new Date(a.updated));
        const op = ollamaExpectedOperation ? operations.find(item => item.id === ollamaExpectedOperation) : operations[0];
        if (res.status === 'disabled' && (!op || op.state === 'ready')) {
            ollamaSetManagedBanner(banner, 'neutral', t('config.ollama.managed_status_disabled'));
            return;
        }
        if (op) {
            if (op.state === 'ready' && !res.running && res.status !== 'disabled') {
                ollamaSetManagedBanner(banner, 'warning', t('config.ollama.managed_status_stopped'));
                return;
            }
            const terminal = ['ready', 'failed', 'cancelled', 'unknown', 'stopped'].includes(op.state);
            ollamaSetManagedBanner(banner, op.state === 'ready' ? 'success' : terminal ? 'warning' : 'neutral', ollamaOperationText(op));
            if (!terminal) ollamaPollTimer = setTimeout(() => ollamaManagedCheckStatus(true), 2000);
        } else if (ollamaExpectedOperation) {
            ollamaSetManagedBanner(banner, 'warning', t('config.ollama.state_unknown'));
        } else {
            const key = res.status === 'disabled' ? 'managed_status_disabled' : res.running ? 'container_unverified' : res.status === 'not_found' ? 'managed_status_not_found' : res.running === false ? 'managed_status_stopped' : 'managed_status_error';
            ollamaSetManagedBanner(banner, 'neutral', t('config.ollama.' + key));
        }
    } catch (error) {
        if (banner.isConnected && generation === ollamaPollGeneration) {
            ollamaSetManagedBanner(banner, 'error', error.message || t('config.ollama.managed_status_error'));
        }
    } finally { clearTimeout(timeout); }
}

async function ollamaTestConnection() {
    const result = document.getElementById('ollama-test-result');
    if (!result) return;
    result.textContent = t('config.ollama.managed_checking');
    try {
        const response = await fetch('/api/ollama/models?integration=ollama', { signal: AbortSignal.timeout(10000) });
        const data = await response.json();
        if (!response.ok) throw new Error(data.message || data.error || t('config.ollama.managed_status_error'));
        if (result.isConnected) result.textContent = t('config.ollama.connection_ok', {version: data.version, count: (data.models || []).length});
    } catch (error) { if (result.isConnected) result.textContent = error.message; }
}

async function ollamaManagedRecreate() {
    if (managedDockerUnavailableReason()) return;
    if (window.AuraConfigState?.isDirty()) return;
    if (!await showConfirm(t('config.ollama.managed_recreate_btn'), t('config.ollama.recreate_confirm'))) return;
    if (managedDockerUnavailableReason() || window.AuraConfigState?.isDirty()) return;
    const statusEl = document.getElementById('ollama-recreate-status');
    if (statusEl) statusEl.textContent = t('config.ollama.managed_recreating');
    try {
        const response = await fetch('/api/ollama/managed/recreate', {method:'POST', signal: AbortSignal.timeout(10000)});
        const data = await response.json();
        if (response.status !== 202 || !data.operation?.id) throw new Error(data.message || data.error || t('config.ollama.managed_status_error'));
        ollamaExpectedOperation = data.operation.id;
        if (statusEl?.isConnected) statusEl.textContent = t('config.ollama.managed_recreate_started');
        ollamaManagedCheckStatus(true);
    } catch (error) { if (statusEl?.isConnected) statusEl.textContent = error.message; }
}
