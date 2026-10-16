import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const window = {};
vm.runInNewContext(fs.readFileSync('ui/js/desktop/apps/bookshelf/progress.js', 'utf8'), { window });

let releaseFirst;
let signalFirstStarted;
const firstStarted = new Promise(resolve => { signalFirstStarted = resolve; });
const firstGate = new Promise(resolve => { releaseFirst = resolve; });
const writes = [];
const writer = window.BookshelfProgress.createWriter(async (item, expectedRevision) => {
  writes.push({ bookId: item.bookId, renditionRevision: item.renditionRevision, expectedRevision });
  if (writes.length === 1) {
    signalFirstStarted();
    await firstGate;
  }
  return { progress: { revision: expectedRevision + 1 } };
});

const item = fraction => ({
  bookId: 'book-one',
  renditionRevision: 'rendition-a',
  fallbackRevision: 7,
  progress: { rendition_revision: 'rendition-a', locator: { cfi: `chapter-${fraction}` }, fraction }
});

const first = writer.enqueue(item(0.25));
await firstStarted;
const second = writer.enqueue(item(0.5));
releaseFirst();
await Promise.all([first, second]);
assert.deepEqual(writes.map(write => write.expectedRevision), [7, 8], 'a queued later write must use the revision returned by the earlier write');
assert.equal(writer.revisionFor({ bookId: 'book-one', renditionRevision: 'rendition-a' }, 0), 9);
assert.equal(writer.revisionFor({ bookId: 'book-two', renditionRevision: 'rendition-a' }, 4), 4, 'revisions must stay book-scoped');
assert.equal(writer.revisionFor({ bookId: 'book-one', renditionRevision: 'rendition-b' }, 4), 4, 'revisions must stay rendition-scoped');

let failOnce = true;
const retryRevisions = [];
const retryWriter = window.BookshelfProgress.createWriter(async (_item, expectedRevision) => {
  retryRevisions.push(expectedRevision);
  if (failOnce) {
    failOnce = false;
    throw Object.assign(new Error('conflict'), { status: 409 });
  }
  return { progress: { revision: expectedRevision + 1 } };
});
await assert.rejects(retryWriter.enqueue(item(0.75)), error => error.status === 409);
retryWriter.remember('book-one', 'rendition-a', 20); // a user explicitly accepted the freshly saved position
await retryWriter.enqueue(item(0.8));
assert.deepEqual(retryRevisions, [7, 20], 'an explicit conflict choice must rebase a retry; a failed write alone must not');

console.log('Bookshelf progress revision queue checks passed.');
