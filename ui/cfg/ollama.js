// cfg/ollama.js — Ollama config section with managed container control
/* global configData, t, cfgIcon, cfgActionResult, cfgStatusBanner, cfgDraftPatch */
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

    const form = window.AuraConfigForm;
    const gpuOptions = ['auto', 'nvidia', 'amd', 'intel', 'vulkan'].map(opt => ({ value: opt, label: opt }));
    const html = form.renderSpec({
        key: 'ollama',
        label: section.label,
        desc: section.desc,
        // Managed-container status sits right under the intro (it is only meaningful when managed is on).
        banner: { id: 'ollama-managed-banner', state: 'checking', detail: t('config.ollama.managed_checking') },
        groups: [{
            titleKey: 'config.refresh.connection',
            fields: [
                form.toggle({ label: t('config.ollama.enabled_label'), help: t('config.ollama.enabled_help'), path: 'ollama.enabled', value: data.enabled === true }),
                form.toggle({ label: t('config.ollama.readonly_label'), help: t('config.ollama.readonly_help'), path: 'ollama.readonly', value: data.readonly === true }),
                // The URL is fixed while the managed container is on (the container decides where it listens).
                managedEnabled
                    ? form.field({ type: 'url', label: t('config.ollama.url_label'), help: t('config.ollama.url_help'), path: 'ollama.url', value: ollamaEffectiveURL(miData), placeholder: 'http://localhost:11434', readonly: true })
                    : form.field({ type: 'url', label: t('config.ollama.url_label'), help: t('config.ollama.url_help'), path: 'ollama.url', value: data.url || '', placeholder: 'http://localhost:11434' }),
                '<div class="field-help">' + t('config.ollama.endpoint_help') + '</div>',
                '<div class="pw-u-inline-actions"><button id="ollama-test-btn" type="button" class="btn-secondary cfg-test-btn" data-config-action-draft="true">' + cfgIcon('plug') + '<span>' + t('config.ollama.test_connection') + '</span></button>' + cfgActionResult.html('ollama-test-result') + '</div>'
            ]
        }, {
            titleKey: 'config.ollama.managed_title',
            fields: [
                // Switching it on/off shows the recreate button and fixes or releases the URL.
                form.toggle({ label: t('config.ollama.managed_enabled_label'), help: t('config.ollama.managed_enabled_help'), path: 'ollama.managed_instance.enabled', value: managedEnabled, onclick: 'toggleBool(this);ollamaManagedToggled(this)' }),
                form.number({ label: t('config.ollama.managed_port_label'), help: t('config.ollama.managed_port_help'), path: 'ollama.managed_instance.container_port', value: miData.container_port || 11434, min: 1, max: 65535 }),
                form.toggle({ label: t('config.ollama.managed_gpu_label'), help: t('config.ollama.managed_gpu_help'), path: 'ollama.managed_instance.use_host_gpu', value: miData.use_host_gpu === true }),
                form.select({ label: t('config.ollama.managed_gpu_backend_label'), help: t('config.ollama.managed_gpu_backend_help'), path: 'ollama.managed_instance.gpu_backend', value: miData.gpu_backend || 'auto', options: gpuOptions }),
                form.field({ label: t('config.ollama.managed_models_label'), help: t('config.ollama.managed_models_help'), path: 'ollama.managed_instance.default_models', value: (miData.default_models || []).join(','), placeholder: 'llama3,mistral', dataType: 'array' }),
                form.field({ label: t('config.ollama.managed_memory_label'), help: t('config.ollama.managed_memory_help'), path: 'ollama.managed_instance.memory_limit', value: miData.memory_limit || '', placeholder: '8g' }),
                form.field({ label: t('config.ollama.managed_volume_label'), help: t('config.ollama.managed_volume_help'), path: 'ollama.managed_instance.volume_path', value: miData.volume_path || '' }),
                // Recreate button (shown when managed is enabled)
                '<div id="ollama-recreate-wrap" class="pw-u-mt-100' + (managedEnabled ? '' : ' pw-u-hidden') + '">'
                    + '<button class="btn-save pw-u-btn-md" id="ollama-recreate-btn" type="button" data-config-action-draft="false">' + cfgIcon('refresh') + '<span>' + t('config.ollama.managed_recreate_btn') + '</span></button>'
                    + cfgActionResult.html('ollama-recreate-status') + '</div>'
            ]
        }]
    });

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    if (typeof applyManagedDockerGuards === 'function') {
        applyManagedDockerGuards('ollama');
    }

    if (window.AuraConfigActions) {
        // The endpoint check runs with the URL typed in; recreating the container needs the saved settings.
        AuraConfigActions.register('ollama-test', {
            elementId: 'ollama-test-btn', requiresSaved: false, run: ollamaTestConnection,
            result: {
                target: 'ollama-test-result',
                pending: t('config.ollama.managed_checking'),
                success: outcome => t('config.ollama.connection_ok', { version: outcome.body.version, count: outcome.body.count })
            }
        });
        AuraConfigActions.register('ollama-recreate', {
            elementId: 'ollama-recreate-btn', requiresSaved: true, run: ollamaManagedRecreate,
            result: {
                target: 'ollama-recreate-status',
                pending: t('config.ollama.managed_recreating'),
                success: t('config.ollama.managed_recreate_started')
            }
        });
    }
    const recreate = document.getElementById('ollama-recreate-btn');
    if (recreate && managedDockerUnavailableReason()) { recreate.disabled = true; recreate.title = managedDockerUnavailableReason(); }
    // Auto-check status
    ollamaManagedCheckStatus(managedEnabled);
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

