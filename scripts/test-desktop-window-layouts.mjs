import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const source = await readFile(new URL('../ui/js/desktop/core/window-layouts-runtime.js', import.meta.url), 'utf8');
const context = vm.createContext({});
vm.runInContext(source, context);
const call = (name, ...args) => vm.runInContext(`${name}(...${JSON.stringify(args)})`, context);
const json = value => JSON.parse(JSON.stringify(value));

assert.deepEqual(json(vm.runInContext('DESKTOP_WINDOW_LAYOUT_ZONES', context)), [
    'left-half', 'right-half', 'top-half', 'bottom-half',
    'top-left', 'top-right', 'bottom-left', 'bottom-right', 'maximize'
]);
assert.deepEqual(json(call('desktopWindowLayoutRect', 'left-half', 101, 81)), { left: 0, top: 0, width: 50, height: 81 });
assert.deepEqual(json(call('desktopWindowLayoutRect', 'right-half', 101, 81)), { left: 50, top: 0, width: 51, height: 81 });
assert.deepEqual(json(call('desktopWindowLayoutRect', 'top-half', 101, 81)), { left: 0, top: 0, width: 101, height: 40 });
assert.deepEqual(json(call('desktopWindowLayoutRect', 'bottom-half', 101, 81)), { left: 0, top: 40, width: 101, height: 41 });
assert.deepEqual(json(call('desktopWindowLayoutRect', 'top-left', 101, 81)), { left: 0, top: 0, width: 50, height: 40 });
assert.deepEqual(json(call('desktopWindowLayoutRect', 'top-right', 101, 81)), { left: 50, top: 0, width: 51, height: 40 });
assert.deepEqual(json(call('desktopWindowLayoutRect', 'bottom-left', 101, 81)), { left: 0, top: 40, width: 50, height: 41 });
assert.deepEqual(json(call('desktopWindowLayoutRect', 'bottom-right', 101, 81)), { left: 50, top: 40, width: 51, height: 41 });
assert.deepEqual(json(call('desktopWindowLayoutRect', 'maximize', 101, 81)), { left: 0, top: 0, width: 101, height: 81 });
assert.equal(call('desktopWindowLayoutRect', 'unknown', 101, 81), null);
assert.equal(call('desktopWindowLayoutFits', { width: 50, height: 40 }, { minWidth: 51, minHeight: 40 }), false);
assert.equal(call('desktopWindowLayoutFits', { width: 101, height: 40 }, { minWidth: 51, minHeight: 41 }), false);
assert.equal(call('desktopWindowLayoutFits', { width: 51, height: 41 }, { minWidth: 51, minHeight: 41 }), true);
assert.equal(call('desktopWindowLayoutFits', { width: 640, height: 450 }, { minWidth: 280, minHeight: 420, maxWidth: 380, maxHeight: 640 }), false);
assert.equal(call('desktopWindowLayoutFits', { width: 380, height: 700 }, { minWidth: 280, minHeight: 420, maxWidth: 380, maxHeight: 640 }), false);
assert.equal(call('desktopWindowLayoutFits', { width: 380, height: 640 }, { minWidth: 280, minHeight: 420, maxWidth: 380, maxHeight: 640 }), true);
assert.equal(call('desktopWindowLayoutMaxLimit', '380px', 1280), 380);
assert.equal(call('desktopWindowLayoutMaxLimit', 'none', 1280), 1280);
assert.equal(call('desktopWindowLayoutOccupiesHalf', { snapped: 'right-half' }, 'right'), true);
assert.equal(call('desktopWindowLayoutOccupiesHalf', { snapped: 'top-right' }, 'right'), true);
assert.equal(call('desktopWindowLayoutOccupiesHalf', { maximized: true }, 'left'), true);
assert.equal(call('desktopWindowLayoutOccupiesHalf', { snapped: 'bottom-left' }, 'right'), false);

for (const [zone, key, expected] of [
    ['', 'ArrowLeft', 'left-half'], ['left-half', 'ArrowUp', 'top-left'],
    ['left-half', 'ArrowDown', 'bottom-left'], ['right-half', 'ArrowUp', 'top-right'],
    ['top-right', 'ArrowDown', 'bottom-right'], ['bottom-left', 'ArrowUp', 'top-left'],
    ['', 'ArrowUp', 'maximize'], ['', 'ArrowDown', 'restore']
]) assert.equal(call('desktopWindowKeyboardLayout', zone, key), expected);

const item = vm.runInContext('({})', context);
const original = { left: 17, top: 29, width: 811, height: 577 };
context.item = item;
context.bounds = original;
vm.runInContext('desktopWindowRememberRestoreBounds(item, bounds)', context);
item.snapped = 'left-half';
context.bounds = { left: 0, top: 0, width: 512, height: 768 };
vm.runInContext('desktopWindowRememberRestoreBounds(item, bounds)', context);
item.snapped = 'bottom-left';
context.bounds = { left: 0, top: 384, width: 512, height: 384 };
vm.runInContext('desktopWindowRememberRestoreBounds(item, bounds)', context);
assert.deepEqual(json(item.restoreBounds), original);
assert.deepEqual(json(call('desktopWindowLayoutRect', item.snapped, 1280, 800)), { left: 0, top: 400, width: 640, height: 400 });
assert.deepEqual(json(item.restoreBounds), original);

const order = [];
const live = new Set(['first', 'cancel', 'later']);
const cancelled = await context.desktopCloseWindowBatch(
    ['first', 'gone', 'cancel', 'later'], id => live.has(id),
    async id => { order.push(id); return id === 'cancel' ? 'cancelled' : 'accepted'; }
);
assert.deepEqual(json(cancelled), { status: 'cancelled', closed: ['first'], skipped: ['gone'] });
assert.deepEqual(order, ['first', 'cancel']);
const skipped = await context.desktopCloseWindowBatch(['gone'], () => false, async () => assert.fail('missing window was closed'));
assert.deepEqual(json(skipped), { status: 'skipped', closed: [], skipped: ['gone'] });

console.log('Desktop window layouts: rectangle geometry, min/max bounds, keyboard transitions, restore bounds and guarded batches verified.');
