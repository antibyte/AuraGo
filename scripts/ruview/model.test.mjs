import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

// The model is a classic script. It runs here the way the other Node-tested desktop modules do:
// in a bare context that has no DOM, only the globals listed below.
const source = readFileSync(new URL('../../ui/js/desktop/apps/ruview-model.js', import.meta.url), 'utf8');
const window = {};
vm.runInNewContext(source, {window, Intl, console, Date, Math, JSON, Set, Map, Number, String, Array, Object});
const M = window.RuViewModel;
// Values built inside the vm context carry that context's prototypes: compare them as plain data.
const plain = value => JSON.parse(JSON.stringify(value));

const NOW = Date.parse('2026-10-25T12:00:00.000Z');
const iso = ms => new Date(ms).toISOString();
const ago = ms => iso(NOW - ms);
const BOARD = {node_id: 7, csi_status: 'active', csi_sequence: 12, csi_last_seen_ms: 800, rssi_dbm: -58};

// A healthy managed ESP32 service with live data; each case overrides what makes its row.
const base = (over = {}) => ({
    enabled: true, readonly: false, agent_access: true, mode: 'docker', source: 'esp32', revision: 'a71634a', revision_verified: true,
    runtime: 'running', phase: '', phase_since: null, ready: true, fresh: true, simulated: false, waiting_for_frames: false,
    data_state: 'live_unverified', last_frame_age_ms: 800, observed_at: ago(1000), last_fresh_at: ago(1000),
    error_code: '', error_detail: '', stop_reason: '', stopped_at: null, udp_scope: 'managed', udp_enabled: true, udp_ready: true,
    receiver_configured: true, storage_limit: false, storage_used_bytes: 1048576, storage_budget_bytes: 1073741824,
    recording_session: 'unknown', recording_ack: '', recording_at: null, hints: [], ...over
});
const idle = (over = {}) => base({runtime: 'not_started', ready: false, fresh: false, data_state: 'unchecked', last_frame_age_ms: null, observed_at: null, last_fresh_at: null, ...over});
const external = (over = {}) => base({mode: 'external', runtime: 'external', ...over});
const wifi = (over = {}) => base({source: 'wifi', ...over});

function derive(status, input = {}) {
    return plain(M.deriveState({
        status, statusLoaded: status !== null, statusFailed: false, statusAt: NOW, now: NOW,
        nodes: {list: [BOARD], liveState: 'ok', loaded: true, error: '', liveCount: 1},
        views: status ? M.availableViews(status) : ['status'], desktopReadonly: false, pending: new Set(), ...input
    }));
}

