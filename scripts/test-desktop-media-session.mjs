import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const source = await readFile(new URL('../ui/js/desktop/core/media-session-runtime.js', import.meta.url), 'utf8');
const nativeHandlers = new Map();
const native = {
    metadata: null,
    playbackState: 'none',
    setActionHandler(action, handler) { nativeHandlers.set(action, handler); }
};
class MediaMetadataMock {
    constructor(metadata) { Object.assign(this, metadata); }
}
const context = vm.createContext({ window: {}, navigator: { mediaSession: native }, MediaMetadata: MediaMetadataMock });
vm.runInContext(source, context);
const media = context.window.AuraDesktopMediaSession;
const changes = [];
const unsubscribe = media.subscribe(snapshot => changes.push(snapshot));
assert.equal(changes[0].available, false);
assert.equal(changes[0].status, 'unavailable');

const calls = [];
const radio = {};
const tv = {};
const personal = {};
const metadata = { title: 'Radio', artist: 'Station', artwork: [{ src: '/cover.png', sizes: '96x96', extra: 'ignored' }] };
media.claim(radio, { priority: 10, activate: true, status: 'playing', playbackState: 'playing', metadata, handlers: {
    play: async () => { calls.push('radio'); }, pause: () => calls.push('radio-pause')
} });
assert.equal(media.snapshot().metadata.title, 'Radio');
assert.deepEqual(Array.from(media.snapshot().actions), ['play', 'pause']);

const copy = media.snapshot();
copy.metadata.title = 'changed';
copy.metadata.artwork[0].src = '/changed.png';
metadata.title = 'mutated source';
metadata.artwork[0].src = '/mutated-source.png';
assert.equal(media.snapshot().metadata.title, 'Radio');
assert.equal(media.snapshot().metadata.artwork[0].src, '/cover.png');
assert.equal('owner' in media.snapshot(), false);
assert.equal(media.snapshot().actions.some(value => typeof value === 'function'), false);

const beforeInactiveUpdate = changes.length;
media.claim(tv, { priority: 10, activate: true, status: 'buffering', playbackState: 'playing', metadata: { title: 'TV' }, handlers: {
    play: async () => { calls.push('tv'); }, nexttrack: async () => { throw new Error('dispatch failure'); }
} });
assert.equal(media.snapshot().metadata.title, 'TV');
const afterSelection = changes.length;
media.claim(radio, { priority: 10, status: 'paused', playbackState: 'paused', metadata: { title: 'Updated radio' }, handlers: { play: () => calls.push('radio-new') } });
assert.equal(media.snapshot().metadata.title, 'TV');
assert.equal(changes.length, afterSelection, 'inactive-owner updates do not notify observers');
assert.ok(afterSelection > beforeInactiveUpdate);

assert.equal(await media.dispatch('play'), true);
assert.deepEqual(calls, ['tv']);
assert.equal(await media.dispatch('stop'), false);
await assert.rejects(media.dispatch('nexttrack'), /dispatch failure/);
nativeHandlers.get('play')();
await Promise.resolve();
await Promise.resolve();
assert.deepEqual(calls, ['tv', 'tv']);

media.claim(personal, { priority: 100, activate: true, status: 'paused', playbackState: 'paused', metadata: { title: 'Personal' }, handlers: { pause: () => calls.push('personal') } });
assert.equal(media.snapshot().metadata.title, 'Personal');
assert.equal(await media.dispatch('pause'), true);
media.release(personal);
assert.equal(media.snapshot().metadata.title, 'TV');
media.release(tv);
assert.equal(media.snapshot().metadata.title, 'Updated radio');
media.release(radio);
assert.equal(nativeHandlers.get('play'), null);
assert.equal(native.metadata, null);
assert.equal(media.snapshot().available, false);
const countBeforeUnsubscribe = changes.length;
unsubscribe();
media.claim({}, { priority: 1, metadata: { title: 'ignored observer' } });
assert.equal(changes.length, countBeforeUnsubscribe);
const inheritedHandlers = Object.create({ play: () => calls.push('inherited') });
media.claim({}, { priority: 2, handlers: inheritedHandlers });
assert.equal(await media.dispatch('play'), false, 'inherited handlers are never dispatched');
const unlistedHandlers = { constructor: () => calls.push('constructor') };
media.claim({}, { priority: 3, handlers: unlistedHandlers });
assert.equal(Array.from(media.snapshot().actions).includes('constructor'), false);
assert.equal(await media.dispatch('constructor'), false, 'unlisted actions are never dispatched');

const noNativeContext = vm.createContext({ window: {}, navigator: {} });
vm.runInContext(source, noNativeContext);
noNativeContext.window.AuraDesktopMediaSession.claim({}, { metadata: { title: 'Snapshot works without native support' } });
assert.equal(noNativeContext.window.AuraDesktopMediaSession.snapshot().metadata.title, 'Snapshot works without native support');

process.stdout.write('Desktop media session: priority, snapshots, subscriptions, dispatch and native fallback verified.\n');
