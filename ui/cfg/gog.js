// Guided gog setup; status reads are local and probes require an explicit click.
async function renderGogSection(section) {
    const tr = key => t('config.gog.' + key);
    const cfg = window.AuraConfigState?.get('gog') || configData.gog || { enabled: false, readonly: true, accounts: [] };
    const accounts = JSON.parse(JSON.stringify(cfg.accounts || []));
    const content = document.getElementById('content');
    content.innerHTML = `<div class="cfg-section active" id="gog-section">
      <div class="section-header">${escapeHtml(section.label)}</div><p>${tr('intro')}</p>
      ${AuraConfigForm.toggle({ path: 'gog.enabled', value: cfg.enabled === true, label: tr('enabled') })}
      ${AuraConfigForm.toggle({ path: 'gog.readonly', value: cfg.readonly !== false, label: tr('readonly') })}
      <div class="field-group"><h3>1. ${tr('install')}</h3><p id="gog-status" role="status">${tr('loading')}</p><button type="button" class="btn-save" id="gog-install">${tr('install')}</button></div>
      <div class="field-group"><h3>2. ${tr('oauth')}</h3><p>${tr('cloud_help')}</p><a href="https://console.cloud.google.com/apis/credentials" target="_blank" rel="noopener noreferrer">Google Cloud Console ↗</a><p>${tr('save_first')}</p></div>
      <textarea hidden data-path="gog.accounts" data-type="json">${escapeHtml(JSON.stringify(accounts))}</textarea>
      <div id="gog-accounts"></div><button type="button" class="btn-secondary" id="gog-add">${tr('add')}</button>
      <p role="status" aria-live="polite" id="gog-message"></p></div>`;
    const root = content.querySelector('#gog-section');
    const hidden = root.querySelector('[data-path="gog.accounts"]');
    const message = root.querySelector('#gog-message');
    let statuses = [];
    const change = () => { hidden.value = JSON.stringify(accounts); hidden.dispatchEvent(new Event('input', { bubbles: true })); setDirty(true); };
    async function request(path, body) {
        const response = await fetch('/api/gog/' + path, body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : { cache: 'no-store' });
        const value = await response.json();
        if (!response.ok || (value.status && value.status !== 'ok')) throw new Error(value.code || value.error || value.message || String(response.status));
        return value;
    }
    async function refresh() {
        const state = await request('status'); if (!root.isConnected) return;
        statuses = state.accounts || [];
        root.querySelector('#gog-status').textContent = state.version + ' · ' + tr(state.installed ? 'installed' : 'missing');
        root.querySelectorAll('[data-account-status]').forEach(el => { const state = statuses.find(s => s.account.id === el.dataset.accountStatus); el.textContent = tr(state?.connected ? 'connected' : 'disconnected'); });
    }
    async function action(button, work, needsSaved = true) {
        if (needsSaved && isDirty) { message.textContent = tr('save_first'); return; }
        button.disabled = true; message.textContent = tr('loading');
        try { await work(); if (root.isConnected) { message.textContent = tr('success'); await refresh(); } }
        catch (error) { if (root.isConnected) message.textContent = tr('error') + ' · ' + error.message; }
        finally { if (root.isConnected) button.disabled = false; }
    }
    function field(parent, label, value, update, type = 'text') {
        const wrap = document.createElement('label'); wrap.className = 'field-group'; wrap.textContent = label;
        const input = document.createElement('input'); input.className = 'field-input'; input.type = type;
        if (type === 'checkbox') input.checked = value === true; else input.value = value || '';
        input.addEventListener('change', () => { update(type === 'checkbox' ? input.checked : input.value); change(); }); wrap.append(input); parent.append(wrap); return input;
    }
    function button(parent, label, work) { const b = document.createElement('button'); b.type = 'button'; b.className = 'btn-secondary'; b.textContent = label; b.addEventListener('click', () => action(b, work)); parent.append(b); return b; }
    function renderAccounts() {
        const list = root.querySelector('#gog-accounts'); list.replaceChildren();
        accounts.forEach((account, index) => {
            account.services ||= {}; const card = document.createElement('div'); card.className = 'field-group';
            const heading = document.createElement('h3'); heading.textContent = account.name || account.id; card.append(heading);
            const status = document.createElement('p'); status.dataset.accountStatus = account.id; status.textContent = tr(statuses.find(s => s.account.id === account.id)?.connected ? 'connected' : 'disconnected'); card.append(status);
            field(card, tr('name'), account.name, v => { account.name = v; heading.textContent = v || account.id; });
            const id = document.createElement('small'); id.textContent = 'ID: ' + account.id; card.append(id);
            field(card, tr('enabled'), account.enabled, v => account.enabled = v, 'checkbox');
            field(card, tr('agent'), account.agent_access, v => account.agent_access = v, 'checkbox');
            field(card, tr('desktop'), account.desktop_access, v => account.desktop_access = v, 'checkbox');
            field(card, tr('client_id'), account.client_id, v => account.client_id = v);
            const secretLabel = document.createElement('label'); secretLabel.textContent = tr('secret'); const secret = document.createElement('input'); secret.type = 'password'; secret.autocomplete = 'new-password'; secret.className = 'field-input'; secretLabel.append(secret); card.append(secretLabel);
            button(card, tr('save_secret'), async () => { await request('client-secret', { account_id: account.id, secret: secret.value }); secret.value = ''; });
            const help = document.createElement('p'); help.textContent = tr('grants_help'); card.append(help);
            const tableWrap = document.createElement('div'); tableWrap.style.overflowX = 'auto'; const table = document.createElement('table'); table.className = 'cfg-table';
            const header = document.createElement('tr'); ['service', 'read', 'create', 'update', 'delete', 'send'].forEach(key => { const cell = document.createElement('th'); cell.scope = 'col'; cell.textContent = tr(key); header.append(cell); }); table.append(header);
            ['gmail', 'calendar', 'drive', 'docs', 'sheets', 'slides', 'contacts', 'tasks'].forEach(service => {
                const row = document.createElement('tr'); const name = document.createElement('th'); name.scope = 'row'; name.textContent = tr(service); row.append(name); account.services[service] ||= {};
                ['read', 'create', 'update', 'delete', 'send'].forEach(grant => { const cell = document.createElement('td'); if (grant !== 'send' || service === 'gmail') { const check = document.createElement('input'); check.type = 'checkbox'; check.checked = account.services[service][grant] === true; check.setAttribute('aria-label', tr(service) + ': ' + tr(grant)); check.addEventListener('change', () => { account.services[service][grant] = check.checked; change(); }); cell.append(check); } row.append(cell); }); table.append(row);
            }); tableWrap.append(table); card.append(tableWrap);
            const actions = document.createElement('div'); actions.className = 'cfg-actions-row'; card.append(actions);
            const authLink = document.createElement('a'); authLink.target = '_blank'; authLink.rel = 'noopener noreferrer'; authLink.hidden = true; authLink.textContent = tr('open_login'); card.append(authLink);
            const redirect = document.createElement('p'); redirect.style.overflowWrap = 'anywhere'; card.append(redirect);
            button(actions, tr('connect'), async () => { const result = await request('oauth/start', { account_id: account.id }); if (!root.isConnected) return; authLink.href = result.auth_url; authLink.hidden = false; redirect.textContent = tr('redirect') + ': ' + result.redirect_uri; authLink.focus(); });
            button(actions, tr('disconnect'), () => request('disconnect', { account_id: account.id }));
            const manual = document.createElement('input'); manual.className = 'field-input'; manual.placeholder = tr('callback'); manual.setAttribute('aria-label', tr('callback')); card.append(manual);
            button(card, tr('complete_login'), async () => { await request('oauth/manual', { url: manual.value }); manual.value = ''; authLink.hidden = true; });
            const service = document.createElement('select'); service.className = 'field-input'; service.setAttribute('aria-label', tr('service')); Object.keys(account.services).forEach(key => { const option = document.createElement('option'); option.value = key; option.textContent = tr(key); service.append(option); }); card.append(service);
            const object = document.createElement('input'); object.className = 'field-input'; object.placeholder = tr('test_object'); object.setAttribute('aria-label', tr('test_object')); card.append(object);
            button(card, tr('test'), () => request('test', { account_id: account.id, service: service.value, resource_id: object.value.trim() }));
            const remove = document.createElement('button'); remove.type = 'button'; remove.className = 'btn-secondary'; remove.textContent = tr('remove'); remove.addEventListener('click', () => { accounts.splice(index, 1); change(); renderAccounts(); }); card.append(remove);
            list.append(card);
        });
    }
    root.querySelector('#gog-add').addEventListener('click', () => { if (accounts.length >= 20) return; accounts.push({ id: 'g' + crypto.randomUUID().replaceAll('-', '').slice(0, 16), name: '', client_id: '', enabled: false, agent_access: false, desktop_access: false, services: {} }); change(); renderAccounts(); });
    root.querySelector('#gog-install').addEventListener('click', event => action(event.currentTarget, () => request('install', {}), false));
    attachChangeListeners(); renderAccounts();
    try { await refresh(); } catch (error) { if (root.isConnected) message.textContent = tr('error') + ' · ' + error.message; }
}
