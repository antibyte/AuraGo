// Browser-only C6 metadata service, layered after the flow and block fixtures.
(function () {
    'use strict';
    const previous = window.fetch.bind(window);
    const flowBase = '/api/desktop/flows', blockBase = '/api/desktop/flow-blocks', folderBase = '/api/desktop/flow-folders';
    const state = { folders: new Map(), libraries: new Map(), notes: new Map(), requests: [], secrets: [], partialNext: false, revoked: false, writeFailure: '' };
    const history = new Map(); let sequence = 0;
    const clone = value => JSON.parse(JSON.stringify(value));
    const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
    const reply = (value, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const fail = (status, code) => reply({ code, error: code }, status);
    const key = (kind, id, revision) => kind + ':' + id + ':' + revision;
    const scrub = text => state.secrets.reduce((value, secret) => value.split(secret).join('[redacted]'), text);
    const sensitive = text => typeof text !== 'string' || text.includes('[redacted]') || scrub(text) !== text;
    function noteValue(kind, id, revision) {
        const original = state.notes.get(key(kind, id, revision)) || { note: '', revision: 0 };
        const note = scrub(original.note), redacted = note !== original.note;
        return { note, revision: original.revision, redacted, editable: !redacted };
    }
    function noteSummary(kind, id, revision) {
        const result = noteValue(kind, id, revision), line = Array.from(result.note.split(/\r?\n/, 1)[0]);
        return { note_preview: line.slice(0, 120).join('') + (line.length > 120 ? '…' : ''), note_revision: result.revision, note_redacted: result.redacted, note_editable: result.editable };
    }
    function libraryValue(id) {
        const raw = state.libraries.get(id) || { folder_id: null, tags: [], revision: 0 };
        const tags = raw.tags.map(scrub), redacted = tags.some((tag, index) => tag !== raw.tags[index]);
        return { folder_id: raw.folder_id, tags, revision: raw.revision, tags_redacted: redacted, tags_editable: !redacted };
    }
    function folderValue(folder) {
        const name = scrub(folder.name), redacted = name !== folder.name;
        return { id: folder.id, name, revision: folder.revision, flow_count: Array.from(state.libraries.values()).filter(meta => meta.folder_id === folder.id).length, redacted, editable: !redacted };
    }
    function changed(id, reason) { setTimeout(() => document.dispatchEvent(new CustomEvent('aurago:flows-changed', { detail: { flow_id: id, reason } })), 0); }
    function record(kind, id) {
        if (kind === 'block') return window.edBlockFixture.state.blocks.get(id);
        const entry = window.edFixture.state.flows.get(id); return entry && entry.rec;
    }
    function remember(kind, rec) {
        if (!rec || !rec.live || !rec.live_revision) return;
        const id = key(kind, rec.id, rec.live_revision);
        if (!history.has(id)) history.set(id, { revision: rec.live_revision, published_at: rec.published_at, node_count: rec.live.nodes.length, edge_count: rec.live.edges.length });
    }
    function validText(value, runes, bytes, multiline) {
        return typeof value === 'string' && Array.from(value).length <= runes && new TextEncoder().encode(value).length <= bytes && !/[\uD800-\uDFFF]/u.test(value.replace(/[\uD800-\uDBFF][\uDC00-\uDFFF]/g, '')) && !(multiline ? /[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]/ : /[\x00-\x1f\x7f-\x9f]/).test(value);
    }
    function validated(value, runes, bytes, multiline) { return validText(value, runes, bytes, multiline) ? (sensitive(value) ? 'FLOW_LIBRARY_SENSITIVE' : '') : 'FLOW_LIBRARY_INVALID'; }
    function folders(method, id, body) {
        if (!id && method === 'GET') return reply({ folders: Array.from(state.folders.values()).map(folderValue) });
        if (!id && method === 'POST') {
            const code = validated(body.name, 80, 320); if (code || !body.name.trim() || state.folders.size >= 1000) return fail(400, code || 'FLOW_LIBRARY_INVALID');
            if (Array.from(state.folders.values()).some(folder => folder.name === body.name.trim())) return fail(409, 'FLOW_LIBRARY_CONFLICT');
            const folder = { id: 'folder_' + String(++sequence).padStart(10, '2'), name: body.name.trim(), revision: 1 }; state.folders.set(folder.id, folder); changed('', 'folders'); return reply({ folder: folderValue(folder) }, 201);
        }
        const folder = state.folders.get(id); if (!folder) return fail(404, 'FLOW_NOT_FOUND');
        if (body.base_revision !== folder.revision) return fail(409, 'FLOW_LIBRARY_CONFLICT');
        if (method === 'PUT') {
            const code = validated(body.name, 80, 320); if (code || !body.name.trim()) return fail(400, code || 'FLOW_LIBRARY_INVALID');
            if (Array.from(state.folders.values()).some(other => other.id !== id && other.name === body.name.trim())) return fail(409, 'FLOW_LIBRARY_CONFLICT');
            folder.name = body.name.trim(); folder.revision++; changed('', 'folders'); return reply({ folder: folderValue(folder) });
        }
        if (method === 'DELETE') { if (folderValue(folder).flow_count) return fail(409, 'FLOW_FOLDER_NOT_EMPTY'); state.folders.delete(id); changed('', 'folders'); return reply({ status: 'deleted' }); }
        return fail(405, 'FLOW_METHOD_NOT_ALLOWED');
    }
    function library(method, id, body) {
        if (!record('flow', id)) return fail(404, 'FLOW_NOT_FOUND');
        if (method === 'GET') return reply({ library: libraryValue(id) });
        if (method !== 'PUT' || !Number.isInteger(body.base_revision) || body.base_revision < 0 || !own(body, 'folder_id') && !own(body, 'tags')) return fail(400, 'FLOW_LIBRARY_INVALID');
        const meta = state.libraries.get(id) || { folder_id: null, tags: [], revision: 0 };
        if (body.base_revision !== meta.revision) return fail(409, 'FLOW_LIBRARY_CONFLICT');
        if (own(body, 'folder_id') && body.folder_id !== null && !state.folders.has(body.folder_id)) return fail(404, 'FLOW_NOT_FOUND');
        if (own(body, 'tags')) {
            if (!Array.isArray(body.tags) || body.tags.length > 16) return fail(400, 'FLOW_LIBRARY_INVALID');
            const tags = body.tags.map(tag => typeof tag === 'string' ? tag.trim() : tag);
            for (const tag of tags) { const code = validated(tag, 40, 160); if (code || !tag) return fail(400, code || 'FLOW_LIBRARY_INVALID'); }
            if (new Set(tags).size !== tags.length) return fail(400, 'FLOW_LIBRARY_INVALID');
            meta.tags = tags;
        }
        if (own(body, 'folder_id')) meta.folder_id = body.folder_id;
        meta.revision++; state.libraries.set(id, meta); changed(id, 'library'); return reply({ library: libraryValue(id) });
    }
    function versionNote(method, kind, id, revision, body) {
        const rec = record(kind, id); remember(kind, rec);
        if (!rec || !Number.isInteger(revision) || revision <= 0 || !history.has(key(kind, id, revision)) && !(kind === 'block' && window.edBlockFixture.state.versions.has(id + ':' + revision))) return fail(404, 'FLOW_NOT_FOUND');
        if (method === 'GET') return reply(noteValue(kind, id, revision));
        if (method !== 'PUT' || !Number.isInteger(body.base_revision) || body.base_revision < 0) return fail(400, 'FLOW_LIBRARY_INVALID');
        const code = validated(body.note, 2048, 4096, true); if (code) return fail(400, code);
        const current = state.notes.get(key(kind, id, revision)) || { note: '', revision: 0 };
        if (current.revision !== body.base_revision) return fail(409, 'FLOW_VERSION_NOTE_CONFLICT');
        state.notes.set(key(kind, id, revision), { note: body.note, revision: current.revision + 1 }); changed(id, 'version_note'); return reply(noteValue(kind, id, revision));
    }
    window.fetch = async (input, options) => {
        const url = new URL(String(input), location.origin), opts = options || {}, method = String(opts.method || 'GET').toUpperCase();
        const isFolders = url.pathname === folderBase || url.pathname.startsWith(folderBase + '/');
        const kind = url.pathname === blockBase || url.pathname.startsWith(blockBase + '/') ? 'block' : url.pathname === flowBase || url.pathname.startsWith(flowBase + '/') ? 'flow' : '';
        if (!isFolders && !kind) return previous(input, opts);
        let body; try { body = opts.body ? JSON.parse(opts.body) : {}; } catch (_) { return fail(400, 'FLOW_BAD_REQUEST'); }
        state.requests.push({ method, path: url.pathname + url.search, body: clone(body) });
        if (state.revoked) return fail(403, 'desktop_scope_required');
        if (method !== 'GET' && method !== 'HEAD' && state.writeFailure) { const code = state.writeFailure; state.writeFailure = ''; return fail(403, code); }
        if (isFolders) return folders(method, decodeURIComponent(url.pathname.slice(folderBase.length).replace(/^\//, '')), body);
        const base = kind === 'block' ? blockBase : flowBase, parts = url.pathname.slice(base.length).split('/').filter(Boolean).map(decodeURIComponent), id = parts[0], rec = record(kind, id);
        if (kind === 'flow' && parts.length === 2 && parts[1] === 'library') return library(method, id, body);
        if (parts.length === 4 && parts[1] === 'versions' && parts[3] === 'note') return versionNote(method, kind, id, Number(parts[2]), body);
        if (parts.length === 2 && parts[1] === 'publish' && method === 'POST' && rec) {
            remember(kind, rec); const before = rec.live_revision, same = before > 0 && rec.published_draft_revision === rec.draft_revision;
            if (own(body, 'note')) { const code = validated(body.note, 2048, 4096, true); if (code) return fail(400, code); if (body.base_revision === rec.draft_revision && same && body.note !== (state.notes.get(key(kind, id, before)) || { note: '' }).note) return fail(409, 'FLOW_VERSION_NOTE_CONFLICT'); }
            const oldPartial = window.edFixture.state.publishPartial;
            if (kind === 'flow' && state.partialNext) window.edFixture.state.publishPartial = 'FLOW_PUBLISH_INCOMPLETE';
            let result; try { result = await previous(input, opts); } finally { window.edFixture.state.publishPartial = oldPartial; state.partialNext = false; }
            if (result.ok) { remember(kind, rec); if (rec.live_revision !== before && own(body, 'note')) state.notes.set(key(kind, id, rec.live_revision), { note: body.note, revision: 1 }); }
            return result;
        }
        if (kind === 'flow' && parts.length === 2 && parts[1] === 'versions' && method === 'GET') {
            if (!rec) return fail(404, 'FLOW_NOT_FOUND'); remember(kind, rec);
            const versions = Array.from(history.entries()).filter(([k]) => k.startsWith(key(kind, id, ''))).map(([, item]) => Object.assign({}, item, noteSummary(kind, id, item.revision))).sort((a, b) => b.revision - a.revision);
            return reply({ versions, live_revision: rec.live_revision });
        }
        const result = await previous(input, opts);
        if (!result.ok || method !== 'GET') return result;
        if (parts.length === 2 && parts[1] === 'publish-preview' && rec) { const data = await result.json(); data.live_revision = rec.live_revision; data.published_draft_revision = rec.published_draft_revision; return reply(data, result.status); }
        if (kind === 'flow' && !parts.length) { const data = await result.json(); data.flows.forEach(item => { item.library = libraryValue(item.id); }); return reply(data, result.status); }
        if (kind === 'block' && parts.length === 2 && parts[1] === 'versions') { const data = await result.json(); data.versions.forEach(item => Object.assign(item, noteSummary(kind, id, item.revision))); return reply(data, result.status); }
        return result;
    };
    window.edMetadataFixture = {
        state,
        seedFolder(name) { const folder = { id: 'folder_' + String(++sequence).padStart(10, '2'), name, revision: 1 }; state.folders.set(folder.id, folder); return folder.id; },
        setLibrary(id, metadata) { state.libraries.set(id, clone(metadata)); },
        setNote(kind, id, revision, note) { state.notes.set(key(kind, id, revision), clone(note)); },
        maskText(text) { if (text && !state.secrets.includes(text)) state.secrets.push(text); },
        revoke() { state.revoked = true; }, restore() { state.revoked = false; }
    };
})();
