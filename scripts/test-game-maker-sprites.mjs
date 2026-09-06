import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../internal/gamemaker/build.go', import.meta.url), 'utf8');
const guard = source.match(/const phaserSpriteGuard = `([\s\S]*?)`/)[1];
const calls = [];
class Loader {
  addFile(input) { calls.push({ loader: this, input }); return this; }
}
const context = { Phaser: { Loader: { LoaderPlugin: Loader } } };
vm.runInNewContext(guard, context);
const wrapped = Loader.prototype.addFile;
vm.runInNewContext(guard, context);
assert.equal(Loader.prototype.addFile, wrapped, 'guard must install only once');
const loader = new Loader();
const url = 'assets/builtin/space-shooter/1/sheet.png';
const good = { type: 'spritesheet', url, config: { frameWidth: 64, frameHeight: 64 } };
assert.equal(loader.addFile(good), loader);
assert.equal(calls[0].loader, loader);
assert.equal(calls[0].input, good);
loader.addFile([good, { type: 'image', url: 'assets/custom.png' }]);
loader.addFile({ type: 'json', url: 'assets/builtin/space-shooter/1/sheet.json' });
loader.addFile({ ...good, config: { frameWidth: 64 } });
for (const file of [
  { type: 'image', url },
  { type: 'image', url: '/api/game-maker/preview/token/' + url + '?v=1' },
  { type: 'image', url: url.replace('space-shooter', 'space%2Dshooter') },
  { ...good, config: { frameWidth: 640, frameHeight: 640 } },
  { ...good, config: { frameWidth: 64, frameHeight: 32 } },
  { ...good, config: { frameWidth: 64, spacing: 1 } },
  { ...good, config: { frameWidth: 64, startFrame: 10 } },
]) {
  const before = calls.length;
  assert.throws(() => loader.addFile(file), /load\.spritesheet.*frameWidth:64/);
  assert.throws(() => loader.addFile([good, file]), /load\.spritesheet/);
  assert.equal(calls.length, before, 'invalid batches must not reach the loader');
}
vm.runInNewContext(guard, {}); // A Three.js game has no Phaser global.
console.log('PASS: built-in sprite loader contract, arrays, URLs and idempotence');
