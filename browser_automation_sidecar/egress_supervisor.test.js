const test = require('node:test');
const assert = require('node:assert/strict');
const { noteEgressProxyExit } = require('./egress_supervisor');

const base = { shuttingDown: false, accepted: true, gaveUp: false, restarts: 0, windowStart: 0, now: 1000, limit: 3, windowMs: 60000 };

test('a filter exit before the proxy was accepted does not restart', () => {
  const decision = noteEgressProxyExit({ ...base, accepted: false });
  assert.equal(decision.action, 'stop');
});

test('a filter exit restarts until the limit, then gives up', () => {
  let state = { ...base };
  for (let i = 0; i < 3; i += 1) {
    state = { ...state, ...noteEgressProxyExit(state), accepted: true, shuttingDown: false, gaveUp: false, now: 1000 + i };
    assert.equal(state.action, 'restart');
  }
  const stopped = noteEgressProxyExit({ ...state, now: 1003 });
  assert.equal(stopped.action, 'give-up');
  assert.equal(stopped.gaveUp, true);
});

test('shutdown does not restart the filter', () => {
  const decision = noteEgressProxyExit({ ...base, shuttingDown: true, restarts: 0 });
  assert.equal(decision.action, 'stop');
});
