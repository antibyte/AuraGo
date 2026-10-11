const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { resolveInside } = require('./path_policy');

test('resolveInside stays under the screenshots root and rejects a symlink', async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'ba-shots-'));
  const shots = path.join(root, 'browser_screenshots');
  await fs.mkdir(shots);
  const outside = path.join(root, 'notes.md');
  await fs.writeFile(outside, 'keep');
  await assert.rejects(() => resolveInside(shots, '../notes.md'));
  const link = path.join(shots, 'link.png');
  await fs.symlink(outside, link);
  await assert.rejects(() => resolveInside(shots, 'link.png'), /symlink/);
  const target = await resolveInside(shots, 'ok.png');
  assert.equal(target, path.join(shots, 'ok.png'));
});
