(function () {
    'use strict';
    let generation = 0;
    let controller = null;
    let timer = null;
    let active = null;
    let posting = false;
    const el = id => document.getElementById('selftest-' + id);
    const text = (id, value) => { const node = el(id); if (node) node.textContent = value; };
    const tr = key => t('dashboard.selftest_' + key);
    function stopPolling() { clearTimeout(timer); timer = null; }
    async function request(path, options = {}) {
        const response = await fetch('/api/self-tests' + path, { credentials: 'same-origin', signal: controller && controller.signal, ...options });
        if (!response.ok) throw new Error(String(response.status));
        return response.json();
    }
    function controls() {
        if (el('start')) el('start').disabled = posting || !!(active && active.status === 'running');
        if (el('stop')) el('stop').disabled = !(active && active.status === 'running');
    }
    function show(run) {
        active = run;
        const summary = run.summary || {};
        const verdicts = summary.verdicts || {};
        text('status', tr('status_' + (run.status === 'storage_error' ? 'error' : run.status)));
        text('summary', `${tr('tool')}: ${summary.executed_tools || 0} / ${summary.enabled_tools || 0} · ${tr('connections')}: ${summary.executed_integrations || 0} / ${summary.enabled_integrations || 0} · PASS ${verdicts.PASS || 0} · FAIL ${verdicts.FAIL || 0} · ${summary.tokens || 0}`);
        const table = el('rows');
        if (table) {
            table.replaceChildren();
            for (const item of run.cases || []) {
                const row = document.createElement('tr');
                for (const value of [item.template_id, item.tool, item.verdict, item.code || item.cleanup]) {
                    const cell = document.createElement('td'); cell.textContent = value || ''; row.appendChild(cell);
                }
                table.appendChild(row);
            }
        }
        for (const [id, file] of [['report', 'report.md'], ['results', 'results.json']]) {
            const link = el(id);
            if (link) { link.hidden = !/^[a-f0-9]{32}$/.test(run.id || ''); if (!link.hidden) link.href = '/api/self-tests/' + run.id + '/' + file; }
        }
        controls();
    }
    function poll(epoch) {
        stopPolling();
        const section = document.getElementById('sec-tools');
        if (!active || active.status !== 'running' || document.hidden || !section || section.hidden) return;
        const id = active.id;
        timer = setTimeout(async () => {
            if (document.hidden || section.hidden || epoch !== generation) return;
            try {
                const run = await request('/' + id);
                if (epoch !== generation || !active || active.id !== id) return;
                show(run); poll(epoch);
            } catch (error) { if (epoch === generation && error.name !== 'AbortError') text('status', tr('failed')); }
        }, 2000);
    }
    async function load() {
        if (!el('status')) return;
        const epoch = ++generation;
        if (controller) controller.abort();
        controller = new AbortController(); stopPolling();
        text('status', tr('loading'));
        try {
            const [catalog, history] = await Promise.all([request('/catalog'), request('')]);
            if (epoch !== generation) return;
            const select = el('target');
            const previous = select.value;
            select.replaceChildren(new Option(tr('all'), ''));
            for (const name of [...new Set((catalog.inventory || []).filter(item => item.enabled).map(item => item.name))].sort()) select.add(new Option(name, name));
            if ([...select.options].some(option => option.value === previous)) select.value = previous;
            const run = (history.runs || []).find(item => item.status === 'running') || (history.runs || [])[0];
            if (run) { const full = await request('/' + run.id); if (epoch !== generation) return; show(full); poll(epoch); }
            else { active = null; text('status', tr('empty')); controls(); }
        } catch (error) { if (epoch === generation && error.name !== 'AbortError') text('status', tr('failed')); }
    }
    async function start() {
        if (posting) return;
        posting = true; controls(); stopPolling();
        const epoch = generation;
        const target = el('target').value;
        try {
            const body = { profile: el('profile').value, targets: target ? [target] : [] };
            if (body.profile === 'targeted') { body.bindings = JSON.parse(el('bindings').value); if (!Array.isArray(body.bindings) || !body.bindings.length) throw new Error('bindings'); }
            const run = await request('', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
            if (epoch !== generation) return;
            show(run); poll(epoch);
        } catch (error) { if (epoch === generation && error.name !== 'AbortError') text('status', tr('failed')); }
        finally { posting = false; controls(); }
    }
    async function cancel() {
        if (!active) return;
        const id = active.id; const epoch = generation;
        try { await request('/' + id + '/cancel', { method: 'POST' }); if (epoch === generation) { text('status', tr('status_cancelled')); await load(); } }
        catch (error) { if (epoch === generation && error.name !== 'AbortError') text('status', tr('failed')); }
    }
    document.addEventListener('click', event => {
        const button = event.target && event.target.closest && event.target.closest('[data-selftest-action]'); if (!button) return;
        const actions = { start, stop: cancel, refresh: load }; const action = actions[button.dataset.selftestAction]; if (action) void action();
    });
    document.addEventListener('change', event => { if (event.target.id === 'selftest-profile') el('bindings-panel').hidden = event.target.value !== 'targeted'; });
    document.addEventListener('visibilitychange', () => { if (document.hidden) stopPolling(); else poll(generation); });
    window.addEventListener('pagehide', () => { ++generation; stopPolling(); if (controller) controller.abort(); });
    window.AuraSelfTests = Object.freeze({ load });
})();
