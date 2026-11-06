(function () {
    'use strict';
    const instances = new Map();
    const domains = ['overview', 'datasets', 'models', 'notebooks', 'competitions', 'benchmarks', 'forums', 'jobs'];
    const navGroups = [{ label: 'browse', domains: ['overview', 'datasets', 'models', 'notebooks', 'competitions', 'benchmarks', 'forums'] }, { label: 'activity', domains: ['jobs'] }];
    const listOperations = { datasets: 'list_datasets', models: 'list_models', notebooks: 'list_kernels', competitions: 'list_competitions', benchmarks: 'list_benchmark_tasks', forums: 'list_topics' };
    const verb = name => name.startsWith('batch_schedule') ? 'run' : name.includes('cancel') ? 'cancel' : name.split('_')[0];
    const fieldWords = { ownerSlug: 'owner', userName: 'owner', user: 'owner', owner: 'owner', slug: 'reference', datasetSlug: 'reference', modelSlug: 'reference', kernelSlug: 'reference', instanceSlug: 'variant', competitionName: 'reference', fileName: 'file', filePath: 'file', path: 'file', title: 'title', newTitle: 'title', search: 'search', searchQuery: 'search', description: 'description', subtitle: 'description', pageSize: 'limit', page: 'page', pageToken: 'page', versionNumber: 'version', datasetVersionNumber: 'version', versionLabel: 'version', isPrivate: 'private', isPublic: 'public', enableGpu: 'gpu', enableTpu: 'tpu', enableInternet: 'internet', language: 'language', files: 'files', text: 'source', forumSlug: 'forum', topicId: 'topic', id: 'reference', framework: 'framework', licenseName: 'license', contentLength: 'bytes', kernelExecutionType: 'execution', sessionTimeoutSeconds: 'timeout' };
    const activeStates = ['queued', 'running', 'remote_running', 'cancel_requested'];
    const jobTones = { completed: 'ok', running: 'busy', remote_running: 'busy', queued: 'info', failed: 'bad', unknown: 'warn', resolved_unknown: 'warn', cancel_requested: 'warn', cancelled: 'muted' };
    // Paging belongs to the result pager, never to the operation form.
    const pagerFields = ['page', 'pageToken'];

    function render(host, windowId, ctx) {
        dispose(windowId);
        const t = key => ctx.t('kaggle.' + key);
        let disposed = false, active = 'overview', generation = 0, editor = null, status = {}, catalog = [], defs = {}, projects = [], selectedProject = '', eventSource, jobFilter = 'all', liveState = false;
        const requests = new Set();
        host.classList.add('kg-host');
        const root = document.createElement('section'); root.className = 'kg-app'; host.replaceChildren(root);
        const nav = document.createElement('nav'); nav.className = 'kg-nav'; nav.setAttribute('aria-label', t('navigation')); root.append(nav);
        for (const group of navGroups) {
            const wrap = document.createElement('div'); wrap.className = 'kg-nav-group';
            const label = document.createElement('span'); label.className = 'kg-nav-label'; label.textContent = t(group.label); wrap.append(label);
            for (const domain of group.domains) { const b = button(wrap, domain, event => navigate(domain, event.detail === 0)); b.dataset.domain = domain; b.setAttribute('aria-current', 'false'); }
            nav.append(wrap);
        }
        const navFoot = document.createElement('div'); navFoot.className = 'kg-nav-foot';
        const navChip = document.createElement('span'); navChip.className = 'kg-chip'; navChip.dataset.tone = 'muted'; navFoot.append(navChip); nav.append(navFoot);
        const main = document.createElement('main'); main.className = 'kg-main'; root.append(main);
        const notice = document.createElement('div'); notice.className = 'kg-notice'; notice.hidden = true;
        const noticeText = document.createElement('p'); noticeText.className = 'kg-notice-text'; noticeText.setAttribute('role', 'status');
        const noticeClose = document.createElement('button'); noticeClose.type = 'button'; noticeClose.className = 'kg-notice-close'; noticeClose.textContent = '×'; noticeClose.setAttribute('aria-label', t('close'));
        noticeClose.addEventListener('click', () => { notice.hidden = true; });
        notice.append(noticeText, noticeClose); main.append(notice);
        const live = document.createElement('div'); live.className = 'kg-sr-only'; live.setAttribute('aria-live', 'polite'); live.setAttribute('aria-atomic', 'true'); main.append(live);
        const panel = document.createElement('div'); panel.className = 'kg-panel'; main.append(panel);
        function announce(value) { if (disposed || !value) return; live.textContent = ''; setTimeout(() => { if (!disposed) live.textContent = value; }, 30); }
        function message(text, tone) { if (disposed) return; noticeText.textContent = text || ''; notice.dataset.tone = tone || 'info'; notice.hidden = !text; noticeText.setAttribute('role', tone === 'error' ? 'alert' : 'status'); }
        function error(err) { message(t('error') + ': ' + err.message, 'error'); }
        async function api(path, options = {}) {
            const controller = new AbortController(); requests.add(controller);
            try { const init = { credentials: 'same-origin', ...options, signal: controller.signal, headers: { ...(options.body !== undefined ? { 'Content-Type': 'application/json' } : {}), ...options.headers } }; if (init.body !== undefined && !(init.body instanceof Blob)) init.body = JSON.stringify(init.body); const response = await fetch('/api/kaggle' + path, init); const body = await response.json(); if (!response.ok) throw Error(response.status === 409 ? t('conflict') : body.error || body.message || response.status); return body; }
            finally { requests.delete(controller); }
        }
        function button(parent, key, action, variant) { const b = document.createElement('button'); b.type = 'button'; b.textContent = t(key); if (variant) b.classList.add('kg-btn-' + variant); b.addEventListener('click', async event => { if (b.getAttribute('aria-busy') === 'true') return; b.setAttribute('aria-busy', 'true'); try { await action(event); } catch (err) { const inline = b.closest('dialog')?.querySelector('[role="alert"]'); if (inline) inline.textContent = t('error') + ': ' + err.message; else error(err); } finally { b.removeAttribute('aria-busy'); b.disabled = b.dataset.blocked === 'true'; } }); parent.append(b); return b; }
        function blockButton(b, reason) { b.dataset.blocked = String(!!reason); b.disabled = !!reason; b.title = reason; }
        function title(parent, key) { const h = document.createElement('h2'); h.textContent = t(key); h.tabIndex = -1; parent.append(h); return h; }
        function text(parent, value, tag = 'p') { const e = document.createElement(tag); e.textContent = value; parent.append(e); return e; }
        function node(value) { return document.createTextNode(String(value)); }
        function chip(label, tone) { const c = document.createElement('span'); c.className = 'kg-chip'; c.dataset.tone = tone || 'muted'; c.textContent = label; return c; }
        function showLoading(target, label) { const box = document.createElement('div'); box.className = 'kg-loading'; const spinner = document.createElement('span'); spinner.className = 'kg-spinner'; spinner.setAttribute('aria-hidden', 'true'); box.append(spinner); const p = document.createElement('p'); p.className = 'kg-loading-text'; p.textContent = label || t('loading'); box.append(p); target.append(box); return box; }
        function emptyState(target, heading, hint, action) { const box = document.createElement('div'); box.className = 'kg-empty'; const head = document.createElement('p'); head.className = 'kg-empty-title'; head.textContent = heading; box.append(head); if (hint) { const p = document.createElement('p'); p.className = 'kg-empty-text'; p.textContent = hint; box.append(p); } if (action) box.append(action); target.append(box); return box; }
        function errorState(target, err, retry) { const box = document.createElement('div'); box.className = 'kg-empty kg-error-state'; const head = document.createElement('p'); head.className = 'kg-empty-title'; head.textContent = t('load_failed'); box.append(head); if (err && err.message) { const p = document.createElement('p'); p.className = 'kg-empty-text'; p.textContent = err.message; box.append(p); } const again = document.createElement('button'); again.type = 'button'; again.className = 'kg-btn-primary'; again.textContent = t('retry'); again.addEventListener('click', () => retry && retry()); box.append(again); target.append(box); return box; }
        function formatBytes(value) { const bytes = Number(value) || 0; if (bytes < 1024) return bytes + ' B'; if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KiB'; return (bytes / 1048576).toFixed(1) + ' MiB'; }
        function relativeTime(value) { const stamp = Date.parse(value || ''); if (!Number.isFinite(stamp)) return ''; const minutes = Math.round((Date.now() - stamp) / 60000); if (minutes < 1) return t('time_now'); if (minutes < 60) return t('time_minutes').replace('{0}', String(minutes)); const hours = Math.round(minutes / 60); if (hours < 24) return t('time_hours').replace('{0}', String(hours)); return t('time_days').replace('{0}', String(Math.round(hours / 24))); }
        function disposeEditor() { editor?.dispose(); editor = null; }
        function syncNavStatus() {
            navChip.dataset.tone = !status.enabled ? 'muted' : status.configured ? 'ok' : 'warn';
            navChip.textContent = !status.enabled ? t('disabled') : status.configured ? t('configured') : t('credentials_missing');
        }
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
        async function navigate(domain, focus = true) {
            const epoch = ++generation; active = domain; disposeEditor(); panel.replaceChildren(); message(''); nav.querySelectorAll('button[data-domain]').forEach(b => b.setAttribute('aria-current', b.dataset.domain === domain ? 'page' : 'false'));
            showLoading(panel);
            try {
                status = await api('/status'); if (disposed || epoch !== generation) return; syncNavStatus();
                if (domain === 'overview') await overview(epoch); else if (domain === 'jobs') await jobs(epoch); else await resources(domain, epoch);
                if (disposed || epoch !== generation) return;
                announce(t(domain));
                if (focus) focusPanel();
            } catch (err) { if (disposed || epoch !== generation) return; panel.replaceChildren(); errorState(panel, err, () => navigate(domain).catch(error)); error(err); }
        }
        function focusPanel() { const heading = panel.querySelector('h2'); if (heading) heading.focus({ preventScroll: true }); }
        async function overview(epoch) {
            const [next, allProjects, allJobs] = await Promise.all([api('/status'), api('/projects'), api('/jobs')]); if (disposed || epoch !== generation) return;
            status = next; projects = allProjects; syncNavStatus(); panel.replaceChildren();
            title(panel, 'overview');
            const stats = document.createElement('div'); stats.className = 'kg-stats'; panel.append(stats);
            const tile = (label, value) => { const card = document.createElement('div'); text(card, t(label), 'small'); const strong = document.createElement('strong'); strong.textContent = value; card.append(strong); stats.append(card); return card; };
            const activeJobs = allJobs.filter(job => activeStates.includes(job.status));
            tile('active_jobs', String(activeJobs.length));
            tile('connection', !status.enabled ? t('disabled') : status.configured ? t('configured') : t('credentials_missing'));
            tile('permissions', status.read_only ? t('read_only') : t('granted'));
            const used = Number(status.storage_bytes) || 0, max = Number(status.max_storage_bytes) || 0;
            const storage = tile('storage', (used / 1073741824).toFixed(2) + ' / ' + (max / 1073741824).toFixed(0) + ' GiB');
            const meter = document.createElement('div'); meter.className = 'kg-meter'; const fill = document.createElement('i'); fill.style.width = (max ? Math.min(100, Math.round(used / max * 100)) : 0) + '%'; meter.append(fill); storage.append(meter);
            const actions = document.createElement('div'); actions.className = 'kg-toolbar'; panel.append(actions);
            blockButton(button(actions, 'test', async () => { const result = await api('/test', { method: 'POST', body: {} }); message(t('connected') + ': ' + result.account, 'success'); announce(t('connected')); }), unavailable({ grant: 'read' }));
            blockButton(button(actions, 'quotas', async () => details(t('quotas'), await api('/quotas'))), unavailable({ grant: 'read' }));
            const config = document.createElement('a'); config.className = 'kg-link-button'; config.href = '/config#kaggle'; config.target = '_blank'; config.rel = 'noopener'; config.textContent = t('settings'); actions.append(config);
            title(panel, 'projects');
            const projectActions = document.createElement('div'); projectActions.className = 'kg-toolbar'; panel.append(projectActions);
            const newProject = button(projectActions, 'new_project', async () => { const result = await formDialog(t('new_project'), { title: { type: 'string', minLength: 1 } }, '', '', values => api('/projects', { method: 'POST', body: values })); if (!result) return; selectedProject = result.id; await navigate('overview'); }, 'primary');
            blockButton(newProject, unavailable({ grant: 'write' }));
            let projectSelect = null;
            if (projects.length) {
                const field = document.createElement('label'); field.className = 'kg-field-inline';
                const label = document.createElement('span'); label.textContent = t('projects'); field.append(label);
                projectSelect = select(field, projects.map(p => [p.id, p.title]), selectedProject);
                projectActions.append(field); selectedProject = projectSelect.value;
            }
            const filePanel = document.createElement('div'); filePanel.className = 'kg-files'; panel.append(filePanel);
            if (projectSelect) projectSelect.addEventListener('change', () => { selectedProject = projectSelect.value; showFiles(filePanel).catch(error); });
            await showFiles(filePanel);
        }
        function select(parent, choices, value) { const input = document.createElement('select'); choices.forEach(([key, name]) => { const option = document.createElement('option'); option.value = key; option.textContent = name; input.append(option); }); if (value) input.value = value; parent.append(input); return input; }
        async function showFiles(target) {
            target.replaceChildren(); if (!selectedProject) { emptyState(target, t('no_projects')); return; }
            const project = selectedProject, files = await api('/projects/' + project + '/files'); if (disposed || selectedProject !== project || !target.isConnected) return;
            const head = document.createElement('div'); head.className = 'kg-section-head';
            const titleText = document.createElement('strong'); titleText.textContent = projects.find(p => p.id === project)?.title || project; head.append(titleText);
            const count = document.createElement('span'); count.className = 'kg-chip-count'; count.textContent = String(files.length); head.append(count);
            target.append(head);
            const actions = document.createElement('div'); actions.className = 'kg-toolbar'; target.append(actions);
            blockButton(button(actions, 'import', () => importFiles(project)), unavailable({ grant: 'write' }));
            blockButton(button(actions, 'new_notebook', async () => {
                const name = await formDialog(t('new_notebook'), { fileName: { type: 'string', minLength: 1 } }, {}, '', async values => {
                    let name = values.fileName.trim(); if (!name.toLowerCase().endsWith('.ipynb')) name += '.ipynb';
                    await api('/projects/' + project + '/notebooks/' + encodeURIComponent(name), { method: 'PUT', headers: { 'If-None-Match': '*' }, body: { nbformat: 4, nbformat_minor: 5, metadata: { kernelspec: { name: 'python3', display_name: 'Python 3', language: 'python' } }, cells: [{ id: crypto.randomUUID().replaceAll('-', ''), cell_type: 'code', source: '', metadata: {}, execution_count: null, outputs: [] }] } });
                    return name;
                });
                if (name) await openNotebook(project, name);
            }, 'primary'), unavailable({ grant: 'write' }));
            if (!files.length) { emptyState(target, t('no_project_files')); return; }
            const list = document.createElement('div'); list.className = 'kg-list'; target.append(list);
            for (const file of files) {
                const row = document.createElement('div'); row.className = 'kg-resource';
                const main = document.createElement('div'); main.className = 'kg-resource-main';
                text(main, file.name, 'strong'); text(main, formatBytes(file.size), 'small');
                const rowActions = document.createElement('div'); rowActions.className = 'kg-resource-actions';
                if (file.name.toLowerCase().endsWith('.ipynb')) button(rowActions, 'open', () => openNotebook(project, file.name), 'primary');
                if (/\.(txt|log|csv|tsv|json|md)$/i.test(file.name)) button(rowActions, 'details', async () => { const preview = await api('/runtime', { method: 'POST', body: { operation: 'read_file', project_id: project, file: file.name } }); details(file.name, preview); });
                const link = document.createElement('a'); link.href = '/api/kaggle/projects/' + project + '/files/' + encodeURIComponent(file.name); link.download = file.name; link.textContent = t('export'); rowActions.append(link);
                row.append(main, rowActions); list.append(row);
            }
        }
        async function importFiles(project) { const input = document.createElement('input'); input.type = 'file'; input.multiple = true; input.addEventListener('change', async () => { try { message(t('loading')); for (const file of input.files) { const response = await fetch('/api/kaggle/projects/' + project + '/files/' + encodeURIComponent(file.name), { method: 'PUT', credentials: 'same-origin', body: file }); if (!response.ok) throw Error((await response.json()).error || t('error')); } navigate('overview'); } catch (err) { error(err); } }); input.click(); }
        async function openNotebook(project, file) {
            const epoch = ++generation, returnDomain = active; disposeEditor(); panel.replaceChildren(); message(''); active = 'notebooks';
            nav.querySelectorAll('button[data-domain]').forEach(b => b.setAttribute('aria-current', b.dataset.domain === active ? 'page' : 'false'));
            showLoading(panel);
            try {
                const next = await window.KaggleNotebook(panel, { api, t, project, projectTitle: projects.find(p => p.id === project)?.title || project, file, windowId, notify: message, selectFiles, unavailable, isClosed: () => disposed || epoch !== generation, remote: (doc, revision) => remoteNotebook(project, file, doc, revision), exit: () => navigate(returnDomain) });
                if (disposed || epoch !== generation) next.dispose(); else editor = next;
            } catch (err) { if (disposed || epoch !== generation) return; panel.replaceChildren(); errorState(panel, err, () => openNotebook(project, file).catch(error)); error(err); }
        }
        async function selectFiles(project) { const files = await api('/projects/' + project + '/files'); return new Promise(resolve => { const dialog = modal(t('select_files')); files.forEach(file => { const label = document.createElement('label'); const check = document.createElement('input'); check.type = 'checkbox'; check.value = file.name; label.append(check, document.createTextNode(file.name)); dialog.body.append(label); }); text(dialog.body, t('local_files_hint')); button(dialog.actions, 'continue', () => { const values = [...dialog.body.querySelectorAll('input:checked')].map(c => c.value); dialog.close(); resolve(values); }); dialog.cancel = () => resolve(null); }); }
        async function remoteNotebook(project, file, doc, revision) {
            status = await api('/status'); const reason = unavailable({ grant: 'execute' }); if (reason) throw Error(reason);
            const values = await formDialog(t('kaggle_run'), { slug: { type: 'string' }, newTitle: { type: 'string' }, datasetDataSources: { type: 'array', items: { type: 'string' } }, competitionDataSources: { type: 'array', items: { type: 'string' } }, modelDataSources: { type: 'array', items: { type: 'string' } } }, {}, t('remote_hint'));
            if (!values) return;
            const result = await api('/operations', { method: 'POST', body: { domain: 'notebooks', operation: 'save_kernel', parameters: { ...values, text: JSON.stringify(doc), kernelType: 'notebook', language: 'python', kernelExecutionType: 'SAVE_AND_RUN_ALL', isPrivate: true, enableGpu: false, enableTpu: false, enableInternet: false } } });
            message(t('job_created') + ': ' + result.job.id, 'success'); announce(t('job_created'));
        }
        async function resources(domain, epoch) {
            panel.replaceChildren();
            title(panel, domain); if (!status.enabled) { emptyState(panel, t('enable_first')); return; }
            const controls = document.createElement('div'); controls.className = 'kg-toolbar'; panel.append(controls);
            const searchWrap = document.createElement('div'); searchWrap.className = 'kg-search';
            const search = document.createElement('input'); search.type = 'search'; search.placeholder = t('search'); search.setAttribute('aria-label', t('search'));
            const clearSearch = document.createElement('button'); clearSearch.type = 'button'; clearSearch.className = 'kg-search-clear'; clearSearch.textContent = '×'; clearSearch.setAttribute('aria-label', t('clear_search')); clearSearch.title = t('clear_search'); clearSearch.hidden = true;
            clearSearch.addEventListener('click', () => { search.value = ''; clearSearch.hidden = true; search.focus(); });
            search.addEventListener('input', () => { clearSearch.hidden = !search.value; });
            searchWrap.append(search, clearSearch); controls.append(searchWrap);
            const items = catalog.filter(op => op.domain === domain); const chooser = select(controls, items.map(op => [op.operation, t('verb_' + verb(op.operation)) + ' · ' + op.operation]), listOperations[domain]); chooser.setAttribute('aria-label', t('operation'));
            let readForm;
            let executeButton;
            let refreshButton;
            const reason = document.createElement('p'); reason.className = 'kg-state kg-reason'; reason.hidden = true;
            const filters = document.createElement('details'); filters.className = 'kg-filters'; filters.hidden = true; const summary = document.createElement('summary'); summary.textContent = t('filters'); filters.append(summary);
            const fields = document.createElement('div'); fields.className = 'kg-fields'; filters.append(fields);
            const hint = document.createElement('p'); hint.className = 'kg-hint'; hint.textContent = t('api_fields_hint'); hint.hidden = true;
            panel.append(reason, filters, hint);
            const searchField = domain === 'forums' ? 'searchQuery' : domain === 'benchmarks' ? 'regexFilter' : 'search';
            const drawForm = () => {
                const op = items.find(o => o.operation === chooser.value); if (!op) return;
                fields.replaceChildren(); const properties = { ...op.schema.properties };
                const supports = !!properties[searchField];
                searchWrap.hidden = !supports;
                if (!supports && search.value) { search.value = ''; clearSearch.hidden = true; }
                delete properties[searchField];
                for (const field of pagerFields) delete properties[field];
                const extended = Object.keys(properties);
                filters.hidden = !extended.length;
                filters.open = extended.length > 0 && op.operation !== listOperations[domain];
                readForm = schemaFields(fields, properties, {}, 0);
                const why = unavailable(op); reason.textContent = why; reason.hidden = !why;
                hint.hidden = !extended.length;
                if (executeButton) blockButton(executeButton, why);
            };
            chooser.addEventListener('change', drawForm);
            const output = document.createElement('div'); output.className = 'kg-results'; panel.append(output);
            const execute = async (pageToken, append) => {
                status = await api('/status'); const op = items.find(o => o.operation === chooser.value); const why = unavailable(op); if (why) { drawForm(); throw Error(why); }
                const parameters = readForm(); if (search.value && op.schema.properties[searchField]) parameters[searchField] = search.value; if (pageToken) parameters.pageToken = pageToken;
                let transfer = {};
                if (op.grant === 'download' || op.operation.includes('upload')) { transfer = await formDialog(t('destination'), { project_id: { type: 'string', enum: projects.map(p => p.id) }, file: { type: 'string' } }); if (!transfer) return; }
                message(t('loading'));
                const loader = showLoading(output); output.setAttribute('aria-busy', 'true');
                let result;
                try { result = await api('/operations', { method: 'POST', body: { domain, operation: op.operation, parameters, ...transfer } }); }
                finally { loader.remove(); output.setAttribute('aria-busy', 'false'); }
                if (disposed || epoch !== generation) return;
                if (result.job) { message(t('job_created') + ': ' + result.job.id, 'success'); announce(t('job_created')); return; }
                message(''); renderResults(output, result.data, domain, append, token => execute(token, true)); if (refreshButton) { refreshButton.hidden = false; refreshButton.disabled = false; }
            };
            executeButton = button(controls, 'execute', () => execute(), 'primary');
            refreshButton = button(controls, 'refresh', () => execute(), 'quiet'); refreshButton.hidden = true;
            drawForm(); search.addEventListener('keydown', event => { if (event.key === 'Enter' && !executeButton.disabled) execute().catch(error); });
            if (!output.childElementCount) emptyState(output, t('resources_hint'));
        }
        function referenceOf(item) {
            if (!item || typeof item !== 'object') return '';
            if (item.ref) return String(item.ref);
            if (item.author && item.slug) return String(item.author) + '/' + String(item.slug);
            return item.slug || item.name || item.id || '';
        }
        async function copyReference(value, source) {
            try {
                if (navigator.clipboard && navigator.clipboard.writeText) await navigator.clipboard.writeText(value);
                else { const area = document.createElement('textarea'); area.value = value; area.setAttribute('readonly', ''); area.style.position = 'fixed'; area.style.opacity = '0'; document.body.append(area); area.select(); try { document.execCommand('copy'); } catch (_) {} area.remove(); }
                if (source) { const label = source.textContent; source.textContent = t('copied'); source.dataset.confirmed = 'true'; setTimeout(() => { source.textContent = label; delete source.dataset.confirmed; }, 1600); }
                message(t('copied') + ': ' + value, 'success');
            } catch (_) { message(t('error'), 'error'); }
        }
        function renderResults(parent, data, domain, append, next) {
            const list = Array.isArray(data) ? data : Object.values(data || {}).find(Array.isArray);
            if (!list) { if (!append) { parent.replaceChildren(); parent.dataset.shown = '0'; } detailsInto(parent, data); return; }
            let head = parent.querySelector('.kg-results-head'), listBox = parent.querySelector('.kg-results-list'), pager = parent.querySelector('.kg-pager');
            if (!head || !listBox || !pager) {
                parent.querySelectorAll('.kg-empty').forEach(el => el.remove());
                head = document.createElement('div'); head.className = 'kg-results-head'; head.setAttribute('role', 'status');
                listBox = document.createElement('div'); listBox.className = 'kg-results-list';
                pager = document.createElement('div'); pager.className = 'kg-pager';
                parent.append(head, listBox, pager);
            }
            if (!append) { listBox.replaceChildren(); parent.dataset.shown = '0'; }
            const shown = Number(parent.dataset.shown || 0) + list.length; parent.dataset.shown = String(shown);
            const headLabel = document.createElement('span'); headLabel.textContent = t('results');
            const headCount = document.createElement('strong'); headCount.textContent = String(shown);
            head.replaceChildren(headLabel, headCount);
            pager.replaceChildren();
            if (!shown) { const empty = document.createElement('div'); empty.className = 'kg-empty'; text(empty, t('no_results')); listBox.append(empty); return; }
            for (const item of list) {
                const row = document.createElement('div'); row.className = 'kg-resource';
                const main = document.createElement('div'); main.className = 'kg-resource-main';
                text(main, item.title || item.name || item.ref || item.slug || item.id || '—', 'strong');
                const description = item.subtitle || item.description || item.url || ''; if (description) text(main, String(description).slice(0, 220), 'small');
                const reference = referenceOf(item);
                if (reference) { const ref = document.createElement('code'); ref.className = 'kg-reference'; ref.textContent = reference; main.append(ref); }
                const rowActions = document.createElement('div'); rowActions.className = 'kg-resource-actions';
                if (reference) button(rowActions, 'copy', event => copyReference(reference, event.currentTarget), 'quiet');
                button(rowActions, 'details', () => details(t(domain), item));
                if (domain === 'notebooks') button(rowActions, 'import_notebook', () => importNotebook(item));
                row.append(main, rowActions); listBox.append(row);
            }
            const token = data?.nextPageToken || data?.pageToken; if (token && next) button(pager, 'next', () => next(token));
        }
        async function importNotebook(item) { const ref = item.ref || (item.author && item.slug ? item.author + '/' + item.slug : ''); const values = await formDialog(t('import_notebook'), { userName: { type: 'string' }, kernelSlug: { type: 'string' }, project_id: { type: 'string', enum: projects.map(p => p.id) }, fileName: { type: 'string' } }, { userName: ref.split('/')[0] || '', kernelSlug: ref.split('/')[1] || '', fileName: (ref.split('/')[1] || 'notebook') + '.ipynb' }); if (!values) return; const { project_id, fileName, ...parameters } = values; const result = await api('/operations', { method: 'POST', body: { domain: 'notebooks', operation: 'get_kernel', parameters } }); const content = JSON.parse(result.data.blob.source); await api('/projects/' + project_id + '/notebooks/' + encodeURIComponent(fileName), { method: 'PUT', headers: { 'If-None-Match': '*' }, body: content }); await openNotebook(project_id, fileName); }
        function detailsInto(parent, data) { if (!data || typeof data !== 'object') { text(parent, String(data ?? '')); return; } const list = document.createElement('dl'); list.className = 'kg-details'; for (const [key, value] of Object.entries(data)) { text(list, fieldLabel(key), 'dt'); const term = document.createElement('dd'); if (value && typeof value === 'object') { const pre = document.createElement('pre'); pre.textContent = JSON.stringify(value, null, 2); term.append(pre); } else term.textContent = String(value ?? ''); list.append(term); } parent.append(list); }
        function details(name, data) { const d = modal(name); detailsInto(d.body, data); button(d.actions, 'close', d.close, 'primary'); }
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
            }, 'primary');
            blockButton(submit, t('resolve_unknown_confirm'));
            check.addEventListener('change', () => blockButton(submit, check.checked ? '' : t('resolve_unknown_confirm')));
            check.focus();
        }
        async function jobs(epoch) {
            const [all, current] = await Promise.all([api('/jobs'), api('/status')]);
            if (disposed || epoch !== generation) return;
            status = current; panel.replaceChildren(); title(panel, 'jobs');
            const filters = document.createElement('div'); filters.className = 'kg-toolbar'; panel.append(filters);
            const counts = { all: all.length, active: 0, failed: 0 };
            for (const job of all) { if (activeStates.includes(job.status)) counts.active++; if (job.status === 'failed') counts.failed++; }
            for (const key of ['all', 'active', 'state_failed']) {
                const filter = key === 'state_failed' ? 'failed' : key;
                const chipButton = button(filters, key, () => { jobFilter = filter; jobs(epoch).catch(error); }, 'quiet');
                chipButton.setAttribute('aria-pressed', jobFilter === filter ? 'true' : 'false');
                const badge = document.createElement('span'); badge.className = 'kg-chip-count'; badge.textContent = String(counts[filter]); chipButton.append(badge);
            }
            const liveChip = chip(t('live'), liveState ? 'ok' : 'warn'); liveChip.classList.add('kg-live-chip'); filters.append(liveChip);
            const visible = all.filter(job => jobFilter === 'active' ? activeStates.includes(job.status) : jobFilter === 'failed' ? job.status === 'failed' : true);
            if (!visible.length) { emptyState(panel, t('no_jobs'), t('jobs_empty_hint')); return; }
            const list = document.createElement('div'); list.className = 'kg-list'; panel.append(list);
            for (const job of visible) {
                const row = document.createElement('div'); row.className = 'kg-resource';
                if (activeStates.includes(job.status)) row.setAttribute('aria-busy', 'true');
                const main = document.createElement('div'); main.className = 'kg-resource-main';
                text(main, job.operation, 'strong');
                const meta = document.createElement('div'); meta.className = 'kg-resource-meta';
                meta.append(chip(t('state_' + job.status), jobTones[job.status]), node(job.domain + ' · ' + job.id));
                main.append(meta);
                const facts = document.createElement('div'); facts.className = 'kg-resource-facts';
                const created = relativeTime(job.created); if (created) facts.append(node(t('created') + ' ' + created));
                if (job.progress > 0) facts.append(node(formatBytes(job.progress)));
                if (facts.childNodes.length) main.append(facts);
                if (job.error) text(main, job.error, 'small');
                if (activeStates.includes(job.status)) { const meter = document.createElement('div'); meter.className = 'kg-meter kg-meter-indeterminate'; meter.append(document.createElement('i')); main.append(meter); }
                const rowActions = document.createElement('div'); rowActions.className = 'kg-resource-actions';
                button(rowActions, 'details', () => details(t('jobs'), job.result || job));
                const cancelOnSite = job.domain === 'notebooks' && job.status === 'remote_running' && !job.result?.kernelSessionId;
                if (['queued', 'running', 'remote_running'].includes(job.status) && !cancelOnSite) {
                    const cancel = button(rowActions, 'cancel', async () => {
                        status = await api('/status');
                        const reason = cancelUnavailable(job); blockButton(cancel, reason);
                        if (reason) throw Error(reason);
                        await api('/jobs/' + job.id + '/cancel', { method: 'POST', body: {} }); await jobs(epoch);
                    }, 'danger');
                    blockButton(cancel, cancelUnavailable(job));
                }
                if (job.status === 'unknown' && status.can_resolve_unknown) button(rowActions, 'resolve_unknown', () => resolveUnknown(job, epoch));
                if (cancelOnSite) {
                    const link = document.createElement('a'); const ref = job.result?.ref;
                    link.href = typeof ref === 'string' && /^[a-zA-Z0-9_-]+\/[a-zA-Z0-9_-]+$/.test(ref) ? 'https://www.kaggle.com/code/' + ref : 'https://www.kaggle.com/code';
                    link.target = '_blank'; link.rel = 'noopener noreferrer'; link.textContent = t('cancel_on_site'); link.title = t('cancel_on_site'); rowActions.append(link);
                }
                row.append(main, rowActions); list.append(row);
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
            const dialog = document.createElement('dialog'); dialog.className = 'kg-dialog'; dialog.setAttribute('aria-label', name); const hostWidth = host.clientWidth || host.getBoundingClientRect().width; if (hostWidth) dialog.style.setProperty('--kg-host-width', hostWidth + 'px'); const heading = document.createElement('h2'); heading.textContent = name; dialog.append(heading); const body = document.createElement('div'); body.className = 'kg-dialog-body'; dialog.append(body); const feedback = document.createElement('p'); feedback.className = 'kg-error'; feedback.setAttribute('role', 'alert'); dialog.append(feedback); const actions = document.createElement('div'); actions.className = 'kg-toolbar'; dialog.append(actions); root.append(dialog); const result = { body, actions, cancel: null, close() { dialog.close(); dialog.remove(); if (returnFocus?.isConnected) returnFocus.focus(); } }; dialog.addEventListener('cancel', event => { event.preventDefault(); result.cancel?.(); result.close(); }); button(actions, 'cancel', () => { result.cancel?.(); result.close(); }); dialog.showModal(); return result;
        }
        function formDialog(name, properties, initial = {}, hint = '', submit) { return new Promise(resolve => { const dialog = modal(name); if (hint) text(dialog.body, hint); const read = schemaFields(dialog.body, properties, initial, 0); button(dialog.actions, 'continue', async () => { const values = read(); const result = submit ? await submit(values) : values; if (!dialog.body.isConnected) return; dialog.close(); resolve(result); }, 'primary'); dialog.cancel = () => resolve(null); dialog.body.querySelector('input,select,textarea')?.focus(); }); }
        (async () => { try { const [capabilities, next, allProjects] = await Promise.all([api('/capabilities'), api('/status'), api('/projects')]); if (disposed) return; catalog = capabilities.operations; defs = capabilities.definitions; status = next; projects = allProjects; syncNavStatus(); await navigate('overview', false); eventSource = new EventSource('/api/kaggle/events'); eventSource.addEventListener('job', () => { if (active === 'jobs') jobs(generation).catch(error); }); eventSource.onopen = () => { liveState = true; }; eventSource.onerror = () => { liveState = false; }; } catch (err) { error(err); } })();
        instances.set(windowId, { dispose() { disposed = true; generation++; disposeEditor(); eventSource?.close(); requests.forEach(c => c.abort()); root.querySelectorAll('dialog').forEach(d => d.dispatchEvent(new Event('cancel'))); host.classList.remove('kg-host'); } });
    }
    function dispose(id) { instances.get(id)?.dispose(); instances.delete(id); }
    window.KaggleApp = { render, dispose };
})();