// One case per row of the state table (rows 1-17, 3a, 12a and the fallback F).
const CASES = [
    {row: '1', name: 'loading before the first answer', status: null, input: {statusLoaded: false},
        expect: {id: 'loading', tone: 'neutral', glyph: '○', pill: 'checking', action: null, actionLabel: null, secondary: null, busy: false, headline: 'headline_loading'}},
    {row: '2', name: 'status failed after a success keeps the last-known time', status: base(), input: {statusFailed: true, statusAt: NOW - 5000},
        expect: {id: 'status_failed', tone: 'warning', glyph: '▲', pill: 'unavailable', action: 'refresh', actionLabel: 'action_try_again', secondary: {kind: 'last_success', at: NOW - 5000}, errorCode: ''}},
    {row: '2', name: 'first status failure is not "off"', status: null, input: {statusLoaded: false, statusFailed: true},
        expect: {id: 'status_failed', action: 'refresh', actionLabel: 'action_try_again', secondary: null}},
    {row: '3', name: 'turned off', status: idle({enabled: false}),
        expect: {id: 'off', tone: 'neutral', glyph: '○', pill: 'off', action: 'open-settings', actionLabel: 'action_open_settings', secondary: {kind: 'about'}, headline: 'headline_off'}},
    {row: '3a', name: 'installing', status: idle({mode: 'native', phase: 'downloading', phase_since: ago(90000)}),
        expect: {id: 'installing', tone: 'info', glyph: '◐', busy: true, pill: 'installing', action: null, secondary: {kind: 'phase_since', sinceMs: 90000}, headline: 'headline_installing', phase: 'downloading'}},
    {row: '4', name: 'read-only blocks the start', status: idle({readonly: true}),
        expect: {id: 'readonly_blocked', tone: 'info', glyph: '◇', pill: 'read_only', action: 'allow-changes', actionLabel: 'action_allow_changes', actionBlock: '', secondaryAction: 'open-settings', secondary: {kind: 'readonly_blocks'}, overlays: []}},
    {row: '4', name: 'read-only on a read-only desktop offers the settings', status: idle({readonly: true}), input: {desktopReadonly: true},
        expect: {id: 'readonly_blocked', action: 'open-settings', actionLabel: 'action_open_settings', secondaryAction: null}},
    {row: '4', name: 'an unreadable storage folder does not hide the read-only headline', status: idle({readonly: true, error_code: 'storage_unreadable'}),
        expect: {id: 'readonly_blocked', overlays: ['storage_unreadable']}},
    {row: '5', name: 'not installed stays visible under read-only, with the install blocked', status: idle({mode: 'native', readonly: true, error_code: 'not_installed'}),
        expect: {id: 'not_installed', action: 'runtime/install', actionBlock: 'reason_readonly', overlays: ['read_only']}},
    {row: '5', name: 'native service not installed', status: idle({mode: 'native', error_code: 'not_installed'}),
        expect: {id: 'not_installed', tone: 'neutral', glyph: '○', pill: 'not_installed', action: 'runtime/install', actionLabel: 'action_install', secondary: {kind: 'install'}}},
    {row: '6', name: 'stopped by the user', status: idle({runtime: 'stopped', data_state: 'stopped', stop_reason: 'user', stopped_at: ago(3600000)}),
        expect: {id: 'stopped', tone: 'neutral', glyph: '○', pill: 'stopped', action: 'runtime/start', actionLabel: 'action_start', secondary: {kind: 'stop_reason', reason: 'user', at: NOW - 3600000}, headline: 'headline_stopped'}},
    {row: '6', name: 'not started since AuraGo started', status: idle(),
        expect: {id: 'stopped', action: 'runtime/start', secondary: {kind: 'not_started'}}},
    {row: '6', name: 'an unreadable storage folder stays a chip, not a headline', status: idle({error_code: 'storage_unreadable'}),
        expect: {id: 'stopped', errorCode: 'storage_unreadable', overlays: ['storage_unreadable']}},
    {row: '7', name: 'starting while the image downloads', status: idle({runtime: 'starting', data_state: 'starting', phase: 'pulling_image', phase_since: ago(5000)}),
        expect: {id: 'starting_pull', tone: 'info', glyph: '◐', busy: true, pill: 'starting', action: 'runtime/start', actionLabel: 'action_starting', actionBlock: 'reason_wait_start', secondary: {kind: 'phase_step', n: 2, total: 4}, headline: 'headline_starting_pull'}},
    {row: '8', name: 'starting with a phase', status: idle({runtime: 'starting', data_state: 'starting', phase: 'creating', phase_since: ago(12000)}),
        expect: {id: 'starting', tone: 'info', glyph: '◐', busy: true, pill: 'starting', actionLabel: 'action_starting', actionBlock: 'reason_wait_start', secondary: {kind: 'phase_since', sinceMs: 12000}, headline: 'headline_starting', phase: 'creating'}},
    {row: '8', name: 'starting without a phase', status: idle({runtime: 'starting', data_state: 'starting'}),
        expect: {id: 'starting', headline: 'headline_starting_plain', secondary: null}},
    {row: '9', name: 'stopping', status: idle({runtime: 'stopping', data_state: 'stopping'}),
        expect: {id: 'stopping', tone: 'info', glyph: '◐', busy: true, pill: 'stopping', action: null, actionLabel: null, secondary: null}},
    {row: '10', name: 'stop not confirmed', status: idle({runtime: 'stop_unconfirmed', error_code: 'stop_failed'}),
        expect: {id: 'stop_failed', tone: 'danger', glyph: '✕', pill: 'problem', action: 'runtime/stop', actionLabel: 'action_stop_again', secondary: null}},
    {row: '10', name: 'a failed stop outranks "turned off" and can be retried', status: idle({enabled: false, runtime: 'stop_unconfirmed', error_code: 'stop_failed'}),
        expect: {id: 'stop_failed', tone: 'danger', action: 'runtime/stop', actionLabel: 'action_stop_again', actionBlock: ''}},
    {row: '10', name: 'a failed stop without the code is still unconfirmed', status: idle({enabled: false, runtime: 'stop_unconfirmed'}),
        expect: {id: 'stop_failed', action: 'runtime/stop', actionBlock: ''}},
    {row: '10', name: 'stop not confirmed with the planner hint', status: idle({runtime: 'stop_unconfirmed', error_code: 'stop_failed', hints: ['stop_unconfirmed']}),
        expect: {id: 'stop_failed', secondary: {kind: 'stop_hint'}}},
    {row: '11', name: 'docker image denied', status: idle({error_code: 'image_denied', error_detail: 'HTTP 403: registry denied'}),
        expect: {id: 'error', tone: 'danger', glyph: '✕', pill: 'problem', action: 'runtime/start', actionLabel: 'action_retry_start', headline: 'err_image_denied', errorCode: 'image_denied', errorDetail: 'HTTP 403: registry denied', secondary: null}},
    {row: '11', name: 'a start error is not hidden by read-only; the start is blocked with the reason', status: idle({readonly: true, error_code: 'image_denied'}),
        expect: {id: 'error', action: 'runtime/start', actionLabel: 'action_retry_start', actionBlock: 'reason_readonly', headline: 'err_image_denied', overlays: ['read_only']}},
    {row: '11', name: 'an unknown code becomes unknown and offers a check', status: idle({error_code: 'brand_new_code'}),
        expect: {id: 'error', errorCode: 'unknown', headline: 'err_unknown', action: 'test', actionLabel: 'action_check_again'}},
    {row: '11', name: 'a legacy error text becomes unknown', status: {...idle(), runtime: undefined, error_code: undefined, error: 'dial tcp refused'},
        expect: {id: 'error', errorCode: 'unknown', action: 'test'}},
    {row: '11', name: 'process exited offers a start again', status: idle({runtime: 'exited', error_code: 'process_exited', last_fresh_at: ago(60000)}),
        expect: {id: 'error', action: 'runtime/start', actionLabel: 'action_start_again', secondary: {kind: 'last_live', at: NOW - 60000}}},
    {row: '11', name: 'a settings problem opens the settings', status: idle({error_code: 'invalid_config'}),
        expect: {id: 'error', action: 'open-settings', actionLabel: 'action_open_settings'}},
    {row: '11', name: 'an external service is only checked again', status: external({ready: false, fresh: false, error_code: 'timeout'}),
        expect: {id: 'error', action: 'test', actionLabel: 'action_check_again', actionBlock: ''}},
    {row: '12', name: 'external service not checked yet', status: external({ready: false, fresh: false, observed_at: null}),
        expect: {id: 'external_unchecked', tone: 'neutral', glyph: '○', pill: 'not_checked', action: 'test', actionLabel: 'action_check_connection', secondary: null}},
    {row: '12a', name: 'external check is old', status: external({observed_at: ago(120000)}),
        expect: {id: 'external_check_old', tone: 'neutral', glyph: '○', pill: 'not_checked', pillAgeMs: 120000, ageMs: 120000, action: 'test', actionLabel: 'check_now', secondary: {kind: 'last_checked', at: NOW - 120000}}},
    {row: '13', name: 'demo data', status: base({source: 'simulated', simulated: true, data_state: 'synthetic', fresh: false}),
        expect: {id: 'demo', tone: 'info', glyph: '◇', pill: 'demo', action: null, secondaryAction: 'open-settings', secondary: {kind: 'demo'}}},
    {row: '14', name: 'waiting with no boards yet offers adding one', status: base({waiting_for_frames: true, fresh: false, last_frame_age_ms: null}), input: {nodes: {list: [], liveState: 'ok', loaded: true, error: '', liveCount: 0}},
        expect: {id: 'waiting', tone: 'info', glyph: '◐', busy: false, pill: 'waiting', action: 'flash', actionLabel: 'action_add_board', headline: 'headline_waiting', secondary: {kind: 'last_checked', at: NOW - 1000}}},
    {row: '14', name: 'waiting with a board shows the boards', status: base({waiting_for_frames: true, fresh: false, last_frame_age_ms: null}),
        expect: {id: 'waiting', action: 'show-boards', actionLabel: 'action_show_boards'}},
    {row: '14', name: 'esp32 without a frame yet is waiting', status: base({fresh: false, last_frame_age_ms: null}),
        expect: {id: 'waiting'}},
    {row: '14', name: 'wifi waiting has its own headline and no board action', status: wifi({waiting_for_frames: true, fresh: false, last_frame_age_ms: null}),
        expect: {id: 'waiting', headline: 'headline_waiting_wifi', action: null}},
    {row: '15', name: 'signal source disconnected (esp32)', status: base({data_state: 'disconnected', fresh: false, last_frame_age_ms: null}),
        expect: {id: 'disconnected', tone: 'warning', glyph: '▲', pill: 'disconnected', action: 'show-boards', actionLabel: 'action_show_boards', secondary: {kind: 'last_live', at: NOW - 1000}}},
    {row: '15', name: 'signal source disconnected (wifi) points to the settings', status: wifi({data_state: 'disconnected', fresh: false, last_frame_age_ms: null}),
        expect: {id: 'disconnected', action: 'open-settings', actionLabel: 'action_open_settings'}},
    {row: '16', name: 'live', status: base(),
        expect: {id: 'live', tone: 'success', glyph: '●', pill: 'live', pillAgeMs: 800, ageMs: 800, boards: 1, action: 'show-boards', actionLabel: 'action_show_boards', headline: 'headline_live_board', secondary: {kind: 'live_meta', at: NOW - 1000, source: 'esp32', mode: 'docker'}}},
    {row: '16', name: 'live with several boards uses the plural headline', status: base(), input: {nodes: {list: [BOARD, {...BOARD, node_id: 8}], liveState: 'ok', loaded: true, error: '', liveCount: 2}},
        expect: {id: 'live', boards: 2, headline: 'headline_live_boards'}},
    {row: '16', name: 'live without a live board count uses the plain headline', status: base(), input: {nodes: {list: [], liveState: 'ok', loaded: true, error: '', liveCount: 0}},
        expect: {id: 'live', boards: 0, headline: 'headline_live'}},
    {row: '16', name: 'live wifi has the wifi headline and no board count', status: wifi(),
        expect: {id: 'live', headline: 'headline_live_wifi', boards: null, action: null}},
    {row: '16', name: 'a recent check keeps an external service live', status: external(),
        expect: {id: 'live'}},
    {row: '17', name: 'no new data', status: base({data_state: 'stale', fresh: false, last_frame_age_ms: 20000}),
        expect: {id: 'stale', tone: 'warning', glyph: '▲', pill: 'no_new_data', ageMs: 20000, action: 'show-boards', actionLabel: 'action_show_boards', headline: 'headline_stale', secondary: {kind: 'last_live', at: NOW - 1000}}},
    {row: '17', name: 'the age grows between polls and flips live to stale', status: base(), input: {statusAt: NOW - 9500},
        expect: {id: 'stale', ageMs: 10300}},
    {row: '16', name: 'ten seconds exactly is still live', status: base({last_frame_age_ms: 10000}),
        expect: {id: 'live'}},
    {row: 'F', name: 'ready but neither fresh nor old enough to call', status: base({fresh: false, last_frame_age_ms: 3000}),
        expect: {id: 'checking', tone: 'neutral', glyph: '○', pill: 'checking', action: 'test', actionLabel: 'check_now'}}
];

