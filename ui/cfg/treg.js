// Optional treg configuration. Endpoint permissions remain in the shared draft.
async function renderTregSection(section) {
    const cfg = window.AuraConfigState?.get('treg') || configData.treg || { enabled: false, readonly: true, max_call_cost_micro: 1000000, allowed_endpoints: [] };
    const tr = key => t('config.treg.' + key);
    const content = document.getElementById('content');
    const secret = cfgSecretPlaceholder(cfg.token);
    content.innerHTML = `<div class="cfg-section active" id="treg-section">
        <div class="section-header">${escapeHtml(section.label)}</div><div class="section-desc">${escapeHtml(section.desc)}</div>
        ${AuraConfigForm.toggle({ path: 'treg.enabled', value: cfg.enabled === true, label: tr('enabled') })}
        ${AuraConfigForm.toggle({ path: 'treg.readonly', value: cfg.readonly !== false, label: tr('readonly') })}
        ${cfgStatusBanner.html('treg-local-status', 'checking', { detail: tr('loading') })}
        <div class="field-group"><label for="treg-cost">${tr('cost')}</label>
            <input id="treg-cost" class="field-input" inputmode="decimal" value="${(Number(cfg.max_call_cost_micro ?? 1000000) / 1000000).toFixed(6)}" aria-describedby="treg-cost-help">
            <input type="hidden" data-path="treg.max_call_cost_micro" data-type="number" value="${Number(cfg.max_call_cost_micro ?? 1000000)}">
            <p class="field-help" id="treg-cost-help">${tr('cost_help')}</p><p id="treg-cost-error" role="alert"></p></div>
        <div class="field-group"><label for="treg-token">${tr('token')}</label>
            <input id="treg-token" class="field-input" type="password" autocomplete="new-password" data-path="treg.token" value="${escapeAttr(cfgSecretValue(cfg.token))}" placeholder="${escapeAttr(secret)}">
            <div class="cfg-actions-row"><button type="button" class="btn-secondary" id="treg-test-btn" data-config-action-draft="true">${tr('test')}</button>${cfgActionResult.html('treg-action-status')}</div></div>
        <div class="field-group"><div class="field-group-title">${tr('grants')}</div><p class="field-help">${tr('draft_hint')}</p>
            <textarea hidden data-path="treg.allowed_endpoints" data-type="json">${escapeHtml(JSON.stringify(cfg.allowed_endpoints || []))}</textarea>
            <div id="treg-grants"></div></div>
        <div class="field-group"><label for="treg-query">${tr('search')}</label><div class="cfg-actions-row">
            <input id="treg-query" class="field-input" maxlength="500"><button type="button" class="btn-save" id="treg-search">${tr('search')}</button></div>
            ${cfgActionResult.html('treg-search-status')}<div id="treg-results"></div><div id="treg-details"></div></div>
    </div>`;
    const root = document.getElementById('treg-section');
    const grantsInput = root.querySelector('[data-path="treg.allowed_endpoints"]');
    const grants = () => JSON.parse(grantsInput.value);
    const changeGrants = value => { grantsInput.value = JSON.stringify(value); setDirty(true); renderGrants(); };
    function renderGrants() {
        const list = root.querySelector('#treg-grants');
        list.replaceChildren();
        if (!grants().length) list.textContent = tr('empty');
        grants().forEach(grant => {
            const row = document.createElement('div'); row.className = 'cfg-actions-row';
            const label = document.createElement('span'); label.style.overflowWrap = 'anywhere';
            label.textContent = `${grant.endpoint_id} · ${tr(grant.operation)} · ${grant.method} ${grant.path}`;
            const button = document.createElement('button'); button.type = 'button'; button.className = 'btn-secondary';
            button.textContent = tr('revoke'); button.setAttribute('aria-label', tr('revoke') + ' ' + grant.endpoint_id);
            button.addEventListener('click', () => changeGrants(grants().filter(item => item.endpoint_id !== grant.endpoint_id)));
            row.append(label, button); list.append(row);
        });
    }
    renderGrants();
    root.querySelector('#treg-cost').addEventListener('input', event => {
        const value = event.target.value.trim();
        const match = /^(\d+)(?:\.(\d{0,6}))?$/.exec(value);
        const micro = match ? Number(BigInt(match[1]) * 1000000n + BigInt((match[2] || '').padEnd(6, '0'))) : -1;
        const valid = Number.isSafeInteger(micro) && micro >= 0 && micro <= 1000000000000;
        event.target.setCustomValidity(valid ? '' : tr('cost_invalid'));
        event.target.setAttribute('aria-invalid', String(!valid));
        root.querySelector('#treg-cost-error').textContent = valid ? '' : tr('cost_invalid');
        root.querySelector('[data-path="treg.max_call_cost_micro"]').value = valid ? micro : -1;
        setDirty(true);
    });
    async function request(path, options) {
        const response = await fetch('/api/treg/' + path, { cache: 'no-store', signal: AbortSignal.timeout(95000), ...options });
        const payload = await response.json();
        if (!response.ok) throw new Error(payload.error || payload.message || `HTTP ${response.status}`);
        return payload;
    }
    // The token is a save-dock secret (data-path); the test runs with what is typed in, the stored token stands in for an empty field.
    root.querySelector('#treg-test-btn').addEventListener('click', event => cfgActionResult(event.currentTarget, () => fetch('/api/treg/test-connection', {
        method: 'POST', cache: 'no-store', signal: AbortSignal.timeout(95000),
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['treg']) })
    }), {
        target: 'treg-action-status',
        pending: tr('loading'),
        success: outcome => {
            const micro = outcome.body && outcome.body.data ? outcome.body.data.balance_micro : null;
            return tr('success') + ' · ' + tr('balance') + ': ' + (micro == null ? '?' : (micro / 1000000).toFixed(6)) + ' USD';
        },
        failure: tr('error') + ':'
    }));
    let sequence = 0;
    async function showDetails(id) {
        const current = ++sequence;
        const status = root.querySelector('#treg-search-status'); status.textContent = tr('loading');
        try {
            const payload = await request('endpoint?id=' + encodeURIComponent(id));
            if (!root.isConnected || current !== sequence) return;
            const ep = payload.data.endpoint;
            const panel = root.querySelector('#treg-details'); panel.replaceChildren();
            const heading = document.createElement('h3'); heading.textContent = ep.name || ep.id;
            const reason = document.createElement('p');
            const grant = grants().find(item => item.endpoint_id === ep.id);
            const blocked = !grant ? 'not_approved' : (grant.method !== ep.method || grant.path !== ep.path) ? 'contract_changed' : root.querySelector('[data-path="treg.readonly"]').classList.contains('on') && grant.operation !== 'read' ? 'readonly_blocked' : '';
            reason.textContent = blocked ? tr(blocked) : tr('configured');
            const description = document.createElement('p'); description.textContent = ep.summary || '';
            panel.append(heading, reason, description);
            for (const [label, value] of [['inputs', ep.input], ['pricing', ep.cost], ['requirements', payload.data.provider]]) {
                const details = document.createElement('details'); const summary = document.createElement('summary'); summary.textContent = tr(label);
                const pre = document.createElement('pre'); pre.style.whiteSpace = 'pre-wrap'; pre.style.overflowWrap = 'anywhere'; pre.textContent = JSON.stringify(value || {}, null, 2);
                details.append(summary, pre); panel.append(details);
            }
            const select = document.createElement('select'); select.className = 'field-select'; select.id = 'treg-permission'; select.setAttribute('aria-label', tr('choose'));
            for (const key of ['', 'read', 'create', 'update', 'delete']) { const option = document.createElement('option'); option.value = key; option.textContent = tr(key || 'choose'); select.append(option); }
            const approve = document.createElement('button'); approve.type = 'button'; approve.className = 'btn-save'; approve.id = 'treg-approve'; approve.textContent = tr('approve'); approve.disabled = true;
            select.addEventListener('change', () => { approve.disabled = !select.value; });
            approve.addEventListener('click', () => {
                if (!select.value) return;
                changeGrants([...grants().filter(item => item.endpoint_id !== ep.id), { endpoint_id: ep.id, method: ep.method, path: ep.path, operation: select.value }]);
                reason.textContent = tr('draft_hint');
            });
            const actions = document.createElement('div');
            actions.className = 'cfg-actions-row';
            actions.append(select, approve);
            panel.append(actions); status.textContent = '';
        } catch (error) { if (root.isConnected && current === sequence) status.textContent = tr('error') + ': ' + error.message; }
    }
    root.querySelector('#treg-search').addEventListener('click', event => cfgActionResult(event.currentTarget, async () => {
        const current = ++sequence;
        const payload = await request('catalog?q=' + encodeURIComponent(root.querySelector('#treg-query').value) + '&limit=20');
        if (!root.isConnected || current !== sequence) return { status: 'ok', rows: -1 };
        const list = root.querySelector('#treg-results'); list.replaceChildren(); root.querySelector('#treg-details').replaceChildren();
        const rows = payload.data.results || [];
        rows.forEach(ep => {
            const button = document.createElement('button'); button.type = 'button'; button.className = 'btn-secondary'; button.style.overflowWrap = 'anywhere';
            button.textContent = ep.id + ' · ' + (ep.name || ep.summary || ''); button.addEventListener('click', () => showDetails(ep.id)); list.append(button);
        });
        return { status: 'ok', rows: rows.length };
    }, {
        target: 'treg-search-status',
        pending: tr('loading'),
        failure: tr('error') + ':',
        // A search with hits needs no confirmation line; an empty one says so.
        onSuccess: outcome => cfgActionResult.show('treg-search-status', outcome.body.rows === 0 ? 'warning' : '', outcome.body.rows === 0 ? tr('empty') : '')
    }));
    root.querySelector('#treg-query').addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); root.querySelector('#treg-search').click(); } });
    attachChangeListeners();
    const localStates = { disabled: 'off', network_disabled: 'degraded', needs_setup: 'unconfigured', configured: 'unknown' };
    await cfgStatusBanner.check('treg-local-status', () => fetch('/api/treg/status', { cache: 'no-store' }),
        body => ({ state: localStates[body && body.status] || 'unknown', detail: tr(body && body.status ? body.status : 'error') }));
}
