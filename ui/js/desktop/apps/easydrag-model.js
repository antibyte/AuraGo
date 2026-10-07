// EasyDrag client document model: ports and graph queries, change sets with undo/redo,
// node keys and the clipboard. Pure (no DOM); the server stays authoritative on validation.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    const RESERVED = new Set(['trigger', 'run', 'flow', 'item', 'index', 'input', 'env', 'vars', 'secrets']);
    const KEY_PATTERN = /^[a-z][a-z0-9_]{0,39}$/;
    const ALPHABET = 'abcdefghijklmnopqrstuvwxyz234567';
    const COALESCE_MS = 1200;
    const MAX_HISTORY = 300;
    const MAX_PASTE_NODES = 500; // flows.MaxNodes
    const MAX_PASTE_EDGES = 2000; // flows.MaxEdges

    function randomSuffix(n) {
        const bytes = new Uint8Array(n);
        globalThis.crypto.getRandomValues(bytes);
        let out = '';
        for (let i = 0; i < n; i++) out += ALPHABET[bytes[i] % ALPHABET.length];
        return out;
    }

    function newNodeID() { return 'n_' + randomSuffix(8); }
    function newEdgeID() { return 'e_' + randomSuffix(8); }

    const UMLAUTS = { 'ä': 'ae', 'ö': 'oe', 'ü': 'ue', 'ß': 'ss', 'Ä': 'ae', 'Ö': 'oe', 'Ü': 'ue' };

    // keyFromLabel mirrors flows.KeyFromLabel: lower case, a-z0-9 and "_", unique, not reserved.
    function keyFromLabel(label, taken) {
        const s = String(label || '').trim().replace(/[äöüßÄÖÜ]/g, ch => UMLAUTS[ch]).toLowerCase();
        let key = '';
        let lastUnderscore = false;
        for (const ch of s) {
            if ((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) { key += ch; lastUnderscore = false; continue; }
            if (!lastUnderscore && key.length) { key += '_'; lastUnderscore = true; }
        }
        key = key.replace(/^_+|_+$/g, '') || 'node';
        if (key[0] >= '0' && key[0] <= '9') key = 'n_' + key;
        if (key.length > 36) key = key.slice(0, 36).replace(/_+$/, '');
        if (RESERVED.has(key)) key += '_node';
        const used = taken instanceof Set ? taken : new Set(taken || []);
        let candidate = key;
        for (let i = 2; used.has(candidate); i++) candidate = key + '_' + i;
        return candidate;
    }

    function keyProblem(key) {
        if (!KEY_PATTERN.test(key)) return 'invalid';
        if (RESERVED.has(key)) return 'reserved';
        return '';
    }

    function clone(value) { return value === undefined ? undefined : JSON.parse(JSON.stringify(value)); }
    function isPlainObject(v) { return !!v && typeof v === 'object' && !Array.isArray(v); }
    function finite(v) { return typeof v === 'number' && isFinite(v) ? v : 0; }

    // minPosition takes the smallest x and y of nodes with a loop (a spread can overflow the stack).
    function minPosition(nodes) {
        let x = Infinity;
        let y = Infinity;
        nodes.forEach(n => { x = Math.min(x, n.position.x); y = Math.min(y, n.position.y); });
        return { x: isFinite(x) ? x : 0, y: isFinite(y) ? y : 0 };
    }
    function same(a, b) { return JSON.stringify(a) === JSON.stringify(b); }
    function isEmpty(v) { return v === undefined || v === null || (typeof v === 'string' && v.trim() === '') || (Array.isArray(v) && !v.length); }

    // normalize fills in defaults. It keeps only plain-object nodes and edges whose ends are
    // plain objects, because a tampered or corrupt emergency copy can reach replaceDoc.
    function normalize(doc) {
        const d = isPlainObject(doc) ? doc : {};
        d.schema = d.schema || 1;
        d.kind = d.kind || 'flow';
        d.nodes = Array.isArray(d.nodes) ? d.nodes.filter(isPlainObject) : [];
        d.edges = Array.isArray(d.edges) ? d.edges.filter(e => isPlainObject(e) && isPlainObject(e.source) && isPlainObject(e.target)) : [];
        d.settings = d.settings || {};
        d.nodes.forEach(n => { n.params = n.params || {}; n.settings = n.settings || {}; n.position = n.position || { x: 0, y: 0 }; });
        return d;
    }

    const META_KEYS = ['name', 'description', 'settings'];

    function create(doc, catalog) {
        const listeners = new Set();
        const undoStack = [];
        const redoStack = [];
        const state = { doc: normalize(clone(doc)), version: 0 };

        function info(type) {
            if (!catalog) return null;
            if (catalog.types && typeof catalog.types.get === 'function') return catalog.types.get(type) || null;
            return (catalog.types && catalog.types[type]) || null;
        }

        function node(id) { return state.doc.nodes.find(n => n.id === id) || null; }
        function edge(id) { return state.doc.edges.find(e => e.id === id) || null; }
        function byKey(key) { return state.doc.nodes.find(n => n.key === key) || null; }

        function inputs(n) {
            const i = info(n.type);
            if (!i) return ['in'];
            return Array.isArray(i.inputs) ? i.inputs.slice() : [];
        }

        function outputs(n) {
            const i = info(n.type);
            let ports;
            if (i && i.dynamic_outputs === 'cases') {
                const cases = Array.isArray(n.params && n.params.cases) ? n.params.cases : [];
                ports = cases.map((_, idx) => 'case_' + (idx + 1)).concat(['default']);
            } else if (i) {
                ports = Array.isArray(i.outputs) ? i.outputs.filter(p => p !== 'error') : [];
            } else {
                ports = ['out'];
            }
            if (n.settings && n.settings.on_error === 'error_port') ports.push('error');
            return ports;
        }

        // fieldsOf lists the output fields of a node (dynamic fields of AI steps and merges included).
        function fieldsOf(n) {
            const i = info(n.type);
            if (!i) return [];
            if (i.dynamic_fields === 'mode') {
                // logic.merge: "append" outputs items and count; every other mode outputs one
                // entry per direct upstream node, keyed by that node's key (internal/flows/nodes_logic.go).
                if (n.params && n.params.mode === 'append') return [{ name: 'items', type: 'list', primary: true }, { name: 'count', type: 'number' }];
                const seen = new Set();
                const out = [];
                incoming(n.id).forEach(e => {
                    const src = node(e.source.node);
                    if (!src || !src.key || seen.has(src.key)) return;
                    seen.add(src.key);
                    out.push({ name: src.key, type: 'object' });
                });
                return out;
            }
            if (i.dynamic_fields === 'fields') {
                const defs = Array.isArray(n.params && n.params.fields) ? n.params.fields : [];
                const own = defs.filter(f => f && f.name).map((f, idx) => ({ name: String(f.name), type: f.type || 'text', primary: idx === 0 }));
                if (n.params && n.params.output_mode === 'fields' && own.length) return own.concat([{ name: 'tokens', type: 'object' }, { name: 'model', type: 'text' }]);
            }
            return Array.isArray(i.output_fields) ? i.output_fields.slice() : [];
        }

        function incoming(id) { return state.doc.edges.filter(e => e.target.node === id); }
        function outgoing(id) { return state.doc.edges.filter(e => e.source.node === id); }

        function walk(start, next) {
            const seen = new Set();
            const queue = [start];
            while (queue.length) {
                const id = queue.shift();
                next(id).forEach(other => { if (!seen.has(other)) { seen.add(other); queue.push(other); } });
            }
            seen.delete(start);
            return seen;
        }

        function upstream(id) { return walk(id, x => incoming(x).map(e => e.source.node)); }
        function downstream(id) { return walk(id, x => outgoing(x).map(e => e.target.node)); }
        function keys() { return new Set(state.doc.nodes.map(n => n.key)); }

        function canConnect(src, port, dst, inPort) {
            const a = node(src);
            const b = node(dst);
            if (!a || !b) return { ok: false, reason: 'missing' };
            if (src === dst) return { ok: false, reason: 'self' };
            if (!outputs(a).includes(port) || !inputs(b).includes(inPort)) return { ok: false, reason: 'port' };
            if (state.doc.edges.some(e => e.source.node === src && e.source.port === port && e.target.node === dst && e.target.port === inPort)) {
                return { ok: false, reason: 'duplicate' };
            }
            if (downstream(dst).has(src)) return { ok: false, reason: 'cycle' };
            return { ok: true, reason: '' };
        }

        // ── change sets ──────────────────────────────────────────────────────────

        // indexOf maps the ids of list to their positions, for lookups without a scan per id. The
        // first item wins, as findIndex does, should a corrupt document hold an id twice.
        function indexOf(list) {
            const at = new Map();
            list.forEach((item, i) => { if (!at.has(item.id)) at.set(item.id, i); });
            return at;
        }

        // recorder collects one change set. The document does not change while a command records
        // (applySet runs after finish), so each list's id index is built once, on first use.
        function recorder() {
            const nodes = new Map();
            const edges = new Map();
            let meta = null;
            let nodeAt = null;
            let edgeAt = null;
            function touchNode(id) {
                if (!nodes.has(id)) {
                    if (!nodeAt) nodeAt = indexOf(state.doc.nodes);
                    const idx = nodeAt.has(id) ? nodeAt.get(id) : -1;
                    nodes.set(id, { before: idx >= 0 ? clone(state.doc.nodes[idx]) : null, index: idx, after: undefined });
                }
                return nodes.get(id);
            }
            function touchEdge(id) {
                if (!edges.has(id)) {
                    if (!edgeAt) edgeAt = indexOf(state.doc.edges);
                    const idx = edgeAt.has(id) ? edgeAt.get(id) : -1;
                    edges.set(id, { before: idx >= 0 ? clone(state.doc.edges[idx]) : null, index: idx, after: undefined });
                }
                return edges.get(id);
            }
            const rec = {
                node(id) {
                    const entry = touchNode(id);
                    if (entry.after === undefined) entry.after = clone(entry.before);
                    return entry.after;
                },
                addNode(n) { touchNode(n.id).after = n; return n; },
                removeNode(id) {
                    touchNode(id).after = null;
                    state.doc.edges.concat(Array.from(edges.values()).map(e => e.after).filter(Boolean))
                        .filter(e => e.source.node === id || e.target.node === id)
                        .forEach(e => rec.removeEdge(e.id));
                },
                edge(id) {
                    const entry = touchEdge(id);
                    if (entry.after === undefined) entry.after = clone(entry.before);
                    return entry.after;
                },
                addEdge(e) { touchEdge(e.id).after = e; return e; },
                removeEdge(id) { touchEdge(id).after = null; },
                meta(patch) {
                    if (!meta) {
                        const before = {};
                        Object.keys(patch).forEach(k => { before[k] = clone(state.doc[k]); });
                        meta = { before, after: {} };
                    }
                    Object.keys(patch).forEach(k => {
                        if (!(k in meta.before)) meta.before[k] = clone(state.doc[k]);
                        meta.after[k] = clone(patch[k]);
                    });
                },
                // view returns the node as it will look after this change set (for queries inside commands).
                view(id) {
                    const entry = nodes.get(id);
                    if (entry && entry.after !== undefined) return entry.after;
                    return node(id);
                },
                finish() {
                    const nodeList = [];
                    nodes.forEach((v, id) => {
                        const after = v.after === undefined ? v.before : v.after;
                        if (!same(v.before, after)) nodeList.push({ id, before: v.before, after, index: v.index });
                    });
                    const edgeList = [];
                    edges.forEach((v, id) => {
                        const after = v.after === undefined ? v.before : v.after;
                        if (!same(v.before, after)) edgeList.push({ id, before: v.before, after, index: v.index });
                    });
                    const metaEntry = meta && !same(meta.before, meta.after) ? meta : null;
                    if (!nodeList.length && !edgeList.length && !metaEntry) return null;
                    return { nodes: nodeList, edges: edgeList, meta: metaEntry };
                }
            };
            return rec;
        }

        // setItems applies entries to list. Ids are looked up in an index that a replacement keeps
        // valid; an insert or removal shifts positions, so the index is built again on next use.
        // A move of the whole selection (replacements only) therefore costs one pass.
        function setItems(list, entries, pick) {
            let at = null;
            entries.forEach(e => {
                if (!at) at = indexOf(list);
                const pos = at.has(e.id) ? at.get(e.id) : -1;
                const value = pick(e);
                if (value === null) {
                    if (pos >= 0) { list.splice(pos, 1); at = null; }
                    return;
                }
                const copy = clone(value);
                if (pos >= 0) list[pos] = copy;
                else if (e.index >= 0 && e.index <= list.length) { list.splice(e.index, 0, copy); at = null; }
                else { list.push(copy); at.set(e.id, list.length - 1); }
            });
        }

        function applySet(set, dir) {
            const pick = entry => dir === 'after' ? entry.after : entry.before;
            setItems(state.doc.nodes, dir === 'after' ? set.nodes : set.nodes.slice().reverse(), pick);
            setItems(state.doc.edges, dir === 'after' ? set.edges : set.edges.slice().reverse(), pick);
            if (set.meta) Object.assign(state.doc, clone(dir === 'after' ? set.meta.after : set.meta.before));
        }

        // restoreOrder puts list back into the id order recorded before a change; items the
        // snapshot does not know keep their relative order at the end.
        function restoreOrder(list, ids) {
            const rank = new Map(ids.map((id, i) => [id, i]));
            const ranked = list.map((item, i) => ({ item, r: rank.has(item.id) ? rank.get(item.id) : ids.length + i }));
            ranked.sort((a, b) => a.r - b.r);
            ranked.forEach((x, i) => { list[i] = x.item; });
        }

        function emit(change) {
            state.version += 1;
            listeners.forEach(fn => { try { fn(change); } catch (err) { console.error('EasyDrag model listener failed', err); } });
        }

        function describeSet(set, kind) {
            return {
                kind,
                nodes: set.nodes.map(e => e.id),
                edges: set.edges.map(e => e.id),
                meta: !!set.meta,
                structural: set.nodes.some(e => !e.before || !e.after) || set.edges.length > 0
            };
        }

        // change runs fn(rec) and records the result as one undo step. opts.coalesce merges
        // consecutive steps with the same key (dragging, typing).
        function change(label, fn, opts) {
            const rec = recorder();
            const result = fn(rec);
            const set = rec.finish();
            if (!set) return result;
            // The id order before the change lets undo restore the exact document order.
            set.order = { nodes: state.doc.nodes.map(n => n.id), edges: state.doc.edges.map(e => e.id) };
            applySet(set, 'after');
            const now = Date.now();
            const top = undoStack[undoStack.length - 1];
            const coalesce = opts && opts.coalesce;
            if (coalesce && top && top.coalesce === coalesce && now - top.at < COALESCE_MS) {
                mergeInto(top, set);
                top.at = now;
            } else {
                undoStack.push(Object.assign({ label, coalesce, at: now }, set));
                if (undoStack.length > MAX_HISTORY) undoStack.shift();
            }
            redoStack.length = 0;
            emit(describeSet(set, 'change'));
            return result;
        }

        // mergeInto folds a coalesced change into the top undo step; top keeps its own (earliest) order
        // snapshot. The entries of top are looked up by id in a Map, not one scan per entry.
        function mergeInto(top, set) {
            const fold = (list, entries) => {
                const byId = new Map();
                list.forEach(x => { if (!byId.has(x.id)) byId.set(x.id, x); });
                entries.forEach(e => {
                    const prev = byId.get(e.id);
                    if (prev) prev.after = e.after;
                    else { list.push(e); byId.set(e.id, e); }
                });
            };
            fold(top.nodes, set.nodes);
            fold(top.edges, set.edges);
            if (set.meta) {
                if (!top.meta) top.meta = set.meta;
                else Object.keys(set.meta.after).forEach(k => {
                    if (!(k in top.meta.before)) top.meta.before[k] = set.meta.before[k];
                    top.meta.after[k] = set.meta.after[k];
                });
            }
        }

        function undo() {
            const set = undoStack.pop();
            if (!set) return false;
            applySet(set, 'before');
            if (set.order) {
                restoreOrder(state.doc.nodes, set.order.nodes);
                restoreOrder(state.doc.edges, set.order.edges);
            }
            redoStack.push(set);
            emit(describeSet(set, 'undo'));
            return true;
        }

        function redo() {
            const set = redoStack.pop();
            if (!set) return false;
            applySet(set, 'after');
            undoStack.push(Object.assign(set, { coalesce: null }));
            emit(describeSet(set, 'redo'));
            return true;
        }

        // ── commands ─────────────────────────────────────────────────────────────

        function defaults(i) {
            const out = {};
            (i && i.params || []).forEach(p => { if (p.default !== undefined && p.default !== null) out[p.name] = clone(p.default); });
            return out;
        }

        // prefill maps the source's primary output into the new target: files go into the first
        // file parameter, every other value into the primary input.
        function prefill(rec, srcId, dstId) {
            const src = rec.view(srcId);
            const dst = rec.view(dstId);
            const dstInfo = info(dst.type);
            const field = fieldsOf(src).find(f => f.primary);
            if (!dstInfo || !field) return;
            const params = dstInfo.params || [];
            const spec = field.type === 'file'
                ? params.find(p => p.kind === 'file')
                : params.find(p => p.name === dstInfo.primary_input && p.kind !== 'file');
            if (!spec || !isEmpty(dst.params[spec.name])) return;
            rec.node(dstId).params[spec.name] = '{{' + src.key + '.' + field.name + '}}';
        }

        // addNode adds a node; from ({node, port}) connects it to an output in the same undo step.
        function addNode(type, position, params, from) {
            const i = info(type);
            const id = newNodeID();
            change('add', rec => {
                const label = (i && i.label) || type;
                rec.addNode({
                    id, key: keyFromLabel(label, keys()), type, type_version: (i && i.version) || 1, label,
                    position: { x: Math.round(position.x), y: Math.round(position.y) },
                    params: Object.assign(defaults(i), clone(params) || {}), settings: {}
                });
                const inPort = i && Array.isArray(i.inputs) ? i.inputs[0] : 'in';
                if (from && inPort && node(from.node) && outputs(node(from.node)).includes(from.port)) {
                    rec.addEdge({ id: newEdgeID(), source: { node: from.node, port: from.port }, target: { node: id, port: inPort } });
                    prefill(rec, from.node, id);
                }
            });
            return id;
        }

        function removeNodes(ids, opts) {
            const list = (ids || []).filter(id => node(id));
            if (!list.length) return;
            change('remove', rec => {
                let bridge = null;
                if (opts && opts.bridge && list.length === 1) {
                    const ins = incoming(list[0]);
                    const outs = outgoing(list[0]);
                    if (ins.length === 1 && outs.length === 1) bridge = { src: ins[0].source, dst: outs[0].target };
                }
                list.forEach(id => rec.removeNode(id));
                const exists = bridge && state.doc.edges.some(e => e.source.node === bridge.src.node && e.source.port === bridge.src.port
                    && e.target.node === bridge.dst.node && e.target.port === bridge.dst.port);
                if (bridge && !exists && bridge.src.node !== bridge.dst.node && !downstream(bridge.dst.node).has(bridge.src.node)) {
                    rec.addEdge({ id: newEdgeID(), source: clone(bridge.src), target: clone(bridge.dst) });
                }
            });
        }

        function moveNodes(ids, dx, dy) {
            if (!dx && !dy) return;
            const present = new Set(state.doc.nodes.map(n => n.id));
            const list = (ids || []).filter(id => present.has(id));
            change('move', rec => {
                list.forEach(id => {
                    const n = rec.node(id);
                    n.position = { x: Math.round(n.position.x + dx), y: Math.round(n.position.y + dy) };
                });
            }, { coalesce: 'move:' + list.slice().sort().join(',') });
        }

        function connect(src, port, dst, inPort) {
            const check = canConnect(src, port, dst, inPort);
            if (!check.ok) return { ok: false, reason: check.reason };
            const id = newEdgeID();
            change('connect', rec => {
                rec.addEdge({ id, source: { node: src, port }, target: { node: dst, port: inPort } });
                prefill(rec, src, dst);
            });
            return { ok: true, id };
        }

        function disconnect(ids) {
            change('disconnect', rec => { (ids || []).forEach(id => { if (edge(id)) rec.removeEdge(id); }); });
        }

        // insertOnEdge places a node (new type or existing id) between the two ends of an edge.
        function insertOnEdge(edgeId, typeOrId, position) {
            const e = edge(edgeId);
            if (!e) return null;
            let id = node(typeOrId) ? typeOrId : null;
            const i = id ? info(node(id).type) : info(typeOrId);
            if (!i || !(i.inputs || []).length) return null;
            change('insert', rec => {
                if (!id) {
                    id = newNodeID();
                    const label = i.label || typeOrId;
                    rec.addNode({ id, key: keyFromLabel(label, keys()), type: typeOrId, type_version: i.version || 1, label,
                        position: { x: Math.round(position.x), y: Math.round(position.y) }, params: defaults(i), settings: {} });
                } else if (position) {
                    rec.node(id).position = { x: Math.round(position.x), y: Math.round(position.y) };
                }
                const view = rec.view(id);
                const outPort = outputs(view)[0];
                rec.removeEdge(edgeId);
                rec.addEdge({ id: newEdgeID(), source: clone(e.source), target: { node: id, port: (i.inputs || ['in'])[0] } });
                if (outPort) rec.addEdge({ id: newEdgeID(), source: { node: id, port: outPort }, target: clone(e.target) });
                prefill(rec, e.source.node, id);
            });
            return id;
        }

        function setParam(id, name, value) {
            if (!node(id)) return;
            change('param', rec => {
                const n = rec.node(id);
                if (value === undefined) delete n.params[name]; else n.params[name] = clone(value);
                if (name === 'cases') dropDanglingEdges(rec, id);
            }, { coalesce: 'param:' + id + ':' + name });
        }

        function dropDanglingEdges(rec, id) {
            const ports = outputs(rec.view(id));
            outgoing(id).forEach(e => { if (!ports.includes(e.source.port)) rec.removeEdge(e.id); });
        }

        function setLabel(id, label) {
            if (!node(id)) return;
            change('label', rec => { rec.node(id).label = String(label).slice(0, 80); }, { coalesce: 'label:' + id });
        }

        // setKey renames a node key and rewrites every template reference in one step.
        function setKey(id, key) {
            const n = node(id);
            if (!n) return { ok: false, reason: 'missing' };
            if (key === n.key) return { ok: true };
            const problem = keyProblem(key);
            if (problem) return { ok: false, reason: problem };
            if (byKey(key)) return { ok: false, reason: 'taken' };
            const oldKey = n.key;
            change('key', rec => {
                rec.node(id).key = key;
                state.doc.nodes.forEach(other => {
                    const renamed = ED.template.renameInValue(other.params, oldKey, key);
                    if (!same(renamed, other.params)) rec.node(other.id).params = renamed;
                });
            });
            return { ok: true };
        }

        function setSettings(id, patch) {
            if (!node(id)) return;
            change('settings', rec => {
                const n = rec.node(id);
                n.settings = Object.assign({}, n.settings, clone(patch));
                Object.keys(n.settings).forEach(k => { if (n.settings[k] === undefined || n.settings[k] === null || n.settings[k] === '') delete n.settings[k]; });
                dropDanglingEdges(rec, id);
            }, { coalesce: 'settings:' + id + ':' + Object.keys(patch).join(',') });
        }

        function toggleDisabled(ids) {
            const list = (ids || []).map(node).filter(Boolean);
            if (!list.length) return;
            const disable = list.some(n => !n.settings.disabled);
            change('disable', rec => {
                list.forEach(n => {
                    const w = rec.node(n.id);
                    if (disable) w.settings.disabled = true; else delete w.settings.disabled;
                });
            });
        }

        function setFlow(patch) {
            const clean = {};
            Object.keys(patch || {}).forEach(k => { if (META_KEYS.includes(k)) clean[k] = patch[k]; });
            change('flow', rec => rec.meta(clean), { coalesce: 'flow:' + Object.keys(clean).join(',') });
        }

        function fragment(ids) {
            const set = new Set(ids || []);
            return {
                easydrag: 1,
                nodes: state.doc.nodes.filter(n => set.has(n.id)).map(clone),
                edges: state.doc.edges.filter(e => set.has(e.source.node) && set.has(e.target.node)).map(clone)
            };
        }

        // sanitizeFragment keeps what paste can trust from clipboard JSON: at most 500 object
        // nodes of a known type with unique ids, plain params and settings and finite positions,
        // and at most 2000 edges between kept nodes on existing ports, without duplicates or
        // cycles. A fragment over either limit is refused (null).
        function sanitizeFragment(frag) {
            if (!isPlainObject(frag) || frag.easydrag !== 1 || !Array.isArray(frag.nodes)) return null;
            const rawEdges = Array.isArray(frag.edges) ? frag.edges : [];
            if (frag.nodes.length > MAX_PASTE_NODES || rawEdges.length > MAX_PASTE_EDGES) return null;
            const nodes = [];
            const byRef = new Map();
            frag.nodes.forEach(n => {
                if (!isPlainObject(n) || typeof n.type !== 'string' || !info(n.type)) return;
                const ref = typeof n.id === 'string' ? n.id : null;
                if (ref !== null && byRef.has(ref)) return;
                const pos = isPlainObject(n.position) ? n.position : {};
                const clean = {
                    ref, key: typeof n.key === 'string' ? n.key : '', type: n.type,
                    type_version: Number.isInteger(n.type_version) && n.type_version > 0 ? n.type_version : 1,
                    label: typeof n.label === 'string' && n.label ? n.label.slice(0, 80) : (info(n.type).label || n.type),
                    position: { x: finite(pos.x), y: finite(pos.y) },
                    params: isPlainObject(n.params) ? clone(n.params) : {},
                    settings: isPlainObject(n.settings) ? clone(n.settings) : {}
                };
                if (ref !== null) byRef.set(ref, clean);
                nodes.push(clean);
            });
            const edges = [];
            const seen = new Set();
            const next = new Map();
            const reaches = (from, to) => {
                const queue = [from];
                const visited = new Set(queue);
                while (queue.length) {
                    const cur = queue.shift();
                    if (cur === to) return true;
                    (next.get(cur) || []).forEach(x => { if (!visited.has(x)) { visited.add(x); queue.push(x); } });
                }
                return false;
            };
            rawEdges.forEach(e => {
                if (!isPlainObject(e) || !isPlainObject(e.source) || !isPlainObject(e.target)) return;
                const a = byRef.get(e.source.node);
                const b = byRef.get(e.target.node);
                if (!a || !b || a === b) return;
                if (!outputs(a).includes(e.source.port) || !inputs(b).includes(e.target.port)) return;
                const id = JSON.stringify([a.ref, e.source.port, b.ref, e.target.port]);
                if (seen.has(id) || reaches(b, a)) return;
                seen.add(id);
                if (!next.has(a)) next.set(a, []);
                next.get(a).push(b);
                edges.push({ a, port: e.source.port, b, inPort: e.target.port });
            });
            return { nodes, edges };
        }

        // paste inserts a clipboard fragment with fresh ids and keys; references between
        // pasted nodes follow the new keys. Returns the new node ids.
        function paste(frag, at) {
            const clean = sanitizeFragment(frag);
            if (!clean || !clean.nodes.length) return [];
            const min = minPosition(clean.nodes);
            const ax = finite(at && at.x);
            const ay = finite(at && at.y);
            const taken = keys();
            const usedOld = new Set();
            // renames maps each fragment key that changes to its new key; renameRoots applies it
            // once per expression, so a -> a_2 and a_2 -> a_2_2 cannot chain.
            const renames = new Map();
            clean.nodes.forEach(n => {
                const own = n.key && !keyProblem(n.key) && !usedOld.has(n.key);
                // Keyless, invalid or duplicate keys get a fresh key from the label, like addNode.
                n.newKey = keyFromLabel(own ? n.key : n.label, taken);
                taken.add(n.newKey);
                n.newId = newNodeID();
                if (own) {
                    usedOld.add(n.key);
                    if (n.newKey !== n.key) renames.set(n.key, n.newKey);
                }
            });
            change('paste', rec => {
                clean.nodes.forEach(n => {
                    rec.addNode({
                        id: n.newId, key: n.newKey, type: n.type, type_version: n.type_version, label: n.label,
                        position: { x: Math.round(ax + n.position.x - min.x), y: Math.round(ay + n.position.y - min.y) },
                        params: renames.size ? ED.template.renameRootsInValue(n.params, renames) : n.params,
                        settings: n.settings
                    });
                });
                clean.edges.forEach(e => {
                    rec.addEdge({ id: newEdgeID(), source: { node: e.a.newId, port: e.port }, target: { node: e.b.newId, port: e.inPort } });
                });
            });
            return clean.nodes.map(n => n.newId);
        }

        function duplicate(idList) {
            const frag = fragment(idList);
            if (!frag.nodes.length) return [];
            const min = minPosition(frag.nodes.map(n => ({ position: { x: finite(n.position.x), y: finite(n.position.y) } })));
            return paste(frag, { x: min.x + 40, y: min.y + 40 });
        }

        // replaceDoc swaps in a document from the server (conflict reload) and clears history.
        function replaceDoc(doc) {
            state.doc = normalize(clone(doc));
            undoStack.length = 0;
            redoStack.length = 0;
            emit({ kind: 'reset', nodes: [], edges: [], meta: true, structural: true });
        }

        return {
            get doc() { return state.doc; },
            get version() { return state.version; },
            info, node, edge, byKey, inputs, outputs, fieldsOf, incoming, outgoing, upstream, downstream, keys, canConnect,
            change, undo, redo,
            canUndo: () => undoStack.length > 0,
            canRedo: () => redoStack.length > 0,
            on(fn) { listeners.add(fn); return () => listeners.delete(fn); },
            toJSON: () => clone(state.doc),
            addNode, removeNodes, moveNodes, connect, disconnect, insertOnEdge, setParam, setLabel, setKey, setSettings,
            toggleDisabled, setFlow, fragment, paste, duplicate, replaceDoc
        };
    }

    ED.model = { create, keyFromLabel, keyProblem, newNodeID, newEdgeID, RESERVED };
})();
