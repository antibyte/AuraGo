// Pure Infra helpers. Loaded first; every other infra-*.js file reads window.InfraAppModel.
(function () {
    'use strict';

    const ICON_LIST = 'rack-server tower-server mini-pc sbc nas disk-shelf workstation laptop tablet smartphone router firewall modem-ont managed-switch switch access-point mesh-node ups pdu patch-panel camera nvr printer 3d-printer smart-hub sensor smart-plug tv console radio-gateway usb-gateway generic-device vm container hypervisor kubernetes database web-service reverse-proxy dns-dhcp vpn storage-share backup monitoring home-automation media-server ai-runtime mqtt logs git-ci mail-server photo-library password-manager cloud-service automation generic-app rack site room lan vlan wlan internet group'.split(' ');
    const ICONS = new Set(ICON_LIST);
    const ICON_ASSETS = new Map();
    const CONTAINER_KINDS = ['site', 'room', 'rack', 'group'];
    const POWERS = new Set(['running', 'stopped', 'paused', 'unknown']);
    // A card is 216×64 world px (Plan D amendment A2); its placement is the centre.
    const CARD_W = 216, CARD_H = 64;
    const CARD_HALF = { w: CARD_W / 2, h: CARD_H / 2 };

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
    const edgeSegment = (a, b, halfA = CARD_HALF, halfB = CARD_HALF) => {
        const x1 = Number(a.x), y1 = Number(a.y), x2 = Number(b.x), y2 = Number(b.y);
        const start = borderPoint({ x: x1, y: y1 }, { x: x2, y: y2 }, halfA);
        const end = borderPoint({ x: x2, y: y2 }, { x: x1, y: y1 }, halfB);
        if ((end.x - start.x) * (x2 - x1) + (end.y - start.y) * (y2 - y1) <= 0) return { x1, y1, x2, y2 };
        return { x1: start.x, y1: start.y, x2: end.x, y2: end.y };
    };
    // edgeLabelClash: would a label of size {w, h}, centred on the segment's midpoint and padded by pad, touch any of rects?
    const edgeLabelClash = (seg, size, rects, pad = 4) => {
        const box = { x: (seg.x1 + seg.x2) / 2 - size.w / 2 - pad, y: (seg.y1 + seg.y2) / 2 - size.h / 2 - pad, w: size.w + pad * 2, h: size.h + pad * 2 };
        return (rects || []).some(rect => intersects(box, rect));
    };
    const edgeCaption = edge => {
        const from = String(edge?.from_port || '').trim(), to = String(edge?.to_port || '').trim(), speed = String(edge?.speed || '').trim();
        const ports = from && to ? `${from} → ${to}` : (from || to);
        return [ports, speed].filter(Boolean).join(' · ');
    };
    // Arrange and the Topology fallback lay cards out on a 252×160 pitch from (126, 100).
    const ARRANGE_X0 = 126, ARRANGE_Y0 = 100, ARRANGE_DX = 252, ARRANGE_DY = 160;
    const overlaps = (left, right) => Math.abs(Number(left.x) - Number(right.x)) < CARD_W - 24 && Math.abs(Number(left.y) - Number(right.y)) < CARD_H + 48;
    // Grid cells make the placement search linear: generated positions are exact
    // grid points, so a Set replaces the per-attempt array scan.
    const gridKey = (x, y) => `${Math.round((Number(x) - ARRANGE_X0) / ARRANGE_DX)},${Math.round((Number(y) - ARRANGE_Y0) / ARRANGE_DY)}`;
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
                const candidate = { x: ARRANGE_X0 + (index % columns) * ARRANGE_DX, y: ARRANGE_Y0 + Math.floor(index / columns) * ARRANGE_DY };
                const key = gridKey(candidate.x, candidate.y);
                if (blocked.has(key) || occupied.some(pos => overlaps(pos, candidate))) continue;
                position = { ...candidate, pinned: false };
                blocked.add(key);
            }
            if (position) changes.set(node.id, position);
        }
        return changes;
    };
    const sourceStamp = source => [source?.kind || '', source?.instance || '', source?.key || '', source?.state || '', source?.observed?.power || '', source?.ignored ? 'ignored' : ''].join('\x1f');
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
        const nodePart = (nodes || []).map(node => [node.id, node.version, node.kind, node.name, node.parent_id || '', node.host_id || '', node.power || '', JSON.stringify(node.placements || {}), (node.sources || []).map(sourceStamp).join('\x1f')].join('\x1f')).join('\x1e');
        const edgePart = (edges || []).map(edge => [edge.id, edge.version, edge.from, edge.to, edge.kind].join('\x1f')).join('\x1e');
        const sourcePart = (sources || []).map(sourceStamp).join('\x1e');
        const capPart = [!!caps?.write, !!caps?.source_edit, !!caps?.control].join('\x1f');
        return [nodePart, edgePart, sourcePart, capPart].join('\x1d');
    };

    // ---- canvas world -------------------------------------------------------
    // World coordinates are CSS pixels at zoom 1. An asset placement (x, y) is
    // the centre of its 216×64 card; a place placement (x, y) is the top-left
    // corner of its region and (w, h) its size (default 360×240).
    const GRID = 24, MIN_ZOOM = 0.25, MAX_ZOOM = 2;
    const REGION_W = 360, REGION_H = 240, REGION_PAD = 48, REGION_MIN_W = 200, REGION_MIN_H = 140;
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
    // ---- semantic zoom (A1) --------------------------------------------------------
    // Below COMPACT_BELOW cards draw in a compact form (icon and name only); once compact they stay so until
    // COMPACT_RELEASE, so a pinch near the boundary does not flicker. Names, region headers and lane labels are
    // counter-scaled so their on-screen size never falls below LABEL_FLOOR px; above 1.0 they scale with the scene.
    const COMPACT_BELOW = 0.65, COMPACT_RELEASE = 0.68, HANDLES_FROM = 0.5, EDGE_LABELS_FROM = 0.75, TINY_BELOW = 0.4;
    const LABEL_FLOOR = { name: [16, 12], header: [16, 13], lane: [14, 13] };
    // labelScale is rounded up to 0.05, so the labels are laid out again only when the step changes.
    const labelScale = (zoom, basePx, floorPx) => {
        const z = Number.isFinite(zoom) && zoom > 0 ? zoom : 1, raw = floorPx / (basePx * z);
        return raw <= 1 ? 1 : Math.ceil(raw * 20) / 20;
    };
    const zoomLevels = (zoom, previousForm = 'full') => ({
        form: zoom < COMPACT_BELOW || (previousForm === 'compact' && zoom < COMPACT_RELEASE) ? 'compact' : 'full',
        tiny: zoom < TINY_BELOW, handles: zoom >= HANDLES_FROM, edgeLabels: zoom >= EDGE_LABELS_FROM,
        // the status ring of a compact icon stays at least 1.6 px on screen
        ring: Math.max(2, Math.ceil(1.6 / (Number.isFinite(zoom) && zoom > 0 ? zoom : 1) * 2) / 2),
        name: labelScale(zoom, ...LABEL_FLOOR.name), header: labelScale(zoom, ...LABEL_FLOOR.header), lane: labelScale(zoom, ...LABEL_FLOOR.lane),
    });
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
    // nearest ring around it whose card (plus gap) overlaps no obstacle rect. The
    // 8 px gap makes a stacked card land one card height plus gap = 72 px (3 cells) away.
    const freeSlot = (want, obstacles, { grid = GRID, gap = 8, maxRing = 40 } = {}) => {
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
    // 240×96 apart (both multiples of the 24 px grid).
    const compactGrid = (count, centre) => {
        const cols = Math.max(1, Math.ceil(Math.sqrt(count))), rows = Math.ceil(count / cols);
        const cellW = CARD_W + 24, cellH = CARD_H + 32;
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
        const drawn = unionRects([...cards.map(cardRect), ...rects.values()]);
        const lanes = view === 'topology' ? laneBands(drawn) : [];
        return { cards, cardById, regions, regionRects: rects, regionDepth: depth, lines, lanes, bounds: lanes.length ? unionRects([drawn, ...lanes.map(lane => lane.rect)]) : drawn };
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
    // countText says "1 asset" or "3 assets": the singular text for exactly one, the plural text otherwise ({count} is filled in).
    const countText = (count, one, other, vars = {}) => fill(Number(count) === 1 ? one : other, { ...vars, count });
    // normalizeNode fills UI defaults. It never adds fields the snapshot did not
    // send: node PUT bodies are clones of these objects and the server rejects
    // unknown fields.
    const normalizeNode = node => ({ ...node, icon: iconId(node.icon), parent_id: node.parent_id || '', profile: { tags: [], interfaces: [], links: [], app_bindings: [], custom: {}, ...(node.profile || {}) }, placements: node.placements || {}, sources: node.sources || [], app_links: node.app_links || [], available_actions: node.available_actions || [] });
    // sceneFits: the whole scene lies inside the visible part of the world (the minimap has nothing to show then).
    const sceneFits = (bounds, view, size) => {
        if (!bounds || !(size.w > 0 && size.h > 0) || !(view.zoom > 0)) return true;
        const left = -view.x / view.zoom, top = -view.y / view.zoom, right = left + size.w / view.zoom, bottom = top + size.h / view.zoom;
        return bounds.x >= left - 1 && bounds.y >= top - 1 && bounds.x + bounds.w <= right + 1 && bounds.y + bounds.h <= bottom + 1;
    };
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
    // sourceLabels names sources of one kind apart without ever showing the opaque instance id. A kind with
    // several instances is numbered in instance order ("Docker 1", "Docker 2", stable across polls); the id only
    // goes into the title. of(source) is {text, title}; several is the set of kinds that were numbered.
    const sourceLabels = (sources, kindLabel) => {
        const instances = new Map();
        for (const source of sources || []) {
            if (!source || source.ignored) continue;
            instances.set(source.kind, (instances.get(source.kind) || new Set()).add(String(source.instance || '')));
        }
        const order = new Map([...instances].filter(([, set]) => set.size > 1).map(([kind, set]) => [kind, [...set].sort(naturalCompare)]));
        return {
            several: new Set(order.keys()),
            of(source) {
                const base = kindLabel(source?.kind), instance = String(source?.instance || ''), rank = (order.get(source?.kind) || []).indexOf(instance);
                return { text: rank >= 0 ? `${base} ${rank + 1}` : base, title: instance ? `${base} · ${instance}` : base };
            },
        };
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

    // ---- List view, inspector and picker helpers -------------------------------
    const TABLE_SORTS = ['name', 'kind', 'place', 'address', 'source', 'status', 'power'];
    const STATUS_ORDER = { removed: 0, stale: 1, unknown: 2, current: 3, manual: 4 };
    const POWER_ORDER = { running: 0, paused: 1, stopped: 2, unknown: 3 };
    const ipv4Number = value => { const match = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(String(value ?? '').trim()); return match ? match.slice(1).reduce((sum, part) => sum * 256 + Number(part), 0) : null; };
    const compareAddress = (a, b) => { const x = ipv4Number(a), y = ipv4Number(b); if (x !== null && y !== null) return x - y; if (x !== null) return -1; if (y !== null) return 1; return naturalCompare(a, b); };
    const tableSortValue = (node, key, ctx) => {
        if (key === 'kind') return ctx.labelOf(node.kind);
        if (key === 'place') return ctx.placeOf(node.id);
        if (key === 'address') return node.profile?.ip_address || node.profile?.mac_address || '';
        if (key === 'source') return ctx.sourceOf(node);
        if (key === 'status') return STATUS_ORDER[sourceState(node)];
        if (key === 'power') return Object.prototype.hasOwnProperty.call(POWER_ORDER, node.power || '') ? POWER_ORDER[node.power] : null;
        return node.name;
    };
    // tableComparator: empty values last in both directions; ties by name, then id.
    const tableComparator = (key = 'name', dir = 'asc', ctx = {}) => {
        const sign = dir === 'desc' ? -1 : 1;
        const context = { labelOf: kind => kind, placeOf: () => '', sourceOf: node => (node.sources || []).map(source => source.kind).join(','), ...ctx };
        const empty = value => value === null || value === undefined || value === '';
        return (a, b) => {
            const x = tableSortValue(a, key, context), y = tableSortValue(b, key, context);
            if (empty(x) || empty(y)) { if (empty(x) && empty(y)) return naturalCompare(a.name, b.name) || String(a.id).localeCompare(String(b.id)); return empty(x) ? 1 : -1; }
            const primary = key === 'address' ? compareAddress(x, y) : typeof x === 'number' && typeof y === 'number' ? x - y : naturalCompare(x, y);
            return primary * sign || naturalCompare(a.name, b.name) || String(a.id).localeCompare(String(b.id));
        };
    };
    // tableRows flattens List rows: top-level assets sorted, guests below their
    // host when expanded. While filtering, a non-matching host above a match is
    // a context row and opens automatically.
    const tableRows = (nodes, { hostOf = new Map(), guestsOf = new Map(), compare = () => 0, matches = () => true, expanded = new Set(), filtering = false } = {}) => {
        const byId = new Map((nodes || []).map(node => [node.id, node])), memo = new Map();
        const visible = (id, trail = new Set()) => {
            if (memo.has(id)) return memo.get(id);
            if (trail.has(id)) return false;
            trail.add(id);
            const node = byId.get(id);
            const result = !!node && (matches(node) || (guestsOf.get(id) || []).some(guest => visible(guest, trail)));
            trail.delete(id); memo.set(id, result);
            return result;
        };
        const rows = [], emitted = new Set();
        const emit = (node, depth) => {
            if (emitted.has(node.id)) return;
            emitted.add(node.id);
            const guests = (guestsOf.get(node.id) || []).map(id => byId.get(id)).filter(Boolean);
            const shown = filtering ? guests.filter(guest => visible(guest.id)) : guests;
            const open = guests.length > 0 && (expanded.has(node.id) || (filtering && shown.length > 0));
            rows.push({ id: node.id, depth, guests: guests.length, expanded: open, context: filtering && !matches(node) });
            if (open) for (const guest of shown.slice().sort(compare)) emit(guest, depth + 1);
        };
        for (const node of (nodes || []).filter(item => !byId.has(hostOf.get(item.id))).sort(compare)) if (!filtering || visible(node.id)) emit(node, 0);
        return rows;
    };
    const INSPECTOR_TABS = ['overview', 'details', 'sources', 'connections'];
    const parseJSON = raw => { try { const data = raw ? JSON.parse(raw) : {}; return data && typeof data === 'object' ? data : {}; } catch (_) { return {}; } };
    const readTablePrefs = raw => { const data = parseJSON(raw); return { sort: TABLE_SORTS.includes(data.sort) ? data.sort : 'name', dir: data.dir === 'desc' ? 'desc' : 'asc' }; };
    const readInspectorPrefs = raw => {
        const data = parseJSON(raw), tabs = {};
        if (data.tabs && typeof data.tabs === 'object') for (const [kind, tab] of Object.entries(data.tabs)) if (/^[a-z]{1,16}$/.test(kind) && INSPECTOR_TABS.includes(tab)) tabs[kind] = tab;
        return { tabs };
    };
    const inspectorTab = (prefs, kind) => (INSPECTOR_TABS.includes(prefs?.tabs?.[kind]) ? prefs.tabs[kind] : 'overview');
    const withInspectorTab = (prefs, kind, tab) => ({ tabs: { ...(prefs?.tabs || {}), ...(INSPECTOR_TABS.includes(tab) && /^[a-z]{1,16}$/.test(kind) ? { [kind]: tab } : {}) } });
    const readDismissed = raw => { let list; try { list = JSON.parse(raw || '[]'); } catch (_) { list = []; } return new Set((Array.isArray(list) ? list : []).filter(item => typeof item === 'string').slice(-500)); };
    const blankValue = value => value === undefined || value === null || value === '' || value === 0 || (Array.isArray(value) && !value.length) || (typeof value === 'object' && value !== null && !Array.isArray(value) && !Object.keys(value).length);
    const sameValue = (a, b) => (blankValue(a) && blankValue(b)) || JSON.stringify(a) === JSON.stringify(b);
    // listEntryText reads one entry of a profile list the way the editor shows it: an
    // interface as "name · ip", a link as "label · url", an app link by its target label
    // (the app and target id when the target is gone). `targets` is state.appTargets.
    const listEntryText = (key, item, { targets = [], t = text => text } = {}) => {
        if (item == null) return '';
        if (typeof item !== 'object') return String(item);
        const join = parts => parts.filter(part => part != null && part !== '').join(' · ');
        if (key === 'interfaces') return join([item.name, item.ip_address, item.mac_address, item.vlan ? `${t('desktop.infra_field_vlan')} ${item.vlan}` : '', item.speed]);
        if (key === 'links') return join([item.label, item.url]);
        if (key === 'app_bindings') return targets.find(target => target.app === item.app && target.target_id === item.target_id)?.label || join([item.app, item.target_id]);
        return JSON.stringify(item).slice(0, 80);
    };
    // draftDiff lists changed fields: name, kind, icon, parent_id, profile.<key>.
    const draftDiff = (node, draft) => {
        const changed = new Set();
        if (!node || !draft) return changed;
        for (const key of ['name', 'kind', 'icon', 'parent_id']) if (!sameValue(node[key], draft[key])) changed.add(key);
        for (const key of new Set([...Object.keys(node.profile || {}), ...Object.keys(draft.profile || {})])) if (!sameValue(node.profile?.[key], draft.profile?.[key])) changed.add(`profile.${key}`);
        return changed;
    };
    const FORM_GROUPS = {
        identity: ['name', 'kind', 'icon', 'parent_id', 'profile.role', 'profile.tags'],
        network: ['profile.ip_address', 'profile.mac_address', 'profile.cidr', 'profile.vlan', 'profile.interfaces'],
        hardware: ['profile.vendor', 'profile.model', 'profile.serial', 'profile.os', 'profile.firmware', 'profile.cpu', 'profile.ram', 'profile.storage', 'profile.rack_position'],
        links: ['profile.links', 'profile.app_bindings'],
        notes: ['profile.notes'],
        custom: ['profile.custom'],
    };
    const fieldValue = (node, path) => (path.startsWith('profile.') ? node?.profile?.[path.slice(8)] : node?.[path]);
    const formGroupFill = node => Object.fromEntries(Object.entries(FORM_GROUPS).map(([group, fields]) => [group, group === 'identity' || fields.some(path => !blankValue(fieldValue(node, path)))]));
    const groupOfField = path => Object.entries(FORM_GROUPS).find(([, fields]) => fields.includes(path))?.[0] || (path.startsWith('profile.') ? 'custom' : 'identity');
    const MESH_ICONS = new Set(['mesh-node', 'radio-gateway']);
    const APP_ORDER = ['network-cameras', 'software-store', 'meshcore'];
    // The apps an asset can link to; the inspector and the context menus read this one set.
    const APP_LINK_TARGETS = new Set(['quick-connect', 'network-cameras', 'software-store', 'meshcore']);
    const isMeshAsset = node => MESH_ICONS.has(node?.icon) || /mesh/i.test(String(node?.profile?.role || '')) || (node?.profile?.tags || []).some(tag => /mesh/i.test(String(tag)));
    const tokensOf = value => new Set(String(value || '').toLocaleLowerCase().split(/[^\p{L}\p{N}]+/u).filter(token => token.length >= 3));
    const targetScore = (node, target) => {
        const label = String(target.label || '').toLocaleLowerCase(), id = String(target.target_id || '').toLocaleLowerCase();
        const ip = String(node?.profile?.ip_address || '').trim().toLocaleLowerCase();
        let score = ip && (label.includes(ip) || id.includes(ip)) ? 3 : 0;
        const theirs = tokensOf(target.label);
        for (const token of tokensOf(node?.name)) if (theirs.has(token)) score += 2;
        return score;
    };
    // appTargetOptions: unbound targets matching the query, suggestions first,
    // then one group per app. MeshCore only for radio/mesh assets or a query of
    // two characters or more.
    const appTargetOptions = (node, targets, query = '', { limit = 50 } = {}) => {
        const q = String(query || '').trim().toLocaleLowerCase();
        const bound = new Set((node?.profile?.app_bindings || []).map(item => `${item.app}\x00${item.target_id}`));
        const meshAllowed = isMeshAsset(node) || q.length >= 2;
        let hiddenMesh = 0;
        const pool = [];
        for (const target of targets || []) {
            if (!target?.app || !target.target_id || bound.has(`${target.app}\x00${target.target_id}`)) continue;
            if (target.app === 'meshcore' && !meshAllowed) { hiddenMesh += 1; continue; }
            const label = String(target.label || target.target_id);
            if (q && !label.toLocaleLowerCase().includes(q) && !String(target.target_id).toLocaleLowerCase().includes(q)) continue;
            pool.push({ app: target.app, target_id: String(target.target_id), label, score: targetScore(node, target) });
        }
        const byLabel = (a, b) => naturalCompare(a.label, b.label) || a.target_id.localeCompare(b.target_id);
        const suggested = pool.filter(option => option.score > 0).sort((a, b) => b.score - a.score || byLabel(a, b)).slice(0, 5);
        const taken = new Set(suggested.map(option => `${option.app}\x00${option.target_id}`));
        const groups = suggested.length ? [{ id: 'suggested', options: suggested }] : [];
        for (const app of APP_ORDER) {
            const list = pool.filter(option => option.app === app && !taken.has(`${option.app}\x00${option.target_id}`)).sort(byLabel);
            if (list.length) groups.push({ id: app, options: list.slice(0, limit), more: Math.max(0, list.length - limit) });
        }
        return { groups, hiddenMesh };
    };
    const appLinkContext = link => ({ 'quick-connect': { device_id: link?.target_id }, 'network-cameras': { camera_id: link?.target_id }, 'software-store': { app_id: link?.target_id }, meshcore: { conversation_id: link?.target_id } }[link?.app] || { target_id: link?.target_id });
    // placePath: place names from the outermost place down to id's parent (or id).
    const placePath = (id, nodeById, { includeSelf = false } = {}) => {
        const path = [], seen = new Set();
        let current = includeSelf ? id : nodeById(id)?.parent_id || '';
        while (current && !seen.has(current)) { seen.add(current); const node = nodeById(current); if (!node) break; path.unshift({ id: node.id, name: node.name }); current = node.parent_id || ''; }
        return path;
    };
    const isDescendant = (id, ancestorId, parentOf) => { const seen = new Set(); for (let current = parentOf(id); current && !seen.has(current); current = parentOf(current)) { if (current === ancestorId) return true; seen.add(current); } return false; };
    const ATTENTION_STATES = new Set(['stale', 'unknown', 'removed']);
    // attentionItems: any linked source that is not current, and unfinished writes.
    const attentionItems = (nodes = [], { unsaved = new Set(), reviews = new Set(), sourceBlocked = new Set(), actionBlocked = new Set() } = {}) => (nodes || []).map(node => {
        const reasons = [];
        for (const source of node.sources || []) if (!source.ignored && ATTENTION_STATES.has(source.state)) reasons.push({ code: `source_${source.state}`, source: source.kind });
        if (unsaved.has(node.id)) reasons.push({ code: 'layout_unsaved' });
        if (reviews.has(node.id)) reasons.push({ code: 'draft_conflict' });
        if (sourceBlocked.has(node.id)) reasons.push({ code: 'source_review' });
        if (actionBlocked.has(node.id)) reasons.push({ code: 'action_review' });
        return { node, reasons };
    }).filter(item => item.reasons.length).sort((a, b) => naturalCompare(a.node.name, b.node.name) || String(a.node.id).localeCompare(String(b.node.id))).map(item => ({ id: item.node.id, reasons: item.reasons }));

    // ---- topology lanes and auto layout (spec §6) --------------------------------
    // Four fixed horizontal world bands, LANE_H px tall, from y = 0. A card the
    // user placed (pinned) belongs to the lane its centre y is in (laneAt);
    // every other card belongs to topologyLane(node). Auto rows sit LANE_CELL_W
    // apart: the 216 px card plus one 24 px grid cell.
    const TOPOLOGY_LANES = ['internet', 'network', 'compute', 'clients'];
    const LANE_H = 480, LANE_ROW0 = 96, LANE_ROW_H = 96, LANE_CELL_W = 240, LANE_PAD_X = 240, LANE_MIN_W = 1440;
    const PHYSICAL_EDGES = new Set(['ethernet', 'fiber', 'wifi']);
    const LANE_ICONS = {
        internet: ['internet', 'modem-ont'],
        network: ['router', 'firewall', 'managed-switch', 'switch', 'patch-panel', 'access-point', 'mesh-node', 'lan', 'vlan', 'wlan'],
        compute: ['rack-server', 'tower-server', 'mini-pc', 'sbc', 'nas', 'hypervisor', 'kubernetes'],
    };
    const LANE_ICON_INDEX = new Map(Object.entries(LANE_ICONS).flatMap(([lane, icons]) => icons.map((icon, rank) => [icon, { lane, rank }])));
    const LANE_ROLE_WORDS = [
        ['internet', ['modem', 'ont', 'isp', 'wan', 'dsl']],
        ['network', ['router', 'firewall', 'switch', 'gateway', 'mesh', 'ap', 'access point']],
        ['compute', ['server', 'nas', 'hypervisor', 'proxmox', 'esxi', 'kubernetes', 'k8s']],
    ];
    // roleWords turns "InternetGatewayDevice" or "dsl-modem" into " internet gateway device ".
    const roleWords = role => ` ${String(role || '').replace(/([a-z0-9])([A-Z])/g, '$1 $2').toLowerCase().replace(/[^a-z0-9]+/g, ' ').trim()} `;
    // topologyLane: the network kind, then the icon, then words in the role,
    // then "runs guests"; everything else is a client.
    const topologyLane = (node, { hasGuests = false } = {}) => {
        if (node?.kind === 'network') return 'network';
        const byIcon = LANE_ICON_INDEX.get(node?.icon);
        if (byIcon) return byIcon.lane;
        const words = roleWords(node?.profile?.role);
        for (const [lane, list] of LANE_ROLE_WORDS) if (list.some(word => words.includes(` ${word} `))) return lane;
        return hasGuests ? 'compute' : 'clients';
    };
    const topologyLaneIndex = lane => Math.max(0, TOPOLOGY_LANES.indexOf(lane));
    const laneAt = y => { const n = Number(y); return Number.isFinite(n) ? Math.min(TOPOLOGY_LANES.length - 1, Math.max(0, Math.floor(n / LANE_H))) : TOPOLOGY_LANES.length - 1; };
    // laneBands spans the four lanes across box (the drawn cards) with
    // LANE_PAD_X to spare on both sides, at least LANE_MIN_W wide.
    const laneBands = box => {
        const centre = box ? box.x + box.w / 2 : 0;
        const width = Math.ceil(Math.max(LANE_MIN_W, box ? box.w + LANE_PAD_X * 2 : 0) / GRID) * GRID;
        const x = snap(centre - width / 2);
        return TOPOLOGY_LANES.map((id, index) => ({ id, index, rect: { x, y: index * LANE_H, w: width, h: LANE_H } }));
    };
    const laneRank = node => LANE_ICON_INDEX.get(node?.icon)?.rank ?? 99;
    // topologyLayout puts nodes into their lanes (laneOf → index) in connection
    // order: a breadth-first walk over Ethernet, fiber and Wi-Fi edges, started
    // from the top lane, numbers the nodes, so parents come before children.
    // A lane holds up to maxRows rows of up to maxColumns cards (more widen the
    // rows); every row is centred on centerX. obstacles are rects of cards that
    // stay put; a slot under one moves to the nearest free grid point.
    const topologyLayout = (nodes, edges, { laneOf = node => topologyLaneIndex(topologyLane(node)), obstacles = [], centerX = 0, maxColumns = 8, maxRows = 4 } = {}) => {
        const list = (nodes || []).filter(node => node && node.id);
        const byId = new Map(list.map(node => [node.id, node]));
        const lane = new Map(list.map(node => [node.id, Math.min(TOPOLOGY_LANES.length - 1, Math.max(0, Math.trunc(Number(laneOf(node))) || 0))]));
        const compare = (a, b) => lane.get(a.id) - lane.get(b.id) || laneRank(a) - laneRank(b) || naturalCompare(a.name, b.name) || String(a.id).localeCompare(String(b.id));
        const adjacency = new Map(list.map(node => [node.id, []]));
        for (const edge of edges || []) {
            if (!PHYSICAL_EDGES.has(edge?.kind) || edge.from === edge.to || !byId.has(edge.from) || !byId.has(edge.to)) continue;
            adjacency.get(edge.from).push(byId.get(edge.to));
            adjacency.get(edge.to).push(byId.get(edge.from));
        }
        for (const neighbours of adjacency.values()) neighbours.sort(compare);
        const order = new Map();
        for (const seed of list.slice().sort(compare)) {
            if (order.has(seed.id)) continue;
            order.set(seed.id, order.size);
            const queue = [seed];
            for (let head = 0; head < queue.length; head += 1) {
                for (const next of adjacency.get(queue[head].id)) {
                    if (order.has(next.id)) continue;
                    order.set(next.id, order.size);
                    queue.push(next);
                }
            }
        }
        const lanes = TOPOLOGY_LANES.map(() => []);
        for (const node of list) lanes[lane.get(node.id)].push(node);
        const nudge = (obstacles || []).length > 0, placed = nudge ? obstacles.slice() : [], out = new Map();
        lanes.forEach((members, index) => {
            members.sort((a, b) => order.get(a.id) - order.get(b.id));
            const rows = Math.max(1, Math.min(maxRows, Math.ceil(members.length / maxColumns)));
            const columns = Math.max(1, Math.ceil(members.length / rows));
            members.forEach((node, i) => {
                const row = Math.floor(i / columns), inRow = Math.min(columns, members.length - row * columns);
                const want = { x: snap(centerX - (inRow - 1) * LANE_CELL_W / 2) + (i % columns) * LANE_CELL_W, y: index * LANE_H + LANE_ROW0 + row * LANE_ROW_H };
                const pos = nudge ? freeSlot(want, placed) : want;
                if (nudge) placed.push(cardRect(pos));
                out.set(node.id, { x: pos.x, y: pos.y, pinned: false });
            });
        });
        return out;
    };
    // topologyAutoPositions: where Topology draws assets without a topology
    // placement. Every eligible asset (not a place, not a guest, not a VM or
    // container without a host) gets the slot of one layout over all of them,
    // so the slots only change when assets or connections change and moving
    // one card never shifts another; a slot under a placed card moves aside.
    const topologyAutoPositions = (nodes, edges, { placementOf = () => null, guests = { hostOf: new Map(), guestsOf: new Map() } } = {}) => {
        const drawable = (nodes || []).filter(node => node && !isContainer(node.kind) && !guests.hostOf.has(node.id));
        const eligible = drawable.filter(node => node.kind !== 'vm' && node.kind !== 'container');
        const slots = topologyLayout(eligible, edges, { laneOf: node => topologyLaneIndex(topologyLane(node, { hasGuests: (guests.guestsOf.get(node.id) || []).length > 0 })) });
        const obstacles = drawable.map(node => placementOf(node.id)).filter(p => p && finite(p.x) != null && finite(p.y) != null).map(cardRect);
        const nudge = obstacles.length > 0, out = new Map();
        for (const node of eligible) {
            if (placementOf(node.id)) continue;
            const want = slots.get(node.id);
            const pos = nudge ? freeSlot(want, obstacles) : want;
            if (nudge) obstacles.push(cardRect(pos));
            out.set(node.id, { x: pos.x, y: pos.y, pinned: false });
        }
        return out;
    };
    // topologyArrange plans Arrange in Topology: the selected cards (centred on
    // their mean x, around the other cards) or, without a selection, every
    // card. A card the user placed (pinned) keeps the lane its y is in and
    // stays pinned; the others follow their type and are saved unpinned.
    const topologyArrange = ({ cards = [], selection = new Set(), nodeById, edges = [], guestsOf = new Map(), placementOf = () => null }) => {
        const chosen = cards.filter(card => selection.has(card.id));
        const targets = (chosen.length ? chosen : cards).filter(card => nodeById(card.id));
        const ids = new Set(targets.map(card => card.id));
        const manual = id => { const placement = placementOf(id); return !!placement && placement.pinned !== false; };
        const yOf = new Map(cards.map(card => [card.id, card.y]));
        const laneOf = node => (manual(node.id) ? laneAt(yOf.get(node.id)) : topologyLaneIndex(topologyLane(node, { hasGuests: (guestsOf.get(node.id) || []).length > 0 })));
        const obstacles = chosen.length ? cards.filter(card => !ids.has(card.id)).map(cardRect) : [];
        const centerX = chosen.length ? snap(chosen.reduce((sum, card) => sum + card.x, 0) / chosen.length) : 0;
        const out = new Map();
        for (const [id, pos] of topologyLayout(targets.map(card => nodeById(card.id)), edges, { laneOf, obstacles, centerX })) out.set(id, { x: pos.x, y: pos.y, pinned: manual(id) });
        return out;
    };

    window.InfraAppModel = {
        esc, clone, iconId, iconURL, assetURL, iconLabelKey, sourceActions, sourceIdentity, virtualSlice, sourceState,
        visibleByCollapsed, isPlaced, validCIDR, validHTTPLink, iconIds: () => [...ICON_LIST],
        acceptsIcon, rememberIcon, versionedAsset, containerKinds: () => [...CONTAINER_KINDS], isContainer, powerLabel,
        edgeSegment, edgeCaption, edgeLabelClash, gridPositions, snapshotStamp,
        layoutSnapshot, applyLayoutPatch,
        CARD_W, CARD_H, GRID, MIN_ZOOM, MAX_ZOOM, REGION_W, REGION_H, REGION_PAD, REGION_MIN_W, REGION_MIN_H,
        clampZoom, zoomLevels, labelScale, toWorld, toScreen, zoomAt, fitView, viewCenter, viewFromCenter, snap, cardRect, intersects, containsPoint,
        normalizeRect, padRect, unionRects, freeSlot, regionRect, regionLayout, regionAt, dropParent, growRegions,
        spatialNeighbour, compactGrid, defaultEdgeKind, guestIndex, matchesFilters, filtersActive, buildScene,
        mergeIntent, layoutItems, rebaseLayout, createHistory, readLayoutPrefs, fill, countText, normalizeNode, minimapTransform, sceneFits, glyph,
        naturalCompare, isOpaqueName, addressesOf, isGenericDiscovery, compareSpecificity, relevanceIndex, discoveryRelevance, groupDiscoveries, bindableFor, linkSuggestions, infraCleanup, sourceLabels,
        TABLE_SORTS, tableComparator, tableRows, readTablePrefs, readInspectorPrefs, inspectorTab, withInspectorTab, readDismissed, listEntryText, draftDiff, formGroupFill, groupOfField,
        isMeshAsset, appTargetOptions, appLinkContext, placePath, isDescendant, attentionItems, APP_LINK_TARGETS,
        topologyLanes: () => [...TOPOLOGY_LANES], LANE_H, topologyLane, topologyLaneIndex, laneAt, laneBands, topologyLayout, topologyAutoPositions, topologyArrange,
    };
})();
