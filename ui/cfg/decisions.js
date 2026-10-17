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
    const saveNotice = node('p', 'cfg-note-banner cfg-note-banner-info', t('config.local_llm.save_first'));
    saveNotice.dataset.decisionsSaveNotice = '';
    saveNotice.hidden = true;

    const localPanel = node('div', 'cfg-group');
    localPanel.dataset.decisionsMode = 'local';
    node('h3', 'cfg-subheading', tr('local_setup'), localPanel);
    node('p', 'field-help', tr('local_server_help'), localPanel);
    const advanced = node('details', 'decisions-details', '', localPanel);
    node('summary', '', tr('advanced'), advanced);
    const deploymentLabel = node('label', 'field-group', '', advanced);
    node('span', 'field-label', tr('deployment'), deploymentLabel);
    const deployment = node('select', 'field-select', '', deploymentLabel);
    deployment.dataset.path = 'decisions.deployment';
    [['auto', tr('auto')], ['native', tr('native')], ['docker', 'Docker']].forEach(([value, label]) => deployment.add(new Option(label, value)));
    deployment.value = data.deployment || 'auto';
    node('p', 'field-help', tr('deployment_help'), advanced);
    toggle('cpu_fallback_allowed', tr('cpu_permission'), advanced);
    node('p', 'field-help', tr('cpu_help'), advanced);

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
    const pinModel = node('button', 'btn-secondary', tr('pin_model'), catalogActions);
    pinModel.type = 'button';
    pinModel.dataset.decisionsPinModel = '';
    pinModel.hidden = true;
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

    const buttons = {};
    node('p', 'field-help', tr('setup_help'), localPanel);
    const setupActions = node('div', 'cfg-actions cfg-actions-row', '', localPanel);
    const setupButton = node('button', 'btn-primary', tr('setup_action'), setupActions);
    setupButton.type = 'button';
    setupButton.dataset.decisionsAction = 'setup';
    setupButton.addEventListener('click', () => run('setup'));
    buttons.setup = setupButton;
    const steps = node('div', 'cfg-actions cfg-actions-row', '', advanced);
    for (const [action, label] of [
        ['check', 'check'], ['install', 'download'], ['start', 'start_step']
    ]) {
        const step = node('div', '', '', steps);
        const button = node('button', 'btn-secondary', tr(label), step);
        button.type = 'button';
        button.dataset.decisionsAction = action;
        button.addEventListener('click', () => run(action));
        buttons[action] = button;
    }
    const checkStep = buttons.check.parentElement;
    const checkResult = node('p', 'cfg-note-banner', '', checkStep);
    checkResult.dataset.decisionsCheck = '';
    checkResult.setAttribute('role', 'status');
    checkResult.hidden = true;
    const setupLink = node('a', 'field-help', tr('setup_instructions'), advanced);
    setupLink.href = 'https://github.com/antibyte/AuraGo/blob/main/documentation/decisions.md';
    setupLink.target = '_blank';
    setupLink.rel = 'noopener noreferrer';
    const localHelp = node('p', 'field-help', tr('install_help'), localPanel);
    localPanel.appendChild(advanced);
    const statusCard = node('div', 'cfg-group decisions-status');
    root.insertBefore(statusCard, localPanel);
    node('h3', 'cfg-subheading', tr('status'), statusCard);
    const statusText = node('p', 'cfg-note-banner', t('common.loading'), statusCard);
    statusText.dataset.decisionsStatus = '';
    statusText.setAttribute('role', 'status');
    const deviceText = node('p', 'field-help', '', statusCard);
    const cpuText = node('p', 'cfg-note-banner cfg-note-banner-warning', '', statusCard);
    cpuText.hidden = true;
    const progress = node('progress', '', '', statusCard);
    progress.max = 100;
    progress.hidden = true;
    progress.setAttribute('aria-label', tr('status'));
    const phaseText = node('p', 'field-help', '', statusCard);
    phaseText.dataset.decisionsProgress = '';
    phaseText.setAttribute('role', 'status');
    phaseText.hidden = true;
    const actions = node('div', 'cfg-actions cfg-actions-row', '', statusCard);
    for (const [action, label] of [['stop', t('config.local_llm.stop')], ['test', tr('test')]]) {
        const button = node('button', 'btn-secondary', label, actions);
        button.type = 'button';
        button.dataset.decisionsAction = action;
        button.addEventListener('click', () => run(action));
        buttons[action] = button;
    }
    const diagnostics = node('details', 'decisions-details', '', statusCard);
    node('summary', '', tr('diagnostics'), diagnostics);
    const errorCode = node('code', '', '', diagnostics);
    node('p', 'field-help', 'Jeff-2B · Q4_K_M · ' + tr('uncalibrated'), diagnostics);
    const localResult = node('p', 'field-help', '', statusCard);
    localResult.dataset.decisionsResult = '';
    localResult.setAttribute('role', 'status');

    const lifetime = new AbortController();
    let status, busy = false, activeAction = '', actionRevision = 0, timer, statusRequest, modelsRequest, activeRequest, activeOperation;
    let loadedModels = [];
    let pendingAction = null;
    let installationCheck = null;
    const attached = () => !lifetime.signal.aborted && root.isConnected;
    const providerMode = () => backend.value === 'provider';
    const localReady = () => status?.ready && status.backend !== 'provider';
    const dirty = () => typeof hasUnsavedConfigChanges === 'function' && hasUnsavedConfigChanges();
    const localDirty = () => window.AuraConfigState ? window.AuraConfigState.dirtyPaths().some(path => path === 'decisions' || path.startsWith('decisions.')) : dirty();
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
    const modelFor = id => {
        const current = status?.provider === providerSelect.value ? status.model_availability?.model : null;
        if (current && [current.id, current.canonical_slug, current.alias_target].includes(id)) return current;
        return loadedModels.find(model => [model.id, model.canonical_slug, model.alias_target].includes(id));
    };
    const modelBlocked = () => {
        if (status?.model === modelSelect.value && status?.provider === providerSelect.value && status.model_availability) return status.model_availability.state === 'blocked';
        const model = modelFor(modelSelect.value);
        return !!model && (model.usable === false || Number(model.context_length) <= 0);
    };
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
        const target = model?.alias_target || model?.canonical_slug;
        pinModel.hidden = !target || target === modelSelect.value;
        pinModel.disabled = busy || modelBlocked();
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
        if (target && target !== modelSelect.value) modelDetails.textContent += ' · ' + tr('alias_target', {value: target});
        if (model.expiration_date) modelDetails.textContent += ' · ' + tr('model_expiry', {value: model.expiration_date});
        if (model.media_limits && model.supports_images) modelDetails.textContent += ' · ' + tr('media_limits', {count: model.media_limits.max_images, bytes: Math.round(model.media_limits.max_image_bytes / 1024)});
        if (model.checked_at) modelDetails.textContent += ' · ' + tr('model_checked', {value: new Date(model.checked_at).toLocaleString()});
        if (modelBlocked()) modelDetails.textContent += ' · ' + errorText(model.unavailable_code || 'model_not_available');
        modelDetails.hidden = false;
    };
    pinModel.addEventListener('click', () => {
        const model = modelFor(modelSelect.value);
        const target = model?.alias_target || model?.canonical_slug;
        if (!target || modelBlocked()) return;
        if (![...modelSelect.options].some(option => option.value === target)) modelSelect.add(new Option(target, target));
        modelSelect.value = target;
        modelSelect.dispatchEvent(new Event('change', {bubbles: true}));
    });
    const updateButtons = () => {
        if (!attached()) return;
        const local = !providerMode();
        const changing = !!pendingAction || status?.setup_active || ['setting_up', 'installing', 'downloading', 'starting', 'stopping', 'discovering_devices'].includes(status?.state);
        // A failed autostart can leave Installed false even after verifying cached models.
        const canStart = status && status.state !== 'not_installed' && status.code !== 'model_not_installed'
            && (status.code !== 'runtime_artifact_missing' || status.installed || installationCheck?.runtime_available);
        const idle = status && (['disabled', 'not_installed', 'installed', 'stopped'].includes(status.state)
            || (status.code === 'runtime_artifact_missing' && !status.installed));
        saveNotice.hidden = local || !dirty();
        for (const [action, button] of Object.entries(buttons)) {
            button.hidden = !local;
            button.disabled = action === 'stop'
                ? (busy && !['test', 'install', 'start', 'setup'].includes(activeAction)) || (!busy && !changing && idle)
                : busy || changing || (action !== 'setup' && localDirty()) || (!['check', 'setup'].includes(action) && !status?.enabled)
                    || (action === 'start' && (!canStart || localReady()))
                    || (action === 'test' && !localReady());
            button.className = 'btn-secondary';
        }
        const next = localReady() ? 'test' : 'setup';
        buttons[next].className = 'btn-primary';
        buttons.stop.textContent = changing ? tr('setup_cancel') : t('config.local_llm.stop');
        buttons.setup.hidden = !local || localReady();
        buttons.test.hidden = !local || !localReady();
        root.querySelectorAll('.toggle-wrap .toggle').forEach(control => {
            const label = control.parentElement.querySelector('.toggle-label');
            if (label) label.textContent = t(control.classList.contains('on') ? 'config.toggle.active' : 'config.toggle.inactive');
        });
        localResult.hidden = !local || !localResult.textContent || localResult.textContent === statusText.textContent;
        localHelp.hidden = !local || !status?.installed;
        progress.hidden = !local || !(status?.setup_active || ['setting_up', 'installing', 'downloading', 'starting', 'discovering_devices'].includes(status?.state));
        phaseText.hidden = progress.hidden;
        deviceText.hidden = local && !status?.device;
        diagnostics.hidden = !local;
        cpuText.hidden = !local || !status?.cpu_fallback_active;
        providerPanel.hidden = !providerMode();
        localPanel.hidden = providerMode();
        externalTest.disabled = busy || dirty() || !status?.enabled || !selectedProvider() || !modelSelect.value || modelBlocked();
        externalTest.title = modelBlocked() ? errorText(status?.model_availability?.code || modelFor(modelSelect.value)?.unavailable_code || 'model_not_available') : '';
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
            lifecycle_busy: 'lifecycle_busy', runtime_start_timeout: 'runtime_start_timeout', runtime_invalid_configuration: 'runtime_invalid_configuration',
            runtime_release_unavailable: 'release_unavailable', runtime_download_failed: 'runtime_download_failed', model_download_failed: 'runtime_download_failed',
            insufficient_disk_space: 'disk_full', disk_space_unavailable: 'disk_unknown', configuration_save_failed: 'save_failed',
            runtime_dependencies_missing: 'runtime_dependencies_missing', runtime_discovery_timeout: 'runtime_discovery_timeout', runtime_discovery_output_limit: 'runtime_discovery_output_limit'
        };
        return tr(exact[code] || 'failed');
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
            provider_error: 'external_provider_failed', model_not_available: 'model_not_available', model_context_unverified: 'model_context_unverified', model_unverified: 'model_unverified', image_limit_exceeded: 'image_limit_exceeded', image_invalid: 'image_invalid', image_minimum_unmet: 'image_minimum_unmet'
        };
        return tr(keys[code] || 'failed');
    };
    const paint = () => {
        if (!attached() || !status) return;
        if (providerMode()) {
            const availability = status.model_availability;
            const state = availability && availability.state !== 'usable' && status.configured ? errorText(availability.code || 'model_unverified') : status.ready ? tr('provider_ready') : status.state === 'error' ? errorText(status.last_error || status.code) : status.configured ? tr('provider_configured') : tr('provider_needs_setup');
            statusText.textContent = state;
            statusText.className = 'cfg-note-banner ' + (status.ready ? 'cfg-note-banner-success' : status.state === 'error' ? 'cfg-note-banner-warning' : '');
            deviceText.textContent = status.last_success ? tr('provider_last_success', {value: new Date(status.last_success).toLocaleString()}) : tr('provider_never_tested');
            if (status.model_revision) deviceText.textContent += ' · ' + tr('responding_model', {value: status.model_revision});
            if (status.alias_changed) deviceText.textContent += ' · ' + tr('alias_changed');
            showModelDetails();
            cpuText.hidden = true;
        } else {
            const stateKeys = {disabled: 'disabled', stopped: 'stopped', installing: 'downloading', downloading: 'downloading', starting: 'starting', discovering_devices: 'starting', blocked: 'error', error: 'error', stopped_with_error: 'error'};
            const checkExplainsError = !checkResult.hidden && installationCheck && (
                (installationCheck.code && installationCheck.code === status.code)
                || (installationCheck.runtime_available && ['runtime_artifact_missing', 'runtime_unavailable'].includes(status.code)));
            const state = localReady() ? tr('ready') : status.setup_active ? tr('action_pending') : status.state === 'installed' ? tr('installed') : checkExplainsError ? tr(status.installed ? 'installed' : 'not_installed') : reason(status.code) || (stateKeys[status.state] ? t('config.local_llm.state_' + stateKeys[status.state]) : tr('not_installed'));
            statusText.textContent = state;
            errorCode.textContent = checkExplainsError ? installationCheck.code || '' : status.code || '';
            if (status.code === 'cpu_consent_required') advanced.open = true;
            statusText.className = 'cfg-note-banner ' + (localReady() || status.state === 'installed' ? 'cfg-note-banner-success' : status.code ? 'cfg-note-banner-warning' : '');
            const mode = {gpu: 'GPU', cpu: 'CPU', mixed: 'GPU + CPU'}[status.execution_mode] || '';
            deviceText.textContent = tr('device') + ': ' + (status.device || '—') + (status.backend ? ' · ' + status.backend : '') + ' · ' + (status.deployment || '—') + (mode ? ' · ' + mode : '');
            cpuText.hidden = !status.cpu_fallback_active;
            const fallbackReason = status.cpu_fallback_reason;
            cpuText.textContent = tr('cpu_active') + (fallbackReason ? ' ' + tr(fallbackReason === 'gpu_accelerator_partial' ? 'cpu_partial_reason' : 'cpu_unavailable_reason') : '');
            if (Number(status.progress?.total_bytes) > 0) progress.value = Math.max(0, Math.min(100, Number(status.progress?.percent) || 0));
            else progress.removeAttribute('value');
            const phases = {downloading_runtime: 'downloading_runtime', testing_runtime: 'testing_runtime', downloading_model: 'downloading', downloading_projector: 'downloading', verifying_runtime: 'verifying_runtime', verifying_model: 'verifying_model', verified_model: 'verifying_model', verifying_projector: 'verifying_projector', verified_projector: 'verifying_projector', loading_model: 'loading_model', loading_projector: 'loading_projector', warming_up: 'warming_up'};
            phaseText.textContent = phases[status.progress?.phase] ? tr(phases[status.progress.phase]) : statusText.textContent;
            if (Number(status.progress?.total_bytes) > 0) {
                const mb = bytes => (Math.max(0, Number(bytes) || 0) / 1000000).toLocaleString(undefined, {maximumFractionDigits: 0});
                phaseText.textContent += ' · ' + mb(status.progress.current_bytes) + ' / ' + mb(status.progress.total_bytes) + ' MB · ' + Math.round(progress.value) + '%';
            }
            if (pendingAction && pendingAction.revision === actionRevision) {
                let outcome = '';
                if (pendingAction.action === 'install' && status.state === 'installed') outcome = tr('installed');
                else if (['start', 'setup'].includes(pendingAction.action) && status.ready) outcome = tr('ready');
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
            if (attached() && statusRequest === request) { statusText.textContent = tr('status_unavailable'); status = null; updateButtons(); }
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
            const saved = modelSelect.value || data.model || '';
            modelSelect.replaceChildren(new Option(tr('model_select'), ''));
            for (const model of loadedModels) {
                const blocked = model.usable === false || Number(model.context_length) <= 0;
                const option = new Option((model.name && model.name !== model.id ? model.name + ' · ' : '') + model.id + (blocked ? ' · ' + errorText(model.unavailable_code || 'model_context_unverified') : ''), model.id);
                option.disabled = blocked;
                modelSelect.add(option);
            }
            if (saved && !loadedModels.some(model => String(model.id) === saved)) {
                const known = loadedModels.find(model => [model.canonical_slug, model.alias_target].includes(saved));
                const option = new Option(known ? saved : tr('model_saved_unavailable', {value: saved}), saved);
                option.disabled = !known || known.usable === false;
                option.dataset.preservedModel = 'true';
                modelSelect.add(option);
            }
            modelSelect.value = saved;
            if (status?.model === saved && status?.provider === id) {
                const model = loadedModels.find(model => [model.id, model.canonical_slug, model.alias_target].includes(saved));
                status.model_availability = {state: model && model.usable !== false && Number(model.context_length) > 0 ? 'usable' : 'blocked', code: model?.unavailable_code || 'model_not_available', model};
            }
            catalogResult.textContent = tr(loadedModels.length ? 'models_loaded' : 'models_empty', {count: loadedModels.length});
            showModelDetails();
        } catch (error) {
            if (attached() && modelsRequest === request) catalogResult.textContent = errorText(error.message || 'external_provider_failed');
        } finally {
            clearTimeout(timeout);
            if (attached() && modelsRequest === request) updateButtons();
        }
    };
    const run = async action => {
        if (!attached() || (busy && !(action === 'stop' && ['test', 'install', 'start', 'setup'].includes(activeAction)))) return;
        const useProvider = providerMode();
        if (action === 'stop') { activeRequest?.abort(); pendingAction = null; }
        if (action !== 'check') checkResult.hidden = true;
        const actionResult = useProvider ? result : action === 'check' ? checkResult : localResult;
        if (!['stop', 'setup'].includes(action) && (useProvider ? dirty() : localDirty())) { actionResult.textContent = t('config.local_llm.save_first'); return; }
        const setupDraft = action === 'setup' ? {enabled: true, backend: 'local', deployment: deployment.value,
            cpu_fallback_allowed: root.querySelector('[data-path="decisions.cpu_fallback_allowed"]')?.classList.contains('on') === true} : null;
        if (setupDraft) {
            const enabled = root.querySelector('[data-path="decisions.enabled"]');
            if (enabled) { enabled.classList.add('on'); enabled.setAttribute('aria-checked', 'true'); }
            window.AuraConfigState?.set('decisions.enabled', true);
        }
        let revision = actionRevision;
        if (useProvider && action === 'test') {
            if (!status?.enabled || !selectedProvider() || !modelSelect.value || modelBlocked()) return;
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
        actionResult.hidden = false;
        if (action === 'check') {
            installationCheck = null;
            checkResult.className = 'cfg-note-banner';
            localResult.textContent = '';
        }
        const request = new AbortController();
        activeRequest = request;
        const abort = () => request.abort();
        lifetime.signal.addEventListener('abort', abort, {once: true});
        const timeout = setTimeout(abort, 125000);
        try {
            if (activeOperation) activeOperation.requestStarted = true;
            const response = await fetch('/api/decisions/' + (useProvider ? 'test' : action), {method: 'POST', credentials: 'same-origin', signal: request.signal,
                ...(setupDraft ? {headers: {'Content-Type': 'application/json'}, body: JSON.stringify({deployment: setupDraft.deployment, cpu_fallback_allowed: setupDraft.cpu_fallback_allowed})} : {})});
            const body = await response.json();
            if (!attached() || revision !== actionRevision) return;
            if (response.ok) {
                if (setupDraft && body.decisions) {
                    configData.decisions = body.decisions;
                    window.AuraConfigState?.commitSectionSent('decisions', body.decisions, setupDraft);
                }
                if (!useProvider && action === 'check') {
                    installationCheck = body;
                    const mode = body.deployment === 'docker' ? 'Docker' : tr('native') + (body.target ? ' · ' + body.target : '');
                    checkResult.textContent = mode + ': ' + (body.runtime_available ? tr('check_ready') : reason(body.code));
                    checkResult.className = 'cfg-note-banner ' + (body.runtime_available ? 'cfg-note-banner-success' : 'cfg-note-banner-warning');
                    checkResult.hidden = false;
                    localResult.textContent = '';
                    errorCode.textContent = body.code || '';
                    paint();
                } else if (!useProvider && response.status === 202 && ['install', 'start', 'setup'].includes(action)) {
                    pendingAction = {action, revision};
                    actionResult.textContent = tr('action_pending');
                } else actionResult.textContent = useProvider || action === 'test' ? tr('test_passed') : action === 'install' ? tr('installed') : action === 'start' ? tr('ready') : t('config.local_llm.state_stopped');
            }
            else actionResult.textContent = useProvider ? errorText(body.code || 'external_provider_failed') : reason(body.code || 'request_failed');
            if (action !== 'check') await refresh();
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
    for (const field of [backend, deployment]) field.addEventListener('change', () => {
        invalidateAction();
        installationCheck = null;
        checkResult.hidden = true;
    });
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
