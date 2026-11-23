// Infra arrange: automatic layout of the Map (by connections, by type, compact grid), the
// container-aware planner that lays places out bottom-up, and grouping with a coloured frame.
// layoutCards and planLayout are pure (scripts/test-infra-ui.mjs runs them in a Node vm);
// create(app) wires them to the store as ONE undoable layout command per action.
(function () {
    'use strict';

    const CARD_W = 216, CARD_H = 64, GRID = 24, PAD = 48, PAD_TOP = 64, GAP = 72;
    const PITCH_X = CARD_W + 72, PITCH_Y = CARD_H + 40;
    const HALF_W = CARD_W / 2, HALF_H = CARD_H / 2;
    const PALETTE = ['slate', 'blue', 'cyan', 'teal', 'green', 'amber', 'orange', 'red', 'pink', 'violet'];
    const MODES = ['connections', 'type', 'grid'];
    const ROOT_ICONS = ['internet', 'modem-ont', 'firewall', 'router'];
    const NETWORK_ICONS = new Set(['router', 'firewall', 'modem-ont', 'switch', 'managed-switch', 'access-point', 'mesh-node', 'patch-panel', 'internet', 'lan', 'vlan', 'wlan']);

    const text = value => String(value ?? '');
    const compare = (a, b) => text(a.name).toLowerCase().localeCompare(text(b.name).toLowerCase()) || text(a.name).localeCompare(text(b.name)) || (text(a.id) < text(b.id) ? -1 : text(a.id) > text(b.id) ? 1 : 0);
    const gridUp = value => Math.ceil(value / GRID) * GRID;
    const snap = value => Math.round(value / GRID) * GRID;
    const cardsPerRow = count => Math.max(1, Math.ceil(Math.sqrt(count * 1.6)));
    const cell = (col, row) => ({ x: Math.round(HALF_W + col * PITCH_X), y: Math.round(HALF_H + row * PITCH_Y) });

    // gridCells puts items (already sorted) into rows of `columns`; returns Map(id -> position) and the row count.
    function gridCells(items, columns, rowOffset = 0, out = new Map()) {
        items.forEach((item, index) => out.set(item.id, cell(index % columns, rowOffset + Math.floor(index / columns))));
        return { out, rows: Math.ceil(items.length / columns) };
    }

    function layoutByType(items) {
        const category = item => (NETWORK_ICONS.has(item.icon) || item.kind === 'network' ? 0 : item.kind === 'device' ? 1 : item.kind === 'vm' ? 2 : item.kind === 'container' ? 3 : item.kind === 'service' ? 4 : 5);
        const bands = [[], [], [], [], [], []];
        for (const item of items) bands[category(item)].push(item);
        const out = new Map();
        let row = 0;
        for (const band of bands) {
            if (!band.length) continue;
            band.sort(compare);
            const { rows } = gridCells(band, Math.max(4, Math.ceil(Math.sqrt(band.length * 1.6))), row, out);
            row += rows + 0.5;
        }
        return out;
    }

    function layoutByConnections(items, edges) {
        const byId = new Map(items.map(item => [item.id, item]));
        const adjacency = new Map(items.map(item => [item.id, new Set()]));
        for (const edge of edges || []) {
            if (edge.kind === 'runs_on' || edge.from === edge.to || !byId.has(edge.from) || !byId.has(edge.to)) continue;
            adjacency.get(edge.from).add(edge.to);
            adjacency.get(edge.to).add(edge.from);
        }
        const sorted = items.slice().sort(compare);
        const neighbours = id => [...adjacency.get(id)].map(other => byId.get(other)).sort(compare);
        const seen = new Set(), components = [], isolated = [];
        for (const item of sorted) {
            if (seen.has(item.id)) continue;
            if (!adjacency.get(item.id).size) { seen.add(item.id); isolated.push(item); continue; }
            const members = [], queue = [item];
            seen.add(item.id);
            while (queue.length) {
                const current = queue.shift();
                members.push(current);
                for (const next of neighbours(current.id)) if (!seen.has(next.id)) { seen.add(next.id); queue.push(next); }
            }
            components.push(members);
        }
        const rank = item => { const index = ROOT_ICONS.indexOf(item.icon); return index < 0 ? ROOT_ICONS.length : index; };
        const out = new Map();
        const laid = components.map(members => {
            const root = members.slice().sort((a, b) => rank(a) - rank(b) || adjacency.get(b.id).size - adjacency.get(a.id).size || compare(a, b))[0];
            const depth = new Map([[root.id, 0]]), queue = [root], layers = [[root]];
            while (queue.length) {
                const current = queue.shift();
                for (const next of neighbours(current.id)) {
                    if (depth.has(next.id)) continue;
                    depth.set(next.id, depth.get(current.id) + 1);
                    (layers[depth.get(next.id)] = layers[depth.get(next.id)] || []).push(next);
                    queue.push(next);
                }
            }
            const position = new Map();
            const reindex = () => layers.forEach(layer => layer.forEach((item, index) => position.set(item.id, index)));
            reindex();
            const sweep = (layer, toward) => {
                const key = new Map(layer.map(item => {
                    const near = [...adjacency.get(item.id)].filter(other => depth.get(other) === toward);
                    return [item.id, near.length ? near.reduce((sum, other) => sum + position.get(other), 0) / near.length : position.get(item.id)];
                }));
                layer.sort((a, b) => key.get(a.id) - key.get(b.id) || position.get(a.id) - position.get(b.id) || compare(a, b));
                layer.forEach((item, index) => position.set(item.id, index));
            };
            for (let pass = 0; pass < 3; pass += 1) {
                for (let level = 1; level < layers.length; level += 1) sweep(layers[level], level - 1);
                for (let level = layers.length - 2; level >= 0; level -= 1) sweep(layers[level], level + 1);
            }
            for (let level = 1; level < layers.length; level += 1) sweep(layers[level], level - 1);
            return { layers, rows: Math.max(...layers.map(layer => layer.length)), name: root };
        }).sort((a, b) => b.layers.reduce((sum, layer) => sum + layer.length, 0) - a.layers.reduce((sum, layer) => sum + layer.length, 0) || compare(a.name, b.name));
        let top = 0;
        for (const component of laid) {
            component.layers.forEach((layer, column) => {
                const shift = (component.rows - layer.length) / 2;
                layer.forEach((item, index) => out.set(item.id, cell(column, top + shift + index)));
            });
            top += component.rows + 1;
        }
        if (isolated.length) {
            isolated.sort((a, b) => text(a.kind).localeCompare(text(b.kind)) || compare(a, b));
            gridCells(isolated, cardsPerRow(isolated.length), laid.length ? top : 0, out);
        }
        return out;
    }

    // layoutCards: Map(id -> {x, y}) card centres; the box of all cards starts at (0, 0).
    function layoutCards(items, edges, { mode = 'connections' } = {}) {
        const list = (items || []).filter(item => item && item.id !== undefined);
        if (!list.length) return new Map();
        if (mode === 'type') return layoutByType(list);
        if (mode === 'grid') return gridCells(list.slice().sort(compare), cardsPerRow(list.length)).out;
        return layoutByConnections(list, edges);
    }

    // shelfPack puts blocks {w, h} left to right in rows no wider than `limit`; returns the offsets and the box.
    function shelfPack(blocks, limit) {
        let x = 0, y = 0, rowH = 0, width = 0;
        const placed = [];
        for (const block of blocks) {
            if (x > 0 && x + block.w > limit) { x = 0; y += rowH + GAP; rowH = 0; }
            placed.push({ block, x, y });
            x += block.w + GAP; rowH = Math.max(rowH, block.h);
            width = Math.max(width, x - GAP);
        }
        return { placed, w: width, h: y + rowH };
    }

    // planLayout lays a container (or the top level when containerId is '') out bottom-up. Result: Map(id -> intent)
    // with absolute world coordinates: cards {x, y, pinned}, regions {x, y, w, h, pinned}.
    function planLayout({ containerId = '', cards = [], regions = [], parentOf, nodeById, edges = [], mode = 'connections', origin = { x: 0, y: 0 } }) {
        const regionIds = new Set(regions.map(region => region.id));
        const holder = id => { const parent = parentOf(id) || ''; return regionIds.has(parent) && parent !== id ? parent : ''; };
        const cardsOf = new Map(), regionsOf = new Map();
        const add = (map, key, value) => { const list = map.get(key) || []; list.push(value); map.set(key, list); };
        for (const card of cards) add(cardsOf, holder(card.id), card.id);
        for (const region of regions) add(regionsOf, holder(region.id), region.id);
        const itemOf = id => { const node = nodeById(id) || {}; return { id, kind: node.kind, icon: node.icon, name: node.name || id }; };
        const visiting = new Set();
        // build returns the content of one container: card centres relative to its content box, child frames and the box size.
        function build(id) {
            visiting.add(id);
            const items = (cardsOf.get(id) || []).map(itemOf), own = new Set(items.map(item => item.id));
            const positions = layoutCards(items, edges.filter(edge => own.has(edge.from) && own.has(edge.to)), { mode });
            let cardsW = 0, cardsH = 0;
            for (const pos of positions.values()) { cardsW = Math.max(cardsW, pos.x + HALF_W); cardsH = Math.max(cardsH, pos.y + HALF_H); }
            cardsW = gridUp(cardsW); cardsH = gridUp(cardsH);
            const children = (regionsOf.get(id) || []).filter(child => !visiting.has(child)).map(child => ({ id: child, ...frame(child) })).sort((a, b) => compare(itemOf(a.id), itemOf(b.id)));
            let w = cardsW, h = cardsH;
            const offsets = [];
            if (children.length) {
                const area = children.reduce((sum, child) => sum + child.w * child.h, 0), widest = Math.max(...children.map(child => child.w));
                const below = cardsW > 3 * PITCH_X, anchor = items.length ? (below ? { x: 0, y: cardsH + GAP } : { x: cardsW + GAP, y: 0 }) : { x: 0, y: 0 };
                const limit = Math.max(widest, below ? cardsW : Math.ceil(Math.sqrt(area) * 1.4));
                const pack = shelfPack(children, limit);
                for (const entry of pack.placed) offsets.push({ child: entry.block, x: anchor.x + entry.x, y: anchor.y + entry.y });
                w = Math.max(w, anchor.x + pack.w); h = Math.max(h, anchor.y + pack.h);
            }
            visiting.delete(id);
            return { positions, children: offsets, w, h };
        }
        // frame is the stored rectangle of a place: its content box padded by 48 (64 above, so the header pill clears the cards).
        function frame(id) {
            const content = build(id);
            const w = Math.max(200, gridUp(content.w + 2 * PAD)), h = Math.max(140, gridUp(content.h + PAD + PAD_TOP));
            return { id, content, w, h };
        }
        const out = new Map();
        const emitContent = (content, x, y) => {
            for (const [cardId, pos] of content.positions) out.set(cardId, { x: Math.round(x + pos.x), y: Math.round(y + pos.y), pinned: true });
            for (const entry of content.children) emitFrame(entry.child, x + entry.x, y + entry.y);
        };
        const emitFrame = (block, x, y) => {
            out.set(block.id, { x, y, w: block.w, h: block.h, pinned: true });
            emitContent(block.content, x + PAD, y + PAD_TOP);
        };
        const start = { x: snap(origin.x || 0), y: snap(origin.y || 0) };
        if (containerId && regionIds.has(containerId)) emitFrame(frame(containerId), start.x, start.y);
        else emitContent(build(''), start.x, start.y);
        return out;
    }

    // freeFrame returns rect itself when it clears every obstacle (padded by 24), else the nearest free spot of the same size:
    // rings of 48 px steps around it (up to 60), closest first, right and down preferred on ties.
    function freeFrame(rect, obstacles, { step = 48, rings = 60, pad = 24 } = {}) {
        const hits = box => obstacles.some(o => box.x - pad < o.x + o.w && box.x + box.w + pad > o.x && box.y - pad < o.y + o.h && box.y + box.h + pad > o.y);
        if (!hits(rect)) return { ...rect };
        for (let ring = 1; ring <= rings; ring += 1) {
            const found = [];
            for (let dy = -ring; dy <= ring; dy += 1) for (let dx = -ring; dx <= ring; dx += 1) {
                if (Math.max(Math.abs(dx), Math.abs(dy)) !== ring) continue;
                const box = { ...rect, x: rect.x + dx * step, y: rect.y + dy * step };
                if (!hits(box)) found.push({ box, dist: Math.hypot(dx, dy), lean: dx + dy });
            }
            if (found.length) { found.sort((a, b) => a.dist - b.dist || b.lean - a.lean); return found[0].box; }
        }
        return { ...rect };
    }

    function create(app) {
        const { store, state, t, model } = app;
        const commands = () => window.InfraAppStore.commands;
        const label = (key, vars) => model.fill(t(key), vars);

        function modeLabel(mode) { return t({ connections: 'desktop.infra_arrange_by_connections', type: 'desktop.infra_arrange_by_type', grid: 'desktop.infra_arrange_grid' }[mode]); }
        const intentsFor = (after, view) => {
            const before = new Map();
            for (const id of after.keys()) { const current = store.currentIntent(id, view, false); if (current) before.set(id, current); else after.delete(id); }
            return before;
        };
        const edgesOf = () => (state.edges || []).map(edge => ({ from: edge.from, to: edge.to, kind: edge.kind }));
        const nodeInfo = id => store.nodeById(id);

        // arrange lays the whole map (or one place, or the given cards) out and saves it as one undoable layout command.
        function arrange(mode = 'connections', { placeId = '', ids = null } = {}) {
            if (!store.canWrite() || app.mapView() !== 'atlas' || !MODES.includes(mode)) return Promise.resolve(false);
            const view = 'atlas', scene = app.canvas.sceneData();
            if (!scene || !scene.cards) return Promise.resolve(false);
            const parentOf = id => store.effectiveParent(id, view);
            let after;
            if (ids && ids.length) {
                const chosen = ids.filter(id => scene.cardById.has(id));
                if (!chosen.length) return Promise.resolve(false);
                const box = model.unionRects(chosen.map(id => model.cardRect(scene.cardById.get(id))));
                const positions = layoutCards(chosen.map(id => { const node = nodeInfo(id) || {}; return { id, kind: node.kind, icon: node.icon, name: node.name || id }; }), edgesOf().filter(edge => chosen.includes(edge.from) && chosen.includes(edge.to)), { mode });
                after = new Map([...positions].map(([id, pos]) => [id, { x: Math.round(box.x + pos.x), y: Math.round(box.y + pos.y), pinned: true }]));
            } else {
                const rect = placeId ? scene.regionRects.get(placeId) : scene.bounds;
                if (placeId && !rect) return Promise.resolve(false);
                const subtree = placeId ? new Set([placeId]) : null;
                if (subtree) for (let grown = true; grown;) { grown = false; for (const item of [...scene.cards, ...scene.regions]) if (!subtree.has(item.id) && subtree.has(parentOf(item.id))) { subtree.add(item.id); grown = true; } }
                const inScope = item => !subtree || subtree.has(item.id);
                after = planLayout({
                    containerId: placeId, cards: scene.cards.filter(inScope), regions: scene.regions.filter(inScope), parentOf, nodeById: nodeInfo, edges: edgesOf(), mode,
                    origin: rect ? { x: rect.x, y: rect.y } : { x: 0, y: 0 },
                });
            }
            const before = intentsFor(after, view);
            if (!after.size) return Promise.resolve(false);
            const text = placeId ? label('desktop.infra_arranged_place', { name: nodeInfo(placeId)?.name || placeId, mode: modeLabel(mode) }) : ids && ids.length ? t('desktop.infra_arranged') : label('desktop.infra_arranged_map', { mode: modeLabel(mode) });
            const done = store.dispatch(commands().moveNodes(view, before, after, text));
            app.announce(text);
            if (!placeId && !(ids && ids.length)) { app.canvas.renderNow?.(); app.canvas.fit?.({ animate: true }); }
            return done;
        }

        // ---- grouping ---------------------------------------------------------------------------------
        const groupName = () => {
            const names = new Set(state.nodes.map(node => node.name));
            for (let n = 1; ; n += 1) { const name = label('desktop.infra_group_default_name', { n }); if (!names.has(name)) return name; }
        };
        const nextColor = () => { const used = state.nodes.filter(node => node.kind === 'group').length, cycle = PALETTE.slice(1); return cycle[used % cycle.length]; };
        const frameOf = rects => {
            const box = model.unionRects(rects);
            const x = Math.floor((box.x - PAD) / GRID) * GRID, y = Math.floor((box.y - PAD_TOP) / GRID) * GRID;
            return { x, y, w: gridUp(box.x + box.w + PAD - x), h: gridUp(box.y + box.h + PAD - y) };
        };

        async function group(ids) {
            const view = app.mapView();
            if (!store.canWrite() || view !== 'atlas') return false;
            const members = [...new Set(ids || [])].filter(id => { const node = store.nodeById(id); return node && !model.isContainer(node.kind); });
            if (!members.length) return false;
            const parent = new Set(members.map(id => store.effectiveParent(id, view))).size === 1 ? store.effectiveParent(members[0], view) : '';
            const body = {
                id: '', version: 0, kind: 'group', name: groupName(), icon: 'group', parent_id: parent,
                profile: { tags: [], interfaces: [], links: [], app_bindings: [], custom: {}, color: nextColor() }, placements: {}, sources: [],
            };
            let node;
            store.setBusy('create', true);
            try {
                const result = await store.request('/nodes', { method: 'POST', body });
                node = result.node || result;
                if (!node?.id) { await store.loadSnapshot(); return false; }
                node = store.replaceNode(node) || node;
            } catch (error) { await store.writeFailed(error); return false; } finally { store.setBusy('create', false); }
            const scene = app.canvas.sceneData(), drawn = members.filter(id => scene.cardById.has(id)), rest = members.filter(id => !drawn.includes(id));
            const work = [];
            let shifted = false;
            const groupLabel = model.countText(members.length, t('desktop.infra_grouped_one'), t('desktop.infra_grouped'));
            if (drawn.length && store.features.layoutParent) {
                const cardRects = drawn.map(id => model.cardRect(scene.cardById.get(id)));
                let rect = frameOf(cardRects), moved = new Map();
                shifted = false;
                const lineage = new Set();
                for (let up = parent; up && !lineage.has(up); up = store.effectiveParent(up, view)) lineage.add(up);
                const crowded = box => scene.cards.some(card => !drawn.includes(card.id) && model.intersects(box, model.cardRect(card)))
                    || scene.regions.some(region => !lineage.has(region.id) && region.id !== node.id && model.intersects(box, region.rect));
                if (crowded(rect)) {
                    const origin = model.unionRects(cardRects);
                    const positions = layoutCards(drawn.map(id => { const info = nodeInfo(id) || {}; return { id, kind: info.kind, icon: info.icon, name: info.name || id }; }), [], { mode: 'grid' });
                    moved = new Map([...positions].map(([id, pos]) => [id, { x: Math.round(origin.x + pos.x), y: Math.round(origin.y + pos.y) }]));
                    rect = frameOf([...moved.values()].map(model.cardRect));
                }
                if (crowded(rect)) {
                    // Still on top of foreign cards or places: carry the whole group, frame and members, to the nearest free spot.
                    const obstacles = [...scene.cards.filter(card => !drawn.includes(card.id)).map(model.cardRect), ...scene.regions.filter(region => !lineage.has(region.id) && region.id !== node.id).map(region => region.rect)];
                    const spot = freeFrame(rect, obstacles), dx = spot.x - rect.x, dy = spot.y - rect.y;
                    if (dx || dy) {
                        for (const id of drawn) { const pos = moved.get(id) || scene.cardById.get(id); moved.set(id, { x: Math.round(pos.x + dx), y: Math.round(pos.y + dy) }); }
                        rect = { ...rect, x: spot.x, y: spot.y };
                        shifted = true;
                    }
                }
                const before = new Map(), after = new Map();
                if (store.features.unplace) before.set(node.id, { unplace: true });
                after.set(node.id, { x: rect.x, y: rect.y, w: rect.w, h: rect.h, pinned: true, parent_id: parent });
                for (const id of drawn) {
                    const pos = moved.get(id) || scene.cardById.get(id);
                    before.set(id, store.currentIntent(id, view, true));
                    after.set(id, { x: Math.round(pos.x), y: Math.round(pos.y), pinned: true, parent_id: node.id });
                }
                const command = commands().reparent(view, before, after, groupLabel);
                if (!store.features.unplace) command.undoable = false;
                work.push(store.dispatch(command));
            } else rest.push(...drawn);
            if (rest.length) work.push(store.dispatch(commands().setParent(rest.map(id => ({ id, before: store.nodeById(id).parent_id || '', after: node.id })), groupLabel)));
            await Promise.all(work).catch(error => store.writeFailed(error));
            store.select([node.id]);
            if (shifted) app.canvas.centerOn?.(node.id, { animate: true });
            app.inspector.open(node.id, 'details');
            app.inspector.focusName?.();
            app.announce(groupLabel);
            return true;
        }

        async function ungroup(placeId) {
            const view = app.mapView(), node = store.nodeById(placeId);
            if (!store.canWrite() || view !== 'atlas' || node?.kind !== 'group') return false;
            const parent = store.effectiveParent(placeId, view), scene = app.canvas.sceneData();
            const kids = state.nodes.filter(item => item.id !== placeId && store.effectiveParent(item.id, view) === placeId).map(item => item.id);
            const layoutIds = kids.filter(id => store.features.layoutParent && (scene.cardById.has(id) || scene.regionRects.has(id))), rest = kids.filter(id => !layoutIds.includes(id));
            const text = model.countText(kids.length, t('desktop.infra_ungrouped_one'), t('desktop.infra_ungrouped'), { name: node.name });
            const work = [];
            if (layoutIds.length) {
                const before = new Map(), after = new Map();
                for (const id of layoutIds) {
                    const current = store.currentIntent(id, view, true);
                    if (!current) { rest.push(id); continue; }
                    before.set(id, current);
                    after.set(id, { ...current, parent_id: parent, pinned: true });
                }
                if (after.size) work.push(store.dispatch(commands().reparent(view, before, after, text)));
            }
            if (rest.length) work.push(store.dispatch(commands().setParent(rest.map(id => ({ id, before: placeId, after: parent })), text)));
            try { await Promise.all(work); } catch (error) { await store.writeFailed(error); return false; }
            if (!await store.whenLayoutIdle(view)) return false;
            const removed = await store.removeNodes([placeId]);
            if (removed.length) { store.select(kids.filter(id => store.nodeById(id))); app.announce(text); }
            return removed.length > 0;
        }

        function setColor(placeId, color) {
            const node = store.nodeById(placeId);
            if (!node || !store.canWrite() || (color && !PALETTE.includes(color))) return Promise.resolve(false);
            const profile = { ...(node.profile || {}) };
            if (color) profile.color = color; else delete profile.color;
            return store.updateNodeFields(placeId, { profile });
        }

        // groupSelection / ungroupSelection serve the keyboard: Ctrl+G and Ctrl+Shift+G.
        function groupSelection() { return group([...state.selection].filter(id => store.nodeById(id))); }
        function ungroupSelection() {
            const picked = [...state.selection].map(id => store.nodeById(id)).filter(node => node?.kind === 'group');
            return picked.length === 1 ? ungroup(picked[0].id) : Promise.resolve(false);
        }
        return { arrange, group, ungroup, setColor, groupSelection, ungroupSelection, modeLabel, PALETTE, MODES, dispose() {} };
    }

    window.InfraAppArrange = { layoutCards, planLayout, freeFrame, PALETTE, MODES, create };
})();
