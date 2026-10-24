// Infra topology lanes: the pure helpers behind the Topology view (spec section 6). Four fixed horizontal
// lanes, the lane of an asset, the deterministic auto layout and the connection paths. Loaded right after infra-model.js
// (it reads window.InfraAppModel) and before every module that draws or moves Topology cards.
(function () {
    'use strict';

    const { CARD_W, CARD_H, snap, cardRect, freeSlot, isContainer, naturalCompare, edgeSegment } = window.InfraAppModel;
    const finite = value => (value === null || value === undefined || value === '' || !Number.isFinite(Number(value)) ? null : Number(value));

    // ---- topology lanes and auto layout (spec §6) --------------------------------
    // Four fixed horizontal world bands, LANE_H px tall, from y = 0. A card the
    // user placed (pinned) belongs to the lane its centre y is in (laneAt);
    // every other card belongs to topologyLane(node). Auto rows sit LANE_CELL_W
    // apart: the 216 px card plus one 24 px grid cell. A lane holds two rows: their
    // centres are LANE_ROW0 and LANE_ROW0 + LANE_ROW_H below the lane top (A17).
    const TOPOLOGY_LANES = ['internet', 'network', 'compute', 'clients'];
    const LANE_H = 384, LANE_ROW0 = 96, LANE_ROW_H = 144, LANE_CELL_W = 240;
    // The stripes of the lanes reach LANE_EXTENT world px to both sides of x = 0 (A18); their labels keep
    // LANE_LABEL_INSET screen px from the left edge of the viewport.
    const LANE_EXTENT = 50000, LANE_LABEL_INSET = 12;
    const PHYSICAL_EDGES = new Set(['ethernet', 'fiber', 'wifi']);
    // The ids are the model's icon catalog. Cameras, printers, TVs, phones, tablets, laptops, workstations, sensors
    // and the smart-home radios are not listed on purpose: they are the clients.
    const LANE_ICONS = {
        internet: ['internet', 'modem-ont'],
        network: ['router', 'firewall', 'vpn', 'managed-switch', 'switch', 'patch-panel', 'access-point', 'mesh-node', 'lan', 'vlan', 'wlan'],
        compute: ['rack-server', 'tower-server', 'mini-pc', 'sbc', 'nas', 'hypervisor', 'kubernetes', 'disk-shelf', 'storage-share', 'backup', 'database', 'nvr', 'ups', 'pdu'],
    };
    const LANE_ICON_INDEX = new Map(Object.entries(LANE_ICONS).flatMap(([lane, icons]) => icons.map((icon, rank) => [icon, { lane, rank }])));
    const LANE_ROLE_WORDS = [
        ['internet', ['modem', 'ont', 'isp', 'wan', 'dsl']],
        ['network', ['router', 'firewall', 'vpn', 'switch', 'gateway', 'mesh', 'ap', 'access point']],
        ['compute', ['server', 'nas', 'disk shelf', 'storage', 'database', 'nvr', 'recorder', 'rack', 'ups', 'pdu', 'hypervisor', 'proxmox', 'esxi', 'kubernetes', 'k8s']],
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
    // laneStripes: the four swimlanes as full-width world stripes, far wider than any content. They are
    // decoration: never part of the scene bounds, fit, minimap or hit tests.
    const laneStripes = () => TOPOLOGY_LANES.map((id, index) => ({ id, index, rect: { x: -LANE_EXTENT, y: index * LANE_H, w: LANE_EXTENT * 2, h: LANE_H } }));
    const laneRank = node => LANE_ICON_INDEX.get(node?.icon)?.rank ?? 99;
    // slotX is the centre x of card i of a lane of n cards: rows of up to `columns` cards, each row centred on x = 0.
    const columnsOf = (n, maxColumns) => Math.max(maxColumns, Math.ceil(n / 2));
    const slotX = (i, n, maxColumns) => {
        const columns = columnsOf(n, maxColumns), row = Math.floor(i / columns), inRow = Math.min(columns, n - row * columns);
        return snap(-(inRow - 1) * LANE_CELL_W / 2) + (i % columns) * LANE_CELL_W;
    };
    // barycentreOrder reorders the cards of every lane (A20) to cut crossings between lanes. members[i] is the
    // lane i list in connection order. One sweep from the top lane down keys a card by the mean x of its
    // neighbours in the lanes above; one sweep from the bottom lane up keys a card that has none by the mean x
    // of its neighbours below. A card whose neighbours are all in its own lane borrows the mean key of those;
    // a card without any connection goes last. Equal keys keep the connection order, so the result is
    // deterministic.
    const barycentreOrder = (members, adjacency, lane, order, maxColumns) => {
        const xs = new Map();
        const place = list => list.forEach((node, i) => xs.set(node.id, slotX(i, list.length, maxColumns)));
        const meanX = (node, wanted) => {
            let sum = 0, count = 0;
            for (const next of adjacency.get(node.id)) if (wanted(lane.get(next.id))) { sum += xs.get(next.id); count += 1; }
            return count ? Math.round(sum / count * 1000) / 1000 : null;
        };
        const sortLane = (list, directKey) => {
            const own = new Map(list.map(node => [node.id, directKey(node)])), keys = new Map();
            for (const node of list) {
                let key = own.get(node.id);
                if (key === null) {
                    let sum = 0, count = 0;
                    for (const next of adjacency.get(node.id)) if (lane.get(next.id) === lane.get(node.id) && own.get(next.id) != null) { sum += own.get(next.id); count += 1; }
                    key = count ? Math.round(sum / count * 1000) / 1000 : null;
                }
                keys.set(node.id, key);
            }
            list.sort((a, b) => {
                const ka = keys.get(a.id), kb = keys.get(b.id);
                if (ka === null || kb === null) return ka === kb ? order.get(a.id) - order.get(b.id) : (ka === null ? 1 : -1);
                return ka - kb || order.get(a.id) - order.get(b.id);
            });
            place(list);
        };
        members.forEach(place);
        const above = new Map();
        for (let index = 1; index < members.length; index += 1) {
            sortLane(members[index], node => { const key = meanX(node, other => other < index); above.set(node.id, key); return key; });
        }
        for (let index = members.length - 2; index >= 0; index -= 1) {
            sortLane(members[index], node => (above.get(node.id) ?? null) !== null ? meanX(node, other => other < index) : meanX(node, other => other > index));
        }
    };
    // topologyLayout puts nodes into their lanes (laneOf → index) in connection
    // order: a breadth-first walk over Ethernet, fiber and Wi-Fi edges, started
    // from the top lane, numbers the nodes, so parents come before children;
    // barycentreOrder then reorders each lane against its neighbours (ordering
    // 'connection' keeps the plain connection order).
    // A lane holds up to two rows of maxColumns cards; a lane that would need a third row widens its
    // rows instead (columns = max(maxColumns, ceil(n / 2)), A17). Every row is centred on x = 0.
    const topologyLayout = (nodes, edges, { laneOf = node => topologyLaneIndex(topologyLane(node)), maxColumns = 8, ordering = 'barycentre' } = {}) => {
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
        const members = TOPOLOGY_LANES.map(() => []);
        for (const node of list) members[lane.get(node.id)].push(node);
        for (const inLane of members) inLane.sort((a, b) => order.get(a.id) - order.get(b.id));
        if (ordering === 'barycentre') barycentreOrder(members, adjacency, lane, order, maxColumns);
        const out = new Map();
        members.forEach((inLane, index) => {
            inLane.forEach((node, i) => {
                const row = Math.floor(i / columnsOf(inLane.length, maxColumns));
                out.set(node.id, { x: slotX(i, inLane.length, maxColumns), y: index * LANE_H + LANE_ROW0 + row * LANE_ROW_H, pinned: false });
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
    // ---- connection paths (A19) ----------------------------------------------------------------------------
    // topologyPaths draws every connection of the Topology view. A connection between two lanes is a cubic curve
    // from the bottom edge of the upper card to the top edge of the lower one with vertical tangents (control
    // offset max(48, dy / 2)). Connections that share a card edge spread over the middle 60% of it in equal slots,
    // ordered by the other end's x (then its id, then the connection's id), so the answer never depends on the
    // input order. A connection inside one lane stays a straight line between the facing sides, and so does one
    // whose cards leave no room (dy under 16) for a curve. lines are {id, from, to} with card ids; cardAt(id) is
    // {x, y} (the centre) or nothing. The answer maps a connection id to {curve, d, x1, y1, x2, y2}: the ends of
    // the path, whose midpoint is also the midpoint of the curve (the label anchor of the A5 clash rule). The
    // same path serves the visible line, its hit area and the selection highlight.
    const SPREAD = 0.6, MIN_CONTROL = 48, MIN_DROP = 16;
    const tenth = value => Math.round(value * 10) / 10;
    const topologyPaths = (lines, cardAt) => {
        const out = new Map(), half = { w: CARD_W / 2, h: CARD_H / 2 }, pending = [], bottoms = new Map(), tops = new Map();
        const join = (map, key, entry) => { const list = map.get(key); if (list) list.push(entry); else map.set(key, [entry]); };
        for (const line of lines || []) {
            const a = cardAt(line.from), b = cardAt(line.to);
            if (!a || !b || line.from === line.to) continue;
            const la = laneAt(a.y), lb = laneAt(b.y);
            const upper = la <= lb ? { id: line.from, card: a } : { id: line.to, card: b }, lower = upper.id === line.from ? { id: line.to, card: b } : { id: line.from, card: a };
            if (la === lb || lower.card.y - half.h - (upper.card.y + half.h) < MIN_DROP) {
                const seg = edgeSegment(a, b, half, half);
                out.set(line.id, { curve: false, d: `M${tenth(seg.x1)} ${tenth(seg.y1)}L${tenth(seg.x2)} ${tenth(seg.y2)}`, x1: seg.x1, y1: seg.y1, x2: seg.x2, y2: seg.y2 });
                continue;
            }
            const entry = { line, upper, lower };
            join(bottoms, upper.id, entry); join(tops, lower.id, entry);
            pending.push(entry);
        }
        const offsets = new Map(); // connection id → [start offset, end offset]
        const spread = (map, slot, otherOf) => {
            for (const list of map.values()) {
                list.sort((p, q) => otherOf(p).card.x - otherOf(q).card.x || naturalCompare(otherOf(p).id, otherOf(q).id) || naturalCompare(p.line.id, q.line.id));
                list.forEach((entry, i) => {
                    const pair = offsets.get(entry.line.id) || [0, 0];
                    pair[slot] = tenth(((i + 0.5) / list.length - 0.5) * SPREAD * CARD_W);
                    offsets.set(entry.line.id, pair);
                });
            }
        };
        spread(bottoms, 0, entry => entry.lower);
        spread(tops, 1, entry => entry.upper);
        for (const { line, upper, lower } of pending) {
            const [from, to] = offsets.get(line.id);
            const x1 = tenth(upper.card.x + from), y1 = upper.card.y + half.h, x2 = tenth(lower.card.x + to), y2 = lower.card.y - half.h;
            const control = Math.max(MIN_CONTROL, (y2 - y1) / 2);
            out.set(line.id, { curve: true, d: `M${x1} ${tenth(y1)}C${x1} ${tenth(y1 + control)} ${x2} ${tenth(y2 - control)} ${x2} ${tenth(y2)}`, x1, y1, x2, y2 });
        }
        return out;
    };

    window.InfraAppLanes = {
        topologyLanes: () => [...TOPOLOGY_LANES], LANE_H, LANE_EXTENT, LANE_LABEL_INSET, topologyLane, topologyLaneIndex, laneAt, laneStripes, topologyPaths, topologyLayout, topologyAutoPositions,
    };
})();