for (const c of CASES) {
    test('state row ' + c.row + ': ' + c.name, () => {
        const derived = derive(c.status, c.input);
        assert.equal(derived.row, c.row, 'row');
        for (const [key, expected] of Object.entries(c.expect)) assert.deepEqual(derived[key], expected, key);
    });
}

test('every state id is reached by a table case and every case id is known', () => {
    const reached = new Set(CASES.map(c => derive(c.status, c.input).id));
    assert.deepEqual([...reached].sort(), [...M.STATE_IDS].sort());
    assert.equal(M.STATE_IDS.length, 20);
});

test('overlays never replace the headline', () => {
    assert.deepEqual(derive(base({readonly: true})).overlays, ['read_only']);
    assert.equal(derive(base({readonly: true})).id, 'live');
    assert.deepEqual(derive(base({storage_limit: true})).overlays, ['storage_full']);
    const both = derive(base({readonly: true, storage_limit: true, error_code: 'storage_unreadable'}));
    assert.equal(both.id, 'live');
    assert.deepEqual(both.overlays, ['read_only', 'storage_full', 'storage_unreadable']);
    assert.deepEqual(derive(base()).overlays, []);
});

test('a stop in progress stays "stopping" even with a stale stop_failed code', () => {
    assert.equal(derive(idle({runtime: 'stopping', data_state: 'stopping', error_code: 'stop_failed'})).id, 'stopping');
});

test('a loaded but malformed status reads as unavailable, never as turned off', () => {
    for (const bad of ['garbage', 42, [], {}, {runtime: 'running', ready: true}, {enabled: 'yes'}, {enabled: null}]) {
        const derived = derive(bad);
        assert.equal(derived.id, 'status_failed', JSON.stringify(bad));
        assert.equal(derived.action, 'refresh');
        assert.equal(derived.secondary, null, 'there is no usable last-known state to show');
        assert.equal(derived.errorCode, '');
    }
    // not loaded yet is still "loading", and a real disabled status is still "off"
    assert.equal(derive(null, {statusLoaded: false}).id, 'loading');
    assert.equal(derive(idle({enabled: false})).id, 'off');
});

