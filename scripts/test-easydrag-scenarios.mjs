// Named scenarios use immutable server snapshots; fresh cases alone send editable sample data.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    await guardAsync('flagship scenarios replay protected snapshots without resending redacted data', async () => {
        const scenario = { id: 'sc_one', name: 'Private sample', version: 2, draft_revision: 3, stale: false, trigger_node: 'n_tttttttt', trigger_data: { credential: '[redacted]' }, fixtures: {}, assertions: [{ node_id: 'n_aaaaaaaa', output: { text: 'expected' } }] };
        const h = sandbox(req => {
            if (req.url.endsWith('/scenarios')) return { scenarios: [{ id: scenario.id, name: scenario.name, version: scenario.version }] };
            if (req.url.endsWith('/scenarios/sc_one')) return req.method === 'PATCH' ? { scenario: { id: scenario.id, name: 'Renamed', version: 3 } } : { scenario };
            if (req.url.endsWith('/simulate')) return { mode: 'simulation', revision: 3, status: 'success', started_at: '2026-11-04T12:00:00Z', steps: [{ node_id: 'n_aaaaaaaa', status: 'success', output: { text: 'actual' } }], assertions: [{ node_id: 'n_aaaaaaaa', passed: false, code: 'output_mismatch' }] };
        });
        const editor = openEditor(h); await settle();
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click'); await settle();
        const d = h.dialogs[h.dialogs.length - 1], mode = d.body.querySelector('[data-ed-test-mode]');
        mode.value = 'simulation'; mode.fire('change'); await settle();
        const picker = d.body.querySelector('[data-ed-scenario]'); picker.value = 'sc_one'; picker.fire('change'); await settle();
        eq('flagship scenario protects displayed samples and forces simulation', [d.body.querySelector('[data-ed-test-data]').readOnly, d.body.querySelector('[data-ed-simulation-fixtures]').readOnly, mode.disabled, mode.value], [true, true, true, 'simulation']);
        h.ctx.promptDialog = async () => 'Renamed';
        d.body.querySelector('[data-ed-scenario-rename]').fire('click'); await settle();
        eq('flagship scenario rename sends only name and observed version', h.requests.find(req => req.method === 'PATCH').body, { name: 'Renamed', expected_version: 2 });
        d.el.querySelector('[data-ed-action="run"]').fire('click'); await settle();
        eq('flagship stored replay sends only ID/version/revision and renders assertion failure', [h.requests.find(req => req.url.endsWith('/simulate')).body, editor.ed.run.record.assertions, editor.el.querySelector('.ed-run-results-body').html.includes('assertion_output_mismatch'), h.sources.length, h.requests.filter(req => req.method === 'POST' && req.url.endsWith('/test')).length, editor.ed.testEvidence.status], [{ base_revision: 3, scenario_id: 'sc_one', scenario_version: 3 }, [{ node_id: 'n_aaaaaaaa', passed: false, code: 'output_mismatch' }], true, 0, 0, 'error']);
        editor.dispose();
    });

    await guardAsync('flagship scenarios preserve nested values and save exact output assertions', async () => {
        let saved;
        const h = sandbox(req => {
            if (req.url.endsWith('/test-data/n_tttttttt')) return { data: { greeting: 'Hello', count: 2, enabled: true, nested: { untouched: 1 } } };
            if (req.url.endsWith('/scenarios') && req.method === 'GET') return { scenarios: saved ? [{ id: saved.id, name: saved.name, version: 1 }] : [] };
            if (req.url.endsWith('/scenarios') && req.method === 'POST') { saved = Object.assign({ id: 'sc_new', version: 1, draft_revision: 3 }, req.body.scenario); return { scenario: saved }; }
            if (req.url.endsWith('/scenarios/sc_new')) return { scenario: saved };
        });
        const editor = openEditor(h); await settle();
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click'); await settle();
        const d = h.dialogs[h.dialogs.length - 1], inputs = d.body.querySelectorAll('[data-ed-sample-field]');
        inputs[0].value = 'Hallo'; inputs[0].fire('input');
        inputs[1].value = ''; inputs[1].fire('input');
        d.el.querySelector('[data-ed-action="run"]').fire('click'); await settle();
        eq('flagship invalid structured numbers cannot run stale previous JSON', h.requests.filter(req => req.url.endsWith('/test')).length, 0);
        const mode = d.body.querySelector('[data-ed-test-mode]'); mode.value = 'simulation'; mode.fire('change'); await settle();
        d.body.querySelector('[data-ed-scenario-name]').value = 'Local greeting';
        d.body.querySelector('[data-ed-scenario-save-button]').fire('click'); await settle();
        eq('flagship failed scenario save preserves an invalid field buffer without storing prior JSON', [d.body.querySelectorAll('[data-ed-sample-field]')[1].value, h.requests.filter(req => req.url.endsWith('/scenarios') && req.method === 'POST').length], ['', 0]);
        inputs[1].value = '7'; inputs[1].fire('input');
        d.body.querySelector('[data-ed-assertion-add]').fire('click');
        d.body.querySelector('[data-ed-assertion-node]').value = 'n_aaaaaaaa';
        d.body.querySelector('[data-ed-assertion-output]').value = 'null';
        d.body.querySelector('[data-ed-assertion-ports]').value = 'out, matched';
        d.body.querySelector('[data-ed-scenario-name]').value = 'Local greeting';
        d.body.querySelector('[data-ed-scenario-save-button]').fire('click'); await settle();
        eq('flagship saved scenario retains nested data, typed fields and meaningful null assertion', h.requests.find(req => req.url.endsWith('/scenarios') && req.method === 'POST').body, { base_revision: 3, expected_version: 0, scenario: { name: 'Local greeting', trigger_node: 'n_tttttttt', trigger_data: { greeting: 'Hallo', count: 7, enabled: true, nested: { untouched: 1 } }, fixtures: {}, assertions: [{ node_id: 'n_aaaaaaaa', output: null, ports: ['out', 'matched'] }] } });
        eq('flagship completed save keeps every stored assertion control disabled', d.body.querySelectorAll('.ed-assertion-row input, .ed-assertion-row textarea, .ed-assertion-row select, .ed-assertion-row button').every(input => input.disabled), true);
        d.body.querySelector('[data-ed-scenario-new]').fire('click');
        eq('flagship fresh scenario clears private samples and unlocks editing', [d.body.querySelector('[data-ed-test-data]').value, d.body.querySelector('[data-ed-test-data]').readOnly, mode.disabled], ['{}', false, false]);
        d.close(); editor.dispose();
    });

    await guardAsync('flagship stale scenarios cannot start simulation', async () => {
        const h = sandbox(req => req.url.endsWith('/scenarios') ? { scenarios: [{ id: 'sc_old', name: 'Old', version: 1, stale: true }] } : req.url.endsWith('/scenarios/sc_old') ? { scenario: { id: 'sc_old', name: 'Old', version: 1, stale: true, trigger_node: 'n_tttttttt', trigger_data: {}, fixtures: {} } } : undefined);
        const editor = openEditor(h); await settle(); editor.el.querySelector('[data-ed-cmd="test"]').fire('click'); await settle();
        const d = h.dialogs[h.dialogs.length - 1], mode = d.body.querySelector('[data-ed-test-mode]'); mode.value = 'simulation'; mode.fire('change'); await settle();
        const picker = d.body.querySelector('[data-ed-scenario]'); picker.value = 'sc_old'; picker.fire('change'); await settle();
        d.el.querySelector('[data-ed-action="run"]').fire('click'); await settle();
        eq('flagship stale private cases explain the next action without execution', [h.requests.filter(req => req.url.endsWith('/simulate')).length, d.body.querySelector('.ed-error').textContent], [0, 'scenario_stale']);
        d.close(); editor.dispose();
    });
}
