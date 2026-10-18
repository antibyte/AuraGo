// Pure Infra map helpers. Loaded before infra-views.js and infra.js.
(function () {
    'use strict';

    const ICON_LIST = 'rack-server tower-server mini-pc sbc nas disk-shelf workstation laptop tablet smartphone router firewall modem-ont managed-switch switch access-point mesh-node ups pdu patch-panel camera nvr printer 3d-printer smart-hub sensor smart-plug tv console radio-gateway usb-gateway generic-device vm container hypervisor kubernetes database web-service reverse-proxy dns-dhcp vpn storage-share backup monitoring home-automation media-server ai-runtime mqtt logs git-ci mail-server photo-library password-manager cloud-service automation generic-app rack site room lan vlan wlan internet group'.split(' ');
    const ICONS = new Set(ICON_LIST);
    const ICON_ASSETS = new Map();
    const CONTAINER_KINDS = ['site', 'room', 'rack', 'group'];
    const POWERS = new Set(['running', 'stopped', 'paused', 'unknown']);
    const TOPOLOGY_LAYER = { network: 0, site: 0, room: 0, rack: 0, device: 1, group: 1, vm: 2, container: 2, service: 3 };

    const esc = value => String(value ?? '').replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch]);
    const clone = value => JSON.parse(JSON.stringify(value));
    const iconId = value => ICONS.has(value) ? value : 'generic-device';
    const acceptsIcon = id => ICONS.has(id) || id === 'icon' || id === 'empty-atlas';
    const rememberIcon = (id, path, labelKey) => { ICON_ASSETS.set(id, { path, label_key: labelKey || '' }); };
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
    const isContainer = kind => CONTAINER_KINDS.includes(kind);
    const powerLabel = power => POWERS.has(power) ? `desktop.infra_power_${power}` : '';
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
    const containerFrame = (container, children, positionOf) => {
        const origin = positionOf(container);
        if (!origin || !Number.isFinite(Number(origin.x)) || !Number.isFinite(Number(origin.y))) return null;
        const placed = (children || []).map(positionOf).filter(position => position && Number.isFinite(Number(position.x)) && Number.isFinite(Number(position.y)));
        if (!placed.length) return null;
        const points = [origin, ...placed];
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        for (const point of points) {
            minX = Math.min(minX, Number(point.x)); minY = Math.min(minY, Number(point.y));
            maxX = Math.max(maxX, Number(point.x)); maxY = Math.max(maxY, Number(point.y));
        }
        const pad = 48;
        return { id: container.id, x: minX - pad, y: minY - pad, width: (maxX - minX) + pad * 2, height: (maxY - minY) + pad * 2 };
    };
    const edgeSegment = (a, b) => {
        const x1 = Number(a.x), y1 = Number(a.y), x2 = Number(b.x), y2 = Number(b.y);
        const dx = x2 - x1, dy = y2 - y1, length = Math.hypot(dx, dy);
        if (!(length > 160)) return { x1, y1, x2, y2 };
        const ux = dx / length, uy = dy / length;
        return { x1: x1 + ux * 78, y1: y1 + uy * 78, x2: x2 - ux * 78, y2: y2 - uy * 78 };
    };
    const edgeCaption = edge => {
        const from = String(edge?.from_port || '').trim(), to = String(edge?.to_port || '').trim(), speed = String(edge?.speed || '').trim();
        const ports = from && to ? `${from} → ${to}` : (from || to);
        return [ports, speed].filter(Boolean).join(' · ');
    };
    const overlaps = (left, right) => Math.abs(Number(left.x) - Number(right.x)) < 160 && Math.abs(Number(left.y) - Number(right.y)) < 110;
    // Grid cells make the placement search linear: generated positions are exact
    // grid points, so a Set replaces the per-attempt array scan.
    const gridKey = (x, y) => `${Math.round((Number(x) - 110) / 220)},${Math.round((Number(y) - 100) / 155)}`;
    const blockedGrid = obstacles => {
        const blocked = new Set();
        for (const pos of obstacles || []) blocked.add(gridKey(pos.x, pos.y));
        return blocked;
    };
    const gridPositions = (nodes, obstacles, count) => {
        const changes = new Map();
        const columns = Math.max(3, Math.ceil(Math.sqrt(Math.max(1, count) * 1.45)));
        const occupied = obstacles || [];
        const blocked = blockedGrid(occupied);
        let index = 0;
        for (const node of nodes) {
            let position;
            for (let attempts = 0; attempts < 20000 && !position; attempts += 1, index += 1) {
                const candidate = { x: 110 + (index % columns) * 220, y: 100 + Math.floor(index / columns) * 155 };
                const key = gridKey(candidate.x, candidate.y);
                if (blocked.has(key) || occupied.some(pos => overlaps(pos, candidate))) continue;
                position = { ...candidate, pinned: false };
                blocked.add(key);
            }
            if (position) changes.set(node.id, position);
        }
        return changes;
    };
    const topologyPositions = (nodes, obstacles) => {
        const layers = [[], [], [], []];
        for (const node of nodes) {
            const layer = TOPOLOGY_LAYER[node.kind];
            if (layer == null) continue;
            layers[layer].push(node);
        }
        const changes = new Map();
        const occupied = obstacles || [];
        const blocked = blockedGrid(occupied);
        layers.forEach((layerNodes, layer) => {
            const ordered = [...layerNodes].sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')));
            let column = 0;
            for (const node of ordered) {
                let position;
                for (let guard = 0; guard < 20000 && !position; guard += 1) {
                    const candidate = { x: 110 + column * 220, y: 100 + layer * 180 };
                    column += 1;
                    const key = gridKey(candidate.x, candidate.y);
                    if (blocked.has(key) || occupied.some(pos => overlaps(pos, candidate))) continue;
                    position = { ...candidate, pinned: false };
                    blocked.add(key);
                }
                if (position) changes.set(node.id, position);
            }
        });
        return changes;
    };
    const sourceStamp = source => [source?.kind || '', source?.instance || '', source?.key || '', source?.state || '', source?.observed?.power || ''].join('\u001f');
    // The layout endpoint answers with the nodes it changed. Only an explicit
    // snapshot replaces the atlas; a bare node list is a patch.
    const layoutSnapshot = result => (result && result.snapshot && Array.isArray(result.snapshot.nodes)) ? result.snapshot : null;
    const applyLayoutPatch = (nodes, patches, view) => {
        const byId = new Map((nodes || []).map(node => [node.id, node]));
        for (const row of patches || []) {
            const node = byId.get(row?.id);
            if (!node) continue;
            node.version = row.version ?? node.version;
            node.placements = node.placements || {};
            node.placements[view] = row.placements?.[view] || { x: row.x, y: row.y, pinned: !!row.pinned };
        }
        return nodes;
    };
    const snapshotStamp = (nodes, edges, sources, caps) => {
        const nodePart = (nodes || []).map(node => [node.id, node.version, node.kind, node.name, node.parent_id || '', node.power || '', JSON.stringify(node.placements || {}), (node.sources || []).map(sourceStamp).join('\u001f')].join('\u001f')).join('\u001e');
        const edgePart = (edges || []).map(edge => [edge.id, edge.version, edge.from, edge.to, edge.kind].join('\u001f')).join('\u001e');
        const sourcePart = (sources || []).map(sourceStamp).join('\u001e');
        const capPart = [!!caps?.write, !!caps?.source_edit, !!caps?.control].join('\u001f');
        return [nodePart, edgePart, sourcePart, capPart].join('\u001d');
    };

    window.InfraAppModel = {
        esc, clone, iconId, iconURL, assetURL, iconLabelKey, sourceActions, sourceIdentity, virtualSlice, sourceState,
        visibleByCollapsed, isPlaced, atlasDimensions, sceneDimensions, validCIDR, validHTTPLink, iconIds: () => [...ICON_LIST],
        acceptsIcon, rememberIcon, versionedAsset, containerKinds: () => [...CONTAINER_KINDS], isContainer, powerLabel,
        containerFrame, edgeSegment, edgeCaption, gridPositions, topologyPositions, snapshotStamp,
        layoutSnapshot, applyLayoutPatch,
    };
})();
