import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const source = await readFile(new URL('../ui/js/desktop/core/file-history-runtime.js', import.meta.url), 'utf8');

function createHistory({ api, refresh = async () => {}, notify = () => {}, isReadonly = () => false, isSessionActive, t } = {}) {
  const window = {};
  const context = { window, console, Date, Set, Promise, JSON, crypto: globalThis.crypto };
  vm.runInNewContext(source, context, { filename: 'file-history-runtime.js' });
  const history = window.AuraDesktopFileHistory;
  history.configure({ api: api || (async () => ({ status: 'ok', undo_receipt: 'redo-default' })), refresh, notify, isReadonly, isSessionActive, t });
  return history;
}

function move(token) {
  return { from: 'Desktop/' + token, path: 'Trash/' + token, undo_receipt: token };
}

{
  const calls = [];
  let next = 0;
  const refreshed = [];
  const history = createHistory({
    api: async (url, options) => {
      calls.push({ url, options, receipt: JSON.parse(options.body).receipt });
      return { status: 'ok', from: 'Trash/file' + next, path: 'Desktop/file' + next, undo_receipt: 'redo-' + next++ };
    },
    refresh: async () => refreshed.push(true)
  });
  const snapshots = [];
  const unsubscribe = history.subscribe(snapshot => snapshots.push(snapshot));
  assert.equal(history.record({ moves: [move('one'), move('two'), move('three')] }, 'Move'), true);
  assert.equal(history.record({ moves: [move('one')] }, 'duplicate'), false, 'duplicate producer receipts are ignored');
  assert.equal(history.snapshot().undoCount, 1, 'batch receipts form one action');
  assert.ok(history.snapshot().undoId, 'top action exposes a toast-safe opaque id');
  assert.equal((await history.undo()).status, 'completed');
  assert.deepEqual(calls.map(call => call.receipt), ['three', 'two', 'one'], 'undo runs in reverse order');
  assert.equal(history.snapshot().canRedo, true);
  assert.equal((await history.redo()).status, 'completed');
  assert.deepEqual(calls.slice(3).map(call => call.receipt), ['redo-2', 'redo-1', 'redo-0'], 'redo restores original order');
  assert.equal(history.snapshot().canUndo, true);
  assert.equal(refreshed.length, 2, 'refresh runs once per completed batch');
  assert.ok(snapshots.every(item => typeof item.revision === 'number'));
  unsubscribe();
}

{
  let fail = true;
  const calls = [];
  const history = createHistory({ api: async (_url, options) => {
    const receipt = JSON.parse(options.body).receipt;
    calls.push(receipt);
    if (receipt === 'second' && fail) {
      const error = new Error('destination exists');
      error.status = 409;
      error.body = { code: 'file_conflict' };
      throw error;
    }
    return { status: 'ok', from: 'Trash/' + receipt, path: 'Desktop/' + receipt, undo_receipt: 'redo-' + receipt };
  } });
  history.record([move('first'), move('second'), move('third')], 'Move');
  assert.equal((await history.undo()).status, 'partial');
  assert.deepEqual(calls, ['third', 'second']);
  assert.equal(history.snapshot().canUndo, true);
  assert.equal(history.snapshot().canRedo, true, 'completed inverse steps remain redoable');
  fail = false;
  await history.undo();
  assert.deepEqual(calls.slice(2), ['second', 'first'], 'retryable conflict retains the failed receipt and remaining work');
  assert.equal(history.snapshot().canUndo, false);
  assert.equal(history.snapshot().canRedo, true);
}

{
  let failSecondRedo = true;
  const calls = [];
  const history = createHistory({ api: async (_url, options) => {
    const receipt = JSON.parse(options.body).receipt;
    calls.push(receipt);
    if (receipt.startsWith('original-')) {
      const name = receipt.slice('original-'.length);
      return { status: 'ok', from: 'Trash/' + name, path: 'Desktop/' + name, undo_receipt: 'redo-' + name };
    }
    if (receipt === 'redo-second' && failSecondRedo) {
      const error = new Error('destination exists');
      error.status = 409;
      error.body = { code: 'file_conflict' };
      throw error;
    }
    const name = receipt.slice('redo-'.length);
    return { status: 'ok', from: 'Desktop/' + name, path: 'Trash/' + name, undo_receipt: 'undo-' + name };
  } });
  history.record(['first', 'second', 'third'].map(name => ({ ...move(name), undo_receipt: 'original-' + name })), 'Move');
  await history.undo();
  assert.deepEqual(calls, ['original-third', 'original-second', 'original-first']);
  calls.length = 0;
  assert.equal((await history.redo()).status, 'partial');
  assert.deepEqual(calls, ['redo-first', 'redo-second'], 'redo applies remaining members in original forward order');
  assert.equal(history.snapshot().canUndo, true, 'confirmed redo steps retain their inverse receipt');
  assert.equal(history.snapshot().canRedo, true, 'failed redo receipt remains available');
  failSecondRedo = false;
  calls.length = 0;
  assert.equal((await history.redo()).status, 'completed');
  assert.deepEqual(calls, ['redo-second', 'redo-third']);
  assert.equal(history.snapshot().canRedo, false);
  assert.equal(history.snapshot().canUndo, true);
}

{
  const calls = [];
  const notices = [];
  const history = createHistory({
    api: async (_url, options) => {
      const receipt = JSON.parse(options.body).receipt;
      calls.push(receipt);
      throw new TypeError('connection lost');
    },
    notify: notice => notices.push(notice)
  });
  history.record(move('uncertain'), 'Move');
  assert.equal((await history.undo()).status, 'failed');
  assert.equal(history.snapshot().canUndo, false, 'unknown outcomes are never replayed');
  assert.equal((await history.undo()).status, 'empty');
  assert.deepEqual(calls, ['uncertain']);
  assert.match(notices[0].message, /will not retry/i);
}

