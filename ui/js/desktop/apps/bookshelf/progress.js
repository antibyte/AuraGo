(function () {
    'use strict';

    function createWriter(send) {
        const revisions = new Map();
        let queue = Promise.resolve();

        function revisionFor(item, fallback) {
            const saved = revisions.get(item.bookId);
            return saved && saved.renditionRevision === item.renditionRevision
                ? saved.revision
                : (fallback ?? item.fallbackRevision ?? item.expectedRevision ?? 0);
        }

        function remember(bookId, renditionRevision, revision) {
            const value = Number(revision);
            if (Number.isFinite(value)) revisions.set(bookId, { renditionRevision, revision: value });
        }

        async function writeNow(item, expectedOverride) {
            const expectedRevision = expectedOverride === undefined
                ? revisionFor(item)
                : expectedOverride;
            const result = await send(item, expectedRevision);
            const progress = result?.progress || result || {};
            if (progress.revision !== undefined && progress.revision !== null) {
                remember(item.bookId, item.renditionRevision, progress.revision);
            }
            return result;
        }

        function enqueue(item) {
            const pending = queue.then(() => writeNow(item));
            queue = pending.catch(() => undefined);
            return pending;
        }

        return { enqueue, writeNow, remember, revisionFor };
    }

    window.BookshelfProgress = { createWriter };
})();
