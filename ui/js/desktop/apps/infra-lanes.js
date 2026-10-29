// Infra topology lanes: the pure helpers behind the Topology view (spec section 6). Four fixed horizontal
// lanes, the lane of an asset, the deterministic auto layout and the connection paths. Loaded right after infra-model.js
// (it reads window.InfraAppModel) and before every module that draws or moves Topology cards.
(function () {
    'use strict';

    const { CARD_W, CARD_H, snap, cardRect, freeSlot, isContainer, naturalCompare, edgeSegment, edgeCurve } = window.InfraAppModel;
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
    // ---- connection paths (A19, A28) -----------------------------------------------------------------------
    // topologyPaths draws every connection of the Topology view. A connection between two lanes is a cubic curve
    // from the bottom edge of the upper card to the top edge of the lower one with vertical tangents (control
    // offset max(48, dy / 2)). A connection inside one lane is the model.edgeCurve spline between the facing sides (a
    // straight line when both cards sit on one row) while no other card is in the straight segment's way; otherwise
    // (A28) it is an arc above the row: it leaves the top edge of both cards and rises clamp(24 + 0.12 dx, 32, 56) px,
    // so it never runs behind a card in between. A card is in the way when
    // its box, padded by 4 px, touches the straight segment, wherever it sits (pinned cards can be anywhere).
    // Connections that share a card edge spread over the middle 60% of it in equal slots. On a top edge the arcs to
    // the left come first, then the curves arriving from the lane above, then the arcs to the right, each group
    // ordered by the other end's x (then its id, then the connection's id), so the answer never depends on the input
    // order and an arc never crosses a curve that arrives on the same card. A connection whose cards leave no room
    // (dy under 16) for a lane curve takes the edgeCurve spline too; `curve` marks the slotted lane curves and arcs.
    // lines are {id, from, to} with card ids; cardAt(id) is {x, y} (the centre) or nothing; ids lists every drawn card (the obstacles; without it nothing is in the way). The answer
    // maps a connection id to {curve, arc, d, x1, y1, x2, y2, mx, my}: the ends of the path and its label anchor,
    // which is the midpoint of the path (the A5 clash rule). The same path serves the visible line, its hit area and
    // the selection highlight.
    const SPREAD = 0.6, MIN_CONTROL = 48, MIN_DROP = 16, BLOCK_PAD = 4, ARC_MIN = 32, ARC_MAX = 56;
    const tenth = value => Math.round(value * 10) / 10;
    // blocks: does the segment touch the box of a card centred on c (padded)? Slab test, so a horizontal line is no special case.
    const blocks = (seg, c) => {
        const box = [[seg.x1, seg.x2 - seg.x1, c.x - CARD_W / 2 - BLOCK_PAD, c.x + CARD_W / 2 + BLOCK_PAD], [seg.y1, seg.y2 - seg.y1, c.y - CARD_H / 2 - BLOCK_PAD, c.y + CARD_H / 2 + BLOCK_PAD]];
        let from = 0, to = 1;
        for (const [start, delta, low, high] of box) {
            if (delta === 0) { if (start < low || start > high) return false; continue; }
            const a = (low - start) / delta, b = (high - start) / delta;
            from = Math.max(from, Math.min(a, b)); to = Math.min(to, Math.max(a, b));
            if (from > to) return false;
        }
        return true;
    };
    const topologyPaths = (lines, cardAt, ids) => {
        const out = new Map(), half = { w: CARD_W / 2, h: CARD_H / 2 }, pending = [], edges = new Map();
        const obstacles = [];
        for (const id of ids || []) { const card = cardAt(id); if (card) obstacles.push({ id, x: card.x, y: card.y }); }
        const join = (key, entry) => { const list = edges.get(key); if (list) list.push(entry); else edges.set(key, [entry]); };
        const straight = (line, a, b) => { const seg = edgeCurve(a, b, half, half); out.set(line.id, { curve: false, arc: false, d: seg.d, x1: seg.x1, y1: seg.y1, x2: seg.x2, y2: seg.y2, mx: seg.mx, my: seg.my }); };
        for (const line of lines || []) {
            const a = cardAt(line.from), b = cardAt(line.to);
            if (!a || !b || line.from === line.to) continue;
            const la = laneAt(a.y), lb = laneAt(b.y);
            const upper = la <= lb ? { id: line.from, card: a } : { id: line.to, card: b }, lower = upper.id === line.from ? { id: line.to, card: b } : { id: line.from, card: a };
            let first = upper, second = lower, arc = false;
            if (la === lb) {
                const seg = edgeSegment(a, b, half, half);
                if (!obstacles.some(card => card.id !== line.from && card.id !== line.to && blocks(seg, card))) { straight(line, a, b); continue; }
                // an arc always runs from its left card to its right one
                arc = true;
                [first, second] = a.x < b.x || (a.x === b.x && String(line.from) <= String(line.to)) ? [{ id: line.from, card: a }, { id: line.to, card: b }] : [{ id: line.to, card: b }, { id: line.from, card: a }];
            } else if (lower.card.y - half.h - (upper.card.y + half.h) < MIN_DROP) {
                straight(line, a, b);
                continue;
            }
            const entry = { line, arc, ends: [first, second] };
            join(`${first.id}|${arc ? 'top' : 'bottom'}`, { entry, end: 0 }); join(`${second.id}|top`, { entry, end: 1 });
            pending.push(entry);
        }
        // slot order on one card edge: arcs to the left, arrivals from above, arcs to the right; then the other end's x
        const groupOf = (item, other) => (item.entry.arc ? (other.card.x < item.entry.ends[item.end].card.x ? -1 : 1) : 0);
        for (const list of edges.values()) {
            const keyed = list.map(item => { const other = item.entry.ends[1 - item.end]; return { item, other, group: groupOf(item, other) }; });
            keyed.sort((p, q) => p.group - q.group || p.other.card.x - q.other.card.x || naturalCompare(p.other.id, q.other.id) || naturalCompare(p.item.entry.line.id, q.item.entry.line.id));
            keyed.forEach(({ item }, i) => {
                const offsets = item.entry.offsets || (item.entry.offsets = [0, 0]);
                offsets[item.end] = tenth(((i + 0.5) / keyed.length - 0.5) * SPREAD * CARD_W);
            });
        }
        for (const { line, arc, ends, offsets } of pending) {
            const [first, second] = ends, x1 = tenth(first.card.x + offsets[0]), x2 = tenth(second.card.x + offsets[1]);
            if (arc) {
                const y1 = first.card.y - half.h, y2 = second.card.y - half.h, rise = Math.min(ARC_MAX, Math.max(ARC_MIN, 24 + 0.12 * Math.abs(second.card.x - first.card.x)));
                out.set(line.id, { curve: true, arc: true, d: `M${x1} ${tenth(y1)}C${x1} ${tenth(y1 - rise)} ${x2} ${tenth(y2 - rise)} ${x2} ${tenth(y2)}`, x1, y1, x2, y2, mx: (x1 + x2) / 2, my: (y1 + y2) / 2 - 0.75 * rise });
                continue;
            }
            const y1 = first.card.y + half.h, y2 = second.card.y - half.h, control = Math.max(MIN_CONTROL, (y2 - y1) / 2);
            out.set(line.id, { curve: true, arc: false, d: `M${x1} ${tenth(y1)}C${x1} ${tenth(y1 + control)} ${x2} ${tenth(y2 - control)} ${x2} ${tenth(y2)}`, x1, y1, x2, y2, mx: (x1 + x2) / 2, my: (y1 + y2) / 2 });
        }
        return out;
    };

    window.InfraAppLanes = {
        topologyLanes: () => [...TOPOLOGY_LANES], LANE_H, LANE_EXTENT, LANE_LABEL_INSET, topologyLane, topologyLaneIndex, laneAt, laneStripes, topologyPaths, topologyLayout, topologyAutoPositions,
    };
})();
