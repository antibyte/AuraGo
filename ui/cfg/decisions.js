// Decisions shares the saved config lifecycle; status polling never starts inference.
function renderDecisionsSection(section) {
    const tr = key => t('config.decisions.' + key);
    const data = configData.decisions || {};
    const root = document.createElement('div');
    root.className = 'cfg-section active';
    root.dataset.decisionsSettings = '';
    const node = (tag, cls, text, parent = root) => {
        const element = document.createElement(tag);
        if (cls) element.className = cls;
        if (text) element.textContent = text;
        parent.appendChild(element);
        return element;
    };
    node('h2', 'section-header', tr('title'));
    node('p', 'section-desc', section.desc);
    node('p', 'cfg-note-banner cfg-note-banner-info', tr('purpose'));
    const toggle = (key, label) => {
        const group = node('div', 'field-group');
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
    const deploymentLabel = node('label', 'field-group');
    node('span', 'field-label', tr('deployment'), deploymentLabel);
    const deployment = node('select', 'field-select', '', deploymentLabel);
    deployment.dataset.path = 'decisions.deployment';
    [['auto', tr('auto')], ['native', tr('native')], ['docker', 'Docker']].forEach(([value, label]) => deployment.add(new Option(label, value)));
    deployment.value = data.deployment || 'auto';
    node('p', 'field-help', tr('gpu_first'));
    toggle('cpu_fallback_allowed', tr('cpu_permission'));
    node('p', 'field-help', tr('cpu_help'));
    node('p', 'field-help', tr('save_help'));
    node('h3', 'cfg-subheading', tr('status'));
    const statusText = node('p', 'cfg-note-banner', t('common.loading'));
    statusText.dataset.decisionsStatus = '';
    statusText.setAttribute('role', 'status');
    const deviceText = node('p', 'field-help');
    const cpuText = node('p', 'cfg-note-banner cfg-note-banner-warning');
    cpuText.hidden = true;
    const progress = node('progress', '');
    progress.max = 100;
    progress.hidden = true;
    progress.setAttribute('aria-label', tr('installation'));
    const actions = node('div', 'cfg-actions cfg-actions-row');
    const buttons = {};
    for (const [action, label] of [['install', t('config.local_llm.install')], ['start', t('config.local_llm.start')], ['stop', t('config.local_llm.stop')], ['test', tr('test')]]) {
        const button = node('button', 'btn-secondary', label, actions);
        button.type = 'button';
        button.dataset.decisionsAction = action;
        button.addEventListener('click', () => run(action));
        buttons[action] = button;
    }
    node('p', 'field-help', tr('install_help'));
    const result = node('p', 'field-help');
    result.dataset.decisionsResult = '';
    result.setAttribute('role', 'status');
    node('p', 'cfg-note-banner', 'Jeff-2B · Q4_K_M · ' + tr('uncalibrated'));

    const lifetime = new AbortController();
    let status, busy = false, activeAction = '', actionRevision = 0, timer, statusRequest;
    const attached = () => !lifetime.signal.aborted && root.isConnected;
    const dirty = () => typeof hasUnsavedConfigChanges === 'function' && hasUnsavedConfigChanges();
    const updateButtons = () => {
        if (!attached()) return;
        const changing = ['installing', 'downloading', 'starting', 'stopping', 'discovering_devices'].includes(status?.state);
        for (const [action, button] of Object.entries(buttons)) {
            button.disabled = action === 'stop' ? busy && activeAction !== 'test' : busy || changing || dirty() || !status?.enabled || (action === 'test' && !status?.ready);
        }
    };
    const reason = code => {
        if (!code) return '';
        const exact = {
            service_disabled: 'disabled', cpu_consent_required: 'cpu_required',
            hardware_unavailable: 'hardware_unavailable', model_not_installed: 'not_installed',
            runtime_unavailable: 'runtime_missing', runtime_artifact_missing: 'runtime_missing', service_not_ready: 'not_ready',
            docker_unavailable: 'docker_unavailable', deployment_denied: 'docker_unavailable',
            model_integrity_failed: 'integrity_failed', runtime_attestation_failed: 'attestation_failed',
            lifecycle_busy: 'lifecycle_busy'
        };
        return tr(exact[code] || 'failed') + ' (' + code + ')';
    };
    const paint = () => {
        if (!attached() || !status) return;
        const stateKeys = {disabled: 'disabled', stopped: 'stopped', installing: 'downloading', downloading: 'downloading', starting: 'starting', discovering_devices: 'starting', blocked: 'error', error: 'error', stopped_with_error: 'error'};
        const state = status.ready ? tr('ready') : reason(status.code) || (stateKeys[status.state] ? t('config.local_llm.state_' + stateKeys[status.state]) : tr('not_ready'));
        statusText.textContent = state;
        statusText.className = 'cfg-note-banner ' + (status.ready ? 'cfg-note-banner-success' : status.code ? 'cfg-note-banner-warning' : '');
        const mode = {gpu: 'GPU', cpu: 'CPU', mixed: 'GPU + CPU'}[status.execution_mode] || '';
        deviceText.textContent = tr('device') + ': ' + (status.device || '—') + (status.backend ? ' · ' + status.backend : '') + ' · ' + (status.deployment || '—') + (mode ? ' · ' + mode : '');
        cpuText.hidden = !status.cpu_fallback_active;
        const fallbackReason = status.cpu_fallback_reason;
        cpuText.textContent = tr('cpu_active') + (fallbackReason ? ' ' + tr(fallbackReason === 'gpu_accelerator_partial' ? 'cpu_partial_reason' : 'cpu_unavailable_reason') + ' (' + fallbackReason + ')' : '');
        progress.hidden = !['installing', 'downloading'].includes(status.state);
        progress.value = Math.max(0, Math.min(100, Number(status.progress?.percent) || 0));
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
        } catch (error) {
            if (attached() && statusRequest === request) { statusText.textContent = tr('failed'); status = null; updateButtons(); }
        } finally {
            clearTimeout(timeout);
            if (attached() && statusRequest === request) timer = setTimeout(refresh, status?.enabled ? 2000 : 10000);
        }
    };
    const run = async action => {
        if (!attached() || (busy && !(action === 'stop' && activeAction === 'test'))) return;
        if (action !== 'stop' && dirty()) { result.textContent = t('config.local_llm.save_first'); return; }
        const revision = ++actionRevision;
        busy = true;
        activeAction = action;
        updateButtons();
        result.textContent = t('config.local_llm.working');
        const request = new AbortController();
        const abort = () => request.abort();
        lifetime.signal.addEventListener('abort', abort, {once: true});
        const timeout = setTimeout(abort, 125000);
        try {
            const response = await fetch('/api/decisions/' + action, {method: 'POST', credentials: 'same-origin', signal: request.signal});
            const body = await response.json();
            if (!attached() || revision !== actionRevision) return;
            result.textContent = response.ok ? (action === 'test' ? tr('test_passed') : '') : reason(body.code || 'request_failed');
            await refresh();
        } catch (error) {
            if (attached() && revision === actionRevision) result.textContent = tr('failed');
        } finally {
            clearTimeout(timeout);
            lifetime.signal.removeEventListener('abort', abort);
            if (revision === actionRevision) {
                busy = false;
                activeAction = '';
                updateButtons();
            }
        }
    };
    const dispose = () => { lifetime.abort(); statusRequest?.abort(); clearTimeout(timer); observer.disconnect(); };
    const observer = new MutationObserver(() => { if (!root.isConnected) dispose(); });
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
