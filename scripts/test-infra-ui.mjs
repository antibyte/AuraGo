import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const files = ['infra-model.js', 'infra-views.js', 'infra.js'];
for (const file of files) {
    const text = fs.readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8');
    const lines = text.split('\n').length;
    assert.ok(lines < 1100, `${file} has ${lines} lines, want fewer than 1100`);
}
const source = files.map(file => fs.readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8')).join('\n');
const window = { BUILD_VERSION: 'test-build', btoa: value => Buffer.from(value, 'binary').toString('base64') };
vm.runInNewContext(source, { window, console, setTimeout, clearTimeout, DOMException, TextEncoder, Map, Set, JSON, Math, Date, URL });
const model = window.InfraAppModel;

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

const placed = [
    { id: 'north', placements: { atlas: { x: -330, y: 85 } } },
    { id: 'far', placements: { atlas: { x: 5200, y: 2910 } } },
    ...Array.from({ length: 1000 }, (_, index) => ({ id: `unplaced-${index}`, placements: {} })),
];
const fullWorld = model.sceneDimensions(placed, 'atlas');
const filteredWorld = model.sceneDimensions(placed.slice(0, 2), 'atlas');
assert.deepEqual(fullWorld, filteredWorld, 'adding or filtering unplaced nodes never changes saved world coordinates');
assert.ok(fullWorld.width > 5200 && fullWorld.height > 2910, 'large positive and negative saved positions remain in the world map');

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
    infra_error: ['disabled', 'readonly', 'unavailable', 'invalid_source', 'invalid_request', 'invalid_action', 'invalid_node_id', 'invalid_candidate_id', 'invalid_edge_id', 'invalid_version', 'not_found', 'source_linked', 'has_children', 'infra_error'],
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

const placedContainer = { id: 'room', kind: 'room' };
const placedChild = { id: 'host', kind: 'device' };
const positions = new Map([['room', { x: 200, y: 200 }], ['host', { x: 420, y: 260 }]]);
const frame = model.containerFrame(placedContainer, [placedChild], item => positions.get(item.id));
assert.equal(frame.x, 200 - 48);
assert.equal(frame.y, 200 - 48);
assert.equal(frame.width, 220 + 96);
assert.equal(frame.height, 60 + 96);
assert.equal(model.containerFrame(placedContainer, [{ id: 'loose' }], () => null), null);

const segment = model.edgeSegment({ x: 0, y: 0 }, { x: 400, y: 0 });
assert.equal(segment.x1, 78);
assert.equal(segment.y1, 0);
assert.equal(segment.x2, 322);
assert.equal(segment.y2, 0);
const short = model.edgeSegment({ x: 0, y: 0 }, { x: 100, y: 0 });
assert.equal(short.x1, 0);
assert.equal(short.y1, 0);
assert.equal(short.x2, 100);
assert.equal(short.y2, 0);
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

console.log(`Infra UI model, grouped virtualization, and ${required.size} translation keys passed for ${locales.length} locales.`);
