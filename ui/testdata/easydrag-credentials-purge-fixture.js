// Browser-only fixture for the credential hint and purge dialog: it answers the scan and purge routes.
(function () {
    'use strict';
    const previous = window.fetch.bind(window), base = '/api/desktop/flows/credentials/';
    const reply = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const dirty = {
        generated_at: '2026-11-11T10:00:00Z', drafts: [{ flow_id: 'flow_cred1', name: 'Wetter', findings: 2 }], live: [], versions_purgeable: 3, versions_pinned: 1,
        runs: 4, steps: 5, test_rows: 1, backups: [], leftovers: [],
        snapshots: [{ name: '.flows-v7-backup-abc123', bytes: 4096, mod_time: '2026-10-01T10:00:00Z' }, { name: '.flows-v8-backup-def456', bytes: 8192, mod_time: '2026-11-01T10:00:00Z' }]
    };
    const state = { requests: [], purged: false, polls: 0 };
    window.fetch = async (value, options = {}) => {
        const url = new URL(String(value), location.origin), method = String(options.method || 'GET').toUpperCase();
        if (!url.pathname.startsWith(base)) return previous(value, options);
        const route = url.pathname.slice(base.length);
        state.requests.push({ route, method, body: options.body ? JSON.parse(options.body) : null });
        if (route === 'scan') return reply({ report: state.purged ? Object.assign({}, dirty, { drafts: [], versions_purgeable: 0, versions_pinned: 0, runs: 0, steps: 0, test_rows: 0, snapshots: [] }) : dirty });
        if (route === 'purge' && method === 'POST') { state.purged = true; return reply({ state: { status: 'running', step: 'backup' } }, 202); }
        if (route === 'purge') {
            state.polls++;
            return reply({ state: state.polls < 2 ? { status: 'running', step: 'redact' } : { status: 'done', clean: true, backup: '.flows-purge-backup-x.enc', remaining: [] } });
        }
        return reply({ code: 'FLOW_NOT_FOUND', error: 'unknown' }, 404);
    };
    window.edCredFixture = { state };
})();
