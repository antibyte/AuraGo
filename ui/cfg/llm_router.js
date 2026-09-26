// Router settings use the shared config draft; previews only read saved settings.
function renderLLMRouterSection(section) {
    const areas = ['general', 'easy', 'normal', 'complex', 'coding', 'research', 'creativity', 'security', 'writing'];
    const data = configData.llm_router || {};
    const tr = key => t('common.llm_router.' + key);
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
    const field = (parent, label, key, value, kind, min, max) => {
        const group = node(kind === 'checkbox' ? 'div' : 'label', 'field-group', '', parent);
        node('span', 'field-label', label, group);
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
    const enabled = field(root, tr('enabled'), 'enabled', data.enabled, 'checkbox');
    node('p', 'cfg-note-banner', tr('areas_hint'));
    const rows = node('div', 'field-grid two-cols');
    const modelSelects = [];
    const providers = (providersCache || []).filter(p => p.id !== 'aurago-qwen-local' && !['embedding', 'embeddings', 'tts', 'asr', 'whisper', 'stability', 'ideogram', 'vision', 'agnes'].includes(p.type));
    const option = (select, value, text) => select.add(new Option(text, value));
    for (const area of areas) {
        const target = (data.areas || {})[area] || {};
        const group = node('fieldset', 'field-group', '', rows);
        node('legend', 'field-label', tr('area_' + area), group);
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
            const ids = new Set((catalog?.models || []).filter(m => p && m.provider === p.type && !m.catalog_only).map(m => m.id));
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
    const helper = field(root, tr('helper'), 'helper_fallback', data.helper_fallback !== false, 'checkbox');
    node('p', 'field-help', tr('helper_hint'));
    const limits = node('div', 'field-grid two-cols');
    field(limits, tr('timeout'), 'helper_timeout_ms', data.helper_timeout_ms ?? 1500, 'number', 250, 5000);
    field(limits, tr('quota'), 'helper_max_calls_per_hour', data.helper_max_calls_per_hour ?? 20, 'number', 0, 120);
    node('h3', '', tr('preview'));
    node('p', 'field-help', tr('preview_hint'));
    const label = node('label', 'field-group', tr('preview_text'));
    const input = node('textarea', 'field-textarea', '', label);
    input.maxLength = 8000;
    input.rows = 3;
    const actions = node('div', 'cfg-actions');
    const localButton = node('button', 'btn-save', tr('preview_local'), actions);
    const helperButton = node('button', 'btn-save', tr('preview_helper'), actions);
    localButton.type = helperButton.type = 'button';
    const result = node('p', 'cfg-note-banner');
    result.setAttribute('role', 'status');
    result.setAttribute('aria-live', 'polite');
    const helperStatus = node('p', 'field-help');
    let available = false;
    let busy = false;
    let helperUseful = false;
    let controller;
    const lifetime = new AbortController();
    const initialTimer = setTimeout(() => lifetime.abort(), 7000);
    const syncButtons = () => {
        localButton.disabled = busy || !input.value.trim();
        helperButton.disabled = busy || !input.value.trim() || !available || !helper.classList.contains('on') || (data.helper_max_calls_per_hour ?? 20) === 0 || !helperUseful;
    };
    input.addEventListener('input', () => { helperUseful = false; syncButtons(); });
    helper.addEventListener('click', syncButtons);
    enabled.addEventListener('click', () => { helperUseful = false; syncButtons(); });
    const preview = async useHelper => {
        if (hasUnsavedConfigChanges()) { result.textContent = tr('save_first'); return; }
        busy = true;
        syncButtons();
        result.textContent = t('common.loading');
        controller?.abort();
        controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), 7000);
        try {
            const response = await fetch('/api/llm-router/preview', { method: 'POST', credentials: 'same-origin', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({text: input.value, helper: useHelper}), signal: controller.signal });
            if (!response.ok) throw new Error('preview');
            const body = await response.json();
            if (!root.isConnected) return;
            helperUseful = !!body.decision?.helper_useful && body.decision?.source === 'default';
            result.textContent = body.enabled && body.decision ? window.AuraLLMRouteLabel(body.decision) : t('common.disabled');
        } catch (error) {
            if (root.isConnected) result.textContent = t('common.error');
        } finally { clearTimeout(timer); busy = false; syncButtons(); }
    };
    localButton.addEventListener('click', () => preview(false));
    helperButton.addEventListener('click', () => preview(true));
    document.getElementById('content').replaceChildren(root);
    attachChangeListeners();
    syncButtons();
    fetch('/api/llm-router/status', {credentials: 'same-origin', signal: lifetime.signal}).then(r => { if (!r.ok) throw new Error('status'); return r.json(); }).then(status => {
        if (!root.isConnected) return;
        available = !!status.helper_available;
        helperStatus.textContent = available ? '' : tr('helper_unavailable');
        syncButtons();
    }).catch(() => { if (root.isConnected) helperStatus.textContent = t('common.error'); });
    fetch('/api/models/catalog', {credentials: 'same-origin', signal: lifetime.signal}).then(r => r.ok ? r.json() : null).then(catalog => {
        if (!root.isConnected || !catalog) return;
        root._routerCatalog = catalog;
        modelSelects.forEach(({updateModels, model}) => updateModels(catalog, model.value));
    }).catch(() => {});
    // The observer also covers section switches and a closing embedded config panel.
    const observer = new MutationObserver(() => {
        if (!root.isConnected) { clearTimeout(initialTimer); lifetime.abort(); controller?.abort(); observer.disconnect(); }
    });
    observer.observe(document.body, {childList: true, subtree: true});
}
