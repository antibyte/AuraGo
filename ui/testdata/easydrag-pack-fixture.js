// Browser-only local-pack protocol fixture; real validation and transactions are covered in Go.
(function () {
    'use strict';
    const previous = window.fetch.bind(window), clone = value => JSON.parse(JSON.stringify(value));
    const base = '/api/desktop/flows', format = 'aurago.easydrag.local-pack';
    const state = { requests: [], downloads: [], modes: { preview: 'ok', import: 'ok', reconcile: 'confirmed', export: 'ok' }, exportRevisionOffset: 0 };
    const proposals = new Map(), applied = new Map(), held = new Set(), releases = new Map();
    let sequence = 0;
    const hash = value => Number(value).toString(16).padStart(64, '0');
    const id = () => 'flow_' + String(++sequence).padStart(26, '0').replace(/[0-9]/g, digit => String.fromCharCode(97 + Number(digit)));
    const key = (flowID, revision) => flowID + ':' + revision;
    const reply = (body, status = 200, headers = {}) => new Response(JSON.stringify(body), { status,
        headers: Object.assign({ 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }, headers) });
    const fail = (status, code) => reply({ code, error: code }, status);
    const node = (name, type, params, x) => ({ id: 'n_' + name.padEnd(8, 'a'), key: name, type, type_version: 1,
        label: '', params: params || {}, settings: {}, position: { x, y: 100 } });
    const edge = (left, right, n) => ({ id: 'e_' + String.fromCharCode(97 + n).padStart(8, 'a'), source: { node: left.id, port: 'out' }, target: { node: right.id, port: 'in' } });

    function blockDoc(flowID, name, nested) {
        const input = node('input', 'block.input', { fields: [{ name: 'name', type: 'text', required: true }] }, 0);
        const output = node('output', 'block.output', { fields: [{ name: 'greeting', type: 'text', required: true }],
            values: { greeting: nested ? '{{child.greeting}}' : 'Hallo {{input.data.name}}' } }, 440);
        const nodes = [input], edges = [];
        if (nested) {
            const child = node('child', 'component.call', { flow_id: nested, revision: 1, inputs: { name: '{{input.data.name}}' } }, 220);
            nodes.push(child); edges.push(edge(input, child, 1), edge(child, output, 2));
        } else edges.push(edge(input, output, 1));
        nodes.push(output);
        return { schema: 1, kind: 'block', id: flowID, name, description: '', settings: {}, nodes, edges };
    }

    function makePack(options = {}) {
        const trigger = node('start', 'trigger.manual', { data: {} }, 0);
        const result = node('result', 'logic.set', { fields: [{ name: 'greeting', type: 'text', value: 'Hallo {{trigger.data.name}}' }], keep_input: false }, 600);
        const doc = { schema: 1, kind: 'flow', id: 'flow_packsource', name: 'Lokale Begrüßung', description: '', settings: {}, nodes: [trigger], edges: [] };
        const blocks = [];
        if (!options.empty) {
            const leaf = blockDoc('flow_packleaf', 'Begrüßung', null);
            const first = blockDoc('flow_packmiddle', 'Begrüßung vorbereiten', leaf.id);
            const second = clone(first); second.description = 'Zweite Quellversion';
            blocks.push({ source_id: leaf.id, source_revision: 1, hash: hash(1), flow: leaf },
                { source_id: first.id, source_revision: 1, hash: hash(2), flow: first },
                { source_id: second.id, source_revision: 2, hash: hash(3), flow: second });
            const a = node('first', 'component.call', { flow_id: first.id, revision: 1, inputs: { name: '{{trigger.data.name}}' } }, 200);
            const b = node('second', 'component.call', { flow_id: second.id, revision: 2, inputs: { name: '{{trigger.data.name}}' } }, 400);
            doc.nodes.push(a, b); doc.edges.push(edge(trigger, a, 1), edge(a, b, 2), edge(b, result, 3));
        } else doc.edges.push(edge(trigger, result, 1));
        doc.nodes.push(result);
        if (options.hostile) {
            doc.name = 'Prüfung <script>window.packInjected=true</script> $& $\' "';
            doc.description = '<img src=x onerror="window.packInjected=true"> {{literal}}';
        }
        if (options.long) result.params.literal = 'Lesbarer Inhalt '.repeat(20000);
        if (options.reordered) blocks.reverse();
        return { format, version: 1, parent: { hash: hash(4), source_revision: 1, flow: doc }, blocks };
    }

    function record(doc, revision, missionID) {
        const now = new Date().toISOString();
        const rec = { id: doc.id, kind: doc.kind, name: doc.name, description: doc.description || '', draft: clone(doc), draft_revision: revision,
            live_revision: doc.kind === 'block' ? 1 : 0, published_draft_revision: doc.kind === 'block' ? revision : 0,
            created_at: now, updated_at: now, published_at: doc.kind === 'block' ? now : '0001-01-01T00:00:00Z' };
        if (missionID) rec.mission_id = missionID;
        if (doc.kind === 'block') rec.live = clone(doc);
        return rec;
    }

    function seed(pack) {
        for (const block of pack.blocks) {
            const rec = record(block.flow, 1);
            rec.live_revision = block.source_revision;
            window.edBlockFixture.state.blocks.set(block.source_id, rec);
            window.edBlockFixture.state.versions.set(key(block.source_id, block.source_revision), { doc: clone(block.flow), published_at: rec.published_at });
        }
        const doc = pack.parent.flow, rec = record(doc, pack.parent.source_revision, 'mission_' + doc.id);
        window.edFixture.state.flows.set(doc.id, { rec, enabled: false });
        return doc.id;
    }

    function preview(pack) {
        const proposal = { pack_digest: hash(++sequence + 100), proposal_digest: hash(++sequence + 200), parent_id: id(), parent_hash: hash(++sequence + 300),
            blocks: pack.blocks.map(block => ({ source_id: block.source_id, source_revision: block.source_revision, local_id: id(), hash: hash(++sequence + 400) })) };
        const review = { parent: { name: pack.parent.flow.name, source_id: pack.parent.flow.id, source_revision: pack.parent.source_revision },
            blocks: pack.blocks.map(block => ({ name: block.flow.name, source_id: block.source_id, source_revision: block.source_revision })).reverse(),
            sample_data_cleared: true, local_only: true };
        proposals.set(proposal.proposal_digest, { pack: clone(pack), proposal: clone(proposal) });
        return { proposal, review };
    }

    function install(pack, proposal) {
        const targets = new Map(proposal.blocks.map(target => [key(target.source_id, target.source_revision), target]));
        function mapped(source, localID) {
            const doc = clone(source); doc.id = localID;
            for (const n of doc.nodes) if (n.type === 'component.call') {
                const target = targets.get(key(n.params.flow_id, n.params.revision));
                n.params.flow_id = target.local_id; n.params.revision = 1;
            }
            return doc;
        }
        const docs = new Map();
        for (const source of pack.blocks) {
            const target = targets.get(key(source.source_id, source.source_revision));
            docs.set(target.local_id, mapped(source.flow, target.local_id));
        }
        const hashes = new Map(proposal.blocks.map(target => [target.local_id, target.hash]));
        function pins(doc, found = new Set()) {
            for (const n of doc.nodes) if (n.type === 'component.call' && !found.has(n.params.flow_id)) {
                found.add(n.params.flow_id); pins(docs.get(n.params.flow_id), found);
            }
            return [...found].sort().map(blockID => ({ block_id: blockID, revision: 1, hash: hashes.get(blockID) }));
        }
        const blocks = proposal.blocks.map(target => {
            const doc = docs.get(target.local_id), rec = record(doc, 1);
            window.edBlockFixture.state.blocks.set(rec.id, rec);
            window.edBlockFixture.state.versions.set(key(rec.id, 1), { doc: clone(doc), published_at: rec.published_at });
            return { id: rec.id, kind: 'block', name: rec.name, draft_revision: 1, live_revision: 1, published_draft_revision: 1, hash: target.hash, pins: pins(doc) };
        }).reverse();
        const parent = mapped(pack.parent.flow, proposal.parent_id), rec = record(parent, 1, 'mission_' + parent.id);
        window.edFixture.state.flows.set(rec.id, { rec, enabled: false });
        const result = { status: 'applied', flow: { id: rec.id, kind: 'flow', name: rec.name, mission_id: rec.mission_id,
            draft_revision: 1, live_revision: 0, published_draft_revision: 0, hash: proposal.parent_hash, pins: [] }, blocks };
        applied.set(proposal.proposal_digest, clone(result));
        return result;
    }

    function exported(flowID) {
        const entry = window.edFixture.flow(flowID);
        if (!entry) return null;
        const doc = clone(entry.rec.draft), blocks = [], found = new Set();
        function visit(flow) {
            for (const n of flow.nodes) {
                if (n.type === 'trigger.manual') n.params.data = {};
                if (n.type !== 'component.call') continue;
                const ref = key(n.params.flow_id, n.params.revision);
                if (found.has(ref)) continue;
                found.add(ref);
                const child = clone(window.edBlockFixture.state.versions.get(ref).doc);
                visit(child); blocks.push({ source_id: child.id, source_revision: n.params.revision, hash: hash(blocks.length + 1), flow: child });
            }
        }
        visit(doc);
        return { format, version: 1, parent: { hash: hash(900), source_revision: entry.rec.draft_revision + state.exportRevisionOffset, flow: doc }, blocks };
    }

    function gate(kind) {
        if (!held.has(kind)) return Promise.resolve();
        return new Promise(resolve => { const list = releases.get(kind) || []; list.push(resolve); releases.set(kind, list); });
    }
    window.fetch = async (value, options = {}) => {
        const url = new URL(String(value), location.origin), method = String(options.method || 'GET').toUpperCase();
        const match = url.pathname.match(/^\/api\/desktop\/flows\/([^/]+)\/pack$/);
        const kind = match && method === 'GET' ? 'export' : url.pathname.startsWith(base + '/packs/') ? url.pathname.slice((base + '/packs/').length) : '';
        if (!['preview', 'import', 'reconcile', 'export'].includes(kind)) return previous(value, options);
        const body = options.body ? JSON.parse(options.body) : {};
        state.requests.push({ kind, method, path: url.pathname, body: clone(body) });
        await gate(kind); // Deliberately allows late replies after client abort to exercise disposal fences.
        const mode = state.modes[kind];
        if (mode === 'error') return fail(500, 'FLOW_INTERNAL');
        if (mode === 'refused') return fail(400, 'FLOW_PACK_INVALID');
        if (mode === 'forbidden') return fail(403, 'FLOW_PERMISSION_DENIED');
        if (kind === 'export') return reply(exported(decodeURIComponent(match[1])), 200, { 'Content-Disposition': 'attachment; filename="local.easydrag-pack.json"' });
        if (kind === 'preview') return reply(preview(body.pack));
        if (kind === 'import') {
            const known = proposals.get(body.proposal.proposal_digest);
            if (!known || JSON.stringify(known.pack) !== JSON.stringify(body.pack) || applied.has(body.proposal.proposal_digest)) return fail(409, 'FLOW_PACK_CONFLICT');
            const result = install(body.pack, body.proposal);
            if (mode === 'lost') throw new TypeError('Fixture import reply lost after commit');
            if (mode === 'malformed') result.flow.published_draft_revision = 1;
            return reply(result);
        }
        const result = applied.get(body.proposal.proposal_digest);
        return reply(mode === 'unknown' || !result ? { status: 'unknown', blocks: null } : Object.assign(clone(result), { status: 'applied_state_confirmed' }));
    };

    const blobs = new Map(), createURL = URL.createObjectURL.bind(URL), revokeURL = URL.revokeObjectURL.bind(URL);
    URL.createObjectURL = blob => { const url = createURL(blob); blobs.set(url, blob); return url; };
    URL.revokeObjectURL = url => { blobs.delete(url); revokeURL(url); };
    const click = HTMLAnchorElement.prototype.click;
    HTMLAnchorElement.prototype.click = function () {
        if (!this.download.endsWith('.easydrag-pack.json') || !blobs.has(this.href)) return click.call(this);
        const blob = blobs.get(this.href), item = { name: this.download, size: blob.size, text: '' };
        state.downloads.push(item); blob.text().then(text => { item.text = text; });
    };
    window.edPackFixture = { state, makePack, seed,
        hold(kind) { held.add(kind); },
        release(kind) { held.delete(kind); const list = releases.get(kind) || []; releases.delete(kind); list.forEach(resolve => resolve()); }
    };
})();
