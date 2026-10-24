// Pure Infra helpers. Loaded first; every other infra-*.js file reads window.InfraAppModel.
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
    // edgeSegment joins two shapes at their borders: each end leaves its centre
    // towards the other and stops at its half-size rectangle. Overlapping shapes
    // keep the centre-to-centre line.
    const borderPoint = (from, to, half) => {
        const dx = to.x - from.x, dy = to.y - from.y;
        if (!dx && !dy) return { x: from.x, y: from.y };
        const scale = Math.min(dx ? half.w / Math.abs(dx) : Infinity, dy ? half.h / Math.abs(dy) : Infinity);
        return { x: from.x + dx * scale, y: from.y + dy * scale };
    };
    const edgeSegment = (a, b, halfA = { w: 92, h: 30 }, halfB = { w: 92, h: 30 }) => {
        const x1 = Number(a.x), y1 = Number(a.y), x2 = Number(b.x), y2 = Number(b.y);
        const start = borderPoint({ x: x1, y: y1 }, { x: x2, y: y2 }, halfA);
        const end = borderPoint({ x: x2, y: y2 }, { x: x1, y: y1 }, halfB);
        if ((end.x - start.x) * (x2 - x1) + (end.y - start.y) * (y2 - y1) <= 0) return { x1, y1, x2, y2 };
        return { x1: start.x, y1: start.y, x2: end.x, y2: end.y };
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
            if (row.placements && typeof row.placements === 'object') {
                // A full node states its placements: a view it lacks was unplaced.
                if (row.placements[view]) node.placements[view] = row.placements[view]; else delete node.placements[view];
                // The server omits an empty parent_id, so absence means "no parent".
                node.parent_id = row.parent_id || '';
            } else node.placements[view] = { x: row.x, y: row.y, pinned: !!row.pinned };
        }
        return nodes;
    };
    const snapshotStamp = (nodes, edges, sources, caps) => {
        const nodePart = (nodes || []).map(node => [node.id, node.version, node.kind, node.name, node.parent_id || '', node.host_id || '', node.power || '', JSON.stringify(node.placements || {}), (node.sources || []).map(sourceStamp).join('\u001f')].join('\u001f')).join('\u001e');
        const edgePart = (edges || []).map(edge => [edge.id, edge.version, edge.from, edge.to, edge.kind].join('\u001f')).join('\u001e');
        const sourcePart = (sources || []).map(sourceStamp).join('\u001e');
        const capPart = [!!caps?.write, !!caps?.source_edit, !!caps?.control].join('\u001f');
        return [nodePart, edgePart, sourcePart, capPart].join('\u001d');
    };

    // ---- canvas world -------------------------------------------------------
    // World coordinates are CSS pixels at zoom 1. An asset placement (x, y) is
    // the centre of its 184×60 card; a place placement (x, y) is the top-left
    // corner of its region and (w, h) its size (default 360×240).
    const CARD_W = 184, CARD_H = 60, GRID = 24, MIN_ZOOM = 0.25, MAX_ZOOM = 2;
    const REGION_W = 360, REGION_H = 240, REGION_PAD = 48, REGION_MIN_W = 200, REGION_MIN_H = 140;
    const CARD_HALF = { w: CARD_W / 2, h: CARD_H / 2 };
    const HOSTED_KINDS = new Set(['vm', 'container', 'service']);
    const WIRELESS_ICONS = new Set(['access-point', 'mesh-node']);
    const finite = value => (value === null || value === undefined || value === '' || !Number.isFinite(Number(value)) ? null : Number(value));
    const clampZoom = zoom => (typeof zoom === 'number' && Number.isFinite(zoom) ? Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom)) : 1);
    const toWorld = (view, p) => ({ x: (p.x - view.x) / view.zoom, y: (p.y - view.y) / view.zoom });
    const toScreen = (view, p) => ({ x: p.x * view.zoom + view.x, y: p.y * view.zoom + view.y });
    // zoomAt changes the zoom and keeps the world point under screen fixed.
    const zoomAt = (view, zoom, screen) => {
        const z = clampZoom(zoom);
        const wx = (screen.x - view.x) / view.zoom, wy = (screen.y - view.y) / view.zoom;
        return { x: screen.x - wx * z, y: screen.y - wy * z, zoom: z };
    };
    // fitView shows box inside size with padding, never above maxZoom.
    const fitView = (box, size, padding = 48, maxZoom = 1) => {
        if (!box) return { x: size.w / 2, y: size.h / 2, zoom: 1 };
        const zoom = clampZoom(Math.min((size.w - padding * 2) / Math.max(box.w, 1), (size.h - padding * 2) / Math.max(box.h, 1), maxZoom));
        return { x: (size.w - box.w * zoom) / 2 - box.x * zoom, y: (size.h - box.h * zoom) / 2 - box.y * zoom, zoom };
    };
    // A stored viewport is its world centre and zoom, so it suits any window size.
    const viewCenter = (view, size) => ({ cx: (size.w / 2 - view.x) / view.zoom, cy: (size.h / 2 - view.y) / view.zoom, zoom: view.zoom });
    const viewFromCenter = (stored, size) => {
        if (!stored || typeof stored !== 'object' || !Number.isFinite(stored.cx) || !Number.isFinite(stored.cy)) return null;
        const zoom = clampZoom(stored.zoom);
        return { x: size.w / 2 - stored.cx * zoom, y: size.h / 2 - stored.cy * zoom, zoom };
    };
    const snap = (value, grid = GRID) => Math.round(value / grid) * grid;
    const cardRect = pos => ({ x: Number(pos.x) - CARD_W / 2, y: Number(pos.y) - CARD_H / 2, w: CARD_W, h: CARD_H });
    const intersects = (a, b) => a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
    const containsPoint = (r, p) => p.x >= r.x && p.x <= r.x + r.w && p.y >= r.y && p.y <= r.y + r.h;
    const normalizeRect = (a, b) => ({ x: Math.min(a.x, b.x), y: Math.min(a.y, b.y), w: Math.abs(b.x - a.x), h: Math.abs(b.y - a.y) });
    const padRect = (r, pad) => ({ x: r.x - pad, y: r.y - pad, w: r.w + pad * 2, h: r.h + pad * 2 });
    const unionRects = rects => {
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        for (const r of rects || []) {
            if (!r) continue;
            minX = Math.min(minX, r.x); minY = Math.min(minY, r.y);
            maxX = Math.max(maxX, r.x + r.w); maxY = Math.max(maxY, r.y + r.h);
        }
        return minX === Infinity ? null : { x: minX, y: minY, w: maxX - minX, h: maxY - minY };
    };
    // freeSlot returns want when the card fits there, else the grid point of the
    // nearest ring around it whose card (plus gap) overlaps no obstacle rect.
    const freeSlot = (want, obstacles, { grid = GRID, gap = 12, maxRing = 40 } = {}) => {
        const reach = (maxRing + 2) * grid + CARD_W;
        const near = (obstacles || []).filter(o => Math.abs(o.x + o.w / 2 - want.x) < reach + o.w / 2 && Math.abs(o.y + o.h / 2 - want.y) < reach + o.h / 2).map(o => padRect(o, gap / 2));
        const blocked = point => { const rect = padRect(cardRect(point), gap / 2); return near.some(o => intersects(rect, o)); };
        if (!blocked(want)) return { x: want.x, y: want.y };
        const base = { x: snap(want.x, grid), y: snap(want.y, grid) };
        for (let ring = 0; ring <= maxRing; ring += 1) {
            let best = null, bestDistance = Infinity;
            for (let dy = -ring; dy <= ring; dy += 1) for (let dx = -ring; dx <= ring; dx += 1) {
                if (Math.max(Math.abs(dx), Math.abs(dy)) !== ring) continue;
                const point = { x: base.x + dx * grid, y: base.y + dy * grid };
                if (blocked(point)) continue;
                const distance = Math.hypot(point.x - want.x, point.y - want.y);
                if (distance < bestDistance) { best = point; bestDistance = distance; }
            }
            if (best) return best;
        }
        return { x: want.x, y: want.y };
    };
    // regionRect is the drawn rectangle of a place: its stored rectangle grown to
    // contain childRects plus the 48 px frame padding.
    const regionRect = (placement, childRects = []) => {
        if (!placement || finite(placement.x) == null || finite(placement.y) == null) return null;
        const base = { x: Number(placement.x), y: Number(placement.y), w: finite(placement.w) || REGION_W, h: finite(placement.h) || REGION_H };
        return unionRects([base, ...childRects.filter(Boolean).map(r => padRect(r, REGION_PAD))]);
    };
    // regionLayout draws placed places bottom-up so nested places grow their parents.
    const regionLayout = (placeIds, { parentOf, placementOf, cardRectsByParent = new Map() }) => {
        const ids = new Set(placeIds), rects = new Map(), depth = new Map(), visiting = new Set(), nested = new Map();
        for (const id of ids) {
            const parent = parentOf(id);
            if (parent !== id && ids.has(parent)) { const list = nested.get(parent) || []; list.push(id); nested.set(parent, list); }
        }
        const depthOf = id => {
            let level = 0;
            const seen = new Set([id]);
            for (let parent = parentOf(id); ids.has(parent) && !seen.has(parent); parent = parentOf(parent)) { seen.add(parent); level += 1; }
            return level;
        };
        const visit = id => {
            if (rects.has(id)) return rects.get(id);
            if (visiting.has(id)) return null;
            visiting.add(id);
            const inner = (nested.get(id) || []).map(visit).filter(Boolean);
            const rect = regionRect(placementOf(id), [...(cardRectsByParent.get(id) || []), ...inner]);
            visiting.delete(id);
            if (rect) { rects.set(id, rect); depth.set(id, depthOf(id)); }
            return rect;
        };
        for (const id of ids) visit(id);
        return { rects, depth };
    };
    // regionAt returns the innermost region containing point: deepest first,
    // then the smallest area. exclude holds ids that cannot be targets.
    const regionAt = (point, rects, depth, exclude = new Set()) => {
        let best = '', bestDepth = -1, bestArea = Infinity;
        for (const [id, r] of rects) {
            if (exclude.has(id) || !containsPoint(r, point)) continue;
            const level = depth.get(id) || 0, area = r.w * r.h;
            if (level > bestDepth || (level === bestDepth && area < bestArea)) { best = id; bestDepth = level; bestArea = area; }
        }
        return best;
    };
    // dropParent says what a drop does to parent_id: {} when it stays, else
    // {parent_id}. A parent that is not drawn in this view is never cleared.
    const dropParent = (current, point, rects, depth, exclude = new Set()) => {
        const target = regionAt(point, rects, depth, exclude);
        const parent = current || '';
        if (target === parent) return {};
        if (!target && parent && !rects.has(parent)) return {};
        return { parent_id: target };
    };
    // growRegions returns stored rectangles that must grow so region id (and its
    // ancestors) contain rect plus REGION_PAD.
    const growRegions = (id, rect, { ids, parentOf, placementOf }) => {
        const changes = new Map(), seen = new Set();
        let target = rect, current = id;
        while (current && ids.has(current) && !seen.has(current)) {
            seen.add(current);
            const p = placementOf(current);
            if (!p) break;
            const base = { x: Number(p.x), y: Number(p.y), w: finite(p.w) || REGION_W, h: finite(p.h) || REGION_H };
            const grown = unionRects([base, padRect(target, REGION_PAD)]);
            if (grown.x === base.x && grown.y === base.y && grown.w === base.w && grown.h === base.h) break;
            changes.set(current, grown);
            target = grown;
            current = parentOf(current);
        }
        return changes;
    };
    // spatialNeighbour picks the nearest item in direction dir (left/right/up/down);
    // the off-axis distance counts twice.
    const spatialNeighbour = (from, items, dir) => {
        const axis = dir === 'left' || dir === 'right' ? 'x' : 'y', other = axis === 'x' ? 'y' : 'x';
        const sign = dir === 'right' || dir === 'down' ? 1 : -1;
        let best = '', bestScore = Infinity;
        for (const item of items || []) {
            if (item.id === from.id) continue;
            const primary = (item[axis] - from[axis]) * sign;
            if (primary <= 0) continue;
            const score = primary + Math.abs(item[other] - from[other]) * 2;
            if (score < bestScore || (score === bestScore && String(item.id) < String(best))) { best = item.id; bestScore = score; }
        }
        return best;
    };
    // compactGrid lays count cards out around centre in rows of ceil(sqrt(count)),
    // 216×96 apart (both multiples of the 24 px grid).
    const compactGrid = (count, centre) => {
        const cols = Math.max(1, Math.ceil(Math.sqrt(count))), rows = Math.ceil(count / cols);
        const cellW = CARD_W + 32, cellH = CARD_H + 36;
        const x0 = snap(centre.x - (cols - 1) * cellW / 2), y0 = snap(centre.y - (rows - 1) * cellH / 2);
        return Array.from({ length: count }, (_, i) => ({ x: x0 + (i % cols) * cellW, y: y0 + Math.floor(i / cols) * cellH }));
    };
    const defaultEdgeKind = (a, b) => {
        if ([a, b].some(node => WIRELESS_ICONS.has(node?.icon))) return 'wifi';
        if ([a, b].some(node => node?.kind === 'service')) return 'depends_on';
        return 'ethernet';
    };
    // guestIndex: a vm, container or service belongs to the node named by
    // host_id (server projection, Plan A) or else to the first target of its
    // runs_on edges. Mutual hosting is broken in favour of the smaller id.
    const guestIndex = (nodes, edges) => {
        const byId = new Map((nodes || []).map(node => [node.id, node]));
        const runsOn = new Map();
        for (const edge of edges || []) {
            if (edge.kind !== 'runs_on' || edge.from === edge.to || !byId.has(edge.to)) continue;
            const list = runsOn.get(edge.from) || []; list.push(edge.to); runsOn.set(edge.from, list);
        }
        const hostOf = new Map(), guestsOf = new Map();
        for (const node of nodes || []) {
            if (!HOSTED_KINDS.has(node.kind)) continue;
            const declared = typeof node.host_id === 'string' && node.host_id !== node.id && byId.has(node.host_id) ? node.host_id : '';
            const host = declared || (runsOn.get(node.id) || []).slice().sort()[0] || '';
            if (host) hostOf.set(node.id, host);
        }
        for (const [guest, host] of [...hostOf]) if (hostOf.get(host) === guest && guest > host) hostOf.delete(guest);
        for (const [guest, host] of hostOf) { const list = guestsOf.get(host) || []; list.push(guest); guestsOf.set(host, list); }
        return { hostOf, guestsOf };
    };
    // matchesFilters is the single filter predicate (search text, type, source, power).
    const matchesFilters = (node, filters = {}, textOf = () => '') => {
        if (filters.kind && node.kind !== filters.kind) return false;
        if (filters.source && !(node.sources || []).some(source => source.kind === filters.source)) return false;
        if (filters.power === 'none' && node.power) return false;
        if (filters.power && filters.power !== 'none' && node.power !== filters.power) return false;
        const query = String(filters.query || '').trim().toLocaleLowerCase();
        return !query || textOf(node).includes(query);
    };
    const filtersActive = filters => !!(filters && (filters.kind || filters.source || filters.power || String(filters.query || '').trim()));
    // buildScene computes what the canvas draws for one view: asset cards (a
    // guest rides in its host's card instead), regions for placed places (Map
    // view only) and connection lines between shape borders. runs_on lines that
    // repeat the nesting are left out; a line ending at a guest attaches to the host.
    const buildScene = ({ nodes, edges, view, placementOf, parentOf, guests }) => {
        const atlas = view === 'atlas';
        const cards = [], placeIds = [];
        for (const node of nodes || []) {
            if (guests.hostOf.has(node.id)) continue;
            const p = placementOf(node.id);
            if (!p || finite(p.x) == null || finite(p.y) == null) continue;
            if (isContainer(node.kind)) { if (atlas) placeIds.push(node.id); continue; }
            cards.push({ id: node.id, x: Number(p.x), y: Number(p.y), guests: guests.guestsOf.get(node.id) || [] });
        }
        const cardById = new Map(cards.map(card => [card.id, card]));
        const cardRectsByParent = new Map();
        if (atlas) for (const card of cards) {
            const parent = parentOf(card.id);
            if (!parent) continue;
            const list = cardRectsByParent.get(parent) || []; list.push(cardRect(card)); cardRectsByParent.set(parent, list);
        }
        const { rects, depth } = atlas ? regionLayout(placeIds, { parentOf, placementOf, cardRectsByParent }) : { rects: new Map(), depth: new Map() };
        const endpoint = id => (cardById.has(id) || rects.has(id) ? id : (cardById.has(guests.hostOf.get(id)) ? guests.hostOf.get(id) : ''));
        const shape = id => {
            const card = cardById.get(id);
            if (card) return { x: card.x, y: card.y, half: CARD_HALF };
            const r = rects.get(id);
            return { x: r.x + r.w / 2, y: r.y + r.h / 2, half: { w: r.w / 2, h: r.h / 2 } };
        };
        const lines = [];
        for (const edge of edges || []) {
            if (edge.kind === 'runs_on' && guests.hostOf.get(edge.from) === edge.to) continue;
            const from = endpoint(edge.from), to = endpoint(edge.to);
            if (!from || !to || from === to) continue;
            const a = shape(from), b = shape(to);
            lines.push({ id: edge.id, kind: edge.kind, from, to, ...edgeSegment(a, b, a.half, b.half) });
        }
        const regions = [...rects].map(([id, rect]) => ({ id, rect, depth: depth.get(id) || 0 })).sort((a, b) => a.depth - b.depth || String(a.id).localeCompare(String(b.id)));
        return { cards, cardById, regions, regionRects: rects, regionDepth: depth, lines, bounds: unionRects([...cards.map(cardRect), ...rects.values()]) };
    };
    // ---- layout intents and history -----------------------------------------
    // mergeIntent lays a later intent over an earlier one. An unplace intent
    // ({unplace: true, parent_id?}) replaces everything before it and keeps only
    // its own parent_id; a later placement lifts it.
    const mergeIntent = (earlier, later) => {
        if (later && later.unplace) return later.parent_id === undefined ? { unplace: true } : { unplace: true, parent_id: later.parent_id };
        const merged = { ...(earlier || {}), ...(later || {}) };
        delete merged.unplace;
        return merged;
    };
    // layoutItems builds PUT /layout items with the node versions of nodesById.
    // parent_id, w/h and unplace go out only when the server supports them (parent).
    const layoutItems = (intents, nodesById, { parent = false, view = 'atlas' } = {}) => {
        const items = [], missing = [];
        for (const [id, intent] of intents) {
            const node = nodesById.get(id);
            if (!node) { missing.push(id); continue; }
            if (intent.unplace) {
                if (parent) items.push(intent.parent_id === undefined ? { id, version: node.version, unplace: true } : { id, version: node.version, unplace: true, parent_id: String(intent.parent_id) });
                continue;
            }
            const item = { id, version: node.version, x: Math.round(Number(intent.x)), y: Math.round(Number(intent.y)), pinned: intent.pinned !== false };
            if (parent && intent.parent_id !== undefined) item.parent_id = String(intent.parent_id);
            if (parent && isContainer(node.kind)) {
                const stored = node.placements?.[view] || {};
                const w = finite(intent.w) ?? finite(stored.w), h = finite(intent.h) ?? finite(stored.h);
                if (w != null && h != null) { item.w = Math.round(w); item.h = Math.round(h); }
            }
            items.push(item);
        }
        return { items, missing };
    };
    // rebaseLayout keeps a conflicted batch for nodes that still exist; a parent
    // that vanished or is no longer a place is dropped from the intent.
    const rebaseLayout = (intents, nodesById) => {
        const kept = new Map(), dropped = [];
        for (const [id, intent] of intents) {
            if (!nodesById.has(id)) { dropped.push(id); continue; }
            const next = { ...intent };
            if (next.parent_id) { const parent = nodesById.get(next.parent_id); if (!parent || !isContainer(parent.kind)) delete next.parent_id; }
            kept.set(id, next);
        }
        return { kept, dropped };
    };
    const createHistory = (limit = 50) => {
        let undo = [], redo = [];
        const cap = list => (list.length > limit ? list.slice(list.length - limit) : list);
        return {
            push(entry) { undo = cap([...undo, entry]); redo = []; },
            pushUndo(entry) { undo = cap([...undo, entry]); },
            pushRedo(entry) { redo = cap([...redo, entry]); },
            takeUndo: () => undo.pop() ?? null,
            takeRedo: () => redo.pop() ?? null,
            canUndo: () => undo.length > 0,
            canRedo: () => redo.length > 0,
            clear() { undo = []; redo = []; },
        };
    };
    // ---- small utilities -----------------------------------------------------
    const clampNumber = (value, min, max, fallback) => { const n = Number(value); return value !== null && value !== '' && Number.isFinite(n) ? Math.min(max, Math.max(min, n)) : fallback; };
    const readLayoutPrefs = raw => {
        let data = {};
        try { data = raw ? JSON.parse(raw) : {}; } catch (_) { data = {}; }
        if (!data || typeof data !== 'object') data = {};
        return { libraryWidth: clampNumber(data.libraryWidth, 220, 400, 264), libraryCollapsed: data.libraryCollapsed === true, inspectorWidth: clampNumber(data.inspectorWidth, 320, 520, 360), minimap: data.minimap !== false };
    };
    const fill = (text, vars = {}) => Object.entries(vars).reduce((out, [key, value]) => out.replaceAll(`{${key}}`, String(value)), String(text));
    // normalizeNode fills UI defaults. It never adds fields the snapshot did not
    // send: node PUT bodies are clones of these objects and the server rejects
    // unknown fields.
    const normalizeNode = node => ({ ...node, icon: iconId(node.icon), parent_id: node.parent_id || '', profile: { tags: [], interfaces: [], links: [], app_bindings: [], custom: {}, ...(node.profile || {}) }, placements: node.placements || {}, sources: node.sources || [], app_links: node.app_links || [], available_actions: node.available_actions || [] });
    const minimapTransform = (bounds, size, pad = 6) => {
        if (!bounds) return { scale: 1, x: 0, y: 0 };
        const scale = Math.min((size.w - pad * 2) / Math.max(bounds.w, 1), (size.h - pad * 2) / Math.max(bounds.h, 1));
        return { scale, x: (size.w - bounds.w * scale) / 2 - bounds.x * scale, y: (size.h - bounds.h * scale) / 2 - bounds.y * scale };
    };
    const GLYPHS = {
        select: '<path d="M3.5 2.5l9 5.2-4.1 1.1-1.9 3.9z" fill="currentColor" stroke="none"/>',
        hand: '<path d="M5.5 8.5V4.2a1 1 0 0 1 2 0v3.3M7.5 7V3.2a1 1 0 0 1 2 0V7m0-.3V4a1 1 0 0 1 2 0v4.2m0-1.6a1 1 0 0 1 2 0v2.6a4.8 4.8 0 0 1-4.8 4.8H8.4a4 4 0 0 1-3.2-1.6L3 9.6a1 1 0 0 1 1.5-1.2l1 1"/>',
        connect: '<circle cx="4" cy="4" r="2"/><circle cx="12" cy="12" r="2"/><path d="M5.6 5.6l4.8 4.8"/>',
        minus: '<path d="M3 8h10"/>', plus: '<path d="M8 3v10M3 8h10"/>',
        fit: '<path d="M2.5 6V2.5H6M10 2.5h3.5V6M13.5 10v3.5H10M6 13.5H2.5V10"/>',
        undo: '<path d="M5.5 3L2.5 6l3 3"/><path d="M2.5 6h7a3.5 3.5 0 0 1 0 7H6"/>',
        redo: '<path d="M10.5 3l3 3-3 3"/><path d="M13.5 6h-7a3.5 3.5 0 0 0 0 7H10"/>',
        library: '<rect x="2" y="3" width="12" height="10" rx="1.5"/><path d="M6 3v10"/>',
        more: '<circle cx="3.5" cy="8" r="1.1" fill="currentColor"/><circle cx="8" cy="8" r="1.1" fill="currentColor"/><circle cx="12.5" cy="8" r="1.1" fill="currentColor"/>',
        search: '<circle cx="7" cy="7" r="4.5"/><path d="M10.4 10.4L14 14"/>',
        filter: '<path d="M2.5 3h11l-4.2 5.2V13l-2.6-1.3V8.2z"/>',
        minimap: '<rect x="2" y="2" width="12" height="12" rx="1.5"/><rect x="8" y="8" width="4" height="4"/>',
        chevron: '<path d="M4.5 6.5L8 10l3.5-3.5"/>',
        close: '<path d="M4 4l8 8M12 4l-8 8"/>',
    };
    const glyph = name => `<svg class="infra-glyph" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">${GLYPHS[name] || ''}</svg>`;

    // ---- relevance and cleanup (spec §9.2) -------------------------------------
    // Only devices and VMs are compared by address: containers share bridge
    // addresses and places have none.
    const DUPLICATE_KINDS = new Set(['device', 'vm']);
    const GENERIC_ROLES = new Set(['mdns-device', 'dial', 'wfadevice', 'basic']);
    const SCAN_TAGS = new Set(['network-scan', 'full-scan']);
    const SOURCE_RANK = { registry: 0, proxmox: 1, docker: 2, fritzbox: 3 };
    const DEFAULT_NAMES = new Set(['', 'unnamed device', 'unnamed candidate', 'fritz!box host', 'unknown', 'device']);
    const OPAQUE_NAMES = [
        /^[0-9a-f]{8,}$/i,
        /^(uuid:)?[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
        /^([0-9a-f]{2}[:-]){5}[0-9a-f]{2}$/i,
        /^\d{1,3}(\.\d{1,3}){3}$/,
        /^[0-9a-f]{0,4}(:[0-9a-f]{0,4}){2,7}$/i,
        /^[a-z][a-z0-9 _-]{0,24}[-_ ][0-9a-f]{12,}$/i,
    ];
    const naturalCompare = (a, b) => String(a ?? '').localeCompare(String(b ?? ''), undefined, { numeric: true, sensitivity: 'base' });
    // isOpaqueName: blank/default names, hex ids, UUIDs, MAC or IP literals and a
    // short word followed by a long hex run (mDNS instance names).
    const isOpaqueName = name => { const value = String(name ?? '').trim(); return DEFAULT_NAMES.has(value.toLowerCase()) || OPAQUE_NAMES.some(pattern => pattern.test(value)); };
    const normalizeIP = value => { const ip = String(value ?? '').trim().toLowerCase(); return !ip || ip === '0.0.0.0' || ip === '::' || ip === '::1' || ip.startsWith('127.') ? '' : ip; };
    const normalizeMAC = value => { const mac = String(value ?? '').trim().toLowerCase().replaceAll('-', ':'); return /^([0-9a-f]{2}:){5}[0-9a-f]{2}$/.test(mac) && mac !== '00:00:00:00:00:00' ? mac : ''; };
    const isLinkLocalIPv6 = ip => /^fe[89ab][0-9a-f]:/i.test(ip);
    const addressesOf = node => {
        const profile = node?.profile || {}, interfaces = Array.isArray(profile.interfaces) ? profile.interfaces : [];
        const ips = new Set([profile.ip_address, ...interfaces.map(item => item?.ip_address)].map(normalizeIP).filter(Boolean));
        const macs = new Set([profile.mac_address, ...interfaces.map(item => item?.mac_address)].map(normalizeMAC).filter(Boolean));
        return { ips: [...ips], macs: [...macs] };
    };
    const isGenericDiscovery = (node, sourceKind) => {
        const profile = node?.profile || {};
        return sourceKind === 'registry' && GENERIC_ROLES.has(String(profile.role || '').trim().toLowerCase())
            && (profile.tags || []).every(tag => SCAN_TAGS.has(String(tag).trim().toLowerCase()))
            && !String(profile.vendor || '').trim() && !String(profile.model || '').trim();
    };
    // A FRITZ!Box names itself as vendor of every host it sees; that says nothing.
    const detailLength = entry => (entry.sourceKind === 'fritzbox' ? 0 : String(entry.node?.profile?.vendor || '').trim().length) + String(entry.node?.profile?.model || '').trim().length;
    // compareSpecificity sorts {id, node, sourceKind} most descriptive first.
    const compareSpecificity = (a, b) => Number(isOpaqueName(a.node?.name)) - Number(isOpaqueName(b.node?.name))
        || detailLength(b) - detailLength(a)
        || (SOURCE_RANK[a.sourceKind] ?? 9) - (SOURCE_RANK[b.sourceKind] ?? 9)
        || Number(isGenericDiscovery(a.node, a.sourceKind)) - Number(isGenericDiscovery(b.node, b.sourceKind))
        || String(a.node?.name || '').length - String(b.node?.name || '').length
        || String(a.id).localeCompare(String(b.id));
    const addressIndex = entries => {
        const byIp = new Map(), byMac = new Map();
        for (const entry of entries) {
            if (!DUPLICATE_KINDS.has(entry.node?.kind)) continue;
            const { ips, macs } = addressesOf(entry.node);
            for (const ip of ips) byIp.set(ip, [...(byIp.get(ip) || []), entry]);
            for (const mac of macs) byMac.set(mac, [...(byMac.get(mac) || []), entry]);
        }
        return { byIp, byMac };
    };
    const candidateEntry = candidate => ({ id: candidate.id, node: candidate.node || {}, sourceKind: candidate.source?.kind || '' });
    const relevanceIndex = (nodes = [], candidates = []) => ({
        nodes: addressIndex((nodes || []).map(node => ({ id: node.id, node }))),
        candidates: addressIndex((candidates || []).filter(Boolean).map(candidateEntry)),
    });
    // discoveryRelevance: an existing asset wins over every discovery; among
    // discoveries sharing an address the most specific one is recommended.
    const discoveryRelevance = (candidate, nodes = [], candidates = [], { sourceHosts = [], index = null } = {}) => {
        const lookup = index || relevanceIndex(nodes, candidates);
        const node = candidate?.node || {}, source = candidate?.source || {};
        const own = addressesOf(node), reasons = [];
        let duplicateOf = null;
        if (DUPLICATE_KINDS.has(node.kind)) {
            for (const [field, match, values] of [['byMac', 'mac', own.macs], ['byIp', 'ip', own.ips]]) {
                const hit = values.map(value => (lookup.nodes[field].get(value) || [])[0]).find(Boolean);
                if (hit) { duplicateOf = { nodeId: hit.id, match }; break; }
            }
            if (!duplicateOf) {
                const peers = new Map();
                for (const [field, match, values] of [['byMac', 'mac', own.macs], ['byIp', 'ip', own.ips]]) {
                    for (const value of values) for (const entry of lookup.candidates[field].get(value) || []) if (entry.id !== candidate.id && !peers.has(entry.id)) peers.set(entry.id, { entry, match });
                }
                if (peers.size) {
                    const best = [candidateEntry(candidate), ...[...peers.values()].map(peer => peer.entry)].sort(compareSpecificity)[0];
                    if (best.id !== candidate.id) duplicateOf = { candidateId: best.id, match: peers.get(best.id).match };
                }
            }
        }
        if (duplicateOf) reasons.push(duplicateOf.match === 'mac' ? 'duplicate_mac' : 'duplicate_ip');
        if (isGenericDiscovery(node, source.kind)) reasons.push('generic_discovery');
        const power = String(source.observed?.power || '');
        const mapped = (sourceHosts || []).some(host => host?.kind === 'docker' && host.instance === source.instance && host.node_id);
        if ((source.kind === 'fritzbox' && power === 'stopped') || (source.kind === 'docker' && power === 'stopped' && !mapped)) reasons.push('inactive');
        if (own.ips.length && own.ips.every(isLinkLocalIPv6)) reasons.push('link_local');
        return { group: duplicateOf ? 'duplicate' : reasons.length ? 'unlikely' : 'recommended', reasons, ...(duplicateOf ? { duplicateOf } : {}) };
    };
    // groupDiscoveries returns the import sheet groups (sorted by name) and every relevance.
    const groupDiscoveries = (candidates = [], nodes = [], { sourceHosts = [] } = {}) => {
        const index = relevanceIndex(nodes, candidates), relevance = new Map(), groups = { recommended: [], duplicate: [], unlikely: [] };
        for (const candidate of candidates || []) {
            if (!candidate?.id) continue;
            const result = discoveryRelevance(candidate, nodes, candidates, { sourceHosts, index });
            relevance.set(candidate.id, result);
            groups[result.group].push(candidate);
        }
        for (const list of Object.values(groups)) list.sort((a, b) => naturalCompare(a.node?.name, b.node?.name) || String(a.id).localeCompare(String(b.id)));
        return { ...groups, relevance };
    };
    const LINKABLE_REGISTRY_KINDS = new Set(['device', 'vm', 'container']);
    // bindableFor: never a place or network; registry devices only onto devices,
    // VMs and containers without another registry device.
    const bindableFor = (candidate, node) => {
        if (!node || isContainer(node.kind) || node.kind === 'network') return false;
        if (candidate?.source?.kind !== 'registry') return true;
        return LINKABLE_REGISTRY_KINDS.has(node.kind) && (!node.registry_id || node.registry_id === candidate.registry_id);
    };
    const normalizedName = value => String(value ?? '').toLocaleLowerCase().replace(/[^a-z0-9]+/g, ' ').trim();
    // linkSuggestions ranks assets a discovery probably is: same MAC, address, name.
    const linkSuggestions = (candidate, nodes = [], limit = 5) => {
        const own = addressesOf(candidate?.node), name = normalizedName(candidate?.node?.name), scored = [];
        for (const node of nodes || []) {
            if (!bindableFor(candidate, node)) continue;
            const theirs = addressesOf(node);
            const score = (own.macs.some(mac => theirs.macs.includes(mac)) ? 4 : 0) + (own.ips.some(ip => theirs.ips.includes(ip)) ? 2 : 0) + (name && normalizedName(node.name) === name ? 1 : 0);
            if (score) scored.push({ id: node.id, name: node.name, score });
        }
        return scored.sort((a, b) => b.score - a.score || naturalCompare(a.name, b.name)).slice(0, limit).map(item => item.id);
    };
    const bestSourceKind = node => (node.sources || []).map(source => source.kind).sort((a, b) => (SOURCE_RANK[a] ?? 9) - (SOURCE_RANK[b] ?? 9))[0] || '';
    // A placed, connected, grouped or manual asset is the one a user invested in.
    const investment = (node, edgeCount) => Object.keys(node.placements || {}).length * 2 + (edgeCount.get(node.id) || 0) + (node.parent_id ? 1 : 0) + ((node.sources || []).length ? 0 : 3);
    const isUntouchedImport = node => (node.sources || []).some(source => [source.name, source.observed?.name].includes(node.name));
    // infraCleanup: duplicates by MAC or address, Docker guests whose engine has
    // no host, other hostless guests, untouched opaque names. `sources` is kept
    // for the spec signature.
    const infraCleanup = (nodes = [], sources = [], sourceHosts = [], { edges = [], dismissed = new Set() } = {}) => {
        const list = (nodes || []).filter(Boolean), edgeCount = new Map();
        for (const edge of edges || []) for (const id of [edge.from, edge.to]) edgeCount.set(id, (edgeCount.get(id) || 0) + 1);
        const rank = (a, b) => investment(b, edgeCount) - investment(a, edgeCount) || compareSpecificity({ id: a.id, node: a, sourceKind: bestSourceKind(a) }, { id: b.id, node: b, sourceKind: bestSourceKind(b) });
        const byName = (a, b) => naturalCompare(a.name, b.name) || String(a.id).localeCompare(String(b.id));
        const duplicates = [], seenGroups = new Set(), inDuplicate = new Set();
        const index = addressIndex(list.map(node => ({ id: node.id, node })));
        for (const [field, match] of [['byMac', 'mac'], ['byIp', 'ip']]) {
            for (const [value, entries] of [...index[field]].sort((a, b) => naturalCompare(a[0], b[0]))) {
                const members = [...new Set(entries.map(entry => entry.id))];
                if (members.length < 2) continue;
                const key = members.slice().sort().join(',');
                if (seenGroups.has(key)) continue;
                seenGroups.add(key);
                const ranked = members.map(id => list.find(node => node.id === id)).sort(rank);
                ranked.forEach(node => inDuplicate.add(node.id));
                duplicates.push({ id: `duplicate:${match}:${value}:${key}`, type: 'duplicate', match, value, keepId: ranked[0].id, removeIds: ranked.slice(1).map(node => node.id), nodeIds: ranked.map(node => node.id) });
            }
        }
        const { hostOf } = guestIndex(list, edges);
        const engines = new Map(), guestHosts = [];
        for (const node of list.filter(item => (item.kind === 'vm' || item.kind === 'container') && !hostOf.has(item.id)).sort(byName)) {
            const docker = (node.sources || []).find(source => source.kind === 'docker' && source.instance);
            if (docker) engines.set(docker.instance, [...(engines.get(docker.instance) || []), node.id]);
            else guestHosts.push({ id: `guest-host:${node.id}`, type: 'guest-host', nodeId: node.id, nodeIds: [node.id] });
        }
        const engineHosts = [...engines].sort((a, b) => naturalCompare(a[0], b[0])).map(([instance, guestIds]) => {
            const entry = (sourceHosts || []).find(host => host?.kind === 'docker' && host.instance === instance) || {};
            return { id: `engine-host:docker:${instance}`, type: 'engine-host', kind: 'docker', instance, version: Number(entry.version) || 0, inferredNodeId: String(entry.inferred_node_id || ''), guestIds, nodeIds: guestIds };
        });
        const opaque = list.filter(node => node.kind === 'device' && !inDuplicate.has(node.id) && isOpaqueName(node.name) && isUntouchedImport(node)
            && (node.sources || []).some(source => source.kind === 'registry' || source.kind === 'fritzbox')).sort(byName)
            .map(node => ({ id: `opaque:${node.id}`, type: 'opaque', nodeId: node.id, nodeIds: [node.id] }));
        return [...duplicates, ...engineHosts, ...guestHosts, ...opaque].filter(item => !dismissed.has(item.id));
    };

    window.InfraAppModel = {
        esc, clone, iconId, iconURL, assetURL, iconLabelKey, sourceActions, sourceIdentity, virtualSlice, sourceState,
        visibleByCollapsed, isPlaced, validCIDR, validHTTPLink, iconIds: () => [...ICON_LIST],
        acceptsIcon, rememberIcon, versionedAsset, containerKinds: () => [...CONTAINER_KINDS], isContainer, powerLabel,
        edgeSegment, edgeCaption, gridPositions, topologyPositions, snapshotStamp,
        layoutSnapshot, applyLayoutPatch,
        CARD_W, CARD_H, GRID, MIN_ZOOM, MAX_ZOOM, REGION_W, REGION_H, REGION_PAD, REGION_MIN_W, REGION_MIN_H,
        clampZoom, toWorld, toScreen, zoomAt, fitView, viewCenter, viewFromCenter, snap, cardRect, intersects, containsPoint,
        normalizeRect, padRect, unionRects, freeSlot, regionRect, regionLayout, regionAt, dropParent, growRegions,
        spatialNeighbour, compactGrid, defaultEdgeKind, guestIndex, matchesFilters, filtersActive, buildScene,
        mergeIntent, layoutItems, rebaseLayout, createHistory, readLayoutPrefs, fill, normalizeNode, minimapTransform, glyph,
        naturalCompare, isOpaqueName, addressesOf, isGenericDiscovery, compareSpecificity, relevanceIndex, discoveryRelevance, groupDiscoveries, bindableFor, linkSuggestions, infraCleanup,
    };
})();
