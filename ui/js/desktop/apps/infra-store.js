// Infra window state: API client, snapshot polling and visibility, CAS writes,
// the per-view layout queue with one conflict rebase, and undo/redo. Nothing
// here touches the DOM outside attach(): scripts/test-infra-ui.mjs runs the
// queue against a fake API inside a Node vm.
(function () {
    'use strict';

    const API = '/api/desktop/infra';
    const VIEWS = ['atlas', 'topology'];
    const REQUEST_TIMEOUT = 20000, REPOLL_DELAY = 250;

    const edgeFields = edge => {
        const fields = { from: edge.from, to: edge.to, kind: edge.kind };
        for (const key of ['label', 'from_port', 'to_port', 'speed']) if (edge[key]) fields[key] = String(edge[key]);
        return fields;
    };
    const layoutCommand = (type, view, before, after, label) => ({
        type, view, before, after, label, optimistic: true, undoable: true,
        apply: store => store.queueLayout(view, after),
        revert: store => store.queueLayout(view, before),
    });
    // placeFromLibrary puts assets on a view that had no placement for them.
    // Undoing it queues {unplace: true} intents, which only a server with
    // unplace support understands; dispatch() calls prepare() so the command
    // knows that server and the previous state of every asset. A drop that
    // changed an asset's parent unplaces it with its previous parent_id.
    // Against an older server the command stays out of history. A caller may
    // pass the previous intents itself (`before`); they win per id, and the
    // rest is derived at dispatch.
    const placeFromLibrary = (view, after, label = '', before = null) => {
        const command = layoutCommand('PlaceFromLibrary', view, before || new Map(), after, label);
        command.undoable = !!before && after.size > 0 && [...after.keys()].every(id => !!before.get(id));
        command.prepare = store => {
            const prior = new Map();
            for (const [id, intent] of after) {
                const given = before?.get(id);
                if (given) { prior.set(id, given); continue; }
                const previous = store.currentIntent(id, view, store.features.layoutParent);
                if (previous) { prior.set(id, previous); continue; }
                const parent = store.effectiveParent(id, view);
                prior.set(id, intent.parent_id !== undefined && intent.parent_id !== parent ? { unplace: true, parent_id: parent } : { unplace: true });
            }
            command.before = prior;
            command.undoable = store.features.unplace === true;
        };
        command.revert = store => store.queueLayout(view, command.before);
        return command;
    };
    // Layout commands are optimistic: the overlay shows the result at once and
    // history records them at once.
    const commands = {
        moveNodes: (view, before, after, label = '') => layoutCommand('MoveNodes', view, before, after, label),
        reparent: (view, before, after, label = '') => layoutCommand('Reparent', view, before, after, label),
        resizePlace: (view, before, after, label = '') => layoutCommand('ResizePlace', view, before, after, label),
        placeFromLibrary,
        // unplace takes assets off the map; `before` holds their current intents so undo puts them back.
        unplace: (view, before, label = '') => layoutCommand('Unplace', view, before, new Map([...before.keys()].map(id => [id, { unplace: true }])), label),
        // setParent changes parent_id through the node PUT (assets that are not drawn inside the new place).
        setParent: (changes, label = '') => ({
            type: 'SetParent', label, undoable: true, changes: changes.map(change => ({ id: change.id, before: change.before || '', after: change.after || '' })),
            async apply(store) { return store.updateParents(this.changes.map(change => [change.id, change.after])); },
            async revert(store) { return store.updateParents(this.changes.map(change => [change.id, change.before])); },
        }),
        createEdge: fields => ({
            type: 'CreateEdge', fields: { ...fields }, edgeId: '', undoable: true,
            async apply(store) { const edge = await store.createEdge(this.fields); this.edgeId = edge?.id || ''; return !!this.edgeId; },
            // settle runs after an unconfirmed write was reloaded: did the connection arrive? Then undo can remove it.
            settle(store) { const found = store.state.edges.find(item => item.from === this.fields.from && item.to === this.fields.to && item.kind === this.fields.kind); this.edgeId = found?.id || ''; return !!this.edgeId; },
            async revert(store) { return store.deleteEdgeById(this.edgeId); },
        }),
        deleteEdge: edge => ({
            type: 'DeleteEdge', fields: edgeFields(edge), edgeId: edge.id, undoable: true,
            async apply(store) { return store.deleteEdgeById(this.edgeId); },
            settle(store) { return !store.state.edges.some(item => item.id === this.edgeId); },
            async revert(store) { const created = await store.createEdge(this.fields); this.edgeId = created?.id || ''; return !!this.edgeId; },
        }),
    };

    function create({ ctx = {}, t: translate, requestTimeout = REQUEST_TIMEOUT } = {}) {
        const model = window.InfraAppModel;
        const t = translate || (key => ctx.t?.(key) || key);
        const listeners = new Set();
        const features = { layoutParent: false, unplace: false };
        const lanes = {}, histories = {};
        for (const view of VIEWS) {
            lanes[view] = { pending: new Map(), inflight: null, unsaved: new Map(), waiters: [], scheduled: false };
            histories[view] = model.createHistory(50);
        }
        const state = {
            disposed: false, epoch: 1, visible: false, inView: true, pollPaused: false, timer: 0, writeSeq: 0, repollTimer: 0, repollExplicit: false,
            requests: new Set(), controller: new AbortController(), manifestController: null, manifestLoaded: false, manifestLoading: false,
            caps: { write: false, source_edit: false, control: false },
            nodes: [], edges: [], sources: [], sourceHosts: [], candidates: [],
            appTargets: [], targetsLoaded: false, targetsError: false,
            credentials: [], credentialsLoaded: false, credentialsLoading: false, credentialsError: false,
            stamp: '', refreshAt: 0, sync: 'syncing',
            view: 'atlas', tool: 'select', filters: { query: '', kind: '', source: '', power: '' },
            selection: new Set(), selectionOrigin: '', selected: '', selectedEdge: '', expandedHosts: new Set(), interacting: 0, historyBusy: false,
            drafts: new Map(), dirty: new Set(), draftRevision: new Map(), nodeReviews: new Map(), customRows: new Map(),
            sourceDrafts: new Map(), sourceDirty: new Set(), sourceRevision: new Map(), sourceReviews: new Map(), sourceBlocked: new Set(), sourcePending: new Set(),
            actionBlocks: new Map(), importSelection: new Set(), importBindings: new Map(), importDialogOwner: '', importOpen: false, dialogGeneration: 0,
            busy: new Set(),
        };
        let detach = () => {};
        let win = null;
        // Events: notice, busy, snapshot, caps, sync, selection, view, filters, tool, expanded, interaction-end,
        // layout, layout-status, history, nodes, edges, targets, manifest, credentials, and (Plan C) sources,
        // candidates, source-hosts. `sources` follows an ignore, restore or removal that ignored sources;
        // `candidates` a changed discovery list; `source-hosts` a changed engine host mapping.
        const emit = change => { if (!state.disposed) for (const fn of [...listeners]) fn(change); };
        const subscribe = fn => { listeners.add(fn); return () => listeners.delete(fn); };
        const notify = (message, kind = 'info') => emit({ type: 'notice', message, kind });
        const setBusy = (key, busy) => { if (busy) state.busy.add(key); else state.busy.delete(key); emit({ type: 'busy', key }); };
        // noteWrite counts every layout, node, edge, ignore and engine host write that was applied.
        // A snapshot request that started before the count moved may predate that write.
        const noteWrite = () => { state.writeSeq += 1; };

        // ---- permissions -----------------------------------------------------
        const canWrite = () => state.caps.write === true && ctx.readonly !== true;
        const canEditSource = () => state.caps.source_edit === true && ctx.readonly !== true;
        const canControl = () => state.caps.control === true && ctx.readonly !== true;
        const allowedAction = (node, action, source = '') => {
            if (!canControl()) return false;
            return !!source && Array.isArray(node?.source_actions?.[model.sourceIdentity(source)]) && node.source_actions[model.sourceIdentity(source)].includes(action);
        };
        const actionReason = (node, source, action) => node?.source_action_reasons?.[model.sourceIdentity(source)]?.[action] || '';
        const actionBlockKey = (nodeID, source, action) => `${nodeID}\x00${typeof source === 'string' ? source : model.sourceIdentity(source)}\x00${action}`;

        // ---- requests --------------------------------------------------------
        const abortRequests = () => {
            state.epoch++;
            clearTimeout(state.timer); state.timer = 0;
            clearTimeout(state.repollTimer); state.repollTimer = 0; state.repollExplicit = false;
            state.requests.forEach(request => request.abort());
            state.requests.clear();
            state.manifestController?.abort();
            state.busy.delete('snapshot');
            state.controller.abort();
            state.controller = new AbortController();
        };
        const request = async (path, options = {}, epoch = state.epoch) => {
            if (state.disposed) throw new DOMException('Disposed', 'AbortError');
            if (!ctx.api) throw new Error('unavailable');
            const controller = new AbortController();
            state.requests.add(controller);
            const relayAbort = () => controller.abort();
            state.controller.signal.addEventListener('abort', relayAbort, { once: true });
            let timedOut = false;
            const timeout = setTimeout(() => { timedOut = true; controller.abort(); }, requestTimeout);
            const opts = { ...options, signal: controller.signal };
            if (opts.body && typeof opts.body !== 'string') {
                opts.headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) };
                opts.body = JSON.stringify(opts.body);
            }
            try {
                const endpoint = path === '/api/desktop/integrations/credentials' ? path : API + path;
                const result = await ctx.api(endpoint, opts);
                if (state.disposed || epoch !== state.epoch) throw new DOMException('Stale request', 'AbortError');
                return result;
            } catch (error) {
                // Running out of time is a failure of its own, not the quiet abort of a hide or close.
                if (timedOut && error?.name === 'AbortError' && !state.disposed && epoch === state.epoch) throw Object.assign(new Error('timeout'), { name: 'TimeoutError' });
                throw error;
            } finally {
                clearTimeout(timeout);
                state.requests.delete(controller);
                state.controller.signal.removeEventListener('abort', relayAbort);
            }
        };
        const messageFor = error => {
            const code = String(error?.body?.error || error?.message || 'generic').toLowerCase().replace(/[^a-z0-9_]/g, '_');
            const key = `desktop.infra_error_${code}`;
            const text = t(key);
            if (text !== key) return text;
            return t(error?.status === 409 ? 'desktop.infra_error_conflict' : 'desktop.infra_error_infra_error');
        };
        // A write that was aborted, timed out or answered with a server error may still have been applied.
        const uncertainMutation = error => error?.name === 'AbortError' || error?.name === 'TimeoutError' || (Number.isFinite(Number(error?.status)) && Number(error.status) >= 500) || (error?.status == null && error?.message !== 'unavailable');
        const isAccessError = error => {
            const code = String(error?.body?.error || '').toLowerCase();
            return [401, 403].includes(Number(error?.status)) || ['disabled', 'desktop_disabled', 'infra_disabled', 'forbidden', 'unauthorized'].includes(code);
        };

        // ---- derived indexes (keyed by array identity; every change replaces the array)
        const nodeIndexCache = new WeakMap(), childrenCache = new WeakMap(), guestCache = new WeakMap();
        const nodeIndex = () => { let index = nodeIndexCache.get(state.nodes); if (!index) { index = new Map(state.nodes.map(node => [node.id, node])); nodeIndexCache.set(state.nodes, index); } return index; };
        const nodeById = id => nodeIndex().get(id);
        const childrenOf = id => {
            let index = childrenCache.get(state.nodes);
            if (!index) { index = new Map(); for (const node of state.nodes) if (node.parent_id) { const list = index.get(node.parent_id) || []; list.push(node); index.set(node.parent_id, list); } childrenCache.set(state.nodes, index); }
            return index.get(id) || [];
        };
        const guests = () => {
            const cached = guestCache.get(state.nodes);
            if (cached && cached.edges === state.edges) return cached.value;
            const value = model.guestIndex(state.nodes, state.edges);
            guestCache.set(state.nodes, { edges: state.edges, value });
            return value;
        };
        const searchableCache = new WeakMap();
        const searchableText = node => {
            const cached = searchableCache.get(node);
            if (cached !== undefined) return cached;
            const parts = [node.name, node.kind, node.power, node.profile?.vendor, node.profile?.model, node.profile?.serial, node.profile?.os, node.profile?.firmware,
                node.profile?.ip_address, node.profile?.mac_address, node.profile?.cidr, node.profile?.vlan, node.profile?.notes,
                ...(node.profile?.tags || []), ...(node.profile?.interfaces || []).flatMap(item => Object.values(item || {})),
                ...(node.profile?.links || []).flatMap(item => [item.label, item.url]),
                ...Object.entries(node.profile?.custom || {}).flatMap(([key, value]) => [key, value])];
            for (const source of node.sources || []) {
                parts.push(source.kind, source.instance, source.key, source.name);
                const observed = source.observed || {};
                for (const key of ['name', 'type', 'model', 'ip_address', 'hostname', 'state', 'status', 'node', 'vmid', 'role', 'os', 'description', 'power']) parts.push(observed[key]);
                for (const item of (Array.isArray(observed.interfaces) ? observed.interfaces : [])) if (item && typeof item === 'object') parts.push(item.name, item.ip_address, item.mac_address, item.network);
            }
            for (const binding of node.profile?.app_bindings || []) {
                const target = state.appTargets.find(item => item.app === binding.app && item.target_id === binding.target_id);
                parts.push(binding.app, binding.target_id, target?.label);
            }
            for (const link of node.app_links || []) parts.push(link.app, link.target_id, state.appTargets.find(item => item.app === link.app && item.target_id === link.target_id)?.label);
            const text = parts.filter(value => value != null).join(' ').toLocaleLowerCase();
            searchableCache.set(node, text);
            return text;
        };
        const matches = node => model.matchesFilters(node, state.filters, searchableText);

        // ---- snapshot ----------------------------------------------------------
        function setSync(next) {
            const previous = state.sync;
            if (previous === next) return;
            state.sync = next;
            const announce = next === 'error' && previous === 'live' ? t('desktop.infra_sync_stale') : next === 'live' && previous === 'error' ? t('desktop.infra_sync_recovered') : '';
            emit({ type: 'sync', announce });
        }
        function pruneSelection() {
            const removed = [...state.selection].filter(id => !nodeById(id));
            if (!removed.length && (!state.selectedEdge || state.edges.some(edge => edge.id === state.selectedEdge))) return;
            if (removed.some(id => state.dirty.has(id))) notify(t('desktop.infra_remote_removed_draft'), 'error');
            for (const id of removed) state.selection.delete(id);
            if (state.selectedEdge && !state.edges.some(edge => edge.id === state.selectedEdge)) state.selectedEdge = '';
            state.selected = state.selection.size === 1 ? [...state.selection][0] : '';
            emit({ type: 'selection' });
        }
        function applySnapshot(data, { explicit = false } = {}) {
            const previousCaps = JSON.stringify(state.caps);
            state.nodes = (data.nodes || []).map(model.normalizeNode);
            state.edges = Array.isArray(data.edges) ? data.edges : [];
            state.sources = Array.isArray(data.sources) ? data.sources : [];
            features.layoutParent = Array.isArray(data.source_hosts);
            features.unplace = features.layoutParent;
            state.sourceHosts = features.layoutParent ? data.source_hosts : [];
            state.caps = { write: false, source_edit: false, control: false, ...(data.capabilities || {}) };
            for (const node of state.nodes) {
                if (state.nodeReviews.has(node.id) && state.dirty.has(node.id)) state.nodeReviews.set(node.id, { latest: model.clone(node) });
                if (!state.dirty.has(node.id)) state.drafts.set(node.id, model.clone(node));
            }
            pruneSelection();
            const stamp = model.snapshotStamp(state.nodes, state.edges, state.sources, state.caps) + '\x1d' + JSON.stringify(state.sourceHosts);
            const structural = stamp !== state.stamp;
            state.stamp = stamp;
            state.refreshAt = Date.now();
            emit({ type: 'snapshot', structural, explicit });
            if (previousCaps !== JSON.stringify(state.caps)) emit({ type: 'caps' });
        }
        // scheduleRepoll asks again shortly; an explicit refresh stays explicit.
        function scheduleRepoll(explicit) {
            state.repollExplicit = state.repollExplicit || explicit;
            if (state.repollTimer || state.disposed) return;
            state.repollTimer = setTimeout(() => {
                const wantsExplicit = state.repollExplicit;
                state.repollTimer = 0; state.repollExplicit = false;
                void loadSnapshot(wantsExplicit);
            }, REPOLL_DELAY);
        }
        async function loadSnapshot(explicit = false) {
            if (state.disposed || !state.visible || state.busy.has('snapshot') || (state.pollPaused && !explicit)) return;
            const generation = state.epoch, writes = state.writeSeq;
            setBusy('snapshot', true);
            try {
                const data = await request(`/snapshot?${explicit ? 'refresh=1&' : ''}include_ignored=1`, {}, generation);
                state.pollPaused = false;
                // A write was applied while this answer was on its way. The answer may predate it,
                // and applying it would roll back the committed versions and positions.
                if (writes !== state.writeSeq) { scheduleRepoll(explicit); return; }
                applySnapshot(data, { explicit });
                setSync('live');
                schedulePoll();
            } catch (error) {
                if (error.name !== 'AbortError') {
                    if (isAccessError(error)) {
                        state.caps = { write: false, source_edit: false, control: false };
                        state.pollPaused = true; abortRequests(); emit({ type: 'caps' });
                        notify(t('desktop.infra_access_unavailable'), 'error'); setSync('paused');
                    } else {
                        // One notice when the outage starts (recovery announces itself); a refresh the user asked for always answers.
                        const entering = state.sync !== 'error';
                        if (entering || explicit) notify(messageFor(error), 'error');
                        setSync('error');
                    }
                }
            } finally { setBusy('snapshot', false); }
        }
        // refreshSnapshot reloads after a write or a conflict. An answer that raced another
        // write is asked for again (twice at most) instead of rolling that write back.
        async function refreshSnapshot() {
            try {
                for (let attempt = 0; ; attempt += 1) {
                    const writes = state.writeSeq;
                    const data = await request('/snapshot?include_ignored=1');
                    if (writes !== state.writeSeq && attempt < 2) continue;
                    applySnapshot(data);
                    return true;
                }
            } catch (error) { if (error?.name !== 'AbortError') notify(messageFor(error), 'error'); return false; }
        }
        async function loadTargets(force = false) {
            if (state.disposed || !state.visible || (state.targetsLoaded && !force)) return;
            try {
                const result = await request('/targets');
                state.appTargets = (result.targets || []).filter(target => ['network-cameras', 'software-store', 'meshcore'].includes(target.app) && target.target_id).map(target => ({ app: target.app, target_id: String(target.target_id), label: String(target.label || target.target_id) }));
                state.targetsLoaded = true; state.targetsError = false; emit({ type: 'targets' });
            } catch (error) {
                if (error.name !== 'AbortError') { state.targetsError = true; emit({ type: 'targets' }); }
            }
        }
        async function loadCredentials() {
            if (state.disposed || !state.visible || state.credentialsLoaded || state.credentialsLoading) return;
            state.credentialsLoading = true;
            try {
                const result = await request('/api/desktop/integrations/credentials');
                const records = Array.isArray(result) ? result : result.credentials || [];
                state.credentials = records.filter(item => item && typeof item.id === 'string' && item.id).map(item => ({ id: item.id, name: String(item.name || item.id), type: String(item.type || '') }));
                state.credentialsLoaded = true; state.credentialsError = false;
                if (state.selected && state.sourceDrafts.get(state.selected)?.source === 'registry') emit({ type: 'credentials' });
            } catch (error) {
                if (error.name !== 'AbortError') { state.credentialsError = true; if (state.selected && state.sourceDrafts.get(state.selected)?.source === 'registry') emit({ type: 'credentials' }); }
            } finally { state.credentialsLoading = false; }
        }
        async function loadIconManifest() {
            if (state.manifestLoaded || state.manifestLoading || state.disposed || !state.visible) return;
            state.manifestLoading = true;
            const controller = new AbortController(), generation = state.epoch;
            state.manifestController = controller;
            try {
                const response = await fetch(model.versionedAsset('/img/infra/manifest.json'), { signal: controller.signal, cache: 'force-cache' });
                if (!response.ok) throw new Error(`manifest ${response.status}`);
                const manifest = await response.json();
                if (state.disposed || !state.visible || generation !== state.epoch || !Array.isArray(manifest?.icons)) return;
                for (const entry of manifest.icons) {
                    const id = String(entry?.id || ''), path = String(entry?.path || '');
                    if (!model.acceptsIcon(id) || path !== `/img/infra/${id}.webp`) continue;
                    model.rememberIcon(id, path, typeof entry.label_key === 'string' ? entry.label_key : '');
                }
                state.manifestLoaded = true;
                emit({ type: 'manifest' });
            } catch (error) {
                if (error?.name !== 'AbortError') state.manifestLoaded = true;
            } finally {
                state.manifestLoading = false;
                if (state.manifestController === controller) state.manifestController = null;
                if (!state.manifestLoaded && !state.pollPaused && state.visible && !state.disposed && generation !== state.epoch) setTimeout(() => void loadIconManifest(), 0);
            }
        }

        const replaceNode = node => {
            noteWrite();
            const normalized = model.normalizeNode(node);
            const nodes = state.nodes.slice();
            const index = nodes.findIndex(item => item.id === normalized.id);
            if (index < 0) nodes.push(normalized); else nodes[index] = normalized;
            state.nodes = nodes;
            if (!state.dirty.has(normalized.id)) state.drafts.set(normalized.id, model.clone(normalized));
            emit({ type: 'nodes' });
            return normalized;
        };
        const removeNode = id => {
            noteWrite();
            state.nodes = state.nodes.filter(item => item.id !== id);
            state.edges = state.edges.filter(edge => edge.from !== id && edge.to !== id);
            state.drafts.delete(id); state.dirty.delete(id);
            for (const view of VIEWS) { lanes[view].pending.delete(id); lanes[view].unsaved.delete(id); }
            pruneSelection();
            emit({ type: 'nodes' }); emit({ type: 'edges' });
        };

        // ---- selection, view, filters --------------------------------------------
        // origin names the control that built the selection ('check' = List checkboxes); any other select resets it.
        function select(ids, { toggle = false, additive = false, origin = '' } = {}) {
            const next = toggle || additive ? new Set(state.selection) : new Set();
            for (const id of ids) { if (toggle && next.has(id)) next.delete(id); else if (nodeById(id)) next.add(id); }
            state.selection = next; state.selectedEdge = ''; state.selectionOrigin = origin;
            state.selected = next.size === 1 ? [...next][0] : '';
            if (state.selected && !state.dirty.has(state.selected)) state.drafts.set(state.selected, model.clone(nodeById(state.selected)));
            emit({ type: 'selection' });
        }
        const selectEdge = id => { state.selection = new Set(); state.selectionOrigin = ''; state.selected = ''; state.selectedEdge = state.edges.some(edge => edge.id === id) ? id : ''; emit({ type: 'selection' }); };
        const clearSelection = () => { if (!state.selection.size && !state.selectedEdge) return; state.selection = new Set(); state.selectionOrigin = ''; state.selected = ''; state.selectedEdge = ''; emit({ type: 'selection' }); };
        const setView = view => { if (state.view === view) return; state.view = view; emit({ type: 'view' }); };
        const setFilters = partial => { state.filters = { ...state.filters, ...partial }; emit({ type: 'filters' }); };
        const setTool = tool => { if (state.tool === tool) return; state.tool = tool; emit({ type: 'tool' }); };
        const toggleHost = id => { if (state.expandedHosts.has(id)) state.expandedHosts.delete(id); else state.expandedHosts.add(id); emit({ type: 'expanded' }); };
        const beginInteraction = () => { state.interacting += 1; };
        const endInteraction = () => { state.interacting = Math.max(0, state.interacting - 1); if (!state.interacting) emit({ type: 'interaction-end' }); };

        // ---- layout overlay -------------------------------------------------------
        const effectiveIntent = (view, id) => {
            const lane = lanes[view];
            if (!lane) return null;
            const parts = [lane.unsaved.get(id), lane.inflight?.get(id), lane.pending.get(id)].filter(Boolean);
            return parts.length ? parts.reduce((merged, part) => model.mergeIntent(merged, part), {}) : null;
        };
        // effectivePlacement is the stored placement under the queued intents;
        // null when the asset has none here, or an unplace is queued for it.
        function effectivePlacement(id, view) {
            const node = nodeById(id);
            if (!node) return null;
            const base = node.placements?.[view] || null;
            const intent = effectiveIntent(view, id);
            if (!intent) return base;
            if (intent.unplace) return null;
            const out = { ...(base || {}), x: intent.x, y: intent.y, pinned: intent.pinned !== false };
            if (intent.w != null && intent.h != null) { out.w = intent.w; out.h = intent.h; }
            return out;
        }
        function effectiveParent(id, view) {
            const intent = effectiveIntent(view, id);
            return intent && intent.parent_id !== undefined ? intent.parent_id : (nodeById(id)?.parent_id || '');
        }
        function currentIntent(id, view, withParent = false) {
            const node = nodeById(id), placement = effectivePlacement(id, view);
            if (!node || !placement) return null;
            const intent = { x: Number(placement.x), y: Number(placement.y), pinned: placement.pinned !== false };
            if (withParent) intent.parent_id = effectiveParent(id, view);
            if (model.isContainer(node.kind)) { intent.w = Number(placement.w) || model.REGION_W; intent.h = Number(placement.h) || model.REGION_H; }
            return intent;
        }
        const layoutState = (id, view) => { const lane = lanes[view]; if (!lane) return ''; if (lane.unsaved.has(id)) return 'unsaved'; return lane.pending.has(id) || lane.inflight?.has(id) ? 'saving' : ''; };
        const unsavedCount = view => lanes[view]?.unsaved.size || 0;
        const laneIdle = lane => !lane.inflight && !lane.pending.size;
        function settle(view) { const lane = lanes[view]; if (!laneIdle(lane)) return; const ok = !lane.unsaved.size; const waiters = lane.waiters.splice(0); waiters.forEach(resolve => resolve(ok)); }
        const whenLayoutIdle = view => { const lane = lanes[view]; return laneIdle(lane) ? Promise.resolve(!lane.unsaved.size) : new Promise(resolve => lane.waiters.push(resolve)); };
        function scheduleFlush(view) {
            const lane = lanes[view];
            if (lane.scheduled) return;
            lane.scheduled = true;
            Promise.resolve().then(() => { lane.scheduled = false; void flush(view); });
        }
        // queueLayout overlays intents at once and saves them in one PUT per view;
        // moves queued while a save is in flight coalesce into the next PUT.
        function queueLayout(view, intents) {
            const lane = lanes[view];
            if (!lane || !intents || !intents.size || !canWrite()) return Promise.resolve(false);
            for (const [id, intent] of intents) {
                const earlier = lane.unsaved.get(id);
                lane.unsaved.delete(id);
                lane.pending.set(id, model.mergeIntent(model.mergeIntent(earlier, lane.pending.get(id)), intent));
            }
            emit({ type: 'layout', view, ids: [...intents.keys()] });
            emit({ type: 'layout-status', view });
            const done = new Promise(resolve => lane.waiters.push(resolve));
            scheduleFlush(view);
            return done;
        }
        function moveToUnsaved(lane, batch) {
            for (const [id, intent] of batch) {
                if (lane.pending.has(id)) lane.pending.set(id, model.mergeIntent(intent, lane.pending.get(id)));
                else lane.unsaved.set(id, model.mergeIntent(lane.unsaved.get(id), intent));
            }
        }
        // alignDraft keeps a dirty draft saveable after our own layout write
        // bumped the node version (and, for a drop, changed its parent).
        function alignDraft(id, previousVersion, intent) {
            const node = nodeById(id), draft = state.drafts.get(id);
            if (!node || !draft) return;
            if (!state.dirty.has(id)) { state.drafts.set(id, model.clone(node)); return; }
            if (draft.version !== previousVersion) return;
            draft.version = node.version;
            if (intent?.parent_id !== undefined) draft.parent_id = node.parent_id;
        }
        function applyLayoutResult(result, view, batch) {
            noteWrite();
            const snapshot = model.layoutSnapshot(result);
            if (snapshot) { applySnapshot(snapshot, { explicit: true }); return; }
            const rows = Array.isArray(result?.nodes) ? result.nodes : Array.isArray(result) ? result : [];
            const before = new Map(rows.map(row => [row?.id, nodeById(row?.id)?.version]));
            model.applyLayoutPatch(state.nodes, rows, view);
            // An unplaced asset has no placement left, whatever shape the answer had.
            for (const [id, intent] of batch) if (intent.unplace) { const node = nodeById(id); if (node?.placements) delete node.placements[view]; }
            state.nodes = state.nodes.slice(); // a parent change invalidates the derived indexes
            for (const row of rows) alignDraft(row?.id, before.get(row?.id), batch.get(row?.id));
            emit({ type: 'nodes' });
        }
        async function sendBatch(view, batch) {
            const { items } = model.layoutItems(batch, nodeIndex(), { parent: features.layoutParent, view });
            if (!items.length) return 'ok';
            setBusy('layout', true);
            try {
                const result = await request('/layout', { method: 'PUT', body: { view, nodes: items } });
                applyLayoutResult(result, view, batch);
                return 'ok';
            } catch (error) {
                if (error?.status === 409) return 'conflict';
                if (error?.name !== 'AbortError') notify(messageFor(error), 'error');
                return 'failed';
            } finally { setBusy('layout', false); }
        }
        async function flush(view) {
            const lane = lanes[view];
            if (state.disposed || lane.inflight || !lane.pending.size) return;
            lane.inflight = lane.pending;
            lane.pending = new Map();
            let attempt = 0;
            for (;;) {
                const outcome = canWrite() ? await sendBatch(view, lane.inflight) : 'failed';
                if (outcome === 'conflict' && attempt === 0) {
                    attempt = 1;
                    histories[view].clear();
                    emit({ type: 'history' });
                    if (await refreshSnapshot()) {
                        const { kept, dropped } = model.rebaseLayout(lane.inflight, nodeIndex());
                        if (dropped.length) notify(t('desktop.infra_layout_dropped'), 'info');
                        lane.inflight = kept;
                        if (kept.size) continue;
                        break;
                    }
                }
                if (outcome !== 'ok') moveToUnsaved(lane, lane.inflight);
                break;
            }
            lane.inflight = null;
            emit({ type: 'layout', view });
            emit({ type: 'layout-status', view });
            if (lane.pending.size) scheduleFlush(view); else settle(view);
        }
        function retryLayout(view) {
            const lane = lanes[view];
            if (!lane.unsaved.size) return whenLayoutIdle(view);
            const intents = new Map(lane.unsaved);
            lane.unsaved.clear();
            return queueLayout(view, intents);
        }

        // ---- edges ------------------------------------------------------------
        async function createEdge(fields) {
            const result = await request('/edges', { method: 'POST', body: { id: '', version: 0, ...fields } });
            noteWrite();
            const edge = result?.edge || result;
            if (!edge?.id) { await refreshSnapshot(); return null; }
            state.edges = [...state.edges.filter(item => item.id !== edge.id), edge];
            emit({ type: 'edges' });
            return edge;
        }
        async function updateEdge(id, body) {
            const result = await request(`/edges/${encodeURIComponent(id)}`, { method: 'PUT', body });
            noteWrite();
            const edge = result?.edge || result;
            if (edge?.id) { state.edges = state.edges.map(item => (item.id === edge.id ? edge : item)); emit({ type: 'edges' }); }
            else await refreshSnapshot();
            return edge;
        }
        async function deleteEdgeById(id) {
            const edge = state.edges.find(item => item.id === id);
            if (!edge) return true;
            try { await request(`/edges/${encodeURIComponent(id)}?version=${encodeURIComponent(edge.version)}`, { method: 'DELETE' }); }
            catch (error) { if (error?.status !== 404) throw error; }
            noteWrite();
            state.edges = state.edges.filter(item => item.id !== id);
            if (state.selectedEdge === id) { state.selectedEdge = ''; emit({ type: 'selection' }); }
            emit({ type: 'edges' });
            return true;
        }

        // ---- sources, discoveries, removal and parents (Plan C) ---------------------
        const identityKey = source => `${source?.kind || ''}\x00${source?.instance || ''}\x00${source?.key || ''}`;
        function mergeSources(rows) {
            noteWrite();
            const byKey = new Map(state.sources.map(source => [identityKey(source), source]));
            for (const row of rows) byKey.set(identityKey(row), { ...(byKey.get(identityKey(row)) || {}), ...row });
            state.sources = [...byKey.values()];
        }
        const uncertainReload = async error => { await refreshSnapshot(); notify(t('desktop.infra_write_outcome_unknown'), 'error'); return error; };
        // writeFailed answers a rejected write: hiding or closing the window stays quiet, an unconfirmed
        // write reloads and says so, anything else shows its own message.
        async function writeFailed(error) {
            if (state.disposed || error?.name === 'AbortError') return;
            if (uncertainMutation(error)) await uncertainReload(error); else notify(messageFor(error), 'error');
        }
        function replaceNodes(list) {
            const at = new Map(state.nodes.map((node, index) => [node.id, index])), nodes = state.nodes.slice(), out = [];
            for (const raw of list || []) {
                if (!raw?.id) continue;
                const normalized = model.normalizeNode(raw);
                if (at.has(normalized.id)) nodes[at.get(normalized.id)] = normalized; else { at.set(normalized.id, nodes.length); nodes.push(normalized); }
                if (!state.dirty.has(normalized.id)) state.drafts.set(normalized.id, model.clone(normalized));
                out.push(normalized);
            }
            if (out.length) { noteWrite(); state.nodes = nodes; emit({ type: 'nodes' }); }
            return out;
        }
        // setSourcesIgnored ignores (or restores) sources in chunks of 1000 and
        // drops ignored discoveries from the import sheet.
        async function setSourcesIgnored(identities, ignored) {
            const items = (identities || []).filter(item => item?.kind && item.key).map(item => ({ kind: item.kind, instance: item.instance || '', key: item.key }));
            if (!items.length || !canWrite()) return false;
            setBusy('ignore', true);
            try {
                for (let start = 0; start < items.length; start += 1000) {
                    const chunk = items.slice(start, start + 1000);
                    const result = await request('/sources/ignore', { method: 'PUT', body: { sources: chunk, ignored: !!ignored } });
                    mergeSources(Array.isArray(result?.sources) ? result.sources : chunk.map(item => ({ ...item, ignored: !!ignored })));
                }
                const gone = new Set(items.map(identityKey));
                if (ignored) state.candidates = state.candidates.filter(candidate => !gone.has(identityKey(candidate.source)));
                emit({ type: 'sources' }); emit({ type: 'candidates' });
                return true;
            } catch (error) {
                if (error?.name === 'AbortError') return false;
                if (uncertainMutation(error)) await uncertainReload(error); else notify(messageFor(error), 'error');
                return false;
            } finally { setBusy('ignore', false); }
        }
        // setSourceHost assigns ('' clears) the device that runs a Docker engine
        // with the stored version, then reloads because host_id is a projection.
        async function setSourceHost(instance, nodeId) {
            const key = `source-host-${instance}`;
            if (!canWrite() || !instance || state.busy.has(key)) return false;
            const entry = state.sourceHosts.find(item => item.kind === 'docker' && item.instance === instance);
            setBusy(key, true);
            try {
                const result = await request('/source-hosts', { method: 'PUT', body: { kind: 'docker', instance, node_id: nodeId || '', version: Number(entry?.version) || 0 } });
                const saved = result?.source_host;
                noteWrite();
                if (saved?.instance) { state.sourceHosts = [...state.sourceHosts.filter(item => !(item.kind === saved.kind && item.instance === saved.instance)), { ...(entry || {}), ...saved }]; emit({ type: 'source-hosts' }); }
                await refreshSnapshot();
                return true;
            } catch (error) {
                if (error?.name === 'AbortError') return false;
                if (error?.status === 409) { await refreshSnapshot(); notify(messageFor(error), 'error'); }
                else if (uncertainMutation(error)) await uncertainReload(error);
                else notify(messageFor(error), 'error');
                return false;
            } finally { setBusy(key, false); }
        }
        // importBatch posts items in chunks of 500 and answers {nodes, errors,
        // uncertain}; imported discoveries leave state.candidates.
        async function importBatch(items) {
            const result = { nodes: [], errors: [], uncertain: false };
            if (!canWrite() || !items?.length) return result;
            setBusy('import', true);
            try {
                for (let start = 0; start < items.length; start += 500) {
                    const chunk = items.slice(start, start + 500);
                    let answer;
                    try { answer = await request('/import/batch', { method: 'POST', body: { items: chunk } }); }
                    catch (error) {
                        if (state.disposed) return result;
                        if (uncertainMutation(error)) { result.uncertain = true; await uncertainReload(error); return result; }
                        for (const item of chunk) result.errors.push({ candidate_id: item.candidate_id, error: String(error?.body?.error || 'infra_error'), message: messageFor(error) });
                        continue;
                    }
                    result.nodes.push(...replaceNodes(Array.isArray(answer?.nodes) ? answer.nodes : []));
                    for (const failure of Array.isArray(answer?.errors) ? answer.errors : []) result.errors.push({ candidate_id: String(failure.candidate_id || ''), error: String(failure.error || 'infra_error'), message: messageFor({ body: { error: failure.error } }) });
                }
                const failed = new Set(result.errors.map(item => item.candidate_id));
                const done = new Set(items.map(item => item.candidate_id).filter(id => !failed.has(id)));
                state.candidates = state.candidates.filter(candidate => !done.has(candidate.id));
                emit({ type: 'candidates' });
                return result;
            } finally { setBusy('import', false); }
        }
        // removeNodes deletes one by one (each with its version) and stops at the
        // first refusal; ignoreSources also ignores the linked sources.
        async function removeNodes(ids, { ignoreSources = false } = {}) {
            const removed = [];
            setBusy('remove', true);
            try {
                for (const id of ids || []) {
                    // The permission can go away while a confirmation is open or between two requests.
                    if (!canWrite()) { notify(t('desktop.infra_error_infra_readonly'), 'info'); break; }
                    const node = nodeById(id);
                    if (!node) continue;
                    const linked = new Map((node.sources || []).map(source => [identityKey(source), source]));
                    try {
                        const result = await request(`/nodes/${encodeURIComponent(id)}?version=${encodeURIComponent(node.version)}${ignoreSources ? '&ignore_sources=1' : ''}`, { method: 'DELETE' });
                        if (ignoreSources) mergeSources((Array.isArray(result?.ignored) ? result.ignored : []).map(identity => ({ ...(linked.get(identityKey(identity)) || {}), ...identity, ignored: true })));
                        removeNode(id);
                        removed.push(id);
                    } catch (error) {
                        if (state.disposed) break;
                        if (uncertainMutation(error)) await uncertainReload(error); else notify(messageFor(error), 'error');
                        break;
                    }
                }
                if (ignoreSources && removed.length) emit({ type: 'sources' });
                return removed;
            } finally { setBusy('remove', false); }
        }
        // updateNodeFields writes top-level fields of the stored node (not the
        // draft); one 409 reloads and retries; a dirty draft follows the version.
        async function updateNodeFields(id, patch) {
            for (let attempt = 0; attempt < 2; attempt += 1) {
                const node = nodeById(id);
                if (!node || !canWrite()) return false;
                try {
                    const result = await request(`/nodes/${encodeURIComponent(id)}`, { method: 'PUT', body: { ...model.clone(node), ...patch, id } });
                    const updated = result?.node || result;
                    if (!updated?.id) { await refreshSnapshot(); return true; }
                    const draft = state.drafts.get(id), dirty = state.dirty.has(id);
                    replaceNode(updated);
                    if (dirty && draft && draft.version === node.version) {
                        draft.version = updated.version;
                        for (const key of Object.keys(patch)) if (JSON.stringify(draft[key]) === JSON.stringify(node[key])) draft[key] = updated[key] ?? patch[key];
                    }
                    return true;
                } catch (error) {
                    if (error?.status === 409 && attempt === 0 && await refreshSnapshot()) continue;
                    if (error?.name === 'AbortError') return false;
                    if (uncertainMutation(error)) await uncertainReload(error); else notify(messageFor(error), 'error');
                    return false;
                }
            }
            return false;
        }
        async function updateParents(pairs) { for (const [id, parent] of pairs) if (!await updateNodeFields(id, { parent_id: parent || '' })) return false; return true; }

        // ---- history ---------------------------------------------------------------
        async function dispatch(command) {
            if (!command || !canWrite()) return false;
            const view = command.view || (state.view === 'table' ? 'atlas' : state.view);
            const history = histories[view];
            command.prepare?.(api);
            if (command.optimistic) {
                const done = command.apply(api);
                if (command.undoable) { history.push(command); emit({ type: 'history' }); }
                return done;
            }
            let ok;
            try { ok = await command.apply(api); }
            catch (error) {
                // An unconfirmed write: reload and say so; the fresh state tells the command whether it took effect.
                if (error?.name === 'AbortError' || !uncertainMutation(error)) throw error;
                await uncertainReload(error);
                ok = !!command.settle?.(api);
            }
            if (ok && command.undoable) { history.push(command); emit({ type: 'history' }); }
            return ok;
        }
        async function step(view, direction) {
            const history = histories[view];
            if (!history || state.historyBusy || !canWrite()) return false;
            const entry = direction === 'undo' ? history.takeUndo() : history.takeRedo();
            if (!entry) return false;
            const forward = direction === 'redo';
            const back = () => (forward ? history.pushRedo(entry) : history.pushUndo(entry));
            const done = () => (forward ? history.pushUndo(entry) : history.pushRedo(entry));
            if (entry.optimistic) { void (forward ? entry.apply(api) : entry.revert(api)); done(); emit({ type: 'history' }); return true; }
            state.historyBusy = true; emit({ type: 'history' });
            try {
                const ok = forward ? await entry.apply(api) : await entry.revert(api);
                if (ok) done(); else back();
                return !!ok;
            } catch (error) { back(); await writeFailed(error); return false; }
            finally { state.historyBusy = false; emit({ type: 'history' }); }
        }
        const canUndo = view => !!histories[view]?.canUndo() && !state.historyBusy;
        const canRedo = view => !!histories[view]?.canRedo() && !state.historyBusy;
        const hasUnsavedWork = () => state.dirty.size > 0 || state.sourceDirty.size > 0 || VIEWS.some(view => !laneIdle(lanes[view]) || lanes[view].unsaved.size > 0);

        // ---- visibility and polling (DOM; never called by the Node test) -----------
        function visibility() {
            const next = !state.disposed && !document.hidden && state.inView && (!win || (!win.hidden && !win.classList.contains('vd-space-hidden') && win.style.display !== 'none' && win.getClientRects().length > 0));
            if (next === state.visible) return;
            state.visible = next;
            if (!next) { clearTimeout(state.timer); state.timer = 0; abortRequests(); }
            else { void loadSnapshot(false); void loadTargets(); void loadIconManifest(); schedulePoll(); }
        }
        function schedulePoll() {
            clearTimeout(state.timer);
            if (state.disposed || !state.visible || state.pollPaused) return;
            state.timer = setTimeout(async () => { await loadSnapshot(false); schedulePoll(); }, 30000);
        }
        function attach({ root, win: windowElement } = {}) {
            win = windowElement || null;
            const onVisibility = () => visibility();
            document.addEventListener('visibilitychange', onVisibility);
            const windowObserver = typeof MutationObserver === 'function' && win ? new MutationObserver(onVisibility) : null;
            windowObserver?.observe(win, { attributes: true, attributeFilter: ['style', 'class', 'hidden', 'data-space-hidden'] });
            const intersection = typeof IntersectionObserver === 'function' && root ? new IntersectionObserver(entries => { state.inView = !!entries[0]?.isIntersecting; visibility(); }, { threshold: 0.01 }) : null;
            intersection?.observe(root);
            if (!intersection) state.inView = true;
            const onPolicy = event => {
                const detail = event?.detail || {};
                if (detail.readonly === true || detail.read_only === true || detail.enabled === false) {
                    ctx.readonly = true;
                    state.caps = { write: false, source_edit: false, control: false };
                    state.pollPaused = true;
                    abortRequests();
                    notify(t('desktop.infra_readonly_revoked'), 'info'); setSync('paused'); emit({ type: 'caps' });
                } else if (detail.readonly === false || detail.read_only === false || detail.enabled === true) {
                    ctx.readonly = false; state.pollPaused = false; abortRequests(); emit({ type: 'caps' });
                    void loadSnapshot(false); void loadTargets(true); void loadIconManifest();
                }
            };
            document.addEventListener('aurago:desktop-policy', onPolicy);
            detach = () => {
                document.removeEventListener('visibilitychange', onVisibility);
                document.removeEventListener('aurago:desktop-policy', onPolicy);
                windowObserver?.disconnect();
                intersection?.disconnect();
            };
            visibility();
        }
        function dispose() {
            if (state.disposed) return;
            abortRequests();
            state.disposed = true;
            clearTimeout(state.timer); state.timer = 0;
            detach();
            listeners.clear();
            for (const view of VIEWS) lanes[view].waiters.splice(0).forEach(resolve => resolve(false));
        }

        const api = {
            state, features, subscribe, notify, setBusy, request, messageFor, uncertainMutation, uncertainReload, writeFailed, isAccessError,
            canWrite, canEditSource, canControl, allowedAction, actionReason, actionBlockKey,
            nodeById, nodeIndex, childrenOf, guests, searchableText, matches,
            applySnapshot, loadSnapshot, loadTargets, loadCredentials, loadIconManifest, replaceNode, removeNode,
            select, selectEdge, clearSelection, setView, setFilters, setTool, toggleHost, beginInteraction, endInteraction,
            effectivePlacement, effectiveParent, currentIntent, layoutState, unsavedCount, queueLayout, whenLayoutIdle, retryLayout,
            createEdge, updateEdge, deleteEdgeById, dispatch, undo: view => step(view, 'undo'), redo: view => step(view, 'redo'),
            reloadSnapshot: refreshSnapshot, replaceNodes, setSourcesIgnored, setSourceHost, importBatch, removeNodes, updateNodeFields, updateParents, identityKey,
            canUndo, canRedo, hasUnsavedWork, attach, abortRequests, dispose,
        };
        return api;
    }

    window.InfraAppStore = { create, commands, edgeFields };
})();
