import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const window = {};
const root = new URL('../', import.meta.url);
vm.runInNewContext(fs.readFileSync(new URL('ui/js/desktop/apps/bookshelf/api.js', root), 'utf8'), { window, URLSearchParams });
vm.runInNewContext(fs.readFileSync(new URL('ui/js/desktop/apps/bookshelf/progress.js', root), 'utf8'), { window });

const writes = [];
let savedRevision = 0;
const api = window.BookshelfAPI.create({ api: async (path, options) => {
  const request = new Request('http://bookshelf.test' + path, options);
  const ifMatch = request.headers.get('If-Match');
  writes.push({ ifMatch, body: await request.json() });
  if (ifMatch === null || Number(ifMatch) !== savedRevision) {
    const body = { error: 'revision_conflict' };
    throw Object.assign(new Error(body.error), { status: 409, body });
  }
  return { progress: { revision: ++savedRevision } };
} });
const writer = window.BookshelfProgress.createWriter((item, expectedRevision) =>
  api.saveProgress(item.bookId, item.progress, expectedRevision));

const item = fraction => ({
  bookId: 'book-one',
  renditionRevision: 'rendition-a',
  fallbackRevision: 0,
  progress: { rendition_revision: 'rendition-a', locator: { cfi: `chapter-${fraction}` }, fraction }
});

const first = writer.enqueue(item(0.25));
const second = writer.enqueue(item(0.5));
await Promise.all([first, second]);
assert.deepEqual(writes.map(write => write.ifMatch), ['0', '1'], 'first-open progress must send If-Match: 0, then successive saves use the acknowledged revision');
assert.equal(writer.revisionFor({ bookId: 'book-one', renditionRevision: 'rendition-a' }, 0), 2);
assert.equal(writer.revisionFor({ bookId: 'book-two', renditionRevision: 'rendition-a' }, 4), 4, 'revisions must stay book-scoped');
assert.equal(writer.revisionFor({ bookId: 'book-one', renditionRevision: 'rendition-b' }, 4), 4, 'revisions must stay rendition-scoped');

savedRevision = 20; // another reader advanced the persisted progress
await assert.rejects(writer.enqueue(item(0.75)), error =>
  error.status === 409 && error.body?.error === 'revision_conflict', 'stale If-Match uses the server error shape');
assert.equal(writer.revisionFor({ bookId: 'book-one', renditionRevision: 'rendition-a' }, 0), 2, 'a stale-window conflict must not silently rebase');
writer.remember('book-one', 'rendition-a', savedRevision); // explicit user choice in the conflict dialog
await writer.enqueue(item(0.8));
assert.deepEqual(writes.map(write => write.ifMatch), ['0', '1', '2', '20'], 'only an explicit conflict choice can rebase the retry');

console.log('Bookshelf first-open, successive-save and stale-window progress checks passed.');
