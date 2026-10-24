import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const files = ['infra-model.js', 'infra-store.js', 'infra-canvas.js', 'infra-gestures.js', 'infra-library.js', 'infra-inspector.js', 'infra-table.js', 'infra-dialogs.js', 'infra.js'];
for (const file of files) {
    const text = fs.readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8');
    const lines = text.split('\n').length;
    assert.ok(lines < 1100, `${file} has ${lines} lines, want fewer than 1100`);
}
const source = files.map(file => fs.readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8')).join('\n');
const window = { BUILD_VERSION: 'test-build', btoa: value => Buffer.from(value, 'binary').toString('base64') };
vm.runInNewContext(source, { window, console, setTimeout, clearTimeout, DOMException, TextEncoder, Map, Set, JSON, Math, Date, URL, AbortController });
const model = window.InfraAppModel;
const plain = value => JSON.parse(JSON.stringify(value));

assert.equal(model.iconURL('rack-server'), '/img/infra/rack-server.webp?v=test-build');
assert.equal(model.iconURL('../secret'), '/img/infra/generic-device.webp?v=test-build');
assert.deepEqual(Array.from(model.sourceActions('docker')), ['start', 'stop', 'restart']);
assert.deepEqual(Array.from(model.sourceActions('proxmox')), ['start', 'shutdown', 'reboot']);
assert.deepEqual(Array.from(model.sourceActions('fritzbox')), ['wake']);
assert.equal(model.sourceIdentity({ kind: 'docker', instance: 'engine-a', key: 'protected-container' }), 'infra_ZG9ja2VyAGVuZ2luZS1hAHByb3RlY3RlZC1jb250YWluZXI');
assert.equal(model.sourceState({ sources: [{ state: 'stale' }, { state: 'current' }] }), 'current');
assert.equal(model.sourceState({ sources: [{ state: 'removed' }] }), 'removed');
assert.equal(model.iconLabelKey('disk-shelf'), 'desktop.infra_icon_disk_shelf');
assert.equal(model.validCIDR('192.168.10.0/24'), true);
assert.equal(model.validCIDR('192.168.10.0/44'), false);
assert.equal(model.validHTTPLink('https://example.test/device'), true);
assert.equal(model.validHTTPLink('https://user:password@example.test/'), false);

const manifest = JSON.parse(fs.readFileSync(path.join(root, 'ui/img/infra/manifest.json'), 'utf8'));
const selectableIcons = manifest.icons.filter(item => item.family !== 'app');
assert.equal(selectableIcons.length, 64, 'manifest should contain the 64 selectable miniatures');
assert.deepEqual(new Set(selectableIcons.map(item => item.id)), new Set(model.iconIds()));
for (const item of manifest.icons) {
    assert.equal(item.path, `/img/infra/${item.id}.webp`);
    const bytes = fs.readFileSync(path.join(root, 'ui', item.path.slice(1)));
    assert.equal(crypto.createHash('sha256').update(bytes).digest('hex'), item.sha256, `${item.id} miniature hash`);
}

for (const size of [25, 250, 1000]) {
    const rows = Array.from({ length: size }, (_, index) => ({ id: `node-${index}` }));
    const start = Math.floor(size / 2);
    const slice = model.virtualSlice(rows, start * 54, 320, 54, 5);
    assert.ok(slice.rows.length < size, `table with ${size} rows should be virtualized`);
    assert.equal(slice.rows[0].id, rows[slice.start].id);
    assert.equal(slice.before, slice.start * 54);
    assert.equal(slice.before + slice.rows.length * 54 + slice.after, size * 54);
}

const grouped = Array.from({ length: 1000 }, (_, index) => index < 20
    ? { id: `group-${index}`, kind: 'group' }
    : { id: `node-${index}`, kind: 'device', parent_id: `group-${Math.floor((index - 20) / 49)}` });
const collapsed = new Set(Array.from({ length: 20 }, (_, index) => `group-${index}`));
assert.equal(Array.from(model.visibleByCollapsed(grouped, collapsed)).length, 20);
assert.equal(Array.from(model.visibleByCollapsed(grouped, new Set())).length, 1000);

const placedNodes = [{ id: 'north', kind: 'device', placements: { atlas: { x: -330, y: 85 } } }, { id: 'far', kind: 'device', placements: { atlas: { x: 5200, y: 2910 } } }];
const unplacedNodes = Array.from({ length: 1000 }, (_, index) => ({ id: `unplaced-${index}`, kind: 'device', placements: {} }));
const worldOf = nodes => { const byId = new Map(nodes.map(node => [node.id, node])); return model.buildScene({ nodes, edges: [], view: 'atlas', placementOf: id => byId.get(id).placements.atlas, parentOf: () => '', guests: model.guestIndex(nodes, []) }).bounds; };
assert.deepEqual(plain(worldOf([...placedNodes, ...unplacedNodes])), plain(worldOf(placedNodes)), 'unplaced nodes never change the world bounds');
const world = worldOf(placedNodes);
assert.ok(world.x < -330 && world.x + world.w > 5200 && world.y + world.h > 2910, 'saved negative and far positions stay in the world');

const locales = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const required = new Set();
for (const match of source.matchAll(/t\('(desktop\.(?:app_infra|infra_[a-zA-Z0-9_]+))'/g)) required.add(match[1]);
for (const key of ['desktop.qc_target_selected', 'desktop.qc_no_connection']) required.add(key);
for (const icon of model.iconIds()) required.add(`desktop.infra_icon_${icon.replaceAll('-', '_')}`);
for (const key of ['desktop.infra_access_unavailable', 'desktop.infra_action_unavailable', 'desktop.infra_saved_newer_draft', 'desktop.infra_source_saved_newer_draft', 'desktop.infra_layout_pending_review', 'desktop.infra_position', 'desktop.infra_source_compare', 'desktop.infra_node_compare', 'desktop.infra_current_source', 'desktop.infra_current_inventory', 'desktop.infra_your_draft', 'desktop.infra_no_field_differences', 'desktop.infra_reapply_draft', 'desktop.infra_reapply_ready', 'desktop.infra_node_conflict_review', 'desktop.infra_target_unavailable', 'desktop.infra_action_outcome_unknown', 'desktop.infra_pve_tags_hint']) required.add(key);
for (const [family, values] of Object.entries({
    infra_kind: ['device', 'vm', 'container', 'service', 'network', 'site', 'room', 'rack', 'group'],
    infra_source: ['registry', 'docker', 'proxmox', 'fritzbox'],
    infra_edge: ['ethernet', 'fiber', 'wifi', 'member_of', 'runs_on', 'depends_on', 'powered_by'],
    infra_state: ['current', 'stale', 'unknown', 'removed', 'manual'],
    infra_power: ['running', 'stopped', 'paused', 'unknown', 'filter', 'all', 'none'],
    infra_action: ['start', 'stop', 'restart', 'shutdown', 'reboot', 'wake'],
    infra_action_help: ['start', 'stop', 'restart', 'shutdown', 'reboot', 'wake'],
    infra_action_status: ['running', 'accepted', 'completed', 'failed', 'unknown'],
    infra_field: ['role', 'vendor', 'model', 'serial', 'os', 'firmware', 'cpu', 'ram', 'storage', 'rack_position', 'ip_address', 'mac_address', 'tags', 'notes', 'interfaces', 'links', 'custom', 'name', 'type', 'protocol', 'port', 'username', 'credential_id', 'description', 'friendly_name', 'hostname', 'cidr', 'vlan'],
    infra_protocol: ['none', 'ssh', 'vnc'],
    infra_reason: ['source_edit_disabled', 'source_readonly', 'managed_container', 'control_disabled', 'source_disabled', 'source_stale', 'source_policy'],
    infra_error: ['disabled', 'readonly', 'unavailable', 'invalid_source', 'invalid_request', 'invalid_action', 'invalid_node_id', 'invalid_candidate_id', 'invalid_edge_id', 'invalid_version', 'not_found', 'source_linked', 'has_children', 'edge_exists', 'infra_error'],
})) for (const value of values) required.add(`desktop.${family}_${value}`);

for (const locale of locales) {
    const dictionary = JSON.parse(fs.readFileSync(path.join(root, `ui/lang/desktop/${locale}.json`), 'utf8'));
    const missing = [...required].filter(key => typeof dictionary[key] !== 'string');
    assert.deepEqual(missing, [], `${locale}.json is missing keys: ${missing.join(', ')}`);
}

// A locale must not be filled with the English source text. Short technical
// labels may be identical by design; prose (spaces, more than 20 characters)
// may not.
const english = JSON.parse(fs.readFileSync(path.join(root, 'ui/lang/desktop/en.json'), 'utf8'));
for (const locale of locales) {
    if (locale === 'en') continue;
    const dictionary = JSON.parse(fs.readFileSync(path.join(root, `ui/lang/desktop/${locale}.json`), 'utf8'));
    const untranslated = Object.keys(english)
        .filter(key => key.startsWith('desktop.infra') && typeof english[key] === 'string')
        .filter(key => dictionary[key] === english[key])
        .filter(key => /\s/.test(english[key]) && english[key].length > 20);
    assert.deepEqual(untranslated, [], `${locale}.json keeps English prose for: ${untranslated.join(', ')}`);
}

assert.equal(model.powerLabel('stopped'), 'desktop.infra_power_stopped');
assert.equal(model.powerLabel(''), '');
assert.equal(model.powerLabel('on'), '');
assert.deepEqual(Array.from(model.containerKinds()), ['site', 'room', 'rack', 'group']);
assert.equal(model.isContainer('room'), true);
assert.equal(model.isContainer('device'), false);

const room = { id: 'room', kind: 'room', parent_id: '' };
const rack = { id: 'rack', kind: 'rack', parent_id: 'room' };
const host = { id: 'host', kind: 'device', parent_id: 'rack' };
assert.equal(model.visibleByCollapsed([room, rack, host], new Set(['room'])).length, 1);

assert.deepEqual(plain(model.edgeSegment({ x: 0, y: 0 }, { x: 400, y: 0 })), { x1: 92, y1: 0, x2: 308, y2: 0 }, 'connections start and end at the card border');
assert.deepEqual(plain(model.edgeSegment({ x: 0, y: 0 }, { x: 0, y: 300 })), { x1: 0, y1: 30, x2: 0, y2: 270 });
assert.deepEqual(plain(model.edgeSegment({ x: 0, y: 0 }, { x: 100, y: 0 })), { x1: 0, y1: 0, x2: 100, y2: 0 }, 'overlapping cards keep the centre line');
assert.equal(model.edgeSegment({ x: 0, y: 0 }, { x: 1000, y: 0 }, undefined, { w: 300, h: 100 }).x2, 700, 'a region end stops at its own border');
assert.equal(model.edgeCaption({ from_port: 'ge-0/0/1', to_port: 'eth0', speed: '1G' }), 'ge-0/0/1 → eth0 · 1G');

const topology = model.topologyPositions([
    { id: 'lan', kind: 'network', name: 'LAN' },
    { id: 'nas', kind: 'device', name: 'NAS' },
    { id: 'backup', kind: 'service', name: 'Backup' },
], [{ x: 110, y: 280 }]);
const placedAt = (id, x, y) => {
    const position = topology.get(id);
    assert.equal(position.x, x);
    assert.equal(position.y, y);
    assert.equal(position.pinned, false);
};
placedAt('lan', 110, 100);
placedAt('nas', 330, 280);
placedAt('backup', 110, 640);

const base = { nodes: [{ id: 'n', version: 1, kind: 'device', name: 'NAS', power: 'running', placements: { atlas: { x: 1, y: 2 } }, sources: [] }], edges: [], sources: [{ kind: 'docker', instance: 'engine', key: 'abc', state: 'current', observed: { power: 'running' }, fetched_at: '2026-01-01T00:00:00Z' }], caps: { write: true, source_edit: false, control: false } };
const same = model.snapshotStamp(base.nodes, base.edges, base.sources, base.caps);
const later = model.snapshotStamp(base.nodes, base.edges, [{ ...base.sources[0], fetched_at: '2026-01-02T00:00:00Z' }], base.caps);
assert.equal(same, later);
const moved = model.snapshotStamp([{ ...base.nodes[0], placements: { atlas: { x: 9, y: 2 } } }], base.edges, base.sources, base.caps);
const powered = model.snapshotStamp([{ ...base.nodes[0], power: 'stopped' }], base.edges, base.sources, base.caps);
assert.notEqual(same, moved);
assert.notEqual(same, powered);

// PUT /layout answers with the nodes it changed; that is a patch, never a snapshot.
assert.equal(model.layoutSnapshot({ nodes: [{ id: 'a', placements: {} }] }), null);
assert.equal(model.layoutSnapshot({ snapshot: { nodes: [] } }).nodes.length, 0);
const patchedNodes = [{ id: 'a', version: 1, placements: {} }, { id: 'b', version: 1, placements: {} }];
model.applyLayoutPatch(patchedNodes, [{ id: 'a', version: 2, x: 10, y: 20, pinned: true }], 'atlas');
assert.equal(patchedNodes.length, 2);
assert.equal(patchedNodes[0].version, 2);
assert.equal(patchedNodes[0].placements.atlas.x, 10);
assert.equal(patchedNodes[0].placements.atlas.y, 20);
assert.equal(patchedNodes[0].placements.atlas.pinned, true);
assert.equal(patchedNodes[1].version, 1);
assert.equal(patchedNodes[1].placements.atlas, undefined);

// ---- canvas viewport --------------------------------------------------------
{
    const view = { x: 40, y: -20, zoom: 0.5 };
    const world = model.toWorld(view, { x: 140, y: 80 });
    assert.deepEqual(plain(world), { x: 200, y: 200 });
    assert.deepEqual(plain(model.toScreen(view, world)), { x: 140, y: 80 });
    const zoomed = model.zoomAt(view, 1, { x: 140, y: 80 });
    assert.equal(zoomed.zoom, 1);
    assert.deepEqual(plain(model.toWorld(zoomed, { x: 140, y: 80 })), { x: 200, y: 200 }, 'zoomAt keeps the world point under the pointer');
    assert.equal(model.clampZoom(9), 2);
    assert.equal(model.clampZoom(0.01), 0.25);
    assert.equal(model.clampZoom(NaN), 1);
    const fitted = model.fitView({ x: 0, y: 0, w: 400, h: 200 }, { w: 1000, h: 800 });
    assert.deepEqual(plain(fitted), { x: 300, y: 300, zoom: 1 }, 'fit never zooms beyond 100 %');
    assert.equal(model.fitView({ x: 0, y: 0, w: 1808, h: 200 }, { w: 1000, h: 800 }).zoom, 0.5, 'fit keeps a 48 px margin');
    const stored = model.viewCenter({ x: 100, y: 50, zoom: 2 }, { w: 800, h: 600 });
    assert.deepEqual(plain(model.viewFromCenter(stored, { w: 800, h: 600 })), { x: 100, y: 50, zoom: 2 });
    assert.equal(model.viewFromCenter({ cx: 'x' }, { w: 1, h: 1 }), null);
    assert.equal(model.snap(35), 24);
    assert.equal(model.snap(37), 48);
    assert.equal(model.snap(-13), -24);
}

// ---- geometry, regions and drops --------------------------------------------
{
    const a = model.cardRect({ x: 0, y: 0 });
    assert.deepEqual(plain(a), { x: -92, y: -30, w: 184, h: 60 });
    const slot = model.freeSlot({ x: 0, y: 0 }, [a]);
    assert.ok(!model.intersects(model.cardRect(slot), a), 'the nudged card no longer overlaps');
    assert.equal(Math.hypot(slot.x, slot.y), 72, 'the nearest free slot is one card height plus the gap away');
    assert.ok(Number.isInteger(slot.x / 24) && Number.isInteger(slot.y / 24), 'a nudged card lands on the grid');
    assert.deepEqual(plain(model.freeSlot({ x: 500, y: 501 }, [a])), { x: 500, y: 501 }, 'a free drop keeps its exact point');
    assert.deepEqual(plain(model.regionRect({ x: 0, y: 0 })), { x: 0, y: 0, w: 360, h: 240 }, 'a place without size gets 360×240');
    assert.deepEqual(plain(model.regionRect({ x: 0, y: 0, w: 200, h: 140 }, [model.cardRect({ x: 400, y: 100 })])), { x: 0, y: 0, w: 540, h: 178 }, 'children grow the region by the 48 px frame padding');
    const placements = { site: { x: 0, y: 0, w: 200, h: 140 }, rack: { x: 300, y: 300, w: 200, h: 140 } };
    const parentOf = id => ({ rack: 'site' })[id] || '';
    const layout = model.regionLayout(['site', 'rack'], { parentOf, placementOf: id => placements[id], cardRectsByParent: new Map([['rack', [model.cardRect({ x: 400, y: 380 })]]]) });
    assert.deepEqual(plain(layout.rects.get('rack')), { x: 260, y: 300, w: 280, h: 158 });
    assert.deepEqual(plain(layout.rects.get('site')), { x: 0, y: 0, w: 588, h: 506 }, 'a nested place grows its parent');
    assert.equal(layout.depth.get('rack'), 1);
    assert.equal(model.regionAt({ x: 400, y: 380 }, layout.rects, layout.depth), 'rack', 'hit-testing returns the innermost region');
    assert.equal(model.regionAt({ x: 50, y: 50 }, layout.rects, layout.depth), 'site');
    assert.equal(model.regionAt({ x: 5000, y: 50 }, layout.rects, layout.depth), '');
    assert.equal(model.regionAt({ x: 400, y: 380 }, layout.rects, layout.depth, new Set(['rack'])), 'site', 'excluded regions are skipped');
    assert.deepEqual(plain(model.dropParent('', { x: 400, y: 380 }, layout.rects, layout.depth)), { parent_id: 'rack' });
    assert.deepEqual(plain(model.dropParent('rack', { x: 400, y: 380 }, layout.rects, layout.depth)), {}, 'staying in the same place leaves parent_id unchanged');
    assert.deepEqual(plain(model.dropParent('rack', { x: 5000, y: 5000 }, layout.rects, layout.depth)), { parent_id: '' }, 'leaving every region clears the parent');
    assert.deepEqual(plain(model.dropParent('hidden-room', { x: 5000, y: 5000 }, layout.rects, layout.depth)), {}, 'a parent that is not drawn here is never cleared');
    const growth = model.growRegions('rack', model.cardRect({ x: 700, y: 380 }), { ids: new Set(['site', 'rack']), parentOf, placementOf: id => placements[id] });
    assert.deepEqual(plain(growth.get('rack')), { x: 300, y: 300, w: 540, h: 158 });
    assert.deepEqual(plain(growth.get('site')), { x: 0, y: 0, w: 888, h: 506 }, 'growth propagates to the ancestors');
    const items = [{ id: 'c', x: 0, y: 0 }, { id: 'r', x: 220, y: 10 }, { id: 'far', x: 600, y: 0 }, { id: 'd', x: 0, y: 155 }, { id: 'diag', x: 150, y: 150 }];
    assert.equal(model.spatialNeighbour(items[0], items, 'right'), 'r');
    assert.equal(model.spatialNeighbour(items[0], items, 'down'), 'd');
    assert.equal(model.spatialNeighbour(items[0], items, 'left'), '');
    assert.deepEqual(plain(model.compactGrid(4, { x: 0, y: 0 })), [{ x: -96, y: -48 }, { x: 120, y: -48 }, { x: -96, y: 48 }, { x: 120, y: 48 }]);
    assert.equal(model.defaultEdgeKind({ kind: 'device', icon: 'router' }, { kind: 'device', icon: 'nas' }), 'ethernet');
    assert.equal(model.defaultEdgeKind({ kind: 'device', icon: 'access-point' }, { kind: 'device', icon: 'laptop' }), 'wifi');
    assert.equal(model.defaultEdgeKind({ kind: 'service', icon: 'web-service' }, { kind: 'device', icon: 'nas' }), 'depends_on');
    const mini = model.minimapTransform({ x: 0, y: 0, w: 1000, h: 500 }, { w: 200, h: 130 });
    assert.ok(Math.abs(mini.scale - 0.188) < 1e-9 && Math.abs(mini.x - 6) < 1e-9 && Math.abs(mini.y - 18) < 1e-9, 'the minimap centres the world');
}

// ---- guests, filters and the scene ------------------------------------------
{
    const guestNodes = [{ id: 'host', kind: 'device' }, { id: 'vm', kind: 'vm', host_id: 'host' }, { id: 'svc', kind: 'service' }, { id: 'ct', kind: 'container', host_id: 'gone' }, { id: 'dev', kind: 'device', host_id: 'host' }];
    const guests = model.guestIndex(guestNodes, [{ id: 'e1', kind: 'runs_on', from: 'svc', to: 'vm' }]);
    assert.equal(guests.hostOf.get('vm'), 'host', 'host_id wins');
    assert.equal(guests.hostOf.get('svc'), 'vm', 'without host_id a runs_on edge names the host');
    assert.equal(guests.hostOf.has('ct'), false, 'an unknown host leaves the guest hostless');
    assert.equal(guests.hostOf.has('dev'), false, 'devices are never guests');
    assert.deepEqual(plain(guests.guestsOf.get('host')), ['vm']);
    const textOf = node => String(node.name).toLocaleLowerCase();
    const nas = { name: 'Cold NAS', kind: 'device', power: 'running', sources: [{ kind: 'docker' }] };
    assert.equal(model.matchesFilters(nas, { query: 'cold' }, textOf), true);
    assert.equal(model.matchesFilters(nas, { query: 'warm' }, textOf), false);
    assert.equal(model.matchesFilters(nas, { kind: 'vm' }, textOf), false);
    assert.equal(model.matchesFilters(nas, { source: 'docker', power: 'running' }, textOf), true);
    assert.equal(model.matchesFilters(nas, { power: 'none' }, textOf), false);
    assert.equal(model.filtersActive({ query: '  ' }), false);
    assert.equal(model.filtersActive({ kind: 'vm' }), true);
    const sceneNodes = [
        { id: 'room', kind: 'room', placements: { atlas: { x: 0, y: 0, w: 600, h: 300 } } },
        { id: 'host', kind: 'device', parent_id: 'room', placements: { atlas: { x: 200, y: 150 } } },
        { id: 'svc', kind: 'service', placements: { atlas: { x: 900, y: 900 } } },
        { id: 'nas', kind: 'device', placements: { atlas: { x: 900, y: 150 } } },
        { id: 'loose', kind: 'device', placements: {} },
    ];
    const sceneEdges = [{ id: 'r', kind: 'runs_on', from: 'svc', to: 'host' }, { id: 'l', kind: 'ethernet', from: 'svc', to: 'nas' }];
    const byId = new Map(sceneNodes.map(node => [node.id, node]));
    const input = { nodes: sceneNodes, edges: sceneEdges, placementOf: id => byId.get(id).placements.atlas, parentOf: id => byId.get(id)?.parent_id || '', guests: model.guestIndex(sceneNodes, sceneEdges) };
    const scene = model.buildScene({ ...input, view: 'atlas' });
    assert.deepEqual(plain(scene.cards.map(card => card.id)), ['host', 'nas'], 'guests and unplaced assets are not cards; places are regions');
    assert.deepEqual(plain(scene.cards[0].guests), ['svc']);
    assert.deepEqual(plain(scene.regions.map(region => region.id)), ['room']);
    assert.deepEqual(plain(scene.lines.map(line => [line.id, line.from, line.to])), [['l', 'host', 'nas']], 'runs_on is implied by nesting and a guest line attaches to its host');
    assert.equal(model.buildScene({ ...input, view: 'topology' }).regions.length, 0, 'topology hides regions');
    const normalized = model.normalizeNode({ id: 'n', icon: 'nope' });
    assert.equal(normalized.icon, 'generic-device');
    assert.equal(normalized.parent_id, '');
    assert.equal('host_id' in normalized, false, 'a node never gains fields the node PUT would reject');
}

// ---- layout intents, history and prefs --------------------------------------
{
    assert.deepEqual(plain(model.mergeIntent({ x: 1, y: 1, parent_id: 'room' }, { x: 5, y: 6, pinned: true })), { x: 5, y: 6, parent_id: 'room', pinned: true });
    assert.deepEqual(plain(model.mergeIntent({ x: 1, y: 1, parent_id: 'room' }, { unplace: true })), { unplace: true }, 'an unplace replaces every earlier intent');
    assert.deepEqual(plain(model.mergeIntent({ unplace: true }, { x: 5, y: 6, pinned: true })), { x: 5, y: 6, pinned: true }, 'a later placement lifts an unplace');
    assert.deepEqual(plain(model.mergeIntent({ x: 1, y: 1, parent_id: 'room' }, { unplace: true, parent_id: '' })), { unplace: true, parent_id: '' }, 'an unplace keeps its own parent_id');
    assert.deepEqual(plain(model.mergeIntent({ unplace: true, parent_id: '' }, { x: 5, y: 6, parent_id: 'room' })), { x: 5, y: 6, parent_id: 'room' }, 'a later placement lifts an unplace with a parent');
    const nodesById = new Map([['a', { id: 'a', kind: 'device', version: 3, placements: {} }], ['r', { id: 'r', kind: 'room', version: 7, placements: { atlas: { x: 0, y: 0, w: 400, h: 300 } } }]]);
    const intents = new Map([['a', { x: 10.4, y: 20.6, parent_id: 'r' }], ['r', { x: 24, y: 48 }], ['gone', { x: 0, y: 0 }]]);
    const legacy = model.layoutItems(intents, nodesById, { parent: false, view: 'atlas' });
    assert.deepEqual(plain(legacy.items), [{ id: 'a', version: 3, x: 10, y: 21, pinned: true }, { id: 'r', version: 7, x: 24, y: 48, pinned: true }], 'without server support parent and size stay local');
    assert.deepEqual(plain(legacy.missing), ['gone']);
    const extended = model.layoutItems(intents, nodesById, { parent: true, view: 'atlas' });
    assert.deepEqual(plain(extended.items), [{ id: 'a', version: 3, x: 10, y: 21, pinned: true, parent_id: 'r' }, { id: 'r', version: 7, x: 24, y: 48, pinned: true, w: 400, h: 300 }], 'a place always carries its size so the server never drops it');
    const unplaceIntents = new Map([['a', { unplace: true }]]);
    assert.deepEqual(plain(model.layoutItems(unplaceIntents, nodesById, { parent: true, view: 'atlas' }).items), [{ id: 'a', version: 3, unplace: true }], 'an unplace carries only id and version');
    assert.deepEqual(plain(model.layoutItems(unplaceIntents, nodesById, { parent: false, view: 'atlas' }).items), [], 'a server without layout support never receives unplace');
    assert.deepEqual(plain(model.layoutItems(new Map([['a', { unplace: true, parent_id: '' }]]), nodesById, { parent: true, view: 'atlas' }).items), [{ id: 'a', version: 3, unplace: true, parent_id: '' }], 'an unplace carries its parent_id when it has one');
    const rebased = model.rebaseLayout(new Map([['a', { x: 1, y: 2, parent_id: 'gone' }], ['b', { x: 3, y: 4 }]]), nodesById);
    assert.deepEqual(plain([...rebased.kept]), [['a', { x: 1, y: 2 }]], 'a vanished parent is dropped, the coordinates stay');
    assert.deepEqual(plain(rebased.dropped), ['b']);
    const history = model.createHistory(2);
    history.push('one'); history.push('two'); history.push('three');
    assert.equal(history.takeUndo(), 'three');
    history.pushRedo('three');
    assert.equal(history.canRedo(), true);
    history.push('four');
    assert.equal(history.canRedo(), false, 'a new action clears redo');
    assert.equal(history.takeUndo(), 'four');
    assert.equal(history.takeUndo(), 'two');
    assert.equal(history.takeUndo(), null, 'depth is capped');
    assert.deepEqual(plain(model.readLayoutPrefs('{"libraryWidth":9999,"inspectorWidth":"x","libraryCollapsed":true,"minimap":false}')), { libraryWidth: 400, libraryCollapsed: true, inspectorWidth: 360, minimap: false });
    assert.deepEqual(plain(model.readLayoutPrefs('not json')), { libraryWidth: 264, libraryCollapsed: false, inspectorWidth: 360, minimap: true });
    assert.equal(model.fill('{count} of {total}', { count: 2, total: 5 }), '2 of 5');
    const full = [{ id: 'c', version: 1, parent_id: 'room', placements: {} }];
    model.applyLayoutPatch(full, [{ id: 'c', version: 3, placements: { atlas: { x: 1, y: 2, pinned: true } } }], 'atlas');
    assert.equal(full[0].parent_id, '', 'a full node without parent_id has left its place');
    model.applyLayoutPatch(full, [{ id: 'c', version: 4, parent_id: 'rack', placements: { atlas: { x: 1, y: 2, pinned: true, w: 300, h: 200 } } }], 'atlas');
    assert.equal(full[0].parent_id, 'rack');
    assert.equal(full[0].placements.atlas.w, 300);
    model.applyLayoutPatch(full, [{ id: 'c', version: 5, x: 3, y: 4, pinned: true }], 'atlas');
    assert.equal(full[0].parent_id, 'rack', 'a bare position patch leaves the parent alone');
    model.applyLayoutPatch(full, [{ id: 'c', version: 6, parent_id: 'rack', placements: { topology: { x: 7, y: 8, pinned: true } } }], 'atlas');
    assert.equal('atlas' in full[0].placements, false, 'a full node without this view has been unplaced there');
    const hostedStamp = model.snapshotStamp([{ ...base.nodes[0], host_id: 'h' }], base.edges, base.sources, base.caps);
    assert.notEqual(same, hostedStamp, 'a new host is a structural change');
}

// ---- store: layout queue, rebase, undo --------------------------------------
{
    const InfraStore = window.InfraAppStore;
    const storeNode = (id, extra = {}) => ({ id, version: 1, kind: 'device', name: id, icon: 'router', profile: {}, placements: { atlas: { x: 0, y: 0, pinned: true } }, sources: [], ...extra });
    const conflict = () => Object.assign(new Error('conflict'), { status: 409, body: { error: 'conflict' } });
    const fakeApi = routes => {
        const calls = [];
        const api = async (url, options = {}) => {
            const route = `${options.method || 'GET'} ${String(url).replace('/api/desktop/infra', '').split('?')[0]}`;
            const body = typeof options.body === 'string' ? JSON.parse(options.body) : undefined;
            calls.push({ route, body });
            const handler = routes[route];
            if (!handler) throw Object.assign(new Error('not_found'), { status: 404, body: { error: 'not_found' } });
            return handler(body);
        };
        return { api, calls };
    };
    const newStore = api => InfraStore.create({ ctx: { api }, t: key => key });
    const echo = body => ({ nodes: body.nodes.map(item => storeNode(item.id, { version: item.version + 1, ...(item.parent_id !== undefined ? { parent_id: item.parent_id } : {}), placements: item.unplace ? {} : { atlas: { x: item.x, y: item.y, pinned: true } } })) });

    { // coalescing; an old server never receives parent_id
        const { api, calls } = fakeApi({ 'PUT /layout': echo });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('a'), storeNode('room', { kind: 'room' })], edges: [], sources: [], capabilities: { write: true } });
        store.queueLayout('atlas', new Map([['a', { x: 24, y: 24 }]]));
        const done = store.queueLayout('atlas', new Map([['a', { x: 48, y: 48, parent_id: 'room' }]]));
        assert.equal(store.effectivePlacement('a', 'atlas').x, 48, 'a queued move shows at once');
        assert.equal(await done, true);
        const puts = calls.filter(call => call.route === 'PUT /layout');
        assert.equal(puts.length, 1, 'moves made in the same tick coalesce into one save');
        assert.deepEqual(plain(puts[0].body.nodes), [{ id: 'a', version: 1, x: 48, y: 48, pinned: true }], 'a server without source_hosts never receives parent_id');
        assert.equal(store.nodeById('a').version, 2);
        store.dispose();
    }
    { // one conflict: reload, reapply onto the fresh version, resend once
        let serverVersion = 1, puts = 0;
        const snapshot = () => ({ nodes: [storeNode('a', { version: serverVersion }), storeNode('room', { kind: 'room' })], edges: [], sources: [], source_hosts: [], capabilities: { write: true } });
        const { api, calls } = fakeApi({
            'GET /snapshot': snapshot,
            'PUT /layout': body => { puts += 1; if (puts === 1) { serverVersion = 5; throw conflict(); } return echo(body); },
        });
        const store = newStore(api);
        store.applySnapshot(snapshot());
        await store.dispatch(InfraStore.commands.reparent('atlas', new Map([['a', { x: 0, y: 0, pinned: true, parent_id: '' }]]), new Map([['a', { x: 96, y: 0, pinned: true, parent_id: 'room' }]])));
        await store.whenLayoutIdle('atlas');
        const sent = calls.filter(call => call.route === 'PUT /layout').map(call => call.body.nodes[0]);
        assert.equal(sent.length, 2, 'a conflict is retried exactly once');
        assert.deepEqual(plain(sent[1]), { id: 'a', version: 5, x: 96, y: 0, pinned: true, parent_id: 'room' }, 'the retry reapplies the intent onto the fresh version');
        assert.equal(calls.filter(call => call.route === 'GET /snapshot').length, 1);
        assert.equal(store.nodeById('a').parent_id, 'room');
        assert.equal(store.unsavedCount('atlas'), 0);
        assert.equal(store.canUndo('atlas'), false, 'a structural conflict clears the view history');
        store.dispose();
    }
    { // a second conflict leaves the position unsaved until Retry
        let fail = true;
        const { api, calls } = fakeApi({
            'GET /snapshot': () => ({ nodes: [storeNode('a')], edges: [], sources: [], source_hosts: [], capabilities: { write: true } }),
            'PUT /layout': body => { if (fail) throw conflict(); return echo(body); },
        });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('a')], edges: [], sources: [], source_hosts: [], capabilities: { write: true } });
        assert.equal(await store.queueLayout('atlas', new Map([['a', { x: 48, y: 0 }]])), false);
        assert.equal(calls.filter(call => call.route === 'PUT /layout').length, 2);
        assert.equal(store.unsavedCount('atlas'), 1, 'a second conflict keeps the position as unsaved');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 48, 'the unsaved position stays visible');
        assert.equal(store.hasUnsavedWork(), true);
        fail = false;
        assert.equal(await store.retryLayout('atlas'), true);
        assert.equal(store.unsavedCount('atlas'), 0);
        store.dispose();
    }
    { // undo/redo and draft alignment
        const { api, calls } = fakeApi({ 'PUT /layout': echo });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('a')], edges: [], sources: [], capabilities: { write: true } });
        store.state.drafts.set('a', { ...store.nodeById('a'), name: 'Draft name' });
        store.state.dirty.add('a');
        await store.dispatch(InfraStore.commands.moveNodes('atlas', new Map([['a', { x: 0, y: 0, pinned: true }]]), new Map([['a', { x: 96, y: 0, pinned: true }]])));
        assert.equal(store.state.drafts.get('a').version, 2, 'our own layout save moves a dirty draft onto the new version');
        assert.equal(store.state.drafts.get('a').name, 'Draft name');
        assert.equal(await store.undo('atlas'), true);
        await store.whenLayoutIdle('atlas');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 0);
        assert.equal(await store.redo('atlas'), true);
        await store.whenLayoutIdle('atlas');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 96);
        assert.deepEqual(plain(calls.map(call => call.body.nodes[0].x)), [96, 0, 96]);
        store.dispose();
    }
    { // placing from the library is undone by unplacing when the server supports it
        const { api, calls } = fakeApi({ 'PUT /layout': echo });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('a', { placements: {} }), storeNode('b'), storeNode('room', { kind: 'room' })], edges: [], sources: [], source_hosts: [], capabilities: { write: true } });
        assert.equal(store.effectivePlacement('a', 'atlas'), null);
        const placed = new Map([['a', { x: 96, y: 48, pinned: true, parent_id: 'room' }], ['b', { x: 240, y: 48, pinned: true }]]);
        await store.dispatch(InfraStore.commands.placeFromLibrary('atlas', placed, 'place'));
        await store.whenLayoutIdle('atlas');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 96);
        assert.equal(store.canUndo('atlas'), true, 'a library placement is undoable against a server with unplace support');
        assert.equal(await store.undo('atlas'), true);
        assert.equal(store.effectivePlacement('a', 'atlas'), null, 'the undo takes the asset off the map at once');
        assert.equal(store.currentIntent('a', 'atlas'), null);
        await store.whenLayoutIdle('atlas');
        const puts = calls.filter(call => call.route === 'PUT /layout').map(call => call.body.nodes);
        assert.deepEqual(plain(puts[1]), [{ id: 'a', version: 2, unplace: true, parent_id: '' }, { id: 'b', version: 2, x: 0, y: 0, pinned: true, parent_id: '' }], 'unplace carries no coordinates but the previous parent; an asset that was placed before goes back to where it was');
        assert.equal(store.nodeById('a').placements.atlas, undefined, 'the saved asset is unplaced afterwards');
        assert.equal(store.layoutState('a', 'atlas'), '');
        assert.equal(await store.redo('atlas'), true);
        await store.whenLayoutIdle('atlas');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 96, 'redo places it again');
        assert.deepEqual(plain(calls.filter(call => call.route === 'PUT /layout')[2].body.nodes[0]), { id: 'a', version: 3, x: 96, y: 48, pinned: true, parent_id: 'room' });
        store.dispose();
    }
    { // undoing a drop out of a place puts the asset back into the place it left
        const { api, calls } = fakeApi({ 'PUT /layout': echo });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('c', { placements: {}, parent_id: 'room' }), storeNode('room', { kind: 'room' })], edges: [], sources: [], source_hosts: [], capabilities: { write: true } });
        await store.dispatch(InfraStore.commands.placeFromLibrary('atlas', new Map([['c', { x: 480, y: 48, pinned: true, parent_id: '' }]]), 'place'));
        await store.whenLayoutIdle('atlas');
        assert.equal(store.nodeById('c').parent_id, '');
        assert.equal(await store.undo('atlas'), true);
        await store.whenLayoutIdle('atlas');
        const puts = calls.filter(call => call.route === 'PUT /layout').map(call => call.body.nodes);
        assert.deepEqual(plain(puts[1]), [{ id: 'c', version: 2, unplace: true, parent_id: 'room' }], 'the unplace restores the parent the drop cleared');
        assert.equal(store.nodeById('c').parent_id, 'room');
        assert.equal(store.nodeById('c').placements.atlas, undefined);
        store.dispose();
    }
    { // a drop that leaves the parent alone unplaces without parent_id
        const { api, calls } = fakeApi({ 'PUT /layout': echo });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('c', { placements: {}, parent_id: 'room' }), storeNode('room', { kind: 'room' })], edges: [], sources: [], source_hosts: [], capabilities: { write: true } });
        await store.dispatch(InfraStore.commands.placeFromLibrary('atlas', new Map([['c', { x: 96, y: 48, pinned: true, parent_id: 'room' }]]), 'place'));
        await store.whenLayoutIdle('atlas');
        await store.undo('atlas');
        await store.whenLayoutIdle('atlas');
        assert.deepEqual(plain(calls.filter(call => call.route === 'PUT /layout').map(call => call.body.nodes)[1]), [{ id: 'c', version: 2, unplace: true }]);
        store.dispose();
    }
    { // against a server without source_hosts a library placement stays out of history
        const { api } = fakeApi({ 'PUT /layout': echo });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('a', { placements: {} })], edges: [], sources: [], capabilities: { write: true } });
        assert.equal(await store.dispatch(InfraStore.commands.placeFromLibrary('atlas', new Map([['a', { x: 96, y: 48, pinned: true }]]), 'place')), true);
        assert.equal(store.effectivePlacement('a', 'atlas').x, 96);
        assert.equal(store.canUndo('atlas'), false, 'an older server cannot remove a placement');
        store.dispose();
    }
    { // an unplace and a placement queued in the same tick resolve to the last one
        const { api, calls } = fakeApi({ 'PUT /layout': echo });
        const store = newStore(api);
        store.applySnapshot({ nodes: [storeNode('a', { placements: {} })], edges: [], sources: [], source_hosts: [], capabilities: { write: true } });
        store.queueLayout('atlas', new Map([['a', { x: 24, y: 24, pinned: true }]]));
        store.queueLayout('atlas', new Map([['a', { unplace: true }]]));
        assert.equal(store.effectivePlacement('a', 'atlas'), null);
        store.queueLayout('atlas', new Map([['a', { x: 48, y: 48, pinned: true }]]));
        assert.equal(store.effectivePlacement('a', 'atlas').x, 48);
        await store.whenLayoutIdle('atlas');
        assert.deepEqual(plain(calls.filter(call => call.route === 'PUT /layout').map(call => call.body.nodes)), [[{ id: 'a', version: 1, x: 48, y: 48, pinned: true }]]);
        store.dispose();
    }
}

