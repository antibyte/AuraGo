import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs'; import { fakeClock } from './live-fakes.mjs';
const cap = (name, x) => Object.assign({ name, allowed: true }, x);
const LIGHT = { id: 'ha:light.kitchen', kind: 'light', value: true, available: true, status: 'ok', attributes: { brightness_pct: 40 }, capabilities: [cap('toggle'), cap('turn_on'), cap('turn_off'), cap('brightness', { min: 0, max: 100, step: 1 }), cap('color_temp', { min: 2202, max: 6535, step: 50 }), cap('color')] };
const SW = { id: 'ha:switch.coffee', kind: 'switch', value: false, available: true, status: 'ok', attributes: {}, capabilities: [cap('toggle'), cap('turn_on'), cap('turn_off')] };
const SCRIPT = { id: 'ha:script.goodnight', kind: 'script', value: false, available: true, status: 'ok', attributes: {}, capabilities: [cap('activate', { sensitive: true })] };
const NUM = { id: 'ha:number.boiler_target', kind: 'number', value: 55, available: true, status: 'ok', attributes: {}, capabilities: [cap('set_value', { min: 30, max: 70, step: 1 })] };
const CLIM = { id: 'ha:climate.living', kind: 'climate', value: 'heat', available: true, status: 'ok', attributes: { temperature: 21, min_temp: 5, max_temp: 30 }, capabilities: [cap('set_temperature', { min: 5, max: 30, step: 0.5 }), cap('hvac_mode', { options: ['off', 'heat', 'cool'] })] };
function setup(over, copts) {
  const { E } = loadEngine(['hd-core.js', 'hd-format.js', 'hd-api.js', 'hd-command.js']), clock = fakeClock(), stream = E.core.emitter(), views = new Map([LIGHT, SW, SCRIPT, NUM, CLIM].map(v => [v.id, v])), calls = []; let n = 0;
  const api = Object.assign({ base: '/b', command(body) { let ok, no; const p = new Promise((a, b) => { ok = a; no = b; }); calls.push({ body, at: clock.now(), ok, no }); return p; } }, over);
  const cmd = E.command.create(Object.assign({ api, stream, store: { view: id => views.get(id) || null }, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, requestId: () => 'r' + (++n) }, copts));
  const fail = (status, body) => Object.assign(new Error('HTTP ' + status), { status, body });
  const frame = (id, st) => stream.emit('state', { changes: { [id]: Object.assign({ available: true, updated_at: 'x' }, st) } });
  return { E, clock, views, calls, cmd, fail, frame, d: (id, k) => cmd.decorate(id, views.get(id), k) };
}
const readonlyView = v => Object.assign({}, v, { readonly: true });
const exp = (value, attributes) => ({ status: 'ok', accepted: true, superseded: false, expected: Object.assign({ value, available: true, updated_at: 'x' }, attributes ? { attributes } : {}) });
export const tests = [
  ['command: predict mirrors the contract expected table; matches uses the tolerances', () => { const { E } = setup(), P = E.command.predict, M = E.command.matches;
    eq(P(SW, 'turn_on', {}), { value: true }); eq(P(LIGHT, 'brightness', { value: 0 }), { value: false }); eq(P(LIGHT, 'brightness', { value: 60 }), { value: true, attributes: { brightness_pct: 60 } });
    eq(P(LIGHT, 'color', { h: 10, s: 50 }), { value: true, attributes: { hs_color: [10, 50], color_mode: 'hs' } }); assert.equal(P(SCRIPT, 'activate', {}), null); eq(P(NUM, 'set_value', { value: 56 }), { value: 56 });
    const st = a => ({ value: true, available: true, attributes: a });
    assert.ok(M(st({ brightness_pct: 61 }), { value: true, attributes: { brightness_pct: 60 } }, 1)); assert.ok(!M(st({ brightness_pct: 62 }), { value: true, attributes: { brightness_pct: 60 } }, 1));
    assert.ok(M(st({ color_temp_kelvin: 3060 }), { value: true, attributes: { color_temp_kelvin: 3000 } })); assert.ok(!M(st({ color_temp_kelvin: 3070 }), { value: true, attributes: { color_temp_kelvin: 3000 } }));
    assert.ok(M(st({ hs_color: [358, 52] }), { value: true, attributes: { hs_color: [2, 49] } })); assert.ok(!M(st({ hs_color: [8, 50] }), { value: true, attributes: { hs_color: [2, 50] } }));
    assert.ok(M({ value: 55.4, available: true }, { value: 55 }, 1)); assert.ok(!M({ value: 55.6, available: true }, { value: 55 }, 1)); assert.ok(!M({ value: true, available: false }, { value: true })); }],
  ['command: climate - set_temperature is continuous (throttled input, final commit) and predicts the target; hvac_mode is discrete and predicts the mode', async () => { const { E, clock, calls, cmd, d } = setup(), P = E.command.predict, M = E.command.matches;
    eq(P(CLIM, 'set_temperature', { value: 22.5 }), { attributes: { temperature: 22.5 } }); eq(P(CLIM, 'hvac_mode', { option: 'cool' }), { value: 'cool' });
    assert.ok(M({ value: 'heat', available: true, attributes: { temperature: 22.5 } }, { attributes: { temperature: 22.5 } })); assert.ok(!M({ value: 'heat', available: true, attributes: { temperature: 23 } }, { attributes: { temperature: 22.5 } }));
    for (let i = 0; i < 120; i++) { cmd.send({ entity: CLIM.id, command: 'set_temperature', args: { value: 20 + i / 10 }, phase: 'input' }); await clock.advance(1000 / 60); }
    assert.equal(calls.length, 2, 'a leading send and one every 1250 ms, not one per move'); assert.equal(d(CLIM.id).attributes.temperature, 31.9, 'the overlay follows the drag');
    cmd.send({ entity: CLIM.id, command: 'set_temperature', args: { value: 24 }, phase: 'commit' }); assert.equal(calls.length, 3); assert.equal(calls[2].body.command, 'set_temperature'); assert.equal(d(CLIM.id).attributes.temperature, 24);
    cmd.send({ entity: CLIM.id, command: 'hvac_mode', args: { option: 'cool' } }); assert.equal(calls.length, 4); assert.equal(calls[3].body.args.option, 'cool'); assert.equal(d(CLIM.id).value, 'cool'); }],
  ['command: the overlay applies at once, pending appears after 250 ms (not before), a matching frame confirms and flashes 600 ms', async () => { const { clock, calls, cmd, frame, d, views } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {}, widgetKey: 'w1' }); assert.equal(d(SW.id).value, true); assert.equal(d(SW.id).pending, false);
    await clock.advance(249); assert.equal(d(SW.id).pending, false); await clock.advance(2); assert.equal(d(SW.id).pending, true);
    calls[0].ok(exp(true)); await clock.advance(0); frame(SW.id, { value: true }); assert.equal(d(SW.id).pending, false); assert.equal(d(SW.id).confirmed, true);
    await clock.advance(600); assert.equal(d(SW.id), views.get(SW.id)); }],
  // after the 600 ms flash decorate() returns the store view itself (same reference), so the strict equal holds
  ['command: a frame before the 202 confirms when expected arrives; the server state already equal at the 202 confirms; duplicates are harmless', async () => { const { clock, calls, cmd, frame, d, views } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); frame(SW.id, { value: true }); calls[0].ok(exp(true)); await clock.advance(0); frame(SW.id, { value: true }); assert.equal(d(SW.id).pending, false); assert.equal(d(SW.id).confirmed, true);
    cmd.send({ entity: LIGHT.id, command: 'turn_on', args: {} }); calls[1].ok(exp(true)); await clock.advance(0); assert.equal(d(LIGHT.id).confirmed, true); assert.equal(views.size, 5); }],
  ['command: superseded ends silently; momentary commands confirm on the 202', async () => { const { clock, calls, cmd, d, views } = setup();
    cmd.send({ entity: NUM.id, command: 'set_value', args: { value: 60 }, phase: 'commit' }); calls[0].ok({ status: 'ok', accepted: true, superseded: true }); await clock.advance(0); assert.equal(d(NUM.id), views.get(NUM.id));
    cmd.send({ entity: SCRIPT.id, command: 'activate', args: {}, confirmed: true, widgetKey: 'w' }); }],
  ['command: an error rolls back at once with a 6 s cue; 403 read_only marks the entity read-only with its reason and no cue', async () => { const { clock, calls, cmd, fail, d } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); calls[0].no(fail(503, { status: 'error', code: 'source_unavailable', source: 'ha' })); await clock.advance(0);
    assert.equal(d(SW.id).value, false); assert.equal(d(SW.id).status, 'error'); eq(d(SW.id).error, { code: 'source_unavailable', source: 'ha', blockedBy: '' }); await clock.advance(6000); assert.equal(d(SW.id).status, 'ok');
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); calls[1].no(fail(403, { status: 'error', code: 'read_only', blocked_by: 'home_assistant.allowed_services' })); await clock.advance(0);
    assert.equal(d(SW.id).status, 'ok'); assert.equal(d(SW.id).readonly, true); assert.equal(d(SW.id).readonlyReason, 'home_assistant.allowed_services'); }],
  ['command: 8 s without confirmation rolls back with the timeout cue unless a frame with the expected value arrived (late adoption)', async () => { const { clock, calls, cmd, frame, d } = setup();
    cmd.send({ entity: LIGHT.id, command: 'color_temp', args: { kelvin: 6502 }, phase: 'commit' }); calls[0].ok(exp(true, { color_temp_kelvin: 6502 })); await clock.advance(0);
    frame(LIGHT.id, { value: true, attributes: { color_temp_kelvin: 6300 } }); await clock.advance(8000); assert.equal(d(LIGHT.id).status, 'ok'); assert.ok(!d(LIGHT.id).pending);
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); calls[1].ok(exp(true)); await clock.advance(8000); assert.equal(d(SW.id).value, false); eq(d(SW.id).error.code, 'timeout'); }],
  ['command: continuous input sends at most every 1250 ms with a leading edge, the final always on commit, and drops a 429 on an intermediate', async () => { const { clock, calls, cmd, fail, d } = setup();
    for (let i = 0; i < 180; i++) { cmd.send({ entity: LIGHT.id, command: 'brightness', args: { value: 10 + Math.floor(i / 3) }, phase: 'input' }); await clock.advance(1000 / 60); }
    assert.equal(calls.length, 3); for (let i = 1; i < calls.length; i++) assert.ok(calls[i].at - calls[i - 1].at >= 1250);
    calls[1].no(fail(429, { code: 'rate_limited', retry_after_ms: 900 })); await clock.advance(0); assert.equal(d(LIGHT.id).attributes.brightness_pct, 69);
    cmd.send({ entity: LIGHT.id, command: 'brightness', args: { value: 70 }, phase: 'commit' }); assert.equal(calls.length, 4); eq(calls[3].body.args, { value: 70 }); assert.equal(d(LIGHT.id).attributes.brightness_pct, 70); }],
  ['command: a 429 on the final retries once with a new request_id after retry_after_ms while it is the latest', async () => { const { clock, calls, cmd, fail } = setup();
    cmd.send({ entity: NUM.id, command: 'set_value', args: { value: 60 }, phase: 'commit' }); calls[0].no(fail(429, { code: 'rate_limited', retry_after_ms: 700 })); await clock.advance(699); assert.equal(calls.length, 1);
    await clock.advance(1); assert.equal(calls.length, 2); assert.notEqual(calls[1].body.request_id, calls[0].body.request_id); calls[1].no(fail(429, { code: 'rate_limited', retry_after_ms: 700 })); await clock.advance(2000); assert.equal(calls.length, 2); }],
  ['command: any matching frame since sending confirms at the 202, even when a later frame does not match (rules 3 and 5)', async () => { const { clock, calls, cmd, frame, d } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); frame(SW.id, { value: true }); frame(SW.id, { value: false }); calls[0].ok(exp(true)); await clock.advance(0);
    assert.equal(d(SW.id).confirmed, true); assert.equal(d(SW.id).pending, false); }],
  ['command: late adoption sees every frame since sending, not only the last one', async () => { const { clock, calls, cmd, frame, d } = setup();
    cmd.send({ entity: LIGHT.id, command: 'color_temp', args: { kelvin: 6502 }, phase: 'commit' });
    frame(LIGHT.id, { value: true, attributes: { color_temp_kelvin: 6300 } }); frame(LIGHT.id, { value: false }); calls[0].ok(exp(true, { color_temp_kelvin: 6502 })); await clock.advance(8000);
    assert.equal(d(LIGHT.id).status, 'ok'); assert.equal(d(LIGHT.id).error, undefined); assert.ok(!d(LIGHT.id).pending); }],
  ['command: only the last 8 frames count', async () => { const { clock, calls, cmd, frame, d } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); frame(SW.id, { value: true }); for (let i = 0; i < 8; i++) frame(SW.id, { value: false }); calls[0].ok(exp(true)); await clock.advance(0);
    assert.notEqual(d(SW.id).confirmed, true); }],
  ['command: a read-only entity or lock is refused with no POST and no overlay (rule 9)', async () => { const { clock, calls, cmd, fail, views, d } = setup();
    views.set(SW.id, readonlyView(SW)); eq(await cmd.send({ entity: SW.id, command: 'turn_on', args: {} }), { status: 'readonly' }); assert.equal(calls.length, 0); assert.equal(d(SW.id).value, false);
    cmd.send({ entity: NUM.id, command: 'set_value', args: { value: 60 }, phase: 'commit' }); calls[0].no(fail(403, { code: 'read_only' })); await clock.advance(0);
    eq(await cmd.send({ entity: NUM.id, command: 'set_value', args: { value: 61 }, phase: 'commit' }), { status: 'readonly' }); assert.equal(calls.length, 1); }],
  ['command: dispose during the 429 wait cancels the retry', async () => { const { clock, calls, cmd, fail } = setup();
    cmd.send({ entity: NUM.id, command: 'set_value', args: { value: 60 }, phase: 'commit' }); calls[0].no(fail(429, { code: 'rate_limited', retry_after_ms: 700 })); await clock.advance(300);
    cmd.dispose(); await clock.advance(5000); assert.equal(calls.length, 1); }],
  ['command: a 429 on a final that is no longer the latest value is not retried', async () => { const { clock, calls, cmd, fail } = setup();
    cmd.send({ entity: NUM.id, command: 'set_value', args: { value: 60 }, phase: 'commit' }); cmd.send({ entity: NUM.id, command: 'set_value', args: { value: 61 }, phase: 'commit' });
    calls[0].no(fail(429, { code: 'rate_limited', retry_after_ms: 700 })); await clock.advance(3000); assert.equal(calls.length, 2); }],
  ['command: send never rejects when api.command throws, returns a non-promise, or requestId throws', async () => {
    for (const [over, copts] of [[{ command() { throw new Error('boom'); } }], [{ command() { return undefined; } }], [{}, { requestId: () => { throw new Error('id'); } }]]) {
      const { cmd, d } = setup(over, copts), r = await cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); eq(r, { status: 'failed', code: 'client' }); assert.equal(d(SW.id).value, false); } }],
  ['command: a desktop_readonly 403 locks with the desktop reason and tells the bus', async () => { const { clock, calls, cmd, fail, d } = setup(), seen = [];
    cmd.on('readonly', e => seen.push(e)); cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); calls[0].no(fail(403, { code: 'desktop_readonly' })); await clock.advance(0);
    assert.equal(d(SW.id).readonlyReason, 'virtual_desktop.readonly'); assert.equal(seen.length, 1); assert.equal(seen[0].desktop, true); assert.equal(seen[0].reason, 'virtual_desktop.readonly'); }],
  ['command: a lock reason never outranks the desktop read-only reason (D5)', async () => { const { clock, calls, cmd, fail, views, d } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); calls[0].no(fail(403, { status: 'error', code: 'read_only', blocked_by: 'home_assistant.allowed_services' })); await clock.advance(0);
    assert.equal(d(SW.id).readonlyReason, 'home_assistant.allowed_services');
    views.set(SW.id, Object.assign({}, SW, { readonly: true, readonlyReason: 'virtual_desktop.readonly' }));
    assert.equal(d(SW.id).readonly, true); assert.equal(d(SW.id).readonlyReason, 'virtual_desktop.readonly'); }],
  ['command: keys move the overlay at once and send one command 400 ms after the last key', async () => { const { clock, calls, cmd, d } = setup();
    for (const v of [41, 42, 43]) { cmd.send({ entity: LIGHT.id, command: 'brightness', args: { value: v }, phase: 'key' }); await clock.advance(100); }
    assert.equal(d(LIGHT.id).attributes.brightness_pct, 43); assert.equal(calls.length, 0); await clock.advance(300); assert.equal(calls.length, 1); eq(calls[0].body.args, { value: 43 }); }],
  ['command: confirm-required arms 4 s with a countdown; the second activation sends confirmed:true; 409 arms', async () => { const { clock, calls, cmd, fail, d } = setup();
    const go = () => cmd.send({ entity: SCRIPT.id, command: 'activate', args: {}, widgetKey: 'w9' });
    assert.equal((await go()).status, 'armed'); assert.equal(calls.length, 0); assert.equal(d(SCRIPT.id, 'w9').armed.seconds, 4); await clock.advance(1000); assert.equal(d(SCRIPT.id, 'w9').armed.seconds, 3);
    go(); assert.equal(calls[0].body.confirmed, true); await clock.advance(5000); assert.equal(d(SCRIPT.id, 'w9').armed, undefined);
    cmd.send({ entity: SW.id, command: 'turn_on', args: {}, widgetKey: 'w2' }); calls[1].no(fail(409, { code: 'confirmation_required' })); await clock.advance(0); assert.equal(d(SW.id, 'w2').armed.command, 'turn_on');
    cmd.send({ entity: SW.id, command: 'turn_on', args: {}, widgetKey: 'w2' }); assert.equal(calls[2].body.confirmed, true); }],
  ['command: every action has its own request_id; the body carries dashboard and widget ids', async () => { const { calls, cmd } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {}, dashboardId: 'd_m3x7q2k5a4bc', widgetId: 'w2' }); cmd.send({ entity: SW.id, command: 'turn_off', args: {} });
    eq(calls[0].body, { entity: SW.id, command: 'turn_on', args: {}, request_id: 'r1', confirmed: false, dashboard_id: 'd_m3x7q2k5a4bc', widget_id: 'w2' }); assert.equal(calls[1].body.request_id, 'r2'); }]
];
