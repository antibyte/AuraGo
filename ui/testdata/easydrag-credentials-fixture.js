// Browser-only credential fixture: adds PARAM_LITERAL_CREDENTIAL (a typed api_key in an http.request
// address) to the validation answers and answers POST .../credentials/move the way the server does.
(function () {
    'use strict';
    const previous = window.fetch.bind(window);
    const base = '/api/desktop/flows';
    const state = { vault: {}, moves: [] };
    const reply = (value, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
    const queryHits = doc => {
        const out = [];
        (doc.nodes || []).forEach(n => {
            const url = n.type === 'http.request' && n.params && typeof n.params.url === 'string' ? n.params.url : '';
            (url.split('?')[1] || '').split('&').forEach(pair => {
                const [key, value] = pair.split('=');
                if (key === 'api_key' && value && !value.includes('{{') && value !== '[redacted]') out.push({ node: n, key });
            });
        });
        return out;
    };
    const issuesOf = doc => queryHits(doc).map(h => ({ code: 'PARAM_LITERAL_CREDENTIAL', severity: 'warning', node_id: h.node.id, param: 'url', location: 'url_query', path: h.key, movable: !h.node.params.auth_secret, message: 'typed credential' }));
    async function withIssues(response, doc) {
        const body = response.ok && doc ? await response.clone().json().catch(() => null) : null;
        if (!body || !Array.isArray(body.issues)) return response;
        return reply(Object.assign(body, { issues: body.issues.concat(issuesOf(doc)) }), response.status);
    }
    function move(rec, body) {
        if (body.revision !== rec.draft_revision) return reply({ code: 'FLOW_REVISION_CONFLICT', error: 'conflict' }, 409);
        if (state.vault[body.secret_name] !== undefined && !body.replace) return reply({ code: 'FLOW_SECRET_EXISTS', error: 'exists' }, 409);
        const node = rec.draft.nodes.find(n => n.id === body.node_id);
        const [head, query = ''] = node.params.url.split('?');
        const kept = [], taken = [];
        query.split('&').forEach(pair => (pair.split('=')[0] === body.path ? taken : kept).push(pair));
        state.vault[body.secret_name] = taken[0].split('=')[1];
        state.moves.push({ name: body.secret_name, node: body.node_id, path: body.path });
        node.params.url = kept.length ? head + '?' + kept.join('&') : head;
        Object.assign(node.params, { auth_secret: body.secret_name, auth_in: 'query', auth_query: body.path });
        rec.draft_revision += 1;
        return reply({ draft_revision: rec.draft_revision, doc: rec.draft, moved: [body.node_id], not_movable: [], issues: issuesOf(rec.draft) });
    }
    window.fetch = async (input, options) => {
        const url = new URL(String(input), location.origin), opts = options || {}, method = String(opts.method || 'GET').toUpperCase();
        if (!url.pathname.startsWith(base)) return previous(input, opts);
        const parts = url.pathname.slice(base.length).split('/').filter(Boolean).map(decodeURIComponent);
        const entry = parts[0] && window.edFixture.state.flows.get(parts[0]);
        if (method === 'POST' && entry && parts[1] === 'credentials' && parts[2] === 'move') return move(entry.rec, JSON.parse(opts.body));
        const response = await previous(input, opts);
        if (method === 'POST' && parts[0] === 'validate') return withIssues(response, JSON.parse(opts.body).doc);
        if (entry && (parts.length === 1 || parts[1] === 'publish-preview')) return withIssues(response, entry.rec.draft);
        return response;
    };
    window.edCredentialsFixture = { state };
})();
