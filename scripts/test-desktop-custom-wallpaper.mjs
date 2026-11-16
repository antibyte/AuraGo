import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../ui/js/desktop/core/custom-wallpaper-runtime.js', import.meta.url), 'utf8');
const requests = [];
const revoked = [];
const writes = [];
const pageListeners = new Map();
let objectURLSequence = 0;
let releaseOldResponse;

class FakeURL {
    static createObjectURL(blob) { return `blob:test/${++objectURLSequence}/${blob.type}`; }
    static revokeObjectURL(value) { revoked.push(value); }
}

class FakeImage {
    constructor() {
        this.naturalWidth = 640;
        this.naturalHeight = 480;
    }
    set src(value) { this._src = value; }
    get src() { return this._src; }
    decode() { return Promise.resolve(); }
}

const target = {
    dataset: {},
    style: {
        properties: {},
        setProperty(name, value) { this.properties[name] = value; },
        removeProperty(name) { delete this.properties[name]; }
    }
};

const window = {
    addEventListener(name, handler) { pageListeners.set(name, handler); }
};

const fetch = (url, options) => {
    requests.push({ url, options });
    const path = new URLSearchParams(String(url).split('?')[1] || '').get('path');
    if (path === 'Pictures/old.png') return new Promise(resolve => { releaseOldResponse = resolve; });
    if (path === 'Pictures/missing.png') {
        return Promise.resolve({ ok: false, status: 404, json: async () => ({ code: 'wallpaper_not_found' }) });
    }
    return Promise.resolve({
        ok: true,
        status: 200,
        blob: async () => ({ type: 'image/png', size: 12 })
    });
};

const context = {
    AbortController,
    FakeImage,
    Image: FakeImage,
    URL: FakeURL,
    URLSearchParams,
    fetch,
    saveDesktopPreference: async (...args) => writes.push(args),
    window
};
vm.runInNewContext(source, context, { filename: 'custom-wallpaper-runtime.js' });
const api = window.AuraDesktopCustomWallpaper;
assert.equal(api.isCustom('file:Pictures/wall.png'), true);
assert.equal(api.path('file:Pictures/wall.png'), 'Pictures/wall.png');
assert.equal(api.isCustom('groupshoot'), false);
assert.equal(api.path('groupshoot'), '');
assert.equal(api.isCustom('file:'), true, 'custom wallpaper prefix remains recognizable even when its path is invalid');

const reports = [];
const oldLoad = api.apply(target, 'file:Pictures/old.png', 'cover', { onStatus: (...args) => reports.push(args) });
assert.equal(requests[0].options.credentials, 'same-origin');
assert.equal(requests[0].options.cache, 'no-store');
assert.ok(requests[0].options.signal instanceof AbortSignal);
assert.equal(target.dataset.customWallpaperState, 'loading');

assert.equal(await api.apply(target, 'file:Pictures/current.png', 'contain', { onStatus: (...args) => reports.push(args) }), true);
assert.equal(target.dataset.customWallpaperState, 'ready');
assert.equal(target.style.properties['--vd-custom-wallpaper-fit'], 'contain');
const currentURL = target.style.properties['--vd-custom-wallpaper-image'];
assert.match(currentURL, /blob:test\/\d+\/image\/png/);
assert.equal(writes.length, 0, 'wallpaper reads must not persist settings, including in a read-only desktop');

releaseOldResponse({ ok: true, status: 200, blob: async () => ({ type: 'image/png', size: 12 }) });
assert.equal(await oldLoad, false, 'late old response must not replace the current wallpaper');
assert.equal(target.style.properties['--vd-custom-wallpaper-image'], currentURL);

assert.equal(await api.apply(target, 'file:Pictures/missing.png', 'cover', { onStatus: (...args) => reports.push(args) }), false);
assert.equal(target.dataset.customWallpaperState, 'missing');
assert.equal(target.style.properties['--vd-custom-wallpaper-image'], undefined);
assert.ok(revoked.some(value => currentURL.includes(value)), 'failed replacement must revoke the prior object URL');
assert.ok(reports.some(([, status, code]) => status === 'missing' && code === 'wallpaper_not_found'));

await api.apply(target, 'file:Pictures/fit.png', 'cover');
assert.equal(target.style.properties['--vd-custom-wallpaper-fit'], 'cover');
api.release(target);
assert.equal(target.dataset.customWallpaperState, undefined);
assert.equal(target.style.properties['--vd-custom-wallpaper-image'], undefined);
assert.equal(target.style.properties['--vd-custom-wallpaper-fit'], undefined);
assert.equal(pageListeners.has('pagehide'), true);

console.log('custom wallpaper runtime checks passed');
