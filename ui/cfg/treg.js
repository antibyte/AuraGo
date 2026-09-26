// Optional treg configuration. Endpoint permissions remain in the shared draft.
async function renderTregSection(section) {
    const cfg = window.AuraConfigState?.get('treg') || configData.treg || { enabled: false, readonly: true, max_call_cost_micro: 1000000, allowed_endpoints: [] };
    const tr = key => t('config.treg.' + key);
    const content = document.getElementById('content');
    const secret = cfgSecretPlaceholder(cfg.token);
    content.innerHTML = `<div class="cfg-section active" id="treg-section">
        <div class="section-header">${escapeHtml(section.label)}</div><div class="section-desc">${escapeHtml(section.desc)}</div>
        <div class="field-group"><div class="field-grid two-cols">
            <div class="cfg-toggle-row-compact"><button type="button" class="toggle ${cfg.enabled ? 'on' : ''}" data-path="treg.enabled" aria-label="${tr('enabled')}"></button><span>${tr('enabled')}</span></div>
            <div class="cfg-toggle-row-compact"><button type="button" class="toggle ${cfg.readonly !== false ? 'on' : ''}" data-path="treg.readonly" aria-label="${tr('readonly')}"></button><span>${tr('readonly')}</span></div>
        </div><p id="treg-local-status" role="status">${tr('loading')}</p></div>
        <div class="field-group"><label for="treg-cost">${tr('cost')}</label>
            <input id="treg-cost" class="field-input" inputmode="decimal" value="${(Number(cfg.max_call_cost_micro ?? 1000000) / 1000000).toFixed(6)}" aria-describedby="treg-cost-help">
            <input type="hidden" data-path="treg.max_call_cost_micro" data-type="number" value="${Number(cfg.max_call_cost_micro ?? 1000000)}">
            <p class="field-help" id="treg-cost-help">${tr('cost_help')}</p><p id="treg-cost-error" role="alert"></p></div>
        <div class="field-group"><label for="treg-token">${tr('token')}</label>
            <input id="treg-token" class="field-input" type="password" autocomplete="new-password" placeholder="${escapeAttr(secret)}">
            <div class="cfg-actions-row"><button type="button" class="btn-save" id="treg-save-token">${tr('save_token')}</button>
            <button type="button" class="btn-save" id="treg-test-btn">${tr('test')}</button></div>
            <p class="field-help">${tr('test_help')}</p><p id="treg-action-status" role="status" aria-live="polite"></p></div>
        <div class="field-group"><div class="field-group-title">${tr('grants')}</div><p class="field-help">${tr('draft_hint')}</p>
            <textarea hidden data-path="treg.allowed_endpoints" data-type="json">${escapeHtml(JSON.stringify(cfg.allowed_endpoints || []))}</textarea>
            <div id="treg-grants"></div></div>
        <div class="field-group"><label for="treg-query">${tr('search')}</label><div class="cfg-actions-row">
            <input id="treg-query" class="field-input" maxlength="500"><button type="button" class="btn-save" id="treg-search">${tr('search')}</button></div>
            <p id="treg-search-status" role="status" aria-live="polite"></p><div id="treg-results"></div><div id="treg-details"></div></div>
    </div>`;
    const root = document.getElementById('treg-section');
    root.querySelectorAll('.toggle').forEach(button => button.addEventListener('click', () => toggleBool(button)));
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
    async function action(button, status, work) {
        button.disabled = true; status.textContent = tr('loading');
        try { await work(); } catch (error) { if (root.isConnected) status.textContent = tr('error') + ': ' + error.message; }
        finally { if (root.isConnected) button.disabled = false; }
    }
    root.querySelector('#treg-save-token').addEventListener('click', event => action(event.currentTarget, root.querySelector('#treg-action-status'), async () => {
        const input = root.querySelector('#treg-token');
        if (!input.value.trim()) throw new Error(tr('token_required'));
        const response = await fetch('/api/vault/secrets', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ key: 'treg_token', value: input.value.trim() }) });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        cfgMarkSecretStored(input, 'treg.token');
        window.AuraConfigState?.markSaved('treg.token', configData.treg.token);
        root.querySelector('#treg-action-status').textContent = tr('success');
    }));
    root.querySelector('#treg-test-btn').addEventListener('click', event => action(event.currentTarget, root.querySelector('#treg-action-status'), async () => {
        const result = await request('test-connection', { method: 'POST' });
        if (root.isConnected) root.querySelector('#treg-action-status').textContent = tr('success') + ' · ' + tr('balance') + ': ' + (result.data.balance_micro == null ? '?' : (result.data.balance_micro / 1000000).toFixed(6)) + ' USD';
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
            panel.append(select, approve); status.textContent = '';
        } catch (error) { if (root.isConnected && current === sequence) status.textContent = tr('error') + ': ' + error.message; }
    }
    root.querySelector('#treg-search').addEventListener('click', event => action(event.currentTarget, root.querySelector('#treg-search-status'), async () => {
        const current = ++sequence;
        const payload = await request('catalog?q=' + encodeURIComponent(root.querySelector('#treg-query').value) + '&limit=20');
        if (!root.isConnected || current !== sequence) return;
        const list = root.querySelector('#treg-results'); list.replaceChildren(); root.querySelector('#treg-details').replaceChildren();
        const rows = payload.data.results || [];
        root.querySelector('#treg-search-status').textContent = rows.length ? '' : tr('empty');
        rows.forEach(ep => {
            const button = document.createElement('button'); button.type = 'button'; button.className = 'btn-secondary'; button.style.overflowWrap = 'anywhere';
            button.textContent = ep.id + ' · ' + (ep.name || ep.summary || ''); button.addEventListener('click', () => showDetails(ep.id)); list.append(button);
        });
    }));
    root.querySelector('#treg-query').addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); root.querySelector('#treg-search').click(); } });
    attachChangeListeners();
    try {
        const local = await request('status');
        if (root.isConnected) root.querySelector('#treg-local-status').textContent = tr(local.status);
    } catch (_) { if (root.isConnected) root.querySelector('#treg-local-status').textContent = tr('error'); }
}