test('an unreadable board list is unknown, not "no boards"', () => {
    const waiting = base({waiting_for_frames: true, fresh: false, last_frame_age_ms: null});
    const emptyOk = {list: [], liveState: 'ok', loaded: true, error: '', liveCount: 0};
    assert.equal(derive(waiting, {nodes: emptyOk}).action, 'flash');
    for (const nodes of [
        {...emptyOk, liveState: 'unreachable', error: 'unavailable', liveCount: null}, {...emptyOk, liveState: 'invalid', error: 'unavailable', liveCount: null},
        {...emptyOk, error: 'unavailable'}, {...emptyOk, liveState: 'unreachable'}, {...emptyOk, liveState: 'not_running'}, {...emptyOk, loaded: false}
    ]) {
        const derived = derive(waiting, {nodes});
        assert.equal(derived.id, 'waiting', JSON.stringify(nodes));
        assert.equal(derived.action, 'show-boards', JSON.stringify(nodes));
        assert.equal(derived.actionLabel, 'action_show_boards');
    }
    // the same rule for the setup steps: the board step is neither done nor the one to fix
    const ready = base({fresh: false, last_fresh_at: null});
    const states = nodes => Object.fromEntries(plain(M.setupSteps(ready, nodes)).map(step => [step.id, step.state]));
    assert.deepEqual(states(emptyOk), {enabled: 'done', writable: 'done', service: 'done', receiver: 'done', board: 'current', data: 'todo'});
    for (const nodes of [
        {...emptyOk, liveState: 'unreachable', error: 'unavailable'}, {...emptyOk, liveState: 'invalid', error: 'unavailable'},
        {...emptyOk, loaded: false}, {list: [BOARD], liveState: 'unreachable', error: 'unavailable', loaded: true}, null
    ]) {
        assert.deepEqual(states(nodes), {enabled: 'done', writable: 'done', service: 'done', receiver: 'done', board: 'todo', data: 'current'}, JSON.stringify(nodes));
    }
});

test('a missing timestamp fails closed: nothing is vouched for as fresh', () => {
    // no time of the last answer: the stored age cannot be aged, so a "fresh" status is not trusted
    for (const statusAt of [null, undefined, 0, NaN]) {
        const derived = derive(base(), {statusAt});
        assert.equal(derived.id, 'checking', String(statusAt));
        assert.equal(derived.action, 'test');
    }
    assert.equal(derive(base({data_state: 'stale', fresh: false, last_frame_age_ms: 20000}), {statusAt: null}).id, 'stale', 'a stale status stays stale');
    assert.equal(derive(base(), {statusAt: NOW}).id, 'live');
});

test('every known error code has a safe next step and its own headline', () => {
    const groups = {
        'runtime/start': ['image_denied', 'image_pull_failed', 'docker_unavailable', 'start_failed', 'timeout', 'cancelled', 'busy', 'container_conflict', 'process_exited'],
        'open-settings': ['docker_readonly', 'invalid_config', 'artifact_missing', 'platform_unsupported', 'source_mismatch', 'unexpected_simulation', 'upstream_auth', 'disabled', 'external_runtime'],
        'runtime/install': ['not_installed', 'download_failed'],
        'allow-changes': ['readonly'],
        test: ['unreachable', 'upstream_error', 'contract_unsupported', 'unknown', 'already_running', 'invalid_node_id', 'storage_full', 'network', 'forbidden']
    };
    const covered = Object.values(groups).flat();
    // stop_failed is row 10 and storage_unreadable is a chip: neither reaches row 11.
    assert.deepEqual([...covered, 'stop_failed', 'storage_unreadable'].sort(), [...M.KNOWN_ERROR_CODES].sort());
    for (const [action, codes] of Object.entries(groups)) {
        for (const code of codes) {
            const managed = derive(idle({mode: 'docker', error_code: code}));
            assert.equal(managed.id, 'error', code);
            assert.equal(managed.errorCode, code, code);
            assert.equal(managed.headline, 'err_' + code, code);
            assert.equal(managed.action, action, code);
            assert.ok(managed.actionLabel, code);
            // An external service has nothing to start or install: it can only be checked again.
            const remote = derive(external({ready: false, fresh: false, error_code: code}));
            assert.ok(!String(remote.action).startsWith('runtime/'), code);
        }
    }
});

test('runtimeOf trusts the runtime field and falls back to the legacy fields', () => {
    for (const runtime of ['external', 'not_started', 'starting', 'running', 'stopping', 'stopped', 'exited', 'stop_unconfirmed']) {
        assert.equal(M.runtimeOf({runtime, ready: true, mode: 'external'}), runtime);
    }
    const legacy = [
        [{mode: 'external', ready: true}, 'external'], [{data_state: 'stopping', error: 'stop_failed'}, 'stopping'],
        [{error: 'stop_failed', data_state: 'starting'}, 'stop_unconfirmed'], [{data_state: 'starting', ready: true}, 'starting'],
        [{ready: true, error: 'process_exited'}, 'running'], [{error: 'process_exited'}, 'exited'], [{data_state: 'stopped'}, 'stopped'],
        [{runtime: 'bogus', ready: true}, 'running'], [{}, 'not_started'], [null, 'not_started']
    ];
    for (const [status, expected] of legacy) assert.equal(M.runtimeOf(status), expected, JSON.stringify(status));
});

test('errorCodeOf prefers the code and reads legacy error text', () => {
    assert.equal(M.errorCodeOf({error_code: 'image_denied', error: 'stop_failed'}), 'image_denied');
    assert.equal(M.errorCodeOf({error: 'stop_failed'}), 'stop_failed');
    assert.equal(M.errorCodeOf({error: 'process_exited'}), 'process_exited');
    assert.equal(M.errorCodeOf({error: 'RuView could not be reached'}), 'unknown');
    assert.equal(M.errorCodeOf({error_code: '', error: ''}), '');
    assert.equal(M.errorCodeOf({}), '');
    assert.equal(M.errorCodeOf(null), '');
});

test('availableViews follows enabled and the source', () => {
    assert.deepEqual(plain(M.availableViews(null)), ['status']);
    assert.deepEqual(plain(M.availableViews({enabled: false, source: 'esp32'})), ['status']);
    assert.deepEqual(plain(M.availableViews({enabled: true, source: 'wifi'})), ['status', 'lab']);
    assert.deepEqual(plain(M.availableViews({enabled: true, source: 'simulated'})), ['status', 'lab']);
    assert.deepEqual(plain(M.availableViews({enabled: true, source: 'esp32'})), ['status', 'boards', 'lab']);
    assert.deepEqual(plain(M.VIEWS), ['status', 'boards', 'lab']);
});

