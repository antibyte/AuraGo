// Router settings use the shared config draft; previews only read saved settings.
function renderLLMRouterSection(section) {
    const areas = ['general', 'easy', 'normal', 'complex', 'coding', 'research', 'creativity', 'security', 'writing'];
    const data = configData.llm_router || {};
    const tr = (key, vars) => t('common.llm_router.' + key, vars);
    const root = document.createElement('div');
    root.className = 'cfg-section active';
    root.dataset.routerSettings = '';
    const node = (tag, cls, text, parent = root) => {
        const el = document.createElement(tag);
        if (cls) el.className = cls;
        if (text) el.textContent = text;
        parent.appendChild(el);
        return el;
    };
    node('h2', 'section-header', section.label);
    node('p', 'section-desc', section.desc);
    const field = (parent, label, key, value, kind, min, max, help) => {
        const group = node(kind === 'checkbox' ? 'div' : 'label', 'field-group', '', parent);
        node('span', 'field-label', label, group);
        if (help) node(kind === 'checkbox' ? 'p' : 'span', 'field-help', help, group);
        const controlParent = kind === 'checkbox' ? node('div', 'toggle-wrap', '', group) : group;
        const input = node(kind === 'checkbox' ? 'div' : 'input', kind === 'checkbox' ? 'toggle' : 'field-input', '', controlParent);
        if (kind !== 'checkbox') input.type = kind;
        input.dataset.path = 'llm_router.' + key;
        if (kind === 'checkbox') {
            input.classList.toggle('on', !!value);
            input.setAttribute('role', 'switch');
            input.setAttribute('aria-label', label);
            input.setAttribute('aria-checked', String(!!value));
            input.addEventListener('click', () => toggleBool(input));
            node('span', 'toggle-label', t(value ? 'config.toggle.active' : 'config.toggle.inactive'), controlParent);
        }
        else { input.value = value; input.min = min; input.max = max; input.step = '1'; }
        return input;
    };
    const enabled = field(root, tr('enabled'), 'enabled', data.enabled, 'checkbox', 0, 0, tr('enabled_hint'));
    node('p', 'cfg-note-banner', tr('areas_hint'));
    const rows = node('div', 'field-grid two-cols');
    const modelSelects = [];
    const isChatModel = (p, model) => p?.type !== 'agnes' || !/^(?:agnes\/)?agnes-(?:image|video)/i.test((model || '').trim());
    const providers = configGeneralProviders().filter(p => p.id !== 'aurago-qwen-local'
        && !['embedding', 'embeddings', 'tts', 'asr', 'whisper', 'stability', 'ideogram', 'vision'].includes(p.type)
        && isChatModel(p, p.model));
    const option = (select, value, text) => select.add(new Option(text, value));
    for (const area of areas) {
        const target = (data.areas || {})[area] || {};
        const group = node('fieldset', 'field-group', '', rows);
        node('legend', 'field-label', tr('area_' + area), group);
        node('p', 'field-help', tr('area_help', {area: tr('area_' + area)}), group);
        const plabel = node('label', '', tr('provider'), group);
        const provider = node('select', 'field-select', '', plabel);
        provider.dataset.path = 'llm_router.areas.' + area + '.provider';
        option(provider, '', tr('default'));
        providers.forEach(p => option(provider, p.id, (p.name || p.id) + (p.model ? ' · ' + p.model : '')));
        if (target.provider && !providers.some(p => p.id === target.provider)) option(provider, target.provider, tr('missing') + ' · ' + target.provider);
        provider.value = target.provider || '';
        const mlabel = node('label', '', tr('model'), group);
        const model = node('select', 'field-select', '', mlabel);
        model.dataset.path = 'llm_router.areas.' + area + '.model';
        const updateModels = (catalog, value) => {
            const p = providers.find(p => p.id === provider.value);
            model.replaceChildren();
            option(model, '', tr('provider_default') + (p?.model ? ' · ' + p.model : ''));
            const ids = new Set((catalog?.models || []).filter(m => p && m.provider === p.type && !m.catalog_only && isChatModel(p, m.id)).map(m => m.id));
            if (p?.model) ids.add(p.model);
            if (value) ids.add(value);
            [...ids].sort().forEach(id => option(model, id, id));
            model.value = value || '';
            model.disabled = !provider.value;
        };
        updateModels(null, target.model);
        modelSelects.push({ updateModels, model });
        provider.addEventListener('change', () => {
            updateModels(root._routerCatalog, '');
            setNestedValue(configData, model.dataset.path, '');
            setDirty(true);
        });
    }
    const decisions = field(root, tr('decisions'), 'decisions_fallback', data.decisions_fallback !== false, 'checkbox', 0, 0, tr('decisions_hint'));
    const decisionStatus = node('p', 'cfg-note-banner', t('common.loading'));
    decisionStatus.dataset.decisionStatus = '';
    const externalHint = node('p', 'field-help', tr('decisions_external_hint'));
    externalHint.hidden = configData.decisions?.backend !== 'provider';
    const settingsButton = node('button', 'btn-save btn-secondary', tr('decisions_settings'));
    settingsButton.type = 'button';
    settingsButton.addEventListener('click', () => selectSection('decisions', {scrollBehavior: 'auto'}));
    const helper = field(root, tr('helper'), 'helper_fallback', data.helper_fallback !== false, 'checkbox', 0, 0, tr('helper_hint'));
    const limits = node('div', 'field-grid two-cols');
    field(limits, tr('timeout'), 'helper_timeout_ms', data.helper_timeout_ms ?? 1500, 'number', 250, 5000, tr('timeout_hint'));
    field(limits, tr('quota'), 'helper_max_calls_per_hour', data.helper_max_calls_per_hour ?? 20, 'number', 0, 120, tr('quota_hint'));
    node('h3', '', tr('preview'));
    node('p', 'field-help', tr('preview_hint'));
    const label = node('label', 'field-group');
    node('span', 'field-label', tr('preview_text'), label);
    node('span', 'field-help', tr('preview_text_hint'), label);
    const input = node('textarea', 'field-input', '', label);
    input.maxLength = 8000;
    input.rows = 3;
    const actions = node('div', 'cfg-actions cfg-actions-row');
    const localButton = node('button', 'btn-save', tr('preview_local'), actions);
    const helperButton = node('button', 'btn-save btn-secondary', tr('preview_helper'), actions);
    const decisionButton = node('button', 'btn-save btn-secondary', tr('preview_decisions'), actions);
    localButton.type = helperButton.type = decisionButton.type = 'button';
    localButton.dataset.classifier = 'local';
    helperButton.dataset.classifier = 'helper';
    decisionButton.dataset.classifier = 'decisions';
    const result = node('p', 'cfg-note-banner');
    result.setAttribute('role', 'status');
    result.setAttribute('aria-live', 'polite');
    const cooldownStatus = node('p', 'field-help');
    cooldownStatus.dataset.routerCooldown = '';
    cooldownStatus.setAttribute('aria-live', 'polite');
    let disposed = false;
    let cooldownUntil = 0;
    let cooldownTimer;
    const timers = new Set();
    const delay = (callback, ms) => { const id = setTimeout(() => { timers.delete(id); callback(); }, ms); timers.add(id); return id; };
    const clearDelay = id => { clearTimeout(id); timers.delete(id); };
    const helperStatus = node('p', 'field-help');
    let available = false;
    let decisionsAvailable = false;
    let busy = false;
    let helperUseful = false;
    let decisionsUseful = false;
    let controller;
    let statusController;
    const lifetime = new AbortController();
    const initialTimer = delay(() => lifetime.abort(), 7000);
    const syncButtons = () => {
        if (disposed) return;
        clearDelay(cooldownTimer);
        const seconds = Math.max(0, Math.ceil((cooldownUntil - Date.now()) / 1000));
        cooldownStatus.hidden = seconds === 0;
        cooldownStatus.textContent = seconds ? tr('cooldown_wait', {seconds}) : '';
        if (seconds) cooldownTimer = delay(syncButtons, 1000);
        const quota = Number(configData.llm_router?.helper_max_calls_per_hour ?? 20);
        localButton.disabled = busy || !input.value.trim();
        helperButton.disabled = busy || seconds > 0 || !input.value.trim() || !available || !helper.classList.contains('on') || quota === 0 || !helperUseful;
        decisionButton.disabled = busy || seconds > 0 || !input.value.trim() || !decisionsAvailable || !decisions.classList.contains('on') || quota === 0 || !decisionsUseful;
    };
    const invalidatePreview = () => {
        helperUseful = decisionsUseful = false;
        controller?.abort();
        controller = null;
        busy = false;
        result.textContent = '';
        syncButtons();
    };
    root.addEventListener('input', invalidatePreview);
    root.addEventListener('change', invalidatePreview);
    helper.addEventListener('click', invalidatePreview);
    decisions.addEventListener('click', invalidatePreview);
    enabled.addEventListener('click', invalidatePreview);
    const preview = async classifier => {
        if (hasUnsavedConfigChanges()) { result.textContent = tr('save_first'); return; }
        busy = true;
        syncButtons();
        result.textContent = t('common.loading');
        controller?.abort();
        const requestController = new AbortController();
        controller = requestController;
        const text = input.value;
        const current = () => root.isConnected && controller === requestController && input.value === text && !hasUnsavedConfigChanges();
        const timer = delay(() => requestController.abort(), 7000);
        try {
            const response = await fetch('/api/llm-router/preview', { method: 'POST', credentials: 'same-origin', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({text, classifier, helper: classifier === 'helper'}), signal: requestController.signal });
            if (!response.ok) throw new Error('preview');
            const body = await response.json();
            if (root.isConnected && Number(body.decision?.retry_after_seconds) > 0) {
                cooldownUntil = Math.max(cooldownUntil, Date.now() + body.decision.retry_after_seconds * 1000);
            }
            if (!current()) return;
            helperUseful = !!body.decision?.helper_useful && body.decision?.source === 'default';
            decisionsUseful = !!body.decision?.decisions_useful && body.decision?.source === 'default';
            result.textContent = body.enabled && body.decision ? window.AuraLLMRouteLabel(body.decision) : t('common.disabled');
        } catch (error) {
            if (current()) result.textContent = t('common.error');
        } finally {
            clearDelay(timer);
            if (controller === requestController) { controller = null; busy = false; syncButtons(); }
        }
    };
    localButton.addEventListener('click', () => preview('local'));
    helperButton.addEventListener('click', () => preview('helper'));
    decisionButton.addEventListener('click', () => preview('decisions'));
    document.getElementById('content').replaceChildren(root);
    attachChangeListeners();
    syncButtons();
    const refreshStatus = () => {
        if (!root.isConnected) return;
        statusController?.abort();
        const active = new AbortController();
        statusController = active;
        const timer = delay(() => active.abort(), 7000);
        fetch('/api/llm-router/status', {credentials: 'same-origin', signal: active.signal}).then(r => { if (!r.ok) throw new Error('status'); return r.json(); }).then(status => {
            if (!root.isConnected || statusController !== active) return;
            available = !!status.helper_available;
            decisionsAvailable = !!status.decisions?.available;
            const info = status.decisions || {};
            const reasons = {service_disabled: 'decisions_disabled', router_disabled: 'decisions_disabled',
                provider_not_configured: 'decisions_not_configured', service_not_ready: 'decisions_not_ready', configuration_changed: 'decisions_changed'};
            const code = reasons[info.code] || 'decisions_unavailable';
            decisionStatus.textContent = (decisionsAvailable ? tr('decisions_available') : tr(code)) +
                (info.model ? ' · ' + (info.provider ? info.provider + ' / ' : '') + info.model : '');
            externalHint.hidden = info.backend !== 'provider';
            helperStatus.textContent = available ? '' : tr('helper_unavailable');
            syncButtons();
        }).catch(() => {
            if (root.isConnected && statusController === active) {
                available = decisionsAvailable = false;
                helperStatus.textContent = decisionStatus.textContent = t('common.error');
                syncButtons();
            }
        }).finally(() => clearDelay(timer));
    };
    const onSaved = () => { invalidatePreview(); refreshStatus(); };
    document.addEventListener('aurago:config-saved', onSaved);
    refreshStatus();
    fetch('/api/models/catalog', {credentials: 'same-origin', signal: lifetime.signal}).then(r => r.ok ? r.json() : null).then(catalog => {
        if (!root.isConnected || !catalog) return;
        root._routerCatalog = catalog;
        modelSelects.forEach(({updateModels, model}) => updateModels(catalog, model.value));
    }).catch(() => {});
    // The observer also covers section switches and a closing embedded config panel.
    const observer = new MutationObserver(() => {
        if (!root.isConnected) {
            disposed = true;
            timers.forEach(clearTimeout);
            timers.clear();
            clearDelay(initialTimer);
            lifetime.abort();
            controller?.abort();
            statusController?.abort();
            document.removeEventListener('aurago:config-saved', onSaved);
            observer.disconnect();
        }
    });
    observer.observe(document.body, {childList: true, subtree: true});
}
