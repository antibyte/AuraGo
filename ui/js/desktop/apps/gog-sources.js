(function () {
    'use strict';
    const base = '/api/desktop/integrations/gog/';
    const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    async function request(path, body, signal, writing = false) {
        let response, result;
        try {
            response = await fetch(base + path, { credentials: 'same-origin', cache: 'no-store', signal, ...(body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : {}) });
            result = await response.json();
            if (!result || typeof result !== 'object' || Array.isArray(result)) throw new Error('invalid_response');
        } catch (error) { error.uncertain = writing; throw error; }
        if (!response.ok || result.status === 'error' || result.status === 'external_status_unknown') {
            const error = new Error(result.code || result.error || response.statusText); error.uncertain = result.status === 'external_status_unknown' || writing && response.status >= 500; throw error;
        }
        return result.data ?? result;
    }
    function dialog(ctx, title, build, signal) {
        return new Promise(resolve => {
            const tr = key => ctx.t('desktop.gog.' + key);
            const node = document.createElement('dialog'); node.className = 'gog-dialog';
            node.innerHTML = `<form method="dialog"><h2>${esc(title)}</h2><div data-fields></div><footer><button value="cancel" type="submit">${esc(tr('cancel'))}</button><button value="ok" type="submit">${esc(tr('continue'))}</button></footer></form>`;
            const values = build(node.querySelector('[data-fields]'));
            const close = () => { node.close('cancel'); };
            const finish = () => { signal?.removeEventListener('abort', close); const result = node.returnValue === 'ok' ? values() : null; node.remove(); resolve(result); };
            node.addEventListener('close', finish, { once: true }); signal?.addEventListener('abort', close, { once: true });
            (document.fullscreenElement || document.body).append(node); node.showModal();
            if (signal?.aborted) close();
        });
    }
    function field(parent, title, value = '', type = 'text', required = false) {
        const label = document.createElement('label'); label.textContent = title;
        const input = document.createElement(type === 'textarea' ? 'textarea' : 'input');
        if (input.tagName === 'INPUT') input.type = type;
        input.setAttribute('aria-label', title); input.value = value; input.required = required; label.append(input); parent.append(label); return input;
    }
    function select(parent, title, options) {
        const label = document.createElement('label'); label.textContent = title;
        const input = document.createElement('select'); input.setAttribute('aria-label', title);
        options.forEach(([value, text]) => { const option = document.createElement('option'); option.value = value; option.textContent = text; input.append(option); });
        label.append(input); parent.append(label); return input;
    }
    function attachSources(host, service, ctx) {
        const local = host.firstElementChild, lifetime = new AbortController(), tr = key => ctx.t('desktop.gog.' + key);
        local.classList.add('gog-source-local'); host.classList.add('gog-source-host');
        const bar = document.createElement('div'); bar.className = 'gog-source-bar'; bar.hidden = true;
        const picker = select(bar, tr('source'), [['', tr('local')]]);
        const panel = document.createElement('section'); panel.className = 'gog-source-panel'; panel.hidden = true;
        panel.innerHTML = `<header><label data-container-label>${esc(tr(service === 'calendar' ? 'calendar' : 'tasklist'))}<select data-container></select></label><label data-search-label>${esc(tr('search'))}<input type="search" data-search></label><button type="button" data-refresh>${esc(tr('refresh'))}</button><button type="button" data-create>${esc(tr('create'))}</button><button type="button" data-list-create>${esc(tr('new_list'))}</button></header><p class="gog-status" role="status" aria-live="polite"></p><div class="gog-records"></div><button type="button" data-next hidden>${esc(tr('more'))}</button><div class="gog-editor"></div>`;
        host.prepend(bar); host.append(panel);
        const find = s => panel.querySelector(s), status = find('.gog-status'), records = find('.gog-records'), editor = find('.gog-editor'), container = find('[data-container]');
        find('[data-container-label]').hidden = service === 'contacts'; find('[data-search-label]').hidden = service !== 'contacts'; find('[data-list-create]').hidden = service !== 'tasks';
        let accounts = [], readonly = true, account = '', resource = '', controller, generation = 0, dirty = false, next = '', busy = false, writing = false, uncertain = false;
        const grants = () => accounts.find(a => a.id === account)?.services?.[service] || {};
        function buttons() {
            picker.disabled = writing; container.disabled = writing;
            panel.querySelectorAll('[data-refresh], [data-next], [data-search]').forEach(el => el.disabled = writing);
            records.querySelectorAll('button').forEach(el => el.disabled = busy || writing);
            find('[data-create]').disabled = busy || writing || readonly || !grants().create || uncertain; find('[data-list-create]').disabled = busy || writing || readonly || !grants().create || uncertain;
        }
        function error(e) { if (e.name !== 'AbortError' || e.uncertain) { uncertain ||= !!e.uncertain; status.textContent = tr(e.uncertain ? 'unknown' : 'error') + ' · ' + e.message; buttons(); } }
        async function call(operation, args = {}, mutation = false) { return request('call', { account_id: account, service, operation, args }, controller.signal, mutation); }
        async function discard() {
            if (writing) return false;
            if (!dirty) return true;
            return !!await dialog(ctx, tr('unsaved'), body => { const p = document.createElement('p'); p.textContent = tr('discard'); body.append(p); return () => true; }, lifetime.signal);
        }
        function begin() { controller?.abort(); controller = new AbortController(); generation++; return generation; }
        function label(item) { return item.summary || item.title || item.name || item.names?.[0]?.displayName || item.id || item.resource || item.resourceName; }
        function refs(item) { return service === 'calendar' ? { calendar_id: resource, event_id: item.id } : service === 'tasks' ? { tasklist_id: resource, task_id: item.id } : { resource_name: item.resourceName || item.resource }; }
        async function load(page = '') {
            if (writing) return;
            const token = begin(); status.textContent = tr('loading'); busy = true; buttons();
            try {
                const args = { max: 100 }; if (page) args.page = page;
                let operation = 'list';
                if (service === 'calendar') { if (!resource) { status.textContent = tr('empty'); return; } operation = 'events'; args.calendar_id = resource; }
                if (service === 'tasks') { if (!resource) { status.textContent = tr('empty'); return; } args.tasklist_id = resource; args.show_completed = true; }
                if (service === 'contacts' && find('[data-search]').value.trim()) { operation = 'search'; args.query = find('[data-search]').value.trim(); delete args.page; }
                const data = await call(operation, args); if (token !== generation) return;
                const items = data.events || data.tasks || data.contacts || [];
                records.replaceChildren();
                items.forEach(raw => { const item = raw.event || raw; const button = document.createElement('button'); button.type = 'button'; button.className = 'gog-record';
                    const title = document.createElement('strong'); title.textContent = label(item); button.append(title);
                    const detail = document.createElement('span'); detail.textContent = service === 'calendar' ? (item.start?.dateTime || item.start?.date || '') + ' — ' + (item.end?.dateTime || item.end?.date || '') : service === 'tasks' ? (item.status === 'completed' ? tr('completed') + ' · ' : '') + (item.due?.slice(0, 10) || '') : item.email || item.emailAddresses?.[0]?.value || ''; button.append(detail);
                    button.addEventListener('click', () => openEditor(item).catch(error)); records.append(button);
                });
                next = data.nextPageToken || data.nextPageTokens?.[0]?.nextPageToken || ''; find('[data-next]').hidden = !next;
                uncertain = false; status.textContent = items.length ? '' : tr('empty');
            } catch (e) { if (token === generation) error(e); }
            finally { if (token === generation) { busy = false; buttons(); } }
        }
        async function switchAccount() {
            const chosen = picker.value;
            if (!await discard()) { picker.value = account; return; }
            begin(); account = chosen; resource = ''; dirty = false; editor.replaceChildren(); records.replaceChildren(); next = ''; find('[data-next]').hidden = true;
            local.hidden = !!account; panel.hidden = !account; if (!account) return;
            status.textContent = tr('loading'); const token = generation;
            try {
                if (service !== 'contacts') {
                    let page = '', items = [], seen = new Set();
                    do { const data = await call(service === 'calendar' ? 'calendars' : 'lists', { max: 100, ...(page ? { page } : {}) }); if (token !== generation) return; items.push(...(data.calendars || data.tasklists || [])); page = data.nextPageToken || ''; if (seen.has(page) || items.length > 1000) throw new Error(tr('limit')); seen.add(page); } while (page);
                    container.replaceChildren(); items.forEach(item => { const option = document.createElement('option'); option.value = item.id; option.textContent = item.summary || item.title || item.id; container.append(option); }); resource = container.value;
                }
                await load();
            } catch (e) { if (token === generation) error(e); }
            buttons();
        }
        async function openEditor(item) {
            if (busy || writing) return;
            if (!await discard()) return;
            const token = generation;
            if (item && service === 'contacts') {
                busy = true; buttons();
                try { const data = await call('get', refs(item)); if (token !== generation) return; item = data.contact; }
                finally { if (token === generation) { busy = false; buttons(); } }
            }
            editor.replaceChildren(); dirty = false;
            const form = document.createElement('form'); form.className = 'gog-form'; const controls = {};
            const add = (name, title, value, type, required) => controls[name] = field(form, tr(title), value || '', type || 'text', required);
            if (service === 'calendar') {
                add('summary', 'title', item?.summary, 'text', true); add('from', 'start', item?.start?.dateTime || item?.start?.date, 'text', true); add('to', 'end', item?.end?.dateTime || item?.end?.date, 'text', true);
                add('description', 'notes', item?.description, 'textarea'); add('location', 'location', item?.location);
                controls.all_day = field(form, tr('all_day'), '', 'checkbox'); controls.all_day.checked = !!item?.start?.date;
                const tz = item?.start?.timeZone || Intl.DateTimeFormat().resolvedOptions().timeZone;
                const zones = [...new Set([tz, 'UTC', ...(Intl.supportedValuesOf?.('timeZone') || [])])]; controls.timezone = select(form, tr('timezone'), zones.map(v => [v, v])); controls.timezone.value = tz;
                add('rrule', 'recurrence', (item?.recurrence || []).join('\n'), 'textarea');
                if (item) { const recurring = !!item.recurringEventId || !!item.recurrence?.length; controls.scope = select(form, tr('scope'), [...(recurring ? [['', tr('choose')]] : []), ['all', tr('all')], ['single', tr('single')], ['future', tr('future')]]); controls.scope.required = true; }
            } else if (service === 'tasks') {
                add('title', 'title', item?.title, 'text', true); add('notes', 'notes', item?.notes, 'textarea'); add('due', 'due', item?.due?.slice(0, 10), 'date');
            } else {
                add('given', 'given', item?.names?.[0]?.givenName, 'text', !item); add('family', 'family', item?.names?.[0]?.familyName); add('email', 'email', item?.emailAddresses?.[0]?.value, 'email'); add('phone', 'phone', item?.phoneNumbers?.[0]?.value, 'tel'); add('org', 'organization', item?.organizations?.[0]?.name); add(item ? 'notes' : 'note', 'notes', item?.biographies?.[0]?.value, 'textarea');
            }
            const original = Object.fromEntries(Object.entries(controls).map(([key, input]) => [key, input.value]));
            const actions = document.createElement('footer'); form.append(actions);
            function button(title, action, grant) { const b = document.createElement('button'); b.type = 'button'; b.textContent = tr(title); b.disabled = readonly || !grants()[grant] || uncertain; b.addEventListener('click', () => mutate(action)); actions.append(b); return b; }
            const save = button('save', item ? 'update' : 'create', item ? 'update' : 'create'); save.type = 'submit'; save.replaceWith(save.cloneNode(true));
            if (item && service === 'tasks') button(item.status === 'completed' ? 'reopen' : 'complete', item.status === 'completed' ? 'reopen' : 'complete', 'update');
            if (item) button('delete', 'delete', 'delete');
            const close = document.createElement('button'); close.type = 'button'; close.textContent = tr('cancel'); close.addEventListener('click', async () => { if (await discard()) { editor.replaceChildren(); dirty = false; } }); actions.append(close);
            form.addEventListener('input', () => { dirty = true; }); form.addEventListener('change', () => { dirty = true; });
            form.addEventListener('submit', e => { e.preventDefault(); mutate(item ? 'update' : 'create'); });
            async function mutate(operation) {
                if (busy || writing || uncertain || (operation !== 'delete' && !form.reportValidity())) return;
                if (operation === 'delete' && !await dialog(ctx, tr('delete'), body => { const p = document.createElement('p'); p.textContent = label(item); body.append(p); return () => true; }, lifetime.signal)) return;
                const args = item ? refs(item) : service === 'calendar' ? { calendar_id: resource } : service === 'tasks' ? { tasklist_id: resource } : {};
                if (operation === 'create' || operation === 'update') Object.entries(controls).forEach(([key, input]) => {
                    if (key === 'scope' || key === 'timezone') return;
                    const value = input.type === 'checkbox' ? input.checked : input.value;
                    if (service === 'contacts' && operation === 'update' && value === original[key]) return;
                    if (key === 'rrule') { if (value.trim() || item && value !== original[key]) args.rrule = value.trim().split('\n'); }
                    else if (key === 'due') { if (value || item && value !== original[key]) args.due = value ? value + 'T00:00:00Z' : ''; }
                    else args[key] = value;
                });
                if (service === 'contacts' && operation === 'update' && Object.keys(args).length === 1) { dirty = false; editor.replaceChildren(); return; }
                if (service === 'calendar') {
                    if (item) { if (!controls.scope.value) { controls.scope.reportValidity(); return; } args.scope = controls.scope.value; if (args.scope !== 'all') args.original_start = item.originalStartTime?.dateTime || item.originalStartTime?.date || item.start?.dateTime || item.start?.date; }
                    if (operation !== 'delete') { args.start_timezone = controls.timezone.value; args.end_timezone = controls.timezone.value; }
                }
                busy = true; writing = true; buttons(); form.querySelectorAll('button').forEach(b => b.disabled = true);
                try { await call(operation, args, true); dirty = false; editor.replaceChildren(); writing = false; busy = false; await load(); }
                catch (e) { error(e); writing = false; busy = false; buttons(); form.querySelectorAll('button').forEach(b => b.disabled = true); close.disabled = false; }
            }
            editor.append(form); form.querySelector('input')?.focus();
        }
        picker.addEventListener('change', () => switchAccount().catch(error));
        container.addEventListener('change', async () => { if (!await discard()) { container.value = resource; return; } resource = container.value; dirty = false; editor.replaceChildren(); load(); });
        find('[data-refresh]').addEventListener('click', async () => { if (await discard()) { dirty = false; editor.replaceChildren(); load(); } });
        find('[data-next]').addEventListener('click', async () => { if (await discard()) { dirty = false; editor.replaceChildren(); load(next); } });
        find('[data-create]').addEventListener('click', () => openEditor(null).catch(error));
        find('[data-list-create]').addEventListener('click', async () => {
            const value = await dialog(ctx, tr('new_list'), body => { const title = field(body, tr('title'), '', 'text', true); return () => title.value; }, lifetime.signal); if (!value || busy) return;
            busy = true; writing = true; buttons(); try { await call('create_list', { title: value }, true); writing = false; busy = false; await switchAccount(); } catch (e) { error(e); } finally { writing = false; busy = false; buttons(); }
        });
        async function policy() {
            try { const data = await request('accounts', null, lifetime.signal); accounts = data.accounts.filter(a => a.services?.[service]?.read); readonly = data.readonly; picker.replaceChildren(); [['', tr('local')], ...accounts.map(a => [a.id, 'Google · ' + a.name])].forEach(([id, name]) => { const o = document.createElement('option'); o.value = id; o.textContent = name; picker.append(o); }); picker.value = account; bar.hidden = !accounts.length;
                if (account) { controller?.abort(); generation++; busy = false; editor.querySelectorAll('button').forEach(b => b.disabled = true); if (!accounts.some(a => a.id === account)) { account = ''; local.hidden = false; panel.hidden = true; editor.replaceChildren(); dirty = false; } else { status.textContent = tr('refresh'); } } buttons();
            } catch (e) { error(e); }
        }
        document.addEventListener('aurago:desktop-policy', policy); policy();
        return () => { lifetime.abort(); controller?.abort(); document.removeEventListener('aurago:desktop-policy', policy); bar.remove(); panel.remove(); local.hidden = false; host.classList.remove('gog-source-host'); };
    }

    async function office(ctx, service, mode, path, signal, providedChoice) {
        const tr = key => ctx.t('desktop.gog.' + key), state = await request('accounts', null, signal);
        const accounts = state.accounts.filter(a => a.services?.[service]?.[mode === 'import' ? 'read' : 'create'] && a.services?.drive?.[mode === 'import' ? 'read' : 'create']);
        if (!accounts.length || state.desktop_readonly || mode === 'export' && state.readonly) throw new Error(tr('no_account'));
        const choice = providedChoice || await dialog(ctx, tr(mode), body => {
            const note = document.createElement('p'); note.textContent = tr('office_help'); body.append(note);
            const account = select(body, tr('source'), accounts.map(a => [a.id, a.name]));
            const resource = mode === 'import' ? field(body, tr('resource_id'), '', 'text', true) : field(body, tr('parent'), 'root');
            return () => ({ account_id: account.value, resource_id: mode === 'import' ? resource.value.trim() : '', parent_id: mode === 'export' ? resource.value.trim() : '' });
        }, signal);
        if (!choice) return;
        if (mode === 'import') { const ext = service === 'docs' ? '.docx' : service === 'sheets' ? '.xlsx' : '.pdf'; const selected = await ctx.saveFileDialog({ title: tr('import'), defaultName: 'Google-' + crypto.randomUUID().slice(0, 8) + ext, defaultExtension: ext, filters: [{ label: ext, extensions: [ext] }] }); if (!selected) return; path = typeof selected === 'string' ? selected : selected.path; }
        return request('office/' + mode, { ...choice, service, path }, signal);
    }
    window.AuraGogDesktop = { attachSources, office };
})();