test('blockReason applies the first matching rule', () => {
    const ctx = (over, extra = {}) => ({status: base(over), pending: new Set(), desktopReadonly: false, ...extra});
    const table = [
        // read-only blocks the writers but never stop, check or the harmless actions
        ['runtime/start', ctx({readonly: true}), 'reason_readonly'], ['flash', ctx({readonly: true}), 'reason_readonly'],
        ['runtime/install', ctx({readonly: true, mode: 'native'}), 'reason_readonly'],
        ['runtime/stop', ctx({readonly: true}), ''], ['test', ctx({readonly: true}), ''], ['show-boards', ctx({readonly: true}), ''],
        // a read-only desktop also blocks "allow changes"
        ['runtime/start', ctx({}, {desktopReadonly: true}), 'reason_desktop_readonly'], ['flash', ctx({}, {desktopReadonly: true}), 'reason_desktop_readonly'],
        ['allow-changes', ctx({}, {desktopReadonly: true}), 'reason_desktop_readonly'], ['runtime/stop', ctx({}, {desktopReadonly: true}), ''],
        ['test', ctx({}, {desktopReadonly: true}), ''], ['allow-changes', ctx({readonly: true}), ''],
        // a pending action reads "working" before anything else
        ['test', ctx({}, {pending: new Set(['test'])}), 'working'], ['runtime/start', ctx({readonly: true}, {pending: new Set(['runtime/start'])}), 'working'],
        ['runtime/stop', ctx({}, {pending: new Set(['test'])}), ''],
        // external services are not started or stopped from here
        ['runtime/stop', ctx({mode: 'external'}), 'external_runtime'], ['runtime/start', ctx({mode: 'external'}), 'external_runtime'], ['test', ctx({mode: 'external'}), ''],
        // install is native-only
        ['runtime/install', ctx({mode: 'docker'}), 'install_native'], ['runtime/install', ctx({mode: 'native'}), ''],
        // no second start while one is running
        ['runtime/start', ctx({runtime: 'starting', ready: false}), 'reason_wait_start'], ['runtime/stop', ctx({runtime: 'starting', ready: false}), ''],
        // a disabled integration needs enabling first, except for the actions that explain or open the settings
        ['test', ctx({enabled: false}), 'enable_first'], ['runtime/start', ctx({enabled: false}), 'enable_first'], ['flash', ctx({enabled: false}), 'enable_first'],
        ['runtime/stop', ctx({enabled: false}), ''], ['runtime/stop', ctx({enabled: false, runtime: 'stop_unconfirmed', error_code: 'stop_failed'}), ''],
        ['open-settings', ctx({enabled: false}), ''], ['refresh', ctx({enabled: false}), ''], ['copy-diagnostics', ctx({enabled: false}), ''],
        ['about', ctx({enabled: false}), ''], ['show-boards', ctx({enabled: false}), ''],
        ['flash', ctx({}), ''], ['test', ctx({}), '']
    ];
    for (const [action, context, expected] of table) {
        assert.equal(M.blockReason(action, context), expected, action + ' ' + JSON.stringify({readonly: context.status.readonly, enabled: context.status.enabled, mode: context.status.mode, rt: context.status.runtime, dr: context.desktopReadonly}));
    }
    assert.equal(M.blockReason('test', {status: null, pending: new Set(), desktopReadonly: false}), '');
    assert.equal(M.blockReason('test'), '');
});

test('setupSteps lists the steps of the mode and source and marks the first open one current', () => {
    const ids = steps => plain(steps).map(step => step.id);
    const states = steps => Object.fromEntries(plain(steps).map(step => [step.id, step.state]));
    // M30: UDP enabled in the settings is not enough, the receiver must really listen.
    const notRunning = M.setupSteps(idle({udp_enabled: true, udp_ready: false, receiver_configured: true}), {list: []});
    assert.deepEqual(ids(notRunning), ['enabled', 'writable', 'service', 'receiver', 'board', 'data']);
    assert.deepEqual(states(notRunning), {enabled: 'done', writable: 'done', service: 'current', receiver: 'todo', board: 'todo', data: 'todo'});
    const listening = M.setupSteps(base({fresh: false, last_fresh_at: null, udp_ready: false}), {list: []});
    assert.deepEqual(states(listening), {enabled: 'done', writable: 'done', service: 'done', receiver: 'current', board: 'todo', data: 'todo'});
    const board = M.setupSteps(base({fresh: false, last_fresh_at: null}), {list: [BOARD]});
    assert.deepEqual(states(board), {enabled: 'done', writable: 'done', service: 'done', receiver: 'done', board: 'done', data: 'current'});
    // other modes and sources
    assert.deepEqual(ids(M.setupSteps(wifi({fresh: false, last_fresh_at: null}), {list: []})), ['enabled', 'service', 'data']);
    assert.deepEqual(ids(M.setupSteps(external({fresh: false, last_fresh_at: null}), {list: []})), ['enabled', 'checked', 'board', 'data']);
    assert.deepEqual(ids(M.setupSteps(external({source: 'wifi', fresh: false, last_fresh_at: null}), {list: []})), ['enabled', 'checked', 'data']);
    assert.deepEqual(states(M.setupSteps(external({fresh: false, last_fresh_at: null, ready: false, observed_at: null}), {list: []})),
        {enabled: 'done', checked: 'current', board: 'todo', data: 'todo'});
    // fix actions
    const actions = Object.fromEntries(plain(notRunning).map(step => [step.id, step.action]));
    assert.deepEqual(actions, {enabled: 'open-settings', writable: 'allow-changes', service: 'runtime/start', receiver: 'open-settings', board: 'flash', data: 'show-boards'});
    assert.equal(plain(M.setupSteps(idle({readonly: true}), {list: []}, {desktopReadonly: true})).find(step => step.id === 'writable').action, 'open-settings');
    assert.equal(plain(M.setupSteps(wifi({fresh: false, last_fresh_at: null}), null)).find(step => step.id === 'data').action, null);
    // data has already arrived, the source is a demo, or nothing is loaded: no setup to show
    assert.equal(M.setupSteps(base(), {list: [BOARD]}), null);
    assert.equal(M.setupSteps(idle({last_fresh_at: ago(60000)}), {list: []}), null);
    assert.equal(M.setupSteps(idle({source: 'simulated'}), {list: []}), null);
    assert.equal(M.setupSteps(null, {list: []}), null);
    assert.equal(M.setupSteps({}, {list: []}), null);
});