// ---- discovery relevance and cleanup -----------------------------------------
{
    const cand = (id, kind, name, profile = {}, source = {}) => ({ id, node: { kind, name, icon: 'generic-device', profile: { tags: [], interfaces: [], ...profile } }, source: { kind: 'registry', instance: 'inventory', key: id, state: 'current', observed: {}, ...source } });
    for (const name of ['3f2a9c1e7b6d', 'b1c2d3e4-f5a6-4b7c-8d9e-0f1a2b3c4d5e', 'AA-BB-CC-DD-EE-FF', '192.168.6.30', 'Chromecast-3c1d2e4f5a6b7c8d', 'Unnamed device']) assert.equal(model.isOpaqueName(name), true, name);
    for (const name of ['Living room TV', 'TheBrain']) assert.equal(model.isOpaqueName(name), false, name);
    const nas = cand('cand-nas', 'device', 'Basement NAS', { ip_address: '192.168.6.20', role: 'server', vendor: 'Synology' });
    const brain = cand('cand-brain', 'device', 'TheBrain', { ip_address: '192.168.6.1', role: 'InternetGatewayDevice' });
    const media = cand('cand-brain-media', 'device', 'TheBrain Mediaserver', { ip_address: '192.168.6.1', role: 'MediaServer' });
    const castNamed = cand('cand-cast', 'device', 'Living room TV', { ip_address: '192.168.6.30', role: 'chromecast' });
    const castUuid = cand('cand-cast-uuid', 'device', 'Chromecast-3c1d2e4f5a6b7c8d', { ip_address: '192.168.6.30', role: 'mdns-device', tags: ['network-scan'] });
    const speaker = cand('cand-speaker', 'device', 'Kitchen speaker', { role: 'mdns-device', tags: ['network-scan', 'full-scan'] });
    const phone = cand('cand-phone', 'device', 'Old phone', { ip_address: '192.168.6.77', vendor: 'AVM FRITZ!Box' }, { kind: 'fritzbox', instance: 'fritz', observed: { power: 'stopped' } });
    const fritzSwitch = cand('cand-switch', 'device', 'Core switch', { ip_address: '192.168.6.2', vendor: 'AVM FRITZ!Box' }, { kind: 'fritzbox', instance: 'fritz', observed: { power: 'running' } });
    const linkLocal = cand('cand-ll', 'device', 'Printer', { ip_address: 'fe80::1c2b:3aff:fe4d:5e6f', role: 'printer' });
    const stopped = cand('cand-ct', 'container', 'legacy-db', {}, { kind: 'docker', instance: 'engine-a', observed: { power: 'stopped' } });
    const nodes = [{ id: 'core-switch', kind: 'device', name: 'Core switch', profile: { ip_address: '192.168.6.2' } }];
    const all = [nas, brain, media, castNamed, castUuid, speaker, phone, fritzSwitch, linkLocal, stopped];
    const rel = candidate => plain(model.discoveryRelevance(candidate, nodes, all));
    assert.deepEqual(rel(nas), { group: 'recommended', reasons: [] });
    assert.deepEqual(rel(brain), { group: 'recommended', reasons: [] }, 'the shorter base name wins an equally detailed pair');
    assert.deepEqual(rel(media), { group: 'duplicate', reasons: ['duplicate_ip'], duplicateOf: { candidateId: 'cand-brain', match: 'ip' } });
    assert.deepEqual(rel(castNamed), { group: 'recommended', reasons: [] });
    assert.deepEqual(rel(castUuid), { group: 'duplicate', reasons: ['duplicate_ip', 'generic_discovery'], duplicateOf: { candidateId: 'cand-cast', match: 'ip' } }, 'the opaque mDNS twin of a Chromecast is the duplicate');
    assert.deepEqual(rel(speaker), { group: 'unlikely', reasons: ['generic_discovery'] });
    assert.deepEqual(rel(phone), { group: 'unlikely', reasons: ['inactive'] });
    assert.deepEqual(rel(fritzSwitch), { group: 'duplicate', reasons: ['duplicate_ip'], duplicateOf: { nodeId: 'core-switch', match: 'ip' } }, 'an existing asset wins over every discovery');
    assert.deepEqual(rel(linkLocal), { group: 'unlikely', reasons: ['link_local'] });
    assert.deepEqual(rel(stopped), { group: 'unlikely', reasons: ['inactive'] });
    assert.deepEqual(plain(model.discoveryRelevance(stopped, nodes, all, { sourceHosts: [{ kind: 'docker', instance: 'engine-a', node_id: 'host-1', version: 1 }] })), { group: 'recommended', reasons: [] }, 'a mapped engine keeps stopped containers relevant');
    const groups = model.groupDiscoveries(all, nodes);
    assert.deepEqual(plain(groups.recommended.map(c => c.id)), ['cand-nas', 'cand-cast', 'cand-brain']);
    assert.deepEqual(plain(groups.duplicate.map(c => c.id)), ['cand-cast-uuid', 'cand-switch', 'cand-brain-media']);
    assert.deepEqual(plain(groups.unlikely.map(c => c.id)), ['cand-speaker', 'cand-ct', 'cand-phone', 'cand-ll']);
    assert.equal(groups.relevance.get('cand-switch').duplicateOf.nodeId, 'core-switch');
    const linkNodes = [
        { id: 'manual-1', kind: 'device', name: 'Atlas gateway', profile: {} },
        { id: 'sw', kind: 'device', name: 'Core switch', profile: { ip_address: '192.168.6.2' } },
        { id: 'sw-reg', kind: 'device', name: 'Switch', registry_id: 'other-device', profile: { ip_address: '192.168.6.2' } },
        { id: 'room', kind: 'room', name: 'Core switch', profile: {} },
    ];
    assert.deepEqual(plain(model.linkSuggestions(fritzSwitch, linkNodes)), ['sw', 'sw-reg'], 'address matches first; places are never link targets');
    const atlas = { ...cand('registry-candidate', 'device', 'Atlas gateway', { ip_address: '192.0.2.9' }), registry_id: 'registry-device-9' };
    assert.deepEqual(plain(model.linkSuggestions(atlas, linkNodes)), ['manual-1'], 'registry discoveries skip assets with another registry device');
    assert.equal(model.bindableFor(atlas, { kind: 'service', name: 'x' }), false);
    const src = (kind, name, extra = {}) => ({ kind, name, observed: { name }, ...extra });
    const cleanupNodes = [
        { id: 'tv', kind: 'device', name: 'Living room TV', placements: { atlas: { x: 0, y: 0 } }, profile: { ip_address: '192.168.6.30' }, sources: [src('registry', 'Living room TV')] },
        { id: 'tv-uuid', kind: 'device', name: 'Chromecast-3c1d2e4f5a6b7c8d', placements: {}, profile: { ip_address: '192.168.6.30' }, sources: [src('registry', 'Chromecast-3c1d2e4f5a6b7c8d')] },
        { id: 'blob', kind: 'device', name: '3f2a9c1e7b6d', placements: {}, profile: {}, sources: [src('fritzbox', '3f2a9c1e7b6d')] },
        { id: 'renamed', kind: 'device', name: 'a1b2c3d4e5f6', placements: {}, profile: {}, sources: [src('registry', 'Garage sensor')] },
        { id: 'ct-1', kind: 'container', name: 'radarr', placements: {}, profile: {}, sources: [{ kind: 'docker', instance: 'engine-a', key: 'c1' }] },
        { id: 'ct-2', kind: 'container', name: 'sonarr', placements: {}, profile: {}, sources: [{ kind: 'docker', instance: 'engine-a', key: 'c2' }] },
        { id: 'ct-hosted', kind: 'container', name: 'plex', host_id: 'host', placements: {}, profile: {}, sources: [{ kind: 'docker', instance: 'engine-a', key: 'c3' }] },
        { id: 'host', kind: 'device', name: 'Docker host', placements: {}, profile: {}, sources: [] },
        { id: 'vm-1', kind: 'vm', name: 'Lab VM', placements: {}, profile: {}, sources: [] },
    ];
    const suggestions = plain(model.infraCleanup(cleanupNodes, [], [{ kind: 'docker', instance: 'engine-a', node_id: '', version: 0, inferred_node_id: '' }], { edges: [] }));
    assert.deepEqual(suggestions.map(item => item.id), ['duplicate:ip:192.168.6.30:tv,tv-uuid', 'engine-host:docker:engine-a', 'guest-host:vm-1', 'opaque:blob'], 'renamed opaque assets and hosted guests make no suggestion');
    assert.deepEqual(suggestions[0], { id: 'duplicate:ip:192.168.6.30:tv,tv-uuid', type: 'duplicate', match: 'ip', value: '192.168.6.30', keepId: 'tv', removeIds: ['tv-uuid'], nodeIds: ['tv', 'tv-uuid'] });
    assert.deepEqual(suggestions[1], { id: 'engine-host:docker:engine-a', type: 'engine-host', kind: 'docker', instance: 'engine-a', version: 0, inferredNodeId: '', guestIds: ['ct-1', 'ct-2'], nodeIds: ['ct-1', 'ct-2'] });
    assert.deepEqual(suggestions[2], { id: 'guest-host:vm-1', type: 'guest-host', nodeId: 'vm-1', nodeIds: ['vm-1'] });
    assert.deepEqual(suggestions[3], { id: 'opaque:blob', type: 'opaque', nodeId: 'blob', nodeIds: ['blob'] });
    const again = plain(model.infraCleanup(cleanupNodes, [], [{ kind: 'docker', instance: 'engine-a', node_id: '', version: 4, inferred_node_id: 'host' }], { edges: [], dismissed: new Set(['opaque:blob']) }));
    assert.deepEqual(again.map(item => item.id), ['duplicate:ip:192.168.6.30:tv,tv-uuid', 'engine-host:docker:engine-a', 'guest-host:vm-1'], 'dismissed suggestions stay hidden');
    assert.equal(again[1].version, 4);
    assert.equal(again[1].inferredNodeId, 'host');
    assert.ok(!plain(model.infraCleanup(cleanupNodes, [], [], { edges: [{ id: 'e', kind: 'runs_on', from: 'vm-1', to: 'host' }] })).some(item => item.id === 'guest-host:vm-1'), 'a runs_on edge gives a guest its host');
}

