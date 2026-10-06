// EasyDrag autosave: saves the draft 1 s after the last change, keeps an emergency copy in
// localStorage, serialises requests and resolves revision conflicts with the user.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const DELAY_MS = 1000;
    const RETRY_MS = 5000;
    const RETRY_MAX_MS = 60000;
    const EMERGENCY_PREFIX = 'aurago.easydrag.draft.';
    const EMERGENCY_MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000;
    // Answers a retry cannot fix; every other coded error counts as transient.
    const PERMANENT_CODES = new Set(['FLOW_TOO_LARGE', 'FLOW_NOT_FOUND', 'FLOW_PERMISSION_DENIED', 'FLOW_LOCKED', 'FLOW_BAD_REQUEST']);

    function emergencyKey(flowId) { return EMERGENCY_PREFIX + flowId; }

    // permanent reports whether a retry cannot fix err: a PERMANENT_CODES answer, or an answer
    // without a code whose HTTP status is a 4xx other than 408 and 429.
    function permanent(err) {
        const code = ED.core.errorCode(err);
        if (code) return PERMANENT_CODES.has(code);
        const status = Number(err && err.status) || 0;
        return status >= 400 && status < 500 && status !== 408 && status !== 429;
    }

    // sweepEmergencyCopies drops emergency copies older than EMERGENCY_MAX_AGE_MS and unreadable ones.
    function sweepEmergencyCopies() {
        const keys = [];
        try {
            for (let i = 0; i < localStorage.length; i++) {
                const key = localStorage.key(i);
                if (key && key.startsWith(EMERGENCY_PREFIX)) keys.push(key);
            }
        } catch (err) { return; /* storage may be blocked */ }
        const now = Date.now();
        keys.forEach(key => {
            const copy = ED.core.storage.get(key, null);
            const at = copy ? Number(copy.at) : NaN;
            if (!Number.isFinite(at) || now - at > EMERGENCY_MAX_AGE_MS) ED.core.storage.remove(key);
        });
    }

    // create returns a saver. options:
    //   api, flowId, model, revision,
    //   onState(state)                       saved | dirty | saving | invalid | offline | failed | conflict
    //   onSaved({revision, issues})
    //   onInvalid(issues)
    //   onConflict() → Promise<"reload" | "keep">
    // A transient error (network, FLOWS_DISABLED, FLOW_INTERNAL, 5xx, 429) sets "offline" and retries
    // after 5 s, doubling up to 60 s until a request succeeds. A permanent error (see permanent) sets
    // "failed" with no automatic retry; the emergency copy stays and the next change or save() tries
    // again. A document the server refused as FLOW_INVALID is not sent again until it changes.
    // saver.error holds the error of the last offline or failed request until a request succeeds.
    // A throwing callback never changes the saver's state.
    function create(options) {
        const core = ED.core;
        const o = options;
        let revision = o.revision;
        let savedVersion = o.model.version;
        let invalidVersion = -1;      // the model version the server refused as FLOW_INVALID
        let conflictChoice = null;    // "keep" or "reload" while the server draft of a conflict is not loaded yet
        let state = 'saved';
        let inFlight = null;
        let pending = false;
        let timer = 0;
        let retryTimer = 0;
        let retryDelay = RETRY_MS;
        let error = null;
        let disposed = false;

        sweepEmergencyCopies();

        function notify(fn, arg) {
            if (typeof fn !== 'function') return;
            try { fn(arg); } catch (err) { console.error('EasyDrag saver callback failed', err); }
        }

        function setState(next) {
            if (next === 'saved') error = null;
            if (state === next) return;
            state = next;
            notify(o.onState, next);
        }

        function writeEmergency() {
            core.storage.set(emergencyKey(o.flowId), { revision, at: Date.now(), doc: o.model.toJSON() });
        }

        // goOffline keeps err and calls save() again after the backoff delay, which then doubles.
        function goOffline(err) {
            error = err;
            setState('offline');
            clearTimeout(retryTimer);
            if (disposed) return;
            retryTimer = setTimeout(() => { save(); }, retryDelay);
            retryDelay = Math.min(retryDelay * 2, RETRY_MAX_MS);
        }

        function schedule() {
            if (disposed) return;
            if (o.model.version === savedVersion) return;
            setState(state === 'conflict' ? 'conflict' : 'dirty');
            writeEmergency();
            clearTimeout(timer);
            timer = setTimeout(() => { save(); }, DELAY_MS);
        }

        async function save() {
            clearTimeout(timer);
            if (disposed || state === 'conflict') return undefined;
            if (inFlight) { pending = true; return inFlight; }
            clearTimeout(retryTimer);
            if (conflictChoice) {
                setState('saving');
                return run(loadServerDraft);
            }
            if (o.model.version === savedVersion) { setState('saved'); return undefined; }
            if (o.model.version === invalidVersion) { setState('invalid'); return undefined; }
            const version = o.model.version;
            const doc = o.model.toJSON();
            setState('saving');
            return run(() => put(version, doc));
        }

        // run performs one request at a time. op returns true when a save must follow at once. A
        // save() that came in meanwhile follows too, unless the request went offline (the retry
        // timer covers it) or the model has not changed since the request started.
        function run(op) {
            const version = o.model.version;
            inFlight = (async () => {
                let again = false;
                try {
                    again = await op();
                } finally {
                    inFlight = null;
                }
                const requested = pending;
                pending = false;
                if (disposed) return undefined;
                if (again || (requested && state !== 'offline' && state !== 'conflict' && o.model.version !== version)) return save();
                return undefined;
            })();
            return inFlight;
        }

        // put sends the draft; it returns true when the model changed meanwhile and must be saved again.
        async function put(version, doc) {
            let res;
            try {
                res = await o.api.save(o.flowId, doc, revision);
                // A proxy's 200 without the JSON answer must not count as saved.
                if (!Number.isInteger(res && res.draft_revision)) throw new Error('the save answer has no draft revision');
            } catch (err) {
                return refused(err, version);
            }
            revision = res.draft_revision;
            savedVersion = version;
            error = null;
            retryDelay = RETRY_MS;
            notify(o.onSaved, { revision, issues: res.issues || [] });
            if (o.model.version === savedVersion) {
                core.storage.remove(emergencyKey(o.flowId));
                setState('saved');
                return false;
            }
            writeEmergency(); // the copy must carry the new base revision, or it is never offered again
            setState('dirty');
            return true;
        }

        async function refused(err, version) {
            const code = core.errorCode(err);
            if (code === 'FLOW_REVISION_CONFLICT') return resolveConflict();
            if (code === 'FLOW_INVALID') {
                invalidVersion = version;
                error = null;
                retryDelay = RETRY_MS;
                setState('invalid');
                notify(o.onInvalid, (err.body && err.body.issues) || []);
                return false;
            }
            if (permanent(err)) {
                error = err;
                retryDelay = RETRY_MS;
                setState('failed');
                return false;
            }
            goOffline(err);
            return false;
        }

        // resolveConflict asks the user once, then loads the server draft for the choice.
        async function resolveConflict() {
            if (disposed) return false;
            error = null;
            setState('conflict');
            let choice;
            try {
                choice = o.onConflict ? await o.onConflict() : 'reload';
            } catch (err) {
                if (!disposed) goOffline(err);
                return false;
            }
            if (disposed) return false;
            conflictChoice = choice === 'keep' ? 'keep' : 'reload';
            return loadServerDraft();
        }

        // loadServerDraft fetches the server draft and applies the remembered conflict choice. While
        // the fetch fails the saver stays offline and retries only the fetch, without asking again.
        async function loadServerDraft() {
            let server;
            try {
                server = await o.api.get(o.flowId);
                if (!Number.isInteger(server && server.flow && server.flow.draft_revision)) throw new Error('the flow answer has no draft revision');
            } catch (err) {
                if (!disposed) goOffline(err);
                return false;
            }
            const choice = conflictChoice;
            conflictChoice = null;
            revision = server.flow.draft_revision;
            error = null;
            retryDelay = RETRY_MS;
            if (choice === 'keep') {
                writeEmergency(); // the kept draft now builds on the server's revision
                savedVersion = -1;
                setState('dirty');
                return true;
            }
            if (disposed) return false;
            o.model.replaceDoc(server.flow.draft);
            savedVersion = o.model.version;
            core.storage.remove(emergencyKey(o.flowId));
            setState('saved');
            return false;
        }

        async function flush() {
            clearTimeout(timer);
            if (inFlight) await inFlight;
            await save();
            return state === 'saved';
        }

        return {
            schedule,
            save,
            flush,
            get revision() { return revision; },
            set revision(value) { revision = value; },
            get state() { return state; },
            get error() { return error; },
            isDirty: () => o.model.version !== savedVersion,
            markSaved() { savedVersion = o.model.version; setState('saved'); },
            dispose() { disposed = true; clearTimeout(timer); clearTimeout(retryTimer); }
        };
    }

    // emergencyCopy returns a locally kept draft that is newer than the server's revision.
    function emergencyCopy(flowId, serverRevision) {
        const copy = ED.core.storage.get(emergencyKey(flowId), null);
        if (!copy || !copy.doc || copy.revision !== serverRevision) return null;
        return copy;
    }

    function dropEmergencyCopy(flowId) { ED.core.storage.remove(emergencyKey(flowId)); }

    ED.saver = { create, emergencyCopy, dropEmergencyCopy };
})();
