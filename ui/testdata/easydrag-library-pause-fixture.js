// Status-only C13 library pause API fixture. It observes exact request order and can hold or fail
// one mutation reply so the native browser test exercises cancellation and uncertain outcomes.
(function () {
    'use strict';
    const previous = window.fetch.bind(window);
    const state = { requests: [], statuses: new Map(), holdNextPost: false, heldPost: null, loseNextPost: false, denyNextPost: false, unreadableNextGet: false };
    const reply = (value, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const fail = (status, code) => reply({ error: code, code }, status);
    const current = id => {
        if (state.statuses.has(id)) return state.statuses.get(id);
        const entry = window.edFixture.state.flows.get(id);
        return entry ? (entry.enabled ? 'enabled' : 'disabled') : 'missing';
    };
    const set = (id, value) => {
        state.statuses.set(id, value);
        const entry = window.edFixture.state.flows.get(id);
        if (entry && (value === 'enabled' || value === 'disabled')) entry.enabled = value === 'enabled';
    };
    window.fetch = async (url, options) => {
        const opts = options || {}, u = new URL(String(url), location.origin);
        const match = /^\/api\/desktop\/flows\/([^/]+)\/enabled$/.exec(u.pathname);
        if (!match) return previous(url, opts);
        const id = decodeURIComponent(match[1]), method = String(opts.method || 'GET').toUpperCase();
        let body = {};
        try { body = opts.body ? JSON.parse(opts.body) : {}; } catch (_) { return fail(400, 'FLOW_BAD_REQUEST'); }
        state.requests.push({ method, id, body });
        if (!window.edFixture.state.flows.has(id)) return fail(404, 'FLOW_NOT_FOUND');
        if (method === 'GET') {
            if (state.unreadableNextGet) { state.unreadableNextGet = false; return fail(500, 'FLOW_INTERNAL'); }
            return reply({ mission_state: current(id) });
        }
        if (method !== 'POST' || body.enabled !== false) return fail(400, 'FLOW_BAD_REQUEST');
        if (state.denyNextPost) { state.denyNextPost = false; return fail(403, 'desktop_readonly'); }
        set(id, 'disabled');
        if (state.loseNextPost) { state.loseNextPost = false; return fail(500, 'FLOW_INTERNAL'); }
        if (state.holdNextPost) {
            state.holdNextPost = false;
            return new Promise(resolve => { state.heldPost = { id, resolve: () => { state.heldPost = null; resolve(reply({ enabled: false })); } }; });
        }
        return reply({ enabled: false });
    };
    window.edPauseFixture = { state, set, current, releaseHeldPost() { if (state.heldPost) state.heldPost.resolve(); } };
})();
