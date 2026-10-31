(function () {
    'use strict';
    const instances = new Map();
    const domains = ['overview', 'datasets', 'models', 'notebooks', 'competitions', 'benchmarks', 'forums', 'jobs'];
    const listOperations = { datasets: 'list_datasets', models: 'list_models', notebooks: 'list_kernels', competitions: 'list_competitions', benchmarks: 'list_benchmark_tasks', forums: 'list_topics' };
    const verb = name => name.startsWith('batch_schedule') ? 'run' : name.includes('cancel') ? 'cancel' : name.split('_')[0];
    const fieldWords = { ownerSlug: 'owner', userName: 'owner', user: 'owner', owner: 'owner', slug: 'reference', datasetSlug: 'reference', modelSlug: 'reference', kernelSlug: 'reference', instanceSlug: 'variant', competitionName: 'reference', fileName: 'file', filePath: 'file', path: 'file', title: 'title', newTitle: 'title', search: 'search', searchQuery: 'search', description: 'description', subtitle: 'description', pageSize: 'limit', page: 'page', pageToken: 'page', versionNumber: 'version', datasetVersionNumber: 'version', versionLabel: 'version', isPrivate: 'private', isPublic: 'public', enableGpu: 'gpu', enableTpu: 'tpu', enableInternet: 'internet', language: 'language', files: 'files', text: 'source', forumSlug: 'forum', topicId: 'topic', id: 'reference', framework: 'framework', licenseName: 'license', contentLength: 'bytes', kernelExecutionType: 'execution', sessionTimeoutSeconds: 'timeout' };

    function render(host, windowId, ctx) {
        dispose(windowId);
        const t = key => ctx.t('kaggle.' + key);
        let disposed = false, active = 'overview', generation = 0, editor = null, status = {}, catalog = [], defs = {}, projects = [], selectedProject = '', eventSource;
        const requests = new Set();
        host.classList.add('kg-host');
        const root = document.createElement('section'); root.className = 'kg-app'; host.replaceChildren(root);
        const nav = document.createElement('nav'); nav.className = 'kg-nav'; nav.setAttribute('aria-label', t('navigation')); root.append(nav);
        const main = document.createElement('main'); main.className = 'kg-main'; root.append(main);
        const notice = document.createElement('p'); notice.className = 'kg-notice'; notice.setAttribute('role', 'status'); main.append(notice);
        const panel = document.createElement('div'); panel.className = 'kg-panel'; main.append(panel);
        function message(text) { if (!disposed) notice.textContent = text; }
        function error(err) { message(t('error') + ': ' + err.message); }
        async function api(path, options = {}) {
            const controller = new AbortController(); requests.add(controller);
            try { const init = { credentials: 'same-origin', ...options, signal: controller.signal, headers: { ...(options.body !== undefined ? { 'Content-Type': 'application/json' } : {}), ...options.headers } }; if (init.body !== undefined && !(init.body instanceof Blob)) init.body = JSON.stringify(init.body); const response = await fetch('/api/kaggle' + path, init); const body = await response.json(); if (!response.ok) throw Error(response.status === 409 ? t('conflict') : body.error || body.message || response.status); return body; }
            finally { requests.delete(controller); }
        }
        function button(parent, key, action) { const b = document.createElement('button'); b.type = 'button'; b.textContent = t(key); b.addEventListener('click', async () => { if (b.getAttribute('aria-busy') === 'true') return; b.setAttribute('aria-busy', 'true'); try { await action(); } catch (err) { const inline = b.closest('dialog')?.querySelector('[role="alert"]'); if (inline) inline.textContent = t('error') + ': ' + err.message; else error(err); } finally { b.removeAttribute('aria-busy'); b.disabled = b.dataset.blocked === 'true'; } }); parent.append(b); return b; }
        function blockButton(b, reason) { b.dataset.blocked = String(!!reason); b.disabled = !!reason; b.title = reason; }
        function title(parent, key) { const h = document.createElement('h2'); h.textContent = t(key); parent.append(h); }
        function text(parent, value, tag = 'p') { const e = document.createElement(tag); e.textContent = value; parent.append(e); return e; }
        function disposeEditor() { editor?.dispose(); editor = null; }
        function unavailable(op) {
            const policy = status.policy || {}, access = status.access || {};
            if (!status.enabled) return t('enable_first');
            if (op.grant === 'local') {
                if (status.read_only) return t('read_only');
                if (!policy.local?.enabled || !policy.local?.allow_execute || !access.write || !access.python || !access.docker) return t('local_help');
                return '';
            }
            if (op.grant === 'write') return access.write ? '' : t('policy_help');
            if (!status.configured) return t('credentials_missing');
            if (!access.network) return t('allow_internet') + ': ' + t('disabled');
            if (op.grant === 'read' || op.grant === 'cancel') return '';
            if (op.grant !== 'download' && status.read_only) return t('read_only');
            const grant = op.grant === 'execute' ? 'remote_execute' : op.grant;
            if (!policy['allow_' + grant] && !(op.operation === 'save_kernel' && policy.allow_create)) return t('allow_' + grant) + ': ' + t('disabled');
            return access.write ? '' : t('policy_help');
        }
        domains.forEach(domain => { const b = button(nav, domain, () => navigate(domain)); b.dataset.domain = domain; });
        async function navigate(domain) {
            const epoch = ++generation; active = domain; disposeEditor(); panel.replaceChildren(); message(''); nav.querySelectorAll('button').forEach(b => b.setAttribute('aria-current', b.dataset.domain === domain ? 'page' : 'false'));
            try { status = await api('/status'); if (disposed || epoch !== generation) return; if (domain === 'overview') await overview(epoch); else if (domain === 'jobs') await jobs(epoch); else await resources(domain, epoch); } catch (err) { if (epoch === generation) error(err); }
        }
        async function overview(epoch) {
            const [next, allProjects, allJobs] = await Promise.all([api('/status'), api('/projects'), api('/jobs')]); if (disposed || epoch !== generation) return; status = next; projects = allProjects;
            title(panel, 'overview');
            const stats = document.createElement('div'); stats.className = 'kg-stats'; panel.append(stats);
            const activeJobs = allJobs.filter(job => ['queued', 'running', 'remote_running', 'cancel_requested'].includes(job.status));
            const jobCard = document.createElement('div'); text(jobCard, t('active_jobs'), 'small'); text(jobCard, String(activeJobs.length), 'strong'); stats.append(jobCard);
            for (const [label, value] of [['connection', !status.enabled ? t('disabled') : status.configured ? t('configured') : t('credentials_missing')], ['permissions', status.read_only ? t('read_only') : t('granted')], ['storage', (status.storage_bytes / 1073741824).toFixed(2) + ' / ' + (status.max_storage_bytes / 1073741824).toFixed(0) + ' GiB']]) { const card = document.createElement('div'); text(card, t(label), 'small'); text(card, value, 'strong'); stats.append(card); }
            const actions = document.createElement('div'); actions.className = 'kg-toolbar'; panel.append(actions);
            blockButton(button(actions, 'test', async () => { const result = await api('/test', { method: 'POST', body: {} }); message(t('connected') + ': ' + result.account); }), unavailable({ grant: 'read' }));
            blockButton(button(actions, 'quotas', async () => details(t('quotas'), await api('/quotas'))), unavailable({ grant: 'read' }));
            const config = document.createElement('a'); config.href = '/config#kaggle'; config.target = '_blank'; config.rel = 'noopener'; config.textContent = t('settings'); actions.append(config);
            title(panel, 'projects');
            blockButton(button(panel, 'new_project', async () => { const result = await formDialog(t('new_project'), { title: { type: 'string', minLength: 1 } }, {}, '', values => api('/projects', { method: 'POST', body: values })); if (!result) return; selectedProject = result.id; await navigate('overview'); }), unavailable({ grant: 'write' }));
            const projectSelect = select(panel, projects.map(p => [p.id, p.title]), selectedProject); projectSelect.setAttribute('aria-label', t('projects')); selectedProject = projectSelect.value;
            const filePanel = document.createElement('div'); panel.append(filePanel);
            projectSelect.addEventListener('change', () => { selectedProject = projectSelect.value; showFiles(filePanel).catch(error); });
            await showFiles(filePanel);
        }
        function select(parent, choices, value) { const input = document.createElement('select'); choices.forEach(([key, name]) => { const option = document.createElement('option'); option.value = key; option.textContent = name; input.append(option); }); if (value) input.value = value; parent.append(input); return input; }
        async function showFiles(target) {
            target.replaceChildren(); if (!selectedProject) { text(target, t('no_projects')); return; }
            const project = selectedProject, files = await api('/projects/' + project + '/files'); if (disposed || selectedProject !== project || !target.isConnected) return;
            const actions = document.createElement('div'); actions.className = 'kg-toolbar'; target.append(actions);
            blockButton(button(actions, 'import', () => importFiles(project)), unavailable({ grant: 'write' }));
            blockButton(button(actions, 'new_notebook', async () => {
                const name = await formDialog(t('new_notebook'), { fileName: { type: 'string', minLength: 1 } }, {}, '', async values => {
                    let name = values.fileName.trim(); if (!name.toLowerCase().endsWith('.ipynb')) name += '.ipynb';
                    await api('/projects/' + project + '/notebooks/' + encodeURIComponent(name), { method: 'PUT', headers: { 'If-None-Match': '*' }, body: { nbformat: 4, nbformat_minor: 5, metadata: { kernelspec: { name: 'python3', display_name: 'Python 3', language: 'python' } }, cells: [{ id: crypto.randomUUID().replaceAll('-', ''), cell_type: 'code', source: '', metadata: {}, execution_count: null, outputs: [] }] } });
                    return name;
                });
                if (name) await openNotebook(project, name);
            }), unavailable({ grant: 'write' }));
            for (const file of files) { const row = document.createElement('div'); row.className = 'kg-resource'; text(row, file.name, 'strong'); text(row, (file.size / 1024).toFixed(1) + ' KiB', 'small'); if (file.name.toLowerCase().endsWith('.ipynb')) button(row, 'open', () => openNotebook(project, file.name)); if (/\.(txt|log|csv|tsv|json|md)$/i.test(file.name)) button(row, 'details', async () => { const preview = await api('/runtime', { method: 'POST', body: { operation: 'read_file', project_id: project, file: file.name } }); details(file.name, preview); }); const link = document.createElement('a'); link.href = '/api/kaggle/projects/' + project + '/files/' + encodeURIComponent(file.name); link.download = file.name; link.textContent = t('export'); row.append(link); target.append(row); }
        }
        async function importFiles(project) { const input = document.createElement('input'); input.type = 'file'; input.multiple = true; input.addEventListener('change', async () => { try { for (const file of input.files) { const response = await fetch('/api/kaggle/projects/' + project + '/files/' + encodeURIComponent(file.name), { method: 'PUT', credentials: 'same-origin', body: file }); if (!response.ok) throw Error((await response.json()).error || t('error')); } navigate('overview'); } catch (err) { error(err); } }); input.click(); }
        async function openNotebook(project, file) { const epoch = ++generation; disposeEditor(); panel.replaceChildren(); message(''); active = 'notebooks'; nav.querySelectorAll('button').forEach(b => b.setAttribute('aria-current', b.dataset.domain === active ? 'page' : 'false')); const next = await window.KaggleNotebook(panel, { api, t, project, file, windowId, notify: message, selectFiles, unavailable, isClosed: () => disposed || epoch !== generation, remote: (doc, revision) => remoteNotebook(project, file, doc, revision) }); if (disposed || epoch !== generation) next.dispose(); else editor = next; }
        async function selectFiles(project) { const files = await api('/projects/' + project + '/files'); return new Promise(resolve => { const dialog = modal(t('select_files')); files.forEach(file => { const label = document.createElement('label'); const check = document.createElement('input'); check.type = 'checkbox'; check.value = file.name; label.append(check, document.createTextNode(file.name)); dialog.body.append(label); }); text(dialog.body, t('local_files_hint')); button(dialog.actions, 'continue', () => { const values = [...dialog.body.querySelectorAll('input:checked')].map(c => c.value); dialog.close(); resolve(values); }); dialog.cancel = () => resolve(null); }); }
        async function remoteNotebook(project, file, doc, revision) {
            status = await api('/status'); const reason = unavailable({ grant: 'execute' }); if (reason) throw Error(reason);
            const values = await formDialog(t('kaggle_run'), { slug: { type: 'string' }, newTitle: { type: 'string' }, datasetDataSources: { type: 'array', items: { type: 'string' } }, competitionDataSources: { type: 'array', items: { type: 'string' } }, modelDataSources: { type: 'array', items: { type: 'string' } } }, {}, t('remote_hint'));
            if (!values) return;
            const result = await api('/operations', { method: 'POST', body: { domain: 'notebooks', operation: 'save_kernel', parameters: { ...values, text: JSON.stringify(doc), kernelType: 'notebook', language: 'python', kernelExecutionType: 'SAVE_AND_RUN_ALL', isPrivate: true, enableGpu: false, enableTpu: false, enableInternet: false } } });
            message(t('job_created') + ': ' + result.job.id);
        }
        async function resources(domain, epoch) {
            title(panel, domain); if (!status.enabled) { text(panel, t('enable_first')); return; }
            const controls = document.createElement('div'); controls.className = 'kg-toolbar'; panel.append(controls);
            const search = document.createElement('input'); search.type = 'search'; search.placeholder = t('search'); search.setAttribute('aria-label', t('search')); controls.append(search);
            const items = catalog.filter(op => op.domain === domain); const chooser = select(controls, items.map(op => [op.operation, t('verb_' + verb(op.operation)) + ' · ' + op.operation]), listOperations[domain]); chooser.setAttribute('aria-label', t('operation'));
            const filters = document.createElement('details'); filters.className = 'kg-filters'; const summary = document.createElement('summary'); summary.textContent = t('filters'); filters.append(summary); panel.append(filters);
            const fields = document.createElement('div'); fields.className = 'kg-fields'; filters.append(fields);
            let readForm;
            let executeButton;
            const reason = document.createElement('p'); reason.className = 'kg-state'; filters.before(reason);
            const searchField = domain === 'forums' ? 'searchQuery' : domain === 'benchmarks' ? 'regexFilter' : 'search';
            const drawForm = () => { const op = items.find(o => o.operation === chooser.value); fields.replaceChildren(); const properties = { ...op.schema.properties }; search.hidden = !properties[searchField]; delete properties[searchField]; filters.open = op.operation !== listOperations[domain]; readForm = schemaFields(fields, properties, {}, 0); const why = unavailable(op); reason.textContent = why; if (executeButton) blockButton(executeButton, why); };
            chooser.addEventListener('change', drawForm);
            const output = document.createElement('div'); output.className = 'kg-results'; panel.append(output);
            const execute = async (pageToken) => {
                status = await api('/status'); const why = unavailable(items.find(o => o.operation === chooser.value)); if (why) { drawForm(); throw Error(why); }
                const op = items.find(o => o.operation === chooser.value), parameters = readForm(); if (search.value && op.schema.properties[searchField]) parameters[searchField] = search.value; if (pageToken) parameters.pageToken = pageToken;
                let transfer = {};
                if (op.grant === 'download' || op.operation.includes('upload')) { transfer = await formDialog(t('destination'), { project_id: { type: 'string', enum: projects.map(p => p.id) }, file: { type: 'string' } }); if (!transfer) return; }
                message(t('loading')); const result = await api('/operations', { method: 'POST', body: { domain, operation: op.operation, parameters, ...transfer } }); if (disposed || epoch !== generation) return;
                if (result.job) { message(t('job_created') + ': ' + result.job.id); return; }
                message(''); renderResults(output, result.data, domain);
                const token = result.data?.nextPageToken || result.data?.pageToken; if (token) button(output, 'next', () => execute(token));
            };
            executeButton = button(controls, 'execute', () => execute()); drawForm(); search.addEventListener('keydown', event => { if (event.key === 'Enter' && !executeButton.disabled) execute().catch(error); });
            text(panel, t('api_fields_hint'), 'small');
        }
        function renderResults(parent, data, domain) { parent.replaceChildren(); const list = Array.isArray(data) ? data : Object.values(data || {}).find(Array.isArray); if (!list) { detailsInto(parent, data); return; } if (!list.length) { text(parent, t('empty')); return; } for (const item of list) { const row = document.createElement('div'); row.className = 'kg-resource'; text(row, item.title || item.name || item.ref || item.slug || item.id || '—', 'strong'); const description = item.subtitle || item.description || item.url || ''; text(row, String(description).slice(0, 220), 'small'); button(row, 'details', () => details(t(domain), item)); if (domain === 'notebooks') button(row, 'import_notebook', () => importNotebook(item)); parent.append(row); } }
        async function importNotebook(item) { const ref = item.ref || (item.author && item.slug ? item.author + '/' + item.slug : ''); const values = await formDialog(t('import_notebook'), { userName: { type: 'string' }, kernelSlug: { type: 'string' }, project_id: { type: 'string', enum: projects.map(p => p.id) }, fileName: { type: 'string' } }, { userName: ref.split('/')[0] || '', kernelSlug: ref.split('/')[1] || '', fileName: (ref.split('/')[1] || 'notebook') + '.ipynb' }); if (!values) return; const { project_id, fileName, ...parameters } = values; const result = await api('/operations', { method: 'POST', body: { domain: 'notebooks', operation: 'get_kernel', parameters } }); const content = JSON.parse(result.data.blob.source); await api('/projects/' + project_id + '/notebooks/' + encodeURIComponent(fileName), { method: 'PUT', headers: { 'If-None-Match': '*' }, body: content }); await openNotebook(project_id, fileName); }
        function detailsInto(parent, data) { if (!data || typeof data !== 'object') { text(parent, String(data ?? '')); return; } const list = document.createElement('dl'); for (const [key, value] of Object.entries(data)) { text(list, fieldLabel(key), 'dt'); const term = document.createElement('dd'); if (value && typeof value === 'object') { const pre = document.createElement('pre'); pre.textContent = JSON.stringify(value, null, 2); term.append(pre); } else term.textContent = String(value ?? ''); list.append(term); } parent.append(list); }
        function details(name, data) { const d = modal(name); detailsInto(d.body, data); button(d.actions, 'close', d.close); }
        function cancelUnavailable(job) {
            if (job.status !== 'remote_running') return '';
            if (!status.configured) return t('credentials_missing');
            return status.access?.network ? '' : t('allow_internet') + ': ' + t('disabled');
        }
        function resolveUnknown(job, epoch) {
            const dialog = modal(t('resolve_unknown'));
            text(dialog.body, t('resolve_unknown_help'));
            text(dialog.body, job.account + ' · ' + job.operation + ' · ' + job.id, 'small');
            const label = document.createElement('label');
            const check = document.createElement('input'); check.type = 'checkbox';
            label.append(check, document.createTextNode(t('resolve_unknown_confirm'))); dialog.body.append(label);
            const submit = button(dialog.actions, 'resolve_unknown', async () => {
                if (!check.checked) return;
                await api('/jobs/' + job.id + '/resolve-unknown', { method: 'POST', body: { confirmed_inactive: true } });
                dialog.close(); await jobs(epoch);
            });
            blockButton(submit, t('resolve_unknown_confirm'));
            check.addEventListener('change', () => blockButton(submit, check.checked ? '' : t('resolve_unknown_confirm')));
            check.focus();
        }
        async function jobs(epoch) {
            const [all, current] = await Promise.all([api('/jobs'), api('/status')]);
            if (disposed || epoch !== generation) return;
            status = current; panel.replaceChildren(); title(panel, 'jobs');
            if (!all.length) text(panel, t('empty'));
            for (const job of all) {
                const row = document.createElement('div'); row.className = 'kg-resource';
                text(row, job.operation, 'strong'); text(row, t('state_' + job.status) + ' · ' + job.id, 'small');
                if (job.error) text(row, job.error);
                if (job.progress) text(row, (job.progress / 1048576).toFixed(1) + ' MiB');
                button(row, 'details', () => details(t('jobs'), job.result || job));
                const cancelOnSite = job.domain === 'notebooks' && job.status === 'remote_running' && !job.result?.kernelSessionId;
                if (['queued', 'running', 'remote_running'].includes(job.status) && !cancelOnSite) {
                    const cancel = button(row, 'cancel', async () => {
                        status = await api('/status');
                        const reason = cancelUnavailable(job); blockButton(cancel, reason);
                        if (reason) throw Error(reason);
                        await api('/jobs/' + job.id + '/cancel', { method: 'POST', body: {} }); await jobs(epoch);
                    });
                    blockButton(cancel, cancelUnavailable(job));
                }
                if (job.status === 'unknown' && status.can_resolve_unknown) button(row, 'resolve_unknown', () => resolveUnknown(job, epoch));
                if (cancelOnSite) {
                    const link = document.createElement('a'); const ref = job.result?.ref;
                    link.href = typeof ref === 'string' && /^[a-zA-Z0-9_-]+\/[a-zA-Z0-9_-]+$/.test(ref) ? 'https://www.kaggle.com/code/' + ref : 'https://www.kaggle.com/code';
                    link.target = '_blank'; link.rel = 'noopener noreferrer'; link.textContent = t('cancel_on_site'); link.title = t('cancel_on_site'); row.append(link);
                }
                panel.append(row);
            }
        }
        function fieldLabel(key) { return fieldWords[key] ? t(fieldWords[key]) : key; }
        function schemaFields(parent, properties, initial, depth) {
            const readers = [];
            for (const [key, original] of Object.entries(properties || {})) {
                const schema = original.$ref ? defs[original.$ref.split('/').pop()] : original; if (!schema || depth > 8) continue;
                const group = document.createElement('label'); group.className = 'kg-field'; const label = document.createElement('span'); label.textContent = fieldLabel(key); group.append(label); parent.append(group);
                if (schema.type === 'object') { const details = document.createElement('details'); const summary = document.createElement('summary'); summary.textContent = t('optional'); details.append(summary); group.append(details); const read = schemaFields(details, schema.properties, initial[key] || {}, depth + 1); readers.push(() => { const value = read(); return Object.keys(value).length ? [key, value] : null; }); continue; }
                let input;
                if (schema.enum) { input = select(group, [['', t('default')], ...schema.enum.map(value => [value, key === 'project_id' ? projects.find(p => p.id === value)?.title || value : value])], initial[key]); }
                else if (schema.type === 'boolean') { input = select(group, [['', t('default')], ['true', t('yes')], ['false', t('no')]], initial[key] === undefined ? '' : String(initial[key])); }
                else { input = document.createElement(schema.type === 'array' || key === 'text' || key === 'description' ? 'textarea' : 'input'); if (schema.type === 'integer' || schema.type === 'number') input.type = 'number'; if (initial[key] !== undefined) input.value = Array.isArray(initial[key]) ? JSON.stringify(initial[key]) : String(initial[key]); group.append(input); }
                if (schema.type === 'array') { const help = document.createElement('small'); help.textContent = schema.items?.type === 'string' ? t('one_per_line') : t('structured_json'); group.append(help); }
                if (schema.minLength) { input.required = true; input.setAttribute('aria-required', 'true'); }
                readers.push(() => { const value = input.value; if (schema.minLength && value.trim().length < schema.minLength) { input.focus(); throw Error(fieldLabel(key) + ': ' + t('required')); } if (value === '') return null; if (schema.type === 'boolean') return [key, value === 'true']; if (schema.type === 'integer' || schema.type === 'number') return [key, Number(value)]; if (schema.type === 'array') { if (value.trim().startsWith('[')) return [key, JSON.parse(value)]; if (schema.items?.type === 'string') return [key, value.split('\n').map(v => v.trim()).filter(Boolean)]; throw Error(t('structured_json')); } return [key, value]; });
            }
            return () => Object.fromEntries(readers.map(read => read()).filter(Boolean));
        }
        function modal(name) {
            const returnFocus = document.activeElement;
            const dialog = document.createElement('dialog'); dialog.className = 'kg-dialog'; dialog.setAttribute('aria-label', name); const heading = document.createElement('h2'); heading.textContent = name; dialog.append(heading); const body = document.createElement('div'); body.className = 'kg-dialog-body'; dialog.append(body); const feedback = document.createElement('p'); feedback.className = 'kg-error'; feedback.setAttribute('role', 'alert'); dialog.append(feedback); const actions = document.createElement('div'); actions.className = 'kg-toolbar'; dialog.append(actions); root.append(dialog); const result = { body, actions, cancel: null, close() { dialog.close(); dialog.remove(); if (returnFocus?.isConnected) returnFocus.focus(); } }; dialog.addEventListener('cancel', event => { event.preventDefault(); result.cancel?.(); result.close(); }); button(actions, 'cancel', () => { result.cancel?.(); result.close(); }); dialog.showModal(); return result;
        }
        function formDialog(name, properties, initial = {}, hint = '', submit) { return new Promise(resolve => { const dialog = modal(name); if (hint) text(dialog.body, hint); const read = schemaFields(dialog.body, properties, initial, 0); button(dialog.actions, 'continue', async () => { const values = read(); const result = submit ? await submit(values) : values; if (!dialog.body.isConnected) return; dialog.close(); resolve(result); }); dialog.cancel = () => resolve(null); dialog.body.querySelector('input,select,textarea')?.focus(); }); }
        (async () => { try { const [capabilities, next, allProjects] = await Promise.all([api('/capabilities'), api('/status'), api('/projects')]); if (disposed) return; catalog = capabilities.operations; defs = capabilities.definitions; status = next; projects = allProjects; await navigate('overview'); eventSource = new EventSource('/api/kaggle/events'); eventSource.addEventListener('job', () => { if (active === 'jobs') jobs(generation).catch(error); }); } catch (err) { error(err); } })();
        instances.set(windowId, { dispose() { disposed = true; generation++; disposeEditor(); eventSource?.close(); requests.forEach(c => c.abort()); root.querySelectorAll('dialog').forEach(d => d.dispatchEvent(new Event('cancel'))); host.classList.remove('kg-host'); } });
    }
    function dispose(id) { instances.get(id)?.dispose(); instances.delete(id); }
    window.KaggleApp = { render, dispose };
})();
