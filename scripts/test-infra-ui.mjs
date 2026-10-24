import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const files = ['infra-model.js', 'infra-lanes.js', 'infra-store.js', 'infra-picker.js', 'infra-canvas.js', 'infra-gestures.js', 'infra-library.js', 'infra-cleanup.js', 'infra-inspector-forms.js', 'infra-inspector.js', 'infra-table.js', 'infra-dialogs.js', 'infra-import.js', 'infra-menus.js', 'infra.js'];
for (const file of files) {
    const text = fs.readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8');
    const lines = text.split('\n').length;
    assert.ok(lines < 1100, `${file} has ${lines} lines, want fewer than 1100`);
}
const source = files.map(file => fs.readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8')).join('\n');
const window = { BUILD_VERSION: 'test-build', btoa: value => Buffer.from(value, 'binary').toString('base64') };
vm.runInNewContext(source, { window, console, setTimeout, clearTimeout, DOMException, TextEncoder, Map, Set, JSON, Math, Date, URL, AbortController });
const model = window.InfraAppModel, lanes = window.InfraAppLanes;
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
for (const key of ['desktop.infra_access_unavailable', 'desktop.infra_action_unavailable', 'desktop.infra_saved_newer_draft', 'desktop.infra_source_saved_newer_draft', 'desktop.infra_layout_pending_review', 'desktop.infra_position', 'desktop.infra_source_compare', 'desktop.infra_node_compare', 'desktop.infra_current_source', 'desktop.infra_current_inventory', 'desktop.infra_your_draft', 'desktop.infra_no_field_differences', 'desktop.infra_reapply_draft', 'desktop.infra_reapply_ready', 'desktop.infra_node_conflict_review', 'desktop.infra_target_unavailable', 'desktop.infra_action_outcome_unknown', 'desktop.infra_pve_tags_hint', 'desktop.infra_error_timeout', 'desktop.infra_field_app_bindings']) required.add(key);
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

// Counts read "1 asset", not "1 assets": every counted text has a singular key without a count placeholder.
const singularKeys = ['connections_summary', 'asset_count', 'imported', 'guest_count', 'region_label', 'moved_to_place', 'moved_out_of_place', 'cleanup_show_dismissed', 'cleanup_engine_host', 'cleanup_show_guests', 'removed_ignored', 'removed_from_map', 'picker_mesh_hidden', 'ignored_count', 'import_partial', 'select_all_rows', 'sources_count', 'moved', 'guests_badge', 'guests_badge_stopped', 'filters_more'].map(name => `desktop.infra_${name}_one`);
for (const locale of locales) {
    const dictionary = JSON.parse(fs.readFileSync(path.join(root, `ui/lang/desktop/${locale}.json`), 'utf8'));
    const bad = singularKeys.filter(key => typeof dictionary[key] !== 'string' || !dictionary[key].trim() || dictionary[key].includes('{count}'));
    assert.deepEqual(bad, [], `${locale}.json lacks singular texts (or keeps a count placeholder) for: ${bad.join(', ')}`);
}
assert.equal(english['desktop.infra_connections_summary_one'], '1 connection');
assert.equal(english['desktop.infra_asset_count_one'], '1 asset');
assert.equal(english['desktop.infra_cleanup_show_guests_one'], 'Show 1 guest');
assert.equal(model.countText(1, '1 connection', '{count} connections'), '1 connection');
assert.equal(model.countText(0, '1 connection', '{count} connections'), '0 connections');
assert.equal(model.countText(3, '1 asset moved to {place}', '{count} assets moved to {place}', { place: 'Rack' }), '3 assets moved to Rack');
assert.deepEqual(Array.from(model.APP_LINK_TARGETS), ['quick-connect', 'network-cameras', 'software-store', 'meshcore'], 'the linkable apps live in the model');
assert.equal((source.match(/\bAPP_LINK_TARGETS\s*=/g) || []).length, 1, 'APP_LINK_TARGETS is defined once and read by the inspector and the menus');
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

assert.deepEqual(plain(model.edgeSegment({ x: 0, y: 0 }, { x: 400, y: 0 })), { x1: 108, y1: 0, x2: 292, y2: 0 }, 'connections start and end at the 216×64 card border');
assert.deepEqual(plain(model.edgeSegment({ x: 0, y: 0 }, { x: 0, y: 300 })), { x1: 0, y1: 32, x2: 0, y2: 268 });
assert.deepEqual(plain(model.edgeSegment({ x: 0, y: 0 }, { x: 100, y: 0 })), { x1: 0, y1: 0, x2: 100, y2: 0 }, 'overlapping cards keep the centre line');
assert.equal(model.edgeSegment({ x: 0, y: 0 }, { x: 1000, y: 0 }, undefined, { w: 300, h: 100 }).x2, 700, 'a region end stops at its own border');
assert.equal(model.edgeCaption({ from_port: 'ge-0/0/1', to_port: 'eth0', speed: '1G' }), 'ge-0/0/1 → eth0 · 1G');
// A5: a connection label sits at the midpoint of the visible segment and is hidden when its box would touch a card.
{
    const cards = [model.cardRect({ x: 0, y: 0 }), model.cardRect({ x: 252, y: 0 })];
    const gap = model.edgeSegment({ x: 0, y: 0 }, { x: 252, y: 0 });
    assert.deepEqual(plain(gap), { x1: 108, y1: 0, x2: 144, y2: 0 }, 'the visible segment between two neighbours is the 36 px gap');
    assert.equal(model.edgeLabelClash(gap, { w: 90, h: 17 }, cards), true, 'a 90 px label does not fit in a 36 px gap');
    assert.equal(model.edgeLabelClash(gap, { w: 20, h: 17 }, cards, 0), false, 'a 20 px label fits in the gap without padding');
    assert.equal(model.edgeLabelClash(gap, { w: 20, h: 17 }, cards, 12), true, 'padding counts');
    const far = model.edgeSegment({ x: 0, y: 0 }, { x: 800, y: 0 });
    assert.equal(model.edgeLabelClash(far, { w: 120, h: 17 }, cards.slice(0, 1)), false, 'a long segment holds its label');
    assert.equal(model.edgeLabelClash(far, { w: 120, h: 17 }, [model.cardRect({ x: 400, y: 0 })]), true, 'a card sitting on the midpoint blocks the label');
    assert.equal(model.edgeLabelClash(far, { w: 120, h: 17 }, [model.cardRect({ x: 400, y: 200 })]), false, 'a card off the line does not');
    assert.equal(model.edgeLabelClash(far, { w: 120, h: 17 }, []), false);
}

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
    // a padding object keeps more room on one side (the bottom, where the tools and the names of compact cards sit)
    const sides = { top: 48, right: 48, bottom: 104, left: 48 };
    assert.deepEqual(plain(model.fitView({ x: 0, y: 0, w: 400, h: 200 }, { w: 1000, h: 800 }, sides)), { x: 300, y: 272, zoom: 1 }, 'the content sits 28 px higher: 48 + (648 - 200) / 2');
    const tall = model.fitView({ x: 0, y: 0, w: 100, h: 1300 }, { w: 1000, h: 800 }, sides);
    assert.ok(Math.abs(tall.zoom - 648 / 1300) < 1e-9 && Math.abs(tall.y - 48) < 1e-9, 'a tall scene gets 48 px above and 104 px below');
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
    assert.deepEqual(plain(a), { x: -108, y: -32, w: 216, h: 64 }, 'a card is 216×64 around its centre');
    const slot = model.freeSlot({ x: 0, y: 0 }, [a]);
    assert.ok(!model.intersects(model.cardRect(slot), a), 'the nudged card no longer overlaps');
    assert.equal(Math.hypot(slot.x, slot.y), 72, 'the nearest free slot is one card height plus the 8 px gap away (3 grid cells)');
    assert.ok(Number.isInteger(slot.x / 24) && Number.isInteger(slot.y / 24), 'a nudged card lands on the grid');
    assert.deepEqual(plain(model.freeSlot({ x: 500, y: 501 }, [a])), { x: 500, y: 501 }, 'a free drop keeps its exact point');
    assert.deepEqual(plain(model.regionRect({ x: 0, y: 0 })), { x: 0, y: 0, w: 360, h: 240 }, 'a place without size gets 360×240');
    assert.deepEqual(plain(model.regionRect({ x: 0, y: 0, w: 200, h: 140 }, [model.cardRect({ x: 400, y: 100 })])), { x: 0, y: 0, w: 556, h: 180 }, 'children grow the region by the 48 px frame padding');
    const placements = { site: { x: 0, y: 0, w: 200, h: 140 }, rack: { x: 300, y: 300, w: 200, h: 140 } };
    const parentOf = id => ({ rack: 'site' })[id] || '';
    const layout = model.regionLayout(['site', 'rack'], { parentOf, placementOf: id => placements[id], cardRectsByParent: new Map([['rack', [model.cardRect({ x: 400, y: 380 })]]]) });
    assert.deepEqual(plain(layout.rects.get('rack')), { x: 244, y: 300, w: 312, h: 160 });
    assert.deepEqual(plain(layout.rects.get('site')), { x: 0, y: 0, w: 604, h: 508 }, 'a nested place grows its parent');
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
    assert.deepEqual(plain(growth.get('rack')), { x: 300, y: 300, w: 556, h: 160 });
    assert.deepEqual(plain(growth.get('site')), { x: 0, y: 0, w: 904, h: 508 }, 'growth propagates to the ancestors');
    const items = [{ id: 'c', x: 0, y: 0 }, { id: 'r', x: 220, y: 10 }, { id: 'far', x: 600, y: 0 }, { id: 'd', x: 0, y: 155 }, { id: 'diag', x: 150, y: 150 }];
    assert.equal(model.spatialNeighbour(items[0], items, 'right'), 'r');
    assert.equal(model.spatialNeighbour(items[0], items, 'down'), 'd');
    assert.equal(model.spatialNeighbour(items[0], items, 'left'), '');
    assert.deepEqual(plain(model.compactGrid(4, { x: 0, y: 0 })), [{ x: -120, y: -48 }, { x: 120, y: -48 }, { x: -120, y: 48 }, { x: 120, y: 48 }], 'a compact grid keeps 240×96 cells on the 24 px grid');
    assert.equal(model.defaultEdgeKind({ kind: 'device', icon: 'router' }, { kind: 'device', icon: 'nas' }), 'ethernet');
    assert.equal(model.defaultEdgeKind({ kind: 'device', icon: 'access-point' }, { kind: 'device', icon: 'laptop' }), 'wifi');
    assert.equal(model.defaultEdgeKind({ kind: 'service', icon: 'web-service' }, { kind: 'device', icon: 'nas' }), 'depends_on');
    const mini = model.minimapTransform({ x: 0, y: 0, w: 1000, h: 500 }, { w: 160, h: 104 });
    assert.ok(Math.abs(mini.scale - 0.148) < 1e-9 && Math.abs(mini.x - 6) < 1e-9 && Math.abs(mini.y - 15) < 1e-9, 'the 160x104 minimap centres the world');
    // The minimap hides itself while the whole scene is inside the viewport (A3).
    const scene = { x: 0, y: 0, w: 800, h: 400 }, screen = { w: 1000, h: 600 };
    assert.equal(model.sceneFits(scene, { x: 100, y: 100, zoom: 1 }, screen), true, 'a scene inside the viewport fits');
    assert.equal(model.sceneFits(scene, { x: 100, y: 100, zoom: 2 }, screen), false, 'zoomed in, the scene overflows');
    assert.equal(model.sceneFits(scene, { x: -50, y: 100, zoom: 1 }, screen), false, 'a scene scrolled partly out of view does not fit');
    assert.equal(model.sceneFits(scene, model.fitView(scene, screen), screen), true, 'the fit view always fits');
    assert.equal(model.sceneFits(null, { x: 0, y: 0, zoom: 1 }, screen), true, 'an empty scene needs no minimap');
    assert.equal(model.sceneFits(scene, { x: 0, y: 0, zoom: 1 }, { w: 0, h: 0 }), true, 'an unmeasured viewport hides it too');
}

