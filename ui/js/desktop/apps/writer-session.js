(function () {
    'use strict';
    // One queue per open document. Acknowledgements only cover the serialized revision.
    function create(options) {
        let revision = 0, savedRevision = 0, timer = null, deadline = null;
        let pending = null, disposed = false, suspended = false, error = null, retryBlocked = false;
        const emit = () => options.onState?.({ revision, dirty: revision !== savedRevision, saving: !!pending, error });
        const clear = () => { clearTimeout(timer); clearTimeout(deadline); timer = deadline = null; };
        function schedule() {
            if (disposed || suspended || options.readonly || revision === savedRevision || retryBlocked) return;
            clearTimeout(timer);
            timer = setTimeout(() => save().catch(() => {}), 800);
            if (!deadline) deadline = setTimeout(() => save().catch(() => {}), 5000);
        }
        async function save() {
            if (disposed) throw new Error('Document is closed');
            if (pending) { await pending; if (revision !== savedRevision) return save(); return; }
            if (revision === savedRevision || options.readonly) return;
            clear();
            error = null; retryBlocked = false;
            const captured = revision;
            pending = (async () => {
                const bytes = await options.serialize();
                if (disposed) return;
                await options.backup?.(bytes, captured);
                if (disposed) return;
                await options.write(bytes);
                if (disposed) return;
                savedRevision = captured;
                if (revision === captured) await options.clearBackup?.();
            })();
            emit();
            try { await pending; }
            catch (cause) { error = cause; retryBlocked = true; throw cause; }
            finally { pending = null; if (!disposed) { emit(); schedule(); } }
        }
        return {
            changed() { if (disposed || options.readonly) return; revision++; error = null; retryBlocked = false; emit(); schedule(); },
            save,
            suspend() { suspended = true; clear(); },
            resume() { suspended = false; schedule(); },
            get revision() { return revision; },
            get dirty() { return revision !== savedRevision; },
            get pending() { return pending; },
            get error() { return error; },
            dispose() { disposed = true; clear(); },
        };
    }
    const databases = new Map();
    function openDrafts(namespace = 'writer') {
        if (!['writer','sheets'].includes(namespace)) throw new Error('Invalid office draft namespace');
        if (!databases.has(namespace)) databases.set(namespace, new Promise((resolve, reject) => {
            const request = indexedDB.open('aurago.' + namespace + '.drafts.v1', 1);
            request.onupgradeneeded = () => request.result.createObjectStore('documents', { keyPath: 'key' });
            request.onsuccess = () => resolve(request.result);
            request.onerror = () => { databases.delete(namespace); reject(request.error); };
        }));
        return databases.get(namespace);
    }
    async function draft(operation, key, value, namespace = 'writer') {
        const db = await openDrafts(namespace);
        return new Promise((resolve, reject) => {
            const tx = db.transaction('documents', operation === 'get' ? 'readonly' : 'readwrite');
            const store = tx.objectStore('documents');
            const request = operation === 'put' ? store.put({ ...value, key }) : store[operation](key);
            tx.oncomplete = () => resolve(request.result);
            tx.onerror = () => reject(tx.error);
            tx.onabort = () => reject(tx.error || new Error('Draft transaction aborted'));
        });
    }
    window.OfficeSession = { create, draft };
    window.WriterSession = window.OfficeSession;
})();