test('boardFreshness mirrors the backend rule: real, active, sequenced CSI at most 10 s old', () => {
    const live = {csi_status: 'active', csi_sequence: 5};
    const opts = (over = {}) => ({source: 'esp32', simulated: false, nodesError: '', ageMs: 100, ...over});
    assert.equal(M.boardFreshness(live, opts({ageMs: 10000})), 'live');
    assert.equal(M.boardFreshness(live, opts({ageMs: 0})), 'live');
    assert.equal(M.boardFreshness(live, opts({ageMs: 10001})), 'stale');
    assert.equal(M.boardFreshness(live, opts({simulated: true})), 'stale');
    assert.equal(M.boardFreshness({csi_status: 'active', csi_sequence: null}, opts()), 'stale');
    assert.equal(M.boardFreshness({csi_status: 'active'}, opts()), 'stale');
    assert.equal(M.boardFreshness(live, opts({source: 'wifi'})), 'stale');
    assert.equal(M.boardFreshness(live, opts({nodesError: 'unavailable'})), 'stale');
    assert.equal(M.boardFreshness(live, opts({ageMs: null})), 'stale');
    assert.equal(M.boardFreshness({csi_status: 'stale', csi_sequence: 5}, opts()), 'stale');
    assert.equal(M.boardFreshness({csi_status: 'offline', csi_sequence: 5}, opts()), 'offline');
    assert.equal(M.boardFreshness({csi_status: 'offline', csi_sequence: null}, opts({simulated: true})), 'offline');
});

test('nodeAgeMs and liveBoardCount age a board list between polls', () => {
    assert.equal(M.nodeAgeMs({csi_last_seen_ms: 1000}, 100, 600), 1500);
    assert.equal(M.nodeAgeMs({csi_last_seen_ms: 1000}, 100, 100), 1000);
    // an unknown timestamp is an unknown age, never "just now"
    assert.equal(M.nodeAgeMs({csi_last_seen_ms: 1000}), null);
    assert.equal(M.nodeAgeMs({csi_last_seen_ms: 1000}, 100), null);
    assert.equal(M.nodeAgeMs({csi_last_seen_ms: 1000}, undefined, 600), null);
    assert.equal(M.nodeAgeMs({csi_last_seen_ms: 1000}, NaN, 600), null);
    assert.equal(M.nodeAgeMs({csi_last_seen_ms: null}, 0, 10), null);
    assert.equal(M.nodeAgeMs({csi_last_seen_ms: -1}, 0, 10), null);
    assert.equal(M.nodeAgeMs({}, 0, 10), null);
    assert.equal(M.nodeAgeMs(null, 0, 10), null);
    const nodes = {
        list: [
            {node_id: 1, csi_status: 'active', csi_sequence: 1, csi_last_seen_ms: 2000},
            {node_id: 2, csi_status: 'active', csi_sequence: 2, csi_last_seen_ms: 20000},
            {node_id: 3, csi_status: 'offline', csi_sequence: 3, csi_last_seen_ms: 1000}
        ],
        loaded: true, liveState: 'ok', error: '', receivedAt: 1000
    };
    const status = {source: 'esp32', simulated: false};
    assert.equal(M.liveBoardCount(nodes, status, 2000), 1);
    assert.equal(M.liveBoardCount(nodes, status, 10000), 0, 'board 1 is 11 s old by now');
    assert.equal(M.liveBoardCount(nodes, {source: 'esp32', simulated: true}, 2000), 0);
    assert.equal(M.liveBoardCount(nodes, {source: 'wifi'}, 2000), null);
    assert.equal(M.liveBoardCount({...nodes, error: 'unavailable'}, status, 2000), null);
    assert.equal(M.liveBoardCount({...nodes, liveState: 'not_applicable'}, status, 2000), null);
    assert.equal(M.liveBoardCount({...nodes, liveState: 'not_running'}, status, 2000), null);
    assert.equal(M.liveBoardCount({...nodes, loaded: false}, status, 2000), null);
    assert.equal(M.liveBoardCount({...nodes, liveState: 'unreachable'}, status, 2000), null);
    assert.equal(M.liveBoardCount({...nodes, liveState: 'invalid'}, status, 2000), null);
    assert.equal(M.liveBoardCount(null, status, 2000), null);
    // fail closed: without the arrival time or the current time no board is vouched for as live
    const { receivedAt, ...unstamped } = nodes;
    assert.equal(M.liveBoardCount(unstamped, status, 2000), 0);
    assert.equal(M.liveBoardCount(nodes, status), 0);
    assert.equal(M.liveBoardCount(nodes, status, NaN), 0);
});

test('mergeNodes keeps the last RSSI of an offline board within one scope and normalizes live_state', () => {
    const online = {node_id: 7, csi_status: 'active', rssi_dbm: -60};
    const offline = {node_id: 7, csi_status: 'offline', rssi_dbm: null};
    const first = plain(M.mergeNodes(null, {nodes: [online], scope: 'managed', live_state: 'ok'}));
    assert.deepEqual(first, {list: [online], scope: 'managed', liveState: 'ok', error: ''});
    const kept = plain(M.mergeNodes(first, {nodes: [offline], scope: 'managed', live_state: 'ok'}));
    assert.equal(kept.list[0].rssi_dbm, -60);
    assert.equal(kept.list[0].csi_status, 'offline');
    assert.equal(plain(M.mergeNodes(first, {nodes: [offline], scope: 'external'})).list[0].rssi_dbm, null, 'another scope keeps nothing');
    assert.equal(plain(M.mergeNodes({list: [{node_id: 7, csi_status: 'offline', rssi_dbm: null}], scope: 'managed'}, {nodes: [offline], scope: 'managed'})).list[0].rssi_dbm, null);
    assert.equal(plain(M.mergeNodes(first, {nodes: [{...offline, rssi_dbm: -70}], scope: 'managed'})).list[0].rssi_dbm, -70, 'a new value wins');
    assert.equal(plain(M.mergeNodes(first, {nodes: [{node_id: 7, csi_status: 'active', rssi_dbm: null}], scope: 'managed'})).list[0].rssi_dbm, null, 'only offline boards keep the old value');
    // live_state: known values pass, an older backend only sends `available`
    for (const [data, liveState, error] of [
        [{nodes: [], live_state: 'not_applicable'}, 'not_applicable', ''], [{nodes: [], live_state: 'not_running'}, 'not_running', ''],
        [{nodes: [], live_state: 'unreachable'}, 'unreachable', 'unavailable'], [{nodes: [], live_state: 'invalid'}, 'invalid', 'unavailable'],
        [{nodes: [], available: false}, 'unreachable', 'unavailable'], [{nodes: [], live_state: 'surprise'}, 'ok', ''], [[], 'ok', '']
    ]) {
        const merged = plain(M.mergeNodes(null, data));
        assert.equal(merged.liveState, liveState, JSON.stringify(data));
        assert.equal(merged.error, error, JSON.stringify(data));
    }
    assert.throws(() => M.mergeNodes(null, {nodes: 'x'}), /Invalid nodes response/);
    assert.throws(() => M.mergeNodes(null, null), /Invalid nodes response/);
});

