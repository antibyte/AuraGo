// Infra window state: API client, snapshot polling and visibility, CAS writes,
// the per-view layout queue with one conflict rebase, and undo/redo. Nothing
// here touches the DOM outside attach(): scripts/test-infra-ui.mjs runs the
// queue against a fake API inside a Node vm.
(function () {
    'use strict';

    const API = '/api/desktop/infra';
    const VIEWS = ['atlas', 'topology'];

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
    // layout-parent support understands; dispatch() calls prepare() so the
    // command knows that server and the previous state of every asset. Against
    // an older server the command stays out of history.
    const placeFromLibrary = (view, after, label = '') => {
        const command = layoutCommand('PlaceFromLibrary', view, new Map(), after, label);
        command.undoable = false;
        command.prepare = store => {
            const before = new Map();
            for (const id of after.keys()) before.set(id, store.currentIntent(id, view, store.features.layoutParent) || { unplace: true });
            command.before = before;
            command.undoable = store.features.layoutParent === true;
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
        createEdge: fields => ({
            type: 'CreateEdge', fields: { ...fields }, edgeId: '', undoable: true,
            async apply(store) { const edge = await store.createEdge(this.fields); this.edgeId = edge?.id || ''; return !!this.edgeId; },
            async revert(store) { return store.deleteEdgeById(this.edgeId); },
        }),
        deleteEdge: edge => ({
            type: 'DeleteEdge', fields: edgeFields(edge), edgeId: edge.id, undoable: true,
            async apply(store) { return store.deleteEdgeById(this.edgeId); },
            async revert(store) { const created = await store.createEdge(this.fields); this.edgeId = created?.id || ''; return !!this.edgeId; },
        }),
    };

    function create({ ctx = {}, t: translate } = {}) {
        const model = window.InfraAppModel;
        const t = translate || (key => ctx.t?.(key) || key);
        const listeners = new Set();
        const features = { layoutParent: false };
        const lanes = {}, histories = {};
        for (const view of VIEWS) {
            lanes[view] = { pending: new Map(), inflight: null, unsaved: new Map(), waiters: [], scheduled: false };
            histories[view] = model.createHistory(50);
        }
        const state = {
            disposed: false, epoch: 1, visible: false, inView: true, pollPaused: false, timer: 0,
            requests: new Set(), controller: new AbortController(), manifestController: null, manifestLoaded: false, manifestLoading: false,
            caps: { write: false, source_edit: false, control: false },
            nodes: [], edges: [], sources: [], sourceHosts: [], candidates: [],
            appTargets: [], targetsLoaded: false, targetsError: false,
            credentials: [], credentialsLoaded: false, credentialsLoading: false, credentialsError: false,
            stamp: '', refreshAt: 0, sync: 'syncing',
            view: 'atlas', tool: 'select', filters: { query: '', kind: '', source: '', power: '' },
            selection: new Set(), selected: '', selectedEdge: '', expandedHosts: new Set(), interacting: 0, historyBusy: false,
            drafts: new Map(), dirty: new Set(), draftRevision: new Map(), nodeReviews: new Map(), customRows: new Map(),
            sourceDrafts: new Map(), sourceDirty: new Set(), sourceRevision: new Map(), sourceReviews: new Map(), sourceBlocked: new Set(), sourcePending: new Set(),
            actionBlocks: new Map(), importSelection: new Set(), importBindings: new Map(), importDialogOwner: '', importOpen: false, dialogGeneration: 0,
            busy: new Set(),
        };
        let detach = () => {};
        let win = null;
        const emit = change => { if (!state.disposed) for (const fn of [...listeners]) fn(change); };
        const subscribe = fn => { listeners.add(fn); return () => listeners.delete(fn); };
        const notify = (message, kind = 'info') => emit({ type: 'notice', message, kind });
        const setBusy = (key, busy) => { if (busy) state.busy.add(key); else state.busy.delete(key); emit({ type: 'busy', key }); };

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
            const timeout = setTimeout(() => controller.abort(), 20000);
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
        const uncertainMutation = error => error?.name === 'AbortError' || (Number.isFinite(Number(error?.status)) && Number(error.status) >= 500) || (error?.status == null && error?.message !== 'unavailable');
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
            state.sourceHosts = features.layoutParent ? data.source_hosts : [];
            state.caps = { write: false, source_edit: false, control: false, ...(data.capabilities || {}) };
            for (const node of state.nodes) {
                if (state.nodeReviews.has(node.id) && state.dirty.has(node.id)) state.nodeReviews.set(node.id, { latest: model.clone(node) });
                if (!state.dirty.has(node.id)) state.drafts.set(node.id, model.clone(node));
            }
            pruneSelection();
            const stamp = model.snapshotStamp(state.nodes, state.edges, state.sources, state.caps);
            const structural = stamp !== state.stamp;
            state.stamp = stamp;
            state.refreshAt = Date.now();
            emit({ type: 'snapshot', structural, explicit });
            if (previousCaps !== JSON.stringify(state.caps)) emit({ type: 'caps' });
        }
        async function loadSnapshot(explicit = false) {
            if (state.disposed || !state.visible || state.busy.has('snapshot') || (state.pollPaused && !explicit)) return;
            const generation = state.epoch;
            setBusy('snapshot', true);
            try {
                const data = await request(`/snapshot${explicit ? '?refresh=1' : ''}`, {}, generation);
                state.pollPaused = false;
                applySnapshot(data, { explicit });
                setSync('live');
                schedulePoll();
            } catch (error) {
                if (error.name !== 'AbortError') {
                    if (isAccessError(error)) {
                        state.caps = { write: false, source_edit: false, control: false };
                        state.pollPaused = true; abortRequests(); emit({ type: 'caps' });
                        notify(t('desktop.infra_access_unavailable'), 'error'); setSync('paused');
                    } else { notify(messageFor(error), 'error'); setSync('error'); }
                }
            } finally { setBusy('snapshot', false); }
        }
        async function refreshSnapshot() {
            try { applySnapshot(await request('/snapshot')); return true; }
            catch (error) { if (error?.name !== 'AbortError') notify(messageFor(error), 'error'); return false; }
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
            state.nodes = state.nodes.filter(item => item.id !== id);
            state.edges = state.edges.filter(edge => edge.from !== id && edge.to !== id);
            state.drafts.delete(id); state.dirty.delete(id);
            for (const view of VIEWS) { lanes[view].pending.delete(id); lanes[view].unsaved.delete(id); }
            pruneSelection();
            emit({ type: 'nodes' }); emit({ type: 'edges' });
        };

        // ---- selection, view, filters --------------------------------------------
        function select(ids, { toggle = false, additive = false } = {}) {
            const next = toggle || additive ? new Set(state.selection) : new Set();
            for (const id of ids) { if (toggle && next.has(id)) next.delete(id); else if (nodeById(id)) next.add(id); }
            state.selection = next; state.selectedEdge = '';
            state.selected = next.size === 1 ? [...next][0] : '';
            if (state.selected && !state.dirty.has(state.selected)) state.drafts.set(state.selected, model.clone(nodeById(state.selected)));
            emit({ type: 'selection' });
        }
        const selectEdge = id => { state.selection = new Set(); state.selected = ''; state.selectedEdge = state.edges.some(edge => edge.id === id) ? id : ''; emit({ type: 'selection' }); };
        const clearSelection = () => { if (!state.selection.size && !state.selectedEdge) return; state.selection = new Set(); state.selected = ''; state.selectedEdge = ''; emit({ type: 'selection' }); };
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
            const edge = result?.edge || result;
            if (!edge?.id) { await refreshSnapshot(); return null; }
            state.edges = [...state.edges.filter(item => item.id !== edge.id), edge];
            emit({ type: 'edges' });
            return edge;
        }
        async function updateEdge(id, body) {
            const result = await request(`/edges/${encodeURIComponent(id)}`, { method: 'PUT', body });
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
            state.edges = state.edges.filter(item => item.id !== id);
            if (state.selectedEdge === id) { state.selectedEdge = ''; emit({ type: 'selection' }); }
            emit({ type: 'edges' });
            return true;
        }

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
            const ok = await command.apply(api);
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
            } catch (error) { back(); notify(messageFor(error), 'error'); return false; }
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
            state, features, subscribe, notify, setBusy, request, messageFor, uncertainMutation, isAccessError,
            canWrite, canEditSource, canControl, allowedAction, actionReason, actionBlockKey,
            nodeById, nodeIndex, childrenOf, guests, searchableText, matches,
            applySnapshot, loadSnapshot, loadTargets, loadCredentials, loadIconManifest, replaceNode, removeNode,
            select, selectEdge, clearSelection, setView, setFilters, setTool, toggleHost, beginInteraction, endInteraction,
            effectivePlacement, effectiveParent, currentIntent, layoutState, unsavedCount, queueLayout, whenLayoutIdle, retryLayout,
            createEdge, updateEdge, deleteEdgeById, dispatch, undo: view => step(view, 'undo'), redo: view => step(view, 'redo'),
            canUndo, canRedo, hasUnsavedWork, attach, abortRequests, dispose,
        };
        return api;
    }

    window.InfraAppStore = { create, commands, edgeFields };
})();
