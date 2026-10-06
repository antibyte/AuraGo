// EasyDrag autosave: saves the draft 1 s after the last change, keeps an emergency copy in
// localStorage, serialises requests and resolves revision conflicts with the user.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const DELAY_MS = 1000;
    const RETRY_MS = 5000;

    function emergencyKey(flowId) { return 'aurago.easydrag.draft.' + flowId; }

    // create returns a saver. options:
    //   api, flowId, model, revision,
    //   onState(state)                       saved | dirty | saving | invalid | offline | conflict
    //   onSaved({revision, issues})
    //   onInvalid(issues)
    //   onConflict() → Promise<"reload" | "keep">
    function create(options) {
        const core = ED.core;
        const o = options;
        let revision = o.revision;
        let savedVersion = o.model.version;
        let state = 'saved';
        let inFlight = null;
        let pending = false;
        let timer = 0;
        let retryTimer = 0;
        let disposed = false;

        function setState(next) {
            if (state === next) return;
            state = next;
            if (o.onState) o.onState(next);
        }

        function writeEmergency() {
            core.storage.set(emergencyKey(o.flowId), { revision, at: Date.now(), doc: o.model.toJSON() });
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
            clearTimeout(retryTimer);
            if (disposed || state === 'conflict') return;
            if (inFlight) { pending = true; return inFlight; }
            if (o.model.version === savedVersion) { setState('saved'); return; }
            const version = o.model.version;
            const doc = o.model.toJSON();
            setState('saving');
            inFlight = (async () => {
                try {
                    const res = await o.api.save(o.flowId, doc, revision);
                    revision = res.draft_revision;
                    savedVersion = version;
                    if (o.onSaved) o.onSaved({ revision, issues: res.issues || [] });
                    if (o.model.version === savedVersion) {
                        core.storage.remove(emergencyKey(o.flowId));
                        setState('saved');
                    } else {
                        setState('dirty');
                        pending = true;
                    }
                } catch (err) {
                    const code = core.errorCode(err);
                    if (code === 'FLOW_REVISION_CONFLICT') {
                        setState('conflict');
                        await resolveConflict();
                    } else if (code === 'FLOW_INVALID') {
                        savedVersion = version;
                        setState('invalid');
                        if (o.onInvalid) o.onInvalid((err.body && err.body.issues) || []);
                    } else {
                        setState('offline');
                        retryTimer = setTimeout(() => { save(); }, RETRY_MS);
                    }
                } finally {
                    inFlight = null;
                }
                if (pending && !disposed) {
                    pending = false;
                    if (o.model.version !== savedVersion && state !== 'conflict') return save();
                }
                return undefined;
            })();
            return inFlight;
        }

        async function resolveConflict() {
            const choice = o.onConflict ? await o.onConflict() : 'reload';
            let server;
            try { server = await o.api.get(o.flowId); } catch (err) { setState('offline'); return; }
            revision = server.flow.draft_revision;
            if (choice === 'keep') {
                setState('dirty');
                savedVersion = -1;
                pending = true;
                return;
            }
            o.model.replaceDoc(server.flow.draft);
            savedVersion = o.model.version;
            core.storage.remove(emergencyKey(o.flowId));
            setState('saved');
        }

        async function flush() {
            clearTimeout(timer);
            if (inFlight) await inFlight;
            if (o.model.version !== savedVersion && state !== 'conflict') await save();
            return state === 'saved';
        }

        return {
            schedule,
            save,
            flush,
            get revision() { return revision; },
            set revision(value) { revision = value; },
            get state() { return state; },
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
