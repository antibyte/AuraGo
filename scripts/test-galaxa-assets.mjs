import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const root = new URL('../ui/', import.meta.url);
const constants = await readFile(new URL('js/desktop/apps/galaxa-constants.js', root), 'utf8');
const sprites = await readFile(new URL('js/desktop/apps/galaxa-sprites.js', root), 'utf8');
const atlas = JSON.parse(await readFile(new URL('img/galaxa/atlas.json', root), 'utf8'));

for (const buildKey of ['BUILD_VERSION', 'AURAGO_BUILD_VERSION']) {
    const version = 'a'.repeat(64), requests = [];
    let failOnce = true;
    function request(path) {
        const url = new URL(path, 'https://aurago.test');
        requests.push(url.pathname);
        assert.equal(url.searchParams.get('v'), version, 'server would reject an obsolete asset version');
        assert.equal(url.origin, 'https://aurago.test');
        return url.pathname;
    }
    const context = vm.createContext({
        window: { [buildKey]: version },
        fetch: async path => {
            assert.equal(request(path), '/img/galaxa/atlas.json');
            if (failOnce) { failOnce = false; return { ok: false, status: 503 }; }
            return { ok: true, json: async () => atlas };
        },
        Image: class {
            set src(path) {
                const name = request(path).split('/').pop();
                const sheet = Object.values(atlas.sheets).find(sheet => sheet.file === name);
                assert.ok(sheet, 'unexpected atlas image');
                this.naturalWidth = sheet.width;
                this.naturalHeight = sheet.height;
                this.onload();
            }
        }
    });
    vm.runInContext(constants, context);
    vm.runInContext(sprites, context);
    const load = context.window.GalaxaCore.loadAtlasAssets;
    await assert.rejects(load(), /HTTP 503/);
    const pending = load();
    assert.equal(load(), pending, 'concurrent loads must share one request');
    const assets = await pending;
    assert.equal(Object.keys(assets.images).length, Object.keys(atlas.sheets).length);
    assert.equal(requests.length, 2 + Object.keys(atlas.sheets).length);
    assert.equal(await load(), assets, 'successful assets must remain cached');
}
console.log('Galaxa asset version, complete atlas loading, retry and shared load checks passed.');
