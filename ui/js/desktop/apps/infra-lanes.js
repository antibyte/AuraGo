// Infra topology lanes: the pure helpers behind the Topology view (spec section 6). Four fixed horizontal
// lanes, the lane of an asset, the deterministic auto layout and Arrange. Loaded right after infra-model.js
// (it reads window.InfraAppModel) and before every module that draws or moves Topology cards.
(function () {
    'use strict';

    const { GRID, snap, cardRect, freeSlot, isContainer, naturalCompare } = window.InfraAppModel;
    const finite = value => (value === null || value === undefined || value === '' || !Number.isFinite(Number(value)) ? null : Number(value));

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

    window.InfraAppLanes = {
        topologyLanes: () => [...TOPOLOGY_LANES], LANE_H, topologyLane, topologyLaneIndex, laneAt, laneBands, topologyLayout, topologyAutoPositions, topologyArrange,
    };
})();