test('ageParts splits a duration for the views and rejects bad values', () => {
    assert.equal(M.ageParts(null), null);
    assert.equal(M.ageParts(undefined), null);
    assert.equal(M.ageParts(NaN), null);
    assert.equal(M.ageParts(-1), null);
    assert.equal(M.ageParts(Infinity), null);
    const table = [
        [0, 0, 'ms'], [999, 999, 'ms'], [1000, 1, 's'], [1500, 1.5, 's'], [2000, 2, 's'], [9960, 10, 's'], [10400, 10, 's'], [59000, 59, 's'],
        [60000, 1, 'min'], [150000, 3, 'min'], [3599999, 60, 'min'], [3600000, 1, 'h'], [7200000, 2, 'h']
    ];
    for (const [ms, value, unit] of table) assert.deepEqual(plain(M.ageParts(ms)), {value, unit}, String(ms));
});

test('formatters give dashes for missing values and locale-aware text otherwise', () => {
    assert.equal(M.formatBytes(null, 'en'), '—');
    assert.equal(M.formatBytes(undefined, 'en'), '—');
    assert.equal(M.formatBytes(-5, 'en'), '—');
    assert.match(M.formatBytes(0, 'en'), /^0\s?MB$/);
    assert.match(M.formatBytes(5 * 1048576, 'en'), /^5\s?MB$/);
    assert.match(M.formatBytes(300 * 1048576, 'de'), /^300\s?MB$/);
    assert.match(M.formatBytes(1.5 * 1073741824, 'en'), /^1\.5\s?GB$/);
    assert.match(M.formatBytes(2 * 1073741824, 'de-DE'), /^2\s?GB$/);
    assert.match(M.formatBytes(1048576, 'not a language'), /MB$/, 'a bad language falls back instead of throwing');
    assert.equal(M.formatDuration(0), '0:00');
    assert.equal(M.formatDuration(7000), '0:07');
    assert.equal(M.formatDuration(125000), '2:05');
    assert.equal(M.formatDuration(3725000), '1:02:05');
    assert.equal(M.formatDuration(-5000), '0:00');
    assert.equal(M.formatDuration(null), '—');
    assert.equal(M.formatTime(null, 'en'), '—');
    const sameDay = new Intl.DateTimeFormat('en', {hour: '2-digit', minute: '2-digit'}).format(new Date(NOW));
    assert.equal(M.formatTime(NOW, 'en', NOW), sameDay);
    assert.notEqual(M.formatTime(NOW - 3 * 86400000, 'en', NOW), new Intl.DateTimeFormat('en', {hour: '2-digit', minute: '2-digit'}).format(new Date(NOW - 3 * 86400000)), 'an older time also shows its date');
    assert.equal(M.fill('Step {n} of {total}', {n: 2, total: 4}), 'Step 2 of 4');
    assert.equal(M.fill('{n} h', {n: 0}), '0 h');
    assert.equal(M.fill('Last {age} ago', {}), 'Last {age} ago');
    assert.equal(M.fill('Last {age} ago', {age: null}), 'Last {age} ago');
    assert.equal(M.fill('{a}{a}', {a: 'x'}), 'xx');
    assert.equal(M.fill(null, {}), '');
});

test('phaseStep counts the steps of the mode', () => {
    assert.deepEqual(plain(M.phaseStep('docker', 'preparing')), {n: 1, total: 4});
    assert.deepEqual(plain(M.phaseStep('docker', 'pulling_image')), {n: 2, total: 4});
    assert.deepEqual(plain(M.phaseStep('docker', 'creating')), {n: 3, total: 4});
    assert.deepEqual(plain(M.phaseStep('docker', 'waiting_ready')), {n: 4, total: 4});
    assert.deepEqual(plain(M.phaseStep('native', 'verifying')), {n: 2, total: 4});
    assert.equal(M.phaseStep('docker', 'verifying'), null);
    assert.equal(M.phaseStep('native', 'pulling_image'), null);
    assert.equal(M.phaseStep('external', 'creating'), null);
    assert.equal(M.phaseStep('docker', ''), null);
    assert.equal(M.phaseStep('docker', 'downloading'), null);
});

