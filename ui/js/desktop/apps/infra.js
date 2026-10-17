(function () {
    'use strict';

    const API = '/api/desktop/infra';
    const ICON_LIST = 'rack-server tower-server mini-pc sbc nas disk-shelf workstation laptop tablet smartphone router firewall modem-ont managed-switch switch access-point mesh-node ups pdu patch-panel camera nvr printer 3d-printer smart-hub sensor smart-plug tv console radio-gateway usb-gateway generic-device vm container hypervisor kubernetes database web-service reverse-proxy dns-dhcp vpn storage-share backup monitoring home-automation media-server ai-runtime mqtt logs git-ci mail-server photo-library password-manager cloud-service automation generic-app rack site room lan vlan wlan internet group'.split(' ');
    const ICONS = new Set(ICON_LIST);
    const ICON_ASSETS = new Map();
    const KINDS = ['device', 'vm', 'container', 'service', 'network', 'site', 'room', 'rack', 'group'];
    const DEFAULT_ICONS = { device: 'generic-device', vm: 'vm', container: 'container', service: 'generic-app', network: 'lan', site: 'site', room: 'room', rack: 'rack', group: 'group' };
    const EDGE_KINDS = ['ethernet', 'fiber', 'wifi', 'member_of', 'runs_on', 'depends_on', 'powered_by'];
    const SOURCE_KINDS = ['registry', 'docker', 'proxmox', 'fritzbox'];
    const SOURCE_EDIT_FIELDS = { registry: ['name', 'type', 'protocol', 'ip_address', 'port', 'username', 'credential_id', 'description', 'tags', 'mac_address'], docker: ['name'], proxmox: ['name', 'hostname', 'description', 'tags'], fritzbox: ['friendly_name'] };
    const INSTANCES = new Map();
    const escDefault = value => String(value ?? '').replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch]);
    const clone = value => JSON.parse(JSON.stringify(value));
    const iconId = value => ICONS.has(value) ? value : 'generic-device';
    const versionedAsset = path => `${path}?v=${encodeURIComponent(window.BUILD_VERSION || 'dev')}`;
    const iconURL = value => versionedAsset(ICON_ASSETS.get(iconId(value))?.path || `/img/infra/${iconId(value)}.webp`);
    const assetURL = value => {
        const id = value === 'icon' || value === 'empty-atlas' ? value : 'empty-atlas';
        return versionedAsset(ICON_ASSETS.get(id)?.path || `/img/infra/${id}.webp`);
    };
    const iconLabelKey = value => ICON_ASSETS.get(iconId(value))?.label_key || `desktop.infra_icon_${iconId(value).replaceAll('-', '_')}`;
    const sourceActions = kind => ({ docker: ['start', 'stop', 'restart'], proxmox: ['start', 'shutdown', 'reboot'], fritzbox: ['wake'] }[kind] || []);
    const sourceIdentity = source => {
        const bytes = new TextEncoder().encode(`${source?.kind || ''}\0${source?.instance || ''}\0${source?.key || ''}`);
        let binary = '';
        for (const byte of bytes) binary += String.fromCharCode(byte);
        return `infra_${window.btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '')}`;
    };
    const virtualSlice = (items, scrollTop, viewportHeight, rowHeight = 54, overscan = 5) => {
        const start = Math.max(0, Math.floor(scrollTop / rowHeight) - overscan);
        const end = Math.min(items.length, Math.ceil((scrollTop + viewportHeight) / rowHeight) + overscan);
        return { start, end, rows: items.slice(start, end), before: start * rowHeight, after: Math.max(0, (items.length - end) * rowHeight) };
    };
    const sourceState = node => {
        const states = (node.sources || []).map(source => source.state);
        return states.includes('current') ? 'current' : states.includes('stale') ? 'stale' : states.includes('unknown') ? 'unknown' : states.includes('removed') ? 'removed' : 'manual';
    };
    const visibleByCollapsed = (nodes, collapsed) => {
        const byParent = new Map();
        for (const node of nodes) if (node.parent_id) { const children = byParent.get(node.parent_id) || []; children.push(node); byParent.set(node.parent_id, children); }
        const hidden = new Set(), queue = [...collapsed].flatMap(id => byParent.get(id) || []);
        while (queue.length) { const node = queue.pop(); if (hidden.has(node.id)) continue; hidden.add(node.id); queue.push(...(byParent.get(node.id) || [])); }
        return nodes.filter(node => !hidden.has(node.id));
    };
    const isPlaced = (node, view) => !!node.placements?.[view];
    const atlasDimensions = count => {
        const columns = Math.max(3, Math.ceil(Math.sqrt(Math.max(1, count) * 1.45)));
        const rows = Math.max(2, Math.ceil(Math.max(1, count) / columns));
        return { columns, rows, width: Math.max(940, columns * 220 + 320), height: Math.max(600, rows * 155 + 220) };
    };
    const sceneDimensions = (nodes, view, pending = new Map()) => {
        let minX = 0, minY = 0, maxX = 620, maxY = 360;
        for (const node of nodes) {
            const position = pending.get(node.id) || node.placements?.[view];
            if (!position || !Number.isFinite(Number(position.x)) || !Number.isFinite(Number(position.y))) continue;
            minX = Math.min(minX, Number(position.x)); minY = Math.min(minY, Number(position.y));
            maxX = Math.max(maxX, Number(position.x)); maxY = Math.max(maxY, Number(position.y));
        }
        const originX = minX - 160, originY = minY - 120;
        return { originX, originY, width: Math.max(940, maxX - originX + 160), height: Math.max(600, maxY - originY + 120) };
    };
    const validCIDR = value => {
        if (!value) return true;
        const parts = String(value).trim().split('/');
        if (parts.length !== 2 || !/^\d{1,3}$/.test(parts[1])) return false;
        const prefix = Number(parts[1]), address = parts[0];
        if (address.includes(':')) {
            if (prefix > 128) return false;
            try { return new URL(`http://[${address}]/`).hostname.length > 0; } catch (_) { return false; }
        }
        const octets = address.split('.');
        return octets.length === 4 && prefix <= 32 && octets.every(part => /^\d{1,3}$/.test(part) && Number(part) <= 255);
    };
    const validHTTPLink = value => {
        try { const url = new URL(String(value)); return ['http:', 'https:'].includes(url.protocol) && !!url.hostname && !url.username && !url.password; }
        catch (_) { return false; }
    };

    window.InfraAppModel = { iconURL, assetURL, iconLabelKey, sourceActions, sourceIdentity, virtualSlice, sourceState, iconId, visibleByCollapsed, atlasDimensions, sceneDimensions, validCIDR, validHTTPLink, iconIds: () => [...ICON_LIST] };

    function render(host, windowId, context = {}) {
        dispose(windowId);
        if (!host) return;
        const ctx = context;
        const esc = ctx.esc || escDefault;
        const t = key => ctx.t?.(key) || key;
        const state = {
            host, windowId, ctx, disposed: false, epoch: 1, caps: { write: false, source_edit: false, control: false },
            nodes: [], edges: [], sources: [], candidates: [], appTargets: [], credentials: [], credentialsLoaded: false, credentialsLoading: false, credentialsError: false, selected: '', view: 'atlas', scope: 'all', kindFilter: '', customRows: new Map(), targetsLoaded: false, targetsError: false, nodeReviews: new Map(), sourceReviews: new Map(),
            query: '', collapsed: new Set(), expandedHosts: new Set(), drafts: new Map(), dirty: new Set(), draftRevision: new Map(), sourceDrafts: new Map(), sourceDirty: new Set(), sourceRevision: new Map(), actionBlocks: new Map(),
            importSelection: new Set(), importBindings: new Map(), importDialogOwner: '', dialogGeneration: 0, layoutUndo: null, layoutRedo: null, pendingLayout: new Map(), pendingLayoutView: '', sourceBlocked: new Set(), sourcePending: new Set(),
            busy: new Set(), notice: '', noticeType: 'info', timer: 0, requests: new Set(), controller: new AbortController(),
            visible: false, inView: true, filterSource: '', refreshAt: 0, manifestLoaded: false, manifestLoading: false, manifestController: null, collapsedInitialized: false, importOpen: false, zoom: 0.9, pollPaused: false,
        };
        INSTANCES.set(windowId, state);
        const safeWindowId = String(windowId).replace(/[^a-zA-Z0-9_-]/g, '');
        host.innerHTML = `<section class="infra-app" data-infra-app="${esc(safeWindowId)}" aria-label="${esc(t('desktop.app_infra'))}">
          <header class="infra-topbar">
            <div class="infra-brand"><img class="infra-brand-mark" src="${esc(assetURL('icon'))}" alt="" draggable="false"><div><span class="infra-eyebrow">${esc(t('desktop.infra_workspace'))}</span><h1>${esc(t('desktop.app_infra'))}</h1></div></div>
            <div class="infra-toolbar" role="toolbar" aria-label="${esc(t('desktop.infra_actions'))}">
              <div class="infra-view-switch" role="group" aria-label="${esc(t('desktop.infra_view'))}">
                <button type="button" data-view="atlas" aria-pressed="true">${esc(t('desktop.infra_atlas'))}</button>
                <button type="button" data-view="topology" aria-pressed="false">${esc(t('desktop.infra_topology'))}</button>
                <button type="button" data-view="table" aria-pressed="false">${esc(t('desktop.infra_inventory'))}</button>
              </div>
              <button type="button" class="infra-button infra-button-quiet" data-action="refresh" title="${esc(t('desktop.infra_refresh'))}"><span aria-hidden="true">↻</span><span>${esc(t('desktop.infra_refresh'))}</span></button>
              <button type="button" class="infra-button infra-button-secondary" data-action="undo" disabled><span aria-hidden="true">↶</span><span>${esc(t('desktop.infra_undo'))}</span></button>
              <button type="button" class="infra-button infra-button-secondary" data-action="retry-layout" disabled>${esc(t('desktop.infra_retry_positions'))}</button>
              <button type="button" class="infra-button infra-button-secondary" data-action="import" disabled>⇣ ${esc(t('desktop.infra_import_title'))}</button>
              <button type="button" class="infra-button infra-button-primary" data-action="add" disabled>＋ ${esc(t('desktop.infra_add_node'))}</button>
              <button type="button" class="infra-icon-button" data-action="more" aria-label="${esc(t('desktop.infra_more_actions'))}" title="${esc(t('desktop.infra_more_actions'))}">···</button>
            </div>
          </header>
          <div class="infra-filterbar">
            <label class="infra-search"><span aria-hidden="true">⌕</span><input type="search" data-search aria-label="${esc(t('desktop.infra_search'))}" placeholder="${esc(t('desktop.infra_search_hint'))}" autocomplete="off"></label>
            <label class="infra-select-label"><span>${esc(t('desktop.infra_type'))}</span><select data-kind-filter aria-label="${esc(t('desktop.infra_type_filter'))}"><option value="">${esc(t('desktop.infra_all_types'))}</option>${KINDS.map(kind => `<option value="${kind}">${esc(t(`desktop.infra_kind_${kind}`))}</option>`).join('')}</select></label>
            <label class="infra-select-label"><span>${esc(t('desktop.infra_source'))}</span><select data-source-filter aria-label="${esc(t('desktop.infra_source_filter'))}"><option value="">${esc(t('desktop.infra_all_sources'))}</option>${SOURCE_KINDS.map(kind => `<option value="${kind}">${esc(t(`desktop.infra_source_${kind}`))}</option>`).join('')}</select></label>
            <div class="infra-live-status" data-live role="status" aria-live="polite"><i></i><span>${esc(t('desktop.infra_loading'))}</span></div>
          </div>
          <div class="infra-workspace">
            <aside class="infra-sidebar" aria-label="${esc(t('desktop.infra_navigation'))}">
              <div class="infra-side-heading"><span>${esc(t('desktop.infra_inventory'))}</span><span data-count class="infra-count">0</span></div>
              <nav class="infra-scope-nav" aria-label="${esc(t('desktop.infra_scopes'))}">
                <button type="button" data-scope="all" aria-pressed="true"><span class="infra-nav-glyph">◫</span>${esc(t('desktop.infra_all_assets'))}<span data-scope-count="all"></span></button>
                <button type="button" data-scope="unplaced" aria-pressed="false"><span class="infra-nav-glyph">⌖</span>${esc(t('desktop.infra_unplaced'))}<span data-scope-count="unplaced"></span></button>
                <button type="button" data-scope="attention" aria-pressed="false"><span class="infra-nav-glyph">◌</span>${esc(t('desktop.infra_attention'))}<span data-scope-count="attention"></span></button>
              </nav>
              <div class="infra-side-heading infra-groups-heading"><span>${esc(t('desktop.infra_groups'))}</span><button type="button" data-action="add-group" aria-label="${esc(t('desktop.infra_add_group'))}" title="${esc(t('desktop.infra_add_group'))}" disabled>＋</button></div>
              <div class="infra-group-tree" data-groups></div>
              <div class="infra-sidebar-bottom"><span class="infra-source-ledger"><i></i><span data-source-summary>${esc(t('desktop.infra_sources_waiting'))}</span></span><span class="infra-updated" data-updated></span></div>
            </aside>
            <main class="infra-main" data-main>
              <section class="infra-stage-view" data-stage-view aria-label="${esc(t('desktop.infra_atlas'))}">
                <div class="infra-stage-head"><div><p class="infra-kicker">${esc(t('desktop.infra_atlas_kicker'))}</p><h2 data-stage-title>${esc(t('desktop.infra_atlas_title'))}</h2></div><div class="infra-stage-tools"><span data-visible-count></span><div class="infra-zoom" role="group" aria-label="${esc(t('desktop.infra_zoom'))}"><button type="button" data-action="zoom-out" aria-label="${esc(t('desktop.infra_zoom_out'))}" title="${esc(t('desktop.infra_zoom_out'))}">−</button><output data-zoom>90%</output><button type="button" data-action="zoom-in" aria-label="${esc(t('desktop.infra_zoom_in'))}" title="${esc(t('desktop.infra_zoom_in'))}">＋</button><button type="button" data-action="zoom-reset" aria-label="${esc(t('desktop.infra_zoom_reset'))}" title="${esc(t('desktop.infra_zoom_reset'))}">⤢</button></div><button type="button" data-action="arrange" disabled>${esc(t('desktop.infra_arrange'))}</button><button type="button" data-action="edge-mode" aria-pressed="false" disabled>${esc(t('desktop.infra_connect'))}</button></div></div>
                <div class="infra-stage-scroll"><div class="infra-stage" data-stage tabindex="0" role="region" aria-label="${esc(t('desktop.infra_stage_hint'))}"><div class="infra-scene" data-scene>
                  <div class="infra-stage-grid" aria-hidden="true"></div><svg class="infra-edges" data-edges aria-hidden="true" focusable="false" viewBox="0 0 100 100" preserveAspectRatio="none"></svg><div class="infra-nodes" data-nodes></div></div>
                  <div class="infra-stage-empty" data-empty hidden><img src="${esc(assetURL('empty-atlas'))}" alt="" draggable="false"><strong>${esc(t('desktop.infra_empty_map'))}</strong><p>${esc(t('desktop.infra_empty_map_hint'))}</p><div class="infra-empty-actions"><button type="button" data-action="add-empty" disabled>${esc(t('desktop.infra_add_first'))}</button><button type="button" class="infra-button-secondary" data-action="import-empty" disabled>${esc(t('desktop.infra_import_existing'))}</button></div></div>
                </div></div>
                <div class="infra-stage-footer"><span><i class="infra-legend-dot current"></i>${esc(t('desktop.infra_state_current'))}<i class="infra-legend-dot stale"></i>${esc(t('desktop.infra_state_stale'))}<i class="infra-legend-dot unknown"></i>${esc(t('desktop.infra_state_unknown'))}</span><span data-map-help>${esc(t('desktop.infra_map_hint'))}</span></div>
              </section>
              <section class="infra-table-view" data-table-view hidden aria-label="${esc(t('desktop.infra_inventory'))}"><div class="infra-table-head"><div><p class="infra-kicker">${esc(t('desktop.infra_inventory_kicker'))}</p><h2>${esc(t('desktop.infra_inventory'))}</h2></div><span data-table-total></span></div><div class="infra-table-scroll" data-table-scroll tabindex="0"><table><thead><tr><th>${esc(t('desktop.infra_name'))}</th><th>${esc(t('desktop.infra_type'))}</th><th>${esc(t('desktop.infra_hardware'))}</th><th>${esc(t('desktop.infra_address'))}</th><th>${esc(t('desktop.infra_source'))}</th><th>${esc(t('desktop.infra_status'))}</th></tr></thead><tbody data-table-body></tbody></table></div></section>
              <section class="infra-tray" data-tray aria-label="${esc(t('desktop.infra_unplaced'))}"><div class="infra-tray-title"><span>${esc(t('desktop.infra_unplaced'))}</span><span data-tray-count></span><span class="infra-tray-rule"></span><span>${esc(t('desktop.infra_unplaced_hint'))}</span></div><div class="infra-tray-items" data-tray-items></div></section>
            </main>
            <aside class="infra-inspector" data-inspector aria-label="${esc(t('desktop.infra_details'))}"><div class="infra-inspector-empty"><span aria-hidden="true">◈</span><strong>${esc(t('desktop.infra_select_asset'))}</strong><p>${esc(t('desktop.infra_select_asset_hint'))}</p></div></aside>
          </div>
          <dialog class="infra-dialog" data-dialog aria-labelledby="infra-dialog-title-${esc(safeWindowId)}"><form method="dialog"><button class="infra-dialog-close" aria-label="${esc(t('desktop.close'))}">×</button></form><div data-dialog-content></div></dialog>
        </section>`;

        const root = host.querySelector('.infra-app');
        const q = selector => root.querySelector(selector);
        const win = host.closest('.vd-window');
        const ownsDialog = owner => !!owner && q('[data-dialog]')?.dataset.infraOwner === owner;
        const closeOwnedDialog = owner => { const dialog = q('[data-dialog]'); if (dialog?.open && ownsDialog(owner)) dialog.close(); };
        const stage = q('[data-stage]');
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
            if (error?.status === 409) return t('desktop.infra_error_conflict');
            const code = String(error?.body?.error || error?.message || 'generic').toLowerCase().replace(/[^a-z0-9_]/g, '_');
            return t(`desktop.infra_error_${code}`);
        };
        const uncertainMutation = error => error?.name === 'AbortError' || (Number.isFinite(Number(error?.status)) && Number(error.status) >= 500) || (error?.status == null && error?.message !== 'unavailable');
        const isAccessError = error => {
            const code = String(error?.body?.error || '').toLowerCase();
            return [401, 403].includes(Number(error?.status)) || ['disabled', 'desktop_disabled', 'infra_disabled', 'forbidden', 'unauthorized'].includes(code);
        };
        const setNotice = (message, type = 'info') => {
            state.notice = message;
            state.noticeType = type;
            if (state.disposed) return;
            const live = q('[data-live]');
            if (!live) return;
            live.classList.toggle('is-error', type === 'error');
            live.classList.toggle('is-busy', type === 'busy');
            live.querySelector('span').textContent = message;
        };
        const statusLabel = status => t(`desktop.infra_state_${status}`);
        const icon = value => `<img src="${esc(iconURL(value))}" alt="" loading="lazy" draggable="false">`;
        async function loadIconManifest() {
            if (state.manifestLoaded || state.manifestLoading || state.disposed || !state.visible) return;
            state.manifestLoading = true;
            const controller = new AbortController(), generation = state.epoch;
            state.manifestController = controller;
            try {
                const response = await fetch(versionedAsset('/img/infra/manifest.json'), { signal: controller.signal, cache: 'force-cache' });
                if (!response.ok) throw new Error(`manifest ${response.status}`);
                const manifest = await response.json();
                if (state.disposed || !state.visible || generation !== state.epoch || !Array.isArray(manifest?.icons)) return;
                for (const entry of manifest.icons) {
                    const id = String(entry?.id || ''), path = String(entry?.path || '');
                    if (!(ICONS.has(id) || id === 'icon' || id === 'empty-atlas') || path !== `/img/infra/${id}.webp`) continue;
                    ICON_ASSETS.set(id, { path, label_key: typeof entry.label_key === 'string' ? entry.label_key : '' });
                }
                state.manifestLoaded = true;
                paintAll();
            } catch (error) {
                if (error?.name !== 'AbortError') state.manifestLoaded = true;
            } finally {
                state.manifestLoading = false;
                if (state.manifestController === controller) state.manifestController = null;
                if (!state.manifestLoaded && !state.pollPaused && state.visible && !state.disposed && generation !== state.epoch) setTimeout(() => void loadIconManifest(), 0);
            }
        }
        const nodeById = id => state.nodes.find(node => node.id === id);
        const groupChildren = id => state.nodes.filter(node => node.parent_id === id);
        const hostedServices = hostID => [...new Set(state.edges.filter(edge => edge.kind === 'runs_on' && edge.to === hostID).map(edge => edge.from))].map(nodeById).filter(node => node?.kind === 'service');
        const hostingHosts = serviceID => state.edges.filter(edge => edge.kind === 'runs_on' && edge.from === serviceID).map(edge => edge.to);
        const searchableText = node => {
            const parts = [node.name, node.kind, node.profile?.vendor, node.profile?.model, node.profile?.serial, node.profile?.os, node.profile?.firmware,
                node.profile?.ip_address, node.profile?.mac_address, node.profile?.cidr, node.profile?.vlan, node.profile?.notes,
                ...(node.profile?.tags || []), ...(node.profile?.interfaces || []).flatMap(item => Object.values(item || {})),
                ...(node.profile?.links || []).flatMap(item => [item.label, item.url]),
                ...Object.entries(node.profile?.custom || {}).flatMap(([key, value]) => [key, value])];
            for (const source of node.sources || []) {
                parts.push(source.kind, source.instance, source.key, source.name);
                const observed = source.observed || {};
                for (const key of ['name', 'type', 'model', 'ip_address', 'hostname', 'state', 'status', 'node', 'vmid', 'role', 'os', 'description']) parts.push(observed[key]);
                for (const item of (Array.isArray(observed.interfaces) ? observed.interfaces : [])) if (item && typeof item === 'object') parts.push(item.name, item.ip_address, item.mac_address, item.network);
            }
            for (const binding of node.profile?.app_bindings || []) {
                const target = state.appTargets.find(item => item.app === binding.app && item.target_id === binding.target_id);
                parts.push(binding.app, binding.target_id, target?.label);
            }
            for (const link of node.app_links || []) parts.push(link.app, link.target_id, state.appTargets.find(item => item.app === link.app && item.target_id === link.target_id)?.label);
            return parts.filter(value => value != null).join(' ').toLocaleLowerCase();
        };
        const displayedNodes = (view = state.view) => {
            let nodes = state.nodes.filter(node => {
                if (state.kindFilter && node.kind !== state.kindFilter) return false;
                if (state.filterSource && !(node.sources || []).some(source => source.kind === state.filterSource)) return false;
                if (state.query && !searchableText(node).includes(state.query.toLocaleLowerCase())) return false;
                if (state.scope === 'unplaced' && isPlaced(node, view)) return false;
                if (state.scope === 'attention' && !['stale', 'unknown', 'removed'].includes(sourceState(node))) return false;
                return true;
            });
            if (view !== 'table') {
                nodes = visibleByCollapsed(nodes, state.collapsed).filter(node => isPlaced(node, view));
                if (view === 'atlas') nodes = nodes.filter(node => node.kind !== 'service' || !hostingHosts(node.id).length || hostingHosts(node.id).some(hostID => state.expandedHosts.has(hostID)));
            }
            return nodes;
        };
        const filteredNodes = () => displayedNodes('table');
        const canWrite = () => state.caps.write === true && ctx.readonly !== true;
        const canEditSource = () => state.caps.source_edit === true && ctx.readonly !== true;
        const canControl = () => state.caps.control === true && ctx.readonly !== true;
        const allowedAction = (node, action, source = '') => {
            if (!canControl()) return false;
            return !!source && Array.isArray(node?.source_actions?.[sourceIdentity(source)]) && node.source_actions[sourceIdentity(source)].includes(action);
        };
        const actionReason = (node, source, action) => node?.source_action_reasons?.[sourceIdentity(source)]?.[action] || '';
        const actionBlockKey = (nodeID, source, action) => `${nodeID}\u0000${typeof source === 'string' ? source : sourceIdentity(source)}\u0000${action}`;
        const syncMenus = () => ctx.setWindowMenus?.(windowId, [{ id: 'infra', labelKey: 'desktop.app_infra', items: [
            { labelKey: 'desktop.infra_refresh', icon: 'refresh', action: () => void loadSnapshot(true) },
            { type: 'separator' },
            { labelKey: 'desktop.infra_import_title', icon: 'download', disabled: !canWrite(), action: openImport },
            { labelKey: 'desktop.infra_new_node', icon: 'plus', disabled: !canWrite(), action: () => openNodeDialog() },
            { labelKey: 'desktop.infra_arrange', icon: 'grid', disabled: !canWrite(), action: () => void arrange() }
        ] }]);
        const labelForKind = kind => t(`desktop.infra_kind_${kind}`);
        const setBusy = (key, busy) => {
            if (busy) state.busy.add(key); else state.busy.delete(key);
            if (state.disposed) return;
            paintToolbar();
            if (key.startsWith('source-')) {
                const sourceID = key.slice('source-'.length);
                const save = state.selected === sourceID ? q('[data-action="save-source"]') : null;
                if (save) {
                    const sourceBusy = state.busy.has(key);
                    save.disabled = sourceBusy || !canEditSource() || !state.sourceDirty.has(sourceID) || state.sourceBlocked.has(sourceID);
                    save.textContent = sourceBusy ? t('desktop.infra_saving') : t('desktop.infra_save_source');
                }
            }
            if (key === 'create') { const submit = q('[data-new-node] [type="submit"]'); if (submit) submit.disabled = busy; }
            if (key === 'import') { const submit = q('[data-import]'); if (submit) submit.disabled = busy || !state.importSelection.size || !canWrite(); }
            if (key.startsWith('edge-')) { const submit = q('[data-edge-form] [type="submit"]'); if (submit) submit.disabled = busy; }
            if (key.startsWith('node-')) { const submit = q('[data-detail-form] button[type="submit"]'); if (submit) submit.disabled = busy || !state.dirty.has(key.slice('node-'.length)); }
            if (key.startsWith('action-')) q('[data-dialog]')?.querySelectorAll('[data-run-action]').forEach(button => { button.disabled = busy || state.actionBlocks.has(actionBlockKey(button.dataset.actionNode, button.dataset.sourceIdentity, button.dataset.runAction)); });
        };
        async function loadSnapshot(explicit = false) {
            if (state.disposed || !state.visible || state.busy.has('snapshot') || (state.pollPaused && !explicit)) return;
            const generation = state.epoch;
            setBusy('snapshot', true); setNotice(t('desktop.infra_loading'), 'busy');
            try {
                const data = await request(`/snapshot${explicit ? '?refresh=1' : ''}`, {}, generation);
                state.nodes = (data.nodes || []).map(node => ({ ...node, icon: iconId(node.icon), profile: { tags: [], interfaces: [], links: [], app_bindings: [], custom: {}, ...(node.profile || {}) }, placements: node.placements || {}, sources: node.sources || [], app_links: node.app_links || [], available_actions: node.available_actions || [] }));
                for (const node of state.nodes) if (state.nodeReviews.has(node.id) && state.dirty.has(node.id)) state.nodeReviews.set(node.id, { latest: clone(node) });
                if (!state.collapsedInitialized) {
                    state.nodes.filter(node => node.kind === 'group' && groupChildren(node.id).length).forEach(node => state.collapsed.add(node.id));
                    state.collapsedInitialized = true;
                }
                state.edges = data.edges || []; state.sources = data.sources || [];
                state.caps = { write: false, source_edit: false, control: false, ...(data.capabilities || {}) };
                state.pollPaused = false;
                for (const node of state.nodes) if (!state.dirty.has(node.id)) state.drafts.set(node.id, clone(node));
                if (state.selected && !nodeById(state.selected)) {
                    const removed = state.selected;
                    state.selected = '';
                    if (state.dirty.has(removed)) setNotice(t('desktop.infra_remote_removed_draft'), 'error');
                }
                state.refreshAt = Date.now();
                if (state.noticeType !== 'error') { state.notice = ''; state.noticeType = 'info'; }
                syncMenus();
                paintAll();
                schedulePoll();
            } catch (error) {
                if (error.name !== 'AbortError') {
                    if (isAccessError(error)) {
                        state.caps = { write: false, source_edit: false, control: false };
                        state.pollPaused = true; abortRequests(); syncMenus(); paintAll();
                        setNotice(t('desktop.infra_access_unavailable'), 'error');
                    } else setNotice(messageFor(error), 'error');
                }
            }
            finally { setBusy('snapshot', false); }
        }
        async function loadTargets(force = false) {
            if (state.disposed || !state.visible || (state.targetsLoaded && !force)) return;
            try {
                const result = await request('/targets');
                state.appTargets = (result.targets || []).filter(target => ['network-cameras', 'software-store', 'meshcore'].includes(target.app) && target.target_id).map(target => ({ app: target.app, target_id: String(target.target_id), label: String(target.label || target.target_id) }));
                state.targetsLoaded = true; state.targetsError = false; paintInspector();
            } catch (error) {
                if (error.name !== 'AbortError') { state.targetsError = true; paintInspector(); }
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
                if (state.selected && state.sourceDrafts.get(state.selected)?.source === 'registry') paintInspector();
            } catch (error) {
                if (error.name !== 'AbortError') { state.credentialsError = true; if (state.selected && state.sourceDrafts.get(state.selected)?.source === 'registry') paintInspector(); }
            } finally { state.credentialsLoading = false; }
        }
        async function loadCandidates(preserveSelection = false, owner = state.importDialogOwner) {
            try {
                const selected = preserveSelection ? new Set(state.importSelection) : new Set();
                const result = await request('/candidates');
                if (state.disposed || !owner || state.importDialogOwner !== owner || !ownsDialog(owner)) return;
                state.candidates = result.candidates || []; state.sources = result.sources || state.sources;
                state.importSelection = preserveSelection ? new Set(state.candidates.filter(candidate => selected.has(candidate.id)).map(candidate => candidate.id)) : new Set(state.candidates.map(candidate => candidate.id));
                paintImportDialog();
            } catch (error) { if (error.name !== 'AbortError') setNotice(messageFor(error), 'error'); }
        }
        const selectNode = (id, focusView = '') => {
            if (!nodeById(id)) return;
            state.selected = id;
            if (!state.dirty.has(id)) state.drafts.set(id, clone(nodeById(id)));
            paintGroups(); paintStage(focusView === 'stage'); paintTable(); paintTray(); paintInspector();
            if (focusView === 'table') root.querySelector(`[data-row="${CSS.escape(id)}"]`)?.focus({ preventScroll: true });
        };
        const nextPosition = (view, id, preferred = { x: 110, y: 100 }) => {
            const occupied = state.nodes.filter(node => node.id !== id && isPlaced(node, view)).map(node => node.placements[view]);
            const columns = Math.max(3, Math.ceil(Math.sqrt((state.nodes.length + 1) * 1.45)));
            const startCol = Math.max(0, Math.round((preferred.x - 110) / 220)), startRow = Math.max(0, Math.round((preferred.y - 100) / 155));
            const open = (col, row) => {
                const x = 110 + col * 220, y = 100 + row * 155;
                return occupied.every(pos => Math.abs(Number(pos.x) - x) >= 160 || Math.abs(Number(pos.y) - y) >= 110) ? { x, y, pinned: false } : null;
            };
            for (let radius = 0; radius < 200; radius++) {
                for (let row = -radius; row <= radius; row++) for (let col = -radius; col <= radius; col++) {
                    if (Math.max(Math.abs(col), Math.abs(row)) !== radius || startCol + col < 0 || startRow + row < 0) continue;
                    const position = open(startCol + col, startRow + row);
                    if (position) return position;
                }
            }
            return { x: 110 + columns * 220, y: 100, pinned: false };
        };
        function placeByChoice(id, choice) {
            ensureMapView();
            if (choice === 'keyboard') { state.keyboardMove = true; state.selected = id; paintStage(true); setNotice(t('desktop.infra_keyboard_hint'), 'info'); return; }
            const preferred = choice === 'northwest' ? { x: 110, y: 100 } : { x: state.sceneGeometry.originX + state.sceneGeometry.width / 2, y: state.sceneGeometry.originY + state.sceneGeometry.height / 2 };
            const pos = { ...nextPosition(state.view, id, preferred), pinned: true };
            void persistLayout(new Map([[id, pos]]));
        }
        async function saveKeyboardMove(id) {
            if (!state.keyboardMove) return;
            state.keyboardMove = false;
            const pos = state.keyboardPosition || nodeById(id)?.placements?.[state.view] || nextPosition(state.view, id);
            state.keyboardPosition = null;
            await persistLayout(new Map([[id, { ...pos, pinned: true }]]));
        }
        async function persistLayout(changes, {undo = false, redo = false, view = state.view} = {}) {
            if (!canWrite() || !changes.size || state.busy.has('layout')) return false;
            if (state.pendingLayout.size && state.pendingLayoutView !== view) {
                setNotice(t('desktop.infra_layout_pending_review'), 'error');
                return false;
            }
            const prior = new Map();
            for (const [id] of changes) { const node = nodeById(id); if (!node) continue; prior.set(id, clone(node.placements?.[view] || { x: null, y: null, pinned: false })); }
            const nodes = [...changes].map(([id, position]) => ({ id, version: nodeById(id)?.version || 0, ...position }));
            const undoable = !undo && [...prior.values()].every(position => position.x != null && position.y != null);
            setBusy('layout', true); setNotice(t('desktop.infra_saving_layout'), 'busy');
            try {
                const result = await request('/layout', { method: 'PUT', body: { view, nodes } });
                const snapshot = result.snapshot || (Array.isArray(result.nodes) && result.nodes.some(node => node.placements) ? result : null);
                if (snapshot && Array.isArray(snapshot.nodes)) {
                    state.nodes = snapshot.nodes.map(node => ({ ...node, icon: iconId(node.icon), profile: { tags: [], interfaces: [], links: [], app_bindings: [], custom: {}, ...(node.profile || {}) }, placements: node.placements || {}, sources: node.sources || [], app_links: node.app_links || [], available_actions: node.available_actions || [] }));
                    state.edges = snapshot.edges || state.edges; state.sources = snapshot.sources || state.sources;
                    if (snapshot.capabilities) state.caps = { write: false, source_edit: false, control: false, ...snapshot.capabilities };
                } else {
                    const updated = result.nodes || result;
                    if (Array.isArray(updated)) updated.forEach(row => {
                        const node = nodeById(row.id); if (!node) return;
                        node.version = row.version ?? node.version; node.placements ||= {};
                        node.placements[view] = row.placements?.[view] || { x: row.x, y: row.y, pinned: !!row.pinned };
                    });
                }
                for (const [id] of changes) state.pendingLayout.delete(id);
                if (!state.pendingLayout.size) state.pendingLayoutView = '';
                if (undo) state.layoutUndo = null;
                else if (redo) state.layoutUndo = { view, prior: new Map([...changes].map(([id]) => [id, clone(state.layoutRedo?.prior?.get(id) || prior.get(id))])), current: new Map([...changes].map(([id]) => [id, clone(changes.get(id))])), ids: [...changes.keys()] };
                else { state.layoutUndo = undoable ? { view, prior, current: new Map(changes), ids: [...changes.keys()] } : null; state.layoutRedo = null; }
                state.keyboardMove = false; setNotice(t('desktop.infra_layout_saved'), 'success'); paintAll();
                return true;
            } catch (error) {
                if (error.name !== 'AbortError') { state.pendingLayoutView = view; changes.forEach((position, id) => state.pendingLayout.set(id, position)); setNotice(messageFor(error), 'error'); }
                paintAll();
                return false;
            } finally { setBusy('layout', false); }
        }
        function updateUndoButton() { q('[data-action="undo"]').disabled = !canWrite() || !state.layoutUndo || state.busy.has('layout'); }
        async function undoLayout() {
            const entry = state.layoutUndo; if (!entry || !canWrite() || state.busy.has('layout')) return;
            const changes = new Map();
            for (const [id, pos] of entry.prior) if (pos.x != null) changes.set(id, { ...pos, pinned: pos.pinned });
            if (!changes.size) { setNotice(t('desktop.infra_undo_unavailable'), 'error'); return; }
                if (await persistLayout(changes, { undo: true, view: entry.view })) {
                state.layoutRedo = { view: entry.view, prior: entry.prior, current: entry.current, ids: entry.ids };
                state.layoutUndo = null; paintToolbar();
            }
        }
        async function arrange() {
            const visible = visibleByCollapsed(state.nodes.filter(matchesFilters), state.collapsed);
            const nodes = visible.filter(node => isPlaced(node, state.view) && !node.placements[state.view].pinned)
                .sort((a, b) => a.kind.localeCompare(b.kind) || a.name.localeCompare(b.name));
            if (!canWrite() || state.busy.has('layout') || nodes.length < 1) return;
            const moving = new Set(nodes.map(node => node.id));
            const obstacles = state.nodes.filter(node => !moving.has(node.id) && isPlaced(node, state.view)).map(node => node.placements[state.view]);
            const changes = new Map();
            const columns = Math.max(3, Math.ceil(Math.sqrt(state.nodes.length * 1.45)));
            let index = 0;
            for (const node of nodes) {
                let position;
                for (let attempts = 0; attempts < 20000; attempts++, index++) {
                    const x = 110 + (index % columns) * 220, y = 100 + Math.floor(index / columns) * 155;
                    const occupied = [...obstacles, ...[...changes.values()]];
                    if (occupied.every(pos => Math.abs(Number(pos.x) - x) >= 160 || Math.abs(Number(pos.y) - y) >= 110)) { position = { x, y, pinned: false }; break; }
                }
                if (position) changes.set(node.id, position);
            }
            if (!changes.size) return;
            await persistLayout(changes);
        }
        function startDrag(event) {
            const button = event.target.closest('[data-node]');
            if (!button) {
                if (event.target.closest('button, input, select, a') || (event.button !== 0 && event.button !== 1)) return;
                const scroll = q('.infra-stage-scroll');
                state.pan = { x: event.clientX, y: event.clientY, left: scroll.scrollLeft, top: scroll.scrollTop, moved: false };
                stage.setPointerCapture?.(event.pointerId); scroll.classList.add('is-panning'); event.preventDefault(); return;
            }
            if (event.button !== 0 || state.edgeMode || state.busy.has('layout')) return;
            if (!canWrite()) return;
            const id = button.dataset.node; const node = nodeById(id); if (!node) return;
            state.drag = { id, button, startX: event.clientX, startY: event.clientY, initial: node.placements[state.view], moved: false };
            button.setPointerCapture?.(event.pointerId);
        }
        function moveDrag(event) {
            if (state.pan) {
                const scroll = q('.infra-stage-scroll'), dx = event.clientX - state.pan.x, dy = event.clientY - state.pan.y;
                if (Math.hypot(dx, dy) > 2) state.pan.moved = true;
                scroll.scrollLeft = state.pan.left - dx; scroll.scrollTop = state.pan.top - dy; return;
            }
            const drag = state.drag; if (!drag) return;
            const rect = q('[data-scene]').getBoundingClientRect();
            const dx = event.clientX - drag.startX, dy = event.clientY - drag.startY;
            if (!drag.moved && Math.hypot(dx, dy) < 4) return;
            drag.moved = true; state.dragMoved = true;
            const x = Math.max(80, Math.min(state.sceneGeometry.width - 80, (event.clientX - rect.left) / state.zoom));
            const y = Math.max(70, Math.min(state.sceneGeometry.height - 70, (event.clientY - rect.top) / state.zoom));
            drag.button.style.left = `${x}px`;
            drag.button.style.top = `${y}px`;
        }
        function endDrag() {
            if (state.pan) { state.pan = null; q('.infra-stage-scroll').classList.remove('is-panning'); return; }
            const drag = state.drag; if (!drag) return;
            state.drag = null;
            if (!drag.moved) return;
            const x = Math.round(parseFloat(drag.button.style.left) + state.sceneGeometry.originX), y = Math.round(parseFloat(drag.button.style.top) + state.sceneGeometry.originY);
            window.setTimeout(() => { state.dragMoved = false; }, 0);
            void persistLayout(new Map([[drag.id, { x, y, pinned: true }]]));
        }
        stage.addEventListener('pointerdown', startDrag);
        stage.addEventListener('pointermove', moveDrag);
        stage.addEventListener('pointerup', endDrag);
        stage.addEventListener('pointercancel', endDrag);
        stage.addEventListener('keydown', event => {
            if (event.code === 'Space' && !event.target.closest('button')) { state.spaceDown = true; event.preventDefault(); }
            if (!canWrite() || !['ArrowUp','ArrowDown','ArrowLeft','ArrowRight'].includes(event.key)) return;
            const focused = event.target.closest?.('[data-node]');
            if (focused?.dataset.node) state.selected = focused.dataset.node;
            if (!state.selected) return;
            const node = nodeById(state.selected); if (!node) return;
            event.preventDefault(); state.keyboardMove = true;
            const pos = state.keyboardPosition || clone(node.placements[state.view] || nextPosition(state.view, node.id));
            const delta = event.shiftKey ? 8 : 24;
            if (event.key === 'ArrowLeft') pos.x -= delta;
            if (event.key === 'ArrowRight') pos.x += delta;
            if (event.key === 'ArrowUp') pos.y -= delta;
            if (event.key === 'ArrowDown') pos.y += delta;
            const button = root.querySelector(`[data-id="${CSS.escape(node.id)}"]`);
            const geometry = state.sceneGeometry;
            state.keyboardPosition = { x: Math.max(geometry.originX + 80, Math.min(geometry.originX + geometry.width - 80, pos.x)), y: Math.max(geometry.originY + 70, Math.min(geometry.originY + geometry.height - 70, pos.y)), pinned: true };
            if (button) { button.style.left = `${state.keyboardPosition.x - geometry.originX}px`; button.style.top = `${state.keyboardPosition.y - geometry.originY}px`; }
        });
        stage.addEventListener('keyup', event => { if (event.code === 'Space') state.spaceDown = false; });
        stage.addEventListener('focusout', () => { if (state.keyboardMove && state.selected && state.keyboardPosition) { const pos = state.keyboardPosition; state.keyboardPosition = null; state.keyboardMove = false; void persistLayout(new Map([[state.selected, pos]])); } });

        function openDialog(title, content, onRender) {
            if (state.disposed || !root.isConnected) return null;
            const dialog = q('[data-dialog]');
            if (state.importOpen && dialog.dataset.infraOwner === state.importDialogOwner) { state.importOpen = false; state.importDialogOwner = ''; }
            dialog.dataset.infraOwner = String(++state.dialogGeneration);
            q('[data-dialog-content]').innerHTML = `<p class="infra-kicker">${esc(t('desktop.infra_workspace'))}</p><h2 id="infra-dialog-title-${esc(safeWindowId)}">${esc(title)}</h2>${content}`;
            if (!dialog.open) dialog.showModal();
            dialog.querySelector('.infra-dialog-close')?.addEventListener('click', event => { event.preventDefault(); dialog.close(); }, { once: true });
            dialog.onclick = event => { if (event.target === dialog) dialog.close(); };
            onRender?.(dialog);
            return dialog;
        }
        function openNodeDialog(kind = 'device') {
            if (!canWrite()) return;
            const dialog = openDialog(t('desktop.infra_new_node'), `<form data-new-node><label class="infra-field"><span>${esc(t('desktop.infra_name'))}</span><input name="name" maxlength="120" required autofocus></label><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_type'))}</span><select name="kind" data-kind-select>${KINDS.map(value => `<option value="${value}" ${value === kind ? 'selected' : ''}>${esc(labelForKind(value))}</option>`).join('')}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_icon'))}</span><select name="icon" data-icon-select>${ICON_LIST.map(value => `<option value="${value}" ${value === (DEFAULT_ICONS[kind] || 'generic-device') ? 'selected' : ''}>${esc(t(iconLabelKey(value)))}</option>`).join('')}</select></label></div><div class="infra-form-actions"><button type="button" data-close>${esc(t('desktop.cancel'))}</button><button class="infra-button-primary" type="submit">${esc(t('desktop.infra_create_node'))}</button></div></form>`, dialog => {
                dialog.querySelector('[data-close]').addEventListener('click', () => dialog.close());
                const kindSelect = dialog.querySelector('[data-kind-select]'), iconSelect = dialog.querySelector('[data-icon-select]');
                kindSelect.addEventListener('change', () => { if (iconSelect.dataset.userChosen !== 'true') iconSelect.value = DEFAULT_ICONS[kindSelect.value] || 'generic-device'; });
                iconSelect.addEventListener('change', () => { iconSelect.dataset.userChosen = 'true'; });
                dialog.querySelector('[data-new-node]').addEventListener('submit', event => { event.preventDefault(); void createNode(new FormData(event.currentTarget), dialog.dataset.infraOwner); });
            });
        }
        async function createNode(form, owner) {
            if (!canWrite() || state.busy.has('create')) return;
            const body = { id: '', version: 0, kind: form.get('kind'), name: String(form.get('name') || '').trim(), icon: iconId(form.get('icon')), profile: { tags: [], interfaces: [], links: [], app_bindings: [], custom: {} }, placements: {}, sources: [] };
            if (!body.name) return;
            setBusy('create', true);
            try {
                const result = await request('/nodes', { method: 'POST', body });
                const node = result.node || result;
                if (node.id) { replaceNode(node); if (ownsDialog(owner)) state.selected = node.id; state.drafts.set(node.id, clone(node)); closeOwnedDialog(owner); paintAll(); }
                else await loadSnapshot();
                setNotice(t('desktop.infra_node_created'), 'success');
            } catch (error) { setNotice(messageFor(error), 'error'); }
            finally { setBusy('create', false); }
        }
        const replaceNode = node => {
            const normalized = { ...node, icon: iconId(node.icon), profile: { tags: [], interfaces: [], links: [], app_bindings: [], custom: {}, ...(node.profile || {}) }, placements: node.placements || {}, sources: node.sources || [], app_links: node.app_links || [], available_actions: node.available_actions || [] };
            const index = state.nodes.findIndex(item => item.id === normalized.id);
            if (index < 0) state.nodes.push(normalized); else state.nodes[index] = normalized;
        };
        function paintImportDialog() {
            const dialog = q('[data-dialog]');
            if (state.disposed || !state.importOpen || !dialog?.open || !ownsDialog(state.importDialogOwner)) return;
            const rows = state.candidates.map(candidate => {
                const node = candidate.node || {};
                const selectedNode = state.importBindings.get(candidate.id) || '';
                const bindableNodes = candidate.source?.kind === 'registry' ? state.nodes.filter(existing => ['device', 'vm', 'container'].includes(existing.kind)) : state.nodes;
                const candidateName = node.name || t('desktop.infra_unnamed_candidate');
                const checkboxLabel = t('desktop.infra_select_candidate').replace('{name}', candidateName);
                return `<article class="infra-candidate"><label class="infra-candidate-check"><input type="checkbox" data-candidate="${esc(candidate.id)}" aria-label="${esc(checkboxLabel)}" ${state.importSelection.has(candidate.id) ? 'checked' : ''}><span class="infra-checkbox-mark"></span></label><div class="infra-candidate-icon">${icon(node.icon)}</div><div class="infra-candidate-copy"><strong>${esc(candidateName)}</strong><span>${esc(t(`desktop.infra_source_${candidate.source?.kind}`))} · ${esc(candidate.source?.name || candidate.source?.instance || '')}</span><small>${esc([node.profile?.vendor, node.profile?.model, node.profile?.ip_address].filter(Boolean).join(' · '))}</small></div><span class="infra-state-pill" data-state="${esc(candidate.source?.state || 'unknown')}"><i></i>${esc(statusLabel(candidate.source?.state || 'unknown'))}</span><label class="infra-field infra-bind-field"><span>${esc(t('desktop.infra_link_existing'))}</span><select data-bind="${esc(candidate.id)}"><option value="">${esc(t('desktop.infra_import_new'))}</option>${bindableNodes.map(existing => `<option value="${esc(existing.id)}" ${selectedNode === existing.id ? 'selected' : ''}>${esc(existing.name)}</option>`).join('')}</select></label></article>`;
            }).join('');
            dialog.querySelector('[data-candidate-sources]').innerHTML = state.sources.map(source => `<span class="infra-source-chip" data-state="${esc(source.state)}"><i></i>${esc(t(`desktop.infra_source_${source.kind}`))} · ${esc(source.name || source.instance || source.kind)}</span>`).join('');
            dialog.querySelector('.infra-candidate-list').innerHTML = rows || `<div class="infra-empty-candidates"><span aria-hidden="true">⌁</span><strong>${esc(t('desktop.infra_no_candidates'))}</strong><p>${esc(t('desktop.infra_no_candidates_hint'))}</p></div>`;
            dialog.querySelectorAll('[data-candidate]').forEach(box => box.addEventListener('change', () => { if (box.checked) state.importSelection.add(box.dataset.candidate); else state.importSelection.delete(box.dataset.candidate); updateImportCount(dialog); }));
            dialog.querySelectorAll('[data-bind]').forEach(select => select.addEventListener('change', () => state.importBindings.set(select.dataset.bind, select.value)));
            updateImportCount(dialog);
        }
        const updateImportCount = dialog => { const count = state.importSelection.size; const label = dialog.querySelector('[data-import-count]'); if (label) label.textContent = t('desktop.infra_selected_count').replace('{count}', count); const button = dialog.querySelector('[data-import]'); if (button) button.disabled = !count || !canWrite() || state.busy.has('import'); };
        async function importSelected() {
            if (!canWrite() || !state.importSelection.size || state.busy.has('import')) return;
            const owner = state.importDialogOwner;
            setBusy('import', true);
            const ids = [...state.importSelection];
            try {
                for (const candidate_id of ids) {
                    await request('/import', { method: 'POST', body: { candidate_id, ...(state.importBindings.get(candidate_id) ? { node_id: state.importBindings.get(candidate_id) } : {}) } });
                    state.importSelection.delete(candidate_id); state.importBindings.delete(candidate_id);
                    state.candidates = state.candidates.filter(candidate => candidate.id !== candidate_id);
                    paintImportDialog();
                }
                closeOwnedDialog(owner); if (state.importDialogOwner === owner) { state.importOpen = false; state.importDialogOwner = ''; }
                state.importSelection.clear(); state.importBindings.clear();
                setNotice(t('desktop.infra_imported').replace('{count}', ids.length), 'success');
                await loadSnapshot();
            } catch (error) {
                if (error.name !== 'AbortError') setNotice(messageFor(error), 'error');
                if (!state.disposed && state.importDialogOwner === owner) await loadCandidates(true, owner);
                paintImportDialog();
            }
            finally { setBusy('import', false); }
        }
        function openEdgeDialog(edgeId = '', fromId = state.selected) {
            if (!canWrite()) return;
            const edge = state.edges.find(item => item.id === edgeId);
            const options = state.nodes.map(node => `<option value="${esc(node.id)}">${esc(node.name)}</option>`).join('');
            const dialog = openDialog(edge ? t('desktop.infra_edit_relation') : t('desktop.infra_add_relation'), `<form data-edge-form><input type="hidden" name="id" value="${esc(edge?.id || '')}"><input type="hidden" name="version" value="${esc(edge?.version || 0)}"><label class="infra-field"><span>${esc(t('desktop.infra_relation_from'))}</span><select name="from">${options}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_kind'))}</span><select name="kind">${EDGE_KINDS.map(kind => `<option value="${kind}">${esc(t(`desktop.infra_edge_${kind}`))}</option>`).join('')}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_to'))}</span><select name="to">${options}</select></label><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_relation_from_port'))}</span><input name="from_port"></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_to_port'))}</span><input name="to_port"></label></div><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_relation_speed'))}</span><input name="speed"></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_label'))}</span><input name="label"></label></div><div class="infra-form-actions">${edge ? `<button type="button" data-delete-edge class="infra-danger-button">${esc(t('desktop.infra_remove_relation'))}</button>` : ''}<span class="infra-flex-spacer"></span><button type="button" data-close>${esc(t('desktop.cancel'))}</button><button type="submit" class="infra-button-primary">${esc(t('desktop.save'))}</button></div></form>`, dlg => {
                const form = dlg.querySelector('[data-edge-form]');
                form.elements.from.value = edge?.from || fromId || '';
                form.elements.to.value = edge?.to || '';
                form.elements.kind.value = edge?.kind || 'ethernet';
                for (const key of ['label', 'from_port', 'to_port', 'speed']) form.elements[key].value = edge?.[key] || '';
                dlg.querySelector('[data-close]').addEventListener('click', () => dlg.close());
                form.addEventListener('submit', event => { event.preventDefault(); void saveEdge(new FormData(form), dlg.dataset.infraOwner); });
                dlg.querySelector('[data-delete-edge]')?.addEventListener('click', () => void deleteEdge(edge, dlg.dataset.infraOwner));
            });
        }
        async function saveEdge(form, owner) {
            if (!canWrite()) return;
            const id = String(form.get('id') || ''), body = { id: '', version: 0, from: form.get('from'), to: form.get('to'), kind: form.get('kind') };
            const busyKey = `edge-${id || 'new'}`;
            if (state.busy.has(busyKey)) return;
            if (id) { body.id = id; body.version = Number(form.get('version') || 0); }
            for (const key of ['label','from_port','to_port','speed']) if (form.get(key)) body[key] = String(form.get(key));
            setBusy(busyKey, true);
            try {
                const result = await request(id ? `/edges/${encodeURIComponent(id)}` : '/edges', { method: id ? 'PUT' : 'POST', body });
                const edge = result.edge || result;
                if (edge.id) { const index = state.edges.findIndex(item => item.id === edge.id); if (index < 0) state.edges.push(edge); else state.edges[index] = edge; }
                else await loadSnapshot();
                closeOwnedDialog(owner); setNotice(t('desktop.infra_relation_saved'), 'success'); paintAll();
            } catch (error) { setNotice(messageFor(error), 'error'); }
            finally { setBusy(busyKey, false); }
        }
        async function deleteEdge(edge, owner) {
            if (!edge || !canWrite()) return;
            const confirmation = await confirmAction(t('desktop.infra_remove_relation'), t('desktop.infra_remove_relation_confirm'), { withOwner: true });
            if (!confirmation.confirmed) return;
            const dialogOwner = confirmation.owner || owner;
            try { await request(`/edges/${encodeURIComponent(edge.id)}?version=${encodeURIComponent(edge.version)}`, { method: 'DELETE' }); state.edges = state.edges.filter(item => item.id !== edge.id); closeOwnedDialog(dialogOwner); paintAll(); }
            catch (error) { setNotice(messageFor(error), 'error'); }
        }
        async function confirmAction(title, message, {withOwner = false, confirmLabel} = {}) {
            const initialOwner = q('[data-dialog]')?.dataset.infraOwner || '';
            if (typeof ctx.showConfirmModal === 'function') {
                const confirmed = !!(await ctx.showConfirmModal(title, message, confirmLabel));
                return withOwner ? { confirmed, owner: initialOwner } : confirmed;
            }
            return new Promise(resolve => {
                const dialog = openDialog(title, `<p class="infra-dialog-intro">${esc(message)}</p><div class="infra-form-actions"><button type="button" data-answer="no">${esc(t('desktop.cancel'))}</button><button type="button" data-answer="yes" class="infra-button-primary">${esc(confirmLabel || t('desktop.ok'))}</button></div>`, dlg => {
                    const owner = dlg.dataset.infraOwner;
                    let done = false;
                    const finish = confirmed => { if (done) return; done = true; resolve(withOwner ? { confirmed, owner } : confirmed); };
                    dlg.querySelector('[data-answer="no"]').addEventListener('click', () => { finish(false); dlg.close(); });
                    dlg.querySelector('[data-answer="yes"]').addEventListener('click', () => { finish(true); dlg.close(); });
                    dlg.addEventListener('close', () => finish(false), { once: true });
                });
                if (!dialog) { resolve(withOwner ? { confirmed: false, owner: initialOwner } : false); return; }
                dialog?.querySelector('[data-answer="no"]')?.focus();
            });
        }
        async function deleteNode(id) {
            const node = nodeById(id); if (!node || !canWrite()) return;
            if (!await confirmAction(t('desktop.infra_remove_node'), t('desktop.infra_remove_node_confirm').replace('{name}', node.name))) return;
            try { await request(`/nodes/${encodeURIComponent(id)}?version=${encodeURIComponent(node.version)}`, { method: 'DELETE' }); state.nodes = state.nodes.filter(item => item.id !== id); state.edges = state.edges.filter(edge => edge.from !== id && edge.to !== id); state.drafts.delete(id); state.dirty.delete(id); state.selected = ''; setNotice(t('desktop.infra_node_removed'), 'success'); paintAll(); }
            catch (error) { setNotice(messageFor(error), 'error'); }
        }
        async function loadSourceEdit(id, source) {
            if (!canEditSource() || !SOURCE_KINDS.includes(source)) return;
            if (source === 'registry') void loadCredentials();
            try { const result = await request(`/nodes/${encodeURIComponent(id)}/source-edit?source=${encodeURIComponent(source)}`); state.sourceDrafts.set(id, result); state.sourceDirty.delete(id); state.sourceReviews.delete(id); if (sourceEditPending(result)) { state.sourcePending.add(id); state.sourceBlocked.add(id); } else { state.sourcePending.delete(id); state.sourceBlocked.delete(id); } paintInspector(); }
            catch (error) { setNotice(messageFor(error), 'error'); }
        }
        const sourceEditPending = result => result?.pending === true || result?.fields?.pending === true;
        const sourceFieldValue = (source, key, value) => {
            if (key !== 'tags') return key === 'port' ? Number(value) : value;
            if (source === 'registry') return String(value).split(',').map(item => item.trim()).filter(Boolean);
            if (source === 'proxmox') return String(value).split(/[;,]/).map(item => item.trim()).filter(Boolean).join('; ');
            return value;
        };
        function reapplySourceDraft(id) {
            const review = state.sourceReviews.get(id), draft = state.sourceDrafts.get(id);
            if (!review || !draft || !canEditSource()) return;
            state.sourceDrafts.set(id, { ...draft, version: review.latest.version, fields: clone(draft.fields) });
            state.sourceReviews.delete(id); state.sourceBlocked.delete(id); state.sourcePending.delete(id); state.sourceDirty.add(id);
            setNotice(t('desktop.infra_reapply_ready'), 'info'); paintInspector();
        }
        function reapplyNodeDraft(id) {
            const review = state.nodeReviews.get(id), draft = state.drafts.get(id);
            if (!review || !draft || !canWrite()) return;
            draft.version = review.latest.version; state.drafts.set(id, draft); state.nodeReviews.delete(id); state.dirty.add(id);
            state.draftRevision.set(id, (state.draftRevision.get(id) || 0) + 1);
            setNotice(t('desktop.infra_reapply_ready'), 'info'); paintInspector();
        }
        async function reviewSourceEdit(id) {
            const draft = state.sourceDrafts.get(id); if (!draft || !canEditSource()) return;
            try {
                const latest = await request(`/nodes/${encodeURIComponent(id)}/source-edit?source=${encodeURIComponent(draft.source)}`);
                const submitted = {};
                q('[data-inspector]').querySelectorAll('[data-source-field]').forEach(input => {
                    const key = input.dataset.sourceField;
                    submitted[key] = sourceFieldValue(draft.source, key, input.value);
                });
                if (sourceEditPending(latest)) {
                    state.sourceDrafts.set(id, { ...latest, fields: submitted }); state.sourceDirty.add(id); state.sourceBlocked.add(id); state.sourcePending.add(id);
                    state.sourceReviews.delete(id);
                    setNotice(t('desktop.infra_source_still_pending'), 'info');
                    paintInspector(); return;
                }
                const confirmed = Object.entries(submitted).every(([key, value]) => JSON.stringify(latest.fields?.[key]) === JSON.stringify(value));
                if (confirmed) {
                    state.sourceDrafts.delete(id); state.sourceDirty.delete(id); state.sourceBlocked.delete(id); state.sourcePending.delete(id); state.sourceReviews.delete(id);
                    setNotice(t('desktop.infra_source_review_confirmed'), 'success');
                } else {
                    state.sourceDrafts.set(id, { ...draft, version: latest.version, fields: submitted }); state.sourceBlocked.add(id); state.sourcePending.delete(id); state.sourceReviews.set(id, { latest, submitted });
                    setNotice(t('desktop.infra_source_reviewed'), 'info');
                }
                paintInspector();
            } catch (error) { setNotice(messageFor(error), 'error'); }
        }
        async function saveSourceEdit(id) {
            const draft = state.sourceDrafts.get(id); if (!draft || !canEditSource() || !draft.editable || state.sourceBlocked.has(id)) return;
            const busyKey = `source-${id}`;
            if (state.busy.has(busyKey)) return;
            const fields = {};
            q('[data-inspector]').querySelectorAll('[data-source-field]').forEach(input => {
                const key = input.dataset.sourceField;
                fields[key] = sourceFieldValue(draft.source, key, input.value);
            });
            const revision = state.sourceRevision.get(id) || 0;
            const body = { source: draft.source, fields, version: draft.version };
            setBusy(busyKey, true);
            try {
                const result = await request(`/nodes/${encodeURIComponent(id)}/source-edit?source=${encodeURIComponent(draft.source)}`, { method: 'PUT', body });
                if (result.status === 'unknown' || result.status === 'pending' || sourceEditPending(result)) {
                    state.sourceBlocked.add(id);
                    if (result.status === 'pending' || sourceEditPending(result)) {
                        state.sourcePending.add(id);
                        setNotice(t('desktop.infra_source_pending'), 'info');
                    } else setNotice(t('desktop.infra_source_outcome_unknown'), 'error');
                    paintInspector(); return;
                }
                if ((state.sourceRevision.get(id) || 0) === revision) {
                    state.sourceDrafts.delete(id); state.sourceDirty.delete(id); state.sourceBlocked.delete(id); state.sourcePending.delete(id); state.sourceReviews.delete(id);
                    await loadSnapshot(); setNotice(t('desktop.infra_source_saved'), 'success');
                } else {
                    const latest = await request(`/nodes/${encodeURIComponent(id)}/source-edit?source=${encodeURIComponent(draft.source)}`);
                    const newer = state.sourceDrafts.get(id);
                    if (newer) newer.version = latest.version;
                    state.sourceDirty.add(id); await loadSnapshot();
                    setNotice(t('desktop.infra_source_saved_newer_draft'), 'info');
                }
            }
            catch (error) {
                if (error.status === 409) { state.sourceBlocked.add(id); state.sourcePending.delete(id); state.sourceReviews.delete(id); paintInspector(); setNotice(messageFor(error), 'error'); }
                else if (uncertainMutation(error)) { state.sourceBlocked.add(id); state.sourcePending.delete(id); paintInspector(); setNotice(t('desktop.infra_source_outcome_unknown'), 'error'); }
                else setNotice(messageFor(error), 'error');
            } finally { setBusy(busyKey, false); }
        }
        function openActionMenu(id, source) {
            const node = nodeById(id); if (!node || !canControl()) return;
            const sourceDescriptor = typeof source === 'string' ? (node.sources || []).find(item => item.kind === source) : source;
            if (!sourceDescriptor) return;
            const actions = sourceActions(sourceDescriptor.kind).filter(action => allowedAction(node, action, sourceDescriptor));
            if (!actions.length) return;
            const blocked = actions.filter(action => state.actionBlocks.has(actionBlockKey(id, sourceDescriptor, action)));
            const dialog = openDialog(t('desktop.infra_source_actions'), `<p class="infra-dialog-intro">${esc(t('desktop.infra_action_warning'))}</p><div class="infra-action-list">${actions.map(action => { const needsReview = state.actionBlocks.has(actionBlockKey(id, sourceDescriptor, action)); return `<button type="button" data-run-action="${esc(action)}" data-action-node="${esc(id)}" data-source-identity="${esc(sourceIdentity(sourceDescriptor))}" class="${['stop','restart','shutdown','reboot'].includes(action) ? 'is-caution' : ''}" ${needsReview ? 'disabled' : ''}><span>${esc(t(`desktop.infra_action_${action}`))}</span><small>${esc(t(`desktop.infra_action_help_${action}`))}</small><b aria-hidden="true">›</b></button>`; }).join('')}</div>${blocked.length ? `<section class="infra-action-review" role="status"><strong>${esc(t('desktop.infra_action_review_required'))}</strong>${blocked.map(action => `<div><span>${esc(t(`desktop.infra_action_${action}`))} · ${esc(state.actionBlocks.get(actionBlockKey(id, sourceDescriptor, action))?.status || 'unknown')}</span><button type="button" data-action-rearm="${esc(action)}">${esc(t('desktop.infra_action_rearm'))}</button></div>`).join('')}</section>` : ''}`, dlg => {
                dlg.querySelectorAll('[data-run-action]').forEach(button => button.addEventListener('click', () => void runAction(id, sourceDescriptor, button.dataset.runAction, dlg)));
                dlg.querySelectorAll('[data-action-rearm]').forEach(button => button.addEventListener('click', () => {
                    state.actionBlocks.delete(actionBlockKey(id, sourceDescriptor, button.dataset.actionRearm));
                    setNotice(t('desktop.infra_action_rearmed'), 'info');
                    openActionMenu(id, sourceDescriptor);
                }));
            });
            return dialog;
        }
        async function runAction(id, source, action, dialog = null) {
            const node = nodeById(id);
            const sourceDescriptor = typeof source === 'string' ? (node?.sources || []).find(item => item.kind === source) : source;
            const blockKey = sourceDescriptor && actionBlockKey(id, sourceDescriptor, action);
            const busyKey = `action-${id}-${sourceDescriptor ? sourceIdentity(sourceDescriptor) : ''}-${action}`;
            let dialogOwner = dialog?.dataset.infraOwner || q('[data-dialog]')?.dataset.infraOwner || '';
            let reviewAllowed = !dialog;
            if (!sourceDescriptor || !allowedAction(node, action, sourceDescriptor) || state.actionBlocks.has(blockKey) || state.busy.has(busyKey)) return;
            setBusy(busyKey, true);
            const request_id = window.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
            try {
                if (['stop','restart','shutdown','reboot'].includes(action)) {
                    const confirmation = await confirmAction(t(`desktop.infra_action_${action}`), t('desktop.infra_confirm_action').replace('{action}', t(`desktop.infra_action_${action}`)).replace('{name}', nodeById(id)?.name || ''), { withOwner: true });
                    if (!confirmation.confirmed) return;
                    dialogOwner = confirmation.owner || dialogOwner;
                    reviewAllowed = true;
                }
                if (state.disposed || !root.isConnected) return;
                const result = await request(`/nodes/${encodeURIComponent(id)}/actions`, { method: 'POST', body: { source: sourceDescriptor.kind, action, request_id, confirmed: true } });
                if (state.disposed || !root.isConnected) return;
                if (['running', 'accepted', 'unknown'].includes(result.status)) {
                    state.actionBlocks.set(blockKey, { status: result.status, request_id });
                    setNotice(result.status === 'unknown' ? t('desktop.infra_action_outcome_unknown') : t('desktop.infra_action_review_required'), result.status === 'unknown' ? 'error' : 'info');
                    if (ownsDialog(dialogOwner) && (!dialog || dialog.open || reviewAllowed)) openActionMenu(id, sourceDescriptor); return;
                }
                setNotice(t(`desktop.infra_action_status_${result.status}`).replace('{status}', result.status), result.status === 'failed' || result.status === 'unknown' ? 'error' : 'success');
                closeOwnedDialog(dialogOwner);
            } catch (error) {
                if (state.disposed || !root.isConnected) return;
                if (uncertainMutation(error)) {
                    state.actionBlocks.set(blockKey, { status: 'unknown', request_id });
                    setNotice(t('desktop.infra_action_outcome_unknown'), 'error');
                    if (ownsDialog(dialogOwner) && (!dialog || dialog.open || reviewAllowed)) openActionMenu(id, sourceDescriptor);
                }
                else setNotice(messageFor(error), 'error');
            }
            finally { setBusy(busyKey, false); }
        }
        function openNodeActions(id) {
            const node = nodeById(id); if (!node) return;
            const content = `<div class="infra-action-list">${canWrite() ? `<button type="button" data-act="edge"><span>${esc(t('desktop.infra_add_relation'))}</span><b>›</b></button><button type="button" data-act="remove" class="is-caution"><span>${esc(t('desktop.infra_remove_node'))}</span><b>›</b></button>` : ''}${canControl() ? (node.sources || []).flatMap(source => sourceActions(source.kind).filter(action => allowedAction(node, action, source)).map(action => `<button type="button" data-act-source="${esc(sourceIdentity(source))}" data-act="${esc(action)}"><span>${esc(t(`desktop.infra_action_${action}`))} · ${esc(t(`desktop.infra_source_${source.kind}`))}</span><b>›</b></button>`)).join('') : ''}</div>`;
            const dialog = openDialog(node.name, content, dlg => {
                dlg.querySelector('[data-act="edge"]')?.addEventListener('click', () => { dlg.close(); openEdgeDialog('', id); });
                dlg.querySelector('[data-act="remove"]')?.addEventListener('click', () => { dlg.close(); void deleteNode(id); });
                dlg.querySelectorAll('[data-act-source]').forEach(button => button.addEventListener('click', () => {
                    dlg.close();
                    const source = (nodeById(id)?.sources || []).find(item => sourceIdentity(item) === button.dataset.actSource);
                    if (source) void runAction(id, source, button.dataset.act);
                }));
            });
            return dialog;
        }
        function chooseEdgeEndpoint(id) {
            if (!state.edgeFrom) { state.edgeFrom = id; setNotice(t('desktop.infra_connect_choose_second'), 'info'); paintStage(); return; }
            const from = state.edgeFrom; state.edgeFrom = ''; state.edgeMode = false;
            q('[data-action="edge-mode"]').setAttribute('aria-pressed', 'false');
            if (from === id) { setNotice(t('desktop.infra_edge_self_error'), 'error'); return; }
            openEdgeDialog('', from); q('[data-dialog-content] select[name="to"]').value = id;
        }
        function openImport() {
            if (!canWrite()) return;
            state.importOpen = true;
            state.candidates = []; state.importSelection.clear(); state.importBindings.clear();
            const dialog = openDialog(t('desktop.infra_import_title'), `<p class="infra-dialog-intro">${esc(t('desktop.infra_import_hint'))}</p><div class="infra-candidate-source-tabs" data-candidate-sources></div><div class="infra-candidate-list"><div class="infra-import-loading">${esc(t('desktop.infra_loading_candidates'))}</div></div><div class="infra-form-actions infra-dialog-actions"><span data-import-count></span><button type="button" data-close>${esc(t('desktop.cancel'))}</button><button type="button" data-import class="infra-button-primary" disabled>${esc(t('desktop.infra_import_selected'))}</button></div>`, dlg => {
                dlg.querySelector('[data-close]').addEventListener('click', () => dlg.close());
                dlg.querySelector('[data-import]').addEventListener('click', () => void importSelected());
            });
            state.importDialogOwner = dialog?.dataset.infraOwner || '';
            if (dialog && !dialog.dataset.importCloseBound) {
                dialog.dataset.importCloseBound = 'true';
                dialog.addEventListener('close', () => {
                    const owner = dialog.dataset.infraOwner;
                    if (state.importDialogOwner === owner) { state.candidates = []; state.importOpen = false; state.importDialogOwner = ''; }
                });
            }
            void loadCandidates();
        }

        function bindEvents() {
            root.addEventListener('click', event => {
                const addList = event.target.closest('[data-list-add]');
                const removeList = event.target.closest('[data-list-remove]');
                if (addList || removeList) {
                    if (!canWrite()) return;
                    const node = nodeById(state.selected), form = q('[data-detail-form]');
                    if (!node || !form) return;
                    const draft = state.drafts.get(node.id) || clone(node);
                    readProfileLists(form, draft, node.id);
                    const family = (addList || removeList).dataset.listAdd || (addList || removeList).dataset.listRemove;
                    draft.profile ||= {};
                    if (family === 'custom') {
                        const rows = state.customRows.get(node.id) || [];
                        if (addList) rows.push({ key: '', value: '' }); else rows.splice(Number(removeList.dataset.index), 1);
                        state.customRows.set(node.id, rows);
                        draft.profile.custom = Object.fromEntries(rows.filter(row => row.key.trim()).map(row => [row.key.trim(), row.value]));
                    } else {
                        const rows = draft.profile[family] || [];
                        if (family === 'interfaces') { if (addList) rows.push({ name: '', ip_address: '', mac_address: '', vlan: '', speed: '' }); else rows.splice(Number(removeList.dataset.index), 1); }
                        if (family === 'links') { if (addList) rows.push({ label: '', url: '' }); else rows.splice(Number(removeList.dataset.index), 1); }
                        draft.profile[family] = rows;
                    }
                    state.drafts.set(node.id, draft); state.dirty.add(node.id);
                    state.draftRevision.set(node.id, (state.draftRevision.get(node.id) || 0) + 1);
                    paintInspector();
                    if (addList) q(`[data-list-row="${CSS.escape(family)}"]:last-child input`)?.focus();
                    return;
                }
                const view = event.target.closest('[data-view]'); if (view) { state.view = view.dataset.view; state.keyboardPosition = null; paintAll(); return; }
                const scope = event.target.closest('[data-scope]'); if (scope) { state.scope = scope.dataset.scope; paintAll(); return; }
                const action = event.target.closest('[data-action]')?.dataset.action; if (!action) return;
                if (action === 'refresh') void loadSnapshot(true);
                else if (action === 'add' || action === 'add-empty') openNodeDialog();
                else if (action === 'add-group') openNodeDialog('group');
                else if (action === 'arrange') void arrange();
                else if (action === 'undo') void undoLayout();
                else if (action === 'retry-layout') void persistLayout(new Map(state.pendingLayout), { view: state.pendingLayoutView });
                else if (action === 'connect') openImport();
                else if (action === 'import') openImport();
                else if (action === 'import-empty') openImport();
                else if (action === 'zoom-in') changeZoom(state.zoom + 0.1);
                else if (action === 'zoom-out') changeZoom(state.zoom - 0.1);
                else if (action === 'zoom-reset') changeZoom(0.9, true);
                else if (action === 'reload-targets') void loadTargets(true);
                else if (action === 'edge-mode') { state.edgeMode = !state.edgeMode; state.edgeFrom = ''; event.target.closest('button').setAttribute('aria-pressed', String(state.edgeMode)); setNotice(t(state.edgeMode ? 'desktop.infra_connect_choose_first' : 'desktop.infra_ready'), 'info'); }
                else if (action === 'more') openQuickActions();
                else if (action === 'node-more') openNodeActions(state.selected);
                else if (action === 'add-edge') openEdgeDialog('', state.selected);
            });
            q('[data-search]').addEventListener('input', event => { state.query = event.target.value; paintStage(); paintTable(); paintTray(); });
            q('[data-kind-filter]').addEventListener('change', event => { state.kindFilter = event.target.value; paintAll(); });
            q('[data-source-filter]').addEventListener('change', event => { state.filterSource = event.target.value; paintAll(); });
            q('[data-table-scroll]').addEventListener('scroll', paintTable, { passive: true });
        }
        function changeZoom(next, reset = false) {
            const scroll = q('.infra-stage-scroll'), stageWidth = stage.offsetWidth || 1, stageHeight = stage.offsetHeight || 1;
            const centerX = (scroll.scrollLeft + scroll.clientWidth / 2) / stageWidth;
            const centerY = (scroll.scrollTop + scroll.clientHeight / 2) / stageHeight;
            state.zoom = Math.max(0.4, Math.min(1.35, Math.round(next * 100) / 100));
            paintStage();
            if (reset) { scroll.scrollLeft = 0; scroll.scrollTop = 0; }
            else {
                scroll.scrollLeft = Math.max(0, centerX * stage.offsetWidth - scroll.clientWidth / 2);
                scroll.scrollTop = Math.max(0, centerY * stage.offsetHeight - scroll.clientHeight / 2);
            }
        }
        function openQuickActions() {
            const dialog = openDialog(t('desktop.infra_actions'), `<div class="infra-action-list"><button type="button" data-quick="import"><span>${esc(t('desktop.infra_import_title'))}</span><small>${esc(t('desktop.infra_import_hint_short'))}</small><b>›</b></button><button type="button" data-quick="new"><span>${esc(t('desktop.infra_new_node'))}</span><small>${esc(t('desktop.infra_manual_hint'))}</small><b>›</b></button></div>`, dlg => {
                dlg.querySelector('[data-quick="import"]').disabled = !canWrite(); dlg.querySelector('[data-quick="new"]').disabled = !canWrite();
                dlg.querySelector('[data-quick="import"]').addEventListener('click', () => { dlg.close(); openImport(); });
                dlg.querySelector('[data-quick="new"]').addEventListener('click', () => { dlg.close(); openNodeDialog(); });
            });
            return dialog;
        }
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
        const { paintToolbar, paintGroups, paintStage, paintTable, paintTray, matchesFilters, paintAll, ensureMapView, paintInspector, readProfileLists } = window.InfraAppViews.create({
            ICON_LIST, KINDS, SOURCE_EDIT_FIELDS, clone, iconId, iconLabelKey,
            sourceActions, sourceIdentity, virtualSlice, sourceState, isPlaced, sceneDimensions,
            validCIDR, validHTTPLink, ctx, esc, t, state,
            safeWindowId, root, q, stage, request, messageFor,
            setNotice, statusLabel, icon, nodeById, groupChildren, hostedServices,
            searchableText, displayedNodes, filteredNodes, canWrite, canEditSource, allowedAction,
            actionReason, labelForKind, setBusy, loadSnapshot, selectNode, nextPosition,
            placeByChoice, saveKeyboardMove, persistLayout, updateUndoButton, replaceNode, openEdgeDialog,
            deleteNode, loadSourceEdit, reapplySourceDraft, reapplyNodeDraft, reviewSourceEdit, saveSourceEdit,
            openActionMenu, chooseEdgeEndpoint
        });
        const visibilityChanged = () => visibility();
        document.addEventListener('visibilitychange', visibilityChanged);
        const windowObserver = typeof MutationObserver === 'function' && win ? new MutationObserver(visibility) : null;
        windowObserver?.observe(win, { attributes: true, attributeFilter: ['style', 'class', 'hidden', 'data-space-hidden'] });
        const intersection = typeof IntersectionObserver === 'function' ? new IntersectionObserver(entries => { state.inView = !!entries[0]?.isIntersecting; visibility(); }, { threshold: 0.01 }) : null;
        intersection?.observe(root);
        const onPolicy = event => {
            const detail = event?.detail || {};
            if (detail.readonly === true || detail.read_only === true || detail.enabled === false) {
                ctx.readonly = true;
                state.caps = { write: false, source_edit: false, control: false };
                state.pollPaused = true;
                abortRequests();
                setNotice(t('desktop.infra_readonly_revoked'), 'info');
                syncMenus(); paintAll();
            } else if (detail.readonly === false || detail.read_only === false || detail.enabled === true) {
                ctx.readonly = false; state.pollPaused = false; abortRequests(); syncMenus(); void loadSnapshot(false); void loadTargets(true); void loadIconManifest();
            }
        };
        document.addEventListener('aurago:desktop-policy', onPolicy);
        bindEvents();
        ctx.setWindowBeforeClose?.(windowId, async () => {
            if (!state.dirty.size && !state.sourceDirty.size && !state.pendingLayout.size && !state.busy.has('layout')) return true;
            return confirmAction(
                t('desktop.app_infra'),
                t('desktop.infra_unsaved_close'),
                { confirmLabel: t('desktop.infra_discard') }
            );
        });
        syncMenus();
        const observer = typeof IntersectionObserver === 'function' ? intersection : null;
        if (!observer) state.inView = true;
        paintAll(); visibility();

        INSTANCES.set(windowId, { ...state, dispose: () => {
            if (state.disposed) return;
            state.disposed = true; clearTimeout(state.timer); abortRequests();
            observer?.disconnect(); windowObserver?.disconnect();
            document.removeEventListener('visibilitychange', visibilityChanged);
            document.removeEventListener('aurago:desktop-policy', onPolicy);
            ctx.setWindowBeforeClose?.(windowId, null);
            ctx.setWindowMenus?.(windowId, []);
            if (q('[data-dialog]')?.open) q('[data-dialog]').close();
            root.replaceChildren();
        } });
    }
    function dispose(windowId) {
        const instance = INSTANCES.get(windowId);
        if (!instance) return;
        if (typeof instance.dispose === 'function') instance.dispose();
        else { instance.disposed = true; clearTimeout(instance.timer); instance.controller?.abort(); instance.requests?.forEach(request => request.abort()); }
        INSTANCES.delete(windowId);
    }

    window.InfraApp = { render, dispose };
})();
