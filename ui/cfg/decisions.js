// Decisions shares the saved config lifecycle; status polling never starts inference.
function renderDecisionsSection(section) {
    const tr = (key, params) => t('config.decisions.' + key, params);
    const data = configData.decisions || {};
    const root = document.createElement('div');
    root.className = 'cfg-section active';
    root.dataset.decisionsSettings = '';
    const node = (tag, cls, text, parent = root) => {
        const element = document.createElement(tag);
        if (cls) element.className = cls;
        if (text !== undefined && text !== null) element.textContent = text;
        parent.appendChild(element);
        return element;
    };
    node('h2', 'section-header', tr('title'));
    node('p', 'section-desc', section.desc);
    node('p', 'cfg-note-banner cfg-note-banner-info', tr('purpose'));

    const backendGroup = node('label', 'field-group');
    node('span', 'field-label', tr('backend'), backendGroup);
    const backend = node('select', 'field-select', '', backendGroup);
    backend.dataset.path = 'decisions.backend';
    for (const [value, key] of [['local', 'backend_local'], ['provider', 'backend_provider']]) {
        backend.add(new Option(tr(key), value));
    }
    backend.value = data.backend === 'provider' ? 'provider' : 'local';
    node('p', 'field-help', tr('backend_help'));

    const toggle = (key, label, parent = root) => {
        const group = node('div', 'field-group', '', parent);
        node('span', 'field-label', label, group);
        const wrap = node('div', 'toggle-wrap', '', group);
        const control = node('div', 'toggle' + (data[key] === true ? ' on' : ''), '', wrap);
        control.dataset.path = 'decisions.' + key;
        control.setAttribute('role', 'switch');
        control.setAttribute('aria-label', label);
        control.setAttribute('aria-checked', String(data[key] === true));
        control.tabIndex = 0;
        control.addEventListener('click', () => toggleBool(control));
        node('span', 'toggle-label', t(data[key] ? 'config.toggle.active' : 'config.toggle.inactive'), wrap);
        return control;
    };
    toggle('enabled', tr('enabled'));

    const localPanel = node('div', 'cfg-group');
    localPanel.dataset.decisionsMode = 'local';
    const deploymentLabel = node('label', 'field-group', '', localPanel);
    node('span', 'field-label', tr('deployment'), deploymentLabel);
    const deployment = node('select', 'field-select', '', deploymentLabel);
    deployment.dataset.path = 'decisions.deployment';
    [['auto', tr('auto')], ['native', tr('native')], ['docker', 'Docker']].forEach(([value, label]) => deployment.add(new Option(label, value)));
    deployment.value = data.deployment || 'auto';
    node('p', 'field-help', tr('gpu_first'), localPanel);
    node('p', 'field-help', tr('deployment_help'), localPanel);
    toggle('cpu_fallback_allowed', tr('cpu_permission'), localPanel);
    node('p', 'field-help', tr('cpu_help'), localPanel);

    const providerPanel = node('div', 'cfg-group');
    providerPanel.dataset.decisionsMode = 'provider';
    const providerGroup = node('label', 'field-group', '', providerPanel);
    node('span', 'field-label', tr('provider'), providerGroup);
    const providerSelect = node('select', 'field-select', '', providerGroup);
    providerSelect.dataset.path = 'decisions.provider';
    providerSelect.add(new Option(tr('provider_select'), ''));
    const providerHelp = node('p', 'field-help', tr('provider_help'), providerPanel);
    const providerValidation = node('p', 'field-help', '', providerPanel);
    providerValidation.setAttribute('role', 'status');
    const providerActions = node('div', 'cfg-actions cfg-actions-row', '', providerPanel);
    const createProvider = node('button', 'btn-secondary', tr('provider_add'), providerActions);
    createProvider.type = 'button';
    createProvider.dataset.decisionsCreateProvider = '';
    const manageProviders = node('button', 'btn-secondary', tr('provider_manage'), providerActions);
    manageProviders.type = 'button';
    const modelGroup = node('label', 'field-group', '', providerPanel);
    node('span', 'field-label', tr('model'), modelGroup);
    const modelSelect = node('select', 'field-select', '', modelGroup);
    modelSelect.dataset.path = 'decisions.model';
    modelSelect.add(new Option(data.model ? tr('model_saved_unverified', {value: data.model}) : tr('model_select'), data.model || ''));
    const modelHelp = node('p', 'field-help', tr('model_help'), providerPanel);
    const catalogActions = node('div', 'cfg-actions cfg-actions-row', '', providerPanel);
    const loadModels = node('button', 'btn-secondary', tr('load_models'), catalogActions);
    loadModels.type = 'button';
    loadModels.dataset.decisionsLoadModels = '';
    const catalogResult = node('p', 'field-help', '', providerPanel);
    catalogResult.dataset.decisionsModels = '';
    catalogResult.setAttribute('role', 'status');
    const modelDetails = node('div', 'cfg-note-banner cfg-note-banner-info', '', providerPanel);
    modelDetails.dataset.decisionsModelDetails = '';
    modelDetails.hidden = true;
    const disclosure = node('p', 'cfg-note-banner cfg-note-banner-warning', tr('external_data_notice'), providerPanel);
    disclosure.dataset.decisionsExternalNotice = '';
    const externalTest = node('button', 'btn-secondary', tr('test'), providerPanel);
    externalTest.type = 'button';
    externalTest.dataset.decisionsAction = 'test-provider';
    const result = node('p', 'field-help', '', providerPanel);
    result.dataset.decisionsProviderResult = '';
    result.setAttribute('role', 'status');

    const localHelp = node('p', 'field-help', tr('install_help'), localPanel);
    node('p', 'field-help', tr('runtime_setup_help'), localPanel);
    const setupLink = node('a', 'field-help', tr('setup_instructions'), localPanel);
    setupLink.href = 'https://github.com/antibyte/AuraGo/blob/main/decisions_runtime/README.md#build-locally';
    setupLink.target = '_blank';
    setupLink.rel = 'noopener noreferrer';
    const statusHeading = node('h3', 'cfg-subheading', tr('status'));
    const statusText = node('p', 'cfg-note-banner', t('common.loading'));
    statusText.dataset.decisionsStatus = '';
    statusText.setAttribute('role', 'status');
    const deviceText = node('p', 'field-help');
    const cpuText = node('p', 'cfg-note-banner cfg-note-banner-warning');
    cpuText.hidden = true;
    const progress = node('progress', '');
    progress.max = 100;
    progress.hidden = true;
    progress.setAttribute('aria-label', tr('status'));
    const phaseText = node('p', 'field-help');
    phaseText.dataset.decisionsProgress = '';
    phaseText.setAttribute('role', 'status');
    phaseText.hidden = true;
    const actions = node('div', 'cfg-actions cfg-actions-row', '', localPanel);
    const buttons = {};
    for (const [action, label] of [['install', t('config.local_llm.install')], ['start', t('config.local_llm.start')], ['stop', t('config.local_llm.stop')], ['test', tr('test')]]) {
        const button = node('button', 'btn-secondary', label, actions);
        button.type = 'button';
        button.dataset.decisionsAction = action;
        button.addEventListener('click', () => run(action));
        buttons[action] = button;
    }
    node('p', 'cfg-note-banner', 'Jeff-2B · Q4_K_M · ' + tr('uncalibrated'), localPanel);
    const localResult = node('p', 'field-help', '', localPanel);
    localResult.dataset.decisionsResult = '';
    localResult.setAttribute('role', 'status');

    const lifetime = new AbortController();
    let status, busy = false, activeAction = '', actionRevision = 0, timer, statusRequest, modelsRequest, activeRequest, activeOperation;
    let loadedModels = [];
    let pendingAction = null;
    const attached = () => !lifetime.signal.aborted && root.isConnected;
    const providerMode = () => backend.value === 'provider';
    const dirty = () => typeof hasUnsavedConfigChanges === 'function' && hasUnsavedConfigChanges();
    const currentProviders = () => Array.isArray(providersCache) ? providersCache : [];
    const providerOptions = () => currentProviders().filter(p => String(p.type || '').toLowerCase() === 'openrouter');
    const selectedProvider = () => currentProviders().find(p => String(p.id) === String(providerSelect.value));
    const cancelModels = () => {
        modelsRequest?.abort();
        modelsRequest = null;
    };
    const invalidateAction = () => {
        if ((!busy || !activeOperation) && !pendingAction) return;
        const operation = activeOperation || {providerMode: false};
        pendingAction = null;
        actionRevision++;
        activeRequest?.abort();
        activeRequest = null;
        busy = false;
        activeAction = '';
        activeOperation = null;
        const uncertain = operation.requestStarted && operation.providerMode && operation.action === 'test';
        (operation.providerMode ? result : localResult).textContent = uncertain
            ? errorText('external_outcome_unknown') : errorText('cancelled');
        updateButtons();
    };
    const isEnabled = () => root.querySelector('[data-path="decisions.enabled"]')?.classList.contains('on') === true;
    const configError = () => {
        if (!providerMode() || !isEnabled()) return '';
        const selected = selectedProvider();
        if (!providerSelect.value) return tr('provider_missing');
        if (!selected) return tr('provider_not_configured');
        if (String(selected.type || '').toLowerCase() !== 'openrouter') return tr('provider_not_openrouter');
        if (!['', 'https://openrouter.ai', 'https://openrouter.ai/api/v1', 'https://openrouter.ai/api/v1/'].includes(String(selected.base_url || ''))) return tr('provider_url_not_canonical');
        if (!modelSelect.value) return tr('model_missing');
        return '';
    };
    const renderProviders = preferred => {
        const current = preferred || providerSelect.value || data.provider || '';
        providerSelect.replaceChildren(new Option(tr('provider_select'), ''));
        for (const p of providerOptions()) providerSelect.add(new Option((p.name || p.id) + ' (' + p.id + ')', p.id));
        const preserve = current && !providerOptions().some(p => String(p.id) === String(current));
        if (preserve) {
            const entry = currentProviders().find(p => String(p.id) === String(current));
            const label = entry ? (entry.name || entry.id) + ' — ' + tr('provider_not_openrouter') : t('config.field.option_missing', {value: current});
            const option = new Option(label, current);
            option.dataset.preservedProvider = 'true';
            providerSelect.add(option);
        }
        providerSelect.value = current;
        providerHelp.textContent = tr('provider_help');
    };
    const modelFor = id => loadedModels.find(model => String(model.id) === String(id));
    const modelPrice = value => {
        if (value == null || String(value).trim() === '') return tr('price_unavailable');
        const n = Number(value);
        if (!Number.isFinite(n)) return tr('price_unavailable');
        if (n < 0) return tr('price_unavailable');
        if (n === 0) return tr('price_free');
        return String(value) + ' ' + tr('usd_per_token');
    };
    const showModelDetails = () => {
        const model = modelFor(modelSelect.value);
        if (!model) {
            modelDetails.hidden = true;
            modelDetails.textContent = '';
            return;
        }
        const modalities = Array.isArray(model.input_modalities) ? model.input_modalities.join(', ') : tr('unknown');
        const context = Number(model.context_length);
        const pricing = model.pricing || {};
        modelDetails.textContent = [
            model.name && model.name !== model.id ? model.name : model.id,
            tr('model_inputs', {value: modalities}),
            Number.isFinite(context) && context > 0 ? tr('model_context', {value: context.toLocaleString()}) : tr('context_unavailable'),
            tr('price_input', {value: modelPrice(pricing.prompt)}),
            tr('price_output', {value: modelPrice(pricing.completion)})
        ].join(' · ');
        modelDetails.hidden = false;
    };
    const updateButtons = () => {
        if (!attached()) return;
        const local = !providerMode();
        const changing = !!pendingAction || ['installing', 'downloading', 'starting', 'stopping', 'discovering_devices'].includes(status?.state);
        for (const [action, button] of Object.entries(buttons)) {
            button.hidden = !local;
            button.disabled = action === 'stop' ? busy && !['test', 'install', 'start'].includes(activeAction) : busy || changing || dirty() || !status?.enabled || (action === 'test' && !status?.ready);
        }
        localHelp.hidden = !local;
        progress.hidden = !local || !['installing', 'downloading', 'starting', 'discovering_devices'].includes(status?.state);
        phaseText.hidden = progress.hidden;
        deviceText.hidden = false;
        cpuText.hidden = !local || !status?.cpu_fallback_active;
        providerPanel.hidden = !providerMode();
        localPanel.hidden = providerMode();
        externalTest.disabled = busy || dirty() || !status?.enabled || !selectedProvider() || !modelSelect.value;
        loadModels.disabled = busy || !providerSelect.value;
        createProvider.disabled = busy;
        manageProviders.disabled = busy;
        const invalid = configError();
        providerValidation.textContent = invalid;
        providerValidation.className = 'field-help' + (invalid ? ' cfg-note-banner-warning' : '');
    };
    const reason = code => {
        if (!code) return '';
        const exact = {
            service_disabled: 'disabled', cpu_consent_required: 'cpu_required',
            hardware_unavailable: 'hardware_unavailable', model_not_installed: 'not_installed',
            runtime_unavailable: 'runtime_missing', runtime_artifact_missing: 'runtime_missing', service_not_ready: 'not_ready',
            docker_unavailable: 'docker_unavailable', deployment_denied: 'docker_unavailable',
            model_integrity_failed: 'integrity_failed', runtime_attestation_failed: 'attestation_failed',
            lifecycle_busy: 'lifecycle_busy', runtime_start_timeout: 'runtime_start_timeout', runtime_invalid_configuration: 'runtime_invalid_configuration'
        };
        return tr(exact[code] || 'failed') + ' (' + code + ')';
    };
    const errorText = code => {
        const keys = {
            provider_not_configured: 'provider_not_configured', provider_key_missing: 'provider_key_missing',
            provider_auth_failed: 'provider_auth_failed', provider_credit_exhausted: 'provider_credit_exhausted',
            provider_rate_limited: 'provider_rate_limited',
            provider_lifecycle_unavailable: 'provider_lifecycle_unavailable', configuration_changed: 'configuration_changed',
            budget_exceeded: 'budget_exceeded', external_provider_failed: 'external_provider_failed',
            external_outcome_unknown: 'external_outcome_unknown', invalid_request: 'invalid_request',
            input_token_budget_exceeded: 'input_token_budget_exceeded', cancelled: 'cancelled',
            provider_error: 'external_provider_failed'
        };
        return tr(keys[code] || 'failed');
    };
    const paint = () => {
        if (!attached() || !status) return;
        if (providerMode()) {
            const state = status.ready ? tr('provider_ready') : status.state === 'error' ? errorText(status.last_error || status.code) : status.configured ? tr('provider_configured') : tr('provider_needs_setup');
            statusText.textContent = state;
            statusText.className = 'cfg-note-banner ' + (status.ready ? 'cfg-note-banner-success' : status.state === 'error' ? 'cfg-note-banner-warning' : '');
            deviceText.textContent = status.last_success ? tr('provider_last_success', {value: new Date(status.last_success).toLocaleString()}) : tr('provider_never_tested');
            cpuText.hidden = true;
        } else {
            const stateKeys = {disabled: 'disabled', stopped: 'stopped', installing: 'downloading', downloading: 'downloading', starting: 'starting', discovering_devices: 'starting', blocked: 'error', error: 'error', stopped_with_error: 'error'};
            const state = status.ready ? tr('ready') : status.state === 'installed' ? tr('installed') : reason(status.code) || (stateKeys[status.state] ? t('config.local_llm.state_' + stateKeys[status.state]) : tr('not_ready'));
            statusText.textContent = state;
            statusText.className = 'cfg-note-banner ' + (status.ready || status.state === 'installed' ? 'cfg-note-banner-success' : status.code ? 'cfg-note-banner-warning' : '');
            const mode = {gpu: 'GPU', cpu: 'CPU', mixed: 'GPU + CPU'}[status.execution_mode] || '';
            deviceText.textContent = tr('device') + ': ' + (status.device || '—') + (status.backend ? ' · ' + status.backend : '') + ' · ' + (status.deployment || '—') + (mode ? ' · ' + mode : '');
            cpuText.hidden = !status.cpu_fallback_active;
            const fallbackReason = status.cpu_fallback_reason;
            cpuText.textContent = tr('cpu_active') + (fallbackReason ? ' ' + tr(fallbackReason === 'gpu_accelerator_partial' ? 'cpu_partial_reason' : 'cpu_unavailable_reason') + ' (' + fallbackReason + ')' : '');
            if (Number(status.progress?.total_bytes) > 0) progress.value = Math.max(0, Math.min(100, Number(status.progress?.percent) || 0));
            else progress.removeAttribute('value');
            const phases = {verifying_runtime: 'verifying_runtime', verifying_model: 'verifying_model', verified_model: 'verifying_model', verifying_projector: 'verifying_projector', verified_projector: 'verifying_projector', loading_model: 'loading_model', loading_projector: 'loading_projector', warming_up: 'warming_up'};
            phaseText.textContent = phases[status.progress?.phase] ? tr(phases[status.progress.phase]) : statusText.textContent;
            if (pendingAction && pendingAction.revision === actionRevision) {
                let outcome = '';
                if (pendingAction.action === 'install' && status.state === 'installed') outcome = tr('installed');
                else if (pendingAction.action === 'start' && status.ready) outcome = tr('ready');
                else if (['error', 'blocked', 'stopped_with_error'].includes(status.state)) outcome = reason(status.code) || tr('failed');
                else if (['stopped', 'disabled'].includes(status.state)) outcome = errorText('cancelled');
                if (outcome) { localResult.textContent = outcome; pendingAction = null; }
            }
        }
        updateButtons();
    };
    const refresh = async () => {
        if (!attached()) return;
        clearTimeout(timer);
        statusRequest?.abort();
        const request = new AbortController();
        statusRequest = request;
        const timeout = setTimeout(() => request.abort(), 10000);
        try {
            const response = await fetch('/api/decisions/status', {credentials: 'same-origin', signal: request.signal});
            if (!response.ok) throw new Error('status');
            const next = await response.json();
            if (!attached() || request.signal.aborted || statusRequest !== request) return;
            status = next;
            paint();
        } catch (_) {
            if (attached() && statusRequest === request) { statusText.textContent = tr('failed'); status = null; updateButtons(); }
        } finally {
            clearTimeout(timeout);
            if (attached() && statusRequest === request) timer = setTimeout(refresh, status?.enabled ? 5000 : 15000);
        }
    };
    const loadModelCatalog = async () => {
        if (!attached()) return;
        const id = providerSelect.value;
        const provider = selectedProvider();
        if (!provider || String(provider.type || '').toLowerCase() !== 'openrouter') { catalogResult.textContent = tr('provider_missing'); return; }
        cancelModels();
        const request = new AbortController();
        modelsRequest = request;
        const timeout = setTimeout(() => request.abort(), 30000);
        loadModels.disabled = true;
        catalogResult.textContent = tr('models_loading');
        try {
            const response = await fetch('/api/decisions/models?provider=' + encodeURIComponent(id), {credentials: 'same-origin', signal: request.signal});
            const body = await response.json();
            if (!attached() || request.signal.aborted || modelsRequest !== request) return;
            if (!response.ok) throw new Error(body?.code || 'external_provider_failed');
            loadedModels = Array.isArray(body.models) ? body.models : [];
            if (!loadedModels.length) {
                catalogResult.textContent = tr('models_empty');
                return;
            }
            const saved = modelSelect.value || data.model || '';
            modelSelect.replaceChildren(new Option(tr('model_select'), ''));
            for (const model of loadedModels) modelSelect.add(new Option((model.name && model.name !== model.id ? model.name + ' · ' : '') + model.id, model.id));
            if (saved && !loadedModels.some(model => String(model.id) === saved)) {
                const option = new Option(tr('model_saved_unavailable', {value: saved}), saved);
                option.dataset.preservedModel = 'true';
                modelSelect.add(option);
            }
            modelSelect.value = saved;
            catalogResult.textContent = tr('models_loaded', {count: loadedModels.length});
            showModelDetails();
        } catch (error) {
            if (attached() && modelsRequest === request) catalogResult.textContent = errorText(error.message || 'external_provider_failed');
        } finally {
            clearTimeout(timeout);
            if (attached() && modelsRequest === request) updateButtons();
        }
    };
    const run = async action => {
        if (!attached() || (busy && !(action === 'stop' && ['test', 'install', 'start'].includes(activeAction)))) return;
        const useProvider = providerMode();
        if (action === 'stop') { activeRequest?.abort(); pendingAction = null; }
        const actionResult = useProvider ? result : localResult;
        if (action !== 'stop' && dirty()) { actionResult.textContent = t('config.local_llm.save_first'); return; }
        let revision = actionRevision;
        if (useProvider && action === 'test') {
            if (!status?.enabled || !selectedProvider() || !modelSelect.value) return;
            const operation = {providerMode: useProvider, action, provider: providerSelect.value, model: modelSelect.value, requestStarted: false};
            revision = ++actionRevision;
            busy = true;
            activeAction = 'confirm_test';
            activeOperation = operation;
            updateButtons();
            const unknown = status?.last_error === 'external_outcome_unknown';
            let confirmed = false;
            try { confirmed = await showConfirm(tr('external_test_title'), tr(unknown ? 'external_unknown_retry_confirm' : 'external_test_confirm')); }
            catch (_) { confirmed = false; }
            if (!confirmed || !attached() || revision !== actionRevision) {
                if (revision === actionRevision) {
                    busy = false;
                    activeAction = '';
                    activeOperation = null;
                    updateButtons();
                }
                return;
            }
            if (providerMode() !== operation.providerMode || providerSelect.value !== operation.provider || modelSelect.value !== operation.model || dirty()) {
                actionResult.textContent = errorText('configuration_changed');
                busy = false;
                activeAction = '';
                activeOperation = null;
                updateButtons();
                return;
            }
            activeAction = action;
        } else {
            revision = ++actionRevision;
            busy = true;
            activeAction = action;
            activeOperation = {providerMode: useProvider, action, provider: providerSelect.value, model: modelSelect.value, requestStarted: false};
        }
        updateButtons();
        actionResult.textContent = t('config.local_llm.working');
        const request = new AbortController();
        activeRequest = request;
        const abort = () => request.abort();
        lifetime.signal.addEventListener('abort', abort, {once: true});
        const timeout = setTimeout(abort, 125000);
        try {
            if (activeOperation) activeOperation.requestStarted = true;
            const response = await fetch('/api/decisions/' + (useProvider ? 'test' : action), {method: 'POST', credentials: 'same-origin', signal: request.signal});
            const body = await response.json();
            if (!attached() || revision !== actionRevision) return;
            if (response.ok) {
                if (!useProvider && response.status === 202 && ['install', 'start'].includes(action)) {
                    pendingAction = {action, revision};
                    actionResult.textContent = tr('action_pending');
                } else actionResult.textContent = useProvider || action === 'test' ? tr('test_passed') : action === 'install' ? tr('installed') : action === 'start' ? tr('ready') : t('config.local_llm.state_stopped');
            }
            else actionResult.textContent = useProvider ? errorText(body.code || 'external_provider_failed') : reason(body.code || 'request_failed');
            await refresh();
        } catch (_) {
            if (attached() && revision === actionRevision) actionResult.textContent = useProvider
                ? errorText('external_outcome_unknown') : tr('failed');
        } finally {
            clearTimeout(timeout);
            lifetime.signal.removeEventListener('abort', abort);
            if (revision === actionRevision) {
                busy = false;
                activeAction = '';
                activeRequest = null;
                activeOperation = null;
                updateButtons();
            }
        }
    };
    const dispose = () => { lifetime.abort(); statusRequest?.abort(); cancelModels(); activeRequest?.abort(); clearTimeout(timer); observer.disconnect(); };
    const observer = new MutationObserver(() => { if (!root.isConnected) dispose(); });
    const updateMode = () => {
        cancelModels();
        invalidateAction();
        if (catalogResult.textContent === tr('models_loading')) catalogResult.textContent = '';
        localPanel.hidden = providerMode();
        providerPanel.hidden = !providerMode();
        updateButtons();
    };
    backend.addEventListener('change', updateMode);
    providerSelect.addEventListener('change', () => {
        cancelModels();
        invalidateAction();
        loadedModels = [];
        modelDetails.hidden = true;
        modelSelect.replaceChildren(new Option(tr('model_select'), ''));
        catalogResult.textContent = '';
        modelSelect.value = '';
        modelSelect.dispatchEvent(new Event('change', {bubbles: true}));
        updateButtons();
    });
    modelSelect.addEventListener('change', () => { invalidateAction(); showModelDetails(); updateButtons(); });
    loadModels.addEventListener('click', loadModelCatalog);
    externalTest.addEventListener('click', () => run('test'));
    createProvider.addEventListener('click', async () => {
        try {
            await loadModule('providers');
            if (typeof providerAdd === 'function') providerAdd('decisions');
        } catch (_) { result.textContent = tr('provider_lifecycle_unavailable'); }
    });
    manageProviders.addEventListener('click', () => {
        const item = document.querySelector('[data-section="providers"]');
        if (item) item.click();
    });
    window.addEventListener('aurago:providers-saved', event => {
        renderProviders(event.detail?.preferredProviderId || '');
        if (event.detail?.preferredProviderId) {
            providerSelect.dispatchEvent(new Event('change', {bubbles: true}));
        }
        updateButtons();
    }, {signal: lifetime.signal});

    renderProviders(data.provider || '');
    updateMode();
    document.getElementById('content').replaceChildren(root);
    observer.observe(document.body, {childList: true, subtree: true});
    window.addEventListener('cfg:section-leave', dispose, {signal: lifetime.signal, once: true});
    window.addEventListener('pagehide', dispose, {signal: lifetime.signal, once: true});
    document.addEventListener('aurago:config-saved', refresh, {signal: lifetime.signal});
    for (const event of ['input', 'change', 'click']) document.addEventListener(event, () => queueMicrotask(updateButtons), {signal: lifetime.signal});
    attachChangeListeners();
    updateButtons();
    refresh();
}