test('diagnostics is plain key=value text without the legacy error, URLs or secrets', () => {
    const status = base({
        error: 'dial tcp http://10.0.0.5:8080/x failed token=abc123',
        error_detail: 'HTTP 403 from https://registry.example/v2/ token=secret99',
        hints: ['sensing_stale', 'storage_limit'], token: 'leak-me-1', api_key: 'leak-me-2'
    });
    const text = M.diagnostics(status, {list: [BOARD], liveState: 'ok', liveCount: 1}, NOW);
    assert.match(text, /^RuView diagnostics 2026-10-25T12:00:00\.000Z\n/);
    assert.doesNotMatch(text, /(^|\n)error=/);
    assert.doesNotMatch(text, /https?:\/\//);
    assert.doesNotMatch(text, /abc123|secret99|leak-me|10\.0\.0\.5/);
    for (const key of ['revision', 'revision_verified', 'mode', 'source', 'enabled', 'readonly', 'agent_access', 'runtime', 'phase', 'phase_since', 'ready',
        'data_state', 'fresh', 'simulated', 'waiting_for_frames', 'last_frame_age_ms', 'error_code', 'error_detail', 'observed_at', 'last_fresh_at',
        'stop_reason', 'stopped_at', 'udp_scope', 'udp_enabled', 'udp_ready', 'receiver_configured', 'storage_used_bytes', 'storage_budget_bytes',
        'storage_limit', 'recording_session', 'recording_ack', 'recording_at', 'hints']) {
        assert.match(text, new RegExp('(^|\\n)' + key + '='), key);
    }
    assert.match(text, /\nruntime=running\n/);
    assert.match(text, /\nerror_detail=HTTP 403 from \[url\] \[redacted\]\n/);
    assert.match(text, /\nhints=sensing_stale,storage_limit\n/);
    assert.match(text, /\nboards live_state=ok known=1 live=1$/);
    // an auth scheme word does not shield the credential, and JSON-style quoted keys are caught too
    const detail = value => M.diagnostics(base({error_detail: value}), null, NOW);
    const basic = detail('Authorization: Basic dXNlcjpzdXBlcnNlY3JldA== was rejected');
    assert.doesNotMatch(basic, /dXNlcjpz|supersecret|Basic/);
    assert.match(basic, /\nerror_detail=\[redacted\]\n/, 'the header is redacted to the end of its line');
    assert.doesNotMatch(detail('authorization: bearer abc.def-ghi_jkl123'), /abc\.def|ghi_jkl123/);
    assert.doesNotMatch(detail('Proxy Authorization = Digest username="u", response="r3sp0nse"'), /r3sp0nse|username/);
    const json = detail('body {"token":"jsonsecret77","access_token": "tok-88","password":"pw with space","api-key":"k-99"} end');
    assert.doesNotMatch(json, /jsonsecret77|tok-88|pw with space|k-99/);
    assert.match(json, /\nerror_detail=body \{[^\n]*\} end\n/);
    assert.doesNotMatch(detail("config { 'secret' : 'sq-secret-1' } done"), /sq-secret-1/);
    assert.match(detail('token="quoted \\"inner\\" rest" ok'), /\nerror_detail=\[redacted\] ok\n/, 'an escaped quote does not end the value early');
    // one value never breaks out of its line
    const multiline = M.diagnostics(base({error_detail: 'line one\nfake_key=1\r\nline three'}), null, NOW);
    assert.match(multiline, /\nerror_detail=line one fake_key=1 line three\n/);
    assert.doesNotMatch(multiline, /\nfake_key=/);
    assert.match(text, /\nstatus=current\n/);
    assert.doesNotMatch(text, /last_success=/);
    // a missing status or board list does not throw
    assert.match(M.diagnostics(null, null, NOW), /^RuView diagnostics [^\n]+\n/);
});

test('diagnostics never reads an unknown status as "off" (H9)', () => {
    const nodes = {list: [BOARD], liveState: 'ok', liveCount: 1};
    const keys = /(^|\n)(runtime|enabled|mode|source|ready|readonly)=/;
    // never loaded: the state is named and none of the status fields appear
    for (const status of [null, undefined, {}, 'garbage', []]) {
        const text = M.diagnostics(status, nodes, NOW, {statusLoaded: false, statusFailed: true});
        assert.match(text, /\nstatus=not_loaded\n/, JSON.stringify(status));
        assert.doesNotMatch(text, keys, JSON.stringify(status));
        assert.match(text, /\nboards live_state=ok known=1 live=1$/, 'the board list still reports');
    }
    assert.match(M.diagnostics(null, null, NOW), /\nstatus=not_loaded\n/);
    // loaded but malformed: not "off" either
    const invalid = M.diagnostics({runtime: 'running', ready: true}, nodes, NOW, {statusLoaded: true, statusFailed: false});
    assert.match(invalid, /\nstatus=invalid\n/);
    assert.doesNotMatch(invalid, keys);
    // failed after a success: the last values stay, labelled, with the time of the last success
    const last = M.diagnostics(base({data_state: 'live_unverified'}), nodes, NOW, {statusLoaded: true, statusFailed: true, statusAt: NOW - 5000});
    assert.match(last, /\nstatus=last_known\nlast_success=2026-10-25T11:59:55\.000Z\n/);
    assert.match(last, /\nenabled=true\n/);
    assert.match(last, /\nruntime=running\n/);
    // an ordinary loaded status
    const current = M.diagnostics(base(), nodes, NOW, {statusLoaded: true, statusFailed: false, statusAt: NOW});
    assert.match(current, /\nstatus=current\n/);
    assert.doesNotMatch(current, /last_success=/);
    assert.match(current, /\nruntime=running\n/);
});

test('the model is a classic DOM-free script under the line budget', () => {
    assert.ok(source.split('\n').length < 900, 'ruview-model.js stays under 900 lines');
    const code = source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
    assert.match(code, /window\.RuViewModel\s*=/);
    assert.doesNotMatch(code, /^\s*(import|export)\s/m, 'a classic script has no module syntax');
    assert.doesNotMatch(code, /\b(document|fetch|localStorage|navigator|requestAnimationFrame)\b/, 'no DOM or network access');
    assert.doesNotMatch(code, /\bt\(/, 'the model returns key ids and never translates');
    assert.deepEqual([...M.KNOWN_ERROR_CODES].filter(code => code !== 'network' && code !== 'forbidden'), [...M.KNOWN_ERROR_CODES].filter(code => code !== 'network' && code !== 'forbidden').sort(), 'the Go codes are sorted');
    assert.ok(M.KNOWN_ERROR_CODES.includes('network') && M.KNOWN_ERROR_CODES.includes('forbidden'));
    assert.equal(M.KNOWN_ERROR_CODES.length, 32);
});

// ---- Batch C checks: stylesheet, file sizes and the Go error codes ------------------------------------------------

const readRepo = path => readFileSync(new URL('../../' + path, import.meta.url), 'utf8');

test('ruview.css has no colour literals and no ID selectors (M6)', () => {
    const css = readRepo('ui/css/ruview.css').replace(/\/\*[\s\S]*?\*\//g, '');
    assert.doesNotMatch(css, /#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})\b/i, 'colours come from tokens or system colours');
    assert.ok(!css.includes('#'), 'no ID selectors and no other hash in the stylesheet');
    assert.doesNotMatch(css, /\bcc-/, 'no cookie-banner-prone class names');
});

test('each RuView desktop script stays under the 900 line budget', () => {
    for (const name of ['ruview-model.js', 'ruview-store.js', 'ruview-views.js', 'ruview.js']) {
        const lines = readRepo('ui/js/desktop/apps/' + name).split('\n').length;
        assert.ok(lines < 900, name + ' has ' + lines + ' lines');
    }
});

test('every Go error code is known to the model', () => {
    const go = readRepo('internal/ruview/errors.go');
    const codes = [...go.matchAll(/\bCode\w+\s*=\s*"(\w+)"/g)].map(match => match[1]);
    assert.ok(codes.length >= 30, 'the Go code list was found (' + codes.length + ')');
    for (const code of codes) assert.ok(M.KNOWN_ERROR_CODES.includes(code), 'model does not know the Go code ' + code);
});
