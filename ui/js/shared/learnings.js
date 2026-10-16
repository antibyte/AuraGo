/* Shared Memory and project knowledge views. Source text is always rendered as text. */
(function () {
    'use strict';
    function element(tag, text, className) {
        const node = document.createElement(tag);
        if (text != null) node.textContent = String(text);
        if (className) node.className = className;
        return node;
    }
    async function mount(host, options = {}) {
        const lifecycle = new AbortController();
        let generation = 0, pending = null, disposed = false, dictionary = {};
        const tr = key => dictionary['learnings.' + key] || key;
        const request = async (url, init = {}) => {
            const response = await fetch(url, { credentials: 'same-origin', signal: lifecycle.signal, ...init });
            if (!response.ok) throw new Error(response.status === 403 ? tr('disabled') : tr('error'));
            return response.json();
        };
        const dispose = () => { disposed = true; generation++; lifecycle.abort(); if (pending) pending.abort(); host.replaceChildren(); };
		if (options.signal) options.signal.addEventListener('abort', dispose, { once: true, signal: lifecycle.signal });
        try {
            const lang = window.SYSTEM_LANG || document.documentElement.lang || 'en';
            const result = await request('/api/i18n?lang=' + encodeURIComponent(lang) + '&sections=learnings');
            dictionary = result.data || {};
        } catch (error) {
            if (!disposed) host.textContent = error.message;
            return dispose;
        }
        if (disposed || !host.isConnected) return dispose;
        dispose.closeLabel = tr('close');
        host.classList.add('learning-view');
        const heading = element('h3', tr('title'));
        const status = element('p', '', 'learning-status'); status.setAttribute('role', 'status'); status.setAttribute('aria-live', 'polite');
        const controls = element('form', null, 'learning-controls');
        const search = element('input'); search.type = 'search'; search.placeholder = tr('search'); search.setAttribute('aria-label', tr('search'));
        const scopes = element('select'); scopes.setAttribute('aria-label', tr('scope'));
        for (const [value, label] of [['', tr('all')], ['global', tr('global')]]) { const option = element('option', label); option.value = value; scopes.append(option); }
        scopes.hidden = !!options.project;
        const states = element('select'); states.setAttribute('aria-label', tr('state'));
        for (const value of ['', 'proposed', 'active', 'revalidation', 'superseded', 'archived']) { const option = element('option', tr(value || 'all')); option.value = value; states.append(option); }
        const refresh = element('button', tr('refresh')); refresh.type = 'submit';
        controls.append(search, scopes, states, refresh);
        const list = element('div', null, 'learning-list');
        const details = element('div', null, 'learning-details');
        host.append(heading, controls, status, list, details);
        const endpoint = options.project ? '/api/desktop/learnings' : '/api/learnings';
        function url(extra = {}) {
            const params = new URLSearchParams({ project: options.project || scopes.value, query: search.value, status: states.value, ...extra });
            return endpoint + '?' + params;
        }
        function button(label, action) { const b = element('button', tr(label)); b.type = 'button'; b.addEventListener('click', action); return b; }
        function report(error) { if (!disposed && error.name !== 'AbortError') status.textContent = error.message; }
        async function history(item) {
            const id = generation;
            status.textContent = tr('loading');
            try {
                const data = await request(url({ id: item.id }));
                if (disposed || id !== generation) return;
                details.replaceChildren(element('h4', tr('history')));
                for (const revision of data.history || []) {
                    const row = element('details');
                    row.append(element('summary', '#' + revision.learning.revision + ' · ' + tr(revision.learning.status) + ' · ' + revision.actor));
                    row.append(element('p', revision.learning.statement), element('p', revision.reason));
                    details.append(row);
                }
                status.textContent = '';
            } catch (error) { report(error); }
        }
        function review(item, action) {
            details.replaceChildren();
            const form = element('form', null, 'learning-review');
            form.append(element('h4', tr(action)));
            const statement = element('textarea', item.statement); statement.maxLength = 2000; statement.rows = 3;
            statement.setAttribute('aria-label', tr('statement')); statement.required = true; statement.readOnly = action !== 'replace';
            const reason = element('textarea'); reason.rows = 2; reason.maxLength = 2000; reason.required = true; reason.placeholder = tr('reason'); reason.setAttribute('aria-label', tr('reason'));
            const submit = element('button', tr('save')); submit.type = 'submit';
            const cancel = button('cancel', () => details.replaceChildren());
            form.append(statement, reason, submit, cancel); details.append(form); reason.focus();
            form.addEventListener('submit', async event => {
                event.preventDefault(); if (!form.reportValidity() || submit.disabled) return;
                submit.disabled = cancel.disabled = true; status.textContent = tr('saving');
                try {
                    await request(url(), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: item.id, revision: item.revision, action, statement: statement.value, reason: reason.value }) });
                    if (disposed) return;
                    details.replaceChildren(); await load(); status.textContent = tr('saved');
                } catch (error) { if (!disposed && error.name !== 'AbortError') status.textContent = tr('write_error'); }
                finally { submit.disabled = cancel.disabled = false; }
            });
        }
        async function load() {
            const id = ++generation;
            if (pending) pending.abort(); pending = new AbortController();
            status.textContent = tr('loading'); refresh.disabled = true;
            try {
                const data = await request(url(), { signal: AbortSignal.any([lifecycle.signal, pending.signal]) });
                if (disposed || generation !== id) return;
                list.replaceChildren(); details.replaceChildren();
                status.textContent = !data.enabled ? tr('disabled') : data.limited ? tr('limited') : '';
                const items = data.learnings || [];
                if (!items.length) list.append(element('p', tr('empty')));
                for (const item of items) {
                    if (item.scope.project && !Array.from(scopes.options).some(o => o.value === item.scope.project)) { const option = element('option', item.scope.project); option.value = item.scope.project; scopes.append(option); }
                    const row = element('article', null, 'learning-item');
                    row.append(element('h4', item.statement), element('p', item.rationale));
                    row.append(element('small', tr(item.status) + ' · ' + (item.scope.project || tr('global')) + (item.scope.resource ? '/' + item.scope.resource : '') + ' · #' + item.revision));
                    const sources = element('details'); sources.append(element('summary', tr('sources')));
                    for (const source of item.sources || []) sources.append(element('p', source.reference + ' · ' + source.session), element('blockquote', source.quote));
                    if (item.condition) sources.append(element('pre', JSON.stringify(item.condition, null, 2)));
                    for (const alternate of item.alternatives || []) sources.append(element('p', tr('alternative') + ': ' + alternate));
                    if (item.previous_id) sources.append(element('p', tr('previous') + ': ' + item.previous_id));
                    if (item.superseded_by) sources.append(element('p', tr('superseded') + ': ' + item.superseded_by));
                    row.append(sources, button('history', () => history(item)));
                    if (data.writable && !['archived', 'superseded'].includes(item.status)) {
                        for (const action of ['confirm', 'replace', 'archive']) {
                            const control = button(action, () => review(item, action));
                            control.disabled = !!item.protected || (action === 'confirm' && item.status === 'active'); row.append(control);
                        }
                        row.append(button(item.protected ? 'unprotect' : 'protect', () => review(item, item.protected ? 'unprotect' : 'protect')));
                    }
                    list.append(row);
                }
                const metrics = data.metrics || {};
                list.append(element('small', tr('metrics') + ': ' + (metrics.retrievals || 0) + ' / ' + (metrics.discarded || 0) + ' / ' + (metrics.context_tokens || 0)));
                if (data.overlaps && data.overlaps.length) {
                    const work = element('details'); work.append(element('summary', tr('work')));
                    for (const target of data.overlaps) work.append(element('p', target.run + ' · ' + target.resource + (target.symbol ? ' · ' + target.symbol : '')));
                    list.append(work);
                }
            } catch (error) { if (generation === id) report(error); }
            finally { if (generation === id) refresh.disabled = false; }
        }
        controls.addEventListener('submit', event => { event.preventDefault(); load(); });
        scopes.addEventListener('change', load); states.addEventListener('change', load);
        if (options.project && options.graph) {
            const graph = element('section', null, 'learning-graph');
            const form = element('form', null, 'learning-controls');
            const query = element('input'); query.setAttribute('aria-label', tr('symbol')); query.placeholder = tr('symbol');
            const mode = element('select'); mode.setAttribute('aria-label', tr('graph'));
            for (const value of ['search', 'outline']) { const option = element('option', tr(value)); option.value = value; mode.append(option); }
            const searchButton = element('button', tr('search')); searchButton.type = 'submit';
            const graphStatus = element('p'); graphStatus.setAttribute('role', 'status');
            const results = element('div');
            let graphGeneration = 0, graphRequest = null;
            const graphQuery = async (operation, extra = {}) => {
                const id = ++graphGeneration;
                if (graphRequest) graphRequest.abort(); graphRequest = new AbortController();
                graphStatus.textContent = tr('loading'); searchButton.disabled = true;
                try {
                    const params = new URLSearchParams({ project: options.project, operation, query: query.value, file: operation === 'outline' ? (query.value || options.file || '').replace(/^\/workspace\//, '') : '', depth: '3', ...extra });
                    const result = await request('/api/desktop/code-graph?' + params, { method: operation === 'rescan' ? 'POST' : 'GET', signal: AbortSignal.any([lifecycle.signal, graphRequest.signal]) });
                    if (disposed || graphGeneration !== id) return;
                    const s = result.status;
                    graphStatus.textContent = (s.complete ? tr('complete') : tr('partial')) + ' · ' + s.indexed + '/' + s.files + ' · ' + tr('unresolved') + ': ' + s.unresolved + ' · ' + new Date(s.updated_at).toLocaleString() + (s.stale ? ' · ' + tr('stale') : '') + (result.truncated ? ' · ' + tr('limited') : '');
                    results.replaceChildren(element('p', tr('graph_warning')));
                    if (!result.symbols.length && !result.edges.length) results.append(element('p', tr('empty')));
                    function sourceLink(item) { const link = element('button', item.file + ':' + item.line); link.type = 'button'; link.disabled = typeof options.openSource !== 'function'; link.addEventListener('click', () => Promise.resolve(options.openSource(item.file, item.line)).catch(report)); return link; }
                    for (const symbol of result.symbols) {
                        const row = element('article', null, 'learning-item'); row.append(element('strong', symbol.name), sourceLink(symbol), element('pre', symbol.snippet));
                        for (const op of ['callers', 'callees', 'impact']) row.append(button(op, () => graphQuery(op, { symbol: symbol.id })));
                        results.append(row);
                    }
                    for (const edge of result.edges) { const row = element('p', edge.name + ' · ' + (edge.to || tr('unresolved')) + ' '); row.append(sourceLink(edge)); results.append(row); }
                    const skipped = element('details'); skipped.append(element('summary', tr('coverage')));
                    for (const [file, reason] of Object.entries(s.skipped || {})) skipped.append(element('p', file + ' · ' + reason));
                    results.append(skipped);
                } catch (error) { if (!disposed && graphGeneration === id && error.name !== 'AbortError') graphStatus.textContent = error.message; }
                finally { if (graphGeneration === id) searchButton.disabled = false; }
            };
            form.append(query, mode, searchButton, button('rescan', () => graphQuery('rescan')));
            form.addEventListener('submit', event => { event.preventDefault(); graphQuery(mode.value); });
            graph.append(element('h3', tr('graph')), form, graphStatus, results); host.append(graph);
            graphQuery('status');
        }
        window.addEventListener('pagehide', dispose, { once: true, signal: lifecycle.signal });
        window.addEventListener('aurago:desktop-policy', () => { details.replaceChildren(); load(); }, { signal: lifecycle.signal });
        await load();
        return dispose;
    }
    function open(options) {
        const dialog = element('dialog', null, 'learning-dialog');
        dialog.setAttribute('aria-label', options.title || '');
        const close = element('button', '×', 'learning-close'); close.type = 'button'; close.setAttribute('aria-label', typeof window.t === 'function' ? window.t('learnings.close') : 'Close');
        const host = element('div'); dialog.append(close, host); (options.parent || document.body).append(dialog);
        const controller = new AbortController();
        let dispose = null, closed = false;
        const cleanup = () => { closed = true; controller.abort(); if (dispose) dispose(); dialog.remove(); };
        close.addEventListener('click', () => dialog.close()); dialog.addEventListener('close', cleanup, { once: true });
        dialog.showModal(); close.focus();
        mount(host, { ...options, signal: controller.signal }).then(fn => { dispose = fn; if (closed) dispose(); const title = host.querySelector('h3'); if (title) { dialog.setAttribute('aria-label', title.textContent); close.setAttribute('aria-label', fn.closeLabel); } });
        return () => { if (!closed) dialog.close(); };
    }
    window.AuraGoLearnings = { mount, open };
    document.addEventListener('DOMContentLoaded', () => { const host = document.querySelector('[data-learnings-memory]'); if (host) mount(host); });
})();
