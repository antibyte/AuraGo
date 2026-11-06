(function () {
    'use strict';

    const SLOTS = Object.freeze(['manual-1', 'manual-2', 'manual-3', 'auto',
        'chapter-1', 'chapter-2', 'chapter-3', 'chapter-4',
        'branch-customs', 'branch-garden', 'branch-hall', 'prefinal']);
    const BUDGET = 4 * 1024 * 1024;
    const error = code => Object.assign(new Error(code), { code });
    const validSlot = slot => { if (!SLOTS.includes(slot)) throw error('slot'); };
    const bytes = value => new TextEncoder().encode(JSON.stringify(value)).byteLength;
    const storageError = value => typeof value?.code === 'string' ? value : error(value?.name === 'QuotaExceededError' ? 'quota' : 'storage');

    async function open() {
        if (!window.indexedDB) throw error('storage');
        const db = await new Promise((resolve, reject) => {
            let refused = false;
            const request = window.indexedDB.open('aurago-goldfish-adventure', 1);
            request.onupgradeneeded = () => {
                if (!request.result.objectStoreNames.contains('slots')) request.result.createObjectStore('slots', { keyPath: 'id' });
            };
            request.onblocked = () => { refused = true; reject(error('storage')); };
            request.onerror = () => reject(storageError(request.error));
            request.onsuccess = () => {
                if (refused) request.result.close();
                else resolve(request.result);
            };
        });
        let closed = false;
        db.onversionchange = () => { closed = true; db.close(); };

        function transaction(mode, run) {
            return new Promise((resolve, reject) => {
                if (closed) { reject(error('storage')); return; }
                let tx, result, failure;
                try { tx = db.transaction('slots', mode); }
                catch (reason) { reject(storageError(reason)); return; }
                tx.oncomplete = () => resolve(result);
                tx.onabort = () => reject(failure || storageError(tx.error));
                tx.onerror = () => { failure ||= storageError(tx.error); };
                const abort = reason => { failure = storageError(reason); tx.abort(); };
                try { run(tx.objectStore('slots'), value => { result = value; }, abort); }
                catch (reason) { abort(reason); }
            });
        }

        function list() {
            return transaction('readonly', (store, done, abort) => {
                const request = store.getAll();
                request.onsuccess = () => {
                    try {
                        done(SLOTS.map(id => {
                            const record = request.result.find(value => value.id === id);
                            if (!record) return { id, revision: 0, empty: true };
                            let valid = true;
                            try { window.GoldfishState.validate(record.current.state); } catch (_) { valid = false; }
                            return { id, revision: record.revision, empty: false, valid,
                                label: typeof record.current?.label === 'string' ? record.current.label.slice(0, 80) : '',
                                updatedAt: record.current?.updatedAt, room: valid ? record.current.state.room : null,
                                ending: valid ? record.current.state.ending : null, hasPrior: !!record.prior };
                        }));
                    } catch (reason) { abort(reason); }
                };
            });
        }

        function read(slot, { prior = false } = {}) {
            validSlot(slot);
            return transaction('readonly', (store, done, abort) => {
                const request = store.get(slot);
                request.onsuccess = () => {
                    try {
                        const record = request.result;
                        if (!record) { done(null); return; }
                        const selected = prior ? record.prior : record.current;
                        if (!selected) { done(null); return; }
                        done({ id: slot, revision: record.revision, state: window.GoldfishState.validate(selected.state),
                            label: selected.label, updatedAt: selected.updatedAt, prior });
                    } catch (reason) { abort(reason); }
                };
            });
        }

        function write(slot, state, { expectedRevision, label = '' } = {}) {
            validSlot(slot);
            const clean = window.GoldfishState.validate(state);
            if (!Number.isSafeInteger(expectedRevision) || expectedRevision < 0 || expectedRevision >= Number.MAX_SAFE_INTEGER ||
                typeof label !== 'string' || label.length > 80) throw error('invalid');
            return transaction('readwrite', (store, done, abort) => {
                const request = store.getAll();
                request.onsuccess = () => {
                    try {
                        const records = request.result;
                        const previous = records.find(value => value.id === slot);
                        if ((previous?.revision || 0) !== expectedRevision) throw error('conflict');
                        if (previous) window.GoldfishState.validate(previous.current.state);
                        const record = { id: slot, revision: expectedRevision + 1,
                            current: { state: clean, label, updatedAt: Date.now() },
                            prior: previous?.current || null };
                        if (bytes(records.filter(value => value.id !== slot).concat(record)) > BUDGET) throw error('quota');
                        store.put(record);
                        done({ id: slot, revision: record.revision, updatedAt: record.current.updatedAt });
                    } catch (reason) { abort(reason); }
                };
            });
        }

        function remove(slot, { expectedRevision } = {}) {
            validSlot(slot);
            return transaction('readwrite', (store, done, abort) => {
                const request = store.get(slot);
                request.onsuccess = () => {
                    try {
                        if ((request.result?.revision || 0) !== expectedRevision) throw error('conflict');
                        store.delete(slot);
                        done(true);
                    } catch (reason) { abort(reason); }
                };
            });
        }

        return Object.freeze({ list, read, write, remove, close: () => { closed = true; db.close(); } });
    }

    window.GoldfishSave = Object.freeze({ open, SLOTS, BUDGET });
}());