{
  const calls = [];
  const history = createHistory({ api: async (_url, options) => {
    calls.push(JSON.parse(options.body).receipt);
    const error = new Error('receipt unavailable');
    error.status = 409;
    error.body = { code: 'file_undo_unavailable' };
    throw error;
  } });
  history.record(move('expired'), 'Move');
  assert.equal((await history.undo()).status, 'failed');
  assert.equal(history.snapshot().canUndo, false, 'unavailable receipt is discarded even when reported as HTTP 409');
  assert.deepEqual(calls, ['expired']);
}

{
  const calls = [];
  let resolveRequest;
  const history = createHistory({ api: (_url, options) => {
    const receipt = JSON.parse(options.body).receipt;
    calls.push(receipt);
    if (receipt === 'old') return new Promise(resolve => { resolveRequest = resolve; });
    return Promise.resolve({ status: 'ok', from: 'Trash/new', path: 'Desktop/new', undo_receipt: 'redo-new' });
  } });
  history.record(move('old'), 'Older move');
  const pendingUndo = history.undo();
  await Promise.resolve();
  assert.equal(history.record(move('new'), 'Newer move'), true, 'new receipts queue while an undo batch is active');
  resolveRequest({ status: 'ok', from: 'Trash/old', path: 'Desktop/old', undo_receipt: 'redo-old' });
  await pendingUndo;
  assert.equal(history.snapshot().undoLabel, 'Newer move', 'queued operation stays above the older in-flight action');
  assert.equal(history.snapshot().canRedo, false, 'new action invalidates the older inverse receipt');
  await history.undo();
  assert.deepEqual(calls, ['old', 'new']);
}

{
  const calls = [];
  const history = createHistory({ api: async (_url, options) => {
    calls.push(JSON.parse(options.body).receipt);
    return { status: 'ok' }; // Malformed success cannot safely be replayed.
  } });
  history.record(move('malformed'), 'Move');
  assert.equal((await history.undo()).status, 'failed');
  assert.equal(history.snapshot().canUndo, false);
  assert.equal((await history.undo()).status, 'empty');
  assert.deepEqual(calls, ['malformed']);
}

{
  let resolveRequest;
  const calls = [];
  const history = createHistory({ api: (_url, options) => {
    calls.push(JSON.parse(options.body).receipt);
    return new Promise(resolve => { resolveRequest = resolve; });
  } });
  history.record(move('in-flight'), 'Move');
  const pending = history.undo();
  await Promise.resolve();
  history.clear();
  resolveRequest({ status: 'ok', from: 'Trash/in-flight', path: 'Desktop/in-flight', undo_receipt: 'redo-in-flight' });
  assert.equal((await pending).status, 'cleared');
  assert.equal(history.snapshot().undoCount, 0, 'clear invalidates in-flight results instead of resurrecting old history');
  assert.equal(history.snapshot().redoCount, 0);
  assert.deepEqual(calls, ['in-flight']);
}

{
  let active = true;
  let resolveRequest;
  const calls = [];
  let refreshes = 0;
  let notices = 0;
  const history = createHistory({
    isSessionActive: () => active,
    api: (_url, options) => {
      calls.push(JSON.parse(options.body).receipt);
      return new Promise(resolve => { resolveRequest = resolve; });
    },
    refresh: async () => { refreshes++; },
    notify: () => { notices++; }
  });
  history.record(move('session-old'), 'Old session');
  const pending = history.undo();
  await Promise.resolve();
  assert.deepEqual(calls, ['session-old']);
  assert.equal(history.record(move('queued-before-logout'), 'Queued action'), true);

  active = false;
  history.clear();
  assert.equal(history.snapshot().canUndo, false);
  assert.equal(history.snapshot().undoCount, 0, 'inactive snapshots expose no old-session history');
  assert.equal(history.record(move('late-producer'), 'Late producer'), false, 'late producer result after logout is ignored');
  resolveRequest({ status: 'ok', from: 'Trash/session-old', path: 'Desktop/session-old', undo_receipt: 'redo-session-old' });
  assert.equal((await pending).status, 'cleared');
  assert.equal(history.snapshot().undoCount, 0, 'in-flight and queued records cannot resurrect after logout');
  assert.equal(history.snapshot().redoCount, 0);
  assert.equal((await history.undo()).status, 'inactive');
  assert.deepEqual(calls, ['session-old'], 'inactive history sends no further requests');
  assert.equal(refreshes, 0, 'obsolete in-flight result does not refresh the new session');
  assert.equal(notices, 0, 'obsolete in-flight result does not notify the new session');
}

{
  let readonly = false;
  const history = createHistory({ isReadonly: () => readonly, api: async () => ({ undo_receipt: 'redo' }) });
  history.record(move('readonly'), 'Move');
  readonly = true;
  assert.equal(history.snapshot().canUndo, false);
  assert.equal((await history.undo()).status, 'readonly');
  assert.equal(history.snapshot().undoCount, 1, 'readonly retains safe receipt for later');
}

{
  const history = createHistory({ api: async () => ({ undo_receipt: 'redo' }) });
  for (let index = 0; index < 23; index++) history.record(move('item-' + index), 'Move ' + index);
  assert.equal(history.snapshot().undoCount, 20, 'history is bounded by action count');
  history.clear();
  assert.equal(history.snapshot().undoCount, 0);
  assert.equal(history.snapshot().redoCount, 0);
}

console.log('Desktop file history runtime checks passed.');
