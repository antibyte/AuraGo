// Browser-only visual-group persistence wrapper for the existing flow and block fixtures.
(function () {
    'use strict';
    const previous = window.fetch.bind(window), clone = value => JSON.parse(JSON.stringify(value));
    const flowBase = '/api/desktop/flows', blockBase = '/api/desktop/flow-blocks';
    const requests = [], versions = new Map();
    const reply = (value, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const fail = (status, code, issues) => reply({ code, error: code, issues }, status);
    const key = (kind, id, revision) => kind + ':' + id + ':' + revision;
    function record(kind, id) {
        if (kind === 'block') return window.edBlockFixture.state.blocks.get(id);
        const entry = window.edFixture.state.flows.get(id); return entry && entry.rec;
    }
    function remember(kind, rec) {
        if (rec && rec.live && rec.live_revision && !versions.has(key(kind, rec.id, rec.live_revision))) versions.set(key(kind, rec.id, rec.live_revision), clone(rec.live));
    }
    function validText(text, runes, bytes, multiline) {
        if (typeof text !== 'string' || Array.from(text).length > runes || new TextEncoder().encode(text).length > bytes) return false;
        if (/[\uD800-\uDFFF]/u.test(text.replace(/[\uD800-\uDBFF][\uDC00-\uDFFF]/g, ''))) return false;
        return !(multiline ? /[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]/ : /[\x00-\x1f\x7f-\x9f]/).test(text);
    }
    function validGroups(doc) {
        if (doc.groups == null) return true;
        if (!Array.isArray(doc.groups) || doc.groups.length > 100) return false;
        const members = new Set(), ids = new Set(), nodes = new Set((doc.nodes || []).map(node => node.id));
        for (const group of doc.groups) {
            if (!group || !/^g_[a-z2-7]{8}$/.test(group.id) || ids.has(group.id) || !validText(group.title, 80, 320, false) || !group.title.trim() || !['neutral', 'blue', 'green', 'amber', 'rose', 'violet'].includes(group.color) || group.note != null && !validText(group.note, 2048, 8192, true) || !Array.isArray(group.members) || !group.members.length || group.members.length > 500) return false;
            ids.add(group.id);
            for (const id of group.members) { if (!nodes.has(id) || members.has(id)) return false; members.add(id); }
        }
        return true;
    }
    function changed(id) { setTimeout(() => document.dispatchEvent(new CustomEvent('aurago:flows-changed', { detail: { flow_id: id, reason: 'saved' } })), 0); }
    window.fetch = async (input, options) => {
        const url = new URL(String(input), location.origin), opts = options || {}, method = String(opts.method || 'GET').toUpperCase();
        const kind = url.pathname.startsWith(blockBase + '/') ? 'block' : url.pathname === flowBase || url.pathname.startsWith(flowBase + '/') ? 'flow' : '';
        if (!kind) return previous(input, opts);
        const base = kind === 'block' ? blockBase : flowBase, parts = url.pathname.slice(base.length).split('/').filter(Boolean).map(decodeURIComponent);
        let body; try { body = opts.body ? JSON.parse(opts.body) : {}; } catch (_) { return fail(400, 'FLOW_BAD_REQUEST'); }
        if (!parts.length && kind === 'flow' && method === 'POST' && body.import) {
            const doc = clone(body.import);
            if (![1, 2].includes(doc.schema)) return fail(400, 'FLOW_BAD_REQUEST');
            if (doc.schema === 2 && (!Array.isArray(doc.groups) || !doc.groups.length) || !validGroups(doc)) return fail(422, 'FLOW_INVALID', [{ code: 'GROUP_INVALID', severity: 'error' }]);
            doc.schema = 1; return previous(input, Object.assign({}, opts, { body: JSON.stringify(Object.assign({}, body, { import: doc })) }));
        }
        const id = parts[0], rec = record(kind, id); if (!rec) return previous(input, opts);
        if (method !== 'GET' && method !== 'HEAD') requests.push({ method, path: url.pathname, body: clone(body) });
        if (parts.length === 1 && method === 'PUT' && body.doc && body.base_revision === rec.draft_revision) {
            const doc = clone(body.doc);
            if (doc.schema !== 1) return fail(400, 'FLOW_BAD_REQUEST');
            if (doc.groups == null && rec.draft.groups != null) doc.groups = clone(rec.draft.groups);
            if (!validGroups(doc)) return fail(422, 'FLOW_INVALID', [{ code: 'GROUP_INVALID', severity: 'error' }]);
            return previous(input, Object.assign({}, opts, { body: JSON.stringify(Object.assign({}, body, { doc })) }));
        }
        if (parts.length === 2 && parts[1] === 'publish' && method === 'POST') {
            remember(kind, rec); const result = await previous(input, opts); if (result.ok) remember(kind, rec); return result;
        }
        if (kind === 'flow' && parts.length === 2 && parts[1] === 'export' && method === 'GET') {
            const response = await previous(input, opts); if (!response.ok) return response;
            const doc = await response.json(); if (Array.isArray(doc.groups) && doc.groups.length) doc.schema = 2;
            return new Response(JSON.stringify(doc), { status: response.status, headers: response.headers });
        }
        if (parts.length === 2 && parts[1] === 'versions') {
            remember(kind, rec);
            if (method === 'POST') {
                if (body.base_revision !== rec.draft_revision) return fail(409, 'FLOW_REVISION_CONFLICT');
                const doc = versions.get(key(kind, id, body.revision)); if (!doc) return fail(404, 'FLOW_NOT_FOUND');
                rec.draft = clone(doc); rec.draft_revision++; rec.name = rec.draft.name; changed(id);
                return reply({ [kind]: clone(rec), issues: [] });
            }
            if (method === 'GET' && kind === 'flow') {
                const rows = Array.from(versions.entries()).filter(([k]) => k.startsWith(key(kind, id, ''))).map(([k, doc]) => ({ revision: Number(k.split(':')[2]), published_at: rec.published_at, node_count: doc.nodes.length, edge_count: doc.edges.length, note_preview: '', note_revision: 0, note_redacted: false, note_editable: true })).sort((a, b) => b.revision - a.revision);
                return reply({ versions: rows, draft_revision: rec.draft_revision, live_revision: rec.live_revision });
            }
        }
        return previous(input, opts);
    };
    function addDraft(name, count) {
        if (!Number.isInteger(count) || count < 1 || count > 500) throw new Error('fixture node count');
        const id = window.edFixture.addEmptyDraft(name), doc = window.edFixture.flow(id).rec.draft;
        const alphabet = 'abcdefghijklmnopqrstuvwxyz234567';
        const suffix = value => { let out = ''; for (let i = 0; i < 8; i++) { out = alphabet[value % 32] + out; value = Math.floor(value / 32); } return out; };
        doc.nodes = Array.from({ length: count }, (_, index) => ({ id: 'n_' + suffix(index), key: index ? 'node_' + index : 'start', type: index ? 'logic.set' : 'trigger.manual', type_version: 1, label: index ? 'Step ' + index : 'Start', params: index ? { fields: [], keep_input: false } : {}, settings: {}, position: { x: (index % 20) * 280, y: Math.floor(index / 20) * (count === 50 ? 340 : 180) } }));
        doc.edges = []; return id;
    }
    window.edGroupsFixture = { requests, versions, addDraft };
})();
