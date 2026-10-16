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
        const t = (key, fallback) => {
            const value = ctx.t?.(key);
            return value && value !== key ? value : (fallback || key);
        };
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
        host.innerHTML = `<section class="infra-app" data-infra-app="${esc(safeWindowId)}" aria-label="${esc(t('desktop.app_infra', 'Infra'))}">
          <header class="infra-topbar">
            <div class="infra-brand"><img class="infra-brand-mark" src="${esc(assetURL('icon'))}" alt="" draggable="false"><div><span class="infra-eyebrow">${esc(t('desktop.infra_workspace', 'INFRASTRUCTURE / WORKSPACE'))}</span><h1>${esc(t('desktop.app_infra', 'Infra'))}</h1></div></div>
            <div class="infra-toolbar" role="toolbar" aria-label="${esc(t('desktop.infra_actions', 'Infrastructure actions'))}">
              <div class="infra-view-switch" role="group" aria-label="${esc(t('desktop.infra_view', 'View'))}">
                <button type="button" data-view="atlas" aria-pressed="true">${esc(t('desktop.infra_atlas', 'Atlas'))}</button>
                <button type="button" data-view="topology" aria-pressed="false">${esc(t('desktop.infra_topology', 'Topology'))}</button>
                <button type="button" data-view="table" aria-pressed="false">${esc(t('desktop.infra_inventory', 'Inventory'))}</button>
              </div>
              <button type="button" class="infra-button infra-button-quiet" data-action="refresh" title="${esc(t('desktop.infra_refresh', 'Refresh'))}"><span aria-hidden="true">↻</span><span>${esc(t('desktop.infra_refresh', 'Refresh'))}</span></button>
              <button type="button" class="infra-button infra-button-secondary" data-action="undo" disabled><span aria-hidden="true">↶</span><span>${esc(t('desktop.infra_undo', 'Undo'))}</span></button>
              <button type="button" class="infra-button infra-button-secondary" data-action="retry-layout" disabled>${esc(t('desktop.infra_retry_positions', 'Retry position save'))}</button>
              <button type="button" class="infra-button infra-button-secondary" data-action="import" disabled>⇣ ${esc(t('desktop.infra_import_title', 'Import'))}</button>
              <button type="button" class="infra-button infra-button-primary" data-action="add" disabled>＋ ${esc(t('desktop.infra_add_node', 'Add node'))}</button>
              <button type="button" class="infra-icon-button" data-action="more" aria-label="${esc(t('desktop.infra_more_actions', 'More actions'))}" title="${esc(t('desktop.infra_more_actions', 'More actions'))}">···</button>
            </div>
          </header>
          <div class="infra-filterbar">
            <label class="infra-search"><span aria-hidden="true">⌕</span><input type="search" data-search aria-label="${esc(t('desktop.infra_search', 'Search infrastructure'))}" placeholder="${esc(t('desktop.infra_search_hint', 'Search name, model, address…'))}" autocomplete="off"></label>
            <label class="infra-select-label"><span>${esc(t('desktop.infra_type', 'Type'))}</span><select data-kind-filter aria-label="${esc(t('desktop.infra_type_filter', 'Filter by type'))}"><option value="">${esc(t('desktop.infra_all_types', 'All types'))}</option>${KINDS.map(kind => `<option value="${kind}">${esc(t(`desktop.infra_kind_${kind}`, kind))}</option>`).join('')}</select></label>
            <label class="infra-select-label"><span>${esc(t('desktop.infra_source', 'Source'))}</span><select data-source-filter aria-label="${esc(t('desktop.infra_source_filter', 'Filter by source'))}"><option value="">${esc(t('desktop.infra_all_sources', 'All sources'))}</option>${SOURCE_KINDS.map(kind => `<option value="${kind}">${esc(t(`desktop.infra_source_${kind}`, kind))}</option>`).join('')}</select></label>
            <div class="infra-live-status" data-live role="status" aria-live="polite"><i></i><span>${esc(t('desktop.infra_loading', 'Loading inventory…'))}</span></div>
          </div>
          <div class="infra-workspace">
            <aside class="infra-sidebar" aria-label="${esc(t('desktop.infra_navigation', 'Infrastructure navigation'))}">
              <div class="infra-side-heading"><span>${esc(t('desktop.infra_inventory', 'Inventory'))}</span><span data-count class="infra-count">0</span></div>
              <nav class="infra-scope-nav" aria-label="${esc(t('desktop.infra_scopes', 'Inventory filters'))}">
                <button type="button" data-scope="all" aria-pressed="true"><span class="infra-nav-glyph">◫</span>${esc(t('desktop.infra_all_assets', 'All assets'))}<span data-scope-count="all"></span></button>
                <button type="button" data-scope="unplaced" aria-pressed="false"><span class="infra-nav-glyph">⌖</span>${esc(t('desktop.infra_unplaced', 'Unplaced'))}<span data-scope-count="unplaced"></span></button>
                <button type="button" data-scope="attention" aria-pressed="false"><span class="infra-nav-glyph">◌</span>${esc(t('desktop.infra_attention', 'Needs attention'))}<span data-scope-count="attention"></span></button>
              </nav>
              <div class="infra-side-heading infra-groups-heading"><span>${esc(t('desktop.infra_groups', 'Groups'))}</span><button type="button" data-action="add-group" aria-label="${esc(t('desktop.infra_add_group', 'Add group'))}" title="${esc(t('desktop.infra_add_group', 'Add group'))}" disabled>＋</button></div>
              <div class="infra-group-tree" data-groups></div>
              <div class="infra-sidebar-bottom"><span class="infra-source-ledger"><i></i><span data-source-summary>${esc(t('desktop.infra_sources_waiting', 'Sources not checked'))}</span></span><span class="infra-updated" data-updated></span></div>
            </aside>
            <main class="infra-main" data-main>
              <section class="infra-stage-view" data-stage-view aria-label="${esc(t('desktop.infra_atlas', 'Atlas'))}">
                <div class="infra-stage-head"><div><p class="infra-kicker">${esc(t('desktop.infra_atlas_kicker', 'PHYSICAL & LOGICAL MAP'))}</p><h2 data-stage-title>${esc(t('desktop.infra_atlas_title', 'Your infrastructure, at a glance'))}</h2></div><div class="infra-stage-tools"><span data-visible-count></span><div class="infra-zoom" role="group" aria-label="${esc(t('desktop.infra_zoom', 'Map zoom'))}"><button type="button" data-action="zoom-out" aria-label="${esc(t('desktop.infra_zoom_out', 'Zoom out'))}" title="${esc(t('desktop.infra_zoom_out', 'Zoom out'))}">−</button><output data-zoom>90%</output><button type="button" data-action="zoom-in" aria-label="${esc(t('desktop.infra_zoom_in', 'Zoom in'))}" title="${esc(t('desktop.infra_zoom_in', 'Zoom in'))}">＋</button><button type="button" data-action="zoom-reset" aria-label="${esc(t('desktop.infra_zoom_reset', 'Reset zoom'))}" title="${esc(t('desktop.infra_zoom_reset', 'Reset zoom'))}">⤢</button></div><button type="button" data-action="arrange" disabled>${esc(t('desktop.infra_arrange', 'Arrange'))}</button><button type="button" data-action="edge-mode" aria-pressed="false" disabled>${esc(t('desktop.infra_connect', 'Connect'))}</button></div></div>
                <div class="infra-stage-scroll"><div class="infra-stage" data-stage tabindex="0" role="region" aria-label="${esc(t('desktop.infra_stage_hint', 'Infrastructure map. Use arrow keys to move the selected item; Enter saves its position.'))}"><div class="infra-scene" data-scene>
                  <div class="infra-stage-grid" aria-hidden="true"></div><svg class="infra-edges" data-edges aria-hidden="true" focusable="false" viewBox="0 0 100 100" preserveAspectRatio="none"></svg><div class="infra-nodes" data-nodes></div></div>
                  <div class="infra-stage-empty" data-empty hidden><img src="${esc(assetURL('empty-atlas'))}" alt="" draggable="false"><strong>${esc(t('desktop.infra_empty_map', 'A place for everything.'))}</strong><p>${esc(t('desktop.infra_empty_map_hint', 'Add a node, import an observed device, or place an existing asset here.'))}</p><div class="infra-empty-actions"><button type="button" data-action="add-empty" disabled>${esc(t('desktop.infra_add_first', 'Create first entry'))}</button><button type="button" class="infra-button-secondary" data-action="import-empty" disabled>${esc(t('desktop.infra_import_existing', 'Import existing assets'))}</button></div></div>
                </div></div>
                <div class="infra-stage-footer"><span><i class="infra-legend-dot current"></i>${esc(t('desktop.infra_state_current', 'Current'))}<i class="infra-legend-dot stale"></i>${esc(t('desktop.infra_state_stale', 'Stale'))}<i class="infra-legend-dot unknown"></i>${esc(t('desktop.infra_state_unknown', 'Unknown'))}</span><span data-map-help>${esc(t('desktop.infra_map_hint', 'Drag to place · select to inspect · connect to edit a relation'))}</span></div>
              </section>
              <section class="infra-table-view" data-table-view hidden aria-label="${esc(t('desktop.infra_inventory', 'Inventory'))}"><div class="infra-table-head"><div><p class="infra-kicker">${esc(t('desktop.infra_inventory_kicker', 'ASSET REGISTER'))}</p><h2>${esc(t('desktop.infra_inventory', 'Inventory'))}</h2></div><span data-table-total></span></div><div class="infra-table-scroll" data-table-scroll tabindex="0"><table><thead><tr><th>${esc(t('desktop.infra_name', 'Name'))}</th><th>${esc(t('desktop.infra_type', 'Type'))}</th><th>${esc(t('desktop.infra_hardware', 'Hardware'))}</th><th>${esc(t('desktop.infra_address', 'Address'))}</th><th>${esc(t('desktop.infra_source', 'Source'))}</th><th>${esc(t('desktop.infra_status', 'Status'))}</th></tr></thead><tbody data-table-body></tbody></table></div></section>
              <section class="infra-tray" data-tray aria-label="${esc(t('desktop.infra_unplaced', 'Unplaced'))}"><div class="infra-tray-title"><span>${esc(t('desktop.infra_unplaced', 'Unplaced'))}</span><span data-tray-count></span><span class="infra-tray-rule"></span><span>${esc(t('desktop.infra_unplaced_hint', 'Place an asset on the map'))}</span></div><div class="infra-tray-items" data-tray-items></div></section>
            </main>
            <aside class="infra-inspector" data-inspector aria-label="${esc(t('desktop.infra_details', 'Asset details'))}"><div class="infra-inspector-empty"><span aria-hidden="true">◈</span><strong>${esc(t('desktop.infra_select_asset', 'Select an asset'))}</strong><p>${esc(t('desktop.infra_select_asset_hint', 'Choose a node on the map or in the inventory to inspect its details.'))}</p></div></aside>
          </div>
          <dialog class="infra-dialog" data-dialog aria-labelledby="infra-dialog-title-${esc(safeWindowId)}"><form method="dialog"><button class="infra-dialog-close" aria-label="${esc(t('desktop.close', 'Close'))}">×</button></form><div data-dialog-content></div></dialog>
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
            if (error?.status === 409) return t('desktop.infra_error_conflict', 'This asset changed elsewhere. Your draft is still here; refresh to review the latest version.');
            const code = String(error?.body?.error || error?.message || 'generic').toLowerCase().replace(/[^a-z0-9_]/g, '_');
            return t(`desktop.infra_error_${code}`, t('desktop.infra_error_generic', 'The request could not be completed. Your draft has been kept.'));
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
        const statusLabel = status => t(`desktop.infra_state_${status}`, status);
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
        const labelForKind = kind => t(`desktop.infra_kind_${kind}`, kind);
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
                    save.textContent = sourceBusy ? t('desktop.infra_saving', 'Saving…') : t('desktop.infra_save_source', 'Save source metadata');
                }
            }
            if (key === 'create') { const submit = q('[data-new-node] [type="submit"]'); if (submit) submit.disabled = busy; }
            if (key === 'import') { const submit = q('[data-import]'); if (submit) submit.disabled = busy || !state.importSelection.size || !canWrite(); }
            if (key.startsWith('edge-')) { const submit = q('[data-edge-form] [type="submit"]'); if (submit) submit.disabled = busy; }
            if (key.startsWith('node-')) { const submit = q('[data-detail-form] button[type="submit"]'); if (submit) submit.disabled = busy || !state.dirty.has(key.slice('node-'.length)); }
            if (key.startsWith('action-')) q('[data-dialog]')?.querySelectorAll('[data-run-action]').forEach(button => { button.disabled = busy || state.actionBlocks.has(actionBlockKey(button.dataset.actionNode, button.dataset.sourceIdentity, button.dataset.runAction)); });
        };
        const paintToolbar = () => {
            if (state.disposed || !root.isConnected) return;
            q('[data-action="add"]').disabled = !canWrite();
            q('[data-action="add-group"]').disabled = !canWrite();
            q('[data-action="import"]').disabled = !canWrite();
            q('[data-action="add-empty"]').disabled = !canWrite();
            q('[data-action="arrange"]').disabled = !canWrite() || state.busy.has('layout') || !displayedNodes().some(node => !node.placements[state.view]?.pinned);
            q('[data-action="edge-mode"]').disabled = !canWrite() || displayedNodes().length < 2;
            q('[data-action="undo"]').disabled = !canWrite() || !state.layoutUndo || state.busy.has('layout');
            q('[data-action="retry-layout"]').disabled = !canWrite() || !state.pendingLayout.size || state.pendingLayoutView !== state.view || state.busy.has('layout');
            q('[data-action="refresh"]').disabled = state.busy.has('snapshot');
            q('[data-action="refresh"] span:first-child').textContent = state.busy.has('snapshot') ? '◌' : '↻';
        };
        const paintStatus = () => {
            if (state.disposed || !root.isConnected) return;
            const live = q('[data-live]');
            live.classList.toggle('is-error', state.noticeType === 'error');
            live.classList.toggle('is-busy', state.noticeType === 'busy');
            live.querySelector('span').textContent = state.notice || (state.refreshAt ? t('desktop.infra_ready', 'Inventory ready') : t('desktop.infra_loading', 'Loading inventory…'));
            q('[data-count]').textContent = String(state.nodes.length);
            const current = state.sources.filter(source => source.state === 'current').length;
            q('[data-source-summary]').textContent = state.sources.length ? t('desktop.infra_sources_count', `${current} of ${state.sources.length} sources current`).replace('{current}', current).replace('{total}', state.sources.length) : t('desktop.infra_sources_waiting', 'Sources not checked');
            q('[data-updated]').textContent = state.refreshAt ? t('desktop.infra_updated', 'Updated {time}').replace('{time}', new Date(state.refreshAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })) : '';
            q('[data-scope-count="all"]').textContent = String(state.nodes.length);
            q('[data-scope-count="unplaced"]').textContent = String(state.nodes.filter(node => !isPlaced(node, state.view === 'table' ? 'atlas' : state.view)).length);
            q('[data-scope-count="attention"]').textContent = String(state.nodes.filter(node => ['stale', 'unknown', 'removed'].includes(sourceState(node))).length);
        };
        const paintGroups = () => {
            const groups = state.nodes.filter(node => node.kind === 'group').sort((a, b) => a.name.localeCompare(b.name));
            const groupIDs = new Set(groups.map(group => group.id)), children = new Map();
            for (const group of groups) if (groupIDs.has(group.parent_id)) { const list = children.get(group.parent_id) || []; list.push(group); children.set(group.parent_id, list); }
            const renderGroup = (group, depth = 0) => {
                const count = groupChildren(group.id).length, collapsed = state.collapsed.has(group.id), nested = children.get(group.id) || [];
                const row = `<div class="infra-group-item ${state.selected === group.id ? 'is-selected' : ''}" style="--infra-group-depth:${depth}"><button type="button" class="infra-group-toggle" data-collapse="${esc(group.id)}" aria-label="${esc(collapsed ? t('desktop.infra_expand_group', 'Expand group') : t('desktop.infra_collapse_group', 'Collapse group'))}" aria-expanded="${!collapsed}">${collapsed ? '›' : '⌄'}</button><button type="button" class="infra-group-select" data-select="${esc(group.id)}" aria-pressed="${state.selected === group.id}">${icon('group')}<span>${esc(group.name)}</span><small>${count}</small></button></div>`;
                return row + (!collapsed ? nested.map(child => renderGroup(child, depth + 1)).join('') : '');
            };
            const roots = groups.filter(group => !groupIDs.has(group.parent_id));
            q('[data-groups]').innerHTML = roots.length ? roots.map(group => renderGroup(group)).join('') : `<p class="infra-groups-empty">${esc(t('desktop.infra_no_groups', 'No groups yet'))}</p>`;
            q('[data-groups]').querySelectorAll('[data-collapse]').forEach(button => button.addEventListener('click', () => {
                const id = button.dataset.collapse;
                if (state.collapsed.has(id)) state.collapsed.delete(id); else state.collapsed.add(id);
                paintGroups(); paintStage(); paintTray();
            }));
            q('[data-groups]').querySelectorAll('[data-select]').forEach(button => button.addEventListener('click', () => selectNode(button.dataset.select)));
        };
        const paintStage = (restoreFocus = false) => {
            const nodes = displayedNodes();
            const stageNodeLayer = q('[data-nodes]');
            const edgeLayer = q('[data-edges]');
            const scene = q('[data-scene]');
            const pending = state.pendingLayoutView === state.view ? state.pendingLayout : new Map();
            const dimensions = sceneDimensions(state.nodes, state.view, pending);
            state.sceneGeometry = dimensions;
            const viewport = q('.infra-stage-scroll');
            const stageWidth = Math.max(Math.round(dimensions.width * state.zoom), viewport.clientWidth), stageHeight = Math.max(Math.round(dimensions.height * state.zoom), viewport.clientHeight);
            stage.style.width = `${stageWidth}px`; stage.style.height = `${stageHeight}px`;
            scene.style.width = `${dimensions.width}px`; scene.style.height = `${dimensions.height}px`;
            scene.style.transform = `scale(${state.zoom})`;
            edgeLayer.setAttribute('viewBox', `0 0 ${dimensions.width} ${dimensions.height}`);
            edgeLayer.setAttribute('width', dimensions.width); edgeLayer.setAttribute('height', dimensions.height);
            q('[data-zoom]').textContent = `${Math.round(state.zoom * 100)}%`;
            const coords = new Map(nodes.map(node => [node.id, pending.get(node.id) || node.placements?.[state.view] || { x: 0, y: 0 }]));
            edgeLayer.innerHTML = state.edges.filter(edge => coords.has(edge.from) && coords.has(edge.to)).map(edge => {
                const a = coords.get(edge.from), b = coords.get(edge.to);
                const x1 = Number(a.x) - dimensions.originX, y1 = Number(a.y) - dimensions.originY, x2 = Number(b.x) - dimensions.originX, y2 = Number(b.y) - dimensions.originY;
                return `<g class="infra-edge infra-edge-${esc(edge.kind)}" data-edge="${esc(edge.id)}"><path d="M${x1} ${y1} L${x2} ${y2}" vector-effect="non-scaling-stroke"/><circle cx="${x1}" cy="${y1}" r="4"/><circle cx="${x2}" cy="${y2}" r="4"/><title>${esc(edge.label || t(`desktop.infra_edge_${edge.kind}`, edge.kind))}</title></g>`;
            }).join('');
            stageNodeLayer.innerHTML = nodes.map(node => {
                const pos = pending.get(node.id) || node.placements?.[state.view] || { x: 0, y: 0 };
                const selected = state.selected === node.id;
                const stateName = sourceState(node);
                const children = node.kind === 'group' ? groupChildren(node.id).length : 0;
                const hint = [node.profile?.vendor, node.profile?.model, node.profile?.ip_address].filter(Boolean).join(' · ');
                return `<button type="button" class="infra-node infra-node-${esc(node.kind)} ${selected ? 'is-selected' : ''} ${state.layoutUndo?.ids?.includes(node.id) ? 'is-layout-dirty' : ''}" data-node="${esc(node.id)}" data-id="${esc(node.id)}" data-state="${esc(stateName)}" aria-pressed="${selected}" aria-label="${esc(`${node.name}, ${labelForKind(node.kind)}, ${statusLabel(stateName)}`)}" style="left:${Number(pos.x) - dimensions.originX}px;top:${Number(pos.y) - dimensions.originY}px;" draggable="false">
                  <span class="infra-node-ring" aria-hidden="true"></span><span class="infra-node-art">${icon(node.icon)}${node.kind === 'group' ? `<b>${children}</b>` : ''}</span><span class="infra-node-copy"><strong>${esc(node.name)}</strong><small>${esc(hint || labelForKind(node.kind))}</small></span><i class="infra-node-status" aria-hidden="true"></i>
                </button>`;
            }).join('');
            q('[data-empty]').hidden = nodes.length > 0;
            q('[data-action="import-empty"]').disabled = !canWrite();
            q('[data-visible-count]').textContent = t('desktop.infra_visible_count', '{count} shown').replace('{count}', nodes.length);
            q('[data-stage-title]').textContent = state.view === 'topology' ? t('desktop.infra_topology_title', 'How everything connects') : t('desktop.infra_atlas_title', 'Your infrastructure, at a glance');
            q('[data-map-help]').textContent = state.edgeMode ? t('desktop.infra_connect_hint', 'Choose two assets to create a relation') : t('desktop.infra_map_hint', 'Drag to place · select to inspect · connect to edit a relation');
            stageNodeLayer.querySelectorAll('[data-id]').forEach(button => {
                button.addEventListener('click', event => {
                    if (state.dragMoved) { state.dragMoved = false; event.preventDefault(); return; }
                    if (state.edgeMode) return chooseEdgeEndpoint(button.dataset.id);
                    selectNode(button.dataset.id, 'stage');
                });
                button.addEventListener('keydown', event => {
                    if (event.key === 'Enter' && state.keyboardMove) { event.preventDefault(); void saveKeyboardMove(button.dataset.id); }
                });
            });
            if (restoreFocus && state.selected) stageNodeLayer.querySelector(`[data-id="${CSS.escape(state.selected)}"]`)?.focus({ preventScroll: true });
            updateUndoButton();
        };
        const paintTable = () => {
            const items = filteredNodes().sort((a, b) => a.name.localeCompare(b.name));
            const scroll = q('[data-table-scroll]');
            const slice = virtualSlice(items, scroll.scrollTop, scroll.clientHeight || 600);
            q('[data-table-total]').textContent = t('desktop.infra_asset_count', '{count} assets').replace('{count}', items.length);
            const row = node => `<tr data-row="${esc(node.id)}" class="${state.selected === node.id ? 'is-selected' : ''}" tabindex="0" aria-selected="${state.selected === node.id}"><td><button class="infra-table-name" data-select="${esc(node.id)}">${icon(node.icon)}<span>${esc(node.name)}</span></button></td><td>${esc(labelForKind(node.kind))}</td><td>${esc([node.profile?.vendor, node.profile?.model].filter(Boolean).join(' · ') || '—')}</td><td>${esc(node.profile?.ip_address || node.profile?.mac_address || '—')}</td><td>${esc((node.sources || []).map(source => t(`desktop.infra_source_${source.kind}`, source.kind)).join(', ') || t('desktop.infra_manual', 'Manual'))}</td><td><span class="infra-state-pill" data-state="${esc(sourceState(node))}"><i></i>${esc(statusLabel(sourceState(node)))}</span></td></tr>`;
            q('[data-table-body]').innerHTML = `<tr class="infra-virtual-spacer"><td colspan="6" style="height:${slice.before}px"></td></tr>${slice.rows.map(row).join('')}<tr class="infra-virtual-spacer"><td colspan="6" style="height:${slice.after}px"></td></tr>`;
            q('[data-table-body]').querySelectorAll('[data-select]').forEach(button => button.addEventListener('click', () => selectNode(button.dataset.select, 'table')));
            q('[data-table-body]').querySelectorAll('[data-row]').forEach(tr => tr.addEventListener('keydown', event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectNode(tr.dataset.row); } }));
        };
        const paintTray = () => {
            const items = state.nodes.filter(node => !isPlaced(node, state.view === 'table' ? 'atlas' : state.view) && matchesFilters(node));
            q('[data-tray-count]').textContent = String(items.length);
            q('[data-tray]').hidden = state.view === 'table' || items.length === 0;
            q('[data-tray-items]').innerHTML = items.map(node => `<button type="button" class="infra-tray-item ${state.selected === node.id ? 'is-selected' : ''}" data-place="${esc(node.id)}" aria-label="${esc(t('desktop.infra_place_asset', 'Place {name} on map').replace('{name}', node.name))}">${icon(node.icon)}<span><strong>${esc(node.name)}</strong><small>${esc(labelForKind(node.kind))}</small></span><i>＋</i></button>`).join('');
            q('[data-tray-items]').querySelectorAll('[data-place]').forEach(button => button.addEventListener('click', () => {
                state.selected = button.dataset.place;
                state.keyboardMove = true;
                const next = nextPosition(state.view, state.selected);
                void persistLayout(new Map([[state.selected, next]]));
            }));
        };
        const matchesFilters = node => {
            if (state.kindFilter && node.kind !== state.kindFilter) return false;
            if (state.filterSource && !(node.sources || []).some(source => source.kind === state.filterSource)) return false;
            if (state.query && !searchableText(node).includes(state.query.toLocaleLowerCase())) return false;
            if (state.scope === 'attention' && !['stale', 'unknown', 'removed'].includes(sourceState(node))) return false;
            return true;
        };
        const paintView = () => {
            const table = state.view === 'table';
            q('[data-stage-view]').hidden = table;
            q('[data-table-view]').hidden = !table;
            q('[data-tray]').hidden = table || !state.nodes.some(node => !isPlaced(node, state.view) && matchesFilters(node));
            root.querySelectorAll('[data-view]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.view === state.view)));
            root.querySelectorAll('[data-scope]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.scope === state.scope)));
            paintStatus(); paintGroups(); paintStage(); paintTable(); paintTray(); paintInspector(); paintToolbar();
        };
        const paintAll = () => { if (!state.disposed && root.isConnected) paintView(); };
        const ensureMapView = () => {
            if (state.view !== 'table') return;
            state.view = 'atlas'; paintAll();
        };
        const profileFields = [
            ['role', 'role'], ['vendor', 'vendor'], ['model', 'model'], ['serial', 'serial'], ['os', 'os'], ['firmware', 'firmware'],
            ['cpu', 'cpu'], ['ram', 'ram'], ['storage', 'storage'], ['rack_position', 'rack_position'], ['ip_address', 'ip_address'], ['mac_address', 'mac_address']
        ];
        const reviewValue = (value, key = '') => {
            if (key === 'credential_id') {
                const credential = state.credentials.find(item => item.id === value);
                if (credential) return credential.name;
            }
            if (Array.isArray(value)) return value.join(key === 'tags' ? '; ' : ', ');
            if (value && typeof value === 'object') return JSON.stringify(value).slice(0, 180);
            return value == null || value === '' ? t('desktop.infra_unknown', 'Unknown') : String(value);
        };
        const nodeReviewRows = (latest, draft) => {
            const rows = [];
            const add = (key, before, after, prefix = '') => {
                if (JSON.stringify(before) !== JSON.stringify(after)) {
                    const labels = { name: t('desktop.infra_name', 'Name'), kind: t('desktop.infra_type', 'Type'), icon: t('desktop.infra_icon', 'Icon'), parent_id: t('desktop.infra_group', 'Group') };
                    const display = value => key === 'parent_id' ? (nodeById(value)?.name || (value ? String(value) : t('desktop.infra_no_group', 'No group'))) : key === 'icon' ? t(iconLabelKey(value), value) : reviewValue(value, key);
                    rows.push({ label: prefix ? t(`desktop.infra_field_${key}`, key.replaceAll('_', ' ')) : labels[key] || key, current: display(before), draft: display(after) });
                }
            };
            for (const key of ['name', 'kind', 'icon', 'parent_id']) add(key, latest?.[key], draft?.[key]);
            const profileKeys = new Set([...Object.keys(latest?.profile || {}), ...Object.keys(draft?.profile || {})]);
            for (const key of profileKeys) add(key, latest?.profile?.[key], draft?.profile?.[key], 'profile');
            return rows;
        };
        const reviewDiffMarkup = (rows, currentLabel) => rows.length ? `<div class="infra-review-diff">${rows.map(row => `<div class="infra-review-row"><strong>${esc(row.label)}</strong><div><small>${esc(currentLabel)}</small><span>${esc(row.current)}</span></div><div><small>${esc(t('desktop.infra_your_draft', 'Your draft'))}</small><span>${esc(row.draft)}</span></div></div>`).join('')}</div>` : `<p class="infra-hint">${esc(t('desktop.infra_no_field_differences', 'The current values match your draft.'))}</p>`;
        const sourceReviewMarkup = id => {
            const review = state.sourceReviews.get(id); if (!review) return '';

            const rows = Object.keys(review.submitted).map(key => ({ label: t(`desktop.infra_field_${key}`, key.replaceAll('_', ' ')), current: reviewValue(review.latest.fields?.[key], key), draft: reviewValue(review.submitted[key], key) })).filter((row, index) => row.current !== row.draft);
            return `<section class="infra-conflict-review" role="region" aria-label="${esc(t('desktop.infra_source_compare', 'Source change review'))}"><strong>${esc(t('desktop.infra_source_compare', 'Review current source values'))}</strong>${reviewDiffMarkup(rows, t('desktop.infra_current_source', 'Current source'))}<button type="button" data-action="reapply-source-draft" class="infra-button-primary">${esc(t('desktop.infra_reapply_draft', 'Keep and reapply my draft'))}</button></section>`;
        };
        const nodeReviewMarkup = id => {
            const review = state.nodeReviews.get(id); if (!review) return '';
            return `<section class="infra-conflict-review" role="region" aria-label="${esc(t('desktop.infra_node_compare', 'Asset change review'))}"><strong>${esc(t('desktop.infra_node_compare', 'Review current asset values'))}</strong>${reviewDiffMarkup(nodeReviewRows(review.latest, state.drafts.get(id)), t('desktop.infra_current_inventory', 'Current inventory'))}<button type="button" data-action="reapply-node-draft" class="infra-button-primary">${esc(t('desktop.infra_reapply_draft', 'Keep and reapply my draft'))}</button></section>`;
        };
        const inputFor = (family, index, key, value, type = 'text', attrs = '') => `<label class="infra-field"><span>${esc(t(`desktop.infra_field_${key}`, key.replaceAll('_', ' ')))}</span><input type="${type}" data-list-family="${family}" data-index="${index}" data-key="${key}" value="${esc(value ?? '')}" ${attrs} ${!canWrite() ? 'readonly' : ''}></label>`;
        const listEditor = (family, title, fields, rows, addKey, removeKey) => `<div class="infra-list-editor" data-list-editor="${family}"><div class="infra-list-heading"><span>${esc(t(`desktop.infra_${title}`, title))}</span><button type="button" data-list-add="${family}" ${!canWrite() ? 'disabled' : ''}>＋ ${esc(t(`desktop.infra_${addKey}`, 'Add'))}</button></div>${rows.length ? rows.map((row, index) => `<div class="infra-list-row" data-list-row="${family}" data-index="${index}"><div class="infra-fields-grid">${fields.map(([key, type, attrs]) => inputFor(family, index, key, row[key], type, attrs || '')).join('')}</div><button type="button" class="infra-list-remove" data-list-remove="${family}" data-index="${index}" aria-label="${esc(t(`desktop.infra_${removeKey}`, 'Remove'))}" ${!canWrite() ? 'disabled' : ''}>×</button></div>`).join('') : `<p class="infra-hint">${esc(t('desktop.infra_list_empty', 'No entries yet. Add one when needed.'))}</p>`}</div>`;
        const customEditor = (nodeID, profile) => {
            const rows = state.customRows.get(nodeID) || Object.entries(profile.custom || {}).map(([key, value]) => ({ key, value }));
            return `<div class="infra-list-editor" data-list-editor="custom"><div class="infra-list-heading"><span>${esc(t('desktop.infra_custom_fields', 'Custom fields'))}</span><button type="button" data-list-add="custom" ${!canWrite() ? 'disabled' : ''}>＋ ${esc(t('desktop.infra_add_custom_field', 'Add field'))}</button></div>${rows.length ? rows.map((row, index) => `<div class="infra-list-row" data-list-row="custom" data-index="${index}"><div class="infra-fields-grid">${inputFor('custom', index, 'key', row.key, 'text', 'required maxlength="80" pattern="[A-Za-z0-9_.-]+"')}<label class="infra-field"><span>${esc(t('desktop.infra_custom_value', 'Value'))}</span><input data-list-family="custom" data-index="${index}" data-key="value" value="${esc(row.value ?? '')}" maxlength="2048" ${!canWrite() ? 'readonly' : ''}></label></div><button type="button" class="infra-list-remove" data-list-remove="custom" data-index="${index}" aria-label="${esc(t('desktop.infra_remove_custom_field', 'Remove field'))}" ${!canWrite() ? 'disabled' : ''}>×</button></div>`).join('') : `<p class="infra-hint">${esc(t('desktop.infra_list_empty', 'No entries yet. Add one when needed.'))}</p>`}</div>`;
        };
        const appBindingEditor = profile => {
            const bindings = profile.app_bindings || [];
            const selected = new Set(bindings.map(item => `${item.app}\u0000${item.target_id}`));
            const available = new Set(state.appTargets.map(target => `${target.app}\u0000${target.target_id}`));
            const unavailable = bindings.filter(item => !available.has(`${item.app}\u0000${item.target_id}`));
            const targetOption = (target, missing = false) => `<label class="infra-target-option ${missing ? 'is-unavailable' : ''}"><input type="checkbox" data-app-target data-app="${esc(target.app)}" data-target-id="${esc(target.target_id)}" ${selected.has(`${target.app}\u0000${target.target_id}`) ? 'checked' : ''} ${!canWrite() ? 'disabled' : ''}><span><strong>${esc(missing ? `${target.target_id} · ${t('desktop.infra_target_unavailable', 'Unavailable')}` : target.label || target.target_id)}</strong><small>${esc(t(`desktop.infra_app_${target.app.replaceAll('-', '_')}`, target.app))}</small></span></label>`;
            const empty = !state.appTargets.length ? `<p class="infra-hint">${esc(t(state.targetsError ? 'desktop.infra_targets_error' : state.targetsLoaded ? 'desktop.infra_no_app_targets' : 'desktop.infra_targets_loading', state.targetsError ? 'Local app targets could not be loaded.' : state.targetsLoaded ? 'No confirmed local targets are available.' : 'Checking available local targets…'))}</p>` : '';
            return `<div class="infra-list-editor" data-list-editor="app-bindings"><div class="infra-list-heading"><span>${esc(t('desktop.infra_application_links', 'Local app links'))}</span><span>${esc(t('desktop.infra_explicit_links_only', 'Only confirmed local targets are listed.'))}</span></div>${empty}<div class="infra-target-list">${state.appTargets.map(target => targetOption(target)).join('')}${unavailable.map(target => targetOption(target, true)).join('')}</div>${state.targetsError ? `<button type="button" data-action="reload-targets">${esc(t('desktop.infra_retry_targets', 'Try again'))}</button>` : ''}</div>`;
        };
        const appContext = link => ({
            'quick-connect': { device_id: link.target_id },
            'network-cameras': { camera_id: link.target_id },
            'software-store': { app_id: link.target_id },
            meshcore: { conversation_id: link.target_id },
        }[link.app] || { target_id: link.target_id });
        const projectedAppLinks = node => (node.app_links || []).map(link => `<button type="button" class="infra-app-link" data-open-app="${esc(link.app)}" data-target-id="${esc(link.target_id)}">${esc(t(`desktop.infra_app_${link.app.replaceAll('-', '_')}`, link.app))}<span aria-hidden="true">↗</span></button>`).join('');
        const sourceFieldMarkup = ([key, value], editable, sourceKind) => {
            const label = esc(t(`desktop.infra_field_${key}`, key.replaceAll('_', ' ')));
            if (key === 'protocol') return `<label class="infra-field"><span>${label}</span><select data-source-field="${esc(key)}" ${editable && canEditSource() ? '' : 'disabled'}>${['none', 'ssh', 'vnc'].map(protocol => `<option value="${protocol}" ${value === protocol ? 'selected' : ''}>${esc(t(`desktop.infra_protocol_${protocol}`, protocol.toUpperCase()))}</option>`).join('')}</select></label>`;
            if (key === 'credential_id') {
                const current = String(value || '');
                const found = state.credentials.some(credential => credential.id === current);
                const extra = current && !found ? `<option value="${esc(current)}" selected>${esc(t('desktop.infra_credential_unlisted', 'Current credential (not listed)'))} · ${esc(current)}</option>` : '';
                const hint = state.credentialsError ? `<small class="infra-field-hint">${esc(t('desktop.infra_credentials_load_failed', 'Credential list could not be loaded. The current selection is preserved.'))}</small>` : '';
                return `<label class="infra-field"><span>${label}</span><select data-source-field="${esc(key)}" ${editable && canEditSource() ? '' : 'disabled'}><option value="">${esc(t('desktop.infra_no_credential', 'No credential'))}</option>${extra}${state.credentials.map(credential => `<option value="${esc(credential.id)}" ${current === credential.id ? 'selected' : ''}>${esc(credential.name)}${credential.type ? ` · ${esc(credential.type)}` : ''}</option>`).join('')}</select>${hint}</label>`;
            }
            const fieldValue = Array.isArray(value) ? value.join(', ') : value ?? '';
            const placeholder = key === 'tags' && sourceKind === 'proxmox' ? ` placeholder="${esc(t('desktop.infra_pve_tags_hint', 'Separate Proxmox tags with semicolons'))}"` : '';
            return `<label class="infra-field"><span>${label}</span><input data-source-field="${esc(key)}" type="${key === 'port' ? 'number' : 'text'}" value="${esc(fieldValue)}"${placeholder} ${editable && canEditSource() ? '' : 'readonly'} ${key === 'port' ? 'min="1" max="65535"' : ''}></label>`;
        };
        const sourceMarkup = (source, node) => {
            const actions = sourceActions(source.kind);
            const providerHost = source.kind === 'proxmox' && String(source.key || '').startsWith('node:');
            const blocked = actions.filter(action => !allowedAction(node, action, source) && actionReason(node, source, action));
            const reasons = [...new Set(blocked.map(action => actionReason(node, source, action)))];
            const reasonLabel = reasons.length ? t(`desktop.infra_reason_${reasons[0]}`, t('desktop.infra_action_unavailable', 'Some controls are unavailable')) : '';
            const blockedLabel = blocked.map(action => t(`desktop.infra_action_${action}`, action)).join(', ');
            return `<div class="infra-source-row" data-source-row><div class="infra-source-icon" data-source="${esc(source.kind)}">${esc(source.kind.slice(0, 1).toUpperCase())}</div><div class="infra-source-copy"><strong>${esc(t(`desktop.infra_source_${source.kind}`, source.kind))}</strong><span>${esc(source.name || source.instance || source.key || '')}</span>${reasonLabel ? `<small class="infra-action-reason">${esc(blockedLabel)} · ${esc(reasonLabel)}</small>` : ''}</div><span class="infra-state-pill" data-state="${esc(source.state)}"><i></i>${esc(statusLabel(source.state))}</span><small>${esc(source.fetched_at ? new Date(source.fetched_at).toLocaleString() : t('desktop.infra_no_observation', 'No observation yet'))}</small>${canEditSource() && !providerHost ? `<button type="button" data-edit-source="${esc(source.kind)}">${esc(t('desktop.infra_edit_source', 'Edit source'))}</button>` : ''}${actions.some(action => allowedAction(node, action, source)) ? `<button type="button" data-source-action="${esc(source.kind)}" data-source-identity="${esc(sourceIdentity(source))}">${esc(t('desktop.infra_actions', 'Actions'))}</button>` : ''}</div>`;
        };
        const paintInspector = () => {
            if (state.disposed || !root.isConnected) return;
            const node = nodeById(state.selected);
            const panel = q('[data-inspector]');
            panel.classList.toggle('is-empty', !node);
            if (!node) {
                panel.innerHTML = `<div class="infra-inspector-empty"><span aria-hidden="true">◈</span><strong>${esc(t('desktop.infra_select_asset', 'Select an asset'))}</strong><p>${esc(t('desktop.infra_select_asset_hint', 'Choose a node on the map or in the inventory to inspect its details.'))}</p></div>`;
                return;
            }
            const draft = state.drafts.get(node.id) || clone(node);
            state.drafts.set(node.id, draft);
            const sources = node.sources || [];
            const placement = node.placements?.[state.view === 'table' ? 'atlas' : state.view];
            const placementLabel = placement
                ? t('desktop.infra_position', 'x {x}, y {y}').replaceAll('{x}', Math.round(placement.x)).replaceAll('{y}', Math.round(placement.y))
                : t('desktop.infra_unplaced', 'Unplaced');
            const dirty = state.dirty.has(node.id);
            const field = (name, label, value, type = 'text') => `<label class="infra-field"><span>${esc(t(`desktop.infra_field_${label}`, label.replaceAll('_', ' ')))}</span><input data-profile="${esc(name)}" type="${type}" value="${esc(value || '')}" ${!canWrite() ? 'readonly' : ''} autocomplete="off"></label>`;
            const sourceEdit = state.sourceDrafts.get(node.id);
            const services = node.kind === 'service' ? [] : hostedServices(node.id);
            panel.innerHTML = `<div class="infra-detail-head"><div class="infra-detail-icon">${icon(node.icon)}</div><div><p class="infra-kicker">${esc(t(`desktop.infra_kind_${node.kind}`, node.kind))} · ${esc(t('desktop.infra_version', 'Version {version}').replace('{version}', node.version))}</p><h2>${esc(node.name)}</h2></div><button type="button" class="infra-detail-close" data-action="close-inspector" aria-label="${esc(t('desktop.close', 'Close'))}" title="${esc(t('desktop.close', 'Close'))}">×</button><button type="button" class="infra-detail-more" data-action="node-more" aria-label="${esc(t('desktop.infra_more_actions', 'More actions'))}">···</button></div>
              ${dirty ? `<div class="infra-draft-banner" role="status"><i></i>${esc(t('desktop.infra_draft_kept', 'Unsaved changes · preserved across refresh'))}</div>` : ''}${nodeReviewMarkup(node.id)}
              <form class="infra-detail-form" id="infra-detail-form" data-detail-form><div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_identity', 'Identity'))}</h3><span class="infra-state-pill" data-state="${esc(sourceState(node))}"><i></i>${esc(statusLabel(sourceState(node)))}</span></div><label class="infra-field"><span>${esc(t('desktop.infra_name', 'Name'))}</span><input data-node-field="name" value="${esc(draft.name)}" ${!canWrite() ? 'readonly' : ''} maxlength="120" required></label><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_type', 'Type'))}</span><select data-node-field="kind" ${!canWrite() ? 'disabled' : ''}>${KINDS.map(kind => `<option value="${kind}" ${draft.kind === kind ? 'selected' : ''}>${esc(labelForKind(kind))}</option>`).join('')}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_icon', 'Icon'))}</span><select data-node-field="icon" ${!canWrite() ? 'disabled' : ''}>${ICON_LIST.map(key => `<option value="${key}" ${iconId(draft.icon) === key ? 'selected' : ''}>${esc(t(iconLabelKey(key), key.replaceAll('-', ' ')))}</option>`).join('')}</select></label></div></div>
              <div class="infra-form-section"><h3>${esc(t('desktop.infra_hardware', 'Hardware & network'))}</h3><div class="infra-fields-grid">${profileFields.map(([name, label]) => field(name, label, draft.profile?.[name])).join('')}</div>${draft.kind === 'network' ? `<section class="infra-network-segment"><h4>${esc(t('desktop.infra_network_segment', 'Network segment'))}</h4><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_cidr', 'CIDR'))}</span><input data-profile="cidr" type="text" value="${esc(draft.profile?.cidr || '')}" placeholder="192.168.10.0/24" ${!canWrite() ? 'readonly' : ''}></label><label class="infra-field"><span>${esc(t('desktop.infra_vlan', 'VLAN'))}</span><input data-profile="vlan" type="number" min="1" max="4094" value="${esc(draft.profile?.vlan || '')}" placeholder="${esc(t('desktop.infra_unknown', 'Unknown'))}" ${!canWrite() ? 'readonly' : ''}></label></div><p class="infra-hint">${esc(t('desktop.infra_cidr_hint', 'Optional subnet in CIDR notation. Leave blank when unknown.'))}</p></section>` : ''}<label class="infra-field"><span>${esc(t('desktop.infra_tags', 'Tags'))}</span><input data-profile="tags" value="${esc((draft.profile?.tags || []).join(', '))}" placeholder="${esc(t('desktop.infra_tags_hint', 'comma separated'))}" ${!canWrite() ? 'readonly' : ''}></label><label class="infra-field"><span>${esc(t('desktop.infra_notes', 'Notes'))}</span><textarea data-profile="notes" rows="3" ${!canWrite() ? 'readonly' : ''}>${esc(draft.profile?.notes || '')}</textarea></label>${listEditor('interfaces', 'interfaces', [['name','text','required maxlength="64"'],['ip_address','text'],['mac_address','text'],['vlan','text'],['speed','text']], draft.profile?.interfaces || [], 'add_interface', 'remove_interface')}${listEditor('links', 'links', [['label','text','required maxlength="128"'],['url','url','placeholder="https://" required maxlength="2048"']], draft.profile?.links || [], 'add_link', 'remove_link')}${customEditor(node.id, draft.profile || {})}${appBindingEditor(draft.profile || {})}</div>
              <div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_placement', 'Placement'))}</h3><span>${esc(placementLabel)}</span></div><div class="infra-quick-place"><button type="button" data-place-choice="center" ${!canWrite() ? 'disabled' : ''}>${esc(t('desktop.infra_place_center', 'Place at center'))}</button><button type="button" data-place-choice="northwest" ${!canWrite() ? 'disabled' : ''}>${esc(t('desktop.infra_place_northwest', 'Place upper left'))}</button><button type="button" data-place-choice="keyboard" ${!canWrite() ? 'disabled' : ''}>${esc(t('desktop.infra_place_keyboard', 'Move with keyboard'))}</button></div><p class="infra-hint">${esc(t('desktop.infra_keyboard_hint', 'Focus a map item, use arrow keys, then Enter to save.'))}</p></div>
              ${sources.length ? `<div class="infra-form-section"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_sources', 'Observed sources'))}</h3><span>${sources.length}</span></div><div class="infra-source-list">${sources.map(source => sourceMarkup(source, node)).join('')}</div></div>` : `<div class="infra-form-section"><h3>${esc(t('desktop.infra_sources', 'Observed sources'))}</h3><p class="infra-hint">${esc(t('desktop.infra_manual_only', 'This is a manually managed asset.'))}</p></div>`}
              ${(node.app_links || []).length ? `<div class="infra-form-section"><h3>${esc(t('desktop.infra_open_in', 'Open in'))}</h3><div class="infra-app-links">${projectedAppLinks(node)}</div></div>` : ''}
              ${sourceEdit ? `<div class="infra-form-section infra-source-edit"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_source_edit_title', 'Source metadata'))}</h3><span>${esc(t(`desktop.infra_source_${sourceEdit.source}`, sourceEdit.source))}</span></div>${state.sourceDirty.has(node.id) ? `<div class="infra-draft-banner" role="status"><i></i>${esc(t('desktop.infra_source_draft', 'Unsaved source changes'))}</div>` : ''}${state.sourcePending.has(node.id) && !state.sourceDirty.has(node.id) ? `<p class="infra-draft-banner" role="status"><i></i><span>${esc(t('desktop.infra_source_pending', 'A source change is pending confirmation.'))}</span></p>` : ''}<div data-source-form>${sourceReviewMarkup(node.id)}${Object.entries(sourceEdit.fields || {}).filter(([key]) => SOURCE_EDIT_FIELDS[sourceEdit.source]?.includes(key)).map(entry => sourceFieldMarkup(entry, sourceEdit.editable, sourceEdit.source)).join('')}<p class="infra-hint" data-source-reason>${esc(t(`desktop.infra_reason_${sourceEdit.reason}`, sourceEdit.reason || ''))}</p>${state.sourceBlocked.has(node.id) ? `<p class="infra-draft-banner" role="status"><i></i><span>${esc(t(state.sourcePending.has(node.id) ? 'desktop.infra_source_pending_review' : 'desktop.infra_source_review_needed', state.sourcePending.has(node.id) ? 'The source change is saved but still pending. Your draft remains here.' : 'The source needs review before another save. Your draft is preserved.'))}</span></p>` : ''}<div class="infra-form-actions"><button type="button" data-action="close-source-edit">${esc(t('desktop.cancel', 'Cancel'))}</button>${state.sourceBlocked.has(node.id) ? `<button type="button" data-action="review-source">${esc(t(state.sourcePending.has(node.id) ? 'desktop.infra_review_pending_source' : 'desktop.infra_review_source', state.sourcePending.has(node.id) ? 'Check pending source' : 'Review current source'))}</button>` : ''}<button type="button" data-action="save-source" class="infra-button-primary" ${!sourceEdit.editable || !canEditSource() || !state.sourceDirty.has(node.id) || state.sourceBlocked.has(node.id) || state.busy.has(`source-${node.id}`) ? 'disabled' : ''}>${esc(state.busy.has(`source-${node.id}`) ? t('desktop.infra_saving', 'Saving…') : t('desktop.infra_save_source', 'Save source metadata'))}</button></div></div></div>` : ''}
              <div class="infra-form-actions infra-detail-actions">${canWrite() ? `<button type="button" data-action="delete-node" class="infra-danger-button">${esc(t('desktop.infra_remove_node', 'Remove from Infra'))}</button><span class="infra-flex-spacer"></span>${dirty ? `<button type="button" data-action="revert-node">${esc(t('desktop.infra_discard', 'Discard'))}</button>` : ''}<button type="submit" form="infra-detail-form" class="infra-button-primary" ${dirty ? '' : 'disabled'}>${esc(state.busy.has('node-' + node.id) ? t('desktop.infra_saving', 'Saving…') : t('desktop.save', 'Save changes'))}</button>` : `<span class="infra-readonly-note">${esc(t('desktop.infra_read_only', 'Read-only access'))}</span>`}</div></form>
              <div class="infra-relations"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_relations', 'Relations'))}</h3>${canWrite() ? `<button type="button" data-action="add-edge">＋ ${esc(t('desktop.infra_add_relation', 'Add relation'))}</button>` : ''}</div>${state.edges.filter(edge => edge.from === node.id || edge.to === node.id).map(edge => { const other = nodeById(edge.from === node.id ? edge.to : edge.from); return `<div class="infra-relation-row"><span class="infra-edge-kind" data-edge-kind="${esc(edge.kind)}"></span><span>${esc(t(`desktop.infra_edge_${edge.kind}`, edge.kind))}</span><strong>${esc(other?.name || edge.to)}</strong><button type="button" data-edit-edge="${esc(edge.id)}" aria-label="${esc(t('desktop.infra_edit_relation', 'Edit relation'))}" ${!canWrite() || edge.source ? 'disabled' : ''}>✎</button></div>`; }).join('') || `<p class="infra-hint">${esc(t('desktop.infra_no_relations', 'No relations yet.'))}</p>`}</div>
              ${services.length ? `<section class="infra-hosted-services"><div class="infra-form-heading"><h3>${esc(t('desktop.infra_hosted_services', 'Hosted services'))} <span>${services.length}</span></h3><button type="button" data-toggle-host-services="${esc(node.id)}" aria-expanded="${state.expandedHosts.has(node.id)}" aria-controls="infra-hosted-${esc(safeWindowId)}-${esc(String(node.id).replace(/[^a-zA-Z0-9_-]/g, ''))}">${esc(t(state.expandedHosts.has(node.id) ? 'desktop.infra_collapse_hosted_services' : 'desktop.infra_expand_hosted_services', state.expandedHosts.has(node.id) ? 'Collapse services' : 'Expand services'))}</button></div><div id="infra-hosted-${esc(safeWindowId)}-${esc(String(node.id).replace(/[^a-zA-Z0-9_-]/g, ''))}" class="infra-component-list" ${state.expandedHosts.has(node.id) ? '' : 'hidden'}>${services.map(service => `<button type="button" data-hosted-service="${esc(service.id)}">${icon(service.icon)}<span>${esc(service.name)}</span><small>${esc(t('desktop.infra_kind_service', 'Service'))}</small></button>`).join('')}</div></section>` : ''}`;
            if (node.kind === 'service') {
                const logicalEdges = state.edges.filter(edge => ['runs_on', 'depends_on', 'powered_by'].includes(edge.kind) && (edge.from === node.id || edge.to === node.id));
                if (logicalEdges.length) {
                    const details = document.createElement('details'); details.className = 'infra-service-components';
                    const summary = document.createElement('summary'); summary.textContent = t('desktop.infra_service_components', 'Expand service components') + ` (${logicalEdges.length})`; details.append(summary);
                    const list = document.createElement('div'); list.className = 'infra-component-list';
                    for (const edge of logicalEdges) {
                        const other = nodeById(edge.from === node.id ? edge.to : edge.from); if (!other) continue;
                        const button = document.createElement('button'); button.type = 'button'; button.dataset.relatedNode = other.id;
                        button.textContent = `${t(`desktop.infra_edge_${edge.kind}`, edge.kind)} · ${other.name}`; list.append(button);
                    }
                    details.append(list); panel.querySelector('.infra-relations')?.before(details);
                    list.querySelectorAll('[data-related-node]').forEach(button => button.addEventListener('click', () => selectNode(button.dataset.relatedNode)));
                }
            }
            const identity = panel.querySelector('.infra-detail-form .infra-form-section');
            if (identity) {
                const label = document.createElement('label'); label.className = 'infra-field';
                label.innerHTML = `<span>${esc(t('desktop.infra_group', 'Group'))}</span><select data-node-field="parent_id" ${!canWrite() ? 'disabled' : ''}><option value="">${esc(t('desktop.infra_no_group', 'No group'))}</option></select>`;
                const select = label.querySelector('select'), descendants = new Set(), queue = [node.id];
                while (queue.length) { const current = queue.pop(); for (const child of groupChildren(current)) if (!descendants.has(child.id)) { descendants.add(child.id); queue.push(child.id); } }
                state.nodes.filter(group => group.kind === 'group' && group.id !== node.id && !descendants.has(group.id)).forEach(group => {
                    const option = document.createElement('option'); option.value = group.id; option.textContent = group.name; option.selected = group.id === draft.parent_id; select.append(option);
                });
                if (!draft.parent_id) select.value = '';
                identity.append(label);
            }
            panel.querySelector('[data-detail-form]')?.addEventListener('input', updateDraft);
            panel.querySelector('[data-detail-form]')?.addEventListener('change', updateDraft);
            panel.querySelector('[data-detail-form]')?.addEventListener('submit', event => { event.preventDefault(); void saveNode(node.id); });
            panel.querySelectorAll('[data-place-choice]').forEach(button => button.addEventListener('click', () => placeByChoice(node.id, button.dataset.placeChoice)));
            panel.querySelectorAll('[data-open-app]').forEach(button => button.addEventListener('click', () => {
                const link = { app: button.dataset.openApp, target_id: button.dataset.targetId };
                ctx.openApp?.(link.app, appContext(link));
            }));
            panel.querySelector('[data-toggle-host-services]')?.addEventListener('click', event => {
                const hostID = event.currentTarget.dataset.toggleHostServices;
                if (state.expandedHosts.has(hostID)) state.expandedHosts.delete(hostID); else state.expandedHosts.add(hostID);
                paintStage(); paintInspector();
            });
            panel.querySelectorAll('[data-hosted-service]').forEach(button => button.addEventListener('click', () => selectNode(button.dataset.hostedService, 'stage')));
            panel.querySelectorAll('[data-edit-source]').forEach(button => button.addEventListener('click', () => void loadSourceEdit(node.id, button.dataset.editSource)));
            panel.querySelectorAll('[data-source-action]').forEach(button => button.addEventListener('click', () => {
                const source = (nodeById(node.id)?.sources || []).find(item => sourceIdentity(item) === button.dataset.sourceIdentity);
                if (source) openActionMenu(node.id, source);
            }));
            panel.querySelector('[data-action="save-source"]')?.addEventListener('click', () => void saveSourceEdit(node.id));
            panel.querySelector('[data-action="review-source"]')?.addEventListener('click', () => void reviewSourceEdit(node.id));
            panel.querySelector('[data-action="close-source-edit"]')?.addEventListener('click', () => { state.sourceDrafts.delete(node.id); state.sourceDirty.delete(node.id); state.sourceBlocked.delete(node.id); state.sourcePending.delete(node.id); state.sourceReviews.delete(node.id); paintInspector(); });
            panel.querySelector('[data-action="reapply-source-draft"]')?.addEventListener('click', () => reapplySourceDraft(node.id));
            panel.querySelector('[data-action="reapply-node-draft"]')?.addEventListener('click', () => reapplyNodeDraft(node.id));
            panel.querySelector('[data-action="delete-node"]')?.addEventListener('click', () => void deleteNode(node.id));
            panel.querySelector('[data-action="revert-node"]')?.addEventListener('click', () => { state.drafts.set(node.id, clone(node)); state.dirty.delete(node.id); state.nodeReviews.delete(node.id); state.customRows.delete(node.id); paintInspector(); });
            panel.querySelectorAll('[data-edit-edge]').forEach(button => button.addEventListener('click', () => openEdgeDialog(button.dataset.editEdge, node.id)));
            panel.querySelector('[data-action="close-inspector"]')?.addEventListener('click', () => {
                const closedID = state.selected;
                state.selected = '';
                paintGroups(); paintStage(); paintTable(); paintTray(); paintInspector();
                const target = state.view === 'table'
                    ? root.querySelector(`[data-row="${CSS.escape(closedID)}"]`)
                    : root.querySelector(`[data-node="${CSS.escape(closedID)}"]`);
                target?.focus({ preventScroll: true });
            });
            const updateSourceDraft = input => {
                const current = state.sourceDrafts.get(node.id);
                if (current) current.fields[input.dataset.sourceField] = input.value;
                state.sourceReviews.delete(node.id);
                state.sourceDirty.add(node.id);
                state.sourceRevision.set(node.id, (state.sourceRevision.get(node.id) || 0) + 1);
                const save = panel.querySelector('[data-action="save-source"]'); if (save) save.disabled = !canEditSource() || state.sourceBlocked.has(node.id);
                if (!panel.querySelector('.infra-source-edit .infra-draft-banner')) { const banner = document.createElement('div'); banner.className = 'infra-draft-banner'; banner.setAttribute('role', 'status'); banner.innerHTML = `<i></i>${esc(t('desktop.infra_source_draft', 'Unsaved source changes'))}`; panel.querySelector('.infra-source-edit .infra-form-heading')?.after(banner); }
            };
            panel.querySelectorAll('[data-source-field]').forEach(input => {
                input.addEventListener('input', () => updateSourceDraft(input));
                input.addEventListener('change', () => updateSourceDraft(input));
            });
        };
        function updateDraft(event) {
            if (!canWrite() || event.target.dataset.sourceField) return;
            const node = nodeById(state.selected); if (!node) return;
            const draft = state.drafts.get(node.id) || clone(node);
            const target = event.target;
            if (target.dataset.nodeField) draft[target.dataset.nodeField] = target.value;
            if (target.dataset.profile) {
                const key = target.dataset.profile; draft.profile ||= {};
                if (key === 'tags') draft.profile[key] = target.value.split(',').map(value => value.trim()).filter(Boolean);
                else if (key === 'vlan') draft.profile[key] = target.value === '' ? 0 : Number(target.value);
                else draft.profile[key] = target.value;
            }
            const form = q('[data-detail-form]');
            if (form) readProfileLists(form, draft, node.id);
            state.drafts.set(node.id, draft);
            state.dirty.add(node.id);
            state.draftRevision.set(node.id, (state.draftRevision.get(node.id) || 0) + 1);
            const valid = form ? validateDetailForm(form, draft, node.id) : true;
            const save = q('[data-inspector] [type="submit"].infra-button-primary'); if (save) save.disabled = !valid;
            let banner = q('[data-inspector] .infra-draft-banner');
            if (banner?.closest('.infra-source-edit')) banner = null;
            if (!banner) { banner = document.createElement('div'); banner.className = 'infra-draft-banner'; banner.setAttribute('role', 'status'); banner.innerHTML = '<i></i><span></span>'; q('[data-inspector]').prepend(banner); }
            let label = banner.querySelector('span');
            if (!label) { label = document.createElement('span'); banner.append(label); }
            label.textContent = t('desktop.infra_draft_kept', 'Unsaved changes · preserved across refresh');
            if (target.matches('[data-node-field="kind"]')) paintInspector();
        }
        function validateDetailForm(form, draft, id) {
            const cidr = form.querySelector('[data-profile="cidr"]');
            if (cidr) cidr.setCustomValidity(validCIDR(cidr.value) ? '' : t('desktop.infra_invalid_cidr', 'Enter a valid IPv4 or IPv6 subnet in CIDR notation.'));
            for (const input of form.querySelectorAll('[data-list-family="links"][data-key="url"]')) {
                input.setCustomValidity(validHTTPLink(input.value) ? '' : t('desktop.infra_invalid_link', 'Use an absolute HTTP or HTTPS URL without user information.'));
            }
            const seen = new Set();
            for (const input of form.querySelectorAll('[data-list-family="custom"][data-key="key"]')) {
                const key = input.value.trim();
                const duplicate = key && seen.has(key);
                input.setCustomValidity(duplicate ? t('desktop.infra_duplicate_custom_key', 'Custom field names must be unique.') : '');
                if (key) seen.add(key);
            }
            return form.checkValidity();
        }
        function readProfileLists(form, draft, id) {
            draft.profile ||= {};
            draft.profile.interfaces = [...form.querySelectorAll('[data-list-row="interfaces"]')].map(row => Object.fromEntries([...row.querySelectorAll('[data-list-family="interfaces"]')].map(input => [input.dataset.key, input.value.trim()])));
            draft.profile.links = [...form.querySelectorAll('[data-list-row="links"]')].map(row => Object.fromEntries([...row.querySelectorAll('[data-list-family="links"]')].map(input => [input.dataset.key, input.value.trim()])));
            const customRows = [...form.querySelectorAll('[data-list-row="custom"]')].map(row => Object.fromEntries([...row.querySelectorAll('[data-list-family="custom"]')].map(input => [input.dataset.key, input.value])));
            if (customRows.length || state.customRows.has(id)) {
                state.customRows.set(id, customRows);
                draft.profile.custom = Object.fromEntries(customRows.filter(row => row.key).map(row => [row.key.trim(), row.value]));
            }
            draft.profile.app_bindings = [...form.querySelectorAll('[data-app-target]:checked')].map(input => ({ app: input.dataset.app, target_id: input.dataset.targetId }));
        }
        async function saveNode(id) {
            if (!canWrite() || !state.dirty.has(id) || state.busy.has(`node-${id}`)) return;
            const form = q('[data-detail-form]'), draftProfile = state.drafts.get(id)?.profile || {};
            if (form && !validateDetailForm(form, { profile: draftProfile }, id)) { form.reportValidity(); return; }
            const revision = state.draftRevision.get(id) || 0;
            const draft = clone(state.drafts.get(id));
            draft.id = id;
            setBusy(`node-${id}`, true);
            try {
                const result = await request(`/nodes/${encodeURIComponent(id)}`, { method: 'PUT', body: draft });
                const updated = result.node || result;
                if (updated?.id) replaceNode(updated);
                else await loadSnapshot(false);
                if ((state.draftRevision.get(id) || 0) === revision || !state.dirty.has(id)) {
                    state.dirty.delete(id); state.nodeReviews.delete(id); state.drafts.set(id, clone(nodeById(id) || updated));
                    state.customRows.delete(id);
                    setNotice(t('desktop.infra_saved', 'Changes saved'), 'success');
                } else {
                    const newer = state.drafts.get(id) || clone(updated);
                    newer.version = updated?.version ?? newer.version;
                    state.drafts.set(id, newer); state.dirty.add(id);
                    setNotice(t('desktop.infra_saved_newer_draft', 'Saved. Newer edits remain unsaved.'), 'info');
                }
                paintAll();
            } catch (error) {
                if (error.status === 409) {
                    await loadSnapshot(true);
                    const latest = nodeById(id);
                    if (latest) state.nodeReviews.set(id, { latest: clone(latest) });
                    setNotice(t('desktop.infra_node_conflict_review', 'This asset changed in the current inventory. Compare the values below before reapplying your draft.'), 'error');
                    paintAll();
                } else if (error.name !== 'AbortError') setNotice(messageFor(error), 'error');
            }
            finally { setBusy(`node-${id}`, false); }
        }
        async function loadSnapshot(explicit = false) {
            if (state.disposed || !state.visible || state.busy.has('snapshot') || (state.pollPaused && !explicit)) return;
            const generation = state.epoch;
            setBusy('snapshot', true); setNotice(t('desktop.infra_loading', 'Loading inventory…'), 'busy');
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
                    if (state.dirty.has(removed)) setNotice(t('desktop.infra_remote_removed_draft', 'This asset was removed elsewhere. Your local draft is still preserved.'), 'error');
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
                        setNotice(t('desktop.infra_access_unavailable', 'Infra is disabled or unavailable. An administrator can enable it in Desktop settings.'), 'error');
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
            if (choice === 'keyboard') { state.keyboardMove = true; state.selected = id; paintStage(true); setNotice(t('desktop.infra_keyboard_hint', 'Focus a map item, use arrow keys, then Enter to save.'), 'info'); return; }
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
                setNotice(t('desktop.infra_layout_pending_review', 'Resolve the unsaved positions in their original map view first.'), 'error');
                return false;
            }
            const prior = new Map();
            for (const [id] of changes) { const node = nodeById(id); if (!node) continue; prior.set(id, clone(node.placements?.[view] || { x: null, y: null, pinned: false })); }
            const nodes = [...changes].map(([id, position]) => ({ id, version: nodeById(id)?.version || 0, ...position }));
            const undoable = !undo && [...prior.values()].every(position => position.x != null && position.y != null);
            setBusy('layout', true); setNotice(t('desktop.infra_saving_layout', 'Saving map positions…'), 'busy');
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
                state.keyboardMove = false; setNotice(t('desktop.infra_layout_saved', 'Map positions saved'), 'success'); paintAll();
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
            if (!changes.size) { setNotice(t('desktop.infra_undo_unavailable', 'Undo is unavailable for an unplaced item.'), 'error'); return; }
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
            q('[data-dialog-content]').innerHTML = `<p class="infra-kicker">${esc(t('desktop.infra_workspace', 'INFRASTRUCTURE / WORKSPACE'))}</p><h2 id="infra-dialog-title-${esc(safeWindowId)}">${esc(title)}</h2>${content}`;
            if (!dialog.open) dialog.showModal();
            dialog.querySelector('.infra-dialog-close')?.addEventListener('click', event => { event.preventDefault(); dialog.close(); }, { once: true });
            dialog.onclick = event => { if (event.target === dialog) dialog.close(); };
            onRender?.(dialog);
            return dialog;
        }
        function openNodeDialog(kind = 'device') {
            if (!canWrite()) return;
            const dialog = openDialog(t('desktop.infra_new_node', 'Add infrastructure node'), `<form data-new-node><label class="infra-field"><span>${esc(t('desktop.infra_name', 'Name'))}</span><input name="name" maxlength="120" required autofocus></label><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_type', 'Type'))}</span><select name="kind" data-kind-select>${KINDS.map(value => `<option value="${value}" ${value === kind ? 'selected' : ''}>${esc(labelForKind(value))}</option>`).join('')}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_icon', 'Icon'))}</span><select name="icon" data-icon-select>${ICON_LIST.map(value => `<option value="${value}" ${value === (DEFAULT_ICONS[kind] || 'generic-device') ? 'selected' : ''}>${esc(t(iconLabelKey(value), value.replaceAll('-', ' ')))}</option>`).join('')}</select></label></div><div class="infra-form-actions"><button type="button" data-close>${esc(t('desktop.cancel', 'Cancel'))}</button><button class="infra-button-primary" type="submit">${esc(t('desktop.infra_create_node', 'Create node'))}</button></div></form>`, dialog => {
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
                setNotice(t('desktop.infra_node_created', 'Node created'), 'success');
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
                const candidateName = node.name || t('desktop.infra_unnamed_candidate', 'Unnamed candidate');
                const checkboxLabel = t('desktop.infra_select_candidate', 'Select {name} for import').replace('{name}', candidateName);
                return `<article class="infra-candidate"><label class="infra-candidate-check"><input type="checkbox" data-candidate="${esc(candidate.id)}" aria-label="${esc(checkboxLabel)}" ${state.importSelection.has(candidate.id) ? 'checked' : ''}><span class="infra-checkbox-mark"></span></label><div class="infra-candidate-icon">${icon(node.icon)}</div><div class="infra-candidate-copy"><strong>${esc(candidateName)}</strong><span>${esc(t(`desktop.infra_source_${candidate.source?.kind}`, candidate.source?.kind || ''))} · ${esc(candidate.source?.name || candidate.source?.instance || '')}</span><small>${esc([node.profile?.vendor, node.profile?.model, node.profile?.ip_address].filter(Boolean).join(' · '))}</small></div><span class="infra-state-pill" data-state="${esc(candidate.source?.state || 'unknown')}"><i></i>${esc(statusLabel(candidate.source?.state || 'unknown'))}</span><label class="infra-field infra-bind-field"><span>${esc(t('desktop.infra_link_existing', 'Link to existing'))}</span><select data-bind="${esc(candidate.id)}"><option value="">${esc(t('desktop.infra_import_new', 'Import as new node'))}</option>${bindableNodes.map(existing => `<option value="${esc(existing.id)}" ${selectedNode === existing.id ? 'selected' : ''}>${esc(existing.name)}</option>`).join('')}</select></label></article>`;
            }).join('');
            dialog.querySelector('[data-candidate-sources]').innerHTML = state.sources.map(source => `<span class="infra-source-chip" data-state="${esc(source.state)}"><i></i>${esc(t(`desktop.infra_source_${source.kind}`, source.kind))} · ${esc(source.name || source.instance || source.kind)}</span>`).join('');
            dialog.querySelector('.infra-candidate-list').innerHTML = rows || `<div class="infra-empty-candidates"><span aria-hidden="true">⌁</span><strong>${esc(t('desktop.infra_no_candidates', 'No import candidates found'))}</strong><p>${esc(t('desktop.infra_no_candidates_hint', 'Configured sources may not have any new assets right now.'))}</p></div>`;
            dialog.querySelectorAll('[data-candidate]').forEach(box => box.addEventListener('change', () => { if (box.checked) state.importSelection.add(box.dataset.candidate); else state.importSelection.delete(box.dataset.candidate); updateImportCount(dialog); }));
            dialog.querySelectorAll('[data-bind]').forEach(select => select.addEventListener('change', () => state.importBindings.set(select.dataset.bind, select.value)));
            updateImportCount(dialog);
        }
        const updateImportCount = dialog => { const count = state.importSelection.size; const label = dialog.querySelector('[data-import-count]'); if (label) label.textContent = t('desktop.infra_selected_count', '{count} selected').replace('{count}', count); const button = dialog.querySelector('[data-import]'); if (button) button.disabled = !count || !canWrite() || state.busy.has('import'); };
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
                setNotice(t('desktop.infra_imported', '{count} assets imported').replace('{count}', ids.length), 'success');
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
            const dialog = openDialog(edge ? t('desktop.infra_edit_relation', 'Edit relation') : t('desktop.infra_add_relation', 'Add relation'), `<form data-edge-form><input type="hidden" name="id" value="${esc(edge?.id || '')}"><input type="hidden" name="version" value="${esc(edge?.version || 0)}"><label class="infra-field"><span>${esc(t('desktop.infra_relation_from', 'From'))}</span><select name="from">${options}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_kind', 'Relation type'))}</span><select name="kind">${EDGE_KINDS.map(kind => `<option value="${kind}">${esc(t(`desktop.infra_edge_${kind}`, kind))}</option>`).join('')}</select></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_to', 'To'))}</span><select name="to">${options}</select></label><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_relation_from_port', 'From port'))}</span><input name="from_port"></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_to_port', 'To port'))}</span><input name="to_port"></label></div><div class="infra-field-row"><label class="infra-field"><span>${esc(t('desktop.infra_relation_speed', 'Speed'))}</span><input name="speed"></label><label class="infra-field"><span>${esc(t('desktop.infra_relation_label', 'Label'))}</span><input name="label"></label></div><div class="infra-form-actions">${edge ? `<button type="button" data-delete-edge class="infra-danger-button">${esc(t('desktop.infra_remove_relation', 'Remove relation'))}</button>` : ''}<span class="infra-flex-spacer"></span><button type="button" data-close>${esc(t('desktop.cancel', 'Cancel'))}</button><button type="submit" class="infra-button-primary">${esc(t('desktop.save', 'Save changes'))}</button></div></form>`, dlg => {
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
                closeOwnedDialog(owner); setNotice(t('desktop.infra_relation_saved', 'Relation saved'), 'success'); paintAll();
            } catch (error) { setNotice(messageFor(error), 'error'); }
            finally { setBusy(busyKey, false); }
        }
        async function deleteEdge(edge, owner) {
            if (!edge || !canWrite()) return;
            const confirmation = await confirmAction(t('desktop.infra_remove_relation', 'Remove relation'), t('desktop.infra_remove_relation_confirm', 'Remove this relation from Infra?'), { withOwner: true });
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
                const dialog = openDialog(title, `<p class="infra-dialog-intro">${esc(message)}</p><div class="infra-form-actions"><button type="button" data-answer="no">${esc(t('desktop.cancel', 'Cancel'))}</button><button type="button" data-answer="yes" class="infra-button-primary">${esc(confirmLabel || t('desktop.confirm', 'Confirm'))}</button></div>`, dlg => {
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
            if (!await confirmAction(t('desktop.infra_remove_node', 'Remove from Infra'), t('desktop.infra_remove_node_confirm', 'Remove {name} from the Infra inventory? Its external source will not be changed.').replace('{name}', node.name))) return;
            try { await request(`/nodes/${encodeURIComponent(id)}?version=${encodeURIComponent(node.version)}`, { method: 'DELETE' }); state.nodes = state.nodes.filter(item => item.id !== id); state.edges = state.edges.filter(edge => edge.from !== id && edge.to !== id); state.drafts.delete(id); state.dirty.delete(id); state.selected = ''; setNotice(t('desktop.infra_node_removed', 'Node removed from Infra'), 'success'); paintAll(); }
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
            setNotice(t('desktop.infra_reapply_ready', 'Draft kept against the current version. Review it, then save when ready.'), 'info'); paintInspector();
        }
        function reapplyNodeDraft(id) {
            const review = state.nodeReviews.get(id), draft = state.drafts.get(id);
            if (!review || !draft || !canWrite()) return;
            draft.version = review.latest.version; state.drafts.set(id, draft); state.nodeReviews.delete(id); state.dirty.add(id);
            state.draftRevision.set(id, (state.draftRevision.get(id) || 0) + 1);
            setNotice(t('desktop.infra_reapply_ready', 'Draft kept against the current version. Review it, then save when ready.'), 'info'); paintInspector();
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
                    setNotice(t('desktop.infra_source_still_pending', 'The source change is still pending. Your draft remains here.'), 'info');
                    paintInspector(); return;
                }
                const confirmed = Object.entries(submitted).every(([key, value]) => JSON.stringify(latest.fields?.[key]) === JSON.stringify(value));
                if (confirmed) {
                    state.sourceDrafts.delete(id); state.sourceDirty.delete(id); state.sourceBlocked.delete(id); state.sourcePending.delete(id); state.sourceReviews.delete(id);
                    setNotice(t('desktop.infra_source_review_confirmed', 'The source now matches your draft. The change is confirmed.'), 'success');
                } else {
                    state.sourceDrafts.set(id, { ...draft, version: latest.version, fields: submitted }); state.sourceBlocked.add(id); state.sourcePending.delete(id); state.sourceReviews.set(id, { latest, submitted });
                    setNotice(t('desktop.infra_source_reviewed', 'Current source values are shown below. Choose reapply to keep your draft against this version.'), 'info');
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
                        setNotice(t('desktop.infra_source_pending', 'A source change is pending confirmation. Your draft is preserved.'), 'info');
                    } else setNotice(t('desktop.infra_source_outcome_unknown', 'The source did not confirm the change. Your draft is preserved; review the source before trying again.'), 'error');
                    paintInspector(); return;
                }
                if ((state.sourceRevision.get(id) || 0) === revision) {
                    state.sourceDrafts.delete(id); state.sourceDirty.delete(id); state.sourceBlocked.delete(id); state.sourcePending.delete(id); state.sourceReviews.delete(id);
                    await loadSnapshot(); setNotice(t('desktop.infra_source_saved', 'Source metadata saved'), 'success');
                } else {
                    const latest = await request(`/nodes/${encodeURIComponent(id)}/source-edit?source=${encodeURIComponent(draft.source)}`);
                    const newer = state.sourceDrafts.get(id);
                    if (newer) newer.version = latest.version;
                    state.sourceDirty.add(id); await loadSnapshot();
                    setNotice(t('desktop.infra_source_saved_newer_draft', 'Saved. Newer source edits remain unsaved.'), 'info');
                }
            }
            catch (error) {
                if (error.status === 409) { state.sourceBlocked.add(id); state.sourcePending.delete(id); state.sourceReviews.delete(id); paintInspector(); setNotice(messageFor(error), 'error'); }
                else if (uncertainMutation(error)) { state.sourceBlocked.add(id); state.sourcePending.delete(id); paintInspector(); setNotice(t('desktop.infra_source_outcome_unknown', 'The source did not confirm the change. Your draft is preserved; review the source before trying again.'), 'error'); }
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
            const dialog = openDialog(t('desktop.infra_source_actions', 'Source actions'), `<p class="infra-dialog-intro">${esc(t('desktop.infra_action_warning', 'This sends one explicit request to the selected source. Uncertain results are never retried automatically.'))}</p><div class="infra-action-list">${actions.map(action => { const needsReview = state.actionBlocks.has(actionBlockKey(id, sourceDescriptor, action)); return `<button type="button" data-run-action="${esc(action)}" data-action-node="${esc(id)}" data-source-identity="${esc(sourceIdentity(sourceDescriptor))}" class="${['stop','restart','shutdown','reboot'].includes(action) ? 'is-caution' : ''}" ${needsReview ? 'disabled' : ''}><span>${esc(t(`desktop.infra_action_${action}`, action))}</span><small>${esc(t(`desktop.infra_action_help_${action}`, ''))}</small><b aria-hidden="true">›</b></button>`; }).join('')}</div>${blocked.length ? `<section class="infra-action-review" role="status"><strong>${esc(t('desktop.infra_action_review_required', 'Check the device state before another attempt.'))}</strong>${blocked.map(action => `<div><span>${esc(t(`desktop.infra_action_${action}`, action))} · ${esc(state.actionBlocks.get(actionBlockKey(id, sourceDescriptor, action))?.status || 'unknown')}</span><button type="button" data-action-rearm="${esc(action)}">${esc(t('desktop.infra_action_rearm', 'I checked the device state · allow one new attempt'))}</button></div>`).join('')}</section>` : ''}`, dlg => {
                dlg.querySelectorAll('[data-run-action]').forEach(button => button.addEventListener('click', () => void runAction(id, sourceDescriptor, button.dataset.runAction, dlg)));
                dlg.querySelectorAll('[data-action-rearm]').forEach(button => button.addEventListener('click', () => {
                    state.actionBlocks.delete(actionBlockKey(id, sourceDescriptor, button.dataset.actionRearm));
                    setNotice(t('desktop.infra_action_rearmed', 'A new attempt is enabled. Choose the action when ready.'), 'info');
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
                    const confirmation = await confirmAction(t(`desktop.infra_action_${action}`, action), t('desktop.infra_confirm_action', 'Confirm {action} for {name}?').replace('{action}', t(`desktop.infra_action_${action}`, action)).replace('{name}', nodeById(id)?.name || ''), { withOwner: true });
                    if (!confirmation.confirmed) return;
                    dialogOwner = confirmation.owner || dialogOwner;
                    reviewAllowed = true;
                }
                if (state.disposed || !root.isConnected) return;
                const result = await request(`/nodes/${encodeURIComponent(id)}/actions`, { method: 'POST', body: { source: sourceDescriptor.kind, action, request_id, confirmed: true } });
                if (state.disposed || !root.isConnected) return;
                if (['running', 'accepted', 'unknown'].includes(result.status)) {
                    state.actionBlocks.set(blockKey, { status: result.status, request_id });
                    setNotice(result.status === 'unknown' ? t('desktop.infra_action_outcome_unknown', 'The action result is unknown. Check the device state before trying again.') : t('desktop.infra_action_review_required', 'Check the device state before another attempt.'), result.status === 'unknown' ? 'error' : 'info');
                    if (ownsDialog(dialogOwner) && (!dialog || dialog.open || reviewAllowed)) openActionMenu(id, sourceDescriptor); return;
                }
                setNotice(t(`desktop.infra_action_status_${result.status}`, t('desktop.infra_action_status_generic', 'Action status: {status}').replace('{status}', result.status)).replace('{status}', result.status), result.status === 'failed' || result.status === 'unknown' ? 'error' : 'success');
                closeOwnedDialog(dialogOwner);
            } catch (error) {
                if (state.disposed || !root.isConnected) return;
                if (uncertainMutation(error)) {
                    state.actionBlocks.set(blockKey, { status: 'unknown', request_id });
                    setNotice(t('desktop.infra_action_outcome_unknown', 'The action result is unknown. Check the device state before trying again.'), 'error');
                    if (ownsDialog(dialogOwner) && (!dialog || dialog.open || reviewAllowed)) openActionMenu(id, sourceDescriptor);
                }
                else setNotice(messageFor(error), 'error');
            }
            finally { setBusy(busyKey, false); }
        }
        function openNodeActions(id) {
            const node = nodeById(id); if (!node) return;
            const content = `<div class="infra-action-list">${canWrite() ? `<button type="button" data-act="edge"><span>${esc(t('desktop.infra_add_relation', 'Add relation'))}</span><b>›</b></button><button type="button" data-act="remove" class="is-caution"><span>${esc(t('desktop.infra_remove_node', 'Remove from Infra'))}</span><b>›</b></button>` : ''}${canControl() ? (node.sources || []).flatMap(source => sourceActions(source.kind).filter(action => allowedAction(node, action, source)).map(action => `<button type="button" data-act-source="${esc(sourceIdentity(source))}" data-act="${esc(action)}"><span>${esc(t(`desktop.infra_action_${action}`, action))} · ${esc(t(`desktop.infra_source_${source.kind}`, source.kind))}</span><b>›</b></button>`)).join('') : ''}</div>`;
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
            if (!state.edgeFrom) { state.edgeFrom = id; setNotice(t('desktop.infra_connect_choose_second', 'Now choose the other asset.'), 'info'); paintStage(); return; }
            const from = state.edgeFrom; state.edgeFrom = ''; state.edgeMode = false;
            q('[data-action="edge-mode"]').setAttribute('aria-pressed', 'false');
            if (from === id) { setNotice(t('desktop.infra_edge_self_error', 'Choose a different second asset.'), 'error'); return; }
            openEdgeDialog('', from); q('[data-dialog-content] select[name="to"]').value = id;
        }
        function openImport() {
            if (!canWrite()) return;
            state.importOpen = true;
            state.candidates = []; state.importSelection.clear(); state.importBindings.clear();
            const dialog = openDialog(t('desktop.infra_import_title', 'Review observed assets'), `<p class="infra-dialog-intro">${esc(t('desktop.infra_import_hint', 'Review each candidate, then choose whether to create a node or explicitly link it to one you already manage.'))}</p><div class="infra-candidate-source-tabs" data-candidate-sources></div><div class="infra-candidate-list"><div class="infra-import-loading">${esc(t('desktop.infra_loading_candidates', 'Checking configured sources…'))}</div></div><div class="infra-form-actions infra-dialog-actions"><span data-import-count></span><button type="button" data-close>${esc(t('desktop.cancel', 'Cancel'))}</button><button type="button" data-import class="infra-button-primary" disabled>${esc(t('desktop.infra_import_selected', 'Import selected'))}</button></div>`, dlg => {
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
                else if (action === 'edge-mode') { state.edgeMode = !state.edgeMode; state.edgeFrom = ''; event.target.closest('button').setAttribute('aria-pressed', String(state.edgeMode)); setNotice(t(state.edgeMode ? 'desktop.infra_connect_choose_first' : 'desktop.infra_ready', state.edgeMode ? 'Choose the first asset to connect.' : 'Inventory ready'), 'info'); }
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
            const dialog = openDialog(t('desktop.infra_actions', 'Infrastructure actions'), `<div class="infra-action-list"><button type="button" data-quick="import"><span>${esc(t('desktop.infra_import_title', 'Review observed assets'))}</span><small>${esc(t('desktop.infra_import_hint_short', 'Choose what to add from configured sources'))}</small><b>›</b></button><button type="button" data-quick="new"><span>${esc(t('desktop.infra_new_node', 'Add infrastructure node'))}</span><small>${esc(t('desktop.infra_manual_hint', 'Create a manually managed asset'))}</small><b>›</b></button></div>`, dlg => {
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
                setNotice(t('desktop.infra_readonly_revoked', 'Write access changed. Unsaved drafts remain here.'), 'info');
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
                t('desktop.app_infra', 'Infra'),
                t('desktop.infra_unsaved_close', 'Close Infra and discard your unsaved changes?'),
                { confirmLabel: t('desktop.infra_discard', 'Discard') }
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