// state: off | checking | connected | degraded | error | unknown (see cfgStatusBanner).
function ollamaSetManagedBanner(banner, state, message) {
    cfgStatusBanner(banner, state, Object.assign({ detail: message }, state === 'error' ? { retry: () => ollamaManagedCheckStatus(true) } : {}));
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
    if (!enabled) ollamaSetManagedBanner(banner, 'off', t('config.ollama.managed_status_disabled'));
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
            ollamaSetManagedBanner(banner, 'off', t('config.ollama.managed_status_disabled'));
            return;
        }
        if (op) {
            if (op.state === 'ready' && !res.running && res.status !== 'disabled') {
                ollamaSetManagedBanner(banner, 'degraded', t('config.ollama.managed_status_stopped'));
                return;
            }
            const terminal = ['ready', 'failed', 'cancelled', 'unknown', 'stopped'].includes(op.state);
            ollamaSetManagedBanner(banner, op.state === 'ready' ? 'connected' : op.state === 'failed' ? 'error' : terminal ? 'degraded' : 'checking', ollamaOperationText(op));
            if (!terminal) ollamaPollTimer = setTimeout(() => ollamaManagedCheckStatus(true), 2000);
        } else if (ollamaExpectedOperation) {
            ollamaSetManagedBanner(banner, 'degraded', t('config.ollama.state_unknown'));
        } else {
            const key = res.status === 'disabled' ? 'managed_status_disabled' : res.running ? 'container_unverified' : res.status === 'not_found' ? 'managed_status_not_found' : res.running === false ? 'managed_status_stopped' : 'managed_status_error';
            ollamaSetManagedBanner(banner, key === 'managed_status_disabled' ? 'off' : key === 'managed_status_stopped' ? 'degraded' : key === 'managed_status_error' ? 'error' : 'unknown', t('config.ollama.' + key));
        }
    } catch (error) {
        if (banner.isConnected && generation === ollamaPollGeneration) {
            ollamaSetManagedBanner(banner, 'error', error.message || t('config.ollama.managed_status_error'));
        }
    } finally { clearTimeout(timeout); }
}

// Checks the endpoint typed into the form (the server reads the URL from the draft).
async function ollamaTestConnection() {
    const response = await fetch('/api/ollama/models?integration=ollama', {
        method: 'POST', signal: AbortSignal.timeout(10000),
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['ollama']) })
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.message || data.error || t('config.ollama.managed_status_error'));
    return { status: 'ok', version: data.version, count: (data.models || []).length };
}

// Runs behind the registered result line (confirmation and busy state come from there).
async function ollamaManagedRecreate() {
    if (managedDockerUnavailableReason()) throw new Error(managedDockerUnavailableReason());
    if (window.AuraConfigState?.isDirty()) throw new Error(t('config.feedback.runtime_only'));
    // A declined confirmation ends the run quietly (cancelled), busy state still resets.
    if (!await showConfirm(t('config.ollama.managed_recreate_btn'), t('config.ollama.recreate_confirm'))) throw new DOMException('cancelled', 'AbortError');
    if (managedDockerUnavailableReason() || window.AuraConfigState?.isDirty()) throw new Error(t('config.feedback.runtime_only'));
    const response = await fetch('/api/ollama/managed/recreate', {method:'POST', signal: AbortSignal.timeout(10000)});
    const data = await response.json();
    if (response.status !== 202 || !data.operation?.id) throw new Error(data.message || data.error || t('config.ollama.managed_status_error'));
    ollamaExpectedOperation = data.operation.id;
    ollamaManagedCheckStatus(true);
    return { status: 'ok' };
}