// ---- List view, inspector and picker helpers -------------------------------------
{
    const rows = [
        { id: 'b', name: 'Beta', kind: 'device', power: 'running', profile: { ip_address: '192.168.6.10' }, sources: [{ kind: 'registry', state: 'current' }] },
        { id: 'a', name: 'alpha', kind: 'vm', power: 'stopped', profile: { ip_address: '192.168.6.9' }, sources: [] },
        { id: 'c', name: 'Gamma 10', kind: 'device', profile: {}, sources: [{ kind: 'docker', state: 'stale' }] },
        { id: 'd', name: 'Gamma 9', kind: 'device', profile: { mac_address: 'aa:bb:cc:dd:ee:ff' }, sources: [{ kind: 'fritzbox', state: 'removed' }] },
    ];
    const ids = (key, dir, ctx) => plain(rows.slice().sort(model.tableComparator(key, dir, ctx)).map(row => row.id));
    assert.deepEqual(ids('name', 'asc'), ['a', 'b', 'd', 'c'], 'names sort naturally, case-insensitive');
    assert.deepEqual(ids('name', 'desc'), ['c', 'd', 'b', 'a']);
    assert.deepEqual(ids('address', 'asc'), ['a', 'b', 'd', 'c'], 'IPv4 numerically, other addresses after, empty last');
    assert.deepEqual(ids('address', 'desc'), ['d', 'b', 'a', 'c'], 'empty stays last in both directions');
    assert.deepEqual(ids('status', 'asc'), ['d', 'c', 'b', 'a'], 'problems first');
    assert.deepEqual(ids('power', 'asc'), ['b', 'a', 'd', 'c']);
    assert.deepEqual(ids('kind', 'asc'), ['b', 'd', 'c', 'a']);
    assert.deepEqual(ids('place', 'asc', { placeOf: id => ({ c: 'Basement', a: 'Attic' })[id] || '' }), ['a', 'c', 'b', 'd']);
    const tree = [{ id: 'host', name: 'Host', kind: 'device' }, { id: 'g2', name: 'web', kind: 'container' }, { id: 'g1', name: 'db', kind: 'container' }, { id: 'solo', name: 'Alone', kind: 'device' }, { id: 'nested', name: 'inner', kind: 'service' }];
    const hostOf = new Map([['g1', 'host'], ['g2', 'host'], ['nested', 'g1']]), guestsOf = new Map([['host', ['g2', 'g1']], ['g1', ['nested']]]);
    const flat = opts => plain(model.tableRows(tree, { hostOf, guestsOf, compare: model.tableComparator('name', 'asc'), ...opts }).map(row => `${row.id}:${row.depth}${row.expanded ? '+' : ''}${row.context ? '~' : ''}`));
    assert.deepEqual(flat({}), ['solo:0', 'host:0'], 'guests stay folded under their host');
    assert.deepEqual(flat({ expanded: new Set(['host']) }), ['solo:0', 'host:0+', 'g1:1', 'g2:1']);
    assert.deepEqual(flat({ expanded: new Set(['host', 'g1']) }), ['solo:0', 'host:0+', 'g1:1+', 'nested:2', 'g2:1']);
    assert.deepEqual(flat({ matches: node => node.id === 'nested', filtering: true }), ['host:0+~', 'g1:1+~', 'nested:2'], 'a matching guest shows its hosts as context rows');
    const targets = [
        { app: 'network-cameras', target_id: 'stream-front-door', label: 'Front door camera' },
        { app: 'software-store', target_id: 'net-tools', label: 'Network Tools' },
        { app: 'meshcore', target_id: 'conv-ipt', label: 'Iptingen Room Server' },
        { app: 'meshcore', target_id: 'conv-lab', label: 'Lab mesh conversation' },
    ];
    const options = (node, query) => { const result = model.appTargetOptions(node, targets, query); return plain({ groups: result.groups.map(group => [group.id, group.options.map(option => option.target_id)]), hiddenMesh: result.hiddenMesh }); };
    const tv = { id: 'tv', name: 'Living room TV', icon: 'tv', profile: { app_bindings: [] } };
    assert.deepEqual(options(tv, ''), { groups: [['network-cameras', ['stream-front-door']], ['software-store', ['net-tools']]], hiddenMesh: 2 }, 'a TV never sees MeshCore by default');
    assert.deepEqual(options(tv, 'ip'), { groups: [['suggested', ['conv-ipt']]], hiddenMesh: 0 }, 'two typed characters search MeshCore too');
    const relay = { id: 'relay', name: 'Garden relay', icon: 'mesh-node', profile: { app_bindings: [{ app: 'meshcore', target_id: 'conv-lab' }] } };
    assert.deepEqual(options(relay, ''), { groups: [['network-cameras', ['stream-front-door']], ['software-store', ['net-tools']], ['meshcore', ['conv-ipt']]], hiddenMesh: 0 }, 'mesh devices see MeshCore; bound targets are not offered again');
    assert.equal(model.appTargetOptions({ name: 'Node', icon: 'router', profile: { tags: ['Mesh backbone'] } }, targets, '').hiddenMesh, 0);
    assert.equal(model.isMeshAsset({ icon: 'radio-gateway', profile: {} }), true);
    assert.equal(model.isMeshAsset({ icon: 'tv', profile: { role: 'chromecast' } }), false);
    assert.equal(model.appTargetOptions({ name: 'Front door', icon: 'camera', profile: {} }, targets, '').groups[0].id, 'suggested');
    const prefs = model.readInspectorPrefs('{"tabs":{"device":"details","vm":"nope","Bad Kind":"sources"}}');
    assert.deepEqual(plain(prefs), { tabs: { device: 'details' } });
    assert.equal(model.inspectorTab(prefs, 'device'), 'details');
    assert.equal(model.inspectorTab(prefs, 'room'), 'overview');
    assert.deepEqual(plain(model.withInspectorTab(prefs, 'room', 'connections')), { tabs: { device: 'details', room: 'connections' } });
    assert.deepEqual(plain(model.readInspectorPrefs('not json')), { tabs: {} });
    assert.deepEqual(plain(model.readTablePrefs('{"sort":"address","dir":"desc"}')), { sort: 'address', dir: 'desc' });
    assert.deepEqual(plain(model.readTablePrefs('{"sort":"secret"}')), { sort: 'name', dir: 'asc' });
    assert.deepEqual([...model.readDismissed('["a",3,"b"]')], ['a', 'b']);
    const stored = { id: 'n', name: 'NAS', kind: 'device', icon: 'nas', parent_id: '', profile: { vendor: '', vlan: 0, interfaces: [], tags: ['lab'], custom: {} } };
    const draft = { ...model.clone(stored), name: 'Big NAS', profile: { ...model.clone(stored.profile), vendor: 'Synology', vlan: '', interfaces: [{ name: 'eth0' }] } };
    assert.deepEqual([...model.draftDiff(stored, draft)].sort(), ['name', 'profile.interfaces', 'profile.vendor'], 'blank values compare equal');
    assert.deepEqual(plain(model.formGroupFill(stored)), { identity: true, network: false, hardware: false, links: false, notes: false, custom: false });
    assert.deepEqual(plain(model.formGroupFill(draft)), { identity: true, network: true, hardware: true, links: false, notes: false, custom: false });
    assert.equal(model.groupOfField('profile.mac_address'), 'network');
    assert.deepEqual(plain(model.attentionItems([
        { id: 'ok', name: 'Fine', sources: [{ kind: 'docker', state: 'current' }] },
        { id: 'stale', name: 'Old', sources: [{ kind: 'docker', state: 'current' }, { kind: 'proxmox', state: 'stale' }] },
        { id: 'moved', name: 'Moved', sources: [] },
        { id: 'quiet', name: 'Quiet', sources: [{ kind: 'docker', state: 'removed', ignored: true }] },
    ], { unsaved: new Set(['moved']) })), [{ id: 'moved', reasons: [{ code: 'layout_unsaved' }] }, { id: 'stale', reasons: [{ code: 'source_stale', source: 'proxmox' }] }], 'any non-current source counts; an ignored source does not');
    const places = { home: { id: 'home', name: 'Home', parent_id: '' }, base: { id: 'base', name: 'Basement', parent_id: 'home' }, rack: { id: 'rack', name: 'Rack 1', parent_id: 'base' } };
    assert.deepEqual(plain(model.placePath('rack', id => places[id]).map(item => item.name)), ['Home', 'Basement']);
    assert.deepEqual(plain(model.placePath('rack', id => places[id], { includeSelf: true }).map(item => item.name)), ['Home', 'Basement', 'Rack 1']);
    assert.equal(model.isDescendant('rack', 'home', id => places[id]?.parent_id || ''), true);
    assert.equal(model.isDescendant('home', 'rack', id => places[id]?.parent_id || ''), false);
    assert.deepEqual(plain(model.appLinkContext({ app: 'meshcore', target_id: 'c1' })), { conversation_id: 'c1' });
    // Plan B already pins mergeIntent, layoutItems and applyLayoutPatch for unplace intents; the stamp is new.
    const ignoredStamp = model.snapshotStamp(base.nodes, base.edges, [{ ...base.sources[0], ignored: true }], base.caps);
    assert.notEqual(same, ignoredStamp, 'ignoring or restoring a source is a structural change');
    assert.notEqual(model.snapshotStamp([{ ...base.nodes[0], sources: [{ ...base.sources[0] }] }], base.edges, base.sources, base.caps), model.snapshotStamp([{ ...base.nodes[0], sources: [{ ...base.sources[0], ignored: true }] }], base.edges, base.sources, base.caps), 'a linked source flips with its ignored flag');
}

for (const file of ['desktop-app-infra.css', 'desktop-app-infra-canvas.css']) {
    const css = fs.readFileSync(path.join(root, 'ui/css', file), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
    const outsideMixes = css.replace(/color-mix\((?:[^()]|\([^()]*\))*\)/g, '');
    assert.deepEqual(outsideMixes.match(/#[0-9a-fA-F]{3,8}\b/g) || [], [], `${file} uses hex colours outside color-mix()`);
    assert.deepEqual(outsideMixes.match(/rgba?\(\s*\d/g) || [], [], `${file} uses literal rgb() colours instead of tokens`);
    assert.ok(!/var\(--vd-font[,)]/.test(css), `${file} uses --vd-font instead of --vd-font-ui`);
    assert.ok(!/copper/i.test(css), `${file} still uses the copper accent`);
}

console.log(`Infra UI model, grouped virtualization, and ${required.size} translation keys passed for ${locales.length} locales.`);
