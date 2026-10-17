import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const source = ['infra-views.js', 'infra.js'].map(file => fs.readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8')).join('\n');
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

console.log(`Infra UI model, grouped virtualization, and ${required.size} translation keys passed for ${locales.length} locales.`);
