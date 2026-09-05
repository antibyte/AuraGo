// MeshCore settings use the shared config draft; radio actions use saved settings only.
function renderMeshCoreSection(section) {
    const form = window.AuraConfigForm;
    const draft = () => window.AuraConfigState.get('meshcore') || configData.meshcore || {};
    const data = draft();
    const tr = key => t('config.meshcore.' + key);
    const button = (action, label) => '<button type="button" class="btn-secondary" data-mesh-action="' + action + '">' + escapeHtml(tr(label)) + '</button>';
    const field = (key, options = {}) => form.field({ path: 'meshcore.' + key, label: tr(key), value: data[key] || '', ...options });
    const toggle = key => form.toggle({ path: 'meshcore.' + key, label: tr(key), value: data[key] === true });
    const list = key => form.textarea({ path: 'meshcore.' + key, label: tr(key), help: tr('keys_help'), value: (data[key] || []).join('\n') }).replace('<textarea ', '<textarea data-type="array-lines" ');
    const limits = { max_command_age_seconds: [600, 86400], future_tolerance_seconds: [120, 3600], retention_days: [7, 90], max_messages: [1000, 10000], peer_runs_per_minute: [2, 60], runs_per_minute: [12, 120] };
    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc,
        beforeHTML: '<div class="meshcore-intro">' + form.note({ text: tr('security') }) + '<div id="meshcore-status" class="pw-status" role="status" aria-live="polite"></div></div>',
        groups: [
            { titleKey: 'config.refresh.connection', fields: [toggle('enabled'),
                form.select({ path: 'meshcore.transport', label: tr('transport'), value: data.transport || 'usb', options: [{ value: 'usb', label: 'USB' }, { value: 'ble', label: 'Bluetooth / Linux' }] }),
                form.select({ path: 'meshcore.port', label: tr('port'), value: data.port || '', options: [{ value: '', label: t('config.common.loading') }, ...(data.port ? [{ value: data.port, label: data.port }] : [])] }), field('address')], content: '<div id="meshcore-device"></div>' + form.actions([{ html: button('refresh', 'refresh') + button('test', 'test') + button('confirm', 'confirm') }])
                    + '<div id="meshcore-test-status" class="pw-status" role="status" aria-live="polite" aria-busy="false" hidden></div>'
                    + '<p id="meshcore-saved-reason" class="field-help" hidden></p>'
                    + form.disclosure({ title: tr('pairing'), className: 'meshcore-pairing', content: form.note({ text: tr('ble_help') }) + form.password({ id: 'meshcore-pin', label: tr('pin') }).replace('autocomplete="off"', 'autocomplete="off" maxlength="16"') + form.actions([{ html: button('scan', 'scan') + button('pair', 'pair') }]) + '<div id="meshcore-discovery" role="region" aria-label="' + escapeHtml(tr('scan')) + '" aria-busy="false"></div>' }) },
            { title: tr('permissions'), fields: [toggle('direct_replies'), list('trusted_nodes'), toggle('proactive_send'), list('send_nodes')], content: '<div id="meshcore-contacts"></div>' },
            { title: tr('channels'), content: form.note({ text: tr('channels_help') }) + '<div id="meshcore-channels"></div>' },
            { title: tr('limits'), content: form.disclosure({ titleKey: 'config.precision.advanced_title', content: Object.entries(limits).map(([key, values]) => form.number({ path: 'meshcore.' + key, label: tr(key), value: data[key] || values[0], min: 1, max: values[1] })).join('') }) },
            { title: tr('inbox'), content: '<div id="meshcore-inbox"></div>' + form.actions([{ html: button('previous', 'previous') + button('next', 'next') }]) }
        ]
    });
    const root = document.getElementById('content').querySelector('.cfg-section');
    let runtime = null;
    let offset = 0;
    let busy = false;
    let lastPageSize = 0;
    const status = root.querySelector('#meshcore-status');
    const testStatus = root.querySelector('#meshcore-test-status');
    const portSelect = root.querySelector('select[data-path="meshcore.port"]');
    function renderPorts(ports) {
        const selected = draft().port || '';
        const found = ports === null ? null : [...new Set((Array.isArray(ports) ? ports : []).filter(port => typeof port === 'string' && port.trim()))].sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
        portSelect.replaceChildren(new Option(tr(found === null ? 'failed' : found.length ? 'select_port' : 'no_ports'), ''));
        for (const port of found || []) portSelect.add(new Option(port, port));
        if (selected && !found?.includes(selected)) portSelect.add(new Option(selected + (found ? ' (' + tr('port_unavailable') + ')' : ''), selected));
        portSelect.value = selected;
        portSelect.setAttribute('aria-busy', 'false');
    }
    function set(path, value) {
        if (path === 'trusted_nodes' || path === 'send_nodes') root.querySelector('[data-path="meshcore.' + path + '"]').value = value.join('\n');
        else if (typeof value === 'string') {
            const control = root.querySelector('[data-path="meshcore.' + path + '"]');
            if (control) control.value = value;
        }
        setNestedValue(configData, 'meshcore.' + path, value);
        window.AuraConfigState.set('meshcore.' + path, value);
    }
    function lockActions() {
        if (!root.isConnected) { document.removeEventListener('cfg:statechange', lockActions); window.removeEventListener('aurago:config-saved', saved); return; }
        const dirty = window.AuraConfigState.isDirty();
        const reason = root.querySelector('#meshcore-saved-reason');
        reason.textContent = dirty ? tr('save_first') : '';
        reason.hidden = !dirty;
        for (const action of ['test', 'pair', 'scan']) {
            root.querySelector('[data-mesh-action="' + action + '"]').disabled = busy || dirty || !runtime || (action !== 'test' && !runtime.ble_supported) || (action === 'test' && !runtime.config.enabled);
        }
        root.querySelector('[data-mesh-action="confirm"]').disabled = busy || !runtime?.status.identity_key;
        root.querySelector('[data-mesh-action="previous"]').disabled = busy || offset === 0;
        root.querySelector('[data-mesh-action="next"]').disabled = busy || lastPageSize < 25;
        root.querySelectorAll('[data-mesh-action="select_device"]').forEach(button => {
            button.disabled = busy;
            button.setAttribute('aria-pressed', String(draft().transport === 'ble' && draft().address === button.dataset.address));
        });
    }
    async function scanDevices() {
        const discovery = root.querySelector('#meshcore-discovery');
        discovery.setAttribute('aria-busy', 'true');
        discovery.innerHTML = '<div class="meshcore-search-status" role="status"><span class="spinner" aria-hidden="true"></span><span>' + escapeHtml(tr('searching')) + '</span></div>';
        try {
            const result = await request('scan', {});
            discovery.replaceChildren();
            for (const device of result.devices || []) {
                if (!/^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(device.address || '')) continue;
                const choice = document.createElement('button');
                choice.type = 'button'; choice.className = 'btn-secondary meshcore-device-choice';
                choice.dataset.meshAction = 'select_device'; choice.dataset.address = device.address.toUpperCase();
                const identity = document.createElement('span'); identity.className = 'meshcore-device-identity';
                const name = document.createElement('strong'); name.textContent = device.name || choice.dataset.address;
                identity.append(name);
                if (device.name) { const address = document.createElement('span'); address.textContent = choice.dataset.address; identity.append(address); }
                const action = document.createElement('span'); action.textContent = tr('use_device');
                choice.append(identity, action); discovery.append(choice);
            }
            if (!discovery.childElementCount) discovery.textContent = tr('empty');
            status.textContent = tr('success');
        } catch (error) {
            discovery.textContent = tr('failed');
            throw error;
        } finally { discovery.setAttribute('aria-busy', 'false'); }
    }
    async function request(action, body) {
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 25000);
        try {
            const response = await fetch('/api/meshcore/' + action, { method: body ? 'POST' : 'GET', headers: body ? { 'Content-Type': 'application/json' } : {}, body: body ? JSON.stringify(body) : undefined, signal: controller.signal });
            if (!response.ok) throw new Error('meshcore_request_failed');
            return await response.json();
        } finally { clearTimeout(timeout); }
    }
    function renderRuntime() {
        if (!root.isConnected || !runtime) return;
        const st = runtime.status;
        const known = ['disabled', 'connecting', 'connected', 'disconnected', 'binding_required', 'binding_changed', 'suspended'];
        status.textContent = tr(known.includes(st.state) ? st.state : 'failed') + ' · ' + tr('hardware_unverified');
        root.querySelector('#meshcore-device').textContent = [st.name, st.identity_key, st.firmware].filter(Boolean).join(' · ');
        const contacts = root.querySelector('#meshcore-contacts');
        contacts.replaceChildren();
        for (const contact of st.contacts || []) {
            const row = document.createElement('p');
            row.textContent = contact.name + ' · ' + contact.key;
            contacts.append(row);
        }
        const channels = root.querySelector('#meshcore-channels');
        channels.replaceChildren();
        const available = st.channels || [];
        const orphaned = (draft().channels || []).filter(rule => !available.some(channel => channel.index === rule.index));
        for (const channel of [...available, ...orphaned.map(rule => ({ index: rule.index, name: tr('binding_changed') }))]) {
            const rule = (draft().channels || []).find(r => r.index === channel.index) || { index: channel.index, mode: 'receive', prefix: '!aura' };
            const row = document.createElement('div');
            row.className = 'field-group meshcore-channel';
            const heading = document.createElement('h3'); heading.textContent = channel.index + ' · ' + channel.name;
            const binding = document.createElement('p'); binding.textContent = tr(rule.binding === channel.binding ? 'bound' : 'binding_required');
            const bind = document.createElement('button'); bind.type = 'button'; bind.className = 'btn-secondary'; bind.textContent = tr('bind_channel');
            function update(change) {
                const rules = [...(draft().channels || [])];
                const index = rules.findIndex(r => r.index === channel.index);
                const next = { ...(index >= 0 ? rules[index] : rule), ...change };
                if (index >= 0) rules[index] = next; else rules.push(next);
                set('channels', rules);
            }
            bind.disabled = !channel.binding;
            bind.addEventListener('click', () => { update({ binding: channel.binding }); binding.textContent = tr('bound'); });
            const remove = document.createElement('button'); remove.type = 'button'; remove.className = 'btn-secondary'; remove.textContent = t('config.common.delete');
            remove.addEventListener('click', () => { set('channels', (draft().channels || []).filter(rule => rule.index !== channel.index)); renderRuntime(); });
            const select = document.createElement('select'); select.className = 'field-select'; select.setAttribute('aria-label', tr('mode'));
            for (const value of ['receive', 'prefix', 'questions']) { const option = document.createElement('option'); option.value = value; option.textContent = tr(value); select.append(option); }
            select.value = rule.mode;
            select.addEventListener('change', () => update({ mode: select.value }));
            const prefix = document.createElement('input'); prefix.className = 'field-input'; prefix.maxLength = 32; prefix.value = rule.prefix || '!aura'; prefix.setAttribute('aria-label', tr('prefix'));
            prefix.addEventListener('input', () => update({ prefix: prefix.value }));
            const allow = document.createElement('label'); const check = document.createElement('input'); check.type = 'checkbox'; check.checked = rule.allow_send === true;
            check.addEventListener('change', () => update({ allow_send: check.checked }));
            allow.append(check, document.createTextNode(' ' + tr('allow_send')));
            const modeLabel = document.createElement('label'); modeLabel.textContent = tr('mode'); modeLabel.append(select);
            const prefixLabel = document.createElement('label'); prefixLabel.textContent = tr('prefix'); prefixLabel.append(prefix);
            row.append(heading, binding, bind, modeLabel, prefixLabel, allow, remove); channels.append(row);
        }
        lockActions();
    }
    async function inbox() {
        const result = await request('messages?limit=25&offset=' + offset);
        if (!root.isConnected) return;
        const box = root.querySelector('#meshcore-inbox'); box.replaceChildren();
        const states = ['pending', 'processing', 'received', 'quarantine', 'completed', 'sending', 'outcome_unknown'];
        const reviews = ['safe', 'suspicious', 'dangerous'];
        const sends = ['not_sent', 'device_accepted', 'delivered', 'outcome_unknown'];
        lastPageSize = result.messages.length;
        if (!lastPageSize) box.textContent = tr('empty');
        for (const msg of result.messages) {
            const row = document.createElement('article'); row.className = 'field-group';
            const title = document.createElement('strong'); title.textContent = new Date(msg.received_at * 1000).toLocaleString() + ' · ' + tr(states.includes(msg.state) ? msg.state : 'failed');
            const source = document.createElement('p'); source.textContent = (msg.kind === 'channel' ? tr('channels') + ' ' + msg.channel : msg.sender) + ' · ' + msg.id;
            const review = document.createElement('p'); review.textContent = tr(reviews.includes(msg.review) ? msg.review : 'unchecked') + (sends.includes(msg.send_state) ? ' · ' + tr(msg.send_state) : '');
            if (msg.reason) review.append(document.createTextNode(' · ' + msg.reason));
            const text = document.createElement('pre'); text.textContent = msg.text;
            row.append(title, source, review, text);
            if (msg.reply) { const reply = document.createElement('pre'); reply.textContent = msg.reply; row.append(reply); }
            if (msg.state === 'quarantine' || msg.state === 'received') {
                const retry = document.createElement('button'); retry.type = 'button'; retry.className = 'btn-secondary'; retry.textContent = tr('recheck'); retry.dataset.meshAction = 'recheck'; retry.dataset.id = msg.id; row.append(retry);
            }
            box.append(row);
        }
        lockActions();
    }
    async function refresh() {
        portSelect.setAttribute('aria-busy', 'true');
        const results = await Promise.allSettled([request('status'), request('devices'), inbox()]);
        if (!root.isConnected) return;
        if (results[0].status === 'fulfilled') { runtime = results[0].value; renderRuntime(); } else { runtime = null; status.textContent = tr('failed'); }
        renderPorts(results[1].status === 'fulfilled' ? results[1].value.ports || [] : null);
        if (results.some(result => result.status === 'rejected')) status.textContent = tr('failed');
        lockActions();
    }
    root.addEventListener('click', async event => {
        const target = event.target.closest('[data-mesh-action]');
        if (!target || target.disabled || busy) return;
        const action = target.dataset.meshAction;
        if (action === 'confirm') { set('identity_key', runtime.status.identity_key); status.textContent = tr('save_first'); return; }
        if (action === 'select_device') {
            set('address', target.dataset.address); set('transport', 'ble');
            status.textContent = tr('save_first'); lockActions(); return;
        }
        if (['test', 'pair', 'scan', 'recheck'].includes(action) && window.AuraConfigState.isDirty()) { status.textContent = tr('save_first'); return; }
        busy = true; lockActions();
        if (action === 'test') {
            testStatus.hidden = false;
            testStatus.setAttribute('aria-busy', 'true');
            target.setAttribute('aria-busy', 'true');
            testStatus.innerHTML = '<span class="spinner" aria-hidden="true"></span> ' + escapeHtml(t('config.common.loading'));
        }
        try {
            if (action === 'previous' || action === 'next') { offset = Math.max(0, offset + (action === 'next' ? 25 : -25)); await inbox(); }
            else if (action === 'refresh') await refresh();
            else if (action === 'scan') await scanDevices();
            else if (action === 'pair') {
                const pin = root.querySelector('#meshcore-pin'); const value = pin.value; pin.value = '';
                await request('pair', { address: runtime.config.address, pin: value }); status.textContent = tr('success');
            } else if (action === 'recheck') { await request('recheck', { id: target.dataset.id }); await inbox(); }
            else if (action === 'test') {
                const result = await request('test', {});
                runtime.status = result.status; renderRuntime();
                const state = result.status.state;
                testStatus.textContent = ['connected', 'binding_required', 'binding_changed'].includes(state)
                    ? tr('success') + ' · ' + tr(state) + (state === 'binding_required' ? ' · ' + tr('confirm') : '')
                    : tr('failed');
            }
        } catch (_) {
            status.textContent = tr('failed');
            if (action === 'test') testStatus.textContent = tr('failed');
        } finally {
            if (action === 'test') { testStatus.setAttribute('aria-busy', 'false'); target.setAttribute('aria-busy', 'false'); }
            busy = false; lockActions();
        }
    });
    function saved() { if (root.isConnected) refresh(); else lockActions(); }
    document.addEventListener('cfg:statechange', lockActions);
    window.addEventListener('aurago:config-saved', saved);
    refresh();
}