// ---- semantic zoom (A1) ------------------------------------------------------
{
    assert.equal(model.zoomLevels(0.64).form, 'compact', 'below 0.65 cards are compact');
    assert.equal(model.zoomLevels(0.65).form, 'full');
    assert.equal(model.zoomLevels(0.67, 'compact').form, 'compact', 'a compact form is kept until 0.68 so a pinch does not flicker');
    assert.equal(model.zoomLevels(0.68, 'compact').form, 'full');
    assert.equal(model.zoomLevels(0.67, 'full').form, 'full');
    assert.deepEqual([0.49, 0.5].map(zoom => model.zoomLevels(zoom).handles), [false, true], 'connection handles go below 0.5');
    assert.deepEqual([0.74, 0.75].map(zoom => model.zoomLevels(zoom).edgeLabels), [false, true], 'edge labels go below 0.75');
    assert.deepEqual([0.39, 0.4].map(zoom => model.zoomLevels(zoom).tiny), [true, false]);
    for (const zoom of [0.25, 0.28, 0.3, 0.44, 0.5, 0.65, 0.74, 0.75, 0.8125, 1, 1.5, 2]) {
        const levels = model.zoomLevels(zoom);
        assert.ok(16 * zoom * levels.name >= 12, `a name is at least 12 px on screen at ${zoom}`);
        assert.ok(16 * zoom * levels.header >= 13, `a region header is at least 13 px on screen at ${zoom}`);
        assert.ok(14 * zoom * levels.lane >= 13, `a lane label is at least 13 px on screen at ${zoom}`);
        assert.ok(zoom * 16 * 1 < 12 || levels.name === 1, 'a label that is large enough is not scaled');
        assert.ok(1.6 <= zoom * levels.ring, `the status ring stays at least 1.6 px at ${zoom}`);
    }
    // A24: the compact icon (its 24 px image) is counter-scaled to min(20 px, 0.8 x the 64 px card height) on screen, in steps of 0.05, at most 2.1.
    assert.deepEqual([0.25, 0.4, 0.44, 0.5, 0.64].map(zoom => model.zoomLevels(zoom).icon), [2.1, 2.1, 1.9, 1.7, 1.35], 'hand computed: 20 / (24 zoom) rounded up to 0.05 and capped at 2.1');
    for (const zoom of [0.25, 0.3, 0.39, 0.44, 0.5, 0.6, 0.64]) {
        const levels = model.zoomLevels(zoom), shown = 24 * levels.icon * zoom, wanted = Math.min(20, 0.8 * 64 * zoom);
        assert.ok(shown >= wanted - 0.5 && shown <= wanted + 24 * 0.05 * zoom + 1e-9, `the compact icon is about ${wanted.toFixed(1)} px on screen at ${zoom}, not ${shown.toFixed(1)}`);
        assert.ok(shown >= 18 || zoom < 0.4, 'from zoom 0.4 on the icon is at least 18 px');
        assert.ok(levels.iconRing * levels.icon * zoom >= 1.6, `the status ring of the scaled icon stays at least 1.6 px at ${zoom}`);
    }
    assert.equal(model.zoomLevels(1).icon, 1, 'the icon is never scaled below 1');
    assert.equal(model.zoomLevels(1).name, 1, 'at 100 % labels are not scaled');
    assert.equal(model.zoomLevels(2).header, 1, 'above 1.0 labels scale with the scene');
    assert.ok(Math.abs(model.labelScale(0.3, 16, 12) * 20 - Math.round(model.labelScale(0.3, 16, 12) * 20)) < 1e-9, 'the counter-scale moves in steps of 0.05');
    assert.equal(model.labelScale(NaN, 16, 12), 1, 'a broken zoom is not scaled');
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

// ---- source labels (A15): never the opaque instance id -----------------------
{
    const kindLabel = kind => ({ docker: 'Docker', registry: 'Registry' })[kind] || kind;
    const sources = [
        { kind: 'docker', instance: 'b9f3c2d1e8a74f6b', key: 'c1' }, { kind: 'registry', instance: 'inventory', key: 'r1' },
        { kind: 'docker', instance: '0a1b2c3d4e5f6071', key: 'c2' }, { kind: 'docker', instance: 'b9f3c2d1e8a74f6b', key: 'c3' },
    ];
    const labels = model.sourceLabels(sources, kindLabel);
    assert.deepEqual(plain([...labels.several]), ['docker'], 'only a kind with several instances is numbered');
    assert.equal(labels.of(sources[2]).text, 'Docker 1', 'instances are numbered in instance order, not in arrival order');
    assert.equal(labels.of(sources[0]).text, 'Docker 2');
    assert.equal(labels.of(sources[3]).text, 'Docker 2', 'every row of one instance shares its number');
    assert.equal(labels.of(sources[1]).text, 'Registry', 'a kind with one instance keeps its plain name');
    for (const source of sources) assert.ok(!labels.of(source).text.includes(source.instance), 'the instance id never shows in the visible text');
    assert.equal(labels.of(sources[0]).title, 'Docker · b9f3c2d1e8a74f6b', 'the instance id only goes into the tooltip');
    const reordered = model.sourceLabels([...sources].reverse(), kindLabel);
    assert.equal(reordered.of(sources[2]).text, 'Docker 1', 'the numbering survives a different poll order');
    const ignored = model.sourceLabels([...sources, { kind: 'docker', instance: 'zzzz', key: 'c9', ignored: true }], kindLabel);
    assert.equal(ignored.of(sources[0]).text, 'Docker 2', 'an ignored instance takes no number');
    assert.equal(model.sourceLabels([{ kind: 'docker', instance: 'solo' }], kindLabel).of({ kind: 'docker', instance: 'solo' }).text, 'Docker', 'a single instance is not numbered');
}

// ---- canvas CSS rules (A9) -----------------------------------------------------
const canvasCSS = fs.readFileSync(path.join(root, 'ui/css/desktop-app-infra-canvas.css'), 'utf8');
const cssRule = selector => { const at = canvasCSS.indexOf(`${selector} {`); return at < 0 ? '' : canvasCSS.slice(at, canvasCSS.indexOf('}', at)); };
{
    const focus = cssRule('.infra-app .infra-viewport[data-stage]:focus-visible');
    assert.ok(focus, 'the canvas has its own keyboard focus rule');
    assert.ok(/box-shadow: inset 0 0 0 1px var\(--vd-theme-accent-border\)/.test(focus), 'the canvas focus is a 1px inset accent hairline');
    assert.ok(/outline: var\(--ds-focus-ring-width\) solid/.test(focus) && focus.includes('--ds-color-border-focus'), 'keyboard focus keeps the standard ring tokens');
    assert.ok(!/\.infra-viewport:focus\s*\{/.test(canvasCSS), 'pointer focus draws nothing on the canvas');
}
// A4: regions are neutral; only the drop target, the selection and the hovered header carry the accent.
{
    const region = cssRule('.infra-region');
    assert.ok(region.includes('border: calc(var(--infra-ring, 2px) * .75) solid var(--infra-outline)'), 'a region is outlined with the visible outline token, 1.5 px on screen');
    assert.ok(fs.readFileSync(path.join(root, 'ui/css/desktop-app-infra.css'), 'utf8').includes('--infra-outline: color-mix(in srgb, var(--vd-text) 16%, var(--vd-theme-border))'), 'A25: the outline mixes the theme text colour into the theme border so it shows in dark themes');
    assert.ok(region.includes('background: color-mix(in srgb, var(--vd-theme-panel-bg) 45%, transparent)'), 'a region is filled with a neutral panel tint');
    assert.ok(!/var\(--vd-accent\)/.test(region) && !/\.infra-region\[data-depth/.test(canvasCSS), 'the accent never tints a plain region');
    for (const selector of ['.infra-region.is-selected', '.infra-region.is-drop-target', '.infra-region-header:hover']) assert.ok(/accent/.test(cssRule(selector)), `${selector} keeps the accent`);
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

    { // the List checkboxes mark their selection; every other way of selecting resets the mark
        const store = newStore(fakeApi({}).api);
        store.applySnapshot({ nodes: [storeNode('a'), storeNode('b')], edges: [], sources: [], capabilities: { write: true } });
        store.select(['a'], { toggle: true, origin: 'check' });
        assert.equal(store.state.selectionOrigin, 'check');
        store.select(['b'], { toggle: true, origin: 'check' });
        assert.equal(store.state.selectionOrigin, 'check');
        assert.deepEqual([...store.state.selection], ['a', 'b']);
        store.select(['a']);
        assert.equal(store.state.selectionOrigin, '', 'selecting by name is not a checkbox selection');
        store.select(['b'], { origin: 'check' });
        store.clearSelection();
        assert.equal(store.state.selectionOrigin, '');
        store.dispose();
    }
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

// ---- store: ignore, batch import, removal, parents, unplace undo ------------------
{
    const InfraStore = window.InfraAppStore;
    const node = (id, extra = {}) => ({ id, version: 1, kind: 'device', name: id, icon: 'router', profile: {}, placements: { atlas: { x: 0, y: 0, pinned: true } }, sources: [], ...extra });
    const recorder = routes => {
        const calls = [];
        const api = async (url, options = {}) => {
            const [pathPart, query = ''] = String(url).replace('/api/desktop/infra', '').split('?');
            const method = options.method || 'GET';
            const body = typeof options.body === 'string' ? JSON.parse(options.body) : undefined;
            calls.push({ route: `${method} ${pathPart}`, query, body });
            const handler = routes[`${method} ${pathPart}`] || routes[`${method} ${pathPart.replace(/\/nodes\/[^/]+$/, '/nodes/:id')}`];
            if (!handler) throw Object.assign(new Error('not_found'), { status: 404, body: { error: 'not_found' } });
            return handler(body, { query, path: pathPart });
        };
        return { api, calls };
    };
    const snap = (nodes, extra = {}) => ({ nodes, edges: [], sources: [], source_hosts: [], capabilities: { write: true }, ...extra });
    const newStore = (api, ctx = {}) => InfraStore.create({ ctx: { api, ...ctx }, t: key => key });
    {
        const { api, calls } = recorder({ 'GET /snapshot': () => snap([node('a')], { sources: [{ kind: 'registry', instance: 'inventory', key: 'k1', name: 'TV', state: 'current', ignored: true }] }) });
        const store = newStore(api);
        assert.equal(await store.reloadSnapshot(), true);
        assert.match(calls[0].query, /(^|&)include_ignored=1(&|$)/, 'snapshots ask for ignored sources');
        assert.equal(store.state.sources[0].ignored, true);
        assert.equal(store.features.unplace, true);
        store.applySnapshot(snap([node('a')], { source_hosts: undefined }));
        assert.equal(store.features.unplace, false, 'a server without source_hosts has no unplace either');
        store.dispose();
    }
    {
        const identity = { kind: 'registry', instance: 'inventory', key: 'k1' };
        const { api, calls } = recorder({ 'PUT /sources/ignore': body => ({ sources: body.sources.map(item => ({ ...item, name: 'TV', state: 'current', ignored: body.ignored })) }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('a')]));
        store.state.candidates = [{ id: 'cand-1', source: { ...identity, state: 'current' }, node: { kind: 'device', name: 'TV' } }];
        const events = [];
        store.subscribe(change => events.push(change.type));
        assert.equal(await store.setSourcesIgnored([identity], true), true);
        assert.deepEqual(plain(calls[0].body), { sources: [identity], ignored: true });
        assert.equal(store.state.candidates.length, 0);
        assert.equal(store.state.sources.find(source => source.key === 'k1').ignored, true);
        assert.ok(events.includes('sources') && events.includes('candidates'));
        assert.equal(await store.setSourcesIgnored([identity], false), true);
        assert.equal(store.state.sources.find(source => source.key === 'k1').ignored, false, 'restoring updates the same row');
        assert.equal(store.state.sources.length, 1);
        store.dispose();
    }
    {
        const { api, calls } = recorder({ 'POST /import/batch': body => ({ nodes: [node('imported-1', { parent_id: 'room', placements: { atlas: { x: 48, y: 72, pinned: true } } })], errors: [{ candidate_id: body.items[1].candidate_id, error: 'source_stale', message: 'x' }] }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('room', { kind: 'room' })]));
        store.state.candidates = [{ id: 'c1', source: {}, node: {} }, { id: 'c2', source: {}, node: {} }];
        const result = await store.importBatch([{ candidate_id: 'c1', parent_id: 'room', placement: { view: 'atlas', x: 48, y: 72 } }, { candidate_id: 'c2' }]);
        assert.deepEqual(plain(calls[0].body.items[0]), { candidate_id: 'c1', parent_id: 'room', placement: { view: 'atlas', x: 48, y: 72 } });
        assert.deepEqual(plain(result.nodes.map(item => item.id)), ['imported-1']);
        assert.deepEqual(plain(result.errors.map(item => [item.candidate_id, item.error])), [['c2', 'source_stale']]);
        assert.equal(store.nodeById('imported-1').parent_id, 'room');
        assert.deepEqual(plain(store.state.candidates.map(item => item.id)), ['c2'], 'a failed discovery stays');
        store.dispose();
    }
    { // an unconfirmed import reloads, tells the user and reports uncertain
        const { api, calls } = recorder({ 'POST /import/batch': () => { throw Object.assign(new Error('boom'), { status: 502 }); }, 'GET /snapshot': () => snap([node('a')]) });
        const store = newStore(api);
        store.applySnapshot(snap([node('a')]));
        store.state.candidates = [{ id: 'c1', source: {}, node: {} }];
        const notices = [];
        store.subscribe(change => { if (change.type === 'notice') notices.push(change.message); });
        const result = await store.importBatch([{ candidate_id: 'c1' }]);
        assert.equal(result.uncertain, true);
        assert.deepEqual(notices, ['desktop.infra_write_outcome_unknown']);
        assert.equal(calls.filter(call => call.route === 'GET /snapshot').length, 1, 'the latest state is reloaded');
        assert.equal(store.state.candidates.length, 1, 'nothing is assumed imported');
        store.dispose();
    }
    {
        const source = { kind: 'registry', instance: 'inventory', key: 'k9', name: 'Old NAS', state: 'current' };
        const { api, calls } = recorder({ 'DELETE /nodes/:id': (_, { path }) => ({ status: 'deleted', id: path.split('/').pop(), ignored: [{ kind: 'registry', instance: 'inventory', key: 'k9' }] }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('nas', { version: 3, sources: [source] }), node('other')]));
        assert.deepEqual(plain(await store.removeNodes(['nas'], { ignoreSources: true })), ['nas']);
        assert.equal(calls[0].route, 'DELETE /nodes/nas');
        assert.equal(calls[0].query, 'version=3&ignore_sources=1');
        assert.equal(store.nodeById('nas'), undefined);
        assert.deepEqual(plain(store.state.sources.filter(item => item.ignored).map(item => [item.key, item.name])), [['k9', 'Old NAS']]);
        store.dispose();
    }
    { // removal stops at the first refusal and keeps what it removed
        const { api, calls } = recorder({ 'DELETE /nodes/:id': (_, { path }) => { if (path.endsWith('/b')) throw Object.assign(new Error('conflict'), { status: 409, body: { error: 'conflict' } }); return { status: 'deleted', id: path.split('/').pop() }; } });
        const store = newStore(api);
        store.applySnapshot(snap([node('a'), node('b'), node('c')]));
        assert.deepEqual(plain(await store.removeNodes(['a', 'b', 'c'])), ['a']);
        assert.equal(calls.length, 2, 'c is never tried');
        assert.deepEqual(plain(store.state.nodes.map(item => item.id)), ['b', 'c']);
        store.dispose();
    }
    {
        const puts = [];
        const { api } = recorder({ 'PUT /nodes/:id': body => { puts.push([body.id, body.parent_id, body.version]); return { node: { ...body, version: body.version + 1 } }; } });
        const store = newStore(api);
        store.applySnapshot(snap([node('a', { placements: {} }), node('room', { kind: 'room' })]));
        store.state.drafts.set('a', { ...store.nodeById('a'), name: 'Draft' });
        store.state.dirty.add('a');
        assert.equal(await store.dispatch(InfraStore.commands.setParent([{ id: 'a', before: '', after: 'room' }], 'moved')), true);
        assert.equal(store.nodeById('a').parent_id, 'room');
        assert.equal(store.state.drafts.get('a').version, 2, 'a dirty draft follows the version');
        assert.equal(store.state.drafts.get('a').parent_id, 'room');
        assert.equal(store.state.drafts.get('a').name, 'Draft');
        assert.equal(await store.undo('atlas'), true);
        assert.equal(store.nodeById('a').parent_id, '');
        assert.deepEqual(puts, [['a', 'room', 1], ['a', '', 2]]);
        store.dispose();
    }
    {
        const { api, calls } = recorder({ 'PUT /layout': body => ({ nodes: body.nodes.map(item => node(item.id, { version: item.version + 1, placements: item.unplace ? {} : { atlas: { x: item.x, y: item.y, pinned: true } } })) }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('a', { placements: {} })]));
        const after = new Map([['a', { x: 96, y: 48, pinned: true }]]);
        const command = InfraStore.commands.placeFromLibrary('atlas', after, 'placed', new Map([['a', { unplace: true }]]));
        assert.equal(command.undoable, true, 'a caller that states the previous intents gets an undoable command');
        await store.dispatch(command);
        await store.whenLayoutIdle('atlas');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 96);
        assert.equal(await store.undo('atlas'), true);
        assert.equal(store.effectivePlacement('a', 'atlas'), null, 'undo hides the card at once');
        await store.whenLayoutIdle('atlas');
        assert.equal(store.nodeById('a').placements.atlas, undefined);
        assert.deepEqual(plain(calls.map(call => call.body.nodes[0])), [{ id: 'a', version: 1, x: 96, y: 48, pinned: true }, { id: 'a', version: 2, unplace: true }]);
        assert.equal(InfraStore.commands.placeFromLibrary('atlas', after, '', new Map()).undoable, false);
        store.dispose();
    }
    { // an old server (no source_hosts) never gets an unplace, even when the caller states one
        const { api } = recorder({ 'PUT /layout': body => ({ nodes: body.nodes.map(item => node(item.id, { version: item.version + 1, placements: { atlas: { x: item.x, y: item.y, pinned: true } } })) }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('a', { placements: {} })], { source_hosts: undefined }));
        await store.dispatch(InfraStore.commands.placeFromLibrary('atlas', new Map([['a', { x: 96, y: 48, pinned: true }]]), 'placed', new Map([['a', { unplace: true }]])));
        await store.whenLayoutIdle('atlas');
        assert.equal(store.canUndo('atlas'), false);
        store.dispose();
    }
    { // the unplace command takes assets off the map and undo puts them back where they were
        const { api, calls } = recorder({ 'PUT /layout': body => ({ nodes: body.nodes.map(item => node(item.id, { version: item.version + 1, placements: item.unplace ? {} : { atlas: { x: item.x, y: item.y, pinned: true } } })) }) });
        const store = newStore(api);
        store.applySnapshot(snap([node('a', { placements: { atlas: { x: 96, y: 48, pinned: true } } })]));
        const before = new Map([['a', store.currentIntent('a', 'atlas', false)]]);
        await store.dispatch(InfraStore.commands.unplace('atlas', before, 'removed'));
        assert.equal(store.effectivePlacement('a', 'atlas'), null);
        await store.whenLayoutIdle('atlas');
        assert.equal(store.nodeById('a').placements.atlas, undefined);
        assert.equal(await store.undo('atlas'), true);
        assert.equal(store.effectivePlacement('a', 'atlas').x, 96);
        await store.whenLayoutIdle('atlas');
        assert.deepEqual(plain(calls.map(call => call.body.nodes[0])), [{ id: 'a', version: 1, unplace: true }, { id: 'a', version: 2, x: 96, y: 48, pinned: true }]);
        store.dispose();
    }
    {
        const { api, calls } = recorder({
            'PUT /source-hosts': body => ({ source_host: { ...body, version: body.version + 1 } }),
            'GET /snapshot': () => snap([node('ct', { kind: 'container', host_id: 'host' }), node('host')], { source_hosts: [{ kind: 'docker', instance: 'engine-a', node_id: 'host', version: 5 }] }),
        });
        const store = newStore(api);
        store.applySnapshot(snap([node('ct', { kind: 'container' }), node('host')], { source_hosts: [{ kind: 'docker', instance: 'engine-a', node_id: '', version: 4, inferred_node_id: '' }] }));
        assert.equal(await store.setSourceHost('engine-a', 'host'), true);
        assert.deepEqual(plain(calls[0].body), { kind: 'docker', instance: 'engine-a', node_id: 'host', version: 4 });
        assert.equal(store.guests().hostOf.get('ct'), 'host');
        store.dispose();
    }
    { // a stale engine host version reloads and tells the user; read-only stores never write
        const { api, calls } = recorder({ 'PUT /source-hosts': () => { throw Object.assign(new Error('conflict'), { status: 409, body: { error: 'conflict' } }); }, 'GET /snapshot': () => snap([node('a')]) });
        const store = newStore(api);
        store.applySnapshot(snap([node('a')], { source_hosts: [{ kind: 'docker', instance: 'engine-a', node_id: '', version: 1 }] }));
        assert.equal(await store.setSourceHost('engine-a', 'a'), false);
        assert.equal(calls.filter(call => call.route === 'GET /snapshot').length, 1);
        store.dispose();
        const readonly = recorder({});
        const locked = newStore(readonly.api, { readonly: true });
        locked.applySnapshot(snap([node('a')]));
        assert.equal(await locked.setSourceHost('engine-a', 'a'), false);
        assert.equal(await locked.setSourcesIgnored([{ kind: 'registry', instance: 'i', key: 'k' }], true), false);
        assert.deepEqual(plain(await locked.removeNodes(['a'])), []);
        assert.equal((await locked.importBatch([{ candidate_id: 'c1' }])).nodes.length, 0);
        assert.equal(await locked.updateParents([['a', 'room']]), false);
        assert.equal(readonly.calls.length, 0, 'a read-only desktop sends nothing');
        locked.dispose();
    }
}

// ---- store: stale polls, request timeouts, repeated poll failures -------------------
{
    const InfraStore = window.InfraAppStore;
    const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
    const tick = () => sleep(0);
    const node = (version, x, extra = {}) => ({ id: 'a', version, kind: 'device', name: 'A', icon: 'router', profile: {}, placements: { atlas: { x, y: 0, pinned: true } }, sources: [], ...extra });
    const snap = (nodes, extra = {}) => ({ nodes, edges: [], sources: [], source_hosts: [], capabilities: { write: true }, ...extra });
    // A fake API that holds every request until the test answers it, and rejects like fetch when aborted.
    const held = () => {
        const pending = [];
        const api = (endpoint, options = {}) => new Promise((resolve, reject) => {
            pending.push({ endpoint, method: options.method || 'GET', body: typeof options.body === 'string' ? JSON.parse(options.body) : options.body, resolve, reject });
            options.signal?.addEventListener('abort', () => reject(new DOMException('The operation was aborted.', 'AbortError')));
        });
        const take = predicate => { const index = pending.findIndex(predicate); assert.ok(index >= 0, 'expected a pending request'); return pending.splice(index, 1)[0]; };
        const isSnapshot = request => request.method === 'GET' && request.endpoint.includes('/snapshot');
        return { api, pending, take, isSnapshot };
    };
    const noticesOf = store => { const notices = []; store.subscribe(change => { if (change.type === 'notice') notices.push(change.message); }); return notices; };
    const failure = status => Object.assign(new Error('boom'), { status, body: { error: 'boom' } });

    { // a poll that started before a committed write never rolls the write back
        const { api, take, isSnapshot, pending } = held();
        const store = InfraStore.create({ ctx: { api }, t: key => key });
        store.applySnapshot(snap([node(1, 0)]));
        store.state.visible = true;
        const poll = store.loadSnapshot(false);
        await tick();
        const olderPoll = take(isSnapshot);
        const move = store.dispatch(InfraStore.commands.moveNodes('atlas', new Map([['a', { x: 0, y: 0, pinned: true }]]), new Map([['a', { x: 48, y: 0, pinned: true }]])));
        await tick();
        const put = take(request => request.method === 'PUT');
        assert.equal(put.body.nodes[0].version, 1);
        put.resolve({ nodes: [node(2, 48)] });
        await move;
        assert.equal(store.nodeById('a').version, 2);
        olderPoll.resolve(snap([node(1, 0)]));
        await poll;
        assert.equal(store.nodeById('a').version, 2, 'the older poll answer is discarded');
        assert.equal(store.effectivePlacement('a', 'atlas').x, 48);
        assert.equal(store.canUndo('atlas'), true, 'the undo history survives');
        assert.equal(pending.length, 0, 'the re-poll is not instant');
        await sleep(400);
        const repoll = take(isSnapshot);
        assert.doesNotMatch(repoll.endpoint, /refresh=1/, 'a quiet poll stays quiet');
        repoll.resolve(snap([node(2, 48)]));
        await sleep(20);
        assert.equal(store.state.sync, 'live');
        const next = store.dispatch(InfraStore.commands.moveNodes('atlas', new Map([['a', { x: 48, y: 0, pinned: true }]]), new Map([['a', { x: 96, y: 0, pinned: true }]])));
        await tick();
        const second = take(request => request.method === 'PUT');
        assert.equal(second.body.nodes[0].version, 2, 'the next move carries the committed version');
        second.resolve({ nodes: [node(3, 96)] });
        await next;
        assert.equal(store.canUndo('atlas'), true);
        store.dispose();
    }
    { // an explicit refresh that raced a write is asked again as an explicit refresh
        const { api, take, isSnapshot } = held();
        const store = InfraStore.create({ ctx: { api }, t: key => key });
        store.applySnapshot(snap([node(1, 0)]));
        store.state.visible = true;
        const refresh = store.loadSnapshot(true);
        await tick();
        const older = take(isSnapshot);
        assert.match(older.endpoint, /refresh=1/);
        const move = store.queueLayout('atlas', new Map([['a', { x: 48, y: 0, pinned: true }]]));
        await tick();
        take(request => request.method === 'PUT').resolve({ nodes: [node(2, 48)] });
        await move;
        older.resolve(snap([node(1, 0)]));
        await refresh;
        assert.equal(store.nodeById('a').version, 2);
        await sleep(400);
        const again = take(isSnapshot);
        assert.match(again.endpoint, /refresh=1/, 'the user still gets the explicit refresh');
        again.resolve(snap([node(2, 48)]));
        await sleep(20);
        assert.equal(store.nodeById('a').version, 2);
        store.dispose();
    }
    { // edge, node and ignore writes count as writes too
        const { api, take, isSnapshot } = held();
        const store = InfraStore.create({ ctx: { api }, t: key => key });
        store.applySnapshot(snap([node(1, 0)]));
        store.state.visible = true;
        const poll = store.loadSnapshot(false);
        await tick();
        const olderPoll = take(isSnapshot);
        const create = store.createEdge({ from: 'a', to: 'b', kind: 'ethernet' });
        await tick();
        take(request => request.method === 'POST').resolve({ edge: { id: 'e1', version: 1, from: 'a', to: 'b', kind: 'ethernet' } });
        await create;
        olderPoll.resolve(snap([node(1, 0)]));
        await poll;
        assert.equal(store.state.edges.length, 1, 'an edge created after the poll started survives its answer');
        store.dispose();
    }
    { // a request that hits the timeout is an error, never a quiet abort
        const { api } = held();
        const store = InfraStore.create({ ctx: { api }, t: key => (key === 'desktop.infra_error_timeout' ? 'Timed out' : key), requestTimeout: 15 });
        const notices = noticesOf(store);
        store.state.visible = true;
        await store.loadSnapshot(false);
        assert.equal(store.state.sync, 'error', 'a hung server is not shown as live');
        assert.deepEqual(notices, ['Timed out']);
        assert.equal(store.state.busy.has('snapshot'), false);
        const timedOut = await store.request('/nodes', { method: 'POST', body: {} }).catch(error => error);
        assert.equal(timedOut.name, 'TimeoutError');
        assert.equal(store.uncertainMutation(timedOut), true, 'a timed-out write has an unknown outcome');
        assert.equal(store.uncertainMutation(new DOMException('aborted', 'AbortError')), true);
        assert.equal(store.uncertainMutation(Object.assign(new Error('bad'), { status: 400 })), false);
        store.dispose();
    }
    { // a timed-out write reloads and reports the unknown outcome; a deliberate abort stays quiet
        const { api, take, isSnapshot, pending } = held();
        const store = InfraStore.create({ ctx: { api }, t: key => key, requestTimeout: 60 });
        store.applySnapshot(snap([node(1, 0)]));
        store.state.visible = true;
        const notices = noticesOf(store);
        const imported = store.importBatch([{ candidate_id: 'c1' }]);
        while (!pending.some(isSnapshot)) await sleep(2);
        const reload = take(isSnapshot);
        reload.resolve(snap([node(1, 0)]));
        const result = await imported;
        assert.equal(result.uncertain, true);
        assert.deepEqual(notices, ['desktop.infra_write_outcome_unknown']);
        notices.length = 0;
        const poll = store.loadSnapshot(false);
        await tick();
        store.abortRequests();
        await poll;
        assert.deepEqual(notices, [], 'hiding or closing the window aborts without a notice');
        assert.notEqual(store.state.sync, 'error');
        store.dispose();
    }
    { // repeated poll failures notify once; recovery announces once
        let failing = true;
        const flaky = async (endpoint, options) => { if (failing) throw failure(502); return snap([node(1, 0)]); };
        const store = InfraStore.create({ ctx: { api: flaky }, t: key => key });
        const notices = noticesOf(store);
        const announced = [];
        store.subscribe(change => { if (change.type === 'sync' && change.announce) announced.push(change.announce); });
        store.state.visible = true;
        for (let index = 0; index < 3; index += 1) await store.loadSnapshot(false);
        assert.equal(store.state.sync, 'error');
        assert.equal(notices.length, 1, 'three failed polls make one notice');
        await store.loadSnapshot(true);
        assert.equal(notices.length, 2, 'a refresh the user asked for always answers');
        failing = false;
        await store.loadSnapshot(false);
        assert.equal(store.state.sync, 'live');
        assert.deepEqual(announced, ['desktop.infra_sync_recovered']);
        failing = true;
        await store.loadSnapshot(false);
        await store.loadSnapshot(false);
        assert.equal(notices.length, 3, 'a new outage notifies again');
        assert.deepEqual(announced, ['desktop.infra_sync_recovered', 'desktop.infra_sync_stale']);
        store.dispose();
    }
}

// ---- dialogs: permissions after a confirmation, unconfirmed writes ----------------------
{
    const InfraStore = window.InfraAppStore;
    const gnode = (id, extra = {}) => ({ id, version: 1, kind: 'device', name: id, icon: 'router', profile: {}, placements: { atlas: { x: 0, y: 0, pinned: true } }, sources: [], ...extra });
    const dockerSource = { kind: 'docker', instance: 'engine', key: 'c1', name: 'c1', state: 'current' };
    const snapOf = (nodes, extra = {}) => ({ nodes, edges: [], sources: [], source_hosts: [], capabilities: { write: true, control: true }, ...extra });
    const setup = handlers => {
        const calls = [];
        const api = async (url, options = {}) => {
            const [pathPart, query = ''] = String(url).replace('/api/desktop/infra', '').split('?');
            const method = options.method || 'GET';
            const body = typeof options.body === 'string' ? JSON.parse(options.body) : undefined;
            calls.push({ method, path: pathPart, query, body });
            const handler = handlers[`${method} ${pathPart}`] || handlers[`${method} ${pathPart.replace(/\/(nodes|edges)\/[^/]+/, (_, kind) => `/${kind}/:id`)}`];
            if (!handler) throw Object.assign(new Error('not_found'), { status: 404, body: { error: 'not_found' } });
            return handler(body, { query, path: pathPart });
        };
        const ctx = { api, readonly: false };
        const store = InfraStore.create({ ctx, t: key => key });
        const notices = [];
        store.subscribe(change => { if (change.type === 'notice') notices.push(change.message); });
        const dialogs = window.InfraAppDialogs.create({ store, state: store.state, t: key => key, esc: model.esc, model, q: () => null, root: { isConnected: true }, ctx, safeId: 'x', announce() {} });
        return { ctx, store, dialogs, calls, notices, sent: method => calls.filter(call => call.method === method && call.path !== '/snapshot') };
    };
    const unconfirmed = () => Object.assign(new Error('boom'), { status: 502 });
    const timedOut = () => Object.assign(new Error('timeout'), { name: 'TimeoutError' });
    const sourceKey = model.sourceIdentity(dockerSource);

    { // a confirmed delete is stopped when write access went away while the modal was open
        const { ctx, store, dialogs, sent, notices } = setup({ 'DELETE /nodes/:id': () => ({ status: 'deleted' }) });
        store.applySnapshot(snapOf([gnode('a')]));
        ctx.showConfirmModal = async () => { ctx.readonly = true; return true; };
        await dialogs.deleteNode('a');
        assert.equal(sent('DELETE').length, 0, 'no request after the permission was revoked');
        assert.deepEqual(notices, ['desktop.infra_error_infra_readonly']);
        assert.ok(store.nodeById('a'));
        store.dispose();
    }
    { // a bulk delete re-checks before every request
        let revoke = () => {};
        const { ctx, store, dialogs, sent, notices } = setup({ 'DELETE /nodes/:id': () => { revoke(); return { status: 'deleted' }; } });
        revoke = () => { ctx.readonly = true; };
        store.applySnapshot(snapOf([gnode('a'), gnode('b'), gnode('c')]));
        ctx.showConfirmModal = async () => true;
        await dialogs.deleteNodes(['a', 'b', 'c']);
        assert.equal(sent('DELETE').length, 1, 'the loop stops once the permission is gone');
        assert.deepEqual(notices, ['desktop.infra_error_infra_readonly']);
        assert.deepEqual(plain(store.state.nodes.map(item => item.id)), ['b', 'c']);
        store.dispose();
    }
    { // a delete uses the version the asset has after the confirmation
        const { ctx, store, dialogs, calls } = setup({ 'DELETE /nodes/:id': () => ({ status: 'deleted' }) });
        store.applySnapshot(snapOf([gnode('a')]));
        ctx.showConfirmModal = async () => { store.applySnapshot(snapOf([gnode('a', { version: 4 })])); return true; };
        await dialogs.deleteNode('a');
        assert.match(calls.find(call => call.method === 'DELETE').query, /version=4/);
        store.dispose();
    }
    { // a confirmed action is not sent once control or the action itself went away
        const node = () => gnode('a', { sources: [dockerSource], source_actions: { [sourceKey]: ['stop'] } });
        const first = setup({ 'POST /nodes/:id/actions': () => ({ status: 'completed' }) });
        first.store.applySnapshot(snapOf([node()]));
        first.ctx.showConfirmModal = async () => { first.ctx.readonly = true; return true; };
        await first.dialogs.runAction('a', dockerSource, 'stop');
        assert.equal(first.sent('POST').length, 0);
        assert.deepEqual(first.notices, ['desktop.infra_error_infra_readonly']);
        first.store.dispose();
        const second = setup({ 'POST /nodes/:id/actions': () => ({ status: 'completed' }) });
        second.store.applySnapshot(snapOf([node()]));
        second.ctx.showConfirmModal = async () => { second.store.applySnapshot(snapOf([gnode('a', { sources: [dockerSource], source_actions: { [sourceKey]: [] } })])); return true; };
        await second.dialogs.runAction('a', dockerSource, 'stop');
        assert.equal(second.sent('POST').length, 0);
        assert.deepEqual(second.notices, ['desktop.infra_action_unavailable']);
        second.store.dispose();
        const third = setup({ 'POST /nodes/:id/actions': () => ({ status: 'completed' }) });
        third.store.applySnapshot(snapOf([node()]));
        third.ctx.showConfirmModal = async () => true;
        await third.dialogs.runAction('a', dockerSource, 'stop');
        assert.equal(third.sent('POST').length, 1, 'an untouched permission still sends');
        third.store.dispose();
    }
    { // an unconfirmed delete or create reloads and says so; a deliberate abort stays quiet
        for (const failure of [unconfirmed, timedOut]) {
            const { ctx, store, dialogs, calls, notices } = setup({ 'DELETE /nodes/:id': () => { throw failure(); }, 'POST /nodes': () => { throw failure(); }, 'GET /snapshot': () => snapOf([gnode('a')]) });
            store.applySnapshot(snapOf([gnode('a')]));
            ctx.showConfirmModal = async () => true;
            await dialogs.deleteNode('a');
            assert.deepEqual(notices, ['desktop.infra_write_outcome_unknown']);
            assert.equal(calls.filter(call => call.path === '/snapshot').length, 1, 'the latest state is reloaded');
            notices.length = 0;
            await dialogs.createNode(new Map([['kind', 'device'], ['name', 'New'], ['icon', 'router']]), '1');
            assert.deepEqual(notices, ['desktop.infra_write_outcome_unknown']);
            assert.equal(calls.filter(call => call.path === '/snapshot').length, 2);
            assert.equal(store.state.busy.has('create'), false);
            store.dispose();
        }
        const quiet = setup({ 'POST /nodes': () => { throw new DOMException('aborted', 'AbortError'); }, 'GET /snapshot': () => snapOf([gnode('a')]) });
        quiet.store.applySnapshot(snapOf([gnode('a')]));
        await quiet.dialogs.createNode(new Map([['kind', 'device'], ['name', 'New'], ['icon', 'router']]), '1');
        assert.deepEqual(quiet.notices, []);
        assert.equal(quiet.calls.filter(call => call.path === '/snapshot').length, 0);
        const rejected = setup({ 'POST /nodes': () => { throw Object.assign(new Error('invalid_request'), { status: 400, body: { error: 'invalid_request' } }); } });
        rejected.store.applySnapshot(snapOf([gnode('a')]));
        await rejected.dialogs.createNode(new Map([['kind', 'device'], ['name', 'New'], ['icon', 'router']]), '1');
        assert.deepEqual(rejected.notices, ['desktop.infra_error_infra_error'], 'a plain refusal keeps its own message');
        quiet.store.dispose(); rejected.store.dispose();
    }
    { // an unconfirmed connection reloads; one that arrived is adopted, with its id in the undo entry
        const arrived = { id: 'e9', version: 1, from: 'a', to: 'b', kind: 'ethernet' };
        let edges = [arrived];
        const { store, dialogs, calls, notices, sent } = setup({ 'POST /edges': () => { throw unconfirmed(); }, 'GET /snapshot': () => snapOf([gnode('a'), gnode('b')], { edges }), 'DELETE /edges/:id': () => ({ status: 'deleted' }) });
        store.applySnapshot(snapOf([gnode('a'), gnode('b')]));
        await dialogs.saveEdge(new Map([['id', ''], ['from', 'a'], ['to', 'b'], ['kind', 'ethernet']]), '1');
        assert.equal(notices[0], 'desktop.infra_write_outcome_unknown');
        assert.equal(calls.filter(call => call.path === '/snapshot').length, 1);
        assert.equal(store.state.edges.length, 1);
        assert.equal(store.canUndo('atlas'), true, 'the connection that arrived can be undone');
        assert.equal(await store.undo('atlas'), true);
        assert.equal(sent('DELETE')[0].path, '/edges/e9');
        store.dispose();
        edges = [];
        const lost = setup({ 'POST /edges': () => { throw unconfirmed(); }, 'GET /snapshot': () => snapOf([gnode('a'), gnode('b')], { edges }) });
        lost.store.applySnapshot(snapOf([gnode('a'), gnode('b')]));
        await lost.dialogs.saveEdge(new Map([['id', ''], ['from', 'a'], ['to', 'b'], ['kind', 'ethernet']]), '1');
        assert.deepEqual(lost.notices, ['desktop.infra_write_outcome_unknown']);
        assert.equal(lost.store.canUndo('atlas'), false, 'nothing arrived, nothing to undo');
        lost.store.dispose();
    }
    { // editing a connection that could not be confirmed reloads too
        const { store, dialogs, calls, notices } = setup({ 'PUT /edges/:id': () => { throw unconfirmed(); }, 'GET /snapshot': () => snapOf([gnode('a'), gnode('b')], { edges: [{ id: 'e1', version: 1, from: 'a', to: 'b', kind: 'ethernet' }] }) });
        store.applySnapshot(snapOf([gnode('a'), gnode('b')], { edges: [{ id: 'e1', version: 1, from: 'a', to: 'b', kind: 'ethernet' }] }));
        await dialogs.saveEdge(new Map([['id', 'e1'], ['version', '1'], ['from', 'a'], ['to', 'b'], ['kind', 'fiber']]), '1');
        assert.deepEqual(notices, ['desktop.infra_write_outcome_unknown']);
        assert.equal(calls.filter(call => call.path === '/snapshot').length, 1);
        store.dispose();
    }
}

// ---- keyboard move mode: assets and places, revert unless Enter ---------------------------
{
    const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
    const gestureSource = ['infra-model.js', 'infra-lanes.js', 'infra-store.js', 'infra-gestures.js'].map(file => fs.readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8')).join('\n');
    // A world with a fake viewport: key and pointer events go to the handlers gestures.attach() registered.
    const world = ({ nodes, selection = [], focus = null, view = 'atlas', sourceHosts = [], readonly = false }) => {
        class FakeElement {}
        const sandbox = { window: { BUILD_VERSION: 'test', btoa: value => Buffer.from(value, 'binary').toString('base64') }, console, setTimeout, clearTimeout, DOMException, TextEncoder, Map, Set, JSON, Math, Date, URL, AbortController, document: { activeElement: null }, Element: FakeElement, requestAnimationFrame: fn => setTimeout(fn, 0), cancelAnimationFrame: clearTimeout };
        vm.runInNewContext(gestureSource, sandbox);
        const { InfraAppModel: gm, InfraAppLanes: gl, InfraAppStore: GS, InfraAppGestures } = sandbox.window;
        const puts = [];
        const api = async (url, options = {}) => {
            const body = typeof options.body === 'string' ? JSON.parse(options.body) : undefined;
            if ((options.method || 'GET') !== 'PUT' || !String(url).endsWith('/layout')) throw Object.assign(new Error('not_found'), { status: 404 });
            puts.push(body);
            return { nodes: body.nodes.map(item => ({ ...nodes.find(node => node.id === item.id), version: item.version + 1, placements: { [body.view]: { x: item.x, y: item.y, w: item.w, h: item.h, pinned: true } } })) };
        };
        const ctx = { api, readonly };
        const store = GS.create({ ctx, t: key => key });
        store.applySnapshot({ nodes, edges: [], sources: [], source_hosts: sourceHosts, capabilities: { write: true } });
        store.setView(view);
        store.state.selection = new Set(selection);
        const listeners = new Map(), elements = new Set(), cardEls = new Map(), headerEls = new Map();
        const viewport = { classList: { add() {}, remove() {}, toggle() {} }, addEventListener: (type, fn) => listeners.set(type, fn), removeEventListener() {}, setPointerCapture() {}, contains: node => node === viewport || elements.has(node), getBoundingClientRect: () => ({ left: 0, top: 0, width: 1000, height: 800 }) };
        const element = (id, kind) => {
            const el = new FakeElement();
            el.dataset = kind === 'card' ? { node: id } : { regionHeader: id };
            el.closest = selector => (selector.split(',').map(part => part.trim()).some(part => (part === '[data-node]' && kind === 'card') || (part === '[data-region-header]' && kind === 'region')) ? el : null);
            el.focus = () => { sandbox.document.activeElement = el; };
            elements.add(el);
            return el;
        };
        // Topology draws unplaced assets at computed slots, as the canvas does.
        const scene = () => {
            const auto = view === 'topology' ? gl.topologyAutoPositions(store.state.nodes, store.state.edges, { placementOf: id => store.effectivePlacement(id, view), guests: store.guests() }) : new Map();
            return gm.buildScene({ nodes: store.state.nodes, edges: [], view, placementOf: id => store.effectivePlacement(id, view) || auto.get(id) || null, parentOf: id => store.effectiveParent(id, view), guests: store.guests() });
        };
        const first = scene();
        for (const card of first.cards) cardEls.set(card.id, element(card.id, 'card'));
        for (const id of first.regionRects.keys()) headerEls.set(id, element(id, 'region'));
        const temps = [], laneTargets = [];
        let cleared = 0;
        const canvas = { viewport, sceneData: scene, cardEl: id => cardEls.get(id), regionEl: id => (headerEls.has(id) ? { querySelector: () => headerEls.get(id) } : null), moveTemp: (cards, regions = new Map()) => temps.push({ cards: new Map(cards), regions: new Map(regions) }), clearTemp: () => { cleared += 1; }, setDropTarget() {}, setLaneTarget: index => laneTargets.push(index), renderNow() {}, getView: () => ({ x: 0, y: 0, zoom: 1 }) };
        const announced = [];
        const app = { store, state: store.state, t: key => key, model: gm, lanes: gl, root: { classList: { add() {}, remove() {}, toggle() {} } }, canvas, mapView: () => view, announce: text => announced.push(text), laneLabel: lane => lane, ensureMapView() {}, q: () => null, dialogs: {} };
        const gestures = InfraAppGestures.create(app);
        gestures.attach();
        if (focus) (cardEls.get(focus) || headerEls.get(focus)).focus();
        const target = () => sandbox.document.activeElement || viewport;
        const press = (key, mods = {}) => { const event = { key, shiftKey: false, altKey: false, ctrlKey: false, metaKey: false, ...mods, target: target(), defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }; listeners.get('keydown')(event); return event; };
        return { store, gm, gl, puts, temps, announced, laneTargets, press, cleared: () => cleared, ctx, fire: (type, event) => listeners.get(type)({ target: target(), ...event }), FakeElement, gestures };
    };
    const card = (id, x, y, extra = {}) => ({ id, version: 1, kind: 'device', name: id, icon: 'router', profile: {}, placements: { atlas: { x, y, pinned: true } }, sources: [], ...extra });
    const room = (x, y, w, h) => card('room', x, y, { kind: 'room', placements: { atlas: { x, y, w, h, pinned: true } } });
    const itemOf = (put, id) => put.nodes.find(item => item.id === id);

    { // Enter saves a keyboard move; Escape, Tab, focus loss and a pointer press put it back
        const quit = {
            'Escape': w => w.press('Escape'),
            'Tab': w => w.press('Tab'),
            'focus loss': w => w.fire('focusout', { relatedTarget: null }),
            'pointer press': w => w.fire('pointerdown', { pointerType: 'mouse', button: 0, pointerId: 1, clientX: 5, clientY: 5, target: Object.assign(new w.FakeElement(), { closest: () => null }) }),
        };
        for (const [name, leave] of Object.entries(quit)) {
            const w = world({ nodes: [card('a', 0, 0), card('b', 240, 0)], selection: ['a'], focus: 'a' });
            w.press('m');
            assert.equal(w.store.state.interacting, 1, `${name}: the move mode started`);
            w.press('ArrowRight'); w.press('ArrowRight');
            assert.equal(w.temps.at(-1).cards.get('a').x, 48);
            leave(w);
            assert.equal(w.store.state.interacting, 0, `${name}: the interaction ended`);
            assert.ok(w.cleared() >= 1, `${name}: the preview was removed`);
            assert.equal(w.announced.at(-1), 'desktop.infra_move_cancelled', `${name}: the revert is announced`);
            await sleep(30);
            assert.equal(w.puts.length, 0, `${name}: nothing was saved`);
            assert.equal(w.store.effectivePlacement('a', 'atlas').x, 0);
            w.store.dispose();
        }
        const w = world({ nodes: [card('a', 0, 0)], selection: ['a'], focus: 'a' });
        w.press('m'); w.press('ArrowRight'); w.press('ArrowRight'); w.press('Enter');
        await sleep(30);
        assert.equal(w.puts.length, 1);
        assert.equal(itemOf(w.puts[0], 'a').x, 48);
        w.store.dispose();
    }
    { // a refused M says why
        const nothing = world({ nodes: [card('a', 0, 0)] });
        nothing.press('m');
        assert.deepEqual(plain(nothing.announced), ['desktop.infra_move_nothing']);
        nothing.store.dispose();
        const locked = world({ nodes: [card('a', 0, 0)], selection: ['a'], focus: 'a', readonly: true });
        locked.press('m');
        assert.deepEqual(plain(locked.announced), ['desktop.infra_error_infra_readonly']);
        assert.equal(locked.store.state.interacting, 0);
        locked.store.dispose();
    }
    { // Topology: an auto-laid card moves between lanes, pins where it lands and undoes back to the automatic layout
        const topo = [card('router', 0, 0, { placements: {} }), card('laptop', 0, 0, { icon: 'laptop', name: 'Laptop', placements: {} })];
        const w = world({ nodes: topo, selection: ['laptop'], focus: 'laptop', view: 'topology', sourceHosts: [] });
        assert.deepEqual(plain(w.gl.topologyAutoPositions(w.store.state.nodes, [], { placementOf: () => null, guests: w.store.guests() }).get('laptop')), { x: 0, y: 1248, pinned: false }, 'the laptop starts in the clients lane (lane 3 top 1152 plus 96)');
        w.press('m');
        for (let step = 0; step < 5; step += 1) w.press('ArrowUp');
        assert.equal(w.temps.at(-1).cards.get('laptop').y, 1128, 'the move starts from the drawn slot, not from a stored placement (1248 - 5 * 24)');
        assert.equal(w.laneTargets.at(-1), 2, 'the lane the card would land in is highlighted');
        w.press('Enter');
        assert.equal(w.laneTargets.at(-1), -1, 'the highlight is cleared');
        assert.equal(w.announced.at(-1), 'desktop.infra_moved_to_lane', 'a lane change is announced');
        await sleep(30);
        assert.deepEqual(plain(itemOf(w.puts[0], 'laptop')), { id: 'laptop', version: 1, x: 0, y: 1128, pinned: true });
        assert.equal(w.store.effectivePlacement('laptop', 'topology').pinned, true);
        assert.equal(w.store.canUndo('topology'), true, 'the first move of an auto-laid card is undoable');
        await w.store.undo('topology');
        await sleep(30);
        assert.equal(w.puts.at(-1).nodes[0].unplace, true, 'undo hands the card back to the automatic layout');
        // a drag that ends where the card is drawn pins nothing
        const still = world({ nodes: topo, selection: ['laptop'], focus: 'laptop', view: 'topology', sourceHosts: [] });
        still.press('m'); still.press('ArrowUp'); still.press('ArrowDown'); still.press('Enter');
        await sleep(30);
        assert.equal(still.puts.length, 0, 'a move that returns to the drawn slot saves nothing');
        // an older server has no unplace: the move still works but cannot be undone
        const old = world({ nodes: topo, selection: ['laptop'], focus: 'laptop', view: 'topology', sourceHosts: null });
        old.press('m'); old.press('ArrowUp'); old.press('Enter');
        await sleep(30);
        assert.equal(old.puts.length, 1);
        assert.equal(old.store.canUndo('topology'), false, 'without unplace support the move stays out of the history');
        for (const item of [w, still, old]) item.store.dispose();
    }
    { // A22: Topology ignores saved pinned:false placements; Arrange hands cards back to the automatic layout and undo restores them
        const placed = (id, x, y, pinned, extra = {}) => card(id, 0, 0, { icon: 'laptop', name: id, placements: { topology: { x, y, pinned } }, ...extra });
        const topoNodes = () => [placed('legacy', 0, 0, false), placed('a', 240, 1128, true), placed('b', -480, 1200, true), card('c', 0, 0, { icon: 'laptop', name: 'c', placements: {} }), card('room', 0, 0, { kind: 'room', name: 'Room', placements: { topology: { x: 0, y: 0, w: 360, h: 240, pinned: true } } })];
        const w = world({ nodes: topoNodes(), view: 'topology', sourceHosts: [] });
        assert.equal(w.store.effectivePlacement('legacy', 'topology'), null, 'a saved pinned:false topology placement is ignored');
        assert.deepEqual(plain(w.store.effectivePlacement('a', 'topology')), { x: 240, y: 1128, pinned: true });
        assert.equal(w.store.effectivePlacement('legacy', 'atlas'), null);
        assert.deepEqual(plain(w.gl.topologyAutoPositions(w.store.state.nodes, [], { placementOf: id => w.store.effectivePlacement(id, 'topology'), guests: w.store.guests() }).get('legacy')), { x: 360, y: 1248, pinned: false }, 'the legacy asset is drawn at its automatic slot (4th of the 4 clients: a, b, c, legacy at -360, -120, 120, 360), not where it was saved');
        assert.deepEqual(plain(w.store.currentIntent('legacy', 'topology')), null, 'there is nothing to restore for it');
        const kept = world({ nodes: [card('m', 0, 0, { placements: { atlas: { x: 6, y: 7, pinned: false } } })], view: 'atlas', sourceHosts: [] });
        assert.deepEqual(plain(kept.store.effectivePlacement('m', 'atlas')), { x: 6, y: 7, pinned: false }, 'the Map keeps pinned:false placements (Arrange there saves them)');
        kept.store.dispose();
        assert.equal(w.gestures.canArrangeTopology(), true);
        await w.gestures.arrange();
        await sleep(30);
        assert.equal(w.puts.length, 1, 'one save for the whole arrangement');
        assert.deepEqual(plain(w.puts[0].nodes.map(item => [item.id, item.unplace === true, 'x' in item])), [['a', true, false], ['b', true, false]], 'every card with a saved pin is handed back; automatic cards, legacy ones and places are not touched');
        assert.equal(w.puts[0].view, 'topology');
        assert.equal(w.announced.at(-1), 'desktop.infra_topology_auto_placed', 'the count is announced');
        assert.equal(w.store.effectivePlacement('a', 'topology'), null);
        assert.equal(w.store.canUndo('topology'), true);
        await w.store.undo('topology');
        await sleep(30);
        assert.deepEqual(plain(w.puts[1].nodes.map(item => [item.id, item.x, item.y, item.pinned])), [['a', 240, 1128, true], ['b', -480, 1200, true]], 'undo restores the previous placements, pinned ones included');
        assert.deepEqual(plain(w.store.effectivePlacement('b', 'topology')), { x: -480, y: 1200, pinned: true });
        assert.equal(w.gestures.canArrangeTopology(), true);
        // a selection arranges only the selected cards
        const picked = world({ nodes: topoNodes(), selection: ['b', 'c'], view: 'topology', sourceHosts: [] });
        await picked.gestures.arrange();
        await sleep(30);
        assert.deepEqual(plain(picked.puts[0].nodes.map(item => item.id)), ['b'], 'only the selected card that has a saved placement');
        assert.equal(picked.announced.at(-1), 'desktop.infra_topology_auto_placed_one');
        // nothing left to hand back: Arrange has nothing to do and saves nothing
        const calm = world({ nodes: [placed('legacy', 0, 0, false), card('c', 0, 0, { icon: 'laptop', placements: {} })], view: 'topology', sourceHosts: [] });
        assert.equal(calm.gestures.canArrangeTopology(), false);
        await calm.gestures.arrange();
        await sleep(30);
        assert.equal(calm.puts.length, 0);
        // a server that cannot unplace cannot arrange in Topology at all
        const old = world({ nodes: topoNodes(), view: 'topology', sourceHosts: null });
        assert.equal(old.gestures.canArrangeTopology(), false);
        await old.gestures.arrange();
        await sleep(30);
        assert.equal(old.puts.length, 0, 'without unplace support Arrange is disabled in Topology and does nothing');
        for (const item of [w, picked, calm, old]) item.store.dispose();
    }
    { // a place moves with everything inside it
        const nodes = [room(400, 120, 480, 320), card('inside', 640, 300, { parent_id: 'room' }), card('outside', 160, 200)];
        const w = world({ nodes, selection: ['room'], focus: 'room', sourceHosts: [] });
        const entered = w.press('M');
        assert.equal(entered.defaultPrevented, true);
        assert.equal(w.announced[0], 'desktop.infra_move_mode_place');
        w.press('ArrowRight'); w.press('ArrowRight'); w.press('ArrowDown');
        const preview = w.temps.at(-1);
        assert.equal(preview.regions.get('room').x, 448);
        assert.equal(preview.regions.get('room').y, 144);
        assert.equal(preview.cards.get('inside').x, 688, 'the asset in the place moves with it');
        assert.equal(preview.cards.has('outside'), false);
        w.press('Enter');
        await sleep(30);
        assert.equal(w.puts.length, 1, 'one save for the place and its assets');
        assert.deepEqual(plain([itemOf(w.puts[0], 'room'), itemOf(w.puts[0], 'inside')].map(item => [item.x, item.y])), [[448, 144], [688, 324]]);
        assert.equal(itemOf(w.puts[0], 'outside'), undefined);
        assert.equal(w.store.state.interacting, 0);
        w.store.dispose();
    }
    { // R switches a place to resizing: arrows resize by 24 px, never below 200 x 140; Enter saves through resizePlace
        const nodes = [room(400, 120, 480, 320), card('inside', 640, 300, { parent_id: 'room' })];
        const w = world({ nodes, selection: ['room'], focus: 'room' });
        w.press('m');
        assert.equal(w.press('r').defaultPrevented, true);
        assert.equal(w.announced.at(-1), 'desktop.infra_resize_mode_on', 'the mode switch is announced');
        const resize = key => w.press(key);
        resize('ArrowRight'); resize('ArrowDown');
        assert.deepEqual(plain([w.temps.at(-1).regions.get('room').w, w.temps.at(-1).regions.get('room').h]), [504, 344], 'exactly 24 px per press');
        assert.equal(w.announced.at(-1), 'desktop.infra_place_size');
        for (let step = 0; step < 20; step += 1) resize('ArrowLeft');
        assert.equal(w.temps.at(-1).regions.get('room').w, 200, 'the minimum width holds');
        for (let step = 0; step < 20; step += 1) resize('ArrowUp');
        assert.equal(w.temps.at(-1).regions.get('room').h, 140, 'the minimum height holds');
        resize('ArrowRight');
        assert.equal(w.temps.at(-1).regions.get('room').w, 224, 'the arrow back is not swallowed by the minimum');
        w.press('r');
        assert.equal(w.announced.at(-1), 'desktop.infra_move_mode_place', 'R again announces the move mode');
        w.press('ArrowRight');
        assert.equal(w.temps.at(-1).regions.get('room').w, 224, 'in move mode the size stays');
        w.press('Enter');
        await sleep(30);
        assert.equal(w.puts.length, 1, 'the move and the resize are saved together');
        const saved = itemOf(w.puts[0], 'room');
        assert.deepEqual(plain([saved.x, saved.y, saved.w, saved.h]), [424, 120, 224, 140]);
        assert.equal(w.store.canUndo('atlas'), true);
        w.store.dispose();
    }
    { // a server without place support cannot resize; assets have nothing to resize
        const noPlaces = world({ nodes: [room(400, 120, 480, 320)], selection: ['room'], focus: 'room', sourceHosts: null });
        noPlaces.press('m');
        const ignored = noPlaces.press('r');
        assert.equal(ignored.defaultPrevented, false, 'R has no meaning without place support');
        assert.ok(!noPlaces.announced.includes('desktop.infra_resize_mode_on'));
        noPlaces.press('Escape');
        await sleep(30);
        assert.equal(noPlaces.puts.length, 0);
        noPlaces.store.dispose();
        const cardOnly = world({ nodes: [card('a', 0, 0)], selection: ['a'], focus: 'a' });
        cardOnly.press('m');
        cardOnly.press('r');
        assert.ok(!cardOnly.announced.includes('desktop.infra_resize_mode_on'), 'assets have nothing to resize');
        const alt = cardOnly.press('ArrowRight', { shiftKey: true, altKey: true });
        assert.equal(alt.defaultPrevented, true, 'Alt+Arrow never navigates the browser history');
        assert.equal(cardOnly.temps.at(-1).cards.get('a').x, 8, 'an asset still only moves');
        cardOnly.store.dispose();
    }
}

{ // the conflict review reads list entries like the editor, never "[object Object]"
    const targets = [{ app: 'network-cameras', target_id: 'cam-1', label: 'Garden camera' }];
    const text = (key, item) => model.listEntryText(key, item, { targets, t: value => (value === 'desktop.infra_field_vlan' ? 'VLAN' : value) });
    assert.equal(text('interfaces', { name: 'eth0', ip_address: '192.0.2.10' }), 'eth0 · 192.0.2.10');
    assert.equal(text('interfaces', { name: 'eth1', ip_address: '', mac_address: '', vlan: '20', speed: '1G' }), 'eth1 · VLAN 20 · 1G');
    assert.equal(text('links', { label: 'Router UI', url: 'https://192.0.2.1/' }), 'Router UI · https://192.0.2.1/');
    assert.equal(text('app_bindings', { app: 'network-cameras', target_id: 'cam-1' }), 'Garden camera');
    assert.equal(text('app_bindings', { app: 'meshcore', target_id: 'gone-7' }), 'meshcore · gone-7', 'a missing target falls back to app and id');
    assert.equal(text('tags', 'lab'), 'lab');
    assert.equal(text('interfaces', null), '');
    for (const entry of [text('interfaces', { name: 'x' }), text('links', { label: 'y' }), text('app_bindings', { app: 'a', target_id: 'b' })]) assert.ok(!entry.includes('[object Object]'));
}

for (const file of ['desktop-app-infra.css', 'desktop-app-infra-canvas.css']) {
    const css = fs.readFileSync(path.join(root, 'ui/css', file), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
    const outsideMixes = css.replace(/color-mix\((?:[^()]|\([^()]*\))*\)/g, '');
    assert.deepEqual(outsideMixes.match(/#[0-9a-fA-F]{3,8}\b/g) || [], [], `${file} uses hex colours outside color-mix()`);
    assert.deepEqual(outsideMixes.match(/rgba?\(\s*\d/g) || [], [], `${file} uses literal rgb() colours instead of tokens`);
    assert.ok(!/var\(--vd-font[,)]/.test(css), `${file} uses --vd-font instead of --vd-font-ui`);
    assert.ok(!/copper/i.test(css), `${file} still uses the copper accent`);
}

// ---- topology lanes and auto layout (spec §6) ------------------------------------
// Cards are 216x64, so auto columns sit 240 px apart (LANE_CELL_W): two cards are centred at -120/120, three at -240/0/240.
{
    const lane = (icon, extra = {}, opts) => lanes.topologyLane({ kind: 'device', icon, profile: {}, ...extra }, opts);
    assert.deepEqual(plain(lanes.topologyLanes()), ['internet', 'network', 'compute', 'clients']);
    assert.equal(lanes.topologyLane({ kind: 'network', icon: 'generic-device', profile: {} }), 'network', 'the network kind is always in the network lane');
    for (const icon of ['internet', 'modem-ont']) assert.equal(lane(icon), 'internet', icon);
    for (const icon of ['router', 'firewall', 'vpn', 'managed-switch', 'switch', 'access-point', 'mesh-node', 'patch-panel', 'lan', 'vlan', 'wlan']) assert.equal(lane(icon), 'network', icon);
    // A21: servers, storage and rack infrastructure are all compute; the groups are separate loops so a failure names its group.
    for (const icon of ['rack-server', 'tower-server', 'mini-pc', 'sbc', 'hypervisor', 'kubernetes']) assert.equal(lane(icon), 'compute', `server ${icon}`);
    for (const icon of ['nas', 'disk-shelf', 'storage-share', 'backup', 'database', 'nvr']) assert.equal(lane(icon), 'compute', `storage ${icon}`);
    for (const icon of ['ups', 'pdu']) assert.equal(lane(icon), 'compute', `rack infrastructure ${icon}`);
    for (const icon of ['camera', 'printer', '3d-printer', 'tv', 'console', 'smartphone', 'tablet', 'laptop', 'workstation', 'sensor', 'smart-hub', 'smart-plug', 'radio-gateway', 'usb-gateway']) assert.equal(lane(icon), 'clients', `client ${icon}`);
    for (const icon of ['rack-server', 'nas', 'disk-shelf', 'nvr', 'database', 'ups', 'pdu', 'vpn', 'firewall', 'camera']) assert.ok(model.iconIds().includes(icon), `${icon} is a real icon id`);
    for (const [role, want] of [['NAS', 'compute'], ['Disk shelf', 'compute'], ['storage appliance', 'compute'], ['PostgreSQL database', 'compute'], ['NVR', 'compute'], ['Network video recorder', 'compute'], ['file server', 'compute'], ['Rack unit', 'compute'], ['APC UPS', 'compute'], ['PDU', 'compute'], ['Hypervisor', 'compute'], ['VPN', 'network'], ['Firewall', 'network'], ['VPN gateway', 'network'], ['camera', 'clients'], ['printer', 'clients'], ['mdns-device', 'clients']]) assert.equal(lane('generic-device', { profile: { role } }), want, `role ${role}`);
    assert.equal(lane('generic-device', { profile: { role: 'InternetGatewayDevice' } }), 'network', 'role words split camel case');
    assert.equal(lane('generic-device', { profile: { role: 'DSL modem' } }), 'internet');
    assert.equal(lane('generic-device', { profile: { role: 'MediaServer' } }), 'compute');
    assert.equal(lane('workstation', {}, { hasGuests: true }), 'compute', 'a device that runs guests is compute');
    assert.equal(lane('router', {}, { hasGuests: true }), 'network', 'an explicit icon wins over guests');
    assert.equal(lane('generic-device', { profile: { role: 'mdns-device' } }), 'clients');
    assert.equal(lane('tv', { profile: { role: 'chromecast' } }), 'clients');
    assert.equal(lanes.topologyLane({ kind: 'service', icon: 'web-service', profile: {} }), 'clients', 'everything else is a client');
    assert.equal(lanes.topologyLaneIndex('compute'), 2);
    assert.deepEqual([-50, 0, 383, 384, 1535, 5000, 'x'].map(y => lanes.laneAt(y)), [0, 0, 0, 1, 3, 3, 3], 'lanes are 384 px tall');
    assert.deepEqual(plain(lanes.laneStripes()), [
        { id: 'internet', index: 0, rect: { x: -50000, y: 0, w: 100000, h: 384 } },
        { id: 'network', index: 1, rect: { x: -50000, y: 384, w: 100000, h: 384 } },
        { id: 'compute', index: 2, rect: { x: -50000, y: 768, w: 100000, h: 384 } },
        { id: 'clients', index: 3, rect: { x: -50000, y: 1152, w: 100000, h: 384 } },
    ], 'the stripes are fixed world rects far wider than any content');

    const topo = [
        { id: 'inet', kind: 'device', name: 'Internet', icon: 'internet', profile: {} },
        { id: 'ont', kind: 'device', name: 'Fiber ONT', icon: 'modem-ont', profile: {} },
        { id: 'router', kind: 'device', name: 'Edge router', icon: 'router', profile: {} },
        { id: 'switch', kind: 'device', name: 'Core switch', icon: 'managed-switch', profile: {} },
        { id: 'ap', kind: 'device', name: 'Attic access point', icon: 'access-point', profile: {} },
        { id: 'nas', kind: 'device', name: 'Basement NAS', icon: 'nas', profile: {} },
        { id: 'pve', kind: 'device', name: 'Proxmox host', icon: 'generic-device', profile: {} },
        { id: 'laptop', kind: 'device', name: 'Admin laptop', icon: 'laptop', profile: {} },
        { id: 'tv', kind: 'device', name: 'Living room TV', icon: 'tv', profile: {} },
    ];
    const topoEdges = [
        { id: 'e1', kind: 'ethernet', from: 'inet', to: 'ont' }, { id: 'e2', kind: 'fiber', from: 'ont', to: 'router' },
        { id: 'e3', kind: 'ethernet', from: 'router', to: 'switch' }, { id: 'e4', kind: 'ethernet', from: 'switch', to: 'ap' },
        { id: 'e5', kind: 'ethernet', from: 'switch', to: 'nas' }, { id: 'e6', kind: 'ethernet', from: 'pve', to: 'switch' },
        { id: 'e7', kind: 'wifi', from: 'ap', to: 'laptop' }, { id: 'e8', kind: 'ethernet', from: 'switch', to: 'tv' },
        { id: 'e9', kind: 'depends_on', from: 'tv', to: 'nas' },
    ];
    const guestsOf = new Map([['pve', ['vm1']]]);
    const laneOf = node => lanes.topologyLaneIndex(lanes.topologyLane(node, { hasGuests: (guestsOf.get(node.id) || []).length > 0 }));
    const asObject = map => Object.fromEntries(plain([...map]));
    const layout = lanes.topologyLayout(topo, topoEdges, { laneOf });
    assert.deepEqual(asObject(layout), {
        inet: { x: -120, y: 96, pinned: false }, ont: { x: 120, y: 96, pinned: false },
        switch: { x: -240, y: 480, pinned: false }, router: { x: 0, y: 480, pinned: false }, ap: { x: 240, y: 480, pinned: false },
        nas: { x: -120, y: 864, pinned: false }, pve: { x: 120, y: 864, pinned: false },
        tv: { x: -120, y: 1248, pinned: false }, laptop: { x: 120, y: 1248, pinned: false },
    }, 'A20: top sweep keys router 120 (ont), nas/pve/tv 0 and laptop 240 (switch, ap at 0 and 240); the bottom sweep keys switch -40 (nas, pve, tv at -120, 120, -120) and ap 120 (laptop), so the lane reads switch, router, ap; the first row of lane i is at 384 i + 96');
    assert.deepEqual(asObject(lanes.topologyLayout(topo, topoEdges, { laneOf, ordering: 'connection' })), {
        inet: { x: -120, y: 96, pinned: false }, ont: { x: 120, y: 96, pinned: false },
        router: { x: -240, y: 480, pinned: false }, switch: { x: 0, y: 480, pinned: false }, ap: { x: 240, y: 480, pinned: false },
        nas: { x: -120, y: 864, pinned: false }, pve: { x: 120, y: 864, pinned: false },
        tv: { x: -120, y: 1248, pinned: false }, laptop: { x: 120, y: 1248, pinned: false },
    }, 'the plain connection order (breadth first from the top lane) is still available');
    assert.deepEqual(asObject(lanes.topologyLayout(topo.slice().reverse(), topoEdges.slice().reverse(), { laneOf })), asObject(layout), 'input order does not change the layout');
    assert.deepEqual(asObject(lanes.topologyLayout(topo, topoEdges.slice(0, 8), { laneOf })), asObject(layout), 'dependencies do not order the layout; only Ethernet, fiber and Wi-Fi do');
    const overridden = lanes.topologyLayout(topo, topoEdges, { laneOf: node => (node.id === 'tv' ? 1 : laneOf(node)) });
    // tv joins lane 1 with only a same-lane link (switch): it borrows the switch's bottom-sweep key 0 and ties with switch and ap.
    assert.deepEqual(plain([overridden.get('switch'), overridden.get('ap'), overridden.get('tv'), overridden.get('router')]), [-360, -120, 120, 360].map(x => ({ x, y: 480, pinned: false })), 'a lane override moves the card; lane 1 reads switch, ap, tv, router');
    assert.deepEqual(plain(overridden.get('laptop')), { x: 0, y: 1248, pinned: false });
    const clients = Array.from({ length: 10 }, (_, i) => ({ id: `c${i}`, kind: 'device', name: `Client ${String(i).padStart(2, '0')}`, icon: 'laptop', profile: {} }));
    const wrapped = lanes.topologyLayout(clients, []);
    // columns per row = max(8, ceil(n / 2)): ten cards make a row of eight (x -840 + 240 i) and a centred row of two, 144 px below.
    assert.deepEqual(plain(wrapped.get('c4')), { x: 120, y: 1248, pinned: false });
    assert.deepEqual(plain(wrapped.get('c7')), { x: 840, y: 1248, pinned: false });
    assert.deepEqual(plain(wrapped.get('c8')), { x: -120, y: 1392, pinned: false }, 'ten cards wrap into a row of eight and a row of two');
    assert.deepEqual(plain(wrapped.get('c9')), { x: 120, y: 1392, pinned: false });
    // a lane that would need a third row widens its rows instead: 17 cards are two rows of nine (x -960 + 240 i) and eight
    const wide = lanes.topologyLayout(Array.from({ length: 17 }, (_, i) => ({ id: `w${String(i).padStart(2, '0')}`, kind: 'device', name: `Wide ${String(i).padStart(2, '0')}`, icon: 'laptop', profile: {} })), []);
    assert.deepEqual(plain([wide.get('w00'), wide.get('w08'), wide.get('w09'), wide.get('w16')]), [{ x: -960, y: 1248, pinned: false }, { x: 960, y: 1248, pinned: false }, { x: -840, y: 1392, pinned: false }, { x: 840, y: 1392, pinned: false }], 'no lane ever has a third row');
    assert.equal(new Set([...wide.values()].map(pos => pos.y)).size, 2);
    for (const pos of layout.values()) assert.ok(pos.x % model.GRID === 0 && pos.y % model.GRID === 0, 'auto slots are grid points');

    const autoNodes = [...topo.slice(0, 3),
        { id: 'room', kind: 'room', name: 'Living room', icon: 'room', profile: {} },
        { id: 'vm1', kind: 'vm', name: 'Lab VM', icon: 'vm', host_id: 'router', profile: {} },
        { id: 'ct', kind: 'container', name: 'orphan', icon: 'container', profile: {} }];
    const autoGuests = model.guestIndex(autoNodes, []);
    const far = { ont: { x: 2000, y: 96, pinned: true } };
    const auto = lanes.topologyAutoPositions(autoNodes, topoEdges.slice(0, 2), { placementOf: id => far[id] || null, guests: autoGuests });
    assert.deepEqual(asObject(auto), { inet: { x: -120, y: 96, pinned: false }, router: { x: 0, y: 480, pinned: false } }, 'placed assets, places, guests and hostless containers get no auto slot');
    const onSlot = { ont: { x: -120, y: 96, pinned: true } };
    assert.deepEqual(plain(lanes.topologyAutoPositions(autoNodes, topoEdges.slice(0, 2), { placementOf: id => onSlot[id] || null, guests: autoGuests }).get('inet')), { x: -120, y: 24, pinned: false }, 'an auto slot under a placed card moves aside');
    const sceneTopo = model.buildScene({ nodes: autoNodes, edges: topoEdges.slice(0, 2), view: 'topology', placementOf: id => far[id] || auto.get(id) || null, parentOf: () => '', guests: autoGuests });
    assert.deepEqual(plain(sceneTopo.cards.map(card => card.id)), ['inet', 'ont', 'router']);
    assert.deepEqual(plain(sceneTopo.bounds), { x: -228, y: 64, w: 2336, h: 448 }, 'the scene bounds are the drawn cards only; the stripes never widen fit or the minimap');
    assert.equal(sceneTopo.lanes, undefined, 'the scene knows no lanes');
    assert.equal(typeof lanes.topologyArrange, 'undefined', 'Arrange in Topology is no layout any more: it unplaces');

    // ---- A20/A21 on the realistic fixture: the 24 assets of the browser world(25) with a believable wiring ----
    const demo = [['Public status', 'service', 'web-service'], ['Atlas hypervisor', 'vm', 'vm'], ['Cold-store NAS', 'device', 'nas'], ['Core switch', 'device', 'managed-switch'], ['North access point', 'device', 'access-point'], ['UPS bank', 'device', 'ups'], ['Entry camera', 'device', 'camera'], ['Camera recorder', 'device', 'nvr'], ['Build workstation', 'device', 'workstation'], ['Admin laptop', 'device', 'laptop'], ['Office printer', 'device', 'printer'], ['Gateway firewall', 'device', 'firewall'], ['Edge router', 'device', 'router'], ['Archive shelf', 'device', 'disk-shelf'], ['Prototype printer', 'device', '3d-printer'], ['Mesh relay', 'device', 'mesh-node'], ['Power distribution', 'device', 'pdu'], ['Fiber ONT', 'device', 'modem-ont'], ['Field tablet', 'device', 'tablet'], ['Room controller', 'device', 'smart-hub'], ['Remote access', 'device', 'vpn'], ['Metrics database', 'device', 'database'], ['Rack server', 'device', 'rack-server'], ['Lab mini PC', 'device', 'mini-pc']];
    const fid = i => `n${String(i).padStart(2, '0')}`;
    const lab = demo.map(([name, kind, icon], k) => ({ id: fid(k + 1), name, kind, icon, profile: {} }));
    const wire = (kind, a, b) => ({ id: `w-${a}-${b}`, kind, from: fid(a), to: fid(b) });
    const labEdges = [wire('fiber', 18, 13), wire('ethernet', 13, 12), wire('ethernet', 12, 4), wire('ethernet', 4, 23), wire('ethernet', 4, 24), wire('ethernet', 4, 9), wire('ethernet', 4, 11), wire('ethernet', 4, 8), wire('ethernet', 8, 7),
        wire('wifi', 5, 10), wire('wifi', 5, 19), wire('wifi', 5, 20), wire('ethernet', 13, 21), wire('ethernet', 23, 22), wire('ethernet', 14, 3), wire('ethernet', 4, 16), wire('ethernet', 4, 5), wire('ethernet', 4, 3),
        { id: 'w-runs-on', kind: 'runs_on', from: fid(1), to: fid(2) }];
    const labGuests = model.guestIndex(lab, labEdges);
    const labAuto = lanes.topologyAutoPositions(lab, labEdges, { placementOf: () => null, guests: labGuests });
    assert.equal(labAuto.size, 22, 'the hostless VM and its guest are not drawn');
    // A21: of 22 assets 1 is internet, 6 network, 8 servers and storage (it was 3), 7 clients and IoT
    const laneCounts = [0, 0, 0, 0];
    for (const node of lab) if (labAuto.has(node.id)) laneCounts[Math.floor(labAuto.get(node.id).y / 384)] += 1;
    assert.deepEqual(laneCounts, [1, 6, 8, 7], 'NAS shelf, recorder, database, UPS, PDU and the server land in compute; VPN and firewall in network');
    const labLane = id => Math.floor(labAuto.get(id).y / 384);
    assert.deepEqual(['n14', 'n08', 'n22', 'n06', 'n17', 'n23', 'n03'].map(labLane), [2, 2, 2, 2, 2, 2, 2]);
    assert.deepEqual(['n21', 'n12'].map(labLane), [1, 1]);
    assert.deepEqual(['n07', 'n15', 'n11', 'n19'].map(labLane), [3, 3, 3, 3], 'cameras, printers and tablets stay clients');
    // A20: crossings of the physical connections, drawn as straight lines between card centres
    const labLaneOf = node => lanes.topologyLaneIndex(lanes.topologyLane(node, { hasGuests: (labGuests.guestsOf.get(node.id) || []).length > 0 }));
    const drawable = lab.filter(node => labAuto.has(node.id));
    const crossings = positions => {
        const wires = labEdges.filter(edge => ['ethernet', 'fiber', 'wifi'].includes(edge.kind)).map(edge => [positions.get(edge.from), positions.get(edge.to), edge]);
        const side = (a, b, c) => Math.sign((b.x - a.x) * (c.y - a.y) - (b.y - a.y) * (c.x - a.x));
        let count = 0;
        for (let i = 0; i < wires.length; i += 1) for (let j = i + 1; j < wires.length; j += 1) {
            const [a, b, e1] = wires[i], [c, d, e2] = wires[j];
            if ([e1.from, e1.to].some(end => [e2.from, e2.to].includes(end))) continue; // wires that share a card meet there
            if (side(a, b, c) * side(a, b, d) < 0 && side(c, d, a) * side(c, d, b) < 0) count += 1;
        }
        return count;
    };
    const byConnection = lanes.topologyLayout(drawable, labEdges, { laneOf: labLaneOf, ordering: 'connection' });
    const byBarycentre = lanes.topologyLayout(drawable, labEdges, { laneOf: labLaneOf });
    assert.equal(crossings(byConnection), 11, 'the plain connection order crosses 11 times');
    assert.equal(crossings(byBarycentre), 5, 'the barycentre order crosses 5 times');
    assert.ok(crossings(byBarycentre) < crossings(byConnection), 'A20: the barycentre order crosses less than the connection order');
    assert.deepEqual(asObject(lanes.topologyLayout(drawable.slice().reverse(), labEdges.slice().reverse(), { laneOf: labLaneOf })), asObject(byBarycentre), 'the barycentre order does not depend on the input order');
    const shuffled = drawable.map((node, i) => [(i * 7 + 3) % drawable.length, node]).sort((a, b) => a[0] - b[0]).map(item => item[1]);
    assert.deepEqual(asObject(lanes.topologyLayout(shuffled, labEdges, { laneOf: labLaneOf })), asObject(byBarycentre), 'nor on a shuffled input');
    const laneOrder = index => [...byBarycentre].filter(([id]) => labLane(id) === index).sort((a, b) => a[1].y - b[1].y || a[1].x - b[1].x).map(([id]) => id);
    assert.deepEqual(laneOrder(3), ['n07', 'n09', 'n11', 'n10', 'n19', 'n20', 'n15'], 'clients are keyed by the card they hang on; the unconnected 3D printer goes last');
}

// ---- A19: Topology connections are curves between lanes ------------------------------------------------
// A connection between two lanes leaves the bottom edge of the upper card and arrives on the top edge of the lower one with
// vertical tangents (control offset max(48, dy / 2)); several connections on one card edge spread over the middle 60% of its
// 216 px (+-64.8 px), in slots of 129.6 / k ordered by the other end's x. Same-lane connections stay straight.
{
    const cardsAt = list => new Map(list.map(([id, x, y]) => [id, { id, x, y }]));
    const draw = (list, wires) => lanes.topologyPaths(wires.map(([id, from, to]) => ({ id, from, to })), id => cardsAt(list).get(id));
    const shape = path => plain({ curve: path.curve, d: path.d, ends: [path.x1, path.y1, path.x2, path.y2] });

    // one connection: bottom centre (0, 96 + 32) to top centre (0, 480 - 32); dy 320, control offset 160
    const single = draw([['u', 0, 96], ['l', 0, 480]], [['x', 'u', 'l']]);
    assert.deepEqual(shape(single.get('x')), { curve: true, d: 'M0 128C0 288 0 288 0 448', ends: [0, 128, 0, 448] });
    assert.equal(draw([['u', 0, 96], ['l', 0, 480]], [['x', 'l', 'u']]).get('x').d, single.get('x').d, 'the direction of the connection does not matter: it always runs from the upper card down');
    // a fan of three below one card: slots at -43.2, 0, 43.2, ordered by the other end's x whatever the input order
    const fan = draw([['u', 0, 96], ['l1', -240, 480], ['l2', 240, 480], ['l3', 0, 480]], [['b', 'u', 'l2'], ['c', 'l3', 'u'], ['a', 'u', 'l1']]);
    assert.equal(fan.get('a').d, 'M-43.2 128C-43.2 288 -240 288 -240 448');
    assert.equal(fan.get('c').d, 'M0 128C0 288 0 288 0 448');
    assert.equal(fan.get('b').d, 'M43.2 128C43.2 288 240 288 240 448');
    // a fan in: two cards above one, arrival slots at -32.4 and 32.4; the midpoint (label anchor) is the curve's midpoint
    const fanIn = draw([['u1', -240, 96], ['u2', 240, 96], ['l', 0, 480]], [['q', 'u2', 'l'], ['p', 'u1', 'l']]);
    assert.deepEqual(shape(fanIn.get('p')), { curve: true, d: 'M-240 128C-240 288 -32.4 288 -32.4 448', ends: [-240, 128, -32.4, 448] });
    assert.equal(fanIn.get('q').d, 'M240 128C240 288 32.4 288 32.4 448');
    // the midpoint of a cubic with vertical tangents is the midpoint of its ends: (-136.2, 288)
    const cubic = (t, a, b, c, d) => (1 - t) ** 3 * a + 3 * (1 - t) ** 2 * t * b + 3 * (1 - t) * t ** 2 * c + t ** 3 * d;
    assert.ok(Math.abs(cubic(0.5, -240, -240, -32.4, -32.4) - (fanIn.get('p').x1 + fanIn.get('p').x2) / 2) < 1e-9 && Math.abs(cubic(0.5, 128, 288, 288, 448) - (fanIn.get('p').y1 + fanIn.get('p').y2) / 2) < 1e-9);
    // the control offset grows with the drop and never falls below 48
    assert.equal(draw([['u', 0, 96], ['l', 0, 864]], [['x', 'u', 'l']]).get('x').d, 'M0 128C0 480 0 480 0 832', 'dy 704 gives offset 352');
    assert.equal(draw([['u', 0, 300], ['l', 0, 420]], [['x', 'u', 'l']]).get('x').d, 'M0 332C0 380 0 340 0 388', 'dy 56 gives the minimum offset 48');
    // parallel connections between the same two cards take neighbouring slots at both ends
    const parallel = draw([['u', 0, 96], ['l', 0, 480]], [['e2', 'u', 'l'], ['e1', 'u', 'l']]);
    assert.equal(parallel.get('e1').d, 'M-32.4 128C-32.4 288 -32.4 288 -32.4 448');
    assert.equal(parallel.get('e2').d, 'M32.4 128C32.4 288 32.4 288 32.4 448');
    // same lane: a straight line between the facing sides (the left card's right edge at -132, the right card's left edge at -108)
    const flat = draw([['a', -240, 480], ['b', 0, 480]], [['x', 'a', 'b']]);
    assert.deepEqual(shape(flat.get('x')), { curve: false, d: 'M-132 480L-108 480', ends: [-132, 480, -108, 480] });
    // cards that overlap across a lane border have no room for a curve: straight, never a loop
    assert.equal(draw([['u', 0, 380], ['l', 0, 400]], [['x', 'u', 'l']]).get('x').curve, false);
    // the lane of a card is the lane its drawn y is in: a card dragged into the next lane is connected like any other
    assert.equal(draw([['u', 0, 96], ['l', 0, 383]], [['x', 'u', 'l']]).get('x').curve, false, 'both in lane 0');
    assert.equal(draw([['u', 0, 96], ['l', 0, 384]], [['x', 'u', 'l']]).get('x').curve, true, 'lane 1 starts at y 384');
    // a connection with a missing end is not drawn; the answer does not depend on the order of the connections
    assert.equal(draw([['u', 0, 96]], [['x', 'u', 'gone']]).size, 0);
    const wires = [['a', 'u', 'l1'], ['b', 'u', 'l2'], ['c', 'u', 'l3'], ['d', 'l1', 'l2'], ['e', 'u', 'l1']];
    const forward = draw([['u', 0, 96], ['l1', -240, 480], ['l2', 240, 480], ['l3', 0, 480]], wires), backward = draw([['u', 0, 96], ['l1', -240, 480], ['l2', 240, 480], ['l3', 0, 480]], wires.slice().reverse());
    assert.deepEqual(plain([...forward].sort((x, y) => x[0].localeCompare(y[0]))), plain([...backward].sort((x, y) => x[0].localeCompare(y[0]))));
}

console.log(`Infra UI model, grouped virtualization, and ${required.size} translation keys passed for ${locales.length} locales.`);
